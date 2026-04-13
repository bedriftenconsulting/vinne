package main

// CI Build Trigger: 2024-10-09
import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"database/sql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	pb "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/service-wallet/internal/clients"
	"github.com/randco/service-wallet/internal/config"
	"github.com/randco/service-wallet/internal/events"
	"github.com/randco/service-wallet/internal/grpc/server"
	"github.com/randco/service-wallet/internal/repositories"
	"github.com/randco/service-wallet/internal/services"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Version information (injected at build time)
var (
	Version        = "dev"
	GitBranch      = "unknown"
	GitCommit      = "unknown"
	GitCommitCount = "0"
	BuildTime      = "unknown"
)

func main() {
	// Load configuration
	// Log version information
	fmt.Printf("Starting service-wallet Service\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Git Branch: %s, Commit: %s (#%s)\n", GitBranch, GitCommit, GitCommitCount)
	fmt.Printf("Build Time: %s\n", BuildTime)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	_ = log.New(os.Stdout, "[wallet-service] ", log.LstdFlags|log.Lshortfile)

	// Initialize tracing if enabled
	if cfg.Tracing.Enabled {
		tp, err := initTracer(cfg.Tracing)
		if err != nil {
			log.Printf("Failed to initialize tracing: %v", err)
		} else {
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := tp.Shutdown(ctx); err != nil {
					log.Printf("Error shutting down tracer provider: %v", err)
				}
			}()
			log.Printf("Tracing initialized successfully - endpoint: %s, sample_rate: %f, service: %s",
				cfg.Tracing.JaegerEndpoint, cfg.Tracing.SampleRate, cfg.Tracing.ServiceName)
		}
	}

	// Initialize database
	db, err := sql.Open("postgres", cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Ping database to ensure connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Create sqlx wrapper for idempotency repository
	sqlxDB := sqlx.NewDb(db, "postgres")

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Initialize event publisher (Kafka) - using nil for now as it's optional
	var eventPublisher *events.Publisher
	// TODO: Initialize Kafka publisher when Kafka config is ready
	// eventPublisher, err := events.NewPublisher(cfg.Kafka.Brokers)
	// if err != nil {
	//     log.Printf("Warning: Failed to initialize event publisher: %v", err)
	// }

	// Initialize repositories
	walletRepo := repositories.NewWalletRepository(db)
	extendedWalletRepo := repositories.NewExtendedWalletRepository(db)
	walletTransactionRepo := repositories.NewWalletTransactionRepository(db, redisClient)
	adminTransactionRepo := repositories.NewAdminTransactionRepository(db, redisClient)
	commissionRepo := repositories.NewCommissionRepository(db)
	idempotencyRepo := repositories.NewIdempotencyRepository(sqlxDB)
	reservationRepo := repositories.NewReservationRepository(sqlxDB)
	reversalRepo := repositories.NewTransactionReversalRepository(db, redisClient)

	// Initialize agent client (optional - wallet service can work without it)
	var agentClient services.AgentClient
	agentServiceAddr := cfg.GetAgentManagementAddr()
	if agentServiceAddr == "" {
		agentServiceAddr = "localhost:50058" // Fallback for local development
	}
	if client, err := initAgentClient(agentServiceAddr); err != nil {
		log.Printf("Warning: Failed to connect to agent management service: %v", err)
		log.Printf("Wallet service will use default commission rates")
		// Continue without agent client - wallet service will use defaults
	} else {
		agentClient = client
		log.Printf("Connected to agent management service at %s", agentServiceAddr)
	}

	// Initialize commission service first (since wallet service depends on it)
	commissionService := services.NewCommissionService(
		db,
		redisClient,
		commissionRepo,
	)

	// Initialize wallet service
	walletService := services.NewWalletService(
		db,
		redisClient,
		walletRepo,
		extendedWalletRepo,
		walletTransactionRepo,
		adminTransactionRepo,
		idempotencyRepo,
		reservationRepo,
		reversalRepo,
		commissionService,
		eventPublisher,
		agentClient,
	)

	// Create gRPC server with tracing
	var grpcOpts []grpc.ServerOption
	if cfg.Tracing.Enabled {
		grpcOpts = append(grpcOpts,
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
		)
	}
	grpcServer := grpc.NewServer(grpcOpts...)

	// Register wallet service
	walletHandler := server.NewWalletServer(walletService, commissionService, agentClient)
	pb.RegisterWalletServiceServer(grpcServer, walletHandler)

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection for development
	reflection.Register(grpcServer)

	// Start server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.Port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Wallet service starting on port %d", cfg.Server.Port)

	// Start gRPC server in goroutine
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down server...")
	grpcServer.GracefulStop()
	log.Println("Server stopped")
}

// initTracer initializes OpenTelemetry tracing
func initTracer(cfg config.TracingConfig) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	// Convert old Jaeger endpoint format to OTLP format if needed
	endpoint := cfg.JaegerEndpoint
	if strings.Contains(endpoint, "/api/traces") {
		endpoint = strings.Replace(endpoint, ":14268/api/traces", ":4318", 1)
		endpoint = strings.Replace(endpoint, "/api/traces", "", 1)
	}

	client := otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint(strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")),
		otlptracehttp.WithInsecure(),
	)

	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	// Create resource with service information
	res := resource.NewWithAttributes(
		"", // No schema URL to avoid conflicts
		attribute.String("service.name", cfg.ServiceName),
		attribute.String("service.version", cfg.ServiceVersion),
		attribute.String("environment", cfg.Environment),
		attribute.String("service.namespace", "randco"),
	)

	// Create tracer provider with sampling
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return tp, nil
}

// initAgentClient initializes the agent management service client
func initAgentClient(address string) (services.AgentClient, error) {
	client, err := clients.NewAgentClient(address)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Build trigger: 1759076135

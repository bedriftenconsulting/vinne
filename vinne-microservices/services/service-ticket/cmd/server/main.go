package main

// CI Build Trigger: 2024-10-09 v2
import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	agentmanagementv1 "github.com/randco/randco-microservices/proto/agent/management/v1"
	gamev1 "github.com/randco/randco-microservices/proto/game/v1"
	ticketv1 "github.com/randco/randco-microservices/proto/ticket/v1"
	walletpb "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/randco-microservices/services/service-ticket/internal/config"
	"github.com/randco/randco-microservices/services/service-ticket/internal/handlers"
	"github.com/randco/randco-microservices/services/service-ticket/internal/repositories"
	"github.com/randco/randco-microservices/services/service-ticket/internal/services"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("✓ Configuration loaded successfully")
	log.Printf("  Server Port: %d", cfg.Server.Port)
	log.Printf("  Database: %s", cfg.Database.URL)
	log.Printf("  Redis: %s:%d", cfg.Redis.Host, cfg.Redis.Port)
	log.Printf("  Tracing: %v", cfg.Tracing.Enabled)
	log.Printf("  Serial Prefix: %s", cfg.Business.SerialPrefix)

	// Create context for initialization
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Initialize OpenTelemetry tracing
	if cfg.Tracing.Enabled {
		shutdown, err := initTracer(ctx, cfg)
		if err != nil {
			log.Fatalf("Failed to initialize tracer: %v", err)
		}
		defer func() {
			if err := shutdown(context.Background()); err != nil {
				log.Printf("Error shutting down tracer: %v", err)
			}
		}()
		log.Printf("✓ OpenTelemetry tracing initialized")
	}

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	// Configure connection pool
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	// Verify database connection
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Printf("✓ Database connection established")

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("Error closing Redis: %v", err)
		}
	}()

	// Verify Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("⚠ Warning: Failed to connect to Redis: %v", err)
		log.Printf("  Service will continue without caching")
		// Set redisClient to nil to indicate it's not available
		redisClient = nil
	} else {
		log.Printf("✓ Redis connection established")
	}

	// Connect to Wallet Service
	walletAddr := fmt.Sprintf("%s:%d", cfg.Services.Wallet.Host, cfg.Services.Wallet.Port)
	walletConn, err := grpc.NewClient(walletAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Wallet Service at %s: %v", walletAddr, err)
	}
	defer func() {
		if err := walletConn.Close(); err != nil {
			log.Printf("Error closing Wallet Service connection: %v", err)
		}
	}()
	walletClient := walletpb.NewWalletServiceClient(walletConn)
	log.Printf("✓ Wallet Service client connected to %s", walletAddr)

	// Connect to Agent Management Service
	agentMgmtAddr := fmt.Sprintf("%s:%d", cfg.Services.AgentManagement.Host, cfg.Services.AgentManagement.Port)
	agentMgmtConn, err := grpc.NewClient(agentMgmtAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Agent Management Service at %s: %v", agentMgmtAddr, err)
	}
	defer func() {
		if err := agentMgmtConn.Close(); err != nil {
			log.Printf("Error closing Agent Management Service connection: %v", err)
		}
	}()
	agentMgmtClient := agentmanagementv1.NewAgentManagementServiceClient(agentMgmtConn)
	log.Printf("✓ Agent Management Service client connected to %s", agentMgmtAddr)

	// Connect to Game Service
	gameAddr := fmt.Sprintf("%s:%d", cfg.Services.Game.Host, cfg.Services.Game.Port)
	gameConn, err := grpc.NewClient(gameAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Game Service at %s: %v", gameAddr, err)
	}
	defer func() {
		if err := gameConn.Close(); err != nil {
			log.Printf("Error closing Game Service connection: %v", err)
		}
	}()
	gameClient := gamev1.NewGameServiceClient(gameConn)
	log.Printf("✓ Game Service client connected to %s", gameAddr)

	// Initialize repositories
	ticketRepo := repositories.NewTicketRepository(db)
	log.Printf("✓ Repositories initialized")

	// Initialize ticket service
	ticketService := services.NewTicketService(
		db,
		ticketRepo,
		walletClient,
		agentMgmtClient,
		gameClient,
		redisClient,
		&services.ServiceConfig{
			Security: services.SecurityConfig{
				SecretKey:     os.Getenv("TICKET_SECRET_KEY"),
				SerialPrefix:  cfg.Business.SerialPrefix,
				QRCodeBaseURL: os.Getenv("QRCODE_BASE_URL"),
				BarcodePrefix: cfg.Business.SerialPrefix,
			},
		},
	)
	log.Printf("✓ Ticket service initialized")

	// Create gRPC handler
	ticketHandler := handlers.NewTicketHandler(ticketService)
	log.Printf("✓ gRPC handler initialized")

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register gRPC handlers
	ticketv1.RegisterTicketServiceServer(grpcServer, ticketHandler)
	log.Printf("✓ gRPC handlers registered")

	// Listen on configured port
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	// Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Start gRPC server in goroutine
	go func() {
		log.Printf("✓ Ticket Service starting on port %d", cfg.Server.Port)
		log.Printf("✓ gRPC server ready and listening")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-shutdown
	log.Printf("Shutting down gracefully...")

	// Stop accepting new connections
	grpcServer.GracefulStop()
	log.Printf("✓ Ticket Service stopped")
}

// initTracer initializes OpenTelemetry tracer with OTLP exporter
func initTracer(ctx context.Context, cfg *config.Config) (func(context.Context) error, error) {
	// Create OTLP HTTP exporter
	exporter, err := otlptrace.New(ctx,
		otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(cfg.Tracing.JaegerEndpoint),
			otlptracehttp.WithInsecure(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.Tracing.ServiceName),
			semconv.ServiceVersion(cfg.Tracing.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Tracing.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(cfg.Tracing.SampleRate),
		)),
	)

	// Register as global tracer provider
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

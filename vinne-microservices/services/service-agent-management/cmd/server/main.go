package main

// Rebuild trigger: 2025-09-28T21:28:03Z

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

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/config"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/grpc/server"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/repositories"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/services"
	"github.com/randco/randco-microservices/shared/common/logger"
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
	// Load environment variables from .env file
	// Log version information
	fmt.Printf("Starting service-agent-management Service\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Git Branch: %s, Commit: %s (#%s)\n", GitBranch, GitCommit, GitCommitCount)
	fmt.Printf("Build Time: %s\n", BuildTime)

	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Warning: .env file not found\n")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logConfig := logger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		ServiceName: "service-agent-management",
		LogFile:     "logs/service-agent-management.log",
	}
	logger := logger.NewLogger(logConfig)
	defer func() {
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing logger: %v\n", err)
		}
	}()

	// Initialize tracing if enabled
	if cfg.Tracing.Enabled {
		tp, err := initTracer(cfg.Tracing)
		if err != nil {
			logger.Error("Failed to initialize tracing", "error", err)
		} else {
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := tp.Shutdown(ctx); err != nil {
					logger.Error("Error shutting down tracer provider", "error", err)
				}
			}()
			logger.Info("Tracing initialized successfully",
				"endpoint", cfg.Tracing.JaegerEndpoint,
				"sample_rate", cfg.Tracing.SampleRate,
				"service", cfg.Tracing.ServiceName)
		}
	}

	// Connect to database - always use DATABASE_URL
	if cfg.Database.URL == "" {
		logger.Fatal("DATABASE_URL must be set")
	}

	db, err := sqlx.Connect("postgres", cfg.Database.URL)
	if err != nil {
		logger.Fatal("Failed to connect to database", "error", err)
	}
	defer func() { _ = db.Close() }()

	logger.Info("Connected to database successfully", "host", cfg.Database.Host, "port", cfg.Database.Port, "database", cfg.Database.Name)

	// Initialize repositories
	repos := repositories.NewRepositories(db)

	// Initialize services with wallet and Kafka configuration
	serviceConfig := &services.ServiceConfig{
		WalletServiceAddress:    cfg.Services.WalletService,
		AgentAuthServiceAddress: cfg.Services.AgentAuthService,
		KafkaBrokers:            []string{"localhost:9092"},
	}
	agentService := services.NewAgentService(repos, serviceConfig)
	posDeviceService := services.NewPOSDeviceService(repos)
	retailerService := services.NewRetailerService(repos, serviceConfig)
	retailerAssignmentService := services.NewRetailerAssignmentService(repos)

	// Create gRPC server with tracing interceptors
	var grpcOpts []grpc.ServerOption
	if cfg.Tracing.Enabled {
		grpcOpts = append(grpcOpts,
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
		)
	}
	grpcServer := grpc.NewServer(grpcOpts...)

	// Create and register the agent management gRPC server
	agentManagementGrpcServer := server.NewAgentManagementServer(
		agentService,
		retailerService,
		retailerAssignmentService,
		posDeviceService,
		logger,
	)
	agentManagementGrpcServer.RegisterServer(grpcServer)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection service for debugging
	reflection.Register(grpcServer)

	// Start listening
	listener, err := net.Listen("tcp", cfg.GetServerAddress())
	if err != nil {
		logger.Fatal("Failed to listen", "address", cfg.GetServerAddress(), "error", err)
	}

	// Start server in goroutine
	go func() {
		logger.Info("Agent Management Service starting", "address", cfg.GetServerAddress())
		logger.Info("Database connection", "host", cfg.Database.Host, "port", cfg.Database.Port, "database", cfg.Database.Name)
		logger.Info("Redis connection", "host", cfg.Redis.Host, "port", cfg.Redis.Port)
		if err := grpcServer.Serve(listener); err != nil {
			logger.Fatal("Failed to serve gRPC server", "error", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down server...")
	grpcServer.GracefulStop()
	logger.Info("Server stopped")
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

// Build trigger: 1759076135

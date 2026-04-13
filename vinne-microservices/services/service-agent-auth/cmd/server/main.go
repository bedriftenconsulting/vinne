package main

// Rebuild trigger: 2025-09-28T21:28:03Z

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
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

	pb "github.com/randco/randco-microservices/proto/agent/auth/v1"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/clients"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/config"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/grpc/server"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/repositories"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/services"
	"github.com/randco/randco-microservices/shared/common/jwt"
	"github.com/randco/randco-microservices/shared/common/logger"
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
	// Load environment variables
	// Log version information
	fmt.Printf("Starting service-agent-auth Service\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Git Branch: %s, Commit: %s (#%s)\n", GitBranch, GitCommit, GitCommitCount)
	fmt.Printf("Build Time: %s\n", BuildTime)

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Warning: .env file not found\n")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := logger.NewLogger(logger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		ServiceName: "service-agent-auth",
		LogFile:     "logs/service-agent-auth.log",
	})
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

	if cfg.Database.URL == "" {
		logger.Fatal("DATABASE_URL must be set")
	}

	db, err := sqlx.Connect("postgres", cfg.Database.URL)
	if err != nil {
		logger.Fatal("Failed to connect to database", "error", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Ping(); err != nil {
		logger.Fatal("Failed to ping database", "error", err)
	}
	logger.Info("Connected to database")

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	authRepo := repositories.NewAuthRepository(db, redisClient)
	sessionRepo := repositories.NewSessionRepository(db.DB, redisClient)
	tokenRepo := repositories.NewTokenRepository(redisClient)
	offlineTokenRepo := repositories.NewOfflineTokenRepository(db)

	agentNotificationClient, err := clients.NewAgentNotificationClient(cfg.Service.Notification)
	if err != nil {
		logger.Fatal("failed to create notification client", "error", err)
	}
	defer agentNotificationClient.Close()

	// Initialize service layer
	authService := services.NewAuthService(
		authRepo,
		sessionRepo,
		tokenRepo,
		offlineTokenRepo,
		cfg.JWT.AccessTokenExpiry,    // Already time.Duration from Viper parsing
		cfg.JWT.RefreshTokenExpiry,   // Already time.Duration from Viper parsing
		cfg.Security.MaxFailedLogins, // Already int from Viper parsing
		cfg.Security.LockoutDuration, // Already time.Duration from Viper parsing
		*agentNotificationClient,
		jwt.NewService(jwt.Config{
			AccessSecret:    cfg.JWT.Secret,
			RefreshSecret:   cfg.JWT.Secret + "-refresh",
			AccessDuration:  cfg.JWT.AccessTokenExpiry,
			RefreshDuration: cfg.JWT.RefreshTokenExpiry,
		}),
	)

	// Create gRPC server with tracing interceptors
	var grpcOpts []grpc.ServerOption
	if cfg.Tracing.Enabled {
		grpcOpts = append(grpcOpts,
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
		)
	}
	grpcServer := grpc.NewServer(grpcOpts...)

	// Register the agent auth service
	authServer := server.NewAgentAuthServer(authService)
	pb.RegisterAgentAuthServiceServer(grpcServer, authServer)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection service for debugging
	reflection.Register(grpcServer)

	// Create listener
	port := fmt.Sprintf(":%d", cfg.Server.Port)
	lis, err := net.Listen("tcp", port)
	if err != nil {
		logger.Fatal("Failed to listen on port", "port", port, "error", err)
	}

	// Start server in goroutine
	go func() {
		logger.Info("Agent Auth Service starting", "port", port)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("Failed to serve", "error", err)
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

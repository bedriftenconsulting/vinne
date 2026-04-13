package main

// Rebuild trigger: 2025-09-28T21:28:03Z

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	adminmanagementv1 "github.com/randco/randco-microservices/proto/admin/management/v1"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/config"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/repositories"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/services"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/randco-microservices/shared/events"
	"github.com/randco/randco-microservices/shared/middleware/auth"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
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
	// Load configuration (Viper handles .env files and environment variables)
	// Log version information
	fmt.Printf("Starting service-admin-management Service\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Git Branch: %s, Commit: %s (#%s)\n", GitBranch, GitCommit, GitCommitCount)
	fmt.Printf("Build Time: %s\n", BuildTime)

	// Load configuration (Viper handles .env files and environment variables)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger := logger.NewLogger(logger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		ServiceName: "service-admin-management",
		LogFile:     cfg.Logging.LogFile,
	})
	defer func() {
		if err := logger.Close(); err != nil {
			// Logger already closed or error occurred
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

	// Connect to database using config - always use DATABASE_URL
	if cfg.Database.URL == "" {
		logger.Fatal("DATABASE_URL must be set")
	}

	db, err := sql.Open("postgres", cfg.Database.URL)
	if err != nil {
		logger.Fatal("Failed to connect to database", "error", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("Error closing database connection", "error", err)
		}
	}()

	// Configure database connection pool
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	// Test database connection
	if err := db.Ping(); err != nil {
		logger.Fatal("Failed to ping database", "error", err)
	}
	logger.Info("Connected to database successfully", "max_conns", cfg.Database.MaxOpenConns)

	// Initialize repositories
	repos := repositories.NewRepositories(db)

	// Initialize JWT manager using config
	jwtConfig := auth.JWTConfig{
		Secret:             cfg.Security.JWTSecret,
		AccessTokenExpiry:  cfg.Security.AccessTokenExpiry,
		RefreshTokenExpiry: cfg.Security.RefreshTokenExpiry,
		Issuer:             cfg.Security.JWTIssuer,
	}
	jwtManager := auth.NewJWTManager(jwtConfig)

	// Initialize event bus (can be nil if not configured)
	var eventBus events.EventBus
	// TODO: Initialize actual event bus implementation if needed
	// eventBus = events.NewKafkaEventBus(cfg.Kafka)

	// Initialize auth service
	authService := services.NewAuthService(
		repos.AdminUser,
		repos.AdminUserAuth,
		repos.Session,
		repos.AuditLog,
		jwtManager,
		eventBus,
		cfg,
	)

	// Initialize gRPC service with auth service and Kafka config
	adminManagementService := services.NewAdminManagementService(repos, authService, cfg.Kafka.Brokers)

	// Create gRPC server with tracing interceptors
	var grpcOpts []grpc.ServerOption
	if cfg.Tracing.Enabled {
		grpcOpts = append(grpcOpts,
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
		)
	}
	grpcServer := grpc.NewServer(grpcOpts...)
	adminmanagementv1.RegisterAdminManagementServiceServer(grpcServer, adminManagementService)

	// Enable reflection for grpcurl and other tools
	reflection.Register(grpcServer)

	// Start listening
	listener, err := net.Listen("tcp", ":"+cfg.Server.Port)
	if err != nil {
		logger.Fatal("Failed to listen on port", "port", cfg.Server.Port, "error", err)
	}

	logger.Info("Admin Management Service starting", "port", cfg.Server.Port)
	logger.Info("Server configuration", "mode", cfg.Server.Mode, "database", cfg.Database.URL, "log_level", cfg.Logging.Level)

	// Start gRPC server
	if err := grpcServer.Serve(listener); err != nil {
		logger.Fatal("Failed to start gRPC server", "error", err)
	}
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

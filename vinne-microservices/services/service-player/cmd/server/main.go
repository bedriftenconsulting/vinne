package main

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

	_ "github.com/jackc/pgx/v5/stdlib"
	playerv1 "github.com/randco/randco-microservices/proto/player/v1"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/service-player/internal/clients"
	"github.com/randco/service-player/internal/config"
	"github.com/randco/service-player/internal/handlers"
	"github.com/randco/service-player/internal/repositories"
	"github.com/randco/service-player/internal/services"
	"github.com/randco/service-player/internal/tracing"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

var (
	Version        = "dev"
	GitBranch      = "unknown"
	GitCommit      = "unknown"
	GitCommitCount = "0"
	BuildTime      = "unknown"
)

func main() {
	fmt.Printf("Starting service-player Service\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Git Branch: %s, Commit: %s (#%s)\n", GitBranch, GitCommit, GitCommitCount)
	fmt.Printf("Build Time: %s\n", BuildTime)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger := logger.NewLogger(logger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		ServiceName: "service-player",
		LogFile:     cfg.Logging.LogFile,
	})
	defer func() {
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing logger: %v\n", err)
		}
	}()

	// Initialize tracing
	tracingProvider, err := tracing.NewProvider(context.Background(), cfg.Tracing)
	if err != nil {
		logger.Error("Failed to initialize tracing", "error", err)
	} else {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tracingProvider.Shutdown(ctx); err != nil {
				logger.Error("Error shutting down tracer provider", "error", err)
			}
		}()
		logger.Info("Tracing initialized successfully",
			"service", cfg.Tracing.ServiceName,
			"version", cfg.Tracing.ServiceVersion,
			"environment", cfg.Tracing.Environment,
			"exporter", cfg.Tracing.ExporterType,
			"sample_rate", cfg.Tracing.SampleRate)
	}

	// Initiate Database Connection
	if cfg.Database.URL == "" {
		logger.Fatal("DATABASE_URL must be set")
	}

	db, err := sql.Open("pgx", cfg.Database.URL)
	if err != nil {
		logger.Fatal("Failed to connect to database", "error", err)
	}
	fmt.Println("Database connection opened successfully")

	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to ping database: %v\n", err)
	}

	fmt.Println("Database ping successful")
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("Error closing database connection", "error", err)
		}
	}()

	// Initialize repositories
	repos := repositories.NewRepositories(db)

	// Initialize external service clients
	notificationClient, err := clients.NewNotificationClient(cfg.Services.Notification)
	if err != nil {
		logger.Fatal("Failed to create notification client", "error", err)
	}

	walletClient, err := clients.NewWalletClient(cfg.Services.Wallet)
	if err != nil {
		logger.Fatal("Failed to create wallet client", "error", err)
	}

	paymentClient, err := clients.NewPaymentClient(cfg.Services.Payment)
	if err != nil {
		logger.Fatal("Failed to create payment client", "error", err)
	}

	// Initialize services with notification client and wallet client
	services := services.NewServices(repos, &cfg.Security, notificationClient, walletClient)

	playerHandler := handlers.NewPlayerServiceHandler(
		services.PlayerAuth,
		services.Registration,
		services.Profile,
		services.Session,
		services.Admin,
		walletClient,
		paymentClient,
		notificationClient,
		services.OTP,
	)

	var grpcOpts []grpc.ServerOption

	grpcOpts = append(grpcOpts,
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 5 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)

	if cfg.Tracing.Enabled {
		grpcOpts = append(grpcOpts,
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
		)
	}

	grpcServer := grpc.NewServer(grpcOpts...)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	reflection.Register(grpcServer)

	playerv1.RegisterPlayerServiceServer(grpcServer, playerHandler)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.Port))
	if err != nil {
		logger.Fatal("Failed to listen on port %d: %v", cfg.Server.Port, err)
	}

	logger.Info("Player Service starting on port %d", cfg.Server.Port)
	logger.Info("Environment: %s", cfg.Server.Environment)
	logger.Info("Tracing enabled: %v", cfg.Tracing.Enabled)
	logger.Info("Authentication middleware enabled")

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("Failed to serve gRPC server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down Player Service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	grpcServer.GracefulStop()

	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	<-ctx.Done()

	log.Println("Player Service stopped")
}

package main

// CI Build Trigger: 2024-10-09

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	drawpb "github.com/randco/randco-microservices/proto/draw/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"

	"github.com/randco/service-draw/internal/config"
	grpcclient "github.com/randco/service-draw/internal/grpc"
	"github.com/randco/service-draw/internal/handlers"
	"github.com/randco/service-draw/internal/repositories"
	"github.com/randco/service-draw/internal/services"
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
	fmt.Printf("Starting service-draw Service\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Git Branch: %s, Commit: %s (#%s)\n", GitBranch, GitCommit, GitCommitCount)
	fmt.Printf("Build Time: %s\n", BuildTime)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize tracing
	if cfg.Tracing.Enabled {
		tp, err := initTracer(cfg.Tracing.ServiceName, cfg.Tracing.Endpoint)
		if err != nil {
			log.Fatalf("Failed to initialize tracer: %v", err)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tp.Shutdown(ctx); err != nil {
				log.Printf("Error shutting down tracer: %v", err)
			}
		}()
	}

	// Initialize database
	db, err := initDatabase(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize logger
	logger := log.New(os.Stdout, "[draw-service] ", log.LstdFlags|log.Lshortfile)

	// Initialize gRPC client manager
	grpcClientManager := grpcclient.NewClientManager(logger)

	// Register service clients
	ticketAddress := fmt.Sprintf("%s:%s", cfg.Services.TicketServiceHost, cfg.Services.TicketServicePort)
	grpcClientManager.RegisterService(grpcclient.ServiceConfig{
		Name:    "ticket",
		Address: ticketAddress,
		Timeout: 30 * time.Second,
	})

	walletAddress := fmt.Sprintf("%s:%s", cfg.Services.WalletServiceHost, cfg.Services.WalletServicePort)
	grpcClientManager.RegisterService(grpcclient.ServiceConfig{
		Name:    "wallet",
		Address: walletAddress,
		Timeout: 30 * time.Second,
	})

	gameAddress := fmt.Sprintf("%s:%s", cfg.Services.GameServiceHost, cfg.Services.GameServicePort)
	grpcClientManager.RegisterService(grpcclient.ServiceConfig{
		Name:    "game",
		Address: gameAddress,
		Timeout: 30 * time.Second,
	})

	// Ensure client manager is closed on shutdown
	defer func() {
		if err := grpcClientManager.Close(); err != nil {
			log.Printf("Error closing gRPC client manager: %v", err)
		}
	}()

	// Initialize repositories
	drawRepo := repositories.NewDrawRepository(db)

	// Initialize services
	drawService := services.NewDrawService(drawRepo, logger, grpcClientManager)

	// Initialize gRPC server
	grpcServer := grpc.NewServer()

	// Register gRPC services
	drawHandler := handlers.NewDrawServiceServer(drawService, logger)
	drawpb.RegisterDrawServiceServer(grpcServer, drawHandler)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.Port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", cfg.Server.Port, err)
	}

	log.Printf("Draw Service gRPC server starting on port %d", cfg.Server.Port)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	log.Println("Shutting down Draw Service...")

	// Graceful shutdown
	grpcServer.GracefulStop()
	log.Println("Draw Service shut down successfully")
}

func initTracer(serviceName, endpoint string) (*tracesdk.TracerProvider, error) {
	client := otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	exp, err := otlptrace.New(context.Background(), client)
	if err != nil {
		return nil, err
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(resource.NewWithAttributes(
			"",
			attribute.String("service.name", serviceName),
		)),
	)

	otel.SetTracerProvider(tp)
	return tp, nil
}

func initDatabase(cfg config.DatabaseConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", cfg.URL)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// Build trigger: 1759076135

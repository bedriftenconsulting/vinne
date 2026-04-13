package main

// CI Build Trigger: 2025-10-16

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	pb "github.com/randco/randco-microservices/proto/game/v1"
	"github.com/randco/randco-microservices/services/service-game/internal/config"
	"github.com/randco/randco-microservices/services/service-game/internal/grpc/clients"
	grpcserver "github.com/randco/randco-microservices/services/service-game/internal/grpc/server"
	"github.com/randco/randco-microservices/services/service-game/internal/repositories"
	"github.com/randco/randco-microservices/services/service-game/internal/services"
	"github.com/randco/randco-microservices/shared/events"
	"github.com/randco/randco-microservices/shared/storage"
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
	fmt.Printf("Starting service-game Service\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Git Branch: %s, Commit: %s (#%s)\n", GitBranch, GitCommit, GitCommitCount)
	fmt.Printf("Build Time: %s\n", BuildTime)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger := log.New(os.Stdout, "[game-service] ", log.LstdFlags|log.Lshortfile)

	// Initialize tracing if enabled
	if cfg.Tracing.Enabled {
		tp, err := initTracer(cfg.Tracing)
		if err != nil {
			logger.Printf("Failed to initialize tracing: %v", err)
		} else {
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := tp.Shutdown(ctx); err != nil {
					logger.Printf("Error shutting down tracer provider: %v", err)
				}
			}()
			logger.Printf("Tracing initialized successfully - endpoint: %s, sample_rate: %f, service: %s",
				cfg.Tracing.JaegerEndpoint, cfg.Tracing.SampleRate, cfg.Tracing.ServiceName)
		}
	}

	// Initialize database
	db, err := sql.Open("postgres", cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Configure database connection pool
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	// Ping database to ensure connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

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
	logger.Println("Redis connection successful")

	// Initialize repositories
	logger.Println("Initializing repositories...")
	gameRepo := repositories.NewGameRepository(db)
	// rulesRepo := repositories.NewGameRulesRepository(db) // Not needed without approval workflow
	// prizeRepo := repositories.NewPrizeStructureRepository(db) // Not needed without approval workflow
	// approvalRepo := repositories.NewGameApprovalRepository(db) // DISABLED - approval workflow removed
	scheduleRepo := repositories.NewGameScheduleRepository(db)
	logger.Println("Repositories initialized successfully")

	// Initialize services with Kafka configuration
	logger.Println("Initializing services...")

	// Initialize event bus (Kafka or in-memory fallback) - shared by game and scheduler services
	var serviceConfig *services.ServiceConfig
	var eventBus events.EventBus
	if len(cfg.Kafka.Brokers) > 0 {
		serviceConfig = &services.ServiceConfig{
			KafkaBrokers: cfg.Kafka.Brokers,
		}
		logger.Printf("Kafka configuration provided: %v", cfg.Kafka.Brokers)

		// Create shared event bus
		bus, err := events.NewKafkaEventBus(cfg.Kafka.Brokers)
		if err != nil {
			logger.Printf("Failed to initialize Kafka event bus: %v, using in-memory event bus", err)
			eventBus = events.NewInMemoryEventBus()
		} else {
			eventBus = bus
			logger.Println("Kafka event bus initialized successfully")
		}
	} else {
		logger.Println("No Kafka brokers configured, will use in-memory event bus")
		eventBus = events.NewInMemoryEventBus()
	}

	// Initialize storage client
	logger.Println("Initializing storage client...")
	storageConfig := storage.Config{
		Provider:        storage.Provider(cfg.Storage.Provider),
		Endpoint:        cfg.Storage.Endpoint,
		Region:          cfg.Storage.Region,
		Bucket:          cfg.Storage.Bucket,
		AccessKeyID:     cfg.Storage.AccessKeyID,
		SecretAccessKey: cfg.Storage.SecretAccessKey,
		CDNEndpoint:     cfg.Storage.CDNEndpoint,
		ForcePathStyle:  cfg.Storage.ForcePathStyle,
	}

	storageClient, err := storage.New(storageConfig)
	if err != nil {
		logger.Printf("WARNING: Failed to initialize storage client: %v", err)
		logger.Println("Logo upload/delete features will be disabled")
		storageClient = nil // Game service will handle nil storage gracefully
	} else {
		logger.Printf("Storage client initialized successfully: provider=%s, bucket=%s", cfg.Storage.Provider, cfg.Storage.Bucket)
	}

	gameService := services.NewGameService(
		db,
		redisClient,
		gameRepo,
		storageClient,
		serviceConfig,
	)
	logger.Println("Game service initialized")

	// Initialize approval service with all dependencies - DISABLED (approval workflow removed)
	// approvalService := services.NewGameApprovalService(
	// 	approvalRepo,
	// 	gameRepo,
	// 	rulesRepo,
	// 	prizeRepo,
	// )
	// logger.Println("Approval service initialized")

	// Initialize schedule service
	scheduleService := services.NewGameScheduleService(
		scheduleRepo,
		gameRepo,
	)
	logger.Println("Schedule service initialized")

	// Log draw service configuration
	logger.Printf("Draw Service Config: Host=%s, Port=%d, Timeout=%v, MaxRetries=%d",
		cfg.DrawService.Host, cfg.DrawService.Port, cfg.DrawService.Timeout, cfg.DrawService.MaxRetries)

	// Initialize Draw Service client
	logger.Println("Initializing Draw Service client...")
	var drawClient clients.DrawServiceClient
	if cfg.DrawService.Host != "" && cfg.DrawService.Port > 0 {
		drawServiceAddr := fmt.Sprintf("%s:%d", cfg.DrawService.Host, cfg.DrawService.Port)
		logger.Printf("Connecting to Draw Service at: %s", drawServiceAddr)
		var err error
		drawClient, err = clients.NewDrawServiceClient(drawServiceAddr)
		if err != nil {
			logger.Printf("WARNING: Failed to initialize draw service client: %v", err)
			logger.Printf("Scheduler will be disabled")
		} else {
			logger.Printf("Draw service client initialized successfully: %s", drawServiceAddr)
		}
	} else {
		logger.Printf("Draw Service not configured (Host=%s, Port=%d)", cfg.DrawService.Host, cfg.DrawService.Port)
	}

	// Log ticket service configuration
	logger.Printf("Ticket Service Config: Host=%s, Port=%d, Timeout=%v, MaxRetries=%d",
		cfg.TicketService.Host, cfg.TicketService.Port, cfg.TicketService.Timeout, cfg.TicketService.MaxRetries)

	// Initialize Ticket Service client
	logger.Println("Initializing Ticket Service client...")
	var ticketClient clients.TicketServiceClient
	if cfg.TicketService.Host != "" && cfg.TicketService.Port > 0 {
		ticketServiceAddr := fmt.Sprintf("%s:%d", cfg.TicketService.Host, cfg.TicketService.Port)
		logger.Printf("Connecting to Ticket Service at: %s", ticketServiceAddr)
		var err error
		ticketClient, err = clients.NewTicketServiceClient(ticketServiceAddr)
		if err != nil {
			logger.Printf("WARNING: Failed to initialize ticket service client: %v", err)
			logger.Printf("Ticket statistics will not be available")
		} else {
			logger.Printf("Ticket service client initialized successfully: %s", ticketServiceAddr)
		}
	} else {
		logger.Printf("Ticket Service not configured (Host=%s, Port=%d)", cfg.TicketService.Host, cfg.TicketService.Port)
	}

	// Log notification service configuration
	logger.Printf("Notification Service Config: Host=%s, Port=%d, Timeout=%v, MaxRetries=%d",
		cfg.NotificationService.Host, cfg.NotificationService.Port, cfg.NotificationService.Timeout, cfg.NotificationService.MaxRetries)

	// Initialize Notification Service client
	logger.Println("Initializing Notification Service client...")
	var notificationClient clients.NotificationServiceClient
	if cfg.NotificationService.Host != "" && cfg.NotificationService.Port > 0 {
		notificationServiceAddr := fmt.Sprintf("%s:%d", cfg.NotificationService.Host, cfg.NotificationService.Port)
		logger.Printf("Connecting to Notification Service at: %s", notificationServiceAddr)
		var err error
		notificationClient, err = clients.NewNotificationServiceClient(notificationServiceAddr)
		if err != nil {
			logger.Printf("WARNING: Failed to initialize notification service client: %v", err)
			logger.Printf("Scheduled notifications will not be sent")
		} else {
			logger.Printf("Notification service client initialized successfully: %s", notificationServiceAddr)
		}
	} else {
		logger.Printf("Notification Service not configured (Host=%s, Port=%d)", cfg.NotificationService.Host, cfg.NotificationService.Port)
	}

	// Log admin service configuration
	logger.Printf("Admin Service Config: Host=%s, Port=%d, Timeout=%v, MaxRetries=%d",
		cfg.AdminService.Host, cfg.AdminService.Port, cfg.AdminService.Timeout, cfg.AdminService.MaxRetries)

	// Initialize Admin Service client
	logger.Println("Initializing Admin Service client...")
	var adminClient clients.AdminServiceClient
	if cfg.AdminService.Host != "" && cfg.AdminService.Port > 0 {
		adminServiceAddr := fmt.Sprintf("%s:%d", cfg.AdminService.Host, cfg.AdminService.Port)
		logger.Printf("Connecting to Admin Service at: %s", adminServiceAddr)
		var err error
		adminClient, err = clients.NewAdminServiceClient(adminServiceAddr)
		if err != nil {
			logger.Printf("WARNING: Failed to initialize admin service client: %v", err)
			logger.Printf("Will use fallback emails for notifications: %v", cfg.Notification.FallbackEmails)
		} else {
			logger.Printf("Admin service client initialized successfully: %s", adminServiceAddr)
		}
	} else {
		logger.Printf("Admin Service not configured (Host=%s, Port=%d)", cfg.AdminService.Host, cfg.AdminService.Port)
	}

	// Log scheduler configuration
	logger.Printf("Scheduler Config: Enabled=%v, Interval=%v, WindowMinutes=%d, Timezone=%s",
		cfg.Scheduler.Enabled, cfg.Scheduler.Interval, cfg.Scheduler.WindowMinutes, cfg.Scheduler.Timezone)

	// Initialize scheduler service if enabled and draw client available
	logger.Println("Checking scheduler prerequisites...")
	var schedulerService *services.SchedulerService
	if cfg.Scheduler.Enabled && drawClient != nil {
		logger.Println("Initializing scheduler service...")

		var err error
		schedulerService, err = services.NewSchedulerService(
			cfg.Scheduler,
			scheduleRepo,
			gameRepo,
			drawClient,
			ticketClient,                    // Pass ticket client (can be nil)
			eventBus,                        // Pass shared event bus
			notificationClient,              // Pass notification client (can be nil)
			adminClient,                     // Pass admin client (can be nil)
			cfg.Notification.FallbackEmails, // Pass fallback emails
			logger,
		)
		if err != nil {
			log.Fatalf("Failed to initialize scheduler service: %v", err)
		}
		logger.Println("Scheduler service initialized successfully")

		// Start scheduler
		logger.Println("Starting scheduler...")
		if err := schedulerService.Start(ctx); err != nil {
			log.Fatalf("Failed to start scheduler: %v", err)
		}
		logger.Println("Scheduler started successfully")
	} else if cfg.Scheduler.Enabled && drawClient == nil {
		logger.Println("WARNING: Scheduler is enabled but draw service client is unavailable")
	} else if !cfg.Scheduler.Enabled {
		logger.Println("Scheduler is disabled")
	}

	// Other services commented out until proto is updated
	/*
		rulesService := services.NewGameRulesService(
			rulesRepo,
			gameRepo,
		)

		prizeService := services.NewPrizeStructureService(
			prizeRepo,
			gameRepo,
		)
	*/

	// Create gRPC server with tracing
	logger.Println("Setting up gRPC server...")
	var grpcOpts []grpc.ServerOption
	if cfg.Tracing.Enabled {
		logger.Println("Adding OpenTelemetry tracing to gRPC server")
		grpcOpts = append(grpcOpts,
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
		)
	}
	grpcServer := grpc.NewServer(grpcOpts...)
	logger.Println("gRPC server created")

	// Register game service with scheduling support (approval workflow removed)
	logger.Println("Registering game service handler...")
	gameHandler := grpcserver.NewGameServerMinimal(gameService, scheduleService)
	pb.RegisterGameServiceServer(grpcServer, gameHandler)
	logger.Println("Game service registered (approval workflow disabled)")

	// Register health check service
	logger.Println("Registering health check service...")
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	logger.Println("Health check service registered")

	// Register reflection for development
	logger.Println("Registering gRPC reflection...")
	reflection.Register(grpcServer)
	logger.Println("gRPC reflection registered")

	// Start server
	logger.Printf("Starting to listen on port %d...", cfg.Server.Port)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.Port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	logger.Printf("Successfully listening on port %d", cfg.Server.Port)

	logger.Printf("Game service starting on port %d", cfg.Server.Port)

	// Start gRPC server in goroutine
	go func() {
		logger.Println("gRPC server goroutine started, calling Serve()...")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()
	logger.Println("gRPC server goroutine launched, service is ready to accept connections")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Println("Shutting down server...")

	// Stop scheduler if running
	if schedulerService != nil {
		schedulerService.Stop()
	}

	// Close draw service client if initialized
	if drawClient != nil {
		if err := drawClient.Close(); err != nil {
			logger.Printf("Error closing draw service client: %v", err)
		}
	}

	// Close ticket service client if initialized
	if ticketClient != nil {
		if err := ticketClient.Close(); err != nil {
			logger.Printf("Error closing ticket service client: %v", err)
		}
	}

	// Close notification service client if initialized
	if notificationClient != nil {
		if err := notificationClient.Close(); err != nil {
			logger.Printf("Error closing notification service client: %v", err)
		}
	}

	// Close admin service client if initialized
	if adminClient != nil {
		if err := adminClient.Close(); err != nil {
			logger.Printf("Error closing admin service client: %v", err)
		}
	}

	grpcServer.GracefulStop()
	logger.Println("Server stopped")
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

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/randco/randco-microservices/proto/notification/v1"
	"github.com/randco/randco-microservices/services/service-notification/internal/cache"
	"github.com/randco/randco-microservices/services/service-notification/internal/clients"
	"github.com/randco/randco-microservices/services/service-notification/internal/config"
	"github.com/randco/randco-microservices/services/service-notification/internal/database"
	grpcserver "github.com/randco/randco-microservices/services/service-notification/internal/grpc/server"
	"github.com/randco/randco-microservices/services/service-notification/internal/kafka"
	"github.com/randco/randco-microservices/services/service-notification/internal/metrics"
	"github.com/randco/randco-microservices/services/service-notification/internal/providers"
	"github.com/randco/randco-microservices/services/service-notification/internal/providers/push"
	"github.com/randco/randco-microservices/services/service-notification/internal/queue"
	"github.com/randco/randco-microservices/services/service-notification/internal/ratelimit"
	repositories "github.com/randco/randco-microservices/services/service-notification/internal/repositories"
	"github.com/randco/randco-microservices/services/service-notification/internal/services"
	"github.com/randco/randco-microservices/services/service-notification/internal/templates"
	"github.com/randco/randco-microservices/services/service-notification/internal/tracing"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/randco-microservices/shared/events"
	"github.com/randco/randco-microservices/shared/idempotency"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
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
	fmt.Printf("Starting service-notification Service\n")
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
		ServiceName: "service-notification",
		LogFile:     cfg.Logging.LogFile,
	})
	defer func() {
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing logger: %v\n", err)
		}
	}()

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

	rawDB, err := database.New(cfg)
	if err != nil {
		logger.Fatal("Failed to connect to database", "error", err)
	}
	logger.Info("Connected to database successfully", "max_conns", cfg.Database.MaxConnections)

	// Initialize repositories with traced database
	notificationRepo := repositories.NewNotificationRepository(rawDB)
	deviceTokenRepo := repositories.NewDeviceTokenRepository(rawDB)
	retailerNotifRepo := repositories.NewRetailerNotificationRepository(rawDB)

	opts, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		logger.Fatal("Failed to parse Redis URL", "error", err)
	}

	opts.PoolSize = cfg.Redis.PoolSize
	opts.MinIdleConns = cfg.Redis.MinIdleConn
	opts.ReadTimeout = time.Duration(cfg.Redis.ReadTimeout) * time.Millisecond
	opts.WriteTimeout = time.Duration(cfg.Redis.WriteTimeout) * time.Millisecond
	opts.MaxRetries = cfg.Redis.RetryCount
	opts.MinRetryBackoff = time.Duration(cfg.Redis.RetryDelay) * time.Millisecond
	opts.MaxRetryBackoff = time.Duration(cfg.Redis.RetryDelay) * time.Millisecond * 3

	// Initialize Redis client
	redisClient := redis.NewClient(opts)
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Error("Error closing Redis connection", "error", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", "error", err)
	}
	logger.Info("Connected to Redis successfully",
		"address", opts.Addr,
		"pool_size", cfg.Redis.PoolSize,
		"min_idle_conns", cfg.Redis.MinIdleConn,
	)

	// Initialize cache, rate limiter, queue (for API backward compatibility), and idempotency managers
	cacheManager := cache.NewRedisCacheManager(redisClient)
	rateLimiterConfig := ratelimit.Config{
		EmailRatePerHour: cfg.RateLimit.EmailRatePerHour,
		SMSRatePerMinute: cfg.RateLimit.SMSRatePerMinute,
	}
	rateLimiter := ratelimit.NewRateLimiter(rateLimiterConfig, redisClient)
	queueManager := queue.NewRedisQueueManager(redisClient) // Keep for API backward compatibility
	idempotencyStore := idempotency.NewStore(redisClient)

	// Health check Redis services
	if err := cacheManager.Health(ctx); err != nil {
		logger.Error("Cache manager health check failed", "error", err)
	}
	if err := queueManager.Health(ctx); err != nil {
		logger.Error("Queue manager health check failed", "error", err)
	}
	logger.Info("Rate limiter initialized successfully",
		"email_rate_per_hour", cfg.RateLimit.EmailRatePerHour,
		"sms_rate_per_minute", cfg.RateLimit.SMSRatePerMinute)

	// Initialize provider manager with distributed rate limiter
	providerManager, err := providers.NewProviderManager(&cfg.Providers, cfg.RateLimit, redisClient, logger)
	if err != nil {
		logger.Fatal("Failed to initialize provider manager", "error", err)
	}
	defer providerManager.Shutdown()
	logger.Info("Provider manager initialized successfully",
		"email_providers", providerManager.ListEmailProviders(),
		"sms_providers", providerManager.ListSMSProviders())

	// Initialize metrics
	metricsInstance := metrics.NewMetricsInstance(cfg.Metrics.Enabled, cfg.Server.Mode)
	if cfg.Metrics.Enabled {
		logger.Info("Metrics initialized successfully", "environment", cfg.Server.Mode, "port", cfg.Metrics.Port)
	} else {
		logger.Info("Metrics disabled")
	}

	// Initialize Firebase provider for push notifications
	var firebaseProvider *push.FirebaseProvider
	var pushNotificationService services.PushNotificationService
	if cfg.Push.Firebase.Enabled {
		var err error
		firebaseProvider, err = push.NewFirebaseProvider(cfg.Push.Firebase.CredentialsPath, logger)
		if err != nil {
			logger.Error("Failed to initialize Firebase provider", "error", err)

			// Check if we're in production - fail fast if Firebase is required
			if cfg.Server.Mode == "production" || cfg.Tracing.Environment == "production" {
				logger.Fatal("Firebase is required in production but failed to initialize", "error", err)
			}

			// In development, warn but continue
			logger.Warn("Continuing without push notifications - Firebase initialization failed")
			logger.Warn("Push notifications will NOT work until Firebase is properly configured")
			pushNotificationService = nil
		} else {
			logger.Info("Firebase provider initialized successfully")
			defer func() {
				if err := firebaseProvider.Close(); err != nil {
					logger.Error("Error closing Firebase provider", "error", err)
				}
			}()

			// Initialize push notification service
			pushNotificationService = services.NewPushNotificationService(
				firebaseProvider,
				deviceTokenRepo,
				retailerNotifRepo,
				logger,
			)
			logger.Info("Push notification service initialized successfully")
		}
	} else {
		logger.Warn("Firebase push notifications are disabled in configuration")
		logger.Warn("To enable push notifications, set PUSH_FIREBASE_ENABLED=true and provide credentials")
	}

	// Initialize services
	notificationService := services.NewNotificationService(notificationRepo, logger)
	retailerNotificationService := services.NewRetailerNotificationService(deviceTokenRepo, retailerNotifRepo, logger)

	// Initialize template service
	templateService, err := templates.NewNotificationTemplateService("./internal/templates/public")
	if err != nil {
		logger.Error("Failed to initialize template service", "error", err)
		templateService = nil
	}

	// Initialize send notification service
	sendNotificationService := services.NewSendNotificationService(
		notificationService,
		templateService,
		providerManager,
		rateLimiter,
		queueManager,
		idempotencyStore,
		metricsInstance,
	)

	// Initialize Kafka event bus with consumer group for proper offset management
	eventBus, err := events.NewKafkaEventBusWithGroup(cfg.Kafka.Brokers, "notification-service-eventbus")
	if err != nil {
		logger.Error("Failed to initialize Kafka event bus", "error", err)
	} else {
		logger.Info("Kafka event bus initialized successfully",
			"brokers", cfg.Kafka.Brokers,
			"consumer_group", "notification-service-eventbus")
		defer func() {
			if err := eventBus.Close(); err != nil {
				logger.Error("Error closing Kafka event bus", "error", err)
			}
		}()
	}

	var kafkaConsumer *kafka.KafkaConsumer
	var gameEventConsumer *kafka.GameEventConsumer
	if eventBus != nil {
		// Start notification requests consumer
		kafkaConsumer = kafka.NewKafkaConsumer(cfg.Kafka, eventBus, queueManager, idempotencyStore, logger)

		kafkaCtx := context.Background()
		if err := kafkaConsumer.Start(kafkaCtx); err != nil {
			logger.Error("Failed to start Kafka consumer", "error", err)
		} else {
			logger.Info("Kafka consumer started successfully")
			defer func() {
				stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := kafkaConsumer.Stop(stopCtx); err != nil {
					logger.Error("Error stopping Kafka consumer", "error", err)
				}
			}()
		}

		// Initialize admin client for dynamic email fetching
		adminServiceAddr := fmt.Sprintf("%s:%s", cfg.Clients.AdminManagementHost, cfg.Clients.AdminManagementPort)
		adminClient, err := clients.NewAdminClient(adminServiceAddr)
		if err != nil {
			logger.Warn("Failed to initialize admin client, game notifications will use fallback recipients",
				"error", err,
				"admin_service_addr", adminServiceAddr,
			)
			adminClient = nil
		} else {
			logger.Info("Admin client initialized successfully", "address", adminServiceAddr)
			defer func() {
				if err := adminClient.Close(); err != nil {
					logger.Error("Error closing admin client", "error", err)
				}
			}()
		}

		// Start game events consumer
		gameEventRecipients := cfg.Notification.GameEndRecipients
		if len(gameEventRecipients) == 0 {
			// Default recipients if not configured
			gameEventRecipients = []string{"paulakabah@gmail.com", "jeffrey@bedriften.xyz"}
			logger.Warn("No game end notification fallback recipients configured, using default", "default", gameEventRecipients)
		}

		gameEventConsumer = kafka.NewGameEventConsumer(cfg.Kafka, eventBus, sendNotificationService, pushNotificationService, deviceTokenRepo, logger, adminClient, idempotencyStore, gameEventRecipients)
		if err := gameEventConsumer.Start(kafkaCtx); err != nil {
			logger.Error("Failed to start game event consumer", "error", err)
		} else {
			logger.Info("Game event consumer started successfully", "fallback_recipients", gameEventRecipients)
			defer func() {
				stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := gameEventConsumer.Stop(stopCtx); err != nil {
					logger.Error("Error stopping game event consumer", "error", err)
				}
			}()
		}

		// Start wallet events consumer for push notifications
		if pushNotificationService != nil {
			walletEventConsumer := kafka.NewWalletEventConsumer(eventBus, pushNotificationService, logger)
			if err := walletEventConsumer.Start(kafkaCtx); err != nil {
				logger.Error("Failed to start wallet event consumer", "error", err)
			} else {
				logger.Info("Wallet event consumer started successfully")
				defer func() {
					stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if err := walletEventConsumer.Stop(stopCtx); err != nil {
						logger.Error("Error stopping wallet event consumer", "error", err)
					}
				}()
			}
		} else {
			logger.Warn("Wallet event consumer not started - push notification service not available")
		}
	}

	// Initialize queue worker to process rate-limited notifications
	workerConfig := queue.WorkerConfig{
		WorkerID:       fmt.Sprintf("notification-worker-%s", cfg.Server.Mode),
		PollInterval:   5 * time.Second,  // Check queue every 5 seconds
		MaxConcurrency: 10,               // Process up to 10 items concurrently
		RetryBackoff:   2 * time.Second,  // Retry delay between attempts
		MaxBackoff:     30 * time.Second, // Maximum retry delay
		BatchSize:      100,              // Not used in current implementation
		MaxItemAge:     24 * time.Hour,   // Skip items older than 24 hours
	}
	worker := queue.NewQueueWorker(queueManager, sendNotificationService, logger, workerConfig)

	workerCtx := context.Background()
	if err := worker.Start(workerCtx); err != nil {
		logger.Error("Failed to start queue worker", "error", err)
	} else {
		logger.Info("Queue worker started successfully",
			"worker_id", workerConfig.WorkerID,
			"poll_interval", workerConfig.PollInterval,
			"max_concurrency", workerConfig.MaxConcurrency,
		)
		defer func() {
			stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := worker.Stop(stopCtx); err != nil {
				logger.Error("Error stopping queue worker", "error", err)
			}
		}()
	}

	// Initialize gRPC server with tracing
	var grpcOpts []grpc.ServerOption
	if cfg.Tracing.Enabled {
		grpcOpts = append(grpcOpts,
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
			// grpc.UnaryInterceptor(grpcTracingMiddleware.UnaryServerInterceptor()),
			// grpc.StreamInterceptor(grpcTracingMiddleware.StreamServerInterceptor()),
		)
	}
	grpcServer := grpc.NewServer(grpcOpts...)

	// Register notification service
	notificationHandler := grpcserver.NewNotificationServer(sendNotificationService, notificationService, retailerNotificationService, queueManager, metricsInstance, idempotencyStore)
	pb.RegisterNotificationServiceServer(grpcServer, notificationHandler)

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Enable reflection for grpcurl and other tools
	reflection.Register(grpcServer)

	// Start listening
	listener, err := net.Listen("tcp", ":"+cfg.Server.Port)
	if err != nil {
		logger.Fatal("Failed to listen on port", "port", cfg.Server.Port, "error", err)
	}

	logger.Info("Notification Service starting", "port", cfg.Server.Port)
	logger.Info("Server configuration", "mode", cfg.Server.Mode, "log_level", cfg.Logging.Level)

	// Start gRPC server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			serverErr <- err
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		logger.Fatal("Failed to start gRPC server", "error", err)
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", "signal", sig)

		// Graceful shutdown with timeout
		logger.Info("Shutting down gracefully...")

		// Stop accepting new requests
		grpcServer.GracefulStop()
		logger.Info("gRPC server stopped")
		logger.Info("All resources cleaned up successfully")
	}
}

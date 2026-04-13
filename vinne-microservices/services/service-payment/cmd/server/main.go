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
	pb "github.com/randco/randco-microservices/proto/payment/v1"
	walletv1 "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/service-payment/internal/config"
	"github.com/randco/service-payment/internal/events"
	"github.com/randco/service-payment/internal/grpc/server"
	"github.com/randco/service-payment/internal/providers"
	"github.com/randco/service-payment/internal/providers/orange"
	"github.com/randco/service-payment/internal/repositories"
	"github.com/randco/service-payment/internal/saga"
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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Version information (injected at build time)
var (
	Version        = "dev"
	GitBranch      = "unknown"
	GitCommit      = "unknown"
	GitCommitCount = "0"
	BuildTime      = "unknown"
)

// noOpPublisher is a temporary no-op event publisher
// TODO: Replace with real Kafka publisher when event streaming is needed
type noOpPublisher struct{}

func (n *noOpPublisher) PublishTransactionEvent(ctx context.Context, event *events.TransactionEvent) error {
	// No-op: events not published until Kafka is integrated
	return nil
}

func (n *noOpPublisher) PublishDepositEvent(ctx context.Context, event *events.DepositEvent) error {
	// No-op: events not published until Kafka is integrated
	return nil
}

func (n *noOpPublisher) PublishWithdrawalEvent(ctx context.Context, event *events.WithdrawalEvent) error {
	// No-op: events not published until Kafka is integrated
	return nil
}

func (n *noOpPublisher) PublishSagaEvent(ctx context.Context, event *events.SagaEvent) error {
	// No-op: events not published until Kafka is integrated
	return nil
}

func (n *noOpPublisher) PublishProviderEvent(ctx context.Context, event *events.ProviderEvent) error {
	// No-op: events not published until Kafka is integrated
	return nil
}

func (n *noOpPublisher) Close() error {
	// No-op: nothing to close
	return nil
}

func main() {
	// Load configuration
	// Log version information
	fmt.Printf("Starting service-payment Service\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Git Branch: %s, Commit: %s (#%s)\n", GitBranch, GitCommit, GitCommitCount)
	fmt.Printf("Build Time: %s\n", BuildTime)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger := logger.NewLogger(logger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		ServiceName: "payment-service",
		LogFile:     "logs/payment-service.log",
	})
	defer func() {
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing logger: %v\n", err)
		}
	}()

	// Log configuration (with masked secrets)
	logger.Info("Configuration loaded successfully",
		"server_port", cfg.Server.Port,
		"database_host", maskConnectionString(cfg.Database.URL),
		"redis_host", cfg.Redis.Host,
		"redis_port", cfg.Redis.Port,
		"kafka_enabled", cfg.Kafka.Enabled,
		"kafka_brokers", strings.Join(cfg.Kafka.Brokers, ","),
		"tracing_enabled", cfg.Tracing.Enabled,
		"logging_level", cfg.Logging.Level)

	logger.Info("Payment configuration",
		"default_currency", cfg.Payment.DefaultCurrency,
		"max_amount", cfg.Payment.MaxAmount,
		"min_amount", cfg.Payment.MinAmount,
		"timeout", cfg.Payment.Timeout,
		"retry_count", cfg.Payment.RetryCount,
		"test_mode", cfg.Payment.TestMode,
		"webhook_secret_set", cfg.Payment.WebhookSecret != "")

	logger.Info("Orange provider configuration",
		"enabled", cfg.Providers.Orange.Enabled,
		"base_url", cfg.Providers.Orange.BaseURL,
		"environment", cfg.Providers.Orange.Environment,
		"timeout", cfg.Providers.Orange.Timeout,
		"retry_attempts", cfg.Providers.Orange.RetryAttempts,
		"callback_url", cfg.Providers.Orange.CallbackURL,
		"callback_secret_set", cfg.Providers.Orange.CallbackSecret != "",
		"secret_key_set", cfg.Providers.Orange.SecretKey != "",
		"secret_token_set", cfg.Providers.Orange.SecretToken != "")

	logger.Info("Bank provider configuration",
		"enabled", cfg.Providers.Banks.Enabled,
		"manual_verification", cfg.Providers.Banks.ManualVerification)

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

	// Initialize database
	db, err := gorm.Open(postgres.Open(cfg.Database.URL), &gorm.Config{})
	if err != nil {
		logger.Fatal("Failed to connect to database", "error", err)
	}

	// Configure connection pool and convert to sqlx
	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatal("Failed to get database instance", "error", err)
	}
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	// Convert to sqlx DB for repositories
	sqlxDB := sqlx.NewDb(sqlDB, "postgres")
	logger.Info("Database connection established")

	// Database migrations are handled by Goose via CI/CD pipeline

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", "error", err)
	}
	logger.Info("Redis connection established")

	// Connect to Wallet Service for deposit/withdrawal sagas
	walletAddr := fmt.Sprintf("%s:%d", cfg.Services.Wallet.Host, cfg.Services.Wallet.Port)
	walletConn, err := grpc.NewClient(walletAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal("Failed to connect to Wallet Service", "address", walletAddr, "error", err)
	}
	defer func() {
		if err := walletConn.Close(); err != nil {
			logger.Error("Error closing Wallet Service connection", "error", err)
		}
	}()
	walletClient := walletv1.NewWalletServiceClient(walletConn)
	logger.Info("Wallet Service client connected", "address", walletAddr)

	// Initialize repositories
	transactionRepo := repositories.NewTransactionRepository(sqlxDB)
	sagaRepo := repositories.NewSagaRepository(sqlxDB)
	idempotencyRepo := repositories.NewIdempotencyRepository(sqlxDB)
	webhookEventRepo := repositories.NewWebhookEventRepository(sqlxDB)
	logger.Info("Repositories initialized")

	// Initialize event publisher (Kafka)
	var eventPublisher events.Publisher
	if cfg.Kafka.Enabled {
		kafkaConfig := &events.PublisherConfig{
			Brokers: cfg.Kafka.Brokers,
			Topic:   cfg.Kafka.Topic,
		}
		eventPublisher = events.NewKafkaPublisher(kafkaConfig)
		logger.Info("Kafka event publisher initialized",
			"brokers", cfg.Kafka.Brokers,
			"topic", cfg.Kafka.Topic)
		defer func() {
			if err := eventPublisher.Close(); err != nil {
				logger.Error("Error closing event publisher", "error", err)
			}
		}()
	} else {
		logger.Info("Kafka disabled, using no-op publisher")
		eventPublisher = &noOpPublisher{}
	}

	// Initialize providers
	providerFactory := providers.NewProviderFactory()

	// Register Orange Money provider (non-fatal if disabled)
	orangeProvider, err := orange.NewProvider(&cfg.Providers.Orange, logger)
	if err != nil {
		logger.Warn("Orange provider not initialized", "error", err)
		// Continue without Orange provider - it's not critical for service startup
	} else {
		if err := providerFactory.RegisterProvider("orange", orangeProvider); err != nil {
			logger.Error("Failed to register Orange provider", "error", err)
		} else {
			logger.Info("Orange provider registered")
		}
		if err := providerFactory.RegisterProvider("mtn", orangeProvider); err != nil {
			logger.Error("Failed to register MTN provider", "error", err)
		} else {
			logger.Info("MTN provider registered")
		}
		if err := providerFactory.RegisterProvider("telecel", orangeProvider); err != nil {
			logger.Error("Failed to register Telecel provider", "error", err)
		} else {
			logger.Info("Telecel provider registered")
		}
		if err := providerFactory.RegisterProvider("airteltigo", orangeProvider); err != nil {
			logger.Error("Failed to register AirtelTigo provider", "error", err)
		} else {
			logger.Info("AirtelTigo provider registered")
		}
	}

	// Log provider registration summary
	logger.Info("Provider initialization complete")

	// Initialize saga orchestrator
	orchestrator := saga.NewOrchestrator(sagaRepo, eventPublisher)
	logger.Info("Saga orchestrator initialized")

	// Initialize deposit, player deposit, and withdrawal sagas
	depositSaga := saga.NewDepositSaga(orchestrator, orangeProvider, walletClient, transactionRepo)
	playerDepositSaga := saga.NewPlayerDepositSaga(orchestrator, orangeProvider, walletClient, transactionRepo)
	withdrawalSaga := saga.NewWithdrawalSaga(orchestrator, orangeProvider, walletClient, transactionRepo)
	logger.Info("Payment sagas initialized (deposit, player-deposit, withdrawal)")

	logger.Info("All services initialized successfully")

	// Create gRPC server with tracing
	var grpcOpts []grpc.ServerOption
	if cfg.Tracing.Enabled {
		grpcOpts = append(grpcOpts,
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
		)
	}
	grpcServer := grpc.NewServer(grpcOpts...)

	// Create and register payment service gRPC handler
	paymentHandler := server.NewPaymentHandler(
		transactionRepo,
		idempotencyRepo,
		webhookEventRepo,
		providerFactory,
		depositSaga,
		playerDepositSaga,
		withdrawalSaga,
		cfg.Providers.Orange.CallbackSecret,
		logger,
	)
	pb.RegisterPaymentServiceServer(grpcServer, paymentHandler)
	logger.Info("Payment service gRPC handler registered")

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection for development
	reflection.Register(grpcServer)

	// Start server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.Port))
	if err != nil {
		logger.Fatal("Failed to listen", "error", err)
	}

	logger.Info("Payment service starting", "port", cfg.Server.Port)

	// Start gRPC server in goroutine
	go func() {
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

	// Services are now used by the gRPC handlers
}

// maskConnectionString masks sensitive parts of connection strings for logging
// Example: postgresql://user:pass@host:port/db -> postgresql://user:****@host:port/db
func maskConnectionString(connStr string) string {
	if connStr == "" {
		return ""
	}

	// Handle postgresql:// URLs
	if strings.Contains(connStr, "://") && strings.Contains(connStr, "@") {
		parts := strings.SplitN(connStr, "://", 2)
		if len(parts) != 2 {
			return "****"
		}

		protocol := parts[0]
		rest := parts[1]

		// Split on @ to separate credentials from host
		credHost := strings.SplitN(rest, "@", 2)
		if len(credHost) != 2 {
			return protocol + "://****"
		}

		// Split credentials on :
		userPass := strings.SplitN(credHost[0], ":", 2)
		if len(userPass) == 2 {
			// Mask password
			return fmt.Sprintf("%s://%s:****@%s", protocol, userPass[0], credHost[1])
		}

		return fmt.Sprintf("%s://****@%s", protocol, credHost[1])
	}

	return "****"
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

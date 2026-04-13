package config

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestConfigLoad_LocalEnvironment(t *testing.T) {
	// Test loading config with local environment (should use defaults)
	_ = os.Setenv("ENVIRONMENT", "local")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check that defaults are loaded
	assert.Equal(t, 50053, cfg.Server.Port)
	assert.Contains(t, cfg.Database.URL, "localhost:5441")
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, "6388", cfg.Redis.Port)
}

func TestConfigLoad_DevelopmentEnvironment(t *testing.T) {
	// Test loading config with development environment (should NOT use localhost defaults)
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set required environment variables
	_ = os.Setenv("DATABASE_URL", "postgresql://test:test@db.example.com:5432/testdb?sslmode=disable")
	_ = os.Setenv("REDIS_HOST", "redis.example.com")
	_ = os.Setenv("REDIS_PORT", "6379")
	_ = os.Setenv("SERVER_PORT", "50053")
	defer func() {
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("REDIS_HOST")
		_ = os.Unsetenv("REDIS_PORT")
		_ = os.Unsetenv("SERVER_PORT")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check that environment variables are used, not localhost defaults
	assert.Equal(t, "postgresql://test:test@db.example.com:5432/testdb?sslmode=disable", cfg.Database.URL)
	assert.Equal(t, "redis.example.com", cfg.Redis.Host)
	assert.Equal(t, "6379", cfg.Redis.Port)
	assert.Equal(t, 50053, cfg.Server.Port)
}

func TestConfigLoad_WithTestcontainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test with testcontainers")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("game_service"),
		postgres.WithUsername("game"),
		postgres.WithPassword("game123"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate postgres container: %s", err)
		}
	}()

	// Get PostgreSQL connection string
	pgHost, err := postgresContainer.Host(ctx)
	require.NoError(t, err)
	pgPort, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	// Start Redis container
	redisContainer, err := redis.Run(ctx,
		"redis:7.4-alpine",
		redis.WithSnapshotting(10, 1),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	require.NoError(t, err)
	defer func() {
		if err := redisContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate redis container: %s", err)
		}
	}()

	// Get Redis connection string
	redisHost, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	// Set environment variables for development environment
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	databaseURL := fmt.Sprintf("postgresql://game:game123@%s:%s/game_service?sslmode=disable",
		pgHost, pgPort.Port())

	_ = os.Setenv("DATABASE_URL", databaseURL)
	_ = os.Setenv("REDIS_HOST", redisHost)
	_ = os.Setenv("REDIS_PORT", redisPort.Port())
	_ = os.Setenv("SERVER_PORT", "50053")
	defer func() {
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("REDIS_HOST")
		_ = os.Unsetenv("REDIS_PORT")
		_ = os.Unsetenv("SERVER_PORT")
	}()

	// Load configuration
	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify configuration is loaded correctly
	assert.Equal(t, databaseURL, cfg.Database.URL)
	assert.Equal(t, redisHost, cfg.Redis.Host)
	assert.Equal(t, redisPort.Port(), cfg.Redis.Port)
	assert.Equal(t, 50053, cfg.Server.Port)

	// Test that we can actually connect to the database with the config
	// (This would normally be done in the service initialization)
	t.Logf("Database URL: %s", cfg.Database.URL)
	t.Logf("Redis Addr: %s:%s", cfg.Redis.Host, cfg.Redis.Port)
}

func TestConfigLoad_MissingRequiredEnvVars(t *testing.T) {
	// Test that config loads even without DATABASE_URL when not in local mode
	// but the URL will be empty
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Don't set DATABASE_URL or REDIS_URL
	_ = os.Setenv("SERVER_PORT", "50053")
	defer func() { _ = os.Unsetenv("SERVER_PORT") }()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Without DATABASE_URL set, the config should have empty database URL
	assert.Empty(t, cfg.Database.URL)
	assert.Empty(t, cfg.Redis.Host)
	assert.Empty(t, cfg.Redis.Port)
	assert.Equal(t, 50053, cfg.Server.Port)
}

func TestConfigLoad_KafkaSettings(t *testing.T) {
	// Test that Kafka settings can be configured
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set Kafka settings
	_ = os.Setenv("KAFKA_BROKERS", "kafka1.example.com:9092,kafka2.example.com:9092")
	_ = os.Setenv("KAFKA_TOPICS_GAME_EVENTS", "game.events.prod")
	_ = os.Setenv("KAFKA_TOPICS_APPROVAL_EVENTS", "approval.events.prod")

	defer func() {
		_ = os.Unsetenv("KAFKA_BROKERS")
		_ = os.Unsetenv("KAFKA_TOPICS_GAME_EVENTS")
		_ = os.Unsetenv("KAFKA_TOPICS_APPROVAL_EVENTS")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check Kafka configuration
	assert.Equal(t, []string{"kafka1.example.com:9092", "kafka2.example.com:9092"}, cfg.Kafka.Brokers)
	assert.Equal(t, "game.events.prod", cfg.Kafka.Topics.GameEvents)
	assert.Equal(t, "approval.events.prod", cfg.Kafka.Topics.ApprovalEvents)
}

func TestConfigLoad_DatabaseSettings(t *testing.T) {
	// Test that database settings can be configured
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set database settings
	_ = os.Setenv("DATABASE_MAX_OPEN_CONNS", "50")
	_ = os.Setenv("DATABASE_MAX_IDLE_CONNS", "20")
	_ = os.Setenv("DATABASE_CONN_MAX_LIFETIME", "10")

	defer func() {
		_ = os.Unsetenv("DATABASE_MAX_OPEN_CONNS")
		_ = os.Unsetenv("DATABASE_MAX_IDLE_CONNS")
		_ = os.Unsetenv("DATABASE_CONN_MAX_LIFETIME")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check database configuration
	assert.Equal(t, 50, cfg.Database.MaxOpenConns)
	assert.Equal(t, 20, cfg.Database.MaxIdleConns)
	assert.Equal(t, 10*time.Minute, cfg.Database.ConnMaxLifetime)
}

func TestConfigLoad_RedisSettings(t *testing.T) {
	// Test Redis configuration
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set Redis settings
	_ = os.Setenv("REDIS_HOST", "redis.example.com")
	_ = os.Setenv("REDIS_PORT", "6379")
	_ = os.Setenv("REDIS_PASSWORD", "secret123")
	_ = os.Setenv("REDIS_DB", "5")

	defer func() {
		_ = os.Unsetenv("REDIS_HOST")
		_ = os.Unsetenv("REDIS_PORT")
		_ = os.Unsetenv("REDIS_PASSWORD")
		_ = os.Unsetenv("REDIS_DB")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check Redis configuration
	assert.Equal(t, "redis.example.com", cfg.Redis.Host)
	assert.Equal(t, "6379", cfg.Redis.Port)
	assert.Equal(t, "secret123", cfg.Redis.Password)
	assert.Equal(t, 5, cfg.Redis.DB)
}

func TestConfigLoad_TracingSettings(t *testing.T) {
	// Test tracing configuration
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set tracing settings
	_ = os.Setenv("TRACING_ENABLED", "false")
	_ = os.Setenv("TRACING_JAEGER_ENDPOINT", "http://jaeger.example.com:4318")
	_ = os.Setenv("TRACING_SERVICE_NAME", "game-service-prod")
	_ = os.Setenv("TRACING_SERVICE_VERSION", "2.0.0")
	_ = os.Setenv("TRACING_ENVIRONMENT", "production")
	_ = os.Setenv("TRACING_SAMPLE_RATE", "0.3")

	defer func() {
		_ = os.Unsetenv("TRACING_ENABLED")
		_ = os.Unsetenv("TRACING_JAEGER_ENDPOINT")
		_ = os.Unsetenv("TRACING_SERVICE_NAME")
		_ = os.Unsetenv("TRACING_SERVICE_VERSION")
		_ = os.Unsetenv("TRACING_ENVIRONMENT")
		_ = os.Unsetenv("TRACING_SAMPLE_RATE")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check tracing configuration
	assert.False(t, cfg.Tracing.Enabled)
	assert.Equal(t, "http://jaeger.example.com:4318", cfg.Tracing.JaegerEndpoint)
	assert.Equal(t, "game-service-prod", cfg.Tracing.ServiceName)
	assert.Equal(t, "2.0.0", cfg.Tracing.ServiceVersion)
	assert.Equal(t, "production", cfg.Tracing.Environment)
	assert.Equal(t, 0.3, cfg.Tracing.SampleRate)
}

func TestConfigLoad_LoggingSettings(t *testing.T) {
	// Test logging configuration
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set logging settings
	_ = os.Setenv("LOGGING_LEVEL", "debug")
	_ = os.Setenv("LOGGING_FORMAT", "text")

	defer func() {
		_ = os.Unsetenv("LOGGING_LEVEL")
		_ = os.Unsetenv("LOGGING_FORMAT")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check logging configuration
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)
}

func TestConfigLoad_URLOverridesComponents(t *testing.T) {
	// Test that DATABASE_URL takes precedence over individual components
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set both URL and components
	_ = os.Setenv("DATABASE_URL", "postgresql://urluser:urlpass@urlhost:5433/urldb?sslmode=disable")
	_ = os.Setenv("REDIS_HOST", "redis.example.com")
	_ = os.Setenv("REDIS_PORT", "6380")
	_ = os.Setenv("DATABASE_HOST", "componenthost")
	_ = os.Setenv("DATABASE_PORT", "5432")
	_ = os.Setenv("DATABASE_NAME", "componentdb")

	defer func() {
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("REDIS_HOST")
		_ = os.Unsetenv("REDIS_PORT")
		_ = os.Unsetenv("DATABASE_HOST")
		_ = os.Unsetenv("DATABASE_PORT")
		_ = os.Unsetenv("DATABASE_NAME")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// DATABASE_URL should be used
	assert.Equal(t, "postgresql://urluser:urlpass@urlhost:5433/urldb?sslmode=disable", cfg.Database.URL)
	assert.Equal(t, "redis.example.com", cfg.Redis.Host)
	assert.Equal(t, "6380", cfg.Redis.Port)
}

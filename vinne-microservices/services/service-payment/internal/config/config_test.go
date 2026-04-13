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
	assert.Equal(t, 50061, cfg.Server.Port)
	assert.Contains(t, cfg.Database.URL, "localhost:5440")
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, "6387", cfg.Redis.Port)
}

func TestConfigLoad_DevelopmentEnvironment(t *testing.T) {
	// Test loading config with development environment (should NOT use localhost defaults)
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set required environment variables
	_ = os.Setenv("DATABASE_URL", "postgresql://test:test@db.example.com:5432/testdb?sslmode=disable")
	_ = os.Setenv("REDIS_HOST", "redis.example.com")
	_ = os.Setenv("REDIS_PORT", "6379")
	_ = os.Setenv("SERVER_PORT", "50061")
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
	assert.Equal(t, 50061, cfg.Server.Port)
}

func TestConfigLoad_WithTestcontainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test with testcontainers")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("payment_service"),
		postgres.WithUsername("payment"),
		postgres.WithPassword("payment123"),
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

	databaseURL := fmt.Sprintf("postgresql://payment:payment123@%s:%s/payment_service?sslmode=disable",
		pgHost, pgPort.Port())

	_ = os.Setenv("DATABASE_URL", databaseURL)
	_ = os.Setenv("REDIS_HOST", redisHost)
	_ = os.Setenv("REDIS_PORT", redisPort.Port())
	_ = os.Setenv("SERVER_PORT", "50061")
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
	assert.Equal(t, 50061, cfg.Server.Port)

	// Test that we can actually connect to the database with the config
	// (This would normally be done in the service initialization)
	t.Logf("Database URL: %s", cfg.Database.URL)
	t.Logf("Redis Host: %s", cfg.Redis.Host)
	t.Logf("Redis Port: %s", cfg.Redis.Port)
}

func TestConfigLoad_MissingRequiredEnvVars(t *testing.T) {
	// Test that config loads even without DATABASE_URL when not in local mode
	// but the URL will be empty
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Don't set DATABASE_URL or REDIS_URL
	_ = os.Setenv("SERVER_PORT", "50061")
	defer func() { _ = os.Unsetenv("SERVER_PORT") }()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Without DATABASE_URL set, the config should have empty database URL
	assert.Empty(t, cfg.Database.URL)
	assert.Empty(t, cfg.Redis.Host)
	assert.Empty(t, cfg.Redis.Port)
	assert.Equal(t, 50061, cfg.Server.Port)
}

func TestConfigLoad_PaymentProviderSettings(t *testing.T) {
	// Test that payment provider settings can be configured
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set Orange Extensibility Service provider settings
	// (aggregates MTN, Telecel, and AirtelTigo)
	_ = os.Setenv("PROVIDERS_ORANGE_ENABLED", "true")
	_ = os.Setenv("PROVIDERS_ORANGE_BASE_URL", "https://api.orangeextensibility.com")
	_ = os.Setenv("PROVIDERS_ORANGE_SECRET_KEY", "test-secret-key")
	_ = os.Setenv("PROVIDERS_ORANGE_SECRET_TOKEN", "test-secret-token")
	_ = os.Setenv("PROVIDERS_ORANGE_ENVIRONMENT", "production")
	_ = os.Setenv("PROVIDERS_ORANGE_TIMEOUT_SECONDS", "45s")
	_ = os.Setenv("PROVIDERS_ORANGE_MAX_RETRY", "5")
	_ = os.Setenv("PROVIDERS_ORANGE_RETRY_DELAY_SECONDS", "3s")

	defer func() {
		_ = os.Unsetenv("PROVIDERS_ORANGE_ENABLED")
		_ = os.Unsetenv("PROVIDERS_ORANGE_BASE_URL")
		_ = os.Unsetenv("PROVIDERS_ORANGE_SECRET_KEY")
		_ = os.Unsetenv("PROVIDERS_ORANGE_SECRET_TOKEN")
		_ = os.Unsetenv("PROVIDERS_ORANGE_ENVIRONMENT")
		_ = os.Unsetenv("PROVIDERS_ORANGE_TIMEOUT_SECONDS")
		_ = os.Unsetenv("PROVIDERS_ORANGE_MAX_RETRY")
		_ = os.Unsetenv("PROVIDERS_ORANGE_RETRY_DELAY_SECONDS")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check Orange provider configuration
	assert.True(t, cfg.Providers.Orange.Enabled)
	assert.Equal(t, "https://api.orangeextensibility.com", cfg.Providers.Orange.BaseURL)
	assert.Equal(t, "test-secret-key", cfg.Providers.Orange.SecretKey)
	assert.Equal(t, "test-secret-token", cfg.Providers.Orange.SecretToken)
	assert.Equal(t, "production", cfg.Providers.Orange.Environment)
	assert.Equal(t, 45*time.Second, cfg.Providers.Orange.Timeout)
	assert.Equal(t, 5, cfg.Providers.Orange.RetryAttempts)
	assert.Equal(t, 3*time.Second, cfg.Providers.Orange.RetryDelay)
}

func TestConfigLoad_PaymentSettings(t *testing.T) {
	// Test that payment-specific settings can be configured
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set payment settings
	_ = os.Setenv("PAYMENT_DEFAULT_CURRENCY", "USD")
	_ = os.Setenv("PAYMENT_MAX_AMOUNT", "500000000") // $5,000,000 in cents
	_ = os.Setenv("PAYMENT_MIN_AMOUNT", "100")       // $1.00 in cents
	_ = os.Setenv("PAYMENT_TIMEOUT_SECONDS", "600s") // 10 minutes
	_ = os.Setenv("PAYMENT_RETRY_COUNT", "5")
	_ = os.Setenv("PAYMENT_RETRY_DELAY_SECONDS", "60s")
	_ = os.Setenv("PAYMENT_TEST_MODE", "false")

	defer func() {
		_ = os.Unsetenv("PAYMENT_DEFAULT_CURRENCY")
		_ = os.Unsetenv("PAYMENT_MAX_AMOUNT")
		_ = os.Unsetenv("PAYMENT_MIN_AMOUNT")
		_ = os.Unsetenv("PAYMENT_TIMEOUT_SECONDS")
		_ = os.Unsetenv("PAYMENT_RETRY_COUNT")
		_ = os.Unsetenv("PAYMENT_RETRY_DELAY_SECONDS")
		_ = os.Unsetenv("PAYMENT_TEST_MODE")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check payment-specific configuration
	assert.Equal(t, "USD", cfg.Payment.DefaultCurrency)
	assert.Equal(t, int64(500000000), cfg.Payment.MaxAmount)
	assert.Equal(t, int64(100), cfg.Payment.MinAmount)
	assert.Equal(t, 600*time.Second, cfg.Payment.Timeout)
	assert.Equal(t, 5, cfg.Payment.RetryCount)
	assert.Equal(t, 60*time.Second, cfg.Payment.RetryDelay)
	assert.False(t, cfg.Payment.TestMode)
}

func TestConfigLoad_TracingSettings(t *testing.T) {
	// Test tracing configuration
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set tracing settings
	_ = os.Setenv("TRACING_ENABLED", "true")
	_ = os.Setenv("TRACING_JAEGER_ENDPOINT", "http://jaeger.example.com:4318")
	_ = os.Setenv("TRACING_SERVICE_NAME", "payment-service-prod")
	_ = os.Setenv("TRACING_SERVICE_VERSION", "2.0.0")
	_ = os.Setenv("TRACING_ENVIRONMENT", "production")
	_ = os.Setenv("TRACING_SAMPLE_RATE", "0.5")

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
	assert.True(t, cfg.Tracing.Enabled)
	assert.Equal(t, "http://jaeger.example.com:4318", cfg.Tracing.JaegerEndpoint)
	assert.Equal(t, "payment-service-prod", cfg.Tracing.ServiceName)
	assert.Equal(t, "2.0.0", cfg.Tracing.ServiceVersion)
	assert.Equal(t, "production", cfg.Tracing.Environment)
	assert.Equal(t, 0.5, cfg.Tracing.SampleRate)
}

func TestConfigLoad_URLOverridesComponents(t *testing.T) {
	// Test that DATABASE_URL and REDIS_URL take precedence over individual components
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

	// DATABASE_URL and REDIS_HOST/PORT should be used
	assert.Equal(t, "postgresql://urluser:urlpass@urlhost:5433/urldb?sslmode=disable", cfg.Database.URL)
	assert.Equal(t, "redis.example.com", cfg.Redis.Host)
	assert.Equal(t, "6380", cfg.Redis.Port)
}

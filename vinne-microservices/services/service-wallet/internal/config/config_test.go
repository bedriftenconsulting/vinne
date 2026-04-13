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
	assert.Equal(t, 50059, cfg.Server.Port)
	assert.Equal(t, "development", cfg.Server.Environment)
	assert.Contains(t, cfg.Database.URL, "localhost:5438")
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, "6385", cfg.Redis.Port)
}

func TestConfigLoad_DevelopmentEnvironment(t *testing.T) {
	// Test loading config with development environment (should NOT use localhost defaults)
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set required environment variables
	_ = os.Setenv("DATABASE_URL", "postgresql://test:test@db.example.com:5432/testdb?sslmode=disable")
	_ = os.Setenv("REDIS_HOST", "redis.example.com")
	_ = os.Setenv("REDIS_PORT", "6379")
	_ = os.Setenv("SERVER_PORT", "50059")
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
	assert.Equal(t, 50059, cfg.Server.Port)
}

func TestConfigLoad_WithTestcontainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test with testcontainers")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("wallet_service"),
		postgres.WithUsername("wallet"),
		postgres.WithPassword("wallet123"),
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

	databaseURL := fmt.Sprintf("postgresql://wallet:wallet123@%s:%s/wallet_service?sslmode=disable",
		pgHost, pgPort.Port())

	_ = os.Setenv("DATABASE_URL", databaseURL)
	_ = os.Setenv("REDIS_HOST", redisHost)
	_ = os.Setenv("REDIS_PORT", redisPort.Port())
	_ = os.Setenv("SERVER_PORT", "50059")
	_ = os.Setenv("SERVER_ENVIRONMENT", "development")
	defer func() {
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("REDIS_HOST")
		_ = os.Unsetenv("REDIS_PORT")
		_ = os.Unsetenv("SERVER_PORT")
		_ = os.Unsetenv("SERVER_ENVIRONMENT")
	}()

	// Load configuration
	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify configuration is loaded correctly
	assert.Equal(t, databaseURL, cfg.Database.URL)
	assert.Equal(t, redisHost, cfg.Redis.Host)
	assert.Equal(t, redisPort.Port(), cfg.Redis.Port)
	assert.Equal(t, 50059, cfg.Server.Port)
	assert.Equal(t, "development", cfg.Server.Environment)

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
	_ = os.Setenv("SERVER_PORT", "50059")
	defer func() { _ = os.Unsetenv("SERVER_PORT") }()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Without DATABASE_URL set, the config should have empty database URL
	assert.Empty(t, cfg.Database.URL)
	assert.Empty(t, cfg.Redis.Host)
	assert.Empty(t, cfg.Redis.Port)
	assert.Equal(t, 50059, cfg.Server.Port)
}

func TestConfigLoad_IndividualWalletSettings(t *testing.T) {
	// Test that individual wallet settings can be configured
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set wallet-specific settings
	_ = os.Setenv("WALLET_DEFAULT_COMMISSION_RATE", "0.25")
	_ = os.Setenv("WALLET_MAX_TRANSFER_AMOUNT", "50000.00")
	_ = os.Setenv("WALLET_MIN_TRANSFER_AMOUNT", "5.00")
	_ = os.Setenv("WALLET_TRANSACTION_TIMEOUT", "60")
	_ = os.Setenv("WALLET_LOCK_TIMEOUT", "15")

	defer func() {
		_ = os.Unsetenv("WALLET_DEFAULT_COMMISSION_RATE")
		_ = os.Unsetenv("WALLET_MAX_TRANSFER_AMOUNT")
		_ = os.Unsetenv("WALLET_MIN_TRANSFER_AMOUNT")
		_ = os.Unsetenv("WALLET_TRANSACTION_TIMEOUT")
		_ = os.Unsetenv("WALLET_LOCK_TIMEOUT")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check wallet-specific configuration
	assert.Equal(t, 0.25, cfg.Wallet.DefaultCommissionRate)
	assert.Equal(t, 50000.00, cfg.Wallet.MaxTransferAmount)
	assert.Equal(t, 5.00, cfg.Wallet.MinTransferAmount)
	assert.Equal(t, 60*time.Second, cfg.Wallet.TransactionTimeout)
	assert.Equal(t, 15*time.Second, cfg.Wallet.LockTimeout)
}

func TestConfigLoad_RedisSettings(t *testing.T) {
	// Test Redis configuration
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set Redis settings
	_ = os.Setenv("REDIS_HOST", "redis.example.com")
	_ = os.Setenv("REDIS_PORT", "6379")
	_ = os.Setenv("REDIS_PASSWORD", "secret123")
	_ = os.Setenv("REDIS_DB", "2")
	_ = os.Setenv("REDIS_POOL_SIZE", "20")
	_ = os.Setenv("REDIS_MAX_RETRIES", "5")
	_ = os.Setenv("REDIS_CACHE_TTL", "600")

	defer func() {
		_ = os.Unsetenv("REDIS_HOST")
		_ = os.Unsetenv("REDIS_PORT")
		_ = os.Unsetenv("REDIS_PASSWORD")
		_ = os.Unsetenv("REDIS_DB")
		_ = os.Unsetenv("REDIS_POOL_SIZE")
		_ = os.Unsetenv("REDIS_MAX_RETRIES")
		_ = os.Unsetenv("REDIS_CACHE_TTL")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check Redis configuration
	assert.Equal(t, "redis.example.com", cfg.Redis.Host)
	assert.Equal(t, "6379", cfg.Redis.Port)
	assert.Equal(t, "secret123", cfg.Redis.Password)
	assert.Equal(t, 2, cfg.Redis.DB)
	assert.Equal(t, 20, cfg.Redis.PoolSize)
	assert.Equal(t, 5, cfg.Redis.MaxRetries)
	assert.Equal(t, 600*time.Second, cfg.Redis.CacheTTL)
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

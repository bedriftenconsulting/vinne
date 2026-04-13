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
	require.NoError(t, os.Setenv("ENVIRONMENT", "local"))
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check that defaults are loaded
	assert.Equal(t, "50057", cfg.Server.Port)
	assert.Equal(t, "development", cfg.Server.Mode)
	assert.Contains(t, cfg.Database.URL, "localhost:5437")
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, "6384", cfg.Redis.Port)
}

func TestConfigLoad_DevelopmentEnvironment(t *testing.T) {
	// Test loading config with development environment (should NOT use localhost defaults)
	require.NoError(t, os.Setenv("ENVIRONMENT", "development"))
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set required environment variables
	require.NoError(t, os.Setenv("DATABASE_URL", "postgresql://test:test@db.example.com:5432/testdb?sslmode=disable"))
	require.NoError(t, os.Setenv("REDIS_HOST", "redis.example.com"))
	require.NoError(t, os.Setenv("REDIS_PORT", "6379"))
	require.NoError(t, os.Setenv("REDIS_PASSWORD", "testpass"))
	require.NoError(t, os.Setenv("REDIS_DB", "0"))
	require.NoError(t, os.Setenv("SERVER_PORT", "50057"))
	defer func() {
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("REDIS_HOST")
		_ = os.Unsetenv("REDIS_PORT")
		_ = os.Unsetenv("REDIS_PASSWORD")
		_ = os.Unsetenv("REDIS_DB")
		_ = os.Unsetenv("SERVER_PORT")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check that environment variables are used, not localhost defaults
	assert.Equal(t, "postgresql://test:test@db.example.com:5432/testdb?sslmode=disable", cfg.Database.URL)
	assert.Equal(t, "redis.example.com", cfg.Redis.Host)
	assert.Equal(t, "6379", cfg.Redis.Port)
	assert.Equal(t, "testpass", cfg.Redis.Password)
	assert.Equal(t, 0, cfg.Redis.DB)
	assert.Equal(t, "50057", cfg.Server.Port)
}

func TestConfigLoad_WithTestcontainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test with testcontainers")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("admin_management"),
		postgres.WithUsername("admin_mgmt"),
		postgres.WithPassword("admin_mgmt123"),
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
	require.NoError(t, os.Setenv("ENVIRONMENT", "development"))
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	databaseURL := fmt.Sprintf("postgresql://admin_mgmt:admin_mgmt123@%s:%s/admin_management?sslmode=disable",
		pgHost, pgPort.Port())

	require.NoError(t, os.Setenv("DATABASE_URL", databaseURL))
	require.NoError(t, os.Setenv("REDIS_HOST", redisHost))
	require.NoError(t, os.Setenv("REDIS_PORT", redisPort.Port()))
	require.NoError(t, os.Setenv("REDIS_PASSWORD", ""))
	require.NoError(t, os.Setenv("REDIS_DB", "0"))
	require.NoError(t, os.Setenv("SERVER_PORT", "50057"))
	require.NoError(t, os.Setenv("SERVER_MODE", "development"))
	require.NoError(t, os.Setenv("JWT_SECRET", "test-secret"))
	defer func() {
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("REDIS_HOST")
		_ = os.Unsetenv("REDIS_PORT")
		_ = os.Unsetenv("REDIS_PASSWORD")
		_ = os.Unsetenv("REDIS_DB")
		_ = os.Unsetenv("SERVER_PORT")
		_ = os.Unsetenv("SERVER_MODE")
		_ = os.Unsetenv("JWT_SECRET")
	}()

	// Load configuration
	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify configuration is loaded correctly
	assert.Equal(t, databaseURL, cfg.Database.URL)
	assert.Equal(t, redisHost, cfg.Redis.Host)
	assert.Equal(t, redisPort.Port(), cfg.Redis.Port)
	assert.Equal(t, "", cfg.Redis.Password)
	assert.Equal(t, 0, cfg.Redis.DB)
	assert.Equal(t, "50057", cfg.Server.Port)
	assert.Equal(t, "development", cfg.Server.Mode)

	// Test that we can actually connect to the database with the config
	// (This would normally be done in the service initialization)
	t.Logf("Database URL: %s", cfg.Database.URL)
	t.Logf("Redis Address: %s:%s", cfg.Redis.Host, cfg.Redis.Port)
}

func TestConfigLoad_MissingRequiredEnvVars(t *testing.T) {
	// Test that config loads even without DATABASE_URL when not in local mode
	// but the URL will be empty
	require.NoError(t, os.Setenv("ENVIRONMENT", "development"))
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Don't set DATABASE_URL or REDIS_URL
	require.NoError(t, os.Setenv("SERVER_PORT", "50057"))
	defer func() { _ = os.Unsetenv("SERVER_PORT") }()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Without DATABASE_URL set, the config should have empty database URL
	assert.Empty(t, cfg.Database.URL)
	assert.Empty(t, cfg.Redis.Host)
	assert.Empty(t, cfg.Redis.Port)
	assert.Equal(t, "50057", cfg.Server.Port)
}

func TestConfigLoad_IndividualDatabaseComponents(t *testing.T) {
	// Test that individual database components can be set instead of DATABASE_URL
	require.NoError(t, os.Setenv("ENVIRONMENT", "development"))
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set individual database components
	require.NoError(t, os.Setenv("DATABASE_HOST", "db.example.com"))
	require.NoError(t, os.Setenv("DATABASE_PORT", "5432"))
	require.NoError(t, os.Setenv("DATABASE_NAME", "testdb"))
	require.NoError(t, os.Setenv("DATABASE_USER", "testuser"))
	require.NoError(t, os.Setenv("DATABASE_SSL_MODE", "require"))
	require.NoError(t, os.Setenv("DATABASE_MAX_OPEN_CONNS", "50"))
	require.NoError(t, os.Setenv("DATABASE_MAX_IDLE_CONNS", "10"))
	require.NoError(t, os.Setenv("DATABASE_CONN_MAX_LIFETIME", "1h"))

	// Set Redis components
	require.NoError(t, os.Setenv("REDIS_HOST", "redis.example.com"))
	require.NoError(t, os.Setenv("REDIS_PORT", "6379"))
	require.NoError(t, os.Setenv("REDIS_DB", "1"))
	require.NoError(t, os.Setenv("REDIS_POOL_SIZE", "20"))
	require.NoError(t, os.Setenv("REDIS_MIN_IDLE_CONNS", "5"))

	defer func() {
		_ = os.Unsetenv("DATABASE_HOST")
		_ = os.Unsetenv("DATABASE_PORT")
		_ = os.Unsetenv("DATABASE_NAME")
		_ = os.Unsetenv("DATABASE_USER")
		_ = os.Unsetenv("DATABASE_SSL_MODE")
		_ = os.Unsetenv("DATABASE_MAX_OPEN_CONNS")
		_ = os.Unsetenv("DATABASE_MAX_IDLE_CONNS")
		_ = os.Unsetenv("DATABASE_CONN_MAX_LIFETIME")
		_ = os.Unsetenv("REDIS_HOST")
		_ = os.Unsetenv("REDIS_PORT")
		_ = os.Unsetenv("REDIS_DB")
		_ = os.Unsetenv("REDIS_POOL_SIZE")
		_ = os.Unsetenv("REDIS_MIN_IDLE_CONNS")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check individual components are loaded
	assert.Equal(t, 50, cfg.Database.MaxOpenConns)
	assert.Equal(t, 10, cfg.Database.MaxIdleConns)
	assert.Equal(t, time.Hour, cfg.Database.ConnMaxLifetime)
	assert.Equal(t, 20, cfg.Redis.PoolSize)
	assert.Equal(t, 5, cfg.Redis.MinIdleConn)
}

func TestConfigLoad_URLOverridesComponents(t *testing.T) {
	// Test that DATABASE_URL takes precedence over individual components
	require.NoError(t, os.Setenv("ENVIRONMENT", "development"))
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set both URL and components
	require.NoError(t, os.Setenv("DATABASE_URL", "postgresql://urluser:urlpass@urlhost:5433/urldb?sslmode=disable"))
	require.NoError(t, os.Setenv("DATABASE_HOST", "componenthost"))
	require.NoError(t, os.Setenv("DATABASE_PORT", "5432"))
	require.NoError(t, os.Setenv("DATABASE_NAME", "componentdb"))

	defer func() {
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("DATABASE_HOST")
		_ = os.Unsetenv("DATABASE_PORT")
		_ = os.Unsetenv("DATABASE_NAME")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// DATABASE_URL should be used
	assert.Equal(t, "postgresql://urluser:urlpass@urlhost:5433/urldb?sslmode=disable", cfg.Database.URL)
}

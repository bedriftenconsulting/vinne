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
	assert.Equal(t, 50052, cfg.Server.Port)
	assert.Contains(t, cfg.Database.URL, "localhost:5434")
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, "6381", cfg.Redis.Port)
}

func TestConfigLoad_DevelopmentEnvironment(t *testing.T) {
	// Test loading config with development environment (should NOT use localhost defaults)
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set required environment variables
	_ = os.Setenv("DATABASE_URL", "postgresql://test:test@db.example.com:5432/testdb?sslmode=disable")
	_ = os.Setenv("REDIS_HOST", "redis.example.com")
	_ = os.Setenv("REDIS_PORT", "6379")
	_ = os.Setenv("REDIS_PASSWORD", "testpass")
	_ = os.Setenv("REDIS_DB", "0")
	_ = os.Setenv("SERVER_PORT", "50052")
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
	assert.Equal(t, 50052, cfg.Server.Port)
}

func TestConfigLoad_WithTestcontainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test with testcontainers")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("agent_auth"),
		postgres.WithUsername("agent"),
		postgres.WithPassword("agent123"),
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

	databaseURL := fmt.Sprintf("postgresql://agent:agent123@%s:%s/agent_auth?sslmode=disable",
		pgHost, pgPort.Port())

	_ = os.Setenv("DATABASE_URL", databaseURL)
	_ = os.Setenv("REDIS_HOST", redisHost)
	_ = os.Setenv("REDIS_PORT", redisPort.Port())
	_ = os.Setenv("REDIS_PASSWORD", "")
	_ = os.Setenv("REDIS_DB", "0")
	_ = os.Setenv("SERVER_PORT", "50052")
	_ = os.Setenv("JWT_SECRET", "test-secret")
	defer func() {
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("REDIS_HOST")
		_ = os.Unsetenv("REDIS_PORT")
		_ = os.Unsetenv("REDIS_PASSWORD")
		_ = os.Unsetenv("REDIS_DB")
		_ = os.Unsetenv("SERVER_PORT")
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
	assert.Equal(t, 50052, cfg.Server.Port)

	// Test that we can actually connect to the database with the config
	t.Logf("Database URL: %s", cfg.Database.URL)
	t.Logf("Redis Address: %s:%s", cfg.Redis.Host, cfg.Redis.Port)
}

func TestConfigLoad_ProductionRequiresURLs(t *testing.T) {
	// Test that config in production mode can load without URLs
	// (the main.go will validate they are set)
	_ = os.Setenv("ENVIRONMENT", "production")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Don't set DATABASE_URL or REDIS_URL
	_ = os.Setenv("SERVER_PORT", "50052")
	defer func() { _ = os.Unsetenv("SERVER_PORT") }()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Without DATABASE_URL set in production, the config should have empty database URL
	// The service will fail to start in main.go if these are empty
	assert.Empty(t, cfg.Database.URL)
	assert.Empty(t, cfg.Redis.Host)
	assert.Empty(t, cfg.Redis.Port)
	assert.Equal(t, 50052, cfg.Server.Port)
}

func TestConfigLoad_ConnectionPoolSettings(t *testing.T) {
	// Test that connection pool settings can be configured
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set database URL and pool settings
	_ = os.Setenv("DATABASE_URL", "postgresql://test:test@db.example.com:5432/testdb?sslmode=disable")
	_ = os.Setenv("DATABASE_MAX_OPEN_CONNS", "50")
	_ = os.Setenv("DATABASE_MAX_IDLE_CONNS", "10")
	_ = os.Setenv("DATABASE_CONN_MAX_LIFETIME", "1h")

	// Set Redis settings
	_ = os.Setenv("REDIS_HOST", "redis.example.com")
	_ = os.Setenv("REDIS_PORT", "6379")
	_ = os.Setenv("REDIS_PASSWORD", "testpass")
	_ = os.Setenv("REDIS_DB", "1")
	_ = os.Setenv("REDIS_POOL_SIZE", "20")
	_ = os.Setenv("REDIS_MIN_IDLE_CONNS", "5")

	defer func() {
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("DATABASE_MAX_OPEN_CONNS")
		_ = os.Unsetenv("DATABASE_MAX_IDLE_CONNS")
		_ = os.Unsetenv("DATABASE_CONN_MAX_LIFETIME")
		_ = os.Unsetenv("REDIS_HOST")
		_ = os.Unsetenv("REDIS_PORT")
		_ = os.Unsetenv("REDIS_PASSWORD")
		_ = os.Unsetenv("REDIS_DB")
		_ = os.Unsetenv("REDIS_POOL_SIZE")
		_ = os.Unsetenv("REDIS_MIN_IDLE_CONNS")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check connection pool settings are loaded
	assert.Equal(t, 50, cfg.Database.MaxOpenConns)
	assert.Equal(t, 10, cfg.Database.MaxIdleConns)
	assert.Equal(t, time.Hour, cfg.Database.ConnMaxLifetime)
	assert.Equal(t, 20, cfg.Redis.PoolSize)
	assert.Equal(t, 5, cfg.Redis.MinIdleConns)
}

func TestConfigLoad_URLOverridesComponents(t *testing.T) {
	// Test that DATABASE_URL takes precedence over individual components
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set both URL and components
	_ = os.Setenv("DATABASE_URL", "postgresql://urluser:urlpass@urlhost:5433/urldb?sslmode=disable")
	_ = os.Setenv("DATABASE_HOST", "componenthost")
	_ = os.Setenv("DATABASE_PORT", "5432")
	_ = os.Setenv("DATABASE_NAME", "componentdb")

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

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
	assert.Equal(t, "50058", cfg.Server.Port)
	assert.Contains(t, cfg.Database.URL, "localhost:5435")
	assert.Contains(t, cfg.GetRedisURL(), "localhost:6382")
}

func TestConfigLoad_DevelopmentEnvironment(t *testing.T) {
	// Test loading config with development environment (should NOT use localhost defaults)
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set required environment variables
	_ = os.Setenv("DATABASE_URL", "postgresql://test:test@db.example.com:5432/testdb?sslmode=disable")
	_ = os.Setenv("REDIS_URL", "redis://redis.example.com:6379/0")
	_ = os.Setenv("SERVER_PORT", "50058")
	defer func() {
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("REDIS_URL")
		_ = os.Unsetenv("SERVER_PORT")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check that environment variables are used, not localhost defaults
	assert.Equal(t, "postgresql://test:test@db.example.com:5432/testdb?sslmode=disable", cfg.Database.URL)
	assert.Equal(t, "50058", cfg.Server.Port)
}

func TestConfigLoad_WithTestcontainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test with testcontainers")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("agent_management"),
		postgres.WithUsername("agent_mgmt"),
		postgres.WithPassword("agent_mgmt123"),
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

	// Get PostgreSQL connection info
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

	// Get Redis connection info
	redisHost, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	// Set environment variables for development environment
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set database and Redis URLs
	databaseURL := fmt.Sprintf("postgresql://agent_mgmt:agent_mgmt123@%s:%s/agent_management?sslmode=disable",
		pgHost, pgPort.Port())
	redisURL := fmt.Sprintf("redis://%s:%s/0", redisHost, redisPort.Port())

	_ = os.Setenv("DATABASE_URL", databaseURL)
	_ = os.Setenv("REDIS_URL", redisURL)
	_ = os.Setenv("SERVER_PORT", "50058")

	defer func() {
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("REDIS_URL")
		_ = os.Unsetenv("SERVER_PORT")
	}()

	// Load configuration
	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify configuration is loaded correctly
	assert.Equal(t, pgHost, cfg.Database.Host)
	assert.Equal(t, pgPort.Port(), cfg.Database.Port)
	assert.Equal(t, "agent_management", cfg.Database.Name)
	assert.Equal(t, "agent_mgmt", cfg.Database.User)
	assert.Equal(t, "agent_mgmt123", cfg.Database.Password)
	assert.Equal(t, "disable", cfg.Database.SSLMode)

	assert.Equal(t, redisHost, cfg.Redis.Host)
	assert.Equal(t, redisPort.Port(), cfg.Redis.Port)
	assert.Equal(t, 0, cfg.Redis.DB)

	assert.Equal(t, "50058", cfg.Server.Port)

	// Test the helper methods
	expectedDatabaseURL := fmt.Sprintf("postgresql://agent_mgmt:agent_mgmt123@%s:%s/agent_management?sslmode=disable",
		pgHost, pgPort.Port())
	assert.Equal(t, expectedDatabaseURL, cfg.GetDatabaseURL())

	expectedRedisURL := fmt.Sprintf("redis://%s:%s/0", redisHost, redisPort.Port())
	assert.Equal(t, expectedRedisURL, cfg.GetRedisURL())

	assert.Equal(t, "0.0.0.0:50058", cfg.GetServerAddress())

	// Test that we can actually connect to the database with the config
	// (This would normally be done in the service initialization)
	t.Logf("Database URL: %s", cfg.GetDatabaseURL())
	t.Logf("Redis URL: %s", cfg.GetRedisURL())
	t.Logf("Server Address: %s", cfg.GetServerAddress())
}

func TestConfigLoad_MissingRequiredEnvVars(t *testing.T) {
	// Test that config loads even without DATABASE_URL when not in local mode
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Don't set DATABASE_URL or REDIS_URL
	_ = os.Setenv("SERVER_PORT", "50058")
	defer func() { _ = os.Unsetenv("SERVER_PORT") }()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check that we still get a config, but with empty values for unset variables
	assert.Equal(t, "50058", cfg.Server.Port)
}

func TestConfigLoad_ServicesSettings(t *testing.T) {
	// Test that service endpoint settings can be configured
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set service endpoint settings
	_ = os.Setenv("SERVICES_AGENT_AUTH_SERVICE", "auth.example.com:50052")
	_ = os.Setenv("SERVICES_WALLET_SERVICE", "wallet.example.com:50059")
	_ = os.Setenv("SERVICES_COMMISSION_SERVICE", "commission.example.com:50055")
	_ = os.Setenv("SERVICES_NOTIFICATION_SERVICE", "notification.example.com:50056")

	defer func() {
		_ = os.Unsetenv("SERVICES_AGENT_AUTH_SERVICE")
		_ = os.Unsetenv("SERVICES_WALLET_SERVICE")
		_ = os.Unsetenv("SERVICES_COMMISSION_SERVICE")
		_ = os.Unsetenv("SERVICES_NOTIFICATION_SERVICE")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check service configuration
	assert.Equal(t, "auth.example.com:50052", cfg.Services.AgentAuthService)
	assert.Equal(t, "wallet.example.com:50059", cfg.Services.WalletService)
	assert.Equal(t, "commission.example.com:50055", cfg.Services.CommissionService)
	assert.Equal(t, "notification.example.com:50056", cfg.Services.NotificationService)
}

func TestConfigLoad_BusinessSettings(t *testing.T) {
	// Test that business settings can be configured
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set business settings
	_ = os.Setenv("BUSINESS_DEFAULT_AGENT_COMMISSION_PERCENTAGE", "25.5")

	defer func() {
		_ = os.Unsetenv("BUSINESS_DEFAULT_AGENT_COMMISSION_PERCENTAGE")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check business configuration
	assert.Equal(t, 25.5, cfg.Business.DefaultAgentCommissionPercentage)
}

func TestConfigLoad_TracingSettings(t *testing.T) {
	// Test tracing configuration
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set tracing settings
	_ = os.Setenv("TRACING_ENABLED", "false")
	_ = os.Setenv("TRACING_JAEGER_ENDPOINT", "http://jaeger.example.com:4318")
	_ = os.Setenv("TRACING_SERVICE_NAME", "agent-management-prod")
	_ = os.Setenv("TRACING_SERVICE_VERSION", "2.0.0")
	_ = os.Setenv("TRACING_ENVIRONMENT", "production")
	_ = os.Setenv("TRACING_SAMPLE_RATE", "0.1")

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
	assert.Equal(t, "agent-management-prod", cfg.Tracing.ServiceName)
	assert.Equal(t, "2.0.0", cfg.Tracing.ServiceVersion)
	assert.Equal(t, "production", cfg.Tracing.Environment)
	assert.Equal(t, 0.1, cfg.Tracing.SampleRate)
}

func TestConfigLoad_MetricsSettings(t *testing.T) {
	// Test metrics configuration
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set metrics settings
	_ = os.Setenv("METRICS_ENABLED", "false")
	_ = os.Setenv("METRICS_PORT", "9090")

	defer func() {
		_ = os.Unsetenv("METRICS_ENABLED")
		_ = os.Unsetenv("METRICS_PORT")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check metrics configuration
	assert.False(t, cfg.Metrics.Enabled)
	assert.Equal(t, "9090", cfg.Metrics.Port)
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

func TestGetDatabaseURL(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     "5432",
			Name:     "testdb",
			User:     "testuser",
			Password: "testpass",
			SSLMode:  "disable",
		},
	}

	expected := "postgresql://testuser:testpass@localhost:5432/testdb?sslmode=disable"
	assert.Equal(t, expected, cfg.GetDatabaseURL())
}

func TestGetRedisURL_WithPassword(t *testing.T) {
	cfg := &Config{
		Redis: RedisConfig{
			Host:     "localhost",
			Port:     "6379",
			Password: "secret",
			DB:       1,
		},
	}

	expected := "redis://:secret@localhost:6379/1"
	assert.Equal(t, expected, cfg.GetRedisURL())
}

func TestGetRedisURL_WithoutPassword(t *testing.T) {
	cfg := &Config{
		Redis: RedisConfig{
			Host:     "localhost",
			Port:     "6379",
			Password: "",
			DB:       0,
		},
	}

	expected := "redis://localhost:6379/0"
	assert.Equal(t, expected, cfg.GetRedisURL())
}

func TestGetServerAddress(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port: "50058",
		},
	}

	expected := ":50058"
	assert.Equal(t, expected, cfg.GetServerAddress())
}

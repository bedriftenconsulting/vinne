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
	assert.Equal(t, 8056, cfg.Server.Port)
	assert.Contains(t, cfg.Database.URL, "localhost:5436")
	assert.Contains(t, cfg.Redis.URL, "localhost:6383")
}

func TestConfigLoad_DevelopmentEnvironment(t *testing.T) {
	// Test loading config with development environment (should NOT use localhost defaults)
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set required environment variables
	_ = os.Setenv("DATABASE_URL", "postgresql://test:test@db.example.com:5432/testdb?sslmode=disable")
	_ = os.Setenv("REDIS_URL", "redis://redis.example.com:6379/0")
	_ = os.Setenv("SERVER_PORT", "8056")
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
	assert.Equal(t, "redis://redis.example.com:6379/0", cfg.Redis.URL)
	assert.Equal(t, 8056, cfg.Server.Port)
}

func TestConfigLoad_WithTestcontainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test with testcontainers")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("draw_service"),
		postgres.WithUsername("draw_service"),
		postgres.WithPassword("draw_service123"),
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

	databaseURL := fmt.Sprintf("postgresql://draw_service:draw_service123@%s:%s/draw_service?sslmode=disable",
		pgHost, pgPort.Port())
	redisURL := fmt.Sprintf("redis://%s:%s/0", redisHost, redisPort.Port())

	_ = os.Setenv("DATABASE_URL", databaseURL)
	_ = os.Setenv("REDIS_URL", redisURL)
	_ = os.Setenv("SERVER_PORT", "8056")
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
	assert.Equal(t, databaseURL, cfg.Database.URL)
	assert.Equal(t, redisURL, cfg.Redis.URL)
	assert.Equal(t, 8056, cfg.Server.Port)

	// Test that we can actually connect to the database with the config
	// (This would normally be done in the service initialization)
	t.Logf("Database URL: %s", cfg.Database.URL)
	t.Logf("Redis URL: %s", cfg.Redis.URL)
}

func TestConfigLoad_MissingRequiredEnvVars(t *testing.T) {
	// Test that config loads even without DATABASE_URL when not in local mode
	// but the URL will be empty
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Don't set DATABASE_URL or REDIS_URL
	_ = os.Setenv("SERVER_PORT", "8056")
	defer func() {
		_ = os.Unsetenv("SERVER_PORT")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Without DATABASE_URL set, the config should have empty database URL
	assert.Empty(t, cfg.Database.URL)
	assert.Empty(t, cfg.Redis.URL)
	assert.Equal(t, 8056, cfg.Server.Port)
}

func TestConfigLoad_ValidationSettings(t *testing.T) {
	// Test that validation settings can be configured
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set validation settings
	_ = os.Setenv("VALIDATION_NLA_API_ENDPOINT", "https://staging.nla.gov.gh/v2")
	_ = os.Setenv("VALIDATION_REQUIRE_CERTIFICATE", "false")
	_ = os.Setenv("VALIDATION_REQUIRE_WITNESS", "false")
	_ = os.Setenv("VALIDATION_MIN_DOCUMENTS", "1")
	_ = os.Setenv("VALIDATION_VALIDATION_TIMEOUT", "60")

	defer func() {
		_ = os.Unsetenv("VALIDATION_NLA_API_ENDPOINT")
		_ = os.Unsetenv("VALIDATION_REQUIRE_CERTIFICATE")
		_ = os.Unsetenv("VALIDATION_REQUIRE_WITNESS")
		_ = os.Unsetenv("VALIDATION_MIN_DOCUMENTS")
		_ = os.Unsetenv("VALIDATION_VALIDATION_TIMEOUT")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check validation configuration
	assert.Equal(t, "https://staging.nla.gov.gh/v2", cfg.Validation.NLAApiEndpoint)
	assert.False(t, cfg.Validation.RequireCertificate)
	assert.False(t, cfg.Validation.RequireWitness)
	assert.Equal(t, 1, cfg.Validation.MinDocuments)
	assert.Equal(t, 60*time.Second, cfg.Validation.ValidationTimeout)
}

func TestConfigLoad_KafkaSettings(t *testing.T) {
	// Test that Kafka settings can be configured
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set Kafka settings
	_ = os.Setenv("KAFKA_BROKERS", "kafka1.example.com:9092,kafka2.example.com:9092")
	_ = os.Setenv("KAFKA_CONSUMER_GROUP", "draw-service-prod")
	_ = os.Setenv("KAFKA_TOPIC_PREFIX", "production")

	defer func() {
		_ = os.Unsetenv("KAFKA_BROKERS")
		_ = os.Unsetenv("KAFKA_CONSUMER_GROUP")
		_ = os.Unsetenv("KAFKA_TOPIC_PREFIX")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check Kafka configuration
	assert.Equal(t, []string{"kafka1.example.com:9092", "kafka2.example.com:9092"}, cfg.Kafka.Brokers)
	assert.Equal(t, "draw-service-prod", cfg.Kafka.ConsumerGroup)
	assert.Equal(t, "production", cfg.Kafka.TopicPrefix)
}

func TestConfigLoad_TracingSettings(t *testing.T) {
	// Test tracing configuration
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set tracing settings
	_ = os.Setenv("TRACING_ENABLED", "false")
	_ = os.Setenv("TRACING_SERVICE_NAME", "draw-service-prod")
	_ = os.Setenv("TRACING_ENDPOINT", "http://jaeger.example.com:14268/api/traces")

	defer func() {
		_ = os.Unsetenv("TRACING_ENABLED")
		_ = os.Unsetenv("TRACING_SERVICE_NAME")
		_ = os.Unsetenv("TRACING_ENDPOINT")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check tracing configuration
	assert.False(t, cfg.Tracing.Enabled)
	assert.Equal(t, "draw-service-prod", cfg.Tracing.ServiceName)
	assert.Equal(t, "http://jaeger.example.com:14268/api/traces", cfg.Tracing.Endpoint)
}

func TestConfigLoad_DatabaseSettings(t *testing.T) {
	// Test that database settings can be configured
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set database settings
	_ = os.Setenv("DATABASE_MAX_OPEN_CONNS", "50")
	_ = os.Setenv("DATABASE_MAX_IDLE_CONNS", "25")
	_ = os.Setenv("DATABASE_CONN_MAX_LIFETIME", "3600")
	_ = os.Setenv("DATABASE_CONN_MAX_IDLE_TIME", "1200")

	defer func() {
		_ = os.Unsetenv("DATABASE_MAX_OPEN_CONNS")
		_ = os.Unsetenv("DATABASE_MAX_IDLE_CONNS")
		_ = os.Unsetenv("DATABASE_CONN_MAX_LIFETIME")
		_ = os.Unsetenv("DATABASE_CONN_MAX_IDLE_TIME")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check database configuration
	assert.Equal(t, 50, cfg.Database.MaxOpenConns)
	assert.Equal(t, 25, cfg.Database.MaxIdleConns)
	assert.Equal(t, 3600*time.Second, cfg.Database.ConnMaxLifetime)
	assert.Equal(t, 1200*time.Second, cfg.Database.ConnMaxIdleTime)
}

func TestConfigLoad_ServerTimeouts(t *testing.T) {
	// Test server timeout configuration
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set server timeout settings
	_ = os.Setenv("SERVER_READ_TIMEOUT", "60")
	_ = os.Setenv("SERVER_WRITE_TIMEOUT", "60")
	_ = os.Setenv("SERVER_IDLE_TIMEOUT", "300")

	defer func() {
		_ = os.Unsetenv("SERVER_READ_TIMEOUT")
		_ = os.Unsetenv("SERVER_WRITE_TIMEOUT")
		_ = os.Unsetenv("SERVER_IDLE_TIMEOUT")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check server timeout configuration
	assert.Equal(t, 60*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 60*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 300*time.Second, cfg.Server.IdleTimeout)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Server: ServerConfig{
					Port: 8056,
				},
				Database: DatabaseConfig{
					URL: "postgresql://user:pass@localhost/db",
				},
				Kafka: KafkaConfig{
					Brokers: []string{"localhost:9092"},
				},
				Validation: ValidationConfig{
					NLAApiEndpoint:    "https://api.nla.gov.gh/v1",
					MinDocuments:      2,
					ValidationTimeout: 30 * time.Second,
				},
			},
			expectErr: false,
		},
		{
			name: "invalid server port",
			config: &Config{
				Server: ServerConfig{
					Port: 0,
				},
				Database: DatabaseConfig{
					URL: "postgresql://user:pass@localhost/db",
				},
				Kafka: KafkaConfig{
					Brokers: []string{"localhost:9092"},
				},
				Validation: ValidationConfig{
					NLAApiEndpoint:    "https://api.nla.gov.gh/v1",
					MinDocuments:      2,
					ValidationTimeout: 30 * time.Second,
				},
			},
			expectErr: true,
		},
		{
			name: "empty database URL",
			config: &Config{
				Server: ServerConfig{
					Port: 8056,
				},
				Database: DatabaseConfig{
					URL: "",
				},
				Kafka: KafkaConfig{
					Brokers: []string{"localhost:9092"},
				},
				Validation: ValidationConfig{
					NLAApiEndpoint:    "https://api.nla.gov.gh/v1",
					MinDocuments:      2,
					ValidationTimeout: 30 * time.Second,
				},
			},
			expectErr: true,
		},
		{
			name: "no kafka brokers",
			config: &Config{
				Server: ServerConfig{
					Port: 8056,
				},
				Database: DatabaseConfig{
					URL: "postgresql://user:pass@localhost/db",
				},
				Kafka: KafkaConfig{
					Brokers: []string{},
				},
				Validation: ValidationConfig{
					NLAApiEndpoint:    "https://api.nla.gov.gh/v1",
					MinDocuments:      2,
					ValidationTimeout: 30 * time.Second,
				},
			},
			expectErr: true,
		},
		{
			name: "negative min documents",
			config: &Config{
				Server: ServerConfig{
					Port: 8056,
				},
				Database: DatabaseConfig{
					URL: "postgresql://user:pass@localhost/db",
				},
				Kafka: KafkaConfig{
					Brokers: []string{"localhost:9092"},
				},
				Validation: ValidationConfig{
					NLAApiEndpoint:    "https://api.nla.gov.gh/v1",
					MinDocuments:      -1,
					ValidationTimeout: 30 * time.Second,
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigLoad_URLOverridesComponents(t *testing.T) {
	// Test that DATABASE_URL and REDIS_URL take precedence over individual components
	_ = os.Setenv("ENVIRONMENT", "development")
	defer func() { _ = os.Unsetenv("ENVIRONMENT") }()

	// Set both URL and components
	_ = os.Setenv("DATABASE_URL", "postgresql://urluser:urlpass@urlhost:5433/urldb?sslmode=disable")
	_ = os.Setenv("REDIS_URL", "redis://redis.example.com:6380/1")
	_ = os.Setenv("DATABASE_HOST", "componenthost")
	_ = os.Setenv("DATABASE_PORT", "5432")
	_ = os.Setenv("DATABASE_NAME", "componentdb")

	defer func() {
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("REDIS_URL")
		_ = os.Unsetenv("DATABASE_HOST")
		_ = os.Unsetenv("DATABASE_PORT")
		_ = os.Unsetenv("DATABASE_NAME")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// DATABASE_URL and REDIS_URL should be used
	assert.Equal(t, "postgresql://urluser:urlpass@urlhost:5433/urldb?sslmode=disable", cfg.Database.URL)
	assert.Equal(t, "redis://redis.example.com:6380/1", cfg.Redis.URL)
}

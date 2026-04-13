package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Security SecurityConfig `mapstructure:"security"`
	Services ServicesConfig `mapstructure:"services"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Tracing  TracingConfig  `mapstructure:"tracing"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

type LoggingConfig struct {
	Level   string `mapstructure:"level"`
	Format  string `mapstructure:"format"`
	LogFile string `mapstructure:"log_file"`
}

type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	Environment  string        `mapstructure:"environment"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
	URL             string        `mapstructure:"url"`
	MaxConnections  int           `mapstructure:"max_connections"`
	IdleConnections int           `mapstructure:"idle_connections"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

type RedisConfig struct {
	URL            string        `mapstructure:"url"`
	MaxConnections int           `mapstructure:"max_connections"`
	PoolTimeout    time.Duration `mapstructure:"pool_timeout"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
}

type SecurityConfig struct {
	JWTSecret          string        `mapstructure:"jwt_secret"`
	JWTIssuer          string        `mapstructure:"jwt_issuer"`
	SessionExpiry      time.Duration `mapstructure:"session_expiry"`
	AccessTokenExpiry  time.Duration `mapstructure:"access_token_expiry"`
	RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
}

type ServicesConfig struct {
	Notification string `mapstructure:"notification"`
	Wallet       string `mapstructure:"wallet"`
	Payment      string `mapstructure:"payment"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topics  struct {
		AuditLogs string `mapstructure:"audit_logs"`
	} `mapstructure:"topics"`
}

type TracingConfig struct {
	Enabled        bool              `mapstructure:"enabled"`
	JaegerEndpoint string            `mapstructure:"jaeger_endpoint"`
	SampleRate     float64           `mapstructure:"sample_rate"`
	ServiceName    string            `mapstructure:"service_name"`
	ServiceVersion string            `mapstructure:"service_version"`
	Environment    string            `mapstructure:"environment"`
	Region         string            `mapstructure:"region"`
	ExporterType   string            // "otlp", "stdout"
	ExporterConfig map[string]string `mapstructure:"exporter_config"`
}

func Load() (*Config, error) {
	isLocalDev := false

	envPaths := []string{".env", "config.env"}
	for _, path := range envPaths {
		if err := godotenv.Load(path); err == nil {
			fmt.Printf("Loaded environment from: %s (local development mode)\n", path)
			isLocalDev = true
			break
		}
	}

	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = os.Getenv("ENV")
	}
	if env == "local" {
		isLocalDev = true
	}

	if isLocalDev {
		setDefaults()
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	_ = viper.BindEnv("server.port", "SERVER_PORT")
	_ = viper.BindEnv("server.mode", "SERVER_MODE")

	// Database
	_ = viper.BindEnv("database.url", "DATABASE_URL")
	_ = viper.BindEnv("database.max_open_conns", "DATABASE_MAX_OPEN_CONNS")
	_ = viper.BindEnv("database.max_idle_conns", "DATABASE_MAX_IDLE_CONNS")
	_ = viper.BindEnv("database.conn_max_lifetime", "DATABASE_CONN_MAX_LIFETIME")

	// Redis
	_ = viper.BindEnv("redis.host", "REDIS_HOST")
	_ = viper.BindEnv("redis.port", "REDIS_PORT")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("redis.db", "REDIS_DB")

	// Logging
	_ = viper.BindEnv("logging.level", "LOGGING_LEVEL")
	_ = viper.BindEnv("logging.format", "LOGGING_FORMAT")

	// Tracing
	_ = viper.BindEnv("tracing.enabled", "TRACING_ENABLED")
	_ = viper.BindEnv("tracing.jaeger_endpoint", "TRACING_JAEGER_ENDPOINT")
	_ = viper.BindEnv("tracing.sample_rate", "TRACING_SAMPLE_RATE")
	_ = viper.BindEnv("tracing.service_name", "TRACING_SERVICE_NAME")
	_ = viper.BindEnv("tracing.service_version", "TRACING_SERVICE_VERSION")
	_ = viper.BindEnv("tracing.environment", "TRACING_ENVIRONMENT")

	// Kafka
	_ = viper.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	_ = viper.BindEnv("kafka.topics.ticket_events", "KAFKA_TOPICS_TICKET_EVENTS")

	// Service Discovery
	_ = viper.BindEnv("services.notification", "SERVICES_NOTIFICATION_SERVICE")
	_ = viper.BindEnv("services.wallet", "SERVICES_WALLET_SERVICE")
	_ = viper.BindEnv("services.payment", "SERVICES_PAYMENT_SERVICE")

	_ = viper.BindEnv("security.jwt_secret", "SECURITY_JWT_SECRET")
	_ = viper.BindEnv("security.jwt_issuer", "SECURITY_JWT_ISSUER")
	_ = viper.BindEnv("security.session_expiry", "SECURITY_SESSION_EXPIRY")
	_ = viper.BindEnv("security.access_token_expiry", "SECURITY_ACCESS_TOKEN_EXPIRY")
	_ = viper.BindEnv("security.refresh_token_expiry", "SECURITY_REFRESH_TOKEN_EXPIRY")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 50064)
	viper.SetDefault("server.environment", "development")

	viper.SetDefault("logging.level", "debug")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.log_file", "logs/service-player.log")

	// Database defaults
	viper.SetDefault("database.url", "postgresql://postgres:password@localhost:5432/player_db?sslmode=disable")
	viper.SetDefault("database.max_connections", 100)
	viper.SetDefault("database.idle_connections", 10)

	// Redis defaults
	viper.SetDefault("redis.url", "redis://localhost:6379/0")
	viper.SetDefault("redis.max_connections", 50)

	viper.SetDefault("security.jwt_secret", "change-this-secret-in-production")
	viper.SetDefault("security.jwt_issuer", "randco-player-service")
	viper.SetDefault("security.session_expiry", "168h")       // 7 days
	viper.SetDefault("security.access_token_expiry", "1h")    // 1 hour
	viper.SetDefault("security.refresh_token_expiry", "168h") // 7 days

	// Services defaults
	viper.SetDefault("services.notification", "localhost:50063")
	viper.SetDefault("services.wallet", "localhost:50059")
	viper.SetDefault("services.payment", "localhost:50061")

	// Kafka defaults
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.topics.audit_logs", "audit.logs")

	// Tracing defaults
	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.jaeger_endpoint", "http://localhost:4318")
	viper.SetDefault("tracing.sample_rate", 1.0)
	viper.SetDefault("tracing.service_name", "service-player")
	viper.SetDefault("tracing.service_version", "1.0.0")
	viper.SetDefault("tracing.environment", "development")
	viper.SetDefault("tracing.exporter_type", "stdout")
	viper.SetDefault("tracing.exporter_config", map[string]string{})
}

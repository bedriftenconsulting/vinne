package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds all configuration for the payment service
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Kafka     KafkaConfig     `mapstructure:"kafka"`
	Tracing   TracingConfig   `mapstructure:"tracing"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Payment   PaymentConfig   `mapstructure:"payment"`
	Providers ProvidersConfig `mapstructure:"providers"`
	Services  ServicesConfig  `mapstructure:"services"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port int `mapstructure:"port"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	URL             string        `mapstructure:"url"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// KafkaConfig holds Kafka configuration
type KafkaConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
}

// TracingConfig holds OpenTelemetry configuration
type TracingConfig struct {
	Enabled        bool    `mapstructure:"enabled"`
	JaegerEndpoint string  `mapstructure:"jaeger_endpoint"`
	ServiceName    string  `mapstructure:"service_name"`
	ServiceVersion string  `mapstructure:"service_version"`
	Environment    string  `mapstructure:"environment"`
	SampleRate     float64 `mapstructure:"sample_rate"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// PaymentConfig holds payment processing configuration
type PaymentConfig struct {
	DefaultCurrency string        `mapstructure:"default_currency"`
	MaxAmount       int64         `mapstructure:"max_amount"`          // Maximum amount in pesewas
	MinAmount       int64         `mapstructure:"min_amount"`          // Minimum amount in pesewas
	Timeout         time.Duration `mapstructure:"timeout_seconds"`     // Payment processing timeout
	RetryCount      int           `mapstructure:"retry_count"`         // Number of retries for failed payments
	RetryDelay      time.Duration `mapstructure:"retry_delay_seconds"` // Delay between retries
	WebhookSecret   string        `mapstructure:"webhook_secret"`      // Secret for webhook validation
	TestMode        bool          `mapstructure:"test_mode"`           // Enable test mode for providers
}

// ProvidersConfig holds payment provider configurations
type ProvidersConfig struct {
	Orange OrangeConfig `mapstructure:"orange"` // Handles MTN, Telecel, AirtelTigo via unified API
	Banks  BanksConfig  `mapstructure:"banks"`
}

// OrangeConfig holds Orange Extensibility Service configuration
// This service aggregates MTN, Telecel, and AirtelTigo mobile money providers
type OrangeConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	BaseURL        string        `mapstructure:"base_url"`
	SecretKey      string        `mapstructure:"secret_key"`
	SecretToken    string        `mapstructure:"secret_token"`
	Environment    string        `mapstructure:"environment"` // sandbox or production
	Timeout        time.Duration `mapstructure:"timeout_seconds"`
	RetryAttempts  int           `mapstructure:"retry_attempts"`
	RetryDelay     time.Duration `mapstructure:"retry_delay_seconds"`
	CallbackURL    string        `mapstructure:"callback_url"`    // URL Orange calls when transaction status changes
	CallbackSecret string        `mapstructure:"callback_secret"` // Secret for validating webhook signatures
}

// BanksConfig holds bank transfer configuration
type BanksConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	ManualVerification bool   `mapstructure:"manual_verification"` // Whether to use manual verification
	NotificationEmail  string `mapstructure:"notification_email"`
}

// ServicesConfig holds service discovery configuration
type ServicesConfig struct {
	Wallet struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"wallet"`
}

// Load loads configuration from environment variables and config files
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

	// Explicitly bind environment variables (canonical standard)
	// Server
	_ = viper.BindEnv("server.port", "SERVER_PORT")
	_ = viper.BindEnv("environment", "ENVIRONMENT")

	// Database (standard: prefer components; fallback to DATABASE_URL)
	_ = viper.BindEnv("database.url", "DATABASE_URL")
	_ = viper.BindEnv("database.host", "DATABASE_HOST")
	_ = viper.BindEnv("database.port", "DATABASE_PORT")
	_ = viper.BindEnv("database.name", "DATABASE_NAME")
	_ = viper.BindEnv("database.user", "DATABASE_USER")
	_ = viper.BindEnv("database.password", "DATABASE_PASSWORD")
	_ = viper.BindEnv("database.ssl_mode", "DATABASE_SSL_MODE")
	_ = viper.BindEnv("database.max_open_conns", "DATABASE_MAX_OPEN_CONNS")
	_ = viper.BindEnv("database.max_idle_conns", "DATABASE_MAX_IDLE_CONNS")
	_ = viper.BindEnv("database.conn_max_lifetime", "DATABASE_CONN_MAX_LIFETIME")

	// Redis (standard: REDIS_HOST/PORT/DB + REDIS_PASSWORD)
	_ = viper.BindEnv("redis.host", "REDIS_HOST")
	_ = viper.BindEnv("redis.port", "REDIS_PORT")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("redis.db", "REDIS_DB")

	// Kafka
	_ = viper.BindEnv("kafka.enabled", "KAFKA_ENABLED")
	_ = viper.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	_ = viper.BindEnv("kafka.topic", "KAFKA_TOPIC")

	// Tracing
	_ = viper.BindEnv("tracing.enabled", "TRACING_ENABLED")
	_ = viper.BindEnv("tracing.jaeger_endpoint", "TRACING_JAEGER_ENDPOINT")
	_ = viper.BindEnv("tracing.service_name", "TRACING_SERVICE_NAME")
	_ = viper.BindEnv("tracing.service_version", "TRACING_SERVICE_VERSION")
	_ = viper.BindEnv("tracing.environment", "TRACING_ENVIRONMENT")
	_ = viper.BindEnv("tracing.sample_rate", "TRACING_SAMPLE_RATE")

	// Logging
	_ = viper.BindEnv("logging.level", "LOGGING_LEVEL")
	_ = viper.BindEnv("logging.format", "LOGGING_FORMAT")

	// Payment configuration
	_ = viper.BindEnv("payment.default_currency", "PAYMENT_DEFAULT_CURRENCY")
	_ = viper.BindEnv("payment.max_amount", "PAYMENT_MAX_AMOUNT")
	_ = viper.BindEnv("payment.min_amount", "PAYMENT_MIN_AMOUNT")
	_ = viper.BindEnv("payment.timeout_seconds", "PAYMENT_TIMEOUT_SECONDS")
	_ = viper.BindEnv("payment.retry_count", "PAYMENT_RETRY_COUNT")
	_ = viper.BindEnv("payment.retry_delay_seconds", "PAYMENT_RETRY_DELAY_SECONDS")
	_ = viper.BindEnv("payment.webhook_secret", "PAYMENT_WEBHOOK_SECRET")
	_ = viper.BindEnv("payment.test_mode", "PAYMENT_TEST_MODE")

	// Providers - Orange (aggregates MTN, Telecel, AirtelTigo)
	_ = viper.BindEnv("providers.orange.enabled", "PROVIDERS_ORANGE_ENABLED")
	_ = viper.BindEnv("providers.orange.base_url", "PROVIDERS_ORANGE_BASE_URL")
	_ = viper.BindEnv("providers.orange.secret_key", "PROVIDERS_ORANGE_SECRET_KEY")
	_ = viper.BindEnv("providers.orange.secret_token", "PROVIDERS_ORANGE_SECRET_TOKEN")
	_ = viper.BindEnv("providers.orange.environment", "PROVIDERS_ORANGE_ENVIRONMENT")
	_ = viper.BindEnv("providers.orange.timeout_seconds", "PROVIDERS_ORANGE_TIMEOUT_SECONDS")
	_ = viper.BindEnv("providers.orange.retry_attempts", "PROVIDERS_ORANGE_MAX_RETRY")
	_ = viper.BindEnv("providers.orange.retry_delay_seconds", "PROVIDERS_ORANGE_RETRY_DELAY_SECONDS")
	_ = viper.BindEnv("providers.orange.callback_url", "PROVIDERS_ORANGE_CALLBACK_URL")
	_ = viper.BindEnv("providers.orange.callback_secret", "PROVIDERS_ORANGE_CALLBACK_SECRET")

	// Providers - Banks
	_ = viper.BindEnv("providers.banks.enabled", "PROVIDERS_BANKS_ENABLED")
	_ = viper.BindEnv("providers.banks.manual_verification", "PROVIDERS_BANKS_MANUAL_VERIFICATION")
	_ = viper.BindEnv("providers.banks.notification_email", "PROVIDERS_BANKS_NOTIFICATION_EMAIL")

	// Service Discovery
	_ = viper.BindEnv("services.wallet.host", "SERVICES_WALLET_HOST")
	_ = viper.BindEnv("services.wallet.port", "SERVICES_WALLET_PORT")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Normalize database URL if not provided
	if config.Database.URL == "" {
		host := viper.GetString("database.host")
		port := viper.GetString("database.port")
		name := viper.GetString("database.name")
		user := viper.GetString("database.user")
		pass := viper.GetString("database.password")
		ssl := viper.GetString("database.ssl_mode")
		if ssl == "" {
			ssl = "disable"
		}
		if host != "" && port != "" && name != "" && user != "" {
			if pass != "" {
				config.Database.URL = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, name, ssl)
			} else {
				config.Database.URL = fmt.Sprintf("postgresql://%s@%s:%s/%s?sslmode=%s", user, host, port, name, ssl)
			}
		}
	}

	return &config, nil
}

// GetRedisAddr returns the Redis address in host:port format
func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%s", c.Redis.Host, c.Redis.Port)
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 50061)

	// Database defaults
	viper.SetDefault("database.url", "postgresql://payment:payment123@localhost:5440/payment_service?sslmode=disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", 300) // 5 minutes

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", "6387")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	// Kafka defaults
	viper.SetDefault("kafka.enabled", true)
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.topic", "payment-events")

	// Tracing defaults
	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.jaeger_endpoint", "http://localhost:4318")
	viper.SetDefault("tracing.service_name", "payment-service")
	viper.SetDefault("tracing.service_version", "1.0.0")
	viper.SetDefault("tracing.environment", "development")
	viper.SetDefault("tracing.sample_rate", 0.1)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")

	// Payment defaults
	viper.SetDefault("payment.default_currency", "GHS")
	viper.SetDefault("payment.max_amount", 100000000) // ₵1,000,000 in pesewas
	viper.SetDefault("payment.min_amount", 50)        // ₵0.50 in pesewas
	viper.SetDefault("payment.timeout_seconds", 300)  // 5 minutes
	viper.SetDefault("payment.retry_count", 3)
	viper.SetDefault("payment.retry_delay_seconds", 30)
	viper.SetDefault("payment.test_mode", true)

	// Provider defaults (test mode configurations)
	// Orange Extensibility defaults (aggregates MTN, Telecel, AirtelTigo)
	viper.SetDefault("providers.orange.enabled", true)
	viper.SetDefault("providers.orange.base_url", "https://api.orangeextensibility.com")
	viper.SetDefault("providers.orange.environment", "sandbox")
	viper.SetDefault("providers.orange.timeout_seconds", 30)
	viper.SetDefault("providers.orange.retry_attempts", 3)
	viper.SetDefault("providers.orange.retry_delay_seconds", 2)

	// Bank transfer defaults
	viper.SetDefault("providers.banks.enabled", false)
	viper.SetDefault("providers.banks.manual_verification", true)

	// Service Discovery defaults
	viper.SetDefault("services.wallet.host", "localhost")
	viper.SetDefault("services.wallet.port", 50059)
}

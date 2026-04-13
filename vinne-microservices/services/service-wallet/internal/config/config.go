package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds all configuration for the wallet service
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
	Tracing  TracingConfig  `mapstructure:"tracing"`
	Wallet   WalletConfig   `mapstructure:"wallet"`
	Services ServicesConfig `mapstructure:"services"`
}

// ServerConfig contains server configuration
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	Host         string        `mapstructure:"host"`
	Environment  string        `mapstructure:"environment"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	URL             string        `mapstructure:"url"`
	MaxConnections  int           `mapstructure:"max_connections"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// RedisConfig contains Redis configuration
type RedisConfig struct {
	Host       string        `mapstructure:"host"`
	Port       string        `mapstructure:"port"`
	Password   string        `mapstructure:"password"`
	DB         int           `mapstructure:"db"`
	PoolSize   int           `mapstructure:"pool_size"`
	MaxRetries int           `mapstructure:"max_retries"`
	CacheTTL   time.Duration `mapstructure:"cache_ttl"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// MetricsConfig contains metrics configuration
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    int    `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

// TracingConfig contains tracing configuration
type TracingConfig struct {
	Enabled        bool    `mapstructure:"enabled"`
	JaegerEndpoint string  `mapstructure:"jaeger_endpoint"`
	SampleRate     float64 `mapstructure:"sample_rate"`
	ServiceName    string  `mapstructure:"service_name"`
	ServiceVersion string  `mapstructure:"service_version"`
	Environment    string  `mapstructure:"environment"`
}

// WalletConfig contains wallet-specific configuration
type WalletConfig struct {
	DefaultCommissionRate float64       `mapstructure:"default_commission_rate"`
	MaxTransferAmount     float64       `mapstructure:"max_transfer_amount"`
	MinTransferAmount     float64       `mapstructure:"min_transfer_amount"`
	MaxCreditAmount       float64       `mapstructure:"max_credit_amount"`
	MinCreditAmount       float64       `mapstructure:"min_credit_amount"`
	TransactionTimeout    time.Duration `mapstructure:"transaction_timeout"`
	LockTimeout           time.Duration `mapstructure:"lock_timeout"`
}

// ServicesConfig contains external service addresses
type ServicesConfig struct {
	AgentManagementHost string `mapstructure:"agent_management_host"`
	AgentManagementPort string `mapstructure:"agent_management_port"`
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

	// Explicitly bind environment variables
	_ = viper.BindEnv("database.url", "DATABASE_URL")
	_ = viper.BindEnv("redis.host", "REDIS_HOST")
	_ = viper.BindEnv("redis.port", "REDIS_PORT")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("redis.db", "REDIS_DB")
	_ = viper.BindEnv("server.port", "SERVER_PORT")
	_ = viper.BindEnv("server.environment", "SERVER_ENVIRONMENT")
	_ = viper.BindEnv("services.agent_management_host", "SERVICES_AGENT_MANAGEMENT_HOST")
	_ = viper.BindEnv("services.agent_management_port", "SERVICES_AGENT_MANAGEMENT_PORT")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

// GetRedisAddr returns the Redis address in host:port format
func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%s", c.Redis.Host, c.Redis.Port)
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 50059)
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.environment", "development")
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)

	// Database defaults
	viper.SetDefault("database.url", "postgresql://wallet:wallet123@localhost:5438/wallet_service?sslmode=disable")
	viper.SetDefault("database.max_connections", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", 300)

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", "6385")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.cache_ttl", 300) // 5 minutes

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")

	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.port", 9093)
	viper.SetDefault("metrics.path", "/metrics")

	// Tracing defaults
	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.jaeger_endpoint", "http://localhost:4318")
	viper.SetDefault("tracing.sample_rate", 1.0)
	viper.SetDefault("tracing.service_name", "wallet-service")
	viper.SetDefault("tracing.service_version", "1.0.0")
	viper.SetDefault("tracing.environment", "development")

	// Wallet defaults
	viper.SetDefault("wallet.default_commission_rate", 0.30) // 30% default commission
	viper.SetDefault("wallet.max_transfer_amount", 100000.00)
	viper.SetDefault("wallet.min_transfer_amount", 1.00)
	viper.SetDefault("wallet.max_credit_amount", 50000.00)
	viper.SetDefault("wallet.min_credit_amount", 1.00)
	viper.SetDefault("wallet.transaction_timeout", 30) // seconds
	viper.SetDefault("wallet.lock_timeout", 10)        // seconds

	// Services defaults
	viper.SetDefault("services.agent_management_host", "service-agent-management-dev.microservices-dev.svc.cluster.local")
	viper.SetDefault("services.agent_management_port", "50058")
}

// GetAgentManagementAddr returns the agent management service address in host:port format
func (c *Config) GetAgentManagementAddr() string {
	if c.Services.AgentManagementHost == "" {
		return ""
	}
	if c.Services.AgentManagementPort == "" {
		return c.Services.AgentManagementHost
	}
	return fmt.Sprintf("%s:%s", c.Services.AgentManagementHost, c.Services.AgentManagementPort)
}

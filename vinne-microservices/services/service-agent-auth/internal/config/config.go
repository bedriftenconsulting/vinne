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
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Service  ServicesConfig `mapstructure:"services"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Security SecurityConfig `mapstructure:"security"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Tracing  TracingConfig  `mapstructure:"tracing"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
}

type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type DatabaseConfig struct {
	URL             string        `mapstructure:"url"`
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Name            string        `mapstructure:"name"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host         string `mapstructure:"host"`
	Port         string `mapstructure:"port"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topics  Topics   `mapstructure:"topics"`
}

type Topics struct {
	AgentEvents     string `mapstructure:"agent_events"`
	DeviceEvents    string `mapstructure:"device_events"`
	TerritoryEvents string `mapstructure:"territory_events"`
}

type JWTConfig struct {
	AccessTokenExpiry  time.Duration `mapstructure:"access_token_expiry"`
	RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
	Secret             string        `mapstructure:"secret"`
	Issuer             string        `mapstructure:"issuer"`
}

type SecurityConfig struct {
	BcryptCost         int           `mapstructure:"bcrypt_cost"`
	MaxFailedLogins    int           `mapstructure:"max_failed_logins"`
	LockoutDuration    time.Duration `mapstructure:"lockout_duration"`
	PasswordMinLength  int           `mapstructure:"password_min_length"`
	RequireSpecialChar bool          `mapstructure:"require_special_char"`
	RequireUppercase   bool          `mapstructure:"require_uppercase"`
	RequireNumber      bool          `mapstructure:"require_number"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type TracingConfig struct {
	Enabled        bool    `mapstructure:"enabled"`
	JaegerEndpoint string  `mapstructure:"jaeger_endpoint"`
	SampleRate     float64 `mapstructure:"sample_rate"`
	ServiceName    string  `mapstructure:"service_name"`
	ServiceVersion string  `mapstructure:"service_version"`
	Environment    string  `mapstructure:"environment"`
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    string `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

type ServicesConfig struct {
	Notification string `mapstructure:"notification"`
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

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	_ = viper.BindEnv("database.url", "DATABASE_URL")
	_ = viper.BindEnv("redis.host", "REDIS_HOST")
	_ = viper.BindEnv("redis.port", "REDIS_PORT")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("redis.db", "REDIS_DB")
	_ = viper.BindEnv("server.port", "SERVER_PORT")
	// JWT configuration bindings
	_ = viper.BindEnv("jwt.access_token_expiry", "SECURITY_ACCESS_TOKEN_EXPIRY")
	_ = viper.BindEnv("jwt.refresh_token_expiry", "SECURITY_REFRESH_TOKEN_EXPIRY")
	_ = viper.BindEnv("jwt.issuer", "SECURITY_JWT_ISSUER")
	_ = viper.BindEnv("jwt.secret", "SECURITY_JWT_SECRET")
	// Security configuration bindings
	_ = viper.BindEnv("security.lockout_duration", "SECURITY_LOCKOUT_DURATION")
	_ = viper.BindEnv("services.notification", "SERVICES_NOTIFICATION_SERVICE")

	viper.AutomaticEnv()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// GetRedisAddr returns the Redis address in host:port format
func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%s", c.Redis.Host, c.Redis.Port)
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 50052)
	viper.SetDefault("server.read_timeout", 10)
	viper.SetDefault("server.write_timeout", 10)
	viper.SetDefault("server.idle_timeout", 120)

	// Database defaults - for local development, provide complete URLs
	viper.SetDefault("database.url", "postgresql://agent:agent123@localhost:5434/agent_auth?sslmode=disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", 5)

	// Redis defaults - for local development
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", "6381")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("redis.min_idle_conns", 5)

	// Kafka defaults
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.topics.agent_events", "agent.events")
	viper.SetDefault("kafka.topics.device_events", "device.events")
	viper.SetDefault("kafka.topics.territory_events", "territory.events")

	// JWT defaults - use time.Duration values to ensure proper parsing
	viper.SetDefault("jwt.access_token_expiry", 15*time.Minute) // 15 minutes
	viper.SetDefault("jwt.refresh_token_expiry", 168*time.Hour) // 7 days
	viper.SetDefault("jwt.secret", "your-super-secret-jwt-key-change-in-production")
	viper.SetDefault("jwt.issuer", "randco-agent-auth-service")

	// Security defaults
	viper.SetDefault("security.max_failed_logins", 5)
	viper.SetDefault("security.lockout_duration", 30*time.Minute) // 30 minutes
	viper.SetDefault("security.password_min_length", 8)
	viper.SetDefault("security.require_special_char", true)
	viper.SetDefault("security.require_uppercase", true)
	viper.SetDefault("security.require_number", true)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")

	// Tracing defaults
	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.jaeger_endpoint", "http://localhost:4318")
	viper.SetDefault("tracing.sample_rate", 1.0)
	viper.SetDefault("tracing.service_name", "agent-auth")
	viper.SetDefault("tracing.service_version", "1.0.0")
	viper.SetDefault("tracing.environment", "development")

	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.port", "9090")
	viper.SetDefault("metrics.path", "/metrics")

	// services
	viper.SetDefault("services.notification", "localhost:50063")
}

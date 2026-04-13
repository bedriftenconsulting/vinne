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
	// Server Configuration
	Server ServerConfig `mapstructure:"server"`

	// Database Configuration
	Database DatabaseConfig `mapstructure:"database"`

	// Redis Configuration
	Redis RedisConfig `mapstructure:"redis"`

	// Security Configuration
	Security SecurityConfig `mapstructure:"security"`

	// Logging Configuration
	Logging LoggingConfig `mapstructure:"logging"`

	// Metrics Configuration
	Metrics MetricsConfig `mapstructure:"metrics"`

	// Tracing Configuration
	Tracing TracingConfig `mapstructure:"tracing"`

	// Admin Management Service Specific
	AdminManagement AdminManagementConfig `mapstructure:"admin_management"`

	// Kafka Configuration
	Kafka KafkaConfig `mapstructure:"kafka"`
}

type ServerConfig struct {
	Port         string        `mapstructure:"port"`
	Mode         string        `mapstructure:"mode"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type DatabaseConfig struct {
	URL             string        `mapstructure:"url"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host        string `mapstructure:"host"`
	Port        string `mapstructure:"port"`
	Password    string `mapstructure:"password"`
	DB          int    `mapstructure:"db"`
	PoolSize    int    `mapstructure:"pool_size"`
	MinIdleConn int    `mapstructure:"min_idle_conns"`
}

type SecurityConfig struct {
	BcryptCost         int           `mapstructure:"bcrypt_cost"`
	PasswordMinLength  int           `mapstructure:"password_min_length"`
	JWTSecret          string        `mapstructure:"jwt_secret"`
	JWTIssuer          string        `mapstructure:"jwt_issuer"`
	SessionExpiry      time.Duration `mapstructure:"session_expiry"`
	AccessTokenExpiry  time.Duration `mapstructure:"access_token_expiry"`
	RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
	MFAIssuer          string        `mapstructure:"mfa_issuer"`
	MaxFailedLogins    int           `mapstructure:"max_failed_logins"`
	LockoutDuration    time.Duration `mapstructure:"lockout_duration"`
}

type LoggingConfig struct {
	Level   string `mapstructure:"level"`
	Format  string `mapstructure:"format"`
	LogFile string `mapstructure:"log_file"`
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    string `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

type TracingConfig struct {
	Enabled        bool    `mapstructure:"enabled"`
	JaegerEndpoint string  `mapstructure:"jaeger_endpoint"`
	SampleRate     float64 `mapstructure:"sample_rate"`
	ServiceName    string  `mapstructure:"service_name"`
	ServiceVersion string  `mapstructure:"service_version"`
	Environment    string  `mapstructure:"environment"`
}

type AdminManagementConfig struct {
	Port        string `mapstructure:"port"`
	DatabaseURL string `mapstructure:"database_url"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topics  struct {
		AuditLogs string `mapstructure:"audit_logs"`
	} `mapstructure:"topics"`
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

	// Explicitly bind environment variables
	_ = viper.BindEnv("database.url", "DATABASE_URL")
	_ = viper.BindEnv("redis.host", "REDIS_HOST")
	_ = viper.BindEnv("redis.port", "REDIS_PORT")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("redis.db", "REDIS_DB")
	_ = viper.BindEnv("server.port", "SERVER_PORT")
	_ = viper.BindEnv("server.mode", "SERVER_MODE")
	_ = viper.BindEnv("security.jwt_secret", "JWT_SECRET")
	_ = viper.BindEnv("security.jwt_issuer", "SECURITY_JWT_ISSUER")
	_ = viper.BindEnv("security.session_expiry", "SECURITY_SESSION_EXPIRY")
	_ = viper.BindEnv("security.access_token_expiry", "SECURITY_ACCESS_TOKEN_EXPIRY")
	_ = viper.BindEnv("security.refresh_token_expiry", "SECURITY_REFRESH_TOKEN_EXPIRY")
	_ = viper.BindEnv("security.lockout_duration", "SECURITY_LOCKOUT_DURATION")

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
	viper.SetDefault("server.port", "50057")
	viper.SetDefault("server.mode", "development")
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)
	viper.SetDefault("server.idle_timeout", 120)

	// Database defaults
	viper.SetDefault("database.url", "postgresql://admin_mgmt:admin_mgmt123@localhost:5437/admin_management?sslmode=disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", 300)

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", "6384")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("redis.min_idle_conns", 5)

	// Security defaults
	viper.SetDefault("security.bcrypt_cost", 10)
	viper.SetDefault("security.password_min_length", 8)
	viper.SetDefault("security.jwt_secret", "change-this-secret-in-production")
	viper.SetDefault("security.jwt_issuer", "randco-admin-management")
	viper.SetDefault("security.session_expiry", "168h")       // 7 days
	viper.SetDefault("security.access_token_expiry", "1h")    // 1 hour
	viper.SetDefault("security.refresh_token_expiry", "168h") // 7 days
	viper.SetDefault("security.mfa_issuer", "RandcoLottery")
	viper.SetDefault("security.max_failed_logins", 5)
	viper.SetDefault("security.lockout_duration", "30m") // 30 minutes

	// Logging defaults
	viper.SetDefault("logging.level", "debug")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.log_file", "logs/service-admin-management.log")

	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.port", "9092")
	viper.SetDefault("metrics.path", "/metrics")

	// Tracing defaults
	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.jaeger_endpoint", "http://localhost:4318")
	viper.SetDefault("tracing.sample_rate", 1.0)
	viper.SetDefault("tracing.service_name", "admin-management")
	viper.SetDefault("tracing.service_version", "1.0.0")
	viper.SetDefault("tracing.environment", "development")

	// Admin Management defaults
	viper.SetDefault("admin_management.port", "50057")
	viper.SetDefault("admin_management.database_url", "postgresql://admin_mgmt:admin_mgmt123@localhost:5437/admin_management?sslmode=disable")

	// Kafka defaults
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.topics.audit_logs", "audit.logs")
}

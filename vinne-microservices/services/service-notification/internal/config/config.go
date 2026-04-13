package config

import (
	"fmt"
	"os"
	"strings"

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

	// Kafka Configuration
	Kafka KafkaConfig `mapstructure:"kafka"`

	// Provider Configuration
	Providers ProviderConfig `mapstructure:"providers"`

	// Push Notification Configuration (Firebase)
	Push PushConfig `mapstructure:"push"`

	// Notification Configuration
	Notification NotificationConfig `mapstructure:"notification"`

	// Rate Limit Configuration
	RateLimit RateLimitConfig `mapstructure:"ratelimit"`

	// Client Configuration
	Clients ClientsConfig `mapstructure:"clients"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	URL            string `mapstructure:"url"`
	MaxConnections int    `mapstructure:"max_connections"`
	MaxIdleConns   int    `mapstructure:"max_idle_connections"`
	MaxLifetime    int    `mapstructure:"max_lifetime"`
}

type RedisConfig struct {
	URL            string `mapstructure:"url"`
	PoolSize       int    `mapstructure:"pool_size"`
	MinIdleConn    int    `mapstructure:"min_idle_conns"`
	MaxConnAge     int    `mapstructure:"max_conn_age"`    // seconds
	ConnectTimeout int    `mapstructure:"connect_timeout"` // milliseconds
	ReadTimeout    int    `mapstructure:"read_timeout"`    // milliseconds
	WriteTimeout   int    `mapstructure:"write_timeout"`   // milliseconds
	RetryCount     int    `mapstructure:"retry_count"`
	RetryDelay     int    `mapstructure:"retry_delay"` // milliseconds
}

type SecurityConfig struct {
	BcryptCost         int    `mapstructure:"bcrypt_cost"`
	PasswordMinLength  int    `mapstructure:"password_min_length"`
	JWTSecret          string `mapstructure:"jwt_secret"`
	JWTIssuer          string `mapstructure:"jwt_issuer"`
	SessionExpiry      int    `mapstructure:"session_expiry"`       // hours
	AccessTokenExpiry  int    `mapstructure:"access_token_expiry"`  // seconds
	RefreshTokenExpiry int    `mapstructure:"refresh_token_expiry"` // hours
	MFAIssuer          string `mapstructure:"mfa_issuer"`
	MaxFailedLogins    int    `mapstructure:"max_failed_logins"`
	LockoutDuration    int    `mapstructure:"lockout_duration"` // minutes
}

type LoggingConfig struct {
	Level   string `mapstructure:"level"`
	Format  string `mapstructure:"format"`
	LogFile string `mapstructure:"log_file"`
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    string `mapstructure:"port"`
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

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topics  struct {
		AuditLogs string `mapstructure:"audit_logs"`
	} `mapstructure:"topics"`
}

type ProviderConfig struct {
	Email EmailProviderConfig `mapstructure:"email"`
	SMS   SMSProviderConfig   `mapstructure:"sms"`
}

type EmailProviderConfig struct {
	DefaultProvider string                `mapstructure:"default_provider"`
	Mailgun         MailgunProviderConfig `mapstructure:"mailgun"`
}

type SMSProviderConfig struct {
	DefaultProvider string               `mapstructure:"default_provider"`
	Hubtel          HubtelProviderConfig `mapstructure:"hubtel"`
}

type MailgunProviderConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	APIKey  string `mapstructure:"api_key"`
	Domain  string `mapstructure:"domain"`
	BaseURL string `mapstructure:"base_url"`
}

type HubtelProviderConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	BaseURL      string `mapstructure:"base_url"`
	SenderID     string `mapstructure:"sender_id"`
}

type PushConfig struct {
	Firebase FirebaseProviderConfig `mapstructure:"firebase"`
}

type FirebaseProviderConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	CredentialsPath string `mapstructure:"credentials_path"`
}

type NotificationConfig struct {
	GameEndRecipients []string `mapstructure:"game_end_recipients"`
}

type RateLimitConfig struct {
	EmailRatePerHour int `mapstructure:"email_rate_per_hour"` // Max emails per hour
	SMSRatePerMinute int `mapstructure:"sms_rate_per_minute"` // Max SMS per minute
}

type ClientsConfig struct {
	AdminManagementHost string `mapstructure:"admin_management_host"`
	AdminManagementPort string `mapstructure:"admin_management_port"`
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
	_ = viper.BindEnv("server.port", "SERVER_PORT")
	_ = viper.BindEnv("server.mode", "SERVER_MODE")

	_ = viper.BindEnv("database.url", "DATABASE_URL")
	_ = viper.BindEnv("database.max_connections", "DATABASE_MAX_OPEN_CONNS")
	_ = viper.BindEnv("database.max_idle_connections", "DATABASE_MAX_IDLE_CONNS")
	_ = viper.BindEnv("database.max_lifetime", "DATABASE_CONN_MAX_LIFETIME")

	_ = viper.BindEnv("redis.url", "REDIS_URL")
	_ = viper.BindEnv("redis.pool_size", "REDIS_POOL_SIZE")
	_ = viper.BindEnv("redis.min_idle_conns", "REDIS_MIN_IDLE_CONNS")
	_ = viper.BindEnv("redis.max_conn_age", "REDIS_MAX_CONN_AGE")
	_ = viper.BindEnv("redis.connect_timeout", "REDIS_CONNECT_TIMEOUT")
	_ = viper.BindEnv("redis.read_timeout", "REDIS_READ_TIMEOUT")
	_ = viper.BindEnv("redis.write_timeout", "REDIS_WRITE_TIMEOUT")
	_ = viper.BindEnv("redis.retry_count", "REDIS_RETRY_COUNT")
	_ = viper.BindEnv("redis.retry_delay", "REDIS_RETRY_DELAY")

	_ = viper.BindEnv("security.jwt_secret", "JWT_SECRET")
	_ = viper.BindEnv("security.bcrypt_cost", "SECURITY_BCRYPT_COST")
	_ = viper.BindEnv("security.password_min_length", "SECURITY_PASSWORD_MIN_LENGTH")
	_ = viper.BindEnv("security.jwt_issuer", "SECURITY_JWT_ISSUER")
	_ = viper.BindEnv("security.session_expiry", "SECURITY_SESSION_EXPIRY")
	_ = viper.BindEnv("security.access_token_expiry", "SECURITY_ACCESS_TOKEN_EXPIRY")
	_ = viper.BindEnv("security.refresh_token_expiry", "SECURITY_REFRESH_TOKEN_EXPIRY")
	_ = viper.BindEnv("security.mfa_issuer", "SECURITY_MFA_ISSUER")
	_ = viper.BindEnv("security.max_failed_logins", "SECURITY_MAX_FAILED_LOGINS")
	_ = viper.BindEnv("security.lockout_duration", "SECURITY_LOCKOUT_DURATION")

	_ = viper.BindEnv("logging.level", "LOGGING_LEVEL")
	_ = viper.BindEnv("logging.format", "LOGGING_FORMAT")
	_ = viper.BindEnv("logging.log_file", "LOGGING_LOG_FILE")

	_ = viper.BindEnv("metrics.enabled", "METRICS_ENABLED")
	_ = viper.BindEnv("metrics.port", "METRICS_PORT")

	_ = viper.BindEnv("tracing.enabled", "TRACING_ENABLED")
	_ = viper.BindEnv("tracing.jaeger_endpoint", "TRACING_JAEGER_ENDPOINT")
	_ = viper.BindEnv("tracing.sample_rate", "TRACING_SAMPLE_RATE")
	_ = viper.BindEnv("tracing.service_name", "TRACING_SERVICE_NAME")
	_ = viper.BindEnv("tracing.service_version", "TRACING_SERVICE_VERSION")
	_ = viper.BindEnv("tracing.environment", "TRACING_ENVIRONMENT")

	_ = viper.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	_ = viper.BindEnv("kafka.topics.audit_logs", "KAFKA_TOPIC_AUDIT_LOGS")

	_ = viper.BindEnv("providers.email.default_provider", "EMAIL_DEFAULT_PROVIDER")
	_ = viper.BindEnv("providers.email.mailgun.enabled", "EMAIL_MAILGUN_ENABLED")
	_ = viper.BindEnv("providers.email.mailgun.api_key", "EMAIL_MAILGUN_API_KEY")
	_ = viper.BindEnv("providers.email.mailgun.domain", "EMAIL_MAILGUN_DOMAIN")
	_ = viper.BindEnv("providers.email.mailgun.base_url", "EMAIL_MAILGUN_BASE_URL")

	_ = viper.BindEnv("providers.sms.default_provider", "SMS_DEFAULT_PROVIDER")
	_ = viper.BindEnv("providers.sms.hubtel.enabled", "SMS_HUBTEL_ENABLED")
	_ = viper.BindEnv("providers.sms.hubtel.client_id", "SMS_HUBTEL_CLIENT_ID")
	_ = viper.BindEnv("providers.sms.hubtel.client_secret", "SMS_HUBTEL_CLIENT_SECRET")
	_ = viper.BindEnv("providers.sms.hubtel.base_url", "SMS_HUBTEL_BASE_URL")
	_ = viper.BindEnv("providers.sms.hubtel.sender_id", "SMS_HUBTEL_SENDER_ID")

	_ = viper.BindEnv("push.firebase.enabled", "PUSH_FIREBASE_ENABLED")
	_ = viper.BindEnv("push.firebase.credentials_path", "PUSH_FIREBASE_CREDENTIALS_PATH")

	_ = viper.BindEnv("notification.game_end_recipients", "NOTIFICATION_GAME_END_RECIPIENTS")

	_ = viper.BindEnv("ratelimit.email_rate_per_hour", "RATELIMIT_EMAIL_RATE_PER_HOUR")
	_ = viper.BindEnv("ratelimit.sms_rate_per_minute", "RATELIMIT_SMS_RATE_PER_MINUTE")

	_ = viper.BindEnv("clients.admin_management_host", "SERVICES_ADMIN_MANAGEMENT_HOST")
	_ = viper.BindEnv("clients.admin_management_port", "SERVICES_ADMIN_MANAGEMENT_PORT")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", "50063")
	viper.SetDefault("server.mode", "development")

	// Database defaults
	viper.SetDefault("database.url", "postgresql://notification:notification_123@localhost:5437/notification?sslmode=disable")
	viper.SetDefault("database.max_connections", 25)
	viper.SetDefault("database.max_idle_connections", 5)
	viper.SetDefault("database.max_lifetime", 300)

	// Redis defaults
	viper.SetDefault("redis.url", "redis://localhost:6389")
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("redis.min_idle_conns", 5)
	viper.SetDefault("redis.max_conn_age", 1800)    // 30 minutes
	viper.SetDefault("redis.connect_timeout", 5000) // 5 seconds
	viper.SetDefault("redis.read_timeout", 3000)    // 3 seconds
	viper.SetDefault("redis.write_timeout", 3000)   // 3 seconds
	viper.SetDefault("redis.retry_count", 3)
	viper.SetDefault("redis.retry_delay", 1000) // 1 second

	// Security defaults
	viper.SetDefault("security.bcrypt_cost", 10)
	viper.SetDefault("security.password_min_length", 8)
	viper.SetDefault("security.jwt_secret", "change-this-secret-in-production")
	viper.SetDefault("security.jwt_issuer", "randco-admin-management")
	viper.SetDefault("security.session_expiry", 168)       // 7 days in hours
	viper.SetDefault("security.access_token_expiry", 3600) // 1 hour in seconds
	viper.SetDefault("security.refresh_token_expiry", 168) // 7 days in hours
	viper.SetDefault("security.mfa_issuer", "RandcoLottery")
	viper.SetDefault("security.max_failed_logins", 5)
	viper.SetDefault("security.lockout_duration", 30) // 30 minutes

	// Logging defaults
	viper.SetDefault("logging.level", "debug")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.log_file", "logs/service-notification.log")

	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.port", "9092")

	// Tracing defaults
	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.jaeger_endpoint", "http://localhost:4318")
	viper.SetDefault("tracing.sample_rate", 1.0)
	viper.SetDefault("tracing.service_name", "notification")
	viper.SetDefault("tracing.service_version", "1.0.0")
	viper.SetDefault("tracing.environment", "development")
	viper.SetDefault("tracing.exporter_type", "stdout")
	viper.SetDefault("tracing.exporter_config", map[string]string{})

	// Kafka defaults
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.topics.audit_logs", "audit.logs")

	// Provider defaults
	viper.SetDefault("providers.email.default_provider", "mailgun")
	viper.SetDefault("providers.email.mailgun.enabled", true)
	viper.SetDefault("providers.email.mailgun.api_key", "")
	viper.SetDefault("providers.email.mailgun.domain", "")
	viper.SetDefault("providers.email.mailgun.base_url", "https://api.mailgun.net/v3")

	viper.SetDefault("providers.sms.default_provider", "hubtel")
	viper.SetDefault("providers.sms.hubtel.enabled", true)
	viper.SetDefault("providers.sms.hubtel.client_id", "")
	viper.SetDefault("providers.sms.hubtel.client_secret", "")
	viper.SetDefault("providers.sms.hubtel.sender_id", "")
	viper.SetDefault("providers.sms.hubtel.base_url", "https://smsc.hubtel.com/v1/messages")

	// Push notification defaults
	viper.SetDefault("push.firebase.enabled", false)
	viper.SetDefault("push.firebase.credentials_path", "")

	// Notification defaults
	viper.SetDefault("notification.game_end_recipients", []string{"paulakabah@gmail.com", "jeffrey@bedriften.xyz"})

	// Rate limit defaults
	viper.SetDefault("ratelimit.email_rate_per_hour", 90) // 90 emails per hour
	viper.SetDefault("ratelimit.sms_rate_per_minute", 60) // 60 SMS per minute

	// Client defaults
	viper.SetDefault("clients.admin_management_host", "localhost")
	viper.SetDefault("clients.admin_management_port", "50057")
}

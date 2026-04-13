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
	Server              ServerConfig              `mapstructure:"server"`
	Database            DatabaseConfig            `mapstructure:"database"`
	Redis               RedisConfig               `mapstructure:"redis"`
	Logging             LoggingConfig             `mapstructure:"logging"`
	Tracing             TracingConfig             `mapstructure:"tracing"`
	Kafka               KafkaConfig               `mapstructure:"kafka"`
	Scheduler           SchedulerConfig           `mapstructure:"scheduler"`
	DrawService         DrawServiceConfig         `mapstructure:"draw_service"`
	TicketService       TicketServiceConfig       `mapstructure:"ticket_service"`
	NotificationService NotificationServiceConfig `mapstructure:"notification_service"`
	AdminService        AdminServiceConfig        `mapstructure:"admin_service"`
	Notification        NotificationConfig        `mapstructure:"notification"`
	Storage             StorageConfig             `mapstructure:"storage"`
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
	Port            string        `mapstructure:"port"`
	Name            string        `mapstructure:"name"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

type RedisConfig struct {
	Host         string `mapstructure:"host"`
	Port         string `mapstructure:"port"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
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

type KafkaConfig struct {
	Brokers       []string `mapstructure:"brokers"`
	ConsumerGroup string   `mapstructure:"consumer_group"`
	TopicPrefix   string   `mapstructure:"topic_prefix"`
	Topics        struct {
		GameEvents     string `mapstructure:"game_events"`
		ApprovalEvents string `mapstructure:"approval_events"`
	} `mapstructure:"topics"`
}

type SchedulerConfig struct {
	Enabled       bool          `mapstructure:"enabled"`
	Interval      time.Duration `mapstructure:"interval"`
	WindowMinutes int           `mapstructure:"window_minutes"`
	Timezone      string        `mapstructure:"timezone"`
}

type DrawServiceConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Timeout      time.Duration `mapstructure:"timeout"`
	MaxRetries   int           `mapstructure:"max_retries"`
	RetryBackoff time.Duration `mapstructure:"retry_backoff"`
}

type TicketServiceConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Timeout      time.Duration `mapstructure:"timeout"`
	MaxRetries   int           `mapstructure:"max_retries"`
	RetryBackoff time.Duration `mapstructure:"retry_backoff"`
}

type NotificationServiceConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Timeout      time.Duration `mapstructure:"timeout"`
	MaxRetries   int           `mapstructure:"max_retries"`
	RetryBackoff time.Duration `mapstructure:"retry_backoff"`
}

type AdminServiceConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Timeout      time.Duration `mapstructure:"timeout"`
	MaxRetries   int           `mapstructure:"max_retries"`
	RetryBackoff time.Duration `mapstructure:"retry_backoff"`
}

type NotificationConfig struct {
	FallbackEmails []string `mapstructure:"fallback_emails"`
}

type StorageConfig struct {
	Provider        string `mapstructure:"provider"`
	Endpoint        string `mapstructure:"endpoint"`
	Region          string `mapstructure:"region"`
	Bucket          string `mapstructure:"bucket"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	CDNEndpoint     string `mapstructure:"cdn_endpoint"`
	ForcePathStyle  bool   `mapstructure:"force_path_style"`
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
	// Server
	_ = viper.BindEnv("server.port", "SERVER_PORT")
	_ = viper.BindEnv("server.read_timeout", "SERVER_READ_TIMEOUT")
	_ = viper.BindEnv("server.write_timeout", "SERVER_WRITE_TIMEOUT")
	_ = viper.BindEnv("server.idle_timeout", "SERVER_IDLE_TIMEOUT")

	// Database
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
	_ = viper.BindEnv("database.conn_max_idle_time", "DATABASE_CONN_MAX_IDLE_TIME")

	// Redis
	_ = viper.BindEnv("redis.host", "REDIS_HOST")
	_ = viper.BindEnv("redis.port", "REDIS_PORT")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("redis.db", "REDIS_DB")
	_ = viper.BindEnv("redis.pool_size", "REDIS_POOL_SIZE")
	_ = viper.BindEnv("redis.min_idle_conns", "REDIS_MIN_IDLE_CONNS")

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
	_ = viper.BindEnv("kafka.consumer_group", "KAFKA_CONSUMER_GROUP")
	_ = viper.BindEnv("kafka.topic_prefix", "KAFKA_TOPIC_PREFIX")
	_ = viper.BindEnv("kafka.topics.game_events", "KAFKA_TOPICS_GAME_EVENTS")
	_ = viper.BindEnv("kafka.topics.approval_events", "KAFKA_TOPICS_APPROVAL_EVENTS")

	// Scheduler
	_ = viper.BindEnv("scheduler.enabled", "SCHEDULER_ENABLED")
	_ = viper.BindEnv("scheduler.interval", "SCHEDULER_INTERVAL")
	_ = viper.BindEnv("scheduler.window_minutes", "SCHEDULER_WINDOW_MINUTES")
	_ = viper.BindEnv("scheduler.timezone", "SCHEDULER_TIMEZONE")

	// Draw Service
	_ = viper.BindEnv("draw_service.host", "SERVICES_DRAW_HOST")
	_ = viper.BindEnv("draw_service.port", "SERVICES_DRAW_PORT")
	_ = viper.BindEnv("draw_service.timeout", "SERVICES_DRAW_TIMEOUT")
	_ = viper.BindEnv("draw_service.max_retries", "SERVICES_DRAW_MAX_RETRIES")
	_ = viper.BindEnv("draw_service.retry_backoff", "SERVICES_DRAW_RETRY_BACKOFF")

	// Ticket Service
	_ = viper.BindEnv("ticket_service.host", "SERVICES_TICKET_HOST")
	_ = viper.BindEnv("ticket_service.port", "SERVICES_TICKET_PORT")
	_ = viper.BindEnv("ticket_service.timeout", "SERVICES_TICKET_TIMEOUT")
	_ = viper.BindEnv("ticket_service.max_retries", "SERVICES_TICKET_MAX_RETRIES")
	_ = viper.BindEnv("ticket_service.retry_backoff", "SERVICES_TICKET_RETRY_BACKOFF")

	// Notification Service
	_ = viper.BindEnv("notification_service.host", "SERVICES_NOTIFICATION_HOST")
	_ = viper.BindEnv("notification_service.port", "SERVICES_NOTIFICATION_PORT")
	_ = viper.BindEnv("notification_service.timeout", "SERVICES_NOTIFICATION_TIMEOUT")
	_ = viper.BindEnv("notification_service.max_retries", "SERVICES_NOTIFICATION_MAX_RETRIES")
	_ = viper.BindEnv("notification_service.retry_backoff", "SERVICES_NOTIFICATION_RETRY_BACKOFF")

	// Admin Service
	_ = viper.BindEnv("admin_service.host", "SERVICES_ADMIN_HOST")
	_ = viper.BindEnv("admin_service.port", "SERVICES_ADMIN_PORT")
	_ = viper.BindEnv("admin_service.timeout", "SERVICES_ADMIN_TIMEOUT")
	_ = viper.BindEnv("admin_service.max_retries", "SERVICES_ADMIN_MAX_RETRIES")
	_ = viper.BindEnv("admin_service.retry_backoff", "SERVICES_ADMIN_RETRY_BACKOFF")

	// Notification Config
	_ = viper.BindEnv("notification.fallback_emails", "NOTIFICATION_FALLBACK_EMAILS")

	// Storage
	_ = viper.BindEnv("storage.provider", "STORAGE_PROVIDER")
	_ = viper.BindEnv("storage.endpoint", "STORAGE_ENDPOINT")
	_ = viper.BindEnv("storage.region", "STORAGE_REGION")
	_ = viper.BindEnv("storage.bucket", "STORAGE_BUCKET")
	_ = viper.BindEnv("storage.access_key_id", "STORAGE_ACCESS_KEY_ID")
	_ = viper.BindEnv("storage.secret_access_key", "STORAGE_SECRET_ACCESS_KEY")
	_ = viper.BindEnv("storage.cdn_endpoint", "STORAGE_CDN_ENDPOINT")
	_ = viper.BindEnv("storage.force_path_style", "STORAGE_FORCE_PATH_STYLE")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Parse comma-separated fallback emails if provided as a string
	if emailsStr := os.Getenv("NOTIFICATION_FALLBACK_EMAILS"); emailsStr != "" {
		// Split by comma and trim whitespace
		emails := strings.Split(emailsStr, ",")
		for i, email := range emails {
			emails[i] = strings.TrimSpace(email)
		}
		// Filter out empty strings
		validEmails := make([]string, 0, len(emails))
		for _, email := range emails {
			if email != "" {
				validEmails = append(validEmails, email)
			}
		}
		if len(validEmails) > 0 {
			config.Notification.FallbackEmails = validEmails
		}
	}

	return &config, nil
}

// GetRedisAddr returns the Redis address in host:port format
func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%s", c.Redis.Host, c.Redis.Port)
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 50053)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.idle_timeout", "120s")

	// Database defaults
	viper.SetDefault("database.url", "postgresql://game:game123@localhost:5441/game_service?sslmode=disable")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", "5441")
	viper.SetDefault("database.name", "game_service")
	viper.SetDefault("database.user", "game")
	viper.SetDefault("database.password", "game123")
	viper.SetDefault("database.ssl_mode", "disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("database.conn_max_lifetime", "1800s")
	viper.SetDefault("database.conn_max_idle_time", "600s")

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", "6388")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("redis.min_idle_conns", 5)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")

	// Tracing defaults
	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.jaeger_endpoint", "http://localhost:4318")
	viper.SetDefault("tracing.sample_rate", 1.0)
	viper.SetDefault("tracing.service_name", "service-game")
	viper.SetDefault("tracing.service_version", "1.0.0")
	viper.SetDefault("tracing.environment", "development")

	// Kafka defaults
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.consumer_group", "game-service-group")
	viper.SetDefault("kafka.topic_prefix", "randco")
	viper.SetDefault("kafka.topics.game_events", "game.events")
	viper.SetDefault("kafka.topics.approval_events", "approval.events")

	// Scheduler defaults
	viper.SetDefault("scheduler.enabled", true)
	viper.SetDefault("scheduler.interval", "1m")
	viper.SetDefault("scheduler.window_minutes", 2)
	viper.SetDefault("scheduler.timezone", "Africa/Accra")

	// Draw Service defaults (for local development)
	viper.SetDefault("draw_service.host", "localhost")
	viper.SetDefault("draw_service.port", 50060)
	viper.SetDefault("draw_service.timeout", "10s")
	viper.SetDefault("draw_service.max_retries", 3)
	viper.SetDefault("draw_service.retry_backoff", "1s")

	// Ticket Service defaults (for local development)
	viper.SetDefault("ticket_service.host", "localhost")
	viper.SetDefault("ticket_service.port", 50062)
	viper.SetDefault("ticket_service.timeout", "10s")
	viper.SetDefault("ticket_service.max_retries", 3)
	viper.SetDefault("ticket_service.retry_backoff", "1s")

	// Notification Service defaults (for local development)
	viper.SetDefault("notification_service.host", "localhost")
	viper.SetDefault("notification_service.port", 50063)
	viper.SetDefault("notification_service.timeout", "10s")
	viper.SetDefault("notification_service.max_retries", 3)
	viper.SetDefault("notification_service.retry_backoff", "1s")

	// Admin Service defaults (for local development)
	viper.SetDefault("admin_service.host", "localhost")
	viper.SetDefault("admin_service.port", 50057)
	viper.SetDefault("admin_service.timeout", "10s")
	viper.SetDefault("admin_service.max_retries", 3)
	viper.SetDefault("admin_service.retry_backoff", "1s")

	// Notification fallback emails
	viper.SetDefault("notification.fallback_emails", []string{"paulakabah@gmail.com", "jeffrey@bedriften.xyz"})

	// Storage defaults (for local development - using DigitalOcean Spaces)
	viper.SetDefault("storage.provider", "spaces")
	viper.SetDefault("storage.endpoint", "https://sgp1.digitaloceanspaces.com")
	viper.SetDefault("storage.region", "sgp1")
	viper.SetDefault("storage.bucket", "rand-dev-static")
	viper.SetDefault("storage.access_key_id", "")
	viper.SetDefault("storage.secret_access_key", "")
	viper.SetDefault("storage.cdn_endpoint", "")
	viper.SetDefault("storage.force_path_style", false)
}

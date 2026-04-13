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
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Kafka      KafkaConfig      `mapstructure:"kafka"`
	Tracing    TracingConfig    `mapstructure:"tracing"`
	Validation ValidationConfig `mapstructure:"validation"`
	Services   ServicesConfig   `mapstructure:"services"`
}

type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type DatabaseConfig struct {
	URL             string        `mapstructure:"url"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

type RedisConfig struct {
	URL      string `mapstructure:"url"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type KafkaConfig struct {
	Brokers       []string `mapstructure:"brokers"`
	ConsumerGroup string   `mapstructure:"consumer_group"`
	TopicPrefix   string   `mapstructure:"topic_prefix"`
}

type TracingConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	ServiceName string `mapstructure:"service_name"`
	Endpoint    string `mapstructure:"endpoint"`
}

type ValidationConfig struct {
	NLAApiEndpoint     string        `mapstructure:"nla_api_endpoint"`
	RequireCertificate bool          `mapstructure:"require_certificate"`
	RequireWitness     bool          `mapstructure:"require_witness"`
	MinDocuments       int           `mapstructure:"min_documents"`
	ValidationTimeout  time.Duration `mapstructure:"validation_timeout"`
}

type ServicesConfig struct {
	TicketServiceHost string `mapstructure:"ticket_host"`
	TicketServicePort string `mapstructure:"ticket_port"`
	WalletServiceHost string `mapstructure:"wallet_host"`
	WalletServicePort string `mapstructure:"wallet_port"`
	GameServiceHost   string `mapstructure:"game_host"`
	GameServicePort   string `mapstructure:"game_port"`
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
	_ = viper.BindEnv("redis.url", "REDIS_URL")
	_ = viper.BindEnv("server.port", "SERVER_PORT")
	_ = viper.BindEnv("validation.nla_api_endpoint", "VALIDATION_NLA_API_ENDPOINT")
	_ = viper.BindEnv("validation.min_documents", "VALIDATION_MIN_DOCUMENTS")
	_ = viper.BindEnv("validation.validation_timeout", "VALIDATION_VALIDATION_TIMEOUT")
	_ = viper.BindEnv("services.ticket_host", "SERVICES_TICKET_HOST")
	_ = viper.BindEnv("services.ticket_port", "SERVICES_TICKET_PORT")
	_ = viper.BindEnv("services.wallet_host", "SERVICES_WALLET_HOST")
	_ = viper.BindEnv("services.wallet_port", "SERVICES_WALLET_PORT")
	_ = viper.BindEnv("services.game_host", "SERVICES_GAME_HOST")
	_ = viper.BindEnv("services.game_port", "SERVICES_GAME_PORT")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Parse KAFKA_BROKERS as comma-separated list
	if kafkaBrokers := os.Getenv("KAFKA_BROKERS"); kafkaBrokers != "" {
		config.Kafka.Brokers = strings.Split(kafkaBrokers, ",")
		// Trim whitespace from each broker
		for i := range config.Kafka.Brokers {
			config.Kafka.Brokers[i] = strings.TrimSpace(config.Kafka.Brokers[i])
		}
	}

	return &config, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 50060)
	// gRPC port defaults to SERVER_PORT unless overridden elsewhere
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)
	viper.SetDefault("server.idle_timeout", 120)

	// Database defaults
	viper.SetDefault("database.url", "postgresql://draw_service:draw_service123@localhost:5436/draw_service?sslmode=disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("database.conn_max_lifetime", 1800) // 30 minutes
	viper.SetDefault("database.conn_max_idle_time", 600) // 10 minutes

	// Redis defaults
	viper.SetDefault("redis.url", "redis://localhost:6383/0")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	// Kafka defaults
	viper.SetDefault("kafka.brokers", []string{"localhost:9097"})
	viper.SetDefault("kafka.consumer_group", "draw-service-group")
	viper.SetDefault("kafka.topic_prefix", "randco")

	// Tracing defaults
	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.service_name", "draw-service")
	viper.SetDefault("tracing.endpoint", "http://localhost:14268/api/traces")

	// Validation defaults
	viper.SetDefault("validation.nla_api_endpoint", "https://api.nla.gov.gh/v1")
	viper.SetDefault("validation.require_certificate", true)
	viper.SetDefault("validation.require_witness", true)
	viper.SetDefault("validation.min_documents", 2)
	viper.SetDefault("validation.validation_timeout", 30)

	// Service discovery defaults
	viper.SetDefault("services.ticket_host", "localhost")
	viper.SetDefault("services.ticket_port", "50062")
	viper.SetDefault("services.wallet_host", "localhost")
	viper.SetDefault("services.wallet_port", "50059")
	viper.SetDefault("services.game_host", "localhost")
	viper.SetDefault("services.game_port", "50053")
}

func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Database.URL == "" {
		return fmt.Errorf("database URL is required")
	}

	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("at least one Kafka broker is required")
	}

	// Validate draw validation configuration
	if c.Validation.NLAApiEndpoint == "" {
		return fmt.Errorf("NLA API endpoint is required")
	}

	if c.Validation.MinDocuments < 0 {
		return fmt.Errorf("minimum documents must be non-negative: %d", c.Validation.MinDocuments)
	}

	if c.Validation.ValidationTimeout <= 0 {
		return fmt.Errorf("validation timeout must be positive: %d", c.Validation.ValidationTimeout)
	}

	return nil
}

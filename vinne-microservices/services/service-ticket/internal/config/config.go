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
	Logging  LoggingConfig  `mapstructure:"logging"`
	Tracing  TracingConfig  `mapstructure:"tracing"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Services ServicesConfig `mapstructure:"services"`
	Business BusinessConfig `mapstructure:"business"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type DatabaseConfig struct {
	URL             string        `mapstructure:"url"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
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
	Brokers []string `mapstructure:"brokers"`
	Topics  struct {
		TicketEvents string `mapstructure:"ticket_events"`
	} `mapstructure:"topics"`
}

type ServicesConfig struct {
	Game struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"game"`
	Draw struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"draw"`
	Payment struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"payment"`
	Wallet struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"wallet"`
	AgentManagement struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"agent_management"`
}

type BusinessConfig struct {
	SerialPrefix string `mapstructure:"serial_prefix"`
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

	// Explicitly bind environment variables to config paths
	_ = viper.BindEnv("server.port", "SERVER_PORT")

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
	_ = viper.BindEnv("services.game.host", "SERVICES_GAME_HOST")
	_ = viper.BindEnv("services.game.port", "SERVICES_GAME_PORT")
	_ = viper.BindEnv("services.draw.host", "SERVICES_DRAW_HOST")
	_ = viper.BindEnv("services.draw.port", "SERVICES_DRAW_PORT")
	_ = viper.BindEnv("services.payment.host", "SERVICES_PAYMENT_HOST")
	_ = viper.BindEnv("services.payment.port", "SERVICES_PAYMENT_PORT")
	_ = viper.BindEnv("services.wallet.host", "SERVICES_WALLET_HOST")
	_ = viper.BindEnv("services.wallet.port", "SERVICES_WALLET_PORT")
	_ = viper.BindEnv("services.agent_management.host", "SERVICES_AGENT_MANAGEMENT_HOST")
	_ = viper.BindEnv("services.agent_management.port", "SERVICES_AGENT_MANAGEMENT_PORT")

	// Business
	_ = viper.BindEnv("business.serial_prefix", "BUSINESS_SERIAL_PREFIX")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 50062)

	// Database defaults
	viper.SetDefault("database.url", "postgresql://ticket:ticket123@localhost:5438/ticket_service?sslmode=disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("database.conn_max_lifetime", "5m")

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6385)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")

	// Tracing defaults
	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.jaeger_endpoint", "localhost:4318")
	viper.SetDefault("tracing.sample_rate", 1.0)
	viper.SetDefault("tracing.service_name", "service-ticket")
	viper.SetDefault("tracing.service_version", "1.0.0")
	viper.SetDefault("tracing.environment", "development")

	// Kafka defaults
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.topics.ticket_events", "ticket.events")

	// Service defaults
	viper.SetDefault("services.game.host", "localhost")
	viper.SetDefault("services.game.port", 50053)
	viper.SetDefault("services.wallet.host", "localhost")
	viper.SetDefault("services.wallet.port", 50059)
	viper.SetDefault("services.agent_management.host", "localhost")
	viper.SetDefault("services.agent_management.port", 50058)

	// Business defaults
	viper.SetDefault("business.serial_prefix", "TKT")
}

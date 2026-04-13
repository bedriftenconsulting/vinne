package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/joho/godotenv"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config represents the gateway configuration
type Config struct {
	Server     ServerConfig     `json:"server" yaml:"server" mapstructure:"server"`
	Cache      CacheConfig      `json:"cache" yaml:"cache" mapstructure:"cache"`
	Redis      RedisConfig      `json:"redis" yaml:"redis" mapstructure:"redis"`
	Terminal   TerminalConfig   `json:"terminal" yaml:"terminal" mapstructure:"terminal"`
	RateLimit  RateLimitConfig  `json:"rate_limit" yaml:"rate_limit" mapstructure:"rate_limit"`
	Services   []ServiceConfig  `json:"services" yaml:"services" mapstructure:"services"`
	Security   SecurityConfig   `json:"security" yaml:"security" mapstructure:"security"`
	Metrics    MetricsConfig    `json:"metrics" yaml:"metrics" mapstructure:"metrics"`
	WebSocket  WebSocketConfig  `json:"websocket" yaml:"websocket" mapstructure:"websocket"`
	Retry      RetryConfig      `json:"retry" yaml:"retry" mapstructure:"retry"`
	Versioning VersioningConfig `json:"versioning" yaml:"versioning" mapstructure:"versioning"`
	Transform  TransformConfig  `json:"transform" yaml:"transform" mapstructure:"transform"`
	Tracing    TracingConfig    `json:"tracing" yaml:"tracing" mapstructure:"tracing"`
	Storage    StorageConfig    `json:"storage" yaml:"storage" mapstructure:"storage"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port            string        `json:"port" yaml:"port" mapstructure:"port"`
	ReadTimeout     time.Duration `json:"read_timeout" yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout" yaml:"write_timeout" mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `json:"idle_timeout" yaml:"idle_timeout" mapstructure:"idle_timeout"`
	MaxHeaderBytes  int           `json:"max_header_bytes" yaml:"max_header_bytes" mapstructure:"max_header_bytes"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout" mapstructure:"shutdown_timeout"`
	LogLevel        string        `json:"log_level" yaml:"log_level" mapstructure:"log_level"`
	LogFormat       string        `json:"log_format" yaml:"log_format" mapstructure:"log_format"`
	LogFile         string        `json:"log_file" yaml:"log_file" mapstructure:"log_file"`
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	Enabled            bool                     `json:"enabled" yaml:"enabled"`
	TTL                time.Duration            `json:"ttl" yaml:"ttl"`
	MaxSize            int64                    `json:"max_size" yaml:"max_size"`
	CacheableEndpoints map[string]time.Duration `json:"cacheable_endpoints" yaml:"cacheable_endpoints"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type TerminalConfig struct {
	EnableNewAuth bool `json:"enable_new_auth" yaml:"enable_new_auth" mapstructure:"enable_new_auth"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled        bool                     `json:"enabled" yaml:"enabled"`
	RequestsPerMin int                      `json:"requests_per_min" yaml:"requests_per_min"`
	BurstSize      int                      `json:"burst_size" yaml:"burst_size"`
	CustomLimits   map[string]RateLimitRule `json:"custom_limits" yaml:"custom_limits"`
}

// RateLimitRule defines a rate limit rule
type RateLimitRule struct {
	RequestsPerMin int  `json:"requests_per_min" yaml:"requests_per_min"`
	BurstSize      int  `json:"burst_size" yaml:"burst_size"`
	PerUser        bool `json:"per_user" yaml:"per_user"`
}

// ServiceConfig holds backend service configuration
type ServiceConfig struct {
	Name        string        `json:"name" yaml:"name"`
	URL         string        `json:"url" yaml:"url"`
	Timeout     time.Duration `json:"timeout" yaml:"timeout"`
	MaxRetries  int           `json:"max_retries" yaml:"max_retries"`
	HealthCheck string        `json:"health_check" yaml:"health_check"`
	Weight      int           `json:"weight" yaml:"weight"`
}

// SecurityConfig holds security configuration
type SecurityConfig struct {
	JWTSecret             string   `json:"jwt_secret" yaml:"jwt_secret" mapstructure:"jwt_secret"`
	JWTIssuer             string   `json:"jwt_issuer" yaml:"jwt_issuer" mapstructure:"jwt_issuer"`
	AllowedOrigins        []string `json:"allowed_origins" yaml:"allowed_origins" mapstructure:"allowed_origins"`
	AllowedHeaders        []string `json:"allowed_headers" yaml:"allowed_headers" mapstructure:"allowed_headers"`
	AllowedMethods        []string `json:"allowed_methods" yaml:"allowed_methods" mapstructure:"allowed_methods"`
	ExposeHeaders         []string `json:"expose_headers" yaml:"expose_headers" mapstructure:"expose_headers"`
	AllowCredentials      bool     `json:"allow_credentials" yaml:"allow_credentials" mapstructure:"allow_credentials"`
	MaxRequestBodySize    int64    `json:"max_request_body_size" yaml:"max_request_body_size" mapstructure:"max_request_body_size"`
	RequireHTTPS          bool     `json:"require_https" yaml:"require_https" mapstructure:"require_https"`
	EnableSecurityHeaders bool     `json:"enable_security_headers" yaml:"enable_security_headers" mapstructure:"enable_security_headers"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Path    string `json:"path" yaml:"path"`
}

// TracingConfig holds tracing configuration
type TracingConfig struct {
	Enabled        bool    `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	JaegerEndpoint string  `json:"jaeger_endpoint" yaml:"jaeger_endpoint" mapstructure:"jaeger_endpoint"`
	SampleRate     float64 `json:"sample_rate" yaml:"sample_rate" mapstructure:"sample_rate"`
	ServiceName    string  `json:"service_name" yaml:"service_name" mapstructure:"service_name"`
	ServiceVersion string  `json:"service_version" yaml:"service_version" mapstructure:"service_version"`
	Environment    string  `json:"environment" yaml:"environment" mapstructure:"environment"`
}

// StorageConfig holds storage configuration
type StorageConfig struct {
	Provider        string `json:"provider" yaml:"provider" mapstructure:"provider"`
	Endpoint        string `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
	Region          string `json:"region" yaml:"region" mapstructure:"region"`
	Bucket          string `json:"bucket" yaml:"bucket" mapstructure:"bucket"`
	AccessKeyID     string `json:"access_key_id" yaml:"access_key_id" mapstructure:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key" yaml:"secret_access_key" mapstructure:"secret_access_key"`
	CDNEndpoint     string `json:"cdn_endpoint" yaml:"cdn_endpoint" mapstructure:"cdn_endpoint"`
	ForcePathStyle  bool   `json:"force_path_style" yaml:"force_path_style" mapstructure:"force_path_style"`
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	Enabled         bool          `json:"enabled" yaml:"enabled"`
	Path            string        `json:"path" yaml:"path"`
	ReadBufferSize  int           `json:"read_buffer_size" yaml:"read_buffer_size"`
	WriteBufferSize int           `json:"write_buffer_size" yaml:"write_buffer_size"`
	MaxMessageSize  int64         `json:"max_message_size" yaml:"max_message_size"`
	PingInterval    time.Duration `json:"ping_interval" yaml:"ping_interval"`
	PongTimeout     time.Duration `json:"pong_timeout" yaml:"pong_timeout"`
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries      int           `json:"max_retries" yaml:"max_retries"`
	InitialInterval time.Duration `json:"initial_interval" yaml:"initial_interval"`
	MaxInterval     time.Duration `json:"max_interval" yaml:"max_interval"`
	Multiplier      float64       `json:"multiplier" yaml:"multiplier"`
	Jitter          bool          `json:"jitter" yaml:"jitter"`
}

// VersioningConfig holds API versioning configuration
type VersioningConfig struct {
	DefaultVersion string                     `json:"default_version" yaml:"default_version"`
	Versions       map[string]VersionSettings `json:"versions" yaml:"versions"`
}

// VersionSettings holds settings for a specific API version
type VersionSettings struct {
	Deprecated bool       `json:"deprecated" yaml:"deprecated"`
	SunsetDate *time.Time `json:"sunset_date" yaml:"sunset_date"`
}

// TransformConfig holds transformation configuration
type TransformConfig struct {
	Enabled bool                            `json:"enabled" yaml:"enabled"`
	Rules   map[string][]TransformationRule `json:"rules" yaml:"rules"`
}

// TransformationRule defines a transformation rule
type TransformationRule struct {
	Path      string                 `json:"path" yaml:"path"`
	Transform string                 `json:"transform" yaml:"transform"`
	Options   map[string]interface{} `json:"options" yaml:"options"`
}

// Manager handles configuration management with hot-reload
type Manager struct {
	config     *Config
	configPath string
	mu         sync.RWMutex
	logger     logger.Logger
	watchers   []ConfigWatcher
	watcher    *fsnotify.Watcher
	stopCh     chan struct{}
}

// ConfigWatcher is called when configuration changes
type ConfigWatcher func(old, new *Config)

func Load() (*Config, error) {
	// Determine if we're running locally (developer machine) or deployed
	isLocalMode := false

	// Try to load .env or config.env for local development
	envPaths := []string{".env", "config.env"}
	for _, path := range envPaths {
		if err := godotenv.Load(path); err == nil {
			fmt.Printf("Loaded environment from: %s (local development mode)\n", path)
			isLocalMode = true
			break
		}
	}

	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = os.Getenv("ENV")
	}

	// Check if this is truly local development (not deployed)
	if env == "local" {
		isLocalMode = true
	}

	// Always set structure defaults for viper to work correctly
	// These will be overridden by environment variables from ConfigMaps/Secrets
	setViperDefaults()

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Explicitly bind environment variables (following ENVIRONMENT_VARIABLES_STANDARD.md)
	_ = viper.BindEnv("server.port", "SERVER_PORT")
	_ = viper.BindEnv("redis.host", "REDIS_HOST")
	_ = viper.BindEnv("redis.port", "REDIS_PORT")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("redis.db", "REDIS_DB")

	// Storage
	_ = viper.BindEnv("storage.provider", "STORAGE_PROVIDER")
	_ = viper.BindEnv("storage.endpoint", "STORAGE_ENDPOINT")
	_ = viper.BindEnv("storage.region", "STORAGE_REGION")
	_ = viper.BindEnv("storage.bucket", "STORAGE_BUCKET")
	_ = viper.BindEnv("storage.access_key_id", "STORAGE_ACCESS_KEY_ID")
	_ = viper.BindEnv("storage.secret_access_key", "STORAGE_SECRET_ACCESS_KEY")
	_ = viper.BindEnv("storage.cdn_endpoint", "STORAGE_CDN_ENDPOINT")
	_ = viper.BindEnv("storage.force_path_style", "STORAGE_FORCE_PATH_STYLE")

	// Feature flags
	_ = viper.BindEnv("terminal.enable_new_auth", "ENABLE_NEW_TERMINAL_AUTH")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Override JWT secret from environment if set
	if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
		config.Security.JWTSecret = jwtSecret
	}

	if origins := viper.GetString("security.allowed_origins"); origins != "" && len(config.Security.AllowedOrigins) == 0 {
		config.Security.AllowedOrigins = strings.Split(origins, ",")
	}
	if headers := viper.GetString("security.allowed_headers"); headers != "" && len(config.Security.AllowedHeaders) == 0 {
		config.Security.AllowedHeaders = strings.Split(headers, ",")
	}
	if methods := viper.GetString("security.allowed_methods"); methods != "" && len(config.Security.AllowedMethods) == 0 {
		config.Security.AllowedMethods = strings.Split(methods, ",")
	}
	if exposeHeaders := viper.GetString("security.expose_headers"); exposeHeaders != "" && len(config.Security.ExposeHeaders) == 0 {
		config.Security.ExposeHeaders = strings.Split(exposeHeaders, ",")
	}

	// Validate configuration based on environment
	// - local: skip validation (developer machine with defaults)
	// - development: skip validation (deployed dev with ConfigMaps)
	// - staging/production: strict validation required
	if !isLocalMode && env != "development" {
		if err := validateProductionConfig(&config); err != nil {
			return nil, fmt.Errorf("production configuration validation failed: %w", err)
		}
	}

	if err := validateSecurityConfig(&config.Security); err != nil {
		return nil, fmt.Errorf("security configuration validation failed: %w", err)
	}

	if len(config.Services) == 0 {
		config.Services = getDefaultServices()
	}

	return &config, nil
}

// GetRedisAddr returns the Redis address in host:port format
func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%s", c.Redis.Host, c.Redis.Port)
}

func setViperDefaults() {
	viper.SetDefault("server.port", "4000")
	viper.SetDefault("server.read_timeout", 60*time.Second)  // Increased for file uploads
	viper.SetDefault("server.write_timeout", 60*time.Second) // Increased for file uploads
	viper.SetDefault("server.idle_timeout", 60*time.Second)
	viper.SetDefault("server.max_header_bytes", 1<<20)
	viper.SetDefault("server.shutdown_timeout", 30*time.Second)
	viper.SetDefault("server.log_level", "debug")
	viper.SetDefault("server.log_format", "json")
	viper.SetDefault("server.log_file", "logs/api-gateway.log")

	// Cache defaults
	viper.SetDefault("cache.enabled", true)
	viper.SetDefault("cache.ttl", 5*time.Minute)
	viper.SetDefault("cache.max_size", 1024*1024)
	viper.SetDefault("cache.redis_url", "redis://localhost:6379")

	// Rate limiting defaults
	viper.SetDefault("rate_limit.enabled", true)
	viper.SetDefault("rate_limit.requests_per_min", 100)
	viper.SetDefault("rate_limit.burst_size", 10)

	// Security defaults - IMPORTANT: Same as admin-management service
	// JWT secret should be set via environment variable for security
	viper.SetDefault("security.jwt_secret", "") // Must be set via env var
	viper.SetDefault("security.jwt_issuer", "randco-api-gateway")
	viper.SetDefault("security.allowed_origins", []string{
		"http://localhost:5173",
		"http://localhost:6176",
		"http://localhost:6177",
		"http://localhost:6178",
		"http://localhost:5174",
		"http://localhost:3000",
	})
	viper.SetDefault("security.allowed_headers", []string{
		"Accept",
		"Authorization",
		"Content-Type",
		"X-Request-ID",
	})
	viper.SetDefault("security.allowed_methods", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	viper.SetDefault("security.expose_headers", []string{"X-Total-Count", "X-Page-Count", "X-Request-ID"})
	viper.SetDefault("security.allow_credentials", true)
	viper.SetDefault("security.max_request_body_size", int64(10*1024*1024)) // 10MB default
	viper.SetDefault("security.require_https", false)                       // Enable in production
	viper.SetDefault("security.enable_security_headers", true)

	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.path", "/metrics")

	// WebSocket defaults
	viper.SetDefault("websocket.enabled", true)
	viper.SetDefault("websocket.path", "/ws")
	viper.SetDefault("websocket.read_buffer_size", 1024)
	viper.SetDefault("websocket.write_buffer_size", 1024)
	viper.SetDefault("websocket.max_message_size", 512*1024)
	viper.SetDefault("websocket.ping_interval", 30*time.Second)
	viper.SetDefault("websocket.pong_timeout", 60*time.Second)

	// Retry defaults
	viper.SetDefault("retry.max_retries", 3)
	viper.SetDefault("retry.initial_interval", 100*time.Millisecond)
	viper.SetDefault("retry.max_interval", 5*time.Second)
	viper.SetDefault("retry.multiplier", 2.0)
	viper.SetDefault("retry.jitter", true)

	// Tracing defaults
	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.jaeger_endpoint", "http://localhost:4318") // OTLP HTTP endpoint
	viper.SetDefault("tracing.sample_rate", 1.0)                         // 100% sampling for development
	viper.SetDefault("tracing.service_name", "api-gateway")
	viper.SetDefault("tracing.service_version", "1.0.0")
	viper.SetDefault("tracing.environment", "development")

	// Additional security defaults
	viper.SetDefault("security.max_request_body_size", 10*1024*1024) // 10MB
	viper.SetDefault("security.require_https", false)                // Set to true in production
	viper.SetDefault("security.enable_security_headers", true)

	// Storage defaults (for file uploads - DigitalOcean Spaces)
	viper.SetDefault("storage.provider", "spaces")
	viper.SetDefault("storage.endpoint", "https://sgp1.digitaloceanspaces.com")
	viper.SetDefault("storage.region", "sgp1")
	viper.SetDefault("storage.bucket", "rand-dev-static")
	viper.SetDefault("storage.access_key_id", "")
	viper.SetDefault("storage.secret_access_key", "")
	viper.SetDefault("storage.cdn_endpoint", "")
	viper.SetDefault("storage.force_path_style", false)

	// Feature flags
	viper.SetDefault("terminal.enable_new_auth", false)
}

func validateProductionConfig(config *Config) error {
	if config.Server.Port == "" {
		return fmt.Errorf("SERVER_PORT must be set in production")
	}

	if config.Cache.Enabled && (config.Redis.Host == "" || config.Redis.Port == "") {
		return fmt.Errorf("REDIS_HOST and REDIS_PORT must be set in production when cache is enabled")
	}

	if len(config.Services) == 0 {
		return fmt.Errorf("services must be configured in production")
	}

	if config.Tracing.Enabled && config.Tracing.JaegerEndpoint == "" {
		return fmt.Errorf("TRACING_JAEGER_ENDPOINT must be set in production when tracing is enabled")
	}

	if !config.Security.RequireHTTPS {
		fmt.Printf("Warning: HTTPS is not required - ensure TLS termination is handled by infrastructure\n")
	}

	return nil
}

func validateSecurityConfig(config *SecurityConfig) error {
	if config.JWTSecret == "" || config.JWTSecret == "change-this-secret-in-production" {
		return fmt.Errorf("JWT secret must be set via JWT_SECRET environment variable")
	}

	if len(config.JWTSecret) < 32 {
		return fmt.Errorf("JWT secret must be at least 32 characters long")
	}

	isProduction := os.Getenv("ENVIRONMENT") == "production" || os.Getenv("ENV") == "prod"
	if isProduction {
		for _, origin := range config.AllowedOrigins {
			if origin == "*" {
				return fmt.Errorf("wildcard origin '*' is not allowed in production")
			}
		}
		config.RequireHTTPS = true
		config.EnableSecurityHeaders = true
	}

	if config.MaxRequestBodySize == 0 {
		config.MaxRequestBodySize = 10 * 1024 * 1024
	}

	return nil
}

func getDefaultServices() []ServiceConfig {
	viper.SetDefault("services.admin_management.host", "localhost")
	viper.SetDefault("services.admin_management.port", "50057")
	viper.SetDefault("services.agent_auth.host", "localhost")
	viper.SetDefault("services.agent_auth.port", "50052")
	viper.SetDefault("services.agent_management.host", "localhost")
	viper.SetDefault("services.agent_management.port", "50056")
	viper.SetDefault("services.game.host", "localhost")
	viper.SetDefault("services.game.port", "50053")
	viper.SetDefault("services.wallet.host", "localhost")
	viper.SetDefault("services.wallet.port", "50059")
	viper.SetDefault("services.terminal.host", "localhost")
	viper.SetDefault("services.terminal.port", "50054")
	viper.SetDefault("services.payment.host", "localhost")
	viper.SetDefault("services.payment.port", "50061")
	viper.SetDefault("services.draw.host", "localhost")
	viper.SetDefault("services.draw.port", "50060")
	viper.SetDefault("services.ticket.host", "localhost")
	viper.SetDefault("services.ticket.port", "50062")
	viper.SetDefault("services.notification.host", "localhost")
	viper.SetDefault("services.notification.port", "50063")
	viper.SetDefault("services.player.host", "localhost")
	viper.SetDefault("services.player.port", "50064")

	adminHost := viper.GetString("services.admin_management.host")
	adminPort := viper.GetString("services.admin_management.port")
	agentAuthHost := viper.GetString("services.agent_auth.host")
	agentAuthPort := viper.GetString("services.agent_auth.port")
	agentMgmtHost := viper.GetString("services.agent_management.host")
	agentMgmtPort := viper.GetString("services.agent_management.port")
	gameHost := viper.GetString("services.game.host")
	gamePort := viper.GetString("services.game.port")

	return []ServiceConfig{
		{
			Name:       "admin-management",
			URL:        adminHost + ":" + adminPort,
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Weight:     1,
		},
		{
			Name:       "agent-auth",
			URL:        agentAuthHost + ":" + agentAuthPort,
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Weight:     1,
		},
		{
			Name:       "agent-management",
			URL:        agentMgmtHost + ":" + agentMgmtPort,
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Weight:     1,
		},
		{
			Name:       "game",
			URL:        gameHost + ":" + gamePort,
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Weight:     1,
		},
		{
			Name:       "wallet",
			URL:        viper.GetString("services.wallet.host") + ":" + viper.GetString("services.wallet.port"),
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Weight:     1,
		},
		{
			Name:       "terminal",
			URL:        viper.GetString("services.terminal.host") + ":" + viper.GetString("services.terminal.port"),
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Weight:     1,
		},
		{
			Name:       "payment",
			URL:        viper.GetString("services.payment.host") + ":" + viper.GetString("services.payment.port"),
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Weight:     1,
		},
		{
			Name:       "draw",
			URL:        viper.GetString("services.draw.host") + ":" + viper.GetString("services.draw.port"),
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Weight:     1,
		},
		{
			Name:       "ticket",
			URL:        viper.GetString("services.ticket.host") + ":" + viper.GetString("services.ticket.port"),
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Weight:     1,
		},
		{
			Name:       "notification",
			URL:        viper.GetString("services.notification.host") + ":" + viper.GetString("services.notification.port"),
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Weight:     1,
		},
		{
			Name:       "player",
			URL:        viper.GetString("services.player.host") + ":" + viper.GetString("services.player.port"),
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Weight:     1,
		},
	}
}

// NewManager creates a new configuration manager
func NewManager(configPath string, logger logger.Logger) (*Manager, error) {
	m := &Manager{
		configPath: configPath,
		logger:     logger,
		watchers:   make([]ConfigWatcher, 0),
		stopCh:     make(chan struct{}),
	}

	// Load initial configuration
	if err := m.Load(); err != nil {
		return nil, err
	}

	// Start watching for changes
	if err := m.startWatcher(); err != nil {
		// If watcher fails, log warning but don't fail - hot reload is optional
		logger.Warn("Failed to start config watcher, hot reload disabled", "error", err)
	}

	return m, nil
}

// Load loads configuration from file
func (m *Manager) Load() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config

	// Determine file format
	if isJSON(m.configPath) {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	} else if isYAML(m.configPath) {
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse YAML config: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported config format")
	}

	// Apply environment variable overrides
	m.applyEnvOverrides(&config)

	// Validate configuration
	if err := m.validate(&config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Update configuration
	m.mu.Lock()
	oldConfig := m.config
	m.config = &config
	m.mu.Unlock()

	// Notify watchers
	if oldConfig != nil {
		m.notifyWatchers(oldConfig, &config)
	}

	m.logger.Info("Configuration loaded", "path", m.configPath)
	return nil
}

// Get returns current configuration
func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Watch registers a configuration watcher
func (m *Manager) Watch(watcher ConfigWatcher) {
	m.watchers = append(m.watchers, watcher)
}

// startWatcher starts file system watcher for hot-reload
func (m *Manager) startWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	m.watcher = watcher

	// Add the config file to watch before starting the goroutine
	if err := watcher.Add(m.configPath); err != nil {
		_ = watcher.Close() // Clean up on error, ignore close error
		return fmt.Errorf("failed to watch config file: %w", err)
	}

	go func() {
		defer func() {
			// Ensure watcher is closed if goroutine exits
			if m.watcher != nil {
				_ = m.watcher.Close()
			}
		}()

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					m.logger.Info("Configuration file changed, reloading", "file", event.Name)
					if err := m.Load(); err != nil {
						m.logger.Error("Failed to reload configuration", "error", err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				m.logger.Error("Config watcher error", "error", err)
			case <-m.stopCh:
				return
			}
		}
	}()

	return nil
}

// Stop stops the configuration manager
func (m *Manager) Stop() {
	// Signal stop to the watcher goroutine
	if m.stopCh != nil {
		select {
		case <-m.stopCh:
			// Already closed
		default:
			close(m.stopCh)
		}
	}

	// Close watcher if it exists
	if m.watcher != nil {
		_ = m.watcher.Close()
		m.watcher = nil
	}
}

// notifyWatchers notifies all registered watchers
func (m *Manager) notifyWatchers(old, new *Config) {
	for _, watcher := range m.watchers {
		go watcher(old, new)
	}
}

// applyEnvOverrides applies environment variable overrides
func (m *Manager) applyEnvOverrides(config *Config) {
	// Server overrides
	if port := os.Getenv("GATEWAY_PORT"); port != "" {
		config.Server.Port = port
	}

	// Additional security overrides if needed

	// Service overrides
	for i, service := range config.Services {
		envKey := fmt.Sprintf("%s_URL", service.Name)
		if url := os.Getenv(envKey); url != "" {
			config.Services[i].URL = url
		}
	}
}

// validate validates configuration
func (m *Manager) validate(config *Config) error {
	// Validate server config
	if config.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}

	// Validate cache config
	if config.Cache.Enabled && (config.Redis.Host == "" || config.Redis.Port == "") {
		return fmt.Errorf("redis host and port are required when cache is enabled")
	}

	// Validate service configs
	for _, service := range config.Services {
		if service.Name == "" || service.URL == "" {
			return fmt.Errorf("service name and URL are required")
		}
	}

	// Validate security config
	if config.Security.JWTSecret == "" {
		return fmt.Errorf("JWT secret is required")
	}

	return nil
}

// isJSON checks if file is JSON
func isJSON(path string) bool {
	return len(path) > 5 && path[len(path)-5:] == ".json"
}

// isYAML checks if file is YAML
func isYAML(path string) bool {
	return (len(path) > 4 && path[len(path)-4:] == ".yml") ||
		(len(path) > 5 && path[len(path)-5:] == ".yaml")
}

// Default returns default configuration
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            ":4000",
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    15 * time.Second,
			IdleTimeout:     60 * time.Second,
			MaxHeaderBytes:  1 << 20,
			ShutdownTimeout: 30 * time.Second,
		},
		Cache: CacheConfig{
			Enabled: true,
			TTL:     5 * time.Minute,
			MaxSize: 1024 * 1024,
		},
		Redis: RedisConfig{
			Host:     "localhost",
			Port:     "6379",
			Password: "",
			DB:       0,
		},
		RateLimit: RateLimitConfig{
			Enabled:        true,
			RequestsPerMin: 100,
			BurstSize:      10,
		},
		Security: SecurityConfig{
			JWTSecret:      "change-this-secret-in-production",
			AllowedOrigins: []string{"*"},
			AllowedHeaders: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
		},
		WebSocket: WebSocketConfig{
			Enabled:         true,
			Path:            "/ws",
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			MaxMessageSize:  512 * 1024,
			PingInterval:    54 * time.Second,
			PongTimeout:     60 * time.Second,
		},
		Retry: RetryConfig{
			MaxRetries:      3,
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     10 * time.Second,
			Multiplier:      2.0,
			Jitter:          true,
		},
		Versioning: VersioningConfig{
			DefaultVersion: "v1",
		},
		Transform: TransformConfig{
			Enabled: false,
		},
	}
}

package config

import (
	"time"
)

// Config holds application configuration
type Config struct {
	// Server
	Port int
	Env  string
	
	// JWT
	JWTAccessSecret    string
	JWTRefreshSecret   string
	JWTAccessDuration  time.Duration
	JWTRefreshDuration time.Duration
	
	// Database
	DatabaseURL      string
	DatabaseMaxConns int
	DatabaseMinConns int
	
	// Redis
	RedisHost     string
	RedisPort     int
	RedisPassword string
	RedisDB       int
	
	// Kafka
	KafkaBrokers []string
	KafkaGroupID string
	
	// Service Discovery
	ServiceRegistryURL string
	
	// Tracing
	JaegerEndpoint string
	
	// Metrics
	MetricsEnabled bool
	MetricsPort    int
}
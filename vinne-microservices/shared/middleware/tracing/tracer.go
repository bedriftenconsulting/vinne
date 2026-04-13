package tracing

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	JaegerEndpoint string
	SampleRate     float64
}

func InitTracer(cfg Config) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()
	
	// Convert old Jaeger endpoint format to OTLP format if needed
	endpoint := cfg.JaegerEndpoint
	if strings.Contains(endpoint, "/api/traces") {
		endpoint = strings.Replace(endpoint, ":14268/api/traces", ":4318", 1)
		endpoint = strings.Replace(endpoint, "/api/traces", "", 1)
	}
	
	// Create OTLP HTTP exporter
	client := otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint(strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")),
		otlptracehttp.WithInsecure(),
	)
	
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("environment", cfg.Environment),
			attribute.String("service.namespace", "randco"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	// Create tracer provider with sampling
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return tp, nil
}

// Helper functions for common span attributes
func UserAttributes(userID, role string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("user.id", userID),
		attribute.String("user.role", role),
	}
}

func RequestAttributes(method, path, clientIP string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.path", path),
		attribute.String("client.ip", clientIP),
	}
}

func DatabaseAttributes(dbType, operation, table string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("db.type", dbType),
		attribute.String("db.operation", operation),
		attribute.String("db.table", table),
	}
}

// GetSampleRate returns the appropriate sample rate based on environment
func GetSampleRate() float64 {
	env := os.Getenv("ENVIRONMENT")
	switch env {
	case "production":
		return 0.01
	case "staging":
		return 0.05
	default:
		return 0.1
	}
}

// IsTracingEnabled checks if tracing is enabled via environment variable
func IsTracingEnabled() bool {
	return os.Getenv("TRACING_ENABLED") != "false"
}
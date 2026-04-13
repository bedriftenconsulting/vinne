package tracing

import (
	"context"
	"fmt"
	"strings"

	"github.com/randco/randco-microservices/services/service-notification/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

type Provider struct {
	tracer trace.Tracer
	config config.TracingConfig
}

func NewProvider(ctx context.Context, cfg config.TracingConfig) (*Provider, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
			attribute.String("region", cfg.Region),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	switch cfg.ExporterType {
	case "otlp":
		exporter, err = createOTLPExporter(ctx, cfg.ExporterConfig)
	case "stdout":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	default:
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRate))),
	)

	otel.SetTracerProvider(tp)

	return &Provider{
		tracer: tp.Tracer(cfg.ServiceName),
		config: cfg,
	}, nil
}

func createOTLPExporter(ctx context.Context, config map[string]string) (sdktrace.SpanExporter, error) {
	endpoint := config["endpoint"]
	if endpoint == "" {
		endpoint = "localhost:4317"
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
	}

	if insecure, ok := config["insecure"]; ok && insecure == "true" {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	if headers, ok := config["headers"]; ok && headers != "" {
		headerMap := parseHeaders(headers)
		opts = append(opts, otlptracegrpc.WithHeaders(headerMap))
	}

	return otlptracegrpc.New(ctx, opts...)
}

func parseHeaders(headerStr string) map[string]string {
	headers := make(map[string]string)
	pairs := strings.Split(headerStr, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return headers
}

func (p *Provider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return p.tracer.Start(ctx, name, opts...)
}

func (p *Provider) Shutdown(ctx context.Context) error {
	if tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		return tp.Shutdown(ctx)
	}
	return nil
}

func (p *Provider) GetTracer() trace.Tracer {
	return p.tracer
}

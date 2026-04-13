package tracing

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware returns HTTP middleware for tracing
func HTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, serviceName,
			otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
		)
	}
}

// HTTPTransport returns an HTTP transport with tracing
func HTTPTransport() http.RoundTripper {
	return otelhttp.NewTransport(http.DefaultTransport)
}

// StartSpanFromContext creates a new span from the context
func StartSpanFromContext(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := otel.Tracer("randco")
	return tracer.Start(ctx, name, opts...)
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
}

// SetStatus sets the status of the current span
func SetStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	span.SetStatus(code, description)
}

// SetAttributes adds attributes to the current span
func SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// ExtractTraceID gets the trace ID from the context
func ExtractTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// CacheAttributes returns attributes for cache operations
func CacheAttributes(operation, key string, hit bool) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("cache.operation", operation),
		attribute.String("cache.key", key),
		attribute.Bool("cache.hit", hit),
	}
}

// ErrorAttributes returns attributes for errors
func ErrorAttributes(err error) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("error.type", fmt.Sprintf("%T", err)),
		attribute.String("error.message", err.Error()),
	}
}

// InjectHTTPHeaders injects trace context into HTTP headers
func InjectHTTPHeaders(ctx context.Context, req *http.Request) {
	otel.GetTextMapPropagator().Inject(ctx, HeaderCarrier(req.Header))
}

// ExtractHTTPHeaders extracts trace context from HTTP headers
func ExtractHTTPHeaders(ctx context.Context, req *http.Request) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, HeaderCarrier(req.Header))
}

// HeaderCarrier adapts http.Header to satisfy the TextMapCarrier interface
type HeaderCarrier http.Header

// Get returns the value associated with the passed key
func (hc HeaderCarrier) Get(key string) string {
	return http.Header(hc).Get(key)
}

// Set stores the key-value pair
func (hc HeaderCarrier) Set(key string, value string) {
	http.Header(hc).Set(key, value)
}

// Keys lists the keys stored in this carrier
func (hc HeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(hc))
	for k := range http.Header(hc) {
		keys = append(keys, k)
	}
	return keys
}
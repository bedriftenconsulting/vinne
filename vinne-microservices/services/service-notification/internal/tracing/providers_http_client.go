package tracing

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	providerInstrumentationName = "github.com/randco/randco-microservices/services/service-notification/providers"
)

type TracedHTTPClient struct {
	client  *http.Client
	tracer  trace.Tracer
	service string
}

func NewTracedHTTPClient(service string, baseClient *http.Client) *TracedHTTPClient {
	if baseClient == nil {
		baseClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	// Wrap the base client with otelhttp.Transport for automatic HTTP tracing
	baseClient.Transport = otelhttp.NewTransport(baseClient.Transport,
		otelhttp.WithTracerProvider(otel.GetTracerProvider()),
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return fmt.Sprintf("provider.%s.http %s", strings.ToLower(service), strings.ToLower(r.Method))
		}),
	)

	return &TracedHTTPClient{
		client:  baseClient,
		tracer:  otel.GetTracerProvider().Tracer(providerInstrumentationName),
		service: service,
	}
}

func (c *TracedHTTPClient) Do(req *http.Request) (*http.Response, error) {
	ctx, span := c.tracer.Start(req.Context(), fmt.Sprintf("provider.%s.request", c.service),
		trace.WithAttributes(
			attribute.String("provider.name", c.service),
			semconv.HTTPMethodKey.String(req.Method),
			semconv.HTTPURLKey.String(req.URL.String()),
			semconv.HTTPRequestContentLengthKey.Int64(req.ContentLength),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	// Add correlation ID if present in context
	if correlationID := req.Context().Value("correlation-id"); correlationID != nil {
		span.SetAttributes(attribute.String("correlation.id", correlationID.(string)))
	}

	// Add request ID if present in context
	if requestID := req.Context().Value("request-id"); requestID != nil {
		span.SetAttributes(attribute.String("request.id", requestID.(string)))
	}

	// Add user agent
	if userAgent := req.Header.Get("User-Agent"); userAgent != "" {
		span.SetAttributes(semconv.HTTPUserAgentKey.String(userAgent))
	}

	// Execute the request
	resp, err := c.client.Do(req.WithContext(ctx))

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(
		semconv.HTTPStatusCodeKey.Int(resp.StatusCode),
		semconv.HTTPResponseContentLengthKey.Int64(resp.ContentLength),
	)

	// Add response headers as attributes (excluding sensitive ones)
	for key, values := range resp.Header {
		if len(values) > 0 && !isSensitiveHeader(key) {
			span.SetAttributes(attribute.String("http.response.header."+key, values[0]))
		}
	}

	if resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP error: %d", resp.StatusCode))
		span.SetAttributes(attribute.String("http.error", resp.Status))
	} else {
		span.SetStatus(codes.Ok, "HTTP request successful")
	}

	return resp, nil
}

func (c *TracedHTTPClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func (c *TracedHTTPClient) Post(url, contentType string, body interface{}) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(fmt.Sprintf("%v", body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(req)
}

func TraceEmailProviderCall(ctx context.Context, provider string, operation string, fn func(context.Context) error) error {
	ctx, span := otel.GetTracerProvider().Tracer(providerInstrumentationName).Start(ctx, fmt.Sprintf("provider.email.%s.%s", provider, operation),
		trace.WithAttributes(
			attribute.String("provider.type", "email"),
			attribute.String("provider.name", provider),
			attribute.String("provider.operation", operation),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return err
}

func TraceSMSProviderCall(ctx context.Context, provider string, operation string, fn func(context.Context) error) error {
	ctx, span := otel.GetTracerProvider().Tracer(providerInstrumentationName).Start(ctx, fmt.Sprintf("provider.sms.%s.%s", provider, operation),
		trace.WithAttributes(
			attribute.String("provider.type", "sms"),
			attribute.String("provider.name", provider),
			attribute.String("provider.operation", operation),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return err
}

func TraceProviderValidation(ctx context.Context, providerType string, provider string, target string, fn func(context.Context) error) error {
	ctx, span := otel.GetTracerProvider().Tracer(providerInstrumentationName).Start(ctx, fmt.Sprintf("provider.%s.%s.validate", providerType, provider),
		trace.WithAttributes(
			attribute.String("provider.type", providerType),
			attribute.String("provider.name", provider),
			attribute.String("provider.operation", "validate"),
			attribute.String("provider.target", maskSensitiveData(target)),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return err
}

func TraceProviderStatusCheck(ctx context.Context, providerType string, provider string, messageID string, fn func(context.Context) error) error {
	ctx, span := otel.GetTracerProvider().Tracer(providerInstrumentationName).Start(ctx, fmt.Sprintf("provider.%s.%s.status", providerType, provider),
		trace.WithAttributes(
			attribute.String("provider.type", providerType),
			attribute.String("provider.name", provider),
			attribute.String("provider.operation", "status_check"),
			attribute.String("provider.message_id", messageID),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return err
}

func maskSensitiveData(data string) string {
	if len(data) == 0 {
		return ""
	}

	// Mask email addresses
	if strings.Contains(data, "@") {
		parts := strings.Split(data, "@")
		if len(parts) == 2 {
			if len(parts[0]) > 2 {
				return parts[0][:2] + "***@" + parts[1]
			}
			return "***@" + parts[1]
		}
	}

	// Mask phone numbers (basic pattern)
	if len(data) > 6 {
		return data[:3] + "***" + data[len(data)-3:]
	}

	// For other data, mask middle part
	if len(data) > 4 {
		return data[:2] + "***" + data[len(data)-2:]
	}

	return "***"
}

func isSensitiveHeader(key string) bool {
	sensitiveHeaders := []string{
		"authorization",
		"x-api-key",
		"x-auth-token",
		"cookie",
		"set-cookie",
		"x-csrf-token",
		"x-access-token",
		"bearer",
		"x-mailgun-api-key",
		"x-hubtel-api-key",
	}

	lowerKey := strings.ToLower(key)
	for _, sensitive := range sensitiveHeaders {
		if lowerKey == sensitive || strings.Contains(lowerKey, sensitive) {
			return true
		}
	}
	return false
}

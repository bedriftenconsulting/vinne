package tracing

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	helpersInstrumentationName = "github.com/randco/randco-microservices/services/service-notification"
)

func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := otel.GetTracerProvider().Tracer(helpersInstrumentationName)
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func WithSpan(ctx context.Context, name string, fn func(context.Context) error, attrs ...attribute.KeyValue) error {
	ctx, span := StartSpan(ctx, name, attrs...)
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

func RecordDuration(ctx context.Context, name string, start time.Time, attrs ...attribute.KeyValue) {
	_, span := StartSpan(ctx, name, attrs...)
	defer span.End()

	duration := time.Since(start)
	span.SetAttributes(attribute.Int64("duration_ms", duration.Milliseconds()))
	span.SetStatus(codes.Ok, "")
}

func AddAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

func RecordError(ctx context.Context, err error, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err, trace.WithAttributes(attrs...))
		span.SetStatus(codes.Error, err.Error())
	}
}

func SetSpanStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetStatus(code, description)
	}
}

type ctxKey string

const (
	traceIDKey ctxKey = "rand_trace_id"
	spanIDKey  ctxKey = "rand_span_id"
)

func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDKey).(string); ok && traceID != "" {
		return traceID
	}

	span := trace.SpanFromContext(ctx)
	spanContext := span.SpanContext()

	if !spanContext.IsValid() || !spanContext.HasTraceID() {
		return ""
	}

	return spanContext.TraceID().String()
}

func GetSpanID(ctx context.Context) string {
	if spanID, ok := ctx.Value(spanIDKey).(string); ok && spanID != "" {
		return spanID
	}

	span := trace.SpanFromContext(ctx)
	spanContext := span.SpanContext()

	if !spanContext.IsValid() || !spanContext.HasSpanID() {
		return ""
	}

	return spanContext.SpanID().String()
}

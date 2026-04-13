package tracing

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestStartSpan(t *testing.T) {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(tracetest.NewInMemoryExporter()),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())

	// Test span creation
	ctx, span := StartSpan(context.Background(), "test.operation",
		attribute.String("test.key", "test.value"),
	)
	defer span.End()

	// Verify span is created
	if span == nil {
		t.Error("Expected span to be created")
	}

	// Verify context contains span
	spanFromCtx := trace.SpanFromContext(ctx)
	if spanFromCtx == nil {
		t.Error("Expected span to be in context")
	}
}

func TestWithSpan(t *testing.T) {
	// Setup test tracer
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(tracetest.NewInMemoryExporter()),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())

	// Test successful operation
	err := WithSpan(context.Background(), "test.success", func(ctx context.Context) error {
		return nil
	}, attribute.String("test.key", "test.value"))

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test error operation
	testErr := errors.New("test error")
	err = WithSpan(context.Background(), "test.error", func(ctx context.Context) error {
		return testErr
	})

	if err != testErr {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}
}

func TestRecordDuration(t *testing.T) {
	// Setup test tracer
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(tracetest.NewInMemoryExporter()),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())

	// Test duration recording
	start := time.Now()
	time.Sleep(10 * time.Millisecond) // Small delay to ensure duration > 0

	RecordDuration(context.Background(), "test.duration", start,
		attribute.String("test.key", "test.value"),
	)
}

func TestAddAttributes(t *testing.T) {
	// Setup test tracer
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(tracetest.NewInMemoryExporter()),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())

	// Create a span
	ctx, span := StartSpan(context.Background(), "test.attributes")
	defer span.End()

	// Add attributes
	AddAttributes(ctx,
		attribute.String("test.key1", "value1"),
		attribute.Int("test.key2", 42),
	)

	// Verify attributes were added
	if sdkSpan, ok := span.(sdktrace.ReadOnlySpan); ok {
		attrs := sdkSpan.Attributes()
		if len(attrs) < 2 {
			t.Errorf("Expected at least 2 attributes, got %d", len(attrs))
		}
	}
}

func TestRecordError(t *testing.T) {
	// Setup test tracer
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(tracetest.NewInMemoryExporter()),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())

	// Create a span
	ctx, span := StartSpan(context.Background(), "test.error")
	defer span.End()

	// Record an error
	testErr := errors.New("test error")
	RecordError(ctx, testErr, attribute.String("error.context", "test"))

	// Verify error was recorded
	if sdkSpan, ok := span.(sdktrace.ReadOnlySpan); ok {
		if sdkSpan.Status().Code != codes.Error {
			t.Error("Expected span to have error status")
		}
	}
}

func TestSetSpanStatus(t *testing.T) {
	// Setup test tracer
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(tracetest.NewInMemoryExporter()),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())

	// Create a span
	ctx, span := StartSpan(context.Background(), "test.status")
	defer span.End()

	// Set status
	SetSpanStatus(ctx, codes.Error, "test error")

	// Verify status was set
	if sdkSpan, ok := span.(sdktrace.ReadOnlySpan); ok {
		if sdkSpan.Status().Code != codes.Error {
			t.Error("Expected span to have error status")
		}
	}
}

func TestGetTraceID(t *testing.T) {
	// Setup test tracer
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(tracetest.NewInMemoryExporter()),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())

	// Create a span
	ctx, span := StartSpan(context.Background(), "test.trace_id")
	defer span.End()

	// Get trace ID
	traceID := GetTraceID(ctx)
	if traceID == "" {
		t.Error("Expected trace ID to be non-empty")
	}

	// Verify it matches the span's trace ID
	spanTraceID := span.SpanContext().TraceID().String()
	if traceID != spanTraceID {
		t.Errorf("Expected trace ID %s, got %s", spanTraceID, traceID)
	}
}

func TestGetSpanID(t *testing.T) {
	// Setup test tracer
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(tracetest.NewInMemoryExporter()),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())

	// Create a span
	ctx, span := StartSpan(context.Background(), "test.span_id")
	defer span.End()

	// Get span ID
	spanID := GetSpanID(ctx)
	if spanID == "" {
		t.Error("Expected span ID to be non-empty")
	}

	// Verify it matches the span's span ID
	spanSpanID := span.SpanContext().SpanID().String()
	if spanID != spanSpanID {
		t.Errorf("Expected span ID %s, got %s", spanSpanID, spanID)
	}
}

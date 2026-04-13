package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func TraceNotificationSend(ctx context.Context, channel string, recipient string, templateID string, fn func(context.Context) error) error {
	ctx, span := StartSpan(ctx, "notification.send",
		attribute.String("notification.channel", channel),
		attribute.String("notification.recipient", maskSensitiveData(recipient)),
		attribute.String("notification.template_id", templateID),
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

func TraceTemplateProcessing(ctx context.Context, templateID string, variables map[string]string, fn func(context.Context) error) error {
	ctx, span := StartSpan(ctx, "template.process",
		attribute.String("template.id", templateID),
		attribute.Int("template.variable_count", len(variables)),
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

func TraceIdempotencyCheck(ctx context.Context, key string, fn func(context.Context) error) error {
	ctx, span := StartSpan(ctx, "idempotency.check",
		attribute.String("idempotency.key", key),
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

func TraceIdempotencyStore(ctx context.Context, key string, fn func(context.Context) error) error {
	ctx, span := StartSpan(ctx, "idempotency.store",
		attribute.String("idempotency.key", key),
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

package middleware

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	instrumentationName = "github.com/randco/randco-microservices/services/service-notification"
)

type TracingMiddleware struct {
	tracer trace.Tracer
}

func NewTracingMiddleware() *TracingMiddleware {
	return &TracingMiddleware{
		tracer: otel.Tracer(instrumentationName),
	}
}

func (tm *TracingMiddleware) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx, span := tm.tracer.Start(ctx, fmt.Sprintf("grpc.%s", info.FullMethod))
		defer span.End()

		// Add span attributes
		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.service", getServiceName(info.FullMethod)),
			attribute.String("rpc.method", getMethodName(info.FullMethod)),
			attribute.String("rpc.grpc.status_code", "OK"), // Will be updated if error occurs
		)

		// Add metadata attributes if available
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if userAgent := md.Get("user-agent"); len(userAgent) > 0 {
				span.SetAttributes(attribute.String("user_agent", userAgent[0]))
			}
			if correlationID := md.Get("correlation-id"); len(correlationID) > 0 {
				span.SetAttributes(attribute.String("correlation_id", correlationID[0]))
			}
			if requestID := md.Get("request-id"); len(requestID) > 0 {
				span.SetAttributes(attribute.String("request_id", requestID[0]))
			}
		}

		// Call the handler
		resp, err := handler(ctx, req)

		if err != nil {
			// Set error status and attributes
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			// Add gRPC status code
			if st, ok := status.FromError(err); ok {
				span.SetAttributes(
					attribute.String("rpc.grpc.status_code", st.Code().String()),
					attribute.String("error.message", st.Message()),
				)
			}
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return resp, err
	}
}

func (tm *TracingMiddleware) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := stream.Context()
		ctx, span := tm.tracer.Start(ctx, fmt.Sprintf("grpc.%s", info.FullMethod))
		defer span.End()

		// Add span attributes
		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.service", getServiceName(info.FullMethod)),
			attribute.String("rpc.method", getMethodName(info.FullMethod)),
			attribute.Bool("rpc.grpc.streaming", true),
		)

		// Create a wrapped stream with the traced context
		wrappedStream := &tracedServerStream{
			ServerStream: stream,
			ctx:          ctx,
		}

		// Call the handler
		err := handler(srv, wrappedStream)

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			if st, ok := status.FromError(err); ok {
				span.SetAttributes(
					attribute.String("rpc.grpc.status_code", st.Code().String()),
					attribute.String("error.message", st.Message()),
				)
			}
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return err
	}
}

type tracedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *tracedServerStream) Context() context.Context {
	return s.ctx
}

func getServiceName(fullMethod string) string {
	// Extract service name from full method (e.g., "/notification.v1.NotificationService/SendEmail" -> "NotificationService")
	for i := len(fullMethod) - 1; i >= 0; i-- {
		if fullMethod[i] == '.' {
			for j := i - 1; j >= 0; j-- {
				if fullMethod[j] == '.' {
					return fullMethod[j+1 : i]
				}
			}
			// If no second dot found, return from first slash to dot
			for j := 0; j < len(fullMethod); j++ {
				if fullMethod[j] == '/' {
					return fullMethod[j+1 : i]
				}
			}
		}
	}
	return "unknown"
}

func getMethodName(fullMethod string) string {
	// Extract method name from full method (e.g., "/notification.v1.NotificationService/SendEmail" -> "SendEmail")
	for i := len(fullMethod) - 1; i >= 0; i-- {
		if fullMethod[i] == '/' {
			return fullMethod[i+1:]
		}
	}
	return fullMethod
}

func StartSpan(ctx context.Context, operationName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := otel.Tracer(instrumentationName)
	return tracer.Start(ctx, operationName, opts...)
}

func AddSpanAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	span.SetAttributes(attrs...)
}

func RecordError(span trace.Span, err error, description string) {
	span.RecordError(err)
	span.SetStatus(codes.Error, description)
}

func SetSpanStatus(span trace.Span, code codes.Code, description string) {
	span.SetStatus(code, description)
}

package tracing

import (
	"google.golang.org/grpc"
)

// WithServerTrace adds tracing to a gRPC server
// Note: For now, services should use otelgrpc.NewServerHandler() directly
// because the otelgrpc package is more comprehensive
func WithServerTrace() grpc.ServerOption {
	// This is a placeholder - services should use otelgrpc.NewServerHandler() directly
	// e.g., grpc.StatsHandler(otelgrpc.NewServerHandler())
	return grpc.EmptyServerOption{}
}

// WithClientTrace adds tracing to a gRPC client connection
// Note: For now, services should use otelgrpc.NewClientHandler() directly
func WithClientTrace() grpc.DialOption {
	// This is a placeholder - services should use otelgrpc.NewClientHandler() directly
	// e.g., grpc.WithStatsHandler(otelgrpc.NewClientHandler())
	return grpc.EmptyDialOption{}
}
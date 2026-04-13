package clients

import (
	"context"
	"fmt"

	notificationv1 "github.com/randco/randco-microservices/proto/notification/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NotificationClient wraps the gRPC client for the Notification Service
type NotificationClient struct {
	client notificationv1.NotificationServiceClient
	conn   *grpc.ClientConn
}

// NewNotificationClient creates a new notification service gRPC client
func NewNotificationClient(address string) (*NotificationClient, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to notification service at %s: %w", address, err)
	}

	return &NotificationClient{
		client: notificationv1.NewNotificationServiceClient(conn),
		conn:   conn,
	}, nil
}

// NewNotificationServiceClient creates a new notification service gRPC client and returns the interface
func NewNotificationServiceClient(address string) (NotificationServiceClient, error) {
	return NewNotificationClient(address)
}

// SendBulkEmail sends bulk email notifications
func (c *NotificationClient) SendBulkEmail(ctx context.Context, req *notificationv1.SendBulkEmailRequest) (*notificationv1.GenericResponse, error) {
	return c.client.SendBulkEmail(ctx, req)
}

// Close closes the gRPC connection
func (c *NotificationClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

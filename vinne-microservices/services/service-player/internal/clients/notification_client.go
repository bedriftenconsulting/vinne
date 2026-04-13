package clients

import (
	"context"
	"fmt"

	notificationpb "github.com/randco/randco-microservices/proto/notification/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NotificationClient struct {
	conn   *grpc.ClientConn
	client notificationpb.NotificationServiceClient
}

func NewNotificationClient(address string) (*NotificationClient, error) {
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to notification service: %w", err)
	}

	return &NotificationClient{
		conn:   conn,
		client: notificationpb.NewNotificationServiceClient(conn),
	}, nil
}

// SendEmail sends an email notification
func (c *NotificationClient) SendEmail(ctx context.Context, req *notificationpb.SendEmailRequest) (*notificationpb.SendResponse, error) {
	resp, err := c.client.SendEmail(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send email: %w", err)
	}
	return resp, nil
}

// SendSMS sends an SMS notification
func (c *NotificationClient) SendSMS(ctx context.Context, req *notificationpb.SendSMSRequest) (*notificationpb.SendResponse, error) {
	resp, err := c.client.SendSMS(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send SMS: %w", err)
	}
	return resp, nil
}

// SendMobilePushNotification sends a mobile push notification
func (c *NotificationClient) SendMobilePushNotification(ctx context.Context, req *notificationpb.SendPushNotificationRequest) (*notificationpb.SendResponse, error) {
	resp, err := c.client.SendMobilePushNotification(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send mobile push notification: %w", err)
	}
	return resp, nil
}

// SendBulkEmail sends bulk email notifications
func (c *NotificationClient) SendBulkEmail(ctx context.Context, req *notificationpb.SendBulkEmailRequest) (*notificationpb.GenericResponse, error) {
	resp, err := c.client.SendBulkEmail(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send bulk email: %w", err)
	}
	return resp, nil
}

// SendBulkSMS sends bulk SMS notifications
func (c *NotificationClient) SendBulkSMS(ctx context.Context, req *notificationpb.SendBulkSMSRequest) (*notificationpb.GenericResponse, error) {
	resp, err := c.client.SendBulkSMS(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send bulk SMS: %w", err)
	}
	return resp, nil
}

// SendBulkPush sends bulk push notifications
func (c *NotificationClient) SendBulkPush(ctx context.Context, req *notificationpb.SendBulkPushRequest) (*notificationpb.GenericResponse, error) {
	resp, err := c.client.SendBulkPush(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send bulk push: %w", err)
	}
	return resp, nil
}

// GetNotificationStatus gets the status of a notification
func (c *NotificationClient) GetNotificationStatus(ctx context.Context, req *notificationpb.GetStatusRequest) (*notificationpb.NotificationStatus, error) {
	resp, err := c.client.GetNotificationStatus(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification status: %w", err)
	}
	return resp, nil
}

// ListNotifications lists notifications
func (c *NotificationClient) ListNotifications(ctx context.Context, req *notificationpb.ListRequest) (*notificationpb.ListResponse, error) {
	resp, err := c.client.ListNotifications(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list notifications: %w", err)
	}
	return resp, nil
}

// RetryNotification retries a failed notification
func (c *NotificationClient) RetryNotification(ctx context.Context, req *notificationpb.RetryRequest) (*notificationpb.SendResponse, error) {
	resp, err := c.client.RetryNotification(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to retry notification: %w", err)
	}
	return resp, nil
}

// HealthCheck checks the health of the notification service
func (c *NotificationClient) HealthCheck(ctx context.Context) (*notificationpb.HealthCheckResponse, error) {
	req := &notificationpb.EmptyRequest{}
	resp, err := c.client.HealthCheck(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to check notification service health: %w", err)
	}
	return resp, nil
}

// Close closes the gRPC connection
func (c *NotificationClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

package clients

import (
	"context"
	"fmt"

	notificationpb "github.com/randco/randco-microservices/proto/notification/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AgentNotificationClient struct {
	conn   *grpc.ClientConn
	client notificationpb.NotificationServiceClient
}

func NewAgentNotificationClient(address string) (*AgentNotificationClient, error) {
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to connect to notification service: %w", err)
	}

	return &AgentNotificationClient{
		conn:   conn,
		client: notificationpb.NewNotificationServiceClient(conn),
	}, nil

}

// sends an email notification
func (c *AgentNotificationClient) SendEmail(ctx context.Context, req *notificationpb.SendEmailRequest) (*notificationpb.SendResponse, error) {
	resp, err := c.client.SendEmail(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send email: %w", err)

	}
	return resp, nil
}

// sends sms notification
func (c *AgentNotificationClient) SendSMS(ctx context.Context, req *notificationpb.SendSMSRequest) (*notificationpb.SendResponse, error) {
	resp, err := c.client.SendSMS(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send sms: %w", err)
	}
	return resp, nil
}

// checks the health of the notification service
func (c *AgentNotificationClient) HealthCheck(ctx context.Context) (*notificationpb.HealthCheckResponse, error) {
	req := &notificationpb.EmptyRequest{}
	resp, err := c.client.HealthCheck(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to check notification service health: %w", err)
	}
	return resp, nil
}

// Close closes the gRPC connection
func (c *AgentNotificationClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

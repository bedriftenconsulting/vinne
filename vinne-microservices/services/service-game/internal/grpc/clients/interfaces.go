package clients

import (
	"context"

	notificationv1 "github.com/randco/randco-microservices/proto/notification/v1"
)

// NotificationServiceClient defines the interface for notification service client
type NotificationServiceClient interface {
	SendBulkEmail(ctx context.Context, req *notificationv1.SendBulkEmailRequest) (*notificationv1.GenericResponse, error)
	Close() error
}

// AdminServiceClient defines the interface for admin management service client
type AdminServiceClient interface {
	ListActiveAdminEmails(ctx context.Context) ([]string, error)
	Close() error
}

package clients

import (
	"context"
	"fmt"
	"time"

	adminmanagementv1 "github.com/randco/randco-microservices/proto/admin/management/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AdminClient defines the interface for admin management service operations
type AdminClient interface {
	// GetActiveAdminEmails retrieves email addresses of all active admin users
	GetActiveAdminEmails(ctx context.Context) ([]string, error)
	// Close closes the gRPC connection
	Close() error
}

// adminClientImpl implements AdminClient
type adminClientImpl struct {
	conn   *grpc.ClientConn
	client adminmanagementv1.AdminManagementServiceClient
}

// NewAdminClient creates a new admin management client
func NewAdminClient(adminServiceAddr string) (AdminClient, error) {
	conn, err := grpc.NewClient(
		adminServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin management client: %w", err)
	}

	// Test connection with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := adminmanagementv1.NewAdminManagementServiceClient(conn)

	// Try a simple call to verify connectivity (with is_active filter to minimize response)
	isActive := true
	_, err = client.ListAdminUsers(ctx, &adminmanagementv1.ListAdminUsersRequest{
		Page:     1,
		PageSize: 1,
		IsActive: &isActive,
	})
	if err != nil {
		_ = conn.Close() // Ignore close error since connection failed anyway
		return nil, fmt.Errorf("failed to connect to admin management service: %w", err)
	}

	return &adminClientImpl{
		conn:   conn,
		client: client,
	}, nil
}

// GetActiveAdminEmails retrieves email addresses of all active admin users
func (c *adminClientImpl) GetActiveAdminEmails(ctx context.Context) ([]string, error) {
	// Request all active admin users
	// Set page_size to a large number to get all admins in one request
	isActive := true
	resp, err := c.client.ListAdminUsers(ctx, &adminmanagementv1.ListAdminUsersRequest{
		Page:     1,
		PageSize: 1000, // Assume we won't have more than 1000 admin users
		IsActive: &isActive,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list admin users: %w", err)
	}

	// Extract email addresses
	emails := make([]string, 0, len(resp.Users))
	for _, user := range resp.Users {
		if user.Email != "" && user.IsActive {
			emails = append(emails, user.Email)
		}
	}

	return emails, nil
}

// Close closes the gRPC connection
func (c *adminClientImpl) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

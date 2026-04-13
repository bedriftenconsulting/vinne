package clients

import (
	"context"
	"fmt"

	adminv1 "github.com/randco/randco-microservices/proto/admin/management/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AdminClient wraps the gRPC client for the Admin Management Service
type AdminClient struct {
	client adminv1.AdminManagementServiceClient
	conn   *grpc.ClientConn
}

// NewAdminClient creates a new admin management service gRPC client
func NewAdminClient(address string) (*AdminClient, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to admin management service at %s: %w", address, err)
	}

	return &AdminClient{
		client: adminv1.NewAdminManagementServiceClient(conn),
		conn:   conn,
	}, nil
}

// NewAdminServiceClient creates a new admin management service gRPC client and returns the interface
func NewAdminServiceClient(address string) (AdminServiceClient, error) {
	return NewAdminClient(address)
}

// ListActiveAdminEmails fetches all active admin user email addresses
func (c *AdminClient) ListActiveAdminEmails(ctx context.Context) ([]string, error) {
	isActive := true
	req := &adminv1.ListAdminUsersRequest{
		Page:     1,
		PageSize: 1000, // Get all active admins (should be sufficient for most organizations)
		IsActive: &isActive,
	}

	resp, err := c.client.ListAdminUsers(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list admin users: %w", err)
	}

	emails := make([]string, 0, len(resp.Users))
	for _, user := range resp.Users {
		if user.Email != "" {
			emails = append(emails, user.Email)
		}
	}

	// If we have users but no valid emails, return an error
	if len(resp.Users) > 0 && len(emails) == 0 {
		return nil, fmt.Errorf("admin service returned %d users but none have valid email addresses", len(resp.Users))
	}

	return emails, nil
}

// Close closes the gRPC connection
func (c *AdminClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

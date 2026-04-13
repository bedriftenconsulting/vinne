package clients

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	pb "github.com/randco/randco-microservices/proto/agent/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AgentAuthClient wraps the gRPC client for service-agent-auth
type AgentAuthClient struct {
	conn   *grpc.ClientConn
	client pb.AgentAuthServiceClient
}

func NewAgentAuthClient(address string) (*AgentAuthClient, error) {
	if address == "" {
		return nil, fmt.Errorf("agent auth service address is required")
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent auth service: %w", err)
	}
	return &AgentAuthClient{
		conn:   conn,
		client: pb.NewAgentAuthServiceClient(conn),
	}, nil
}

func (c *AgentAuthClient) Close() error {
	return c.conn.Close()
}

// CreateAgentAuth creates login credentials for a newly created agent
func (c *AgentAuthClient) CreateAgentAuth(ctx context.Context, agentID uuid.UUID, agentCode, email, phone, password, createdBy string) error {
	_, err := c.client.CreateAgentAuth(ctx, &pb.CreateAgentAuthRequest{
		AgentId:   agentID.String(),
		AgentCode: agentCode,
		Email:     email,
		Phone:     phone,
		Password:  password,
		CreatedBy: createdBy,
	})
	if err != nil {
		return fmt.Errorf("failed to create agent auth credentials: %w", err)
	}
	return nil
}

// CreateRetailerAuth creates login credentials (PIN) for a newly created retailer
func (c *AgentAuthClient) CreateRetailerAuth(ctx context.Context, retailerID uuid.UUID, retailerCode, email, phone, pin, createdBy string) error {
	_, err := c.client.CreateRetailerAuth(ctx, &pb.CreateRetailerAuthRequest{
		RetailerId:   retailerID.String(),
		RetailerCode: retailerCode,
		Email:        email,
		Phone:        phone,
		Pin:          pin,
		CreatedBy:    createdBy,
	})
	if err != nil {
		return fmt.Errorf("failed to create retailer auth credentials: %w", err)
	}
	return nil
}

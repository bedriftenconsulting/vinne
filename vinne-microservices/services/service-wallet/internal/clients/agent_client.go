package clients

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	agentpb "github.com/randco/randco-microservices/proto/agent/management/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AgentClient provides methods to interact with the agent management service
type AgentClient struct {
	conn   *grpc.ClientConn
	client agentpb.AgentManagementServiceClient
}

// NewAgentClient creates a new agent management service client
func NewAgentClient(address string) (*AgentClient, error) {
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent management service: %w", err)
	}

	return &AgentClient{
		conn:   conn,
		client: agentpb.NewAgentManagementServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection
func (c *AgentClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetAgent retrieves agent details by ID
func (c *AgentClient) GetAgent(ctx context.Context, agentID uuid.UUID) (*agentpb.Agent, error) {
	req := &agentpb.GetAgentRequest{
		Id: agentID.String(),
	}

	resp, err := c.client.GetAgent(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	return resp, nil
}

// GetAgentCommissionPercentage retrieves just the commission percentage for an agent
func (c *AgentClient) GetAgentCommissionPercentage(ctx context.Context, agentID uuid.UUID) (float64, error) {
	agent, err := c.GetAgent(ctx, agentID)
	if err != nil {
		return 0, err
	}

	// Return the commission percentage
	// If it's 0, return default 30%
	if agent.CommissionPercentage == 0 {
		return 30.0, nil
	}

	return agent.CommissionPercentage, nil
}

// GetRetailer retrieves retailer details by ID
func (c *AgentClient) GetRetailer(ctx context.Context, retailerID uuid.UUID) (*agentpb.Retailer, error) {
	req := &agentpb.GetRetailerRequest{
		Id: retailerID.String(),
	}

	resp, err := c.client.GetRetailer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get retailer: %w", err)
	}

	return resp, nil
}

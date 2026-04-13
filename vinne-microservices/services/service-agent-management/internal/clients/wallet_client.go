package clients

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	walletpb "github.com/randco/randco-microservices/proto/wallet/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// WalletClient provides methods to interact with the wallet service
type WalletClient struct {
	conn   *grpc.ClientConn
	client walletpb.WalletServiceClient
}

// NewWalletClient creates a new wallet service client
func NewWalletClient(address string) (*WalletClient, error) {
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to wallet service: %w", err)
	}

	return &WalletClient{
		conn:   conn,
		client: walletpb.NewWalletServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection
func (c *WalletClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CreateAgentWallet creates a new wallet for an agent
func (c *WalletClient) CreateAgentWallet(ctx context.Context, agentID uuid.UUID, agentCode string, commissionRate float64, createdBy string) error {
	req := &walletpb.CreateAgentWalletRequest{
		AgentId:               agentID.String(),
		AgentCode:             agentCode,
		InitialCommissionRate: commissionRate,
		CreatedBy:             createdBy,
	}

	resp, err := c.client.CreateAgentWallet(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create agent wallet: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("wallet creation failed: %s", resp.Message)
	}

	return nil
}

// CreateRetailerWallets creates stake and winning wallets for a retailer
func (c *WalletClient) CreateRetailerWallets(ctx context.Context, retailerID uuid.UUID, retailerCode string, parentAgentID *uuid.UUID, createdBy string) error {
	req := &walletpb.CreateRetailerWalletsRequest{
		RetailerId:   retailerID.String(),
		RetailerCode: retailerCode,
		CreatedBy:    createdBy,
	}

	if parentAgentID != nil {
		req.ParentAgentId = parentAgentID.String()
	}

	resp, err := c.client.CreateRetailerWallets(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create retailer wallets: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("wallet creation failed: %s", resp.Message)
	}

	return nil
}

// SetCommissionRate sets the commission rate for an agent in the wallet service
func (c *WalletClient) SetCommissionRate(ctx context.Context, agentID uuid.UUID, newRate float64, updatedBy string) error {
	req := &walletpb.SetCommissionRateRequest{
		AgentId: agentID.String(),
		Rate:    newRate / 100.0, // Convert percentage to decimal
		SetBy:   updatedBy,
	}

	resp, err := c.client.SetCommissionRate(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to set agent commission rate in wallet service: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("commission rate update failed: %s", resp.Message)
	}

	return nil
}

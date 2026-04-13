package clients

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	walletpb "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/service-player/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WalletClient struct {
	conn   *grpc.ClientConn
	client walletpb.WalletServiceClient
}

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

func (c *WalletClient) GetPlayerWalletBalance(ctx context.Context, playerID uuid.UUID) (*models.WalletBalance, error) {
	req := &walletpb.GetPlayerWalletBalanceRequest{
		PlayerId: playerID.String(),
	}

	resp, err := c.client.GetPlayerWalletBalance(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet balance: %w", err)
	}

	return &models.WalletBalance{
		Balance:          int64(resp.Balance),
		PendingBalance:   int64(resp.PendingBalance),
		AvailableBalance: int64(resp.AvailableBalance),
		LastUpdated:      resp.LastUpdated.AsTime(),
	}, nil
}

// InitiateDeposit initiates a deposit for a player
func (c *WalletClient) InitiateDeposit(ctx context.Context, req models.DepositRequest) (*models.DepositResponse, error) {
	idempotencyKey := generateDepositIdempotencyKey(req.PlayerID, req.MobileMoneyPhone, req.Amount, req.PaymentMethod)

	walletReq := &walletpb.CreditPlayerWalletRequest{
		PlayerId:       req.PlayerID.String(),
		Amount:         float64(req.Amount),
		Reference:      fmt.Sprintf("deposit_%s", uuid.New().String()),
		Notes:          fmt.Sprintf("Mobile money deposit from %s via %s", req.MobileMoneyPhone, req.PaymentMethod),
		IdempotencyKey: idempotencyKey,
	}

	resp, err := c.client.CreditPlayerWallet(ctx, walletReq)
	if err != nil {
		return nil, fmt.Errorf("failed to credit player wallet: %w", err)
	}

	return &models.DepositResponse{
		TransactionID: resp.TransactionId,
		Status:        "completed",
		Message:       resp.Message,
	}, nil
}

// InitiateWithdrawal initiates a withdrawal for a player using two-phase commit
func (c *WalletClient) InitiateWithdrawal(ctx context.Context, req models.WithdrawalRequest) (*models.WithdrawalResponse, error) {
	reference := fmt.Sprintf("withdrawal_%s", uuid.New().String())
	reserveReq := &walletpb.ReservePlayerWalletFundsRequest{
		PlayerId:   req.PlayerID.String(),
		Amount:     float64(req.Amount),
		Reference:  reference,
		TtlSeconds: 600,
		Reason:     fmt.Sprintf("Withdrawal to %s", req.MobileMoneyPhone),
	}

	reserveResp, err := c.client.ReservePlayerWalletFunds(ctx, reserveReq)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve funds: %w", err)
	}

	return &models.WithdrawalResponse{
		TransactionID: reserveResp.ReservationId,
		Status:        "pending",
		Message:       "Withdrawal initiated - funds reserved",
	}, nil
}

func (c *WalletClient) GetTransactionHistory(ctx context.Context, playerID uuid.UUID, filter models.TransactionFilter) ([]*models.Transaction, error) {
	req := &walletpb.GetTransactionHistoryRequest{
		WalletOwnerId: playerID.String(),
		WalletType:    walletpb.WalletType_PLAYER_WALLET,
		Page:          int32(filter.Page),
		PageSize:      int32(filter.PerPage),
	}

	if filter.FromDate != nil {
		req.StartDate = timestamppb.New(*filter.FromDate)
	}
	if filter.ToDate != nil {
		req.EndDate = timestamppb.New(*filter.ToDate)
	}

	resp, err := c.client.GetTransactionHistory(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction history: %w", err)
	}

	transactions := make([]*models.Transaction, len(resp.Transactions))
	for i, tx := range resp.Transactions {
		txUUID, err := uuid.Parse(tx.Id)
		if err != nil {
			return nil, fmt.Errorf("invalid transaction ID: %w", err)
		}

		transactions[i] = &models.Transaction{
			ID:        txUUID,
			PlayerID:  playerID,
			Type:      tx.Type.String(),
			Amount:    int64(tx.Amount),
			Reference: tx.Reference,
			Status:    tx.Status.String(),
			CreatedAt: tx.CreatedAt.AsTime(),
		}
	}

	return transactions, nil
}

func (c *WalletClient) CreatePlayerWallet(ctx context.Context, playerID uuid.UUID, playerCode string) error {
	req := &walletpb.CreatePlayerWalletRequest{
		PlayerId:   playerID.String(),
		PlayerCode: playerCode,
	}

	_, err := c.client.CreatePlayerWallet(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create player wallet: %w", err)
	}

	return nil
}

// Close closes the gRPC connection
func (c *WalletClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// generateDepositIdempotencyKey creates a deterministic idempotency key for deposits
func generateDepositIdempotencyKey(playerID uuid.UUID, mobileMoneyPhone string, amount int64, paymentMethod string) string {
	paymentData := fmt.Sprintf("%s_%s_%d_%s",
		playerID.String(),
		mobileMoneyPhone,
		amount,
		paymentMethod)

	hash := sha256.Sum256([]byte(paymentData))
	return fmt.Sprintf("deposit_%s_%s", playerID.String(), hex.EncodeToString(hash[:8]))
}

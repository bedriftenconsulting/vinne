package server

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	pb "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/service-wallet/internal/models"
	"github.com/randco/service-wallet/internal/repositories"
	"github.com/randco/service-wallet/internal/services"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// WalletServer implements the gRPC WalletService
type WalletServer struct {
	pb.UnimplementedWalletServiceServer
	walletService     services.WalletService
	commissionService services.CommissionService
	agentClient       services.AgentClient
	tracer            trace.Tracer
}

// NewWalletServer creates a new WalletServer instance
func NewWalletServer(
	walletService services.WalletService,
	commissionService services.CommissionService,
	agentClient services.AgentClient,
) *WalletServer {
	return &WalletServer{
		walletService:     walletService,
		commissionService: commissionService,
		agentClient:       agentClient,
		tracer:            otel.Tracer("wallet-grpc-server"),
	}
}

// Helper functions to convert model enums to proto enums

func walletTypeToProto(walletType models.WalletType) pb.WalletType {
	switch walletType {
	case models.WalletTypeAgentStake:
		return pb.WalletType_AGENT_STAKE
	case models.WalletTypeRetailerStake:
		return pb.WalletType_RETAILER_STAKE
	case models.WalletTypeRetailerWinning:
		return pb.WalletType_RETAILER_WINNING
	default:
		return pb.WalletType_WALLET_TYPE_UNSPECIFIED
	}
}

func transactionTypeToProto(txType models.TransactionType) pb.TransactionType {
	switch txType {
	case models.TransactionTypeCredit:
		return pb.TransactionType_CREDIT
	case models.TransactionTypeDebit:
		return pb.TransactionType_DEBIT
	case models.TransactionTypeTransfer:
		return pb.TransactionType_TRANSFER
	case models.TransactionTypeCommission:
		return pb.TransactionType_COMMISSION
	case models.TransactionTypePayout:
		return pb.TransactionType_PAYOUT
	default:
		return pb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}

func transactionStatusToProto(status models.TransactionStatus) pb.TransactionStatus {
	switch status {
	case models.TransactionStatusPending:
		return pb.TransactionStatus_PENDING
	case models.TransactionStatusCompleted:
		return pb.TransactionStatus_COMPLETED
	case models.TransactionStatusFailed:
		return pb.TransactionStatus_FAILED
	case models.TransactionStatusReversed:
		return pb.TransactionStatus_REVERSED
	default:
		return pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
}

// Helper function to convert proto CreditSource enum to model CreditSource
func creditSourceFromProto(cs pb.CreditSource) models.CreditSource {
	switch cs {
	case pb.CreditSource_CREDIT_SOURCE_ADMIN_DIRECT:
		return models.CreditSourceAdminDirect
	case pb.CreditSource_CREDIT_SOURCE_MOBILE_MONEY:
		return models.CreditSourceMobileMoney
	case pb.CreditSource_CREDIT_SOURCE_BANK_TRANSFER:
		return models.CreditSourceBankTransfer
	case pb.CreditSource_CREDIT_SOURCE_SYSTEM_ADJUSTMENT:
		return models.CreditSourceSystemAdjustment
	case pb.CreditSource_CREDIT_SOURCE_REVERSAL:
		return models.CreditSourceReversal
	default:
		return models.CreditSourceAdminDirect // Default to admin_direct for unspecified
	}
}

// CreateAgentWallet creates a new agent wallet
func (s *WalletServer) CreateAgentWallet(ctx context.Context, req *pb.CreateAgentWalletRequest) (*pb.CreateAgentWalletResponse, error) {
	ctx, span := s.tracer.Start(ctx, "CreateAgentWallet")
	defer span.End()

	// Validate request
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.AgentCode == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_code is required")
	}
	if req.CreatedBy == "" {
		return nil, status.Error(codes.InvalidArgument, "created_by is required")
	}

	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid agent_id format")
	}

	span.SetAttributes(
		attribute.String("agent.id", req.AgentId),
		attribute.String("agent.code", req.AgentCode),
		attribute.Float64("commission.rate", req.InitialCommissionRate),
	)

	// Call service to create agent wallet
	err = s.walletService.CreateAgentWallet(ctx, agentID, req.AgentCode, req.InitialCommissionRate, req.CreatedBy)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to create agent wallet")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create agent wallet: %v", err))
	}

	// Get the created wallet to return details
	wallet, err := s.walletService.GetAgentBalance(ctx, agentID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get created wallet")
		return nil, status.Error(codes.Internal, "failed to get created wallet details")
	}

	return &pb.CreateAgentWalletResponse{
		Success:   true,
		WalletId:  wallet.ID.String(),
		AgentId:   req.AgentId,
		Balance:   float64(wallet.Balance) / 100, // Convert from pesewas to cedis
		Message:   "Agent wallet created successfully",
		CreatedAt: timestamppb.New(wallet.CreatedAt),
	}, nil
}

// CreateRetailerWallets creates both stake and winning wallets for a retailer
func (s *WalletServer) CreateRetailerWallets(ctx context.Context, req *pb.CreateRetailerWalletsRequest) (*pb.CreateRetailerWalletsResponse, error) {
	ctx, span := s.tracer.Start(ctx, "CreateRetailerWallets")
	defer span.End()

	// Validate request
	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}
	if req.RetailerCode == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_code is required")
	}
	if req.CreatedBy == "" {
		return nil, status.Error(codes.InvalidArgument, "created_by is required")
	}

	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid retailer_id format")
	}

	var parentAgentID *uuid.UUID
	if req.ParentAgentId != "" {
		agentID, err := uuid.Parse(req.ParentAgentId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid parent_agent_id format")
		}
		parentAgentID = &agentID
	}

	span.SetAttributes(
		attribute.String("retailer.id", req.RetailerId),
		attribute.String("retailer.code", req.RetailerCode),
	)

	if parentAgentID != nil {
		span.SetAttributes(attribute.String("parent_agent.id", parentAgentID.String()))
	}

	// Call service to create retailer wallets
	err = s.walletService.CreateRetailerWallets(ctx, retailerID, req.RetailerCode, parentAgentID, req.CreatedBy)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to create retailer wallets")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create retailer wallets: %v", err))
	}

	// For now, return success with placeholder wallet IDs
	// In a full implementation, we'd get the actual wallet IDs from the service
	return &pb.CreateRetailerWalletsResponse{
		Success:         true,
		StakeWalletId:   fmt.Sprintf("stake_%s", retailerID.String()),
		WinningWalletId: fmt.Sprintf("winning_%s", retailerID.String()),
		RetailerId:      req.RetailerId,
		Message:         "Retailer wallets created successfully",
		CreatedAt:       timestamppb.Now(),
	}, nil
}

// CreditAgentWallet handles agent wallet credit requests with fixed 30% commission gross-up
func (s *WalletServer) CreditAgentWallet(ctx context.Context, req *pb.CreditAgentWalletRequest) (*pb.CreditAgentWalletResponse, error) {
	ctx, span := s.tracer.Start(ctx, "CreditAgentWallet")
	defer span.End()

	// Validate request
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}

	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid agent_id format")
	}

	// Convert proto CreditSource to model CreditSource
	creditSource := creditSourceFromProto(req.CreditSource)

	span.SetAttributes(
		attribute.String("agent.id", req.AgentId),
		attribute.Float64("base_amount", req.Amount),
		attribute.String("reference", req.Reference),
		attribute.String("credit_source", string(creditSource)),
	)

	// Amount is already in pesewas from API Gateway
	baseAmountPesewas := int64(req.Amount)

	// Apply fixed 30% gross-up where base is 70% of final
	// Formula: gross = base / 0.7, commission = gross - base
	grossAmountPesewas := int64(float64(baseAmountPesewas) / 0.7)
	commissionAmountPesewas := grossAmountPesewas - baseAmountPesewas

	span.SetAttributes(
		attribute.Int64("base_amount.pesewas", baseAmountPesewas),
		attribute.Int64("commission_amount.pesewas", commissionAmountPesewas),
		attribute.Int64("gross_amount.pesewas", grossAmountPesewas),
	)

	// Build descriptions for separate transactions
	baseDescription := fmt.Sprintf("Credit via %s - %.2f GHS", req.PaymentMethod, float64(baseAmountPesewas)/100)
	if req.Notes != "" {
		baseDescription += " - " + req.Notes
	}

	commissionDescription := fmt.Sprintf("Commission (30%%) on %.2f GHS credit - %.2f GHS",
		float64(baseAmountPesewas)/100,
		float64(commissionAmountPesewas)/100,
	)

	// Transaction 1: Credit base amount
	baseTransaction, err := s.walletService.CreditAgentWallet(ctx, agentID, baseAmountPesewas, baseDescription, req.IdempotencyKey, creditSource)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to credit base amount")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to credit base amount: %v", err))
	}

	// Transaction 2: Credit commission amount
	commissionIdempotencyKey := req.IdempotencyKey + "-commission"
	commissionTransaction, err := s.walletService.CreditAgentWallet(ctx, agentID, commissionAmountPesewas, commissionDescription, commissionIdempotencyKey, creditSource)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to credit commission")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to credit commission: %v", err))
	}

	// Record commission transaction for audit and metrics
	if err := s.commissionService.RecordCommissionTransaction(ctx, agentID, baseAmountPesewas, grossAmountPesewas, commissionAmountPesewas, 3000, models.CommissionTypeDeposit, commissionTransaction.ID); err != nil {
		// Log the error but don't fail the operation since wallet credit already succeeded
		log.Printf("ERROR: Failed to record commission transaction: %v", err)
		span.RecordError(fmt.Errorf("failed to record commission transaction: %w", err))
	}

	// Get updated balance
	wallet, err := s.walletService.GetAgentBalance(ctx, agentID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get balance")
		return nil, status.Error(codes.Internal, "failed to get updated balance")
	}

	return &pb.CreditAgentWalletResponse{
		Success:          true,
		TransactionId:    baseTransaction.ID.String(),
		BaseAmount:       float64(baseAmountPesewas),       // Return pesewas
		CommissionAmount: float64(commissionAmountPesewas), // Return pesewas
		GrossAmount:      float64(grossAmountPesewas),      // Return pesewas
		NewBalance:       float64(wallet.Balance),          // Return pesewas
		Message: fmt.Sprintf("Wallet credited successfully. Base: %.2f GHS + Commission (30%%): %.2f GHS = Total: %.2f GHS",
			float64(baseAmountPesewas)/100,
			float64(commissionAmountPesewas)/100,
			float64(grossAmountPesewas)/100,
		),
		Timestamp: timestamppb.New(baseTransaction.CreatedAt),
	}, nil
}

// GetAgentWalletBalance retrieves the current balance of an agent's wallet
func (s *WalletServer) GetAgentWalletBalance(ctx context.Context, req *pb.GetAgentWalletBalanceRequest) (*pb.GetAgentWalletBalanceResponse, error) {
	ctx, span := s.tracer.Start(ctx, "GetAgentWalletBalance")
	defer span.End()

	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid agent_id format")
	}

	span.SetAttributes(attribute.String("agent.id", req.AgentId))

	wallet, err := s.walletService.GetAgentBalance(ctx, agentID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get balance")
		return nil, status.Error(codes.Internal, "failed to get wallet balance")
	}

	return &pb.GetAgentWalletBalanceResponse{
		AgentId:          req.AgentId,
		Balance:          float64(wallet.Balance),                         // Return pesewas
		PendingBalance:   float64(wallet.PendingBalance),                  // Return pesewas
		AvailableBalance: float64(wallet.Balance - wallet.PendingBalance), // Return pesewas
		LastUpdated:      timestamppb.New(wallet.UpdatedAt),
	}, nil
}

// CreditRetailerWallet handles retailer wallet credit requests
// - STAKE wallet: Applies 30% commission gross-up (base is 70% of final)
// - WINNING wallet: Credits exact amount without commission
func (s *WalletServer) CreditRetailerWallet(ctx context.Context, req *pb.CreditRetailerWalletRequest) (*pb.CreditRetailerWalletResponse, error) {
	ctx, span := s.tracer.Start(ctx, "CreditRetailerWallet")
	defer span.End()

	// Validate request
	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}
	if req.WalletType == pb.WalletType_WALLET_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "wallet_type is required")
	}

	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid retailer_id format")
	}

	// Convert proto CreditSource to model CreditSource
	creditSource := creditSourceFromProto(req.CreditSource)

	span.SetAttributes(
		attribute.String("retailer.id", req.RetailerId),
		attribute.Float64("base_amount", req.Amount),
		attribute.String("wallet_type", req.WalletType.String()),
		attribute.String("credit_source", string(creditSource)),
	)

	// Amount is already in pesewas from the API Gateway
	baseAmountPesewas := int64(req.Amount)

	// Map proto wallet type to model
	var walletType models.WalletType
	switch req.WalletType {
	case pb.WalletType_RETAILER_STAKE:
		walletType = models.WalletTypeRetailerStake
	case pb.WalletType_RETAILER_WINNING:
		walletType = models.WalletTypeRetailerWinning
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid wallet type for retailer")
	}

	// Build description
	var description string
	if walletType == models.WalletTypeRetailerStake {
		description = fmt.Sprintf("Top-Up - Ref: %s", req.Reference)
	} else {
		description = fmt.Sprintf("Winning Payout - Ref: %s", req.Reference)
	}

	if req.Notes != "" {
		description += " - " + req.Notes
	}

	span.SetAttributes(
		attribute.Int64("base_amount.pesewas", baseAmountPesewas),
		attribute.String("wallet_type", string(walletType)),
	)

	// Credit the wallet with base amount - service will handle commission splitting for STAKE wallet
	transaction, err := s.walletService.CreditRetailerWallet(ctx, retailerID, baseAmountPesewas, walletType, description, req.IdempotencyKey, creditSource)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to credit wallet")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to credit wallet: %v", err))
	}

	// Get updated balance
	newBalance, err := s.walletService.GetRetailerBalance(ctx, retailerID, walletType)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get balance")
		return nil, status.Error(codes.Internal, "failed to get updated balance")
	}

	// Calculate commission for response message (service creates two transactions internally)
	var grossAmountPesewas, commissionAmountPesewas int64
	var message string
	if walletType == models.WalletTypeRetailerStake {
		grossAmountPesewas = int64(float64(baseAmountPesewas) / 0.7)
		commissionAmountPesewas = grossAmountPesewas - baseAmountPesewas
		message = fmt.Sprintf("Stake wallet credited. Base: %.2f GHS + Commission (30%%): %.2f GHS = Total: %.2f GHS",
			float64(baseAmountPesewas)/100,
			float64(commissionAmountPesewas)/100,
			float64(grossAmountPesewas)/100,
		)
	} else {
		grossAmountPesewas = baseAmountPesewas
		commissionAmountPesewas = 0
		message = fmt.Sprintf("Winning wallet credited. Amount: %.2f GHS",
			float64(baseAmountPesewas)/100,
		)
	}

	return &pb.CreditRetailerWalletResponse{
		Success:          true,
		TransactionId:    transaction.ID.String(),
		BaseAmount:       float64(baseAmountPesewas),       // Return pesewas
		CommissionAmount: float64(commissionAmountPesewas), // Return pesewas (0 for winning wallet)
		GrossAmount:      float64(grossAmountPesewas),      // Return pesewas
		NewBalance:       float64(newBalance),              // Return pesewas
		Message:          message,
		Timestamp:        timestamppb.New(transaction.CreatedAt),
	}, nil
}

// DebitRetailerWallet handles retailer wallet debit requests with balance validation
func (s *WalletServer) DebitRetailerWallet(ctx context.Context, req *pb.DebitRetailerWalletRequest) (*pb.DebitRetailerWalletResponse, error) {
	ctx, span := s.tracer.Start(ctx, "DebitRetailerWallet")
	defer span.End()

	// Validate request
	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}
	if req.WalletType == pb.WalletType_WALLET_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "wallet_type is required")
	}

	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid retailer_id format")
	}

	span.SetAttributes(
		attribute.String("retailer.id", req.RetailerId),
		attribute.Float64("amount", req.Amount),
		attribute.String("wallet_type", req.WalletType.String()),
	)

	// Amount is already in pesewas from the API Gateway
	amountPesewas := int64(req.Amount)

	// Map proto wallet type to model
	var walletType models.WalletType
	switch req.WalletType {
	case pb.WalletType_RETAILER_STAKE:
		walletType = models.WalletTypeRetailerStake
	case pb.WalletType_RETAILER_WINNING:
		walletType = models.WalletTypeRetailerWinning
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid wallet type for retailer")
	}

	// Build description
	description := fmt.Sprintf("Debit - Ref: %s", req.Reference)
	if req.Reason != "" {
		description += " - " + req.Reason
	}

	// Debit the wallet (includes balance validation)
	transaction, err := s.walletService.DebitRetailerWallet(ctx, retailerID, amountPesewas, walletType, description)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to debit wallet")

		// Return appropriate gRPC error codes based on error type
		errMsg := err.Error()
		if strings.Contains(errMsg, "insufficient funds") {
			return nil, status.Error(codes.FailedPrecondition, errMsg)
		}
		if strings.Contains(errMsg, "wallet not found") || strings.Contains(errMsg, "not found") {
			return nil, status.Error(codes.NotFound, errMsg)
		}
		if strings.Contains(errMsg, "invalid") {
			return nil, status.Error(codes.InvalidArgument, errMsg)
		}
		// Default to Internal for unexpected errors
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to debit wallet: %v", err))
	}

	// Get updated balance
	newBalance, err := s.walletService.GetRetailerBalance(ctx, retailerID, walletType)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get balance")
		return nil, status.Error(codes.Internal, "failed to get updated balance")
	}

	return &pb.DebitRetailerWalletResponse{
		Success:       true,
		TransactionId: transaction.ID.String(),
		DebitedAmount: float64(transaction.Amount), // Return pesewas
		NewBalance:    float64(newBalance),         // Return pesewas
		Message:       "Wallet debited successfully",
		Timestamp:     timestamppb.New(transaction.CreatedAt),
	}, nil
}

// GetRetailerWalletBalance retrieves the balance of a retailer's wallet
func (s *WalletServer) GetRetailerWalletBalance(ctx context.Context, req *pb.GetRetailerWalletBalanceRequest) (*pb.GetRetailerWalletBalanceResponse, error) {
	ctx, span := s.tracer.Start(ctx, "GetRetailerWalletBalance")
	defer span.End()

	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}
	if req.WalletType == pb.WalletType_WALLET_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "wallet_type is required")
	}

	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid retailer_id format")
	}

	// Map proto wallet type to model
	var walletType models.WalletType
	switch req.WalletType {
	case pb.WalletType_RETAILER_STAKE:
		walletType = models.WalletTypeRetailerStake
	case pb.WalletType_RETAILER_WINNING:
		walletType = models.WalletTypeRetailerWinning
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid wallet type for retailer")
	}

	balance, err := s.walletService.GetRetailerBalance(ctx, retailerID, walletType)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get balance")
		return nil, status.Error(codes.Internal, "failed to get wallet balance")
	}

	return &pb.GetRetailerWalletBalanceResponse{
		RetailerId:       req.RetailerId,
		WalletType:       req.WalletType,
		Balance:          float64(balance), // Return pesewas
		PendingBalance:   0,                // TODO: Implement pending balance
		AvailableBalance: float64(balance), // Return pesewas
		LastUpdated:      timestamppb.New(time.Now()),
	}, nil
}

// GetTransactionHistory retrieves transaction history for a wallet
func (s *WalletServer) GetTransactionHistory(ctx context.Context, req *pb.GetTransactionHistoryRequest) (*pb.GetTransactionHistoryResponse, error) {
	ctx, span := s.tracer.Start(ctx, "GetTransactionHistory")
	defer span.End()

	if req.WalletOwnerId == "" {
		return nil, status.Error(codes.InvalidArgument, "wallet_owner_id is required")
	}

	ownerID, err := uuid.Parse(req.WalletOwnerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid wallet_owner_id format")
	}

	// Map proto wallet type to model
	var walletType models.WalletType
	switch req.WalletType {
	case pb.WalletType_AGENT_STAKE:
		walletType = models.WalletTypeAgentStake
	case pb.WalletType_RETAILER_STAKE:
		walletType = models.WalletTypeRetailerStake
	case pb.WalletType_RETAILER_WINNING:
		walletType = models.WalletTypeRetailerWinning
	case pb.WalletType_PLAYER_WALLET:
		walletType = models.WalletTypePlayerWallet
	default:
		walletType = models.WalletTypeAgentStake // Default to agent
	}

	// Set default pagination
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	// Get transactions
	transactions, totalCount, err := s.walletService.GetTransactionHistory(ctx, ownerID, walletType, int(pageSize), int(offset))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get transaction history")
		return nil, status.Error(codes.Internal, "failed to get transaction history")
	}

	// Convert to proto format
	pbTransactions := make([]*pb.Transaction, len(transactions))
	for i, tx := range transactions {
		// Map transaction type from the stored transaction type field
		var txType pb.TransactionType
		switch tx.TransactionType {
		case models.TransactionTypeCredit:
			txType = pb.TransactionType_CREDIT
		case models.TransactionTypeDebit:
			txType = pb.TransactionType_DEBIT
		case models.TransactionTypeTransfer:
			txType = pb.TransactionType_TRANSFER
		case models.TransactionTypeCommission:
			txType = pb.TransactionType_COMMISSION
		case models.TransactionTypePayout:
			txType = pb.TransactionType_PAYOUT
		default:
			txType = pb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
		}

		// Map transaction status
		var txStatus pb.TransactionStatus
		switch tx.Status {
		case models.TransactionStatusPending:
			txStatus = pb.TransactionStatus_PENDING
		case models.TransactionStatusCompleted:
			txStatus = pb.TransactionStatus_COMPLETED
		case models.TransactionStatusFailed:
			txStatus = pb.TransactionStatus_FAILED
		case models.TransactionStatusReversed:
			txStatus = pb.TransactionStatus_REVERSED
		default:
			txStatus = pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
		}

		// Handle nullable reference
		reference := ""
		if tx.Reference != nil {
			reference = *tx.Reference
		}

		// Handle nullable description
		description := ""
		if tx.Description != nil {
			description = *tx.Description
		}

		// Map wallet type from transaction
		var pbWalletType pb.WalletType
		switch tx.WalletType {
		case models.WalletTypeAgentStake:
			pbWalletType = pb.WalletType_AGENT_STAKE
		case models.WalletTypeRetailerStake:
			pbWalletType = pb.WalletType_RETAILER_STAKE
		case models.WalletTypeRetailerWinning:
			pbWalletType = pb.WalletType_RETAILER_WINNING
		case models.WalletTypePlayerWallet:
			pbWalletType = pb.WalletType_PLAYER_WALLET
		default:
			pbWalletType = pb.WalletType_WALLET_TYPE_UNSPECIFIED
		}

		pbTransactions[i] = &pb.Transaction{
			Id:            tx.ID.String(),
			WalletOwnerId: req.WalletOwnerId,
			WalletType:    pbWalletType,
			Type:          txType,
			Amount:        float64(tx.Amount),        // Return pesewas
			BalanceBefore: float64(tx.BalanceBefore), // Return pesewas
			BalanceAfter:  float64(tx.BalanceAfter),  // Return pesewas
			Reference:     reference,
			Description:   description,
			Status:        txStatus,
			CreatedAt:     timestamppb.New(tx.CreatedAt),
		}
	}

	hasMore := totalCount > int(offset+int32(len(transactions)))

	return &pb.GetTransactionHistoryResponse{
		Transactions: pbTransactions,
		TotalCount:   int32(totalCount),
		Page:         page,
		PageSize:     pageSize,
		HasMore:      hasMore,
	}, nil
}

// SetCommissionRate sets the commission rate for an agent
func (s *WalletServer) SetCommissionRate(ctx context.Context, req *pb.SetCommissionRateRequest) (*pb.SetCommissionRateResponse, error) {
	ctx, span := s.tracer.Start(ctx, "SetCommissionRate")
	defer span.End()

	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.Rate < 0 || req.Rate > 1 {
		return nil, status.Error(codes.InvalidArgument, "rate must be between 0 and 1")
	}

	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid agent_id format")
	}

	// Convert rate to basis points (multiply by 10000)
	rateBasisPoints := int32(req.Rate * 10000)

	err = s.commissionService.SetAgentCommissionRate(ctx, agentID, rateBasisPoints, req.Notes)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to set commission rate")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to set commission rate: %v", err))
	}

	// Parse effective from date
	effectiveFrom := time.Now()
	if req.EffectiveFrom != "" {
		parsedTime, err := time.Parse(time.RFC3339, req.EffectiveFrom)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, "invalid effective date format")
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid effective date format: %v", err))
		}
		effectiveFrom = parsedTime
	}

	return &pb.SetCommissionRateResponse{
		Success:       true,
		Message:       "Commission rate updated successfully",
		NewRate:       req.Rate,
		EffectiveFrom: timestamppb.New(effectiveFrom),
	}, nil
}

// GetCommissionRate retrieves the commission rate for an agent
func (s *WalletServer) GetCommissionRate(ctx context.Context, req *pb.GetCommissionRateRequest) (*pb.GetCommissionRateResponse, error) {
	ctx, span := s.tracer.Start(ctx, "GetCommissionRate")
	defer span.End()

	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid agent_id format")
	}

	rate, err := s.commissionService.GetAgentCommissionRate(ctx, agentID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get commission rate")
		return nil, status.Error(codes.Internal, "failed to get commission rate")
	}

	return &pb.GetCommissionRateResponse{
		AgentId:       req.AgentId,
		Rate:          float64(rate.Rate) / 10000, // Convert from basis points
		EffectiveFrom: timestamppb.New(rate.EffectiveFrom),
		CreatedAt:     timestamppb.New(rate.CreatedAt),
		CreatedBy:     rate.CreatedBy.String(),
	}, nil
}

// GetCommissionReport generates a commission report for an agent
func (s *WalletServer) GetCommissionReport(ctx context.Context, req *pb.GetCommissionReportRequest) (*pb.GetCommissionReportResponse, error) {
	ctx, span := s.tracer.Start(ctx, "GetCommissionReport")
	defer span.End()

	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid agent_id format")
	}

	// Set default pagination
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	// Get commission entries
	entries, totalCommission, totalCount, err := s.commissionService.GetCommissionReport(
		ctx, agentID, req.StartDate.AsTime(), req.EndDate.AsTime(), int(pageSize), int(offset),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get commission report")
		return nil, status.Error(codes.Internal, "failed to generate commission report")
	}

	// Convert to proto format
	pbEntries := make([]*pb.CommissionEntry, len(entries))
	for i, entry := range entries {
		pbEntries[i] = &pb.CommissionEntry{
			Id:               entry.ID.String(),
			TransactionId:    entry.TransactionID.String(),
			AgentId:          req.AgentId,
			OriginalAmount:   float64(entry.InputAmount),      // Return pesewas
			GrossAmount:      float64(entry.GrossAmount),      // Return pesewas
			CommissionAmount: float64(entry.CommissionAmount), // Return pesewas
			CommissionRate:   float64(entry.CommissionRate) / 10000,
			Type:             string(entry.TransactionType),
			CreatedAt:        timestamppb.New(entry.CreatedAt),
			Reference:        "", // TODO: Add reference field
		}
	}

	hasMore := totalCount > int(offset+int32(len(entries)))

	return &pb.GetCommissionReportResponse{
		Entries:         pbEntries,
		TotalCommission: float64(totalCommission), // Return pesewas
		TotalCount:      int32(totalCount),
		Page:            page,
		PageSize:        pageSize,
		HasMore:         hasMore,
	}, nil
}

// GetDailyCommissions retrieves daily commission metrics for dashboard
func (s *WalletServer) GetDailyCommissions(ctx context.Context, req *pb.GetDailyCommissionsRequest) (*pb.GetDailyCommissionsResponse, error) {
	ctx, span := s.tracer.Start(ctx, "GetDailyCommissions")
	defer span.End()

	span.SetAttributes(
		attribute.String("date", req.Date),
		attribute.Bool("include_comparison", req.IncludeComparison),
	)

	// Call service to get daily commission metrics
	result, err := s.commissionService.GetDailyCommissions(ctx, req.Date, req.IncludeComparison)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get daily commissions")
		return nil, status.Error(codes.Internal, "failed to get daily commissions")
	}

	// Build response
	response := &pb.GetDailyCommissionsResponse{
		Date: result.Date,
		Commissions: &pb.CommissionMetric{
			Amount:           result.Amount,
			ChangePercentage: result.ChangePercentage,
			PreviousAmount:   result.PreviousAmount,
		},
	}

	span.SetAttributes(
		attribute.Int64("commission.amount", result.Amount),
		attribute.Float64("commission.change", result.ChangePercentage),
	)

	span.SetStatus(otelcodes.Ok, "daily commissions retrieved successfully")
	return response, nil
}

// TransferAgentToRetailer handles transfers from agent to retailer with commission
func (s *WalletServer) TransferAgentToRetailer(ctx context.Context, req *pb.TransferAgentToRetailerRequest) (*pb.TransferAgentToRetailerResponse, error) {
	ctx, span := s.tracer.Start(ctx, "TransferAgentToRetailer")
	defer span.End()

	// Validate request
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}

	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid agent_id format")
	}

	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid retailer_id format")
	}

	span.SetAttributes(
		attribute.String("agent.id", req.AgentId),
		attribute.String("retailer.id", req.RetailerId),
		attribute.Float64("amount", req.Amount),
	)

	// Amount is already in pesewas from the API Gateway
	amountPesewas := int64(req.Amount)

	// Build description
	description := fmt.Sprintf("Transfer to retailer - Ref: %s", req.Reference)
	if req.Notes != "" {
		description += " - " + req.Notes
	}

	// Perform transfer (commission is handled internally)
	transaction, err := s.walletService.TransferAgentToRetailer(ctx, agentID, retailerID, amountPesewas, description)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to transfer")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to transfer: %v", err))
	}

	// Get commission details
	commissionAmount, grossAmount, err := s.commissionService.GetTransactionCommission(ctx, transaction.ID)
	if err != nil {
		// Log but don't fail
		span.RecordError(err)
		commissionAmount = 0
		grossAmount = amountPesewas
	}

	// Get updated balances
	agentWallet, err := s.walletService.GetAgentBalance(ctx, agentID)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, "failed to get agent balance")
	}

	retailerBalance, err := s.walletService.GetRetailerBalance(ctx, retailerID, models.WalletTypeRetailerStake)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, "failed to get retailer balance")
	}

	return &pb.TransferAgentToRetailerResponse{
		Success:            true,
		TransactionId:      transaction.ID.String(),
		TransferredAmount:  float64(amountPesewas),       // Return pesewas
		CommissionCharged:  float64(commissionAmount),    // Return pesewas
		TotalDeducted:      float64(grossAmount),         // Return pesewas
		AgentNewBalance:    float64(agentWallet.Balance), // Return pesewas
		RetailerNewBalance: float64(retailerBalance),     // Return pesewas
		Message:            "Transfer completed successfully",
		Timestamp:          timestamppb.New(transaction.CreatedAt),
	}, nil
}

// UpdateAgentCommission updates the commission rate for an agent (called by agent management service)
func (s *WalletServer) UpdateAgentCommission(ctx context.Context, req *pb.UpdateAgentCommissionRequest) (*pb.UpdateAgentCommissionResponse, error) {
	ctx, span := s.tracer.Start(ctx, "UpdateAgentCommission")
	defer span.End()

	// Validate request
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	if req.NewRate < 0 || req.NewRate > 1 {
		return nil, status.Error(codes.InvalidArgument, "rate must be between 0 and 1")
	}

	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid agent_id format")
	}

	// Update the commission using wallet service
	previousRate, err := s.walletService.UpdateAgentCommission(ctx, agentID, req.NewRate, req.UpdatedBy, req.Reason)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to update commission")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to update commission: %v", err))
	}

	return &pb.UpdateAgentCommissionResponse{
		Success:      true,
		Message:      fmt.Sprintf("Commission rate updated from %.2f%% to %.2f%%", previousRate*100, req.NewRate*100),
		PreviousRate: previousRate,
		NewRate:      req.NewRate,
		UpdatedAt:    timestamppb.Now(),
	}, nil
}

// ReserveRetailerWalletFunds reserves funds in a retailer wallet (Phase 1 of two-phase commit)
func (s *WalletServer) ReserveRetailerWalletFunds(ctx context.Context, req *pb.ReserveRetailerWalletFundsRequest) (*pb.ReserveRetailerWalletFundsResponse, error) {
	ctx, span := s.tracer.Start(ctx, "ReserveRetailerWalletFunds")
	defer span.End()

	// Validate request
	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}
	if req.WalletType == pb.WalletType_WALLET_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "wallet_type is required")
	}
	if req.Reference == "" {
		return nil, status.Error(codes.InvalidArgument, "reference is required")
	}

	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid retailer_id format")
	}

	span.SetAttributes(
		attribute.String("retailer.id", req.RetailerId),
		attribute.Float64("amount", req.Amount),
		attribute.String("wallet_type", req.WalletType.String()),
		attribute.String("reference", req.Reference),
		attribute.Int64("ttl_seconds", int64(req.TtlSeconds)),
	)

	// Map proto wallet type to model
	var walletType models.WalletType
	switch req.WalletType {
	case pb.WalletType_RETAILER_STAKE:
		walletType = models.WalletTypeRetailerStake
	case pb.WalletType_RETAILER_WINNING:
		walletType = models.WalletTypeRetailerWinning
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid wallet type for retailer")
	}

	// Amount is already in pesewas
	amountPesewas := int64(req.Amount)

	// Use reference as idempotency key if not provided
	idempotencyKey := req.Reference

	// Call service to reserve funds
	reservationID, err := s.walletService.ReserveRetailerWalletFunds(
		ctx,
		retailerID,
		walletType,
		amountPesewas,
		req.Reference,
		req.TtlSeconds,
		req.Reason,
		idempotencyKey,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to reserve funds")

		// Return appropriate error codes
		errMsg := err.Error()
		if strings.Contains(errMsg, "insufficient funds") {
			return nil, status.Error(codes.FailedPrecondition, errMsg)
		}
		if strings.Contains(errMsg, "not found") {
			return nil, status.Error(codes.NotFound, errMsg)
		}
		if strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "duplicate") {
			return nil, status.Error(codes.AlreadyExists, errMsg)
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to reserve funds: %v", err))
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(time.Duration(req.TtlSeconds) * time.Second)

	// Get updated balance
	balance, err := s.walletService.GetRetailerBalance(ctx, retailerID, walletType)
	if err != nil {
		span.RecordError(err)
		// Don't fail the reservation, just return zero balance
		balance = 0
	}

	span.SetAttributes(
		attribute.String("reservation_id", reservationID),
		attribute.Int64("available_balance", balance),
	)

	return &pb.ReserveRetailerWalletFundsResponse{
		Success:          true,
		ReservationId:    reservationID,
		ReservedAmount:   req.Amount,
		AvailableBalance: float64(balance),
		Message:          fmt.Sprintf("Funds reserved successfully. Reservation ID: %s", reservationID),
		ReservedAt:       timestamppb.Now(),
		ExpiresAt:        timestamppb.New(expiresAt),
	}, nil
}

// CommitReservedDebit commits a reserved fund debit (Phase 2 of two-phase commit)
func (s *WalletServer) CommitReservedDebit(ctx context.Context, req *pb.CommitReservedDebitRequest) (*pb.CommitReservedDebitResponse, error) {
	ctx, span := s.tracer.Start(ctx, "CommitReservedDebit")
	defer span.End()

	// Validate request
	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}
	if req.ReservationId == "" {
		return nil, status.Error(codes.InvalidArgument, "reservation_id is required")
	}

	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid retailer_id format")
	}

	span.SetAttributes(
		attribute.String("retailer.id", req.RetailerId),
		attribute.String("reservation_id", req.ReservationId),
		attribute.String("reference", req.Reference),
	)

	// Call service to commit debit
	transactionID, err := s.walletService.CommitReservedDebit(ctx, req.ReservationId, req.Notes)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to commit debit")

		// Return appropriate error codes
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			return nil, status.Error(codes.NotFound, errMsg)
		}
		if strings.Contains(errMsg, "already processed") || strings.Contains(errMsg, "expired") {
			return nil, status.Error(codes.FailedPrecondition, errMsg)
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to commit debit: %v", err))
	}

	// Get wallet type from the transaction to fetch balance
	// For now, we'll try both wallet types to get the balance
	var newBalance int64
	stakeBalance, err1 := s.walletService.GetRetailerBalance(ctx, retailerID, models.WalletTypeRetailerStake)
	winningBalance, err2 := s.walletService.GetRetailerBalance(ctx, retailerID, models.WalletTypeRetailerWinning)

	if err1 == nil {
		newBalance = stakeBalance
	} else if err2 == nil {
		newBalance = winningBalance
	}

	span.SetAttributes(
		attribute.String("transaction_id", transactionID.String()),
		attribute.Int64("new_balance", newBalance),
	)

	return &pb.CommitReservedDebitResponse{
		Success:       true,
		TransactionId: transactionID.String(),
		DebitedAmount: 0, // We don't have this info readily available
		NewBalance:    float64(newBalance),
		Message:       "Debit committed successfully",
		CommittedAt:   timestamppb.Now(),
	}, nil
}

// ReleaseReservation releases a fund reservation (compensation/cancellation)
func (s *WalletServer) ReleaseReservation(ctx context.Context, req *pb.ReleaseReservationRequest) (*pb.ReleaseReservationResponse, error) {
	ctx, span := s.tracer.Start(ctx, "ReleaseReservation")
	defer span.End()

	// Validate request
	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}
	if req.ReservationId == "" {
		return nil, status.Error(codes.InvalidArgument, "reservation_id is required")
	}

	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid retailer_id format")
	}

	span.SetAttributes(
		attribute.String("retailer.id", req.RetailerId),
		attribute.String("reservation_id", req.ReservationId),
		attribute.String("reason", req.Reason),
	)

	// Call service to release reservation
	err = s.walletService.ReleaseReservation(ctx, req.ReservationId, req.Reason)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to release reservation")

		// Return appropriate error codes
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			return nil, status.Error(codes.NotFound, errMsg)
		}
		// Don't fail if already released (idempotency)
		if strings.Contains(errMsg, "already processed") {
			return &pb.ReleaseReservationResponse{
				Success:        true,
				ReleasedAmount: 0,
				Message:        "Reservation already released",
			}, nil
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to release reservation: %v", err))
	}

	// Get updated balance (try both wallet types)
	var newBalance int64
	stakeBalance, err1 := s.walletService.GetRetailerBalance(ctx, retailerID, models.WalletTypeRetailerStake)
	winningBalance, err2 := s.walletService.GetRetailerBalance(ctx, retailerID, models.WalletTypeRetailerWinning)

	if err1 == nil {
		newBalance = stakeBalance
	} else if err2 == nil {
		newBalance = winningBalance
	}

	span.SetAttributes(
		attribute.String("released", "true"),
		attribute.Int64("new_balance", newBalance),
	)

	return &pb.ReleaseReservationResponse{
		Success:        true,
		ReleasedAmount: 0, // We don't have this info readily available
		Message:        "Reservation released successfully",
	}, nil
}

// GetAllTransactions retrieves all transactions with filters (Admin only)
func (s *WalletServer) GetAllTransactions(ctx context.Context, req *pb.GetAllTransactionsRequest) (*pb.GetAllTransactionsResponse, error) {
	ctx, span := s.tracer.Start(ctx, "GetAllTransactions")
	defer span.End()

	// Set default pagination
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	span.SetAttributes(
		attribute.Int("page", int(page)),
		attribute.Int("page_size", int(pageSize)),
		attribute.Int("transaction_types_count", len(req.TransactionTypes)),
		attribute.Int("wallet_types_count", len(req.WalletTypes)),
		attribute.Int("statuses_count", len(req.Statuses)),
	)

	// Convert proto enums to model types
	var transactionTypes []models.TransactionType
	for _, protoType := range req.TransactionTypes {
		switch protoType {
		case pb.TransactionType_CREDIT:
			transactionTypes = append(transactionTypes, models.TransactionTypeCredit)
		case pb.TransactionType_DEBIT:
			transactionTypes = append(transactionTypes, models.TransactionTypeDebit)
		case pb.TransactionType_TRANSFER:
			transactionTypes = append(transactionTypes, models.TransactionTypeTransfer)
		case pb.TransactionType_COMMISSION:
			transactionTypes = append(transactionTypes, models.TransactionTypeCommission)
		case pb.TransactionType_PAYOUT:
			transactionTypes = append(transactionTypes, models.TransactionTypePayout)
		}
	}

	var walletTypes []models.WalletType
	for _, protoWalletType := range req.WalletTypes {
		switch protoWalletType {
		case pb.WalletType_AGENT_STAKE:
			walletTypes = append(walletTypes, models.WalletTypeAgentStake)
		case pb.WalletType_RETAILER_STAKE:
			walletTypes = append(walletTypes, models.WalletTypeRetailerStake)
		case pb.WalletType_RETAILER_WINNING:
			walletTypes = append(walletTypes, models.WalletTypeRetailerWinning)
		case pb.WalletType_PLAYER_WALLET:
			walletTypes = append(walletTypes, models.WalletTypePlayerWallet)
		}
	}

	var statuses []models.TransactionStatus
	for _, protoStatus := range req.Statuses {
		switch protoStatus {
		case pb.TransactionStatus_PENDING:
			statuses = append(statuses, models.TransactionStatusPending)
		case pb.TransactionStatus_COMPLETED:
			statuses = append(statuses, models.TransactionStatusCompleted)
		case pb.TransactionStatus_FAILED:
			statuses = append(statuses, models.TransactionStatusFailed)
		case pb.TransactionStatus_REVERSED:
			statuses = append(statuses, models.TransactionStatusReversed)
		}
	}

	// Convert timestamps to strings
	var startDate, endDate *string
	if req.StartDate != nil {
		startDateStr := req.StartDate.AsTime().Format(time.RFC3339)
		startDate = &startDateStr
	}
	if req.EndDate != nil {
		endDateStr := req.EndDate.AsTime().Format(time.RFC3339)
		endDate = &endDateStr
	}

	// Prepare search term
	var searchTerm *string
	if req.SearchTerm != "" {
		searchTerm = &req.SearchTerm
	}

	// Build filters
	filters := repositories.AdminTransactionFilters{
		TransactionTypes: transactionTypes,
		WalletTypes:      walletTypes,
		Statuses:         statuses,
		StartDate:        startDate,
		EndDate:          endDate,
		SearchTerm:       searchTerm,
		Page:             int(page),
		PageSize:         int(pageSize),
		SortBy:           req.SortBy,
		SortOrder:        req.SortOrder,
	}

	// Get transactions from service
	transactions, totalCount, err := s.walletService.GetAllTransactions(ctx, filters)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get all transactions")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get transactions: %v", err))
	}

	// Get statistics using the same filters
	stats, err := s.walletService.GetTransactionStatistics(ctx, filters)
	if err != nil {
		span.RecordError(err)
		// Don't fail the request if statistics fail, just log it
		span.AddEvent("statistics_error", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		// Set stats to nil, will be handled below
		stats = nil
	}

	// Convert to proto format
	pbTransactions := make([]*pb.TransactionDetail, len(transactions))
	for i, tx := range transactions {
		// Map transaction type
		var protoTxType pb.TransactionType
		switch tx.TransactionType {
		case models.TransactionTypeCredit:
			protoTxType = pb.TransactionType_CREDIT
		case models.TransactionTypeDebit:
			protoTxType = pb.TransactionType_DEBIT
		case models.TransactionTypeTransfer:
			protoTxType = pb.TransactionType_TRANSFER
		case models.TransactionTypeCommission:
			protoTxType = pb.TransactionType_COMMISSION
		case models.TransactionTypePayout:
			protoTxType = pb.TransactionType_PAYOUT
		default:
			protoTxType = pb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
		}

		// Map wallet type
		var protoWalletType pb.WalletType
		switch tx.WalletType {
		case models.WalletTypeAgentStake:
			protoWalletType = pb.WalletType_AGENT_STAKE
		case models.WalletTypeRetailerStake:
			protoWalletType = pb.WalletType_RETAILER_STAKE
		case models.WalletTypeRetailerWinning:
			protoWalletType = pb.WalletType_RETAILER_WINNING
		case models.WalletTypePlayerWallet:
			protoWalletType = pb.WalletType_PLAYER_WALLET
		default:
			protoWalletType = pb.WalletType_WALLET_TYPE_UNSPECIFIED
		}

		// Map status
		var protoStatus pb.TransactionStatus
		switch tx.Status {
		case models.TransactionStatusPending:
			protoStatus = pb.TransactionStatus_PENDING
		case models.TransactionStatusCompleted:
			protoStatus = pb.TransactionStatus_COMPLETED
		case models.TransactionStatusFailed:
			protoStatus = pb.TransactionStatus_FAILED
		case models.TransactionStatusReversed:
			protoStatus = pb.TransactionStatus_REVERSED
		default:
			protoStatus = pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
		}

		// Handle nullable fields
		reference := ""
		if tx.Reference != nil {
			reference = *tx.Reference
		}

		description := ""
		if tx.Description != nil {
			description = *tx.Description
		}

		// Extract owner name and code from metadata
		ownerName := ""
		ownerCode := ""
		if tx.Metadata != nil {
			if name, ok := tx.Metadata["owner_name"].(string); ok {
				ownerName = name
			}
			if code, ok := tx.Metadata["owner_code"].(string); ok {
				ownerCode = code
			}
		}

		// If owner name/code not in metadata, fetch from agent management service
		if (ownerName == "" || ownerCode == "") && s.agentClient != nil {
			switch tx.WalletType {
			case models.WalletTypeAgentStake:
				// Fetch agent details
				log.Printf("[FETCH] Agent name for owner=%s", tx.WalletOwnerID.String()[:8])
				if agent, err := s.agentClient.GetAgent(ctx, tx.WalletOwnerID); err == nil {
					ownerName = agent.Name
					ownerCode = agent.AgentCode
					log.Printf("[FETCHED] Agent: name=%s code=%s", agent.Name, agent.AgentCode)
					span.AddEvent("fetched_agent_details", trace.WithAttributes(
						attribute.String("agent.name", agent.Name),
						attribute.String("agent.code", agent.AgentCode),
					))
				} else {
					log.Printf("[FETCH ERROR] Agent fetch failed: %v", err)
				}
			case models.WalletTypeRetailerStake, models.WalletTypeRetailerWinning:
				// Fetch retailer details
				log.Printf("[FETCH] Retailer name for owner=%s", tx.WalletOwnerID.String()[:8])
				if retailer, err := s.agentClient.GetRetailer(ctx, tx.WalletOwnerID); err == nil {
					ownerName = retailer.Name
					ownerCode = retailer.RetailerCode
					log.Printf("[FETCHED] Retailer: name=%s code=%s", retailer.Name, retailer.RetailerCode)
					span.AddEvent("fetched_retailer_details", trace.WithAttributes(
						attribute.String("retailer.name", retailer.Name),
						attribute.String("retailer.code", retailer.RetailerCode),
					))
				} else {
					log.Printf("[FETCH ERROR] Retailer fetch failed: %v", err)
				}
			}
		}

		// Convert metadata map to proto format
		protoMetadata := make(map[string]string)
		if tx.Metadata != nil {
			for k, v := range tx.Metadata {
				protoMetadata[k] = fmt.Sprintf("%v", v)
			}
		}

		pbTransactions[i] = &pb.TransactionDetail{
			Id:              tx.ID.String(),
			TransactionId:   tx.TransactionID,
			WalletOwnerId:   tx.WalletOwnerID.String(),
			WalletOwnerName: ownerName,
			WalletOwnerCode: ownerCode,
			WalletType:      protoWalletType,
			Type:            protoTxType,
			Amount:          float64(tx.Amount),        // Return pesewas
			BalanceBefore:   float64(tx.BalanceBefore), // Return pesewas
			BalanceAfter:    float64(tx.BalanceAfter),  // Return pesewas
			Reference:       reference,
			Description:     description,
			Status:          protoStatus,
			CreatedAt:       timestamppb.New(tx.CreatedAt),
			CompletedAt:     nil,
			ReversedAt:      nil,
			Metadata:        protoMetadata,
		}

		if tx.CompletedAt != nil {
			pbTransactions[i].CompletedAt = timestamppb.New(*tx.CompletedAt)
		}
		if tx.ReversedAt != nil {
			pbTransactions[i].ReversedAt = timestamppb.New(*tx.ReversedAt)
		}
	}

	// Calculate pagination metadata
	totalPages := int32((totalCount + int(pageSize) - 1) / int(pageSize))
	hasMore := page < totalPages

	span.SetAttributes(
		attribute.Int("transactions.count", len(transactions)),
		attribute.Int("total.count", totalCount),
		attribute.Int("total.pages", int(totalPages)),
	)

	// Convert statistics to proto format (if available)
	var pbStats *pb.TransactionStatistics
	if stats != nil {
		pbStats = &pb.TransactionStatistics{
			TotalVolume:     float64(stats.TotalVolume),
			TotalCredits:    float64(stats.TotalCredits),
			TotalDebits:     float64(stats.TotalDebits),
			PendingAmount:   float64(stats.PendingAmount),
			PendingCount:    int32(stats.PendingCount),
			CompletedCount:  int32(stats.CompletedCount),
			FailedCount:     int32(stats.FailedCount),
			CreditCount:     int32(stats.CreditCount),
			DebitCount:      int32(stats.DebitCount),
			TransferCount:   int32(stats.TransferCount),
			CommissionCount: int32(stats.CommissionCount),
			PayoutCount:     int32(stats.PayoutCount),
		}

		span.SetAttributes(
			attribute.Int64("stats.total_volume", stats.TotalVolume),
			attribute.Int64("stats.total_credits", stats.TotalCredits),
			attribute.Int64("stats.total_debits", stats.TotalDebits),
			attribute.Int("stats.pending_count", stats.PendingCount),
			attribute.Int("stats.completed_count", stats.CompletedCount),
		)
	}

	span.SetStatus(otelcodes.Ok, "transactions retrieved successfully")

	return &pb.GetAllTransactionsResponse{
		Transactions: pbTransactions,
		TotalCount:   int32(totalCount),
		Page:         page,
		PageSize:     pageSize,
		TotalPages:   totalPages,
		HasMore:      hasMore,
		Statistics:   pbStats,
	}, nil
}

// CreatePlayerWallet creates a unified player wallet
func (s *WalletServer) CreatePlayerWallet(ctx context.Context, req *pb.CreatePlayerWalletRequest) (*pb.CreatePlayerWalletResponse, error) {
	ctx, span := s.tracer.Start(ctx, "CreatePlayerWallet")
	defer span.End()

	if req.PlayerId == "" {
		return nil, status.Error(codes.InvalidArgument, "player_id is required")
	}
	if req.PlayerCode == "" {
		return nil, status.Error(codes.InvalidArgument, "player_code is required")
	}

	span.SetAttributes(
		attribute.String("player.id", req.PlayerId),
		attribute.String("player.code", req.PlayerCode),
	)

	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid player ID format")
		return nil, status.Error(codes.InvalidArgument, "invalid player_id format")
	}

	walletID, err := s.walletService.CreatePlayerWallet(ctx, playerID, req.PlayerCode)
	if err != nil || walletID == uuid.Nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to create player wallet")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create player wallet: %v", err))
	}

	span.SetStatus(otelcodes.Ok, "player wallet created successfully")
	return &pb.CreatePlayerWalletResponse{
		Success:   true,
		WalletId:  walletID.String(),
		PlayerId:  req.PlayerId,
		Message:   "Player wallet created successfully",
		CreatedAt: timestamppb.New(time.Now()),
	}, nil
}

// CreditPlayerWallet credits a player's wallet
func (s *WalletServer) CreditPlayerWallet(ctx context.Context, req *pb.CreditPlayerWalletRequest) (*pb.CreditPlayerWalletResponse, error) {
	ctx, span := s.tracer.Start(ctx, "CreditPlayerWallet")
	defer span.End()

	if req.PlayerId == "" {
		return nil, status.Error(codes.InvalidArgument, "player_id is required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than 0")
	}

	span.SetAttributes(
		attribute.String("player.id", req.PlayerId),
		attribute.Float64("amount", req.Amount),
		attribute.String("reference", req.Reference),
	)

	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid player ID format")
		return nil, status.Error(codes.InvalidArgument, "invalid player_id format")
	}

	// Convert proto CreditSource to model CreditSource
	creditSource := creditSourceFromProto(req.CreditSource)

	// assuming req.Amount is in pesewas
	amountPesewas := int64(req.Amount)

	span.SetAttributes(
		attribute.String("credit_source", string(creditSource)),
	)

	transaction, err := s.walletService.CreditPlayerWallet(ctx, playerID, amountPesewas, req.Notes, req.IdempotencyKey, creditSource)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to credit player wallet")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to credit player wallet: %v", err))
	}

	span.SetStatus(otelcodes.Ok, "player wallet credited successfully")
	return &pb.CreditPlayerWalletResponse{
		Success:        true,
		TransactionId:  transaction.TransactionID,
		CreditedAmount: req.Amount,
		NewBalance:     float64(transaction.BalanceAfter),
		Message:        "Player wallet credited successfully",
		Timestamp:      timestamppb.New(transaction.CreatedAt),
	}, nil
}

// DebitPlayerWallet debits a player's wallet
func (s *WalletServer) DebitPlayerWallet(ctx context.Context, req *pb.DebitPlayerWalletRequest) (*pb.DebitPlayerWalletResponse, error) {
	ctx, span := s.tracer.Start(ctx, "DebitPlayerWallet")
	defer span.End()

	if req.PlayerId == "" {
		return nil, status.Error(codes.InvalidArgument, "player_id is required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than 0")
	}

	span.SetAttributes(
		attribute.String("player.id", req.PlayerId),
		attribute.Float64("amount", req.Amount),
		attribute.String("reference", req.Reference),
	)

	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid player ID format")
		return nil, status.Error(codes.InvalidArgument, "invalid player_id format")
	}

	// assuming req.Amount is in pesewas
	amountPesewas := int64(req.Amount)

	// Format description consistently with retailer debits
	description := fmt.Sprintf("Debit - Ref: %s", req.Reference)
	if req.Reason != "" {
		description += " - " + req.Reason
	}

	transaction, err := s.walletService.DebitPlayerWallet(ctx, playerID, amountPesewas, description)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to debit player wallet")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to debit player wallet: %v", err))
	}

	span.SetStatus(otelcodes.Ok, "player wallet debited successfully")
	return &pb.DebitPlayerWalletResponse{
		Success:       true,
		TransactionId: transaction.TransactionID,
		DebitedAmount: req.Amount,
		NewBalance:    float64(transaction.BalanceAfter),
		Message:       "Player wallet debited successfully",
		Timestamp:     timestamppb.New(transaction.CreatedAt),
	}, nil
}

func (s *WalletServer) ReservePlayerWalletFunds(ctx context.Context, req *pb.ReservePlayerWalletFundsRequest) (*pb.ReservePlayerWalletFundsResponse, error) {
	ctx, span := s.tracer.Start(ctx, "ReservePlayerWalletFunds")
	defer span.End()

	if req.PlayerId == "" {
		return nil, status.Error(codes.InvalidArgument, "player_id is required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than 0")
	}
	if req.TtlSeconds <= 0 {
		return nil, status.Error(codes.InvalidArgument, "ttl_seconds must be greater than 0")
	}

	span.SetAttributes(
		attribute.String("player.id", req.PlayerId),
		attribute.Float64("amount", req.Amount),
		attribute.String("reference", req.Reference),
		attribute.Int("ttl_seconds", int(req.TtlSeconds)),
	)

	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid player ID format")
		return nil, status.Error(codes.InvalidArgument, "invalid player_id format")
	}

	amountPesewas := int64(req.Amount)

	reservationID, err := s.walletService.ReservePlayerWalletFunds(ctx, playerID, amountPesewas, req.Reference, req.TtlSeconds, req.Reason, "")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to reserve player wallet funds")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to reserve player wallet funds: %v", err))
	}

	expiresAt := time.Now().Add(time.Duration(req.TtlSeconds) * time.Second)

	wallet, err := s.walletService.GetPlayerBalance(ctx, playerID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get player balance")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get player balance: %v", err))
	}

	span.SetStatus(otelcodes.Ok, "player wallet funds reserved successfully")
	return &pb.ReservePlayerWalletFundsResponse{
		Success:          true,
		ReservationId:    reservationID,
		ReservedAmount:   req.Amount,
		AvailableBalance: float64(wallet.AvailableBalance),
		Message:          "Player wallet funds reserved successfully",
		ReservedAt:       timestamppb.New(time.Now()),
		ExpiresAt:        timestamppb.New(expiresAt),
	}, nil
}

func (s *WalletServer) GetPlayerWalletBalance(ctx context.Context, req *pb.GetPlayerWalletBalanceRequest) (*pb.GetPlayerWalletBalanceResponse, error) {
	ctx, span := s.tracer.Start(ctx, "GetPlayerWalletBalance")
	defer span.End()

	if req.PlayerId == "" {
		return nil, status.Error(codes.InvalidArgument, "player_id is required")
	}

	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid player_id format")
	}

	wallet, err := s.walletService.GetPlayerBalance(ctx, playerID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get player balance")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get player balance: %v", err))
	}

	return &pb.GetPlayerWalletBalanceResponse{
		PlayerId:         req.PlayerId,
		Balance:          float64(wallet.Balance),
		PendingBalance:   float64(wallet.PendingBalance),
		AvailableBalance: float64(wallet.AvailableBalance),
		LastUpdated:      timestamppb.New(wallet.UpdatedAt),
	}, nil
}

// ReverseTransaction reverses a completed credit transaction (Admin only)
func (s *WalletServer) ReverseTransaction(ctx context.Context, req *pb.ReverseTransactionRequest) (*pb.ReverseTransactionResponse, error) {
	ctx, span := s.tracer.Start(ctx, "ReverseTransaction")
	defer span.End()

	// Validate request
	if req.TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id is required")
	}
	if req.Reason == "" {
		return nil, status.Error(codes.InvalidArgument, "reason is required")
	}
	if len(req.Reason) < 20 {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("reason must be at least 20 characters, got %d", len(req.Reason)))
	}
	if req.AdminId == "" {
		return nil, status.Error(codes.InvalidArgument, "admin_id is required")
	}
	if req.AdminName == "" {
		return nil, status.Error(codes.InvalidArgument, "admin_name is required")
	}
	if req.AdminEmail == "" {
		return nil, status.Error(codes.InvalidArgument, "admin_email is required")
	}

	// Parse transaction ID
	txID, err := uuid.Parse(req.TransactionId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid transaction_id format: %v", err))
	}

	// Parse admin ID
	adminID, err := uuid.Parse(req.AdminId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid admin_id format: %v", err))
	}

	span.SetAttributes(
		attribute.String("transaction.id", txID.String()),
		attribute.String("admin.id", adminID.String()),
		attribute.String("reason", req.Reason),
	)

	// Call service to perform reversal
	originalTx, reversalTx, err := s.walletService.ReverseTransaction(
		ctx,
		txID,
		req.Reason,
		adminID,
		req.AdminName,
		req.AdminEmail,
	)
	if err != nil {
		// Map errors to appropriate gRPC codes
		errMsg := err.Error()
		span.RecordError(err)

		if strings.Contains(errMsg, "not found") {
			span.SetStatus(otelcodes.Error, "transaction not found")
			return nil, status.Error(codes.NotFound, errMsg)
		}
		if strings.Contains(errMsg, "only CREDIT") || strings.Contains(errMsg, "only COMPLETED") {
			span.SetStatus(otelcodes.Error, "invalid transaction state")
			return nil, status.Error(codes.FailedPrecondition, errMsg)
		}
		if strings.Contains(errMsg, "already reversed") {
			span.SetStatus(otelcodes.Error, "duplicate reversal")
			return nil, status.Error(codes.AlreadyExists, errMsg)
		}
		if strings.Contains(errMsg, "too old") {
			span.SetStatus(otelcodes.Error, "transaction too old")
			return nil, status.Error(codes.FailedPrecondition, errMsg)
		}

		span.SetStatus(otelcodes.Error, "reversal failed")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to reverse transaction: %v", err))
	}

	// Convert transactions to proto format
	originalTxDetail := &pb.TransactionDetail{
		Id:            originalTx.ID.String(),
		TransactionId: originalTx.TransactionID,
		WalletOwnerId: originalTx.WalletOwnerID.String(),
		WalletType:    walletTypeToProto(originalTx.WalletType),
		Type:          transactionTypeToProto(originalTx.TransactionType),
		Amount:        float64(originalTx.Amount),
		BalanceBefore: float64(originalTx.BalanceBefore),
		BalanceAfter:  float64(originalTx.BalanceAfter),
		Status:        transactionStatusToProto(originalTx.Status),
		CreatedAt:     timestamppb.New(originalTx.CreatedAt),
	}
	if originalTx.Description != nil {
		originalTxDetail.Description = *originalTx.Description
	}
	if originalTx.Reference != nil {
		originalTxDetail.Reference = *originalTx.Reference
	}

	reversalTxDetail := &pb.TransactionDetail{
		Id:            reversalTx.ID.String(),
		TransactionId: reversalTx.TransactionID,
		WalletOwnerId: reversalTx.WalletOwnerID.String(),
		WalletType:    walletTypeToProto(reversalTx.WalletType),
		Type:          transactionTypeToProto(reversalTx.TransactionType),
		Amount:        float64(reversalTx.Amount),
		BalanceBefore: float64(reversalTx.BalanceBefore),
		BalanceAfter:  float64(reversalTx.BalanceAfter),
		Status:        transactionStatusToProto(reversalTx.Status),
		CreatedAt:     timestamppb.New(reversalTx.CreatedAt),
	}
	if reversalTx.Description != nil {
		reversalTxDetail.Description = *reversalTx.Description
	}
	if reversalTx.Reference != nil {
		reversalTxDetail.Reference = *reversalTx.Reference
	}

	balanceIsNegative := reversalTx.BalanceAfter < 0

	span.SetAttributes(
		attribute.String("reversal.transaction_id", reversalTx.ID.String()),
		attribute.Int64("new.balance", reversalTx.BalanceAfter),
		attribute.Bool("balance.is_negative", balanceIsNegative),
	)

	span.SetStatus(otelcodes.Ok, "transaction reversed successfully")

	return &pb.ReverseTransactionResponse{
		Success:               true,
		Message:               "Transaction reversed successfully",
		ReversalTransactionId: reversalTx.ID.String(),
		ReversedAmount:        float64(originalTx.Amount),
		NewWalletBalance:      float64(reversalTx.BalanceAfter),
		ReversedAt:            timestamppb.New(*originalTx.ReversedAt),
		OriginalTransaction:   originalTxDetail,
		ReversalTransaction:   reversalTxDetail,
		BalanceIsNegative:     balanceIsNegative,
	}, nil
}

func (s *WalletServer) PlaceHoldOnWallet(ctx context.Context, req *pb.PlaceHoldOnWalletRequest) (*pb.PlaceHoldOnWalletResponse, error) {

	ctx, span := s.tracer.Start(ctx, "PlaceHoldOnWallet")
	defer span.End()

	// Validate request params
	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}
	if req.PlacedBy == "" {
		return nil, status.Error(codes.InvalidArgument, "placed_by is required")
	}

	placedByID, err := uuid.Parse(req.PlacedBy)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid placed_by format")
	}
	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid retailer_id format")
	}

	err = s.walletService.PlaceHoldOnWallet(ctx, retailerID, placedByID, req.Reason, req.ExpiresAt.AsTime())
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to place hold on wallet: %v", err))
	}

	return &pb.PlaceHoldOnWalletResponse{
		RetailerId: retailerID.String(),
		PlacedBy:   placedByID.String(),
		ExpiresAt:  req.ExpiresAt,
	}, nil
}

func (s *WalletServer) GetHoldOnWallet(ctx context.Context, req *pb.GetHoldOnWalletRequest) (*pb.GetHoldOnWalletResponse, error) {
	ctx, span := s.tracer.Start(ctx, "GetHoldOnWallet")
	defer span.End()

	if req.GetHoldId() == "" {
		return nil, status.Error(codes.InvalidArgument, "hold_id is required")
	}

	holdID, err := uuid.Parse(req.GetHoldId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid hold_id format")
	}

	hold, err := s.walletService.GetHoldOnWallet(ctx, holdID)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get wallet hold: %v", err))
	}
	if hold == nil {
		return &pb.GetHoldOnWalletResponse{}, nil
	}

	return &pb.GetHoldOnWalletResponse{Hold: convertHoldToProto(hold)}, nil
}

func (s *WalletServer) GetHoldByRetailer(ctx context.Context, req *pb.GetHoldByRetailerRequest) (*pb.GetHoldByRetailerResponse, error) {
	ctx, span := s.tracer.Start(ctx, "GetHoldByRetailer")
	defer span.End()

	if req.GetRetailerId() == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}
	if req.GetWalletType() != pb.WalletType_RETAILER_WINNING && req.GetWalletType() != pb.WalletType_WALLET_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "wallet_type must be RETAILER_WINNING")
	}

	retailerID, err := uuid.Parse(req.GetRetailerId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid retailer_id format")
	}

	hold, err := s.walletService.GetHoldByRetailer(ctx, retailerID)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get wallet hold: %v", err))
	}
	if hold == nil {
		return &pb.GetHoldByRetailerResponse{}, nil
	}
	return &pb.GetHoldByRetailerResponse{Hold: convertHoldToProto(hold)}, nil
}

func (s *WalletServer) ReleaseHoldOnWallet(ctx context.Context, req *pb.ReleaseHoldOnWalletRequest) (*pb.ReleaseHoldOnWalletResponse, error) {
	ctx, span := s.tracer.Start(ctx, "ReleaseHoldOnWallet")
	defer span.End()

	if req.GetHoldId() == "" || req.GetReleasedBy() == "" {
		return nil, status.Error(codes.InvalidArgument, "hold_id and released_by are required")
	}

	holdID, err := uuid.Parse(req.GetHoldId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid hold_id format")
	}

	// Load hold to get retailer context
	currentHold, err := s.walletService.GetHoldOnWallet(ctx, holdID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load hold")
	}
	if currentHold == nil {
		return nil, status.Error(codes.NotFound, "hold not found")
	}

	releasedByID, err := uuid.Parse(req.GetReleasedBy())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid released_by format")
	}

	if err := s.walletService.ReleaseHoldOnWallet(ctx, currentHold.WalletID, currentHold.RetailerID, releasedByID); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to release wallet hold: %v", err))
	}

	updatedHold, _ := s.walletService.GetHoldOnWallet(ctx, currentHold.WalletID)
	return &pb.ReleaseHoldOnWalletResponse{Success: true, Hold: convertHoldToProto(updatedHold)}, nil
}

func convertHoldToProto(h *models.RetailerWinningWalletHold) *pb.Hold {
	if h == nil {
		return nil
	}
	return &pb.Hold{
		Id:         h.WalletID.String(),
		RetailerId: h.RetailerID.String(),
		WalletType: pb.WalletType_RETAILER_WINNING,
		Status:     string(h.Status),
		Reason:     h.Reason,
		CreatedBy:  h.PlacedBy.String(),
		ReleasedBy: "",
		CreatedAt:  timestamppb.New(h.CreatedAt),
		UpdatedAt:  timestamppb.New(h.UpdatedAt),
		ReleasedAt: nil,
		Metadata:   map[string]string{},
	}
}

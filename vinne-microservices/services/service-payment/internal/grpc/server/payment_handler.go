package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	pb "github.com/randco/randco-microservices/proto/payment/v1"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/service-payment/internal/models"
	"github.com/randco/service-payment/internal/providers"
	"github.com/randco/service-payment/internal/repositories"
	"github.com/randco/service-payment/internal/saga"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var tracer = otel.Tracer("payment-service/grpc")

// PaymentHandler implements the gRPC PaymentService interface
type PaymentHandler struct {
	pb.UnimplementedPaymentServiceServer
	transactionRepo      repositories.TransactionRepository
	idempotencyRepo      repositories.IdempotencyRepository
	webhookEventRepo     repositories.WebhookEventRepository
	providerFactory      *providers.ProviderFactory
	depositSaga          *saga.DepositSaga
	playerDepositSaga    *saga.PlayerDepositSaga
	withdrawalSaga       *saga.WithdrawalSaga
	logger               logger.Logger
	orangeCallbackSecret string
}

// NewPaymentHandler creates a new payment handler
func NewPaymentHandler(
	transactionRepo repositories.TransactionRepository,
	idempotencyRepo repositories.IdempotencyRepository,
	webhookEventRepo repositories.WebhookEventRepository,
	providerFactory *providers.ProviderFactory,
	depositSaga *saga.DepositSaga,
	playerDepositSaga *saga.PlayerDepositSaga,
	withdrawalSaga *saga.WithdrawalSaga,
	orangeCallbackSecret string,
	logger logger.Logger,
) *PaymentHandler {
	return &PaymentHandler{
		transactionRepo:      transactionRepo,
		idempotencyRepo:      idempotencyRepo,
		webhookEventRepo:     webhookEventRepo,
		providerFactory:      providerFactory,
		depositSaga:          depositSaga,
		playerDepositSaga:    playerDepositSaga,
		withdrawalSaga:       withdrawalSaga,
		orangeCallbackSecret: orangeCallbackSecret,
		logger:               logger,
	}
}

// InitiateDeposit handles deposit initiation (Mobile Money -> Stake Wallet)
func (h *PaymentHandler) InitiateDeposit(ctx context.Context, req *pb.InitiateDepositRequest) (*pb.InitiateDepositResponse, error) {
	ctx, span := tracer.Start(ctx, "payment_handler.initiate_deposit",
		trace.WithAttributes(
			attribute.String("user_id", req.UserId),
			attribute.String("reference", req.Reference),
			attribute.Int64("amount", req.Amount),
			attribute.String("provider", req.WalletProvider.String()),
		))
	defer span.End()

	// Validate request
	if err := h.validateDepositRequest(req); err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Check for duplicate transaction (idempotency)
	if existing, err := h.idempotencyRepo.GetByKey(ctx, req.Reference); err == nil && existing != nil && existing.TransactionID != nil {
		h.logger.Info("Duplicate deposit request detected", "reference", req.Reference, "transaction_id", existing.TransactionID)
		span.SetAttributes(attribute.Bool("duplicate", true))

		// Return existing transaction
		txn, err := h.transactionRepo.GetByID(ctx, *existing.TransactionID)
		if err != nil {
			span.RecordError(err)
			return nil, status.Error(codes.Internal, "failed to retrieve existing transaction")
		}

		return &pb.InitiateDepositResponse{
			Success:     true,
			Message:     "Duplicate request - returning existing transaction",
			Transaction: h.transactionToProto(txn),
		}, nil
	}

	// Create transaction record
	now := time.Now()
	txn := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             req.Reference,
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                req.Amount,
		Currency:              req.Currency,
		Narration:             req.Narration,
		SourceType:            "mobile_money",
		SourceIdentifier:      req.WalletNumber,
		SourceName:            h.walletProviderToString(req.WalletProvider), // Provider name (e.g., "telecel")
		DestinationType:       "stake_wallet",
		DestinationIdentifier: req.UserId,
		DestinationName:       req.CustomerName, // Customer name goes to destination
		UserID:                uuid.MustParse(req.UserId),
		ProviderName:          h.walletProviderToString(req.WalletProvider),
		Metadata:              req.Metadata,
		RequestedAt:           now,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	// Initialize maps
	if txn.Metadata == nil {
		txn.Metadata = make(map[string]string)
	}
	txn.ProviderData = make(map[string]interface{})

	// Save transaction
	if err := h.transactionRepo.Create(ctx, txn); err != nil {
		span.RecordError(err)
		h.logger.Error("Failed to create deposit transaction", "error", err, "reference", req.Reference)
		return nil, status.Error(codes.Internal, "failed to create transaction")
	}

	// Record idempotency key (only if not already exists)
	// Use proper initialization to avoid constraint violations
	idempotencyRecord := models.CreateIdempotencyRecord(
		req.Reference, // idempotency key
		"",            // request hash (not needed for deposit initiation)
		"InitiateDeposit",
		"POST",
		24*time.Hour, // TTL: 24 hours
	)
	idempotencyRecord.TransactionID = &txn.ID
	if err := h.idempotencyRepo.Create(ctx, idempotencyRecord); err != nil {
		h.logger.Error("Failed to record idempotency key", "error", err, "reference", req.Reference)
		// Continue - transaction is already created
		// If it's a duplicate, that's okay (ON CONFLICT handles it gracefully)
	}

	// Determine user type from metadata
	userRole := ""
	if txn.Metadata != nil {
		userRole = txn.Metadata["user_role"]
	}

	// Execute appropriate saga asynchronously
	go func() {
		sagaCtx := context.Background()
		var err error
		if userRole == "player" {
			err = h.playerDepositSaga.Execute(sagaCtx, txn)
			if err != nil {
				h.logger.Error("Player deposit saga failed", "error", err, "transaction_id", txn.ID, "reference", txn.Reference)
			} else {
				h.logger.Info("Player deposit saga completed successfully", "transaction_id", txn.ID, "reference", txn.Reference)
			}
		} else {
			err = h.depositSaga.Execute(sagaCtx, txn)
			if err != nil {
				h.logger.Error("Deposit saga failed", "error", err, "transaction_id", txn.ID, "reference", txn.Reference)
			} else {
				h.logger.Info("Deposit saga completed successfully", "transaction_id", txn.ID, "reference", txn.Reference)
			}
		}

		// Update transaction status if saga failed (but not if it's pending confirmation)
		if err != nil {
			// Check if error is PENDING_CONFIRMATION (expected behavior)
			if strings.Contains(err.Error(), "PENDING_CONFIRMATION") {
				h.logger.Info("Deposit pending user confirmation - waiting for webhook",
					"transaction_id", txn.ID,
					"reference", txn.Reference)
				// Keep transaction as PENDING, don't mark as failed
				// Webhook will update when user confirms
			} else {
				// Real failure - mark as failed
				txn.Status = models.StatusFailed
				errMsg := err.Error()
				txn.ErrorMessage = &errMsg
				if updateErr := h.transactionRepo.Update(sagaCtx, txn); updateErr != nil {
					h.logger.Error("Failed to update transaction after saga failure", "error", updateErr, "transaction_id", txn.ID)
				}
			}
		}
	}()

	span.SetAttributes(attribute.String("transaction_id", txn.ID.String()))

	return &pb.InitiateDepositResponse{
		Success:     true,
		Message:     "Deposit initiated successfully",
		Transaction: h.transactionToProto(txn),
	}, nil
}

// InitiateWithdrawal handles withdrawal initiation (Winning Wallet -> Mobile Money)
func (h *PaymentHandler) InitiateWithdrawal(ctx context.Context, req *pb.InitiateWithdrawalRequest) (*pb.InitiateWithdrawalResponse, error) {
	ctx, span := tracer.Start(ctx, "payment_handler.initiate_withdrawal",
		trace.WithAttributes(
			attribute.String("user_id", req.UserId),
			attribute.String("reference", req.Reference),
			attribute.Int64("amount", req.Amount),
			attribute.String("provider", req.WalletProvider.String()),
		))
	defer span.End()

	// Validate request
	if err := h.validateWithdrawalRequest(req); err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Check for duplicate transaction (idempotency)
	if existing, err := h.idempotencyRepo.GetByKey(ctx, req.Reference); err == nil && existing != nil && existing.TransactionID != nil {
		h.logger.Info("Duplicate withdrawal request detected", "reference", req.Reference, "transaction_id", existing.TransactionID)
		span.SetAttributes(attribute.Bool("duplicate", true))

		// Return existing transaction
		txn, err := h.transactionRepo.GetByID(ctx, *existing.TransactionID)
		if err != nil {
			span.RecordError(err)
			return nil, status.Error(codes.Internal, "failed to retrieve existing transaction")
		}

		return &pb.InitiateWithdrawalResponse{
			Success:     true,
			Message:     "Duplicate request - returning existing transaction",
			Transaction: h.transactionToProto(txn),
		}, nil
	}

	// Create transaction record
	now := time.Now()
	txn := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             req.Reference,
		Type:                  models.TypeWithdrawal,
		Status:                models.StatusPending,
		Amount:                req.Amount,
		Currency:              req.Currency,
		Narration:             req.Narration,
		SourceType:            "winning_wallet",
		SourceIdentifier:      req.UserId,
		SourceName:            "Winning Wallet",
		DestinationType:       "mobile_money",
		DestinationIdentifier: req.WalletNumber,
		DestinationName:       req.CustomerName,
		UserID:                uuid.MustParse(req.UserId),
		ProviderName:          h.walletProviderToString(req.WalletProvider),
		Metadata:              req.Metadata,
		RequestedAt:           now,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	// Initialize maps
	if txn.Metadata == nil {
		txn.Metadata = make(map[string]string)
	}
	txn.ProviderData = make(map[string]interface{})

	// Save transaction
	if err := h.transactionRepo.Create(ctx, txn); err != nil {
		span.RecordError(err)
		h.logger.Error("Failed to create withdrawal transaction", "error", err, "reference", req.Reference)
		return nil, status.Error(codes.Internal, "failed to create transaction")
	}

	// Record idempotency key (only if not already exists)
	// Use proper initialization to avoid constraint violations
	idempotencyRecord := models.CreateIdempotencyRecord(
		req.Reference, // idempotency key
		"",            // request hash (not needed for withdrawal initiation)
		"InitiateWithdrawal",
		"POST",
		24*time.Hour, // TTL: 24 hours
	)
	idempotencyRecord.TransactionID = &txn.ID
	if err := h.idempotencyRepo.Create(ctx, idempotencyRecord); err != nil {
		h.logger.Error("Failed to record idempotency key", "error", err, "reference", req.Reference)
		// Continue - transaction is already created
		// If it's a duplicate, that's okay (ON CONFLICT handles it gracefully)
	}

	// Execute withdrawal saga asynchronously
	go func() {
		sagaCtx := context.Background()
		if err := h.withdrawalSaga.Execute(sagaCtx, txn); err != nil {
			h.logger.Error("Withdrawal saga failed", "error", err, "transaction_id", txn.ID, "reference", txn.Reference)
			// Update transaction status to failed
			txn.Status = models.StatusFailed
			errMsg := err.Error()
			txn.ErrorMessage = &errMsg
			if updateErr := h.transactionRepo.Update(sagaCtx, txn); updateErr != nil {
				h.logger.Error("Failed to update transaction after saga failure", "error", updateErr, "transaction_id", txn.ID)
			}
		} else {
			h.logger.Info("Withdrawal saga completed successfully", "transaction_id", txn.ID, "reference", txn.Reference)
		}
	}()

	span.SetAttributes(attribute.String("transaction_id", txn.ID.String()))

	return &pb.InitiateWithdrawalResponse{
		Success:     true,
		Message:     "Withdrawal initiated successfully",
		Transaction: h.transactionToProto(txn),
	}, nil
}

// GetTransaction retrieves a transaction by reference or ID
func (h *PaymentHandler) GetTransaction(ctx context.Context, req *pb.GetTransactionRequest) (*pb.GetTransactionResponse, error) {
	ctx, span := tracer.Start(ctx, "payment_handler.get_transaction",
		trace.WithAttributes(
			attribute.String("reference", req.Reference),
		))
	defer span.End()

	if req.Reference == "" {
		return nil, status.Error(codes.InvalidArgument, "reference is required")
	}

	// Try to parse as UUID first
	if id, err := uuid.Parse(req.Reference); err == nil {
		txn, err := h.transactionRepo.GetByID(ctx, id)
		if err != nil {
			span.RecordError(err)
			return nil, status.Error(codes.NotFound, "transaction not found")
		}
		return &pb.GetTransactionResponse{
			Success:     true,
			Message:     "Transaction retrieved successfully",
			Transaction: h.transactionToProto(txn),
		}, nil
	}

	// Otherwise, treat as reference
	txn, err := h.transactionRepo.GetByReference(ctx, req.Reference)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.NotFound, "transaction not found")
	}

	return &pb.GetTransactionResponse{
		Success:     true,
		Message:     "Transaction retrieved successfully",
		Transaction: h.transactionToProto(txn),
	}, nil
}

// GetDepositStatus retrieves deposit status
func (h *PaymentHandler) GetDepositStatus(ctx context.Context, req *pb.GetDepositStatusRequest) (*pb.GetDepositStatusResponse, error) {
	// Delegate to GetTransaction
	txnResp, err := h.GetTransaction(ctx, &pb.GetTransactionRequest{Reference: req.Reference})
	if err != nil {
		return nil, err
	}

	return &pb.GetDepositStatusResponse{
		Success:     txnResp.Success,
		Message:     txnResp.Message,
		Transaction: txnResp.Transaction,
	}, nil
}

// VerifyDepositStatus verifies deposit status by querying provider
func (h *PaymentHandler) VerifyDepositStatus(ctx context.Context, req *pb.VerifyDepositStatusRequest) (*pb.VerifyDepositStatusResponse, error) {
	ctx, span := tracer.Start(ctx, "payment_handler.verify_deposit_status",
		trace.WithAttributes(
			attribute.String("transaction_id", req.TransactionId),
			attribute.String("reference", req.Reference),
			attribute.Bool("force_refresh", req.ForceRefresh),
		))
	defer span.End()

	// Validate request - at least one identifier required
	if req.TransactionId == "" && req.Reference == "" {
		return nil, status.Error(codes.InvalidArgument, "either transaction_id or reference is required")
	}

	h.logger.Info("Verifying deposit status",
		"transaction_id", req.TransactionId,
		"reference", req.Reference,
		"force_refresh", req.ForceRefresh)

	// 1. Retrieve transaction from database
	var txn *models.Transaction
	var err error

	if req.Reference != "" {
		txn, err = h.transactionRepo.GetByReference(ctx, req.Reference)
	} else {
		txnID, parseErr := uuid.Parse(req.TransactionId)
		if parseErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid transaction_id format")
		}
		txn, err = h.transactionRepo.GetByID(ctx, txnID)
	}

	if err != nil {
		span.RecordError(err)
		h.logger.Error("Transaction not found", "error", err, "reference", req.Reference)
		return nil, status.Error(codes.NotFound, "transaction not found")
	}

	// 2. Check if already SUCCESS/FAILED (unless force_refresh)
	if !req.ForceRefresh && (txn.Status == models.StatusSuccess || txn.Status == models.StatusFailed) {
		h.logger.Info("Returning cached transaction status",
			"transaction_id", txn.ID,
			"status", txn.Status)

		return h.buildVerifyStatusResponse(txn), nil
	}

	// 3. Query provider for latest status
	provider, err := h.providerFactory.GetProvider(txn.ProviderName)
	if err != nil {
		span.RecordError(err)
		h.logger.Error("Failed to get provider", "provider", txn.ProviderName, "error", err)
		return nil, status.Error(codes.Internal, "provider not available")
	}

	providerTxID := ""
	if txn.ProviderTransactionID != nil {
		providerTxID = *txn.ProviderTransactionID
	}

	providerResp, err := provider.CheckTransactionStatus(ctx, providerTxID, txn.Reference)
	if err != nil {
		span.RecordError(err)
		h.logger.Error("Provider status check failed",
			"error", err,
			"provider", txn.ProviderName,
			"transaction_id", txn.ID)
		return nil, status.Error(codes.Internal, fmt.Sprintf("status check failed: %v", err))
	}

	h.logger.Info("Provider status retrieved",
		"transaction_id", txn.ID,
		"provider_status", providerResp.Status,
		"provider_status_code", providerResp.ProviderStatusCode)

	// 4. Map provider status to our status
	oldStatus := txn.Status
	txn.Status = h.mapProviderStatus(providerResp.Status)
	txn.ProviderTransactionID = &providerResp.TransactionID
	txn.UpdatedAt = time.Now()

	// Update completed timestamp if successful
	if txn.Status == models.StatusSuccess && providerResp.CompletedAt != nil {
		txn.CompletedAt = providerResp.CompletedAt
	}

	// Store provider data
	if txn.ProviderData == nil {
		txn.ProviderData = make(map[string]interface{})
	}
	txn.ProviderData["last_status_check"] = time.Now()
	txn.ProviderData["provider_status_code"] = providerResp.ProviderStatusCode
	for k, v := range providerResp.ProviderData {
		txn.ProviderData[k] = v
	}

	// 5. Update transaction in database
	if err := h.transactionRepo.Update(ctx, txn); err != nil {
		span.RecordError(err)
		h.logger.Error("Failed to update transaction after status verification",
			"error", err,
			"transaction_id", txn.ID)
		// Continue - we still return the status to user
	}

	span.SetAttributes(
		attribute.String("old_status", string(oldStatus)),
		attribute.String("new_status", string(txn.Status)),
	)

	// 6. If status changed to SUCCESS, log verification complete
	// NOTE: Saga continuation is handled during initial deposit - wallet credit happens immediately
	// This status verification is for user confirmation only
	if oldStatus != models.StatusSuccess && txn.Status == models.StatusSuccess {
		h.logger.Info("Transaction verified as successful",
			"transaction_id", txn.ID,
			"reference", txn.Reference,
			"note", "Wallet credit already completed during initial deposit")
	}

	return h.buildVerifyStatusResponse(txn), nil
}

// buildVerifyStatusResponse builds the proto response from transaction
func (h *PaymentHandler) buildVerifyStatusResponse(txn *models.Transaction) *pb.VerifyDepositStatusResponse {
	statusInfo := &pb.TransactionStatusInfo{
		TransactionId: txn.ID.String(),
		Reference:     txn.Reference,
		Status:        h.modelStatusToProto(txn.Status),
	}

	// Add provider status code if available
	if providerCode, ok := txn.ProviderData["provider_status_code"].(string); ok {
		statusInfo.ProviderStatusCode = providerCode
	}

	// Add timestamps
	statusInfo.RequestedAt = timestamppb.New(txn.RequestedAt)
	if txn.CompletedAt != nil {
		statusInfo.CompletedAt = timestamppb.New(*txn.CompletedAt)
	}

	// Add error message if failed
	if txn.Status == models.StatusFailed && txn.ErrorMessage != nil && *txn.ErrorMessage != "" {
		statusInfo.ErrorMessage = *txn.ErrorMessage
	}

	return &pb.VerifyDepositStatusResponse{
		Success:     true,
		Message:     fmt.Sprintf("Transaction status: %s", txn.Status),
		StatusInfo:  statusInfo,
		Transaction: h.transactionToProto(txn),
	}
}

// mapProviderStatus maps provider status to model status
func (h *PaymentHandler) mapProviderStatus(providerStatus providers.TransactionStatus) models.TransactionStatus {
	switch providerStatus {
	case providers.StatusSuccess:
		return models.StatusSuccess
	case providers.StatusFailed:
		return models.StatusFailed
	case providers.StatusPending:
		return models.StatusPending
	case providers.StatusDuplicate:
		return models.StatusDuplicate
	default:
		return models.StatusPending
	}
}

// modelStatusToProto converts model status to proto status
func (h *PaymentHandler) modelStatusToProto(modelStatus models.TransactionStatus) pb.TransactionStatus {
	switch modelStatus {
	case models.StatusPending:
		return pb.TransactionStatus_TRANSACTION_STATUS_PENDING
	case models.StatusProcessing:
		return pb.TransactionStatus_TRANSACTION_STATUS_PROCESSING
	case models.StatusSuccess:
		return pb.TransactionStatus_TRANSACTION_STATUS_SUCCESS
	case models.StatusFailed:
		return pb.TransactionStatus_TRANSACTION_STATUS_FAILED
	case models.StatusVerifying:
		return pb.TransactionStatus_TRANSACTION_STATUS_VERIFYING
	case models.StatusDuplicate:
		return pb.TransactionStatus_TRANSACTION_STATUS_DUPLICATE
	default:
		return pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
}

// GetWithdrawalStatus retrieves withdrawal status
func (h *PaymentHandler) GetWithdrawalStatus(ctx context.Context, req *pb.GetWithdrawalStatusRequest) (*pb.GetWithdrawalStatusResponse, error) {
	// Delegate to GetTransaction
	txnResp, err := h.GetTransaction(ctx, &pb.GetTransactionRequest{Reference: req.Reference})
	if err != nil {
		return nil, err
	}

	return &pb.GetWithdrawalStatusResponse{
		Success:     txnResp.Success,
		Message:     txnResp.Message,
		Transaction: txnResp.Transaction,
	}, nil
}

// ListTransactions lists transactions with filtering
func (h *PaymentHandler) ListTransactions(ctx context.Context, req *pb.ListTransactionsRequest) (*pb.ListTransactionsResponse, error) {
	ctx, span := tracer.Start(ctx, "payment_handler.list_transactions",
		trace.WithAttributes(
			attribute.String("user_id", req.UserId),
			attribute.String("type", req.Type.String()),
			attribute.String("status", req.Status.String()),
		))
	defer span.End()

	// Build filter
	filter := &models.TransactionFilter{}

	// Set page and page_size
	page := int(req.Page)
	pageSize := int(req.PageSize)

	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Calculate limit and offset
	filter.Limit = pageSize
	filter.Offset = (page - 1) * pageSize

	// Apply user filter
	if req.UserId != "" {
		userID := uuid.MustParse(req.UserId)
		filter.UserID = &userID
	}

	// Apply type filter
	if req.Type != pb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED {
		typeStr := h.transactionTypeFromProto(req.Type)
		filter.Type = &typeStr
	}

	// Apply status filter
	if req.Status != pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED {
		statusStr := h.transactionStatusFromProto(req.Status)
		filter.Status = &statusStr
	}

	// Apply date filters
	if req.StartDate != nil {
		startDate := req.StartDate.AsTime()
		filter.StartDate = &startDate
	}
	if req.EndDate != nil {
		endDate := req.EndDate.AsTime()
		filter.EndDate = &endDate
	}

	// List transactions
	txns, total, err := h.transactionRepo.List(ctx, filter)
	if err != nil {
		span.RecordError(err)
		h.logger.Error("Failed to list transactions", "error", err)
		return nil, status.Error(codes.Internal, "failed to list transactions")
	}

	// Convert to proto
	protoTxns := make([]*pb.Transaction, len(txns))
	for i, txn := range txns {
		protoTxns[i] = h.transactionToProto(txn)
	}

	// Calculate pagination
	totalPages := (int(total) + pageSize - 1) / pageSize

	return &pb.ListTransactionsResponse{
		Success:      true,
		Message:      fmt.Sprintf("Retrieved %d transactions", len(txns)),
		Transactions: protoTxns,
		Pagination: &pb.PaginationMetadata{
			CurrentPage: int32(page),
			PageSize:    int32(pageSize),
			TotalItems:  total,
			TotalPages:  int32(totalPages),
		},
	}, nil
}

// CancelTransaction cancels a pending transaction
func (h *PaymentHandler) CancelTransaction(ctx context.Context, req *pb.CancelTransactionRequest) (*pb.CancelTransactionResponse, error) {
	ctx, span := tracer.Start(ctx, "payment_handler.cancel_transaction",
		trace.WithAttributes(
			attribute.String("reference", req.Reference),
			attribute.String("reason", req.Reason),
		))
	defer span.End()

	if req.Reference == "" {
		return nil, status.Error(codes.InvalidArgument, "reference is required")
	}

	// Get transaction
	var txn *models.Transaction
	var err error

	if id, parseErr := uuid.Parse(req.Reference); parseErr == nil {
		txn, err = h.transactionRepo.GetByID(ctx, id)
	} else {
		txn, err = h.transactionRepo.GetByReference(ctx, req.Reference)
	}

	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.NotFound, "transaction not found")
	}

	// Check if transaction can be cancelled
	if txn.Status != models.StatusPending {
		return nil, status.Error(codes.FailedPrecondition, "only pending transactions can be cancelled")
	}

	// Update transaction status
	txn.Status = models.StatusFailed
	errMsg := fmt.Sprintf("Cancelled: %s", req.Reason)
	txn.ErrorMessage = &errMsg

	if err := h.transactionRepo.Update(ctx, txn); err != nil {
		span.RecordError(err)
		h.logger.Error("Failed to cancel transaction", "error", err, "transaction_id", txn.ID)
		return nil, status.Error(codes.Internal, "failed to cancel transaction")
	}

	return &pb.CancelTransactionResponse{
		Success:     true,
		Message:     "Transaction cancelled successfully",
		Transaction: h.transactionToProto(txn),
	}, nil
}

// VerifyWallet verifies a mobile money wallet
func (h *PaymentHandler) VerifyWallet(ctx context.Context, req *pb.VerifyWalletRequest) (*pb.VerifyWalletResponse, error) {
	ctx, span := tracer.Start(ctx, "payment_handler.verify_wallet",
		trace.WithAttributes(
			attribute.String("wallet_number", req.WalletNumber),
			attribute.String("provider", req.WalletProvider.String()),
		))
	defer span.End()

	// Validate request
	if req.WalletNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "wallet_number is required")
	}
	if req.WalletProvider == pb.WalletProvider_WALLET_PROVIDER_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "wallet_provider is required")
	}

	// Get provider
	providerName := h.walletProviderToString(req.WalletProvider)
	h.logger.Info("Getting payment provider for wallet verification",
		"provider_enum", req.WalletProvider.String(),
		"provider_name", providerName,
		"wallet_number", req.WalletNumber,
		"reference", req.Reference)

	provider, err := h.providerFactory.GetProvider(providerName)
	if err != nil {
		span.RecordError(err)
		h.logger.Error("Failed to get provider", "provider", providerName, "error", err)
		return nil, status.Error(codes.Internal, "provider not available")
	}

	// Call provider verification
	verifyReq := &providers.VerifyWalletRequest{
		WalletNumber:   req.WalletNumber,
		WalletProvider: providerName,
		Reference:      req.Reference,
	}

	h.logger.Info("Calling provider VerifyWallet",
		"provider", providerName,
		"wallet_number", req.WalletNumber,
		"wallet_provider", verifyReq.WalletProvider,
		"reference", req.Reference)

	verifyResp, err := provider.VerifyWallet(ctx, verifyReq)
	if err != nil {
		span.RecordError(err)
		h.logger.Error("Wallet verification failed",
			"error", err,
			"provider", providerName,
			"wallet_number", req.WalletNumber,
			"reference", req.Reference)
		return nil, status.Error(codes.Internal, fmt.Sprintf("verification failed: %v", err))
	}

	h.logger.Info("Provider VerifyWallet response received",
		"provider", providerName,
		"is_valid", verifyResp.IsValid,
		"account_name", verifyResp.AccountName,
		"wallet_number", verifyResp.WalletNumber,
		"wallet_provider", verifyResp.WalletProvider,
		"reference", verifyResp.Reference,
		"provider_data", verifyResp.ProviderData)

	span.SetAttributes(
		attribute.Bool("is_valid", verifyResp.IsValid),
		attribute.String("account_name", verifyResp.AccountName),
	)

	return &pb.VerifyWalletResponse{
		Success: true,
		Message: "Wallet verified successfully",
		Verification: &pb.WalletVerification{
			IsValid:        verifyResp.IsValid,
			AccountName:    verifyResp.AccountName,
			WalletNumber:   verifyResp.WalletNumber,
			WalletProvider: verifyResp.WalletProvider,
			Reference:      verifyResp.Reference,
		},
	}, nil
}

// VerifyBankAccount verifies a bank account
func (h *PaymentHandler) VerifyBankAccount(ctx context.Context, req *pb.VerifyBankAccountRequest) (*pb.VerifyBankAccountResponse, error) {
	ctx, span := tracer.Start(ctx, "payment_handler.verify_bank_account",
		trace.WithAttributes(
			attribute.String("account_number", req.AccountNumber),
			attribute.String("bank_code", req.BankCode),
		))
	defer span.End()

	// Validate request
	if req.AccountNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "account_number is required")
	}
	if req.BankCode == "" {
		return nil, status.Error(codes.InvalidArgument, "bank_code is required")
	}

	// Use Orange provider for bank verification (aggregator)
	provider, err := h.providerFactory.GetProvider("orange")
	if err != nil {
		span.RecordError(err)
		h.logger.Error("Failed to get provider for bank verification", "error", err)
		return nil, status.Error(codes.Internal, "provider not available")
	}

	// Call provider verification
	verifyReq := &providers.VerifyBankAccountRequest{
		AccountNumber: req.AccountNumber,
		BankCode:      req.BankCode,
		Reference:     req.Reference,
	}

	verifyResp, err := provider.VerifyBankAccount(ctx, verifyReq)
	if err != nil {
		span.RecordError(err)
		h.logger.Error("Bank account verification failed", "error", err, "reference", req.Reference)
		return nil, status.Error(codes.Internal, fmt.Sprintf("verification failed: %v", err))
	}

	span.SetAttributes(
		attribute.Bool("is_valid", verifyResp.IsValid),
		attribute.String("account_name", verifyResp.AccountName),
	)

	return &pb.VerifyBankAccountResponse{
		Success: true,
		Message: "Bank account verified successfully",
		Verification: &pb.BankAccountVerification{
			IsValid:       verifyResp.IsValid,
			AccountName:   verifyResp.AccountName,
			AccountNumber: verifyResp.AccountNumber,
			BankCode:      verifyResp.BankCode,
			BankName:      verifyResp.BankName,
			Reference:     verifyResp.Reference,
		},
	}, nil
}

// VerifyIdentity verifies identity documents (Ghana Card, Passport, etc.)
func (h *PaymentHandler) VerifyIdentity(ctx context.Context, req *pb.VerifyIdentityRequest) (*pb.VerifyIdentityResponse, error) {
	ctx, span := tracer.Start(ctx, "payment_handler.verify_identity",
		trace.WithAttributes(
			attribute.String("identity_type", req.IdentityType.String()),
			attribute.String("identity_number", req.IdentityNumber),
		))
	defer span.End()

	// Validate request
	if req.IdentityType == pb.IdentityType_IDENTITY_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "identity_type is required")
	}
	if req.IdentityNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "identity_number is required")
	}

	// Use Orange provider for identity verification (aggregator)
	provider, err := h.providerFactory.GetProvider("orange")
	if err != nil {
		span.RecordError(err)
		h.logger.Error("Failed to get provider for identity verification", "error", err)
		return nil, status.Error(codes.Internal, "provider not available")
	}

	// Convert identity type
	identityType := h.identityTypeToString(req.IdentityType)

	// Call provider verification
	verifyReq := &providers.VerifyIdentityRequest{
		IdentityType:   identityType,
		IdentityNumber: req.IdentityNumber,
		FullKYC:        req.FullKyc,
		Reference:      req.Reference,
	}

	verifyResp, err := provider.VerifyIdentity(ctx, verifyReq)
	if err != nil {
		span.RecordError(err)
		h.logger.Error("Identity verification failed", "error", err, "reference", req.Reference)
		return nil, status.Error(codes.Internal, fmt.Sprintf("verification failed: %v", err))
	}

	span.SetAttributes(
		attribute.Bool("is_valid", verifyResp.IsValid),
		attribute.String("full_name", verifyResp.FullName),
	)

	// Build response
	verification := &pb.IdentityVerification{
		IsValid:        verifyResp.IsValid,
		FullName:       verifyResp.FullName,
		Nationality:    verifyResp.Nationality,
		IdentityNumber: verifyResp.IdentityNumber,
		Reference:      verifyResp.Reference,
	}

	// Add timestamps if available
	if verifyResp.DateOfBirth != nil {
		verification.DateOfBirth = timestamppb.New(*verifyResp.DateOfBirth)
	}
	if verifyResp.CardValidFrom != nil {
		verification.CardValidFrom = timestamppb.New(*verifyResp.CardValidFrom)
	}
	if verifyResp.CardValidTo != nil {
		verification.CardValidTo = timestamppb.New(*verifyResp.CardValidTo)
	}

	// Add KYC data if full KYC requested
	if req.FullKyc && verifyResp.ProviderData != nil {
		verification.KycData = make(map[string]string)
		for k, v := range verifyResp.ProviderData {
			verification.KycData[k] = fmt.Sprintf("%v", v)
		}
	}

	return &pb.VerifyIdentityResponse{
		Success:      true,
		Message:      "Identity verified successfully",
		Verification: verification,
	}, nil
}

// ListProviders lists available payment providers
func (h *PaymentHandler) ListProviders(ctx context.Context, req *pb.ListProvidersRequest) (*pb.ListProvidersResponse, error) {
	ctx, span := tracer.Start(ctx, "payment_handler.list_providers")
	defer span.End()

	providerNames := h.providerFactory.ListProviders()
	providers := make([]*pb.Provider, 0, len(providerNames))

	for _, name := range providerNames {
		provider, err := h.providerFactory.GetProvider(name)
		if err != nil {
			h.logger.Error("Failed to get provider", "provider", name, "error", err)
			continue
		}

		// Skip disabled providers if requested
		if req.EnabledOnly {
			// Check health
			if err := provider.HealthCheck(ctx); err != nil {
				continue
			}
		}

		// Get provider metadata
		operations := provider.GetSupportedOperations()
		operationStrs := make([]string, len(operations))
		for i, op := range operations {
			operationStrs[i] = string(op)
		}

		currencies := provider.GetSupportedCurrencies()
		limits := provider.GetTransactionLimits()

		isHealthy := provider.HealthCheck(ctx) == nil

		pbProvider := &pb.Provider{
			Name:                provider.GetProviderName(),
			Type:                string(provider.GetProviderType()),
			IsEnabled:           true, // TODO: Add enable/disable logic
			IsHealthy:           isHealthy,
			SupportedOperations: operationStrs,
			SupportedCurrencies: currencies,
		}

		if limits != nil {
			pbProvider.Limits = &pb.TransactionLimits{
				MinAmount:    int64(limits.MinAmount),
				MaxAmount:    int64(limits.MaxAmount),
				DailyLimit:   int64(limits.DailyLimit),
				MonthlyLimit: int64(limits.MonthlyLimit),
				Currency:     limits.Currency,
			}
		}

		providers = append(providers, pbProvider)
	}

	span.SetAttributes(attribute.Int("provider_count", len(providers)))

	return &pb.ListProvidersResponse{
		Success:   true,
		Message:   fmt.Sprintf("Retrieved %d providers", len(providers)),
		Providers: providers,
	}, nil
}

// GetProviderHealth checks provider health status
func (h *PaymentHandler) GetProviderHealth(ctx context.Context, req *pb.GetProviderHealthRequest) (*pb.GetProviderHealthResponse, error) {
	ctx, span := tracer.Start(ctx, "payment_handler.get_provider_health",
		trace.WithAttributes(
			attribute.String("provider_name", req.ProviderName),
		))
	defer span.End()

	healthChecks := make([]*pb.ProviderHealth, 0)

	// Check specific provider or all providers
	providerNames := []string{}
	if req.ProviderName != "" {
		providerNames = append(providerNames, req.ProviderName)
	} else {
		providerNames = h.providerFactory.ListProviders()
	}

	for _, name := range providerNames {
		provider, err := h.providerFactory.GetProvider(name)
		if err != nil {
			h.logger.Error("Failed to get provider", "provider", name, "error", err)
			continue
		}

		// Perform health check
		startTime := time.Now()
		healthErr := provider.HealthCheck(ctx)
		responseTime := time.Since(startTime).Milliseconds()

		healthCheck := &pb.ProviderHealth{
			ProviderName:   name,
			IsHealthy:      healthErr == nil,
			CircuitState:   "CLOSED", // TODO: Integrate circuit breaker
			FailureCount:   0,        // TODO: Track failures
			ResponseTimeMs: int32(responseTime),
			LastChecked:    timestamppb.Now(),
		}

		if healthErr != nil {
			healthCheck.ErrorMessage = healthErr.Error()
		}

		healthChecks = append(healthChecks, healthCheck)
	}

	span.SetAttributes(attribute.Int("health_checks", len(healthChecks)))

	return &pb.GetProviderHealthResponse{
		Success:      true,
		Message:      fmt.Sprintf("Retrieved health for %d providers", len(healthChecks)),
		HealthChecks: healthChecks,
	}, nil
}

// ProcessWebhook handles provider webhook callbacks
func (h *PaymentHandler) ProcessWebhook(ctx context.Context, req *pb.ProcessWebhookRequest) (*pb.ProcessWebhookResponse, error) {
	ctx, span := tracer.Start(ctx, "payment_handler.process_webhook",
		trace.WithAttributes(
			attribute.String("provider_name", req.ProviderName),
			attribute.String("event_type", req.EventType),
		))
	defer span.End()

	// Validate request
	if req.ProviderName == "" {
		return nil, status.Error(codes.InvalidArgument, "provider_name is required")
	}
	if req.EventType == "" {
		return nil, status.Error(codes.InvalidArgument, "event_type is required")
	}

	// Get signature from headers
	signature := ""
	if req.Headers != nil {
		signature = req.Headers["X-Orange-Signature"]
	}

	h.logger.Info("Processing webhook",
		"provider", req.ProviderName,
		"event_type", req.EventType,
		"payload_size", len(req.Payload),
		"has_signature", signature != "")

	// Step 1: Verify webhook signature
	signatureValid := h.verifyWebhookSignature(req.ProviderName, string(req.Payload), signature)
	if !signatureValid {
		h.logger.Error("Webhook signature verification failed",
			"provider", req.ProviderName,
			"event_type", req.EventType)
		span.SetAttributes(attribute.Bool("signature_valid", false))
		return nil, status.Error(codes.Unauthenticated, "invalid webhook signature")
	}

	span.SetAttributes(attribute.Bool("signature_valid", true))

	// Step 2: Parse payload based on provider
	var orangePayload models.OrangeWebhookPayload
	if err := json.Unmarshal(req.Payload, &orangePayload); err != nil {
		h.logger.Error("Failed to parse webhook payload",
			"error", err,
			"provider", req.ProviderName)
		span.RecordError(err)
		return nil, status.Error(codes.InvalidArgument, "invalid webhook payload format")
	}

	h.logger.Info("Webhook payload parsed",
		"provider", req.ProviderName,
		"status", orangePayload.Status,
		"message", orangePayload.Message,
		"transaction_id", orangePayload.Data.TransactionID,
		"reference", orangePayload.Data.Reference,
		"amount", orangePayload.Data.Amount)

	// Validate payload has required fields
	if orangePayload.Data.Reference == "" {
		return nil, status.Error(codes.InvalidArgument, "webhook payload missing transaction reference")
	}
	if orangePayload.Data.TransactionID == "" {
		return nil, status.Error(codes.InvalidArgument, "webhook payload missing provider transaction ID")
	}

	// Step 3: Check for duplicate webhook (deduplication)
	existingWebhook, err := h.webhookEventRepo.GetByProviderTransactionID(ctx, req.ProviderName, orangePayload.Data.TransactionID)
	if err == nil && existingWebhook != nil {
		h.logger.Info("Duplicate webhook detected - already processed",
			"provider", req.ProviderName,
			"provider_transaction_id", orangePayload.Data.TransactionID,
			"reference", orangePayload.Data.Reference,
			"previous_status", existingWebhook.Status)
		span.SetAttributes(attribute.Bool("duplicate", true))

		return &pb.ProcessWebhookResponse{
			Success: true,
			Message: fmt.Sprintf("Duplicate webhook - already processed with status: %s", existingWebhook.Status),
		}, nil
	}

	// Step 4: Create webhook event record
	// Parse raw payload into map for storage
	var rawPayloadMap map[string]interface{}
	if err := json.Unmarshal(req.Payload, &rawPayloadMap); err != nil {
		h.logger.Error("Failed to parse raw payload for webhook event",
			"error", err,
			"provider", req.ProviderName)
		rawPayloadMap = make(map[string]interface{})
	}

	webhookEvent := &models.WebhookEvent{
		Provider:          req.ProviderName,
		EventType:         req.EventType,
		TransactionID:     nil, // Will be set after we find our transaction
		Reference:         orangePayload.Data.Reference,
		Status:            "pending",
		RawPayload:        rawPayloadMap,
		SignatureVerified: signatureValid,
		ReceivedAt:        time.Now(),
	}

	h.logger.Debug("Creating webhook event record",
		"provider", req.ProviderName,
		"event_type", req.EventType,
		"reference", orangePayload.Data.Reference,
		"provider_transaction_id", orangePayload.Data.TransactionID,
		"signature_verified", signatureValid)

	if err := h.webhookEventRepo.Create(ctx, webhookEvent); err != nil {
		h.logger.Error("Failed to create webhook event record",
			"error", err,
			"error_type", fmt.Sprintf("%T", err),
			"provider", req.ProviderName,
			"reference", orangePayload.Data.Reference,
			"provider_transaction_id", orangePayload.Data.TransactionID,
			"event_type", req.EventType)
		span.RecordError(err)
		// Continue processing even if webhook event creation fails (non-critical)
	} else {
		h.logger.Info("Webhook event record created successfully",
			"webhook_event_id", webhookEvent.ID,
			"provider", req.ProviderName,
			"reference", orangePayload.Data.Reference,
			"provider_transaction_id", orangePayload.Data.TransactionID)
	}

	// Step 5: Lock transaction row to prevent concurrent updates (SELECT FOR UPDATE)
	txn, err := h.transactionRepo.GetByReferenceForUpdate(ctx, orangePayload.Data.Reference)
	if err != nil {
		h.logger.Error("Transaction not found for webhook",
			"error", err,
			"reference", orangePayload.Data.Reference,
			"provider", req.ProviderName)
		span.RecordError(err)

		// Update webhook event status to failed
		webhookEvent.Status = "failed"
		webhookEvent.ErrorMessage = fmt.Sprintf("Transaction not found: %v", err)
		webhookEvent.ProcessedAt = timePtr(time.Now())
		_ = h.webhookEventRepo.Update(ctx, webhookEvent)

		return nil, status.Error(codes.NotFound, "transaction not found")
	}

	h.logger.Info("Transaction found and locked for webhook",
		"transaction_id", txn.ID,
		"reference", txn.Reference,
		"current_status", txn.Status)

	// Link webhook event to transaction (for foreign key relationship)
	txnID := txn.ID.String()
	webhookEvent.TransactionID = &txnID

	// Step 6: Check if transaction is already in a terminal state (idempotency)
	if txn.Status == models.StatusSuccess || txn.Status == models.StatusFailed {
		h.logger.Info("Transaction already in terminal state, returning idempotent response",
			"transaction_id", txn.ID,
			"reference", txn.Reference,
			"status", txn.Status)

		// Update webhook event status to completed (idempotent)
		webhookEvent.Status = "completed_idempotent"
		webhookEvent.ProcessedAt = timePtr(time.Now())
		_ = h.webhookEventRepo.Update(ctx, webhookEvent)

		return &pb.ProcessWebhookResponse{
			Success: true,
			Message: fmt.Sprintf("Transaction already in terminal state: %s", txn.Status),
		}, nil
	}

	// Step 7: Map Orange status to our status
	oldStatus := txn.Status
	newStatus := h.mapOrangeStatusCodeToModelStatus(orangePayload.Status)

	h.logger.Info("Mapping webhook status",
		"orange_status_code", orangePayload.Status,
		"orange_message", orangePayload.Message,
		"old_status", oldStatus,
		"new_status", newStatus)

	// Update transaction with new status
	txn.Status = newStatus
	txn.ProviderTransactionID = &orangePayload.Data.TransactionID
	txn.UpdatedAt = time.Now()

	// Update completed timestamp if successful
	if newStatus == models.StatusSuccess && orangePayload.Data.ApproveDate != nil {
		txn.CompletedAt = orangePayload.Data.ApproveDate
	}

	// Store webhook data in provider_data
	if txn.ProviderData == nil {
		txn.ProviderData = make(map[string]interface{})
	}
	txn.ProviderData["webhook_received"] = time.Now()
	txn.ProviderData["webhook_status_code"] = orangePayload.Status
	txn.ProviderData["webhook_message"] = orangePayload.Message
	txn.ProviderData["orange_approve_date"] = orangePayload.Data.ApproveDate

	// Update transaction in database
	if err := h.transactionRepo.Update(ctx, txn); err != nil {
		h.logger.Error("Failed to update transaction from webhook",
			"error", err,
			"transaction_id", txn.ID,
			"reference", txn.Reference)
		span.RecordError(err)

		// Update webhook event status to failed
		webhookEvent.Status = "failed"
		webhookEvent.ErrorMessage = fmt.Sprintf("Failed to update transaction: %v", err)
		webhookEvent.ProcessedAt = timePtr(time.Now())
		_ = h.webhookEventRepo.Update(ctx, webhookEvent)

		return nil, status.Error(codes.Internal, "failed to update transaction")
	}

	span.SetAttributes(
		attribute.String("transaction_id", txn.ID.String()),
		attribute.String("old_status", string(oldStatus)),
		attribute.String("new_status", string(newStatus)),
	)

	h.logger.Info("Transaction updated from webhook",
		"transaction_id", txn.ID,
		"reference", txn.Reference,
		"old_status", oldStatus,
		"new_status", newStatus)

	// Step 8: Execute saga SYNCHRONOUSLY based on status (no race condition)
	var sagaErr error
	switch newStatus {
	case models.StatusSuccess:
		// Resume saga to credit wallet (if deposit)
		h.logger.Info("Webhook confirmed SUCCESS - executing saga to credit wallet",
			"transaction_id", txn.ID,
			"reference", txn.Reference,
			"type", txn.Type)

		switch txn.Type {
		case models.TypeDeposit:
			// Determine user type from metadata
			userRole := ""
			if txn.Metadata != nil {
				userRole = txn.Metadata["user_role"]
			}

			// Execute saga synchronously - wallet service has idempotency built-in
			if userRole == "player" {
				sagaErr = h.playerDepositSaga.Execute(ctx, txn)
			} else {
				sagaErr = h.depositSaga.Execute(ctx, txn)
			}

			// Only treat as failure if it's not a pending confirmation error
			if sagaErr != nil && !isSagaPendingError(sagaErr) {
				h.logger.Error("Failed to execute deposit saga after webhook",
					"error", sagaErr,
					"transaction_id", txn.ID,
					"reference", txn.Reference)

				// Mark transaction as failed
				txn.Status = models.StatusFailed
				errMsg := fmt.Sprintf("Wallet credit failed: %v", sagaErr)
				txn.ErrorMessage = &errMsg
				txn.UpdatedAt = time.Now()
				_ = h.transactionRepo.Update(ctx, txn)

				// Update webhook event status to failed
				webhookEvent.Status = "failed"
				webhookEvent.ErrorMessage = fmt.Sprintf("Saga execution failed: %v", sagaErr)
				webhookEvent.ProcessedAt = timePtr(time.Now())
				_ = h.webhookEventRepo.Update(ctx, webhookEvent)

				return nil, status.Error(codes.Internal, fmt.Sprintf("failed to credit wallet: %v", sagaErr))
			}

			h.logger.Info("Deposit saga executed successfully after webhook",
				"transaction_id", txn.ID,
				"reference", txn.Reference)
		case models.TypeWithdrawal:
			// For withdrawals, saga already ran - just log confirmation
			h.logger.Info("Withdrawal confirmed by webhook",
				"transaction_id", txn.ID,
				"reference", txn.Reference)
		}
	case models.StatusFailed:
		// Payment failed - no saga needed since wallet wasn't credited
		h.logger.Info("Webhook confirmed FAILED - no saga execution needed",
			"transaction_id", txn.ID,
			"reference", txn.Reference,
			"message", orangePayload.Message)
	default:
		// Still pending - just log
		h.logger.Info("Webhook status still PENDING - no saga execution yet",
			"transaction_id", txn.ID,
			"reference", txn.Reference)
	}

	// Update webhook event status to completed
	webhookEvent.Status = "completed"
	webhookEvent.ProcessedAt = timePtr(time.Now())
	_ = h.webhookEventRepo.Update(ctx, webhookEvent)

	return &pb.ProcessWebhookResponse{
		Success: true,
		Message: fmt.Sprintf("Webhook processed successfully - transaction status: %s", newStatus),
	}, nil
}

// verifyWebhookSignature verifies the webhook signature from provider using HMAC-SHA256
func (h *PaymentHandler) verifyWebhookSignature(providerName, payload, signature string) bool {
	// Check if signature is provided
	if signature == "" {
		// Get environment from env var
		environment := os.Getenv("TRACING_ENVIRONMENT")
		if environment == "" {
			environment = "development" // Default to development if not set
		}

		// Reject unsigned webhooks in production
		if environment == "production" {
			h.logger.Error("Webhook received without signature in production - rejecting",
				"provider", providerName)
			return false
		}

		// Allow for development/testing environments
		h.logger.Warn("Webhook received without signature - allowing for development",
			"provider", providerName,
			"environment", environment)
		return true
	}

	// Get the callback secret for this provider
	var callbackSecret string
	switch providerName {
	case "orange":
		callbackSecret = h.orangeCallbackSecret
	default:
		h.logger.Error("Unknown provider for signature verification",
			"provider", providerName)
		return false
	}

	// Verify HMAC-SHA256 signature
	expectedSignature := h.computeHMAC256(payload, callbackSecret)

	// Compare signatures (constant time comparison to prevent timing attacks)
	isValid := signature == expectedSignature

	h.logger.Info("Webhook signature verification",
		"provider", providerName,
		"signature_valid", isValid,
		"signature_length", len(signature))

	return isValid
}

// computeHMAC256 computes HMAC-SHA256 signature of payload using secret
func (h *PaymentHandler) computeHMAC256(payload, secret string) string {
	// Import crypto/hmac and crypto/sha256 at the top
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))
	return signature
}

// isSagaPendingError checks if the saga error is a "pending confirmation" error
// which should not be treated as a failure (operation is still in progress)
// TODO: Replace with typed error checking (errors.Is) when saga errors are refactored to use typed errors
func isSagaPendingError(err error) bool {
	if err == nil {
		return false
	}
	// Check for specific error message that indicates pending status
	// This is intentionally checking for "PENDING_CONFIRMATION" in the error message
	return strings.Contains(err.Error(), "PENDING_CONFIRMATION")
}

// timePtr returns a pointer to the given time (helper function)
func timePtr(t time.Time) *time.Time {
	return &t
}

// mapOrangeStatusCodeToModelStatus maps Orange numeric status code to model status
func (h *PaymentHandler) mapOrangeStatusCodeToModelStatus(orangeStatusCode int) models.TransactionStatus {
	switch orangeStatusCode {
	case 1: // SUCCESS
		return models.StatusSuccess
	case 0: // FAILED
		return models.StatusFailed
	case 3: // PENDING
		return models.StatusPending
	case 2: // DUPLICATE
		return models.StatusDuplicate
	default:
		return models.StatusPending
	}
}

// Stub implementations for bank transfers (to be implemented)

func (h *PaymentHandler) InitiateBankTransfer(ctx context.Context, req *pb.InitiateBankTransferRequest) (*pb.InitiateBankTransferResponse, error) {
	return nil, status.Error(codes.Unimplemented, "bank transfers not yet implemented")
}

func (h *PaymentHandler) GetBankTransferStatus(ctx context.Context, req *pb.GetBankTransferStatusRequest) (*pb.GetBankTransferStatusResponse, error) {
	return nil, status.Error(codes.Unimplemented, "bank transfers not yet implemented")
}

// Helper methods

func (h *PaymentHandler) validateDepositRequest(req *pb.InitiateDepositRequest) error {
	if req.UserId == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.WalletNumber == "" {
		return fmt.Errorf("wallet_number is required")
	}
	if req.WalletProvider == pb.WalletProvider_WALLET_PROVIDER_UNSPECIFIED {
		return fmt.Errorf("wallet_provider is required")
	}
	if req.Amount <= 0 {
		return fmt.Errorf("amount must be greater than 0")
	}
	if req.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if req.Reference == "" {
		return fmt.Errorf("reference is required")
	}
	if req.Narration == "" {
		return fmt.Errorf("narration is required")
	}
	return nil
}

func (h *PaymentHandler) validateWithdrawalRequest(req *pb.InitiateWithdrawalRequest) error {
	if req.UserId == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.WalletNumber == "" {
		return fmt.Errorf("wallet_number is required")
	}
	if req.WalletProvider == pb.WalletProvider_WALLET_PROVIDER_UNSPECIFIED {
		return fmt.Errorf("wallet_provider is required")
	}
	if req.Amount <= 0 {
		return fmt.Errorf("amount must be greater than 0")
	}
	if req.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if req.Reference == "" {
		return fmt.Errorf("reference is required")
	}
	return nil
}

// walletProviderToString converts WalletProvider enum to string
// Returns UPPERCASE provider names as required by Orange Extensibility API
func (h *PaymentHandler) walletProviderToString(provider pb.WalletProvider) string {
	switch provider {
	case pb.WalletProvider_WALLET_PROVIDER_MTN:
		return "MTN" // UPPERCASE for Orange API
	case pb.WalletProvider_WALLET_PROVIDER_TELECEL:
		return "TELECEL" // UPPERCASE for Orange API
	case pb.WalletProvider_WALLET_PROVIDER_AIRTELTIGO:
		return "AIRTELTIGO" // UPPERCASE for Orange API
	default:
		return "UNKNOWN"
	}
}

func (h *PaymentHandler) identityTypeToString(identityType pb.IdentityType) string {
	switch identityType {
	case pb.IdentityType_IDENTITY_TYPE_GHANA_CARD:
		return "GHANA_CARD"
	case pb.IdentityType_IDENTITY_TYPE_PASSPORT:
		return "PASSPORT"
	case pb.IdentityType_IDENTITY_TYPE_DRIVERS_LICENSE:
		return "DRIVERS_LICENSE"
	default:
		return "UNKNOWN"
	}
}

func (h *PaymentHandler) transactionTypeFromProto(txnType pb.TransactionType) models.TransactionType {
	switch txnType {
	case pb.TransactionType_TRANSACTION_TYPE_DEPOSIT:
		return models.TypeDeposit
	case pb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL:
		return models.TypeWithdrawal
	case pb.TransactionType_TRANSACTION_TYPE_BANK_TRANSFER:
		return models.TypeBankTransfer
	default:
		return ""
	}
}

func (h *PaymentHandler) transactionStatusFromProto(txnStatus pb.TransactionStatus) models.TransactionStatus {
	switch txnStatus {
	case pb.TransactionStatus_TRANSACTION_STATUS_PENDING:
		return models.StatusPending
	case pb.TransactionStatus_TRANSACTION_STATUS_PROCESSING:
		return models.StatusProcessing
	case pb.TransactionStatus_TRANSACTION_STATUS_SUCCESS:
		return models.StatusSuccess
	case pb.TransactionStatus_TRANSACTION_STATUS_FAILED:
		return models.StatusFailed
	case pb.TransactionStatus_TRANSACTION_STATUS_VERIFYING:
		return models.StatusVerifying
	case pb.TransactionStatus_TRANSACTION_STATUS_DUPLICATE:
		return models.StatusDuplicate
	default:
		return ""
	}
}

func (h *PaymentHandler) transactionToProto(txn *models.Transaction) *pb.Transaction {
	if txn == nil {
		return nil
	}

	providerTxID := ""
	if txn.ProviderTransactionID != nil {
		providerTxID = *txn.ProviderTransactionID
	}

	errorMsg := ""
	if txn.ErrorMessage != nil {
		errorMsg = *txn.ErrorMessage
	}

	errorCode := ""
	if txn.ErrorCode != nil {
		errorCode = *txn.ErrorCode
	}

	protoTxn := &pb.Transaction{
		Id:                    txn.ID.String(),
		Reference:             txn.Reference,
		ProviderTransactionId: providerTxID,
		Amount:                txn.Amount,
		Currency:              txn.Currency,
		Narration:             txn.Narration,
		ProviderName:          txn.ProviderName,
		SourceType:            txn.SourceType,
		SourceIdentifier:      txn.SourceIdentifier,
		SourceName:            txn.SourceName,
		DestinationType:       txn.DestinationType,
		DestinationIdentifier: txn.DestinationIdentifier,
		DestinationName:       txn.DestinationName,
		UserId:                txn.UserID.String(),
		ErrorMessage:          errorMsg,
		ErrorCode:             errorCode,
		RetryCount:            int32(txn.RetryCount),
		Metadata:              txn.Metadata,
	}

	// Convert type
	switch txn.Type {
	case models.TypeDeposit:
		protoTxn.Type = pb.TransactionType_TRANSACTION_TYPE_DEPOSIT
	case models.TypeWithdrawal:
		protoTxn.Type = pb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL
	case models.TypeBankTransfer:
		protoTxn.Type = pb.TransactionType_TRANSACTION_TYPE_BANK_TRANSFER
	}

	// Convert status
	switch txn.Status {
	case models.StatusPending:
		protoTxn.Status = pb.TransactionStatus_TRANSACTION_STATUS_PENDING
	case models.StatusProcessing:
		protoTxn.Status = pb.TransactionStatus_TRANSACTION_STATUS_PROCESSING
	case models.StatusSuccess:
		protoTxn.Status = pb.TransactionStatus_TRANSACTION_STATUS_SUCCESS
	case models.StatusFailed:
		protoTxn.Status = pb.TransactionStatus_TRANSACTION_STATUS_FAILED
	case models.StatusVerifying:
		protoTxn.Status = pb.TransactionStatus_TRANSACTION_STATUS_VERIFYING
	case models.StatusDuplicate:
		protoTxn.Status = pb.TransactionStatus_TRANSACTION_STATUS_DUPLICATE
	}

	// Convert timestamps
	protoTxn.RequestedAt = timestamppb.New(txn.RequestedAt)
	if txn.CompletedAt != nil {
		protoTxn.CompletedAt = timestamppb.New(*txn.CompletedAt)
	}
	protoTxn.CreatedAt = timestamppb.New(txn.CreatedAt)
	protoTxn.UpdatedAt = timestamppb.New(txn.UpdatedAt)

	return protoTxn
}

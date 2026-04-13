package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	agentpb "github.com/randco/randco-microservices/proto/agent/management/v1"
	"github.com/randco/service-wallet/internal/events"
	"github.com/randco/service-wallet/internal/models"
	"github.com/randco/service-wallet/internal/repositories"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// walletLogger is used for wallet service logging
var walletLogger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// AgentClient defines the interface for interacting with the agent management service
type AgentClient interface {
	GetAgent(ctx context.Context, agentID uuid.UUID) (*agentpb.Agent, error)
	GetAgentCommissionPercentage(ctx context.Context, agentID uuid.UUID) (float64, error)
	GetRetailer(ctx context.Context, retailerID uuid.UUID) (*agentpb.Retailer, error)
}

// WalletService defines the interface for wallet business logic
type WalletService interface {
	// Wallet creation operations
	CreateAgentWallet(ctx context.Context, agentID uuid.UUID, agentCode string, initialCommissionRate float64, createdBy string) error
	CreateRetailerWallets(ctx context.Context, retailerID uuid.UUID, retailerCode string, parentAgentID *uuid.UUID, createdBy string) error

	// Credit operations
	CreditAgentWallet(ctx context.Context, agentID uuid.UUID, amount int64, description string, idempotencyKey string, creditSource models.CreditSource) (*models.WalletTransaction, error)
	CreditRetailerWallet(ctx context.Context, retailerID uuid.UUID, amount int64, walletType models.WalletType, description string, idempotencyKey string, creditSource models.CreditSource) (*models.WalletTransaction, error)

	// Debit operations
	DebitRetailerWallet(ctx context.Context, retailerID uuid.UUID, amount int64, walletType models.WalletType, description string) (*models.WalletTransaction, error)

	// Balance operations
	GetAgentBalance(ctx context.Context, agentID uuid.UUID) (*models.AgentStakeWallet, error)
	GetRetailerBalance(ctx context.Context, retailerID uuid.UUID, walletType models.WalletType) (int64, error)

	// Transfer operations
	TransferAgentToRetailer(ctx context.Context, agentID, retailerID uuid.UUID, amount int64, description string) (*models.WalletTransaction, error)

	// Transaction history
	GetTransactionHistory(ctx context.Context, walletOwnerID uuid.UUID, walletType models.WalletType, limit, offset int) ([]*models.WalletTransaction, int, error)

	// Admin operations
	GetAllTransactions(ctx context.Context, filters repositories.AdminTransactionFilters) ([]*models.WalletTransaction, int, error)
	GetTransactionStatistics(ctx context.Context, filters repositories.AdminTransactionFilters) (*repositories.TransactionStatistics, error)
	ReverseTransaction(ctx context.Context, txID uuid.UUID, reason string, adminID uuid.UUID, adminName, adminEmail string) (*models.WalletTransaction, *models.WalletTransaction, error)
	PlaceHoldOnWallet(ctx context.Context, retailerID uuid.UUID, placedBy uuid.UUID, reason string, expiresAt time.Time) error
	GetHoldOnWallet(ctx context.Context, holdID uuid.UUID) (*models.RetailerWinningWalletHold, error)
	GetHoldByRetailer(ctx context.Context, retailerID uuid.UUID) (*models.RetailerWinningWalletHold, error)
	ReleaseHoldOnWallet(ctx context.Context, holdID uuid.UUID, retailerID uuid.UUID, releasedBy uuid.UUID) error

	// Lock operations for security
	LockWallet(ctx context.Context, walletID uuid.UUID, reason string) error
	UnlockWallet(ctx context.Context, walletID uuid.UUID) error

	// Commission operations
	UpdateAgentCommission(ctx context.Context, agentID uuid.UUID, newRate float64, updatedBy string, reason string) (float64, error)

	// Two-Phase Commit operations for external transactions
	ReserveRetailerWalletFunds(ctx context.Context, retailerID uuid.UUID, walletType models.WalletType, amount int64, reference string, ttlSeconds int32, reason string, idempotencyKey string) (string, error)
	CommitReservedDebit(ctx context.Context, reservationID string, notes string) (uuid.UUID, error)
	ReleaseReservation(ctx context.Context, reservationID string, reason string) error

	CreatePlayerWallet(ctx context.Context, playerID uuid.UUID, playerCode string) (uuid.UUID, error)
	CreditPlayerWallet(ctx context.Context, playerID uuid.UUID, amount int64, description string, idempotencyKey string, creditSource models.CreditSource) (*models.WalletTransaction, error)
	DebitPlayerWallet(ctx context.Context, playerID uuid.UUID, amount int64, description string) (*models.WalletTransaction, error)
	ReservePlayerWalletFunds(ctx context.Context, playerID uuid.UUID, amount int64, reference string, ttlSeconds int32, reason string, idempotencyKey string) (string, error)
	GetPlayerBalance(ctx context.Context, playerID uuid.UUID) (*models.PlayerWallet, error)
}

type walletService struct {
	db                   *sql.DB
	cache                *redis.Client
	walletRepo           repositories.WalletRepository
	extendedWalletRepo   repositories.ExtendedWalletRepository
	transactionRepo      repositories.WalletTransactionRepository
	adminTransactionRepo repositories.AdminTransactionRepository
	idempotencyRepo      repositories.IdempotencyRepository
	reservationRepo      repositories.ReservationRepository
	reversalRepo         repositories.TransactionReversalRepository
	commissionService    CommissionService
	eventPublisher       *events.Publisher
	agentClient          AgentClient
	tracer               trace.Tracer
}

// NewWalletService creates a new instance of WalletService
func NewWalletService(
	db *sql.DB,
	cache *redis.Client,
	walletRepo repositories.WalletRepository,
	extendedWalletRepo repositories.ExtendedWalletRepository,
	transactionRepo repositories.WalletTransactionRepository,
	adminTransactionRepo repositories.AdminTransactionRepository,
	idempotencyRepo repositories.IdempotencyRepository,
	reservationRepo repositories.ReservationRepository,
	reversalRepo repositories.TransactionReversalRepository,
	commissionService CommissionService,
	eventPublisher *events.Publisher,
	agentClient AgentClient,
) WalletService {
	return &walletService{
		db:                   db,
		cache:                cache,
		walletRepo:           walletRepo,
		extendedWalletRepo:   extendedWalletRepo,
		transactionRepo:      transactionRepo,
		adminTransactionRepo: adminTransactionRepo,
		idempotencyRepo:      idempotencyRepo,
		reservationRepo:      reservationRepo,
		reversalRepo:         reversalRepo,
		commissionService:    commissionService,
		eventPublisher:       eventPublisher,
		agentClient:          agentClient,
		tracer:               otel.Tracer("wallet-service"),
	}
}

// CreateAgentWallet creates a new agent stake wallet
func (s *walletService) CreateAgentWallet(ctx context.Context, agentID uuid.UUID, agentCode string, initialCommissionRate float64, createdBy string) error {
	ctx, span := s.tracer.Start(ctx, "CreateAgentWallet")
	defer span.End()

	span.SetAttributes(
		attribute.String("agent.id", agentID.String()),
		attribute.String("agent.code", agentCode),
		attribute.Float64("commission.rate", initialCommissionRate),
	)

	// Check if wallet already exists
	existingWallet, err := s.walletRepo.GetByAgentID(ctx, agentID)
	if err == nil && existingWallet != nil {
		span.SetStatus(codes.Error, "wallet already exists")
		return fmt.Errorf("wallet already exists for agent %s", agentID)
	}

	// Create new wallet
	now := time.Now()
	wallet := &models.AgentStakeWallet{
		ID:                uuid.New(),
		AgentID:           agentID,
		Balance:           0,
		PendingBalance:    0,
		AvailableBalance:  0,
		Currency:          "GHS",
		Status:            "active",
		LastTransactionAt: nil,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.walletRepo.Create(ctx, wallet); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create wallet")
		return fmt.Errorf("failed to create agent wallet: %w", err)
	}

	// Set initial commission rate
	if initialCommissionRate > 0 {
		// Convert percentage to basis points (1% = 100 basis points)
		rateBasisPoints := int32(initialCommissionRate * 100)
		if err := s.commissionService.SetAgentCommissionRate(ctx, agentID, rateBasisPoints, createdBy); err != nil {
			span.RecordError(err)
			// Log error but don't fail wallet creation
			span.AddEvent("commission_rate_error", trace.WithAttributes(
				attribute.String("error", err.Error()),
			))
		}
	}

	span.SetStatus(codes.Ok, "wallet created successfully")
	return nil
}

// CreateRetailerWallets creates both stake and winning wallets for a retailer
func (s *walletService) CreateRetailerWallets(ctx context.Context, retailerID uuid.UUID, retailerCode string, parentAgentID *uuid.UUID, createdBy string) error {
	// Log wallet creation start
	fmt.Printf("[WALLET] Creating retailer wallets - ID: %s, Code: %s, CreatedBy: %s\n",
		retailerID.String(), retailerCode, createdBy)
	if parentAgentID != nil {
		fmt.Printf("[WALLET] Retailer has parent agent: %s\n", parentAgentID.String())
	}

	ctx, span := s.tracer.Start(ctx, "CreateRetailerWallets")
	defer span.End()

	span.SetAttributes(
		attribute.String("retailer.id", retailerID.String()),
		attribute.String("retailer.code", retailerCode),
	)

	if parentAgentID != nil {
		span.SetAttributes(attribute.String("parent_agent.id", parentAgentID.String()))
	}

	// Create stake wallet
	now := time.Now()
	stakeWallet := &models.RetailerStakeWallet{
		ID:                uuid.New(),
		RetailerID:        retailerID,
		Balance:           0,
		PendingBalance:    0,
		AvailableBalance:  0,
		Currency:          "GHS",
		Status:            "ACTIVE",
		LastTransactionAt: nil,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.walletRepo.CreateRetailerStakeWallet(ctx, stakeWallet); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create stake wallet")
		fmt.Printf("[WALLET] Failed to create stake wallet: %v\n", err)
		return fmt.Errorf("failed to create retailer stake wallet: %w", err)
	}

	fmt.Printf("[WALLET] Stake wallet created - ID: %s, RetailerID: %s\n",
		stakeWallet.ID.String(), retailerID.String())

	// Create winning wallet
	winningWallet := &models.RetailerWinningWallet{
		ID:                uuid.New(),
		RetailerID:        retailerID,
		Balance:           0,
		PendingBalance:    0,
		AvailableBalance:  0,
		Currency:          "GHS",
		Status:            "active",
		LastTransactionAt: nil,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.walletRepo.CreateRetailerWinningWallet(ctx, winningWallet); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create winning wallet")
		fmt.Printf("[WALLET] Failed to create winning wallet: %v\n", err)
		return fmt.Errorf("failed to create retailer winning wallet: %w", err)
	}

	fmt.Printf("[WALLET] Winning wallet created - ID: %s, RetailerID: %s\n",
		winningWallet.ID.String(), retailerID.String())
	fmt.Printf("[WALLET] Both wallets created successfully for retailer %s (%s)\n",
		retailerCode, retailerID.String())

	// TODO: Publish wallet creation event when Publisher is fully implemented
	// if s.eventPublisher != nil {
	// 	event := map[string]any{
	// 		"event_type":        "wallet.retailer.created",
	// 		"retailer_id":       retailerID.String(),
	// 		"retailer_code":     retailerCode,
	// 		"stake_wallet_id":   stakeWallet.ID.String(),
	// 		"winning_wallet_id": winningWallet.ID.String(),
	// 		"created_by":        createdBy,
	// 		"created_at":        now,
	// 	}
	// 	if parentAgentID != nil {
	// 		event["parent_agent_id"] = parentAgentID.String()
	// 	}
	// 	if err := s.eventPublisher.PublishAsync("wallet.events", event); err != nil {
	// 		span.RecordError(err)
	// 		span.AddEvent("event_publish_error", trace.WithAttributes(
	// 			attribute.String("error", err.Error()),
	// 		))
	// 	}
	// }

	span.SetStatus(codes.Ok, "wallets created successfully")
	return nil
}

// CreditAgentWallet credits an agent's wallet with automatic commission calculation
func (s *walletService) CreditAgentWallet(ctx context.Context, agentID uuid.UUID, amount int64, description string, idempotencyKey string, creditSource models.CreditSource) (*models.WalletTransaction, error) {
	ctx, span := s.tracer.Start(ctx, "CreditAgentWallet")
	defer span.End()

	span.SetAttributes(
		attribute.String("agent.id", agentID.String()),
		attribute.Int64("amount.pesewas", amount),
		attribute.String("description", description),
	)

	walletLogger.Info("CreditAgentWallet called",
		"agent_id", agentID.String(),
		"amount_pesewas", amount,
		"amount_ghs", models.PesewasToGHS(amount),
		"description", description,
		"has_idempotency_key", idempotencyKey != "")

	// Check idempotency if key provided (backward compatible)
	if idempotencyKey != "" {
		span.SetAttributes(attribute.String("idempotency.key", idempotencyKey))
		walletLogger.Debug("Checking idempotency key", "idempotency_key", idempotencyKey)

		existingTx, err := s.idempotencyRepo.CheckIdempotencyKey(ctx, idempotencyKey)
		if err != nil {
			span.RecordError(err)
			walletLogger.Error("Failed to check idempotency",
				"error", err.Error(),
				"idempotency_key", idempotencyKey)
			return nil, fmt.Errorf("failed to check idempotency: %w", err)
		}
		if existingTx != nil {
			// Key already used - return existing transaction (idempotent response)
			span.SetAttributes(
				attribute.String("result", "idempotent_response"),
				attribute.String("existing.transaction_id", existingTx.ID.String()),
			)
			span.SetStatus(codes.Ok, "returned existing transaction")
			walletLogger.Info("Idempotent request detected - returning existing transaction",
				"idempotency_key", idempotencyKey,
				"existing_transaction_id", existingTx.ID.String(),
				"existing_amount", existingTx.Amount)
			return existingTx, nil
		}
		walletLogger.Debug("Idempotency key not found - processing new request", "idempotency_key", idempotencyKey)
	}

	// CRITICAL: Create audit transaction with PENDING status FIRST
	// This ensures we have a trace even if the operation fails
	now := time.Now()
	auditTransactionID := fmt.Sprintf("TXN-%s", uuid.New().String())
	auditDescription := fmt.Sprintf("AUDIT: %s", description)

	var idempKeyPtr *string
	if idempotencyKey != "" {
		idempKeyPtr = &idempotencyKey
	}

	auditTransaction := &models.WalletTransaction{
		ID:              uuid.New(),
		TransactionID:   auditTransactionID,
		WalletOwnerID:   agentID,
		WalletType:      models.WalletTypeAgentStake,
		TransactionType: models.TransactionTypeCredit,
		Amount:          amount,
		BalanceBefore:   0,
		BalanceAfter:    0,
		Description:     &auditDescription,
		Status:          models.TransactionStatusPending,
		CreditSource:    creditSource,
		IdempotencyKey:  idempKeyPtr,
		Metadata:        map[string]interface{}{"audit_trace": true, "operation": "credit_attempt"},
		CreatedAt:       now,
	}

	// Save audit transaction (auto-commits immediately)
	if err := s.transactionRepo.CreateTransaction(ctx, auditTransaction); err != nil {
		// Log error but continue - we still want to attempt the operation
		span.RecordError(err)
		span.AddEvent("audit_transaction_creation_failed", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
	} else {
		span.SetAttributes(
			attribute.String("audit.transaction_id", auditTransactionID),
			attribute.String("audit.status", "pending"),
		)
	}

	// Check if wallet exists, create if needed (outside transaction for performance)
	_, err := s.walletRepo.GetByAgentID(ctx, agentID)
	if err == sql.ErrNoRows {
		// Create new wallet
		now := time.Now()
		newWallet := &models.AgentStakeWallet{
			ID:                uuid.New(),
			AgentID:           agentID,
			Balance:           0,
			PendingBalance:    0,
			LastTransactionAt: &now,
			CreatedAt:         now,
			UpdatedAt:         now,
		}

		if err := s.walletRepo.Create(ctx, newWallet); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create wallet")

			// Update audit transaction to FAILED
			if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed); updateErr != nil {
				span.AddEvent("audit_update_failed", trace.WithAttributes(
					attribute.String("error", updateErr.Error()),
				))
			} else {
				span.SetAttributes(attribute.String("audit.status", "failed"))
			}

			return nil, fmt.Errorf("failed to create wallet: %w", err)
		}
	} else if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get wallet")

		// Update audit transaction to FAILED
		if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed); updateErr != nil {
			span.AddEvent("audit_update_failed", trace.WithAttributes(
				attribute.String("error", updateErr.Error()),
			))
		} else {
			span.SetAttributes(attribute.String("audit.status", "failed"))
		}

		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}

	// Calculate commission using the commission service (outside transaction)
	commission, grossAmount, err := s.commissionService.CalculateAgentCommission(ctx, agentID, amount)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to calculate commission")

		// Update audit transaction to FAILED
		if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed); updateErr != nil {
			span.AddEvent("audit_update_failed", trace.WithAttributes(
				attribute.String("error", updateErr.Error()),
			))
		} else {
			span.SetAttributes(attribute.String("audit.status", "failed"))
		}

		return nil, fmt.Errorf("failed to calculate commission: %w", err)
	}

	span.SetAttributes(
		attribute.Int64("commission.pesewas", commission),
		attribute.Int64("gross_amount.pesewas", grossAmount),
	)

	// Fetch agent details to populate owner name and code
	metadata := make(map[string]any)
	if s.agentClient != nil {
		agent, err := s.agentClient.GetAgent(ctx, agentID)
		if err != nil {
			// Log error but don't fail the transaction
			span.RecordError(err)
			span.AddEvent("agent_details_fetch_failed", trace.WithAttributes(
				attribute.String("error", err.Error()),
			))
		} else {
			metadata["owner_name"] = agent.Name
			metadata["owner_code"] = agent.AgentCode
			span.SetAttributes(
				attribute.String("owner.name", agent.Name),
				attribute.String("owner.code", agent.AgentCode),
			)
		}
	}

	// Start database transaction to prevent race conditions
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to begin transaction")

		// Update audit transaction to FAILED
		if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed); updateErr != nil {
			span.AddEvent("audit_update_failed", trace.WithAttributes(
				attribute.String("error", updateErr.Error()),
			))
		} else {
			span.SetAttributes(attribute.String("audit.status", "failed"))
		}

		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			// Update audit transaction to FAILED on any error
			_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)
		}
	}()

	// Get wallet with row-level lock
	wallet, err := s.walletRepo.GetByAgentIDForUpdate(ctx, tx, agentID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to lock wallet")
		return nil, fmt.Errorf("failed to lock wallet: %w", err)
	}

	balance := wallet.Balance

	// Update wallet balance with GROSS amount (base + commission)
	wallet.Balance += grossAmount
	now = time.Now()
	wallet.LastTransactionAt = &now
	wallet.UpdatedAt = now

	if err = s.extendedWalletRepo.UpdateAgentWalletTx(ctx, tx, wallet); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update wallet")
		return nil, fmt.Errorf("failed to update wallet: %w", err)
	}

	// Create TWO transactions: base amount + commission
	baseTransactionID := fmt.Sprintf("TXN-%s", uuid.New().String())
	commissionTransactionID := fmt.Sprintf("COMM-%s", uuid.New().String())

	// Transaction 1: Base amount (Top-Up)
	baseDescription := fmt.Sprintf("Top-Up via %s (Base Amount)", description)
	baseMetadata := make(map[string]any)
	for k, v := range metadata {
		baseMetadata[k] = v
	}
	baseMetadata["commission_transaction_id"] = commissionTransactionID
	baseMetadata["has_commission"] = true

	baseTransaction := &models.WalletTransaction{
		ID:              uuid.New(),
		TransactionID:   baseTransactionID,
		WalletOwnerID:   agentID,
		WalletType:      models.WalletTypeAgentStake,
		TransactionType: models.TransactionTypeCredit,
		Amount:          amount, // Base amount (e.g., GHS100 = 10,000 pesewas)
		BalanceBefore:   balance,
		BalanceAfter:    balance + amount,
		Description:     &baseDescription,
		Reference:       nil,
		Status:          models.TransactionStatusCompleted,
		CreditSource:    creditSource,
		IdempotencyKey:  idempKeyPtr,
		Metadata:        baseMetadata,
		CreatedAt:       time.Now(),
	}

	// Save base transaction
	if err = s.transactionRepo.CreateTransactionTx(ctx, tx, baseTransaction); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create base transaction")
		return nil, fmt.Errorf("failed to create base transaction: %w", err)
	}

	// Transaction 2: Commission
	commissionDescription := fmt.Sprintf("Commission (30%%) on Top-Up")
	commissionMetadata := make(map[string]any)
	for k, v := range metadata {
		commissionMetadata[k] = v
	}
	commissionMetadata["base_transaction_id"] = baseTransactionID
	commissionMetadata["commission_rate"] = 30

	commissionTransaction := &models.WalletTransaction{
		ID:              uuid.New(),
		TransactionID:   commissionTransactionID,
		WalletOwnerID:   agentID,
		WalletType:      models.WalletTypeAgentStake,
		TransactionType: models.TransactionTypeCommission,
		Amount:          commission, // Commission amount (e.g., GHS42.85 = 4,285 pesewas)
		BalanceBefore:   balance + amount,
		BalanceAfter:    balance + grossAmount, // Final balance after both transactions
		Description:     &commissionDescription,
		Reference:       nil,
		Status:          models.TransactionStatusCompleted,
		CreditSource:    creditSource,
		IdempotencyKey:  nil, // Only base transaction gets idempotency key
		Metadata:        commissionMetadata,
		CreatedAt:       time.Now(),
	}

	// Save commission transaction
	if err = s.transactionRepo.CreateTransactionTx(ctx, tx, commissionTransaction); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create commission transaction")
		return nil, fmt.Errorf("failed to create commission transaction: %w", err)
	}

	// Use base transaction as the primary transaction to return
	transaction := baseTransaction

	// CRITICAL: Save idempotency key BEFORE committing to ensure atomicity
	// This acts as a distributed lock - only one request can insert the key
	// If commit fails, idempotency key won't be saved (transaction rolls back)
	// If key already exists (race condition), we rollback and return existing transaction
	if idempotencyKey != "" {
		idempRecord := &models.IdempotencyKey{
			IdempotencyKey: idempotencyKey,
			TransactionID:  transaction.ID,
			WalletOwnerID:  agentID,
			WalletType:     string(models.WalletTypeAgentStake),
			OperationType:  "credit",
			Amount:         amount,
			CreatedAt:      time.Now(),
		}

		inserted, err := s.idempotencyRepo.SaveIdempotencyKeyTx(ctx, tx, idempRecord)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to save idempotency key")
			return nil, fmt.Errorf("failed to save idempotency key: %w", err)
		}

		// RACE CONDITION DETECTED: Key already existed (concurrent request won)
		if !inserted {
			_ = tx.Rollback()
			span.SetAttributes(
				attribute.String("result", "duplicate_request_detected"),
				attribute.String("idempotency.key", idempotencyKey),
			)

			// Fetch and return the existing transaction
			existingTx, fetchErr := s.idempotencyRepo.CheckIdempotencyKey(ctx, idempotencyKey)
			if fetchErr != nil {
				span.RecordError(fetchErr)

				// Update audit transaction to FAILED
				_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

				return nil, fmt.Errorf("duplicate request detected but failed to fetch existing transaction: %w", fetchErr)
			}
			if existingTx == nil {
				// This shouldn't happen - key exists but transaction not found
				// Update audit transaction to FAILED
				_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

				return nil, fmt.Errorf("duplicate request detected but existing transaction not found")
			}

			span.SetStatus(codes.Ok, "returned existing transaction (duplicate request)")
			return existingTx, nil
		}
	}

	// Commit the transaction (includes wallet + idempotency key atomically)
	if err = tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Record commission transaction
	if commission > 0 {
		if err := s.commissionService.RecordCommissionTransaction(ctx, agentID, amount, grossAmount, commission, 3000, models.CommissionTypeDeposit, commissionTransaction.ID); err != nil {
			// Log error but don't fail the operation
			span.RecordError(err)
		}
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("wallet:agent:%s:balance", agentID.String())
	s.cache.Del(ctx, cacheKey)

	// Update audit transaction to COMPLETED
	if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusCompleted); updateErr != nil {
		// Log error but don't fail - the actual transaction succeeded
		span.AddEvent("audit_update_failed", trace.WithAttributes(
			attribute.String("error", updateErr.Error()),
		))
	} else {
		span.SetAttributes(attribute.String("audit.status", "completed"))
	}

	span.SetStatus(codes.Ok, "wallet credited successfully")
	return transaction, nil
}

// CreditRetailerWallet credits a retailer's wallet
func (s *walletService) CreditRetailerWallet(ctx context.Context, retailerID uuid.UUID, amount int64, walletType models.WalletType, description string, idempotencyKey string, creditSource models.CreditSource) (*models.WalletTransaction, error) {
	ctx, span := s.tracer.Start(ctx, "CreditRetailerWallet")
	defer span.End()

	span.SetAttributes(
		attribute.String("retailer.id", retailerID.String()),
		attribute.String("wallet.type", string(walletType)),
		attribute.Int64("amount.pesewas", amount),
		attribute.String("description", description),
	)

	walletLogger.Info("CreditRetailerWallet called",
		"retailer_id", retailerID.String(),
		"wallet_type", string(walletType),
		"amount_pesewas", amount,
		"amount_ghs", models.PesewasToGHS(amount),
		"description", description,
		"has_idempotency_key", idempotencyKey != "")

	// Check idempotency if key provided (backward compatible)
	if idempotencyKey != "" {
		span.SetAttributes(attribute.String("idempotency.key", idempotencyKey))
		walletLogger.Debug("Checking idempotency key", "idempotency_key", idempotencyKey)

		existingTx, err := s.idempotencyRepo.CheckIdempotencyKey(ctx, idempotencyKey)
		if err != nil {
			span.RecordError(err)
			walletLogger.Error("Failed to check idempotency",
				"error", err.Error(),
				"idempotency_key", idempotencyKey)
			return nil, fmt.Errorf("failed to check idempotency: %w", err)
		}
		if existingTx != nil {
			// Key already used - return existing transaction (idempotent response)
			span.SetAttributes(
				attribute.String("result", "idempotent_response"),
				attribute.String("existing.transaction_id", existingTx.ID.String()),
			)
			span.SetStatus(codes.Ok, "returned existing transaction")
			walletLogger.Info("Idempotent request detected - returning existing transaction",
				"idempotency_key", idempotencyKey,
				"existing_transaction_id", existingTx.ID.String(),
				"existing_amount", existingTx.Amount)
			return existingTx, nil
		}
		walletLogger.Debug("Idempotency key not found - processing new request", "idempotency_key", idempotencyKey)
	}

	// Validate wallet type
	if walletType != models.WalletTypeRetailerStake && walletType != models.WalletTypeRetailerWinning {
		err := fmt.Errorf("invalid wallet type for retailer: %s", walletType)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid wallet type")
		return nil, err
	}

	// CRITICAL: Create audit transaction with PENDING status BEFORE DB transaction
	// This ensures we have a trace even if the DB transaction fails
	now := time.Now()
	auditTransactionID := fmt.Sprintf("TXN-%s", uuid.New().String())
	auditDescription := fmt.Sprintf("AUDIT: %s", description)

	auditTransaction := &models.WalletTransaction{
		ID:              uuid.New(),
		TransactionID:   auditTransactionID,
		WalletOwnerID:   retailerID,
		WalletType:      walletType,
		TransactionType: models.TransactionTypeCredit,
		Amount:          amount,
		BalanceBefore:   0, // Will be unknown at this point
		BalanceAfter:    0, // Will be unknown at this point
		Description:     &auditDescription,
		Status:          models.TransactionStatusPending,
		CreditSource:    creditSource,
		IdempotencyKey:  nil, // Audit transactions don't use idempotency keys - only business transactions do
		Metadata:        map[string]interface{}{"audit_trace": true, "operation": "credit_attempt"},
		CreatedAt:       now,
	}

	// Save audit transaction OUTSIDE DB transaction (auto-commits immediately)
	if err := s.transactionRepo.CreateTransaction(ctx, auditTransaction); err != nil {
		// Log error but continue - we still want to attempt the operation
		span.RecordError(err)
		span.AddEvent("audit_transaction_creation_failed", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
	} else {
		span.SetAttributes(
			attribute.String("audit.transaction_id", auditTransactionID),
			attribute.String("audit.status", "pending"),
		)
	}

	// Start database transaction to prevent race conditions
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to begin transaction")

		// Update audit transaction to FAILED
		_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			// Update audit transaction to FAILED on any error
			_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)
		}
	}()

	var balance int64

	// Get or create appropriate wallet based on type with row-level locking
	if walletType == models.WalletTypeRetailerStake {
		wallet, err := s.walletRepo.GetRetailerStakeWalletForUpdate(ctx, tx, retailerID)
		if err == sql.ErrNoRows {
			// Wallet doesn't exist - rollback transaction and create it outside transaction
			_ = tx.Rollback()
			wallet = &models.RetailerStakeWallet{
				ID:                uuid.New(),
				RetailerID:        retailerID,
				Balance:           0,
				PendingBalance:    0,
				AvailableBalance:  0,
				Currency:          "GHS",
				Status:            models.WalletStatusActive,
				LastTransactionAt: &now,
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			if err := s.walletRepo.CreateRetailerStakeWallet(ctx, wallet); err != nil {
				// Check if this is a race condition (another goroutine created the wallet)
				if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
					// Log the race condition but don't fail - let it proceed to retry GET operation
					walletLogger.Info("race condition detected: wallet already created by concurrent request",
						slog.String("retailer_id", retailerID.String()),
						slog.String("wallet_type", "stake"))
					// Don't return error - continue to start new transaction and retry GET at line 828
				} else {
					// Actual error - fail the operation
					span.RecordError(err)
					span.SetStatus(codes.Error, "failed to create stake wallet")

					// Update audit transaction to FAILED
					_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

					return nil, fmt.Errorf("failed to create retailer stake wallet: %w", err)
				}
			}
			// Start new transaction after wallet creation
			tx, err = s.db.BeginTx(ctx, nil)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to begin transaction")

				// Update audit transaction to FAILED
				_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

				return nil, fmt.Errorf("failed to begin transaction: %w", err)
			}
			// Lock the newly created wallet
			wallet, err = s.walletRepo.GetRetailerStakeWalletForUpdate(ctx, tx, retailerID)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to lock stake wallet")

				// Update audit transaction to FAILED
				_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

				return nil, fmt.Errorf("failed to lock retailer stake wallet: %w", err)
			}
		} else if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to get stake wallet")

			// Update audit transaction to FAILED (defer won't catch this - local err scope)
			_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

			return nil, fmt.Errorf("failed to get retailer stake wallet: %w", err)
		}

		balance = wallet.Balance

		// Calculate gross amount for STAKE wallet (includes 30% commission)
		// Use math.Ceil to round UP to avoid underpaying commission due to truncation
		// Example: 100 pesewas / 0.7 = 142.857... → Ceil = 143 (not 142)
		grossAmount := int64(math.Ceil(float64(amount) / 0.7))

		// Update wallet balance with GROSS amount (base + commission)
		wallet.Balance += grossAmount
		wallet.LastTransactionAt = &now
		wallet.UpdatedAt = now

		if err = s.extendedWalletRepo.UpdateRetailerStakeWalletTx(ctx, tx, wallet); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to update stake wallet")

			// Update audit transaction to FAILED
			_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

			return nil, fmt.Errorf("failed to update retailer stake wallet: %w", err)
		}
	} else {
		// Winning wallet
		wallet, err := s.walletRepo.GetRetailerWinningWalletForUpdate(ctx, tx, retailerID)
		if err == sql.ErrNoRows {
			// Wallet doesn't exist - rollback transaction and create it outside transaction
			_ = tx.Rollback()
			wallet = &models.RetailerWinningWallet{
				ID:                uuid.New(),
				RetailerID:        retailerID,
				Balance:           0,
				PendingBalance:    0,
				AvailableBalance:  0,
				Currency:          "GHS",
				Status:            models.WalletStatusActive,
				LastTransactionAt: &now,
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			if err := s.walletRepo.CreateRetailerWinningWallet(ctx, wallet); err != nil {
				// Check if this is a race condition (another goroutine created the wallet)
				if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
					// Log the race condition but don't fail - let it proceed to retry GET operation
					walletLogger.Info("race condition detected: wallet already created by concurrent request",
						slog.String("retailer_id", retailerID.String()),
						slog.String("wallet_type", "winning"))
					// Don't return error - continue to start new transaction and retry GET at line 908
				} else {
					// Actual error - fail the operation
					span.RecordError(err)
					span.SetStatus(codes.Error, "failed to create winning wallet")

					// Update audit transaction to FAILED
					_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

					return nil, fmt.Errorf("failed to create retailer winning wallet: %w", err)
				}
			}
			// Start new transaction after wallet creation
			tx, err = s.db.BeginTx(ctx, nil)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to begin transaction")

				// Update audit transaction to FAILED
				_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

				return nil, fmt.Errorf("failed to begin transaction: %w", err)
			}
			// Lock the newly created wallet
			wallet, err = s.walletRepo.GetRetailerWinningWalletForUpdate(ctx, tx, retailerID)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to lock winning wallet")

				// Update audit transaction to FAILED
				_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

				return nil, fmt.Errorf("failed to lock retailer winning wallet: %w", err)
			}
		} else if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to get winning wallet")

			// Update audit transaction to FAILED (defer won't catch this - local err scope)
			_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

			return nil, fmt.Errorf("failed to get retailer winning wallet: %w", err)
		}

		balance = wallet.Balance

		// Update wallet balance
		wallet.Balance += amount
		wallet.LastTransactionAt = &now
		wallet.UpdatedAt = now

		if err = s.extendedWalletRepo.UpdateRetailerWinningWalletTx(ctx, tx, wallet); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to update winning wallet")

			// Update audit transaction to FAILED
			_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

			return nil, fmt.Errorf("failed to update retailer winning wallet: %w", err)
		}
	}

	// Fetch retailer details to populate owner name and code
	metadata := make(map[string]any)
	if s.agentClient != nil {
		retailer, err := s.agentClient.GetRetailer(ctx, retailerID)
		if err != nil {
			// Log error but don't fail the transaction
			span.RecordError(err)
			span.AddEvent("retailer_details_fetch_failed", trace.WithAttributes(
				attribute.String("error", err.Error()),
			))
		} else {
			metadata["owner_name"] = retailer.Name
			metadata["owner_code"] = retailer.RetailerCode
			span.SetAttributes(
				attribute.String("owner.name", retailer.Name),
				attribute.String("owner.code", retailer.RetailerCode),
			)
		}
	}

	// For RETAILER_STAKE wallet: Create TWO transactions (base + commission)
	// For RETAILER_WINNING wallet: Create ONE transaction (exact amount, no commission)
	var transaction *models.WalletTransaction

	if walletType == models.WalletTypeRetailerStake {
		// Calculate commission: gross = base / 0.7, commission = gross - base
		// Use math.Ceil to round UP to avoid underpaying commission due to truncation
		// Example: 100 pesewas / 0.7 = 142.857... → Ceil = 143 (not 142)
		grossAmount := int64(math.Ceil(float64(amount) / 0.7))
		commission := grossAmount - amount

		baseTransactionID := fmt.Sprintf("TXN-%s", uuid.New().String())
		commissionTransactionID := fmt.Sprintf("TXN-%s-COMM", uuid.New().String())

		var idempKeyPtr *string
		if idempotencyKey != "" {
			idempKeyPtr = &idempotencyKey
		}

		// Transaction 1: Base amount (Top-Up)
		baseDescription := fmt.Sprintf("%s (Base Amount)", description)
		baseMetadata := make(map[string]any)
		for k, v := range metadata {
			baseMetadata[k] = v
		}
		baseMetadata["commission_transaction_id"] = commissionTransactionID
		baseMetadata["has_commission"] = true

		baseTransaction := &models.WalletTransaction{
			ID:              uuid.New(),
			TransactionID:   baseTransactionID,
			WalletOwnerID:   retailerID,
			WalletType:      walletType,
			TransactionType: models.TransactionTypeCredit,
			Amount:          amount, // Base amount (e.g., GHS100 = 10,000 pesewas)
			BalanceBefore:   balance,
			BalanceAfter:    balance + amount,
			Description:     &baseDescription,
			Reference:       nil,
			Status:          models.TransactionStatusCompleted,
			CreditSource:    creditSource,
			IdempotencyKey:  idempKeyPtr,
			Metadata:        baseMetadata,
			CreatedAt:       now,
		}

		// Save base transaction
		if err = s.transactionRepo.CreateTransactionTx(ctx, tx, baseTransaction); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create base transaction")

			// Update audit transaction to FAILED
			_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

			return nil, fmt.Errorf("failed to create base transaction: %w", err)
		}

		// Transaction 2: Commission
		commissionDescription := fmt.Sprintf("Commission (30%%) on Top-Up")
		commissionMetadata := make(map[string]any)
		for k, v := range metadata {
			commissionMetadata[k] = v
		}
		commissionMetadata["base_transaction_id"] = baseTransactionID
		commissionMetadata["commission_rate"] = 30

		commissionTransaction := &models.WalletTransaction{
			ID:              uuid.New(),
			TransactionID:   commissionTransactionID,
			WalletOwnerID:   retailerID,
			WalletType:      walletType,
			TransactionType: models.TransactionTypeCommission,
			Amount:          commission, // Commission amount (e.g., GHS42.85 = 4,285 pesewas)
			BalanceBefore:   balance + amount,
			BalanceAfter:    balance + grossAmount, // Final balance after both transactions
			Description:     &commissionDescription,
			Reference:       nil,
			Status:          models.TransactionStatusCompleted,
			CreditSource:    creditSource,
			IdempotencyKey:  nil, // Only base transaction gets idempotency key
			Metadata:        commissionMetadata,
			CreatedAt:       now,
		}

		// Save commission transaction
		if err = s.transactionRepo.CreateTransactionTx(ctx, tx, commissionTransaction); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create commission transaction")

			// Update audit transaction to FAILED
			_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

			return nil, fmt.Errorf("failed to create commission transaction: %w", err)
		}

		// Use base transaction as the primary transaction to return
		transaction = baseTransaction
	} else {
		// RETAILER_WINNING: Single transaction, no commission
		transactionID := fmt.Sprintf("TXN-%s", uuid.New().String())
		var idempKeyPtr *string
		if idempotencyKey != "" {
			idempKeyPtr = &idempotencyKey
		}

		transaction = &models.WalletTransaction{
			ID:              uuid.New(),
			TransactionID:   transactionID,
			WalletOwnerID:   retailerID,
			WalletType:      walletType,
			TransactionType: models.TransactionTypeCredit,
			Amount:          amount,
			BalanceBefore:   balance,
			BalanceAfter:    balance + amount,
			Description:     &description,
			Reference:       nil,
			Status:          models.TransactionStatusCompleted,
			CreditSource:    creditSource,
			IdempotencyKey:  idempKeyPtr,
			Metadata:        metadata,
			CreatedAt:       now,
		}

		// Save transaction within the database transaction
		if err = s.transactionRepo.CreateTransactionTx(ctx, tx, transaction); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create transaction")

			// Update audit transaction to FAILED
			_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

			return nil, fmt.Errorf("failed to create transaction: %w", err)
		}
	}

	// CRITICAL: Save idempotency key BEFORE committing to ensure atomicity
	// This acts as a distributed lock - only one request can insert the key
	// If commit fails, idempotency key won't be saved (transaction rolls back)
	// If key already exists (race condition), we rollback and return existing transaction
	if idempotencyKey != "" {
		idempRecord := &models.IdempotencyKey{
			IdempotencyKey: idempotencyKey,
			TransactionID:  transaction.ID,
			WalletOwnerID:  retailerID,
			WalletType:     string(walletType),
			OperationType:  "credit",
			Amount:         amount,
			CreatedAt:      time.Now(),
		}

		inserted, err := s.idempotencyRepo.SaveIdempotencyKeyTx(ctx, tx, idempRecord)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to save idempotency key")

			// Update audit transaction to FAILED
			_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

			return nil, fmt.Errorf("failed to save idempotency key: %w", err)
		}

		// RACE CONDITION DETECTED: Key already existed (concurrent request won)
		if !inserted {
			_ = tx.Rollback()
			span.SetAttributes(
				attribute.String("result", "duplicate_request_detected"),
				attribute.String("idempotency.key", idempotencyKey),
			)

			// Fetch and return the existing transaction
			existingTx, fetchErr := s.idempotencyRepo.CheckIdempotencyKey(ctx, idempotencyKey)
			if fetchErr != nil {
				span.RecordError(fetchErr)

				// Update audit transaction to FAILED
				_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

				return nil, fmt.Errorf("duplicate request detected but failed to fetch existing transaction: %w", fetchErr)
			}
			if existingTx == nil {
				// This shouldn't happen - key exists but transaction not found
				// Update audit transaction to FAILED
				_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

				return nil, fmt.Errorf("duplicate request detected but existing transaction not found")
			}

			span.SetStatus(codes.Ok, "returned existing transaction (duplicate request)")
			return existingTx, nil
		}
	}

	// Commit the database transaction (includes wallet + idempotency key atomically)
	if err = tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to commit transaction")

		// Update audit transaction to FAILED
		if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed); updateErr != nil {
			span.AddEvent("audit_update_failed", trace.WithAttributes(
				attribute.String("error", updateErr.Error()),
			))
		}

		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Update audit transaction to COMPLETED
	if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusCompleted); updateErr != nil {
		// Log error but don't fail - the actual transaction succeeded
		span.AddEvent("audit_update_failed", trace.WithAttributes(
			attribute.String("error", updateErr.Error()),
		))
	} else {
		span.SetAttributes(attribute.String("audit.status", "completed"))
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("wallet:retailer:%s:%s:balance", retailerID.String(), walletType)
	s.cache.Del(ctx, cacheKey)

	span.SetStatus(codes.Ok, "wallet credited successfully")
	return transaction, nil
}

// DebitRetailerWallet debits (reduces) a retailer's wallet balance with insufficient funds check
func (s *walletService) DebitRetailerWallet(ctx context.Context, retailerID uuid.UUID, amount int64, walletType models.WalletType, description string) (*models.WalletTransaction, error) {
	ctx, span := s.tracer.Start(ctx, "DebitRetailerWallet")
	defer span.End()

	span.SetAttributes(
		attribute.String("retailer.id", retailerID.String()),
		attribute.String("wallet.type", string(walletType)),
		attribute.Int64("amount.pesewas", amount),
		attribute.String("description", description),
	)

	// Validate wallet type
	if walletType != models.WalletTypeRetailerStake && walletType != models.WalletTypeRetailerWinning {
		err := fmt.Errorf("invalid wallet type for retailer: %s", walletType)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid wallet type")
		return nil, err
	}

	// Validate amount is positive
	if amount <= 0 {
		err := fmt.Errorf("debit amount must be positive, got: %d pesewas", amount)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid amount")
		return nil, err
	}

	// CRITICAL: Create audit transaction with PENDING status BEFORE DB transaction
	// This ensures we have a trace even if the DB transaction fails
	now := time.Now()
	auditTransactionID := fmt.Sprintf("TXN-%s", uuid.New().String())
	auditDescription := fmt.Sprintf("AUDIT: %s", description)

	auditTransaction := &models.WalletTransaction{
		ID:              uuid.New(),
		TransactionID:   auditTransactionID,
		WalletOwnerID:   retailerID,
		WalletType:      walletType,
		TransactionType: models.TransactionTypeDebit,
		Amount:          amount,
		BalanceBefore:   0, // Will be unknown at this point
		BalanceAfter:    0, // Will be unknown at this point
		Description:     &auditDescription,
		Status:          models.TransactionStatusPending,
		Metadata:        map[string]interface{}{"audit_trace": true, "operation": "debit_attempt"},
		CreatedAt:       now,
	}

	// Save audit transaction OUTSIDE DB transaction (auto-commits immediately)
	if err := s.transactionRepo.CreateTransaction(ctx, auditTransaction); err != nil {
		// Log error but continue - we still want to attempt the operation
		span.RecordError(err)
		span.AddEvent("audit_transaction_creation_failed", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
	} else {
		span.SetAttributes(
			attribute.String("audit.transaction_id", auditTransactionID),
			attribute.String("audit.status", "pending"),
		)
	}

	// Start database transaction to prevent race conditions
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to begin transaction")

		// Update audit transaction to FAILED
		_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)

		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			// Update audit transaction to FAILED on any error
			_ = s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed)
		}
	}()

	var balance int64

	// Get appropriate wallet based on type with row-level lock
	if walletType == models.WalletTypeRetailerStake {
		wallet, err := s.walletRepo.GetRetailerStakeWalletForUpdate(ctx, tx, retailerID)
		if err == sql.ErrNoRows {
			err = fmt.Errorf("retailer stake wallet not found for retailer %s", retailerID)
			span.RecordError(err)
			span.SetStatus(codes.Error, "wallet not found")
			return nil, err
		} else if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to get stake wallet")
			return nil, fmt.Errorf("failed to get retailer stake wallet: %w", err)
		}

		balance = wallet.Balance

		// Check for sufficient funds
		if wallet.Balance < amount {
			err = fmt.Errorf("insufficient funds: available %d pesewas (GH₵ %.2f), required %d pesewas (GH₵ %.2f)",
				wallet.Balance, models.PesewasToGHS(wallet.Balance), amount, models.PesewasToGHS(amount))
			span.RecordError(err)
			span.SetStatus(codes.Error, "insufficient funds")
			return nil, err
		}

		// Update wallet balance
		wallet.Balance -= amount
		wallet.LastTransactionAt = &now
		wallet.UpdatedAt = now

		if err = s.extendedWalletRepo.UpdateRetailerStakeWalletTx(ctx, tx, wallet); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to update stake wallet")
			return nil, fmt.Errorf("failed to update retailer stake wallet: %w", err)
		}
	} else {
		// Winning wallet
		wallet, err := s.walletRepo.GetRetailerWinningWalletForUpdate(ctx, tx, retailerID)
		if err == sql.ErrNoRows {
			err = fmt.Errorf("retailer winning wallet not found for retailer %s", retailerID)
			span.RecordError(err)
			span.SetStatus(codes.Error, "wallet not found")
			return nil, err
		} else if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to get winning wallet")
			return nil, fmt.Errorf("failed to get retailer winning wallet: %w", err)
		}

		balance = wallet.Balance

		// Check for sufficient funds
		if wallet.Balance < amount {
			err = fmt.Errorf("insufficient funds: available %d pesewas (GH₵ %.2f), required %d pesewas (GH₵ %.2f)",
				wallet.Balance, models.PesewasToGHS(wallet.Balance), amount, models.PesewasToGHS(amount))
			span.RecordError(err)
			span.SetStatus(codes.Error, "insufficient funds")
			return nil, err
		}

		// Update wallet balance
		wallet.Balance -= amount
		wallet.LastTransactionAt = &now
		wallet.UpdatedAt = now

		if err = s.extendedWalletRepo.UpdateRetailerWinningWalletTx(ctx, tx, wallet); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to update winning wallet")
			return nil, fmt.Errorf("failed to update retailer winning wallet: %w", err)
		}
	}

	// Create transaction record
	transactionID := fmt.Sprintf("TXN-%s", uuid.New().String())
	transaction := &models.WalletTransaction{
		ID:              uuid.New(),
		TransactionID:   transactionID,
		WalletOwnerID:   retailerID,
		WalletType:      walletType,
		TransactionType: models.TransactionTypeDebit,
		Amount:          amount,
		BalanceBefore:   balance,
		BalanceAfter:    balance - amount,
		Description:     &description,
		Reference:       nil,
		Status:          models.TransactionStatusCompleted,
		IdempotencyKey:  nil,
		Metadata:        make(map[string]any),
		CreatedAt:       now,
	}

	// Save transaction within the same database transaction
	if err = s.transactionRepo.CreateTransactionTx(ctx, tx, transaction); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create transaction")
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to commit transaction")

		// Update audit transaction to FAILED
		if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed); updateErr != nil {
			span.AddEvent("audit_update_failed", trace.WithAttributes(
				attribute.String("error", updateErr.Error()),
			))
		}

		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Update audit transaction to COMPLETED
	if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusCompleted); updateErr != nil {
		// Log error but don't fail - the actual transaction succeeded
		span.AddEvent("audit_update_failed", trace.WithAttributes(
			attribute.String("error", updateErr.Error()),
		))
	} else {
		span.SetAttributes(attribute.String("audit.status", "completed"))
	}

	// Invalidate cache after successful commit
	cacheKey := fmt.Sprintf("wallet:retailer:%s:%s:balance", retailerID.String(), walletType)
	s.cache.Del(ctx, cacheKey)

	span.SetStatus(codes.Ok, "wallet debited successfully")
	return transaction, nil
}

// GetAgentBalance retrieves the current balance of an agent's wallet
// If the wallet doesn't exist, it automatically creates one
func (s *walletService) GetAgentBalance(ctx context.Context, agentID uuid.UUID) (*models.AgentStakeWallet, error) {
	ctx, span := s.tracer.Start(ctx, "GetAgentBalance")
	defer span.End()

	span.SetAttributes(
		attribute.String("agent.id", agentID.String()),
	)

	// Try cache first
	cacheKey := fmt.Sprintf("wallet:agent:%s:balance", agentID.String())
	cached, err := s.cache.Get(ctx, cacheKey).Int64()
	if err == nil {
		span.SetAttributes(
			attribute.Bool("cache.hit", true),
			attribute.Int64("balance.pesewas", cached),
		)
		// Return a wallet object with cached balance
		return &models.AgentStakeWallet{
			ID:      uuid.New(),
			AgentID: agentID,
			Balance: cached,
		}, nil
	}

	// Get from database
	wallet, err := s.walletRepo.GetByAgentID(ctx, agentID)
	if err == sql.ErrNoRows {
		// Wallet doesn't exist, create it automatically
		span.AddEvent("wallet_not_found_creating", trace.WithAttributes(
			attribute.String("agent.id", agentID.String()),
		))

		now := time.Now()
		wallet = &models.AgentStakeWallet{
			ID:                uuid.New(),
			AgentID:           agentID,
			Balance:           0,
			PendingBalance:    0,
			AvailableBalance:  0,
			Currency:          "GHS",
			Status:            "active",
			LastTransactionAt: nil,
			CreatedAt:         now,
			UpdatedAt:         now,
		}

		if err := s.walletRepo.Create(ctx, wallet); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create wallet")
			return nil, fmt.Errorf("failed to create wallet: %w", err)
		}

		span.AddEvent("wallet_created", trace.WithAttributes(
			attribute.String("wallet.id", wallet.ID.String()),
			attribute.String("agent.id", agentID.String()),
		))

		// Get actual commission rate from agent management service
		commissionPercentage := 30.0 // Default fallback
		if s.agentClient != nil {
			agentCommission, err := s.agentClient.GetAgentCommissionPercentage(ctx, agentID)
			if err != nil {
				// Log error but use default
				span.RecordError(err)
				span.AddEvent("agent_commission_fetch_error", trace.WithAttributes(
					attribute.String("error", err.Error()),
					attribute.Float64("using_default", commissionPercentage),
				))
			} else {
				commissionPercentage = agentCommission
				span.SetAttributes(attribute.Float64("commission.percentage", commissionPercentage))
			}
		}

		// Convert percentage to basis points (1% = 100 basis points)
		commissionRateBasisPoints := int32(commissionPercentage * 100)
		if err := s.commissionService.SetAgentCommissionRate(ctx, agentID, commissionRateBasisPoints, "system-auto-create"); err != nil {
			// Log error but don't fail wallet creation
			span.RecordError(err)
			span.AddEvent("commission_rate_error", trace.WithAttributes(
				attribute.String("error", err.Error()),
			))
		}

	} else if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get wallet")
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}

	// Cache the balance
	s.cache.Set(ctx, cacheKey, wallet.Balance, 5*time.Minute)

	span.SetAttributes(
		attribute.Bool("cache.hit", false),
		attribute.Int64("balance.pesewas", wallet.Balance),
	)

	span.SetStatus(codes.Ok, "balance retrieved successfully")
	return wallet, nil
}

// GetRetailerBalance retrieves the balance of a retailer's wallet
func (s *walletService) GetRetailerBalance(ctx context.Context, retailerID uuid.UUID, walletType models.WalletType) (int64, error) {
	ctx, span := s.tracer.Start(ctx, "GetRetailerBalance")
	defer span.End()

	span.SetAttributes(
		attribute.String("retailer.id", retailerID.String()),
		attribute.String("wallet.type", string(walletType)),
	)

	// Try cache first
	cacheKey := fmt.Sprintf("wallet:retailer:%s:%s:balance", retailerID.String(), walletType)
	cached, err := s.cache.Get(ctx, cacheKey).Int64()
	if err == nil {
		span.SetAttributes(
			attribute.Bool("cache.hit", true),
			attribute.Int64("balance.pesewas", cached),
		)
		return cached, nil
	}

	var balance int64
	now := time.Now()

	// Get balance based on wallet type
	switch walletType {
	case models.WalletTypeRetailerStake:
		wallet, err := s.walletRepo.GetRetailerStakeWallet(ctx, retailerID)
		if err == sql.ErrNoRows {
			// Wallet doesn't exist, create it automatically
			span.AddEvent("wallet_not_found_creating", trace.WithAttributes(
				attribute.String("retailer.id", retailerID.String()),
				attribute.String("wallet.type", "stake"),
			))

			wallet = &models.RetailerStakeWallet{
				ID:                uuid.New(),
				RetailerID:        retailerID,
				Balance:           0,
				PendingBalance:    0,
				AvailableBalance:  0,
				Currency:          "GHS",
				Status:            models.WalletStatusActive,
				LastTransactionAt: nil,
				CreatedAt:         now,
				UpdatedAt:         now,
			}

			if err := s.walletRepo.CreateRetailerStakeWallet(ctx, wallet); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to create stake wallet")
				return 0, fmt.Errorf("failed to create retailer stake wallet: %w", err)
			}

			balance = 0
		} else if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to get stake wallet")
			return 0, fmt.Errorf("failed to get retailer stake wallet: %w", err)
		} else {
			balance = wallet.Balance
		}
	case models.WalletTypeRetailerWinning:
		wallet, err := s.walletRepo.GetRetailerWinningWallet(ctx, retailerID)
		if err == sql.ErrNoRows {
			// Wallet doesn't exist, create it automatically
			span.AddEvent("wallet_not_found_creating", trace.WithAttributes(
				attribute.String("retailer.id", retailerID.String()),
				attribute.String("wallet.type", "winning"),
			))

			wallet = &models.RetailerWinningWallet{
				ID:                uuid.New(),
				RetailerID:        retailerID,
				Balance:           0,
				PendingBalance:    0,
				AvailableBalance:  0,
				Currency:          "GHS",
				Status:            models.WalletStatusActive,
				LastTransactionAt: nil,
				CreatedAt:         now,
				UpdatedAt:         now,
			}

			if err := s.walletRepo.CreateRetailerWinningWallet(ctx, wallet); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to create winning wallet")
				return 0, fmt.Errorf("failed to create retailer winning wallet: %w", err)
			}

			balance = 0
		} else if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to get winning wallet")
			return 0, fmt.Errorf("failed to get retailer winning wallet: %w", err)
		} else {
			balance = wallet.Balance
		}
	default:
		return 0, fmt.Errorf("invalid wallet type for retailer: %s", walletType)
	}

	// Cache the balance
	s.cache.Set(ctx, cacheKey, balance, 5*time.Minute)

	span.SetAttributes(
		attribute.Bool("cache.hit", false),
		attribute.Int64("balance.pesewas", balance),
	)

	span.SetStatus(codes.Ok, "balance retrieved successfully")
	return balance, nil
}

// TransferAgentToRetailer transfers funds from agent to retailer with commission
func (s *walletService) TransferAgentToRetailer(ctx context.Context, agentID, retailerID uuid.UUID, amount int64, description string) (*models.WalletTransaction, error) {
	_, span := s.tracer.Start(ctx, "TransferAgentToRetailer")
	defer span.End()

	return nil, fmt.Errorf("agent to retailer transfer not yet implemented")
}

// GetTransactionHistory retrieves transaction history for a wallet
func (s *walletService) GetTransactionHistory(ctx context.Context, walletOwnerID uuid.UUID, walletType models.WalletType, limit, offset int) ([]*models.WalletTransaction, int, error) {
	_, span := s.tracer.Start(ctx, "GetTransactionHistory")
	defer span.End()

	span.SetAttributes(
		attribute.String("owner.id", walletOwnerID.String()),
		attribute.String("wallet.type", string(walletType)),
		attribute.Int("limit", limit),
		attribute.Int("offset", offset),
	)

	// Get transactions from repository
	transactions, err := s.transactionRepo.GetTransactionHistory(ctx, walletOwnerID, walletType, limit, offset)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get transactions")
		return nil, 0, fmt.Errorf("failed to get transactions: %w", err)
	}

	// Get total count - for now, we'll estimate based on whether we got a full page
	totalCount := offset + len(transactions)
	if len(transactions) == limit {
		// If we got a full page, there might be more
		totalCount = offset + len(transactions) + 1
	}

	span.SetAttributes(
		attribute.Int("transaction.count", len(transactions)),
		attribute.Int("total.count", totalCount),
	)

	return transactions, totalCount, nil
}

// LockWallet locks a wallet to prevent transactions
func (s *walletService) LockWallet(ctx context.Context, walletID uuid.UUID, reason string) error {
	_, span := s.tracer.Start(ctx, "LockWallet")
	defer span.End()

	return fmt.Errorf("wallet locking not yet implemented")
}

// UnlockWallet unlocks a previously locked wallet
func (s *walletService) UnlockWallet(ctx context.Context, walletID uuid.UUID) error {
	_, span := s.tracer.Start(ctx, "UnlockWallet")
	defer span.End()

	return fmt.Errorf("wallet unlocking not yet implemented")
}

// UpdateAgentCommission updates the commission rate for an agent
// This is called by the agent management service when an agent's commission is updated
func (s *walletService) UpdateAgentCommission(ctx context.Context, agentID uuid.UUID, newRate float64, updatedBy string, reason string) (float64, error) {
	_, span := s.tracer.Start(ctx, "UpdateAgentCommission")
	defer span.End()

	span.SetAttributes(
		attribute.String("agent.id", agentID.String()),
		attribute.Float64("commission.new_rate", newRate),
		attribute.String("updated.by", updatedBy),
		attribute.String("update.reason", reason),
	)

	// Get the current commission rate
	currentRateModel, err := s.commissionService.GetAgentCommissionRate(ctx, agentID)
	var previousRate float64
	if err != nil {
		span.RecordError(err)
		// If no current rate exists, we'll set it as a new rate
		previousRate = 0
	} else if currentRateModel != nil {
		// Convert from basis points to decimal (e.g., 3000 -> 0.30)
		previousRate = float64(currentRateModel.Rate) / 10000
	}

	// Convert decimal rate to basis points for storage (e.g., 0.30 -> 3000)
	newRateBasisPoints := int32(newRate * 10000)

	// Update the commission rate using the commission service
	err = s.commissionService.SetAgentCommissionRate(ctx, agentID, newRateBasisPoints, updatedBy)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update commission rate")
		return previousRate, fmt.Errorf("failed to update commission rate: %w", err)
	}

	// Log the update
	fmt.Printf("Updated commission rate for agent %s from %.2f%% to %.2f%% (reason: %s, by: %s)\n",
		agentID.String(), previousRate*100, newRate*100, reason, updatedBy)

	span.SetStatus(codes.Ok, "commission rate updated successfully")
	return previousRate, nil
}

// ReserveRetailerWalletFunds reserves funds in a retailer's wallet (Phase 1 of 2PC)
// This is used for external transactions like withdrawals where we need to ensure funds are available
// before attempting the external operation
func (s *walletService) ReserveRetailerWalletFunds(ctx context.Context, retailerID uuid.UUID, walletType models.WalletType, amount int64, reference string, ttlSeconds int32, reason string, idempotencyKey string) (string, error) {
	ctx, span := s.tracer.Start(ctx, "ReserveRetailerWalletFunds")
	defer span.End()

	span.SetAttributes(
		attribute.String("retailer.id", retailerID.String()),
		attribute.String("wallet.type", string(walletType)),
		attribute.Int64("amount.pesewas", amount),
		attribute.String("reference", reference),
		attribute.Int("ttl.seconds", int(ttlSeconds)),
	)

	// Check idempotency - if we already have a reservation for this reference, return it
	if idempotencyKey != "" {
		existingReservation, err := s.reservationRepo.GetByReference(ctx, reference)
		if err == nil && existingReservation != nil {
			span.SetAttributes(
				attribute.String("result", "idempotent_response"),
				attribute.String("existing.reservation_id", existingReservation.ReservationID),
			)
			span.SetStatus(codes.Ok, "returned existing reservation")
			return existingReservation.ReservationID, nil
		}
	}

	// Validate wallet type
	if walletType != models.WalletTypeRetailerWinning {
		err := fmt.Errorf("reservations only supported for RETAILER_WINNING wallets, got: %s", walletType)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid wallet type")
		return "", err
	}

	// Get wallet
	wallet, err := s.walletRepo.GetRetailerWinningWallet(ctx, retailerID)
	if err == sql.ErrNoRows {
		err = fmt.Errorf("retailer winning wallet not found for retailer %s", retailerID)
		span.RecordError(err)
		span.SetStatus(codes.Error, "wallet not found")
		return "", err
	} else if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get wallet")
		return "", fmt.Errorf("failed to get retailer winning wallet: %w", err)
	}

	// Check for sufficient funds (must account for existing reservations)
	activeReservations, err := s.reservationRepo.GetActiveReservations(ctx, retailerID, walletType)
	if err != nil {
		span.RecordError(err)
		return "", fmt.Errorf("failed to get active reservations: %w", err)
	}

	var totalReserved int64
	for _, res := range activeReservations {
		totalReserved += res.Amount
	}

	availableBalance := wallet.Balance - totalReserved
	if availableBalance < amount {
		err = fmt.Errorf("insufficient funds: available %d pesewas (GH₵ %.2f), required %d pesewas (GH₵ %.2f), reserved %d pesewas",
			availableBalance, models.PesewasToGHS(availableBalance), amount, models.PesewasToGHS(amount), totalReserved)
		span.RecordError(err)
		span.SetStatus(codes.Error, "insufficient funds")
		return "", err
	}

	// Create reservation
	reservationID := fmt.Sprintf("RES-%s", uuid.New().String())
	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	var idempKeyPtr *string
	if idempotencyKey != "" {
		idempKeyPtr = &idempotencyKey
	}

	reservation := &models.WalletReservation{
		ID:             uuid.New(),
		ReservationID:  reservationID,
		WalletOwnerID:  retailerID,
		WalletType:     walletType,
		Amount:         amount,
		Reference:      reference,
		Reason:         reason,
		Status:         models.ReservationStatusActive,
		IdempotencyKey: idempKeyPtr,
		CreatedAt:      time.Now(),
		ExpiresAt:      expiresAt,
	}

	if err := s.reservationRepo.Create(ctx, reservation); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create reservation")
		return "", fmt.Errorf("failed to create reservation: %w", err)
	}

	span.SetAttributes(
		attribute.String("reservation.id", reservationID),
		attribute.Int64("available.balance", availableBalance),
		attribute.String("expires.at", expiresAt.Format(time.RFC3339)),
	)

	span.SetStatus(codes.Ok, "funds reserved successfully")
	return reservationID, nil
}

// CommitReservedDebit commits a reservation and actually debits the wallet (Phase 2 of 2PC)
// This should only be called after the external operation (e.g., mobile money credit) succeeded
func (s *walletService) CommitReservedDebit(ctx context.Context, reservationID string, notes string) (uuid.UUID, error) {
	ctx, span := s.tracer.Start(ctx, "CommitReservedDebit")
	defer span.End()

	span.SetAttributes(
		attribute.String("reservation.id", reservationID),
	)

	// Get reservation
	reservation, err := s.reservationRepo.GetByReservationID(ctx, reservationID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "reservation not found")
		return uuid.Nil, fmt.Errorf("reservation not found: %w", err)
	}

	// Verify reservation is active
	if reservation.Status != models.ReservationStatusActive {
		err = fmt.Errorf("reservation is not active: %s (status: %s)", reservationID, reservation.Status)
		span.RecordError(err)
		span.SetStatus(codes.Error, "reservation not active")
		return uuid.Nil, err
	}

	// Check if reservation expired
	if time.Now().After(reservation.ExpiresAt) {
		err = fmt.Errorf("reservation has expired: %s (expired at: %s)", reservationID, reservation.ExpiresAt.Format(time.RFC3339))
		span.RecordError(err)
		span.SetStatus(codes.Error, "reservation expired")
		return uuid.Nil, err
	}

	// Get wallet
	var balance int64
	now := time.Now()

	if reservation.WalletType == models.WalletTypeRetailerWinning {
		wallet, err := s.walletRepo.GetRetailerWinningWallet(ctx, reservation.WalletOwnerID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "wallet not found")
			return uuid.Nil, fmt.Errorf("wallet not found: %w", err)
		}

		balance = wallet.Balance

		// Debit the wallet
		wallet.Balance -= reservation.Amount
		wallet.LastTransactionAt = &now
		wallet.UpdatedAt = now

		if err := s.extendedWalletRepo.UpdateRetailerWinningWallet(ctx, wallet); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to update wallet")
			return uuid.Nil, fmt.Errorf("failed to update wallet: %w", err)
		}
	} else {
		err = fmt.Errorf("commit only supported for RETAILER_WINNING wallets")
		span.RecordError(err)
		span.SetStatus(codes.Error, "unsupported wallet type")
		return uuid.Nil, err
	}

	transactionID := fmt.Sprintf("TXN-%s", uuid.New().String())
	description := fmt.Sprintf("%s (committed from reservation %s)", notes, reservationID)
	transaction := &models.WalletTransaction{
		ID:              uuid.New(),
		TransactionID:   transactionID,
		WalletOwnerID:   reservation.WalletOwnerID,
		WalletType:      reservation.WalletType,
		TransactionType: models.TransactionTypeDebit,
		Amount:          reservation.Amount,
		BalanceBefore:   balance,
		BalanceAfter:    balance - reservation.Amount,
		Description:     &description,
		Reference:       &reservation.Reference,
		Status:          models.TransactionStatusCompleted,
		Metadata:        map[string]any{"reservation_id": reservationID},
		CreatedAt:       now,
	}

	if err := s.transactionRepo.CreateTransaction(ctx, transaction); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create transaction")
		return uuid.Nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	if err := s.reservationRepo.MarkAsCommitted(ctx, reservationID, transaction.ID); err != nil {
		span.RecordError(err)
		// Transaction already created, log error but don't fail
		span.AddEvent("reservation_commit_failed", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
	}

	cacheKey := fmt.Sprintf("wallet:retailer:%s:%s:balance", reservation.WalletOwnerID.String(), reservation.WalletType)
	s.cache.Del(ctx, cacheKey)

	span.SetAttributes(
		attribute.String("transaction.id", transaction.ID.String()),
		attribute.Int64("new.balance", balance-reservation.Amount),
	)

	span.SetStatus(codes.Ok, "reservation committed successfully")
	return transaction.ID, nil
}

// ReleaseReservation releases a reservation without debiting the wallet (Compensation)
// This is called when the external operation (e.g., mobile money credit) failed
func (s *walletService) ReleaseReservation(ctx context.Context, reservationID string, reason string) error {
	ctx, span := s.tracer.Start(ctx, "ReleaseReservation")
	defer span.End()

	span.SetAttributes(
		attribute.String("reservation.id", reservationID),
		attribute.String("reason", reason),
	)

	// Get reservation
	reservation, err := s.reservationRepo.GetByReservationID(ctx, reservationID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "reservation not found")
		return fmt.Errorf("reservation not found: %w", err)
	}

	// Verify reservation is active
	if reservation.Status != models.ReservationStatusActive {
		// Already released or committed - this is idempotent
		span.SetAttributes(
			attribute.String("result", "idempotent"),
			attribute.String("current.status", string(reservation.Status)),
		)
		span.SetStatus(codes.Ok, "reservation already processed")
		return nil
	}

	// Mark reservation as released
	if err := s.reservationRepo.MarkAsReleased(ctx, reservationID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to release reservation")
		return fmt.Errorf("failed to release reservation: %w", err)
	}

	span.SetAttributes(
		attribute.Int64("released.amount", reservation.Amount),
		attribute.String("wallet.owner", reservation.WalletOwnerID.String()),
	)

	span.SetStatus(codes.Ok, "reservation released successfully")
	return nil
}

// GetAllTransactions retrieves all transactions with filters (Admin only)
func (s *walletService) GetAllTransactions(ctx context.Context, filters repositories.AdminTransactionFilters) ([]*models.WalletTransaction, int, error) {
	ctx, span := s.tracer.Start(ctx, "GetAllTransactions")
	defer span.End()

	span.SetAttributes(
		attribute.Int("page", filters.Page),
		attribute.Int("page_size", filters.PageSize),
		attribute.Int("transaction_types_count", len(filters.TransactionTypes)),
		attribute.Int("wallet_types_count", len(filters.WalletTypes)),
		attribute.Int("statuses_count", len(filters.Statuses)),
	)

	if filters.SearchTerm != nil && *filters.SearchTerm != "" {
		span.SetAttributes(attribute.String("search_term", *filters.SearchTerm))
	}

	// Call repository layer
	transactions, totalCount, err := s.adminTransactionRepo.GetAllTransactions(ctx, filters)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get all transactions")
		return nil, 0, fmt.Errorf("failed to get all transactions: %w", err)
	}

	span.SetAttributes(
		attribute.Int("transactions.count", len(transactions)),
		attribute.Int("total.count", totalCount),
	)

	span.SetStatus(codes.Ok, "transactions retrieved successfully")
	return transactions, totalCount, nil
}

// GetTransactionStatistics calculates aggregated transaction statistics based on filters
func (s *walletService) GetTransactionStatistics(ctx context.Context, filters repositories.AdminTransactionFilters) (*repositories.TransactionStatistics, error) {
	ctx, span := s.tracer.Start(ctx, "GetTransactionStatistics")
	defer span.End()

	span.SetAttributes(
		attribute.Int("transaction_types_count", len(filters.TransactionTypes)),
		attribute.Int("wallet_types_count", len(filters.WalletTypes)),
		attribute.Int("statuses_count", len(filters.Statuses)),
	)

	// Call repository layer to get statistics
	stats, err := s.adminTransactionRepo.GetTransactionStatistics(ctx, filters)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get transaction statistics")
		return nil, fmt.Errorf("failed to get transaction statistics: %w", err)
	}

	span.SetAttributes(
		attribute.Int64("total.volume", stats.TotalVolume),
		attribute.Int64("total.credits", stats.TotalCredits),
		attribute.Int64("total.debits", stats.TotalDebits),
		attribute.Int("pending.count", stats.PendingCount),
		attribute.Int("completed.count", stats.CompletedCount),
	)

	span.SetStatus(codes.Ok, "statistics retrieved successfully")
	return stats, nil
}

// Player wallet operations

func (s *walletService) CreatePlayerWallet(ctx context.Context, playerID uuid.UUID, playerCode string) (uuid.UUID, error) {
	fmt.Printf("[WALLET] Creating player wallet - ID: %s, Code: %s\n",
		playerID.String(), playerCode)

	ctx, span := s.tracer.Start(ctx, "CreatePlayerWallet")
	defer span.End()

	span.SetAttributes(
		attribute.String("player.id", playerID.String()),
		attribute.String("player.code", playerCode),
	)

	existingWallet, err := s.walletRepo.GetPlayerWallet(ctx, playerID)
	if err == nil && existingWallet != nil {
		span.SetStatus(codes.Ok, "wallet already exists")
		return existingWallet.ID, nil
	}

	now := time.Now()
	wallet := &models.PlayerWallet{
		ID:                uuid.New(),
		PlayerID:          playerID,
		Balance:           0,
		PendingBalance:    0,
		AvailableBalance:  0,
		Currency:          "GHS",
		Status:            models.WalletStatusActive,
		LastTransactionAt: nil,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.walletRepo.CreatePlayerWallet(ctx, wallet); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create player wallet")
		fmt.Printf("[WALLET] Failed to create player wallet: %v\n", err)
		return uuid.Nil, fmt.Errorf("failed to create player wallet: %w", err)
	}

	fmt.Printf("[WALLET] Player wallet created - ID: %s, PlayerID: %s\n",
		wallet.ID.String(), playerID.String())
	fmt.Printf("[WALLET] Wallet created successfully for player %s (%s)\n",
		playerCode, playerID.String())

	span.SetStatus(codes.Ok, "player wallet created successfully")
	return wallet.ID, nil
}

func (s *walletService) CreditPlayerWallet(ctx context.Context, playerID uuid.UUID, amount int64, description string, idempotencyKey string, creditSource models.CreditSource) (*models.WalletTransaction, error) {
	ctx, span := s.tracer.Start(ctx, "CreditPlayerWallet")
	defer span.End()

	span.SetAttributes(
		attribute.String("player.id", playerID.String()),
		attribute.Int64("amount", amount),
		attribute.String("description", description),
	)

	if idempotencyKey != "" {
		existingTx, err := s.transactionRepo.GetTransactionByIdempotencyKey(ctx, idempotencyKey)
		if err == nil && existingTx != nil {
			span.AddEvent("duplicate_transaction_prevented", trace.WithAttributes(
				attribute.String("transaction.id", existingTx.ID.String()),
			))
			return existingTx, nil
		}
	}

	wallet, err := s.walletRepo.GetPlayerWallet(ctx, playerID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "wallet not found")
		return nil, fmt.Errorf("player wallet not found: %w", err)
	}

	if wallet.Status != models.WalletStatusActive {
		err := fmt.Errorf("wallet is not active")
		span.RecordError(err)
		span.SetStatus(codes.Error, "wallet not active")
		return nil, err
	}

	// CRITICAL: Create audit transaction with PENDING status FIRST
	// This ensures we have a trace even if the operation fails
	now := time.Now()
	auditTransactionID := fmt.Sprintf("TXN-%s", uuid.New().String())
	auditDescription := fmt.Sprintf("AUDIT: %s", description)

	var idempKeyPtr *string
	if idempotencyKey != "" {
		idempKeyPtr = &idempotencyKey
	}

	auditTransaction := &models.WalletTransaction{
		ID:              uuid.New(),
		TransactionID:   auditTransactionID,
		WalletOwnerID:   playerID,
		WalletType:      models.WalletTypePlayerWallet,
		TransactionType: models.TransactionTypeCredit,
		Amount:          amount,
		BalanceBefore:   0,
		BalanceAfter:    0,
		Description:     &auditDescription,
		Status:          models.TransactionStatusPending,
		CreditSource:    creditSource,
		IdempotencyKey:  idempKeyPtr,
		Metadata:        map[string]interface{}{"audit_trace": true, "operation": "credit_attempt"},
		CreatedAt:       now,
	}

	// Save audit transaction (auto-commits immediately)
	if err := s.transactionRepo.CreateTransaction(ctx, auditTransaction); err != nil {
		// Log error but continue - we still want to attempt the operation
		span.RecordError(err)
		span.AddEvent("audit_transaction_creation_failed", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
	} else {
		span.SetAttributes(
			attribute.String("audit.transaction_id", auditTransactionID),
			attribute.String("audit.status", "pending"),
		)
	}

	transaction := &models.WalletTransaction{
		ID:              uuid.New(),
		TransactionID:   uuid.New().String(),
		WalletOwnerID:   playerID,
		WalletType:      models.WalletTypePlayerWallet,
		TransactionType: models.TransactionTypeCredit,
		Amount:          amount,
		BalanceBefore:   0,
		BalanceAfter:    0,
		Description:     &description,
		Status:          models.TransactionStatusCompleted,
		CreditSource:    creditSource,
		IdempotencyKey:  &idempotencyKey,
		CreatedAt:       time.Now(),
		Metadata:        make(map[string]any),
	}

	transaction.BalanceBefore = wallet.Balance
	wallet.Balance += amount
	wallet.AvailableBalance += amount
	wallet.LastTransactionAt = &transaction.CreatedAt
	transaction.BalanceAfter = wallet.Balance

	if err := s.walletRepo.UpdatePlayerWallet(ctx, wallet); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update wallet")

		// Update audit transaction to FAILED
		if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed); updateErr != nil {
			span.AddEvent("audit_update_failed", trace.WithAttributes(
				attribute.String("error", updateErr.Error()),
			))
		} else {
			span.SetAttributes(attribute.String("audit.status", "failed"))
		}

		return nil, fmt.Errorf("failed to update player wallet: %w", err)
	}

	if err := s.transactionRepo.CreateTransaction(ctx, transaction); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create transaction")

		// Update audit transaction to FAILED
		if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed); updateErr != nil {
			span.AddEvent("audit_update_failed", trace.WithAttributes(
				attribute.String("error", updateErr.Error()),
			))
		} else {
			span.SetAttributes(attribute.String("audit.status", "failed"))
		}

		return nil, fmt.Errorf("failed to create wallet transaction: %w", err)
	}

	// Update audit transaction to COMPLETED
	if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusCompleted); updateErr != nil {
		// Log error but don't fail - the actual transaction succeeded
		span.AddEvent("audit_update_failed", trace.WithAttributes(
			attribute.String("error", updateErr.Error()),
		))
	} else {
		span.SetAttributes(attribute.String("audit.status", "completed"))
	}

	span.SetStatus(codes.Ok, "player wallet credited successfully")
	return transaction, nil
}

func (s *walletService) DebitPlayerWallet(ctx context.Context, playerID uuid.UUID, amount int64, description string) (*models.WalletTransaction, error) {
	ctx, span := s.tracer.Start(ctx, "DebitPlayerWallet")
	defer span.End()

	span.SetAttributes(
		attribute.String("player.id", playerID.String()),
		attribute.Int64("amount", amount),
		attribute.String("description", description),
	)

	// CRITICAL: Create audit transaction with PENDING status FIRST
	// This ensures we have a trace even if the operation fails
	now := time.Now()
	auditTransactionID := fmt.Sprintf("TXN-%s", uuid.New().String())
	auditDescription := fmt.Sprintf("AUDIT: %s", description)

	auditTransaction := &models.WalletTransaction{
		ID:              uuid.New(),
		TransactionID:   auditTransactionID,
		WalletOwnerID:   playerID,
		WalletType:      models.WalletTypePlayerWallet,
		TransactionType: models.TransactionTypeDebit,
		Amount:          amount,
		BalanceBefore:   0,
		BalanceAfter:    0,
		Description:     &auditDescription,
		Status:          models.TransactionStatusPending,
		Metadata:        map[string]interface{}{"audit_trace": true, "operation": "debit_attempt"},
		CreatedAt:       now,
	}

	// Save audit transaction (auto-commits immediately)
	if err := s.transactionRepo.CreateTransaction(ctx, auditTransaction); err != nil {
		// Log error but continue - we still want to attempt the operation
		span.RecordError(err)
		span.AddEvent("audit_transaction_creation_failed", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
	} else {
		span.SetAttributes(
			attribute.String("audit.transaction_id", auditTransactionID),
			attribute.String("audit.status", "pending"),
		)
	}

	wallet, err := s.walletRepo.GetPlayerWallet(ctx, playerID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "wallet not found")

		// Update audit transaction to FAILED
		if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed); updateErr != nil {
			span.AddEvent("audit_update_failed", trace.WithAttributes(
				attribute.String("error", updateErr.Error()),
			))
		} else {
			span.SetAttributes(attribute.String("audit.status", "failed"))
		}

		return nil, fmt.Errorf("player wallet not found: %w", err)
	}

	if wallet.Status != models.WalletStatusActive {
		err := fmt.Errorf("wallet is not active")
		span.RecordError(err)
		span.SetStatus(codes.Error, "wallet not active")

		// Update audit transaction to FAILED
		if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed); updateErr != nil {
			span.AddEvent("audit_update_failed", trace.WithAttributes(
				attribute.String("error", updateErr.Error()),
			))
		} else {
			span.SetAttributes(attribute.String("audit.status", "failed"))
		}

		return nil, err
	}

	if wallet.AvailableBalance < amount {
		err := fmt.Errorf("insufficient balance: %d < %d", wallet.AvailableBalance, amount)
		span.RecordError(err)
		span.SetStatus(codes.Error, "insufficient balance")

		// Update audit transaction to FAILED
		if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed); updateErr != nil {
			span.AddEvent("audit_update_failed", trace.WithAttributes(
				attribute.String("error", updateErr.Error()),
			))
		} else {
			span.SetAttributes(attribute.String("audit.status", "failed"))
		}

		return nil, err
	}

	transaction := &models.WalletTransaction{
		ID:              uuid.New(),
		TransactionID:   uuid.New().String(),
		WalletOwnerID:   playerID,
		WalletType:      models.WalletTypePlayerWallet,
		TransactionType: models.TransactionTypeDebit,
		Amount:          amount,
		BalanceBefore:   0,
		BalanceAfter:    0,
		Description:     &description,
		Status:          models.TransactionStatusCompleted,
		Metadata:        make(map[string]any),
		CreatedAt:       time.Now(),
	}

	transaction.BalanceBefore = wallet.Balance
	wallet.Balance -= amount
	wallet.AvailableBalance -= amount
	wallet.LastTransactionAt = &transaction.CreatedAt
	transaction.BalanceAfter = wallet.Balance

	if err := s.walletRepo.UpdatePlayerWallet(ctx, wallet); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update wallet")

		// Update audit transaction to FAILED
		if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed); updateErr != nil {
			span.AddEvent("audit_update_failed", trace.WithAttributes(
				attribute.String("error", updateErr.Error()),
			))
		} else {
			span.SetAttributes(attribute.String("audit.status", "failed"))
		}

		return nil, fmt.Errorf("failed to update player wallet: %w", err)
	}

	if err := s.transactionRepo.CreateTransaction(ctx, transaction); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create transaction")

		// Update audit transaction to FAILED
		if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusFailed); updateErr != nil {
			span.AddEvent("audit_update_failed", trace.WithAttributes(
				attribute.String("error", updateErr.Error()),
			))
		} else {
			span.SetAttributes(attribute.String("audit.status", "failed"))
		}

		return nil, fmt.Errorf("failed to create wallet transaction: %w", err)
	}

	// Update audit transaction to COMPLETED
	if updateErr := s.transactionRepo.UpdateTransactionStatus(ctx, auditTransactionID, models.TransactionStatusCompleted); updateErr != nil {
		// Log error but don't fail - the actual transaction succeeded
		span.AddEvent("audit_update_failed", trace.WithAttributes(
			attribute.String("error", updateErr.Error()),
		))
	} else {
		span.SetAttributes(attribute.String("audit.status", "completed"))
	}

	span.SetStatus(codes.Ok, "player wallet debited successfully")
	return transaction, nil
}

func (s *walletService) ReservePlayerWalletFunds(ctx context.Context, playerID uuid.UUID, amount int64, reference string, ttlSeconds int32, reason string, idempotencyKey string) (string, error) {
	ctx, span := s.tracer.Start(ctx, "ReservePlayerWalletFunds")
	defer span.End()

	span.SetAttributes(
		attribute.String("player.id", playerID.String()),
		attribute.Int64("amount", amount),
		attribute.String("reference", reference),
		attribute.Int("ttl_seconds", int(ttlSeconds)),
		attribute.String("reason", reason),
	)

	wallet, err := s.walletRepo.GetPlayerWallet(ctx, playerID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "wallet not found")
		return "", fmt.Errorf("player wallet not found: %w", err)
	}

	if wallet.Status != models.WalletStatusActive {
		err := fmt.Errorf("wallet is not active")
		span.RecordError(err)
		span.SetStatus(codes.Error, "wallet not active")
		return "", err
	}

	if wallet.AvailableBalance < amount {
		err := fmt.Errorf("insufficient balance: %d < %d", wallet.AvailableBalance, amount)
		span.RecordError(err)
		span.SetStatus(codes.Error, "insufficient balance")
		return "", err
	}

	reservationID := uuid.New().String()
	now := time.Now()
	expiresAt := now.Add(time.Duration(ttlSeconds) * time.Second)

	reservation := &models.WalletReservation{
		ID:            uuid.New(),
		ReservationID: reservationID,
		WalletOwnerID: playerID,
		WalletType:    models.WalletTypePlayerWallet,
		Amount:        amount,
		Reference:     reference,
		Reason:        reason,
		Status:        models.ReservationStatusActive,
		ExpiresAt:     expiresAt,
		CreatedAt:     now,
	}

	if err := s.reservationRepo.Create(ctx, reservation); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create reservation")
		return "", fmt.Errorf("failed to create wallet reservation: %w", err)
	}

	wallet.AvailableBalance -= amount
	wallet.PendingBalance += amount
	wallet.LastTransactionAt = &now

	if err := s.walletRepo.UpdatePlayerWallet(ctx, wallet); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update wallet")
		return "", fmt.Errorf("failed to update player wallet: %w", err)
	}

	span.SetStatus(codes.Ok, "player wallet funds reserved successfully")
	return reservationID, nil
}

func (s *walletService) GetPlayerBalance(ctx context.Context, playerID uuid.UUID) (*models.PlayerWallet, error) {
	ctx, span := s.tracer.Start(ctx, "GetPlayerBalance")
	defer span.End()

	span.SetAttributes(
		attribute.String("player.id", playerID.String()),
	)

	wallet, err := s.walletRepo.GetPlayerWallet(ctx, playerID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "wallet not found")
		return nil, fmt.Errorf("player wallet not found: %w", err)
	}

	span.SetStatus(codes.Ok, "player balance retrieved successfully")
	return wallet, nil
}

// ReverseTransaction reverses a completed credit transaction (Admin only)
// Returns: (originalTransaction, reversalTransaction, error)
func (s *walletService) ReverseTransaction(ctx context.Context, txID uuid.UUID, reason string, adminID uuid.UUID, adminName, adminEmail string) (*models.WalletTransaction, *models.WalletTransaction, error) {
	ctx, span := s.tracer.Start(ctx, "ReverseTransaction")
	defer span.End()

	span.SetAttributes(
		attribute.String("transaction.id", txID.String()),
		attribute.String("admin.id", adminID.String()),
		attribute.String("admin.name", adminName),
		attribute.String("reason", reason),
	)

	// Validate reason length
	if len(reason) < 20 {
		err := fmt.Errorf("reversal reason must be at least 20 characters, got %d", len(reason))
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid reason")
		return nil, nil, err
	}

	// Step 1: Get the original transaction
	originalTx, err := s.reversalRepo.GetTransactionByID(ctx, txID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "transaction not found")
		return nil, nil, fmt.Errorf("transaction not found: %w", err)
	}

	span.SetAttributes(
		attribute.String("original.wallet_type", string(originalTx.WalletType)),
		attribute.String("original.transaction_type", string(originalTx.TransactionType)),
		attribute.Int64("original.amount", originalTx.Amount),
		attribute.String("original.status", string(originalTx.Status)),
	)

	// Step 2: Validate transaction eligibility
	// Only CREDIT transactions can be reversed (not COMMISSION directly)
	if originalTx.TransactionType == models.TransactionTypeCommission {
		err := fmt.Errorf("cannot reverse COMMISSION transaction directly - please reverse the associated CREDIT transaction instead")
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid transaction type")
		return nil, nil, err
	}
	if originalTx.TransactionType != models.TransactionTypeCredit {
		err := fmt.Errorf("only CREDIT transactions can be reversed, got: %s", originalTx.TransactionType)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid transaction type")
		return nil, nil, err
	}

	// Transaction must be COMPLETED
	if originalTx.Status != models.TransactionStatusCompleted {
		err := fmt.Errorf("only COMPLETED transactions can be reversed, got status: %s", originalTx.Status)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid transaction status")
		return nil, nil, err
	}

	// Check if already reversed
	if originalTx.Status == models.TransactionStatusReversed {
		err := fmt.Errorf("transaction already reversed")
		span.RecordError(err)
		span.SetStatus(codes.Error, "already reversed")
		return nil, nil, err
	}

	// Check if a reversal already exists for this transaction
	existingReversal, err := s.reversalRepo.GetReversalByOriginalTransaction(ctx, txID)
	if err != nil {
		span.RecordError(err)
		return nil, nil, fmt.Errorf("failed to check existing reversal: %w", err)
	}
	if existingReversal != nil {
		err := fmt.Errorf("transaction already has a reversal record")
		span.RecordError(err)
		span.SetStatus(codes.Error, "duplicate reversal")
		return nil, nil, err
	}

	// Validate transaction age (24 hours)
	if time.Since(originalTx.CreatedAt) > 24*time.Hour {
		err := fmt.Errorf("transaction too old for reversal: created at %s (age: %s)",
			originalTx.CreatedAt.Format(time.RFC3339),
			time.Since(originalTx.CreatedAt).Round(time.Hour))
		span.RecordError(err)
		span.SetStatus(codes.Error, "transaction too old")
		return nil, nil, err
	}

	// Step 2.5: Check if this CREDIT transaction has an associated COMMISSION transaction
	var commissionTx *models.WalletTransaction
	var hasCommission bool
	totalReversalAmount := originalTx.Amount

	if originalTx.Metadata != nil {
		if hasCommVal, ok := originalTx.Metadata["has_commission"]; ok {
			if hasCommBool, ok := hasCommVal.(bool); ok && hasCommBool {
				hasCommission = true
				// Get commission transaction ID from metadata
				commTxID, ok := originalTx.Metadata["commission_transaction_id"].(string)
				if !ok || commTxID == "" {
					// DATA INTEGRITY ERROR: has_commission is true but commission_transaction_id is missing
					err := fmt.Errorf("data integrity error: transaction has commission flag but missing commission_transaction_id in metadata")
					span.RecordError(err)
					span.SetStatus(codes.Error, "missing commission transaction ID")
					return nil, nil, err
				}

				// Find the commission transaction by transaction_id (not UUID)
				commissionTx, err = s.reversalRepo.GetTransactionByTransactionID(ctx, commTxID)
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, "failed to find commission transaction")
					return nil, nil, fmt.Errorf("failed to find commission transaction: %w", err)
				}

				// Validate commission transaction
				if commissionTx.TransactionType != models.TransactionTypeCommission {
					err := fmt.Errorf("linked transaction is not a COMMISSION type: %s", commissionTx.TransactionType)
					span.RecordError(err)
					return nil, nil, err
				}

				// Calculate total reversal amount (base + commission)
				totalReversalAmount = originalTx.Amount + commissionTx.Amount

				span.SetAttributes(
					attribute.Bool("has_commission", true),
					attribute.String("commission_transaction.id", commissionTx.ID.String()),
					attribute.Int64("commission.amount", commissionTx.Amount),
					attribute.Int64("total_reversal.amount", totalReversalAmount),
				)
			}
		}
	}

	span.SetAttributes(
		attribute.Bool("reversal.has_commission", hasCommission),
		attribute.Int64("reversal.total_amount", totalReversalAmount),
	)

	// Step 3: Begin database transaction
	dbTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to begin transaction")
		return nil, nil, fmt.Errorf("failed to begin database transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = dbTx.Rollback()
		}
	}()

	// Step 4: Lock the original transaction
	lockedTx, err := s.reversalRepo.GetTransactionForReversalWithLock(ctx, dbTx, txID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to lock transaction")
		return nil, nil, fmt.Errorf("failed to lock transaction: %w", err)
	}

	// Re-validate status after locking (prevent race condition)
	if lockedTx.Status != models.TransactionStatusCompleted {
		err := fmt.Errorf("transaction status changed after locking: %s", lockedTx.Status)
		span.RecordError(err)
		span.SetStatus(codes.Error, "status changed")
		return nil, nil, err
	}

	// Step 5: Get and lock the wallet
	var balanceBefore int64
	now := time.Now()

	switch originalTx.WalletType {
	case models.WalletTypeAgentStake:
		w, err := s.walletRepo.GetByAgentIDForUpdate(ctx, dbTx, originalTx.WalletOwnerID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to lock agent wallet")
			return nil, nil, fmt.Errorf("failed to lock agent wallet: %w", err)
		}
		balanceBefore = w.Balance

		// Debit the wallet (reverse the credit) - ALLOW NEGATIVE BALANCE
		// Use totalReversalAmount to reverse both base and commission if applicable
		w.Balance -= totalReversalAmount
		w.LastTransactionAt = &now
		w.UpdatedAt = now

		if err = s.extendedWalletRepo.UpdateAgentWalletTx(ctx, dbTx, w); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to update agent wallet")
			return nil, nil, fmt.Errorf("failed to update agent wallet: %w", err)
		}

	case models.WalletTypeRetailerStake:
		w, err := s.walletRepo.GetRetailerStakeWalletForUpdate(ctx, dbTx, originalTx.WalletOwnerID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to lock retailer stake wallet")
			return nil, nil, fmt.Errorf("failed to lock retailer stake wallet: %w", err)
		}
		balanceBefore = w.Balance

		// Debit the wallet (reverse the credit) - ALLOW NEGATIVE BALANCE
		// Use totalReversalAmount to reverse both base and commission if applicable
		w.Balance -= totalReversalAmount
		w.LastTransactionAt = &now
		w.UpdatedAt = now

		if err = s.extendedWalletRepo.UpdateRetailerStakeWalletTx(ctx, dbTx, w); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to update retailer stake wallet")
			return nil, nil, fmt.Errorf("failed to update retailer stake wallet: %w", err)
		}

	case models.WalletTypeRetailerWinning:
		w, err := s.walletRepo.GetRetailerWinningWalletForUpdate(ctx, dbTx, originalTx.WalletOwnerID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to lock retailer winning wallet")
			return nil, nil, fmt.Errorf("failed to lock retailer winning wallet: %w", err)
		}
		balanceBefore = w.Balance

		// Debit the wallet (reverse the credit) - ALLOW NEGATIVE BALANCE
		// Use totalReversalAmount to reverse both base and commission if applicable
		w.Balance -= totalReversalAmount
		w.LastTransactionAt = &now
		w.UpdatedAt = now

		if err = s.extendedWalletRepo.UpdateRetailerWinningWalletTx(ctx, dbTx, w); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to update retailer winning wallet")
			return nil, nil, fmt.Errorf("failed to update retailer winning wallet: %w", err)
		}

	default:
		err := fmt.Errorf("unsupported wallet type for reversal: %s", originalTx.WalletType)
		span.RecordError(err)
		span.SetStatus(codes.Error, "unsupported wallet type")
		return nil, nil, err
	}

	// Calculate balanceAfter using totalReversalAmount (base + commission if applicable)
	balanceAfter := balanceBefore - totalReversalAmount
	isNegative := balanceAfter < 0

	span.SetAttributes(
		attribute.Int64("wallet.balance_before", balanceBefore),
		attribute.Int64("wallet.balance_after", balanceAfter),
		attribute.Bool("wallet.balance_is_negative", isNegative),
	)

	// Step 6: Create the reversal transaction (DEBIT for base transaction)
	reversalTxID := fmt.Sprintf("TXN-REV-%s", uuid.New().String())
	reversalDescription := fmt.Sprintf("Reversal of transaction %s - %s", originalTx.TransactionID, reason)

	reversalTx := &models.WalletTransaction{
		ID:              uuid.New(),
		TransactionID:   reversalTxID,
		WalletOwnerID:   originalTx.WalletOwnerID,
		WalletType:      originalTx.WalletType,
		TransactionType: models.TransactionTypeDebit, // Reversing a CREDIT means DEBIT
		Amount:          originalTx.Amount,           // Reversal amount for BASE transaction only
		BalanceBefore:   balanceBefore,
		BalanceAfter:    balanceBefore - originalTx.Amount, // After reversing BASE only
		Description:     &reversalDescription,
		Reference:       originalTx.Reference,
		Status:          models.TransactionStatusCompleted,
		Metadata: map[string]any{
			"reversal_of":    originalTx.ID.String(),
			"reversed_by_id": adminID.String(),
			"reversed_by":    adminName,
			"reason":         reason,
		},
		CreatedAt: now,
	}

	if err = s.transactionRepo.CreateTransactionTx(ctx, dbTx, reversalTx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create reversal transaction")
		return nil, nil, fmt.Errorf("failed to create reversal transaction: %w", err)
	}

	// Step 7: Mark original transaction as reversed
	if err = s.reversalRepo.MarkTransactionAsReversed(ctx, dbTx, originalTx.ID, reversalTx.ID, reason); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to mark transaction as reversed")
		return nil, nil, fmt.Errorf("failed to mark transaction as reversed: %w", err)
	}

	// Step 7.5: If this transaction has an associated commission, reverse it too
	var commissionReversalTx *models.WalletTransaction
	if hasCommission && commissionTx != nil {
		// Create reversal transaction for commission
		commissionReversalTxID := fmt.Sprintf("TXN-REV-%s-COMM", uuid.New().String())
		commissionReversalDescription := fmt.Sprintf("Reversal of commission %s - %s", commissionTx.TransactionID, reason)

		// Balance progression: balanceBefore - originalTx.Amount (after base reversal) - commissionTx.Amount (final)
		commissionBalanceBefore := balanceBefore - originalTx.Amount
		commissionBalanceAfter := balanceAfter // This is already balanceBefore - totalReversalAmount

		commissionReversalTx = &models.WalletTransaction{
			ID:              uuid.New(),
			TransactionID:   commissionReversalTxID,
			WalletOwnerID:   originalTx.WalletOwnerID,
			WalletType:      originalTx.WalletType,
			TransactionType: models.TransactionTypeDebit, // Reversing COMMISSION means DEBIT
			Amount:          commissionTx.Amount,
			BalanceBefore:   commissionBalanceBefore,
			BalanceAfter:    commissionBalanceAfter,
			Description:     &commissionReversalDescription,
			Reference:       commissionTx.Reference,
			Status:          models.TransactionStatusCompleted,
			Metadata: map[string]any{
				"reversal_of":            commissionTx.ID.String(),
				"base_reversal_tx_id":    reversalTx.ID.String(),
				"reversed_by_id":         adminID.String(),
				"reversed_by":            adminName,
				"reason":                 reason,
				"original_commission_tx": commissionTx.TransactionID,
			},
			CreatedAt: now,
		}

		if err = s.transactionRepo.CreateTransactionTx(ctx, dbTx, commissionReversalTx); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create commission reversal transaction")
			return nil, nil, fmt.Errorf("failed to create commission reversal transaction: %w", err)
		}

		// Mark commission transaction as reversed
		if err = s.reversalRepo.MarkTransactionAsReversed(ctx, dbTx, commissionTx.ID, commissionReversalTx.ID, reason); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to mark commission transaction as reversed")
			return nil, nil, fmt.Errorf("failed to mark commission transaction as reversed: %w", err)
		}

		// Create reversal audit for commission transaction
		commissionReversalAudit := &models.TransactionReversal{
			OriginalTransactionID: commissionTx.ID,
			ReversalTransactionID: &commissionReversalTx.ID,
			OriginalAmount:        commissionTx.Amount,
			WalletOwnerID:         originalTx.WalletOwnerID,
			WalletType:            originalTx.WalletType,
			Reason:                reason,
			ReversedBy:            adminID,
			ReversedByName:        &adminName,
			ReversedByEmail:       &adminEmail,
			ReversedAt:            now,
			Metadata: map[string]interface{}{
				"commission_reversal":       true,
				"base_reversal_tx_id":       reversalTx.ID.String(),
				"original_base_tx_id":       originalTx.ID.String(),
				"balance_before_reversal":   commissionBalanceBefore,
				"balance_after_reversal":    commissionBalanceAfter,
				"original_transaction_date": commissionTx.CreatedAt,
			},
		}

		if err = s.reversalRepo.CreateReversalAudit(ctx, dbTx, commissionReversalAudit); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create commission reversal audit")
			return nil, nil, fmt.Errorf("failed to create commission reversal audit: %w", err)
		}

		span.SetAttributes(
			attribute.String("commission_reversal.transaction_id", commissionReversalTx.ID.String()),
			attribute.Int64("commission_reversal.amount", commissionTx.Amount),
		)
	}

	// Step 8: Create reversal audit record
	reversalAudit := &models.TransactionReversal{
		OriginalTransactionID: originalTx.ID,
		ReversalTransactionID: &reversalTx.ID,
		OriginalAmount:        originalTx.Amount,
		WalletOwnerID:         originalTx.WalletOwnerID,
		WalletType:            originalTx.WalletType,
		Reason:                reason,
		ReversedBy:            adminID,
		ReversedByName:        &adminName,
		ReversedByEmail:       &adminEmail,
		ReversedAt:            now,
		Metadata: map[string]interface{}{
			"balance_before_reversal":   balanceBefore,
			"balance_after_reversal":    balanceAfter,
			"balance_is_negative":       isNegative,
			"original_transaction_date": originalTx.CreatedAt,
		},
	}

	if err = s.reversalRepo.CreateReversalAudit(ctx, dbTx, reversalAudit); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create reversal audit")
		return nil, nil, fmt.Errorf("failed to create reversal audit: %w", err)
	}

	// Step 9: Commit the database transaction
	if err = dbTx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to commit transaction")
		return nil, nil, fmt.Errorf("failed to commit database transaction: %w", err)
	}

	// Step 10: Post-reversal actions (asynchronous)
	// Invalidate caches
	cacheKeys := []string{
		fmt.Sprintf("wallet:%s:%s:balance", string(originalTx.WalletType), originalTx.WalletOwnerID.String()),
	}
	for _, key := range cacheKeys {
		s.cache.Del(ctx, key)
	}

	// TODO: Emit Kafka event when Publisher is fully implemented
	// if s.eventPublisher != nil {
	// 	event := map[string]any{
	// 		"event_type": "transaction.reversed",
	// 		"original_transaction_id": originalTx.ID.String(),
	// 		"reversal_transaction_id": reversalTx.ID.String(),
	// 		"wallet_owner_id": originalTx.WalletOwnerID.String(),
	// 		"wallet_type": string(originalTx.WalletType),
	// 		"amount": originalTx.Amount,
	// 		"reason": reason,
	// 		"balance_is_negative": isNegative,
	// 		"reversed_by": map[string]string{
	// 			"admin_id": adminID.String(),
	// 			"admin_name": adminName,
	// 			"admin_email": adminEmail,
	// 		},
	// 		"timestamp": now,
	// 	}
	// 	_ = s.eventPublisher.PublishAsync("wallet.events", event)
	// }

	span.SetAttributes(
		attribute.String("reversal.transaction_id", reversalTx.ID.String()),
		attribute.Bool("reversal.success", true),
	)

	span.SetStatus(codes.Ok, "transaction reversed successfully")

	// Update original transaction status in memory for return
	originalTx.Status = models.TransactionStatusReversed
	originalTx.ReversedAt = &now

	return originalTx, reversalTx, nil
}

func (s *walletService) PlaceHoldOnWallet(ctx context.Context, retailerID uuid.UUID, placedBy uuid.UUID, reason string, expiresAt time.Time) error {
	// Validate input parameters
	if retailerID == uuid.Nil || placedBy == uuid.Nil {
		return fmt.Errorf("invalid input parameters")
	}

	// find wallet belonging to retailer
	wallet, err := s.walletRepo.GetRetailerWinningWallet(ctx, retailerID)
	if err != nil {
		return fmt.Errorf("failed to find retailer wallet: %w", err)
	}

	// check if wallet is already on hold
	hold, err := s.walletRepo.GetWalletHoldByID(ctx, wallet.ID)
	if err != nil {
		return fmt.Errorf("failed to check wallet hold status: %w", err)
	}
	if hold != nil && hold.Status == models.WalletHoldStatusActive {
		// GetWalletHoldByID returns nil if no rows were found
		return fmt.Errorf("wallet is already on hold")
	}

	// place hold on wallet
	err = s.walletRepo.PlaceHoldOnWallet(ctx, wallet.ID, wallet.RetailerID, placedBy, reason, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to place hold on wallet: %w", err)
	}

	return nil
}

func (s *walletService) GetHoldOnWallet(ctx context.Context, holdID uuid.UUID) (*models.RetailerWinningWalletHold, error) {
	if holdID == uuid.Nil {
		return nil, fmt.Errorf("invalid hold id")
	}
	hold, err := s.walletRepo.GetWalletHoldByID(ctx, holdID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet hold: %w", err)
	}
	return hold, nil
}

func (s *walletService) GetHoldByRetailer(ctx context.Context, retailerID uuid.UUID) (*models.RetailerWinningWalletHold, error) {
	if retailerID == uuid.Nil {
		return nil, fmt.Errorf("invalid retailer id")
	}
	hold, err := s.walletRepo.GetWalletHoldByRetailerID(ctx, retailerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get retailer wallet hold: %w", err)
	}
	return hold, nil
}

func (s *walletService) ReleaseHoldOnWallet(ctx context.Context, holdID uuid.UUID, retailerID uuid.UUID, releasedBy uuid.UUID) error {
	if holdID == uuid.Nil || retailerID == uuid.Nil {
		return fmt.Errorf("invalid input parameters")
	}
	// Release by wallet (holdID maps to wallet_id in current schema)
	if err := s.walletRepo.ReleaseHoldOnWallet(ctx, holdID, retailerID, releasedBy); err != nil {
		return fmt.Errorf("failed to release wallet hold: %w", err)
	}
	return nil
}

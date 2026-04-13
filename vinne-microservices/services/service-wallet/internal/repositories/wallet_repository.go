package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/randco/service-wallet/internal/models"
	"github.com/redis/go-redis/v9"
)

// WalletRepository defines the interface for wallet data operations
type WalletRepository interface {
	// Agent wallet operations
	Create(ctx context.Context, wallet *models.AgentStakeWallet) error
	GetByAgentID(ctx context.Context, agentID uuid.UUID) (*models.AgentStakeWallet, error)
	GetByAgentIDForUpdate(ctx context.Context, tx *sql.Tx, agentID uuid.UUID) (*models.AgentStakeWallet, error)
	Update(ctx context.Context, wallet *models.AgentStakeWallet) error

	// Retailer stake wallet operations
	CreateRetailerStakeWallet(ctx context.Context, wallet *models.RetailerStakeWallet) error
	GetRetailerStakeWallet(ctx context.Context, retailerID uuid.UUID) (*models.RetailerStakeWallet, error)
	GetRetailerStakeWalletForUpdate(ctx context.Context, tx *sql.Tx, retailerID uuid.UUID) (*models.RetailerStakeWallet, error)
	UpdateRetailerStakeWallet(ctx context.Context, wallet *models.RetailerStakeWallet) error

	// Retailer winning wallet operations
	CreateRetailerWinningWallet(ctx context.Context, wallet *models.RetailerWinningWallet) error
	GetRetailerWinningWallet(ctx context.Context, retailerID uuid.UUID) (*models.RetailerWinningWallet, error)
	PlaceHoldOnWallet(ctx context.Context, walletID uuid.UUID, retailerID uuid.UUID, placedBy uuid.UUID, reason string, expiresAt time.Time) error
	ReleaseHoldOnWallet(ctx context.Context, walletID uuid.UUID, retailerID uuid.UUID, placedBy uuid.UUID) error
	GetWalletHoldByID(ctx context.Context, walletID uuid.UUID) (*models.RetailerWinningWalletHold, error)
	GetWalletHoldByRetailerID(ctx context.Context, retailerID uuid.UUID) (*models.RetailerWinningWalletHold, error)
	ListWalletHolds(ctx context.Context) ([]*models.RetailerWinningWalletHold, error)

	// Player wallet operations
	CreatePlayerWallet(ctx context.Context, wallet *models.PlayerWallet) error
	GetPlayerWallet(ctx context.Context, playerID uuid.UUID) (*models.PlayerWallet, error)
	UpdatePlayerWallet(ctx context.Context, wallet *models.PlayerWallet) error
	GetRetailerWinningWalletForUpdate(ctx context.Context, tx *sql.Tx, retailerID uuid.UUID) (*models.RetailerWinningWallet, error)

	// Generic wallet operations (for reservation cleanup job)
	GetByOwnerAndType(ctx context.Context, ownerID uuid.UUID, walletType models.WalletType) (interface{}, error)
	ReleaseReservation(ctx context.Context, walletID uuid.UUID, walletType models.WalletType, reservationID string, amount int64) error
}

// ExtendedWalletRepository for additional operations
type ExtendedWalletRepository interface {
	UpdateAgentWalletTx(ctx context.Context, tx *sql.Tx, wallet *models.AgentStakeWallet) error
	UpdateRetailerWinningWallet(ctx context.Context, wallet *models.RetailerWinningWallet) error
	UpdateRetailerWinningWalletTx(ctx context.Context, tx *sql.Tx, wallet *models.RetailerWinningWallet) error
	UpdateRetailerStakeWalletTx(ctx context.Context, tx *sql.Tx, wallet *models.RetailerStakeWallet) error
	LockWallet(ctx context.Context, lock *models.WalletLock) error
	UnlockWallet(ctx context.Context, walletID uuid.UUID) error
}

// WalletTransactionRepository defines the interface for wallet transaction operations
type WalletTransactionRepository interface {
	CreateTransaction(ctx context.Context, tx *models.WalletTransaction) error
	CreateTransactionTx(ctx context.Context, dbTx *sql.Tx, tx *models.WalletTransaction) error
	GetTransaction(ctx context.Context, transactionID string) (*models.WalletTransaction, error)
	GetTransactionByIdempotencyKey(ctx context.Context, key string) (*models.WalletTransaction, error)
	UpdateTransactionStatus(ctx context.Context, transactionID string, status models.TransactionStatus) error
	GetTransactionHistory(ctx context.Context, walletOwnerID uuid.UUID, walletType models.WalletType, limit, offset int) ([]*models.WalletTransaction, error)
	CountTransactions(ctx context.Context, walletOwnerID uuid.UUID, walletType models.WalletType) (int, error)
	CreateTransfer(ctx context.Context, transfer *models.WalletTransfer) error
}

// TransactionReversalRepository defines the interface for transaction reversal operations
type TransactionReversalRepository interface {
	GetTransactionByID(ctx context.Context, txID uuid.UUID) (*models.WalletTransaction, error)
	GetTransactionByTransactionID(ctx context.Context, transactionID string) (*models.WalletTransaction, error)
	GetTransactionForReversalWithLock(ctx context.Context, dbTx *sql.Tx, txID uuid.UUID) (*models.WalletTransaction, error)
	MarkTransactionAsReversed(ctx context.Context, dbTx *sql.Tx, txID uuid.UUID, reversalTxID uuid.UUID, reason string) error
	CreateReversalAudit(ctx context.Context, dbTx *sql.Tx, reversal *models.TransactionReversal) error
	GetReversalByOriginalTransaction(ctx context.Context, txID uuid.UUID) (*models.TransactionReversal, error)
}

// AdminTransactionFilters defines filters for admin transaction queries
type AdminTransactionFilters struct {
	TransactionTypes []models.TransactionType
	WalletTypes      []models.WalletType
	Statuses         []models.TransactionStatus
	StartDate        *string // ISO 8601 timestamp
	EndDate          *string // ISO 8601 timestamp
	SearchTerm       *string
	Page             int
	PageSize         int
	SortBy           string // Field to sort by (default: "created_at")
	SortOrder        string // "asc" or "desc" (default: "desc")
}

// TransactionStatistics holds aggregated transaction statistics
type TransactionStatistics struct {
	TotalVolume     int64 // Sum of absolute values of all amounts
	TotalCredits    int64 // Sum of all credit transaction amounts
	TotalDebits     int64 // Sum of all debit transaction amounts (absolute value)
	PendingAmount   int64 // Sum of all pending transaction amounts (absolute value)
	PendingCount    int   // Count of pending transactions
	CompletedCount  int   // Count of completed transactions
	FailedCount     int   // Count of failed transactions
	CreditCount     int   // Count of credit transactions
	DebitCount      int   // Count of debit transactions
	TransferCount   int   // Count of transfer transactions
	CommissionCount int   // Count of commission transactions
	PayoutCount     int   // Count of payout transactions
}

// AdminTransactionRepository defines the interface for admin-level transaction operations
type AdminTransactionRepository interface {
	GetAllTransactions(ctx context.Context, filters AdminTransactionFilters) ([]*models.WalletTransaction, int, error)
	GetTransactionStatistics(ctx context.Context, filters AdminTransactionFilters) (*TransactionStatistics, error)
}

// walletRepository implements WalletRepository and ExtendedWalletRepository interfaces
type walletRepository struct {
	db    *sql.DB
	cache *redis.Client
}

// NewWalletRepository creates a new instance of WalletRepository
func NewWalletRepository(db *sql.DB) WalletRepository {
	return &walletRepository{
		db: db,
	}
}

// NewExtendedWalletRepository creates a new instance of ExtendedWalletRepository
func NewExtendedWalletRepository(db *sql.DB) ExtendedWalletRepository {
	return &walletRepository{
		db: db,
	}
}

// Create creates a new agent wallet
func (r *walletRepository) Create(ctx context.Context, wallet *models.AgentStakeWallet) error {
	query := `
		INSERT INTO agent_stake_wallets (
			id, agent_id, balance, pending_balance, available_balance,
			currency, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING created_at, updated_at`

	wallet.ID = uuid.New()
	err := r.db.QueryRowContext(ctx, query,
		wallet.ID,
		wallet.AgentID,
		wallet.Balance,
		wallet.PendingBalance,
		wallet.AvailableBalance,
		wallet.Currency,
		wallet.Status,
	).Scan(&wallet.CreatedAt, &wallet.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create agent wallet: %w", err)
	}

	// Invalidate cache
	r.invalidateWalletCache(ctx, wallet.AgentID.String())

	return nil
}

// GetByAgentID retrieves an agent's wallet by agent ID
func (r *walletRepository) GetByAgentID(ctx context.Context, agentID uuid.UUID) (*models.AgentStakeWallet, error) {
	query := `
		SELECT id, agent_id, balance, pending_balance, available_balance,
			   currency, status, last_transaction_at, created_at, updated_at
		FROM agent_stake_wallets
		WHERE agent_id = $1`

	wallet := &models.AgentStakeWallet{}
	err := r.db.QueryRowContext(ctx, query, agentID).Scan(
		&wallet.ID,
		&wallet.AgentID,
		&wallet.Balance,
		&wallet.PendingBalance,
		&wallet.AvailableBalance,
		&wallet.Currency,
		&wallet.Status,
		&wallet.LastTransactionAt,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return wallet, nil
}

// Update updates an agent wallet
func (r *walletRepository) Update(ctx context.Context, wallet *models.AgentStakeWallet) error {
	query := `
		UPDATE agent_stake_wallets
		SET balance = $1,
			pending_balance = $2,
			available_balance = $3,
			status = $4,
			last_transaction_at = $5,
			updated_at = NOW()
		WHERE agent_id = $6`

	result, err := r.db.ExecContext(ctx, query,
		wallet.Balance,
		wallet.PendingBalance,
		wallet.AvailableBalance,
		wallet.Status,
		wallet.LastTransactionAt,
		wallet.AgentID)
	if err != nil {
		return fmt.Errorf("failed to update agent wallet balance: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	// Invalidate cache
	r.invalidateWalletCache(ctx, wallet.AgentID.String())

	return nil
}

// GetByAgentIDForUpdate retrieves an agent's wallet by agent ID with row-level lock (FOR UPDATE)
func (r *walletRepository) GetByAgentIDForUpdate(ctx context.Context, tx *sql.Tx, agentID uuid.UUID) (*models.AgentStakeWallet, error) {
	query := `
		SELECT id, agent_id, balance, pending_balance, available_balance,
			   currency, status, last_transaction_at, created_at, updated_at
		FROM agent_stake_wallets
		WHERE agent_id = $1
		FOR UPDATE`

	wallet := &models.AgentStakeWallet{}
	err := tx.QueryRowContext(ctx, query, agentID).Scan(
		&wallet.ID,
		&wallet.AgentID,
		&wallet.Balance,
		&wallet.PendingBalance,
		&wallet.AvailableBalance,
		&wallet.Currency,
		&wallet.Status,
		&wallet.LastTransactionAt,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return wallet, nil
}

// UpdateAgentWalletTx updates an agent wallet within a transaction
func (r *walletRepository) UpdateAgentWalletTx(ctx context.Context, tx *sql.Tx, wallet *models.AgentStakeWallet) error {
	query := `
		UPDATE agent_stake_wallets
		SET balance = $1,
			pending_balance = $2,
			available_balance = $3,
			status = $4,
			last_transaction_at = $5,
			updated_at = NOW()
		WHERE agent_id = $6`

	result, err := tx.ExecContext(ctx, query,
		wallet.Balance,
		wallet.PendingBalance,
		wallet.AvailableBalance,
		wallet.Status,
		wallet.LastTransactionAt,
		wallet.AgentID)
	if err != nil {
		return fmt.Errorf("failed to update agent wallet balance: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// CreateRetailerStakeWallet creates a new retailer stake wallet
func (r *walletRepository) CreateRetailerStakeWallet(ctx context.Context, wallet *models.RetailerStakeWallet) error {
	query := `
		INSERT INTO retailer_stake_wallets (
			id, retailer_id, balance, pending_balance, available_balance,
			currency, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING created_at, updated_at`

	wallet.ID = uuid.New()
	err := r.db.QueryRowContext(ctx, query,
		wallet.ID,
		wallet.RetailerID,
		wallet.Balance,
		wallet.PendingBalance,
		wallet.AvailableBalance,
		wallet.Currency,
		wallet.Status,
	).Scan(&wallet.CreatedAt, &wallet.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create retailer stake wallet: %w", err)
	}

	// Invalidate cache
	r.invalidateWalletCache(ctx, wallet.RetailerID.String())

	return nil
}

// GetRetailerStakeWallet retrieves a retailer's stake wallet
func (r *walletRepository) GetRetailerStakeWallet(ctx context.Context, retailerID uuid.UUID) (*models.RetailerStakeWallet, error) {
	query := `
		SELECT id, retailer_id, balance, pending_balance, available_balance,
			   currency, status, last_transaction_at, created_at, updated_at
		FROM retailer_stake_wallets
		WHERE retailer_id = $1`

	wallet := &models.RetailerStakeWallet{}
	err := r.db.QueryRowContext(ctx, query, retailerID).Scan(
		&wallet.ID,
		&wallet.RetailerID,
		&wallet.Balance,
		&wallet.PendingBalance,
		&wallet.AvailableBalance,
		&wallet.Currency,
		&wallet.Status,
		&wallet.LastTransactionAt,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return wallet, nil
}

// UpdateRetailerStakeWallet updates a retailer stake wallet
func (r *walletRepository) UpdateRetailerStakeWallet(ctx context.Context, wallet *models.RetailerStakeWallet) error {
	query := `
		UPDATE retailer_stake_wallets
		SET balance = $1,
			pending_balance = $2,
			available_balance = $3,
			status = $4,
			last_transaction_at = $5,
			updated_at = NOW()
		WHERE retailer_id = $6`

	result, err := r.db.ExecContext(ctx, query,
		wallet.Balance,
		wallet.PendingBalance,
		wallet.AvailableBalance,
		wallet.Status,
		wallet.LastTransactionAt,
		wallet.RetailerID)
	if err != nil {
		return fmt.Errorf("failed to update retailer stake wallet balance: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	// Invalidate cache
	r.invalidateWalletCache(ctx, wallet.RetailerID.String())

	return nil
}

// CreateRetailerWinningWallet creates a new retailer winning wallet
func (r *walletRepository) CreateRetailerWinningWallet(ctx context.Context, wallet *models.RetailerWinningWallet) error {
	query := `
		INSERT INTO retailer_winning_wallets (
			id, retailer_id, balance, pending_balance, available_balance,
			currency, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING created_at, updated_at`

	wallet.ID = uuid.New()
	err := r.db.QueryRowContext(ctx, query,
		wallet.ID,
		wallet.RetailerID,
		wallet.Balance,
		wallet.PendingBalance,
		wallet.AvailableBalance,
		wallet.Currency,
		wallet.Status,
	).Scan(&wallet.CreatedAt, &wallet.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create retailer winning wallet: %w", err)
	}

	// Invalidate cache
	r.invalidateWalletCache(ctx, wallet.RetailerID.String())

	return nil
}

// GetRetailerWinningWallet retrieves a retailer's winning wallet
func (r *walletRepository) GetRetailerWinningWallet(ctx context.Context, retailerID uuid.UUID) (*models.RetailerWinningWallet, error) {
	query := `
		SELECT id, retailer_id, balance, pending_balance, available_balance,
			   currency, status, last_transaction_at, created_at, updated_at
		FROM retailer_winning_wallets
		WHERE retailer_id = $1`

	wallet := &models.RetailerWinningWallet{}
	err := r.db.QueryRowContext(ctx, query, retailerID).Scan(
		&wallet.ID,
		&wallet.RetailerID,
		&wallet.Balance,
		&wallet.PendingBalance,
		&wallet.AvailableBalance,
		&wallet.Currency,
		&wallet.Status,
		&wallet.LastTransactionAt,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return wallet, nil
}

// PlaceHoldOnWallet places a hold on a retailer's wallet
func (r *walletRepository) PlaceHoldOnWallet(ctx context.Context, walletID uuid.UUID, retailerID uuid.UUID, placedBy uuid.UUID, reason string, expiresAt time.Time) error {
	query := `
		INSERT INTO wallet_holds (
    wallet_id, retailer_id, placed_by, reason, status, created_at, updated_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW(), $6)
		ON CONFLICT (wallet_id)
		DO UPDATE SET
			retailer_id = EXCLUDED.retailer_id,
			placed_by   = EXCLUDED.placed_by,
			reason      = EXCLUDED.reason,
			status      = EXCLUDED.status,
			updated_at  = NOW(),
			expires_at  = $6;

	`
	_, err := r.db.ExecContext(ctx, query,
		walletID,
		retailerID,
		placedBy,
		reason,
		models.WalletHoldStatusActive,
		expiresAt,
	)

	if err != nil {
		return fmt.Errorf("failed to place hold on wallet: %w", err)
	}

	return nil
}

// PlaceHoldOnWallet places a hold on a retailer's wallet
func (r *walletRepository) ReleaseHoldOnWallet(ctx context.Context, walletID uuid.UUID, retailerID uuid.UUID, placedBy uuid.UUID) error {
	query := `
		UPDATE wallet_holds
		SET 
			status = $4, 
			updated_at = NOW(),
			expires_at = null
		WHERE wallet_id = $1 AND retailer_id = $2 AND status = $3;
	`
	_, err := r.db.ExecContext(ctx, query, walletID, retailerID, models.WalletHoldStatusActive, models.WalletHoldStatusReleased)
	if err != nil {
		return fmt.Errorf("failed to release hold on wallet: %w", err)
	}

	return nil
}

func (r *walletRepository) GetWalletHoldByID(ctx context.Context, walletID uuid.UUID) (*models.RetailerWinningWalletHold, error) {
	query := `
		SELECT wallet_id, retailer_id, placed_by, reason, status,
			expires_at, created_at, updated_at
		FROM wallet_holds
		WHERE wallet_id = $1`

	walletHold := &models.RetailerWinningWalletHold{}
	err := r.db.QueryRowContext(ctx, query, walletID).Scan(
		&walletHold.WalletID,
		&walletHold.RetailerID,
		&walletHold.PlacedBy,
		&walletHold.Reason,
		&walletHold.Status,
		&walletHold.ExpiresAt,
		&walletHold.CreatedAt,
		&walletHold.UpdatedAt,
	)

	if err != nil {
		// Return nil instead of error when no rows are found
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return walletHold, nil
}

func (r *walletRepository) GetWalletHoldByRetailerID(ctx context.Context, retailerID uuid.UUID) (*models.RetailerWinningWalletHold, error) {
	query := `
		SELECT wallet_id, retailer_id, placed_by, reason, status,
			expires_at, created_at, updated_at
		FROM wallet_holds
		WHERE retailer_id = $1`

	walletHold := &models.RetailerWinningWalletHold{}
	err := r.db.QueryRowContext(ctx, query, retailerID).Scan(
		&walletHold.WalletID,
		&walletHold.RetailerID,
		&walletHold.PlacedBy,
		&walletHold.Reason,
		&walletHold.Status,
		&walletHold.ExpiresAt,
		&walletHold.CreatedAt,
		&walletHold.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return walletHold, nil
}

func (r *walletRepository) ListWalletHolds(ctx context.Context) ([]*models.RetailerWinningWalletHold, error) {
	query := `
		SELECT wallet_id, retailer_id, placed_by, reason, status,
			expires_at, created_at, updated_at
		FROM wallet_holds
		WHERE status = $1`

	rows, err := r.db.QueryContext(ctx, query, models.WalletHoldStatusActive)
	if err != nil {
		return nil, fmt.Errorf("failed to list wallet holds: %w", err)
	}
	defer rows.Close()

	var walletHolds []*models.RetailerWinningWalletHold
	for rows.Next() {
		walletHold := &models.RetailerWinningWalletHold{}
		if err := rows.Scan(
			&walletHold.WalletID,
			&walletHold.RetailerID,
			&walletHold.PlacedBy,
			&walletHold.Reason,
			&walletHold.Status,
			&walletHold.ExpiresAt,
			&walletHold.CreatedAt,
			&walletHold.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan wallet hold: %w", err)
		}
		walletHolds = append(walletHolds, walletHold)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate over wallet holds: %w", err)
	}

	return walletHolds, nil
}

// GetRetailerStakeWalletForUpdate retrieves a retailer's stake wallet with row-level lock
func (r *walletRepository) GetRetailerStakeWalletForUpdate(ctx context.Context, tx *sql.Tx, retailerID uuid.UUID) (*models.RetailerStakeWallet, error) {
	query := `
		SELECT id, retailer_id, balance, pending_balance, available_balance,
			   currency, status, last_transaction_at, created_at, updated_at
		FROM retailer_stake_wallets
		WHERE retailer_id = $1
		FOR UPDATE`

	wallet := &models.RetailerStakeWallet{}
	err := tx.QueryRowContext(ctx, query, retailerID).Scan(
		&wallet.ID,
		&wallet.RetailerID,
		&wallet.Balance,
		&wallet.PendingBalance,
		&wallet.AvailableBalance,
		&wallet.Currency,
		&wallet.Status,
		&wallet.LastTransactionAt,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return wallet, nil
}

// GetRetailerWinningWalletForUpdate retrieves a retailer's winning wallet with row-level lock
func (r *walletRepository) GetRetailerWinningWalletForUpdate(ctx context.Context, tx *sql.Tx, retailerID uuid.UUID) (*models.RetailerWinningWallet, error) {
	query := `
		SELECT id, retailer_id, balance, pending_balance, available_balance,
			   currency, status, last_transaction_at, created_at, updated_at
		FROM retailer_winning_wallets
		WHERE retailer_id = $1
		FOR UPDATE`

	wallet := &models.RetailerWinningWallet{}
	err := tx.QueryRowContext(ctx, query, retailerID).Scan(
		&wallet.ID,
		&wallet.RetailerID,
		&wallet.Balance,
		&wallet.PendingBalance,
		&wallet.AvailableBalance,
		&wallet.Currency,
		&wallet.Status,
		&wallet.LastTransactionAt,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return wallet, nil
}

// UpdateRetailerWinningWallet updates a retailer winning wallet
func (r *walletRepository) UpdateRetailerWinningWallet(ctx context.Context, wallet *models.RetailerWinningWallet) error {
	query := `
		UPDATE retailer_winning_wallets
		SET balance = $1,
			pending_balance = $2,
			available_balance = $3,
			status = $4,
			last_transaction_at = $5,
			updated_at = NOW()
		WHERE retailer_id = $6`

	result, err := r.db.ExecContext(ctx, query,
		wallet.Balance,
		wallet.PendingBalance,
		wallet.AvailableBalance,
		wallet.Status,
		wallet.LastTransactionAt,
		wallet.RetailerID)
	if err != nil {
		return fmt.Errorf("failed to update retailer winning wallet: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	// Invalidate cache
	r.invalidateWalletCache(ctx, wallet.RetailerID.String())

	return nil
}

// Player wallet operations

func (r *walletRepository) CreatePlayerWallet(ctx context.Context, wallet *models.PlayerWallet) error {
	query := `
		INSERT INTO player_wallets (
			id, player_id, balance, pending_balance, available_balance,
			currency, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING created_at, updated_at`

	wallet.ID = uuid.New()
	err := r.db.QueryRowContext(ctx, query,
		wallet.ID,
		wallet.PlayerID,
		wallet.Balance,
		wallet.PendingBalance,
		wallet.AvailableBalance,
		wallet.Currency,
		wallet.Status,
	).Scan(&wallet.CreatedAt, &wallet.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create player wallet: %w", err)
	}

	r.invalidateWalletCache(ctx, wallet.PlayerID.String())

	return nil
}

func (r *walletRepository) GetPlayerWallet(ctx context.Context, playerID uuid.UUID) (*models.PlayerWallet, error) {
	query := `
		SELECT id, player_id, balance, pending_balance, available_balance,
			   currency, status, last_transaction_at, created_at, updated_at
		FROM player_wallets
		WHERE player_id = $1`

	wallet := &models.PlayerWallet{}
	err := r.db.QueryRowContext(ctx, query, playerID).Scan(
		&wallet.ID,
		&wallet.PlayerID,
		&wallet.Balance,
		&wallet.PendingBalance,
		&wallet.AvailableBalance,
		&wallet.Currency,
		&wallet.Status,
		&wallet.LastTransactionAt,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return wallet, nil
}

func (r *walletRepository) UpdatePlayerWallet(ctx context.Context, wallet *models.PlayerWallet) error {
	query := `
		UPDATE player_wallets
		SET balance = $1,
			pending_balance = $2,
			available_balance = $3,
			status = $4,
			last_transaction_at = $5,
			updated_at = NOW()
		WHERE player_id = $6`

	result, err := r.db.ExecContext(ctx, query,
		wallet.Balance,
		wallet.PendingBalance,
		wallet.AvailableBalance,
		wallet.Status,
		wallet.LastTransactionAt,
		wallet.PlayerID)
	if err != nil {
		return fmt.Errorf("failed to update player wallet: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	r.invalidateWalletCache(ctx, wallet.PlayerID.String())

	return nil
}

// GetByOwnerAndType retrieves a wallet by owner ID and wallet type
// Returns interface{} that can be type-asserted to the specific wallet type
func (r *walletRepository) GetByOwnerAndType(ctx context.Context, ownerID uuid.UUID, walletType models.WalletType) (interface{}, error) {
	switch walletType {
	case models.WalletTypeAgentStake:
		return r.GetByAgentID(ctx, ownerID)
	case models.WalletTypeRetailerStake:
		return r.GetRetailerStakeWallet(ctx, ownerID)
	case models.WalletTypeRetailerWinning:
		return r.GetRetailerWinningWallet(ctx, ownerID)
	case models.WalletTypePlayerWallet:
		return r.GetPlayerWallet(ctx, ownerID)
	default:
		return nil, fmt.Errorf("unsupported wallet type: %s", walletType)
	}
}

// ReleaseReservation releases a reservation and restores funds to available balance
// This moves the amount from pending_balance back to available_balance
func (r *walletRepository) ReleaseReservation(ctx context.Context, walletID uuid.UUID, walletType models.WalletType, reservationID string, amount int64) error {
	var tableName string
	switch walletType {
	case models.WalletTypeAgentStake:
		tableName = "agent_stake_wallets"
	case models.WalletTypeRetailerStake:
		tableName = "retailer_stake_wallets"
	case models.WalletTypeRetailerWinning:
		tableName = "retailer_winning_wallets"
	case models.WalletTypePlayerWallet:
		tableName = "player_wallets"
	default:
		return fmt.Errorf("unsupported wallet type: %s", walletType)
	}

	query := fmt.Sprintf(`
		UPDATE %s
		SET pending_balance = pending_balance - $1,
			available_balance = available_balance + $1,
			updated_at = NOW()
		WHERE id = $2 AND pending_balance >= $1`, tableName)

	result, err := r.db.ExecContext(ctx, query, amount, walletID)
	if err != nil {
		return fmt.Errorf("failed to release reservation: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("wallet not found or insufficient pending balance: wallet_id=%s reservation_id=%s amount=%d",
			walletID.String(), reservationID, amount)
	}

	log.Printf("Released reservation: wallet_id=%s wallet_type=%s reservation_id=%s amount=%d",
		walletID.String(), walletType, reservationID, amount)

	return nil
}

// UpdateRetailerWinningWalletTx updates a retailer winning wallet within a transaction
func (r *walletRepository) UpdateRetailerWinningWalletTx(ctx context.Context, tx *sql.Tx, wallet *models.RetailerWinningWallet) error {
	query := `
		UPDATE retailer_winning_wallets
		SET balance = $1,
			pending_balance = $2,
			available_balance = $3,
			status = $4,
			last_transaction_at = $5,
			updated_at = NOW()
		WHERE retailer_id = $6`

	result, err := tx.ExecContext(ctx, query,
		wallet.Balance,
		wallet.PendingBalance,
		wallet.AvailableBalance,
		wallet.Status,
		wallet.LastTransactionAt,
		wallet.RetailerID)
	if err != nil {
		return fmt.Errorf("failed to update retailer winning wallet: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// UpdateRetailerStakeWalletTx updates a retailer stake wallet within a transaction
func (r *walletRepository) UpdateRetailerStakeWalletTx(ctx context.Context, tx *sql.Tx, wallet *models.RetailerStakeWallet) error {
	query := `
		UPDATE retailer_stake_wallets
		SET balance = $1,
			pending_balance = $2,
			available_balance = $3,
			status = $4,
			last_transaction_at = $5,
			updated_at = NOW()
		WHERE retailer_id = $6`

	result, err := tx.ExecContext(ctx, query,
		wallet.Balance,
		wallet.PendingBalance,
		wallet.AvailableBalance,
		wallet.Status,
		wallet.LastTransactionAt,
		wallet.RetailerID)
	if err != nil {
		return fmt.Errorf("failed to update retailer stake wallet: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// LockWallet locks a wallet
func (r *walletRepository) LockWallet(ctx context.Context, lock *models.WalletLock) error {
	// First update wallet status to locked based on wallet type
	var updateQuery string
	switch lock.WalletType {
	case models.WalletTypeAgentStake:
		updateQuery = `UPDATE agent_stake_wallets SET status = $1 WHERE id = $2`
	case models.WalletTypeRetailerStake:
		updateQuery = `UPDATE retailer_stake_wallets SET status = $1 WHERE id = $2`
	case models.WalletTypeRetailerWinning:
		updateQuery = `UPDATE retailer_winning_wallets SET status = $1 WHERE id = $2`
	default:
		return fmt.Errorf("invalid wallet type: %s", lock.WalletType)
	}

	_, err := r.db.ExecContext(ctx, updateQuery, models.WalletStatusLocked, lock.WalletID)
	if err != nil {
		return fmt.Errorf("failed to lock wallet: %w", err)
	}

	// Insert lock record
	insertQuery := `
		INSERT INTO wallet_locks (id, wallet_id, wallet_type, lock_reason, locked_by, locked_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	lock.ID = uuid.New()
	_, err = r.db.ExecContext(ctx, insertQuery,
		lock.ID,
		lock.WalletID,
		lock.WalletType,
		lock.LockReason,
		lock.LockedBy,
		lock.LockedAt,
		lock.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create lock record: %w", err)
	}

	return nil
}

// UnlockWallet unlocks a wallet
func (r *walletRepository) UnlockWallet(ctx context.Context, walletID uuid.UUID) error {
	// First get the wallet type from the lock record
	var walletType models.WalletType
	err := r.db.QueryRowContext(ctx,
		`SELECT wallet_type FROM wallet_locks WHERE wallet_id = $1 AND released_at IS NULL LIMIT 1`,
		walletID,
	).Scan(&walletType)
	if err != nil {
		return fmt.Errorf("failed to get wallet lock info: %w", err)
	}

	// Update wallet status to active based on wallet type
	var updateQuery string
	switch walletType {
	case models.WalletTypeAgentStake:
		updateQuery = `UPDATE agent_stake_wallets SET status = $1 WHERE id = $2`
	case models.WalletTypeRetailerStake:
		updateQuery = `UPDATE retailer_stake_wallets SET status = $1 WHERE id = $2`
	case models.WalletTypeRetailerWinning:
		updateQuery = `UPDATE retailer_winning_wallets SET status = $1 WHERE id = $2`
	default:
		return fmt.Errorf("invalid wallet type: %s", walletType)
	}

	_, err = r.db.ExecContext(ctx, updateQuery, models.WalletStatusActive, walletID)
	if err != nil {
		return fmt.Errorf("failed to unlock wallet: %w", err)
	}

	// Update lock record
	lockQuery := `
		UPDATE wallet_locks 
		SET released_at = NOW()
		WHERE wallet_id = $1 AND released_at IS NULL`

	_, err = r.db.ExecContext(ctx, lockQuery, walletID)
	if err != nil {
		return fmt.Errorf("failed to update lock record: %w", err)
	}

	return nil
}

// invalidateWalletCache invalidates wallet cache
func (r *walletRepository) invalidateWalletCache(ctx context.Context, ownerID string) {
	if r.cache != nil {
		cacheKey := fmt.Sprintf("wallet:%s", ownerID)
		r.cache.Del(ctx, cacheKey)
	}
}

// walletTransactionRepository implements WalletTransactionRepository interface
type walletTransactionRepository struct {
	db    *sql.DB
	cache *redis.Client
}

// NewWalletTransactionRepository creates a new instance of WalletTransactionRepository
func NewWalletTransactionRepository(db *sql.DB, cache *redis.Client) WalletTransactionRepository {
	return &walletTransactionRepository{
		db:    db,
		cache: cache,
	}
}

// CreateTransaction creates a new wallet transaction
func (r *walletTransactionRepository) CreateTransaction(ctx context.Context, tx *models.WalletTransaction) error {
	query := `
		INSERT INTO wallet_transactions (
			id, transaction_id, wallet_owner_id, wallet_type, transaction_type,
			amount, balance_before, balance_after, reference, description,
			status, credit_source, idempotency_key, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
		RETURNING created_at`

	// Convert metadata to JSON
	var metadataJSON []byte
	if tx.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(tx.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	tx.ID = uuid.New()
	err := r.db.QueryRowContext(ctx, query,
		tx.ID,
		tx.TransactionID,
		tx.WalletOwnerID,
		tx.WalletType,
		tx.TransactionType,
		tx.Amount,
		tx.BalanceBefore,
		tx.BalanceAfter,
		tx.Reference,
		tx.Description,
		tx.Status,
		tx.CreditSource,
		tx.IdempotencyKey,
		metadataJSON,
	).Scan(&tx.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// Invalidate caches after successful transaction creation
	if r.cache != nil {
		// Invalidate all admin queries
		invalidateAllTransactionsCache(ctx, r.cache)

		// Invalidate owner-specific cache
		invalidateOwnerTransactionCache(ctx, r.cache, tx.WalletOwnerID, tx.WalletType)
	}

	return nil
}

// CreateTransactionTx creates a new wallet transaction within a database transaction
func (r *walletTransactionRepository) CreateTransactionTx(ctx context.Context, dbTx *sql.Tx, tx *models.WalletTransaction) error {
	query := `
		INSERT INTO wallet_transactions (
			id, transaction_id, wallet_owner_id, wallet_type, transaction_type,
			amount, balance_before, balance_after, reference, description,
			status, credit_source, idempotency_key, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
		RETURNING created_at`

	// Convert metadata to JSON
	var metadataJSON []byte
	if tx.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(tx.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	tx.ID = uuid.New()
	err := dbTx.QueryRowContext(ctx, query,
		tx.ID,
		tx.TransactionID,
		tx.WalletOwnerID,
		tx.WalletType,
		tx.TransactionType,
		tx.Amount,
		tx.BalanceBefore,
		tx.BalanceAfter,
		tx.Reference,
		tx.Description,
		tx.Status,
		tx.CreditSource,
		tx.IdempotencyKey,
		metadataJSON,
	).Scan(&tx.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// Invalidate caches after successful transaction creation
	if r.cache != nil {
		// Invalidate all admin queries
		invalidateAllTransactionsCache(ctx, r.cache)

		// Invalidate owner-specific cache
		invalidateOwnerTransactionCache(ctx, r.cache, tx.WalletOwnerID, tx.WalletType)
	}

	return nil
}

// GetTransaction retrieves a transaction by ID
func (r *walletTransactionRepository) GetTransaction(ctx context.Context, transactionID string) (*models.WalletTransaction, error) {
	// Generate cache key
	cacheKey := fmt.Sprintf("transaction:id:%s", transactionID)

	// Try cache first
	if r.cache != nil {
		cached, err := r.cache.Get(ctx, cacheKey).Result()
		if err == nil {
			// Cache hit - unmarshal and return
			var tx models.WalletTransaction
			if err := json.Unmarshal([]byte(cached), &tx); err == nil {
				log.Printf("[CACHE HIT] GetTransaction: %s", transactionID)
				return &tx, nil
			}
		}
	}

	log.Printf("[CACHE MISS] GetTransaction: %s (querying database)", transactionID)

	query := `
		SELECT id, transaction_id, wallet_owner_id, wallet_type, transaction_type,
			   amount, balance_before, balance_after, reference, description,
			   status, idempotency_key, metadata, created_at, completed_at, reversed_at
		FROM wallet_transactions
		WHERE transaction_id = $1`

	tx := &models.WalletTransaction{}
	var metadataJSON []byte
	err := r.db.QueryRowContext(ctx, query, transactionID).Scan(
		&tx.ID,
		&tx.TransactionID,
		&tx.WalletOwnerID,
		&tx.WalletType,
		&tx.TransactionType,
		&tx.Amount,
		&tx.BalanceBefore,
		&tx.BalanceAfter,
		&tx.Reference,
		&tx.Description,
		&tx.Status,
		&tx.IdempotencyKey,
		&metadataJSON,
		&tx.CreatedAt,
		&tx.CompletedAt,
		&tx.ReversedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("transaction not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Unmarshal metadata JSON if present
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &tx.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	// Cache the result (30 minute TTL - transactions are mostly immutable)
	if r.cache != nil {
		if data, err := json.Marshal(tx); err == nil {
			r.cache.Set(ctx, cacheKey, data, 30*time.Minute)
		}
	}

	return tx, nil
}

// GetTransactionByIdempotencyKey retrieves a transaction by idempotency key
func (r *walletTransactionRepository) GetTransactionByIdempotencyKey(ctx context.Context, key string) (*models.WalletTransaction, error) {
	query := `
		SELECT id, transaction_id, wallet_owner_id, wallet_type, transaction_type,
			   amount, balance_before, balance_after, reference, description,
			   status, idempotency_key, metadata, created_at, completed_at, reversed_at
		FROM wallet_transactions
		WHERE idempotency_key = $1`

	tx := &models.WalletTransaction{}
	var metadataJSON []byte
	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&tx.ID,
		&tx.TransactionID,
		&tx.WalletOwnerID,
		&tx.WalletType,
		&tx.TransactionType,
		&tx.Amount,
		&tx.BalanceBefore,
		&tx.BalanceAfter,
		&tx.Reference,
		&tx.Description,
		&tx.Status,
		&tx.IdempotencyKey,
		&metadataJSON,
		&tx.CreatedAt,
		&tx.CompletedAt,
		&tx.ReversedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction by idempotency key: %w", err)
	}

	// Unmarshal metadata JSON if present
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &tx.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return tx, nil
}

// UpdateTransactionStatus updates the status of a transaction
func (r *walletTransactionRepository) UpdateTransactionStatus(ctx context.Context, transactionID string, status models.TransactionStatus) error {
	// First, get transaction owner details directly from DB for cache invalidation
	// (Don't use GetTransaction as it returns cached data which might be stale)
	var ownerID uuid.UUID
	var walletType models.WalletType
	checkQuery := `SELECT wallet_owner_id, wallet_type FROM wallet_transactions WHERE transaction_id = $1`
	err := r.db.QueryRowContext(ctx, checkQuery, transactionID).Scan(&ownerID, &walletType)
	if err == sql.ErrNoRows {
		return fmt.Errorf("transaction not found")
	}
	if err != nil {
		return fmt.Errorf("failed to get transaction details: %w", err)
	}

	var query string
	switch status {
	case models.TransactionStatusCompleted:
		query = `UPDATE wallet_transactions SET status = $1, completed_at = NOW() WHERE transaction_id = $2`
	case models.TransactionStatusReversed:
		query = `UPDATE wallet_transactions SET status = $1, reversed_at = NOW() WHERE transaction_id = $2`
	default:
		query = `UPDATE wallet_transactions SET status = $1 WHERE transaction_id = $2`
	}

	result, err := r.db.ExecContext(ctx, query, status, transactionID)
	if err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("transaction not found")
	}

	// Invalidate caches after successful status update
	if r.cache != nil {
		// Invalidate specific transaction cache
		specificCacheKey := fmt.Sprintf("transaction:id:%s", transactionID)
		r.cache.Del(ctx, specificCacheKey)

		// Invalidate all admin queries (status affects filters)
		invalidateAllTransactionsCache(ctx, r.cache)

		// Invalidate owner-specific cache
		invalidateOwnerTransactionCache(ctx, r.cache, ownerID, walletType)
	}

	return nil
}

// GetTransactionHistory retrieves transaction history for a wallet
func (r *walletTransactionRepository) GetTransactionHistory(ctx context.Context, walletOwnerID uuid.UUID, walletType models.WalletType, limit, offset int) ([]*models.WalletTransaction, error) {
	// Generate cache key
	page := (offset / limit) + 1
	cacheKey := fmt.Sprintf("transactions:owner:%s:type:%s:page:%d:size:%d",
		walletOwnerID.String(), walletType, page, limit)

	// Try cache first
	if r.cache != nil {
		cached, err := r.cache.Get(ctx, cacheKey).Result()
		if err == nil {
			// Cache hit - unmarshal and return
			var transactions []*models.WalletTransaction
			if err := json.Unmarshal([]byte(cached), &transactions); err == nil {
				log.Printf("[CACHE HIT] GetTransactionHistory: owner=%s type=%s page=%d (count=%d)",
					walletOwnerID.String()[:8], walletType, page, len(transactions))
				return transactions, nil
			}
		}
	}

	log.Printf("[CACHE MISS] GetTransactionHistory: owner=%s type=%s page=%d (querying database)",
		walletOwnerID.String()[:8], walletType, page)

	query := `
		SELECT id, transaction_id, wallet_owner_id, wallet_type, transaction_type,
			   amount, balance_before, balance_after, reference, description,
			   status, idempotency_key, metadata, created_at, completed_at, reversed_at
		FROM wallet_transactions
		WHERE wallet_owner_id = $1 AND wallet_type = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`

	rows, err := r.db.QueryContext(ctx, query, walletOwnerID, walletType, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction history: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Log the error but don't override the main error
			// This is a best-effort cleanup
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var transactions []*models.WalletTransaction
	for rows.Next() {
		tx := &models.WalletTransaction{}
		var metadataJSON []byte
		err := rows.Scan(
			&tx.ID,
			&tx.TransactionID,
			&tx.WalletOwnerID,
			&tx.WalletType,
			&tx.TransactionType,
			&tx.Amount,
			&tx.BalanceBefore,
			&tx.BalanceAfter,
			&tx.Reference,
			&tx.Description,
			&tx.Status,
			&tx.IdempotencyKey,
			&metadataJSON,
			&tx.CreatedAt,
			&tx.CompletedAt,
			&tx.ReversedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		// Unmarshal metadata JSON if present
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &tx.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		// Skip audit transactions (internal tracing only - not for user display)
		if tx.Metadata != nil {
			if auditTrace, exists := tx.Metadata["audit_trace"]; exists && auditTrace == true {
				continue // Don't include audit transactions in user-facing history
			}
		}

		transactions = append(transactions, tx)
	}

	// Cache the results (5 minute TTL)
	if r.cache != nil {
		if data, err := json.Marshal(transactions); err == nil {
			r.cache.Set(ctx, cacheKey, data, 5*time.Minute)
		}
	}

	return transactions, nil
}

// CountTransactions counts total transactions for a wallet
func (r *walletTransactionRepository) CountTransactions(ctx context.Context, walletOwnerID uuid.UUID, walletType models.WalletType) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM wallet_transactions
		WHERE wallet_owner_id = $1 AND wallet_type = $2`

	var count int
	err := r.db.QueryRowContext(ctx, query, walletOwnerID, walletType).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	return count, nil
}

// CreateTransfer creates a new wallet transfer
func (r *walletTransactionRepository) CreateTransfer(ctx context.Context, transfer *models.WalletTransfer) error {
	query := `
		INSERT INTO wallet_transfers (
			id, transfer_id, from_wallet_id, from_wallet_type, to_wallet_id, to_wallet_type,
			amount, commission_amount, total_deducted, reference, notes,
			status, idempotency_key, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW())
		RETURNING created_at`

	transfer.ID = uuid.New()
	err := r.db.QueryRowContext(ctx, query,
		transfer.ID,
		transfer.TransferID,
		transfer.FromWalletID,
		transfer.FromWalletType,
		transfer.ToWalletID,
		transfer.ToWalletType,
		transfer.Amount,
		transfer.CommissionAmount,
		transfer.TotalDeducted,
		transfer.Reference,
		transfer.Notes,
		transfer.Status,
		transfer.IdempotencyKey,
	).Scan(&transfer.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create transfer: %w", err)
	}

	return nil
}

// GetTransferByIdempotencyKey retrieves a transfer by idempotency key
func (r *walletTransactionRepository) GetTransferByIdempotencyKey(ctx context.Context, key string) (*models.WalletTransfer, error) {
	query := `
		SELECT id, transfer_id, from_wallet_id, from_wallet_type, to_wallet_id, to_wallet_type,
			   amount, commission_amount, total_deducted, reference, notes,
			   status, idempotency_key, created_at, completed_at, reversed_at
		FROM wallet_transfers
		WHERE idempotency_key = $1`

	transfer := &models.WalletTransfer{}
	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&transfer.ID,
		&transfer.TransferID,
		&transfer.FromWalletID,
		&transfer.FromWalletType,
		&transfer.ToWalletID,
		&transfer.ToWalletType,
		&transfer.Amount,
		&transfer.CommissionAmount,
		&transfer.TotalDeducted,
		&transfer.Reference,
		&transfer.Notes,
		&transfer.Status,
		&transfer.IdempotencyKey,
		&transfer.CreatedAt,
		&transfer.CompletedAt,
		&transfer.ReversedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transfer by idempotency key: %w", err)
	}

	return transfer, nil
}

// adminTransactionRepository implements AdminTransactionRepository interface
type adminTransactionRepository struct {
	db    *sql.DB
	cache *redis.Client
}

// NewAdminTransactionRepository creates a new instance of AdminTransactionRepository
func NewAdminTransactionRepository(db *sql.DB, cache *redis.Client) AdminTransactionRepository {
	return &adminTransactionRepository{
		db:    db,
		cache: cache,
	}
}

// GetAllTransactions retrieves all transactions with filters for admin use
func (r *adminTransactionRepository) GetAllTransactions(ctx context.Context, filters AdminTransactionFilters) ([]*models.WalletTransaction, int, error) {
	// Generate cache key
	filtersHash := generateFiltersHash(filters)
	cacheKey := fmt.Sprintf("transactions:all:v2:filters:%s:page:%d", filtersHash, filters.Page)

	// Try cache first
	if r.cache != nil {
		cached, err := r.cache.Get(ctx, cacheKey).Result()
		if err == nil {
			// Cache hit - unmarshal and return
			var cachedData struct {
				Transactions []*models.WalletTransaction
				TotalCount   int
			}
			if err := json.Unmarshal([]byte(cached), &cachedData); err == nil {
				log.Printf("[CACHE HIT] GetAllTransactions: hash=%s page=%d (count=%d, total=%d)",
					filtersHash[:8], filters.Page, len(cachedData.Transactions), cachedData.TotalCount)
				return cachedData.Transactions, cachedData.TotalCount, nil
			}
		}
	}

	log.Printf("[CACHE MISS] GetAllTransactions: hash=%s page=%d (querying database)", filtersHash[:8], filters.Page)

	// Build the WHERE clause dynamically based on filters
	whereConditions := []string{
		"(metadata IS NULL OR metadata->>'audit_trace' IS DISTINCT FROM 'true')",
	}
	args := []interface{}{}
	argIndex := 1

	// Filter by transaction types
	if len(filters.TransactionTypes) > 0 {
		placeholders := make([]string, len(filters.TransactionTypes))
		for i, txType := range filters.TransactionTypes {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, txType)
			argIndex++
		}
		whereConditions = append(whereConditions, fmt.Sprintf("transaction_type IN (%s)", joinStrings(placeholders, ", ")))
	}

	// Filter by wallet types
	if len(filters.WalletTypes) > 0 {
		placeholders := make([]string, len(filters.WalletTypes))
		for i, wType := range filters.WalletTypes {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, wType)
			argIndex++
		}
		whereConditions = append(whereConditions, fmt.Sprintf("wallet_type IN (%s)", joinStrings(placeholders, ", ")))
	}

	// Filter by statuses
	if len(filters.Statuses) > 0 {
		placeholders := make([]string, len(filters.Statuses))
		for i, status := range filters.Statuses {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, status)
			argIndex++
		}
		whereConditions = append(whereConditions, fmt.Sprintf("status IN (%s)", joinStrings(placeholders, ", ")))
	}

	// Filter by start date
	if filters.StartDate != nil && *filters.StartDate != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filters.StartDate)
		argIndex++
	}

	// Filter by end date
	if filters.EndDate != nil && *filters.EndDate != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filters.EndDate)
		argIndex++
	}

	// Search term (transaction_id, reference, description)
	if filters.SearchTerm != nil && *filters.SearchTerm != "" {
		searchPattern := "%" + *filters.SearchTerm + "%"
		whereConditions = append(whereConditions, fmt.Sprintf(
			"(transaction_id ILIKE $%d OR reference ILIKE $%d OR description ILIKE $%d)",
			argIndex, argIndex, argIndex,
		))
		args = append(args, searchPattern)
		argIndex++
	}

	// Build WHERE clause
	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + joinStrings(whereConditions, " AND ")
	}

	// Get total count for pagination
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM wallet_transactions %s", whereClause)
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	// Validate and set defaults for sorting
	sortBy := "created_at"
	if filters.SortBy != "" {
		// Whitelist allowed sort fields for security
		allowedSortFields := map[string]bool{
			"created_at":       true,
			"amount":           true,
			"transaction_type": true,
			"status":           true,
		}
		if allowedSortFields[filters.SortBy] {
			sortBy = filters.SortBy
		}
	}

	sortOrder := "DESC"
	if filters.SortOrder == "asc" {
		sortOrder = "ASC"
	}

	// Validate and set pagination
	page := filters.Page
	if page < 1 {
		page = 1
	}

	pageSize := filters.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100 // Max limit for safety
	}

	offset := (page - 1) * pageSize

	// Build and execute main query
	query := fmt.Sprintf(`
		SELECT id, transaction_id, wallet_owner_id, wallet_type, transaction_type,
			   amount, balance_before, balance_after, reference, description,
			   status, idempotency_key, metadata, created_at, completed_at, reversed_at
		FROM wallet_transactions
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		whereClause,
		sortBy,
		sortOrder,
		argIndex,
		argIndex+1,
	)

	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get all transactions: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var transactions []*models.WalletTransaction
	for rows.Next() {
		tx := &models.WalletTransaction{}
		var metadataJSON []byte

		err := rows.Scan(
			&tx.ID,
			&tx.TransactionID,
			&tx.WalletOwnerID,
			&tx.WalletType,
			&tx.TransactionType,
			&tx.Amount,
			&tx.BalanceBefore,
			&tx.BalanceAfter,
			&tx.Reference,
			&tx.Description,
			&tx.Status,
			&tx.IdempotencyKey,
			&metadataJSON,
			&tx.CreatedAt,
			&tx.CompletedAt,
			&tx.ReversedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan transaction: %w", err)
		}

		// Unmarshal metadata JSON if present
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &tx.Metadata); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		// Initialize metadata if nil
		if tx.Metadata == nil {
			tx.Metadata = make(map[string]interface{})
		}

		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating rows: %w", err)
	}

	// Cache the results (2 minute TTL)
	if r.cache != nil {
		cacheData := struct {
			Transactions []*models.WalletTransaction
			TotalCount   int
		}{
			Transactions: transactions,
			TotalCount:   totalCount,
		}
		if data, err := json.Marshal(cacheData); err == nil {
			r.cache.Set(ctx, cacheKey, data, 2*time.Minute)
		}
	}

	return transactions, totalCount, nil
}

// GetTransactionStatistics calculates aggregated statistics based on filters
func (r *adminTransactionRepository) GetTransactionStatistics(ctx context.Context, filters AdminTransactionFilters) (*TransactionStatistics, error) {
	// Generate cache key
	filtersHash := generateFiltersHash(filters)
	cacheKey := fmt.Sprintf("transactions:stats:v2:filters:%s", filtersHash)

	// Try cache first
	if r.cache != nil {
		cached, err := r.cache.Get(ctx, cacheKey).Result()
		if err == nil {
			// Cache hit - unmarshal and return
			var stats TransactionStatistics
			if err := json.Unmarshal([]byte(cached), &stats); err == nil {
				return &stats, nil
			}
		}
	}

	// Build the WHERE clause dynamically (same logic as GetAllTransactions)
	whereConditions := []string{
		"(metadata IS NULL OR metadata->>'audit_trace' IS DISTINCT FROM 'true')",
	}
	args := []interface{}{}
	argIndex := 1

	// Filter by transaction types
	if len(filters.TransactionTypes) > 0 {
		placeholders := make([]string, len(filters.TransactionTypes))
		for i, txType := range filters.TransactionTypes {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, txType)
			argIndex++
		}
		whereConditions = append(whereConditions, fmt.Sprintf("transaction_type IN (%s)", joinStrings(placeholders, ", ")))
	}

	// Filter by wallet types
	if len(filters.WalletTypes) > 0 {
		placeholders := make([]string, len(filters.WalletTypes))
		for i, wType := range filters.WalletTypes {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, wType)
			argIndex++
		}
		whereConditions = append(whereConditions, fmt.Sprintf("wallet_type IN (%s)", joinStrings(placeholders, ", ")))
	}

	// Filter by statuses
	if len(filters.Statuses) > 0 {
		placeholders := make([]string, len(filters.Statuses))
		for i, status := range filters.Statuses {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, status)
			argIndex++
		}
		whereConditions = append(whereConditions, fmt.Sprintf("status IN (%s)", joinStrings(placeholders, ", ")))
	}

	// Filter by start date
	if filters.StartDate != nil && *filters.StartDate != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filters.StartDate)
		argIndex++
	}

	// Filter by end date
	if filters.EndDate != nil && *filters.EndDate != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filters.EndDate)
		argIndex++
	}

	// Search term
	if filters.SearchTerm != nil && *filters.SearchTerm != "" {
		searchPattern := "%" + *filters.SearchTerm + "%"
		whereConditions = append(whereConditions, fmt.Sprintf(
			"(transaction_id ILIKE $%d OR reference ILIKE $%d OR description ILIKE $%d)",
			argIndex, argIndex, argIndex,
		))
		args = append(args, searchPattern)
	}

	// Build WHERE clause
	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + joinStrings(whereConditions, " AND ")
	}

	// Build statistics query using aggregate functions
	query := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(ABS(amount)), 0) as total_volume,
			COALESCE(SUM(CASE WHEN transaction_type = 'CREDIT' THEN amount ELSE 0 END), 0) as total_credits,
			COALESCE(SUM(CASE WHEN transaction_type = 'DEBIT' THEN ABS(amount) ELSE 0 END), 0) as total_debits,
			COALESCE(SUM(CASE WHEN status = 'PENDING' THEN ABS(amount) ELSE 0 END), 0) as pending_amount,
			COUNT(CASE WHEN status = 'PENDING' THEN 1 END) as pending_count,
			COUNT(CASE WHEN status = 'COMPLETED' THEN 1 END) as completed_count,
			COUNT(CASE WHEN status = 'FAILED' THEN 1 END) as failed_count,
			COUNT(CASE WHEN transaction_type = 'CREDIT' THEN 1 END) as credit_count,
			COUNT(CASE WHEN transaction_type = 'DEBIT' THEN 1 END) as debit_count,
			COUNT(CASE WHEN transaction_type = 'TRANSFER' THEN 1 END) as transfer_count,
			COUNT(CASE WHEN transaction_type = 'COMMISSION' THEN 1 END) as commission_count,
			COUNT(CASE WHEN transaction_type = 'PAYOUT' THEN 1 END) as payout_count
		FROM wallet_transactions
		%s`, whereClause)

	stats := &TransactionStatistics{}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.TotalVolume,
		&stats.TotalCredits,
		&stats.TotalDebits,
		&stats.PendingAmount,
		&stats.PendingCount,
		&stats.CompletedCount,
		&stats.FailedCount,
		&stats.CreditCount,
		&stats.DebitCount,
		&stats.TransferCount,
		&stats.CommissionCount,
		&stats.PayoutCount,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get transaction statistics: %w", err)
	}

	// Cache the statistics (2 minute TTL)
	if r.cache != nil {
		if data, err := json.Marshal(stats); err == nil {
			r.cache.Set(ctx, cacheKey, data, 2*time.Minute)
		}
	}

	return stats, nil
}

// joinStrings is a helper function to join string slices
func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

// Cache helper functions

// generateFiltersHash generates a consistent hash for filter combinations
func generateFiltersHash(filters AdminTransactionFilters) string {
	// Serialize filters to consistent string representation
	var parts []string

	// Transaction types
	for _, tt := range filters.TransactionTypes {
		parts = append(parts, fmt.Sprintf("tt:%s", tt))
	}

	// Wallet types
	for _, wt := range filters.WalletTypes {
		parts = append(parts, fmt.Sprintf("wt:%s", wt))
	}

	// Statuses
	for _, st := range filters.Statuses {
		parts = append(parts, fmt.Sprintf("st:%s", st))
	}

	// Dates
	if filters.StartDate != nil {
		parts = append(parts, fmt.Sprintf("sd:%s", *filters.StartDate))
	}
	if filters.EndDate != nil {
		parts = append(parts, fmt.Sprintf("ed:%s", *filters.EndDate))
	}

	// Search term
	if filters.SearchTerm != nil {
		parts = append(parts, fmt.Sprintf("q:%s", *filters.SearchTerm))
	}

	// Sort
	parts = append(parts, fmt.Sprintf("sort:%s:%s", filters.SortBy, filters.SortOrder))

	// Join and hash
	data := joinStrings(parts, "|")
	hash := fmt.Sprintf("%x", []byte(data))

	// Return first 16 characters for brevity
	if len(hash) > 16 {
		return hash[:16]
	}
	return hash
}

// invalidateOwnerTransactionCache invalidates cache for a specific owner's transactions
func invalidateOwnerTransactionCache(ctx context.Context, cache *redis.Client, ownerID uuid.UUID, walletType models.WalletType) {
	if cache == nil {
		return
	}

	pattern := fmt.Sprintf("transactions:owner:%s:type:%s:*", ownerID.String(), walletType)
	keys, err := cache.Keys(ctx, pattern).Result()
	if err == nil && len(keys) > 0 {
		cache.Del(ctx, keys...)
		log.Printf("[CACHE INVALIDATE] Owner cache: owner=%s type=%s (deleted %d keys)",
			ownerID.String()[:8], walletType, len(keys))
	}
}

// invalidateAllTransactionsCache invalidates all admin transaction queries
func invalidateAllTransactionsCache(ctx context.Context, cache *redis.Client) {
	if cache == nil {
		return
	}

	totalDeleted := 0

	// Invalidate admin list queries
	keys1, err1 := cache.Keys(ctx, "transactions:all:*").Result()
	if err1 == nil && len(keys1) > 0 {
		cache.Del(ctx, keys1...)
		totalDeleted += len(keys1)
	}

	// Invalidate statistics queries
	keys2, err2 := cache.Keys(ctx, "transactions:stats:*").Result()
	if err2 == nil && len(keys2) > 0 {
		cache.Del(ctx, keys2...)
		totalDeleted += len(keys2)
	}

	if totalDeleted > 0 {
		log.Printf("[CACHE INVALIDATE] Admin cache: deleted %d keys (all+stats)", totalDeleted)
	}
}

// transactionReversalRepository implements TransactionReversalRepository interface
type transactionReversalRepository struct {
	db    *sql.DB
	cache *redis.Client
}

// NewTransactionReversalRepository creates a new instance of TransactionReversalRepository
func NewTransactionReversalRepository(db *sql.DB, cache *redis.Client) TransactionReversalRepository {
	return &transactionReversalRepository{
		db:    db,
		cache: cache,
	}
}

// GetTransactionByID retrieves a transaction by its UUID
func (r *transactionReversalRepository) GetTransactionByID(ctx context.Context, txID uuid.UUID) (*models.WalletTransaction, error) {
	query := `
		SELECT id, transaction_id, wallet_owner_id, wallet_type, transaction_type,
			   amount, balance_before, balance_after, reference, description,
			   status, idempotency_key, metadata, created_at, completed_at, reversed_at,
			   reversed_by_transaction_id, reversal_reason
		FROM wallet_transactions
		WHERE id = $1`

	tx := &models.WalletTransaction{}
	var metadataJSON []byte
	var reversedByTxID sql.NullString
	var reversalReason sql.NullString

	err := r.db.QueryRowContext(ctx, query, txID).Scan(
		&tx.ID,
		&tx.TransactionID,
		&tx.WalletOwnerID,
		&tx.WalletType,
		&tx.TransactionType,
		&tx.Amount,
		&tx.BalanceBefore,
		&tx.BalanceAfter,
		&tx.Reference,
		&tx.Description,
		&tx.Status,
		&tx.IdempotencyKey,
		&metadataJSON,
		&tx.CreatedAt,
		&tx.CompletedAt,
		&tx.ReversedAt,
		&reversedByTxID,
		&reversalReason,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("transaction not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Unmarshal metadata JSON if present
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &tx.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	// Populate reversal fields if present
	if reversedByTxID.Valid {
		txUUID, err := uuid.Parse(reversedByTxID.String)
		if err == nil {
			tx.ReversedByTransactionID = &txUUID
		}
	}
	if reversalReason.Valid {
		tx.ReversalReason = &reversalReason.String
	}

	return tx, nil
}

// GetTransactionByTransactionID retrieves a transaction by its transaction_id (string)
func (r *transactionReversalRepository) GetTransactionByTransactionID(ctx context.Context, transactionID string) (*models.WalletTransaction, error) {
	query := `
		SELECT id, transaction_id, wallet_owner_id, wallet_type, transaction_type,
			   amount, balance_before, balance_after, reference, description,
			   status, idempotency_key, metadata, created_at, completed_at, reversed_at,
			   reversed_by_transaction_id, reversal_reason
		FROM wallet_transactions
		WHERE transaction_id = $1`

	tx := &models.WalletTransaction{}
	var metadataJSON []byte
	var reversedByTxID sql.NullString
	var reversalReason sql.NullString

	err := r.db.QueryRowContext(ctx, query, transactionID).Scan(
		&tx.ID,
		&tx.TransactionID,
		&tx.WalletOwnerID,
		&tx.WalletType,
		&tx.TransactionType,
		&tx.Amount,
		&tx.BalanceBefore,
		&tx.BalanceAfter,
		&tx.Reference,
		&tx.Description,
		&tx.Status,
		&tx.IdempotencyKey,
		&metadataJSON,
		&tx.CreatedAt,
		&tx.CompletedAt,
		&tx.ReversedAt,
		&reversedByTxID,
		&reversalReason,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("transaction not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Unmarshal metadata JSON if present
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &tx.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	// Populate reversal fields if present
	if reversedByTxID.Valid {
		txUUID, err := uuid.Parse(reversedByTxID.String)
		if err == nil {
			tx.ReversedByTransactionID = &txUUID
		}
	}
	if reversalReason.Valid {
		tx.ReversalReason = &reversalReason.String
	}

	return tx, nil
}

// GetTransactionForReversalWithLock retrieves a transaction with row-level lock for reversal
func (r *transactionReversalRepository) GetTransactionForReversalWithLock(ctx context.Context, dbTx *sql.Tx, txID uuid.UUID) (*models.WalletTransaction, error) {
	query := `
		SELECT id, transaction_id, wallet_owner_id, wallet_type, transaction_type,
			   amount, balance_before, balance_after, reference, description,
			   status, idempotency_key, metadata, created_at, completed_at, reversed_at,
			   reversed_by_transaction_id, reversal_reason
		FROM wallet_transactions
		WHERE id = $1
		FOR UPDATE`

	tx := &models.WalletTransaction{}
	var metadataJSON []byte
	var reversedByTxID sql.NullString
	var reversalReason sql.NullString

	err := dbTx.QueryRowContext(ctx, query, txID).Scan(
		&tx.ID,
		&tx.TransactionID,
		&tx.WalletOwnerID,
		&tx.WalletType,
		&tx.TransactionType,
		&tx.Amount,
		&tx.BalanceBefore,
		&tx.BalanceAfter,
		&tx.Reference,
		&tx.Description,
		&tx.Status,
		&tx.IdempotencyKey,
		&metadataJSON,
		&tx.CreatedAt,
		&tx.CompletedAt,
		&tx.ReversedAt,
		&reversedByTxID,
		&reversalReason,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("transaction not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Unmarshal metadata JSON if present
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &tx.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	// Populate reversal fields if present
	if reversedByTxID.Valid {
		txUUID, err := uuid.Parse(reversedByTxID.String)
		if err == nil {
			tx.ReversedByTransactionID = &txUUID
		}
	}
	if reversalReason.Valid {
		tx.ReversalReason = &reversalReason.String
	}

	return tx, nil
}

// MarkTransactionAsReversed marks a transaction as reversed
func (r *transactionReversalRepository) MarkTransactionAsReversed(ctx context.Context, dbTx *sql.Tx, txID uuid.UUID, reversalTxID uuid.UUID, reason string) error {
	query := `
		UPDATE wallet_transactions
		SET status = $1,
		    reversed_at = NOW(),
		    reversed_by_transaction_id = $2,
		    reversal_reason = $3
		WHERE id = $4`

	result, err := dbTx.ExecContext(ctx, query, models.TransactionStatusReversed, reversalTxID, reason, txID)
	if err != nil {
		return fmt.Errorf("failed to mark transaction as reversed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("transaction not found")
	}

	return nil
}

// CreateReversalAudit creates a reversal audit record
func (r *transactionReversalRepository) CreateReversalAudit(ctx context.Context, dbTx *sql.Tx, reversal *models.TransactionReversal) error {
	query := `
		INSERT INTO transaction_reversals (
			id, original_transaction_id, reversal_transaction_id, original_amount,
			wallet_owner_id, wallet_type, reason, reversed_by, reversed_by_name,
			reversed_by_email, reversed_at, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
		RETURNING created_at`

	// Convert metadata to JSON
	var metadataJSON []byte
	if reversal.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(reversal.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	reversal.ID = uuid.New()
	err := dbTx.QueryRowContext(ctx, query,
		reversal.ID,
		reversal.OriginalTransactionID,
		reversal.ReversalTransactionID,
		reversal.OriginalAmount,
		reversal.WalletOwnerID,
		reversal.WalletType,
		reversal.Reason,
		reversal.ReversedBy,
		reversal.ReversedByName,
		reversal.ReversedByEmail,
		reversal.ReversedAt,
		metadataJSON,
	).Scan(&reversal.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create reversal audit: %w", err)
	}

	return nil
}

// GetReversalByOriginalTransaction retrieves a reversal record by original transaction ID
func (r *transactionReversalRepository) GetReversalByOriginalTransaction(ctx context.Context, txID uuid.UUID) (*models.TransactionReversal, error) {
	query := `
		SELECT id, original_transaction_id, reversal_transaction_id, original_amount,
			   wallet_owner_id, wallet_type, reason, reversed_by, reversed_by_name,
			   reversed_by_email, reversed_at, metadata, created_at
		FROM transaction_reversals
		WHERE original_transaction_id = $1`

	reversal := &models.TransactionReversal{}
	var metadataJSON []byte
	err := r.db.QueryRowContext(ctx, query, txID).Scan(
		&reversal.ID,
		&reversal.OriginalTransactionID,
		&reversal.ReversalTransactionID,
		&reversal.OriginalAmount,
		&reversal.WalletOwnerID,
		&reversal.WalletType,
		&reversal.Reason,
		&reversal.ReversedBy,
		&reversal.ReversedByName,
		&reversal.ReversedByEmail,
		&reversal.ReversedAt,
		&metadataJSON,
		&reversal.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No reversal found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get reversal: %w", err)
	}

	// Unmarshal metadata JSON if present
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &reversal.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return reversal, nil
}

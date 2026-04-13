package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/randco/service-wallet/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// IdempotencyRepository defines idempotency key operations
type IdempotencyRepository interface {
	// CheckIdempotencyKey checks if an idempotency key already exists
	// Returns the existing transaction if found, nil if not found
	CheckIdempotencyKey(ctx context.Context, key string) (*models.WalletTransaction, error)

	// SaveIdempotencyKey saves an idempotency key with transaction reference
	SaveIdempotencyKey(ctx context.Context, record *models.IdempotencyKey) error

	// SaveIdempotencyKeyTx saves an idempotency key inside a database transaction
	// This ensures atomicity with the wallet operation
	// Returns true if key was inserted, false if key already existed (conflict)
	SaveIdempotencyKeyTx(ctx context.Context, tx *sql.Tx, record *models.IdempotencyKey) (bool, error)

	// CleanupExpiredKeys removes idempotency keys older than the specified days
	CleanupExpiredKeys(ctx context.Context, daysOld int) (int64, error)
}

type idempotencyRepository struct {
	db *sqlx.DB
}

// NewIdempotencyRepository creates a new idempotency repository
func NewIdempotencyRepository(db *sqlx.DB) IdempotencyRepository {
	return &idempotencyRepository{db: db}
}

// CheckIdempotencyKey checks if an idempotency key already exists
func (r *idempotencyRepository) CheckIdempotencyKey(ctx context.Context, key string) (*models.WalletTransaction, error) {
	tracer := otel.Tracer("wallet-service")
	ctx, span := tracer.Start(ctx, "IdempotencyRepository.CheckIdempotencyKey")
	defer span.End()

	span.SetAttributes(
		attribute.String("idempotency.key", key),
	)

	// Check if key exists in idempotency table
	query := `
		SELECT transaction_id
		FROM wallet_idempotency_keys
		WHERE idempotency_key = $1`

	var transactionID uuid.UUID
	err := r.db.GetContext(ctx, &transactionID, query, key)
	if err != nil {
		if err == sql.ErrNoRows {
			// Key doesn't exist - this is a new request
			span.SetAttributes(attribute.String("result", "key_not_found"))
			return nil, nil
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to check idempotency key: %w", err)
	}

	// Key exists - fetch the existing transaction
	span.SetAttributes(
		attribute.String("result", "key_found"),
		attribute.String("transaction.id", transactionID.String()),
	)

	txQuery := `
		SELECT id, transaction_id, wallet_owner_id, wallet_type, transaction_type,
		       amount, balance_before, balance_after, reference, description,
		       status, idempotency_key, metadata, created_at, completed_at, reversed_at
		FROM wallet_transactions
		WHERE id = $1`

	// Scan into a temporary struct with json.RawMessage for metadata
	var tx struct {
		ID              uuid.UUID                `db:"id"`
		TransactionID   string                   `db:"transaction_id"`
		WalletOwnerID   uuid.UUID                `db:"wallet_owner_id"`
		WalletType      models.WalletType        `db:"wallet_type"`
		TransactionType models.TransactionType   `db:"transaction_type"`
		Amount          int64                    `db:"amount"`
		BalanceBefore   int64                    `db:"balance_before"`
		BalanceAfter    int64                    `db:"balance_after"`
		Reference       *string                  `db:"reference"`
		Description     *string                  `db:"description"`
		Status          models.TransactionStatus `db:"status"`
		IdempotencyKey  *string                  `db:"idempotency_key"`
		MetadataJSON    json.RawMessage          `db:"metadata"`
		CreatedAt       sql.NullTime             `db:"created_at"`
		CompletedAt     sql.NullTime             `db:"completed_at"`
		ReversedAt      sql.NullTime             `db:"reversed_at"`
	}

	err = r.db.GetContext(ctx, &tx, txQuery, transactionID)
	if err != nil {
		if err == sql.ErrNoRows {
			// This shouldn't happen - idempotency key references non-existent transaction
			span.SetAttributes(attribute.String("error", "orphaned_idempotency_key"))
			return nil, fmt.Errorf("idempotency key exists but transaction not found")
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to fetch existing transaction: %w", err)
	}

	// Unmarshal metadata JSON
	var metadata map[string]interface{}
	if len(tx.MetadataJSON) > 0 {
		if err := json.Unmarshal(tx.MetadataJSON, &metadata); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	// Convert to WalletTransaction model
	transaction := &models.WalletTransaction{
		ID:              tx.ID,
		TransactionID:   tx.TransactionID,
		WalletOwnerID:   tx.WalletOwnerID,
		WalletType:      tx.WalletType,
		TransactionType: tx.TransactionType,
		Amount:          tx.Amount,
		BalanceBefore:   tx.BalanceBefore,
		BalanceAfter:    tx.BalanceAfter,
		Reference:       tx.Reference,
		Description:     tx.Description,
		Status:          tx.Status,
		IdempotencyKey:  tx.IdempotencyKey,
		Metadata:        metadata,
	}

	// Handle nullable time fields
	if tx.CreatedAt.Valid {
		transaction.CreatedAt = tx.CreatedAt.Time
	}
	if tx.CompletedAt.Valid {
		transaction.CompletedAt = &tx.CompletedAt.Time
	}
	if tx.ReversedAt.Valid {
		transaction.ReversedAt = &tx.ReversedAt.Time
	}

	return transaction, nil
}

// SaveIdempotencyKey saves an idempotency key with transaction reference
func (r *idempotencyRepository) SaveIdempotencyKey(ctx context.Context, record *models.IdempotencyKey) error {
	tracer := otel.Tracer("wallet-service")
	ctx, span := tracer.Start(ctx, "IdempotencyRepository.SaveIdempotencyKey")
	defer span.End()

	span.SetAttributes(
		attribute.String("idempotency.key", record.IdempotencyKey),
		attribute.String("transaction.id", record.TransactionID.String()),
		attribute.String("wallet.owner_id", record.WalletOwnerID.String()),
		attribute.String("operation.type", record.OperationType),
	)

	query := `
		INSERT INTO wallet_idempotency_keys (
			idempotency_key, transaction_id, wallet_owner_id, wallet_type,
			operation_type, amount, created_at
		) VALUES (
			:idempotency_key, :transaction_id, :wallet_owner_id, :wallet_type,
			:operation_type, :amount, :created_at
		)
		ON CONFLICT (idempotency_key) DO NOTHING`

	result, err := r.db.NamedExecContext(ctx, query, record)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to save idempotency key: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	span.SetAttributes(attribute.Int64("rows.affected", rowsAffected))
	return nil
}

// SaveIdempotencyKeyTx saves an idempotency key within a database transaction
// This ensures atomicity - idempotency key is only saved if wallet operation commits
// Returns (true, nil) if key was inserted successfully
// Returns (false, nil) if key already existed (duplicate request detected)
// Returns (false, err) if database error occurred
func (r *idempotencyRepository) SaveIdempotencyKeyTx(ctx context.Context, tx *sql.Tx, record *models.IdempotencyKey) (bool, error) {
	tracer := otel.Tracer("wallet-service")
	ctx, span := tracer.Start(ctx, "IdempotencyRepository.SaveIdempotencyKeyTx")
	defer span.End()

	span.SetAttributes(
		attribute.String("idempotency.key", record.IdempotencyKey),
		attribute.String("transaction.id", record.TransactionID.String()),
		attribute.String("wallet.owner_id", record.WalletOwnerID.String()),
		attribute.String("operation.type", record.OperationType),
	)

	query := `
		INSERT INTO wallet_idempotency_keys (
			idempotency_key, transaction_id, wallet_owner_id, wallet_type,
			operation_type, amount, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
		ON CONFLICT (idempotency_key) DO NOTHING`

	result, err := tx.ExecContext(ctx, query,
		record.IdempotencyKey,
		record.TransactionID,
		record.WalletOwnerID,
		record.WalletType,
		record.OperationType,
		record.Amount,
		record.CreatedAt,
	)
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("failed to save idempotency key in transaction: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to get rows affected: %w", err)
	}

	span.SetAttributes(
		attribute.Int64("rows.affected", rowsAffected),
		attribute.Bool("key.inserted", rowsAffected > 0),
	)

	// If rows affected is 0, the key already existed (conflict)
	if rowsAffected == 0 {
		span.SetAttributes(attribute.String("result", "duplicate_key_detected"))
		return false, nil
	}

	return true, nil
}

// CleanupExpiredKeys removes idempotency keys older than the specified days
func (r *idempotencyRepository) CleanupExpiredKeys(ctx context.Context, daysOld int) (int64, error) {
	tracer := otel.Tracer("wallet-service")
	ctx, span := tracer.Start(ctx, "IdempotencyRepository.CleanupExpiredKeys")
	defer span.End()

	span.SetAttributes(attribute.Int("days.old", daysOld))

	query := `
		DELETE FROM wallet_idempotency_keys
		WHERE created_at < NOW() - INTERVAL '1 day' * $1`

	result, err := r.db.ExecContext(ctx, query, daysOld)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("failed to cleanup expired idempotency keys: %w", err)
	}

	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	span.SetAttributes(attribute.Int64("rows.deleted", rowsDeleted))
	return rowsDeleted, nil
}

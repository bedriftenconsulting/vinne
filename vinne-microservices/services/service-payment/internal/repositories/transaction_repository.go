package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/randco/service-payment/internal/models"
)

var tracer = otel.Tracer("payment-service/repositories")

// TransactionRepository defines the interface for transaction data access
type TransactionRepository interface {
	Create(ctx context.Context, tx *models.Transaction) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Transaction, error)
	GetByReference(ctx context.Context, reference string) (*models.Transaction, error)
	GetByReferenceForUpdate(ctx context.Context, reference string) (*models.Transaction, error)
	Update(ctx context.Context, tx *models.Transaction) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.TransactionStatus) error
	List(ctx context.Context, filter *models.TransactionFilter) ([]*models.Transaction, int64, error)
	GetPendingTransactions(ctx context.Context, limit int) ([]*models.Transaction, error)
	GetTransactionsByStatus(ctx context.Context, status models.TransactionStatus, limit int) ([]*models.Transaction, error)
}

// transactionRepository implements TransactionRepository
type transactionRepository struct {
	db *sqlx.DB
}

// NewTransactionRepository creates a new transaction repository
func NewTransactionRepository(db *sqlx.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

// Create creates a new transaction
func (r *transactionRepository) Create(ctx context.Context, tx *models.Transaction) error {
	ctx, span := tracer.Start(ctx, "transaction_repository.create",
		trace.WithAttributes(
			attribute.String("reference", tx.Reference),
			attribute.String("type", string(tx.Type)),
		))
	defer span.End()

	query := `
		INSERT INTO transactions (
			id, reference, type, status, amount, currency, narration,
			provider_name, source_type, source_identifier, source_name,
			destination_type, destination_identifier, destination_name,
			user_id, customer_remarks, metadata, provider_data,
			requested_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20, $21
		)
	`

	metadataJSON, err := json.Marshal(tx.Metadata)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	providerDataJSON, err := json.Marshal(tx.ProviderData)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal provider data: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		tx.ID, tx.Reference, tx.Type, tx.Status, tx.Amount, tx.Currency, tx.Narration,
		tx.ProviderName, tx.SourceType, tx.SourceIdentifier, tx.SourceName,
		tx.DestinationType, tx.DestinationIdentifier, tx.DestinationName,
		tx.UserID, tx.CustomerRemarks, metadataJSON, providerDataJSON,
		tx.RequestedAt, tx.CreatedAt, tx.UpdatedAt,
	)

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	span.SetAttributes(attribute.String("transaction_id", tx.ID.String()))
	return nil
}

// GetByID retrieves a transaction by ID
func (r *transactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Transaction, error) {
	ctx, span := tracer.Start(ctx, "transaction_repository.get_by_id",
		trace.WithAttributes(attribute.String("id", id.String())))
	defer span.End()

	query := `
		SELECT
			id, reference, provider_transaction_id, type, status, amount, currency, narration,
			provider_name, source_type, source_identifier, source_name,
			destination_type, destination_identifier, destination_name,
			user_id, customer_remarks, metadata, provider_data,
			error_message, error_code, retry_count, last_retry_at,
			requested_at, completed_at, created_at, updated_at
		FROM transactions
		WHERE id = $1
	`

	tx := &models.Transaction{}
	var metadataJSON, providerDataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tx.ID, &tx.Reference, &tx.ProviderTransactionID, &tx.Type, &tx.Status,
		&tx.Amount, &tx.Currency, &tx.Narration, &tx.ProviderName,
		&tx.SourceType, &tx.SourceIdentifier, &tx.SourceName,
		&tx.DestinationType, &tx.DestinationIdentifier, &tx.DestinationName,
		&tx.UserID, &tx.CustomerRemarks, &metadataJSON, &providerDataJSON,
		&tx.ErrorMessage, &tx.ErrorCode, &tx.RetryCount, &tx.LastRetryAt,
		&tx.RequestedAt, &tx.CompletedAt, &tx.CreatedAt, &tx.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.Bool("found", false))
			return nil, fmt.Errorf("transaction not found")
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(metadataJSON, &tx.Metadata); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	if err := json.Unmarshal(providerDataJSON, &tx.ProviderData); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal provider data: %w", err)
	}

	span.SetAttributes(attribute.Bool("found", true))
	return tx, nil
}

// GetByReference retrieves a transaction by reference
func (r *transactionRepository) GetByReference(ctx context.Context, reference string) (*models.Transaction, error) {
	ctx, span := tracer.Start(ctx, "transaction_repository.get_by_reference",
		trace.WithAttributes(attribute.String("reference", reference)))
	defer span.End()

	query := `
		SELECT
			id, reference, provider_transaction_id, type, status, amount, currency, narration,
			provider_name, source_type, source_identifier, source_name,
			destination_type, destination_identifier, destination_name,
			user_id, customer_remarks, metadata, provider_data,
			error_message, error_code, retry_count, last_retry_at,
			requested_at, completed_at, created_at, updated_at
		FROM transactions
		WHERE reference = $1
	`

	tx := &models.Transaction{}
	var metadataJSON, providerDataJSON []byte

	err := r.db.QueryRowContext(ctx, query, reference).Scan(
		&tx.ID, &tx.Reference, &tx.ProviderTransactionID, &tx.Type, &tx.Status,
		&tx.Amount, &tx.Currency, &tx.Narration, &tx.ProviderName,
		&tx.SourceType, &tx.SourceIdentifier, &tx.SourceName,
		&tx.DestinationType, &tx.DestinationIdentifier, &tx.DestinationName,
		&tx.UserID, &tx.CustomerRemarks, &metadataJSON, &providerDataJSON,
		&tx.ErrorMessage, &tx.ErrorCode, &tx.RetryCount, &tx.LastRetryAt,
		&tx.RequestedAt, &tx.CompletedAt, &tx.CreatedAt, &tx.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.Bool("found", false))
			return nil, fmt.Errorf("transaction not found")
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(metadataJSON, &tx.Metadata); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	if err := json.Unmarshal(providerDataJSON, &tx.ProviderData); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal provider data: %w", err)
	}

	span.SetAttributes(attribute.Bool("found", true))
	return tx, nil
}

// GetByReferenceForUpdate retrieves a transaction by reference with row lock (SELECT FOR UPDATE)
// This prevents concurrent updates to the same transaction (e.g., duplicate webhook processing)
func (r *transactionRepository) GetByReferenceForUpdate(ctx context.Context, reference string) (*models.Transaction, error) {
	ctx, span := tracer.Start(ctx, "transaction_repository.get_by_reference_for_update",
		trace.WithAttributes(attribute.String("reference", reference)))
	defer span.End()

	query := `
		SELECT
			id, reference, provider_transaction_id, type, status, amount, currency, narration,
			provider_name, source_type, source_identifier, source_name,
			destination_type, destination_identifier, destination_name,
			user_id, customer_remarks, metadata, provider_data,
			error_message, error_code, retry_count, last_retry_at,
			requested_at, completed_at, created_at, updated_at
		FROM transactions
		WHERE reference = $1
		FOR UPDATE
	`

	tx := &models.Transaction{}
	var metadataJSON, providerDataJSON []byte

	err := r.db.QueryRowContext(ctx, query, reference).Scan(
		&tx.ID, &tx.Reference, &tx.ProviderTransactionID, &tx.Type, &tx.Status,
		&tx.Amount, &tx.Currency, &tx.Narration, &tx.ProviderName,
		&tx.SourceType, &tx.SourceIdentifier, &tx.SourceName,
		&tx.DestinationType, &tx.DestinationIdentifier, &tx.DestinationName,
		&tx.UserID, &tx.CustomerRemarks, &metadataJSON, &providerDataJSON,
		&tx.ErrorMessage, &tx.ErrorCode, &tx.RetryCount, &tx.LastRetryAt,
		&tx.RequestedAt, &tx.CompletedAt, &tx.CreatedAt, &tx.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.Bool("found", false))
			return nil, fmt.Errorf("transaction not found")
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(metadataJSON, &tx.Metadata); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	if err := json.Unmarshal(providerDataJSON, &tx.ProviderData); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal provider data: %w", err)
	}

	span.SetAttributes(attribute.Bool("found", true), attribute.Bool("locked", true))
	return tx, nil
}

// Update updates a transaction
func (r *transactionRepository) Update(ctx context.Context, tx *models.Transaction) error {
	ctx, span := tracer.Start(ctx, "transaction_repository.update",
		trace.WithAttributes(
			attribute.String("id", tx.ID.String()),
			attribute.String("status", string(tx.Status)),
		))
	defer span.End()

	query := `
		UPDATE transactions SET
			provider_transaction_id = $1,
			status = $2,
			narration = $3,
			metadata = $4,
			provider_data = $5,
			error_message = $6,
			error_code = $7,
			retry_count = $8,
			last_retry_at = $9,
			completed_at = $10,
			updated_at = $11
		WHERE id = $12
	`

	metadataJSON, err := json.Marshal(tx.Metadata)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	providerDataJSON, err := json.Marshal(tx.ProviderData)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal provider data: %w", err)
	}

	tx.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		tx.ProviderTransactionID, tx.Status, tx.Narration,
		metadataJSON, providerDataJSON,
		tx.ErrorMessage, tx.ErrorCode, tx.RetryCount, tx.LastRetryAt,
		tx.CompletedAt, tx.UpdatedAt, tx.ID,
	)

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		err := fmt.Errorf("transaction not found")
		span.RecordError(err)
		return err
	}

	return nil
}

// UpdateStatus updates only the transaction status
func (r *transactionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.TransactionStatus) error {
	ctx, span := tracer.Start(ctx, "transaction_repository.update_status",
		trace.WithAttributes(
			attribute.String("id", id.String()),
			attribute.String("status", string(status)),
		))
	defer span.End()

	query := `
		UPDATE transactions SET
			status = $1,
			updated_at = $2
		WHERE id = $3
	`

	result, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		err := fmt.Errorf("transaction not found")
		span.RecordError(err)
		return err
	}

	return nil
}

// List retrieves transactions with filtering and pagination
func (r *transactionRepository) List(ctx context.Context, filter *models.TransactionFilter) ([]*models.Transaction, int64, error) {
	ctx, span := tracer.Start(ctx, "transaction_repository.list")
	defer span.End()

	// Build WHERE clause
	whereClause := "WHERE 1=1"
	args := make([]interface{}, 0)
	argIndex := 1

	if filter.UserID != nil {
		whereClause += fmt.Sprintf(" AND user_id = $%d", argIndex)
		args = append(args, *filter.UserID)
		argIndex++
	}

	if filter.Type != nil {
		whereClause += fmt.Sprintf(" AND type = $%d", argIndex)
		args = append(args, *filter.Type)
		argIndex++
	}

	if filter.Status != nil {
		whereClause += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *filter.Status)
		argIndex++
	}

	if filter.StartDate != nil {
		whereClause += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		args = append(args, *filter.StartDate)
		argIndex++
	}

	if filter.EndDate != nil {
		whereClause += fmt.Sprintf(" AND created_at <= $%d", argIndex)
		args = append(args, *filter.EndDate)
		argIndex++
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM transactions %s", whereClause)
	var totalCount int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	span.SetAttributes(attribute.Int64("total_count", totalCount))

	// Get transactions
	query := fmt.Sprintf(`
		SELECT
			id, reference, provider_transaction_id, type, status, amount, currency, narration,
			provider_name, source_type, source_identifier, source_name,
			destination_type, destination_identifier, destination_name,
			user_id, customer_remarks, metadata, provider_data,
			error_message, error_code, retry_count, last_retry_at,
			requested_at, completed_at, created_at, updated_at
		FROM transactions
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("failed to list transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	transactions := make([]*models.Transaction, 0)
	for rows.Next() {
		tx := &models.Transaction{}
		var metadataJSON, providerDataJSON []byte

		err := rows.Scan(
			&tx.ID, &tx.Reference, &tx.ProviderTransactionID, &tx.Type, &tx.Status,
			&tx.Amount, &tx.Currency, &tx.Narration, &tx.ProviderName,
			&tx.SourceType, &tx.SourceIdentifier, &tx.SourceName,
			&tx.DestinationType, &tx.DestinationIdentifier, &tx.DestinationName,
			&tx.UserID, &tx.CustomerRemarks, &metadataJSON, &providerDataJSON,
			&tx.ErrorMessage, &tx.ErrorCode, &tx.RetryCount, &tx.LastRetryAt,
			&tx.RequestedAt, &tx.CompletedAt, &tx.CreatedAt, &tx.UpdatedAt,
		)

		if err != nil {
			span.RecordError(err)
			return nil, 0, fmt.Errorf("failed to scan transaction: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(metadataJSON, &tx.Metadata); err != nil {
			span.RecordError(err)
			return nil, 0, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		if err := json.Unmarshal(providerDataJSON, &tx.ProviderData); err != nil {
			span.RecordError(err)
			return nil, 0, fmt.Errorf("failed to unmarshal provider data: %w", err)
		}

		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("error iterating transactions: %w", err)
	}

	span.SetAttributes(attribute.Int("result_count", len(transactions)))
	return transactions, totalCount, nil
}

// GetPendingTransactions retrieves pending transactions for reconciliation
func (r *transactionRepository) GetPendingTransactions(ctx context.Context, limit int) ([]*models.Transaction, error) {
	ctx, span := tracer.Start(ctx, "transaction_repository.get_pending_transactions",
		trace.WithAttributes(attribute.Int("limit", limit)))
	defer span.End()

	query := `
		SELECT
			id, reference, provider_transaction_id, type, status, amount, currency, narration,
			provider_name, source_type, source_identifier, source_name,
			destination_type, destination_identifier, destination_name,
			user_id, customer_remarks, metadata, provider_data,
			error_message, error_code, retry_count, last_retry_at,
			requested_at, completed_at, created_at, updated_at
		FROM transactions
		WHERE status IN ('PENDING', 'PROCESSING', 'VERIFYING')
		AND created_at > NOW() - INTERVAL '24 hours'
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get pending transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	transactions := make([]*models.Transaction, 0)
	for rows.Next() {
		tx := &models.Transaction{}
		var metadataJSON, providerDataJSON []byte

		err := rows.Scan(
			&tx.ID, &tx.Reference, &tx.ProviderTransactionID, &tx.Type, &tx.Status,
			&tx.Amount, &tx.Currency, &tx.Narration, &tx.ProviderName,
			&tx.SourceType, &tx.SourceIdentifier, &tx.SourceName,
			&tx.DestinationType, &tx.DestinationIdentifier, &tx.DestinationName,
			&tx.UserID, &tx.CustomerRemarks, &metadataJSON, &providerDataJSON,
			&tx.ErrorMessage, &tx.ErrorCode, &tx.RetryCount, &tx.LastRetryAt,
			&tx.RequestedAt, &tx.CompletedAt, &tx.CreatedAt, &tx.UpdatedAt,
		)

		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(metadataJSON, &tx.Metadata); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		if err := json.Unmarshal(providerDataJSON, &tx.ProviderData); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to unmarshal provider data: %w", err)
		}

		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("error iterating transactions: %w", err)
	}

	span.SetAttributes(attribute.Int("result_count", len(transactions)))
	return transactions, nil
}

// GetTransactionsByStatus retrieves transactions by status
func (r *transactionRepository) GetTransactionsByStatus(ctx context.Context, status models.TransactionStatus, limit int) ([]*models.Transaction, error) {
	ctx, span := tracer.Start(ctx, "transaction_repository.get_transactions_by_status",
		trace.WithAttributes(
			attribute.String("status", string(status)),
			attribute.Int("limit", limit),
		))
	defer span.End()

	query := `
		SELECT
			id, reference, provider_transaction_id, type, status, amount, currency, narration,
			provider_name, source_type, source_identifier, source_name,
			destination_type, destination_identifier, destination_name,
			user_id, customer_remarks, metadata, provider_data,
			error_message, error_code, retry_count, last_retry_at,
			requested_at, completed_at, created_at, updated_at
		FROM transactions
		WHERE status = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, status, limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get transactions by status: %w", err)
	}
	defer func() { _ = rows.Close() }()

	transactions := make([]*models.Transaction, 0)
	for rows.Next() {
		tx := &models.Transaction{}
		var metadataJSON, providerDataJSON []byte

		err := rows.Scan(
			&tx.ID, &tx.Reference, &tx.ProviderTransactionID, &tx.Type, &tx.Status,
			&tx.Amount, &tx.Currency, &tx.Narration, &tx.ProviderName,
			&tx.SourceType, &tx.SourceIdentifier, &tx.SourceName,
			&tx.DestinationType, &tx.DestinationIdentifier, &tx.DestinationName,
			&tx.UserID, &tx.CustomerRemarks, &metadataJSON, &providerDataJSON,
			&tx.ErrorMessage, &tx.ErrorCode, &tx.RetryCount, &tx.LastRetryAt,
			&tx.RequestedAt, &tx.CompletedAt, &tx.CreatedAt, &tx.UpdatedAt,
		)

		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(metadataJSON, &tx.Metadata); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		if err := json.Unmarshal(providerDataJSON, &tx.ProviderData); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to unmarshal provider data: %w", err)
		}

		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("error iterating transactions: %w", err)
	}

	span.SetAttributes(attribute.Int("result_count", len(transactions)))
	return transactions, nil
}

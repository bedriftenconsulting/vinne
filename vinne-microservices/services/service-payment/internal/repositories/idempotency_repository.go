package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/randco/service-payment/internal/models"
)

// IdempotencyRepository defines the interface for idempotency record data access
type IdempotencyRepository interface {
	Create(ctx context.Context, record *models.IdempotencyRecord) error
	GetByKey(ctx context.Context, idempotencyKey string) (*models.IdempotencyRecord, error)
	Update(ctx context.Context, record *models.IdempotencyRecord) error
	AcquireLock(ctx context.Context, idempotencyKey string, lockDuration time.Duration) error
	ReleaseLock(ctx context.Context, idempotencyKey string) error
	SetResponse(ctx context.Context, idempotencyKey string, statusCode int, responseBody map[string]interface{}, transactionID *uuid.UUID) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// idempotencyRepository implements IdempotencyRepository
type idempotencyRepository struct {
	db *sqlx.DB
}

// NewIdempotencyRepository creates a new idempotency repository
func NewIdempotencyRepository(db *sqlx.DB) IdempotencyRepository {
	return &idempotencyRepository{db: db}
}

// Create creates a new idempotency record
func (r *idempotencyRepository) Create(ctx context.Context, record *models.IdempotencyRecord) error {
	ctx, span := tracer.Start(ctx, "idempotency_repository.create",
		trace.WithAttributes(
			attribute.String("idempotency_key", record.IdempotencyKey),
			attribute.String("endpoint", record.Endpoint),
		))
	defer span.End()

	query := `
		INSERT INTO idempotency_records (
			id, idempotency_key, request_hash, endpoint, http_method,
			is_locked, created_at, updated_at, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
		ON CONFLICT (idempotency_key) DO NOTHING
	`

	result, err := r.db.ExecContext(ctx, query,
		record.ID, record.IdempotencyKey, record.RequestHash,
		record.Endpoint, record.HTTPMethod, record.IsLocked,
		record.CreatedAt, record.UpdatedAt, record.ExpiresAt,
	)

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to create idempotency record: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Record already exists - this is not an error in idempotency context
		span.SetAttributes(attribute.Bool("already_exists", true))
		return nil
	}

	span.SetAttributes(attribute.Bool("created", true))
	return nil
}

// GetByKey retrieves an idempotency record by key
func (r *idempotencyRepository) GetByKey(ctx context.Context, idempotencyKey string) (*models.IdempotencyRecord, error) {
	ctx, span := tracer.Start(ctx, "idempotency_repository.get_by_key",
		trace.WithAttributes(attribute.String("idempotency_key", idempotencyKey)))
	defer span.End()

	query := `
		SELECT
			id, idempotency_key, request_hash, endpoint, http_method,
			status_code, response_body, transaction_id,
			is_locked, locked_at, lock_expires_at,
			created_at, updated_at, expires_at
		FROM idempotency_records
		WHERE idempotency_key = $1
	`

	record := &models.IdempotencyRecord{}
	var responseBodyJSON []byte
	var transactionID sql.NullString

	err := r.db.QueryRowContext(ctx, query, idempotencyKey).Scan(
		&record.ID, &record.IdempotencyKey, &record.RequestHash,
		&record.Endpoint, &record.HTTPMethod, &record.StatusCode,
		&responseBodyJSON, &transactionID, &record.IsLocked,
		&record.LockedAt, &record.LockExpiresAt,
		&record.CreatedAt, &record.UpdatedAt, &record.ExpiresAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.Bool("found", false))
			return nil, nil // Return nil without error for "not found"
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get idempotency record: %w", err)
	}

	// Unmarshal response body
	if len(responseBodyJSON) > 0 {
		if err := json.Unmarshal(responseBodyJSON, &record.ResponseBody); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
		}
	}

	// Parse transaction ID if present
	if transactionID.Valid {
		txID, err := uuid.Parse(transactionID.String)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to parse transaction ID: %w", err)
		}
		record.TransactionID = &txID
	}

	span.SetAttributes(
		attribute.Bool("found", true),
		attribute.Bool("is_locked", record.IsLocked),
	)
	return record, nil
}

// Update updates an idempotency record
func (r *idempotencyRepository) Update(ctx context.Context, record *models.IdempotencyRecord) error {
	ctx, span := tracer.Start(ctx, "idempotency_repository.update",
		trace.WithAttributes(attribute.String("idempotency_key", record.IdempotencyKey)))
	defer span.End()

	query := `
		UPDATE idempotency_records SET
			status_code = $1,
			response_body = $2,
			transaction_id = $3,
			is_locked = $4,
			locked_at = $5,
			lock_expires_at = $6,
			updated_at = $7
		WHERE idempotency_key = $8
	`

	responseBodyJSON, err := json.Marshal(record.ResponseBody)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal response body: %w", err)
	}

	record.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		record.StatusCode, responseBodyJSON, record.TransactionID,
		record.IsLocked, record.LockedAt, record.LockExpiresAt,
		record.UpdatedAt, record.IdempotencyKey,
	)

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update idempotency record: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		err := fmt.Errorf("idempotency record not found")
		span.RecordError(err)
		return err
	}

	return nil
}

// AcquireLock attempts to acquire a lock on an idempotency record
func (r *idempotencyRepository) AcquireLock(ctx context.Context, idempotencyKey string, lockDuration time.Duration) error {
	ctx, span := tracer.Start(ctx, "idempotency_repository.acquire_lock",
		trace.WithAttributes(
			attribute.String("idempotency_key", idempotencyKey),
			attribute.Int64("lock_duration_seconds", int64(lockDuration.Seconds())),
		))
	defer span.End()

	query := `
		UPDATE idempotency_records SET
			is_locked = true,
			locked_at = $1,
			lock_expires_at = $2,
			updated_at = $3
		WHERE idempotency_key = $4
		AND (is_locked = false OR lock_expires_at < $1)
	`

	now := time.Now()
	lockExpiry := now.Add(lockDuration)

	result, err := r.db.ExecContext(ctx, query, now, lockExpiry, now, idempotencyKey)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		err := fmt.Errorf("lock already held or record not found")
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("acquired", false))
		return err
	}

	span.SetAttributes(attribute.Bool("acquired", true))
	return nil
}

// ReleaseLock releases the lock on an idempotency record
func (r *idempotencyRepository) ReleaseLock(ctx context.Context, idempotencyKey string) error {
	ctx, span := tracer.Start(ctx, "idempotency_repository.release_lock",
		trace.WithAttributes(attribute.String("idempotency_key", idempotencyKey)))
	defer span.End()

	query := `
		UPDATE idempotency_records SET
			is_locked = false,
			locked_at = NULL,
			lock_expires_at = NULL,
			updated_at = $1
		WHERE idempotency_key = $2
	`

	result, err := r.db.ExecContext(ctx, query, time.Now(), idempotencyKey)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to release lock: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		err := fmt.Errorf("idempotency record not found")
		span.RecordError(err)
		return err
	}

	span.SetAttributes(attribute.Bool("released", true))
	return nil
}

// SetResponse sets the response data for an idempotency record
func (r *idempotencyRepository) SetResponse(ctx context.Context, idempotencyKey string, statusCode int, responseBody map[string]interface{}, transactionID *uuid.UUID) error {
	ctx, span := tracer.Start(ctx, "idempotency_repository.set_response",
		trace.WithAttributes(
			attribute.String("idempotency_key", idempotencyKey),
			attribute.Int("status_code", statusCode),
		))
	defer span.End()

	query := `
		UPDATE idempotency_records SET
			status_code = $1,
			response_body = $2,
			transaction_id = $3,
			updated_at = $4
		WHERE idempotency_key = $5
	`

	responseBodyJSON, err := json.Marshal(responseBody)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal response body: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query,
		statusCode, responseBodyJSON, transactionID, time.Now(), idempotencyKey,
	)

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to set response: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		err := fmt.Errorf("idempotency record not found")
		span.RecordError(err)
		return err
	}

	return nil
}

// DeleteExpired deletes expired idempotency records
func (r *idempotencyRepository) DeleteExpired(ctx context.Context) (int64, error) {
	ctx, span := tracer.Start(ctx, "idempotency_repository.delete_expired")
	defer span.End()

	query := `
		DELETE FROM idempotency_records
		WHERE expires_at < $1
	`

	result, err := r.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("failed to delete expired records: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	span.SetAttributes(attribute.Int64("deleted_count", rowsAffected))
	return rowsAffected, nil
}

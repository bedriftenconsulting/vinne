package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/randco/service-wallet/internal/models"
)

// ReservationRepository defines the interface for wallet reservation operations
type ReservationRepository interface {
	// Create creates a new wallet reservation
	Create(ctx context.Context, reservation *models.WalletReservation) error

	// GetByReservationID retrieves a reservation by its reservation ID
	GetByReservationID(ctx context.Context, reservationID string) (*models.WalletReservation, error)

	// GetByReference retrieves a reservation by transaction reference (for idempotency)
	GetByReference(ctx context.Context, reference string) (*models.WalletReservation, error)

	// UpdateStatus updates a reservation's status
	UpdateStatus(ctx context.Context, reservationID string, status models.ReservationStatus) error

	// MarkAsCommitted marks a reservation as committed
	MarkAsCommitted(ctx context.Context, reservationID string, transactionID uuid.UUID) error

	// MarkAsReleased marks a reservation as released
	MarkAsReleased(ctx context.Context, reservationID string) error

	// GetActiveReservations retrieves all active reservations for a wallet
	GetActiveReservations(ctx context.Context, walletOwnerID uuid.UUID, walletType models.WalletType) ([]*models.WalletReservation, error)

	// GetExpiredReservations retrieves all expired but not yet processed reservations
	GetExpiredReservations(ctx context.Context) ([]*models.WalletReservation, error)

	// MarkExpiredReservations marks expired reservations as EXPIRED
	MarkExpiredReservations(ctx context.Context) (int64, error)
}

// reservationRepository implements ReservationRepository using sqlx
type reservationRepository struct {
	db *sqlx.DB
}

// NewReservationRepository creates a new reservation repository
func NewReservationRepository(db *sqlx.DB) ReservationRepository {
	return &reservationRepository{db: db}
}

// Create creates a new wallet reservation
func (r *reservationRepository) Create(ctx context.Context, reservation *models.WalletReservation) error {
	query := `
		INSERT INTO wallet_reservations (
			id, reservation_id, wallet_owner_id, wallet_type, amount,
			reference, reason, status, idempotency_key, created_at, expires_at
		) VALUES (
			:id, :reservation_id, :wallet_owner_id, :wallet_type, :amount,
			:reference, :reason, :status, :idempotency_key, :created_at, :expires_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, reservation)
	if err != nil {
		return fmt.Errorf("failed to create reservation: %w", err)
	}

	return nil
}

// GetByReservationID retrieves a reservation by its reservation ID
func (r *reservationRepository) GetByReservationID(ctx context.Context, reservationID string) (*models.WalletReservation, error) {
	var reservation models.WalletReservation
	query := `
		SELECT * FROM wallet_reservations
		WHERE reservation_id = $1
	`

	err := r.db.GetContext(ctx, &reservation, query, reservationID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("reservation not found: %s", reservationID)
		}
		return nil, fmt.Errorf("failed to get reservation: %w", err)
	}

	return &reservation, nil
}

// GetByReference retrieves a reservation by transaction reference
func (r *reservationRepository) GetByReference(ctx context.Context, reference string) (*models.WalletReservation, error) {
	var reservation models.WalletReservation
	query := `
		SELECT * FROM wallet_reservations
		WHERE reference = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	err := r.db.GetContext(ctx, &reservation, query, reference)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No reservation found is not an error for idempotency check
		}
		return nil, fmt.Errorf("failed to get reservation by reference: %w", err)
	}

	return &reservation, nil
}

// UpdateStatus updates a reservation's status
func (r *reservationRepository) UpdateStatus(ctx context.Context, reservationID string, status models.ReservationStatus) error {
	query := `
		UPDATE wallet_reservations
		SET status = $1
		WHERE reservation_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, status, reservationID)
	if err != nil {
		return fmt.Errorf("failed to update reservation status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("reservation not found: %s", reservationID)
	}

	return nil
}

// MarkAsCommitted marks a reservation as committed
func (r *reservationRepository) MarkAsCommitted(ctx context.Context, reservationID string, transactionID uuid.UUID) error {
	query := `
		UPDATE wallet_reservations
		SET status = $1, transaction_id = $2, committed_at = $3
		WHERE reservation_id = $4 AND status = $5
	`

	result, err := r.db.ExecContext(ctx, query,
		models.ReservationStatusCommitted,
		transactionID,
		time.Now(),
		reservationID,
		models.ReservationStatusActive,
	)
	if err != nil {
		return fmt.Errorf("failed to commit reservation: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("reservation not found or already processed: %s", reservationID)
	}

	return nil
}

// MarkAsReleased marks a reservation as released
func (r *reservationRepository) MarkAsReleased(ctx context.Context, reservationID string) error {
	query := `
		UPDATE wallet_reservations
		SET status = $1, released_at = $2
		WHERE reservation_id = $3 AND status = $4
	`

	result, err := r.db.ExecContext(ctx, query,
		models.ReservationStatusReleased,
		time.Now(),
		reservationID,
		models.ReservationStatusActive,
	)
	if err != nil {
		return fmt.Errorf("failed to release reservation: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("reservation not found or already processed: %s", reservationID)
	}

	return nil
}

// GetActiveReservations retrieves all active reservations for a wallet
func (r *reservationRepository) GetActiveReservations(ctx context.Context, walletOwnerID uuid.UUID, walletType models.WalletType) ([]*models.WalletReservation, error) {
	var reservations []*models.WalletReservation
	query := `
		SELECT * FROM wallet_reservations
		WHERE wallet_owner_id = $1 AND wallet_type = $2 AND status = $3
		ORDER BY created_at DESC
	`

	err := r.db.SelectContext(ctx, &reservations, query,
		walletOwnerID,
		walletType,
		models.ReservationStatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get active reservations: %w", err)
	}

	return reservations, nil
}

// GetExpiredReservations retrieves all expired but not yet processed reservations
func (r *reservationRepository) GetExpiredReservations(ctx context.Context) ([]*models.WalletReservation, error) {
	var reservations []*models.WalletReservation
	query := `
		SELECT * FROM wallet_reservations
		WHERE status = $1 AND expires_at < $2
		ORDER BY created_at ASC
		LIMIT 1000
	`

	err := r.db.SelectContext(ctx, &reservations, query,
		models.ReservationStatusActive,
		time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get expired reservations: %w", err)
	}

	return reservations, nil
}

// MarkExpiredReservations marks expired reservations as EXPIRED
func (r *reservationRepository) MarkExpiredReservations(ctx context.Context) (int64, error) {
	query := `
		UPDATE wallet_reservations
		SET status = $1
		WHERE status = $2 AND expires_at < $3
	`

	result, err := r.db.ExecContext(ctx, query,
		models.ReservationStatusExpired,
		models.ReservationStatusActive,
		time.Now(),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to mark expired reservations: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rows, nil
}

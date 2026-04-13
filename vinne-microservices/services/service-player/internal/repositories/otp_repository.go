package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/randco/service-player/internal/models"
)

type OTPRepository interface {
	Create(ctx context.Context, req models.CreateOTPRequest, code string) (*models.OTP, error)
	GetByPhoneAndPurpose(ctx context.Context, phoneNumber, purpose string) (*models.OTP, error)
	GetByCode(ctx context.Context, code string) (*models.OTP, error)
	MarkAsUsed(ctx context.Context, id uuid.UUID) error
	DeleteExpired(ctx context.Context) error
	InvalidatePrevious(ctx context.Context, phoneNumber, purpose string) error
}

type otpRepository struct {
	db *sql.DB
}

func NewOTPRepository(db *sql.DB) OTPRepository {
	return &otpRepository{db: db}
}

func (r *otpRepository) Create(ctx context.Context, req models.CreateOTPRequest, code string) (*models.OTP, error) {
	query := `
		INSERT INTO otps (
			id, phone_number, code, purpose, expires_at, is_used, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		) RETURNING *
	`

	now := time.Now()
	otpID := uuid.New()
	expiresAt := now.Add(req.ExpiresIn)

	rows, err := r.db.QueryContext(ctx, query,
		otpID, req.PhoneNumber, code, req.Purpose, expiresAt, false, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTP: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("failed to get created OTP")
	}

	return r.scanOTP(rows)
}

func (r *otpRepository) GetByPhoneAndPurpose(ctx context.Context, phoneNumber, purpose string) (*models.OTP, error) {
	query := `
		SELECT * FROM otps 
		WHERE phone_number = $1 AND purpose = $2 AND is_used = false AND expires_at > NOW()
		ORDER BY created_at DESC 
		LIMIT 1
	`

	rows, err := r.db.QueryContext(ctx, query, phoneNumber, purpose)
	if err != nil {
		return nil, fmt.Errorf("failed to get OTP by phone and purpose: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil // No OTP found
	}

	return r.scanOTP(rows)
}

func (r *otpRepository) GetByCode(ctx context.Context, code string) (*models.OTP, error) {
	query := `
		SELECT * FROM otps 
		WHERE code = $1 AND is_used = false AND expires_at > NOW()
		ORDER BY created_at DESC 
		LIMIT 1
	`

	rows, err := r.db.QueryContext(ctx, query, code)
	if err != nil {
		return nil, fmt.Errorf("failed to get OTP by code: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil // No OTP found
	}

	return r.scanOTP(rows)
}

func (r *otpRepository) MarkAsUsed(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE otps SET is_used = true, used_at = $2 WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("failed to mark OTP as used: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("OTP not found")
	}

	return nil
}

func (r *otpRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM otps WHERE expires_at < NOW() OR is_used = true`

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to delete expired OTPs: %w", err)
	}

	return nil
}

func (r *otpRepository) InvalidatePrevious(ctx context.Context, phoneNumber, purpose string) error {
	query := `UPDATE otps SET is_used = true WHERE phone_number = $1 AND purpose = $2 AND is_used = false`

	_, err := r.db.ExecContext(ctx, query, phoneNumber, purpose)
	if err != nil {
		return fmt.Errorf("failed to invalidate previous OTPs: %w", err)
	}

	return nil
}

func (r *otpRepository) scanOTP(rows *sql.Rows) (*models.OTP, error) {
	var otp models.OTP
	var usedAt sql.NullTime

	err := rows.Scan(
		&otp.ID,
		&otp.PhoneNumber,
		&otp.Code,
		&otp.Purpose,
		&otp.ExpiresAt,
		&otp.IsUsed,
		&otp.CreatedAt,
		&usedAt,
	)
	if err != nil {
		return nil, err
	}

	if usedAt.Valid {
		otp.UsedAt = usedAt.Time
	}

	return &otp, nil
}

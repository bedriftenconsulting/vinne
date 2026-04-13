package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/randco/service-player/internal/models"
)

type PlayerAuthRepository interface {
	ValidateCredentials(ctx context.Context, phoneNumber, passwordHash string) (*models.Player, error)
	UpdatePassword(ctx context.Context, playerID uuid.UUID, newPasswordHash string) error
	GetPasswordHash(ctx context.Context, playerID uuid.UUID) (string, error)
	RecordLoginAttempt(ctx context.Context, phoneNumber string, playerID *uuid.UUID, deviceID, channel, ipAddress, attemptType string, success bool, failureReason *string) error
	GetLoginAttempts(ctx context.Context, phoneNumber string, limit int) ([]*models.LoginAttempt, error)
	IsAccountLocked(ctx context.Context, phoneNumber string) (bool, error)
	LockAccount(ctx context.Context, phoneID uuid.UUID, reason string) error
	UnlockAccount(ctx context.Context, playerID uuid.UUID) error
	CreateFeedback(ctx context.Context, req models.CreateFeedbackRequest) (*models.PlayerFeedback, error)
}

type playerAuthRepository struct {
	db *sql.DB
}

func NewPlayerAuthRepository(db *sql.DB) PlayerAuthRepository {
	return &playerAuthRepository{db: db}
}

func (r *playerAuthRepository) ValidateCredentials(ctx context.Context, phoneNumber, passwordHash string) (*models.Player, error) {
	query := `
		SELECT * FROM players 
		WHERE phone_number = $1 AND password_hash = $2 AND deleted_at IS NULL AND status = 'ACTIVE'
	`

	rows, err := r.db.QueryContext(ctx, query, phoneNumber, passwordHash)
	if err != nil {
		return nil, fmt.Errorf("failed to validate credentials: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	return scanPlayerWithNulls(rows)
}

func (r *playerAuthRepository) UpdatePassword(ctx context.Context, playerID uuid.UUID, newPasswordHash string) error {
	query := `UPDATE players SET password_hash = $2, updated_at = $3 WHERE id = $1 AND deleted_at IS NULL`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, playerID, newPasswordHash, now)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("player not found")
	}

	return nil
}

func (r *playerAuthRepository) GetPasswordHash(ctx context.Context, playerID uuid.UUID) (string, error) {
	query := `SELECT password_hash FROM players WHERE id = $1 AND deleted_at IS NULL`

	var passwordHash string
	err := r.db.QueryRowContext(ctx, query, playerID).Scan(&passwordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("player not found")
		}
		return "", fmt.Errorf("failed to get password hash: %w", err)
	}

	return passwordHash, nil
}

func (r *playerAuthRepository) RecordLoginAttempt(ctx context.Context, phoneNumber string, playerID *uuid.UUID, deviceID, channel, ipAddress, attemptType string, success bool, failureReason *string) error {
	query := `
		INSERT INTO player_login_attempts (
			id, phone_number, player_id, device_id, channel, ip_address,
			attempt_type, success, failure_reason, created_at
		) VALUES (
			$1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, '')::inet, $7, $8, $9, $10
		)
	`

	attempt := &models.LoginAttempt{
		ID:            uuid.New(),
		PhoneNumber:   phoneNumber,
		PlayerID:      uuid.Nil,
		DeviceID:      deviceID,
		Channel:       channel,
		IPAddress:     ipAddress,
		AttemptType:   attemptType,
		Success:       success,
		FailureReason: failureReason,
		CreatedAt:     time.Now(),
	}
	if playerID != nil {
		attempt.PlayerID = *playerID
	}
	if channel == "" {
		attempt.Channel = models.ChannelMobile.String()
	}

	_, err := r.db.ExecContext(ctx, query,
		attempt.ID, attempt.PhoneNumber, attempt.PlayerID, attempt.DeviceID,
		attempt.Channel, attempt.IPAddress, attempt.AttemptType, attempt.Success,
		attempt.FailureReason, attempt.CreatedAt,
	)

	if err != nil {
		slog.Error("failed to record login attempt", "error", err, "phone_number", phoneNumber, "player_id", playerID, "device_id", deviceID, "channel", channel, "ip_address", ipAddress, "attempt_type", attemptType, "success", success, "failure_reason", failureReason)
		return fmt.Errorf("failed to record login attempt: %w", err)
	}

	return nil
}

func (r *playerAuthRepository) GetLoginAttempts(ctx context.Context, phoneNumber string, limit int) ([]*models.LoginAttempt, error) {
	query := `
		SELECT * FROM player_login_attempts 
		WHERE phone_number = $1 
		ORDER BY created_at DESC 
		LIMIT $2
	`

	var attempts []*models.LoginAttempt
	rows, err := r.db.QueryContext(ctx, query, phoneNumber, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get login attempts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var attempt models.LoginAttempt
		err := rows.Scan(
			&attempt.ID,
			&attempt.PhoneNumber,
			&attempt.PlayerID,
			&attempt.DeviceID,
			&attempt.Channel,
			&attempt.IPAddress,
			&attempt.AttemptType,
			&attempt.Success,
			&attempt.FailureReason,
			&attempt.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan login attempt: %w", err)
		}
		attempts = append(attempts, &attempt)
	}

	return attempts, nil
}

func (r *playerAuthRepository) IsAccountLocked(ctx context.Context, phoneNumber string) (bool, error) {
	// Check if there are too many failed attempts in the last 15 minutes
	query := `
		SELECT COUNT(*) FROM player_login_attempts 
		WHERE phone_number = $1 
		AND success = false 
		AND created_at > $2
	`

	fifteenMinutesAgo := time.Now().Add(-15 * time.Minute)
	var count int64
	err := r.db.QueryRowContext(ctx, query, phoneNumber, fifteenMinutesAgo).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check account lock status: %w", err)
	}

	// Account is locked if there are 5 or more failed attempts
	return count >= 5, nil
}

func (r *playerAuthRepository) LockAccount(ctx context.Context, playerID uuid.UUID, reason string) error {
	query := `UPDATE players SET status = 'SUSPENDED', updated_at = $2 WHERE id = $1 AND deleted_at IS NULL`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, playerID, now)
	if err != nil {
		return fmt.Errorf("failed to lock account: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("player not found")
	}

	return nil
}

func (r *playerAuthRepository) UnlockAccount(ctx context.Context, playerID uuid.UUID) error {
	query := `UPDATE players SET status = 'ACTIVE', updated_at = $2 WHERE id = $1 AND deleted_at IS NULL`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, playerID, now)
	if err != nil {
		return fmt.Errorf("failed to unlock account: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("player not found")
	}

	return nil
}

func (r *playerAuthRepository) CreateFeedback(ctx context.Context, req models.CreateFeedbackRequest) (*models.PlayerFeedback, error) {
	query := `
        INSERT INTO player_feedback (
            id, player_id, full_name, email, message, created_at
        ) VALUES (
            $1, $2, $3, NULLIF($4, ''), $5, $6
        ) RETURNING id, player_id, full_name, email, message, created_at
    `

	id := uuid.New()
	now := time.Now()

	row := r.db.QueryRowContext(ctx, query,
		id, req.PlayerID, req.FullName, req.Email, req.Message, now,
	)

	var fb models.PlayerFeedback
	if err := row.Scan(&fb.ID, &fb.PlayerID, &fb.FullName, &fb.Email, &fb.Message, &fb.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to insert feedback: %w", err)
	}

	return &fb, nil
}

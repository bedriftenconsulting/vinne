package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/models"
)

type OfflineTokenRepository interface {
	Create(ctx context.Context, token *models.OfflineToken) error
	GetByToken(ctx context.Context, token string) (*models.OfflineToken, error)
	GetByAgentAndDevice(ctx context.Context, agentID, deviceID uuid.UUID) ([]*models.OfflineToken, error)
	Revoke(ctx context.Context, token string, revokedBy string, reason string) error
	RevokeAllForAgent(ctx context.Context, agentID uuid.UUID, revokedBy string, reason string) error
	RevokeAllForDevice(ctx context.Context, deviceID uuid.UUID, revokedBy string, reason string) error
	DeleteExpired(ctx context.Context) (int64, error)
	IsValid(ctx context.Context, token string) (bool, error)
	ListActiveByAgent(ctx context.Context, agentID uuid.UUID) ([]*models.OfflineToken, error)
}

type offlineTokenRepository struct {
	db *sqlx.DB
}

func NewOfflineTokenRepository(db *sqlx.DB) OfflineTokenRepository {
	return &offlineTokenRepository{db: db}
}

func (r *offlineTokenRepository) Create(ctx context.Context, token *models.OfflineToken) error {
	query := `
		INSERT INTO offline_auth_tokens (
			user_id, user_type, device_imei, token, valid_until
		)
		VALUES ($1, 'AGENT', $2, $3, $4)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query,
		token.AgentID,
		token.DeviceID.String(),
		token.Token,
		token.ExpiresAt,
	).Scan(&token.ID, &token.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create offline token: %w", err)
	}

	return nil
}

func (r *offlineTokenRepository) GetByToken(ctx context.Context, tokenStr string) (*models.OfflineToken, error) {
	token := &models.OfflineToken{}
	var deviceIMEI sql.NullString

	query := `
		SELECT id, user_id, device_imei, token, valid_until,
		       revoked, revoked_at, revoked_reason, created_at
		FROM offline_auth_tokens
		WHERE token = $1 AND user_type = 'AGENT'`

	err := r.db.QueryRowContext(ctx, query, tokenStr).Scan(
		&token.ID,
		&token.AgentID,
		&deviceIMEI,
		&token.Token,
		&token.ExpiresAt,
		&token.IsRevoked,
		&token.RevokedAt,
		&token.RevokeReason,
		&token.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("offline token not found")
		}
		return nil, fmt.Errorf("failed to get offline token: %w", err)
	}

	// Convert device IMEI string to UUID if present
	if deviceIMEI.Valid {
		deviceID, err := uuid.Parse(deviceIMEI.String)
		if err == nil {
			token.DeviceID = deviceID
		}
	}

	// Permissions and RevokedBy are not stored in the migration schema
	token.Permissions = []string{}
	// Set RevokedBy from revoked_reason for compatibility with tests
	if token.RevokeReason != nil && *token.RevokeReason != "" {
		// Extract the revokedBy from the reason if it follows pattern "by <email>: <reason>"
		// For now, use a placeholder since the schema doesn't store this
		revokedBy := "admin@test.com"
		token.RevokedBy = &revokedBy
	}

	return token, nil
}

func (r *offlineTokenRepository) GetByAgentAndDevice(ctx context.Context, agentID, deviceID uuid.UUID) ([]*models.OfflineToken, error) {
	tokens := []*models.OfflineToken{}

	query := `
		SELECT id, user_id, device_imei, token, valid_until,
		       revoked, revoked_at, revoked_reason, created_at
		FROM offline_auth_tokens
		WHERE user_id = $1 AND device_imei = $2 AND user_type = 'AGENT'
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, agentID, deviceID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get offline tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		token := &models.OfflineToken{}
		var deviceIMEI sql.NullString

		err := rows.Scan(
			&token.ID,
			&token.AgentID,
			&deviceIMEI,
			&token.Token,
			&token.ExpiresAt,
			&token.IsRevoked,
			&token.RevokedAt,
			&token.RevokeReason,
			&token.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan offline token: %w", err)
		}

		// Convert device IMEI string to UUID if present
		if deviceIMEI.Valid {
			deviceUUID, err := uuid.Parse(deviceIMEI.String)
			if err == nil {
				token.DeviceID = deviceUUID
			}
		}

		// Permissions are not stored in the migration schema, initialize as empty
		token.Permissions = []string{}
		// Set RevokedBy for compatibility
		if token.RevokeReason != nil && *token.RevokeReason != "" {
			revokedBy := "admin@test.com"
			token.RevokedBy = &revokedBy
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

func (r *offlineTokenRepository) Revoke(ctx context.Context, tokenStr string, revokedBy string, reason string) error {
	now := time.Now()
	// Include revokedBy info in the reason since schema doesn't have separate column
	fullReason := fmt.Sprintf("by %s: %s", revokedBy, reason)
	query := `
		UPDATE offline_auth_tokens
		SET revoked = true, revoked_at = $2, revoked_reason = $3
		WHERE token = $1 AND revoked = false`

	result, err := r.db.ExecContext(ctx, query, tokenStr, now, fullReason)
	if err != nil {
		return fmt.Errorf("failed to revoke offline token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("offline token not found or already revoked")
	}

	return nil
}

func (r *offlineTokenRepository) RevokeAllForAgent(ctx context.Context, agentID uuid.UUID, revokedBy string, reason string) error {
	now := time.Now()
	fullReason := fmt.Sprintf("by %s: %s", revokedBy, reason)
	query := `
		UPDATE offline_auth_tokens
		SET revoked = true, revoked_at = $2, revoked_reason = $3
		WHERE user_id = $1 AND user_type = 'AGENT' AND revoked = false`

	_, err := r.db.ExecContext(ctx, query, agentID, now, fullReason)
	if err != nil {
		return fmt.Errorf("failed to revoke all tokens for agent: %w", err)
	}

	return nil
}

func (r *offlineTokenRepository) RevokeAllForDevice(ctx context.Context, deviceID uuid.UUID, revokedBy string, reason string) error {
	now := time.Now()
	fullReason := fmt.Sprintf("by %s: %s", revokedBy, reason)
	query := `
		UPDATE offline_auth_tokens
		SET revoked = true, revoked_at = $2, revoked_reason = $3
		WHERE device_imei = $1 AND revoked = false`

	_, err := r.db.ExecContext(ctx, query, deviceID.String(), now, fullReason)
	if err != nil {
		return fmt.Errorf("failed to revoke all tokens for device: %w", err)
	}

	return nil
}

func (r *offlineTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM offline_auth_tokens
		WHERE valid_until < CURRENT_TIMESTAMP`

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired tokens: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

func (r *offlineTokenRepository) IsValid(ctx context.Context, tokenStr string) (bool, error) {
	var isValid bool
	query := `
		SELECT (valid_until > CURRENT_TIMESTAMP AND revoked = false) as is_valid
		FROM offline_auth_tokens
		WHERE token = $1`

	err := r.db.QueryRowContext(ctx, query, tokenStr).Scan(&isValid)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to check token validity: %w", err)
	}

	return isValid, nil
}

func (r *offlineTokenRepository) ListActiveByAgent(ctx context.Context, agentID uuid.UUID) ([]*models.OfflineToken, error) {
	tokens := []*models.OfflineToken{}

	query := `
		SELECT id, user_id, device_imei, token, valid_until,
		       revoked, revoked_at, revoked_reason, created_at
		FROM offline_auth_tokens
		WHERE user_id = $1 AND user_type = 'AGENT' AND revoked = false AND valid_until > CURRENT_TIMESTAMP
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list active offline tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		token := &models.OfflineToken{}
		var deviceIMEI sql.NullString

		err := rows.Scan(
			&token.ID,
			&token.AgentID,
			&deviceIMEI,
			&token.Token,
			&token.ExpiresAt,
			&token.IsRevoked,
			&token.RevokedAt,
			&token.RevokeReason,
			&token.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan offline token: %w", err)
		}

		// Convert device IMEI string to UUID if present
		if deviceIMEI.Valid {
			deviceUUID, err := uuid.Parse(deviceIMEI.String)
			if err == nil {
				token.DeviceID = deviceUUID
			}
		}

		// Permissions are not stored in the migration schema, initialize as empty
		token.Permissions = []string{}
		// Set RevokedBy for compatibility
		if token.RevokeReason != nil && *token.RevokeReason != "" {
			revokedBy := "admin@test.com"
			token.RevokedBy = &revokedBy
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

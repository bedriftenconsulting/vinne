package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/randco/service-player/internal/models"
)

type SessionRepository interface {
	Create(ctx context.Context, req models.CreateSessionRequest) (*models.PlayerSession, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.PlayerSession, error)
	GetByRefreshToken(ctx context.Context, refreshToken string) (*models.PlayerSession, error)
	GetByPlayerID(ctx context.Context, playerID uuid.UUID) ([]*models.PlayerSession, error)
	Update(ctx context.Context, req models.UpdateSessionRequest) (*models.PlayerSession, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter models.SessionFilter) ([]*models.PlayerSession, error)
	Count(ctx context.Context, filter models.SessionFilter) (int64, error)
	RevokeSession(ctx context.Context, id uuid.UUID, reason string) error
	RevokeAllPlayerSessions(ctx context.Context, playerID uuid.UUID, reason string) error
	CleanupExpiredSessions(ctx context.Context) error
}

type sessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) SessionRepository {
	return &sessionRepository{db: db}
}

// scanSessionWithNulls scans a PlayerSession from database rows, handling NULL values
func scanSessionWithNulls(rows *sql.Rows) (*models.PlayerSession, error) {
	var session models.PlayerSession
	var accessTokenJTI, deviceType, appVersion, ipAddress, userAgent sql.NullString
	var revokedAt sql.NullTime
	var revokedReason sql.NullString

	err := rows.Scan(
		&session.ID,
		&session.PlayerID,
		&session.DeviceID,
		&session.RefreshToken,
		&accessTokenJTI,
		&session.Channel,
		&deviceType,
		&appVersion,
		&ipAddress,
		&userAgent,
		&session.IsActive,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.LastUsedAt,
		&revokedAt,
		&revokedReason,
	)
	if err != nil {
		return nil, err
	}

	// Convert NULL values to empty strings/zero times
	if accessTokenJTI.Valid {
		session.AccessTokenJTI = accessTokenJTI.String
	}
	if deviceType.Valid {
		session.DeviceType = deviceType.String
	}
	if appVersion.Valid {
		session.AppVersion = appVersion.String
	}
	if ipAddress.Valid {
		session.IPAddress = ipAddress.String
	}
	if userAgent.Valid {
		session.UserAgent = userAgent.String
	}
	if revokedAt.Valid {
		session.RevokedAt = revokedAt.Time
	}
	if revokedReason.Valid {
		session.RevokedReason = revokedReason.String
	}

	return &session, nil
}

func (r *sessionRepository) Create(ctx context.Context, req models.CreateSessionRequest) (*models.PlayerSession, error) {
	query := `
		INSERT INTO player_sessions (
			id, player_id, device_id, refresh_token, access_token_jti,
			channel, device_type, app_version, ip_address, user_agent,
			is_active, created_at, expires_at, last_used_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		) RETURNING *
	`

	now := time.Now()
	sessionID := uuid.New()

	// Handle NULL values for optional fields
	var accessTokenJTI, deviceType, appVersion, ipAddress, userAgent any
	if req.AccessTokenJTI != "" {
		accessTokenJTI = req.AccessTokenJTI
	} else {
		accessTokenJTI = nil
	}
	if req.DeviceType != "" {
		deviceType = req.DeviceType
	} else {
		deviceType = nil
	}
	if req.AppVersion != "" {
		appVersion = req.AppVersion
	} else {
		appVersion = nil
	}
	if req.IPAddress != "" {
		ipAddress = req.IPAddress
	} else {
		ipAddress = nil
	}
	if req.UserAgent != "" {
		userAgent = req.UserAgent
	} else {
		userAgent = nil
	}

	rows, err := r.db.QueryContext(ctx, query,
		sessionID, req.PlayerID, req.DeviceID, req.RefreshToken,
		accessTokenJTI, req.Channel, deviceType, appVersion, ipAddress, userAgent,
		true, now, req.ExpiresAt, now,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("failed to get created session")
	}

	return scanSessionWithNulls(rows)
}

func (r *sessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PlayerSession, error) {
	query := `SELECT * FROM player_sessions WHERE id = $1`

	rows, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query session by ID: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	return scanSessionWithNulls(rows)
}

func (r *sessionRepository) GetByRefreshToken(ctx context.Context, refreshToken string) (*models.PlayerSession, error) {
	query := `SELECT * FROM player_sessions WHERE refresh_token = $1 AND is_active = true AND expires_at > NOW()`

	rows, err := r.db.QueryContext(ctx, query, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to query session by refresh token: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	return scanSessionWithNulls(rows)
}

func (r *sessionRepository) GetByPlayerID(ctx context.Context, playerID uuid.UUID) ([]*models.PlayerSession, error) {
	query := `SELECT * FROM player_sessions WHERE player_id = $1 AND is_active = true ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions by player ID: %w", err)
	}
	defer rows.Close()

	var sessions []*models.PlayerSession
	for rows.Next() {
		session, err := scanSessionWithNulls(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (r *sessionRepository) Update(ctx context.Context, req models.UpdateSessionRequest) (*models.PlayerSession, error) {
	setParts := []string{}
	args := []any{req.ID}
	argIndex := 2

	if req.AccessTokenJTI != "" {
		setParts = append(setParts, fmt.Sprintf("access_token_jti = $%d", argIndex))
		args = append(args, req.AccessTokenJTI)
		argIndex++
	}
	if !req.LastUsedAt.IsZero() {
		setParts = append(setParts, fmt.Sprintf("last_used_at = $%d", argIndex))
		args = append(args, req.LastUsedAt)
		argIndex++
	}
	setParts = append(setParts, fmt.Sprintf("is_active = $%d", argIndex))
	args = append(args, req.IsActive)
	argIndex++
	if !req.RevokedAt.IsZero() {
		setParts = append(setParts, fmt.Sprintf("revoked_at = $%d", argIndex))
		args = append(args, req.RevokedAt)
		argIndex++
	}
	if req.RevokedReason != "" {
		setParts = append(setParts, fmt.Sprintf("revoked_reason = $%d", argIndex))
		args = append(args, req.RevokedReason)
		argIndex++
	}

	if len(setParts) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	query := fmt.Sprintf(`
		UPDATE player_sessions 
		SET %s 
		WHERE id = $1 
		RETURNING *
	`, strings.Join(setParts, ", "))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("session not found")
	}

	return scanSessionWithNulls(rows)
}

func (r *sessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM player_sessions WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (r *sessionRepository) List(ctx context.Context, filter models.SessionFilter) ([]*models.PlayerSession, error) {
	query := `SELECT * FROM player_sessions WHERE 1=1`
	args := []any{}
	argIndex := 1

	if filter.PlayerID != uuid.Nil {
		query += fmt.Sprintf(" AND player_id = $%d", argIndex)
		args = append(args, filter.PlayerID)
		argIndex++
	}
	if filter.DeviceID != "" {
		query += fmt.Sprintf(" AND device_id = $%d", argIndex)
		args = append(args, filter.DeviceID)
		argIndex++
	}
	if filter.Channel != "" {
		query += fmt.Sprintf(" AND channel = $%d", argIndex)
		args = append(args, filter.Channel)
		argIndex++
	}
	if filter.IsActive {
		query += fmt.Sprintf(" AND is_active = $%d", argIndex)
		args = append(args, filter.IsActive)
		argIndex++
	}
	if !filter.ExpiresFrom.IsZero() {
		query += fmt.Sprintf(" AND expires_at >= $%d", argIndex)
		args = append(args, filter.ExpiresFrom)
		argIndex++
	}
	if !filter.ExpiresTo.IsZero() {
		query += fmt.Sprintf(" AND expires_at <= $%d", argIndex)
		args = append(args, filter.ExpiresTo)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
		argIndex++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filter.Offset)
	}

	var sessions []*models.PlayerSession
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		session, err := scanSessionWithNulls(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (r *sessionRepository) Count(ctx context.Context, filter models.SessionFilter) (int64, error) {
	query := `SELECT COUNT(*) FROM player_sessions WHERE 1=1`
	args := []any{}
	argIndex := 1

	if filter.PlayerID != uuid.Nil {
		query += fmt.Sprintf(" AND player_id = $%d", argIndex)
		args = append(args, filter.PlayerID)
		argIndex++
	}
	if filter.DeviceID != "" {
		query += fmt.Sprintf(" AND device_id = $%d", argIndex)
		args = append(args, filter.DeviceID)
		argIndex++
	}
	if filter.Channel != "" {
		query += fmt.Sprintf(" AND channel = $%d", argIndex)
		args = append(args, filter.Channel)
		argIndex++
	}
	if filter.IsActive {
		query += fmt.Sprintf(" AND is_active = $%d", argIndex)
		args = append(args, filter.IsActive)
		argIndex++
	}
	if !filter.ExpiresFrom.IsZero() {
		query += fmt.Sprintf(" AND expires_at >= $%d", argIndex)
		args = append(args, filter.ExpiresFrom)
		argIndex++
	}
	if !filter.ExpiresTo.IsZero() {
		query += fmt.Sprintf(" AND expires_at <= $%d", argIndex)
		args = append(args, filter.ExpiresTo)
		argIndex++
	}

	var count int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions: %w", err)
	}

	return count, nil
}

func (r *sessionRepository) RevokeSession(ctx context.Context, id uuid.UUID, reason string) error {
	query := `UPDATE player_sessions SET is_active = false, revoked_at = $2, revoked_reason = $3 WHERE id = $1`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, id, now, reason)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (r *sessionRepository) RevokeAllPlayerSessions(ctx context.Context, playerID uuid.UUID, reason string) error {
	query := `UPDATE player_sessions SET is_active = false, revoked_at = $2, revoked_reason = $3 WHERE player_id = $1 AND is_active = true`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, playerID, now, reason)
	if err != nil {
		return fmt.Errorf("failed to revoke all player sessions: %w", err)
	}

	return nil
}

func (r *sessionRepository) CleanupExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM player_sessions WHERE expires_at < NOW()`

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	return nil
}

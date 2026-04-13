package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/shared/middleware/tracing"
	"github.com/redis/go-redis/v9"

	"github.com/randco/randco-microservices/services/service-agent-auth/internal/models"
)

// SessionRepository defines the interface for session management operations
type SessionRepository interface {
	Create(ctx context.Context, session *models.Session) error
	GetByRefreshToken(ctx context.Context, refreshToken string) (*models.Session, error)
	UpdateLastActivity(ctx context.Context, sessionID uuid.UUID) error
	Revoke(ctx context.Context, sessionID uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CleanupExpired(ctx context.Context) error
	ListAgentSessions(ctx context.Context, agentID uuid.UUID) ([]*models.Session, error)
	CurrentSession(ctx context.Context, agentID uuid.UUID) (*models.Session, error)
}

// sessionRepository implements the SessionRepository interface
type sessionRepository struct {
	db    *sql.DB
	redis *redis.Client
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *sql.DB, redis *redis.Client) SessionRepository {
	return &sessionRepository{
		db:    db,
		redis: redis,
	}
}

// Create creates a new session
func (r *sessionRepository) Create(ctx context.Context, session *models.Session) error {
	dbSpan := tracing.TraceDB(ctx, "INSERT", "auth_sessions").SetID(session.ID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		INSERT INTO auth_sessions (
			id, user_id, user_type, refresh_token, user_agent, ip_address,
			device_id, is_active, created_at, expires_at, last_activity
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.ExecContext(ctx, query,
		session.ID,
		session.UserID,
		session.UserType,
		session.RefreshToken,
		session.UserAgent,
		session.IPAddress,
		session.DeviceID,
		session.IsActive,
		session.CreatedAt,
		session.ExpiresAt,
		session.LastActivity,
	)

	if err != nil {
		return dbSpan.End(err)
	}

	// Cache session with tracing
	cacheSpan := tracing.TraceCache(ctx, "SET", fmt.Sprintf("session:%s", session.RefreshToken))
	ctx = cacheSpan.Context()
	sessionKey := fmt.Sprintf("session:%s", session.RefreshToken)
	r.redis.Set(ctx, sessionKey, session.UserID.String(), time.Until(session.ExpiresAt))
	_ = cacheSpan.End(nil)

	return dbSpan.End(nil)
}

// GetByRefreshToken retrieves a session by refresh token
func (r *sessionRepository) GetByRefreshToken(ctx context.Context, refreshToken string) (*models.Session, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "auth_sessions")
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, user_id, user_type, refresh_token, user_agent, ip_address,
		       device_id, is_active, created_at, expires_at, last_activity
		FROM auth_sessions
		WHERE refresh_token = $1 AND is_active = true
	`

	var session models.Session
	err := r.db.QueryRowContext(ctx, query, refreshToken).Scan(
		&session.ID,
		&session.UserID,
		&session.UserType,
		&session.RefreshToken,
		&session.UserAgent,
		&session.IPAddress,
		&session.DeviceID,
		&session.IsActive,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.LastActivity,
	)

	if err != nil {
		return nil, dbSpan.End(err)
	}

	return &session, dbSpan.End(nil)
}

// UpdateLastActivity updates the last activity timestamp
func (r *sessionRepository) UpdateLastActivity(ctx context.Context, sessionID uuid.UUID) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "auth_sessions")
	dbSpan.SetID(sessionID.String()).SetQuery("UPDATE auth_sessions SET last_activity = $1 WHERE id = $2")
	ctx = dbSpan.Context()

	query := `
		UPDATE auth_sessions 
		SET last_activity = $1
		WHERE id = $2 AND is_active = true
	`

	_, err := r.db.ExecContext(ctx, query, time.Now(), sessionID)
	return dbSpan.End(err)
}

// Revoke deactivates a session
func (r *sessionRepository) Revoke(ctx context.Context, sessionID uuid.UUID) error {
	query := `
		UPDATE auth_sessions 
		SET is_active = false
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, sessionID)
	return err
}

// RevokeAllForUser deactivates all sessions for a user
func (r *sessionRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `
		UPDATE auth_sessions 
		SET is_active = false
		WHERE user_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return 0, err
	}

	sessionsRevoked, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return sessionsRevoked, nil
}

// CleanupExpired removes expired sessions
func (r *sessionRepository) CleanupExpired(ctx context.Context) error {
	query := `
		DELETE FROM auth_sessions 
		WHERE expires_at < $1 OR (is_active = false AND created_at < $2)
	`

	// Delete expired or inactive sessions older than 30 days
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	_, err := r.db.ExecContext(ctx, query, time.Now(), thirtyDaysAgo)
	return err
}

func (r *sessionRepository) ListAgentSessions(ctx context.Context, agentID uuid.UUID) ([]*models.Session, error) {
	var sessions []*models.Session
	query := `
		SELECT id, user_id, user_type, refresh_token, user_agent, ip_address,
		       device_id, is_active, created_at, expires_at, last_activity
		FROM auth_sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var session models.Session
		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.UserType,
			&session.RefreshToken,
			&session.UserAgent,
			&session.IPAddress,
			&session.DeviceID,
			&session.IsActive,
			&session.CreatedAt,
			&session.ExpiresAt,
			&session.LastActivity,
		)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

func (r *sessionRepository) CurrentSession(ctx context.Context, agentID uuid.UUID) (*models.Session, error) {
	query := `
		SELECT id, user_id, user_type, refresh_token, user_agent, ip_address,
		       device_id, is_active, created_at, expires_at, last_activity
		FROM auth_sessions
		WHERE user_id = $1 AND is_active = true
		ORDER BY created_at DESC
	`

	var session models.Session
	err := r.db.QueryRowContext(ctx, query, agentID).Scan(
		&session.ID,
		&session.UserID,
		&session.UserType,
		&session.RefreshToken,
		&session.UserAgent,
		&session.IPAddress,
		&session.DeviceID,
		&session.IsActive,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.LastActivity,
	)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

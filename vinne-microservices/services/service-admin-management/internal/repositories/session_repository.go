package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
	"github.com/randco/randco-microservices/shared/middleware/tracing"
)

// SessionRepository defines the interface for session data operations
type SessionRepository interface {
	Create(ctx context.Context, session *models.AdminSession) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AdminSession, error)
	GetByToken(ctx context.Context, refreshToken string) (*models.AdminSession, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.AdminSession, error)
	Update(ctx context.Context, session *models.AdminSession) error
	InvalidateByToken(ctx context.Context, refreshToken string) error
	InvalidateAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

type sessionRepository struct {
	db *sql.DB
}

// NewSessionRepository creates a new instance of SessionRepository
func NewSessionRepository(db *sql.DB) SessionRepository {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) Create(ctx context.Context, session *models.AdminSession) error {
	dbSpan := tracing.TraceDB(ctx, "INSERT", "admin_sessions")
	dbSpan.SetID(session.UserID.String()).SetQuery("INSERT INTO admin_sessions (id, user_id, refresh_token, ...) VALUES (...)")
	ctx = dbSpan.Context()

	query := `
		INSERT INTO admin_sessions (id, user_id, refresh_token, user_agent, ip_address, created_at, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}

	_, err := r.db.ExecContext(ctx, query,
		session.ID, session.UserID, session.RefreshToken,
		session.UserAgent, session.IPAddress, session.CreatedAt,
		session.ExpiresAt, session.IsActive)
	return dbSpan.End(err)
}

func (r *sessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AdminSession, error) {
	var session models.AdminSession
	query := `
		SELECT id, user_id, refresh_token, user_agent, ip_address, created_at, expires_at, is_active
		FROM admin_sessions
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&session.ID, &session.UserID, &session.RefreshToken,
		&session.UserAgent, &session.IPAddress, &session.CreatedAt,
		&session.ExpiresAt, &session.IsActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &session, nil
}

func (r *sessionRepository) GetByToken(ctx context.Context, refreshToken string) (*models.AdminSession, error) {
	var session models.AdminSession
	query := `
		SELECT id, user_id, refresh_token, user_agent, ip_address, created_at, expires_at, is_active
		FROM admin_sessions
		WHERE refresh_token = $1
	`

	err := r.db.QueryRowContext(ctx, query, refreshToken).Scan(
		&session.ID, &session.UserID, &session.RefreshToken,
		&session.UserAgent, &session.IPAddress, &session.CreatedAt,
		&session.ExpiresAt, &session.IsActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &session, nil
}

func (r *sessionRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.AdminSession, error) {
	var sessions []*models.AdminSession
	query := `
		SELECT id, user_id, refresh_token, user_agent, ip_address, created_at, expires_at, is_active
		FROM admin_sessions
		WHERE user_id = $1 AND is_active = true
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var session models.AdminSession
		err := rows.Scan(
			&session.ID, &session.UserID, &session.RefreshToken,
			&session.UserAgent, &session.IPAddress, &session.CreatedAt,
			&session.ExpiresAt, &session.IsActive,
		)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

func (r *sessionRepository) Update(ctx context.Context, session *models.AdminSession) error {
	query := `
		UPDATE admin_sessions
		SET refresh_token = $1,
		    expires_at = $2,
		    is_active = $3
		WHERE id = $4
	`

	result, err := r.db.ExecContext(ctx, query,
		session.RefreshToken, session.ExpiresAt, session.IsActive, session.ID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (r *sessionRepository) InvalidateByToken(ctx context.Context, refreshToken string) error {
	query := `
		UPDATE admin_sessions
		SET is_active = false
		WHERE refresh_token = $1
	`

	result, err := r.db.ExecContext(ctx, query, refreshToken)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (r *sessionRepository) InvalidateAllForUser(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE admin_sessions
		SET is_active = false
		WHERE user_id = $1 AND is_active = true
	`

	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

func (r *sessionRepository) DeleteExpired(ctx context.Context) error {
	query := `
		DELETE FROM admin_sessions
		WHERE expires_at < NOW()
	`

	_, err := r.db.ExecContext(ctx, query)
	return err
}

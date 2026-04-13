package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/models"
	"github.com/randco/randco-microservices/shared/middleware/tracing"
	"github.com/redis/go-redis/v9"
)

// AuthUser represents a user that can be authenticated (agent or retailer)
type AuthUser struct {
	ID                  uuid.UUID  `db:"id" json:"id"`
	Code                string     `db:"code" json:"code"` // agent_code or retailer_code
	Email               *string    `db:"email" json:"email"`
	Phone               *string    `db:"phone" json:"phone"`
	PasswordHash        string     `db:"password_hash" json:"password_hash"`
	PinHash             *string    `db:"pin_hash" json:"pin_hash"` // For retailer POS authentication
	IsActive            bool       `db:"is_active" json:"is_active"`
	FailedLoginAttempts int        `db:"failed_login_attempts" json:"failed_login_attempts"`
	LockedUntil         *time.Time `db:"locked_until" json:"locked_until"`
	LastLoginAt         *time.Time `db:"last_login_at" json:"last_login_at"`
	PasswordChangedAt   *time.Time `db:"password_changed_at" json:"password_changed_at"`
	CreatedAt           time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at" json:"updated_at"`
	UserType            string     // AGENT or RETAILER
}

// AgentAuthRepository handles agent authentication operations (max 10 methods)
type AgentAuthRepository interface {
	GetAgentByID(ctx context.Context, id uuid.UUID) (*AuthUser, error)
	GetAgentByEmail(ctx context.Context, email string) (*AuthUser, error)
	GetAgentByCode(ctx context.Context, code string) (*AuthUser, error)
	GetAgentByPhone(ctx context.Context, phone string) (*AuthUser, error)
	UpdateAgentLastLogin(ctx context.Context, id uuid.UUID) error
	UpdateAgentPassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	CreateAgent(ctx context.Context, agent *AuthUser) error
	UnlockAgentAccount(ctx context.Context, id uuid.UUID) error
	IncrementAgentFailedLogin(ctx context.Context, id uuid.UUID) (int, error)
	LockAgentAccount(ctx context.Context, id uuid.UUID, until time.Time) error
	GetAgentByIdentifier(ctx context.Context, identifier string) (*models.AgentRole, error)
	UpdatePassword(ctx context.Context, agentID uuid.UUID, hashedPassword string) error
	CreatePasswordResetLog(ctx context.Context, log *models.PasswordResetLog) error
	GetRecentResetAttempts(ctx context.Context, agentID uuid.UUID, duration time.Duration, resetToken *string) (int, error)
	AddPasswordToHistory(ctx context.Context, agentID uuid.UUID, hashedPassword string) error
	GetPasswordHistory(ctx context.Context, agentID uuid.UUID, limit int) ([]string, error)
}

// RetailerAuthRepository handles retailer authentication operations (max 10 methods)
type RetailerAuthRepository interface {
	GetRetailerByID(ctx context.Context, id uuid.UUID) (*AuthUser, error)
	GetRetailerByEmail(ctx context.Context, email string) (*AuthUser, error)
	GetRetailerByPhone(ctx context.Context, phone string) (*AuthUser, error)
	GetRetailerByCode(ctx context.Context, code string) (*AuthUser, error)
	CreateRetailer(ctx context.Context, retailer *AuthUser) error
	UpdateRetailerLastLogin(ctx context.Context, id uuid.UUID) error
	UpdateRetailerPassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	UpdateRetailerPin(ctx context.Context, id uuid.UUID, pinHash string) error
	UnlockRetailerAccount(ctx context.Context, id uuid.UUID) error
	IncrementRetailerFailedLogin(ctx context.Context, id uuid.UUID) (int, error)
	LockRetailerAccount(ctx context.Context, id uuid.UUID, until time.Time) error
	CreatePINChangeLog(ctx context.Context, log *models.PINChangeLog) error
}

// AuthRepository combines agent and retailer auth repositories with transaction support
type AuthRepository interface {
	AgentAuthRepository
	RetailerAuthRepository
	// Transaction support
	WithTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error
}

// DBExecutor is an interface that both *sqlx.DB and *sqlx.Tx implement
type DBExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
}

type authRepository struct {
	db    *sqlx.DB
	redis *redis.Client
}

// NewAuthRepository creates a new authentication repository that implements all auth interfaces
func NewAuthRepository(db *sqlx.DB, redis *redis.Client) AuthRepository {
	return &authRepository{
		db:    db,
		redis: redis,
	}
}

// txKey is the context key for storing transactions
type txKey struct{}

// WithTx executes a function within a database transaction
func (r *authRepository) WithTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := r.db.BeginTxx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Execute the function
	if err := fn(tx); err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("failed to rollback transaction: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	// Commit on success
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// getDBFromContext returns the transaction if one exists in the context, otherwise returns the regular DB
func (r *authRepository) getDBFromContext(ctx context.Context) DBExecutor {
	if tx, ok := ctx.Value(txKey{}).(*sqlx.Tx); ok && tx != nil {
		return tx
	}
	return r.db
}

// NewAgentAuthRepository creates a repository for agent authentication
func NewAgentAuthRepository(db *sqlx.DB, redis *redis.Client) AgentAuthRepository {
	return &authRepository{
		db:    db,
		redis: redis,
	}
}

// NewRetailerAuthRepository creates a repository for retailer authentication
func NewRetailerAuthRepository(db *sqlx.DB, redis *redis.Client) RetailerAuthRepository {
	return &authRepository{
		db:    db,
		redis: redis,
	}
}

// Agent Authentication Methods

func (r *authRepository) GetAgentByID(ctx context.Context, id uuid.UUID) (*AuthUser, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "agents_auth").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, agent_code as code, email, phone, password_hash, is_active, 
		       failed_login_attempts, locked_until, last_login_at, 
		       password_changed_at, created_at, updated_at
		FROM agents_auth
		WHERE id = $1
	`

	var user AuthUser
	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		return nil, dbSpan.End(err)
	}
	user.UserType = "AGENT"
	return &user, dbSpan.End(nil)
}

func (r *authRepository) GetAgentByEmail(ctx context.Context, email string) (*AuthUser, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "agents_auth")
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, agent_code as code, email, phone, password_hash, is_active, 
		       failed_login_attempts, locked_until, last_login_at, 
		       password_changed_at, created_at, updated_at
		FROM agents_auth
		WHERE email = $1
	`

	var user AuthUser
	err := r.db.GetContext(ctx, &user, query, email)
	if err != nil {
		return nil, dbSpan.End(err)
	}
	user.UserType = "AGENT"
	return &user, dbSpan.End(nil)
}

func (r *authRepository) GetAgentByCode(ctx context.Context, code string) (*AuthUser, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "agents_auth")
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, agent_code as code, email, phone, password_hash, is_active, 
		       failed_login_attempts, locked_until, last_login_at, 
		       password_changed_at, created_at, updated_at
		FROM agents_auth
		WHERE agent_code = $1
	`

	var user AuthUser
	err := r.db.GetContext(ctx, &user, query, code)
	if err != nil {
		return nil, dbSpan.End(err)
	}
	user.UserType = "AGENT"
	return &user, dbSpan.End(nil)
}

func (r *authRepository) GetAgentByPhone(ctx context.Context, phone string) (*AuthUser, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "agents_auth")
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, agent_code as code, email, phone, password_hash, is_active,
		       failed_login_attempts, locked_until, last_login_at,
		       password_changed_at, created_at, updated_at
		FROM agents_auth
		WHERE phone = $1
	`

	var user AuthUser
	err := r.db.GetContext(ctx, &user, query, phone)
	if err != nil {
		return nil, err
	}
	user.UserType = "AGENT"
	dbSpan.SetID(user.ID.String())
	return &user, nil
}

func (r *authRepository) UpdateAgentLastLogin(ctx context.Context, id uuid.UUID) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "agents_auth").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		UPDATE agents_auth 
		SET last_login_at = NOW(), 
		    failed_login_attempts = 0,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id)
	return dbSpan.End(err)
}

func (r *authRepository) IncrementAgentFailedLogin(ctx context.Context, id uuid.UUID) (int, error) {
	// Check if we're in a transaction context
	db := r.getDBFromContext(ctx)

	dbSpan := tracing.TraceDB(ctx, "UPDATE", "agents_auth").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	var attempts int
	query := `
		UPDATE agents_auth
		SET failed_login_attempts = failed_login_attempts + 1,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING failed_login_attempts
	`
	err := db.GetContext(ctx, &attempts, query, id)
	return attempts, dbSpan.End(err)
}

func (r *authRepository) LockAgentAccount(ctx context.Context, id uuid.UUID, until time.Time) error {
	// Check if we're in a transaction context
	db := r.getDBFromContext(ctx)

	dbSpan := tracing.TraceDB(ctx, "UPDATE", "agents_auth").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		UPDATE agents_auth
		SET locked_until = $2,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := db.ExecContext(ctx, query, id, until)
	return dbSpan.End(err)
}

func (r *authRepository) UnlockAgentAccount(ctx context.Context, id uuid.UUID) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "agents_auth").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		UPDATE agents_auth
		SET locked_until = NULL,
		    failed_login_attempts = 0,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id)
	return dbSpan.End(err)
}

func (r *authRepository) UpdateAgentPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "agents_auth").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		UPDATE agents_auth
		SET password_hash = $2,
		    password_changed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, passwordHash)
	return dbSpan.End(err)
}

func (r *authRepository) CreateAgent(ctx context.Context, agent *AuthUser) error {
	dbSpan := tracing.TraceDB(ctx, "INSERT", "agents_auth").SetID(agent.ID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
	`
	_, err := r.db.ExecContext(ctx, query, agent.ID, agent.Code, agent.Email, agent.Phone, agent.PasswordHash, agent.IsActive)
	return dbSpan.End(err)
}

func (r *authRepository) GetAgentByIdentifier(ctx context.Context, identifier string) (*models.AgentRole, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "agents_auth").SetID(identifier)
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, agent_code, phone, email, password_hash, is_active
		FROM agents_auth
		WHERE email = $1 OR phone = $1 OR agent_code = $1`

	var agent models.AgentRole
	err := r.db.GetContext(ctx, &agent, query, identifier)
	if err != nil {
		return nil, dbSpan.End(err)
	}

	return &agent, dbSpan.End(nil)

}

func (r *authRepository) UpdatePassword(ctx context.Context, agentID uuid.UUID, hashedPassword string) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "agents_auth").SetID(agentID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		UPDATE agents_auth
		SET password_hash = $1, 
			password_updated_at = NOW(),
			password_reset_count = password_reset_count + 1
		WHERE id = $2`

	_, err := r.db.ExecContext(ctx, query, hashedPassword, agentID)
	return err
}

func (r *authRepository) CreatePasswordResetLog(ctx context.Context, logs *models.PasswordResetLog) error {
	dbSpan := tracing.TraceDB(ctx, "INSERT", "password_reset_logs").SetID(logs.ID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		INSERT INTO password_reset_logs (id, agent_id, reset_token, request_ip, user_agent, 
		channel, status, otp_attempts, completed_at, expires_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) 
			returning id`

	row := r.db.QueryRowContext(
		ctx,
		query,
		logs.ID,
		logs.AgentID,
		logs.ResetToken,
		logs.RequestIP,
		logs.UserAgent,
		logs.Channel,
		logs.Status,
		logs.OTPAttempts,
		logs.CompletedAt,
		logs.ExpiresAt,
		logs.CreatedAt,
		logs.UpdatedAt,
	)
	var id string
	err := row.Scan(&id)
	return dbSpan.End(err)
}

func (r *authRepository) GetRecentResetAttempts(ctx context.Context, agentID uuid.UUID, duration time.Duration, resetToken *string) (int, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "password_reset_logs").SetID(duration.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
			SELECT COUNT (*)
			FROM password_reset_logs
			WHERE agent_id = $1
			AND created_at >= NOW() - INTERVAL '1 second' * $2
			`

	args := []any{agentID, int(duration.Seconds())}

	if resetToken != nil {
		query += " AND reset_token = $3"
		args = append(args, resetToken)
	}

	var count int
	err := r.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *authRepository) AddPasswordToHistory(ctx context.Context, agentID uuid.UUID, hashedPassword string) error {
	fmt.Printf("Adding password history for agent ID: %s\n", agentID.String())
	dbSpan := tracing.TraceDB(ctx, "INSERT", "agent_password_history").SetID(agentID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		INSERT INTO agent_password_history (id, agent_id, password_hash, created_at)
		VALUES ($1, $2, $3, NOW())
	`
	_, err := r.db.ExecContext(ctx, query, uuid.New(), agentID, hashedPassword)
	return dbSpan.End(err)
}

func (r *authRepository) GetPasswordHistory(ctx context.Context, agentID uuid.UUID, limit int) ([]string, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "agent_password_history").SetID(agentID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT password_hash FROM agent_password_history
		WHERE agent_id = $1 
		ORDER BY created_at DESC LIMIT $2
	`
	var hashes []string
	err := r.db.SelectContext(ctx, &hashes, query, agentID, limit)

	return hashes, dbSpan.End(err)
}

// Retailer Authentication Methods

func (r *authRepository) GetRetailerByID(ctx context.Context, id uuid.UUID) (*AuthUser, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "retailers_auth").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, retailer_code as code, email, phone, password_hash, pin_hash,
		       is_active, failed_login_attempts, locked_until, last_login_at,
		       password_changed_at, created_at, updated_at
		FROM retailers_auth
		WHERE id = $1
	`

	var user AuthUser
	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		return nil, dbSpan.End(err)
	}
	user.UserType = "RETAILER"
	return &user, dbSpan.End(nil)
}

func (r *authRepository) GetRetailerByEmail(ctx context.Context, email string) (*AuthUser, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "retailers_auth")
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, retailer_code as code, email, phone, password_hash, pin_hash,
		       is_active, failed_login_attempts, locked_until, last_login_at, 
		       password_changed_at, created_at, updated_at
		FROM retailers_auth
		WHERE email = $1
	`

	var user AuthUser
	err := r.db.GetContext(ctx, &user, query, email)
	if err != nil {
		return nil, dbSpan.End(err)
	}
	user.UserType = "RETAILER"
	return &user, dbSpan.End(nil)
}

func (r *authRepository) GetRetailerByPhone(ctx context.Context, phone string) (*AuthUser, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "retailers_auth")
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, retailer_code as code, email, phone, password_hash, pin_hash,
		       is_active, failed_login_attempts, locked_until, last_login_at,
		       password_changed_at, created_at, updated_at
		FROM retailers_auth
		WHERE phone = $1
	`

	var user AuthUser
	err := r.db.GetContext(ctx, &user, query, phone)
	if err != nil {
		return nil, dbSpan.End(err)
	}
	user.UserType = "RETAILER"
	dbSpan.SetID(user.ID.String())
	return &user, dbSpan.End(nil)
}

func (r *authRepository) GetRetailerByCode(ctx context.Context, code string) (*AuthUser, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "retailers_auth")
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, retailer_code as code, email, phone, password_hash, pin_hash,
		       is_active, failed_login_attempts, locked_until, last_login_at, 
		       password_changed_at, created_at, updated_at
		FROM retailers_auth
		WHERE retailer_code = $1
	`

	var user AuthUser
	err := r.db.GetContext(ctx, &user, query, code)
	if err != nil {
		return nil, dbSpan.End(err)
	}
	user.UserType = "RETAILER"
	return &user, dbSpan.End(nil)
}

func (r *authRepository) UpdateRetailerLastLogin(ctx context.Context, id uuid.UUID) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "retailers_auth").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		UPDATE retailers_auth
		SET last_login_at = NOW(),
		    failed_login_attempts = 0,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id)
	return dbSpan.End(err)
}

func (r *authRepository) UpdateRetailerPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "retailers_auth").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		UPDATE retailers_auth
		SET password_hash = $2,
		    password_changed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, passwordHash)
	return dbSpan.End(err)
}

func (r *authRepository) IncrementRetailerFailedLogin(ctx context.Context, id uuid.UUID) (int, error) {
	// Check if we're in a transaction context
	db := r.getDBFromContext(ctx)

	dbSpan := tracing.TraceDB(ctx, "UPDATE", "retailers_auth").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	var attempts int
	query := `
		UPDATE retailers_auth
		SET failed_login_attempts = failed_login_attempts + 1,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING failed_login_attempts
	`
	err := db.GetContext(ctx, &attempts, query, id)
	return attempts, dbSpan.End(err)
}

func (r *authRepository) LockRetailerAccount(ctx context.Context, id uuid.UUID, until time.Time) error {
	// Check if we're in a transaction context
	db := r.getDBFromContext(ctx)

	dbSpan := tracing.TraceDB(ctx, "UPDATE", "retailers_auth").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		UPDATE retailers_auth
		SET locked_until = $2,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := db.ExecContext(ctx, query, id, until)
	return dbSpan.End(err)
}

func (r *authRepository) UnlockRetailerAccount(ctx context.Context, id uuid.UUID) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "retailers_auth").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		UPDATE retailers_auth
		SET locked_until = NULL,
		    failed_login_attempts = 0,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id)
	return dbSpan.End(err)
}

func (r *authRepository) UpdateRetailerPin(ctx context.Context, id uuid.UUID, pinHash string) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "retailers_auth").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		UPDATE retailers_auth
		SET pin_hash = $2,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, pinHash)
	return dbSpan.End(err)
}

// CreateRetailer creates a new retailer authentication record
func (r *authRepository) CreateRetailer(ctx context.Context, retailer *AuthUser) error {
	dbSpan := tracing.TraceDB(ctx, "auth", "create.retailer")
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		INSERT INTO retailers_auth (id, retailer_code, email, phone, password_hash, pin_hash, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		retailer.ID,
		retailer.Code,
		retailer.Email,
		retailer.Phone,
		retailer.PasswordHash, // For retailers, we store PIN in password_hash field
		retailer.PasswordHash, // Also store in pin_hash for compatibility
		retailer.IsActive,
		retailer.CreatedAt,
		retailer.UpdatedAt,
	)

	return dbSpan.End(err)
}

func (r *authRepository) CreatePINChangeLog(ctx context.Context, log *models.PINChangeLog) error {
	dbSpan := tracing.TraceDB(ctx, "INSERT", "retailers_auth").SetID(log.RetailerID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	//TODO: Add change_reason, user_agent

	query := `
		INSERT INTO retailer_pin_change_logs (id, retailer_id, retailer_code, device_imei, ip_address, sessions_invalidated, changed_by, success, failure_reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
	`
	_, err := r.db.ExecContext(ctx, query, log.ID, log.RetailerID, log.RetailerCode, log.DeviceIMEI, log.IPAddress, log.SessionsInvalidated, log.ChangedBy, log.Success, log.FailureReason, log.CreatedAt)
	return dbSpan.End(err)
}

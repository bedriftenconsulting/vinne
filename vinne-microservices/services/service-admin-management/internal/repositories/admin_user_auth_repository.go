package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// AdminUserAuthRepository defines the interface for admin user authentication operations
type AdminUserAuthRepository interface {
	VerifyCredentials(ctx context.Context, email, password string) (*models.AdminUser, error)
	UpdateLastLogin(ctx context.Context, userID uuid.UUID, ipAddress string) error
	UpdatePassword(ctx context.Context, userID uuid.UUID, hashedPassword string) error
	UpdateMFAStatus(ctx context.Context, userID uuid.UUID, enabled bool) error
	UpdateMFASecret(ctx context.Context, userID uuid.UUID, secret string) error
	GetMFASecret(ctx context.Context, userID uuid.UUID) (string, error)
}

// VerifyCredentials verifies admin user credentials
func (r *adminUserRepository) VerifyCredentials(ctx context.Context, email, password string) (*models.AdminUser, error) {
	log.Printf("[VerifyCredentials] Starting credential verification for email: %s", email)

	// First get the user with their roles
	query := `
		SELECT 
			u.id, u.email, u.username, u.password_hash, u.first_name, u.last_name,
			u.mfa_enabled, u.is_active, u.ip_whitelist, u.last_login, u.last_login_ip,
			u.created_at, u.updated_at,
			COALESCE(
				json_agg(
					DISTINCT jsonb_build_object(
						'id', r.id,
						'name', r.name,
						'description', r.description,
						'created_at', r.created_at
					)
				) FILTER (WHERE r.id IS NOT NULL), 
				'[]'
			) as roles
		FROM admin_users u
		LEFT JOIN admin_user_roles ur ON u.id = ur.user_id
		LEFT JOIN admin_roles r ON ur.role_id = r.id AND r.deleted_at IS NULL
		WHERE u.email = $1
		GROUP BY u.id, u.email, u.username, u.password_hash, u.first_name, u.last_name,
				 u.mfa_enabled, u.is_active, u.ip_whitelist, u.last_login, u.last_login_ip,
				 u.created_at, u.updated_at
	`

	log.Printf("[VerifyCredentials] Executing query to find user by email: %s", email)

	var user models.AdminUser
	var passwordHash string
	var rolesJSON []byte

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&passwordHash,
		&user.FirstName,
		&user.LastName,
		&user.MFAEnabled,
		&user.IsActive,
		pq.Array(&user.IPWhitelist),
		&user.LastLogin,
		&user.LastLoginIP,
		&user.CreatedAt,
		&user.UpdatedAt,
		&rolesJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[VerifyCredentials] User not found for email: %s", email)
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	log.Printf("[VerifyCredentials] User found: ID=%s, Email=%s, IsActive=%v", user.ID, user.Email, user.IsActive)

	// Check if account is active
	if !user.IsActive {
		log.Printf("[VerifyCredentials] User account is deactivated for email: %s", email)
		return nil, fmt.Errorf("account is deactivated")
	}

	// Verify password
	log.Printf("[VerifyCredentials] Verifying password for user: %s", email)
	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err != nil {
		log.Printf("[VerifyCredentials] Password verification failed for email %s: %v", email, err)
		return nil, fmt.Errorf("invalid credentials")
	}

	log.Printf("[VerifyCredentials] Password verified successfully for email: %s", email)

	// Parse roles JSON
	if err := parseRolesJSON(rolesJSON, &user.Roles); err != nil {
		return nil, fmt.Errorf("failed to parse roles: %w", err)
	}

	// For each role, get its permissions
	for i, role := range user.Roles {
		permissions, err := r.getRolePermissions(ctx, role.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get permissions for role %s: %w", role.ID, err)
		}
		user.Roles[i].Permissions = permissions
	}

	return &user, nil
}

// UpdateLastLogin updates the last login timestamp and IP
func (r *adminUserRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID, ipAddress string) error {
	query := `
		UPDATE admin_users 
		SET last_login = $1, last_login_ip = $2, updated_at = $3
		WHERE id = $4
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, now, ipAddress, now, userID)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}

// UpdatePassword updates the user's password
func (r *adminUserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, hashedPassword string) error {
	query := `
		UPDATE admin_users 
		SET password_hash = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.ExecContext(ctx, query, hashedPassword, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// UpdateMFAStatus updates the MFA enabled status
func (r *adminUserRepository) UpdateMFAStatus(ctx context.Context, userID uuid.UUID, enabled bool) error {
	query := `
		UPDATE admin_users 
		SET mfa_enabled = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.ExecContext(ctx, query, enabled, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update MFA status: %w", err)
	}

	return nil
}

// UpdateMFASecret updates the user's MFA secret
func (r *adminUserRepository) UpdateMFASecret(ctx context.Context, userID uuid.UUID, secret string) error {
	query := `
		UPDATE admin_users 
		SET mfa_secret = $1, updated_at = $2
		WHERE id = $3
	`

	// Convert empty string to NULL
	var secretValue interface{}
	if secret == "" {
		secretValue = nil
	} else {
		secretValue = secret
	}

	_, err := r.db.ExecContext(ctx, query, secretValue, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update MFA secret: %w", err)
	}

	return nil
}

// GetMFASecret retrieves the user's MFA secret
func (r *adminUserRepository) GetMFASecret(ctx context.Context, userID uuid.UUID) (string, error) {
	var secret sql.NullString
	query := `SELECT mfa_secret FROM admin_users WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, userID).Scan(&secret)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("user not found")
		}
		return "", fmt.Errorf("failed to get MFA secret: %w", err)
	}

	return secret.String, nil
}

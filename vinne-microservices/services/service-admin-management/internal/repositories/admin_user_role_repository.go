package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
	"github.com/randco/randco-microservices/shared/middleware/tracing"
)

// AdminUserRoleRepository defines the interface for admin user role management operations
type AdminUserRoleRepository interface {
	AssignRole(ctx context.Context, userID, roleID uuid.UUID) error
	RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error
	GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*models.Role, error)
}

// AssignRole assigns a role to a user
func (r *adminUserRepository) AssignRole(ctx context.Context, userID, roleID uuid.UUID) error {
	dbSpan := tracing.TraceDB(ctx, "INSERT", "admin_user_roles")
	dbSpan.SetID(userID.String()).SetQuery("INSERT INTO admin_user_roles (user_id, role_id, created_at) VALUES (...)")
	ctx = dbSpan.Context()

	query := `
		INSERT INTO admin_user_roles (user_id, role_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, role_id) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query, userID, roleID, time.Now())
	return dbSpan.End(err)
}

// RemoveRole removes a role from a user
func (r *adminUserRepository) RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error {
	dbSpan := tracing.TraceDB(ctx, "DELETE", "admin_user_roles")
	dbSpan.SetID(userID.String()).SetQuery("DELETE FROM admin_user_roles WHERE user_id = $1 AND role_id = $2")
	ctx = dbSpan.Context()

	query := `
		DELETE FROM admin_user_roles
		WHERE user_id = $1 AND role_id = $2
	`

	_, err := r.db.ExecContext(ctx, query, userID, roleID)
	return dbSpan.End(err)
}

// GetUserRoles gets all roles assigned to a user
func (r *adminUserRepository) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*models.Role, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "admin_roles")
	dbSpan.SetID(userID.String()).SetQuery("SELECT r.* FROM admin_roles r JOIN admin_user_roles ur ON r.id = ur.role_id WHERE ur.user_id = $1")
	ctx = dbSpan.Context()

	query := `
		SELECT r.id, r.name, r.description, r.created_at, r.updated_at
		FROM admin_roles r
		INNER JOIN admin_user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1 AND r.deleted_at IS NULL
		ORDER BY r.name
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, dbSpan.End(fmt.Errorf("failed to get user roles: %w", err))
	}
	defer func() { _ = rows.Close() }()

	var roles []*models.Role
	for rows.Next() {
		role := &models.Role{}
		err := rows.Scan(
			&role.ID,
			&role.Name,
			&role.Description,
			&role.CreatedAt,
			&role.UpdatedAt,
		)
		if err != nil {
			return nil, dbSpan.End(fmt.Errorf("failed to scan role: %w", err))
		}

		// Get permissions for this role
		permissions, err := r.getRolePermissions(ctx, role.ID)
		if err != nil {
			return nil, dbSpan.End(fmt.Errorf("failed to get permissions for role %s: %w", role.ID, err))
		}
		role.Permissions = permissions

		roles = append(roles, role)
	}

	_ = dbSpan.End(nil)
	return roles, nil
}

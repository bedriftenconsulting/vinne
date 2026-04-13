package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
)

// RoleRepository defines the interface for role data operations
type RoleRepository interface {
	Create(ctx context.Context, role *models.Role) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error)
	GetByName(ctx context.Context, name string) (*models.Role, error)
	List(ctx context.Context, filter models.RoleFilter) ([]*models.Role, int, error)
	Update(ctx context.Context, role *models.Role) error
	Delete(ctx context.Context, id uuid.UUID) error
	AssignPermission(ctx context.Context, roleID, permissionID uuid.UUID) error
	RemovePermission(ctx context.Context, roleID, permissionID uuid.UUID) error
	GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]*models.Permission, error)
	SetRolePermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error
}

type roleRepository struct {
	db *sql.DB
}

// NewRoleRepository creates a new instance of RoleRepository
func NewRoleRepository(db *sql.DB) RoleRepository {
	return &roleRepository{db: db}
}

func (r *roleRepository) Create(ctx context.Context, role *models.Role) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Insert role
	query := `
		INSERT INTO admin_roles (id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	roleID := uuid.New()
	role.ID = roleID
	role.CreatedAt = time.Now()
	role.UpdatedAt = time.Now()

	_, err = tx.ExecContext(ctx, query,
		roleID,
		role.Name,
		role.Description,
		role.CreatedAt,
		role.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert role: %w", err)
	}

	return tx.Commit()
}

func (r *roleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error) {
	query := `
		SELECT id, name, description, created_at, updated_at, deleted_at
		FROM admin_roles
		WHERE id = $1 AND deleted_at IS NULL
	`

	role := &models.Role{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&role.ID,
		&role.Name,
		&role.Description,
		&role.CreatedAt,
		&role.UpdatedAt,
		&role.DeletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil when not found
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	// Load permissions
	permissions, err := r.GetRolePermissions(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load permissions: %w", err)
	}
	role.Permissions = permissions

	return role, nil
}

func (r *roleRepository) GetByName(ctx context.Context, name string) (*models.Role, error) {
	query := `
		SELECT id, name, description, created_at, updated_at, deleted_at
		FROM admin_roles
		WHERE name = $1 AND deleted_at IS NULL
	`

	role := &models.Role{}
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&role.ID,
		&role.Name,
		&role.Description,
		&role.CreatedAt,
		&role.UpdatedAt,
		&role.DeletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil when not found
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	return role, nil
}

func (r *roleRepository) List(ctx context.Context, filter models.RoleFilter) ([]*models.Role, int, error) {
	var whereConditions []string
	var args []interface{}
	argIndex := 1

	// Base condition
	whereConditions = append(whereConditions, "deleted_at IS NULL")

	// Apply filters
	if filter.Name != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("name ILIKE $%d", argIndex))
		args = append(args, "%"+filter.Name+"%")
		argIndex++
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Count total records
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM admin_roles %s", whereClause)
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count roles: %w", err)
	}

	// Apply pagination
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}

	offset := (filter.Page - 1) * filter.PageSize
	args = append(args, filter.PageSize, offset)

	// Main query
	query := fmt.Sprintf(`
		SELECT id, name, description, created_at, updated_at, deleted_at
		FROM admin_roles
		%s
		ORDER BY name
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list roles: %w", err)
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
			&role.DeletedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan role: %w", err)
		}

		// Load permissions for each role
		permissions, err := r.GetRolePermissions(ctx, role.ID)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to load permissions for role %s: %w", role.ID, err)
		}
		role.Permissions = permissions

		roles = append(roles, role)
	}

	return roles, totalCount, nil
}

func (r *roleRepository) Update(ctx context.Context, role *models.Role) error {
	role.UpdatedAt = time.Now()

	query := `
		UPDATE admin_roles
		SET name = $2, description = $3, updated_at = $4
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query,
		role.ID,
		role.Name,
		role.Description,
		role.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return nil // Return nil when not found
	}

	return nil
}

func (r *roleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE admin_roles
		SET deleted_at = $2, updated_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, id, now)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return nil // Return nil when not found
	}

	return nil
}

func (r *roleRepository) AssignPermission(ctx context.Context, roleID, permissionID uuid.UUID) error {
	query := `
		INSERT INTO role_permissions (role_id, permission_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query, roleID, permissionID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to assign permission: %w", err)
	}

	return nil
}

func (r *roleRepository) RemovePermission(ctx context.Context, roleID, permissionID uuid.UUID) error {
	query := `DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2`

	result, err := r.db.ExecContext(ctx, query, roleID, permissionID)
	if err != nil {
		return fmt.Errorf("failed to remove permission: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("permission assignment not found")
	}

	return nil
}

func (r *roleRepository) GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]*models.Permission, error) {
	query := `
		SELECT p.id, p.resource, p.action, p.description, p.created_at
		FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.resource, p.action
	`

	rows, err := r.db.QueryContext(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var permissions []*models.Permission
	for rows.Next() {
		permission := &models.Permission{}
		err := rows.Scan(
			&permission.ID,
			&permission.Resource,
			&permission.Action,
			&permission.Description,
			&permission.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, permission)
	}

	return permissions, nil
}

func (r *roleRepository) SetRolePermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Remove all existing permissions
	deleteQuery := `DELETE FROM role_permissions WHERE role_id = $1`
	_, err = tx.ExecContext(ctx, deleteQuery, roleID)
	if err != nil {
		return fmt.Errorf("failed to remove existing permissions: %w", err)
	}

	// Add new permissions
	if len(permissionIDs) > 0 {
		insertQuery := `
			INSERT INTO role_permissions (role_id, permission_id, created_at)
			VALUES ($1, $2, $3)
		`
		now := time.Now()
		for _, permissionID := range permissionIDs {
			_, err = tx.ExecContext(ctx, insertQuery, roleID, permissionID, now)
			if err != nil {
				return fmt.Errorf("failed to assign permission %s: %w", permissionID, err)
			}
		}
	}

	return tx.Commit()
}

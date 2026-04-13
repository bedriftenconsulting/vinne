package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
)

// PermissionRepository defines the interface for permission data operations
type PermissionRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Permission, error)
	List(ctx context.Context, filter models.PermissionFilter) ([]*models.Permission, int, error)
	GetByResourceAction(ctx context.Context, resource, action string) (*models.Permission, error)
}

type permissionRepository struct {
	db *sql.DB
}

// NewPermissionRepository creates a new instance of PermissionRepository
func NewPermissionRepository(db *sql.DB) PermissionRepository {
	return &permissionRepository{db: db}
}

func (r *permissionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Permission, error) {
	query := `
		SELECT id, resource, action, description, created_at
		FROM permissions
		WHERE id = $1
	`

	permission := &models.Permission{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&permission.ID,
		&permission.Resource,
		&permission.Action,
		&permission.Description,
		&permission.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil when not found
		}
		return nil, fmt.Errorf("failed to get permission: %w", err)
	}

	return permission, nil
}

func (r *permissionRepository) GetByResourceAction(ctx context.Context, resource, action string) (*models.Permission, error) {
	query := `
		SELECT id, resource, action, description, created_at
		FROM permissions
		WHERE resource = $1 AND action = $2
	`

	permission := &models.Permission{}
	err := r.db.QueryRowContext(ctx, query, resource, action).Scan(
		&permission.ID,
		&permission.Resource,
		&permission.Action,
		&permission.Description,
		&permission.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil when not found
		}
		return nil, fmt.Errorf("failed to get permission: %w", err)
	}

	return permission, nil
}

func (r *permissionRepository) List(ctx context.Context, filter models.PermissionFilter) ([]*models.Permission, int, error) {
	var whereConditions []string
	var args []interface{}
	argIndex := 1

	// Apply filters
	if filter.Resource != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("resource = $%d", argIndex))
		args = append(args, filter.Resource)
		argIndex++
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Count total records
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM permissions %s", whereClause)
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count permissions: %w", err)
	}

	// Apply pagination
	if filter.PageSize <= 0 {
		filter.PageSize = 50
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}

	offset := (filter.Page - 1) * filter.PageSize
	args = append(args, filter.PageSize, offset)

	// Main query
	query := fmt.Sprintf(`
		SELECT id, resource, action, description, created_at
		FROM permissions
		%s
		ORDER BY resource, action
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list permissions: %w", err)
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
			return nil, 0, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, permission)
	}

	return permissions, totalCount, nil
}

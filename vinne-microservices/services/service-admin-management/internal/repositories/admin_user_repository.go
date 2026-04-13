package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
	"github.com/randco/randco-microservices/shared/middleware/tracing"
	"golang.org/x/crypto/bcrypt"
)

// AdminUserRepository defines the interface for core admin user CRUD operations
type AdminUserRepository interface {
	Create(ctx context.Context, user *models.AdminUser, password string) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AdminUser, error)
	GetByEmail(ctx context.Context, email string) (*models.AdminUser, error)
	GetByUsername(ctx context.Context, username string) (*models.AdminUser, error)
	List(ctx context.Context, filter models.AdminUserFilter) ([]*models.AdminUser, int, error)
	Update(ctx context.Context, user *models.AdminUser) error
	Delete(ctx context.Context, id uuid.UUID) error
	Activate(ctx context.Context, id uuid.UUID) error
	Deactivate(ctx context.Context, id uuid.UUID) error
}

type adminUserRepository struct {
	db *sql.DB
}

// NewAdminUserRepository creates a new instance of AdminUserRepository (deprecated - use NewAdminUserRepositories)
func NewAdminUserRepository(db *sql.DB) AdminUserRepository {
	return &adminUserRepository{db: db}
}

// NewAdminUserRepositories creates instances of all admin user repository interfaces
func NewAdminUserRepositories(db *sql.DB) (AdminUserRepository, AdminUserAuthRepository, AdminUserRoleRepository) {
	impl := &adminUserRepository{db: db}
	return impl, impl, impl
}

func (r *adminUserRepository) Create(ctx context.Context, user *models.AdminUser, password string) error {
	// Simple tracing with the new helper
	dbSpan := tracing.TraceDB(ctx, "INSERT", "admin_users")
	dbSpan.SetID(user.Email)
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to hash password: %w", err))
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Insert user
	query := `
		INSERT INTO admin_users (
			id, email, username, password_hash, first_name, last_name, 
			mfa_enabled, is_active, ip_whitelist, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`

	userID := uuid.New()
	user.ID = userID
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	_, err = tx.ExecContext(ctx, query,
		userID,
		user.Email,
		user.Username,
		string(hashedPassword),
		user.FirstName,
		user.LastName,
		user.MFAEnabled,
		user.IsActive,
		pq.Array(user.IPWhitelist),
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to insert admin user: %w", err))
	}

	err = tx.Commit()
	if err != nil {
		return dbSpan.End(err)
	}

	dbSpan.SetID(userID.String())
	return dbSpan.End(nil)
}

func (r *adminUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AdminUser, error) {
	// Simple tracing with the new helper
	dbSpan := tracing.TraceDB(ctx, "SELECT", "admin_users").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT 
			id, email, username, first_name, last_name, mfa_enabled, 
			is_active, last_login, last_login_ip, ip_whitelist, 
			created_at, updated_at, deleted_at
		FROM admin_users 
		WHERE id = $1 AND deleted_at IS NULL
	`

	user := &models.AdminUser{}
	var ipWhitelist pq.StringArray

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.MFAEnabled,
		&user.IsActive,
		&user.LastLogin,
		&user.LastLoginIP,
		&ipWhitelist,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			_ = dbSpan.End(err) // Helper handles sql.ErrNoRows appropriately
			return nil, fmt.Errorf("admin user not found")
		}
		return nil, dbSpan.End(fmt.Errorf("failed to get admin user: %w", err))
	}

	user.IPWhitelist = []string(ipWhitelist)

	// Load roles
	roles, err := r.getUserRoles(ctx, id)
	if err != nil {
		return nil, dbSpan.End(fmt.Errorf("failed to load user roles: %w", err))
	}
	user.Roles = roles

	return user, dbSpan.End(nil)
}

func (r *adminUserRepository) GetByEmail(ctx context.Context, email string) (*models.AdminUser, error) {
	// Simple tracing with the new helper
	dbSpan := tracing.TraceDB(ctx, "SELECT", "admin_users").SetID(email)
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT 
			id, email, username, first_name, last_name, mfa_enabled, 
			is_active, last_login, last_login_ip, ip_whitelist, 
			created_at, updated_at, deleted_at
		FROM admin_users 
		WHERE email = $1 AND deleted_at IS NULL
	`

	user := &models.AdminUser{}
	var ipWhitelist pq.StringArray

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.MFAEnabled,
		&user.IsActive,
		&user.LastLogin,
		&user.LastLoginIP,
		&ipWhitelist,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			_ = dbSpan.End(err) // Helper handles sql.ErrNoRows appropriately
			return nil, fmt.Errorf("admin user not found")
		}
		return nil, dbSpan.End(fmt.Errorf("failed to get admin user: %w", err))
	}

	user.IPWhitelist = []string(ipWhitelist)
	return user, dbSpan.End(nil)
}

func (r *adminUserRepository) GetByUsername(ctx context.Context, username string) (*models.AdminUser, error) {
	query := `
		SELECT 
			id, email, username, first_name, last_name, mfa_enabled, 
			is_active, last_login, last_login_ip, ip_whitelist, 
			created_at, updated_at, deleted_at
		FROM admin_users 
		WHERE username = $1 AND deleted_at IS NULL
	`

	user := &models.AdminUser{}
	var ipWhitelist pq.StringArray

	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.MFAEnabled,
		&user.IsActive,
		&user.LastLogin,
		&user.LastLoginIP,
		&ipWhitelist,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("admin user not found")
		}
		return nil, fmt.Errorf("failed to get admin user: %w", err)
	}

	user.IPWhitelist = []string(ipWhitelist)
	return user, nil
}

func (r *adminUserRepository) List(ctx context.Context, filter models.AdminUserFilter) ([]*models.AdminUser, int, error) {
	var whereConditions []string
	var args []interface{}
	argIndex := 1

	// Base query conditions
	whereConditions = append(whereConditions, "deleted_at IS NULL")

	// Apply filters
	if filter.Email != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("email ILIKE $%d", argIndex))
		args = append(args, "%"+filter.Email+"%")
		argIndex++
	}
	if filter.Username != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("username ILIKE $%d", argIndex))
		args = append(args, "%"+filter.Username+"%")
		argIndex++
	}
	if filter.IsActive != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *filter.IsActive)
		argIndex++
	}
	if filter.MFAEnabled != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("mfa_enabled = $%d", argIndex))
		args = append(args, *filter.MFAEnabled)
		argIndex++
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Count total records
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM admin_users %s", whereClause)
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count admin users: %w", err)
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
		SELECT 
			id, email, username, first_name, last_name, mfa_enabled, 
			is_active, last_login, last_login_ip, ip_whitelist, 
			created_at, updated_at, deleted_at
		FROM admin_users 
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list admin users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*models.AdminUser
	for rows.Next() {
		user := &models.AdminUser{}
		var ipWhitelist pq.StringArray

		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.Username,
			&user.FirstName,
			&user.LastName,
			&user.MFAEnabled,
			&user.IsActive,
			&user.LastLogin,
			&user.LastLoginIP,
			&ipWhitelist,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.DeletedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan admin user: %w", err)
		}

		user.IPWhitelist = []string(ipWhitelist)
		users = append(users, user)
	}

	// Batch load roles for all users in a single query
	if len(users) > 0 {
		userIDs := make([]uuid.UUID, len(users))
		userMap := make(map[uuid.UUID]*models.AdminUser)
		for i, user := range users {
			userIDs[i] = user.ID
			userMap[user.ID] = user
		}

		rolesMap, err := r.batchGetUserRoles(ctx, userIDs)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to batch load roles: %w", err)
		}

		// Assign roles to users
		for userID, roles := range rolesMap {
			if user, ok := userMap[userID]; ok {
				user.Roles = roles
			}
		}
	}

	return users, totalCount, nil
}

func (r *adminUserRepository) Update(ctx context.Context, user *models.AdminUser) error {
	user.UpdatedAt = time.Now()

	query := `
		UPDATE admin_users 
		SET email = $2, username = $3, first_name = $4, last_name = $5, 
		    mfa_enabled = $6, ip_whitelist = $7, updated_at = $8
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.Email,
		user.Username,
		user.FirstName,
		user.LastName,
		user.MFAEnabled,
		pq.Array(user.IPWhitelist),
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update admin user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("admin user not found or already deleted")
	}

	return nil
}

func (r *adminUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE admin_users 
		SET deleted_at = $2, updated_at = $2 
		WHERE id = $1 AND deleted_at IS NULL
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, id, now)
	if err != nil {
		return fmt.Errorf("failed to delete admin user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("admin user not found")
	}

	return nil
}

func (r *adminUserRepository) Activate(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE admin_users 
		SET is_active = TRUE, updated_at = $2 
		WHERE id = $1 AND deleted_at IS NULL
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, id, now)
	if err != nil {
		return fmt.Errorf("failed to activate admin user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("admin user not found")
	}

	return nil
}

func (r *adminUserRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE admin_users 
		SET is_active = FALSE, updated_at = $2 
		WHERE id = $1 AND deleted_at IS NULL
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, id, now)
	if err != nil {
		return fmt.Errorf("failed to deactivate admin user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("admin user not found")
	}

	return nil
}

// Internal helper methods

// batchGetUserRoles fetches roles for multiple users in a single query
func (r *adminUserRepository) batchGetUserRoles(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID][]*models.Role, error) {
	if len(userIDs) == 0 {
		return make(map[uuid.UUID][]*models.Role), nil
	}

	// Build query with placeholders for IN clause
	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT aur.user_id, r.id, r.name, r.description, r.created_at, r.updated_at
		FROM admin_roles r
		INNER JOIN admin_user_roles aur ON r.id = aur.role_id
		WHERE aur.user_id IN (%s) AND r.deleted_at IS NULL
		ORDER BY aur.user_id, r.name
	`, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to batch get user roles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	rolesMap := make(map[uuid.UUID][]*models.Role)
	for rows.Next() {
		var userID uuid.UUID
		role := &models.Role{}
		err := rows.Scan(
			&userID,
			&role.ID,
			&role.Name,
			&role.Description,
			&role.CreatedAt,
			&role.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}

		if _, ok := rolesMap[userID]; !ok {
			rolesMap[userID] = make([]*models.Role, 0)
		}
		rolesMap[userID] = append(rolesMap[userID], role)
	}

	// Ensure all users have an entry in the map, even if they have no roles
	for _, userID := range userIDs {
		if _, ok := rolesMap[userID]; !ok {
			rolesMap[userID] = make([]*models.Role, 0)
		}
	}

	return rolesMap, nil
}

func (r *adminUserRepository) getUserRoles(ctx context.Context, userID uuid.UUID) ([]*models.Role, error) {
	query := `
		SELECT r.id, r.name, r.description, r.created_at, r.updated_at
		FROM admin_roles r
		INNER JOIN admin_user_roles aur ON r.id = aur.role_id
		WHERE aur.user_id = $1 AND r.deleted_at IS NULL
		ORDER BY r.name
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
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
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		roles = append(roles, role)
	}

	return roles, nil
}

func (r *adminUserRepository) getRolePermissions(ctx context.Context, roleID uuid.UUID) ([]*models.Permission, error) {
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
		perm := &models.Permission{}
		err := rows.Scan(
			&perm.ID,
			&perm.Resource,
			&perm.Action,
			&perm.Description,
			&perm.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

func parseRolesJSON(rolesJSON []byte, roles *[]*models.Role) error {
	// Parse the JSON array into a temporary structure
	var rolesData []map[string]interface{}
	if err := json.Unmarshal(rolesJSON, &rolesData); err != nil {
		return fmt.Errorf("failed to unmarshal roles JSON: %w", err)
	}

	// Convert to Role models
	*roles = make([]*models.Role, 0, len(rolesData))
	for _, roleData := range rolesData {
		role := &models.Role{}

		// Parse ID
		if idStr, ok := roleData["id"].(string); ok {
			if id, err := uuid.Parse(idStr); err == nil {
				role.ID = id
			}
		}

		// Parse name
		if name, ok := roleData["name"].(string); ok {
			role.Name = name
		}

		// Parse description
		if desc, ok := roleData["description"].(string); ok {
			role.Description = desc
		}

		// Parse created_at
		if createdAtStr, ok := roleData["created_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
				role.CreatedAt = t
			}
		}

		*roles = append(*roles, role)
	}

	return nil
}

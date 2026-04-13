package repositories

import (
	"database/sql"
)

// Repositories aggregates all repository interfaces for dependency injection
type Repositories struct {
	AdminUser     AdminUserRepository
	AdminUserAuth AdminUserAuthRepository
	AdminUserRole AdminUserRoleRepository
	Role          RoleRepository
	Permission    PermissionRepository
	AuditLog      AuditLogRepository
	Session       SessionRepository
}

// NewRepositories creates a new instance of all repositories
func NewRepositories(db *sql.DB) *Repositories {
	// Create admin user repositories (all three interfaces from same implementation)
	adminUser, adminUserAuth, adminUserRole := NewAdminUserRepositories(db)

	return &Repositories{
		AdminUser:     adminUser,
		AdminUserAuth: adminUserAuth,
		AdminUserRole: adminUserRole,
		Role:          NewRoleRepository(db),
		Permission:    NewPermissionRepository(db),
		AuditLog:      NewAuditLogRepository(db),
		Session:       NewSessionRepository(db),
	}
}

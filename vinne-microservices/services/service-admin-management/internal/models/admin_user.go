package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/shared/common/models"
)

type AdminUser struct {
	models.BaseModel
	Email        string          `db:"email" json:"email"`
	Username     string          `db:"username" json:"username"`
	PasswordHash string          `db:"password_hash" json:"-"`
	FirstName    *string         `db:"first_name" json:"first_name"`
	LastName     *string         `db:"last_name" json:"last_name"`
	MFASecret    *string         `db:"mfa_secret" json:"-"`
	MFAEnabled   bool            `db:"mfa_enabled" json:"mfa_enabled"`
	IsActive     bool            `db:"is_active" json:"is_active"`
	LastLogin    *time.Time      `db:"last_login" json:"last_login"`
	LastLoginIP  *string         `db:"last_login_ip" json:"last_login_ip"`
	IPWhitelist  []string        `db:"ip_whitelist" json:"ip_whitelist"`
	Roles        []*Role         `json:"roles,omitempty"`
	Sessions     []*AdminSession `json:"sessions,omitempty"`
}

type AdminSession struct {
	ID           uuid.UUID `db:"id" json:"id"`
	UserID       uuid.UUID `db:"user_id" json:"user_id"`
	RefreshToken string    `db:"refresh_token" json:"-"`
	UserAgent    string    `db:"user_agent" json:"user_agent"`
	IPAddress    string    `db:"ip_address" json:"ip_address"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	ExpiresAt    time.Time `db:"expires_at" json:"expires_at"`
	IsActive     bool      `db:"is_active" json:"is_active"`
}

type Role struct {
	ID          uuid.UUID     `db:"id" json:"id"`
	Name        string        `db:"name" json:"name"`
	Description string        `db:"description" json:"description"`
	Permissions []*Permission `json:"permissions,omitempty"`
	CreatedAt   time.Time     `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time     `db:"updated_at" json:"updated_at"`
	DeletedAt   *time.Time    `db:"deleted_at" json:"deleted_at"`
}

type Permission struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Resource    string    `db:"resource" json:"resource"`
	Action      string    `db:"action" json:"action"`
	Description *string   `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

type AuditLog struct {
	ID             uuid.UUID              `db:"id" json:"id"`
	AdminUserID    uuid.UUID              `db:"admin_user_id" json:"admin_user_id"`
	AdminUser      *AdminUser             `json:"admin_user,omitempty"`
	Action         string                 `db:"action" json:"action"`
	Resource       *string                `db:"resource" json:"resource"`
	ResourceID     *string                `db:"resource_id" json:"resource_id"`
	IPAddress      string                 `db:"ip_address" json:"ip_address"`
	UserAgent      string                 `db:"user_agent" json:"user_agent"`
	RequestData    map[string]interface{} `db:"request_data" json:"request_data"`
	ResponseStatus int                    `db:"response_status" json:"response_status"`
	CreatedAt      time.Time              `db:"created_at" json:"created_at"`
}

// Filter structs for querying
type AdminUserFilter struct {
	Email      string
	Username   string
	RoleID     *uuid.UUID
	IsActive   *bool
	MFAEnabled *bool
	Page       int
	PageSize   int
}

type RoleFilter struct {
	Name     string
	Page     int
	PageSize int
}

type PermissionFilter struct {
	Resource string
	Page     int
	PageSize int
}

type AuditLogFilter struct {
	UserID    *uuid.UUID
	Action    *string
	Resource  *string
	StartDate *time.Time
	EndDate   *time.Time
	Page      int
	PageSize  int
}

// Request/Response DTOs
type CreateAdminUserRequest struct {
	Email       string   `json:"email" validate:"required,email"`
	Username    string   `json:"username" validate:"required,min=3,max=50"`
	Password    string   `json:"password" validate:"required,min=8"`
	FirstName   *string  `json:"first_name"`
	LastName    *string  `json:"last_name"`
	RoleIDs     []string `json:"role_ids"`
	IPWhitelist []string `json:"ip_whitelist"`
}

type UpdateAdminUserRequest struct {
	ID          uuid.UUID `json:"id" validate:"required"`
	Email       *string   `json:"email" validate:"omitempty,email"`
	Username    *string   `json:"username" validate:"omitempty,min=3,max=50"`
	FirstName   *string   `json:"first_name"`
	LastName    *string   `json:"last_name"`
	IPWhitelist []string  `json:"ip_whitelist"`
	MFAEnabled  *bool     `json:"mfa_enabled"`
}

type CreateRoleRequest struct {
	Name          string   `json:"name" validate:"required,min=2,max=50"`
	Description   string   `json:"description" validate:"required,min=5,max=500"`
	PermissionIDs []string `json:"permission_ids"`
}

type UpdateRoleRequest struct {
	ID            uuid.UUID `json:"id" validate:"required"`
	Name          *string   `json:"name" validate:"omitempty,min=2,max=50"`
	Description   *string   `json:"description" validate:"omitempty,min=5,max=500"`
	PermissionIDs []string  `json:"permission_ids"`
}

type PaginatedResponse struct {
	TotalCount int `json:"total_count"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalPages int `json:"total_pages"`
}

// Authentication-related structs
type LoginRequest struct {
	Email    string  `json:"email" validate:"required,email"`
	Password string  `json:"password" validate:"required"`
	MFAToken *string `json:"mfa_token,omitempty"`
}

type LoginResponse struct {
	User         *AdminUser `json:"user"`
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
	ExpiresIn    int        `json:"expires_in"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

type EnableMFAResponse struct {
	Secret      string   `json:"secret"`
	QRCode      string   `json:"qr_code"`
	BackupCodes []string `json:"backup_codes"`
}

type VerifyMFARequest struct {
	Token string `json:"token" validate:"required,len=6"`
}

type DisableMFARequest struct {
	Token string `json:"token" validate:"required,len=6"`
}

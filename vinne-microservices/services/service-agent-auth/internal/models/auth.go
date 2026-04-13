package models

import (
	"time"

	"github.com/google/uuid"
)

// UserType represents the type of user authenticating
type UserType string

const (
	UserTypeAgent    UserType = "AGENT"
	UserTypeRetailer UserType = "RETAILER"
)

// AgentRole represents the role of an agent
type AgentRole string

const (
	AgentRoleAgent      AgentRole = "AGENT"
	AgentRoleSuperAgent AgentRole = "SUPER_AGENT"
	AgentRoleManager    AgentRole = "MANAGER"
	AgentRoleSupervisor AgentRole = "SUPERVISOR"
)

// DeviceType represents the type of device
type DeviceType string

const (
	DeviceTypePOS     DeviceType = "POS"
	DeviceTypeMobile  DeviceType = "MOBILE"
	DeviceTypeTablet  DeviceType = "TABLET"
	DeviceTypeDesktop DeviceType = "DESKTOP"
)

// DeviceStatus represents the status of a device
type DeviceStatus string

const (
	DeviceStatusActive    DeviceStatus = "ACTIVE"
	DeviceStatusInactive  DeviceStatus = "INACTIVE"
	DeviceStatusSuspended DeviceStatus = "SUSPENDED"
	DeviceStatusBlocked   DeviceStatus = "BLOCKED"
)

// NOTE: Agent business model is in agent-management service
// Auth service only deals with authentication credentials via AuthUser interface

// NOTE: Territory management not supported per user requirement

// NOTE: Commission tier management is in agent-management service

// AuthDevice represents a device for authentication purposes only
// Business device management is in agent-management service
type AuthDevice struct {
	ID           uuid.UUID    `json:"id" db:"id"`
	UserID       uuid.UUID    `json:"user_id" db:"user_id"`
	UserType     UserType     `json:"user_type" db:"user_type"`
	DeviceName   string       `json:"device_name" db:"device_name"`
	DeviceType   DeviceType   `json:"device_type" db:"device_type"`
	IMEI         string       `json:"imei" db:"imei"`
	Status       DeviceStatus `json:"status" db:"status"`
	IsActive     bool         `json:"is_active" db:"is_active"`
	LastUsed     *time.Time   `json:"last_used" db:"last_used"`
	RegisteredAt time.Time    `json:"registered_at" db:"registered_at"`
	UpdatedAt    time.Time    `json:"updated_at" db:"updated_at"`
}

// OfflineToken represents an offline token for POS devices
type OfflineToken struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	AgentID      uuid.UUID  `json:"agent_id" db:"agent_id"`
	DeviceID     uuid.UUID  `json:"device_id" db:"device_id"`
	Token        string     `json:"token" db:"token"`
	Permissions  []string   `json:"permissions" db:"permissions"`
	ExpiresAt    time.Time  `json:"expires_at" db:"expires_at"`
	IsRevoked    bool       `json:"is_revoked" db:"is_revoked"`
	RevokedBy    *string    `json:"revoked_by" db:"revoked_by"`
	RevokedAt    *time.Time `json:"revoked_at" db:"revoked_at"`
	RevokeReason *string    `json:"revoke_reason" db:"revoke_reason"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}

// Retailer represents a retailer for authentication purposes only
// Business logic and management moved to agent-management-service
type Retailer struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	RetailerCode     string     `json:"retailer_code" db:"retailer_code"`
	Email            string     `json:"email" db:"email"`
	Phone            string     `json:"phone" db:"phone"`
	PasswordHash     string     `json:"-" db:"password_hash"`
	IsActive         bool       `json:"is_active" db:"is_active"`
	LastLogin        *time.Time `json:"last_login" db:"last_login"`
	FailedLoginCount int        `json:"failed_login_count" db:"failed_login_count"`
	LockedUntil      *time.Time `json:"locked_until" db:"locked_until"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// Session represents an active session for any user type
type Session struct {
	ID           uuid.UUID `json:"id" db:"id"`
	UserID       uuid.UUID `json:"user_id" db:"user_id"`
	UserType     UserType  `json:"user_type" db:"user_type"`
	RefreshToken string    `json:"-" db:"refresh_token"`
	UserAgent    string    `json:"user_agent" db:"user_agent"`
	IPAddress    string    `json:"ip_address" db:"ip_address"`
	DeviceID     string    `json:"device_id" db:"device_id"` // For POS/Mobile device tracking
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
	LastActivity time.Time `json:"last_activity" db:"last_activity"`
}

// PasswordResetLog tracks password reset requests and their statuses
type PasswordResetLog struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	AgentID     uuid.UUID  `json:"agent_id" db:"agent_id"`
	ResetToken  string     `json:"reset_token" db:"reset_token"`
	RequestIP   string     `json:"request_ip" db:"request_ip"`
	UserAgent   string     `json:"user_agent" db:"user_agent"`
	Channel     string     `json:"channel" db:"channel"` // e.g., email or SMS
	Status      string     `json:"status" db:"status"`   // e.g., 'requested', 'otp_sent', 'validated', 'completed', 'expired', 'failed'
	OTPAttempts int        `json:"otp_attempts" db:"otp_attempts"`
	CompletedAt *time.Time `json:"completed_at" db:"completed_at"`
	ExpiresAt   time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// AuthUser interface for common authentication operations
type AuthUser interface {
	GetID() uuid.UUID
	GetEmail() string
	GetPasswordHash() string
	IsLocked() bool
	CanLogin() bool
}

// Interface for PIN change logging
type PINChangeLog struct {
	ID                  uuid.UUID  `db:"id" json:"id"`
	RetailerID          uuid.UUID  `db:"retailer_id" json:"retailer_id"`
	RetailerCode        string     `db:"retailer_code" json:"retailer_code"`
	ChangedBy           uuid.UUID  `db:"changed_by" json:"changed_by"`
	ChangeReason        string    `db:"change_reason" json:"change_reason,omitempty"`
	DeviceIMEI          string    `db:"device_imei" json:"device_imei,omitempty"`
	IPAddress           string    `db:"ip_address" json:"ip_address,omitempty"`
	UserAgent           string    `db:"user_agent" json:"user_agent,omitempty"`
	Success             bool       `db:"success" json:"success"`
	FailureReason       string    `db:"failure_reason" json:"failure_reason,omitempty"`
	SessionsInvalidated int        `db:"sessions_invalidated" json:"sessions_invalidated"`
	CreatedAt           time.Time  `db:"created_at" json:"created_at"`
}

// NOTE: Agent helper methods removed - use AuthUser interface instead

// Helper methods for Retailer

func (r *Retailer) GetID() uuid.UUID {
	return r.ID
}

func (r *Retailer) GetEmail() string {
	return r.Email
}

func (r *Retailer) GetPasswordHash() string {
	return r.PasswordHash
}

func (r *Retailer) IsLocked() bool {
	return r.LockedUntil != nil && r.LockedUntil.After(time.Now())
}

func (r *Retailer) CanLogin() bool {
	return r.IsActive && !r.IsLocked()
}

// Helper methods for Session

func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

func (s *Session) IsValid() bool {
	return s.IsActive && !s.IsExpired()
}

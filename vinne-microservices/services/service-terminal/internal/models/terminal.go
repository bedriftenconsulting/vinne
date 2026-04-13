package models

import (
	"time"

	"github.com/google/uuid"
)

// TerminalStatus represents the status of a terminal
type TerminalStatus string

const (
	TerminalStatusActive         TerminalStatus = "ACTIVE"
	TerminalStatusInactive       TerminalStatus = "INACTIVE"
	TerminalStatusFaulty         TerminalStatus = "FAULTY"
	TerminalStatusMaintenance    TerminalStatus = "MAINTENANCE"
	TerminalStatusSuspended      TerminalStatus = "SUSPENDED"
	TerminalStatusDecommissioned TerminalStatus = "DECOMMISSIONED"
)

// HealthStatus represents the health status of a terminal
type HealthStatus string

const (
	HealthStatusHealthy  HealthStatus = "HEALTHY"
	HealthStatusWarning  HealthStatus = "WARNING"
	HealthStatusCritical HealthStatus = "CRITICAL"
	HealthStatusOffline  HealthStatus = "OFFLINE"
)

// TerminalModel represents the model/type of terminal
type TerminalModel string

const (
	TerminalModelAndroidPOSV1 TerminalModel = "ANDROID_POS_V1"
	TerminalModelAndroidPOSV2 TerminalModel = "ANDROID_POS_V2"
	TerminalModelWebTerminal  TerminalModel = "WEB_TERMINAL"
	TerminalModelMobile       TerminalModel = "MOBILE_TERMINAL"
)

// Terminal represents a POS terminal device
type Terminal struct {
	ID                uuid.UUID         `db:"id" json:"id"`
	DeviceID          string            `db:"device_id" json:"device_id"`
	Name              string            `db:"name" json:"name"`
	Model             TerminalModel     `db:"model" json:"model"`
	SerialNumber      string            `db:"serial_number" json:"serial_number"`
	IMEI              string            `db:"imei" json:"imei"`
	AndroidVersion    string            `db:"android_version" json:"android_version"`
	AppVersion        string            `db:"app_version" json:"app_version"`
	Vendor            string            `db:"vendor" json:"vendor"`
	Manufacturer      string            `db:"manufacturer" json:"manufacturer"`
	PurchaseDate      *time.Time        `db:"purchase_date" json:"purchase_date"`
	Status            TerminalStatus    `db:"status" json:"status"`
	HealthStatus      HealthStatus      `db:"health_status" json:"health_status"`
	RetailerID        *uuid.UUID        `db:"retailer_id" json:"retailer_id"`
	AssignmentDate    *time.Time        `db:"assignment_date" json:"assignment_date"`
	LastSync          *time.Time        `db:"last_sync" json:"last_sync"`
	LastTransaction   *time.Time        `db:"last_transaction" json:"last_transaction"`
	LastHeartbeat     *time.Time        `db:"last_heartbeat" json:"last_heartbeat"`
	Location          string            `db:"location" json:"location"`
	TotalTransactions int64             `db:"total_transactions" json:"total_transactions"`
	TotalValue        int64             `db:"total_value" json:"total_value"` // in cents
	IsOnline          bool              `db:"is_online" json:"is_online"`
	Metadata          map[string]string `db:"metadata" json:"metadata"`
	CreatedAt         time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time         `db:"updated_at" json:"updated_at"`
	DeletedAt         *time.Time        `db:"deleted_at" json:"-"`
	DeletedBy         *uuid.UUID        `db:"deleted_by" json:"deleted_by,omitempty"`

	// Virtual fields for API responses
	Retailer       string `json:"retailer,omitempty"`
	Agent          string `json:"agent,omitempty"`
	BatteryLevel   int    `json:"battery_level,omitempty"`
	SignalStrength int    `json:"signal_strength,omitempty"`
}

// TableName specifies the table name for Terminal
func (Terminal) TableName() string {
	return "terminals"
}

// TerminalAssignment represents the assignment of a terminal to a retailer
type TerminalAssignment struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	TerminalID   uuid.UUID  `db:"terminal_id" json:"terminal_id"`
	Terminal     *Terminal  `json:"terminal,omitempty"`
	RetailerID   uuid.UUID  `db:"retailer_id" json:"retailer_id"`
	AssignedBy   uuid.UUID  `db:"assigned_by" json:"assigned_by"`
	AssignedAt   time.Time  `db:"assigned_at" json:"assigned_at"`
	UnassignedAt *time.Time `db:"unassigned_at" json:"unassigned_at"`
	IsActive     bool       `db:"is_active" json:"is_active"`
	Notes        string     `db:"notes" json:"notes"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at,omitempty" json:"-"`
}

// TableName specifies the table name for TerminalAssignment
func (TerminalAssignment) TableName() string {
	return "terminal_assignments"
}

// TerminalVersion represents terminal software version information
type TerminalVersion struct {
	ID             uuid.UUID  `db:"id" json:"id"`
	TerminalID     uuid.UUID  `db:"terminal_id" json:"terminal_id"`
	Terminal       *Terminal  `db:"-" json:"terminal,omitempty"`
	AppVersion     string     `db:"app_version" json:"app_version"`
	AndroidVersion string     `db:"android_version" json:"android_version"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
	UpdatedBy      string     `db:"updated_by" json:"updated_by"`
	UpdateNotes    string     `db:"update_notes" json:"update_notes"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	DeletedAt      *time.Time `db:"deleted_at" json:"-"`
}

// TableName specifies the table name for TerminalVersion
func (TerminalVersion) TableName() string {
	return "terminal_versions"
}

// TerminalConfig represents terminal configuration settings
type TerminalConfig struct {
	ID                  uuid.UUID         `db:"id" json:"id"`
	TerminalID          uuid.UUID         `db:"terminal_id" json:"terminal_id"`
	Terminal            *Terminal         `db:"-" json:"terminal,omitempty"`
	TransactionLimit    int               `db:"transaction_limit" json:"transaction_limit"`
	DailyLimit          int               `db:"daily_limit" json:"daily_limit"`
	OfflineModeEnabled  bool              `db:"offline_mode_enabled" json:"offline_mode_enabled"`
	OfflineSyncInterval int               `db:"offline_sync_interval" json:"offline_sync_interval"` // in minutes
	AutoUpdateEnabled   bool              `db:"auto_update_enabled" json:"auto_update_enabled"`
	MinimumAppVersion   string            `db:"minimum_app_version" json:"minimum_app_version"`
	Settings            map[string]string `db:"settings" json:"settings"`
	CreatedAt           time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time         `db:"updated_at" json:"updated_at"`
	DeletedAt           *time.Time        `db:"deleted_at" json:"-"`
}

// TableName specifies the table name for TerminalConfig
func (TerminalConfig) TableName() string {
	return "terminal_configs"
}

// TerminalHealth represents the health status of a terminal
type TerminalHealth struct {
	ID               uuid.UUID         `db:"id" json:"id"`
	TerminalID       uuid.UUID         `db:"terminal_id" json:"terminal_id"`
	Terminal         *Terminal         `db:"-" json:"terminal,omitempty"`
	Status           HealthStatus      `db:"status" json:"status"`
	BatteryLevel     int               `db:"battery_level" json:"battery_level"`
	SignalStrength   int               `db:"signal_strength" json:"signal_strength"`
	StorageAvailable int64             `db:"storage_available" json:"storage_available"` // in bytes
	StorageTotal     int64             `db:"storage_total" json:"storage_total"`         // in bytes
	MemoryUsage      int               `db:"memory_usage" json:"memory_usage"`           // percentage
	CPUUsage         int               `db:"cpu_usage" json:"cpu_usage"`                 // percentage
	LastHeartbeat    time.Time         `db:"last_heartbeat" json:"last_heartbeat"`
	Diagnostics      map[string]string `db:"diagnostics" json:"diagnostics"`
	CreatedAt        time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time         `db:"updated_at" json:"updated_at"`
}

// TableName specifies the table name for TerminalHealth
func (TerminalHealth) TableName() string {
	return "terminal_health"
}

// TerminalHealthHistory represents historical health records
type TerminalHealthHistory struct {
	ID               uuid.UUID         `db:"id" json:"id"`
	TerminalID       uuid.UUID         `db:"terminal_id" json:"terminal_id"`
	Terminal         *Terminal         `db:"-" json:"terminal,omitempty"`
	Status           HealthStatus      `db:"status" json:"status"`
	BatteryLevel     int               `db:"battery_level" json:"battery_level"`
	SignalStrength   int               `db:"signal_strength" json:"signal_strength"`
	StorageAvailable int64             `db:"storage_available" json:"storage_available"`
	StorageTotal     int64             `db:"storage_total" json:"storage_total"`
	MemoryUsage      int               `db:"memory_usage" json:"memory_usage"`
	CPUUsage         int               `db:"cpu_usage" json:"cpu_usage"`
	Diagnostics      map[string]string `db:"diagnostics" json:"diagnostics"`
	RecordedAt       time.Time         `db:"recorded_at" json:"recorded_at"`
	CreatedAt        time.Time         `db:"created_at" json:"created_at"`
}

// TableName specifies the table name for TerminalHealthHistory
func (TerminalHealthHistory) TableName() string {
	return "terminal_health_history"
}

// TerminalAuditLog represents audit log entries for terminal operations
type TerminalAuditLog struct {
	ID          uuid.UUID         `db:"id" json:"id"`
	TerminalID  uuid.UUID         `db:"terminal_id" json:"terminal_id"`
	Terminal    *Terminal         `db:"-" json:"terminal,omitempty"`
	Action      string            `db:"action" json:"action"`
	Details     map[string]string `db:"details" json:"details"`
	PerformedBy string            `db:"action_by" json:"action_by"`
	IPAddress   string            `db:"ip_address" json:"ip_address"`
	UserAgent   string            `db:"user_agent" json:"user_agent"`
	CreatedAt   time.Time         `db:"created_at" json:"created_at"`
}

// TableName specifies the table name for TerminalAuditLog
func (TerminalAuditLog) TableName() string {
	return "terminal_audit_logs"
}

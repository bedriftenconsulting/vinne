package models

import (
	"time"

	"github.com/google/uuid"
)

// BaseModel contains common fields for all domain models
type BaseModel struct {
	ID        uuid.UUID  `json:"id" db:"id" gorm:"type:uuid;primary_key"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	Version   int        `json:"version" db:"version"` // For optimistic locking
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"` // Soft delete
}

// BeforeCreate sets default values before creating a new record
func (b *BaseModel) BeforeCreate() {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	now := time.Now().UTC()
	b.CreatedAt = now
	b.UpdatedAt = now
	b.Version = 1
}

// BeforeUpdate updates the timestamp and increments version
func (b *BaseModel) BeforeUpdate() {
	b.UpdatedAt = time.Now().UTC()
	b.Version++
}

// IsDeleted checks if the record is soft deleted
func (b *BaseModel) IsDeleted() bool {
	return b.DeletedAt != nil
}

// SoftDelete marks the record as deleted
func (b *BaseModel) SoftDelete() {
	now := time.Now().UTC()
	b.DeletedAt = &now
	b.UpdatedAt = now
}

// Restore undeletes a soft deleted record
func (b *BaseModel) Restore() {
	b.DeletedAt = nil
	b.UpdatedAt = time.Now().UTC()
}

// TableName interface for custom table names
type TableNamer interface {
	TableName() string
}

// Auditable interface for models that need audit trails
type Auditable interface {
	GetAuditInfo() AuditInfo
}

// AuditInfo contains audit trail information
type AuditInfo struct {
	CreatedBy string    `json:"created_by,omitempty" db:"created_by"`
	UpdatedBy string    `json:"updated_by,omitempty" db:"updated_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	Action    string    `json:"action,omitempty"`
	Changes   map[string]interface{} `json:"changes,omitempty"`
}
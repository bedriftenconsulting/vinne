package models

import (
	"time"

	"github.com/google/uuid"
)

// IdempotencyRecord represents an idempotency record for preventing duplicate requests
type IdempotencyRecord struct {
	ID             uuid.UUID              `db:"id"`
	IdempotencyKey string                 `db:"idempotency_key"`
	RequestHash    string                 `db:"request_hash"`
	Endpoint       string                 `db:"endpoint"`
	HTTPMethod     string                 `db:"http_method"`
	StatusCode     int                    `db:"status_code"`
	ResponseBody   map[string]interface{} `db:"response_body"`
	TransactionID  *uuid.UUID             `db:"transaction_id"`
	IsLocked       bool                   `db:"is_locked"`
	LockedAt       *time.Time             `db:"locked_at"`
	LockExpiresAt  *time.Time             `db:"lock_expires_at"`
	CreatedAt      time.Time              `db:"created_at"`
	UpdatedAt      time.Time              `db:"updated_at"`
	ExpiresAt      time.Time              `db:"expires_at"`
}

// IsExpired checks if the idempotency record has expired
func (i *IdempotencyRecord) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// IsLockExpired checks if the lock has expired
func (i *IdempotencyRecord) IsLockExpired() bool {
	if !i.IsLocked || i.LockExpiresAt == nil {
		return true
	}
	return time.Now().After(*i.LockExpiresAt)
}

// AcquireLock attempts to acquire a lock on the idempotency record
func (i *IdempotencyRecord) AcquireLock(lockDuration time.Duration) {
	now := time.Now()
	lockExpiry := now.Add(lockDuration)
	i.IsLocked = true
	i.LockedAt = &now
	i.LockExpiresAt = &lockExpiry
	i.UpdatedAt = now
}

// ReleaseLock releases the lock on the idempotency record
func (i *IdempotencyRecord) ReleaseLock() {
	i.IsLocked = false
	i.LockedAt = nil
	i.LockExpiresAt = nil
	i.UpdatedAt = time.Now()
}

// SetResponse sets the response data for the idempotency record
func (i *IdempotencyRecord) SetResponse(statusCode int, responseBody map[string]interface{}, transactionID *uuid.UUID) {
	i.StatusCode = statusCode
	i.ResponseBody = responseBody
	i.TransactionID = transactionID
	i.UpdatedAt = time.Now()
}

// CreateIdempotencyRecord creates a new idempotency record
func CreateIdempotencyRecord(
	idempotencyKey string,
	requestHash string,
	endpoint string,
	httpMethod string,
	ttl time.Duration,
) *IdempotencyRecord {
	now := time.Now()
	return &IdempotencyRecord{
		ID:             uuid.New(),
		IdempotencyKey: idempotencyKey,
		RequestHash:    requestHash,
		Endpoint:       endpoint,
		HTTPMethod:     httpMethod,
		IsLocked:       false,
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiresAt:      now.Add(ttl),
		ResponseBody:   make(map[string]interface{}),
	}
}

package models

import (
	"time"

	"github.com/google/uuid"
)

type OTP struct {
	ID          uuid.UUID `json:"id" db:"id"`
	PhoneNumber string    `json:"phone_number" db:"phone_number"`
	Code        string    `json:"code" db:"code"`
	Purpose     string    `json:"purpose" db:"purpose"`
	ExpiresAt   time.Time `json:"expires_at" db:"expires_at"`
	IsUsed      bool      `json:"is_used" db:"is_used"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UsedAt      time.Time `json:"used_at" db:"used_at"`
}

type CreateOTPRequest struct {
	PhoneNumber string
	Purpose     string
	ExpiresIn   time.Duration
}

type VerifyOTPRequest struct {
	PhoneNumber string
	Code        string
	Purpose     string
}

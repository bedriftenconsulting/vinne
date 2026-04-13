package models

import (
	"time"

	"github.com/google/uuid"
)

type ValidateLoginRequest struct {
	PhoneNumber string `json:"phone_number" db:"phone_number"`
	Password    string `json:"password" db:"password"`
	DeviceType  string `json:"device_type" db:"device_type"`
	DeviceID    string `json:"device_id" db:"device_id"`
	Channel     string `json:"channel" db:"channel"`
	IPAddress   string `json:"ip_address" db:"ip_address"`
	AppVersion  string `json:"app_version" db:"app_version"`
	UserAgent   string `json:"user_agent" db:"user_agent"`
}

type LoginAttempt struct {
	ID            uuid.UUID `json:"id" db:"id"`
	PhoneNumber   string    `json:"phone_number" db:"phone_number"`
	PlayerID      uuid.UUID `json:"player_id" db:"player_id"`
	DeviceID      string    `json:"device_id" db:"device_id"`
	Channel       string    `json:"channel" db:"channel"`
	IPAddress     string    `json:"ip_address" db:"ip_address"`
	AttemptType   string    `json:"attempt_type" db:"attempt_type"`
	Success       bool      `json:"success" db:"success"`
	FailureReason *string   `json:"failure_reason" db:"failure_reason"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

type AuditLog struct {
	ID          uuid.UUID      `json:"id" db:"id"`
	PlayerID    uuid.UUID      `json:"player_id" db:"player_id"`
	Action      string         `json:"action" db:"action"`
	Channel     string         `json:"channel" db:"channel"`
	PerformedBy uuid.UUID      `json:"performed_by" db:"performed_by"`
	IPAddress   string         `json:"ip_address" db:"ip_address"`
	UserAgent   string         `json:"user_agent" db:"user_agent"`
	OldValue    map[string]any `json:"old_value" db:"old_value"`
	NewValue    map[string]any `json:"new_value" db:"new_value"`
	Metadata    map[string]any `json:"metadata" db:"metadata"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
}

type ChannelAnalytics struct {
	ID                   uuid.UUID `json:"id" db:"id"`
	PlayerID             uuid.UUID `json:"player_id" db:"player_id"`
	Channel              string    `json:"channel" db:"channel"`
	LoginCount           int       `json:"login_count" db:"login_count"`
	LastLoginAt          time.Time `json:"last_login_at" db:"last_login_at"`
	TotalSessionDuration int64     `json:"total_session_duration" db:"total_session_duration"`
	DeviceTypes          []string  `json:"device_types" db:"device_types"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time `json:"updated_at" db:"updated_at"`
}

type PlayerFeedback struct {
	ID        uuid.UUID `json:"id" db:"id"`
	PlayerID  uuid.UUID `json:"player_id" db:"player_id"`
	FullName  string    `json:"full_name" db:"full_name"`
	Email     string    `json:"email" db:"email"`
	Message   string    `json:"message" db:"message"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type CreateFeedbackRequest struct {
	PlayerID uuid.UUID
	FullName string
	Email    string
	Message  string
}

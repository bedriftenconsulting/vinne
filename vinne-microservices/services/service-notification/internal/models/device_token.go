package models

import "time"

// DeviceToken represents an FCM device token for push notifications
type DeviceToken struct {
	ID         string     `db:"id" json:"id"`
	RetailerID string     `db:"retailer_id" json:"retailer_id"` // 8-digit retailer code
	DeviceID   string     `db:"device_id" json:"device_id"`     // Android device ID
	FCMToken   string     `db:"fcm_token" json:"fcm_token"`     // Firebase Cloud Messaging token
	Platform   string     `db:"platform" json:"platform"`       // 'android' or 'ios'
	AppVersion *string    `db:"app_version" json:"app_version,omitempty"`
	IsActive   bool       `db:"is_active" json:"is_active"`
	LastUsedAt *time.Time `db:"last_used_at" json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at" json:"updated_at"`
}

// CreateDeviceTokenRequest represents the request to register a device token
type CreateDeviceTokenRequest struct {
	RetailerID string `json:"retailer_id" validate:"required"`
	DeviceID   string `json:"device_id" validate:"required"`
	FCMToken   string `json:"fcm_token" validate:"required"`
	Platform   string `json:"platform" validate:"required,oneof=android ios"`
	AppVersion string `json:"app_version,omitempty"`
}

package models

import (
	"time"

	"github.com/google/uuid"
)

type PlayerDevice struct {
	ID          uuid.UUID `json:"id" db:"id"`
	PlayerID    uuid.UUID `json:"player_id" db:"player_id"`
	DeviceID    string    `json:"device_id" db:"device_id"`
	DeviceType  string    `json:"device_type" db:"device_type"`
	DeviceName  string    `json:"device_name" db:"device_name"`
	DeviceOS    string    `json:"device_os" db:"os"`
	OSVersion   string    `json:"os_version" db:"os_version"`
	AppVersion  string    `json:"app_version" db:"app_version"`
	PushToken   string    `json:"push_token" db:"push_token"`
	Fingerprint string    `json:"fingerprint" db:"fingerprint"`
	IsTrusted   bool      `json:"is_trusted" db:"is_trusted"`
	IsBlocked   bool      `json:"is_blocked" db:"is_blocked"`
	FirstSeenAt time.Time `json:"first_seen_at" db:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at" db:"last_seen_at"`
	TrustScore  int       `json:"trust_score" db:"trust_score"`
}

type CreateDeviceRequest struct {
	PlayerID    uuid.UUID
	DeviceID    string
	DeviceType  string
	DeviceName  string
	OS          string
	OSVersion   string
	AppVersion  string
	PushToken   string
	Fingerprint string
	TrustScore  int
}

type UpdateDeviceRequest struct {
	ID          uuid.UUID
	DeviceName  string
	PushToken   string
	Fingerprint string
	IsTrusted   bool
	IsBlocked   bool
	LastSeenAt  time.Time
	TrustScore  int
}

type DeviceFilter struct {
	PlayerID      uuid.UUID
	DeviceID      string
	DeviceType    string
	IsTrusted     bool
	IsBlocked     bool
	TrustScoreMin int
	TrustScoreMax int
	Limit         int
	Offset        int
}

type DeviceType string

const (
	DeviceTypeMobile  DeviceType = "mobile"
	DeviceTypeTablet  DeviceType = "tablet"
	DeviceTypeDesktop DeviceType = "desktop"
)

func (dt DeviceType) IsValid() bool {
	switch dt {
	case DeviceTypeMobile, DeviceTypeTablet, DeviceTypeDesktop:
		return true
	default:
		return false
	}
}

func (dt DeviceType) String() string {
	return string(dt)
}

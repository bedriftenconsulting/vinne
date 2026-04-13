package models

import (
	"time"

	"github.com/google/uuid"
)

type PlayerSession struct {
	ID             uuid.UUID `json:"id" db:"id"`
	PlayerID       uuid.UUID `json:"player_id" db:"player_id"`
	DeviceID       string    `json:"device_id" db:"device_id"`
	RefreshToken   string    `json:"-" db:"refresh_token"`
	AccessTokenJTI string    `json:"-" db:"access_token_jti"`
	Channel        string    `json:"channel" db:"channel"`
	DeviceType     string    `json:"device_type" db:"device_type"`
	AppVersion     string    `json:"app_version" db:"app_version"`
	IPAddress      string    `json:"ip_address" db:"ip_address"`
	UserAgent      string    `json:"user_agent" db:"user_agent"`
	IsActive       bool      `json:"is_active" db:"is_active"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	ExpiresAt      time.Time `json:"expires_at" db:"expires_at"`
	LastUsedAt     time.Time `json:"last_used_at" db:"last_used_at"`
	RevokedAt      time.Time `json:"revoked_at" db:"revoked_at"`
	RevokedReason  string    `json:"revoked_reason" db:"revoked_reason"`
}

type CreateSessionRequest struct {
	PlayerID       uuid.UUID
	DeviceID       string
	RefreshToken   string
	AccessTokenJTI string
	Channel        string
	DeviceType     string
	AppVersion     string
	IPAddress      string
	UserAgent      string
	ExpiresAt      time.Time
}

type UpdateSessionRequest struct {
	ID             uuid.UUID
	AccessTokenJTI string
	LastUsedAt     time.Time
	IsActive       bool
	RevokedAt      time.Time
	RevokedReason  string
}

type SessionFilter struct {
	PlayerID    uuid.UUID
	DeviceID    string
	Channel     string
	IsActive    bool
	ExpiresFrom time.Time
	ExpiresTo   time.Time
	Limit       int
	Offset      int
}

type Channel string

const (
	ChannelMobile   Channel = "mobile"
	ChannelWeb      Channel = "web"
	ChannelTelegram Channel = "telegram"
	ChannelUSSD     Channel = "ussd"
)

func (c Channel) IsValid() bool {
	switch c {
	case ChannelMobile, ChannelWeb, ChannelTelegram, ChannelUSSD:
		return true
	default:
		return false
	}
}

func (c Channel) String() string {
	return string(c)
}

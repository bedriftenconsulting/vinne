package models

import (
	"time"

	"github.com/google/uuid"
)

type Player struct {
	ID                  uuid.UUID    `json:"id" db:"id"`
	PhoneNumber         string       `json:"phone_number" db:"phone_number"`
	Email               string       `json:"email" db:"email"`
	PasswordHash        string       `json:"-" db:"password_hash"`
	FirstName           string       `json:"first_name" db:"first_name"`
	LastName            string       `json:"last_name" db:"last_name"`
	DateOfBirth         time.Time    `json:"date_of_birth" db:"date_of_birth"`
	NationalID          string       `json:"national_id" db:"national_id"`
	MobileMoneyPhone    string       `json:"mobile_money_phone" db:"mobile_money_phone"`
	Status              PlayerStatus `json:"status" db:"status"`
	EmailVerified       bool         `json:"email_verified" db:"email_verified"`
	PhoneVerified       bool         `json:"phone_verified" db:"phone_verified"`
	RegistrationChannel string       `json:"registration_channel" db:"registration_channel"`
	TermsAccepted       bool         `json:"terms_accepted" db:"terms_accepted"`
	MarketingConsent    bool         `json:"marketing_consent" db:"marketing_consent"`
	CreatedAt           time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at" db:"updated_at"`
	LastLoginAt         time.Time    `json:"last_login_at" db:"last_login_at"`
	DeletedAt           time.Time    `json:"deleted_at" db:"deleted_at"`
}

type PlayerStatus string

const (
	PlayerStatusActive    PlayerStatus = "ACTIVE"
	PlayerStatusSuspended PlayerStatus = "SUSPENDED"
	PlayerStatusBanned    PlayerStatus = "BANNED"
)

func (ps PlayerStatus) IsValid() bool {
	switch ps {
	case PlayerStatusActive, PlayerStatusSuspended, PlayerStatusBanned:
		return true
	default:
		return false
	}
}

func (ps PlayerStatus) String() string {
	return string(ps)
}

type CreatePlayerRequest struct {
	PhoneNumber         string
	Email               string
	PasswordHash        string
	FirstName           string
	LastName            string
	DateOfBirth         time.Time
	NationalID          string
	MobileMoneyPhone    string
	RegistrationChannel string
	TermsAccepted       bool
	MarketingConsent    bool
}

type UpdatePlayerRequest struct {
	ID               uuid.UUID
	Email            string
	FirstName        string
	LastName         string
	DateOfBirth      time.Time
	NationalID       string
	MobileMoneyPhone string
	Status           PlayerStatus
	EmailVerified    bool
	PhoneVerified    bool
	LastLoginAt      time.Time
}

type PlayerFilter struct {
	PhoneNumber string
	Email       string
	Status      PlayerStatus
	Channel     string
	CreatedFrom time.Time
	CreatedTo   time.Time
	Limit       int
	Offset      int
}

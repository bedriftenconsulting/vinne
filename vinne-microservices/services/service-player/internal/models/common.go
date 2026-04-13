package models

import (
	"time"

	"github.com/google/uuid"
)

// TokenPair represents a pair of access and refresh tokens
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// TokenClaims represents JWT token claims
type TokenClaims struct {
	PlayerID uuid.UUID `json:"sub"`
	Phone    string    `json:"phone"`
	Channel  string    `json:"channel"`
	DeviceID string    `json:"device_id"`
	Exp      int64     `json:"exp"`
	Iat      int64     `json:"iat"`
	Jti      string    `json:"jti"`
}

// USSDRegisterRequest represents USSD registration request
type USSDRegisterRequest struct {
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password"`
	SessionID   string `json:"session_id"`
}

// WalletBalance represents wallet balance information
type WalletBalance struct {
	Balance          int64     `json:"balance"`
	PendingBalance   int64     `json:"pending_balance"`
	AvailableBalance int64     `json:"available_balance"`
	LastUpdated      time.Time `json:"last_updated"`
}

// DepositRequest represents a deposit request
type DepositRequest struct {
	PlayerID         uuid.UUID `json:"player_id"`
	Amount           int64     `json:"amount"` // in pesewas
	MobileMoneyPhone string    `json:"mobile_money_phone"`
	PaymentMethod    string    `json:"payment_method"`
}

// DepositResponse represents a deposit response
type DepositResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

// WithdrawalRequest represents a withdrawal request
type WithdrawalRequest struct {
	PlayerID         uuid.UUID `json:"player_id"`
	Amount           int64     `json:"amount"` // in pesewas
	MobileMoneyPhone string    `json:"mobile_money_phone"`
}

// WithdrawalResponse represents a withdrawal response
type WithdrawalResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

// TransactionFilter represents filters for transaction queries
type TransactionFilter struct {
	Type     string     `json:"type"`
	FromDate *time.Time `json:"from_date"`
	ToDate   *time.Time `json:"to_date"`
	Page     int        `json:"page"`
	PerPage  int        `json:"per_page"`
}

// RegistrationRequest represents a registration request from client
type RegistrationRequest struct {
	PhoneNumber         string    `json:"phone_number"`
	Password            string    `json:"password"`
	Email               string    `json:"email"`
	FirstName           string    `json:"first_name"`
	LastName            string    `json:"last_name"`
	DateOfBirth         time.Time `json:"date_of_birth"`
	NationalID          string    `json:"national_id"`
	MobileMoneyPhone    string    `json:"mobile_money_phone"`
	DeviceID            string    `json:"device_id"`
	RegistrationChannel string    `json:"registration_channel"`
	TermsAccepted       bool      `json:"terms_accepted"`
	MarketingConsent    bool      `json:"marketing_consent"`
}

// Transaction represents a financial transaction
type Transaction struct {
	ID        uuid.UUID `json:"id"`
	PlayerID  uuid.UUID `json:"player_id"`
	Type      string    `json:"type"`   // deposit, withdrawal, bet, win, refund
	Amount    int64     `json:"amount"` // in pesewas
	Reference string    `json:"reference"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/randco/service-player/internal/models"
)

// AuthService handles authentication operations
type AuthService interface {
	ValidateCredentials(ctx context.Context, req models.ValidateLoginRequest) (*models.Player, error)
	GenerateTokens(ctx context.Context, player *models.Player, deviceID, channel, deviceType, appVersion, ipAddress, userAgent string) (*models.TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (*models.TokenPair, error)
	RevokeToken(ctx context.Context, refreshToken string) error
	ValidateToken(ctx context.Context, token string) (*models.TokenClaims, error)
	RequestPasswordReset(ctx context.Context, phoneNumber string) (string, error) // Returns session ID
	ValidatePasswordResetOTP(ctx context.Context, sessionID, otp string) error
	ConfirmPasswordReset(ctx context.Context, playerID uuid.UUID, newPassword, otp string) error
	SubmitFeedback(ctx context.Context, req models.CreateFeedbackRequest) (*models.PlayerFeedback, error)
}

// RegistrationService handles player registration
type RegistrationService interface {
	RegisterPlayer(ctx context.Context, req models.RegistrationRequest) (*models.Player, error)
	VerifyOTP(ctx context.Context, phoneNumber, otp string) error
	USSDRegister(ctx context.Context, req models.USSDRegisterRequest) (*models.Player, error)
}

// ProfileService handles player profile operations
type ProfileService interface {
	GetProfile(ctx context.Context, playerID uuid.UUID) (*models.Player, error)
	UpdateProfile(ctx context.Context, req models.UpdatePlayerRequest) (*models.Player, error)
	ChangePassword(ctx context.Context, playerID uuid.UUID, currentPassword, newPassword string) error
	UpdateMobileMoneyPhone(ctx context.Context, playerID uuid.UUID, phoneNumber, otp string) error
}

// WalletService handles wallet operations (delegates to Wallet Service)
type WalletService interface {
	GetBalance(ctx context.Context, playerID uuid.UUID) (*models.WalletBalance, error)
	InitiateDeposit(ctx context.Context, req models.DepositRequest) (*models.DepositResponse, error)
	InitiateWithdrawal(ctx context.Context, req models.WithdrawalRequest) (*models.WithdrawalResponse, error)
	GetTransactionHistory(ctx context.Context, playerID uuid.UUID, filter models.TransactionFilter) ([]*models.Transaction, error)
}

// SessionService handles session management
type SessionService interface {
	CreateSession(ctx context.Context, playerID uuid.UUID, deviceID, channel string) (*models.PlayerSession, error)
	GetSession(ctx context.Context, sessionID uuid.UUID) (*models.PlayerSession, error)
	UpdateSession(ctx context.Context, sessionID uuid.UUID, updates models.UpdateSessionRequest) (*models.PlayerSession, error)
	RevokeSession(ctx context.Context, sessionID uuid.UUID, reason string) error
	GetPlayerSessions(ctx context.Context, playerID uuid.UUID) ([]*models.PlayerSession, error)
	CleanupExpiredSessions(ctx context.Context) error
}

// AdminService handles admin operations for player management
type AdminService interface {
	SearchPlayers(ctx context.Context, query string, page, perPage int) ([]*models.Player, int64, error)
	SuspendPlayer(ctx context.Context, playerID uuid.UUID, reason string) error
	ActivatePlayer(ctx context.Context, playerID uuid.UUID) error
}

// OTPService handles OTP generation, storage, and verification
type OTPService interface {
	GenerateAndSendOTP(ctx context.Context, phoneNumber, purpose string) error
	VerifyOTP(ctx context.Context, sessionID, code, purpose string) error
	CleanupExpiredOTPs(ctx context.Context) error
	ResendRegistrationOTP(ctx context.Context, phoneNumber string) error
}

package services

import (
	"context"
	"fmt"

	"github.com/randco/randco-microservices/shared/validation"
	"github.com/randco/service-player/internal/clients"
	"github.com/randco/service-player/internal/models"
	"github.com/randco/service-player/internal/repositories"
	"golang.org/x/crypto/bcrypt"
)

type registrationService struct {
	playerRepo   repositories.PlayerRepository
	authRepo     repositories.PlayerAuthRepository
	otpService   OTPService
	walletClient *clients.WalletClient
}

func NewRegistrationService(
	playerRepo repositories.PlayerRepository,
	authRepo repositories.PlayerAuthRepository,
	otpService OTPService,
	walletClient *clients.WalletClient,
) RegistrationService {
	return &registrationService{
		playerRepo:   playerRepo,
		authRepo:     authRepo,
		otpService:   otpService,
		walletClient: walletClient,
	}
}

// RegisterPlayer registers a new player
func (s *registrationService) RegisterPlayer(ctx context.Context, req models.RegistrationRequest) (*models.Player, error) {
	normalizedPhone := validation.NormalizePhone(req.PhoneNumber)

	existingPlayer, err := s.playerRepo.GetByPhoneNumber(ctx, normalizedPhone)
	if err == nil && existingPlayer != nil {
		return nil, fmt.Errorf("phone number already registered")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	normalizedMobileMoneyPhone := req.MobileMoneyPhone
	if normalizedMobileMoneyPhone != "" {
		normalizedMobileMoneyPhone = validation.NormalizePhone(req.MobileMoneyPhone)
	}

	playerReq := models.CreatePlayerRequest{
		PhoneNumber:         normalizedPhone,
		Email:               req.Email,
		PasswordHash:        string(hashedPassword),
		FirstName:           req.FirstName,
		LastName:            req.LastName,
		DateOfBirth:         req.DateOfBirth,
		NationalID:          req.NationalID,
		MobileMoneyPhone:    normalizedMobileMoneyPhone,
		RegistrationChannel: req.RegistrationChannel,
		TermsAccepted:       req.TermsAccepted,
		MarketingConsent:    req.MarketingConsent,
	}

	createdPlayer, err := s.playerRepo.Create(ctx, playerReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create player: %w", err)
	}

	// Create player wallets after successful player creation
	err = s.walletClient.CreatePlayerWallet(ctx, createdPlayer.ID, createdPlayer.PhoneNumber)
	if err != nil {
		fmt.Printf("Failed to create player wallets for player %s: %v\n", createdPlayer.ID.String(), err)
	}

	if err := s.authRepo.RecordLoginAttempt(ctx, normalizedPhone, &createdPlayer.ID, "", req.RegistrationChannel, "", "registration", true, nil); err != nil {
		fmt.Printf("Failed to record login attempt for player %s: %v\n", createdPlayer.ID.String(), err)
	}

	return createdPlayer, nil
}

// VerifyOTP verifies OTP for phone number
func (s *registrationService) VerifyOTP(ctx context.Context, phoneNumber, otp string) error {
	err := s.otpService.VerifyOTP(ctx, phoneNumber, otp, "registration")
	if err != nil {
		return fmt.Errorf("failed to verify OTP: %w", err)
	}

	player, err := s.playerRepo.GetByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return fmt.Errorf("player not found")
	}

	updateReq := models.UpdatePlayerRequest{
		ID:            player.ID,
		PhoneVerified: true,
	}

	_, err = s.playerRepo.Update(ctx, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update player verification status: %w", err)
	}

	return nil
}

// USSDRegister registers a player via USSD (no OTP required)
func (s *registrationService) USSDRegister(ctx context.Context, req models.USSDRegisterRequest) (*models.Player, error) {
	normalizedPhone := validation.NormalizePhone(req.PhoneNumber)

	existingPlayer, err := s.playerRepo.GetByPhoneNumber(ctx, normalizedPhone)
	if err == nil && existingPlayer != nil {
		return nil, fmt.Errorf("phone number already registered")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	playerReq := models.CreatePlayerRequest{
		PhoneNumber:         normalizedPhone,
		PasswordHash:        string(hashedPassword),
		RegistrationChannel: "ussd",
		TermsAccepted:       true, // Assume terms accepted for USSD
		MarketingConsent:    false,
	}

	createdPlayer, err := s.playerRepo.Create(ctx, playerReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create player: %w", err)
	}

	err = s.walletClient.CreatePlayerWallet(ctx, createdPlayer.ID, createdPlayer.PhoneNumber)
	if err != nil {
		fmt.Printf("Failed to create player wallets for player %s: %v\n", createdPlayer.ID.String(), err)
	}

	if err := s.authRepo.RecordLoginAttempt(ctx, normalizedPhone, &createdPlayer.ID, "", "ussd", "", "registration", true, nil); err != nil {
		fmt.Printf("Failed to record login attempt for player %s: %v\n", createdPlayer.ID.String(), err)
	}

	return createdPlayer, nil
}

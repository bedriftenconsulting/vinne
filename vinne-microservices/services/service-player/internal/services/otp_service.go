package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	notificationv1 "github.com/randco/randco-microservices/proto/notification/v1"
	"github.com/randco/service-player/internal/clients"
	"github.com/randco/service-player/internal/models"
	"github.com/randco/service-player/internal/repositories"
)

type otpService struct {
	otpRepo            repositories.OTPRepository
	playerRepo         repositories.PlayerRepository
	notificationClient *clients.NotificationClient
}

func NewOTPService(otpRepo repositories.OTPRepository, notificationClient *clients.NotificationClient, playerRepo repositories.PlayerRepository) OTPService {
	return &otpService{
		otpRepo:            otpRepo,
		notificationClient: notificationClient,
		playerRepo:         playerRepo,
	}
}

func (s *otpService) GenerateAndSendOTP(ctx context.Context, phoneNumber, purpose string) error {
	code, err := s.generateOTPCode(6)
	if err != nil {
		return fmt.Errorf("failed to generate OTP code: %w", err)
	}

	err = s.otpRepo.InvalidatePrevious(ctx, phoneNumber, purpose)
	if err != nil {
		return fmt.Errorf("failed to invalidate previous OTPs: %w", err)
	}

	createReq := models.CreateOTPRequest{
		PhoneNumber: phoneNumber,
		Purpose:     purpose,
		ExpiresIn:   10 * time.Minute,
	}

	otp, err := s.otpRepo.Create(ctx, createReq, code)
	if err != nil {
		return fmt.Errorf("failed to create OTP record: %w", err)
	}

	err = s.sendOTPSMS(ctx, phoneNumber, code, purpose, otp.ID.String())
	if err != nil {
		return fmt.Errorf("failed to send OTP SMS: %w", err)
	}

	return nil
}

func (s *otpService) VerifyOTP(ctx context.Context, sessionID, code, purpose string) error {
	otp, err := s.otpRepo.GetByCode(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to get OTP: %w", err)
	}

	if otp == nil {
		return fmt.Errorf("invalid OTP code")
	}

	if otp.Code != code {
		return fmt.Errorf("invalid OTP code")
	}

	if otp.Purpose != purpose {
		return fmt.Errorf("invalid OTP purpose")
	}

	if time.Now().After(otp.ExpiresAt) {
		return fmt.Errorf("OTP code has expired")
	}

	err = s.otpRepo.MarkAsUsed(ctx, otp.ID)
	if err != nil {
		return fmt.Errorf("failed to mark OTP as used: %w", err)
	}
	err = s.playerRepo.VerifyPhoneNumber(ctx, otp.PhoneNumber)
	if err != nil {
		return fmt.Errorf("failed to verify phone number: %w", err)
	}

	return nil
}

func (s *otpService) CleanupExpiredOTPs(ctx context.Context) error {
	return s.otpRepo.DeleteExpired(ctx)
}

func (s *otpService) generateOTPCode(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("invalid OTP length")
	}

	max := new(big.Int)
	max.Exp(big.NewInt(10), big.NewInt(int64(length)), nil)

	num, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("failed to generate random number: %w", err)
	}

	code := fmt.Sprintf("%0*d", length, num.Int64())
	return code, nil
}

func (s *otpService) sendOTPSMS(ctx context.Context, phoneNumber, code, purpose string, otpID string) error {
	var message string
	switch purpose {
	case "registration":
		message = fmt.Sprintf("Your registration code is: %s. This code expires in 10 minutes. Do not share this code with anyone.", code)
	case "login":
		message = fmt.Sprintf("Your login code is: %s. This code expires in 10 minutes. Do not share this code with anyone.", code)
	case "password_reset":
		message = fmt.Sprintf("Your password reset code is: %s. This code expires in 10 minutes. Do not share this code with anyone.", code)
	default:
		message = fmt.Sprintf("Your verification code is: %s. This code expires in 10 minutes. Do not share this code with anyone.", code)
	}

	req := &notificationv1.SendSMSRequest{
		To:             phoneNumber,
		Content:        message,
		IdempotencyKey: otpID,
	}

	_, err := s.notificationClient.SendSMS(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send OTP SMS: %w", err)
	}

	return nil
}

func (s *otpService) ResendRegistrationOTP(ctx context.Context, phoneNumber string) error {
	player, err := s.playerRepo.GetByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return fmt.Errorf("failed to get player: %w", err)
	}
	if player == nil {
		return fmt.Errorf("player not found")
	}

	if player.PhoneVerified {
		return fmt.Errorf("player already has a verified phone number")
	}

	otp, err := s.otpRepo.GetByPhoneAndPurpose(ctx, phoneNumber, "registration")
	if err != nil {
		return fmt.Errorf("failed to get existing OTP: %w", err)
	}
	if otp != nil {
		if time.Now().Before(otp.ExpiresAt) {
			return fmt.Errorf("OTP already exists and is not expired")
		}
	}

	err = s.otpRepo.InvalidatePrevious(ctx, phoneNumber, "registration")
	if err != nil {
		return fmt.Errorf("failed to invalidate previous OTPs: %w", err)
	}

	err = s.GenerateAndSendOTP(ctx, phoneNumber, "registration")
	if err != nil {
		return fmt.Errorf("failed to generate and send OTP: %w", err)
	}

	return nil
}

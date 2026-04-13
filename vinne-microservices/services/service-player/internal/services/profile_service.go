package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/shared/validation"
	"github.com/randco/service-player/internal/models"
	"github.com/randco/service-player/internal/repositories"
	"golang.org/x/crypto/bcrypt"
)

type profileService struct {
	playerRepo repositories.PlayerRepository
	authRepo   repositories.PlayerAuthRepository
}

func NewProfileService(
	playerRepo repositories.PlayerRepository,
	authRepo repositories.PlayerAuthRepository,
) ProfileService {
	return &profileService{
		playerRepo: playerRepo,
		authRepo:   authRepo,
	}
}

func (s *profileService) GetProfile(ctx context.Context, playerID uuid.UUID) (*models.Player, error) {
	player, err := s.playerRepo.GetByID(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("player not found: %w", err)
	}

	if player == nil {
		return nil, fmt.Errorf("player not found")
	}

	player.PasswordHash = ""

	return player, nil
}

func (s *profileService) UpdateProfile(ctx context.Context, req models.UpdatePlayerRequest) (*models.Player, error) {
	// Get existing player to verify it exists
	player, err := s.playerRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("player not found: %w", err)
	}
	if player == nil {
		return nil, fmt.Errorf("player not found")
	}

	// Update fields if provided
	updateReq := models.UpdatePlayerRequest{
		ID: req.ID,
	}

	// Only update fields that are not empty/zero values
	if req.FirstName != "" {
		updateReq.FirstName = req.FirstName
	}
	if req.LastName != "" {
		updateReq.LastName = req.LastName
	}
	if req.Email != "" {
		// Check if email is already taken by another player
		existingPlayerWithEmail, err := s.playerRepo.GetByEmail(ctx, req.Email)
		if err == nil && existingPlayerWithEmail != nil && existingPlayerWithEmail.ID != req.ID {
			return nil, fmt.Errorf("email already taken")
		}
		updateReq.Email = req.Email
	}
	if !req.DateOfBirth.IsZero() {
		updateReq.DateOfBirth = req.DateOfBirth
	}
	if req.NationalID != "" {
		updateReq.NationalID = req.NationalID
	}

	updatedPlayer, err := s.playerRepo.Update(ctx, updateReq)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	// Don't return password hash
	updatedPlayer.PasswordHash = ""

	return updatedPlayer, nil
}

// ChangePassword changes player password
func (s *profileService) ChangePassword(ctx context.Context, playerID uuid.UUID, currentPassword, newPassword string) error {
	// Get player to verify current password
	player, err := s.playerRepo.GetByID(ctx, playerID)
	if err != nil {
		return fmt.Errorf("player not found: %w", err)
	}

	// Verify current password
	err = bcrypt.CompareHashAndPassword([]byte(player.PasswordHash), []byte(currentPassword))
	if err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	// Validate new password
	if len(newPassword) < 6 {
		return fmt.Errorf("new password must be at least 6 characters")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password using auth repository
	err = s.authRepo.UpdatePassword(ctx, playerID, string(hashedPassword))
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// UpdateMobileMoneyPhone updates player's mobile money phone number
func (s *profileService) UpdateMobileMoneyPhone(ctx context.Context, playerID uuid.UUID, phoneNumber, otp string) error {
	_, err := s.playerRepo.GetByID(ctx, playerID)
	if err != nil {
		return fmt.Errorf("player not found: %w", err)
	}

	normalizedPhone := validation.NormalizePhone(phoneNumber)

	updateReq := models.UpdatePlayerRequest{
		ID:               playerID,
		MobileMoneyPhone: normalizedPhone,
	}

	_, err = s.playerRepo.Update(ctx, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update mobile money phone: %w", err)
	}

	return nil
}

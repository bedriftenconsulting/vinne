package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/randco/service-player/internal/models"
	"github.com/randco/service-player/internal/repositories"
)

type adminService struct {
	playerRepo repositories.PlayerRepository
}

func NewAdminService(playerRepo repositories.PlayerRepository) AdminService {
	return &adminService{
		playerRepo: playerRepo,
	}
}

// SearchPlayers searches for players by query string
func (s *adminService) SearchPlayers(ctx context.Context, query string, page, perPage int) ([]*models.Player, int64, error) {
	// Validate input
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20 // Default page size
	}

	offset := (page - 1) * perPage

	// Search players
	players, err := s.playerRepo.Search(ctx, query, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search players: %w", err)
	}

	// Get total count for pagination
	// For simplicity, we'll use the same search query to count
	// In a real implementation, you might want a separate count method
	totalCount := int64(len(players))
	if len(players) == perPage {
		// If we got a full page, there might be more results
		// For now, we'll estimate the total count
		totalCount = int64(len(players) + offset)
	}

	return players, totalCount, nil
}

// SuspendPlayer suspends a player
func (s *adminService) SuspendPlayer(ctx context.Context, playerID uuid.UUID, reason string) error {
	// Validate input
	if reason == "" {
		return fmt.Errorf("suspension reason is required")
	}

	// Check if player exists
	player, err := s.playerRepo.GetByID(ctx, playerID)
	if err != nil {
		return fmt.Errorf("failed to get player: %w", err)
	}
	if player == nil {
		return fmt.Errorf("player not found")
	}

	// Check if player is already suspended
	if player.Status == models.PlayerStatusSuspended {
		return fmt.Errorf("player is already suspended")
	}

	// Suspend player
	err = s.playerRepo.Suspend(ctx, playerID, reason)
	if err != nil {
		return fmt.Errorf("failed to suspend player: %w", err)
	}

	return nil
}

// ActivatePlayer activates a player
func (s *adminService) ActivatePlayer(ctx context.Context, playerID uuid.UUID) error {
	// Check if player exists
	player, err := s.playerRepo.GetByID(ctx, playerID)
	if err != nil {
		return fmt.Errorf("failed to get player: %w", err)
	}
	if player == nil {
		return fmt.Errorf("player not found")
	}

	// Check if player is already active
	if player.Status == models.PlayerStatusActive {
		return fmt.Errorf("player is already active")
	}

	// Activate player
	err = s.playerRepo.Activate(ctx, playerID)
	if err != nil {
		return fmt.Errorf("failed to activate player: %w", err)
	}

	return nil
}

package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"github.com/randco/randco-microservices/services/service-game/internal/repositories"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// PrizeStructureService defines the interface for prize structure business logic
type PrizeStructureService interface {
	CreatePrizeStructure(ctx context.Context, req models.CreatePrizeStructureRequest) (*models.PrizeStructure, error)
	GetPrizeStructure(ctx context.Context, gameID uuid.UUID) (*models.PrizeStructure, error)
	UpdatePrizeStructure(ctx context.Context, structure *models.PrizeStructure) error
	ValidatePrizeStructure(ctx context.Context, structure *models.PrizeStructure) error
}

// prizeStructureService implements PrizeStructureService interface
type prizeStructureService struct {
	prizeRepo repositories.PrizeStructureRepository
	gameRepo  repositories.GameRepository
}

// NewPrizeStructureService creates a new instance of PrizeStructureService
func NewPrizeStructureService(
	prizeRepo repositories.PrizeStructureRepository,
	gameRepo repositories.GameRepository,
) PrizeStructureService {
	return &prizeStructureService{
		prizeRepo: prizeRepo,
		gameRepo:  gameRepo,
	}
}

// CreatePrizeStructure creates a new prize structure for a game
func (s *prizeStructureService) CreatePrizeStructure(ctx context.Context, req models.CreatePrizeStructureRequest) (*models.PrizeStructure, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.prize_structure.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("game.id", req.GameID.String()),
		attribute.Int64("total_prize_pool", req.TotalPrizePool),
		attribute.Float64("house_edge", req.HouseEdgePercentage),
		attribute.Int("tiers_count", len(req.Tiers)),
	)

	// Verify game exists
	game, err := s.gameRepo.GetByID(ctx, req.GameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return nil, fmt.Errorf("game not found: %w", err)
	}

	// Validate game can be modified
	if !game.CanBeModified() {
		err := fmt.Errorf("game cannot be modified in status: %s", game.Status)
		span.RecordError(err)
		span.SetStatus(codes.Error, "game cannot be modified")
		return nil, err
	}

	// Create prize structure object
	structure := &models.PrizeStructure{
		GameID:              req.GameID,
		TotalPrizePool:      req.TotalPrizePool,
		HouseEdgePercentage: req.HouseEdgePercentage,
		Tiers:               req.Tiers,
	}

	// Validate prize structure
	if err := s.ValidatePrizeStructure(ctx, structure); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid prize structure")
		return nil, err
	}

	// Create prize structure in repository
	if err := s.prizeRepo.Create(ctx, structure); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create prize structure")
		return nil, fmt.Errorf("failed to create prize structure: %w", err)
	}

	span.SetAttributes(attribute.String("structure.id", structure.ID.String()))
	return structure, nil
}

// GetPrizeStructure retrieves the prize structure for a game
func (s *prizeStructureService) GetPrizeStructure(ctx context.Context, gameID uuid.UUID) (*models.PrizeStructure, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.prize_structure.get")
	defer span.End()

	span.SetAttributes(attribute.String("game.id", gameID.String()))

	// Get active prize structure for the game
	structure, err := s.prizeRepo.GetActivePrizeStructure(ctx, gameID)
	if err != nil {
		// If no active structure, try to get the latest structure
		structure, err = s.prizeRepo.GetByGameID(ctx, gameID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "prize structure not found")
			return nil, fmt.Errorf("prize structure not found: %w", err)
		}
	}

	span.SetAttributes(
		attribute.String("structure.id", structure.ID.String()),
		attribute.Int("tiers_count", len(structure.Tiers)),
	)
	return structure, nil
}

// UpdatePrizeStructure updates an existing prize structure
func (s *prizeStructureService) UpdatePrizeStructure(ctx context.Context, structure *models.PrizeStructure) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.prize_structure.update")
	defer span.End()

	span.SetAttributes(
		attribute.String("structure.id", structure.ID.String()),
		attribute.String("game.id", structure.GameID.String()),
	)

	// Verify game exists and can be modified
	game, err := s.gameRepo.GetByID(ctx, structure.GameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return fmt.Errorf("game not found: %w", err)
	}

	if !game.CanBeModified() {
		err := fmt.Errorf("game cannot be modified in status: %s", game.Status)
		span.RecordError(err)
		span.SetStatus(codes.Error, "game cannot be modified")
		return err
	}

	// Validate prize structure
	if err := s.ValidatePrizeStructure(ctx, structure); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid prize structure")
		return err
	}

	// Update prize structure
	if err := s.prizeRepo.Update(ctx, structure); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update prize structure")
		return fmt.Errorf("failed to update prize structure: %w", err)
	}

	// Update tiers if provided
	if len(structure.Tiers) > 0 {
		for _, tier := range structure.Tiers {
			tier.PrizeStructureID = structure.ID
			if tier.ID == uuid.Nil {
				if err := s.prizeRepo.CreatePrizeTier(ctx, &tier); err != nil {
					return fmt.Errorf("failed to create prize tier: %w", err)
				}
			}
		}
	}

	return nil
}

// ValidatePrizeStructure validates prize structure business logic
func (s *prizeStructureService) ValidatePrizeStructure(ctx context.Context, structure *models.PrizeStructure) error {
	_, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.prize_structure.validate")
	defer span.End()

	// Validate house edge percentage
	if structure.HouseEdgePercentage < 0 || structure.HouseEdgePercentage > 100 {
		return fmt.Errorf("house edge percentage must be between 0 and 100, got: %.2f",
			structure.HouseEdgePercentage)
	}

	// Validate total prize pool
	if structure.TotalPrizePool < 0 {
		return fmt.Errorf("total prize pool cannot be negative")
	}

	// Validate tiers
	if len(structure.Tiers) == 0 {
		return fmt.Errorf("at least one prize tier is required")
	}

	// Validate tier percentages sum
	var totalPercentage float64
	for _, tier := range structure.Tiers {
		if tier.PrizePercentage < 0 || tier.PrizePercentage > 100 {
			return fmt.Errorf("prize percentage for tier %d must be between 0 and 100", tier.TierNumber)
		}
		totalPercentage += tier.PrizePercentage

		// Validate tier number
		if tier.TierNumber < 1 {
			return fmt.Errorf("tier number must be positive, got: %d", tier.TierNumber)
		}

		// Validate matches required
		if tier.MatchesRequired < 0 {
			return fmt.Errorf("matches required cannot be negative for tier %d", tier.TierNumber)
		}
	}

	// Allow some rounding tolerance
	maxPercentage := 100.0 - structure.HouseEdgePercentage
	if totalPercentage > maxPercentage+0.01 {
		return fmt.Errorf("total prize percentages (%.2f%%) exceed available pool after house edge (%.2f%%)",
			totalPercentage, maxPercentage)
	}

	// Validate tier uniqueness
	tierNumbers := make(map[int32]bool)
	for _, tier := range structure.Tiers {
		if tierNumbers[tier.TierNumber] {
			return fmt.Errorf("duplicate tier number: %d", tier.TierNumber)
		}
		tierNumbers[tier.TierNumber] = true
	}

	span.SetStatus(codes.Ok, "prize structure validated successfully")
	return nil
}

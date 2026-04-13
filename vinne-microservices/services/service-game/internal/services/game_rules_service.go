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

// GameRulesService defines the interface for game rules business logic
type GameRulesService interface {
	CreateGameRules(ctx context.Context, req models.CreateGameRulesRequest) (*models.GameRules, error)
	GetGameRules(ctx context.Context, gameID uuid.UUID) (*models.GameRules, error)
	UpdateGameRules(ctx context.Context, rules *models.GameRules) error
	ValidateGameRules(ctx context.Context, rules *models.GameRules) error
}

// gameRulesService implements GameRulesService interface
type gameRulesService struct {
	rulesRepo repositories.GameRulesRepository
	gameRepo  repositories.GameRepository
}

// NewGameRulesService creates a new instance of GameRulesService
func NewGameRulesService(
	rulesRepo repositories.GameRulesRepository,
	gameRepo repositories.GameRepository,
) GameRulesService {
	return &gameRulesService{
		rulesRepo: rulesRepo,
		gameRepo:  gameRepo,
	}
}

// CreateGameRules creates new rules for a game
func (s *gameRulesService) CreateGameRules(ctx context.Context, req models.CreateGameRulesRequest) (*models.GameRules, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_rules.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("game.id", req.GameID.String()),
		attribute.Int("numbers_to_pick", int(req.NumbersToPick)),
		attribute.Int("total_numbers", int(req.TotalNumbers)),
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

	// Create rules object
	rules := &models.GameRules{
		GameID:         req.GameID,
		NumbersToPick:  req.NumbersToPick,
		TotalNumbers:   req.TotalNumbers,
		MinSelections:  req.MinSelections,
		MaxSelections:  req.MaxSelections,
		AllowQuickPick: req.AllowQuickPick,
		SpecialRules:   req.SpecialRules,
	}

	// Validate rules
	if err := s.ValidateGameRules(ctx, rules); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid game rules")
		return nil, err
	}

	// Create rules in repository
	if err := s.rulesRepo.Create(ctx, rules); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create game rules")
		return nil, fmt.Errorf("failed to create game rules: %w", err)
	}

	span.SetAttributes(attribute.String("rules.id", rules.ID.String()))
	return rules, nil
}

// GetGameRules retrieves rules for a game
func (s *gameRulesService) GetGameRules(ctx context.Context, gameID uuid.UUID) (*models.GameRules, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_rules.get")
	defer span.End()

	span.SetAttributes(attribute.String("game.id", gameID.String()))

	// Get active rules for the game
	rules, err := s.rulesRepo.GetActiveRules(ctx, gameID)
	if err != nil {
		// If no active rules, try to get the latest rules
		rules, err = s.rulesRepo.GetByGameID(ctx, gameID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "game rules not found")
			return nil, fmt.Errorf("game rules not found: %w", err)
		}
	}

	span.SetAttributes(
		attribute.String("rules.id", rules.ID.String()),
		attribute.Bool("rules.allow_quick_pick", rules.AllowQuickPick),
	)
	return rules, nil
}

// UpdateGameRules updates existing game rules
func (s *gameRulesService) UpdateGameRules(ctx context.Context, rules *models.GameRules) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_rules.update")
	defer span.End()

	span.SetAttributes(
		attribute.String("rules.id", rules.ID.String()),
		attribute.String("game.id", rules.GameID.String()),
	)

	// Verify game exists and can be modified
	game, err := s.gameRepo.GetByID(ctx, rules.GameID)
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

	// Validate rules
	if err := s.ValidateGameRules(ctx, rules); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid game rules")
		return err
	}

	// Update rules
	if err := s.rulesRepo.Update(ctx, rules); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update game rules")
		return fmt.Errorf("failed to update game rules: %w", err)
	}

	return nil
}

// ValidateGameRules validates game rules business logic
func (s *gameRulesService) ValidateGameRules(ctx context.Context, rules *models.GameRules) error {
	_, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_rules.validate")
	defer span.End()

	// Validate numbers to pick vs total numbers
	if rules.NumbersToPick > rules.TotalNumbers {
		return fmt.Errorf("numbers to pick (%d) cannot exceed total numbers (%d)",
			rules.NumbersToPick, rules.TotalNumbers)
	}

	// Validate min/max selections
	if rules.MinSelections < 1 {
		return fmt.Errorf("minimum selections must be at least 1")
	}

	if rules.MaxSelections < rules.MinSelections {
		return fmt.Errorf("maximum selections (%d) cannot be less than minimum selections (%d)",
			rules.MaxSelections, rules.MinSelections)
	}

	// Validate for specific game types
	if rules.NumbersToPick == 5 && rules.TotalNumbers == 90 {
		// 5/90 game validation
		if rules.MinSelections > 10 {
			return fmt.Errorf("5/90 games typically allow maximum 10 selections")
		}
	} else if rules.NumbersToPick == 6 && rules.TotalNumbers == 49 {
		// 6/49 game validation
		if rules.MinSelections > 10 {
			return fmt.Errorf("6/49 games typically allow maximum 10 selections")
		}
	}

	span.SetStatus(codes.Ok, "game rules validated successfully")
	return nil
}

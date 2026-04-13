package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// GameRulesRepository defines the interface for game rules data operations
type GameRulesRepository interface {
	Create(ctx context.Context, rules *models.GameRules) error
	GetByGameID(ctx context.Context, gameID uuid.UUID) (*models.GameRules, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.GameRules, error)
	Update(ctx context.Context, rules *models.GameRules) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetActiveRules(ctx context.Context, gameID uuid.UUID) (*models.GameRules, error)
}

// gameRulesRepository implements GameRulesRepository interface
type gameRulesRepository struct {
	db *sql.DB
}

// NewGameRulesRepository creates a new instance of GameRulesRepository
func NewGameRulesRepository(db *sql.DB) GameRulesRepository {
	return &gameRulesRepository{
		db: db,
	}
}

// Create creates new game rules
func (r *gameRulesRepository) Create(ctx context.Context, rules *models.GameRules) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_rules.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.table", "game_rules"),
		attribute.String("game.id", rules.GameID.String()),
	)

	query := `
		INSERT INTO game_rules (
			id, game_id, numbers_to_pick, total_numbers, 
			min_selections, max_selections, number_range_min, 
			number_range_max, selection_count, allow_quick_pick, 
			special_rules, effective_from, effective_to
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING created_at, updated_at`

	rules.ID = uuid.New()

	err := r.db.QueryRowContext(ctx, query,
		rules.ID, rules.GameID, rules.NumbersToPick, rules.TotalNumbers,
		rules.MinSelections, rules.MaxSelections, rules.NumberRangeMin,
		rules.NumberRangeMax, rules.SelectionCount, rules.AllowQuickPick,
		rules.SpecialRules, rules.EffectiveFrom, rules.EffectiveTo,
	).Scan(&rules.CreatedAt, &rules.UpdatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create game rules")
		return fmt.Errorf("failed to create game rules: %w", err)
	}

	span.SetAttributes(attribute.String("game_rules.id", rules.ID.String()))
	return nil
}

// GetByGameID retrieves game rules by game ID
func (r *gameRulesRepository) GetByGameID(ctx context.Context, gameID uuid.UUID) (*models.GameRules, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_rules.get_by_game_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_rules"),
		attribute.String("game.id", gameID.String()),
	)

	query := `
		SELECT id, game_id, numbers_to_pick, total_numbers,
			min_selections, max_selections, number_range_min,
			number_range_max, selection_count, allow_quick_pick,
			special_rules, effective_from, effective_to,
			created_at, updated_at
		FROM game_rules
		WHERE game_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	rules := &models.GameRules{}
	err := r.db.QueryRowContext(ctx, query, gameID).Scan(
		&rules.ID, &rules.GameID, &rules.NumbersToPick, &rules.TotalNumbers,
		&rules.MinSelections, &rules.MaxSelections, &rules.NumberRangeMin,
		&rules.NumberRangeMax, &rules.SelectionCount, &rules.AllowQuickPick,
		&rules.SpecialRules, &rules.EffectiveFrom, &rules.EffectiveTo,
		&rules.CreatedAt, &rules.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.Bool("rules.found", false))
			return nil, fmt.Errorf("game rules not found")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get game rules")
		return nil, fmt.Errorf("failed to get game rules: %w", err)
	}

	span.SetAttributes(attribute.Bool("rules.found", true))
	return rules, nil
}

// GetByID retrieves game rules by ID
func (r *gameRulesRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.GameRules, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_rules.get_by_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_rules"),
		attribute.String("rules.id", id.String()),
	)

	query := `
		SELECT id, game_id, numbers_to_pick, total_numbers,
			min_selections, max_selections, number_range_min,
			number_range_max, selection_count, allow_quick_pick,
			special_rules, effective_from, effective_to,
			created_at, updated_at
		FROM game_rules
		WHERE id = $1`

	rules := &models.GameRules{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&rules.ID, &rules.GameID, &rules.NumbersToPick, &rules.TotalNumbers,
		&rules.MinSelections, &rules.MaxSelections, &rules.NumberRangeMin,
		&rules.NumberRangeMax, &rules.SelectionCount, &rules.AllowQuickPick,
		&rules.SpecialRules, &rules.EffectiveFrom, &rules.EffectiveTo,
		&rules.CreatedAt, &rules.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("game rules not found")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get game rules")
		return nil, fmt.Errorf("failed to get game rules: %w", err)
	}

	return rules, nil
}

// Update updates existing game rules
func (r *gameRulesRepository) Update(ctx context.Context, rules *models.GameRules) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_rules.update")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "game_rules"),
		attribute.String("rules.id", rules.ID.String()),
	)

	query := `
		UPDATE game_rules SET
			numbers_to_pick = $2, total_numbers = $3,
			min_selections = $4, max_selections = $5,
			number_range_min = $6, number_range_max = $7,
			selection_count = $8, allow_quick_pick = $9,
			special_rules = $10, effective_from = $11,
			effective_to = $12, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	err := r.db.QueryRowContext(ctx, query,
		rules.ID, rules.NumbersToPick, rules.TotalNumbers,
		rules.MinSelections, rules.MaxSelections, rules.NumberRangeMin,
		rules.NumberRangeMax, rules.SelectionCount, rules.AllowQuickPick,
		rules.SpecialRules, rules.EffectiveFrom, rules.EffectiveTo,
	).Scan(&rules.UpdatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update game rules")
		return fmt.Errorf("failed to update game rules: %w", err)
	}

	return nil
}

// Delete deletes game rules
func (r *gameRulesRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_rules.delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "DELETE"),
		attribute.String("db.table", "game_rules"),
		attribute.String("rules.id", id.String()),
	)

	query := `DELETE FROM game_rules WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete game rules")
		return fmt.Errorf("failed to delete game rules: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("game rules not found")
	}

	return nil
}

// GetActiveRules retrieves currently active rules for a game
func (r *gameRulesRepository) GetActiveRules(ctx context.Context, gameID uuid.UUID) (*models.GameRules, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_rules.get_active")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_rules"),
		attribute.String("game.id", gameID.String()),
	)

	query := `
		SELECT id, game_id, numbers_to_pick, total_numbers,
			min_selections, max_selections, number_range_min,
			number_range_max, selection_count, allow_quick_pick,
			special_rules, effective_from, effective_to,
			created_at, updated_at
		FROM game_rules
		WHERE game_id = $1
			AND effective_from <= NOW()
			AND (effective_to IS NULL OR effective_to > NOW())
		ORDER BY effective_from DESC
		LIMIT 1`

	rules := &models.GameRules{}
	err := r.db.QueryRowContext(ctx, query, gameID).Scan(
		&rules.ID, &rules.GameID, &rules.NumbersToPick, &rules.TotalNumbers,
		&rules.MinSelections, &rules.MaxSelections, &rules.NumberRangeMin,
		&rules.NumberRangeMax, &rules.SelectionCount, &rules.AllowQuickPick,
		&rules.SpecialRules, &rules.EffectiveFrom, &rules.EffectiveTo,
		&rules.CreatedAt, &rules.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active game rules found")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get active game rules")
		return nil, fmt.Errorf("failed to get active game rules: %w", err)
	}

	return rules, nil
}

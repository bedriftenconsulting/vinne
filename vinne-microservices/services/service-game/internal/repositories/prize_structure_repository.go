package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// PrizeStructureRepository defines the interface for prize structure data operations
type PrizeStructureRepository interface {
	Create(ctx context.Context, structure *models.PrizeStructure) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.PrizeStructure, error)
	GetByGameID(ctx context.Context, gameID uuid.UUID) (*models.PrizeStructure, error)
	Update(ctx context.Context, structure *models.PrizeStructure) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetActivePrizeStructure(ctx context.Context, gameID uuid.UUID) (*models.PrizeStructure, error)
	CreatePrizeTier(ctx context.Context, tier *models.PrizeTier) error
	GetPrizeTiers(ctx context.Context, structureID uuid.UUID) ([]*models.PrizeTier, error)
}

// prizeStructureRepository implements PrizeStructureRepository interface
type prizeStructureRepository struct {
	db *sql.DB
}

// NewPrizeStructureRepository creates a new instance of PrizeStructureRepository
func NewPrizeStructureRepository(db *sql.DB) PrizeStructureRepository {
	return &prizeStructureRepository{
		db: db,
	}
}

// Create creates a new prize structure
func (r *prizeStructureRepository) Create(ctx context.Context, structure *models.PrizeStructure) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.prize_structure.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.table", "prize_structures"),
		attribute.String("game.id", structure.GameID.String()),
	)

	// Start transaction for prize structure and tiers
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			// Log the error but don't fail the operation
			log.Printf("[WARN] Failed to rollback transaction in CreateWithTiers: %v", err)
		}
	}()

	// Insert prize structure
	query := `
		INSERT INTO prize_structures (
			id, game_id, total_prize_pool, house_edge_percentage,
			effective_from, effective_to
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at`

	structure.ID = uuid.New()

	err = tx.QueryRowContext(ctx, query,
		structure.ID, structure.GameID, structure.TotalPrizePool,
		structure.HouseEdgePercentage, structure.EffectiveFrom, structure.EffectiveTo,
	).Scan(&structure.CreatedAt, &structure.UpdatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create prize structure")
		return fmt.Errorf("failed to create prize structure: %w", err)
	}

	// Insert prize tiers if provided
	if len(structure.Tiers) > 0 {
		for i := range structure.Tiers {
			tier := &structure.Tiers[i]
			tier.PrizeStructureID = structure.ID
			if err := r.createPrizeTierTx(ctx, tx, tier); err != nil {
				return fmt.Errorf("failed to create prize tier %d: %w", tier.TierNumber, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	span.SetAttributes(attribute.String("prize_structure.id", structure.ID.String()))
	return nil
}

// createPrizeTierTx creates a prize tier within a transaction
func (r *prizeStructureRepository) createPrizeTierTx(ctx context.Context, tx *sql.Tx, tier *models.PrizeTier) error {
	query := `
		INSERT INTO prize_tiers (
			id, prize_structure_id, tier_number, name,
			matches_required, prize_amount, prize_percentage,
			estimated_winners, description
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at`

	tier.ID = uuid.New()

	return tx.QueryRowContext(ctx, query,
		tier.ID, tier.PrizeStructureID, tier.TierNumber, tier.Name,
		tier.MatchesRequired, tier.PrizeAmount, tier.PrizePercentage,
		tier.EstimatedWinners, tier.Description,
	).Scan(&tier.CreatedAt, &tier.UpdatedAt)
}

// GetByID retrieves a prize structure by ID
func (r *prizeStructureRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PrizeStructure, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.prize_structure.get_by_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "prize_structures"),
		attribute.String("structure.id", id.String()),
	)

	query := `
		SELECT id, game_id, total_prize_pool, house_edge_percentage,
			effective_from, effective_to, created_at, updated_at
		FROM prize_structures
		WHERE id = $1`

	structure := &models.PrizeStructure{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&structure.ID, &structure.GameID, &structure.TotalPrizePool,
		&structure.HouseEdgePercentage, &structure.EffectiveFrom, &structure.EffectiveTo,
		&structure.CreatedAt, &structure.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("prize structure not found")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get prize structure")
		return nil, fmt.Errorf("failed to get prize structure: %w", err)
	}

	// Load prize tiers
	tiers, err := r.GetPrizeTiers(ctx, structure.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get prize tiers: %w", err)
	}

	if tiers != nil {
		structure.Tiers = make([]models.PrizeTier, len(tiers))
		for i, tier := range tiers {
			structure.Tiers[i] = *tier
		}
	}

	return structure, nil
}

// GetByGameID retrieves the latest prize structure for a game
func (r *prizeStructureRepository) GetByGameID(ctx context.Context, gameID uuid.UUID) (*models.PrizeStructure, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.prize_structure.get_by_game_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "prize_structures"),
		attribute.String("game.id", gameID.String()),
	)

	query := `
		SELECT id, game_id, total_prize_pool, house_edge_percentage,
			effective_from, effective_to, created_at, updated_at
		FROM prize_structures
		WHERE game_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	structure := &models.PrizeStructure{}
	err := r.db.QueryRowContext(ctx, query, gameID).Scan(
		&structure.ID, &structure.GameID, &structure.TotalPrizePool,
		&structure.HouseEdgePercentage, &structure.EffectiveFrom, &structure.EffectiveTo,
		&structure.CreatedAt, &structure.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.Bool("structure.found", false))
			return nil, fmt.Errorf("prize structure not found for game")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get prize structure")
		return nil, fmt.Errorf("failed to get prize structure: %w", err)
	}

	// Load prize tiers
	tiers, err := r.GetPrizeTiers(ctx, structure.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get prize tiers: %w", err)
	}

	if tiers != nil {
		structure.Tiers = make([]models.PrizeTier, len(tiers))
		for i, tier := range tiers {
			structure.Tiers[i] = *tier
		}
	}

	span.SetAttributes(attribute.Bool("structure.found", true))
	return structure, nil
}

// Update updates an existing prize structure
func (r *prizeStructureRepository) Update(ctx context.Context, structure *models.PrizeStructure) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.prize_structure.update")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "prize_structures"),
		attribute.String("structure.id", structure.ID.String()),
	)

	query := `
		UPDATE prize_structures SET
			total_prize_pool = $2, house_edge_percentage = $3,
			effective_from = $4, effective_to = $5,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	err := r.db.QueryRowContext(ctx, query,
		structure.ID, structure.TotalPrizePool, structure.HouseEdgePercentage,
		structure.EffectiveFrom, structure.EffectiveTo,
	).Scan(&structure.UpdatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update prize structure")
		return fmt.Errorf("failed to update prize structure: %w", err)
	}

	return nil
}

// Delete deletes a prize structure
func (r *prizeStructureRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.prize_structure.delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "DELETE"),
		attribute.String("db.table", "prize_structures"),
		attribute.String("structure.id", id.String()),
	)

	query := `DELETE FROM prize_structures WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete prize structure")
		return fmt.Errorf("failed to delete prize structure: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("prize structure not found")
	}

	return nil
}

// GetActivePrizeStructure retrieves the currently active prize structure for a game
func (r *prizeStructureRepository) GetActivePrizeStructure(ctx context.Context, gameID uuid.UUID) (*models.PrizeStructure, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.prize_structure.get_active")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "prize_structures"),
		attribute.String("game.id", gameID.String()),
	)

	query := `
		SELECT id, game_id, total_prize_pool, house_edge_percentage,
			effective_from, effective_to, created_at, updated_at
		FROM prize_structures
		WHERE game_id = $1
			AND effective_from <= NOW()
			AND (effective_to IS NULL OR effective_to > NOW())
		ORDER BY effective_from DESC
		LIMIT 1`

	structure := &models.PrizeStructure{}
	err := r.db.QueryRowContext(ctx, query, gameID).Scan(
		&structure.ID, &structure.GameID, &structure.TotalPrizePool,
		&structure.HouseEdgePercentage, &structure.EffectiveFrom, &structure.EffectiveTo,
		&structure.CreatedAt, &structure.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active prize structure found")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get active prize structure")
		return nil, fmt.Errorf("failed to get active prize structure: %w", err)
	}

	// Load prize tiers
	tiers, err := r.GetPrizeTiers(ctx, structure.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get prize tiers: %w", err)
	}

	if tiers != nil {
		structure.Tiers = make([]models.PrizeTier, len(tiers))
		for i, tier := range tiers {
			structure.Tiers[i] = *tier
		}
	}

	return structure, nil
}

// CreatePrizeTier creates a new prize tier
func (r *prizeStructureRepository) CreatePrizeTier(ctx context.Context, tier *models.PrizeTier) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.prize_tier.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.table", "prize_tiers"),
		attribute.String("structure.id", tier.PrizeStructureID.String()),
	)

	query := `
		INSERT INTO prize_tiers (
			id, prize_structure_id, tier_number, name,
			matches_required, prize_amount, prize_percentage,
			estimated_winners, description
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at`

	tier.ID = uuid.New()

	err := r.db.QueryRowContext(ctx, query,
		tier.ID, tier.PrizeStructureID, tier.TierNumber, tier.Name,
		tier.MatchesRequired, tier.PrizeAmount, tier.PrizePercentage,
		tier.EstimatedWinners, tier.Description,
	).Scan(&tier.CreatedAt, &tier.UpdatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create prize tier")
		return fmt.Errorf("failed to create prize tier: %w", err)
	}

	span.SetAttributes(attribute.String("prize_tier.id", tier.ID.String()))
	return nil
}

// GetPrizeTiers retrieves all prize tiers for a structure
func (r *prizeStructureRepository) GetPrizeTiers(ctx context.Context, structureID uuid.UUID) ([]*models.PrizeTier, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.prize_tier.list")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "prize_tiers"),
		attribute.String("structure.id", structureID.String()),
	)

	query := `
		SELECT id, prize_structure_id, tier_number, name,
			matches_required, prize_amount, prize_percentage,
			estimated_winners, description, created_at, updated_at
		FROM prize_tiers
		WHERE prize_structure_id = $1
		ORDER BY tier_number ASC`

	rows, err := r.db.QueryContext(ctx, query, structureID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get prize tiers")
		return nil, fmt.Errorf("failed to get prize tiers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tiers []*models.PrizeTier
	for rows.Next() {
		tier := &models.PrizeTier{}
		err := rows.Scan(
			&tier.ID, &tier.PrizeStructureID, &tier.TierNumber, &tier.Name,
			&tier.MatchesRequired, &tier.PrizeAmount, &tier.PrizePercentage,
			&tier.EstimatedWinners, &tier.Description, &tier.CreatedAt, &tier.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan prize tier: %w", err)
		}
		tiers = append(tiers, tier)
	}

	span.SetAttributes(attribute.Int("tiers.count", len(tiers)))
	return tiers, nil
}

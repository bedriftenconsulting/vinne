package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// GameRepository defines the interface for game data operations
type GameRepository interface {
	Create(ctx context.Context, game *models.Game) error
	CreateWithTx(ctx context.Context, tx *sql.Tx, game *models.Game) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Game, error)
	Update(ctx context.Context, game *models.Game) error
	UpdateLogo(ctx context.Context, gameID uuid.UUID, logoURL *string, brandColor *string) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter models.GameFilter, page, limit int) ([]*models.Game, int64, error)
	GetByName(ctx context.Context, name string) (*models.Game, error)
	GetByNameWithTx(ctx context.Context, tx *sql.Tx, name string) (*models.Game, error)
	GetActiveGames(ctx context.Context) ([]*models.Game, error)
	GetEnabledBetTypesByGameIDs(ctx context.Context, gameIDs []uuid.UUID) (map[uuid.UUID][]models.BetType, error)
}

// gameRepository implements GameRepository interface
type gameRepository struct {
	db *sql.DB
}

// NewGameRepository creates a new instance of GameRepository
func NewGameRepository(db *sql.DB) GameRepository {
	return &gameRepository{
		db: db,
	}
}

// Create creates a new game
func (r *gameRepository) Create(ctx context.Context, game *models.Game) error {
	return r.CreateWithTx(ctx, nil, game)
}

// CreateWithTx creates a new game within a transaction
func (r *gameRepository) CreateWithTx(ctx context.Context, tx *sql.Tx, game *models.Game) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.table", "games"),
		attribute.String("game.name", game.Name),
		attribute.String("game.type", game.Type),
		attribute.Bool("in_transaction", tx != nil),
	)

	query := `
		INSERT INTO games (id, code, name, type, game_type, game_format, game_category, organizer,
			min_stake_amount, max_stake_amount, max_tickets_per_player, draw_frequency,
			draw_days, draw_time, weekly_schedule, status, description,
			start_time, end_time, version, start_time_str, end_time_str, draw_time_str,
			number_range_min, number_range_max, selection_count, sales_cutoff_minutes,
			base_price, multi_draw_enabled, max_draws_advance, logo_url, brand_color)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32)
		RETURNING created_at, updated_at`

	game.ID = uuid.New()
	if game.Status == "" {
		game.Status = "DRAFT"
	}
	if game.Version == "" {
		game.Version = "1.0"
	}

	// Marshal DrawDays as JSON since the column is JSONB
	drawDaysJSON, err := json.Marshal(game.DrawDays)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal draw days")
		return fmt.Errorf("failed to marshal draw days: %w", err)
	}

	// Use transaction if provided, otherwise use the database connection
	var row *sql.Row
	if tx != nil {
		row = tx.QueryRowContext(ctx, query,
			game.ID, game.Code, game.Name, game.Type, game.GameType, game.GameFormat, game.GameCategory,
			game.Organizer, game.MinStakeAmount, game.MaxStakeAmount, game.MaxTicketsPerPlayer,
			game.DrawFrequency, drawDaysJSON, game.DrawTime, game.WeeklySchedule,
			game.Status, game.Description, game.StartTime,
			game.EndTime, game.Version, game.StartTimeStr, game.EndTimeStr, game.DrawTimeStr,
			game.NumberRangeMin, game.NumberRangeMax, game.SelectionCount, game.SalesCutoffMinutes,
			game.BasePrice, game.MultiDrawEnabled, game.MaxDrawsAdvance, game.LogoURL, game.BrandColor,
		)
	} else {
		row = r.db.QueryRowContext(ctx, query,
			game.ID, game.Code, game.Name, game.Type, game.GameType, game.GameFormat, game.GameCategory,
			game.Organizer, game.MinStakeAmount, game.MaxStakeAmount, game.MaxTicketsPerPlayer,
			game.DrawFrequency, drawDaysJSON, game.DrawTime, game.WeeklySchedule,
			game.Status, game.Description, game.StartTime,
			game.EndTime, game.Version, game.StartTimeStr, game.EndTimeStr, game.DrawTimeStr,
			game.NumberRangeMin, game.NumberRangeMax, game.SelectionCount, game.SalesCutoffMinutes,
			game.BasePrice, game.MultiDrawEnabled, game.MaxDrawsAdvance, game.LogoURL, game.BrandColor,
		)
	}

	err = row.Scan(&game.CreatedAt, &game.UpdatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create game")
		return fmt.Errorf("failed to create game: %w", err)
	}

	span.SetAttributes(attribute.String("game.id", game.ID.String()))
	return nil
}

// GetByID retrieves a game by ID
func (r *gameRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Game, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game.get_by_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "games"),
		attribute.String("game.id", id.String()),
	)

	query := `
		SELECT id, code, name, type, game_type, game_format, game_category, organizer,
			min_stake_amount, max_stake_amount, max_tickets_per_player,
			draw_frequency, draw_days, draw_time, sales_cutoff_minutes, weekly_schedule, status,
			description, start_time, end_time, version,
			start_time_str, end_time_str, draw_time_str,
			number_range_min, number_range_max, selection_count,
			base_price, multi_draw_enabled, max_draws_advance,
			logo_url, brand_color,
			created_at, updated_at
		FROM games
		WHERE id = $1`

	game := &models.Game{}
	var drawDaysJSON []byte
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&game.ID, &game.Code, &game.Name, &game.Type, &game.GameType, &game.GameFormat,
		&game.GameCategory, &game.Organizer, &game.MinStakeAmount,
		&game.MaxStakeAmount, &game.MaxTicketsPerPlayer, &game.DrawFrequency,
		&drawDaysJSON, &game.DrawTime, &game.SalesCutoffMinutes, &game.WeeklySchedule, &game.Status,
		&game.Description, &game.StartTime,
		&game.EndTime, &game.Version, &game.StartTimeStr, &game.EndTimeStr, &game.DrawTimeStr,
		&game.NumberRangeMin, &game.NumberRangeMax, &game.SelectionCount,
		&game.BasePrice, &game.MultiDrawEnabled, &game.MaxDrawsAdvance,
		&game.LogoURL, &game.BrandColor,
		&game.CreatedAt, &game.UpdatedAt,
	)

	if err == nil {
		// Unmarshal the JSON data into the DrawDays slice
		if len(drawDaysJSON) > 0 {
			err = json.Unmarshal(drawDaysJSON, &game.DrawDays)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to unmarshal draw days")
				return nil, fmt.Errorf("failed to unmarshal draw days: %w", err)
			}
		}
	}

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.Bool("game.found", false))
			return nil, fmt.Errorf("game not found")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get game")
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	span.SetAttributes(
		attribute.Bool("game.found", true),
		attribute.String("game.name", game.Name),
	)
	return game, nil
}

// Update updates an existing game
func (r *gameRepository) Update(ctx context.Context, game *models.Game) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game.update")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "games"),
		attribute.String("game.id", game.ID.String()),
	)

	query := `
		UPDATE games SET
			code = $2, name = $3, type = $4, game_type = $5, game_format = $6, game_category = $7,
			organizer = $8, min_stake_amount = $9, max_stake_amount = $10,
			max_tickets_per_player = $11, draw_frequency = $12, draw_days = $13,
			draw_time = $14, weekly_schedule = $15, status = $16, description = $17,
			start_time = $18, end_time = $19,
			version = $20, start_time_str = $21, end_time_str = $22, draw_time_str = $23,
			number_range_min = $24, number_range_max = $25, selection_count = $26, sales_cutoff_minutes = $27,
			base_price = $28, multi_draw_enabled = $29, max_draws_advance = $30,
			logo_url = $31, brand_color = $32,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	// Marshal DrawDays as JSON since the column is JSONB
	drawDaysJSON, err := json.Marshal(game.DrawDays)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal draw days")
		return fmt.Errorf("failed to marshal draw days: %w", err)
	}

	fmt.Printf("[GameRepository] Update: Saving sales_cutoff_minutes to database: game_id=%s, value=%d\n",
		game.ID.String(), game.SalesCutoffMinutes)

	err = r.db.QueryRowContext(ctx, query,
		game.ID, game.Code, game.Name, game.Type, game.GameType, game.GameFormat,
		game.GameCategory, game.Organizer, game.MinStakeAmount, game.MaxStakeAmount,
		game.MaxTicketsPerPlayer, game.DrawFrequency, drawDaysJSON, game.DrawTime,
		game.WeeklySchedule, game.Status, game.Description,
		game.StartTime, game.EndTime, game.Version,
		game.StartTimeStr, game.EndTimeStr, game.DrawTimeStr,
		game.NumberRangeMin, game.NumberRangeMax, game.SelectionCount, game.SalesCutoffMinutes,
		game.BasePrice, game.MultiDrawEnabled, game.MaxDrawsAdvance,
		game.LogoURL, game.BrandColor,
	).Scan(&game.UpdatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update game")
		return fmt.Errorf("failed to update game: %w", err)
	}

	return nil
}

// UpdateLogo updates the logo URL and brand color for a game
func (r *gameRepository) UpdateLogo(ctx context.Context, gameID uuid.UUID, logoURL *string, brandColor *string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game.update_logo")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "games"),
		attribute.String("game.id", gameID.String()),
	)

	query := `
		UPDATE games SET
			logo_url = $2,
			brand_color = $3,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, query, gameID, logoURL, brandColor).Scan(&updatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update game logo")
		return fmt.Errorf("failed to update game logo: %w", err)
	}

	span.SetAttributes(
		attribute.String("game.logo_url", func() string {
			if logoURL != nil {
				return *logoURL
			}
			return "null"
		}()),
		attribute.String("game.brand_color", func() string {
			if brandColor != nil {
				return *brandColor
			}
			return "null"
		}()),
	)

	return nil
}

// Delete soft deletes a game (marks as terminated)
func (r *gameRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game.delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "games"),
		attribute.String("game.id", id.String()),
	)

	query := `UPDATE games SET status = $2, updated_at = NOW() WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id, "TERMINATED")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete game")
		return fmt.Errorf("failed to delete game: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("game not found")
	}

	return nil
}

// List retrieves games with filtering and pagination
func (r *gameRepository) List(ctx context.Context, filter models.GameFilter, page, limit int) ([]*models.Game, int64, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game.list")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "games"),
		attribute.Int("pagination.page", page),
		attribute.Int("pagination.limit", limit),
	)

	// Build WHERE clause
	conditions := []string{}
	args := []interface{}{}
	argCount := 0

	if filter.GameFormat != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("game_format = $%d", argCount))
		args = append(args, *filter.GameFormat)
	}

	if filter.GameCategory != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("game_category = $%d", argCount))
		args = append(args, *filter.GameCategory)
	}

	if filter.Organizer != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("organizer = $%d", argCount))
		args = append(args, *filter.Organizer)
	}

	if filter.Status != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("status = $%d", argCount))
		args = append(args, *filter.Status)
	}

	if filter.SearchQuery != nil && *filter.SearchQuery != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argCount))
		args = append(args, "%"+*filter.SearchQuery+"%")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM games %s", whereClause)
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to count games")
		return nil, 0, fmt.Errorf("failed to count games: %w", err)
	}

	// Get paginated results
	offset := (page - 1) * limit
	argCount++
	limitArg := argCount
	argCount++
	offsetArg := argCount

	query := fmt.Sprintf(`
		SELECT id, code, name, type, game_type, game_format, game_category, organizer,
			min_stake_amount, max_stake_amount, max_tickets_per_player,
			draw_frequency, draw_days, draw_time, sales_cutoff_minutes, weekly_schedule, status,
			description, start_time, end_time, version,
			start_time_str, end_time_str, draw_time_str,
			number_range_min, number_range_max, selection_count,
			base_price, multi_draw_enabled, max_draws_advance,
			logo_url, brand_color,
			created_at, updated_at
		FROM games %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, limitArg, offsetArg)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to list games")
		return nil, 0, fmt.Errorf("failed to list games: %w", err)
	}
	defer func() { _ = rows.Close() }()

	games := []*models.Game{}
	for rows.Next() {
		game := &models.Game{}
		var drawDaysJSON []byte
		err := rows.Scan(
			&game.ID, &game.Code, &game.Name, &game.Type, &game.GameType, &game.GameFormat,
			&game.GameCategory, &game.Organizer, &game.MinStakeAmount,
			&game.MaxStakeAmount, &game.MaxTicketsPerPlayer, &game.DrawFrequency,
			&drawDaysJSON, &game.DrawTime, &game.SalesCutoffMinutes, &game.WeeklySchedule, &game.Status,
			&game.Description, &game.StartTime,
			&game.EndTime, &game.Version, &game.StartTimeStr, &game.EndTimeStr, &game.DrawTimeStr,
			&game.NumberRangeMin, &game.NumberRangeMax, &game.SelectionCount,
			&game.BasePrice, &game.MultiDrawEnabled, &game.MaxDrawsAdvance,
			&game.LogoURL, &game.BrandColor,
			&game.CreatedAt, &game.UpdatedAt,
		)
		if err != nil {
			span.RecordError(err)
			return nil, 0, fmt.Errorf("failed to scan game: %w", err)
		}

		// Unmarshal DrawDays JSON
		if drawDaysJSON != nil {
			if err := json.Unmarshal(drawDaysJSON, &game.DrawDays); err != nil {
				span.RecordError(err)
				return nil, 0, fmt.Errorf("failed to unmarshal draw days: %w", err)
			}
		}

		games = append(games, game)
	}

	span.SetAttributes(
		attribute.Int64("games.count", int64(len(games))),
		attribute.Int64("games.total", total),
	)

	return games, total, nil
}

// GetByName retrieves a game by name
func (r *gameRepository) GetByName(ctx context.Context, name string) (*models.Game, error) {
	return r.GetByNameWithTx(ctx, nil, name)
}

// GetByNameWithTx retrieves a game by name within a transaction
func (r *gameRepository) GetByNameWithTx(ctx context.Context, tx *sql.Tx, name string) (*models.Game, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game.get_by_name")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "games"),
		attribute.String("game.name", name),
		attribute.Bool("in_transaction", tx != nil),
	)

	query := `
		SELECT id, code, name, type, game_type, game_format, game_category, organizer,
			min_stake_amount, max_stake_amount, max_tickets_per_player,
			draw_frequency, draw_days, draw_time, sales_cutoff_minutes, weekly_schedule, status,
			description, start_time, end_time, version,
			start_time_str, end_time_str, draw_time_str,
			number_range_min, number_range_max, selection_count,
			base_price, multi_draw_enabled, max_draws_advance,
			logo_url, brand_color,
			created_at, updated_at
		FROM games
		WHERE name = $1
		FOR UPDATE` // Add row-level lock for transaction

	game := &models.Game{}
	var drawDaysJSON []byte

	// Use transaction if provided, otherwise use the database connection
	var row *sql.Row
	if tx != nil {
		row = tx.QueryRowContext(ctx, query, name)
	} else {
		// Remove FOR UPDATE if not in transaction
		queryNoLock := strings.Replace(query, "FOR UPDATE", "", 1)
		row = r.db.QueryRowContext(ctx, queryNoLock, name)
	}

	err := row.Scan(
		&game.ID, &game.Code, &game.Name, &game.Type, &game.GameType, &game.GameFormat,
		&game.GameCategory, &game.Organizer, &game.MinStakeAmount,
		&game.MaxStakeAmount, &game.MaxTicketsPerPlayer, &game.DrawFrequency,
		&drawDaysJSON, &game.DrawTime, &game.SalesCutoffMinutes, &game.WeeklySchedule, &game.Status,
		&game.Description, &game.StartTime,
		&game.EndTime, &game.Version, &game.StartTimeStr, &game.EndTimeStr, &game.DrawTimeStr,
		&game.NumberRangeMin, &game.NumberRangeMax, &game.SelectionCount,
		&game.BasePrice, &game.MultiDrawEnabled, &game.MaxDrawsAdvance,
		&game.LogoURL, &game.BrandColor,
		&game.CreatedAt, &game.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("game not found")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get game by name")
		return nil, fmt.Errorf("failed to get game by name: %w", err)
	}

	// Unmarshal DrawDays JSON
	if drawDaysJSON != nil {
		if err := json.Unmarshal(drawDaysJSON, &game.DrawDays); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to unmarshal draw days: %w", err)
		}
	}

	return game, nil
}

// GetActiveGames retrieves all active games
func (r *gameRepository) GetActiveGames(ctx context.Context) ([]*models.Game, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game.get_active")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "games"),
		attribute.String("filter.status", "ACTIVE"),
	)

	query := `
		SELECT id, code, name, type, game_type, game_format, game_category, organizer,
			min_stake_amount, max_stake_amount, max_tickets_per_player,
			draw_frequency, draw_days, draw_time, sales_cutoff_minutes, weekly_schedule, status,
			description, start_time, end_time, version,
			start_time_str, end_time_str, draw_time_str,
			number_range_min, number_range_max, selection_count,
			base_price, multi_draw_enabled, max_draws_advance,
			logo_url, brand_color,
			created_at, updated_at
		FROM games
		WHERE UPPER(status) = 'ACTIVE'
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get active games")
		return nil, fmt.Errorf("failed to get active games: %w", err)
	}
	defer func() { _ = rows.Close() }()

	games := []*models.Game{}
	for rows.Next() {
		game := &models.Game{}
		var drawDaysJSON []byte
		err := rows.Scan(
			&game.ID, &game.Code, &game.Name, &game.Type, &game.GameType, &game.GameFormat,
			&game.GameCategory, &game.Organizer, &game.MinStakeAmount,
			&game.MaxStakeAmount, &game.MaxTicketsPerPlayer, &game.DrawFrequency,
			&drawDaysJSON, &game.DrawTime, &game.SalesCutoffMinutes, &game.WeeklySchedule, &game.Status,
			&game.Description, &game.StartTime,
			&game.EndTime, &game.Version, &game.StartTimeStr, &game.EndTimeStr, &game.DrawTimeStr,
			&game.NumberRangeMin, &game.NumberRangeMax, &game.SelectionCount,
			&game.BasePrice, &game.MultiDrawEnabled, &game.MaxDrawsAdvance,
			&game.LogoURL, &game.BrandColor,
			&game.CreatedAt, &game.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan game: %w", err)
		}

		// Unmarshal DrawDays JSON
		if drawDaysJSON != nil {
			if err := json.Unmarshal(drawDaysJSON, &game.DrawDays); err != nil {
				return nil, fmt.Errorf("failed to unmarshal draw days: %w", err)
			}
		}

		games = append(games, game)
	}

	span.SetAttributes(attribute.Int("games.active_count", len(games)))
	return games, nil
}

// GetEnabledBetTypesByGameIDs fetches enabled bet types for the specified games
func (r *gameRepository) GetEnabledBetTypesByGameIDs(ctx context.Context, gameIDs []uuid.UUID) (map[uuid.UUID][]models.BetType, error) {
	if len(gameIDs) == 0 {
		return map[uuid.UUID][]models.BetType{}, nil
	}

	placeholders := make([]string, 0, len(gameIDs))
	args := make([]any, 0, len(gameIDs))
	for i, id := range gameIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, id)
	}

	// Join game_bet_types -> bet_types and return mappings
	query := fmt.Sprintf(`
        SELECT gbt.game_id, bt.id, bt.name, gbt.enabled, gbt.multiplier
        FROM game_bet_types gbt
        JOIN bet_types bt ON bt.id = gbt.bet_type_id
        WHERE gbt.enabled = TRUE AND gbt.game_id IN (%s)
    `, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query bet types: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[uuid.UUID][]models.BetType)
	for rows.Next() {
		var gameID uuid.UUID
		var bt models.BetType
		if err := rows.Scan(&gameID, &bt.ID, &bt.Name, &bt.Enabled, &bt.Multiplier); err != nil {
			return nil, fmt.Errorf("failed to scan bet type: %w", err)
		}
		result[gameID] = append(result[gameID], bt)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bet types: %w", err)
	}

	return result, nil
}

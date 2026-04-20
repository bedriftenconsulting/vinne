package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// GameScheduleRepository defines the interface for game schedule data operations
type GameScheduleRepository interface {
	Create(ctx context.Context, schedule *models.GameSchedule) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.GameSchedule, error)
	GetByGameID(ctx context.Context, gameID uuid.UUID) ([]*models.GameSchedule, error)
	Update(ctx context.Context, schedule *models.GameSchedule) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetActiveSchedules(ctx context.Context) ([]*models.GameSchedule, error)
	GetUpcomingSchedules(ctx context.Context, limit int) ([]*models.GameSchedule, error)
	GetSchedulesInTimeRange(ctx context.Context, start, end time.Time) ([]*models.GameSchedule, error)
	HasScheduleForGameInRange(ctx context.Context, gameID uuid.UUID, start, end time.Time) (bool, error)
	DeleteSchedulesInTimeRange(ctx context.Context, start, end time.Time) error
	DeleteUnplayedSchedulesInTimeRange(ctx context.Context, start, end time.Time) error
	GetSchedulesDueForProcessing(ctx context.Context, eventType string, windowMinutes int) ([]*models.GameSchedule, error)
}

// gameScheduleRepository implements GameScheduleRepository interface
type gameScheduleRepository struct {
	db *sql.DB
}

// NewGameScheduleRepository creates a new instance of GameScheduleRepository
func NewGameScheduleRepository(db *sql.DB) GameScheduleRepository {
	return &gameScheduleRepository{
		db: db,
	}
}

// Create creates a new game schedule
func (r *gameScheduleRepository) Create(ctx context.Context, schedule *models.GameSchedule) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_schedule.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.table", "game_schedules"),
		attribute.String("game.id", schedule.GameID.String()),
		attribute.String("frequency", string(schedule.Frequency)),
	)

	query := `
		INSERT INTO game_schedules (
			id, game_id, game_name, scheduled_start, scheduled_end, scheduled_draw,
			frequency, is_active, status, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at`

	schedule.ID = uuid.New()
	// Set default status if not set
	if schedule.Status == "" {
		schedule.Status = models.ScheduleStatusScheduled
	}
	// IsActive defaults to true only if not explicitly set
	// Note: Go's zero value for bool is false, so we can't distinguish between
	// explicit false and unset. The test sets values explicitly, so we trust the input.

	err := r.db.QueryRowContext(ctx, query,
		schedule.ID, schedule.GameID, schedule.GameName, schedule.ScheduledStart, schedule.ScheduledEnd,
		schedule.ScheduledDraw, schedule.Frequency, schedule.IsActive, schedule.Status, schedule.Notes,
	).Scan(&schedule.CreatedAt, &schedule.UpdatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create game schedule")
		return fmt.Errorf("failed to create game schedule: %w", err)
	}

	span.SetAttributes(attribute.String("schedule.id", schedule.ID.String()))
	return nil
}

// GetByID retrieves a game schedule by ID
func (r *gameScheduleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_schedule.get_by_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_schedules"),
		attribute.String("schedule.id", id.String()),
	)

	query := `
		SELECT gs.id, gs.game_id, gs.game_name, g.code AS game_code, g.game_category AS game_category,
			g.logo_url, g.brand_color, gs.scheduled_start, gs.scheduled_end, gs.scheduled_draw,
			gs.frequency, gs.is_active, gs.status, gs.draw_result_id, gs.notes, gs.created_at, gs.updated_at
		FROM game_schedules gs
		LEFT JOIN games g ON gs.game_id = g.id
		WHERE gs.id = $1`

	schedule := &models.GameSchedule{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&schedule.ID, &schedule.GameID, &schedule.GameName, &schedule.GameCode, &schedule.GameCategory,
		&schedule.LogoURL, &schedule.BrandColor, &schedule.ScheduledStart, &schedule.ScheduledEnd,
		&schedule.ScheduledDraw, &schedule.Frequency, &schedule.IsActive, &schedule.Status,
		&schedule.DrawResultID, &schedule.Notes, &schedule.CreatedAt, &schedule.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("game schedule not found")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get game schedule")
		return nil, fmt.Errorf("failed to get game schedule: %w", err)
	}

	return schedule, nil
}

// GetByGameID retrieves all schedules for a game
func (r *gameScheduleRepository) GetByGameID(ctx context.Context, gameID uuid.UUID) ([]*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_schedule.get_by_game_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_schedules"),
		attribute.String("game.id", gameID.String()),
	)

	query := `
		SELECT gs.id, gs.game_id, gs.game_name, g.code AS game_code, g.game_category AS game_category, gs.scheduled_start, gs.scheduled_end, gs.scheduled_draw,
			gs.frequency, gs.is_active, gs.status, gs.draw_result_id, gs.notes, gs.created_at, gs.updated_at
		FROM game_schedules gs
		LEFT JOIN games g ON gs.game_id = g.id
		WHERE gs.game_id = $1
		ORDER BY gs.scheduled_start ASC`

	rows, err := r.db.QueryContext(ctx, query, gameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get game schedules")
		return nil, fmt.Errorf("failed to get game schedules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var schedules []*models.GameSchedule
	for rows.Next() {
		schedule := &models.GameSchedule{}
		err := rows.Scan(
			&schedule.ID, &schedule.GameID, &schedule.GameName, &schedule.GameCode, &schedule.GameCategory, &schedule.ScheduledStart, &schedule.ScheduledEnd,
			&schedule.ScheduledDraw, &schedule.Frequency, &schedule.IsActive, &schedule.Status,
			&schedule.DrawResultID, &schedule.Notes, &schedule.CreatedAt, &schedule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %w", err)
		}
		schedules = append(schedules, schedule)
	}

	span.SetAttributes(attribute.Int("schedules.count", len(schedules)))
	return schedules, nil
}

// Update updates an existing game schedule
func (r *gameScheduleRepository) Update(ctx context.Context, schedule *models.GameSchedule) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_schedule.update")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "game_schedules"),
		attribute.String("schedule.id", schedule.ID.String()),
	)

	query := `
		UPDATE game_schedules SET
			game_name = $2, scheduled_start = $3, scheduled_end = $4, scheduled_draw = $5,
			frequency = $6, is_active = $7, status = $8, draw_result_id = $9, notes = $10,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	err := r.db.QueryRowContext(ctx, query,
		schedule.ID, schedule.GameName, schedule.ScheduledStart, schedule.ScheduledEnd,
		schedule.ScheduledDraw, schedule.Frequency, schedule.IsActive, schedule.Status,
		schedule.DrawResultID, schedule.Notes,
	).Scan(&schedule.UpdatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update game schedule")
		return fmt.Errorf("failed to update game schedule: %w", err)
	}

	return nil
}

// Delete deletes a game schedule
func (r *gameScheduleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_schedule.delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "DELETE"),
		attribute.String("db.table", "game_schedules"),
		attribute.String("schedule.id", id.String()),
	)

	query := `DELETE FROM game_schedules WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete game schedule")
		return fmt.Errorf("failed to delete game schedule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("game schedule not found")
	}

	return nil
}

// GetActiveSchedules retrieves all active game schedules
func (r *gameScheduleRepository) GetActiveSchedules(ctx context.Context) ([]*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_schedule.get_active")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_schedules"),
		attribute.Bool("filter.is_active", true),
	)

	query := `
		SELECT gs.id, gs.game_id, gs.game_name, g.code AS game_code, g.game_category AS game_category, gs.scheduled_start, gs.scheduled_end, gs.scheduled_draw,
			gs.frequency, gs.is_active, gs.status, gs.draw_result_id, gs.notes, gs.created_at, gs.updated_at
		FROM game_schedules gs
		LEFT JOIN games g ON gs.game_id = g.id
		WHERE gs.is_active = true
		ORDER BY gs.scheduled_start ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get active schedules")
		return nil, fmt.Errorf("failed to get active schedules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var schedules []*models.GameSchedule
	for rows.Next() {
		schedule := &models.GameSchedule{}
		err := rows.Scan(
			&schedule.ID, &schedule.GameID, &schedule.GameName, &schedule.GameCode, &schedule.GameCategory, &schedule.ScheduledStart, &schedule.ScheduledEnd,
			&schedule.ScheduledDraw, &schedule.Frequency, &schedule.IsActive, &schedule.Status,
			&schedule.DrawResultID, &schedule.Notes, &schedule.CreatedAt, &schedule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %w", err)
		}
		schedules = append(schedules, schedule)
	}

	span.SetAttributes(attribute.Int("schedules.active_count", len(schedules)))
	return schedules, nil
}

// GetUpcomingSchedules retrieves upcoming game schedules
func (r *gameScheduleRepository) GetUpcomingSchedules(ctx context.Context, limit int) ([]*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_schedule.get_upcoming")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_schedules"),
		attribute.Int("limit", limit),
	)

	query := `
		SELECT gs.id, gs.game_id, gs.game_name, g.code AS game_code, g.game_category AS game_category, gs.scheduled_start, gs.scheduled_end, gs.scheduled_draw,
			gs.frequency, gs.is_active, gs.status, gs.draw_result_id, gs.notes, gs.created_at, gs.updated_at
		FROM game_schedules gs
		LEFT JOIN games g ON gs.game_id = g.id
		WHERE gs.is_active = true
			AND gs.scheduled_start > NOW()
		ORDER BY gs.scheduled_start ASC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get upcoming schedules")
		return nil, fmt.Errorf("failed to get upcoming schedules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var schedules []*models.GameSchedule
	for rows.Next() {
		schedule := &models.GameSchedule{}
		err := rows.Scan(
			&schedule.ID, &schedule.GameID, &schedule.GameName, &schedule.GameCode, &schedule.GameCategory, &schedule.ScheduledStart, &schedule.ScheduledEnd,
			&schedule.ScheduledDraw, &schedule.Frequency, &schedule.IsActive, &schedule.Status,
			&schedule.DrawResultID, &schedule.Notes, &schedule.CreatedAt, &schedule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %w", err)
		}
		schedules = append(schedules, schedule)
	}

	span.SetAttributes(
		attribute.Int("schedules.upcoming_count", len(schedules)),
		attribute.String("next.start", func() string {
			if len(schedules) > 0 {
				return schedules[0].ScheduledStart.Format(time.RFC3339)
			}
			return ""
		}()),
	)
	return schedules, nil
}

// HasScheduleForGameInRange returns true if a non-cancelled schedule already exists for the game in the range
func (r *gameScheduleRepository) HasScheduleForGameInRange(ctx context.Context, gameID uuid.UUID, start, end time.Time) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM game_schedules
		 WHERE game_id = $1 AND scheduled_draw >= $2 AND scheduled_draw <= $3
		   AND status NOT IN ('CANCELLED', 'cancelled')`,
		gameID, start, end,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("HasScheduleForGameInRange: %w", err)
	}
	return count > 0, nil
}

// GetSchedulesInTimeRange retrieves schedules within a time range
func (r *gameScheduleRepository) GetSchedulesInTimeRange(ctx context.Context, start, end time.Time) ([]*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_schedule.get_time_range")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_schedules"),
		attribute.String("start", start.Format(time.RFC3339)),
		attribute.String("end", end.Format(time.RFC3339)),
	)

	query := `
        SELECT gs.id, gs.game_id, gs.game_name,
               g.code AS game_code, g.game_category AS game_category,
               g.logo_url, g.brand_color,
               gs.scheduled_start, gs.scheduled_end, gs.scheduled_draw,
               gs.frequency, gs.is_active, gs.status, gs.draw_result_id,
               gs.notes, gs.created_at, gs.updated_at,
               COALESCE(bt_agg.bet_types, '[]') AS bet_types_json
        FROM game_schedules gs
        LEFT JOIN games g ON gs.game_id = g.id
        LEFT JOIN LATERAL (
            SELECT json_agg(json_build_object(
                'id', bt.id,
                'name', bt.name,
                'enabled', gbt.enabled,
                'multiplier', gbt.multiplier
            )) AS bet_types
            FROM game_bet_types gbt
            JOIN bet_types bt ON bt.id = gbt.bet_type_id
            WHERE gbt.enabled = TRUE AND gbt.game_id = gs.game_id
        ) bt_agg ON TRUE
        WHERE gs.scheduled_start >= $1 AND gs.scheduled_start <= $2
        ORDER BY gs.scheduled_start ASC`

	rows, err := r.db.QueryContext(ctx, query, start, end)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get schedules in time range")
		return nil, fmt.Errorf("failed to get schedules in time range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var schedules []*models.GameSchedule
	for rows.Next() {
		schedule := &models.GameSchedule{}
		var betTypesJSON []byte
		err := rows.Scan(
			&schedule.ID, &schedule.GameID, &schedule.GameName, &schedule.GameCode, &schedule.GameCategory,
			&schedule.LogoURL, &schedule.BrandColor,
			&schedule.ScheduledStart, &schedule.ScheduledEnd,
			&schedule.ScheduledDraw, &schedule.Frequency, &schedule.IsActive, &schedule.Status,
			&schedule.DrawResultID, &schedule.Notes, &schedule.CreatedAt, &schedule.UpdatedAt,
			&betTypesJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %w", err)
		}
		if len(betTypesJSON) > 0 {
			var bts []models.BetType
			if err := json.Unmarshal(betTypesJSON, &bts); err == nil {
				schedule.BetTypes = bts
			}
		}
		schedules = append(schedules, schedule)
	}

	span.SetAttributes(attribute.Int("schedules.count", len(schedules)))
	return schedules, nil
}

// DeleteSchedulesInTimeRange deletes schedules within a time range
func (r *gameScheduleRepository) DeleteSchedulesInTimeRange(ctx context.Context, start, end time.Time) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_schedule.delete_time_range")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "DELETE"),
		attribute.String("db.table", "game_schedules"),
		attribute.String("start", start.Format(time.RFC3339)),
		attribute.String("end", end.Format(time.RFC3339)),
	)

	query := `DELETE FROM game_schedules WHERE scheduled_start >= $1 AND scheduled_start <= $2`
	result, err := r.db.ExecContext(ctx, query, start, end)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete schedules in time range")
		return fmt.Errorf("failed to delete schedules in time range: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	span.SetAttributes(attribute.Int64("rows_deleted", rowsAffected))
	return nil
}

// DeleteUnplayedSchedulesInTimeRange deletes only unplayed (SCHEDULED status) schedules within a time range
func (r *gameScheduleRepository) DeleteUnplayedSchedulesInTimeRange(ctx context.Context, start, end time.Time) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_schedule.delete_unplayed_time_range")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "DELETE"),
		attribute.String("db.table", "game_schedules"),
		attribute.String("start", start.Format(time.RFC3339)),
		attribute.String("end", end.Format(time.RFC3339)),
		attribute.String("filter.status", "SCHEDULED"),
	)

	// Delete SCHEDULED entries for this week — use scheduled_draw range to catch special games too
	query := `DELETE FROM game_schedules
		WHERE (
			(scheduled_start >= $1 AND scheduled_start <= $2)
			OR (scheduled_draw >= $1 AND scheduled_draw <= $2)
		)
		AND (status = 'SCHEDULED' OR status IS NULL)`

	result, err := r.db.ExecContext(ctx, query, start, end)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete unplayed schedules in time range")
		return fmt.Errorf("failed to delete unplayed schedules in time range: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	span.SetAttributes(attribute.Int64("rows_deleted", rowsAffected))
	return nil
}

// GetSchedulesDueForProcessing retrieves schedules that are due for processing within a time window
func (r *gameScheduleRepository) GetSchedulesDueForProcessing(ctx context.Context, eventType string, windowMinutes int) ([]*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_schedule.get_due_for_processing")
	defer span.End()

	now := time.Now()
	windowEnd := now.Add(time.Duration(windowMinutes) * time.Minute)

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_schedules"),
		attribute.String("event_type", eventType),
		attribute.Int("window_minutes", windowMinutes),
		attribute.String("window_start", now.Format(time.RFC3339)),
		attribute.String("window_end", windowEnd.Format(time.RFC3339)),
	)

	var query string
	switch eventType {
	case "sales_cutoff":
		// Find schedules where sales cutoff time is within the window (past or approaching)
		// We want: scheduled_end <= NOW + window (approaching or already passed)
		query = `
			SELECT gs.id, gs.game_id, gs.game_name, g.code AS game_code, g.game_category AS game_category, gs.scheduled_start, gs.scheduled_end, gs.scheduled_draw,
				gs.frequency, gs.is_active, gs.status, gs.draw_result_id, gs.notes, gs.created_at, gs.updated_at
			FROM game_schedules gs
			LEFT JOIN games g ON gs.game_id = g.id
			WHERE gs.status = 'SCHEDULED'
				AND gs.is_active = true
				AND gs.scheduled_end <= $1
			ORDER BY gs.scheduled_end ASC`
	case "draw_time":
		// Find schedules where draw time is within the window (past or approaching)
		query = `
			SELECT gs.id, gs.game_id, gs.game_name, g.code AS game_code, g.game_category AS game_category, gs.scheduled_start, gs.scheduled_end, gs.scheduled_draw,
				gs.frequency, gs.is_active, gs.status, gs.draw_result_id, gs.notes, gs.created_at, gs.updated_at
			FROM game_schedules gs
			LEFT JOIN games g ON gs.game_id = g.id
			WHERE (gs.status = 'SCHEDULED' OR gs.status = 'IN_PROGRESS')
				AND gs.is_active = true
				AND gs.scheduled_draw <= $1
			ORDER BY gs.scheduled_draw ASC`
	default:
		return nil, fmt.Errorf("invalid event type: %s", eventType)
	}

	rows, err := r.db.QueryContext(ctx, query, windowEnd)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get schedules due for processing")
		return nil, fmt.Errorf("failed to get schedules due for processing: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			span.RecordError(fmt.Errorf("failed to close rows: %w", err))
		}
	}()

	var schedules []*models.GameSchedule
	for rows.Next() {
		schedule := &models.GameSchedule{}
		err := rows.Scan(
			&schedule.ID, &schedule.GameID, &schedule.GameName, &schedule.GameCode, &schedule.GameCategory, &schedule.ScheduledStart, &schedule.ScheduledEnd,
			&schedule.ScheduledDraw, &schedule.Frequency, &schedule.IsActive, &schedule.Status,
			&schedule.DrawResultID, &schedule.Notes, &schedule.CreatedAt, &schedule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %w", err)
		}
		schedules = append(schedules, schedule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	span.SetAttributes(attribute.Int("schedules.count", len(schedules)))
	return schedules, nil
}

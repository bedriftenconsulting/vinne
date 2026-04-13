package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/randco/service-draw/internal/models"
)

type DrawRepository interface {
	// Draw operations
	Create(ctx context.Context, draw *models.Draw) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Draw, error)
	Update(ctx context.Context, draw *models.Draw) error
	List(ctx context.Context, gameID *uuid.UUID, status *models.DrawStatus, startDate, endDate *time.Time, page, perPage int) ([]*models.Draw, int64, error)
	ListCompletedPublic(ctx context.Context, gameID *uuid.UUID, gameCode string, latestOnly bool, startDate, endDate *time.Time, page, perPage int) ([]*models.Draw, int64, error)

	UpdateTicketStats(ctx context.Context, drawID uuid.UUID, totalTicketsSold, totalPrizePool int64) error

	// Draw schedule operations
	CreateSchedule(ctx context.Context, schedule *models.DrawSchedule) error
	GetScheduleByID(ctx context.Context, id uuid.UUID) (*models.DrawSchedule, error)
	ListSchedules(ctx context.Context, gameID *uuid.UUID, startDate, endDate *time.Time, activeOnly bool) ([]*models.DrawSchedule, error)
	UpdateSchedule(ctx context.Context, schedule *models.DrawSchedule) error

	// Payout record operations
	CreatePayoutRecord(ctx context.Context, record *models.DrawPayoutRecord) error
	GetPayoutRecord(ctx context.Context, drawID, ticketID uuid.UUID) (*models.DrawPayoutRecord, error)
	UpdatePayoutRecord(ctx context.Context, record *models.DrawPayoutRecord) error
	ListPayoutRecords(ctx context.Context, drawID uuid.UUID, status *models.PayoutStatus, limit, offset int) ([]*models.DrawPayoutRecord, int64, error)
	CreatePayoutRecordsBatch(ctx context.Context, records []*models.DrawPayoutRecord) error
	GetPendingPayouts(ctx context.Context, drawID uuid.UUID, limit int) ([]*models.DrawPayoutRecord, error)
	GetFailedPayouts(ctx context.Context, drawID uuid.UUID, maxAttempts int) ([]*models.DrawPayoutRecord, error)
	GetPayoutSummary(ctx context.Context, drawID uuid.UUID) (*models.PayoutSummary, error)
}

type drawRepository struct {
	db *sqlx.DB
}

func NewDrawRepository(db *sqlx.DB) DrawRepository {
	return &drawRepository{db: db}
}

func (r *drawRepository) Create(ctx context.Context, draw *models.Draw) error {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.Create")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw.game_id", draw.GameID.String()),
		attribute.String("draw.name", draw.DrawName),
		attribute.String("draw.status", draw.Status.String()),
	)

	// Set defaults
	if draw.ID == uuid.Nil {
		draw.ID = uuid.New()
	}
	if draw.Status == "" {
		draw.Status = models.DrawStatusScheduled
	}
	now := time.Now()
	draw.CreatedAt = now
	draw.UpdatedAt = now

	// Auto-generate draw_number if not set
	if draw.DrawNumber == 0 {
		var maxDrawNumber sql.NullInt64
		err := r.db.GetContext(ctx, &maxDrawNumber,
			"SELECT MAX(draw_number) FROM draws WHERE game_id = $1", draw.GameID)
		if err != nil && err != sql.ErrNoRows {
			span.SetAttributes(attribute.String("error", err.Error()))
			return fmt.Errorf("failed to get max draw number: %w", err)
		}

		if maxDrawNumber.Valid {
			draw.DrawNumber = int(maxDrawNumber.Int64) + 1
		} else {
			draw.DrawNumber = 1
		}

		span.SetAttributes(attribute.Int("draw.number", draw.DrawNumber))
	}

	query := `
		INSERT INTO draws (id, game_id, draw_number, game_name, game_code, game_schedule_id, draw_name, status, scheduled_time, draw_location, stage_data, created_at, updated_at)
		VALUES (:id, :game_id, :draw_number, :game_name, :game_code, :game_schedule_id, :draw_name, :status, :scheduled_time, :draw_location, :stage_data, :created_at, :updated_at)`

	_, err := r.db.NamedExecContext(ctx, query, draw)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return fmt.Errorf("failed to create draw: %w", err)
	}

	return nil
}

func (r *drawRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Draw, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.GetByID")
	defer span.End()

	span.SetAttributes(attribute.String("draw.id", id.String()))

	query := `
		SELECT id, game_id, draw_number, game_name, COALESCE(game_code, '') as game_code, game_schedule_id, draw_name, status, scheduled_time, executed_time, winning_numbers, machine_numbers,
		       nla_draw_reference, draw_location, nla_official_signature, total_tickets_sold, total_prize_pool,
		       verification_hash, stage_data, created_at, updated_at
		FROM draws
		WHERE id = $1`

	var draw models.Draw
	err := r.db.GetContext(ctx, &draw, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.String("result", "not_found"))
			return nil, fmt.Errorf("draw not found")
		}
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get draw: %w", err)
	}

	// Add stage data to trace span for observability
	if draw.StageData != nil {
		span.SetAttributes(
			attribute.Int("draw.stage_data.current_stage", draw.StageData.CurrentStage),
			attribute.String("draw.stage_data.status", string(draw.StageData.StageStatus)),
		)
	}

	return &draw, nil
}

func (r *drawRepository) Update(ctx context.Context, draw *models.Draw) error {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.Update")
	defer span.End()

	fmt.Printf("[REPOSITORY] Updating draw in database\n")
	fmt.Printf("[REPOSITORY] Draw ID: %s\n", draw.ID.String())
	fmt.Printf("[REPOSITORY] Status: %s\n", draw.Status.String())

	if draw.StageData != nil {
		fmt.Printf("[REPOSITORY] Stage Data: CurrentStage=%d, StageName=%s, StageStatus=%s\n",
			draw.StageData.CurrentStage, draw.StageData.StageName, draw.StageData.StageStatus)
		if draw.StageData.ResultCalculationData != nil {
			fmt.Printf("[REPOSITORY] Result Calculation Data: WinningTickets=%d, TotalWinnings=%d pesewas\n",
				draw.StageData.ResultCalculationData.WinningTicketsCount, draw.StageData.ResultCalculationData.TotalWinnings)
		}
	}

	span.SetAttributes(
		attribute.String("draw.id", draw.ID.String()),
		attribute.String("draw.status", draw.Status.String()),
	)

	query := `
		UPDATE draws
		SET status = :status, draw_name = :draw_name, scheduled_time = :scheduled_time,
		    executed_time = :executed_time, winning_numbers = :winning_numbers,
		    machine_numbers = :machine_numbers,
		    nla_draw_reference = :nla_draw_reference, draw_location = :draw_location,
		    nla_official_signature = :nla_official_signature, total_tickets_sold = :total_tickets_sold,
		    total_prize_pool = :total_prize_pool, verification_hash = :verification_hash,
		    game_schedule_id = :game_schedule_id, game_code = :game_code,
		    stage_data = :stage_data, updated_at = :updated_at
		WHERE id = :id`

	draw.UpdatedAt = time.Now()

	fmt.Printf("[REPOSITORY] Executing UPDATE query...\n")
	result, err := r.db.NamedExecContext(ctx, query, draw)
	if err != nil {
		fmt.Printf("[REPOSITORY] ERROR: Failed to execute UPDATE: %v\n", err)
		span.SetAttributes(attribute.String("error", err.Error()))
		return fmt.Errorf("failed to update draw: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		fmt.Printf("[REPOSITORY] ERROR: Failed to get rows affected: %v\n", err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		fmt.Printf("[REPOSITORY] WARNING: No rows affected - draw not found\n")
		span.SetAttributes(attribute.String("result", "not_found"))
		return fmt.Errorf("draw not found")
	}

	fmt.Printf("[REPOSITORY] SUCCESS: Draw updated successfully, rows_affected=%d\n", rowsAffected)
	return nil
}

func (r *drawRepository) UpdateTicketStats(ctx context.Context, drawID uuid.UUID, totalTicketsSold, totalPrizePool int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE draws SET total_tickets_sold = $1, total_prize_pool = $2, updated_at = NOW() WHERE id = $3`,
		totalTicketsSold, totalPrizePool, drawID,
	)
	return err
}

func (r *drawRepository) List(ctx context.Context, gameID *uuid.UUID, status *models.DrawStatus, startDate, endDate *time.Time, page, perPage int) ([]*models.Draw, int64, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.List")
	defer span.End()

	if page <= 0 {
		page = 1
	}
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}

	span.SetAttributes(
		attribute.Int("pagination.page", page),
		attribute.Int("pagination.per_page", perPage),
	)

	// Build WHERE conditions
	whereClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	if gameID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("game_id = $%d", argIndex))
		args = append(args, *gameID)
		argIndex++
		span.SetAttributes(attribute.String("filter.game_id", gameID.String()))
	}

	if status != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, string(*status))
		argIndex++
		span.SetAttributes(attribute.String("filter.status", status.String()))
	}

	if startDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("scheduled_time >= $%d", argIndex))
		args = append(args, *startDate)
		argIndex++
	}

	if endDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("scheduled_time <= $%d", argIndex))
		args = append(args, *endDate)
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + whereClauses[0]
		for _, clause := range whereClauses[1:] {
			whereClause += " AND " + clause
		}
	}

	// Count total records
	countQuery := "SELECT COUNT(*) FROM draws " + whereClause
	var total int64
	err := r.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to count draws: %w", err)
	}

	// Get paginated results
	offset := (page - 1) * perPage
	query := `
		SELECT id, game_id, draw_number, game_name, COALESCE(game_code, '') as game_code, game_schedule_id, draw_name, status,
		       scheduled_time, executed_time, winning_numbers, machine_numbers, nla_draw_reference, draw_location,
		       nla_official_signature, total_tickets_sold, total_prize_pool, verification_hash,
		       stage_data, created_at, updated_at
		FROM draws ` + whereClause + `
		ORDER BY scheduled_time DESC
		LIMIT $` + fmt.Sprintf("%d", argIndex) + ` OFFSET $` + fmt.Sprintf("%d", argIndex+1)

	args = append(args, perPage, offset)

	var draws []*models.Draw
	err = r.db.SelectContext(ctx, &draws, query, args...)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to list draws: %w", err)
	}

	span.SetAttributes(attribute.Int64("result.total_count", total))
	return draws, total, nil
}

func (r *drawRepository) ListCompletedPublic(ctx context.Context, gameID *uuid.UUID, gameCode string, latestOnly bool, startDate, endDate *time.Time, page, perPage int) ([]*models.Draw, int64, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.ListCompletedPublic")
	defer span.End()

	if page <= 0 {
		page = 1
	}
	if perPage <= 0 || perPage > 50 {
		perPage = 10
	}

	span.SetAttributes(
		attribute.Int("pagination.page", page),
		attribute.Int("pagination.per_page", perPage),
		attribute.Bool("filter.latest_only", latestOnly),
	)

	// Build WHERE conditions - only completed draws with winning numbers
	whereClauses := []string{"status = 'completed'", "cardinality(winning_numbers) > 0"}
	args := []interface{}{}
	argIndex := 1

	if gameID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("game_id = $%d", argIndex))
		args = append(args, *gameID)
		argIndex++
		span.SetAttributes(attribute.String("filter.game_id", gameID.String()))
	}

	if gameCode != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("game_code = $%d", argIndex))
		args = append(args, gameCode)
		argIndex++
		span.SetAttributes(attribute.String("filter.game_code", gameCode))
	}

	if startDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("scheduled_time >= $%d", argIndex))
		args = append(args, *startDate)
		argIndex++
		span.SetAttributes(attribute.String("filter.start_date", startDate.Format(time.RFC3339)))
	}

	if endDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("scheduled_time <= $%d", argIndex))
		args = append(args, *endDate)
		argIndex++
		span.SetAttributes(attribute.String("filter.end_date", endDate.Format(time.RFC3339)))
	}

	whereClause := "WHERE " + whereClauses[0]
	for _, clause := range whereClauses[1:] {
		whereClause += " AND " + clause
	}

	// If latest_only, we need to use a different query with DISTINCT ON
	if latestOnly {
		// Get the latest draw per game - Note: draws table doesn't have logo_url/brand_color
		// These would need to be added via migration or fetched separately if needed
		query := `
			SELECT DISTINCT ON (d.game_id)
			       d.id, d.game_id, d.draw_number, d.game_name, COALESCE(d.game_code, '') as game_code,
			       NULL as game_logo_url, NULL as game_brand_color,
			       d.game_schedule_id, d.draw_name, d.status, d.scheduled_time, d.executed_time,
			       d.winning_numbers, d.machine_numbers, d.nla_draw_reference, d.draw_location, d.nla_official_signature,
			       d.total_tickets_sold, d.total_prize_pool, d.verification_hash, d.stage_data,
			       d.created_at, d.updated_at
			FROM draws d
			` + whereClause + `
			ORDER BY d.game_id, d.scheduled_time DESC
		`

		var draws []*models.Draw
		err := r.db.SelectContext(ctx, &draws, query, args...)
		if err != nil {
			span.SetAttributes(attribute.String("error", err.Error()))
			return nil, 0, fmt.Errorf("failed to list latest completed draws: %w", err)
		}

		span.SetAttributes(
			attribute.Int("result.draws_count", len(draws)),
			attribute.Int64("result.total_count", int64(len(draws))),
		)

		return draws, int64(len(draws)), nil
	}

	// Count total records
	countQuery := "SELECT COUNT(*) FROM draws " + whereClause
	var total int64
	err := r.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to count completed draws: %w", err)
	}

	// Get paginated results
	offset := (page - 1) * perPage
	query := `
		SELECT id, game_id, draw_number, game_name, COALESCE(game_code, '') as game_code,
		       NULL as game_logo_url, NULL as game_brand_color,
		       game_schedule_id, draw_name, status, scheduled_time, executed_time,
		       winning_numbers, machine_numbers, nla_draw_reference, draw_location, nla_official_signature,
		       total_tickets_sold, total_prize_pool, verification_hash, stage_data,
		       created_at, updated_at
		FROM draws ` + whereClause + `
		ORDER BY scheduled_time DESC
		LIMIT $` + fmt.Sprintf("%d", argIndex) + ` OFFSET $` + fmt.Sprintf("%d", argIndex+1)

	args = append(args, perPage, offset)

	var draws []*models.Draw
	err = r.db.SelectContext(ctx, &draws, query, args...)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to list completed draws: %w", err)
	}

	span.SetAttributes(
		attribute.Int("result.draws_count", len(draws)),
		attribute.Int64("result.total_count", total),
	)

	return draws, total, nil
}

func (r *drawRepository) CreateSchedule(ctx context.Context, schedule *models.DrawSchedule) error {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.CreateSchedule")
	defer span.End()

	span.SetAttributes(
		attribute.String("schedule.game_id", schedule.GameID.String()),
		attribute.String("schedule.draw_name", schedule.DrawName),
		attribute.String("schedule.frequency", schedule.Frequency.String()),
	)

	query := `
		INSERT INTO draw_schedules (id, game_id, draw_name, scheduled_time, frequency, is_active, created_by, notes, created_at)
		VALUES (:id, :game_id, :draw_name, :scheduled_time, :frequency, :is_active, :created_by, :notes, :created_at)`

	// Set defaults
	if schedule.ID == uuid.Nil {
		schedule.ID = uuid.New()
	}
	if schedule.Frequency == "" {
		schedule.Frequency = models.FrequencyOneTime
	}
	schedule.CreatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, schedule)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return fmt.Errorf("failed to create draw schedule: %w", err)
	}

	return nil
}

func (r *drawRepository) GetScheduleByID(ctx context.Context, id uuid.UUID) (*models.DrawSchedule, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.GetScheduleByID")
	defer span.End()

	span.SetAttributes(attribute.String("schedule.id", id.String()))

	query := `
		SELECT id, game_id, draw_name, scheduled_time, frequency, is_active, created_by, notes, created_at
		FROM draw_schedules 
		WHERE id = $1`

	var schedule models.DrawSchedule
	err := r.db.GetContext(ctx, &schedule, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.String("result", "not_found"))
			return nil, fmt.Errorf("draw schedule not found")
		}
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get draw schedule: %w", err)
	}

	return &schedule, nil
}

func (r *drawRepository) ListSchedules(ctx context.Context, gameID *uuid.UUID, startDate, endDate *time.Time, activeOnly bool) ([]*models.DrawSchedule, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.ListSchedules")
	defer span.End()

	span.SetAttributes(attribute.Bool("filter.active_only", activeOnly))

	// Build WHERE conditions
	whereClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	if gameID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("game_id = $%d", argIndex))
		args = append(args, *gameID)
		argIndex++
		span.SetAttributes(attribute.String("filter.game_id", gameID.String()))
	}

	if activeOnly {
		whereClauses = append(whereClauses, "is_active = true")
	}

	if startDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("scheduled_time >= $%d", argIndex))
		args = append(args, *startDate)
		argIndex++
	}

	if endDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("scheduled_time <= $%d", argIndex))
		args = append(args, *endDate)
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + whereClauses[0]
		for _, clause := range whereClauses[1:] {
			whereClause += " AND " + clause
		}
	}

	query := `
		SELECT id, game_id, draw_name, scheduled_time, frequency, is_active, created_by, notes, created_at
		FROM draw_schedules ` + whereClause + `
		ORDER BY scheduled_time ASC`

	var schedules []*models.DrawSchedule
	err := r.db.SelectContext(ctx, &schedules, query, args...)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to list draw schedules: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(schedules)))
	return schedules, nil
}

func (r *drawRepository) UpdateSchedule(ctx context.Context, schedule *models.DrawSchedule) error {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.UpdateSchedule")
	defer span.End()

	span.SetAttributes(
		attribute.String("schedule.id", schedule.ID.String()),
		attribute.Bool("schedule.is_active", schedule.IsActive),
	)

	query := `
		UPDATE draw_schedules 
		SET scheduled_time = :scheduled_time, frequency = :frequency, is_active = :is_active, notes = :notes
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, schedule)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return fmt.Errorf("failed to update draw schedule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		span.SetAttributes(attribute.String("result", "not_found"))
		return fmt.Errorf("draw schedule not found")
	}

	return nil
}

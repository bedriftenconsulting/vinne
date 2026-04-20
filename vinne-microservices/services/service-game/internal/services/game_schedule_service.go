package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"github.com/randco/randco-microservices/services/service-game/internal/repositories"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// GameScheduleService defines the interface for game schedule business logic
type GameScheduleService interface {
	ScheduleGame(ctx context.Context, gameID uuid.UUID, start, end, draw time.Time, frequency string) (*models.GameSchedule, error)
	GetGameSchedule(ctx context.Context, gameID uuid.UUID) ([]*models.GameSchedule, error)
	GetScheduledGameByID(ctx context.Context, scheduleID uuid.UUID) (*models.GameSchedule, error)
	ValidateScheduleForTicketSales(ctx context.Context, scheduleID uuid.UUID) error
	UpdateSchedule(ctx context.Context, schedule *models.GameSchedule) error
	UpdateScheduledGame(ctx context.Context, scheduleID uuid.UUID, req *models.UpdateGameScheduleRequest) (*models.GameSchedule, error)
	CancelSchedule(ctx context.Context, scheduleID uuid.UUID) error
	GetActiveSchedules(ctx context.Context) ([]*models.GameSchedule, error)
	GetUpcomingSchedules(ctx context.Context, limit int) ([]*models.GameSchedule, error)

	// Weekly scheduling functionality
	GenerateWeeklySchedule(ctx context.Context, weekStart time.Time) ([]*models.GameSchedule, error)
	GetWeeklySchedule(ctx context.Context, weekStart time.Time) ([]*models.GameSchedule, error)
	ClearWeeklySchedule(ctx context.Context, weekStart time.Time) error
}

// gameScheduleService implements GameScheduleService interface
type gameScheduleService struct {
	scheduleRepo repositories.GameScheduleRepository
	gameRepo     repositories.GameRepository
}

// NewGameScheduleService creates a new instance of GameScheduleService
func NewGameScheduleService(
	scheduleRepo repositories.GameScheduleRepository,
	gameRepo repositories.GameRepository,
) GameScheduleService {
	return &gameScheduleService{
		scheduleRepo: scheduleRepo,
		gameRepo:     gameRepo,
	}
}

// ScheduleGame creates a schedule for a game
func (s *gameScheduleService) ScheduleGame(ctx context.Context, gameID uuid.UUID, start, end, draw time.Time, frequency string) (*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_schedule.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("game.id", gameID.String()),
		attribute.String("frequency", string(frequency)),
		attribute.String("start_time", start.Format(time.RFC3339)),
		attribute.String("draw_time", draw.Format(time.RFC3339)),
	)

	// Verify game exists
	game, err := s.gameRepo.GetByID(ctx, gameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return nil, fmt.Errorf("game not found: %w", err)
	}

	// Validate game is draft or active
	if game.Status != "DRAFT" && game.Status != "ACTIVE" {
		err := fmt.Errorf("game must be draft or active to schedule, current status: %s", game.Status)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid game status")
		return nil, err
	}

	// Validate schedule times
	if err := s.validateScheduleTimes(start, end, draw); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid schedule times")
		return nil, err
	}

	// Create schedule
	schedule := &models.GameSchedule{
		GameID:         gameID,
		GameName:       &game.Name,
		ScheduledStart: start,
		ScheduledEnd:   end,
		ScheduledDraw:  draw,
		Frequency:      models.DrawFrequency(frequency),
		IsActive:       true,
		Status:         models.ScheduleStatusScheduled,
	}

	if err := s.scheduleRepo.Create(ctx, schedule); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create schedule")
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}

	// Update game draw time if not set
	if game.DrawTime == nil {
		game.DrawTime = &draw
		if err := s.gameRepo.Update(ctx, game); err != nil {
			// Log error but don't fail the schedule creation
			span.RecordError(err)
		}
	}

	span.SetAttributes(attribute.String("schedule.id", schedule.ID.String()))
	return schedule, nil
}

// GetGameSchedule retrieves all schedules for a game
func (s *gameScheduleService) GetGameSchedule(ctx context.Context, gameID uuid.UUID) ([]*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_schedule.get")
	defer span.End()

	span.SetAttributes(attribute.String("game.id", gameID.String()))

	schedules, err := s.scheduleRepo.GetByGameID(ctx, gameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get schedules")
		return nil, fmt.Errorf("failed to get schedules: %w", err)
	}

	span.SetAttributes(attribute.Int("schedules.count", len(schedules)))
	return schedules, nil
}

// GetScheduledGameByID retrieves a scheduled game by its ID
func (s *gameScheduleService) GetScheduledGameByID(ctx context.Context, scheduleID uuid.UUID) (*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_schedule.get_by_id")
	defer span.End()

	span.SetAttributes(attribute.String("schedule.id", scheduleID.String()))

	schedule, err := s.scheduleRepo.GetByID(ctx, scheduleID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "schedule not found")
		return nil, fmt.Errorf("schedule not found: %w", err)
	}

	return schedule, nil
}

// ValidateScheduleForTicketSales validates that a schedule is in a valid state for ticket sales
func (s *gameScheduleService) ValidateScheduleForTicketSales(ctx context.Context, scheduleID uuid.UUID) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_schedule.validate_for_sales")
	defer span.End()

	span.SetAttributes(attribute.String("schedule.id", scheduleID.String()))

	schedule, err := s.scheduleRepo.GetByID(ctx, scheduleID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "schedule not found")
		return fmt.Errorf("schedule not found: %w", err)
	}

	// Check if schedule is active
	if !schedule.IsActive {
		err := fmt.Errorf("schedule is not active")
		span.RecordError(err)
		span.SetStatus(codes.Error, "schedule not active")
		return err
	}

	// Check schedule status - must be SCHEDULED or IN_PROGRESS
	if schedule.Status != models.ScheduleStatusScheduled && schedule.Status != models.ScheduleStatusInProgress {
		err := fmt.Errorf("schedule status is %s, cannot accept ticket sales", schedule.Status)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid schedule status")
		return err
	}

	// Check if sales period has ended
	now := time.Now()
	if now.After(schedule.ScheduledEnd) {
		err := fmt.Errorf("ticket sales period has ended")
		span.RecordError(err)
		span.SetStatus(codes.Error, "sales period ended")
		return err
	}

	// Check if sales period has started
	if now.Before(schedule.ScheduledStart) {
		err := fmt.Errorf("ticket sales period has not started yet")
		span.RecordError(err)
		span.SetStatus(codes.Error, "sales period not started")
		return err
	}

	span.SetStatus(codes.Ok, "schedule valid for ticket sales")
	return nil
}

// UpdateSchedule updates an existing schedule
func (s *gameScheduleService) UpdateSchedule(ctx context.Context, schedule *models.GameSchedule) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_schedule.update")
	defer span.End()

	span.SetAttributes(
		attribute.String("schedule.id", schedule.ID.String()),
		attribute.String("game.id", schedule.GameID.String()),
	)

	// Verify game exists
	game, err := s.gameRepo.GetByID(ctx, schedule.GameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return fmt.Errorf("game not found: %w", err)
	}

	// Validate game status allows schedule updates
	if game.Status == "TERMINATED" {
		err := fmt.Errorf("cannot update schedule for terminated game")
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid game status")
		return err
	}

	// Validate schedule times
	if err := s.validateScheduleTimes(schedule.ScheduledStart, schedule.ScheduledEnd, schedule.ScheduledDraw); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid schedule times")
		return err
	}

	// Update schedule
	if err := s.scheduleRepo.Update(ctx, schedule); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update schedule")
		return fmt.Errorf("failed to update schedule: %w", err)
	}

	return nil
}

// UpdateScheduledGame updates specific fields of a scheduled game
func (s *gameScheduleService) UpdateScheduledGame(ctx context.Context, scheduleID uuid.UUID, req *models.UpdateGameScheduleRequest) (*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_schedule.update_scheduled_game")
	defer span.End()

	span.SetAttributes(attribute.String("schedule.id", scheduleID.String()))

	// Get existing schedule
	schedule, err := s.scheduleRepo.GetByID(ctx, scheduleID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "schedule not found")
		return nil, fmt.Errorf("schedule not found: %w", err)
	}

	// Apply updates
	if req.ScheduledEnd != nil {
		schedule.ScheduledEnd = *req.ScheduledEnd
	}
	if req.ScheduledDraw != nil {
		schedule.ScheduledDraw = *req.ScheduledDraw
	}
	if req.Status != nil {
		schedule.Status = *req.Status
	}
	if req.IsActive != nil {
		schedule.IsActive = *req.IsActive
	}
	if req.Notes != nil {
		schedule.Notes = req.Notes
	}

	// Validate schedule times
	if err := s.validateScheduleTimes(schedule.ScheduledStart, schedule.ScheduledEnd, schedule.ScheduledDraw); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid schedule times")
		return nil, err
	}

	// Verify game exists and is not terminated
	game, err := s.gameRepo.GetByID(ctx, schedule.GameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return nil, fmt.Errorf("game not found: %w", err)
	}

	if game.Status == "TERMINATED" {
		err := fmt.Errorf("cannot update schedule for terminated game")
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid game status")
		return nil, err
	}

	// Update schedule in database
	if err := s.scheduleRepo.Update(ctx, schedule); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update schedule")
		return nil, fmt.Errorf("failed to update schedule: %w", err)
	}

	span.SetStatus(codes.Ok, "schedule updated successfully")
	return schedule, nil
}

// CancelSchedule cancels a game schedule
func (s *gameScheduleService) CancelSchedule(ctx context.Context, scheduleID uuid.UUID) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_schedule.cancel")
	defer span.End()

	span.SetAttributes(attribute.String("schedule.id", scheduleID.String()))

	// Get schedule
	schedule, err := s.scheduleRepo.GetByID(ctx, scheduleID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "schedule not found")
		return fmt.Errorf("schedule not found: %w", err)
	}

	// Check if schedule has already started
	if time.Now().After(schedule.ScheduledStart) {
		err := fmt.Errorf("cannot cancel schedule that has already started")
		span.RecordError(err)
		span.SetStatus(codes.Error, "schedule already started")
		return err
	}

	// Mark schedule as inactive
	schedule.IsActive = false
	if err := s.scheduleRepo.Update(ctx, schedule); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to cancel schedule")
		return fmt.Errorf("failed to cancel schedule: %w", err)
	}

	span.SetStatus(codes.Ok, "schedule cancelled successfully")
	return nil
}

// GetActiveSchedules retrieves all active schedules
func (s *gameScheduleService) GetActiveSchedules(ctx context.Context) ([]*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_schedule.get_active")
	defer span.End()

	schedules, err := s.scheduleRepo.GetActiveSchedules(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get active schedules")
		return nil, fmt.Errorf("failed to get active schedules: %w", err)
	}

	span.SetAttributes(attribute.Int("schedules.active_count", len(schedules)))
	return schedules, nil
}

// GetUpcomingSchedules retrieves upcoming schedules
func (s *gameScheduleService) GetUpcomingSchedules(ctx context.Context, limit int) ([]*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_schedule.get_upcoming")
	defer span.End()

	span.SetAttributes(attribute.Int("limit", limit))

	schedules, err := s.scheduleRepo.GetUpcomingSchedules(ctx, limit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get upcoming schedules")
		return nil, fmt.Errorf("failed to get upcoming schedules: %w", err)
	}

	span.SetAttributes(attribute.Int("schedules.upcoming_count", len(schedules)))
	return schedules, nil
}

// validateScheduleTimes validates schedule times
func (s *gameScheduleService) validateScheduleTimes(start, end, draw time.Time) error {
	// Start must be before end
	if start.After(end) || start.Equal(end) {
		return fmt.Errorf("start time must be before end time")
	}

	// End must be before or equal to draw
	if end.After(draw) {
		return fmt.Errorf("end time (sales cutoff) must be before or at draw time")
	}

	// Draw must be after end
	if draw.Before(end) {
		return fmt.Errorf("draw time must be after or at end time (sales cutoff)")
	}

	// Start can be in the past for weekly schedules (week start)
	// Only validate that the draw time is in the future
	if draw.Before(time.Now()) {
		return fmt.Errorf("draw time must be in the future")
	}

	// Reasonable time limits (e.g., not more than 1 year in the future)
	maxFutureTime := time.Now().AddDate(1, 0, 0)
	if start.After(maxFutureTime) {
		return fmt.Errorf("start time cannot be more than 1 year in the future")
	}

	return nil
}

// GenerateWeeklySchedule generates schedules for all active games for a given week
func (s *gameScheduleService) GenerateWeeklySchedule(ctx context.Context, weekStart time.Time) ([]*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_schedule.generate_weekly")
	defer span.End()

	fmt.Printf("[GameScheduleService] GenerateWeeklySchedule called, original_week_start=%s\n", weekStart.Format("2006-01-02"))

	// Ensure weekStart is a Sunday
	if weekStart.Weekday() != time.Sunday {
		weekStart = weekStart.AddDate(0, 0, -(int(weekStart.Weekday())))
		fmt.Printf("[GameScheduleService] Adjusted week_start to Sunday: %s\n", weekStart.Format("2006-01-02"))
	}
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())
	weekEnd := weekStart.AddDate(0, 0, 7).Add(-time.Second)

	fmt.Printf("[GameScheduleService] Week range calculated: week_start=%s, week_end=%s\n", weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02"))

	span.SetAttributes(
		attribute.String("week_start", weekStart.Format(time.RFC3339)),
		attribute.String("week_end", weekEnd.Format(time.RFC3339)),
	)

	// Clear only unplayed (scheduled) games for the week to prevent duplicates
	// This preserves completed or in-progress games
	fmt.Println("[GameScheduleService] Clearing existing unplayed schedules in week range")
	if err := s.scheduleRepo.DeleteUnplayedSchedulesInTimeRange(ctx, weekStart, weekEnd); err != nil {
		fmt.Printf("[GameScheduleService] ERROR: Failed to clear existing unplayed schedules: %v\n", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to clear existing unplayed schedules")
		return nil, fmt.Errorf("failed to clear existing unplayed schedules: %w", err)
	}

	// Get all active/approved games
	fmt.Println("[GameScheduleService] Fetching active games")
	games, err := s.gameRepo.GetActiveGames(ctx)
	if err != nil {
		fmt.Printf("[GameScheduleService] ERROR: Failed to get active games: %v\n", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get active games")
		return nil, fmt.Errorf("failed to get active games: %w", err)
	}

	fmt.Printf("[GameScheduleService] Active games retrieved: count=%d\n", len(games))
	for i, game := range games {
		fmt.Printf("[GameScheduleService] Active game #%d: id=%s, code=%s, name=%s, frequency=%s, draw_days=%v, draw_time=%v\n",
			i, game.ID, game.Code, game.Name, game.DrawFrequency, game.DrawDays, game.DrawTime)
	}

	var allSchedules []*models.GameSchedule

	for _, game := range games {
		fmt.Printf("[GameScheduleService] Generating schedules for game: id=%s, code=%s, frequency=%s\n", game.ID, game.Code, game.DrawFrequency)
		schedules, err := s.generateGameSchedulesForWeek(ctx, game, weekStart)
		if err != nil {
			fmt.Printf("[GameScheduleService] ERROR: Failed to generate schedules for game %s: %v\n", game.Code, err)
			span.RecordError(err)
			// Log error but continue with other games
			continue
		}
		fmt.Printf("[GameScheduleService] Generated %d schedules for game %s\n", len(schedules), game.Code)
		allSchedules = append(allSchedules, schedules...)
	}

	fmt.Printf("[GameScheduleService] Total schedules generated before saving: count=%d\n", len(allSchedules))

	// Save all schedules in a batch
	savedCount := 0
	for _, schedule := range allSchedules {
		if err := s.scheduleRepo.Create(ctx, schedule); err != nil {
			fmt.Printf("[GameScheduleService] ERROR: Failed to save schedule for game %s at %s: %v\n", schedule.GameID, schedule.ScheduledDraw, err)
			span.RecordError(err)
			// Log error but continue
		} else {
			savedCount++
		}
	}

	fmt.Printf("[GameScheduleService] Schedules saved successfully: saved_count=%d, total_generated=%d\n", savedCount, len(allSchedules))

	span.SetAttributes(attribute.Int("schedules.created", len(allSchedules)))
	return allSchedules, nil
}

// generateGameSchedulesForWeek generates schedules for a specific game within the current 7-day window.
// All frequencies are scoped to the current week only — no monthly calendar math, no look-ahead.
// - daily:              7 draws, one per day
// - weekly/bi_weekly:   1 draw on the configured draw day(s) that fall in the week
// - monthly/special:    1 draw — use the configured draw day or default to Saturday
func (s *gameScheduleService) generateGameSchedulesForWeek(ctx context.Context, game *models.Game, weekStart time.Time) ([]*models.GameSchedule, error) {
	weekEnd := weekStart.AddDate(0, 0, 7)
	_ = weekEnd // used implicitly via day iteration

	// For future weeks (not the current week), only generate special games
	// Daily/weekly games should only be scheduled for the current week
	now := time.Now().UTC()
	currentWeekStart := now.AddDate(0, 0, -int(now.Weekday()))
	currentWeekStart = time.Date(currentWeekStart.Year(), currentWeekStart.Month(), currentWeekStart.Day(), 0, 0, 0, 0, time.UTC)
	isFutureWeek := weekStart.After(currentWeekStart.AddDate(0, 0, 6))

	freq := strings.ToUpper(game.DrawFrequency)
	if isFutureWeek && freq != "SPECIAL" && freq != "MONTHLY" {
		fmt.Printf("[GameScheduleService] Skipping non-special game %s for future week %s\n", game.Code, weekStart.Format("2006-01-02"))
		return nil, nil
	}

	dayMap := map[string]time.Weekday{
		"Wednesday": time.Wednesday, "Thursday": time.Thursday,
		"Friday": time.Friday, "Saturday": time.Saturday,
	}

	// Helper: build a schedule entry for a given day
	makeSchedule := func(day time.Time, freq models.DrawFrequency) *models.GameSchedule {
		var drawTime time.Time
		if game.DrawTime != nil {
			drawTime = time.Date(day.Year(), day.Month(), day.Day(),
				game.DrawTime.Hour(), game.DrawTime.Minute(), 0, 0, day.Location())
		} else {
			drawTime = time.Date(day.Year(), day.Month(), day.Day(), 20, 0, 0, 0, day.Location())
		}
		salesEnd := drawTime.Add(-time.Duration(game.SalesCutoffMinutes) * time.Minute)
		note := fmt.Sprintf("%s draw for %s", string(freq), game.Name)
		return &models.GameSchedule{
			GameID:         game.ID,
			GameName:       &game.Name,
			ScheduledStart: weekStart,
			ScheduledEnd:   salesEnd,
			ScheduledDraw:  drawTime,
			Frequency:      freq,
			IsActive:       true,
			Status:         models.ScheduleStatusScheduled,
			Notes:          &note,
			LogoURL:        game.LogoURL,
			BrandColor:     game.BrandColor,
		}
	}

	// freq already declared above — reuse it (lowercase for switch)
	switch strings.ToLower(freq) {
	case "daily":
		// One draw per day for all 7 days of the week
		var schedules []*models.GameSchedule
		for i := 0; i < 7; i++ {
			day := weekStart.AddDate(0, 0, i)
			schedules = append(schedules, makeSchedule(day, models.DrawFrequencyDaily))
		}
		fmt.Printf("[GameScheduleService] daily: game=%s, 7 draws\n", game.Code)
		return schedules, nil

	case "weekly", "bi_weekly":
		// One draw per configured draw day that falls in this week
		var schedules []*models.GameSchedule
		if len(game.DrawDays) == 0 {
			// No draw days configured — default to Friday
			for i := 0; i < 7; i++ {
				day := weekStart.AddDate(0, 0, i)
				if day.Weekday() == time.Friday {
					schedules = append(schedules, makeSchedule(day, models.DrawFrequencyWeekly))
					break
				}
			}
		} else {
			for i := 0; i < 7; i++ {
				day := weekStart.AddDate(0, 0, i)
				for _, d := range game.DrawDays {
					if wd, ok := dayMap[d]; ok && day.Weekday() == wd {
						schedules = append(schedules, makeSchedule(day, models.DrawFrequencyWeekly))
					}
				}
			}
		}
		fmt.Printf("[GameScheduleService] weekly/bi-weekly: game=%s, %d draws\n", game.Code, len(schedules))
		return schedules, nil

	case "special":
		// Special draws use the game's draw_date (stored as end_date in DB)
		if game.EndDate == nil {
			fmt.Printf("[GameScheduleService] SPECIAL game %s has no draw_date — skipping\n", game.Code)
			return nil, nil
		}

		weekEnd := weekStart.AddDate(0, 0, 7)

		// Determine draw time: use game.DrawTime on the EndDate day
		var drawTime time.Time
		if game.DrawTime != nil {
			drawTime = time.Date(game.EndDate.Year(), game.EndDate.Month(), game.EndDate.Day(),
				game.DrawTime.Hour(), game.DrawTime.Minute(), 0, 0, game.EndDate.Location())
		} else {
			drawTime = time.Date(game.EndDate.Year(), game.EndDate.Month(), game.EndDate.Day(),
				20, 0, 0, 0, game.EndDate.Location())
		}

		// Only schedule if the draw falls within this week
		if drawTime.Before(weekStart) || !drawTime.Before(weekEnd) {
			fmt.Printf("[GameScheduleService] SPECIAL game %s draw on %s is outside week %s — skipping\n",
				game.Code, drawTime.Format("2006-01-02"), weekStart.Format("2006-01-02"))
			return nil, nil
		}

		startDay := game.EndDate
		if game.StartDate != nil {
			startDay = game.StartDate
		}
		salesStart := time.Date(startDay.Year(), startDay.Month(), startDay.Day(),
			0, 0, 0, 0, startDay.Location())
		salesEnd := drawTime.Add(-time.Duration(game.SalesCutoffMinutes) * time.Minute)

		note := fmt.Sprintf("Special draw for %s", game.Name)
		schedule := &models.GameSchedule{
			GameID:         game.ID,
			GameName:       &game.Name,
			ScheduledStart: salesStart,
			ScheduledEnd:   salesEnd,
			ScheduledDraw:  drawTime,
			Frequency:      models.DrawFrequencySpecial,
			IsActive:       true,
			Status:         models.ScheduleStatusScheduled,
			Notes:          &note,
			LogoURL:        game.LogoURL,
			BrandColor:     game.BrandColor,
		}
		fmt.Printf("[GameScheduleService] SPECIAL game %s: sales %s → %s, draw %s\n",
			game.Code, salesStart.Format("2006-01-02"), salesEnd.Format("2006-01-02 15:04"), drawTime.Format("2006-01-02 15:04"))
		return []*models.GameSchedule{schedule}, nil

	default:
		// monthly — one draw this week using configured draw day or Saturday
		drawDay := weekStart.AddDate(0, 0, 6) // default Saturday
		if len(game.DrawDays) > 0 {
			for i := 0; i < 7; i++ {
				candidate := weekStart.AddDate(0, 0, i)
				for _, d := range game.DrawDays {
					if wd, ok := dayMap[d]; ok && candidate.Weekday() == wd {
						drawDay = candidate
						goto foundDay
					}
				}
			}
		foundDay:
		}
		fmt.Printf("[GameScheduleService] %s: game=%s, 1 draw on %s\n", freq, game.Code, drawDay.Format("2006-01-02"))
		return []*models.GameSchedule{makeSchedule(drawDay, models.DrawFrequencyWeekly)}, nil
	}
}

// generateDailySchedules creates daily schedules for a game
func (s *gameScheduleService) generateDailySchedules(game *models.Game, weekStart time.Time) []*models.GameSchedule {
	var schedules []*models.GameSchedule

	fmt.Printf("[GameScheduleService] generateDailySchedules: game=%s, sales_cutoff_minutes=%d\n", game.Code, game.SalesCutoffMinutes)

	for i := 0; i < 7; i++ {
		day := weekStart.AddDate(0, 0, i)

		// Parse game times
		var drawTime time.Time
		if game.DrawTime != nil {
			drawTime = time.Date(day.Year(), day.Month(), day.Day(),
				game.DrawTime.Hour(), game.DrawTime.Minute(), game.DrawTime.Second(), 0, day.Location())
		} else {
			drawTime = time.Date(day.Year(), day.Month(), day.Day(), 20, 0, 0, 0, day.Location()) // Default 8 PM
		}

		// Calculate sales period
		// Sales start at the beginning of the week to allow advance ticket purchases
		salesStart := weekStart

		salesEnd := drawTime.Add(-time.Duration(game.SalesCutoffMinutes) * time.Minute)

		fmt.Printf("[GameScheduleService] Daily schedule day %d: game=%s, draw_time=%s, cutoff_minutes=%d, sales_end=%s\n",
			i, game.Code, drawTime.Format("2006-01-02 15:04"), game.SalesCutoffMinutes, salesEnd.Format("2006-01-02 15:04"))

		schedule := &models.GameSchedule{
			GameID:         game.ID,
			GameName:       &game.Name,
			ScheduledStart: salesStart,
			ScheduledEnd:   salesEnd,
			ScheduledDraw:  drawTime,
			Frequency:      models.DrawFrequencyDaily,
			IsActive:       true,
			Status:         models.ScheduleStatusScheduled,
			Notes:          &[]string{fmt.Sprintf("Daily draw for %s", game.Name)}[0],
			LogoURL:        game.LogoURL,
			BrandColor:     game.BrandColor,
		}

		schedules = append(schedules, schedule)
	}

	return schedules
}

// generateWeeklySchedules creates weekly schedules based on game's draw days
func (s *gameScheduleService) generateWeeklySchedules(game *models.Game, weekStart time.Time) []*models.GameSchedule {
	var schedules []*models.GameSchedule

	fmt.Printf("[GameScheduleService] generateWeeklySchedules called: game_id=%s, game_code=%s, draw_days=%v, draw_days_count=%d\n",
		game.ID, game.Code, game.DrawDays, len(game.DrawDays))

	if len(game.DrawDays) == 0 {
		fmt.Printf("[GameScheduleService] WARNING: No draw days configured for weekly game %s\n", game.Code)
		return schedules
	}

	for _, dayStr := range game.DrawDays {
		fmt.Printf("[GameScheduleService] Processing draw day: game=%s, day_string=%s\n", game.Code, dayStr)
		weekday := parseWeekday(dayStr)
		if weekday == -1 {
			fmt.Printf("[GameScheduleService] ERROR: Failed to parse weekday: game=%s, day_string=%s\n", game.Code, dayStr)
			continue
		}

		fmt.Printf("[GameScheduleService] Parsed weekday successfully: game=%s, day_string=%s, weekday=%d\n", game.Code, dayStr, weekday)

		// Calculate the actual day
		daysFromSunday := int(weekday)
		if weekday == time.Sunday {
			daysFromSunday = 0
		}

		day := weekStart.AddDate(0, 0, daysFromSunday)
		fmt.Printf("[GameScheduleService] Calculated schedule day: game=%s, day_string=%s, schedule_day=%s\n",
			game.Code, dayStr, day.Format("2006-01-02"))

		var drawTime time.Time
		if game.DrawTime != nil {
			drawTime = time.Date(day.Year(), day.Month(), day.Day(),
				game.DrawTime.Hour(), game.DrawTime.Minute(), game.DrawTime.Second(), 0, day.Location())
		} else {
			drawTime = time.Date(day.Year(), day.Month(), day.Day(), 20, 0, 0, 0, day.Location())
		}

		// Calculate sales period
		// Sales start at the beginning of the week to allow advance ticket purchases
		salesStart := weekStart

		salesEnd := drawTime.Add(-time.Duration(game.SalesCutoffMinutes) * time.Minute)

		fmt.Printf("[GameScheduleService] Weekly schedule: game=%s, day=%s, draw_time=%s, cutoff_minutes=%d, sales_end=%s\n",
			game.Code, dayStr, drawTime.Format("2006-01-02 15:04"), game.SalesCutoffMinutes, salesEnd.Format("2006-01-02 15:04"))

		schedule := &models.GameSchedule{
			GameID:         game.ID,
			GameName:       &game.Name,
			ScheduledStart: salesStart,
			ScheduledEnd:   salesEnd,
			ScheduledDraw:  drawTime,
			Frequency:      models.DrawFrequencyWeekly,
			IsActive:       true,
			Status:         models.ScheduleStatusScheduled,
			Notes:          &[]string{fmt.Sprintf("Weekly draw for %s on %s", game.Name, dayStr)}[0],
			LogoURL:        game.LogoURL,
			BrandColor:     game.BrandColor,
		}

		schedules = append(schedules, schedule)
	}

	return schedules
}

// generateBiWeeklySchedules creates bi-weekly schedules
func (s *gameScheduleService) generateBiWeeklySchedules(game *models.Game, weekStart time.Time) []*models.GameSchedule {
	// For simplicity, treat bi-weekly as once per week for now
	// In a real implementation, you'd need to track which week of the bi-weekly cycle this is
	return s.generateWeeklySchedules(game, weekStart)
}

// generateMonthlyOrSpecialSchedule creates a single draw for a monthly or special game.
// For special/monthly, only ONE schedule is created per calendar month — if one already exists, skip.
func (s *gameScheduleService) generateMonthlyOrSpecialSchedule(ctx context.Context, game *models.Game, weekStart time.Time) []*models.GameSchedule {
	fmt.Printf("[GameScheduleService] generateMonthlyOrSpecialSchedule: game=%s, frequency=%s\n", game.Code, game.DrawFrequency)

	// Check if a schedule already exists anywhere in this calendar month
	monthStart := time.Date(weekStart.Year(), weekStart.Month(), 1, 0, 0, 0, 0, weekStart.Location())
	monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Second)

	exists, err := s.scheduleRepo.HasScheduleForGameInRange(ctx, game.ID, monthStart, monthEnd)
	if err != nil {
		fmt.Printf("[GameScheduleService] WARNING: could not check existing schedules for game %s: %v\n", game.Code, err)
	}
	if exists {
		fmt.Printf("[GameScheduleService] Skipping game %s — already has a schedule in %s\n", game.Code, monthStart.Format("January 2006"))
		return nil
	}

	// Determine draw day: use first configured draw day if available, else last Saturday of month
	var drawDay time.Time
	if len(game.DrawDays) > 0 {
		weekday := parseWeekday(game.DrawDays[0])
		if weekday != -1 {
			daysFromSunday := int(weekday)
			drawDay = weekStart.AddDate(0, 0, daysFromSunday)
		}
	}
	if drawDay.IsZero() {
		// Use last Saturday of the month
		drawDay = monthEnd
		for drawDay.Weekday() != time.Saturday {
			drawDay = drawDay.AddDate(0, 0, -1)
		}
	}

	var drawTime time.Time
	if game.DrawTime != nil {
		drawTime = time.Date(drawDay.Year(), drawDay.Month(), drawDay.Day(),
			game.DrawTime.Hour(), game.DrawTime.Minute(), game.DrawTime.Second(), 0, drawDay.Location())
	} else {
		drawTime = time.Date(drawDay.Year(), drawDay.Month(), drawDay.Day(), 20, 0, 0, 0, drawDay.Location())
	}

	salesStart := monthStart
	salesEnd := drawTime.Add(-time.Duration(game.SalesCutoffMinutes) * time.Minute)

	freq := models.DrawFrequencyMonthly
	if game.DrawFrequency == "SPECIAL" || game.DrawFrequency == "special" {
		freq = models.DrawFrequencySpecial
	}

	note := fmt.Sprintf("%s draw for %s", string(freq), game.Name)
	schedule := &models.GameSchedule{
		GameID:         game.ID,
		GameName:       &game.Name,
		ScheduledStart: salesStart,
		ScheduledEnd:   salesEnd,
		ScheduledDraw:  drawTime,
		Frequency:      freq,
		IsActive:       true,
		Status:         models.ScheduleStatusScheduled,
		Notes:          &note,
		LogoURL:        game.LogoURL,
		BrandColor:     game.BrandColor,
	}

	fmt.Printf("[GameScheduleService] Monthly/Special schedule: game=%s, draw_time=%s\n", game.Code, drawTime.Format("2006-01-02 15:04"))
	return []*models.GameSchedule{schedule}
}

// GetWeeklySchedule retrieves all schedules for a specific week
func (s *gameScheduleService) GetWeeklySchedule(ctx context.Context, weekStart time.Time) ([]*models.GameSchedule, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_schedule.get_weekly")
	defer span.End()

	// Ensure weekStart is a Sunday
	if weekStart.Weekday() != time.Sunday {
		weekStart = weekStart.AddDate(0, 0, -(int(weekStart.Weekday())))
	}
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())
	weekEnd := weekStart.AddDate(0, 0, 7).Add(-time.Second)

	span.SetAttributes(
		attribute.String("week_start", weekStart.Format(time.RFC3339)),
		attribute.String("week_end", weekEnd.Format(time.RFC3339)),
	)

	schedules, err := s.scheduleRepo.GetSchedulesInTimeRange(ctx, weekStart, weekEnd)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get weekly schedules")
		return nil, fmt.Errorf("failed to get weekly schedules: %w", err)
	}

	span.SetAttributes(attribute.Int("schedules.count", len(schedules)))

	if len(schedules) == 0 {
		return schedules, nil
	}

	gameIDs := make([]uuid.UUID, 0, len(schedules))
	for _, sc := range schedules {
		gameIDs = append(gameIDs, sc.GameID)
	}

	btMap, err := s.gameRepo.GetEnabledBetTypesByGameIDs(ctx, gameIDs)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to load bet types: %w", err)
	}

	for _, sc := range schedules {
		if bts, ok := btMap[sc.GameID]; ok {
			sc.BetTypes = bts
		}
	}

	return schedules, nil
}

// ClearWeeklySchedule removes all schedules for a specific week
func (s *gameScheduleService) ClearWeeklySchedule(ctx context.Context, weekStart time.Time) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_schedule.clear_weekly")
	defer span.End()

	// Ensure weekStart is a Sunday
	if weekStart.Weekday() != time.Sunday {
		weekStart = weekStart.AddDate(0, 0, -(int(weekStart.Weekday())))
	}
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())
	weekEnd := weekStart.AddDate(0, 0, 7).Add(-time.Second)

	span.SetAttributes(
		attribute.String("week_start", weekStart.Format(time.RFC3339)),
		attribute.String("week_end", weekEnd.Format(time.RFC3339)),
	)

	if err := s.scheduleRepo.DeleteSchedulesInTimeRange(ctx, weekStart, weekEnd); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to clear weekly schedules")
		return fmt.Errorf("failed to clear weekly schedules: %w", err)
	}

	return nil
}

// Helper functions

// parseWeekday converts day string to time.Weekday
func parseWeekday(dayStr string) time.Weekday {
	switch dayStr {
	case "sunday", "Sunday", "SUNDAY", "sun", "Sun", "SUN":
		return time.Sunday
	case "monday", "Monday", "MONDAY", "mon", "Mon", "MON":
		return time.Monday
	case "tuesday", "Tuesday", "TUESDAY", "tue", "Tue", "TUE":
		return time.Tuesday
	case "wednesday", "Wednesday", "WEDNESDAY", "wed", "Wed", "WED":
		return time.Wednesday
	case "thursday", "Thursday", "THURSDAY", "thu", "Thu", "THU":
		return time.Thursday
	case "friday", "Friday", "FRIDAY", "fri", "Fri", "FRI":
		return time.Friday
	case "saturday", "Saturday", "SATURDAY", "sat", "Sat", "SAT":
		return time.Saturday
	default:
		return -1
	}
}

package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	notificationv1 "github.com/randco/randco-microservices/proto/notification/v1"
	"github.com/randco/randco-microservices/services/service-game/internal/common"
	"github.com/randco/randco-microservices/services/service-game/internal/config"
	"github.com/randco/randco-microservices/services/service-game/internal/grpc/clients"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"github.com/randco/randco-microservices/services/service-game/internal/repositories"
	"github.com/randco/randco-microservices/shared/events"
	"github.com/robfig/cron/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SchedulerService handles scheduled tasks for games and draws
type SchedulerService struct {
	scheduleRepo        repositories.GameScheduleRepository
	gameRepo            repositories.GameRepository
	drawServiceClient   clients.DrawServiceClient
	ticketServiceClient clients.TicketServiceClient
	notificationClient  clients.NotificationServiceClient
	adminClient         clients.AdminServiceClient
	eventBus            events.EventBus
	cron                *cron.Cron
	location            *time.Location
	windowMinutes       int
	processedCache      *common.ProcessedScheduleCache
	fallbackEmails      []string
	tracer              trace.Tracer
	logger              *log.Logger
}

// NewSchedulerService creates a new SchedulerService instance
func NewSchedulerService(
	cfg config.SchedulerConfig,
	scheduleRepo repositories.GameScheduleRepository,
	gameRepo repositories.GameRepository,
	drawClient clients.DrawServiceClient,
	ticketClient clients.TicketServiceClient,
	eventBus events.EventBus,
	notificationClient clients.NotificationServiceClient,
	adminClient clients.AdminServiceClient,
	fallbackEmails []string,
	logger *log.Logger,
) (*SchedulerService, error) {
	// Load timezone
	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone %s: %w", cfg.Timezone, err)
	}

	// Create cron with timezone and second-level precision
	c := cron.New(
		cron.WithLocation(location),
		cron.WithSeconds(),
	)

	return &SchedulerService{
		scheduleRepo:        scheduleRepo,
		gameRepo:            gameRepo,
		drawServiceClient:   drawClient,
		ticketServiceClient: ticketClient,
		notificationClient:  notificationClient,
		adminClient:         adminClient,
		eventBus:            eventBus,
		cron:                c,
		location:            location,
		windowMinutes:       cfg.WindowMinutes,
		processedCache:      common.NewProcessedScheduleCache(),
		fallbackEmails:      fallbackEmails,
		tracer:              otel.Tracer("service-game"),
		logger:              logger,
	}, nil
}

// Start starts the scheduler and registers cron jobs
func (s *SchedulerService) Start(ctx context.Context) error {
	// Schedule sales cutoff checker - runs every minute at the top of the minute
	_, err := s.cron.AddFunc("0 * * * * *", func() {
		s.checkSalesCutoffs(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to add sales cutoff job: %w", err)
	}

	// Schedule draw time checker - runs every minute at the top of the minute
	_, err = s.cron.AddFunc("0 * * * * *", func() {
		s.checkDrawTimes(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to add draw time job: %w", err)
	}

	// Schedule cache cleanup - runs every 5 minutes
	_, err = s.cron.AddFunc("0 */5 * * * *", func() {
		s.cleanupProcessedCache()
	})
	if err != nil {
		return fmt.Errorf("failed to add cleanup job: %w", err)
	}

	s.cron.Start()
	s.logger.Printf("Scheduler started successfully (timezone: %s, window: %d minutes)",
		s.location.String(), s.windowMinutes)
	return nil
}

// Stop stops the scheduler
func (s *SchedulerService) Stop() {
	s.logger.Println("Stopping scheduler...")
	s.cron.Stop()
	s.logger.Println("Scheduler stopped")
}

// GetCurrentTime returns current time in configured timezone
func (s *SchedulerService) GetCurrentTime() time.Time {
	return time.Now().In(s.location)
}

// IsTimeReached checks if scheduled time has been reached in the correct timezone
func (s *SchedulerService) IsTimeReached(scheduledTime time.Time) bool {
	now := s.GetCurrentTime()
	scheduled := scheduledTime.In(s.location)
	return now.After(scheduled) || now.Equal(scheduled)
}

// checkSalesCutoffs checks for schedules where sales cutoff time has been reached
func (s *SchedulerService) checkSalesCutoffs(ctx context.Context) {
	ctx, span := s.tracer.Start(ctx, "scheduler.check_sales_cutoffs")
	defer span.End()

	// Get schedules where sales cutoff is within next N minutes
	schedules, err := s.scheduleRepo.GetSchedulesDueForProcessing(
		ctx,
		"sales_cutoff",
		s.windowMinutes,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get schedules")
		s.logger.Printf("Error getting schedules for sales cutoff: %v", err)
		return
	}

	span.SetAttributes(attribute.Int("schedules.found", len(schedules)))

	for _, schedule := range schedules {
		// Skip if recently processed
		if s.processedCache.WasRecentlyProcessed(schedule.ID, "sales_cutoff", 5*time.Minute) {
			continue
		}

		// Check if cutoff time has been reached
		if s.IsTimeReached(schedule.ScheduledEnd) {
			if err := s.handleSalesCutoff(ctx, schedule); err != nil {
				span.RecordError(err)
				s.logger.Printf("Error handling sales cutoff for schedule %s: %v", schedule.ID, err)
				continue
			}

			// Mark as processed
			s.processedCache.MarkProcessed(schedule.ID, "sales_cutoff")
			gameName := "unknown"
			if schedule.GameName != nil {
				gameName = *schedule.GameName
			}
			s.logger.Printf("Sales cutoff processed for schedule %s (game: %s)",
				schedule.ID, gameName)
		}
	}
}

// handleSalesCutoff handles the sales cutoff event for a schedule
func (s *SchedulerService) handleSalesCutoff(ctx context.Context, schedule *models.GameSchedule) error {
	ctx, span := s.tracer.Start(ctx, "scheduler.handle_sales_cutoff")
	defer span.End()

	span.SetAttributes(
		attribute.String("schedule.id", schedule.ID.String()),
		attribute.String("game.id", schedule.GameID.String()),
		attribute.String("scheduled_end", schedule.ScheduledEnd.Format(time.RFC3339)),
	)

	// Get game details
	game, err := s.gameRepo.GetByID(ctx, schedule.GameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get game")
		return fmt.Errorf("failed to get game: %w", err)
	}

	// Update schedule status to IN_PROGRESS
	schedule.Status = models.ScheduleStatusInProgress
	if err := s.scheduleRepo.Update(ctx, schedule); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update schedule status")
		return fmt.Errorf("failed to update schedule status: %w", err)
	}

	// Get game name and code
	gameName := game.Name
	if schedule.GameName != nil {
		gameName = *schedule.GameName
	}
	gameCode := game.Code
	if schedule.GameCode != nil {
		gameCode = *schedule.GameCode
	}

	// Publish game.sales_cutoff_reached event to Kafka for notification service to consume
	salesCutoffEvent := events.NewGameSalesCutoffReachedEvent(
		"service-game",
		schedule.ID.String(),
		schedule.GameID.String(),
		gameName,
		gameCode,
		schedule.ScheduledEnd,
		schedule.ScheduledDraw,
	)

	if err := s.eventBus.Publish(ctx, "game.events", salesCutoffEvent); err != nil {
		s.logger.Printf("Failed to publish sales cutoff event for schedule %s: %v", schedule.ID, err)
		span.RecordError(err)
		span.SetAttributes(attribute.String("event.publish.error", err.Error()))
		// Continue despite event publish failure - notification service will handle retries
	} else {
		s.logger.Printf("Published sales cutoff event for schedule %s to game.events topic", schedule.ID)
		span.SetAttributes(
			attribute.String("event.id", salesCutoffEvent.EventID),
			attribute.String("event.type", string(salesCutoffEvent.EventType)),
		)
	}

	s.logger.Printf("Sales cutoff reached for schedule %s - sales are now closed", schedule.ID)
	return nil
}

// checkDrawTimes checks for schedules where draw time has been reached
func (s *SchedulerService) checkDrawTimes(ctx context.Context) {
	ctx, span := s.tracer.Start(ctx, "scheduler.check_draw_times")
	defer span.End()

	schedules, err := s.scheduleRepo.GetSchedulesDueForProcessing(
		ctx,
		"draw_time",
		s.windowMinutes,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get schedules")
		s.logger.Printf("Error getting schedules for draw time: %v", err)
		return
	}

	span.SetAttributes(attribute.Int("schedules.found", len(schedules)))

	for _, schedule := range schedules {
		// Skip if recently processed
		if s.processedCache.WasRecentlyProcessed(schedule.ID, "draw_time", 5*time.Minute) {
			continue
		}

		if s.IsTimeReached(schedule.ScheduledDraw) {
			if err := s.handleDrawTime(ctx, schedule); err != nil {
				span.RecordError(err)
				s.logger.Printf("Error handling draw time for schedule %s: %v", schedule.ID, err)
				continue
			}

			s.processedCache.MarkProcessed(schedule.ID, "draw_time")
			gameName := "unknown"
			if schedule.GameName != nil {
				gameName = *schedule.GameName
			}
			s.logger.Printf("Draw time processed for schedule %s (game: %s)",
				schedule.ID, gameName)
		}
	}
}

// handleDrawTime handles the draw time event for a schedule
func (s *SchedulerService) handleDrawTime(ctx context.Context, schedule *models.GameSchedule) error {
	ctx, span := s.tracer.Start(ctx, "scheduler.handle_draw_time")
	defer span.End()

	span.SetAttributes(
		attribute.String("schedule.id", schedule.ID.String()),
		attribute.String("game.id", schedule.GameID.String()),
		attribute.String("scheduled_draw", schedule.ScheduledDraw.Format(time.RFC3339)),
	)

	// Get game details
	game, err := s.gameRepo.GetByID(ctx, schedule.GameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get game")
		return fmt.Errorf("failed to get game: %w", err)
	}

	// Create draw record in Draw Service via gRPC
	drawID, err := s.createDrawRecord(ctx, game, schedule)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create draw record")
		return fmt.Errorf("failed to create draw record: %w", err)
	}

	// Update schedule with draw result ID
	schedule.DrawResultID = &drawID
	schedule.Status = models.ScheduleStatusCompleted
	if err := s.scheduleRepo.Update(ctx, schedule); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update schedule")
		return fmt.Errorf("failed to update schedule: %w", err)
	}

	// Publish game.draw_executed event
	gameName := game.Name
	if schedule.GameName != nil {
		gameName = *schedule.GameName
	}
	gameCode := game.Code
	if schedule.GameCode != nil {
		gameCode = *schedule.GameCode
	}

	// Send notification via gRPC
	if err := s.sendDrawExecutedNotification(ctx, schedule, gameName, gameCode, drawID); err != nil {
		// Log error but don't fail the operation
		s.logger.Printf("Failed to send draw executed notification for schedule %s: %v", schedule.ID, err)
		span.RecordError(err)
		span.SetAttributes(attribute.String("notification.error", err.Error()))
	}

	s.logger.Printf("Draw record created successfully: draw_id=%s, schedule_id=%s",
		drawID, schedule.ID)
	return nil
}

// createDrawRecord creates a draw record in the Draw Service
func (s *SchedulerService) createDrawRecord(
	ctx context.Context,
	game *models.Game,
	schedule *models.GameSchedule,
) (uuid.UUID, error) {
	ctx, span := s.tracer.Start(ctx, "scheduler.create_draw_record")
	defer span.End()

	span.SetAttributes(
		attribute.String("game.id", game.ID.String()),
		attribute.String("game.name", game.Name),
		attribute.String("schedule.id", schedule.ID.String()),
	)

	drawID, err := s.drawServiceClient.CreateDraw(ctx, game, schedule, s.ticketServiceClient)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create draw via gRPC")
		return uuid.Nil, fmt.Errorf("failed to create draw via gRPC: %w", err)
	}

	span.SetAttributes(attribute.String("draw.id", drawID.String()))
	span.SetStatus(codes.Ok, "draw created successfully")
	return drawID, nil
}

// cleanupProcessedCache removes old entries from the processed cache
func (s *SchedulerService) cleanupProcessedCache() {
	_, span := s.tracer.Start(context.Background(), "scheduler.cleanup_cache")
	defer span.End()

	// Remove entries older than 10 minutes
	removedCount := s.processedCache.Cleanup(10 * time.Minute)

	span.SetAttributes(
		attribute.Int("cache.removed", removedCount),
		attribute.Int("cache.size", s.processedCache.Size()),
	)

	if removedCount > 0 {
		s.logger.Printf("Cache cleanup: removed %d entries, current size: %d",
			removedCount, s.processedCache.Size())
	}
}

// fetchAdminEmails retrieves active admin emails with retry logic and fallback
func (s *SchedulerService) fetchAdminEmails(ctx context.Context) []string {
	ctx, span := s.tracer.Start(ctx, "scheduler.fetch_admin_emails")
	defer span.End()

	var emails []string

	// If admin client is nil, use fallback emails
	if s.adminClient == nil {
		s.logger.Printf("Admin client not available, using fallback emails: %v", s.fallbackEmails)
		span.SetAttributes(
			attribute.Bool("used_fallback", true),
		)
		emails = s.fallbackEmails
	} else {
		// Try to fetch emails from Admin Service with retry logic
		maxRetries := 3
		retryDelay := 1 * time.Second

		for attempt := 1; attempt <= maxRetries; attempt++ {
			// Create a fresh context with timeout for each retry
			retryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			fetchedEmails, err := s.adminClient.ListActiveAdminEmails(retryCtx)
			cancel() // Clean up context

			if err == nil && len(fetchedEmails) > 0 {
				s.logger.Printf("Successfully fetched %d admin emails from Admin Service", len(fetchedEmails))
				span.SetAttributes(
					attribute.Bool("used_fallback", false),
					attribute.Int("attempt", attempt),
				)
				emails = fetchedEmails
				break
			}

			if err != nil {
				s.logger.Printf("Failed to fetch admin emails from Admin Service (attempt %d/%d): %v",
					attempt, maxRetries, err)
				span.RecordError(err)
			} else {
				s.logger.Printf("Admin Service returned empty email list (attempt %d/%d)", attempt, maxRetries)
			}

			// Wait before retry (except on last attempt)
			if attempt < maxRetries {
				time.Sleep(retryDelay)
				retryDelay *= 2 // Exponential backoff
			}
		}

		// All retries failed, use fallback emails
		if len(emails) == 0 {
			s.logger.Printf("All attempts to fetch admin emails failed, using fallback emails: %v", s.fallbackEmails)
			span.SetAttributes(
				attribute.Bool("used_fallback", true),
				attribute.String("reason", "all_retries_failed"),
			)
			emails = s.fallbackEmails
		}
	}

	// Deduplicate emails to prevent sending duplicates
	uniqueEmails := deduplicateEmails(emails)
	if len(uniqueEmails) != len(emails) {
		s.logger.Printf("Deduplicated admin emails: %d -> %d unique addresses", len(emails), len(uniqueEmails))
		span.SetAttributes(
			attribute.Int("email_count_before_dedup", len(emails)),
			attribute.Int("email_count_after_dedup", len(uniqueEmails)),
		)
	}

	span.SetAttributes(attribute.Int("email_count", len(uniqueEmails)))
	return uniqueEmails
}

// deduplicateEmails removes duplicate email addresses from the list
func deduplicateEmails(emails []string) []string {
	if len(emails) == 0 {
		return emails
	}

	seen := make(map[string]bool, len(emails))
	unique := make([]string, 0, len(emails))

	for _, email := range emails {
		// Normalize to lowercase for comparison
		normalized := strings.ToLower(strings.TrimSpace(email))
		if normalized == "" {
			continue
		}
		if !seen[normalized] {
			seen[normalized] = true
			unique = append(unique, email) // Keep original casing
		}
	}

	return unique
}

// sendDrawExecutedNotification sends draw executed notification via gRPC
func (s *SchedulerService) sendDrawExecutedNotification(
	ctx context.Context,
	schedule *models.GameSchedule,
	gameName, gameCode string,
	drawID uuid.UUID,
) error {
	ctx, span := s.tracer.Start(ctx, "scheduler.send_draw_executed_notification")
	defer span.End()

	span.SetAttributes(
		attribute.String("schedule.id", schedule.ID.String()),
		attribute.String("game.name", gameName),
		attribute.String("game.code", gameCode),
		attribute.String("draw.id", drawID.String()),
	)

	// Check if notification client is available
	if s.notificationClient == nil {
		s.logger.Println("Notification client not available, skipping draw executed notification")
		span.SetAttributes(attribute.Bool("notification.skipped", true))
		return nil
	}

	// Fetch admin emails
	adminEmails := s.fetchAdminEmails(ctx)
	if len(adminEmails) == 0 {
		s.logger.Printf("WARNING: No admin emails available, skipping draw executed notification for schedule %s", schedule.ID)
		span.SetAttributes(
			attribute.Bool("notification.skipped", true),
			attribute.String("skip_reason", "no_admin_emails"),
		)
		return nil
	}

	span.SetAttributes(attribute.Int("recipient_count", len(adminEmails)))

	// Prepare template variables for game_end template
	now := time.Now()
	variables := map[string]string{
		"GameName":          gameName,
		"GameCode":          gameCode,
		"ScheduledDrawTime": schedule.ScheduledDraw.In(s.location).Format("2006-01-02 15:04:05 MST"),
		"ActualDrawTime":    now.In(s.location).Format("2006-01-02 15:04:05 MST"),
		"DrawID":            drawID.String(),
		"ScheduleID":        schedule.ID.String(),
		"NotificationTime":  now.In(s.location).Format("2006-01-02 15:04:05 MST"),
		"CompanyName":       "RAND Lottery",
		"CompanyAddress":    "Accra, Ghana",
		"CurrentYear":       fmt.Sprintf("%d", now.Year()),
	}

	// Build request list with idempotency keys and template
	requests := make([]*notificationv1.SendEmailRequest, len(adminEmails))
	for i, email := range adminEmails {
		// Use schedule_id + email as idempotency key
		idempotencyKey := fmt.Sprintf("draw_executed_%s_%s", schedule.ID.String(), email)
		requests[i] = &notificationv1.SendEmailRequest{
			IdempotencyKey: idempotencyKey,
			To:             email,
			TemplateId:     "game_end",
			Variables:      variables,
		}
	}

	// Send via gRPC with shared idempotency key for the bulk operation
	bulkIdempotencyKey := fmt.Sprintf("draw_executed_bulk_%s", schedule.ID.String())
	req := &notificationv1.SendBulkEmailRequest{
		TemplateId:     "game_end",
		Requests:       requests,
		IdempotencyKey: bulkIdempotencyKey,
	}

	// Retry logic for notification sending
	maxRetries := 3
	retryDelay := 1 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create a fresh context with timeout for each retry
		retryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		resp, err := s.notificationClient.SendBulkEmail(retryCtx, req)
		cancel() // Clean up context

		if err == nil {
			s.logger.Printf("Draw executed notification sent successfully to %d admins (schedule: %s, draw: %s)",
				len(adminEmails), schedule.ID, drawID)
			if resp != nil && resp.Message != "" {
				s.logger.Printf("Notification service response: %s", resp.Message)
			}
			span.SetAttributes(
				attribute.Bool("notification.sent", true),
				attribute.Int("attempt", attempt),
			)
			return nil
		}

		s.logger.Printf("Failed to send draw executed notification (attempt %d/%d): %v",
			attempt, maxRetries, err)
		span.RecordError(err)

		// Wait before retry (except on last attempt)
		if attempt < maxRetries {
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
		}
	}

	// All retries failed
	err := fmt.Errorf("failed to send draw executed notification after %d attempts", maxRetries)
	span.RecordError(err)
	span.SetStatus(codes.Error, "notification sending failed")
	return err
}

package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	gamev1 "github.com/randco/randco-microservices/proto/game/v1"
	ticketv1 "github.com/randco/randco-microservices/proto/ticket/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/randco/service-draw/internal/models"
)

type DrawRepository interface {
	Create(ctx context.Context, draw *models.Draw) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Draw, error)
	Update(ctx context.Context, draw *models.Draw) error
	List(ctx context.Context, gameID *uuid.UUID, status *models.DrawStatus, startDate, endDate *time.Time, page, perPage int) ([]*models.Draw, int64, error)
	ListCompletedPublic(ctx context.Context, gameID *uuid.UUID, gameCode string, latestOnly bool, startDate, endDate *time.Time, page, perPage int) ([]*models.Draw, int64, error)
	UpdateTicketStats(ctx context.Context, drawID uuid.UUID, totalTicketsSold, totalPrizePool int64) error
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

type DrawService interface {
	CreateDraw(ctx context.Context, request CreateDrawRequest) (*models.Draw, error)
	UpdateDraw(ctx context.Context, request UpdateDrawRequest) (*models.Draw, error)
	GetDraw(ctx context.Context, id uuid.UUID) (*models.Draw, error)
	ListDraws(ctx context.Context, request ListDrawsRequest) ([]*models.Draw, int64, error)
	GetPublicCompletedDraws(ctx context.Context, request GetPublicCompletedDrawsRequest) ([]*models.Draw, int64, error)
	RecordDrawResults(ctx context.Context, request RecordDrawResultsRequest) (*models.DrawResult, error)
	ScheduleDraw(ctx context.Context, request ScheduleDrawRequest) (*models.DrawSchedule, error)
	GetScheduledDraws(ctx context.Context, request GetScheduledDrawsRequest) ([]*models.DrawSchedule, error)
	CancelScheduledDraw(ctx context.Context, scheduleID uuid.UUID, cancelledBy, reason string) error

	// Draw execution workflow - Stage 1: Preparation
	StartDrawPreparation(ctx context.Context, drawID uuid.UUID, initiatedBy string) (*models.Draw, error)
	CompleteDrawPreparation(ctx context.Context, drawID uuid.UUID, completedBy string) (*models.Draw, error)

	// Draw execution workflow - Stage 2: Number Selection
	StartNumberSelection(ctx context.Context, drawID uuid.UUID, initiatedBy string) (*models.Draw, error)
	SubmitVerificationAttempt(ctx context.Context, drawID uuid.UUID, numbers []int32, submittedBy string) (*models.Draw, int32, error)
	ValidateVerificationAttempts(ctx context.Context, drawID uuid.UUID, validatedBy string) (bool, []int32, string, error)
	ResetVerificationAttempts(ctx context.Context, drawID uuid.UUID, resetBy, reason string) (*models.Draw, error)
	CompleteNumberSelection(ctx context.Context, drawID uuid.UUID, winningNumbers []int32, completedBy string) (*models.Draw, error)

	// Draw execution workflow - Stage 3: Result Calculation
	CommitResults(ctx context.Context, drawID uuid.UUID, committedBy string) (*models.Draw, *ResultCalculationSummary, error)

	// Draw execution workflow - Stage 4: Payout Processing
	ProcessPayouts(ctx context.Context, drawID uuid.UUID, processedBy string) (*models.Draw, *PayoutSummary, error)
	ProcessBigWinPayout(ctx context.Context, drawID uuid.UUID, ticketID string, approve bool, processedBy, rejectionReason string) (*models.BigWinPayout, error)
	CompleteDraw(ctx context.Context, drawID uuid.UUID, completedBy string) (*models.Draw, error)

	// Machine Numbers (cosmetic, entered after draw completion)
	UpdateMachineNumbers(ctx context.Context, drawID uuid.UUID, machineNumbers []int32, updatedBy string) (*models.Draw, error)

	// gRPC client access for enrichment
	GameServiceClient(ctx context.Context) (gamev1.GameServiceClient, error)
}

type CreateDrawRequest struct {
	GameID         uuid.UUID `json:"game_id" validate:"required"`
	GameName       string    `json:"game_name" validate:"required,min=2,max=255"`
	GameCode       string    `json:"game_code" validate:"required,min=1,max=20"`
	GameScheduleID uuid.UUID `json:"game_schedule_id"` // Link to the game schedule this draw is created from
	DrawName       string    `json:"draw_name" validate:"required,min=2,max=255"`
	ScheduledTime  time.Time `json:"scheduled_time" validate:"required"`
	DrawLocation   string    `json:"draw_location" validate:"required"`
}

type UpdateDrawRequest struct {
	ID            uuid.UUID `json:"id" validate:"required"`
	DrawName      string    `json:"draw_name"`
	ScheduledTime time.Time `json:"scheduled_time"`
	DrawLocation  string    `json:"draw_location"`
}

type RecordDrawResultsRequest struct {
	DrawID               uuid.UUID             `json:"draw_id" validate:"required"`
	WinningNumbers       []int32               `json:"winning_numbers" validate:"required,min=1"`
	NLADrawReference     string                `json:"nla_draw_reference" validate:"required"`
	NLAOfficialSignature string                `json:"nla_official_signature" validate:"required"`
	Validation           models.DrawValidation `json:"validation"`
	RecordedBy           string                `json:"recorded_by" validate:"required"`
}

type ListDrawsRequest struct {
	GameID    *uuid.UUID         `json:"game_id"`
	Status    *models.DrawStatus `json:"status"`
	StartDate *time.Time         `json:"start_date"`
	EndDate   *time.Time         `json:"end_date"`
	Page      int                `json:"page"`
	PerPage   int                `json:"per_page"`
}

type GetPublicCompletedDrawsRequest struct {
	GameID     *uuid.UUID `json:"game_id"`
	GameCode   string     `json:"game_code"`
	Page       int        `json:"page"`
	PerPage    int        `json:"per_page"`
	LatestOnly bool       `json:"latest_only"`
	StartDate  *time.Time `json:"start_date"`
	EndDate    *time.Time `json:"end_date"`
}

type ScheduleDrawRequest struct {
	GameID        uuid.UUID                `json:"game_id" validate:"required"`
	DrawName      string                   `json:"draw_name" validate:"required,min=2,max=255"`
	ScheduledTime time.Time                `json:"scheduled_time" validate:"required"`
	Frequency     models.ScheduleFrequency `json:"frequency"`
	CreatedBy     string                   `json:"created_by" validate:"required"`
	Notes         *string                  `json:"notes"`
}

type GetScheduledDrawsRequest struct {
	GameID     *uuid.UUID `json:"game_id"`
	StartDate  *time.Time `json:"start_date"`
	EndDate    *time.Time `json:"end_date"`
	ActiveOnly bool       `json:"active_only"`
}

type drawService struct {
	drawRepo          DrawRepository
	logger            *log.Logger
	grpcClientManager GRPCClientManager
	payoutProcessor   *PayoutProcessor
}

// GRPCClientManager interface for managing gRPC client connections
type GRPCClientManager interface {
	TicketServiceClient(ctx context.Context) (interface{}, error)
	WalletServiceClient(ctx context.Context) (interface{}, error)
	GameServiceClient(ctx context.Context) (gamev1.GameServiceClient, error)
}

func NewDrawService(drawRepo DrawRepository, logger *log.Logger, grpcClientManager GRPCClientManager) DrawService {
	ds := &drawService{
		drawRepo:          drawRepo,
		logger:            logger,
		grpcClientManager: grpcClientManager,
	}

	// Initialize the payout processor
	ds.payoutProcessor = NewPayoutProcessor(ds)

	return ds
}

func (s *drawService) CreateDraw(ctx context.Context, request CreateDrawRequest) (*models.Draw, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawService.CreateDraw")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw.game_id", request.GameID.String()),
		attribute.String("draw.name", request.DrawName),
		attribute.String("draw.location", request.DrawLocation),
	)

	// Validate request
	if request.GameID == uuid.Nil {
		err := fmt.Errorf("game ID is required")
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, err
	}

	// Note: We allow past scheduled times because the Game Service scheduler
	// may be creating draws for schedules that have already passed their draw time

	// Create the draw record
	draw := &models.Draw{
		ID:             uuid.New(),
		GameID:         request.GameID,
		GameName:       request.GameName,
		GameCode:       request.GameCode,
		GameScheduleID: request.GameScheduleID, // Link to game schedule for ticket updates
		DrawName:       request.DrawName,
		Status:         models.DrawStatusScheduled,
		ScheduledTime:  request.ScheduledTime,
		DrawLocation:   &request.DrawLocation,
	}

	err := s.drawRepo.Create(ctx, draw)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create draw: %w", err)
	}

	s.logger.Printf("Draw created successfully: draw_id=%s, game_schedule_id=%s", draw.ID, draw.GameScheduleID)

	// Update tickets with draw_id if game_schedule_id is available
	if draw.GameScheduleID != uuid.Nil {
		s.logger.Printf("Updating tickets for schedule %s with draw_id %s", draw.GameScheduleID, draw.ID)

		ticketClient, err := s.grpcClientManager.TicketServiceClient(ctx)
		if err != nil {
			s.logger.Printf("WARNING: Failed to get ticket client: %v", err)
			// Don't fail the draw creation, just log the warning
		} else if client, ok := ticketClient.(ticketv1.TicketServiceClient); ok {
			updateResp, err := client.UpdateTicketsDrawId(ctx, &ticketv1.UpdateTicketsDrawIdRequest{
				GameScheduleId: draw.GameScheduleID.String(),
				DrawId:         draw.ID.String(),
			})

			if err != nil {
				s.logger.Printf("WARNING: Failed to update tickets with draw_id: %v", err)
			} else if updateResp.Success {
				s.logger.Printf("SUCCESS: Updated %d tickets with draw_id", updateResp.UpdatedCount)
				span.SetAttributes(attribute.Int64("tickets_updated", updateResp.UpdatedCount))
			} else {
				s.logger.Printf("WARNING: Ticket update returned failure: %s", updateResp.Message)
			}
		}
	}

	span.SetAttributes(attribute.String("draw.id", draw.ID.String()))
	return draw, nil
}

func (s *drawService) UpdateDraw(ctx context.Context, request UpdateDrawRequest) (*models.Draw, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawService.UpdateDraw")
	defer span.End()

	span.SetAttributes(attribute.String("draw.id", request.ID.String()))

	// Get existing draw
	draw, err := s.drawRepo.GetByID(ctx, request.ID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, err
	}

	// Update fields if provided
	if request.DrawName != "" {
		draw.DrawName = request.DrawName
	}
	if !request.ScheduledTime.IsZero() {
		draw.ScheduledTime = request.ScheduledTime
	}
	if request.DrawLocation != "" {
		draw.DrawLocation = &request.DrawLocation
	}

	err = s.drawRepo.Update(ctx, draw)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update draw: %w", err)
	}

	return draw, nil
}

func (s *drawService) RecordDrawResults(ctx context.Context, request RecordDrawResultsRequest) (*models.DrawResult, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawService.RecordDrawResults")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw.id", request.DrawID.String()),
		attribute.String("nla_reference", request.NLADrawReference),
		attribute.Int("winning_numbers.count", len(request.WinningNumbers)),
	)

	// Get the draw
	draw, err := s.drawRepo.GetByID(ctx, request.DrawID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, err
	}

	// Validate draw can accept results
	if draw.Status != models.DrawStatusScheduled && draw.Status != models.DrawStatusInProgress {
		err = fmt.Errorf("draw is not in a state to accept results: %s", draw.Status)
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, err
	}

	// Update draw with results
	now := time.Now()
	draw.Status = models.DrawStatusCompleted
	draw.ExecutedTime = &now
	draw.WinningNumbers = request.WinningNumbers
	draw.NLADrawReference = &request.NLADrawReference
	draw.NLAOfficialSignature = &request.NLAOfficialSignature

	// Create verification hash
	verificationData := fmt.Sprintf("%v_%s_%s_%d",
		request.WinningNumbers,
		request.NLADrawReference,
		request.NLAOfficialSignature,
		now.Unix())
	verificationHash := fmt.Sprintf("%x", verificationData) // Simplified hash
	draw.VerificationHash = &verificationHash

	err = s.drawRepo.Update(ctx, draw)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update draw with results: %w", err)
	}

	// Create draw result record
	drawResult := &models.DrawResult{
		ID:             uuid.New(),
		DrawID:         request.DrawID,
		WinningNumbers: request.WinningNumbers,
		IsPublished:    false,
		CreatedAt:      now,
	}

	span.SetAttributes(
		attribute.String("draw_result.id", drawResult.ID.String()),
		attribute.String("result", "success"),
	)

	return drawResult, nil
}

func (s *drawService) GetDraw(ctx context.Context, id uuid.UUID) (*models.Draw, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawService.GetDraw")
	defer span.End()

	span.SetAttributes(attribute.String("draw.id", id.String()))

	draw, err := s.drawRepo.GetByID(ctx, id)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, err
	}

	// Enrich winning tickets with complete ticket details from ticket service
	s.logger.Printf("DEBUG: Checking if we should enrich tickets - StageData: %v, ResultCalc: %v, WinningTickets: %d",
		draw.StageData != nil,
		draw.StageData != nil && draw.StageData.ResultCalculationData != nil,
		func() int {
			if draw.StageData != nil && draw.StageData.ResultCalculationData != nil {
				return len(draw.StageData.ResultCalculationData.WinningTickets)
			}
			return 0
		}())

	if draw.StageData != nil && draw.StageData.ResultCalculationData != nil &&
		len(draw.StageData.ResultCalculationData.WinningTickets) > 0 {

		s.logger.Printf("DEBUG: Starting ticket enrichment for %d winning tickets", len(draw.StageData.ResultCalculationData.WinningTickets))

		// Get ticket service client
		ticketClient, err := s.grpcClientManager.TicketServiceClient(ctx)
		if err != nil {
			s.logger.Printf("Error getting ticket service client for enrichment: %v", err)
			// Continue without enrichment - return basic winning ticket data
		} else if client, ok := ticketClient.(ticketv1.TicketServiceClient); ok {
			s.logger.Printf("DEBUG: Got ticket service client, enriching %d tickets", len(draw.StageData.ResultCalculationData.WinningTickets))
			// Enrich each winning ticket with complete details
			for i, winTicket := range draw.StageData.ResultCalculationData.WinningTickets {
				s.logger.Printf("DEBUG: Fetching ticket %d/%d: %s", i+1, len(draw.StageData.ResultCalculationData.WinningTickets), winTicket.TicketID)
				// Fetch complete ticket details from ticket service
				ticketResp, err := client.GetTicket(ctx, &ticketv1.GetTicketRequest{
					TicketId: winTicket.TicketID,
				})
				if err != nil {
					s.logger.Printf("Error fetching ticket %s: %v", winTicket.TicketID, err)
					continue // Skip this ticket if we can't fetch details
				}

				if ticketResp.Ticket != nil {
					s.logger.Printf("DEBUG: Enriching ticket %s with agent_code=%s, terminal_id=%s, phone=%s",
						winTicket.TicketID,
						ticketResp.Ticket.IssuerDetails.GetRetailerCode(),
						ticketResp.Ticket.IssuerDetails.GetTerminalId(),
						ticketResp.Ticket.CustomerPhone)
					// Enrich the winning ticket with additional details
					ticket := ticketResp.Ticket
					if ticket.IssuerDetails != nil {
						draw.StageData.ResultCalculationData.WinningTickets[i].AgentCode = ticket.IssuerDetails.RetailerCode
						draw.StageData.ResultCalculationData.WinningTickets[i].TerminalID = ticket.IssuerDetails.TerminalId
					}
					draw.StageData.ResultCalculationData.WinningTickets[i].CustomerPhone = ticket.CustomerPhone
					draw.StageData.ResultCalculationData.WinningTickets[i].PaymentMethod = ticket.PaymentMethod
					draw.StageData.ResultCalculationData.WinningTickets[i].Status = ticket.Status
					s.logger.Printf("DEBUG: Ticket %s enriched successfully", winTicket.TicketID)
				} else {
					s.logger.Printf("DEBUG: No ticket data returned for %s", winTicket.TicketID)
				}
			}
			s.logger.Printf("DEBUG: Ticket enrichment completed")
		} else {
			s.logger.Printf("DEBUG: Failed to cast ticket client to TicketServiceClient")
		}
	} else {
		s.logger.Printf("DEBUG: Skipping enrichment - no winning tickets to enrich")
	}

	return draw, nil
}

func (s *drawService) ListDraws(ctx context.Context, request ListDrawsRequest) ([]*models.Draw, int64, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawService.ListDraws")
	defer span.End()

	if request.GameID != nil {
		span.SetAttributes(attribute.String("filter.game_id", request.GameID.String()))
	}
	if request.Status != nil {
		span.SetAttributes(attribute.String("filter.status", request.Status.String()))
	}
	span.SetAttributes(
		attribute.Int("pagination.page", request.Page),
		attribute.Int("pagination.per_page", request.PerPage),
	)

	draws, total, err := s.drawRepo.List(ctx, request.GameID, request.Status, request.StartDate, request.EndDate, request.Page, request.PerPage)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, 0, err
	}

	// For draws that still show zero tickets, sync stats from the ticket service.
	ticketClient, tcErr := s.grpcClientManager.TicketServiceClient(ctx)
	if tcErr == nil {
		if tc, ok := ticketClient.(ticketv1.TicketServiceClient); ok {
			for _, draw := range draws {
				if draw.TotalTicketsSold > 0 {
					continue
				}
				if draw.GameScheduleID == uuid.Nil {
					continue
				}
				resp, err := tc.ListTickets(ctx, &ticketv1.ListTicketsRequest{
					Filter:   &ticketv1.TicketFilter{GameScheduleId: draw.GameScheduleID.String()},
					PageSize: 1, // only need total count
				})
				if err != nil || resp.Total == 0 {
					continue
				}
				// Fetch all to sum stakes
				all, err := tc.ListTickets(ctx, &ticketv1.ListTicketsRequest{
					Filter:   &ticketv1.TicketFilter{GameScheduleId: draw.GameScheduleID.String()},
					PageSize: int32(resp.Total) + 1,
				})
				if err != nil {
					continue
				}
				var totalStakes int64
				for _, t := range all.Tickets {
					totalStakes += t.TotalAmount
				}
				draw.TotalTicketsSold = int64(resp.Total)
				draw.TotalPrizePool = totalStakes
				// Persist so next load is instant
				_ = s.drawRepo.UpdateTicketStats(ctx, draw.ID, draw.TotalTicketsSold, draw.TotalPrizePool)
			}
		}
	}

	span.SetAttributes(
		attribute.Int("result.draws_count", len(draws)),
		attribute.Int64("result.total_count", total),
	)

	return draws, total, nil
}

func (s *drawService) GetPublicCompletedDraws(ctx context.Context, request GetPublicCompletedDrawsRequest) ([]*models.Draw, int64, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawService.GetPublicCompletedDraws")
	defer span.End()

	if request.GameID != nil {
		span.SetAttributes(attribute.String("filter.game_id", request.GameID.String()))
	}
	if request.GameCode != "" {
		span.SetAttributes(attribute.String("filter.game_code", request.GameCode))
	}
	if request.StartDate != nil {
		span.SetAttributes(attribute.String("filter.start_date", request.StartDate.Format(time.RFC3339)))
	}
	if request.EndDate != nil {
		span.SetAttributes(attribute.String("filter.end_date", request.EndDate.Format(time.RFC3339)))
	}
	span.SetAttributes(
		attribute.Int("pagination.page", request.Page),
		attribute.Int("pagination.per_page", request.PerPage),
		attribute.Bool("filter.latest_only", request.LatestOnly),
	)

	// Call repository to get completed draws with winning numbers
	draws, total, err := s.drawRepo.ListCompletedPublic(ctx, request.GameID, request.GameCode, request.LatestOnly, request.StartDate, request.EndDate, request.Page, request.PerPage)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to list public completed draws: %w", err)
	}

	span.SetAttributes(
		attribute.Int("result.draws_count", len(draws)),
		attribute.Int64("result.total_count", total),
	)

	return draws, total, nil
}

func (s *drawService) ScheduleDraw(ctx context.Context, request ScheduleDrawRequest) (*models.DrawSchedule, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawService.ScheduleDraw")
	defer span.End()

	span.SetAttributes(
		attribute.String("schedule.game_id", request.GameID.String()),
		attribute.String("schedule.draw_name", request.DrawName),
		attribute.String("schedule.frequency", request.Frequency.String()),
		attribute.String("schedule.created_by", request.CreatedBy),
	)

	// Validate request
	if request.GameID == uuid.Nil {
		err := fmt.Errorf("game ID is required")
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, err
	}

	// Note: We allow past scheduled times for backfilling or retroactive scheduling

	if request.Frequency == "" {
		request.Frequency = models.FrequencyOneTime
	}

	// Create schedule
	schedule := &models.DrawSchedule{
		ID:            uuid.New(),
		GameID:        request.GameID,
		DrawName:      request.DrawName,
		ScheduledTime: request.ScheduledTime,
		Frequency:     request.Frequency,
		IsActive:      true,
		CreatedBy:     request.CreatedBy,
		Notes:         request.Notes,
	}

	err := s.drawRepo.CreateSchedule(ctx, schedule)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create draw schedule: %w", err)
	}

	span.SetAttributes(attribute.String("schedule.id", schedule.ID.String()))
	return schedule, nil
}

func (s *drawService) GetScheduledDraws(ctx context.Context, request GetScheduledDrawsRequest) ([]*models.DrawSchedule, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawService.GetScheduledDraws")
	defer span.End()

	if request.GameID != nil {
		span.SetAttributes(attribute.String("filter.game_id", request.GameID.String()))
	}
	span.SetAttributes(attribute.Bool("filter.active_only", request.ActiveOnly))

	schedules, err := s.drawRepo.ListSchedules(ctx, request.GameID, request.StartDate, request.EndDate, request.ActiveOnly)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, err
	}

	span.SetAttributes(attribute.Int("result.schedules_count", len(schedules)))
	return schedules, nil
}

func (s *drawService) CancelScheduledDraw(ctx context.Context, scheduleID uuid.UUID, cancelledBy, reason string) error {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawService.CancelScheduledDraw")
	defer span.End()

	span.SetAttributes(
		attribute.String("schedule.id", scheduleID.String()),
		attribute.String("cancelled_by", cancelledBy),
		attribute.String("cancellation_reason", reason),
	)

	// Get the schedule
	schedule, err := s.drawRepo.GetScheduleByID(ctx, scheduleID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return err
	}

	// Update schedule to inactive
	schedule.IsActive = false
	if reason != "" {
		notes := fmt.Sprintf("Cancelled by %s: %s", cancelledBy, reason)
		schedule.Notes = &notes
	}

	err = s.drawRepo.UpdateSchedule(ctx, schedule)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return fmt.Errorf("failed to cancel scheduled draw: %w", err)
	}

	span.SetAttributes(attribute.String("result", "cancelled"))
	return nil
}

// ResultCalculationSummary contains summary data from Stage 3
type ResultCalculationSummary struct {
	WinningTicketsCount int64                `json:"winning_tickets_count"`
	TotalWinnings       int64                `json:"total_winnings"` // in pesewas
	WinningTiers        []models.WinningTier `json:"winning_tiers"`
}

// PayoutSummary contains summary data from Stage 4
type PayoutSummary struct {
	AutoProcessedCount   int64 `json:"auto_processed_count"`
	ManualApprovalCount  int64 `json:"manual_approval_count"`
	AutoProcessedAmount  int64 `json:"auto_processed_amount"`  // in pesewas
	ManualApprovalAmount int64 `json:"manual_approval_amount"` // in pesewas
}

// ============================================================================
// Stage 1: Preparation
// ============================================================================

// StartDrawPreparation initializes the draw execution workflow
func (s *drawService) StartDrawPreparation(ctx context.Context, drawID uuid.UUID, initiatedBy string) (*models.Draw, error) {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "DrawService.StartDrawPreparation")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw_id", drawID.String()),
		attribute.String("initiated_by", initiatedBy),
	)

	// Get the draw
	draw, err := s.drawRepo.GetByID(ctx, drawID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get draw: %w", err)
	}

	// Validate draw is in scheduled status
	if draw.Status != models.DrawStatusScheduled {
		return nil, fmt.Errorf("draw must be in scheduled status to start preparation, current status: %s", draw.Status)
	}

	// Initialize stage data if not exists
	if draw.StageData == nil {
		draw.StageData = &models.DrawStage{
			CurrentStage: 1,
			StageName:    "Preparation",
			StageStatus:  models.StageStatusInProgress,
		}
	} else {
		// Validate we can start this stage
		if !draw.StageData.CanStartStage(1) {
			return nil, fmt.Errorf("cannot start preparation stage, current stage: %d, status: %s",
				draw.StageData.CurrentStage, draw.StageData.StageStatus)
		}
		draw.StageData.CurrentStage = 1
		draw.StageData.StageName = "Preparation"
		draw.StageData.StageStatus = models.StageStatusInProgress
	}

	now := time.Now()
	draw.StageData.StageStartedAt = &now

	// Initialize preparation data
	draw.StageData.PreparationData = &models.PreparationStageData{
		TicketsLocked: 0,
		TotalStakes:   0,
		SalesLocked:   false,
		LockTime:      nil,
	}

	// Update draw status to in_progress
	draw.Status = models.DrawStatusInProgress

	// Update the draw
	if err := s.drawRepo.Update(ctx, draw); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update draw: %w", err)
	}

	s.logger.Printf("Draw preparation started: draw_id=%s, initiated_by=%s", drawID, initiatedBy)
	span.SetAttributes(attribute.String("result", "preparation_started"))
	return draw, nil
}

// CompleteDrawPreparation completes the preparation stage
func (s *drawService) CompleteDrawPreparation(ctx context.Context, drawID uuid.UUID, completedBy string) (*models.Draw, error) {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "DrawService.CompleteDrawPreparation")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw_id", drawID.String()),
		attribute.String("completed_by", completedBy),
	)

	// Get the draw
	draw, err := s.drawRepo.GetByID(ctx, drawID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get draw: %w", err)
	}

	// Validate stage data exists
	if draw.StageData == nil || draw.StageData.CurrentStage != 1 {
		return nil, fmt.Errorf("draw is not in preparation stage")
	}

	// Validate stage is in progress
	if draw.StageData.StageStatus != models.StageStatusInProgress {
		return nil, fmt.Errorf("preparation stage is not in progress, current status: %s", draw.StageData.StageStatus)
	}

	// Fetch ticket statistics from Ticket Service using draw_id
	ticketClient, err := s.grpcClientManager.TicketServiceClient(ctx)
	if err != nil {
		s.logger.Printf("Error getting ticket service client: %v", err)
		// Continue without statistics
	} else {
		client, ok := ticketClient.(ticketv1.TicketServiceClient)
		if ok {
			// Filter by game_schedule_id so tickets created before the draw record
			// was inserted (which have draw_id=NULL) are also counted.
			ticketFilter := &ticketv1.TicketFilter{}
			if draw.GameScheduleID != uuid.Nil {
				ticketFilter.GameScheduleId = draw.GameScheduleID.String()
			} else {
				ticketFilter.DrawId = drawID.String()
			}
			listResp, err := client.ListTickets(ctx, &ticketv1.ListTicketsRequest{
				Filter:   ticketFilter,
				PageSize: 10000,
			})
			if err != nil {
				s.logger.Printf("Error fetching ticket statistics: %v", err)
			} else {
				// Calculate totals
				var ticketCount int64
				var totalStakes int64
				for _, ticket := range listResp.Tickets {
					ticketCount++
					totalStakes += ticket.TotalAmount
				}

				draw.StageData.PreparationData.TicketsLocked = ticketCount
				draw.StageData.PreparationData.TotalStakes = totalStakes

				// Also update draw-level statistics
				draw.TotalTicketsSold = ticketCount
				draw.TotalPrizePool = totalStakes

				s.logger.Printf("Fetched ticket statistics: tickets=%d, stakes=%d pesewas", ticketCount, totalStakes)
			}
		}
	}

	now := time.Now()
	draw.StageData.PreparationData.SalesLocked = true
	draw.StageData.PreparationData.LockTime = &now

	// Mark stage as completed
	draw.StageData.StageStatus = models.StageStatusCompleted
	draw.StageData.StageCompletedAt = &now

	// Automatically transition to Stage 2: Physical Draw Recording
	draw.StageData.CurrentStage = 2
	draw.StageData.StageName = "Physical Draw Recording"
	draw.StageData.StageStatus = models.StageStatusInProgress
	draw.StageData.StageStartedAt = &now
	draw.StageData.StageCompletedAt = nil
	draw.StageData.NumberSelectionData = &models.NumberSelectionStageData{
		// Initialize empty - will be filled when numbers are recorded
		VerificationAttempts: []models.VerificationAttempt{},
		IsVerified:           false,
	}

	// Update the draw
	if err := s.drawRepo.Update(ctx, draw); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update draw: %w", err)
	}

	s.logger.Printf("Draw preparation completed: draw_id=%s, completed_by=%s, tickets=%d, stakes=%d",
		drawID, completedBy, draw.StageData.PreparationData.TicketsLocked, draw.StageData.PreparationData.TotalStakes)

	span.SetAttributes(
		attribute.String("result", "preparation_completed"),
		attribute.Int64("tickets_locked", draw.StageData.PreparationData.TicketsLocked),
		attribute.Int64("total_stakes", draw.StageData.PreparationData.TotalStakes),
	)

	return draw, nil
}

// ============================================================================
// Stage 2: Number Selection
// ============================================================================

// StartNumberSelection begins the number selection stage
func (s *drawService) StartNumberSelection(ctx context.Context, drawID uuid.UUID, initiatedBy string) (*models.Draw, error) {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "DrawService.StartNumberSelection")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw_id", drawID.String()),
		attribute.String("initiated_by", initiatedBy),
	)

	// Get the draw
	draw, err := s.drawRepo.GetByID(ctx, drawID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get draw: %w", err)
	}

	// Validate we're in preparation stage and it's completed
	if draw.StageData == nil || draw.StageData.CurrentStage != 1 {
		return nil, fmt.Errorf("draw must complete preparation stage first, current stage: %d", draw.StageData.CurrentStage)
	}

	if draw.StageData.StageStatus != models.StageStatusCompleted {
		return nil, fmt.Errorf("preparation stage must be completed first, current status: %s", draw.StageData.StageStatus)
	}

	// Move to stage 2
	now := time.Now()
	draw.StageData.CurrentStage = 2
	draw.StageData.StageName = "Number Selection"
	draw.StageData.StageStatus = models.StageStatusInProgress
	draw.StageData.StageStartedAt = &now
	draw.StageData.StageCompletedAt = nil

	// Initialize number selection data
	draw.StageData.NumberSelectionData = &models.NumberSelectionStageData{
		WinningNumbers:       []int32{},
		VerificationAttempts: []models.VerificationAttempt{},
		IsVerified:           false,
	}

	// Update the draw
	if err := s.drawRepo.Update(ctx, draw); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update draw: %w", err)
	}

	s.logger.Printf("Number selection started: draw_id=%s, initiated_by=%s", drawID, initiatedBy)
	span.SetAttributes(attribute.String("result", "number_selection_started"))
	return draw, nil
}

// SubmitVerificationAttempt records a verification attempt for winning numbers
func (s *drawService) SubmitVerificationAttempt(ctx context.Context, drawID uuid.UUID, numbers []int32, submittedBy string) (*models.Draw, int32, error) {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "DrawService.SubmitVerificationAttempt")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw_id", drawID.String()),
		attribute.String("submitted_by", submittedBy),
		attribute.Int("numbers_count", len(numbers)),
	)

	// Get the draw
	draw, err := s.drawRepo.GetByID(ctx, drawID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to get draw: %w", err)
	}

	// Validate we're in number selection stage
	if draw.StageData == nil || draw.StageData.CurrentStage != 2 {
		return nil, 0, fmt.Errorf("draw must be in number selection stage, current stage: %d", draw.StageData.CurrentStage)
	}

	if draw.StageData.StageStatus != models.StageStatusInProgress {
		return nil, 0, fmt.Errorf("number selection stage must be in progress, current status: %s", draw.StageData.StageStatus)
	}

	// Validate numbers (must be exactly 5 numbers for 5/90 game)
	if len(numbers) != 5 {
		return nil, 0, fmt.Errorf("must provide exactly 5 numbers, got %d", len(numbers))
	}

	// Validate numbers are in range 1-90 and unique
	seen := make(map[int32]bool)
	for _, num := range numbers {
		if num < 1 || num > 90 {
			return nil, 0, fmt.Errorf("all numbers must be between 1 and 90, got %d", num)
		}
		if seen[num] {
			return nil, 0, fmt.Errorf("duplicate number found: %d", num)
		}
		seen[num] = true
	}

	// Check if already verified
	if draw.StageData.NumberSelectionData.IsVerified {
		return nil, 0, fmt.Errorf("winning numbers already verified and finalized")
	}

	// Check attempt count (max 3 attempts allowed for triple-entry validation)
	attemptCount := len(draw.StageData.NumberSelectionData.VerificationAttempts)
	if attemptCount >= 3 {
		return nil, 0, fmt.Errorf("maximum verification attempts (3) already reached")
	}

	// Create new verification attempt
	now := time.Now()
	newAttempt := models.VerificationAttempt{
		AttemptNumber: int32(attemptCount + 1),
		Numbers:       numbers,
		SubmittedBy:   submittedBy,
		SubmittedAt:   now,
	}

	draw.StageData.NumberSelectionData.VerificationAttempts = append(
		draw.StageData.NumberSelectionData.VerificationAttempts,
		newAttempt,
	)

	// Update the draw
	if err := s.drawRepo.Update(ctx, draw); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to update draw: %w", err)
	}

	s.logger.Printf("Verification attempt submitted: draw_id=%s, attempt=%d, submitted_by=%s",
		drawID, newAttempt.AttemptNumber, submittedBy)

	span.SetAttributes(
		attribute.String("result", "attempt_submitted"),
		attribute.Int("attempt_number", int(newAttempt.AttemptNumber)),
	)

	// If this is the 3rd attempt, automatically validate all attempts
	if newAttempt.AttemptNumber == 3 {
		s.logger.Printf("Third attempt submitted, automatically validating: draw_id=%s", drawID)

		// Reload draw to get latest data
		draw, err = s.drawRepo.GetByID(ctx, drawID)
		if err != nil {
			s.logger.Printf("Failed to reload draw for validation: %v", err)
			return draw, newAttempt.AttemptNumber, nil // Return success for submission, validation can be done manually
		}

		// Validate the 3 attempts
		isValid, winningNumbers, validationError, err := s.ValidateVerificationAttempts(ctx, drawID, submittedBy)
		if err != nil {
			s.logger.Printf("Auto-validation failed with error: %v", err)
			return draw, newAttempt.AttemptNumber, nil // Return success for submission
		}

		if !isValid {
			s.logger.Printf("Auto-validation failed: %s", validationError)
			return draw, newAttempt.AttemptNumber, nil // Return success for submission, numbers don't match
		}

		s.logger.Printf("Auto-validation successful: draw_id=%s, winning_numbers=%v", drawID, winningNumbers)

		// Reload draw to return updated data with verified numbers
		draw, err = s.drawRepo.GetByID(ctx, drawID)
		if err != nil {
			s.logger.Printf("Failed to reload draw after validation: %v", err)
		}
	}

	return draw, newAttempt.AttemptNumber, nil
}

// ValidateVerificationAttempts checks if all 3 attempts match and validates them
func (s *drawService) ValidateVerificationAttempts(ctx context.Context, drawID uuid.UUID, validatedBy string) (bool, []int32, string, error) {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "DrawService.ValidateVerificationAttempts")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw_id", drawID.String()),
		attribute.String("validated_by", validatedBy),
	)

	// Get the draw
	draw, err := s.drawRepo.GetByID(ctx, drawID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return false, nil, "", fmt.Errorf("failed to get draw: %w", err)
	}

	// Validate we're in number selection stage
	if draw.StageData == nil || draw.StageData.CurrentStage != 2 {
		return false, nil, "", fmt.Errorf("draw must be in number selection stage")
	}

	// Check we have exactly 3 attempts (triple-entry validation)
	attempts := draw.StageData.NumberSelectionData.VerificationAttempts
	if len(attempts) != 3 {
		return false, nil, fmt.Sprintf("need exactly 3 verification attempts, currently have %d", len(attempts)), nil
	}

	// Compare all three attempts - they must all match
	attempt1 := attempts[0].Numbers
	attempt2 := attempts[1].Numbers
	attempt3 := attempts[2].Numbers

	// Log all attempts for debugging
	s.logger.Printf("Validating verification attempts: draw_id=%s", drawID)
	s.logger.Printf("  Attempt 1: %v", attempt1)
	s.logger.Printf("  Attempt 2: %v", attempt2)
	s.logger.Printf("  Attempt 3: %v", attempt3)

	if !numbersMatch(attempt1, attempt2) || !numbersMatch(attempt1, attempt3) {
		errorMsg := "verification attempts do not match - numbers differ"
		s.logger.Printf("Validation failed: draw_id=%s, reason=%s", drawID, errorMsg)
		s.logger.Printf("  Attempt 1 vs 2 match: %v", numbersMatch(attempt1, attempt2))
		s.logger.Printf("  Attempt 1 vs 3 match: %v", numbersMatch(attempt1, attempt3))
		span.SetAttributes(
			attribute.String("result", "validation_failed"),
			attribute.String("reason", "numbers_mismatch"),
		)
		return false, nil, errorMsg, nil
	}

	// All 3 attempts match - mark as verified
	now := time.Now()
	draw.StageData.NumberSelectionData.IsVerified = true
	draw.StageData.NumberSelectionData.VerifiedBy = validatedBy
	draw.StageData.NumberSelectionData.VerifiedAt = &now
	draw.StageData.NumberSelectionData.WinningNumbers = attempt1 // Use first attempt as verified numbers

	// Set winning numbers at draw level for compatibility with CommitResults
	draw.WinningNumbers = attempt1

	// Complete Stage 2
	draw.StageData.StageStatus = models.StageStatusCompleted
	draw.StageData.StageCompletedAt = &now

	// Automatically calculate winners and move to Stage 3
	s.logger.Printf("Stage 2 completed, automatically calculating winners for Stage 3: draw_id=%s", drawID)

	// Move to Stage 3 and calculate results
	draw.StageData.CurrentStage = 3
	draw.StageData.StageName = "Result Calculation"
	draw.StageData.StageStatus = models.StageStatusInProgress
	stageStartedAt := time.Now()
	draw.StageData.StageStartedAt = &stageStartedAt
	draw.StageData.StageCompletedAt = nil

	// Calculate winners using bet rules engine
	s.logger.Printf("Starting result calculation: draw_id=%s, winning_numbers=%v", drawID, draw.WinningNumbers)

	// Initialize bet rules engine
	betEngine := NewBetRulesEngine()

	// Prepare result calculation data
	var winningTicketsCount int64
	var totalWinnings int64
	winningTierMap := make(map[string]*models.WinningTier)
	var winningTickets []models.WinningTicketDetail

	// Fetch tickets from Ticket Service using draw_id for direct filtering
	s.logger.Printf("=== STAGE 2 COMPLETE -> STARTING STAGE 3 RESULT CALCULATION ===")
	s.logger.Printf("[SERVICE] Draw ID: %s", drawID)
	s.logger.Printf("[SERVICE] Winning Numbers: %v", draw.WinningNumbers)
	s.logger.Printf("[SERVICE] Game ID: %s", draw.GameID)
	s.logger.Printf("[SERVICE] Attempting to fetch ticket service client...")

	ticketClient, err := s.grpcClientManager.TicketServiceClient(ctx)
	if err != nil {
		s.logger.Printf("[SERVICE] CRITICAL ERROR: Failed to get ticket service client: %v - CANNOT CALCULATE WINNERS!", err)
		span.SetAttributes(attribute.String("critical_error", "ticket_client_failed"))
	} else if client, ok := ticketClient.(ticketv1.TicketServiceClient); ok {
		s.logger.Printf("[SERVICE] SUCCESS: Got ticket service client successfully")

		// Use GameScheduleId filter to catch ALL tickets (draw_id may be NULL for tickets
		// created before the draw was prepared). Fall back to draw_id if no schedule ID.
		ticketFilter := &ticketv1.TicketFilter{Status: "issued"}
		if draw.GameScheduleID != uuid.Nil {
			s.logger.Printf("[GRPC] Fetching tickets by game_schedule_id: %s", draw.GameScheduleID.String())
			ticketFilter.GameScheduleId = draw.GameScheduleID.String()
		} else {
			s.logger.Printf("[GRPC] No game_schedule_id on draw, falling back to draw_id: %s", drawID.String())
			ticketFilter.DrawId = drawID.String()
		}

		// First probe to get total count
		probeResp, err := client.ListTickets(ctx, &ticketv1.ListTicketsRequest{
			Filter:   ticketFilter,
			PageSize: 1,
		})
		var allTickets []*ticketv1.Ticket
		if err != nil {
			s.logger.Printf("[GRPC] CRITICAL ERROR: ListTickets probe failed: %v - CANNOT CALCULATE WINNERS!", err)
			span.SetAttributes(attribute.String("critical_error", fmt.Sprintf("list_tickets_failed: %v", err)))
		} else {
			totalTickets := probeResp.Total
			s.logger.Printf("[GRPC] Total tickets for draw: %d", totalTickets)
			if totalTickets > 0 {
				fullResp, err2 := client.ListTickets(ctx, &ticketv1.ListTicketsRequest{
					Filter:   ticketFilter,
					PageSize: int32(totalTickets) + 10,
				})
				if err2 != nil {
					s.logger.Printf("[GRPC] CRITICAL ERROR: ListTickets full fetch failed: %v", err2)
				} else {
					allTickets = fullResp.Tickets
					s.logger.Printf("[GRPC] Fetched %d tickets for result calculation", len(allTickets))
				}
			}
		}

		// Wrap in a compatible response struct so the rest of the code is unchanged
		listResp := &ticketv1.GetAllTicketsForDrawResponse{Tickets: allTickets}
		if len(allTickets) == 0 {
			s.logger.Printf("[GRPC] WARNING: No tickets returned for draw %s (schedule=%s)", drawID, draw.GameScheduleID)
		} else {
			s.logger.Printf("[GRPC] Ticket details from response:")
			for idx, t := range allTickets {
				s.logger.Printf("[GRPC]   Ticket %d: ID=%s, Serial=%s, IssuerID=%s, TotalAmount=%d, BetLines=%d",
					idx+1, t.Id, t.SerialNumber, t.IssuerId, t.TotalAmount, len(t.BetLines))
			}
		}

		// Process each ticket
		s.logger.Printf("=== PROCESSING TICKETS FOR WINNING CALCULATION ===")
		s.logger.Printf("[SERVICE] Total tickets to process: %d", len(listResp.Tickets))
		s.logger.Printf("[SERVICE] Winning numbers to match against: %v", draw.WinningNumbers)
		s.logger.Printf("[SERVICE] Starting ticket-by-ticket processing...")

		// Initialize arrays for batch ticket status updates (to avoid N+1 problem)
		var winningUpdates []*ticketv1.TicketStatusUpdate
		var losingUpdates []*ticketv1.TicketStatusUpdate
		s.logger.Printf("[SERVICE] Batch update arrays initialized")

		for i, ticket := range listResp.Tickets {
			s.logger.Printf("\n[SERVICE] ========================================")
			s.logger.Printf("[SERVICE] Processing ticket %d/%d", i+1, len(listResp.Tickets))
			s.logger.Printf("[SERVICE] Ticket ID: %s", ticket.Id)
			s.logger.Printf("[SERVICE] Serial Number: %s", ticket.SerialNumber)
			s.logger.Printf("[SERVICE] Issuer ID: %s", ticket.IssuerId)
			s.logger.Printf("[SERVICE] Total Amount: %d pesewas", ticket.TotalAmount)
			s.logger.Printf("[SERVICE] Bet Lines Count: %d", len(ticket.BetLines))

			// DEBUG: Log proto bet lines BEFORE conversion
			s.logger.Printf("[GRPC] Proto bet lines from gRPC response (BEFORE conversion):")
			for idx, pbl := range ticket.BetLines {
				s.logger.Printf("[GRPC]   Line %d: BetType=%q, SelectedNumbers=%v, TotalAmount=%d",
					idx+1, pbl.BetType, pbl.SelectedNumbers, pbl.TotalAmount)
			}

			// Convert ticket bet lines to engine format
			s.logger.Printf("[SERVICE] Converting proto bet lines to engine format...")
			betLines := convertTicketBetLines(ticket.BetLines)
			s.logger.Printf("[SERVICE] Conversion completed - %d bet lines converted", len(betLines))

			// DEBUG: Log converted bet lines AFTER conversion
			s.logger.Printf("[SERVICE] Converted bet lines for engine (AFTER conversion):")
			for idx, bl := range betLines {
				s.logger.Printf("[SERVICE]   Line %d: BetType=%q, SelectedNumbers=%v, TotalAmount=%d",
					idx+1, bl.BetType, bl.SelectedNumbers, bl.TotalAmount)
			}

			// Calculate winnings for this ticket
			s.logger.Printf("[SERVICE] Calling CalculateWinnings engine...")
			s.logger.Printf("[SERVICE] Input: BetLines=%d, WinningNumbers=%v", len(betLines), draw.WinningNumbers)
			ticketWinnings, results, err := betEngine.CalculateWinnings(betLines, draw.WinningNumbers)
			if err != nil {
				s.logger.Printf("[SERVICE] ERROR: CalculateWinnings failed for ticket %s: %v", ticket.Id, err)
				continue
			}

			s.logger.Printf("[SERVICE] CalculateWinnings completed successfully")
			s.logger.Printf("[SERVICE] Total ticket winnings: %d pesewas (%.2f GHS)", ticketWinnings, float64(ticketWinnings)/100)
			s.logger.Printf("[SERVICE] Result details count: %d", len(results))

			// Log each bet line result
			if len(results) > 0 {
				s.logger.Printf("[SERVICE] Detailed results per bet line:")
				for ridx, result := range results {
					s.logger.Printf("[SERVICE]   Result %d:", ridx+1)
					s.logger.Printf("[SERVICE]     BetType: %s", result.BetLine.BetType)
					s.logger.Printf("[SERVICE]     SelectedNumbers: %v", result.BetLine.SelectedNumbers)
					s.logger.Printf("[SERVICE]     MatchedCount: %d", result.MatchedCount)
					s.logger.Printf("[SERVICE]     IsWinner: %t", result.IsWinner)
					s.logger.Printf("[SERVICE]     WinningAmount: %d pesewas (%.2f GHS)", result.WinningAmount, float64(result.WinningAmount)/100)
				}
			}

			if ticketWinnings > 0 {
				winningTicketsCount++
				totalWinnings += ticketWinnings

				s.logger.Printf("[SERVICE] *** WINNING TICKET DETECTED ***")
				s.logger.Printf("[SERVICE] Ticket ID: %s", ticket.Id)
				s.logger.Printf("[SERVICE] Serial: %s", ticket.SerialNumber)
				s.logger.Printf("[SERVICE] Total winnings: %d pesewas (%.2f GHS)", ticketWinnings, float64(ticketWinnings)/100)
				s.logger.Printf("[SERVICE] Current total winning tickets count: %d", winningTicketsCount)
				s.logger.Printf("[SERVICE] Current cumulative winnings: %d pesewas (%.2f GHS)", totalWinnings, float64(totalWinnings)/100)

				// Count how many numbers matched
				var matchesCount int32
				var primaryBetType string
				s.logger.Printf("[SERVICE] Analyzing winning results to find primary bet type and max matches...")
				for _, result := range results {
					if result.IsWinner && int32(result.MatchedCount) > matchesCount {
						matchesCount = int32(result.MatchedCount)
						primaryBetType = result.BetLine.BetType
						s.logger.Printf("[SERVICE]   Updated max matches: %d, BetType: %s", matchesCount, primaryBetType)
					}
				}

				// Create winning ticket detail
				isBigWin := ticketWinnings > 2500000 // > 25,000 GHS
				s.logger.Printf("[SERVICE] Creating winning ticket detail:")
				s.logger.Printf("[SERVICE]   Primary BetType: %s", primaryBetType)
				s.logger.Printf("[SERVICE]   Matches Count: %d", matchesCount)
				s.logger.Printf("[SERVICE]   Stake Amount: %d pesewas", ticket.TotalAmount)
				s.logger.Printf("[SERVICE]   Is Big Win (>25k GHS): %t", isBigWin)

				// Get selected numbers safely (with bounds check)
				var selectedNumbers []int32
				if len(ticket.BetLines) > 0 {
					selectedNumbers = ticket.BetLines[0].SelectedNumbers
				} else {
					s.logger.Printf("[SERVICE] WARNING: Ticket %s has no bet lines, using empty numbers array", ticket.Id)
					selectedNumbers = []int32{}
				}

				winningTicket := models.WinningTicketDetail{
					TicketID:      ticket.Id,
					SerialNumber:  ticket.SerialNumber,
					RetailerID:    ticket.IssuerId,
					Numbers:       selectedNumbers,
					BetType:       primaryBetType,
					StakeAmount:   ticket.TotalAmount,
					WinningAmount: ticketWinnings,
					MatchesCount:  matchesCount,
					IsBigWin:      isBigWin,
				}
				winningTickets = append(winningTickets, winningTicket)
				s.logger.Printf("[SERVICE] Winning ticket added to results list")

				// Aggregate by bet type
				s.logger.Printf("[SERVICE] Aggregating winnings by bet type for reporting...")
				for _, result := range results {
					if result.IsWinner {
						betType := result.BetLine.BetType
						if tier, exists := winningTierMap[betType]; exists {
							s.logger.Printf("[SERVICE]   Updating existing tier %s: current_count=%d, adding_amount=%d",
								betType, tier.WinnersCount, result.WinningAmount)
							tier.WinnersCount++
							tier.TotalAmount += result.WinningAmount
						} else {
							s.logger.Printf("[SERVICE]   Creating new tier %s: initial_count=1, amount=%d",
								betType, result.WinningAmount)
							winningTierMap[betType] = &models.WinningTier{
								BetType:      betType,
								WinnersCount: 1,
								TotalAmount:  result.WinningAmount,
							}
						}
					}
				}

				// Add to batch update array for winning tickets (avoiding N+1 problem)
				s.logger.Printf("[SERVICE] Adding ticket to winning updates batch...")
				winningUpdates = append(winningUpdates, &ticketv1.TicketStatusUpdate{
					TicketId:      ticket.Id,
					Status:        "won",
					WinningAmount: ticketWinnings,
					Matches:       matchesCount,
					PrizeTier:     primaryBetType,
				})
				s.logger.Printf("[SERVICE] Ticket %s queued for batch 'won' update", ticket.Id)
			} else {
				s.logger.Printf("[SERVICE] Ticket %s is NOT a winner (winnings=0)", ticket.Id)

				// Add to batch update array for losing tickets (avoiding N+1 problem)
				s.logger.Printf("[SERVICE] Adding ticket to losing updates batch...")
				losingUpdates = append(losingUpdates, &ticketv1.TicketStatusUpdate{
					TicketId:      ticket.Id,
					Status:        "lost",
					WinningAmount: 0,
					Matches:       0,
					PrizeTier:     "",
				})
				s.logger.Printf("[SERVICE] Ticket %s queued for batch 'lost' update", ticket.Id)
			}
		}

		s.logger.Printf("\n[SERVICE] ========================================")
		s.logger.Printf("[SERVICE] TICKET PROCESSING COMPLETED")
		s.logger.Printf("[SERVICE] Total tickets processed: %d", len(listResp.Tickets))
		s.logger.Printf("[SERVICE] Total winning tickets: %d", winningTicketsCount)
		s.logger.Printf("[SERVICE] Total winnings amount: %d pesewas (%.2f GHS)", totalWinnings, float64(totalWinnings)/100)
		s.logger.Printf("[SERVICE] ========================================\n")

		// Batch update ticket statuses via Ticket Service (to avoid N+1 problem)
		s.logger.Printf("\n[SERVICE] ========================================")
		s.logger.Printf("[SERVICE] EXECUTING BATCH TICKET STATUS UPDATES")
		s.logger.Printf("[SERVICE] Winning tickets to update: %d", len(winningUpdates))
		s.logger.Printf("[SERVICE] Losing tickets to update: %d", len(losingUpdates))
		s.logger.Printf("[SERVICE] ========================================\n")

		// Batch update winning tickets
		if len(winningUpdates) > 0 {
			s.logger.Printf("[SERVICE] Calling UpdateTicketStatuses for %d WINNING tickets...", len(winningUpdates))
			batchResp, err := client.UpdateTicketStatuses(ctx, &ticketv1.UpdateTicketStatusesRequest{
				Updates: winningUpdates,
				DrawId:  drawID.String(),
			})
			if err != nil {
				s.logger.Printf("[SERVICE] CRITICAL ERROR: Batch update for winning tickets failed: %v", err)
				span.SetAttributes(attribute.String("batch_update_winners_error", err.Error()))
				// TODO: Consider marking draw as failed or creating compensation record
			} else {
				s.logger.Printf("[SERVICE] Batch update for winning tickets completed:")
				s.logger.Printf("[SERVICE]   Total requested: %d", batchResp.TotalRequested)
				s.logger.Printf("[SERVICE]   Successful: %d", batchResp.Successful)
				s.logger.Printf("[SERVICE]   Failed: %d", batchResp.Failed)
				s.logger.Printf("[SERVICE]   Message: %s", batchResp.Message)

				// Log failures if any
				if batchResp.Failed > 0 {
					s.logger.Printf("[SERVICE] WARNING: Some winning tickets failed to update:")
					for _, result := range batchResp.Results {
						if !result.Success {
							s.logger.Printf("[SERVICE]   - Ticket %s: %s", result.TicketId, result.Message)
						}
					}
					span.SetAttributes(attribute.Int64("winning_tickets_update_failures", batchResp.Failed))
				}
			}
		}

		// Batch update losing tickets
		if len(losingUpdates) > 0 {
			s.logger.Printf("[SERVICE] Calling UpdateTicketStatuses for %d LOSING tickets...", len(losingUpdates))
			batchResp, err := client.UpdateTicketStatuses(ctx, &ticketv1.UpdateTicketStatusesRequest{
				Updates: losingUpdates,
				DrawId:  drawID.String(),
			})
			if err != nil {
				s.logger.Printf("[SERVICE] CRITICAL ERROR: Batch update for losing tickets failed: %v", err)
				span.SetAttributes(attribute.String("batch_update_losers_error", err.Error()))
				// TODO: Consider marking draw as failed or creating compensation record
			} else {
				s.logger.Printf("[SERVICE] Batch update for losing tickets completed:")
				s.logger.Printf("[SERVICE]   Total requested: %d", batchResp.TotalRequested)
				s.logger.Printf("[SERVICE]   Successful: %d", batchResp.Successful)
				s.logger.Printf("[SERVICE]   Failed: %d", batchResp.Failed)
				s.logger.Printf("[SERVICE]   Message: %s", batchResp.Message)

				// Log failures if any
				if batchResp.Failed > 0 {
					s.logger.Printf("[SERVICE] WARNING: Some losing tickets failed to update:")
					for _, result := range batchResp.Results {
						if !result.Success {
							s.logger.Printf("[SERVICE]   - Ticket %s: %s", result.TicketId, result.Message)
						}
					}
					span.SetAttributes(attribute.Int64("losing_tickets_update_failures", batchResp.Failed))
				}
			}
		}

		s.logger.Printf("[SERVICE] ========================================")
		s.logger.Printf("[SERVICE] BATCH STATUS UPDATES COMPLETED")
		s.logger.Printf("[SERVICE] ========================================\n")
	}

	// Convert tier map to slice
	s.logger.Printf("[SERVICE] ========================================")
	s.logger.Printf("[SERVICE] AGGREGATING WINNING TIERS")
	var winningTiers []models.WinningTier
	for betType, tier := range winningTierMap {
		s.logger.Printf("[SERVICE] Tier %s: Winners=%d, TotalAmount=%d pesewas (%.2f GHS)",
			betType, tier.WinnersCount, tier.TotalAmount, float64(tier.TotalAmount)/100)
		winningTiers = append(winningTiers, *tier)
	}
	s.logger.Printf("[SERVICE] Total tiers created: %d", len(winningTiers))
	s.logger.Printf("[SERVICE] ========================================\n")

	// Initialize result calculation data (draft state - not yet committed)
	calculatedAt := time.Now()
	s.logger.Printf("[SERVICE] ========================================")
	s.logger.Printf("[SERVICE] INITIALIZING STAGE 3 RESULT CALCULATION DATA")
	s.logger.Printf("[SERVICE] Calculation timestamp: %s", calculatedAt.Format(time.RFC3339))
	s.logger.Printf("[SERVICE] Winning tickets count: %d", winningTicketsCount)
	s.logger.Printf("[SERVICE] Total winnings: %d pesewas (%.2f GHS)", totalWinnings, float64(totalWinnings)/100)
	s.logger.Printf("[SERVICE] Winning tiers count: %d", len(winningTiers))
	s.logger.Printf("[SERVICE] Winning tickets details count: %d", len(winningTickets))

	draw.StageData.ResultCalculationData = &models.ResultCalculationStageData{
		WinningTicketsCount: winningTicketsCount,
		TotalWinnings:       totalWinnings,
		WinningTiers:        winningTiers,
		WinningTickets:      winningTickets,
		CalculatedAt:        &calculatedAt,
	}
	s.logger.Printf("[SERVICE] Result calculation data initialized successfully")
	s.logger.Printf("[SERVICE] ========================================\n")

	// Keep Stage 3 in progress (draft state) - NOT completed yet
	// Stage 3 will be completed when CommitResults is called
	s.logger.Printf("[SERVICE] NOTE: Stage 3 remains in IN_PROGRESS state (draft)")
	s.logger.Printf("[SERVICE] Stage 3 will be completed when CommitResults is called")

	// Update draw totals
	s.logger.Printf("[SERVICE] Updating draw totals:")
	s.logger.Printf("[SERVICE]   TotalPrizePool: %d pesewas (%.2f GHS)", totalWinnings, float64(totalWinnings)/100)
	draw.TotalPrizePool = totalWinnings

	// Update the draw
	s.logger.Printf("[SERVICE] ========================================")
	s.logger.Printf("[SERVICE] SAVING DRAW TO DATABASE")
	s.logger.Printf("[SERVICE] Calling repository Update method...")
	if err := s.drawRepo.Update(ctx, draw); err != nil {
		s.logger.Printf("[SERVICE] CRITICAL ERROR: Failed to save draw to database: %v", err)
		span.SetAttributes(attribute.String("error", err.Error()))
		return false, nil, "", fmt.Errorf("failed to update draw: %w", err)
	}
	s.logger.Printf("[SERVICE] Draw saved to database successfully")
	s.logger.Printf("[SERVICE] ========================================\n")

	s.logger.Printf("[SERVICE] *** STAGE 2 VERIFICATION COMPLETE ***")
	s.logger.Printf("[SERVICE] Draw ID: %s", drawID)
	s.logger.Printf("[SERVICE] Winning Numbers: %v", attempt1)
	s.logger.Printf("[SERVICE] Validated By: %s", validatedBy)
	s.logger.Printf("[SERVICE] Winning Tickets: %d", winningTicketsCount)
	s.logger.Printf("[SERVICE] Total Winnings: %d pesewas (%.2f GHS)", totalWinnings, float64(totalWinnings)/100)
	s.logger.Printf("[SERVICE] Stage 3 Status: IN_PROGRESS (awaiting commit)")

	span.SetAttributes(
		attribute.String("result", "validation_success"),
		attribute.IntSlice("winning_numbers", int32SliceToIntSlice(attempt1)),
		attribute.String("stage_status", "completed"),
	)

	return true, attempt1, "", nil
}

// ResetVerificationAttempts clears all verification attempts (when they don't match)
func (s *drawService) ResetVerificationAttempts(ctx context.Context, drawID uuid.UUID, resetBy, reason string) (*models.Draw, error) {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "DrawService.ResetVerificationAttempts")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw_id", drawID.String()),
		attribute.String("reset_by", resetBy),
		attribute.String("reason", reason),
	)

	// Get the draw
	draw, err := s.drawRepo.GetByID(ctx, drawID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get draw: %w", err)
	}

	// Validate we're in number selection stage
	if draw.StageData == nil || draw.StageData.CurrentStage != 2 {
		return nil, fmt.Errorf("draw must be in number selection stage")
	}

	// Clear verification attempts
	draw.StageData.NumberSelectionData.VerificationAttempts = []models.VerificationAttempt{}
	draw.StageData.NumberSelectionData.IsVerified = false
	draw.StageData.NumberSelectionData.VerifiedBy = ""
	draw.StageData.NumberSelectionData.VerifiedAt = nil
	draw.StageData.NumberSelectionData.WinningNumbers = []int32{}

	// Update the draw
	if err := s.drawRepo.Update(ctx, draw); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update draw: %w", err)
	}

	s.logger.Printf("Verification attempts reset: draw_id=%s, reset_by=%s, reason=%s",
		drawID, resetBy, reason)

	span.SetAttributes(attribute.String("result", "attempts_reset"))
	return draw, nil
}

// CompleteNumberSelection finalizes the number selection stage
func (s *drawService) CompleteNumberSelection(ctx context.Context, drawID uuid.UUID, winningNumbers []int32, completedBy string) (*models.Draw, error) {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "DrawService.CompleteNumberSelection")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw_id", drawID.String()),
		attribute.String("completed_by", completedBy),
	)

	// Get the draw
	draw, err := s.drawRepo.GetByID(ctx, drawID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get draw: %w", err)
	}

	// Validate we're in number selection stage
	if draw.StageData == nil || draw.StageData.CurrentStage != 2 {
		return nil, fmt.Errorf("draw must be in number selection stage")
	}

	// Validate numbers are verified
	if !draw.StageData.NumberSelectionData.IsVerified {
		return nil, fmt.Errorf("winning numbers must be verified before completing this stage")
	}

	// Validate provided numbers match verified numbers
	if !numbersMatch(winningNumbers, draw.StageData.NumberSelectionData.WinningNumbers) {
		return nil, fmt.Errorf("provided numbers do not match verified winning numbers")
	}

	// Mark stage as completed
	now := time.Now()
	draw.StageData.StageStatus = models.StageStatusCompleted
	draw.StageData.StageCompletedAt = &now

	// Update main draw winning numbers
	draw.WinningNumbers = winningNumbers

	// Update the draw
	if err := s.drawRepo.Update(ctx, draw); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update draw: %w", err)
	}

	s.logger.Printf("Number selection completed: draw_id=%s, winning_numbers=%v, completed_by=%s",
		drawID, winningNumbers, completedBy)

	span.SetAttributes(
		attribute.String("result", "number_selection_completed"),
		attribute.IntSlice("winning_numbers", int32SliceToIntSlice(winningNumbers)),
	)

	return draw, nil
}

// Helper functions

// numbersMatch checks if two slices of numbers are exactly equal (same numbers in same order)
func numbersMatch(nums1, nums2 []int32) bool {
	if len(nums1) != len(nums2) {
		return false
	}

	// Compare numbers in exact order - order matters for triple-entry validation
	for i := range nums1 {
		if nums1[i] != nums2[i] {
			return false
		}
	}

	return true
}

// int32SliceToIntSlice converts []int32 to []int for OpenTelemetry
func int32SliceToIntSlice(nums []int32) []int {
	result := make([]int, len(nums))
	for i, num := range nums {
		result[i] = int(num)
	}
	return result
}

// ============================================================================
// Stage 3: Result Calculation
// ============================================================================

// CommitResults commits the already calculated winning results (from Stage 2 verification)
func (s *drawService) CommitResults(ctx context.Context, drawID uuid.UUID, committedBy string) (*models.Draw, *ResultCalculationSummary, error) {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "DrawService.CommitResults")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw_id", drawID.String()),
		attribute.String("committed_by", committedBy),
	)

	// Get the draw
	draw, err := s.drawRepo.GetByID(ctx, drawID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, nil, fmt.Errorf("failed to get draw: %w", err)
	}

	// Validate we're in Stage 3 (Result Calculation) and it's in progress (draft state)
	if draw.StageData == nil || draw.StageData.CurrentStage != 3 {
		return nil, nil, fmt.Errorf("draw must be in result calculation stage, current stage: %d", draw.StageData.CurrentStage)
	}

	if draw.StageData.StageStatus != models.StageStatusInProgress {
		return nil, nil, fmt.Errorf("result calculation stage must be in progress (draft state), current status: %s", draw.StageData.StageStatus)
	}

	// Validate that calculation data exists (should have been calculated at end of Stage 2)
	if draw.StageData.ResultCalculationData == nil {
		return nil, nil, fmt.Errorf("no calculation data found - winners should have been calculated at the end of Stage 2")
	}

	// Validate calculation data integrity
	calcData := draw.StageData.ResultCalculationData

	// Check for invalid winning tickets count
	if calcData.WinningTicketsCount < 0 {
		return nil, nil, fmt.Errorf("invalid winning tickets count: %d (must be non-negative)", calcData.WinningTicketsCount)
	}

	// Check for invalid total winnings
	if calcData.TotalWinnings < 0 {
		return nil, nil, fmt.Errorf("invalid total winnings: %d pesewas (must be non-negative)", calcData.TotalWinnings)
	}

	// Validate consistency: if there are winning tickets, total winnings should be > 0 and vice versa
	if calcData.WinningTicketsCount > 0 && calcData.TotalWinnings == 0 {
		return nil, nil, fmt.Errorf("inconsistent calculation data: %d winning tickets but total winnings is 0", calcData.WinningTicketsCount)
	}

	if calcData.WinningTicketsCount == 0 && calcData.TotalWinnings > 0 {
		return nil, nil, fmt.Errorf("inconsistent calculation data: 0 winning tickets but total winnings is %d pesewas", calcData.TotalWinnings)
	}

	// Validate winning tiers sum matches total
	if len(calcData.WinningTiers) > 0 {
		var tierTotal int64
		var tierWinnersTotal int64
		for _, tier := range calcData.WinningTiers {
			tierTotal += tier.TotalAmount
			tierWinnersTotal += tier.WinnersCount
		}

		if tierTotal != calcData.TotalWinnings {
			return nil, nil, fmt.Errorf("winning tiers total (%d pesewas) does not match total winnings (%d pesewas)",
				tierTotal, calcData.TotalWinnings)
		}

		if tierWinnersTotal != calcData.WinningTicketsCount {
			return nil, nil, fmt.Errorf("winning tiers winners count (%d) does not match winning tickets count (%d)",
				tierWinnersTotal, calcData.WinningTicketsCount)
		}
	}

	s.logger.Printf("STAGE3_COMMIT_VALIDATION_PASSED: draw_id=%s, winning_tickets=%d, total_winnings=%d pesewas, tiers=%d",
		drawID, calcData.WinningTicketsCount, calcData.TotalWinnings, len(calcData.WinningTiers))

	// Mark Stage 3 as completed (finalize/commit the results)
	now := time.Now()
	draw.StageData.StageStatus = models.StageStatusCompleted
	draw.StageData.StageCompletedAt = &now

	// BUG FIX #3: Automatically transition to Stage 4 (Payout Processing)
	draw.StageData.CurrentStage = 4
	draw.StageData.StageName = draw.StageData.GetStageName(4)
	draw.StageData.StageStatus = models.StageStatusPending
	draw.StageData.StageStartedAt = &now
	draw.StageData.StageCompletedAt = nil // Reset completion time for new stage

	s.logger.Printf("STAGE3_TRANSITION_TO_STAGE4: draw_id=%s, stage_name=%s, status=%s",
		drawID, draw.StageData.StageName, draw.StageData.StageStatus)

	// Update the draw
	if err := s.drawRepo.Update(ctx, draw); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, nil, fmt.Errorf("failed to update draw: %w", err)
	}

	s.logger.Printf("Results committed successfully: draw_id=%s, committed_by=%s, winning_tickets=%d, total_winnings=%d pesewas",
		drawID, committedBy, draw.StageData.ResultCalculationData.WinningTicketsCount, draw.StageData.ResultCalculationData.TotalWinnings)

	span.SetAttributes(
		attribute.String("result", "results_committed"),
		attribute.Int64("winning_tickets", draw.StageData.ResultCalculationData.WinningTicketsCount),
		attribute.Int64("total_winnings", draw.StageData.ResultCalculationData.TotalWinnings),
	)

	summary := &ResultCalculationSummary{
		WinningTicketsCount: draw.StageData.ResultCalculationData.WinningTicketsCount,
		TotalWinnings:       draw.StageData.ResultCalculationData.TotalWinnings,
		WinningTiers:        draw.StageData.ResultCalculationData.WinningTiers,
	}

	return draw, summary, nil
}

// ============================================================================
// Stage 4: Payout Processing
// ============================================================================
// ProcessPayouts processes all winning ticket payouts using the robust PayoutProcessor
func (s *drawService) ProcessPayouts(ctx context.Context, drawID uuid.UUID, processedBy string) (*models.Draw, *PayoutSummary, error) {
	// Delegate to the robust payout processor
	return s.payoutProcessor.ProcessPayouts(ctx, drawID, processedBy)
}

// ProcessBigWinPayout approves or rejects a big win payout
func (s *drawService) ProcessBigWinPayout(ctx context.Context, drawID uuid.UUID, ticketID string, approve bool, processedBy, rejectionReason string) (*models.BigWinPayout, error) {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "DrawService.ProcessBigWinPayout")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw_id", drawID.String()),
		attribute.String("ticket_id", ticketID),
		attribute.Bool("approve", approve),
		attribute.String("processed_by", processedBy),
	)

	// Get the draw
	draw, err := s.drawRepo.GetByID(ctx, drawID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get draw: %w", err)
	}

	// Validate we're in payout stage
	if draw.StageData == nil || draw.StageData.CurrentStage != 4 {
		return nil, fmt.Errorf("draw must be in payout stage, current stage: %d", draw.StageData.CurrentStage)
	}

	// Find the big win payout
	var targetPayout *models.BigWinPayout
	for i := range draw.StageData.PayoutData.BigWinPayouts {
		if draw.StageData.PayoutData.BigWinPayouts[i].TicketID == ticketID {
			targetPayout = &draw.StageData.PayoutData.BigWinPayouts[i]
			break
		}
	}

	if targetPayout == nil {
		return nil, fmt.Errorf("big win payout not found for ticket: %s", ticketID)
	}

	// Check if already processed
	if targetPayout.Status != "pending" {
		return nil, fmt.Errorf("payout already processed with status: %s", targetPayout.Status)
	}

	now := time.Now()

	if approve {
		// Approve and process payout
		targetPayout.Status = "approved"
		targetPayout.ApprovedBy = processedBy
		targetPayout.ProcessedAt = &now

		// TODO: Credit wallet via Wallet Service
		// ticket, err := s.ticketClient.GetTicket(ctx, ticketID)
		// if err != nil {
		//     return nil, fmt.Errorf("failed to get ticket: %w", err)
		// }
		//
		// if ticket.IssuerType == "retailer" {
		//     err = s.walletClient.CreditRetailerWinningWallet(ctx, ticket.IssuerID, targetPayout.Amount, draw.ID)
		// } else if ticket.IssuerType == "player" {
		//     err = s.walletClient.CreditPlayerWallet(ctx, ticket.IssuerID, targetPayout.Amount, draw.ID)
		// }
		// if err != nil {
		//     return nil, fmt.Errorf("failed to credit wallet: %w", err)
		// }
		//
		// // Mark ticket as paid
		// err = s.ticketClient.MarkTicketAsPaid(ctx, ticketID)
		// if err != nil {
		//     return nil, fmt.Errorf("failed to mark ticket as paid: %w", err)
		// }

		// Update counters
		draw.StageData.PayoutData.ProcessedCount++
		draw.StageData.PayoutData.PendingCount--

		s.logger.Printf("Big win payout approved: draw_id=%s, ticket_id=%s, amount=%d, approved_by=%s",
			drawID, ticketID, targetPayout.Amount, processedBy)
	} else {
		// Reject payout
		targetPayout.Status = "rejected"
		targetPayout.RejectionReason = rejectionReason
		targetPayout.ProcessedAt = &now

		// Update counters
		draw.StageData.PayoutData.PendingCount--

		s.logger.Printf("Big win payout rejected: draw_id=%s, ticket_id=%s, reason=%s, rejected_by=%s",
			drawID, ticketID, rejectionReason, processedBy)
	}

	// Check if all big wins are processed
	if draw.StageData.PayoutData.PendingCount == 0 {
		draw.StageData.StageStatus = models.StageStatusCompleted
		completedAt := time.Now()
		draw.StageData.StageCompletedAt = &completedAt
	}

	// Update the draw
	if err := s.drawRepo.Update(ctx, draw); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update draw: %w", err)
	}

	span.SetAttributes(
		attribute.String("result", "big_win_processed"),
		attribute.String("status", targetPayout.Status),
	)

	return targetPayout, nil
}

// CompleteDraw finalizes the draw execution workflow
func (s *drawService) CompleteDraw(ctx context.Context, drawID uuid.UUID, completedBy string) (*models.Draw, error) {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "DrawService.CompleteDraw")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw_id", drawID.String()),
		attribute.String("completed_by", completedBy),
	)

	// Get the draw
	draw, err := s.drawRepo.GetByID(ctx, drawID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get draw: %w", err)
	}

	// Validate we're in payout stage and it's completed
	if draw.StageData == nil || draw.StageData.CurrentStage != 4 {
		return nil, fmt.Errorf("draw must be in payout stage, current stage: %d", draw.StageData.CurrentStage)
	}

	if draw.StageData.StageStatus != models.StageStatusCompleted {
		return nil, fmt.Errorf("payout stage must be completed first, current status: %s", draw.StageData.StageStatus)
	}

	// Check for any pending big win payouts
	if draw.StageData.PayoutData.PendingCount > 0 {
		return nil, fmt.Errorf("cannot complete draw with %d pending big win payouts", draw.StageData.PayoutData.PendingCount)
	}

	// Mark draw as completed
	now := time.Now()
	draw.Status = models.DrawStatusCompleted
	draw.ExecutedTime = &now

	// Update the draw
	if err := s.drawRepo.Update(ctx, draw); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update draw: %w", err)
	}

	s.logger.Printf("Draw completed successfully: draw_id=%s, completed_by=%s, total_winnings=%d",
		drawID, completedBy, draw.TotalPrizePool)

	span.SetAttributes(
		attribute.String("result", "draw_completed"),
		attribute.Int64("total_prize_pool", draw.TotalPrizePool),
	)

	return draw, nil
}

// UpdateMachineNumbers updates the machine numbers for a completed draw
// Machine numbers are cosmetic identifiers entered after draw completion
func (s *drawService) UpdateMachineNumbers(ctx context.Context, drawID uuid.UUID, machineNumbers []int32, updatedBy string) (*models.Draw, error) {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "DrawService.UpdateMachineNumbers")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw_id", drawID.String()),
		attribute.String("updated_by", updatedBy),
		attribute.Int("machine_numbers_count", len(machineNumbers)),
	)

	// Get the draw
	draw, err := s.drawRepo.GetByID(ctx, drawID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get draw: %w", err)
	}

	// Validate draw is completed
	if draw.Status != models.DrawStatusCompleted {
		return nil, fmt.Errorf("can only update machine numbers for completed draws, current status: %s", draw.Status)
	}

	// Update machine numbers
	draw.MachineNumbers = machineNumbers

	// Save to database
	if err := s.drawRepo.Update(ctx, draw); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update machine numbers: %w", err)
	}

	s.logger.Printf("Machine numbers updated successfully: draw_id=%s, updated_by=%s, machine_numbers=%v",
		drawID, updatedBy, machineNumbers)

	span.SetAttributes(attribute.String("result", "machine_numbers_updated"))

	return draw, nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// convertTicketBetLines converts ticket proto bet lines to engine bet lines
// normalizeBetType converts various bet type formats to the standard format expected by the bet engine
// Handles legacy formats and ensures consistency:
// - "DIRECT" → "DIRECT-N" (where N is count of selected numbers) - for old data
// - "Direct 1" → "DIRECT-1" (space to hyphen, uppercase) - for old data
// - "PERM_2" → "PERM-2" (underscore to hyphen) - for data with underscores
// - "BANKER_ALL" → "BANKER ALL" (underscore to space) - for data with underscores
// Standard format: "DIRECT-1", "DIRECT-2", "PERM-2", "PERM-3", "BANKER ALL", "BANKER AG"
func normalizeBetType(betType string, selectedNumbers []int32) string {
	// Convert to uppercase first
	normalized := strings.ToUpper(betType)

	// Replace underscores with hyphens for DIRECT/PERM types
	if strings.HasPrefix(normalized, "DIRECT_") || strings.HasPrefix(normalized, "PERM_") {
		normalized = strings.ReplaceAll(normalized, "_", "-")
	}

	// Replace underscores with spaces for BANKER types
	if strings.HasPrefix(normalized, "BANKER_") {
		normalized = strings.ReplaceAll(normalized, "_", " ")
	}

	// Replace spaces with hyphens for DIRECT/PERM  types (e.g., "Direct 1" -> "DIRECT-1")
	if strings.HasPrefix(normalized, "DIRECT ") || strings.HasPrefix(normalized, "PERM ") {
		normalized = strings.ReplaceAll(normalized, " ", "-")
	}

	// Handle generic "DIRECT" without suffix - infer number from selected numbers count
	if normalized == "DIRECT" && len(selectedNumbers) > 0 {
		return fmt.Sprintf("DIRECT-%d", len(selectedNumbers))
	}

	// Handle generic "PERM" without suffix - infer number from selected numbers count
	if normalized == "PERM" && len(selectedNumbers) > 0 {
		return fmt.Sprintf("PERM-%d", len(selectedNumbers))
	}

	return normalized
}

func convertTicketBetLines(protoBetLines []*ticketv1.BetLine) []*BetLine {
	betLines := make([]*BetLine, 0, len(protoBetLines))

	for _, pbl := range protoBetLines {
		// Normalize bet type to match bet engine expectations
		normalizedBetType := normalizeBetType(pbl.BetType, pbl.SelectedNumbers)

		betLine := &BetLine{
			LineNumber: pbl.LineNumber,
			BetType:    normalizedBetType,

			// Compact storage format fields
			SelectedNumbers:      pbl.SelectedNumbers,
			TotalAmount:          pbl.TotalAmount,
			NumberOfCombinations: pbl.NumberOfCombinations,
			AmountPerCombination: pbl.AmountPerCombination,

			// Banker/Against fields
			Banker:  pbl.Banker,
			Opposed: pbl.Opposed,
		}
		betLines = append(betLines, betLine)
	}

	return betLines
}

// GameServiceClient returns the game service client for enrichment purposes
func (s *drawService) GameServiceClient(ctx context.Context) (gamev1.GameServiceClient, error) {
	return s.grpcClientManager.GameServiceClient(ctx)
}

package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	drawpb "github.com/randco/randco-microservices/proto/draw/v1"
	gamepb "github.com/randco/randco-microservices/proto/game/v1"
	"github.com/randco/service-draw/internal/models"
	"github.com/randco/service-draw/internal/services"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DrawServiceServer implements the DrawService gRPC service
type DrawServiceServer struct {
	drawpb.UnimplementedDrawServiceServer
	drawService services.DrawService
	logger      *log.Logger
}

// NewDrawServiceServer creates a new DrawServiceServer
func NewDrawServiceServer(drawService services.DrawService, logger *log.Logger) *DrawServiceServer {
	return &DrawServiceServer{
		drawService: drawService,
		logger:      logger,
	}
}

// CreateDraw creates a new draw record
func (s *DrawServiceServer) CreateDraw(ctx context.Context, req *drawpb.CreateDrawRequest) (*drawpb.CreateDrawResponse, error) {
	s.logger.Printf("CreateDraw called: game_id=%s, game_name=%s, draw_name=%s", req.GameId, req.GameName, req.DrawName)

	// Parse game ID
	gameID, err := uuid.Parse(req.GameId)
	if err != nil {
		s.logger.Printf("Invalid game_id: %v", err)
		return &drawpb.CreateDrawResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid game_id: %v", err),
		}, nil
	}

	// Convert proto timestamp to time.Time
	scheduledTime := req.ScheduledTime.AsTime()

	// Parse game_schedule_id if provided
	var gameScheduleID uuid.UUID
	if req.GameScheduleId != "" {
		parsedScheduleID, err := uuid.Parse(req.GameScheduleId)
		if err != nil {
			s.logger.Printf("Invalid game_schedule_id: %v", err)
			return &drawpb.CreateDrawResponse{
				Success: false,
				Message: fmt.Sprintf("Invalid game_schedule_id: %v", err),
			}, nil
		}
		gameScheduleID = parsedScheduleID
	}

	// Create draw request
	drawReq := services.CreateDrawRequest{
		GameID:         gameID,
		GameName:       req.GameName,
		GameCode:       req.GameCode,   // Pass game code from request
		GameScheduleID: gameScheduleID, // Pass game schedule ID for ticket linking
		DrawName:       req.DrawName,
		ScheduledTime:  scheduledTime,
		DrawLocation:   req.DrawLocation,
	}

	// Create draw via service
	createdDraw, err := s.drawService.CreateDraw(ctx, drawReq)
	if err != nil {
		s.logger.Printf("Failed to create draw: %v", err)
		return &drawpb.CreateDrawResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create draw: %v", err),
		}, nil
	}

	s.logger.Printf("Draw created successfully: id=%s", createdDraw.ID)

	// Convert model to proto
	protoDraw := &drawpb.Draw{
		Id:            createdDraw.ID.String(),
		GameId:        createdDraw.GameID.String(),
		DrawNumber:    int32(createdDraw.DrawNumber),
		GameName:      createdDraw.GameName,
		DrawName:      createdDraw.DrawName,
		Status:        convertDrawStatusToProto(createdDraw.Status),
		ScheduledTime: timestamppb.New(createdDraw.ScheduledTime),
		CreatedAt:     timestamppb.New(createdDraw.CreatedAt),
		UpdatedAt:     timestamppb.New(createdDraw.UpdatedAt),
	}

	if createdDraw.ExecutedTime != nil {
		protoDraw.ExecutedTime = timestamppb.New(*createdDraw.ExecutedTime)
	}
	if createdDraw.DrawLocation != nil {
		protoDraw.DrawLocation = *createdDraw.DrawLocation
	}
	if createdDraw.NLADrawReference != nil {
		protoDraw.NlaDrawReference = *createdDraw.NLADrawReference
	}
	if createdDraw.NLAOfficialSignature != nil {
		protoDraw.NlaOfficialSignature = *createdDraw.NLAOfficialSignature
	}

	return &drawpb.CreateDrawResponse{
		Draw:    protoDraw,
		Success: true,
		Message: "Draw created successfully",
	}, nil
}

// GetDraw retrieves a draw by ID
func (s *DrawServiceServer) GetDraw(ctx context.Context, req *drawpb.GetDrawRequest) (*drawpb.GetDrawResponse, error) {
	s.logger.Printf("GetDraw called: id=%s", req.Id)

	drawID, err := uuid.Parse(req.Id)
	if err != nil {
		return &drawpb.GetDrawResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid draw_id: %v", err),
		}, nil
	}

	draw, err := s.drawService.GetDraw(ctx, drawID)
	if err != nil {
		s.logger.Printf("Failed to get draw: %v", err)
		return &drawpb.GetDrawResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to get draw: %v", err),
		}, nil
	}

	// Use the shared conversion function to ensure stage data is included
	protoDraw := convertDrawToProto(draw)

	return &drawpb.GetDrawResponse{
		Draw:    protoDraw,
		Success: true,
		Message: "Draw retrieved successfully",
	}, nil
}

// ListDraws returns a list of draws with optional filters
func (s *DrawServiceServer) ListDraws(ctx context.Context, req *drawpb.ListDrawsRequest) (*drawpb.ListDrawsResponse, error) {
	s.logger.Printf("ListDraws called: game_id=%s, status=%v, page=%d, per_page=%d",
		req.GameId, req.StatusFilter, req.Page, req.PerPage)

	// Parse optional game ID filter
	var gameID *uuid.UUID
	if req.GameId != "" {
		parsed, err := uuid.Parse(req.GameId)
		if err != nil {
			return &drawpb.ListDrawsResponse{
				Success: false,
				Message: fmt.Sprintf("Invalid game_id: %v", err),
			}, nil
		}
		gameID = &parsed
	}

	// Parse optional status filter
	var status *models.DrawStatus
	if req.StatusFilter != drawpb.DrawStatus_DRAW_STATUS_UNSPECIFIED {
		modelStatus := convertProtoToDrawStatus(req.StatusFilter)
		status = &modelStatus
	}

	// Parse optional date filters
	var startDate, endDate *timestamppb.Timestamp
	if req.StartDate != nil {
		startDate = req.StartDate
	}
	if req.EndDate != nil {
		endDate = req.EndDate
	}

	// Convert proto timestamps to time.Time pointers
	var startTime, endTime *timestamppb.Timestamp
	if startDate != nil {
		startTime = startDate
	}
	if endDate != nil {
		endTime = endDate
	}

	// Create service request
	listReq := services.ListDrawsRequest{
		GameID:  gameID,
		Status:  status,
		Page:    int(req.Page),
		PerPage: int(req.PerPage),
	}
	if startTime != nil {
		t := startTime.AsTime()
		listReq.StartDate = &t
	}
	if endTime != nil {
		t := endTime.AsTime()
		listReq.EndDate = &t
	}

	// Get draws from service
	draws, total, err := s.drawService.ListDraws(ctx, listReq)
	if err != nil {
		s.logger.Printf("Failed to list draws: %v", err)
		return &drawpb.ListDrawsResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to list draws: %v", err),
		}, nil
	}

	// Convert draws to proto
	protoDraws := make([]*drawpb.Draw, len(draws))
	for i, draw := range draws {
		protoDraws[i] = &drawpb.Draw{
			Id:            draw.ID.String(),
			GameId:        draw.GameID.String(),
			DrawNumber:    int32(draw.DrawNumber),
			GameName:      draw.GameName,
			DrawName:      draw.DrawName,
			Status:        convertDrawStatusToProto(draw.Status),
			ScheduledTime: timestamppb.New(draw.ScheduledTime),
			CreatedAt:     timestamppb.New(draw.CreatedAt),
			UpdatedAt:     timestamppb.New(draw.UpdatedAt),
		}

		if draw.ExecutedTime != nil {
			protoDraws[i].ExecutedTime = timestamppb.New(*draw.ExecutedTime)
		}
		if draw.DrawLocation != nil {
			protoDraws[i].DrawLocation = *draw.DrawLocation
		}
		if draw.NLADrawReference != nil {
			protoDraws[i].NlaDrawReference = *draw.NLADrawReference
		}
		if draw.NLAOfficialSignature != nil {
			protoDraws[i].NlaOfficialSignature = *draw.NLAOfficialSignature
		}
		if len(draw.WinningNumbers) > 0 {
			protoDraws[i].WinningNumbers = draw.WinningNumbers
		}
		if draw.VerificationHash != nil {
			protoDraws[i].VerificationHash = *draw.VerificationHash
		}
		protoDraws[i].TotalTicketsSold = draw.TotalTicketsSold
		protoDraws[i].TotalPrizePool = draw.TotalPrizePool
	}

	s.logger.Printf("ListDraws succeeded: returned %d draws (total: %d)", len(protoDraws), total)

	return &drawpb.ListDrawsResponse{
		Draws:      protoDraws,
		TotalCount: total,
		Page:       req.Page,
		PerPage:    req.PerPage,
		Success:    true,
		Message:    "Draws retrieved successfully",
	}, nil
}

// UpdateDraw updates an existing draw
func (s *DrawServiceServer) UpdateDraw(ctx context.Context, req *drawpb.UpdateDrawRequest) (*drawpb.UpdateDrawResponse, error) {
	s.logger.Printf("UpdateDraw called: id=%s", req.Id)

	// Not implemented yet - stub response
	return &drawpb.UpdateDrawResponse{
		Success: false,
		Message: "UpdateDraw not yet implemented",
	}, nil
}

// GetPublicCompletedDraws returns completed draws for public consumption (no auth required)
func (s *DrawServiceServer) GetPublicCompletedDraws(ctx context.Context, req *drawpb.GetPublicCompletedDrawsRequest) (*drawpb.GetPublicCompletedDrawsResponse, error) {
	s.logger.Printf("GetPublicCompletedDraws called: game_id=%s, game_code=%s, page=%d, per_page=%d, latest_only=%v, start_date=%v, end_date=%v",
		req.GameId, req.GameCode, req.Page, req.PerPage, req.LatestOnly, req.StartDate, req.EndDate)

	// Parse optional game ID filter
	var gameID *uuid.UUID
	if req.GameId != "" {
		parsed, err := uuid.Parse(req.GameId)
		if err != nil {
			return &drawpb.GetPublicCompletedDrawsResponse{
				Success: false,
				Message: fmt.Sprintf("Invalid game_id: %v", err),
			}, nil
		}
		gameID = &parsed
	}

	// Set default page and per_page
	page := int32(1)
	if req.Page > 0 {
		page = req.Page
	}

	perPage := int32(10)
	if req.PerPage > 0 {
		perPage = req.PerPage
		if perPage > 50 {
			perPage = 50
		}
	}

	// Parse optional date filters
	var startDate, endDate *time.Time
	if req.StartDate != nil {
		t := req.StartDate.AsTime()
		startDate = &t
	}
	if req.EndDate != nil {
		t := req.EndDate.AsTime()
		endDate = &t
	}

	// Create service request
	serviceReq := services.GetPublicCompletedDrawsRequest{
		GameID:     gameID,
		GameCode:   req.GameCode,
		Page:       int(page),
		PerPage:    int(perPage),
		LatestOnly: req.LatestOnly,
		StartDate:  startDate,
		EndDate:    endDate,
	}

	// Get completed draws from service
	draws, total, err := s.drawService.GetPublicCompletedDraws(ctx, serviceReq)
	if err != nil {
		s.logger.Printf("Failed to get public completed draws: %v", err)
		return &drawpb.GetPublicCompletedDrawsResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to get completed draws: %v", err),
		}, nil
	}

	// Enrich draws with game branding information (logo_url, brand_color)
	gameBrandingMap := make(map[uuid.UUID]struct {
		LogoURL    string
		BrandColor string
	})

	// Extract unique game IDs
	uniqueGameIDs := make(map[uuid.UUID]bool)
	for _, draw := range draws {
		uniqueGameIDs[draw.GameID] = true
	}

	// Fetch game branding for each unique game ID
	if len(uniqueGameIDs) > 0 {
		gameClient, err := s.drawService.GameServiceClient(ctx)
		if err != nil {
			s.logger.Printf("Warning: Failed to get game service client for branding enrichment: %v", err)
			// Continue without enrichment - don't fail the request
		} else {
			for gameID := range uniqueGameIDs {
				gameResp, err := gameClient.GetGame(ctx, &gamepb.GetGameRequest{
					Id: gameID.String(),
				})
				if err != nil {
					s.logger.Printf("Warning: Failed to fetch game branding for game_id=%s: %v", gameID, err)
					// Continue without enrichment for this game - don't fail the request
					continue
				}

				if gameResp.Success && gameResp.Game != nil {
					gameBrandingMap[gameID] = struct {
						LogoURL    string
						BrandColor string
					}{
						LogoURL:    gameResp.Game.LogoUrl,
						BrandColor: gameResp.Game.BrandColor,
					}
				}
			}
		}
	}

	// Convert draws to proto PublicDrawResult
	protoDraws := make([]*drawpb.PublicDrawResult, len(draws))
	for i, draw := range draws {
		// Get branding information from map
		branding := gameBrandingMap[draw.GameID]

		protoDraws[i] = &drawpb.PublicDrawResult{
			DrawId:         draw.ID.String(),
			GameId:         draw.GameID.String(),
			GameName:       draw.GameName,
			GameScheduleId: draw.GameScheduleID.String(),
			DrawNumber:     int32(draw.DrawNumber),
			WinningNumbers: draw.WinningNumbers,
			MachineNumbers: draw.MachineNumbers,
			DrawDate:       timestamppb.New(draw.ScheduledTime),
			DrawName:       draw.DrawName,
			GameLogoUrl:    branding.LogoURL,
			GameBrandColor: branding.BrandColor,
		}
	}

	// Calculate total pages
	totalPages := int64(0)
	if perPage > 0 {
		totalPages = (total + int64(perPage) - 1) / int64(perPage)
	}

	s.logger.Printf("GetPublicCompletedDraws succeeded: returned %d draws (total: %d, page: %d/%d)",
		len(protoDraws), total, page, totalPages)

	return &drawpb.GetPublicCompletedDrawsResponse{
		Draws:      protoDraws,
		TotalCount: total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
		Success:    true,
		Message:    "Completed draws retrieved successfully",
	}, nil
}

// ScheduleDraw schedules a new draw
func (s *DrawServiceServer) ScheduleDraw(ctx context.Context, req *drawpb.ScheduleDrawRequest) (*drawpb.ScheduleDrawResponse, error) {
	s.logger.Printf("ScheduleDraw called: game_id=%s", req.GameId)

	// Not implemented yet - stub response
	return &drawpb.ScheduleDrawResponse{
		Success: false,
		Message: "ScheduleDraw not yet implemented",
	}, nil
}

// GetScheduledDraws returns scheduled draws
func (s *DrawServiceServer) GetScheduledDraws(ctx context.Context, req *drawpb.GetScheduledDrawsRequest) (*drawpb.GetScheduledDrawsResponse, error) {
	s.logger.Printf("GetScheduledDraws called: game_id=%s", req.GameId)

	// Not implemented yet - stub response
	return &drawpb.GetScheduledDrawsResponse{
		Success: true,
		Message: "GetScheduledDraws not yet implemented",
	}, nil
}

// CancelScheduledDraw cancels a scheduled draw
func (s *DrawServiceServer) CancelScheduledDraw(ctx context.Context, req *drawpb.CancelScheduledDrawRequest) (*drawpb.CancelScheduledDrawResponse, error) {
	s.logger.Printf("CancelScheduledDraw called: schedule_id=%s", req.ScheduleId)

	// Not implemented yet - stub response
	return &drawpb.CancelScheduledDrawResponse{
		Success: false,
		Message: "CancelScheduledDraw not yet implemented",
	}, nil
}

// RecordDrawResults records the results of a draw
func (s *DrawServiceServer) RecordDrawResults(ctx context.Context, req *drawpb.RecordDrawResultsRequest) (*drawpb.RecordDrawResultsResponse, error) {
	s.logger.Printf("RecordDrawResults called: draw_id=%s", req.DrawId)

	// Not implemented yet - stub response
	return &drawpb.RecordDrawResultsResponse{
		Success: false,
		Message: "RecordDrawResults not yet implemented",
	}, nil
}

// GetDrawResults retrieves draw results
func (s *DrawServiceServer) GetDrawResults(ctx context.Context, req *drawpb.GetDrawResultsRequest) (*drawpb.GetDrawResultsResponse, error) {
	s.logger.Printf("GetDrawResults called: draw_id=%s", req.DrawId)

	// Not implemented yet - stub response
	return &drawpb.GetDrawResultsResponse{
		Success: false,
		Message: "GetDrawResults not yet implemented",
	}, nil
}

// PublishDrawResults publishes draw results
func (s *DrawServiceServer) PublishDrawResults(ctx context.Context, req *drawpb.PublishDrawResultsRequest) (*drawpb.PublishDrawResultsResponse, error) {
	s.logger.Printf("PublishDrawResults called: draw_id=%s", req.DrawId)

	// Not implemented yet - stub response
	return &drawpb.PublishDrawResultsResponse{
		Success: false,
		Message: "PublishDrawResults not yet implemented",
	}, nil
}

// ValidateDrawResults validates draw results
func (s *DrawServiceServer) ValidateDrawResults(ctx context.Context, req *drawpb.ValidateDrawResultsRequest) (*drawpb.ValidateDrawResultsResponse, error) {
	s.logger.Printf("ValidateDrawResults called: draw_id=%s", req.DrawId)

	// Not implemented yet - stub response
	return &drawpb.ValidateDrawResultsResponse{
		Success: false,
		Message: "ValidateDrawResults not yet implemented",
	}, nil
}

// convertDrawStatusToProto converts model DrawStatus to proto DrawStatus
func convertDrawStatusToProto(status models.DrawStatus) drawpb.DrawStatus {
	switch status {
	case models.DrawStatusScheduled:
		return drawpb.DrawStatus_DRAW_STATUS_SCHEDULED
	case models.DrawStatusInProgress:
		return drawpb.DrawStatus_DRAW_STATUS_IN_PROGRESS
	case models.DrawStatusCompleted:
		return drawpb.DrawStatus_DRAW_STATUS_COMPLETED
	case models.DrawStatusFailed:
		return drawpb.DrawStatus_DRAW_STATUS_FAILED
	case models.DrawStatusCancelled:
		return drawpb.DrawStatus_DRAW_STATUS_CANCELLED
	default:
		return drawpb.DrawStatus_DRAW_STATUS_UNSPECIFIED
	}
}

// convertProtoToDrawStatus converts proto DrawStatus to model DrawStatus
func convertProtoToDrawStatus(status drawpb.DrawStatus) models.DrawStatus {
	switch status {
	case drawpb.DrawStatus_DRAW_STATUS_SCHEDULED:
		return models.DrawStatusScheduled
	case drawpb.DrawStatus_DRAW_STATUS_IN_PROGRESS:
		return models.DrawStatusInProgress
	case drawpb.DrawStatus_DRAW_STATUS_COMPLETED:
		return models.DrawStatusCompleted
	case drawpb.DrawStatus_DRAW_STATUS_FAILED:
		return models.DrawStatusFailed
	case drawpb.DrawStatus_DRAW_STATUS_CANCELLED:
		return models.DrawStatusCancelled
	default:
		return models.DrawStatusScheduled
	}
}

// ============================================================================
// Draw Execution Workflow Handlers - Stage 1: Preparation
// ============================================================================

// StartDrawPreparation starts the draw preparation stage
func (s *DrawServiceServer) StartDrawPreparation(ctx context.Context, req *drawpb.StartDrawPreparationRequest) (*drawpb.StartDrawPreparationResponse, error) {
	s.logger.Printf("StartDrawPreparation called: draw_id=%s, initiated_by=%s", req.DrawId, req.InitiatedBy)

	// Parse draw ID
	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		s.logger.Printf("Invalid draw_id: %v", err)
		return &drawpb.StartDrawPreparationResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid draw_id: %v", err),
		}, nil
	}

	// Start preparation via service
	draw, err := s.drawService.StartDrawPreparation(ctx, drawID, req.InitiatedBy)
	if err != nil {
		s.logger.Printf("Failed to start draw preparation: %v", err)
		return &drawpb.StartDrawPreparationResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to start draw preparation: %v", err),
		}, nil
	}

	s.logger.Printf("Draw preparation started successfully: draw_id=%s", draw.ID)

	// Convert model to proto
	protoDraw := convertDrawToProto(draw)

	return &drawpb.StartDrawPreparationResponse{
		Draw:    protoDraw,
		Success: true,
		Message: "Draw preparation started successfully",
	}, nil
}

// CompleteDrawPreparation completes the draw preparation stage
func (s *DrawServiceServer) CompleteDrawPreparation(ctx context.Context, req *drawpb.CompleteDrawPreparationRequest) (*drawpb.CompleteDrawPreparationResponse, error) {
	s.logger.Printf("CompleteDrawPreparation called: draw_id=%s, completed_by=%s", req.DrawId, req.CompletedBy)

	// Parse draw ID
	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		s.logger.Printf("Invalid draw_id: %v", err)
		return &drawpb.CompleteDrawPreparationResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid draw_id: %v", err),
		}, nil
	}

	// Complete preparation via service
	draw, err := s.drawService.CompleteDrawPreparation(ctx, drawID, req.CompletedBy)
	if err != nil {
		s.logger.Printf("Failed to complete draw preparation: %v", err)
		return &drawpb.CompleteDrawPreparationResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to complete draw preparation: %v", err),
		}, nil
	}

	s.logger.Printf("Draw preparation completed successfully: draw_id=%s", draw.ID)

	// Convert model to proto
	protoDraw := convertDrawToProto(draw)

	return &drawpb.CompleteDrawPreparationResponse{
		Draw:    protoDraw,
		Success: true,
		Message: "Draw preparation completed successfully",
	}, nil
}

// convertDrawToProto converts a Draw model to proto message
func convertDrawToProto(draw *models.Draw) *drawpb.Draw {
	protoDraw := &drawpb.Draw{
		Id:               draw.ID.String(),
		GameId:           draw.GameID.String(),
		GameScheduleId:   draw.GameScheduleID.String(),
		DrawNumber:       int32(draw.DrawNumber),
		GameName:         draw.GameName,
		DrawName:         draw.DrawName,
		Status:           convertDrawStatusToProto(draw.Status),
		ScheduledTime:    timestamppb.New(draw.ScheduledTime),
		TotalTicketsSold: draw.TotalTicketsSold,
		TotalPrizePool:   draw.TotalPrizePool,
		CreatedAt:        timestamppb.New(draw.CreatedAt),
		UpdatedAt:        timestamppb.New(draw.UpdatedAt),
	}

	if draw.ExecutedTime != nil {
		protoDraw.ExecutedTime = timestamppb.New(*draw.ExecutedTime)
	}
	if len(draw.WinningNumbers) > 0 {
		protoDraw.WinningNumbers = draw.WinningNumbers
	}
	if len(draw.MachineNumbers) > 0 {
		protoDraw.MachineNumbers = draw.MachineNumbers
	}
	if draw.DrawLocation != nil {
		protoDraw.DrawLocation = *draw.DrawLocation
	}
	if draw.NLADrawReference != nil {
		protoDraw.NlaDrawReference = *draw.NLADrawReference
	}
	if draw.NLAOfficialSignature != nil {
		protoDraw.NlaOfficialSignature = *draw.NLAOfficialSignature
	}
	if draw.VerificationHash != nil {
		protoDraw.VerificationHash = *draw.VerificationHash
	}

	// Convert stage data
	if draw.StageData != nil {
		protoDraw.Stage = convertStageDataToProto(draw.StageData)
	}

	return protoDraw
}

// convertStageDataToProto converts stage data model to proto
func convertStageDataToProto(stage *models.DrawStage) *drawpb.DrawStage {
	protoStage := &drawpb.DrawStage{
		CurrentStage: int32(stage.CurrentStage),
		StageName:    stage.StageName,
		StageStatus:  convertStageStatusToProto(stage.StageStatus),
	}

	if stage.StageStartedAt != nil {
		protoStage.StageStartedAt = timestamppb.New(*stage.StageStartedAt)
	}
	if stage.StageCompletedAt != nil {
		protoStage.StageCompletedAt = timestamppb.New(*stage.StageCompletedAt)
	}

	// Convert preparation data
	if stage.PreparationData != nil {
		protoStage.PreparationData = &drawpb.PreparationStageData{
			TicketsLocked: stage.PreparationData.TicketsLocked,
			TotalStakes:   stage.PreparationData.TotalStakes,
			SalesLocked:   stage.PreparationData.SalesLocked,
		}
		if stage.PreparationData.LockTime != nil {
			protoStage.PreparationData.LockTime = timestamppb.New(*stage.PreparationData.LockTime)
		}
	}

	// Convert number selection data
	if stage.NumberSelectionData != nil {
		protoStage.NumberSelectionData = &drawpb.NumberSelectionStageData{
			WinningNumbers: stage.NumberSelectionData.WinningNumbers,
			IsVerified:     stage.NumberSelectionData.IsVerified,
			VerifiedBy:     stage.NumberSelectionData.VerifiedBy,
		}
		if stage.NumberSelectionData.VerifiedAt != nil {
			protoStage.NumberSelectionData.VerifiedAt = timestamppb.New(*stage.NumberSelectionData.VerifiedAt)
		}
		for _, attempt := range stage.NumberSelectionData.VerificationAttempts {
			protoStage.NumberSelectionData.VerificationAttempts = append(
				protoStage.NumberSelectionData.VerificationAttempts,
				&drawpb.VerificationAttempt{
					AttemptNumber: attempt.AttemptNumber,
					Numbers:       attempt.Numbers,
					SubmittedBy:   attempt.SubmittedBy,
					SubmittedAt:   timestamppb.New(attempt.SubmittedAt),
				},
			)
		}
	}

	// Convert result calculation data
	if stage.ResultCalculationData != nil {
		protoStage.ResultCalculationData = &drawpb.ResultCalculationStageData{
			WinningTicketsCount: stage.ResultCalculationData.WinningTicketsCount,
			TotalWinnings:       stage.ResultCalculationData.TotalWinnings,
		}
		if stage.ResultCalculationData.CalculatedAt != nil {
			protoStage.ResultCalculationData.CalculatedAt = timestamppb.New(*stage.ResultCalculationData.CalculatedAt)
		}
		for _, tier := range stage.ResultCalculationData.WinningTiers {
			protoStage.ResultCalculationData.WinningTiers = append(
				protoStage.ResultCalculationData.WinningTiers,
				&drawpb.WinningTier{
					BetType:      tier.BetType,
					WinnersCount: tier.WinnersCount,
					TotalAmount:  tier.TotalAmount,
				},
			)
		}
		// Convert winning tickets
		for _, ticket := range stage.ResultCalculationData.WinningTickets {
			protoStage.ResultCalculationData.WinningTickets = append(
				protoStage.ResultCalculationData.WinningTickets,
				&drawpb.WinningTicketDetail{
					TicketId:      ticket.TicketID,
					SerialNumber:  ticket.SerialNumber,
					RetailerId:    ticket.RetailerID,
					Numbers:       ticket.Numbers,
					BetType:       ticket.BetType,
					StakeAmount:   ticket.StakeAmount,
					WinningAmount: ticket.WinningAmount,
					MatchesCount:  ticket.MatchesCount,
					IsBigWin:      ticket.IsBigWin,
					AgentCode:     ticket.AgentCode,
					TerminalId:    ticket.TerminalID,
					CustomerPhone: ticket.CustomerPhone,
					PaymentMethod: ticket.PaymentMethod,
					Status:        ticket.Status,
				},
			)
		}
	}

	// Convert payout data
	if stage.PayoutData != nil {
		protoStage.PayoutData = &drawpb.PayoutStageData{
			AutoProcessedCount:   stage.PayoutData.AutoProcessedCount,
			ManualApprovalCount:  stage.PayoutData.ManualApprovalCount,
			AutoProcessedAmount:  stage.PayoutData.AutoProcessedAmount,
			ManualApprovalAmount: stage.PayoutData.ManualApprovalAmount,
			ProcessedCount:       stage.PayoutData.ProcessedCount,
			PendingCount:         stage.PayoutData.PendingCount,
		}
		for _, payout := range stage.PayoutData.BigWinPayouts {
			protoPayout := &drawpb.BigWinPayout{
				TicketId:        payout.TicketID,
				Amount:          payout.Amount,
				Status:          payout.Status,
				ApprovedBy:      payout.ApprovedBy,
				RejectionReason: payout.RejectionReason,
			}
			if payout.ProcessedAt != nil {
				protoPayout.ProcessedAt = timestamppb.New(*payout.ProcessedAt)
			}
			protoStage.PayoutData.BigWinPayouts = append(protoStage.PayoutData.BigWinPayouts, protoPayout)
		}
	}

	return protoStage
}

// convertStageStatusToProto converts stage status to proto
func convertStageStatusToProto(status models.StageStatus) drawpb.StageStatus {
	switch status {
	case models.StageStatusPending:
		return drawpb.StageStatus_STAGE_STATUS_PENDING
	case models.StageStatusInProgress:
		return drawpb.StageStatus_STAGE_STATUS_IN_PROGRESS
	case models.StageStatusCompleted:
		return drawpb.StageStatus_STAGE_STATUS_COMPLETED
	case models.StageStatusFailed:
		return drawpb.StageStatus_STAGE_STATUS_FAILED
	default:
		return drawpb.StageStatus_STAGE_STATUS_UNSPECIFIED
	}
}

// ============================================================================
// Draw Execution Workflow Handlers - Stage 2: Number Selection
// ============================================================================

// StartNumberSelection starts the number selection stage
func (s *DrawServiceServer) StartNumberSelection(ctx context.Context, req *drawpb.StartNumberSelectionRequest) (*drawpb.StartNumberSelectionResponse, error) {
	s.logger.Printf("StartNumberSelection called: draw_id=%s, initiated_by=%s", req.DrawId, req.InitiatedBy)

	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		s.logger.Printf("Invalid draw_id: %v", err)
		return &drawpb.StartNumberSelectionResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid draw_id: %v", err),
		}, nil
	}

	draw, err := s.drawService.StartNumberSelection(ctx, drawID, req.InitiatedBy)
	if err != nil {
		s.logger.Printf("Failed to start number selection: %v", err)
		return &drawpb.StartNumberSelectionResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to start number selection: %v", err),
		}, nil
	}

	s.logger.Printf("Number selection started successfully: draw_id=%s", draw.ID)

	return &drawpb.StartNumberSelectionResponse{
		Draw:    convertDrawToProto(draw),
		Success: true,
		Message: "Number selection started successfully",
	}, nil
}

// SubmitVerificationAttempt records a verification attempt
func (s *DrawServiceServer) SubmitVerificationAttempt(ctx context.Context, req *drawpb.SubmitVerificationAttemptRequest) (*drawpb.SubmitVerificationAttemptResponse, error) {
	s.logger.Printf("SubmitVerificationAttempt called: draw_id=%s, submitted_by=%s", req.DrawId, req.SubmittedBy)

	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		s.logger.Printf("Invalid draw_id: %v", err)
		return &drawpb.SubmitVerificationAttemptResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid draw_id: %v", err),
		}, nil
	}

	draw, attemptNumber, err := s.drawService.SubmitVerificationAttempt(ctx, drawID, req.Numbers, req.SubmittedBy)
	if err != nil {
		s.logger.Printf("Failed to submit verification attempt: %v", err)
		return &drawpb.SubmitVerificationAttemptResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to submit verification attempt: %v", err),
		}, nil
	}

	s.logger.Printf("Verification attempt submitted: draw_id=%s, attempt=%d", draw.ID, attemptNumber)

	return &drawpb.SubmitVerificationAttemptResponse{
		Draw:          convertDrawToProto(draw),
		AttemptNumber: attemptNumber,
		Success:       true,
		Message:       fmt.Sprintf("Verification attempt %d submitted successfully", attemptNumber),
	}, nil
}

// ValidateVerificationAttempts validates that both attempts match
func (s *DrawServiceServer) ValidateVerificationAttempts(ctx context.Context, req *drawpb.ValidateVerificationAttemptsRequest) (*drawpb.ValidateVerificationAttemptsResponse, error) {
	s.logger.Printf("ValidateVerificationAttempts called: draw_id=%s, validated_by=%s", req.DrawId, req.ValidatedBy)

	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		s.logger.Printf("Invalid draw_id: %v", err)
		return &drawpb.ValidateVerificationAttemptsResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid draw_id: %v", err),
		}, nil
	}

	isValid, winningNumbers, validationError, err := s.drawService.ValidateVerificationAttempts(ctx, drawID, req.ValidatedBy)
	if err != nil {
		s.logger.Printf("Failed to validate verification attempts: %v", err)
		return &drawpb.ValidateVerificationAttemptsResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to validate verification attempts: %v", err),
		}, nil
	}

	s.logger.Printf("Validation result: draw_id=%s, is_valid=%v", drawID, isValid)

	return &drawpb.ValidateVerificationAttemptsResponse{
		IsValid:         isValid,
		WinningNumbers:  winningNumbers,
		ValidationError: validationError,
		Success:         true,
		Message:         "Validation completed",
	}, nil
}

// ResetVerificationAttempts resets all verification attempts
func (s *DrawServiceServer) ResetVerificationAttempts(ctx context.Context, req *drawpb.ResetVerificationAttemptsRequest) (*drawpb.ResetVerificationAttemptsResponse, error) {
	s.logger.Printf("ResetVerificationAttempts called: draw_id=%s, reset_by=%s, reason=%s", req.DrawId, req.ResetBy, req.Reason)

	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		s.logger.Printf("Invalid draw_id: %v", err)
		return &drawpb.ResetVerificationAttemptsResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid draw_id: %v", err),
		}, nil
	}

	draw, err := s.drawService.ResetVerificationAttempts(ctx, drawID, req.ResetBy, req.Reason)
	if err != nil {
		s.logger.Printf("Failed to reset verification attempts: %v", err)
		return &drawpb.ResetVerificationAttemptsResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to reset verification attempts: %v", err),
		}, nil
	}

	s.logger.Printf("Verification attempts reset: draw_id=%s", draw.ID)

	return &drawpb.ResetVerificationAttemptsResponse{
		Draw:    convertDrawToProto(draw),
		Success: true,
		Message: "Verification attempts reset successfully",
	}, nil
}

// CompleteNumberSelection completes the number selection stage
func (s *DrawServiceServer) CompleteNumberSelection(ctx context.Context, req *drawpb.CompleteNumberSelectionRequest) (*drawpb.CompleteNumberSelectionResponse, error) {
	s.logger.Printf("CompleteNumberSelection called: draw_id=%s, completed_by=%s", req.DrawId, req.CompletedBy)

	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		s.logger.Printf("Invalid draw_id: %v", err)
		return &drawpb.CompleteNumberSelectionResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid draw_id: %v", err),
		}, nil
	}

	draw, err := s.drawService.CompleteNumberSelection(ctx, drawID, req.WinningNumbers, req.CompletedBy)
	if err != nil {
		s.logger.Printf("Failed to complete number selection: %v", err)
		return &drawpb.CompleteNumberSelectionResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to complete number selection: %v", err),
		}, nil
	}

	s.logger.Printf("Number selection completed: draw_id=%s, winning_numbers=%v", draw.ID, req.WinningNumbers)

	return &drawpb.CompleteNumberSelectionResponse{
		Draw:    convertDrawToProto(draw),
		Success: true,
		Message: "Number selection completed successfully",
	}, nil
}

// ============================================================================
// Draw Execution Workflow Handlers - Stage 3: Result Calculation
// ============================================================================

// CommitResults calculates winnings for all tickets
func (s *DrawServiceServer) CommitResults(ctx context.Context, req *drawpb.CommitResultsRequest) (*drawpb.CommitResultsResponse, error) {
	s.logger.Printf("CommitResults called: draw_id=%s, committed_by=%s", req.DrawId, req.CommittedBy)

	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		s.logger.Printf("Invalid draw_id: %v", err)
		return &drawpb.CommitResultsResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid draw_id: %v", err),
		}, nil
	}

	draw, summary, err := s.drawService.CommitResults(ctx, drawID, req.CommittedBy)
	if err != nil {
		s.logger.Printf("Failed to commit results: %v", err)
		return &drawpb.CommitResultsResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to commit results: %v", err),
		}, nil
	}

	s.logger.Printf("Results committed successfully: draw_id=%s, winning_tickets=%d, total_winnings=%d",
		draw.ID, summary.WinningTicketsCount, summary.TotalWinnings)

	// Convert winning tiers to proto
	protoTiers := make([]*drawpb.WinningTier, len(summary.WinningTiers))
	for i, tier := range summary.WinningTiers {
		protoTiers[i] = &drawpb.WinningTier{
			BetType:      tier.BetType,
			WinnersCount: tier.WinnersCount,
			TotalAmount:  tier.TotalAmount,
		}
	}

	return &drawpb.CommitResultsResponse{
		Draw:                convertDrawToProto(draw),
		WinningTicketsCount: summary.WinningTicketsCount,
		TotalWinnings:       summary.TotalWinnings,
		WinningTiers:        protoTiers,
		Success:             true,
		Message:             "Results committed successfully",
	}, nil
}

// ============================================================================
// Draw Execution Workflow Handlers - Stage 4: Payout Processing
// ============================================================================

// ProcessPayouts processes all winning ticket payouts
func (s *DrawServiceServer) ProcessPayouts(ctx context.Context, req *drawpb.ProcessPayoutsRequest) (*drawpb.ProcessPayoutsResponse, error) {
	s.logger.Printf("ProcessPayouts called: draw_id=%s, processed_by=%s", req.DrawId, req.ProcessedBy)

	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		s.logger.Printf("Invalid draw_id: %v", err)
		return &drawpb.ProcessPayoutsResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid draw_id: %v", err),
		}, nil
	}

	draw, summary, err := s.drawService.ProcessPayouts(ctx, drawID, req.ProcessedBy)
	if err != nil {
		s.logger.Printf("Failed to process payouts: %v", err)
		return &drawpb.ProcessPayoutsResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to process payouts: %v", err),
		}, nil
	}

	s.logger.Printf("Payouts processed: draw_id=%s, auto=%d, manual=%d",
		draw.ID, summary.AutoProcessedCount, summary.ManualApprovalCount)

	return &drawpb.ProcessPayoutsResponse{
		Draw:                 convertDrawToProto(draw),
		AutoProcessedCount:   summary.AutoProcessedCount,
		ManualApprovalCount:  summary.ManualApprovalCount,
		AutoProcessedAmount:  summary.AutoProcessedAmount,
		ManualApprovalAmount: summary.ManualApprovalAmount,
		Success:              true,
		Message:              "Payouts processed successfully",
	}, nil
}

// ProcessBigWinPayout approves or rejects a big win payout
func (s *DrawServiceServer) ProcessBigWinPayout(ctx context.Context, req *drawpb.ProcessBigWinPayoutRequest) (*drawpb.ProcessBigWinPayoutResponse, error) {
	s.logger.Printf("ProcessBigWinPayout called: draw_id=%s, ticket_id=%s, approve=%v, processed_by=%s",
		req.DrawId, req.TicketId, req.Approve, req.ProcessedBy)

	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		s.logger.Printf("Invalid draw_id: %v", err)
		return &drawpb.ProcessBigWinPayoutResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid draw_id: %v", err),
		}, nil
	}

	payout, err := s.drawService.ProcessBigWinPayout(ctx, drawID, req.TicketId, req.Approve, req.ProcessedBy, req.RejectionReason)
	if err != nil {
		s.logger.Printf("Failed to process big win payout: %v", err)
		return &drawpb.ProcessBigWinPayoutResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to process big win payout: %v", err),
		}, nil
	}

	s.logger.Printf("Big win payout processed: ticket_id=%s, status=%s", req.TicketId, payout.Status)

	protoPayout := &drawpb.BigWinPayout{
		TicketId:        payout.TicketID,
		Amount:          payout.Amount,
		Status:          payout.Status,
		ApprovedBy:      payout.ApprovedBy,
		RejectionReason: payout.RejectionReason,
	}
	if payout.ProcessedAt != nil {
		protoPayout.ProcessedAt = timestamppb.New(*payout.ProcessedAt)
	}

	return &drawpb.ProcessBigWinPayoutResponse{
		Payout:  protoPayout,
		Success: true,
		Message: fmt.Sprintf("Big win payout %s successfully", payout.Status),
	}, nil
}

// CompleteDraw finalizes the draw execution workflow
func (s *DrawServiceServer) CompleteDraw(ctx context.Context, req *drawpb.CompleteDrawRequest) (*drawpb.CompleteDrawResponse, error) {
	s.logger.Printf("CompleteDraw called: draw_id=%s, completed_by=%s", req.DrawId, req.CompletedBy)

	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		s.logger.Printf("Invalid draw_id: %v", err)
		return &drawpb.CompleteDrawResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid draw_id: %v", err),
		}, nil
	}

	draw, err := s.drawService.CompleteDraw(ctx, drawID, req.CompletedBy)
	if err != nil {
		s.logger.Printf("Failed to complete draw: %v", err)
		return &drawpb.CompleteDrawResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to complete draw: %v", err),
		}, nil
	}

	s.logger.Printf("Draw completed successfully: draw_id=%s, status=%s", draw.ID, draw.Status)

	return &drawpb.CompleteDrawResponse{
		Draw:    convertDrawToProto(draw),
		Success: true,
		Message: "Draw completed successfully",
	}, nil
}

// UpdateMachineNumbers updates the machine numbers for a completed draw
func (s *DrawServiceServer) UpdateMachineNumbers(ctx context.Context, req *drawpb.UpdateMachineNumbersRequest) (*drawpb.UpdateMachineNumbersResponse, error) {
	s.logger.Printf("UpdateMachineNumbers called: draw_id=%s, updated_by=%s, machine_numbers=%v",
		req.DrawId, req.UpdatedBy, req.MachineNumbers)

	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		s.logger.Printf("Invalid draw_id: %v", err)
		return &drawpb.UpdateMachineNumbersResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid draw_id: %v", err),
		}, nil
	}

	draw, err := s.drawService.UpdateMachineNumbers(ctx, drawID, req.MachineNumbers, req.UpdatedBy)
	if err != nil {
		s.logger.Printf("Failed to update machine numbers: %v", err)
		return &drawpb.UpdateMachineNumbersResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update machine numbers: %v", err),
		}, nil
	}

	s.logger.Printf("Machine numbers updated successfully: draw_id=%s, machine_numbers=%v", draw.ID, draw.MachineNumbers)

	return &drawpb.UpdateMachineNumbersResponse{
		Draw:    convertDrawToProto(draw),
		Success: true,
		Message: "Machine numbers updated successfully",
	}, nil
}

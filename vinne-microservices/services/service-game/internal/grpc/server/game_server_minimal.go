package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	pb "github.com/randco/randco-microservices/proto/game/v1"
	"github.com/randco/randco-microservices/services/service-game/internal/grpc/clients"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"github.com/randco/randco-microservices/services/service-game/internal/services"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GameServerMinimal implements the minimal gRPC GameService
type GameServerMinimal struct {
	pb.UnimplementedGameServiceServer
	gameService         services.GameService
	gameScheduleService services.GameScheduleService
	drawClient          clients.DrawServiceClient // optional: creates Draw records in draw service
}

// NewGameServerMinimal creates a new minimal game server
func NewGameServerMinimal(gameService services.GameService, gameScheduleService services.GameScheduleService, drawClient clients.DrawServiceClient) *GameServerMinimal {
	return &GameServerMinimal{
		gameService:         gameService,
		gameScheduleService: gameScheduleService,
		drawClient:          drawClient,
	}
}

// CreateGame creates a new game
func (s *GameServerMinimal) CreateGame(ctx context.Context, req *pb.CreateGameRequest) (*pb.CreateGameResponse, error) {
	log.Printf("[GameServerMinimal] CreateGame called with code: %s, name: %s", req.Code, req.Name)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "CreateGame"),
		attribute.String("game.name", req.Name),
		attribute.String("game.code", req.Code),
	)

	// Parse draw_date from YYYY-MM-DD — sets both start_date and end_date in DB
	var createDrawDate *time.Time
	fmt.Printf("[GameServerMinimal] CreateGame: draw_date=%q, draw_frequency=%q\n", req.DrawDate, req.DrawFrequency)
	if req.DrawDate != "" {
		if t, err := time.Parse("2006-01-02", req.DrawDate); err == nil {
			createDrawDate = &t
		}
	}

	// Parse prize_details JSON string to []models.PrizeDetail
	var createPrizes []models.PrizeDetail
	if req.PrizeDetails != "" {
		if err := json.Unmarshal([]byte(req.PrizeDetails), &createPrizes); err != nil {
			log.Printf("[GameServerMinimal] CreateGame: failed to parse prize_details JSON: %v", err)
			// Non-fatal — proceed with empty prize list
		}
	}

	// Convert proto request to service request
	serviceReq := models.CreateGameRequest{
		Code:                req.Code,
		Name:                req.Name,
		Organizer:           req.Organizer,
		GameCategory:        req.GameCategory,
		GameFormat:          req.GameFormat,
		GameType:            convertProtoStringPtr(req.GameType),
		BetTypes:            convertProtoBetTypes(req.BetTypes),
		NumberRangeMin:      req.NumberRangeMin,
		NumberRangeMax:      req.NumberRangeMax,
		SelectionCount:      req.SelectionCount,
		StartTime:           convertProtoTimeStringPtr(req.StartTime),
		EndTime:             convertProtoTimeStringPtr(req.EndTime),
		DrawFrequency:       req.DrawFrequency,
		DrawDays:            req.DrawDays,
		DrawTime:            convertProtoTimeStringPtr(req.DrawTime),
		SalesCutoffMinutes:  req.SalesCutoffMinutes,
		MinStake:            req.MinStake,
		MaxStake:            req.MaxStake,
		BasePrice:           req.BasePrice,
		MaxTicketsPerPlayer: req.MaxTicketsPerPlayer,
		MultiDrawEnabled:    req.MultiDrawEnabled,
		MaxDrawsAdvance:     convertProtoInt32Ptr(req.MaxDrawsAdvance),
		WeeklySchedule:      convertProtoBoolPtr(req.WeeklySchedule),
		Description:         convertProtoStringPtr(req.Description),
		PrizeDetails:        createPrizes,
		Rules:               convertProtoStringPtr(req.Rules),
		TotalTickets:        req.TotalTickets,
		StartDate:           createDrawDate,
		EndDate:             createDrawDate,
		Status:              strings.ToUpper(req.Status),
	}

	// Call service
	log.Printf("[GameServerMinimal] Calling gameService.CreateGame with request: %+v", serviceReq)
	game, err := s.gameService.CreateGame(ctx, serviceReq)
	if err != nil {
		log.Printf("[GameServerMinimal] Error creating game: %v", err)
		span.RecordError(err)
		return &pb.CreateGameResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}
	log.Printf("[GameServerMinimal] Game created successfully: ID=%s, Code=%s", game.ID, game.Code)

	// Convert to protobuf response
	return &pb.CreateGameResponse{
		Game:    convertGameToProto(game),
		Success: true,
		Message: "Game created successfully",
	}, nil
}

// ListGames lists games with filters
func (s *GameServerMinimal) ListGames(ctx context.Context, req *pb.ListGamesRequest) (*pb.ListGamesResponse, error) {
	log.Printf("[GameServerMinimal] ListGames called with page: %d, perPage: %d", req.Page, req.PerPage)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "ListGames"),
		attribute.Int64("pagination.page", int64(req.Page)),
		attribute.Int64("pagination.per_page", int64(req.PerPage)),
	)

	// Convert protobuf filters to service filters
	filter := models.GameFilter{
		Organizer:   convertProtoOrganizerFilter(req.OrganizerFilter),
		Status:      convertProtoStatusFilter(req.StatusFilter),
		SearchQuery: nil, // Not in proto anymore
	}

	// Default pagination values
	page := int(req.Page)
	perPage := int(req.PerPage)
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	// Call service
	log.Printf("[GameServerMinimal] Calling gameService.ListGames with filter: %+v, page: %d, perPage: %d", filter, page, perPage)
	games, total, err := s.gameService.ListGames(ctx, filter, page, perPage)
	if err != nil {
		log.Printf("[GameServerMinimal] Error listing games: %v", err)
		span.RecordError(err)
		return &pb.ListGamesResponse{
			Success: false,
			Message: err.Error(),
			Games:   []*pb.Game{},
			Total:   0,
			Page:    req.Page,
			PerPage: req.PerPage,
		}, nil
	}
	log.Printf("[GameServerMinimal] Found %d games, total: %d", len(games), total)

	// Convert games to protobuf
	pbGames := make([]*pb.Game, 0, len(games))
	for _, game := range games {
		pbGames = append(pbGames, convertGameToProto(game))
	}

	span.SetAttributes(
		attribute.Int("games.returned", len(pbGames)),
		attribute.Int64("games.total", total),
	)

	return &pb.ListGamesResponse{
		Games:   pbGames,
		Total:   int32(total),
		Page:    req.Page,
		PerPage: req.PerPage,
		Success: true,
		Message: "Games retrieved successfully",
	}, nil
}

// GetGame retrieves a single game by ID
func (s *GameServerMinimal) GetGame(ctx context.Context, req *pb.GetGameRequest) (*pb.GetGameResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "GetGame"),
		attribute.String("game.id", req.Id),
	)

	// Parse UUID
	id, err := uuid.Parse(req.Id)
	if err != nil {
		span.RecordError(err)
		return &pb.GetGameResponse{
			Success: false,
			Message: "Invalid game ID format",
		}, nil
	}

	// Call service
	game, err := s.gameService.GetGame(ctx, id)
	if err != nil {
		span.RecordError(err)
		return &pb.GetGameResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Convert to protobuf response
	return &pb.GetGameResponse{
		Game:    convertGameToProto(game),
		Success: true,
		Message: "Game retrieved successfully",
	}, nil
}

// UpdateGame updates an existing game
func (s *GameServerMinimal) UpdateGame(ctx context.Context, req *pb.UpdateGameRequest) (*pb.UpdateGameResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "UpdateGame"),
		attribute.String("game.id", req.Id),
	)

	// Parse UUID
	id, err := uuid.Parse(req.Id)
	if err != nil {
		span.RecordError(err)
		return &pb.UpdateGameResponse{
			Success: false,
			Message: "Invalid game ID format",
		}, nil
	}

	// Parse draw time if provided (validation will happen in service layer)
	var drawTime *time.Time
	if req.DrawTime != "" {
		if parsed, err := time.Parse("15:04", req.DrawTime); err == nil {
			drawTime = &parsed
		} else {
			span.RecordError(err)
			return &pb.UpdateGameResponse{
				Success: false,
				Message: fmt.Sprintf("Invalid draw time format '%s': %v. Expected HH:MM format", req.DrawTime, err),
			}, nil
		}
	}

	// Parse draw_date from start_date/end_date proto fields
	var updateDrawDate *time.Time
	if req.DrawDate != "" {
		if parsed, perr := time.Parse("2006-01-02", req.DrawDate); perr == nil {
			updateDrawDate = &parsed
		}
	}

	// Parse prize_details JSON string to []models.PrizeDetail.
	var updatePrizes []models.PrizeDetail
	if req.PrizeDetails != "" {
		if err := json.Unmarshal([]byte(req.PrizeDetails), &updatePrizes); err != nil {
			log.Printf("[GameServerMinimal] UpdateGame: failed to parse prize_details JSON: %v", err)
			updatePrizes = nil
		}
	}

	fmt.Printf("[GameServerMinimal] UpdateGame: draw_date=%q prize_details=%q\n", req.DrawDate, req.PrizeDetails)
	serviceReq := models.UpdateGameRequest{
		ID:                  id,
		Name:                convertProtoStringPtr(req.Name),
		Description:         convertProtoStringPtr(req.Description),
		DrawFrequency:       convertProtoStringPtr(req.DrawFrequency),
		DrawDays:            req.DrawDays,
		DrawTime:            drawTime,
		SalesCutoffMinutes:  convertProtoInt32Ptr(req.SalesCutoffMinutes),
		StartTime:           convertProtoTimeStringPtr(req.StartTime),
		EndTime:             convertProtoTimeStringPtr(req.EndTime),
		MinStake:            convertProtoFloat64Ptr(req.MinStake),
		MaxStake:            convertProtoFloat64Ptr(req.MaxStake),
		BasePrice:           convertProtoFloat64Ptr(req.BasePrice),
		MaxTicketsPerPlayer: convertProtoInt32Ptr(req.MaxTicketsPerPlayer),
		MultiDrawEnabled:    convertProtoBoolPtr(req.MultiDrawEnabled),
		MaxDrawsAdvance:     convertProtoInt32Ptr(req.MaxDrawsAdvance),
		WeeklySchedule:      convertProtoBoolPtr(req.WeeklySchedule),
		PrizeDetails:        updatePrizes,
		Rules:               convertProtoStringPtr(req.Rules),
		TotalTickets:        convertProtoInt32Ptr(req.TotalTickets),
		DrawDate:            updateDrawDate,
	}

	// Call service
	game, err := s.gameService.UpdateGame(ctx, serviceReq)
	if err != nil {
		span.RecordError(err)
		return &pb.UpdateGameResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Convert to protobuf response
	return &pb.UpdateGameResponse{
		Game:    convertGameToProto(game),
		Success: true,
		Message: "Game updated successfully",
	}, nil
}

// DeleteGame deletes a game
func (s *GameServerMinimal) DeleteGame(ctx context.Context, req *pb.DeleteGameRequest) (*pb.DeleteGameResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "DeleteGame"),
		attribute.String("game.id", req.Id),
	)

	// Parse UUID
	id, err := uuid.Parse(req.Id)
	if err != nil {
		span.RecordError(err)
		return &pb.DeleteGameResponse{
			Success: false,
			Message: "Invalid game ID format",
		}, nil
	}

	// Call service
	err = s.gameService.DeleteGame(ctx, id)
	if err != nil {
		span.RecordError(err)
		return &pb.DeleteGameResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.DeleteGameResponse{
		Success: true,
		Message: "Game deleted successfully",
	}, nil
}

// ActivateGame activates a game
func (s *GameServerMinimal) ActivateGame(ctx context.Context, req *pb.ActivateGameRequest) (*pb.ActivateGameResponse, error) {
	log.Printf("[GameServerMinimal] ActivateGame called with gameID: %s", req.Id)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "ActivateGame"),
		attribute.String("game.id", req.Id),
	)

	// Parse UUID
	id, err := uuid.Parse(req.Id)
	if err != nil {
		span.RecordError(err)
		return &pb.ActivateGameResponse{
			Success: false,
			Message: "Invalid game ID format",
		}, nil
	}

	// Call service
	log.Printf("[GameServerMinimal] Calling gameService.ActivateGame for ID: %s", id.String())
	game, err := s.gameService.ActivateGame(ctx, id)
	if err != nil {
		log.Printf("[GameServerMinimal] ActivateGame failed: %v", err)
		span.RecordError(err)
		return &pb.ActivateGameResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}
	log.Printf("[GameServerMinimal] ActivateGame successful for game: %s", game.Code)

	// Convert to protobuf response
	return &pb.ActivateGameResponse{
		Game:    convertGameToProto(game),
		Success: true,
		Message: "Game activated successfully",
	}, nil
}

// SuspendGame suspends an active game
func (s *GameServerMinimal) SuspendGame(ctx context.Context, req *pb.SuspendGameRequest) (*pb.SuspendGameResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "SuspendGame"),
		attribute.String("game.id", req.Id),
	)

	// Parse UUID
	id, err := uuid.Parse(req.Id)
	if err != nil {
		span.RecordError(err)
		return &pb.SuspendGameResponse{
			Success: false,
			Message: "Invalid game ID format",
		}, nil
	}

	// Call service
	game, err := s.gameService.SuspendGame(ctx, id)
	if err != nil {
		span.RecordError(err)
		return &pb.SuspendGameResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Convert to protobuf response
	return &pb.SuspendGameResponse{
		Game:    convertGameToProto(game),
		Success: true,
		Message: "Game suspended successfully",
	}, nil
}

// The rest of the methods can return not implemented for now
func (s *GameServerMinimal) CreateGameRules(ctx context.Context, req *pb.CreateGameRulesRequest) (*pb.CreateGameRulesResponse, error) {
	return &pb.CreateGameRulesResponse{
		Success: false,
		Message: "Not implemented",
	}, nil
}

func (s *GameServerMinimal) GetGameRules(ctx context.Context, req *pb.GetGameRulesRequest) (*pb.GetGameRulesResponse, error) {
	return &pb.GetGameRulesResponse{
		Success: false,
		Message: "Not implemented",
	}, nil
}

func (s *GameServerMinimal) UpdateGameRules(ctx context.Context, req *pb.UpdateGameRulesRequest) (*pb.UpdateGameRulesResponse, error) {
	return &pb.UpdateGameRulesResponse{
		Success: false,
		Message: "Not implemented",
	}, nil
}

func (s *GameServerMinimal) CreatePrizeStructure(ctx context.Context, req *pb.CreatePrizeStructureRequest) (*pb.CreatePrizeStructureResponse, error) {
	return &pb.CreatePrizeStructureResponse{
		Success: false,
		Message: "Not implemented",
	}, nil
}

func (s *GameServerMinimal) GetPrizeStructure(ctx context.Context, req *pb.GetPrizeStructureRequest) (*pb.GetPrizeStructureResponse, error) {
	return &pb.GetPrizeStructureResponse{
		Success: false,
		Message: "Not implemented",
	}, nil
}

func (s *GameServerMinimal) UpdatePrizeStructure(ctx context.Context, req *pb.UpdatePrizeStructureRequest) (*pb.UpdatePrizeStructureResponse, error) {
	return &pb.UpdatePrizeStructureResponse{
		Success: false,
		Message: "Not implemented",
	}, nil
}

// Schedule methods are removed from proto - commenting out for now
/*
func (s *GameServerMinimal) CreateSchedule(ctx context.Context, req *pb.CreateScheduleRequest) (*pb.CreateScheduleResponse, error) {
	return &pb.CreateScheduleResponse{
		Success: false,
		Message: "Not implemented",
	}, nil
}

func (s *GameServerMinimal) GetSchedules(ctx context.Context, req *pb.GetSchedulesRequest) (*pb.GetSchedulesResponse, error) {
	return &pb.GetSchedulesResponse{
		Success: false,
		Message: "Not implemented",
	}, nil
}

func (s *GameServerMinimal) UpdateSchedule(ctx context.Context, req *pb.UpdateScheduleRequest) (*pb.UpdateScheduleResponse, error) {
	return &pb.UpdateScheduleResponse{
		Success: false,
		Message: "Not implemented",
	}, nil
}
*/

func (s *GameServerMinimal) SubmitForApproval(ctx context.Context, req *pb.SubmitForApprovalRequest) (*pb.SubmitForApprovalResponse, error) {
	log.Printf("[GameServerMinimal] SubmitForApproval called - THIS SHOULD NOT BE CALLED, SHOULD USE GameServerWithApproval")
	return &pb.SubmitForApprovalResponse{
		Success: false,
		Message: "Not implemented",
	}, nil
}

func (s *GameServerMinimal) ApproveGame(ctx context.Context, req *pb.ApproveGameRequest) (*pb.ApproveGameResponse, error) {
	return &pb.ApproveGameResponse{
		Success: false,
		Message: "Not implemented",
	}, nil
}

func (s *GameServerMinimal) RejectGame(ctx context.Context, req *pb.RejectGameRequest) (*pb.RejectGameResponse, error) {
	return &pb.RejectGameResponse{
		Success: false,
		Message: "Not implemented",
	}, nil
}

// GetApprovals is removed from proto - commenting out for now
/*
func (s *GameServerMinimal) GetApprovals(ctx context.Context, req *pb.GetApprovalsRequest) (*pb.GetApprovalsResponse, error) {
	return &pb.GetApprovalsResponse{
		Success: false,
		Message: "Not implemented",
	}, nil
}
*/

// ScheduleGame creates a schedule entry for a specific game and immediately creates a Draw record
func (s *GameServerMinimal) ScheduleGame(ctx context.Context, req *pb.ScheduleGameRequest) (*pb.ScheduleGameResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "ScheduleGame"),
		attribute.String("game_id", req.GameId),
	)

	if s.gameScheduleService == nil {
		return &pb.ScheduleGameResponse{Success: false, Message: "Schedule service not available"}, nil
	}

	gameID, err := uuid.Parse(req.GameId)
	if err != nil {
		return &pb.ScheduleGameResponse{Success: false, Message: "Invalid game ID format"}, nil
	}

	start := req.ScheduledStart.AsTime()
	end := req.ScheduledEnd.AsTime()
	draw := req.ScheduledDraw.AsTime()

	schedule, err := s.gameScheduleService.ScheduleGame(ctx, gameID, start, end, draw, req.Frequency)
	if err != nil {
		span.RecordError(err)
		return &pb.ScheduleGameResponse{Success: false, Message: err.Error()}, nil
	}

	// Attach notes from request if provided
	if req.Notes != "" && schedule.Notes == nil {
		schedule.Notes = &req.Notes
	}

	// Create a Draw record in the draw service so it's immediately visible
	if s.drawClient != nil {
		game, gameErr := s.gameService.GetGame(ctx, gameID)
		if gameErr == nil && game != nil {
			if _, drawErr := s.drawClient.CreateDraw(ctx, game, schedule, nil); drawErr != nil {
				log.Printf("[ScheduleGame] WARNING: failed to create draw record for schedule %s: %v", schedule.ID, drawErr)
			}
		}
	}

	return &pb.ScheduleGameResponse{
		Schedule: convertGameScheduleToProto(schedule),
		Success:  true,
		Message:  "Game scheduled successfully",
	}, nil
}

// GetGameSchedule retrieves all schedules for a specific game
func (s *GameServerMinimal) GetGameSchedule(ctx context.Context, req *pb.GetGameScheduleRequest) (*pb.GetGameScheduleResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "GetGameSchedule"),
		attribute.String("game_id", req.GameId),
	)

	if s.gameScheduleService == nil {
		return &pb.GetGameScheduleResponse{Success: false, Message: "Schedule service not available"}, nil
	}

	gameID, err := uuid.Parse(req.GameId)
	if err != nil {
		return &pb.GetGameScheduleResponse{Success: false, Message: "Invalid game ID format"}, nil
	}

	schedules, err := s.gameScheduleService.GetGameSchedule(ctx, gameID)
	if err != nil {
		span.RecordError(err)
		return &pb.GetGameScheduleResponse{Success: false, Message: err.Error()}, nil
	}

	pbSchedules := make([]*pb.GameSchedule, 0, len(schedules))
	for _, sc := range schedules {
		pbSchedules = append(pbSchedules, convertGameScheduleToProto(sc))
	}

	return &pb.GetGameScheduleResponse{
		Schedules: pbSchedules,
		Success:   true,
		Message:   fmt.Sprintf("Found %d schedules", len(pbSchedules)),
	}, nil
}

func (s *GameServerMinimal) UpdateScheduledGame(ctx context.Context, req *pb.UpdateScheduledGameRequest) (*pb.UpdateScheduledGameResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "UpdateScheduledGame"),
		attribute.String("schedule_id", req.ScheduleId),
	)

	// Check if schedule service is available
	if s.gameScheduleService == nil {
		return &pb.UpdateScheduledGameResponse{
			Success: false,
			Message: "Schedule service not available",
		}, nil
	}

	// Parse schedule ID
	scheduleID, err := uuid.Parse(req.ScheduleId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid schedule ID")
		return &pb.UpdateScheduledGameResponse{
			Success: false,
			Message: "Invalid schedule ID format",
		}, nil
	}

	// Build update request
	updateReq := &models.UpdateGameScheduleRequest{}

	if req.ScheduledEnd != nil {
		scheduledEnd := req.ScheduledEnd.AsTime()
		updateReq.ScheduledEnd = &scheduledEnd
	}

	if req.ScheduledDraw != nil {
		scheduledDraw := req.ScheduledDraw.AsTime()
		updateReq.ScheduledDraw = &scheduledDraw
	}

	if req.Status != "" {
		status := models.ScheduleStatus(req.Status)
		updateReq.Status = &status
	}

	if req.IsActive {
		updateReq.IsActive = &req.IsActive
	}

	if req.Notes != "" {
		updateReq.Notes = &req.Notes
	}

	// Update scheduled game
	schedule, err := s.gameScheduleService.UpdateScheduledGame(ctx, scheduleID, updateReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update scheduled game")
		return &pb.UpdateScheduledGameResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update scheduled game: %v", err),
		}, nil
	}

	// Convert to protobuf
	pbSchedule := &pb.GameSchedule{
		Id:     schedule.ID.String(),
		GameId: schedule.GameID.String(),
		GameName: func() string {
			if schedule.GameName != nil {
				return *schedule.GameName
			}
			return ""
		}(),
		ScheduledStart: timestamppb.New(schedule.ScheduledStart),
		ScheduledEnd:   timestamppb.New(schedule.ScheduledEnd),
		ScheduledDraw:  timestamppb.New(schedule.ScheduledDraw),
		Frequency:      string(schedule.Frequency),
		IsActive:       schedule.IsActive,
		Status:         string(schedule.Status),
		Notes: func() string {
			if schedule.Notes != nil {
				return *schedule.Notes
			}
			return ""
		}(),
		DrawResultId: func() string {
			if schedule.DrawResultID != nil {
				return schedule.DrawResultID.String()
			}
			return ""
		}(),
	}

	span.SetStatus(codes.Ok, "scheduled game updated successfully")
	return &pb.UpdateScheduledGameResponse{
		Schedule: pbSchedule,
		Success:  true,
		Message:  "Scheduled game updated successfully",
	}, nil
}

// Weekly scheduling endpoints
func (s *GameServerMinimal) GenerateWeeklySchedule(ctx context.Context, req *pb.GenerateWeeklyScheduleRequest) (*pb.GenerateWeeklyScheduleResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "GenerateWeeklySchedule"),
		attribute.String("week_start", req.WeekStart.AsTime().Format("2006-01-02")),
	)

	// Check if schedule service is available
	if s.gameScheduleService == nil {
		return &pb.GenerateWeeklyScheduleResponse{
			Success: false,
			Message: "Schedule service not available",
		}, nil
	}

	// Convert protobuf timestamp to Go time
	weekStart := req.WeekStart.AsTime()

	// Call service
	schedules, err := s.gameScheduleService.GenerateWeeklySchedule(ctx, weekStart)
	if err != nil {
		span.RecordError(err)
		return &pb.GenerateWeeklyScheduleResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Convert schedules to protobuf and create Draw records for each schedule
	pbSchedules := make([]*pb.GameSchedule, 0, len(schedules))
	drawsCreated := 0
	for _, schedule := range schedules {
		pbSchedules = append(pbSchedules, convertGameScheduleToProto(schedule))

		// Create Draw record immediately so it's visible in the Draws page
		if s.drawClient != nil {
			game, gameErr := s.gameService.GetGame(ctx, schedule.GameID)
			if gameErr == nil && game != nil {
				if _, drawErr := s.drawClient.CreateDraw(ctx, game, schedule, nil); drawErr != nil {
					log.Printf("[GenerateWeeklySchedule] WARNING: failed to create draw for schedule %s: %v", schedule.ID, drawErr)
				} else {
					drawsCreated++
				}
			}
		}
	}

	span.SetAttributes(
		attribute.Int("schedules.created", len(pbSchedules)),
		attribute.Int("draws.created", drawsCreated),
	)

	return &pb.GenerateWeeklyScheduleResponse{
		Schedules:        pbSchedules,
		Success:          true,
		Message:          fmt.Sprintf("Weekly schedule generated successfully (%d schedules, %d draws created)", len(pbSchedules), drawsCreated),
		SchedulesCreated: int32(len(pbSchedules)),
	}, nil
}

func (s *GameServerMinimal) GetWeeklySchedule(ctx context.Context, req *pb.GetWeeklyScheduleRequest) (*pb.GetWeeklyScheduleResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "GetWeeklySchedule"),
		attribute.String("week_start", req.WeekStart.AsTime().Format("2006-01-02")),
	)

	// Convert protobuf timestamp to Go time
	weekStart := req.WeekStart.AsTime()

	// Call service
	schedules, err := s.gameScheduleService.GetWeeklySchedule(ctx, weekStart)
	if err != nil {
		span.RecordError(err)
		return &pb.GetWeeklyScheduleResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Convert schedules to protobuf
	pbSchedules := make([]*pb.GameSchedule, 0, len(schedules))
	for _, schedule := range schedules {
		pbSchedules = append(pbSchedules, convertGameScheduleToProto(schedule))
	}

	span.SetAttributes(attribute.Int("schedules.found", len(pbSchedules)))

	return &pb.GetWeeklyScheduleResponse{
		Schedules: pbSchedules,
		Success:   true,
		Message:   "Weekly schedule retrieved successfully",
	}, nil
}

func (s *GameServerMinimal) ClearWeeklySchedule(ctx context.Context, req *pb.ClearWeeklyScheduleRequest) (*pb.ClearWeeklyScheduleResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "ClearWeeklySchedule"),
		attribute.String("week_start", req.WeekStart.AsTime().Format("2006-01-02")),
	)

	// Convert protobuf timestamp to Go time
	weekStart := req.WeekStart.AsTime()

	// Call service
	err := s.gameScheduleService.ClearWeeklySchedule(ctx, weekStart)
	if err != nil {
		span.RecordError(err)
		return &pb.ClearWeeklyScheduleResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.ClearWeeklyScheduleResponse{
		Success: true,
		Message: "Weekly schedule cleared successfully",
	}, nil
}

// Helper conversion functions for protobuf to Go types
func convertProtoFloat64Ptr(value float64) *float64 {
	if value == 0 {
		return nil
	}
	return &value
}

// convertGameScheduleToProto converts models.GameSchedule to proto.GameSchedule
func convertGameScheduleToProto(schedule *models.GameSchedule) *pb.GameSchedule {
	notes := ""
	if schedule.Notes != nil {
		notes = *schedule.Notes
	}
	gameName := ""
	if schedule.GameName != nil {
		gameName = *schedule.GameName
	}
	gameCode := ""
	if schedule.GameCode != nil {
		gameCode = *schedule.GameCode
	}

	gameCategory := ""
	if schedule.GameCategory != nil {
		gameCategory = *schedule.GameCategory
	}
	logoURL := ""
	if schedule.LogoURL != nil {
		logoURL = *schedule.LogoURL
	}
	brandColor := ""
	if schedule.BrandColor != nil {
		brandColor = *schedule.BrandColor
	}
	drawResultID := ""
	if schedule.DrawResultID != nil {
		drawResultID = schedule.DrawResultID.String()
	}
	gs := &pb.GameSchedule{
		Id:             schedule.ID.String(),
		GameId:         schedule.GameID.String(),
		GameName:       gameName,
		GameCode:       gameCode,
		GameCategory:   gameCategory,
		LogoUrl:        logoURL,
		BrandColor:     brandColor,
		ScheduledStart: timestamppb.New(schedule.ScheduledStart),
		ScheduledEnd:   timestamppb.New(schedule.ScheduledEnd),
		ScheduledDraw:  timestamppb.New(schedule.ScheduledDraw),
		Frequency:      string(schedule.Frequency),
		IsActive:       schedule.IsActive,
		Status:         string(schedule.Status),
		DrawResultId:   drawResultID,
		Notes:          notes,
	}

	if len(schedule.BetTypes) > 0 {
		gs.BetTypes = make([]*pb.BetType, 0, len(schedule.BetTypes))
		for _, bt := range schedule.BetTypes {
			gs.BetTypes = append(gs.BetTypes, &pb.BetType{
				Id:         bt.ID,
				Name:       bt.Name,
				Multiplier: float32(bt.Multiplier),
				Enabled:    bt.Enabled,
			})
		}
	}

	return gs
}

// GetScheduleById retrieves a specific schedule by its ID
func (s *GameServerMinimal) GetScheduleById(ctx context.Context, req *pb.GetScheduleByIdRequest) (*pb.GetScheduleByIdResponse, error) {
	scheduleID, err := uuid.Parse(req.ScheduleId)
	if err != nil {
		return &pb.GetScheduleByIdResponse{
			Success: false,
			Message: "Invalid schedule ID format",
		}, nil
	}

	schedule, err := s.gameScheduleService.GetScheduledGameByID(ctx, scheduleID)
	if err != nil {
		return &pb.GetScheduleByIdResponse{
			Success: false,
			Message: "Schedule not found",
		}, nil
	}

	return &pb.GetScheduleByIdResponse{
		Schedule: convertGameScheduleToProto(schedule),
		Success:  true,
		Message:  "Schedule retrieved successfully",
	}, nil
}

// UpdateGameLogo updates the logo URL and brand color for a game
func (s *GameServerMinimal) UpdateGameLogo(ctx context.Context, req *pb.UpdateGameLogoRequest) (*pb.UpdateGameLogoResponse, error) {
	log.Printf("[GameServerMinimal] UpdateGameLogo called for game: %s", req.GameId)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "UpdateGameLogo"),
		attribute.String("game.id", req.GameId),
		attribute.String("logo.url", req.LogoUrl),
	)

	// Parse game ID
	gameID, err := uuid.Parse(req.GameId)
	if err != nil {
		log.Printf("[GameServerMinimal] Invalid game ID format: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid game ID")
		return &pb.UpdateGameLogoResponse{
			Success: false,
			Message: "Invalid game ID format",
		}, nil
	}

	// Prepare brand color pointer
	var brandColor *string
	if req.BrandColor != "" {
		brandColor = &req.BrandColor
	}

	// Verify game exists
	_, err = s.gameService.GetGame(ctx, gameID)
	if err != nil {
		log.Printf("[GameServerMinimal] Game not found: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return &pb.UpdateGameLogoResponse{
			Success: false,
			Message: "Game not found",
		}, nil
	}

	// Prepare logo URL pointer
	var logoURL *string
	if req.LogoUrl != "" {
		logoURL = &req.LogoUrl
	}

	// Update logo in database via game service
	// Note: The actual file upload happens via API Gateway before calling this
	// This handler just updates the database with the provided URL
	result, err := s.gameService.UpdateGameLogo(ctx, gameID, logoURL, brandColor)
	if err != nil {
		log.Printf("[GameServerMinimal] Failed to update logo: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "update failed")
		return &pb.UpdateGameLogoResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update logo: %v", err),
		}, nil
	}

	log.Printf("[GameServerMinimal] Logo updated successfully for game %s", gameID)
	span.SetStatus(codes.Ok, "logo updated")

	cdnURL := result.CDNURL
	if cdnURL == "" {
		cdnURL = result.LogoURL
	}

	return &pb.UpdateGameLogoResponse{
		Success:    true,
		Message:    "Game logo updated successfully",
		LogoUrl:    result.LogoURL,
		CdnUrl:     cdnURL,
		BrandColor: req.BrandColor,
	}, nil
}

// DeleteGameLogo removes the logo from a game
func (s *GameServerMinimal) DeleteGameLogo(ctx context.Context, req *pb.DeleteGameLogoRequest) (*pb.DeleteGameLogoResponse, error) {
	log.Printf("[GameServerMinimal] DeleteGameLogo called for game: %s", req.GameId)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "DeleteGameLogo"),
		attribute.String("game.id", req.GameId),
	)

	// Parse game ID
	gameID, err := uuid.Parse(req.GameId)
	if err != nil {
		log.Printf("[GameServerMinimal] Invalid game ID format: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid game ID")
		return &pb.DeleteGameLogoResponse{
			Success: false,
			Message: "Invalid game ID format",
		}, nil
	}

	// Call service to delete logo
	if err := s.gameService.DeleteGameLogo(ctx, gameID); err != nil {
		log.Printf("[GameServerMinimal] Error deleting game logo: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete logo")
		return &pb.DeleteGameLogoResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to delete game logo: %v", err),
		}, nil
	}

	log.Printf("[GameServerMinimal] Logo deleted successfully for game %s", gameID)
	span.SetStatus(codes.Ok, "logo deleted")

	return &pb.DeleteGameLogoResponse{
		Success: true,
		Message: "Game logo deleted successfully",
	}, nil
}

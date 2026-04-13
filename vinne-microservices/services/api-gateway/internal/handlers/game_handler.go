package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	gamepb "github.com/randco/randco-microservices/proto/game/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/response"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/services/api-gateway/internal/timeutil"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/randco-microservices/shared/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type gameHandler struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
	ntpTime     *timeutil.NTPTimeService
	storage     storage.Storage
}

// NewGameHandler creates a new game handler with gRPC integration
func NewGameHandler(grpcManager *grpc.ClientManager, log logger.Logger, ntpTime *timeutil.NTPTimeService, storageClient storage.Storage) *gameHandler {
	return &gameHandler{
		grpcManager: grpcManager,
		log:         log,
		ntpTime:     ntpTime,
		storage:     storageClient,
	}
}

// BetType represents a bet type with its configuration
type BetType struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Enabled    bool    `json:"enabled"`
	Multiplier float64 `json:"multiplier"`
}

// CreateGameRequest represents the JSON request from frontend
type CreateGameRequest struct {
	Code                string    `json:"code"`
	Name                string    `json:"name"`
	Organizer           string    `json:"organizer"`
	GameCategory        string    `json:"game_category"`
	GameFormat          string    `json:"game_format"`
	GameType            string    `json:"game_type,omitempty"`
	BetTypes            []BetType `json:"bet_types,omitempty"`
	NumberRangeMin      int32     `json:"number_range_min"`
	NumberRangeMax      int32     `json:"number_range_max"`
	SelectionCount      int32     `json:"selection_count"`
	StartTime           string    `json:"start_time,omitempty"`
	EndTime             string    `json:"end_time,omitempty"`
	DrawFrequency       string    `json:"draw_frequency"`
	DrawDays            []string  `json:"draw_days,omitempty"`
	DrawTime            string    `json:"draw_time,omitempty"`
	SalesCutoffMinutes  int32     `json:"sales_cutoff_minutes"`
	MinStake            float64   `json:"min_stake,omitempty"`
	MaxStake            float64   `json:"max_stake,omitempty"`
	BasePrice           float64   `json:"base_price"`
	MaxTicketsPerPlayer int32     `json:"max_tickets_per_player"`
	MultiDrawEnabled    bool      `json:"multi_draw_enabled"`
	MaxDrawsAdvance     int32     `json:"max_draws_advance,omitempty"`
	WeeklySchedule      bool      `json:"weekly_schedule,omitempty"`
	Description         string    `json:"description,omitempty"`
}

// UpdateGameRequest represents the JSON request from frontend for updating a game
type UpdateGameRequest struct {
	Name                *string  `json:"name,omitempty"`
	DrawFrequency       *string  `json:"draw_frequency,omitempty"`
	DrawDays            []string `json:"draw_days,omitempty"`
	DrawTime            *string  `json:"draw_time,omitempty"`
	SalesCutoffMinutes  *int32   `json:"sales_cutoff_minutes,omitempty"`
	StartTime           *string  `json:"start_time,omitempty"`
	EndTime             *string  `json:"end_time,omitempty"`
	MinStake            *float64 `json:"min_stake,omitempty"`
	MaxStake            *float64 `json:"max_stake,omitempty"`
	BasePrice           *float64 `json:"base_price,omitempty"`
	MaxTicketsPerPlayer *int32   `json:"max_tickets_per_player,omitempty"`
	MultiDrawEnabled    *bool    `json:"multi_draw_enabled,omitempty"`
	MaxDrawsAdvance     *int32   `json:"max_draws_advance,omitempty"`
	WeeklySchedule      *bool    `json:"weekly_schedule,omitempty"`
}

// CreateGame creates a new game
func (h *gameHandler) CreateGame(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var jsonReq CreateGameRequest
	if err := json.NewDecoder(r.Body).Decode(&jsonReq); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	// Log received bet types for debugging
	h.log.Debug("Received bet_types from frontend",
		"bet_types_count", len(jsonReq.BetTypes),
		"bet_types", jsonReq.BetTypes)

	// Convert bet types from JSON to proto
	var protoBetTypes []*gamepb.BetType
	for _, bt := range jsonReq.BetTypes {
		protoBetTypes = append(protoBetTypes, &gamepb.BetType{
			Id:         bt.ID,
			Name:       bt.Name,
			Enabled:    bt.Enabled,
			Multiplier: float32(bt.Multiplier),
		})
	}

	h.log.Debug("Converted bet_types to proto",
		"proto_bet_types_count", len(protoBetTypes))

	// Convert JSON request to proto request
	protoReq := &gamepb.CreateGameRequest{
		Code:                jsonReq.Code,
		Name:                jsonReq.Name,
		Organizer:           jsonReq.Organizer,
		GameCategory:        jsonReq.GameCategory,
		GameFormat:          jsonReq.GameFormat,
		GameType:            jsonReq.GameType,
		BetTypes:            protoBetTypes,
		NumberRangeMin:      jsonReq.NumberRangeMin,
		NumberRangeMax:      jsonReq.NumberRangeMax,
		SelectionCount:      jsonReq.SelectionCount,
		StartTime:           jsonReq.StartTime,
		EndTime:             jsonReq.EndTime,
		DrawFrequency:       jsonReq.DrawFrequency,
		DrawDays:            jsonReq.DrawDays,
		DrawTime:            jsonReq.DrawTime,
		SalesCutoffMinutes:  jsonReq.SalesCutoffMinutes,
		MinStake:            jsonReq.MinStake,
		MaxStake:            jsonReq.MaxStake,
		BasePrice:           jsonReq.BasePrice,
		MaxTicketsPerPlayer: jsonReq.MaxTicketsPerPlayer,
		MultiDrawEnabled:    jsonReq.MultiDrawEnabled,
		MaxDrawsAdvance:     jsonReq.MaxDrawsAdvance,
		WeeklySchedule:      jsonReq.WeeklySchedule,
		Description:         jsonReq.Description,
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	game, err := client.CreateGame(ctx, protoReq)
	if err != nil {
		h.log.Error("Failed to create game", "error", err)
		// Extract the actual error message from the gRPC error
		errorMessage := err.Error()
		// Try to extract validation errors which usually contain "validation failed:"
		if strings.Contains(errorMessage, "validation failed:") {
			return response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", errorMessage, nil)
		}
		return response.Error(w, http.StatusInternalServerError, "CREATE_FAILED", errorMessage, nil)
	}

	return response.Success(w, http.StatusCreated, "Game created successfully", game)
}

// GetGame retrieves a game by ID
func (h *gameHandler) GetGame(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	gameID := router.GetParam(r, "id")

	if gameID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Game ID is required", nil)
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.GetGameRequest{Id: gameID}
	game, err := client.GetGame(ctx, req)
	if err != nil {
		h.log.Error("Failed to get game", "error", err, "gameID", gameID)
		return response.Error(w, http.StatusInternalServerError, "GET_FAILED", "Failed to get game", err)
	}

	return response.Success(w, http.StatusOK, "Game retrieved successfully", game)
}

// ListGames retrieves all games with filtering
func (h *gameHandler) ListGames(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	req := &gamepb.ListGamesRequest{
		Page:    1,
		PerPage: 50,
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	resp, err := client.ListGames(ctx, req)
	if err != nil {
		h.log.Error("Failed to list games", "error", err)
		return response.Error(w, http.StatusInternalServerError, "LIST_FAILED", "Failed to list games", err)
	}

	// Format the response with games array
	games := resp.Games
	if games == nil {
		games = []*gamepb.Game{} // Return empty array if nil
	}

	data := map[string]interface{}{
		"games":    games,
		"total":    resp.Total,
		"page":     resp.Page,
		"per_page": resp.PerPage,
	}

	return response.Success(w, http.StatusOK, "Games retrieved successfully", data)
}

// UpdateGame updates an existing game
func (h *gameHandler) UpdateGame(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	gameID := router.GetParam(r, "id")

	if gameID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Game ID is required", nil)
	}

	var jsonReq UpdateGameRequest
	if err := json.NewDecoder(r.Body).Decode(&jsonReq); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	// Log sales_cutoff_minutes from request
	if jsonReq.SalesCutoffMinutes != nil {
		h.log.Info("UpdateGame: Received sales_cutoff_minutes from frontend",
			"game_id", gameID,
			"sales_cutoff_minutes", *jsonReq.SalesCutoffMinutes)
	} else {
		h.log.Info("UpdateGame: sales_cutoff_minutes is nil in request", "game_id", gameID)
	}

	// Convert JSON request to proto request
	protoReq := &gamepb.UpdateGameRequest{
		Id: gameID,
	}

	// Map all the fields from JSON to proto
	if jsonReq.Name != nil {
		protoReq.Name = *jsonReq.Name
	}
	if jsonReq.DrawFrequency != nil {
		protoReq.DrawFrequency = *jsonReq.DrawFrequency
	}
	if len(jsonReq.DrawDays) > 0 {
		protoReq.DrawDays = jsonReq.DrawDays
	}
	if jsonReq.DrawTime != nil {
		protoReq.DrawTime = *jsonReq.DrawTime
	}
	if jsonReq.SalesCutoffMinutes != nil {
		protoReq.SalesCutoffMinutes = *jsonReq.SalesCutoffMinutes
	}
	if jsonReq.StartTime != nil {
		protoReq.StartTime = *jsonReq.StartTime
	}
	if jsonReq.EndTime != nil {
		protoReq.EndTime = *jsonReq.EndTime
	}
	if jsonReq.MinStake != nil {
		protoReq.MinStake = *jsonReq.MinStake
	}
	if jsonReq.MaxStake != nil {
		protoReq.MaxStake = *jsonReq.MaxStake
	}
	if jsonReq.BasePrice != nil {
		protoReq.BasePrice = *jsonReq.BasePrice
	}
	if jsonReq.MaxTicketsPerPlayer != nil {
		protoReq.MaxTicketsPerPlayer = *jsonReq.MaxTicketsPerPlayer
	}
	if jsonReq.MultiDrawEnabled != nil {
		protoReq.MultiDrawEnabled = *jsonReq.MultiDrawEnabled
	}
	if jsonReq.MaxDrawsAdvance != nil {
		protoReq.MaxDrawsAdvance = *jsonReq.MaxDrawsAdvance
	}
	if jsonReq.WeeklySchedule != nil {
		protoReq.WeeklySchedule = *jsonReq.WeeklySchedule
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	game, err := client.UpdateGame(ctx, protoReq)
	if err != nil {
		h.log.Error("Failed to update game", "error", err, "gameID", gameID)
		return response.Error(w, http.StatusInternalServerError, "UPDATE_FAILED", "Failed to update game", err)
	}

	return response.Success(w, http.StatusOK, "Game updated successfully", game)
}

// DeleteGame deletes a game
func (h *gameHandler) DeleteGame(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	gameID := router.GetParam(r, "id")

	if gameID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Game ID is required", nil)
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.DeleteGameRequest{
		Id: gameID,
	}

	_, err = client.DeleteGame(ctx, req)
	if err != nil {
		h.log.Error("Failed to delete game", "error", err, "gameID", gameID)
		return response.Error(w, http.StatusInternalServerError, "DELETE_FAILED", "Failed to delete game", err)
	}

	return response.Success(w, http.StatusOK, "Game deleted successfully", nil)
}

// SubmitForApproval submits a game for approval workflow
func (h *gameHandler) SubmitForApproval(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Log request details
	h.log.Debug("SubmitForApproval handler called",
		"method", r.Method,
		"url", r.URL.String(),
		"path", r.URL.Path,
		"raw_path", r.URL.RawPath)

	// Try multiple methods to extract game ID
	// Get ID from route parameter
	gameID := router.GetParam(r, "id")
	h.log.Debug("Route parameter extraction", "gameID", gameID)

	if gameID == "" {
		h.log.Error("Failed to extract game ID", "path", r.URL.Path)
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Game ID is required", nil)
	}

	h.log.Info("Game ID extracted successfully", "gameID", gameID)

	// Get authenticated user ID from context
	userID, ok := ctx.Value(router.ContextUserID).(string)
	if !ok || userID == "" {
		h.log.Error("Failed to get user ID from context")
		return response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
	}

	h.log.Debug("Extracted user from context", "userID", userID)

	// Parse request body for notes (optional)
	var reqBody struct {
		Notes string `json:"notes"`
	}
	// Allow empty body since notes are optional
	body, _ := io.ReadAll(r.Body)
	if len(body) > 0 {
		if err := json.Unmarshal(body, &reqBody); err != nil {
			h.log.Debug("Failed to parse request body", "error", err)
			// Notes are optional, so continue without them
		}
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.SubmitForApprovalRequest{
		GameId:      gameID,
		SubmittedBy: userID, // Use authenticated user's ID
		Notes:       reqBody.Notes,
	}

	h.log.Debug("Sending gRPC request",
		"gameID", req.GameId,
		"submittedBy", req.SubmittedBy,
		"notes", req.Notes)

	resp, err := client.SubmitForApproval(ctx, req)
	if err != nil {
		h.log.Error("Failed to submit game for approval", "error", err, "gameID", gameID)
		return response.Error(w, http.StatusInternalServerError, "SUBMIT_FAILED", "Failed to submit game for approval", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusBadRequest, "SUBMIT_FAILED", resp.Message, nil)
	}

	return response.Success(w, http.StatusOK, resp.Message, resp.Approval)
}

// ApproveGame approves a game (handles both first and second approval)
func (h *gameHandler) ApproveGame(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Log request details
	h.log.Debug("ApproveGame handler called",
		"method", r.Method,
		"url", r.URL.String(),
		"path", r.URL.Path)

	// Try multiple methods to extract game ID
	// Get ID from route parameter
	gameID := router.GetParam(r, "id")
	h.log.Debug("Route parameter extraction", "gameID", gameID)

	if gameID == "" {
		h.log.Error("Failed to extract game ID", "path", r.URL.Path)
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Game ID is required", nil)
	}

	// Get authenticated user ID from context
	userID, ok := ctx.Value(router.ContextUserID).(string)
	if !ok || userID == "" {
		h.log.Error("Failed to get user ID from context")
		return response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
	}

	var reqBody struct {
		Notes string `json:"notes"`
	}

	// Allow empty body since notes are optional
	body, _ := io.ReadAll(r.Body)
	if len(body) > 0 {
		if err := json.Unmarshal(body, &reqBody); err != nil {
			h.log.Debug("Failed to parse request body", "error", err)
			// Notes are optional, so continue without them
		}
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.ApproveGameRequest{
		GameId:     gameID,
		ApprovedBy: userID, // Use authenticated user's ID
		Notes:      reqBody.Notes,
	}

	h.log.Debug("Sending gRPC approval request",
		"gameID", req.GameId,
		"approvedBy", req.ApprovedBy,
		"notes", req.Notes)

	resp, err := client.ApproveGame(ctx, req)
	if err != nil {
		h.log.Error("Failed to approve game", "error", err, "gameID", gameID)
		return response.Error(w, http.StatusInternalServerError, "APPROVE_FAILED", "Failed to approve game", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusBadRequest, "APPROVE_FAILED", resp.Message, nil)
	}

	data := map[string]interface{}{
		"approval_stage": resp.ApprovalStage,
		"approval":       resp.Approval,
	}

	return response.Success(w, http.StatusOK, resp.Message, data)
}

// RejectGame rejects a game
func (h *gameHandler) RejectGame(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Log request details
	h.log.Debug("RejectGame handler called",
		"method", r.Method,
		"url", r.URL.String(),
		"path", r.URL.Path)

	// Try multiple methods to extract game ID
	// Get ID from route parameter
	gameID := router.GetParam(r, "id")
	h.log.Debug("Route parameter extraction", "gameID", gameID)

	if gameID == "" {
		h.log.Error("Failed to extract game ID", "path", r.URL.Path)
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Game ID is required", nil)
	}

	// Get authenticated user ID from context
	userID, ok := ctx.Value(router.ContextUserID).(string)
	if !ok || userID == "" {
		h.log.Error("Failed to get user ID from context")
		return response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
	}

	var reqBody struct {
		Reason string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	// Reason is required for rejection
	if reqBody.Reason == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_REASON", "Rejection reason is required", nil)
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.RejectGameRequest{
		GameId:     gameID,
		RejectedBy: userID, // Use authenticated user's ID
		Reason:     reqBody.Reason,
	}

	h.log.Debug("Sending gRPC rejection request",
		"gameID", req.GameId,
		"rejectedBy", req.RejectedBy,
		"reason", req.Reason)

	resp, err := client.RejectGame(ctx, req)
	if err != nil {
		h.log.Error("Failed to reject game", "error", err, "gameID", gameID)
		return response.Error(w, http.StatusInternalServerError, "REJECT_FAILED", "Failed to reject game", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusBadRequest, "REJECT_FAILED", resp.Message, nil)
	}

	return response.Success(w, http.StatusOK, resp.Message, resp.Approval)
}

// GetApprovalStatus gets the approval status for a game
func (h *gameHandler) GetApprovalStatus(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	gameID := router.GetParam(r, "id")

	if gameID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Game ID is required", nil)
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.GetApprovalStatusRequest{
		GameId: gameID,
	}

	resp, err := client.GetApprovalStatus(ctx, req)
	if err != nil {
		h.log.Error("Failed to get approval status", "error", err, "gameID", gameID)
		return response.Error(w, http.StatusInternalServerError, "GET_FAILED", "Failed to get approval status", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusBadRequest, "GET_FAILED", resp.Message, nil)
	}

	return response.Success(w, http.StatusOK, resp.Message, resp.Approval)
}

// GetPendingApprovals gets pending approvals based on type
func (h *gameHandler) GetPendingApprovals(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Get query parameters
	approvalType := r.URL.Query().Get("type")
	if approvalType == "" {
		approvalType = "all"
	}

	page := 1
	perPage := 20

	// Parse pagination parameters
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if perPageStr := r.URL.Query().Get("per_page"); perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
			perPage = pp
		}
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.GetPendingApprovalsRequest{
		ApprovalType: approvalType,
		Page:         int32(page),
		PerPage:      int32(perPage),
	}

	resp, err := client.GetPendingApprovals(ctx, req)
	if err != nil {
		h.log.Error("Failed to get pending approvals", "error", err)
		return response.Error(w, http.StatusInternalServerError, "GET_FAILED", "Failed to get pending approvals", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusBadRequest, "GET_FAILED", resp.Message, nil)
	}

	approvals := resp.Approvals
	if approvals == nil {
		approvals = []*gamepb.GameApproval{}
	}

	data := map[string]interface{}{
		"approvals": approvals,
		"total":     resp.Total,
		"page":      resp.Page,
		"per_page":  resp.PerPage,
	}

	return response.Success(w, http.StatusOK, resp.Message, data)
}

func (h *gameHandler) GetPrizeStructure(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, http.StatusNotImplemented, "Prize structure not yet implemented", nil)
}

func (h *gameHandler) UpdatePrizeStructure(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, http.StatusNotImplemented, "Prize structure update not yet implemented", nil)
}

// ScheduleGame schedules a single game
func (h *gameHandler) ScheduleGame(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	gameID := router.GetParam(r, "id")

	if gameID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Game ID is required", nil)
	}

	var reqBody struct {
		ScheduledStart string `json:"scheduled_start"`
		ScheduledEnd   string `json:"scheduled_end"`
		ScheduledDraw  string `json:"scheduled_draw"`
		Frequency      string `json:"frequency"`
		Notes          string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	// Parse timestamps
	scheduledStart, err := time.Parse(time.RFC3339, reqBody.ScheduledStart)
	if err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_TIME", "Invalid scheduled_start time format", err)
	}

	scheduledEnd, err := time.Parse(time.RFC3339, reqBody.ScheduledEnd)
	if err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_TIME", "Invalid scheduled_end time format", err)
	}

	scheduledDraw, err := time.Parse(time.RFC3339, reqBody.ScheduledDraw)
	if err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_TIME", "Invalid scheduled_draw time format", err)
	}

	req := &gamepb.ScheduleGameRequest{
		GameId:         gameID,
		ScheduledStart: timestamppb.New(scheduledStart),
		ScheduledEnd:   timestamppb.New(scheduledEnd),
		ScheduledDraw:  timestamppb.New(scheduledDraw),
		Frequency:      reqBody.Frequency,
		Notes:          reqBody.Notes,
	}

	resp, err := client.ScheduleGame(ctx, req)
	if err != nil {
		h.log.Error("Failed to schedule game", "error", err, "gameID", gameID)
		return response.Error(w, http.StatusInternalServerError, "SCHEDULE_FAILED", "Failed to schedule game", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusBadRequest, "SCHEDULE_FAILED", resp.Message, nil)
	}

	return response.Success(w, http.StatusOK, resp.Message, resp.Schedule)
}

// GetGameSchedule gets all schedules for a game
func (h *gameHandler) GetGameSchedule(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	gameID := router.GetParam(r, "id")

	if gameID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Game ID is required", nil)
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.GetGameScheduleRequest{
		GameId: gameID,
	}

	resp, err := client.GetGameSchedule(ctx, req)
	if err != nil {
		h.log.Error("Failed to get game schedule", "error", err, "gameID", gameID)
		return response.Error(w, http.StatusInternalServerError, "GET_FAILED", "Failed to get game schedule", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusBadRequest, "GET_FAILED", resp.Message, nil)
	}

	schedules := resp.Schedules
	if schedules == nil {
		schedules = []*gamepb.GameSchedule{}
	}

	// Transform protobuf schedules to proper JSON objects
	transformedSchedules := make([]map[string]interface{}, 0, len(schedules))
	for _, schedule := range schedules {
		scheduleMap := map[string]interface{}{
			"id":              schedule.Id,
			"game_id":         schedule.GameId,
			"game_code":       schedule.GameCode,
			"game_name":       schedule.GameName,
			"game_category":   schedule.GameCategory,
			"scheduled_start": nil,
			"scheduled_end":   nil,
			"scheduled_draw":  nil,
			"frequency":       schedule.Frequency,
			"is_active":       schedule.IsActive,
			"notes":           schedule.Notes,
			"status":          schedule.Status,
			"draw_result_id":  schedule.DrawResultId,
			"logo_url":        schedule.LogoUrl,
			"brand_color":     schedule.BrandColor,
		}

		// Handle timestamps properly
		if schedule.ScheduledStart != nil {
			scheduleMap["scheduled_start"] = schedule.ScheduledStart.AsTime()
		}
		if schedule.ScheduledEnd != nil {
			scheduleMap["scheduled_end"] = schedule.ScheduledEnd.AsTime()
		}
		if schedule.ScheduledDraw != nil {
			scheduleMap["scheduled_draw"] = schedule.ScheduledDraw.AsTime()
		}

		transformedSchedules = append(transformedSchedules, scheduleMap)
	}

	data := map[string]interface{}{
		"schedules": transformedSchedules,
	}

	return response.Success(w, http.StatusOK, resp.Message, data)
}

// UpdateScheduledGame updates a scheduled game
func (h *gameHandler) UpdateScheduledGame(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	scheduleID := router.GetParam(r, "id")

	if scheduleID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Schedule ID is required", nil)
	}

	var reqBody struct {
		ScheduledEnd  *string `json:"scheduled_end,omitempty"`
		ScheduledDraw *string `json:"scheduled_draw,omitempty"`
		Status        *string `json:"status,omitempty"`
		IsActive      *bool   `json:"is_active,omitempty"`
		Notes         *string `json:"notes,omitempty"`
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return response.Error(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body", err)
	}

	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	h.log.Debug("UpdateScheduledGame called",
		"scheduleID", scheduleID,
		"status", reqBody.Status)

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	// Build gRPC request
	grpcReq := &gamepb.UpdateScheduledGameRequest{
		ScheduleId: scheduleID,
	}

	if reqBody.ScheduledEnd != nil && *reqBody.ScheduledEnd != "" {
		scheduledEnd, err := time.Parse(time.RFC3339, *reqBody.ScheduledEnd)
		if err != nil {
			return response.Error(w, http.StatusBadRequest, "INVALID_TIME", "Invalid scheduled_end time format", err)
		}
		grpcReq.ScheduledEnd = timestamppb.New(scheduledEnd)
	}

	if reqBody.ScheduledDraw != nil && *reqBody.ScheduledDraw != "" {
		scheduledDraw, err := time.Parse(time.RFC3339, *reqBody.ScheduledDraw)
		if err != nil {
			return response.Error(w, http.StatusBadRequest, "INVALID_TIME", "Invalid scheduled_draw time format", err)
		}
		grpcReq.ScheduledDraw = timestamppb.New(scheduledDraw)
	}

	if reqBody.Status != nil {
		grpcReq.Status = *reqBody.Status
	}

	if reqBody.IsActive != nil {
		grpcReq.IsActive = *reqBody.IsActive
	}

	if reqBody.Notes != nil {
		grpcReq.Notes = *reqBody.Notes
	}

	// Call gRPC service
	resp, err := client.UpdateScheduledGame(ctx, grpcReq)
	if err != nil {
		h.log.Error("Failed to update scheduled game", "error", err, "scheduleID", scheduleID)
		return response.Error(w, http.StatusInternalServerError, "UPDATE_FAILED", "Failed to update scheduled game", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusBadRequest, "UPDATE_FAILED", resp.Message, nil)
	}

	// Transform schedule response
	schedule := resp.Schedule
	scheduleMap := map[string]interface{}{
		"id":              schedule.Id,
		"game_id":         schedule.GameId,
		"game_code":       schedule.GameCode,
		"game_name":       schedule.GameName,
		"game_category":   schedule.GameCategory,
		"scheduled_start": nil,
		"scheduled_end":   nil,
		"scheduled_draw":  nil,
		"frequency":       schedule.Frequency,
		"is_active":       schedule.IsActive,
		"notes":           schedule.Notes,
		"status":          schedule.Status,
		"draw_result_id":  schedule.DrawResultId,
	}

	if schedule.ScheduledStart != nil {
		scheduleMap["scheduled_start"] = schedule.ScheduledStart.AsTime()
	}
	if schedule.ScheduledEnd != nil {
		scheduleMap["scheduled_end"] = schedule.ScheduledEnd.AsTime()
	}
	if schedule.ScheduledDraw != nil {
		scheduleMap["scheduled_draw"] = schedule.ScheduledDraw.AsTime()
	}

	data := map[string]interface{}{
		"schedule": scheduleMap,
	}

	return response.Success(w, http.StatusOK, resp.Message, data)
}

func (h *gameHandler) UpdateGameStatus(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	gameID := router.GetParam(r, "id")

	if gameID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Game ID is required", nil)
	}

	var reqBody struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	// Validate status value
	if reqBody.Status == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_STATUS", "Status is required", nil)
	}

	h.log.Debug("UpdateGameStatus called",
		"gameID", gameID,
		"status", reqBody.Status)

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	// Handle different status values
	switch reqBody.Status {
	case "Active":
		req := &gamepb.ActivateGameRequest{
			Id: gameID,
		}

		game, err := client.ActivateGame(ctx, req)
		if err != nil {
			h.log.Error("Failed to activate game", "error", err, "gameID", gameID)
			return response.Error(w, http.StatusInternalServerError, "ACTIVATE_FAILED", "Failed to activate game", err)
		}

		if !game.Success {
			return response.Error(w, http.StatusBadRequest, "ACTIVATE_FAILED", game.Message, nil)
		}

		return response.Success(w, http.StatusOK, game.Message, game.Game)

	case "Suspended":
		req := &gamepb.SuspendGameRequest{
			Id:     gameID,
			Reason: "Suspended via admin interface",
		}

		game, err := client.SuspendGame(ctx, req)
		if err != nil {
			h.log.Error("Failed to suspend game", "error", err, "gameID", gameID)
			return response.Error(w, http.StatusInternalServerError, "SUSPEND_FAILED", "Failed to suspend game", err)
		}

		if !game.Success {
			return response.Error(w, http.StatusBadRequest, "SUSPEND_FAILED", game.Message, nil)
		}

		return response.Success(w, http.StatusOK, game.Message, game.Game)

	default:
		return response.Error(w, http.StatusBadRequest, "INVALID_STATUS", "Invalid status value. Supported values: Active, Suspended", nil)
	}
}

func (h *gameHandler) GetGameStatistics(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, http.StatusNotImplemented, "Game statistics not yet implemented", nil)
}

func (h *gameHandler) CloneGame(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, http.StatusNotImplemented, "Game cloning not yet implemented", nil)
}

func (h *gameHandler) GetGameRules(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, http.StatusNotImplemented, "Game rules not yet implemented", nil)
}

func (h *gameHandler) UpdateGameRules(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, http.StatusNotImplemented, "Game rules update not yet implemented", nil)
}

// GenerateWeeklySchedule generates schedules for all active games for a week
func (h *gameHandler) GenerateWeeklySchedule(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	h.log.Info("GenerateWeeklySchedule called")

	var reqBody struct {
		WeekStart string `json:"week_start"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		h.log.Error("Failed to decode request body", "error", err)
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	h.log.Info("Received request to generate weekly schedule", "week_start", reqBody.WeekStart)

	// Parse week start time
	weekStart, err := time.Parse("2006-01-02", reqBody.WeekStart)
	if err != nil {
		h.log.Error("Failed to parse week_start", "week_start", reqBody.WeekStart, "error", err)
		return response.Error(w, http.StatusBadRequest, "INVALID_DATE", "Invalid week_start date format (use YYYY-MM-DD)", err)
	}

	h.log.Info("Parsed week_start successfully", "week_start", weekStart.Format("2006-01-02"))

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.GenerateWeeklyScheduleRequest{
		WeekStart: timestamppb.New(weekStart),
	}

	h.log.Info("Calling game service GenerateWeeklySchedule", "week_start", weekStart.Format("2006-01-02"))

	resp, err := client.GenerateWeeklySchedule(ctx, req)
	if err != nil {
		h.log.Error("Failed to generate weekly schedule", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SCHEDULE_FAILED", "Failed to generate weekly schedule", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusBadRequest, "SCHEDULE_FAILED", resp.Message, nil)
	}

	schedules := resp.Schedules
	if schedules == nil {
		schedules = []*gamepb.GameSchedule{}
	}

	data := map[string]interface{}{
		"schedules":         schedules,
		"schedules_created": resp.SchedulesCreated,
	}

	return response.Success(w, http.StatusOK, resp.Message, data)
}

// GetWeeklySchedule gets all schedules for a specific week
func (h *gameHandler) GetWeeklySchedule(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Get week_start from query parameters
	weekStartStr := r.URL.Query().Get("week_start")
	if weekStartStr == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_WEEK_START", "week_start query parameter is required (YYYY-MM-DD format)", nil)
	}

	// Parse week start time
	weekStart, err := time.Parse("2006-01-02", weekStartStr)
	if err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_DATE", "Invalid week_start date format (use YYYY-MM-DD)", err)
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.GetWeeklyScheduleRequest{
		WeekStart: timestamppb.New(weekStart),
	}

	resp, err := client.GetWeeklySchedule(ctx, req)
	if err != nil {
		h.log.Error("Failed to get weekly schedule", "error", err)
		return response.Error(w, http.StatusInternalServerError, "GET_FAILED", "Failed to get weekly schedule", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusBadRequest, "GET_FAILED", resp.Message, nil)
	}

	schedules := resp.Schedules
	if schedules == nil {
		schedules = []*gamepb.GameSchedule{}
	}

	// Transform protobuf schedules to proper JSON objects
	transformedSchedules := make([]map[string]interface{}, 0, len(schedules))
	for _, schedule := range schedules {
		scheduleMap := map[string]interface{}{
			"id":              schedule.Id,
			"game_id":         schedule.GameId,
			"game_code":       schedule.GameCode,
			"game_name":       schedule.GameName,
			"game_category":   schedule.GameCategory,
			"scheduled_start": nil,
			"scheduled_end":   nil,
			"scheduled_draw":  nil,
			"frequency":       schedule.Frequency,
			"is_active":       schedule.IsActive,
			"notes":           schedule.Notes,
			"status":          schedule.Status,
			"draw_result_id":  schedule.DrawResultId,
			"logo_url":        schedule.LogoUrl,
			"brand_color":     schedule.BrandColor,
		}

		// Handle timestamps properly
		if schedule.ScheduledStart != nil {
			scheduleMap["scheduled_start"] = schedule.ScheduledStart.AsTime()
		}
		if schedule.ScheduledEnd != nil {
			scheduleMap["scheduled_end"] = schedule.ScheduledEnd.AsTime()
		}
		if schedule.ScheduledDraw != nil {
			scheduleMap["scheduled_draw"] = schedule.ScheduledDraw.AsTime()
		}

		transformedSchedules = append(transformedSchedules, scheduleMap)
	}

	data := map[string]interface{}{
		"schedules": transformedSchedules,
	}

	return response.Success(w, http.StatusOK, resp.Message, data)
}

// ClearWeeklySchedule clears all schedules for a specific week
func (h *gameHandler) ClearWeeklySchedule(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var reqBody struct {
		WeekStart string `json:"week_start"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	// Parse week start time
	weekStart, err := time.Parse("2006-01-02", reqBody.WeekStart)
	if err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_DATE", "Invalid week_start date format (use YYYY-MM-DD)", err)
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.ClearWeeklyScheduleRequest{
		WeekStart: timestamppb.New(weekStart),
	}

	resp, err := client.ClearWeeklySchedule(ctx, req)
	if err != nil {
		h.log.Error("Failed to clear weekly schedule", "error", err)
		return response.Error(w, http.StatusInternalServerError, "CLEAR_FAILED", "Failed to clear weekly schedule", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusBadRequest, "CLEAR_FAILED", resp.Message, nil)
	}

	data := map[string]interface{}{
		"schedules_deleted": resp.SchedulesDeleted,
	}

	return response.Success(w, http.StatusOK, resp.Message, data)
}

// GetScheduleByID gets a specific schedule by ID
func (h *gameHandler) GetScheduleByID(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	scheduleID := router.GetParam(r, "scheduleId")

	if scheduleID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Schedule ID is required", nil)
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.GetScheduleByIdRequest{
		ScheduleId: scheduleID,
	}

	resp, err := client.GetScheduleById(ctx, req)
	if err != nil {
		h.log.Error("Failed to get schedule", "error", err, "scheduleID", scheduleID)
		return response.Error(w, http.StatusInternalServerError, "GET_FAILED", "Failed to get schedule", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusNotFound, "NOT_FOUND", resp.Message, nil)
	}

	// Transform the schedule to proper JSON object
	scheduleMap := map[string]interface{}{
		"id":              resp.Schedule.Id,
		"game_id":         resp.Schedule.GameId,
		"game_code":       resp.Schedule.GameCode,
		"game_name":       resp.Schedule.GameName,
		"game_category":   resp.Schedule.GameCategory,
		"scheduled_start": nil,
		"scheduled_end":   nil,
		"scheduled_draw":  nil,
		"frequency":       resp.Schedule.Frequency,
		"is_active":       resp.Schedule.IsActive,
		"notes":           resp.Schedule.Notes,
		"status":          resp.Schedule.Status,
		"draw_result_id":  resp.Schedule.DrawResultId,
	}

	// Handle timestamps properly
	if resp.Schedule.ScheduledStart != nil {
		scheduleMap["scheduled_start"] = resp.Schedule.ScheduledStart.AsTime()
	}
	if resp.Schedule.ScheduledEnd != nil {
		scheduleMap["scheduled_end"] = resp.Schedule.ScheduledEnd.AsTime()
	}
	if resp.Schedule.ScheduledDraw != nil {
		scheduleMap["scheduled_draw"] = resp.Schedule.ScheduledDraw.AsTime()
	}

	data := map[string]interface{}{
		"schedule": scheduleMap,
	}

	return response.Success(w, http.StatusOK, "Schedule retrieved successfully", data)
}

// GetActiveGames retrieves all active games (for retailers/public)
func (h *gameHandler) GetActiveGames(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Only fetch active games for retailers
	req := &gamepb.ListGamesRequest{
		Page:         1,
		PerPage:      100, // Get all active games
		StatusFilter: "Active",
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	resp, err := client.ListGames(ctx, req)
	if err != nil {
		h.log.Error("Failed to list active games", "error", err)
		return response.Error(w, http.StatusInternalServerError, "LIST_FAILED", "Failed to list games", err)
	}

	// Format the response with games array
	games := resp.Games
	if games == nil {
		games = []*gamepb.Game{} // Return empty array if nil
	}

	// Filter out sensitive admin data - only return what retailers need
	publicGames := make([]map[string]interface{}, 0, len(games))
	for _, game := range games {
		publicGame := map[string]interface{}{
			"id":                     game.Id,
			"code":                   game.Code,
			"name":                   game.Name,
			"organizer":              game.Organizer,
			"game_category":          game.GameCategory,
			"game_format":            game.GameFormat,
			"number_range_min":       game.NumberRangeMin,
			"number_range_max":       game.NumberRangeMax,
			"selection_count":        game.SelectionCount,
			"draw_frequency":         game.DrawFrequency,
			"draw_days":              game.DrawDays,
			"draw_time":              game.DrawTime,
			"sales_cutoff_minutes":   game.SalesCutoffMinutes,
			"min_stake":              game.MinStake,
			"max_stake":              game.MaxStake,
			"base_price":             game.BasePrice,
			"max_tickets_per_player": game.MaxTicketsPerPlayer,
			"multi_draw_enabled":     game.MultiDrawEnabled,
			"max_draws_advance":      game.MaxDrawsAdvance,
			"status":                 game.Status,
			"description":            game.Description,
		}
		publicGames = append(publicGames, publicGame)
	}

	data := map[string]interface{}{
		"games": publicGames,
		"total": resp.Total,
	}

	return response.Success(w, http.StatusOK, "Active games retrieved successfully", data)
}

// GetScheduledGamesForRetailer gets all scheduled games for the current week (for retailers)
func (h *gameHandler) GetScheduledGamesForRetailer(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Get week_start from query parameters, default to current week
	weekStartStr := r.URL.Query().Get("week_start")
	var weekStart time.Time
	var err error

	if weekStartStr == "" {
		// Default to start of current week (Sunday) in GMT timezone (Ghana is GMT/UTC+0)
		// Use NTP time to ensure accurate time regardless of server clock issues
		gmtLocation := time.FixedZone("GMT", 0)
		now := h.ntpTime.Now().In(gmtLocation)
		weekday := now.Weekday()
		// Calculate days to subtract to get to Sunday
		daysToSunday := int(weekday)
		weekStart = now.AddDate(0, 0, -daysToSunday)
		// Set to midnight in GMT
		weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, gmtLocation)

		h.log.Debug("Week start calculation for retailer (using NTP time)",
			"now_gmt", now.Format(time.RFC3339),
			"weekday", weekday,
			"weekday_name", weekday.String(),
			"days_to_subtract", daysToSunday,
			"calculated_week_start", weekStart.Format("2006-01-02"),
			"ntp_offset", h.ntpTime.GetOffset(),
			"ntp_last_sync", h.ntpTime.LastSyncTime(),
		)
	} else {
		// Parse provided week start
		weekStart, err = time.Parse("2006-01-02", weekStartStr)
		if err != nil {
			return response.Error(w, http.StatusBadRequest, "INVALID_DATE", "Invalid week_start date format (use YYYY-MM-DD)", err)
		}
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.GetWeeklyScheduleRequest{
		WeekStart: timestamppb.New(weekStart),
	}

	resp, err := client.GetWeeklySchedule(ctx, req)
	if err != nil {
		h.log.Error("Failed to get weekly schedule for retailer", "error", err)
		return response.Error(w, http.StatusInternalServerError, "GET_FAILED", "Failed to get scheduled games", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusBadRequest, "GET_FAILED", resp.Message, nil)
	}

	schedules := resp.Schedules
	if schedules == nil {
		schedules = []*gamepb.GameSchedule{}
	}

	// Transform and filter schedules - only return active schedules
	retailerSchedules := make([]map[string]interface{}, 0)
	for _, schedule := range schedules {
		// Only return active schedules with status "scheduled", "active" or "pending"
		// Skip completed or cancelled schedules
		if !schedule.IsActive {
			continue
		}

		status := strings.ToLower(schedule.Status)
		if status != "scheduled" && status != "active" && status != "pending" {
			continue
		}

		scheduleMap := map[string]interface{}{
			"id":              schedule.Id,
			"game_id":         schedule.GameId,
			"game_code":       schedule.GameCode,
			"game_name":       schedule.GameName,
			"game_category":   schedule.GameCategory,
			"scheduled_start": nil,
			"scheduled_end":   nil,
			"scheduled_draw":  nil,
			"frequency":       schedule.Frequency,
			"status":          schedule.Status,
			"logo_url":        schedule.LogoUrl,
			"brand_color":     schedule.BrandColor,
		}

		// Handle timestamps properly
		if schedule.ScheduledStart != nil {
			scheduleMap["scheduled_start"] = schedule.ScheduledStart.AsTime()
		}
		if schedule.ScheduledEnd != nil {
			scheduleMap["scheduled_end"] = schedule.ScheduledEnd.AsTime()
		}
		if schedule.ScheduledDraw != nil {
			scheduleMap["scheduled_draw"] = schedule.ScheduledDraw.AsTime()
		}

		if len(schedule.BetTypes) > 0 {
			bt := make([]map[string]any, 0, len(schedule.BetTypes))
			for _, b := range schedule.BetTypes {
				bt = append(bt, map[string]any{
					"id":         b.Id,
					"name":       b.Name,
					"multiplier": b.Multiplier,
					"enabled":    b.Enabled,
				})
			}
			scheduleMap["bet_types"] = bt
		}

		retailerSchedules = append(retailerSchedules, scheduleMap)
	}

	// Include server time so clients can't manipulate device time to play expired games
	// Use NTP time and GMT timezone (Ghana is in GMT/UTC+0)
	gmtLocation := time.FixedZone("GMT", 0)
	serverTime := h.ntpTime.Now().In(gmtLocation)

	data := map[string]interface{}{
		"schedules":   retailerSchedules,
		"week_start":  weekStart.Format("2006-01-02"),
		"server_time": serverTime, // ISO 8601 format for Android parsing (Ghana timezone)
	}

	return response.Success(w, http.StatusOK, "Scheduled games retrieved successfully", data)
}

// GetScheduledGamesForPlayer gets all scheduled games for the current week (for players)
func (h *gameHandler) GetScheduledGamesForPlayer(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	weekStartStr := r.URL.Query().Get("week_start")
	var weekStart time.Time
	var err error

	if weekStartStr == "" {
		// Default to start of current week (Sunday) in GMT timezone (Ghana is GMT/UTC+0)
		// Use NTP time to ensure accurate time regardless of server clock issues
		gmtLocation := time.FixedZone("GMT", 0)
		now := h.ntpTime.Now().In(gmtLocation)
		weekday := now.Weekday()
		// Calculate days to subtract to get to Sunday
		daysToSunday := int(weekday)
		weekStart = now.AddDate(0, 0, -daysToSunday)
		// Set to midnight in GMT
		weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, gmtLocation)

		h.log.Debug("Week start calculation for player (using NTP time)",
			"now_gmt", now.Format(time.RFC3339),
			"weekday", weekday,
			"weekday_name", weekday.String(),
			"days_to_subtract", daysToSunday,
			"calculated_week_start", weekStart.Format("2006-01-02"),
			"ntp_offset", h.ntpTime.GetOffset(),
			"ntp_last_sync", h.ntpTime.LastSyncTime(),
		)
	} else {
		weekStart, err = time.Parse("2006-01-02", weekStartStr)
		if err != nil {
			return response.Error(w, http.StatusBadRequest, "INVALID_DATE", "Invalid week_start date format (use YYYY-MM-DD)", err)
		}
	}

	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	req := &gamepb.GetWeeklyScheduleRequest{
		WeekStart: timestamppb.New(weekStart),
	}

	resp, err := client.GetWeeklySchedule(ctx, req)
	if err != nil {
		h.log.Error("Failed to get weekly schedule for player", "error", err)
		return response.Error(w, http.StatusInternalServerError, "GET_FAILED", "Failed to get scheduled games", err)
	}

	if !resp.Success {
		return response.Error(w, http.StatusBadRequest, "GET_FAILED", resp.Message, nil)
	}

	schedules := resp.Schedules
	if schedules == nil {
		schedules = []*gamepb.GameSchedule{}
	}

	playerSchedules := make([]map[string]any, 0)
	for _, schedule := range schedules {
		// Only return active schedules with status "scheduled" or "in_progress"
		if !schedule.IsActive {
			continue
		}

		status := strings.ToLower(schedule.Status)
		if status != "scheduled" && status != "in_progress" {
			continue
		}

		scheduleMap := map[string]any{
			"id":              schedule.Id,
			"game_id":         schedule.GameId,
			"game_code":       schedule.GameCode,
			"game_name":       schedule.GameName,
			"game_category":   schedule.GameCategory,
			"scheduled_start": nil,
			"scheduled_end":   nil,
			"scheduled_draw":  nil,
			"frequency":       schedule.Frequency,
			"status":          schedule.Status,
			"logo_url":        schedule.LogoUrl,
			"brand_color":     schedule.BrandColor,
		}

		// Handle timestamps properly
		if schedule.ScheduledStart != nil {
			scheduleMap["scheduled_start"] = schedule.ScheduledStart.AsTime()
		}
		if schedule.ScheduledEnd != nil {
			scheduleMap["scheduled_end"] = schedule.ScheduledEnd.AsTime()
		}
		if schedule.ScheduledDraw != nil {
			scheduleMap["scheduled_draw"] = schedule.ScheduledDraw.AsTime()
		}

		if len(schedule.BetTypes) > 0 {
			bt := make([]map[string]any, 0, len(schedule.BetTypes))
			for _, b := range schedule.BetTypes {
				bt = append(bt, map[string]any{
					"id":         b.Id,
					"name":       b.Name,
					"multiplier": b.Multiplier,
					"enabled":    b.Enabled,
				})
			}
			scheduleMap["bet_types"] = bt
		}

		playerSchedules = append(playerSchedules, scheduleMap)
	}

	// Use NTP time and GMT timezone (Ghana is in GMT/UTC+0)
	gmtLocation := time.FixedZone("GMT", 0)
	serverTime := h.ntpTime.Now().In(gmtLocation)

	data := map[string]any{
		"schedules":   playerSchedules,
		"week_start":  weekStart.Format("2006-01-02"),
		"server_time": serverTime,
	}

	return response.Success(w, http.StatusOK, "Scheduled games retrieved successfully", data)
}

// UploadGameLogo handles logo upload for a game
func (h *gameHandler) UploadGameLogo(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Get game ID from URL
	gameID := router.GetParam(r, "id")
	if gameID == "" {
		return response.Error(w, http.StatusBadRequest, "INVALID_GAME_ID", "Game ID is required", nil)
	}

	h.log.Info("Uploading logo for game", "game_id", gameID)

	// Parse multipart form (max 10MB for the entire request)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		h.log.Error("Failed to parse multipart form", "error", err)
		return response.Error(w, http.StatusBadRequest, "INVALID_FORM", "Invalid multipart form", err)
	}

	// Get file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		h.log.Error("Failed to get file from form", "error", err)
		return response.Error(w, http.StatusBadRequest, "FILE_REQUIRED", "Logo file is required", err)
	}
	defer func() { _ = file.Close() }()

	// Validate file size (5MB max)
	if header.Size > 5<<20 {
		return response.Error(w, http.StatusBadRequest, "FILE_TOO_LARGE", "File size must not exceed 5MB", nil)
	}

	// Get optional brand color from form
	brandColor := r.FormValue("brand_color")

	// Check if storage is configured
	if h.storage == nil {
		h.log.Error("Storage not configured", "game_id", gameID)
		return response.Error(w, http.StatusInternalServerError, "STORAGE_NOT_CONFIGURED", "Storage not configured", nil)
	}

	// Read file data FIRST before any processing
	fileData, err := io.ReadAll(file)
	if err != nil {
		h.log.Error("Failed to read file data", "error", err)
		return response.Error(w, http.StatusInternalServerError, "FILE_READ_ERROR", "Failed to read file", err)
	}

	// Detect content type from magic bytes
	contentType := header.Header.Get("Content-Type")
	detectedType := detectImageType(fileData)

	// Use detected type as fallback if header is missing or empty
	if contentType == "" && detectedType != "" {
		contentType = detectedType
		h.log.Info("Content type detected from magic bytes", "detected", detectedType)
	}

	// Validate content type
	allowedTypes := map[string]bool{
		"image/png":  true,
		"image/jpeg": true,
		"image/jpg":  true,
		"image/webp": true,
	}
	if !allowedTypes[contentType] {
		h.log.Warn("Invalid file type", "content_type", contentType, "detected_type", detectedType)
		return response.Error(w, http.StatusBadRequest, "INVALID_FILE_TYPE", "Only PNG, JPEG, JPG, and WebP images are allowed", nil)
	}

	// Derive file extension from the VALIDATED content type
	var ext string
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	case "image/webp":
		ext = ".webp"
	default:
		ext = ".png" // Fallback (should never reach here due to validation)
	}

	// Create standardized filename: logo.{ext}
	standardFileName := "logo" + ext

	// Upload to storage
	uploadInfo := storage.UploadInfo{
		GameID:      gameID,
		FileName:    standardFileName,
		ContentType: contentType,
		Size:        header.Size,
		Data:        fileData,
		Permission:  "public", // Logos are public
	}

	objectInfo, err := h.storage.Upload(ctx, uploadInfo)
	if err != nil {
		h.log.Error("Failed to upload to storage", "error", err, "game_id", gameID)
		return response.Error(w, http.StatusInternalServerError, "UPLOAD_FAILED", "Failed to upload file to storage", err)
	}

	// Use CDN URL if available, otherwise use regular URL
	logoURL := objectInfo.CDNURL
	if logoURL == "" {
		logoURL = objectInfo.URL
	}

	h.log.Info("File uploaded to storage", "game_id", gameID, "logo_url", logoURL)

	// Get game service client
	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		// Try to clean up uploaded file
		_ = h.storage.Delete(ctx, gameID)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	// Update game with logo URL in database
	uploadResp, err := client.UpdateGameLogo(ctx, &gamepb.UpdateGameLogoRequest{
		GameId:     gameID,
		LogoUrl:    logoURL,
		BrandColor: brandColor,
	})
	if err != nil {
		h.log.Error("Failed to update game logo in database", "error", err, "game_id", gameID)
		// Try to clean up uploaded file
		_ = h.storage.Delete(ctx, gameID)
		return response.Error(w, http.StatusInternalServerError, "UPLOAD_FAILED", "Failed to update game logo", err)
	}

	if !uploadResp.Success {
		// Try to clean up uploaded file
		_ = h.storage.Delete(ctx, gameID)
		return response.Error(w, http.StatusBadRequest, "UPLOAD_FAILED", uploadResp.Message, nil)
	}

	h.log.Info("Logo uploaded successfully", "game_id", gameID, "logo_url", uploadResp.LogoUrl)

	data := map[string]any{
		"logo_url":    uploadResp.LogoUrl,
		"cdn_url":     uploadResp.CdnUrl,
		"brand_color": uploadResp.BrandColor,
	}

	return response.Success(w, http.StatusOK, "Game logo uploaded successfully", data)
}

// DeleteGameLogo handles logo deletion for a game
func (h *gameHandler) DeleteGameLogo(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Get game ID from URL
	gameID := router.GetParam(r, "id")
	if gameID == "" {
		return response.Error(w, http.StatusBadRequest, "INVALID_GAME_ID", "Game ID is required", nil)
	}

	h.log.Info("Deleting logo for game", "game_id", gameID)

	// Get game service client
	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	// Call game service to delete logo
	deleteResp, err := client.DeleteGameLogo(ctx, &gamepb.DeleteGameLogoRequest{
		GameId: gameID,
	})
	if err != nil {
		h.log.Error("Failed to delete game logo", "error", err, "game_id", gameID)
		return response.Error(w, http.StatusInternalServerError, "DELETE_FAILED", "Failed to delete logo", err)
	}

	if !deleteResp.Success {
		return response.Error(w, http.StatusBadRequest, "DELETE_FAILED", deleteResp.Message, nil)
	}

	h.log.Info("Logo deleted successfully", "game_id", gameID)

	return response.Success(w, http.StatusOK, "Game logo deleted successfully", nil)
}

// UpdateBrandColor updates only the brand color of a game
func (h *gameHandler) UpdateBrandColor(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Get game ID from URL
	gameID := router.GetParam(r, "id")
	if gameID == "" {
		return response.Error(w, http.StatusBadRequest, "INVALID_GAME_ID", "Game ID is required", nil)
	}

	h.log.Info("Updating brand color for game", "game_id", gameID)

	// Parse JSON request body
	var reqBody struct {
		BrandColor string `json:"brand_color"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	// Validate brand color is provided
	if reqBody.BrandColor == "" {
		return response.Error(w, http.StatusBadRequest, "BRAND_COLOR_REQUIRED", "Brand color is required", nil)
	}

	// Get game service client
	client, err := h.grpcManager.GameServiceClient()
	if err != nil {
		h.log.Error("Failed to get game service client", "error", err)
		return response.Error(w, http.StatusInternalServerError, "SERVICE_UNAVAILABLE", "Service unavailable", nil)
	}

	// First, get the current game to preserve the existing logo URL
	getGameReq := &gamepb.GetGameRequest{Id: gameID}
	game, err := client.GetGame(ctx, getGameReq)
	if err != nil {
		h.log.Error("Failed to get game", "error", err, "game_id", gameID)
		return response.Error(w, http.StatusInternalServerError, "GET_FAILED", "Failed to get game", err)
	}

	// Update game brand color using the existing UpdateGameLogo gRPC method
	// Pass the existing logo URL to preserve it
	updateResp, err := client.UpdateGameLogo(ctx, &gamepb.UpdateGameLogoRequest{
		GameId:     gameID,
		LogoUrl:    game.Game.LogoUrl, // Preserve existing logo
		BrandColor: reqBody.BrandColor,
	})
	if err != nil {
		h.log.Error("Failed to update game brand color", "error", err, "game_id", gameID)
		return response.Error(w, http.StatusInternalServerError, "UPDATE_FAILED", "Failed to update brand color", err)
	}

	if !updateResp.Success {
		return response.Error(w, http.StatusBadRequest, "UPDATE_FAILED", updateResp.Message, nil)
	}

	h.log.Info("Brand color updated successfully", "game_id", gameID, "brand_color", reqBody.BrandColor)

	data := map[string]any{
		"brand_color": updateResp.BrandColor,
	}

	return response.Success(w, http.StatusOK, "Game brand color updated successfully", data)
}

// detectImageType detects image MIME type from magic bytes (file signature)
// Returns empty string if the file is not a recognized image type
func detectImageType(data []byte) string {
	if len(data) < 12 {
		return "" // Not enough data to detect
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A {
		return "image/png"
	}

	// JPEG: FF D8 FF
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}

	// WebP: RIFF .... WEBP
	if len(data) >= 12 &&
		data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return "image/webp"
	}

	return "" // Unknown type
}

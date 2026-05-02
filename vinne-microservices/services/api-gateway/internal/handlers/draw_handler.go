package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	drawv1 "github.com/randco/randco-microservices/proto/draw/v1"
	gamepb "github.com/randco/randco-microservices/proto/game/v1"
	ticketv1 "github.com/randco/randco-microservices/proto/ticket/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/response"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// drawHandler implements the DrawHandler interface
type drawHandler struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
}

// NewDrawHandler creates a new draw handler
func NewDrawHandler(grpcManager *grpc.ClientManager, log logger.Logger) DrawHandler {
	return &drawHandler{
		grpcManager: grpcManager,
		log:         log,
	}
}

// GetDraw retrieves a single draw by ID
func (h *drawHandler) GetDraw(w http.ResponseWriter, r *http.Request) error {
	drawID := router.GetParam(r, "id")
	if drawID == "" {
		return response.ValidationError(w, "draw_id is required", nil)
	}

	// Get Draw Service connection
	conn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	// Create Draw Service client
	client := drawv1.NewDrawServiceClient(conn)

	// Call GetDraw
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.GetDraw(ctx, &drawv1.GetDrawRequest{
		Id: drawID,
	})
	if err != nil {
		h.log.Error("Failed to get draw", "error", err, "draw_id", drawID)
		return response.InternalError(w, "Failed to get draw")
	}

	if !resp.Success {
		return response.NotFoundError(w, "Draw")
	}

	// DEBUG: Log stage data
	if resp.Draw != nil && resp.Draw.Stage != nil {
		h.log.Info("DEBUG: Draw has stage data",
			"draw_id", resp.Draw.Id,
			"current_stage", resp.Draw.Stage.CurrentStage,
			"stage_status", resp.Draw.Stage.StageStatus,
		)
	} else {
		h.log.Info("DEBUG: Draw stage is nil", "draw_id", drawID)
	}

	return response.Success(w, http.StatusOK, resp.Message, resp.Draw)
}

// ListDraws retrieves a list of draws with optional filters
func (h *drawHandler) ListDraws(w http.ResponseWriter, r *http.Request) error {
	// Parse query parameters
	gameID := r.URL.Query().Get("game_id")
	statusParam := r.URL.Query().Get("status")
	startDateParam := r.URL.Query().Get("start_date")
	endDateParam := r.URL.Query().Get("end_date")
	pageParam := r.URL.Query().Get("page")
	perPageParam := r.URL.Query().Get("per_page")

	// Set default pagination
	page := int32(1)
	perPage := int32(20)

	if pageParam != "" {
		if p, err := strconv.ParseInt(pageParam, 10, 32); err == nil {
			page = int32(p)
		}
	}
	if perPageParam != "" {
		if pp, err := strconv.ParseInt(perPageParam, 10, 32); err == nil {
			perPage = int32(pp)
		}
	}

	// Build request
	req := &drawv1.ListDrawsRequest{
		GameId:  gameID,
		Page:    page,
		PerPage: perPage,
	}

	// Parse status filter
	if statusParam != "" {
		status := parseDrawStatus(statusParam)
		if status != drawv1.DrawStatus_DRAW_STATUS_UNSPECIFIED {
			req.StatusFilter = status
		}
	}

	// Parse date filters
	if startDateParam != "" {
		if t, err := time.Parse(time.RFC3339, startDateParam); err == nil {
			req.StartDate = timestamppb.New(t)
		}
	}
	if endDateParam != "" {
		if t, err := time.Parse(time.RFC3339, endDateParam); err == nil {
			req.EndDate = timestamppb.New(t)
		}
	}

	// Get Draw Service connection
	conn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	// Create Draw Service client
	client := drawv1.NewDrawServiceClient(conn)

	// Call ListDraws
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.ListDraws(ctx, req)
	if err != nil {
		h.log.Error("Failed to list draws", "error", err)
		return response.InternalError(w, "Failed to list draws")
	}

	if !resp.Success {
		return response.InternalError(w, resp.Message)
	}

	// Transform draws to include total_stakes field for frontend compatibility
	transformedDraws := make([]map[string]interface{}, len(resp.Draws))
	for i, draw := range resp.Draws {
		drawMap := map[string]interface{}{
			"id":                     draw.Id,
			"game_id":                draw.GameId,
			"draw_number":            draw.DrawNumber,
			"game_name":              draw.GameName,
			"draw_name":              draw.DrawName,
			"status":                 draw.Status,
			"scheduled_time":         draw.ScheduledTime,
			"executed_time":          draw.ExecutedTime,
			"winning_numbers":        draw.WinningNumbers,
			"draw_location":          draw.DrawLocation,
			"nla_draw_reference":     draw.NlaDrawReference,
			"nla_official_signature": draw.NlaOfficialSignature,
			"total_tickets_sold":     draw.TotalTicketsSold,
			"total_stakes":           draw.TotalPrizePool,
			"total_prize_pool":       draw.TotalPrizePool,
			"verification_hash":      draw.VerificationHash,
			"stage":                  draw.Stage,
			"created_at":             draw.CreatedAt,
			"updated_at":             draw.UpdatedAt,
		}
		transformedDraws[i] = drawMap
	}

	// Fetch real paid ticket counts from the ticket service concurrently.
	// total_tickets_sold on the draw record is stale for USSD purchases (Flask writes
	// directly to the ticket DB, bypassing the draw service counter).
	ticketClient, ticketConnErr := h.grpcManager.TicketServiceClient()
	if ticketConnErr == nil {
		var wg sync.WaitGroup
		const nilUUID = "00000000-0000-0000-0000-000000000000"
		for i, draw := range resp.Draws {
			wg.Add(1)
			go func(idx int, d *drawv1.Draw) {
				defer wg.Done()
				countCtx, countCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer countCancel()
				filter := &ticketv1.TicketFilter{PaymentStatus: "completed"}
				if d.GameScheduleId != "" && d.GameScheduleId != nilUUID {
					filter.GameScheduleId = d.GameScheduleId
				} else {
					filter.DrawId = d.Id
				}
				countResp, err := ticketClient.ListTickets(countCtx, &ticketv1.ListTicketsRequest{
					Filter:   filter,
					Page:     1,
					PageSize: 1,
				})
				if err == nil {
					transformedDraws[idx]["total_tickets_sold"] = countResp.Total
				}
			}(i, draw)
		}
		wg.Wait()
	}

	// Build response with pagination metadata
	result := map[string]interface{}{
		"draws":       transformedDraws,
		"total_count": resp.TotalCount,
		"page":        resp.Page,
		"per_page":    resp.PerPage,
		"total_pages": (resp.TotalCount + int64(resp.PerPage) - 1) / int64(resp.PerPage),
	}

	return response.Success(w, http.StatusOK, resp.Message, result)
}

// BulkUploadTickets creates tickets from an uploaded list and sends SMS to each recipient.
// POST /api/v1/admin/draws/{id}/tickets/bulk-upload
// Body: {"entries": [{"phone":"233241234567","name":"John Doe","quantity":2}, ...]}
func (h *drawHandler) BulkUploadTickets(w http.ResponseWriter, r *http.Request) error {
	drawID := router.GetParam(r, "id")
	if drawID == "" {
		return response.ValidationError(w, "draw_id is required", nil)
	}

	// Parse request body
	var req struct {
		Entries []struct {
			Phone    string `json:"phone"`
			Name     string `json:"name"`
			Quantity int    `json:"quantity"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return response.ValidationError(w, "invalid request body", nil)
	}
	if len(req.Entries) == 0 {
		return response.ValidationError(w, "entries list is empty", nil)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	// ── 1. Fetch the draw ────────────────────────────────────────────────────
	drawConn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		return response.ServiceUnavailableError(w, "Draw")
	}
	drawClient := drawv1.NewDrawServiceClient(drawConn)
	drawResp, err := drawClient.GetDraw(ctx, &drawv1.GetDrawRequest{Id: drawID})
	if err != nil || !drawResp.Success {
		return response.NotFoundError(w, "Draw")
	}
	draw := drawResp.Draw

	// ── 2. Resolve game_code via the game schedule ───────────────────────────
	gameCode := draw.GameCode
	const nilUUID = "00000000-0000-0000-0000-000000000000"
	if gameCode == "" && draw.GameScheduleId != "" && draw.GameScheduleId != nilUUID {
		gameClient, gcErr := h.grpcManager.GameServiceClient()
		if gcErr == nil {
			schedResp, schedErr := gameClient.GetScheduleById(ctx, &gamepb.GetScheduleByIdRequest{
				ScheduleId: draw.GameScheduleId,
			})
			if schedErr == nil && schedResp.Schedule != nil {
				gameCode = schedResp.Schedule.GetGameCode()
			}
		}
	}
	// If schedule lookup didn't return a code, look up an existing ticket for this schedule
	// to get the authoritative game_code stored in the ticket DB
	if gameCode == "" {
		ticketClientForLookup, tcErr := h.grpcManager.TicketServiceClient()
		if tcErr == nil {
			existingResp, existingErr := ticketClientForLookup.ListTickets(ctx, &ticketv1.ListTicketsRequest{
				Filter:   &ticketv1.TicketFilter{GameScheduleId: draw.GameScheduleId},
				Page:     1,
				PageSize: 1,
			})
			if existingErr == nil && len(existingResp.Tickets) > 0 {
				gameCode = existingResp.Tickets[0].GameCode
			}
		}
	}
	if gameCode == "" {
		return response.ValidationError(w, "could not resolve game code for this draw", nil)
	}

	// ── 3. Ticket service client ─────────────────────────────────────────────
	ticketClient, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Ticket")
	}

	// ── 4. Create tickets concurrently (max 8 goroutines) ───────────────────
	type entryResult struct {
		Phone    string   `json:"phone"`
		Name     string   `json:"name"`
		Quantity int      `json:"quantity"`
		Tickets  []string `json:"tickets"`
		SMSSent  bool     `json:"sms_sent"`
		Error    string   `json:"error,omitempty"`
	}

	results := make([]entryResult, len(req.Entries))
	sem := make(chan struct{}, 8)
	var mu sync.Mutex
	totalCreated := 0

	var wg sync.WaitGroup
	for i, entry := range req.Entries {
		if entry.Phone == "" {
			results[i] = entryResult{Phone: entry.Phone, Error: "phone is required"}
			continue
		}
		qty := entry.Quantity
		if qty <= 0 {
			qty = 1
		}

		wg.Add(1)
		go func(idx int, phone, name string, quantity int) {
			defer wg.Done()
			var serials []string
			var issueErr string

			for q := 0; q < quantity; q++ {
				sem <- struct{}{}
				issueResp, err := ticketClient.IssueTicket(ctx, &ticketv1.IssueTicketRequest{
					GameCode:       gameCode,
					GameScheduleId: draw.GameScheduleId,
					DrawNumber:     draw.DrawNumber,
					BetLines: []*ticketv1.BetLine{
						{LineNumber: 1, BetType: "RAFFLE", TotalAmount: 2000},
					},
					IssuerType:    "ADMIN",
					IssuerId:      "admin-bulk:" + name,
					CustomerPhone: phone,
					PaymentMethod: "external",
				})
				<-sem

				if err != nil {
					issueErr = err.Error()
					break
				}
				if issueResp.Ticket != nil {
					serial := issueResp.Ticket.SerialNumber
					if serial == "" {
						serial = issueResp.Ticket.Id
					}
					serials = append(serials, serial)
				}
			}

			mu.Lock()
			totalCreated += len(serials)
			results[idx] = entryResult{
				Phone:    phone,
				Name:     name,
				Quantity: quantity,
				Tickets:  serials,
				Error:    issueErr,
			}
			mu.Unlock()
		}(i, entry.Phone, entry.Name, qty)
	}
	wg.Wait()

	// ── 5. Send SMS via mNotify directly (same as OTP handler) ─────────────
	smsSentCount := 0
	for i, res := range results {
		if len(res.Tickets) == 0 {
			continue
		}
		ticketList := ""
		for _, t := range res.Tickets {
			ticketList += "\n" + t
		}
		name := res.Name
		if name == "" {
			name = "Customer"
		}
		msg := "Hi " + name + "! Your WinBig Africa ticket(s) for Draw #" +
			strconv.Itoa(int(draw.DrawNumber)) + " (" + draw.GameName + "):" +
			ticketList +
			"\nDraw date: May 3, 2026. Good luck!"

		phone := bulkNormalisePhone(res.Phone)
		if err := bulkSendMNotifySMS(phone, msg); err == nil {
			smsSentCount++
			results[i].SMSSent = true
		}
	}

	// ── 6. Return summary ────────────────────────────────────────────────────
	return response.Success(w, http.StatusOK, "Bulk upload complete", map[string]interface{}{
		"total_entries":   len(req.Entries),
		"tickets_created": totalCreated,
		"sms_sent":        smsSentCount,
		"results":         results,
	})
}

// BulkUploadBySchedule creates tickets directly for a game schedule — no draw required.
// The draw is only needed at draw execution time, not at ticket creation time.
// POST /api/v1/admin/schedules/{scheduleId}/tickets/bulk-upload
func (h *drawHandler) BulkUploadBySchedule(w http.ResponseWriter, r *http.Request) error {
	scheduleID := router.GetParam(r, "scheduleId")
	if scheduleID == "" {
		return response.ValidationError(w, "scheduleId is required", nil)
	}

	// Parse request body
	var req struct {
		Entries []struct {
			Phone    string `json:"phone"`
			Name     string `json:"name"`
			Quantity int    `json:"quantity"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return response.ValidationError(w, "invalid request body", nil)
	}
	if len(req.Entries) == 0 {
		return response.ValidationError(w, "entries list is empty", nil)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	// ── 1. Resolve game_code from the schedule via an existing ticket or game service ──
	const nilUUID = "00000000-0000-0000-0000-000000000000"
	gameCode := ""

	// Try game service first
	gameClient, gcErr := h.grpcManager.GameServiceClient()
	if gcErr == nil {
		schedResp, schedErr := gameClient.GetScheduleById(ctx, &gamepb.GetScheduleByIdRequest{
			ScheduleId: scheduleID,
		})
		if schedErr == nil && schedResp.Schedule != nil {
			gameCode = schedResp.Schedule.GetGameCode()
		}
	}

	// Fallback: look up an existing ticket for this schedule
	if gameCode == "" {
		ticketClient, tcErr := h.grpcManager.TicketServiceClient()
		if tcErr == nil {
			existingResp, existingErr := ticketClient.ListTickets(ctx, &ticketv1.ListTicketsRequest{
				Filter:   &ticketv1.TicketFilter{GameScheduleId: scheduleID},
				Page:     1,
				PageSize: 1,
			})
			if existingErr == nil && len(existingResp.Tickets) > 0 {
				gameCode = existingResp.Tickets[0].GameCode
			}
		}
	}

	if gameCode == "" {
		return response.ValidationError(w, "could not resolve game code for this schedule", nil)
	}

	// ── 2. Ticket service client ─────────────────────────────────────────────
	ticketClient, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Ticket")
	}

	// ── 3. Create tickets concurrently (max 8 goroutines) ───────────────────
	type entryResult struct {
		Phone    string   `json:"phone"`
		Name     string   `json:"name"`
		Quantity int      `json:"quantity"`
		Tickets  []string `json:"tickets"`
		SMSSent  bool     `json:"sms_sent"`
		Error    string   `json:"error,omitempty"`
	}

	results := make([]entryResult, len(req.Entries))
	sem := make(chan struct{}, 8)
	var mu sync.Mutex
	totalCreated := 0

	var wg sync.WaitGroup
	for i, entry := range req.Entries {
		if entry.Phone == "" {
			results[i] = entryResult{Phone: entry.Phone, Error: "phone is required"}
			continue
		}
		qty := entry.Quantity
		if qty <= 0 {
			qty = 1
		}
		name := entry.Name

		wg.Add(1)
		go func(idx int, phone, entryName string, quantity int) {
			defer wg.Done()
			var serials []string
			var issueErr string

			for q := 0; q < quantity; q++ {
				sem <- struct{}{}
				issueResp, err := ticketClient.IssueTicket(ctx, &ticketv1.IssueTicketRequest{
					GameCode:       gameCode,
					GameScheduleId: scheduleID,
					DrawNumber:     0, // will be set when draw is created
					BetLines: []*ticketv1.BetLine{
						{LineNumber: 1, BetType: "RAFFLE", TotalAmount: 2000},
					},
					IssuerType:    "ADMIN",
					IssuerId:      "admin-bulk:" + entryName,
					CustomerPhone: phone,
					PaymentMethod: "external",
				})
				<-sem

				if err != nil {
					issueErr = err.Error()
					break
				}
				if issueResp.Ticket != nil {
					serial := issueResp.Ticket.SerialNumber
					if serial == "" {
						serial = issueResp.Ticket.Id
					}
					serials = append(serials, serial)
				}
			}

			mu.Lock()
			totalCreated += len(serials)
			results[idx] = entryResult{
				Phone:    phone,
				Name:     entryName,
				Quantity: quantity,
				Tickets:  serials,
				Error:    issueErr,
			}
			mu.Unlock()
		}(i, entry.Phone, name, qty)
	}
	wg.Wait()

	// ── 4. Send SMS via mNotify ──────────────────────────────────────────────
	smsSentCount := 0
	for i, res := range results {
		if len(res.Tickets) == 0 {
			continue
		}
		ticketList := ""
		for _, t := range res.Tickets {
			ticketList += "\n" + t
		}
		name := res.Name
		if name == "" {
			name = "Customer"
		}
		msg := "Hi " + name + "! Your WinBig Africa ticket(s) for " + gameCode + ":" +
			ticketList + "\nGood luck!"

		phone := bulkNormalisePhone(res.Phone)
		if err := bulkSendMNotifySMS(phone, msg); err == nil {
			smsSentCount++
			results[i].SMSSent = true
		}
	}

	return response.Success(w, http.StatusOK, "Bulk upload complete", map[string]interface{}{
		"total_entries":   len(req.Entries),
		"tickets_created": totalCreated,
		"sms_sent":        smsSentCount,
		"results":         results,
	})
}

// GetAgentDrawHistory retrieves draw history for agents with summary statistics
func (h *drawHandler) GetAgentDrawHistory(w http.ResponseWriter, r *http.Request) error {
	// Parse query parameters
	gameID := r.URL.Query().Get("game_id")
	gameCode := r.URL.Query().Get("game_code")
	statusParam := r.URL.Query().Get("status")
	drawIDParam := r.URL.Query().Get("draw_id")
	startDateParam := r.URL.Query().Get("start_date")
	endDateParam := r.URL.Query().Get("end_date")
	pageParam := r.URL.Query().Get("page")
	perPageParam := r.URL.Query().Get("per_page")
	includeSummaryParam := r.URL.Query().Get("include_summary")

	// Set default pagination
	page := int32(1)
	perPage := int32(20)
	if pageParam != "" {
		if p, err := strconv.ParseInt(pageParam, 10, 32); err == nil {
			page = int32(p)
		}
	}
	if perPageParam != "" {
		if pp, err := strconv.ParseInt(perPageParam, 10, 32); err == nil {
			perPage = int32(pp)
		}
	}

	// Parse include_summary (default true)
	includeSummary := true
	if includeSummaryParam == "false" {
		includeSummary = false
	}

	// Handle draw_id filter - if provided, return single draw
	if drawIDParam != "" {
		return h.getSingleDrawForAgent(w, r, drawIDParam, includeSummary)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Resolve game_code to game_id if needed
	finalGameID := gameID
	if gameCode != "" && gameID == "" {
		gameClient, err := h.grpcManager.GameServiceClient()
		if err != nil {
			h.log.Warn("Failed to get game service client for game_code lookup", "error", err)
		} else {
			// List games and find matching code
			listResp, err := gameClient.ListGames(ctx, &gamepb.ListGamesRequest{
				Page:    1,
				PerPage: 100,
			})
			if err == nil && listResp != nil {
				for _, game := range listResp.Games {
					if game.Code == gameCode {
						finalGameID = game.Id
						break
					}
				}
			}
		}
	}

	// Get Draw Service connection
	conn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	client := drawv1.NewDrawServiceClient(conn)

	// Build base request for main query
	baseReq := &drawv1.ListDrawsRequest{
		GameId:  finalGameID,
		Page:    page,
		PerPage: perPage,
	}

	// Parse status filter
	if statusParam != "" {
		status := parseDrawStatus(statusParam)
		if status != drawv1.DrawStatus_DRAW_STATUS_UNSPECIFIED {
			baseReq.StatusFilter = status
		}
	}

	// Parse date filters
	if startDateParam != "" {
		if t, err := time.Parse(time.RFC3339, startDateParam); err == nil {
			baseReq.StartDate = timestamppb.New(t)
		}
	}
	if endDateParam != "" {
		if t, err := time.Parse(time.RFC3339, endDateParam); err == nil {
			baseReq.EndDate = timestamppb.New(t)
		}
	}

	// Fetch main draw list
	mainResp, err := client.ListDraws(ctx, baseReq)
	if err != nil {
		h.log.Error("Failed to list draws", "error", err)
		return response.InternalError(w, "Failed to list draws")
	}

	if !mainResp.Success {
		return response.InternalError(w, mainResp.Message)
	}

	// Calculate summary statistics in parallel if requested
	var summary map[string]int64
	if includeSummary {
		// Only calculate summary if no status filter is applied (otherwise totals would be incorrect)
		hasStatusFilter := statusParam != "" && parseDrawStatus(statusParam) != drawv1.DrawStatus_DRAW_STATUS_UNSPECIFIED
		if !hasStatusFilter {
			summary = h.calculateDrawSummary(ctx, client, finalGameID, baseReq.StartDate, baseReq.EndDate, mainResp.TotalCount)
		} else {
			// With status filter, summary counts would be incorrect, so skip it
			h.log.Debug("Skipping summary calculation due to status filter")
		}
	}

	// Transform draws to response format
	transformedDraws := make([]map[string]interface{}, len(mainResp.Draws))
	for i, draw := range mainResp.Draws {
		drawMap := map[string]interface{}{
			"id":                 draw.Id,
			"game_id":            draw.GameId,
			"game_code":          draw.GameCode,
			"draw_number":        draw.DrawNumber,
			"game_name":          draw.GameName,
			"draw_name":          draw.DrawName,
			"status":             draw.Status,
			"scheduled_time":     draw.ScheduledTime,
			"executed_time":      draw.ExecutedTime,
			"winning_numbers":    draw.WinningNumbers,
			"total_tickets_sold": draw.TotalTicketsSold,
			"total_prize_pool":   draw.TotalPrizePool,
			"created_at":         draw.CreatedAt,
			"updated_at":         draw.UpdatedAt,
		}
		transformedDraws[i] = drawMap
	}

	// Build response
	result := map[string]interface{}{
		"data": transformedDraws,
		"pagination": map[string]interface{}{
			"page":        mainResp.Page,
			"per_page":    mainResp.PerPage,
			"total_count": mainResp.TotalCount,
			"total_pages": (mainResp.TotalCount + int64(mainResp.PerPage) - 1) / int64(mainResp.PerPage),
		},
	}

	if includeSummary && summary != nil {
		result["summary"] = summary
	}

	return response.Success(w, http.StatusOK, "Draw history retrieved successfully", result)
}

// getSingleDrawForAgent handles the case when draw_id filter is provided
func (h *drawHandler) getSingleDrawForAgent(w http.ResponseWriter, r *http.Request, drawID string, includeSummary bool) error {
	conn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	client := drawv1.NewDrawServiceClient(conn)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.GetDraw(ctx, &drawv1.GetDrawRequest{
		Id: drawID,
	})
	if err != nil {
		h.log.Error("Failed to get draw", "error", err, "draw_id", drawID)
		return response.InternalError(w, "Failed to get draw")
	}

	if !resp.Success {
		return response.NotFoundError(w, "Draw")
	}

	draw := resp.Draw
	drawMap := map[string]interface{}{
		"id":                 draw.Id,
		"game_id":            draw.GameId,
		"game_code":          draw.GameCode,
		"draw_number":        draw.DrawNumber,
		"game_name":          draw.GameName,
		"draw_name":          draw.DrawName,
		"status":             draw.Status,
		"scheduled_time":     draw.ScheduledTime,
		"executed_time":      draw.ExecutedTime,
		"winning_numbers":    draw.WinningNumbers,
		"total_tickets_sold": draw.TotalTicketsSold,
		"total_prize_pool":   draw.TotalPrizePool,
		"created_at":         draw.CreatedAt,
		"updated_at":         draw.UpdatedAt,
	}

	result := map[string]interface{}{
		"data": []map[string]interface{}{drawMap},
		"pagination": map[string]interface{}{
			"page":        1,
			"per_page":    1,
			"total_count": 1,
			"total_pages": 1,
		},
	}

	if includeSummary {
		// For single draw, summary is simple
		total := int64(1)
		var completed, pending int64
		if draw.Status == drawv1.DrawStatus_DRAW_STATUS_COMPLETED {
			completed = 1
		} else if draw.Status == drawv1.DrawStatus_DRAW_STATUS_SCHEDULED || draw.Status == drawv1.DrawStatus_DRAW_STATUS_IN_PROGRESS {
			pending = 1
		}
		result["summary"] = map[string]int64{
			"total_draws":     total,
			"completed_draws": completed,
			"pending_draws":   pending,
		}
	}

	return response.Success(w, http.StatusOK, "Draw retrieved successfully", result)
}

// calculateDrawSummary calculates summary statistics for draws
func (h *drawHandler) calculateDrawSummary(ctx context.Context, client drawv1.DrawServiceClient, gameID string, startDate, endDate *timestamppb.Timestamp, totalCount int64) map[string]int64 {
	summary := map[string]int64{
		"total_draws":     totalCount, // Use the count from main query
		"completed_draws": 0,
		"pending_draws":   0,
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Get completed draws count
	wg.Add(1)
	go func() {
		defer wg.Done()
		req := &drawv1.ListDrawsRequest{
			GameId:       gameID,
			StatusFilter: drawv1.DrawStatus_DRAW_STATUS_COMPLETED,
			Page:         1,
			PerPage:      1,
			StartDate:    startDate,
			EndDate:      endDate,
		}
		resp, err := client.ListDraws(ctx, req)
		if err != nil {
			h.log.Warn("Failed to get completed draws count", "error", err)
			return
		}
		mu.Lock()
		summary["completed_draws"] = resp.TotalCount
		mu.Unlock()
	}()

	// Get pending draws count (scheduled + in_progress)
	wg.Add(1)
	go func() {
		defer wg.Done()
		// For pending, we need to count both scheduled and in_progress
		// Make two calls and sum them
		scheduledReq := &drawv1.ListDrawsRequest{
			GameId:       gameID,
			StatusFilter: drawv1.DrawStatus_DRAW_STATUS_SCHEDULED,
			Page:         1,
			PerPage:      1,
			StartDate:    startDate,
			EndDate:      endDate,
		}
		scheduledResp, err := client.ListDraws(ctx, scheduledReq)
		scheduledCount := int64(0)
		if err == nil {
			scheduledCount = scheduledResp.TotalCount
		}

		inProgressReq := &drawv1.ListDrawsRequest{
			GameId:       gameID,
			StatusFilter: drawv1.DrawStatus_DRAW_STATUS_IN_PROGRESS,
			Page:         1,
			PerPage:      1,
			StartDate:    startDate,
			EndDate:      endDate,
		}
		inProgressResp, err := client.ListDraws(ctx, inProgressReq)
		inProgressCount := int64(0)
		if err == nil {
			inProgressCount = inProgressResp.TotalCount
		}

		mu.Lock()
		summary["pending_draws"] = scheduledCount + inProgressCount
		mu.Unlock()
	}()

	wg.Wait()

	return summary
}

// parseDrawStatus converts string status to proto DrawStatus
func parseDrawStatus(status string) drawv1.DrawStatus {
	switch status {
	case "scheduled":
		return drawv1.DrawStatus_DRAW_STATUS_SCHEDULED
	case "in_progress":
		return drawv1.DrawStatus_DRAW_STATUS_IN_PROGRESS
	case "completed":
		return drawv1.DrawStatus_DRAW_STATUS_COMPLETED
	case "failed":
		return drawv1.DrawStatus_DRAW_STATUS_FAILED
	case "cancelled":
		return drawv1.DrawStatus_DRAW_STATUS_CANCELLED
	default:
		return drawv1.DrawStatus_DRAW_STATUS_UNSPECIFIED
	}
}

// CreateDraw - Draws are created automatically by scheduler
func (h *drawHandler) CreateDraw(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, http.StatusOK, "CreateDraw not needed - draws are created automatically by scheduler", nil)
}

// UpdateDraw - Update draw details (stub)
func (h *drawHandler) UpdateDraw(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, http.StatusOK, "UpdateDraw not yet implemented", nil)
}

// DeleteDraw - Delete a draw (stub)
func (h *drawHandler) DeleteDraw(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, http.StatusOK, "DeleteDraw not yet implemented", nil)
}

// ============================================================================
// Stage 1: Preparation - Draw Execution Workflow
// ============================================================================

// PrepareDraw starts the draw preparation stage (Stage 1)
func (h *drawHandler) PrepareDraw(w http.ResponseWriter, r *http.Request) error {
	drawID := router.GetParam(r, "id")
	if drawID == "" {
		return response.ValidationError(w, "draw_id is required", nil)
	}

	// Extract email from JWT token (set by auth middleware)
	initiatedBy := router.GetEmail(r)
	if initiatedBy == "" {
		h.log.Error("Failed to extract email from JWT token", "draw_id", drawID)
		return response.UnauthorizedError(w, "Invalid authentication token")
	}

	// Parse request body
	var req struct {
		Complete bool `json:"complete"` // If true, complete preparation; otherwise just start
	}
	if err := parseRequestBody(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{"error": err.Error()})
	}

	// Get Draw Service connection
	conn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	client := drawv1.NewDrawServiceClient(conn)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var grpcResp interface{ GetDraw() *drawv1.Draw }

	if req.Complete {
		// Complete preparation stage
		resp, err := client.CompleteDrawPreparation(ctx, &drawv1.CompleteDrawPreparationRequest{
			DrawId:      drawID,
			CompletedBy: initiatedBy,
		})
		if err != nil {
			h.log.Error("Failed to complete draw preparation", "error", err, "draw_id", drawID)
			return response.InternalError(w, "Failed to complete draw preparation")
		}
		grpcResp = resp
	} else {
		// Start preparation stage
		resp, err := client.StartDrawPreparation(ctx, &drawv1.StartDrawPreparationRequest{
			DrawId:      drawID,
			InitiatedBy: initiatedBy,
		})
		if err != nil {
			h.log.Error("Failed to start draw preparation", "error", err, "draw_id", drawID)
			return response.InternalError(w, "Failed to start draw preparation")
		}
		grpcResp = resp
	}

	draw := grpcResp.GetDraw()
	return response.Success(w, http.StatusOK, "Draw preparation updated successfully", draw)
}

// ============================================================================
// Stage 2: Number Selection - Draw Execution Workflow
// ============================================================================

// ExecuteDraw handles the number selection stage (Stage 2)
func (h *drawHandler) ExecuteDraw(w http.ResponseWriter, r *http.Request) error {
	drawID := router.GetParam(r, "id")
	if drawID == "" {
		return response.ValidationError(w, "draw_id is required", nil)
	}

	// Parse request body
	var req struct {
		Action         string  `json:"action"` // "start", "submit_verification", "validate", "reset", "complete"
		SubmittedBy    string  `json:"submitted_by"`
		Numbers        []int32 `json:"numbers,omitempty"`
		ValidatedBy    string  `json:"validated_by,omitempty"`
		ResetBy        string  `json:"reset_by,omitempty"`
		ResetReason    string  `json:"reset_reason,omitempty"`
		CompletedBy    string  `json:"completed_by,omitempty"`
		WinningNumbers []int32 `json:"winning_numbers,omitempty"`
	}
	if err := parseRequestBody(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{"error": err.Error()})
	}

	// Get Draw Service connection
	conn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	client := drawv1.NewDrawServiceClient(conn)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	switch req.Action {
	case "start":
		// Extract email from JWT token (set by auth middleware)
		initiatedBy := router.GetEmail(r)
		if initiatedBy == "" {
			h.log.Error("Failed to extract email from JWT token", "draw_id", drawID)
			return response.UnauthorizedError(w, "Invalid authentication token")
		}

		resp, err := client.StartNumberSelection(ctx, &drawv1.StartNumberSelectionRequest{
			DrawId:      drawID,
			InitiatedBy: initiatedBy,
		})
		if err != nil {
			h.log.Error("Failed to start number selection", "error", err, "draw_id", drawID)
			return response.InternalError(w, "Failed to start number selection")
		}
		return response.Success(w, http.StatusOK, "Number selection started", resp.Draw)

	case "submit_verification":
		// Extract email from JWT token (set by auth middleware)
		submittedBy := router.GetEmail(r)
		if submittedBy == "" {
			h.log.Error("Failed to extract email from JWT token", "draw_id", drawID)
			return response.UnauthorizedError(w, "Invalid authentication token")
		}

		resp, err := client.SubmitVerificationAttempt(ctx, &drawv1.SubmitVerificationAttemptRequest{
			DrawId:      drawID,
			Numbers:     req.Numbers,
			SubmittedBy: submittedBy,
		})
		if err != nil {
			h.log.Error("Failed to submit verification attempt", "error", err, "draw_id", drawID)
			return response.InternalError(w, "Failed to submit verification attempt")
		}
		result := map[string]interface{}{
			"draw":           resp.Draw,
			"attempt_number": resp.AttemptNumber,
		}
		return response.Success(w, http.StatusOK, "Verification attempt submitted", result)

	case "validate":
		resp, err := client.ValidateVerificationAttempts(ctx, &drawv1.ValidateVerificationAttemptsRequest{
			DrawId:      drawID,
			ValidatedBy: req.ValidatedBy,
		})
		if err != nil {
			h.log.Error("Failed to validate verification attempts", "error", err, "draw_id", drawID)
			return response.InternalError(w, "Failed to validate verification attempts")
		}
		result := map[string]interface{}{
			"is_valid":         resp.IsValid,
			"winning_numbers":  resp.WinningNumbers,
			"validation_error": resp.ValidationError,
		}
		return response.Success(w, http.StatusOK, resp.Message, result)

	case "reset":
		// Extract email from JWT token (set by auth middleware)
		resetBy := router.GetEmail(r)
		if resetBy == "" {
			h.log.Error("Failed to extract email from JWT token", "draw_id", drawID)
			return response.UnauthorizedError(w, "Invalid authentication token")
		}

		resp, err := client.ResetVerificationAttempts(ctx, &drawv1.ResetVerificationAttemptsRequest{
			DrawId:  drawID,
			ResetBy: resetBy,
			Reason:  req.ResetReason,
		})
		if err != nil {
			h.log.Error("Failed to reset verification attempts", "error", err, "draw_id", drawID)
			return response.InternalError(w, "Failed to reset verification attempts")
		}
		return response.Success(w, http.StatusOK, "Verification attempts reset", resp.Draw)

	case "complete":
		resp, err := client.CompleteNumberSelection(ctx, &drawv1.CompleteNumberSelectionRequest{
			DrawId:         drawID,
			WinningNumbers: req.WinningNumbers,
			CompletedBy:    req.CompletedBy,
		})
		if err != nil {
			h.log.Error("Failed to complete number selection", "error", err, "draw_id", drawID)
			return response.InternalError(w, "Failed to complete number selection")
		}
		return response.Success(w, http.StatusOK, "Number selection completed", resp.Draw)

	default:
		return response.ValidationError(w, "Invalid action", map[string]string{"action": req.Action})
	}
}

// SaveDrawProgress - Not needed for our 4-stage workflow
func (h *drawHandler) SaveDrawProgress(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, http.StatusOK, "SaveDrawProgress not needed - workflow automatically persists stage data", nil)
}

// RestartDraw - Not needed for our 4-stage workflow
func (h *drawHandler) RestartDraw(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, http.StatusOK, "RestartDraw not needed - use ResetVerificationAttempts for Stage 2", nil)
}

// RecordPhysicalDraw - Maps to number selection stage
func (h *drawHandler) RecordPhysicalDraw(w http.ResponseWriter, r *http.Request) error {
	// This is an alias for ExecuteDraw with action="complete"
	return h.ExecuteDraw(w, r)
}

// ============================================================================
// Stage 3: Result Calculation - Draw Execution Workflow
// ============================================================================

// CommitDrawResults calculates and commits draw results (Stage 3)
func (h *drawHandler) CommitDrawResults(w http.ResponseWriter, r *http.Request) error {
	drawID := router.GetParam(r, "id")
	if drawID == "" {
		return response.ValidationError(w, "draw_id is required", nil)
	}

	// Extract email from JWT token (set by auth middleware)
	committedBy := router.GetEmail(r)
	if committedBy == "" {
		h.log.Error("Failed to extract email from JWT token", "draw_id", drawID)
		return response.UnauthorizedError(w, "Invalid authentication token")
	}

	// Get Draw Service connection
	conn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	client := drawv1.NewDrawServiceClient(conn)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	resp, err := client.CommitResults(ctx, &drawv1.CommitResultsRequest{
		DrawId:      drawID,
		CommittedBy: committedBy,
	})
	if err != nil {
		h.log.Error("Failed to commit draw results", "error", err, "draw_id", drawID)
		return response.InternalError(w, "Failed to commit draw results")
	}

	result := map[string]interface{}{
		"draw":                  resp.Draw,
		"winning_tickets_count": resp.WinningTicketsCount,
		"total_winnings":        resp.TotalWinnings,
		"winning_tiers":         resp.WinningTiers,
	}

	return response.Success(w, http.StatusOK, "Draw results committed successfully", result)
}

// ============================================================================
// Stage 4: Payout Processing - Draw Execution Workflow
// ============================================================================

// ProcessPayout handles payout processing (Stage 4)
func (h *drawHandler) ProcessPayout(w http.ResponseWriter, r *http.Request) error {
	drawID := router.GetParam(r, "id")
	if drawID == "" {
		return response.ValidationError(w, "draw_id is required", nil)
	}

	// Parse request body - support both old and new formats
	var req struct {
		// New format (frontend sends this)
		PayoutMode     string `json:"payout_mode"`      // "auto" or "manual"
		ExcludeBigWins *bool  `json:"exclude_big_wins"` // optional boolean

		// Old format (for backward compatibility)
		Action          string `json:"action"` // "process", "approve_big_win", "reject_big_win", "complete"
		ProcessedBy     string `json:"processed_by"`
		TicketID        string `json:"ticket_id,omitempty"`
		Approve         bool   `json:"approve,omitempty"`
		RejectionReason string `json:"rejection_reason,omitempty"`
		CompletedBy     string `json:"completed_by,omitempty"`
	}
	if err := parseRequestBody(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{"error": err.Error()})
	}

	// Get admin email from auth context
	processedBy := "admin@randlottery.com" // Default admin email
	if authClaims, ok := r.Context().Value("auth").(map[string]interface{}); ok {
		if email, ok := authClaims["email"].(string); ok && email != "" {
			processedBy = email
		}
	}

	// Get Draw Service connection
	conn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	client := drawv1.NewDrawServiceClient(conn)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// Handle new format from frontend (payout_mode present)
	if req.PayoutMode != "" {
		// Frontend sends payout_mode: "auto" with exclude_big_wins: true
		// This maps to the "process" action
		resp, err := client.ProcessPayouts(ctx, &drawv1.ProcessPayoutsRequest{
			DrawId:      drawID,
			ProcessedBy: processedBy,
		})
		if err != nil {
			h.log.Error("Failed to process payouts", "error", err, "draw_id", drawID)
			return response.InternalError(w, "Failed to process payouts")
		}
		result := map[string]interface{}{
			"draw":                   resp.Draw,
			"auto_processed_count":   resp.AutoProcessedCount,
			"manual_approval_count":  resp.ManualApprovalCount,
			"auto_processed_amount":  resp.AutoProcessedAmount,
			"manual_approval_amount": resp.ManualApprovalAmount,
		}
		return response.Success(w, http.StatusOK, "Payouts processed successfully", result)
	}

	// Handle old format (action-based)
	switch req.Action {
	case "process":
		if req.ProcessedBy == "" {
			req.ProcessedBy = processedBy
		}
		resp, err := client.ProcessPayouts(ctx, &drawv1.ProcessPayoutsRequest{
			DrawId:      drawID,
			ProcessedBy: req.ProcessedBy,
		})
		if err != nil {
			h.log.Error("Failed to process payouts", "error", err, "draw_id", drawID)
			return response.InternalError(w, "Failed to process payouts")
		}
		result := map[string]interface{}{
			"draw":                   resp.Draw,
			"auto_processed_count":   resp.AutoProcessedCount,
			"manual_approval_count":  resp.ManualApprovalCount,
			"auto_processed_amount":  resp.AutoProcessedAmount,
			"manual_approval_amount": resp.ManualApprovalAmount,
		}
		return response.Success(w, http.StatusOK, "Payouts processed successfully", result)

	case "approve_big_win", "reject_big_win":
		approve := req.Action == "approve_big_win"
		if req.ProcessedBy == "" {
			req.ProcessedBy = processedBy
		}
		resp, err := client.ProcessBigWinPayout(ctx, &drawv1.ProcessBigWinPayoutRequest{
			DrawId:          drawID,
			TicketId:        req.TicketID,
			Approve:         approve,
			ProcessedBy:     req.ProcessedBy,
			RejectionReason: req.RejectionReason,
		})
		if err != nil {
			h.log.Error("Failed to process big win payout", "error", err, "draw_id", drawID, "ticket_id", req.TicketID)
			return response.InternalError(w, "Failed to process big win payout")
		}
		return response.Success(w, http.StatusOK, "Big win payout processed", resp.Payout)

	case "complete":
		if req.CompletedBy == "" {
			req.CompletedBy = processedBy
		}
		resp, err := client.CompleteDraw(ctx, &drawv1.CompleteDrawRequest{
			DrawId:      drawID,
			CompletedBy: req.CompletedBy,
		})
		if err != nil {
			h.log.Error("Failed to complete draw", "error", err, "draw_id", drawID)
			return response.InternalError(w, "Failed to complete draw")
		}
		return response.Success(w, http.StatusOK, "Draw completed successfully", resp.Draw)

	default:
		return response.ValidationError(w, "Invalid action or payout_mode", map[string]string{"action": req.Action, "payout_mode": req.PayoutMode})
	}
}

// UpdateMachineNumbers updates machine numbers for a completed draw
func (h *drawHandler) UpdateMachineNumbers(w http.ResponseWriter, r *http.Request) error {
	drawID := router.GetParam(r, "id")
	if drawID == "" {
		return response.ValidationError(w, "draw_id is required", nil)
	}

	// Parse request body
	var req struct {
		MachineNumbers []int32 `json:"machine_numbers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Failed to decode request body", "error", err)
		return response.ValidationError(w, "Invalid request body", nil)
	}

	// Validate machine numbers
	if len(req.MachineNumbers) == 0 {
		return response.ValidationError(w, "machine_numbers is required", nil)
	}

	// Get Draw Service connection
	conn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	client := drawv1.NewDrawServiceClient(conn)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Extract user email from JWT context
	updatedBy, ok := r.Context().Value(router.ContextEmail).(string)
	if !ok || updatedBy == "" {
		return response.UnauthorizedError(w, "User information not available")
	}

	// Call Draw Service
	resp, err := client.UpdateMachineNumbers(ctx, &drawv1.UpdateMachineNumbersRequest{
		DrawId:         drawID,
		MachineNumbers: req.MachineNumbers,
		UpdatedBy:      updatedBy,
	})
	if err != nil {
		h.log.Error("Failed to update machine numbers", "error", err, "draw_id", drawID)
		return response.InternalError(w, "Failed to update machine numbers")
	}

	if !resp.Success {
		return response.InternalError(w, resp.Message)
	}

	return response.Success(w, http.StatusOK, "Machine numbers updated successfully", resp.Draw)
}

// ============================================================================
// Draw Results and Statistics
// ============================================================================

// GetDrawResults retrieves draw results
func (h *drawHandler) GetDrawResults(w http.ResponseWriter, r *http.Request) error {
	drawID := router.GetParam(r, "id")
	if drawID == "" {
		return response.ValidationError(w, "draw_id is required", nil)
	}

	// Get Draw Service connection
	conn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	client := drawv1.NewDrawServiceClient(conn)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Get draw with full stage data
	resp, err := client.GetDraw(ctx, &drawv1.GetDrawRequest{
		Id: drawID,
	})
	if err != nil {
		h.log.Error("Failed to get draw results", "error", err, "draw_id", drawID)
		return response.InternalError(w, "Failed to get draw results")
	}

	if !resp.Success {
		return response.NotFoundError(w, "Draw")
	}

	// Extract relevant result data from stage
	draw := resp.Draw
	result := map[string]interface{}{
		"draw_id":         draw.Id,
		"winning_numbers": draw.WinningNumbers,
		"status":          draw.Status,
		"executed_time":   draw.ExecutedTime,
		"stage":           draw.Stage,
	}

	return response.Success(w, http.StatusOK, "Draw results retrieved successfully", result)
}

// ValidateDraw - Validation happens automatically in the workflow
func (h *drawHandler) ValidateDraw(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, http.StatusOK, "ValidateDraw not needed - validation happens in Stage 2 (ValidateVerificationAttempts)", nil)
}

// GetDrawStatistics retrieves draw statistics
func (h *drawHandler) GetDrawStatistics(w http.ResponseWriter, r *http.Request) error {
	drawID := router.GetParam(r, "id")
	if drawID == "" {
		return response.ValidationError(w, "draw_id is required", nil)
	}

	// Get Draw Service connection
	conn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	client := drawv1.NewDrawServiceClient(conn)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.GetDraw(ctx, &drawv1.GetDrawRequest{
		Id: drawID,
	})
	if err != nil {
		h.log.Error("Failed to get draw statistics", "error", err, "draw_id", drawID)
		return response.InternalError(w, "Failed to get draw statistics")
	}

	if !resp.Success {
		return response.NotFoundError(w, "Draw")
	}

	draw := resp.Draw
	stage := draw.Stage

	// Count actual tickets from the ticket service using game_schedule_id so that
	// tickets created after the draw record was inserted are also counted.
	// Only count tickets with payment_status = "completed" — failed/pending payments
	// are not valid draw entries and must not inflate the ticket count.
	liveTicketCount := int64(draw.TotalTicketsSold)
	liveTotalStakes := int64(0)
	const nilUUID = "00000000-0000-0000-0000-000000000000"
	scheduleIdForStats := draw.GameScheduleId
	if scheduleIdForStats == nilUUID {
		scheduleIdForStats = ""
	}
	if scheduleIdForStats != "" {
		ticketClient, tcErr := h.grpcManager.TicketServiceClient()
		if tcErr == nil {
			paidFilter := &ticketv1.TicketFilter{
				GameScheduleId: scheduleIdForStats,
				PaymentStatus:  "completed",
			}
			ticketResp, tcErr := ticketClient.ListTickets(ctx, &ticketv1.ListTicketsRequest{
				Filter:   paidFilter,
				Page:     1,
				PageSize: 1, // We only need the total count
			})
			if tcErr == nil {
				liveTicketCount = int64(ticketResp.Total)
				// Sum stakes from all paid tickets (fetch a larger page for stake sum)
				allTickets, tcErr2 := ticketClient.ListTickets(ctx, &ticketv1.ListTicketsRequest{
					Filter:   paidFilter,
					Page:     1,
					PageSize: int32(ticketResp.Total) + 1,
				})
				if tcErr2 == nil {
					for _, t := range allTickets.Tickets {
						liveTotalStakes += t.TotalAmount
					}
				}
			}
		}
	}

	// Build statistics from stage data
	stats := map[string]interface{}{
		"draw_id":            draw.Id,
		"status":             draw.Status,
		"total_tickets":      liveTicketCount, // matches frontend field name
		"total_tickets_sold": liveTicketCount, // alias for compatibility
		"total_prize_pool":   draw.TotalPrizePool,
		"total_stakes":       liveTotalStakes,
	}

	if stage != nil {
		stats["current_stage"] = stage.CurrentStage
		stats["stage_name"] = stage.StageName
		stats["stage_status"] = stage.StageStatus

		if stage.PreparationData != nil {
			stats["tickets_locked"] = stage.PreparationData.TicketsLocked
			// Only override live stakes with preparation data when it's been populated
			if stage.PreparationData.TotalStakes > 0 {
				stats["total_stakes"] = stage.PreparationData.TotalStakes
			}
		}

		if stage.ResultCalculationData != nil {
			stats["winning_tickets_count"] = stage.ResultCalculationData.WinningTicketsCount
			stats["total_winnings"] = stage.ResultCalculationData.TotalWinnings
			stats["winning_tiers"] = stage.ResultCalculationData.WinningTiers
		}

		if stage.PayoutData != nil {
			stats["auto_processed_count"] = stage.PayoutData.AutoProcessedCount
			stats["manual_approval_count"] = stage.PayoutData.ManualApprovalCount
			stats["processed_count"] = stage.PayoutData.ProcessedCount
			stats["pending_count"] = stage.PayoutData.PendingCount
		}
	}

	return response.Success(w, http.StatusOK, "Draw statistics retrieved successfully", stats)
}

// GetWinningNumbers retrieves winning numbers for a draw
func (h *drawHandler) GetWinningNumbers(w http.ResponseWriter, r *http.Request) error {
	drawID := router.GetParam(r, "id")
	if drawID == "" {
		return response.ValidationError(w, "draw_id is required", nil)
	}

	// Get Draw Service connection
	conn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	client := drawv1.NewDrawServiceClient(conn)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.GetDraw(ctx, &drawv1.GetDrawRequest{
		Id: drawID,
	})
	if err != nil {
		h.log.Error("Failed to get winning numbers", "error", err, "draw_id", drawID)
		return response.InternalError(w, "Failed to get winning numbers")
	}

	if !resp.Success {
		return response.NotFoundError(w, "Draw")
	}

	draw := resp.Draw
	result := map[string]interface{}{
		"draw_id":         draw.Id,
		"winning_numbers": draw.WinningNumbers,
		"status":          draw.Status,
		"executed_time":   draw.ExecutedTime,
	}

	return response.Success(w, http.StatusOK, "Winning numbers retrieved successfully", result)
}

// GetPublicCompletedDraws retrieves completed draws with winning numbers (public, no auth)
func (h *drawHandler) GetPublicCompletedDraws(w http.ResponseWriter, r *http.Request) error {
	// Parse query parameters
	gameID := r.URL.Query().Get("game_id")
	gameCode := r.URL.Query().Get("game_code")
	latestOnlyParam := r.URL.Query().Get("latest_only")
	startDateParam := r.URL.Query().Get("start_date")
	endDateParam := r.URL.Query().Get("end_date")
	pageParam := r.URL.Query().Get("page")
	perPageParam := r.URL.Query().Get("per_page")

	// Set default pagination
	page := int32(1)
	perPage := int32(10)

	if pageParam != "" {
		if p, err := strconv.ParseInt(pageParam, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}
	if perPageParam != "" {
		if pp, err := strconv.ParseInt(perPageParam, 10, 32); err == nil && pp > 0 {
			perPage = int32(pp)
			if perPage > 50 {
				perPage = 50 // Enforce maximum
			}
		}
	}

	latestOnly := latestOnlyParam == "true"

	// Build request
	req := &drawv1.GetPublicCompletedDrawsRequest{
		GameId:     gameID,
		GameCode:   gameCode,
		LatestOnly: latestOnly,
		Page:       page,
		PerPage:    perPage,
	}

	// Parse date filters (support multiple formats: YYYY-MM-DD, RFC3339)
	if startDateParam != "" {
		// Try parsing as YYYY-MM-DD first (simpler format)
		if t, err := time.Parse("2006-01-02", startDateParam); err == nil {
			req.StartDate = timestamppb.New(t)
		} else if t, err := time.Parse(time.RFC3339, startDateParam); err == nil {
			// Fall back to RFC3339
			req.StartDate = timestamppb.New(t)
		}
	}
	if endDateParam != "" {
		// Try parsing as YYYY-MM-DD first (simpler format)
		if t, err := time.Parse("2006-01-02", endDateParam); err == nil {
			// Set to end of day (23:59:59)
			req.EndDate = timestamppb.New(t.Add(23*time.Hour + 59*time.Minute + 59*time.Second))
		} else if t, err := time.Parse(time.RFC3339, endDateParam); err == nil {
			// Fall back to RFC3339
			req.EndDate = timestamppb.New(t)
		}
	}

	// Get Draw Service connection
	conn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	client := drawv1.NewDrawServiceClient(conn)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.GetPublicCompletedDraws(ctx, req)
	if err != nil {
		h.log.Error("Failed to get public completed draws", "error", err)
		return response.InternalError(w, "Failed to get completed draws")
	}

	if !resp.Success {
		return response.InternalError(w, resp.Message)
	}

	// Transform to simpler public format
	draws := make([]map[string]interface{}, len(resp.Draws))
	for i, draw := range resp.Draws {
		draws[i] = map[string]interface{}{
			"draw_id":          draw.DrawId,
			"game_id":          draw.GameId,
			"game_name":        draw.GameName,
			"game_schedule_id": draw.GameScheduleId,
			"draw_number":      draw.DrawNumber,
			"winning_numbers":  draw.WinningNumbers,
			"machine_numbers":  draw.MachineNumbers,
			"draw_date":        draw.DrawDate,
			"draw_name":        draw.DrawName,
			"game_logo_url":    draw.GameLogoUrl,
			"game_brand_color": draw.GameBrandColor,
		}
	}

	result := map[string]interface{}{
		"draws":       draws,
		"total_count": resp.TotalCount,
		"page":        resp.Page,
		"per_page":    resp.PerPage,
		"total_pages": resp.TotalPages,
	}

	return response.Success(w, http.StatusOK, resp.Message, result)
}

// GetCompletedDraws retrieves completed draws for retailers (with auth)
func (h *drawHandler) GetCompletedDraws(w http.ResponseWriter, r *http.Request) error {
	// This uses the same handler as public, but is accessed through authenticated route
	return h.GetPublicCompletedDraws(w, r)
}

// GetDrawTickets retrieves all tickets attached to a draw
func (h *drawHandler) GetDrawTickets(w http.ResponseWriter, r *http.Request) error {
	drawID := router.GetParam(r, "id")
	if drawID == "" {
		return response.ValidationError(w, "draw_id is required", nil)
	}

	// Parse query parameters for filtering
	status := r.URL.Query().Get("status")           // e.g., "issued", "winning", "paid"
	issuerID := r.URL.Query().Get("issuer_id")      // Filter by specific retailer/agent
	pageParam := r.URL.Query().Get("page")          // Pagination page number
	pageSizeParam := r.URL.Query().Get("page_size") // Pagination page size

	// Set default pagination
	page := int32(1)
	pageSize := int32(100)

	if pageParam != "" {
		if p, err := strconv.ParseInt(pageParam, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}
	if pageSizeParam != "" {
		if ps, err := strconv.ParseInt(pageSizeParam, 10, 32); err == nil && ps > 0 {
			pageSize = int32(ps)
		}
	}

	// First, get the draw to extract game_id and draw_number
	drawConn, err := h.grpcManager.GetConnection("draw")
	if err != nil {
		h.log.Error("Failed to get draw service connection", "error", err)
		return response.ServiceUnavailableError(w, "Draw")
	}

	drawClient := drawv1.NewDrawServiceClient(drawConn)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	drawResp, err := drawClient.GetDraw(ctx, &drawv1.GetDrawRequest{
		Id: drawID,
	})
	if err != nil {
		h.log.Error("Failed to get draw", "error", err, "draw_id", drawID)
		return response.InternalError(w, "Failed to get draw")
	}

	if !drawResp.Success {
		return response.NotFoundError(w, "Draw")
	}

	draw := drawResp.Draw

	// Get Ticket Service connection
	ticketClient, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.ServiceUnavailableError(w, "Ticket")
	}

	// Filter by game_schedule_id so we catch all tickets for this draw's schedule,
	// including those created after the draw record was inserted (which won't have
	// draw_id set yet). We also backfill draw_id on those tickets so both fields
	// stay in sync going forward.
	//
	// NOTE: We show ALL tickets here (including failed payments) so admins can see
	// the full picture. The draw execution (Stage 3) separately enforces
	// payment_status = "completed" when selecting eligible tickets.
	const nilUUID = "00000000-0000-0000-0000-000000000000"
	filter := &ticketv1.TicketFilter{}
	if draw.GameScheduleId != "" && draw.GameScheduleId != nilUUID {
		filter.GameScheduleId = draw.GameScheduleId
		// Backfill draw_id on any tickets that are missing it for this schedule
		go func() {
			_, err := ticketClient.UpdateTicketsDrawId(ctx, &ticketv1.UpdateTicketsDrawIdRequest{
				GameScheduleId: draw.GameScheduleId,
				DrawId:         drawID,
			})
			if err != nil {
				h.log.Warn("Failed to backfill draw_id on tickets", "draw_id", drawID, "schedule_id", draw.GameScheduleId, "error", err)
			}
		}()
	} else {
		// Fallback: no schedule linked, filter by draw_id directly
		filter.DrawId = drawID
	}

	// Only show paid tickets — unpaid/failed entries are not valid draw participants
	filter.PaymentStatus = "completed"

	// Add optional filters
	if status != "" {
		filter.Status = status
	}
	if issuerID != "" {
		filter.IssuerId = issuerID
	}

	// Build ListTickets request
	ticketReq := &ticketv1.ListTicketsRequest{
		Filter:   filter,
		Page:     page,
		PageSize: pageSize,
	}

	// Call Ticket Service
	ticketResp, err := ticketClient.ListTickets(ctx, ticketReq)
	if err != nil {
		h.log.Error("Failed to list tickets", "error", err, "draw_id", drawID)
		return response.InternalError(w, "Failed to fetch tickets")
	}

	// Build response with pagination metadata
	result := map[string]interface{}{
		"draw_id":     draw.Id,
		"game_id":     draw.GameId,
		"draw_number": draw.DrawNumber,
		"tickets":     ticketResp.Tickets,
		"total":       ticketResp.Total,
		"page":        ticketResp.Page,
		"page_size":   ticketResp.PageSize,
	}

	return response.Success(w, http.StatusOK, "Tickets retrieved successfully", result)
}

// GetPublicWinners returns winning tickets for completed draws (public, no auth)
// GET /api/v1/public/winners?limit=20
func (h *drawHandler) GetPublicWinners(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	limit := int32(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.ParseInt(l, 10, 32); err == nil && v > 0 && v <= 50 {
			limit = int32(v)
		}
	}

	ticketClient, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.ServiceUnavailableError(w, "Ticket")
	}

	ticketResp, err := ticketClient.ListTickets(ctx, &ticketv1.ListTicketsRequest{
		Filter:   &ticketv1.TicketFilter{Status: "won"},
		Page:     1,
		PageSize: limit,
	})
	if err != nil {
		h.log.Error("Failed to list winning tickets", "error", err)
		return response.InternalError(w, "Failed to retrieve winners")
	}

	type winner struct {
		Name         string `json:"name"`
		Prize        string `json:"prize"`
		SerialNumber string `json:"serial_number"`
		DrawDate     string `json:"draw_date"`
	}

	winners := make([]winner, 0, len(ticketResp.Tickets))
	for _, t := range ticketResp.Tickets {
		// Redact phone for privacy: show first 4 digits + ****
		name := "Winner"
		if t.CustomerPhone != "" {
			phone := t.CustomerPhone
			if len(phone) > 4 {
				name = phone[:4] + "****"
			} else {
				name = phone
			}
		}

		drawDate := ""
		if t.DrawDate != nil {
			drawDate = t.DrawDate.AsTime().Format("Jan 2, 2006")
		} else if t.CreatedAt != nil {
			drawDate = t.CreatedAt.AsTime().Format("Jan 2, 2006")
		}

		winners = append(winners, winner{
			Name:         name,
			Prize:        t.GameName,
			SerialNumber: t.SerialNumber,
			DrawDate:     drawDate,
		})
	}

	return response.Success(w, http.StatusOK, "Winners retrieved successfully", map[string]interface{}{
		"winners": winners,
		"total":   ticketResp.Total,
	})
}



// ── mNotify helpers for bulk upload SMS ─────────────────────────────────────

const (
	bulkMNotifyAPIKey = "F9XhjQbbJnqKt2fy9lhPIQCSD"
	bulkMNotifySender = "CARPARK"
	bulkMNotifyURL    = "https://api.mnotify.com/api/sms/quick"
)

// bulkNormalisePhone converts 233XXXXXXXXX or +233XXXXXXXXX to 0XXXXXXXXX for mNotify.
func bulkNormalisePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if strings.HasPrefix(phone, "+233") {
		return "0" + phone[4:]
	}
	if strings.HasPrefix(phone, "233") {
		return "0" + phone[3:]
	}
	return phone
}

// bulkSendMNotifySMS sends a single SMS via mNotify.
func bulkSendMNotifySMS(phone, message string) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"recipient":     []string{phone},
		"sender":        bulkMNotifySender,
		"message":       message,
		"is_schedule":   false,
		"schedule_date": "",
	})
	url := fmt.Sprintf("%s?key=%s", bulkMNotifyURL, bulkMNotifyAPIKey)
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if status, _ := result["status"].(string); status != "success" {
		return fmt.Errorf("mNotify error: %v", result["message"])
	}
	return nil
}

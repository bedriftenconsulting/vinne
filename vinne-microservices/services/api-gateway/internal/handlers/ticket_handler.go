package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	agentmgmtv1 "github.com/randco/randco-microservices/proto/agent/management/v1"
	playerv1 "github.com/randco/randco-microservices/proto/player/v1"
	ticketv1 "github.com/randco/randco-microservices/proto/ticket/v1"
	walletv1 "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/response"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ticketHandler struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
}

// NewTicketHandler creates a new ticket handler with gRPC integration
func NewTicketHandler(grpcManager *grpc.ClientManager, log logger.Logger) *ticketHandler {
	return &ticketHandler{
		grpcManager: grpcManager,
		log:         log,
	}
}

// JSON request/response models

type BetLine struct {
	LineNumber int32  `json:"line_number"`
	BetType    string `json:"bet_type"` // "DIRECT-1", "PERM-2", "PERM-3", "BANKER", "BANKER ALL", "AGAINST"

	// For DIRECT and PERM bets
	SelectedNumbers []int32 `json:"selected_numbers,omitempty"` // Player's chosen numbers

	// For BANKER and AGAINST bets
	Banker  []int32 `json:"banker,omitempty"`
	Opposed []int32 `json:"opposed,omitempty"`

	// For PERM and Banker bets (compact format)
	NumberOfCombinations *int32 `json:"number_of_combinations,omitempty"` // C(n,r) - calculated value
	AmountPerCombination *int64 `json:"amount_per_combination,omitempty"` // Amount per combination in pesewas

	// Common fields
	TotalAmount int64 `json:"total_amount"` // Total bet amount in pesewas
}

type IssuerDetails struct {
	TerminalID   *string `json:"terminal_id,omitempty"`
	RetailerCode *string `json:"retailer_code,omitempty"`
	PlayerID     *string `json:"player_id,omitempty"`
	Location     *string `json:"location,omitempty"`
}

type IssueTicketRequest struct {
	GameCode        string         `json:"game_code"`
	GameScheduleID  string         `json:"game_schedule_id"`
	DrawNumber      int32          `json:"draw_number"`
	SelectedNumbers []int32        `json:"selected_numbers,omitempty"`
	BetLines        []BetLine      `json:"bet_lines"`
	IssuerType      string         `json:"issuer_type"`
	IssuerID        string         `json:"issuer_id"`
	IssuerDetails   *IssuerDetails `json:"issuer_details,omitempty"`
	CustomerPhone   string         `json:"customer_phone,omitempty"`
	CustomerEmail   string         `json:"customer_email,omitempty"`
	PaymentMethod   string         `json:"payment_method"`
}

type ValidateTicketRequest struct {
	ValidatorID   string `json:"validator_id"`
	ValidatorType string `json:"validator_type"`
}

type CancelTicketRequest struct {
	Reason      string `json:"reason"`
	CancelledBy string `json:"cancelled_by"`
}

type VoidTicketRequest struct {
	Reason   string `json:"reason"`
	VoidedBy string `json:"voided_by"`
}

type ReprintTicketRequest struct {
	ReprintedBy string `json:"reprinted_by"`
	TerminalID  string `json:"terminal_id"`
}

type ListTicketsRequest struct {
	Status     string `json:"status,omitempty"`
	GameCode   string `json:"game_code,omitempty"`
	ScheduleID string `json:"schedule_id,omitempty"`
	DrawNumber int32  `json:"draw_number,omitempty"`
	IssuerType string `json:"issuer_type,omitempty"`
	IssuerID   string `json:"issuer_id,omitempty"`
	StartDate  string `json:"start_date,omitempty"`
	EndDate    string `json:"end_date,omitempty"`
	Page       int32  `json:"page,omitempty"`
	PageSize   int32  `json:"page_size,omitempty"`
}

// Handler methods

// IssueTicket creates a new lottery ticket
func (h *ticketHandler) IssueTicket(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var req IssueTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	// Convert bet lines to proto
	var protoBetLines []*ticketv1.BetLine
	for _, bl := range req.BetLines {
		protoBetLine := &ticketv1.BetLine{
			LineNumber:      bl.LineNumber,
			BetType:         bl.BetType,
			SelectedNumbers: bl.SelectedNumbers,
			Banker:          bl.Banker,
			Opposed:         bl.Opposed,
			TotalAmount:     bl.TotalAmount,
		}

		// Add compact format fields if provided
		if bl.NumberOfCombinations != nil {
			protoBetLine.NumberOfCombinations = *bl.NumberOfCombinations
		}
		if bl.AmountPerCombination != nil {
			protoBetLine.AmountPerCombination = *bl.AmountPerCombination
		}

		protoBetLines = append(protoBetLines, protoBetLine)
	}

	// Convert issuer details if provided
	var protoIssuerDetails *ticketv1.IssuerDetails
	if req.IssuerDetails != nil {
		protoIssuerDetails = &ticketv1.IssuerDetails{
			TerminalId:   stringPtrValue(req.IssuerDetails.TerminalID),
			RetailerCode: stringPtrValue(req.IssuerDetails.RetailerCode),
			PlayerId:     stringPtrValue(req.IssuerDetails.PlayerID),
			Location:     stringPtrValue(req.IssuerDetails.Location),
		}
	}

	protoReq := &ticketv1.IssueTicketRequest{
		GameCode:        req.GameCode,
		GameScheduleId:  req.GameScheduleID,
		DrawNumber:      req.DrawNumber,
		SelectedNumbers: req.SelectedNumbers,
		BetLines:        protoBetLines,
		IssuerType:      req.IssuerType,
		IssuerId:        req.IssuerID,
		IssuerDetails:   protoIssuerDetails,
		CustomerPhone:   req.CustomerPhone,
		CustomerEmail:   req.CustomerEmail,
		PaymentMethod:   req.PaymentMethod,
	}

	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	result, err := client.IssueTicket(ctx, protoReq)
	if err != nil {
		h.log.Error("Failed to issue ticket", "error", err)
		return handleGRPCError(w, err, "Failed to issue ticket")
	}

	return response.Success(w, http.StatusCreated, "Ticket issued successfully", result)
}

// GetTicket retrieves a ticket by ID
func (h *ticketHandler) GetTicket(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	ticketID := router.GetParam(r, "id")
	if ticketID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Ticket ID is required", nil)
	}

	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	result, err := client.GetTicket(ctx, &ticketv1.GetTicketRequest{TicketId: ticketID})
	if err != nil {
		h.log.Error("Failed to get ticket", "error", err, "ticket_id", ticketID)
		return handleGRPCError(w, err, "Failed to retrieve ticket")
	}

	return response.Success(w, http.StatusOK, "Ticket retrieved successfully", result)
}

// GetTicketBySerial retrieves a ticket by serial number
func (h *ticketHandler) GetTicketBySerial(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	serialNumber := router.GetParam(r, "serial")
	if serialNumber == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_SERIAL", "Serial number is required", nil)
	}

	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	result, err := client.GetTicketBySerial(ctx, &ticketv1.GetTicketBySerialRequest{SerialNumber: serialNumber})
	if err != nil {
		h.log.Error("Failed to get ticket by serial", "error", err, "serial", serialNumber)
		return handleGRPCError(w, err, "Failed to retrieve ticket")
	}

	return response.Success(w, http.StatusOK, "Ticket retrieved successfully", result)
}

// ListTickets retrieves a paginated list of tickets with optional filters
func (h *ticketHandler) ListTickets(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Extract authenticated user ID from JWT context (set by AuthMiddleware)
	userID, ok := ctx.Value(router.ContextUserID).(string)
	if !ok || userID == "" {
		h.log.Error("User ID not found in context", "path", r.URL.Path)
		return response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User authentication required", nil)
	}

	// Parse query parameters manually
	query := r.URL.Query()
	req := ListTicketsRequest{
		Status:     query.Get("status"),
		GameCode:   query.Get("game_code"),
		ScheduleID: query.Get("schedule_id"),
		IssuerType: query.Get("issuer_type"),
		IssuerID:   query.Get("issuer_id"),
		StartDate:  query.Get("start_date"),
		EndDate:    query.Get("end_date"),
	}

	// Security: For retailer endpoints, enforce issuer_id filter to authenticated user
	// Admins can query all tickets or filter by specific issuer_id
	if strings.Contains(r.URL.Path, "/retailer/") {
		req.IssuerID = userID
		h.log.Info("Listing tickets for authenticated retailer", "retailer_id", userID, "path", r.URL.Path)
	} else {
		// Admin endpoint - allow querying all tickets or filtering by issuer_id
		h.log.Info("Listing tickets for admin", "admin_id", userID, "path", r.URL.Path, "issuer_filter", req.IssuerID)
	}

	// Parse pagination parameters
	if pageStr := query.Get("page"); pageStr != "" {
		var page int
		if _, err := fmt.Sscanf(pageStr, "%d", &page); err == nil {
			req.Page = int32(page)
		}
	}
	if pageSizeStr := query.Get("limit"); pageSizeStr != "" {
		var pageSize int
		if _, err := fmt.Sscanf(pageSizeStr, "%d", &pageSize); err == nil {
			req.PageSize = int32(pageSize)
		}
	}

	// Parse draw_number if present
	if drawNumStr := query.Get("draw_number"); drawNumStr != "" {
		var drawNum int
		if _, err := fmt.Sscanf(drawNumStr, "%d", &drawNum); err == nil {
			req.DrawNumber = int32(drawNum)
		}
	}

	// Set default pagination if not provided
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 10
	}

	// Build proto filter
	protoFilter := &ticketv1.TicketFilter{
		Status:         req.Status,
		GameCode:       req.GameCode,
		GameScheduleId: req.ScheduleID,
		DrawNumber:     req.DrawNumber,
		IssuerType:     req.IssuerType,
		IssuerId:       req.IssuerID,
	}

	// Note: Timestamp conversion would be needed for StartDate/EndDate if required

	protoReq := &ticketv1.ListTicketsRequest{
		Filter:   protoFilter,
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	result, err := client.ListTickets(ctx, protoReq)
	if err != nil {
		h.log.Error("Failed to list tickets", "error", err)
		return handleGRPCError(w, err, "Failed to retrieve tickets")
	}

	return response.Success(w, http.StatusOK, "Tickets retrieved successfully", result)
}

// ValidateTicket marks a ticket as validated and checks for winnings
func (h *ticketHandler) ValidateTicket(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	ticketID := router.GetParam(r, "id")
	if ticketID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Ticket ID is required", nil)
	}

	var req ValidateTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	protoReq := &ticketv1.ValidateTicketRequest{
		TicketId:      ticketID,
		ValidatorId:   req.ValidatorID,
		ValidatorType: req.ValidatorType,
	}

	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	result, err := client.ValidateTicket(ctx, protoReq)
	if err != nil {
		h.log.Error("Failed to validate ticket", "error", err, "ticket_id", ticketID)
		return handleGRPCError(w, err, "Failed to validate ticket")
	}

	return response.Success(w, http.StatusOK, "Ticket validated successfully", result)
}

// CancelTicket cancels a ticket before validation
func (h *ticketHandler) CancelTicket(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	ticketID := router.GetParam(r, "id")
	if ticketID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Ticket ID is required", nil)
	}

	var req CancelTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	protoReq := &ticketv1.CancelTicketRequest{
		TicketId:    ticketID,
		Reason:      req.Reason,
		CancelledBy: req.CancelledBy,
	}

	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	result, err := client.CancelTicket(ctx, protoReq)
	if err != nil {
		h.log.Error("Failed to cancel ticket", "error", err, "ticket_id", ticketID)
		return handleGRPCError(w, err, "Failed to cancel ticket")
	}

	return response.Success(w, http.StatusOK, "Ticket cancelled successfully", result)
}

// VoidTicket voids a ticket (administrative action)
func (h *ticketHandler) VoidTicket(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	ticketID := router.GetParam(r, "id")
	if ticketID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Ticket ID is required", nil)
	}

	var req VoidTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	protoReq := &ticketv1.VoidTicketRequest{
		TicketId: ticketID,
		Reason:   req.Reason,
		VoidedBy: req.VoidedBy,
	}

	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	result, err := client.VoidTicket(ctx, protoReq)
	if err != nil {
		h.log.Error("Failed to void ticket", "error", err, "ticket_id", ticketID)
		return handleGRPCError(w, err, "Failed to void ticket")
	}

	return response.Success(w, http.StatusOK, "Ticket voided successfully", result)
}

// ReprintTicket generates a reprint of an existing ticket
func (h *ticketHandler) ReprintTicket(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	ticketID := router.GetParam(r, "id")
	if ticketID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Ticket ID is required", nil)
	}

	var req ReprintTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	protoReq := &ticketv1.ReprintTicketRequest{
		TicketId:    ticketID,
		ReprintedBy: req.ReprintedBy,
		TerminalId:  req.TerminalID,
	}

	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	result, err := client.ReprintTicket(ctx, protoReq)
	if err != nil {
		h.log.Error("Failed to reprint ticket", "error", err, "ticket_id", ticketID)
		return handleGRPCError(w, err, "Failed to reprint ticket")
	}

	return response.Success(w, http.StatusOK, "Ticket reprinted successfully", result)
}

// CheckWinnings checks if a ticket has winnings without validating
func (h *ticketHandler) CheckWinnings(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	ticketID := router.GetParam(r, "id")
	if ticketID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_ID", "Ticket ID is required", nil)
	}

	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	result, err := client.CheckWinnings(ctx, &ticketv1.CheckWinningsRequest{TicketId: ticketID})
	if err != nil {
		h.log.Error("Failed to check winnings", "error", err, "ticket_id", ticketID)
		return handleGRPCError(w, err, "Failed to check winnings")
	}

	return response.Success(w, http.StatusOK, "Winnings checked successfully", result)
}

// Helper functions

// handleGRPCError converts gRPC errors to HTTP errors
func handleGRPCError(w http.ResponseWriter, err error, msg string) error {
	st, ok := status.FromError(err)
	if !ok {
		return response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", msg, nil)
	}

	switch st.Code() {
	case codes.NotFound:
		return response.Error(w, http.StatusNotFound, "NOT_FOUND", st.Message(), nil)
	case codes.InvalidArgument:
		return response.Error(w, http.StatusBadRequest, "INVALID_ARGUMENT", st.Message(), nil)
	case codes.PermissionDenied:
		return response.Error(w, http.StatusForbidden, "PERMISSION_DENIED", st.Message(), nil)
	case codes.Unauthenticated:
		return response.Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", st.Message(), nil)
	case codes.AlreadyExists:
		return response.Error(w, http.StatusConflict, "ALREADY_EXISTS", st.Message(), nil)
	default:
		return response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", st.Message(), nil)
	}
}

// stringPtrValue returns the value of a string pointer or empty string if nil
func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// GetDailyMetrics retrieves daily metrics for dashboard analytics
func (h *ticketHandler) GetDailyMetrics(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Parse query parameters
	date := r.URL.Query().Get("date")
	includeComparison := r.URL.Query().Get("compare_with_previous") != "false" // default true

	// Build proto request
	protoReq := &ticketv1.GetDailyMetricsRequest{
		Date:              date,
		IncludeComparison: includeComparison,
	}

	// Get ticket service client
	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	// Call Ticket Service gRPC
	ticketResult, err := client.GetDailyMetrics(ctx, protoReq)
	if err != nil {
		h.log.Error("Failed to get daily metrics from ticket service", "error", err, "date", date)
		return handleGRPCError(w, err, "Failed to retrieve daily metrics")
	}

	// Call Wallet Service for commissions
	walletClient, err := h.grpcManager.WalletServiceClient()
	if err != nil {
		h.log.Error("Failed to get wallet service client", "error", err)
		// Continue without commission data rather than failing the entire request
	}

	var commissionsResult *walletv1.GetDailyCommissionsResponse
	if walletClient != nil {
		commissionsReq := &walletv1.GetDailyCommissionsRequest{
			Date:              date,
			IncludeComparison: includeComparison,
		}
		commissionsResult, err = walletClient.GetDailyCommissions(ctx, commissionsReq)
		if err != nil {
			h.log.Error("Failed to get commissions from wallet service", "error", err, "date", date)
			// Continue without commission data rather than failing the entire request
		}
	}

	// Call Agent Management Service for retailer count
	agentMgmtClient, err := h.grpcManager.AgentManagementClient()
	if err != nil {
		h.log.Error("Failed to get agent management service client", "error", err)
		// Continue without retailer count rather than failing the entire request
	}

	var retailerCountResult *agentmgmtv1.GetRetailerCountResponse
	if agentMgmtClient != nil {
		retailerCountReq := &agentmgmtv1.GetRetailerCountRequest{}
		retailerCountResult, err = agentMgmtClient.GetRetailerCount(ctx, retailerCountReq)
		if err != nil {
			h.log.Error("Failed to get retailer count from agent management service", "error", err)
			// Continue without retailer count rather than failing the entire request
		}
	}

	// Transform proto response to JSON-friendly format
	result := ticketResult
	metricsData := map[string]interface{}{
		"date": result.Date,
		"metrics": map[string]interface{}{
			"gross_revenue": map[string]interface{}{
				"amount":              result.GrossRevenue.Amount,
				"amount_ghs":          float64(result.GrossRevenue.Amount) / 100.0,
				"change_percentage":   result.GrossRevenue.ChangePercentage,
				"previous_amount":     result.GrossRevenue.PreviousAmount,
				"previous_amount_ghs": float64(result.GrossRevenue.PreviousAmount) / 100.0,
			},
			"tickets": map[string]interface{}{
				"count":             result.Tickets.Count,
				"change_percentage": result.Tickets.ChangePercentage,
				"previous_count":    result.Tickets.PreviousCount,
			},
			"payouts": map[string]interface{}{
				"amount":              result.Payouts.Amount,
				"amount_ghs":          float64(result.Payouts.Amount) / 100.0,
				"change_percentage":   result.Payouts.ChangePercentage,
				"previous_amount":     result.Payouts.PreviousAmount,
				"previous_amount_ghs": float64(result.Payouts.PreviousAmount) / 100.0,
			},
			"win_rate": map[string]interface{}{
				"percentage":      result.WinRate.Percentage,
				"winning_tickets": result.WinRate.WinningTickets,
				"total_tickets":   result.WinRate.TotalTickets,
			},
			// NEW: Additional metrics for enhanced dashboard
		},
	}

	// NEW: Add additional metrics for enhanced dashboard if available
	if result.Stakes != nil {
		metricsData["metrics"].(map[string]interface{})["stakes"] = map[string]interface{}{
			"count":             result.Stakes.Count,
			"change_percentage": result.Stakes.ChangePercentage,
			"previous_count":    result.Stakes.PreviousCount,
		}
	}

	if result.StakesAmount != nil {
		metricsData["metrics"].(map[string]interface{})["stakes_amount"] = map[string]interface{}{
			"amount":              result.StakesAmount.Amount,
			"amount_ghs":          float64(result.StakesAmount.Amount) / 100.0,
			"change_percentage":   result.StakesAmount.ChangePercentage,
			"previous_amount":     result.StakesAmount.PreviousAmount,
			"previous_amount_ghs": float64(result.StakesAmount.PreviousAmount) / 100.0,
		}
	}

	if result.PaidTickets != nil {
		metricsData["metrics"].(map[string]interface{})["paid_tickets"] = map[string]interface{}{
			"count":             result.PaidTickets.Count,
			"change_percentage": result.PaidTickets.ChangePercentage,
			"previous_count":    result.PaidTickets.PreviousCount,
		}
	}

	if result.PaymentsAmount != nil {
		metricsData["metrics"].(map[string]interface{})["payments_amount"] = map[string]interface{}{
			"amount":              result.PaymentsAmount.Amount,
			"amount_ghs":          float64(result.PaymentsAmount.Amount) / 100.0,
			"change_percentage":   result.PaymentsAmount.ChangePercentage,
			"previous_amount":     result.PaymentsAmount.PreviousAmount,
			"previous_amount_ghs": float64(result.PaymentsAmount.PreviousAmount) / 100.0,
		}
	}

	if result.UnpaidTickets != nil {
		metricsData["metrics"].(map[string]interface{})["unpaid_tickets"] = map[string]interface{}{
			"count":             result.UnpaidTickets.Count,
			"change_percentage": result.UnpaidTickets.ChangePercentage,
			"previous_count":    result.UnpaidTickets.PreviousCount,
		}
	}

	if result.UnpaidAmount != nil {
		metricsData["metrics"].(map[string]interface{})["unpaid_amount"] = map[string]interface{}{
			"amount":              result.UnpaidAmount.Amount,
			"amount_ghs":          float64(result.UnpaidAmount.Amount) / 100.0,
			"change_percentage":   result.UnpaidAmount.ChangePercentage,
			"previous_amount":     result.UnpaidAmount.PreviousAmount,
			"previous_amount_ghs": float64(result.UnpaidAmount.PreviousAmount) / 100.0,
		}
	}

	// Add commissions if available
	if commissionsResult != nil && commissionsResult.Commissions != nil {
		metricsData["metrics"].(map[string]interface{})["commissions"] = map[string]interface{}{
			"amount":              commissionsResult.Commissions.Amount,
			"amount_ghs":          float64(commissionsResult.Commissions.Amount) / 100.0,
			"change_percentage":   commissionsResult.Commissions.ChangePercentage,
			"previous_amount":     commissionsResult.Commissions.PreviousAmount,
			"previous_amount_ghs": float64(commissionsResult.Commissions.PreviousAmount) / 100.0,
		}
	}

	// Add retailer count if available
	if retailerCountResult != nil {
		metricsData["metrics"].(map[string]interface{})["retailers"] = map[string]interface{}{
			"count": retailerCountResult.Count,
		}
	}

	return response.Success(w, http.StatusOK, "Daily metrics retrieved successfully", metricsData)
}

// GetMonthlyMetrics retrieves monthly aggregated metrics for charts
func (h *ticketHandler) GetMonthlyMetrics(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Parse query parameters
	months := int32(6) // default to 6 months
	if monthsParam := r.URL.Query().Get("months"); monthsParam != "" {
		// Convert string to int32
		var monthsInt int
		if _, err := fmt.Sscanf(monthsParam, "%d", &monthsInt); err == nil {
			months = int32(monthsInt)
		}
	}

	// Build proto request
	protoReq := &ticketv1.GetMonthlyMetricsRequest{
		Months: months,
	}

	// Get ticket service client
	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	// Call gRPC service
	result, err := client.GetMonthlyMetrics(ctx, protoReq)
	if err != nil {
		h.log.Error("Failed to get monthly metrics", "error", err, "months", months)
		return handleGRPCError(w, err, "Failed to retrieve monthly metrics")
	}

	// Transform proto response to JSON-friendly format
	data := make([]map[string]interface{}, 0, len(result.Data))
	for _, m := range result.Data {
		data = append(data, map[string]interface{}{
			"month":       m.Month,
			"year":        m.Year,
			"revenue":     m.Revenue,
			"revenue_ghs": m.RevenueGhs,
			"tickets":     m.Tickets,
			"payouts":     m.Payouts,
			"payouts_ghs": m.PayoutsGhs,
		})
	}

	return response.Success(w, http.StatusOK, "Monthly metrics retrieved successfully", map[string]interface{}{
		"data": data,
	})
}

// GetTopPerformingAgents retrieves top performing agents by revenue
func (h *ticketHandler) GetTopPerformingAgents(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Parse query parameters
	date := r.URL.Query().Get("date")     // YYYY-MM-DD format
	period := r.URL.Query().Get("period") // daily, monthly, yearly
	limit := int32(10)                    // default to 10

	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		var limitInt int
		if _, err := fmt.Sscanf(limitParam, "%d", &limitInt); err == nil {
			limit = int32(limitInt)
		}
	}

	// Build proto request
	protoReq := &ticketv1.GetTopPerformingAgentsRequest{
		Date:   date,
		Period: period,
		Limit:  limit,
	}

	// Get ticket service client
	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	// Call gRPC service
	result, err := client.GetTopPerformingAgents(ctx, protoReq)
	if err != nil {
		h.log.Error("Failed to get top performing agents", "error", err, "period", period, "date", date)
		return handleGRPCError(w, err, "Failed to retrieve top performing agents")
	}

	// Transform proto response to JSON-friendly format
	agents := make([]map[string]interface{}, 0, len(result.Agents))
	for _, agent := range result.Agents {
		agents = append(agents, map[string]interface{}{
			"id":             agent.AgentId,
			"agent_code":     agent.AgentCode,
			"name":           agent.AgentName,
			"revenue":        agent.TotalRevenueGhs,
			"tickets":        agent.TotalTickets,
			"retailer_count": agent.RetailerCount,
		})
	}

	return response.Success(w, http.StatusOK, "Top performing agents retrieved successfully", map[string]interface{}{
		"agents": agents,
		"period": result.Period,
		"date":   result.Date,
	})
}

// Player-specific ticket handlers

// IssueTicketForPlayer creates a new lottery ticket for a player using their wallet balance
func (h *ticketHandler) IssueTicketForPlayer(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_PLAYER_ID", "Player ID is required", nil)
	}

	authenticatedUserID, ok := ctx.Value(router.ContextUserID).(string)
	if !ok || authenticatedUserID != playerID {
		h.log.Warn("Player ID mismatch", "url_player_id", playerID, "authenticated_user_id", authenticatedUserID)
		return response.Error(w, http.StatusForbidden, "FORBIDDEN", "Access denied", nil)
	}

	var req IssueTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err)
	}

	var protoBetLines []*ticketv1.BetLine
	for _, bl := range req.BetLines {
		protoBetLine := &ticketv1.BetLine{
			LineNumber:      bl.LineNumber,
			BetType:         bl.BetType,
			SelectedNumbers: bl.SelectedNumbers,
			Banker:          bl.Banker,
			Opposed:         bl.Opposed,
			TotalAmount:     bl.TotalAmount,
		}

		if bl.NumberOfCombinations != nil {
			protoBetLine.NumberOfCombinations = *bl.NumberOfCombinations
		}
		if bl.AmountPerCombination != nil {
			protoBetLine.AmountPerCombination = *bl.AmountPerCombination
		}

		protoBetLines = append(protoBetLines, protoBetLine)
	}

	var protoIssuerDetails *ticketv1.IssuerDetails
	if req.IssuerDetails != nil {
		protoIssuerDetails = &ticketv1.IssuerDetails{
			TerminalId:   stringPtrValue(req.IssuerDetails.TerminalID),
			RetailerCode: stringPtrValue(req.IssuerDetails.RetailerCode),
			PlayerId:     stringPtrValue(req.IssuerDetails.PlayerID),
			Location:     stringPtrValue(req.IssuerDetails.Location),
		}
	}

	protoReq := &ticketv1.IssueTicketRequest{
		GameCode:        req.GameCode,
		GameScheduleId:  req.GameScheduleID,
		DrawNumber:      req.DrawNumber,
		SelectedNumbers: req.SelectedNumbers,
		BetLines:        protoBetLines,
		IssuerType:      "player",
		IssuerId:        playerID,
		IssuerDetails:   protoIssuerDetails,
		CustomerPhone:   req.CustomerPhone,
		CustomerEmail:   req.CustomerEmail,
		PaymentMethod:   "wallet",
	}

	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	result, err := client.IssueTicket(ctx, protoReq)
	if err != nil {
		h.log.Error("Failed to issue ticket for player", "error", err, "player_id", playerID)
		return handleGRPCError(w, err, "Failed to issue ticket")
	}

	return response.Success(w, http.StatusCreated, "Ticket issued successfully", result)
}

// ListPlayerTickets retrieves a paginated list of tickets for a specific player
func (h *ticketHandler) ListPlayerTickets(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_PLAYER_ID", "Player ID is required", nil)
	}

	authenticatedUserID, ok := ctx.Value(router.ContextUserID).(string)
	if !ok || authenticatedUserID != playerID {
		h.log.Warn("Player ID mismatch", "url_player_id", playerID, "authenticated_user_id", authenticatedUserID)
		return response.Error(w, http.StatusForbidden, "FORBIDDEN", "Access denied", nil)
	}

	query := r.URL.Query()
	req := ListTicketsRequest{
		Status:     query.Get("status"),
		GameCode:   query.Get("game_code"),
		ScheduleID: query.Get("schedule_id"),
		IssuerType: "player",
		IssuerID:   playerID,
		StartDate:  query.Get("start_date"),
		EndDate:    query.Get("end_date"),
	}

	if pageStr := query.Get("page"); pageStr != "" {
		var page int
		if _, err := fmt.Sscanf(pageStr, "%d", &page); err == nil {
			req.Page = int32(page)
		}
	}
	if pageSizeStr := query.Get("limit"); pageSizeStr != "" {
		var pageSize int
		if _, err := fmt.Sscanf(pageSizeStr, "%d", &pageSize); err == nil {
			req.PageSize = int32(pageSize)
		}
	}

	if drawNumStr := query.Get("draw_number"); drawNumStr != "" {
		var drawNum int
		if _, err := fmt.Sscanf(drawNumStr, "%d", &drawNum); err == nil {
			req.DrawNumber = int32(drawNum)
		}
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 10
	}

	protoFilter := &ticketv1.TicketFilter{
		Status:         req.Status,
		GameCode:       req.GameCode,
		GameScheduleId: req.ScheduleID,
		DrawNumber:     req.DrawNumber,
		IssuerType:     "player",
		IssuerId:       playerID,
	}

	protoReq := &ticketv1.ListTicketsRequest{
		Filter:   protoFilter,
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	result, err := client.ListTickets(ctx, protoReq)
	if err != nil {
		h.log.Error("Failed to list player tickets", "error", err, "player_id", playerID)
		return handleGRPCError(w, err, "Failed to retrieve tickets")
	}

	// Also fetch USSD tickets linked by phone (for players who bought via *899*92#)
	if ussdTickets := h.fetchUSSDTickets(ctx, playerID, client); len(ussdTickets) > 0 {
		seen := make(map[string]bool, len(result.Tickets))
		for _, t := range result.Tickets {
			seen[t.Id] = true
		}
		for _, t := range ussdTickets {
			if !seen[t.Id] {
				result.Tickets = append(result.Tickets, t)
				result.Total++
			}
		}
	}

	return response.Success(w, http.StatusOK, "Player tickets retrieved successfully", result)
}

// fetchUSSDTickets looks up the player's phone and returns any tickets bought via USSD or
// uploaded by admin on their behalf. Returns nil silently on any error.
func (h *ticketHandler) fetchUSSDTickets(ctx context.Context, playerID string, ticketClient ticketv1.TicketServiceClient) []*ticketv1.Ticket {
	playerClient, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		return nil
	}
	profile, err := playerClient.GetProfile(ctx, &playerv1.GetProfileRequest{PlayerId: playerID})
	if err != nil || profile.PhoneNumber == "" {
		return nil
	}
	phone := strings.TrimPrefix(profile.PhoneNumber, "+")

	// Build all phone variants so we match regardless of how customer_phone was stored
	phoneSet := map[string]bool{phone: true}
	var base9 string
	if strings.HasPrefix(phone, "233") && len(phone) > 3 {
		base9 = phone[3:]
	} else if strings.HasPrefix(phone, "0") && len(phone) > 1 {
		base9 = phone[1:]
	} else {
		base9 = phone
	}
	phoneSet["0"+base9] = true
	phoneSet["233"+base9] = true
	phoneSet[base9] = true

	// Fetch all completed tickets and match by customer_phone — covers USSD, admin-uploaded, external
	resp, err := ticketClient.ListTickets(ctx, &ticketv1.ListTicketsRequest{
		Filter:   &ticketv1.TicketFilter{PaymentStatus: "completed"},
		Page:     1,
		PageSize: 500,
	})
	if err != nil {
		return nil
	}
	var matched []*ticketv1.Ticket
	for _, t := range resp.Tickets {
		tp := strings.TrimPrefix(t.CustomerPhone, "+")
		if phoneSet[tp] {
			matched = append(matched, t)
		}
	}
	return matched
}

// GetPlayerTicket retrieves a specific ticket for a player with ownership validation
func (h *ticketHandler) GetPlayerTicket(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Extract player ID and ticket ID from URL parameters
	playerID := router.GetParam(r, "id")
	ticketID := router.GetParam(r, "ticket_id")
	if playerID == "" || ticketID == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_PARAMS", "Player ID and Ticket ID are required", nil)
	}

	// Validate player ID matches authenticated user
	authenticatedUserID, ok := ctx.Value(router.ContextUserID).(string)
	if !ok || authenticatedUserID != playerID {
		h.log.Warn("Player ID mismatch", "url_player_id", playerID, "authenticated_user_id", authenticatedUserID)
		return response.Error(w, http.StatusForbidden, "FORBIDDEN", "Access denied", nil)
	}

	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	result, err := client.GetTicket(ctx, &ticketv1.GetTicketRequest{TicketId: ticketID})
	if err != nil {
		h.log.Error("Failed to get ticket", "error", err, "ticket_id", ticketID, "player_id", playerID)
		return handleGRPCError(w, err, "Failed to retrieve ticket")
	}

	if result.Ticket.IssuerType != "player" || result.Ticket.IssuerId != playerID {
		h.log.Warn("Ticket ownership mismatch",
			"ticket_id", ticketID,
			"player_id", playerID,
			"ticket_issuer_type", result.Ticket.IssuerType,
			"ticket_issuer_id", result.Ticket.IssuerId)
		return response.Error(w, http.StatusForbidden, "FORBIDDEN", "Ticket does not belong to player", nil)
	}

	return response.Success(w, http.StatusOK, "Ticket retrieved successfully", result)
}

// GetPlayerTicketBySerial retrieves a ticket by serial number for a player with ownership validation
func (h *ticketHandler) GetPlayerTicketBySerial(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Extract player ID and serial number from URL parameters
	playerID := router.GetParam(r, "id")
	serialNumber := router.GetParam(r, "serial")
	if playerID == "" || serialNumber == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_PARAMS", "Player ID and Serial number are required", nil)
	}

	// Validate player ID matches authenticated user
	authenticatedUserID, ok := ctx.Value(router.ContextUserID).(string)
	if !ok || authenticatedUserID != playerID {
		h.log.Warn("Player ID mismatch", "url_player_id", playerID, "authenticated_user_id", authenticatedUserID)
		return response.Error(w, http.StatusForbidden, "FORBIDDEN", "Access denied", nil)
	}

	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket service client", "error", err)
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	result, err := client.GetTicketBySerial(ctx, &ticketv1.GetTicketBySerialRequest{SerialNumber: serialNumber})
	if err != nil {
		h.log.Error("Failed to get ticket by serial", "error", err, "serial", serialNumber, "player_id", playerID)
		return handleGRPCError(w, err, "Failed to retrieve ticket")
	}

	// Verify ticket ownership
	if result.Ticket.IssuerType != "player" || result.Ticket.IssuerId != playerID {
		h.log.Warn("Ticket ownership mismatch",
			"serial", serialNumber,
			"player_id", playerID,
			"ticket_issuer_type", result.Ticket.IssuerType,
			"ticket_issuer_id", result.Ticket.IssuerId)
		return response.Error(w, http.StatusForbidden, "FORBIDDEN", "Ticket does not belong to player", nil)
	}

	return response.Success(w, http.StatusOK, "Ticket retrieved successfully", result)
}

// GetTicketsByPhone returns completed tickets for a given phone number.
// Used by admin-uploaded ticket holders who have no player account.
// Route: GET /api/v1/public/tickets/by-phone/{phone}
func (h *ticketHandler) GetTicketsByPhone(w http.ResponseWriter, r *http.Request) error {
	phone := router.GetParam(r, "phone")
	if phone == "" {
		return response.Error(w, http.StatusBadRequest, "MISSING_PHONE", "phone is required", nil)
	}

	// Normalise: strip leading + so it matches DB format
	phone = strings.TrimPrefix(phone, "+")
	// Build all possible formats to match against DB
	// Input could be: 256826832, 233256826832, 0256826832
	var phoneVariants []string
	phoneVariants = append(phoneVariants, phone)
	if strings.HasPrefix(phone, "233") {
		// 233XXXXXXXXX → 0XXXXXXXXX
		phoneVariants = append(phoneVariants, "0"+phone[3:])
	} else if strings.HasPrefix(phone, "0") {
		// 0XXXXXXXXX → 233XXXXXXXXX
		phoneVariants = append(phoneVariants, "233"+phone[1:])
	} else {
		// bare 9-digit: try both 0XXXXXXXXX and 233XXXXXXXXX
		phoneVariants = append(phoneVariants, "0"+phone)
		phoneVariants = append(phoneVariants, "233"+phone)
	}

	ctx := r.Context()
	client, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		return response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ticket service unavailable", nil)
	}

	paymentStatus := "completed"
	result, err := client.ListTickets(ctx, &ticketv1.ListTicketsRequest{
		Filter: &ticketv1.TicketFilter{
			PaymentStatus: paymentStatus,
		},
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		return handleGRPCError(w, err, "Failed to retrieve tickets")
	}

	// Filter by phone in Go — match any of the phone variants
	phoneSet := make(map[string]bool)
	for _, v := range phoneVariants {
		phoneSet[v] = true
	}
	var matched []*ticketv1.Ticket
	for _, t := range result.Tickets {
		tp := strings.TrimPrefix(t.CustomerPhone, "+")
		if phoneSet[tp] || phoneSet[t.CustomerPhone] {
			matched = append(matched, t)
		}
	}

	return response.Success(w, http.StatusOK, "Tickets retrieved successfully", map[string]interface{}{
		"tickets": matched,
		"total":   len(matched),
	})
}

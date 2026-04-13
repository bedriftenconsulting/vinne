package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	walletpb "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// WalletHandler defines the interface for wallet-related handlers
type WalletHandler interface {
	GetAgentWalletBalance(w http.ResponseWriter, r *http.Request) error
	GetRetailerStakeBalance(w http.ResponseWriter, r *http.Request) error
	GetRetailerWinningBalance(w http.ResponseWriter, r *http.Request) error
	GetAgentTransactionHistory(w http.ResponseWriter, r *http.Request) error
	GetRetailerTransactionHistory(w http.ResponseWriter, r *http.Request) error
	GetAllTransactions(w http.ResponseWriter, r *http.Request) error
	CreditAgentWallet(w http.ResponseWriter, r *http.Request) error
	CreditRetailerWallet(w http.ResponseWriter, r *http.Request) error
	SetCommissionRate(w http.ResponseWriter, r *http.Request) error
	GetCommissionRate(w http.ResponseWriter, r *http.Request) error
	ReverseTransaction(w http.ResponseWriter, r *http.Request) error
	GetHoldOnWallet(w http.ResponseWriter, r *http.Request) error
	ReleaseHoldOnWallet(w http.ResponseWriter, r *http.Request) error
	PlaceHoldOnWallet(w http.ResponseWriter, r *http.Request) error
	GetHoldByRetailer(w http.ResponseWriter, r *http.Request) error
}

// walletHandlerImpl handles wallet-related HTTP requests
type walletHandlerImpl struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
}

// NewWalletHandler creates a new wallet handler
func NewWalletHandler(grpcManager *grpc.ClientManager, log logger.Logger) WalletHandler {
	return &walletHandlerImpl{
		grpcManager: grpcManager,
		log:         log,
	}
}

// GetAgentWalletBalance handles GET /api/v1/agents/{agentId}/wallet/balance
func (h *walletHandlerImpl) GetAgentWalletBalance(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "GetAgentWalletBalance")
	defer span.End()

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	agentID := router.GetParam(r, "agentId")

	span.SetAttributes(
		attribute.String("agent.id", agentID),
		attribute.String("http.method", r.Method),
		attribute.String("http.path", r.URL.Path),
	)

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Call wallet service
	req := &walletpb.GetAgentWalletBalanceRequest{
		AgentId: agentID,
	}

	resp, err := client.GetAgentWalletBalance(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get wallet balance")
		h.log.Error("Failed to get wallet balance", "error", err, "agent_id", agentID)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve wallet balance")
	}

	// Convert response to JSON
	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"agent_id":          resp.AgentId,
			"balance":           resp.Balance,
			"pending_balance":   resp.PendingBalance,
			"available_balance": resp.AvailableBalance,
			"currency":          "GHS",
			"last_updated":      resp.LastUpdated.AsTime().Format(time.RFC3339),
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// GetRetailerStakeBalance handles GET /api/v1/retailers/{retailerId}/wallet/stake/balance
func (h *walletHandlerImpl) GetRetailerStakeBalance(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "GetRetailerStakeBalance")
	defer span.End()

	retailerID := router.GetParam(r, "retailerId")

	span.SetAttributes(
		attribute.String("retailer.id", retailerID),
		attribute.String("wallet.type", "stake"),
		attribute.String("http.method", r.Method),
		attribute.String("http.path", r.URL.Path),
	)

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Call wallet service
	req := &walletpb.GetRetailerWalletBalanceRequest{
		RetailerId: retailerID,
		WalletType: walletpb.WalletType_RETAILER_STAKE,
	}

	resp, err := client.GetRetailerWalletBalance(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get stake wallet balance")
		h.log.Error("Failed to get stake wallet balance", "error", err, "retailer_id", retailerID)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve stake wallet balance")
	}

	// Convert response to JSON
	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"retailer_id":       resp.RetailerId,
			"wallet_type":       "stake",
			"balance":           resp.Balance,
			"pending_balance":   resp.PendingBalance,
			"available_balance": resp.AvailableBalance,
			"currency":          "GHS",
			"last_updated":      resp.LastUpdated.AsTime().Format(time.RFC3339),
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// GetRetailerWinningBalance handles GET /api/v1/retailers/{retailerId}/wallet/winning/balance
func (h *walletHandlerImpl) GetRetailerWinningBalance(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "GetRetailerWinningBalance")
	defer span.End()

	retailerID := router.GetParam(r, "retailerId")

	span.SetAttributes(
		attribute.String("retailer.id", retailerID),
		attribute.String("wallet.type", "winning"),
		attribute.String("http.method", r.Method),
		attribute.String("http.path", r.URL.Path),
	)

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Call wallet service
	req := &walletpb.GetRetailerWalletBalanceRequest{
		RetailerId: retailerID,
		WalletType: walletpb.WalletType_RETAILER_WINNING,
	}

	resp, err := client.GetRetailerWalletBalance(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get winning wallet balance")
		h.log.Error("Failed to get winning wallet balance", "error", err, "retailer_id", retailerID)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve winning wallet balance")
	}

	// Convert response to JSON
	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"retailer_id":       resp.RetailerId,
			"wallet_type":       "winning",
			"balance":           resp.Balance,
			"pending_balance":   resp.PendingBalance,
			"available_balance": resp.AvailableBalance,
			"currency":          "GHS",
			"last_updated":      resp.LastUpdated.AsTime().Format(time.RFC3339),
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// GetAgentTransactionHistory handles GET /api/v1/agents/{agentId}/wallet/transactions
func (h *walletHandlerImpl) GetAgentTransactionHistory(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "GetAgentTransactionHistory")
	defer span.End()

	agentID := router.GetParam(r, "agentId")

	// Parse query parameters
	page := 1
	pageSize := 20

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	span.SetAttributes(
		attribute.String("agent.id", agentID),
		attribute.Int("page", page),
		attribute.Int("page_size", pageSize),
	)

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Call wallet service
	req := &walletpb.GetTransactionHistoryRequest{
		WalletOwnerId: agentID,
		WalletType:    walletpb.WalletType_AGENT_STAKE,
		Page:          int32(page),
		PageSize:      int32(pageSize),
	}

	resp, err := client.GetTransactionHistory(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get transaction history")
		h.log.Warn("Failed to get transaction history, returning empty array", "error", err, "agent_id", agentID)

		// Return empty array when wallet service is unavailable or no data exists
		emptyResponse := map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"transactions": []map[string]interface{}{},
				"pagination": map[string]interface{}{
					"page":      page,
					"page_size": pageSize,
					"total":     0,
					"has_more":  false,
				},
			},
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		return router.WriteJSON(w, http.StatusOK, emptyResponse)
	}

	// Convert transactions to JSON format
	transactions := make([]map[string]interface{}, 0, len(resp.Transactions))
	for _, tx := range resp.Transactions {
		transactions = append(transactions, map[string]interface{}{
			"id":             tx.Id,
			"type":           tx.Type.String(),
			"amount":         tx.Amount,
			"balance_before": tx.BalanceBefore,
			"balance_after":  tx.BalanceAfter,
			"reference":      tx.Reference,
			"description":    tx.Description,
			"status":         tx.Status.String(),
			"created_at":     tx.CreatedAt.AsTime().Format(time.RFC3339),
			"metadata":       tx.Metadata,
		})
	}

	// Convert response to JSON
	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"transactions": transactions,
			"pagination": map[string]interface{}{
				"page":      resp.Page,
				"page_size": resp.PageSize,
				"total":     resp.TotalCount,
				"has_more":  resp.HasMore,
			},
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// GetRetailerTransactionHistory handles GET /api/v1/retailers/{retailerId}/wallet/transactions
func (h *walletHandlerImpl) GetRetailerTransactionHistory(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "GetRetailerTransactionHistory")
	defer span.End()

	retailerID := router.GetParam(r, "retailerId")

	// Parse wallet type from query param
	walletType := r.URL.Query().Get("wallet_type")
	var pbWalletType walletpb.WalletType
	switch walletType {
	case "winning":
		pbWalletType = walletpb.WalletType_RETAILER_WINNING
	default:
		pbWalletType = walletpb.WalletType_RETAILER_STAKE
		walletType = "stake"
	}

	// Parse pagination
	page := 1
	pageSize := 20

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	span.SetAttributes(
		attribute.String("retailer.id", retailerID),
		attribute.String("wallet.type", walletType),
		attribute.Int("page", page),
		attribute.Int("page_size", pageSize),
	)

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Call wallet service
	req := &walletpb.GetTransactionHistoryRequest{
		WalletOwnerId: retailerID,
		WalletType:    pbWalletType,
		Page:          int32(page),
		PageSize:      int32(pageSize),
	}

	resp, err := client.GetTransactionHistory(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get transaction history")
		h.log.Warn("Failed to get transaction history, returning empty array", "error", err, "retailer_id", retailerID)

		// Return empty array when wallet service is unavailable or no data exists
		emptyResponse := map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"wallet_type":  walletType,
				"transactions": []map[string]interface{}{},
				"pagination": map[string]interface{}{
					"page":      page,
					"page_size": pageSize,
					"total":     0,
					"has_more":  false,
				},
			},
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		return router.WriteJSON(w, http.StatusOK, emptyResponse)
	}

	// Convert transactions to JSON format
	transactions := make([]map[string]interface{}, 0, len(resp.Transactions))
	for _, tx := range resp.Transactions {
		transactions = append(transactions, map[string]interface{}{
			"id":             tx.Id,
			"type":           tx.Type.String(),
			"amount":         tx.Amount,
			"balance_before": tx.BalanceBefore,
			"balance_after":  tx.BalanceAfter,
			"reference":      tx.Reference,
			"description":    tx.Description,
			"status":         tx.Status.String(),
			"created_at":     tx.CreatedAt.AsTime().Format(time.RFC3339),
			"metadata":       tx.Metadata,
		})
	}

	// Convert response to JSON
	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"wallet_type":  walletType,
			"transactions": transactions,
			"pagination": map[string]interface{}{
				"page":      resp.Page,
				"page_size": resp.PageSize,
				"total":     resp.TotalCount,
				"has_more":  resp.HasMore,
			},
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// GetAllTransactions handles GET /api/v1/admin/wallet/transactions
func (h *walletHandlerImpl) GetAllTransactions(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "GetAllTransactions")
	defer span.End()

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Parse query parameters
	query := r.URL.Query()

	// Parse pagination
	page := 1
	pageSize := 20

	if pageStr := query.Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := query.Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// Parse sorting
	sortBy := query.Get("sort_by")
	if sortBy == "" {
		sortBy = "created_at"
	}

	sortOrder := query.Get("sort_order")
	if sortOrder == "" {
		sortOrder = "desc"
	}

	// Parse transaction types filter (multiple values)
	var transactionTypes []walletpb.TransactionType
	if txTypes := query["transaction_types[]"]; len(txTypes) > 0 {
		for _, txType := range txTypes {
			switch txType {
			case "CREDIT":
				transactionTypes = append(transactionTypes, walletpb.TransactionType_CREDIT)
			case "DEBIT":
				transactionTypes = append(transactionTypes, walletpb.TransactionType_DEBIT)
			case "TRANSFER":
				transactionTypes = append(transactionTypes, walletpb.TransactionType_TRANSFER)
			case "COMMISSION":
				transactionTypes = append(transactionTypes, walletpb.TransactionType_COMMISSION)
			case "PAYOUT":
				transactionTypes = append(transactionTypes, walletpb.TransactionType_PAYOUT)
			}
		}
	}

	// Parse wallet types filter (multiple values)
	var walletTypes []walletpb.WalletType
	if wTypes := query["wallet_types[]"]; len(wTypes) > 0 {
		for _, wType := range wTypes {
			switch wType {
			case "AGENT_STAKE":
				walletTypes = append(walletTypes, walletpb.WalletType_AGENT_STAKE)
			case "RETAILER_STAKE":
				walletTypes = append(walletTypes, walletpb.WalletType_RETAILER_STAKE)
			case "RETAILER_WINNING":
				walletTypes = append(walletTypes, walletpb.WalletType_RETAILER_WINNING)
			}
		}
	}

	// Parse statuses filter (multiple values)
	var statuses []walletpb.TransactionStatus
	if statusFilters := query["statuses[]"]; len(statusFilters) > 0 {
		for _, status := range statusFilters {
			switch status {
			case "PENDING":
				statuses = append(statuses, walletpb.TransactionStatus_PENDING)
			case "COMPLETED":
				statuses = append(statuses, walletpb.TransactionStatus_COMPLETED)
			case "FAILED":
				statuses = append(statuses, walletpb.TransactionStatus_FAILED)
			case "REVERSED":
				statuses = append(statuses, walletpb.TransactionStatus_REVERSED)
			}
		}
	}

	// Parse date filters
	var startDate, endDate *time.Time
	if startDateStr := query.Get("start_date"); startDateStr != "" {
		if t, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			startDate = &t
		}
	}

	if endDateStr := query.Get("end_date"); endDateStr != "" {
		if t, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			endDate = &t
		}
	}

	// Parse search term
	searchTerm := query.Get("search")

	span.SetAttributes(
		attribute.Int("page", page),
		attribute.Int("page_size", pageSize),
		attribute.String("sort_by", sortBy),
		attribute.String("sort_order", sortOrder),
		attribute.Int("filter.transaction_types_count", len(transactionTypes)),
		attribute.Int("filter.wallet_types_count", len(walletTypes)),
		attribute.Int("filter.statuses_count", len(statuses)),
	)

	if searchTerm != "" {
		span.SetAttributes(attribute.String("filter.search_term", searchTerm))
	}

	// Log filter parameters for debugging
	h.log.Debug("Processing transaction filters",
		"start_date", startDate,
		"end_date", endDate,
		"search_term", searchTerm,
		"transaction_types_count", len(transactionTypes),
		"wallet_types_count", len(walletTypes),
		"statuses_count", len(statuses),
	)

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Build gRPC request
	req := &walletpb.GetAllTransactionsRequest{
		TransactionTypes: transactionTypes,
		WalletTypes:      walletTypes,
		Statuses:         statuses,
		SearchTerm:       searchTerm,
		Page:             int32(page),
		PageSize:         int32(pageSize),
		SortBy:           sortBy,
		SortOrder:        sortOrder,
	}

	// Add timestamps if provided
	if startDate != nil {
		req.StartDate = timestamppb.New(*startDate)
	}

	if endDate != nil {
		req.EndDate = timestamppb.New(*endDate)
	}

	// Call wallet service
	resp, err := client.GetAllTransactions(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get all transactions")
		h.log.Error("Failed to get all transactions", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve transactions")
	}

	// Convert transactions to JSON format
	transactions := make([]map[string]interface{}, 0, len(resp.Transactions))
	for _, tx := range resp.Transactions {
		txMap := map[string]interface{}{
			"id":                tx.Id,
			"transaction_id":    tx.TransactionId,
			"wallet_owner_id":   tx.WalletOwnerId,
			"wallet_owner_name": tx.WalletOwnerName,
			"wallet_owner_code": tx.WalletOwnerCode,
			"wallet_type":       tx.WalletType.String(),
			"type":              tx.Type.String(),
			"amount":            tx.Amount / 100,        // Convert pesewas to GHS
			"balance_before":    tx.BalanceBefore / 100, // Convert pesewas to GHS
			"balance_after":     tx.BalanceAfter / 100,  // Convert pesewas to GHS
			"reference":         tx.Reference,
			"description":       tx.Description,
			"status":            tx.Status.String(),
			"created_at":        tx.CreatedAt.AsTime().Format(time.RFC3339),
			"metadata":          tx.Metadata,
		}

		// Add optional timestamp fields
		if tx.CompletedAt != nil {
			txMap["completed_at"] = tx.CompletedAt.AsTime().Format(time.RFC3339)
		}
		if tx.ReversedAt != nil {
			txMap["reversed_at"] = tx.ReversedAt.AsTime().Format(time.RFC3339)
		}

		transactions = append(transactions, txMap)
	}

	// Convert statistics to JSON format (if available)
	var statistics map[string]interface{}
	if resp.Statistics != nil {
		statistics = map[string]interface{}{
			"total_volume":     resp.Statistics.TotalVolume / 100,   // Convert pesewas to GHS
			"total_credits":    resp.Statistics.TotalCredits / 100,  // Convert pesewas to GHS
			"total_debits":     resp.Statistics.TotalDebits / 100,   // Convert pesewas to GHS
			"pending_amount":   resp.Statistics.PendingAmount / 100, // Convert pesewas to GHS
			"pending_count":    resp.Statistics.PendingCount,
			"completed_count":  resp.Statistics.CompletedCount,
			"failed_count":     resp.Statistics.FailedCount,
			"credit_count":     resp.Statistics.CreditCount,
			"debit_count":      resp.Statistics.DebitCount,
			"transfer_count":   resp.Statistics.TransferCount,
			"commission_count": resp.Statistics.CommissionCount,
			"payout_count":     resp.Statistics.PayoutCount,
		}
	}

	// Convert response to JSON
	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"transactions": transactions,
			"pagination": map[string]interface{}{
				"page":        resp.Page,
				"page_size":   resp.PageSize,
				"total":       resp.TotalCount,
				"total_pages": resp.TotalPages,
				"has_more":    resp.HasMore,
			},
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Add statistics to response if available
	if statistics != nil {
		response["data"].(map[string]interface{})["statistics"] = statistics
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// CreditAgentWallet handles POST /api/v1/admin/agents/{agentId}/wallet/credit
func (h *walletHandlerImpl) CreditAgentWallet(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "CreditAgentWallet")
	defer span.End()

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	agentID := router.GetParam(r, "agentId")

	// Parse request body
	var req struct {
		Amount        float64 `json:"amount"`
		CreditType    string  `json:"credit_type"`    // "payment" or "credit_loan"
		Reference     string  `json:"reference"`      // Transaction ID/reference
		PaymentMethod string  `json:"payment_method"` // cash, bank_transfer, mobile_money, etc.
		Notes         string  `json:"notes"`
		CreditedBy    string  `json:"credited_by"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Validate amount
	if req.Amount <= 0 {
		return router.ErrorResponse(w, http.StatusBadRequest, "Amount must be greater than 0")
	}

	// Get user info from context
	userID, userIDOk := r.Context().Value(router.ContextUserID).(string)
	userEmail, userEmailOk := r.Context().Value(router.ContextEmail).(string)
	if !userIDOk || !userEmailOk || userID == "" || userEmail == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
	}

	span.SetAttributes(
		attribute.String("agent.id", agentID),
		attribute.Float64("base_amount", req.Amount),
		attribute.String("credit_type", req.CreditType),
		attribute.String("user.id", userID),
		attribute.String("http.method", r.Method),
		attribute.String("http.path", r.URL.Path),
	)

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Generate idempotency key with UUID for uniqueness
	shortUUID := uuid.New().String()[:8]
	idempotencyKey := agentID + "-" + time.Now().Format("20060102150405") + "-" + shortUUID

	// Convert amount from GHS to pesewas (multiply by 100)
	// This is the BASE amount - wallet service will apply 30% gross-up
	baseAmountInPesewas := req.Amount * 100

	// Call wallet service with BASE amount only
	// Wallet service will calculate: gross_amount = base_amount + (base_amount * 0.30)
	creditReq := &walletpb.CreditAgentWalletRequest{
		AgentId:        agentID,
		Amount:         baseAmountInPesewas,
		Reference:      req.Reference,
		PaymentMethod:  req.PaymentMethod,
		Notes:          req.Notes + " (Credit Type: " + req.CreditType + ", By: " + userEmail + ")",
		IdempotencyKey: idempotencyKey,
	}

	resp, err := client.CreditAgentWallet(ctx, creditReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to credit wallet")
		h.log.Error("Failed to credit agent wallet", "error", err, "agent_id", agentID)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to credit wallet")
	}

	// Convert response to JSON
	// All amounts from proto are in pesewas, convert to GHS for frontend
	response := map[string]interface{}{
		"success": resp.Success,
		"data": map[string]interface{}{
			"transaction_id":    resp.TransactionId,
			"base_amount":       resp.BaseAmount / 100,       // Convert pesewas to GHS
			"commission_amount": resp.CommissionAmount / 100, // Convert pesewas to GHS
			"gross_amount":      resp.GrossAmount / 100,      // Convert pesewas to GHS
			"new_balance":       resp.NewBalance / 100,       // Convert pesewas to GHS
			"currency":          "GHS",
			"credited_at":       resp.Timestamp.AsTime().Format(time.RFC3339),
			"credit_type":       req.CreditType,
			"payment_method":    req.PaymentMethod,
			"reference":         req.Reference,
		},
		"message":   resp.Message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// CreditRetailerWallet handles POST /api/v1/admin/retailers/{retailerId}/wallet/credit
func (h *walletHandlerImpl) CreditRetailerWallet(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "CreditRetailerWallet")
	defer span.End()

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	retailerID := router.GetParam(r, "retailerId")

	// Parse request body
	var req struct {
		Amount        float64 `json:"amount"`
		CreditType    string  `json:"credit_type"`    // "payment" or "credit_loan"
		WalletType    string  `json:"wallet_type"`    // "stake" or "winning" - defaults to "stake"
		Reference     string  `json:"reference"`      // Transaction ID/reference
		PaymentMethod string  `json:"payment_method"` // cash, bank_transfer, mobile_money, etc.
		AgentID       string  `json:"agent_id"`       // If credited by agent
		Notes         string  `json:"notes"`
		CreditedBy    string  `json:"credited_by"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Validate amount
	if req.Amount <= 0 {
		return router.ErrorResponse(w, http.StatusBadRequest, "Amount must be greater than 0")
	}

	// Default wallet type to stake if not specified
	if req.WalletType == "" {
		req.WalletType = "stake"
	}

	// Convert wallet type string to proto enum
	var walletType walletpb.WalletType
	switch req.WalletType {
	case "winning":
		walletType = walletpb.WalletType_RETAILER_WINNING
	default:
		walletType = walletpb.WalletType_RETAILER_STAKE
	}

	// Get user info from context
	userID, userIDOk := r.Context().Value(router.ContextUserID).(string)
	userEmail, userEmailOk := r.Context().Value(router.ContextEmail).(string)
	if !userIDOk || !userEmailOk || userID == "" || userEmail == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
	}

	span.SetAttributes(
		attribute.String("retailer.id", retailerID),
		attribute.Float64("base_amount", req.Amount),
		attribute.String("credit_type", req.CreditType),
		attribute.String("wallet_type", req.WalletType),
		attribute.String("user.id", userID),
		attribute.String("http.method", r.Method),
		attribute.String("http.path", r.URL.Path),
	)

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Generate idempotency key with UUID for uniqueness
	shortUUID := uuid.New().String()[:8]
	idempotencyKey := retailerID + "-" + req.WalletType + "-" + time.Now().Format("20060102150405") + "-" + shortUUID

	// Convert amount from GHS to pesewas (multiply by 100)
	// This is the BASE amount - wallet service will apply 30% gross-up
	baseAmountInPesewas := req.Amount * 100

	// Call wallet service with BASE amount only
	// Wallet service will calculate: gross_amount = base_amount + (base_amount * 0.30)
	creditReq := &walletpb.CreditRetailerWalletRequest{
		RetailerId:     retailerID,
		WalletType:     walletType,
		Amount:         baseAmountInPesewas,
		Reference:      req.Reference,
		AgentId:        req.AgentID,
		Notes:          req.Notes + " (Credit Type: " + req.CreditType + ", By: " + userEmail + ")",
		IdempotencyKey: idempotencyKey,
		CreditSource:   walletpb.CreditSource_CREDIT_SOURCE_ADMIN_DIRECT, // Admin direct credit from dashboard
	}

	resp, err := client.CreditRetailerWallet(ctx, creditReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to credit wallet")
		h.log.Error("Failed to credit retailer wallet", "error", err, "retailer_id", retailerID)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to credit wallet")
	}

	// Convert response to JSON
	// All amounts from proto are in pesewas, convert to GHS for frontend
	response := map[string]interface{}{
		"success": resp.Success,
		"data": map[string]interface{}{
			"transaction_id":    resp.TransactionId,
			"base_amount":       resp.BaseAmount / 100,       // Convert pesewas to GHS
			"commission_amount": resp.CommissionAmount / 100, // Convert pesewas to GHS
			"gross_amount":      resp.GrossAmount / 100,      // Convert pesewas to GHS
			"new_balance":       resp.NewBalance / 100,       // Convert pesewas to GHS
			"currency":          "GHS",
			"wallet_type":       req.WalletType,
			"credited_at":       resp.Timestamp.AsTime().Format(time.RFC3339),
			"credit_type":       req.CreditType,
			"payment_method":    req.PaymentMethod,
			"reference":         req.Reference,
		},
		"message":   resp.Message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// SetCommissionRate handles PUT /api/v1/admin/agents/{agentId}/commission-rate
func (h *walletHandlerImpl) SetCommissionRate(w http.ResponseWriter, r *http.Request) error {
	// TODO: Implement commission rate update
	return router.ErrorResponse(w, http.StatusNotImplemented, "Commission rate update not yet implemented")
}

// GetCommissionRate handles GET /api/v1/admin/agents/{agentId}/commission-rate
func (h *walletHandlerImpl) GetCommissionRate(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "GetCommissionRate")
	defer span.End()

	agentID := router.GetParam(r, "agentId")

	span.SetAttributes(
		attribute.String("agent.id", agentID),
	)

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Call wallet service
	req := &walletpb.GetCommissionRateRequest{
		AgentId: agentID,
	}

	resp, err := client.GetCommissionRate(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get commission rate")
		h.log.Error("Failed to get commission rate", "error", err, "agent_id", agentID)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve commission rate")
	}

	// Convert response to JSON
	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"agent_id":        resp.AgentId,
			"rate":            resp.Rate,
			"rate_percentage": resp.Rate * 100, // Convert decimal to percentage
			"effective_from":  resp.EffectiveFrom.AsTime().Format(time.RFC3339),
			"created_at":      resp.CreatedAt.AsTime().Format(time.RFC3339),
			"created_by":      resp.CreatedBy,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// ReverseTransaction handles POST /api/v1/admin/wallet/transactions/{transactionId}/reverse
func (h *walletHandlerImpl) ReverseTransaction(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "ReverseTransaction")
	defer span.End()

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	transactionID := router.GetParam(r, "transactionId")

	// Parse request body
	var req struct {
		Reason    string `json:"reason"`
		Confirmed bool   `json:"confirmed"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Validate reason
	if req.Reason == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Reason is required")
	}

	if len(req.Reason) < 20 {
		return router.ErrorResponse(w, http.StatusBadRequest, "Reason must be at least 20 characters")
	}

	// Validate confirmation
	if !req.Confirmed {
		return router.ErrorResponse(w, http.StatusBadRequest, "Confirmation is required")
	}

	// Get user info from context (admin user performing reversal)
	userID, userIDOk := r.Context().Value(router.ContextUserID).(string)
	userName, userNameOk := r.Context().Value(router.ContextUsername).(string)
	userEmail, userEmailOk := r.Context().Value(router.ContextEmail).(string)

	if !userIDOk || !userNameOk || !userEmailOk || userID == "" || userName == "" || userEmail == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
	}

	span.SetAttributes(
		attribute.String("transaction.id", transactionID),
		attribute.String("admin.id", userID),
		attribute.String("admin.email", userEmail),
		attribute.String("reason", req.Reason),
	)

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Call wallet service
	reverseReq := &walletpb.ReverseTransactionRequest{
		TransactionId: transactionID,
		Reason:        req.Reason,
		AdminId:       userID,
		AdminName:     userName,
		AdminEmail:    userEmail,
	}

	resp, err := client.ReverseTransaction(ctx, reverseReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to reverse transaction")
		h.log.Error("Failed to reverse transaction",
			"error", err,
			"transaction_id", transactionID,
			"admin_id", userID)

		// Map gRPC errors to HTTP status codes
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			return router.ErrorResponse(w, http.StatusNotFound, "Transaction not found")
		}
		if strings.Contains(errMsg, "only CREDIT") || strings.Contains(errMsg, "only COMPLETED") {
			return router.ErrorResponse(w, http.StatusConflict, err.Error())
		}
		if strings.Contains(errMsg, "already reversed") {
			return router.ErrorResponse(w, http.StatusConflict, "Transaction already reversed")
		}
		if strings.Contains(errMsg, "too old") {
			return router.ErrorResponse(w, http.StatusConflict, "Transaction too old for reversal (24 hour limit exceeded)")
		}

		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to reverse transaction")
	}

	// Convert transactions to JSON format
	var originalTransaction map[string]interface{}
	if resp.OriginalTransaction != nil {
		originalTransaction = map[string]interface{}{
			"id":              resp.OriginalTransaction.Id,
			"transaction_id":  resp.OriginalTransaction.TransactionId,
			"wallet_owner_id": resp.OriginalTransaction.WalletOwnerId,
			"wallet_type":     resp.OriginalTransaction.WalletType,
			"type":            resp.OriginalTransaction.Type,
			"amount":          resp.OriginalTransaction.Amount / 100, // Convert pesewas to GHS
			"balance_before":  resp.OriginalTransaction.BalanceBefore / 100,
			"balance_after":   resp.OriginalTransaction.BalanceAfter / 100,
			"status":          resp.OriginalTransaction.Status,
			"description":     resp.OriginalTransaction.Description,
			"reference":       resp.OriginalTransaction.Reference,
			"created_at":      resp.OriginalTransaction.CreatedAt.AsTime().Format(time.RFC3339),
		}
	}

	var reversalTransaction map[string]interface{}
	if resp.ReversalTransaction != nil {
		reversalTransaction = map[string]interface{}{
			"id":              resp.ReversalTransaction.Id,
			"transaction_id":  resp.ReversalTransaction.TransactionId,
			"wallet_owner_id": resp.ReversalTransaction.WalletOwnerId,
			"wallet_type":     resp.ReversalTransaction.WalletType,
			"type":            resp.ReversalTransaction.Type,
			"amount":          resp.ReversalTransaction.Amount / 100, // Convert pesewas to GHS
			"balance_before":  resp.ReversalTransaction.BalanceBefore / 100,
			"balance_after":   resp.ReversalTransaction.BalanceAfter / 100,
			"status":          resp.ReversalTransaction.Status,
			"description":     resp.ReversalTransaction.Description,
			"reference":       resp.ReversalTransaction.Reference,
			"created_at":      resp.ReversalTransaction.CreatedAt.AsTime().Format(time.RFC3339),
		}
	}

	// Convert response to JSON
	response := map[string]interface{}{
		"success": resp.Success,
		"message": resp.Message,
		"data": map[string]interface{}{
			"reversal_transaction_id": resp.ReversalTransactionId,
			"reversed_amount":         resp.ReversedAmount / 100, // Convert pesewas to GHS
			"new_wallet_balance":      resp.NewWalletBalance / 100,
			"balance_is_negative":     resp.BalanceIsNegative,
			"reversed_at":             resp.ReversedAt.AsTime().Format(time.RFC3339),
			"original_transaction":    originalTransaction,
			"reversal_transaction":    reversalTransaction,
			"reversed_by": map[string]string{
				"admin_id":    userID,
				"admin_name":  userName,
				"admin_email": userEmail,
			},
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	span.SetStatus(codes.Ok, "transaction reversed successfully")

	return router.WriteJSON(w, http.StatusOK, response)
}

func (h *walletHandlerImpl) GetHoldOnWallet(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "GetHoldOnWallet")
	defer span.End()

	holdID := router.GetParam(r, "hold_id")
	if holdID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Hold ID is required")
	}

	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	resp, err := client.GetHoldOnWallet(ctx, &walletpb.GetHoldOnWalletRequest{HoldId: holdID})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get hold on wallet")
		h.log.Error("Failed to get hold on wallet", "error", err, "hold_id", holdID)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to get hold on wallet")
	}

	return router.WriteJSON(w, http.StatusOK, resp)
}

func (h *walletHandlerImpl) ReleaseHoldOnWallet(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "ReleaseHoldOnWallet")
	defer span.End()

	userID, userIDOk := r.Context().Value(router.ContextUserID).(string)
	if !userIDOk || userID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
	}

	holdID := router.GetParam(r, "hold_id")
	if holdID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Hold ID is required")
	}

	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	resp, err := client.ReleaseHoldOnWallet(ctx, &walletpb.ReleaseHoldOnWalletRequest{HoldId: holdID, ReleasedBy: userID})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to release hold on wallet")
		h.log.Error("Failed to release hold on wallet", "error", err, "hold_id", holdID)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to release hold on wallet")
	}

	return router.WriteJSON(w, http.StatusOK, resp)
}

func (h *walletHandlerImpl) PlaceHoldOnWallet(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "PlaceHoldOnWallet")
	defer span.End()

	userID, userIDOk := r.Context().Value(router.ContextUserID).(string)
	if !userIDOk || userID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
	}

	var req struct {
		RetailerID string `json:"retailer_id"`
		Reason     string `json:"reason"`
		ExpiresAt  string `json:"expires_at"`
	}
	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.RetailerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Retailer ID is required")
	}
	if req.Reason == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Reason is required")
	}
	if req.ExpiresAt == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Expires at is required")
	}

	span.SetAttributes(
		attribute.String("retailer.id", req.RetailerID),
		attribute.String("user.id", userID),
		attribute.String("reason", req.Reason),
	)

	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	expiresAt, err := time.Parse("2006-01-02", req.ExpiresAt)
	if err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid expires at")
	}
	resp, err := client.PlaceHoldOnWallet(ctx, &walletpb.PlaceHoldOnWalletRequest{
		RetailerId: req.RetailerID,
		PlacedBy:   userID,
		Reason:     req.Reason,
		ExpiresAt: &timestamppb.Timestamp{
			Seconds: expiresAt.Unix(),
			Nanos:   int32(expiresAt.Nanosecond()),
		},
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to place hold on wallet")
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to place hold on wallet")
	}

	return router.WriteJSON(w, http.StatusOK, resp)
}

func (h *walletHandlerImpl) GetHoldByRetailer(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "GetHoldByRetailer")
	defer span.End()

	retailerID := router.GetParam(r, "retailerId")
	if retailerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Retailer ID is required")
	}

	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	resp, err := client.GetHoldByRetailer(ctx, &walletpb.GetHoldByRetailerRequest{RetailerId: retailerID})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get hold by retailer")
		h.log.Error("Failed to get hold by retailer", "error", err, "retailer_id", retailerID)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to get hold by retailer")
	}

	return router.WriteJSON(w, http.StatusOK, resp)
}

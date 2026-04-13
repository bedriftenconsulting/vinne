package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	walletpb "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// POSHandler handles POS-specific operations for retailers
type POSHandler struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
}

// NewPOSHandler creates a new POS handler
func NewPOSHandler(grpcManager *grpc.ClientManager, log logger.Logger) *POSHandler {
	return &POSHandler{
		grpcManager: grpcManager,
		log:         log,
	}
}

// GetMyStakeBalance handles GET /api/v1/retailer/pos/wallet/stake
// Returns the stake wallet balance for the authenticated retailer
func (h *POSHandler) GetMyStakeBalance(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "POS.GetMyStakeBalance")
	defer span.End()

	// Get retailer ID from JWT context (set by auth middleware)
	retailerID := router.GetUserID(r)
	if retailerID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
	}

	span.SetAttributes(
		attribute.String("retailer.id", retailerID),
		attribute.String("wallet.type", "stake"),
	)

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Wallet service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Call wallet service for stake balance
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
		"wallet_type":       "stake",
		"balance":           resp.Balance,
		"pending_balance":   resp.PendingBalance,
		"available_balance": resp.AvailableBalance,
		"currency":          "GHS",
		"last_updated":      resp.LastUpdated.AsTime().Format(time.RFC3339),
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// GetMyWinningsBalance handles GET /api/v1/retailer/pos/wallet/winnings
// Returns the winnings wallet balance for the authenticated retailer
func (h *POSHandler) GetMyWinningsBalance(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "POS.GetMyWinningsBalance")
	defer span.End()

	// Get retailer ID from JWT context (set by auth middleware)
	retailerID := router.GetUserID(r)
	if retailerID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
	}

	span.SetAttributes(
		attribute.String("retailer.id", retailerID),
		attribute.String("wallet.type", "winning"),
	)

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Wallet service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Call wallet service for winnings balance
	req := &walletpb.GetRetailerWalletBalanceRequest{
		RetailerId: retailerID,
		WalletType: walletpb.WalletType_RETAILER_WINNING,
	}

	resp, err := client.GetRetailerWalletBalance(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get winnings wallet balance")
		h.log.Error("Failed to get winnings wallet balance", "error", err, "retailer_id", retailerID)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve winnings wallet balance")
	}

	// Convert response to JSON
	response := map[string]interface{}{
		"wallet_type":       "winnings",
		"balance":           resp.Balance,
		"pending_balance":   resp.PendingBalance,
		"available_balance": resp.AvailableBalance,
		"currency":          "GHS",
		"last_updated":      resp.LastUpdated.AsTime().Format(time.RFC3339),
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// GetMyWallets handles GET /api/v1/retailer/pos/wallets
// Returns both stake and winnings wallet balances for the authenticated retailer
func (h *POSHandler) GetMyWallets(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "POS.GetMyWallets")
	defer span.End()

	// Get retailer ID from JWT context (set by auth middleware)
	retailerID := router.GetUserID(r)
	if retailerID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
	}

	span.SetAttributes(
		attribute.String("retailer.id", retailerID),
	)

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Wallet service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Fetch both wallet balances in parallel
	type walletResult struct {
		walletType string
		resp       *walletpb.GetRetailerWalletBalanceResponse
		err        error
	}

	results := make(chan walletResult, 2)

	// Fetch stake wallet
	go func() {
		req := &walletpb.GetRetailerWalletBalanceRequest{
			RetailerId: retailerID,
			WalletType: walletpb.WalletType_RETAILER_STAKE,
		}
		resp, err := client.GetRetailerWalletBalance(ctx, req)
		results <- walletResult{"stake", resp, err}
	}()

	// Fetch winnings wallet
	go func() {
		req := &walletpb.GetRetailerWalletBalanceRequest{
			RetailerId: retailerID,
			WalletType: walletpb.WalletType_RETAILER_WINNING,
		}
		resp, err := client.GetRetailerWalletBalance(ctx, req)
		results <- walletResult{"winnings", resp, err}
	}()

	// Collect results
	wallets := make([]map[string]interface{}, 0, 2)
	var lastError error

	for i := 0; i < 2; i++ {
		result := <-results
		if result.err != nil {
			h.log.Error(fmt.Sprintf("Failed to get %s wallet", result.walletType),
				"error", result.err, "retailer_id", retailerID)
			lastError = result.err
			continue
		}

		wallet := map[string]interface{}{
			"wallet_type":       result.walletType,
			"balance":           result.resp.Balance,
			"pending_balance":   result.resp.PendingBalance,
			"available_balance": result.resp.AvailableBalance,
			"currency":          "GHS",
			"last_updated":      result.resp.LastUpdated.AsTime().Format(time.RFC3339),
		}
		wallets = append(wallets, wallet)
	}

	// Return error if both failed
	if len(wallets) == 0 && lastError != nil {
		span.RecordError(lastError)
		span.SetStatus(codes.Error, "failed to get any wallet balance")
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve wallet balances")
	}

	// Calculate totals
	var totalBalance, totalPending, totalAvailable float64
	for _, wallet := range wallets {
		totalBalance += wallet["balance"].(float64)
		totalPending += wallet["pending_balance"].(float64)
		totalAvailable += wallet["available_balance"].(float64)
	}

	response := map[string]interface{}{
		"wallets": wallets,
		"summary": map[string]interface{}{
			"total_balance":   totalBalance,
			"total_pending":   totalPending,
			"total_available": totalAvailable,
			"currency":        "GHS",
		},
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// GetMyTransactions handles GET /api/v1/retailer/pos/transactions
// Returns transaction history for the authenticated retailer
func (h *POSHandler) GetMyTransactions(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "POS.GetMyTransactions")
	defer span.End()

	// Get retailer ID from JWT context (set by auth middleware)
	retailerID := router.GetUserID(r)
	if retailerID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
	}

	// Parse query parameters
	walletType := r.URL.Query().Get("wallet_type") // "stake" or "winnings"
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	// Default values
	page := 1
	pageSize := 20

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// Determine wallet type
	var wType walletpb.WalletType
	switch walletType {
	case "stake":
		wType = walletpb.WalletType_RETAILER_STAKE
	case "winnings":
		wType = walletpb.WalletType_RETAILER_WINNING
	case "":
		wType = walletpb.WalletType_WALLET_TYPE_UNSPECIFIED // Will return all
	default:
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid wallet_type. Use 'stake' or 'winnings'")
	}

	span.SetAttributes(
		attribute.String("retailer.id", retailerID),
		attribute.String("wallet.type", walletType),
		attribute.Int("page", page),
		attribute.Int("page_size", pageSize),
	)

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Get wallet service client
	conn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Wallet service unavailable")
	}
	client := walletpb.NewWalletServiceClient(conn)

	// Build request
	req := &walletpb.GetTransactionHistoryRequest{
		WalletOwnerId: retailerID,
		WalletType:    wType,
		Page:          int32(page),
		PageSize:      int32(pageSize),
	}

	// Parse dates if provided
	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			req.StartDate = timestamppb.New(t)
		}
	}

	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			// Set to end of day
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			req.EndDate = timestamppb.New(t)
		}
	}

	// Call wallet service
	resp, err := client.GetTransactionHistory(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get transaction history")
		h.log.Error("Failed to get transaction history", "error", err, "retailer_id", retailerID)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve transaction history")
	}

	// Convert transactions to JSON format
	transactions := make([]map[string]interface{}, 0, len(resp.Transactions))
	for _, tx := range resp.Transactions {
		transaction := map[string]interface{}{
			"id":             tx.Id,
			"wallet_type":    getWalletTypeName(tx.WalletType),
			"type":           getTransactionTypeName(tx.Type),
			"amount":         tx.Amount,
			"balance_before": tx.BalanceBefore,
			"balance_after":  tx.BalanceAfter,
			"reference":      tx.Reference,
			"description":    tx.Description,
			"status":         getTransactionStatusName(tx.Status),
			"created_at":     tx.CreatedAt.AsTime().Format(time.RFC3339),
		}

		// Add metadata if present
		if len(tx.Metadata) > 0 {
			transaction["metadata"] = tx.Metadata
		}

		transactions = append(transactions, transaction)
	}

	response := map[string]interface{}{
		"transactions": transactions,
		"pagination": map[string]interface{}{
			"total":    resp.TotalCount,
			"page":     resp.Page,
			"size":     resp.PageSize,
			"has_more": resp.HasMore,
		},
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// Helper functions to convert enum values to strings
func getWalletTypeName(wt walletpb.WalletType) string {
	switch wt {
	case walletpb.WalletType_RETAILER_STAKE:
		return "stake"
	case walletpb.WalletType_RETAILER_WINNING:
		return "winnings"
	default:
		return "unknown"
	}
}

func getTransactionTypeName(tt walletpb.TransactionType) string {
	switch tt {
	case walletpb.TransactionType_CREDIT:
		return "credit"
	case walletpb.TransactionType_DEBIT:
		return "debit"
	case walletpb.TransactionType_TRANSFER:
		return "transfer"
	case walletpb.TransactionType_COMMISSION:
		return "commission"
	case walletpb.TransactionType_PAYOUT:
		return "payout"
	default:
		return "unknown"
	}
}

func getTransactionStatusName(ts walletpb.TransactionStatus) string {
	switch ts {
	case walletpb.TransactionStatus_PENDING:
		return "pending"
	case walletpb.TransactionStatus_COMPLETED:
		return "completed"
	case walletpb.TransactionStatus_FAILED:
		return "failed"
	case walletpb.TransactionStatus_REVERSED:
		return "reversed"
	default:
		return "unknown"
	}
}

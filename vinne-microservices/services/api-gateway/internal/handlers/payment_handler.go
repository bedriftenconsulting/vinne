package handlers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	paymentpb "github.com/randco/randco-microservices/proto/payment/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// PaymentHandler defines the interface for payment-related handlers
type PaymentHandler interface {
	// Mobile Money Topup
	InitiateTopup(w http.ResponseWriter, r *http.Request) error
	GetTopupStatus(w http.ResponseWriter, r *http.Request) error
	VerifyDepositStatus(w http.ResponseWriter, r *http.Request) error
	// Account Verification
	VerifyWallet(w http.ResponseWriter, r *http.Request) error
}

// paymentHandlerImpl handles payment-related HTTP requests
type paymentHandlerImpl struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
}

// NewPaymentHandler creates a new payment handler
func NewPaymentHandler(grpcManager *grpc.ClientManager, log logger.Logger) PaymentHandler {
	return &paymentHandlerImpl{
		grpcManager: grpcManager,
		log:         log,
	}
}

// TopupRequest represents the mobile money topup request
type TopupRequest struct {
	Amount       float64 `json:"amount"`        // Amount in GHS
	PhoneNumber  string  `json:"phone_number"`  // Mobile money number
	Provider     string  `json:"provider"`      // MTN, TELECEL, AIRTELTIGO
	Narration    string  `json:"narration"`     // Optional transaction description
	CustomerName string  `json:"customer_name"` // Customer name
}

// Validate validates the topup request
func (req *TopupRequest) Validate() error {
	// Validate amount
	if req.Amount <= 0 {
		return fmt.Errorf("amount must be greater than 0")
	}
	if req.Amount < 1 {
		return fmt.Errorf("amount must be at least 1 GHS")
	}
	if req.Amount > 100000 {
		return fmt.Errorf("amount cannot exceed 100,000 GHS")
	}

	// Validate phone number
	if req.PhoneNumber == "" {
		return fmt.Errorf("phone_number is required")
	}

	// Validate Ghanaian phone number format (10 digits starting with 0)
	phoneRegex := regexp.MustCompile(`^0[2-5][0-9]{8}$`)
	if !phoneRegex.MatchString(req.PhoneNumber) {
		return fmt.Errorf("invalid phone number format (must be 10 digits starting with 0)")
	}

	// Validate provider
	if req.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	req.Provider = normalizeProvider(req.Provider)
	if req.Provider == "" {
		return fmt.Errorf("invalid provider (must be MTN, TELECEL, or AIRTELTIGO)")
	}

	// Validate phone number matches provider network
	if err := validatePhoneProvider(req.PhoneNumber, req.Provider); err != nil {
		return err
	}

	// Validate customer name
	if req.CustomerName == "" {
		return fmt.Errorf("customer_name is required")
	}
	if len(req.CustomerName) < 2 {
		return fmt.Errorf("customer_name must be at least 2 characters")
	}
	if len(req.CustomerName) > 100 {
		return fmt.Errorf("customer_name cannot exceed 100 characters")
	}

	// Set default narration if empty
	if req.Narration == "" {
		req.Narration = "Stake wallet topup"
	}
	if len(req.Narration) > 100 {
		return fmt.Errorf("narration cannot exceed 100 characters")
	}

	return nil
}

// normalizeProvider normalizes provider name to uppercase
func normalizeProvider(provider string) string {
	switch provider {
	case "MTN", "mtn", "Mtn":
		return "MTN"
	case "TELECEL", "telecel", "Telecel":
		return "TELECEL"
	case "AIRTELTIGO", "airteltigo", "AirtelTigo", "airtel", "tigo":
		return "AIRTELTIGO"
	default:
		return ""
	}
}

// validatePhoneProvider validates that phone number matches the provider network
func validatePhoneProvider(phone, provider string) error {
	if len(phone) != 10 {
		return fmt.Errorf("invalid phone number length")
	}

	prefix := phone[:3]

	switch provider {
	case "MTN":
		// MTN prefixes: 024, 054, 055, 059
		validPrefixes := []string{"024", "054", "055", "059"}
		for _, p := range validPrefixes {
			if prefix == p {
				return nil
			}
		}
		return fmt.Errorf("phone number does not match MTN network (should start with 024, 054, 055, or 059)")

	case "TELECEL":
		// Telecel prefixes: 020, 050
		validPrefixes := []string{"020", "050"}
		for _, p := range validPrefixes {
			if prefix == p {
				return nil
			}
		}
		return fmt.Errorf("phone number does not match Telecel network (should start with 020 or 050)")

	case "AIRTELTIGO":
		// AirtelTigo prefixes: 027, 057, 026, 056
		validPrefixes := []string{"027", "057", "026", "056"}
		for _, p := range validPrefixes {
			if prefix == p {
				return nil
			}
		}
		return fmt.Errorf("phone number does not match AirtelTigo network (should start with 026, 027, 056, or 057)")

	default:
		return fmt.Errorf("invalid provider")
	}
}

// mapProviderToProto maps provider string to proto enum
func mapProviderToProto(provider string) paymentpb.WalletProvider {
	switch provider {
	case "MTN":
		return paymentpb.WalletProvider_WALLET_PROVIDER_MTN
	case "TELECEL":
		return paymentpb.WalletProvider_WALLET_PROVIDER_TELECEL
	case "AIRTELTIGO":
		return paymentpb.WalletProvider_WALLET_PROVIDER_AIRTELTIGO
	default:
		return paymentpb.WalletProvider_WALLET_PROVIDER_UNSPECIFIED
	}
}

// generateReference generates a unique reference for the transaction
func generateReference() string {
	return fmt.Sprintf("REF-%s-%d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000)
}

// InitiateTopup handles POST /api/v1/{role}/wallet/topup
func (h *paymentHandlerImpl) InitiateTopup(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "InitiateTopup")
	defer span.End()

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get user ID from context (injected by auth middleware)
	userID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || userID == "" {
		h.log.Error("User ID not found in context")
		return router.ErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
	}

	// Infer user role from URL path
	userRole := "unknown"
	if strings.Contains(r.URL.Path, "/agent/") {
		userRole = "agent"
	} else if strings.Contains(r.URL.Path, "/retailer/") {
		userRole = "retailer"
	} else if strings.Contains(r.URL.Path, "/admin/") {
		userRole = "admin"
	}

	// Parse request body
	var req TopupRequest
	if err := router.ReadJSON(r, &req); err != nil {
		h.log.Error("Failed to parse request body", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Validate request
	if err := req.Validate(); err != nil {
		h.log.Warn("Topup request validation failed", "error", err, "user_id", userID)
		return router.ErrorResponse(w, http.StatusBadRequest, err.Error())
	}

	// Convert amount from GHS to pesewas
	amountInPesewas := int64(req.Amount * 100)

	// Generate unique reference
	reference := generateReference()

	span.SetAttributes(
		attribute.String("user.id", userID),
		attribute.String("user.role", userRole),
		attribute.Int64("amount.pesewas", amountInPesewas),
		attribute.Float64("amount.ghs", req.Amount),
		attribute.String("provider", req.Provider),
		attribute.String("reference", reference),
		attribute.String("http.method", r.Method),
		attribute.String("http.path", r.URL.Path),
	)

	h.log.Info("Initiating mobile money topup",
		"user_id", userID,
		"user_role", userRole,
		"amount_ghs", req.Amount,
		"provider", req.Provider,
		"reference", reference,
	)

	// Get payment service client
	conn, err := h.grpcManager.GetConnection("payment")
	if err != nil {
		h.log.Error("Failed to get payment service connection", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "payment service unavailable")
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Payment service unavailable")
	}
	client := paymentpb.NewPaymentServiceClient(conn)

	// Call payment service
	depositReq := &paymentpb.InitiateDepositRequest{
		UserId:         userID,
		WalletNumber:   req.PhoneNumber,
		WalletProvider: mapProviderToProto(req.Provider),
		Amount:         amountInPesewas,
		Currency:       "GHS",
		Narration:      req.Narration,
		Reference:      reference,
		CustomerName:   req.CustomerName,
		Metadata: map[string]string{
			"user_role":      userRole,
			"source":         "api_gateway",
			"initiated_from": r.RemoteAddr,
		},
	}

	resp, err := client.InitiateDeposit(ctx, depositReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to initiate deposit")
		h.log.Error("Failed to initiate deposit", "error", err, "user_id", userID, "reference", reference)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to initiate topup")
	}

	if !resp.Success {
		h.log.Warn("Deposit initiation failed", "message", resp.Message, "user_id", userID, "reference", reference)
		return router.ErrorResponse(w, http.StatusUnprocessableEntity, resp.Message)
	}

	// Convert response to JSON
	response := map[string]interface{}{
		"success": true,
		"message": "Mobile money topup initiated successfully",
		"data": map[string]interface{}{
			"transaction_id":          resp.Transaction.Id,
			"reference":               resp.Transaction.Reference,
			"provider_transaction_id": resp.Transaction.ProviderTransactionId,
			"amount":                  resp.Transaction.Amount,
			"amount_ghs":              float64(resp.Transaction.Amount) / 100,
			"currency":                resp.Transaction.Currency,
			"status":                  resp.Transaction.Status.String(),
			"provider":                req.Provider,
			"phone_number":            req.PhoneNumber,
			"initiated_at":            resp.Transaction.RequestedAt.AsTime().Format(time.RFC3339),
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	h.log.Info("Mobile money topup initiated successfully",
		"user_id", userID,
		"reference", reference,
		"transaction_id", resp.Transaction.Id,
		"status", resp.Transaction.Status.String(),
	)

	return router.WriteJSON(w, http.StatusCreated, response)
}

// GetTopupStatus handles GET /api/v1/{role}/wallet/topup/:reference
func (h *paymentHandlerImpl) GetTopupStatus(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "GetTopupStatus")
	defer span.End()

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Get user ID from context
	userID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || userID == "" {
		h.log.Error("User ID not found in context")
		return router.ErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
	}

	// Get reference from URL parameter
	reference := router.GetParam(r, "reference")
	if reference == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Reference parameter is required")
	}

	span.SetAttributes(
		attribute.String("user.id", userID),
		attribute.String("reference", reference),
		attribute.String("http.method", r.Method),
		attribute.String("http.path", r.URL.Path),
	)

	h.log.Info("Getting topup status", "user_id", userID, "reference", reference)

	// Get payment service client
	conn, err := h.grpcManager.GetConnection("payment")
	if err != nil {
		h.log.Error("Failed to get payment service connection", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "payment service unavailable")
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Payment service unavailable")
	}
	client := paymentpb.NewPaymentServiceClient(conn)

	// Call payment service
	statusReq := &paymentpb.GetDepositStatusRequest{
		Reference: reference,
	}

	resp, err := client.GetDepositStatus(ctx, statusReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get deposit status")
		h.log.Error("Failed to get deposit status", "error", err, "user_id", userID, "reference", reference)
		return router.ErrorResponse(w, http.StatusNotFound, "Transaction not found")
	}

	if !resp.Success {
		h.log.Warn("Failed to get deposit status", "message", resp.Message, "reference", reference)
		return router.ErrorResponse(w, http.StatusNotFound, resp.Message)
	}

	// Verify that the transaction belongs to the requesting user
	if resp.Transaction.UserId != userID {
		h.log.Warn("User attempted to access another user's transaction",
			"requesting_user", userID,
			"transaction_user", resp.Transaction.UserId,
			"reference", reference,
		)
		return router.ErrorResponse(w, http.StatusForbidden, "Access denied")
	}

	// Convert response to JSON
	response := map[string]interface{}{
		"success": true,
		"message": "Transaction status retrieved successfully",
		"data": map[string]interface{}{
			"transaction_id":          resp.Transaction.Id,
			"reference":               resp.Transaction.Reference,
			"provider_transaction_id": resp.Transaction.ProviderTransactionId,
			"amount":                  resp.Transaction.Amount,
			"amount_ghs":              float64(resp.Transaction.Amount) / 100,
			"currency":                resp.Transaction.Currency,
			"status":                  resp.Transaction.Status.String(),
			"provider":                resp.Transaction.ProviderName,
			"phone_number":            resp.Transaction.SourceIdentifier,
			"initiated_at":            resp.Transaction.RequestedAt.AsTime().Format(time.RFC3339),
			"error_message":           resp.Transaction.ErrorMessage,
			"error_code":              resp.Transaction.ErrorCode,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Add completed_at if transaction is completed
	if resp.Transaction.CompletedAt != nil {
		response["data"].(map[string]interface{})["completed_at"] = resp.Transaction.CompletedAt.AsTime().Format(time.RFC3339)
	}

	h.log.Info("Topup status retrieved successfully",
		"user_id", userID,
		"reference", reference,
		"status", resp.Transaction.Status.String(),
	)

	return router.WriteJSON(w, http.StatusOK, response)
}

// VerifyDepositStatus handles POST /api/v1/{role}/wallet/verify-status
func (h *paymentHandlerImpl) VerifyDepositStatus(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "VerifyDepositStatus")
	defer span.End()

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Get user ID from context
	userID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || userID == "" {
		h.log.Error("User ID not found in context")
		return router.ErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
	}

	// Parse request body
	var req struct {
		TransactionID string `json:"transaction_id"` // Optional if reference provided
		Reference     string `json:"reference"`      // Client reference from original request
		ForceRefresh  bool   `json:"force_refresh"`  // Force provider status check (bypass cache)
	}

	if err := router.ReadJSON(r, &req); err != nil {
		h.log.Error("Failed to parse request body", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Validate that at least one identifier is provided
	if req.TransactionID == "" && req.Reference == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Either transaction_id or reference must be provided")
	}

	span.SetAttributes(
		attribute.String("user.id", userID),
		attribute.String("transaction_id", req.TransactionID),
		attribute.String("reference", req.Reference),
		attribute.Bool("force_refresh", req.ForceRefresh),
		attribute.String("http.method", r.Method),
		attribute.String("http.path", r.URL.Path),
	)

	h.log.Info("Verifying deposit status",
		"user_id", userID,
		"transaction_id", req.TransactionID,
		"reference", req.Reference,
		"force_refresh", req.ForceRefresh,
	)

	// Get payment service client
	conn, err := h.grpcManager.GetConnection("payment")
	if err != nil {
		h.log.Error("Failed to get payment service connection", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "payment service unavailable")
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Payment service unavailable")
	}
	client := paymentpb.NewPaymentServiceClient(conn)

	// Call payment service
	verifyReq := &paymentpb.VerifyDepositStatusRequest{
		TransactionId: req.TransactionID,
		Reference:     req.Reference,
		ForceRefresh:  req.ForceRefresh,
	}

	resp, err := client.VerifyDepositStatus(ctx, verifyReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to verify deposit status")
		h.log.Error("Failed to verify deposit status",
			"error", err,
			"user_id", userID,
			"transaction_id", req.TransactionID,
			"reference", req.Reference,
		)
		return router.ErrorResponse(w, http.StatusNotFound, "Transaction not found")
	}

	if !resp.Success {
		h.log.Warn("Failed to verify deposit status", "message", resp.Message)
		return router.ErrorResponse(w, http.StatusNotFound, resp.Message)
	}

	// Verify that the transaction belongs to the requesting user
	if resp.Transaction != nil && resp.Transaction.UserId != userID {
		h.log.Warn("User attempted to verify another user's transaction",
			"requesting_user", userID,
			"transaction_user", resp.Transaction.UserId,
			"reference", req.Reference,
		)
		return router.ErrorResponse(w, http.StatusForbidden, "Access denied")
	}

	// Convert response to JSON
	response := map[string]interface{}{
		"success": true,
		"message": resp.Message,
		"data": map[string]interface{}{
			"status_info": map[string]interface{}{
				"transaction_id":       resp.StatusInfo.TransactionId,
				"reference":            resp.StatusInfo.Reference,
				"status":               resp.StatusInfo.Status.String(),
				"provider_status_code": resp.StatusInfo.ProviderStatusCode,
				"requested_at":         resp.StatusInfo.RequestedAt.AsTime().Format(time.RFC3339),
				"error_message":        resp.StatusInfo.ErrorMessage,
			},
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Add completed_at if available
	if resp.StatusInfo.CompletedAt != nil && resp.StatusInfo.CompletedAt.IsValid() {
		response["data"].(map[string]interface{})["status_info"].(map[string]interface{})["completed_at"] = resp.StatusInfo.CompletedAt.AsTime().Format(time.RFC3339)
	}

	// Add full transaction details if available
	if resp.Transaction != nil {
		response["data"].(map[string]interface{})["transaction"] = map[string]interface{}{
			"transaction_id":          resp.Transaction.Id,
			"reference":               resp.Transaction.Reference,
			"provider_transaction_id": resp.Transaction.ProviderTransactionId,
			"amount":                  resp.Transaction.Amount,
			"amount_ghs":              float64(resp.Transaction.Amount) / 100,
			"currency":                resp.Transaction.Currency,
			"status":                  resp.Transaction.Status.String(),
			"provider":                resp.Transaction.ProviderName,
			"phone_number":            resp.Transaction.SourceIdentifier,
			"initiated_at":            resp.Transaction.RequestedAt.AsTime().Format(time.RFC3339),
			"error_message":           resp.Transaction.ErrorMessage,
			"error_code":              resp.Transaction.ErrorCode,
		}

		// Add completed_at if transaction is completed
		if resp.Transaction.CompletedAt != nil {
			response["data"].(map[string]interface{})["transaction"].(map[string]interface{})["completed_at"] = resp.Transaction.CompletedAt.AsTime().Format(time.RFC3339)
		}
	}

	h.log.Info("Deposit status verified successfully",
		"user_id", userID,
		"transaction_id", resp.StatusInfo.TransactionId,
		"reference", resp.StatusInfo.Reference,
		"status", resp.StatusInfo.Status.String(),
		"provider_status_code", resp.StatusInfo.ProviderStatusCode,
	)

	return router.WriteJSON(w, http.StatusOK, response)
}

// VerifyWallet handles POST /api/v1/{role}/wallet/verify-account
func (h *paymentHandlerImpl) VerifyWallet(w http.ResponseWriter, r *http.Request) error {
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(r.Context(), "VerifyWallet")
	defer span.End()

	// Add timeout to context (30s to accommodate Orange authentication + verification)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get user ID from context (for audit purposes)
	userID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || userID == "" {
		h.log.Error("User ID not found in context")
		return router.ErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
	}

	// Parse request body
	var req struct {
		WalletNumber   string `json:"wallet_number"`   // Phone number
		WalletProvider string `json:"wallet_provider"` // MTN, TELECEL, AIRTELTIGO
	}

	if err := router.ReadJSON(r, &req); err != nil {
		h.log.Error("Failed to parse request body", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Validate wallet number
	if req.WalletNumber == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "wallet_number is required")
	}

	// Validate phone number format (10 digits starting with 0)
	phoneRegex := regexp.MustCompile(`^0[2-5][0-9]{8}$`)
	if !phoneRegex.MatchString(req.WalletNumber) {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid phone number format (must be 10 digits starting with 0)")
	}

	// Validate provider
	if req.WalletProvider == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "wallet_provider is required")
	}
	req.WalletProvider = normalizeProvider(req.WalletProvider)
	if req.WalletProvider == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid provider (must be MTN, TELECEL, or AIRTELTIGO)")
	}

	// Validate phone number matches provider network
	if err := validatePhoneProvider(req.WalletNumber, req.WalletProvider); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, err.Error())
	}

	// Generate reference for tracking
	reference := generateReference()

	span.SetAttributes(
		attribute.String("user.id", userID),
		attribute.String("wallet_number", req.WalletNumber),
		attribute.String("provider", req.WalletProvider),
		attribute.String("reference", reference),
		attribute.String("http.method", r.Method),
		attribute.String("http.path", r.URL.Path),
	)

	h.log.Info("Verifying wallet account",
		"user_id", userID,
		"wallet_number", req.WalletNumber,
		"provider", req.WalletProvider,
		"reference", reference,
	)

	// Get payment service client
	conn, err := h.grpcManager.GetConnection("payment")
	if err != nil {
		h.log.Error("Failed to get payment service connection", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "payment service unavailable")
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Payment service unavailable")
	}
	client := paymentpb.NewPaymentServiceClient(conn)

	// Call payment service
	verifyReq := &paymentpb.VerifyWalletRequest{
		WalletNumber:   req.WalletNumber,
		WalletProvider: mapProviderToProto(req.WalletProvider),
		Reference:      reference,
	}

	h.log.Debug("Calling payment service to verify wallet",
		"user_id", userID,
		"wallet_number", req.WalletNumber,
		"provider", req.WalletProvider,
		"reference", reference,
		"timeout", "30s",
	)

	startTime := time.Now()
	resp, err := client.VerifyWallet(ctx, verifyReq)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to verify wallet")
		h.log.Error("Failed to verify wallet",
			"error", err,
			"user_id", userID,
			"wallet_number", req.WalletNumber,
			"reference", reference,
			"duration_ms", duration.Milliseconds(),
			"duration", duration.String(),
		)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to verify wallet account")
	}

	h.log.Debug("Payment service responded",
		"user_id", userID,
		"wallet_number", req.WalletNumber,
		"reference", reference,
		"duration_ms", duration.Milliseconds(),
		"duration", duration.String(),
		"success", resp.Success,
		"message", resp.Message,
	)

	if !resp.Success {
		h.log.Warn("Wallet verification failed",
			"message", resp.Message,
			"user_id", userID,
			"wallet_number", req.WalletNumber,
			"reference", reference,
			"duration_ms", duration.Milliseconds(),
		)
		return router.ErrorResponse(w, http.StatusUnprocessableEntity, resp.Message)
	}

	// Convert response to JSON
	response := map[string]interface{}{
		"success": true,
		"message": "Wallet verified successfully",
		"data": map[string]interface{}{
			"is_valid":        resp.Verification.IsValid,
			"account_name":    resp.Verification.AccountName,
			"wallet_number":   resp.Verification.WalletNumber,
			"wallet_provider": resp.Verification.WalletProvider,
			"reference":       resp.Verification.Reference,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	h.log.Info("Wallet verified successfully",
		"user_id", userID,
		"wallet_number", req.WalletNumber,
		"account_name", resp.Verification.AccountName,
		"is_valid", resp.Verification.IsValid,
		"reference", reference,
	)

	return router.WriteJSON(w, http.StatusOK, response)
}

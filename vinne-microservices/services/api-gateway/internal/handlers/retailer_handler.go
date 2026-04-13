package handlers

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	agentauthv1 "github.com/randco/randco-microservices/proto/agent/auth/v1"
	agentmgmtpb "github.com/randco/randco-microservices/proto/agent/management/v1"
	terminalpb "github.com/randco/randco-microservices/proto/terminal/v1"
	terminalv1 "github.com/randco/randco-microservices/proto/terminal/v1"
	ticketv1 "github.com/randco/randco-microservices/proto/ticket/v1"
	walletpb "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/config"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/response"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// retailerHandlerImpl handles retailer management requests
type retailerHandlerImpl struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
	config      *config.Config
}

// NewRetailerHandler creates a new retailer handler
func NewRetailerHandler(grpcManager *grpc.ClientManager, log logger.Logger, cfg *config.Config) RetailerHandler {
	return &retailerHandlerImpl{
		grpcManager: grpcManager,
		log:         log,
		config:      cfg,
	}
}

// RetailerCreatePayload is the normalized payload for retailer creation
type RetailerCreatePayload struct {
	Name         string  `json:"name"`
	BusinessName *string `json:"business_name"`
	Email        string  `json:"email"`
	PhoneNumber  string  `json:"phone_number"`
	Address      string  `json:"address"`
	AgentID      *string `json:"agent_id,omitempty"`
	CreatedBy    string  `json:"created_by,omitempty"` // accepted but ignored; actor is taken from JWT
}

// createRetailerCore centralizes retailer creation and optional auth provisioning
func (h *retailerHandlerImpl) createRetailerCore(ctx context.Context, payload RetailerCreatePayload, createdBy string, onboardingMethod string, createAuth bool, pin string) (map[string]any, bool, error) {
	// Build gRPC request for agent-management
	mgmtConn, err := h.grpcManager.GetConnection("agent-management")
	if err != nil {
		h.log.Error("Failed to get agent management connection", "error", err)
		return nil, false, status.Error(codes.Unavailable, "Service unavailable")
	}
	mgmtClient := agentmgmtpb.NewAgentManagementServiceClient(mgmtConn)

	businessName := payload.Name
	if payload.BusinessName != nil && *payload.BusinessName != "" {
		businessName = *payload.BusinessName
	}
	createReq := &agentmgmtpb.CreateRetailerRequest{
		Name:             payload.Name,
		BusinessName:     businessName,
		Email:            payload.Email,
		PhoneNumber:      payload.PhoneNumber,
		Address:          payload.Address,
		CreatedBy:        createdBy,
		OnboardingMethod: onboardingMethod,
	}
	if payload.AgentID != nil && *payload.AgentID != "" {
		createReq.AgentId = *payload.AgentID
	}

	retailerResp, err := mgmtClient.CreateRetailer(ctx, createReq)
	if err != nil {
		h.log.Error("Failed to create retailer", "error", err)
		return nil, false, err
	}

	authCreated := false
	if createAuth {
		authConn, err := h.grpcManager.GetConnection("agent-auth")
		if err != nil {
			h.log.Error("Failed to get agent auth connection", "error", err)
		} else {
			authClient := agentauthv1.NewAgentAuthServiceClient(authConn)
			authReq := &agentauthv1.CreateRetailerAuthRequest{
				RetailerId:   retailerResp.Id,
				RetailerCode: retailerResp.RetailerCode,
				Email:        retailerResp.Email,
				Phone:        retailerResp.PhoneNumber,
				Pin:          pin,
				CreatedBy:    createdBy,
			}
			if _, authErr := authClient.CreateRetailerAuth(ctx, authReq); authErr != nil {
				h.log.Error("Failed to create retailer auth", "error", authErr,
					"retailer_id", retailerResp.Id,
					"retailer_code", retailerResp.RetailerCode)
			} else {
				authCreated = true
			}
		}
	}

	resp := map[string]any{
		"retailer": map[string]any{
			"id":            retailerResp.Id,
			"retailer_code": retailerResp.RetailerCode,
			"name":          retailerResp.Name,
			"email":         retailerResp.Email,
			"phone_number":  retailerResp.PhoneNumber,
			"address":       retailerResp.Address,
			"status":        convertStatusEnumToString(retailerResp.Status),
			"created_at": func() string {
				if retailerResp.CreatedAt != nil {
					return retailerResp.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z")
				}
				return ""
			}(),
		},
	}

	return resp, authCreated, nil
}

// CreateRetailer handles creating a new retailer with auth credentials
func (h *retailerHandlerImpl) CreateRetailer(w http.ResponseWriter, r *http.Request) error {
	var req RetailerCreatePayload
	if err := router.ReadJSON(r, &req); err != nil {
		h.log.Error("Failed to parse request body", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Actor identity
	userID, ok := r.Context().Value("user_id").(string)
	if !ok || userID == "" {
		userID = "admin"
	}

	// PIN generation policy
	var pin string
	env := h.config.Tracing.Environment
	if env == "local" || env == "development" || env == "" {
		pin = "1234"
		h.log.Info("Using hardcoded PIN for local development", "phone", req.PhoneNumber, "pin", pin)
	} else {
		pin = generateSimplePIN()
		h.log.Info("Generated PIN for retailer", "phone", req.PhoneNumber, "pin", pin)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, authCreated, err := h.createRetailerCore(ctx, req, userID, "RAND_LOTTERY_LTD_DIRECT", true, pin)
	if err != nil {
		return h.handleGRPCError(w, err)
	}

	resp["auth_created"] = authCreated
	if authCreated {
		resp["message"] = "Retailer created successfully"
		resp["initial_pin"] = pin
	} else {
		resp["message"] = "Retailer created but authentication setup failed. Manual intervention required."
	}

	return router.WriteJSON(w, http.StatusCreated, resp)
}

// ListRetailers handles listing retailers
func (h *retailerHandlerImpl) ListRetailers(w http.ResponseWriter, r *http.Request) error {
	// Get agent management client
	conn, err := h.grpcManager.GetConnection("agent-management")
	if err != nil {
		h.log.Error("Failed to get agent management connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := agentmgmtpb.NewAgentManagementServiceClient(conn)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Build the request
	grpcReq := &agentmgmtpb.ListRetailersRequest{
		PageSize: 100,
		Page:     1,
	}

	// Create filter if needed
	filter := &agentmgmtpb.RetailerFilter{}
	hasFilter := false

	// Get status filter if provided
	if status := r.URL.Query().Get("status"); status != "" {
		// Convert string to EntityStatus enum
		// TODO: Properly map status string to enum
		hasFilter = true
	}

	// Get agent filter if provided
	if agentID := r.URL.Query().Get("agent_id"); agentID != "" {
		filter.AgentId = agentID
		hasFilter = true
	}

	if hasFilter {
		grpcReq.Filter = filter
	}

	resp, err := client.ListRetailers(ctx, grpcReq)
	if err != nil {
		h.log.Debug("List retailers failed", "error", err)
		return h.handleGRPCError(w, err)
	}

	// Convert retailers to response format
	retailers := make([]map[string]any, 0, len(resp.Retailers))
	for _, r := range resp.Retailers {
		retailer := map[string]any{
			"id":            r.Id,
			"retailer_code": r.RetailerCode,
			"name":          r.Name,
			"email":         r.Email,
			"phone_number":  r.PhoneNumber,
			"address":       r.Address,
			"status":        convertStatusEnumToString(r.Status),
			"created_at":    r.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
		}

		// Add agent ID if available
		if r.AgentId != "" {
			retailer["agent_id"] = r.AgentId
		}

		// Add timestamps if available
		if r.UpdatedAt != nil {
			retailer["updated_at"] = r.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z")
		}

		retailers = append(retailers, retailer)
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"data": retailers,
		"pagination": map[string]any{
			"page":        resp.Page,
			"page_size":   resp.PageSize,
			"total_count": resp.TotalCount,
			"total_pages": (resp.TotalCount + int32(resp.PageSize) - 1) / int32(resp.PageSize),
		},
	})
}

// GetRetailer handles getting a single retailer (not implemented yet)
func (h *retailerHandlerImpl) GetRetailer(w http.ResponseWriter, r *http.Request) error {
	retailerID := router.GetParam(r, "id")
	if retailerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Retailer ID is required")
	}

	// Get agent management client
	conn, err := h.grpcManager.GetConnection("agent-management")
	if err != nil {
		h.log.Error("Failed to get agent management connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := agentmgmtpb.NewAgentManagementServiceClient(conn)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentmgmtpb.GetRetailerRequest{
		Id: retailerID,
	}

	resp, err := client.GetRetailer(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Get retailer failed", "retailer_id", retailerID, "error", err)
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return router.ErrorResponse(w, http.StatusNotFound, "Retailer not found")
		}
		return h.handleGRPCError(w, err)
	}

	// Convert response to map
	retailer := map[string]any{
		"id":            resp.Id,
		"retailer_code": resp.RetailerCode,
		"name":          resp.Name,
		"email":         resp.Email,
		"phone_number":  resp.PhoneNumber,
		"address":       resp.Address,
		"status":        convertStatusEnumToString(resp.Status),
		"created_by":    resp.CreatedBy,
		"created_at":    resp.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
	}

	// Add agent ID if available
	if resp.AgentId != "" {
		retailer["agent_id"] = resp.AgentId
	}

	// Add timestamps if available
	if resp.UpdatedAt != nil {
		retailer["updated_at"] = resp.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"retailer": retailer,
	})
}

// UpdateRetailer handles updating a retailer
func (h *retailerHandlerImpl) UpdateRetailer(w http.ResponseWriter, r *http.Request) error {
	retailerID := router.GetParam(r, "id")
	if retailerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Retailer ID is required")
	}

	var req struct {
		Name        *string `json:"name"`
		Email       *string `json:"email"`
		PhoneNumber *string `json:"phone_number"`
		Address     *string `json:"address"`
	}
	if err := router.ReadJSON(r, &req); err != nil {
		h.log.Error("Failed to read request body", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Extract updater identity from JWT context
	updatedBy, _ := r.Context().Value(router.ContextEmail).(string)
	if updatedBy == "" {
		// Fallback to user_id if email not present
		if uid, ok := r.Context().Value(router.ContextUserID).(string); ok {
			updatedBy = uid
		}
	}
	if updatedBy == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "User information not available")
	}

	// gRPC client
	conn, err := h.grpcManager.GetConnection("agent-management")
	if err != nil {
		h.log.Error("Failed to get agent management connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := agentmgmtpb.NewAgentManagementServiceClient(conn)

	// Build gRPC request
	grpcReq := &agentmgmtpb.UpdateRetailerRequest{
		Id:        retailerID,
		UpdatedBy: updatedBy,
	}
	if req.Name != nil {
		grpcReq.Name = *req.Name
	}
	if req.Email != nil {
		grpcReq.Email = *req.Email
	}
	if req.PhoneNumber != nil {
		grpcReq.PhoneNumber = *req.PhoneNumber
	}
	if req.Address != nil {
		grpcReq.Address = *req.Address
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.UpdateRetailer(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Update retailer failed", "retailer_id", retailerID, "error", err)
		return h.handleGRPCError(w, err)
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"retailer": map[string]any{
			"id":            resp.Id,
			"retailer_code": resp.RetailerCode,
			"name":          resp.Name,
			"email":         resp.Email,
			"phone_number":  resp.PhoneNumber,
			"address":       resp.Address,
			"status":        convertStatusEnumToString(resp.Status),
			"updated_by":    resp.UpdatedBy,
			"updated_at": func() string {
				if resp.UpdatedAt != nil {
					return resp.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z")
				}
				return ""
			}(),
		},
		"message": "Retailer updated successfully",
	})
}

// UpdateRetailerStatus handles updating retailer status
func (h *retailerHandlerImpl) UpdateRetailerStatus(w http.ResponseWriter, r *http.Request) error {
	retailerID := router.GetParam(r, "id")
	if retailerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Retailer ID is required")
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}
	if req.Status == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Status is required")
	}

	// Extract user email from JWT
	updatedBy, _ := r.Context().Value(router.ContextEmail).(string)
	if updatedBy == "" {
		if uid, ok := r.Context().Value(router.ContextUserID).(string); ok {
			updatedBy = uid
		}
	}
	if updatedBy == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "User information not available")
	}

	// Map string status to enum
	enumStatus := convertStatusStringToEnum(strings.ToLower(req.Status))

	conn, err := h.grpcManager.GetConnection("agent-management")
	if err != nil {
		h.log.Error("Failed to get agent management connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	client := agentmgmtpb.NewAgentManagementServiceClient(conn)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, err = client.UpdateRetailerStatus(ctx, &agentmgmtpb.UpdateRetailerStatusRequest{
		Id:        retailerID,
		Status:    enumStatus,
		UpdatedBy: updatedBy,
	})
	if err != nil {
		h.log.Debug("Update retailer status failed", "retailer_id", retailerID, "status", req.Status, "error", err)
		return h.handleGRPCError(w, err)
	}

	return router.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Retailer status updated successfully",
	})
}

// ListAgentRetailers handles listing retailers for an authenticated agent
func (h *retailerHandlerImpl) ListAgentRetailers(w http.ResponseWriter, r *http.Request) error {
	// Get agent ID from JWT context
	agentID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || agentID == "" {
		h.log.Error("Agent ID not found in context")
		return router.ErrorResponse(w, http.StatusUnauthorized, "Agent authentication required")
	}

	// Get agent management client
	amConn, err := h.grpcManager.GetConnection("agent-management")
	if err != nil {
		h.log.Error("Failed to get agent management connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	amClient := agentmgmtpb.NewAgentManagementServiceClient(amConn)

	walletConn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	walletClient := walletpb.NewWalletServiceClient(walletConn)

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Call GetAgentRetailers gRPC method
	grpcReq := &agentmgmtpb.GetAgentRetailersRequest{
		AgentId: agentID,
	}

	resp, err := amClient.GetAgentRetailers(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Get agent retailers failed", "agent_id", agentID, "error", err)
		return h.handleGRPCError(w, err)
	}

	// Convert retailers to response format with additional data
	retailers := make([]map[string]any, 0, len(resp.Retailers))

	// Use wait group to fetch additional data in parallel for each retailer
	var wg sync.WaitGroup
	mu := sync.Mutex{}

	for _, r := range resp.Retailers {
		wg.Add(1)
		go func(retailer *agentmgmtpb.Retailer) {
			defer wg.Done()

			retailerID := retailer.Id
			retailerData := map[string]any{
				"id":            retailer.Id,
				"retailer_code": retailer.RetailerCode,
				"name":          retailer.Name,
				"email":         retailer.Email,
				"phone_number":  retailer.PhoneNumber,
				"address":       retailer.Address,
				"status":        convertStatusEnumToString(retailer.Status),
				"created_at":    retailer.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
			}

			// Add agent ID if available
			if retailer.AgentId != "" {
				retailerData["agent_id"] = retailer.AgentId
			}

			// Add timestamps if available
			if retailer.UpdatedAt != nil {
				retailerData["updated_at"] = retailer.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z")
			}

			// Fetch wallet balance (stake wallet)
			stakeBalanceResp, err := walletClient.GetRetailerWalletBalance(ctx, &walletpb.GetRetailerWalletBalanceRequest{
				RetailerId: retailerID,
				WalletType: walletpb.WalletType_RETAILER_STAKE,
			})
			if err != nil {
				h.log.Warn("Failed to get stake balance", "retailer_id", retailerID, "error", err)
				retailerData["balance"] = 0.0
			} else {
				retailerData["balance"] = stakeBalanceResp.Balance / 100.0 // Convert pesewas to GHS
			}

			// Fetch POS device count
			posDevicesResp, err := amClient.ListPOSDevices(ctx, &agentmgmtpb.ListPOSDevicesRequest{
				PageSize: 100,
				Page:     1,
				Filter: &agentmgmtpb.POSDeviceFilter{
					RetailerId: retailerID,
				},
			})
			if err != nil {
				h.log.Warn("Failed to get POS devices", "retailer_id", retailerID, "error", err)
				retailerData["pos_devices_count"] = 0
			} else {
				retailerData["pos_devices_count"] = len(posDevicesResp.Devices)
			}

			// This field is kept for consistency with potential future requirements.
			retailerData["commission_earned"] = 0.0

			// Fetch last transaction date from wallet transaction history
			txHistoryResp, err := walletClient.GetTransactionHistory(ctx, &walletpb.GetTransactionHistoryRequest{
				WalletOwnerId: retailerID,
				WalletType:    walletpb.WalletType_RETAILER_STAKE,
				Page:          1,
				PageSize:      1,
			})
			if err != nil {
				h.log.Warn("Failed to get transaction history", "retailer_id", retailerID, "error", err)
				retailerData["last_transaction_date"] = nil
			} else if len(txHistoryResp.Transactions) > 0 {
				lastTx := txHistoryResp.Transactions[0]
				if lastTx.CreatedAt != nil {
					retailerData["last_transaction_date"] = lastTx.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z")
				} else {
					retailerData["last_transaction_date"] = nil
				}
			} else {
				retailerData["last_transaction_date"] = nil
			}

			mu.Lock()
			retailers = append(retailers, retailerData)
			mu.Unlock()
		}(r)
	}

	wg.Wait()

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    retailers,
		"count":   len(retailers),
	})
}

// OnboardRetailer allows an authenticated agent to create a retailer under themselves
func (h *retailerHandlerImpl) OnboardRetailer(w http.ResponseWriter, r *http.Request) error {
	agentID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || agentID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Agent authentication required")
	}

	var in struct {
		Name         string `json:"owner_name"`
		BusinessName string `json:"business_name"`
		Email        string `json:"owner_email"`
		PhoneNumber  string `json:"owner_phone"`
		Address      string `json:"physical_address"`
	}
	if err := router.ReadJSON(r, &in); err != nil {
		h.log.Error("Failed to parse request body", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	name := in.BusinessName
	if name == "" {
		name = in.Name
	}
	payload := RetailerCreatePayload{
		Name:         name,
		BusinessName: &in.BusinessName,
		Email:        in.Email,
		PhoneNumber:  in.PhoneNumber,
		Address:      in.Address,
	}
	payload.AgentID = &agentID

	var pin string
	env := h.config.Tracing.Environment
	if env == "local" || env == "development" || env == "" {
		pin = "1234"
	} else {
		pin = generateSimplePIN()
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, authCreated, err := h.createRetailerCore(ctx, payload, agentID, "AGENT_ONBOARDED", true, pin)
	if err != nil {
		h.log.Debug("Onboard retailer failed", "agent_id", agentID, "error", err)
		return h.handleGRPCError(w, err)
	}

	resp["auth_created"] = authCreated
	resp["message"] = "Retailer onboarded successfully"
	if authCreated {
		resp["initial_pin"] = pin
	}
	return router.WriteJSON(w, http.StatusCreated, resp)
}

// GetRetailerDetails handles getting retailer details for an authenticated agent
func (h *retailerHandlerImpl) GetRetailerDetails(w http.ResponseWriter, r *http.Request) error {
	// Get retailer ID from URL params
	retailerID := router.GetParam(r, "id")
	if retailerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Retailer ID is required")
	}

	// Get agent ID from JWT context
	agentID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || agentID == "" {
		h.log.Error("Agent ID not found in context")
		return router.ErrorResponse(w, http.StatusUnauthorized, "Agent authentication required")
	}

	// Get agent management client
	amConn, err := h.grpcManager.GetConnection("agent-management")
	if err != nil {
		h.log.Error("Failed to get agent management connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	amClient := agentmgmtpb.NewAgentManagementServiceClient(amConn)

	// Get wallet client
	walletConn, err := h.grpcManager.GetConnection("wallet")
	if err != nil {
		h.log.Error("Failed to get wallet connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	walletClient := walletpb.NewWalletServiceClient(walletConn)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Get retailer details (includes KYC)
	grpcReq := &agentmgmtpb.GetRetailerDetailsRequest{
		Id: retailerID,
	}

	detailsResp, err := amClient.GetRetailerDetails(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Get retailer details failed", "retailer_id", retailerID, "error", err)
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return router.ErrorResponse(w, http.StatusNotFound, "Retailer not found")
		}
		return h.handleGRPCError(w, err)
	}

	// Authorization check: Verify retailer belongs to this agent
	if detailsResp.Retailer == nil || detailsResp.Retailer.AgentId != agentID {
		h.log.Warn("Agent attempted to access retailer from different agent",
			"agent_id", agentID,
			"retailer_id", retailerID)
		if detailsResp.Retailer != nil {
			h.log.Warn("Retailer agent ID", "retailer_agent_id", detailsResp.Retailer.AgentId)
		}
		return router.ErrorResponse(w, http.StatusForbidden, "You don't have permission to view this retailer")
	}

	// Convert retailer response to map
	retailer := map[string]any{
		"id":            detailsResp.Retailer.Id,
		"retailer_code": detailsResp.Retailer.RetailerCode,
		"name":          detailsResp.Retailer.Name,
		"email":         detailsResp.Retailer.Email,
		"phone_number":  detailsResp.Retailer.PhoneNumber,
		"address":       detailsResp.Retailer.Address,
		"status":        convertStatusEnumToString(detailsResp.Retailer.Status),
		"agent_id":      detailsResp.Retailer.AgentId,
		"created_by":    detailsResp.Retailer.CreatedBy,
		"created_at":    detailsResp.Retailer.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
	}

	// Add optional fields
	if detailsResp.Retailer.UpdatedAt != nil {
		retailer["updated_at"] = detailsResp.Retailer.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z")
	}
	if detailsResp.Retailer.UpdatedBy != "" {
		retailer["updated_by"] = detailsResp.Retailer.UpdatedBy
	}

	// Prepare KYC (already in response)
	var kyc map[string]any
	if detailsResp.Kyc != nil {
		kyc = map[string]any{
			"id":                           detailsResp.Kyc.Id,
			"retailer_id":                  detailsResp.Kyc.RetailerId,
			"id_number":                    detailsResp.Kyc.IdNumber,
			"id_type":                      detailsResp.Kyc.IdType,
			"business_name":                detailsResp.Kyc.BusinessName,
			"business_registration_number": detailsResp.Kyc.BusinessRegistrationNumber,
			"status":                       convertKYCStatusToString(detailsResp.Kyc.Status),
		}
		if detailsResp.Kyc.ReviewedBy != "" {
			kyc["reviewed_by"] = detailsResp.Kyc.ReviewedBy
		}
		if detailsResp.Kyc.ExpiryDate != nil {
			kyc["expiry_date"] = detailsResp.Kyc.ExpiryDate.AsTime().Format("2006-01-02T15:04:05Z")
		}
		if detailsResp.Kyc.CreatedAt != nil {
			kyc["created_at"] = detailsResp.Kyc.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z")
		}
		if detailsResp.Kyc.UpdatedAt != nil {
			kyc["updated_at"] = detailsResp.Kyc.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z")
		}
	}

	// Fetch wallet balances in parallel
	var wg sync.WaitGroup
	var stakeBalance, winningsBalance float64
	var stakeErr, winningsErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		stakeResp, err := walletClient.GetRetailerWalletBalance(ctx, &walletpb.GetRetailerWalletBalanceRequest{
			RetailerId: retailerID,
			WalletType: walletpb.WalletType_RETAILER_STAKE,
		})
		if err != nil {
			stakeErr = err
			stakeBalance = 0.0
		} else {
			stakeBalance = stakeResp.Balance / 100.0 // Convert pesewas to GHS
		}
	}()

	go func() {
		defer wg.Done()
		winningsResp, err := walletClient.GetRetailerWalletBalance(ctx, &walletpb.GetRetailerWalletBalanceRequest{
			RetailerId: retailerID,
			WalletType: walletpb.WalletType_RETAILER_WINNING,
		})
		if err != nil {
			winningsErr = err
			winningsBalance = 0.0
		} else {
			winningsBalance = winningsResp.Balance / 100.0 // Convert pesewas to GHS
		}
	}()

	wg.Wait()

	if stakeErr != nil {
		h.log.Warn("Failed to get stake balance", "retailer_id", retailerID, "error", stakeErr)
	}
	if winningsErr != nil {
		h.log.Warn("Failed to get winnings balance", "retailer_id", retailerID, "error", winningsErr)
	}

	response := map[string]any{
		"success":  true,
		"retailer": retailer,
	}

	if kyc != nil {
		response["kyc"] = kyc
	}

	response["wallet"] = map[string]any{
		"stake": map[string]any{
			"balance":  stakeBalance,
			"currency": "GHS",
		},
		"winnings": map[string]any{
			"balance":  winningsBalance,
			"currency": "GHS",
		},
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// GetRetailerPOSDevices handles getting POS devices for a retailer
func (h *retailerHandlerImpl) GetRetailerPOSDevices(w http.ResponseWriter, r *http.Request) error {
	retailerID := router.GetParam(r, "id")
	if retailerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Retailer ID is required")
	}

	agentID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || agentID == "" {
		h.log.Error("Agent ID not found in context")
		return router.ErrorResponse(w, http.StatusUnauthorized, "Agent authentication required")
	}

	amConn, err := h.grpcManager.GetConnection("agent-management")
	if err != nil {
		h.log.Error("Failed to get agent management connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	amClient := agentmgmtpb.NewAgentManagementServiceClient(amConn)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	getRetailerReq := &agentmgmtpb.GetRetailerRequest{Id: retailerID}
	retailerResp, err := amClient.GetRetailer(ctx, getRetailerReq)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return router.ErrorResponse(w, http.StatusNotFound, "Retailer not found")
		}
		return h.handleGRPCError(w, err)
	}

	if retailerResp.AgentId != agentID {
		h.log.Warn("Agent attempted to access retailer from different agent",
			"agent_id", agentID,
			"retailer_id", retailerID,
			"retailer_agent_id", retailerResp.AgentId)
		return router.ErrorResponse(w, http.StatusForbidden, "You don't have permission to view this retailer")
	}

	posReq := &agentmgmtpb.ListPOSDevicesRequest{
		PageSize: 100,
		Page:     1,
		Filter: &agentmgmtpb.POSDeviceFilter{
			RetailerId: retailerID,
		},
	}

	posResp, err := amClient.ListPOSDevices(ctx, posReq)
	if err != nil {
		h.log.Error("Failed to list POS devices", "error", err)
		return h.handleGRPCError(w, err)
	}

	devices := make([]map[string]any, 0, len(posResp.Devices))
	for _, device := range posResp.Devices {
		deviceMap := map[string]any{
			"id":              device.Id,
			"deviceId":        device.DeviceCode,
			"emiNumber":       device.Imei,
			"status":          convertDeviceStatusToString(device.Status),
			"softwareVersion": device.SoftwareVersion,
		}

		if device.LastSync != nil {
			deviceMap["lastSync"] = device.LastSync.AsTime().Format("2006-01-02T15:04:05Z")
		}
		if device.LastTransaction != nil {
			deviceMap["lastTransaction"] = device.LastTransaction.AsTime().Format("2006-01-02T15:04:05Z")
		}

		if device.CreatedAt != nil {
			deviceMap["assignmentDate"] = device.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z")
		}

		devices = append(devices, deviceMap)
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    devices,
		"count":   len(devices),
	})
}

// GetRetailerPerformance handles getting performance metrics for a retailer
func (h *retailerHandlerImpl) GetRetailerPerformance(w http.ResponseWriter, r *http.Request) error {
	retailerID := router.GetParam(r, "id")
	if retailerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Retailer ID is required")
	}

	// Get agent ID from JWT context
	agentID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || agentID == "" {
		h.log.Error("Agent ID not found in context")
		return router.ErrorResponse(w, http.StatusUnauthorized, "Agent authentication required")
	}

	// Verify retailer belongs to agent
	amConn, err := h.grpcManager.GetConnection("agent-management")
	if err != nil {
		h.log.Error("Failed to get agent management connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	amClient := agentmgmtpb.NewAgentManagementServiceClient(amConn)

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Verify retailer ownership
	getRetailerReq := &agentmgmtpb.GetRetailerRequest{Id: retailerID}
	retailerResp, err := amClient.GetRetailer(ctx, getRetailerReq)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return router.ErrorResponse(w, http.StatusNotFound, "Retailer not found")
		}
		return h.handleGRPCError(w, err)
	}

	if retailerResp.AgentId != agentID {
		return router.ErrorResponse(w, http.StatusForbidden, "You don't have permission to view this retailer")
	}

	// Get ticket service client
	ticketClient, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get ticket connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}

	// Calculate date ranges
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	// Week start (Monday of current week)
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday becomes 7
	}
	weekStart := now.AddDate(0, 0, -weekday+1)
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, time.UTC)

	// Fetch tickets in parallel for month and week
	var wg sync.WaitGroup
	var monthSales, weekSales float64
	var monthTickets, weekTickets int64
	var monthErr, weekErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		monthReq := &ticketv1.ListTicketsRequest{
			Page:     1,
			PageSize: 10000, // Large page size to get all tickets
			Filter: &ticketv1.TicketFilter{
				IssuerType: "pos",      // POS tickets are issued by retailers
				IssuerId:   retailerID, // Use retailer ID (UUID) as issuer_id
				StartDate:  timestamppb.New(monthStart),
				EndDate:    timestamppb.New(now),
			},
		}
		monthResp, err := ticketClient.ListTickets(ctx, monthReq)
		if err != nil {
			monthErr = err
		} else {
			// Sum total_amount from tickets
			for _, ticket := range monthResp.Tickets {
				monthSales += float64(ticket.TotalAmount) / 100.0 // Convert pesewas to GHS
				monthTickets++
			}
		}
	}()

	go func() {
		defer wg.Done()
		weekReq := &ticketv1.ListTicketsRequest{
			Page:     1,
			PageSize: 10000,
			Filter: &ticketv1.TicketFilter{
				IssuerType: "pos",
				IssuerId:   retailerID, // Use retailer ID (UUID) as issuer_id
				StartDate:  timestamppb.New(weekStart),
				EndDate:    timestamppb.New(now),
			},
		}
		weekResp, err := ticketClient.ListTickets(ctx, weekReq)
		if err != nil {
			weekErr = err
		} else {
			for _, ticket := range weekResp.Tickets {
				weekSales += float64(ticket.TotalAmount) / 100.0
				weekTickets++
			}
		}
	}()

	wg.Wait()

	if monthErr != nil {
		h.log.Warn("Failed to get month sales", "error", monthErr)
		monthSales = 0.0
		monthTickets = 0
	}
	if weekErr != nil {
		h.log.Warn("Failed to get week sales", "error", weekErr)
		weekSales = 0.0
		weekTickets = 0
	}

	// Calculate average ticket
	var avgTicketMonth, avgTicketWeek float64
	if monthTickets > 0 {
		avgTicketMonth = monthSales / float64(monthTickets)
	}
	if weekTickets > 0 {
		avgTicketWeek = weekSales / float64(weekTickets)
	}

	// Use week average if available, otherwise month average
	avgTicket := avgTicketWeek
	if avgTicket == 0 && monthTickets > 0 {
		avgTicket = avgTicketMonth
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"total_sales": map[string]any{
				"this_month": monthSales,
				"this_week":  weekSales,
				"currency":   "GHS",
			},
			"avg_ticket": map[string]any{
				"amount":   avgTicket,
				"currency": "GHS",
			},
		},
	})
}

// CreatePOSRequest handles creating a POS request (not implemented yet)
func (h *retailerHandlerImpl) CreatePOSRequest(w http.ResponseWriter, r *http.Request) error {
	return router.ErrorResponse(w, http.StatusNotImplemented, "Not implemented")
}

// GetPOSRequest handles getting POS request status (not implemented yet)
func (h *retailerHandlerImpl) GetPOSRequest(w http.ResponseWriter, r *http.Request) error {
	return router.ErrorResponse(w, http.StatusNotImplemented, "Not implemented")
}

// ListPOSRequests handles listing POS requests (not implemented yet)
func (h *retailerHandlerImpl) ListPOSRequests(w http.ResponseWriter, r *http.Request) error {
	return router.ErrorResponse(w, http.StatusNotImplemented, "Not implemented")
}

func (h *retailerHandlerImpl) handleGRPCError(w http.ResponseWriter, err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Internal server error")
	}

	switch st.Code() {
	case codes.NotFound:
		return router.ErrorResponse(w, http.StatusNotFound, st.Message())
	case codes.InvalidArgument:
		return router.ErrorResponse(w, http.StatusBadRequest, st.Message())
	case codes.AlreadyExists:
		return router.ErrorResponse(w, http.StatusConflict, st.Message())
	case codes.PermissionDenied:
		return router.ErrorResponse(w, http.StatusForbidden, st.Message())
	case codes.Unauthenticated:
		return router.ErrorResponse(w, http.StatusUnauthorized, st.Message())
	case codes.ResourceExhausted:
		return router.ErrorResponse(w, http.StatusTooManyRequests, st.Message())
	case codes.Unavailable:
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service temporarily unavailable")
	default:
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to process request")
	}
}

// generateSimplePIN generates a random 4-digit PIN
func generateSimplePIN() string {
	// Generate a random 4-digit number
	pin := rand.Intn(9000) + 1000 // Ensures 1000-9999 range
	return fmt.Sprintf("%04d", pin)
}

func (h *retailerHandlerImpl) GetAgentRetailersSummary(w http.ResponseWriter, r *http.Request) error {
	//Total Retailers
	//Active
	//Total Balance
	//Month Commission
	return nil
}

func (h *retailerHandlerImpl) GetRetailerWinningTickets(w http.ResponseWriter, r *http.Request) error {
	retailerID := router.GetParam(r, "id")
	if retailerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Retailer ID is required")
	}

	agentID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || agentID == "" {
		h.log.Error("Agent ID not found in context")
		return router.ErrorResponse(w, http.StatusUnauthorized, "Agent authentication required")
	}

	amConn, err := h.grpcManager.GetConnection("agent-management")
	if err != nil {
		h.log.Error("Failed to get agent management connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}
	amClient := agentmgmtpb.NewAgentManagementServiceClient(amConn)

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	getRetailerReq := &agentmgmtpb.GetRetailerRequest{Id: retailerID}
	retailerResp, err := amClient.GetRetailer(ctx, getRetailerReq)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return router.ErrorResponse(w, http.StatusNotFound, "Retailer not found")
		}
		return h.handleGRPCError(w, err)
	}

	if retailerResp.AgentId != agentID {
		return router.ErrorResponse(w, http.StatusForbidden, "You don't have permission to view this retailer")
	}

	ticketClient, err := h.grpcManager.TicketServiceClient()
	if err != nil {
		h.log.Error("Failed to get wallet connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable")
	}

	winningTicketsReq := &ticketv1.ListTicketsRequest{
		Filter: &ticketv1.TicketFilter{
			IssuerType: "pos",
			IssuerId:   retailerID,
			Status:     "won",
		},
		Page:     1,
		PageSize: 100,
	}
	winningTicketsResp, err := ticketClient.ListTickets(ctx, winningTicketsReq)
	if err != nil {
		h.log.Error("Failed to get winning tickets", "error", err, "retailer_id", retailerID)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve winning tickets")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"data":      winningTicketsResp.Tickets,
		"total":     winningTicketsResp.Total,
		"page":      winningTicketsResp.Page,
		"page_size": winningTicketsResp.PageSize,
	})
}

func (h *retailerHandlerImpl) GetAssignedTerminal(w http.ResponseWriter, r *http.Request) error {
	retailerID := router.GetUserID(r)
	if retailerID == "" {
		return response.UnauthorizedError(w, "Authorization required")
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.GetTerminalByRetailer(ctx, &terminalpb.GetTerminalByRetailerRequest{
		RetailerId:      retailerID,
		IncludeInactive: false,
	})
	if err != nil {
		h.log.Error("Failed to get terminal", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to get terminal")
	}

	if len(resp.Terminals) == 0 {
		return response.NotFoundError(w, "No terminal assigned to this retailer")
	}

	assignedTerminal := resp.Terminals[0]
	terminalID := assignedTerminal.Terminal.Id

	configResp, err := client.GetTerminalConfig(ctx, &terminalpb.GetTerminalConfigRequest{
		TerminalId: terminalID,
	})
	if err != nil {
		h.log.Error("Failed to get terminal config", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to get terminal config")
	}

	healthResp, err := client.GetTerminalHealth(ctx, &terminalpb.GetTerminalHealthRequest{
		TerminalId: terminalID,
	})
	if err != nil {
		h.log.Error("Failed to get terminal health", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to get terminal health")
	}

	return response.Success(w, http.StatusOK, "Terminal retrieved successfully", map[string]interface{}{
		"terminal":      h.convertTerminal(assignedTerminal.Terminal),
		"configuration": configResp.Config,
		"health":        healthResp.Health,
	})
}

func (h *retailerHandlerImpl) UpdateHeartbeat(w http.ResponseWriter, r *http.Request) error {

	retailerID := router.GetUserID(r)
	if retailerID == "" {
		return response.UnauthorizedError(w, "")
	}

	var req struct {
		BatteryLevel     int32 `json:"battery_level"`
		SignalStrength   int32 `json:"signal_strength"`
		StorageAvailable int64 `json:"storage_available"`
		// AppVersion       string `json:"app_version"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{
			"error": err.Error(),
		})
	}

	if req.BatteryLevel < 0 || req.BatteryLevel > 100 {
		return response.ValidationError(w, "Battery level must be between 0 and 100", nil)
	}

	if req.SignalStrength < 0 {
		return response.ValidationError(w, "Signal strength cannot be below 0", nil)
	}

	if req.StorageAvailable < 0 {
		return response.ValidationError(w, "Storage available cannot be below 0", nil)
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	assignResp, err := client.GetTerminalByRetailer(ctx, &terminalv1.GetTerminalByRetailerRequest{
		RetailerId:      retailerID,
		IncludeInactive: false,
	})
	if err != nil {
		h.log.Error("Failed to get assigned terminal", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to get terminal assignment")
	}

	if assignResp == nil || len(assignResp.Terminals) == 0 {
		return response.ForbiddenError(w, "No terminals assigned to this retailer")
	}

	assignedTerminal := assignResp.Terminals[0]
	terminalID := assignedTerminal.Terminal.Id

	_, err = client.UpdateTerminalHealth(ctx, &terminalpb.UpdateTerminalHealthRequest{
		TerminalId:       terminalID,
		BatteryLevel:     req.BatteryLevel,
		SignalStrength:   req.SignalStrength,
		StorageAvailable: req.StorageAvailable,
		// AppVersion:       req.AppVersion,
	})
	if err != nil {
		h.log.Error("Failed to update terminal heartbeat", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to update terminal heartbeat")
	}

	return response.Success(w, http.StatusOK, "Terminal heartbeat updated successfully", map[string]interface{}{
		"terminal_id":       terminalID,
		"last_heartbeat":    time.Now().UTC(),
		"battery_level":     req.BatteryLevel,
		"signal_strength":   req.SignalStrength,
		"storage_available": req.StorageAvailable,
		// "app_version":       req.AppVersion,
	})
}

func (h *retailerHandlerImpl) GetAssignedTerminalConfig(w http.ResponseWriter, r *http.Request) error {
	retailerID := router.GetUserID(r)
	if retailerID == "" {
		return response.UnauthorizedError(w, "")
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	assignResp, err := client.GetTerminalByRetailer(ctx, &terminalv1.GetTerminalByRetailerRequest{
		RetailerId:      retailerID,
		IncludeInactive: false,
	})
	if err != nil {
		h.log.Error("Failed to get assigned terminals", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to get terminal assignment")
	}

	if assignResp == nil || len(assignResp.Terminals) == 0 {
		return response.ForbiddenError(w, "No terminals assigned to this retailer")
	}

	assignedTerminal := assignResp.Terminals[0]
	terminalID := assignedTerminal.Terminal.Id

	resp, err := client.GetTerminalConfig(ctx, &terminalpb.GetTerminalConfigRequest{
		TerminalId: terminalID,
	})
	if err != nil {
		h.log.Error("Failed to get terminal config", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to get terminal config")
	}

	return response.Success(w, http.StatusOK, "Terminal configuration retrieved successfully", map[string]interface{}{
		"terminal_id": terminalID,
		"config":      resp.Config,
	})
}

func (h *retailerHandlerImpl) convertTerminal(t *terminalpb.Terminal) map[string]interface{} {
	if t == nil {
		return nil
	}

	result := map[string]interface{}{
		"id":              t.Id,
		"device_id":       t.DeviceId,
		"name":            t.Name,
		"model":           t.Model,
		"serial_number":   t.SerialNumber,
		"imei":            t.Imei,
		"android_version": t.AndroidVersion,
		"vendor":          t.Vendor,
		"status":          t.Status,
		"created_at":      t.CreatedAt,
		"updated_at":      t.UpdatedAt,
	}

	if t.RetailerId != "" {
		result["retailer_id"] = t.RetailerId
		result["assignment_date"] = t.AssignmentDate
	}

	return result
}

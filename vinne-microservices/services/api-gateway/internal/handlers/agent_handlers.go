package handlers

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	agentauthv1 "github.com/randco/randco-microservices/proto/agent/auth/v1"
	agentmgmtpb "github.com/randco/randco-microservices/proto/agent/management/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateAgent handles creating a new agent with orchestration between management and auth services
func (h *agentHandlerImpl) CreateAgent(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name                 string   `json:"name"`
		Email                string   `json:"email"`
		PhoneNumber          string   `json:"phone_number"`
		Password             string   `json:"password"`
		Address              string   `json:"address"`
		CommissionPercentage *float64 `json:"commission_percentage"`
		CreatedBy            string   `json:"created_by"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Validate required fields
	req.Name = strings.TrimSpace(req.Name)
	req.PhoneNumber = strings.TrimSpace(req.PhoneNumber)
	req.Email = strings.TrimSpace(req.Email)
	req.CreatedBy = strings.TrimSpace(req.CreatedBy)
	if req.Name == "" || req.PhoneNumber == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Name and phone number are required")
	}
	if req.CreatedBy == "" {
		if userEmail, ok := r.Context().Value(router.ContextEmail).(string); ok && strings.TrimSpace(userEmail) != "" {
			req.CreatedBy = strings.TrimSpace(userEmail)
		} else if userID, ok := r.Context().Value("user_id").(string); ok && strings.TrimSpace(userID) != "" {
			req.CreatedBy = strings.TrimSpace(userID)
		} else {
			req.CreatedBy = "admin"
		}
	}

	// Generate simple password if not provided
	if req.Password == "" {
		// Check if we're in local development environment using config
		env := h.config.Tracing.Environment
		if env == "local" || env == "development" || env == "" {
			// Use hardcoded password for local testing
			req.Password = "123456"
			h.log.Info("Using hardcoded password for local development", "phone", req.PhoneNumber, "password", req.Password)
		} else {
			// Generate random password for non-local environments
			req.Password = generateSimplePassword()
			// TODO: Send password via SMS to req.PhoneNumber
			h.log.Info("Generated password for agent", "phone", req.PhoneNumber, "password", req.Password)
		}
	}

	// Orchestration: Create agent profile first, then auth credentials
	return h.createAgentWithAuth(w, r, req)
}

// GetAgent handles getting an agent by ID
func (h *agentHandlerImpl) GetAgent(w http.ResponseWriter, r *http.Request) error {
	agentID := router.GetParam(r, "id")
	if agentID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Agent ID is required")
	}

	client, err := h.grpcManager.AgentManagementClient()
	if err != nil {
		h.log.Error("Failed to get agent management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Agent management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentmgmtpb.GetAgentRequest{
		Id: agentID,
	}

	resp, err := client.GetAgent(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Get agent failed", "agent_id", agentID, "error", err)
		return router.ErrorResponse(w, http.StatusNotFound, "Agent not found")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"agent": convertAgentToMap(resp),
	})
}

// GetAgentByCode handles getting an agent by code
func (h *agentHandlerImpl) GetAgentByCode(w http.ResponseWriter, r *http.Request) error {
	agentCode := router.GetParam(r, "code")
	if agentCode == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Agent code is required")
	}

	client, err := h.grpcManager.AgentManagementClient()
	if err != nil {
		h.log.Error("Failed to get agent management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Agent management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentmgmtpb.GetAgentByCodeRequest{
		AgentCode: agentCode,
	}

	resp, err := client.GetAgentByCode(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Get agent by code failed", "agent_code", agentCode, "error", err)
		return router.ErrorResponse(w, http.StatusNotFound, "Agent not found")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"agent": convertAgentToMap(resp),
	})
}

// ListAgents handles listing agents with pagination and filters
func (h *agentHandlerImpl) ListAgents(w http.ResponseWriter, r *http.Request) error {
	// Parse query parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	client, err := h.grpcManager.AgentManagementClient()
	if err != nil {
		h.log.Error("Failed to get agent management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Agent management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &agentmgmtpb.ListAgentsRequest{
		Page:     int32(page),
		PageSize: int32(pageSize),
	}

	// Create filter
	filter := &agentmgmtpb.AgentFilter{}

	if status := r.URL.Query().Get("status"); status != "" {
		// Convert status string to enum
		filter.Status = convertStatusStringToEnum(status)
	}
	if name := r.URL.Query().Get("name"); name != "" {
		filter.Name = name
	}
	if email := r.URL.Query().Get("email"); email != "" {
		filter.Email = email
	}

	grpcReq.Filter = filter

	resp, err := client.ListAgents(ctx, grpcReq)
	if err != nil {
		h.log.Error("List agents failed", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to list agents")
	}

	agents := make([]map[string]interface{}, len(resp.Agents))
	for i, agent := range resp.Agents {
		agents[i] = convertAgentToMap(agent)
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"data": agents,
		"pagination": map[string]interface{}{
			"total_count": resp.TotalCount,
			"page":        resp.Page,
			"page_size":   resp.PageSize,
		},
	})
}

// UpdateAgent handles updating an agent
func (h *agentHandlerImpl) UpdateAgent(w http.ResponseWriter, r *http.Request) error {
	agentID := router.GetParam(r, "id")
	if agentID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Agent ID is required")
	}

	// Extract user email from JWT context
	userEmail, ok := r.Context().Value(router.ContextEmail).(string)
	if !ok || userEmail == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "User information not available")
	}

	var req struct {
		Name                 *string  `json:"name"`
		Email                *string  `json:"email"`
		PhoneNumber          *string  `json:"phone_number"`
		Address              *string  `json:"address"`
		CommissionPercentage *float64 `json:"commission_percentage"`
		// UpdatedBy is no longer needed in request - extracted from JWT
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	client, err := h.grpcManager.AgentManagementClient()
	if err != nil {
		h.log.Error("Failed to get agent management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Agent management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &agentmgmtpb.UpdateAgentRequest{
		Id:        agentID,
		UpdatedBy: userEmail, // Use email from JWT
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
	if req.CommissionPercentage != nil {
		grpcReq.CommissionPercentage = *req.CommissionPercentage
	}

	resp, err := client.UpdateAgent(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Update agent failed", "agent_id", agentID, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to update agent")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Agent updated successfully",
		"agent":   convertAgentToMap(resp),
	})
}

// UpdateAgentStatus handles updating agent status
func (h *agentHandlerImpl) UpdateAgentStatus(w http.ResponseWriter, r *http.Request) error {
	agentID := router.GetParam(r, "id")
	if agentID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Agent ID is required")
	}

	// Extract user email from JWT context
	userEmail, ok := r.Context().Value(router.ContextEmail).(string)
	if !ok || userEmail == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "User information not available")
	}

	var req struct {
		Status string `json:"status"`
		// UpdatedBy is no longer needed in request - extracted from JWT
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Normalize status to lowercase for consistency
	req.Status = strings.ToLower(req.Status)

	client, err := h.grpcManager.AgentManagementClient()
	if err != nil {
		h.log.Error("Failed to get agent management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Agent management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentmgmtpb.UpdateAgentStatusRequest{
		Id:        agentID,
		Status:    convertStatusStringToEnum(req.Status),
		UpdatedBy: userEmail, // Use email from JWT
	}

	_, err = client.UpdateAgentStatus(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Update agent status failed", "agent_id", agentID, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to update agent status")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Agent status updated successfully",
	})
}

// GetAgentCommissions retrieves commission data for an agent
func (h *agentHandlerImpl) GetAgentCommissions(w http.ResponseWriter, r *http.Request) error {
	agentID := router.GetParam(r, "id")
	if agentID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Agent ID is required")
	}

	// TODO: Implement actual commission data retrieval when the service is ready
	// For now, return placeholder data to prevent frontend errors
	h.log.Debug("Getting agent commissions (placeholder)", "agent_id", agentID)

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"agent_id":              agentID,
			"today_commission":      0,
			"month_commission":      0,
			"total_commission":      0,
			"commission_percentage": 30, // Default percentage
			"transactions":          []interface{}{},
		},
	})
}

// createAgentWithAuth orchestrates creating an agent profile and auth credentials
func (h *agentHandlerImpl) createAgentWithAuth(w http.ResponseWriter, r *http.Request, req struct {
	Name                 string   `json:"name"`
	Email                string   `json:"email"`
	PhoneNumber          string   `json:"phone_number"`
	Password             string   `json:"password"`
	Address              string   `json:"address"`
	CommissionPercentage *float64 `json:"commission_percentage"`
	CreatedBy            string   `json:"created_by"`
}) error {
	// Step 1: Create agent profile in agent-management service
	agentClient, err := h.grpcManager.AgentManagementClient()
	if err != nil {
		h.log.Error("Failed to get agent management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Agent management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Create agent profile
	agentReq := &agentmgmtpb.CreateAgentRequest{
		Name:             req.Name,
		Email:            req.Email,
		PhoneNumber:      req.PhoneNumber,
		Address:          req.Address,
		CreatedBy:        req.CreatedBy,
		OnboardingMethod: "RAND_LOTTERY_LTD_DIRECT", // Default onboarding method for admin-created agents
	}

	if req.CommissionPercentage != nil {
		agentReq.CommissionPercentage = *req.CommissionPercentage
	}

	agentResp, err := agentClient.CreateAgent(ctx, agentReq)
	if err != nil {
		h.log.Error("Create agent profile failed", "name", req.Name, "error", err)
		return h.handleAgentCreateGRPCError(w, err, "Failed to create agent profile")
	}

	// Step 2: Create auth credentials in agent-auth service
	authClient, err := h.grpcManager.AgentAuthClient()
	if err != nil {
		h.log.Error("Failed to get agent auth client", "error", err)
		// Rollback: Delete the created agent
		deleteReq := &agentmgmtpb.DeleteAgentRequest{
			Id:        agentResp.Id,
			DeletedBy: req.CreatedBy,
		}
		if _, deleteErr := agentClient.DeleteAgent(ctx, deleteReq); deleteErr != nil {
			h.log.Error("Failed to rollback agent creation", "agent_id", agentResp.Id, "error", deleteErr)
		}
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Agent auth service unavailable")
	}

	authReq := &agentauthv1.CreateAgentAuthRequest{
		AgentId:   agentResp.Id,
		AgentCode: agentResp.AgentCode,
		Email:     req.Email,
		Phone:     req.PhoneNumber,
		Password:  req.Password,
		CreatedBy: req.CreatedBy,
	}

	authResp, err := authClient.CreateAgentAuth(ctx, authReq)
	if err != nil {
		h.log.Error("Create agent auth failed", "agent_id", agentResp.Id, "error", err)
		// Rollback: Delete the created agent
		deleteReq := &agentmgmtpb.DeleteAgentRequest{
			Id:        agentResp.Id,
			DeletedBy: req.CreatedBy,
		}
		if _, deleteErr := agentClient.DeleteAgent(ctx, deleteReq); deleteErr != nil {
			h.log.Error("Failed to rollback agent creation", "agent_id", agentResp.Id, "error", deleteErr)
		}
		return h.handleAgentCreateGRPCError(w, err, "Failed to create agent authentication")
	}

	// Return successful response with agent profile data
	return router.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Agent created successfully",
		"agent":   convertAgentToMap(agentResp),
		"auth": map[string]interface{}{
			"agent_id":   authResp.AgentId,
			"agent_code": authResp.AgentCode,
			"success":    authResp.Success,
		},
	})
}

func (h *agentHandlerImpl) handleAgentCreateGRPCError(w http.ResponseWriter, err error, fallback string) error {
	st, ok := status.FromError(err)
	if !ok {
		return router.ErrorResponse(w, http.StatusInternalServerError, fallback)
	}

	message := st.Message()
	if message == "" {
		message = fallback
	}

	switch st.Code() {
	case codes.InvalidArgument:
		return router.ErrorResponse(w, http.StatusBadRequest, message)
	case codes.AlreadyExists:
		return router.ErrorResponse(w, http.StatusConflict, message)
	case codes.NotFound:
		return router.ErrorResponse(w, http.StatusNotFound, message)
	case codes.Unauthenticated:
		return router.ErrorResponse(w, http.StatusUnauthorized, message)
	case codes.PermissionDenied:
		return router.ErrorResponse(w, http.StatusForbidden, message)
	case codes.Unavailable:
		return router.ErrorResponse(w, http.StatusServiceUnavailable, message)
	default:
		return router.ErrorResponse(w, http.StatusInternalServerError, message)
	}
}

// generateSimplePassword generates a simple 6-digit password
func generateSimplePassword() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

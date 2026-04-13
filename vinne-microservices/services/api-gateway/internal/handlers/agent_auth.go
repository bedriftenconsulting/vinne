package handlers

import (
	"context"
	"net"
	"net/http"
	"time"

	agentauthv1 "github.com/randco/randco-microservices/proto/agent/auth/v1"
	agentmgmtpb "github.com/randco/randco-microservices/proto/agent/management/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
)

// AgentAuthHandler handles agent authentication requests
type AgentAuthHandler struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
}

// NewAgentAuthHandler creates a new agent auth handler
func NewAgentAuthHandler(grpcManager *grpc.ClientManager, log logger.Logger) *AgentAuthHandler {
	return &AgentAuthHandler{
		grpcManager: grpcManager,
		log:         log,
	}
}

// Login handles agent login
func (h *AgentAuthHandler) Login(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		AgentCode string `json:"agent_code"` // Can be agent code or email
		Password  string `json:"password"`
		DeviceID  string `json:"device_id,omitempty"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Get gRPC client
	client, err := h.grpcManager.AgentAuthClient()
	if err != nil {
		h.log.Error("Failed to get agent auth client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Authentication service unavailable")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Extract IP address from RemoteAddr (removes port)
	clientIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		clientIP = host
	}

	grpcReq := &agentauthv1.LoginRequest{
		AgentCode: req.AgentCode,
		Password:  req.Password,
		DeviceId:  req.DeviceID,
		IpAddress: clientIP,
		UserAgent: r.UserAgent(),
	}

	resp, err := client.Login(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Agent login failed", "agent_code", req.AgentCode, "error", err)
		return router.ErrorResponse(w, http.StatusUnauthorized, "Invalid credentials")
	}

	// Convert response
	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token":      resp.AccessToken,
		"refresh_token":     resp.RefreshToken,
		"expires_in":        resp.ExpiresIn,
		"device_registered": resp.DeviceRegistered,
		"agent": map[string]any{
			"id":         resp.User.Id,
			"agent_code": resp.User.AgentCode,
			"email":      resp.User.Email,
			"phone":      resp.User.Phone,
			"first_name": "", // These will be populated from agent-management service
			"last_name":  "", // These will be populated from agent-management service
			"role":       resp.User.Role.String(),
			"is_active":  resp.User.IsActive,
			"created_at": "",
			"updated_at": "",
		},
	})
}

// Logout handles agent logout
func (h *AgentAuthHandler) Logout(w http.ResponseWriter, r *http.Request) error {
	// Get agent ID from context (added by auth middleware)
	agentID := router.GetUserID(r)
	if agentID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
	}

	// Get refresh token from body
	var req struct {
		RefreshToken string `json:"refresh_token"`
		DeviceID     string `json:"device_id,omitempty"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Get gRPC client
	client, err := h.grpcManager.AgentAuthClient()
	if err != nil {
		h.log.Error("Failed to get agent auth client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Authentication service unavailable")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentauthv1.LogoutRequest{
		AgentId:      agentID,
		RefreshToken: req.RefreshToken,
		DeviceId:     req.DeviceID,
	}

	_, err = client.Logout(ctx, grpcReq)
	if err != nil {
		h.log.Error("Agent logout failed", "agent_id", agentID, "error", err)
		// Still return success to client
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Logged out successfully",
	})
}

// RefreshToken handles token refresh
func (h *AgentAuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Get gRPC client
	client, err := h.grpcManager.AgentAuthClient()
	if err != nil {
		h.log.Error("Failed to get agent auth client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Authentication service unavailable")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentauthv1.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	}

	resp, err := client.RefreshToken(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Token refresh failed", "error", err)
		return router.ErrorResponse(w, http.StatusUnauthorized, "Invalid refresh token")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
		"expires_in":    resp.ExpiresIn,
	})
}

// ChangePassword handles password change
func (h *AgentAuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) error {
	// Get agent ID from context (added by auth middleware)
	agentID := router.GetUserID(r)
	if agentID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Get gRPC client
	client, err := h.grpcManager.AgentAuthClient()
	if err != nil {
		h.log.Error("Failed to get agent auth client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Authentication service unavailable")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentauthv1.ChangePasswordRequest{
		AgentId:         agentID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}

	_, err = client.ChangePassword(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Password change failed", "agent_id", agentID, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, err.Error())
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Password changed successfully",
	})
}

// GetProfile handles getting basic agent auth profile
// Note: For full agent business profile, use agent-management service
func (h *AgentAuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) error {
	// Get agent ID from context (added by auth middleware)
	agentID := router.GetUserID(r)
	if agentID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
	}

	amClient, err := h.grpcManager.AgentManagementClient()
	if err != nil {
		h.log.Error("Failed to get agent management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Agent management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	amResp, err := amClient.GetAgent(ctx, &agentmgmtpb.GetAgentRequest{Id: agentID})
	if err != nil {
		h.log.Error("Failed to get agent", "error", err)
		return router.ErrorResponse(w, http.StatusBadGateway, "Failed to get agent")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"agent": map[string]any{
			"id":         amResp.Id,
			"agent_code": amResp.AgentCode,
			"email":      amResp.Email,
			"phone":      amResp.PhoneNumber,
			"first_name": amResp.Name,
			"last_name":  "",
			"role":       "AGENT",
			"is_active":  amResp.Status == agentmgmtpb.EntityStatus_ENTITY_STATUS_ACTIVE,
			"created_at": amResp.CreatedAt.AsTime(),
			"updated_at": amResp.UpdatedAt.AsTime(),
		},
	})
}

func (h *AgentAuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) error {
	agentID := router.GetUserID(r)
	if agentID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
	}

	var req struct {
		Name        string `json:"first_name"`
		Email       string `json:"email"`
		PhoneNumber string `json:"phone"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		h.log.Error("Failed to read request body", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.Name == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Name is required")
	}
	if req.Email == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Email is required")
	}
	if req.PhoneNumber == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Phone number is required")
	}

	amClient, err := h.grpcManager.AgentManagementClient()
	if err != nil {
		h.log.Error("Failed to get agent management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Agent management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentmgmtpb.UpdateAgentRequest{
		Id:          agentID,
		Name:        req.Name,
		Email:       req.Email,
		PhoneNumber: req.PhoneNumber,
		UpdatedBy:   agentID,
	}

	resp, err := amClient.UpdateAgent(ctx, grpcReq)
	if err != nil {
		h.log.Error("Failed to update agent", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to update agent")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"agent": resp,
	})
}

// ListDevices handles listing agent devices
func (h *AgentAuthHandler) ListDevices(w http.ResponseWriter, r *http.Request) error {
	// Get agent ID from context (added by auth middleware)
	agentID := router.GetUserID(r)
	if agentID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
	}

	// Get active_only query parameter
	activeOnly := r.URL.Query().Get("active_only") == "true"

	// Get gRPC client
	client, err := h.grpcManager.AgentAuthClient()
	if err != nil {
		h.log.Error("Failed to get agent auth client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Authentication service unavailable")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentauthv1.ListAgentDevicesRequest{
		AgentId:    agentID,
		ActiveOnly: activeOnly,
	}

	resp, err := client.ListAgentDevices(ctx, grpcReq)
	if err != nil {
		h.log.Error("Failed to list agent devices", "agent_id", agentID, "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to list devices")
	}

	// Convert devices to map
	devices := make([]map[string]any, len(resp.Devices))
	for i, device := range resp.Devices {
		devices[i] = map[string]any{
			"id":            device.Id,
			"user_id":       device.UserId,
			"user_type":     device.UserType,
			"device_name":   device.DeviceName,
			"device_type":   device.DeviceType,
			"imei":          device.Imei,
			"status":        device.Status,
			"is_active":     device.IsActive,
			"last_used":     device.LastUsed.AsTime(),
			"registered_at": device.RegisteredAt.AsTime(),
			"updated_at":    device.UpdatedAt.AsTime(),
		}
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"devices": devices,
	})
}

// GetPermissions handles getting agent permissions
func (h *AgentAuthHandler) GetPermissions(w http.ResponseWriter, r *http.Request) error {
	// Get agent ID from context (added by auth middleware)
	agentID := router.GetUserID(r)
	if agentID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
	}

	// Get gRPC client
	client, err := h.grpcManager.AgentAuthClient()
	if err != nil {
		h.log.Error("Failed to get agent auth client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Authentication service unavailable")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentauthv1.GetAgentPermissionsRequest{
		AgentId: agentID,
	}

	resp, err := client.GetAgentPermissions(ctx, grpcReq)
	if err != nil {
		h.log.Error("Failed to get agent permissions", "agent_id", agentID, "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to get permissions")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"permissions": resp.Permissions,
	})
}

// RequestPasswordReset handles password reset requests
func (h *AgentAuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) error {

	var req struct {
		Identifier string `json:"identifier"`
		// Channel    string `json:"channel"` // e.g., "email" or "sms"
		IpAddress string `json:"ip_address"`
		UserType  string `json:"user_type"` // e.g., "AGENT" or "RETAILER"
	}

	if err := router.ReadJSON(r, &req); err != nil {
		h.log.Error("Failed to parse JSON body", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	client, err := h.grpcManager.AgentAuthClient()
	if err != nil {
		h.log.Error("Failed to get agent auth client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Authentication service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clientIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		clientIP = host
	}

	grpcReq := &agentauthv1.PasswordResetRequest{
		Identifier: req.Identifier,
		IpAddress:  clientIP,
		UserAgent:  r.UserAgent(),
		UserType:   req.UserType,
	}

	resp, err := client.RequestPasswordReset(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Password reset failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Password change failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"success":            resp.Success,
		"message":            resp.Message,
		"reset_token":        resp.ResetToken,
		"otp_expiry_seconds": resp.OtpExpirySeconds,
	})
}

func (h *AgentAuthHandler) ValidateResetOTP(w http.ResponseWriter, r *http.Request) error {

	var req struct {
		ResetToken string `json:"reset_token"`
		OtpCode    string `json:"otp_code"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Get gRPC client
	client, err := h.grpcManager.AgentAuthClient()
	if err != nil {
		h.log.Error("Failed to get agent auth client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Authentication service unavailable")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentauthv1.ValidateResetOTPRequest{
		ResetToken: req.ResetToken,
		OtpCode:    req.OtpCode,
	}

	resp, err := client.ValidateResetOTP(ctx, grpcReq)
	if err != nil {
		h.log.Debug("OTP validation failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "OTP validation failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"success":            resp.Valid,
		"message":            resp.Message,
		"remaining_attempts": resp.RemainingAttempts,
	})
}

func (h *AgentAuthHandler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) error {

	var req struct {
		ResetToken      string `json:"reset_token"`
		OtpCode         string `json:"otp_code"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.NewPassword != req.ConfirmPassword {
		return router.ErrorResponse(w, http.StatusBadRequest, "Passwords do not match")
	}

	// Get gRPC client
	client, err := h.grpcManager.AgentAuthClient()
	if err != nil {
		h.log.Error("Failed to get agent auth client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Authentication service unavailable")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentauthv1.ConfirmPasswordResetRequest{
		ResetToken:      req.ResetToken,
		OtpCode:         req.OtpCode,
		NewPassword:     req.NewPassword,
		ConfirmPassword: req.ConfirmPassword,
	}

	resp, err := client.ConfirmPasswordReset(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Password reset confirmation failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Password reset failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"success": resp.Success,
		"message": resp.Message,
	})

}

func (h *AgentAuthHandler) ResendPasswordResetOTP(w http.ResponseWriter, r *http.Request) error {

	var req struct {
		Identifier string `json:"identifier"`
		ResetToken string `json:"reset_token"`
		Channel    string `json:"channel"` // e.g., "email" or "sms"
	}
	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// get gRPC client
	client, err := h.grpcManager.AgentAuthClient()
	if err != nil {
		h.log.Error("Failed to get agent auth client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Authentication service unavailable")
	}

	// call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentauthv1.ResendPasswordResetOTPRequest{
		Identifier: req.Identifier,
		ResetToken: req.ResetToken,
		Channel:    req.Channel,
	}

	resp, err := client.ResendPasswordResetOTP(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Resend OTP failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Resend OTP failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"success":             resp.Success,
		"message":             resp.Message,
		"otp_expiry_seconds":  resp.OtpExpirySeconds,
		"next_resend_seconds": resp.NextResendSeconds,
	})
}

func (h *AgentAuthHandler) ListAgentSessions(w http.ResponseWriter, r *http.Request) error {

	agentID := router.GetUserID(r)
	if agentID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
	}

	client, err := h.grpcManager.AgentAuthClient()
	if err != nil {
		h.log.Error("Failed to get agent auth client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Authentication service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentauthv1.ListAgentSessionsRequest{
		AgentId: agentID,
	}

	resp, err := client.ListAgentSessions(ctx, grpcReq)
	if err != nil {
		h.log.Error("Failed to list agent sessions", "agent_id", agentID, "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to list sessions")
	}
	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"sessions": resp.Sessions,
	})
}

func (h *AgentAuthHandler) AgentCurrentSession(w http.ResponseWriter, r *http.Request) error {
	agentID := router.GetUserID(r)
	if agentID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
	}

	client, err := h.grpcManager.AgentAuthClient()
	if err != nil {
		h.log.Error("Failed to get agent auth client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Authentication service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &agentauthv1.ListAgentSessionsRequest{
		AgentId: agentID,
	}

	resp, err := client.AgentCurrentSession(ctx, grpcReq)
	if err != nil {
		h.log.Error("Failed to get agent current session", "agent_id", agentID, "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to get current session")
	}
	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"session": resp.Session,
	})
}

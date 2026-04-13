package handlers

import (
	"context"
	"net"
	"net/http"
	"time"

	agentauthv1 "github.com/randco/randco-microservices/proto/agent/auth/v1"
	agentmgmtpb "github.com/randco/randco-microservices/proto/agent/management/v1"
	terminalv1 "github.com/randco/randco-microservices/proto/terminal/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/config"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/response"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/jwt"
	"github.com/randco/randco-microservices/shared/common/logger"
)

// RetailerAuthHandler handles retailer authentication requests
type RetailerAuthHandler struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
	jwtService  jwt.Service
	config      *config.Config
}

// NewRetailerAuthHandler creates a new retailer auth handler
func NewRetailerAuthHandler(grpcManager *grpc.ClientManager, log logger.Logger, jwtService jwt.Service, config *config.Config) *RetailerAuthHandler {
	return &RetailerAuthHandler{
		grpcManager: grpcManager,
		log:         log,
		jwtService:  jwtService,
		config:      config,
	}
}

// POSLogin handles retailer POS login with PIN
func (h *RetailerAuthHandler) POSLogin(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		RetailerCode string `json:"retailer_code"`         // 8-digit retailer ID
		PIN          string `json:"pin"`                   // 4-digit PIN
		DeviceIMEI   string `json:"device_imei,omitempty"` // Optional device IMEI
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Validate retailer code (should be 8 digits)
	if len(req.RetailerCode) != 8 {
		return router.ErrorResponse(w, http.StatusBadRequest, "Retailer code must be 8 digits")
	}

	// Validate PIN (should be 4 digits)
	if len(req.PIN) != 4 {
		return router.ErrorResponse(w, http.StatusBadRequest, "PIN must be 4 digits")
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

	// Ensure we have a valid IP address for the database
	if clientIP == "" || clientIP == "::1" {
		clientIP = "127.0.0.1"
	}

	grpcReq := &agentauthv1.RetailerPOSLoginRequest{
		RetailerCode: req.RetailerCode,
		Pin:          req.PIN,
		DeviceImei:   req.DeviceIMEI,
		IpAddress:    clientIP,
		UserAgent:    r.UserAgent(),
	}

	resp, err := client.RetailerPOSLogin(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Retailer POS login failed", "retailer_code", req.RetailerCode, "error", err)
		return router.ErrorResponse(w, http.StatusUnauthorized, "Invalid credentials")
	}

	if h.config.Terminal.EnableNewAuth {
		terminalClient, err := h.grpcManager.TerminalServiceClient()
		if err != nil {
			return response.ServiceUnavailableError(w, "Terminal")
		}

		assignResp, err := terminalClient.GetTerminalByRetailer(ctx,
			&terminalv1.GetTerminalByRetailerRequest{
				RetailerId:      resp.User.Id,
				IncludeInactive: false,
			})
		if err != nil {
			return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to get terminal assignment")
		}

		isKnown := false
		for _, t := range assignResp.Terminals {
			if t.Terminal.Imei == req.DeviceIMEI {
				isKnown = true
				break
			}
		}

		if !isKnown {
			h.log.Warn("Blocked login from unknown terminal", "imei", req.DeviceIMEI, "retailerID", resp.User.Id)
			return router.ErrorResponse(w, http.StatusForbidden, "Login blocked: unregistered terminal")
		}
	}

	// Now fetch retailer details from agent management service
	retailerInfo, _ := h.getRetailerInfo(ctx, resp.User.Id)

	// Convert response
	response := map[string]interface{}{
		"access_token":      resp.AccessToken,
		"refresh_token":     resp.RefreshToken,
		"expires_in":        resp.ExpiresIn,
		"device_registered": resp.DeviceRegistered,
		"retailer": map[string]interface{}{
			"id":            resp.User.Id,
			"retailer_code": resp.User.AgentCode, // AgentCode field contains retailer code
			"phone":         resp.User.Phone,
			"email":         resp.User.Email,
			"is_active":     resp.User.IsActive,
		},
	}

	// Add retailer name if we got it from agent management service
	if retailerInfo != nil {
		response["retailer"].(map[string]interface{})["name"] = retailerInfo.Name
		response["retailer"].(map[string]interface{})["address"] = retailerInfo.Address
		response["retailer"].(map[string]interface{})["created_at"] = retailerInfo.CreatedAt
		response["retailer"].(map[string]interface{})["updated_at"] = retailerInfo.UpdatedAt
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

// getRetailerInfo fetches retailer details from agent management service
func (h *RetailerAuthHandler) getRetailerInfo(ctx context.Context, retailerID string) (*agentmgmtpb.Retailer, error) {
	// Get agent management client
	conn, err := h.grpcManager.GetConnection("agent-management")
	if err != nil {
		h.log.Error("Failed to get agent management connection", "error", err)
		return nil, err
	}
	client := agentmgmtpb.NewAgentManagementServiceClient(conn)

	// Call GetRetailer
	grpcReq := &agentmgmtpb.GetRetailerRequest{
		Id: retailerID,
	}

	resp, err := client.GetRetailer(ctx, grpcReq)
	if err != nil {
		h.log.Error("Failed to get retailer info", "retailer_id", retailerID, "error", err)
		return nil, err
	}

	return resp, nil
}

// RefreshToken handles retailer token refresh
func (h *RetailerAuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) error {
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

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
		"expires_in":    resp.ExpiresIn,
	})
}

// Logout handles retailer logout
func (h *RetailerAuthHandler) Logout(w http.ResponseWriter, r *http.Request) error {
	// Get retailer ID from context (added by auth middleware)
	retailerID := router.GetUserID(r)
	if retailerID == "" {
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
		AgentId:      retailerID, // Using AgentId field for retailer ID as well
		RefreshToken: req.RefreshToken,
		DeviceId:     req.DeviceID,
	}

	_, err = client.Logout(ctx, grpcReq)
	if err != nil {
		h.log.Error("Retailer logout failed", "retailer_id", retailerID, "error", err)
		// Still return success to client
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Logged out successfully",
	})
}

func (h *RetailerAuthHandler) ChangeRetailerPIN(w http.ResponseWriter, r *http.Request) error {
	// Get retailer ID from context (added by auth middleware)
	retailerID := router.GetUserID(r)
	if retailerID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
	}

	var req struct {
		CurrentPIN    string `json:"current_pin"`     // 4-digit current PIN
		NewPIN        string `json:"new_pin"`         // 4-digit new PIN
		ConfirmNewPIN string `json:"confirm_new_pin"` // 4-digit confirm new PIN
		RetailerCode  string `json:"retailer_code"`
		DeviceIMEI    string `json:"device_imei,omitempty"` // Optional device IMEI
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Request field Validations
	if len(req.RetailerCode) != 8 {
		return router.ErrorResponse(w, http.StatusBadRequest, "Retailer code must be 8 digits")
	}
	if len(req.CurrentPIN) != 4 {
		return router.ErrorResponse(w, http.StatusBadRequest, "Current PIN must be 4 digits")
	}
	if len(req.NewPIN) != 4 {
		return router.ErrorResponse(w, http.StatusBadRequest, "New PIN must be 4 digits")
	}
	if len(req.ConfirmNewPIN) != 4 {
		return router.ErrorResponse(w, http.StatusBadRequest, "Confirm New PIN must be 4 digits")
	}
	if req.ConfirmNewPIN != req.NewPIN {
		return router.ErrorResponse(w, http.StatusBadRequest, "New PIN and Confirm New PIN must match")
	}
	if req.CurrentPIN == req.NewPIN {
		return router.ErrorResponse(w, http.StatusBadRequest, "New PIN and Current PIN should not be the same")
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

	// Ensure we have a valid IP address for the database
	if clientIP == "" || clientIP == "::1" {
		clientIP = "127.0.0.1"
	}

	grpcReq := &agentauthv1.ChangeRetailerPINRequest{
		RetailerCode:  req.RetailerCode,
		RetailerId:    retailerID,
		CurrentPin:    req.CurrentPIN,
		ConfirmNewPin: req.ConfirmNewPIN,
		NewPin:        req.NewPIN,
		DeviceImei:    req.DeviceIMEI,
		IpAddress:     clientIP,
		ChangedBy:     retailerID,
	}

	resp, err := client.ChangeRetailerPIN(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Retailer PIN change failed", "retailer_code", req.RetailerCode, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, err.Error())
	}

	// Convert response
	response := map[string]interface{}{
		"success":              resp.Success,
		"message":              resp.Message,
		"sessions_invalidated": resp.SessionsInvalidated,
	}

	return router.WriteJSON(w, http.StatusOK, response)
}

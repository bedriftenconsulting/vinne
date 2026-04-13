package handlers

import (
	"context"
	"net/http"
	"time"

	adminmgmtpb "github.com/randco/randco-microservices/proto/admin/management/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/response"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AdminAuthHandler handles admin authentication requests with standard responses
type AdminAuthHandler interface {
	Register(w http.ResponseWriter, r *http.Request) error
	Login(w http.ResponseWriter, r *http.Request) error
	RefreshToken(w http.ResponseWriter, r *http.Request) error
	Logout(w http.ResponseWriter, r *http.Request) error
	GetProfile(w http.ResponseWriter, r *http.Request) error
	UpdateProfile(w http.ResponseWriter, r *http.Request) error
	ChangePassword(w http.ResponseWriter, r *http.Request) error
	EnableMFA(w http.ResponseWriter, r *http.Request) error
	VerifyMFA(w http.ResponseWriter, r *http.Request) error
	DisableMFA(w http.ResponseWriter, r *http.Request) error
}

// adminAuthHandlerImpl handles admin authentication requests
type adminAuthHandlerImpl struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
}

// NewAdminAuthHandler creates a new admin auth handler with standard responses
func NewAdminAuthHandler(grpcManager *grpc.ClientManager, log logger.Logger) AdminAuthHandler {
	return &adminAuthHandlerImpl{
		grpcManager: grpcManager,
		log:         log,
	}
}

// Register handles admin registration
func (h *adminAuthHandlerImpl) Register(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Email     string   `json:"email"`
		Username  string   `json:"username"`
		Password  string   `json:"password"`
		FirstName string   `json:"first_name"`
		LastName  string   `json:"last_name"`
		RoleIDs   []string `json:"role_ids"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{
			"error": err.Error(),
		})
	}

	// Get gRPC client
	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return response.ServiceUnavailableError(w, "Admin management")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.CreateAdminUserRequest{
		Email:     req.Email,
		Username:  req.Username,
		Password:  req.Password,
		FirstName: &req.FirstName,
		LastName:  &req.LastName,
		RoleIds:   req.RoleIDs,
	}

	resp, err := client.CreateAdminUser(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Registration failed", "email", req.Email, "error", err)
		return h.handleGRPCError(w, err, "Registration failed")
	}

	// Convert response
	return response.Success(w, http.StatusCreated, "Registration successful", map[string]interface{}{
		"user": map[string]interface{}{
			"id":       resp.User.Id,
			"email":    resp.User.Email,
			"username": resp.User.Username,
		},
	})
}

// Login handles admin login
func (h *adminAuthHandlerImpl) Login(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Email    string  `json:"email"`
		Password string  `json:"password"`
		MFAToken *string `json:"mfa_token,omitempty"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		h.log.Debug("Failed to parse login request", "error", err)
		return response.ValidationError(w, "Invalid request body", map[string]string{
			"error": err.Error(),
		})
	}

	// Get client IP and user agent
	ipAddress := r.Header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}
	userAgent := r.Header.Get("User-Agent")

	// Get gRPC client
	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return response.ServiceUnavailableError(w, "Admin management")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.LoginRequest{
		Email:     req.Email,
		Password:  req.Password,
		IpAddress: ipAddress,
		UserAgent: userAgent,
	}

	if req.MFAToken != nil {
		grpcReq.MfaToken = req.MFAToken
	}

	resp, err := client.Login(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Login failed", "email", req.Email, "error", err)
		return h.handleGRPCError(w, err, "Login failed")
	}

	// Convert response
	responseData := map[string]interface{}{
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
		"user": map[string]interface{}{
			"id":       resp.User.Id,
			"email":    resp.User.Email,
			"username": resp.User.Username,
		},
	}

	if resp.User != nil && resp.User.MfaEnabled {
		responseData["mfa_required"] = true
		return response.Success(w, http.StatusOK, "MFA required", responseData)
	}

	return response.Success(w, http.StatusOK, "Login successful", responseData)
}

// RefreshToken handles token refresh
func (h *adminAuthHandlerImpl) RefreshToken(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{
			"error": err.Error(),
		})
	}

	// Get gRPC client
	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return response.ServiceUnavailableError(w, "Admin management")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	}

	resp, err := client.RefreshToken(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Token refresh failed", "error", err)
		return h.handleGRPCError(w, err, "Token refresh failed")
	}

	// Convert response
	return response.Success(w, http.StatusOK, "Token refreshed successfully", map[string]interface{}{
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
	})
}

// Logout handles admin logout
func (h *adminAuthHandlerImpl) Logout(w http.ResponseWriter, r *http.Request) error {
	// Extract user ID from context (set by auth middleware)
	userID := router.GetUserID(r)
	if userID == "" {
		return response.UnauthorizedError(w, "")
	}

	// Get gRPC client
	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return response.ServiceUnavailableError(w, "Admin management")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.LogoutRequest{
		UserId: userID,
	}

	_, err = client.Logout(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Logout failed", "user_id", userID, "error", err)
		// Log error but don't fail the logout from user's perspective
	}

	return response.Success(w, http.StatusOK, "Logout successful", nil)
}

// GetProfile handles getting admin profile
func (h *adminAuthHandlerImpl) GetProfile(w http.ResponseWriter, r *http.Request) error {
	// Extract user ID from context (set by auth middleware)
	userID := router.GetUserID(r)
	if userID == "" {
		return response.UnauthorizedError(w, "")
	}

	// Get gRPC client
	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return response.ServiceUnavailableError(w, "Admin management")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.GetAdminUserRequest{
		Id: userID,
	}

	resp, err := client.GetAdminUser(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Get profile failed", "user_id", userID, "error", err)
		return h.handleGRPCError(w, err, "Failed to get profile")
	}

	// Convert response
	return response.Success(w, http.StatusOK, "Profile retrieved successfully", map[string]interface{}{
		"user": map[string]interface{}{
			"id":          resp.User.Id,
			"email":       resp.User.Email,
			"username":    resp.User.Username,
			"first_name":  resp.User.FirstName,
			"last_name":   resp.User.LastName,
			"mfa_enabled": resp.User.MfaEnabled,
			"created_at":  resp.User.CreatedAt,
			"updated_at":  resp.User.UpdatedAt,
		},
	})
}

// UpdateProfile handles updating admin profile
func (h *adminAuthHandlerImpl) UpdateProfile(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{
			"error": err.Error(),
		})
	}

	// Extract user ID from context
	userID := router.GetUserID(r)
	if userID == "" {
		return response.UnauthorizedError(w, "")
	}

	// Get gRPC client
	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return response.ServiceUnavailableError(w, "Admin management")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.UpdateAdminUserRequest{
		Id:        userID,
		FirstName: &req.FirstName,
		LastName:  &req.LastName,
	}

	resp, err := client.UpdateAdminUser(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Update profile failed", "user_id", userID, "error", err)
		return h.handleGRPCError(w, err, "Failed to update profile")
	}

	// Convert response
	return response.Success(w, http.StatusOK, "Profile updated successfully", map[string]interface{}{
		"user": map[string]interface{}{
			"id":         resp.User.Id,
			"email":      resp.User.Email,
			"username":   resp.User.Username,
			"first_name": resp.User.FirstName,
			"last_name":  resp.User.LastName,
		},
	})
}

// ChangePassword handles password change
func (h *adminAuthHandlerImpl) ChangePassword(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{
			"error": err.Error(),
		})
	}

	// Extract user ID from context
	userID := router.GetUserID(r)
	if userID == "" {
		return response.UnauthorizedError(w, "")
	}

	// Get gRPC client
	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return response.ServiceUnavailableError(w, "Admin management")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.ChangePasswordRequest{
		UserId:          userID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}

	_, err = client.ChangePassword(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Change password failed", "user_id", userID, "error", err)
		return h.handleGRPCError(w, err, "Failed to change password")
	}

	return response.Success(w, http.StatusOK, "Password changed successfully", nil)
}

// EnableMFA handles enabling multi-factor authentication
func (h *adminAuthHandlerImpl) EnableMFA(w http.ResponseWriter, r *http.Request) error {
	// Extract user ID from context
	userID := router.GetUserID(r)
	if userID == "" {
		return response.UnauthorizedError(w, "")
	}

	// Get gRPC client
	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return response.ServiceUnavailableError(w, "Admin management")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.EnableMFARequest{
		UserId: userID,
	}

	resp, err := client.EnableMFA(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Enable MFA failed", "user_id", userID, "error", err)
		return h.handleGRPCError(w, err, "Failed to enable MFA")
	}

	// Convert response
	return response.Success(w, http.StatusOK, "MFA enabled successfully", map[string]interface{}{
		"secret":       resp.Secret,
		"qr_code_url":  resp.QrCodeUrl,
		"backup_codes": resp.BackupCodes,
	})
}

// VerifyMFA handles MFA token verification
func (h *adminAuthHandlerImpl) VerifyMFA(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Token string `json:"token"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{
			"error": err.Error(),
		})
	}

	// Extract user ID from context
	userID := router.GetUserID(r)
	if userID == "" {
		return response.UnauthorizedError(w, "")
	}

	// Get gRPC client
	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return response.ServiceUnavailableError(w, "Admin management")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.VerifyMFARequest{
		UserId: userID,
		Token:  req.Token,
	}

	_, err = client.VerifyMFA(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Verify MFA failed", "user_id", userID, "error", err)
		return h.handleGRPCError(w, err, "Invalid MFA token")
	}

	return response.Success(w, http.StatusOK, "MFA verified successfully", nil)
}

// DisableMFA handles disabling multi-factor authentication
func (h *adminAuthHandlerImpl) DisableMFA(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Token string `json:"token"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{
			"error": err.Error(),
		})
	}

	// Extract user ID from context
	userID := router.GetUserID(r)
	if userID == "" {
		return response.UnauthorizedError(w, "")
	}

	// Get gRPC client
	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return response.ServiceUnavailableError(w, "Admin management")
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.DisableMFARequest{
		UserId: userID,
		Token:  req.Token,
	}

	_, err = client.DisableMFA(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Disable MFA failed", "user_id", userID, "error", err)
		return h.handleGRPCError(w, err, "Failed to disable MFA")
	}

	return response.Success(w, http.StatusOK, "MFA disabled successfully", nil)
}

// handleGRPCError converts gRPC errors to HTTP responses
func (h *adminAuthHandlerImpl) handleGRPCError(w http.ResponseWriter, err error, defaultMsg string) error {
	st, ok := status.FromError(err)
	if !ok {
		h.log.Error(defaultMsg, "error", err)
		return response.InternalError(w, defaultMsg)
	}

	switch st.Code() {
	case codes.NotFound:
		return response.NotFoundError(w, "User")
	case codes.InvalidArgument:
		return response.ValidationError(w, st.Message(), nil)
	case codes.AlreadyExists:
		return response.ConflictError(w, st.Message())
	case codes.PermissionDenied:
		return response.ForbiddenError(w, st.Message())
	case codes.Unauthenticated:
		return response.UnauthorizedError(w, st.Message())
	case codes.Unavailable:
		return response.ServiceUnavailableError(w, "Admin management")
	default:
		h.log.Error(defaultMsg, "error", err, "code", st.Code())
		return response.InternalError(w, defaultMsg)
	}
}

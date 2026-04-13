package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	adminmgmtpb "github.com/randco/randco-microservices/proto/admin/management/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AdminManagementHandler handles admin management requests
type AdminManagementHandler struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
}

// NewAdminManagementHandler creates a new admin management handler
func NewAdminManagementHandler(grpcManager *grpc.ClientManager, log logger.Logger) *AdminManagementHandler {
	return &AdminManagementHandler{
		grpcManager: grpcManager,
		log:         log,
	}
}

// CreateAdminUser handles creating a new admin user
func (h *adminUserHandlerImpl) CreateAdminUser(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Email       string   `json:"email"`
		Username    string   `json:"username"`
		Password    string   `json:"password"`
		FirstName   *string  `json:"first_name"`
		LastName    *string  `json:"last_name"`
		RoleIDs     []string `json:"role_ids"`
		IPWhitelist []string `json:"ip_whitelist"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.CreateAdminUserRequest{
		Email:       req.Email,
		Username:    req.Username,
		Password:    req.Password,
		RoleIds:     req.RoleIDs,
		IpWhitelist: req.IPWhitelist,
	}

	if req.FirstName != nil {
		grpcReq.FirstName = req.FirstName
	}
	if req.LastName != nil {
		grpcReq.LastName = req.LastName
	}

	var resp *adminmgmtpb.AdminUserResponse
	err = h.grpcManager.ExecuteWithRetry(ctx, "admin-management", func(ctx context.Context) error {
		var callErr error
		resp, callErr = client.CreateAdminUser(ctx, grpcReq)
		return callErr
	})
	if err != nil {
		h.log.Debug("Create admin user failed", "email", req.Email, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to create admin user")
	}

	return router.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Admin user created successfully",
		"user":    convertAdminUserToMap(resp.User),
	})
}

// GetAdminUser handles getting an admin user by ID
func (h *adminUserHandlerImpl) GetAdminUser(w http.ResponseWriter, r *http.Request) error {
	userID := router.GetParam(r, "id")
	if userID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "User ID is required")
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.GetAdminUserRequest{
		Id: userID,
	}

	resp, err := client.GetAdminUser(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Get admin user failed", "user_id", userID, "error", err)
		return router.ErrorResponse(w, http.StatusNotFound, "Admin user not found")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user": convertAdminUserToMap(resp.User),
	})
}

// ListAdminUsers handles listing admin users with pagination and filters
func (h *adminUserHandlerImpl) ListAdminUsers(w http.ResponseWriter, r *http.Request) error {
	// Parse query parameters
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(r.URL.Query().Get("page_size"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.ListAdminUsersRequest{
		Page:     int32(page),
		PageSize: int32(pageSize),
	}

	// Add filters if provided
	if email := r.URL.Query().Get("email"); email != "" {
		grpcReq.Email = &email
	}
	if username := r.URL.Query().Get("username"); username != "" {
		grpcReq.Username = &username
	}
	if roleID := r.URL.Query().Get("role_id"); roleID != "" {
		grpcReq.RoleId = &roleID
	}
	if isActive := r.URL.Query().Get("is_active"); isActive != "" {
		if active, err := strconv.ParseBool(isActive); err == nil {
			grpcReq.IsActive = &active
		}
	}
	if mfaEnabled := r.URL.Query().Get("mfa_enabled"); mfaEnabled != "" {
		if mfa, err := strconv.ParseBool(mfaEnabled); err == nil {
			grpcReq.MfaEnabled = &mfa
		}
	}

	resp, err := client.ListAdminUsers(ctx, grpcReq)
	if err != nil {
		h.log.Error("List admin users failed", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to list admin users")
	}

	users := make([]map[string]interface{}, len(resp.Users))
	for i, user := range resp.Users {
		users[i] = convertAdminUserToMap(user)
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"data": users,
		"pagination": map[string]interface{}{
			"total_count": resp.TotalCount,
			"page":        resp.Page,
			"page_size":   resp.PageSize,
			"total_pages": resp.TotalPages,
		},
	})
}

// UpdateAdminUser handles updating an admin user
func (h *adminUserHandlerImpl) UpdateAdminUser(w http.ResponseWriter, r *http.Request) error {
	userID := router.GetParam(r, "id")
	if userID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "User ID is required")
	}

	var req struct {
		Email       *string  `json:"email"`
		Username    *string  `json:"username"`
		FirstName   *string  `json:"first_name"`
		LastName    *string  `json:"last_name"`
		IPWhitelist []string `json:"ip_whitelist"`
		MFAEnabled  *bool    `json:"mfa_enabled"`
		RoleIDs     []string `json:"role_ids"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	// Debug logging to see what we're receiving - using INFO level to ensure it shows
	h.log.Info("UpdateAdminUser request received", "user_id", userID, "role_ids", req.RoleIDs, "role_ids_len", len(req.RoleIDs))

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.UpdateAdminUserRequest{
		Id:          userID,
		IpWhitelist: req.IPWhitelist,
	}

	if req.Email != nil {
		grpcReq.Email = req.Email
	}
	if req.Username != nil {
		grpcReq.Username = req.Username
	}
	if req.FirstName != nil {
		grpcReq.FirstName = req.FirstName
	}
	if req.LastName != nil {
		grpcReq.LastName = req.LastName
	}
	if req.MFAEnabled != nil {
		grpcReq.MfaEnabled = req.MFAEnabled
	}

	resp, err := client.UpdateAdminUser(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Update admin user failed", "user_id", userID, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to update admin user")
	}

	// Handle role assignments if provided
	h.log.Info("Checking role assignment", "role_ids_nil", req.RoleIDs == nil, "role_ids_len", len(req.RoleIDs))
	if req.RoleIDs != nil {
		h.log.Info("Processing role assignments", "role_ids", req.RoleIDs)
		// Get current user roles to remove them first
		currentUserResp, err := client.GetAdminUser(ctx, &adminmgmtpb.GetAdminUserRequest{Id: userID})
		if err != nil {
			h.log.Debug("Failed to get current user roles", "user_id", userID, "error", err)
		} else {
			// Remove all current roles
			for _, role := range currentUserResp.User.Roles {
				_, err := client.RemoveRole(ctx, &adminmgmtpb.RemoveRoleRequest{
					UserId: userID,
					RoleId: role.Id,
				})
				if err != nil {
					h.log.Debug("Failed to remove role", "user_id", userID, "role_id", role.Id, "error", err)
				}
			}
		}

		// Assign new roles
		for _, roleID := range req.RoleIDs {
			_, err := client.AssignRole(ctx, &adminmgmtpb.AssignRoleRequest{
				UserId: userID,
				RoleId: roleID,
			})
			if err != nil {
				h.log.Debug("Failed to assign role", "user_id", userID, "role_id", roleID, "error", err)
			}
		}

		// Refresh user data to include updated roles
		resp, err = client.GetAdminUser(ctx, &adminmgmtpb.GetAdminUserRequest{Id: userID})
		if err != nil {
			h.log.Debug("Failed to refresh user data", "user_id", userID, "error", err)
		}
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Admin user updated successfully",
		"user":    convertAdminUserToMap(resp.User),
	})
}

// DeleteAdminUser handles deleting an admin user
func (h *adminUserHandlerImpl) DeleteAdminUser(w http.ResponseWriter, r *http.Request) error {
	userID := router.GetParam(r, "id")
	if userID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "User ID is required")
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.DeleteAdminUserRequest{
		Id: userID,
	}

	_, err = client.DeleteAdminUser(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Delete admin user failed", "user_id", userID, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to delete admin user")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Admin user deleted successfully",
	})
}

// ActivateAdminUser handles activating an admin user
func (h *adminUserHandlerImpl) UpdateAdminUserStatus(w http.ResponseWriter, r *http.Request) error {
	userID := router.GetParam(r, "id")
	if userID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "User ID is required")
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.ActivateAdminUserRequest{
		Id: userID,
	}

	resp, err := client.ActivateAdminUser(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Activate admin user failed", "user_id", userID, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to activate admin user")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Admin user activated successfully",
		"user":    convertAdminUserToMap(resp.User),
	})
}

// DeactivateAdminUser handles deactivating an admin user
func (h *adminUserHandlerImpl) DeactivateAdminUser(w http.ResponseWriter, r *http.Request) error {
	userID := router.GetParam(r, "id")
	if userID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "User ID is required")
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.DeactivateAdminUserRequest{
		Id: userID,
	}

	resp, err := client.DeactivateAdminUser(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Deactivate admin user failed", "user_id", userID, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to deactivate admin user")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Admin user deactivated successfully",
		"user":    convertAdminUserToMap(resp.User),
	})
}

// convertUserToMap converts protobuf AdminUser to map for JSON response
// convertUserToMap - moved to admin_handlers.go
// func (h *adminUserHandlerImpl) convertUserToMap(user *adminmgmtpb.AdminUser) map[string]interface{} {
// Role Management Operations

// CreateRole handles creating a new role
func (h *adminRoleHandlerImpl) CreateRole(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		PermissionIDs []string `json:"permission_ids"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.CreateRoleRequest{
		Name:          req.Name,
		Description:   req.Description,
		PermissionIds: req.PermissionIDs,
	}

	resp, err := client.CreateRole(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Create role failed", "name", req.Name, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to create role")
	}

	return router.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Role created successfully",
		"role":    convertRoleToMap(resp.Role),
	})
}

// GetRole handles getting a role by ID
func (h *adminRoleHandlerImpl) GetRole(w http.ResponseWriter, r *http.Request) error {
	roleID := router.GetParam(r, "id")
	if roleID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Role ID is required")
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.GetRoleRequest{
		Id: roleID,
	}

	resp, err := client.GetRole(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Get role failed", "role_id", roleID, "error", err)
		return router.ErrorResponse(w, http.StatusNotFound, "Role not found")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"role": convertRoleToMap(resp.Role),
	})
}

// ListRoles handles listing roles with pagination
func (h *adminRoleHandlerImpl) ListRoles(w http.ResponseWriter, r *http.Request) error {
	// Parse query parameters
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(r.URL.Query().Get("page_size"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.ListRolesRequest{
		Page:     int32(page),
		PageSize: int32(pageSize),
	}

	resp, err := client.ListRoles(ctx, grpcReq)
	if err != nil {
		h.log.Error("List roles failed", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to list roles")
	}

	roles := make([]map[string]interface{}, len(resp.Roles))
	for i, role := range resp.Roles {
		roles[i] = convertRoleToMap(role)
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"data": roles,
		"pagination": map[string]interface{}{
			"total_count": resp.TotalCount,
			"page":        resp.Page,
			"page_size":   resp.PageSize,
			"total_pages": resp.TotalPages,
		},
	})
}

// UpdateRole handles updating a role
func (h *adminRoleHandlerImpl) UpdateRole(w http.ResponseWriter, r *http.Request) error {
	roleID := router.GetParam(r, "id")
	if roleID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Role ID is required")
	}

	var req struct {
		Name          *string  `json:"name"`
		Description   *string  `json:"description"`
		PermissionIDs []string `json:"permission_ids"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.UpdateRoleRequest{
		Id:            roleID,
		PermissionIds: req.PermissionIDs,
	}

	if req.Name != nil {
		grpcReq.Name = req.Name
	}
	if req.Description != nil {
		grpcReq.Description = req.Description
	}

	resp, err := client.UpdateRole(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Update role failed", "role_id", roleID, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to update role")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Role updated successfully",
		"role":    convertRoleToMap(resp.Role),
	})
}

// DeleteRole handles deleting a role
func (h *adminRoleHandlerImpl) DeleteRole(w http.ResponseWriter, r *http.Request) error {
	roleID := router.GetParam(r, "id")
	if roleID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Role ID is required")
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.DeleteRoleRequest{
		Id: roleID,
	}

	_, err = client.DeleteRole(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Delete role failed", "role_id", roleID, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to delete role")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Role deleted successfully",
	})
}

// convertRoleToMap converts protobuf Role to map for JSON response
// convertRoleToMap - moved to admin_handlers.go
// func (h *adminRoleHandlerImpl) convertRoleToMap(role *adminmgmtpb.Role) map[string]interface{} {
// Permission Management Operations

// ListPermissions handles listing permissions with pagination and filters
func (h *adminRoleHandlerImpl) ListPermissions(w http.ResponseWriter, r *http.Request) error {
	// Parse query parameters
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(r.URL.Query().Get("page_size"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.ListPermissionsRequest{
		Page:     int32(page),
		PageSize: int32(pageSize),
	}

	// Add resource filter if provided
	if resource := r.URL.Query().Get("resource"); resource != "" {
		grpcReq.Resource = &resource
	}

	resp, err := client.ListPermissions(ctx, grpcReq)
	if err != nil {
		h.log.Error("List permissions failed", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to list permissions")
	}

	permissions := make([]map[string]interface{}, len(resp.Permissions))
	for i, perm := range resp.Permissions {
		permData := map[string]interface{}{
			"id":       perm.Id,
			"resource": perm.Resource,
			"action":   perm.Action,
		}
		if perm.Description != nil {
			permData["description"] = *perm.Description
		}
		permissions[i] = permData
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"data": permissions,
		"pagination": map[string]interface{}{
			"total_count": resp.TotalCount,
			"page":        resp.Page,
			"page_size":   resp.PageSize,
			"total_pages": resp.TotalPages,
		},
	})
}

// Role Assignment Operations

// AssignRole handles assigning a role to a user
func (h *adminRoleHandlerImpl) AssignRole(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		UserID string `json:"user_id"`
		RoleID string `json:"role_id"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.UserID == "" || req.RoleID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "user_id and role_id are required")
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.AssignRoleRequest{
		UserId: req.UserID,
		RoleId: req.RoleID,
	}

	_, err = client.AssignRole(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Assign role failed", "user_id", req.UserID, "role_id", req.RoleID, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to assign role")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Role assigned successfully",
	})
}

// RemoveRole handles removing a role from a user
func (h *adminRoleHandlerImpl) RemoveRole(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		UserID string `json:"user_id"`
		RoleID string `json:"role_id"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.UserID == "" || req.RoleID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "user_id and role_id are required")
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.RemoveRoleRequest{
		UserId: req.UserID,
		RoleId: req.RoleID,
	}

	_, err = client.RemoveRole(ctx, grpcReq)
	if err != nil {
		h.log.Debug("Remove role failed", "user_id", req.UserID, "role_id", req.RoleID, "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to remove role")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Role removed successfully",
	})
}

// Audit Log Operations

// GetAuditLogs handles getting audit logs with pagination and filters
func (h *adminAuditHandlerImpl) GetAuditLogs(w http.ResponseWriter, r *http.Request) error {
	// Parse query parameters
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(r.URL.Query().Get("page_size"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	client, err := h.grpcManager.AdminManagementClient()
	if err != nil {
		h.log.Error("Failed to get admin management client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Admin management service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &adminmgmtpb.GetAuditLogsRequest{
		Page:     int32(page),
		PageSize: int32(pageSize),
	}

	// Add filters if provided
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		grpcReq.UserId = &userID
	}
	if action := r.URL.Query().Get("action"); action != "" {
		grpcReq.Action = &action
	}
	if resource := r.URL.Query().Get("resource"); resource != "" {
		grpcReq.Resource = &resource
	}

	// Parse date filters if provided
	if startDate := r.URL.Query().Get("start_date"); startDate != "" {
		if t, err := time.Parse(time.RFC3339, startDate); err == nil {
			grpcReq.StartDate = timestamppb.New(t)
		}
	}
	if endDate := r.URL.Query().Get("end_date"); endDate != "" {
		if t, err := time.Parse(time.RFC3339, endDate); err == nil {
			grpcReq.EndDate = timestamppb.New(t)
		}
	}

	resp, err := client.GetAuditLogs(ctx, grpcReq)
	if err != nil {
		h.log.Error("Get audit logs failed", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to get audit logs")
	}

	logs := make([]map[string]interface{}, len(resp.Logs))
	for i, log := range resp.Logs {
		logData := map[string]interface{}{
			"id":              log.Id,
			"admin_user_id":   log.AdminUserId,
			"action":          log.Action,
			"ip_address":      log.IpAddress,
			"user_agent":      log.UserAgent,
			"request_data":    log.RequestData,
			"response_status": log.ResponseStatus,
			"created_at":      log.CreatedAt.AsTime(),
		}

		if log.Resource != nil {
			logData["resource"] = *log.Resource
		}
		if log.ResourceId != nil {
			logData["resource_id"] = *log.ResourceId
		}
		if log.AdminUser != nil {
			logData["admin_user"] = convertAdminUserToMap(log.AdminUser)
		}

		logs[i] = logData
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"logs":        logs,
		"total_count": resp.TotalCount,
		"page":        resp.Page,
		"page_size":   resp.PageSize,
		"total_pages": resp.TotalPages,
	})
}

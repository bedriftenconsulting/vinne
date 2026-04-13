package handlers

import (
	adminmgmtpb "github.com/randco/randco-microservices/proto/admin/management/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/shared/common/logger"
)

// adminUserHandlerImpl handles admin user management requests
type adminUserHandlerImpl struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
}

// NewAdminUserHandler creates a new admin user handler
func NewAdminUserHandler(grpcManager *grpc.ClientManager, log logger.Logger) AdminUserHandler {
	return &adminUserHandlerImpl{
		grpcManager: grpcManager,
		log:         log,
	}
}

// adminRoleHandlerImpl handles admin role management requests
type adminRoleHandlerImpl struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
}

// NewAdminRoleHandler creates a new admin role handler
func NewAdminRoleHandler(grpcManager *grpc.ClientManager, log logger.Logger) AdminRoleHandler {
	return &adminRoleHandlerImpl{
		grpcManager: grpcManager,
		log:         log,
	}
}

// adminAuditHandlerImpl handles admin audit requests
type adminAuditHandlerImpl struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
}

// NewAdminAuditHandler creates a new admin audit handler
func NewAdminAuditHandler(grpcManager *grpc.ClientManager, log logger.Logger) AdminAuditHandler {
	return &adminAuditHandlerImpl{
		grpcManager: grpcManager,
		log:         log,
	}
}

// Helper function to convert admin user proto to map
func convertAdminUserToMap(user *adminmgmtpb.AdminUser) map[string]interface{} {
	if user == nil {
		return nil
	}

	result := map[string]interface{}{
		"id":          user.Id,
		"email":       user.Email,
		"username":    user.Username,
		"first_name":  user.FirstName,
		"last_name":   user.LastName,
		"is_active":   user.IsActive,
		"mfa_enabled": user.MfaEnabled,
		"created_at":  user.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
	}

	if user.UpdatedAt != nil {
		result["updated_at"] = user.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z")
	}

	// Add roles if present
	if len(user.Roles) > 0 {
		roles := make([]map[string]interface{}, len(user.Roles))
		for i, role := range user.Roles {
			roles[i] = convertRoleToMap(role)
		}
		result["roles"] = roles
	}

	if user.LastLogin != nil {
		result["last_login"] = user.LastLogin.AsTime().Format("2006-01-02T15:04:05Z")
	}

	return result
}

// Helper function to convert role proto to map
func convertRoleToMap(role *adminmgmtpb.Role) map[string]interface{} {
	if role == nil {
		return nil
	}

	result := map[string]interface{}{
		"id":          role.Id,
		"name":        role.Name,
		"description": role.Description,
		"created_at":  role.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
	}

	// Add permissions if present
	if len(role.Permissions) > 0 {
		permissions := make([]map[string]interface{}, len(role.Permissions))
		for i, perm := range role.Permissions {
			permissions[i] = map[string]interface{}{
				"id":          perm.Id,
				"resource":    perm.Resource,
				"action":      perm.Action,
				"description": perm.Description,
			}
		}
		result["permissions"] = permissions
	}

	return result
}

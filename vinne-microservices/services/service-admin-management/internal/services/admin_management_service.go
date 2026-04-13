package services

import (
	"context"
	"fmt"
	"log"

	adminmanagementv1 "github.com/randco/randco-microservices/proto/admin/management/v1"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/repositories"
	"google.golang.org/protobuf/types/known/emptypb"
)

// AdminManagementService is the main gRPC service that delegates to internal services
type AdminManagementService struct {
	adminmanagementv1.UnimplementedAdminManagementServiceServer
	authService  AuthService
	userService  UserManagementService
	roleService  RoleManagementService
	auditService AuditService
}

// NewAdminManagementService creates a new admin management gRPC service
func NewAdminManagementService(repos *repositories.Repositories, authService AuthService, kafkaBrokers []string) *AdminManagementService {
	// Create service config for services that need Kafka
	serviceConfig := &ServiceConfig{
		KafkaBrokers: kafkaBrokers,
	}

	// Create internal services
	userService := NewUserManagementService(
		repos.AdminUser,
		repos.AdminUserAuth,
		repos.AdminUserRole,
		repos.Role,
		repos.AuditLog,
	)

	roleService := NewRoleManagementService(
		repos.Role,
		repos.Permission,
		repos.AdminUserRole,
		repos.AuditLog,
		serviceConfig,
	)

	auditService := NewAuditService(repos.AuditLog)

	return &AdminManagementService{
		authService:  authService,
		userService:  userService,
		roleService:  roleService,
		auditService: auditService,
	}
}

// ========== Authentication Methods (Delegated to AuthService) ==========

func (s *AdminManagementService) Login(ctx context.Context, req *adminmanagementv1.LoginRequest) (*adminmanagementv1.LoginResponse, error) {
	log.Printf("[AdminManagementService.Login] Request received for email: %s, IP: %s", req.Email, req.IpAddress)

	if s.authService == nil {
		log.Printf("[AdminManagementService.Login] ERROR: Auth service is nil/not configured")
		return nil, fmt.Errorf("auth service not configured")
	}

	log.Printf("[AdminManagementService.Login] Delegating to authService.Login")
	resp, err := s.authService.Login(ctx, req)

	if err != nil {
		log.Printf("[AdminManagementService.Login] Login failed: %v", err)
		return nil, err
	}

	log.Printf("[AdminManagementService.Login] Login successful for email: %s", req.Email)
	return resp, nil
}

func (s *AdminManagementService) Logout(ctx context.Context, req *adminmanagementv1.LogoutRequest) (*emptypb.Empty, error) {
	if s.authService == nil {
		return nil, fmt.Errorf("auth service not configured")
	}
	return s.authService.Logout(ctx, req)
}

func (s *AdminManagementService) RefreshToken(ctx context.Context, req *adminmanagementv1.RefreshTokenRequest) (*adminmanagementv1.RefreshTokenResponse, error) {
	if s.authService == nil {
		return nil, fmt.Errorf("auth service not configured")
	}
	return s.authService.RefreshToken(ctx, req)
}

func (s *AdminManagementService) ChangePassword(ctx context.Context, req *adminmanagementv1.ChangePasswordRequest) (*emptypb.Empty, error) {
	if s.authService == nil {
		return nil, fmt.Errorf("auth service not configured")
	}
	return s.authService.ChangePassword(ctx, req)
}

func (s *AdminManagementService) EnableMFA(ctx context.Context, req *adminmanagementv1.EnableMFARequest) (*adminmanagementv1.EnableMFAResponse, error) {
	if s.authService == nil {
		return nil, fmt.Errorf("auth service not configured")
	}
	return s.authService.EnableMFA(ctx, req)
}

func (s *AdminManagementService) VerifyMFA(ctx context.Context, req *adminmanagementv1.VerifyMFARequest) (*emptypb.Empty, error) {
	if s.authService == nil {
		return nil, fmt.Errorf("auth service not configured")
	}
	return s.authService.VerifyMFA(ctx, req)
}

func (s *AdminManagementService) DisableMFA(ctx context.Context, req *adminmanagementv1.DisableMFARequest) (*emptypb.Empty, error) {
	if s.authService == nil {
		return nil, fmt.Errorf("auth service not configured")
	}
	return s.authService.DisableMFA(ctx, req)
}

// ========== User Management Methods (Delegated to UserManagementService) ==========

func (s *AdminManagementService) CreateAdminUser(ctx context.Context, req *adminmanagementv1.CreateAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error) {
	if s.userService == nil {
		return nil, fmt.Errorf("user service not configured")
	}
	return s.userService.CreateAdminUser(ctx, req)
}

func (s *AdminManagementService) GetAdminUser(ctx context.Context, req *adminmanagementv1.GetAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error) {
	if s.userService == nil {
		return nil, fmt.Errorf("user service not configured")
	}
	return s.userService.GetAdminUser(ctx, req)
}

func (s *AdminManagementService) UpdateAdminUser(ctx context.Context, req *adminmanagementv1.UpdateAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error) {
	if s.userService == nil {
		return nil, fmt.Errorf("user service not configured")
	}
	return s.userService.UpdateAdminUser(ctx, req)
}

func (s *AdminManagementService) DeleteAdminUser(ctx context.Context, req *adminmanagementv1.DeleteAdminUserRequest) (*emptypb.Empty, error) {
	if s.userService == nil {
		return nil, fmt.Errorf("user service not configured")
	}
	return s.userService.DeleteAdminUser(ctx, req)
}

func (s *AdminManagementService) ActivateAdminUser(ctx context.Context, req *adminmanagementv1.ActivateAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error) {
	if s.userService == nil {
		return nil, fmt.Errorf("user service not configured")
	}
	return s.userService.ActivateAdminUser(ctx, req)
}

func (s *AdminManagementService) DeactivateAdminUser(ctx context.Context, req *adminmanagementv1.DeactivateAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error) {
	if s.userService == nil {
		return nil, fmt.Errorf("user service not configured")
	}
	return s.userService.DeactivateAdminUser(ctx, req)
}

func (s *AdminManagementService) ListAdminUsers(ctx context.Context, req *adminmanagementv1.ListAdminUsersRequest) (*adminmanagementv1.ListAdminUsersResponse, error) {
	if s.userService == nil {
		return nil, fmt.Errorf("user service not configured")
	}
	return s.userService.ListAdminUsers(ctx, req)
}

func (s *AdminManagementService) GetUserByEmail(ctx context.Context, req *adminmanagementv1.GetUserByEmailRequest) (*adminmanagementv1.AdminUserResponse, error) {
	if s.userService == nil {
		return nil, fmt.Errorf("user service not configured")
	}
	return s.userService.GetUserByEmail(ctx, req)
}

func (s *AdminManagementService) VerifyUserCredentials(ctx context.Context, req *adminmanagementv1.VerifyUserCredentialsRequest) (*adminmanagementv1.VerifyUserCredentialsResponse, error) {
	if s.userService == nil {
		return nil, fmt.Errorf("user service not configured")
	}
	return s.userService.VerifyUserCredentials(ctx, req)
}

func (s *AdminManagementService) UpdateLastLogin(ctx context.Context, req *adminmanagementv1.UpdateLastLoginRequest) (*emptypb.Empty, error) {
	if s.userService == nil {
		return nil, fmt.Errorf("user service not configured")
	}
	return s.userService.UpdateLastLogin(ctx, req)
}

// ========== Role Management Methods (Delegated to RoleManagementService) ==========

func (s *AdminManagementService) CreateRole(ctx context.Context, req *adminmanagementv1.CreateRoleRequest) (*adminmanagementv1.RoleResponse, error) {
	if s.roleService == nil {
		return nil, fmt.Errorf("role service not configured")
	}
	return s.roleService.CreateRole(ctx, req)
}

func (s *AdminManagementService) GetRole(ctx context.Context, req *adminmanagementv1.GetRoleRequest) (*adminmanagementv1.RoleResponse, error) {
	if s.roleService == nil {
		return nil, fmt.Errorf("role service not configured")
	}
	return s.roleService.GetRole(ctx, req)
}

func (s *AdminManagementService) UpdateRole(ctx context.Context, req *adminmanagementv1.UpdateRoleRequest) (*adminmanagementv1.RoleResponse, error) {
	if s.roleService == nil {
		return nil, fmt.Errorf("role service not configured")
	}
	return s.roleService.UpdateRole(ctx, req)
}

func (s *AdminManagementService) DeleteRole(ctx context.Context, req *adminmanagementv1.DeleteRoleRequest) (*emptypb.Empty, error) {
	if s.roleService == nil {
		return nil, fmt.Errorf("role service not configured")
	}
	return s.roleService.DeleteRole(ctx, req)
}

func (s *AdminManagementService) ListRoles(ctx context.Context, req *adminmanagementv1.ListRolesRequest) (*adminmanagementv1.ListRolesResponse, error) {
	if s.roleService == nil {
		return nil, fmt.Errorf("role service not configured")
	}
	return s.roleService.ListRoles(ctx, req)
}

func (s *AdminManagementService) AssignRole(ctx context.Context, req *adminmanagementv1.AssignRoleRequest) (*emptypb.Empty, error) {
	if s.roleService == nil {
		return nil, fmt.Errorf("role service not configured")
	}
	return s.roleService.AssignRole(ctx, req)
}

func (s *AdminManagementService) RemoveRole(ctx context.Context, req *adminmanagementv1.RemoveRoleRequest) (*emptypb.Empty, error) {
	if s.roleService == nil {
		return nil, fmt.Errorf("role service not configured")
	}
	return s.roleService.RemoveRole(ctx, req)
}

func (s *AdminManagementService) ListPermissions(ctx context.Context, req *adminmanagementv1.ListPermissionsRequest) (*adminmanagementv1.ListPermissionsResponse, error) {
	if s.roleService == nil {
		return nil, fmt.Errorf("role service not configured")
	}
	return s.roleService.ListPermissions(ctx, req)
}

// ========== Audit Methods (Delegated to AuditService) ==========

func (s *AdminManagementService) GetAuditLogs(ctx context.Context, req *adminmanagementv1.GetAuditLogsRequest) (*adminmanagementv1.GetAuditLogsResponse, error) {
	if s.auditService == nil {
		return nil, fmt.Errorf("audit service not configured")
	}
	return s.auditService.GetAuditLogs(ctx, req)
}

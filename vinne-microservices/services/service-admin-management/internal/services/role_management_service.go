package services

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	adminmanagementv1 "github.com/randco/randco-microservices/proto/admin/management/v1"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/repositories"
	"github.com/randco/randco-microservices/shared/audit"
	"github.com/randco/randco-microservices/shared/common/errors"
	"github.com/randco/randco-microservices/shared/events"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// RoleManagementService handles role and permission management operations
type RoleManagementService interface {
	CreateRole(ctx context.Context, req *adminmanagementv1.CreateRoleRequest) (*adminmanagementv1.RoleResponse, error)
	GetRole(ctx context.Context, req *adminmanagementv1.GetRoleRequest) (*adminmanagementv1.RoleResponse, error)
	UpdateRole(ctx context.Context, req *adminmanagementv1.UpdateRoleRequest) (*adminmanagementv1.RoleResponse, error)
	DeleteRole(ctx context.Context, req *adminmanagementv1.DeleteRoleRequest) (*emptypb.Empty, error)
	ListRoles(ctx context.Context, req *adminmanagementv1.ListRolesRequest) (*adminmanagementv1.ListRolesResponse, error)
	AssignRole(ctx context.Context, req *adminmanagementv1.AssignRoleRequest) (*emptypb.Empty, error)
	RemoveRole(ctx context.Context, req *adminmanagementv1.RemoveRoleRequest) (*emptypb.Empty, error)
	ListPermissions(ctx context.Context, req *adminmanagementv1.ListPermissionsRequest) (*adminmanagementv1.ListPermissionsResponse, error)
}

type roleManagementService struct {
	roleRepo          repositories.RoleRepository
	permissionRepo    repositories.PermissionRepository
	adminUserRoleRepo repositories.AdminUserRoleRepository
	auditRepo         repositories.AuditLogRepository
	auditLogger       audit.Logger
}

// ServiceConfig holds configuration for services
type ServiceConfig struct {
	KafkaBrokers []string
}

// NewRoleManagementService creates a new role management service
func NewRoleManagementService(
	roleRepo repositories.RoleRepository,
	permissionRepo repositories.PermissionRepository,
	adminUserRoleRepo repositories.AdminUserRoleRepository,
	auditRepo repositories.AuditLogRepository,
	config *ServiceConfig,
) RoleManagementService {
	// Initialize event bus for audit logging
	var eventBus events.EventBus
	if config != nil && len(config.KafkaBrokers) > 0 {
		bus, err := events.NewKafkaEventBus(config.KafkaBrokers)
		if err != nil {
			log.Printf("Failed to initialize Kafka event bus for audit: %v, using in-memory event bus", err)
			eventBus = events.NewInMemoryEventBus()
		} else {
			eventBus = bus
		}
	} else {
		eventBus = events.NewInMemoryEventBus()
	}

	// Create audit logger
	auditLogger := audit.NewEventLogger("service-admin-management", eventBus, true)

	return &roleManagementService{
		roleRepo:          roleRepo,
		permissionRepo:    permissionRepo,
		adminUserRoleRepo: adminUserRoleRepo,
		auditRepo:         auditRepo,
		auditLogger:       auditLogger,
	}
}

func (s *roleManagementService) CreateRole(ctx context.Context, req *adminmanagementv1.CreateRoleRequest) (*adminmanagementv1.RoleResponse, error) {
	role := &models.Role{
		Name:        req.Name,
		Description: req.Description,
	}

	// Create role
	err := s.roleRepo.Create(ctx, role)
	if err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	// Add permissions if provided
	if len(req.PermissionIds) > 0 {
		permissionIDs := make([]uuid.UUID, 0, len(req.PermissionIds))
		for _, permIDStr := range req.PermissionIds {
			permID, err := uuid.Parse(permIDStr)
			if err != nil {
				log.Printf("Invalid permission ID: %s", permIDStr)
				continue
			}
			permissionIDs = append(permissionIDs, permID)
		}

		if len(permissionIDs) > 0 {
			err = s.roleRepo.SetRolePermissions(ctx, role.ID, permissionIDs)
			if err != nil {
				log.Printf("Failed to set role permissions: %v", err)
			}
		}
	}

	// Retrieve role with permissions
	fullRole, err := s.roleRepo.GetByID(ctx, role.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve created role: %w", err)
	}

	// Audit log the role creation
	if s.auditLogger != nil {
		_ = s.auditLogger.LogSuccess(ctx, audit.ActionRoleCreated, role.ID.String(), "role",
			map[string]interface{}{
				"role_name":      req.Name,
				"permission_ids": req.PermissionIds,
			})
	}

	return &adminmanagementv1.RoleResponse{
		Role: modelRoleToProto(fullRole),
	}, nil
}

func (s *roleManagementService) GetRole(ctx context.Context, req *adminmanagementv1.GetRoleRequest) (*adminmanagementv1.RoleResponse, error) {
	roleID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, errors.NewBadRequestError("invalid role ID")
	}

	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	return &adminmanagementv1.RoleResponse{
		Role: modelRoleToProto(role),
	}, nil
}

func (s *roleManagementService) UpdateRole(ctx context.Context, req *adminmanagementv1.UpdateRoleRequest) (*adminmanagementv1.RoleResponse, error) {
	roleID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, errors.NewBadRequestError("invalid role ID")
	}

	// Get existing role
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	// Update fields
	if req.Name != nil {
		role.Name = *req.Name
	}
	if req.Description != nil {
		role.Description = *req.Description
	}

	// Update role
	err = s.roleRepo.Update(ctx, role)
	if err != nil {
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	// Update permissions if provided
	if req.PermissionIds != nil {
		permissionIDs := make([]uuid.UUID, 0, len(req.PermissionIds))
		for _, permIDStr := range req.PermissionIds {
			permID, err := uuid.Parse(permIDStr)
			if err != nil {
				log.Printf("Invalid permission ID: %s", permIDStr)
				continue
			}
			permissionIDs = append(permissionIDs, permID)
		}

		err = s.roleRepo.SetRolePermissions(ctx, roleID, permissionIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to update role permissions: %w", err)
		}
	}

	// Retrieve updated role with permissions
	updatedRole, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated role: %w", err)
	}

	return &adminmanagementv1.RoleResponse{
		Role: modelRoleToProto(updatedRole),
	}, nil
}

func (s *roleManagementService) DeleteRole(ctx context.Context, req *adminmanagementv1.DeleteRoleRequest) (*emptypb.Empty, error) {
	roleID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, errors.NewBadRequestError("invalid role ID")
	}

	err = s.roleRepo.Delete(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete role: %w", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *roleManagementService) ListRoles(ctx context.Context, req *adminmanagementv1.ListRolesRequest) (*adminmanagementv1.ListRolesResponse, error) {
	// Build filter
	filter := models.RoleFilter{
		Page:     int(req.Page),
		PageSize: int(req.PageSize),
	}

	// ListRolesRequest doesn't have a Name field in the proto
	// You might want to add search capability later

	// Get roles
	roles, total, err := s.roleRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	// Convert to proto
	protoRoles := make([]*adminmanagementv1.Role, len(roles))
	for i, role := range roles {
		protoRoles[i] = modelRoleToProto(role)
	}

	return &adminmanagementv1.ListRolesResponse{
		Roles:      protoRoles,
		TotalCount: int32(total),
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: int32((total + int(req.PageSize) - 1) / int(req.PageSize)),
	}, nil
}

func (s *roleManagementService) AssignRole(ctx context.Context, req *adminmanagementv1.AssignRoleRequest) (*emptypb.Empty, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, errors.NewBadRequestError("invalid user ID")
	}

	roleID, err := uuid.Parse(req.RoleId)
	if err != nil {
		return nil, errors.NewBadRequestError("invalid role ID")
	}

	err = s.adminUserRoleRepo.AssignRole(ctx, userID, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to assign role: %w", err)
	}

	// Audit log the role assignment
	if s.auditLogger != nil {
		_ = s.auditLogger.LogSuccess(ctx, audit.ActionRoleAssigned, userID.String(), "user",
			map[string]interface{}{
				"user_id": req.UserId,
				"role_id": req.RoleId,
			})
	}

	return &emptypb.Empty{}, nil
}

func (s *roleManagementService) RemoveRole(ctx context.Context, req *adminmanagementv1.RemoveRoleRequest) (*emptypb.Empty, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, errors.NewBadRequestError("invalid user ID")
	}

	roleID, err := uuid.Parse(req.RoleId)
	if err != nil {
		return nil, errors.NewBadRequestError("invalid role ID")
	}

	err = s.adminUserRoleRepo.RemoveRole(ctx, userID, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to remove role: %w", err)
	}

	// Audit log the role removal
	if s.auditLogger != nil {
		_ = s.auditLogger.LogSuccess(ctx, audit.ActionRoleRemoved, userID.String(), "user",
			map[string]interface{}{
				"user_id": req.UserId,
				"role_id": req.RoleId,
			})
	}

	return &emptypb.Empty{}, nil
}

func (s *roleManagementService) ListPermissions(ctx context.Context, req *adminmanagementv1.ListPermissionsRequest) (*adminmanagementv1.ListPermissionsResponse, error) {
	// Build filter
	filter := models.PermissionFilter{
		Page:     int(req.Page),
		PageSize: int(req.PageSize),
	}

	if req.Resource != nil && *req.Resource != "" {
		filter.Resource = *req.Resource
	}

	// Get permissions
	permissions, total, err := s.permissionRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list permissions: %w", err)
	}

	// Convert to proto
	protoPermissions := make([]*adminmanagementv1.Permission, len(permissions))
	for i, perm := range permissions {
		protoPermissions[i] = &adminmanagementv1.Permission{
			Id:          perm.ID.String(),
			Resource:    perm.Resource,
			Action:      perm.Action,
			Description: perm.Description,
		}
	}

	return &adminmanagementv1.ListPermissionsResponse{
		Permissions: protoPermissions,
		TotalCount:  int32(total),
		Page:        req.Page,
		PageSize:    req.PageSize,
		TotalPages:  int32((total + int(req.PageSize) - 1) / int(req.PageSize)),
	}, nil
}

// Helper function to convert model to proto
func modelRoleToProto(role *models.Role) *adminmanagementv1.Role {
	if role == nil {
		return nil
	}

	protoRole := &adminmanagementv1.Role{
		Id:          role.ID.String(),
		Name:        role.Name,
		Description: role.Description,
		CreatedAt:   timestamppb.New(role.CreatedAt),
	}

	// Add permissions
	if len(role.Permissions) > 0 {
		protoRole.Permissions = make([]*adminmanagementv1.Permission, len(role.Permissions))
		for i, perm := range role.Permissions {
			protoRole.Permissions[i] = &adminmanagementv1.Permission{
				Id:          perm.ID.String(),
				Resource:    perm.Resource,
				Action:      perm.Action,
				Description: perm.Description,
			}
		}
	}

	return protoRole
}

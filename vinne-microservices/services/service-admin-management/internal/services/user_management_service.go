package services

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	adminmanagementv1 "github.com/randco/randco-microservices/proto/admin/management/v1"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/repositories"
	"github.com/randco/randco-microservices/shared/common/errors"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// UserManagementService handles user management operations
type UserManagementService interface {
	CreateAdminUser(ctx context.Context, req *adminmanagementv1.CreateAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error)
	GetAdminUser(ctx context.Context, req *adminmanagementv1.GetAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error)
	UpdateAdminUser(ctx context.Context, req *adminmanagementv1.UpdateAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error)
	DeleteAdminUser(ctx context.Context, req *adminmanagementv1.DeleteAdminUserRequest) (*emptypb.Empty, error)
	ActivateAdminUser(ctx context.Context, req *adminmanagementv1.ActivateAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error)
	DeactivateAdminUser(ctx context.Context, req *adminmanagementv1.DeactivateAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error)
	ListAdminUsers(ctx context.Context, req *adminmanagementv1.ListAdminUsersRequest) (*adminmanagementv1.ListAdminUsersResponse, error)
	GetUserByEmail(ctx context.Context, req *adminmanagementv1.GetUserByEmailRequest) (*adminmanagementv1.AdminUserResponse, error)
	VerifyUserCredentials(ctx context.Context, req *adminmanagementv1.VerifyUserCredentialsRequest) (*adminmanagementv1.VerifyUserCredentialsResponse, error)
	UpdateLastLogin(ctx context.Context, req *adminmanagementv1.UpdateLastLoginRequest) (*emptypb.Empty, error)
}

type userManagementService struct {
	adminUserRepo     repositories.AdminUserRepository
	adminUserAuthRepo repositories.AdminUserAuthRepository
	adminUserRoleRepo repositories.AdminUserRoleRepository
	roleRepo          repositories.RoleRepository
	auditRepo         repositories.AuditLogRepository
}

// NewUserManagementService creates a new user management service
func NewUserManagementService(
	adminUserRepo repositories.AdminUserRepository,
	adminUserAuthRepo repositories.AdminUserAuthRepository,
	adminUserRoleRepo repositories.AdminUserRoleRepository,
	roleRepo repositories.RoleRepository,
	auditRepo repositories.AuditLogRepository,
) UserManagementService {
	return &userManagementService{
		adminUserRepo:     adminUserRepo,
		adminUserAuthRepo: adminUserAuthRepo,
		adminUserRoleRepo: adminUserRoleRepo,
		roleRepo:          roleRepo,
		auditRepo:         auditRepo,
	}
}

func (s *userManagementService) CreateAdminUser(ctx context.Context, req *adminmanagementv1.CreateAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error) {
	log.Printf("CreateAdminUser called for email: %s", req.Email)

	// Validate password strength
	if len(req.Password) < 8 {
		return nil, errors.NewBadRequestError("password must be at least 8 characters")
	}

	// Create user model
	user := &models.AdminUser{
		Email:     req.Email,
		Username:  req.Username,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		IsActive:  true,
	}

	// Create user with password
	err := s.adminUserRepo.Create(ctx, user, req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	// Assign roles if provided
	if len(req.RoleIds) > 0 {
		for _, roleIDStr := range req.RoleIds {
			roleID, err := uuid.Parse(roleIDStr)
			if err != nil {
				log.Printf("Invalid role ID: %s", roleIDStr)
				continue
			}

			err = s.adminUserRoleRepo.AssignRole(ctx, user.ID, roleID)
			if err != nil {
				log.Printf("Failed to assign role %s to user %s: %v", roleIDStr, user.ID, err)
			}
		}
	}

	// Retrieve user with roles for response
	fullUser, err := s.adminUserRepo.GetByID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve created user: %w", err)
	}

	return &adminmanagementv1.AdminUserResponse{
		User: userModelToProto(fullUser),
	}, nil
}

func (s *userManagementService) GetAdminUser(ctx context.Context, req *adminmanagementv1.GetAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error) {
	log.Printf("GetAdminUser called for ID: %s", req.Id)

	userID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, errors.NewBadRequestError("invalid user ID")
	}

	user, err := s.adminUserRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin user: %w", err)
	}

	return &adminmanagementv1.AdminUserResponse{
		User: userModelToProto(user),
	}, nil
}

func (s *userManagementService) UpdateAdminUser(ctx context.Context, req *adminmanagementv1.UpdateAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error) {
	log.Printf("UpdateAdminUser called for ID: %s", req.Id)

	userID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, errors.NewBadRequestError("invalid user ID")
	}

	// Get existing user
	user, err := s.adminUserRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin user: %w", err)
	}

	// Update fields
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Username != nil {
		user.Username = *req.Username
	}
	if req.FirstName != nil {
		user.FirstName = req.FirstName
	}
	if req.LastName != nil {
		user.LastName = req.LastName
	}
	if req.MfaEnabled != nil {
		user.MFAEnabled = *req.MfaEnabled
	}

	// Update user
	err = s.adminUserRepo.Update(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to update admin user: %w", err)
	}

	// Retrieve updated user with roles
	updatedUser, err := s.adminUserRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated user: %w", err)
	}

	return &adminmanagementv1.AdminUserResponse{
		User: userModelToProto(updatedUser),
	}, nil
}

func (s *userManagementService) DeleteAdminUser(ctx context.Context, req *adminmanagementv1.DeleteAdminUserRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteAdminUser called for ID: %s", req.Id)

	userID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, errors.NewBadRequestError("invalid user ID")
	}

	err = s.adminUserRepo.Delete(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete admin user: %w", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *userManagementService) ActivateAdminUser(ctx context.Context, req *adminmanagementv1.ActivateAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error) {
	userID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, errors.NewBadRequestError("invalid user ID")
	}

	err = s.adminUserRepo.Activate(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to activate admin user: %w", err)
	}

	// Retrieve updated user
	user, err := s.adminUserRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve activated user: %w", err)
	}

	return &adminmanagementv1.AdminUserResponse{
		User: userModelToProto(user),
	}, nil
}

func (s *userManagementService) DeactivateAdminUser(ctx context.Context, req *adminmanagementv1.DeactivateAdminUserRequest) (*adminmanagementv1.AdminUserResponse, error) {
	userID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, errors.NewBadRequestError("invalid user ID")
	}

	err = s.adminUserRepo.Deactivate(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to deactivate admin user: %w", err)
	}

	// Retrieve updated user
	user, err := s.adminUserRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve deactivated user: %w", err)
	}

	return &adminmanagementv1.AdminUserResponse{
		User: userModelToProto(user),
	}, nil
}

func (s *userManagementService) ListAdminUsers(ctx context.Context, req *adminmanagementv1.ListAdminUsersRequest) (*adminmanagementv1.ListAdminUsersResponse, error) {
	log.Printf("ListAdminUsers called - page: %d, size: %d", req.Page, req.PageSize)

	// Build filter
	filter := models.AdminUserFilter{
		Page:     int(req.Page),
		PageSize: int(req.PageSize),
	}

	if req.IsActive != nil {
		filter.IsActive = req.IsActive
	}

	if req.RoleId != nil && *req.RoleId != "" {
		roleID, err := uuid.Parse(*req.RoleId)
		if err != nil {
			return nil, errors.NewBadRequestError("invalid role ID")
		}
		filter.RoleID = &roleID
	}

	// Get users
	users, total, err := s.adminUserRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list admin users: %w", err)
	}

	// Convert to proto
	protoUsers := make([]*adminmanagementv1.AdminUser, len(users))
	for i, user := range users {
		protoUsers[i] = userModelToProto(user)
	}

	return &adminmanagementv1.ListAdminUsersResponse{
		Users:      protoUsers,
		TotalCount: int32(total),
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: int32((total + int(req.PageSize) - 1) / int(req.PageSize)),
	}, nil
}

func (s *userManagementService) GetUserByEmail(ctx context.Context, req *adminmanagementv1.GetUserByEmailRequest) (*adminmanagementv1.AdminUserResponse, error) {
	log.Printf("GetUserByEmail called for email: %s", req.Email)

	user, err := s.adminUserRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &adminmanagementv1.AdminUserResponse{
		User: userModelToProto(user),
	}, nil
}

func (s *userManagementService) VerifyUserCredentials(ctx context.Context, req *adminmanagementv1.VerifyUserCredentialsRequest) (*adminmanagementv1.VerifyUserCredentialsResponse, error) {
	log.Printf("VerifyUserCredentials called for email: %s", req.Email)

	user, err := s.adminUserAuthRepo.VerifyCredentials(ctx, req.Email, req.Password)
	if err != nil {
		return &adminmanagementv1.VerifyUserCredentialsResponse{
			Valid:   false,
			Message: err.Error(),
		}, nil
	}

	return &adminmanagementv1.VerifyUserCredentialsResponse{
		Valid:   true,
		User:    userModelToProto(user),
		Message: "Credentials verified successfully",
	}, nil
}

func (s *userManagementService) UpdateLastLogin(ctx context.Context, req *adminmanagementv1.UpdateLastLoginRequest) (*emptypb.Empty, error) {
	log.Printf("UpdateLastLogin called for user: %s", req.UserId)

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, errors.NewBadRequestError("invalid user ID")
	}

	err = s.adminUserAuthRepo.UpdateLastLogin(ctx, userID, req.IpAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to update last login: %w", err)
	}

	return &emptypb.Empty{}, nil
}

// Helper function to convert model to proto
func userModelToProto(user *models.AdminUser) *adminmanagementv1.AdminUser {
	if user == nil {
		return nil
	}

	protoUser := &adminmanagementv1.AdminUser{
		Id:         user.ID.String(),
		Email:      user.Email,
		Username:   user.Username,
		IsActive:   user.IsActive,
		MfaEnabled: user.MFAEnabled,
		CreatedAt:  timestamppb.New(user.CreatedAt),
		UpdatedAt:  timestamppb.New(user.UpdatedAt),
	}

	if user.FirstName != nil {
		protoUser.FirstName = user.FirstName
	}
	if user.LastName != nil {
		protoUser.LastName = user.LastName
	}
	if user.LastLogin != nil {
		protoUser.LastLogin = timestamppb.New(*user.LastLogin)
	}
	if user.LastLoginIP != nil {
		protoUser.LastLoginIp = user.LastLoginIP
	}

	// Add roles
	if len(user.Roles) > 0 {
		protoUser.Roles = make([]*adminmanagementv1.Role, len(user.Roles))
		for i, role := range user.Roles {
			protoUser.Roles[i] = &adminmanagementv1.Role{
				Id:          role.ID.String(),
				Name:        role.Name,
				Description: role.Description,
				CreatedAt:   timestamppb.New(role.CreatedAt),
			}

			// Add permissions
			if len(role.Permissions) > 0 {
				protoUser.Roles[i].Permissions = make([]*adminmanagementv1.Permission, len(role.Permissions))
				for j, perm := range role.Permissions {
					protoUser.Roles[i].Permissions[j] = &adminmanagementv1.Permission{
						Id:          perm.ID.String(),
						Resource:    perm.Resource,
						Action:      perm.Action,
						Description: perm.Description,
					}
				}
			}
		}
	}

	// Set IP whitelist
	protoUser.IpWhitelist = user.IPWhitelist

	return protoUser
}

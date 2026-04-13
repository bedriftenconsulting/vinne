package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	adminmanagementv1 "github.com/randco/randco-microservices/proto/admin/management/v1"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
	commonModels "github.com/randco/randco-microservices/shared/common/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AdminManagementServiceTestSuite struct {
	BaseServiceTestSuite
	service *AdminManagementService
}

func TestAdminManagementServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AdminManagementServiceTestSuite))
}

func (s *AdminManagementServiceTestSuite) SetupTest() {
	// Call base setup
	s.BaseServiceTestSuite.SetupTest()

	// Create mock auth service
	mockAuth := &mockAuthService{}

	// Initialize service with mock auth and empty Kafka brokers for tests
	kafkaBrokers := []string{} // Empty for tests, will use in-memory event bus
	s.service = NewAdminManagementService(s.repos, mockAuth, kafkaBrokers)
}

// Test VerifyUserCredentials
func (s *AdminManagementServiceTestSuite) TestVerifyUserCredentials_Valid() {
	req := &adminmanagementv1.VerifyUserCredentialsRequest{
		Email:    "test.admin@example.com",
		Password: "TestPassword123!",
	}

	resp, err := s.service.VerifyUserCredentials(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.True(s.T(), resp.Valid)
	assert.NotNil(s.T(), resp.User)
	assert.Equal(s.T(), "test.admin@example.com", resp.User.Email)
	assert.Equal(s.T(), "testadmin", resp.User.Username)
	assert.Contains(s.T(), resp.Message, "verified successfully")
}

func (s *AdminManagementServiceTestSuite) TestVerifyUserCredentials_Invalid() {
	req := &adminmanagementv1.VerifyUserCredentialsRequest{
		Email:    "test.admin@example.com",
		Password: "WrongPassword",
	}

	resp, err := s.service.VerifyUserCredentials(s.ctx, req)
	assert.NoError(s.T(), err) // Service returns nil error, but resp.Valid = false
	assert.NotNil(s.T(), resp)
	assert.False(s.T(), resp.Valid)
	assert.Nil(s.T(), resp.User)
	assert.NotEmpty(s.T(), resp.Message)
}

// Test GetUserByEmail
func (s *AdminManagementServiceTestSuite) TestGetUserByEmail_Success() {
	req := &adminmanagementv1.GetUserByEmailRequest{
		Email: "test.admin@example.com",
	}

	resp, err := s.service.GetUserByEmail(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.NotNil(s.T(), resp.User)
	assert.Equal(s.T(), "test.admin@example.com", resp.User.Email)
	assert.Equal(s.T(), "testadmin", resp.User.Username)
}

func (s *AdminManagementServiceTestSuite) TestGetUserByEmail_NotFound() {
	req := &adminmanagementv1.GetUserByEmailRequest{
		Email: "nonexistent@example.com",
	}

	resp, err := s.service.GetUserByEmail(s.ctx, req)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "not found")
}

// Test UpdateLastLogin
func (s *AdminManagementServiceTestSuite) TestUpdateLastLogin_Success() {
	req := &adminmanagementv1.UpdateLastLoginRequest{
		UserId:    s.testAdminUser.ID.String(),
		IpAddress: "192.168.1.100",
	}

	_, err := s.service.UpdateLastLogin(s.ctx, req)
	assert.NoError(s.T(), err)

	// Verify last login was updated
	var lastLoginIP string
	var lastLogin time.Time
	err = s.db.QueryRow(`
		SELECT last_login_ip, last_login FROM admin_users WHERE id = $1
	`, s.testAdminUser.ID).Scan(&lastLoginIP, &lastLogin)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "192.168.1.100", lastLoginIP)
	assert.WithinDuration(s.T(), time.Now(), lastLogin, 2*time.Second)
}

// Test CreateAdminUser
func (s *AdminManagementServiceTestSuite) TestCreateAdminUser_Success() {
	req := &adminmanagementv1.CreateAdminUserRequest{
		Email:       "new.admin@example.com",
		Username:    "newadmin",
		Password:    "SecurePassword123!",
		FirstName:   stringPtr("New"),
		LastName:    stringPtr("Admin"),
		RoleIds:     []string{s.testRoles[0].ID.String()},
		IpWhitelist: []string{"192.168.1.0/24"},
	}

	resp, err := s.service.CreateAdminUser(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.NotNil(s.T(), resp.User)
	assert.Equal(s.T(), "new.admin@example.com", resp.User.Email)
	assert.Equal(s.T(), "newadmin", resp.User.Username)
	assert.True(s.T(), resp.User.IsActive)
	assert.Len(s.T(), resp.User.Roles, 1)
	assert.Equal(s.T(), "super_admin", resp.User.Roles[0].Name)

	// Verify user was created in database
	var count int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM admin_users WHERE email = $1
	`, "new.admin@example.com").Scan(&count)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)

	// Verify role was assigned
	var roleCount int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM admin_user_roles au
		JOIN admin_users u ON au.user_id = u.id
		WHERE u.email = $1
	`, "new.admin@example.com").Scan(&roleCount)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, roleCount)
}

func (s *AdminManagementServiceTestSuite) TestCreateAdminUser_DuplicateEmail() {
	req := &adminmanagementv1.CreateAdminUserRequest{
		Email:    "test.admin@example.com", // Already exists
		Username: "anotheruser",
		Password: "SecurePassword123!",
	}

	resp, err := s.service.CreateAdminUser(s.ctx, req)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "email")
}

// Test GetAdminUser
func (s *AdminManagementServiceTestSuite) TestGetAdminUser_Success() {
	req := &adminmanagementv1.GetAdminUserRequest{
		Id: s.testAdminUser.ID.String(),
	}

	resp, err := s.service.GetAdminUser(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.NotNil(s.T(), resp.User)
	assert.Equal(s.T(), s.testAdminUser.ID.String(), resp.User.Id)
	assert.Equal(s.T(), "test.admin@example.com", resp.User.Email)
}

func (s *AdminManagementServiceTestSuite) TestGetAdminUser_NotFound() {
	req := &adminmanagementv1.GetAdminUserRequest{
		Id: uuid.New().String(),
	}

	resp, err := s.service.GetAdminUser(s.ctx, req)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "not found")
}

// Test UpdateAdminUser
func (s *AdminManagementServiceTestSuite) TestUpdateAdminUser_Success() {
	req := &adminmanagementv1.UpdateAdminUserRequest{
		Id:        s.testAdminUser.ID.String(),
		FirstName: stringPtr("Updated"),
		LastName:  stringPtr("Name"),
	}

	resp, err := s.service.UpdateAdminUser(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.NotNil(s.T(), resp.User)
	assert.NotNil(s.T(), resp.User.FirstName)
	assert.NotNil(s.T(), resp.User.LastName)
	assert.Equal(s.T(), "Updated", *resp.User.FirstName)
	assert.Equal(s.T(), "Name", *resp.User.LastName)

	// Verify in database
	var firstName, lastName string
	err = s.db.QueryRow(`
		SELECT first_name, last_name FROM admin_users WHERE id = $1
	`, s.testAdminUser.ID).Scan(&firstName, &lastName)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated", firstName)
	assert.Equal(s.T(), "Name", lastName)
}

// Test DeleteAdminUser
func (s *AdminManagementServiceTestSuite) TestDeleteAdminUser_Success() {
	// Create a user to delete
	newUser := &models.AdminUser{
		BaseModel: commonModels.BaseModel{
			ID: uuid.New(),
		},
		Email:        "to.delete@example.com",
		Username:     "todelete",
		PasswordHash: "hash",
		IsActive:     true,
	}

	err := s.repos.AdminUser.Create(s.ctx, newUser, "Password123!")
	require.NoError(s.T(), err)

	req := &adminmanagementv1.DeleteAdminUserRequest{
		Id: newUser.ID.String(),
	}

	_, err = s.service.DeleteAdminUser(s.ctx, req)
	assert.NoError(s.T(), err)

	// Verify user is soft deleted
	var deletedAt *time.Time
	err = s.db.QueryRow(`
		SELECT deleted_at FROM admin_users WHERE id = $1
	`, newUser.ID).Scan(&deletedAt)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), deletedAt)
}

// Test ListAdminUsers
func (s *AdminManagementServiceTestSuite) TestListAdminUsers_Success() {
	// Create additional users
	for i := 0; i < 3; i++ {
		user := &models.AdminUser{
			Email:        "user" + string(rune('1'+i)) + "@example.com",
			Username:     "user" + string(rune('1'+i)),
			PasswordHash: "hash",
			IsActive:     true,
		}
		err := s.repos.AdminUser.Create(s.ctx, user, "Password123!")
		require.NoError(s.T(), err)
	}

	req := &adminmanagementv1.ListAdminUsersRequest{
		Page:     1,
		PageSize: 10,
	}

	resp, err := s.service.ListAdminUsers(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.GreaterOrEqual(s.T(), len(resp.Users), 4) // At least test user + 3 created
	assert.GreaterOrEqual(s.T(), resp.TotalCount, int32(4))
}

func (s *AdminManagementServiceTestSuite) TestListAdminUsers_WithFilter() {
	req := &adminmanagementv1.ListAdminUsersRequest{
		Page:     1,
		PageSize: 10,
		Email:    stringPtr("test.admin"),
	}

	resp, err := s.service.ListAdminUsers(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), 1, len(resp.Users))
	assert.Equal(s.T(), "test.admin@example.com", resp.Users[0].Email)
}

// Test AssignRoleToUser
// TODO: Implement these methods in the service
/*
func (s *AdminManagementServiceTestSuite) TestAssignRoleToUser_Success() {
	// Create a new user without roles
	newUser := &models.AdminUser{
		BaseModel: commonModels.BaseModel{
			ID: uuid.New(),
		},
		Email:        "noroles@example.com",
		Username:     "noroles",
		PasswordHash: "hash",
		IsActive:     true,
	}

	err := s.repos.AdminUser.Create(s.ctx, newUser, "Password123!")
	require.NoError(s.T(), err)

	req := &adminmanagementv1.AssignRoleToUserRequest{
		UserId: newUser.ID.String(),
		RoleId: s.testRoles[0].ID.String(),
	}

	_, err = s.service.AssignRoleToUser(s.ctx, req)
	assert.NoError(s.T(), err)

	// Verify role was assigned
	var count int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM admin_user_roles
		WHERE user_id = $1 AND role_id = $2
	`, newUser.ID, s.testRoles[0].ID).Scan(&count)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)
}

func (s *AdminManagementServiceTestSuite) TestAssignRoleToUser_DuplicateAssignment() {
	// Assign role first time
	req := &adminmanagementv1.AssignRoleToUserRequest{
		UserId: s.testAdminUser.ID.String(),
		RoleId: s.testRoles[0].ID.String(),
	}

	_, err := s.service.AssignRoleToUser(s.ctx, req)
	assert.NoError(s.T(), err) // Should succeed even if already assigned
}

// Test RemoveRoleFromUser
func (s *AdminManagementServiceTestSuite) TestRemoveRoleFromUser_Success() {
	req := &adminmanagementv1.RemoveRoleFromUserRequest{
		UserId: s.testAdminUser.ID.String(),
		RoleId: s.testRoles[0].ID.String(),
	}

	_, err := s.service.RemoveRoleFromUser(s.ctx, req)
	assert.NoError(s.T(), err)

	// Verify role was removed
	var count int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM admin_user_roles
		WHERE user_id = $1 AND role_id = $2
	`, s.testAdminUser.ID, s.testRoles[0].ID).Scan(&count)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 0, count)
}

// Test Role operations
func (s *AdminManagementServiceTestSuite) TestCreateRole_Success() {
	req := &adminmanagementv1.CreateRoleRequest{
		Name:        "test_role",
		Description: "Test Role Description",
	}

	resp, err := s.service.CreateRole(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.NotNil(s.T(), resp.Role)
	assert.Equal(s.T(), "test_role", resp.Role.Name)
	assert.Equal(s.T(), "Test Role Description", resp.Role.Description)

	// Verify in database
	var count int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM admin_roles WHERE name = $1
	`, "test_role").Scan(&count)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)
}

func (s *AdminManagementServiceTestSuite) TestCreateRole_DuplicateName() {
	req := &adminmanagementv1.CreateRoleRequest{
		Name:        "super_admin", // Already exists
		Description: "Another super admin",
	}

	resp, err := s.service.CreateRole(s.ctx, req)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "already exists")
}

func (s *AdminManagementServiceTestSuite) TestGetRole_Success() {
	req := &adminmanagementv1.GetRoleRequest{
		Id: s.testRoles[0].ID.String(),
	}

	resp, err := s.service.GetRole(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.NotNil(s.T(), resp.Role)
	assert.Equal(s.T(), s.testRoles[0].ID.String(), resp.Role.Id)
	assert.Equal(s.T(), "super_admin", resp.Role.Name)
}

func (s *AdminManagementServiceTestSuite) TestUpdateRole_Success() {
	// Create a role to update
	newRole := &models.Role{
		Name:        "role_to_update",
		Description: "Original description",
	}
	err := s.repos.Role.Create(s.ctx, newRole)
	require.NoError(s.T(), err)

	req := &adminmanagementv1.UpdateRoleRequest{
		Id:          newRole.ID.String(),
		Name:        stringPtr("updated_role"),
		Description: stringPtr("Updated description"),
	}

	resp, err := s.service.UpdateRole(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), "updated_role", resp.Role.Name)
	assert.Equal(s.T(), "Updated description", resp.Role.Description)
}

func (s *AdminManagementServiceTestSuite) TestDeleteRole_Success() {
	// Create a role to delete
	roleToDelete := &models.Role{
		Name:        "role_to_delete",
		Description: "Will be deleted",
	}
	err := s.repos.Role.Create(s.ctx, roleToDelete)
	require.NoError(s.T(), err)

	req := &adminmanagementv1.DeleteRoleRequest{
		Id: roleToDelete.ID.String(),
	}

	_, err = s.service.DeleteRole(s.ctx, req)
	assert.NoError(s.T(), err)

	// Verify role is soft deleted
	role, err := s.repos.Role.GetByID(s.ctx, roleToDelete.ID)
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), role)
}

func (s *AdminManagementServiceTestSuite) TestListRoles_Success() {
	req := &adminmanagementv1.ListRolesRequest{
		Page:     1,
		PageSize: 10,
	}

	resp, err := s.service.ListRoles(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.GreaterOrEqual(s.T(), len(resp.Roles), 2) // At least the test roles
	assert.GreaterOrEqual(s.T(), resp.TotalCount, int32(2))
}

// Test Permission operations
func (s *AdminManagementServiceTestSuite) TestListPermissions_Success() {
	req := &adminmanagementv1.ListPermissionsRequest{
		Page:     1,
		PageSize: 10,
	}

	resp, err := s.service.ListPermissions(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.GreaterOrEqual(s.T(), len(resp.Permissions), 3) // At least the test permissions
}

func (s *AdminManagementServiceTestSuite) TestListPermissions_WithResourceFilter() {
	req := &adminmanagementv1.ListPermissionsRequest{
		Page:     1,
		PageSize: 10,
		Resource: stringPtr("users"),
	}

	resp, err := s.service.ListPermissions(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), 3, len(resp.Permissions))
	for _, perm := range resp.Permissions {
		assert.Equal(s.T(), "users", perm.Resource)
	}
}

// Test AssignPermissionToRole
func (s *AdminManagementServiceTestSuite) TestAssignPermissionToRole_Success() {
	// Create a new permission
	newPerm := &models.Permission{
		ID:       uuid.New(),
		Resource: "test_resource",
		Action:   "test_action",
	}

	desc := "Test permission"
	_, err := s.db.Exec(`
		INSERT INTO permissions (id, resource, action, description)
		VALUES ($1, $2, $3, $4)
	`, newPerm.ID, newPerm.Resource, newPerm.Action, desc)
	require.NoError(s.T(), err)

	req := &adminmanagementv1.AssignPermissionToRoleRequest{
		RoleId:       s.testRoles[0].ID.String(),
		PermissionId: newPerm.ID.String(),
	}

	_, err = s.service.AssignPermissionToRole(s.ctx, req)
	assert.NoError(s.T(), err)

	// Verify permission was assigned
	var count int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM role_permissions
		WHERE role_id = $1 AND permission_id = $2
	`, s.testRoles[0].ID, newPerm.ID).Scan(&count)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)
}

// Test RemovePermissionFromRole
func (s *AdminManagementServiceTestSuite) TestRemovePermissionFromRole_Success() {
	req := &adminmanagementv1.RemovePermissionFromRoleRequest{
		RoleId:       s.testRoles[0].ID.String(),
		PermissionId: s.testPerms[0].ID.String(),
	}

	_, err := s.service.RemovePermissionFromRole(s.ctx, req)
	assert.NoError(s.T(), err)

	// Verify permission was removed
	var count int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM role_permissions
		WHERE role_id = $1 AND permission_id = $2
	`, s.testRoles[0].ID, s.testPerms[0].ID).Scan(&count)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 0, count)
}

// Test GetUserPermissions
func (s *AdminManagementServiceTestSuite) TestGetUserPermissions_Success() {
	req := &adminmanagementv1.GetUserPermissionsRequest{
		UserId: s.testAdminUser.ID.String(),
	}

	resp, err := s.service.GetUserPermissions(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.GreaterOrEqual(s.T(), len(resp.Permissions), 3) // Should have permissions through roles
}

// Test helper - model to proto conversion
func (s *AdminManagementServiceTestSuite) TestModelUserToProto() {
	user := s.testAdminUser
	pbUser := s.service.modelUserToProto(user)

	assert.Equal(s.T(), user.ID.String(), pbUser.Id)
	assert.Equal(s.T(), user.Email, pbUser.Email)
	assert.Equal(s.T(), user.Username, pbUser.Username)
	assert.Equal(s.T(), *user.FirstName, pbUser.FirstName)
	assert.Equal(s.T(), *user.LastName, pbUser.LastName)
	assert.Equal(s.T(), user.IsActive, pbUser.IsActive)
	assert.Equal(s.T(), user.MFAEnabled, pbUser.MfaEnabled)
	assert.Len(s.T(), pbUser.Roles, len(user.Roles))
}
*/

package repositories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"
)

type AdminUserRepositoryTestSuite struct {
	suite.Suite
	db        *sql.DB
	repo      AdminUserRepository
	authRepo  AdminUserAuthRepository
	roleRepo  AdminUserRoleRepository
	container testcontainers.Container
	ctx       context.Context
}

func TestAdminUserRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(AdminUserRepositoryTestSuite))
}

func (s *AdminUserRepositoryTestSuite) SetupSuite() {
	s.ctx = context.Background()

	// Start PostgreSQL testcontainer
	postgresContainer, err := postgres.Run(s.ctx, "postgres:17-alpine",
		postgres.WithDatabase("admin_management_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(s.T(), err)
	s.container = postgresContainer

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(s.ctx, "sslmode=disable")
	require.NoError(s.T(), err)

	// Connect to database
	s.db, err = sql.Open("postgres", connStr)
	require.NoError(s.T(), err)

	// Verify connection
	err = s.db.PingContext(s.ctx)
	require.NoError(s.T(), err)

	// Run migrations using the test helper
	testHelper := NewTestHelper()
	err = testHelper.SetupTestDB(s.T(), s.db)
	require.NoError(s.T(), err)

	// Initialize repositories (all three interfaces from same implementation)
	s.repo, s.authRepo, s.roleRepo = NewAdminUserRepositories(s.db)
}

func (s *AdminUserRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(s.T(), err)
	}
}

func (s *AdminUserRepositoryTestSuite) SetupTest() {
	// Clean up tables before each test using test helper
	testHelper := NewTestHelper()
	err := testHelper.CleanupTestData(s.T(), s.db)
	require.NoError(s.T(), err)

	// Seed default data using test helper
	err = testHelper.SeedDefaultData(s.T(), s.db)
	require.NoError(s.T(), err)
}

func (s *AdminUserRepositoryTestSuite) TestCreate() {
	user := &models.AdminUser{
		Email:       "test@example.com",
		Username:    "testuser",
		FirstName:   stringPtr("Test"),
		LastName:    stringPtr("User"),
		MFAEnabled:  false,
		IsActive:    true,
		IPWhitelist: []string{"127.0.0.1", "192.168.1.1"},
	}

	err := s.repo.Create(s.ctx, user, "testpassword")
	require.NoError(s.T(), err)

	// Verify user was created
	assert.NotEqual(s.T(), uuid.Nil, user.ID)
	assert.NotZero(s.T(), user.CreatedAt)
	assert.NotZero(s.T(), user.UpdatedAt)

	// Verify user can be retrieved
	retrieved, err := s.repo.GetByID(s.ctx, user.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), user.Email, retrieved.Email)
	assert.Equal(s.T(), user.Username, retrieved.Username)
	assert.Equal(s.T(), user.IPWhitelist, retrieved.IPWhitelist)
}

func (s *AdminUserRepositoryTestSuite) TestGetByEmail() {
	// Create test user
	user := &models.AdminUser{
		Email:    "email@example.com",
		Username: "emailuser",
		IsActive: true,
	}
	err := s.repo.Create(s.ctx, user, "testpassword")
	require.NoError(s.T(), err)

	// Test GetByEmail
	retrieved, err := s.repo.GetByEmail(s.ctx, "email@example.com")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), user.ID, retrieved.ID)
	assert.Equal(s.T(), user.Username, retrieved.Username)

	// Test non-existent email
	_, err = s.repo.GetByEmail(s.ctx, "nonexistent@example.com")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *AdminUserRepositoryTestSuite) TestList() {
	// Create multiple users
	users := []*models.AdminUser{
		{Email: "user1@example.com", Username: "user1", IsActive: true, MFAEnabled: true},
		{Email: "user2@example.com", Username: "user2", IsActive: false, MFAEnabled: false},
		{Email: "user3@example.com", Username: "user3", IsActive: true, MFAEnabled: false},
	}

	for _, user := range users {
		err := s.repo.Create(s.ctx, user, "testpassword")
		require.NoError(s.T(), err)
	}

	// Test list all users
	filter := models.AdminUserFilter{Page: 1, PageSize: 10}
	retrieved, total, err := s.repo.List(s.ctx, filter)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 3, total)
	assert.Len(s.T(), retrieved, 3)

	// Test filter by active status
	activeFilter := models.AdminUserFilter{IsActive: boolPtr(true), Page: 1, PageSize: 10}
	activeUsers, activeTotal, err := s.repo.List(s.ctx, activeFilter)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 2, activeTotal)
	assert.Len(s.T(), activeUsers, 2)
}

func (s *AdminUserRepositoryTestSuite) TestRoleAssignment() {
	// Create test user
	user := &models.AdminUser{
		Email:    "roles@example.com",
		Username: "rolesuser",
		IsActive: true,
	}
	err := s.repo.Create(s.ctx, user, "testpassword")
	require.NoError(s.T(), err)

	// Get a role ID
	var roleID uuid.UUID
	err = s.db.QueryRow("SELECT id FROM admin_roles WHERE name = 'admin'").Scan(&roleID)
	require.NoError(s.T(), err)

	// Assign role
	err = s.roleRepo.AssignRole(s.ctx, user.ID, roleID)
	require.NoError(s.T(), err)

	// Verify role assignment
	roles, err := s.roleRepo.GetUserRoles(s.ctx, user.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), roles, 1)
	assert.Equal(s.T(), roleID, roles[0].ID)
	assert.Equal(s.T(), "admin", roles[0].Name)

	// Remove role
	err = s.roleRepo.RemoveRole(s.ctx, user.ID, roleID)
	require.NoError(s.T(), err)

	// Verify role removal
	roles, err = s.roleRepo.GetUserRoles(s.ctx, user.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), roles, 0)
}

func (s *AdminUserRepositoryTestSuite) TestDelete() {
	// Create a user
	user := &models.AdminUser{
		Email:     "deletetest@example.com",
		Username:  "deletetest",
		FirstName: stringPtr("Delete"),
		LastName:  stringPtr("Test"),
		IsActive:  true,
	}

	err := s.repo.Create(s.ctx, user, "password123")
	require.NoError(s.T(), err)

	// Delete the user
	err = s.repo.Delete(s.ctx, user.ID)
	require.NoError(s.T(), err)

	// Try to get deleted user - should fail
	_, err = s.repo.GetByID(s.ctx, user.ID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *AdminUserRepositoryTestSuite) TestActivateDeactivate() {
	// Create an inactive user
	user := &models.AdminUser{
		Email:     "activatetest@example.com",
		Username:  "activatetest",
		FirstName: stringPtr("Activate"),
		LastName:  stringPtr("Test"),
		IsActive:  false,
	}

	err := s.repo.Create(s.ctx, user, "password123")
	require.NoError(s.T(), err)

	// Activate the user
	err = s.repo.Activate(s.ctx, user.ID)
	require.NoError(s.T(), err)

	// Verify user is active
	activeUser, err := s.repo.GetByID(s.ctx, user.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), activeUser.IsActive)

	// Deactivate the user
	err = s.repo.Deactivate(s.ctx, user.ID)
	require.NoError(s.T(), err)

	// Verify user is inactive
	inactiveUser, err := s.repo.GetByID(s.ctx, user.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), inactiveUser.IsActive)
}

func (s *AdminUserRepositoryTestSuite) TestVerifyCredentials() {
	// Create a user
	password := "testpassword123"
	user := &models.AdminUser{
		Email:     "verifycreds@example.com",
		Username:  "verifycreds",
		FirstName: stringPtr("Verify"),
		LastName:  stringPtr("Creds"),
		IsActive:  true,
	}

	err := s.repo.Create(s.ctx, user, password)
	require.NoError(s.T(), err)

	// Test valid credentials
	verifiedUser, err := s.authRepo.VerifyCredentials(s.ctx, user.Email, password)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), user.ID, verifiedUser.ID)
	assert.Equal(s.T(), user.Email, verifiedUser.Email)

	// Test invalid password
	_, err = s.authRepo.VerifyCredentials(s.ctx, user.Email, "wrongpassword")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "invalid credentials")

	// Test non-existent user
	_, err = s.authRepo.VerifyCredentials(s.ctx, "nonexistent@example.com", password)
	assert.Error(s.T(), err)

	// Test inactive user
	err = s.repo.Deactivate(s.ctx, user.ID)
	require.NoError(s.T(), err)

	_, err = s.authRepo.VerifyCredentials(s.ctx, user.Email, password)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "account is deactivated")
}

func (s *AdminUserRepositoryTestSuite) TestUpdateLastLogin() {
	// Create a user
	user := &models.AdminUser{
		Email:     "lastlogin@example.com",
		Username:  "lastlogin",
		FirstName: stringPtr("Last"),
		LastName:  stringPtr("Login"),
		IsActive:  true,
	}

	err := s.repo.Create(s.ctx, user, "password123")
	require.NoError(s.T(), err)

	// Update last login
	ipAddress := "192.168.1.100"
	err = s.authRepo.UpdateLastLogin(s.ctx, user.ID, ipAddress)
	require.NoError(s.T(), err)

	// Verify last login was updated
	updatedUser, err := s.repo.GetByID(s.ctx, user.ID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), updatedUser.LastLogin)
	assert.NotNil(s.T(), updatedUser.LastLoginIP)
	assert.Equal(s.T(), ipAddress, *updatedUser.LastLoginIP)
}

func (s *AdminUserRepositoryTestSuite) TestUpdatePassword() {
	// Create a user
	oldPassword := "oldpassword123"
	user := &models.AdminUser{
		Email:     "updatepass@example.com",
		Username:  "updatepass",
		FirstName: stringPtr("Update"),
		LastName:  stringPtr("Password"),
		IsActive:  true,
	}

	err := s.repo.Create(s.ctx, user, oldPassword)
	require.NoError(s.T(), err)

	// Update password
	newPassword := "newpassword456"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	require.NoError(s.T(), err)

	err = s.authRepo.UpdatePassword(s.ctx, user.ID, string(hashedPassword))
	require.NoError(s.T(), err)

	// Verify old password doesn't work
	_, err = s.authRepo.VerifyCredentials(s.ctx, user.Email, oldPassword)
	assert.Error(s.T(), err)

	// Verify new password works
	verifiedUser, err := s.authRepo.VerifyCredentials(s.ctx, user.Email, newPassword)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), user.ID, verifiedUser.ID)
}

func (s *AdminUserRepositoryTestSuite) TestMFAOperations() {
	// Create a user
	user := &models.AdminUser{
		Email:      "mfatest@example.com",
		Username:   "mfatest",
		FirstName:  stringPtr("MFA"),
		LastName:   stringPtr("Test"),
		IsActive:   true,
		MFAEnabled: false,
	}

	err := s.repo.Create(s.ctx, user, "password123")
	require.NoError(s.T(), err)

	// Update MFA secret
	mfaSecret := "JBSWY3DPEHPK3PXP"
	err = s.authRepo.UpdateMFASecret(s.ctx, user.ID, mfaSecret)
	require.NoError(s.T(), err)

	// Get MFA secret
	retrievedSecret, err := s.authRepo.GetMFASecret(s.ctx, user.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), mfaSecret, retrievedSecret)

	// Enable MFA
	err = s.authRepo.UpdateMFAStatus(s.ctx, user.ID, true)
	require.NoError(s.T(), err)

	// Verify MFA is enabled
	updatedUser, err := s.repo.GetByID(s.ctx, user.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), updatedUser.MFAEnabled)

	// Disable MFA
	err = s.authRepo.UpdateMFAStatus(s.ctx, user.ID, false)
	require.NoError(s.T(), err)

	// Verify MFA is disabled
	updatedUser, err = s.repo.GetByID(s.ctx, user.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), updatedUser.MFAEnabled)
}

func (s *AdminUserRepositoryTestSuite) TestGetByUsername() {
	// Create a user
	user := &models.AdminUser{
		Email:     "getbyusername@example.com",
		Username:  "uniqueusername",
		FirstName: stringPtr("Get"),
		LastName:  stringPtr("Username"),
		IsActive:  true,
	}

	err := s.repo.Create(s.ctx, user, "password123")
	require.NoError(s.T(), err)

	// Get by username
	retrievedUser, err := s.repo.GetByUsername(s.ctx, user.Username)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), user.ID, retrievedUser.ID)
	assert.Equal(s.T(), user.Username, retrievedUser.Username)

	// Test non-existent username
	_, err = s.repo.GetByUsername(s.ctx, "nonexistentusername")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *AdminUserRepositoryTestSuite) TestUpdate() {
	// Create a user
	user := &models.AdminUser{
		Email:     "updatetest@example.com",
		Username:  "updatetest",
		FirstName: stringPtr("Original"),
		LastName:  stringPtr("Name"),
		IsActive:  true,
	}

	err := s.repo.Create(s.ctx, user, "password123")
	require.NoError(s.T(), err)

	// Update user
	user.FirstName = stringPtr("Updated")
	user.LastName = stringPtr("User")
	user.IPWhitelist = []string{"192.168.1.0/24", "10.0.0.0/8"}

	err = s.repo.Update(s.ctx, user)
	require.NoError(s.T(), err)

	// Verify update
	updatedUser, err := s.repo.GetByID(s.ctx, user.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated", *updatedUser.FirstName)
	assert.Equal(s.T(), "User", *updatedUser.LastName)
	assert.Len(s.T(), updatedUser.IPWhitelist, 2)
	assert.Contains(s.T(), updatedUser.IPWhitelist, "192.168.1.0/24")
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

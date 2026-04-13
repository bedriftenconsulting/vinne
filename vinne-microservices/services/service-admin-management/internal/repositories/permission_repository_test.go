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
)

type PermissionRepositoryTestSuite struct {
	suite.Suite
	db        *sql.DB
	repo      PermissionRepository
	container testcontainers.Container
	ctx       context.Context
}

func TestPermissionRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(PermissionRepositoryTestSuite))
}

func (s *PermissionRepositoryTestSuite) SetupSuite() {
	s.ctx = context.Background()

	// Start PostgreSQL testcontainer
	postgresContainer, err := postgres.Run(s.ctx, "postgres:17-alpine",
		postgres.WithDatabase("permissions_test"),
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

	// Run migrations using test helper
	testHelper := NewTestHelper()
	err = testHelper.SetupTestDB(s.T(), s.db)
	require.NoError(s.T(), err)

	// Initialize repository
	s.repo = NewPermissionRepository(s.db)
}

func (s *PermissionRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(s.T(), err)
	}
}

func (s *PermissionRepositoryTestSuite) SetupTest() {
	// Clean up tables before each test using test helper
	testHelper := NewTestHelper()
	err := testHelper.CleanupTestData(s.T(), s.db)
	require.NoError(s.T(), err)

	// Insert default test permissions
	err = s.seedTestPermissions()
	require.NoError(s.T(), err)
}

func (s *PermissionRepositoryTestSuite) seedTestPermissions() error {
	permissions := []struct {
		id          string
		resource    string
		action      string
		description string
	}{
		{"a1eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", "users", "create", "Create new users"},
		{"a2eebc99-9c0b-4ef8-bb6d-6bb9bd380a12", "users", "read", "View user information"},
		{"a3eebc99-9c0b-4ef8-bb6d-6bb9bd380a13", "users", "update", "Update user information"},
		{"a4eebc99-9c0b-4ef8-bb6d-6bb9bd380a14", "users", "delete", "Delete users"},
		{"b1eebc99-9c0b-4ef8-bb6d-6bb9bd380a21", "games", "create", "Create new games"},
		{"b2eebc99-9c0b-4ef8-bb6d-6bb9bd380a22", "games", "read", "View game information"},
		{"b3eebc99-9c0b-4ef8-bb6d-6bb9bd380a23", "games", "update", "Update game settings"},
		{"c1eebc99-9c0b-4ef8-bb6d-6bb9bd380a31", "reports", "read", "View reports"},
		{"c2eebc99-9c0b-4ef8-bb6d-6bb9bd380a32", "reports", "export", "Export reports"},
	}

	for _, perm := range permissions {
		_, err := s.db.Exec(`
			INSERT INTO permissions (id, resource, action, description)
			VALUES ($1, $2, $3, $4)
		`, perm.id, perm.resource, perm.action, perm.description)
		if err != nil {
			return err
		}
	}
	return nil
}

// Test GetByID
func (s *PermissionRepositoryTestSuite) TestGetByID() {
	permissionID := uuid.MustParse("a1eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")

	permission, err := s.repo.GetByID(s.ctx, permissionID)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), permission)
	assert.Equal(s.T(), permissionID, permission.ID)
	assert.Equal(s.T(), "users", permission.Resource)
	assert.Equal(s.T(), "create", permission.Action)
	assert.NotNil(s.T(), permission.Description)
	assert.Equal(s.T(), "Create new users", *permission.Description)
}

// Test GetByID with non-existent ID
func (s *PermissionRepositoryTestSuite) TestGetByID_NotFound() {
	permission, err := s.repo.GetByID(s.ctx, uuid.New())
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), permission)
}

// Test GetByResourceAction
func (s *PermissionRepositoryTestSuite) TestGetByResourceAction() {
	permission, err := s.repo.GetByResourceAction(s.ctx, "users", "read")
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), permission)
	assert.Equal(s.T(), "users", permission.Resource)
	assert.Equal(s.T(), "read", permission.Action)
	assert.Equal(s.T(), uuid.MustParse("a2eebc99-9c0b-4ef8-bb6d-6bb9bd380a12"), permission.ID)
}

// Test GetByResourceAction with non-existent combination
func (s *PermissionRepositoryTestSuite) TestGetByResourceAction_NotFound() {
	permission, err := s.repo.GetByResourceAction(s.ctx, "nonexistent", "action")
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), permission)
}

// Test List with no filters
func (s *PermissionRepositoryTestSuite) TestList_NoFilters() {
	filter := models.PermissionFilter{
		Page:     1,
		PageSize: 10,
	}

	permissions, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 9, total) // We seeded 9 permissions
	assert.Len(s.T(), permissions, 9)

	// Verify they're ordered by resource and action
	for i := 1; i < len(permissions); i++ {
		prev := permissions[i-1]
		curr := permissions[i]

		if prev.Resource == curr.Resource {
			assert.True(s.T(), prev.Action <= curr.Action,
				"Permissions with same resource should be ordered by action")
		} else {
			assert.True(s.T(), prev.Resource <= curr.Resource,
				"Permissions should be ordered by resource")
		}
	}
}

// Test List with resource filter
func (s *PermissionRepositoryTestSuite) TestList_WithResourceFilter() {
	filter := models.PermissionFilter{
		Resource: "users",
		Page:     1,
		PageSize: 10,
	}

	permissions, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 4, total) // 4 user permissions
	assert.Len(s.T(), permissions, 4)

	// Verify all permissions are for users resource
	for _, perm := range permissions {
		assert.Equal(s.T(), "users", perm.Resource)
	}

	// Verify they're ordered by action
	expectedActions := []string{"create", "delete", "read", "update"}
	for i, perm := range permissions {
		assert.Equal(s.T(), expectedActions[i], perm.Action)
	}
}

// Test List with pagination
func (s *PermissionRepositoryTestSuite) TestList_WithPagination() {
	// First page
	filter := models.PermissionFilter{
		Page:     1,
		PageSize: 3,
	}

	permissions1, total1, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 9, total1)
	assert.Len(s.T(), permissions1, 3)

	// Second page
	filter.Page = 2
	permissions2, total2, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 9, total2)
	assert.Len(s.T(), permissions2, 3)

	// Third page
	filter.Page = 3
	permissions3, total3, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 9, total3)
	assert.Len(s.T(), permissions3, 3)

	// Fourth page (should be empty or partial)
	filter.Page = 4
	permissions4, total4, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 9, total4)
	assert.Len(s.T(), permissions4, 0)

	// Verify no duplicates across pages
	ids := make(map[uuid.UUID]bool)
	for _, perm := range permissions1 {
		ids[perm.ID] = true
	}
	for _, perm := range permissions2 {
		assert.False(s.T(), ids[perm.ID], "Found duplicate permission in page 2")
		ids[perm.ID] = true
	}
	for _, perm := range permissions3 {
		assert.False(s.T(), ids[perm.ID], "Found duplicate permission in page 3")
		ids[perm.ID] = true
	}
}

// Test List with resource filter and pagination
func (s *PermissionRepositoryTestSuite) TestList_WithResourceFilterAndPagination() {
	filter := models.PermissionFilter{
		Resource: "users",
		Page:     1,
		PageSize: 2,
	}

	// First page
	permissions1, total1, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 4, total1)
	assert.Len(s.T(), permissions1, 2)
	assert.Equal(s.T(), "create", permissions1[0].Action)
	assert.Equal(s.T(), "delete", permissions1[1].Action)

	// Second page
	filter.Page = 2
	permissions2, total2, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 4, total2)
	assert.Len(s.T(), permissions2, 2)
	assert.Equal(s.T(), "read", permissions2[0].Action)
	assert.Equal(s.T(), "update", permissions2[1].Action)
}

// Test empty table
func (s *PermissionRepositoryTestSuite) TestList_EmptyTable() {
	// Clear all permissions
	_, err := s.db.Exec("TRUNCATE permissions RESTART IDENTITY CASCADE")
	require.NoError(s.T(), err)

	filter := models.PermissionFilter{
		Page:     1,
		PageSize: 10,
	}

	permissions, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 0, total)
	assert.Len(s.T(), permissions, 0)
}

// Test GetByResourceAction case sensitivity
func (s *PermissionRepositoryTestSuite) TestGetByResourceAction_CaseSensitive() {
	// Test with different cases (should be case-sensitive)
	permission, err := s.repo.GetByResourceAction(s.ctx, "Users", "Read")
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), permission) // Should not find with different case

	permission, err = s.repo.GetByResourceAction(s.ctx, "USERS", "READ")
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), permission) // Should not find with different case
}

// Test List with invalid pagination
func (s *PermissionRepositoryTestSuite) TestList_InvalidPagination() {
	// Test with page 0 (should default to 1)
	filter := models.PermissionFilter{
		Page:     0,
		PageSize: 5,
	}

	permissions, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 9, total)
	assert.Len(s.T(), permissions, 5) // Should return first page

	// Test with negative page size (should default to reasonable size)
	filter = models.PermissionFilter{
		Page:     1,
		PageSize: -1,
	}

	permissions, total, err = s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 9, total)
	assert.True(s.T(), len(permissions) > 0) // Should return some results
}

// Test permissions without descriptions
func (s *PermissionRepositoryTestSuite) TestGetByID_NullDescription() {
	// Insert a permission without description
	permID := uuid.New()
	_, err := s.db.Exec(`
		INSERT INTO permissions (id, resource, action, description)
		VALUES ($1, $2, $3, NULL)
	`, permID, "test", "action")
	require.NoError(s.T(), err)

	permission, err := s.repo.GetByID(s.ctx, permID)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), permission)
	assert.Nil(s.T(), permission.Description)
}

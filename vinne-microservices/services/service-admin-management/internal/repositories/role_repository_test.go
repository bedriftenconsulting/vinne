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

type RoleRepositoryTestSuite struct {
	suite.Suite
	db          *sql.DB
	repo        RoleRepository
	container   testcontainers.Container
	ctx         context.Context
	testRoleID  uuid.UUID
	testPermIDs []uuid.UUID
}

func TestRoleRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(RoleRepositoryTestSuite))
}

func (s *RoleRepositoryTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.testRoleID = uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	s.testPermIDs = []uuid.UUID{
		uuid.MustParse("b1eebc99-9c0b-4ef8-bb6d-6bb9bd380b11"),
		uuid.MustParse("b2eebc99-9c0b-4ef8-bb6d-6bb9bd380b12"),
		uuid.MustParse("b3eebc99-9c0b-4ef8-bb6d-6bb9bd380b13"),
	}

	// Start PostgreSQL testcontainer
	postgresContainer, err := postgres.Run(s.ctx, "postgres:17-alpine",
		postgres.WithDatabase("roles_test"),
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
	s.repo = NewRoleRepository(s.db)
}

func (s *RoleRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(s.T(), err)
	}
}

func (s *RoleRepositoryTestSuite) SetupTest() {
	// Clean up tables before each test using test helper
	testHelper := NewTestHelper()
	err := testHelper.CleanupTestData(s.T(), s.db)
	require.NoError(s.T(), err)

	// Insert test data
	err = s.seedTestData()
	require.NoError(s.T(), err)
}

func (s *RoleRepositoryTestSuite) seedTestData() error {
	// Insert test permissions
	permissions := []struct {
		id       uuid.UUID
		resource string
		action   string
		desc     string
	}{
		{s.testPermIDs[0], "users", "create", "Create users"},
		{s.testPermIDs[1], "users", "read", "Read users"},
		{s.testPermIDs[2], "users", "update", "Update users"},
		{uuid.MustParse("c1eebc99-9c0b-4ef8-bb6d-6bb9bd380c11"), "games", "create", "Create games"},
		{uuid.MustParse("c2eebc99-9c0b-4ef8-bb6d-6bb9bd380c12"), "games", "read", "Read games"},
	}

	for _, p := range permissions {
		_, err := s.db.Exec(`
			INSERT INTO permissions (id, resource, action, description)
			VALUES ($1, $2, $3, $4)
		`, p.id, p.resource, p.action, p.desc)
		if err != nil {
			return err
		}
	}

	// Insert test roles
	roles := []struct {
		id   string
		name string
		desc string
	}{
		{s.testRoleID.String(), "admin", "Administrator role"},
		{"a1eebc99-9c0b-4ef8-bb6d-6bb9bd380a12", "manager", "Manager role"},
		{"a2eebc99-9c0b-4ef8-bb6d-6bb9bd380a13", "viewer", "Viewer role"},
	}

	for _, r := range roles {
		_, err := s.db.Exec(`
			INSERT INTO admin_roles (id, name, description)
			VALUES ($1, $2, $3)
		`, r.id, r.name, r.desc)
		if err != nil {
			return err
		}
	}

	// Assign some permissions to admin role
	for _, permID := range s.testPermIDs[:2] { // First 2 permissions
		_, err := s.db.Exec(`
			INSERT INTO role_permissions (role_id, permission_id)
			VALUES ($1, $2)
		`, s.testRoleID, permID)
		if err != nil {
			return err
		}
	}

	return nil
}

// Test Create
func (s *RoleRepositoryTestSuite) TestCreate() {
	role := &models.Role{
		ID:          uuid.New(),
		Name:        "test_role",
		Description: "Test role description",
	}

	err := s.repo.Create(s.ctx, role)
	assert.NoError(s.T(), err)

	// Verify role was created
	var count int
	err = s.db.QueryRow("SELECT COUNT(*) FROM admin_roles WHERE id = $1", role.ID).Scan(&count)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)
}

// Test Create with duplicate name
func (s *RoleRepositoryTestSuite) TestCreate_DuplicateName() {
	role := &models.Role{
		ID:          uuid.New(),
		Name:        "admin", // Already exists
		Description: "Another admin role",
	}

	err := s.repo.Create(s.ctx, role)
	assert.Error(s.T(), err)
}

// Test GetByID
func (s *RoleRepositoryTestSuite) TestGetByID() {
	role, err := s.repo.GetByID(s.ctx, s.testRoleID)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), role)
	assert.Equal(s.T(), s.testRoleID, role.ID)
	assert.Equal(s.T(), "admin", role.Name)
	assert.Equal(s.T(), "Administrator role", role.Description)
}

// Test GetByID with non-existent ID
func (s *RoleRepositoryTestSuite) TestGetByID_NotFound() {
	role, err := s.repo.GetByID(s.ctx, uuid.New())
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), role)
}

// Test GetByName
func (s *RoleRepositoryTestSuite) TestGetByName() {
	role, err := s.repo.GetByName(s.ctx, "manager")
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), role)
	assert.Equal(s.T(), "manager", role.Name)
	assert.Equal(s.T(), "Manager role", role.Description)
}

// Test GetByName with non-existent name
func (s *RoleRepositoryTestSuite) TestGetByName_NotFound() {
	role, err := s.repo.GetByName(s.ctx, "nonexistent")
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), role)
}

// Test List with no filters
func (s *RoleRepositoryTestSuite) TestList_NoFilters() {
	filter := models.RoleFilter{
		Page:     1,
		PageSize: 10,
	}

	roles, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 3, total)
	assert.Len(s.T(), roles, 3)

	// Verify ordering by name
	assert.Equal(s.T(), "admin", roles[0].Name)
	assert.Equal(s.T(), "manager", roles[1].Name)
	assert.Equal(s.T(), "viewer", roles[2].Name)
}

// Test List with name filter
func (s *RoleRepositoryTestSuite) TestList_WithNameFilter() {
	filter := models.RoleFilter{
		Name:     "admin",
		Page:     1,
		PageSize: 10,
	}

	roles, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, total)
	assert.Len(s.T(), roles, 1)
	assert.Equal(s.T(), "admin", roles[0].Name)
}

// Test List with partial name filter (LIKE search)
func (s *RoleRepositoryTestSuite) TestList_WithPartialNameFilter() {
	filter := models.RoleFilter{
		Name:     "view", // Should match "viewer"
		Page:     1,
		PageSize: 10,
	}

	roles, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, total)
	assert.Len(s.T(), roles, 1)
	assert.Equal(s.T(), "viewer", roles[0].Name)
}

// Test List with pagination
func (s *RoleRepositoryTestSuite) TestList_WithPagination() {
	filter := models.RoleFilter{
		Page:     1,
		PageSize: 2,
	}

	// First page
	roles1, total1, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 3, total1)
	assert.Len(s.T(), roles1, 2)

	// Second page
	filter.Page = 2
	roles2, total2, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 3, total2)
	assert.Len(s.T(), roles2, 1)

	// Verify no duplicates
	names := make(map[string]bool)
	for _, role := range roles1 {
		names[role.Name] = true
	}
	for _, role := range roles2 {
		assert.False(s.T(), names[role.Name])
	}
}

// Test Update
func (s *RoleRepositoryTestSuite) TestUpdate() {
	// Get existing role
	role, err := s.repo.GetByID(s.ctx, s.testRoleID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), role)

	// Update the role
	role.Name = "super_admin"
	role.Description = "Updated description"

	err = s.repo.Update(s.ctx, role)
	assert.NoError(s.T(), err)

	// Verify update
	updated, err := s.repo.GetByID(s.ctx, s.testRoleID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "super_admin", updated.Name)
	assert.Equal(s.T(), "Updated description", updated.Description)
}

// Test Update with duplicate name
func (s *RoleRepositoryTestSuite) TestUpdate_DuplicateName() {
	// Get admin role
	role, err := s.repo.GetByID(s.ctx, s.testRoleID)
	require.NoError(s.T(), err)

	// Try to change name to existing name
	role.Name = "manager" // Already exists

	err = s.repo.Update(s.ctx, role)
	assert.Error(s.T(), err)
}

// Test Delete (soft delete)
func (s *RoleRepositoryTestSuite) TestDelete() {
	roleID := uuid.MustParse("a1eebc99-9c0b-4ef8-bb6d-6bb9bd380a12") // manager role

	err := s.repo.Delete(s.ctx, roleID)
	assert.NoError(s.T(), err)

	// Verify soft delete
	role, err := s.repo.GetByID(s.ctx, roleID)
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), role) // Should not be found

	// Verify it still exists in database with deleted_at set
	var deletedAt sql.NullTime
	err = s.db.QueryRow("SELECT deleted_at FROM admin_roles WHERE id = $1", roleID).Scan(&deletedAt)
	assert.NoError(s.T(), err)
	assert.True(s.T(), deletedAt.Valid)
}

// Test AssignPermission
func (s *RoleRepositoryTestSuite) TestAssignPermission() {
	roleID := uuid.MustParse("a1eebc99-9c0b-4ef8-bb6d-6bb9bd380a12") // manager role
	permID := uuid.MustParse("c1eebc99-9c0b-4ef8-bb6d-6bb9bd380c11") // games.create

	err := s.repo.AssignPermission(s.ctx, roleID, permID)
	assert.NoError(s.T(), err)

	// Verify assignment
	var count int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM role_permissions 
		WHERE role_id = $1 AND permission_id = $2
	`, roleID, permID).Scan(&count)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)
}

// Test AssignPermission duplicate (should not error)
func (s *RoleRepositoryTestSuite) TestAssignPermission_Duplicate() {
	// This permission is already assigned to admin role
	err := s.repo.AssignPermission(s.ctx, s.testRoleID, s.testPermIDs[0])
	assert.NoError(s.T(), err) // Should not error on duplicate
}

// Test RemovePermission
func (s *RoleRepositoryTestSuite) TestRemovePermission() {
	// Remove an existing permission from admin role
	err := s.repo.RemovePermission(s.ctx, s.testRoleID, s.testPermIDs[0])
	assert.NoError(s.T(), err)

	// Verify removal
	var count int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM role_permissions 
		WHERE role_id = $1 AND permission_id = $2
	`, s.testRoleID, s.testPermIDs[0]).Scan(&count)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 0, count)
}

// Test GetRolePermissions
func (s *RoleRepositoryTestSuite) TestGetRolePermissions() {
	permissions, err := s.repo.GetRolePermissions(s.ctx, s.testRoleID)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), permissions, 2) // Admin role has 2 permissions

	// Verify permission IDs
	permIDs := make(map[uuid.UUID]bool)
	for _, perm := range permissions {
		permIDs[perm.ID] = true
	}
	assert.True(s.T(), permIDs[s.testPermIDs[0]])
	assert.True(s.T(), permIDs[s.testPermIDs[1]])
}

// Test SetRolePermissions
func (s *RoleRepositoryTestSuite) TestSetRolePermissions() {
	roleID := uuid.MustParse("a2eebc99-9c0b-4ef8-bb6d-6bb9bd380a13") // viewer role

	// Set new permissions
	newPermIDs := []uuid.UUID{
		uuid.MustParse("b2eebc99-9c0b-4ef8-bb6d-6bb9bd380b12"), // users.read
		uuid.MustParse("c2eebc99-9c0b-4ef8-bb6d-6bb9bd380c12"), // games.read
	}

	err := s.repo.SetRolePermissions(s.ctx, roleID, newPermIDs)
	assert.NoError(s.T(), err)

	// Verify permissions were set
	permissions, err := s.repo.GetRolePermissions(s.ctx, roleID)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), permissions, 2)

	// Update permissions (replace with different set)
	updatedPermIDs := []uuid.UUID{
		uuid.MustParse("c1eebc99-9c0b-4ef8-bb6d-6bb9bd380c11"), // games.create
	}

	err = s.repo.SetRolePermissions(s.ctx, roleID, updatedPermIDs)
	assert.NoError(s.T(), err)

	// Verify old permissions were removed and new ones added
	permissions, err = s.repo.GetRolePermissions(s.ctx, roleID)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), permissions, 1)
	assert.Equal(s.T(), updatedPermIDs[0], permissions[0].ID)
}

// Test SetRolePermissions with empty list (remove all)
func (s *RoleRepositoryTestSuite) TestSetRolePermissions_Empty() {
	// Remove all permissions from admin role
	err := s.repo.SetRolePermissions(s.ctx, s.testRoleID, []uuid.UUID{})
	assert.NoError(s.T(), err)

	// Verify no permissions
	permissions, err := s.repo.GetRolePermissions(s.ctx, s.testRoleID)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), permissions, 0)
}

// Test Delete with role that has permissions
func (s *RoleRepositoryTestSuite) TestDelete_WithPermissions() {
	// Admin role has permissions assigned
	err := s.repo.Delete(s.ctx, s.testRoleID)
	assert.NoError(s.T(), err)

	// Verify role is soft deleted
	role, err := s.repo.GetByID(s.ctx, s.testRoleID)
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), role)

	// Verify role_permissions still exist (soft delete doesn't cascade delete)
	var count int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM role_permissions WHERE role_id = $1
	`, s.testRoleID).Scan(&count)
	assert.NoError(s.T(), err)
	assert.True(s.T(), count > 0) // Permissions should still exist in DB
}

// Test List excludes soft-deleted roles
func (s *RoleRepositoryTestSuite) TestList_ExcludesDeleted() {
	// Soft delete a role
	err := s.repo.Delete(s.ctx, s.testRoleID)
	require.NoError(s.T(), err)

	// List should not include deleted role
	filter := models.RoleFilter{
		Page:     1,
		PageSize: 10,
	}

	roles, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 2, total) // Should be 2 instead of 3

	// Verify deleted role is not in list
	for _, role := range roles {
		assert.NotEqual(s.T(), s.testRoleID, role.ID)
	}
}

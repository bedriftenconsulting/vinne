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

type AuditLogRepositoryTestSuite struct {
	suite.Suite
	db         *sql.DB
	repo       AuditLogRepository
	container  testcontainers.Container
	ctx        context.Context
	testUserID uuid.UUID
}

func TestAuditLogRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(AuditLogRepositoryTestSuite))
}

func (s *AuditLogRepositoryTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.testUserID = uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")

	// Start PostgreSQL testcontainer
	postgresContainer, err := postgres.Run(s.ctx, "postgres:17-alpine",
		postgres.WithDatabase("audit_logs_test"),
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
	s.repo = NewAuditLogRepository(s.db)
}

func (s *AuditLogRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(s.T(), err)
	}
}

func (s *AuditLogRepositoryTestSuite) SetupTest() {
	// Clean up tables before each test using test helper
	testHelper := NewTestHelper()
	err := testHelper.CleanupTestData(s.T(), s.db)
	require.NoError(s.T(), err)

	// Create a test user
	err = s.createTestUser()
	require.NoError(s.T(), err)
}

func (s *AuditLogRepositoryTestSuite) createTestUser() error {
	_, err := s.db.Exec(`
		INSERT INTO admin_users (id, email, username, password_hash)
		VALUES ($1, $2, $3, $4)
	`, s.testUserID, "test@example.com", "testuser", "hashedpassword")
	return err
}

// Test Create
func (s *AuditLogRepositoryTestSuite) TestCreate() {
	auditLog := &models.AuditLog{
		ID:             uuid.New(),
		AdminUserID:    s.testUserID,
		Action:         "admin.login",
		Resource:       auditStringPtr("auth"),
		ResourceID:     auditStringPtr("session-123"),
		IPAddress:      "127.0.0.1",
		UserAgent:      "Mozilla/5.0",
		RequestData:    map[string]interface{}{"email": "test@example.com"},
		ResponseStatus: 200,
		CreatedAt:      time.Now(),
	}

	err := s.repo.Create(s.ctx, auditLog)
	assert.NoError(s.T(), err)

	// Verify audit log was created
	var count int
	err = s.db.QueryRow("SELECT COUNT(*) FROM admin_audit_logs WHERE id = $1", auditLog.ID).Scan(&count)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)
}

// Test List with no filters
func (s *AuditLogRepositoryTestSuite) TestList_NoFilters() {
	// Create multiple audit logs
	for i := 0; i < 5; i++ {
		auditLog := &models.AuditLog{
			ID:             uuid.New(),
			AdminUserID:    s.testUserID,
			Action:         "test.action",
			IPAddress:      "127.0.0.1",
			UserAgent:      "TestAgent",
			ResponseStatus: 200,
			CreatedAt:      time.Now().Add(time.Duration(-i) * time.Hour),
		}
		err := s.repo.Create(s.ctx, auditLog)
		require.NoError(s.T(), err)
	}

	// List without filters
	filter := models.AuditLogFilter{
		Page:     1,
		PageSize: 10,
	}

	logs, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 5, total)
	assert.Len(s.T(), logs, 5)

	// Verify ordering (most recent first)
	for i := 1; i < len(logs); i++ {
		assert.True(s.T(), logs[i-1].CreatedAt.After(logs[i].CreatedAt) ||
			logs[i-1].CreatedAt.Equal(logs[i].CreatedAt))
	}
}

// Test List with user filter
func (s *AuditLogRepositoryTestSuite) TestList_WithUserFilter() {
	// Create another test user
	otherUserID := uuid.MustParse("b1ffcd99-8b1a-3de7-aa5c-5aa8ac271900")
	_, err := s.db.Exec(`
		INSERT INTO admin_users (id, email, username, password_hash)
		VALUES ($1, $2, $3, $4)
	`, otherUserID, "other@example.com", "otheruser", "hashedpassword")
	require.NoError(s.T(), err)

	// Create logs for different users
	for i := 0; i < 3; i++ {
		auditLog := &models.AuditLog{
			ID:             uuid.New(),
			AdminUserID:    s.testUserID,
			Action:         "user1.action",
			IPAddress:      "127.0.0.1",
			UserAgent:      "TestAgent",
			ResponseStatus: 200,
		}
		err := s.repo.Create(s.ctx, auditLog)
		require.NoError(s.T(), err)
	}

	for i := 0; i < 2; i++ {
		auditLog := &models.AuditLog{
			ID:             uuid.New(),
			AdminUserID:    otherUserID,
			Action:         "user2.action",
			IPAddress:      "127.0.0.1",
			UserAgent:      "TestAgent",
			ResponseStatus: 200,
		}
		err := s.repo.Create(s.ctx, auditLog)
		require.NoError(s.T(), err)
	}

	// Filter by user
	filter := models.AuditLogFilter{
		UserID:   &s.testUserID,
		Page:     1,
		PageSize: 10,
	}

	logs, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 3, total)
	assert.Len(s.T(), logs, 3)

	// Verify all logs belong to the filtered user
	for _, log := range logs {
		assert.Equal(s.T(), s.testUserID, log.AdminUserID)
	}
}

// Test List with action filter
func (s *AuditLogRepositoryTestSuite) TestList_WithActionFilter() {
	// Create logs with different actions
	actions := []string{"admin.login", "admin.logout", "admin.login", "user.create"}
	for _, action := range actions {
		auditLog := &models.AuditLog{
			ID:             uuid.New(),
			AdminUserID:    s.testUserID,
			Action:         action,
			IPAddress:      "127.0.0.1",
			UserAgent:      "TestAgent",
			ResponseStatus: 200,
		}
		err := s.repo.Create(s.ctx, auditLog)
		require.NoError(s.T(), err)
	}

	// Filter by action
	loginAction := "admin.login"
	filter := models.AuditLogFilter{
		Action:   &loginAction,
		Page:     1,
		PageSize: 10,
	}

	logs, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 2, total)
	assert.Len(s.T(), logs, 2)

	// Verify all logs have the filtered action
	for _, log := range logs {
		assert.Equal(s.T(), loginAction, log.Action)
	}
}

// Test List with resource filter
func (s *AuditLogRepositoryTestSuite) TestList_WithResourceFilter() {
	// Create logs with different resources
	resources := []string{"auth", "users", "auth", "roles"}
	for _, resource := range resources {
		auditLog := &models.AuditLog{
			ID:             uuid.New(),
			AdminUserID:    s.testUserID,
			Action:         "test.action",
			Resource:       &resource,
			IPAddress:      "127.0.0.1",
			UserAgent:      "TestAgent",
			ResponseStatus: 200,
		}
		err := s.repo.Create(s.ctx, auditLog)
		require.NoError(s.T(), err)
	}

	// Filter by resource
	authResource := "auth"
	filter := models.AuditLogFilter{
		Resource: &authResource,
		Page:     1,
		PageSize: 10,
	}

	logs, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 2, total)
	assert.Len(s.T(), logs, 2)

	// Verify all logs have the filtered resource
	for _, log := range logs {
		assert.NotNil(s.T(), log.Resource)
		assert.Equal(s.T(), authResource, *log.Resource)
	}
}

// Test List with date range filter
func (s *AuditLogRepositoryTestSuite) TestList_WithDateRange() {
	now := time.Now()

	// Create logs at different times
	timestamps := []time.Time{
		now.Add(-72 * time.Hour), // 3 days ago
		now.Add(-48 * time.Hour), // 2 days ago
		now.Add(-24 * time.Hour), // 1 day ago
		now.Add(-12 * time.Hour), // 12 hours ago
		now.Add(-1 * time.Hour),  // 1 hour ago
	}

	for _, timestamp := range timestamps {
		_, err := s.db.Exec(`
			INSERT INTO admin_audit_logs (id, admin_user_id, action, ip_address, user_agent, response_status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), s.testUserID, "test.action", "127.0.0.1", "TestAgent", 200, timestamp)
		require.NoError(s.T(), err)
	}

	// Filter by date range (last 2 days)
	startDate := now.Add(-49 * time.Hour)
	endDate := now
	filter := models.AuditLogFilter{
		StartDate: &startDate,
		EndDate:   &endDate,
		Page:      1,
		PageSize:  10,
	}

	logs, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 4, total) // Should exclude the log from 3 days ago
	assert.Len(s.T(), logs, 4)

	// Verify all logs are within the date range
	for _, log := range logs {
		assert.True(s.T(), log.CreatedAt.After(startDate) || log.CreatedAt.Equal(startDate))
		assert.True(s.T(), log.CreatedAt.Before(endDate) || log.CreatedAt.Equal(endDate))
	}
}

// Test List with pagination
func (s *AuditLogRepositoryTestSuite) TestList_WithPagination() {
	// Create 15 audit logs
	for i := 0; i < 15; i++ {
		auditLog := &models.AuditLog{
			ID:             uuid.New(),
			AdminUserID:    s.testUserID,
			Action:         "test.action",
			IPAddress:      "127.0.0.1",
			UserAgent:      "TestAgent",
			ResponseStatus: 200,
			CreatedAt:      time.Now().Add(time.Duration(-i) * time.Minute),
		}
		err := s.repo.Create(s.ctx, auditLog)
		require.NoError(s.T(), err)
	}

	// Get first page
	filter := models.AuditLogFilter{
		Page:     1,
		PageSize: 5,
	}
	logs1, total1, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 15, total1)
	assert.Len(s.T(), logs1, 5)

	// Get second page
	filter.Page = 2
	logs2, total2, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 15, total2)
	assert.Len(s.T(), logs2, 5)

	// Get third page
	filter.Page = 3
	logs3, total3, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 15, total3)
	assert.Len(s.T(), logs3, 5)

	// Verify no overlap between pages
	ids := make(map[uuid.UUID]bool)
	for _, log := range logs1 {
		ids[log.ID] = true
	}
	for _, log := range logs2 {
		assert.False(s.T(), ids[log.ID], "Found duplicate log in page 2")
		ids[log.ID] = true
	}
	for _, log := range logs3 {
		assert.False(s.T(), ids[log.ID], "Found duplicate log in page 3")
	}
}

// Test List with combined filters
func (s *AuditLogRepositoryTestSuite) TestList_WithCombinedFilters() {
	now := time.Now()

	// Create varied audit logs
	testData := []struct {
		action    string
		resource  string
		createdAt time.Time
	}{
		{"admin.login", "auth", now.Add(-2 * time.Hour)},
		{"admin.login", "auth", now.Add(-1 * time.Hour)},
		{"admin.logout", "auth", now.Add(-30 * time.Minute)},
		{"user.create", "users", now.Add(-15 * time.Minute)},
		{"admin.login", "auth", now.Add(-3 * 24 * time.Hour)}, // Old login
	}

	for _, data := range testData {
		_, err := s.db.Exec(`
			INSERT INTO admin_audit_logs (id, admin_user_id, action, resource, ip_address, user_agent, response_status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, uuid.New(), s.testUserID, data.action, data.resource, "127.0.0.1", "TestAgent", 200, data.createdAt)
		require.NoError(s.T(), err)
	}

	// Filter by action and date range
	loginAction := "admin.login"
	startDate := now.Add(-24 * time.Hour)
	filter := models.AuditLogFilter{
		Action:    &loginAction,
		StartDate: &startDate,
		Page:      1,
		PageSize:  10,
	}

	logs, total, err := s.repo.List(s.ctx, filter)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 2, total) // Should only get recent logins
	assert.Len(s.T(), logs, 2)

	for _, log := range logs {
		assert.Equal(s.T(), loginAction, log.Action)
		assert.True(s.T(), log.CreatedAt.After(startDate))
	}
}

// Test Create with minimal data
func (s *AuditLogRepositoryTestSuite) TestCreate_MinimalData() {
	auditLog := &models.AuditLog{
		ID:             uuid.New(),
		AdminUserID:    s.testUserID,
		Action:         "test.minimal",
		IPAddress:      "127.0.0.1",
		UserAgent:      "TestAgent",
		ResponseStatus: 200,
		// No Resource, ResourceID, or RequestData
	}

	err := s.repo.Create(s.ctx, auditLog)
	assert.NoError(s.T(), err)

	// Verify it was created
	var count int
	err = s.db.QueryRow("SELECT COUNT(*) FROM admin_audit_logs WHERE id = $1", auditLog.ID).Scan(&count)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)
}

// Helper function for audit logs
func auditStringPtr(s string) *string {
	return &s
}

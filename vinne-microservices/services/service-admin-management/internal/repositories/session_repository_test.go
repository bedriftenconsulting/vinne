package repositories

import (
	"context"
	"database/sql"
	"fmt"
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

type SessionRepositoryTestSuite struct {
	suite.Suite
	db         *sql.DB
	repo       SessionRepository
	container  testcontainers.Container
	ctx        context.Context
	testHelper *TestHelper
}

func TestSessionRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(SessionRepositoryTestSuite))
}

func (s *SessionRepositoryTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.testHelper = NewTestHelper()

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

	// Run actual migrations from the migrations directory
	err = s.testHelper.SetupTestDB(s.T(), s.db)
	require.NoError(s.T(), err, "Migrations should apply successfully")

	// Initialize repository
	s.repo = NewSessionRepository(s.db)
}

func (s *SessionRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(s.T(), err)
	}
}

func (s *SessionRepositoryTestSuite) SetupTest() {
	// Clean up test data while preserving structure
	err := s.testHelper.CleanupTestData(s.T(), s.db)
	require.NoError(s.T(), err)

	// Seed default data
	err = s.testHelper.SeedDefaultData(s.T(), s.db)
	require.NoError(s.T(), err)

	// Create a test user
	err = s.createTestUser()
	require.NoError(s.T(), err)
}

// Test that validates migration correctness
func (s *SessionRepositoryTestSuite) TestMigrationValid() {
	// Verify session table exists with correct structure
	var exists bool
	err := s.db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'admin_sessions'
		)
	`).Scan(&exists)
	require.NoError(s.T(), err)
	require.True(s.T(), exists, "admin_sessions table should exist")

	// Verify indexes exist
	indexes := []string{
		"idx_admin_sessions_user_id",
		"idx_admin_sessions_refresh_token",
		"idx_admin_sessions_expires_at",
	}

	for _, idx := range indexes {
		err := s.db.QueryRow(`
			SELECT EXISTS (
				SELECT FROM pg_indexes 
				WHERE indexname = $1
			)
		`, idx).Scan(&exists)
		require.NoError(s.T(), err)
		require.True(s.T(), exists, fmt.Sprintf("Index %s should exist", idx))
	}
}

func (s *SessionRepositoryTestSuite) createTestUser() error {
	_, err := s.db.Exec(`
		INSERT INTO admin_users (id, email, username, password_hash)
		VALUES ($1, $2, $3, $4)
	`, uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"), "test@example.com", "testuser", "hashedpassword")
	return err
}

// Test Create
func (s *SessionRepositoryTestSuite) TestCreate() {
	session := &models.AdminSession{
		ID:           uuid.New(),
		UserID:       uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
		RefreshToken: "test-refresh-token-" + uuid.NewString(),
		UserAgent:    "Mozilla/5.0",
		IPAddress:    "127.0.0.1",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		IsActive:     true,
	}

	err := s.repo.Create(s.ctx, session)
	assert.NoError(s.T(), err)

	// Verify session was created
	var count int
	err = s.db.QueryRow("SELECT COUNT(*) FROM admin_sessions WHERE id = $1", session.ID).Scan(&count)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)
}

// Test GetByID
func (s *SessionRepositoryTestSuite) TestGetByID() {
	// Create a session
	sessionID := uuid.New()
	refreshToken := "test-refresh-token-" + uuid.NewString()
	_, err := s.db.Exec(`
		INSERT INTO admin_sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, sessionID, uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
		refreshToken, "Mozilla/5.0", "127.0.0.1",
		time.Now().Add(24*time.Hour), true)
	require.NoError(s.T(), err)

	// Get session by ID
	session, err := s.repo.GetByID(s.ctx, sessionID)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), session)
	assert.Equal(s.T(), sessionID, session.ID)
	assert.Equal(s.T(), refreshToken, session.RefreshToken)
}

// Test GetByToken
func (s *SessionRepositoryTestSuite) TestGetByToken() {
	// Create a session
	sessionID := uuid.New()
	refreshToken := "test-refresh-token-" + uuid.NewString()
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")

	_, err := s.db.Exec(`
		INSERT INTO admin_sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, sessionID, userID, refreshToken, "Mozilla/5.0", "127.0.0.1",
		time.Now().Add(24*time.Hour), true)
	require.NoError(s.T(), err)

	// Get session by token
	session, err := s.repo.GetByToken(s.ctx, refreshToken)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), session)
	assert.Equal(s.T(), sessionID, session.ID)
	assert.Equal(s.T(), userID, session.UserID)
}

// Test GetByUserID
func (s *SessionRepositoryTestSuite) TestGetByUserID() {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")

	// Create multiple sessions for the same user
	for i := 0; i < 3; i++ {
		_, err := s.db.Exec(`
			INSERT INTO admin_sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), userID, "token-"+uuid.NewString(),
			"Mozilla/5.0", "127.0.0.1", time.Now().Add(24*time.Hour), true)
		require.NoError(s.T(), err)
	}

	// Get sessions by user ID
	sessions, err := s.repo.GetByUserID(s.ctx, userID)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), sessions, 3)
}

// Test Update
func (s *SessionRepositoryTestSuite) TestUpdate() {
	// Create a session
	sessionID := uuid.New()
	oldToken := "old-token-" + uuid.NewString()

	_, err := s.db.Exec(`
		INSERT INTO admin_sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, sessionID, uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
		oldToken, "Mozilla/5.0", "127.0.0.1",
		time.Now().Add(24*time.Hour), true)
	require.NoError(s.T(), err)

	// Update the session
	newToken := "new-token-" + uuid.NewString()
	newExpiry := time.Now().Add(48 * time.Hour)
	session := &models.AdminSession{
		ID:           sessionID,
		RefreshToken: newToken,
		ExpiresAt:    newExpiry,
		IsActive:     false,
	}

	err = s.repo.Update(s.ctx, session)
	assert.NoError(s.T(), err)

	// Verify update
	var refreshToken string
	var isActive bool
	err = s.db.QueryRow("SELECT refresh_token, is_active FROM admin_sessions WHERE id = $1", sessionID).
		Scan(&refreshToken, &isActive)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), newToken, refreshToken)
	assert.False(s.T(), isActive)
}

// Test InvalidateByToken
func (s *SessionRepositoryTestSuite) TestInvalidateByToken() {
	// Create an active session
	refreshToken := "test-token-" + uuid.NewString()

	_, err := s.db.Exec(`
		INSERT INTO admin_sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, uuid.New(), uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
		refreshToken, "Mozilla/5.0", "127.0.0.1",
		time.Now().Add(24*time.Hour), true)
	require.NoError(s.T(), err)

	// Invalidate the session
	err = s.repo.InvalidateByToken(s.ctx, refreshToken)
	assert.NoError(s.T(), err)

	// Verify session is inactive
	var isActive bool
	err = s.db.QueryRow("SELECT is_active FROM admin_sessions WHERE refresh_token = $1", refreshToken).
		Scan(&isActive)
	assert.NoError(s.T(), err)
	assert.False(s.T(), isActive)
}

// Test InvalidateAllForUser
func (s *SessionRepositoryTestSuite) TestInvalidateAllForUser() {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")

	// Create multiple active sessions
	for i := 0; i < 3; i++ {
		_, err := s.db.Exec(`
			INSERT INTO admin_sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), userID, "token-"+uuid.NewString(),
			"Mozilla/5.0", "127.0.0.1", time.Now().Add(24*time.Hour), true)
		require.NoError(s.T(), err)
	}

	// Invalidate all sessions for user
	err := s.repo.InvalidateAllForUser(s.ctx, userID)
	assert.NoError(s.T(), err)

	// Verify all sessions are inactive
	var activeCount int
	err = s.db.QueryRow("SELECT COUNT(*) FROM admin_sessions WHERE user_id = $1 AND is_active = true", userID).
		Scan(&activeCount)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 0, activeCount)
}

// Test DeleteExpired
func (s *SessionRepositoryTestSuite) TestDeleteExpired() {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")

	// Create expired session
	_, err := s.db.Exec(`
		INSERT INTO admin_sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, uuid.New(), userID, "expired-token-"+uuid.NewString(),
		"Mozilla/5.0", "127.0.0.1", time.Now().Add(-24*time.Hour), true)
	require.NoError(s.T(), err)

	// Create valid session
	_, err = s.db.Exec(`
		INSERT INTO admin_sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, uuid.New(), userID, "valid-token-"+uuid.NewString(),
		"Mozilla/5.0", "127.0.0.1", time.Now().Add(24*time.Hour), true)
	require.NoError(s.T(), err)

	// Delete expired sessions
	err = s.repo.DeleteExpired(s.ctx)
	assert.NoError(s.T(), err)

	// Verify only valid session remains
	var count int
	err = s.db.QueryRow("SELECT COUNT(*) FROM admin_sessions WHERE user_id = $1", userID).Scan(&count)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)
}

// Test GetByID with non-existent ID
func (s *SessionRepositoryTestSuite) TestGetByID_NotFound() {
	session, err := s.repo.GetByID(s.ctx, uuid.New())
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), session)
}

// Test GetByToken with non-existent token
func (s *SessionRepositoryTestSuite) TestGetByToken_NotFound() {
	session, err := s.repo.GetByToken(s.ctx, "non-existent-token")
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), session)
}

// Test Create with duplicate token
func (s *SessionRepositoryTestSuite) TestCreate_DuplicateToken() {
	refreshToken := "duplicate-token-" + uuid.NewString()
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")

	// Create first session
	session1 := &models.AdminSession{
		ID:           uuid.New(),
		UserID:       userID,
		RefreshToken: refreshToken,
		UserAgent:    "Mozilla/5.0",
		IPAddress:    "127.0.0.1",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		IsActive:     true,
	}
	err := s.repo.Create(s.ctx, session1)
	assert.NoError(s.T(), err)

	// Try to create second session with same token
	session2 := &models.AdminSession{
		ID:           uuid.New(),
		UserID:       userID,
		RefreshToken: refreshToken,
		UserAgent:    "Chrome/96.0",
		IPAddress:    "192.168.1.1",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		IsActive:     true,
	}
	err = s.repo.Create(s.ctx, session2)
	assert.Error(s.T(), err)
}

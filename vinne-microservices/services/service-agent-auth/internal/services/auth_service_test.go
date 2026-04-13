package services

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	redisClient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"

	_ "github.com/lib/pq"
	pb "github.com/randco/randco-microservices/proto/agent/auth/v1"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/clients"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/repositories"
)

// AuthServiceTestSuite is the test suite for auth service integration tests
type AuthServiceTestSuite struct {
	suite.Suite
	db                *sqlx.DB
	sqlDB             *sql.DB
	redis             *redisClient.Client
	postgresContainer *postgres.PostgresContainer
	redisContainer    testcontainers.Container
	authService       AuthService
	authRepo          repositories.AuthRepository
	sessionRepo       repositories.SessionRepository
	tokenRepo         repositories.TokenRepositoryInterface
	offlineTokenRepo  repositories.OfflineTokenRepository
}

// TestAuthServiceTestSuite runs the auth service test suite
func TestAuthServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AuthServiceTestSuite))
}

// SetupSuite sets up the test infrastructure once for all tests
func (s *AuthServiceTestSuite) SetupSuite() {
	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("auth_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(s.T(), err)
	s.postgresContainer = postgresContainer

	// Start Redis container
	redisContainer, err := redis.Run(ctx,
		"redis:7-alpine",
		redis.WithSnapshotting(10, 1),
	)
	require.NoError(s.T(), err)
	s.redisContainer = redisContainer

	// Get PostgreSQL connection details
	dbHost, err := postgresContainer.Host(ctx)
	require.NoError(s.T(), err)
	dbPort, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(s.T(), err)

	// Connect to PostgreSQL
	dsn := fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=auth_test sslmode=disable",
		dbHost, dbPort.Port())
	db, err := sqlx.Connect("postgres", dsn)
	require.NoError(s.T(), err)
	s.db = db
	s.sqlDB = db.DB

	// Get Redis connection details
	redisHost, err := redisContainer.Host(ctx)
	require.NoError(s.T(), err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(s.T(), err)

	// Connect to Redis
	s.redis = redisClient.NewClient(&redisClient.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort.Port()),
	})

	// Run database migrations
	s.runMigrations()

	// Initialize repositories
	s.authRepo = repositories.NewAuthRepository(s.db, s.redis)
	s.sessionRepo = repositories.NewSessionRepository(s.sqlDB, s.redis)
	s.tokenRepo = repositories.NewTokenRepository(s.redis)
	s.offlineTokenRepo = repositories.NewOfflineTokenRepository(s.db)

	// Initialize auth service with real repositories
	s.authService = NewAuthService(
		s.authRepo,
		s.sessionRepo,
		s.tokenRepo,
		s.offlineTokenRepo,
		15*time.Minute, // access token expiry
		7*24*time.Hour, // refresh token expiry
		3,              // max failed logins
		30*time.Minute, // lockout duration
		clients.AgentNotificationClient{},
		nil,
	)
}

// TearDownSuite cleans up after all tests
func (s *AuthServiceTestSuite) TearDownSuite() {
	ctx := context.Background()
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.redis != nil {
		_ = s.redis.Close()
	}
	if s.postgresContainer != nil {
		_ = s.postgresContainer.Terminate(ctx)
	}
	if s.redisContainer != nil {
		_ = s.redisContainer.Terminate(ctx)
	}
}

// SetupTest cleans data before each test
func (s *AuthServiceTestSuite) SetupTest() {
	// Clean up test data
	s.cleanupTestData()
}

// runMigrations runs database migrations using Goose
func (s *AuthServiceTestSuite) runMigrations() {
	// Set Goose dialect
	err := goose.SetDialect("postgres")
	require.NoError(s.T(), err)

	// Get migrations directory
	migrationsDir := filepath.Join("..", "..", "migrations")

	// Run migrations
	err = goose.Up(s.sqlDB, migrationsDir)
	require.NoError(s.T(), err)

	s.T().Log("✅ Successfully ran all migrations")
}

// cleanupTestData removes all test data from tables
func (s *AuthServiceTestSuite) cleanupTestData() {
	ctx := context.Background()

	// Clean up in reverse order of foreign key dependencies
	tables := []string{
		"auth_audit_logs",
		"password_reset_tokens",
		"offline_auth_tokens",
		"auth_sessions",
		"retailers_auth",
		"agents_auth",
		"password_reset_logs",
	}

	for _, table := range tables {
		_, err := s.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			s.T().Logf("Warning: Failed to clean table %s: %v", table, err)
		}
	}

	// Clear Redis
	_ = s.redis.FlushAll(ctx)
}

// Test Agent Login with valid credentials
func (s *AuthServiceTestSuite) TestAgentLogin_Success() {
	ctx := context.Background()

	// Create test agent in database
	agentID := uuid.New()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Test@123456"), bcrypt.DefaultCost)
	require.NoError(s.T(), err)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, "1001", "agent@test.com", "+233500000001", string(hashedPassword), true)
	require.NoError(s.T(), err)

	// Test login
	req := &LoginRequest{
		Identifier: "agent@test.com",
		Password:   "Test@123456",
		UserType:   "AGENT",
		DeviceID:   "test-device",
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
	}

	resp, err := s.authService.AgentLogin(ctx, req)

	// Assertions
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.NotEmpty(s.T(), resp.AccessToken)
	assert.NotEmpty(s.T(), resp.RefreshToken)
	assert.Equal(s.T(), agentID, resp.UserID)
	assert.Equal(s.T(), "AGENT", resp.UserType)
	assert.Equal(s.T(), "1001", resp.UserCode)
	assert.Equal(s.T(), "agent@test.com", resp.Email)
	assert.Equal(s.T(), "+233500000001", resp.Phone)

	// Verify JWT token is valid
	token, err := jwt.Parse(resp.AccessToken, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret-key-at-least-32-characters-long"), nil
	})
	require.NoError(s.T(), err)
	assert.True(s.T(), token.Valid)

	// Verify session was created in database
	var sessionCount int
	err = s.db.GetContext(ctx, &sessionCount,
		"SELECT COUNT(*) FROM auth_sessions WHERE user_id = $1 AND is_active = true", agentID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, sessionCount)

	// Verify last login was updated
	var lastLogin *time.Time
	err = s.db.GetContext(ctx, &lastLogin,
		"SELECT last_login_at FROM agents_auth WHERE id = $1", agentID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), lastLogin)
	assert.WithinDuration(s.T(), time.Now(), *lastLogin, 5*time.Second)
}

// Test Agent Login with invalid password
func (s *AuthServiceTestSuite) TestAgentLogin_InvalidPassword() {
	ctx := context.Background()

	// Create test agent
	agentID := uuid.New()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Test@123456"), bcrypt.DefaultCost)
	require.NoError(s.T(), err)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, "1002", "agent2@test.com", "+233500000002", string(hashedPassword), true)
	require.NoError(s.T(), err)

	// Test login with wrong password
	req := &LoginRequest{
		Identifier: "agent2@test.com",
		Password:   "WrongPassword",
		UserType:   "AGENT",
	}

	resp, err := s.authService.AgentLogin(ctx, req)

	// Assertions
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "invalid credentials")

	// Verify failed login attempt was recorded
	var failedAttempts int
	err = s.db.GetContext(ctx, &failedAttempts,
		"SELECT failed_login_attempts FROM agents_auth WHERE id = $1", agentID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, failedAttempts)
}

// Test Agent Login with account lockout after max failed attempts
func (s *AuthServiceTestSuite) TestAgentLogin_AccountLockout() {
	ctx := context.Background()

	// Create test agent
	agentID := uuid.New()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Test@123456"), bcrypt.DefaultCost)
	require.NoError(s.T(), err)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, "1003", "agent3@test.com", "+233500000003", string(hashedPassword), true)
	require.NoError(s.T(), err)

	// Attempt login with wrong password 3 times
	req := &LoginRequest{
		Identifier: "agent3@test.com",
		Password:   "WrongPassword",
		UserType:   "AGENT",
	}

	// First 2 attempts should fail with invalid credentials
	for i := 0; i < 2; i++ {
		resp, err := s.authService.AgentLogin(ctx, req)
		assert.Error(s.T(), err)
		assert.Nil(s.T(), resp)
		assert.Contains(s.T(), err.Error(), "invalid credentials")
	}

	// Third attempt should trigger account lock
	resp, err := s.authService.AgentLogin(ctx, req)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "account locked")

	// Verify account is locked in database
	var lockedUntil *time.Time
	err = s.db.GetContext(ctx, &lockedUntil,
		"SELECT locked_until FROM agents_auth WHERE id = $1", agentID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), lockedUntil)
	assert.True(s.T(), lockedUntil.After(time.Now()))

	// Verify correct password also fails when locked
	req.Password = "Test@123456"
	resp, err = s.authService.AgentLogin(ctx, req)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "account locked")
}

// Test Agent Login with inactive account
func (s *AuthServiceTestSuite) TestAgentLogin_InactiveAccount() {
	ctx := context.Background()

	// Create inactive agent
	agentID := uuid.New()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Test@123456"), bcrypt.DefaultCost)
	require.NoError(s.T(), err)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, "1004", "inactive@test.com", "+233500000004", string(hashedPassword), false)
	require.NoError(s.T(), err)

	// Test login
	req := &LoginRequest{
		Identifier: "inactive@test.com",
		Password:   "Test@123456",
		UserType:   "AGENT",
	}

	resp, err := s.authService.AgentLogin(ctx, req)

	// Assertions
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "account is inactive")
}

// Test Agent Login with non-existent user
func (s *AuthServiceTestSuite) TestAgentLogin_UserNotFound() {
	ctx := context.Background()

	req := &LoginRequest{
		Identifier: "nonexistent@test.com",
		Password:   "Test@123456",
		UserType:   "AGENT",
	}

	resp, err := s.authService.AgentLogin(ctx, req)

	// Assertions
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "invalid credentials")
}

// Test Agent Login with different identifiers (email, phone, code)
func (s *AuthServiceTestSuite) TestAgentLogin_DifferentIdentifiers() {
	ctx := context.Background()

	// Create test agent
	agentID := uuid.New()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Test@123456"), bcrypt.DefaultCost)
	require.NoError(s.T(), err)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, "1005", "multi@test.com", "+233500000005", string(hashedPassword), true)
	require.NoError(s.T(), err)

	testCases := []struct {
		name       string
		identifier string
	}{
		{"Login with email", "multi@test.com"},
		{"Login with phone", "+233500000005"},
		{"Login with agent code", "1005"},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			req := &LoginRequest{
				Identifier: tc.identifier,
				Password:   "Test@123456",
				UserType:   "AGENT",
			}

			resp, err := s.authService.AgentLogin(ctx, req)

			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, agentID, resp.UserID)
			assert.Equal(t, "1005", resp.UserCode)
		})
	}
}

// Test RefreshToken with valid token
func (s *AuthServiceTestSuite) TestRefreshToken_Success() {
	ctx := context.Background()

	// Create test agent and login to get tokens
	agentID := uuid.New()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Test@123456"), bcrypt.DefaultCost)
	require.NoError(s.T(), err)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, "1006", "refresh@test.com", "+233500000006", string(hashedPassword), true)
	require.NoError(s.T(), err)

	// Login to get initial tokens
	loginReq := &LoginRequest{
		Identifier: "refresh@test.com",
		Password:   "Test@123456",
		UserType:   "AGENT",
	}

	loginResp, err := s.authService.AgentLogin(ctx, loginReq)
	require.NoError(s.T(), err)

	// Wait a moment to ensure different token generation
	time.Sleep(100 * time.Millisecond)

	// Refresh the token
	refreshResp, err := s.authService.RefreshToken(ctx, loginResp.RefreshToken)

	// Assertions
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), refreshResp)
	assert.NotEmpty(s.T(), refreshResp.AccessToken)
	assert.NotEmpty(s.T(), refreshResp.RefreshToken)
	assert.NotEqual(s.T(), loginResp.RefreshToken, refreshResp.RefreshToken) // New refresh token
	assert.NotEqual(s.T(), loginResp.AccessToken, refreshResp.AccessToken)   // New access token
	assert.Equal(s.T(), agentID, refreshResp.UserID)

	// Verify old session was revoked
	var oldSessionActive bool
	err = s.db.GetContext(ctx, &oldSessionActive,
		"SELECT is_active FROM auth_sessions WHERE refresh_token = $1", loginResp.RefreshToken)
	require.NoError(s.T(), err)
	assert.False(s.T(), oldSessionActive)

	// Verify new session was created
	var newSessionCount int
	err = s.db.GetContext(ctx, &newSessionCount,
		"SELECT COUNT(*) FROM auth_sessions WHERE refresh_token = $1 AND is_active = true",
		refreshResp.RefreshToken)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, newSessionCount)
}

// Test RefreshToken with expired token
func (s *AuthServiceTestSuite) TestRefreshToken_Expired() {
	ctx := context.Background()

	// Create an expired session directly in database
	sessionID := uuid.New()
	userID := uuid.New()
	expiredToken := "expired-refresh-token"

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_sessions (id, user_id, user_type, refresh_token, is_active, expires_at, created_at, last_activity)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		sessionID, userID, "AGENT", expiredToken, true,
		time.Now().Add(-1*time.Hour), // Expired
		time.Now().Add(-2*time.Hour),
		time.Now().Add(-1*time.Hour))
	require.NoError(s.T(), err)

	// Try to refresh
	resp, err := s.authService.RefreshToken(ctx, expiredToken)

	// Assertions
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "refresh token expired")
}

// Test Logout
func (s *AuthServiceTestSuite) TestLogout_Success() {
	ctx := context.Background()

	// Create test agent and login
	agentID := uuid.New()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Test@123456"), bcrypt.DefaultCost)
	require.NoError(s.T(), err)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, "1007", "logout@test.com", "+233500000007", string(hashedPassword), true)
	require.NoError(s.T(), err)

	// Login
	loginReq := &LoginRequest{
		Identifier: "logout@test.com",
		Password:   "Test@123456",
		UserType:   "AGENT",
	}

	loginResp, err := s.authService.AgentLogin(ctx, loginReq)
	require.NoError(s.T(), err)

	// Logout
	err = s.authService.Logout(ctx, loginResp.RefreshToken)
	require.NoError(s.T(), err)

	// Verify session was revoked
	var sessionActive bool
	err = s.db.GetContext(ctx, &sessionActive,
		"SELECT is_active FROM auth_sessions WHERE refresh_token = $1", loginResp.RefreshToken)
	require.NoError(s.T(), err)
	assert.False(s.T(), sessionActive)

	// Verify refresh token no longer works
	resp, err := s.authService.RefreshToken(ctx, loginResp.RefreshToken)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
}

// Test CreateAgentAuth
func (s *AuthServiceTestSuite) TestCreateAgentAuth_Success() {
	ctx := context.Background()

	agentID := uuid.New()
	err := s.authService.CreateAgentAuth(ctx,
		agentID, "1008", "newagent@test.com", "+233500000008", "Test@123456", "admin")

	// Assertions
	require.NoError(s.T(), err)

	// Verify agent was created in database
	var count int
	err = s.db.GetContext(ctx, &count,
		"SELECT COUNT(*) FROM agents_auth WHERE id = $1 AND agent_code = $2", agentID, "1008")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)

	// Verify password was hashed
	var passwordHash string
	err = s.db.GetContext(ctx, &passwordHash,
		"SELECT password_hash FROM agents_auth WHERE id = $1", agentID)
	require.NoError(s.T(), err)
	assert.NotEqual(s.T(), "Test@123456", passwordHash)

	// Verify can login with created credentials
	loginReq := &LoginRequest{
		Identifier: "newagent@test.com",
		Password:   "Test@123456",
		UserType:   "AGENT",
	}

	resp, err := s.authService.AgentLogin(ctx, loginReq)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), agentID, resp.UserID)
}

// Test CreateAgentAuth with duplicate agent code
func (s *AuthServiceTestSuite) TestCreateAgentAuth_DuplicateCode() {
	ctx := context.Background()

	// Create first agent
	firstID := uuid.New()
	err := s.authService.CreateAgentAuth(ctx,
		firstID, "1009", "first@test.com", "+233500000009", "Test@123456", "admin")
	require.NoError(s.T(), err)

	// Try to create second agent with same code
	secondID := uuid.New()
	err = s.authService.CreateAgentAuth(ctx,
		secondID, "1009", "second@test.com", "+233500000010", "Test@123456", "admin")

	// Assertions
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "already exists")
}

// Test CreateAgentAuth with duplicate email
func (s *AuthServiceTestSuite) TestCreateAgentAuth_DuplicateEmail() {
	ctx := context.Background()

	// Create first agent
	firstID := uuid.New()
	err := s.authService.CreateAgentAuth(ctx,
		firstID, "1010", "duplicate@test.com", "+233500000011", "Test@123456", "admin")
	require.NoError(s.T(), err)

	// Try to create second agent with same email
	secondID := uuid.New()
	err = s.authService.CreateAgentAuth(ctx,
		secondID, "1011", "duplicate@test.com", "+233500000012", "Test@123456", "admin")

	// Assertions
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "already exists")
}

// Test CreateAgentAuth with duplicate phone
func (s *AuthServiceTestSuite) TestCreateAgentAuth_DuplicatePhone() {
	ctx := context.Background()

	// Create first agent
	firstID := uuid.New()
	err := s.authService.CreateAgentAuth(ctx,
		firstID, "1012", "phone1@test.com", "+233500000013", "Test@123456", "admin")
	require.NoError(s.T(), err)

	// Try to create second agent with same phone
	secondID := uuid.New()
	err = s.authService.CreateAgentAuth(ctx,
		secondID, "1013", "phone2@test.com", "+233500000013", "Test@123456", "admin")

	// Assertions
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "already exists")
}

// Test ChangePassword
func (s *AuthServiceTestSuite) TestChangePassword_Success() {
	ctx := context.Background()

	// Create test agent
	agentID := uuid.New()
	oldPassword := "OldPass@123456"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.DefaultCost)
	require.NoError(s.T(), err)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, "1014", "changepass@test.com", "+233500000014", string(hashedPassword), true)
	require.NoError(s.T(), err)

	// Login first to create a session
	loginReq := &LoginRequest{
		Identifier: "changepass@test.com",
		Password:   oldPassword,
		UserType:   "AGENT",
	}
	loginResp, err := s.authService.AgentLogin(ctx, loginReq)
	require.NoError(s.T(), err)

	// Change password
	newPassword := "NewPass@123456"
	err = s.authService.ChangePassword(ctx, agentID, "AGENT", oldPassword, newPassword)
	require.NoError(s.T(), err)

	// Verify old password no longer works
	loginReq.Password = oldPassword
	resp, err := s.authService.AgentLogin(ctx, loginReq)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)

	// Verify new password works
	loginReq.Password = newPassword
	resp, err = s.authService.AgentLogin(ctx, loginReq)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)

	// Verify old session was revoked
	var sessionActive bool
	err = s.db.GetContext(ctx, &sessionActive,
		"SELECT is_active FROM auth_sessions WHERE refresh_token = $1", loginResp.RefreshToken)
	require.NoError(s.T(), err)
	assert.False(s.T(), sessionActive)
}

func (s *AuthServiceTestSuite) TestChangeRetailerPIN_Success() {
	ctx := context.Background()

	// Create test retailer
	retailerID := uuid.New()
	oldPIN := "Th1sP@ssword1sforthestreets"
	hashedPIN, err := bcrypt.GenerateFromPassword([]byte(oldPIN), bcrypt.DefaultCost)
	require.NoError(s.T(), err)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO retailers_auth (id, retailer_code, email, phone, password_hash, pin_hash, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		retailerID, "2025", "changepin@test.com", "+233500000014", string(hashedPIN), string(hashedPIN), true, time.Now(), time.Now())
	require.NoError(s.T(), err)

	// Login first to create a session
	loginReq := &LoginRequest{
		Identifier: "changepin@test.com",
		Password:   oldPIN,
		UserType:   "RETAILER",
	}
	loginResp, err := s.authService.RetailerLogin(ctx, loginReq)
	require.NoError(s.T(), err)

	// Change PIN
	newPIN := "ForgetMyP@st420"
	_, err = s.authService.ChangeRetailerPIN(ctx, retailerID, oldPIN, newPIN, "867400020220040", "2025", "192.168.1.100")
	require.NoError(s.T(), err)

	// Verify old pin no longer works
	loginReq.Password = oldPIN
	resp, err := s.authService.RetailerLogin(ctx, loginReq)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)

	// Verify new pin works
	loginReq.Password = newPIN
	resp, err = s.authService.RetailerLogin(ctx, loginReq)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)

	// Verify old session was revoked
	var sessionActive bool
	err = s.db.GetContext(ctx, &sessionActive,
		"SELECT is_active FROM auth_sessions WHERE refresh_token = $1", loginResp.RefreshToken)
	require.NoError(s.T(), err)
	assert.False(s.T(), sessionActive)

	// verify PIN audit log was created
	var logCount int
	err = s.db.GetContext(ctx, &logCount,
		"SELECT COUNT(*) FROM retailer_pin_change_logs WHERE retailer_id = $1 AND success = $2",
		retailerID, true)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, logCount)
}

// Test password validation
func (s *AuthServiceTestSuite) TestCreateAgentAuth_WeakPassword() {
	ctx := context.Background()

	weakPasswords := []string{
		"weak",  // Too short (less than 6)
		"12345", // Too short (less than 6)
	}

	for i, weakPass := range weakPasswords {
		agentID := uuid.New()
		agentCode := fmt.Sprintf("%d", 2000+i)
		email := fmt.Sprintf("weak%d@test.com", i)
		phone := fmt.Sprintf("+233500001%03d", i)

		err := s.authService.CreateAgentAuth(ctx, agentID, agentCode, email, phone, weakPass, "admin")

		assert.Error(s.T(), err, "Password '%s' should be rejected", weakPass)
		assert.Contains(s.T(), err.Error(), "at least 6 characters", "Should indicate minimum length requirement for '%s'", weakPass)
	}
}

// Test that 6-digit passwords are accepted (for auto-generated passwords)
func (s *AuthServiceTestSuite) TestCreateAgentAuth_SixDigitPassword() {
	ctx := context.Background()

	// Test 6-digit password is accepted
	agentID := uuid.New()
	agentCode := "3000"
	email := "sixdigit@test.com"
	phone := "+233500003000"
	password := "123456" // 6-digit password

	err := s.authService.CreateAgentAuth(ctx, agentID, agentCode, email, phone, password, "admin")

	// Should succeed with 6-digit password
	assert.NoError(s.T(), err, "6-digit password should be accepted")

	// Verify agent was created
	var exists bool
	err = s.db.GetContext(ctx, &exists,
		"SELECT EXISTS(SELECT 1 FROM agents_auth WHERE agent_code = $1)", agentCode)
	require.NoError(s.T(), err)
	assert.True(s.T(), exists, "Agent should be created with 6-digit password")
}

// Test concurrent login attempts
func (s *AuthServiceTestSuite) TestAgentLogin_Concurrent() {
	ctx := context.Background()

	// Create test agent
	agentID := uuid.New()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Test@123456"), bcrypt.DefaultCost)
	require.NoError(s.T(), err)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, "1015", "concurrent@test.com", "+233500000015", string(hashedPassword), true)
	require.NoError(s.T(), err)

	// Run multiple concurrent login attempts
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func(deviceNum int) {
			req := &LoginRequest{
				Identifier: "concurrent@test.com",
				Password:   "Test@123456",
				UserType:   "AGENT",
				DeviceID:   fmt.Sprintf("device-%d", deviceNum),
				IPAddress:  "127.0.0.1",
			}

			resp, err := s.authService.AgentLogin(ctx, req)
			assert.NoError(s.T(), err)
			assert.NotNil(s.T(), resp)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify all sessions were created
	var sessionCount int
	err = s.db.GetContext(ctx, &sessionCount,
		"SELECT COUNT(*) FROM auth_sessions WHERE user_id = $1 AND is_active = true", agentID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 5, sessionCount, "All concurrent sessions should be created")
}

func (s *AuthServiceTestSuite) TestRequestPasswordReset_ValidAgent() {
	ctx := context.Background()

	// Create a valid agent
	agentID := uuid.New()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Password@123"), bcrypt.DefaultCost)
	require.NoError(s.T(), err)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, "114412", "validagent@test.com", "+2335000114412", string(hashedPassword), true)
	require.NoError(s.T(), err)

	// request for password reset
	req := &pb.PasswordResetRequest{
		Identifier: "114412", // using agent_code
		UserType:   "AGENT",
		Channel:    "email",
		IpAddress:  "127.0.0.1",
	}

	// Call the service
	resp, err := s.authService.RequestPasswordReset(ctx, req)

	// Assertions
	require.NoError(s.T(), err, "RequestPasswordReset should not return an error for valid agent")
	assert.True(s.T(), resp.Success)
	assert.NotEmpty(s.T(), resp.ResetToken)
	assert.Equal(s.T(), "If the account exists, an OTP has been sent", resp.Message)

	// Verify OTP hash stored in Redis
	storedData, err := s.redis.HGetAll(ctx, "reset_token:"+resp.ResetToken).Result()
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), storedData, "Reset token should exist in Redis")

	assert.Equal(s.T(), agentID.String(), storedData["agent_id"])
	assert.NotEmpty(s.T(), storedData["token"], "Token hash should be stored")

	// Verify expiry time
	ttl, err := s.redis.TTL(ctx, "reset_token:"+resp.ResetToken).Result()
	require.NoError(s.T(), err)
	assert.GreaterOrEqual(s.T(), int64(ttl.Seconds()), int64(290)) // close to 300 seconds
}

func (s *AuthServiceTestSuite) TestRequestPasswordReset_InvalidAgent() {
	ctx := context.Background()

	// request for an invalid agent
	req := &pb.PasswordResetRequest{
		Identifier: "999999",
		UserType:   "AGENT",
		Channel:    "email",
		IpAddress:  "127.0.0.1",
	}

	// Call the service
	resp, err := s.authService.RequestPasswordReset(ctx, req)

	// Assertions
	require.NoError(s.T(), err, "RequestPasswordReset should not error for invalid agent")
	assert.NotNil(s.T(), resp)
	assert.False(s.T(), resp.Success, "Response should not be marked as success for invalid agent")
	assert.Equal(s.T(), "If the account exists, an OTP has been sent", resp.Message)

	// Ensure no reset token was returned or stored
	assert.Empty(s.T(), resp.ResetToken, "No reset token should be returned for invalid agent")

	// no unexpected keys should exist in redis
	iter := s.redis.Scan(ctx, 0, "reset_token:*", 0).Iterator()
	hasKeys := false
	for iter.Next(ctx) {
		hasKeys = true
		break
	}
	require.NoError(s.T(), iter.Err())
	assert.False(s.T(), hasKeys, "No reset_token keys should be created in Redis for invalid agent")
}

func (s *AuthServiceTestSuite) TestRequestPasswordReset_RateLimited() {
	ctx := context.Background()

	// Create test agent
	agentID := uuid.New()
	email := "ratelimit@test.com"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("Test@123456"), bcrypt.DefaultCost)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, "9999", email, "+23350009999", string(hashedPassword), true)
	s.Require().NoError(err)

	req := &pb.PasswordResetRequest{
		Identifier: email,
		UserType:   "AGENT",
		Channel:    "email",
	}

	// Simulate previous reset attempts to hit the limit
	for i := 0; i < 3; i++ {
		res, err := s.authService.RequestPasswordReset(ctx, req)
		s.Require().NoError(err)
		s.True(res.Success)
	}

	// Fourth attempt should hit the rate limit
	res, err := s.authService.RequestPasswordReset(ctx, req)
	s.Require().NoError(err)
	s.False(res.Success)
	s.Contains(res.Message, "Only 3 requests per hour allowed")
}

func (s *AuthServiceTestSuite) TestValidateOTP_Success() {
	ctx := context.Background()

	// Create test agent
	agentID := uuid.New()
	email := "otpvalid@test.com"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("Test@123456"), bcrypt.DefaultCost)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, "5000", email, "+23350005000", string(hashedPassword), true)
	s.Require().NoError(err)

	// Generate OTP
	otp := "123456"
	otpHash, _ := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	resetToken := uuid.NewString()

	// Store OTP in Redis for 5 minutes
	err = s.tokenRepo.StoreResetToken(ctx, resetToken, agentID.String(), string(otpHash), 5*time.Minute)
	s.Require().NoError(err)

	// Validate OTP
	req := &pb.ValidateResetOTPRequest{
		ResetToken: resetToken,
		OtpCode:    otp,
	}
	resp, err := s.authService.ValidateResetOTP(ctx, req)
	s.Require().NoError(err)

	s.True(resp.Valid, "OTP should be valid")
	s.Equal("OTP is valid", resp.Message)
	s.Equal(int32(3), resp.RemainingAttempts)
}

func (s *AuthServiceTestSuite) TestValidateOTP_Expired() {
	ctx := context.Background()

	// create a reset token that doesn't exist in Redis
	resetToken := uuid.NewString()

	req := &pb.ValidateResetOTPRequest{
		ResetToken: resetToken,
		OtpCode:    "123456",
	}

	// Act
	resp, err := s.authService.ValidateResetOTP(ctx, req)

	// Assert
	require.NoError(s.T(), err)
	require.NotNil(s.T(), resp)
	assert.False(s.T(), resp.Valid, "Expired token should not be valid")
	assert.Equal(s.T(), "Invalid or expired token", resp.Message)
}

func (s *AuthServiceTestSuite) TestConfirmPasswordReset_Success() {
	ctx := context.Background()

	// Create test agent
	agentID := uuid.New()
	email := "confirmreset@test.com"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("OldPass@123"), bcrypt.DefaultCost)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agents_auth (id, agent_code, email, phone, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, "6000", email, "+23350006000", string(hashedPassword), true)
	s.Require().NoError(err)

	// Generate OTP and store in Redis
	otp := "654321"
	otpHash, _ := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	resetToken := uuid.NewString()
	err = s.tokenRepo.StoreResetToken(ctx, resetToken, agentID.String(), string(otpHash), 5*time.Minute)
	s.Require().NoError(err)

	// Prepare request
	req := &pb.ConfirmPasswordResetRequest{
		ResetToken:      resetToken,
		OtpCode:         otp,
		NewPassword:     "NewPass@123",
		ConfirmPassword: "NewPass@123",
	}

	// Call service
	resp, err := s.authService.ConfirmPasswordReset(ctx, req)
	s.Require().NoError(err)

	// Assertions
	s.True(resp.Success, "expected success")
	s.Equal("Password reset successful", resp.Message)

	// Verify password was updated
	var updatedHash string
	err = s.db.GetContext(ctx, &updatedHash, "SELECT password_hash FROM agents_auth WHERE id=$1", agentID)
	s.Require().NoError(err)
	s.NotEqual(string(hashedPassword), updatedHash, "password hash should have changed")

	// Verify new password matches stored hash
	err = bcrypt.CompareHashAndPassword([]byte(updatedHash), []byte(req.NewPassword))
	s.Require().NoError(err, "new password should match stored hash")
}

func (s *AuthServiceTestSuite) TestConfirmPasswordReset_PasswordValidation() {
	ctx := context.Background()

	req := &pb.ConfirmPasswordResetRequest{
		ResetToken:      "dummy-reset-token",
		OtpCode:         "123456",
		NewPassword:     "weakpass",
		ConfirmPassword: "weakpass",
	}

	resp, err := s.authService.ConfirmPasswordReset(ctx, req)

	require.NoError(s.T(), err)
	assert.False(s.T(), resp.Success)
	assert.Equal(s.T(),
		"Password  must be at least 8 characters and include uppercase, lowercase, number, and special character",
		resp.Message)
}

func (s *AuthServiceTestSuite) TestConfirmPasswordReset_PasswordMismatch() {
	ctx := context.Background()

	req := &pb.ConfirmPasswordResetRequest{
		ResetToken:      "dummy-reset-token",
		OtpCode:         "123456",
		NewPassword:     "StrongPass@124",
		ConfirmPassword: "StrongPass@123",
	}

	resp, err := s.authService.ConfirmPasswordReset(ctx, req)

	require.NoError(s.T(), err)
	assert.False(s.T(), resp.Success)
	assert.Equal(s.T(), "Passwords do not match", resp.Message)
}

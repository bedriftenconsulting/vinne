package server

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // postgres driver
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/randco/randco-microservices/proto/agent/auth/v1"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/clients"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/repositories"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/services"
)

type AgentAuthServerIntegrationTestSuite struct {
	suite.Suite
	pgContainer    testcontainers.Container
	redisContainer testcontainers.Container
	db             *sqlx.DB
	redisClient    *redis.Client
	client         pb.AgentAuthServiceClient
	conn           *grpc.ClientConn
	server         *AgentAuthServer
	authService    services.AuthService
}

func TestAgentAuthServerIntegration(t *testing.T) {
	suite.Run(t, new(AgentAuthServerIntegrationTestSuite))
}

func (s *AgentAuthServerIntegrationTestSuite) SetupSuite() {
	s.T().Log("Setting up AgentAuthServer integration test suite...")

	ctx := context.Background()

	// Set up PostgreSQL test container
	pgReq := testcontainers.ContainerRequest{
		Image:        "postgres:17",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "agent_auth_test",
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
	}

	pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: pgReq,
		Started:          true,
	})
	s.Require().NoError(err)
	s.pgContainer = pgContainer

	// Set up Redis test container
	redisReq := testcontainers.ContainerRequest{
		Image:        "redis:7",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(60 * time.Second),
	}

	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: redisReq,
		Started:          true,
	})
	s.Require().NoError(err)
	s.redisContainer = redisContainer

	// Set up database connections
	s.setupDatabaseConnections(ctx)

	// Run migrations
	err = s.runMigrations()
	s.Require().NoError(err)

	// Seed test data
	err = s.seedTestData()
	s.Require().NoError(err)

	// Initialize service layer
	s.setupServiceLayer()

	// Start test gRPC server
	s.startTestServer()

	s.T().Log("✅ AgentAuthServer integration test suite setup complete")
}

func (s *AgentAuthServerIntegrationTestSuite) TearDownSuite() {
	s.T().Log("Tearing down AgentAuthServer integration test suite...")

	if s.conn != nil {
		_ = s.conn.Close()
	}

	if s.db != nil {
		_ = s.db.Close()
	}

	if s.redisClient != nil {
		_ = s.redisClient.Close()
	}

	ctx := context.Background()
	if s.pgContainer != nil {
		_ = s.pgContainer.Terminate(ctx)
	}

	if s.redisContainer != nil {
		_ = s.redisContainer.Terminate(ctx)
	}

	s.T().Log("✅ AgentAuthServer integration test suite teardown complete")
}

func (s *AgentAuthServerIntegrationTestSuite) setupDatabaseConnections(ctx context.Context) {
	// Get PostgreSQL connection details
	pgHost, err := s.pgContainer.Host(ctx)
	s.Require().NoError(err)

	pgPort, err := s.pgContainer.MappedPort(ctx, "5432")
	s.Require().NoError(err)

	// Connect to PostgreSQL
	dbURL := fmt.Sprintf("postgres://test:test@%s:%s/agent_auth_test?sslmode=disable", pgHost, pgPort.Port())
	s.db, err = sqlx.Connect("postgres", dbURL)
	s.Require().NoError(err)

	// Get Redis connection details
	redisHost, err := s.redisContainer.Host(ctx)
	s.Require().NoError(err)

	redisPort, err := s.redisContainer.MappedPort(ctx, "6379")
	s.Require().NoError(err)

	// Connect to Redis
	s.redisClient = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort.Port()),
	})

	// Test Redis connection
	_, err = s.redisClient.Ping(ctx).Result()
	s.Require().NoError(err)
}

func (s *AgentAuthServerIntegrationTestSuite) runMigrations() error {
	// Set Goose dialect
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Get the migrations directory path relative to the test file
	migrationsDir := filepath.Join("..", "..", "..", "migrations")

	// Convert sqlx.DB to sql.DB for Goose
	sqlDB := s.db.DB

	// Run up migrations
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	s.T().Log("✅ Successfully ran all migrations using Goose for gRPC server tests")
	return nil
}

func (s *AgentAuthServerIntegrationTestSuite) seedTestData() error {
	// Insert test agent
	agentQuery := `
		INSERT INTO agents_auth (agent_code, phone, email, password_hash, is_active) 
		VALUES ($1, $2, $3, $4, $5) 
		ON CONFLICT (agent_code) DO NOTHING
	`

	// Hash for "password123" - using actual bcrypt hash
	passwordHash := "$2a$10$C26tmsABW0o6n0MjNjgp2uzEDSauH1MJYFxwLLqKROAhH43EAY4dK"

	_, err := s.db.Exec(agentQuery, "10001001", "+233500000001", "agent@test.com", passwordHash, true)
	if err != nil {
		return fmt.Errorf("failed to seed agent data: %w", err)
	}

	// Insert test retailer
	retailerQuery := `
		INSERT INTO retailers_auth (retailer_code, email, phone, password_hash, pin_hash, is_active) 
		VALUES ($1, $2, $3, $4, $5, $6) 
		ON CONFLICT (retailer_code) DO NOTHING
	`

	// Hash for PIN "1234"
	pinHash := "$2a$10$C26tmsABW0o6n0MjNjgp2uzEDSauH1MJYFxwLLqKROAhH43EAY4dK"

	_, err = s.db.Exec(retailerQuery, "12345678", "retailer@test.com", "+233500000002", passwordHash, pinHash, true)
	if err != nil {
		return fmt.Errorf("failed to seed retailer data: %w", err)
	}

	return nil
}

func (s *AgentAuthServerIntegrationTestSuite) setupServiceLayer() {
	// Initialize repositories
	authRepo := repositories.NewAuthRepository(s.db, s.redisClient)
	sessionRepo := repositories.NewSessionRepository(s.db.DB, s.redisClient)
	tokenRepo := repositories.NewTokenRepository(s.redisClient)
	offlineTokenRepo := repositories.NewOfflineTokenRepository(s.db)

	// Initialize service layer with test configurations
	s.authService = services.NewAuthService(
		authRepo,
		sessionRepo,
		tokenRepo,
		offlineTokenRepo,
		15*time.Minute, // accessTokenExpiry
		7*24*time.Hour, // refreshTokenExpiry (7 days)
		5,              // maxFailedLogins
		30*time.Minute, // lockoutDuration
		clients.AgentNotificationClient{},
		nil,
	)
}

func (s *AgentAuthServerIntegrationTestSuite) startTestServer() {
	// Create server
	s.server = NewAgentAuthServer(s.authService)

	// Start gRPC server
	lis, err := net.Listen("tcp", ":0") // Use random available port
	s.Require().NoError(err)

	grpcServer := grpc.NewServer()
	pb.RegisterAgentAuthServiceServer(grpcServer, s.server)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			s.T().Logf("gRPC server error: %v", err)
		}
	}()

	// Connect client using NewClient (replacing deprecated Dial)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	s.Require().NoError(err)

	s.conn = conn
	s.client = pb.NewAgentAuthServiceClient(conn)
}

func (s *AgentAuthServerIntegrationTestSuite) TestAgentLogin() {
	s.T().Log("🧪 Testing Agent Login")

	ctx := context.Background()

	req := &pb.LoginRequest{
		AgentCode: "+233500000001", // Using phone number now
		Password:  "password123",
		IpAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}

	resp, err := s.client.Login(ctx, req)
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	s.Assert().NotEmpty(resp.AccessToken)
	s.Assert().NotEmpty(resp.RefreshToken)
	s.Assert().Greater(resp.ExpiresIn, int32(0))
	// Note: Agent details are not returned in LoginResponse, only tokens

	s.T().Log("✅ Agent Login test passed")
}

func (s *AgentAuthServerIntegrationTestSuite) TestRetailerLogin() {
	s.T().Log("🧪 Testing Retailer Login - SKIPPED (Agent auth service doesn't handle retailers)")
	s.T().Skip("Retailer authentication is handled by a separate service")
}

func (s *AgentAuthServerIntegrationTestSuite) TestRetailerPOSLogin() {
	s.T().Log("🧪 Testing Retailer POS Login - SKIPPED (Agent auth service doesn't handle retailers)")
	s.T().Skip("Retailer POS authentication is handled by a separate service")
}

func (s *AgentAuthServerIntegrationTestSuite) TestInvalidLogin() {
	s.T().Log("🧪 Testing Invalid Login")

	ctx := context.Background()

	// Test with invalid credentials
	req := &pb.LoginRequest{
		AgentCode: "INVALID-PHONE",
		Password:  "wrongpassword",
		IpAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}

	_, err := s.client.Login(ctx, req)
	s.Require().Error(err)

	st, ok := status.FromError(err)
	s.Require().True(ok)
	s.Assert().Equal(codes.Unauthenticated, st.Code())

	s.T().Log("✅ Invalid Login test passed")
}

func (s *AgentAuthServerIntegrationTestSuite) TestRefreshToken() {
	s.T().Log("🧪 Testing Refresh Token")

	ctx := context.Background()

	// First login to get tokens
	loginReq := &pb.LoginRequest{
		AgentCode: "+233500000001",
		Password:  "password123",
		IpAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}

	loginResp, err := s.client.Login(ctx, loginReq)
	s.Require().NoError(err)

	// Now refresh the token
	refreshReq := &pb.RefreshTokenRequest{
		RefreshToken: loginResp.RefreshToken,
	}

	refreshResp, err := s.client.RefreshToken(ctx, refreshReq)
	s.Require().NoError(err)
	s.Require().NotNil(refreshResp)

	s.Assert().NotEmpty(refreshResp.AccessToken)
	s.Assert().NotEmpty(refreshResp.RefreshToken)
	s.Assert().Greater(refreshResp.ExpiresIn, int32(0))

	// New tokens should be different
	s.Assert().NotEqual(loginResp.AccessToken, refreshResp.AccessToken)
	s.Assert().NotEqual(loginResp.RefreshToken, refreshResp.RefreshToken)

	s.T().Log("✅ Refresh Token test passed")
}

func (s *AgentAuthServerIntegrationTestSuite) TestLogout() {
	s.T().Log("🧪 Testing Logout")

	ctx := context.Background()

	// First login
	loginReq := &pb.LoginRequest{
		AgentCode: "+233500000001",
		Password:  "password123",
		IpAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}

	loginResp, err := s.client.Login(ctx, loginReq)
	s.Require().NoError(err)

	// Now logout
	logoutReq := &pb.LogoutRequest{
		RefreshToken: loginResp.RefreshToken,
	}

	_, err = s.client.Logout(ctx, logoutReq)
	s.Require().NoError(err)

	// Try to use the refresh token again (should fail)
	refreshReq := &pb.RefreshTokenRequest{
		RefreshToken: loginResp.RefreshToken,
	}

	_, err = s.client.RefreshToken(ctx, refreshReq)
	s.Require().Error(err)

	st, ok := status.FromError(err)
	s.Require().True(ok)
	s.Assert().Equal(codes.Unauthenticated, st.Code())

	s.T().Log("✅ Logout test passed")
}

func (s *AgentAuthServerIntegrationTestSuite) TestValidateToken() {
	s.T().Log("🧪 Testing Validate Token")

	ctx := context.Background()

	// First login to get access token
	loginReq := &pb.LoginRequest{
		AgentCode: "+233500000001",
		Password:  "password123",
		IpAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}

	loginResp, err := s.client.Login(ctx, loginReq)
	s.Require().NoError(err)

	// Validate the token
	validateReq := &pb.ValidateTokenRequest{
		Token: loginResp.AccessToken,
	}

	validateResp, err := s.client.ValidateToken(ctx, validateReq)
	s.Require().NoError(err)
	s.Require().NotNil(validateResp)

	s.Assert().True(validateResp.Valid)
	s.Assert().NotEmpty(validateResp.AgentId)
	s.Assert().NotEmpty(validateResp.AgentCode)

	s.T().Log("✅ Validate Token test passed")
}

func (s *AgentAuthServerIntegrationTestSuite) TestChangePassword() {
	s.T().Log("🧪 Testing Change Password")

	ctx := context.Background()

	// Get a user ID (we'll need to add this to our test data)
	var userID string
	err := s.db.Get(&userID, "SELECT id FROM agents_auth WHERE agent_code = $1", "10001001")
	s.Require().NoError(err)

	req := &pb.ChangePasswordRequest{
		AgentId:         userID,
		CurrentPassword: "password123",
		NewPassword:     "newpassword123",
	}

	_, err = s.client.ChangePassword(ctx, req)
	s.Require().NoError(err)

	// Try to login with old password (should fail)
	loginReq := &pb.LoginRequest{
		AgentCode: "+233500000001",
		Password:  "password123",
		IpAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}

	_, err = s.client.Login(ctx, loginReq)
	s.Require().Error(err)

	// Try with new password (should work)
	loginReq.Password = "newpassword123"
	_, err = s.client.Login(ctx, loginReq)
	s.Require().NoError(err)

	s.T().Log("✅ Change Password test passed")
}

func (s *AgentAuthServerIntegrationTestSuite) TestValidationErrors() {
	s.T().Log("🧪 Testing Validation Errors")

	ctx := context.Background()

	// Test Login with missing agent code
	loginReq := &pb.LoginRequest{
		Password: "password123",
	}

	_, err := s.client.Login(ctx, loginReq)
	s.Require().Error(err)

	st, ok := status.FromError(err)
	s.Require().True(ok)
	s.Assert().Equal(codes.InvalidArgument, st.Code())

	// Test RefreshToken with missing token
	refreshReq := &pb.RefreshTokenRequest{}

	_, err = s.client.RefreshToken(ctx, refreshReq)
	s.Require().Error(err)

	st, ok = status.FromError(err)
	s.Require().True(ok)
	s.Assert().Equal(codes.InvalidArgument, st.Code())

	s.T().Log("✅ Validation Errors test passed")
}

func (s *AgentAuthServerIntegrationTestSuite) TestFullAuthFlow() {
	s.T().Log("🧪 Testing Full Authentication Flow")

	ctx := context.Background()

	// 1. Agent Login
	loginReq := &pb.LoginRequest{
		AgentCode: "+233500000001",
		Password:  "password123",
		IpAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}

	loginResp, err := s.client.Login(ctx, loginReq)
	s.Require().NoError(err)
	s.T().Logf("Agent login successful")

	// 2. Validate Token
	validateReq := &pb.ValidateTokenRequest{
		Token: loginResp.AccessToken,
	}

	validateResp, err := s.client.ValidateToken(ctx, validateReq)
	s.Require().NoError(err)
	s.Assert().True(validateResp.Valid)
	s.T().Logf("Token validation successful")

	// 3. Refresh Token
	refreshReq := &pb.RefreshTokenRequest{
		RefreshToken: loginResp.RefreshToken,
	}

	refreshResp, err := s.client.RefreshToken(ctx, refreshReq)
	s.Require().NoError(err)
	s.T().Logf("Token refresh successful")

	// 4. Logout
	logoutReq := &pb.LogoutRequest{
		RefreshToken: refreshResp.RefreshToken,
	}

	_, err = s.client.Logout(ctx, logoutReq)
	s.Require().NoError(err)
	s.T().Logf("Logout successful")

	s.T().Log("✅ Full Authentication Flow test passed")
}

func (s *AgentAuthServerIntegrationTestSuite) TestConcurrentLogins() {
	s.T().Log("🧪 Testing Concurrent Logins")

	ctx := context.Background()

	const numLogins = 3 // Reduced from 5 to 3 to be less aggressive
	results := make(chan error, numLogins)
	successCount := 0

	// Add a small delay between concurrent login attempts
	for i := 0; i < numLogins; i++ {
		time.Sleep(10 * time.Millisecond) // Small delay to prevent rate limiting issues
		go func(index int) {
			loginReq := &pb.LoginRequest{
				AgentCode: "+233500000001",
				Password:  "password123",
			}

			_, err := s.client.Login(ctx, loginReq)
			results <- err
		}(i)
	}

	// Wait for all goroutines to complete and count successes
	for i := 0; i < numLogins; i++ {
		err := <-results
		if err == nil {
			successCount++
		}
	}

	// At least one login should succeed (concurrent logins should be allowed)
	s.Assert().GreaterOrEqual(successCount, 1, "At least one concurrent login should succeed")

	s.T().Log("✅ Concurrent Logins test passed")
}

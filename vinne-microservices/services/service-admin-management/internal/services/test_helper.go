package services

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	adminmanagementv1 "github.com/randco/randco-microservices/proto/admin/management/v1"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/repositories"
	commonModels "github.com/randco/randco-microservices/shared/common/models"
	"github.com/randco/randco-microservices/shared/events"
	"github.com/randco/randco-microservices/shared/middleware/auth"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/types/known/emptypb"
)

// BaseServiceTestSuite provides common test setup for service tests
type BaseServiceTestSuite struct {
	suite.Suite
	db         *sql.DB
	container  testcontainers.Container
	ctx        context.Context
	repos      *repositories.Repositories
	jwtManager *auth.JWTManager
	eventBus   events.EventBus
	testHelper *repositories.TestHelper

	// Common test data
	testAdminUser *models.AdminUser
	testRoles     []*models.Role
	testPerms     []*models.Permission
}

func (s *BaseServiceTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.testHelper = repositories.NewTestHelper()

	// Start PostgreSQL testcontainer
	postgresContainer, err := postgres.Run(s.ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("admin_service_test"),
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

	// Run migrations
	err = s.testHelper.SetupTestDB(s.T(), s.db)
	require.NoError(s.T(), err)

	// Initialize repositories
	s.repos = repositories.NewRepositories(s.db)

	// Initialize JWT manager with test config
	jwtConfig := auth.JWTConfig{
		Secret:             "test-secret-key-for-testing-only",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 24 * time.Hour,
		Issuer:             "test-issuer",
	}
	s.jwtManager = auth.NewJWTManager(jwtConfig)

	// Initialize mock event bus
	s.eventBus = &mockEventBus{}
}

func (s *BaseServiceTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		s.NoError(err)
	}
}

func (s *BaseServiceTestSuite) SetupTest() {
	// Clean up test data while preserving structure
	err := s.testHelper.CleanupTestData(s.T(), s.db)
	require.NoError(s.T(), err)

	// Clean up sessions explicitly
	_, err = s.db.Exec("DELETE FROM admin_sessions")
	require.NoError(s.T(), err)

	// Seed default data
	err = s.testHelper.SeedDefaultData(s.T(), s.db)
	require.NoError(s.T(), err)

	// Create test data
	s.createTestData()
}

func (s *BaseServiceTestSuite) createTestData() {
	// Create test permissions
	s.testPerms = []*models.Permission{
		{
			ID:       uuid.MustParse("b1eebc99-9c0b-4ef8-bb6d-6bb9bd380a01"),
			Resource: "users",
			Action:   "create",
		},
		{
			ID:       uuid.MustParse("b2eebc99-9c0b-4ef8-bb6d-6bb9bd380a02"),
			Resource: "users",
			Action:   "read",
		},
		{
			ID:       uuid.MustParse("b3eebc99-9c0b-4ef8-bb6d-6bb9bd380a03"),
			Resource: "users",
			Action:   "update",
		},
	}

	// Insert test permissions
	for _, perm := range s.testPerms {
		desc := "Test permission"
		_, err := s.db.Exec(`
			INSERT INTO permissions (id, resource, action, description)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (resource, action) DO NOTHING
		`, perm.ID, perm.Resource, perm.Action, desc)
		require.NoError(s.T(), err)
	}

	// Create test roles (using existing seeded roles)
	s.testRoles = []*models.Role{
		{
			ID:   uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
			Name: "super_admin",
		},
		{
			ID:   uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12"),
			Name: "admin",
		},
	}

	// Assign permissions to roles
	for _, role := range s.testRoles {
		for _, perm := range s.testPerms {
			_, err := s.db.Exec(`
				INSERT INTO role_permissions (role_id, permission_id)
				VALUES ($1, $2)
				ON CONFLICT DO NOTHING
			`, role.ID, perm.ID)
			require.NoError(s.T(), err)
		}
	}

	// Create test admin user
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("TestPassword123!"), bcrypt.DefaultCost)
	require.NoError(s.T(), err)

	s.testAdminUser = &models.AdminUser{
		BaseModel: commonModels.BaseModel{
			ID:        uuid.MustParse("c1eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Version:   1,
		},
		Email:        "test.admin@example.com",
		Username:     "testadmin",
		PasswordHash: string(hashedPassword),
		FirstName:    stringPtr("Test"),
		LastName:     stringPtr("Admin"),
		IsActive:     true,
		MFAEnabled:   false,
	}

	// Insert test user
	_, err = s.db.Exec(`
		INSERT INTO admin_users (id, email, username, password_hash, first_name, last_name, 
			is_active, mfa_enabled, created_at, updated_at, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, s.testAdminUser.ID, s.testAdminUser.Email, s.testAdminUser.Username,
		s.testAdminUser.PasswordHash, s.testAdminUser.FirstName, s.testAdminUser.LastName,
		s.testAdminUser.IsActive, s.testAdminUser.MFAEnabled,
		s.testAdminUser.CreatedAt, s.testAdminUser.UpdatedAt, s.testAdminUser.Version)
	require.NoError(s.T(), err)

	// Assign roles to test user
	for _, role := range s.testRoles {
		_, err = s.db.Exec(`
			INSERT INTO admin_user_roles (user_id, role_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, s.testAdminUser.ID, role.ID)
		require.NoError(s.T(), err)
	}

	// Load complete user with roles
	s.testAdminUser.Roles = s.testRoles
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

// mockEventBus implements a mock event bus for testing
type mockEventBus struct {
	publishedEvents []events.Event
}

func (m *mockEventBus) Publish(ctx context.Context, topic string, event events.Event) error {
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}

func (m *mockEventBus) Subscribe(ctx context.Context, topic string, handler events.EventHandler) error {
	return nil
}

func (m *mockEventBus) Close() error {
	return nil
}

func (m *mockEventBus) GetPublishedEvents() []events.Event {
	return m.publishedEvents
}

// mockAuthService implements a mock AuthService for testing
type mockAuthService struct {
	loginFunc          func(ctx context.Context, req *adminmanagementv1.LoginRequest) (*adminmanagementv1.LoginResponse, error)
	logoutFunc         func(ctx context.Context, req *adminmanagementv1.LogoutRequest) (*emptypb.Empty, error)
	refreshTokenFunc   func(ctx context.Context, req *adminmanagementv1.RefreshTokenRequest) (*adminmanagementv1.RefreshTokenResponse, error)
	changePasswordFunc func(ctx context.Context, req *adminmanagementv1.ChangePasswordRequest) (*emptypb.Empty, error)
	enableMFAFunc      func(ctx context.Context, req *adminmanagementv1.EnableMFARequest) (*adminmanagementv1.EnableMFAResponse, error)
	verifyMFAFunc      func(ctx context.Context, req *adminmanagementv1.VerifyMFARequest) (*emptypb.Empty, error)
	disableMFAFunc     func(ctx context.Context, req *adminmanagementv1.DisableMFARequest) (*emptypb.Empty, error)
}

func (m *mockAuthService) Login(ctx context.Context, req *adminmanagementv1.LoginRequest) (*adminmanagementv1.LoginResponse, error) {
	if m.loginFunc != nil {
		return m.loginFunc(ctx, req)
	}
	// Default implementation
	firstName := "Test"
	lastName := "User"
	return &adminmanagementv1.LoginResponse{
		AccessToken:  "mock-access-token",
		RefreshToken: "mock-refresh-token",
		ExpiresIn:    900,
		User: &adminmanagementv1.AdminUser{
			Id:        uuid.New().String(),
			Email:     req.Email,
			Username:  "testuser",
			FirstName: &firstName,
			LastName:  &lastName,
			IsActive:  true,
		},
	}, nil
}

func (m *mockAuthService) Logout(ctx context.Context, req *adminmanagementv1.LogoutRequest) (*emptypb.Empty, error) {
	if m.logoutFunc != nil {
		return m.logoutFunc(ctx, req)
	}
	return &emptypb.Empty{}, nil
}

func (m *mockAuthService) RefreshToken(ctx context.Context, req *adminmanagementv1.RefreshTokenRequest) (*adminmanagementv1.RefreshTokenResponse, error) {
	if m.refreshTokenFunc != nil {
		return m.refreshTokenFunc(ctx, req)
	}
	return &adminmanagementv1.RefreshTokenResponse{
		AccessToken:  "new-mock-access-token",
		RefreshToken: "new-mock-refresh-token",
		ExpiresIn:    900,
	}, nil
}

func (m *mockAuthService) ChangePassword(ctx context.Context, req *adminmanagementv1.ChangePasswordRequest) (*emptypb.Empty, error) {
	if m.changePasswordFunc != nil {
		return m.changePasswordFunc(ctx, req)
	}
	return &emptypb.Empty{}, nil
}

func (m *mockAuthService) EnableMFA(ctx context.Context, req *adminmanagementv1.EnableMFARequest) (*adminmanagementv1.EnableMFAResponse, error) {
	if m.enableMFAFunc != nil {
		return m.enableMFAFunc(ctx, req)
	}
	return &adminmanagementv1.EnableMFAResponse{
		Secret:      "mock-mfa-secret",
		QrCodeUrl:   "mock-qr-code-url",
		BackupCodes: []string{"code1", "code2", "code3"},
	}, nil
}

func (m *mockAuthService) VerifyMFA(ctx context.Context, req *adminmanagementv1.VerifyMFARequest) (*emptypb.Empty, error) {
	if m.verifyMFAFunc != nil {
		return m.verifyMFAFunc(ctx, req)
	}
	return &emptypb.Empty{}, nil
}

func (m *mockAuthService) DisableMFA(ctx context.Context, req *adminmanagementv1.DisableMFARequest) (*emptypb.Empty, error) {
	if m.disableMFAFunc != nil {
		return m.disableMFAFunc(ctx, req)
	}
	return &emptypb.Empty{}, nil
}

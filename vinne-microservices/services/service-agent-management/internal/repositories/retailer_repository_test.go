package repositories

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/lib/pq"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
)

type RetailerRepositoryTestSuite struct {
	suite.Suite
	db        *sqlx.DB
	container *postgres.PostgresContainer
	repo      RetailerRepository
}

func TestRetailerRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(RetailerRepositoryTestSuite))
}

func (s *RetailerRepositoryTestSuite) SetupSuite() {
	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("retailer_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(s.T(), err)
	s.container = postgresContainer

	// Get connection details
	dbHost, err := postgresContainer.Host(ctx)
	require.NoError(s.T(), err)
	dbPort, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(s.T(), err)

	// Connect directly using sqlx for testing
	dsn := "host=" + dbHost + " port=" + dbPort.Port() + " user=testuser password=testpass dbname=retailer_test sslmode=disable"
	db, err := sqlx.Connect("postgres", dsn)
	require.NoError(s.T(), err)
	s.db = db

	// Run migrations using Goose
	s.runMigrations()

	// Initialize repository
	s.repo = NewRetailerRepository(s.db)
}

func (s *RetailerRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(context.Background())
	}
}

func (s *RetailerRepositoryTestSuite) SetupTest() {
	// Clean up data before each test
	s.cleanupRetailerData()
}

func (s *RetailerRepositoryTestSuite) runMigrations() {
	// Set Goose dialect
	if err := goose.SetDialect("postgres"); err != nil {
		s.T().Fatalf("Failed to set goose dialect: %v", err)
	}

	// Get the migrations directory path relative to the test file
	migrationsDir := filepath.Join("..", "..", "migrations")

	// Convert sqlx.DB to sql.DB for Goose
	sqlDB := s.db.DB

	// Run up migrations
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		s.T().Fatalf("Failed to run migrations: %v", err)
	}

	s.T().Log("✅ Successfully ran all migrations using Goose")
}

func (s *RetailerRepositoryTestSuite) cleanupRetailerData() {
	cleanupQueries := []string{
		"DELETE FROM retailer_kyc",
		"DELETE FROM agent_kyc",
		"DELETE FROM pos_devices",
		"DELETE FROM agent_retailers",
		"DELETE FROM retailers",
		"DELETE FROM agents",
	}

	for _, query := range cleanupQueries {
		_, err := s.db.Exec(query)
		// Ignore errors for tables that might not exist yet
		if err != nil {
			s.T().Logf("Warning: Failed to clean up table (may not exist): %v", err)
		}
	}
}

// Test GetNextRetailerCode for independent retailers
func (s *RetailerRepositoryTestSuite) TestGetNextRetailerCodeIndependent() {
	ctx := context.Background()

	// Test 1: First independent retailer code should be 00000001
	nextCode, err := s.repo.GetNextRetailerCode(ctx, "")
	assert.NoError(s.T(), err, "GetNextRetailerCode should not return error")
	assert.Equal(s.T(), "00000001", nextCode, "First independent retailer code should be 00000001")
	s.T().Logf("✅ First independent retailer code generated: %s", nextCode)

	// Create a retailer with this code
	retailer := &models.Retailer{
		ID:               uuid.New(),
		RetailerCode:     nextCode,
		Name:             "Independent Retailer 1",
		OwnerName:        "Owner 1",
		Email:            "retailer1@test.com",
		PhoneNumber:      "1234567890",
		Address:          "123 Main St",
		City:             "Accra",
		Region:           "Greater Accra",
		Status:           models.StatusActive,
		OnboardingMethod: models.OnboardingRandLotteryLtdDirect,
		CreatedBy:        "test",
		UpdatedBy:        "test",
	}
	err = s.repo.Create(ctx, retailer)
	assert.NoError(s.T(), err)

	// Test 2: Next independent retailer code should be 00000002
	nextCode, err = s.repo.GetNextRetailerCode(ctx, "")
	assert.NoError(s.T(), err, "GetNextRetailerCode should not return error")
	assert.Equal(s.T(), "00000002", nextCode, "Second independent retailer code should be 00000002")
	s.T().Logf("✅ Second independent retailer code generated: %s", nextCode)
}

// Test GetNextRetailerCode for agent-managed retailers
func (s *RetailerRepositoryTestSuite) TestGetNextRetailerCodeAgentManaged() {
	ctx := context.Background()

	// First create an agent to use
	agent := &models.Agent{
		ID:                   uuid.New(),
		AgentCode:            "1001",
		BusinessName:         "Test Agent",
		ContactEmail:         "agent@test.com",
		ContactPhone:         "1234567890",
		Status:               models.StatusActive,
		OnboardingMethod:     models.OnboardingRandLotteryLtdDirect,
		CommissionPercentage: 30.0,
		CreatedBy:            "test",
		UpdatedBy:            "test",
	}

	// Insert the agent directly
	_, err := s.db.Exec(`
		INSERT INTO agents (id, agent_code, business_name, contact_email, contact_phone, 
			status, onboarding_method, commission_percentage, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		agent.ID, agent.AgentCode, agent.BusinessName, agent.ContactEmail, agent.ContactPhone,
		agent.Status, agent.OnboardingMethod, agent.CommissionPercentage, agent.CreatedBy, agent.UpdatedBy)
	require.NoError(s.T(), err)

	// Test 1: First retailer for agent 1001 should be 10010001
	nextCode, err := s.repo.GetNextRetailerCode(ctx, "1001")
	assert.NoError(s.T(), err, "GetNextRetailerCode should not return error")
	assert.Equal(s.T(), "10010001", nextCode, "First retailer for agent 1001 should be 10010001")
	s.T().Logf("✅ First agent-managed retailer code generated: %s", nextCode)

	// Create a retailer with this code
	retailer := &models.Retailer{
		ID:               uuid.New(),
		RetailerCode:     nextCode,
		Name:             "Agent Managed Retailer 1",
		OwnerName:        "Owner 1",
		Email:            "agentretailer1@test.com",
		PhoneNumber:      "9876543210",
		Address:          "456 Market St",
		City:             "Kumasi",
		Region:           "Ashanti",
		Status:           models.StatusActive,
		OnboardingMethod: models.OnboardingAgentManaged,
		AgentID:          &agent.ID,
		CreatedBy:        "test",
		UpdatedBy:        "test",
	}
	err = s.repo.Create(ctx, retailer)
	assert.NoError(s.T(), err)

	// Test 2: Next retailer for agent 1001 should be 10010002
	nextCode, err = s.repo.GetNextRetailerCode(ctx, "1001")
	assert.NoError(s.T(), err, "GetNextRetailerCode should not return error")
	assert.Equal(s.T(), "10010002", nextCode, "Second retailer for agent 1001 should be 10010002")
	s.T().Logf("✅ Second agent-managed retailer code generated: %s", nextCode)

	// Test 3: Test with a different agent code
	nextCode, err = s.repo.GetNextRetailerCode(ctx, "2002")
	assert.NoError(s.T(), err, "GetNextRetailerCode should not return error")
	assert.Equal(s.T(), "20020001", nextCode, "First retailer for agent 2002 should be 20020001")
	s.T().Logf("✅ First retailer for different agent code generated: %s", nextCode)
}

// Test reaching the sequence limit for retailer codes
func (s *RetailerRepositoryTestSuite) TestRetailerCodeSequenceLimit() {
	ctx := context.Background()

	// Create a retailer with code near the limit
	retailer := &models.Retailer{
		ID:               uuid.New(),
		RetailerCode:     "10019999", // Last valid code for agent 1001
		Name:             "Near Limit Retailer",
		OwnerName:        "Owner",
		Email:            "nearlimit@test.com",
		PhoneNumber:      "5555555555",
		Address:          "999 Limit St",
		City:             "Accra",
		Region:           "Greater Accra",
		Status:           models.StatusActive,
		OnboardingMethod: models.OnboardingAgentManaged,
		CreatedBy:        "test",
		UpdatedBy:        "test",
	}
	err := s.repo.Create(ctx, retailer)
	assert.NoError(s.T(), err)

	// Test: Next code should fail as we've reached the limit
	nextCode, err := s.repo.GetNextRetailerCode(ctx, "1001")
	assert.Error(s.T(), err, "Should return error when sequence limit is reached")
	assert.Contains(s.T(), err.Error(), "sequence limit reached", "Error should mention sequence limit")
	assert.Empty(s.T(), nextCode, "No code should be returned when limit is reached")
	s.T().Log("✅ Correctly handles sequence limit for retailer codes")
}

// Test repository interface compliance
func (s *RetailerRepositoryTestSuite) TestRepositoryInterfaceCompliance() {
	// Verify the repository implements the RetailerRepository interface
	_ = RetailerRepository(s.repo)

	s.T().Log("✅ Retailer repository implements RetailerRepository interface")
	s.T().Log("✅ Repository can be tested with real database operations")
}

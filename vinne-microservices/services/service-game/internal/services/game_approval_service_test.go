package services

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"github.com/randco/randco-microservices/services/service-game/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type GameApprovalServiceTestSuite struct {
	suite.Suite
	db              *sql.DB
	approvalService GameApprovalService
	gameService     GameService
	rulesService    GameRulesService
	prizeService    PrizeStructureService
	approvalRepo    repositories.GameApprovalRepository
	gameRepo        repositories.GameRepository
	rulesRepo       repositories.GameRulesRepository
	prizeRepo       repositories.PrizeStructureRepository
	container       testcontainers.Container
	ctx             context.Context
	testGameID      uuid.UUID
	testApproverID  uuid.UUID
	testRejectorID  uuid.UUID
}

func TestGameApprovalServiceTestSuite(t *testing.T) {
	suite.Run(t, new(GameApprovalServiceTestSuite))
}

func (s *GameApprovalServiceTestSuite) SetupSuite() {
	s.ctx = context.Background()

	// Start PostgreSQL testcontainer
	postgresContainer, err := postgres.Run(s.ctx, "postgres:17-alpine",
		postgres.WithDatabase("service_game_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(s.T(), err)
	s.container = postgresContainer

	// Get database connection string
	connStr, err := postgresContainer.ConnectionString(s.ctx, "sslmode=disable")
	require.NoError(s.T(), err)

	// Connect to database
	s.db, err = sql.Open("postgres", connStr)
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.db.Ping())

	// Run migrations using Goose to ensure consistency with real schema
	s.runMigrations()

	// Initialize repositories
	s.approvalRepo = repositories.NewGameApprovalRepository(s.db)
	s.gameRepo = repositories.NewGameRepository(s.db)
	s.rulesRepo = repositories.NewGameRulesRepository(s.db)
	s.prizeRepo = repositories.NewPrizeStructureRepository(s.db)

	// Initialize services
	serviceConfig := &ServiceConfig{
		KafkaBrokers: []string{}, // Empty for tests, will use in-memory event bus
	}
	s.gameService = NewGameService(s.db, nil, s.gameRepo, nil, serviceConfig) // No Redis cache or storage for tests
	s.rulesService = NewGameRulesService(s.rulesRepo, s.gameRepo)
	s.prizeService = NewPrizeStructureService(s.prizeRepo, s.gameRepo)
	s.approvalService = NewGameApprovalService(
		s.approvalRepo,
		s.gameRepo,
		s.rulesRepo,
		s.prizeRepo,
	)

	// Generate test UUIDs
	s.testGameID = uuid.New()
	s.testApproverID = uuid.New()
	s.testRejectorID = uuid.New()
}

func (s *GameApprovalServiceTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(s.ctx)
	}
}

func (s *GameApprovalServiceTestSuite) SetupTest() {
	// Clean up test data before each test
	s.cleanupTestData()
	// Create a test game for approval tests
	s.createTestGame()
}

func (s *GameApprovalServiceTestSuite) TestSubmitForApproval() {
	// Test successful submission
	notes := "Game ready for approval"
	err := s.approvalService.SubmitForApproval(s.ctx, s.testGameID, s.testApproverID, notes)
	assert.NoError(s.T(), err)

	// Verify approval record was created
	approval, err := s.approvalService.GetApprovalStatus(s.ctx, s.testGameID)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), approval)
	assert.Equal(s.T(), models.ApprovalStageSubmitted, approval.ApprovalStage)
	assert.Equal(s.T(), notes, *approval.Notes)
}

func (s *GameApprovalServiceTestSuite) TestApproveGame_FirstApproval() {
	// First, submit for approval
	err := s.approvalService.SubmitForApproval(s.ctx, s.testGameID, s.testApproverID, "Ready for review")
	require.NoError(s.T(), err)

	// Test first approval
	notes := "First approval granted"
	err = s.approvalService.ApproveGame(s.ctx, s.testGameID, s.testApproverID, notes)
	assert.NoError(s.T(), err)

	// Verify approval status
	approval, err := s.approvalService.GetApprovalStatus(s.ctx, s.testGameID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), models.ApprovalStage("FIRST_APPROVED"), approval.ApprovalStage)
	assert.Equal(s.T(), int(1), approval.ApprovalCount)
}

func (s *GameApprovalServiceTestSuite) TestApproveGame_SecondApproval() {
	// Submit for approval
	err := s.approvalService.SubmitForApproval(s.ctx, s.testGameID, s.testApproverID, "Ready for review")
	require.NoError(s.T(), err)

	// First approval
	err = s.approvalService.ApproveGame(s.ctx, s.testGameID, s.testApproverID, "First approval")
	require.NoError(s.T(), err)

	// Second approval by different user
	secondApproverID := uuid.New()
	notes := "Second approval granted"
	err = s.approvalService.ApproveGame(s.ctx, s.testGameID, secondApproverID, notes)
	assert.NoError(s.T(), err)

	// Verify approval status
	approval, err := s.approvalService.GetApprovalStatus(s.ctx, s.testGameID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), models.ApprovalStageApproved, approval.ApprovalStage)
	assert.Equal(s.T(), int(2), approval.ApprovalCount)
}

func (s *GameApprovalServiceTestSuite) TestRejectGame() {
	// Submit for approval
	err := s.approvalService.SubmitForApproval(s.ctx, s.testGameID, s.testApproverID, "Ready for review")
	require.NoError(s.T(), err)

	// Test rejection
	reason := "Game does not meet requirements"
	err = s.approvalService.RejectGame(s.ctx, s.testGameID, s.testRejectorID, reason)
	assert.NoError(s.T(), err)

	// Verify rejection status
	approval, err := s.approvalService.GetApprovalStatus(s.ctx, s.testGameID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), models.ApprovalStageRejected, approval.ApprovalStage)
	assert.Equal(s.T(), reason, *approval.Reason)
}

func (s *GameApprovalServiceTestSuite) TestGetPendingApprovals() {
	// Create multiple games with different approval states
	gameID1 := uuid.New()
	gameID2 := uuid.New()

	// Create test games
	actualGameID1 := s.createTestGameWithID(gameID1)
	actualGameID2 := s.createTestGameWithID(gameID2)

	// Submit both for approval
	err := s.approvalService.SubmitForApproval(s.ctx, actualGameID1, s.testApproverID, "Game 1 for approval")
	require.NoError(s.T(), err)
	err = s.approvalService.SubmitForApproval(s.ctx, actualGameID2, s.testApproverID, "Game 2 for approval")
	require.NoError(s.T(), err)

	// Get pending approvals
	approvals, err := s.approvalService.GetPendingApprovals(s.ctx)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), approvals, 2)

	// Verify all are in submitted state
	for _, approval := range approvals {
		assert.Equal(s.T(), models.ApprovalStageSubmitted, approval.ApprovalStage)
	}
}

func (s *GameApprovalServiceTestSuite) TestGetPendingFirstApprovals() {
	// Submit a game for approval
	err := s.approvalService.SubmitForApproval(s.ctx, s.testGameID, s.testApproverID, "Ready for first approval")
	require.NoError(s.T(), err)

	// Get pending first approvals
	approvals, err := s.approvalService.GetPendingFirstApprovals(s.ctx)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), approvals, 1)
	assert.Equal(s.T(), models.ApprovalStageSubmitted, approvals[0].ApprovalStage)
}

func (s *GameApprovalServiceTestSuite) TestGetPendingSecondApprovals() {
	// Submit and first approve a game
	err := s.approvalService.SubmitForApproval(s.ctx, s.testGameID, s.testApproverID, "Ready for review")
	require.NoError(s.T(), err)
	err = s.approvalService.ApproveGame(s.ctx, s.testGameID, s.testApproverID, "First approval")
	require.NoError(s.T(), err)

	// Get pending second approvals
	approvals, err := s.approvalService.GetPendingSecondApprovals(s.ctx)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), approvals, 1)
	assert.Equal(s.T(), models.ApprovalStage("FIRST_APPROVED"), approvals[0].ApprovalStage)
}

// Helper methods

func (s *GameApprovalServiceTestSuite) runMigrations() {
	// Set Goose dialect
	if err := goose.SetDialect("postgres"); err != nil {
		s.T().Fatalf("Failed to set goose dialect: %v", err)
	}

	// Get the migrations directory path relative to the test file
	migrationsDir := filepath.Join("..", "..", "migrations")

	// Run up migrations
	if err := goose.Up(s.db, migrationsDir); err != nil {
		s.T().Fatalf("Failed to run migrations: %v", err)
	}
}

func (s *GameApprovalServiceTestSuite) cleanupTestData() {
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM game_approvals")
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM prize_structures")
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM game_rules")
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM games")
}

func (s *GameApprovalServiceTestSuite) createTestGame() {
	s.testGameID = s.createTestGameWithID(s.testGameID)
}

func (s *GameApprovalServiceTestSuite) createTestGameWithID(gameID uuid.UUID) uuid.UUID {
	// Create game using the service layer
	gameReq := models.CreateGameRequest{
		Code:                "TEST-" + gameID.String()[:8],
		Name:                "Test Game " + gameID.String()[:8], // Make name unique
		Organizer:           "NLA",
		GameCategory:        "lottery",
		GameFormat:          "draw",
		NumberRangeMin:      1,
		NumberRangeMax:      90,
		SelectionCount:      5,
		DrawFrequency:       "weekly",
		DrawDays:            []string{"monday", "wednesday", "friday"}, // Added draw days for weekly frequency
		SalesCutoffMinutes:  30,
		MinStake:            1.0,
		MaxStake:            100.0,
		BasePrice:           1.0,
		MaxTicketsPerPlayer: 10,
		MultiDrawEnabled:    false,
		Description:         stringPtr("Test game for approval testing"),
	}

	game, err := s.gameService.CreateGame(s.ctx, gameReq)
	require.NoError(s.T(), err)
	actualGameID := game.ID

	// Create basic game rules for approval validation
	rulesReq := models.CreateGameRulesRequest{
		GameID:         actualGameID,
		NumbersToPick:  5,
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  10,
		AllowQuickPick: true,
	}

	_, err = s.rulesService.CreateGameRules(s.ctx, rulesReq)
	require.NoError(s.T(), err)

	// Create basic prize structure for approval validation
	prizeReq := models.CreatePrizeStructureRequest{
		GameID:              actualGameID,
		TotalPrizePool:      100000,
		HouseEdgePercentage: 15.0,
		Tiers: []models.PrizeTier{
			{
				TierNumber:       1,
				Name:             "Tier 1",
				MatchesRequired:  5,
				PrizeAmount:      60000,
				PrizePercentage:  60.0,
				EstimatedWinners: 1,
			},
			{
				TierNumber:       2,
				Name:             "Tier 2",
				MatchesRequired:  4,
				PrizeAmount:      25000,
				PrizePercentage:  25.0,
				EstimatedWinners: 5,
			},
		},
	}

	_, err = s.prizeService.CreatePrizeStructure(s.ctx, prizeReq)
	require.NoError(s.T(), err)

	return actualGameID
}

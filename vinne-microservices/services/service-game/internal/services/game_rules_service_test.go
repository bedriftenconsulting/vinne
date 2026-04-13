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

type GameRulesServiceTestSuite struct {
	suite.Suite
	db           *sql.DB
	rulesService GameRulesService
	rulesRepo    repositories.GameRulesRepository
	gameRepo     repositories.GameRepository
	container    testcontainers.Container
	ctx          context.Context
	testGameID   uuid.UUID
}

func TestGameRulesServiceTestSuite(t *testing.T) {
	suite.Run(t, new(GameRulesServiceTestSuite))
}

func (s *GameRulesServiceTestSuite) SetupSuite() {
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

	// Run Goose migrations
	s.runMigrations()

	// Initialize repositories
	s.rulesRepo = repositories.NewGameRulesRepository(s.db)
	s.gameRepo = repositories.NewGameRepository(s.db)

	// Initialize service
	s.rulesService = NewGameRulesService(s.rulesRepo, s.gameRepo)

	// Generate test UUID
	s.testGameID = uuid.New()
}

func (s *GameRulesServiceTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(s.ctx)
	}
}

func (s *GameRulesServiceTestSuite) SetupTest() {
	// Clean up test data before each test
	s.cleanupTestData()
	// Create a test game for rules tests
	s.createTestGame()
}

func (s *GameRulesServiceTestSuite) TestCreateGameRules() {
	// Test successful creation
	req := models.CreateGameRulesRequest{
		GameID:         s.testGameID,
		NumbersToPick:  5,
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  10,
		AllowQuickPick: true,
	}

	rules, err := s.rulesService.CreateGameRules(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), rules)
	assert.Equal(s.T(), s.testGameID, rules.GameID)
	assert.Equal(s.T(), int32(5), rules.NumbersToPick)
	assert.Equal(s.T(), int32(90), rules.TotalNumbers)
	assert.True(s.T(), rules.AllowQuickPick)
}

func (s *GameRulesServiceTestSuite) TestCreateGameRules_InvalidGame() {
	// Test creation with invalid game ID
	invalidGameID := uuid.New()
	req := models.CreateGameRulesRequest{
		GameID:         invalidGameID,
		NumbersToPick:  5,
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  10,
		AllowQuickPick: true,
	}

	_, err := s.rulesService.CreateGameRules(s.ctx, req)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "game not found")
}

func (s *GameRulesServiceTestSuite) TestGetGameRules() {
	// First create rules
	req := models.CreateGameRulesRequest{
		GameID:         s.testGameID,
		NumbersToPick:  6,
		TotalNumbers:   45,
		MinSelections:  1,
		MaxSelections:  5,
		AllowQuickPick: false,
	}

	created, err := s.rulesService.CreateGameRules(s.ctx, req)
	require.NoError(s.T(), err)

	// Test retrieval
	rules, err := s.rulesService.GetGameRules(s.ctx, s.testGameID)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), rules)
	assert.Equal(s.T(), created.ID, rules.ID)
	assert.Equal(s.T(), s.testGameID, rules.GameID)
	assert.Equal(s.T(), int32(6), rules.NumbersToPick)
	assert.Equal(s.T(), int32(45), rules.TotalNumbers)
	assert.False(s.T(), rules.AllowQuickPick)
}

func (s *GameRulesServiceTestSuite) TestGetGameRules_NotFound() {
	// Test retrieval with non-existent game
	invalidGameID := uuid.New()

	rules, err := s.rulesService.GetGameRules(s.ctx, invalidGameID)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), rules)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *GameRulesServiceTestSuite) TestUpdateGameRules() {
	// First create rules
	req := models.CreateGameRulesRequest{
		GameID:         s.testGameID,
		NumbersToPick:  5,
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  10,
		AllowQuickPick: true,
	}

	created, err := s.rulesService.CreateGameRules(s.ctx, req)
	require.NoError(s.T(), err)

	// Test update
	created.NumbersToPick = 7
	created.TotalNumbers = 49
	created.AllowQuickPick = false

	err = s.rulesService.UpdateGameRules(s.ctx, created)
	assert.NoError(s.T(), err)

	// Verify update
	updated, err := s.rulesService.GetGameRules(s.ctx, s.testGameID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), int32(7), updated.NumbersToPick)
	assert.Equal(s.T(), int32(49), updated.TotalNumbers)
	assert.False(s.T(), updated.AllowQuickPick)
}

func (s *GameRulesServiceTestSuite) TestValidateGameRules() {
	// Test valid rules
	validRules := &models.GameRules{
		ID:             uuid.New(),
		GameID:         s.testGameID,
		NumbersToPick:  5,
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  10,
		AllowQuickPick: true,
		EffectiveFrom:  time.Now(),
	}

	err := s.rulesService.ValidateGameRules(s.ctx, validRules)
	assert.NoError(s.T(), err)
}

func (s *GameRulesServiceTestSuite) TestValidateGameRules_InvalidNumbersToPick() {
	// Test invalid numbers to pick (more than total numbers)
	invalidRules := &models.GameRules{
		ID:             uuid.New(),
		GameID:         s.testGameID,
		NumbersToPick:  95, // More than total numbers
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  10,
		AllowQuickPick: true,
		EffectiveFrom:  time.Now(),
	}

	err := s.rulesService.ValidateGameRules(s.ctx, invalidRules)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "cannot exceed total numbers")
}

func (s *GameRulesServiceTestSuite) TestValidateGameRules_InvalidSelections() {
	// Test invalid min/max selections
	invalidRules := &models.GameRules{
		ID:             uuid.New(),
		GameID:         s.testGameID,
		NumbersToPick:  5,
		TotalNumbers:   90,
		MinSelections:  10, // Min greater than max
		MaxSelections:  5,
		AllowQuickPick: true,
		EffectiveFrom:  time.Now(),
	}

	err := s.rulesService.ValidateGameRules(s.ctx, invalidRules)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "cannot be less than minimum selections")
}

// Helper methods

func (s *GameRulesServiceTestSuite) runMigrations() {
	// Set Goose dialect
	err := goose.SetDialect("postgres")
	require.NoError(s.T(), err)

	// Get migrations directory path
	migrationsDir := filepath.Join("..", "..", "migrations")

	// Run migrations
	err = goose.Up(s.db, migrationsDir)
	require.NoError(s.T(), err)
}

func (s *GameRulesServiceTestSuite) cleanupTestData() {
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM game_rules")
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM games")
}

func (s *GameRulesServiceTestSuite) createTestGame() {
	drawDaysJSON := `["WEDNESDAY", "SATURDAY"]`
	query := `INSERT INTO games (
		id, code, name, type, game_type, game_format, game_category, organizer,
		min_stake_amount, max_stake_amount, max_tickets_per_player, draw_frequency,
		draw_days, number_range_min, number_range_max, selection_count, 
		sales_cutoff_minutes, base_price, multi_draw_enabled, version, status
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)`
	_, err := s.db.ExecContext(s.ctx, query,
		s.testGameID,
		"TEST001", // code
		"Test Game for Rules",
		"5_90",       // type
		"5_90",       // game_type
		"5_by_90",    // game_format
		"NUMBERS",    // game_category
		"NLA",        // organizer
		1.0,          // min_stake_amount
		100.0,        // max_stake_amount
		10,           // max_tickets_per_player
		"weekly",     // draw_frequency
		drawDaysJSON, // draw_days (JSON)
		1,            // number_range_min
		90,           // number_range_max
		5,            // selection_count
		30,           // sales_cutoff_minutes
		1.0,          // base_price
		false,        // multi_draw_enabled
		"1.0",        // version
		"DRAFT")      // status
	require.NoError(s.T(), err)
}

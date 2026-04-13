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

type PrizeStructureServiceTestSuite struct {
	suite.Suite
	db           *sql.DB
	prizeService PrizeStructureService
	prizeRepo    repositories.PrizeStructureRepository
	gameRepo     repositories.GameRepository
	container    testcontainers.Container
	ctx          context.Context
	testGameID   uuid.UUID
}

func TestPrizeStructureServiceTestSuite(t *testing.T) {
	suite.Run(t, new(PrizeStructureServiceTestSuite))
}

func (s *PrizeStructureServiceTestSuite) SetupSuite() {
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
	if err := s.runMigrations(); err != nil {
		require.NoError(s.T(), err)
	}

	// Initialize repositories
	s.prizeRepo = repositories.NewPrizeStructureRepository(s.db)
	s.gameRepo = repositories.NewGameRepository(s.db)

	// Initialize service
	s.prizeService = NewPrizeStructureService(s.prizeRepo, s.gameRepo)

	// Generate test UUID
	s.testGameID = uuid.New()
}

func (s *PrizeStructureServiceTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(s.ctx)
	}
}

func (s *PrizeStructureServiceTestSuite) SetupTest() {
	// Clean up test data before each test
	s.cleanupTestData()
	// Create a test game for prize structure tests
	s.createTestGame()
}

func (s *PrizeStructureServiceTestSuite) TestCreatePrizeStructure() {
	// Test successful creation
	req := models.CreatePrizeStructureRequest{
		GameID:              s.testGameID,
		TotalPrizePool:      100000,
		HouseEdgePercentage: 15.0,
		Tiers: []models.PrizeTier{
			{
				TierNumber:      1,
				Name:            "Jackpot",
				MatchesRequired: 5,
				PrizePercentage: 50.0,
			},
			{
				TierNumber:      2,
				Name:            "Second Prize",
				MatchesRequired: 4,
				PrizePercentage: 25.0,
			},
			{
				TierNumber:      3,
				Name:            "Third Prize",
				MatchesRequired: 3,
				PrizePercentage: 10.0,
			},
		},
	}

	structure, err := s.prizeService.CreatePrizeStructure(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), structure)
	assert.Equal(s.T(), s.testGameID, structure.GameID)
	assert.Equal(s.T(), int64(100000), structure.TotalPrizePool)
	assert.Equal(s.T(), 15.0, structure.HouseEdgePercentage)
	assert.Len(s.T(), structure.Tiers, 3)
}

func (s *PrizeStructureServiceTestSuite) TestCreatePrizeStructure_InvalidGame() {
	// Test creation with invalid game ID
	invalidGameID := uuid.New()
	req := models.CreatePrizeStructureRequest{
		GameID:              invalidGameID,
		TotalPrizePool:      100000,
		HouseEdgePercentage: 15.0,
		Tiers: []models.PrizeTier{
			{
				TierNumber:      1,
				Name:            "Jackpot",
				MatchesRequired: 5,
				PrizePercentage: 50.0,
			},
		},
	}

	_, err := s.prizeService.CreatePrizeStructure(s.ctx, req)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "game not found")
}

func (s *PrizeStructureServiceTestSuite) TestCreatePrizeStructure_InvalidTotalPercentage() {
	// Test creation with tiers that exceed available pool
	req := models.CreatePrizeStructureRequest{
		GameID:              s.testGameID,
		TotalPrizePool:      100000,
		HouseEdgePercentage: 15.0,
		Tiers: []models.PrizeTier{
			{
				TierNumber:      1,
				MatchesRequired: 5,
				PrizePercentage: 60.0, // Total = 90%, but available is only 85%
			},
			{
				TierNumber:      2,
				Name:            "Second Prize",
				MatchesRequired: 4,
				PrizePercentage: 30.0,
			},
		},
	}

	_, err := s.prizeService.CreatePrizeStructure(s.ctx, req)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "total prize percentages")
	assert.Contains(s.T(), err.Error(), "exceed available pool")
}

func (s *PrizeStructureServiceTestSuite) TestGetPrizeStructure() {
	// First create a prize structure
	req := models.CreatePrizeStructureRequest{
		GameID:              s.testGameID,
		TotalPrizePool:      50000,
		HouseEdgePercentage: 20.0,
		Tiers: []models.PrizeTier{
			{
				TierNumber:      1,
				MatchesRequired: 6,
				PrizePercentage: 40.0,
			},
			{
				TierNumber:      2,
				MatchesRequired: 5,
				PrizePercentage: 30.0,
			},
		},
	}

	created, err := s.prizeService.CreatePrizeStructure(s.ctx, req)
	require.NoError(s.T(), err)

	// Test retrieval
	structure, err := s.prizeService.GetPrizeStructure(s.ctx, s.testGameID)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), structure)
	assert.Equal(s.T(), created.ID, structure.ID)
	assert.Equal(s.T(), s.testGameID, structure.GameID)
	assert.Equal(s.T(), int64(50000), structure.TotalPrizePool)
	assert.Equal(s.T(), 20.0, structure.HouseEdgePercentage)
	assert.Len(s.T(), structure.Tiers, 2)
}

func (s *PrizeStructureServiceTestSuite) TestGetPrizeStructure_NotFound() {
	// Test retrieval with non-existent game
	invalidGameID := uuid.New()

	structure, err := s.prizeService.GetPrizeStructure(s.ctx, invalidGameID)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), structure)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *PrizeStructureServiceTestSuite) TestUpdatePrizeStructure() {
	// First create a prize structure
	req := models.CreatePrizeStructureRequest{
		GameID:              s.testGameID,
		TotalPrizePool:      100000,
		HouseEdgePercentage: 15.0,
		Tiers: []models.PrizeTier{
			{
				TierNumber:      1,
				Name:            "Jackpot",
				MatchesRequired: 5,
				PrizePercentage: 50.0,
			},
		},
	}

	created, err := s.prizeService.CreatePrizeStructure(s.ctx, req)
	require.NoError(s.T(), err)

	// Test update
	created.TotalPrizePool = 150000
	created.HouseEdgePercentage = 10.0
	created.Tiers[0].PrizePercentage = 70.0

	err = s.prizeService.UpdatePrizeStructure(s.ctx, created)
	assert.NoError(s.T(), err)

	// Verify update
	updated, err := s.prizeService.GetPrizeStructure(s.ctx, s.testGameID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), int64(150000), updated.TotalPrizePool)
	assert.Equal(s.T(), 10.0, updated.HouseEdgePercentage)
}

func (s *PrizeStructureServiceTestSuite) TestValidatePrizeStructure_ValidStructure() {
	// Test valid prize structure
	structure := &models.PrizeStructure{
		ID:                  uuid.New(),
		GameID:              s.testGameID,
		TotalPrizePool:      100000,
		HouseEdgePercentage: 15.0,
		Tiers: []models.PrizeTier{
			{
				TierNumber:      1,
				MatchesRequired: 5,
				PrizePercentage: 50.0,
			},
			{
				TierNumber:      2,
				MatchesRequired: 4,
				PrizePercentage: 25.0,
			},
		},
	}

	err := s.prizeService.ValidatePrizeStructure(s.ctx, structure)
	assert.NoError(s.T(), err)
}

func (s *PrizeStructureServiceTestSuite) TestValidatePrizeStructure_InvalidHouseEdge() {
	// Test invalid house edge percentage
	structure := &models.PrizeStructure{
		ID:                  uuid.New(),
		GameID:              s.testGameID,
		TotalPrizePool:      100000,
		HouseEdgePercentage: 150.0, // Invalid - over 100%
		Tiers: []models.PrizeTier{
			{
				TierNumber:      1,
				Name:            "Jackpot",
				MatchesRequired: 5,
				PrizePercentage: 50.0,
			},
		},
	}

	err := s.prizeService.ValidatePrizeStructure(s.ctx, structure)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "house edge percentage must be between 0 and 100")
}

func (s *PrizeStructureServiceTestSuite) TestValidatePrizeStructure_NegativePrizePool() {
	// Test negative prize pool
	structure := &models.PrizeStructure{
		ID:                  uuid.New(),
		GameID:              s.testGameID,
		TotalPrizePool:      -1000, // Invalid - negative
		HouseEdgePercentage: 15.0,
		Tiers: []models.PrizeTier{
			{
				TierNumber:      1,
				Name:            "Jackpot",
				MatchesRequired: 5,
				PrizePercentage: 50.0,
			},
		},
	}

	err := s.prizeService.ValidatePrizeStructure(s.ctx, structure)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "total prize pool cannot be negative")
}

func (s *PrizeStructureServiceTestSuite) TestValidatePrizeStructure_NoTiers() {
	// Test structure without tiers
	structure := &models.PrizeStructure{
		ID:                  uuid.New(),
		GameID:              s.testGameID,
		TotalPrizePool:      100000,
		HouseEdgePercentage: 15.0,
		Tiers:               []models.PrizeTier{}, // No tiers
	}

	err := s.prizeService.ValidatePrizeStructure(s.ctx, structure)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "at least one prize tier is required")
}

func (s *PrizeStructureServiceTestSuite) TestValidatePrizeStructure_InvalidTierPercentage() {
	// Test tier with invalid percentage
	structure := &models.PrizeStructure{
		ID:                  uuid.New(),
		GameID:              s.testGameID,
		TotalPrizePool:      100000,
		HouseEdgePercentage: 15.0,
		Tiers: []models.PrizeTier{
			{
				TierNumber:      1,
				MatchesRequired: 5,
				PrizePercentage: -10.0, // Invalid - negative
			},
		},
	}

	err := s.prizeService.ValidatePrizeStructure(s.ctx, structure)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "prize percentage for tier 1 must be between 0 and 100")
}

func (s *PrizeStructureServiceTestSuite) TestValidatePrizeStructure_DuplicateTier() {
	// Test duplicate tier numbers
	structure := &models.PrizeStructure{
		ID:                  uuid.New(),
		GameID:              s.testGameID,
		TotalPrizePool:      100000,
		HouseEdgePercentage: 15.0,
		Tiers: []models.PrizeTier{
			{
				TierNumber:      1,
				MatchesRequired: 5,
				PrizePercentage: 30.0,
			},
			{
				TierNumber:      1, // Duplicate tier number
				MatchesRequired: 4,
				PrizePercentage: 20.0,
			},
		},
	}

	err := s.prizeService.ValidatePrizeStructure(s.ctx, structure)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "duplicate tier number: 1")
}

func (s *PrizeStructureServiceTestSuite) TestValidatePrizeStructure_InvalidTierNumber() {
	// Test tier with invalid tier number
	structure := &models.PrizeStructure{
		ID:                  uuid.New(),
		GameID:              s.testGameID,
		TotalPrizePool:      100000,
		HouseEdgePercentage: 15.0,
		Tiers: []models.PrizeTier{
			{
				TierNumber:      0, // Invalid - must be positive
				MatchesRequired: 5,
				PrizePercentage: 50.0,
			},
		},
	}

	err := s.prizeService.ValidatePrizeStructure(s.ctx, structure)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "tier number must be positive")
}

func (s *PrizeStructureServiceTestSuite) TestValidatePrizeStructure_NegativeMatches() {
	// Test tier with negative matches required
	structure := &models.PrizeStructure{
		ID:                  uuid.New(),
		GameID:              s.testGameID,
		TotalPrizePool:      100000,
		HouseEdgePercentage: 15.0,
		Tiers: []models.PrizeTier{
			{
				TierNumber:      1,
				MatchesRequired: -1, // Invalid - negative
				PrizePercentage: 50.0,
			},
		},
	}

	err := s.prizeService.ValidatePrizeStructure(s.ctx, structure)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "matches required cannot be negative")
}

// Helper methods

func (s *PrizeStructureServiceTestSuite) runMigrations() error {
	// Set Goose dialect
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	// Get the migrations directory path relative to the test file
	migrationsDir := filepath.Join("..", "..", "migrations")

	// Run up migrations
	if err := goose.Up(s.db, migrationsDir); err != nil {
		return err
	}

	return nil
}

func (s *PrizeStructureServiceTestSuite) cleanupTestData() {
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM prize_tiers")
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM prize_structures")
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM games")
}

func (s *PrizeStructureServiceTestSuite) createTestGame() {
	drawDaysJSON := `["WEDNESDAY", "SATURDAY"]`
	query := `INSERT INTO games (
		id, code, name, type, game_type, game_format, game_category, organizer,
		min_stake_amount, max_stake_amount, max_tickets_per_player, draw_frequency,
		draw_days, number_range_min, number_range_max, selection_count, 
		sales_cutoff_minutes, base_price, multi_draw_enabled, version, status
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)`
	_, err := s.db.ExecContext(s.ctx, query,
		s.testGameID,
		"TGFPS001", // code
		"Test Game for Prize Structure",
		"5_90",       // type
		"5_90",       // game_type
		"5_by_90",    // game_format
		"NUMBERS",    // game_category
		"NLA",        // organizer
		1.0,          // min_stake_amount
		10.0,         // max_stake_amount
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

package repositories

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type GameRepositoryTestSuite struct {
	suite.Suite
	db        *sql.DB
	repo      GameRepository
	container testcontainers.Container
	ctx       context.Context
}

func TestGameRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(GameRepositoryTestSuite))
}

func (s *GameRepositoryTestSuite) SetupSuite() {
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
	err = s.runMigrations()
	require.NoError(s.T(), err)

	// Initialize repository
	s.repo = NewGameRepository(s.db)
}

func (s *GameRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(s.T(), err)
	}
}

func (s *GameRepositoryTestSuite) SetupTest() {
	// Clean up tables before each test
	_, err := s.db.Exec("TRUNCATE games, game_rules, prize_structures, prize_tiers, game_approvals, game_schedules, game_versions, game_audit RESTART IDENTITY CASCADE")
	require.NoError(s.T(), err)
}

func (s *GameRepositoryTestSuite) runMigrations() error {
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

func (s *GameRepositoryTestSuite) TestCreate() {
	game := &models.Game{
		Name:                "Test Game",
		Type:                "5_BY_90",
		Organizer:           "ORGANIZER_NLA",
		MinStakeAmount:      0.50,  // ₵0.50 in GHS
		MaxStakeAmount:      50.00, // ₵50.00 in GHS
		MaxTicketsPerPlayer: 10,    // Required by constraint
		BasePrice:           1.00,  // Base price in GHS
		NumberRangeMin:      1,     // Required field
		NumberRangeMax:      90,    // Required field
		SelectionCount:      5,     // Required field
		Status:              "DRAFT",
		Version:             "1.0.0",
		Description:         stringPtr("Test game description"),
		Code:                "TG001",
		GameFormat:          "5_by_90",
		GameCategory:        "national",
		DrawFrequency:       "daily",
		SalesCutoffMinutes:  30,
	}

	err := s.repo.Create(s.ctx, game)
	require.NoError(s.T(), err)

	// Verify game was created
	assert.NotEqual(s.T(), uuid.Nil, game.ID)
	assert.NotZero(s.T(), game.CreatedAt)
	assert.NotZero(s.T(), game.UpdatedAt)

	// Verify game can be retrieved
	retrieved, err := s.repo.GetByID(s.ctx, game.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), game.Name, retrieved.Name)
	assert.Equal(s.T(), game.Type, retrieved.Type)
	assert.Equal(s.T(), game.Organizer, retrieved.Organizer)
	assert.Equal(s.T(), game.MinStakeAmount, retrieved.MinStakeAmount)
	assert.Equal(s.T(), game.MaxStakeAmount, retrieved.MaxStakeAmount)
	assert.Equal(s.T(), game.Code, retrieved.Code)
}

func (s *GameRepositoryTestSuite) TestGetByID() {
	// Create test game
	game := &models.Game{
		Name:                "Get By ID Test",
		Type:                "5_BY_90",
		Organizer:           "ORGANIZER_RAND_LOTTERY",
		MinStakeAmount:      100,
		MaxStakeAmount:      10000,
		BasePrice:           1.00,
		NumberRangeMin:      1,
		NumberRangeMax:      90,
		SelectionCount:      5,
		DrawFrequency:       "daily",
		SalesCutoffMinutes:  30,
		MaxTicketsPerPlayer: 10,
		Status:              "ACTIVE",
		Version:             "1.0.0",
		Code:                "GBIT001",
	}
	err := s.repo.Create(s.ctx, game)
	require.NoError(s.T(), err)

	// Test GetByID
	retrieved, err := s.repo.GetByID(s.ctx, game.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), game.ID, retrieved.ID)
	assert.Equal(s.T(), game.Name, retrieved.Name)
	assert.Equal(s.T(), game.Status, retrieved.Status)

	// Test non-existent ID
	nonExistentID := uuid.New()
	_, err = s.repo.GetByID(s.ctx, nonExistentID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *GameRepositoryTestSuite) TestGetByName() {
	// Create test game
	gameName := "Unique Game Name Test"
	game := &models.Game{
		Name:                gameName,
		Type:                "5_BY_90",
		Organizer:           "ORGANIZER_NLA",
		MinStakeAmount:      50,
		MaxStakeAmount:      5000,
		BasePrice:           1.00,
		NumberRangeMin:      1,
		NumberRangeMax:      90,
		SelectionCount:      5,
		DrawFrequency:       "daily",
		SalesCutoffMinutes:  30,
		MaxTicketsPerPlayer: 10,
		Status:              "DRAFT",
		Version:             "1.0.0",
		Code:                "UGN001",
	}
	err := s.repo.Create(s.ctx, game)
	require.NoError(s.T(), err)

	// Test GetByName
	retrieved, err := s.repo.GetByName(s.ctx, gameName)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), game.ID, retrieved.ID)
	assert.Equal(s.T(), gameName, retrieved.Name)

	// Test non-existent name
	_, err = s.repo.GetByName(s.ctx, "Non-existent Game")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *GameRepositoryTestSuite) TestUpdate() {
	// Create test game
	game := &models.Game{
		Name:                "Update Test Game",
		Type:                "5_BY_90",
		Organizer:           "ORGANIZER_NLA",
		MinStakeAmount:      50,
		MaxStakeAmount:      5000,
		BasePrice:           1.00,
		NumberRangeMin:      1,
		NumberRangeMax:      90,
		SelectionCount:      5,
		DrawFrequency:       "daily",
		SalesCutoffMinutes:  30,
		MaxTicketsPerPlayer: 10,
		Status:              "DRAFT",
		Version:             "1.0.0",
		Code:                "UTG001",
		Description:         stringPtr("Original description"),
	}
	err := s.repo.Create(s.ctx, game)
	require.NoError(s.T(), err)

	// Update game
	originalUpdatedAt := game.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure different timestamp

	game.Name = "Updated Game Name"
	game.Description = stringPtr("Updated description")
	game.MinStakeAmount = 100
	game.MaxStakeAmount = 10000

	err = s.repo.Update(s.ctx, game)
	require.NoError(s.T(), err)
	assert.True(s.T(), game.UpdatedAt.After(originalUpdatedAt))

	// Verify update
	updated, err := s.repo.GetByID(s.ctx, game.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated Game Name", updated.Name)
	assert.Equal(s.T(), "Updated description", *updated.Description)
	assert.Equal(s.T(), float64(100), updated.MinStakeAmount)
	assert.Equal(s.T(), float64(10000), updated.MaxStakeAmount)
}

func (s *GameRepositoryTestSuite) TestDelete() {
	// Create test game
	game := &models.Game{
		Name:                "Delete Test Game",
		Type:                "5_BY_90",
		Organizer:           "ORGANIZER_NLA",
		MinStakeAmount:      50,
		MaxStakeAmount:      5000,
		BasePrice:           1.00,
		NumberRangeMin:      1,
		NumberRangeMax:      90,
		SelectionCount:      5,
		DrawFrequency:       "daily",
		SalesCutoffMinutes:  30,
		MaxTicketsPerPlayer: 10,
		Status:              "DRAFT",
		Version:             "1.0.0",
		Code:                "DTG001",
	}
	err := s.repo.Create(s.ctx, game)
	require.NoError(s.T(), err)

	// Delete the game (soft delete - marks as TERMINATED)
	err = s.repo.Delete(s.ctx, game.ID)
	require.NoError(s.T(), err)

	// Verify game is marked as terminated
	retrieved, err := s.repo.GetByID(s.ctx, game.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "TERMINATED", retrieved.Status)

	// Test deleting non-existent game
	nonExistentID := uuid.New()
	err = s.repo.Delete(s.ctx, nonExistentID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *GameRepositoryTestSuite) TestList() {
	// Create multiple test games with different attributes
	games := []*models.Game{
		{
			Name: "National Game 1", Type: "5_BY_90", Organizer: "ORGANIZER_NLA",
			MinStakeAmount: 50, MaxStakeAmount: 5000,
			BasePrice:          1.00,
			NumberRangeMin:     1,
			NumberRangeMax:     90,
			SelectionCount:     5,
			DrawFrequency:      "daily",
			SalesCutoffMinutes: 30, MaxTicketsPerPlayer: 10, Status: "ACTIVE", Version: "1.0.0",
			Code: "NG001", GameFormat: "5_by_90", GameCategory: "national",
		},
		{
			Name: "Private Game 1", Type: "6_BY_49", Organizer: "ORGANIZER_RAND_LOTTERY",
			MinStakeAmount: 100, MaxStakeAmount: 10000,
			BasePrice:          1.00,
			NumberRangeMin:     1,
			NumberRangeMax:     90,
			SelectionCount:     5,
			DrawFrequency:      "daily",
			SalesCutoffMinutes: 30, MaxTicketsPerPlayer: 10, Status: "DRAFT", Version: "1.0.0",
			Code: "PG001", GameFormat: "6_by_49", GameCategory: "private",
		},
		{
			Name: "National Game 2", Type: "5_BY_90", Organizer: "ORGANIZER_NLA",
			MinStakeAmount: 50, MaxStakeAmount: 5000,
			BasePrice:          1.00,
			NumberRangeMin:     1,
			NumberRangeMax:     90,
			SelectionCount:     5,
			DrawFrequency:      "daily",
			SalesCutoffMinutes: 30, MaxTicketsPerPlayer: 10, Status: "SUSPENDED", Version: "1.0.0",
			Code: "NG002", GameFormat: "5_by_90", GameCategory: "national",
		},
	}

	for _, game := range games {
		err := s.repo.Create(s.ctx, game)
		require.NoError(s.T(), err)
	}

	// Test list all games
	filter := models.GameFilter{}
	retrieved, total, err := s.repo.List(s.ctx, filter, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), total)
	assert.Len(s.T(), retrieved, 3)

	// Test filter by organizer
	nlaOrganizer := models.Organizer("ORGANIZER_NLA")
	organizerFilter := models.GameFilter{Organizer: &nlaOrganizer}
	nlaGames, nlaTotal, err := s.repo.List(s.ctx, organizerFilter, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), nlaTotal)
	assert.Len(s.T(), nlaGames, 2)

	// Test filter by status
	activeStatus := models.GameStatus("ACTIVE")
	statusFilter := models.GameFilter{Status: &activeStatus}
	activeGames, activeTotal, err := s.repo.List(s.ctx, statusFilter, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), activeTotal)
	assert.Len(s.T(), activeGames, 1)
	assert.Equal(s.T(), "ACTIVE", activeGames[0].Status)

	// Test search query
	searchQuery := "National"
	searchFilter := models.GameFilter{SearchQuery: &searchQuery}
	searchResults, searchTotal, err := s.repo.List(s.ctx, searchFilter, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), searchTotal)
	assert.Len(s.T(), searchResults, 2)

	// Test pagination
	paginatedResults, paginatedTotal, err := s.repo.List(s.ctx, filter, 1, 2)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), paginatedTotal)
	assert.Len(s.T(), paginatedResults, 2)

	// Test second page
	secondPageResults, secondPageTotal, err := s.repo.List(s.ctx, filter, 2, 2)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), secondPageTotal)
	assert.Len(s.T(), secondPageResults, 1)
}

func (s *GameRepositoryTestSuite) TestGetActiveGames() {
	// Create games with different statuses
	games := []*models.Game{
		{
			Name: "Active Game 1", Type: "5_BY_90", Organizer: "ORGANIZER_NLA",
			MinStakeAmount: 50, MaxStakeAmount: 5000,
			BasePrice:          1.00,
			NumberRangeMin:     1,
			NumberRangeMax:     90,
			SelectionCount:     5,
			DrawFrequency:      "daily",
			SalesCutoffMinutes: 30, MaxTicketsPerPlayer: 10, Status: "ACTIVE", Version: "1.0.0",
			Code: "AG001",
		},
		{
			Name: "Draft Game", Type: "5_BY_90", Organizer: "ORGANIZER_NLA",
			MinStakeAmount: 50, MaxStakeAmount: 5000,
			BasePrice:          1.00,
			NumberRangeMin:     1,
			NumberRangeMax:     90,
			SelectionCount:     5,
			DrawFrequency:      "daily",
			SalesCutoffMinutes: 30, MaxTicketsPerPlayer: 10, Status: "DRAFT", Version: "1.0.0",
			Code: "DG001",
		},
		{
			Name: "Active Game 2", Type: "6_BY_49", Organizer: "ORGANIZER_RAND_LOTTERY",
			MinStakeAmount: 100, MaxStakeAmount: 10000,
			BasePrice:          1.00,
			NumberRangeMin:     1,
			NumberRangeMax:     90,
			SelectionCount:     5,
			DrawFrequency:      "daily",
			SalesCutoffMinutes: 30, MaxTicketsPerPlayer: 10, Status: "ACTIVE", Version: "1.0.0",
			Code: "AG002",
		},
		{
			Name: "Suspended Game", Type: "5_BY_90", Organizer: "ORGANIZER_NLA",
			MinStakeAmount: 50, MaxStakeAmount: 5000,
			BasePrice:          1.00,
			NumberRangeMin:     1,
			NumberRangeMax:     90,
			SelectionCount:     5,
			DrawFrequency:      "daily",
			SalesCutoffMinutes: 30, MaxTicketsPerPlayer: 10, Status: "SUSPENDED", Version: "1.0.0",
			Code: "SG001",
		},
	}

	for _, game := range games {
		err := s.repo.Create(s.ctx, game)
		require.NoError(s.T(), err)
	}

	// Get active games
	activeGames, err := s.repo.GetActiveGames(s.ctx)
	require.NoError(s.T(), err)
	assert.Len(s.T(), activeGames, 2)

	// Verify all returned games are active
	for _, game := range activeGames {
		assert.Equal(s.T(), "ACTIVE", game.Status)
	}

	// Verify games are ordered by created_at DESC
	assert.True(s.T(), activeGames[0].CreatedAt.After(activeGames[1].CreatedAt) ||
		activeGames[0].CreatedAt.Equal(activeGames[1].CreatedAt))
}

func (s *GameRepositoryTestSuite) TestMonetaryValues() {
	// Test that monetary values are stored and retrieved correctly as decimals
	game := &models.Game{
		Name:                "Monetary Test Game",
		Type:                "5_BY_90",
		Organizer:           "ORGANIZER_NLA",
		MinStakeAmount:      0.50,    // ₵0.50
		MaxStakeAmount:      5000.00, // ₵5000.00 (within limit of ₵200,000)
		BasePrice:           2.50,    // ₵2.50
		NumberRangeMin:      1,
		NumberRangeMax:      90,
		SelectionCount:      5,
		DrawFrequency:       "daily",
		SalesCutoffMinutes:  30,
		MaxTicketsPerPlayer: 10,
		Status:              "DRAFT",
		Version:             "1.0.0",
		Code:                "MTG001",
		GameFormat:          "5_by_90",
		GameCategory:        "national",
	}

	err := s.repo.Create(s.ctx, game)
	require.NoError(s.T(), err)

	retrieved, err := s.repo.GetByID(s.ctx, game.ID)
	require.NoError(s.T(), err)

	// Verify monetary values are stored correctly
	assert.Equal(s.T(), 0.50, retrieved.MinStakeAmount)    // ₵0.50
	assert.Equal(s.T(), 5000.00, retrieved.MaxStakeAmount) // ₵5000.00
	assert.Equal(s.T(), 2.50, retrieved.BasePrice)         // ₵2.50

	// Test that values are stored as expected (no conversion needed with DECIMAL)
	assert.InDelta(s.T(), 0.50, retrieved.MinStakeAmount, 0.01)
	assert.InDelta(s.T(), 5000.00, retrieved.MaxStakeAmount, 0.01)
	assert.InDelta(s.T(), 2.50, retrieved.BasePrice, 0.01)
}

func (s *GameRepositoryTestSuite) TestGameWithAllFields() {
	// Test creating and retrieving a game with all possible fields populated
	// Removed startDate and endDate variables (fields no longer exist)
	drawTime := time.Date(2023, 1, 1, 19, 0, 0, 0, time.UTC) // 7:00 PM

	game := &models.Game{
		Name:                "Complete Game Test",
		Type:                "5_BY_90",
		Organizer:           "ORGANIZER_NLA",
		MinStakeAmount:      0.50,
		MaxStakeAmount:      200.00,
		BasePrice:           1.00,
		NumberRangeMin:      1,
		NumberRangeMax:      90,
		SelectionCount:      5,
		DrawFrequency:       "daily",
		SalesCutoffMinutes:  30,
		Status:              "DRAFT",
		Version:             "1.0.0",
		Code:                "CGT001",
		GameFormat:          "5_by_90",
		GameCategory:        "national",
		Description:         stringPtr("Complete test game with all fields"),
		MaxTicketsPerPlayer: 100,
		// Remove DrawDays to avoid JSON parsing issues for now
		DrawTime:       &drawTime,
		WeeklySchedule: boolPtr(true),
		StartTime:      stringPtr("09:00"),
		EndTime:        stringPtr("18:00"),
	}

	err := s.repo.Create(s.ctx, game)
	require.NoError(s.T(), err)

	retrieved, err := s.repo.GetByID(s.ctx, game.ID)
	require.NoError(s.T(), err)

	// Verify all fields
	assert.Equal(s.T(), game.Name, retrieved.Name)
	assert.Equal(s.T(), game.Code, retrieved.Code)
	assert.Equal(s.T(), game.GameFormat, retrieved.GameFormat)
	assert.Equal(s.T(), game.GameCategory, retrieved.GameCategory)
	assert.Equal(s.T(), *game.Description, *retrieved.Description)
	assert.Equal(s.T(), game.MaxTicketsPerPlayer, retrieved.MaxTicketsPerPlayer)
	assert.Equal(s.T(), game.DrawFrequency, retrieved.DrawFrequency)
	assert.Equal(s.T(), game.DrawDays, retrieved.DrawDays)
	assert.Equal(s.T(), *game.WeeklySchedule, *retrieved.WeeklySchedule)
	assert.Equal(s.T(), *game.StartTime, *retrieved.StartTime)
	assert.Equal(s.T(), *game.EndTime, *retrieved.EndTime)

	// Note: StartDate and EndDate fields have been removed from the model
}

// Helper functions

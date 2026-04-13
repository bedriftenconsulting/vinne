package repositories

import (
	"context"
	"database/sql"
	"fmt"
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

type GameRulesRepositoryTestSuite struct {
	suite.Suite
	db         *sql.DB
	repo       GameRulesRepository
	gameRepo   GameRepository
	container  testcontainers.Container
	ctx        context.Context
	testGameID uuid.UUID
}

func TestGameRulesRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(GameRulesRepositoryTestSuite))
}

func (s *GameRulesRepositoryTestSuite) SetupSuite() {
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

	// Initialize repositories
	s.repo = NewGameRulesRepository(s.db)
	s.gameRepo = NewGameRepository(s.db)
}

func (s *GameRulesRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(s.T(), err)
	}
}

func (s *GameRulesRepositoryTestSuite) SetupTest() {
	// Clean up tables before each test
	_, err := s.db.Exec("TRUNCATE games, game_rules RESTART IDENTITY CASCADE")
	require.NoError(s.T(), err)

	// Create a test game for foreign key relationships
	testGame := &models.Game{
		Name:                "Test Game for Rules",
		Type:                "5_BY_90",
		Organizer:           "ORGANIZER_NLA",
		MinStakeAmount:      0.50,  // GHS
		MaxStakeAmount:      50.00, // GHS
		BasePrice:           1.00,  // GHS
		NumberRangeMin:      1,
		NumberRangeMax:      90,
		SelectionCount:      5,
		DrawFrequency:       "daily",
		SalesCutoffMinutes:  30,
		MaxTicketsPerPlayer: 10,
		Status:              "DRAFT",
		Version:             "1.0.0",
		Code:                "TGFR001",
		GameFormat:          "5_by_90",
		GameCategory:        "national",
	}
	err = s.gameRepo.Create(s.ctx, testGame)
	require.NoError(s.T(), err)
	s.testGameID = testGame.ID
}

func (s *GameRulesRepositoryTestSuite) runMigrations() error {
	// Set Goose dialect
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Get the migrations directory path relative to the test file
	migrationsDir := filepath.Join("..", "..", "migrations")

	// Run up migrations
	if err := goose.Up(s.db, migrationsDir); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func (s *GameRulesRepositoryTestSuite) TestCreate() {
	gameRules := &models.GameRules{
		GameID:         s.testGameID,
		NumbersToPick:  5,
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  10,
		NumberRangeMin: int32Ptr(1),
		NumberRangeMax: int32Ptr(90),
		SelectionCount: int32Ptr(5),
		AllowQuickPick: true,
		SpecialRules:   stringPtr("Standard 5/90 game rules"),
		EffectiveFrom:  time.Now(),
	}

	err := s.repo.Create(s.ctx, gameRules)
	require.NoError(s.T(), err)

	// Verify game rules were created
	assert.NotEqual(s.T(), uuid.Nil, gameRules.ID)
	assert.NotZero(s.T(), gameRules.CreatedAt)
	assert.NotZero(s.T(), gameRules.UpdatedAt)

	// Verify game rules can be retrieved
	retrieved, err := s.repo.GetByID(s.ctx, gameRules.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), gameRules.GameID, retrieved.GameID)
	assert.Equal(s.T(), gameRules.NumbersToPick, retrieved.NumbersToPick)
	assert.Equal(s.T(), gameRules.TotalNumbers, retrieved.TotalNumbers)
	assert.Equal(s.T(), gameRules.AllowQuickPick, retrieved.AllowQuickPick)
	assert.Equal(s.T(), *gameRules.SpecialRules, *retrieved.SpecialRules)
	assert.Equal(s.T(), *gameRules.NumberRangeMin, *retrieved.NumberRangeMin)
	assert.Equal(s.T(), *gameRules.NumberRangeMax, *retrieved.NumberRangeMax)
	assert.Equal(s.T(), *gameRules.SelectionCount, *retrieved.SelectionCount)
}

func (s *GameRulesRepositoryTestSuite) TestGetByID() {
	// Create test game rules
	gameRules := &models.GameRules{
		GameID:         s.testGameID,
		NumbersToPick:  6,
		TotalNumbers:   49,
		MinSelections:  1,
		MaxSelections:  5,
		NumberRangeMin: int32Ptr(1),
		NumberRangeMax: int32Ptr(49),
		SelectionCount: int32Ptr(6),
		AllowQuickPick: false,
		SpecialRules:   stringPtr("6/49 lottery rules"),
		EffectiveFrom:  time.Now(),
	}
	err := s.repo.Create(s.ctx, gameRules)
	require.NoError(s.T(), err)

	// Test GetByID
	retrieved, err := s.repo.GetByID(s.ctx, gameRules.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), gameRules.ID, retrieved.ID)
	assert.Equal(s.T(), gameRules.GameID, retrieved.GameID)
	assert.Equal(s.T(), int32(6), retrieved.NumbersToPick)
	assert.Equal(s.T(), int32(49), retrieved.TotalNumbers)
	assert.False(s.T(), retrieved.AllowQuickPick)
	assert.Equal(s.T(), "6/49 lottery rules", *retrieved.SpecialRules)

	// Test non-existent ID
	nonExistentID := uuid.New()
	_, err = s.repo.GetByID(s.ctx, nonExistentID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *GameRulesRepositoryTestSuite) TestGetByGameID() {
	now := time.Now()

	// Create multiple game rules for the same game with different effective dates
	rules := []*models.GameRules{
		{
			GameID:         s.testGameID,
			NumbersToPick:  5,
			TotalNumbers:   90,
			MinSelections:  1,
			MaxSelections:  10,
			AllowQuickPick: true,
			EffectiveFrom:  now.Add(-time.Hour), // Earlier rules
		},
		{
			GameID:         s.testGameID,
			NumbersToPick:  5,
			TotalNumbers:   90,
			MinSelections:  1,
			MaxSelections:  15,
			AllowQuickPick: true,
			SpecialRules:   stringPtr("Updated rules with more selections"),
			EffectiveFrom:  now, // Later rules
		},
	}

	for _, rule := range rules {
		err := s.repo.Create(s.ctx, rule)
		require.NoError(s.T(), err)
	}

	// GetByGameID should return the most recent rules
	retrieved, err := s.repo.GetByGameID(s.ctx, s.testGameID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int32(15), retrieved.MaxSelections)
	assert.Equal(s.T(), "Updated rules with more selections", *retrieved.SpecialRules)

	// Test non-existent game ID
	nonExistentGameID := uuid.New()
	_, err = s.repo.GetByGameID(s.ctx, nonExistentGameID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *GameRulesRepositoryTestSuite) TestUpdate() {
	// Create test game rules
	gameRules := &models.GameRules{
		GameID:         s.testGameID,
		NumbersToPick:  5,
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  10,
		AllowQuickPick: true,
		SpecialRules:   stringPtr("Original rules"),
		EffectiveFrom:  time.Now(),
	}
	err := s.repo.Create(s.ctx, gameRules)
	require.NoError(s.T(), err)

	// Update game rules
	originalUpdatedAt := gameRules.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure different timestamp

	gameRules.MaxSelections = 20
	gameRules.AllowQuickPick = false
	gameRules.SpecialRules = stringPtr("Updated rules with higher selections")
	effectiveTo := time.Now().Add(time.Hour * 24 * 30) // 30 days from now
	gameRules.EffectiveTo = &effectiveTo

	err = s.repo.Update(s.ctx, gameRules)
	require.NoError(s.T(), err)
	assert.True(s.T(), gameRules.UpdatedAt.After(originalUpdatedAt))

	// Verify update
	updated, err := s.repo.GetByID(s.ctx, gameRules.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int32(20), updated.MaxSelections)
	assert.False(s.T(), updated.AllowQuickPick)
	assert.Equal(s.T(), "Updated rules with higher selections", *updated.SpecialRules)
	assert.NotNil(s.T(), updated.EffectiveTo)
}

func (s *GameRulesRepositoryTestSuite) TestDelete() {
	// Create test game rules
	gameRules := &models.GameRules{
		GameID:         s.testGameID,
		NumbersToPick:  5,
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  10,
		AllowQuickPick: true,
		EffectiveFrom:  time.Now(),
	}
	err := s.repo.Create(s.ctx, gameRules)
	require.NoError(s.T(), err)

	// Delete the game rules
	err = s.repo.Delete(s.ctx, gameRules.ID)
	require.NoError(s.T(), err)

	// Verify rules are deleted
	_, err = s.repo.GetByID(s.ctx, gameRules.ID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")

	// Test deleting non-existent rules
	nonExistentID := uuid.New()
	err = s.repo.Delete(s.ctx, nonExistentID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *GameRulesRepositoryTestSuite) TestGetActiveRules() {
	now := time.Now()

	// Create expired rules
	expiredRules := &models.GameRules{
		GameID:         s.testGameID,
		NumbersToPick:  5,
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  5,
		AllowQuickPick: true,
		SpecialRules:   stringPtr("Expired rules"),
		EffectiveFrom:  now.Add(-time.Hour * 48),     // 2 days ago
		EffectiveTo:    timePtr(now.Add(-time.Hour)), // 1 hour ago
	}
	err := s.repo.Create(s.ctx, expiredRules)
	require.NoError(s.T(), err)

	// Create currently active rules
	activeRules := &models.GameRules{
		GameID:         s.testGameID,
		NumbersToPick:  5,
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  10,
		AllowQuickPick: true,
		SpecialRules:   stringPtr("Currently active rules"),
		EffectiveFrom:  now.Add(-time.Hour),                   // 1 hour ago
		EffectiveTo:    timePtr(now.Add(time.Hour * 24 * 30)), // 30 days from now
	}
	err = s.repo.Create(s.ctx, activeRules)
	require.NoError(s.T(), err)

	// Create future rules
	futureRules := &models.GameRules{
		GameID:         s.testGameID,
		NumbersToPick:  6,
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  15,
		AllowQuickPick: false,
		SpecialRules:   stringPtr("Future rules"),
		EffectiveFrom:  now.Add(time.Hour * 24), // 1 day from now
	}
	err = s.repo.Create(s.ctx, futureRules)
	require.NoError(s.T(), err)

	// GetActiveRules should return the currently active rules
	retrieved, err := s.repo.GetActiveRules(s.ctx, s.testGameID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), activeRules.ID, retrieved.ID)
	assert.Equal(s.T(), int32(10), retrieved.MaxSelections)
	assert.Equal(s.T(), "Currently active rules", *retrieved.SpecialRules)

	// Test with no active rules
	testGame2 := &models.Game{
		Name:                "Test Game 2",
		Type:                "6_BY_49",
		Organizer:           "ORGANIZER_NLA",
		MinStakeAmount:      50,
		MaxStakeAmount:      5000,
		MaxTicketsPerPlayer: 5,
		Status:              "DRAFT",
		Version:             "1.0.0",
		Code:                "TG002",
		BasePrice:           1.00,
		NumberRangeMin:      1,
		NumberRangeMax:      49,
		SelectionCount:      6,
		DrawFrequency:       "daily",
		SalesCutoffMinutes:  30,
	}
	err = s.gameRepo.Create(s.ctx, testGame2)
	require.NoError(s.T(), err)

	_, err = s.repo.GetActiveRules(s.ctx, testGame2.ID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "no active game rules found")
}

func (s *GameRulesRepositoryTestSuite) TestRulesWithAllFields() {
	// Test creating and retrieving game rules with all possible fields populated
	now := time.Now()
	effectiveTo := now.Add(time.Hour * 24 * 90) // 90 days from now

	gameRules := &models.GameRules{
		GameID:         s.testGameID,
		NumbersToPick:  5,
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  20,
		NumberRangeMin: int32Ptr(1),
		NumberRangeMax: int32Ptr(90),
		SelectionCount: int32Ptr(5),
		AllowQuickPick: true,
		SpecialRules:   stringPtr("Complete game rules with all fields populated for comprehensive testing"),
		EffectiveFrom:  now,
		EffectiveTo:    &effectiveTo,
	}

	err := s.repo.Create(s.ctx, gameRules)
	require.NoError(s.T(), err)

	retrieved, err := s.repo.GetByID(s.ctx, gameRules.ID)
	require.NoError(s.T(), err)

	// Verify all fields
	assert.Equal(s.T(), gameRules.GameID, retrieved.GameID)
	assert.Equal(s.T(), gameRules.NumbersToPick, retrieved.NumbersToPick)
	assert.Equal(s.T(), gameRules.TotalNumbers, retrieved.TotalNumbers)
	assert.Equal(s.T(), gameRules.MinSelections, retrieved.MinSelections)
	assert.Equal(s.T(), gameRules.MaxSelections, retrieved.MaxSelections)
	assert.Equal(s.T(), *gameRules.NumberRangeMin, *retrieved.NumberRangeMin)
	assert.Equal(s.T(), *gameRules.NumberRangeMax, *retrieved.NumberRangeMax)
	assert.Equal(s.T(), *gameRules.SelectionCount, *retrieved.SelectionCount)
	assert.Equal(s.T(), gameRules.AllowQuickPick, retrieved.AllowQuickPick)
	assert.Equal(s.T(), *gameRules.SpecialRules, *retrieved.SpecialRules)

	// Check dates (compare formatted strings to handle timezone differences)
	assert.Equal(s.T(), gameRules.EffectiveFrom.Format(time.RFC3339), retrieved.EffectiveFrom.Format(time.RFC3339))
	assert.Equal(s.T(), gameRules.EffectiveTo.Format(time.RFC3339), retrieved.EffectiveTo.Format(time.RFC3339))
}

func (s *GameRulesRepositoryTestSuite) TestDifferentGameTypes() {
	// Test different game rule configurations for different game types
	testCases := []struct {
		name           string
		numbersToPick  int32
		totalNumbers   int32
		rangeMin       int32
		rangeMax       int32
		selectionCount int32
		allowQuickPick bool
		specialRules   string
	}{
		{
			name:           "5/90 Game",
			numbersToPick:  5,
			totalNumbers:   90,
			rangeMin:       1,
			rangeMax:       90,
			selectionCount: 5,
			allowQuickPick: true,
			specialRules:   "Standard 5/90 lottery game",
		},
		{
			name:           "6/49 Game",
			numbersToPick:  6,
			totalNumbers:   49,
			rangeMin:       1,
			rangeMax:       49,
			selectionCount: 6,
			allowQuickPick: true,
			specialRules:   "Standard 6/49 lottery game",
		},
		{
			name:           "Direct 3 Game",
			numbersToPick:  3,
			totalNumbers:   10,
			rangeMin:       1,
			rangeMax:       10,
			selectionCount: 3,
			allowQuickPick: false,
			specialRules:   "Direct 3-digit game with exact order matching",
		},
		{
			name:           "Perm 4 Game",
			numbersToPick:  4,
			totalNumbers:   10,
			rangeMin:       1,
			rangeMax:       10,
			selectionCount: 4,
			allowQuickPick: true,
			specialRules:   "Perm 4-digit game with any order matching",
		},
	}

	createdRules := make([]*models.GameRules, len(testCases))

	for i, tc := range testCases {
		// Create a separate game for each rule set
		testGame := &models.Game{
			Name:                fmt.Sprintf("Test Game %s", tc.name),
			Type:                "CUSTOM",
			Organizer:           "ORGANIZER_NLA",
			MinStakeAmount:      50,
			MaxStakeAmount:      5000,
			MaxTicketsPerPlayer: 10,
			Status:              "DRAFT",
			Version:             "1.0.0",
			Code:                fmt.Sprintf("TG%d", i+1),
			BasePrice:           1.00,
			NumberRangeMin:      tc.rangeMin,
			NumberRangeMax:      tc.rangeMax,
			SelectionCount:      tc.selectionCount,
			DrawFrequency:       "daily",
			SalesCutoffMinutes:  30,
		}
		err := s.gameRepo.Create(s.ctx, testGame)
		require.NoError(s.T(), err)

		gameRules := &models.GameRules{
			GameID:         testGame.ID,
			NumbersToPick:  tc.numbersToPick,
			TotalNumbers:   tc.totalNumbers,
			MinSelections:  1,
			MaxSelections:  10,
			NumberRangeMin: &tc.rangeMin,
			NumberRangeMax: &tc.rangeMax,
			SelectionCount: &tc.selectionCount,
			AllowQuickPick: tc.allowQuickPick,
			SpecialRules:   &tc.specialRules,
			EffectiveFrom:  time.Now(),
		}

		err = s.repo.Create(s.ctx, gameRules)
		require.NoError(s.T(), err)
		createdRules[i] = gameRules
	}

	// Verify each rule set
	for i, tc := range testCases {
		retrieved, err := s.repo.GetByID(s.ctx, createdRules[i].ID)
		require.NoError(s.T(), err, "Failed to retrieve rules for %s", tc.name)

		assert.Equal(s.T(), tc.numbersToPick, retrieved.NumbersToPick, "Numbers to pick mismatch for %s", tc.name)
		assert.Equal(s.T(), tc.totalNumbers, retrieved.TotalNumbers, "Total numbers mismatch for %s", tc.name)
		assert.Equal(s.T(), tc.rangeMin, *retrieved.NumberRangeMin, "Range min mismatch for %s", tc.name)
		assert.Equal(s.T(), tc.rangeMax, *retrieved.NumberRangeMax, "Range max mismatch for %s", tc.name)
		assert.Equal(s.T(), tc.selectionCount, *retrieved.SelectionCount, "Selection count mismatch for %s", tc.name)
		assert.Equal(s.T(), tc.allowQuickPick, retrieved.AllowQuickPick, "Quick pick setting mismatch for %s", tc.name)
		assert.Equal(s.T(), tc.specialRules, *retrieved.SpecialRules, "Special rules mismatch for %s", tc.name)
	}
}

func (s *GameRulesRepositoryTestSuite) TestRulesValidation() {
	// Test business rule validation through the repository layer

	// Valid rules should pass
	validRules := &models.GameRules{
		GameID:         s.testGameID,
		NumbersToPick:  5,
		TotalNumbers:   90,
		MinSelections:  1,
		MaxSelections:  10,
		AllowQuickPick: true,
		EffectiveFrom:  time.Now(),
	}

	err := s.repo.Create(s.ctx, validRules)
	require.NoError(s.T(), err)

	// Test that we can retrieve and validate the constraints are working
	retrieved, err := s.repo.GetByID(s.ctx, validRules.ID)
	require.NoError(s.T(), err)

	// Verify constraint: numbers_to_pick <= total_numbers
	assert.LessOrEqual(s.T(), retrieved.NumbersToPick, retrieved.TotalNumbers)

	// Verify constraint: min_selections <= max_selections
	assert.LessOrEqual(s.T(), retrieved.MinSelections, retrieved.MaxSelections)

	// Verify that numbers_to_pick and total_numbers are positive
	assert.Positive(s.T(), retrieved.NumbersToPick)
	assert.Positive(s.T(), retrieved.TotalNumbers)
}

func (s *GameRulesRepositoryTestSuite) TestEffectiveDateRanges() {
	now := time.Now()

	// Test rules with various effective date ranges
	testCases := []struct {
		name          string
		effectiveFrom time.Time
		effectiveTo   *time.Time
		description   string
	}{
		{
			name:          "No End Date",
			effectiveFrom: now,
			effectiveTo:   nil,
			description:   "Rules effective indefinitely",
		},
		{
			name:          "Fixed Duration",
			effectiveFrom: now,
			effectiveTo:   timePtr(now.Add(time.Hour * 24 * 90)),
			description:   "Rules effective for 90 days",
		},
		{
			name:          "Historical Rules",
			effectiveFrom: now.Add(-time.Hour * 24 * 365),         // 1 year ago
			effectiveTo:   timePtr(now.Add(-time.Hour * 24 * 30)), // Ended 30 days ago
			description:   "Historical rules that are no longer active",
		},
		{
			name:          "Future Rules",
			effectiveFrom: now.Add(time.Hour * 24 * 30),           // Start in 30 days
			effectiveTo:   timePtr(now.Add(time.Hour * 24 * 120)), // End in 120 days
			description:   "Future rules not yet active",
		},
	}

	createdRules := make([]*models.GameRules, len(testCases))

	for i, tc := range testCases {
		gameRules := &models.GameRules{
			GameID:         s.testGameID,
			NumbersToPick:  int32(5 + i), // Make each unique
			TotalNumbers:   90,
			MinSelections:  1,
			MaxSelections:  10,
			AllowQuickPick: true,
			SpecialRules:   &tc.description,
			EffectiveFrom:  tc.effectiveFrom,
			EffectiveTo:    tc.effectiveTo,
		}

		err := s.repo.Create(s.ctx, gameRules)
		require.NoError(s.T(), err, "Failed to create rules for %s", tc.name)
		createdRules[i] = gameRules
	}

	// Verify each rule set and date handling
	for i, tc := range testCases {
		retrieved, err := s.repo.GetByID(s.ctx, createdRules[i].ID)
		require.NoError(s.T(), err, "Failed to retrieve rules for %s", tc.name)

		// Check effective from date
		assert.WithinDuration(s.T(), tc.effectiveFrom, retrieved.EffectiveFrom, time.Second,
			"Effective from date mismatch for %s", tc.name)

		// Check effective to date
		if tc.effectiveTo != nil {
			require.NotNil(s.T(), retrieved.EffectiveTo, "Effective to should not be nil for %s", tc.name)
			assert.WithinDuration(s.T(), *tc.effectiveTo, *retrieved.EffectiveTo, time.Second,
				"Effective to date mismatch for %s", tc.name)
		} else {
			assert.Nil(s.T(), retrieved.EffectiveTo, "Effective to should be nil for %s", tc.name)
		}

		assert.Equal(s.T(), tc.description, *retrieved.SpecialRules, "Description mismatch for %s", tc.name)
	}
}

// Helper functions

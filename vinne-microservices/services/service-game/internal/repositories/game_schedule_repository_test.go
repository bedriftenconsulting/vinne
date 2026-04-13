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

type GameScheduleRepositoryTestSuite struct {
	suite.Suite
	db         *sql.DB
	repo       GameScheduleRepository
	gameRepo   GameRepository
	container  testcontainers.Container
	ctx        context.Context
	testGameID uuid.UUID
}

func TestGameScheduleRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(GameScheduleRepositoryTestSuite))
}

func (s *GameScheduleRepositoryTestSuite) SetupSuite() {
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
	s.repo = NewGameScheduleRepository(s.db)
	s.gameRepo = NewGameRepository(s.db)
}

func (s *GameScheduleRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(s.T(), err)
	}
}

func (s *GameScheduleRepositoryTestSuite) SetupTest() {
	// Clean up tables before each test
	_, err := s.db.Exec("TRUNCATE games, game_schedules RESTART IDENTITY CASCADE")
	require.NoError(s.T(), err)

	// Create a test game for foreign key relationships
	logoURL := "https://example.com/logo.png"
	brandColor := "#FF5733"
	testGame := &models.Game{
		Name:                "Test Game for Schedule",
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
		Code:                "TGFS001",
		GameFormat:          "5_by_90",
		GameCategory:        "national",
		LogoURL:             &logoURL,
		BrandColor:          &brandColor,
	}
	err = s.gameRepo.Create(s.ctx, testGame)
	require.NoError(s.T(), err)
	s.testGameID = testGame.ID
}

func (s *GameScheduleRepositoryTestSuite) runMigrations() error {
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

func (s *GameScheduleRepositoryTestSuite) TestCreate() {
	now := time.Now()
	scheduledStart := now.Add(time.Hour)
	scheduledEnd := now.Add(time.Hour * 23)  // 23 hours from now
	scheduledDraw := now.Add(time.Hour * 24) // 24 hours from now

	schedule := &models.GameSchedule{
		GameID:         s.testGameID,
		ScheduledStart: scheduledStart,
		ScheduledEnd:   scheduledEnd,
		ScheduledDraw:  scheduledDraw,
		Frequency:      models.DrawFrequencyDaily,
		IsActive:       true,
		Notes:          stringPtr("Daily draw schedule"),
	}

	err := s.repo.Create(s.ctx, schedule)
	require.NoError(s.T(), err)

	// Verify schedule was created
	assert.NotEqual(s.T(), uuid.Nil, schedule.ID)
	assert.NotZero(s.T(), schedule.CreatedAt)
	assert.NotZero(s.T(), schedule.UpdatedAt)

	// Verify schedule can be retrieved
	retrieved, err := s.repo.GetByID(s.ctx, schedule.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), schedule.GameID, retrieved.GameID)
	assert.Equal(s.T(), schedule.Frequency, retrieved.Frequency)
	assert.True(s.T(), retrieved.IsActive)
	assert.Equal(s.T(), *schedule.Notes, *retrieved.Notes)
	// Verify game_category is populated from JOIN with games table
	assert.NotNil(s.T(), retrieved.GameCategory)
	assert.Equal(s.T(), "national", *retrieved.GameCategory)
	// Verify logo_url and brand_color are populated from JOIN with games table
	assert.NotNil(s.T(), retrieved.LogoURL)
	assert.Equal(s.T(), "https://example.com/logo.png", *retrieved.LogoURL)
	assert.NotNil(s.T(), retrieved.BrandColor)
	assert.Equal(s.T(), "#FF5733", *retrieved.BrandColor)

	// Check timestamps are close (within 1 second due to potential precision differences)
	assert.WithinDuration(s.T(), scheduledStart, retrieved.ScheduledStart, time.Second)
	assert.WithinDuration(s.T(), scheduledEnd, retrieved.ScheduledEnd, time.Second)
	assert.WithinDuration(s.T(), scheduledDraw, retrieved.ScheduledDraw, time.Second)
}

func (s *GameScheduleRepositoryTestSuite) TestGetByID() {
	now := time.Now()
	schedule := &models.GameSchedule{
		GameID:         s.testGameID,
		ScheduledStart: now.Add(time.Hour),
		ScheduledEnd:   now.Add(time.Hour * 6),
		ScheduledDraw:  now.Add(time.Hour * 7),
		Frequency:      models.DrawFrequencyWeekly,
		IsActive:       true,
		Notes:          stringPtr("Weekly draw test"),
	}
	err := s.repo.Create(s.ctx, schedule)
	require.NoError(s.T(), err)

	// Test GetByID
	retrieved, err := s.repo.GetByID(s.ctx, schedule.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), schedule.ID, retrieved.ID)
	assert.Equal(s.T(), schedule.GameID, retrieved.GameID)
	assert.Equal(s.T(), models.DrawFrequencyWeekly, retrieved.Frequency)
	assert.Equal(s.T(), "Weekly draw test", *retrieved.Notes)
	// Verify game_category is populated
	assert.NotNil(s.T(), retrieved.GameCategory)
	assert.Equal(s.T(), "national", *retrieved.GameCategory)
	// Verify logo_url and brand_color are populated
	assert.NotNil(s.T(), retrieved.LogoURL)
	assert.Equal(s.T(), "https://example.com/logo.png", *retrieved.LogoURL)
	assert.NotNil(s.T(), retrieved.BrandColor)
	assert.Equal(s.T(), "#FF5733", *retrieved.BrandColor)

	// Test non-existent ID
	nonExistentID := uuid.New()
	_, err = s.repo.GetByID(s.ctx, nonExistentID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *GameScheduleRepositoryTestSuite) TestGetByGameID() {
	now := time.Now()

	// Create multiple schedules for the same game
	schedules := []*models.GameSchedule{
		{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(time.Hour),
			ScheduledEnd:   now.Add(time.Hour * 6),
			ScheduledDraw:  now.Add(time.Hour * 7),
			Frequency:      models.DrawFrequencyDaily,
			IsActive:       true,
			Notes:          stringPtr("First schedule"),
		},
		{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(time.Hour * 25), // Next day
			ScheduledEnd:   now.Add(time.Hour * 30),
			ScheduledDraw:  now.Add(time.Hour * 31),
			Frequency:      models.DrawFrequencyDaily,
			IsActive:       true,
			Notes:          stringPtr("Second schedule"),
		},
		{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(time.Hour * 49), // Day after
			ScheduledEnd:   now.Add(time.Hour * 54),
			ScheduledDraw:  now.Add(time.Hour * 55),
			Frequency:      models.DrawFrequencyDaily,
			IsActive:       false, // Inactive schedule
			Notes:          stringPtr("Third schedule - inactive"),
		},
	}

	for _, schedule := range schedules {
		err := s.repo.Create(s.ctx, schedule)
		require.NoError(s.T(), err)
	}

	// GetByGameID should return all schedules ordered by scheduled_start ASC
	retrieved, err := s.repo.GetByGameID(s.ctx, s.testGameID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), retrieved, 3)

	// Verify ordering
	assert.Equal(s.T(), "First schedule", *retrieved[0].Notes)
	assert.Equal(s.T(), "Second schedule", *retrieved[1].Notes)
	assert.Equal(s.T(), "Third schedule - inactive", *retrieved[2].Notes)
	assert.False(s.T(), retrieved[2].IsActive)

	// Verify all schedules have game_category populated
	for _, schedule := range retrieved {
		assert.NotNil(s.T(), schedule.GameCategory)
		assert.Equal(s.T(), "national", *schedule.GameCategory)
	}

	// Test non-existent game ID
	nonExistentGameID := uuid.New()
	emptyResult, err := s.repo.GetByGameID(s.ctx, nonExistentGameID)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), emptyResult)
}

func (s *GameScheduleRepositoryTestSuite) TestUpdate() {
	now := time.Now()
	schedule := &models.GameSchedule{
		GameID:         s.testGameID,
		ScheduledStart: now.Add(time.Hour),
		ScheduledEnd:   now.Add(time.Hour * 6),
		ScheduledDraw:  now.Add(time.Hour * 7),
		Frequency:      models.DrawFrequencyDaily,
		IsActive:       true,
		Notes:          stringPtr("Original schedule"),
	}
	err := s.repo.Create(s.ctx, schedule)
	require.NoError(s.T(), err)

	// Update schedule
	originalUpdatedAt := schedule.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure different timestamp

	schedule.ScheduledStart = now.Add(time.Hour * 2)
	schedule.ScheduledEnd = now.Add(time.Hour * 8)
	schedule.ScheduledDraw = now.Add(time.Hour * 9)
	schedule.Frequency = models.DrawFrequencyWeekly
	schedule.IsActive = false
	schedule.Notes = stringPtr("Updated schedule")

	err = s.repo.Update(s.ctx, schedule)
	require.NoError(s.T(), err)
	assert.True(s.T(), schedule.UpdatedAt.After(originalUpdatedAt))

	// Verify update
	updated, err := s.repo.GetByID(s.ctx, schedule.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.DrawFrequencyWeekly, updated.Frequency)
	assert.False(s.T(), updated.IsActive)
	assert.Equal(s.T(), "Updated schedule", *updated.Notes)
	assert.WithinDuration(s.T(), now.Add(time.Hour*2), updated.ScheduledStart, time.Second)
}

func (s *GameScheduleRepositoryTestSuite) TestDelete() {
	now := time.Now()
	schedule := &models.GameSchedule{
		GameID:         s.testGameID,
		ScheduledStart: now.Add(time.Hour),
		ScheduledEnd:   now.Add(time.Hour * 6),
		ScheduledDraw:  now.Add(time.Hour * 7),
		Frequency:      models.DrawFrequencyDaily,
		IsActive:       true,
	}
	err := s.repo.Create(s.ctx, schedule)
	require.NoError(s.T(), err)

	// Delete the schedule
	err = s.repo.Delete(s.ctx, schedule.ID)
	require.NoError(s.T(), err)

	// Verify schedule is deleted
	_, err = s.repo.GetByID(s.ctx, schedule.ID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")

	// Test deleting non-existent schedule
	nonExistentID := uuid.New()
	err = s.repo.Delete(s.ctx, nonExistentID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *GameScheduleRepositoryTestSuite) TestGetActiveSchedules() {
	now := time.Now()

	// Create schedules with different active states
	schedules := []*models.GameSchedule{
		{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(time.Hour),
			ScheduledEnd:   now.Add(time.Hour * 6),
			ScheduledDraw:  now.Add(time.Hour * 7),
			Frequency:      models.DrawFrequencyDaily,
			IsActive:       true,
			Notes:          stringPtr("Active schedule 1"),
		},
		{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(time.Hour * 25),
			ScheduledEnd:   now.Add(time.Hour * 30),
			ScheduledDraw:  now.Add(time.Hour * 31),
			Frequency:      models.DrawFrequencyDaily,
			IsActive:       false, // Inactive
			Notes:          stringPtr("Inactive schedule"),
		},
		{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(time.Hour * 49),
			ScheduledEnd:   now.Add(time.Hour * 54),
			ScheduledDraw:  now.Add(time.Hour * 55),
			Frequency:      models.DrawFrequencyWeekly,
			IsActive:       true,
			Notes:          stringPtr("Active schedule 2"),
		},
	}

	for _, schedule := range schedules {
		err := s.repo.Create(s.ctx, schedule)
		require.NoError(s.T(), err)
	}

	// GetActiveSchedules should return only active schedules
	activeSchedules, err := s.repo.GetActiveSchedules(s.ctx)
	require.NoError(s.T(), err)
	assert.Len(s.T(), activeSchedules, 2)

	// Verify all returned schedules are active
	for _, schedule := range activeSchedules {
		assert.True(s.T(), schedule.IsActive)
	}

	// Verify ordering by scheduled_start ASC
	assert.True(s.T(), activeSchedules[0].ScheduledStart.Before(activeSchedules[1].ScheduledStart) ||
		activeSchedules[0].ScheduledStart.Equal(activeSchedules[1].ScheduledStart))
}

func (s *GameScheduleRepositoryTestSuite) TestGetUpcomingSchedules() {
	now := time.Now()

	// Create schedules with different timing
	schedules := []*models.GameSchedule{
		{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(-time.Hour), // Past schedule
			ScheduledEnd:   now.Add(-time.Minute * 30),
			ScheduledDraw:  now.Add(-time.Minute * 20),
			Frequency:      models.DrawFrequencyDaily,
			IsActive:       true,
			Notes:          stringPtr("Past schedule"),
		},
		{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(time.Hour), // Future schedule 1
			ScheduledEnd:   now.Add(time.Hour * 6),
			ScheduledDraw:  now.Add(time.Hour * 7),
			Frequency:      models.DrawFrequencyDaily,
			IsActive:       true,
			Notes:          stringPtr("Upcoming schedule 1"),
		},
		{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(time.Hour * 25), // Future schedule 2
			ScheduledEnd:   now.Add(time.Hour * 30),
			ScheduledDraw:  now.Add(time.Hour * 31),
			Frequency:      models.DrawFrequencyDaily,
			IsActive:       true,
			Notes:          stringPtr("Upcoming schedule 2"),
		},
		{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(time.Hour * 49), // Future schedule 3
			ScheduledEnd:   now.Add(time.Hour * 54),
			ScheduledDraw:  now.Add(time.Hour * 55),
			Frequency:      models.DrawFrequencyWeekly,
			IsActive:       false, // Inactive future schedule
			Notes:          stringPtr("Inactive future schedule"),
		},
		{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(time.Hour * 73), // Future schedule 4
			ScheduledEnd:   now.Add(time.Hour * 78),
			ScheduledDraw:  now.Add(time.Hour * 79),
			Frequency:      models.DrawFrequencyWeekly,
			IsActive:       true,
			Notes:          stringPtr("Upcoming schedule 3"),
		},
	}

	for _, schedule := range schedules {
		err := s.repo.Create(s.ctx, schedule)
		require.NoError(s.T(), err)
	}

	// Test without limit (get all upcoming active schedules)
	upcomingSchedules, err := s.repo.GetUpcomingSchedules(s.ctx, 10)
	require.NoError(s.T(), err)
	assert.Len(s.T(), upcomingSchedules, 3) // Only active future schedules

	// Verify all schedules are in the future and active
	for _, schedule := range upcomingSchedules {
		assert.True(s.T(), schedule.ScheduledStart.After(now))
		assert.True(s.T(), schedule.IsActive)
	}

	// Verify ordering by scheduled_start ASC
	for i := 1; i < len(upcomingSchedules); i++ {
		assert.True(s.T(), upcomingSchedules[i-1].ScheduledStart.Before(upcomingSchedules[i].ScheduledStart) ||
			upcomingSchedules[i-1].ScheduledStart.Equal(upcomingSchedules[i].ScheduledStart))
	}

	// Test with limit
	limitedSchedules, err := s.repo.GetUpcomingSchedules(s.ctx, 2)
	require.NoError(s.T(), err)
	assert.Len(s.T(), limitedSchedules, 2)

	// Verify we got the earliest upcoming schedules
	assert.Equal(s.T(), "Upcoming schedule 1", *limitedSchedules[0].Notes)
	assert.Equal(s.T(), "Upcoming schedule 2", *limitedSchedules[1].Notes)
}

func (s *GameScheduleRepositoryTestSuite) TestDifferentFrequencies() {
	now := time.Now()

	// Test all supported frequency types
	frequencies := []models.DrawFrequency{
		models.DrawFrequencyDaily,
		models.DrawFrequencyWeekly,
		models.DrawFrequencyBiWeekly,
		models.DrawFrequencyMonthly,
		models.DrawFrequencySpecial,
	}

	createdSchedules := make([]*models.GameSchedule, len(frequencies))

	for i, frequency := range frequencies {
		schedule := &models.GameSchedule{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(time.Duration(i+1) * time.Hour),
			ScheduledEnd:   now.Add(time.Duration(i+7) * time.Hour),
			ScheduledDraw:  now.Add(time.Duration(i+8) * time.Hour),
			Frequency:      frequency,
			IsActive:       true,
			Notes:          stringPtr(fmt.Sprintf("Schedule with %s frequency", frequency)),
		}

		err := s.repo.Create(s.ctx, schedule)
		require.NoError(s.T(), err, "Failed to create schedule with frequency %s", frequency)
		createdSchedules[i] = schedule
	}

	// Verify each frequency was stored correctly
	for i, expectedFreq := range frequencies {
		retrieved, err := s.repo.GetByID(s.ctx, createdSchedules[i].ID)
		require.NoError(s.T(), err, "Failed to retrieve schedule with frequency %s", expectedFreq)
		assert.Equal(s.T(), expectedFreq, retrieved.Frequency, "Frequency mismatch for %s", expectedFreq)
	}

	// Verify all schedules are returned when getting by game ID
	allSchedules, err := s.repo.GetByGameID(s.ctx, s.testGameID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), allSchedules, len(frequencies))
}

func (s *GameScheduleRepositoryTestSuite) TestScheduleTimeValidation() {
	now := time.Now()

	// Test various time scenarios
	testCases := []struct {
		name           string
		scheduledStart time.Time
		scheduledEnd   time.Time
		scheduledDraw  time.Time
		shouldPass     bool
		description    string
	}{
		{
			name:           "Valid Order",
			scheduledStart: now.Add(time.Hour),
			scheduledEnd:   now.Add(time.Hour * 6),
			scheduledDraw:  now.Add(time.Hour * 7),
			shouldPass:     true,
			description:    "Normal valid schedule order",
		},
		{
			name:           "Same Start and End",
			scheduledStart: now.Add(time.Hour),
			scheduledEnd:   now.Add(time.Hour),
			scheduledDraw:  now.Add(time.Hour * 2),
			shouldPass:     true,
			description:    "Start and end at same time",
		},
		{
			name:           "Past Schedule",
			scheduledStart: now.Add(-time.Hour * 24),
			scheduledEnd:   now.Add(-time.Hour * 18),
			scheduledDraw:  now.Add(-time.Hour * 17),
			shouldPass:     true,
			description:    "Historical schedule",
		},
		{
			name:           "Very Close Times",
			scheduledStart: now.Add(time.Minute),
			scheduledEnd:   now.Add(time.Minute * 2),
			scheduledDraw:  now.Add(time.Minute * 3),
			shouldPass:     true,
			description:    "Schedule with minute intervals",
		},
	}

	for _, tc := range testCases {
		schedule := &models.GameSchedule{
			GameID:         s.testGameID,
			ScheduledStart: tc.scheduledStart,
			ScheduledEnd:   tc.scheduledEnd,
			ScheduledDraw:  tc.scheduledDraw,
			Frequency:      models.DrawFrequencyDaily,
			IsActive:       true,
			Notes:          &tc.description,
		}

		err := s.repo.Create(s.ctx, schedule)
		if tc.shouldPass {
			require.NoError(s.T(), err, "Test case '%s' should have passed", tc.name)

			// Verify the schedule was created correctly
			retrieved, err := s.repo.GetByID(s.ctx, schedule.ID)
			require.NoError(s.T(), err, "Failed to retrieve schedule for test case '%s'", tc.name)

			assert.WithinDuration(s.T(), tc.scheduledStart, retrieved.ScheduledStart, time.Second,
				"ScheduledStart mismatch for test case '%s'", tc.name)
			assert.WithinDuration(s.T(), tc.scheduledEnd, retrieved.ScheduledEnd, time.Second,
				"ScheduledEnd mismatch for test case '%s'", tc.name)
			assert.WithinDuration(s.T(), tc.scheduledDraw, retrieved.ScheduledDraw, time.Second,
				"ScheduledDraw mismatch for test case '%s'", tc.name)
			assert.Equal(s.T(), tc.description, *retrieved.Notes)
		} else {
			require.Error(s.T(), err, "Test case '%s' should have failed", tc.name)
		}

		// Clean up for next test case
		if tc.shouldPass {
			_ = s.repo.Delete(s.ctx, schedule.ID)
		}
	}
}

func (s *GameScheduleRepositoryTestSuite) TestScheduleWithoutNotes() {
	// Test creating schedule without optional notes field
	now := time.Now()
	schedule := &models.GameSchedule{
		GameID:         s.testGameID,
		ScheduledStart: now.Add(time.Hour),
		ScheduledEnd:   now.Add(time.Hour * 6),
		ScheduledDraw:  now.Add(time.Hour * 7),
		Frequency:      models.DrawFrequencyDaily,
		IsActive:       true,
		// Notes intentionally nil
	}

	err := s.repo.Create(s.ctx, schedule)
	require.NoError(s.T(), err)

	retrieved, err := s.repo.GetByID(s.ctx, schedule.ID)
	require.NoError(s.T(), err)
	assert.Nil(s.T(), retrieved.Notes)
	assert.True(s.T(), retrieved.IsActive) // Should default to true
}

func (s *GameScheduleRepositoryTestSuite) TestMultipleGamesSchedules() {
	// Create another game
	testGame2 := &models.Game{
		Name:                "Test Game 2 for Schedule",
		Type:                "6_BY_49",
		Organizer:           "ORGANIZER_RAND_LOTTERY",
		MinStakeAmount:      100,
		MaxStakeAmount:      10000,
		MaxTicketsPerPlayer: 5,
		Status:              "DRAFT",
		Version:             "1.0.0",
		Code:                "TG2S001",
		BasePrice:           2.00,
		NumberRangeMin:      1,
		NumberRangeMax:      49,
		SelectionCount:      6,
		DrawFrequency:       "weekly",
		DrawDays:            []string{"MON", "WED", "FRI"},
		SalesCutoffMinutes:  60,
	}
	err := s.gameRepo.Create(s.ctx, testGame2)
	require.NoError(s.T(), err)

	now := time.Now()

	// Create schedules for both games
	schedules := []*models.GameSchedule{
		{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(time.Hour),
			ScheduledEnd:   now.Add(time.Hour * 6),
			ScheduledDraw:  now.Add(time.Hour * 7),
			Frequency:      models.DrawFrequencyDaily,
			IsActive:       true,
			Notes:          stringPtr("Game 1 Schedule 1"),
		},
		{
			GameID:         testGame2.ID,
			ScheduledStart: now.Add(time.Hour * 2),
			ScheduledEnd:   now.Add(time.Hour * 8),
			ScheduledDraw:  now.Add(time.Hour * 9),
			Frequency:      models.DrawFrequencyWeekly,
			IsActive:       true,
			Notes:          stringPtr("Game 2 Schedule 1"),
		},
		{
			GameID:         s.testGameID,
			ScheduledStart: now.Add(time.Hour * 25),
			ScheduledEnd:   now.Add(time.Hour * 31),
			ScheduledDraw:  now.Add(time.Hour * 32),
			Frequency:      models.DrawFrequencyDaily,
			IsActive:       true,
			Notes:          stringPtr("Game 1 Schedule 2"),
		},
	}

	for _, schedule := range schedules {
		err := s.repo.Create(s.ctx, schedule)
		require.NoError(s.T(), err)
	}

	// Verify schedules are properly separated by game
	game1Schedules, err := s.repo.GetByGameID(s.ctx, s.testGameID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), game1Schedules, 2)

	game2Schedules, err := s.repo.GetByGameID(s.ctx, testGame2.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), game2Schedules, 1)
	assert.Equal(s.T(), "Game 2 Schedule 1", *game2Schedules[0].Notes)

	// Verify GetActiveSchedules returns schedules from all games
	allActiveSchedules, err := s.repo.GetActiveSchedules(s.ctx)
	require.NoError(s.T(), err)
	assert.Len(s.T(), allActiveSchedules, 3)

	// Verify upcoming schedules includes all games
	upcomingSchedules, err := s.repo.GetUpcomingSchedules(s.ctx, 10)
	require.NoError(s.T(), err)
	assert.Len(s.T(), upcomingSchedules, 3) // All are future schedules
}

// Helper functions

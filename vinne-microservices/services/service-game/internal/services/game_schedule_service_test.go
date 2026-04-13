package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"github.com/randco/randco-microservices/services/service-game/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type GameScheduleServiceTestSuite struct {
	suite.Suite
	db              *sql.DB
	scheduleService GameScheduleService
	scheduleRepo    repositories.GameScheduleRepository
	gameRepo        repositories.GameRepository
	container       testcontainers.Container
	ctx             context.Context
	testGameID      uuid.UUID
	futureStart     time.Time
	futureEnd       time.Time
	futureDraw      time.Time
}

func TestGameScheduleServiceTestSuite(t *testing.T) {
	suite.Run(t, new(GameScheduleServiceTestSuite))
}

func (s *GameScheduleServiceTestSuite) SetupSuite() {
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

	// Run migrations using test helper
	testHelper := repositories.NewTestHelper()
	err = testHelper.SetupTestDB(s.T(), s.db)
	require.NoError(s.T(), err)

	// Initialize repositories
	s.scheduleRepo = repositories.NewGameScheduleRepository(s.db)
	s.gameRepo = repositories.NewGameRepository(s.db)

	// Initialize service
	s.scheduleService = NewGameScheduleService(s.scheduleRepo, s.gameRepo)

	// Generate test UUID and times
	s.testGameID = uuid.New()
	now := time.Now()
	s.futureStart = now.Add(1 * time.Hour)
	s.futureEnd = now.Add(25 * time.Hour)
	s.futureDraw = now.Add(26 * time.Hour)
}

func (s *GameScheduleServiceTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(s.ctx)
	}
}

func (s *GameScheduleServiceTestSuite) SetupTest() {
	// Clean up test data before each test using test helper
	testHelper := repositories.NewTestHelper()
	err := testHelper.CleanupTestData(s.T(), s.db)
	require.NoError(s.T(), err)
	// Create a test game for schedule tests
	s.createTestGame()
}

func (s *GameScheduleServiceTestSuite) TestScheduleGame() {
	// Test successful scheduling
	frequency := "WEEKLY"
	schedule, err := s.scheduleService.ScheduleGame(s.ctx, s.testGameID, s.futureStart, s.futureEnd, s.futureDraw, frequency)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), schedule)
	assert.Equal(s.T(), s.testGameID, schedule.GameID)
	assert.Equal(s.T(), models.DrawFrequency(frequency), schedule.Frequency)
	assert.True(s.T(), schedule.IsActive)
	assert.Equal(s.T(), s.futureStart, schedule.ScheduledStart)
	assert.Equal(s.T(), s.futureEnd, schedule.ScheduledEnd)
	assert.Equal(s.T(), s.futureDraw, schedule.ScheduledDraw)
	// Verify game_category is populated from the game
	assert.NotNil(s.T(), schedule.GameCategory)
	assert.Equal(s.T(), "national", *schedule.GameCategory)
	// Verify logo_url and brand_color are populated from the game
	assert.NotNil(s.T(), schedule.LogoURL)
	assert.Equal(s.T(), "https://example.com/logo.png", *schedule.LogoURL)
	assert.NotNil(s.T(), schedule.BrandColor)
	assert.Equal(s.T(), "#FF5733", *schedule.BrandColor)
}

func (s *GameScheduleServiceTestSuite) TestScheduleGame_InvalidGame() {
	// Test scheduling with invalid game ID
	invalidGameID := uuid.New()
	_, err := s.scheduleService.ScheduleGame(s.ctx, invalidGameID, s.futureStart, s.futureEnd, s.futureDraw, "DAILY")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "game not found")
}

func (s *GameScheduleServiceTestSuite) TestScheduleGame_InvalidStatus() {
	// Create game with DRAFT status (invalid for scheduling)
	draftGameID := uuid.New()
	drawDaysJSON := `["MONDAY", "WEDNESDAY", "FRIDAY"]`
	query := `INSERT INTO games (
		id, code, name, status, type, game_type, game_format, game_category,
		organizer, min_stake_amount, max_stake_amount, max_tickets_per_player,
		draw_frequency, draw_days, number_range_min, number_range_max,
		selection_count, sales_cutoff_minutes, base_price, version
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`
	_, err := s.db.ExecContext(s.ctx, query,
		draftGameID, "DRAFT-TEST", "Draft Game", "DRAFT", "5_90", "5_90",
		"5_by_90", "national", "NLA", 1.0, 100.0, 10,
		"weekly", drawDaysJSON, 1, 90, 5, 30, 1.0, "1.0")
	require.NoError(s.T(), err)

	_, err = s.scheduleService.ScheduleGame(s.ctx, draftGameID, s.futureStart, s.futureEnd, s.futureDraw, "WEEKLY")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "game must be approved or active to schedule")
}

func (s *GameScheduleServiceTestSuite) TestScheduleGame_InvalidTimes() {
	// Test invalid schedule times (start after end)
	invalidStart := s.futureEnd.Add(1 * time.Hour)
	_, err := s.scheduleService.ScheduleGame(s.ctx, s.testGameID, invalidStart, s.futureEnd, s.futureDraw, "DAILY")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "start time must be before end time")
}

func (s *GameScheduleServiceTestSuite) TestGetGameSchedule() {
	// First create a schedule
	frequency := "MONTHLY"
	created, err := s.scheduleService.ScheduleGame(s.ctx, s.testGameID, s.futureStart, s.futureEnd, s.futureDraw, frequency)
	require.NoError(s.T(), err)

	// Test retrieval
	schedules, err := s.scheduleService.GetGameSchedule(s.ctx, s.testGameID)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), schedules, 1)
	assert.Equal(s.T(), created.ID, schedules[0].ID)
	assert.Equal(s.T(), s.testGameID, schedules[0].GameID)
	assert.Equal(s.T(), models.DrawFrequency(frequency), schedules[0].Frequency)
	// Verify game_category is populated
	assert.NotNil(s.T(), schedules[0].GameCategory)
	assert.Equal(s.T(), "national", *schedules[0].GameCategory)
	// Verify logo_url and brand_color are populated
	assert.NotNil(s.T(), schedules[0].LogoURL)
	assert.Equal(s.T(), "https://example.com/logo.png", *schedules[0].LogoURL)
	assert.NotNil(s.T(), schedules[0].BrandColor)
	assert.Equal(s.T(), "#FF5733", *schedules[0].BrandColor)
}

func (s *GameScheduleServiceTestSuite) TestGetGameSchedule_NotFound() {
	// Test retrieval with non-existent game
	invalidGameID := uuid.New()
	schedules, err := s.scheduleService.GetGameSchedule(s.ctx, invalidGameID)
	assert.NoError(s.T(), err)
	assert.Empty(s.T(), schedules)
}

func (s *GameScheduleServiceTestSuite) TestUpdateSchedule() {
	// First create a schedule
	created, err := s.scheduleService.ScheduleGame(s.ctx, s.testGameID, s.futureStart, s.futureEnd, s.futureDraw, "WEEKLY")
	require.NoError(s.T(), err)

	// Test update
	newStart := s.futureStart.Add(2 * time.Hour)
	newEnd := s.futureEnd.Add(2 * time.Hour)
	newDraw := s.futureDraw.Add(2 * time.Hour)
	created.ScheduledStart = newStart
	created.ScheduledEnd = newEnd
	created.ScheduledDraw = newDraw
	created.Frequency = "DAILY"

	err = s.scheduleService.UpdateSchedule(s.ctx, created)
	assert.NoError(s.T(), err)

	// Verify update
	schedules, err := s.scheduleService.GetGameSchedule(s.ctx, s.testGameID)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), schedules, 1)
	assert.Equal(s.T(), newStart.UTC(), schedules[0].ScheduledStart.UTC())
	assert.Equal(s.T(), newEnd.UTC(), schedules[0].ScheduledEnd.UTC())
	assert.Equal(s.T(), newDraw.UTC(), schedules[0].ScheduledDraw.UTC())
	assert.Equal(s.T(), models.DrawFrequency("DAILY"), schedules[0].Frequency)
}

func (s *GameScheduleServiceTestSuite) TestUpdateSchedule_TerminatedGame() {
	// Create schedule first
	created, err := s.scheduleService.ScheduleGame(s.ctx, s.testGameID, s.futureStart, s.futureEnd, s.futureDraw, "WEEKLY")
	require.NoError(s.T(), err)

	// Update game status to TERMINATED
	_, err = s.db.ExecContext(s.ctx, "UPDATE games SET status = 'TERMINATED' WHERE id = $1", s.testGameID)
	require.NoError(s.T(), err)

	// Test update fails for terminated game
	created.Frequency = "DAILY"
	err = s.scheduleService.UpdateSchedule(s.ctx, created)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "cannot update schedule for terminated game")
}

func (s *GameScheduleServiceTestSuite) TestCancelSchedule() {
	// Create a schedule
	created, err := s.scheduleService.ScheduleGame(s.ctx, s.testGameID, s.futureStart, s.futureEnd, s.futureDraw, "WEEKLY")
	require.NoError(s.T(), err)

	// Test cancellation
	err = s.scheduleService.CancelSchedule(s.ctx, created.ID)
	assert.NoError(s.T(), err)

	// Verify schedule is inactive
	schedules, err := s.scheduleService.GetGameSchedule(s.ctx, s.testGameID)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), schedules, 1)
	assert.False(s.T(), schedules[0].IsActive)
}

func (s *GameScheduleServiceTestSuite) TestCancelSchedule_AlreadyStarted() {
	// Create a schedule that has already started
	pastStart := time.Now().Add(-1 * time.Hour)
	pastEnd := time.Now().Add(23 * time.Hour)
	pastDraw := time.Now().Add(24 * time.Hour)

	// We need to create this schedule directly in the database since the service validates future times
	scheduleID := uuid.New()
	query := `INSERT INTO game_schedules (id, game_id, scheduled_start, scheduled_end, scheduled_draw, frequency, is_active) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.db.ExecContext(s.ctx, query, scheduleID, s.testGameID, pastStart, pastEnd, pastDraw, "WEEKLY", true)
	require.NoError(s.T(), err)

	// Test cancellation fails for already started schedule
	err = s.scheduleService.CancelSchedule(s.ctx, scheduleID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "cannot cancel schedule that has already started")
}

func (s *GameScheduleServiceTestSuite) TestGetActiveSchedules() {
	// Create multiple schedules with different statuses
	schedule1, err := s.scheduleService.ScheduleGame(s.ctx, s.testGameID, s.futureStart, s.futureEnd, s.futureDraw, "WEEKLY")
	require.NoError(s.T(), err)

	// Create second game and schedule
	gameID2 := uuid.New()
	s.createTestGameWithID(gameID2)
	schedule2, err := s.scheduleService.ScheduleGame(s.ctx, gameID2, s.futureStart, s.futureEnd, s.futureDraw, "DAILY")
	require.NoError(s.T(), err)

	// Cancel one schedule
	err = s.scheduleService.CancelSchedule(s.ctx, schedule2.ID)
	require.NoError(s.T(), err)

	// Get active schedules
	activeSchedules, err := s.scheduleService.GetActiveSchedules(s.ctx)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), activeSchedules, 1)
	assert.Equal(s.T(), schedule1.ID, activeSchedules[0].ID)
}

func (s *GameScheduleServiceTestSuite) TestGetUpcomingSchedules() {
	// Create schedules with different start times
	nearFuture := time.Now().Add(30 * time.Minute)
	farFuture := time.Now().Add(48 * time.Hour)

	// Create schedules in the database directly for more control over timing
	scheduleID1 := uuid.New()
	scheduleID2 := uuid.New()

	gameID2 := uuid.New()
	s.createTestGameWithID(gameID2)

	query := `INSERT INTO game_schedules (id, game_id, scheduled_start, scheduled_end, scheduled_draw, frequency, is_active) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7)`

	// Near future schedule
	_, err := s.db.ExecContext(s.ctx, query, scheduleID1, s.testGameID,
		nearFuture, nearFuture.Add(24*time.Hour), nearFuture.Add(25*time.Hour), "DAILY", true)
	require.NoError(s.T(), err)

	// Far future schedule
	_, err = s.db.ExecContext(s.ctx, query, scheduleID2, gameID2,
		farFuture, farFuture.Add(24*time.Hour), farFuture.Add(25*time.Hour), "WEEKLY", true)
	require.NoError(s.T(), err)

	// Get upcoming schedules with limit
	upcomingSchedules, err := s.scheduleService.GetUpcomingSchedules(s.ctx, 1)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), upcomingSchedules, 1)
	// Should get the nearer schedule first
	assert.Equal(s.T(), scheduleID1, upcomingSchedules[0].ID)
}

func (s *GameScheduleServiceTestSuite) TestOverrideUnplayedSchedules() {
	// Test that only SCHEDULED status games are overridden, not COMPLETED ones

	// Create test game
	gameID := uuid.New()
	s.createTestGameWithID(gameID)

	// Get week boundaries
	weekStart := time.Now().Truncate(24 * time.Hour)
	for weekStart.Weekday() != time.Monday {
		weekStart = weekStart.AddDate(0, 0, -1)
	}
	weekEnd := weekStart.AddDate(0, 0, 7)

	// Create existing schedules with different statuses
	scheduledID := uuid.New()
	completedID := uuid.New()
	inProgressID := uuid.New()
	gameName := "Test Game for Override"

	// Insert SCHEDULED status schedule (should be deleted)
	query := `INSERT INTO game_schedules (
		id, game_id, game_name, scheduled_start, scheduled_end, scheduled_draw,
		frequency, is_active, status
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := s.db.ExecContext(s.ctx, query,
		scheduledID, gameID, gameName,
		weekStart.Add(2*time.Hour), weekStart.Add(3*time.Hour), weekStart.Add(3*time.Hour),
		"DAILY", true, "SCHEDULED")
	require.NoError(s.T(), err)

	// Insert COMPLETED status schedule (should NOT be deleted)
	_, err = s.db.ExecContext(s.ctx, query,
		completedID, gameID, gameName,
		weekStart.Add(26*time.Hour), weekStart.Add(27*time.Hour), weekStart.Add(27*time.Hour),
		"DAILY", true, "COMPLETED")
	require.NoError(s.T(), err)

	// Insert IN_PROGRESS status schedule (should NOT be deleted)
	_, err = s.db.ExecContext(s.ctx, query,
		inProgressID, gameID, gameName,
		weekStart.Add(50*time.Hour), weekStart.Add(51*time.Hour), weekStart.Add(51*time.Hour),
		"DAILY", true, "IN_PROGRESS")
	require.NoError(s.T(), err)

	// Delete unplayed schedules in time range
	err = s.scheduleRepo.DeleteUnplayedSchedulesInTimeRange(s.ctx, weekStart, weekEnd)
	require.NoError(s.T(), err)

	// Verify SCHEDULED was deleted
	_, err = s.scheduleRepo.GetByID(s.ctx, scheduledID)
	assert.Error(s.T(), err, "SCHEDULED status schedule should be deleted")

	// Verify COMPLETED still exists
	completed, err := s.scheduleRepo.GetByID(s.ctx, completedID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), completed, "COMPLETED status schedule should remain")
	assert.Equal(s.T(), models.ScheduleStatus("COMPLETED"), completed.Status)

	// Verify IN_PROGRESS still exists
	inProgress, err := s.scheduleRepo.GetByID(s.ctx, inProgressID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), inProgress, "IN_PROGRESS status schedule should remain")
	assert.Equal(s.T(), models.ScheduleStatus("IN_PROGRESS"), inProgress.Status)
}

func (s *GameScheduleServiceTestSuite) TestGameNamePopulationInSchedules() {
	// Test that game names are properly populated in schedules

	// Create games with distinct names
	game1ID := uuid.New()
	game2ID := uuid.New()

	// Create first game
	s.createTestGameWithNameAndID(game1ID, "Morning Lottery Draw")

	// Create second game
	s.createTestGameWithNameAndID(game2ID, "Evening Jackpot Special")

	// Schedule games
	schedule1, err := s.scheduleService.ScheduleGame(s.ctx, game1ID,
		s.futureStart, s.futureEnd, s.futureDraw, "DAILY")
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), schedule1.GameName)
	assert.Equal(s.T(), "Morning Lottery Draw", *schedule1.GameName)

	schedule2, err := s.scheduleService.ScheduleGame(s.ctx, game2ID,
		s.futureStart.Add(4*time.Hour), s.futureEnd.Add(4*time.Hour),
		s.futureDraw.Add(4*time.Hour), "WEEKLY")
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), schedule2.GameName)
	assert.Equal(s.T(), "Evening Jackpot Special", *schedule2.GameName)

	// Verify through GetByGameID
	schedules1, err := s.scheduleRepo.GetByGameID(s.ctx, game1ID)
	require.NoError(s.T(), err)
	for _, schedule := range schedules1 {
		assert.NotNil(s.T(), schedule.GameName)
		assert.Equal(s.T(), "Morning Lottery Draw", *schedule.GameName)
	}

	schedules2, err := s.scheduleRepo.GetByGameID(s.ctx, game2ID)
	require.NoError(s.T(), err)
	for _, schedule := range schedules2 {
		assert.NotNil(s.T(), schedule.GameName)
		assert.Equal(s.T(), "Evening Jackpot Special", *schedule.GameName)
	}
}

func (s *GameScheduleServiceTestSuite) TestScheduleStatusTransitions() {
	// Test status transitions and their impact on schedule deletion

	gameID := uuid.New()
	s.createTestGameWithID(gameID)

	// Create a schedule with SCHEDULED status
	schedule, err := s.scheduleService.ScheduleGame(s.ctx, gameID,
		s.futureStart, s.futureEnd, s.futureDraw, "DAILY")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.ScheduleStatusScheduled, schedule.Status)

	// Update to IN_PROGRESS
	schedule.Status = models.ScheduleStatusInProgress
	err = s.scheduleRepo.Update(s.ctx, schedule)
	require.NoError(s.T(), err)

	// Verify status change
	updated, err := s.scheduleRepo.GetByID(s.ctx, schedule.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.ScheduleStatusInProgress, updated.Status)

	// Try to delete unplayed schedules (should not delete IN_PROGRESS)
	err = s.scheduleRepo.DeleteUnplayedSchedulesInTimeRange(s.ctx,
		time.Now().Add(-1*time.Hour), time.Now().Add(48*time.Hour))
	require.NoError(s.T(), err)

	// Verify schedule still exists
	stillExists, err := s.scheduleRepo.GetByID(s.ctx, schedule.ID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), stillExists)
	assert.Equal(s.T(), models.ScheduleStatusInProgress, stillExists.Status)

	// Update to COMPLETED
	schedule.Status = models.ScheduleStatusCompleted
	drawResultID := uuid.New()
	schedule.DrawResultID = &drawResultID
	err = s.scheduleRepo.Update(s.ctx, schedule)
	require.NoError(s.T(), err)

	// Verify COMPLETED status is preserved
	completed, err := s.scheduleRepo.GetByID(s.ctx, schedule.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.ScheduleStatusCompleted, completed.Status)
	assert.NotNil(s.T(), completed.DrawResultID)
}

// Helper methods

func (s *GameScheduleServiceTestSuite) createTestGame() {
	s.createTestGameWithID(s.testGameID)
}

func (s *GameScheduleServiceTestSuite) createTestGameWithID(gameID uuid.UUID) {
	s.createTestGameWithNameAndID(gameID, "Test Game for Schedule")
}

func (s *GameScheduleServiceTestSuite) createTestGameWithNameAndID(gameID uuid.UUID, name string) {
	drawDaysJSON := `["WEDNESDAY", "SATURDAY"]`
	query := `INSERT INTO games (
		id, code, name, type, game_type, game_format, game_category, organizer,
		min_stake_amount, max_stake_amount, max_tickets_per_player, draw_frequency,
		draw_days, weekly_schedule, number_range_min, number_range_max, selection_count,
		sales_cutoff_minutes, base_price, multi_draw_enabled, max_draws_advance,
		version, status, description, logo_url, brand_color
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26)`
	_, err := s.db.ExecContext(s.ctx, query,
		gameID,
		"SCHEDULE-"+gameID.String()[:8],          // code
		name,                                     // use provided name
		"5_90",                                   // type
		"5_90",                                   // game_type
		"5_by_90",                                // game_format
		"national",                               // game_category (valid: national, private, special)
		"NLA",                                    // organizer
		1.0,                                      // min_stake_amount
		100.0,                                    // max_stake_amount
		10,                                       // max_tickets_per_player
		"weekly",                                 // draw_frequency
		drawDaysJSON,                             // draw_days (JSON)
		false,                                    // weekly_schedule
		1,                                        // number_range_min
		90,                                       // number_range_max
		5,                                        // selection_count
		30,                                       // sales_cutoff_minutes
		1.0,                                      // base_price
		false,                                    // multi_draw_enabled
		nil,                                      // max_draws_advance
		"1.0",                                    // version
		"APPROVED",                               // status
		"Test game for schedule service testing", // description
		"https://example.com/logo.png",           // logo_url
		"#FF5733")                                // brand_color
	require.NoError(s.T(), err)
}

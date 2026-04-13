package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/randco/randco-microservices/services/service-game/internal/config"
	"github.com/randco/randco-microservices/services/service-game/internal/grpc/clients"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"github.com/randco/randco-microservices/services/service-game/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestSchedulerService_CheckSalesCutoffs(t *testing.T) {
	ctx := context.Background()

	// Setup test database with testcontainers
	postgresContainer, db := setupTestDatabase(t, ctx)
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	// Run migrations
	if err := runMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create repositories
	scheduleRepo := repositories.NewGameScheduleRepository(db)
	gameRepo := repositories.NewGameRepository(db)

	// Create test game
	game := &models.Game{
		ID:                  uuid.New(),
		Code:                "TEST-GAME",
		Name:                "Test Game",
		Type:                "5_BY_90",
		Organizer:           "ORGANIZER_RAND_LOTTERY",
		GameCategory:        "private",
		GameFormat:          "5_by_90",
		Status:              "ACTIVE",
		Version:             "1.0.0",
		BasePrice:           1.00,
		MinStakeAmount:      0.50,
		MaxStakeAmount:      50.00,
		MaxTicketsPerPlayer: 10,
		NumberRangeMin:      1,
		NumberRangeMax:      90,
		SelectionCount:      5,
		DrawFrequency:       "daily",
		SalesCutoffMinutes:  30,
	}
	err := gameRepo.Create(ctx, game)
	require.NoError(t, err)

	// Create test schedule that should trigger in 30 seconds
	now := time.Now()
	schedule := &models.GameSchedule{
		GameID:         game.ID,
		GameName:       &game.Name,
		ScheduledStart: now.Add(-1 * time.Hour),
		ScheduledEnd:   now.Add(30 * time.Second), // 30 seconds from now
		ScheduledDraw:  now.Add(1 * time.Hour),
		Frequency:      models.DrawFrequencyDaily,
		IsActive:       true,
		Status:         models.ScheduleStatusScheduled,
	}
	err = scheduleRepo.Create(ctx, schedule)
	require.NoError(t, err)

	// Create mock draw client
	mockDrawClient := &clients.MockDrawServiceClient{
		CreateDrawFunc: func(ctx context.Context, game *models.Game, schedule *models.GameSchedule, ticketClient clients.TicketServiceClient) (uuid.UUID, error) {
			return uuid.New(), nil
		},
	}

	// Create scheduler service
	logger := log.New(os.Stdout, "[test-scheduler] ", log.LstdFlags)
	schedulerService, err := NewSchedulerService(
		config.SchedulerConfig{
			Enabled:       true,
			Interval:      1 * time.Minute,
			WindowMinutes: 2,
			Timezone:      "Africa/Accra",
		},
		scheduleRepo,
		gameRepo,
		mockDrawClient,
		nil,                          // ticket client not needed for this test
		nil,                          // event bus not needed for this test
		nil,                          // notification client not needed for this test
		nil,                          // admin client not needed for this test
		[]string{"test@example.com"}, // fallback emails
		logger,
	)
	require.NoError(t, err)

	// Check sales cutoffs - should not trigger yet (schedule is 30 seconds away)
	schedulerService.checkSalesCutoffs(ctx)

	// Verify schedule is still SCHEDULED
	updated, err := scheduleRepo.GetByID(ctx, schedule.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ScheduleStatusScheduled, updated.Status, "Schedule should still be SCHEDULED")

	// Update schedule to have cutoff time within the window
	// Query looks for scheduled_end >= NOW and <= NOW+2min
	// Set it to NOW + 500ms to be within window, and by the time IsTimeReached checks, it will have passed
	updateTime := time.Now()
	schedule.ScheduledEnd = updateTime.Add(500 * time.Millisecond)
	err = scheduleRepo.Update(ctx, schedule)
	require.NoError(t, err)

	t.Logf("Schedule updated: scheduled_end=%v", schedule.ScheduledEnd)

	// Verify the update worked
	verifySchedule, err := scheduleRepo.GetByID(ctx, schedule.ID)
	require.NoError(t, err)
	t.Logf("Schedule in DB after update: scheduled_end=%v, status=%s", verifySchedule.ScheduledEnd, verifySchedule.Status)

	// Sleep briefly to ensure the time has passed
	time.Sleep(600 * time.Millisecond)

	checkTime := time.Now()
	t.Logf("Checking at: %v (scheduled_end was %v)", checkTime, schedule.ScheduledEnd)
	t.Logf("Time difference: %v", checkTime.Sub(schedule.ScheduledEnd))

	// Manually verify query would find this schedule
	foundSchedules, err := scheduleRepo.GetSchedulesDueForProcessing(ctx, "sales_cutoff", 2)
	require.NoError(t, err)
	t.Logf("Found %d schedules due for processing", len(foundSchedules))
	for i, s := range foundSchedules {
		t.Logf("  Schedule %d: ID=%s, scheduled_end=%v, status=%s", i, s.ID, s.ScheduledEnd, s.Status)
	}

	// Check if time is reached using scheduler's method
	timeReached := schedulerService.IsTimeReached(verifySchedule.ScheduledEnd)
	t.Logf("Is time reached? %v", timeReached)

	// Check again - should trigger now since cutoff time has passed
	schedulerService.checkSalesCutoffs(ctx)

	// Verify schedule status was updated to IN_PROGRESS
	updated, err = scheduleRepo.GetByID(ctx, schedule.ID)
	require.NoError(t, err)
	t.Logf("Schedule status after check: %s", updated.Status)
	t.Logf("Schedule updated_at: %v", updated.UpdatedAt)

	assert.Equal(t, models.ScheduleStatusInProgress, updated.Status, "Schedule should be IN_PROGRESS after cutoff")
}

func TestSchedulerService_CheckDrawTimes(t *testing.T) {
	ctx := context.Background()

	// Setup test database
	postgresContainer, db := setupTestDatabase(t, ctx)
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	// Run migrations
	if err := runMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create repositories
	scheduleRepo := repositories.NewGameScheduleRepository(db)
	gameRepo := repositories.NewGameRepository(db)

	// Create test game
	game := &models.Game{
		ID:                  uuid.New(),
		Code:                "TEST-DRAW",
		Name:                "Test Draw Game",
		Type:                "5_BY_90",
		Organizer:           "ORGANIZER_RAND_LOTTERY",
		GameCategory:        "private",
		GameFormat:          "5_by_90",
		Status:              "ACTIVE",
		Version:             "1.0.0",
		BasePrice:           1.00,
		MinStakeAmount:      0.50,
		MaxStakeAmount:      50.00,
		MaxTicketsPerPlayer: 10,
		NumberRangeMin:      1,
		NumberRangeMax:      90,
		SelectionCount:      5,
		DrawFrequency:       "daily",
		SalesCutoffMinutes:  30,
	}
	err := gameRepo.Create(ctx, game)
	require.NoError(t, err)

	// Create test schedule with draw time that will be reached soon
	now := time.Now()
	schedule := &models.GameSchedule{
		GameID:         game.ID,
		GameName:       &game.Name,
		ScheduledStart: now.Add(-2 * time.Hour),
		ScheduledEnd:   now.Add(-1 * time.Hour),
		ScheduledDraw:  now.Add(500 * time.Millisecond), // Draw time within window
		Frequency:      models.DrawFrequencyDaily,
		IsActive:       true,
		Status:         models.ScheduleStatusScheduled,
	}
	err = scheduleRepo.Create(ctx, schedule)
	require.NoError(t, err)

	// Create mock draw client that tracks calls
	var drawCreated bool
	var createdDrawID uuid.UUID
	mockDrawClient := &clients.MockDrawServiceClient{
		CreateDrawFunc: func(ctx context.Context, game *models.Game, schedule *models.GameSchedule, ticketClient clients.TicketServiceClient) (uuid.UUID, error) {
			drawCreated = true
			createdDrawID = uuid.New()
			return createdDrawID, nil
		},
	}

	// Create scheduler service
	logger := log.New(os.Stdout, "[test-scheduler] ", log.LstdFlags)
	schedulerService, err := NewSchedulerService(
		config.SchedulerConfig{
			Enabled:       true,
			Interval:      1 * time.Minute,
			WindowMinutes: 2,
			Timezone:      "Africa/Accra",
		},
		scheduleRepo,
		gameRepo,
		mockDrawClient,
		nil,                          // ticket client not needed for this test
		nil,                          // event bus not needed for this test
		nil,                          // notification client not needed for this test
		nil,                          // admin client not needed for this test
		[]string{"test@example.com"}, // fallback emails
		logger,
	)
	require.NoError(t, err)

	// Sleep briefly to ensure the draw time has passed
	time.Sleep(600 * time.Millisecond)

	// Check draw times - should trigger
	schedulerService.checkDrawTimes(ctx)

	// Verify draw was created
	assert.True(t, drawCreated, "Draw should have been created")
	assert.NotEqual(t, uuid.Nil, createdDrawID, "Draw ID should not be nil")

	// Verify schedule was updated
	updated, err := scheduleRepo.GetByID(ctx, schedule.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ScheduleStatusCompleted, updated.Status, "Schedule should be COMPLETED")
	assert.NotNil(t, updated.DrawResultID, "Draw result ID should be set")
	assert.Equal(t, createdDrawID, *updated.DrawResultID, "Draw result ID should match created draw ID")
}

func TestSchedulerService_TimezoneHandling(t *testing.T) {
	// Create mock repositories
	mockScheduleRepo := &mockScheduleRepository{}
	mockGameRepo := &mockGameRepository{}
	mockDrawClient := &clients.MockDrawServiceClient{}

	logger := log.New(os.Stdout, "[test-scheduler] ", log.LstdFlags)

	// Create scheduler with Ghana timezone
	schedulerService, err := NewSchedulerService(
		config.SchedulerConfig{
			Enabled:       true,
			Interval:      1 * time.Minute,
			WindowMinutes: 2,
			Timezone:      "Africa/Accra",
		},
		mockScheduleRepo,
		mockGameRepo,
		mockDrawClient,
		nil,                          // ticket client not needed for this test
		nil,                          // event bus not needed for this test
		nil,                          // notification client not needed for this test
		nil,                          // admin client not needed for this test
		[]string{"test@example.com"}, // fallback emails
		logger,
	)
	require.NoError(t, err)

	// Verify timezone is set correctly
	assert.Equal(t, "Africa/Accra", schedulerService.location.String())

	// Test GetCurrentTime returns time in Ghana timezone
	currentTime := schedulerService.GetCurrentTime()
	assert.Equal(t, "Africa/Accra", currentTime.Location().String())

	// Test IsTimeReached with different timezones
	pastTime := time.Now().Add(-1 * time.Hour)
	futureTime := time.Now().Add(1 * time.Hour)

	assert.True(t, schedulerService.IsTimeReached(pastTime), "Past time should be reached")
	assert.False(t, schedulerService.IsTimeReached(futureTime), "Future time should not be reached")
}

// Helper function to setup test database
func setupTestDatabase(t *testing.T, ctx context.Context) (testcontainers.Container, *sql.DB) {
	// Start PostgreSQL testcontainer
	postgresContainer, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("scheduler_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Connect to database
	db, err := sql.Open("postgres", connStr)
	require.NoError(t, err)

	// Verify connection
	err = db.PingContext(ctx)
	require.NoError(t, err)

	return postgresContainer, db
}

// Helper function to run migrations
func runMigrations(db *sql.DB) error {
	// Set Goose dialect
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	// Get the migrations directory path relative to the test file
	migrationsDir := filepath.Join("..", "..", "migrations")

	// Run up migrations
	if err := goose.Up(db, migrationsDir); err != nil {
		return err
	}

	return nil
}

// Mock implementations for unit tests
type mockScheduleRepository struct {
	schedules []*models.GameSchedule
}

func (m *mockScheduleRepository) Create(ctx context.Context, schedule *models.GameSchedule) error {
	schedule.ID = uuid.New()
	m.schedules = append(m.schedules, schedule)
	return nil
}

func (m *mockScheduleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.GameSchedule, error) {
	for _, s := range m.schedules {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, fmt.Errorf("schedule not found")
}

func (m *mockScheduleRepository) GetByGameID(ctx context.Context, gameID uuid.UUID) ([]*models.GameSchedule, error) {
	return m.schedules, nil
}

func (m *mockScheduleRepository) Update(ctx context.Context, schedule *models.GameSchedule) error {
	return nil
}

func (m *mockScheduleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockScheduleRepository) GetActiveSchedules(ctx context.Context) ([]*models.GameSchedule, error) {
	return m.schedules, nil
}

func (m *mockScheduleRepository) GetUpcomingSchedules(ctx context.Context, limit int) ([]*models.GameSchedule, error) {
	return m.schedules, nil
}

func (m *mockScheduleRepository) GetSchedulesInTimeRange(ctx context.Context, start, end time.Time) ([]*models.GameSchedule, error) {
	return m.schedules, nil
}

func (m *mockScheduleRepository) DeleteSchedulesInTimeRange(ctx context.Context, start, end time.Time) error {
	return nil
}

func (m *mockScheduleRepository) DeleteUnplayedSchedulesInTimeRange(ctx context.Context, start, end time.Time) error {
	return nil
}

func (m *mockScheduleRepository) GetSchedulesDueForProcessing(ctx context.Context, eventType string, windowMinutes int) ([]*models.GameSchedule, error) {
	return m.schedules, nil
}

type mockGameRepository struct {
	games []*models.Game
}

func (m *mockGameRepository) Create(ctx context.Context, game *models.Game) error {
	game.ID = uuid.New()
	m.games = append(m.games, game)
	return nil
}

func (m *mockGameRepository) CreateWithTx(ctx context.Context, tx *sql.Tx, game *models.Game) error {
	return m.Create(ctx, game)
}

func (m *mockGameRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Game, error) {
	for _, g := range m.games {
		if g.ID == id {
			return g, nil
		}
	}
	return nil, fmt.Errorf("game not found")
}

func (m *mockGameRepository) GetByName(ctx context.Context, name string) (*models.Game, error) {
	for _, g := range m.games {
		if g.Name == name {
			return g, nil
		}
	}
	return nil, fmt.Errorf("game not found")
}

func (m *mockGameRepository) GetByNameWithTx(ctx context.Context, tx *sql.Tx, name string) (*models.Game, error) {
	return m.GetByName(ctx, name)
}

func (m *mockGameRepository) Update(ctx context.Context, game *models.Game) error {
	return nil
}

func (m *mockGameRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockGameRepository) List(ctx context.Context, filter models.GameFilter, page, limit int) ([]*models.Game, int64, error) {
	return m.games, int64(len(m.games)), nil
}

func (m *mockGameRepository) GetActiveGames(ctx context.Context) ([]*models.Game, error) {
	return m.games, nil
}

func (m *mockGameRepository) GetEnabledBetTypesByGameIDs(ctx context.Context, gameIDs []uuid.UUID) (map[uuid.UUID][]models.BetType, error) {
	return nil, nil
}

func (m *mockGameRepository) UpdateLogo(ctx context.Context, gameID uuid.UUID, logoURL *string, brandColor *string) error {
	for _, g := range m.games {
		if g.ID == gameID {
			g.LogoURL = logoURL
			g.BrandColor = brandColor
			return nil
		}
	}
	return fmt.Errorf("game not found")
}

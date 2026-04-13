package services

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"github.com/randco/randco-microservices/services/service-game/internal/repositories"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// GameServiceIntegrationTestSuite tests the Game service repository layer
// with real infrastructure using TestContainers
type GameServiceIntegrationTestSuite struct {
	suite.Suite
	// Infrastructure
	pgContainer    *postgres.PostgresContainer
	redisContainer *rediscontainer.RedisContainer
	db             *sql.DB
	redisClient    *redis.Client

	// Repositories
	gameRepo     repositories.GameRepository
	rulesRepo    repositories.GameRulesRepository
	approvalRepo repositories.GameApprovalRepository
	scheduleRepo repositories.GameScheduleRepository

	// Test context
	ctx    context.Context
	cancel context.CancelFunc
}

// SetupSuite initializes the test infrastructure using TestContainers
func (suite *GameServiceIntegrationTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(suite.ctx,
		"postgres:17",
		postgres.WithDatabase("game_service_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Minute),
		),
	)
	suite.Require().NoError(err)
	suite.pgContainer = pgContainer

	// Get PostgreSQL connection string
	connStr, err := pgContainer.ConnectionString(suite.ctx, "sslmode=disable")
	suite.Require().NoError(err)

	// Connect to PostgreSQL
	suite.db, err = sql.Open("postgres", connStr)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.db.Ping())

	// Start Redis container
	redisContainer, err := rediscontainer.Run(suite.ctx,
		"redis:7.4-alpine",
		rediscontainer.WithSnapshotting(10, 1),
		rediscontainer.WithLogLevel(rediscontainer.LogLevelVerbose),
	)
	suite.Require().NoError(err)
	suite.redisContainer = redisContainer

	// Get Redis connection string
	connString, err := redisContainer.ConnectionString(suite.ctx)
	suite.Require().NoError(err)

	// Extract host:port from redis://host:port format
	redisAddr := connString[8:] // Remove "redis://" prefix

	// Connect to Redis
	suite.redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	suite.Require().NoError(suite.redisClient.Ping(suite.ctx).Err())

	// Run database migrations
	err = suite.runMigrations(connStr)
	suite.Require().NoError(err)

	// Initialize repositories
	suite.gameRepo = repositories.NewGameRepository(suite.db)
	suite.rulesRepo = repositories.NewGameRulesRepository(suite.db)
	suite.approvalRepo = repositories.NewGameApprovalRepository(suite.db)
	suite.scheduleRepo = repositories.NewGameScheduleRepository(suite.db)
}

// TearDownSuite cleans up all test infrastructure
func (suite *GameServiceIntegrationTestSuite) TearDownSuite() {
	// Close database and Redis connections
	if suite.db != nil {
		_ = suite.db.Close()
	}
	if suite.redisClient != nil {
		_ = suite.redisClient.Close()
	}

	// Terminate containers
	if suite.pgContainer != nil {
		_ = suite.pgContainer.Terminate(suite.ctx)
	}
	if suite.redisContainer != nil {
		_ = suite.redisContainer.Terminate(suite.ctx)
	}

	suite.cancel()
}

// SetupTest runs before each test method
func (suite *GameServiceIntegrationTestSuite) SetupTest() {
	// Clean up database tables for each test
	suite.cleanupDatabase()
}

// runMigrations runs the Goose migrations to set up the test database schema
func (suite *GameServiceIntegrationTestSuite) runMigrations(connStr string) error {
	// Find the project root directory and migrations folder
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Navigate up to find the migrations directory
	migrationsDir := ""
	for i := 0; i < 10; i++ { // Safety limit
		testDir := filepath.Join(wd, "migrations")
		if _, err := os.Stat(testDir); err == nil {
			migrationsDir = testDir
			break
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			break // Reached root
		}
		wd = parent
	}

	if migrationsDir == "" {
		return fmt.Errorf("migrations directory not found")
	}

	// Run Goose migrations
	cmd := exec.Command("goose", "-dir", migrationsDir, "postgres", connStr, "up")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// cleanupDatabase removes all test data between tests
func (suite *GameServiceIntegrationTestSuite) cleanupDatabase() {
	tables := []string{
		"game_schedules",
		"game_approvals",
		"game_rules",
		"games",
	}

	for _, table := range tables {
		_, err := suite.db.ExecContext(suite.ctx, "DELETE FROM "+table)
		suite.Require().NoError(err)
	}
}

// TestGameRepositoryIntegration tests the game repository with real database
func (suite *GameServiceIntegrationTestSuite) TestGameRepositoryIntegration() {
	// Test data preparation
	// Removed startTime and endTime variables (StartDate/EndDate fields no longer exist)

	testGame := &models.Game{
		Code:                "LOTTO-5-90-TEST",
		Name:                "Ghana Lotto 5/90 Integration Test",
		Type:                "5_90",
		GameType:            stringPtr("5_90"),
		GameFormat:          "5_by_90",
		GameCategory:        "NUMBERS",
		Organizer:           "NLA",
		MinStakeAmount:      1.0,  // 1 GHS
		MaxStakeAmount:      10.0, // 10 GHS
		MaxTicketsPerPlayer: 10,
		DrawFrequency:       "weekly",
		DrawDays:            []string{"WEDNESDAY", "SATURDAY"},
		DrawTime:            timePtr(time.Date(2023, 1, 1, 18, 0, 0, 0, time.UTC)),
		WeeklySchedule:      boolPtr(true),
		Status:              "DRAFT",
		Description:         stringPtr("Integration test for Ghana National Lottery 5/90"),
		StartTime:           stringPtr("09:00"),
		EndTime:             stringPtr("17:00"),
		NumberRangeMin:      1,
		NumberRangeMax:      90,
		SelectionCount:      5,
		SalesCutoffMinutes:  30,
		BasePrice:           1.0,
		MultiDrawEnabled:    false,
		MaxDrawsAdvance:     nil,
		Version:             "1.0",
	}

	// Test Create
	err := suite.gameRepo.Create(suite.ctx, testGame)
	suite.Require().NoError(err)
	suite.NotEmpty(testGame.ID)
	suite.NotZero(testGame.CreatedAt)
	suite.NotZero(testGame.UpdatedAt)

	// Test GetByID
	retrievedGame, err := suite.gameRepo.GetByID(suite.ctx, testGame.ID)
	suite.Require().NoError(err)
	suite.Equal(testGame.Code, retrievedGame.Code)
	suite.Equal(testGame.Name, retrievedGame.Name)
	suite.Equal(testGame.Type, retrievedGame.Type)
	suite.Equal(testGame.DrawDays, retrievedGame.DrawDays)

	// Test Update
	retrievedGame.Name = "Updated Ghana Lotto 5/90"
	retrievedGame.MinStakeAmount = 2.0 // 2 GHS (above minimum 0.50)
	err = suite.gameRepo.Update(suite.ctx, retrievedGame)
	suite.Require().NoError(err)

	// Verify update
	updatedGame, err := suite.gameRepo.GetByID(suite.ctx, retrievedGame.ID)
	suite.Require().NoError(err)
	suite.Equal("Updated Ghana Lotto 5/90", updatedGame.Name)
	suite.Equal(2.0, updatedGame.MinStakeAmount)
	suite.True(updatedGame.UpdatedAt.After(updatedGame.CreatedAt))

	// Test GetByName
	namedGame, err := suite.gameRepo.GetByName(suite.ctx, "Updated Ghana Lotto 5/90")
	suite.Require().NoError(err)
	suite.Equal(updatedGame.ID, namedGame.ID)

	// Test List with filters - use simple string pointers
	filter := models.GameFilter{
		GameFormat:   nil, // Skip filters to get all games for now
		GameCategory: nil,
		Status:       nil,
	}
	games, total, err := suite.gameRepo.List(suite.ctx, filter, 1, 10)
	suite.Require().NoError(err)
	suite.Equal(int64(1), total)
	suite.Len(games, 1)
	suite.Equal(updatedGame.ID, games[0].ID)

	// Test Delete (soft delete)
	err = suite.gameRepo.Delete(suite.ctx, updatedGame.ID)
	suite.Require().NoError(err)

	// Verify game is marked as terminated
	deletedGame, err := suite.gameRepo.GetByID(suite.ctx, updatedGame.ID)
	suite.Require().NoError(err)
	suite.Equal("TERMINATED", deletedGame.Status)
}

// TestGameWorkflowIntegration tests the complete workflow with multiple repositories
func (suite *GameServiceIntegrationTestSuite) TestGameWorkflowIntegration() {
	// Step 1: Create a game
	testGame := &models.Game{
		Code:                "WORKFLOW-TEST",
		Name:                "Workflow Test Game",
		Type:                "6_49",
		GameType:            stringPtr("6_49"),
		GameFormat:          "6_by_49",
		GameCategory:        "NUMBERS",
		Organizer:           "RAND_LOTTERY",
		MinStakeAmount:      5.0,
		MaxStakeAmount:      50.0,
		MaxTicketsPerPlayer: 5,
		DrawFrequency:       "weekly",
		DrawDays:            []string{"SUNDAY"},
		DrawTime:            timePtr(time.Date(2023, 1, 1, 20, 0, 0, 0, time.UTC)),
		Status:              "DRAFT",
		Description:         stringPtr("Game for testing complete workflow"),
		NumberRangeMin:      1,
		NumberRangeMax:      49,
		SelectionCount:      6,
		SalesCutoffMinutes:  30,
		BasePrice:           5.0,
		MultiDrawEnabled:    false,
		MaxDrawsAdvance:     nil,
		Version:             "1.0",
	}

	err := suite.gameRepo.Create(suite.ctx, testGame)
	suite.Require().NoError(err)

	// Step 2: Add game rules
	testRules := &models.GameRules{
		GameID:         testGame.ID,
		NumbersToPick:  6,
		TotalNumbers:   49,
		MinSelections:  1,
		MaxSelections:  10,
		NumberRangeMin: int32Ptr(1),
		NumberRangeMax: int32Ptr(49),
		SelectionCount: int32Ptr(6),
		AllowQuickPick: true,
		SpecialRules:   stringPtr("Standard 6/49 lottery rules"),
	}

	err = suite.rulesRepo.Create(suite.ctx, testRules)
	suite.Require().NoError(err)
	suite.NotEmpty(testRules.ID)

	// Verify rules can be retrieved
	retrievedRules, err := suite.rulesRepo.GetByGameID(suite.ctx, testGame.ID)
	suite.Require().NoError(err)
	suite.Equal(testRules.NumbersToPick, retrievedRules.NumbersToPick)
	suite.Equal(testRules.TotalNumbers, retrievedRules.TotalNumbers)

	// Step 3: Submit for approval
	testApproval := &models.GameApproval{
		GameID:        testGame.ID,
		ApprovalStage: models.ApprovalStageSubmitted,
		Notes:         stringPtr("Submitted for regulatory review"),
	}

	err = suite.approvalRepo.Create(suite.ctx, testApproval)
	suite.Require().NoError(err)
	suite.NotEmpty(testApproval.ID)

	// Update game status to pending approval
	testGame.Status = "PENDING_APPROVAL"
	err = suite.gameRepo.Update(suite.ctx, testGame)
	suite.Require().NoError(err)

	// Verify approval workflow
	approval, err := suite.approvalRepo.GetByGameID(suite.ctx, testGame.ID)
	suite.Require().NoError(err)
	suite.Equal(models.ApprovalStageSubmitted, approval.ApprovalStage)

	// Step 4: Schedule the game
	scheduleStart := time.Now().Add(48 * time.Hour)
	scheduleEnd := scheduleStart.Add(2 * time.Hour)
	scheduleDraw := scheduleEnd.Add(1 * time.Hour)

	testSchedule := &models.GameSchedule{
		GameID:         testGame.ID,
		ScheduledStart: scheduleStart,
		ScheduledEnd:   scheduleEnd,
		ScheduledDraw:  scheduleDraw,
		Frequency:      "weekly",
		IsActive:       true,
		Notes:          stringPtr("Weekly Sunday lottery"),
	}

	err = suite.scheduleRepo.Create(suite.ctx, testSchedule)
	suite.Require().NoError(err)
	suite.NotEmpty(testSchedule.ID)

	// Verify schedule
	schedules, err := suite.scheduleRepo.GetByGameID(suite.ctx, testGame.ID)
	suite.Require().NoError(err)
	suite.Len(schedules, 1)
	suite.Equal(models.DrawFrequency("weekly"), schedules[0].Frequency)
	suite.True(schedules[0].IsActive)

	// Verify complete workflow integrity
	finalGame, err := suite.gameRepo.GetByID(suite.ctx, testGame.ID)
	suite.Require().NoError(err)
	suite.Equal("PENDING_APPROVAL", finalGame.Status)

	finalRules, err := suite.rulesRepo.GetByGameID(suite.ctx, testGame.ID)
	suite.Require().NoError(err)
	suite.Equal(int32(6), finalRules.NumbersToPick)

	finalApproval, err := suite.approvalRepo.GetByGameID(suite.ctx, testGame.ID)
	suite.Require().NoError(err)
	suite.Equal(models.ApprovalStageSubmitted, finalApproval.ApprovalStage)
}

// TestRedisIntegration tests Redis caching functionality
func (suite *GameServiceIntegrationTestSuite) TestRedisIntegration() {
	// Test basic Redis operations
	key := "test:game:cache"
	value := "test-game-data"

	err := suite.redisClient.Set(suite.ctx, key, value, time.Minute).Err()
	suite.Require().NoError(err)

	retrievedValue, err := suite.redisClient.Get(suite.ctx, key).Result()
	suite.Require().NoError(err)
	suite.Equal(value, retrievedValue)

	// Test Redis with JSON data
	gameData := map[string]interface{}{
		"id":   "test-game-id",
		"name": "Test Game",
		"type": "5_90",
	}

	err = suite.redisClient.HSet(suite.ctx, "game:test-game-id", gameData).Err()
	suite.Require().NoError(err)

	retrievedData, err := suite.redisClient.HGetAll(suite.ctx, "game:test-game-id").Result()
	suite.Require().NoError(err)
	suite.Equal("Test Game", retrievedData["name"])
	suite.Equal("5_90", retrievedData["type"])
}

// TestConcurrentRepositoryOperations tests repository operations under concurrent load
func (suite *GameServiceIntegrationTestSuite) TestConcurrentRepositoryOperations() {
	const numWorkers = 5
	const gamesPerWorker = 3

	// Channel to collect results
	results := make(chan error, numWorkers)

	// Launch concurrent game creation
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer func() {
				if r := recover(); r != nil {
					results <- fmt.Errorf("worker %d panicked: %v", workerID, r)
					return
				}
			}()

			for j := 0; j < gamesPerWorker; j++ {
				testGame := &models.Game{
					Code:                fmt.Sprintf("CONC-%d-%d", workerID, j),
					Name:                fmt.Sprintf("Concurrent Game %d-%d", workerID, j),
					Type:                "5_90",
					GameType:            stringPtr("5_90"),
					GameFormat:          "LOTTERY",
					GameCategory:        "NUMBERS",
					Organizer:           "NLA",
					MinStakeAmount:      1.0,
					MaxStakeAmount:      10.0,
					MaxTicketsPerPlayer: 5,
					DrawFrequency:       "DAILY",
					DrawDays:            []string{"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY"},
					Status:              "DRAFT",
					Description:         stringPtr(fmt.Sprintf("Concurrent test game by worker %d", workerID)),
					StartTime:           stringPtr("09:00"),
					EndTime:             stringPtr("17:00"),
					NumberRangeMin:      1,
					NumberRangeMax:      90,
					SelectionCount:      5,
					SalesCutoffMinutes:  30,
					BasePrice:           1.0,
					MultiDrawEnabled:    false,
					MaxDrawsAdvance:     nil,
					Version:             "1.0",
				}

				err := suite.gameRepo.Create(suite.ctx, testGame)
				if err != nil {
					results <- fmt.Errorf("worker %d, game %d failed: %w", workerID, j, err)
					return
				}
			}

			results <- nil // Success
		}(i)
	}

	// Wait for all workers to complete
	for i := 0; i < numWorkers; i++ {
		result := <-results
		suite.NoError(result)
	}

	// Verify all games were created
	games, total, err := suite.gameRepo.List(suite.ctx, models.GameFilter{}, 1, 100)
	suite.Require().NoError(err)
	suite.Equal(int64(numWorkers*gamesPerWorker), total)
	suite.Len(games, numWorkers*gamesPerWorker)
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// Run the integration test suite
func TestGameServiceIntegrationSuite(t *testing.T) {
	suite.Run(t, new(GameServiceIntegrationTestSuite))
}

package repositories

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/randco/service-draw/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type DrawRepositoryTestSuite struct {
	suite.Suite
	db         *sqlx.DB
	repo       DrawRepository
	container  testcontainers.Container
	ctx        context.Context
	testGameID uuid.UUID
}

func TestDrawRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(DrawRepositoryTestSuite))
}

func (s *DrawRepositoryTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.testGameID = uuid.New()

	// Start PostgreSQL testcontainer
	postgresContainer, err := postgres.Run(s.ctx, "postgres:17-alpine",
		postgres.WithDatabase("draw_test"),
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
	s.db, err = sqlx.Connect("postgres", connStr)
	require.NoError(s.T(), err)

	// Verify connection
	err = s.db.PingContext(s.ctx)
	require.NoError(s.T(), err)

	// Run migrations
	testHelper := NewTestHelper()
	err = testHelper.SetupTestDB(s.T(), s.db.DB)
	require.NoError(s.T(), err)

	// Initialize repository
	s.repo = NewDrawRepository(s.db)
}

func (s *DrawRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(s.T(), err)
	}
}

func (s *DrawRepositoryTestSuite) SetupTest() {
	// Clean up tables before each test
	testHelper := NewTestHelper()
	err := testHelper.CleanupTestData(s.T(), s.db.DB)
	require.NoError(s.T(), err)
}

// ============================================================================
// Create Tests
// ============================================================================

func (s *DrawRepositoryTestSuite) TestCreate_Success() {
	draw := &models.Draw{
		ID:            uuid.New(),
		GameID:        s.testGameID,
		GameName:      "VAG MONDAY",
		DrawName:      "Monday Draw 1001",
		DrawNumber:    1001,
		ScheduledTime: time.Now().Add(24 * time.Hour),
		Status:        models.DrawStatusScheduled,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err := s.repo.Create(s.ctx, draw)
	require.NoError(s.T(), err)

	// Verify the draw was created
	retrieved, err := s.repo.GetByID(s.ctx, draw.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), draw.ID, retrieved.ID)
	assert.Equal(s.T(), draw.GameName, retrieved.GameName)
	assert.Equal(s.T(), draw.DrawNumber, retrieved.DrawNumber)
	assert.Equal(s.T(), draw.Status, retrieved.Status)
}

func (s *DrawRepositoryTestSuite) TestCreate_WithStageData() {
	now := time.Now()
	draw := &models.Draw{
		ID:            uuid.New(),
		GameID:        s.testGameID,
		GameName:      "VAG MONDAY",
		DrawName:      "Monday Draw 1002",
		DrawNumber:    1002,
		ScheduledTime: time.Now().Add(24 * time.Hour),
		Status:        models.DrawStatusInProgress,
		StageData: &models.DrawStage{
			CurrentStage:     1,
			StageName:        "Preparation",
			StageStatus:      models.StageStatusInProgress,
			StageStartedAt:   &now,
			StageCompletedAt: nil,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := s.repo.Create(s.ctx, draw)
	require.NoError(s.T(), err)

	// Verify stage data was saved
	retrieved, err := s.repo.GetByID(s.ctx, draw.ID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), retrieved.StageData)
	assert.Equal(s.T(), 1, retrieved.StageData.CurrentStage)
	assert.Equal(s.T(), "Preparation", retrieved.StageData.StageName)
	assert.Equal(s.T(), models.StageStatusInProgress, retrieved.StageData.StageStatus)
}

func (s *DrawRepositoryTestSuite) TestCreate_DuplicateDrawNumber() {
	draw1 := &models.Draw{
		ID:            uuid.New(),
		GameID:        s.testGameID,
		GameName:      "VAG MONDAY",
		DrawName:      "Monday Draw 1003",
		DrawNumber:    1003,
		ScheduledTime: time.Now().Add(24 * time.Hour),
		Status:        models.DrawStatusScheduled,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err := s.repo.Create(s.ctx, draw1)
	require.NoError(s.T(), err)

	// Try to create another draw with same draw number
	draw2 := &models.Draw{
		ID:            uuid.New(),
		GameID:        s.testGameID,
		GameName:      "VAG MONDAY",
		DrawName:      "Monday Draw 1003 Duplicate",
		DrawNumber:    1003, // Same as draw1
		ScheduledTime: time.Now().Add(48 * time.Hour),
		Status:        models.DrawStatusScheduled,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err = s.repo.Create(s.ctx, draw2)
	assert.Error(s.T(), err, "should fail due to unique constraint on draw_number")
}

// ============================================================================
// GetByID Tests
// ============================================================================

func (s *DrawRepositoryTestSuite) TestGetByID_Success() {
	draw := s.createTestDraw(1004, models.DrawStatusScheduled)

	retrieved, err := s.repo.GetByID(s.ctx, draw.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), draw.ID, retrieved.ID)
	assert.Equal(s.T(), draw.DrawNumber, retrieved.DrawNumber)
}

func (s *DrawRepositoryTestSuite) TestGetByID_NotFound() {
	nonExistentID := uuid.New()

	_, err := s.repo.GetByID(s.ctx, nonExistentID)
	assert.Error(s.T(), err)
}

// ============================================================================
// Update Tests
// ============================================================================

func (s *DrawRepositoryTestSuite) TestUpdate_StatusChange() {
	draw := s.createTestDraw(1005, models.DrawStatusScheduled)

	// Update status
	draw.Status = models.DrawStatusInProgress
	draw.UpdatedAt = time.Now()

	err := s.repo.Update(s.ctx, draw)
	require.NoError(s.T(), err)

	// Verify update
	retrieved, err := s.repo.GetByID(s.ctx, draw.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.DrawStatusInProgress, retrieved.Status)
}

func (s *DrawRepositoryTestSuite) TestUpdate_WithWinningNumbers() {
	draw := s.createTestDraw(1006, models.DrawStatusInProgress)

	// Add winning numbers (ExecutedTime must be after ScheduledTime due to DB constraint)
	draw.WinningNumbers = []int32{5, 12, 23, 45, 67}
	executedTime := draw.ScheduledTime.Add(2 * time.Hour) // After scheduled time
	draw.ExecutedTime = &executedTime

	err := s.repo.Update(s.ctx, draw)
	require.NoError(s.T(), err)

	// Verify update
	retrieved, err := s.repo.GetByID(s.ctx, draw.ID)
	require.NoError(s.T(), err)
	assert.ElementsMatch(s.T(), []int32{5, 12, 23, 45, 67}, []int32(retrieved.WinningNumbers))
	assert.NotNil(s.T(), retrieved.ExecutedTime)
}

func (s *DrawRepositoryTestSuite) TestUpdate_WithMachineNumbers() {
	draw := s.createTestDraw(1020, models.DrawStatusCompleted)

	// Add winning numbers and machine numbers
	draw.WinningNumbers = []int32{5, 12, 23, 45, 67}
	draw.MachineNumbers = []int32{10, 20, 30, 40, 50}
	executedTime := draw.ScheduledTime.Add(2 * time.Hour)
	draw.ExecutedTime = &executedTime

	err := s.repo.Update(s.ctx, draw)
	require.NoError(s.T(), err)

	// Verify update
	retrieved, err := s.repo.GetByID(s.ctx, draw.ID)
	require.NoError(s.T(), err)
	assert.ElementsMatch(s.T(), []int32{5, 12, 23, 45, 67}, []int32(retrieved.WinningNumbers))
	assert.ElementsMatch(s.T(), []int32{10, 20, 30, 40, 50}, []int32(retrieved.MachineNumbers))
	assert.NotNil(s.T(), retrieved.ExecutedTime)
}

func (s *DrawRepositoryTestSuite) TestUpdate_StageData() {
	draw := s.createTestDraw(1007, models.DrawStatusInProgress)

	// Update stage data
	now := time.Now()
	draw.StageData = &models.DrawStage{
		CurrentStage:     2,
		StageName:        "Number Selection",
		StageStatus:      models.StageStatusCompleted,
		StageStartedAt:   &now,
		StageCompletedAt: &now,
		NumberSelectionData: &models.NumberSelectionStageData{
			WinningNumbers: []int32{5, 12, 23, 45, 67},
			IsVerified:     true,
			VerifiedBy:     "test-admin",
			VerifiedAt:     &now,
		},
	}

	err := s.repo.Update(s.ctx, draw)
	require.NoError(s.T(), err)

	// Verify stage data update
	retrieved, err := s.repo.GetByID(s.ctx, draw.ID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), retrieved.StageData)
	assert.Equal(s.T(), 2, retrieved.StageData.CurrentStage)
	assert.Equal(s.T(), "Number Selection", retrieved.StageData.StageName)
	require.NotNil(s.T(), retrieved.StageData.NumberSelectionData)
	assert.Equal(s.T(), []int32{5, 12, 23, 45, 67}, retrieved.StageData.NumberSelectionData.WinningNumbers)
}

// ============================================================================
// List Tests
// ============================================================================

func (s *DrawRepositoryTestSuite) TestList_AllDraws() {
	// Create multiple draws
	s.createTestDraw(2001, models.DrawStatusScheduled)
	s.createTestDraw(2002, models.DrawStatusInProgress)
	s.createTestDraw(2003, models.DrawStatusCompleted)

	draws, total, err := s.repo.List(s.ctx, nil, nil, nil, nil, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), total)
	assert.Len(s.T(), draws, 3)
}

func (s *DrawRepositoryTestSuite) TestList_FilterByGameID() {
	gameID1 := uuid.New()
	gameID2 := uuid.New()

	// Create draws for different games
	s.createTestDrawWithGameID(3001, gameID1, models.DrawStatusScheduled)
	s.createTestDrawWithGameID(3002, gameID1, models.DrawStatusScheduled)
	s.createTestDrawWithGameID(3003, gameID2, models.DrawStatusScheduled)

	draws, total, err := s.repo.List(s.ctx, &gameID1, nil, nil, nil, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), total)
	assert.Len(s.T(), draws, 2)
}

func (s *DrawRepositoryTestSuite) TestList_FilterByStatus() {
	s.createTestDraw(4001, models.DrawStatusScheduled)
	s.createTestDraw(4002, models.DrawStatusScheduled)
	s.createTestDraw(4003, models.DrawStatusCompleted)

	status := models.DrawStatusScheduled
	draws, total, err := s.repo.List(s.ctx, nil, &status, nil, nil, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), total)
	assert.Len(s.T(), draws, 2)
}

func (s *DrawRepositoryTestSuite) TestList_Pagination() {
	// Create 5 draws
	for i := 5001; i <= 5005; i++ {
		s.createTestDraw(i, models.DrawStatusScheduled)
	}

	// Get first page
	page1, total, err := s.repo.List(s.ctx, nil, nil, nil, nil, 1, 2)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), total)
	assert.Len(s.T(), page1, 2)

	// Get second page
	page2, total, err := s.repo.List(s.ctx, nil, nil, nil, nil, 2, 2)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), total)
	assert.Len(s.T(), page2, 2)

	// Verify no overlap
	assert.NotEqual(s.T(), page1[0].ID, page2[0].ID)
}

// ============================================================================
// Helper Functions
// ============================================================================

func (s *DrawRepositoryTestSuite) createTestDraw(drawNumber int, status models.DrawStatus) *models.Draw {
	return s.createTestDrawWithGameID(drawNumber, s.testGameID, status)
}

func (s *DrawRepositoryTestSuite) createTestDrawWithGameID(drawNumber int, gameID uuid.UUID, status models.DrawStatus) *models.Draw {
	draw := &models.Draw{
		ID:            uuid.New(),
		GameID:        gameID,
		GameName:      "VAG MONDAY",
		DrawName:      fmt.Sprintf("Test Draw %d", drawNumber),
		DrawNumber:    drawNumber,
		ScheduledTime: time.Now().Add(24 * time.Hour),
		Status:        status,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err := s.repo.Create(s.ctx, draw)
	require.NoError(s.T(), err)

	return draw
}

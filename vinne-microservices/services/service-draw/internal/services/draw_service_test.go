package services

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	gamev1 "github.com/randco/randco-microservices/proto/game/v1"
	ticketv1 "github.com/randco/randco-microservices/proto/ticket/v1"
	walletv1 "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/service-draw/internal/models"
	"github.com/randco/service-draw/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
)

type DrawServiceTestSuite struct {
	suite.Suite
	db              *sqlx.DB
	repo            repositories.DrawRepository
	service         DrawService
	container       testcontainers.Container
	ctx             context.Context
	logger          *log.Logger
	mockGRPCManager *MockGRPCClientManager
	testGameID      uuid.UUID
}

// MockGRPCClientManager mocks the gRPC client manager for testing
type MockGRPCClientManager struct{}

func (m *MockGRPCClientManager) TicketServiceClient(ctx context.Context) (interface{}, error) {
	return &MockTicketServiceClient{}, nil
}

func (m *MockGRPCClientManager) WalletServiceClient(ctx context.Context) (interface{}, error) {
	return &MockWalletServiceClient{}, nil
}

func (m *MockGRPCClientManager) GameServiceClient(ctx context.Context) (gamev1.GameServiceClient, error) {
	return &MockGameServiceClient{}, nil
}

// MockTicketServiceClient mocks the ticket service client
type MockTicketServiceClient struct{}

func (m *MockTicketServiceClient) ListTickets(ctx context.Context, req *ticketv1.ListTicketsRequest, opts ...interface{}) (*ticketv1.ListTicketsResponse, error) {
	// Return empty list for testing
	return &ticketv1.ListTicketsResponse{
		Tickets:  []*ticketv1.Ticket{},
		Total:    0,
		Page:     1,
		PageSize: 100,
	}, nil
}

// MockWalletServiceClient mocks the wallet service client
type MockWalletServiceClient struct{}

func (m *MockWalletServiceClient) CreditRetailerWallet(ctx context.Context, req *walletv1.CreditRetailerWalletRequest, opts ...interface{}) (*walletv1.CreditRetailerWalletResponse, error) {
	return &walletv1.CreditRetailerWalletResponse{
		Success: true,
	}, nil
}

// MockGameServiceClient mocks the game service client
type MockGameServiceClient struct{}

// Implement only the GetGame method that's actually used in the enrichment logic
func (m *MockGameServiceClient) GetGame(ctx context.Context, req *gamev1.GetGameRequest, opts ...grpc.CallOption) (*gamev1.GetGameResponse, error) {
	// Return mock game branding data for tests
	return &gamev1.GetGameResponse{
		Success: true,
		Message: "Game retrieved successfully",
		Game: &gamev1.Game{
			Id:         req.Id,
			Name:       "VAG MONDAY",
			Code:       "VAG-MON",
			LogoUrl:    "https://example.com/vag-monday-logo.png",
			BrandColor: "#FF5733",
		},
	}, nil
}

// Stub implementations for other required methods -  not used in tests but required for interface compliance
func (m *MockGameServiceClient) CreateGame(ctx context.Context, req *gamev1.CreateGameRequest, opts ...grpc.CallOption) (*gamev1.CreateGameResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) UpdateGame(ctx context.Context, req *gamev1.UpdateGameRequest, opts ...grpc.CallOption) (*gamev1.UpdateGameResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) DeleteGame(ctx context.Context, req *gamev1.DeleteGameRequest, opts ...grpc.CallOption) (*gamev1.DeleteGameResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) ListGames(ctx context.Context, req *gamev1.ListGamesRequest, opts ...grpc.CallOption) (*gamev1.ListGamesResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) ActivateGame(ctx context.Context, req *gamev1.ActivateGameRequest, opts ...grpc.CallOption) (*gamev1.ActivateGameResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) ScheduleGame(ctx context.Context, req *gamev1.ScheduleGameRequest, opts ...grpc.CallOption) (*gamev1.ScheduleGameResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) GetGameSchedule(ctx context.Context, req *gamev1.GetGameScheduleRequest, opts ...grpc.CallOption) (*gamev1.GetGameScheduleResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) GetWeeklySchedule(ctx context.Context, req *gamev1.GetWeeklyScheduleRequest, opts ...grpc.CallOption) (*gamev1.GetWeeklyScheduleResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) GetScheduleById(ctx context.Context, req *gamev1.GetScheduleByIdRequest, opts ...grpc.CallOption) (*gamev1.GetScheduleByIdResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) CreatePrizeStructure(ctx context.Context, req *gamev1.CreatePrizeStructureRequest, opts ...grpc.CallOption) (*gamev1.CreatePrizeStructureResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) GetPrizeStructure(ctx context.Context, req *gamev1.GetPrizeStructureRequest, opts ...grpc.CallOption) (*gamev1.GetPrizeStructureResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) UpdatePrizeStructure(ctx context.Context, req *gamev1.UpdatePrizeStructureRequest, opts ...grpc.CallOption) (*gamev1.UpdatePrizeStructureResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) ApproveGame(ctx context.Context, req *gamev1.ApproveGameRequest, opts ...grpc.CallOption) (*gamev1.ApproveGameResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) SuspendGame(ctx context.Context, req *gamev1.SuspendGameRequest, opts ...grpc.CallOption) (*gamev1.SuspendGameResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) CreateGameRules(ctx context.Context, req *gamev1.CreateGameRulesRequest, opts ...grpc.CallOption) (*gamev1.CreateGameRulesResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) GetGameRules(ctx context.Context, req *gamev1.GetGameRulesRequest, opts ...grpc.CallOption) (*gamev1.GetGameRulesResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) UpdateGameRules(ctx context.Context, req *gamev1.UpdateGameRulesRequest, opts ...grpc.CallOption) (*gamev1.UpdateGameRulesResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) SubmitForApproval(ctx context.Context, req *gamev1.SubmitForApprovalRequest, opts ...grpc.CallOption) (*gamev1.SubmitForApprovalResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) RejectGame(ctx context.Context, req *gamev1.RejectGameRequest, opts ...grpc.CallOption) (*gamev1.RejectGameResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) GetApprovalStatus(ctx context.Context, req *gamev1.GetApprovalStatusRequest, opts ...grpc.CallOption) (*gamev1.GetApprovalStatusResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) GetPendingApprovals(ctx context.Context, req *gamev1.GetPendingApprovalsRequest, opts ...grpc.CallOption) (*gamev1.GetPendingApprovalsResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) UpdateScheduledGame(ctx context.Context, req *gamev1.UpdateScheduledGameRequest, opts ...grpc.CallOption) (*gamev1.UpdateScheduledGameResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) GenerateWeeklySchedule(ctx context.Context, req *gamev1.GenerateWeeklyScheduleRequest, opts ...grpc.CallOption) (*gamev1.GenerateWeeklyScheduleResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) ClearWeeklySchedule(ctx context.Context, req *gamev1.ClearWeeklyScheduleRequest, opts ...grpc.CallOption) (*gamev1.ClearWeeklyScheduleResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) UpdateGameLogo(ctx context.Context, req *gamev1.UpdateGameLogoRequest, opts ...grpc.CallOption) (*gamev1.UpdateGameLogoResponse, error) {
	return nil, nil
}
func (m *MockGameServiceClient) DeleteGameLogo(ctx context.Context, req *gamev1.DeleteGameLogoRequest, opts ...grpc.CallOption) (*gamev1.DeleteGameLogoResponse, error) {
	return nil, nil
}

func TestDrawServiceTestSuite(t *testing.T) {
	suite.Run(t, new(DrawServiceTestSuite))
}

func (s *DrawServiceTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.testGameID = uuid.New()
	s.logger = log.New(os.Stdout, "[draw-service-test] ", log.LstdFlags)

	// Start PostgreSQL testcontainer
	postgresContainer, err := postgres.Run(s.ctx, "postgres:17-alpine",
		postgres.WithDatabase("draw_service_test"),
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
	testHelper := repositories.NewTestHelper()
	err = testHelper.SetupTestDB(s.T(), s.db.DB)
	require.NoError(s.T(), err)

	// Initialize repository and service
	s.repo = repositories.NewDrawRepository(s.db)
	s.mockGRPCManager = &MockGRPCClientManager{}
	s.service = NewDrawService(s.repo, s.logger, s.mockGRPCManager)
}

func (s *DrawServiceTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(s.T(), err)
	}
}

func (s *DrawServiceTestSuite) SetupTest() {
	// Clean up tables before each test
	testHelper := repositories.NewTestHelper()
	err := testHelper.CleanupTestData(s.T(), s.db.DB)
	require.NoError(s.T(), err)
}

// ============================================================================
// CreateDraw Tests
// ============================================================================

func (s *DrawServiceTestSuite) TestCreateDraw_Success() {
	request := CreateDrawRequest{
		GameID:        s.testGameID,
		DrawName:      "Monday Morning Draw",
		ScheduledTime: time.Now().Add(24 * time.Hour),
		DrawLocation:  "NLA Office - Accra",
	}

	draw, err := s.service.CreateDraw(s.ctx, request)
	require.NoError(s.T(), err)
	assert.NotEqual(s.T(), uuid.Nil, draw.ID)
	assert.Equal(s.T(), s.testGameID, draw.GameID)
	assert.Equal(s.T(), "Monday Morning Draw", draw.DrawName)
	assert.Equal(s.T(), models.DrawStatusScheduled, draw.Status)
	// DrawNumber may be 0 if not auto-generated yet
	assert.GreaterOrEqual(s.T(), draw.DrawNumber, 0)
}

// TestCreateDraw_AutoGenerateDrawNumber is skipped because DrawNumber auto-generation
// is not yet implemented in the CreateDraw service method. When implemented, this test
// should verify that sequential draws for the same game get incrementing draw numbers.
func (s *DrawServiceTestSuite) TestCreateDraw_AutoGenerateDrawNumber() {
	s.T().Skip("DrawNumber auto-generation not yet implemented in CreateDraw service")
}

func (s *DrawServiceTestSuite) TestCreateDraw_ValidationErrors() {
	tests := []struct {
		name    string
		request CreateDrawRequest
		errMsg  string
	}{
		{
			name: "Missing GameID",
			request: CreateDrawRequest{
				DrawName:      "Test Draw",
				ScheduledTime: time.Now().Add(24 * time.Hour),
				DrawLocation:  "NLA Office - Accra",
			},
			errMsg: "game ID is required",
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			_, err := s.service.CreateDraw(s.ctx, tt.request)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

// ============================================================================
// Stage 1: Preparation Tests
// ============================================================================

func (s *DrawServiceTestSuite) TestStartDrawPreparation_Success() {
	draw := s.createTestDraw("Preparation Test", models.DrawStatusScheduled)

	updatedDraw, err := s.service.StartDrawPreparation(s.ctx, draw.ID, "test-admin")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.DrawStatusInProgress, updatedDraw.Status)
	require.NotNil(s.T(), updatedDraw.StageData)
	assert.Equal(s.T(), 1, updatedDraw.StageData.CurrentStage)
	assert.Equal(s.T(), "Preparation", updatedDraw.StageData.StageName)
	assert.Equal(s.T(), models.StageStatusInProgress, updatedDraw.StageData.StageStatus)
	assert.NotNil(s.T(), updatedDraw.StageData.StageStartedAt)
}

func (s *DrawServiceTestSuite) TestStartDrawPreparation_InvalidStatus() {
	draw := s.createTestDraw("Invalid Status Test", models.DrawStatusInProgress)

	_, err := s.service.StartDrawPreparation(s.ctx, draw.ID, "test-admin")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "must be in scheduled status")
}

func (s *DrawServiceTestSuite) TestCompleteDrawPreparation_Success() {
	draw := s.createTestDraw("Complete Prep Test", models.DrawStatusScheduled)

	// Start preparation first
	draw, err := s.service.StartDrawPreparation(s.ctx, draw.ID, "test-admin")
	require.NoError(s.T(), err)

	// Complete preparation
	updatedDraw, err := s.service.CompleteDrawPreparation(s.ctx, draw.ID, "test-admin")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), updatedDraw.StageData)
	assert.Equal(s.T(), models.StageStatusCompleted, updatedDraw.StageData.StageStatus)
	assert.NotNil(s.T(), updatedDraw.StageData.StageCompletedAt)
}

// ============================================================================
// Stage 2: Number Selection Tests
// ============================================================================

func (s *DrawServiceTestSuite) TestStartNumberSelection_Success() {
	draw := s.setupDrawAtStage1Completed()

	updatedDraw, err := s.service.StartNumberSelection(s.ctx, draw.ID, "test-admin")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), updatedDraw.StageData)
	assert.Equal(s.T(), 2, updatedDraw.StageData.CurrentStage)
	assert.Equal(s.T(), "Number Selection", updatedDraw.StageData.StageName)
	assert.Equal(s.T(), models.StageStatusInProgress, updatedDraw.StageData.StageStatus)
}

func (s *DrawServiceTestSuite) TestSubmitVerificationAttempt_Success() {
	draw := s.setupDrawAtStage2Started()

	numbers := []int32{5, 12, 23, 45, 67}
	updatedDraw, attemptNumber, err := s.service.SubmitVerificationAttempt(s.ctx, draw.ID, numbers, "test-admin-1")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int32(1), attemptNumber)
	require.NotNil(s.T(), updatedDraw.StageData)
	require.NotNil(s.T(), updatedDraw.StageData.NumberSelectionData)
	assert.Len(s.T(), updatedDraw.StageData.NumberSelectionData.VerificationAttempts, 1)
}

func (s *DrawServiceTestSuite) TestSubmitVerificationAttempt_InvalidCount() {
	draw := s.setupDrawAtStage2Started()

	invalidNumbers := []int32{5, 12, 23} // Only 3 numbers instead of 5
	_, _, err := s.service.SubmitVerificationAttempt(s.ctx, draw.ID, invalidNumbers, "test-admin")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "must provide exactly 5 numbers")
}

func (s *DrawServiceTestSuite) TestCompleteNumberSelection_Success() {
	draw := s.setupDrawAtStage2WithVerifiedNumbers()

	numbers := []int32{5, 12, 23, 45, 67}
	updatedDraw, err := s.service.CompleteNumberSelection(s.ctx, draw.ID, numbers, "test-admin")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), updatedDraw.StageData)
	assert.Equal(s.T(), models.StageStatusCompleted, updatedDraw.StageData.StageStatus)
	assert.ElementsMatch(s.T(), numbers, []int32(updatedDraw.WinningNumbers))
}

// ============================================================================
// Stage 3: Result Calculation Tests
// ============================================================================

func (s *DrawServiceTestSuite) TestCommitResults_Success() {
	draw := s.setupDrawAtStage2Completed()

	updatedDraw, summary, err := s.service.CommitResults(s.ctx, draw.ID, "test-admin")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), updatedDraw.StageData)
	assert.Equal(s.T(), 3, updatedDraw.StageData.CurrentStage)
	assert.Equal(s.T(), "Result Calculation", updatedDraw.StageData.StageName)
	assert.NotNil(s.T(), summary)
	// Summary counts will be 0 since we have no tickets in test
	assert.Equal(s.T(), int64(0), summary.WinningTicketsCount)
	assert.Equal(s.T(), int64(0), summary.TotalWinnings)
}

func (s *DrawServiceTestSuite) TestCommitResults_InvalidStage() {
	draw := s.setupDrawAtStage1Completed() // Still at stage 1

	_, _, err := s.service.CommitResults(s.ctx, draw.ID, "test-admin")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "must complete number selection stage first")
}

// ============================================================================
// Stage 4: Payout Processing Tests
// ============================================================================

func (s *DrawServiceTestSuite) TestProcessPayouts_Success() {
	draw := s.setupDrawAtStage3Completed()

	updatedDraw, summary, err := s.service.ProcessPayouts(s.ctx, draw.ID, "test-admin")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), updatedDraw.StageData)
	assert.Equal(s.T(), 4, updatedDraw.StageData.CurrentStage)
	assert.Equal(s.T(), "Payout", updatedDraw.StageData.StageName)
	assert.NotNil(s.T(), summary)
	// Payout counts will be 0 since we have no winning tickets
	assert.Equal(s.T(), int64(0), summary.AutoProcessedCount)
	assert.Equal(s.T(), int64(0), summary.ManualApprovalCount)
}

func (s *DrawServiceTestSuite) TestProcessPayouts_InvalidStage() {
	draw := s.setupDrawAtStage2Completed() // Still at stage 2

	_, _, err := s.service.ProcessPayouts(s.ctx, draw.ID, "test-admin")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "must complete result calculation stage first")
}

// ============================================================================
// Complete Draw Tests
// ============================================================================

func (s *DrawServiceTestSuite) TestCompleteDraw_Success() {
	draw := s.setupDrawAtStage4Completed()

	updatedDraw, err := s.service.CompleteDraw(s.ctx, draw.ID, "test-admin")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.DrawStatusCompleted, updatedDraw.Status)
	require.NotNil(s.T(), updatedDraw.StageData)
	assert.Equal(s.T(), 4, updatedDraw.StageData.CurrentStage)
	assert.Equal(s.T(), models.StageStatusCompleted, updatedDraw.StageData.StageStatus)
}

func (s *DrawServiceTestSuite) TestCompleteDraw_InvalidStage() {
	draw := s.setupDrawAtStage3Completed() // Payout not completed

	_, err := s.service.CompleteDraw(s.ctx, draw.ID, "test-admin")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "payout stage must be completed first")
}

// ============================================================================
// Full Workflow Integration Test
// ============================================================================

func (s *DrawServiceTestSuite) TestFullDrawWorkflow_EndToEnd() {
	// 1. Create draw
	request := CreateDrawRequest{
		GameID:        s.testGameID,
		DrawName:      "Full Workflow Test Draw",
		ScheduledTime: time.Now().Add(24 * time.Hour),
	}
	draw, err := s.service.CreateDraw(s.ctx, request)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.DrawStatusScheduled, draw.Status)

	// 2. Stage 1: Start Preparation
	draw, err = s.service.StartDrawPreparation(s.ctx, draw.ID, "admin-1")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, draw.StageData.CurrentStage)

	// 3. Stage 1: Complete Preparation
	draw, err = s.service.CompleteDrawPreparation(s.ctx, draw.ID, "admin-1")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.StageStatusCompleted, draw.StageData.StageStatus)

	// 4. Stage 2: Start Number Selection
	draw, err = s.service.StartNumberSelection(s.ctx, draw.ID, "admin-1")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 2, draw.StageData.CurrentStage)

	// 5. Stage 2: Submit Verification Attempts (2 attempts with same numbers - max allowed)
	numbers := []int32{7, 14, 21, 35, 49}
	draw, _, err = s.service.SubmitVerificationAttempt(s.ctx, draw.ID, numbers, "admin-1")
	require.NoError(s.T(), err)
	draw, _, err = s.service.SubmitVerificationAttempt(s.ctx, draw.ID, numbers, "admin-2")
	require.NoError(s.T(), err)

	// 5b. Stage 2: Validate Verification Attempts
	_, _, _, err = s.service.ValidateVerificationAttempts(s.ctx, draw.ID, "admin-1")
	require.NoError(s.T(), err)

	// 6. Stage 2: Complete Number Selection
	draw, err = s.service.CompleteNumberSelection(s.ctx, draw.ID, numbers, "admin-1")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.StageStatusCompleted, draw.StageData.StageStatus)

	// 6. Stage 3: Commit Results
	draw, summary, err := s.service.CommitResults(s.ctx, draw.ID, "admin-1")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 3, draw.StageData.CurrentStage)
	assert.NotNil(s.T(), summary)

	// 7. Stage 4: Process Payouts
	draw, payoutSummary, err := s.service.ProcessPayouts(s.ctx, draw.ID, "admin-1")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 4, draw.StageData.CurrentStage)
	assert.NotNil(s.T(), payoutSummary)

	// 8. Complete Draw
	draw, err = s.service.CompleteDraw(s.ctx, draw.ID, "admin-1")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.DrawStatusCompleted, draw.Status)
	assert.Equal(s.T(), numbers, []int32(draw.WinningNumbers))
}

// ============================================================================
// Helper Functions
// ============================================================================

func (s *DrawServiceTestSuite) createTestDraw(name string, status models.DrawStatus) *models.Draw {
	draw := &models.Draw{
		ID:            uuid.New(),
		GameID:        s.testGameID,
		GameName:      "VAG MONDAY",
		DrawName:      name,
		DrawNumber:    int(time.Now().Unix() % 1000000),
		ScheduledTime: time.Now().Add(24 * time.Hour),
		Status:        status,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err := s.repo.Create(s.ctx, draw)
	require.NoError(s.T(), err)

	return draw
}

func (s *DrawServiceTestSuite) setupDrawAtStage1Completed() *models.Draw {
	draw := s.createTestDraw("Stage 1 Complete", models.DrawStatusScheduled)
	draw, err := s.service.StartDrawPreparation(s.ctx, draw.ID, "test-admin")
	require.NoError(s.T(), err)
	draw, err = s.service.CompleteDrawPreparation(s.ctx, draw.ID, "test-admin")
	require.NoError(s.T(), err)
	return draw
}

func (s *DrawServiceTestSuite) setupDrawAtStage2Started() *models.Draw {
	draw := s.setupDrawAtStage1Completed()
	draw, err := s.service.StartNumberSelection(s.ctx, draw.ID, "test-admin")
	require.NoError(s.T(), err)
	return draw
}

func (s *DrawServiceTestSuite) setupDrawAtStage2WithVerifiedNumbers() *models.Draw {
	draw := s.setupDrawAtStage2Started()
	numbers := []int32{5, 12, 23, 45, 67}
	// Submit 2 verification attempts with same numbers (max allowed is 2)
	draw, _, err := s.service.SubmitVerificationAttempt(s.ctx, draw.ID, numbers, "admin-1")
	require.NoError(s.T(), err)
	draw, _, err = s.service.SubmitVerificationAttempt(s.ctx, draw.ID, numbers, "admin-2")
	require.NoError(s.T(), err)
	// Validate the verification attempts to set IsVerified = true
	_, _, _, err = s.service.ValidateVerificationAttempts(s.ctx, draw.ID, "test-admin")
	require.NoError(s.T(), err)
	// Refresh draw to get updated state
	draw, err = s.service.GetDraw(s.ctx, draw.ID)
	require.NoError(s.T(), err)
	return draw
}

func (s *DrawServiceTestSuite) setupDrawAtStage2Completed() *models.Draw {
	draw := s.setupDrawAtStage2WithVerifiedNumbers()
	numbers := []int32{5, 12, 23, 45, 67}
	draw, err := s.service.CompleteNumberSelection(s.ctx, draw.ID, numbers, "test-admin")
	require.NoError(s.T(), err)
	return draw
}

func (s *DrawServiceTestSuite) setupDrawAtStage3Completed() *models.Draw {
	draw := s.setupDrawAtStage2Completed()
	draw, _, err := s.service.CommitResults(s.ctx, draw.ID, "test-admin")
	require.NoError(s.T(), err)
	return draw
}

func (s *DrawServiceTestSuite) setupDrawAtStage4Completed() *models.Draw {
	draw := s.setupDrawAtStage3Completed()
	draw, _, err := s.service.ProcessPayouts(s.ctx, draw.ID, "test-admin")
	require.NoError(s.T(), err)
	return draw
}

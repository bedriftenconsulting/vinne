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

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	walletpb "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/randco-microservices/services/service-ticket/internal/models"
	"github.com/randco/randco-microservices/services/service-ticket/internal/repositories"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
)

// MockWalletClient is a mock implementation of the wallet service client
type MockWalletClient struct {
	mock.Mock
}

func (m *MockWalletClient) CreatePlayerWallet(ctx context.Context, in *walletpb.CreatePlayerWalletRequest, opts ...grpc.CallOption) (*walletpb.CreatePlayerWalletResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.CreatePlayerWalletResponse), args.Error(1)
}

func (m *MockWalletClient) GetPlayerWalletBalance(ctx context.Context, in *walletpb.GetPlayerWalletBalanceRequest, opts ...grpc.CallOption) (*walletpb.GetPlayerWalletBalanceResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.GetPlayerWalletBalanceResponse), args.Error(1)
}

func (m *MockWalletClient) CreditPlayerWallet(ctx context.Context, in *walletpb.CreditPlayerWalletRequest, opts ...grpc.CallOption) (*walletpb.CreditPlayerWalletResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.CreditPlayerWalletResponse), args.Error(1)
}

func (m *MockWalletClient) DebitPlayerWallet(ctx context.Context, in *walletpb.DebitPlayerWalletRequest, opts ...grpc.CallOption) (*walletpb.DebitPlayerWalletResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.DebitPlayerWalletResponse), args.Error(1)
}

func (m *MockWalletClient) ReservePlayerWalletFunds(ctx context.Context, in *walletpb.ReservePlayerWalletFundsRequest, opts ...grpc.CallOption) (*walletpb.ReservePlayerWalletFundsResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.ReservePlayerWalletFundsResponse), args.Error(1)
}
func (m *MockWalletClient) DebitRetailerWallet(ctx context.Context, in *walletpb.DebitRetailerWalletRequest, opts ...grpc.CallOption) (*walletpb.DebitRetailerWalletResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.DebitRetailerWalletResponse), args.Error(1)
}

func (m *MockWalletClient) CreateAgentWallet(ctx context.Context, in *walletpb.CreateAgentWalletRequest, opts ...grpc.CallOption) (*walletpb.CreateAgentWalletResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.CreateAgentWalletResponse), args.Error(1)
}

func (m *MockWalletClient) CreateRetailerWallets(ctx context.Context, in *walletpb.CreateRetailerWalletsRequest, opts ...grpc.CallOption) (*walletpb.CreateRetailerWalletsResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.CreateRetailerWalletsResponse), args.Error(1)
}

func (m *MockWalletClient) CreditAgentWallet(ctx context.Context, in *walletpb.CreditAgentWalletRequest, opts ...grpc.CallOption) (*walletpb.CreditAgentWalletResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.CreditAgentWalletResponse), args.Error(1)
}

func (m *MockWalletClient) CreditRetailerWallet(ctx context.Context, in *walletpb.CreditRetailerWalletRequest, opts ...grpc.CallOption) (*walletpb.CreditRetailerWalletResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.CreditRetailerWalletResponse), args.Error(1)
}

func (m *MockWalletClient) GetAgentWalletBalance(ctx context.Context, in *walletpb.GetAgentWalletBalanceRequest, opts ...grpc.CallOption) (*walletpb.GetAgentWalletBalanceResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.GetAgentWalletBalanceResponse), args.Error(1)
}

func (m *MockWalletClient) GetRetailerWalletBalance(ctx context.Context, in *walletpb.GetRetailerWalletBalanceRequest, opts ...grpc.CallOption) (*walletpb.GetRetailerWalletBalanceResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.GetRetailerWalletBalanceResponse), args.Error(1)
}

func (m *MockWalletClient) TransferAgentToRetailer(ctx context.Context, in *walletpb.TransferAgentToRetailerRequest, opts ...grpc.CallOption) (*walletpb.TransferAgentToRetailerResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.TransferAgentToRetailerResponse), args.Error(1)
}

func (m *MockWalletClient) GetCommissionRate(ctx context.Context, in *walletpb.GetCommissionRateRequest, opts ...grpc.CallOption) (*walletpb.GetCommissionRateResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.GetCommissionRateResponse), args.Error(1)
}

func (m *MockWalletClient) GetCommissionReport(ctx context.Context, in *walletpb.GetCommissionReportRequest, opts ...grpc.CallOption) (*walletpb.GetCommissionReportResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.GetCommissionReportResponse), args.Error(1)
}

func (m *MockWalletClient) GetTransactionHistory(ctx context.Context, in *walletpb.GetTransactionHistoryRequest, opts ...grpc.CallOption) (*walletpb.GetTransactionHistoryResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.GetTransactionHistoryResponse), args.Error(1)
}

func (m *MockWalletClient) SetCommissionRate(ctx context.Context, in *walletpb.SetCommissionRateRequest, opts ...grpc.CallOption) (*walletpb.SetCommissionRateResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.SetCommissionRateResponse), args.Error(1)
}

func (m *MockWalletClient) UpdateAgentCommission(ctx context.Context, in *walletpb.UpdateAgentCommissionRequest, opts ...grpc.CallOption) (*walletpb.UpdateAgentCommissionResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.UpdateAgentCommissionResponse), args.Error(1)
}

func (m *MockWalletClient) CommitReservedDebit(ctx context.Context, in *walletpb.CommitReservedDebitRequest, opts ...grpc.CallOption) (*walletpb.CommitReservedDebitResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.CommitReservedDebitResponse), args.Error(1)
}

func (m *MockWalletClient) GetAllTransactions(ctx context.Context, in *walletpb.GetAllTransactionsRequest, opts ...grpc.CallOption) (*walletpb.GetAllTransactionsResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.GetAllTransactionsResponse), args.Error(1)
}

func (m *MockWalletClient) ReleaseReservation(ctx context.Context, in *walletpb.ReleaseReservationRequest, opts ...grpc.CallOption) (*walletpb.ReleaseReservationResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.ReleaseReservationResponse), args.Error(1)
}

func (m *MockWalletClient) ReserveRetailerWalletFunds(ctx context.Context, in *walletpb.ReserveRetailerWalletFundsRequest, opts ...grpc.CallOption) (*walletpb.ReserveRetailerWalletFundsResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.ReserveRetailerWalletFundsResponse), args.Error(1)
}

func (m *MockWalletClient) ReverseTransaction(ctx context.Context, in *walletpb.ReverseTransactionRequest, opts ...grpc.CallOption) (*walletpb.ReverseTransactionResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.ReverseTransactionResponse), args.Error(1)
}

func (m *MockWalletClient) GetDailyCommissions(ctx context.Context, in *walletpb.GetDailyCommissionsRequest, opts ...grpc.CallOption) (*walletpb.GetDailyCommissionsResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.GetDailyCommissionsResponse), args.Error(1)
}

func (m *MockWalletClient) PlaceHoldOnWallet(ctx context.Context, in *walletpb.PlaceHoldOnWalletRequest, opts ...grpc.CallOption) (*walletpb.PlaceHoldOnWalletResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.PlaceHoldOnWalletResponse), args.Error(1)
}

func (m *MockWalletClient) GetHoldByRetailer(ctx context.Context, in *walletpb.GetHoldByRetailerRequest, opts ...grpc.CallOption) (*walletpb.GetHoldByRetailerResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.GetHoldByRetailerResponse), args.Error(1)
}

func (m *MockWalletClient) GetHoldOnWallet(ctx context.Context, in *walletpb.GetHoldOnWalletRequest, opts ...grpc.CallOption) (*walletpb.GetHoldOnWalletResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.GetHoldOnWalletResponse), args.Error(1)
}

func (m *MockWalletClient) ReleaseHoldOnWallet(ctx context.Context, in *walletpb.ReleaseHoldOnWalletRequest, opts ...grpc.CallOption) (*walletpb.ReleaseHoldOnWalletResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*walletpb.ReleaseHoldOnWalletResponse), args.Error(1)
}

// Note: We don't mock AgentManagementClient in these integration tests
// as the existing tests don't exercise the top performing agents functionality
// that requires cross-service calls

// TicketServiceIntegrationTestSuite tests the Ticket service with real infrastructure
type TicketServiceIntegrationTestSuite struct {
	suite.Suite
	// Infrastructure
	pgContainer *postgres.PostgresContainer
	db          *sql.DB

	// Repositories and Services
	ticketRepo    repositories.TicketRepository
	walletClient  *MockWalletClient
	ticketService TicketService

	// Test context
	ctx    context.Context
	cancel context.CancelFunc
}

// SetupSuite initializes the test infrastructure using TestContainers
func (suite *TicketServiceIntegrationTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(suite.ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("ticket_service_test"),
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

	// Run database migrations
	err = suite.runMigrations(connStr)
	suite.Require().NoError(err)

	// Initialize repositories and services
	suite.ticketRepo = repositories.NewTicketRepository(suite.db)
	suite.walletClient = new(MockWalletClient)
	suite.ticketService = NewTicketService(
		suite.db,
		suite.ticketRepo,
		suite.walletClient,
		nil, // Agent Management client not needed for basic integration tests
		nil, // Game client not needed for basic integration tests
		nil, // Redis client not needed for basic integration tests
		&ServiceConfig{
			Security: SecurityConfig{
				SecretKey:     "test-secret-key-for-integration-tests",
				SerialPrefix:  "TKT",
				QRCodeBaseURL: "https://verify.test.com",
				BarcodePrefix: "TKT",
			},
		},
	)
}

// TearDownSuite cleans up all test infrastructure
func (suite *TicketServiceIntegrationTestSuite) TearDownSuite() {
	if suite.db != nil {
		_ = suite.db.Close()
	}
	if suite.pgContainer != nil {
		_ = suite.pgContainer.Terminate(suite.ctx)
	}
	suite.cancel()
}

// SetupTest runs before each test method
func (suite *TicketServiceIntegrationTestSuite) SetupTest() {
	suite.cleanupDatabase()
}

// runMigrations runs the Goose migrations
func (suite *TicketServiceIntegrationTestSuite) runMigrations(connStr string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	migrationsDir := ""
	for i := 0; i < 10; i++ {
		testDir := filepath.Join(wd, "migrations")
		if _, err := os.Stat(testDir); err == nil {
			migrationsDir = testDir
			break
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			break
		}
		wd = parent
	}

	if migrationsDir == "" {
		return fmt.Errorf("migrations directory not found")
	}

	cmd := exec.Command("goose", "-dir", migrationsDir, "postgres", connStr, "up")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// cleanupDatabase removes all test data
func (suite *TicketServiceIntegrationTestSuite) cleanupDatabase() {
	tables := []string{
		"ticket_validations",
		"ticket_reprints",
		"ticket_voids",
		"ticket_cancellations",
		"ticket_payments",
		"tickets",
	}

	for _, table := range tables {
		_, err := suite.db.ExecContext(suite.ctx, "DELETE FROM "+table)
		suite.Require().NoError(err)
	}
}

// TestIssueTicket tests ticket issuance
func (suite *TicketServiceIntegrationTestSuite) TestIssueTicket() {
	req := IssueTicketRequest{
		GameCode:        "5BY90",
		DrawNumber:      100,
		SelectedNumbers: []int32{5, 12, 23, 34, 45},
		BetLines: []models.BetLine{
			{
				LineNumber: 1,
				BetType:    "straight",
				Numbers:    []int32{5, 12, 23, 34, 45},
				Amount:     200,
			},
		},
		IssuerType:    string(models.IssuerTypePOS),
		IssuerID:      uuid.New().String(),
		PaymentMethod: string(models.PaymentMethodCash),
	}

	ticket, err := suite.ticketService.IssueTicket(suite.ctx, req)
	suite.Require().NoError(err)
	suite.NotEmpty(ticket.ID)
	suite.NotEmpty(ticket.SerialNumber)
	suite.Equal(req.GameCode, ticket.GameCode)
	suite.Equal(string(models.TicketStatusIssued), ticket.Status)
}

// TestGetTicketBySerial tests retrieval by serial number
func (suite *TicketServiceIntegrationTestSuite) TestGetTicketBySerial() {
	ticket, err := suite.issueTestTicket()
	suite.Require().NoError(err)

	retrieved, err := suite.ticketService.GetTicketBySerial(suite.ctx, ticket.SerialNumber)
	suite.Require().NoError(err)
	suite.Equal(ticket.ID, retrieved.ID)
}

// TestListTickets tests listing with filters
func (suite *TicketServiceIntegrationTestSuite) TestListTickets() {
	_, err := suite.issueTestTicket()
	suite.Require().NoError(err)

	tickets, total, err := suite.ticketService.ListTickets(suite.ctx, models.TicketFilter{}, 1, 10)
	suite.Require().NoError(err)
	suite.Equal(int64(1), total)
	suite.Len(tickets, 1)
}

// issueTestTicket helper
func (suite *TicketServiceIntegrationTestSuite) issueTestTicket() (*models.Ticket, error) {
	req := IssueTicketRequest{
		GameCode:        "5BY90",
		DrawNumber:      100,
		SelectedNumbers: []int32{5, 12, 23, 34, 45},
		BetLines: []models.BetLine{
			{
				LineNumber: 1,
				BetType:    "straight",
				Numbers:    []int32{5, 12, 23, 34, 45},
				Amount:     200,
			},
		},
		IssuerType:    string(models.IssuerTypePOS),
		IssuerID:      uuid.New().String(),
		PaymentMethod: string(models.PaymentMethodCash),
	}

	return suite.ticketService.IssueTicket(suite.ctx, req)
}

// Run the integration test suite
func TestTicketServiceIntegrationSuite(t *testing.T) {
	suite.Run(t, new(TicketServiceIntegrationTestSuite))
}

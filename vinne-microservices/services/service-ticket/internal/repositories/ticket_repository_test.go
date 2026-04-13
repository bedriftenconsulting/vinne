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
	"github.com/randco/randco-microservices/services/service-ticket/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TicketRepositoryTestSuite struct {
	suite.Suite
	db        *sql.DB
	repo      TicketRepository
	container testcontainers.Container
	ctx       context.Context
}

func TestTicketRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(TicketRepositoryTestSuite))
}

func (s *TicketRepositoryTestSuite) SetupSuite() {
	s.ctx = context.Background()

	// Start PostgreSQL testcontainer
	postgresContainer, err := postgres.Run(s.ctx, "postgres:17-alpine",
		postgres.WithDatabase("service_ticket_test"),
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
	s.repo = NewTicketRepository(s.db)
}

func (s *TicketRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(s.T(), err)
	}
}

func (s *TicketRepositoryTestSuite) SetupTest() {
	// Clean up tables before each test
	_, err := s.db.Exec(`
		TRUNCATE tickets, ticket_payments, ticket_cancellations,
		ticket_voids, ticket_reprints, ticket_validations
		RESTART IDENTITY CASCADE
	`)
	require.NoError(s.T(), err)
}

func (s *TicketRepositoryTestSuite) runMigrations() error {
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

func (s *TicketRepositoryTestSuite) TestCreate() {
	ticket := s.createTestTicket()

	err := s.repo.Create(s.ctx, ticket)
	require.NoError(s.T(), err)

	// Verify ticket was created
	assert.NotEqual(s.T(), uuid.Nil, ticket.ID)
	assert.NotZero(s.T(), ticket.CreatedAt)
	assert.NotZero(s.T(), ticket.UpdatedAt)

	// Verify ticket can be retrieved
	retrieved, err := s.repo.GetByID(s.ctx, ticket.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), ticket.SerialNumber, retrieved.SerialNumber)
	assert.Equal(s.T(), ticket.GameCode, retrieved.GameCode)
	assert.Equal(s.T(), ticket.DrawNumber, retrieved.DrawNumber)
	assert.Equal(s.T(), ticket.TotalAmount, retrieved.TotalAmount)
	assert.Equal(s.T(), ticket.Status, retrieved.Status)
	assert.Len(s.T(), retrieved.BetLines, len(ticket.BetLines))
}

func (s *TicketRepositoryTestSuite) TestGetByID() {
	// Create test ticket
	ticket := s.createTestTicket()
	err := s.repo.Create(s.ctx, ticket)
	require.NoError(s.T(), err)

	// Retrieve by ID
	retrieved, err := s.repo.GetByID(s.ctx, ticket.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), ticket.ID, retrieved.ID)
	assert.Equal(s.T(), ticket.SerialNumber, retrieved.SerialNumber)

	// Test with non-existent ID
	_, err = s.repo.GetByID(s.ctx, uuid.New())
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "ticket not found")
}

func (s *TicketRepositoryTestSuite) TestGetBySerial() {
	// Create test ticket
	ticket := s.createTestTicket()
	err := s.repo.Create(s.ctx, ticket)
	require.NoError(s.T(), err)

	// Retrieve by serial number
	retrieved, err := s.repo.GetBySerial(s.ctx, ticket.SerialNumber)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), ticket.ID, retrieved.ID)
	assert.Equal(s.T(), ticket.SerialNumber, retrieved.SerialNumber)

	// Test with non-existent serial
	_, err = s.repo.GetBySerial(s.ctx, "INVALID-SERIAL")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "ticket not found")
}

func (s *TicketRepositoryTestSuite) TestUpdate() {
	// Create test ticket
	ticket := s.createTestTicket()
	err := s.repo.Create(s.ctx, ticket)
	require.NoError(s.T(), err)

	// Update ticket
	ticket.Status = string(models.TicketStatusValidated)
	ticket.WinningAmount = 50000 // 500 GHS in pesewas

	err = s.repo.Update(s.ctx, ticket)
	require.NoError(s.T(), err)

	// Verify update
	retrieved, err := s.repo.GetByID(s.ctx, ticket.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), string(models.TicketStatusValidated), retrieved.Status)
	assert.Equal(s.T(), int64(50000), retrieved.WinningAmount)
}

func (s *TicketRepositoryTestSuite) TestList() {
	// Create multiple test tickets
	ticket1 := s.createTestTicket()
	ticket1.GameCode = "GAME-001"
	err := s.repo.Create(s.ctx, ticket1)
	require.NoError(s.T(), err)

	ticket2 := s.createTestTicket()
	ticket2.SerialNumber = "TKT-000002"
	ticket2.GameCode = "GAME-002"
	err = s.repo.Create(s.ctx, ticket2)
	require.NoError(s.T(), err)

	ticket3 := s.createTestTicket()
	ticket3.SerialNumber = "TKT-000003"
	ticket3.GameCode = "GAME-001"
	ticket3.Status = string(models.TicketStatusValidated)
	err = s.repo.Create(s.ctx, ticket3)
	require.NoError(s.T(), err)

	// Test list without filter
	tickets, total, err := s.repo.List(s.ctx, models.TicketFilter{}, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), total)
	assert.Len(s.T(), tickets, 3)

	// Test with status filter
	statusFilter := string(models.TicketStatusIssued)
	tickets, total, err = s.repo.List(s.ctx, models.TicketFilter{Status: &statusFilter}, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), total)
	assert.Len(s.T(), tickets, 2)

	// Test with game code filter
	gameCode := "GAME-001"
	tickets, total, err = s.repo.List(s.ctx, models.TicketFilter{GameCode: &gameCode}, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), total)
	assert.Len(s.T(), tickets, 2)

	// Test pagination
	tickets, total, err = s.repo.List(s.ctx, models.TicketFilter{}, 1, 2)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), total)
	assert.Len(s.T(), tickets, 2)

	tickets, total, err = s.repo.List(s.ctx, models.TicketFilter{}, 2, 2)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), total)
	assert.Len(s.T(), tickets, 1)
}

func (s *TicketRepositoryTestSuite) TestGetByGameAndDraw() {
	// Create tickets for different games and draws
	ticket1 := s.createTestTicket()
	ticket1.GameCode = "GAME-001"
	ticket1.DrawNumber = 100
	err := s.repo.Create(s.ctx, ticket1)
	require.NoError(s.T(), err)

	ticket2 := s.createTestTicket()
	ticket2.SerialNumber = "TKT-000002"
	ticket2.GameCode = "GAME-001"
	ticket2.DrawNumber = 100
	err = s.repo.Create(s.ctx, ticket2)
	require.NoError(s.T(), err)

	ticket3 := s.createTestTicket()
	ticket3.SerialNumber = "TKT-000003"
	ticket3.GameCode = "GAME-001"
	ticket3.DrawNumber = 101
	err = s.repo.Create(s.ctx, ticket3)
	require.NoError(s.T(), err)

	// Get tickets for GAME-001, draw 100
	tickets, err := s.repo.GetByGameAndDraw(s.ctx, "GAME-001", 100)
	require.NoError(s.T(), err)
	assert.Len(s.T(), tickets, 2)

	// Get tickets for GAME-001, draw 101
	tickets, err = s.repo.GetByGameAndDraw(s.ctx, "GAME-001", 101)
	require.NoError(s.T(), err)
	assert.Len(s.T(), tickets, 1)
}

func (s *TicketRepositoryTestSuite) TestCreateWithTx() {
	// Start a transaction
	tx, err := s.db.BeginTx(s.ctx, nil)
	require.NoError(s.T(), err)

	// Create ticket within transaction
	ticket := s.createTestTicket()
	err = s.repo.CreateWithTx(s.ctx, tx, ticket)
	require.NoError(s.T(), err)

	// Rollback transaction
	err = tx.Rollback()
	require.NoError(s.T(), err)

	// Verify ticket was not persisted
	_, err = s.repo.GetByID(s.ctx, ticket.ID)
	assert.Error(s.T(), err)

	// Try again with commit
	tx, err = s.db.BeginTx(s.ctx, nil)
	require.NoError(s.T(), err)

	ticket2 := s.createTestTicket()
	ticket2.SerialNumber = "TKT-000002"
	err = s.repo.CreateWithTx(s.ctx, tx, ticket2)
	require.NoError(s.T(), err)

	err = tx.Commit()
	require.NoError(s.T(), err)

	// Verify ticket was persisted
	retrieved, err := s.repo.GetByID(s.ctx, ticket2.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), ticket2.SerialNumber, retrieved.SerialNumber)
}

func (s *TicketRepositoryTestSuite) TestJSONBFields() {
	// Create ticket with complex JSONB fields
	ticket := s.createTestTicket()

	// Add multiple bet lines
	ticket.BetLines = []models.BetLine{
		{
			LineNumber: 1,
			BetType:    "straight",
			Numbers:    []int32{5, 12, 23, 34, 45},
			Amount:     100, // 1 GHS in pesewas
		},
		{
			LineNumber: 2,
			BetType:    "perm",
			Banker:     []int32{7, 14},
			Opposed:    []int32{21, 28, 35, 42},
			Amount:     200, // 2 GHS in pesewas
		},
	}

	// Complex issuer details
	terminalID := "TERM-001"
	agentCode := "OP-123"
	ticket.IssuerDetails = &models.IssuerDetails{
		TerminalID: &terminalID,
		AgentCode:  &agentCode,
	}

	err := s.repo.Create(s.ctx, ticket)
	require.NoError(s.T(), err)

	// Retrieve and verify JSONB fields
	retrieved, err := s.repo.GetByID(s.ctx, ticket.ID)
	require.NoError(s.T(), err)

	// Verify bet lines
	assert.Len(s.T(), retrieved.BetLines, 2)
	assert.Equal(s.T(), "straight", retrieved.BetLines[0].BetType)
	assert.Equal(s.T(), []int32{5, 12, 23, 34, 45}, retrieved.BetLines[0].Numbers)
	assert.Equal(s.T(), "perm", retrieved.BetLines[1].BetType)
	assert.Equal(s.T(), []int32{7, 14}, retrieved.BetLines[1].Banker)

	// Verify issuer details
	require.NotNil(s.T(), retrieved.IssuerDetails)
	assert.Equal(s.T(), "TERM-001", *retrieved.IssuerDetails.TerminalID)
	assert.Equal(s.T(), "OP-123", *retrieved.IssuerDetails.AgentCode)
}

// Helper functions

func (s *TicketRepositoryTestSuite) createTestTicket() *models.Ticket {
	now := time.Now()
	drawDate := now.AddDate(0, 0, 1) // Tomorrow

	betLines := []models.BetLine{
		{
			LineNumber: 1,
			BetType:    "straight",
			Numbers:    []int32{5, 12, 23, 34, 45},
			Amount:     100, // 1 GHS in pesewas
		},
	}

	terminalID := "TERM-001"
	issuerDetails := &models.IssuerDetails{
		TerminalID: &terminalID,
	}

	securityFeatures := &models.SecurityFeatures{
		QRCode:           "https://verify.example.com/TKT-000001",
		Barcode:          "TKT000001",
		VerificationCode: "123456",
	}

	gameScheduleID := uuid.New()
	paymentMethod := string(models.PaymentMethodCash)

	return &models.Ticket{
		SerialNumber:     "TKT-000001",
		GameCode:         "5BY90",
		GameScheduleID:   &gameScheduleID,
		DrawNumber:       100,
		DrawDate:         &drawDate,
		SelectedNumbers:  []int32{5, 12, 23, 34, 45},
		BetLines:         betLines,
		IssuerType:       string(models.IssuerTypePOS),
		IssuerID:         uuid.New().String(),
		IssuerDetails:    issuerDetails,
		CustomerPhone:    stringPtr("+233240000000"),
		PaymentMethod:    &paymentMethod,
		UnitPrice:        100,
		TotalAmount:      100,
		SecurityHash:     "abc123hash",
		SecurityFeatures: securityFeatures,
		Status:           string(models.TicketStatusIssued),
	}
}

func stringPtr(s string) *string {
	return &s
}

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/randco/randco-microservices/services/service-terminal/internal/models"
	"github.com/randco/randco-microservices/services/service-terminal/internal/repositories"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TerminalServiceTestSuite struct {
	suite.Suite
	db              *sqlx.DB
	redisClient     *redis.Client
	pgContainer     testcontainers.Container
	redisContainer  testcontainers.Container
	terminalService TerminalService
	logger          logger.Logger
}

func TestTerminalServiceTestSuite(t *testing.T) {
	suite.Run(t, new(TerminalServiceTestSuite))
}

func (s *TerminalServiceTestSuite) SetupSuite() {
	ctx := context.Background()

	// Initialize logger
	s.logger = logger.NewLogger(logger.Config{
		Level:       "debug",
		Format:      "json",
		ServiceName: "terminal-service-test",
	})

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("terminal_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(s.T(), err)
	s.pgContainer = pgContainer

	// Get PostgreSQL connection string
	dbHost, err := pgContainer.Host(ctx)
	require.NoError(s.T(), err)
	dbPort, err := pgContainer.MappedPort(ctx, "5432")
	require.NoError(s.T(), err)

	dsn := fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=terminal_test sslmode=disable",
		dbHost, dbPort.Port())

	// Connect to database
	db, err := sqlx.ConnectContext(ctx, "postgres", dsn)
	require.NoError(s.T(), err)
	s.db = db

	// Run migrations
	runMigrations(s.T(), db)

	// Start Redis container
	redisContainer, err := tcredis.Run(ctx,
		"redis:7.4-alpine",
		tcredis.WithSnapshotting(10, 1),
		tcredis.WithLogLevel(tcredis.LogLevelVerbose),
	)
	require.NoError(s.T(), err)
	s.redisContainer = redisContainer

	// Get Redis connection
	redisHost, err := redisContainer.Host(ctx)
	require.NoError(s.T(), err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(s.T(), err)

	// Connect to Redis
	s.redisClient = redis.NewClient(&redis.Options{
		Addr: redisHost + ":" + redisPort.Port(),
		DB:   0,
	})

	// Initialize repositories
	terminalRepo := repositories.NewTerminalRepository(s.db)
	assignmentRepo := repositories.NewTerminalAssignmentRepository(s.db)
	healthRepo := repositories.NewTerminalHealthRepository(s.db)
	configRepo := repositories.NewTerminalConfigRepository(s.db)

	// Initialize terminal service
	s.terminalService = NewTerminalService(terminalRepo, assignmentRepo, healthRepo, configRepo, s.logger)
}

func runMigrations(t *testing.T, db *sqlx.DB) {
	// Set Goose dialect
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("Failed to set goose dialect: %v", err)
	}

	// Get the migrations directory path relative to the test file
	migrationsDir := filepath.Join("..", "..", "migrations")

	// Convert sqlx.DB to sql.DB for Goose
	sqlDB := db.DB

	// Run up migrations
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	t.Log("✅ Successfully ran all migrations using Goose")
}

func (s *TerminalServiceTestSuite) TearDownSuite() {
	ctx := context.Background()
	if s.pgContainer != nil {
		_ = s.pgContainer.Terminate(ctx)
	}
	if s.redisContainer != nil {
		_ = s.redisContainer.Terminate(ctx)
	}
}

func (s *TerminalServiceTestSuite) SetupTest() {
	// Clean database between tests
	s.db.Exec("TRUNCATE TABLE terminals, terminal_assignments, terminal_versions, terminal_configs, terminal_health, terminal_health_history, terminal_audit_logs CASCADE")

	// Clear Redis
	ctx := context.Background()
	s.redisClient.FlushDB(ctx)
}

func generateTestIMEI() string {
	// Generate a unique 16-20 char IMEI-safe string
	return fmt.Sprintf("IMEI-%d", time.Now().UnixNano()%1_000_000_000_000)
}

// Test Terminal Registration
func (s *TerminalServiceTestSuite) TestRegisterTerminal() {
	ctx := context.Background()

	tests := []struct {
		name     string
		terminal *models.Terminal
		wantErr  bool
	}{
		{
			name: "successful registration",
			terminal: &models.Terminal{
				DeviceID:       "POS-2025-000001",
				Name:           "Test Terminal 1",
				Model:          models.TerminalModelAndroidPOSV1,
				SerialNumber:   "SN123456789",
				IMEI:           "123456789012345",
				AndroidVersion: "11.0",
				AppVersion:     "1.0.0",
				Vendor:         "TestVendor",
				Status:         models.TerminalStatusInactive,
			},
			wantErr: false,
		},
		{
			name: "duplicate device ID",
			terminal: &models.Terminal{
				DeviceID:     "POS-2025-000001", // Same as first
				Name:         "Test Terminal 2",
				Model:        models.TerminalModelAndroidPOSV2,
				SerialNumber: "SN987654321",
			},
			wantErr: true,
		},
		{
			name: "different model terminal",
			terminal: &models.Terminal{
				DeviceID:     "WEB-2025-000001",
				Name:         "Web Terminal 1",
				Model:        models.TerminalModelWebTerminal,
				SerialNumber: "WEB-SN-001",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			err := s.terminalService.RegisterTerminal(ctx, tt.terminal)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, tt.terminal.ID)

				// Verify terminal was created
				retrieved, err := s.terminalService.GetTerminal(ctx, tt.terminal.ID)
				assert.NoError(t, err)
				assert.Equal(t, tt.terminal.DeviceID, retrieved.DeviceID)
				assert.Equal(t, tt.terminal.Name, retrieved.Name)

				// Verify config was created
				var config models.TerminalConfig
				var settingsJSON []byte

				query := `SELECT id, terminal_id, transaction_limit, daily_limit, offline_mode_enabled,
          offline_sync_interval, auto_update_enabled, minimum_app_version, settings,
          created_at, updated_at
          FROM terminal_configs
          WHERE terminal_id = $1
          LIMIT 1`

				err = s.db.QueryRowx(query, tt.terminal.ID).Scan(
					&config.ID,
					&config.TerminalID,
					&config.TransactionLimit,
					&config.DailyLimit,
					&config.OfflineModeEnabled,
					&config.OfflineSyncInterval,
					&config.AutoUpdateEnabled,
					&config.MinimumAppVersion,
					&settingsJSON,
					&config.CreatedAt,
					&config.UpdatedAt,
				)
				require.NoError(t, err)

				// Unmarshal JSON into the map
				err = json.Unmarshal(settingsJSON, &config.Settings)
				require.NoError(t, err)
				assert.NoError(t, err)
				assert.Equal(t, 10000, config.TransactionLimit)
				assert.Equal(t, 100000, config.DailyLimit)
			}
		})
	}
}

// Test Terminal Assignment to Retailer
func (s *TerminalServiceTestSuite) TestAssignTerminalToRetailer() {
	ctx := context.Background()

	// Register terminals
	terminal1 := &models.Terminal{
		DeviceID:     "POS-2025-000010",
		Name:         "Terminal for Retailer 1",
		Model:        models.TerminalModelAndroidPOSV1,
		IMEI:         generateTestIMEI(),
		SerialNumber: "SN-ASSIGN-001",
		Status:       models.TerminalStatusInactive,
	}
	err := s.terminalService.RegisterTerminal(ctx, terminal1)
	require.NoError(s.T(), err)

	terminal2 := &models.Terminal{
		DeviceID:     "POS-2025-000011",
		Name:         "Terminal for Retailer 2",
		Model:        models.TerminalModelAndroidPOSV1,
		IMEI:         generateTestIMEI(),
		SerialNumber: "SN-ASSIGN-002",
		Status:       models.TerminalStatusInactive,
	}
	err = s.terminalService.RegisterTerminal(ctx, terminal2)
	require.NoError(s.T(), err)

	retailerID1 := uuid.New()
	retailerID2 := uuid.New()

	tests := []struct {
		name       string
		terminalID uuid.UUID
		retailerID uuid.UUID
		assignedBy uuid.UUID
		wantErr    bool
		checkFunc  func()
	}{
		{
			name:       "assign terminal to retailer",
			terminalID: terminal1.ID,
			retailerID: retailerID1,
			assignedBy: uuid.New(),
			wantErr:    false,
			checkFunc: func() {
				// Verify terminal status changed to active
				terminal, _ := s.terminalService.GetTerminal(ctx, terminal1.ID)
				assert.Equal(s.T(), models.TerminalStatusActive, terminal.Status)

				// Verify assignment exists
				var assignment models.TerminalAssignment
				query := `
   						SELECT *
    					FROM terminal_assignments
    					WHERE terminal_id = $1 AND retailer_id = $2 AND is_active = $3
    					LIMIT 1
						`
				err := s.db.Get(&assignment, query, terminal1.ID, retailerID1, true)
				assert.NoError(s.T(), err)
				assert.NotEqual(s.T(), uuid.Nil, assignment.AssignedBy)
			},
		},
		{
			name:       "assign already assigned terminal",
			terminalID: terminal1.ID,
			retailerID: retailerID2,
			assignedBy: uuid.New(),
			wantErr:    true,
		},
		{
			name:       "assign second terminal to same retailer",
			terminalID: terminal2.ID,
			retailerID: retailerID1,
			assignedBy: uuid.New(),
			wantErr:    true,
			checkFunc: func() {
				// Verify first assignment is deactivated
				var oldAssignment models.TerminalAssignment
				query := `
   					SELECT *
    				FROM terminal_assignments
    				WHERE terminal_id = $1 AND is_active = $2
    				LIMIT 1
				`
				err := s.db.Get(&oldAssignment, query, terminal1.ID, true)
				assert.Error(s.T(), err) // Should not find active assignment for terminal1

				// Verify new assignment is active
				var newAssignment models.TerminalAssignment
				query = `
    				SELECT *
    				FROM terminal_assignments
    				WHERE terminal_id = $1 AND retailer_id = $2 AND is_active = $3
    				LIMIT 1
				`
				err = s.db.Get(&newAssignment, query, terminal2.ID, retailerID1, true)

				assert.NoError(s.T(), err)
			},
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			err := s.terminalService.AssignTerminalToRetailer(ctx, tt.terminalID, tt.retailerID, tt.assignedBy)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc()
				}
			}
		})
	}
}

// Test Get Terminal by Retailer
func (s *TerminalServiceTestSuite) TestGetTerminalByRetailer() {
	ctx := context.Background()

	// Setup: Create and assign terminal
	terminal := &models.Terminal{
		DeviceID:     "POS-2025-000020",
		Name:         "Retailer Terminal",
		Model:        models.TerminalModelAndroidPOSV2,
		SerialNumber: "SN-RETAILER-001",
		Status:       models.TerminalStatusInactive,
	}
	err := s.terminalService.RegisterTerminal(ctx, terminal)
	require.NoError(s.T(), err)

	retailerID := uuid.New()
	assignedBy := uuid.New()
	err = s.terminalService.AssignTerminalToRetailer(ctx, terminal.ID, retailerID, assignedBy)
	require.NoError(s.T(), err)

	tests := []struct {
		name       string
		retailerID uuid.UUID
		wantCount  int
		wantDevice string
	}{
		{
			name:       "get terminal for retailer with assignment",
			retailerID: retailerID,
			wantCount:  1,
			wantDevice: "POS-2025-000020",
		},
		{
			name:       "get terminal for retailer without assignment",
			retailerID: uuid.New(),
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			result, err := s.terminalService.GetTerminalsByRetailer(ctx, tt.retailerID)
			assert.NoError(t, err)
			assert.Len(t, result, tt.wantCount)
			if tt.wantCount > 0 {
				assert.Equal(t, tt.wantDevice, result[0].DeviceID)
			}
		})
	}
}

// Test Unassign Terminal
func (s *TerminalServiceTestSuite) TestUnassignTerminal() {
	ctx := context.Background()

	// Setup: Create and assign terminal
	terminal := &models.Terminal{
		DeviceID:     "POS-2025-000030",
		Name:         "Terminal to Unassign",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: "SN-UNASSIGN-001",
		Status:       models.TerminalStatusInactive,
	}
	err := s.terminalService.RegisterTerminal(ctx, terminal)
	require.NoError(s.T(), err)

	retailerID := uuid.New()
	assignedBy := uuid.New()
	err = s.terminalService.AssignTerminalToRetailer(ctx, terminal.ID, retailerID, assignedBy)
	require.NoError(s.T(), err)

	// Test unassignment
	unassignedBy := uuid.New()
	err = s.terminalService.UnassignTerminal(ctx, terminal.ID, unassignedBy, "Retailer closed")
	assert.NoError(s.T(), err)

	// Verify terminal status changed to inactive
	updatedTerminal, err := s.terminalService.GetTerminal(ctx, terminal.ID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), models.TerminalStatusInactive, updatedTerminal.Status)

	// Verify assignment is inactive
	var assignment models.TerminalAssignment
	query := `
    SELECT *
    FROM terminal_assignments
    WHERE terminal_id = $1 AND is_active = $2
    LIMIT 1
`
	err = s.db.Get(&assignment, query, terminal.ID, true)
	assert.Error(s.T(), err) // Should not find active assignment

	// Verify retailer no longer has terminal
	terminals, err := s.terminalService.GetTerminalsByRetailer(ctx, retailerID)
	assert.NoError(s.T(), err)
	assert.Empty(s.T(), terminals)

	// Test unassigning already unassigned terminal
	unassignedBy2 := uuid.New()
	err = s.terminalService.UnassignTerminal(ctx, terminal.ID, unassignedBy2, "Test")
	assert.Error(s.T(), err)
}

// Test Update Terminal Status
func (s *TerminalServiceTestSuite) TestUpdateTerminalStatus() {
	ctx := context.Background()

	terminal := &models.Terminal{
		DeviceID:     "POS-2025-000040",
		Name:         "Status Test Terminal",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: "SN-STATUS-001",
		Status:       models.TerminalStatusInactive,
	}
	err := s.terminalService.RegisterTerminal(ctx, terminal)
	require.NoError(s.T(), err)

	statuses := []models.TerminalStatus{
		models.TerminalStatusActive,
		models.TerminalStatusMaintenance,
		models.TerminalStatusFaulty,
		models.TerminalStatusSuspended,
		models.TerminalStatusDecommissioned,
	}

	for _, status := range statuses {
		s.T().Run(string(status), func(t *testing.T) {
			err := s.terminalService.UpdateTerminalStatus(ctx, terminal.ID, status)
			assert.NoError(t, err)

			// Verify status was updated
			updated, err := s.terminalService.GetTerminal(ctx, terminal.ID)
			assert.NoError(t, err)
			assert.Equal(t, status, updated.Status)
		})
	}

	// Test updating non-existent terminal
	err = s.terminalService.UpdateTerminalStatus(ctx, uuid.New(), models.TerminalStatusActive)
	assert.Error(s.T(), err)
}

// Test List Terminals with Filters
func (s *TerminalServiceTestSuite) TestListTerminals() {
	ctx := context.Background()

	// Create multiple terminals with different statuses
	terminals := []struct {
		deviceID string
		status   models.TerminalStatus
		retailer *uuid.UUID
	}{
		{"POS-2025-100001", models.TerminalStatusActive, nil},
		{"POS-2025-100002", models.TerminalStatusActive, nil},
		{"POS-2025-100003", models.TerminalStatusInactive, nil},
		{"POS-2025-100004", models.TerminalStatusMaintenance, nil},
		{"POS-2025-100005", models.TerminalStatusFaulty, nil},
	}

	retailerID := uuid.New()
	for i, t := range terminals {
		terminal := &models.Terminal{
			DeviceID:     t.deviceID,
			Name:         fmt.Sprintf("List Test Terminal %d", i+1),
			Model:        models.TerminalModelAndroidPOSV1,
			SerialNumber: fmt.Sprintf("SN-LIST-%03d", i+1),
			IMEI:         generateTestIMEI(),
			Status:       t.status,
			CreatedAt:    time.Now().Add(time.Duration(i) * time.Second),
		}
		err := s.terminalService.RegisterTerminal(ctx, terminal)
		require.NoError(s.T(), err)

		// Assign first two active terminals to retailer
		if i < 2 && t.status == models.TerminalStatusActive {
			assignedBy := uuid.New()
			err = s.terminalService.AssignTerminalToRetailer(ctx, terminal.ID, retailerID, assignedBy)

			if i == 0 {
				// First assignment should succeed
				require.NoError(s.T(), err)
				terminals[i].retailer = &retailerID
			} else {
				// Second assignment should fail
				require.Error(s.T(), err)
				assert.Contains(s.T(), err.Error(), "already has an active terminal")
			}
		}
	}

	tests := []struct {
		name      string
		filter    TerminalFilter
		wantCount int
	}{
		{
			name: "list all terminals",
			filter: TerminalFilter{
				Page:     1,
				PageSize: 10,
			},
			wantCount: 5,
		},
		{
			name: "filter by active status",
			filter: TerminalFilter{
				Status:   &[]models.TerminalStatus{models.TerminalStatusActive}[0],
				Page:     1,
				PageSize: 10,
			},
			wantCount: 2,
		},
		{
			name: "filter by retailer",
			filter: TerminalFilter{
				RetailerID: &retailerID,
				Page:       1,
				PageSize:   10,
			},
			wantCount: 1, // Only one active assignment per retailer
		},
		{
			name: "pagination test",
			filter: TerminalFilter{
				Page:     1,
				PageSize: 2,
			},
			wantCount: 2,
		},
		{
			name: "sort by created_at descending",
			filter: TerminalFilter{
				Page:     1,
				PageSize: 10,
				SortBy:   "created_at",
				SortDesc: true,
			},
			wantCount: 5,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			results, total, err := s.terminalService.ListTerminals(ctx, tt.filter)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantCount, len(results))

			if tt.filter.PageSize > 0 && tt.filter.Page == 1 && tt.filter.Status == nil && tt.filter.RetailerID == nil {
				assert.Equal(t, 5, int(total)) // Total count when not filtered
			}

			// Verify sorting if specified
			if tt.filter.SortDesc && len(results) > 1 {
				assert.Greater(t, results[0].CreatedAt, results[1].CreatedAt)
			}
		})
	}
}

// Test Terminal Update
func (s *TerminalServiceTestSuite) TestUpdateTerminal() {
	ctx := context.Background()

	terminal := &models.Terminal{
		DeviceID:       "POS-2025-000050",
		Name:           "Original Name",
		Model:          models.TerminalModelAndroidPOSV1,
		SerialNumber:   "SN-UPDATE-001",
		AndroidVersion: "10.0",
		AppVersion:     "1.0.0",
		Status:         models.TerminalStatusInactive,
	}
	err := s.terminalService.RegisterTerminal(ctx, terminal)
	require.NoError(s.T(), err)

	// Update terminal details
	terminal.Name = "Updated Name"
	terminal.AppVersion = "1.1.0"
	terminal.AndroidVersion = "11.0"
	terminal.Metadata = map[string]string{
		"location": "Store A",
		"notes":    "Upgraded firmware",
	}

	err = s.terminalService.UpdateTerminal(ctx, terminal)
	assert.NoError(s.T(), err)

	// Verify updates
	updated, err := s.terminalService.GetTerminal(ctx, terminal.ID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated Name", updated.Name)
	assert.Equal(s.T(), "1.1.0", updated.AppVersion)
	assert.Equal(s.T(), "11.0", updated.AndroidVersion)
	assert.Equal(s.T(), "Store A", updated.Metadata["location"])
}

// Test One Terminal Per Retailer Constraint
func (s *TerminalServiceTestSuite) TestOneTerminalPerRetailerConstraint() {
	ctx := context.Background()

	// Create three terminals
	terminal1 := &models.Terminal{
		DeviceID:     "POS-2025-000061",
		Name:         "Terminal 1",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: "SN-CONSTRAINT-001",
		IMEI:         generateTestIMEI(),
		Status:       models.TerminalStatusInactive,
	}
	err := s.terminalService.RegisterTerminal(ctx, terminal1)
	require.NoError(s.T(), err)

	terminal2 := &models.Terminal{
		DeviceID:     "POS-2025-000062",
		Name:         "Terminal 2",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: "SN-CONSTRAINT-002",
		IMEI:         generateTestIMEI(),
		Status:       models.TerminalStatusInactive,
	}
	err = s.terminalService.RegisterTerminal(ctx, terminal2)
	require.NoError(s.T(), err)

	terminal3 := &models.Terminal{
		DeviceID:     "POS-2025-000063",
		Name:         "Terminal 3",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: "SN-CONSTRAINT-003",
		IMEI:         generateTestIMEI(),
		Status:       models.TerminalStatusInactive,
	}
	err = s.terminalService.RegisterTerminal(ctx, terminal3)
	require.NoError(s.T(), err)

	retailerID := uuid.New()
	assignedBy := uuid.New()

	// Assign first terminal
	err = s.terminalService.AssignTerminalToRetailer(ctx, terminal1.ID, retailerID, assignedBy)
	assert.NoError(s.T(), err)

	// Assign second terminal – should fail due to one active terminal rule
	err = s.terminalService.AssignTerminalToRetailer(ctx, terminal2.ID, retailerID, assignedBy)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "already has an active terminal")

	// Assign third terminal – should also fail
	err = s.terminalService.AssignTerminalToRetailer(ctx, terminal3.ID, retailerID, assignedBy)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "already has an active terminal")

	// Count active assignments for retailer (should be exactly 1)
	var activeCount int64
	query := `
    SELECT COUNT(*)
    FROM terminal_assignments
    WHERE retailer_id = $1 AND is_active = $2
	`
	err = s.db.Get(&activeCount, query, retailerID, true)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), activeCount)
}

// Test Non-existent Terminal Operations
func (s *TerminalServiceTestSuite) TestNonExistentTerminalOperations() {
	ctx := context.Background()
	nonExistentID := uuid.New()

	// Get non-existent terminal
	terminal, err := s.terminalService.GetTerminal(ctx, nonExistentID)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), terminal)

	// Update non-existent terminal
	err = s.terminalService.UpdateTerminalStatus(ctx, nonExistentID, models.TerminalStatusActive)
	assert.Error(s.T(), err)

	// Assign non-existent terminal
	assignedBy := uuid.New()
	err = s.terminalService.AssignTerminalToRetailer(ctx, nonExistentID, uuid.New(), assignedBy)
	assert.Error(s.T(), err)

	// Unassign non-existent terminal
	unassignedBy := uuid.New()
	err = s.terminalService.UnassignTerminal(ctx, nonExistentID, unassignedBy, "Test")
	assert.Error(s.T(), err)
}

// TestExistingImplementation exercises repositories, service logic, gRPC handlers and DB constraints
func (s *TerminalServiceTestSuite) TestExistingImplementation() {
	ctx := context.Background()
	t := s.T()

	// Register a terminal using the service layer
	deviceID := "POS-INTEG-0001"
	serial := "SN-INTEG-0001"
	imei := generateTestIMEI()

	terminal := &models.Terminal{
		DeviceID:       deviceID,
		Name:           "Integration Terminal 1",
		Model:          models.TerminalModelAndroidPOSV1,
		SerialNumber:   serial,
		IMEI:           imei,
		AndroidVersion: "12.0",
		AppVersion:     "2.0.0",
		Vendor:         "IntegrationVendor",
		Status:         models.TerminalStatusInactive,
		HealthStatus:   models.HealthStatusOffline,
		Metadata:       make(map[string]string),
	}

	require.NoError(t, s.terminalService.RegisterTerminal(ctx, terminal))
	require.NotEqual(t, uuid.Nil, terminal.ID)

	//  Retrieve terminal and verify
	got, err := s.terminalService.GetTerminal(ctx, terminal.ID)
	require.NoError(t, err)
	assert.Equal(t, deviceID, got.DeviceID)
	assert.Equal(t, serial, got.SerialNumber)

	//  Assign to retailer
	retailerID := uuid.New()
	assignedBy := uuid.New()
	require.NoError(t, s.terminalService.AssignTerminalToRetailer(ctx, terminal.ID, retailerID, assignedBy))

	// Verify assignment exists via service
	terminals, err := s.terminalService.GetTerminalsByRetailer(ctx, retailerID)
	require.NoError(t, err)
	assert.Len(t, terminals, 1)

	//  Update status
	require.NoError(t, s.terminalService.UpdateTerminalStatus(ctx, terminal.ID, models.TerminalStatusActive))
	updated, err := s.terminalService.GetTerminal(ctx, terminal.ID)
	require.NoError(t, err)
	assert.Equal(t, models.TerminalStatusActive, updated.Status)

	//  Update terminal details
	updated.Name = "Integration Terminal 1 - Updated"
	updated.AppVersion = "2.1.0"
	updated.Metadata = map[string]string{"location": "integration-lab"}
	require.NoError(t, s.terminalService.UpdateTerminal(ctx, updated))
	afterUpdate, err := s.terminalService.GetTerminal(ctx, terminal.ID)
	require.NoError(t, err)
	assert.Equal(t, "Integration Terminal 1 - Updated", afterUpdate.Name)
	assert.Equal(t, "2.1.0", afterUpdate.AppVersion)
	assert.Equal(t, "integration-lab", afterUpdate.Metadata["location"])

	//  Update config
	config, err := s.terminalService.GetTerminalConfig(ctx, terminal.ID)
	require.NoError(t, err)
	config.TransactionLimit = 5000
	config.DailyLimit = 50000
	config.OfflineModeEnabled = true
	config.OfflineSyncInterval = 15
	config.AutoUpdateEnabled = true
	config.MinimumAppVersion = "2.0.0"
	config.Settings = map[string]string{"env": "integration"}
	require.NoError(t, s.terminalService.UpdateTerminalConfig(ctx, config))
	gotConfig, err := s.terminalService.GetTerminalConfig(ctx, terminal.ID)
	require.NoError(t, err)
	assert.Equal(t, 5000, gotConfig.TransactionLimit)
	assert.Equal(t, "integration", gotConfig.Settings["env"])

	//  Update health
	health := &models.TerminalHealth{
		TerminalID:       terminal.ID,
		Status:           models.HealthStatusHealthy,
		BatteryLevel:     95,
		SignalStrength:   4,
		StorageAvailable: 1024 * 1024 * 100,
		StorageTotal:     1024 * 1024 * 200,
		MemoryUsage:      30,
		CPUUsage:         10,
		Diagnostics:      map[string]string{"cpu_temp": "50C"},
	}
	require.NoError(t, s.terminalService.UpdateTerminalHealth(ctx, terminal.ID, health))
	gotHealth, err := s.terminalService.GetTerminalHealth(ctx, terminal.ID)
	require.NoError(t, err)
	assert.Equal(t, models.HealthStatusHealthy, gotHealth.Status)

	// List terminals and ensure presence
	list, _, err := s.terminalService.ListTerminals(ctx, TerminalFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 1)
	found := false
	for _, it := range list {
		if it.DeviceID == deviceID {
			found = true
			break
		}
	}
	assert.True(t, found)

	//  Attempt duplicate registration to ensure terminal binding enforcement
	dup := &models.Terminal{
		DeviceID:     deviceID,
		Name:         "Dup Terminal",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: "DUP-SN",
	}
	err = s.terminalService.RegisterTerminal(ctx, dup)
	assert.Error(t, err)

	//  Delete terminal
	deletedBy := uuid.New()
	require.NoError(t, s.terminalService.DeleteTerminal(ctx, terminal.ID, deletedBy))

	// Verify terminal deleted
	_, err = s.terminalService.GetTerminal(ctx, terminal.ID)
	assert.Error(t, err)
}

func (s *TerminalServiceTestSuite) TestTerminalRegistrationFlow() {
	ctx := context.Background()
	t := s.T()

	//  Successful registration
	deviceID := "POS-TEST-0001"
	serial := "SN-TEST-0001"
	imei := fmt.Sprintf("IMEI-%d", time.Now().UnixNano()%1_000_000)

	term := &models.Terminal{
		DeviceID:     deviceID,
		Name:         "Test Terminal",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: serial,
		IMEI:         imei,
		Status:       models.TerminalStatusInactive,
		Metadata:     map[string]string{"env": "test"},
	}

	require.NoError(t, s.terminalService.RegisterTerminal(ctx, term))
	require.NotEqual(t, uuid.Nil, term.ID)

	// Verify default config created
	var cfgCount int
	err := s.db.GetContext(ctx, &cfgCount,
		"SELECT COUNT(*) FROM terminal_configs WHERE terminal_id = $1",
		term.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, cfgCount)

	// Verify health record initialized
	var healthCount int
	err = s.db.GetContext(ctx, &healthCount,
		"SELECT COUNT(*) FROM terminal_health WHERE terminal_id = $1",
		term.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, healthCount)

	//  Duplicate device_id rejection
	dup := &models.Terminal{
		DeviceID:     deviceID, // same device id
		Name:         "Dup Terminal",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: "SN-UNIQUE-1",
	}
	err = s.terminalService.RegisterTerminal(ctx, dup)
	assert.Error(t, err, "duplicate device_id should be rejected by DB unique constraint")

	// 3) Duplicate serial_number rejection
	dupSerial := &models.Terminal{
		DeviceID:     "POS-TEST-0002",
		Name:         "Dup Serial Terminal",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: serial, // same serial
	}
	err = s.terminalService.RegisterTerminal(ctx, dupSerial)
	assert.Error(t, err, "duplicate serial_number should be rejected by DB unique constraint")
}

func (s *TerminalServiceTestSuite) TestTerminalAssignmentConstraints() {
	ctx := context.Background()
	t := s.T()

	// Register three terminals
	terminal1 := &models.Terminal{
		DeviceID:     "POS-ASSIGN-001",
		Name:         "Assign Terminal 1",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: "SN-ASSIGN-001",
		IMEI:         generateTestIMEI(),
		Status:       models.TerminalStatusInactive,
		Metadata:     map[string]string{},
	}
	require.NoError(t, s.terminalService.RegisterTerminal(ctx, terminal1))

	terminal2 := &models.Terminal{
		DeviceID:     "POS-ASSIGN-002",
		Name:         "Assign Terminal 2",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: "SN-ASSIGN-002",
		IMEI:         generateTestIMEI(),
		Status:       models.TerminalStatusInactive,
		Metadata:     map[string]string{},
	}
	require.NoError(t, s.terminalService.RegisterTerminal(ctx, terminal2))

	// Use one retailer
	retailerID := uuid.New()
	assignedBy := uuid.New()

	//  Assign terminal1 to retailer
	require.NoError(t, s.terminalService.AssignTerminalToRetailer(ctx, terminal1.ID, retailerID, assignedBy))

	//  Attempt to assign terminal2 to same retailer (one active terminal per retailer)
	err := s.terminalService.AssignTerminalToRetailer(ctx, terminal2.ID, retailerID, uuid.New())
	assert.Error(t, err)

	//  Attempt to assign terminal1 again to another retailer
	otherRetailer := uuid.New()
	err = s.terminalService.AssignTerminalToRetailer(ctx, terminal1.ID, otherRetailer, uuid.New())
	assert.Error(t, err)

	// Check assignment record exists and is active
	var activeCount int
	query :=
		`SELECT COUNT(*) FROM terminal_assignments 
		WHERE terminal_id = $1 
		AND retailer_id = $2 AND is_active = true`
	err = s.db.GetContext(ctx, &activeCount, query, terminal1.ID, retailerID)
	require.NoError(t, err)
	assert.Equal(t, 1, activeCount)

	// Unassign terminal1
	unassignedBy := uuid.New()
	require.NoError(t, s.terminalService.UnassignTerminal(ctx, terminal1.ID, unassignedBy, "routine unassign"))

	// After unassignment, the previous assignment should be marked inactive and have unassigned_at
	var inactiveCount int
	query2 :=
		`SELECT COUNT(*) 
		FROM terminal_assignments 
		WHERE terminal_id = $1 
		AND is_active = false AND unassigned_at IS NOT NULL`
	err = s.db.GetContext(ctx, &inactiveCount, query2, terminal1.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, inactiveCount, 1)

	// Assign terminal2 to the retailer
	require.NoError(t, s.terminalService.AssignTerminalToRetailer(ctx, terminal2.ID, retailerID, uuid.New()))

	// Verify assignment history: there should be at least two records for this retailer (old inactive + new active)
	var totalAssignments int
	query3 := `SELECT COUNT(*) FROM terminal_assignments WHERE retailer_id = $1`
	err = s.db.GetContext(ctx, &totalAssignments, query3, retailerID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, totalAssignments, 2)

	// Clean up: unassign terminal2
	require.NoError(t, s.terminalService.UnassignTerminal(ctx, terminal2.ID, uuid.New(), "cleanup"))
}

func (s *TerminalServiceTestSuite) TestRetailerTerminalAuthentication() {
	ctx := context.Background()
	t := s.T()

	// Create two terminals
	terminalA := &models.Terminal{
		DeviceID:     "POS-AUTH-001",
		Name:         "Auth Terminal A",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: "SN-AUTH-001",
		IMEI:         generateTestIMEI(),
		Status:       models.TerminalStatusInactive,
	}
	require.NoError(t, s.terminalService.RegisterTerminal(ctx, terminalA))

	terminalB := &models.Terminal{
		DeviceID:     "POS-AUTH-002",
		Name:         "Auth Terminal B",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: "SN-AUTH-002",
		IMEI:         generateTestIMEI(),
		Status:       models.TerminalStatusInactive,
	}
	require.NoError(t, s.terminalService.RegisterTerminal(ctx, terminalB))

	retailerID := uuid.New()
	assignedBy := uuid.New()

	// Assign terminalA to retailer
	require.NoError(t, s.terminalService.AssignTerminalToRetailer(ctx, terminalA.ID, retailerID, assignedBy))

	// We need access to the concrete service implementation to call ValidateRetailerTerminal
	termService, ok := s.terminalService.(*terminalService)
	require.True(t, ok, "actual terminalService implementation is used for this test")

	// Activate terminalA for testing
	require.NoError(t, s.terminalService.UpdateTerminalStatus(ctx, terminalA.ID, models.TerminalStatusActive))

	// success when assigned and active
	err := termService.ValidateRetailerTerminal(ctx, retailerID, terminalA.ID)
	assert.NoError(t, err)

	// wrong terminal presented (terminalB not assigned)
	err = termService.ValidateRetailerTerminal(ctx, retailerID, terminalB.ID)
	assert.Error(t, err)

	// terminal becomes inactive - validation should fail
	require.NoError(t, s.terminalService.UpdateTerminalStatus(ctx, terminalA.ID, models.TerminalStatusInactive))
	err = termService.ValidateRetailerTerminal(ctx, retailerID, terminalA.ID)
	assert.Error(t, err)

	// unassigned retailer - create a new retailer without assignment
	unassignedRetailer := uuid.New()
	// Create and activate a terminal for retailer (without assignment)
	unaTerminal := &models.Terminal{
		DeviceID:     "POS-AUTH-003",
		Name:         "Unauth Terminal",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: "SN-AUTH-003",
		IMEI:         generateTestIMEI(),
		Status:       models.TerminalStatusActive,
	}
	require.NoError(t, s.terminalService.RegisterTerminal(ctx, unaTerminal))

	err = termService.ValidateRetailerTerminal(ctx, unassignedRetailer, unaTerminal.ID)
	assert.Error(t, err)
}

func (s *TerminalServiceTestSuite) TestHealthMonitoring() {
	ctx := context.Background()
	t := s.T()

	// Register a terminal
	term := &models.Terminal{
		DeviceID:     "POS-HEALTH-001",
		Name:         "Health Terminal",
		Model:        models.TerminalModelAndroidPOSV1,
		SerialNumber: "SN-HEALTH-001",
		IMEI:         generateTestIMEI(),
		Status:       models.TerminalStatusInactive,
	}
	require.NoError(t, s.terminalService.RegisterTerminal(ctx, term))

	// initial health should exist and be OFFLINE
	var initialStatus string
	err := s.db.GetContext(ctx, &initialStatus, "SELECT status FROM terminal_health WHERE terminal_id = $1 LIMIT 1", term.ID)
	require.NoError(t, err)
	assert.Equal(t, string(models.HealthStatusOffline), initialStatus)

	// Update health to HEALTHY and verify status change
	health := &models.TerminalHealth{
		TerminalID:       term.ID,
		Status:           models.HealthStatusHealthy,
		BatteryLevel:     90,
		SignalStrength:   4,
		StorageAvailable: 1024 * 1024 * 50,
		StorageTotal:     1024 * 1024 * 200,
		MemoryUsage:      20,
		CPUUsage:         5,
		Diagnostics:      map[string]string{"note": "ok"},
	}
	require.NoError(t, s.terminalService.UpdateTerminalHealth(ctx, term.ID, health))

	// Verify terminal_health updated
	var afterStatus string
	var lastHB time.Time
	err = s.db.GetContext(ctx, &afterStatus, "SELECT status FROM terminal_health WHERE terminal_id = $1 LIMIT 1", term.ID)
	require.NoError(t, err)
	assert.Equal(t, string(models.HealthStatusHealthy), afterStatus)

	// Verify health history recorded
	var historyCount int
	err = s.db.GetContext(ctx, &historyCount, "SELECT COUNT(*) FROM terminal_health_history WHERE terminal_id = $1", term.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, historyCount, 1)

	// Test offline detection
	past := time.Now().Add(-1 * time.Hour)
	_, err = s.db.ExecContext(ctx, "UPDATE terminal_health SET last_heartbeat = $1 WHERE terminal_id = $2", past, term.ID)
	require.NoError(t, err)

	// mark terminal as offline if no heartbeat in last 10 minutes
	offlineResults, err := s.terminalService.GetOfflineTerminals(ctx, 10*time.Minute)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(offlineResults), 1)

	// Update health again to WARNING and verify transition
	health2 := &models.TerminalHealth{
		TerminalID:     term.ID,
		Status:         models.HealthStatusWarning,
		BatteryLevel:   50,
		SignalStrength: 2,
		Diagnostics:    map[string]string{"temp": "75C"},
	}
	require.NoError(t, s.terminalService.UpdateTerminalHealth(ctx, term.ID, health2))

	// Verify latest status is WARNING
	err = s.db.QueryRowContext(ctx, "SELECT status, last_heartbeat FROM terminal_health WHERE terminal_id = $1 LIMIT 1", term.ID).Scan(&afterStatus, &lastHB)
	require.NoError(t, err)
	assert.Equal(t, string(models.HealthStatusWarning), afterStatus)
	assert.WithinDuration(t, time.Now(), lastHB, 1*time.Minute)
}

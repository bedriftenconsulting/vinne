package repositories

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type MigrationTestSuite struct {
	suite.Suite
	db        *sqlx.DB
	container *postgres.PostgresContainer
}

func TestMigrationTestSuite(t *testing.T) {
	suite.Run(t, new(MigrationTestSuite))
}

func (s *MigrationTestSuite) SetupSuite() {
	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("migration_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(s.T(), err)
	s.container = postgresContainer

	// Get connection details
	dbHost, err := postgresContainer.Host(ctx)
	require.NoError(s.T(), err)
	dbPort, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(s.T(), err)

	// Connect directly using sqlx for testing
	dsn := "host=" + dbHost + " port=" + dbPort.Port() + " user=testuser password=testpass dbname=migration_test sslmode=disable"
	db, err := sqlx.Connect("postgres", dsn)
	require.NoError(s.T(), err)
	s.db = db
}

func (s *MigrationTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(context.Background())
	}
}

func (s *MigrationTestSuite) TestGooseMigrationsSetup() {
	s.T().Log("✅ Testing Goose Migrations Setup")

	// Set Goose dialect
	err := goose.SetDialect("postgres")
	require.NoError(s.T(), err, "Should set Goose dialect successfully")

	// Get the migrations directory path relative to the test file
	migrationsDir := filepath.Join("..", "..", "migrations")

	// Convert sqlx.DB to sql.DB for Goose
	sqlDB := s.db.DB

	// Run up migrations
	err = goose.Up(sqlDB, migrationsDir)
	require.NoError(s.T(), err, "Should run up migrations successfully")

	s.T().Log("✅ Successfully ran all migrations using Goose")

	// Verify that key tables exist by querying them
	tables := []string{"agents", "retailers", "agent_retailers", "pos_devices", "agent_kyc", "retailer_kyc"}

	for _, table := range tables {
		var exists bool
		err := s.db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = $1
			)`, table).Scan(&exists)
		require.NoError(s.T(), err, "Should query table existence without error")
		require.True(s.T(), exists, "Table %s should exist after migrations", table)
		s.T().Logf("✅ Table '%s' exists", table)
	}

	// Test that we can query the agents table structure
	var count int
	err = s.db.QueryRow("SELECT COUNT(*) FROM agents").Scan(&count)
	require.NoError(s.T(), err, "Should be able to query agents table")
	require.Equal(s.T(), 0, count, "Agents table should be empty initially")
	s.T().Log("✅ Can query agents table successfully")

	// Run down migrations to clean up
	err = goose.DownTo(sqlDB, migrationsDir, 0)
	require.NoError(s.T(), err, "Should run down migrations successfully")
	s.T().Log("✅ Successfully ran down migrations for cleanup")

	// Verify tables are removed
	for _, table := range tables {
		var exists bool
		err := s.db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = $1
			)`, table).Scan(&exists)
		require.NoError(s.T(), err, "Should query table existence without error")
		require.False(s.T(), exists, "Table %s should not exist after down migrations", table)
	}
	s.T().Log("✅ All tables cleaned up successfully")
}

func (s *MigrationTestSuite) TestMigrationConsistency() {
	s.T().Log("✅ Testing Migration Consistency Between Test and Production")

	// This test ensures that the migration-based test setup
	// creates the same schema that would be used in production

	// Set Goose dialect
	err := goose.SetDialect("postgres")
	require.NoError(s.T(), err)

	migrationsDir := filepath.Join("..", "..", "migrations")
	sqlDB := s.db.DB

	// Run migrations
	err = goose.Up(sqlDB, migrationsDir)
	require.NoError(s.T(), err)

	// Test that critical columns exist with correct types
	testCases := []struct {
		table    string
		column   string
		dataType string
	}{
		{"agents", "id", "uuid"},
		{"agents", "agent_code", "character varying"},
		{"agents", "business_name", "character varying"},
		{"agents", "commission_percentage", "numeric"},
		{"retailers", "id", "uuid"},
		{"retailers", "retailer_code", "character varying"},
		{"pos_devices", "serial_number", "character varying"},
		{"pos_devices", "status", "character varying"},
	}

	for _, tc := range testCases {
		var dataType string
		err := s.db.QueryRow(`
			SELECT data_type 
			FROM information_schema.columns 
			WHERE table_schema = 'public' 
			AND table_name = $1 
			AND column_name = $2
		`, tc.table, tc.column).Scan(&dataType)

		require.NoError(s.T(), err, "Should find column %s.%s", tc.table, tc.column)
		require.Equal(s.T(), tc.dataType, dataType,
			"Column %s.%s should have type %s, got %s",
			tc.table, tc.column, tc.dataType, dataType)

		s.T().Logf("✅ Column %s.%s has correct type: %s", tc.table, tc.column, dataType)
	}

	s.T().Log("✅ Schema consistency verified - migrations create expected structure")
}

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
		postgres.WithDatabase("terminal_migration_test"),
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
	dsn := "host=" + dbHost + " port=" + dbPort.Port() + " user=testuser password=testpass dbname=terminal_migration_test sslmode=disable"
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
	s.T().Log("✅ Testing Goose Migrations Setup for Terminal Service")

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

	// Verify that key terminal tables exist
	terminalTables := []string{
		"terminals",
		"terminal_assignments",
		"terminal_versions",
		"terminal_configs",
		"terminal_health",
		"terminal_health_history",
		"terminal_audit_logs",
	}

	for _, table := range terminalTables {
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

	// Test that we can query the terminals table structure
	var count int
	err = s.db.QueryRow("SELECT COUNT(*) FROM terminals").Scan(&count)
	require.NoError(s.T(), err, "Should be able to query terminals table")
	require.Equal(s.T(), 0, count, "terminals table should be empty initially")
	s.T().Log("✅ Can query terminals table successfully")

	// Run down migrations to clean up
	err = goose.DownTo(sqlDB, migrationsDir, 0)
	require.NoError(s.T(), err, "Should run down migrations successfully")
	s.T().Log("✅ Successfully ran down migrations for cleanup")

	// Verify tables are removed
	for _, table := range terminalTables {
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

func (s *MigrationTestSuite) TestTerminalTableStructure() {
	s.T().Log("✅ Testing Terminal Table Structure")

	// Set Goose dialect and run migrations
	err := goose.SetDialect("postgres")
	require.NoError(s.T(), err)

	migrationsDir := filepath.Join("..", "..", "migrations")
	sqlDB := s.db.DB

	err = goose.Up(sqlDB, migrationsDir)
	require.NoError(s.T(), err)

	// Test critical columns exist with correct types
	testCases := []struct {
		table       string
		column      string
		dataType    string
		description string
	}{
		// Terminals table
		{"terminals", "id", "uuid", "Terminal UUID"},
		{"terminals", "device_id", "character varying", "Unique terminal identifier"},
		{"terminals", "serial_number", "character varying", "Terminal serial number"},
		{"terminals", "model", "character varying", "Terminal model"},
		{"terminals", "vendor", "character varying", "Terminal vendor"},
		{"terminals", "app_version", "character varying", "App version"},
		{"terminals", "status", "character varying", "Terminal status"},
		{"terminals", "last_sync", "timestamp without time zone", "Last sync timestamp"},

		// Terminal Assignments table
		{"terminal_assignments", "id", "uuid", "Assignment UUID"},
		{"terminal_assignments", "terminal_id", "uuid", "Terminal reference"},
		{"terminal_assignments", "retailer_id", "uuid", "Retailer reference"},
		{"terminal_assignments", "assigned_by", "uuid", "Assigned by user"},
		{"terminal_assignments", "assigned_at", "timestamp without time zone", "Assignment timestamp"},
		{"terminal_assignments", "is_active", "boolean", "Active assignment flag"},

		// Terminal Versions table
		{"terminal_versions", "id", "uuid", "Version UUID"},
		{"terminal_versions", "terminal_id", "uuid", "Terminal reference"},
		{"terminal_versions", "version_type", "character varying", "Version type"},
		{"terminal_versions", "version_number", "character varying", "Version number"},
		{"terminal_versions", "update_status", "character varying", "Update status"},

		// Terminal Health table
		{"terminal_health", "id", "uuid", "Health UUID"},
		{"terminal_health", "terminal_id", "uuid", "Terminal reference"},
		{"terminal_health", "status", "character varying", "Health status"},
		{"terminal_health", "battery_level", "integer", "Battery level"},
		{"terminal_health", "signal_strength", "integer", "Signal strength"},

		// Terminal Configs table
		{"terminal_configs", "id", "uuid", "Config UUID"},
		{"terminal_configs", "terminal_id", "uuid", "Terminal reference"},
		{"terminal_configs", "transaction_limit", "integer", "Transaction limit"},
		{"terminal_configs", "daily_limit", "integer", "Daily limit"},
		{"terminal_configs", "offline_mode_enabled", "boolean", "Offline mode flag"},

		// Terminal Audit Logs table
		{"terminal_audit_logs", "id", "uuid", "Audit UUID"},
		{"terminal_audit_logs", "terminal_id", "uuid", "Terminal reference"},
		{"terminal_audit_logs", "action", "character varying", "Audit action"},
		{"terminal_audit_logs", "old_value", "jsonb", "Old value"},
		{"terminal_audit_logs", "new_value", "jsonb", "New value"},
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
			"Column %s.%s should have type %s for %s, got %s",
			tc.table, tc.column, tc.dataType, tc.description, dataType)

		s.T().Logf("✅ Column %s.%s correctly uses %s for %s", tc.table, tc.column, dataType, tc.description)
	}

	s.T().Log("✅ All terminal table columns have correct types")
}

func (s *MigrationTestSuite) TestIndexesCreated() {
	s.T().Log("✅ Testing Database Indexes Creation for Terminal Service")

	// Run migrations
	err := goose.SetDialect("postgres")
	require.NoError(s.T(), err)

	migrationsDir := filepath.Join("..", "..", "migrations")
	sqlDB := s.db.DB

	err = goose.Up(sqlDB, migrationsDir)
	require.NoError(s.T(), err)

	// Test that important indexes exist
	indexes := []struct {
		table     string
		indexName string
	}{
		{"terminals", "idx_terminals_device_id"},
		{"terminals", "idx_terminals_status"},
		{"terminals", "idx_terminals_retailer_id"},
		{"terminals", "idx_terminals_health_status"},
		{"terminal_assignments", "idx_terminal_assignments_terminal_id"},
		{"terminal_assignments", "idx_terminal_assignments_retailer_id"},
		{"terminal_assignments", "idx_terminal_assignments_is_active"},
		{"terminal_versions", "idx_terminal_versions_terminal_id"},
		{"terminal_configs", "idx_terminal_configs_terminal_id"},
		{"terminal_health", "idx_terminal_health_terminal_id"},
		{"terminal_health", "idx_terminal_health_status"},
		{"terminal_audit_logs", "idx_terminal_audit_logs_terminal_id"},
		{"terminal_audit_logs", "idx_terminal_audit_logs_action"},
	}

	for _, idx := range indexes {
		var exists bool
		err := s.db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes 
				WHERE schemaname = 'public' 
				AND tablename = $1 
				AND indexname = $2
			)`, idx.table, idx.indexName).Scan(&exists)

		require.NoError(s.T(), err, "Should query index existence without error")
		require.True(s.T(), exists, "Index %s on table %s should exist", idx.indexName, idx.table)
		s.T().Logf("✅ Index %s exists on table %s", idx.indexName, idx.table)
	}
}

func (s *MigrationTestSuite) TestConstraintsAndDefaults() {
	s.T().Log("✅ Testing Constraints and Default Values for Terminal Service")

	// Run migrations
	err := goose.SetDialect("postgres")
	require.NoError(s.T(), err)

	migrationsDir := filepath.Join("..", "..", "migrations")
	sqlDB := s.db.DB

	err = goose.Up(sqlDB, migrationsDir)
	require.NoError(s.T(), err)

	// Test unique constraints
	uniqueConstraints := []struct {
		table      string
		column     string
		constraint string
	}{
		{"terminals", "device_id", "terminals_device_id_key"},
		{"terminals", "serial_number", "terminals_serial_number_key"},
		{"terminal_configs", "terminal_id", "terminal_configs_terminal_id_key"},
		{"terminal_health", "terminal_id", "terminal_health_terminal_id_key"},
	}

	for _, uc := range uniqueConstraints {
		var exists bool
		err := s.db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.table_constraints 
				WHERE constraint_schema = 'public' 
				AND table_name = $1 
				AND constraint_name = $2
				AND constraint_type = 'UNIQUE'
			)`, uc.table, uc.constraint).Scan(&exists)

		require.NoError(s.T(), err, "Should query constraint existence without error")
		require.True(s.T(), exists, "Unique constraint %s on table %s should exist", uc.constraint, uc.table)
		s.T().Logf("✅ Unique constraint %s exists on table %s", uc.constraint, uc.table)
	}

	// Test default values
	defaultValues := []struct {
		table        string
		column       string
		defaultValue string
	}{
		{"terminals", "status", "'INACTIVE'::character varying"},
		{"terminal_assignments", "is_active", "true"},
		{"terminal_configs", "offline_mode_enabled", "true"},
		{"terminal_health", "status", "'OFFLINE'::character varying"},
	}

	for _, dv := range defaultValues {
		var columnDefault string
		err := s.db.QueryRow(`
			SELECT column_default 
			FROM information_schema.columns 
			WHERE table_schema = 'public' 
			AND table_name = $1 
			AND column_name = $2
		`, dv.table, dv.column).Scan(&columnDefault)

		require.NoError(s.T(), err, "Should find default value for %s.%s", dv.table, dv.column)
		require.Contains(s.T(), columnDefault, dv.defaultValue,
			"Column %s.%s should have default value containing %s, got %s",
			dv.table, dv.column, dv.defaultValue, columnDefault)

		s.T().Logf("✅ Column %s.%s has correct default value: %s", dv.table, dv.column, columnDefault)
	}
}

func (s *MigrationTestSuite) TestForeignKeyConstraints() {
	s.T().Log("✅ Testing Foreign Key Constraints for Terminal Service")

	// Run migrations
	err := goose.SetDialect("postgres")
	require.NoError(s.T(), err)

	migrationsDir := filepath.Join("..", "..", "migrations")
	sqlDB := s.db.DB

	err = goose.Up(sqlDB, migrationsDir)
	require.NoError(s.T(), err)

	// Test foreign key constraints
	foreignKeys := []struct {
		table           string
		constraintName  string
		referencedTable string
	}{
		{"terminal_assignments", "terminal_assignments_terminal_id_fkey", "terminals"},
		{"terminal_versions", "terminal_versions_terminal_id_fkey", "terminals"},
		{"terminal_configs", "terminal_configs_terminal_id_fkey", "terminals"},
		{"terminal_health", "terminal_health_terminal_id_fkey", "terminals"},
		{"terminal_audit_logs", "terminal_audit_logs_terminal_id_fkey", "terminals"},
	}

	for _, fk := range foreignKeys {
		var exists bool
		err := s.db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 
				FROM information_schema.table_constraints tc
				JOIN information_schema.constraint_column_usage ccu
				ON tc.constraint_name = ccu.constraint_name
				WHERE tc.constraint_type = 'FOREIGN KEY'
				AND tc.table_name = $1
				AND tc.constraint_name = $2
				AND ccu.table_name = $3
			)`, fk.table, fk.constraintName, fk.referencedTable).Scan(&exists)

		require.NoError(s.T(), err, "Should query foreign key existence without error")
		require.True(s.T(), exists, "Foreign key %s on table %s referencing %s should exist",
			fk.constraintName, fk.table, fk.referencedTable)
		s.T().Logf("✅ Foreign key %s exists on table %s referencing %s",
			fk.constraintName, fk.table, fk.referencedTable)
	}
}

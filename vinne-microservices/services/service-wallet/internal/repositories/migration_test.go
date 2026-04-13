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
		postgres.WithDatabase("wallet_migration_test"),
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
	dsn := "host=" + dbHost + " port=" + dbPort.Port() + " user=testuser password=testpass dbname=wallet_migration_test sslmode=disable"
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
	s.T().Log("✅ Testing Goose Migrations Setup for Wallet Service")

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

	// Verify that key wallet tables exist
	walletTables := []string{
		"agent_stake_wallets",
		"retailer_stake_wallets",
		"retailer_winning_wallets",
		"wallet_transactions",
		"wallet_transfers",
		"wallet_locks",
		"agent_commission_rates",
		"commission_transactions",
		"commission_calculations",
		"commission_audit",
	}

	for _, table := range walletTables {
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

	// Test that we can query the agent_stake_wallets table structure
	var count int
	err = s.db.QueryRow("SELECT COUNT(*) FROM agent_stake_wallets").Scan(&count)
	require.NoError(s.T(), err, "Should be able to query agent_stake_wallets table")
	require.Equal(s.T(), 0, count, "agent_stake_wallets table should be empty initially")
	s.T().Log("✅ Can query agent_stake_wallets table successfully")

	// Run down migrations to clean up
	err = goose.DownTo(sqlDB, migrationsDir, 0)
	require.NoError(s.T(), err, "Should run down migrations successfully")
	s.T().Log("✅ Successfully ran down migrations for cleanup")

	// Verify tables are removed
	for _, table := range walletTables {
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

func (s *MigrationTestSuite) TestMonetaryColumnTypes() {
	s.T().Log("✅ Testing Monetary Column Types Use BIGINT")

	// Set Goose dialect and run migrations
	err := goose.SetDialect("postgres")
	require.NoError(s.T(), err)

	migrationsDir := filepath.Join("..", "..", "migrations")
	sqlDB := s.db.DB

	err = goose.Up(sqlDB, migrationsDir)
	require.NoError(s.T(), err)

	// Test that all monetary columns use BIGINT as per PRD requirements
	monetaryColumns := []struct {
		table       string
		column      string
		dataType    string
		description string
	}{
		// Wallet balance columns
		{"agent_stake_wallets", "balance", "bigint", "Agent wallet balance in pesewas"},
		{"agent_stake_wallets", "pending_balance", "bigint", "Agent pending balance in pesewas"},
		{"agent_stake_wallets", "available_balance", "bigint", "Agent available balance in pesewas"},
		{"retailer_stake_wallets", "balance", "bigint", "Retailer stake balance in pesewas"},
		{"retailer_stake_wallets", "pending_balance", "bigint", "Retailer pending balance in pesewas"},
		{"retailer_stake_wallets", "available_balance", "bigint", "Retailer available balance in pesewas"},
		{"retailer_winning_wallets", "balance", "bigint", "Retailer winning balance in pesewas"},
		{"retailer_winning_wallets", "pending_balance", "bigint", "Retailer winning pending balance in pesewas"},
		{"retailer_winning_wallets", "available_balance", "bigint", "Retailer winning available balance in pesewas"},

		// Transaction amount columns
		{"wallet_transactions", "amount", "bigint", "Transaction amount in pesewas"},
		{"wallet_transactions", "balance_before", "bigint", "Balance before transaction in pesewas"},
		{"wallet_transactions", "balance_after", "bigint", "Balance after transaction in pesewas"},
		{"wallet_transfers", "amount", "bigint", "Transfer amount in pesewas"},
		{"wallet_transfers", "commission_amount", "bigint", "Commission amount in pesewas"},
		{"wallet_transfers", "total_deducted", "bigint", "Total amount deducted in pesewas"},

		// Commission columns
		{"agent_commission_rates", "rate", "integer", "Commission rate in basis points"},
		{"commission_transactions", "original_amount", "bigint", "Original amount in pesewas"},
		{"commission_transactions", "gross_amount", "bigint", "Gross amount in pesewas"},
		{"commission_transactions", "commission_amount", "bigint", "Commission amount in pesewas"},
		{"commission_transactions", "commission_rate", "integer", "Commission rate in basis points"},
		{"commission_calculations", "input_amount", "bigint", "Input amount in pesewas"},
		{"commission_calculations", "commission_rate", "integer", "Commission rate in basis points"},
		{"commission_calculations", "gross_amount", "bigint", "Gross amount in pesewas"},
		{"commission_calculations", "commission_amount", "bigint", "Commission amount in pesewas"},
		{"commission_calculations", "net_amount", "bigint", "Net amount in pesewas"},
	}

	for _, tc := range monetaryColumns {
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

	s.T().Log("✅ All monetary columns correctly use BIGINT/INTEGER as per PRD requirements")
}

func (s *MigrationTestSuite) TestIndexesCreated() {
	s.T().Log("✅ Testing Database Indexes Creation")

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
		{"agent_stake_wallets", "idx_agent_stake_wallets_agent_id"},
		{"retailer_stake_wallets", "idx_retailer_stake_wallets_retailer_id"},
		{"retailer_winning_wallets", "idx_retailer_winning_wallets_retailer_id"},
		{"wallet_transactions", "idx_wallet_transactions_wallet_owner_id"},
		{"wallet_transactions", "idx_wallet_transactions_idempotency"},
		{"wallet_transfers", "idx_wallet_transfers_idempotency"},
		{"agent_commission_rates", "idx_agent_commission_rates_agent_id"},
		{"commission_transactions", "idx_commission_transactions_agent_id"},
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
	s.T().Log("✅ Testing Constraints and Default Values")

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
		{"agent_stake_wallets", "agent_id", "agent_stake_wallets_agent_id_key"},
		{"retailer_stake_wallets", "retailer_id", "retailer_stake_wallets_retailer_id_key"},
		{"retailer_winning_wallets", "retailer_id", "retailer_winning_wallets_retailer_id_key"},
		{"wallet_transactions", "transaction_id", "wallet_transactions_transaction_id_key"},
		{"wallet_transfers", "transfer_id", "wallet_transfers_transfer_id_key"},
		{"commission_transactions", "commission_id", "commission_transactions_commission_id_key"},
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
		{"agent_stake_wallets", "balance", "0"},
		{"agent_stake_wallets", "currency", "'GHS'::character varying"},
		{"agent_stake_wallets", "status", "'ACTIVE'::character varying"},
		{"wallet_transactions", "status", "'PENDING'::character varying"},
		{"commission_transactions", "status", "'PENDING'::character varying"},
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

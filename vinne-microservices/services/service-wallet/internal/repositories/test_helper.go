package repositories

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

// TestHelper provides common test setup functionality for wallet service tests
type TestHelper struct {
	migrationPath string
}

// NewTestHelper creates a new test helper
func NewTestHelper() *TestHelper {
	// Migrations are located at ../../migrations relative to repositories directory
	return &TestHelper{
		migrationPath: filepath.Join("..", "..", "migrations"),
	}
}

// SetupTestDB runs migrations on the test database using Goose
func (h *TestHelper) SetupTestDB(t *testing.T, db *sql.DB) error {
	// Set Goose dialect
	err := goose.SetDialect("postgres")
	if err != nil {
		return fmt.Errorf("failed to set Goose dialect: %w", err)
	}

	// Get absolute path to migrations directory
	absPath, err := filepath.Abs(h.migrationPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Run up migrations
	err = goose.Up(db, absPath)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	t.Logf("Successfully ran migrations from %s", absPath)
	return nil
}

// CleanupTestData truncates all tables while preserving structure
func (h *TestHelper) CleanupTestData(t *testing.T, db *sql.DB) error {
	// Get list of all tables
	rows, err := db.Query(`
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		AND tablename NOT LIKE 'pg_%'
		AND tablename NOT LIKE 'sql_%'
		AND tablename != 'goose_db_version'
	`)
	if err != nil {
		return fmt.Errorf("failed to get table list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, table)
	}

	if len(tables) == 0 {
		return nil
	}

	// Truncate all tables in a single statement to handle foreign key constraints
	truncateSQL := fmt.Sprintf("TRUNCATE %s RESTART IDENTITY CASCADE", strings.Join(tables, ", "))
	_, err = db.Exec(truncateSQL)
	if err != nil {
		return fmt.Errorf("failed to truncate tables: %w", err)
	}

	t.Logf("Truncated %d tables", len(tables))
	return nil
}

// SeedDefaultCommissionRates seeds default commission rates for testing
func (h *TestHelper) SeedDefaultCommissionRates(t *testing.T, db *sql.DB) error {
	// Insert default commission rates for test agents
	_, err := db.Exec(`
		INSERT INTO agent_commission_rates (id, agent_id, rate, effective_from)
		VALUES
			('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380c11', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 500, NOW()), -- 5%
			('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380c12', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12', 750, NOW()), -- 7.5%
			('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380c13', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13', 1000, NOW()) -- 10%
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("failed to seed commission rates: %w", err)
	}

	t.Log("Seeded default commission rates")
	return nil
}

// AssertMigrationValid validates that migrations can be applied successfully
func AssertMigrationValid(t *testing.T, db *sql.DB) {
	helper := NewTestHelper()
	err := helper.SetupTestDB(t, db)
	require.NoError(t, err, "Migrations should apply successfully")

	// Verify key wallet tables exist
	tables := []string{
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

	for _, table := range tables {
		var exists bool
		err := db.QueryRow(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public'
				AND table_name = $1
			)
		`, table).Scan(&exists)

		require.NoError(t, err, "Should be able to check if table exists")
		require.True(t, exists, fmt.Sprintf("Table %s should exist after migrations", table))
	}

	t.Log("All expected wallet tables exist after migration")
}

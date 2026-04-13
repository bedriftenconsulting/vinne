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

// TestHelper provides common test setup functionality for game service tests
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

// SeedDefaultGames seeds default games for testing
func (h *TestHelper) SeedDefaultGames(t *testing.T, db *sql.DB) error {
	// Insert some default games for testing
	_, err := db.Exec(`
		INSERT INTO games (id, code, name, type, game_type, game_format, game_category,
			organizer, min_stake_amount, max_stake_amount, max_tickets_per_player,
			draw_frequency, number_range_min, number_range_max, selection_count,
			sales_cutoff_minutes, base_price, status, version)
		VALUES
			('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380b11', 'TEST-5-90', 'Test 5/90', '5_90', '5_90',
			 '5_by_90', 'NUMBERS', 'NLA', 1.0, 100.0, 10, 'daily', 1, 90, 5, 30, 1.0, 'APPROVED', '1.0'),
			('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380b12', 'TEST-6-49', 'Test 6/49', '6_49', '6_49',
			 '6_by_49', 'NUMBERS', 'NLA', 2.0, 200.0, 20, 'weekly', 1, 49, 6, 60, 2.0, 'APPROVED', '1.0')
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("failed to seed default games: %w", err)
	}

	t.Log("Seeded default games")
	return nil
}

// AssertMigrationValid validates that migrations can be applied successfully
func AssertMigrationValid(t *testing.T, db *sql.DB) {
	helper := NewTestHelper()
	err := helper.SetupTestDB(t, db)
	require.NoError(t, err, "Migrations should apply successfully")

	// Verify key game tables exist
	tables := []string{
		"games",
		"game_schedules",
		"game_approvals",
		"prize_structures",
		"prize_tiers",
		"game_rules",
		"rule_validations",
		"game_feature_flags",
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

	t.Log("All expected game tables exist after migration")
}

package repositories

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// MigrationRunner helps run migrations for tests
type MigrationRunner struct {
	db            *sql.DB
	migrationPath string
}

// NewMigrationRunner creates a new migration runner
func NewMigrationRunner(db *sql.DB, migrationPath string) *MigrationRunner {
	return &MigrationRunner{
		db:            db,
		migrationPath: migrationPath,
	}
}

// RunMigrations runs all migration files in the specified directory
func (m *MigrationRunner) RunMigrations(t *testing.T) error {
	// Get absolute path to migrations directory
	absPath, err := filepath.Abs(m.migrationPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Read all SQL files from migrations directory
	files, err := os.ReadDir(absPath)
	if err != nil {
		return fmt.Errorf("failed to read migration directory %s: %w", absPath, err)
	}

	// Filter and sort SQL files
	var sqlFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
			sqlFiles = append(sqlFiles, filepath.Join(absPath, file.Name()))
		}
	}

	if len(sqlFiles) == 0 {
		return fmt.Errorf("no SQL migration files found in %s", absPath)
	}

	t.Logf("Found %d migration files to run", len(sqlFiles))

	// Execute each migration file
	for _, filePath := range sqlFiles {
		t.Logf("Running migration: %s", filepath.Base(filePath))

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filePath, err)
		}

		// Parse and execute the UP migration
		upSQL := extractUpMigration(string(content))
		if upSQL == "" {
			t.Logf("Skipping file %s: no UP migration found", filepath.Base(filePath))
			continue
		}

		// Execute the migration
		_, err = m.db.Exec(upSQL)
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", filepath.Base(filePath), err)
		}

		t.Logf("Successfully executed migration: %s", filepath.Base(filePath))
	}

	return nil
}

// extractUpMigration extracts the UP migration SQL from a goose migration file
func extractUpMigration(content string) string {
	lines := strings.Split(content, "\n")
	var upSQL strings.Builder
	inUpSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for UP section start
		if strings.Contains(trimmed, "+goose Up") {
			inUpSection = true
			continue
		}

		// Check for DOWN section start (end of UP section)
		if strings.Contains(trimmed, "+goose Down") {
			break
		}

		// Check for statement begin/end markers
		if strings.Contains(trimmed, "+goose StatementBegin") {
			continue
		}
		if strings.Contains(trimmed, "+goose StatementEnd") {
			continue
		}

		// Collect SQL if we're in the UP section
		if inUpSection {
			// Skip goose directives
			if strings.HasPrefix(trimmed, "--") && strings.Contains(trimmed, "+goose") {
				continue
			}
			upSQL.WriteString(line)
			upSQL.WriteString("\n")
		}
	}

	return strings.TrimSpace(upSQL.String())
}

// TestHelper provides common test setup functionality
type TestHelper struct {
	migrationPath string
}

// NewTestHelper creates a new test helper
func NewTestHelper() *TestHelper {
	// Assuming tests are run from repository root or service directory
	// Adjust path as needed based on your test execution context
	return &TestHelper{
		migrationPath: "../../migrations", // Relative to repositories directory
	}
}

// SetupTestDB runs migrations on the test database
func (h *TestHelper) SetupTestDB(t *testing.T, db *sql.DB) error {
	runner := NewMigrationRunner(db, h.migrationPath)
	return runner.RunMigrations(t)
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

// SeedDefaultData seeds default data after migrations
// This can be overridden or extended by specific test suites
func (h *TestHelper) SeedDefaultData(t *testing.T, db *sql.DB) error {
	// Re-insert default roles if they don't exist
	// This is safe to run multiple times due to ON CONFLICT clause
	_, err := db.Exec(`
		INSERT INTO admin_roles (id, name, description) VALUES
		    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'super_admin', 'Full system access with all permissions'),
		    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12', 'admin', 'Administrative access with most permissions'),
		    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13', 'manager', 'Management access for operations'),
		    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14', 'support', 'Customer support access'),
		    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15', 'viewer', 'Read-only access to system')
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("failed to seed default roles: %w", err)
	}

	t.Log("Seeded default data")
	return nil
}

// AssertMigrationValid validates that migrations can be applied successfully
func AssertMigrationValid(t *testing.T, db *sql.DB, migrationPath string) {
	runner := NewMigrationRunner(db, migrationPath)
	err := runner.RunMigrations(t)
	require.NoError(t, err, "Migrations should apply successfully")

	// Verify key tables exist
	tables := []string{
		"admin_users",
		"admin_roles",
		"permissions",
		"admin_user_roles",
		"role_permissions",
		"admin_sessions",
		"admin_audit_logs",
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

	t.Log("All expected tables exist after migration")
}

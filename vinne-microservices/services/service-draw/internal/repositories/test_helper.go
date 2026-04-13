package repositories

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestHelper provides utilities for integration testing
type TestHelper struct{}

// NewTestHelper creates a new test helper
func NewTestHelper() *TestHelper {
	return &TestHelper{}
}

// SetupTestDB runs migrations and sets up the test database
func (h *TestHelper) SetupTestDB(t *testing.T, db *sql.DB) error {
	t.Helper()

	// Find migrations directory (go up from current test location)
	migrationsDir, err := h.findMigrationsDir()
	require.NoError(t, err, "failed to find migrations directory")

	// Read and execute all migration files in order
	files, err := os.ReadDir(migrationsDir)
	require.NoError(t, err, "failed to read migrations directory")

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		migrationPath := filepath.Join(migrationsDir, file.Name())
		content, err := os.ReadFile(migrationPath)
		require.NoError(t, err, "failed to read migration file: %s", file.Name())

		// Parse goose migration format
		upSQL := h.extractUpMigration(string(content))
		if upSQL == "" {
			continue
		}

		// Execute UP migration
		_, err = db.Exec(upSQL)
		require.NoError(t, err, "failed to execute migration: %s", file.Name())
		t.Logf("Applied migration: %s", file.Name())
	}

	return nil
}

// CleanupTestData truncates all tables for a clean test state
func (h *TestHelper) CleanupTestData(t *testing.T, db *sql.DB) error {
	t.Helper()

	queries := []string{
		"TRUNCATE TABLE draws CASCADE",
		"TRUNCATE TABLE draw_schedules CASCADE",
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			// Ignore errors for tables that don't exist yet
			continue
		}
	}

	return nil
}

// findMigrationsDir locates the migrations directory
func (h *TestHelper) findMigrationsDir() (string, error) {
	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Try multiple possible locations
	possiblePaths := []string{
		filepath.Join(wd, "../../migrations"),
		filepath.Join(wd, "../../../migrations"),
		filepath.Join(wd, "../../../../migrations"),
		"./migrations",
		"../migrations",
		"../../migrations",
	}

	for _, path := range possiblePaths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err == nil {
			return absPath, nil
		}
	}

	return "", os.ErrNotExist
}

// extractUpMigration extracts the UP section from a goose migration
func (h *TestHelper) extractUpMigration(content string) string {
	lines := strings.Split(content, "\n")
	var upSQL strings.Builder
	inUpSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "-- +goose Up") {
			inUpSection = true
			continue
		}

		if strings.HasPrefix(trimmed, "-- +goose Down") {
			break
		}

		if inUpSection {
			upSQL.WriteString(line)
			upSQL.WriteString("\n")
		}
	}

	return upSQL.String()
}

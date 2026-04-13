package saga

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/randco/service-payment/internal/events"
)

// mockPublisher is a shared mock for all saga tests
type mockPublisher struct{}

func (m *mockPublisher) PublishTransactionEvent(ctx context.Context, event *events.TransactionEvent) error {
	return nil
}

func (m *mockPublisher) PublishDepositEvent(ctx context.Context, event *events.DepositEvent) error {
	return nil
}

func (m *mockPublisher) PublishWithdrawalEvent(ctx context.Context, event *events.WithdrawalEvent) error {
	return nil
}

func (m *mockPublisher) PublishSagaEvent(ctx context.Context, event *events.SagaEvent) error {
	return nil
}

func (m *mockPublisher) PublishProviderEvent(ctx context.Context, event *events.ProviderEvent) error {
	return nil
}

func (m *mockPublisher) Close() error {
	return nil
}

// setupTestDB creates a PostgreSQL testcontainer with the payment schema
func setupTestDB(t *testing.T) (*sqlx.DB, func()) {
	ctx := context.Background()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("payment_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test123"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	pgHost, err := postgresContainer.Host(ctx)
	require.NoError(t, err)
	pgPort, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	connStr := fmt.Sprintf("host=%s port=%s user=test password=test123 dbname=payment_test sslmode=disable",
		pgHost, pgPort.Port())

	db, err := sqlx.Connect("postgres", connStr)
	require.NoError(t, err)

	// Run migrations using Goose to ensure consistency with real schema
	runMigrations(t, db)

	cleanup := func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close database: %s", err)
		}
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}

	return db, cleanup
}

// runMigrations runs all database migrations using Goose
func runMigrations(t *testing.T, db *sqlx.DB) {
	// Set Goose dialect
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("Failed to set goose dialect: %v", err)
	}

	// Get the migrations directory path relative to the test file
	// From internal/saga/ to migrations/ is ../../migrations
	migrationsDir := filepath.Join("..", "..", "migrations")

	// Convert sqlx.DB to sql.DB for Goose
	sqlDB := db.DB

	// Run up migrations
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	t.Log("✅ Successfully ran all migrations using Goose")
}

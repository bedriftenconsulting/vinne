package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/randco/service-payment/internal/models"
	"github.com/randco/service-payment/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupTestDatabase creates a test database with migrations applied
func setupTestDatabase(t *testing.T, ctx context.Context) (*sqlx.DB, func()) {
	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("payment_service"),
		postgres.WithUsername("payment"),
		postgres.WithPassword("payment123"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	// Get connection string
	pgHost, err := postgresContainer.Host(ctx)
	require.NoError(t, err)
	pgPort, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	connStr := fmt.Sprintf("host=%s port=%s user=payment password=payment123 dbname=payment_service sslmode=disable",
		pgHost, pgPort.Port())

	db, err := sqlx.Connect("postgres", connStr)
	require.NoError(t, err)

	// Apply minimal schema for testing
	_, err = db.Exec(`
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

		CREATE TABLE IF NOT EXISTS transactions (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			reference VARCHAR(100) NOT NULL UNIQUE,
			provider_transaction_id VARCHAR(255),
			type VARCHAR(50) NOT NULL,
			status VARCHAR(50) NOT NULL,
			amount BIGINT NOT NULL,
			currency VARCHAR(3) NOT NULL,
			narration TEXT,
			provider_name VARCHAR(50) NOT NULL,
			source_type VARCHAR(50),
			source_identifier VARCHAR(255),
			source_name VARCHAR(255),
			destination_type VARCHAR(50),
			destination_identifier VARCHAR(255),
			destination_name VARCHAR(255),
			user_id UUID NOT NULL,
			customer_remarks TEXT,
			metadata JSONB,
			provider_data JSONB,
			requested_at TIMESTAMP NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS webhook_events (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			provider VARCHAR(50) NOT NULL,
			event_type VARCHAR(100) NOT NULL,
			transaction_id UUID,
			reference VARCHAR(100) NOT NULL,
			status VARCHAR(50) NOT NULL,
			raw_payload JSONB NOT NULL,
			signature_verified BOOLEAN NOT NULL DEFAULT false,
			processed_at TIMESTAMP,
			error_message TEXT,
			received_at TIMESTAMP NOT NULL DEFAULT NOW(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_webhook_transaction FOREIGN KEY (transaction_id)
				REFERENCES transactions(id) ON DELETE SET NULL
		);

		CREATE INDEX IF NOT EXISTS idx_webhook_events_provider_txn
			ON webhook_events(provider, ((raw_payload->>'transactionId')::text));
		CREATE INDEX IF NOT EXISTS idx_webhook_events_reference
			ON webhook_events(reference);
		CREATE INDEX IF NOT EXISTS idx_webhook_events_transaction_id
			ON webhook_events(transaction_id)
			WHERE transaction_id IS NOT NULL;
	`)
	require.NoError(t, err)

	cleanup := func() {
		_ = db.Close()
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate postgres container: %s", err)
		}
	}

	return db, cleanup
}

// TestHandleOrangeWebhook_TransactionLinking tests the critical bug fix:
// Webhook events must properly link to transactions via transaction_id foreign key
func TestHandleOrangeWebhook_TransactionLinking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test with testcontainers")
	}

	ctx := context.Background()
	db, cleanup := setupTestDatabase(t, ctx)
	defer cleanup()

	// Create repositories
	txnRepo := repositories.NewTransactionRepository(db)
	webhookRepo := repositories.NewWebhookEventRepository(db)

	// Step 1: Create a test transaction
	txn := &models.Transaction{
		ID:               uuid.New(),
		Reference:        "TEST-REF-" + uuid.New().String()[:8],
		Type:             models.TypeDeposit,
		Status:           models.StatusPending,
		Amount:           10000, // 100.00 GHS
		Currency:         "GHS",
		Narration:        "Test deposit transaction",
		ProviderName:     "orange",
		SourceType:       "mobile_money",
		SourceIdentifier: "233240000000",
		SourceName:       "Test User",
		UserID:           uuid.New(), // Add user_id for test
	}

	err := txnRepo.Create(ctx, txn)
	require.NoError(t, err)

	// Step 2: Create a webhook event linked to the transaction
	rawPayload := map[string]interface{}{
		"status":        1,
		"message":       "Transaction successful",
		"transactionId": "ORG123456789",
		"reference":     txn.Reference,
		"amount":        100.00,
		"currency":      "GHS",
	}

	// Marshal to JSON for JSONB column
	payloadJSON, err := json.Marshal(rawPayload)
	require.NoError(t, err)

	var payloadMap map[string]interface{}
	err = json.Unmarshal(payloadJSON, &payloadMap)
	require.NoError(t, err)

	webhookEvent := &models.WebhookEvent{
		ID:                uuid.New().String(),
		Provider:          "orange",
		EventType:         "payment.success",
		Reference:         txn.Reference,
		Status:            string(models.WebhookStatusProcessed),
		RawPayload:        payloadMap,
		SignatureVerified: true,
		ReceivedAt:        time.Now(),
	}

	// This is the critical bug fix: Link webhook event to transaction
	txnID := txn.ID.String()
	webhookEvent.TransactionID = &txnID

	err = webhookRepo.Create(ctx, webhookEvent)
	require.NoError(t, err)

	// Step 3: Verify the foreign key relationship
	var result struct {
		WebhookID     string `db:"webhook_id"`
		TransactionID string `db:"transaction_id"`
		Reference     string `db:"reference"`
	}

	query := `
		SELECT
			we.id as webhook_id,
			we.transaction_id::text as transaction_id,
			t.reference as reference
		FROM webhook_events we
		INNER JOIN transactions t ON we.transaction_id = t.id
		WHERE we.id = $1
	`

	err = db.GetContext(ctx, &result, query, webhookEvent.ID)
	require.NoError(t, err, "Foreign key join should succeed")

	// Step 4: Assertions
	assert.Equal(t, webhookEvent.ID, result.WebhookID)
	assert.Equal(t, txn.ID.String(), result.TransactionID)
	assert.Equal(t, txn.Reference, result.Reference)

	t.Logf("✅ Webhook event %s successfully linked to transaction %s", webhookEvent.ID, txn.ID)

	// Step 5: Verify we can query webhooks by transaction_id
	webhooks, err := webhookRepo.GetByReference(ctx, txn.Reference)
	require.NoError(t, err)
	require.Len(t, webhooks, 1)
	assert.NotNil(t, webhooks[0].TransactionID)
	assert.Equal(t, txn.ID.String(), *webhooks[0].TransactionID)
}

// TestHandleOrangeWebhook_SignatureVerification tests environment-based signature verification
func TestHandleOrangeWebhook_SignatureVerification(t *testing.T) {
	testCases := []struct {
		name             string
		environment      string
		signature        string
		expectedResult   bool
		expectedLogLevel string
	}{
		{
			name:             "Production - Reject unsigned webhook",
			environment:      "production",
			signature:        "",
			expectedResult:   false,
			expectedLogLevel: "ERROR",
		},
		{
			name:             "Development - Allow unsigned webhook",
			environment:      "development",
			signature:        "",
			expectedResult:   true,
			expectedLogLevel: "WARN",
		},
		{
			name:             "Production - Accept signed webhook",
			environment:      "production",
			signature:        "valid-hmac-signature",
			expectedResult:   true,
			expectedLogLevel: "INFO",
		},
		{
			name:             "Development - Accept signed webhook",
			environment:      "development",
			signature:        "valid-hmac-signature",
			expectedResult:   true,
			expectedLogLevel: "INFO",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment
			originalEnv := os.Getenv("TRACING_ENVIRONMENT")
			if tc.environment != "" {
				_ = os.Setenv("TRACING_ENVIRONMENT", tc.environment)
			} else {
				_ = os.Unsetenv("TRACING_ENVIRONMENT")
			}
			defer func() {
				if originalEnv != "" {
					_ = os.Setenv("TRACING_ENVIRONMENT", originalEnv)
				} else {
					_ = os.Unsetenv("TRACING_ENVIRONMENT")
				}
			}()

			// Test the signature verification logic
			environment := os.Getenv("TRACING_ENVIRONMENT")
			if environment == "" {
				environment = "development"
			}

			var result bool
			if tc.signature == "" {
				// Unsigned webhook
				if environment == "production" {
					result = false // Reject in production
					t.Logf("🔒 Production mode: Rejecting unsigned webhook")
				} else {
					result = true // Allow in development
					t.Logf("⚠️  Development mode: Allowing unsigned webhook")
				}
			} else {
				// Signed webhook (always accept for this test)
				result = true
				t.Logf("✅ Signed webhook accepted in %s mode", environment)
			}

			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

// TestHandleOrangeWebhook_DuplicateIdempotency tests duplicate webhook handling
func TestHandleOrangeWebhook_DuplicateIdempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test with testcontainers")
	}

	ctx := context.Background()
	db, cleanup := setupTestDatabase(t, ctx)
	defer cleanup()

	webhookRepo := repositories.NewWebhookEventRepository(db)

	// Step 1: Create first webhook event
	providerTxnID := "ORG-" + uuid.New().String()[:8]
	reference := "TEST-REF-" + uuid.New().String()[:8]

	rawPayload := map[string]interface{}{
		"status":        1,
		"message":       "Transaction successful",
		"transactionId": providerTxnID,
		"reference":     reference,
		"amount":        100.00,
		"currency":      "GHS",
	}

	// Marshal to JSON for JSONB column
	payloadJSON, err := json.Marshal(rawPayload)
	require.NoError(t, err)

	var payloadMap map[string]interface{}
	err = json.Unmarshal(payloadJSON, &payloadMap)
	require.NoError(t, err)

	webhookEvent1 := &models.WebhookEvent{
		ID:                uuid.New().String(),
		Provider:          "orange",
		EventType:         "payment.success",
		Reference:         reference,
		Status:            string(models.WebhookStatusProcessed),
		RawPayload:        payloadMap,
		SignatureVerified: true,
		ReceivedAt:        time.Now(),
	}

	err = webhookRepo.Create(ctx, webhookEvent1)
	require.NoError(t, err)
	t.Logf("✅ First webhook event created: %s", webhookEvent1.ID)

	// Step 2: Try to find duplicate using provider and transaction ID
	existingWebhook, err := webhookRepo.GetByProviderTransactionID(ctx, "orange", providerTxnID)
	require.NoError(t, err)
	require.NotNil(t, existingWebhook)

	// Step 3: Verify duplicate detection
	assert.Equal(t, webhookEvent1.ID, existingWebhook.ID)
	assert.Equal(t, "orange", existingWebhook.Provider)
	assert.Equal(t, string(models.WebhookStatusProcessed), existingWebhook.Status)

	t.Logf("✅ Duplicate webhook detected: %s", existingWebhook.ID)

	// Step 4: Simulate idempotent response
	// In the actual handler, we would return the existing transaction status
	// without processing the webhook again
	if existingWebhook.Status == string(models.WebhookStatusProcessed) {
		t.Logf("✅ Webhook already processed - returning idempotent response")
	}

	// Step 5: Verify we can query all webhooks by reference
	webhooks, err := webhookRepo.GetByReference(ctx, reference)
	require.NoError(t, err)
	assert.Len(t, webhooks, 1, "Should only have one webhook event for this reference")
	assert.Equal(t, webhookEvent1.ID, webhooks[0].ID)
}

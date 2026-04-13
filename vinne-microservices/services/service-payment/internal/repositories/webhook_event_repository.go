package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/randco/service-payment/internal/models"
)

// WebhookEventRepository defines the interface for webhook event operations
type WebhookEventRepository interface {
	Create(ctx context.Context, event *models.WebhookEvent) error
	GetByProviderTransactionID(ctx context.Context, providerName, providerTransactionID string) (*models.WebhookEvent, error)
	GetByReference(ctx context.Context, reference string) ([]*models.WebhookEvent, error)
	Update(ctx context.Context, event *models.WebhookEvent) error
}

// webhookEventRepository implements WebhookEventRepository
type webhookEventRepository struct {
	db *sqlx.DB
}

// NewWebhookEventRepository creates a new webhook event repository
func NewWebhookEventRepository(db *sqlx.DB) WebhookEventRepository {
	return &webhookEventRepository{db: db}
}

// Create creates a new webhook event record
func (r *webhookEventRepository) Create(ctx context.Context, event *models.WebhookEvent) error {
	query := `
		INSERT INTO webhook_events (
			id, provider, event_type, transaction_id, reference, status,
			raw_payload, signature_verified, received_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = time.Now()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	// Marshal RawPayload to JSON for JSONB column
	rawPayloadJSON, err := json.Marshal(event.RawPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal raw_payload: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		event.ID,
		event.Provider,
		event.EventType,
		event.TransactionID,
		event.Reference,
		event.Status,
		rawPayloadJSON,
		event.SignatureVerified,
		event.ReceivedAt,
		event.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create webhook event: %w", err)
	}

	return nil
}

// GetByProviderTransactionID retrieves a webhook event by provider and provider transaction ID
func (r *webhookEventRepository) GetByProviderTransactionID(ctx context.Context, providerName, providerTransactionID string) (*models.WebhookEvent, error) {
	query := `
		SELECT id, provider, event_type, transaction_id, reference, status,
		       raw_payload, processed_at, error_message, signature_verified,
		       received_at, created_at
		FROM webhook_events
		WHERE provider = $1 AND raw_payload->>'transactionId' = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	type webhookEventRow struct {
		ID                string         `db:"id"`
		Provider          string         `db:"provider"`
		EventType         string         `db:"event_type"`
		TransactionID     *string        `db:"transaction_id"`
		Reference         string         `db:"reference"`
		Status            string         `db:"status"`
		RawPayload        []byte         `db:"raw_payload"`
		ProcessedAt       *time.Time     `db:"processed_at"`
		ErrorMessage      sql.NullString `db:"error_message"`
		SignatureVerified bool           `db:"signature_verified"`
		ReceivedAt        time.Time      `db:"received_at"`
		CreatedAt         time.Time      `db:"created_at"`
	}

	var row webhookEventRow
	err := r.db.GetContext(ctx, &row, query, providerName, providerTransactionID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("webhook event not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get webhook event: %w", err)
	}

	// Unmarshal raw_payload from JSONB
	var rawPayload map[string]interface{}
	if err := json.Unmarshal(row.RawPayload, &rawPayload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw_payload: %w", err)
	}

	event := &models.WebhookEvent{
		ID:                row.ID,
		Provider:          row.Provider,
		EventType:         row.EventType,
		TransactionID:     row.TransactionID,
		Reference:         row.Reference,
		Status:            row.Status,
		RawPayload:        rawPayload,
		ProcessedAt:       row.ProcessedAt,
		ErrorMessage:      row.ErrorMessage.String,
		SignatureVerified: row.SignatureVerified,
		ReceivedAt:        row.ReceivedAt,
		CreatedAt:         row.CreatedAt,
	}

	return event, nil
}

// GetByReference retrieves all webhook events for a given transaction reference
func (r *webhookEventRepository) GetByReference(ctx context.Context, reference string) ([]*models.WebhookEvent, error) {
	query := `
		SELECT id, provider, event_type, transaction_id, reference, status,
		       raw_payload, processed_at, error_message, signature_verified,
		       received_at, created_at
		FROM webhook_events
		WHERE reference = $1
		ORDER BY created_at DESC
	`

	type webhookEventRow struct {
		ID                string         `db:"id"`
		Provider          string         `db:"provider"`
		EventType         string         `db:"event_type"`
		TransactionID     *string        `db:"transaction_id"`
		Reference         string         `db:"reference"`
		Status            string         `db:"status"`
		RawPayload        []byte         `db:"raw_payload"`
		ProcessedAt       *time.Time     `db:"processed_at"`
		ErrorMessage      sql.NullString `db:"error_message"`
		SignatureVerified bool           `db:"signature_verified"`
		ReceivedAt        time.Time      `db:"received_at"`
		CreatedAt         time.Time      `db:"created_at"`
	}

	var rows []webhookEventRow
	err := r.db.SelectContext(ctx, &rows, query, reference)
	if err != nil {
		return nil, fmt.Errorf("failed to get webhook events: %w", err)
	}

	// Convert rows to events with proper JSONB unmarshaling
	events := make([]*models.WebhookEvent, len(rows))
	for i, row := range rows {
		var rawPayload map[string]interface{}
		if err := json.Unmarshal(row.RawPayload, &rawPayload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal raw_payload: %w", err)
		}

		events[i] = &models.WebhookEvent{
			ID:                row.ID,
			Provider:          row.Provider,
			EventType:         row.EventType,
			TransactionID:     row.TransactionID,
			Reference:         row.Reference,
			Status:            row.Status,
			RawPayload:        rawPayload,
			ProcessedAt:       row.ProcessedAt,
			ErrorMessage:      row.ErrorMessage.String,
			SignatureVerified: row.SignatureVerified,
			ReceivedAt:        row.ReceivedAt,
			CreatedAt:         row.CreatedAt,
		}
	}

	return events, nil
}

// Update updates a webhook event
func (r *webhookEventRepository) Update(ctx context.Context, event *models.WebhookEvent) error {
	query := `
		UPDATE webhook_events
		SET status = $1, processed_at = $2, error_message = $3, transaction_id = $4
		WHERE id = $5
	`

	_, err := r.db.ExecContext(ctx, query,
		event.Status,
		event.ProcessedAt,
		event.ErrorMessage,
		event.TransactionID,
		event.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update webhook event: %w", err)
	}

	return nil
}

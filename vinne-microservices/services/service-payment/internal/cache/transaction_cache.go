package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/randco/service-payment/internal/models"
)

// TransactionCache provides caching for transaction data
type TransactionCache struct {
	cache Cache
}

// NewTransactionCache creates a new transaction cache
func NewTransactionCache(cache Cache) *TransactionCache {
	return &TransactionCache{
		cache: cache,
	}
}

// CachedTransaction represents a cached transaction (lighter than full model)
type CachedTransaction struct {
	ID                    string     `json:"id"`
	Reference             string     `json:"reference"`
	ProviderTransactionID string     `json:"provider_transaction_id"`
	Type                  string     `json:"type"`
	Status                string     `json:"status"`
	Amount                int64      `json:"amount"`
	Currency              string     `json:"currency"`
	ProviderName          string     `json:"provider_name"`
	UserID                string     `json:"user_id"`
	RequestedAt           time.Time  `json:"requested_at"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
	ErrorMessage          string     `json:"error_message,omitempty"`
}

// GetTransaction retrieves a transaction from cache by reference
func (tc *TransactionCache) GetTransaction(ctx context.Context, reference string) (*CachedTransaction, error) {
	ctx, span := tracer.Start(ctx, "transaction_cache.get",
		trace.WithAttributes(attribute.String("reference", reference)))
	defer span.End()

	key := fmt.Sprintf("transaction:ref:%s", reference)

	var tx CachedTransaction
	err := tc.cache.Get(ctx, key, &tx)
	if err != nil {
		span.SetAttributes(attribute.Bool("cache_hit", false))
		return nil, nil // Cache miss, not an error
	}

	span.SetAttributes(
		attribute.Bool("cache_hit", true),
		attribute.String("status", tx.Status),
	)
	return &tx, nil
}

// SetTransaction stores a transaction in cache
func (tc *TransactionCache) SetTransaction(ctx context.Context, tx *models.Transaction) error {
	ctx, span := tracer.Start(ctx, "transaction_cache.set",
		trace.WithAttributes(
			attribute.String("reference", tx.Reference),
			attribute.String("status", string(tx.Status)),
		))
	defer span.End()

	providerTxID := ""
	if tx.ProviderTransactionID != nil {
		providerTxID = *tx.ProviderTransactionID
	}

	errorMsg := ""
	if tx.ErrorMessage != nil {
		errorMsg = *tx.ErrorMessage
	}

	cached := &CachedTransaction{
		ID:                    tx.ID.String(),
		Reference:             tx.Reference,
		ProviderTransactionID: providerTxID,
		Type:                  string(tx.Type),
		Status:                string(tx.Status),
		Amount:                tx.Amount,
		Currency:              tx.Currency,
		ProviderName:          tx.ProviderName,
		UserID:                tx.UserID.String(),
		RequestedAt:           tx.RequestedAt,
		CompletedAt:           tx.CompletedAt,
		ErrorMessage:          errorMsg,
	}

	key := fmt.Sprintf("transaction:ref:%s", tx.Reference)

	// Cache for different durations based on status
	var ttl time.Duration
	if tx.IsTerminal() {
		// Terminal states (SUCCESS, FAILED) - cache longer
		ttl = 24 * time.Hour
	} else {
		// Pending states - cache shorter to ensure fresh data
		ttl = 5 * time.Minute
	}

	err := tc.cache.Set(ctx, key, cached, ttl)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to cache transaction: %w", err)
	}

	return nil
}

// InvalidateTransaction removes a transaction from cache
func (tc *TransactionCache) InvalidateTransaction(ctx context.Context, reference string) error {
	ctx, span := tracer.Start(ctx, "transaction_cache.invalidate",
		trace.WithAttributes(attribute.String("reference", reference)))
	defer span.End()

	key := fmt.Sprintf("transaction:ref:%s", reference)

	err := tc.cache.Delete(ctx, key)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to invalidate transaction: %w", err)
	}

	return nil
}

// GetTransactionByID retrieves a transaction from cache by ID
func (tc *TransactionCache) GetTransactionByID(ctx context.Context, id uuid.UUID) (*CachedTransaction, error) {
	ctx, span := tracer.Start(ctx, "transaction_cache.get_by_id",
		trace.WithAttributes(attribute.String("id", id.String())))
	defer span.End()

	key := fmt.Sprintf("transaction:id:%s", id.String())

	var tx CachedTransaction
	err := tc.cache.Get(ctx, key, &tx)
	if err != nil {
		span.SetAttributes(attribute.Bool("cache_hit", false))
		return nil, nil // Cache miss, not an error
	}

	span.SetAttributes(
		attribute.Bool("cache_hit", true),
		attribute.String("status", tx.Status),
	)
	return &tx, nil
}

// SetTransactionByID stores a transaction in cache indexed by ID
func (tc *TransactionCache) SetTransactionByID(ctx context.Context, tx *models.Transaction) error {
	ctx, span := tracer.Start(ctx, "transaction_cache.set_by_id",
		trace.WithAttributes(
			attribute.String("id", tx.ID.String()),
			attribute.String("status", string(tx.Status)),
		))
	defer span.End()

	providerTxID := ""
	if tx.ProviderTransactionID != nil {
		providerTxID = *tx.ProviderTransactionID
	}

	errorMsg := ""
	if tx.ErrorMessage != nil {
		errorMsg = *tx.ErrorMessage
	}

	cached := &CachedTransaction{
		ID:                    tx.ID.String(),
		Reference:             tx.Reference,
		ProviderTransactionID: providerTxID,
		Type:                  string(tx.Type),
		Status:                string(tx.Status),
		Amount:                tx.Amount,
		Currency:              tx.Currency,
		ProviderName:          tx.ProviderName,
		UserID:                tx.UserID.String(),
		RequestedAt:           tx.RequestedAt,
		CompletedAt:           tx.CompletedAt,
		ErrorMessage:          errorMsg,
	}

	key := fmt.Sprintf("transaction:id:%s", tx.ID.String())

	// Same TTL logic as SetTransaction
	var ttl time.Duration
	if tx.IsTerminal() {
		ttl = 24 * time.Hour
	} else {
		ttl = 5 * time.Minute
	}

	err := tc.cache.Set(ctx, key, cached, ttl)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to cache transaction by ID: %w", err)
	}

	return nil
}

// UpdateTransactionStatus updates only the status in cache
func (tc *TransactionCache) UpdateTransactionStatus(ctx context.Context, reference string, status string) error {
	ctx, span := tracer.Start(ctx, "transaction_cache.update_status",
		trace.WithAttributes(
			attribute.String("reference", reference),
			attribute.String("status", status),
		))
	defer span.End()

	// Simple approach: invalidate the cache entry
	// The next read will fetch fresh data from database
	return tc.InvalidateTransaction(ctx, reference)
}

// CacheTransactionStatus stores just the status for quick lookups
func (tc *TransactionCache) CacheTransactionStatus(ctx context.Context, reference string, status string, ttl time.Duration) error {
	ctx, span := tracer.Start(ctx, "transaction_cache.cache_status",
		trace.WithAttributes(
			attribute.String("reference", reference),
			attribute.String("status", status),
		))
	defer span.End()

	key := fmt.Sprintf("transaction:status:%s", reference)

	err := tc.cache.Set(ctx, key, map[string]string{
		"status": status,
	}, ttl)

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to cache status: %w", err)
	}

	return nil
}

// GetTransactionStatus retrieves just the status from cache
func (tc *TransactionCache) GetTransactionStatus(ctx context.Context, reference string) (string, error) {
	ctx, span := tracer.Start(ctx, "transaction_cache.get_status",
		trace.WithAttributes(attribute.String("reference", reference)))
	defer span.End()

	key := fmt.Sprintf("transaction:status:%s", reference)

	var data map[string]string
	err := tc.cache.Get(ctx, key, &data)
	if err != nil {
		span.SetAttributes(attribute.Bool("cache_hit", false))
		return "", nil // Cache miss
	}

	status := data["status"]
	span.SetAttributes(
		attribute.Bool("cache_hit", true),
		attribute.String("status", status),
	)
	return status, nil
}

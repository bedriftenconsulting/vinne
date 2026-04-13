package cache

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// IdempotencyCache provides Level 1 (fast path) idempotency checking using Redis
type IdempotencyCache struct {
	cache Cache
}

// NewIdempotencyCache creates a new idempotency cache
func NewIdempotencyCache(cache Cache) *IdempotencyCache {
	return &IdempotencyCache{
		cache: cache,
	}
}

// IdempotencyCacheEntry represents a cached idempotency entry
type IdempotencyCacheEntry struct {
	Reference     string                 `json:"reference"`
	TransactionID string                 `json:"transaction_id"`
	Status        string                 `json:"status"`
	StatusCode    int                    `json:"status_code"`
	Response      map[string]interface{} `json:"response"`
	CreatedAt     time.Time              `json:"created_at"`
}

// CheckIdempotency checks if a request has been processed (Level 1 - fast path)
func (ic *IdempotencyCache) CheckIdempotency(ctx context.Context, idempotencyKey string) (*IdempotencyCacheEntry, error) {
	ctx, span := tracer.Start(ctx, "idempotency_cache.check",
		trace.WithAttributes(attribute.String("idempotency_key", idempotencyKey)))
	defer span.End()

	key := fmt.Sprintf("idempotency:%s", idempotencyKey)

	var entry IdempotencyCacheEntry
	err := ic.cache.Get(ctx, key, &entry)
	if err != nil {
		// Cache miss - this is normal, not an error
		span.SetAttributes(attribute.Bool("cache_hit", false))
		return nil, nil
	}

	span.SetAttributes(
		attribute.Bool("cache_hit", true),
		attribute.String("status", entry.Status),
	)
	return &entry, nil
}

// StoreIdempotency stores an idempotency result in cache (Level 1)
func (ic *IdempotencyCache) StoreIdempotency(ctx context.Context, idempotencyKey string, entry *IdempotencyCacheEntry) error {
	ctx, span := tracer.Start(ctx, "idempotency_cache.store",
		trace.WithAttributes(
			attribute.String("idempotency_key", idempotencyKey),
			attribute.String("status", entry.Status),
		))
	defer span.End()

	key := fmt.Sprintf("idempotency:%s", idempotencyKey)

	// Store for 24 hours (idempotency window)
	ttl := 24 * time.Hour

	err := ic.cache.Set(ctx, key, entry, ttl)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to store idempotency: %w", err)
	}

	return nil
}

// ReserveIdempotencyKey atomically reserves an idempotency key (prevents concurrent processing)
func (ic *IdempotencyCache) ReserveIdempotencyKey(ctx context.Context, idempotencyKey string, ttl time.Duration) (bool, error) {
	ctx, span := tracer.Start(ctx, "idempotency_cache.reserve",
		trace.WithAttributes(
			attribute.String("idempotency_key", idempotencyKey),
			attribute.Int64("ttl_seconds", int64(ttl.Seconds())),
		))
	defer span.End()

	key := fmt.Sprintf("idempotency:processing:%s", idempotencyKey)

	// Use SetNX to atomically reserve the key
	reserved, err := ic.cache.SetNX(ctx, key, map[string]interface{}{
		"reserved_at": time.Now(),
	}, ttl)

	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("failed to reserve idempotency key: %w", err)
	}

	span.SetAttributes(attribute.Bool("reserved", reserved))
	return reserved, nil
}

// ReleaseIdempotencyKey releases a reserved idempotency key
func (ic *IdempotencyCache) ReleaseIdempotencyKey(ctx context.Context, idempotencyKey string) error {
	ctx, span := tracer.Start(ctx, "idempotency_cache.release",
		trace.WithAttributes(attribute.String("idempotency_key", idempotencyKey)))
	defer span.End()

	key := fmt.Sprintf("idempotency:processing:%s", idempotencyKey)

	err := ic.cache.Delete(ctx, key)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to release idempotency key: %w", err)
	}

	return nil
}

// DeleteIdempotency removes an idempotency entry from cache
func (ic *IdempotencyCache) DeleteIdempotency(ctx context.Context, idempotencyKey string) error {
	ctx, span := tracer.Start(ctx, "idempotency_cache.delete",
		trace.WithAttributes(attribute.String("idempotency_key", idempotencyKey)))
	defer span.End()

	key := fmt.Sprintf("idempotency:%s", idempotencyKey)

	err := ic.cache.Delete(ctx, key)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to delete idempotency: %w", err)
	}

	return nil
}

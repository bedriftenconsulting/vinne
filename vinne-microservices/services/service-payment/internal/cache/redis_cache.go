package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("payment-service/cache")

// Cache defines the interface for caching operations
type Cache interface {
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string, dest interface{}) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
	Increment(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

// redisCache implements Cache using Redis
type redisCache struct {
	client *redis.Client
	prefix string
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(client *redis.Client, prefix string) Cache {
	return &redisCache{
		client: client,
		prefix: prefix,
	}
}

// makeKey creates a prefixed key
func (c *redisCache) makeKey(key string) string {
	if c.prefix == "" {
		return key
	}
	return fmt.Sprintf("%s:%s", c.prefix, key)
}

// Set stores a value in cache with TTL
func (c *redisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	ctx, span := tracer.Start(ctx, "redis_cache.set",
		trace.WithAttributes(
			attribute.String("key", key),
			attribute.Int64("ttl_seconds", int64(ttl.Seconds())),
		))
	defer span.End()

	fullKey := c.makeKey(key)

	// Marshal value to JSON
	data, err := json.Marshal(value)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	err = c.client.Set(ctx, fullKey, data, ttl).Err()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to set cache: %w", err)
	}

	span.SetAttributes(attribute.Int("value_size", len(data)))
	return nil
}

// Get retrieves a value from cache
func (c *redisCache) Get(ctx context.Context, key string, dest interface{}) error {
	ctx, span := tracer.Start(ctx, "redis_cache.get",
		trace.WithAttributes(attribute.String("key", key)))
	defer span.End()

	fullKey := c.makeKey(key)

	data, err := c.client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			span.SetAttributes(attribute.Bool("found", false))
			return fmt.Errorf("cache miss")
		}
		span.RecordError(err)
		return fmt.Errorf("failed to get cache: %w", err)
	}

	// Unmarshal JSON to destination
	err = json.Unmarshal(data, dest)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to unmarshal value: %w", err)
	}

	span.SetAttributes(
		attribute.Bool("found", true),
		attribute.Int("value_size", len(data)),
	)
	return nil
}

// Delete removes a value from cache
func (c *redisCache) Delete(ctx context.Context, key string) error {
	ctx, span := tracer.Start(ctx, "redis_cache.delete",
		trace.WithAttributes(attribute.String("key", key)))
	defer span.End()

	fullKey := c.makeKey(key)

	err := c.client.Del(ctx, fullKey).Err()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to delete cache: %w", err)
	}

	return nil
}

// Exists checks if a key exists in cache
func (c *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	ctx, span := tracer.Start(ctx, "redis_cache.exists",
		trace.WithAttributes(attribute.String("key", key)))
	defer span.End()

	fullKey := c.makeKey(key)

	count, err := c.client.Exists(ctx, fullKey).Result()
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	exists := count > 0
	span.SetAttributes(attribute.Bool("exists", exists))
	return exists, nil
}

// SetNX sets a value only if the key doesn't exist (atomic)
func (c *redisCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	ctx, span := tracer.Start(ctx, "redis_cache.setnx",
		trace.WithAttributes(
			attribute.String("key", key),
			attribute.Int64("ttl_seconds", int64(ttl.Seconds())),
		))
	defer span.End()

	fullKey := c.makeKey(key)

	// Marshal value to JSON
	data, err := json.Marshal(value)
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("failed to marshal value: %w", err)
	}

	success, err := c.client.SetNX(ctx, fullKey, data, ttl).Result()
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("failed to setnx: %w", err)
	}

	span.SetAttributes(
		attribute.Bool("success", success),
		attribute.Int("value_size", len(data)),
	)
	return success, nil
}

// Increment atomically increments a counter
func (c *redisCache) Increment(ctx context.Context, key string) (int64, error) {
	ctx, span := tracer.Start(ctx, "redis_cache.increment",
		trace.WithAttributes(attribute.String("key", key)))
	defer span.End()

	fullKey := c.makeKey(key)

	value, err := c.client.Incr(ctx, fullKey).Result()
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("failed to increment: %w", err)
	}

	span.SetAttributes(attribute.Int64("value", value))
	return value, nil
}

// Expire sets the TTL for an existing key
func (c *redisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	ctx, span := tracer.Start(ctx, "redis_cache.expire",
		trace.WithAttributes(
			attribute.String("key", key),
			attribute.Int64("ttl_seconds", int64(ttl.Seconds())),
		))
	defer span.End()

	fullKey := c.makeKey(key)

	err := c.client.Expire(ctx, fullKey, ttl).Err()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to set expiration: %w", err)
	}

	return nil
}

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/randco/randco-microservices/shared/common/errors"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type CacheManager interface {
	SetTemplate(ctx context.Context, templateID string, template any) error
	GetTemplate(ctx context.Context, templateID string, dest any) error
	DeleteTemplate(ctx context.Context, templateID string) error

	// Provider response caching
	SetProviderResponse(ctx context.Context, notificationID string, provider string, response any) error
	GetProviderResponse(ctx context.Context, notificationID string, provider string, dest any) error

	// Rate limiting
	SetRateLimit(ctx context.Context, serviceID string, window string, count int64, expiry time.Duration) error
	GetRateLimit(ctx context.Context, serviceID string, window string) (int64, error)
	IncrementRateLimit(ctx context.Context, serviceID string, window string, limit int64, windowDuration time.Duration) (int64, error)

	// Generic operations
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string, dest any) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)

	// Health check
	Health(ctx context.Context) error
}

type redisCacheManager struct {
	client *redis.Client
	tracer trace.Tracer
}

func NewRedisCacheManager(client *redis.Client) CacheManager {
	return &redisCacheManager{
		client: client,
		tracer: otel.Tracer("github.com/randco/randco-microservices/services/service-notification/cache"),
	}
}

func (c *redisCacheManager) SetTemplate(ctx context.Context, templateID string, template any) error {
	return c.Set(ctx, templateKey(templateID), template, time.Hour)
}

func (c *redisCacheManager) GetTemplate(ctx context.Context, templateID string, dest any) error {
	return c.Get(ctx, templateKey(templateID), dest)
}

func (c *redisCacheManager) DeleteTemplate(ctx context.Context, templateID string) error {
	return c.Delete(ctx, templateKey(templateID))
}

func (c *redisCacheManager) SetProviderResponse(ctx context.Context, notificationID string, provider string, response any) error {
	return c.Set(ctx, providerResponseKey(notificationID, provider), response, 5*time.Minute)
}

func (c *redisCacheManager) GetProviderResponse(ctx context.Context, notificationID string, provider string, dest any) error {
	return c.Get(ctx, providerResponseKey(notificationID, provider), dest)
}

func (c *redisCacheManager) SetRateLimit(ctx context.Context, serviceID string, window string, count int64, expiry time.Duration) error {
	key := rateLimitKey(serviceID, window)
	return c.client.Set(ctx, key, count, expiry).Err()
}

func (c *redisCacheManager) GetRateLimit(ctx context.Context, serviceID string, window string) (int64, error) {
	key := rateLimitKey(serviceID, window)
	result, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, errors.NewInternalError("failed to get rate limit", err)
	}

	var count int64
	if _, err := fmt.Sscanf(result, "%d", &count); err != nil {
		return 0, errors.NewInternalError("failed to parse rate limit", err)
	}
	return count, nil
}

func (c *redisCacheManager) IncrementRateLimit(ctx context.Context, serviceID string, window string, limit int64, windowDuration time.Duration) (int64, error) {
	key := rateLimitKey(serviceID, window)

	// Use Redis pipeline for atomic operations
	pipe := c.client.Pipeline()

	// Get current count
	getCmd := pipe.Get(ctx, key)
	// Increment counter
	incrCmd := pipe.Incr(ctx, key)
	// Set expiry if key doesn't exist
	pipe.Expire(ctx, key, windowDuration)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, errors.NewInternalError("failed to increment rate limit", err)
	}

	currentCount := incrCmd.Val()
	getResult := getCmd.Val()

	// Check if this is a new window (first increment)
	if getResult == "" || getResult == "0" {
		currentCount = 1
		pipe := c.client.Pipeline()
		pipe.Set(ctx, key, 1, windowDuration)
		pipe.Exec(ctx)
	}

	if currentCount > limit {
		return currentCount, errors.NewRateLimitError("rate limit exceeded")
	}

	return currentCount, nil
}

// Set sets a value in cache with TTL
func (c *redisCacheManager) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	ctx, span := c.tracer.Start(ctx, "cache.set")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.String("cache.operation", "set"),
		attribute.Int64("cache.ttl_seconds", int64(ttl.Seconds())),
	)

	data, err := json.Marshal(value)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal value")
		return errors.NewInternalError("failed to marshal value", err)
	}

	span.SetAttributes(attribute.Int("cache.value_size", len(data)))

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to set cache value")
		return errors.NewInternalError("failed to set cache value", err)
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// Get retrieves a value from cache
func (c *redisCacheManager) Get(ctx context.Context, key string, dest any) error {
	ctx, span := c.tracer.Start(ctx, "cache.get")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.String("cache.operation", "get"),
	)

	data, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		span.SetAttributes(attribute.Bool("cache.hit", false))
		span.SetStatus(codes.Ok, "cache miss")
		return errors.NewNotFoundError("cache key not found")
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get cache value")
		return errors.NewInternalError("failed to get cache value", err)
	}

	span.SetAttributes(
		attribute.Bool("cache.hit", true),
		attribute.Int("cache.value_size", len(data)),
	)

	if err := json.Unmarshal([]byte(data), dest); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal cache value")
		return errors.NewInternalError("failed to unmarshal cache value", err)
	}

	span.SetStatus(codes.Ok, "cache hit")
	return nil
}

// Delete removes a key from cache
func (c *redisCacheManager) Delete(ctx context.Context, key string) error {
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return errors.NewInternalError("failed to delete cache key", err)
	}
	return nil
}

// Exists checks if a key exists in cache
func (c *redisCacheManager) Exists(ctx context.Context, key string) (bool, error) {
	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, errors.NewInternalError("failed to check if cache key exists", err)
	}
	return count > 0, nil
}

// SetNX sets a value only if it doesn't exist (atomic operation)
func (c *redisCacheManager) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, errors.NewInternalError("failed to marshal value", err)
	}

	result, err := c.client.SetNX(ctx, key, data, ttl).Result()
	if err != nil {
		return false, errors.NewInternalError("failed to set cache value", err)
	}

	return result, nil
}

// Health checks Redis connectivity
func (c *redisCacheManager) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := c.client.Ping(ctx).Err(); err != nil {
		return errors.NewInternalError("Redis health check failed", err)
	}
	return nil
}

// Key helper functions
func templateKey(templateID string) string {
	return fmt.Sprintf("notification:cache:template:%s", templateID)
}

func providerResponseKey(notificationID, provider string) string {
	return fmt.Sprintf("notification:cache:provider:%s:%s", provider, notificationID)
}

func rateLimitKey(serviceID, window string) string {
	return fmt.Sprintf("notification:rl:%s:%s", serviceID, window)
}

// CacheError is a custom error type for cache operations
type CacheError struct {
	Operation string
	Key       string
	Err       error
}

func (e CacheError) Error() string {
	return fmt.Sprintf("cache operation %s failed for key %s: %v", e.Operation, e.Key, e.Err)
}

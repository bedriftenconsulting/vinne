package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter manages rate limiting for different notification channels using Redis
type RateLimiter struct {
	redis  *redis.Client
	config Config
}

// RedisClient interface for Redis operations (for testing)
type RedisClient interface {
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
}

// Config holds rate limiting configuration
type Config struct {
	EmailRatePerHour int // Number of emails allowed per hour
	SMSRatePerMinute int // Number of SMS allowed per minute
}

// NewRateLimiter creates a new distributed rate limiter using Redis
func NewRateLimiter(config Config, redisClient *redis.Client) *RateLimiter {
	return &RateLimiter{
		redis:  redisClient,
		config: config,
	}
}

// Allow checks if a request for the given channel is allowed using Redis
// It returns an error if the rate limit is exceeded
// Uses Lua script for atomic check-before-increment to prevent counter inflation
func (rl *RateLimiter) Allow(ctx context.Context, channel string) error {
	limit, window := rl.getLimitAndWindow(channel)
	if limit == 0 {
		// No rate limit configured for this channel, allow it
		return nil
	}

	// Generate Redis key with current time window
	now := time.Now()
	windowKey := rl.getWindowKey(channel, now, window)

	// Lua script for atomic check-and-increment
	// This script:
	// 1. Gets current count
	// 2. Checks if it's below limit
	// 3. Only increments if below limit
	// 4. Sets expiration on first increment
	// 5. Returns the count AFTER increment (if allowed) or current count (if denied)
	luaScript := `
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])

		local current = tonumber(redis.call('GET', key) or '0')

		-- Check if limit would be exceeded
		if current >= limit then
			return {0, current}  -- Denied, return current count
		end

		-- Increment counter
		local new_count = redis.call('INCR', key)

		-- Set expiration on first increment
		if new_count == 1 then
			redis.call('EXPIRE', key, window)
		end

		return {1, new_count}  -- Allowed, return new count
	`

	// Execute Lua script atomically
	result, err := rl.redis.Eval(ctx, luaScript, []string{windowKey}, limit, int64(window.Seconds())).Result()
	if err != nil {
		// If Redis fails, we log but don't block the request
		// This is a fail-open approach for availability
		return nil
	}

	// Parse result: [allowed, count]
	resultSlice, ok := result.([]interface{})
	if !ok || len(resultSlice) != 2 {
		// Unexpected result format, fail open
		return nil
	}

	allowed, _ := resultSlice[0].(int64)
	count, _ := resultSlice[1].(int64)

	// Check if request was allowed
	if allowed == 0 {
		return &RateLimitError{
			Channel: channel,
			Limit:   limit,
			Current: int(count),
			Message: fmt.Sprintf("Rate limit exceeded for %s notifications: %d/%d in current window", channel, count, limit),
		}
	}

	return nil
}

// Release decrements the rate limit counter when a send operation fails
// This ensures failed sends don't count against the rate limit
func (rl *RateLimiter) Release(ctx context.Context, channel string) error {
	limit, window := rl.getLimitAndWindow(channel)
	if limit == 0 {
		// No rate limit configured for this channel
		return nil
	}

	// Generate Redis key with current time window
	now := time.Now()
	windowKey := rl.getWindowKey(channel, now, window)

	// Decrement the counter, but don't go below 0
	luaScript := `
		local key = KEYS[1]
		local current = tonumber(redis.call('GET', key) or '0')

		if current > 0 then
			return redis.call('DECR', key)
		end

		return 0
	`

	_, err := rl.redis.Eval(ctx, luaScript, []string{windowKey}).Result()
	if err != nil {
		// Log error but don't fail the operation
		return err
	}

	return nil
}

// Wait is not supported with Redis-based rate limiting
// Use Allow() and implement retry logic in the caller
func (rl *RateLimiter) Wait(ctx context.Context, channel string) error {
	return rl.Allow(ctx, channel)
}

// getWindowKey generates a Redis key for the current time window
func (rl *RateLimiter) getWindowKey(channel string, now time.Time, window time.Duration) string {
	var windowStart int64

	switch channel {
	case "email":
		// For hourly limits, get the start of the current hour
		windowStart = now.Truncate(time.Hour).Unix()
	case "sms":
		// For minute limits, get the start of the current minute
		windowStart = now.Truncate(time.Minute).Unix()
	default:
		windowStart = now.Unix()
	}

	return fmt.Sprintf("ratelimit:%s:%d", channel, windowStart)
}

// getLimitAndWindow returns the limit and time window for a channel
func (rl *RateLimiter) getLimitAndWindow(channel string) (int, time.Duration) {
	switch channel {
	case "email":
		return rl.config.EmailRatePerHour, time.Hour
	case "sms":
		return rl.config.SMSRatePerMinute, time.Minute
	default:
		return 0, 0
	}
}

// RateLimitError represents a rate limit violation
type RateLimitError struct {
	Channel string
	Limit   int
	Current int
	Message string
}

func (e *RateLimitError) Error() string {
	return e.Message
}

// IsRateLimitError checks if an error is a rate limit error
func IsRateLimitError(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}

package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/redis/go-redis/v9"
)

// RateLimiter interface for different rate limiting strategies
type RateLimiter interface {
	Allow(key string) (bool, error)
}

// TokenBucketLimiter implements token bucket algorithm
type TokenBucketLimiter struct {
	capacity     int
	refillRate   int
	refillPeriod time.Duration
	buckets      sync.Map
	redis        *redis.Client
	useRedis     bool
	stopCh       chan struct{}
}

// bucket represents a token bucket
type bucket struct {
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucketLimiter creates a new token bucket rate limiter
func NewTokenBucketLimiter(capacity, refillRate int, refillPeriod time.Duration, redisClient *redis.Client) *TokenBucketLimiter {
	limiter := &TokenBucketLimiter{
		capacity:     capacity,
		refillRate:   refillRate,
		refillPeriod: refillPeriod,
		redis:        redisClient,
		useRedis:     redisClient != nil,
		stopCh:       make(chan struct{}),
	}

	// Start cleanup goroutine for in-memory buckets
	if !limiter.useRedis {
		go limiter.cleanupBuckets()
	}

	return limiter
}

// Allow checks if request is allowed
func (t *TokenBucketLimiter) Allow(key string) (bool, error) {
	if t.useRedis {
		return t.allowRedis(key)
	}
	return t.allowLocal(key), nil
}

// allowLocal uses in-memory storage
func (t *TokenBucketLimiter) allowLocal(key string) bool {
	now := time.Now()

	// Get or create bucket
	val, _ := t.buckets.LoadOrStore(key, &bucket{
		tokens:     t.capacity,
		lastRefill: now,
	})

	b := val.(*bucket)
	b.mu.Lock()
	defer b.mu.Unlock()

	// Calculate token refill based on elapsed time
	elapsed := now.Sub(b.lastRefill)
	if elapsed >= t.refillPeriod {
		// Calculate number of complete refill periods
		refills := int(elapsed / t.refillPeriod)
		tokensToAdd := refills * t.refillRate

		// Add tokens but don't exceed capacity
		b.tokens = min(b.tokens+tokensToAdd, t.capacity)

		// Update last refill time to account for complete periods only
		b.lastRefill = b.lastRefill.Add(time.Duration(refills) * t.refillPeriod)
	}

	// Check if request is allowed
	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

// allowRedis uses Redis for distributed rate limiting
func (t *TokenBucketLimiter) allowRedis(key string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	now := time.Now().Unix()

	// Use Redis Lua script for atomic operations
	script := `
		local key = KEYS[1]
		local capacity = tonumber(ARGV[1])
		local refill_rate = tonumber(ARGV[2])
		local refill_period = tonumber(ARGV[3])
		local now = tonumber(ARGV[4])
		
		local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
		local tokens = tonumber(bucket[1]) or capacity
		local last_refill = tonumber(bucket[2]) or now
		
		-- Refill tokens
		local elapsed = now - last_refill
		local refills = math.floor(elapsed / refill_period)
		if refills > 0 then
			tokens = math.min(tokens + (refills * refill_rate), capacity)
			last_refill = last_refill + (refills * refill_period)
		end
		
		-- Check if request is allowed
		if tokens > 0 then
			tokens = tokens - 1
			redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
			redis.call('EXPIRE', key, refill_period * 2)
			return 1
		else
			redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
			redis.call('EXPIRE', key, refill_period * 2)
			return 0
		end
	`

	result, err := t.redis.Eval(ctx, script, []string{key},
		t.capacity, t.refillRate, int(t.refillPeriod.Seconds()), now).Result()
	if err != nil {
		return false, err
	}

	return result.(int64) == 1, nil
}

// cleanupBuckets periodically removes old buckets from memory
func (t *TokenBucketLimiter) cleanupBuckets() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			var toDelete []string

			// Find buckets that haven't been accessed recently
			t.buckets.Range(func(key, value interface{}) bool {
				b := value.(*bucket)
				b.mu.Lock()
				// Remove buckets not accessed in the last hour
				if now.Sub(b.lastRefill) > time.Hour {
					toDelete = append(toDelete, key.(string))
				}
				b.mu.Unlock()
				return true
			})

			// Delete old buckets
			for _, key := range toDelete {
				t.buckets.Delete(key)
			}
		case <-t.stopCh:
			return
		}
	}
}

// Stop stops the cleanup goroutine
func (t *TokenBucketLimiter) Stop() {
	select {
	case <-t.stopCh:
		// Already stopped
	default:
		close(t.stopCh)
	}
}

// SlidingWindowLimiter implements sliding window algorithm
type SlidingWindowLimiter struct {
	limit    int
	window   time.Duration
	redis    *redis.Client
	useRedis bool
	windows  sync.Map // for local storage
}

// NewSlidingWindowLimiter creates a new sliding window rate limiter
func NewSlidingWindowLimiter(limit int, window time.Duration, redisClient *redis.Client) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		limit:    limit,
		window:   window,
		redis:    redisClient,
		useRedis: redisClient != nil,
	}
}

// Allow checks if request is allowed
func (s *SlidingWindowLimiter) Allow(key string) (bool, error) {
	if s.useRedis {
		return s.allowRedis(key)
	}
	return s.allowLocal(key), nil
}

// windowData holds request timestamps
type windowData struct {
	timestamps []time.Time
	mu         sync.Mutex
}

// allowLocal uses in-memory storage
func (s *SlidingWindowLimiter) allowLocal(key string) bool {
	now := time.Now()
	windowStart := now.Add(-s.window)

	// Get or create window data
	val, _ := s.windows.LoadOrStore(key, &windowData{
		timestamps: []time.Time{},
	})

	wd := val.(*windowData)
	wd.mu.Lock()
	defer wd.mu.Unlock()

	// Remove old timestamps outside the window
	var validTimestamps []time.Time
	for _, ts := range wd.timestamps {
		if ts.After(windowStart) {
			validTimestamps = append(validTimestamps, ts)
		}
	}

	// Check if under limit
	if len(validTimestamps) < s.limit {
		validTimestamps = append(validTimestamps, now)
		wd.timestamps = validTimestamps
		return true
	}

	wd.timestamps = validTimestamps
	return false
}

// allowRedis uses Redis for distributed rate limiting
func (s *SlidingWindowLimiter) allowRedis(key string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	now := time.Now()
	windowStart := now.Add(-s.window).UnixMilli()
	nowMs := now.UnixMilli()

	// Remove old entries and count current ones
	pipe := s.redis.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	pipe.ZCard(ctx, key)
	results, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	count := results[1].(*redis.IntCmd).Val()
	if count < int64(s.limit) {
		// Add current request
		s.redis.ZAdd(ctx, key, redis.Z{
			Score:  float64(nowMs),
			Member: nowMs,
		})
		s.redis.Expire(ctx, key, s.window)
		return true, nil
	}

	return false, nil
}

// RateLimitMiddleware creates rate limiting middleware
func RateLimitMiddleware(limiter RateLimiter, keyFunc func(*http.Request) string, log logger.Logger) router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			key := keyFunc(r)

			allowed, err := limiter.Allow(key)
			if err != nil {
				log.Error("Rate limiter error", "error", err)
				// On error, allow the request but log it
				return next(w, r)
			}

			if !allowed {
				w.Header().Set("Retry-After", "60")
				return router.ErrorResponse(w, http.StatusTooManyRequests, "Rate limit exceeded")
			}

			return next(w, r)
		}
	}
}

// IPKeyFunc returns client IP as rate limit key
func IPKeyFunc(r *http.Request) string {
	// Try to get real IP from headers
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}
	}
	return fmt.Sprintf("rate_limit:ip:%s", ip)
}

// UserKeyFunc returns user ID as rate limit key
func UserKeyFunc(r *http.Request) string {
	userID := router.GetUserID(r)
	if userID == "" {
		return IPKeyFunc(r) // Fall back to IP if no user
	}
	return fmt.Sprintf("rate_limit:user:%s", userID)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

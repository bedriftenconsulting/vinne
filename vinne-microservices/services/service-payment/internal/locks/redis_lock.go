package locks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("payment-service/locks")

var (
	// ErrLockNotAcquired is returned when lock acquisition fails
	ErrLockNotAcquired = errors.New("lock not acquired")

	// ErrLockNotHeld is returned when attempting to unlock a lock that is not held
	ErrLockNotHeld = errors.New("lock not held")
)

// RedisLock implements distributed locking using Redis
type RedisLock struct {
	client *redis.Client
	key    string
	token  string
	ttl    time.Duration
}

// DistributedLocker defines the interface for distributed locks
type DistributedLocker interface {
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
	Extend(ctx context.Context, duration time.Duration) error
	IsHeld(ctx context.Context) (bool, error)
}

// LockManager manages distributed locks
type LockManager struct {
	client *redis.Client
}

// NewLockManager creates a new lock manager
func NewLockManager(client *redis.Client) *LockManager {
	return &LockManager{
		client: client,
	}
}

// NewLock creates a new distributed lock
func (lm *LockManager) NewLock(key string, ttl time.Duration) (*RedisLock, error) {
	if key == "" {
		return nil, fmt.Errorf("lock key cannot be empty")
	}

	if ttl <= 0 {
		return nil, fmt.Errorf("lock TTL must be positive")
	}

	// Generate unique token for this lock instance
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate lock token: %w", err)
	}

	return &RedisLock{
		client: lm.client,
		key:    fmt.Sprintf("payment:lock:%s", key),
		token:  token,
		ttl:    ttl,
	}, nil
}

// Lock acquires the distributed lock
func (l *RedisLock) Lock(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "redis_lock.lock",
		trace.WithAttributes(
			attribute.String("key", l.key),
			attribute.Int64("ttl_seconds", int64(l.ttl.Seconds())),
		))
	defer span.End()

	// Use Redis SET with NX (only set if not exists) and EX (expiration)
	success, err := l.client.SetNX(ctx, l.key, l.token, l.ttl).Result()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !success {
		span.SetAttributes(attribute.Bool("acquired", false))
		return ErrLockNotAcquired
	}

	span.SetAttributes(attribute.Bool("acquired", true))
	return nil
}

// Unlock releases the distributed lock
func (l *RedisLock) Unlock(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "redis_lock.unlock",
		trace.WithAttributes(
			attribute.String("key", l.key),
		))
	defer span.End()

	// Lua script to ensure we only delete the lock if we own it (token matches)
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := l.client.Eval(ctx, script, []string{l.key}, l.token).Int()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to release lock: %w", err)
	}

	if result == 0 {
		span.SetAttributes(attribute.Bool("released", false))
		return ErrLockNotHeld
	}

	span.SetAttributes(attribute.Bool("released", true))
	return nil
}

// Extend extends the TTL of the lock
func (l *RedisLock) Extend(ctx context.Context, duration time.Duration) error {
	ctx, span := tracer.Start(ctx, "redis_lock.extend",
		trace.WithAttributes(
			attribute.String("key", l.key),
			attribute.Int64("extend_seconds", int64(duration.Seconds())),
		))
	defer span.End()

	// Lua script to extend TTL only if we own the lock (token matches)
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	result, err := l.client.Eval(ctx, script, []string{l.key}, l.token, int(duration.Seconds())).Int()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to extend lock: %w", err)
	}

	if result == 0 {
		span.SetAttributes(attribute.Bool("extended", false))
		return ErrLockNotHeld
	}

	span.SetAttributes(attribute.Bool("extended", true))
	l.ttl = duration // Update local TTL
	return nil
}

// IsHeld checks if the lock is currently held by this instance
func (l *RedisLock) IsHeld(ctx context.Context) (bool, error) {
	ctx, span := tracer.Start(ctx, "redis_lock.is_held",
		trace.WithAttributes(
			attribute.String("key", l.key),
		))
	defer span.End()

	value, err := l.client.Get(ctx, l.key).Result()
	if err != nil {
		if err == redis.Nil {
			// Key doesn't exist - lock not held
			span.SetAttributes(attribute.Bool("held", false))
			return false, nil
		}
		span.RecordError(err)
		return false, fmt.Errorf("failed to check lock status: %w", err)
	}

	held := value == l.token
	span.SetAttributes(attribute.Bool("held", held))
	return held, nil
}

// LockWithRetry attempts to acquire a lock with retries
func (lm *LockManager) LockWithRetry(ctx context.Context, key string, ttl time.Duration, maxRetries int, retryDelay time.Duration) (*RedisLock, error) {
	ctx, span := tracer.Start(ctx, "lock_manager.lock_with_retry",
		trace.WithAttributes(
			attribute.String("key", key),
			attribute.Int("max_retries", maxRetries),
		))
	defer span.End()

	lock, err := lm.NewLock(key, ttl)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			span.AddEvent("retrying lock acquisition",
				trace.WithAttributes(attribute.Int("attempt", attempt)))

			// Wait before retrying
			select {
			case <-ctx.Done():
				span.RecordError(ctx.Err())
				return nil, ctx.Err()
			case <-time.After(retryDelay):
			}
		}

		err := lock.Lock(ctx)
		if err == nil {
			span.SetAttributes(
				attribute.Bool("acquired", true),
				attribute.Int("attempts", attempt+1),
			)
			return lock, nil
		}

		if err != ErrLockNotAcquired {
			// Non-retry-able error
			span.RecordError(err)
			return nil, err
		}

		// Last attempt failed
		if attempt == maxRetries {
			span.SetAttributes(attribute.Bool("acquired", false))
			return nil, ErrLockNotAcquired
		}
	}

	return nil, ErrLockNotAcquired
}

// WithLock executes a function while holding a distributed lock
func (lm *LockManager) WithLock(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) error) error {
	ctx, span := tracer.Start(ctx, "lock_manager.with_lock",
		trace.WithAttributes(
			attribute.String("key", key),
		))
	defer span.End()

	// Acquire lock
	lock, err := lm.NewLock(key, ttl)
	if err != nil {
		span.RecordError(err)
		return err
	}

	if err := lock.Lock(ctx); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	span.SetAttributes(attribute.Bool("lock_acquired", true))

	// Ensure lock is released
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if unlockErr := lock.Unlock(unlockCtx); unlockErr != nil {
			span.RecordError(unlockErr)
			// Log error but don't override the original error
		}
	}()

	// Execute function
	return fn(ctx)
}

// WithLockRetry executes a function while holding a distributed lock with retries
func (lm *LockManager) WithLockRetry(ctx context.Context, key string, ttl time.Duration, maxRetries int, retryDelay time.Duration, fn func(context.Context) error) error {
	ctx, span := tracer.Start(ctx, "lock_manager.with_lock_retry",
		trace.WithAttributes(
			attribute.String("key", key),
			attribute.Int("max_retries", maxRetries),
		))
	defer span.End()

	// Acquire lock with retry
	lock, err := lm.LockWithRetry(ctx, key, ttl, maxRetries, retryDelay)
	if err != nil {
		span.RecordError(err)
		return err
	}

	span.SetAttributes(attribute.Bool("lock_acquired", true))

	// Ensure lock is released
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if unlockErr := lock.Unlock(unlockCtx); unlockErr != nil {
			span.RecordError(unlockErr)
			// Log error but don't override the original error
		}
	}()

	// Execute function
	return fn(ctx)
}

// generateToken generates a random token for lock ownership
func generateToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

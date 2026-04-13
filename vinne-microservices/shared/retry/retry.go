package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// Config holds retry configuration
type Config struct {
	MaxAttempts     int                          // Maximum number of retry attempts
	InitialInterval time.Duration                // Initial retry interval
	MaxInterval     time.Duration                // Maximum retry interval
	Multiplier      float64                      // Exponential backoff multiplier
	Jitter          bool                         // Add jitter to retry intervals
	RetryIf         func(error) bool             // Function to determine if error is retryable
	OnRetry         func(attempt int, err error) // Optional callback on each retry
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() Config {
	return Config{
		MaxAttempts:     3,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
		Jitter:          true,
		RetryIf:         IsRetryable,
	}
}

// Retrier provides retry functionality with exponential backoff
type Retrier struct {
	config Config
}

// New creates a new retrier with the given configuration
func New(config Config) *Retrier {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.InitialInterval <= 0 {
		config.InitialInterval = 100 * time.Millisecond
	}
	if config.MaxInterval <= 0 {
		config.MaxInterval = 30 * time.Second
	}
	if config.Multiplier <= 1 {
		config.Multiplier = 2.0
	}
	if config.RetryIf == nil {
		config.RetryIf = IsRetryable
	}

	return &Retrier{
		config: config,
	}
}

// Execute runs the given function with retry logic
func (r *Retrier) Execute(ctx context.Context, fn func(context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt < r.config.MaxAttempts; attempt++ {
		// Check context before attempting
		if err := ctx.Err(); err != nil {
			return err
		}

		// Execute the function
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !r.config.RetryIf(err) {
			return err
		}

		// Don't retry if this was the last attempt
		if attempt == r.config.MaxAttempts-1 {
			break
		}

		// Call retry callback if provided
		if r.config.OnRetry != nil {
			r.config.OnRetry(attempt+1, err)
		}

		// Calculate wait time with exponential backoff
		waitTime := r.calculateWaitTime(attempt)

		// Wait with context cancellation support
		select {
		case <-time.After(waitTime):
			// Continue to next retry
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}

// calculateWaitTime calculates the wait time for the given attempt
func (r *Retrier) calculateWaitTime(attempt int) time.Duration {
	// Calculate exponential backoff
	interval := float64(r.config.InitialInterval) * math.Pow(r.config.Multiplier, float64(attempt))

	// Cap at max interval
	if interval > float64(r.config.MaxInterval) {
		interval = float64(r.config.MaxInterval)
	}

	// Add jitter if configured
	if r.config.Jitter {
		// Add up to 20% jitter
		jitter := rand.Float64() * 0.2 * interval
		interval = interval + jitter
	}

	return time.Duration(interval)
}

// Common error types for retry logic

var (
	// ErrNotRetryable indicates an error that should not be retried
	ErrNotRetryable = errors.New("error is not retryable")
)

// IsRetryable determines if an error should trigger a retry
// This is the default retry function
func IsRetryable(err error) bool {
	// Don't retry if error is nil
	if err == nil {
		return false
	}

	// Don't retry context errors
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for explicit non-retryable error
	if errors.Is(err, ErrNotRetryable) {
		return false
	}

	// By default, retry all other errors
	// In production, you might want to be more selective
	return true
}

// WithExponentialBackoff is a helper function that executes a function with retry and exponential backoff
func WithExponentialBackoff(ctx context.Context, fn func(context.Context) error) error {
	retrier := New(DefaultConfig())
	return retrier.Execute(ctx, fn)
}

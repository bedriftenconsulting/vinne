package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries      int
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
	Jitter          bool
	RetryableStatus []int
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:      3,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		Multiplier:      2.0,
		Jitter:          true,
		RetryableStatus: []int{
			http.StatusRequestTimeout,
			http.StatusTooManyRequests,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
		},
	}
}

// Retryer handles request retries
type Retryer struct {
	config *RetryConfig
}

// NewRetryer creates a new retryer
func NewRetryer(config *RetryConfig) *Retryer {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &Retryer{
		config: config,
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() (*http.Response, error)

// Execute executes a function with retry logic
func (r *Retryer) Execute(ctx context.Context, fn RetryableFunc) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Execute the function
		resp, err := fn()

		// Check if we should retry
		if !r.shouldRetry(resp, err, attempt) {
			return resp, err
		}

		lastErr = err

		// Calculate backoff duration
		backoff := r.calculateBackoff(attempt)

		// Wait with context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			// Continue to next retry
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// ExecuteWithCallback executes with retry and calls callback on each attempt
func (r *Retryer) ExecuteWithCallback(
	ctx context.Context,
	fn RetryableFunc,
	onRetry func(attempt int, err error, nextDelay time.Duration),
) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Execute the function
		resp, err := fn()

		// Check if we should retry
		if !r.shouldRetry(resp, err, attempt) {
			return resp, err
		}

		lastErr = err

		// Calculate backoff duration
		backoff := r.calculateBackoff(attempt)

		// Call callback if provided
		if onRetry != nil && attempt < r.config.MaxRetries {
			onRetry(attempt+1, err, backoff)
		}

		// Wait with context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			// Continue to next retry
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// shouldRetry determines if a request should be retried
func (r *Retryer) shouldRetry(resp *http.Response, err error, attempt int) bool {
	// Don't retry if we've exceeded max retries
	if attempt >= r.config.MaxRetries {
		return false
	}

	// Retry on network errors
	if err != nil {
		return true
	}

	// Check if status code is retryable
	if resp != nil {
		for _, status := range r.config.RetryableStatus {
			if resp.StatusCode == status {
				return true
			}
		}
	}

	return false
}

// calculateBackoff calculates the backoff duration for a given attempt
func (r *Retryer) calculateBackoff(attempt int) time.Duration {
	// Calculate exponential backoff
	backoff := float64(r.config.InitialInterval) * math.Pow(r.config.Multiplier, float64(attempt))

	// Cap at max interval
	if backoff > float64(r.config.MaxInterval) {
		backoff = float64(r.config.MaxInterval)
	}

	// Add jitter if enabled
	if r.config.Jitter {
		jitter := rand.Float64() * 0.3 * backoff // 0-30% jitter
		backoff = backoff + jitter
	}

	return time.Duration(backoff)
}

// CircuitBreaker implements a circuit breaker pattern
type CircuitBreaker struct {
	name             string
	maxFailures      int
	resetTimeout     time.Duration
	halfOpenMaxCalls int

	mu              sync.RWMutex
	state           State
	failures        int
	lastFailureTime time.Time
	successCount    int
}

// State represents circuit breaker state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// String returns string representation of state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		maxFailures:      maxFailures,
		resetTimeout:     resetTimeout,
		halfOpenMaxCalls: 3,
		state:            StateClosed,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.canExecute() {
		return fmt.Errorf("circuit breaker is open for %s", cb.name)
	}

	err := fn()
	cb.recordResult(err)
	return err
}

// canExecute checks if request can be executed
func (cb *CircuitBreaker) canExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.successCount = 0
			return true
		}
		return false
	case StateHalfOpen:
		// Allow limited requests in half-open state
		return cb.successCount < cb.halfOpenMaxCalls
	default:
		return false
	}
}

// recordResult records the result of an execution
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		if err != nil {
			cb.failures++
			cb.lastFailureTime = time.Now()
			if cb.failures >= cb.maxFailures {
				cb.state = StateOpen
			}
		} else {
			cb.failures = 0
		}
	case StateHalfOpen:
		if err != nil {
			cb.state = StateOpen
			cb.lastFailureTime = time.Now()
		} else {
			cb.successCount++
			if cb.successCount >= cb.halfOpenMaxCalls {
				cb.state = StateClosed
				cb.failures = 0
			}
		}
	}
}

// GetState returns current circuit breaker state
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failures = 0
	cb.successCount = 0
}

// RetryWithCircuitBreaker combines retry logic with circuit breaker
type RetryWithCircuitBreaker struct {
	retryer        *Retryer
	circuitBreaker *CircuitBreaker
}

// NewRetryWithCircuitBreaker creates a new retry with circuit breaker
func NewRetryWithCircuitBreaker(retryer *Retryer, cb *CircuitBreaker) *RetryWithCircuitBreaker {
	return &RetryWithCircuitBreaker{
		retryer:        retryer,
		circuitBreaker: cb,
	}
}

// Execute executes with both retry and circuit breaker
func (rcb *RetryWithCircuitBreaker) Execute(ctx context.Context, fn RetryableFunc) (*http.Response, error) {
	var resp *http.Response
	var err error

	cbErr := rcb.circuitBreaker.Execute(func() error {
		resp, err = rcb.retryer.Execute(ctx, fn)
		return err
	})

	if cbErr != nil {
		return nil, cbErr
	}

	return resp, err
}

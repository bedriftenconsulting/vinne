package providers

import (
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	StateClosed   CircuitState = iota // Normal operation
	StateOpen                         // Circuit is open, provider is down
	StateHalfOpen                     // Testing if provider has recovered
)

// CircuitBreaker implements the circuit breaker pattern for provider failures
type CircuitBreaker struct {
	maxFailures      int           // Number of failures before opening circuit
	resetTimeout     time.Duration // Time to wait before transitioning from Open to HalfOpen
	halfOpenMaxTries int           // Max requests to try in HalfOpen state before closing

	mu               sync.RWMutex
	state            CircuitState
	failures         int
	lastFailureTime  time.Time
	lastStateChange  time.Time
	halfOpenAttempts int
}

// CircuitBreakerConfig holds configuration for circuit breaker
type CircuitBreakerConfig struct {
	MaxFailures      int           // Default: 5
	ResetTimeout     time.Duration // Default: 30s
	HalfOpenMaxTries int           // Default: 3
}

// NewCircuitBreaker creates a new circuit breaker with the given config
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.MaxFailures == 0 {
		config.MaxFailures = 5
	}
	if config.ResetTimeout == 0 {
		config.ResetTimeout = 30 * time.Second
	}
	if config.HalfOpenMaxTries == 0 {
		config.HalfOpenMaxTries = 3
	}

	return &CircuitBreaker{
		maxFailures:      config.MaxFailures,
		resetTimeout:     config.ResetTimeout,
		halfOpenMaxTries: config.HalfOpenMaxTries,
		state:            StateClosed,
		lastStateChange:  time.Now(),
	}
}

// Allow checks if a request should be allowed through the circuit breaker
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		// Normal operation, allow request
		return nil

	case StateOpen:
		// Check if enough time has passed to try half-open
		if now.Sub(cb.lastFailureTime) >= cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.halfOpenAttempts = 0
			cb.lastStateChange = now
			return nil
		}
		// Circuit is open, reject request
		return &CircuitBreakerError{
			State:          "open",
			Message:        "circuit breaker is open, provider is unavailable",
			NextRetryAfter: cb.lastFailureTime.Add(cb.resetTimeout),
			FailureCount:   cb.failures,
		}

	case StateHalfOpen:
		// Allow limited number of requests to test recovery
		if cb.halfOpenAttempts < cb.halfOpenMaxTries {
			cb.halfOpenAttempts++
			return nil
		}
		// Max attempts reached in half-open, back to open
		cb.state = StateOpen
		cb.lastFailureTime = now
		cb.lastStateChange = now
		return &CircuitBreakerError{
			State:          "half-open-exhausted",
			Message:        "circuit breaker half-open attempts exhausted",
			NextRetryAfter: now.Add(cb.resetTimeout),
			FailureCount:   cb.failures,
		}
	}

	return nil
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		// Reset failure counter on success in closed state
		cb.failures = 0

	case StateHalfOpen:
		// Success in half-open state means provider has recovered
		cb.state = StateClosed
		cb.failures = 0
		cb.halfOpenAttempts = 0
		cb.lastStateChange = now
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	cb.failures++
	cb.lastFailureTime = now

	switch cb.state {
	case StateClosed:
		// Open circuit if failures exceed threshold
		if cb.failures >= cb.maxFailures {
			cb.state = StateOpen
			cb.lastStateChange = now
		}

	case StateHalfOpen:
		// Any failure in half-open state transitions back to open
		cb.state = StateOpen
		cb.halfOpenAttempts = 0
		cb.lastStateChange = now
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		State:            cb.state,
		Failures:         cb.failures,
		LastFailureTime:  cb.lastFailureTime,
		LastStateChange:  cb.lastStateChange,
		HalfOpenAttempts: cb.halfOpenAttempts,
	}
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.halfOpenAttempts = 0
	cb.lastStateChange = time.Now()
}

// CircuitBreakerStats holds statistics about circuit breaker state
type CircuitBreakerStats struct {
	State            CircuitState
	Failures         int
	LastFailureTime  time.Time
	LastStateChange  time.Time
	HalfOpenAttempts int
}

// CircuitBreakerError represents an error when circuit breaker is open
type CircuitBreakerError struct {
	State          string
	Message        string
	NextRetryAfter time.Time
	FailureCount   int
}

func (e *CircuitBreakerError) Error() string {
	return fmt.Sprintf("%s (failures: %d, retry after: %s)",
		e.Message, e.FailureCount, e.NextRetryAfter.Format(time.RFC3339))
}

// IsCircuitBreakerError checks if an error is a circuit breaker error
func IsCircuitBreakerError(err error) bool {
	_, ok := err.(*CircuitBreakerError)
	return ok
}

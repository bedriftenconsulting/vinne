package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the circuit breaker state
type CircuitState string

const (
	CircuitStateClosed   CircuitState = "CLOSED"    // Normal operation
	CircuitStateOpen     CircuitState = "OPEN"      // Failing - reject requests
	CircuitStateHalfOpen CircuitState = "HALF_OPEN" // Testing recovery
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name             string
	maxFailures      int
	timeout          time.Duration
	halfOpenRequests int
	state            CircuitState
	failures         int
	successes        int
	lastFailureTime  time.Time
	nextRetryTime    time.Time
	mu               sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		maxFailures:      maxFailures,
		timeout:          timeout,
		halfOpenRequests: 3, // Number of test requests in half-open state
		state:            CircuitStateClosed,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	// Check if circuit is open
	if cb.isOpen() {
		return fmt.Errorf("circuit breaker %s is OPEN", cb.name)
	}

	// Execute function
	err := fn(ctx)

	// Record result
	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

// isOpen checks if the circuit is open
func (cb *CircuitBreaker) isOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == CircuitStateClosed {
		return false
	}

	if cb.state == CircuitStateOpen {
		// Check if timeout has elapsed
		if time.Now().After(cb.nextRetryTime) {
			// Transition to half-open
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = CircuitStateHalfOpen
			cb.successes = 0
			cb.mu.Unlock()
			cb.mu.RLock()
			return false
		}
		return true
	}

	// Half-open state - allow limited requests
	return false
}

// recordFailure records a failure
func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailureTime = time.Now()

	if cb.state == CircuitStateHalfOpen {
		// Failed while testing - back to open
		cb.state = CircuitStateOpen
		cb.nextRetryTime = time.Now().Add(cb.timeout)
		cb.failures = 0
		fmt.Printf("Circuit breaker %s re-opened after half-open failure\n", cb.name)
		return
	}

	if cb.failures >= cb.maxFailures {
		// Too many failures - open circuit
		cb.state = CircuitStateOpen
		cb.nextRetryTime = time.Now().Add(cb.timeout)
		fmt.Printf("Circuit breaker %s opened after %d failures\n", cb.name, cb.failures)
		// TODO: Send alert to monitoring system
	}
}

// recordSuccess records a success
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitStateHalfOpen:
		cb.successes++
		if cb.successes >= cb.halfOpenRequests {
			// Enough successes - close circuit
			cb.state = CircuitStateClosed
			cb.failures = 0
			cb.successes = 0
			fmt.Printf("Circuit breaker %s closed after recovery\n", cb.name)
		}
	case CircuitStateClosed:
		// Reset failure counter on success
		cb.failures = 0
	}
}

// GetState returns the current circuit state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns current circuit breaker statistics
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		Name:            cb.name,
		State:           string(cb.state),
		Failures:        cb.failures,
		Successes:       cb.successes,
		LastFailureTime: cb.lastFailureTime,
		NextRetryTime:   cb.nextRetryTime,
	}
}

// CircuitBreakerStats holds circuit breaker statistics
type CircuitBreakerStats struct {
	Name            string
	State           string
	Failures        int
	Successes       int
	LastFailureTime time.Time
	NextRetryTime   time.Time
}

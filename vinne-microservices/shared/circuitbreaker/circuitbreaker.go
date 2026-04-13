package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

var (
	ErrOpenCircuit     = errors.New("circuit breaker is open")
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// Config holds circuit breaker configuration
type Config struct {
	MaxFailures      int                  // Number of failures before opening circuit
	ResetTimeout     time.Duration        // Time to wait before attempting reset
	HalfOpenRequests int                  // Max requests allowed in half-open state
	OnStateChange    func(from, to State) // Optional callback for state changes
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() Config {
	return Config{
		MaxFailures:      5,
		ResetTimeout:     30 * time.Second,
		HalfOpenRequests: 3,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config Config

	mu              sync.RWMutex
	state           State
	failures        int
	lastFailureTime time.Time
	halfOpenSuccess int
	halfOpenFailure int
}

// New creates a new circuit breaker with the given configuration
func New(config Config) *CircuitBreaker {
	if config.MaxFailures <= 0 {
		config.MaxFailures = 5
	}
	if config.ResetTimeout <= 0 {
		config.ResetTimeout = 30 * time.Second
	}
	if config.HalfOpenRequests <= 0 {
		config.HalfOpenRequests = 3
	}

	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// Execute runs the given function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	if err := cb.canExecute(); err != nil {
		return err
	}

	err := fn(ctx)
	cb.recordResult(err)
	return err
}

// canExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) canExecute() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return nil

	case StateOpen:
		if time.Since(cb.lastFailureTime) > cb.config.ResetTimeout {
			cb.transitionTo(StateHalfOpen)
			return nil
		}
		return ErrOpenCircuit

	case StateHalfOpen:
		total := cb.halfOpenSuccess + cb.halfOpenFailure
		if total >= cb.config.HalfOpenRequests {
			return ErrTooManyRequests
		}
		return nil

	default:
		return nil
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
			if cb.failures >= cb.config.MaxFailures {
				cb.transitionTo(StateOpen)
			}
		} else {
			cb.failures = 0
		}

	case StateHalfOpen:
		if err != nil {
			cb.halfOpenFailure++
			cb.transitionTo(StateOpen)
		} else {
			cb.halfOpenSuccess++
			if cb.halfOpenSuccess >= cb.config.HalfOpenRequests {
				cb.transitionTo(StateClosed)
			}
		}
	}
}

// transitionTo changes the circuit breaker state
func (cb *CircuitBreaker) transitionTo(newState State) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState

	// Reset counters when transitioning
	switch newState {
	case StateClosed:
		cb.failures = 0
		cb.halfOpenSuccess = 0
		cb.halfOpenFailure = 0
	case StateHalfOpen:
		cb.halfOpenSuccess = 0
		cb.halfOpenFailure = 0
	case StateOpen:
		cb.lastFailureTime = time.Now()
	}

	// Call state change callback if provided
	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(oldState, newState)
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionTo(StateClosed)
}

// String returns the string representation of a state
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

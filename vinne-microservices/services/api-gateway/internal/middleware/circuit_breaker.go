package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name            string
	maxFailures     int
	resetTimeout    time.Duration
	halfOpenMax     int
	state           CircuitState
	failures        int
	lastFailureTime time.Time
	halfOpenCount   int
	mu              sync.RWMutex
	log             logger.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration, log logger.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		halfOpenMax:  maxFailures / 2,
		state:        StateClosed,
		log:          log,
	}
}

// Allow checks if request is allowed through the circuit breaker
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if we should transition to half-open
		if now.Sub(cb.lastFailureTime) > cb.resetTimeout {
			cb.log.Info("Circuit breaker transitioning to half-open", "name", cb.name)
			cb.state = StateHalfOpen
			cb.halfOpenCount = 0
			cb.failures = cb.maxFailures / 2 // Start with partial failures to be cautious
			return true
		}
		return false

	case StateHalfOpen:
		// Don't increment here - let RecordSuccess/RecordFailure handle counting
		return true

	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateHalfOpen:
		// In half-open state, decrement failures on success
		if cb.failures > 0 {
			cb.failures--
		}

		// If we've had enough successful requests, close the circuit
		cb.halfOpenCount++
		if cb.halfOpenCount >= cb.halfOpenMax {
			cb.log.Info("Circuit breaker closing", "name", cb.name)
			cb.state = StateClosed
			cb.failures = 0
			cb.halfOpenCount = 0
		}

	case StateClosed:
		// Reset consecutive failures on success
		if cb.failures > 0 {
			cb.failures = 0
		}
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.maxFailures {
			cb.log.Warn("Circuit breaker opening", "name", cb.name, "failures", cb.failures)
			cb.state = StateOpen
		}

	case StateHalfOpen:
		cb.log.Warn("Circuit breaker reopening from half-open", "name", cb.name)
		cb.state = StateOpen
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
	log      logger.Logger
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(log logger.Logger) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
		log:      log,
	}
}

// GetBreaker gets or creates a circuit breaker for a service
func (m *CircuitBreakerManager) GetBreaker(service string) *CircuitBreaker {
	m.mu.RLock()
	breaker, exists := m.breakers[service]
	m.mu.RUnlock()

	if exists {
		return breaker
	}

	// Create new breaker if it doesn't exist
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if breaker, exists := m.breakers[service]; exists {
		return breaker
	}

	breaker = NewCircuitBreaker(
		service,
		5,              // max failures
		30*time.Second, // reset timeout
		m.log,
	)
	m.breakers[service] = breaker
	return breaker
}

// CircuitBreakerMiddleware creates circuit breaker middleware
func CircuitBreakerMiddleware(manager *CircuitBreakerManager, serviceFunc func(*http.Request) string) router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			service := serviceFunc(r)
			breaker := manager.GetBreaker(service)

			if !breaker.Allow() {
				return router.ErrorResponse(w, http.StatusServiceUnavailable,
					fmt.Sprintf("Service %s is temporarily unavailable", service))
			}

			// Execute the request
			err := next(w, r)

			// Record result
			if err != nil {
				breaker.RecordFailure()
			} else {
				breaker.RecordSuccess()
			}

			return err
		}
	}
}

// ServiceFromPath extracts service name from request path
func ServiceFromPath(r *http.Request) string {
	// Extract service from path like /api/v1/admin/...
	parts := splitPath(r.URL.Path)
	if len(parts) >= 4 {
		return parts[3] // e.g., "admin" from /api/v1/admin/...
	}
	return "unknown"
}

// splitPath splits a path into parts
func splitPath(path string) []string {
	var parts []string
	for _, part := range strings.Split(path, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

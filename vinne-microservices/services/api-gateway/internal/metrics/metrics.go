package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	// HTTP metrics
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	RequestSize     *prometheus.HistogramVec
	ResponseSize    *prometheus.HistogramVec
	ActiveRequests  prometheus.Gauge

	// Cache metrics
	CacheHits      *prometheus.CounterVec
	CacheMisses    *prometheus.CounterVec
	CacheEvictions prometheus.Counter
	CacheSize      prometheus.Gauge

	// Circuit breaker metrics
	CircuitBreakerOpen     *prometheus.CounterVec
	CircuitBreakerClosed   *prometheus.CounterVec
	CircuitBreakerHalfOpen *prometheus.CounterVec

	// Rate limiting metrics
	RateLimitHits    *prometheus.CounterVec
	RateLimitAllowed *prometheus.CounterVec

	// Backend service metrics
	BackendRequestsTotal   *prometheus.CounterVec
	BackendRequestDuration *prometheus.HistogramVec
	BackendErrors          *prometheus.CounterVec

	// WebSocket metrics
	WebSocketConnections prometheus.Gauge
	WebSocketMessages    *prometheus.CounterVec
	WebSocketErrors      *prometheus.CounterVec

	// Business metrics
	AuthenticationAttempts  *prometheus.CounterVec
	AuthenticationSuccesses *prometheus.CounterVec
	AuthenticationFailures  *prometheus.CounterVec
}

// NewMetrics creates and registers all metrics
func NewMetrics() *Metrics {
	m := &Metrics{
		// HTTP metrics
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status", "version"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "api_gateway_request_duration_seconds",
				Help:    "HTTP request latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint", "status", "version"},
		),
		RequestSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "api_gateway_request_size_bytes",
				Help:    "HTTP request size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 6),
			},
			[]string{"method", "endpoint"},
		),
		ResponseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "api_gateway_response_size_bytes",
				Help:    "HTTP response size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 6),
			},
			[]string{"method", "endpoint"},
		),
		ActiveRequests: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "api_gateway_active_requests",
				Help: "Number of active HTTP requests",
			},
		),

		// Cache metrics
		CacheHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"endpoint"},
		),
		CacheMisses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"endpoint"},
		),
		CacheEvictions: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "api_gateway_cache_evictions_total",
				Help: "Total number of cache evictions",
			},
		),
		CacheSize: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "api_gateway_cache_size_bytes",
				Help: "Current cache size in bytes",
			},
		),

		// Circuit breaker metrics
		CircuitBreakerOpen: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_circuit_breaker_open_total",
				Help: "Total number of times circuit breaker opened",
			},
			[]string{"service"},
		),
		CircuitBreakerClosed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_circuit_breaker_closed_total",
				Help: "Total number of times circuit breaker closed",
			},
			[]string{"service"},
		),
		CircuitBreakerHalfOpen: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_circuit_breaker_half_open_total",
				Help: "Total number of times circuit breaker entered half-open state",
			},
			[]string{"service"},
		),

		// Rate limiting metrics
		RateLimitHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_rate_limit_hits_total",
				Help: "Total number of rate limit hits",
			},
			[]string{"endpoint", "user"},
		),
		RateLimitAllowed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_rate_limit_allowed_total",
				Help: "Total number of requests allowed by rate limiter",
			},
			[]string{"endpoint", "user"},
		),

		// Backend service metrics
		BackendRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_backend_requests_total",
				Help: "Total number of requests to backend services",
			},
			[]string{"service", "method", "status"},
		),
		BackendRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "api_gateway_backend_request_duration_seconds",
				Help:    "Backend request latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "method"},
		),
		BackendErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_backend_errors_total",
				Help: "Total number of backend errors",
			},
			[]string{"service", "error_type"},
		),

		// WebSocket metrics
		WebSocketConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "api_gateway_websocket_connections",
				Help: "Current number of WebSocket connections",
			},
		),
		WebSocketMessages: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_websocket_messages_total",
				Help: "Total number of WebSocket messages",
			},
			[]string{"direction", "type"},
		),
		WebSocketErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_websocket_errors_total",
				Help: "Total number of WebSocket errors",
			},
			[]string{"error_type"},
		),

		// Business metrics
		AuthenticationAttempts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_auth_attempts_total",
				Help: "Total number of authentication attempts",
			},
			[]string{"method"},
		),
		AuthenticationSuccesses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_auth_successes_total",
				Help: "Total number of successful authentications",
			},
			[]string{"method"},
		),
		AuthenticationFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_gateway_auth_failures_total",
				Help: "Total number of failed authentications",
			},
			[]string{"method", "reason"},
		),
	}

	// Register all metrics
	prometheus.MustRegister(
		m.RequestsTotal,
		m.RequestDuration,
		m.RequestSize,
		m.ResponseSize,
		m.ActiveRequests,
		m.CacheHits,
		m.CacheMisses,
		m.CacheEvictions,
		m.CacheSize,
		m.CircuitBreakerOpen,
		m.CircuitBreakerClosed,
		m.CircuitBreakerHalfOpen,
		m.RateLimitHits,
		m.RateLimitAllowed,
		m.BackendRequestsTotal,
		m.BackendRequestDuration,
		m.BackendErrors,
		m.WebSocketConnections,
		m.WebSocketMessages,
		m.WebSocketErrors,
		m.AuthenticationAttempts,
		m.AuthenticationSuccesses,
		m.AuthenticationFailures,
	)

	return m
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += n
	return n, err
}

// MetricsMiddleware creates a middleware for collecting metrics
func MetricsMiddleware(metrics *Metrics) router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			start := time.Now()

			// Track active requests
			metrics.ActiveRequests.Inc()
			defer metrics.ActiveRequests.Dec()

			// Get request size
			var requestSize float64
			if r.ContentLength > 0 {
				requestSize = float64(r.ContentLength)
			}

			// Wrap response writer to capture status code and size
			rw := newResponseWriter(w)

			// Execute handler
			err := next(rw, r)

			// Calculate duration
			duration := time.Since(start).Seconds()

			// Extract labels
			endpoint := r.URL.Path
			method := r.Method
			status := strconv.Itoa(rw.statusCode)
			version := r.Header.Get("X-API-Version")
			if version == "" {
				version = "v1"
			}

			// Record metrics
			metrics.RequestsTotal.WithLabelValues(method, endpoint, status, version).Inc()
			metrics.RequestDuration.WithLabelValues(method, endpoint, status, version).Observe(duration)

			if requestSize > 0 {
				metrics.RequestSize.WithLabelValues(method, endpoint).Observe(requestSize)
			}

			if rw.written > 0 {
				metrics.ResponseSize.WithLabelValues(method, endpoint).Observe(float64(rw.written))
			}

			// Check cache header
			if cacheStatus := w.Header().Get("X-Cache"); cacheStatus != "" {
				if cacheStatus == "HIT" {
					metrics.CacheHits.WithLabelValues(endpoint).Inc()
				} else if cacheStatus == "MISS" {
					metrics.CacheMisses.WithLabelValues(endpoint).Inc()
				}
			}

			return err
		}
	}
}

// Handler returns the Prometheus metrics handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// RecordBackendRequest records metrics for a backend service request
func (m *Metrics) RecordBackendRequest(service, method string, duration time.Duration, status int) {
	m.BackendRequestsTotal.WithLabelValues(service, method, strconv.Itoa(status)).Inc()
	m.BackendRequestDuration.WithLabelValues(service, method).Observe(duration.Seconds())

	if status >= 500 {
		m.BackendErrors.WithLabelValues(service, "5xx").Inc()
	} else if status >= 400 {
		m.BackendErrors.WithLabelValues(service, "4xx").Inc()
	}
}

// RecordCircuitBreakerState records circuit breaker state changes
func (m *Metrics) RecordCircuitBreakerState(service, state string) {
	switch state {
	case "open":
		m.CircuitBreakerOpen.WithLabelValues(service).Inc()
	case "closed":
		m.CircuitBreakerClosed.WithLabelValues(service).Inc()
	case "half-open":
		m.CircuitBreakerHalfOpen.WithLabelValues(service).Inc()
	}
}

// RecordRateLimit records rate limit events
func (m *Metrics) RecordRateLimit(endpoint, user string, allowed bool) {
	if allowed {
		m.RateLimitAllowed.WithLabelValues(endpoint, user).Inc()
	} else {
		m.RateLimitHits.WithLabelValues(endpoint, user).Inc()
	}
}

// RecordAuthentication records authentication events
func (m *Metrics) RecordAuthentication(method string, success bool, reason string) {
	m.AuthenticationAttempts.WithLabelValues(method).Inc()

	if success {
		m.AuthenticationSuccesses.WithLabelValues(method).Inc()
	} else {
		m.AuthenticationFailures.WithLabelValues(method, reason).Inc()
	}
}

// RecordWebSocketConnection records WebSocket connection changes
func (m *Metrics) RecordWebSocketConnection(delta float64) {
	if delta > 0 {
		m.WebSocketConnections.Add(delta)
	} else {
		m.WebSocketConnections.Sub(-delta)
	}
}

// RecordWebSocketMessage records WebSocket messages
func (m *Metrics) RecordWebSocketMessage(direction, messageType string) {
	m.WebSocketMessages.WithLabelValues(direction, messageType).Inc()
}

// RecordWebSocketError records WebSocket errors
func (m *Metrics) RecordWebSocketError(errorType string) {
	m.WebSocketErrors.WithLabelValues(errorType).Inc()
}

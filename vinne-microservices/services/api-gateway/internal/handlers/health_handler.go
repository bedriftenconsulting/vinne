package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
	"google.golang.org/grpc/connectivity"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
	version     string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(grpcManager *grpc.ClientManager, log logger.Logger, version string) *HealthHandler {
	return &HealthHandler{
		grpcManager: grpcManager,
		log:         log,
		version:     version,
	}
}

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Connected bool   `json:"connected"`
	Latency   string `json:"latency,omitempty"`
	Error     string `json:"error,omitempty"`
}

// HealthResponse represents the overall health response
type HealthResponse struct {
	Status    string                   `json:"status"`
	Version   string                   `json:"version"`
	Services  map[string]ServiceHealth `json:"services"`
	Timestamp string                   `json:"timestamp"`
}

// Health checks the health of the API Gateway
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) error {
	// Simple health check with version
	return router.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"version": h.version,
	})
}

// HealthDetailed provides detailed health status including service connectivity
func (h *HealthHandler) HealthDetailed(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Service list to check
	services := []string{
		"admin-management",
		"agent-auth",
		"agent-management",
		"wallet",
		"terminal",
		"game",
		"payment",
	}

	// Check services concurrently
	var wg sync.WaitGroup
	healthStatuses := make(map[string]ServiceHealth)
	healthMutex := &sync.Mutex{}
	overallHealthy := true

	for _, serviceName := range services {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			health := ServiceHealth{
				Name:      name,
				Status:    "unknown",
				Connected: false,
			}

			// Try to get connection
			start := time.Now()
			conn, err := h.grpcManager.GetConnection(name)
			latency := time.Since(start)

			if err != nil {
				health.Status = "unavailable"
				health.Error = err.Error()
				overallHealthy = false
			} else if conn == nil {
				health.Status = "unavailable"
				health.Error = "no connection"
				overallHealthy = false
			} else {
				// Check connection state
				state := conn.GetState()
				health.Latency = latency.String()

				switch state {
				case connectivity.Ready:
					health.Status = "healthy"
					health.Connected = true
				case connectivity.Idle:
					// Try to connect
					conn.Connect()
					// Wait a bit and check again
					time.Sleep(100 * time.Millisecond)
					state = conn.GetState()
					if state == connectivity.Ready {
						health.Status = "healthy"
						health.Connected = true
					} else {
						health.Status = "idle"
						health.Connected = false
						overallHealthy = false
					}
				case connectivity.Connecting:
					health.Status = "connecting"
					health.Connected = false
					overallHealthy = false
				case connectivity.TransientFailure:
					health.Status = "unhealthy"
					health.Connected = false
					health.Error = "transient failure"
					overallHealthy = false
				case connectivity.Shutdown:
					health.Status = "shutdown"
					health.Connected = false
					overallHealthy = false
				default:
					health.Status = "unknown"
					health.Connected = false
					overallHealthy = false
				}
			}

			healthMutex.Lock()
			healthStatuses[name] = health
			healthMutex.Unlock()
		}(serviceName)
	}

	// Wait for all checks to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All checks completed
	case <-ctx.Done():
		// Timeout reached
		h.log.Warn("Health check timeout reached")
		overallHealthy = false
	}

	// Determine overall status
	overallStatus := "healthy"
	if !overallHealthy {
		overallStatus = "degraded"
	}

	response := HealthResponse{
		Status:    overallStatus,
		Version:   h.version,
		Services:  healthStatuses,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Set appropriate status code
	statusCode := http.StatusOK
	if !overallHealthy {
		statusCode = http.StatusServiceUnavailable
	}

	return router.WriteJSON(w, statusCode, response)
}

// CheckServiceConnectivity checks if a specific service is reachable
func (h *HealthHandler) CheckServiceConnectivity(serviceName string) (bool, error) {
	conn, err := h.grpcManager.GetConnection(serviceName)
	if err != nil {
		return false, err
	}

	if conn == nil {
		return false, nil
	}

	state := conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle, nil
}

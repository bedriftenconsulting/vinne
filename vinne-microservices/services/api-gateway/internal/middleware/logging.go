package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
)

// loggingResponseWriter wraps http.ResponseWriter to capture status code
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *loggingResponseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

func (rw *loggingResponseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// LoggingMiddleware creates logging middleware
func LoggingMiddleware(log logger.Logger) router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &loggingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Extract request ID
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = generateRequestID()
				r.Header.Set("X-Request-ID", requestID)
			}

			// Set request ID in response header
			w.Header().Set("X-Request-ID", requestID)

			// Log request
			log.Debug("Incoming request",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)

			// Process request
			err := next(wrapped, r)

			// Calculate duration
			duration := time.Since(start)

			// Log response
			fields := []interface{}{
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
			}

			// Add user info if available
			if userID := router.GetUserID(r); userID != "" {
				fields = append(fields, "user_id", userID)
			}

			if err != nil {
				fields = append(fields, "error", err.Error())
				log.Error("Request failed", fields...)
			} else if wrapped.statusCode >= 400 {
				log.Warn("Request completed with error status", fields...)
			} else {
				log.Info("Request completed", fields...)
			}

			return err
		}
	}
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	// Use crypto/rand for secure random generation
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp if crypto/rand fails
		return time.Now().Format("20060102150405.999999999")
	}
	return time.Now().Format("20060102150405") + "-" + hex.EncodeToString(b)
}

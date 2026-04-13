package middleware

import (
	"net/http"
	"strings"

	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
)

// RequestSizeLimitMiddleware limits the size of incoming requests
func RequestSizeLimitMiddleware(maxBytes int64) router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			return next(w, r)
		}
	}
}

// SecurityHeadersMiddleware adds security headers to responses
func SecurityHeadersMiddleware() router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Add security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")

			// Add HSTS header for HTTPS connections
			if r.TLS != nil {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			return next(w, r)
		}
	}
}

// Helper functions

// ValidateOrigin validates the origin against allowed origins
func ValidateOrigin(origin string, allowedOrigins []string, isDev bool) bool {
	// Never allow wildcard in production
	if !isDev && contains(allowedOrigins, "*") {
		return false
	}

	for _, allowed := range allowedOrigins {
		if allowed == origin {
			return true
		}
		// Support wildcard subdomains
		if strings.HasPrefix(allowed, "*.") {
			domain := strings.TrimPrefix(allowed, "*.")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}
	return false
}

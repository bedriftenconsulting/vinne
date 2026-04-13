package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
)

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// NewCORSConfig creates CORS config from security config
func NewCORSConfig(securityConfig *SecurityConfig) CORSConfig {
	// Use configured values or defaults
	allowedOrigins := securityConfig.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:3000"} // Development default
	}

	allowedMethods := securityConfig.AllowedMethods
	if len(allowedMethods) == 0 {
		allowedMethods = []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		}
	}

	allowedHeaders := securityConfig.AllowedHeaders
	if len(allowedHeaders) == 0 {
		allowedHeaders = []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-Request-ID",
		}
	}

	exposedHeaders := securityConfig.ExposeHeaders
	if len(exposedHeaders) == 0 {
		exposedHeaders = []string{"X-Request-ID"}
	}

	return CORSConfig{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   allowedMethods,
		AllowedHeaders:   allowedHeaders,
		ExposedHeaders:   exposedHeaders,
		AllowCredentials: securityConfig.AllowCredentials,
		MaxAge:           86400, // 24 hours
	}
}

// SecurityConfig holds security configuration (imported from config package)
type SecurityConfig struct {
	JWTSecret        string   `json:"jwt_secret" yaml:"jwt_secret" mapstructure:"jwt_secret"`
	JWTIssuer        string   `json:"jwt_issuer" yaml:"jwt_issuer" mapstructure:"jwt_issuer"`
	AllowedOrigins   []string `json:"allowed_origins" yaml:"allowed_origins" mapstructure:"allowed_origins"`
	AllowedHeaders   []string `json:"allowed_headers" yaml:"allowed_headers" mapstructure:"allowed_headers"`
	AllowedMethods   []string `json:"allowed_methods" yaml:"allowed_methods" mapstructure:"allowed_methods"`
	ExposeHeaders    []string `json:"expose_headers" yaml:"expose_headers" mapstructure:"expose_headers"`
	AllowCredentials bool     `json:"allow_credentials" yaml:"allow_credentials" mapstructure:"allow_credentials"`
}

// CORSMiddleware creates CORS middleware with improved security
func CORSMiddleware(config CORSConfig, isDevelopment bool) router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			origin := r.Header.Get("Origin")

			// Strict validation - never allow wildcard in production
			if !isDevelopment && contains(config.AllowedOrigins, "*") {
				// Replace wildcard with empty list in production
				config.AllowedOrigins = []string{}
			}

			// Check if origin is allowed and set headers
			originAllowed := false
			if isOriginAllowed(origin, config.AllowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				originAllowed = true
			} else if isDevelopment && len(config.AllowedOrigins) == 1 && config.AllowedOrigins[0] == "*" {
				// Only allow wildcard in development
				w.Header().Set("Access-Control-Allow-Origin", "*")
				originAllowed = true
			}

			// If origin is allowed, set other CORS headers
			if originAllowed {
				if config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				if len(config.ExposedHeaders) > 0 {
					w.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposedHeaders, ", "))
				}

				// Handle preflight requests
				if r.Method == http.MethodOptions {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
					if config.MaxAge > 0 {
						w.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
					}
					w.WriteHeader(http.StatusNoContent)
					return nil
				}
			} else {
				// Origin not allowed - still process the request but without CORS headers
				// This allows the browser to see the error but blocks the response
				// The browser will show a CORS error which is the correct behavior
				if r.Method == http.MethodOptions {
					// For preflight, we must reject unauthorized origins
					return router.ErrorResponse(w, http.StatusForbidden, "Origin not allowed")
				}
			}

			// Always continue to next handler to allow proper error responses
			return next(w, r)
		}
	}
}

// isOriginAllowed checks if origin is in allowed list with strict validation
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range allowedOrigins {
		if allowed == origin {
			return true
		}
		// Support wildcard subdomains (but not full wildcard)
		if strings.HasPrefix(allowed, "*.") && allowed != "*" {
			domain := strings.TrimPrefix(allowed, "*.")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}
	return false
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

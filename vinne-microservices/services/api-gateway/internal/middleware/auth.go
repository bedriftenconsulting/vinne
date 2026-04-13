package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/jwt"
	"github.com/randco/randco-microservices/shared/common/logger"
)

// AuthMiddleware creates authentication middleware with enhanced validation
func AuthMiddleware(jwtService jwt.Service, log logger.Logger, config *AuthConfig) router.Middleware {
	if config == nil {
		config = &AuthConfig{
			MinTokenLength:          20,
			MaxTokenLength:          2048,
			RequireSecureConnection: true,
		}
	}

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Log the incoming request
			log.Debug("Auth middleware processing request",
				"path", r.URL.Path,
				"method", r.Method,
				"has_auth_header", r.Header.Get("Authorization") != "")

			// Enforce HTTPS in production
			if config.RequireSecureConnection && r.TLS == nil && !isLocalRequest(r) {
				log.Warn("Insecure connection rejected", "path", r.URL.Path, "remote", r.RemoteAddr)
				return router.ErrorResponse(w, http.StatusForbidden, "Secure connection required")
			}

			// Skip auth for public endpoints
			if isPublicEndpoint(r.URL.Path) {
				log.Debug("Skipping auth for public endpoint", "path", r.URL.Path)
				return next(w, r)
			}

			// Extract token from header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Warn("Missing authorization header",
					"path", r.URL.Path,
					"method", r.Method)
				return router.ErrorResponse(w, http.StatusUnauthorized, "Missing authorization header")
			}

			// Check Bearer prefix
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				log.Warn("Invalid authorization header format",
					"path", r.URL.Path,
					"header_parts", len(parts),
					"prefix", parts[0])
				return router.ErrorResponse(w, http.StatusUnauthorized, "Invalid authorization header format")
			}

			token := parts[1]

			// Validate token length
			if len(token) < config.MinTokenLength || len(token) > config.MaxTokenLength {
				log.Warn("Invalid token length",
					"path", r.URL.Path,
					"token_length", len(token))
				return router.ErrorResponse(w, http.StatusUnauthorized, "Invalid token format")
			}

			log.Debug("Extracted token from header",
				"path", r.URL.Path,
				"token_length", len(token),
				"token_prefix", token[:minInt(10, len(token))]+"...")

			// Validate token with additional checks
			claims, err := jwtService.ValidateAccessToken(token)
			if err != nil {
				log.Error("Token validation failed",
					"path", r.URL.Path,
					"error", err,
					"token_prefix", token[:minInt(10, len(token))]+"...")
				return router.ErrorResponse(w, http.StatusUnauthorized, "Invalid or expired token")
			}

			// Additional validation: check for required claims
			if claims.UserID == "" || claims.Email == "" {
				log.Warn("Token missing required claims",
					"path", r.URL.Path,
					"has_user_id", claims.UserID != "",
					"has_email", claims.Email != "")
				return router.ErrorResponse(w, http.StatusUnauthorized, "Invalid token claims")
			}

			log.Debug("Token validated successfully",
				"path", r.URL.Path,
				"user_id", claims.UserID,
				"email", claims.Email,
				"username", claims.Username,
				"roles", claims.Roles)

			// Add claims to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, router.ContextUserID, claims.UserID)
			ctx = context.WithValue(ctx, router.ContextEmail, claims.Email)
			ctx = context.WithValue(ctx, router.ContextUsername, claims.Username)
			ctx = context.WithValue(ctx, router.ContextRoles, claims.Roles)

			log.Debug("Request authenticated successfully",
				"path", r.URL.Path,
				"user_id", claims.UserID)

			// Continue with updated context
			return next(w, r.WithContext(ctx))
		}
	}
}

// Helper function for minInt (renamed to avoid conflict)
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// isPublicEndpoint checks if the endpoint requires authentication
func isPublicEndpoint(path string) bool {
	publicPaths := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/auth/forgot-password",
		"/api/v1/auth/reset-password",
		"/api/v1/admin/auth/login",
		"/api/v1/admin/auth/refresh",
		"/api/v1/admin/auth/register",
		"/api/v1/agent/auth/login",
		"/api/v1/agent/auth/refresh",
		"/api/v1/retailer/auth/pos-login",
		"/api/v1/retailer/auth/refresh",
		"/api/v1/public/", // All public endpoints
		"/health",
		"/metrics",
		"/api/docs",
	}

	for _, publicPath := range publicPaths {
		if strings.HasPrefix(path, publicPath) {
			return true
		}
	}

	return false
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	MinTokenLength          int
	MaxTokenLength          int
	RequireSecureConnection bool
}

// isLocalRequest checks if the request is from localhost
func isLocalRequest(r *http.Request) bool {
	return strings.HasPrefix(r.RemoteAddr, "127.0.0.1:") ||
		strings.HasPrefix(r.RemoteAddr, "[::1]:")
}

// RequireRoles creates middleware that requires specific roles
func RequireRoles(requiredRoles ...string) router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			userRoles := router.GetRoles(r)

			// Check if user has any of the required roles
			hasRole := false
			for _, required := range requiredRoles {
				for _, userRole := range userRoles {
					if userRole == required {
						hasRole = true
						break
					}
				}
				if hasRole {
					break
				}
			}

			if !hasRole {
				return router.ErrorResponse(w, http.StatusForbidden, "Insufficient permissions")
			}

			return next(w, r)
		}
	}
}

// RequirePermissions creates middleware that requires specific permissions
func RequirePermissions(requiredPerms ...string) router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Permission checking will be implemented when permission service is ready
			return next(w, r)
		}
	}
}

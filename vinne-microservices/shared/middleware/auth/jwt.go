package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/randco/randco-microservices/shared/common/errors"
)

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret               string
	AccessTokenExpiry    time.Duration
	RefreshTokenExpiry   time.Duration
	Issuer              string
}

// Claims represents JWT claims
type Claims struct {
	UserID    string   `json:"user_id"`
	Email     string   `json:"email"`
	Username  string   `json:"username"`
	Roles     []string `json:"roles"`
	TokenType string   `json:"token_type"` // access or refresh
	jwt.RegisteredClaims
}

// JWTManager handles JWT operations
type JWTManager struct {
	config JWTConfig
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(config JWTConfig) *JWTManager {
	return &JWTManager{
		config: config,
	}
}

// GenerateTokenPair generates both access and refresh tokens
func (m *JWTManager) GenerateTokenPair(userID, email, username string, roles []string) (string, string, error) {
	accessToken, err := m.GenerateAccessToken(userID, email, username, roles)
	if err != nil {
		return "", "", err
	}
	
	refreshToken, err := m.GenerateRefreshToken(userID)
	if err != nil {
		return "", "", err
	}
	
	return accessToken, refreshToken, nil
}

// GenerateAccessToken generates an access token
func (m *JWTManager) GenerateAccessToken(userID, email, username string, roles []string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userID,
		Email:     email,
		Username:  username,
		Roles:     roles,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(), // Add unique JWT ID
			Issuer:    m.config.Issuer,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(m.config.AccessTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.config.Secret))
}

// GenerateRefreshToken generates a refresh token
func (m *JWTManager) GenerateRefreshToken(userID string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userID,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(), // Add unique JWT ID
			Issuer:    m.config.Issuer,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(m.config.RefreshTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.config.Secret))
}

// ValidateToken validates a JWT token
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.NewUnauthorizedError("invalid signing method")
		}
		return []byte(m.config.Secret), nil
	})
	
	if err != nil {
		return nil, errors.NewUnauthorizedError("invalid token")
	}
	
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.NewUnauthorizedError("invalid token claims")
	}
	
	return claims, nil
}

// Context keys for storing auth data
type contextKey string

const (
	UserContextKey   contextKey = "user"
	ClaimsContextKey contextKey = "claims"
)

// UserInfo represents authenticated user information
type UserInfo struct {
	ID       string
	Email    string
	Username string
	Roles    []string
}

// GetUserFromContext extracts user info from context
func GetUserFromContext(ctx context.Context) (*UserInfo, error) {
	user, ok := ctx.Value(UserContextKey).(*UserInfo)
	if !ok {
		return nil, errors.NewUnauthorizedError("user not found in context")
	}
	return user, nil
}

// GetClaimsFromContext extracts claims from context
func GetClaimsFromContext(ctx context.Context) (*Claims, error) {
	claims, ok := ctx.Value(ClaimsContextKey).(*Claims)
	if !ok {
		return nil, errors.NewUnauthorizedError("claims not found in context")
	}
	return claims, nil
}

// JWTMiddleware creates a middleware for JWT authentication
func JWTMiddleware(jwtManager *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeUnauthorizedResponse(w, "missing authorization header")
				return
			}
			
			// Check Bearer prefix
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				writeUnauthorizedResponse(w, "invalid authorization header format")
				return
			}
			
			tokenString := parts[1]
			
			// Validate token
			claims, err := jwtManager.ValidateToken(tokenString)
			if err != nil {
				writeUnauthorizedResponse(w, err.Error())
				return
			}
			
			// Check token type
			if claims.TokenType != "access" {
				writeUnauthorizedResponse(w, "invalid token type")
				return
			}
			
			// Create user info
			userInfo := &UserInfo{
				ID:       claims.UserID,
				Email:    claims.Email,
				Username: claims.Username,
				Roles:    claims.Roles,
			}
			
			// Add to context
			ctx := context.WithValue(r.Context(), UserContextKey, userInfo)
			ctx = context.WithValue(ctx, ClaimsContextKey, claims)
			
			// Call next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRoles creates a middleware that checks for specific roles
func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := GetUserFromContext(r.Context())
			if err != nil {
				writeUnauthorizedResponse(w, "unauthorized")
				return
			}
			
			// Check if user has any of the required roles
			hasRole := false
			for _, requiredRole := range roles {
				for _, userRole := range user.Roles {
					if userRole == requiredRole {
						hasRole = true
						break
					}
				}
				if hasRole {
					break
				}
			}
			
			if !hasRole {
				writeForbiddenResponse(w, "insufficient permissions")
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// Helper functions for writing responses
func writeUnauthorizedResponse(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"` + message + `"}`))
}

func writeForbiddenResponse(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error":"` + message + `"}`))
}
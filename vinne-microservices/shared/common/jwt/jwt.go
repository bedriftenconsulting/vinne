package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Service provides JWT operations
type Service interface {
	GenerateAccessToken(claims Claims) (string, error)
	GenerateRefreshToken(claims Claims) (string, error)
	ValidateAccessToken(token string) (*Claims, error)
	ValidateRefreshToken(token string) (*Claims, error)
}

// Config holds JWT configuration
type Config struct {
	AccessSecret    string
	RefreshSecret   string
	AccessDuration  time.Duration
	RefreshDuration time.Duration
}

// Claims represents JWT claims
type Claims struct {
	UserID   string   `json:"sub"`
	Email    string   `json:"email"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`

	// Player-specific fields
	Phone    string `json:"phone,omitempty"`
	Channel  string `json:"channel,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
	JTI      string `json:"jti,omitempty"`
	Issuer   string `json:"iss,omitempty"`

	// Terminal context
	TerminalID string `json:"terminal_id,omitempty"`
	UserType   string `json:"user_type,omitempty"`

	Exp int64 `json:"exp"`
	Iat int64 `json:"iat"`
}

// jwtService implements Service
type jwtService struct {
	config Config
}

// NewService creates a new JWT service
func NewService(config Config) Service {
	return &jwtService{
		config: config,
	}
}

// GenerateAccessToken generates an access token
func (s *jwtService) GenerateAccessToken(claims Claims) (string, error) {
	now := time.Now()
	claims.Iat = now.Unix()
	claims.Exp = now.Add(s.config.AccessDuration).Unix()

	return s.generateToken(claims, s.config.AccessSecret)
}

// GenerateRefreshToken generates a refresh token
func (s *jwtService) GenerateRefreshToken(claims Claims) (string, error) {
	now := time.Now()
	claims.Iat = now.Unix()
	claims.Exp = now.Add(s.config.RefreshDuration).Unix()

	return s.generateToken(claims, s.config.RefreshSecret)
}

// ValidateAccessToken validates an access token
func (s *jwtService) ValidateAccessToken(token string) (*Claims, error) {
	return s.validateToken(token, s.config.AccessSecret)
}

// ValidateRefreshToken validates a refresh token
func (s *jwtService) ValidateRefreshToken(token string) (*Claims, error) {
	return s.validateToken(token, s.config.RefreshSecret)
}

// generateToken generates a JWT token
func (s *jwtService) generateToken(claims Claims, secret string) (string, error) {
	// Create header
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}

	// Encode header
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Encode claims
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	claimsEncoded := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Create signature
	message := headerEncoded + "." + claimsEncoded
	signature := s.sign(message, secret)

	// Combine all parts
	token := message + "." + signature

	return token, nil
}

// validateToken validates a JWT token
func (s *jwtService) validateToken(token string, secret string) (*Claims, error) {
	// Split token
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	headerEncoded := parts[0]
	claimsEncoded := parts[1]
	signatureProvided := parts[2]

	// Verify signature
	message := headerEncoded + "." + claimsEncoded
	signatureExpected := s.sign(message, secret)

	if signatureProvided != signatureExpected {
		return nil, errors.New("invalid signature")
	}

	// Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(claimsEncoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode claims: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal claims: %w", err)
	}

	// Check expiration
	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}

// sign creates HMAC SHA256 signature
func (s *jwtService) sign(message, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	signature := h.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(signature)
}

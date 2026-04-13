package orange

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/service-payment/internal/providers"
)

// AuthManager handles authentication for Orange API
type AuthManager struct {
	client      *HTTPClient
	baseURL     string
	secretKey   string
	secretToken string
	log         logger.Logger

	// Token management
	currentToken *providers.AuthResult
	mu           sync.RWMutex

	// Auto-refresh
	refreshTimer *time.Timer
}

// NewAuthManager creates a new authentication manager
func NewAuthManager(client *HTTPClient, baseURL, secretKey, secretToken string, log logger.Logger) *AuthManager {
	return &AuthManager{
		client:      client,
		baseURL:     baseURL,
		secretKey:   secretKey,
		secretToken: secretToken,
		log:         log,
	}
}

// Authenticate authenticates with Orange API and obtains access token
func (a *AuthManager) Authenticate(ctx context.Context) (*providers.AuthResult, error) {
	ctx, span := tracer.Start(ctx, "orange.auth_manager.authenticate")
	defer span.End()

	// Check if we already have a valid token
	a.mu.RLock()
	if a.currentToken != nil && time.Now().Before(a.currentToken.ExpiresAt) {
		span.SetAttributes(attribute.Bool("cached", true))
		token := a.currentToken
		a.mu.RUnlock()
		a.log.Debug("Using cached authentication token",
			"expires_at", token.ExpiresAt.Format(time.RFC3339),
			"time_until_expiry", time.Until(token.ExpiresAt).String())
		return token, nil
	}
	a.mu.RUnlock()

	// Need to acquire new token
	a.mu.Lock()
	defer a.mu.Unlock()

	// Double-check after acquiring write lock
	if a.currentToken != nil && time.Now().Before(a.currentToken.ExpiresAt) {
		span.SetAttributes(attribute.Bool("cached", true))
		a.log.Debug("Using cached authentication token (double-check)",
			"expires_at", a.currentToken.ExpiresAt.Format(time.RFC3339))
		return a.currentToken, nil
	}

	span.SetAttributes(attribute.Bool("cached", false))

	// Log authentication attempt
	// Ensure no double slashes in URL construction
	baseURL := strings.TrimSuffix(a.baseURL, "/")
	authURL := fmt.Sprintf("%s/Auth/token", baseURL)
	a.log.Info("Attempting Orange API authentication",
		"endpoint", authURL,
		"base_url", a.baseURL,
		"has_secret_key", a.secretKey != "",
		"has_secret_token", a.secretToken != "")

	// Prepare request (Orange uses custom header auth, not OAuth)
	req := &HTTPRequest{
		Method: "POST",
		URL:    authURL,
		Headers: map[string]string{
			"secretKey":    a.secretKey,
			"secretToken":  a.secretToken,
			"Content-Type": "application/json",
		},
		Body: nil, // Orange Auth/token endpoint requires no body
	}

	a.log.Debug("Authentication request prepared",
		"method", req.Method,
		"url", req.URL,
		"headers_count", len(req.Headers))

	// Execute request
	// Orange API response format:
	// {
	//   "status": 1,
	//   "message": "Success",
	//   "data": {
	//     "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
	//     "expiry": "2025-10-15T23:59:59Z",
	//     "merchantName": "Rand Lottery Ltd"
	//   }
	// }
	var response struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Data    struct {
			Token        string `json:"token"`
			Expiry       string `json:"expiry"`
			MerchantName string `json:"merchantName"`
		} `json:"data"`
	}

	if err := a.client.Do(ctx, req, &response); err != nil {
		span.RecordError(err)
		a.log.Error("Orange API authentication failed",
			"error", err,
			"url", authURL)
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	a.log.Info("Authentication response received",
		"status", response.Status,
		"message", response.Message,
		"merchant_name", response.Data.MerchantName,
		"has_token", response.Data.Token != "",
		"token_expiry", response.Data.Expiry)

	// Check response status
	if response.Status != 1 {
		err := fmt.Errorf("authentication failed with status %d: %s", response.Status, response.Message)
		span.RecordError(err)
		a.log.Error("Orange API authentication rejected",
			"status", response.Status,
			"message", response.Message)
		return nil, err
	}

	// Parse expiry time
	expiresAt, err := time.Parse(time.RFC3339, response.Data.Expiry)
	if err != nil {
		// Fallback: assume token valid for 1 hour
		expiresAt = time.Now().Add(55 * time.Minute) // 55 minutes buffer
		a.log.Warn("Failed to parse token expiry, using 1 hour default",
			"error", err,
			"expiry_string", response.Data.Expiry,
			"fallback_expiry", expiresAt.Format(time.RFC3339))
	}

	// Store token
	a.currentToken = &providers.AuthResult{
		Token:     response.Data.Token,
		ExpiresAt: expiresAt,
		ProviderData: map[string]interface{}{
			"merchant_name": response.Data.MerchantName,
			"status":        response.Status,
			"message":       response.Message,
			"expiry":        response.Data.Expiry,
		},
	}

	span.SetAttributes(
		attribute.String("merchant_name", response.Data.MerchantName),
		attribute.Int("status", response.Status),
		attribute.String("expires_at", expiresAt.Format(time.RFC3339)),
	)

	a.log.Info("Orange API authentication successful",
		"merchant_name", response.Data.MerchantName,
		"expires_at", expiresAt.Format(time.RFC3339),
		"time_until_expiry", time.Until(expiresAt).String())

	// Schedule auto-refresh (refresh 10 minutes before expiration)
	a.scheduleRefresh(expiresAt)

	return a.currentToken, nil
}

// RefreshAuth refreshes the access token
func (a *AuthManager) RefreshAuth(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "orange.auth_manager.refresh_auth")
	defer span.End()

	// Orange uses client_credentials flow, so we just re-authenticate
	_, err := a.Authenticate(ctx)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("token refresh failed: %w", err)
	}

	return nil
}

// IsAuthenticated checks if we have a valid token
func (a *AuthManager) IsAuthenticated() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.currentToken != nil && time.Now().Before(a.currentToken.ExpiresAt)
}

// GetToken returns the current access token
func (a *AuthManager) GetToken() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.currentToken == nil {
		return "", fmt.Errorf("not authenticated")
	}

	if time.Now().After(a.currentToken.ExpiresAt) {
		return "", fmt.Errorf("token expired")
	}

	return a.currentToken.Token, nil
}

// scheduleRefresh schedules automatic token refresh
func (a *AuthManager) scheduleRefresh(expiresAt time.Time) {
	// Cancel existing timer if any
	if a.refreshTimer != nil {
		a.refreshTimer.Stop()
	}

	// Calculate refresh time (10 minutes before expiration)
	refreshIn := time.Until(expiresAt) - (10 * time.Minute)
	if refreshIn < 0 {
		// If less than 10 minutes left, refresh in 1 minute
		refreshIn = time.Minute
	}

	a.refreshTimer = time.AfterFunc(refreshIn, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := a.RefreshAuth(ctx); err != nil {
			// Log error (in production, this should be proper logging)
			fmt.Printf("Auto-refresh failed: %v\n", err)

			// Retry after 1 minute
			a.mu.Lock()
			if a.refreshTimer != nil {
				a.refreshTimer.Reset(time.Minute)
			}
			a.mu.Unlock()
		}
	})
}

// Stop stops the auto-refresh timer
func (a *AuthManager) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.refreshTimer != nil {
		a.refreshTimer.Stop()
		a.refreshTimer = nil
	}
}

package orange

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/service-payment/internal/config"
	"github.com/randco/service-payment/internal/providers"
	"github.com/randco/service-payment/internal/resilience"
)

// Provider implements the PaymentProvider interface for Orange Extensibility API
type Provider struct {
	config         *config.OrangeConfig
	httpClient     *HTTPClient
	authManager    *AuthManager
	circuitBreaker *resilience.CircuitBreaker
	log            logger.Logger
}

// NewProvider creates a new Orange payment provider
func NewProvider(cfg *config.OrangeConfig, log logger.Logger) (*Provider, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("orange provider is disabled")
	}

	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("orange base URL is required")
	}

	if cfg.SecretKey == "" || cfg.SecretToken == "" {
		return nil, fmt.Errorf("orange credentials are required")
	}

	// Create HTTP client with retry logic
	httpClient := NewHTTPClient(cfg.Timeout, cfg.RetryAttempts, cfg.RetryDelay)
	httpClient.SetLogger(log) // Set logger for detailed request/response logging

	// Create auth manager
	authManager := NewAuthManager(httpClient, cfg.BaseURL, cfg.SecretKey, cfg.SecretToken, log)

	// Create circuit breaker (5 failures, 60 second timeout)
	circuitBreaker := resilience.NewCircuitBreaker("orange", 5, 60*time.Second)

	log.Info("Orange payment provider initialized",
		"base_url", cfg.BaseURL,
		"environment", cfg.Environment,
		"timeout", cfg.Timeout,
		"retry_attempts", cfg.RetryAttempts)

	return &Provider{
		config:         cfg,
		httpClient:     httpClient,
		authManager:    authManager,
		circuitBreaker: circuitBreaker,
		log:            log,
	}, nil
}

// GetProviderName returns the provider name
func (p *Provider) GetProviderName() string {
	return "Orange"
}

// GetProviderType returns the provider type
func (p *Provider) GetProviderType() providers.ProviderType {
	return providers.ProviderTypeAggregator
}

// Authenticate authenticates with Orange API
func (p *Provider) Authenticate(ctx context.Context) (*providers.AuthResult, error) {
	ctx, span := tracer.Start(ctx, "orange.provider.authenticate")
	defer span.End()

	var result *providers.AuthResult
	var err error

	// Execute with circuit breaker protection
	cbErr := p.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		result, err = p.authManager.Authenticate(ctx)
		return err
	})

	if cbErr != nil {
		span.RecordError(cbErr)
		return nil, cbErr
	}

	return result, nil
}

// RefreshAuth refreshes the authentication token
func (p *Provider) RefreshAuth(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "orange.provider.refresh_auth")
	defer span.End()

	var err error

	// Execute with circuit breaker protection
	cbErr := p.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		err = p.authManager.RefreshAuth(ctx)
		return err
	})

	if cbErr != nil {
		span.RecordError(cbErr)
		return cbErr
	}

	return nil
}

// IsAuthenticated checks if the provider is authenticated
func (p *Provider) IsAuthenticated() bool {
	return p.authManager.IsAuthenticated()
}

// HealthCheck performs a health check on the provider
func (p *Provider) HealthCheck(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "orange.provider.health_check",
		trace.WithAttributes(
			attribute.String("provider", "orange"),
			attribute.String("circuit_state", string(p.circuitBreaker.GetState())),
		))
	defer span.End()

	// Check circuit breaker state
	if p.circuitBreaker.GetState() == resilience.CircuitStateOpen {
		err := fmt.Errorf("circuit breaker is open")
		span.RecordError(err)
		return err
	}

	// Try to authenticate (will use cached token if valid)
	if _, err := p.Authenticate(ctx); err != nil {
		span.RecordError(err)
		return fmt.Errorf("authentication failed: %w", err)
	}

	span.SetAttributes(attribute.Bool("healthy", true))
	return nil
}

// GetSupportedOperations returns the operations supported by Orange
func (p *Provider) GetSupportedOperations() []providers.OperationType {
	return []providers.OperationType{
		providers.OpWalletVerify,
		providers.OpBankAccountVerify,
		providers.OpIdentityVerify,
		providers.OpWalletDebit,
		providers.OpWalletCredit,
		providers.OpBankTransfer,
		providers.OpStatusCheck,
	}
}

// GetTransactionLimits returns the transaction limits for Orange
func (p *Provider) GetTransactionLimits() *providers.TransactionLimits {
	// These are example limits - should be configured or fetched from Orange
	return &providers.TransactionLimits{
		MinAmount:    100,       // ₵1.00 in pesewas
		MaxAmount:    10000000,  // ₵100,000 in pesewas
		DailyLimit:   50000000,  // ₵500,000 in pesewas
		MonthlyLimit: 200000000, // ₵2,000,000 in pesewas
		Currency:     "GHS",
	}
}

// GetSupportedCurrencies returns the currencies supported by Orange
func (p *Provider) GetSupportedCurrencies() []string {
	return []string{"GHS"} // Ghana Cedis
}

// makeAuthenticatedRequest makes an authenticated request to Orange API
func (p *Provider) makeAuthenticatedRequest(ctx context.Context, method, endpoint string, body interface{}, result interface{}) error {
	ctx, span := tracer.Start(ctx, "orange.provider.make_authenticated_request",
		trace.WithAttributes(
			attribute.String("method", method),
			attribute.String("endpoint", endpoint),
		))
	defer span.End()

	p.log.Debug("Making authenticated request to Orange API",
		"method", method,
		"endpoint", endpoint,
		"full_url", fmt.Sprintf("%s%s", p.config.BaseURL, endpoint))

	// Get access token
	token, err := p.authManager.GetToken()
	if err != nil {
		p.log.Warn("No valid token available, attempting authentication",
			"error", err)
		// Try to authenticate if we don't have a token
		if _, authErr := p.Authenticate(ctx); authErr != nil {
			span.RecordError(authErr)
			p.log.Error("Authentication failed",
				"error", authErr)
			return fmt.Errorf("authentication failed: %w", authErr)
		}

		p.log.Info("Authentication successful, retrying token retrieval")

		// Try to get token again
		token, err = p.authManager.GetToken()
		if err != nil {
			span.RecordError(err)
			p.log.Error("Failed to get token after authentication",
				"error", err)
			return fmt.Errorf("failed to get token: %w", err)
		}
	}

	p.log.Debug("Token retrieved successfully",
		"has_token", token != "",
		"token_length", len(token))

	// Prepare request - ensure no double slashes in URL
	baseURL := strings.TrimSuffix(p.config.BaseURL, "/")
	fullURL := fmt.Sprintf("%s%s", baseURL, endpoint)
	req := &HTTPRequest{
		Method: method,
		URL:    fullURL,
		Headers: map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", token),
			"Content-Type":  "application/json",
		},
		Body: body,
	}

	p.log.Debug("Request prepared, executing with circuit breaker",
		"circuit_state", string(p.circuitBreaker.GetState()))

	// Execute with circuit breaker protection
	var requestErr error
	cbErr := p.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		requestErr = p.httpClient.Do(ctx, req, result)
		return requestErr
	})

	if cbErr != nil {
		span.RecordError(cbErr)
		p.log.Error("Request failed",
			"error", cbErr,
			"method", method,
			"endpoint", endpoint)
		return cbErr
	}

	p.log.Debug("Request completed successfully",
		"method", method,
		"endpoint", endpoint)

	return nil
}

// Stop gracefully stops the provider
func (p *Provider) Stop() {
	p.authManager.Stop()
}

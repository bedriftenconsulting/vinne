package providers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/randco/randco-microservices/services/service-notification/internal/config"
	"github.com/randco/randco-microservices/services/service-notification/internal/providers/email/mailgun"
	"github.com/randco/randco-microservices/services/service-notification/internal/providers/sms/hubtel"
	"github.com/randco/randco-microservices/services/service-notification/internal/ratelimit"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/redis/go-redis/v9"
)

// ProviderManager manages provider instances and health checks
type ProviderManager struct {
	factory         *ProviderFactory
	config          *config.ProviderConfig
	healthChecker   *HealthChecker
	circuitBreakers map[string]*CircuitBreaker
	mu              sync.RWMutex
}

type HealthChecker struct {
	providers map[string]ProviderHealth
	interval  time.Duration
	stopCh    chan struct{}
	mu        sync.RWMutex
}

type ProviderHealth struct {
	IsHealthy  bool      `json:"is_healthy"`
	LastCheck  time.Time `json:"last_check"`
	LastError  error     `json:"last_error,omitempty"`
	CheckCount int       `json:"check_count"`
	ErrorCount int       `json:"error_count"`
}

func NewProviderManager(cfg *config.ProviderConfig, rateLimiterCfg config.RateLimitConfig, redisClient *redis.Client, log logger.Logger) (*ProviderManager, error) {
	factory := NewProviderFactory()

	// Initialize distributed rate limiter with Redis
	rateLimiter := ratelimit.NewRateLimiter(ratelimit.Config{
		EmailRatePerHour: rateLimiterCfg.EmailRatePerHour,
		SMSRatePerMinute: rateLimiterCfg.SMSRatePerMinute,
	}, redisClient)

	if cfg.Email.Mailgun.Enabled {
		if cfg.Email.Mailgun.APIKey == "" || cfg.Email.Mailgun.Domain == "" {
			return nil, fmt.Errorf("mailgun configuration incomplete: missing api_key or domain")
		}

		mailgunAdapter := mailgun.NewMailgunAdapter(mailgun.MailgunConfig{
			APIKey:  cfg.Email.Mailgun.APIKey,
			Domain:  cfg.Email.Mailgun.Domain,
			BaseURL: cfg.Email.Mailgun.BaseURL,
		}, log, rateLimiter)
		factory.RegisterEmailProvider("mailgun", &MailgunWrapper{adapter: mailgunAdapter})
	}

	if cfg.SMS.Hubtel.Enabled {
		if cfg.SMS.Hubtel.ClientID == "" || cfg.SMS.Hubtel.ClientSecret == "" || cfg.SMS.Hubtel.SenderID == "" {
			return nil, fmt.Errorf("hubtel configuration incomplete: missing client_id or client_secret")
		}

		hubtelAdapter := hubtel.NewHubtelAdapter(hubtel.HubtelConfig{
			ClientID:     cfg.SMS.Hubtel.ClientID,
			ClientSecret: cfg.SMS.Hubtel.ClientSecret,
			BaseURL:      cfg.SMS.Hubtel.BaseURL,
			SenderID:     cfg.SMS.Hubtel.SenderID,
		})
		factory.RegisterSMSProvider("hubtel", &HubtelWrapper{adapter: hubtelAdapter})
	}

	healthChecker := &HealthChecker{
		providers: make(map[string]ProviderHealth),
		interval:  30 * time.Second,
		stopCh:    make(chan struct{}),
	}

	// Initialize circuit breakers for each provider
	circuitBreakers := make(map[string]*CircuitBreaker)

	// Create circuit breakers for email providers
	for _, providerName := range factory.ListEmailProviders() {
		circuitBreakers[providerName] = NewCircuitBreaker(CircuitBreakerConfig{
			MaxFailures:      5,                // Open circuit after 5 consecutive failures
			ResetTimeout:     30 * time.Second, // Try to recover after 30 seconds
			HalfOpenMaxTries: 3,                // Allow 3 test requests in half-open state
		})
	}

	// Create circuit breakers for SMS providers
	for _, providerName := range factory.ListSMSProviders() {
		circuitBreakers[providerName] = NewCircuitBreaker(CircuitBreakerConfig{
			MaxFailures:      5,
			ResetTimeout:     30 * time.Second,
			HalfOpenMaxTries: 3,
		})
	}

	manager := &ProviderManager{
		factory:         factory,
		config:          cfg,
		healthChecker:   healthChecker,
		circuitBreakers: circuitBreakers,
	}

	// go manager.healthChecker.start(manager.factory)

	return manager, nil
}

func (pm *ProviderManager) GetEmailProvider(name ...string) (EmailProvider, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	providerName := pm.config.Email.DefaultProvider
	if len(name) > 0 && name[0] != "" {
		providerName = name[0]
	}

	return pm.factory.GetEmailProvider(providerName)
}

func (pm *ProviderManager) GetSMSProvider(name ...string) (SMSProvider, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	providerName := pm.config.SMS.DefaultProvider
	if len(name) > 0 && name[0] != "" {
		providerName = name[0]
	}

	return pm.factory.GetSMSProvider(providerName)
}

func (pm *ProviderManager) SendEmail(ctx context.Context, req *EmailRequest, providerName ...string) (*EmailResponse, error) {
	provider, err := pm.GetEmailProvider(providerName...)
	if err != nil {
		return nil, fmt.Errorf("failed to get email provider: %w", err)
	}

	// Check circuit breaker before attempting to send
	circuitBreaker := pm.getCircuitBreaker(provider.GetProviderName())
	if circuitBreaker != nil {
		if err := circuitBreaker.Allow(); err != nil {
			// Circuit is open, provider is unavailable
			return nil, fmt.Errorf("circuit breaker open for provider %s: %w", provider.GetProviderName(), err)
		}
	}

	response, err := provider.SendEmail(ctx, req)
	if err != nil {
		pm.healthChecker.recordError(provider.GetProviderName(), err)
		if circuitBreaker != nil {
			circuitBreaker.RecordFailure()
		}
		return nil, err
	}

	pm.healthChecker.recordSuccess(provider.GetProviderName())
	if circuitBreaker != nil {
		circuitBreaker.RecordSuccess()
	}
	return response, nil
}

func (pm *ProviderManager) SendSMS(ctx context.Context, req *SMSRequest, providerName ...string) (*SMSResponse, error) {
	provider, err := pm.GetSMSProvider(providerName...)
	if err != nil {
		return nil, fmt.Errorf("failed to get SMS provider: %w", err)
	}

	// Check circuit breaker before attempting to send
	circuitBreaker := pm.getCircuitBreaker(provider.GetProviderName())
	if circuitBreaker != nil {
		if err := circuitBreaker.Allow(); err != nil {
			// Circuit is open, provider is unavailable
			return nil, fmt.Errorf("circuit breaker open for provider %s: %w", provider.GetProviderName(), err)
		}
	}

	response, err := provider.SendSMS(ctx, req)
	if err != nil {
		pm.healthChecker.recordError(provider.GetProviderName(), err)
		if circuitBreaker != nil {
			circuitBreaker.RecordFailure()
		}
		return nil, err
	}

	pm.healthChecker.recordSuccess(provider.GetProviderName())
	if circuitBreaker != nil {
		circuitBreaker.RecordSuccess()
	}
	return response, nil
}

func (pm *ProviderManager) GetProviderHealth() map[string]ProviderHealth {
	return pm.healthChecker.getAllHealth()
}

func (pm *ProviderManager) Shutdown() {
	close(pm.healthChecker.stopCh)
}

func (pm *ProviderManager) ListEmailProviders() []string {
	return pm.factory.ListEmailProviders()
}

func (pm *ProviderManager) ListSMSProviders() []string {
	return pm.factory.ListSMSProviders()
}

//nolint:unused // Reserved for future health check implementation
func (hc *HealthChecker) start(factory *ProviderFactory) {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.performHealthChecks(factory)
		case <-hc.stopCh:
			return
		}
	}
}

//nolint:unused // Reserved for future health check implementation
func (hc *HealthChecker) performHealthChecks(factory *ProviderFactory) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check email providers
	for _, providerName := range factory.ListEmailProviders() {
		provider, err := factory.GetEmailProvider(providerName)
		if err != nil {
			continue
		}

		err = provider.IsHealthy(ctx)
		hc.updateHealth(providerName, err)
	}

	// Check SMS providers
	for _, providerName := range factory.ListSMSProviders() {
		provider, err := factory.GetSMSProvider(providerName)
		if err != nil {
			continue
		}

		err = provider.IsHealthy(ctx)
		hc.updateHealth(providerName, err)
	}
}

// updateHealth updates the health status of a provider
func (hc *HealthChecker) updateHealth(providerName string, err error) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	health := hc.providers[providerName]
	health.LastCheck = time.Now()
	health.CheckCount++

	if err != nil {
		health.IsHealthy = false
		health.LastError = err
		health.ErrorCount++
	} else {
		health.IsHealthy = true
		health.LastError = nil
	}

	hc.providers[providerName] = health
}

// recordError records an error for a provider
func (hc *HealthChecker) recordError(providerName string, err error) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	health := hc.providers[providerName]
	health.LastError = err
	health.ErrorCount++
	health.IsHealthy = false
	hc.providers[providerName] = health
}

// getCircuitBreaker returns the circuit breaker for a provider
func (pm *ProviderManager) getCircuitBreaker(providerName string) *CircuitBreaker {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.circuitBreakers[providerName]
}

// GetCircuitBreakerStats returns circuit breaker statistics for all providers
func (pm *ProviderManager) GetCircuitBreakerStats() map[string]CircuitBreakerStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := make(map[string]CircuitBreakerStats)
	for providerName, cb := range pm.circuitBreakers {
		stats[providerName] = cb.GetStats()
	}
	return stats
}

// recordSuccess records a successful operation for a provider
func (hc *HealthChecker) recordSuccess(providerName string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	health := hc.providers[providerName]
	health.LastError = nil
	health.IsHealthy = true
	hc.providers[providerName] = health
}

// isHealthy checks if a provider is currently healthy
func (hc *HealthChecker) isHealthy(providerName string) bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	health, exists := hc.providers[providerName]
	if !exists {
		return true // Assume healthy if not checked yet
	}

	return health.IsHealthy
}

// getAllHealth returns the health status of all providers
func (hc *HealthChecker) getAllHealth() map[string]ProviderHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result := make(map[string]ProviderHealth)
	for name, health := range hc.providers {
		result[name] = health
	}
	return result
}

// RetryableError checks if an error is retryable
func RetryableError(err error) bool {
	if providerErr, ok := err.(*ProviderError); ok {
		return providerErr.Retryable
	}
	return false
}

// GetErrorCode extracts the error code from a provider error
func GetErrorCode(err error) string {
	if providerErr, ok := err.(*ProviderError); ok {
		return providerErr.Code
	}
	return "UNKNOWN_ERROR"
}

// IsProviderError checks if an error is a provider error
func IsProviderError(err error) bool {
	_, ok := err.(*ProviderError)
	return ok
}

// MailgunWrapper wraps the mailgun adapter to implement the common EmailProvider interface
type MailgunWrapper struct {
	adapter *mailgun.MailgunAdapter
}

func (w *MailgunWrapper) SendEmail(ctx context.Context, req *EmailRequest) (*EmailResponse, error) {
	mailgunReq := &mailgun.EmailRequest{
		To:          req.To,
		CC:          req.CC,
		BCC:         req.BCC,
		Subject:     req.Subject,
		HTMLContent: req.HTMLContent,
		TextContent: req.TextContent,
		Attachments: convertAttachments(req.Attachments),
		Variables:   req.Variables,
		Priority:    mailgun.Priority(req.Priority),
	}

	resp, err := w.adapter.SendEmail(ctx, mailgunReq)
	if err != nil {
		return nil, err
	}

	return &EmailResponse{
		MessageID:    resp.MessageID,
		Status:       DeliveryStatus(resp.Status),
		Provider:     resp.Provider,
		SentAt:       resp.SentAt,
		ProviderData: resp.ProviderData,
	}, nil
}

func (w *MailgunWrapper) GetStatus(ctx context.Context, messageID string) (*DeliveryStatus, error) {
	status, err := w.adapter.GetStatus(ctx, messageID)
	if err != nil {
		return nil, err
	}

	deliveryStatus := DeliveryStatus(*status)
	return &deliveryStatus, nil
}

func (w *MailgunWrapper) ValidateAddress(ctx context.Context, email string) error {
	return w.adapter.ValidateAddress(ctx, email)
}

func (w *MailgunWrapper) GetProviderName() string {
	return w.adapter.GetProviderName()
}

func (w *MailgunWrapper) IsHealthy(ctx context.Context) error {
	return w.adapter.IsHealthy(ctx)
}

// HubtelWrapper wraps the hubtel adapter to implement the common SMSProvider interface
type HubtelWrapper struct {
	adapter *hubtel.HubtelAdapter
}

func (w *HubtelWrapper) SendSMS(ctx context.Context, req *SMSRequest) (*SMSResponse, error) {
	hubtelReq := &hubtel.SMSRequest{
		To:        req.To,
		Content:   req.Content,
		Variables: req.Variables,
		Priority:  hubtel.Priority(req.Priority),
	}

	resp, err := w.adapter.SendSMS(ctx, hubtelReq)
	if err != nil {
		return nil, err
	}

	return &SMSResponse{
		MessageID:    resp.MessageID,
		Status:       DeliveryStatus(resp.Status),
		Provider:     resp.Provider,
		SentAt:       resp.SentAt,
		ProviderData: resp.ProviderData,
	}, nil
}

func (w *HubtelWrapper) GetStatus(ctx context.Context, messageID string) (*DeliveryStatus, error) {
	status, err := w.adapter.GetStatus(ctx, messageID)
	if err != nil {
		return nil, err
	}

	deliveryStatus := DeliveryStatus(*status)
	return &deliveryStatus, nil
}

func (w *HubtelWrapper) ValidateNumber(ctx context.Context, phone string) error {
	return w.adapter.ValidateNumber(ctx, phone)
}

func (w *HubtelWrapper) GetProviderName() string {
	return w.adapter.GetProviderName()
}

func (w *HubtelWrapper) IsHealthy(ctx context.Context) error {
	return w.adapter.IsHealthy(ctx)
}

// convertAttachments converts common attachments to mailgun attachments
func convertAttachments(attachments []Attachment) []mailgun.Attachment {
	result := make([]mailgun.Attachment, len(attachments))
	for i, att := range attachments {
		result[i] = mailgun.Attachment{
			Filename:    att.Filename,
			ContentType: att.ContentType,
			Data:        att.Data,
		}
	}
	return result
}

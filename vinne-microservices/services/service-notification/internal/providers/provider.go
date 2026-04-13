package providers

import (
	"context"
	"time"
)

type DeliveryStatus string

const (
	StatusQueued    DeliveryStatus = "queued"
	StatusSending   DeliveryStatus = "sending"
	StatusDelivered DeliveryStatus = "delivered"
	StatusFailed    DeliveryStatus = "failed"
	StatusBounced   DeliveryStatus = "bounced"
	StatusRejected  DeliveryStatus = "rejected"
)

type Priority int

const (
	PriorityLow      Priority = 0
	PriorityNormal   Priority = 1
	PriorityHigh     Priority = 2
	PriorityCritical Priority = 3
)

type EmailRequest struct {
	To          string            `json:"to"`
	CC          []string          `json:"cc,omitempty"`
	BCC         []string          `json:"bcc,omitempty"`
	Subject     string            `json:"subject"`
	HTMLContent string            `json:"html_content,omitempty"`
	TextContent string            `json:"text_content,omitempty"`
	Attachments []Attachment      `json:"attachments,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	Priority    Priority          `json:"priority"`
}

type SMSRequest struct {
	To        string            `json:"to"`
	Content   string            `json:"content"`
	Variables map[string]string `json:"variables,omitempty"`
	Priority  Priority          `json:"priority"`
}

type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Data        []byte `json:"data"`
}

type EmailResponse struct {
	MessageID    string         `json:"message_id"`
	Status       DeliveryStatus `json:"status"`
	Provider     string         `json:"provider"`
	SentAt       time.Time      `json:"sent_at"`
	ProviderData map[string]any `json:"provider_data,omitempty"`
}

type SMSResponse struct {
	MessageID    string         `json:"message_id"`
	Status       DeliveryStatus `json:"status"`
	Provider     string         `json:"provider"`
	SentAt       time.Time      `json:"sent_at"`
	ProviderData map[string]any `json:"provider_data,omitempty"`
}

type ProviderError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Provider   string `json:"provider"`
	Retryable  bool   `json:"retryable"`
	StatusCode int    `json:"status_code,omitempty"`
}

func (e *ProviderError) Error() string {
	return e.Message
}

type EmailProvider interface {
	// SendEmail sends an email and returns the response
	SendEmail(ctx context.Context, req *EmailRequest) (*EmailResponse, error)

	// GetStatus retrieves the delivery status of an email
	GetStatus(ctx context.Context, messageID string) (*DeliveryStatus, error)

	// ValidateAddress validates an email address format
	ValidateAddress(ctx context.Context, email string) error

	// GetProviderName returns the name of the provider
	GetProviderName() string

	// IsHealthy checks if the provider is healthy and reachable
	IsHealthy(ctx context.Context) error
}

type SMSProvider interface {
	// SendSMS sends an SMS and returns the response
	SendSMS(ctx context.Context, req *SMSRequest) (*SMSResponse, error)

	// GetStatus retrieves the delivery status of an SMS
	GetStatus(ctx context.Context, messageID string) (*DeliveryStatus, error)

	// ValidateNumber validates a phone number format
	ValidateNumber(ctx context.Context, phone string) error

	// GetProviderName returns the name of the provider
	GetProviderName() string

	// IsHealthy checks if the provider is healthy and reachable
	IsHealthy(ctx context.Context) error
}

type ProviderFactory struct {
	emailProviders map[string]EmailProvider
	smsProviders   map[string]SMSProvider
}

func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		emailProviders: make(map[string]EmailProvider),
		smsProviders:   make(map[string]SMSProvider),
	}
}

func (pf *ProviderFactory) RegisterEmailProvider(name string, provider EmailProvider) {
	pf.emailProviders[name] = provider
}

func (pf *ProviderFactory) RegisterSMSProvider(name string, provider SMSProvider) {
	pf.smsProviders[name] = provider
}

func (pf *ProviderFactory) GetEmailProvider(name string) (EmailProvider, error) {
	provider, exists := pf.emailProviders[name]
	if !exists {
		return nil, &ProviderError{
			Code:      "PROVIDER_NOT_FOUND",
			Message:   "Email provider not found: " + name,
			Provider:  name,
			Retryable: false,
		}
	}
	return provider, nil
}

func (pf *ProviderFactory) GetSMSProvider(name string) (SMSProvider, error) {
	provider, exists := pf.smsProviders[name]
	if !exists {
		return nil, &ProviderError{
			Code:      "PROVIDER_NOT_FOUND",
			Message:   "SMS provider not found: " + name,
			Provider:  name,
			Retryable: false,
		}
	}
	return provider, nil
}

func (pf *ProviderFactory) ListEmailProviders() []string {
	providers := make([]string, 0, len(pf.emailProviders))
	for name := range pf.emailProviders {
		providers = append(providers, name)
	}
	return providers
}

func (pf *ProviderFactory) ListSMSProviders() []string {
	providers := make([]string, 0, len(pf.smsProviders))
	for name := range pf.smsProviders {
		providers = append(providers, name)
	}
	return providers
}

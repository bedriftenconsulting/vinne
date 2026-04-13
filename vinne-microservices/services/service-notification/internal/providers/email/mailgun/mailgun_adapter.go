package mailgun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/randco/randco-microservices/services/service-notification/internal/tracing"
	"github.com/randco/randco-microservices/shared/common/logger"
)

const (
	mailgunUSAPIURL = "https://api.mailgun.net"
	mailgunEUAPIURL = "https://api.eu.mailgun.net"
	providerName    = "mailgun"
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

type RateLimiter interface {
	Wait(ctx context.Context, channel string) error
	Release(ctx context.Context, channel string) error
}

type MailgunAdapter struct {
	apiKey      string
	domain      string
	baseURL     string
	httpClient  *http.Client
	logger      logger.Logger
	rateLimiter RateLimiter
}

type MailgunConfig struct {
	APIKey  string `json:"api_key"`
	Domain  string `json:"domain"`
	Region  string `json:"region,omitempty"` // "us" or "eu", defaults to "us"
	BaseURL string `json:"base_url,omitempty"`
}

func NewMailgunAdapter(config MailgunConfig, log logger.Logger, rateLimiter RateLimiter) *MailgunAdapter {
	baseURL := config.BaseURL
	if baseURL == "" {
		// Default to US region, only use EU if explicitly specified
		if strings.ToLower(config.Region) == "eu" {
			baseURL = mailgunEUAPIURL
		} else {
			baseURL = mailgunUSAPIURL
		}
	}

	return &MailgunAdapter{
		apiKey:      config.APIKey,
		domain:      config.Domain,
		baseURL:     baseURL,
		logger:      log,
		rateLimiter: rateLimiter,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (m *MailgunAdapter) SendEmail(ctx context.Context, req *EmailRequest) (*EmailResponse, error) {
	// ========================================
	// HARD BLOCK: EMAIL SENDING DISABLED
	// ========================================
	// Temporary block to prevent email flooding until root cause is resolved
	// To re-enable: Comment out the next 13 lines and uncomment the rest of the function
	m.logger.Warn("[MAILGUN] EMAIL SENDING BLOCKED - All emails temporarily disabled to prevent flooding",
		"to", req.To,
		"subject", req.Subject,
	)
	return &EmailResponse{
		MessageID: fmt.Sprintf("blocked-%d", time.Now().Unix()),
		Status:    StatusQueued, // Report as queued to avoid triggering error retries
		Provider:  providerName,
		SentAt:    time.Now(),
		ProviderData: map[string]any{
			"blocked": true,
			"reason":  "Email sending temporarily disabled to prevent flooding",
		},
	}, nil
	// ========================================

	// COMMENTED OUT - Original email sending code (uncomment to re-enable)
	/*
		var response *EmailResponse
		err := tracing.TraceEmailProviderCall(ctx, providerName, "send", func(ctx context.Context) error {
			// Validate recipient email address
			if err := m.ValidateAddress(ctx, req.To); err != nil {
				return err
			}

			// Validate that we have at least some content to send
			// Allow empty subject for flexibility (some notifications may not need it)
			if req.HTMLContent == "" && req.TextContent == "" {
				return &ProviderError{
					Code:      "MISSING_CONTENT",
					Message:   "Email must have either HTML or text content",
					Provider:  providerName,
					Retryable: false,
				}
			}

			// Apply rate limiting - wait if necessary
			if m.rateLimiter != nil {
				if err := m.rateLimiter.Wait(ctx, "email"); err != nil {
					m.logger.Warn("[MAILGUN] Rate limit reached for email sending",
						"error", err,
						"to", req.To,
					)
					return &ProviderError{
						Code:      "RATE_LIMIT_EXCEEDED",
						Message:   fmt.Sprintf("Rate limit exceeded for email sending: %v", err),
						Provider:  providerName,
						Retryable: true,
					}
				}
			}

			formData, contentType, err := m.prepareFormData(req)
			if err != nil {
				return &ProviderError{
					Code:      "FORM_DATA_ERROR",
					Message:   fmt.Sprintf("Failed to prepare form data: %v", err),
					Provider:  providerName,
					Retryable: false,
				}
			}

			url := fmt.Sprintf("%s/v3/%s/messages", m.baseURL, m.domain)
			httpReq, err := http.NewRequestWithContext(ctx, "POST", url, formData)
			if err != nil {
				return &ProviderError{
					Code:      "REQUEST_ERROR",
					Message:   fmt.Sprintf("Failed to create request: %v", err),
					Provider:  providerName,
					Retryable: true,
				}
			}

			httpReq.Header.Set("Content-Type", contentType)
			httpReq.SetBasicAuth("api", m.apiKey)

			// Log detailed request information
			m.logger.Info("[MAILGUN] Sending email via Mailgun API",
				"url", url,
				"method", "POST",
				"content_type", contentType,
				"to", req.To,
				"cc_count", len(req.CC),
				"bcc_count", len(req.BCC),
				"subject", req.Subject,
				"has_html", req.HTMLContent != "",
				"has_text", req.TextContent != "",
				"attachments_count", len(req.Attachments),
			)

			startTime := time.Now()
			resp, err := m.httpClient.Do(httpReq)
			duration := time.Since(startTime)
			if err != nil {
				m.logger.Error("[MAILGUN] Network error calling Mailgun API",
					"error", err,
					"url", url,
					"duration_ms", duration.Milliseconds(),
				)
				// Release rate limit slot since send failed
				if m.rateLimiter != nil {
					if releaseErr := m.rateLimiter.Release(ctx, "email"); releaseErr != nil {
						m.logger.Warn("[MAILGUN] Failed to release rate limit after network error", "error", releaseErr)
					}
				}
				return &ProviderError{
					Code:      "NETWORK_ERROR",
					Message:   fmt.Sprintf("Network error: %v", err),
					Provider:  providerName,
					Retryable: true,
				}
			}
			defer func() {
				if closeErr := resp.Body.Close(); closeErr != nil {
					m.logger.Error("[MAILGUN] Error closing response body", "error", closeErr)
				}
			}()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				m.logger.Error("[MAILGUN] Failed to read response body",
					"error", err,
					"status_code", resp.StatusCode,
					"duration_ms", duration.Milliseconds(),
				)
				// Release rate limit slot since send failed
				if m.rateLimiter != nil {
					if releaseErr := m.rateLimiter.Release(ctx, "email"); releaseErr != nil {
						m.logger.Warn("[MAILGUN] Failed to release rate limit after response error", "error", releaseErr)
					}
				}
				return &ProviderError{
					Code:      "RESPONSE_ERROR",
					Message:   fmt.Sprintf("Failed to read response: %v", err),
					Provider:  providerName,
					Retryable: true,
				}
			}

			// Log response details
			m.logger.Info("[MAILGUN] Received response from Mailgun API",
				"status_code", resp.StatusCode,
				"duration_ms", duration.Milliseconds(),
				"response_body", string(body),
				"content_length", len(body),
			)

			if resp.StatusCode >= 400 {
				m.logger.Error("[MAILGUN] Mailgun API returned error status",
					"status_code", resp.StatusCode,
					"response_body", string(body),
					"duration_ms", duration.Milliseconds(),
				)
				// Release rate limit slot since send failed
				if m.rateLimiter != nil {
					if releaseErr := m.rateLimiter.Release(ctx, "email"); releaseErr != nil {
						m.logger.Warn("[MAILGUN] Failed to release rate limit after HTTP error", "error", releaseErr)
					}
				}
				return m.parseErrorResponse(resp.StatusCode, body)
			}

			var mailgunResp MailgunResponse
			if err := json.Unmarshal(body, &mailgunResp); err != nil {
				m.logger.Error("[MAILGUN] Failed to parse Mailgun response",
					"error", err,
					"response_body", string(body),
				)
				return &ProviderError{
					Code:      "PARSE_ERROR",
					Message:   fmt.Sprintf("Failed to parse response: %v", err),
					Provider:  providerName,
					Retryable: false,
				}
			}

			response = &EmailResponse{
				MessageID: mailgunResp.ID,
				Status:    StatusSending,
				Provider:  providerName,
				SentAt:    time.Now(),
				ProviderData: map[string]any{
					"mailgun_id": mailgunResp.ID,
					"message":    mailgunResp.Message,
				},
			}

			m.logger.Info("[MAILGUN] Email sent successfully via Mailgun",
				"message_id", mailgunResp.ID,
				"mailgun_message", mailgunResp.Message,
				"status", StatusSending,
				"duration_ms", duration.Milliseconds(),
				"to", req.To,
			)

			return nil
		})

		return response, err
	*/
}

func (m *MailgunAdapter) GetStatus(ctx context.Context, messageID string) (*DeliveryStatus, error) {
	var status *DeliveryStatus
	err := tracing.TraceEmailProviderCall(ctx, providerName, "status", func(ctx context.Context) error {
		url := fmt.Sprintf("%s/v3/%s/events", m.baseURL, m.domain)
		httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return &ProviderError{
				Code:      "REQUEST_ERROR",
				Message:   fmt.Sprintf("Failed to create request: %v", err),
				Provider:  providerName,
				Retryable: true,
			}
		}

		httpReq.SetBasicAuth("api", m.apiKey)

		q := httpReq.URL.Query()
		q.Add("message-id", messageID)
		httpReq.URL.RawQuery = q.Encode()

		resp, err := m.httpClient.Do(httpReq)
		if err != nil {
			return &ProviderError{
				Code:      "NETWORK_ERROR",
				Message:   fmt.Sprintf("Network error: %v", err),
				Provider:  providerName,
				Retryable: true,
			}
		}
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				m.logger.Error("[MAILGUN] Error closing response body", "error", closeErr)
			}
		}()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return &ProviderError{
				Code:      "RESPONSE_ERROR",
				Message:   fmt.Sprintf("Failed to read response: %v", err),
				Provider:  providerName,
				Retryable: true,
			}
		}

		if resp.StatusCode >= 400 {
			return m.parseErrorResponse(resp.StatusCode, body)
		}

		var eventsResp MailgunEventsResponse
		if err := json.Unmarshal(body, &eventsResp); err != nil {
			return &ProviderError{
				Code:      "PARSE_ERROR",
				Message:   fmt.Sprintf("Failed to parse response: %v", err),
				Provider:  providerName,
				Retryable: false,
			}
		}

		deliveryStatus := m.determineStatusFromEvents(eventsResp.Items)
		status = &deliveryStatus
		return nil
	})

	return status, err
}

func (m *MailgunAdapter) ValidateAddress(ctx context.Context, email string) error {
	return tracing.TraceProviderValidation(ctx, "email", providerName, email, func(ctx context.Context) error {
		// Check for empty email
		email = strings.TrimSpace(email)
		if email == "" {
			return &ProviderError{
				Code:      "INVALID_EMAIL",
				Message:   "Email address cannot be empty",
				Provider:  providerName,
				Retryable: false,
			}
		}

		if !strings.Contains(email, "@") {
			return &ProviderError{
				Code:      "INVALID_EMAIL",
				Message:   "Invalid email format: missing @ symbol",
				Provider:  providerName,
				Retryable: false,
			}
		}

		parts := strings.Split(email, "@")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return &ProviderError{
				Code:      "INVALID_EMAIL",
				Message:   "Invalid email format: invalid structure",
				Provider:  providerName,
				Retryable: false,
			}
		}

		return nil
	})
}

func (m *MailgunAdapter) GetProviderName() string {
	return providerName
}

func (m *MailgunAdapter) IsHealthy(ctx context.Context) error {
	url := fmt.Sprintf("%s/v3/%s/stats/total", m.baseURL, m.domain)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &ProviderError{
			Code:      "REQUEST_ERROR",
			Message:   fmt.Sprintf("Failed to create health check request: %v", err),
			Provider:  providerName,
			Retryable: true,
		}
	}

	httpReq.SetBasicAuth("api", m.apiKey)

	resp, err := m.httpClient.Do(httpReq)
	if err != nil {
		return &ProviderError{
			Code:      "NETWORK_ERROR",
			Message:   fmt.Sprintf("Health check failed: %v", err),
			Provider:  providerName,
			Retryable: true,
		}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			m.logger.Error("[MAILGUN] Error closing response body", "error", closeErr)
		}
	}()

	if resp.StatusCode >= 400 {
		return &ProviderError{
			Code:       "HEALTH_CHECK_FAILED",
			Message:    fmt.Sprintf("Health check returned status %d", resp.StatusCode),
			Provider:   providerName,
			Retryable:  true,
			StatusCode: resp.StatusCode,
		}
	}

	return nil
}

//nolint:unused // Temporarily unused while email sending is blocked - will be used when block is removed
func (m *MailgunAdapter) prepareFormData(req *EmailRequest) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add required fields
	if err := writer.WriteField("from", fmt.Sprintf("noreply@%s", m.domain)); err != nil {
		return nil, "", err
	}
	if err := writer.WriteField("to", req.To); err != nil {
		return nil, "", err
	}
	if err := writer.WriteField("subject", req.Subject); err != nil {
		return nil, "", err
	}

	// Add CC and BCC
	for _, cc := range req.CC {
		if err := writer.WriteField("cc", cc); err != nil {
			return nil, "", err
		}
	}
	for _, bcc := range req.BCC {
		if err := writer.WriteField("bcc", bcc); err != nil {
			return nil, "", err
		}
	}

	// Add content
	if req.HTMLContent != "" {
		if err := writer.WriteField("html", req.HTMLContent); err != nil {
			return nil, "", err
		}
	}
	if req.TextContent != "" {
		if err := writer.WriteField("text", req.TextContent); err != nil {
			return nil, "", err
		}
	}

	// Add attachments
	for _, attachment := range req.Attachments {
		part, err := writer.CreateFormFile("attachment", attachment.Filename)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(attachment.Data); err != nil {
			return nil, "", err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &buf, writer.FormDataContentType(), nil
}

func (m *MailgunAdapter) parseErrorResponse(statusCode int, body []byte) *ProviderError {
	var errorResp MailgunErrorResponse
	if err := json.Unmarshal(body, &errorResp); err != nil {
		// Fallback to generic error based on status code
		return m.createGenericError(statusCode)
	}

	// Map Mailgun-specific error codes to retryable status
	retryable := m.isRetryableError(statusCode, errorResp.Code)

	return &ProviderError{
		Code:       errorResp.Code,
		Message:    errorResp.Message,
		Provider:   providerName,
		Retryable:  retryable,
		StatusCode: statusCode,
	}
}

func (m *MailgunAdapter) createGenericError(statusCode int) *ProviderError {
	var code, message string
	var retryable bool

	switch statusCode {
	case 400:
		code = "BAD_REQUEST"
		message = "Bad Request - Check request parameters"
		retryable = false
	case 401:
		code = "UNAUTHORIZED"
		message = "Unauthorized - Invalid or missing API key"
		retryable = false
	case 403:
		code = "FORBIDDEN"
		message = "Forbidden - Valid credentials but access denied"
		retryable = false
	case 404:
		code = "NOT_FOUND"
		message = "Not Found - Resource not found"
		retryable = false
	case 429:
		code = "RATE_LIMITED"
		message = "Rate Limited - Rate limits exceeded"
		retryable = true
	case 500:
		code = "INTERNAL_ERROR"
		message = "Internal Server Error - Mailgun server error"
		retryable = true
	default:
		code = "UNKNOWN_ERROR"
		message = fmt.Sprintf("Unknown error (status %d)", statusCode)
		retryable = statusCode >= 500
	}

	return &ProviderError{
		Code:       code,
		Message:    message,
		Provider:   providerName,
		Retryable:  retryable,
		StatusCode: statusCode,
	}
}

func (m *MailgunAdapter) isRetryableError(statusCode int, errorCode string) bool {
	// Based on Mailgun documentation, these are retryable scenarios
	switch statusCode {
	case 429: // Rate Limited
		return true
	case 500: // Internal Error
		return true
	case 502, 503, 504: // Gateway errors
		return true
	}

	// Some specific error codes that are retryable
	switch errorCode {
	case "temporary_failure", "timeout", "connection_error":
		return true
	}

	return false
}

func (m *MailgunAdapter) determineStatusFromEvents(events []MailgunEvent) DeliveryStatus {
	if len(events) == 0 {
		return StatusQueued
	}

	latestEvent := events[0]

	switch latestEvent.Event {
	case "delivered":
		return StatusDelivered
	case "failed", "rejected":
		return StatusFailed
	case "bounced":
		return StatusBounced
	case "accepted":
		return StatusSending
	default:
		return StatusSending
	}
}

type MailgunResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

type MailgunErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type MailgunEventsResponse struct {
	Items []MailgunEvent `json:"items"`
}

type MailgunEvent struct {
	Event string `json:"event"`
	Time  int64  `json:"timestamp"`
}

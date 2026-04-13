package hubtel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/randco/randco-microservices/services/service-notification/internal/tracing"
)

const (
	hubtelAPIURL = "https://smsc.hubtel.com/v1/messages"
	providerName = "hubtel"
)

const (
	ErrCodePayloadError      = "PAYLOAD_ERROR"
	ErrCodeRequestError      = "REQUEST_ERROR"
	ErrCodeNetworkError      = "NETWORK_ERROR"
	ErrCodeResponseError     = "RESPONSE_ERROR"
	ErrCodeParseError        = "PARSE_ERROR"
	ErrCodeInvalidPhone      = "INVALID_PHONE"
	ErrCodeHealthCheckFailed = "HEALTH_CHECK_FAILED"
	ErrCodeUnknownError      = "UNKNOWN_ERROR"
)

// Local types mirroring the common provider types to avoid import cycles
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

type SMSRequest struct {
	To        string            `json:"to"`
	Content   string            `json:"content"`
	Variables map[string]string `json:"variables,omitempty"`
	Priority  Priority          `json:"priority"`
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

type SMSProvider interface {
	SendSMS(ctx context.Context, req *SMSRequest) (*SMSResponse, error)
	GetStatus(ctx context.Context, messageID string) (*DeliveryStatus, error)
	ValidateNumber(ctx context.Context, phone string) error
	GetProviderName() string
	IsHealthy(ctx context.Context) error
}

type HubtelAdapter struct {
	clientID     string
	clientSecret string
	baseURL      string
	senderID     string
	httpClient   *http.Client
}

type HubtelConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	BaseURL      string `json:"base_url,omitempty"`
	SenderID     string `json:"sender_id,omitempty"`
}

func NewHubtelAdapter(config HubtelConfig) *HubtelAdapter {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = hubtelAPIURL
	}

	if config.SenderID == "" || config.ClientID == "" || config.ClientSecret == "" {
		slog.Error("sender_id, client_id, and client_secret are required", "sender_id", config.SenderID, "client_id", config.ClientID, "client_secret", config.ClientSecret)
		log.Fatalf("sender_id, client_id, and client_secret are required")
	}

	return &HubtelAdapter{
		clientID:     config.ClientID,
		clientSecret: config.ClientSecret,
		baseURL:      baseURL,
		senderID:     config.SenderID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (h *HubtelAdapter) SendSMS(ctx context.Context, req *SMSRequest) (*SMSResponse, error) {
	var response *SMSResponse
	err := tracing.TraceSMSProviderCall(ctx, providerName, "send", func(ctx context.Context) error {
		return h.sendSMSInternal(ctx, req, &response)
	})

	return response, err
}

func (h *HubtelAdapter) sendSMSInternal(ctx context.Context, req *SMSRequest, response **SMSResponse) error {
	if err := h.ValidateNumber(ctx, req.To); err != nil {
		return err
	}

	payload := map[string]string{
		"from":    h.senderID,
		"to":      req.To,
		"content": req.Content,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return &ProviderError{
			Code:      ErrCodePayloadError,
			Message:   fmt.Sprintf("Failed to marshal request payload: %v", err),
			Provider:  providerName,
			Retryable: false,
		}
	}

	url := h.baseURL + "/send"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return &ProviderError{
			Code:      ErrCodeRequestError,
			Message:   fmt.Sprintf("Failed to create request: %v", err),
			Provider:  providerName,
			Retryable: true,
		}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth(h.clientID, h.clientSecret)

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		return &ProviderError{
			Code:      ErrCodeNetworkError,
			Message:   fmt.Sprintf("Network error: %v", err),
			Provider:  providerName,
			Retryable: true,
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ProviderError{
			Code:      ErrCodeResponseError,
			Message:   fmt.Sprintf("Failed to read response: %v", err),
			Provider:  providerName,
			Retryable: true,
		}
	}

	if resp.StatusCode >= 400 {
		return h.parseErrorResponse(resp.StatusCode, body)
	}

	var hubtelResp HubtelResponse
	if err := json.Unmarshal(body, &hubtelResp); err != nil {
		return &ProviderError{
			Code:      ErrCodeParseError,
			Message:   fmt.Sprintf("Failed to parse response: %v", err),
			Provider:  providerName,
			Retryable: false,
		}
	}

	*response = &SMSResponse{
		MessageID: hubtelResp.MessageID,
		Status:    h.mapHubtelStatus(hubtelResp.Status),
		Provider:  providerName,
		SentAt:    time.Now(),
		ProviderData: map[string]any{
			"hubtel_message_id":  hubtelResp.MessageID,
			"status":             hubtelResp.Status,
			"status_description": hubtelResp.StatusDescription,
			"rate":               hubtelResp.Rate,
			"network_id":         hubtelResp.NetworkID,
		},
	}
	return nil
}

func (h *HubtelAdapter) GetStatus(ctx context.Context, messageID string) (*DeliveryStatus, error) {
	var status *DeliveryStatus
	err := tracing.TraceSMSProviderCall(ctx, providerName, "status", func(ctx context.Context) error {
		url := fmt.Sprintf("%s/%s", h.baseURL, messageID)

		httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return &ProviderError{
				Code:      ErrCodeRequestError,
				Message:   fmt.Sprintf("Failed to create request: %v", err),
				Provider:  providerName,
				Retryable: true,
			}
		}

		// Set Basic Auth header
		httpReq.SetBasicAuth(h.clientID, h.clientSecret)

		resp, err := h.httpClient.Do(httpReq)
		if err != nil {
			return &ProviderError{
				Code:      ErrCodeNetworkError,
				Message:   fmt.Sprintf("Network error: %v", err),
				Provider:  providerName,
				Retryable: true,
			}
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return &ProviderError{
				Code:      ErrCodeResponseError,
				Message:   fmt.Sprintf("Failed to read response: %v", err),
				Provider:  providerName,
				Retryable: true,
			}
		}

		if resp.StatusCode >= 400 {
			return h.parseErrorResponse(resp.StatusCode, body)
		}

		var statusResp HubtelStatusResponse
		if err := json.Unmarshal(body, &statusResp); err != nil {
			return &ProviderError{
				Code:      ErrCodeParseError,
				Message:   fmt.Sprintf("Failed to parse response: %v", err),
				Provider:  providerName,
				Retryable: false,
			}
		}

		deliveryStatus := h.mapHubtelStatusString(statusResp.Status)
		status = &deliveryStatus
		return nil
	})

	return status, err
}

func (h *HubtelAdapter) ValidateNumber(ctx context.Context, phone string) error {
	return tracing.TraceProviderValidation(ctx, "sms", providerName, phone, func(ctx context.Context) error {
		cleanPhone := regexp.MustCompile(`[^\d+]`).ReplaceAllString(phone, "")

		if strings.HasPrefix(cleanPhone, "+") {
			if len(cleanPhone) < 11 {
				return &ProviderError{
					Code:      ErrCodeInvalidPhone,
					Message:   "Invalid phone number: too short for international format",
					Provider:  providerName,
					Retryable: false,
				}
			}
		} else {
			// Local format - should be at least 9 digits
			if len(cleanPhone) < 9 {
				return &ProviderError{
					Code:      ErrCodeInvalidPhone,
					Message:   "Invalid phone number: too short for local format",
					Provider:  providerName,
					Retryable: false,
				}
			}
		}

		if !regexp.MustCompile(`^\+?[0-9]+$`).MatchString(cleanPhone) {
			return &ProviderError{
				Code:      ErrCodeInvalidPhone,
				Message:   "Invalid phone number: contains invalid characters",
				Provider:  providerName,
				Retryable: false,
			}
		}

		return nil
	})
}

func (h *HubtelAdapter) GetProviderName() string {
	return providerName
}

func (h *HubtelAdapter) IsHealthy(ctx context.Context) error {
	// Simple health check - try to make a request to the base URL
	// We'll use a minimal test request
	testURL := h.baseURL + "?clientid=test&clientsecret=test&from=test&to=+1234567890&content=test"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return &ProviderError{
			Code:      ErrCodeRequestError,
			Message:   fmt.Sprintf("Failed to create health check request: %v", err),
			Provider:  providerName,
			Retryable: true,
		}
	}

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		return &ProviderError{
			Code:      ErrCodeNetworkError,
			Message:   fmt.Sprintf("Health check failed: %v", err),
			Provider:  providerName,
			Retryable: true,
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Accept any response as healthy (even 400/401 for invalid credentials)
	// as long as we can reach the server
	if resp.StatusCode >= 500 {
		return &ProviderError{
			Code:       ErrCodeHealthCheckFailed,
			Message:    fmt.Sprintf("Health check returned status %d", resp.StatusCode),
			Provider:   providerName,
			Retryable:  true,
			StatusCode: resp.StatusCode,
		}
	}

	return nil
}

func (h *HubtelAdapter) parseErrorResponse(statusCode int, body []byte) *ProviderError {
	var errorResp HubtelErrorResponse
	if err := json.Unmarshal(body, &errorResp); err != nil {
		return &ProviderError{
			Code:       ErrCodeUnknownError,
			Message:    fmt.Sprintf("Unknown error (status %d)", statusCode),
			Provider:   providerName,
			Retryable:  statusCode >= 500,
			StatusCode: statusCode,
		}
	}

	return &ProviderError{
		Code:       errorResp.Code,
		Message:    errorResp.Message,
		Provider:   providerName,
		Retryable:  statusCode >= 500,
		StatusCode: statusCode,
	}
}

func (h *HubtelAdapter) mapHubtelStatus(hubtelStatus int) DeliveryStatus {
	switch hubtelStatus {
	case 0:
		return StatusQueued // Message accepted
	case 1:
		return StatusSending // Message being processed
	case 2:
		return StatusDelivered // Message delivered
	case 3:
		return StatusFailed // Message failed
	case 4:
		return StatusRejected // Message rejected
	default:
		return StatusSending // Unknown status, assume sending
	}
}

func (h *HubtelAdapter) mapHubtelStatusString(hubtelStatus string) DeliveryStatus {
	switch strings.ToLower(hubtelStatus) {
	case "delivered":
		return StatusDelivered
	case "sent":
		return StatusSending
	case "pending":
		return StatusQueued
	case "blacklisted", "undeliverable", "failed", "unrouteable", "error", "rejected":
		return StatusFailed
	case "nack/0x0000000b", "invalid destination address":
		return StatusFailed // Invalid recipient number
	case "nack/0x0000000a", "invalid source address":
		return StatusFailed // Invalid sender ID
	default:
		return StatusSending // Unknown status, assume sending
	}
}

type HubtelResponse struct {
	Rate              float64 `json:"rate"`
	MessageID         string  `json:"messageId"`
	Status            int     `json:"status"`
	StatusDescription *string `json:"statusDescription"`
	NetworkID         string  `json:"networkId"`
}

type HubtelStatusResponse struct {
	Rate            float64 `json:"rate"`
	Units           int     `json:"units"`
	MessageID       string  `json:"messageId"`
	Content         string  `json:"content"`
	Status          string  `json:"status"`
	ClientReference *string `json:"clientReference"`
	NetworkID       *string `json:"networkId"`
	UpdateTime      string  `json:"updateTime"`
	Time            string  `json:"time"`
	To              string  `json:"to"`
	From            string  `json:"from"`
}

type HubtelErrorResponse struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

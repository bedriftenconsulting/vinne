package hubtel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHubtelAdapter(t *testing.T) {
	tests := []struct {
		name   string
		config HubtelConfig
		want   *HubtelAdapter
	}{
		{
			name: "default configuration",
			config: HubtelConfig{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
			want: &HubtelAdapter{
				clientID:     "test-client-id",
				clientSecret: "test-client-secret",
				baseURL:      hubtelAPIURL,
				senderID:     "",
			},
		},
		{
			name: "custom configuration",
			config: HubtelConfig{
				ClientID:     "custom-client-id",
				ClientSecret: "custom-client-secret",
				BaseURL:      "https://custom.hubtel.com/v1/messages/send",
				SenderID:     "CustomSender",
			},
			want: &HubtelAdapter{
				clientID:     "custom-client-id",
				clientSecret: "custom-client-secret",
				baseURL:      "https://custom.hubtel.com/v1/messages/send",
				senderID:     "CustomSender",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewHubtelAdapter(tt.config)

			assert.Equal(t, tt.want.clientID, adapter.clientID)
			assert.Equal(t, tt.want.clientSecret, adapter.clientSecret)
			assert.Equal(t, tt.want.baseURL, adapter.baseURL)
			assert.Equal(t, tt.want.senderID, adapter.senderID)
			assert.NotNil(t, adapter.httpClient)
			assert.Equal(t, 30*time.Second, adapter.httpClient.Timeout)
		})
	}
}

func TestHubtelAdapter_GetStatus(t *testing.T) {
	t.Run("delivered status", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request method and URL
			assert.Equal(t, "GET", r.Method)
			assert.True(t, strings.Contains(r.URL.Path, "/v1/messages/c9d729d6-a802-425e-9862-9fe4c0f09d63"))

			// Verify Basic Auth
			username, password, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "test-client-id", username)
			assert.Equal(t, "test-client-secret", password)

			// Set response
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(HubtelStatusResponse{
				Rate:            0.03,
				Units:           0,
				MessageID:       "c9d729d6-a802-425e-9862-9fe4c0f09d63",
				Content:         "Hello World",
				Status:          "Delivered",
				ClientReference: nil,
				NetworkID:       nil,
				UpdateTime:      "2025-04-15T14:09:03",
				Time:            "2025-04-15T14:08:56.2711269Z",
				To:              "+233200585542",
				From:            "RSETest",
			})
		}))
		defer server.Close()

		// Create adapter with test server URL
		adapter := &HubtelAdapter{
			clientID:     "test-client-id",
			clientSecret: "test-client-secret",
			baseURL:      server.URL + "/v1/messages/send",
			httpClient:   &http.Client{Timeout: 5 * time.Second},
		}

		// Test GetStatus
		ctx := context.Background()
		status, err := adapter.GetStatus(ctx, "c9d729d6-a802-425e-9862-9fe4c0f09d63")

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, StatusDelivered, *status)
	})

	t.Run("server error", func(t *testing.T) {
		// Create test server that returns error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "server error"})
		}))
		defer server.Close()

		adapter := &HubtelAdapter{
			clientID:     "test-client-id",
			clientSecret: "test-client-secret",
			baseURL:      server.URL + "/v1/messages/send",
			httpClient:   &http.Client{Timeout: 5 * time.Second},
		}

		ctx := context.Background()
		status, err := adapter.GetStatus(ctx, "test-message-id")

		assert.Error(t, err)
		providerErr, ok := err.(*ProviderError)
		require.True(t, ok)
		assert.Equal(t, ErrCodeUnknownError, providerErr.Code)
		assert.Nil(t, status)
	})
}

func TestHubtelAdapter_ValidateNumber(t *testing.T) {
	tests := []struct {
		name        string
		phone       string
		expectError bool
		errorCode   string
	}{
		{
			name:        "valid international number",
			phone:       "+233200585542",
			expectError: false,
		},
		{
			name:        "valid local number",
			phone:       "0200585542",
			expectError: false,
		},
		{
			name:        "valid number with spaces",
			phone:       "+233 20 058 5542",
			expectError: false,
		},
		{
			name:        "valid number with dashes",
			phone:       "+233-20-058-5542",
			expectError: false,
		},
		{
			name:        "too short international",
			phone:       "+123456789",
			expectError: true,
			errorCode:   ErrCodeInvalidPhone,
		},
		{
			name:        "too short local",
			phone:       "12345678",
			expectError: true,
			errorCode:   ErrCodeInvalidPhone,
		},
		{
			name:        "invalid characters",
			phone:       "+233200585542abc",
			expectError: true,
			errorCode:   ErrCodeInvalidPhone,
		},
		{
			name:        "empty phone",
			phone:       "",
			expectError: true,
			errorCode:   ErrCodeInvalidPhone,
		},
	}

	adapter := &HubtelAdapter{
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
		baseURL:      hubtelAPIURL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := adapter.ValidateNumber(ctx, tt.phone)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != "" {
					providerErr, ok := err.(*ProviderError)
					require.True(t, ok)
					assert.Equal(t, tt.errorCode, providerErr.Code)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHubtelAdapter_IsHealthy(t *testing.T) {
	t.Run("healthy server", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request method and URL
			assert.Equal(t, "GET", r.Method)
			assert.True(t, strings.Contains(r.URL.Path, "/v1/messages/send"))

			// Set response
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Create adapter with test server URL
		adapter := &HubtelAdapter{
			clientID:     "test-client-id",
			clientSecret: "test-client-secret",
			baseURL:      server.URL + "/v1/messages/send",
			httpClient:   &http.Client{Timeout: 5 * time.Second},
		}

		// Test IsHealthy
		ctx := context.Background()
		err := adapter.IsHealthy(ctx)

		assert.NoError(t, err)
	})

	t.Run("server error", func(t *testing.T) {
		// Create test server that returns error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		adapter := &HubtelAdapter{
			clientID:     "test-client-id",
			clientSecret: "test-client-secret",
			baseURL:      server.URL + "/v1/messages/send",
			httpClient:   &http.Client{Timeout: 5 * time.Second},
		}

		ctx := context.Background()
		err := adapter.IsHealthy(ctx)

		assert.Error(t, err)
		providerErr, ok := err.(*ProviderError)
		require.True(t, ok)
		assert.Equal(t, ErrCodeHealthCheckFailed, providerErr.Code)
	})
}

func TestHubtelAdapter_GetProviderName(t *testing.T) {
	adapter := &HubtelAdapter{}
	assert.Equal(t, providerName, adapter.GetProviderName())
}

func TestHubtelAdapter_mapHubtelStatus(t *testing.T) {
	tests := []struct {
		name         string
		hubtelStatus int
		expected     DeliveryStatus
	}{
		{"status 0 - queued", 0, StatusQueued},
		{"status 1 - sending", 1, StatusSending},
		{"status 2 - delivered", 2, StatusDelivered},
		{"status 3 - failed", 3, StatusFailed},
		{"status 4 - rejected", 4, StatusRejected},
		{"unknown status", 99, StatusSending},
	}

	adapter := &HubtelAdapter{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.mapHubtelStatus(tt.hubtelStatus)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHubtelAdapter_mapHubtelStatusString(t *testing.T) {
	tests := []struct {
		name         string
		hubtelStatus string
		expected     DeliveryStatus
	}{
		{"delivered", "Delivered", StatusDelivered},
		{"delivered lowercase", "delivered", StatusDelivered},
		{"sent", "Sent", StatusSending},
		{"sent lowercase", "sent", StatusSending},
		{"pending", "Pending", StatusQueued},
		{"pending lowercase", "pending", StatusQueued},
		{"blacklisted", "Blacklisted", StatusFailed},
		{"blacklisted lowercase", "blacklisted", StatusFailed},
		{"failed", "Failed", StatusFailed},
		{"failed lowercase", "failed", StatusFailed},
		{"undeliverable", "Undeliverable", StatusFailed},
		{"unrouteable", "Unrouteable", StatusFailed},
		{"error", "Error", StatusFailed},
		{"rejected", "Rejected", StatusFailed},
		{"nack invalid destination", "NACK/0x0000000b", StatusFailed},
		{"invalid destination address", "Invalid Destination Address", StatusFailed},
		{"nack invalid source", "NACK/0x0000000a", StatusFailed},
		{"invalid source address", "Invalid Source Address", StatusFailed},
		{"unknown status", "UnknownStatus", StatusSending},
		{"empty status", "", StatusSending},
	}

	adapter := &HubtelAdapter{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.mapHubtelStatusString(tt.hubtelStatus)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHubtelAdapter_parseErrorResponse(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody []byte
		expectedCode string
		expectedMsg  string
		retryable    bool
	}{
		{
			name:         "valid error response",
			statusCode:   http.StatusBadRequest,
			responseBody: []byte(`{"Code": "INVALID_REQUEST", "Message": "Invalid request parameters"}`),
			expectedCode: "INVALID_REQUEST",
			expectedMsg:  "Invalid request parameters",
			retryable:    false,
		},
		{
			name:         "server error response",
			statusCode:   http.StatusInternalServerError,
			responseBody: []byte(`{"Code": "SERVER_ERROR", "Message": "Internal server error"}`),
			expectedCode: "SERVER_ERROR",
			expectedMsg:  "Internal server error",
			retryable:    true,
		},
		{
			name:         "invalid JSON response",
			statusCode:   http.StatusBadRequest,
			responseBody: []byte(`invalid json`),
			expectedCode: ErrCodeUnknownError,
			expectedMsg:  "Unknown error (status 400)",
			retryable:    false,
		},
	}

	adapter := &HubtelAdapter{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.parseErrorResponse(tt.statusCode, tt.responseBody)

			assert.Equal(t, tt.expectedCode, err.Code)
			assert.Equal(t, tt.expectedMsg, err.Message)
			assert.Equal(t, providerName, err.Provider)
			assert.Equal(t, tt.retryable, err.Retryable)
			assert.Equal(t, tt.statusCode, err.StatusCode)
		})
	}
}

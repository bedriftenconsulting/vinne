package orange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/randco/randco-microservices/shared/common/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("payment-service/orange")

// HTTPClient wraps http.Client with retry logic
type HTTPClient struct {
	client        *http.Client
	retryAttempts int
	retryDelay    time.Duration
	log           logger.Logger
}

// NewHTTPClient creates a new HTTP client with retry configuration
func NewHTTPClient(timeout time.Duration, retryAttempts int, retryDelay time.Duration) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
		retryAttempts: retryAttempts,
		retryDelay:    retryDelay,
		log:           nil, // Will be set by provider
	}
}

// SetLogger sets the logger for the HTTP client
func (c *HTTPClient) SetLogger(log logger.Logger) {
	c.log = log
}

// HTTPRequest represents a request to be made
type HTTPRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    interface{}
}

// HTTPError represents an HTTP error response
type HTTPError struct {
	StatusCode int
	Message    string
	Body       []byte
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// Do executes an HTTP request with retry logic
func (c *HTTPClient) Do(ctx context.Context, req *HTTPRequest, result interface{}) error {
	ctx, span := tracer.Start(ctx, "orange.http_client.do",
		trace.WithAttributes(
			attribute.String("method", req.Method),
			attribute.String("url", req.URL),
			attribute.Int("retry_attempts", c.retryAttempts),
		))
	defer span.End()

	var lastErr error

	for attempt := 0; attempt <= c.retryAttempts; attempt++ {
		if attempt > 0 {
			// Add span event for retry
			span.AddEvent("retrying request",
				trace.WithAttributes(
					attribute.Int("attempt", attempt),
					attribute.String("last_error", lastErr.Error()),
				))

			// Wait before retrying
			select {
			case <-ctx.Done():
				span.RecordError(ctx.Err())
				return ctx.Err()
			case <-time.After(c.retryDelay):
			}
		}

		span.SetAttributes(attribute.Int("attempt", attempt))

		err := c.doRequest(ctx, req, result)
		if err == nil {
			span.SetAttributes(attribute.Bool("success", true))
			return nil
		}

		lastErr = err

		// Don't retry on client errors (4xx), duplicate reference errors, or context cancellation
		if httpErr, ok := err.(*HTTPError); ok {
			// Don't retry on 4xx client errors
			if isClientError(httpErr.StatusCode) {
				span.RecordError(err)
				span.SetAttributes(
					attribute.Bool("retryable", false),
					attribute.String("reason", "client_error"),
				)
				return err
			}

			// Don't retry on duplicate reference errors (idempotency violation)
			// This means the request was already accepted/processed by the provider
			if httpErr.StatusCode == 500 && isDuplicateReferenceError(string(httpErr.Body)) {
				span.RecordError(err)
				span.SetAttributes(
					attribute.Bool("retryable", false),
					attribute.String("reason", "duplicate_reference"),
				)
				if c.log != nil {
					c.log.Warn("Duplicate reference detected, not retrying",
						"status_code", httpErr.StatusCode,
						"message", httpErr.Message)
				}
				return err
			}
		}

		if ctx.Err() != nil {
			span.RecordError(ctx.Err())
			return ctx.Err()
		}

		// Last attempt failed, don't wait
		if attempt == c.retryAttempts {
			span.RecordError(lastErr)
			span.SetAttributes(attribute.Bool("success", false))
			break
		}
	}

	return fmt.Errorf("request failed after %d attempts: %w", c.retryAttempts+1, lastErr)
}

// doRequest executes a single HTTP request
func (c *HTTPClient) doRequest(ctx context.Context, req *HTTPRequest, result interface{}) error {
	ctx, span := tracer.Start(ctx, "orange.http_client.do_request")
	defer span.End()

	// Marshal request body if present
	var bodyReader io.Reader
	var requestBodyBytes []byte
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			span.RecordError(err)
			if c.log != nil {
				c.log.Error("Failed to marshal Orange API request body",
					"error", err,
					"url", req.URL,
					"method", req.Method)
			}
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
		requestBodyBytes = bodyBytes
		span.SetAttributes(attribute.Int("request_body_size", len(bodyBytes)))
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bodyReader)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set default content type if body is present
	if req.Body != nil && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// Log request details
	if c.log != nil {
		sanitizedHeaders := sanitizeHeaders(req.Headers)
		c.log.Info("Sending request to Orange API",
			"method", req.Method,
			"url", req.URL,
			"headers", sanitizedHeaders,
			"request_body", string(requestBodyBytes))
	}

	// Execute request
	startTime := time.Now()
	resp, err := c.client.Do(httpReq)
	duration := time.Since(startTime)

	span.SetAttributes(
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			span.RecordError(err)
		}
	}()

	span.SetAttributes(attribute.Int("status_code", resp.StatusCode))

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to read response body: %w", err)
	}

	span.SetAttributes(attribute.Int("response_body_size", len(body)))

	// Log response details
	if c.log != nil {
		c.log.Info("Received response from Orange API",
			"method", req.Method,
			"url", req.URL,
			"status_code", resp.StatusCode,
			"duration_ms", duration.Milliseconds(),
			"response_body", string(body))
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		httpErr := &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    http.StatusText(resp.StatusCode),
			Body:       body,
		}

		// Try to extract error message from body
		var errResp struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil {
			if errResp.Error != "" {
				httpErr.Message = errResp.Error
			} else if errResp.Message != "" {
				httpErr.Message = errResp.Message
			}
		}

		span.RecordError(httpErr)
		if c.log != nil {
			c.log.Error("Orange API returned error response",
				"status_code", resp.StatusCode,
				"error_message", httpErr.Message,
				"response_body", string(body),
				"url", req.URL,
				"method", req.Method)
		}
		return httpErr
	}

	// Unmarshal response body into result
	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			span.RecordError(err)
			if c.log != nil {
				c.log.Error("Failed to unmarshal Orange API response",
					"error", err,
					"response_body", string(body),
					"url", req.URL,
					"method", req.Method)
			}
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// isClientError checks if a status code is a client error (4xx)
func isClientError(statusCode int) bool {
	return statusCode >= 400 && statusCode < 500
}

// isDuplicateReferenceError checks if the error body indicates a duplicate reference
func isDuplicateReferenceError(body string) bool {
	return strings.Contains(strings.ToLower(body), "duplicate reference") ||
		strings.Contains(strings.ToLower(body), "duplicate_reference")
}

// sanitizeHeaders removes sensitive information from headers for logging
func sanitizeHeaders(headers map[string]string) map[string]string {
	sanitized := make(map[string]string)
	for key, value := range headers {
		lowerKey := strings.ToLower(key)
		if lowerKey == "authorization" || lowerKey == "api-key" || lowerKey == "api_key" {
			// Show only first/last 4 chars of token for debugging
			if len(value) > 12 {
				sanitized[key] = value[:6] + "..." + value[len(value)-4:]
			} else {
				sanitized[key] = "***REDACTED***"
			}
		} else {
			sanitized[key] = value
		}
	}
	return sanitized
}

package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	paymentv1 "github.com/randco/randco-microservices/proto/payment/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/response"
	"github.com/randco/randco-microservices/shared/common/logger"
)

// WebhookHandler handles webhook callbacks from payment providers
type WebhookHandler struct {
	grpcManager *grpc.ClientManager
	logger      logger.Logger
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(grpcManager *grpc.ClientManager, logger logger.Logger) *WebhookHandler {
	return &WebhookHandler{
		grpcManager: grpcManager,
		logger:      logger,
	}
}

// HandleOrangeWebhook handles webhook callbacks from Orange Money API
// POST /api/v1/webhooks/orange
func (h *WebhookHandler) HandleOrangeWebhook(w http.ResponseWriter, r *http.Request) error {
	h.logger.Info("Received Orange webhook",
		"method", r.Method,
		"content_type", r.Header.Get("Content-Type"),
		"remote_addr", getClientIP(r))

	// Validate Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json; charset=utf-8" {
		h.logger.Warn("Invalid content type for webhook",
			"content_type", contentType,
			"remote_addr", getClientIP(r))
		// Continue anyway - some providers send without proper content-type header
	}

	// Read raw body (preserve it for signature verification)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read webhook body", "error", err)
		return response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to read request body", nil)
	}

	// Validate body size (max 1MB for webhooks)
	if len(body) > 1024*1024 {
		h.logger.Error("Webhook body too large",
			"body_size", len(body),
			"remote_addr", getClientIP(r))
		return response.Error(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large", nil)
	}

	// Log summary only (not full body - security/GDPR compliance)
	h.logger.Info("Webhook body received",
		"body_size", len(body),
		"remote_addr", getClientIP(r))

	// Extract headers that might be needed for signature verification
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	h.logger.Debug("Webhook headers extracted",
		"header_count", len(headers),
		"has_signature", headers["X-Orange-Signature"] != "")

	// Create context with timeout (30 seconds - webhooks should respond quickly)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	h.logger.Debug("Forwarding webhook to Payment Service via gRPC",
		"provider", "ORANGE",
		"body_size", len(body))

	// Get payment service connection
	conn, err := h.grpcManager.GetConnection("payment")
	if err != nil {
		h.logger.Error("Failed to get payment service connection",
			"error", err)
		return response.ServiceUnavailableError(w, "Payment")
	}

	paymentClient := paymentv1.NewPaymentServiceClient(conn)

	// Forward to Payment Service via gRPC with timeout
	// IMPORTANT: Provider name must be UPPERCASE when sent to Orange
	resp, err := paymentClient.ProcessWebhook(ctx, &paymentv1.ProcessWebhookRequest{
		ProviderName: "ORANGE", // Uppercase for Orange API
		EventType:    "payment.status",
		Payload:      body,
		Headers:      headers,
	})

	if err != nil {
		h.logger.Error("Failed to process webhook via gRPC",
			"error", err,
			"error_type", fmt.Sprintf("%T", err),
			"body_size", len(body),
			"remote_addr", getClientIP(r))

		// Check if timeout
		if ctx.Err() == context.DeadlineExceeded {
			h.logger.Error("Webhook processing timeout",
				"timeout", "30s",
				"provider", "ORANGE")
			return response.TimeoutError(w, "Webhook processing timeout")
		}

		return response.Error(w, http.StatusInternalServerError, "PROCESSING_FAILED", "Failed to process webhook", map[string]string{
			"error": err.Error(),
		})
	}

	h.logger.Info("Webhook processed successfully",
		"success", resp.Success,
		"message", resp.Message)

	// Return success to Orange
	return response.Success(w, http.StatusOK, resp.Message, map[string]bool{
		"success": resp.Success,
	})
}

// HandleMTNWebhook handles webhook callbacks from MTN Mobile Money
// POST /api/v1/webhooks/mtn
func (h *WebhookHandler) HandleMTNWebhook(w http.ResponseWriter, r *http.Request) error {
	h.logger.Info("Received MTN webhook",
		"method", r.Method,
		"content_type", r.Header.Get("Content-Type"),
		"remote_addr", getClientIP(r))

	// Validate Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json; charset=utf-8" {
		h.logger.Warn("Invalid content type for webhook",
			"content_type", contentType,
			"remote_addr", getClientIP(r))
		// Continue anyway - some providers send without proper content-type header
	}

	// Read raw body (preserve it for signature verification)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read webhook body", "error", err)
		return response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to read request body", nil)
	}

	// Validate body size (max 1MB for webhooks)
	if len(body) > 1024*1024 {
		h.logger.Error("Webhook body too large",
			"body_size", len(body),
			"remote_addr", getClientIP(r))
		return response.Error(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large", nil)
	}

	// Log summary only (not full body - security/GDPR compliance)
	h.logger.Info("Webhook body received",
		"body_size", len(body),
		"remote_addr", getClientIP(r))

	// Extract headers that might be needed for signature verification
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	h.logger.Debug("Webhook headers extracted",
		"header_count", len(headers))

	// Create context with timeout (30 seconds - webhooks should respond quickly)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	h.logger.Debug("Forwarding webhook to Payment Service via gRPC",
		"provider", "MTN",
		"body_size", len(body))

	// Get payment service connection
	conn, err := h.grpcManager.GetConnection("payment")
	if err != nil {
		h.logger.Error("Failed to get payment service connection",
			"error", err)
		return response.ServiceUnavailableError(w, "Payment")
	}

	paymentClient := paymentv1.NewPaymentServiceClient(conn)

	// Forward to Payment Service via gRPC with timeout
	// IMPORTANT: Provider name must be UPPERCASE when sent to Orange
	resp, err := paymentClient.ProcessWebhook(ctx, &paymentv1.ProcessWebhookRequest{
		ProviderName: "MTN", // Uppercase for Orange API
		EventType:    "payment.status",
		Payload:      body,
		Headers:      headers,
	})

	if err != nil {
		h.logger.Error("Failed to process webhook via gRPC",
			"error", err,
			"error_type", fmt.Sprintf("%T", err),
			"body_size", len(body),
			"remote_addr", getClientIP(r))

		// Check if timeout
		if ctx.Err() == context.DeadlineExceeded {
			h.logger.Error("Webhook processing timeout",
				"timeout", "30s",
				"provider", "MTN")
			return response.TimeoutError(w, "Webhook processing timeout")
		}

		return response.Error(w, http.StatusInternalServerError, "PROCESSING_FAILED", "Failed to process webhook", map[string]string{
			"error": err.Error(),
		})
	}

	h.logger.Info("Webhook processed successfully",
		"success", resp.Success,
		"message", resp.Message)

	// Return success to MTN
	return response.Success(w, http.StatusOK, resp.Message, map[string]bool{
		"success": resp.Success,
	})
}

// HandleTelecelWebhook handles webhook callbacks from Telecel Mobile Money
// POST /api/v1/webhooks/telecel
func (h *WebhookHandler) HandleTelecelWebhook(w http.ResponseWriter, r *http.Request) error {
	h.logger.Info("Received Telecel webhook",
		"method", r.Method,
		"content_type", r.Header.Get("Content-Type"),
		"remote_addr", getClientIP(r))

	// Validate Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json; charset=utf-8" {
		h.logger.Warn("Invalid content type for webhook",
			"content_type", contentType,
			"remote_addr", getClientIP(r))
		// Continue anyway - some providers send without proper content-type header
	}

	// Read raw body (preserve it for signature verification)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read webhook body", "error", err)
		return response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to read request body", nil)
	}

	// Validate body size (max 1MB for webhooks)
	if len(body) > 1024*1024 {
		h.logger.Error("Webhook body too large",
			"body_size", len(body),
			"remote_addr", getClientIP(r))
		return response.Error(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large", nil)
	}

	// Log summary only (not full body - security/GDPR compliance)
	h.logger.Info("Webhook body received",
		"body_size", len(body),
		"remote_addr", getClientIP(r))

	// Extract headers that might be needed for signature verification
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	h.logger.Debug("Webhook headers extracted",
		"header_count", len(headers))

	// Create context with timeout (30 seconds - webhooks should respond quickly)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	h.logger.Debug("Forwarding webhook to Payment Service via gRPC",
		"provider", "TELECEL",
		"body_size", len(body))

	// Get payment service connection
	conn, err := h.grpcManager.GetConnection("payment")
	if err != nil {
		h.logger.Error("Failed to get payment service connection",
			"error", err)
		return response.ServiceUnavailableError(w, "Payment")
	}

	paymentClient := paymentv1.NewPaymentServiceClient(conn)

	// Forward to Payment Service via gRPC with timeout
	// IMPORTANT: Provider name must be UPPERCASE when sent to Orange
	resp, err := paymentClient.ProcessWebhook(ctx, &paymentv1.ProcessWebhookRequest{
		ProviderName: "TELECEL", // Uppercase for Orange API
		EventType:    "payment.status",
		Payload:      body,
		Headers:      headers,
	})

	if err != nil {
		h.logger.Error("Failed to process webhook via gRPC",
			"error", err,
			"error_type", fmt.Sprintf("%T", err),
			"body_size", len(body),
			"remote_addr", getClientIP(r))

		// Check if timeout
		if ctx.Err() == context.DeadlineExceeded {
			h.logger.Error("Webhook processing timeout",
				"timeout", "30s",
				"provider", "TELECEL")
			return response.TimeoutError(w, "Webhook processing timeout")
		}

		return response.Error(w, http.StatusInternalServerError, "PROCESSING_FAILED", "Failed to process webhook", map[string]string{
			"error": err.Error(),
		})
	}

	h.logger.Info("Webhook processed successfully",
		"success", resp.Success,
		"message", resp.Message)

	// Return success to Telecel
	return response.Success(w, http.StatusOK, resp.Message, map[string]bool{
		"success": resp.Success,
	})
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fallback to RemoteAddr
	return r.RemoteAddr
}

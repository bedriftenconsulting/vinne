package models

import "time"

// OrangeWebhookPayload represents the webhook payload from Orange Extensibility API
// Based on Orange API documentation for transaction status callbacks
type OrangeWebhookPayload struct {
	Status  int                    `json:"status"`  // 1=success, 0=failed, 3=pending, 2=duplicate
	Message string                 `json:"message"` // Status message
	Data    OrangeWebhookData      `json:"data"`
	Extra   map[string]interface{} `json:"extra,omitempty"` // Additional provider-specific data
}

// OrangeWebhookData represents the data section of Orange webhook
type OrangeWebhookData struct {
	TransactionID string     `json:"transactionId"` // Orange's transaction ID
	Reference     string     `json:"reference"`     // Our transaction reference
	Beneficiary   string     `json:"beneficiary"`   // Wallet number or beneficiary identifier
	Amount        float64    `json:"amount"`        // Amount in cedis
	Currency      string     `json:"currency"`      // GHS
	RequestDate   *time.Time `json:"requestDate"`   // When transaction was initiated
	ApproveDate   *time.Time `json:"approveDate"`   // When user approved (null if still pending)
}

// WebhookEvent represents a processed webhook event stored in the database
type WebhookEvent struct {
	ID                string                 `json:"id" db:"id"`
	Provider          string                 `json:"provider" db:"provider"`                     // orange, mtn, etc.
	EventType         string                 `json:"event_type" db:"event_type"`                 // payment.success, payment.failed, etc.
	TransactionID     *string                `json:"transaction_id" db:"transaction_id"`         // Our transaction UUID (nullable)
	Reference         string                 `json:"reference" db:"reference"`                   // Transaction reference
	Status            string                 `json:"status" db:"status"`                         // processed, failed, duplicate
	RawPayload        map[string]interface{} `json:"raw_payload" db:"raw_payload"`               // Raw webhook payload (JSONB)
	ProcessedAt       *time.Time             `json:"processed_at" db:"processed_at"`             // When webhook was successfully processed
	ErrorMessage      string                 `json:"error_message" db:"error_message"`           // Error if processing failed
	SignatureVerified bool                   `json:"signature_verified" db:"signature_verified"` // Whether signature was valid
	ReceivedAt        time.Time              `json:"received_at" db:"received_at"`               // When webhook was received
	CreatedAt         time.Time              `json:"created_at" db:"created_at"`
}

// WebhookEventStatus represents the status of webhook processing
type WebhookEventStatus string

const (
	WebhookStatusPending   WebhookEventStatus = "pending"   // Received but not yet processed
	WebhookStatusProcessed WebhookEventStatus = "processed" // Successfully processed
	WebhookStatusFailed    WebhookEventStatus = "failed"    // Failed to process
	WebhookStatusDuplicate WebhookEventStatus = "duplicate" // Duplicate webhook (already processed)
)

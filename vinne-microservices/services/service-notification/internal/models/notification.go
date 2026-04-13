package models

import "time"

type NotificationType string

const (
	NotificationTypeEmail NotificationType = "email"
	NotificationTypeSMS   NotificationType = "sms"
	NotificationTypePush  NotificationType = "push"
)

type NotificationStatus string

const (
	NotificationStatusQueued    NotificationStatus = "queued"
	NotificationStatusSent      NotificationStatus = "sent"
	NotificationStatusDelivered NotificationStatus = "delivered"
	NotificationStatusFailed    NotificationStatus = "failed"
	NotificationStatusBounced   NotificationStatus = "bounced"
)

type EventType string

const (
	EventTypeQueued    EventType = "queued"
	EventTypeSent      EventType = "sent"
	EventTypeDelivered EventType = "delivered"
	EventTypeFailed    EventType = "failed"
	EventTypeBounced   EventType = "bounced"
	EventTypeClicked   EventType = "clicked"
)

type RecipientType string

const (
	RecipientTypeTo  RecipientType = "to"
	RecipientTypeCC  RecipientType = "cc"
	RecipientTypeBCC RecipientType = "bcc"
)

type Notification struct {
	ID                string             `db:"id" json:"id"`
	IdempotencyKey    string             `db:"idempotency_key" json:"idempotency_key,omitempty"`
	Type              NotificationType   `db:"type" json:"type"`
	Recipient         []Recipient        `db:"recipient" json:"recipient"`
	CC                []Recipient        `db:"cc" json:"cc,omitempty"`
	BCC               []Recipient        `db:"bcc" json:"bcc,omitempty"`
	Subject           string             `db:"subject" json:"subject,omitempty"`
	Content           string             `db:"content" json:"content"`
	Status            NotificationStatus `db:"status" json:"status"`
	Provider          string             `db:"provider" json:"provider,omitempty"`
	ProviderMessageID *string            `db:"provider_message_id" json:"provider_message_id,omitempty"`
	ProviderResponse  any                `db:"provider_response" json:"provider_response,omitempty"` // JSONB
	RetryCount        int8               `db:"retry_count" json:"retry_count"`
	ScheduledFor      *time.Time         `db:"scheduled_for" json:"scheduled_for,omitempty"`
	TemplateID        string             `db:"template_id" json:"template_id,omitempty"`
	SentAt            *time.Time         `db:"sent_at" json:"sent_at,omitempty"`
	DeliveredAt       *time.Time         `db:"delivered_at" json:"delivered_at,omitempty"`
	FailedAt          *time.Time         `db:"failed_at" json:"failed_at,omitempty"`
	ErrorMessage      *string            `db:"error_message" json:"error_message,omitempty"`
	Variables         map[string]string  `db:"variables" json:"variables,omitempty"` // JSONB
	CreatedAt         time.Time          `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time          `db:"updated_at" json:"updated_at"`
}

type NotificationEvent struct {
	ID             string    `db:"id" json:"id"`
	NotificationID string    `db:"notification_id" json:"notification_id"`
	EventType      EventType `db:"event_type" json:"event_type"`
	EventData      any       `db:"event_data" json:"event_data"` // JSONB
	OccurredAt     time.Time `db:"occurred_at" json:"occurred_at"`
}

type Recipient struct {
	ID             int           `db:"id" json:"id"`
	NotificationID string        `db:"notification_id" json:"notification_id"`
	Type           RecipientType `db:"type" json:"type"`
	Address        string        `db:"address" json:"address"` // email, phone number, device token
	CreatedAt      time.Time     `db:"created_at" json:"created_at"`
}

type NotificationFilter struct {
	Type     NotificationType
	Status   NotificationStatus
	Provider string
}

type CreateNotificationRequest struct {
	IdempotencyKey string                   `json:"idempotency_key,omitempty"`
	Type           NotificationType         `json:"type" validate:"required"`
	Recipients     []CreateRecipientRequest `json:"recipients" validate:"required,min=1"`
	CC             []CreateRecipientRequest `json:"cc,omitempty"`
	BCC            []CreateRecipientRequest `json:"bcc,omitempty"`
	Subject        string                   `json:"subject,omitempty"`
	Content        string                   `json:"content,omitempty"`
	TemplateID     string                   `json:"template_id,omitempty"`
	Variables      map[string]string        `json:"variables,omitempty"`
	ScheduledFor   *time.Time               `json:"scheduled_for,omitempty"`
	Provider       string                   `json:"provider,omitempty"`
}

type CreateRecipientRequest struct {
	Address string `json:"address" validate:"required"`
}

type UpdateNotificationRequest struct {
	ID               string              `json:"id" validate:"required"`
	Subject          *string             `json:"subject,omitempty"`
	Content          *string             `json:"content,omitempty"`
	Status           *NotificationStatus `json:"status,omitempty"`
	ScheduledFor     *time.Time          `json:"scheduled_for,omitempty"`
	Provider         *string             `json:"provider,omitempty"`
	ProviderResponse interface{}         `json:"provider_response,omitempty"`
}

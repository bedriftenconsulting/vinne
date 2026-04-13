package models

import "time"

type NotificationEventType string
type RecipientAddress string

const (
	TABLE_NOTIFICATIONS = "notifications"

	NotificationEventTypeQueued    NotificationEventType = "queued"
	NotificationEventTypeSent      NotificationEventType = "sent"
	NotificationEventTypeDelivered NotificationEventType = "delivered"
	NotificationEventTypeFailed    NotificationEventType = "failed"
	NotificationEventTypeBounced   NotificationEventType = "bounced"
	NotificationEventTypeClicked   NotificationEventType = "clicked"

	RecipientAddressEmail  RecipientAddress = "email"
	RecipientAddressPhone  RecipientAddress = "phone"
	RecipientAddressDevice RecipientAddress = "device_token"
)

type NotificationRequest struct {
	EventID       string                  `json:"event_id"`
	EventType     string                  `json:"event_type"`
	CorrelationID string                  `json:"correlation_id"`
	Timestamp     time.Time               `json:"timestamp"`
	Data          NotificationRequestData `json:"data"`
}

type NotificationRequestData struct {
	IdempotencyKey string         `json:"idempotency_key"`
	Channel        string         `json:"channel"`
	Recipient      string         `json:"recipient"`
	TemplateID     string         `json:"template_id"`
	Variables      map[string]any `json:"variables"`
	Priority       string         `json:"priority"`
}

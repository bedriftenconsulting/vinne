package models

import "time"

// RetailerNotificationType represents the type of retailer notification
type RetailerNotificationType string

const (
	RetailerNotificationTypeStake      RetailerNotificationType = "stake"
	RetailerNotificationTypeWinning    RetailerNotificationType = "winning"
	RetailerNotificationTypeCommission RetailerNotificationType = "commission"
	RetailerNotificationTypeLowBalance RetailerNotificationType = "low_balance"
	RetailerNotificationTypeGeneral    RetailerNotificationType = "general"
)

// RetailerNotification represents a notification for a specific retailer
type RetailerNotification struct {
	ID             string                   `db:"id" json:"id"`
	RetailerID     string                   `db:"retailer_id" json:"retailer_id"` // 8-digit retailer code
	Type           RetailerNotificationType `db:"type" json:"type"`
	Title          string                   `db:"title" json:"title"`
	Body           string                   `db:"body" json:"body"`
	Amount         *int64                   `db:"amount" json:"amount,omitempty"` // Amount in pesewas (GHS * 100)
	TransactionID  *string                  `db:"transaction_id" json:"transaction_id,omitempty"`
	IsRead         bool                     `db:"is_read" json:"is_read"`
	ReadAt         *time.Time               `db:"read_at" json:"read_at,omitempty"`
	NotificationID *string                  `db:"notification_id" json:"notification_id,omitempty"` // Link to notifications table
	CreatedAt      time.Time                `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time                `db:"updated_at" json:"updated_at"`
}

// CreateRetailerNotificationRequest represents the request to create a retailer notification
type CreateRetailerNotificationRequest struct {
	RetailerID     string                   `json:"retailer_id" validate:"required"`
	Type           RetailerNotificationType `json:"type" validate:"required"`
	Title          string                   `json:"title" validate:"required,max=500"`
	Body           string                   `json:"body" validate:"required"`
	Amount         *int64                   `json:"amount,omitempty"`
	TransactionID  *string                  `json:"transaction_id,omitempty"`
	NotificationID *string                  `json:"notification_id,omitempty"`
}

// RetailerNotificationFilter represents filters for querying retailer notifications
type RetailerNotificationFilter struct {
	RetailerID string
	Type       RetailerNotificationType
	UnreadOnly bool
	Page       int
	PageSize   int
}

package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of event
type EventType string

// Event types
const (
	// User events
	UserCreated         EventType = "user.created"
	UserUpdated         EventType = "user.updated"
	UserDeleted         EventType = "user.deleted"
	UserLoggedIn        EventType = "user.logged_in"
	UserLoggedOut       EventType = "user.logged_out"
	UserPasswordChanged EventType = "user.password_changed"

	// Game events
	GameCreated     EventType = "game.created"
	GameUpdated     EventType = "game.updated"
	GameDeleted     EventType = "game.deleted"
	GameActivated   EventType = "game.activated"
	GameDeactivated EventType = "game.deactivated"

	// Draw events
	DrawScheduled        EventType = "draw.scheduled"
	DrawExecuted         EventType = "draw.executed"
	DrawCancelled        EventType = "draw.cancelled"
	DrawResultsPublished EventType = "draw.results.published"
	SalesCutoffReached   EventType = "game.sales_cutoff_reached"

	// Ticket events
	TicketPurchased EventType = "ticket.purchased"
	TicketCancelled EventType = "ticket.cancelled"
	TicketWon       EventType = "ticket.won"
	TicketClaimed   EventType = "ticket.claimed"

	// Payment events
	PaymentInitiated EventType = "payment.initiated"
	PaymentCompleted EventType = "payment.completed"
	PaymentFailed    EventType = "payment.failed"
	PaymentRefunded  EventType = "payment.refunded"

	// Payout events
	PayoutInitiated EventType = "payout.initiated"
	PayoutApproved  EventType = "payout.approved"
	PayoutCompleted EventType = "payout.completed"
	PayoutFailed    EventType = "payout.failed"
)

// BaseEvent contains common fields for all events
type BaseEvent struct {
	EventID       string                 `json:"event_id"`           // Unique event ID for idempotency
	EventType     EventType              `json:"event_type"`         // Type of event
	CorrelationID string                 `json:"correlation_id"`     // For tracking across services
	Timestamp     time.Time              `json:"timestamp"`          // When the event occurred
	Version       int                    `json:"version"`            // Event schema version
	Source        string                 `json:"source"`             // Service that generated the event
	UserID        string                 `json:"user_id,omitempty"`  // User associated with the event
	Metadata      map[string]interface{} `json:"metadata,omitempty"` // Additional metadata
}

// NewBaseEvent creates a new base event
func NewBaseEvent(eventType EventType, source string) BaseEvent {
	return BaseEvent{
		EventID:       uuid.New().String(),
		EventType:     eventType,
		CorrelationID: uuid.New().String(),
		Timestamp:     time.Now().UTC(),
		Version:       1,
		Source:        source,
		Metadata:      make(map[string]interface{}),
	}
}

// WithCorrelationID sets the correlation ID
func (e BaseEvent) WithCorrelationID(id string) BaseEvent {
	e.CorrelationID = id
	return e
}

// WithUserID sets the user ID
func (e BaseEvent) WithUserID(userID string) BaseEvent {
	e.UserID = userID
	return e
}

// WithMetadata adds metadata to the event
func (e BaseEvent) WithMetadata(key string, value interface{}) BaseEvent {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// Event interface that all events must implement
type Event interface {
	GetEventID() string
	GetEventType() EventType
	GetTimestamp() time.Time
	GetSource() string
	Marshal() ([]byte, error)
}

// GetEventID returns the event ID
func (e BaseEvent) GetEventID() string {
	return e.EventID
}

// GetEventType returns the event type
func (e BaseEvent) GetEventType() EventType {
	return e.EventType
}

// GetTimestamp returns the event timestamp
func (e BaseEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// GetSource returns the event source
func (e BaseEvent) GetSource() string {
	return e.Source
}

// Marshal serializes the event to JSON
func (e BaseEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// EventEnvelope wraps an event with routing information
type EventEnvelope struct {
	Topic     string            `json:"topic"`
	Key       string            `json:"key"`
	Headers   map[string]string `json:"headers"`
	Payload   json.RawMessage   `json:"payload"`
	Timestamp time.Time         `json:"timestamp"`
}

// NewEventEnvelope creates a new event envelope
func NewEventEnvelope(topic, key string, event Event) (*EventEnvelope, error) {
	payload, err := event.Marshal()
	if err != nil {
		return nil, err
	}

	return &EventEnvelope{
		Topic: topic,
		Key:   key,
		Headers: map[string]string{
			"event_id":   event.GetEventID(),
			"event_type": string(event.GetEventType()),
			"source":     event.GetSource(),
		},
		Payload:   payload,
		Timestamp: event.GetTimestamp(),
	}, nil
}

// Example domain events

// UserCreatedEvent represents a user creation event
type UserCreatedEvent struct {
	BaseEvent
	User UserData `json:"user"`
}

// UserData represents user information in events
type UserData struct {
	ID       string   `json:"id"`
	Email    string   `json:"email"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

// NewUserCreatedEvent creates a new user created event
func NewUserCreatedEvent(source string, user UserData) *UserCreatedEvent {
	return &UserCreatedEvent{
		BaseEvent: NewBaseEvent(UserCreated, source).WithUserID(user.ID),
		User:      user,
	}
}

// Marshal serializes the event to JSON
func (e *UserCreatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

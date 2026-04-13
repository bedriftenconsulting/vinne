package events

import (
	"encoding/json"
)

// UserLoggedInEvent represents a user login event
type UserLoggedInEvent struct {
	BaseEvent
	User UserData `json:"user"`
}

// NewUserLoggedInEvent creates a new user logged in event
func NewUserLoggedInEvent(source string, user UserData) *UserLoggedInEvent {
	return &UserLoggedInEvent{
		BaseEvent: NewBaseEvent(UserLoggedIn, source).WithUserID(user.ID),
		User:      user,
	}
}

// Marshal serializes the event to JSON
func (e *UserLoggedInEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}
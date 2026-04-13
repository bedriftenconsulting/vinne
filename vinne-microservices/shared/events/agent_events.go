package events

import (
	"encoding/json"
	"time"
)

// Agent event types
const (
	// Agent events
	AgentCreated   EventType = "agent.created"
	AgentUpdated   EventType = "agent.updated"
	AgentDeleted   EventType = "agent.deleted"
	AgentApproved  EventType = "agent.approved"
	AgentSuspended EventType = "agent.suspended"
	AgentActivated EventType = "agent.activated"

	// Retailer events
	RetailerCreated   EventType = "retailer.created"
	RetailerUpdated   EventType = "retailer.updated"
	RetailerDeleted   EventType = "retailer.deleted"
	RetailerApproved  EventType = "retailer.approved"
	RetailerSuspended EventType = "retailer.suspended"
	RetailerActivated EventType = "retailer.activated"

	// POS Device events
	POSDeviceRegistered  EventType = "pos_device.registered"
	POSDeviceAssigned    EventType = "pos_device.assigned"
	POSDeviceUnassigned  EventType = "pos_device.unassigned"
	POSDeviceDeactivated EventType = "pos_device.deactivated"
)

// AgentData represents agent information in events
type AgentData struct {
	ID                   string  `json:"id"`
	AgentCode            string  `json:"agent_code"`
	Name                 string  `json:"name"`
	Email                string  `json:"email"`
	PhoneNumber          string  `json:"phone_number"`
	Status               string  `json:"status"`
	City                 string  `json:"city"`
	Region               string  `json:"region"`
	CommissionPercentage float64 `json:"commission_percentage"`
}

// RetailerData represents retailer information in events
type RetailerData struct {
	ID           string `json:"id"`
	RetailerCode string `json:"retailer_code"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	PhoneNumber  string `json:"phone_number"`
	AgentID      string `json:"agent_id"`
	AgentCode    string `json:"agent_code"`
	Status       string `json:"status"`
	City         string `json:"city"`
	Region       string `json:"region"`
}

// POSDeviceData represents POS device information in events
type POSDeviceData struct {
	ID           string `json:"id"`
	DeviceCode   string `json:"device_code"`
	IMEI         string `json:"imei"`
	Model        string `json:"model"`
	RetailerID   string `json:"retailer_id,omitempty"`
	RetailerCode string `json:"retailer_code,omitempty"`
	Status       string `json:"status"`
}

// AgentCreatedEvent represents an agent creation event
type AgentCreatedEvent struct {
	BaseEvent
	Agent     AgentData `json:"agent"`
	CreatedBy string    `json:"created_by"`
}

// NewAgentCreatedEvent creates a new agent created event
func NewAgentCreatedEvent(source string, agent AgentData, createdBy string) *AgentCreatedEvent {
	return &AgentCreatedEvent{
		BaseEvent: NewBaseEvent(AgentCreated, source).
			WithUserID(createdBy).
			WithMetadata("agent_id", agent.ID).
			WithMetadata("agent_code", agent.AgentCode),
		Agent:     agent,
		CreatedBy: createdBy,
	}
}

// Marshal serializes the event to JSON
func (e *AgentCreatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// AgentUpdatedEvent represents an agent update event
type AgentUpdatedEvent struct {
	BaseEvent
	Agent     AgentData              `json:"agent"`
	UpdatedBy string                 `json:"updated_by"`
	Changes   map[string]interface{} `json:"changes,omitempty"`
}

// NewAgentUpdatedEvent creates a new agent updated event
func NewAgentUpdatedEvent(source string, agent AgentData, updatedBy string, changes map[string]interface{}) *AgentUpdatedEvent {
	return &AgentUpdatedEvent{
		BaseEvent: NewBaseEvent(AgentUpdated, source).
			WithUserID(updatedBy).
			WithMetadata("agent_id", agent.ID).
			WithMetadata("agent_code", agent.AgentCode),
		Agent:     agent,
		UpdatedBy: updatedBy,
		Changes:   changes,
	}
}

// Marshal serializes the event to JSON
func (e *AgentUpdatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// AgentDeletedEvent represents an agent deletion event
type AgentDeletedEvent struct {
	BaseEvent
	AgentID   string    `json:"agent_id"`
	AgentCode string    `json:"agent_code"`
	DeletedBy string    `json:"deleted_by"`
	DeletedAt time.Time `json:"deleted_at"`
}

// NewAgentDeletedEvent creates a new agent deleted event
func NewAgentDeletedEvent(source string, agentID, agentCode, deletedBy string) *AgentDeletedEvent {
	return &AgentDeletedEvent{
		BaseEvent: NewBaseEvent(AgentDeleted, source).
			WithUserID(deletedBy).
			WithMetadata("agent_id", agentID).
			WithMetadata("agent_code", agentCode),
		AgentID:   agentID,
		AgentCode: agentCode,
		DeletedBy: deletedBy,
		DeletedAt: time.Now().UTC(),
	}
}

// Marshal serializes the event to JSON
func (e *AgentDeletedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// AgentStatusChangedEvent represents an agent status change event
type AgentStatusChangedEvent struct {
	BaseEvent
	AgentID   string    `json:"agent_id"`
	AgentCode string    `json:"agent_code"`
	OldStatus string    `json:"old_status"`
	NewStatus string    `json:"new_status"`
	ChangedBy string    `json:"changed_by"`
	ChangedAt time.Time `json:"changed_at"`
	Reason    string    `json:"reason,omitempty"`
}

// NewAgentStatusChangedEvent creates a new agent status changed event
func NewAgentStatusChangedEvent(source string, agentID, agentCode, oldStatus, newStatus, changedBy, reason string) *AgentStatusChangedEvent {
	eventType := AgentUpdated
	switch newStatus {
	case "approved":
		eventType = AgentApproved
	case "suspended":
		eventType = AgentSuspended
	case "active":
		eventType = AgentActivated
	}

	return &AgentStatusChangedEvent{
		BaseEvent: NewBaseEvent(eventType, source).
			WithUserID(changedBy).
			WithMetadata("agent_id", agentID).
			WithMetadata("agent_code", agentCode),
		AgentID:   agentID,
		AgentCode: agentCode,
		OldStatus: oldStatus,
		NewStatus: newStatus,
		ChangedBy: changedBy,
		ChangedAt: time.Now().UTC(),
		Reason:    reason,
	}
}

// Marshal serializes the event to JSON
func (e *AgentStatusChangedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// RetailerCreatedEvent represents a retailer creation event
type RetailerCreatedEvent struct {
	BaseEvent
	Retailer  RetailerData `json:"retailer"`
	CreatedBy string       `json:"created_by"`
}

// NewRetailerCreatedEvent creates a new retailer created event
func NewRetailerCreatedEvent(source string, retailer RetailerData, createdBy string) *RetailerCreatedEvent {
	return &RetailerCreatedEvent{
		BaseEvent: NewBaseEvent(RetailerCreated, source).
			WithUserID(createdBy).
			WithMetadata("retailer_id", retailer.ID).
			WithMetadata("retailer_code", retailer.RetailerCode).
			WithMetadata("agent_id", retailer.AgentID),
		Retailer:  retailer,
		CreatedBy: createdBy,
	}
}

// Marshal serializes the event to JSON
func (e *RetailerCreatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// POSDeviceRegisteredEvent represents a POS device registration event
type POSDeviceRegisteredEvent struct {
	BaseEvent
	Device       POSDeviceData `json:"device"`
	RegisteredBy string        `json:"registered_by"`
}

// NewPOSDeviceRegisteredEvent creates a new POS device registered event
func NewPOSDeviceRegisteredEvent(source string, device POSDeviceData, registeredBy string) *POSDeviceRegisteredEvent {
	return &POSDeviceRegisteredEvent{
		BaseEvent: NewBaseEvent(POSDeviceRegistered, source).
			WithUserID(registeredBy).
			WithMetadata("device_id", device.ID).
			WithMetadata("device_code", device.DeviceCode).
			WithMetadata("imei", device.IMEI),
		Device:       device,
		RegisteredBy: registeredBy,
	}
}

// Marshal serializes the event to JSON
func (e *POSDeviceRegisteredEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// POSDeviceAssignedEvent represents a POS device assignment event
type POSDeviceAssignedEvent struct {
	BaseEvent
	DeviceID     string    `json:"device_id"`
	DeviceCode   string    `json:"device_code"`
	RetailerID   string    `json:"retailer_id"`
	RetailerCode string    `json:"retailer_code"`
	AssignedBy   string    `json:"assigned_by"`
	AssignedAt   time.Time `json:"assigned_at"`
}

// NewPOSDeviceAssignedEvent creates a new POS device assigned event
func NewPOSDeviceAssignedEvent(source string, deviceID, deviceCode, retailerID, retailerCode, assignedBy string) *POSDeviceAssignedEvent {
	return &POSDeviceAssignedEvent{
		BaseEvent: NewBaseEvent(POSDeviceAssigned, source).
			WithUserID(assignedBy).
			WithMetadata("device_id", deviceID).
			WithMetadata("retailer_id", retailerID),
		DeviceID:     deviceID,
		DeviceCode:   deviceCode,
		RetailerID:   retailerID,
		RetailerCode: retailerCode,
		AssignedBy:   assignedBy,
		AssignedAt:   time.Now().UTC(),
	}
}

// Marshal serializes the event to JSON
func (e *POSDeviceAssignedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

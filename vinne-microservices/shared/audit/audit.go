package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ActionType represents the type of action being audited
type ActionType string

const (
	// User management actions
	ActionUserCreated       ActionType = "user.created"
	ActionUserUpdated       ActionType = "user.updated"
	ActionUserDeleted       ActionType = "user.deleted"
	ActionUserStatusChanged ActionType = "user.status_changed"

	// Authentication actions
	ActionLogin        ActionType = "auth.login"
	ActionLogout       ActionType = "auth.logout"
	ActionLoginFailed  ActionType = "auth.login_failed"
	ActionTokenRefresh ActionType = "auth.token_refresh"

	// Role and permission actions
	ActionRoleAssigned ActionType = "role.assigned"
	ActionRoleRemoved  ActionType = "role.removed"
	ActionRoleCreated  ActionType = "role.created"
	ActionRoleUpdated  ActionType = "role.updated"
	ActionRoleDeleted  ActionType = "role.deleted"

	// Permission actions
	ActionPermissionGranted ActionType = "permission.granted"
	ActionPermissionRevoked ActionType = "permission.revoked"
	ActionPermissionCreated ActionType = "permission.created"
	ActionPermissionUpdated ActionType = "permission.updated"
	ActionPermissionDeleted ActionType = "permission.deleted"

	// Data access actions
	ActionDataAccess   ActionType = "data.access"
	ActionDataExport   ActionType = "data.export"
	ActionDataModified ActionType = "data.modified"
	ActionDataDeleted  ActionType = "data.deleted"
)

// LogEntry represents an audit log entry
type LogEntry struct {
	ID            uuid.UUID              `json:"id"`
	Timestamp     time.Time              `json:"timestamp"`
	ServiceName   string                 `json:"service_name"`
	Action        ActionType             `json:"action"`
	ActorID       string                 `json:"actor_id"`
	ActorType     string                 `json:"actor_type"` // user, system, service
	ActorMetadata map[string]interface{} `json:"actor_metadata,omitempty"`
	ResourceID    string                 `json:"resource_id,omitempty"`
	ResourceType  string                 `json:"resource_type,omitempty"`
	Details       map[string]interface{} `json:"details,omitempty"`
	Result        string                 `json:"result"` // success, failure
	ErrorMessage  string                 `json:"error_message,omitempty"`
	IPAddress     string                 `json:"ip_address,omitempty"`
	UserAgent     string                 `json:"user_agent,omitempty"`
	TraceID       string                 `json:"trace_id,omitempty"`
	SpanID        string                 `json:"span_id,omitempty"`
}

// Logger interface for audit logging
type Logger interface {
	Log(ctx context.Context, entry *LogEntry) error
	LogAction(ctx context.Context, action ActionType, details map[string]interface{}) error
	LogSuccess(ctx context.Context, action ActionType, resourceID, resourceType string, details map[string]interface{}) error
	LogFailure(ctx context.Context, action ActionType, resourceID, resourceType string, err error, details map[string]interface{}) error
	Query(ctx context.Context, filter QueryFilter) ([]*LogEntry, error)
}

// QueryFilter for querying audit logs
type QueryFilter struct {
	StartTime    *time.Time
	EndTime      *time.Time
	ActorID      *string
	Action       *ActionType
	ResourceID   *string
	ResourceType *string
	Result       *string
	ServiceName  *string
	Limit        int
	Offset       int
}

// NewLogEntry creates a new audit log entry
func NewLogEntry(action ActionType, actorID, actorType string) *LogEntry {
	return &LogEntry{
		ID:        uuid.New(),
		Timestamp: time.Now().UTC(),
		Action:    action,
		ActorID:   actorID,
		ActorType: actorType,
		Details:   make(map[string]interface{}),
		Result:    "success",
	}
}

// WithService sets the service name
func (e *LogEntry) WithService(serviceName string) *LogEntry {
	e.ServiceName = serviceName
	return e
}

// WithResource sets the resource information
func (e *LogEntry) WithResource(resourceID, resourceType string) *LogEntry {
	e.ResourceID = resourceID
	e.ResourceType = resourceType
	return e
}

// WithDetails adds details to the log entry
func (e *LogEntry) WithDetails(key string, value interface{}) *LogEntry {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithError marks the entry as failure and adds error message
func (e *LogEntry) WithError(err error) *LogEntry {
	e.Result = "failure"
	if err != nil {
		e.ErrorMessage = err.Error()
	}
	return e
}

// WithRequestInfo adds HTTP request information
func (e *LogEntry) WithRequestInfo(ipAddress, userAgent string) *LogEntry {
	e.IPAddress = ipAddress
	e.UserAgent = userAgent
	return e
}

// WithTracing adds distributed tracing information
func (e *LogEntry) WithTracing(traceID, spanID string) *LogEntry {
	e.TraceID = traceID
	e.SpanID = spanID
	return e
}

// Marshal serializes the log entry to JSON
func (e *LogEntry) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// ExtractTraceInfo extracts trace information from context
func ExtractTraceInfo(ctx context.Context) (traceID, spanID string) {
	// This would be implemented based on the actual tracing library used
	// For now, return empty strings
	return "", ""
}

package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/randco/randco-microservices/shared/events"
)

// EventLogger implements audit logging via event bus
type EventLogger struct {
	serviceName string
	eventBus    events.EventBus
	fallbackLog bool // whether to use standard logging as fallback
}

// NewEventLogger creates a new event-based audit logger
func NewEventLogger(serviceName string, eventBus events.EventBus, fallbackLog bool) Logger {
	return &EventLogger{
		serviceName: serviceName,
		eventBus:    eventBus,
		fallbackLog: fallbackLog,
	}
}

// Log logs an audit entry
func (l *EventLogger) Log(ctx context.Context, entry *LogEntry) error {
	if entry.ServiceName == "" {
		entry.ServiceName = l.serviceName
	}

	// Extract trace information from context
	traceID, spanID := ExtractTraceInfo(ctx)
	if traceID != "" {
		entry.TraceID = traceID
	}
	if spanID != "" {
		entry.SpanID = spanID
	}

	// Create audit event
	auditEvent := &AuditLogEvent{
		BaseEvent: events.NewBaseEvent(events.EventType("audit.log"), l.serviceName),
		LogEntry:  *entry,
	}

	// Publish to event bus
	if l.eventBus != nil {
		if err := l.eventBus.Publish(ctx, "audit.logs", auditEvent); err != nil {
			if l.fallbackLog {
				l.logToStderr(entry)
			}
			return fmt.Errorf("failed to publish audit log: %w", err)
		}
	} else if l.fallbackLog {
		l.logToStderr(entry)
	}

	return nil
}

// LogAction logs a simple action
func (l *EventLogger) LogAction(ctx context.Context, action ActionType, details map[string]interface{}) error {
	// Extract actor information from context (this would need to be implemented based on your auth system)
	actorID := extractActorID(ctx)

	entry := NewLogEntry(action, actorID, "user").
		WithService(l.serviceName)

	for k, v := range details {
		entry.WithDetails(k, v)
	}

	return l.Log(ctx, entry)
}

// LogSuccess logs a successful action
func (l *EventLogger) LogSuccess(ctx context.Context, action ActionType, resourceID, resourceType string, details map[string]interface{}) error {
	actorID := extractActorID(ctx)

	entry := NewLogEntry(action, actorID, "user").
		WithService(l.serviceName).
		WithResource(resourceID, resourceType)

	for k, v := range details {
		entry.WithDetails(k, v)
	}

	return l.Log(ctx, entry)
}

// LogFailure logs a failed action
func (l *EventLogger) LogFailure(ctx context.Context, action ActionType, resourceID, resourceType string, err error, details map[string]interface{}) error {
	actorID := extractActorID(ctx)

	entry := NewLogEntry(action, actorID, "user").
		WithService(l.serviceName).
		WithResource(resourceID, resourceType).
		WithError(err)

	for k, v := range details {
		entry.WithDetails(k, v)
	}

	return l.Log(ctx, entry)
}

// Query queries audit logs (not implemented for event-based logger)
func (l *EventLogger) Query(ctx context.Context, filter QueryFilter) ([]*LogEntry, error) {
	return nil, fmt.Errorf("query not supported in event-based audit logger")
}

// logToStderr logs to stderr as fallback
func (l *EventLogger) logToStderr(entry *LogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("AUDIT: Failed to marshal log entry: %v", err)
		return
	}
	log.Printf("AUDIT: %s", string(data))
}

// extractActorID extracts the actor ID from context
// This should be implemented based on your authentication system
func extractActorID(ctx context.Context) string {
	// TODO: Implement based on your auth context
	// For now, return "system"
	return "system"
}

// AuditLogEvent represents an audit log event
type AuditLogEvent struct {
	events.BaseEvent
	LogEntry LogEntry `json:"log_entry"`
}

// Marshal serializes the event to JSON
func (e *AuditLogEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

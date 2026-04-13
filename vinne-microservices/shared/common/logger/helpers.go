package logger

import (
	"log/slog"
)

// Common attribute keys for consistent logging
const (
	KeyError     = "error"
	KeyUserID    = "user_id"
	KeyRequestID = "request_id"
	KeyService   = "service"
	KeyMethod    = "method"
	KeyDuration  = "duration"
	KeyStatus    = "status"
	KeyPath      = "path"
	KeyIP        = "ip"
)

// Attr creates a slog attribute - helper for structured logging
func Attr(key string, value interface{}) slog.Attr {
	return slog.Any(key, value)
}

// Error creates an error attribute
func ErrorAttr(err error) slog.Attr {
	if err == nil {
		return slog.String(KeyError, "")
	}
	return slog.String(KeyError, err.Error())
}

// Group creates a group of attributes
func Group(name string, attrs ...slog.Attr) slog.Attr {
	return slog.Group(name, slogAttrsToAny(attrs)...)
}

func slogAttrsToAny(attrs []slog.Attr) []any {
	result := make([]any, len(attrs))
	for i, attr := range attrs {
		result[i] = attr
	}
	return result
}
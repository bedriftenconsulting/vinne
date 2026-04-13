package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// StandardResponse represents the standard API response format per PRD
type StandardResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Error     *ErrorInfo  `json:"error,omitempty"`
	Meta      Meta        `json:"meta"`
	Timestamp string      `json:"timestamp"`
}

// ErrorInfo represents detailed error information
type ErrorInfo struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// Meta contains metadata about the request
type Meta struct {
	RequestID string `json:"request_id"`
	Version   string `json:"version"`
}

// PaginationMeta extends Meta with pagination information
type PaginationMeta struct {
	Meta
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// StandardResponseWithPagination includes pagination metadata
type StandardResponseWithPagination struct {
	Success   bool           `json:"success"`
	Message   string         `json:"message,omitempty"`
	Data      interface{}    `json:"data,omitempty"`
	Error     *ErrorInfo     `json:"error,omitempty"`
	Meta      PaginationMeta `json:"meta"`
	Timestamp string         `json:"timestamp"`
}

const (
	// API version
	APIVersion = "1.0.0"
)

// Common error codes
const (
	ErrCodeValidation         = "VALIDATION_ERROR"
	ErrCodeUnauthorized       = "UNAUTHORIZED"
	ErrCodeForbidden          = "FORBIDDEN"
	ErrCodeNotFound           = "NOT_FOUND"
	ErrCodeConflict           = "CONFLICT"
	ErrCodeInternal           = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrCodeBadGateway         = "BAD_GATEWAY"
	ErrCodeTimeout            = "TIMEOUT"
	ErrCodeRateLimit          = "RATE_LIMIT_EXCEEDED"
)

// Success sends a successful response
func Success(w http.ResponseWriter, statusCode int, message string, data interface{}) error {
	return JSON(w, statusCode, StandardResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Meta:      newMeta(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// SuccessWithPagination sends a successful response with pagination
func SuccessWithPagination(w http.ResponseWriter, message string, data interface{}, page, perPage, total int) error {
	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}

	return JSON(w, http.StatusOK, StandardResponseWithPagination{
		Success: true,
		Message: message,
		Data:    data,
		Meta: PaginationMeta{
			Meta:       newMeta(),
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Error sends an error response
func Error(w http.ResponseWriter, statusCode int, code string, message string, details interface{}) error {
	return JSON(w, statusCode, StandardResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
		Meta:      newMeta(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// ValidationError sends a validation error response
func ValidationError(w http.ResponseWriter, message string, details interface{}) error {
	return Error(w, http.StatusBadRequest, ErrCodeValidation, message, details)
}

// UnauthorizedError sends an unauthorized error response
func UnauthorizedError(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Authentication required"
	}
	return Error(w, http.StatusUnauthorized, ErrCodeUnauthorized, message, nil)
}

// ForbiddenError sends a forbidden error response
func ForbiddenError(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Access denied"
	}
	return Error(w, http.StatusForbidden, ErrCodeForbidden, message, nil)
}

// NotFoundError sends a not found error response
func NotFoundError(w http.ResponseWriter, resource string) error {
	message := "Resource not found"
	if resource != "" {
		message = resource + " not found"
	}
	return Error(w, http.StatusNotFound, ErrCodeNotFound, message, nil)
}

// ConflictError sends a conflict error response
func ConflictError(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Resource conflict"
	}
	return Error(w, http.StatusConflict, ErrCodeConflict, message, nil)
}

// InternalError sends an internal server error response
func InternalError(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Internal server error"
	}
	return Error(w, http.StatusInternalServerError, ErrCodeInternal, message, nil)
}

// ServiceUnavailableError sends a service unavailable error response
func ServiceUnavailableError(w http.ResponseWriter, service string) error {
	message := "Service temporarily unavailable"
	if service != "" {
		message = service + " service unavailable"
	}
	return Error(w, http.StatusServiceUnavailable, ErrCodeServiceUnavailable, message, nil)
}

// BadGatewayError sends a bad gateway error response
func BadGatewayError(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Bad gateway"
	}
	return Error(w, http.StatusBadGateway, ErrCodeBadGateway, message, nil)
}

// TimeoutError sends a timeout error response
func TimeoutError(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Request timeout"
	}
	return Error(w, http.StatusGatewayTimeout, ErrCodeTimeout, message, nil)
}

// RateLimitError sends a rate limit exceeded error response
func RateLimitError(w http.ResponseWriter) error {
	return Error(w, http.StatusTooManyRequests, ErrCodeRateLimit, "Rate limit exceeded", nil)
}

// JSON writes a JSON response
func JSON(w http.ResponseWriter, statusCode int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Convert protobuf messages to JSON-compatible format
	jsonData := convertProtoToJSON(data)

	return json.NewEncoder(w).Encode(jsonData)
}

// convertProtoToJSON recursively converts protobuf messages to JSON-compatible format
func convertProtoToJSON(data interface{}) interface{} {
	if data == nil {
		return nil
	}

	// Check if it's a protobuf message
	if msg, ok := data.(proto.Message); ok {
		fmt.Printf("DEBUG convertProtoToJSON: Found proto.Message of type %T\n", msg)
		// Use protojson to marshal to bytes
		marshaler := protojson.MarshalOptions{
			UseProtoNames:   true, // true = Use proto field names (e.g., id, game_code, game_name) for consistent snake_case
			EmitUnpopulated: true, // Emit zero values to prevent null fields
		}
		jsonBytes, err := marshaler.Marshal(msg)
		if err != nil {
			return data // Fallback to original data
		}

		// Unmarshal to map for standard JSON encoding
		var result interface{}
		if err := json.Unmarshal(jsonBytes, &result); err != nil {
			return data // Fallback to original data
		}
		return result
	}

	// Handle StandardResponse and StandardResponseWithPagination
	switch v := data.(type) {
	case StandardResponse:
		return StandardResponse{
			Success:   v.Success,
			Message:   v.Message,
			Data:      convertProtoToJSON(v.Data),
			Error:     v.Error,
			Meta:      v.Meta,
			Timestamp: v.Timestamp,
		}
	case StandardResponseWithPagination:
		return StandardResponseWithPagination{
			Success:   v.Success,
			Message:   v.Message,
			Data:      convertProtoToJSON(v.Data),
			Error:     v.Error,
			Meta:      v.Meta,
			Timestamp: v.Timestamp,
		}
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, val := range v {
			result[key] = convertProtoToJSON(val)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = convertProtoToJSON(val)
		}
		return result
	default:
		return data
	}
}

// newMeta creates a new Meta object with request ID and version
func newMeta() Meta {
	return Meta{
		RequestID: "req_" + uuid.New().String(),
		Version:   APIVersion,
	}
}

// GetRequestID extracts request ID from context or generates a new one
func GetRequestID(r *http.Request) string {
	// Try to get from context first (if set by middleware)
	if reqID := r.Context().Value("request_id"); reqID != nil {
		if id, ok := reqID.(string); ok && id != "" {
			return id
		}
	}

	// Try to get from header
	if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
		return reqID
	}

	// Generate new one
	return "req_" + uuid.New().String()
}

// SetRequestID sets the request ID in the response header
func SetRequestID(w http.ResponseWriter, requestID string) {
	w.Header().Set("X-Request-ID", requestID)
}

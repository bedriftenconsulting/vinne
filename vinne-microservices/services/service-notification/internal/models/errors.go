package models

const (
	// Client errors (4xx equivalent)
	ErrInvalidRecipient = "INVALID_RECIPIENT"
	ErrTemplateNotFound = "TEMPLATE_NOT_FOUND"
	ErrMissingVariables = "MISSING_VARIABLES"
	ErrInvalidPriority  = "INVALID_PRIORITY"

	// Server errors (5xx equivalent)
	ErrProviderFailure = "PROVIDER_FAILURE"
	ErrQueueFull       = "QUEUE_FULL"
	ErrDatabaseError   = "DATABASE_ERROR"
	ErrInternalError   = "INTERNAL_ERROR"

	// Rate limiting
	ErrRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
	ErrQuotaExhausted    = "QUOTA_EXHAUSTED"

	// Idempotency
	ErrDuplicateRequest = "DUPLICATE_REQUEST"
)

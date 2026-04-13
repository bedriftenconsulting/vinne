package storage

import (
	"context"
	"time"
)

// Provider represents a storage provider type
type Provider string

const (
	ProviderSpaces Provider = "spaces"
	ProviderS3     Provider = "s3"
	ProviderAzure  Provider = "azure"

	// Maximum file size (5MB)
	MaxFileSize = 5 * 1024 * 1024

	// Allowed MIME types for images
	AllowedMIMETypes = "image/png,image/jpeg,image/jpg,image/webp"

	// Default retry attempts
	DefaultRetryAttempts = 3

	// Default retry backoff
	DefaultRetryBackoff = 100 * time.Millisecond
)

// ObjectInfo represents metadata about a stored object
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ContentType  string
	ETag         string
	URL          string
	CDNURL       string
}

// UploadInfo contains information needed for uploading files
type UploadInfo struct {
	GameID      string
	FileName    string
	ContentType string
	Size        int64
	Data        []byte // File data for retryable uploads
	Permission  string // "public" or "private"
}

// Storage defines the interface for storage operations
type Storage interface {
	// Upload stores a file and returns its object info
	Upload(ctx context.Context, info UploadInfo) (*ObjectInfo, error)

	// Delete removes a file by its key
	Delete(ctx context.Context, gameID string) error

	// GetURL returns a presigned URL for accessing the file
	GetURL(ctx context.Context, key string, expires time.Duration) (string, error)

	// Close releases any resources used by the storage
	Close() error
}

// Config holds configuration for storage providers
type Config struct {
	Provider        Provider
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	CDNEndpoint     string
	ForcePathStyle  bool
	// Retry config
	MaxRetries   int
	RetryBackoff time.Duration
}

// New creates a new storage instance based on the provider
func New(cfg Config) (Storage, error) {
	// Set defaults
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = DefaultRetryAttempts
	}
	if cfg.RetryBackoff == 0 {
		cfg.RetryBackoff = DefaultRetryBackoff
	}

	switch cfg.Provider {
	case ProviderSpaces:
		return newSpacesStorage(cfg)
	case ProviderS3:
		// Future implementation
		return nil, ErrUnsupportedProvider
	case ProviderAzure:
		// Future implementation
		return nil, ErrUnsupportedProvider
	default:
		return nil, ErrUnsupportedProvider
	}
}

// buildKey creates a consistent key structure for game logos
func buildKey(gameID, fileName string) string {
	// games/{gameID}/logo.{ext}
	return "games/" + gameID + "/" + fileName
}

// ValidateImage validates that the file is an allowed image type and size
func ValidateImage(contentType string, size int64) error {
	// Check file size
	if size > MaxFileSize {
		return ErrFileTooLarge
	}

	// Check MIME type
	switch contentType {
	case "image/png", "image/jpeg", "image/jpg", "image/webp":
		return nil
	default:
		return ErrInvalidFileType
	}
}

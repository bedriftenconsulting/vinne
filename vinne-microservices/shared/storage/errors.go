package storage

import "errors"

var (
	// ErrUnsupportedProvider is returned when an unsupported storage provider is specified
	ErrUnsupportedProvider = errors.New("unsupported storage provider")

	// ErrInvalidConfig is returned when the storage configuration is invalid
	ErrInvalidConfig = errors.New("invalid storage configuration")

	// ErrFileNotFound is returned when a file is not found
	ErrFileNotFound = errors.New("file not found")

	// ErrInvalidFileType is returned when an unsupported file type is uploaded
	ErrInvalidFileType = errors.New("invalid file type, only images are allowed")

	// ErrFileTooLarge is returned when a file exceeds the maximum size limit
	ErrFileTooLarge = errors.New("file too large, maximum size is 5MB")

	// ErrOperationTimeout is returned when a storage operation times out
	ErrOperationTimeout = errors.New("storage operation timed out")

	// ErrUploadFailed is returned when an upload operation fails
	ErrUploadFailed = errors.New("failed to upload file")

	// ErrDeleteFailed is returned when a delete operation fails
	ErrDeleteFailed = errors.New("failed to delete file")
)

package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Logger interface defines logging methods
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Fatal(msg string, fields ...interface{})
	With(fields ...interface{}) Logger
	WithContext(ctx context.Context) Logger
	Close() error // Close any open file handles
}

// Config holds logger configuration
type Config struct {
	Level       string
	Format      string
	ServiceName string // Name of the service for tagging logs
	LogFile     string // Path to log file (optional)
}

// SlogLogger implements Logger using log/slog
type SlogLogger struct {
	logger  *slog.Logger
	logFile *os.File // Keep reference to close it if needed
}

// NewLogger creates a new logger instance using slog
func NewLogger(config Config) Logger {
	level := parseLevel(config.Level)
	
	// Create multi-writer for stdout and optional file
	writers := []io.Writer{os.Stdout}
	
	var logFile *os.File
	if config.LogFile != "" {
		// Create log directory if it doesn't exist
		logDir := filepath.Dir(config.LogFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create log directory %s: %v\n", logDir, err)
		} else {
			// Open or create log file with append mode
			file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open log file %s: %v\n", config.LogFile, err)
			} else {
				logFile = file
				writers = append(writers, file)
			}
		}
	}
	
	// Create multi-writer
	multiWriter := io.MultiWriter(writers...)
	
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
		AddSource: false,
	}
	
	switch strings.ToLower(config.Format) {
	case "json":
		handler = slog.NewJSONHandler(multiWriter, opts)
	default:
		handler = slog.NewTextHandler(multiWriter, opts)
	}
	
	// Add service name as a default attribute if provided
	logger := slog.New(handler)
	if config.ServiceName != "" {
		logger = logger.With("service", config.ServiceName)
	}
	
	return &SlogLogger{
		logger:  logger,
		logFile: logFile,
	}
}

// Debug logs a debug message
func (l *SlogLogger) Debug(msg string, fields ...interface{}) {
	l.logger.Debug(msg, fields...)
}

// Info logs an info message
func (l *SlogLogger) Info(msg string, fields ...interface{}) {
	l.logger.Info(msg, fields...)
}

// Warn logs a warning message
func (l *SlogLogger) Warn(msg string, fields ...interface{}) {
	l.logger.Warn(msg, fields...)
}

// Error logs an error message
func (l *SlogLogger) Error(msg string, fields ...interface{}) {
	l.logger.Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func (l *SlogLogger) Fatal(msg string, fields ...interface{}) {
	l.logger.Error(msg, fields...) // Log as error first
	os.Exit(1)
}

// With returns a new logger with additional fields
func (l *SlogLogger) With(fields ...interface{}) Logger {
	return &SlogLogger{
		logger:  l.logger.With(fields...),
		logFile: l.logFile, // Preserve file reference
	}
}

// WithContext returns a new logger with context
func (l *SlogLogger) WithContext(ctx context.Context) Logger {
	// For now, just return the same logger
	// In Go 1.21+, you can use: logger: l.logger.WithContext(ctx)
	return l
}

// Close closes the log file if it was opened
func (l *SlogLogger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// parseLevel parses the log level string to slog.Level
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// SetDefault sets this logger as the default slog logger
func SetDefault(logger Logger) {
	if sl, ok := logger.(*SlogLogger); ok {
		slog.SetDefault(sl.logger)
	}
}
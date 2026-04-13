package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/randco/randco-microservices/services/service-notification/internal/config"
)

func New(cfg *config.Config) (*sql.DB, error) {
	// Log the database URL for debugging (mask password)
	maskedURL := maskPassword(cfg.Database.URL)

	if cfg.Database.URL == "" {
		fmt.Fprintf(os.Stderr, "ERROR: Database URL is empty!\n")
		return nil, fmt.Errorf("database URL is empty")
	}

	db, err := sql.Open("pgx", cfg.Database.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		fmt.Fprintf(os.Stderr, "Connection string (masked): %s\n", maskedURL)
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	fmt.Println("Database connection opened successfully")

	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to ping database: %v\n", err)
		fmt.Fprintf(os.Stderr, "Connection string (masked): %s\n", maskedURL)
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	fmt.Println("Database ping successful")
	return db, nil
}

func maskPassword(url string) string {
	// Simple password masking for logging
	if url == "" {
		return "<empty>"
	}
	// Find password in postgresql://user:password@host:port/db format
	start := 0
	for i, c := range url {
		if c == ':' && i > 0 {
			start = i + 1
			break
		}
	}
	end := start
	for i := start; i < len(url); i++ {
		if url[i] == '@' {
			end = i
			break
		}
	}
	if start > 0 && end > start {
		return url[:start] + "***" + url[end:]
	}
	return url
}

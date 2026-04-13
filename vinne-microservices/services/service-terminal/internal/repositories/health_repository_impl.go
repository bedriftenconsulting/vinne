package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/randco/randco-microservices/services/service-terminal/internal/models"
	"github.com/randco/randco-microservices/shared/middleware/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type healthRepositoryImpl struct {
	db     *sqlx.DB
	tracer trace.Tracer
}

func NewTerminalHealthRepository(db *sqlx.DB) TerminalHealthRepository {
	return &healthRepositoryImpl{
		db:     db,
		tracer: otel.Tracer("service-terminal.health-repository"),
	}
}

func (r *healthRepositoryImpl) CreateOrUpdateHealth(ctx context.Context, health *models.TerminalHealth) error {
	dbSpan := tracing.TraceDB(ctx, "INSERT", "terminal_health").SetID(health.TerminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	if health.ID == uuid.Nil {
		health.ID = uuid.New()
	}
	if health.LastHeartbeat.IsZero() {
		health.LastHeartbeat = time.Now()
	}

	query := `
		INSERT INTO terminal_health (
			id, terminal_id, status, battery_level, signal_strength,
			storage_available, storage_total, memory_usage, cpu_usage, last_heartbeat, diagnostics, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW()
		)
		ON CONFLICT (terminal_id) DO UPDATE SET
			status = EXCLUDED.status,
			battery_level = EXCLUDED.battery_level,
			signal_strength = EXCLUDED.signal_strength,
			storage_available = EXCLUDED.storage_available,
			storage_total = EXCLUDED.storage_total,
			memory_usage = EXCLUDED.memory_usage,
			cpu_usage = EXCLUDED.cpu_usage,
			last_heartbeat = EXCLUDED.last_heartbeat,
			diagnostics = EXCLUDED.diagnostics,
			updated_at = NOW()
	`

	diagnosticsJSON, err := json.Marshal(health.Diagnostics)
	if err != nil {
		dbSpan.End(err)
		return fmt.Errorf("failed to marshal diagnostics: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		health.ID,
		health.TerminalID,
		health.Status,
		health.BatteryLevel,
		health.SignalStrength,
		health.StorageAvailable,
		health.StorageTotal,
		health.MemoryUsage,
		health.CPUUsage,
		health.LastHeartbeat,
		diagnosticsJSON,
	)

	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to create or update health: %w", err))
	}

	return dbSpan.End(nil)
}

func (r *healthRepositoryImpl) GetHealthByTerminalID(ctx context.Context, terminalID uuid.UUID) (*models.TerminalHealth, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminal_health").SetID(terminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, terminal_id, status, battery_level, signal_strength, storage_available,
				storage_total, memory_usage, cpu_usage, last_heartbeat, diagnostics, created_at, updated_at
		FROM terminal_health
		WHERE terminal_id = $1 AND deleted_at IS NULL
		LIMIT 1
	`

	var health models.TerminalHealth
	var diagnosticsJSON []byte

	err := r.db.QueryRowContext(ctx, query, terminalID).Scan(
		&health.ID,
		&health.TerminalID,
		&health.Status,
		&health.BatteryLevel,
		&health.SignalStrength,
		&health.StorageAvailable,
		&health.StorageTotal,
		&health.MemoryUsage,
		&health.CPUUsage,
		&health.LastHeartbeat,
		&diagnosticsJSON,
		&health.CreatedAt,
		&health.UpdatedAt,
	)

	if err != nil {
		return nil, dbSpan.End(err)
	}

	if err := json.Unmarshal(diagnosticsJSON, &health.Diagnostics); err != nil {
		dbSpan.End(err)
		return nil, fmt.Errorf("failed to unmarshal diagnostics: %w", err)
	}

	return &health, dbSpan.End(nil)
}

func (r *healthRepositoryImpl) RecordHealthHistory(ctx context.Context, history *models.TerminalHealthHistory) error {
	dbSpan := tracing.TraceDB(ctx, "INSERT", "terminal_health_history").SetID(history.TerminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		INSERT INTO terminal_health_history (
			id, terminal_id, status, battery_level, signal_strength, storage_available,
			storage_total, memory_usage, cpu_usage, diagnostics, recorded_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW()
		)
	`

	diagnosticsJSON, err := json.Marshal(history.Diagnostics)
	if err != nil {
		dbSpan.End(err)
		return fmt.Errorf("failed to marshal diagnostics: %w", err)
	}

	newID := uuid.New()

	_, err = r.db.ExecContext(ctx, query,
		newID,
		history.TerminalID,
		history.Status,
		history.BatteryLevel,
		history.SignalStrength,
		history.StorageAvailable,
		history.StorageTotal,
		history.MemoryUsage,
		history.CPUUsage,
		diagnosticsJSON,
	)

	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to record health history: %w", err))
	}

	return dbSpan.End(nil)
}

func (r *healthRepositoryImpl) GetHealthHistory(ctx context.Context, terminalID uuid.UUID, fromTime time.Time, toTime time.Time) ([]*models.TerminalHealthHistory, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminal_health_history").SetID(terminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT 
			id, terminal_id, status, battery_level, signal_strength, storage_available,
			storage_total, memory_usage, cpu_usage, diagnostics, recorded_at
		FROM terminal_health_history
		WHERE terminal_id = $1 AND deleted_at IS NULL AND recorded_at BETWEEN $2 AND $3
		ORDER BY recorded_at DESC
		`
	rows, err := r.db.QueryContext(ctx, query, terminalID, fromTime, toTime)
	if err != nil {
		return nil, dbSpan.End(err)
	}
	defer rows.Close()

	var history []*models.TerminalHealthHistory

	for rows.Next() {
		var record models.TerminalHealthHistory
		var diagnosticsJSON []byte

		err := rows.Scan(
			&record.ID,
			&record.TerminalID,
			&record.Status,
			&record.BatteryLevel,
			&record.SignalStrength,
			&record.StorageAvailable,
			&record.StorageTotal,
			&record.MemoryUsage,
			&record.CPUUsage,
			&diagnosticsJSON,
			&record.RecordedAt,
		)
		if err != nil {
			dbSpan.End(err)
			return nil, fmt.Errorf("failed to scan health history row: %w", err)
		}

		if err := json.Unmarshal(diagnosticsJSON, &record.Diagnostics); err != nil {
			dbSpan.End(err)
			return nil, fmt.Errorf("failed to unmarshal diagnostics: %w", err)
		}

		history = append(history, &record)
	}

	if err := rows.Err(); err != nil {
		return nil, dbSpan.End(err)
	}
	return history, dbSpan.End(nil)
}

func (r *healthRepositoryImpl) GetUnhealthyTerminals(ctx context.Context) ([]*models.TerminalHealth, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminal_health").SetID("unhealthy")
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, terminal_id, status, battery_level, signal_strength, storage_available,
				storage_total, memory_usage, cpu_usage, last_heartbeat, diagnostics, created_at, updated_at
		FROM terminal_health
		WHERE deleted_at IS NULL AND status IN ('WARNING', 'CRITICAL', 'OFFLINE')`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, dbSpan.End(err)
	}
	defer rows.Close()

	var healthRecords []*models.TerminalHealth

	for rows.Next() {
		var health models.TerminalHealth
		var diagnosticsJSON []byte

		err := rows.Scan(
			&health.ID,
			&health.TerminalID,
			&health.Status,
			&health.BatteryLevel,
			&health.SignalStrength,
			&health.StorageAvailable,
			&health.StorageTotal,
			&health.MemoryUsage,
			&health.CPUUsage,
			&health.LastHeartbeat,
			&diagnosticsJSON,
			&health.CreatedAt,
			&health.UpdatedAt,
		)
		if err != nil {
			dbSpan.End(err)
			return nil, fmt.Errorf("failed to scan health history row: %w", err)
		}

		if err := json.Unmarshal(diagnosticsJSON, &health.Diagnostics); err != nil {
			dbSpan.End(err)
			return nil, fmt.Errorf("failed to unmarshal diagnostics: %w", err)
		}

		healthRecords = append(healthRecords, &health)
	}

	if err := rows.Err(); err != nil {
		return nil, dbSpan.End(err)
	}

	return healthRecords, dbSpan.End(nil)
}

func (r *healthRepositoryImpl) GetOfflineTerminals(ctx context.Context, threshold time.Duration) ([]*models.TerminalHealth, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminal_health").SetID("offline")
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	cutoffTime := time.Now().Add(-threshold)

	query := `
		SELECT id, terminal_id, status, battery_level, signal_strength, storage_available,
				storage_total, memory_usage, cpu_usage, last_heartbeat, diagnostics, created_at, updated_at
		FROM terminal_health 
		WHERE deleted_at IS NULL AND (last_heartbeat < $1 OR status = 'OFFLINE')`

	rows, err := r.db.QueryContext(ctx, query, cutoffTime)
	if err != nil {
		return nil, dbSpan.End(err)
	}
	defer rows.Close()

	var healthRecords []*models.TerminalHealth

	for rows.Next() {
		var health models.TerminalHealth
		var diagnosticsJSON []byte

		err := rows.Scan(
			&health.ID,
			&health.TerminalID,
			&health.Status,
			&health.BatteryLevel,
			&health.SignalStrength,
			&health.StorageAvailable,
			&health.StorageTotal,
			&health.MemoryUsage,
			&health.CPUUsage,
			&health.LastHeartbeat,
			&diagnosticsJSON,
			&health.CreatedAt,
			&health.UpdatedAt,
		)
		if err != nil {
			dbSpan.End(err)
			return nil, fmt.Errorf("failed to scan health history row: %w", err)
		}

		if err := json.Unmarshal(diagnosticsJSON, &health.Diagnostics); err != nil {
			dbSpan.End(err)
			return nil, fmt.Errorf("failed to unmarshal diagnostics: %w", err)
		}

		healthRecords = append(healthRecords, &health)
	}

	if err := rows.Err(); err != nil {
		return nil, dbSpan.End(err)
	}

	return healthRecords, dbSpan.End(nil)
}

func (r *healthRepositoryImpl) GetTerminalDiagnostics(ctx context.Context, terminalID uuid.UUID) (*models.TerminalHealth, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminal_health").SetID(terminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, terminal_id, status, battery_level, signal_strength, storage_available,
				storage_total, memory_usage, cpu_usage, last_heartbeat, diagnostics, created_at, updated_at
		FROM terminal_health 
		WHERE deleted_at IS NULL AND terminal_id = $1 
		LIMIT 1`

	var health models.TerminalHealth
	var diagnosticsJSON []byte

	err := r.db.QueryRowContext(ctx, query, terminalID).Scan(
		&health.ID,
		&health.TerminalID,
		&health.Status,
		&health.BatteryLevel,
		&health.SignalStrength,
		&health.StorageAvailable,
		&health.StorageTotal,
		&health.MemoryUsage,
		&health.CPUUsage,
		&health.LastHeartbeat,
		&diagnosticsJSON,
		&health.CreatedAt,
		&health.UpdatedAt,
	)

	if err != nil {
		return nil, dbSpan.End(err)
	}

	if err := json.Unmarshal(diagnosticsJSON, &health.Diagnostics); err != nil {
		dbSpan.End(err)
		return nil, fmt.Errorf("failed to unmarshal diagnostics: %w", err)
	}

	return &health, dbSpan.End(nil)
}

func (r *healthRepositoryImpl) UpdateHeartbeat(ctx context.Context, terminalID uuid.UUID) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "terminal_health").SetID(terminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	now := time.Now()

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to start transaction: %w", err))
	}
	defer tx.Rollback()

	// update health record
	_, err = tx.ExecContext(ctx, `
		UPDATE terminal_health
		SET last_heartbeat = $1, status = $2, updated_at = NOW()
		WHERE terminal_id = $3
	`, now, models.HealthStatusHealthy, terminalID)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to update terminal health: %w", err))
	}

	// update terminal record
	_, err = tx.ExecContext(ctx, `
		UPDATE terminals
		SET last_heartbeat = $1, health_status = $2, is_online = true, updated_at = NOW()
		WHERE id = $3
		`, now, models.HealthStatusHealthy, terminalID)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to update terminals: %w", err))
	}

	if err = tx.Commit(); err != nil {
		return dbSpan.End(fmt.Errorf("failed to update heartbeat: %w", err))
	}

	return dbSpan.End(nil)
}

func (r *healthRepositoryImpl) DeleteTerminalHealth(ctx context.Context, terminalID uuid.UUID) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "terminal_health").SetID(terminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
        UPDATE terminal_health
        SET deleted_at = NOW()
        WHERE terminal_id = $1
    `

	_, err := r.db.ExecContext(ctx, query, terminalID)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to soft delete terminal health: %w", err))
	}

	return dbSpan.End(nil)
}

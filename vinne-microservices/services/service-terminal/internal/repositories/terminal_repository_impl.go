package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/randco/randco-microservices/services/service-terminal/internal/models"
	"github.com/randco/randco-microservices/shared/middleware/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type terminalRepositoryImpl struct {
	db     *sqlx.DB
	tracer trace.Tracer
}

func NewTerminalRepository(db *sqlx.DB) TerminalRepository {
	return &terminalRepositoryImpl{
		db:     db,
		tracer: otel.Tracer("service-terminal.terminal-repository"),
	}
}

func (r *terminalRepositoryImpl) CreateTerminal(ctx context.Context, terminal *models.Terminal) error {
	dbSpan := tracing.TraceDB(ctx, "INSERT", "terminals").
		SetID(fmt.Sprintf("device_id:%s|model:%s", terminal.DeviceID, terminal.Model))
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		INSERT INTO terminals (
			device_id, name, model, serial_number,
			imei, android_version, app_version, vendor,
			purchase_date, status, retailer_id, assignment_date,
			last_sync, last_transaction, health_status, metadata,
			created_at, updated_at, manufacturer
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING id
		`
	metadataJSON, err := json.Marshal(terminal.Metadata)
	if err != nil {
		dbSpan.End(err)
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	err = r.db.QueryRowContext(ctx, query,
		terminal.DeviceID,
		terminal.Name,
		terminal.Model,
		terminal.SerialNumber,
		terminal.IMEI,
		terminal.AndroidVersion,
		terminal.AppVersion,
		terminal.Vendor,
		terminal.PurchaseDate,
		terminal.Status,
		terminal.RetailerID,
		terminal.AssignmentDate,
		terminal.LastSync,
		terminal.LastTransaction,
		terminal.HealthStatus,
		metadataJSON,
		terminal.CreatedAt,
		terminal.UpdatedAt,
		terminal.Manufacturer,
	).Scan(&terminal.ID)

	if err != nil {
		return dbSpan.End(err)
	}

	return dbSpan.End(nil)
}

func (r *terminalRepositoryImpl) GetTerminalByID(ctx context.Context, id uuid.UUID) (*models.Terminal, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminals").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, device_id, name, model, serial_number,
			imei, android_version, app_version, vendor, manufacturer,
			purchase_date, status, retailer_id, assignment_date,
			last_sync, last_transaction, health_status, metadata,
			created_at, updated_at
		FROM terminals
		WHERE id = $1 AND deleted_at IS NULL
	`

	var terminal models.Terminal
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&terminal.ID,
		&terminal.DeviceID,
		&terminal.Name,
		&terminal.Model,
		&terminal.SerialNumber,
		&terminal.IMEI,
		&terminal.AndroidVersion,
		&terminal.AppVersion,
		&terminal.Vendor,
		&terminal.Manufacturer,
		&terminal.PurchaseDate,
		&terminal.Status,
		&terminal.RetailerID,
		&terminal.AssignmentDate,
		&terminal.LastSync,
		&terminal.LastTransaction,
		&terminal.HealthStatus,
		&metadataJSON,
		&terminal.CreatedAt,
		&terminal.UpdatedAt,
	)
	if err != nil {
		return nil, dbSpan.End(err)
	}

	if err := json.Unmarshal(metadataJSON, &terminal.Metadata); err != nil {
		dbSpan.End(err)
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &terminal, dbSpan.End(nil)
}

func (r *terminalRepositoryImpl) GetTerminalByDeviceID(ctx context.Context, deviceID string) (*models.Terminal, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminals").SetID(deviceID)
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, device_id, name, model, serial_number,
			imei, android_version, app_version, vendor, manufacturer,
			purchase_date, status, retailer_id, assignment_date,
			last_sync, last_transaction, health_status, metadata,
			created_at, updated_at
		FROM terminals
		WHERE device_id = $1 AND deleted_at IS NULL
	`

	var terminal models.Terminal
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, deviceID).Scan(
		&terminal.ID,
		&terminal.DeviceID,
		&terminal.Name,
		&terminal.Model,
		&terminal.SerialNumber,
		&terminal.IMEI,
		&terminal.AndroidVersion,
		&terminal.AppVersion,
		&terminal.Vendor,
		&terminal.Manufacturer,
		&terminal.PurchaseDate,
		&terminal.Status,
		&terminal.RetailerID,
		&terminal.AssignmentDate,
		&terminal.LastSync,
		&terminal.LastTransaction,
		&terminal.HealthStatus,
		&metadataJSON,
		&terminal.CreatedAt,
		&terminal.UpdatedAt,
	)
	if err != nil {
		return nil, dbSpan.End(err)
	}

	if err := json.Unmarshal(metadataJSON, &terminal.Metadata); err != nil {
		dbSpan.End(err)
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &terminal, dbSpan.End(nil)
}

func (r *terminalRepositoryImpl) UpdateTerminal(ctx context.Context, terminal *models.Terminal) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "terminals").SetID(terminal.DeviceID)
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		UPDATE terminals
		SET
			device_id = $1,
			name = $2,
			model = $3,
			serial_number = $4,
			imei = $5,
			android_version = $6,
			app_version = $7,
			vendor = $8,
			purchase_date = $9,
			status = $10,
			retailer_id = $11,
			assignment_date = $12,
			last_sync = $13,
			last_transaction = $14,
			health_status = $15,
			metadata = $16,
			updated_at = NOW(),
			manufacturer = $17
		WHERE id = $18
	`

	metadataJSON, err := json.Marshal(terminal.Metadata)
	if err != nil {
		dbSpan.End(err)
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,

		terminal.DeviceID,
		terminal.Name,
		terminal.Model,
		terminal.SerialNumber,
		terminal.IMEI,
		terminal.AndroidVersion,
		terminal.AppVersion,
		terminal.Vendor,
		terminal.PurchaseDate,
		terminal.Status,
		terminal.RetailerID,
		terminal.AssignmentDate,
		terminal.LastSync,
		terminal.LastTransaction,
		terminal.HealthStatus,
		metadataJSON,
		terminal.Manufacturer,
		terminal.ID,
	)

	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to update terminal: %w", err))
	}

	return dbSpan.End(nil)
}

func (r *terminalRepositoryImpl) DeleteTerminal(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "terminals").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `UPDATE terminals
		SET deleted_at = NOW(), deleted_by = $2
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id, deletedBy)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to delete terminal: %w", err))
	}

	return dbSpan.End(nil)
}

func (r *terminalRepositoryImpl) ListTerminals(ctx context.Context, filters TerminalFilters) ([]*models.Terminal, int64, error) {
	query := `SELECT * FROM terminals WHERE 1=1 AND deleted_at IS NULL`
	args := []interface{}{}
	argIndex := 1

	// Apply filters
	if filters.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, filters.Status)
		argIndex++
	}

	if filters.Model != nil {
		query += fmt.Sprintf(" AND model = $%d", argIndex)
		args = append(args, filters.Model)
		argIndex++
	}

	if filters.RetailerID != nil {
		query += fmt.Sprintf(" AND retailer_id = $%d", argIndex)
		args = append(args, filters.RetailerID)
		argIndex++
	}

	if filters.HealthStatus != nil {
		query += fmt.Sprintf(" AND health_status = $%d", argIndex)
		args = append(args, filters.HealthStatus)
		argIndex++
	}

	if filters.SearchTerm != "" {
		search := "%" + strings.ToLower(filters.SearchTerm) + "%"
		query += fmt.Sprintf(` AND (LOWER(name) LIKE $%d OR LOWER(device_id) LIKE $%d OR LOWER(serial_number) LIKE $%d OR LOWER(imei) LIKE $%d
		)`, argIndex, argIndex+1, argIndex+2, argIndex+3)
		args = append(args, search, search, search, search)
		argIndex += 4
	}

	if filters.LastSyncAfter != nil {
		query += fmt.Sprintf(" AND last_sync > $%d", argIndex)
		args = append(args, filters.LastSyncAfter)
		argIndex++
	}

	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS count_query"
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count terminals: %w", err)
	}

	query += " ORDER BY created_at DESC"

	// Apply pagination
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filters.Limit)
		argIndex++
	}
	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filters.Offset)
		argIndex++
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list terminals: %w", err)
	}
	defer rows.Close()

	var terminals []*models.Terminal

	for rows.Next() {
		var terminal models.Terminal
		var metadataJSON []byte
		err := rows.Scan(
			&terminal.ID, &terminal.DeviceID, &terminal.Name, &terminal.Model, &terminal.SerialNumber,
			&terminal.IMEI, &terminal.AndroidVersion, &terminal.AppVersion, &terminal.Vendor,
			&terminal.PurchaseDate, &terminal.Status, &terminal.RetailerID, &terminal.AssignmentDate,
			&terminal.LastSync, &terminal.LastTransaction, &terminal.HealthStatus, &metadataJSON,
			&terminal.CreatedAt, &terminal.UpdatedAt, &terminal.Manufacturer, &terminal.DeletedAt, &terminal.DeletedBy,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan terminals: %w", err)

		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &terminal.Metadata); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		terminals = append(terminals, &terminal)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error reading terminal rows: %w", err)
	}

	return terminals, total, nil
}

func (r *terminalRepositoryImpl) GetTerminalsByRetailerID(ctx context.Context, retailerID uuid.UUID) ([]*models.Terminal, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminals").SetID(retailerID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, device_id, name, model, serial_number,
			imei, android_version, app_version, vendor, manufacturer,
			purchase_date, status, retailer_id, assignment_date,
			last_sync, last_transaction, health_status, metadata,
			created_at, updated_at
		FROM terminals
		WHERE deleted_at IS NULL AND
		retailer_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, retailerID)
	if err != nil {
		return nil, dbSpan.End(err)
	}
	defer rows.Close()

	var terminals []*models.Terminal

	for rows.Next() {
		var terminal models.Terminal
		var metadataJSON []byte

		err := rows.Scan(
			&terminal.ID,
			&terminal.DeviceID,
			&terminal.Name,
			&terminal.Model,
			&terminal.SerialNumber,
			&terminal.IMEI,
			&terminal.AndroidVersion,
			&terminal.AppVersion,
			&terminal.Vendor,
			&terminal.Manufacturer,
			&terminal.PurchaseDate,
			&terminal.Status,
			&terminal.RetailerID,
			&terminal.AssignmentDate,
			&terminal.LastSync,
			&terminal.LastTransaction,
			&terminal.HealthStatus,
			&metadataJSON,
			&terminal.CreatedAt,
			&terminal.UpdatedAt,
		)
		if err != nil {
			dbSpan.End(err)
			return nil, fmt.Errorf("failed to scan terminal row: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &terminal.Metadata); err != nil {
			dbSpan.End(err)
			return nil, fmt.Errorf("failed to unmarshal diagnostics: %w", err)
		}

		terminals = append(terminals, &terminal)
	}

	if err := rows.Err(); err != nil {
		return nil, dbSpan.End(err)
	}

	return terminals, dbSpan.End(nil)
}

func (r *terminalRepositoryImpl) UpdateTerminalStatus(ctx context.Context, id uuid.UUID, status models.TerminalStatus) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "terminals").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		UPDATE terminals
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	res, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to update terminal status: %w", err))
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to get affected rows: %w", err))
	}

	if rows == 0 {
		return dbSpan.End(fmt.Errorf("terminal %s not found", id))
	}

	return dbSpan.End(nil)
}

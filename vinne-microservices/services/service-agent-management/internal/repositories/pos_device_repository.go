package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
)

// POSDeviceRepository defines core POS device operations (max 10 methods)
type POSDeviceRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, device *models.POSDevice) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.POSDevice, error)
	GetByCode(ctx context.Context, deviceCode string) (*models.POSDevice, error)
	GetByIMEI(ctx context.Context, imei string) (*models.POSDevice, error)
	Update(ctx context.Context, device *models.POSDevice) error
	Delete(ctx context.Context, id uuid.UUID) error

	// List operations with filtering
	List(ctx context.Context, filters POSDeviceFilters) ([]models.POSDevice, error)
	Count(ctx context.Context, filters POSDeviceFilters) (int, error)

	// Code generation
	GetNextDeviceCode(ctx context.Context) (string, error)
}

// POSDeviceAssignmentRepository handles device assignment operations
type POSDeviceAssignmentRepository interface {
	AssignToRetailer(ctx context.Context, deviceID, retailerID uuid.UUID, assignedBy string) error
	UnassignFromRetailer(ctx context.Context, deviceID uuid.UUID, unassignedBy string) error
	GetByRetailerID(ctx context.Context, retailerID uuid.UUID) ([]models.POSDevice, error)
	GetAvailableDevices(ctx context.Context) ([]models.POSDevice, error)
}

// POSDeviceStatusRepository handles device status operations
type POSDeviceStatusRepository interface {
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.DeviceStatus, updatedBy string) error
	GetByStatus(ctx context.Context, status models.DeviceStatus) ([]models.POSDevice, error)
}

// POSDeviceMaintenanceRepository handles device maintenance operations
type POSDeviceMaintenanceRepository interface {
	UpdateLastSync(ctx context.Context, id uuid.UUID, syncTime time.Time) error
	UpdateLastTransaction(ctx context.Context, id uuid.UUID, transactionTime time.Time) error
	UpdateSoftwareVersion(ctx context.Context, id uuid.UUID, version string) error
}

// POSDeviceFilters defines filtering options for POS device queries
type POSDeviceFilters struct {
	Status             *models.DeviceStatus
	AssignedRetailerID *uuid.UUID
	Model              *string
	Manufacturer       *string
	SoftwareVersion    *string
	NetworkOperator    *string
	LastSyncAfter      *time.Time
	LastSyncBefore     *time.Time
	AvailableOnly      bool
	AssignedOnly       bool
	Limit              int
	Offset             int
	OrderBy            string
	OrderDirection     string
}

type posDeviceRepository struct {
	db *sqlx.DB
}

// NewPOSDeviceRepository creates a new POS device repository
func NewPOSDeviceRepository(db *sqlx.DB) POSDeviceRepository {
	return &posDeviceRepository{db: db}
}

// NewPOSDeviceAssignmentRepository creates a new POS device assignment repository
func NewPOSDeviceAssignmentRepository(db *sqlx.DB) POSDeviceAssignmentRepository {
	return &posDeviceRepository{db: db}
}

// NewPOSDeviceStatusRepository creates a new POS device status repository
func NewPOSDeviceStatusRepository(db *sqlx.DB) POSDeviceStatusRepository {
	return &posDeviceRepository{db: db}
}

// NewPOSDeviceMaintenanceRepository creates a new POS device maintenance repository
func NewPOSDeviceMaintenanceRepository(db *sqlx.DB) POSDeviceMaintenanceRepository {
	return &posDeviceRepository{db: db}
}

func (r *posDeviceRepository) Create(ctx context.Context, device *models.POSDevice) error {
	if device.ID == uuid.Nil {
		device.ID = uuid.New()
	}

	device.CreatedAt = time.Now()
	device.UpdatedAt = time.Now()

	query := `
		INSERT INTO pos_devices (
			id, device_code, imei, serial_number, model, manufacturer,
			assigned_retailer_id, assignment_date, status, software_version,
			network_operator, sim_card_number, created_at, updated_at, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)`

	_, err := r.db.ExecContext(ctx, query,
		device.ID, device.DeviceCode, device.IMEI, device.SerialNumber,
		device.Model, device.Manufacturer, device.AssignedRetailerID,
		device.AssignmentDate, device.Status, device.SoftwareVersion,
		device.NetworkOperator, device.SIMCardNumber, device.CreatedAt,
		device.UpdatedAt, device.CreatedBy,
	)

	return err
}

func (r *posDeviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.POSDevice, error) {
	var device models.POSDevice
	query := `
		SELECT id, device_code, imei, serial_number, model, manufacturer,
		       assigned_retailer_id, assignment_date, last_sync, last_transaction,
		       status, software_version, network_operator, sim_card_number,
		       created_at, updated_at, created_by, updated_by
		FROM pos_devices
		WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&device.ID, &device.DeviceCode, &device.IMEI, &device.SerialNumber,
		&device.Model, &device.Manufacturer, &device.AssignedRetailerID,
		&device.AssignmentDate, &device.LastSync, &device.LastTransaction,
		&device.Status, &device.SoftwareVersion, &device.NetworkOperator,
		&device.SIMCardNumber, &device.CreatedAt, &device.UpdatedAt,
		&device.CreatedBy, &device.UpdatedBy,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("POS device not found")
		}
		return nil, err
	}

	return &device, nil
}

func (r *posDeviceRepository) GetByCode(ctx context.Context, deviceCode string) (*models.POSDevice, error) {
	var device models.POSDevice
	query := `
		SELECT id, device_code, imei, serial_number, model, manufacturer,
		       assigned_retailer_id, assignment_date, last_sync, last_transaction,
		       status, software_version, network_operator, sim_card_number,
		       created_at, updated_at, created_by, updated_by
		FROM pos_devices
		WHERE device_code = $1`

	err := r.db.QueryRowContext(ctx, query, deviceCode).Scan(
		&device.ID, &device.DeviceCode, &device.IMEI, &device.SerialNumber,
		&device.Model, &device.Manufacturer, &device.AssignedRetailerID,
		&device.AssignmentDate, &device.LastSync, &device.LastTransaction,
		&device.Status, &device.SoftwareVersion, &device.NetworkOperator,
		&device.SIMCardNumber, &device.CreatedAt, &device.UpdatedAt,
		&device.CreatedBy, &device.UpdatedBy,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("POS device not found")
		}
		return nil, err
	}

	return &device, nil
}

func (r *posDeviceRepository) GetByIMEI(ctx context.Context, imei string) (*models.POSDevice, error) {
	var device models.POSDevice
	query := `
		SELECT id, device_code, imei, serial_number, model, manufacturer,
		       assigned_retailer_id, assignment_date, last_sync, last_transaction,
		       status, software_version, network_operator, sim_card_number,
		       created_at, updated_at, created_by, updated_by
		FROM pos_devices
		WHERE imei = $1`

	err := r.db.QueryRowContext(ctx, query, imei).Scan(
		&device.ID, &device.DeviceCode, &device.IMEI, &device.SerialNumber,
		&device.Model, &device.Manufacturer, &device.AssignedRetailerID,
		&device.AssignmentDate, &device.LastSync, &device.LastTransaction,
		&device.Status, &device.SoftwareVersion, &device.NetworkOperator,
		&device.SIMCardNumber, &device.CreatedAt, &device.UpdatedAt,
		&device.CreatedBy, &device.UpdatedBy,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("POS device not found")
		}
		return nil, err
	}

	return &device, nil
}

func (r *posDeviceRepository) Update(ctx context.Context, device *models.POSDevice) error {
	device.UpdatedAt = time.Now()

	query := `
		UPDATE pos_devices SET
			device_code = $2, imei = $3, serial_number = $4, model = $5,
			manufacturer = $6, assigned_retailer_id = $7, assignment_date = $8,
			last_sync = $9, last_transaction = $10, status = $11,
			software_version = $12, network_operator = $13, sim_card_number = $14,
			updated_at = $15, updated_by = $16
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		device.ID, device.DeviceCode, device.IMEI, device.SerialNumber,
		device.Model, device.Manufacturer, device.AssignedRetailerID,
		device.AssignmentDate, device.LastSync, device.LastTransaction,
		device.Status, device.SoftwareVersion, device.NetworkOperator,
		device.SIMCardNumber, device.UpdatedAt, device.UpdatedBy,
	)

	return err
}

func (r *posDeviceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM pos_devices WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *posDeviceRepository) List(ctx context.Context, filters POSDeviceFilters) ([]models.POSDevice, error) {
	query := `
		SELECT id, device_code, imei, serial_number, model, manufacturer,
		       assigned_retailer_id, assignment_date, last_sync, last_transaction,
		       status, software_version, network_operator, sim_card_number,
		       created_at, updated_at, created_by, updated_by
		FROM pos_devices
		WHERE 1=1`

	args := []interface{}{}
	argCounter := 1

	if filters.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argCounter)
		args = append(args, *filters.Status)
		argCounter++
	}

	if filters.AssignedRetailerID != nil {
		query += fmt.Sprintf(" AND assigned_retailer_id = $%d", argCounter)
		args = append(args, *filters.AssignedRetailerID)
		argCounter++
	}

	if filters.Model != nil && *filters.Model != "" {
		query += fmt.Sprintf(" AND model ILIKE $%d", argCounter)
		args = append(args, "%"+*filters.Model+"%")
		argCounter++
	}

	query += " ORDER BY created_at DESC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCounter)
		args = append(args, filters.Limit)
		argCounter++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCounter)
		args = append(args, filters.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var devices []models.POSDevice
	for rows.Next() {
		var device models.POSDevice
		err := rows.Scan(
			&device.ID, &device.DeviceCode, &device.IMEI, &device.SerialNumber,
			&device.Model, &device.Manufacturer, &device.AssignedRetailerID,
			&device.AssignmentDate, &device.LastSync, &device.LastTransaction,
			&device.Status, &device.SoftwareVersion, &device.NetworkOperator,
			&device.SIMCardNumber, &device.CreatedAt, &device.UpdatedAt,
			&device.CreatedBy, &device.UpdatedBy,
		)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}

	return devices, nil
}

func (r *posDeviceRepository) Count(ctx context.Context, filters POSDeviceFilters) (int, error) {
	query := `SELECT COUNT(*) FROM pos_devices WHERE 1=1`

	args := []interface{}{}
	argCounter := 1

	if filters.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argCounter)
		args = append(args, *filters.Status)
		argCounter++
	}

	if filters.AssignedRetailerID != nil {
		query += fmt.Sprintf(" AND assigned_retailer_id = $%d", argCounter)
		args = append(args, *filters.AssignedRetailerID)
		argCounter++
	}

	if filters.Model != nil && *filters.Model != "" {
		query += fmt.Sprintf(" AND model ILIKE $%d", argCounter)
		args = append(args, "%"+*filters.Model+"%")
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *posDeviceRepository) AssignToRetailer(ctx context.Context, deviceID, retailerID uuid.UUID, assignedBy string) error {
	now := time.Now()
	query := `
		UPDATE pos_devices SET
			assigned_retailer_id = $2,
			assignment_date = $3,
			status = $4,
			updated_at = $5,
			updated_by = $6
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		deviceID, retailerID, now, models.DeviceStatusAssigned, now, assignedBy,
	)

	return err
}

func (r *posDeviceRepository) UnassignFromRetailer(ctx context.Context, deviceID uuid.UUID, unassignedBy string) error {
	query := `
		UPDATE pos_devices SET
			assigned_retailer_id = NULL,
			assignment_date = NULL,
			status = $2,
			updated_at = $3,
			updated_by = $4
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		deviceID, models.DeviceStatusAvailable, time.Now(), unassignedBy,
	)

	return err
}

func (r *posDeviceRepository) GetByRetailerID(ctx context.Context, retailerID uuid.UUID) ([]models.POSDevice, error) {
	query := `
		SELECT id, device_code, imei, serial_number, model, manufacturer,
		       assigned_retailer_id, assignment_date, last_sync, last_transaction,
		       status, software_version, network_operator, sim_card_number,
		       created_at, updated_at, created_by, updated_by
		FROM pos_devices
		WHERE assigned_retailer_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, retailerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var devices []models.POSDevice
	for rows.Next() {
		var device models.POSDevice
		err := rows.Scan(
			&device.ID, &device.DeviceCode, &device.IMEI, &device.SerialNumber,
			&device.Model, &device.Manufacturer, &device.AssignedRetailerID,
			&device.AssignmentDate, &device.LastSync, &device.LastTransaction,
			&device.Status, &device.SoftwareVersion, &device.NetworkOperator,
			&device.SIMCardNumber, &device.CreatedAt, &device.UpdatedAt,
			&device.CreatedBy, &device.UpdatedBy,
		)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}

	return devices, nil
}

func (r *posDeviceRepository) GetNextDeviceCode(ctx context.Context) (string, error) {
	var lastCode sql.NullString
	query := `SELECT device_code FROM pos_devices WHERE device_code LIKE 'POS-%' ORDER BY device_code DESC LIMIT 1`

	err := r.db.QueryRowContext(ctx, query).Scan(&lastCode)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}

	year := time.Now().Year()
	var nextNum int

	if lastCode.Valid {
		// Parse the last code to get the number
		_, _ = fmt.Sscanf(lastCode.String, "POS-%d-%d", &year, &nextNum)
		nextNum++
	} else {
		nextNum = 1
	}

	newCode := fmt.Sprintf("POS-%d-%06d", year, nextNum)
	return newCode, nil
}

func (r *posDeviceRepository) GetAvailableDevices(ctx context.Context) ([]models.POSDevice, error) {
	query := `
		SELECT id, device_code, imei, serial_number, model, manufacturer,
		       assigned_retailer_id, assignment_date, last_sync, last_transaction,
		       status, software_version, network_operator, sim_card_number,
		       created_at, updated_at, created_by, updated_by
		FROM pos_devices
		WHERE status = $1 AND assigned_retailer_id IS NULL
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, models.DeviceStatusAvailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var devices []models.POSDevice
	for rows.Next() {
		var device models.POSDevice
		err := rows.Scan(
			&device.ID, &device.DeviceCode, &device.IMEI, &device.SerialNumber,
			&device.Model, &device.Manufacturer, &device.AssignedRetailerID,
			&device.AssignmentDate, &device.LastSync, &device.LastTransaction,
			&device.Status, &device.SoftwareVersion, &device.NetworkOperator,
			&device.SIMCardNumber, &device.CreatedAt, &device.UpdatedAt,
			&device.CreatedBy, &device.UpdatedBy,
		)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}

	return devices, nil
}

func (r *posDeviceRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.DeviceStatus, updatedBy string) error {
	query := `
		UPDATE pos_devices SET
			status = $2,
			updated_at = $3,
			updated_by = $4
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id, status, time.Now(), updatedBy)
	return err
}

func (r *posDeviceRepository) GetByStatus(ctx context.Context, status models.DeviceStatus) ([]models.POSDevice, error) {
	query := `
		SELECT id, device_code, imei, serial_number, model, manufacturer,
		       assigned_retailer_id, assignment_date, last_sync, last_transaction,
		       status, software_version, network_operator, sim_card_number,
		       created_at, updated_at, created_by, updated_by
		FROM pos_devices
		WHERE status = $1
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var devices []models.POSDevice
	for rows.Next() {
		var device models.POSDevice
		err := rows.Scan(
			&device.ID, &device.DeviceCode, &device.IMEI, &device.SerialNumber,
			&device.Model, &device.Manufacturer, &device.AssignedRetailerID,
			&device.AssignmentDate, &device.LastSync, &device.LastTransaction,
			&device.Status, &device.SoftwareVersion, &device.NetworkOperator,
			&device.SIMCardNumber, &device.CreatedAt, &device.UpdatedAt,
			&device.CreatedBy, &device.UpdatedBy,
		)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}

	return devices, nil
}

func (r *posDeviceRepository) UpdateLastSync(ctx context.Context, id uuid.UUID, syncTime time.Time) error {
	query := `
		UPDATE pos_devices SET
			last_sync = $2,
			updated_at = $3
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id, syncTime, time.Now())
	return err
}

func (r *posDeviceRepository) UpdateLastTransaction(ctx context.Context, id uuid.UUID, transactionTime time.Time) error {
	query := `
		UPDATE pos_devices SET
			last_transaction = $2,
			updated_at = $3
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id, transactionTime, time.Now())
	return err
}

func (r *posDeviceRepository) UpdateSoftwareVersion(ctx context.Context, id uuid.UUID, version string) error {
	query := `
		UPDATE pos_devices SET
			software_version = $2,
			updated_at = $3
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id, version, time.Now())
	return err
}

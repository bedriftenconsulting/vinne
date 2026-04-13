package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/randco/service-player/internal/models"
)

type DeviceRepository interface {
	Create(ctx context.Context, req models.CreateDeviceRequest) (*models.PlayerDevice, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.PlayerDevice, error)
	GetByPlayerAndDeviceID(ctx context.Context, playerID uuid.UUID, deviceID string) (*models.PlayerDevice, error)
	GetByPlayerID(ctx context.Context, playerID uuid.UUID) ([]*models.PlayerDevice, error)
	Update(ctx context.Context, req models.UpdateDeviceRequest) (*models.PlayerDevice, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter models.DeviceFilter) ([]*models.PlayerDevice, error)
	Count(ctx context.Context, filter models.DeviceFilter) (int64, error)
	UpdateLastSeen(ctx context.Context, playerID uuid.UUID, deviceID string) error
	UpdateTrustScore(ctx context.Context, playerID uuid.UUID, deviceID string, trustScore int) error
	BlockDevice(ctx context.Context, playerID uuid.UUID, deviceID string, reason string) error
	UnblockDevice(ctx context.Context, playerID uuid.UUID, deviceID string) error
}

type deviceRepository struct {
	db *sql.DB
}

func NewDeviceRepository(db *sql.DB) DeviceRepository {
	return &deviceRepository{db: db}
}

func (r *deviceRepository) Create(ctx context.Context, req models.CreateDeviceRequest) (*models.PlayerDevice, error) {
	query := `
		INSERT INTO player_devices (
			id, player_id, device_id, device_type, device_name,
			os, os_version, app_version, push_token, fingerprint,
			is_trusted, is_blocked, first_seen_at, last_seen_at, trust_score
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		) RETURNING *
	`

	now := time.Now()
	device := &models.PlayerDevice{
		ID:          uuid.New(),
		PlayerID:    req.PlayerID,
		DeviceID:    req.DeviceID,
		DeviceType:  req.DeviceType,
		DeviceName:  req.DeviceName,
		DeviceOS:    req.OS,
		OSVersion:   req.OSVersion,
		AppVersion:  req.AppVersion,
		PushToken:   req.PushToken,
		Fingerprint: req.Fingerprint,
		IsTrusted:   false,
		IsBlocked:   false,
		FirstSeenAt: now,
		LastSeenAt:  now,
		TrustScore:  req.TrustScore,
	}

	err := r.db.QueryRowContext(ctx, query,
		device.ID, device.PlayerID, device.DeviceID, device.DeviceType,
		device.DeviceName, device.DeviceOS, device.OSVersion, device.AppVersion,
		device.PushToken, device.Fingerprint, device.IsTrusted,
		device.IsBlocked, device.FirstSeenAt, device.LastSeenAt, device.TrustScore,
	).Scan(
		&device.ID, &device.PlayerID, &device.DeviceID, &device.DeviceType,
		&device.DeviceName, &device.DeviceOS, &device.OSVersion, &device.AppVersion,
		&device.PushToken, &device.Fingerprint, &device.IsTrusted,
		&device.IsBlocked, &device.FirstSeenAt, &device.LastSeenAt, &device.TrustScore,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create device: %w", err)
	}

	return device, nil
}

func (r *deviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PlayerDevice, error) {
	query := `SELECT * FROM player_devices WHERE id = $1`

	var device models.PlayerDevice
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&device.ID, &device.PlayerID, &device.DeviceID, &device.DeviceType,
		&device.DeviceName, &device.DeviceOS, &device.OSVersion, &device.AppVersion,
		&device.PushToken, &device.Fingerprint, &device.IsTrusted,
		&device.IsBlocked, &device.FirstSeenAt, &device.LastSeenAt, &device.TrustScore,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get device by ID: %w", err)
	}

	return &device, nil
}

func (r *deviceRepository) GetByPlayerAndDeviceID(ctx context.Context, playerID uuid.UUID, deviceID string) (*models.PlayerDevice, error) {
	query := `SELECT * FROM player_devices WHERE player_id = $1 AND device_id = $2`

	var device models.PlayerDevice
	err := r.db.QueryRowContext(ctx, query, playerID, deviceID).Scan(
		&device.ID, &device.PlayerID, &device.DeviceID, &device.DeviceType,
		&device.DeviceName, &device.DeviceOS, &device.OSVersion, &device.AppVersion,
		&device.PushToken, &device.Fingerprint, &device.IsTrusted,
		&device.IsBlocked, &device.FirstSeenAt, &device.LastSeenAt, &device.TrustScore,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get device by player and device ID: %w", err)
	}

	return &device, nil
}

func (r *deviceRepository) GetByPlayerID(ctx context.Context, playerID uuid.UUID) ([]*models.PlayerDevice, error) {
	query := `SELECT * FROM player_devices WHERE player_id = $1 ORDER BY last_seen_at DESC`

	rows, err := r.db.QueryContext(ctx, query, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query devices by player ID: %w", err)
	}
	defer rows.Close()

	var devices []*models.PlayerDevice
	for rows.Next() {
		var device models.PlayerDevice
		err := rows.Scan(
			&device.ID, &device.PlayerID, &device.DeviceID, &device.DeviceType,
			&device.DeviceName, &device.DeviceOS, &device.OSVersion, &device.AppVersion,
			&device.PushToken, &device.Fingerprint, &device.IsTrusted,
			&device.IsBlocked, &device.FirstSeenAt, &device.LastSeenAt, &device.TrustScore,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan device by player ID: %w", err)
		}
		devices = append(devices, &device)
	}

	return devices, nil
}

func (r *deviceRepository) Update(ctx context.Context, req models.UpdateDeviceRequest) (*models.PlayerDevice, error) {
	setParts := []string{}
	args := []interface{}{req.ID}
	argIndex := 2

	if req.DeviceName != "" {
		setParts = append(setParts, fmt.Sprintf("device_name = $%d", argIndex))
		args = append(args, req.DeviceName)
		argIndex++
	}
	if req.PushToken != "" {
		setParts = append(setParts, fmt.Sprintf("push_token = $%d", argIndex))
		args = append(args, req.PushToken)
		argIndex++
	}
	if req.Fingerprint != "" {
		setParts = append(setParts, fmt.Sprintf("fingerprint = $%d", argIndex))
		args = append(args, req.Fingerprint)
		argIndex++
	}
	setParts = append(setParts, fmt.Sprintf("is_trusted = $%d", argIndex))
	args = append(args, req.IsTrusted)
	argIndex++
	setParts = append(setParts, fmt.Sprintf("is_blocked = $%d", argIndex))
	args = append(args, req.IsBlocked)
	argIndex++
	if !req.LastSeenAt.IsZero() {
		setParts = append(setParts, fmt.Sprintf("last_seen_at = $%d", argIndex))
		args = append(args, req.LastSeenAt)
		argIndex++
	}
	if req.TrustScore > 0 {
		setParts = append(setParts, fmt.Sprintf("trust_score = $%d", argIndex))
		args = append(args, req.TrustScore)
		argIndex++
	}

	if len(setParts) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	query := fmt.Sprintf(`
		UPDATE player_devices 
		SET %s 
		WHERE id = $1 
		RETURNING *
	`, strings.Join(setParts, ", "))

	var device models.PlayerDevice
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&device.ID, &device.PlayerID, &device.DeviceID, &device.DeviceType,
		&device.DeviceName, &device.DeviceOS, &device.OSVersion, &device.AppVersion,
		&device.PushToken, &device.Fingerprint, &device.IsTrusted,
		&device.IsBlocked, &device.FirstSeenAt, &device.LastSeenAt, &device.TrustScore,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("failed to update device: %w", err)
		}
		return nil, fmt.Errorf("failed to update device: %w", err)
	}
	return &device, nil
}

func (r *deviceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM player_devices WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device not found")
	}

	return nil
}

func (r *deviceRepository) List(ctx context.Context, filter models.DeviceFilter) ([]*models.PlayerDevice, error) {
	query := `SELECT * FROM player_devices WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if filter.PlayerID != uuid.Nil {
		query += fmt.Sprintf(" AND player_id = $%d", argIndex)
		args = append(args, filter.PlayerID)
		argIndex++
	}
	if filter.DeviceID != "" {
		query += fmt.Sprintf(" AND device_id = $%d", argIndex)
		args = append(args, filter.DeviceID)
		argIndex++
	}
	if filter.DeviceType != "" {
		query += fmt.Sprintf(" AND device_type = $%d", argIndex)
		args = append(args, filter.DeviceType)
		argIndex++
	}
	if !filter.IsTrusted {
		query += fmt.Sprintf(" AND is_trusted = $%d", argIndex)
		args = append(args, filter.IsTrusted)
		argIndex++
	}
	if !filter.IsBlocked {
		query += fmt.Sprintf(" AND is_blocked = $%d", argIndex)
		args = append(args, filter.IsBlocked)
		argIndex++
	}
	if filter.TrustScoreMin > 0 {
		query += fmt.Sprintf(" AND trust_score >= $%d", argIndex)
		args = append(args, filter.TrustScoreMin)
		argIndex++
	}
	if filter.TrustScoreMax > 0 {
		query += fmt.Sprintf(" AND trust_score <= $%d", argIndex)
		args = append(args, filter.TrustScoreMax)
		argIndex++
	}

	query += " ORDER BY last_seen_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
		argIndex++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filter.Offset)
	}

	var devices []*models.PlayerDevice
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var device models.PlayerDevice
		err := rows.Scan(
			&device.ID, &device.PlayerID, &device.DeviceID, &device.DeviceType,
			&device.DeviceName, &device.DeviceOS, &device.OSVersion, &device.AppVersion,
			&device.PushToken, &device.Fingerprint, &device.IsTrusted,
			&device.IsBlocked, &device.FirstSeenAt, &device.LastSeenAt, &device.TrustScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan device: %w", err)
		}
		devices = append(devices, &device)
	}

	return devices, nil
}

func (r *deviceRepository) Count(ctx context.Context, filter models.DeviceFilter) (int64, error) {
	query := `SELECT COUNT(*) FROM player_devices WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if filter.PlayerID != uuid.Nil {
		query += fmt.Sprintf(" AND player_id = $%d", argIndex)
		args = append(args, filter.PlayerID)
		argIndex++
	}
	if filter.DeviceID != "" {
		query += fmt.Sprintf(" AND device_id = $%d", argIndex)
		args = append(args, filter.DeviceID)
		argIndex++
	}
	if filter.DeviceType != "" {
		query += fmt.Sprintf(" AND device_type = $%d", argIndex)
		args = append(args, filter.DeviceType)
		argIndex++
	}
	if !filter.IsTrusted {
		query += fmt.Sprintf(" AND is_trusted = $%d", argIndex)
		args = append(args, filter.IsTrusted)
		argIndex++
	}
	if !filter.IsBlocked {
		query += fmt.Sprintf(" AND is_blocked = $%d", argIndex)
		args = append(args, filter.IsBlocked)
		argIndex++
	}
	if filter.TrustScoreMin > 0 {
		query += fmt.Sprintf(" AND trust_score >= $%d", argIndex)
		args = append(args, filter.TrustScoreMin)
		argIndex++
	}
	if filter.TrustScoreMax > 0 {
		query += fmt.Sprintf(" AND trust_score <= $%d", argIndex)
		args = append(args, filter.TrustScoreMax)
		argIndex++
	}

	var count int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count devices: %w", err)
	}

	return count, nil
}

func (r *deviceRepository) UpdateLastSeen(ctx context.Context, playerID uuid.UUID, deviceID string) error {
	query := `UPDATE player_devices SET last_seen_at = $3 WHERE player_id = $1 AND device_id = $2`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, playerID, deviceID, now)
	if err != nil {
		return fmt.Errorf("failed to update last seen: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device not found")
	}

	return nil
}

func (r *deviceRepository) UpdateTrustScore(ctx context.Context, playerID uuid.UUID, deviceID string, trustScore int) error {
	query := `UPDATE player_devices SET trust_score = $3 WHERE player_id = $1 AND device_id = $2`

	result, err := r.db.ExecContext(ctx, query, playerID, deviceID, trustScore)
	if err != nil {
		return fmt.Errorf("failed to update trust score: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device not found")
	}

	return nil
}

func (r *deviceRepository) BlockDevice(ctx context.Context, playerID uuid.UUID, deviceID string, reason string) error {
	query := `UPDATE player_devices SET is_blocked = true WHERE player_id = $1 AND device_id = $2`

	result, err := r.db.ExecContext(ctx, query, playerID, deviceID)
	if err != nil {
		return fmt.Errorf("failed to block device: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device not found")
	}

	return nil
}

func (r *deviceRepository) UnblockDevice(ctx context.Context, playerID uuid.UUID, deviceID string) error {
	query := `UPDATE player_devices SET is_blocked = false WHERE player_id = $1 AND device_id = $2`

	result, err := r.db.ExecContext(ctx, query, playerID, deviceID)
	if err != nil {
		return fmt.Errorf("failed to unblock device: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device not found")
	}

	return nil
}

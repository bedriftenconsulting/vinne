package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/randco/randco-microservices/services/service-notification/internal/database"
	"github.com/randco/randco-microservices/services/service-notification/internal/models"
)

// DeviceTokenRepository defines the interface for device token data operations
type DeviceTokenRepository interface {
	Create(ctx context.Context, req *models.CreateDeviceTokenRequest) (*models.DeviceToken, error)
	GetByDeviceID(ctx context.Context, deviceID string) (*models.DeviceToken, error)
	GetByRetailerID(ctx context.Context, retailerID string) ([]*models.DeviceToken, error)
	GetActiveTokensByRetailerID(ctx context.Context, retailerID string) ([]*models.DeviceToken, error)
	GetAllActiveRetailerIDs(ctx context.Context) ([]string, error)
	UpdateToken(ctx context.Context, deviceID string, fcmToken string, appVersion string) error
	MarkAsInactive(ctx context.Context, deviceID string) error
	UpdateLastUsed(ctx context.Context, deviceID string) error
}

type deviceTokenRepository struct {
	db database.DBInterface
}

// NewDeviceTokenRepository creates a new device token repository
func NewDeviceTokenRepository(rawDB *sql.DB) DeviceTokenRepository {
	return &deviceTokenRepository{db: database.NewTracedDBInterface(rawDB)}
}

// Create creates a new device token or updates existing one
func (r *deviceTokenRepository) Create(ctx context.Context, req *models.CreateDeviceTokenRequest) (*models.DeviceToken, error) {
	query := `
		INSERT INTO device_tokens (
			retailer_id, device_id, fcm_token, platform, app_version, is_active
		) VALUES ($1, $2, $3, $4, $5, true)
		ON CONFLICT (device_id)
		DO UPDATE SET
			fcm_token = EXCLUDED.fcm_token,
			app_version = EXCLUDED.app_version,
			is_active = true,
			updated_at = NOW()
		RETURNING id, retailer_id, device_id, fcm_token, platform, app_version, is_active, last_used_at, created_at, updated_at
	`

	var token models.DeviceToken
	var appVersion sql.NullString
	var lastUsedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query,
		req.RetailerID, req.DeviceID, req.FCMToken, req.Platform, sql.NullString{String: req.AppVersion, Valid: req.AppVersion != ""}).
		Scan(&token.ID, &token.RetailerID, &token.DeviceID, &token.FCMToken, &token.Platform,
			&appVersion, &token.IsActive, &lastUsedAt, &token.CreatedAt, &token.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create device token: %w", err)
	}

	if appVersion.Valid {
		token.AppVersion = &appVersion.String
	}
	if lastUsedAt.Valid {
		token.LastUsedAt = &lastUsedAt.Time
	}

	return &token, nil
}

// GetByDeviceID retrieves a device token by device ID
func (r *deviceTokenRepository) GetByDeviceID(ctx context.Context, deviceID string) (*models.DeviceToken, error) {
	query := `
		SELECT id, retailer_id, device_id, fcm_token, platform, app_version, is_active, last_used_at, created_at, updated_at
		FROM device_tokens
		WHERE device_id = $1
	`

	var token models.DeviceToken
	var appVersion sql.NullString
	var lastUsedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, deviceID).
		Scan(&token.ID, &token.RetailerID, &token.DeviceID, &token.FCMToken, &token.Platform,
			&appVersion, &token.IsActive, &lastUsedAt, &token.CreatedAt, &token.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get device token: %w", err)
	}

	if appVersion.Valid {
		token.AppVersion = &appVersion.String
	}
	if lastUsedAt.Valid {
		token.LastUsedAt = &lastUsedAt.Time
	}

	return &token, nil
}

// GetByRetailerID retrieves all device tokens for a retailer
func (r *deviceTokenRepository) GetByRetailerID(ctx context.Context, retailerID string) ([]*models.DeviceToken, error) {
	query := `
		SELECT id, retailer_id, device_id, fcm_token, platform, app_version, is_active, last_used_at, created_at, updated_at
		FROM device_tokens
		WHERE retailer_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, retailerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get retailer device tokens: %w", err)
	}
	defer func() {
		_ = rows.Close() // Ignore error in defer cleanup
	}()

	var tokens []*models.DeviceToken
	for rows.Next() {
		var token models.DeviceToken
		var appVersion sql.NullString
		var lastUsedAt sql.NullTime

		if err := rows.Scan(&token.ID, &token.RetailerID, &token.DeviceID, &token.FCMToken, &token.Platform,
			&appVersion, &token.IsActive, &lastUsedAt, &token.CreatedAt, &token.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan device token: %w", err)
		}

		if appVersion.Valid {
			token.AppVersion = &appVersion.String
		}
		if lastUsedAt.Valid {
			token.LastUsedAt = &lastUsedAt.Time
		}

		tokens = append(tokens, &token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating device token rows: %w", err)
	}

	return tokens, nil
}

// GetActiveTokensByRetailerID retrieves all active device tokens for a retailer
func (r *deviceTokenRepository) GetActiveTokensByRetailerID(ctx context.Context, retailerID string) ([]*models.DeviceToken, error) {
	query := `
		SELECT id, retailer_id, device_id, fcm_token, platform, app_version, is_active, last_used_at, created_at, updated_at
		FROM device_tokens
		WHERE retailer_id = $1 AND is_active = true
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, retailerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active device tokens: %w", err)
	}
	defer func() {
		_ = rows.Close() // Ignore error in defer cleanup
	}()

	var tokens []*models.DeviceToken
	for rows.Next() {
		var token models.DeviceToken
		var appVersion sql.NullString
		var lastUsedAt sql.NullTime

		if err := rows.Scan(&token.ID, &token.RetailerID, &token.DeviceID, &token.FCMToken, &token.Platform,
			&appVersion, &token.IsActive, &lastUsedAt, &token.CreatedAt, &token.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan device token: %w", err)
		}

		if appVersion.Valid {
			token.AppVersion = &appVersion.String
		}
		if lastUsedAt.Valid {
			token.LastUsedAt = &lastUsedAt.Time
		}

		tokens = append(tokens, &token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating active device token rows: %w", err)
	}

	return tokens, nil
}

// UpdateToken updates FCM token and app version for a device
func (r *deviceTokenRepository) UpdateToken(ctx context.Context, deviceID string, fcmToken string, appVersion string) error {
	query := `
		UPDATE device_tokens
		SET fcm_token = $2, app_version = $3, is_active = true, updated_at = NOW()
		WHERE device_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, deviceID, fcmToken, appVersion)
	if err != nil {
		return fmt.Errorf("failed to update device token: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("device token not found")
	}

	return nil
}

// MarkAsInactive marks a device token as inactive
func (r *deviceTokenRepository) MarkAsInactive(ctx context.Context, deviceID string) error {
	query := `
		UPDATE device_tokens
		SET is_active = false, updated_at = NOW()
		WHERE device_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, deviceID)
	if err != nil {
		return fmt.Errorf("failed to mark device token as inactive: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("device token not found")
	}

	return nil
}

// GetAllActiveRetailerIDs retrieves all unique retailer IDs that have active device tokens
func (r *deviceTokenRepository) GetAllActiveRetailerIDs(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT retailer_id
		FROM device_tokens
		WHERE is_active = true
		ORDER BY retailer_id
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active retailer IDs: %w", err)
	}
	defer func() {
		_ = rows.Close() // Ignore error in defer cleanup
	}()

	var retailerIDs []string
	for rows.Next() {
		var retailerID string
		if err := rows.Scan(&retailerID); err != nil {
			return nil, fmt.Errorf("failed to scan retailer ID: %w", err)
		}
		retailerIDs = append(retailerIDs, retailerID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating retailer ID rows: %w", err)
	}

	return retailerIDs, nil
}

// UpdateLastUsed updates the last_used_at timestamp for a device token
func (r *deviceTokenRepository) UpdateLastUsed(ctx context.Context, deviceID string) error {
	query := `
		UPDATE device_tokens
		SET last_used_at = $2, updated_at = NOW()
		WHERE device_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, deviceID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update last used timestamp: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("device token not found")
	}

	return nil
}

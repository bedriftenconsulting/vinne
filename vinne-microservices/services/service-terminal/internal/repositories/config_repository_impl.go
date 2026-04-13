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

type configRepositoryImpl struct {
	db     *sqlx.DB
	tracer trace.Tracer
}

func NewTerminalConfigRepository(db *sqlx.DB) TerminalConfigRepository {
	return &configRepositoryImpl{
		db:     db,
		tracer: otel.Tracer("service-terminal.config-repository"),
	}
}

func (r *configRepositoryImpl) CreateConfig(ctx context.Context, config *models.TerminalConfig) error {
	dbSpan := tracing.TraceDB(ctx, "INSERT", "terminal_configs").SetID(config.TerminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		INSERT INTO terminal_configs (
			terminal_id, transaction_limit, daily_limit, offline_mode_enabled,
			offline_sync_interval, auto_update_enabled, minimum_app_version, settings,
			created_at, updated_at)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()
		)
		RETURNING id
		`
	settingsJSON, err := json.Marshal(config.Settings)
	if err != nil {
		dbSpan.End(err)
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	err = r.db.QueryRowContext(ctx, query,
		config.TerminalID,
		config.TransactionLimit,
		config.DailyLimit,
		config.OfflineModeEnabled,
		config.OfflineSyncInterval,
		config.AutoUpdateEnabled,
		config.MinimumAppVersion,
		settingsJSON,
	).Scan(&config.ID)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to create terminal config: %w", err))
	}

	return dbSpan.End(nil)
}

func (r *configRepositoryImpl) GetConfigByTerminalID(ctx context.Context, terminalID uuid.UUID) (*models.TerminalConfig, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminal_configs").SetID(terminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, terminal_id, transaction_limit, daily_limit, offline_mode_enabled,
			offline_sync_interval, auto_update_enabled, minimum_app_version, settings,
			created_at, updated_at
		FROM terminal_configs
		WHERE terminal_id = $1 AND deleted_at IS NULL
		LIMIT 1`

	var config models.TerminalConfig
	var settingsJSON []byte

	err := r.db.QueryRowContext(ctx, query, terminalID).Scan(
		&config.ID,
		&config.TerminalID,
		&config.TransactionLimit,
		&config.DailyLimit,
		&config.OfflineModeEnabled,
		&config.OfflineSyncInterval,
		&config.AutoUpdateEnabled,
		&config.MinimumAppVersion,
		&settingsJSON,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err != nil {
		return nil, dbSpan.End(err)
	}

	if err := json.Unmarshal(settingsJSON, &config.Settings); err != nil {
		dbSpan.End(err)
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	return &config, dbSpan.End(nil)
}

func (r *configRepositoryImpl) UpdateConfig(ctx context.Context, config *models.TerminalConfig) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "terminal_configs").SetID(config.TerminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		UPDATE terminal_configs
		SET 
			transaction_limit = $1,
			daily_limit = $2,
			offline_mode_enabled = $3,
			offline_sync_interval = $4,
			auto_update_enabled = $5,
			minimum_app_version = $6,
			settings = $7,
			updated_at = NOW()
		WHERE terminal_id = $8
		`

	settingsJSON, err := json.Marshal(config.Settings)
	if err != nil {
		dbSpan.End(err)
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		config.TransactionLimit,
		config.DailyLimit,
		config.OfflineModeEnabled,
		config.OfflineSyncInterval,
		config.AutoUpdateEnabled,
		config.MinimumAppVersion,
		settingsJSON,
		config.TerminalID,
	)

	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to update terminal config: %w", err))
	}

	return dbSpan.End(nil)
}

func (r *configRepositoryImpl) DeleteConfig(ctx context.Context, terminalID uuid.UUID) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "terminal_configs").SetID(terminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `UPDATE terminal_configs 
		SET deleted_at = NOW() 
		WHERE terminal_id = $1`

	_, err := r.db.ExecContext(ctx, query, terminalID)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to delete config: %w", err))
	}

	return dbSpan.End(nil)
}

func (r *configRepositoryImpl) GetDefaultConfig(ctx context.Context) (*models.TerminalConfig, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminal_configs").SetID("default")
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	// Return default configuration
	defaultConfig := &models.TerminalConfig{
		TransactionLimit:    10000,
		DailyLimit:          100000,
		OfflineModeEnabled:  true,
		OfflineSyncInterval: 30,
		AutoUpdateEnabled:   true,
		Settings:            map[string]string{},
	}

	return defaultConfig, dbSpan.End(nil)
}

func (r *configRepositoryImpl) BulkUpdateConfigs(ctx context.Context, terminalIDs []uuid.UUID, updates map[string]interface{}) error {
	if len(terminalIDs) == 0 {
		return nil
	}

	dbSpan := tracing.TraceDB(ctx, "UPDATE", "terminal_configs").SetID("bulk_update")
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	// Build SET clause dynamically
	setParts := []string{}
	args := []interface{}{}
	i := 1
	for col, val := range updates {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}

	setClause := strings.Join(setParts, ", ")

	// terminal_ids slice will be the last argument
	args = append(args, terminalIDs)

	query := fmt.Sprintf(`
		UPDATE terminal_configs
		SET %s, updated_at = NOW()
		WHERE terminal_id = ANY($%d)
	`, setClause, i)

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to bulk update terminal configs: %w", err))
	}

	return dbSpan.End(nil)
}

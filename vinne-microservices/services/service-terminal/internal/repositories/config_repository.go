package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-terminal/internal/models"
)

type TerminalConfigRepository interface {
	CreateConfig(ctx context.Context, config *models.TerminalConfig) error
	GetConfigByTerminalID(ctx context.Context, terminalID uuid.UUID) (*models.TerminalConfig, error)
	UpdateConfig(ctx context.Context, config *models.TerminalConfig) error
	DeleteConfig(ctx context.Context, terminalID uuid.UUID) error
	GetDefaultConfig(ctx context.Context) (*models.TerminalConfig, error)
	BulkUpdateConfigs(ctx context.Context, terminalIDs []uuid.UUID, updates map[string]interface{}) error
}

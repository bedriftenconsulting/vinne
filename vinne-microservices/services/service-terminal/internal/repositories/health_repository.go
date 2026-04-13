package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-terminal/internal/models"
)

type TerminalHealthRepository interface {
	CreateOrUpdateHealth(ctx context.Context, health *models.TerminalHealth) error
	GetHealthByTerminalID(ctx context.Context, terminalID uuid.UUID) (*models.TerminalHealth, error)
	RecordHealthHistory(ctx context.Context, history *models.TerminalHealthHistory) error
	GetHealthHistory(ctx context.Context, terminalID uuid.UUID, fromTime time.Time, toTime time.Time) ([]*models.TerminalHealthHistory, error)
	GetUnhealthyTerminals(ctx context.Context) ([]*models.TerminalHealth, error)
	GetOfflineTerminals(ctx context.Context, threshold time.Duration) ([]*models.TerminalHealth, error)
	GetTerminalDiagnostics(ctx context.Context, terminalID uuid.UUID) (*models.TerminalHealth, error)
	UpdateHeartbeat(ctx context.Context, terminalID uuid.UUID) error
	DeleteTerminalHealth(ctx context.Context, terminalID uuid.UUID) error
}

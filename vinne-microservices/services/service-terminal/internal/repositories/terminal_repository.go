package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-terminal/internal/models"
)

type TerminalRepository interface {
	CreateTerminal(ctx context.Context, terminal *models.Terminal) error
	GetTerminalByID(ctx context.Context, id uuid.UUID) (*models.Terminal, error)
	GetTerminalByDeviceID(ctx context.Context, deviceID string) (*models.Terminal, error)
	UpdateTerminal(ctx context.Context, terminal *models.Terminal) error
	DeleteTerminal(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error
	ListTerminals(ctx context.Context, filters TerminalFilters) ([]*models.Terminal, int64, error)
	GetTerminalsByRetailerID(ctx context.Context, retailerID uuid.UUID) ([]*models.Terminal, error)
	UpdateTerminalStatus(ctx context.Context, id uuid.UUID, status models.TerminalStatus) error
}

type TerminalFilters struct {
	Status        *models.TerminalStatus
	Model         *models.TerminalModel
	RetailerID    *uuid.UUID
	HealthStatus  *models.HealthStatus
	SearchTerm    string
	LastSyncAfter *time.Time
	Limit         int
	Offset        int
}

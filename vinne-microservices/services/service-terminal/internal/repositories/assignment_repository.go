package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-terminal/internal/models"
)

type TerminalAssignmentRepository interface {
	CreateAssignment(ctx context.Context, assignment *models.TerminalAssignment) error
	GetAssignmentByID(ctx context.Context, id uuid.UUID) (*models.TerminalAssignment, error)
	GetActiveAssignmentByTerminalID(ctx context.Context, terminalID uuid.UUID) (*models.TerminalAssignment, error)
	GetActiveAssignmentByRetailerID(ctx context.Context, retailerID uuid.UUID) (*models.TerminalAssignment, error)
	UnassignTerminal(ctx context.Context, terminalID uuid.UUID, unassignedBy uuid.UUID, notes string) error
	GetAssignmentHistory(ctx context.Context, terminalID uuid.UUID) ([]*models.TerminalAssignment, error)
	ListAssignments(ctx context.Context, filters AssignmentFilters) ([]*models.TerminalAssignment, int64, error)
}

type AssignmentFilters struct {
	TerminalID *uuid.UUID
	RetailerID *uuid.UUID
	IsActive   *bool
	AssignedBy *uuid.UUID
	DateFrom   *time.Time
	DateTo     *time.Time
	Limit      int
	Offset     int
}

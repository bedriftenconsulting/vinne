package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-terminal/internal/models"
	"github.com/randco/randco-microservices/services/service-terminal/internal/repositories"
	"github.com/randco/randco-microservices/shared/common/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TerminalService interface defines terminal business operations
type TerminalService interface {
	RegisterTerminal(ctx context.Context, terminal *models.Terminal) error
	GetTerminal(ctx context.Context, terminalID uuid.UUID) (*models.Terminal, error)
	GetTerminalByDeviceID(ctx context.Context, deviceID string) (*models.Terminal, error)
	UpdateTerminal(ctx context.Context, terminal *models.Terminal) error
	ListTerminals(ctx context.Context, filter TerminalFilter) ([]*models.Terminal, int64, error)
	DeleteTerminal(ctx context.Context, terminalID uuid.UUID, deletedBy uuid.UUID) error
	AssignTerminalToRetailer(ctx context.Context, terminalID, retailerID uuid.UUID, assignedBy uuid.UUID) error
	UnassignTerminal(ctx context.Context, terminalID uuid.UUID, unassignedBy uuid.UUID, reason string) error
	GetTerminalsByRetailer(ctx context.Context, retailerID uuid.UUID) ([]*models.Terminal, error)
	UpdateTerminalStatus(ctx context.Context, terminalID uuid.UUID, status models.TerminalStatus) error
	GetTerminalHealth(ctx context.Context, terminalID uuid.UUID) (*models.TerminalHealth, error)
	UpdateTerminalHealth(ctx context.Context, terminalID uuid.UUID, health *models.TerminalHealth) error
	GetTerminalDiagnostics(ctx context.Context, terminalID uuid.UUID) (*models.TerminalHealth, error)
	UpdateHeartbeat(ctx context.Context, terminalID uuid.UUID) error
	GetTerminalConfig(ctx context.Context, terminalID uuid.UUID) (*models.TerminalConfig, error)
	UpdateTerminalConfig(ctx context.Context, config *models.TerminalConfig) error
	GetDashboardStats(ctx context.Context) (*DashboardStats, error)
	GetOfflineTerminals(ctx context.Context, threshold time.Duration) ([]*models.Terminal, error)
	GetActiveAssignment(ctx context.Context, terminalID uuid.UUID) (*models.TerminalAssignment, error)
}

// TerminalFilter represents filtering options for terminal listing
type TerminalFilter struct {
	Status        *models.TerminalStatus
	Model         *models.TerminalModel
	RetailerID    *uuid.UUID
	HealthStatus  *models.HealthStatus
	SearchTerm    string
	LastSyncAfter *time.Time
	Page          int
	PageSize      int
	SortBy        string
	SortDesc      bool
}

// DashboardStats represents terminal dashboard statistics
type DashboardStats struct {
	TotalTerminals    int64            `json:"total_terminals"`
	ActiveTerminals   int64            `json:"active_terminals"`
	OnlineTerminals   int64            `json:"online_terminals"`
	OfflineTerminals  int64            `json:"offline_terminals"`
	HealthyTerminals  int64            `json:"healthy_terminals"`
	WarningTerminals  int64            `json:"warning_terminals"`
	CriticalTerminals int64            `json:"critical_terminals"`
	StatusBreakdown   map[string]int64 `json:"status_breakdown"`
	ModelBreakdown    map[string]int64 `json:"model_breakdown"`
}

// terminalService implements TerminalService interface
type terminalService struct {
	terminalRepo   repositories.TerminalRepository
	assignmentRepo repositories.TerminalAssignmentRepository
	healthRepo     repositories.TerminalHealthRepository
	configRepo     repositories.TerminalConfigRepository
	logger         logger.Logger
	tracer         trace.Tracer
}

// NewTerminalService creates a new terminal service instance
func NewTerminalService(
	terminalRepo repositories.TerminalRepository,
	assignmentRepo repositories.TerminalAssignmentRepository,
	healthRepo repositories.TerminalHealthRepository,
	configRepo repositories.TerminalConfigRepository,
	logger logger.Logger,
) TerminalService {
	return &terminalService{
		terminalRepo:   terminalRepo,
		assignmentRepo: assignmentRepo,
		healthRepo:     healthRepo,
		configRepo:     configRepo,
		logger:         logger,
		tracer:         otel.Tracer("service-terminal.terminal-service"),
	}
}

// RegisterTerminal registers a new terminal
func (s *terminalService) RegisterTerminal(ctx context.Context, terminal *models.Terminal) error {
	ctx, span := s.tracer.Start(ctx, "terminal_service.register_terminal")
	defer span.End()

	span.SetAttributes(
		attribute.String("device_id", terminal.DeviceID),
		attribute.String("model", string(terminal.Model)),
	)

	// Create terminal
	if err := s.terminalRepo.CreateTerminal(ctx, terminal); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to register terminal: %w", err)
	}

	// Create default configuration for the terminal
	config := &models.TerminalConfig{
		TerminalID:          terminal.ID,
		TransactionLimit:    10000,
		DailyLimit:          100000,
		OfflineModeEnabled:  true,
		OfflineSyncInterval: 30,
		AutoUpdateEnabled:   true,
		Settings:            make(map[string]string),
	}

	if err := s.configRepo.CreateConfig(ctx, config); err != nil {
		s.logger.Error("Failed to create terminal config", "error", err, "terminal_id", terminal.ID)
		span.RecordError(err)
	}

	// Create initial health record
	health := &models.TerminalHealth{
		TerminalID:    terminal.ID,
		Status:        models.HealthStatusOffline,
		LastHeartbeat: time.Now(),
		Diagnostics:   make(map[string]string),
	}

	if err := s.healthRepo.CreateOrUpdateHealth(ctx, health); err != nil {
		s.logger.Error("Failed to create terminal health record", "error", err, "terminal_id", terminal.ID)
		span.RecordError(err)
	}

	s.logger.Info("Terminal registered successfully", "terminal_id", terminal.ID, "device_id", terminal.DeviceID)
	return nil
}

// GetTerminal retrieves a terminal by ID
func (s *terminalService) GetTerminal(ctx context.Context, terminalID uuid.UUID) (*models.Terminal, error) {
	ctx, span := s.tracer.Start(ctx, "terminal_service.get_terminal")
	defer span.End()

	span.SetAttributes(attribute.String("terminal_id", terminalID.String()))

	terminal, err := s.terminalRepo.GetTerminalByID(ctx, terminalID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get terminal: %w", err)
	}

	return terminal, nil
}

// GetTerminalByDeviceID retrieves a terminal by device ID
func (s *terminalService) GetTerminalByDeviceID(ctx context.Context, deviceID string) (*models.Terminal, error) {
	ctx, span := s.tracer.Start(ctx, "terminal_service.get_terminal_by_device_id")
	defer span.End()

	span.SetAttributes(attribute.String("device_id", deviceID))

	terminal, err := s.terminalRepo.GetTerminalByDeviceID(ctx, deviceID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get terminal by device ID: %w", err)
	}

	return terminal, nil
}

func (s *terminalService) GetActiveAssignment(ctx context.Context, terminalID uuid.UUID) (*models.TerminalAssignment, error) {
	{
		ctx, span := s.tracer.Start(ctx, "terminal_service.get_active_assignment")
		defer span.End()

		span.SetAttributes(attribute.String("terminal_id", terminalID.String()))

		assignment, err := s.assignmentRepo.GetActiveAssignmentByTerminalID(ctx, terminalID)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to get active assignment: %w", err)
		}

		return assignment, nil
	}

}

// UpdateTerminal updates a terminal's information
func (s *terminalService) UpdateTerminal(ctx context.Context, terminal *models.Terminal) error {
	ctx, span := s.tracer.Start(ctx, "terminal_service.update_terminal")
	defer span.End()

	span.SetAttributes(
		attribute.String("terminal_id", terminal.ID.String()),
		attribute.String("device_id", terminal.DeviceID),
	)

	if err := s.terminalRepo.UpdateTerminal(ctx, terminal); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update terminal: %w", err)
	}

	s.logger.Info("Terminal updated successfully", "terminal_id", terminal.ID)
	return nil
}

// ListTerminals lists terminals with filtering and pagination
func (s *terminalService) ListTerminals(ctx context.Context, filter TerminalFilter) ([]*models.Terminal, int64, error) {
	ctx, span := s.tracer.Start(ctx, "terminal_service.list_terminals")
	defer span.End()

	// Convert service filter to repository filter
	repoFilters := repositories.TerminalFilters{
		Status:        filter.Status,
		Model:         filter.Model,
		RetailerID:    filter.RetailerID,
		HealthStatus:  filter.HealthStatus,
		SearchTerm:    filter.SearchTerm,
		LastSyncAfter: filter.LastSyncAfter,
	}

	// Apply pagination
	if filter.PageSize > 0 {
		repoFilters.Limit = filter.PageSize
		if filter.Page > 0 {
			repoFilters.Offset = (filter.Page - 1) * filter.PageSize
		}
	}

	span.SetAttributes(
		attribute.Int("page", filter.Page),
		attribute.Int("page_size", filter.PageSize),
		attribute.String("search_term", filter.SearchTerm),
	)

	terminals, total, err := s.terminalRepo.ListTerminals(ctx, repoFilters)
	if err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("failed to list terminals: %w", err)
	}

	span.SetAttributes(attribute.Int64("result_count", total))
	return terminals, total, nil
}

// DeleteTerminal deletes a terminal by ID
func (s *terminalService) DeleteTerminal(ctx context.Context, terminalID uuid.UUID, deletedBy uuid.UUID) error {
	ctx, span := s.tracer.Start(ctx, "terminal_service.delete_terminal")
	defer span.End()

	span.SetAttributes(attribute.String("terminal_id", terminalID.String()))
	span.SetAttributes(attribute.String("deleted_by", deletedBy.String()))

	if err := s.configRepo.DeleteConfig(ctx, terminalID); err != nil {
		return fmt.Errorf("failed to delete terminal configs: %w", err)
	}

	if err := s.healthRepo.DeleteTerminalHealth(ctx, terminalID); err != nil {
		return fmt.Errorf("failed to delete terminal health record: %w", err)
	}

	if err := s.terminalRepo.DeleteTerminal(ctx, terminalID, deletedBy); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to delete terminal: %w", err)
	}

	s.logger.Info("Terminal deleted successfully", "terminal_id", terminalID, "deleted_by", deletedBy)
	return nil
}

// AssignTerminalToRetailer assigns a terminal to a retailer
func (s *terminalService) AssignTerminalToRetailer(ctx context.Context, terminalID, retailerID uuid.UUID, assignedBy uuid.UUID) error {
	ctx, span := s.tracer.Start(ctx, "terminal_service.assign_terminal_to_retailer")
	defer span.End()

	span.SetAttributes(
		attribute.String("terminal_id", terminalID.String()),
		attribute.String("retailer_id", retailerID.String()),
		attribute.String("assigned_by", assignedBy.String()),
	)

	// Create assignment
	assignment := &models.TerminalAssignment{
		TerminalID: terminalID,
		RetailerID: retailerID,
		AssignedBy: assignedBy,
		IsActive:   true,
	}

	if err := s.assignmentRepo.CreateAssignment(ctx, assignment); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to assign terminal to retailer: %w", err)
	}

	s.logger.Info("Terminal assigned to retailer successfully",
		"terminal_id", terminalID,
		"retailer_id", retailerID,
		"assigned_by", assignedBy)

	return nil
}

// UnassignTerminal unassigns a terminal from a retailer
func (s *terminalService) UnassignTerminal(ctx context.Context, terminalID uuid.UUID, unassignedBy uuid.UUID, reason string) error {
	ctx, span := s.tracer.Start(ctx, "terminal_service.unassign_terminal")
	defer span.End()

	span.SetAttributes(
		attribute.String("terminal_id", terminalID.String()),
		attribute.String("unassigned_by", unassignedBy.String()),
		attribute.String("reason", reason),
	)

	if err := s.assignmentRepo.UnassignTerminal(ctx, terminalID, unassignedBy, reason); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to unassign terminal: %w", err)
	}

	s.logger.Info("Terminal unassigned successfully",
		"terminal_id", terminalID,
		"unassigned_by", unassignedBy,
		"reason", reason)

	return nil
}

// GetTerminalsByRetailer gets terminals assigned to a retailer
func (s *terminalService) GetTerminalsByRetailer(ctx context.Context, retailerID uuid.UUID) ([]*models.Terminal, error) {
	ctx, span := s.tracer.Start(ctx, "terminal_service.get_terminals_by_retailer")
	defer span.End()

	span.SetAttributes(attribute.String("retailer_id", retailerID.String()))

	terminals, err := s.terminalRepo.GetTerminalsByRetailerID(ctx, retailerID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get terminals by retailer: %w", err)
	}

	if len(terminals) == 0 {
		return []*models.Terminal{}, nil //fmt.Errorf("no terminals found for retailer %s", retailerID.String())
	}

	span.SetAttributes(attribute.Int("result_count", len(terminals)))
	return terminals, nil
}

// UpdateTerminalStatus updates the status of a terminal
func (s *terminalService) UpdateTerminalStatus(ctx context.Context, terminalID uuid.UUID, newStatus models.TerminalStatus) error {
	ctx, span := s.tracer.Start(ctx, "terminal_service.update_terminal_status")
	defer span.End()

	span.SetAttributes(
		attribute.String("terminal_id", terminalID.String()),
		attribute.String("status", string(newStatus)),
	)

	if err := s.terminalRepo.UpdateTerminalStatus(ctx, terminalID, newStatus); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update terminal status: %w", err)
	}

	s.logger.Info("Terminal status updated", "terminal_id", terminalID, "status", newStatus)
	return nil
}

// Get health by terminal ID
func (s *terminalService) GetTerminalHealth(ctx context.Context, terminalID uuid.UUID) (*models.TerminalHealth, error) {
	ctx, span := s.tracer.Start(ctx, "terminal_service.get_terminal_health")
	defer span.End()

	span.SetAttributes(attribute.String("terminal_id", terminalID.String()))

	health, err := s.healthRepo.GetHealthByTerminalID(ctx, terminalID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get terminal health: %w", err)
	}

	return health, nil
}

// UpdateTerminalHealth updates the health status of a terminal
func (s *terminalService) UpdateTerminalHealth(ctx context.Context, terminalID uuid.UUID, health *models.TerminalHealth) error {
	ctx, span := s.tracer.Start(ctx, "terminal_service.update_terminal_health")
	defer span.End()

	span.SetAttributes(
		attribute.String("terminal_id", terminalID.String()),
		attribute.String("status", string(health.Status)),
	)

	health.TerminalID = terminalID
	health.LastHeartbeat = time.Now()

	// Update current health
	if err := s.healthRepo.CreateOrUpdateHealth(ctx, health); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update terminal health: %w", err)
	}

	// Record health history
	history := &models.TerminalHealthHistory{
		TerminalID:       health.TerminalID,
		Status:           health.Status,
		BatteryLevel:     health.BatteryLevel,
		SignalStrength:   health.SignalStrength,
		StorageAvailable: health.StorageAvailable,
		StorageTotal:     health.StorageTotal,
		MemoryUsage:      health.MemoryUsage,
		CPUUsage:         health.CPUUsage,
		Diagnostics:      health.Diagnostics,
		RecordedAt:       health.LastHeartbeat,
	}

	if err := s.healthRepo.RecordHealthHistory(ctx, history); err != nil {
		s.logger.Error("Failed to record health history", "error", err, "terminal_id", terminalID)
		span.RecordError(err)
	}

	return nil
}

// GetTerminalDiagnostics retrieves terminal diagnostics
func (s *terminalService) GetTerminalDiagnostics(ctx context.Context, terminalID uuid.UUID) (*models.TerminalHealth, error) {
	ctx, span := s.tracer.Start(ctx, "terminal_service.get_terminal_diagnostics")
	defer span.End()

	span.SetAttributes(attribute.String("terminal_id", terminalID.String()))

	health, err := s.healthRepo.GetTerminalDiagnostics(ctx, terminalID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get terminal diagnostics: %w", err)
	}

	return health, nil
}

// UpdateHeartbeat updates terminal heartbeat
func (s *terminalService) UpdateHeartbeat(ctx context.Context, terminalID uuid.UUID) error {
	ctx, span := s.tracer.Start(ctx, "terminal_service.update_heartbeat")
	defer span.End()

	span.SetAttributes(attribute.String("terminal_id", terminalID.String()))

	if err := s.healthRepo.UpdateHeartbeat(ctx, terminalID); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}

	return nil
}

// GetTerminalConfig retrieves the configuration for a terminal
func (s *terminalService) GetTerminalConfig(ctx context.Context, terminalID uuid.UUID) (*models.TerminalConfig, error) {
	ctx, span := s.tracer.Start(ctx, "terminal_service.get_terminal_config")
	defer span.End()

	span.SetAttributes(attribute.String("terminal_id", terminalID.String()))

	config, err := s.configRepo.GetConfigByTerminalID(ctx, terminalID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get terminal config: %w", err)
	}

	return config, nil
}

// UpdateTerminalConfig updates the configuration for a terminal
func (s *terminalService) UpdateTerminalConfig(ctx context.Context, config *models.TerminalConfig) error {
	ctx, span := s.tracer.Start(ctx, "terminal_service.update_terminal_config")
	defer span.End()

	span.SetAttributes(attribute.String("terminal_id", config.TerminalID.String()))

	if err := s.configRepo.UpdateConfig(ctx, config); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update terminal config: %w", err)
	}

	s.logger.Info("Terminal config updated", "terminal_id", config.TerminalID)
	return nil
}

// GetDashboardStats retrieves dashboard statistics
func (s *terminalService) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	ctx, span := s.tracer.Start(ctx, "terminal_service.get_dashboard_stats")
	defer span.End()

	// Get all terminals for stats calculation
	terminals, _, err := s.terminalRepo.ListTerminals(ctx, repositories.TerminalFilters{})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get terminals for stats: %w", err)
	}

	stats := &DashboardStats{
		StatusBreakdown: make(map[string]int64),
		ModelBreakdown:  make(map[string]int64),
	}

	for _, terminal := range terminals {
		stats.TotalTerminals++

		// Count by status
		stats.StatusBreakdown[string(terminal.Status)]++
		switch terminal.Status {
		case models.TerminalStatusActive:
			stats.ActiveTerminals++
		}

		// Count by health status
		switch terminal.HealthStatus {
		case models.HealthStatusHealthy:
			stats.HealthyTerminals++
		case models.HealthStatusWarning:
			stats.WarningTerminals++
		case models.HealthStatusCritical:
			stats.CriticalTerminals++
		case models.HealthStatusOffline:
			stats.OfflineTerminals++
		}

		// Count online status
		if terminal.IsOnline {
			stats.OnlineTerminals++
		}

		// Count by model
		stats.ModelBreakdown[string(terminal.Model)]++
	}

	span.SetAttributes(
		attribute.Int64("total_terminals", stats.TotalTerminals),
		attribute.Int64("active_terminals", stats.ActiveTerminals),
		attribute.Int64("online_terminals", stats.OnlineTerminals),
	)

	return stats, nil
}

// GetOfflineTerminals retrieves terminals that are offline
func (s *terminalService) GetOfflineTerminals(ctx context.Context, threshold time.Duration) ([]*models.Terminal, error) {
	ctx, span := s.tracer.Start(ctx, "terminal_service.get_offline_terminals")
	defer span.End()

	span.SetAttributes(attribute.String("threshold", threshold.String()))

	healthRecords, err := s.healthRepo.GetOfflineTerminals(ctx, threshold)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get offline terminals: %w", err)
	}

	terminals := make([]*models.Terminal, len(healthRecords))
	for i, health := range healthRecords {
		terminals[i] = health.Terminal
	}

	span.SetAttributes(attribute.Int("result_count", len(terminals)))
	return terminals, nil
}

func (s *terminalService) ValidateRetailerTerminal(ctx context.Context, retailerID uuid.UUID, terminalID uuid.UUID) error {
	ctx, span := s.tracer.Start(ctx, "terminal_service.validate_retailer_terminal")
	defer span.End()

	// Get active assignment for retailer
	assignment, err := s.assignmentRepo.GetActiveAssignmentByRetailerID(ctx, retailerID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("no active terminal assigned to retailer")
	}

	// Verify terminal ID matches
	if assignment.TerminalID != terminalID {
		return fmt.Errorf("terminal mismatch for retailer")
	}

	// Verify terminal is active
	terminal, err := s.terminalRepo.GetTerminalByID(ctx, terminalID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("terminal not found")
	}

	if terminal.Status != models.TerminalStatusActive {
		return fmt.Errorf("terminal is not active: status=%s", terminal.Status)
	}

	// Update last heartbeat
	if err := s.healthRepo.UpdateHeartbeat(ctx, terminalID); err != nil {
		s.logger.Error("Failed to update heartbeat", "error", err, "terminal_id", terminalID)
	}

	return nil
}

// GetRetailerTerminal returns the active terminal for a retailer
func (s *terminalService) GetRetailerTerminal(ctx context.Context, retailerID uuid.UUID) (*models.Terminal, error) {
	ctx, span := s.tracer.Start(ctx, "terminal_service.get_retailer_terminal")
	defer span.End()

	assignment, err := s.assignmentRepo.GetActiveAssignmentByRetailerID(ctx, retailerID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("no active terminal for retailer: %w", err)
	}

	terminal, err := s.terminalRepo.GetTerminalByID(ctx, assignment.TerminalID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get terminal: %w", err)
	}

	return terminal, nil
}

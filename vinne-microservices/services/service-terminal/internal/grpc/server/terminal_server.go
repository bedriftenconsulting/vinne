package server

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	pb "github.com/randco/randco-microservices/proto/terminal/v1"
	"github.com/randco/randco-microservices/services/service-terminal/internal/models"
	"github.com/randco/randco-microservices/services/service-terminal/internal/services"
	"github.com/randco/randco-microservices/shared/common/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TerminalServer implements the gRPC TerminalService
type TerminalServer struct {
	pb.UnimplementedTerminalServiceServer
	terminalService services.TerminalService
	logger          logger.Logger
	tracer          trace.Tracer
}

// NewTerminalServer creates a new terminal gRPC server
func NewTerminalServer(terminalService services.TerminalService, logger logger.Logger) *TerminalServer {
	return &TerminalServer{
		terminalService: terminalService,
		logger:          logger,
		tracer:          otel.Tracer("terminal-service"),
	}
}

// RegisterTerminal registers a new terminal
func (s *TerminalServer) RegisterTerminal(ctx context.Context, req *pb.RegisterTerminalRequest) (*pb.RegisterTerminalResponse, error) {
	ctx, span := s.tracer.Start(ctx, "grpc.RegisterTerminal")
	defer span.End()

	span.SetAttributes(
		attribute.String("device.id", req.DeviceId),
		attribute.String("terminal.name", req.Name),
		attribute.String("terminal.model", req.Model.String()),
	)

	// Validate request
	if req.DeviceId == "" {
		span.RecordError(fmt.Errorf("device_id is required"))
		span.SetStatus(otelcodes.Error, "validation failed")
		return nil, status.Errorf(codes.InvalidArgument, "device_id is required")
	}

	// Convert proto model to internal model
	terminal := &models.Terminal{
		DeviceID:       req.DeviceId,
		Name:           req.Name,
		Model:          convertProtoModelToInternal(req.Model),
		SerialNumber:   req.SerialNumber,
		IMEI:           req.Imei,
		AndroidVersion: req.AndroidVersion,
		AppVersion:     req.AppVersion,
		Vendor:         req.Vendor,
		Manufacturer:   req.Manufacturer,
		Status:         models.TerminalStatusInactive,
		HealthStatus:   models.HealthStatusOffline,
		Metadata:       req.Metadata,
	}

	if req.PurchaseDate != nil {
		purchaseDate := req.PurchaseDate.AsTime()
		terminal.PurchaseDate = &purchaseDate
	}

	// Register terminal
	if err := s.terminalService.RegisterTerminal(ctx, terminal); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "registration failed")
		s.logger.Error("Failed to register terminal", "error", err, "device_id", req.DeviceId)
		return nil, status.Errorf(codes.Internal, "failed to register terminal: %v", err)
	}

	span.SetAttributes(attribute.String("terminal.id", terminal.ID.String()))

	return &pb.RegisterTerminalResponse{
		Success:  true,
		Terminal: convertTerminalToProto(terminal),
		Message:  "Terminal registered successfully",
	}, nil
}

// GetTerminal retrieves a terminal by ID
func (s *TerminalServer) GetTerminal(ctx context.Context, req *pb.GetTerminalRequest) (*pb.GetTerminalResponse, error) {
	ctx, span := s.tracer.Start(ctx, "grpc.GetTerminal")
	defer span.End()

	span.SetAttributes(attribute.String("terminal.id", req.TerminalId))

	terminalID, err := uuid.Parse(req.TerminalId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid terminal_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid terminal_id format")
	}

	terminal, err := s.terminalService.GetTerminal(ctx, terminalID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get terminal")
		s.logger.Error("Failed to get terminal", "error", err, "terminal_id", terminalID)
		return nil, status.Errorf(codes.NotFound, err.Error())
	}

	assignment, err := s.terminalService.GetActiveAssignment(ctx, terminalID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get assignment")
		s.logger.Error("Failed to get terminal assignment", "error", err, "terminal_id", terminalID)
		// Not returning error to allow terminal retrieval even if assignment fails
	}

	config, err := s.terminalService.GetTerminalConfig(ctx, terminalID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get config")
		s.logger.Error("Failed to get terminal config", "error", err, "terminal_id", terminalID)
		// Not returning error to allow terminal retrieval even if config fails
	}

	health, err := s.terminalService.GetTerminalDiagnostics(ctx, terminalID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get health")
		s.logger.Error("Failed to get terminal health", "error", err, "terminal_id", terminalID)
		// Not returning error to allow terminal retrieval even if health fails
	}

	return &pb.GetTerminalResponse{
		Terminal:   convertTerminalToProto(terminal),
		Assignment: convertAssignmentToProto(assignment),
		Config:     convertConfigToProto(config),
		Health:     convertHealthToProto(health),
	}, nil
}

// ListTerminals lists terminals with filtering
func (s *TerminalServer) ListTerminals(ctx context.Context, req *pb.ListTerminalsRequest) (*pb.ListTerminalsResponse, error) {
	ctx, span := s.tracer.Start(ctx, "grpc.ListTerminals")
	defer span.End()

	span.SetAttributes(
		attribute.Int("page", int(req.Page)),
		attribute.Int("page.size", int(req.PageSize)),
	)

	filter := services.TerminalFilter{
		Page:     int(req.Page),
		PageSize: int(req.PageSize),
		SortBy:   req.SortBy,
		SortDesc: req.SortDesc,
	}

	if req.Status != pb.TerminalStatus_TERMINAL_STATUS_UNSPECIFIED {
		status := convertProtoStatusToInternal(req.Status)
		filter.Status = &status
	}

	if req.RetailerId != "" {
		retailerID, err := uuid.Parse(req.RetailerId)
		if err == nil {
			filter.RetailerID = &retailerID
		}
	}

	terminals, total, err := s.terminalService.ListTerminals(ctx, filter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to list terminals")
		s.logger.Error("Failed to list terminals", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to list terminals: %v", err)
	}

	// Convert terminals to proto format
	protoTerminals := make([]*pb.Terminal, len(terminals))
	for i, terminal := range terminals {
		protoTerminals[i] = convertTerminalToProto(terminal)
	}

	hasMore := req.PageSize > 0 && total > int64(req.Page*req.PageSize)

	return &pb.ListTerminalsResponse{
		Terminals:  protoTerminals,
		TotalCount: int32(total),
		Page:       req.Page,
		PageSize:   req.PageSize,
		HasMore:    hasMore,
	}, nil
}

// DeleteTerminal deletes a terminal by ID
func (s *TerminalServer) DeleteTerminal(ctx context.Context, req *pb.DeleteTerminalRequest) (*pb.DeleteTerminalResponse, error) {
	ctx, span := s.tracer.Start(ctx, "grpc.DeleteTerminal")
	defer span.End()

	span.SetAttributes(attribute.String("terminal.id", req.TerminalId))

	terminalID, err := uuid.Parse(req.TerminalId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid terminal_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid terminal_id format")
	}

	deletedBy, err := uuid.Parse(req.DeletedBy)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid deleted_by")
		return nil, status.Errorf(codes.InvalidArgument, "invalid deleted_by format")
	}

	if err := s.terminalService.DeleteTerminal(ctx, terminalID, deletedBy); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "deletion failed")
		s.logger.Error("Failed to delete terminal", "error", err, "terminal_id", terminalID)
		return nil, status.Errorf(codes.Internal, "failed to delete terminal: %v", err)
	}

	return &pb.DeleteTerminalResponse{
		Success: true,
		Message: "Terminal deleted successfully",
	}, nil
}

// AssignTerminalToRetailer assigns a terminal to a retailer
func (s *TerminalServer) AssignTerminalToRetailer(ctx context.Context, req *pb.AssignTerminalRequest) (*pb.AssignTerminalResponse, error) {
	ctx, span := s.tracer.Start(ctx, "grpc.AssignTerminalToRetailer")
	defer span.End()

	span.SetAttributes(
		attribute.String("terminal.id", req.TerminalId),
		attribute.String("retailer.id", req.RetailerId),
		attribute.String("assigned.by", req.AssignedBy),
	)

	terminalID, err := uuid.Parse(req.TerminalId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid terminal_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid terminal_id format")
	}

	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid retailer_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid retailer_id format")
	}

	assignedByID, err := uuid.Parse(req.AssignedBy)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid assigned_by")
		return nil, status.Errorf(codes.InvalidArgument, "invalid assigned_by format")
	}

	if err := s.terminalService.AssignTerminalToRetailer(ctx, terminalID, retailerID, assignedByID); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "assignment failed")
		s.logger.Error("Failed to assign terminal", "error", err, "terminal_id", terminalID, "retailer_id", retailerID)
		return nil, status.Errorf(codes.Internal, "failed to assign terminal: %v", err)
	}

	return &pb.AssignTerminalResponse{
		Success: true,
		Assignment: &pb.TerminalAssignment{
			Id:         uuid.New().String(),
			TerminalId: req.TerminalId,
			RetailerId: req.RetailerId,
			AssignedBy: req.AssignedBy,
			AssignedAt: timestamppb.Now(),
			IsActive:   true,
			Notes:      req.Notes,
		},
		Message: "Terminal assigned successfully",
	}, nil
}

// UnassignTerminal unassigns a terminal from a retailer
func (s *TerminalServer) UnassignTerminal(ctx context.Context, req *pb.UnassignTerminalRequest) (*pb.UnassignTerminalResponse, error) {
	ctx, span := s.tracer.Start(ctx, "grpc.UnassignTerminal")
	defer span.End()

	span.SetAttributes(
		attribute.String("terminal.id", req.TerminalId),
		attribute.String("reason", req.Reason),
	)

	terminalID, err := uuid.Parse(req.TerminalId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid terminal_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid terminal_id format")
	}

	// For now, use a system UUID since proto doesn't specify who's unassigning
	systemUUID := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	if err := s.terminalService.UnassignTerminal(ctx, terminalID, systemUUID, req.Reason); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "unassignment failed")
		s.logger.Error("Failed to unassign terminal", "error", err, "terminal_id", terminalID)
		return nil, status.Errorf(codes.Internal, "failed to unassign terminal: %v", err)
	}

	return &pb.UnassignTerminalResponse{
		Success: true,
		Message: "Terminal unassigned successfully",
	}, nil
}

// GetTerminalByRetailer gets terminal(s) assigned to a retailer
func (s *TerminalServer) GetTerminalByRetailer(ctx context.Context, req *pb.GetTerminalByRetailerRequest) (*pb.GetTerminalByRetailerResponse, error) {
	ctx, span := s.tracer.Start(ctx, "grpc.GetTerminalByRetailer")
	defer span.End()

	span.SetAttributes(attribute.String("retailer.id", req.RetailerId))

	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid retailer_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid retailer_id format")
	}

	terminals, err := s.terminalService.GetTerminalsByRetailer(ctx, retailerID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get terminals")
		s.logger.Error("Failed to get terminals by retailer", "error", err, "retailer_id", retailerID)
		return nil, status.Errorf(codes.NotFound, err.Error())
	}

	// Convert terminals to proto format
	// protoTerminals := make([]*pb.Terminal, len(terminals))
	// for i, terminal := range terminals {
	// 	protoTerminals[i] = convertTerminalToProto(terminal)
	// }

	var terminalsWithAssignments []*pb.TerminalWithAssignment
	for _, terminal := range terminals {
		assignment, _ := s.terminalService.GetActiveAssignment(ctx, terminal.ID)
		terminalsWithAssignments = append(terminalsWithAssignments, &pb.TerminalWithAssignment{
			Terminal:   convertTerminalToProto(terminal),
			Assignment: convertAssignmentToProto(assignment),
		})
	}

	return &pb.GetTerminalByRetailerResponse{
		Terminals: terminalsWithAssignments,
		// TODO: Add assignments
	}, nil
}

// UpdateTerminalStatus updates the status of a terminal
func (s *TerminalServer) UpdateTerminalStatus(ctx context.Context, req *pb.UpdateTerminalStatusRequest) (*pb.UpdateTerminalStatusResponse, error) {
	ctx, span := s.tracer.Start(ctx, "grpc.UpdateTerminalStatus")
	defer span.End()

	span.SetAttributes(
		attribute.String("terminal.id", req.TerminalId),
		attribute.String("status", req.Status.String()),
		attribute.String("updated.by", req.UpdatedBy),
	)

	terminalID, err := uuid.Parse(req.TerminalId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid terminal_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid terminal_id format")
	}

	terminalStatus := convertProtoStatusToInternal(req.Status)
	if err := s.terminalService.UpdateTerminalStatus(ctx, terminalID, terminalStatus); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "update failed")
		s.logger.Error("Failed to update terminal status", "error", err, "terminal_id", terminalID)
		return nil, status.Errorf(codes.Internal, "failed to update terminal status: %v", err)
	}

	// Get updated terminal
	terminal, _ := s.terminalService.GetTerminal(ctx, terminalID)

	return &pb.UpdateTerminalStatusResponse{
		Success:  true,
		Terminal: convertTerminalToProto(terminal),
		Message:  "Terminal status updated successfully",
	}, nil
}

// UpdateTerminal updates a terminal's information
func (s *TerminalServer) UpdateTerminal(ctx context.Context, req *pb.UpdateTerminalRequest) (*pb.UpdateTerminalResponse, error) {
	ctx, span := s.tracer.Start(ctx, "grpc.UpdateTerminal")
	defer span.End()

	span.SetAttributes(
		attribute.String("terminal.id", req.TerminalId),
	)

	terminalID, err := uuid.Parse(req.TerminalId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid terminal_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid terminal_id format")
	}

	// Get existing terminal
	existingTerminal, err := s.terminalService.GetTerminal(ctx, terminalID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "terminal not found")
		return nil, status.Errorf(codes.NotFound, err.Error())
	}

	// Update fields that are available in the proto
	if req.Name != "" {
		existingTerminal.Name = req.Name
	}
	if req.AppVersion != "" {
		existingTerminal.AppVersion = req.AppVersion
	}
	if req.Metadata != nil {
		existingTerminal.Metadata = req.Metadata
	}

	if err := s.terminalService.UpdateTerminal(ctx, existingTerminal); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "update failed")
		s.logger.Error("Failed to update terminal", "error", err, "terminal_id", terminalID)
		return nil, status.Errorf(codes.Internal, "failed to update terminal: %v", err)
	}

	return &pb.UpdateTerminalResponse{
		Success:  true,
		Terminal: convertTerminalToProto(existingTerminal),
		Message:  "Terminal updated successfully",
	}, nil
}

func (s *TerminalServer) GetTerminalHealth(ctx context.Context, req *pb.GetTerminalHealthRequest) (*pb.GetTerminalHealthResponse, error) {
	ctx, span := s.tracer.Start(ctx, "grpc.GetTerminalHealth")
	defer span.End()

	span.SetAttributes(attribute.String("terminal.id", req.TerminalId))

	terminalID, err := uuid.Parse(req.TerminalId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid terminal_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid terminal_id format")
	}

	health, err := s.terminalService.GetTerminalHealth(ctx, terminalID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get health")
		s.logger.Error("Failed to get terminal health", "error", err, "terminal_id", terminalID)
		return nil, status.Errorf(codes.NotFound, err.Error())
	}

	return &pb.GetTerminalHealthResponse{
		Success: true,
		Health:  convertHealthToProto(health),
		Message: "Terminal health retrieved successfully",
	}, nil
}

func (s *TerminalServer) UpdateTerminalHealth(ctx context.Context, req *pb.UpdateTerminalHealthRequest) (*pb.UpdateTerminalHealthResponse, error) {
	ctx, span := s.tracer.Start(ctx, "grpc.UpdateTerminalHealth")
	defer span.End()

	span.SetAttributes(
		attribute.String("terminal.id", req.TerminalId),
		attribute.String("health.status", req.Status.String()),
	)

	terminalID, err := uuid.Parse(req.TerminalId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid terminal_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid terminal_id format")
	}

	// Convert proto health to internal model
	health := &models.TerminalHealth{
		TerminalID:       terminalID,
		Status:           convertProtoHealthToInternal(req.Status),
		BatteryLevel:     int(req.BatteryLevel),
		SignalStrength:   int(req.SignalStrength),
		StorageAvailable: req.StorageAvailable,
		StorageTotal:     req.StorageTotal,
		MemoryUsage:      int(req.MemoryUsage),
		CPUUsage:         int(req.CpuUsage),
		LastHeartbeat:    time.Now(),
		Diagnostics:      req.Diagnostics,
	}

	if err := s.terminalService.UpdateTerminalHealth(ctx, terminalID, health); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "update failed")
		s.logger.Error("Failed to update terminal health", "error", err, "terminal_id", terminalID)
		return nil, status.Errorf(codes.Internal, "failed to update terminal health: %v", err)
	}

	return &pb.UpdateTerminalHealthResponse{
		Success: true,
		Health:  convertHealthToProto(health),
		Message: "Terminal health updated successfully",
	}, nil
}

func (s *TerminalServer) UpdateTerminalConfig(ctx context.Context, req *pb.UpdateTerminalConfigRequest) (*pb.UpdateTerminalConfigResponse, error) {
	ctx, span := s.tracer.Start(ctx, "grpc.UpdateTerminalConfig")
	defer span.End()

	span.SetAttributes(
		attribute.String("terminal.id", req.TerminalId),
	)

	terminalID, err := uuid.Parse(req.TerminalId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid terminal_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid terminal_id format")
	}

	// Get existing config or create new
	config, err := s.terminalService.GetTerminalConfig(ctx, terminalID)
	if err != nil {
		// Create new config if not found
		config = &models.TerminalConfig{
			TerminalID: terminalID,
		}
	}

	// Update config fields directly from request
	if req.TransactionLimit > 0 {
		config.TransactionLimit = int(req.TransactionLimit)
	}
	if req.DailyLimit > 0 {
		config.DailyLimit = int(req.DailyLimit)
	}
	config.OfflineModeEnabled = req.OfflineModeEnabled
	if req.OfflineSyncInterval > 0 {
		config.OfflineSyncInterval = int(req.OfflineSyncInterval)
	}
	config.AutoUpdateEnabled = req.AutoUpdateEnabled
	if req.MinimumAppVersion != "" {
		config.MinimumAppVersion = req.MinimumAppVersion
	}
	if req.Settings != nil {
		config.Settings = req.Settings
	}

	if err := s.terminalService.UpdateTerminalConfig(ctx, config); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "update failed")
		s.logger.Error("Failed to update terminal config", "error", err, "terminal_id", terminalID)
		return nil, status.Errorf(codes.Internal, "failed to update terminal config: %v", err)
	}

	return &pb.UpdateTerminalConfigResponse{
		Success: true,
		Config:  convertConfigToProto(config),
		Message: "Terminal configuration updated successfully",
	}, nil
}

func (s *TerminalServer) GetTerminalConfig(ctx context.Context, req *pb.GetTerminalConfigRequest) (*pb.GetTerminalConfigResponse, error) {
	ctx, span := s.tracer.Start(ctx, "grpc.GetTerminalConfig")
	defer span.End()

	span.SetAttributes(attribute.String("terminal.id", req.TerminalId))

	terminalID, err := uuid.Parse(req.TerminalId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid terminal_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid terminal_id format")
	}

	config, err := s.terminalService.GetTerminalConfig(ctx, terminalID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "config not found")
		s.logger.Error("Failed to get terminal config", "error", err, "terminal_id", terminalID)
		return nil, status.Errorf(codes.NotFound, err.Error())
	}

	return &pb.GetTerminalConfigResponse{
		Config: convertConfigToProto(config),
	}, nil
}

// Helper functions to convert between proto and internal types

func convertTerminalToProto(t *models.Terminal) *pb.Terminal {
	if t == nil {
		return nil
	}

	terminal := &pb.Terminal{
		Id:             t.ID.String(),
		DeviceId:       t.DeviceID,
		Name:           t.Name,
		Model:          convertInternalModelToProto(t.Model),
		SerialNumber:   t.SerialNumber,
		Imei:           t.IMEI,
		AndroidVersion: t.AndroidVersion,
		AppVersion:     t.AppVersion,
		Vendor:         t.Vendor,
		Manufacturer:   t.Manufacturer,
		Status:         convertInternalStatusToProto(t.Status),
		HealthStatus:   convertInternalHealthToProto(t.HealthStatus),
		CreatedAt:      timestamppb.New(t.CreatedAt),
		UpdatedAt:      timestamppb.New(t.UpdatedAt),
		Metadata:       t.Metadata,
	}

	if t.PurchaseDate != nil {
		terminal.PurchaseDate = timestamppb.New(*t.PurchaseDate)
	}
	if t.LastSync != nil {
		terminal.LastSync = timestamppb.New(*t.LastSync)
	}
	if t.LastTransaction != nil {
		terminal.LastTransaction = timestamppb.New(*t.LastTransaction)
	}

	return terminal
}

func convertAssignmentToProto(a *models.TerminalAssignment) *pb.TerminalAssignment {
	if a == nil {
		return nil
	}

	assignment := &pb.TerminalAssignment{
		Id:         a.ID.String(),
		TerminalId: a.TerminalID.String(),
		RetailerId: a.RetailerID.String(),
		// AssignedBy: a.AssignedBy.String(),
		AssignedAt: timestamppb.New(a.AssignedAt),
		IsActive:   a.IsActive,
		Notes:      a.Notes,
	}

	if a.AssignedBy != uuid.Nil {
		assignment.AssignedBy = a.AssignedBy.String()
	}

	if a.UnassignedAt != nil {
		assignment.UnassignedAt = timestamppb.New(*a.UnassignedAt)
	}

	return assignment
}

func convertProtoModelToInternal(m pb.TerminalModel) models.TerminalModel {
	switch m {
	case pb.TerminalModel_ANDROID_POS_V1:
		return models.TerminalModelAndroidPOSV1
	case pb.TerminalModel_ANDROID_POS_V2:
		return models.TerminalModelAndroidPOSV2
	case pb.TerminalModel_WEB_TERMINAL:
		return models.TerminalModelWebTerminal
	case pb.TerminalModel_MOBILE_TERMINAL:
		return models.TerminalModelMobile
	default:
		return models.TerminalModelAndroidPOSV1
	}
}

func convertInternalModelToProto(m models.TerminalModel) pb.TerminalModel {
	switch m {
	case models.TerminalModelAndroidPOSV1:
		return pb.TerminalModel_ANDROID_POS_V1
	case models.TerminalModelAndroidPOSV2:
		return pb.TerminalModel_ANDROID_POS_V2
	case models.TerminalModelWebTerminal:
		return pb.TerminalModel_WEB_TERMINAL
	case models.TerminalModelMobile:
		return pb.TerminalModel_MOBILE_TERMINAL
	default:
		return pb.TerminalModel_TERMINAL_MODEL_UNSPECIFIED
	}
}

func convertProtoStatusToInternal(s pb.TerminalStatus) models.TerminalStatus {
	switch s {
	case pb.TerminalStatus_ACTIVE:
		return models.TerminalStatusActive
	case pb.TerminalStatus_INACTIVE:
		return models.TerminalStatusInactive
	case pb.TerminalStatus_FAULTY:
		return models.TerminalStatusFaulty
	case pb.TerminalStatus_MAINTENANCE:
		return models.TerminalStatusMaintenance
	case pb.TerminalStatus_SUSPENDED:
		return models.TerminalStatusSuspended
	case pb.TerminalStatus_DECOMMISSIONED:
		return models.TerminalStatusDecommissioned
	default:
		return models.TerminalStatusInactive
	}
}

func convertInternalStatusToProto(s models.TerminalStatus) pb.TerminalStatus {
	switch s {
	case models.TerminalStatusActive:
		return pb.TerminalStatus_ACTIVE
	case models.TerminalStatusInactive:
		return pb.TerminalStatus_INACTIVE
	case models.TerminalStatusFaulty:
		return pb.TerminalStatus_FAULTY
	case models.TerminalStatusMaintenance:
		return pb.TerminalStatus_MAINTENANCE
	case models.TerminalStatusSuspended:
		return pb.TerminalStatus_SUSPENDED
	case models.TerminalStatusDecommissioned:
		return pb.TerminalStatus_DECOMMISSIONED
	default:
		return pb.TerminalStatus_TERMINAL_STATUS_UNSPECIFIED
	}
}

func convertInternalHealthToProto(h models.HealthStatus) pb.HealthStatus {
	switch h {
	case models.HealthStatusHealthy:
		return pb.HealthStatus_HEALTHY
	case models.HealthStatusWarning:
		return pb.HealthStatus_WARNING
	case models.HealthStatusCritical:
		return pb.HealthStatus_CRITICAL
	case models.HealthStatusOffline:
		return pb.HealthStatus_OFFLINE
	default:
		return pb.HealthStatus_HEALTH_STATUS_UNSPECIFIED
	}
}

func convertProtoHealthToInternal(h pb.HealthStatus) models.HealthStatus {
	switch h {
	case pb.HealthStatus_HEALTHY:
		return models.HealthStatusHealthy
	case pb.HealthStatus_WARNING:
		return models.HealthStatusWarning
	case pb.HealthStatus_CRITICAL:
		return models.HealthStatusCritical
	case pb.HealthStatus_OFFLINE:
		return models.HealthStatusOffline
	default:
		return models.HealthStatusOffline
	}
}

func convertHealthToProto(h *models.TerminalHealth) *pb.TerminalHealth {
	if h == nil {
		return nil
	}

	return &pb.TerminalHealth{
		TerminalId:       h.TerminalID.String(),
		Status:           convertInternalHealthToProto(h.Status),
		BatteryLevel:     int32(h.BatteryLevel),
		SignalStrength:   int32(h.SignalStrength),
		StorageAvailable: h.StorageAvailable,
		StorageTotal:     h.StorageTotal,
		MemoryUsage:      int32(h.MemoryUsage),
		CpuUsage:         int32(h.CPUUsage),
		LastHeartbeat:    timestamppb.New(h.LastHeartbeat),
		Diagnostics:      h.Diagnostics,
	}
}

func convertConfigToProto(c *models.TerminalConfig) *pb.TerminalConfig {
	if c == nil {
		return nil
	}

	return &pb.TerminalConfig{
		TerminalId:          c.TerminalID.String(),
		TransactionLimit:    int32(c.TransactionLimit),
		DailyLimit:          int32(c.DailyLimit),
		OfflineModeEnabled:  c.OfflineModeEnabled,
		OfflineSyncInterval: int32(c.OfflineSyncInterval),
		AutoUpdateEnabled:   c.AutoUpdateEnabled,
		MinimumAppVersion:   c.MinimumAppVersion,
		Settings:            c.Settings,
		UpdatedAt:           timestamppb.New(c.UpdatedAt),
	}
}

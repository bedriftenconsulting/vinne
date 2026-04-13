package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	pb "github.com/randco/randco-microservices/proto/agent/management/v1"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/repositories"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/services"
	"github.com/randco/randco-microservices/shared/common/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AgentManagementServer struct {
	pb.UnimplementedAgentManagementServiceServer
	agentService              services.AgentService
	posDeviceService          services.POSDeviceService
	retailerService           services.RetailerService
	retailerAssignmentService services.RetailerAssignmentService
	logger                    logger.Logger
}

func NewAgentManagementServer(
	agentService services.AgentService,
	retailerService services.RetailerService,
	retailerAssignmentService services.RetailerAssignmentService,
	posDeviceService services.POSDeviceService,
	logger logger.Logger,
) *AgentManagementServer {
	return &AgentManagementServer{
		agentService:              agentService,
		retailerService:           retailerService,
		retailerAssignmentService: retailerAssignmentService,
		posDeviceService:          posDeviceService,
		logger:                    logger,
	}
}

// Agent operations

func (s *AgentManagementServer) CreateAgent(ctx context.Context, req *pb.CreateAgentRequest) (*pb.Agent, error) {
	s.logger.Info("Creating agent", "name", req.Name, "email", req.Email, "phone", req.PhoneNumber)

	// Validate request
	if req.Name == "" {
		s.logger.Error("CreateAgent validation failed", "error", "name is required")
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.PhoneNumber == "" {
		s.logger.Error("CreateAgent validation failed", "error", "phone number is required")
		return nil, status.Error(codes.InvalidArgument, "phone number is required")
	}

	// Convert proto request to service request
	serviceReq := &services.CreateAgentRequest{
		BusinessName:         req.Name,
		ContactEmail:         req.Email,
		ContactPhone:         req.PhoneNumber,
		PhysicalAddress:      req.Address,
		CommissionPercentage: &req.CommissionPercentage,
		CreatedBy:            req.CreatedBy,
	}

	// Call service to create agent
	agent, err := s.agentService.CreateAgent(ctx, serviceReq)
	if err != nil {
		s.logger.Error("Failed to create agent", "error", err)
		msg := err.Error()
		lower := strings.ToLower(msg)

		switch {
		case strings.Contains(lower, "already exists"):
			return nil, status.Error(codes.AlreadyExists, msg)
		case strings.Contains(lower, "validation failed"),
			strings.Contains(lower, "required"),
			strings.Contains(lower, "invalid"):
			return nil, status.Error(codes.InvalidArgument, msg)
		default:
			return nil, status.Error(codes.Internal, msg)
		}
	}

	// Convert domain model to proto response
	pbAgent := &pb.Agent{
		Id:                   agent.ID.String(),
		AgentCode:            agent.AgentCode,
		Name:                 agent.BusinessName,
		Email:                agent.ContactEmail,
		PhoneNumber:          agent.ContactPhone,
		Address:              agent.PhysicalAddress,
		CommissionPercentage: agent.CommissionPercentage,
		Status:               mapStatusToProto(agent.Status),
		CreatedBy:            agent.CreatedBy,
		CreatedAt:            timestamppb.New(agent.CreatedAt),
		UpdatedAt:            timestamppb.New(agent.UpdatedAt),
		InitialPassword:      agent.InitialPassword,
	}

	s.logger.Info("Agent created successfully", "id", agent.ID, "agent_code", agent.AgentCode)
	return pbAgent, nil
}

func (s *AgentManagementServer) GetAgent(ctx context.Context, req *pb.GetAgentRequest) (*pb.Agent, error) {
	s.logger.Info("Getting agent", "id", req.Id)

	// Validate request
	if req.Id == "" {
		s.logger.Error("GetAgent validation failed", "error", "id is required")
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// Parse agent ID
	agentID, err := uuid.Parse(req.Id)
	if err != nil {
		s.logger.Error("Invalid agent ID", "id", req.Id, "error", err)
		return nil, status.Error(codes.InvalidArgument, "invalid agent ID")
	}

	// Call service to get agent
	agent, err := s.agentService.GetAgent(ctx, agentID)
	if err != nil {
		s.logger.Error("Failed to get agent", "id", req.Id, "error", err)
		return nil, status.Error(codes.NotFound, "agent not found")
	}

	// Convert domain model to proto response
	pbAgent := &pb.Agent{
		Id:          agent.ID.String(),
		AgentCode:   agent.AgentCode,
		Name:        agent.BusinessName,
		Email:       agent.ContactEmail,
		PhoneNumber: agent.ContactPhone,
		Address:     agent.PhysicalAddress,
		Status:      mapStatusToProto(agent.Status),
		CreatedBy:   agent.CreatedBy,
		UpdatedBy:   agent.UpdatedBy,
		CreatedAt:   timestamppb.New(agent.CreatedAt),
		UpdatedAt:   timestamppb.New(agent.UpdatedAt),
	}

	pbAgent.CommissionPercentage = agent.CommissionPercentage

	s.logger.Info("Agent retrieved successfully", "id", agent.ID, "agent_code", agent.AgentCode)
	return pbAgent, nil
}

func (s *AgentManagementServer) GetAgentByCode(ctx context.Context, req *pb.GetAgentByCodeRequest) (*pb.Agent, error) {
	// TODO: Implement actual service call
	return &pb.Agent{
		Id:          "agent-1",
		AgentCode:   req.AgentCode,
		Name:        "Test Agent",
		Email:       "agent@test.com",
		PhoneNumber: "+233123456789",
		Address:     "Test Address",
		Status:      pb.EntityStatus_ENTITY_STATUS_ACTIVE,
		CreatedAt:   timestamppb.Now(),
		UpdatedAt:   timestamppb.Now(),
	}, nil
}

func (s *AgentManagementServer) UpdateAgent(ctx context.Context, req *pb.UpdateAgentRequest) (*pb.Agent, error) {
	s.logger.Info("Updating agent", "id", req.Id, "name", req.Name, "updated_by", req.UpdatedBy)

	// Validate request
	if req.Id == "" {
		s.logger.Error("UpdateAgent validation failed", "error", "id is required")
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if req.UpdatedBy == "" {
		s.logger.Error("UpdateAgent validation failed", "error", "updated_by is required")
		return nil, status.Error(codes.InvalidArgument, "updated_by is required")
	}

	// Parse agent ID
	agentID, err := uuid.Parse(req.Id)
	if err != nil {
		s.logger.Error("Invalid agent ID", "id", req.Id, "error", err)
		return nil, status.Error(codes.InvalidArgument, "invalid agent ID")
	}

	// Convert proto request to service request
	serviceReq := &services.UpdateAgentRequest{
		ID:        agentID,
		UpdatedBy: req.UpdatedBy,
	}

	// Set optional fields if provided
	if req.Name != "" {
		serviceReq.BusinessName = &req.Name
	}
	if req.Email != "" {
		serviceReq.ContactEmail = &req.Email
	}
	if req.PhoneNumber != "" {
		serviceReq.ContactPhone = &req.PhoneNumber
	}
	if req.Address != "" {
		serviceReq.PhysicalAddress = &req.Address
	}
	if req.CommissionPercentage > 0 {
		serviceReq.CommissionPercentage = &req.CommissionPercentage
	}

	// Call service to update agent
	agent, err := s.agentService.UpdateAgent(ctx, serviceReq)
	if err != nil {
		s.logger.Error("Failed to update agent", "id", req.Id, "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to update agent: %v", err))
	}

	// Convert domain model to proto response
	pbAgent := &pb.Agent{
		Id:                   agent.ID.String(),
		AgentCode:            agent.AgentCode,
		Name:                 agent.BusinessName,
		Email:                agent.ContactEmail,
		PhoneNumber:          agent.ContactPhone,
		Address:              agent.PhysicalAddress,
		CommissionPercentage: agent.CommissionPercentage,
		Status:               mapStatusToProto(agent.Status),
		CreatedBy:            agent.CreatedBy,
		UpdatedBy:            agent.UpdatedBy,
		CreatedAt:            timestamppb.New(agent.CreatedAt),
		UpdatedAt:            timestamppb.New(agent.UpdatedAt),
	}

	s.logger.Info("Agent updated successfully", "id", agent.ID, "agent_code", agent.AgentCode)
	return pbAgent, nil
}

func (s *AgentManagementServer) ListAgents(ctx context.Context, req *pb.ListAgentsRequest) (*pb.ListAgentsResponse, error) {
	// Build service request
	serviceReq := &services.ListAgentsRequest{
		Limit:  int(req.PageSize),
		Offset: int((req.Page - 1) * req.PageSize),
	}

	// Apply optional filters
	if req.Filter != nil {
		if req.Filter.Status != pb.EntityStatus_ENTITY_STATUS_UNSPECIFIED {
			status := mapProtoToStatus(req.Filter.Status)
			serviceReq.Status = &status
		}
		if req.Filter.Name != "" {
			serviceReq.BusinessName = &req.Filter.Name
		}
		if req.Filter.Email != "" {
			serviceReq.ContactEmail = &req.Filter.Email
		}
	}

	// Call service to list agents
	response, err := s.agentService.ListAgents(ctx, serviceReq)
	if err != nil {
		s.logger.Error("Failed to list agents", "error", err)
		return nil, status.Error(codes.Internal, "failed to list agents")
	}

	agents := response.Agents
	totalCount := response.Total

	// Convert domain models to proto response
	pbAgents := make([]*pb.Agent, len(agents))
	for i, agent := range agents {
		pbAgents[i] = &pb.Agent{
			Id:          agent.ID.String(),
			AgentCode:   agent.AgentCode,
			Name:        agent.BusinessName,
			Email:       agent.ContactEmail,
			PhoneNumber: agent.ContactPhone,
			Address:     agent.PhysicalAddress,
			Status:      mapStatusToProto(agent.Status),
			CreatedBy:   agent.CreatedBy,
			UpdatedBy:   agent.UpdatedBy,
			CreatedAt:   timestamppb.New(agent.CreatedAt),
			UpdatedAt:   timestamppb.New(agent.UpdatedAt),
		}

		pbAgents[i].CommissionPercentage = agent.CommissionPercentage
	}

	return &pb.ListAgentsResponse{
		Agents:     pbAgents,
		TotalCount: int32(totalCount),
		Page:       req.Page,
		PageSize:   req.PageSize,
	}, nil
}

func (s *AgentManagementServer) UpdateAgentStatus(ctx context.Context, req *pb.UpdateAgentStatusRequest) (*emptypb.Empty, error) {
	s.logger.Info("Updating agent status", "id", req.Id, "status", req.Status, "updated_by", req.UpdatedBy)

	// Validate request
	if req.Id == "" {
		s.logger.Error("UpdateAgentStatus validation failed", "error", "id is required")
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if req.UpdatedBy == "" {
		s.logger.Error("UpdateAgentStatus validation failed", "error", "updated_by is required")
		return nil, status.Error(codes.InvalidArgument, "updated_by is required")
	}

	// Parse agent ID
	agentID, err := uuid.Parse(req.Id)
	if err != nil {
		s.logger.Error("Invalid agent ID", "id", req.Id, "error", err)
		return nil, status.Error(codes.InvalidArgument, "invalid agent ID")
	}

	// Convert proto status to domain status
	domainStatus := mapProtoToStatus(req.Status)

	// Call service to update agent status
	err = s.agentService.UpdateAgentStatus(ctx, agentID, domainStatus, req.UpdatedBy)
	if err != nil {
		s.logger.Error("Failed to update agent status", "id", req.Id, "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to update agent status: %v", err))
	}

	s.logger.Info("Agent status updated successfully", "id", req.Id, "status", req.Status)
	return &emptypb.Empty{}, nil
}

func (s *AgentManagementServer) DeleteAgent(ctx context.Context, req *pb.DeleteAgentRequest) (*emptypb.Empty, error) {
	s.logger.Info("Deleting agent", "id", req.Id, "deleted_by", req.DeletedBy)

	// Validate request
	if req.Id == "" {
		s.logger.Error("DeleteAgent validation failed", "error", "id is required")
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if req.DeletedBy == "" {
		s.logger.Error("DeleteAgent validation failed", "error", "deleted_by is required")
		return nil, status.Error(codes.InvalidArgument, "deleted_by is required")
	}

	// Parse agent ID
	agentID, err := uuid.Parse(req.Id)
	if err != nil {
		s.logger.Error("Invalid agent ID", "id", req.Id, "error", err)
		return nil, status.Error(codes.InvalidArgument, "invalid agent ID")
	}

	// Call service to delete agent
	err = s.agentService.DeleteAgent(ctx, agentID, req.DeletedBy)
	if err != nil {
		s.logger.Error("Failed to delete agent", "id", req.Id, "error", err)
		// Return appropriate error code based on error type
		errMsg := err.Error()
		if len(errMsg) >= 15 && errMsg[:15] == "agent not found" {
			return nil, status.Error(codes.NotFound, "agent not found")
		}
		if len(errMsg) >= 6 && errMsg[:6] == "cannot" { // Business rule violation
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to delete agent: %v", err))
	}

	s.logger.Info("Agent deleted successfully", "id", req.Id)
	return &emptypb.Empty{}, nil
}

// Retailer operations

func (s *AgentManagementServer) CreateRetailer(ctx context.Context, req *pb.CreateRetailerRequest) (*pb.Retailer, error) {
	// Map protobuf request to service request
	businessName := req.BusinessName
	if businessName == "" {
		businessName = req.Name
	}
	serviceReq := &services.CreateRetailerRequest{
		BusinessName:     businessName,
		ContactName:      req.Name, // Using name as contact name for now
		ContactEmail:     req.Email,
		ContactPhone:     req.PhoneNumber,
		PhysicalAddress:  req.Address,
		OnboardingMethod: req.OnboardingMethod,
		CreatedBy:        req.CreatedBy,
	}

	// Handle agent ID (required field)
	if req.AgentId != "" && req.AgentId != "none" {
		agentID, err := uuid.Parse(req.AgentId)
		if err == nil && agentID != uuid.Nil {
			serviceReq.AgentID = agentID
		}
	}

	// Call service to create retailer
	retailer, err := s.retailerService.CreateRetailer(ctx, serviceReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create retailer: %v", err)
	}

	// Convert model to protobuf response
	return &pb.Retailer{
		Id:           retailer.ID.String(),
		RetailerCode: retailer.RetailerCode,
		Name:         retailer.Name,
		Email:        retailer.Email,
		PhoneNumber:  retailer.PhoneNumber,
		Address:      retailer.Address,
		AgentId: func() string {
			if retailer.AgentID != nil {
				return retailer.AgentID.String()
			}
			return ""
		}(),
		Status:    convertEntityStatus(retailer.Status),
		CreatedBy: retailer.CreatedBy,
		UpdatedBy: retailer.UpdatedBy,
		CreatedAt: timestamppb.New(retailer.CreatedAt),
		UpdatedAt: timestamppb.New(retailer.UpdatedAt),
	}, nil
}

func (s *AgentManagementServer) GetRetailer(ctx context.Context, req *pb.GetRetailerRequest) (*pb.Retailer, error) {
	retailerID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid retailer ID: %v", err)
	}

	retailer, err := s.retailerService.GetRetailerByID(ctx, retailerID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "retailer not found: %v", err)
	}

	// Convert model to protobuf response
	return &pb.Retailer{
		Id:           retailer.ID.String(),
		RetailerCode: retailer.RetailerCode,
		Name:         retailer.Name,
		Email:        retailer.Email,
		PhoneNumber:  retailer.PhoneNumber,
		Address:      retailer.Address,
		AgentId: func() string {
			if retailer.AgentID != nil {
				return retailer.AgentID.String()
			}
			return ""
		}(),
		Status:    convertEntityStatus(retailer.Status),
		CreatedBy: retailer.CreatedBy,
		UpdatedBy: retailer.UpdatedBy,
		CreatedAt: timestamppb.New(retailer.CreatedAt),
		UpdatedAt: timestamppb.New(retailer.UpdatedAt),
	}, nil
}

func (s *AgentManagementServer) GetRetailerByCode(ctx context.Context, req *pb.GetRetailerByCodeRequest) (*pb.Retailer, error) {
	// TODO: Implement actual service call
	return &pb.Retailer{
		Id:           "retailer-1",
		RetailerCode: req.RetailerCode,
		Name:         "Test Retailer",
		Email:        "retailer@test.com",
		PhoneNumber:  "+233123456789",
		Address:      "Test Retailer Address",
		AgentId:      "agent-1",
		Status:       pb.EntityStatus_ENTITY_STATUS_ACTIVE,
		CreatedAt:    timestamppb.Now(),
		UpdatedAt:    timestamppb.Now(),
	}, nil
}

func (s *AgentManagementServer) UpdateRetailer(ctx context.Context, req *pb.UpdateRetailerRequest) (*pb.Retailer, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if req.UpdatedBy == "" {
		return nil, status.Error(codes.InvalidArgument, "updated_by is required")
	}

	retailerID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid retailer ID: %v", err)
	}

	serviceReq := &services.UpdateRetailerRequest{
		UpdatedBy: req.UpdatedBy,
	}
	if req.Name != "" {
		serviceReq.BusinessName = &req.Name
	}
	if req.Email != "" {
		serviceReq.ContactEmail = &req.Email
	}
	if req.PhoneNumber != "" {
		serviceReq.ContactPhone = &req.PhoneNumber
	}
	if req.Address != "" {
		serviceReq.PhysicalAddress = &req.Address
	}

	retailer, err := s.retailerService.UpdateRetailer(ctx, retailerID, serviceReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update retailer: %v", err)
	}

	return &pb.Retailer{
		Id:           retailer.ID.String(),
		RetailerCode: retailer.RetailerCode,
		Name:         retailer.Name,
		Email:        retailer.Email,
		PhoneNumber:  retailer.PhoneNumber,
		Address:      retailer.Address,
		AgentId: func() string {
			if retailer.AgentID != nil {
				return retailer.AgentID.String()
			}
			return ""
		}(),
		Status:    convertEntityStatus(retailer.Status),
		CreatedBy: retailer.CreatedBy,
		UpdatedBy: retailer.UpdatedBy,
		CreatedAt: timestamppb.New(retailer.CreatedAt),
		UpdatedAt: timestamppb.New(retailer.UpdatedAt),
	}, nil
}

func (s *AgentManagementServer) ListRetailers(ctx context.Context, req *pb.ListRetailersRequest) (*pb.ListRetailersResponse, error) {
	// Map protobuf filters to service filters
	filters := repositories.RetailerFilters{}

	// Handle pagination
	if req.PageSize > 0 {
		filters.Limit = int(req.PageSize)
	} else {
		filters.Limit = 20
	}
	if req.Page > 1 {
		filters.Offset = (int(req.Page) - 1) * filters.Limit
	}

	// Handle filters
	if req.Filter != nil {
		if req.Filter.Status != pb.EntityStatus_ENTITY_STATUS_UNSPECIFIED {
			status := convertProtobufToEntityStatus(req.Filter.Status)
			filters.Status = &status
		}
		if req.Filter.AgentId != "" && req.Filter.AgentId != "independent" {
			agentID, err := uuid.Parse(req.Filter.AgentId)
			if err == nil && agentID != uuid.Nil {
				filters.AgentID = &agentID
			}
		}
		if req.Filter.Name != "" {
			filters.Name = &req.Filter.Name
		}
		if req.Filter.Email != "" {
			filters.Email = &req.Filter.Email
		}
	}

	// Call service to list retailers
	retailers, total, err := s.retailerService.ListRetailers(ctx, filters)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list retailers: %v", err)
	}

	// Convert models to protobuf
	pbRetailers := make([]*pb.Retailer, len(retailers))
	for i, retailer := range retailers {
		pbRetailers[i] = &pb.Retailer{
			Id:           retailer.ID.String(),
			RetailerCode: retailer.RetailerCode,
			Name:         retailer.Name,
			Email:        retailer.Email,
			PhoneNumber:  retailer.PhoneNumber,
			Address:      retailer.Address,
			AgentId: func() string {
				if retailer.AgentID != nil {
					return retailer.AgentID.String()
				}
				return ""
			}(),
			Status:    convertEntityStatus(retailer.Status),
			CreatedBy: retailer.CreatedBy,
			UpdatedBy: retailer.UpdatedBy,
			CreatedAt: timestamppb.New(retailer.CreatedAt),
			UpdatedAt: timestamppb.New(retailer.UpdatedAt),
		}
	}

	return &pb.ListRetailersResponse{
		Retailers:  pbRetailers,
		TotalCount: int32(total),
		Page:       req.Page,
		PageSize:   req.PageSize,
	}, nil
}

func (s *AgentManagementServer) UpdateRetailerStatus(ctx context.Context, req *pb.UpdateRetailerStatusRequest) (*emptypb.Empty, error) {
	// Parse UUID
	retailerID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid retailer ID: %v", err)
	}

	// Convert status
	status := models.EntityStatus(req.Status.String())

	// Update status
	if err := s.retailerService.UpdateRetailerStatus(ctx, retailerID, status, req.UpdatedBy); err != nil {
		s.logger.Error("Failed to update retailer status", "error", err, "retailer_id", req.Id)
		return nil, err
	}

	s.logger.Info("Retailer status updated", "retailer_id", req.Id, "status", status)
	return &emptypb.Empty{}, nil
}

func (s *AgentManagementServer) DisableRetailer(ctx context.Context, req *pb.DisableRetailerRequest) (*emptypb.Empty, error) {
	// Parse UUID
	retailerID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid retailer ID: %v", err)
	}

	// Disable retailer (set status to inactive)
	if err := s.retailerService.UpdateRetailerStatus(ctx, retailerID, models.StatusInactive, req.DisabledBy); err != nil {
		s.logger.Error("Failed to disable retailer", "error", err, "retailer_id", req.Id)
		return nil, err
	}

	// TODO: Add audit log for disable action with reason
	s.logger.Info("Retailer disabled", "retailer_id", req.Id, "reason", req.Reason, "disabled_by", req.DisabledBy)
	return &emptypb.Empty{}, nil
}

func (s *AgentManagementServer) EnableRetailer(ctx context.Context, req *pb.EnableRetailerRequest) (*emptypb.Empty, error) {
	// Parse UUID
	retailerID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid retailer ID: %v", err)
	}

	// Enable retailer (set status to active)
	if err := s.retailerService.UpdateRetailerStatus(ctx, retailerID, models.StatusActive, req.EnabledBy); err != nil {
		s.logger.Error("Failed to enable retailer", "error", err, "retailer_id", req.Id)
		return nil, err
	}

	// TODO: Add audit log for enable action
	s.logger.Info("Retailer enabled", "retailer_id", req.Id, "enabled_by", req.EnabledBy)
	return &emptypb.Empty{}, nil
}

func (s *AgentManagementServer) GetRetailerDetails(ctx context.Context, req *pb.GetRetailerDetailsRequest) (*pb.RetailerDetailsResponse, error) {
	// Parse UUID
	retailerID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid retailer ID: %v", err)
	}

	// Get retailer
	retailer, err := s.retailerService.GetRetailer(ctx, retailerID)
	if err != nil {
		s.logger.Error("Failed to get retailer", "error", err, "retailer_id", req.Id)
		return nil, err
	}

	// Convert to proto
	pbRetailer := convertRetailerToProto(retailer)

	// Build response
	response := &pb.RetailerDetailsResponse{
		Retailer: pbRetailer,
	}

	// Get parent agent if assigned
	if retailer.AgentID != nil {
		agent, err := s.agentService.GetAgent(ctx, *retailer.AgentID)
		if err == nil {
			response.ParentAgent = convertAgentToProto(agent)
		}
	}

	// TODO: Get KYC info if available
	// TODO: Get POS devices if assigned
	// TODO: Get terminal info from Terminal Service

	return response, nil
}

// Agent-Retailer relationship operations

func (s *AgentManagementServer) AssignRetailerToAgent(ctx context.Context, req *pb.AssignRetailerToAgentRequest) (*emptypb.Empty, error) {
	// Parse UUIDs
	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, fmt.Errorf("invalid retailer ID: %v", err)
	}

	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID: %v", err)
	}

	// Call service method
	err = s.retailerAssignmentService.AssignRetailerToAgent(ctx, retailerID, agentID, req.AssignedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to assign retailer to agent: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *AgentManagementServer) ReassignRetailerToAgent(ctx context.Context, req *pb.ReassignRetailerToAgentRequest) (*emptypb.Empty, error) {
	// Parse UUIDs
	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, fmt.Errorf("invalid retailer ID: %v", err)
	}

	newAgentID, err := uuid.Parse(req.NewAgentId)
	if err != nil {
		return nil, fmt.Errorf("invalid new agent ID: %v", err)
	}

	// Call service method
	err = s.retailerAssignmentService.ReassignRetailerToAgent(ctx, retailerID, newAgentID, req.ReassignedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to reassign retailer to agent: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *AgentManagementServer) UnassignRetailerFromAgent(ctx context.Context, req *pb.UnassignRetailerFromAgentRequest) (*emptypb.Empty, error) {
	// Parse UUID
	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, fmt.Errorf("invalid retailer ID: %v", err)
	}

	// Call service method
	err = s.retailerAssignmentService.UnassignRetailerFromAgent(ctx, retailerID, req.UnassignedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to unassign retailer from agent: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *AgentManagementServer) GetAgentRetailers(ctx context.Context, req *pb.GetAgentRetailersRequest) (*pb.GetAgentRetailersResponse, error) {
	s.logger.Info("Getting retailers for agent", "agent_id", req.AgentId)

	// Validate agent ID
	if req.AgentId == "" {
		s.logger.Error("GetAgentRetailers validation failed", "error", "agent_id is required")
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	agentUUID, err := uuid.Parse(req.AgentId)
	if err != nil {
		s.logger.Error("Invalid agent UUID", "agent_id", req.AgentId, "error", err)
		return nil, status.Error(codes.InvalidArgument, "invalid agent_id format")
	}

	// Build filters to get retailers for this agent
	filters := repositories.RetailerFilters{
		AgentID: &agentUUID,
	}

	// Get retailers from service
	retailers, _, err := s.retailerService.ListRetailers(ctx, filters)
	if err != nil {
		s.logger.Error("Failed to get agent retailers", "agent_id", req.AgentId, "error", err)
		return nil, status.Error(codes.Internal, "failed to retrieve retailers")
	}

	// Convert to proto format
	pbRetailers := make([]*pb.Retailer, len(retailers))
	for i, retailer := range retailers {
		agentID := ""
		if retailer.AgentID != nil {
			agentID = retailer.AgentID.String()
		}

		pbRetailers[i] = &pb.Retailer{
			Id:           retailer.ID.String(),
			RetailerCode: retailer.RetailerCode,
			Name:         retailer.Name,
			Email:        retailer.Email,
			PhoneNumber:  retailer.PhoneNumber,
			Address:      retailer.Address,
			AgentId:      agentID,
			Status:       convertEntityStatus(retailer.Status),
			CreatedAt:    timestamppb.New(retailer.CreatedAt),
			UpdatedAt:    timestamppb.New(retailer.UpdatedAt),
			CreatedBy:    retailer.CreatedBy,
			UpdatedBy:    retailer.UpdatedBy,
		}
	}

	s.logger.Info("Successfully retrieved agent retailers", "agent_id", req.AgentId, "count", len(pbRetailers))

	return &pb.GetAgentRetailersResponse{
		Retailers: pbRetailers,
	}, nil
}

func (s *AgentManagementServer) GetRetailerAgent(ctx context.Context, req *pb.GetRetailerAgentRequest) (*pb.Agent, error) {
	// TODO: Implement actual service call
	return &pb.Agent{
		Id:          "agent-1",
		AgentCode:   "AGT-2025-000001",
		Name:        "Test Agent",
		Email:       "agent@test.com",
		PhoneNumber: "+233123456789",
		Address:     "Test Address",
		Status:      pb.EntityStatus_ENTITY_STATUS_ACTIVE,
		CreatedAt:   timestamppb.Now(),
		UpdatedAt:   timestamppb.Now(),
	}, nil
}

// POS Device operations - placeholder implementations
func (s *AgentManagementServer) CreatePOSDevice(ctx context.Context, req *pb.CreatePOSDeviceRequest) (*pb.POSDevice, error) {
	return &pb.POSDevice{}, nil
}

func (s *AgentManagementServer) GetPOSDevice(ctx context.Context, req *pb.GetPOSDeviceRequest) (*pb.POSDevice, error) {
	return &pb.POSDevice{}, nil
}

func (s *AgentManagementServer) UpdatePOSDevice(ctx context.Context, req *pb.UpdatePOSDeviceRequest) (*pb.POSDevice, error) {
	return &pb.POSDevice{}, nil
}

func (s *AgentManagementServer) ListPOSDevices(ctx context.Context, req *pb.ListPOSDevicesRequest) (*pb.ListPOSDevicesResponse, error) {
	retailerID, err := uuid.Parse(req.Filter.RetailerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid retailer ID: %v", err)
	}
	filter := &repositories.POSDeviceFilters{
		AssignedRetailerID: &retailerID,
	}
	devices, total, err := s.posDeviceService.ListPOSDevices(ctx, filter, req.Page, req.PageSize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list POS devices: %v", err)
	}

	return &pb.ListPOSDevicesResponse{
		Devices:    convertPOSDevicesToProto(devices),
		TotalCount: int32(total),
		Page:       req.Page,
		PageSize:   req.PageSize,
	}, nil
}

func (s *AgentManagementServer) AssignPOSDeviceToRetailer(ctx context.Context, req *pb.AssignPOSDeviceToRetailerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *AgentManagementServer) UnassignPOSDeviceFromRetailer(ctx context.Context, req *pb.UnassignPOSDeviceFromRetailerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// Commission Tier operations removed - now using direct commission percentage on agents

func convertPOSDevicesToProto(devices []models.POSDevice) []*pb.POSDevice {
	pbDevices := make([]*pb.POSDevice, len(devices))
	for i, device := range devices {
		pbDevices[i] = &pb.POSDevice{
			Id:              device.ID.String(),
			DeviceCode:      device.DeviceCode,
			Imei:            device.IMEI,
			Model:           device.Model,
			RetailerId:      device.AssignedRetailerID.String(),
			Status:          convertDeviceStatusToProto(device.Status),
			SoftwareVersion: device.SoftwareVersion,
			LastSync: func() *timestamppb.Timestamp {
				if device.LastSync != nil {
					return timestamppb.New(*device.LastSync)
				}
				return nil
			}(),
			LastTransaction: func() *timestamppb.Timestamp {
				if device.LastTransaction != nil {
					return timestamppb.New(*device.LastTransaction)
				}
				return nil
			}(),
			CreatedAt: timestamppb.New(device.CreatedAt),
			UpdatedAt: timestamppb.New(device.UpdatedAt),
		}
	}
	return pbDevices
}

func convertDeviceStatusToProto(status models.DeviceStatus) pb.DeviceStatus {
	switch status {
	case models.DeviceStatusAvailable:
		return pb.DeviceStatus_DEVICE_STATUS_AVAILABLE
	case models.DeviceStatusAssigned:
		return pb.DeviceStatus_DEVICE_STATUS_ASSIGNED
	case models.DeviceStatusFaulty:
		return pb.DeviceStatus_DEVICE_STATUS_MAINTENANCE
	default:
		return pb.DeviceStatus_DEVICE_STATUS_RETIRED
	}
}

// KYC operations - placeholder implementations
func (s *AgentManagementServer) CreateAgentKYC(ctx context.Context, req *pb.CreateAgentKYCRequest) (*pb.AgentKYC, error) {
	return &pb.AgentKYC{}, nil
}

func (s *AgentManagementServer) UpdateAgentKYCStatus(ctx context.Context, req *pb.UpdateAgentKYCStatusRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *AgentManagementServer) CreateRetailerKYC(ctx context.Context, req *pb.CreateRetailerKYCRequest) (*pb.RetailerKYC, error) {
	return &pb.RetailerKYC{}, nil
}

func (s *AgentManagementServer) UpdateRetailerKYCStatus(ctx context.Context, req *pb.UpdateRetailerKYCStatusRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// Analytics operations

func (s *AgentManagementServer) GetRetailerCount(ctx context.Context, req *pb.GetRetailerCountRequest) (*pb.GetRetailerCountResponse, error) {
	s.logger.Info("Getting total retailer count")

	// Call service to get total retailer count
	count, err := s.retailerService.GetTotalRetailerCount(ctx)
	if err != nil {
		s.logger.Error("Failed to get retailer count", "error", err)
		return nil, status.Error(codes.Internal, "failed to get retailer count")
	}

	s.logger.Info("Successfully retrieved retailer count", "count", count)

	return &pb.GetRetailerCountResponse{
		Count: count,
	}, nil
}

// Converter functions
func convertEntityStatus(status models.EntityStatus) pb.EntityStatus {
	switch status {
	case models.StatusActive:
		return pb.EntityStatus_ENTITY_STATUS_ACTIVE
	case models.StatusSuspended:
		return pb.EntityStatus_ENTITY_STATUS_SUSPENDED
	case models.StatusUnderReview:
		return pb.EntityStatus_ENTITY_STATUS_UNDER_REVIEW
	case models.StatusTerminated:
		return pb.EntityStatus_ENTITY_STATUS_TERMINATED
	default:
		return pb.EntityStatus_ENTITY_STATUS_UNSPECIFIED
	}
}

func convertProtobufToEntityStatus(status pb.EntityStatus) models.EntityStatus {
	switch status {
	case pb.EntityStatus_ENTITY_STATUS_ACTIVE:
		return models.StatusActive
	case pb.EntityStatus_ENTITY_STATUS_SUSPENDED:
		return models.StatusSuspended
	case pb.EntityStatus_ENTITY_STATUS_UNDER_REVIEW:
		return models.StatusUnderReview
	case pb.EntityStatus_ENTITY_STATUS_TERMINATED:
		return models.StatusTerminated
	default:
		return models.StatusActive
	}
}

// Helper functions for status mapping
func mapStatusToProto(status models.EntityStatus) pb.EntityStatus {
	switch status {
	case models.StatusActive:
		return pb.EntityStatus_ENTITY_STATUS_ACTIVE
	case models.StatusSuspended:
		return pb.EntityStatus_ENTITY_STATUS_SUSPENDED
	case models.StatusUnderReview:
		return pb.EntityStatus_ENTITY_STATUS_UNDER_REVIEW
	case models.StatusInactive:
		return pb.EntityStatus_ENTITY_STATUS_INACTIVE
	case models.StatusTerminated:
		return pb.EntityStatus_ENTITY_STATUS_TERMINATED
	default:
		return pb.EntityStatus_ENTITY_STATUS_UNSPECIFIED
	}
}

func mapProtoToStatus(status pb.EntityStatus) models.EntityStatus {
	switch status {
	case pb.EntityStatus_ENTITY_STATUS_ACTIVE:
		return models.StatusActive
	case pb.EntityStatus_ENTITY_STATUS_SUSPENDED:
		return models.StatusSuspended
	case pb.EntityStatus_ENTITY_STATUS_UNDER_REVIEW:
		return models.StatusUnderReview
	case pb.EntityStatus_ENTITY_STATUS_TERMINATED:
		return models.StatusTerminated
	default:
		return models.StatusActive
	}
}

// Helper functions for proto conversions

func convertAgentToProto(agent *models.Agent) *pb.Agent {
	if agent == nil {
		return nil
	}

	return &pb.Agent{
		Id:          agent.ID.String(),
		AgentCode:   agent.AgentCode,
		Name:        agent.BusinessName,
		Email:       agent.ContactEmail,
		PhoneNumber: agent.ContactPhone,
		Address:     agent.PhysicalAddress,
		Status:      mapStatusToProto(agent.Status),
		DateJoined:  timestamppb.New(agent.CreatedAt), // Using CreatedAt as DateJoined
		CreatedBy:   agent.CreatedBy,
		UpdatedBy:   agent.UpdatedBy,
		CreatedAt:   timestamppb.New(agent.CreatedAt),
		UpdatedAt:   timestamppb.New(agent.UpdatedAt),
	}
}

func convertRetailerToProto(retailer *models.Retailer) *pb.Retailer {
	if retailer == nil {
		return nil
	}

	var agentID string
	if retailer.AgentID != nil {
		agentID = retailer.AgentID.String()
	}

	return &pb.Retailer{
		Id:           retailer.ID.String(),
		RetailerCode: retailer.RetailerCode,
		Name:         retailer.Name,
		Email:        retailer.Email,
		PhoneNumber:  retailer.PhoneNumber,
		Address:      retailer.Address,
		AgentId:      agentID,
		Status:       mapStatusToProto(retailer.Status),
		DateJoined:   timestamppb.New(retailer.CreatedAt), // Using CreatedAt as DateJoined
		CreatedBy:    retailer.CreatedBy,
		UpdatedBy:    retailer.UpdatedBy,
		CreatedAt:    timestamppb.New(retailer.CreatedAt),
		UpdatedAt:    timestamppb.New(retailer.UpdatedAt),
	}
}

// RegisterServer registers the AgentManagementServer with the grpc server
func (s *AgentManagementServer) RegisterServer(server *grpc.Server) {
	pb.RegisterAgentManagementServiceServer(server, s)
}

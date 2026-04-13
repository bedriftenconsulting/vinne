package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/repositories"
)

// AgentService defines the interface for agent-specific operations
type AgentService interface {
	CreateAgent(ctx context.Context, req *CreateAgentRequest) (*models.Agent, error)
	GetAgent(ctx context.Context, agentID uuid.UUID) (*models.Agent, error)
	UpdateAgent(ctx context.Context, req *UpdateAgentRequest) (*models.Agent, error)
	ListAgents(ctx context.Context, req *ListAgentsRequest) (*ListAgentsResponse, error)
	UpdateAgentStatus(ctx context.Context, agentID uuid.UUID, status models.EntityStatus, updatedBy string) error
	DeleteAgent(ctx context.Context, agentID uuid.UUID, deletedBy string) error
}

// RetailerService defines the interface for retailer-specific operations
type RetailerService interface {
	CreateRetailer(ctx context.Context, req *CreateRetailerRequest) (*models.Retailer, error)
	GetRetailer(ctx context.Context, retailerID uuid.UUID) (*models.Retailer, error)
	GetRetailerByID(ctx context.Context, retailerID uuid.UUID) (*models.Retailer, error)
	UpdateRetailer(ctx context.Context, retailerID uuid.UUID, req *UpdateRetailerRequest) (*models.Retailer, error)
	ListRetailers(ctx context.Context, filters repositories.RetailerFilters) ([]models.Retailer, int, error)
	UpdateRetailerStatus(ctx context.Context, retailerID uuid.UUID, status models.EntityStatus, updatedBy string) error
	GetTotalRetailerCount(ctx context.Context) (int64, error)
}

// RetailerAssignmentService defines the interface for retailer assignment operations
type RetailerAssignmentService interface {
	GetAgentRetailers(ctx context.Context, agentID uuid.UUID) ([]models.Retailer, error)
	AssignRetailerToAgent(ctx context.Context, retailerID uuid.UUID, agentID uuid.UUID, assignedBy string) error
	ReassignRetailerToAgent(ctx context.Context, retailerID uuid.UUID, newAgentID uuid.UUID, reassignedBy string) error
	UnassignRetailerFromAgent(ctx context.Context, retailerID uuid.UUID, unassignedBy string) error
}

// POSDeviceService defines the interface for POS device operations
type POSDeviceService interface {
	ListPOSDevices(ctx context.Context, filter *repositories.POSDeviceFilters, page int32, pageSize int32) ([]models.POSDevice, int, error)
}

// ServiceConfig holds configuration for the services
type ServiceConfig struct {
	WalletServiceAddress             string   // e.g., "localhost:50053"
	AgentAuthServiceAddress          string   // e.g., "localhost:50052"
	DefaultAgentCommissionPercentage float64  // e.g., 30.0 for 30%
	KafkaBrokers                     []string // e.g., ["localhost:9092"]
}

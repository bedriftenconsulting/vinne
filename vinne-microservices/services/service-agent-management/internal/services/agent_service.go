package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/clients"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/repositories"
	"github.com/randco/randco-microservices/shared/events"
)

// agentService handles agent-specific business logic
type agentService struct {
	repos                            *repositories.Repositories
	walletClient                     *clients.WalletClient
	agentAuthClient                  *clients.AgentAuthClient
	eventBus                         events.EventBus
	defaultAgentCommissionPercentage float64
}

// NewAgentService creates a new agent service
func NewAgentService(repos *repositories.Repositories, config *ServiceConfig) AgentService {
	service := &agentService{
		repos:                            repos,
		defaultAgentCommissionPercentage: 30.0, // Default fallback
	}

	if config != nil {
		// Set commission percentage from config
		if config.DefaultAgentCommissionPercentage > 0 {
			service.defaultAgentCommissionPercentage = config.DefaultAgentCommissionPercentage
		}

		// Initialize wallet client if address provided
		if config.WalletServiceAddress != "" {
			walletClient, err := clients.NewWalletClient(config.WalletServiceAddress)
			if err != nil {
				log.Printf("Warning: failed to connect to wallet service: %v", err)
			} else {
				service.walletClient = walletClient
			}
		}

		// Initialize agent auth client if address provided
		if config.AgentAuthServiceAddress != "" {
			agentAuthClient, err := clients.NewAgentAuthClient(config.AgentAuthServiceAddress)
			if err != nil {
				log.Printf("Warning: failed to connect to agent auth service: %v", err)
			} else {
				service.agentAuthClient = agentAuthClient
			}
		}

		// Initialize event bus if Kafka brokers provided
		if len(config.KafkaBrokers) > 0 {
			eventBus, err := events.NewKafkaEventBus(config.KafkaBrokers)
			if err != nil {
				log.Printf("Warning: failed to connect to Kafka: %v", err)
			} else {
				service.eventBus = eventBus
			}
		}
	}

	return service
}

// CreateAgent creates a new agent with business validation
func (s *agentService) CreateAgent(ctx context.Context, req *CreateAgentRequest) (*models.Agent, error) {
	// Initial validation using request validation
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check for duplicates by email only if email is provided
	if req.ContactEmail != "" {
		existing, err := s.repos.Agent.GetByEmail(ctx, req.ContactEmail)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to check existing agent by email: %w", err)
		}
		if existing != nil {
			return nil, fmt.Errorf("agent with email %s already exists", req.ContactEmail)
		}
	}

	// TODO: Check for duplicates by phone when GetByPhone method is added to repository

	// Generate unique agent code
	agentCode, err := s.repos.Agent.GetNextAgentCode(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate agent code: %w", err)
	}

	// Set commission percentage
	commissionPercentage := s.defaultAgentCommissionPercentage
	if req.CommissionPercentage != nil && *req.CommissionPercentage > 0 {
		commissionPercentage = *req.CommissionPercentage
	}

	// Create agent model with business logic
	agent := &models.Agent{
		ID:                   uuid.New(),
		AgentCode:            agentCode,
		BusinessName:         req.BusinessName,
		RegistrationNumber:   req.RegistrationNumber,
		TaxID:                req.TaxID,
		ContactEmail:         req.ContactEmail,
		ContactPhone:         req.ContactPhone,
		PrimaryContactName:   req.PrimaryContactName,
		PhysicalAddress:      req.PhysicalAddress,
		City:                 req.City,
		Region:               req.Region,
		GPSCoordinates:       sql.NullString{String: req.GPSCoordinates, Valid: req.GPSCoordinates != ""},
		BankName:             req.BankName,
		BankAccountNumber:    req.BankAccountNumber,
		BankAccountName:      req.BankAccountName,
		Status:               models.StatusActive,                   // Start as active
		OnboardingMethod:     models.OnboardingRandLotteryLtdDirect, // Default to direct onboarding
		CommissionPercentage: commissionPercentage,
		CreatedBy:            req.CreatedBy,
		UpdatedBy:            req.CreatedBy,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	// Additional business validation using model methods
	if err := agent.Validate(); err != nil {
		return nil, fmt.Errorf("agent validation failed: %w", err)
	}

	// Persist to database
	err = s.repos.Agent.Create(ctx, agent)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Create wallet for agent asynchronously
	if s.walletClient != nil {
		go func(parentCtx context.Context) {
			retryCtx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
			defer cancel()
			_ = retryCtx
			// Wallet creation will be implemented when wallet client is ready
		}(ctx)
	}

	// Publish agent created event
	if s.eventBus != nil {
		agentData := events.AgentData{
			ID:                   agent.ID.String(),
			AgentCode:            agent.AgentCode,
			Name:                 agent.BusinessName,
			Email:                agent.ContactEmail,
			PhoneNumber:          agent.ContactPhone,
			Status:               string(agent.Status),
			City:                 agent.City,
			Region:               agent.Region,
			CommissionPercentage: agent.CommissionPercentage,
		}

		event := events.NewAgentCreatedEvent(
			"service-agent-management",
			agentData,
			agent.CreatedBy,
		)

		if err := s.eventBus.Publish(ctx, "agent.events", event); err != nil {
			log.Printf("Failed to publish agent created event for agent %s: %v", agent.AgentCode, err)
		} else {
			log.Printf("Published agent created event for agent %s", agent.AgentCode)
		}
	}

	// Note: Auth credentials are created by the api-gateway after agent creation
	// to avoid double-creation. The api-gateway calls CreateAgentAuth separately.

	return agent, nil
}

// generateInitialPassword generates a random 6-digit numeric password for new agents
func generateInitialPassword() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

func (s *agentService) GetAgent(ctx context.Context, agentID uuid.UUID) (*models.Agent, error) {
	agent, err := s.repos.Agent.GetByID(ctx, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("agent not found")
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}
	return agent, nil
}

// UpdateAgent updates an existing agent
func (s *agentService) UpdateAgent(ctx context.Context, req *UpdateAgentRequest) (*models.Agent, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing agent
	agent, err := s.repos.Agent.GetByID(ctx, req.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("agent not found")
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Track changes for event
	changes := make(map[string]interface{})

	// Update fields if provided
	if req.BusinessName != nil && *req.BusinessName != agent.BusinessName {
		changes["business_name"] = *req.BusinessName
		agent.BusinessName = *req.BusinessName
	}
	if req.RegistrationNumber != nil && *req.RegistrationNumber != agent.RegistrationNumber {
		changes["registration_number"] = *req.RegistrationNumber
		agent.RegistrationNumber = *req.RegistrationNumber
	}
	if req.TaxID != nil && *req.TaxID != agent.TaxID {
		changes["tax_id"] = *req.TaxID
		agent.TaxID = *req.TaxID
	}
	if req.ContactEmail != nil && *req.ContactEmail != agent.ContactEmail {
		// Check for duplicate email
		existing, err := s.repos.Agent.GetByEmail(ctx, *req.ContactEmail)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to check existing agent by email: %w", err)
		}
		if existing != nil && existing.ID != agent.ID {
			return nil, fmt.Errorf("agent with email %s already exists", *req.ContactEmail)
		}
		changes["contact_email"] = *req.ContactEmail
		agent.ContactEmail = *req.ContactEmail
	}
	if req.ContactPhone != nil && *req.ContactPhone != agent.ContactPhone {
		// TODO: Check for duplicate phone when GetByPhone method is added to repository
		changes["contact_phone"] = *req.ContactPhone
		agent.ContactPhone = *req.ContactPhone
	}
	if req.PrimaryContactName != nil && *req.PrimaryContactName != agent.PrimaryContactName {
		changes["primary_contact_name"] = *req.PrimaryContactName
		agent.PrimaryContactName = *req.PrimaryContactName
	}
	if req.PhysicalAddress != nil && *req.PhysicalAddress != agent.PhysicalAddress {
		changes["physical_address"] = *req.PhysicalAddress
		agent.PhysicalAddress = *req.PhysicalAddress
	}
	if req.Region != nil && *req.Region != agent.Region {
		changes["region"] = *req.Region
		agent.Region = *req.Region
	}
	if req.City != nil && *req.City != agent.City {
		changes["city"] = *req.City
		agent.City = *req.City
	}
	if req.GPSCoordinates != nil {
		newGPS := sql.NullString{String: *req.GPSCoordinates, Valid: *req.GPSCoordinates != ""}
		if newGPS.String != agent.GPSCoordinates.String {
			changes["gps_coordinates"] = *req.GPSCoordinates
			agent.GPSCoordinates = newGPS
		}
	}
	if req.BankName != nil && *req.BankName != agent.BankName {
		changes["bank_name"] = *req.BankName
		agent.BankName = *req.BankName
	}
	if req.BankAccountNumber != nil && *req.BankAccountNumber != agent.BankAccountNumber {
		changes["bank_account_number"] = *req.BankAccountNumber
		agent.BankAccountNumber = *req.BankAccountNumber
	}
	if req.BankAccountName != nil && *req.BankAccountName != agent.BankAccountName {
		changes["bank_account_name"] = *req.BankAccountName
		agent.BankAccountName = *req.BankAccountName
	}
	if req.CommissionPercentage != nil && *req.CommissionPercentage != agent.CommissionPercentage {
		changes["commission_percentage"] = *req.CommissionPercentage
		agent.CommissionPercentage = *req.CommissionPercentage
	}

	// Update metadata
	agent.UpdatedBy = req.UpdatedBy
	agent.UpdatedAt = time.Now()

	// Validate updated agent
	if err := agent.Validate(); err != nil {
		return nil, fmt.Errorf("agent validation failed: %w", err)
	}

	// Persist changes
	err = s.repos.Agent.Update(ctx, agent)
	if err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	// Publish agent updated event if there were changes
	if s.eventBus != nil && len(changes) > 0 {
		agentData := events.AgentData{
			ID:                   agent.ID.String(),
			AgentCode:            agent.AgentCode,
			Name:                 agent.BusinessName,
			Email:                agent.ContactEmail,
			PhoneNumber:          agent.ContactPhone,
			Status:               string(agent.Status),
			City:                 agent.City,
			Region:               agent.Region,
			CommissionPercentage: agent.CommissionPercentage,
		}

		event := events.NewAgentUpdatedEvent(
			"service-agent-management",
			agentData,
			agent.UpdatedBy,
			changes,
		)

		if err := s.eventBus.Publish(ctx, "agent.events", event); err != nil {
			log.Printf("Failed to publish agent updated event for agent %s: %v", agent.AgentCode, err)
		} else {
			log.Printf("Published agent updated event for agent %s with %d changes", agent.AgentCode, len(changes))
		}
	}

	return agent, nil
}

// ListAgents retrieves agents with filtering
func (s *agentService) ListAgents(ctx context.Context, req *ListAgentsRequest) (*ListAgentsResponse, error) {
	// Build filters
	filters := repositories.AgentFilters{
		Status:       req.Status,
		City:         req.City,
		Region:       req.Region,
		ContactEmail: req.ContactEmail,
		ContactPhone: req.ContactPhone,
		BusinessName: req.BusinessName,
		Limit:        req.Limit,
		Offset:       req.Offset,
	}

	// Get agents from repository
	agents, err := s.repos.Agent.List(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	// Get total count
	total, err := s.repos.Agent.Count(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to count agents: %w", err)
	}

	// Convert []models.Agent to []*models.Agent
	agentPointers := make([]*models.Agent, len(agents))
	for i := range agents {
		agentPointers[i] = &agents[i]
	}

	return &ListAgentsResponse{
		Agents: agentPointers,
		Total:  total,
	}, nil
}

// UpdateAgentStatus updates the status of an agent
func (s *agentService) UpdateAgentStatus(ctx context.Context, agentID uuid.UUID, status models.EntityStatus, updatedBy string) error {
	// Get existing agent to ensure it exists and get current status
	agent, err := s.repos.Agent.GetByID(ctx, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("agent not found")
		}
		return fmt.Errorf("failed to get agent: %w", err)
	}

	oldStatus := agent.Status

	// Check if status transition is valid using model method
	if !agent.CanTransitionTo(status) {
		return fmt.Errorf("invalid status transition from %s to %s", oldStatus, status)
	}

	// Update status
	err = s.repos.AgentStatus.UpdateStatus(ctx, agentID, status, updatedBy)
	if err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}

	// Publish status change event
	if s.eventBus != nil {
		event := events.NewAgentStatusChangedEvent(
			"service-agent-management",
			agentID.String(),
			agent.AgentCode,
			string(oldStatus),
			string(status),
			updatedBy,
			"", // No specific reason provided
		)

		if err := s.eventBus.Publish(ctx, "agent.events", event); err != nil {
			log.Printf("Failed to publish agent status changed event for agent %s: %v", agent.AgentCode, err)
		} else {
			log.Printf("Published agent status changed event for agent %s: %s -> %s", agent.AgentCode, oldStatus, status)
		}
	}

	return nil
}

// DeleteAgent removes an agent from the system
func (s *agentService) DeleteAgent(ctx context.Context, agentID uuid.UUID, deletedBy string) error {
	// Get existing agent to ensure it exists
	agent, err := s.repos.Agent.GetByID(ctx, agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	// Check if agent has any active retailers
	retailers, err := s.repos.RetailerRelationship.GetByAgentID(ctx, agentID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check agent retailers: %w", err)
	}

	// Don't delete if agent has active retailers
	if len(retailers) > 0 {
		activeCount := 0
		for _, r := range retailers {
			if r.Status == models.StatusActive {
				activeCount++
			}
		}
		if activeCount > 0 {
			return fmt.Errorf("cannot delete agent with %d active retailers", activeCount)
		}
	}

	// Note: POS devices are linked to retailers, not directly to agents
	// If agent has retailers, the retailers check above will prevent deletion

	// Delete agent from database
	err = s.repos.Agent.Delete(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	// Publish agent deleted event
	if s.eventBus != nil {
		event := events.NewAgentDeletedEvent(
			"service-agent-management",
			agentID.String(),
			agent.AgentCode,
			deletedBy,
		)

		if err := s.eventBus.Publish(ctx, "agent.events", event); err != nil {
			log.Printf("Failed to publish agent deleted event for agent %s: %v", agent.AgentCode, err)
		} else {
			log.Printf("Published agent deleted event for agent %s", agent.AgentCode)
		}
	}

	// TODO: Delete agent from auth service

	return nil
}

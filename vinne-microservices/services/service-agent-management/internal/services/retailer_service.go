package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/clients"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/repositories"
	"github.com/randco/randco-microservices/shared/events"
)

// retailerService handles retailer-specific business logic
type retailerService struct {
	repos           *repositories.Repositories
	walletClient    *clients.WalletClient
	agentAuthClient *clients.AgentAuthClient
	eventBus        events.EventBus
}

// NewRetailerService creates a new retailer service
func NewRetailerService(repos *repositories.Repositories, config *ServiceConfig) RetailerService {
	service := &retailerService{repos: repos}

	if config != nil {
		if config.WalletServiceAddress != "" {
			walletClient, err := clients.NewWalletClient(config.WalletServiceAddress)
			if err != nil {
				log.Printf("Warning: failed to connect to wallet service: %v", err)
			} else {
				service.walletClient = walletClient
			}
		}

		if config.AgentAuthServiceAddress != "" {
			agentAuthClient, err := clients.NewAgentAuthClient(config.AgentAuthServiceAddress)
			if err != nil {
				log.Printf("Warning: failed to connect to agent auth service: %v", err)
			} else {
				service.agentAuthClient = agentAuthClient
			}
		}

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

// CreateRetailer creates a new retailer with business validation
func (s *retailerService) CreateRetailer(ctx context.Context, req *CreateRetailerRequest) (*models.Retailer, error) {
	// Sanitize optional email — trim whitespace, clear if invalid so it doesn't block creation
	req.ContactEmail = strings.TrimSpace(req.ContactEmail)

	// Log retailer creation start
	log.Printf("[AGENT-MGMT] Creating retailer - AgentID: %s, Name: %s, Phone: %s, Email: %s, Address: %s",
		req.AgentID.String(), req.BusinessName, req.ContactPhone, req.ContactEmail, req.PhysicalAddress)

	// Initial validation using request validation
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if this is an independent retailer (zero UUID)
	isIndependent := req.AgentID == uuid.Nil

	// If agent-managed, check if agent exists and is active
	var agent *models.Agent
	var agentCode string
	if !isIndependent {
		var err error
		agent, err = s.repos.Agent.GetByID(ctx, req.AgentID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("agent not found")
			}
			return nil, fmt.Errorf("failed to get agent: %w", err)
		}

		if agent.Status != models.StatusActive {
			return nil, fmt.Errorf("cannot create retailer for inactive agent")
		}
		agentCode = agent.AgentCode
	} else {
		// For independent retailers, use empty string for code generation
		agentCode = ""
	}

	// Check for duplicates by phone
	existing, err := s.repos.Retailer.GetByPhone(ctx, req.ContactPhone)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing retailer by phone: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("retailer with phone %s already exists", req.ContactPhone)
	}

	// Generate unique retailer code based on agent code (or empty for independent)
	retailerCode, err := s.repos.Retailer.GetNextRetailerCode(ctx, agentCode)
	if err != nil {
		return nil, fmt.Errorf("failed to generate retailer code: %w", err)
	}

	// For independent retailers, AgentID should be nil, not zero UUID
	var agentIDPtr *uuid.UUID
	if !isIndependent {
		agentIDPtr = &req.AgentID
	}

	// Create retailer model with business logic
	retailer := &models.Retailer{
		ID:               uuid.New(),
		RetailerCode:     retailerCode,
		Name:             req.BusinessName,
		OwnerName:        req.ContactName,
		PhoneNumber:      req.ContactPhone,
		Email:            req.ContactEmail,
		Address:          req.PhysicalAddress,
		Region:           req.Region,
		City:             req.City,
		AgentID:          agentIDPtr, // nil for independent retailers, pointer to AgentID for agent-managed
		OnboardingMethod: models.OnboardingMethod(req.OnboardingMethod),
		Status:           models.StatusActive, // Start as active
		CreatedBy:        req.CreatedBy,
		UpdatedBy:        req.CreatedBy,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Additional business validation using model methods
	if err := retailer.Validate(); err != nil {
		return nil, fmt.Errorf("retailer validation failed: %w", err)
	}

	// Persist to database
	err = s.repos.Retailer.Create(ctx, retailer)
	if err != nil {
		log.Printf("[AGENT-MGMT] Failed to create retailer in database: %v", err)
		return nil, fmt.Errorf("failed to create retailer: %w", err)
	}

	log.Printf("[AGENT-MGMT] Retailer created in database - ID: %s, Code: %s, AgentID: %s",
		retailer.ID.String(), retailer.RetailerCode, req.AgentID.String())

	// Create wallet for retailer asynchronously
	if s.walletClient != nil {
		go func() {
			retryCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// For independent retailers, pass nil as parent agent ID
			var parentAgentID *uuid.UUID
			if !isIndependent {
				parentAgentID = &req.AgentID
			}

			// Create retailer wallets (stake and winning wallets)
			err := s.walletClient.CreateRetailerWallets(retryCtx, retailer.ID, retailer.RetailerCode, parentAgentID, req.CreatedBy)
			if err != nil {
				log.Printf("Failed to create retailer wallets for retailer %s (%s): %v", retailer.RetailerCode, retailer.ID, err)
			} else {
				log.Printf("Successfully created wallets for retailer %s (%s)", retailer.RetailerCode, retailer.ID)
			}
		}()
	} else {
		log.Printf("Warning: Wallet client not initialized, skipping wallet creation for retailer %s", retailer.RetailerCode)
	}

	// Publish retailer created event
	if s.eventBus != nil {
		retailerData := events.RetailerData{
			ID:           retailer.ID.String(),
			RetailerCode: retailer.RetailerCode,
			Name:         retailer.Name,
			Email:        retailer.Email,
			PhoneNumber:  retailer.PhoneNumber,
			AgentID:      req.AgentID.String(),
			AgentCode:    agentCode, // Use agentCode variable which is empty for independent retailers
			Status:       string(retailer.Status),
			City:         retailer.City,
			Region:       retailer.Region,
		}

		event := events.NewRetailerCreatedEvent(
			"service-agent-management",
			retailerData,
			retailer.CreatedBy,
		)

		if err := s.eventBus.Publish(ctx, "retailer.events", event); err != nil {
			log.Printf("Failed to publish retailer created event for retailer %s: %v", retailer.RetailerCode, err)
		} else {
			log.Printf("Published retailer created event for retailer %s", retailer.RetailerCode)
		}
	}

	return retailer, nil
}

// GetRetailer retrieves a retailer by ID
func (s *retailerService) GetRetailer(ctx context.Context, retailerID uuid.UUID) (*models.Retailer, error) {
	retailer, err := s.repos.Retailer.GetByID(ctx, retailerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("retailer not found")
		}
		return nil, fmt.Errorf("failed to get retailer: %w", err)
	}
	return retailer, nil
}

// GetRetailerByID retrieves a retailer by ID (duplicate method for compatibility)
func (s *retailerService) GetRetailerByID(ctx context.Context, retailerID uuid.UUID) (*models.Retailer, error) {
	return s.GetRetailer(ctx, retailerID)
}

// UpdateRetailer updates an existing retailer
func (s *retailerService) UpdateRetailer(ctx context.Context, retailerID uuid.UUID, req *UpdateRetailerRequest) (*models.Retailer, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing retailer
	retailer, err := s.repos.Retailer.GetByID(ctx, retailerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("retailer not found")
		}
		return nil, fmt.Errorf("failed to get retailer: %w", err)
	}

	// Track changes for event
	changes := make(map[string]interface{})

	// Update fields if provided
	if req.BusinessName != nil && *req.BusinessName != retailer.Name {
		changes["business_name"] = *req.BusinessName
		retailer.Name = *req.BusinessName
	}
	if req.ContactName != nil && *req.ContactName != retailer.OwnerName {
		changes["contact_name"] = *req.ContactName
		retailer.OwnerName = *req.ContactName
	}
	if req.ContactPhone != nil && *req.ContactPhone != retailer.PhoneNumber {
		// Check for duplicate phone
		existing, err := s.repos.Retailer.GetByPhone(ctx, *req.ContactPhone)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to check existing retailer by phone: %w", err)
		}
		if existing != nil && existing.ID != retailer.ID {
			return nil, fmt.Errorf("retailer with phone %s already exists", *req.ContactPhone)
		}
		changes["contact_phone"] = *req.ContactPhone
		retailer.PhoneNumber = *req.ContactPhone
	}
	if req.ContactEmail != nil && *req.ContactEmail != retailer.Email {
		changes["contact_email"] = *req.ContactEmail
		retailer.Email = *req.ContactEmail
	}
	if req.PhysicalAddress != nil && *req.PhysicalAddress != retailer.Address {
		changes["physical_address"] = *req.PhysicalAddress
		retailer.Address = *req.PhysicalAddress
	}
	if req.Region != nil && *req.Region != retailer.Region {
		changes["region"] = *req.Region
		retailer.Region = *req.Region
	}
	if req.City != nil && *req.City != retailer.City {
		changes["city"] = *req.City
		retailer.City = *req.City
	}

	// Update metadata
	retailer.UpdatedBy = req.UpdatedBy
	retailer.UpdatedAt = time.Now()

	// Validate updated retailer
	if err := retailer.Validate(); err != nil {
		return nil, fmt.Errorf("retailer validation failed: %w", err)
	}

	// Persist changes
	err = s.repos.Retailer.Update(ctx, retailer)
	if err != nil {
		return nil, fmt.Errorf("failed to update retailer: %w", err)
	}

	// TODO: Publish retailer updated event if there were changes

	return retailer, nil
}

// ListRetailers retrieves retailers with filtering
func (s *retailerService) ListRetailers(ctx context.Context, filters repositories.RetailerFilters) ([]models.Retailer, int, error) {
	// Get retailers from repository
	retailers, err := s.repos.Retailer.List(ctx, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list retailers: %w", err)
	}

	// Get total count
	total, err := s.repos.Retailer.Count(ctx, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count retailers: %w", err)
	}

	return retailers, total, nil
}

// UpdateRetailerStatus updates the status of a retailer
func (s *retailerService) UpdateRetailerStatus(ctx context.Context, retailerID uuid.UUID, status models.EntityStatus, updatedBy string) error {
	// Get existing retailer to ensure it exists and get current status
	retailer, err := s.repos.Retailer.GetByID(ctx, retailerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("retailer not found")
		}
		return fmt.Errorf("failed to get retailer: %w", err)
	}

	oldStatus := retailer.Status

	// Check if status transition is valid using model method
	if !retailer.CanTransitionTo(status) {
		return fmt.Errorf("invalid status transition from %s to %s", oldStatus, status)
	}

	// Update status
	err = s.repos.RetailerStatus.UpdateStatus(ctx, retailerID, status, updatedBy)
	if err != nil {
		return fmt.Errorf("failed to update retailer status: %w", err)
	}

	// TODO: Publish status change event

	return nil
}

// GetTotalRetailerCount retrieves the total count of all retailers
func (s *retailerService) GetTotalRetailerCount(ctx context.Context) (int64, error) {
	// Get total count using empty filters to count all retailers
	total, err := s.repos.Retailer.Count(ctx, repositories.RetailerFilters{})
	if err != nil {
		return 0, fmt.Errorf("failed to count retailers: %w", err)
	}

	// Convert int to int64
	return int64(total), nil
}

package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/repositories"
	"github.com/randco/randco-microservices/shared/events"
)

// retailerAssignmentService handles retailer-agent assignment business logic
type retailerAssignmentService struct {
	repos    *repositories.Repositories
	eventBus events.EventBus
}

// NewRetailerAssignmentService creates a new retailer assignment service
func NewRetailerAssignmentService(repos *repositories.Repositories) RetailerAssignmentService {
	service := &retailerAssignmentService{
		repos: repos,
	}

	// Initialize event bus if available (optional for assignment service)
	// This could be passed via config if needed
	return service
}

// GetAgentRetailers retrieves all retailers assigned to an agent
func (s *retailerAssignmentService) GetAgentRetailers(ctx context.Context, agentID uuid.UUID) ([]models.Retailer, error) {
	// Verify agent exists
	_, err := s.repos.Agent.GetByID(ctx, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("agent not found")
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Get retailers assigned to this agent
	retailers, err := s.repos.RetailerRelationship.GetByAgentID(ctx, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []models.Retailer{}, nil // Return empty slice if no retailers
		}
		return nil, fmt.Errorf("failed to get agent retailers: %w", err)
	}

	return retailers, nil
}

// AssignRetailerToAgent assigns a retailer to an agent
func (s *retailerAssignmentService) AssignRetailerToAgent(ctx context.Context, retailerID uuid.UUID, agentID uuid.UUID, assignedBy string) error {
	// Verify retailer exists
	retailer, err := s.repos.Retailer.GetByID(ctx, retailerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("retailer not found")
		}
		return fmt.Errorf("failed to get retailer: %w", err)
	}

	// Check if retailer is already assigned
	if retailer.AgentID != nil {
		return fmt.Errorf("retailer is already assigned to an agent")
	}

	// Verify agent exists and is active
	agent, err := s.repos.Agent.GetByID(ctx, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("agent not found")
		}
		return fmt.Errorf("failed to get agent: %w", err)
	}

	if agent.Status != models.StatusActive {
		return fmt.Errorf("cannot assign retailer to inactive agent")
	}

	// Perform assignment
	err = s.repos.RetailerRelationship.AssignToAgent(ctx, retailerID, agentID, assignedBy)
	if err != nil {
		return fmt.Errorf("failed to assign retailer to agent: %w", err)
	}

	// TODO: Publish retailer assigned event
	if s.eventBus != nil {
		log.Printf("Retailer %s assigned to agent %s by %s", retailer.RetailerCode, agent.AgentCode, assignedBy)
	}

	return nil
}

// ReassignRetailerToAgent reassigns a retailer from one agent to another
func (s *retailerAssignmentService) ReassignRetailerToAgent(ctx context.Context, retailerID uuid.UUID, newAgentID uuid.UUID, reassignedBy string) error {
	// Verify retailer exists
	retailer, err := s.repos.Retailer.GetByID(ctx, retailerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("retailer not found")
		}
		return fmt.Errorf("failed to get retailer: %w", err)
	}

	// Check if retailer is currently assigned
	if retailer.AgentID == nil {
		return fmt.Errorf("retailer is not currently assigned to any agent")
	}

	// Check if already assigned to the target agent
	if *retailer.AgentID == newAgentID {
		return fmt.Errorf("retailer is already assigned to this agent")
	}

	// Verify new agent exists and is active
	newAgent, err := s.repos.Agent.GetByID(ctx, newAgentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("new agent not found")
		}
		return fmt.Errorf("failed to get new agent: %w", err)
	}

	if newAgent.Status != models.StatusActive {
		return fmt.Errorf("cannot reassign retailer to inactive agent")
	}

	oldAgentID := *retailer.AgentID

	// First unassign from current agent
	err = s.repos.RetailerRelationship.UnassignFromAgent(ctx, retailerID, reassignedBy)
	if err != nil {
		return fmt.Errorf("failed to unassign retailer from current agent: %w", err)
	}

	// Then assign to new agent
	err = s.repos.RetailerRelationship.AssignToAgent(ctx, retailerID, newAgentID, reassignedBy)
	if err != nil {
		// Try to rollback by reassigning to old agent
		_ = s.repos.RetailerRelationship.AssignToAgent(ctx, retailerID, oldAgentID, reassignedBy)
		return fmt.Errorf("failed to assign retailer to new agent: %w", err)
	}

	// TODO: Publish retailer reassigned event
	if s.eventBus != nil {
		log.Printf("Retailer %s reassigned from agent %s to agent %s by %s",
			retailer.RetailerCode, oldAgentID, newAgent.AgentCode, reassignedBy)
	}

	return nil
}

// UnassignRetailerFromAgent unassigns a retailer from their agent
func (s *retailerAssignmentService) UnassignRetailerFromAgent(ctx context.Context, retailerID uuid.UUID, unassignedBy string) error {
	// Verify retailer exists
	retailer, err := s.repos.Retailer.GetByID(ctx, retailerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("retailer not found")
		}
		return fmt.Errorf("failed to get retailer: %w", err)
	}

	// Check if retailer is currently assigned
	if retailer.AgentID == nil {
		return fmt.Errorf("retailer is not assigned to any agent")
	}

	// Perform unassignment
	err = s.repos.RetailerRelationship.UnassignFromAgent(ctx, retailerID, unassignedBy)
	if err != nil {
		return fmt.Errorf("failed to unassign retailer from agent: %w", err)
	}

	// TODO: Publish retailer unassigned event
	if s.eventBus != nil {
		log.Printf("Retailer %s unassigned from agent by %s", retailer.RetailerCode, unassignedBy)
	}

	return nil
}

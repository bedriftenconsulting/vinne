package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
)

// Agent Request/Response structures

// CreateAgentRequest represents a request to create a new agent
type CreateAgentRequest struct {
	BusinessName         string   `json:"business_name"`
	RegistrationNumber   string   `json:"registration_number"`
	TaxID                string   `json:"tax_id"`
	ContactEmail         string   `json:"contact_email"`
	ContactPhone         string   `json:"contact_phone"`
	PrimaryContactName   string   `json:"primary_contact_name"`
	PhysicalAddress      string   `json:"physical_address"`
	City                 string   `json:"city"`
	Region               string   `json:"region"`
	GPSCoordinates       string   `json:"gps_coordinates"`
	BankName             string   `json:"bank_name"`
	BankAccountNumber    string   `json:"bank_account_number"`
	BankAccountName      string   `json:"bank_account_name"`
	CommissionPercentage *float64 `json:"commission_percentage"` // Optional, uses default if not provided
	CreatedBy            string   `json:"created_by"`
}

// Validate validates the create agent request
func (r *CreateAgentRequest) Validate() error {
	if r.BusinessName == "" {
		return fmt.Errorf("business name is required")
	}
	if r.ContactPhone == "" {
		return fmt.Errorf("contact phone is required")
	}
	if r.CreatedBy == "" {
		return fmt.Errorf("created by is required")
	}
	return nil
}

// UpdateAgentRequest represents a request to update an existing agent
type UpdateAgentRequest struct {
	ID                   uuid.UUID `json:"id"`
	BusinessName         *string   `json:"business_name"`
	RegistrationNumber   *string   `json:"registration_number"`
	TaxID                *string   `json:"tax_id"`
	ContactEmail         *string   `json:"contact_email"`
	ContactPhone         *string   `json:"contact_phone"`
	PrimaryContactName   *string   `json:"primary_contact_name"`
	PhysicalAddress      *string   `json:"physical_address"`
	City                 *string   `json:"city"`
	Region               *string   `json:"region"`
	GPSCoordinates       *string   `json:"gps_coordinates"`
	BankName             *string   `json:"bank_name"`
	BankAccountNumber    *string   `json:"bank_account_number"`
	BankAccountName      *string   `json:"bank_account_name"`
	CommissionPercentage *float64  `json:"commission_percentage"` // Commission as percentage (e.g., 30 for 30%)
	UpdatedBy            string    `json:"updated_by"`
}

// Validate validates the update agent request
func (r *UpdateAgentRequest) Validate() error {
	if r.UpdatedBy == "" {
		return fmt.Errorf("updated by is required")
	}
	return nil
}

// ListAgentsRequest represents a request to list agents with filtering
type ListAgentsRequest struct {
	Status         *models.EntityStatus `json:"status"`
	BusinessName   *string              `json:"business_name"`
	ContactEmail   *string              `json:"contact_email"`
	ContactPhone   *string              `json:"contact_phone"`
	Region         *string              `json:"region"`
	City           *string              `json:"city"`
	CreatedAfter   *time.Time           `json:"created_after"`
	CreatedBefore  *time.Time           `json:"created_before"`
	Limit          int                  `json:"limit"`
	Offset         int                  `json:"offset"`
	OrderBy        string               `json:"order_by"`
	OrderDirection string               `json:"order_direction"`
}

// ListAgentsResponse represents the response from listing agents
type ListAgentsResponse struct {
	Agents []*models.Agent `json:"agents"`
	Total  int             `json:"total"`
}

// Retailer Request/Response structures

// CreateRetailerRequest represents a request to create a new retailer
type CreateRetailerRequest struct {
	BusinessName     string    `json:"business_name"`
	ContactName      string    `json:"contact_name"`
	ContactPhone     string    `json:"contact_phone"`
	ContactEmail     string    `json:"contact_email"`
	PhysicalAddress  string    `json:"physical_address"`
	PostalAddress    string    `json:"postal_address"`
	Region           string    `json:"region"`
	City             string    `json:"city"`
	District         string    `json:"district"`
	AgentID          uuid.UUID `json:"agent_id"`
	OnboardingMethod string    `json:"onboarding_method"`
	CreatedBy        string    `json:"created_by"`
}

// Validate validates the create retailer request
func (r *CreateRetailerRequest) Validate() error {
	if r.BusinessName == "" {
		return fmt.Errorf("business name is required")
	}
	if r.ContactName == "" {
		return fmt.Errorf("contact name is required")
	}
	if r.ContactPhone == "" {
		return fmt.Errorf("contact phone is required")
	}
	// Email and address are optional
	if r.CreatedBy == "" {
		return fmt.Errorf("created by is required")
	}
	return nil
}

// UpdateRetailerRequest represents a request to update an existing retailer
type UpdateRetailerRequest struct {
	BusinessName    *string `json:"business_name"`
	ContactName     *string `json:"contact_name"`
	ContactPhone    *string `json:"contact_phone"`
	ContactEmail    *string `json:"contact_email"`
	PhysicalAddress *string `json:"physical_address"`
	PostalAddress   *string `json:"postal_address"`
	Region          *string `json:"region"`
	City            *string `json:"city"`
	District        *string `json:"district"`
	UpdatedBy       string  `json:"updated_by"`
}

// Validate validates the update retailer request
func (r *UpdateRetailerRequest) Validate() error {
	if r.UpdatedBy == "" {
		return fmt.Errorf("updated by is required")
	}
	return nil
}

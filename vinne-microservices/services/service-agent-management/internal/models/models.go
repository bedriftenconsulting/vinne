package models

import (
	"database/sql"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// Status types for agents and retailers
type EntityStatus string

const (
	StatusActive      EntityStatus = "ACTIVE"
	StatusSuspended   EntityStatus = "SUSPENDED"
	StatusUnderReview EntityStatus = "UNDER_REVIEW"
	StatusInactive    EntityStatus = "INACTIVE"
	StatusTerminated  EntityStatus = "TERMINATED"
)

// Onboarding methods
type OnboardingMethod string

const (
	OnboardingRandLotteryLtdDirect OnboardingMethod = "RAND_LOTTERY_LTD_DIRECT"
	OnboardingAgentManaged         OnboardingMethod = "AGENT_ONBOARDED"
	OnboardingReferral             OnboardingMethod = "REFERRAL"
)

// KYC status types
type KYCStatus string

const (
	KYCStatusPending     KYCStatus = "PENDING"
	KYCStatusSubmitted   KYCStatus = "SUBMITTED"
	KYCStatusUnderReview KYCStatus = "UNDER_REVIEW"
	KYCStatusApproved    KYCStatus = "APPROVED"
	KYCStatusRejected    KYCStatus = "REJECTED"
	KYCStatusExpired     KYCStatus = "EXPIRED"
)

// POS Device status types
type DeviceStatus string

const (
	DeviceStatusAvailable      DeviceStatus = "AVAILABLE"
	DeviceStatusAssigned       DeviceStatus = "ASSIGNED"
	DeviceStatusActive         DeviceStatus = "ACTIVE"
	DeviceStatusInactive       DeviceStatus = "INACTIVE"
	DeviceStatusFaulty         DeviceStatus = "FAULTY"
	DeviceStatusDecommissioned DeviceStatus = "DECOMMISSIONED"
)

// Commission tier model removed - agents now use direct commission percentage

// Agent represents an agent in the RANDCO system
type Agent struct {
	ID                 uuid.UUID `json:"id" db:"id"`
	AgentCode          string    `json:"agent_code" db:"agent_code"`
	BusinessName       string    `json:"business_name" db:"business_name"`
	RegistrationNumber string    `json:"registration_number" db:"registration_number"`
	TaxID              string    `json:"tax_id" db:"tax_id"`

	// Contact Information
	ContactEmail       string `json:"contact_email" db:"contact_email"`
	ContactPhone       string `json:"contact_phone" db:"contact_phone"`
	PrimaryContactName string `json:"primary_contact_name" db:"primary_contact_name"`

	// Location Information
	PhysicalAddress string         `json:"physical_address" db:"physical_address"`
	City            string         `json:"city" db:"city"`
	Region          string         `json:"region" db:"region"`
	GPSCoordinates  sql.NullString `json:"gps_coordinates" db:"gps_coordinates"` // Stored as nullable string for JSON

	// Banking Details
	BankName          string `json:"bank_name" db:"bank_name"`
	BankAccountNumber string `json:"bank_account_number" db:"bank_account_number"`
	BankAccountName   string `json:"bank_account_name" db:"bank_account_name"`

	// Status and Classification
	Status           EntityStatus     `json:"status" db:"status"`
	OnboardingMethod OnboardingMethod `json:"onboarding_method" db:"onboarding_method"`

	// Commission
	CommissionPercentage float64 `json:"commission_percentage" db:"commission_percentage"` // Stored as percentage (e.g., 30 for 30%)

	// Related data (loaded separately)
	KYC *AgentKYC `json:"kyc,omitempty"`

	// Metadata
	CreatedBy string    `json:"created_by" db:"created_by"`
	UpdatedBy string    `json:"updated_by" db:"updated_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Transient field - only populated on creation, never stored in DB
	InitialPassword string `json:"initial_password,omitempty" db:"-"`
}
type Retailer struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	RetailerCode string     `json:"retailer_code" db:"retailer_code"`
	Name         string     `json:"name" db:"business_name"`         // Map to business_name column in DB
	Email        string     `json:"email" db:"contact_email"`        // Map to contact_email column in DB
	PhoneNumber  string     `json:"phone_number" db:"contact_phone"` // Map to contact_phone column in DB
	Address      string     `json:"address" db:"physical_address"`   // Map to physical_address column in DB
	AgentID      *uuid.UUID `json:"agent_id" db:"parent_agent_id"`   // Map to parent_agent_id column in DB

	// Additional DB fields that we still need
	OwnerName      string `json:"owner_name" db:"owner_name"`
	City           string `json:"city" db:"city"`
	Region         string `json:"region" db:"region"`
	GPSCoordinates string `json:"gps_coordinates" db:"gps_coordinates"`

	// Business Details
	BusinessLicense string `json:"business_license" db:"business_license"`
	ShopType        string `json:"shop_type" db:"shop_type"`

	// Status and Classification
	Status           EntityStatus     `json:"status" db:"status"`
	OnboardingMethod OnboardingMethod `json:"onboarding_method" db:"onboarding_method"`

	// Related data (loaded separately)
	ParentAgent *Agent       `json:"parent_agent,omitempty"`
	KYC         *RetailerKYC `json:"kyc,omitempty"`
	POSDevices  []POSDevice  `json:"pos_devices,omitempty"`

	// Metadata
	CreatedBy string    `json:"created_by" db:"created_by"`
	UpdatedBy string    `json:"updated_by" db:"updated_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Transient field - only populated on creation, never stored in DB
	InitialPin string `json:"initial_pin,omitempty" db:"-"`
}

// AgentRetailer represents the explicit relationship between agents and retailers
type AgentRetailer struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	AgentID          uuid.UUID  `json:"agent_id" db:"agent_id"`
	RetailerID       uuid.UUID  `json:"retailer_id" db:"retailer_id"`
	RelationshipType string     `json:"relationship_type" db:"relationship_type"`
	AssignedDate     time.Time  `json:"assigned_date" db:"assigned_date"`
	UnassignedDate   *time.Time `json:"unassigned_date" db:"unassigned_date"`
	IsActive         bool       `json:"is_active" db:"is_active"`
	AssignedBy       string     `json:"assigned_by" db:"assigned_by"`
	Notes            string     `json:"notes" db:"notes"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`

	// Related data (loaded separately)
	Agent    *Agent    `json:"agent,omitempty"`
	Retailer *Retailer `json:"retailer,omitempty"`
}

// POSDevice represents a point-of-sale device
type POSDevice struct {
	ID           uuid.UUID `json:"id" db:"id"`
	DeviceCode   string    `json:"device_code" db:"device_code"`
	IMEI         string    `json:"imei" db:"imei"`
	SerialNumber string    `json:"serial_number" db:"serial_number"`
	Model        string    `json:"model" db:"model"`
	Manufacturer string    `json:"manufacturer" db:"manufacturer"`

	// Assignment Information
	AssignedRetailerID *uuid.UUID `json:"assigned_retailer_id" db:"assigned_retailer_id"`
	AssignmentDate     *time.Time `json:"assignment_date" db:"assignment_date"`
	LastSync           *time.Time `json:"last_sync" db:"last_sync"`
	LastTransaction    *time.Time `json:"last_transaction" db:"last_transaction"`

	// Device Status
	Status          DeviceStatus `json:"status" db:"status"`
	SoftwareVersion string       `json:"software_version" db:"software_version"`

	// Network Information
	NetworkOperator string `json:"network_operator" db:"network_operator"`
	SIMCardNumber   string `json:"sim_card_number" db:"sim_card_number"`

	// Related data (loaded separately)
	AssignedRetailer *Retailer `json:"assigned_retailer,omitempty"`

	// Metadata
	AssignedBy string    `json:"assigned_by" db:"assigned_by"`
	CreatedBy  string    `json:"created_by" db:"created_by"`
	UpdatedBy  string    `json:"updated_by" db:"updated_by"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// AgentKYC represents KYC information for agents
type AgentKYC struct {
	ID      uuid.UUID `json:"id" db:"id"`
	AgentID uuid.UUID `json:"agent_id" db:"agent_id"`

	// KYC Status
	KYCStatus KYCStatus `json:"kyc_status" db:"kyc_status"`

	// Document Information
	BusinessRegistrationCert string `json:"business_registration_cert" db:"business_registration_cert"`
	TaxClearanceCert         string `json:"tax_clearance_cert" db:"tax_clearance_cert"`
	DirectorIDDocument       string `json:"director_id_document" db:"director_id_document"`
	ProofOfAddress           string `json:"proof_of_address" db:"proof_of_address"`
	BankAccountVerification  string `json:"bank_account_verification" db:"bank_account_verification"`

	// Review Information
	ReviewedBy      string     `json:"reviewed_by" db:"reviewed_by"`
	ReviewedAt      *time.Time `json:"reviewed_at" db:"reviewed_at"`
	RejectionReason string     `json:"rejection_reason" db:"rejection_reason"`
	Notes           string     `json:"notes" db:"notes"`

	// Expiry Information
	ExpiresAt           *time.Time `json:"expires_at" db:"expires_at"`
	RenewalReminderSent bool       `json:"renewal_reminder_sent" db:"renewal_reminder_sent"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// IsExpired checks if the KYC has expired
func (k *AgentKYC) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return k.ExpiresAt.Before(time.Now())
}

// IsApproved checks if the KYC is approved
func (k *AgentKYC) IsApproved() bool {
	return k.KYCStatus == KYCStatusApproved && !k.IsExpired()
}

// HasAllDocuments checks if all required documents are provided
func (k *AgentKYC) HasAllDocuments() bool {
	return k.BusinessRegistrationCert != "" &&
		k.TaxClearanceCert != "" &&
		k.DirectorIDDocument != "" &&
		k.ProofOfAddress != "" &&
		k.BankAccountVerification != ""
}

// CanBeReviewed checks if KYC can be reviewed
func (k *AgentKYC) CanBeReviewed() bool {
	return k.KYCStatus == KYCStatusSubmitted && k.HasAllDocuments()
}

// Approve approves the KYC
func (k *AgentKYC) Approve(reviewedBy string, expiryMonths int, notes string) error {
	if !k.CanBeReviewed() {
		return fmt.Errorf("KYC cannot be approved in current status: %s", k.KYCStatus)
	}
	now := time.Now()
	expiresAt := now.AddDate(0, expiryMonths, 0)
	k.KYCStatus = KYCStatusApproved
	k.ReviewedBy = reviewedBy
	k.ReviewedAt = &now
	k.ExpiresAt = &expiresAt
	k.Notes = notes
	k.RejectionReason = ""
	k.UpdatedAt = now
	return nil
}

// Reject rejects the KYC
func (k *AgentKYC) Reject(reviewedBy string, reason string, notes string) error {
	if !k.CanBeReviewed() {
		return fmt.Errorf("KYC cannot be rejected in current status: %s", k.KYCStatus)
	}
	now := time.Now()
	k.KYCStatus = KYCStatusRejected
	k.ReviewedBy = reviewedBy
	k.ReviewedAt = &now
	k.RejectionReason = reason
	k.Notes = notes
	k.UpdatedAt = now
	return nil
}

// NeedsRenewalReminder checks if a renewal reminder should be sent
func (k *AgentKYC) NeedsRenewalReminder(daysBefore int) bool {
	if k.RenewalReminderSent || k.KYCStatus != KYCStatusApproved || k.ExpiresAt == nil {
		return false
	}
	reminderDate := k.ExpiresAt.AddDate(0, 0, -daysBefore)
	return time.Now().After(reminderDate)
}

// RetailerKYC represents KYC information for retailers
type RetailerKYC struct {
	ID         uuid.UUID `json:"id" db:"id"`
	RetailerID uuid.UUID `json:"retailer_id" db:"retailer_id"`

	// KYC Status
	KYCStatus KYCStatus `json:"kyc_status" db:"kyc_status"`

	// Document Information
	BusinessLicense string   `json:"business_license" db:"business_license"`
	OwnerIDDocument string   `json:"owner_id_document" db:"owner_id_document"`
	ProofOfAddress  string   `json:"proof_of_address" db:"proof_of_address"`
	ShopPhotos      []string `json:"shop_photos" db:"shop_photos"`

	// Review Information
	ReviewedBy      string     `json:"reviewed_by" db:"reviewed_by"`
	ReviewedAt      *time.Time `json:"reviewed_at" db:"reviewed_at"`
	RejectionReason string     `json:"rejection_reason" db:"rejection_reason"`
	Notes           string     `json:"notes" db:"notes"`

	// Expiry Information
	ExpiresAt           *time.Time `json:"expires_at" db:"expires_at"`
	RenewalReminderSent bool       `json:"renewal_reminder_sent" db:"renewal_reminder_sent"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// IsExpired checks if the KYC has expired
func (k *RetailerKYC) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return k.ExpiresAt.Before(time.Now())
}

// IsApproved checks if the KYC is approved
func (k *RetailerKYC) IsApproved() bool {
	return k.KYCStatus == KYCStatusApproved && !k.IsExpired()
}

// HasAllDocuments checks if all required documents are provided
func (k *RetailerKYC) HasAllDocuments() bool {
	return k.BusinessLicense != "" &&
		k.OwnerIDDocument != "" &&
		k.ProofOfAddress != "" &&
		len(k.ShopPhotos) > 0
}

// CanBeReviewed checks if KYC can be reviewed
func (k *RetailerKYC) CanBeReviewed() bool {
	return k.KYCStatus == KYCStatusSubmitted && k.HasAllDocuments()
}

// Approve approves the KYC
func (k *RetailerKYC) Approve(reviewedBy string, expiryMonths int, notes string) error {
	if !k.CanBeReviewed() {
		return fmt.Errorf("KYC cannot be approved in current status: %s", k.KYCStatus)
	}
	now := time.Now()
	expiresAt := now.AddDate(0, expiryMonths, 0)
	k.KYCStatus = KYCStatusApproved
	k.ReviewedBy = reviewedBy
	k.ReviewedAt = &now
	k.ExpiresAt = &expiresAt
	k.Notes = notes
	k.RejectionReason = ""
	k.UpdatedAt = now
	return nil
}

// Reject rejects the KYC
func (k *RetailerKYC) Reject(reviewedBy string, reason string, notes string) error {
	if !k.CanBeReviewed() {
		return fmt.Errorf("KYC cannot be rejected in current status: %s", k.KYCStatus)
	}
	now := time.Now()
	k.KYCStatus = KYCStatusRejected
	k.ReviewedBy = reviewedBy
	k.ReviewedAt = &now
	k.RejectionReason = reason
	k.Notes = notes
	k.UpdatedAt = now
	return nil
}

// AgentPerformance represents performance metrics for agents
type AgentPerformance struct {
	ID      uuid.UUID `json:"id" db:"id"`
	AgentID uuid.UUID `json:"agent_id" db:"agent_id"`

	// Performance Period
	PeriodYear  int `json:"period_year" db:"period_year"`
	PeriodMonth int `json:"period_month" db:"period_month"`

	// Performance Metrics
	TotalRetailersActive   int     `json:"total_retailers_active" db:"total_retailers_active"`
	TotalRetailersInactive int     `json:"total_retailers_inactive" db:"total_retailers_inactive"`
	TotalSalesAmount       float64 `json:"total_sales_amount" db:"total_sales_amount"`
	TotalCommissionEarned  float64 `json:"total_commission_earned" db:"total_commission_earned"`
	TotalTransactions      int     `json:"total_transactions" db:"total_transactions"`

	// Calculation timestamp
	CalculatedAt *time.Time `json:"calculated_at" db:"calculated_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}

// RetailerPerformance represents performance metrics for retailers
type RetailerPerformance struct {
	ID         uuid.UUID `json:"id" db:"id"`
	RetailerID uuid.UUID `json:"retailer_id" db:"retailer_id"`

	// Performance Period
	PeriodYear  int `json:"period_year" db:"period_year"`
	PeriodMonth int `json:"period_month" db:"period_month"`

	// Performance Metrics
	TotalSalesAmount      float64 `json:"total_sales_amount" db:"total_sales_amount"`
	TotalCommissionEarned float64 `json:"total_commission_earned" db:"total_commission_earned"`
	TotalTransactions     int     `json:"total_transactions" db:"total_transactions"`
	AvgTransactionValue   float64 `json:"avg_transaction_value" db:"avg_transaction_value"`

	// Calculation timestamp
	CalculatedAt *time.Time `json:"calculated_at" db:"calculated_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}

// AgentPerformanceSummary represents aggregated performance metrics for agents
type AgentPerformanceSummary struct {
	AgentID                 uuid.UUID `json:"agent_id"`
	AgentName               string    `json:"agent_name"`
	Year                    int       `json:"year"`
	TotalRetailersOnboarded int       `json:"total_retailers_onboarded"`
	TotalTransactions       int       `json:"total_transactions"`
	TotalTransactionValue   float64   `json:"total_transaction_value"`
	TotalCommissionEarned   float64   `json:"total_commission_earned"`
	AverageActiveRetailers  float64   `json:"avg_active_retailers"`
}

// RetailerPerformanceSummary represents aggregated performance metrics for retailers
type RetailerPerformanceSummary struct {
	RetailerID            uuid.UUID `json:"retailer_id"`
	RetailerName          string    `json:"retailer_name"`
	Year                  int       `json:"year"`
	TotalTransactions     int       `json:"total_transactions"`
	TotalTransactionValue float64   `json:"total_transaction_value"`
	TotalTicketsSold      int       `json:"total_tickets_sold"`
	TotalPayoutAmount     float64   `json:"total_payout_amount"`
	AverageDailySales     float64   `json:"avg_daily_sales"`
}

// Helper methods for validation and ID generation

// IsValidStatus checks if the status is valid for the entity type
func (s EntityStatus) IsValid() bool {
	switch s {
	case StatusActive, StatusSuspended, StatusUnderReview, StatusInactive, StatusTerminated:
		return true
	default:
		return false
	}
}

// String returns the string representation of EntityStatus
func (s EntityStatus) String() string {
	return string(s)
}

// IsValidOnboardingMethod checks if the onboarding method is valid
func (o OnboardingMethod) IsValid() bool {
	switch o {
	case OnboardingRandLotteryLtdDirect, OnboardingAgentManaged, OnboardingReferral:
		return true
	default:
		return false
	}
}

// IsValidKYCStatus checks if the KYC status is valid
func (k KYCStatus) IsValid() bool {
	switch k {
	case KYCStatusPending, KYCStatusSubmitted, KYCStatusUnderReview, KYCStatusApproved, KYCStatusRejected, KYCStatusExpired:
		return true
	default:
		return false
	}
}

// CanTransitionTo checks if KYC status can transition to another status
func (k KYCStatus) CanTransitionTo(newStatus KYCStatus) bool {
	if !newStatus.IsValid() {
		return false
	}

	switch k {
	case KYCStatusPending:
		return newStatus == KYCStatusSubmitted || newStatus == KYCStatusExpired
	case KYCStatusSubmitted:
		return newStatus == KYCStatusUnderReview || newStatus == KYCStatusRejected || newStatus == KYCStatusExpired
	case KYCStatusUnderReview:
		return newStatus == KYCStatusApproved || newStatus == KYCStatusRejected || newStatus == KYCStatusExpired
	case KYCStatusApproved:
		return newStatus == KYCStatusExpired || newStatus == KYCStatusPending // Can restart KYC
	case KYCStatusRejected:
		return newStatus == KYCStatusPending || newStatus == KYCStatusSubmitted // Can resubmit
	case KYCStatusExpired:
		return newStatus == KYCStatusPending || newStatus == KYCStatusSubmitted // Can restart
	default:
		return false
	}
}

// IsValidDeviceStatus checks if the device status is valid
func (d DeviceStatus) IsValid() bool {
	switch d {
	case DeviceStatusAvailable, DeviceStatusAssigned, DeviceStatusActive, DeviceStatusInactive, DeviceStatusFaulty, DeviceStatusDecommissioned:
		return true
	default:
		return false
	}
}

// Helper functions

// isValidEmail validates email format
func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// CanBeActivated checks if an agent can be activated
func (a *Agent) CanBeActivated() bool {
	return a.Status == StatusInactive || a.Status == StatusUnderReview
}

// CanBeSuspended checks if an agent can be suspended
func (a *Agent) CanBeSuspended() bool {
	return a.Status == StatusActive
}

// HasActiveKYC checks if the agent has active KYC
func (a *Agent) HasActiveKYC() bool {
	return a.KYC != nil && a.KYC.KYCStatus == KYCStatusApproved
}

// Validate performs comprehensive validation of agent data
func (a *Agent) Validate() error {
	if a.BusinessName == "" {
		return fmt.Errorf("business name is required")
	}
	if a.ContactPhone == "" {
		return fmt.Errorf("contact phone is required")
	}
	if a.ContactEmail != "" && !isValidEmail(a.ContactEmail) {
		return fmt.Errorf("invalid email format")
	}
	if a.CommissionPercentage < 0 || a.CommissionPercentage > 100 {
		return fmt.Errorf("commission percentage must be between 0 and 100")
	}
	if !a.Status.IsValid() {
		return fmt.Errorf("invalid agent status: %s", a.Status)
	}
	if !a.OnboardingMethod.IsValid() {
		return fmt.Errorf("invalid onboarding method: %s", a.OnboardingMethod)
	}
	if a.BankAccountNumber != "" && a.BankName == "" {
		return fmt.Errorf("bank name is required when bank account number is provided")
	}
	return nil
}

// SetStatus updates the agent's status with validation
func (a *Agent) SetStatus(newStatus EntityStatus) error {
	// Validate status transition
	if !a.CanTransitionTo(newStatus) {
		return fmt.Errorf("cannot transition from %s to %s", a.Status, newStatus)
	}
	a.Status = newStatus
	a.UpdatedAt = time.Now()
	return nil
}

// CanTransitionTo checks if a status transition is valid
func (a *Agent) CanTransitionTo(newStatus EntityStatus) bool {
	if !newStatus.IsValid() {
		return false
	}

	switch a.Status {
	case StatusUnderReview:
		// From under review, can go to active, inactive, or terminated
		return newStatus == StatusActive || newStatus == StatusInactive || newStatus == StatusTerminated
	case StatusActive:
		// From active, can go to suspended, inactive, or terminated
		return newStatus == StatusSuspended || newStatus == StatusInactive || newStatus == StatusTerminated
	case StatusSuspended:
		// From suspended, can go to active or terminated
		return newStatus == StatusActive || newStatus == StatusTerminated
	case StatusInactive:
		// From inactive, can go to active or terminated
		return newStatus == StatusActive || newStatus == StatusTerminated || newStatus == StatusUnderReview
	case StatusTerminated:
		// Terminal state - no transitions allowed
		return false
	default:
		return false
	}
}

// CalculateCommissionAmount calculates commission for a given transaction amount
func (a *Agent) CalculateCommissionAmount(transactionAmount float64) float64 {
	return transactionAmount * (a.CommissionPercentage / 100.0)
}

// NeedsKYCRenewal checks if KYC renewal is needed
func (a *Agent) NeedsKYCRenewal() bool {
	if a.KYC == nil {
		return true
	}
	if a.KYC.KYCStatus != KYCStatusApproved {
		return true
	}
	if a.KYC.ExpiresAt != nil && a.KYC.ExpiresAt.Before(time.Now()) {
		return true
	}
	return false
}

// IsOperational checks if the agent can perform operations
func (a *Agent) IsOperational() bool {
	return a.Status == StatusActive && a.HasActiveKYC()
}

// CanBeActivated checks if a retailer can be activated
func (r *Retailer) CanBeActivated() bool {
	return r.Status == StatusInactive || r.Status == StatusUnderReview
}

// CanBeSuspended checks if a retailer can be suspended
func (r *Retailer) CanBeSuspended() bool {
	return r.Status == StatusActive
}

// HasActiveKYC checks if the retailer has active KYC
func (r *Retailer) HasActiveKYC() bool {
	return r.KYC != nil && r.KYC.KYCStatus == KYCStatusApproved
}

// IsAgentManaged checks if the retailer is managed by an agent
func (r *Retailer) IsAgentManaged() bool {
	return r.AgentID != nil && r.OnboardingMethod == OnboardingAgentManaged
}

// Validate performs comprehensive validation of retailer data
func (r *Retailer) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("retailer name is required")
	}
	if r.PhoneNumber == "" {
		return fmt.Errorf("phone number is required")
	}
	if r.Email != "" && !isValidEmail(r.Email) {
		r.Email = "" // clear invalid optional email rather than blocking creation
	}
	if !r.Status.IsValid() {
		return fmt.Errorf("invalid retailer status: %s", r.Status)
	}
	if !r.OnboardingMethod.IsValid() {
		return fmt.Errorf("invalid onboarding method: %s", r.OnboardingMethod)
	}
	if r.OnboardingMethod == OnboardingAgentManaged && r.AgentID == nil {
		return fmt.Errorf("agent ID is required for agent-managed retailers")
	}
	return nil
}

// SetStatus updates the retailer's status with validation
func (r *Retailer) SetStatus(newStatus EntityStatus) error {
	// Validate status transition
	if !r.CanTransitionTo(newStatus) {
		return fmt.Errorf("cannot transition from %s to %s", r.Status, newStatus)
	}
	r.Status = newStatus
	r.UpdatedAt = time.Now()
	return nil
}

// CanTransitionTo checks if a status transition is valid for a retailer
func (r *Retailer) CanTransitionTo(newStatus EntityStatus) bool {
	if !newStatus.IsValid() {
		return false
	}

	// Similar rules as agent but may differ in the future
	switch r.Status {
	case StatusUnderReview:
		return newStatus == StatusActive || newStatus == StatusInactive || newStatus == StatusTerminated
	case StatusActive:
		return newStatus == StatusSuspended || newStatus == StatusInactive || newStatus == StatusTerminated
	case StatusSuspended:
		return newStatus == StatusActive || newStatus == StatusTerminated
	case StatusInactive:
		return newStatus == StatusActive || newStatus == StatusTerminated || newStatus == StatusUnderReview
	case StatusTerminated:
		return false // Terminal state
	default:
		return false
	}
}

// NeedsKYCRenewal checks if KYC renewal is needed
func (r *Retailer) NeedsKYCRenewal() bool {
	if r.KYC == nil {
		return true
	}
	if r.KYC.KYCStatus != KYCStatusApproved {
		return true
	}
	if r.KYC.ExpiresAt != nil && r.KYC.ExpiresAt.Before(time.Now()) {
		return true
	}
	return false
}

// IsOperational checks if the retailer can perform operations
func (r *Retailer) IsOperational() bool {
	return r.Status == StatusActive && r.HasActiveKYC()
}

// CanAcceptPOSDevice checks if the retailer can be assigned a POS device
func (r *Retailer) CanAcceptPOSDevice() bool {
	return r.IsOperational() && r.Status == StatusActive
}

// CanBeAssigned checks if a POS device can be assigned
func (p *POSDevice) CanBeAssigned() bool {
	return p.Status == DeviceStatusAvailable || p.Status == DeviceStatusInactive
}

// IsAssigned checks if a POS device is currently assigned
func (p *POSDevice) IsAssigned() bool {
	return p.AssignedRetailerID != nil && (p.Status == DeviceStatusAssigned || p.Status == DeviceStatusActive)
}

// Validate performs comprehensive validation of POS device data
func (p *POSDevice) Validate() error {
	if p.DeviceCode == "" {
		return fmt.Errorf("device code is required")
	}
	if p.IMEI == "" {
		return fmt.Errorf("IMEI is required")
	}
	if p.SerialNumber == "" {
		return fmt.Errorf("serial number is required")
	}
	if p.Model == "" {
		return fmt.Errorf("model is required")
	}
	if p.Manufacturer == "" {
		return fmt.Errorf("manufacturer is required")
	}
	if !p.Status.IsValid() {
		return fmt.Errorf("invalid device status: %s", p.Status)
	}
	return nil
}

// AssignToRetailer assigns the device to a retailer
func (p *POSDevice) AssignToRetailer(retailerID uuid.UUID, assignedBy string) error {
	if !p.CanBeAssigned() {
		return fmt.Errorf("device cannot be assigned in current status: %s", p.Status)
	}
	now := time.Now()
	p.AssignedRetailerID = &retailerID
	p.AssignmentDate = &now
	p.Status = DeviceStatusAssigned
	p.AssignedBy = assignedBy
	p.UpdatedBy = assignedBy
	p.UpdatedAt = now
	return nil
}

// Unassign removes the device assignment
func (p *POSDevice) Unassign(unassignedBy string) error {
	if !p.IsAssigned() {
		return fmt.Errorf("device is not currently assigned")
	}
	p.AssignedRetailerID = nil
	p.AssignmentDate = nil
	p.Status = DeviceStatusAvailable
	p.UpdatedBy = unassignedBy
	p.UpdatedAt = time.Now()
	return nil
}

// Activate marks the device as active
func (p *POSDevice) Activate() error {
	if p.Status != DeviceStatusAssigned {
		return fmt.Errorf("can only activate assigned devices")
	}
	p.Status = DeviceStatusActive
	now := time.Now()
	p.LastSync = &now
	p.UpdatedAt = now
	return nil
}

// RecordTransaction updates the last transaction time
func (p *POSDevice) RecordTransaction() {
	now := time.Now()
	p.LastTransaction = &now
	p.LastSync = &now
}

// NeedsSync checks if the device needs synchronization
func (p *POSDevice) NeedsSync(syncIntervalMinutes int) bool {
	if p.LastSync == nil {
		return true
	}
	syncThreshold := time.Now().Add(-time.Duration(syncIntervalMinutes) * time.Minute)
	return p.LastSync.Before(syncThreshold)
}

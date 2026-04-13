package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
)

// RetailerRepository defines core retailer operations (max 10 methods)
type RetailerRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, retailer *models.Retailer) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Retailer, error)
	GetByCode(ctx context.Context, retailerCode string) (*models.Retailer, error)
	GetByPhone(ctx context.Context, phone string) (*models.Retailer, error)
	Update(ctx context.Context, retailer *models.Retailer) error
	Delete(ctx context.Context, id uuid.UUID) error

	// List operations with filtering
	List(ctx context.Context, filters RetailerFilters) ([]models.Retailer, error)
	Count(ctx context.Context, filters RetailerFilters) (int, error)

	// Code generation
	GetNextRetailerCode(ctx context.Context, agentCode string) (string, error)
}

// RetailerRelationshipRepository handles retailer-agent relationship operations
type RetailerRelationshipRepository interface {
	GetByAgentID(ctx context.Context, agentID uuid.UUID) ([]models.Retailer, error)
	GetIndependentRetailers(ctx context.Context) ([]models.Retailer, error)
	AssignToAgent(ctx context.Context, retailerID, agentID uuid.UUID, assignedBy string) error
	UnassignFromAgent(ctx context.Context, retailerID uuid.UUID, unassignedBy string) error
}

// RetailerStatusRepository handles retailer status operations
type RetailerStatusRepository interface {
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.EntityStatus, updatedBy string) error
	GetByStatus(ctx context.Context, status models.EntityStatus) ([]models.Retailer, error)
}

// RetailerFilters defines filtering options for retailer queries
type RetailerFilters struct {
	Status           *models.EntityStatus
	AgentID          *uuid.UUID // Maps to parent_agent_id in DB
	OnboardingMethod *models.OnboardingMethod
	Name             *string // Maps to business_name in DB
	Email            *string // Maps to contact_email in DB
	OwnerName        *string
	ContactPhone     *string
	Region           *string
	City             *string
	CreatedAfter     *time.Time
	CreatedBefore    *time.Time
	IndependentOnly  bool
	AgentManagedOnly bool
	Limit            int
	Offset           int
	OrderBy          string
	OrderDirection   string
}

type retailerRepository struct {
	db *sqlx.DB
}

// NewRetailerRepository creates a new retailer repository
func NewRetailerRepository(db *sqlx.DB) RetailerRepository {
	return &retailerRepository{db: db}
}

// NewRetailerRelationshipRepository creates a new retailer relationship repository
func NewRetailerRelationshipRepository(db *sqlx.DB) RetailerRelationshipRepository {
	return &retailerRepository{db: db}
}

// NewRetailerStatusRepository creates a new retailer status repository
func NewRetailerStatusRepository(db *sqlx.DB) RetailerStatusRepository {
	return &retailerRepository{db: db}
}

func (r *retailerRepository) Create(ctx context.Context, retailer *models.Retailer) error {
	// Generate UUID if not set
	if retailer.ID == uuid.Nil {
		retailer.ID = uuid.New()
	}

	// Map model fields to database columns
	businessName := retailer.Name        // Name maps to business_name column
	ownerName := retailer.Name           // Use same value for owner_name
	contactEmail := retailer.Email       // Email maps to contact_email column
	contactPhone := retailer.PhoneNumber // PhoneNumber maps to contact_phone column
	physicalAddress := retailer.Address  // Address maps to physical_address column

	// Handle optional AgentID field
	var parentAgentID interface{}
	if retailer.AgentID != nil {
		parentAgentID = *retailer.AgentID // AgentID maps to parent_agent_id column
	} else {
		parentAgentID = nil
	}

	// Set timestamps
	now := time.Now()
	retailer.CreatedAt = now
	retailer.UpdatedAt = now

	query := `
		INSERT INTO retailers (
			id, retailer_code, business_name, owner_name, contact_email,
			contact_phone, physical_address, city, region, status,
			onboarding_method, parent_agent_id, created_by, updated_by,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	_, err := r.db.ExecContext(ctx, query,
		retailer.ID, retailer.RetailerCode, businessName, ownerName, contactEmail,
		contactPhone, physicalAddress, retailer.City, retailer.Region, retailer.Status,
		retailer.OnboardingMethod, parentAgentID, retailer.CreatedBy, retailer.UpdatedBy,
		retailer.CreatedAt, retailer.UpdatedAt,
	)
	return err
}

func (r *retailerRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Retailer, error) {
	var retailer models.Retailer
	var businessName, ownerName, contactEmail, contactPhone, physicalAddress string
	var city, region sql.NullString
	var parentAgentID sql.NullString

	query := `
		SELECT id, retailer_code, business_name, owner_name, contact_email,
		       contact_phone, physical_address, city, region, status,
		       onboarding_method, parent_agent_id, created_by, updated_by,
		       created_at, updated_at
		FROM retailers
		WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&retailer.ID, &retailer.RetailerCode, &businessName, &ownerName, &contactEmail,
		&contactPhone, &physicalAddress, &city, &region, &retailer.Status,
		&retailer.OnboardingMethod, &parentAgentID, &retailer.CreatedBy, &retailer.UpdatedBy,
		&retailer.CreatedAt, &retailer.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Map database columns back to model fields
	retailer.Name = businessName        // business_name column maps to Name
	retailer.OwnerName = ownerName      // Keep owner_name for DB
	retailer.Email = contactEmail       // contact_email column maps to Email
	retailer.PhoneNumber = contactPhone // contact_phone column maps to PhoneNumber
	retailer.Address = physicalAddress  // physical_address column maps to Address

	if city.Valid {
		retailer.City = city.String
	}
	if region.Valid {
		retailer.Region = region.String
	}
	if parentAgentID.Valid {
		agentID, _ := uuid.Parse(parentAgentID.String)
		retailer.AgentID = &agentID // parent_agent_id column maps to AgentID
	}

	return &retailer, nil
}

func (r *retailerRepository) GetByPhone(ctx context.Context, phone string) (*models.Retailer, error) {
	var retailer models.Retailer
	query := `
		SELECT id, retailer_code, business_name, owner_name, contact_email,
		       contact_phone, physical_address, city, region, status,
		       onboarding_method, parent_agent_id, created_by, updated_by,
		       created_at, updated_at
		FROM retailers
		WHERE contact_phone = $1`

	err := r.db.QueryRowContext(ctx, query, phone).Scan(
		&retailer.ID, &retailer.RetailerCode, &retailer.Name, &retailer.OwnerName,
		&retailer.Email, &retailer.PhoneNumber, &retailer.Address,
		&retailer.City, &retailer.Region, &retailer.Status,
		&retailer.OnboardingMethod, &retailer.AgentID,
		&retailer.CreatedBy, &retailer.UpdatedBy,
		&retailer.CreatedAt, &retailer.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &retailer, nil
}

func (r *retailerRepository) GetByCode(ctx context.Context, retailerCode string) (*models.Retailer, error) {
	var retailer models.Retailer
	var businessName, ownerName, contactEmail, contactPhone, physicalAddress string
	var city, region sql.NullString
	var parentAgentID sql.NullString

	query := `
		SELECT id, retailer_code, business_name, owner_name, contact_email,
		       contact_phone, physical_address, city, region, status,
		       onboarding_method, parent_agent_id, created_by, updated_by,
		       created_at, updated_at
		FROM retailers
		WHERE retailer_code = $1`

	err := r.db.QueryRowContext(ctx, query, retailerCode).Scan(
		&retailer.ID, &retailer.RetailerCode, &businessName, &ownerName, &contactEmail,
		&contactPhone, &physicalAddress, &city, &region, &retailer.Status,
		&retailer.OnboardingMethod, &parentAgentID, &retailer.CreatedBy, &retailer.UpdatedBy,
		&retailer.CreatedAt, &retailer.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Map database columns back to model fields
	retailer.Name = businessName
	retailer.OwnerName = ownerName
	retailer.Email = contactEmail
	retailer.PhoneNumber = contactPhone
	retailer.Address = physicalAddress

	if city.Valid {
		retailer.City = city.String
	}
	if region.Valid {
		retailer.Region = region.String
	}
	if parentAgentID.Valid {
		agentID, _ := uuid.Parse(parentAgentID.String)
		retailer.AgentID = &agentID
	}

	return &retailer, nil
}

func (r *retailerRepository) Update(ctx context.Context, retailer *models.Retailer) error {
	// Map model fields to database columns
	businessName := retailer.Name
	ownerName := retailer.OwnerName
	if ownerName == "" {
		ownerName = retailer.Name // Default to Name if OwnerName is empty
	}
	contactEmail := retailer.Email
	contactPhone := retailer.PhoneNumber
	physicalAddress := retailer.Address

	// Handle optional AgentID field
	var parentAgentID interface{}
	if retailer.AgentID != nil {
		parentAgentID = *retailer.AgentID
	} else {
		parentAgentID = nil
	}

	retailer.UpdatedAt = time.Now()

	query := `
		UPDATE retailers SET
			business_name = $2, owner_name = $3, contact_email = $4,
			contact_phone = $5, physical_address = $6, city = $7, region = $8,
			status = $9, onboarding_method = $10, parent_agent_id = $11,
			updated_by = $12, updated_at = $13
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		retailer.ID, businessName, ownerName, contactEmail,
		contactPhone, physicalAddress, retailer.City, retailer.Region,
		retailer.Status, retailer.OnboardingMethod, parentAgentID,
		retailer.UpdatedBy, retailer.UpdatedAt,
	)
	return err
}

func (r *retailerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM retailers WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *retailerRepository) List(ctx context.Context, filters RetailerFilters) ([]models.Retailer, error) {
	query := `
		SELECT id, retailer_code, business_name, owner_name, contact_email,
		       contact_phone, physical_address, city, region, status,
		       onboarding_method, parent_agent_id, created_by, updated_by,
		       created_at, updated_at
		FROM retailers
		WHERE 1=1`

	args := []interface{}{}
	argCounter := 1

	// Apply filters
	if filters.Status != nil && *filters.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argCounter)
		args = append(args, *filters.Status)
		argCounter++
	}

	if filters.AgentID != nil && *filters.AgentID != uuid.Nil {
		query += fmt.Sprintf(" AND parent_agent_id = $%d", argCounter)
		args = append(args, *filters.AgentID)
		argCounter++
	}

	if filters.Name != nil && *filters.Name != "" {
		query += fmt.Sprintf(" AND business_name ILIKE $%d", argCounter)
		args = append(args, "%"+*filters.Name+"%")
		argCounter++
	}

	// Add pagination
	query += " ORDER BY created_at DESC"
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCounter)
		args = append(args, filters.Limit)
		argCounter++
	}
	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCounter)
		args = append(args, filters.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var retailers []models.Retailer
	for rows.Next() {
		var retailer models.Retailer
		var businessName, ownerName, contactEmail, contactPhone, physicalAddress string
		var city, region sql.NullString
		var parentAgentID sql.NullString

		err := rows.Scan(
			&retailer.ID, &retailer.RetailerCode, &businessName, &ownerName, &contactEmail,
			&contactPhone, &physicalAddress, &city, &region, &retailer.Status,
			&retailer.OnboardingMethod, &parentAgentID, &retailer.CreatedBy, &retailer.UpdatedBy,
			&retailer.CreatedAt, &retailer.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Map database columns back to model fields
		retailer.Name = businessName
		retailer.OwnerName = ownerName
		retailer.Email = contactEmail
		retailer.PhoneNumber = contactPhone
		retailer.Address = physicalAddress

		if city.Valid {
			retailer.City = city.String
		}
		if region.Valid {
			retailer.Region = region.String
		}
		if parentAgentID.Valid {
			agentID, _ := uuid.Parse(parentAgentID.String)
			retailer.AgentID = &agentID
		}

		retailers = append(retailers, retailer)
	}

	return retailers, nil
}

func (r *retailerRepository) Count(ctx context.Context, filters RetailerFilters) (int, error) {
	query := `SELECT COUNT(*) FROM retailers WHERE 1=1`

	args := []interface{}{}
	argCounter := 1

	// Apply filters
	if filters.Status != nil && *filters.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argCounter)
		args = append(args, *filters.Status)
		argCounter++
	}

	if filters.AgentID != nil && *filters.AgentID != uuid.Nil {
		query += fmt.Sprintf(" AND parent_agent_id = $%d", argCounter)
		args = append(args, *filters.AgentID)
		argCounter++
	}

	if filters.Name != nil && *filters.Name != "" {
		query += fmt.Sprintf(" AND business_name ILIKE $%d", argCounter)
		args = append(args, "%"+*filters.Name+"%")
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *retailerRepository) GetIndependentRetailers(ctx context.Context) ([]models.Retailer, error) {
	query := `
		SELECT id, retailer_code, business_name, owner_name, contact_email,
		       contact_phone, physical_address, city, region, status,
		       onboarding_method, parent_agent_id, created_by, updated_by,
		       created_at, updated_at
		FROM retailers
		WHERE parent_agent_id IS NULL
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var retailers []models.Retailer
	for rows.Next() {
		var retailer models.Retailer
		var businessName, ownerName, contactEmail, contactPhone, physicalAddress string
		var city, region sql.NullString
		var parentAgentID sql.NullString

		err := rows.Scan(
			&retailer.ID, &retailer.RetailerCode, &businessName, &ownerName,
			&contactEmail, &contactPhone, &physicalAddress, &city, &region,
			&retailer.Status, &retailer.OnboardingMethod, &parentAgentID,
			&retailer.CreatedBy, &retailer.UpdatedBy,
			&retailer.CreatedAt, &retailer.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Map database columns back to model fields
		retailer.Name = businessName
		retailer.OwnerName = ownerName
		retailer.Email = contactEmail
		retailer.PhoneNumber = contactPhone
		retailer.Address = physicalAddress
		retailer.City = city.String
		retailer.Region = region.String

		// Handle parent_agent_id NULL value
		if parentAgentID.Valid {
			agentUUID, err := uuid.Parse(parentAgentID.String)
			if err == nil {
				retailer.AgentID = &agentUUID
			}
		}

		retailers = append(retailers, retailer)
	}

	return retailers, nil
}

func (r *retailerRepository) GetByAgentID(ctx context.Context, agentID uuid.UUID) ([]models.Retailer, error) {
	query := `
		SELECT id, retailer_code, business_name, owner_name, contact_email,
		       contact_phone, physical_address, city, region, status,
		       onboarding_method, parent_agent_id, created_by, updated_by,
		       created_at, updated_at
		FROM retailers
		WHERE parent_agent_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, agentID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var retailers []models.Retailer
	for rows.Next() {
		var retailer models.Retailer
		var businessName, ownerName, contactEmail, contactPhone, physicalAddress string
		var city, region sql.NullString
		var parentAgentID sql.NullString

		err := rows.Scan(
			&retailer.ID, &retailer.RetailerCode, &businessName, &ownerName, &contactEmail,
			&contactPhone, &physicalAddress, &city, &region, &retailer.Status,
			&retailer.OnboardingMethod, &parentAgentID, &retailer.CreatedBy, &retailer.UpdatedBy,
			&retailer.CreatedAt, &retailer.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Map database columns back to model fields
		retailer.Name = businessName
		retailer.OwnerName = ownerName
		retailer.Email = contactEmail
		retailer.PhoneNumber = contactPhone
		retailer.Address = physicalAddress

		if city.Valid {
			retailer.City = city.String
		}
		if region.Valid {
			retailer.Region = region.String
		}
		if parentAgentID.Valid {
			agentID, _ := uuid.Parse(parentAgentID.String)
			retailer.AgentID = &agentID
		}

		retailers = append(retailers, retailer)
	}

	return retailers, nil
}

func (r *retailerRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.EntityStatus, updatedBy string) error {
	query := `UPDATE retailers SET status = $2, updated_by = $3, updated_at = $4 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, status, updatedBy, time.Now())
	return err
}

func (r *retailerRepository) AssignToAgent(ctx context.Context, retailerID, agentID uuid.UUID, assignedBy string) error {
	query := `
		UPDATE retailers SET
			parent_agent_id = $2,
			updated_at = $3,
			updated_by = $4
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, retailerID, agentID, time.Now(), assignedBy)
	return err
}

func (r *retailerRepository) UnassignFromAgent(ctx context.Context, retailerID uuid.UUID, unassignedBy string) error {
	query := `
		UPDATE retailers SET
			parent_agent_id = NULL,
			updated_at = $2,
			updated_by = $3
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, retailerID, time.Now(), unassignedBy)
	return err
}

func (r *retailerRepository) GetByStatus(ctx context.Context, status models.EntityStatus) ([]models.Retailer, error) {
	query := `
		SELECT id, retailer_code, business_name, owner_name, contact_email,
		       contact_phone, physical_address, city, region, status,
		       onboarding_method, parent_agent_id, created_by, updated_by,
		       created_at, updated_at
		FROM retailers
		WHERE status = $1
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var retailers []models.Retailer
	for rows.Next() {
		var retailer models.Retailer
		var businessName, ownerName, contactEmail, contactPhone, physicalAddress string
		var city, region sql.NullString
		var parentAgentID sql.NullString

		err := rows.Scan(
			&retailer.ID, &retailer.RetailerCode, &businessName, &ownerName, &contactEmail,
			&contactPhone, &physicalAddress, &city, &region, &retailer.Status,
			&retailer.OnboardingMethod, &parentAgentID, &retailer.CreatedBy, &retailer.UpdatedBy,
			&retailer.CreatedAt, &retailer.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Map database columns back to model fields
		retailer.Name = businessName
		retailer.OwnerName = ownerName
		retailer.Email = contactEmail
		retailer.PhoneNumber = contactPhone
		retailer.Address = physicalAddress

		if city.Valid {
			retailer.City = city.String
		}
		if region.Valid {
			retailer.Region = region.String
		}
		if parentAgentID.Valid {
			agentID, _ := uuid.Parse(parentAgentID.String)
			retailer.AgentID = &agentID
		}

		retailers = append(retailers, retailer)
	}

	return retailers, nil
}

func (r *retailerRepository) GetNextRetailerCode(ctx context.Context, agentCode string) (string, error) {
	// Per PRD: Retailer codes are 8-digit numbers
	// Agent-managed: [4-digit agent code][4-digit sequence]
	// Independent: 0000[4-digit sequence]

	prefix := "0000" // Default for independent retailers
	if agentCode != "" {
		// For agent-managed retailers, use agent code as prefix
		// Pad agent code to 4 digits if needed
		if len(agentCode) < 4 {
			prefix = fmt.Sprintf("%04s", agentCode)
		} else {
			prefix = agentCode[:4] // Take first 4 digits if longer
		}
	}

	// Find the highest sequence number for this prefix
	query := `
		SELECT retailer_code 
		FROM retailers 
		WHERE retailer_code LIKE $1
		ORDER BY retailer_code DESC 
		LIMIT 1`

	pattern := prefix + "%"
	var lastCode sql.NullString
	err := r.db.QueryRowContext(ctx, query, pattern).Scan(&lastCode)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}

	nextSequence := 1
	if lastCode.Valid && len(lastCode.String) == 8 {
		// Extract the last 4 digits as the sequence number
		if seq, err := fmt.Sscanf(lastCode.String[4:], "%d", &nextSequence); seq == 1 && err == nil {
			nextSequence++
		}
	}

	// Format: [4-digit prefix][4-digit sequence]
	return fmt.Sprintf("%s%04d", prefix, nextSequence), nil
}

func (r *retailerRepository) GetByEmail(ctx context.Context, email string) (*models.Retailer, error) {
	var retailer models.Retailer
	var businessName, ownerName, contactEmail, contactPhone, physicalAddress string
	var city, region sql.NullString
	var parentAgentID sql.NullString

	query := `
		SELECT id, retailer_code, business_name, owner_name, contact_email,
		       contact_phone, physical_address, city, region, status,
		       onboarding_method, parent_agent_id, created_by, updated_by,
		       created_at, updated_at
		FROM retailers
		WHERE contact_email = $1`

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&retailer.ID, &retailer.RetailerCode, &businessName, &ownerName, &contactEmail,
		&contactPhone, &physicalAddress, &city, &region, &retailer.Status,
		&retailer.OnboardingMethod, &parentAgentID, &retailer.CreatedBy, &retailer.UpdatedBy,
		&retailer.CreatedAt, &retailer.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No retailer found
		}
		return nil, err
	}

	// Map database columns back to model fields
	retailer.Name = businessName
	retailer.OwnerName = ownerName
	retailer.Email = contactEmail
	retailer.PhoneNumber = contactPhone
	retailer.Address = physicalAddress

	if city.Valid {
		retailer.City = city.String
	}
	if region.Valid {
		retailer.Region = region.String
	}
	if parentAgentID.Valid {
		agentID, _ := uuid.Parse(parentAgentID.String)
		retailer.AgentID = &agentID
	}

	return &retailer, nil
}

func (r *retailerRepository) GetByPhoneNumber(ctx context.Context, phoneNumber string) (*models.Retailer, error) {
	var retailer models.Retailer
	var businessName, ownerName, contactEmail, contactPhone, physicalAddress string
	var city, region sql.NullString
	var parentAgentID sql.NullString

	query := `
		SELECT id, retailer_code, business_name, owner_name, contact_email,
		       contact_phone, physical_address, city, region, status,
		       onboarding_method, parent_agent_id, created_by, updated_by,
		       created_at, updated_at
		FROM retailers
		WHERE contact_phone = $1`

	err := r.db.QueryRowContext(ctx, query, phoneNumber).Scan(
		&retailer.ID, &retailer.RetailerCode, &businessName, &ownerName, &contactEmail,
		&contactPhone, &physicalAddress, &city, &region, &retailer.Status,
		&retailer.OnboardingMethod, &parentAgentID, &retailer.CreatedBy, &retailer.UpdatedBy,
		&retailer.CreatedAt, &retailer.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No retailer found
		}
		return nil, err
	}

	// Map database columns back to model fields
	retailer.Name = businessName
	retailer.OwnerName = ownerName
	retailer.Email = contactEmail
	retailer.PhoneNumber = contactPhone
	retailer.Address = physicalAddress

	if city.Valid {
		retailer.City = city.String
	}
	if region.Valid {
		retailer.Region = region.String
	}
	if parentAgentID.Valid {
		agentID, _ := uuid.Parse(parentAgentID.String)
		retailer.AgentID = &agentID
	}

	return &retailer, nil
}

func (r *retailerRepository) GetByIDWithRelations(ctx context.Context, id uuid.UUID) (*models.Retailer, error) {
	// For now, just get the basic retailer
	// This could be enhanced to include related data like agent, KYC, etc.
	return r.GetByID(ctx, id)
}

func (r *retailerRepository) GetByAgentIDAndStatus(ctx context.Context, agentID uuid.UUID, status models.EntityStatus) ([]models.Retailer, error) {
	query := `
		SELECT id, retailer_code, business_name, owner_name, contact_email,
		       contact_phone, physical_address, city, region, status,
		       onboarding_method, parent_agent_id, created_by, updated_by,
		       created_at, updated_at
		FROM retailers
		WHERE parent_agent_id = $1 AND status = $2
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, agentID, status)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var retailers []models.Retailer
	for rows.Next() {
		var retailer models.Retailer
		var businessName, ownerName, contactEmail, contactPhone, physicalAddress string
		var city, region sql.NullString
		var parentAgentID sql.NullString

		err := rows.Scan(
			&retailer.ID, &retailer.RetailerCode, &businessName, &ownerName, &contactEmail,
			&contactPhone, &physicalAddress, &city, &region, &retailer.Status,
			&retailer.OnboardingMethod, &parentAgentID, &retailer.CreatedBy, &retailer.UpdatedBy,
			&retailer.CreatedAt, &retailer.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Map database columns back to model fields
		retailer.Name = businessName
		retailer.OwnerName = ownerName
		retailer.Email = contactEmail
		retailer.PhoneNumber = contactPhone
		retailer.Address = physicalAddress

		if city.Valid {
			retailer.City = city.String
		}
		if region.Valid {
			retailer.Region = region.String
		}
		if parentAgentID.Valid {
			agentID, _ := uuid.Parse(parentAgentID.String)
			retailer.AgentID = &agentID
		}

		retailers = append(retailers, retailer)
	}

	return retailers, nil
}

func (r *retailerRepository) GetActiveRetailerCount(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM retailers WHERE status = $1`
	err := r.db.QueryRowContext(ctx, query, models.StatusActive).Scan(&count)
	return count, err
}

func (r *retailerRepository) GetRetailerCountByAgent(ctx context.Context, agentID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM retailers WHERE parent_agent_id = $1`
	err := r.db.QueryRowContext(ctx, query, agentID).Scan(&count)
	return count, err
}

func (r *retailerRepository) GetRecentRetailers(ctx context.Context, limit int) ([]models.Retailer, error) {
	query := `
		SELECT id, retailer_code, business_name, owner_name, contact_email,
		       contact_phone, physical_address, city, region, status,
		       onboarding_method, parent_agent_id, created_by, updated_by,
		       created_at, updated_at
		FROM retailers
		ORDER BY created_at DESC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var retailers []models.Retailer
	for rows.Next() {
		var retailer models.Retailer
		var businessName, ownerName, contactEmail, contactPhone, physicalAddress string
		var city, region sql.NullString
		var parentAgentID sql.NullString

		err := rows.Scan(
			&retailer.ID, &retailer.RetailerCode, &businessName, &ownerName, &contactEmail,
			&contactPhone, &physicalAddress, &city, &region, &retailer.Status,
			&retailer.OnboardingMethod, &parentAgentID, &retailer.CreatedBy, &retailer.UpdatedBy,
			&retailer.CreatedAt, &retailer.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Map database columns back to model fields
		retailer.Name = businessName
		retailer.OwnerName = ownerName
		retailer.Email = contactEmail
		retailer.PhoneNumber = contactPhone
		retailer.Address = physicalAddress

		if city.Valid {
			retailer.City = city.String
		}
		if region.Valid {
			retailer.Region = region.String
		}
		if parentAgentID.Valid {
			agentID, _ := uuid.Parse(parentAgentID.String)
			retailer.AgentID = &agentID
		}

		retailers = append(retailers, retailer)
	}

	return retailers, nil
}

func (r *retailerRepository) BulkCreate(ctx context.Context, retailers []models.Retailer) error {
	// Simple implementation - can be optimized with batch insert
	for i := range retailers {
		if err := r.Create(ctx, &retailers[i]); err != nil {
			return fmt.Errorf("failed to create retailer %d: %w", i, err)
		}
	}
	return nil
}

func (r *retailerRepository) UpdateOnboardingMethod(ctx context.Context, id uuid.UUID, method models.OnboardingMethod, updatedBy string) error {
	query := `UPDATE retailers SET onboarding_method = $2, updated_by = $3, updated_at = $4 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, method, updatedBy, time.Now())
	return err
}

func (r *retailerRepository) SearchRetailers(ctx context.Context, searchTerm string, limit int) ([]models.Retailer, error) {
	query := `
		SELECT id, retailer_code, business_name, owner_name, contact_email,
		       contact_phone, physical_address, city, region, status,
		       onboarding_method, parent_agent_id, created_by, updated_by,
		       created_at, updated_at
		FROM retailers
		WHERE business_name ILIKE $1
		   OR retailer_code ILIKE $1
		   OR contact_email ILIKE $1
		   OR contact_phone ILIKE $1
		ORDER BY created_at DESC
		LIMIT $2`

	searchPattern := "%" + searchTerm + "%"
	rows, err := r.db.QueryContext(ctx, query, searchPattern, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var retailers []models.Retailer
	for rows.Next() {
		var retailer models.Retailer
		var businessName, ownerName, contactEmail, contactPhone, physicalAddress string
		var city, region sql.NullString
		var parentAgentID sql.NullString

		err := rows.Scan(
			&retailer.ID, &retailer.RetailerCode, &businessName, &ownerName, &contactEmail,
			&contactPhone, &physicalAddress, &city, &region, &retailer.Status,
			&retailer.OnboardingMethod, &parentAgentID, &retailer.CreatedBy, &retailer.UpdatedBy,
			&retailer.CreatedAt, &retailer.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Map database columns back to model fields
		retailer.Name = businessName
		retailer.OwnerName = ownerName
		retailer.Email = contactEmail
		retailer.PhoneNumber = contactPhone
		retailer.Address = physicalAddress

		if city.Valid {
			retailer.City = city.String
		}
		if region.Valid {
			retailer.Region = region.String
		}
		if parentAgentID.Valid {
			agentID, _ := uuid.Parse(parentAgentID.String)
			retailer.AgentID = &agentID
		}

		retailers = append(retailers, retailer)
	}

	return retailers, nil
}

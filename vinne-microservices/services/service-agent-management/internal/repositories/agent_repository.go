package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
)

// AgentRepository defines the interface for agent data operations (max 10 methods)
type AgentRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, agent *models.Agent) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error)
	GetByCode(ctx context.Context, agentCode string) (*models.Agent, error)
	GetByEmail(ctx context.Context, email string) (*models.Agent, error)
	Update(ctx context.Context, agent *models.Agent) error
	Delete(ctx context.Context, id uuid.UUID) error

	// List operations with filtering
	List(ctx context.Context, filters AgentFilters) ([]models.Agent, error)
	Count(ctx context.Context, filters AgentFilters) (int, error)

	// Code generation
	GetNextAgentCode(ctx context.Context) (string, error)
}

// AgentStatusRepository handles agent status operations (separated to stay within limit)
type AgentStatusRepository interface {
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.EntityStatus, updatedBy string) error
	GetByStatus(ctx context.Context, status models.EntityStatus) ([]models.Agent, error)
}

// AgentFilters defines filtering options for agent queries
type AgentFilters struct {
	Status         *models.EntityStatus
	BusinessName   *string
	ContactEmail   *string
	ContactPhone   *string
	Region         *string
	City           *string
	CreatedAfter   *time.Time
	CreatedBefore  *time.Time
	Limit          int
	Offset         int
	OrderBy        string
	OrderDirection string
}

type agentRepository struct {
	db *sqlx.DB
}

// NewAgentRepository creates a new agent repository
func NewAgentRepository(db *sqlx.DB) AgentRepository {
	return &agentRepository{db: db}
}

// NewAgentStatusRepository creates a new agent status repository
func NewAgentStatusRepository(db *sqlx.DB) AgentStatusRepository {
	return &agentRepository{db: db}
}

func (r *agentRepository) Create(ctx context.Context, agent *models.Agent) error {
	query := `
		INSERT INTO agents (
			id, agent_code, business_name, registration_number, tax_id,
			contact_email, contact_phone, primary_contact_name,
			physical_address, city, region, gps_coordinates,
			bank_name, bank_account_number, bank_account_name,
			status, onboarding_method, commission_percentage,
			created_by, updated_by, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, 
			$16, $17, $18, $19, $20, $21, $22
		)`

	agent.ID = uuid.New()
	agent.CreatedAt = time.Now()
	agent.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, query,
		agent.ID, agent.AgentCode, agent.BusinessName, agent.RegistrationNumber, agent.TaxID,
		agent.ContactEmail, agent.ContactPhone, agent.PrimaryContactName,
		agent.PhysicalAddress, agent.City, agent.Region, agent.GPSCoordinates,
		agent.BankName, agent.BankAccountNumber, agent.BankAccountName,
		agent.Status, agent.OnboardingMethod, agent.CommissionPercentage,
		agent.CreatedBy, agent.UpdatedBy, agent.CreatedAt, agent.UpdatedAt,
	)

	return err
}

func (r *agentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	query := `
		SELECT id, agent_code, business_name, registration_number, tax_id,
			   contact_email, contact_phone, primary_contact_name,
			   physical_address, city, region, gps_coordinates,
			   bank_name, bank_account_number, bank_account_name,
			   status, onboarding_method, commission_percentage,
			   created_by, updated_by, created_at, updated_at
		FROM agents
		WHERE id = $1`

	row := r.db.QueryRowContext(ctx, query, id)

	agent := &models.Agent{}

	err := row.Scan(
		&agent.ID, &agent.AgentCode, &agent.BusinessName, &agent.RegistrationNumber, &agent.TaxID,
		&agent.ContactEmail, &agent.ContactPhone, &agent.PrimaryContactName,
		&agent.PhysicalAddress, &agent.City, &agent.Region, &agent.GPSCoordinates,
		&agent.BankName, &agent.BankAccountNumber, &agent.BankAccountName,
		&agent.Status, &agent.OnboardingMethod, &agent.CommissionPercentage,
		&agent.CreatedBy, &agent.UpdatedBy, &agent.CreatedAt, &agent.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return agent, nil
}

func (r *agentRepository) GetByCode(ctx context.Context, agentCode string) (*models.Agent, error) {
	query := `
		SELECT a.id, a.agent_code, a.business_name, a.registration_number, a.tax_id,
			   a.contact_email, a.contact_phone, a.primary_contact_name,
			   a.physical_address, a.city, a.region, a.gps_coordinates,
			   a.bank_name, a.bank_account_number, a.bank_account_name,
			   a.status, a.onboarding_method, a.commission_percentage,
			   a.created_by, a.updated_by, a.created_at, a.updated_at
		FROM agents a
		WHERE a.agent_code = $1`

	row := r.db.QueryRowContext(ctx, query, agentCode)

	agent := &models.Agent{}
	err := row.Scan(
		&agent.ID, &agent.AgentCode, &agent.BusinessName, &agent.RegistrationNumber, &agent.TaxID,
		&agent.ContactEmail, &agent.ContactPhone, &agent.PrimaryContactName,
		&agent.PhysicalAddress, &agent.City, &agent.Region, &agent.GPSCoordinates,
		&agent.BankName, &agent.BankAccountNumber, &agent.BankAccountName,
		&agent.Status, &agent.OnboardingMethod, &agent.CommissionPercentage,
		&agent.CreatedBy, &agent.UpdatedBy, &agent.CreatedAt, &agent.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return agent, nil
}

func (r *agentRepository) GetByEmail(ctx context.Context, email string) (*models.Agent, error) {
	query := `
		SELECT a.id, a.agent_code, a.business_name, a.registration_number, a.tax_id,
			   a.contact_email, a.contact_phone, a.primary_contact_name,
			   a.physical_address, a.city, a.region, a.gps_coordinates,
			   a.bank_name, a.bank_account_number, a.bank_account_name,
			   a.status, a.onboarding_method, a.commission_percentage,
			   a.created_by, a.updated_by, a.created_at, a.updated_at
		FROM agents a
		WHERE a.contact_email = $1`

	row := r.db.QueryRowContext(ctx, query, email)

	agent := &models.Agent{}
	err := row.Scan(
		&agent.ID, &agent.AgentCode, &agent.BusinessName, &agent.RegistrationNumber, &agent.TaxID,
		&agent.ContactEmail, &agent.ContactPhone, &agent.PrimaryContactName,
		&agent.PhysicalAddress, &agent.City, &agent.Region, &agent.GPSCoordinates,
		&agent.BankName, &agent.BankAccountNumber, &agent.BankAccountName,
		&agent.Status, &agent.OnboardingMethod, &agent.CommissionPercentage,
		&agent.CreatedBy, &agent.UpdatedBy, &agent.CreatedAt, &agent.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return agent, nil
}

func (r *agentRepository) Update(ctx context.Context, agent *models.Agent) error {
	query := `
		UPDATE agents SET
			business_name = $2, registration_number = $3, tax_id = $4,
			contact_email = $5, contact_phone = $6, primary_contact_name = $7,
			physical_address = $8, city = $9, region = $10, gps_coordinates = $11,
			bank_name = $12, bank_account_number = $13, bank_account_name = $14,
			status = $15, commission_percentage = $16,
			updated_by = $17, updated_at = $18
		WHERE id = $1`

	agent.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		agent.ID, agent.BusinessName, agent.RegistrationNumber, agent.TaxID,
		agent.ContactEmail, agent.ContactPhone, agent.PrimaryContactName,
		agent.PhysicalAddress, agent.City, agent.Region, agent.GPSCoordinates,
		agent.BankName, agent.BankAccountNumber, agent.BankAccountName,
		agent.Status, agent.CommissionPercentage,
		agent.UpdatedBy, agent.UpdatedAt,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("agent not found")
	}

	return nil
}

func (r *agentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM agents WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("agent not found")
	}

	return nil
}

func (r *agentRepository) List(ctx context.Context, filters AgentFilters) ([]models.Agent, error) {
	baseQuery := `
		SELECT id, agent_code, business_name, registration_number, tax_id,
			   contact_email, contact_phone, primary_contact_name,
			   physical_address, city, region, gps_coordinates,
			   bank_name, bank_account_number, bank_account_name,
			   status, onboarding_method, commission_percentage,
			   created_by, updated_by, created_at, updated_at
		FROM agents`

	whereConditions := []string{}
	args := []interface{}{}
	argIndex := 1

	// Apply filters
	if filters.Status != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *filters.Status)
		argIndex++
	}

	if filters.BusinessName != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("business_name ILIKE $%d", argIndex))
		args = append(args, "%"+*filters.BusinessName+"%")
		argIndex++
	}

	if filters.ContactEmail != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("contact_email = $%d", argIndex))
		args = append(args, *filters.ContactEmail)
		argIndex++
	}

	if filters.ContactPhone != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("contact_phone = $%d", argIndex))
		args = append(args, *filters.ContactPhone)
		argIndex++
	}

	if filters.Region != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("region ILIKE $%d", argIndex))
		args = append(args, "%"+*filters.Region+"%")
		argIndex++
	}

	if filters.City != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("city ILIKE $%d", argIndex))
		args = append(args, "%"+*filters.City+"%")
		argIndex++
	}

	if filters.CreatedAfter != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filters.CreatedAfter)
		argIndex++
	}

	if filters.CreatedBefore != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filters.CreatedBefore)
		argIndex++
	}

	// Build final query
	query := baseQuery
	if len(whereConditions) > 0 {
		query += " WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Add ordering
	orderBy := "created_at"
	if filters.OrderBy != "" {
		orderBy = filters.OrderBy
	}

	orderDirection := "DESC"
	if filters.OrderDirection != "" && strings.ToUpper(filters.OrderDirection) == "ASC" {
		orderDirection = "ASC"
	}

	query += fmt.Sprintf(" ORDER BY %s %s", orderBy, orderDirection)

	// Add pagination
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filters.Limit)
		argIndex++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filters.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var agents []models.Agent
	for rows.Next() {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		agent := models.Agent{}

		err := rows.Scan(
			&agent.ID, &agent.AgentCode, &agent.BusinessName, &agent.RegistrationNumber, &agent.TaxID,
			&agent.ContactEmail, &agent.ContactPhone, &agent.PrimaryContactName,
			&agent.PhysicalAddress, &agent.City, &agent.Region, &agent.GPSCoordinates,
			&agent.BankName, &agent.BankAccountNumber, &agent.BankAccountName,
			&agent.Status, &agent.OnboardingMethod, &agent.CommissionPercentage,
			&agent.CreatedBy, &agent.UpdatedBy, &agent.CreatedAt, &agent.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		agents = append(agents, agent)
	}

	return agents, rows.Err()
}

func (r *agentRepository) Count(ctx context.Context, filters AgentFilters) (int, error) {
	baseQuery := `SELECT COUNT(*) FROM agents`

	whereConditions := []string{}
	args := []interface{}{}
	argIndex := 1

	// Apply same filters as List method
	if filters.Status != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *filters.Status)
		argIndex++
	}

	if filters.BusinessName != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("business_name ILIKE $%d", argIndex))
		args = append(args, "%"+*filters.BusinessName+"%")
		argIndex++
	}

	if filters.Region != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("region ILIKE $%d", argIndex))
		args = append(args, "%"+*filters.Region+"%")
		argIndex++
	}

	if filters.CreatedAfter != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filters.CreatedAfter)
		argIndex++
	}

	if filters.CreatedBefore != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("a.created_at <= $%d", argIndex))
		args = append(args, *filters.CreatedBefore)
	}

	query := baseQuery
	if len(whereConditions) > 0 {
		query += " WHERE " + strings.Join(whereConditions, " AND ")
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *agentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.EntityStatus, updatedBy string) error {
	query := `
		UPDATE agents SET
			status = $2, updated_by = $3, updated_at = $4
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id, status, updatedBy, time.Now())
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("agent not found")
	}

	return nil
}

func (r *agentRepository) GetByStatus(ctx context.Context, status models.EntityStatus) ([]models.Agent, error) {
	filters := AgentFilters{Status: &status}
	return r.List(ctx, filters)
}

// Commission tier methods removed - now using direct commission percentage on agents

func (r *agentRepository) GetNextAgentCode(ctx context.Context) (string, error) {
	// Get the highest numeric agent code
	// Per PRD: Agent codes are 4-digit numbers (e.g., 1001, 9999)
	// Expandable to 5+ digits when 4-digit capacity is reached
	query := `
		SELECT agent_code
		FROM agents
		WHERE agent_code ~ '^\d+$'
		ORDER BY CAST(agent_code AS INTEGER) DESC
		LIMIT 1`

	var lastCode sql.NullString
	err := r.db.QueryRowContext(ctx, query).Scan(&lastCode)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}

	nextNumber := 1001 // Start from 1001 as default
	if lastCode.Valid {
		// Parse the last code as an integer
		if lastNum, err := strconv.Atoi(lastCode.String); err == nil {
			nextNumber = lastNum + 1
		}
	}

	// Return as string (will be 4 digits initially, can grow to 5+ when needed)
	return strconv.Itoa(nextNumber), nil
}

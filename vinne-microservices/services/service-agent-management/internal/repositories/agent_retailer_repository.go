package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
)

// AgentRetailerRepository defines the interface for agent-retailer relationship operations
type AgentRetailerRepository interface {
	Create(ctx context.Context, relationship *models.AgentRetailer) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AgentRetailer, error)
	GetByAgentID(ctx context.Context, agentID uuid.UUID) ([]models.AgentRetailer, error)
	GetByRetailerID(ctx context.Context, retailerID uuid.UUID) (*models.AgentRetailer, error)
	GetActiveRelationships(ctx context.Context) ([]models.AgentRetailer, error)
	Update(ctx context.Context, relationship *models.AgentRetailer) error
	DeactivateRelationship(ctx context.Context, id uuid.UUID, unassignedBy string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type agentRetailerRepository struct {
	db *sqlx.DB
}

// NewAgentRetailerRepository creates a new agent-retailer repository
func NewAgentRetailerRepository(db *sqlx.DB) AgentRetailerRepository {
	return &agentRetailerRepository{db: db}
}

func (r *agentRetailerRepository) Create(ctx context.Context, relationship *models.AgentRetailer) error {
	if relationship.ID == uuid.Nil {
		relationship.ID = uuid.New()
	}

	if relationship.AssignedDate.IsZero() {
		relationship.AssignedDate = time.Now()
	}

	relationship.CreatedAt = time.Now()

	query := `
		INSERT INTO agent_retailers (
			id, agent_id, retailer_id, relationship_type,
			assigned_date, assigned_by, notes, is_active, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)`

	_, err := r.db.ExecContext(ctx, query,
		relationship.ID,
		relationship.AgentID,
		relationship.RetailerID,
		relationship.RelationshipType,
		relationship.AssignedDate,
		relationship.AssignedBy,
		relationship.Notes,
		relationship.IsActive,
		relationship.CreatedAt,
	)

	return err
}

func (r *agentRetailerRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AgentRetailer, error) {
	var relationship models.AgentRetailer
	query := `
		SELECT id, agent_id, retailer_id, relationship_type,
		       assigned_date, assigned_by, notes, is_active, created_at
		FROM agent_retailers
		WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&relationship.ID,
		&relationship.AgentID,
		&relationship.RetailerID,
		&relationship.RelationshipType,
		&relationship.AssignedDate,
		&relationship.AssignedBy,
		&relationship.Notes,
		&relationship.IsActive,
		&relationship.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &relationship, nil
}

func (r *agentRetailerRepository) GetByAgentID(ctx context.Context, agentID uuid.UUID) ([]models.AgentRetailer, error) {
	query := `
		SELECT id, agent_id, retailer_id, relationship_type,
		       assigned_date, assigned_by, notes, is_active, created_at
		FROM agent_retailers
		WHERE agent_id = $1 AND is_active = true
		ORDER BY assigned_date DESC`

	rows, err := r.db.QueryContext(ctx, query, agentID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var relationships []models.AgentRetailer
	for rows.Next() {
		var rel models.AgentRetailer
		err := rows.Scan(
			&rel.ID,
			&rel.AgentID,
			&rel.RetailerID,
			&rel.RelationshipType,
			&rel.AssignedDate,
			&rel.AssignedBy,
			&rel.Notes,
			&rel.IsActive,
			&rel.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, rel)
	}

	return relationships, nil
}

func (r *agentRetailerRepository) GetByRetailerID(ctx context.Context, retailerID uuid.UUID) (*models.AgentRetailer, error) {
	var relationship models.AgentRetailer
	query := `
		SELECT id, agent_id, retailer_id, relationship_type,
		       assigned_date, assigned_by, notes, is_active, created_at
		FROM agent_retailers
		WHERE retailer_id = $1 AND is_active = true
		ORDER BY assigned_date DESC
		LIMIT 1`

	err := r.db.QueryRowContext(ctx, query, retailerID).Scan(
		&relationship.ID,
		&relationship.AgentID,
		&relationship.RetailerID,
		&relationship.RelationshipType,
		&relationship.AssignedDate,
		&relationship.AssignedBy,
		&relationship.Notes,
		&relationship.IsActive,
		&relationship.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &relationship, nil
}

func (r *agentRetailerRepository) GetActiveRelationships(ctx context.Context) ([]models.AgentRetailer, error) {
	query := `
		SELECT id, agent_id, retailer_id, relationship_type,
		       assigned_date, assigned_by, notes, is_active, created_at
		FROM agent_retailers
		WHERE is_active = true
		ORDER BY assigned_date DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var relationships []models.AgentRetailer
	for rows.Next() {
		var rel models.AgentRetailer
		err := rows.Scan(
			&rel.ID,
			&rel.AgentID,
			&rel.RetailerID,
			&rel.RelationshipType,
			&rel.AssignedDate,
			&rel.AssignedBy,
			&rel.Notes,
			&rel.IsActive,
			&rel.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, rel)
	}

	return relationships, nil
}

func (r *agentRetailerRepository) Update(ctx context.Context, relationship *models.AgentRetailer) error {
	query := `
		UPDATE agent_retailers
		SET agent_id = $1, retailer_id = $2, relationship_type = $3,
		    assigned_date = $4, assigned_by = $5, notes = $6, is_active = $7
		WHERE id = $8`

	_, err := r.db.ExecContext(ctx, query,
		relationship.AgentID,
		relationship.RetailerID,
		relationship.RelationshipType,
		relationship.AssignedDate,
		relationship.AssignedBy,
		relationship.Notes,
		relationship.IsActive,
		relationship.ID,
	)

	return err
}

func (r *agentRetailerRepository) DeactivateRelationship(ctx context.Context, id uuid.UUID, unassignedBy string) error {
	query := `
		UPDATE agent_retailers
		SET is_active = false, 
		    notes = COALESCE(notes || E'\n', '') || 'Deactivated by ' || $2 || ' at ' || NOW()
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id, unassignedBy)
	return err
}

func (r *agentRetailerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM agent_retailers WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

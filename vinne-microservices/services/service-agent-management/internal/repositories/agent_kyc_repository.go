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

// AgentKYCRepository defines the interface for agent KYC operations
type AgentKYCRepository interface {
	Create(ctx context.Context, kyc *models.AgentKYC) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AgentKYC, error)
	GetByAgentID(ctx context.Context, agentID uuid.UUID) (*models.AgentKYC, error)
	Update(ctx context.Context, kyc *models.AgentKYC) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.KYCStatus, reviewedBy string) error
	GetByStatus(ctx context.Context, status models.KYCStatus) ([]models.AgentKYC, error)
	GetExpiringSoon(ctx context.Context, days int) ([]models.AgentKYC, error)
}

type agentKYCRepository struct {
	db *sqlx.DB
}

// NewAgentKYCRepository creates a new agent KYC repository
func NewAgentKYCRepository(db *sqlx.DB) AgentKYCRepository {
	return &agentKYCRepository{db: db}
}

func (r *agentKYCRepository) Create(ctx context.Context, kyc *models.AgentKYC) error {
	if kyc.ID == uuid.Nil {
		kyc.ID = uuid.New()
	}

	kyc.CreatedAt = time.Now()
	kyc.UpdatedAt = time.Now()

	query := `
		INSERT INTO agent_kyc (
			id, agent_id, kyc_status, business_registration_cert,
			tax_clearance_cert, director_id_document, proof_of_address,
			bank_account_verification, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)`

	_, err := r.db.ExecContext(ctx, query,
		kyc.ID,
		kyc.AgentID,
		kyc.KYCStatus,
		kyc.BusinessRegistrationCert,
		kyc.TaxClearanceCert,
		kyc.DirectorIDDocument,
		kyc.ProofOfAddress,
		kyc.BankAccountVerification,
		kyc.CreatedAt,
		kyc.UpdatedAt,
	)

	return err
}

func (r *agentKYCRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AgentKYC, error) {
	var kyc models.AgentKYC
	query := `
		SELECT id, agent_id, kyc_status, business_registration_cert,
		       tax_clearance_cert, director_id_document, proof_of_address,
		       bank_account_verification, reviewed_by, reviewed_at,
		       rejection_reason, notes, expires_at,
		       created_at, updated_at
		FROM agent_kyc
		WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&kyc.ID, &kyc.AgentID, &kyc.KYCStatus,
		&kyc.BusinessRegistrationCert, &kyc.TaxClearanceCert,
		&kyc.DirectorIDDocument, &kyc.ProofOfAddress,
		&kyc.BankAccountVerification, &kyc.ReviewedBy,
		&kyc.ReviewedAt, &kyc.RejectionReason, &kyc.Notes,
		&kyc.ExpiresAt, &kyc.CreatedAt, &kyc.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("agent KYC not found")
		}
		return nil, err
	}

	return &kyc, nil
}

func (r *agentKYCRepository) GetByAgentID(ctx context.Context, agentID uuid.UUID) (*models.AgentKYC, error) {
	var kyc models.AgentKYC
	query := `
		SELECT id, agent_id, kyc_status, business_registration_cert,
		       tax_clearance_cert, director_id_document, proof_of_address,
		       bank_account_verification, reviewed_by, reviewed_at,
		       rejection_reason, notes, expires_at,
		       created_at, updated_at
		FROM agent_kyc
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	err := r.db.QueryRowContext(ctx, query, agentID).Scan(
		&kyc.ID, &kyc.AgentID, &kyc.KYCStatus,
		&kyc.BusinessRegistrationCert, &kyc.TaxClearanceCert,
		&kyc.DirectorIDDocument, &kyc.ProofOfAddress,
		&kyc.BankAccountVerification, &kyc.ReviewedBy,
		&kyc.ReviewedAt, &kyc.RejectionReason, &kyc.Notes,
		&kyc.ExpiresAt, &kyc.CreatedAt, &kyc.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No KYC record found for agent
		}
		return nil, err
	}

	return &kyc, nil
}

func (r *agentKYCRepository) Update(ctx context.Context, kyc *models.AgentKYC) error {
	kyc.UpdatedAt = time.Now()

	query := `
		UPDATE agent_kyc SET
			kyc_status = $2, business_registration_cert = $3,
			tax_clearance_cert = $4, director_id_document = $5,
			proof_of_address = $6, bank_account_verification = $7,
			reviewed_by = $8, reviewed_at = $9, rejection_reason = $10,
			notes = $11, expires_at = $12, updated_at = $13
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		kyc.ID, kyc.KYCStatus, kyc.BusinessRegistrationCert,
		kyc.TaxClearanceCert, kyc.DirectorIDDocument,
		kyc.ProofOfAddress, kyc.BankAccountVerification,
		kyc.ReviewedBy, kyc.ReviewedAt, kyc.RejectionReason,
		kyc.Notes, kyc.ExpiresAt, kyc.UpdatedAt,
	)

	return err
}

func (r *agentKYCRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.KYCStatus, reviewedBy string) error {
	now := time.Now()
	query := `
		UPDATE agent_kyc SET
			kyc_status = $2,
			reviewed_by = $3,
			reviewed_at = $4,
			updated_at = $5
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		id, status, reviewedBy, now, now,
	)

	return err
}

func (r *agentKYCRepository) GetPendingKYC(ctx context.Context) ([]models.AgentKYC, error) {
	return r.GetByStatus(ctx, models.KYCStatusPending)
}

func (r *agentKYCRepository) GetByStatus(ctx context.Context, status models.KYCStatus) ([]models.AgentKYC, error) {
	query := `
		SELECT id, agent_id, kyc_status, business_registration_cert,
		       tax_clearance_cert, director_id_document, proof_of_address,
		       bank_account_verification, reviewed_by, reviewed_at,
		       rejection_reason, notes, expires_at,
		       created_at, updated_at
		FROM agent_kyc
		WHERE kyc_status = $1
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var kycList []models.AgentKYC
	for rows.Next() {
		var kyc models.AgentKYC
		err := rows.Scan(
			&kyc.ID, &kyc.AgentID, &kyc.KYCStatus,
			&kyc.BusinessRegistrationCert, &kyc.TaxClearanceCert,
			&kyc.DirectorIDDocument, &kyc.ProofOfAddress,
			&kyc.BankAccountVerification, &kyc.ReviewedBy,
			&kyc.ReviewedAt, &kyc.RejectionReason, &kyc.Notes,
			&kyc.ExpiresAt, &kyc.CreatedAt, &kyc.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		kycList = append(kycList, kyc)
	}

	return kycList, nil
}

func (r *agentKYCRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM agent_kyc WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *agentKYCRepository) GetExpiringSoon(ctx context.Context, days int) ([]models.AgentKYC, error) {
	expiryDate := time.Now().AddDate(0, 0, days)

	query := `
		SELECT id, agent_id, kyc_status, business_registration_cert,
		       tax_clearance_cert, director_id_document, proof_of_address,
		       bank_account_verification, reviewed_by, reviewed_at,
		       rejection_reason, notes, expires_at,
		       created_at, updated_at
		FROM agent_kyc
		WHERE kyc_status = $1 
		  AND expires_at IS NOT NULL
		  AND expires_at <= $2
		  AND expires_at > $3
		ORDER BY expires_at ASC`

	rows, err := r.db.QueryContext(ctx, query,
		models.KYCStatusApproved, expiryDate, time.Now(),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var kycList []models.AgentKYC
	for rows.Next() {
		var kyc models.AgentKYC
		err := rows.Scan(
			&kyc.ID, &kyc.AgentID, &kyc.KYCStatus,
			&kyc.BusinessRegistrationCert, &kyc.TaxClearanceCert,
			&kyc.DirectorIDDocument, &kyc.ProofOfAddress,
			&kyc.BankAccountVerification, &kyc.ReviewedBy,
			&kyc.ReviewedAt, &kyc.RejectionReason, &kyc.Notes,
			&kyc.ExpiresAt, &kyc.CreatedAt, &kyc.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		kycList = append(kycList, kyc)
	}

	return kycList, nil
}

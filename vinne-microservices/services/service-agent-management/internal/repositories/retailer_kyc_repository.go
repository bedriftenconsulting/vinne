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

// RetailerKYCRepository defines the interface for retailer KYC operations
type RetailerKYCRepository interface {
	Create(ctx context.Context, kyc *models.RetailerKYC) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.RetailerKYC, error)
	GetByRetailerID(ctx context.Context, retailerID uuid.UUID) (*models.RetailerKYC, error)
	Update(ctx context.Context, kyc *models.RetailerKYC) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.KYCStatus, reviewedBy string) error
	GetByStatus(ctx context.Context, status models.KYCStatus) ([]models.RetailerKYC, error)
	GetExpiringSoon(ctx context.Context, days int) ([]models.RetailerKYC, error)
}

type retailerKYCRepository struct {
	db *sqlx.DB
}

// NewRetailerKYCRepository creates a new retailer KYC repository
func NewRetailerKYCRepository(db *sqlx.DB) RetailerKYCRepository {
	return &retailerKYCRepository{db: db}
}

func (r *retailerKYCRepository) Create(ctx context.Context, kyc *models.RetailerKYC) error {
	query := `
		INSERT INTO retailer_kyc (
			id, retailer_id, kyc_status, business_license,
			owner_id_document, proof_of_address,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)`

	kyc.ID = uuid.New()
	kyc.CreatedAt = time.Now()
	kyc.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, query,
		kyc.ID,
		kyc.RetailerID,
		kyc.KYCStatus,
		kyc.BusinessLicense,
		kyc.OwnerIDDocument,
		kyc.ProofOfAddress,
		kyc.CreatedAt,
		kyc.UpdatedAt,
	)

	return err
}

func (r *retailerKYCRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.RetailerKYC, error) {
	var kyc models.RetailerKYC
	query := `
		SELECT id, retailer_id, kyc_status, business_license,
		       owner_id_document, proof_of_address,
		       reviewed_by, reviewed_at,
		       rejection_reason, notes, expires_at,
		       created_at, updated_at
		FROM retailer_kyc
		WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&kyc.ID, &kyc.RetailerID, &kyc.KYCStatus,
		&kyc.BusinessLicense,
		&kyc.OwnerIDDocument, &kyc.ProofOfAddress,
		&kyc.ReviewedBy,
		&kyc.ReviewedAt, &kyc.RejectionReason, &kyc.Notes,
		&kyc.ExpiresAt, &kyc.CreatedAt, &kyc.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("retailer KYC not found")
		}
		return nil, err
	}

	return &kyc, nil
}

func (r *retailerKYCRepository) GetByRetailerID(ctx context.Context, retailerID uuid.UUID) (*models.RetailerKYC, error) {
	var kyc models.RetailerKYC
	query := `
		SELECT id, retailer_id, kyc_status, business_license,
		       owner_id_document, proof_of_address,
		       reviewed_by, reviewed_at,
		       rejection_reason, notes, expires_at,
		       created_at, updated_at
		FROM retailer_kyc
		WHERE retailer_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	err := r.db.QueryRowContext(ctx, query, retailerID).Scan(
		&kyc.ID, &kyc.RetailerID, &kyc.KYCStatus,
		&kyc.BusinessLicense,
		&kyc.OwnerIDDocument, &kyc.ProofOfAddress,
		&kyc.ReviewedBy,
		&kyc.ReviewedAt, &kyc.RejectionReason, &kyc.Notes,
		&kyc.ExpiresAt, &kyc.CreatedAt, &kyc.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No KYC record found for retailer
		}
		return nil, err
	}

	return &kyc, nil
}

func (r *retailerKYCRepository) Update(ctx context.Context, kyc *models.RetailerKYC) error {
	kyc.UpdatedAt = time.Now()

	query := `
		UPDATE retailer_kyc SET
			kyc_status = $2, business_license = $3,
			owner_id_document = $4,
			proof_of_address = $5,
			reviewed_by = $6, reviewed_at = $7, rejection_reason = $8,
			notes = $9, expires_at = $10, updated_at = $11
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		kyc.ID, kyc.KYCStatus, kyc.BusinessLicense,
		kyc.OwnerIDDocument,
		kyc.ProofOfAddress,
		kyc.ReviewedBy, kyc.ReviewedAt, kyc.RejectionReason,
		kyc.Notes, kyc.ExpiresAt, kyc.UpdatedAt,
	)

	return err
}

func (r *retailerKYCRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.KYCStatus, reviewedBy string) error {
	now := time.Now()
	query := `
		UPDATE retailer_kyc SET
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

func (r *retailerKYCRepository) GetPendingKYC(ctx context.Context) ([]models.RetailerKYC, error) {
	return r.GetByStatus(ctx, models.KYCStatusPending)
}

func (r *retailerKYCRepository) GetByStatus(ctx context.Context, status models.KYCStatus) ([]models.RetailerKYC, error) {
	query := `
		SELECT id, retailer_id, kyc_status, business_license,
		       owner_id_document, proof_of_address,
		       reviewed_by, reviewed_at,
		       rejection_reason, notes, expires_at,
		       created_at, updated_at
		FROM retailer_kyc
		WHERE kyc_status = $1
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var kycList []models.RetailerKYC
	for rows.Next() {
		var kyc models.RetailerKYC
		err := rows.Scan(
			&kyc.ID, &kyc.RetailerID, &kyc.KYCStatus,
			&kyc.BusinessLicense,
			&kyc.OwnerIDDocument, &kyc.ProofOfAddress,
			&kyc.ReviewedBy,
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

func (r *retailerKYCRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM retailer_kyc WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *retailerKYCRepository) GetExpiringSoon(ctx context.Context, days int) ([]models.RetailerKYC, error) {
	expiryDate := time.Now().AddDate(0, 0, days)

	query := `
		SELECT id, retailer_id, kyc_status, business_license,
		       owner_id_document, proof_of_address,
		       reviewed_by, reviewed_at,
		       rejection_reason, notes, expires_at,
		       created_at, updated_at
		FROM retailer_kyc
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

	var kycList []models.RetailerKYC
	for rows.Next() {
		var kyc models.RetailerKYC
		err := rows.Scan(
			&kyc.ID, &kyc.RetailerID, &kyc.KYCStatus,
			&kyc.BusinessLicense,
			&kyc.OwnerIDDocument, &kyc.ProofOfAddress,
			&kyc.ReviewedBy,
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

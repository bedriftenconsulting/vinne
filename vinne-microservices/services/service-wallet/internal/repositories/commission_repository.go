package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/service-wallet/internal/models"
)

// CommissionRepository handles commission-related database operations
type CommissionRepository interface {
	Create(ctx context.Context, rate *models.AgentCommissionRate) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AgentCommissionRate, error)
	GetByAgentID(ctx context.Context, agentID uuid.UUID) (*models.AgentCommissionRate, error)
	Update(ctx context.Context, rate *models.AgentCommissionRate) error
	RecordTransaction(ctx context.Context, transaction *models.CommissionTransaction) error
	RecordCalculation(ctx context.Context, calculation *models.CommissionCalculation) error
	RecordAudit(ctx context.Context, audit *models.CommissionAudit) error
	GetTransactionHistory(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*models.CommissionTransaction, int, error)
	GetTransactionsByDateRange(ctx context.Context, agentID uuid.UUID, startDate, endDate time.Time) ([]*models.CommissionTransaction, error)
	GetCalculationsByDateRange(ctx context.Context, agentID uuid.UUID, startDate, endDate time.Time, limit, offset int) ([]*models.CommissionCalculation, int, error)
	GetCalculationByTransactionID(ctx context.Context, transactionID uuid.UUID) (*models.CommissionCalculation, error)
	GetDailyCommissions(ctx context.Context, date time.Time) (*DailyCommissions, error)
}

// DailyCommissions holds commission metrics for a specific day and the previous day
type DailyCommissions struct {
	TodayAmount     int64
	YesterdayAmount int64
}

type commissionRepository struct {
	db *sql.DB
}

// NewCommissionRepository creates a new commission repository
func NewCommissionRepository(db *sql.DB) CommissionRepository {
	return &commissionRepository{db: db}
}

// Create creates a new agent commission rate
func (r *commissionRepository) Create(ctx context.Context, rate *models.AgentCommissionRate) error {
	query := `
		INSERT INTO agent_commission_rates (
			id, agent_id, rate, effective_from, effective_to,
			notes, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(ctx, query,
		rate.ID,
		rate.AgentID,
		rate.Rate,
		rate.EffectiveFrom,
		rate.EffectiveTo,
		rate.Notes,
		rate.CreatedBy,
		rate.CreatedAt,
		rate.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create commission rate: %w", err)
	}

	return nil
}

// GetByID retrieves a commission rate by ID
func (r *commissionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AgentCommissionRate, error) {
	query := `
		SELECT 
			id, agent_id, rate, effective_from, effective_to,
			notes, created_by, created_at, updated_at
		FROM agent_commission_rates
		WHERE id = $1
	`

	rate := &models.AgentCommissionRate{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&rate.ID,
		&rate.AgentID,
		&rate.Rate,
		&rate.EffectiveFrom,
		&rate.EffectiveTo,
		&rate.Notes,
		&rate.CreatedBy,
		&rate.CreatedAt,
		&rate.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return rate, nil
}

// GetByAgentID retrieves the active commission rate for an agent
func (r *commissionRepository) GetByAgentID(ctx context.Context, agentID uuid.UUID) (*models.AgentCommissionRate, error) {
	query := `
		SELECT 
			id, agent_id, rate, effective_from, effective_to,
			notes, created_by, created_at, updated_at
		FROM agent_commission_rates
		WHERE agent_id = $1 AND effective_to IS NULL
		ORDER BY effective_from DESC
		LIMIT 1
	`

	rate := &models.AgentCommissionRate{}
	err := r.db.QueryRowContext(ctx, query, agentID).Scan(
		&rate.ID,
		&rate.AgentID,
		&rate.Rate,
		&rate.EffectiveFrom,
		&rate.EffectiveTo,
		&rate.Notes,
		&rate.CreatedBy,
		&rate.CreatedAt,
		&rate.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return rate, nil
}

// Update updates an existing commission rate
func (r *commissionRepository) Update(ctx context.Context, rate *models.AgentCommissionRate) error {
	query := `
		UPDATE agent_commission_rates
		SET 
			rate = $2,
			effective_from = $3,
			effective_to = $4,
			notes = $5,
			updated_at = $6
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		rate.ID,
		rate.Rate,
		rate.EffectiveFrom,
		rate.EffectiveTo,
		rate.Notes,
		rate.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update commission rate: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// RecordTransaction records a commission transaction
func (r *commissionRepository) RecordTransaction(ctx context.Context, transaction *models.CommissionTransaction) error {
	query := `
		INSERT INTO commission_transactions (
			id, commission_id, transaction_id, agent_id,
			original_amount, gross_amount, commission_amount, commission_rate,
			commission_type, status, reference, notes,
			created_at, credited_at, reversed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	_, err := r.db.ExecContext(ctx, query,
		transaction.ID,
		transaction.CommissionID,
		transaction.TransactionID,
		transaction.AgentID,
		transaction.OriginalAmount,
		transaction.GrossAmount,
		transaction.CommissionAmount,
		transaction.CommissionRate,
		transaction.CommissionType,
		transaction.Status,
		transaction.Reference,
		transaction.Notes,
		transaction.CreatedAt,
		transaction.CreditedAt,
		transaction.ReversedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to record commission transaction: %w", err)
	}

	return nil
}

// RecordCalculation records a commission calculation
func (r *commissionRepository) RecordCalculation(ctx context.Context, calculation *models.CommissionCalculation) error {
	query := `
		INSERT INTO commission_calculations (
			id, agent_id, transaction_type, input_amount, rate_basis_points,
			commission_amount, gross_amount, net_amount, calculated_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.ExecContext(ctx, query,
		calculation.ID,
		calculation.AgentID,
		calculation.TransactionType,
		calculation.InputAmount,
		calculation.RateBasisPoints,
		calculation.CommissionAmount,
		calculation.GrossAmount,
		calculation.NetAmount,
		calculation.CalculatedAt,
		calculation.CreatedAt,
		calculation.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to record commission calculation: %w", err)
	}

	return nil
}

// RecordAudit records a commission audit entry
func (r *commissionRepository) RecordAudit(ctx context.Context, audit *models.CommissionAudit) error {
	query := `
		INSERT INTO commission_audit (
			id, agent_id, transaction_id, commission_amount,
			action, action_by, action_at, details, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(ctx, query,
		audit.ID,
		audit.AgentID,
		audit.TransactionID,
		audit.CommissionAmount,
		audit.Action,
		audit.ActionBy,
		audit.ActionAt,
		audit.Details,
		audit.CreatedAt,
		audit.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to record commission audit: %w", err)
	}

	return nil
}

// GetTransactionHistory retrieves commission transaction history for an agent
func (r *commissionRepository) GetTransactionHistory(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*models.CommissionTransaction, int, error) {
	// Count total transactions
	countQuery := `SELECT COUNT(*) FROM commission_transactions WHERE agent_id = $1`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, agentID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	// Get transactions
	query := `
		SELECT
			id, commission_id, transaction_id, agent_id,
			original_amount, gross_amount, commission_amount, commission_rate,
			commission_type, status, reference, notes,
			created_at, credited_at, reversed_at
		FROM commission_transactions
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, agentID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get transaction history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var transactions []*models.CommissionTransaction
	for rows.Next() {
		tx := &models.CommissionTransaction{}
		err := rows.Scan(
			&tx.ID,
			&tx.CommissionID,
			&tx.TransactionID,
			&tx.AgentID,
			&tx.OriginalAmount,
			&tx.GrossAmount,
			&tx.CommissionAmount,
			&tx.CommissionRate,
			&tx.CommissionType,
			&tx.Status,
			&tx.Reference,
			&tx.Notes,
			&tx.CreatedAt,
			&tx.CreditedAt,
			&tx.ReversedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan transaction: %w", err)
		}
		// Set UpdatedAt to CreatedAt since it's not in the database
		tx.UpdatedAt = tx.CreatedAt
		transactions = append(transactions, tx)
	}

	return transactions, total, nil
}

// GetTransactionsByDateRange retrieves commission transactions within a date range
func (r *commissionRepository) GetTransactionsByDateRange(ctx context.Context, agentID uuid.UUID, startDate, endDate time.Time) ([]*models.CommissionTransaction, error) {
	query := `
		SELECT
			id, commission_id, transaction_id, agent_id,
			original_amount, gross_amount, commission_amount, commission_rate,
			commission_type, status, reference, notes,
			created_at, credited_at, reversed_at
		FROM commission_transactions
		WHERE agent_id = $1 AND created_at >= $2 AND created_at <= $3
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, agentID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions by date range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var transactions []*models.CommissionTransaction
	for rows.Next() {
		tx := &models.CommissionTransaction{}
		err := rows.Scan(
			&tx.ID,
			&tx.CommissionID,
			&tx.TransactionID,
			&tx.AgentID,
			&tx.OriginalAmount,
			&tx.GrossAmount,
			&tx.CommissionAmount,
			&tx.CommissionRate,
			&tx.CommissionType,
			&tx.Status,
			&tx.Reference,
			&tx.Notes,
			&tx.CreatedAt,
			&tx.CreditedAt,
			&tx.ReversedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		// Set UpdatedAt to CreatedAt since it's not in the database
		tx.UpdatedAt = tx.CreatedAt
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// GetCalculationsByDateRange retrieves commission calculations within a date range with pagination
func (r *commissionRepository) GetCalculationsByDateRange(ctx context.Context, agentID uuid.UUID, startDate, endDate time.Time, limit, offset int) ([]*models.CommissionCalculation, int, error) {
	// First get total count
	countQuery := `
		SELECT COUNT(*) 
		FROM commission_calculations 
		WHERE agent_id = $1 
		AND calculated_at >= $2 
		AND calculated_at <= $3
	`

	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, agentID, startDate, endDate).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %w", err)
	}

	// Get calculations with pagination
	query := `
		SELECT 
			id, agent_id, transaction_id, transaction_type, input_amount,
			rate_basis_points, commission_amount, gross_amount, net_amount,
			calculated_at, created_at, updated_at
		FROM commission_calculations
		WHERE agent_id = $1
		AND calculated_at >= $2
		AND calculated_at <= $3
		ORDER BY calculated_at DESC
		LIMIT $4 OFFSET $5
	`

	rows, err := r.db.QueryContext(ctx, query, agentID, startDate, endDate, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query calculations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var calculations []*models.CommissionCalculation
	for rows.Next() {
		calc := &models.CommissionCalculation{}
		err := rows.Scan(
			&calc.ID,
			&calc.AgentID,
			&calc.TransactionID,
			&calc.TransactionType,
			&calc.InputAmount,
			&calc.RateBasisPoints,
			&calc.CommissionAmount,
			&calc.GrossAmount,
			&calc.NetAmount,
			&calc.CalculatedAt,
			&calc.CreatedAt,
			&calc.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan calculation: %w", err)
		}
		calculations = append(calculations, calc)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	return calculations, totalCount, nil
}

// GetCalculationByTransactionID retrieves a commission calculation by transaction ID
func (r *commissionRepository) GetCalculationByTransactionID(ctx context.Context, transactionID uuid.UUID) (*models.CommissionCalculation, error) {
	query := `
		SELECT
			id, agent_id, transaction_id, transaction_type, input_amount,
			rate_basis_points, commission_amount, gross_amount, net_amount,
			calculated_at, created_at, updated_at
		FROM commission_calculations
		WHERE transaction_id = $1
		LIMIT 1
	`

	calc := &models.CommissionCalculation{}
	err := r.db.QueryRowContext(ctx, query, transactionID).Scan(
		&calc.ID,
		&calc.AgentID,
		&calc.TransactionID,
		&calc.TransactionType,
		&calc.InputAmount,
		&calc.RateBasisPoints,
		&calc.CommissionAmount,
		&calc.GrossAmount,
		&calc.NetAmount,
		&calc.CalculatedAt,
		&calc.CreatedAt,
		&calc.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return calc, nil
}

// GetDailyCommissions retrieves total commission amounts for a specific date and the previous day
func (r *commissionRepository) GetDailyCommissions(ctx context.Context, date time.Time) (*DailyCommissions, error) {
	// Calculate previous day
	previousDate := date.AddDate(0, 0, -1)

	// Query to get commission amounts for both days from wallet_transactions
	query := `
		WITH today_commissions AS (
			SELECT COALESCE(SUM(amount), 0) as today_amount
			FROM wallet_transactions
			WHERE DATE(created_at) = $1::date
			AND transaction_type = 'COMMISSION'
			AND status = 'COMPLETED'
		),
		yesterday_commissions AS (
			SELECT COALESCE(SUM(amount), 0) as yesterday_amount
			FROM wallet_transactions
			WHERE DATE(created_at) = $2::date
			AND transaction_type = 'COMMISSION'
			AND status = 'COMPLETED'
		)
		SELECT
			t.today_amount,
			y.yesterday_amount
		FROM today_commissions t, yesterday_commissions y
	`

	result := &DailyCommissions{}
	err := r.db.QueryRowContext(ctx, query, date, previousDate).Scan(
		&result.TodayAmount,
		&result.YesterdayAmount,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get daily commissions: %w", err)
	}

	return result, nil
}

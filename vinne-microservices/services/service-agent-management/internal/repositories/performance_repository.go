package repositories

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
)

// PerformanceRepository defines the interface for performance tracking operations
type PerformanceRepository interface {
	// Agent performance
	CreateAgentPerformance(ctx context.Context, performance *models.AgentPerformance) error
	GetAgentPerformance(ctx context.Context, agentID uuid.UUID, year, month int) (*models.AgentPerformance, error)
	GetAgentPerformanceRange(ctx context.Context, agentID uuid.UUID, fromYear, fromMonth, toYear, toMonth int) ([]models.AgentPerformance, error)
	UpdateAgentPerformance(ctx context.Context, performance *models.AgentPerformance) error

	// Retailer performance
	CreateRetailerPerformance(ctx context.Context, performance *models.RetailerPerformance) error
	GetRetailerPerformance(ctx context.Context, retailerID uuid.UUID, year, month int) (*models.RetailerPerformance, error)
	GetRetailerPerformanceRange(ctx context.Context, retailerID uuid.UUID, fromYear, fromMonth, toYear, toMonth int) ([]models.RetailerPerformance, error)
	UpdateRetailerPerformance(ctx context.Context, performance *models.RetailerPerformance) error
}

type performanceRepository struct {
	db *sqlx.DB
}

// NewPerformanceRepository creates a new performance repository
func NewPerformanceRepository(db *sqlx.DB) PerformanceRepository {
	return &performanceRepository{db: db}
}

// Agent performance methods
func (r *performanceRepository) CreateAgentPerformance(ctx context.Context, performance *models.AgentPerformance) error {
	if performance.ID == uuid.Nil {
		performance.ID = uuid.New()
	}

	performance.CreatedAt = time.Now()
	calculatedAt := time.Now()
	performance.CalculatedAt = &calculatedAt

	query := `
		INSERT INTO agent_performance (
			id, agent_id, period_year, period_month, total_retailers_active,
			total_retailers_inactive, total_sales_amount, total_commission_earned,
			total_transactions, calculated_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)`

	_, err := r.db.ExecContext(ctx, query,
		performance.ID, performance.AgentID, performance.PeriodYear, performance.PeriodMonth,
		performance.TotalRetailersActive, performance.TotalRetailersInactive,
		performance.TotalSalesAmount, performance.TotalCommissionEarned,
		performance.TotalTransactions, performance.CalculatedAt,
		performance.CreatedAt,
	)

	return err
}

func (r *performanceRepository) GetAgentPerformance(ctx context.Context, agentID uuid.UUID, year, month int) (*models.AgentPerformance, error) {
	var performance models.AgentPerformance
	query := `
		SELECT id, agent_id, period_year, period_month, total_retailers_active,
		       total_retailers_inactive, total_sales_amount, total_commission_earned,
		       total_transactions, calculated_at, created_at
		FROM agent_performance
		WHERE agent_id = $1 AND period_year = $2 AND period_month = $3`

	err := r.db.QueryRowContext(ctx, query, agentID, year, month).Scan(
		&performance.ID, &performance.AgentID, &performance.PeriodYear, &performance.PeriodMonth,
		&performance.TotalRetailersActive, &performance.TotalRetailersInactive,
		&performance.TotalSalesAmount, &performance.TotalCommissionEarned,
		&performance.TotalTransactions, &performance.CalculatedAt,
		&performance.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &performance, nil
}

func (r *performanceRepository) GetAgentPerformanceRange(ctx context.Context, agentID uuid.UUID, fromYear, fromMonth, toYear, toMonth int) ([]models.AgentPerformance, error) {
	query := `
		SELECT id, agent_id, period_year, period_month, total_retailers_active,
		       total_retailers_inactive, total_sales_amount, total_commission_earned,
		       total_transactions, calculated_at, created_at
		FROM agent_performance
		WHERE agent_id = $1 
		  AND ((period_year = $2 AND period_month >= $3) OR (period_year > $2))
		  AND ((period_year = $4 AND period_month <= $5) OR (period_year < $4))
		ORDER BY period_year, period_month`

	rows, err := r.db.QueryContext(ctx, query, agentID, fromYear, fromMonth, toYear, toMonth)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var performances []models.AgentPerformance
	for rows.Next() {
		var performance models.AgentPerformance
		err := rows.Scan(
			&performance.ID, &performance.AgentID, &performance.PeriodYear, &performance.PeriodMonth,
			&performance.TotalRetailersActive, &performance.TotalRetailersInactive,
			&performance.TotalSalesAmount, &performance.TotalCommissionEarned,
			&performance.TotalTransactions, &performance.CalculatedAt,
			&performance.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		performances = append(performances, performance)
	}

	return performances, nil
}

func (r *performanceRepository) UpdateAgentPerformance(ctx context.Context, performance *models.AgentPerformance) error {
	calculatedAt := time.Now()
	performance.CalculatedAt = &calculatedAt

	query := `
		UPDATE agent_performance SET
			total_retailers_active = $2,
			total_retailers_inactive = $3,
			total_sales_amount = $4,
			total_commission_earned = $5,
			total_transactions = $6,
			calculated_at = $7
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		performance.ID,
		performance.TotalRetailersActive,
		performance.TotalRetailersInactive,
		performance.TotalSalesAmount,
		performance.TotalCommissionEarned,
		performance.TotalTransactions,
		performance.CalculatedAt,
	)

	return err
}

// Retailer performance methods
func (r *performanceRepository) CreateRetailerPerformance(ctx context.Context, performance *models.RetailerPerformance) error {
	if performance.ID == uuid.Nil {
		performance.ID = uuid.New()
	}

	performance.CreatedAt = time.Now()
	calculatedAt := time.Now()
	performance.CalculatedAt = &calculatedAt

	query := `
		INSERT INTO retailer_performance (
			id, retailer_id, period_year, period_month, total_sales_amount,
			total_commission_earned, total_transactions, avg_transaction_value,
			calculated_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)`

	_, err := r.db.ExecContext(ctx, query,
		performance.ID, performance.RetailerID, performance.PeriodYear, performance.PeriodMonth,
		performance.TotalSalesAmount, performance.TotalCommissionEarned,
		performance.TotalTransactions, performance.AvgTransactionValue,
		performance.CalculatedAt, performance.CreatedAt,
	)

	return err
}

func (r *performanceRepository) GetRetailerPerformance(ctx context.Context, retailerID uuid.UUID, year, month int) (*models.RetailerPerformance, error) {
	var performance models.RetailerPerformance
	query := `
		SELECT id, retailer_id, period_year, period_month, total_sales_amount,
		       total_commission_earned, total_transactions, avg_transaction_value,
		       calculated_at, created_at
		FROM retailer_performance
		WHERE retailer_id = $1 AND period_year = $2 AND period_month = $3`

	err := r.db.QueryRowContext(ctx, query, retailerID, year, month).Scan(
		&performance.ID, &performance.RetailerID, &performance.PeriodYear, &performance.PeriodMonth,
		&performance.TotalSalesAmount, &performance.TotalCommissionEarned,
		&performance.TotalTransactions, &performance.AvgTransactionValue,
		&performance.CalculatedAt, &performance.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &performance, nil
}

func (r *performanceRepository) GetRetailerPerformanceRange(ctx context.Context, retailerID uuid.UUID, fromYear, fromMonth, toYear, toMonth int) ([]models.RetailerPerformance, error) {
	query := `
		SELECT id, retailer_id, period_year, period_month, total_sales_amount,
		       total_commission_earned, total_transactions, avg_transaction_value,
		       calculated_at, created_at
		FROM retailer_performance
		WHERE retailer_id = $1 
		  AND ((period_year = $2 AND period_month >= $3) OR (period_year > $2))
		  AND ((period_year = $4 AND period_month <= $5) OR (period_year < $4))
		ORDER BY period_year, period_month`

	rows, err := r.db.QueryContext(ctx, query, retailerID, fromYear, fromMonth, toYear, toMonth)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var performances []models.RetailerPerformance
	for rows.Next() {
		var performance models.RetailerPerformance
		err := rows.Scan(
			&performance.ID, &performance.RetailerID, &performance.PeriodYear, &performance.PeriodMonth,
			&performance.TotalSalesAmount, &performance.TotalCommissionEarned,
			&performance.TotalTransactions, &performance.AvgTransactionValue,
			&performance.CalculatedAt, &performance.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		performances = append(performances, performance)
	}

	return performances, nil
}

func (r *performanceRepository) UpdateRetailerPerformance(ctx context.Context, performance *models.RetailerPerformance) error {
	calculatedAt := time.Now()
	performance.CalculatedAt = &calculatedAt

	query := `
		UPDATE retailer_performance SET
			total_sales_amount = $2,
			total_commission_earned = $3,
			total_transactions = $4,
			avg_transaction_value = $5,
			calculated_at = $6
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		performance.ID,
		performance.TotalSalesAmount,
		performance.TotalCommissionEarned,
		performance.TotalTransactions,
		performance.AvgTransactionValue,
		performance.CalculatedAt,
	)

	return err
}

// Summary and aggregation methods - returning empty until we define summary models
func (r *performanceRepository) GetTopPerformingAgents(ctx context.Context, limit int, year, month int) ([]models.AgentPerformanceSummary, error) {
	// Placeholder until AgentPerformanceSummary model is defined
	return nil, nil
}

func (r *performanceRepository) GetTopPerformingRetailers(ctx context.Context, limit int, year, month int) ([]models.RetailerPerformanceSummary, error) {
	// Placeholder until RetailerPerformanceSummary model is defined
	return nil, nil
}

func (r *performanceRepository) GetAgentPerformanceSummary(ctx context.Context, agentID uuid.UUID, year int) (*models.AgentPerformanceSummary, error) {
	// Placeholder until AgentPerformanceSummary model is defined
	return nil, nil
}

func (r *performanceRepository) GetRetailerPerformanceSummary(ctx context.Context, retailerID uuid.UUID, year int) (*models.RetailerPerformanceSummary, error) {
	// Placeholder until RetailerPerformanceSummary model is defined
	return nil, nil
}

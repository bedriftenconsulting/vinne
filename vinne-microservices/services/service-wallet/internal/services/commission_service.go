package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/service-wallet/internal/models"
	"github.com/randco/service-wallet/internal/repositories"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// CommissionService defines the interface for commission calculations and management
type CommissionService interface {
	// Commission rate management
	SetAgentCommissionRate(ctx context.Context, agentID uuid.UUID, rateBasisPoints int32, notes string) error
	GetAgentCommissionRate(ctx context.Context, agentID uuid.UUID) (*models.AgentCommissionRate, error)

	// Commission calculations
	CalculateAgentCommission(ctx context.Context, agentID uuid.UUID, amount int64) (commission int64, grossAmount int64, err error)
	CalculateTransferCommission(ctx context.Context, agentID uuid.UUID, amount int64) (commission int64, grossAmount int64, err error)

	// Commission recording
	RecordCommissionTransaction(ctx context.Context, agentID uuid.UUID, originalAmount, grossAmount, commissionAmount int64, commissionRate int32, commissionType models.CommissionType, transactionID uuid.UUID) error

	// Commission reporting
	GetCommissionReport(ctx context.Context, agentID uuid.UUID, startDate, endDate time.Time, limit, offset int) ([]*models.CommissionCalculation, int64, int, error)
	GetCommissionHistory(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*models.CommissionTransaction, int, error)
	GetTransactionCommission(ctx context.Context, transactionID uuid.UUID) (commission int64, grossAmount int64, err error)

	// Daily metrics
	GetDailyCommissions(ctx context.Context, date string, includeComparison bool) (*DailyCommissionsResult, error)
}

// DailyCommissionsResult holds the result of daily commission metrics with change tracking
type DailyCommissionsResult struct {
	Date             string
	Amount           int64
	ChangePercentage float64
	PreviousAmount   int64
}

type commissionService struct {
	db             *sql.DB
	cache          *redis.Client
	commissionRepo repositories.CommissionRepository
	tracer         trace.Tracer
}

// NewCommissionService creates a new instance of CommissionService
func NewCommissionService(
	db *sql.DB,
	cache *redis.Client,
	commissionRepo repositories.CommissionRepository,
) CommissionService {
	return &commissionService{
		db:             db,
		cache:          cache,
		commissionRepo: commissionRepo,
		tracer:         otel.Tracer("commission-service"),
	}
}

// SetAgentCommissionRate sets the commission rate for an agent
func (s *commissionService) SetAgentCommissionRate(ctx context.Context, agentID uuid.UUID, rateBasisPoints int32, notes string) error {
	ctx, span := s.tracer.Start(ctx, "SetAgentCommissionRate")
	defer span.End()

	span.SetAttributes(
		attribute.String("agent.id", agentID.String()),
		attribute.Int("rate.basis_points", int(rateBasisPoints)),
		attribute.Float64("rate.percentage", float64(rateBasisPoints)/100),
	)

	// Validate rate (0-10000 basis points = 0-100%)
	if rateBasisPoints < 0 || rateBasisPoints > 10000 {
		return fmt.Errorf("invalid commission rate: must be between 0 and 10000 basis points")
	}

	// Check if rate already exists
	existingRate, err := s.commissionRepo.GetByAgentID(ctx, agentID)
	if err != nil && err != sql.ErrNoRows {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to check existing rate")
		return fmt.Errorf("failed to check existing rate: %w", err)
	}

	if existingRate != nil {
		// Update existing rate
		existingRate.Rate = rateBasisPoints
		existingRate.UpdatedAt = time.Now()

		if err := s.commissionRepo.Update(ctx, existingRate); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to update commission rate")
			return fmt.Errorf("failed to update commission rate: %w", err)
		}
	} else {
		// Create new rate
		rate := &models.AgentCommissionRate{
			ID:            uuid.New(),
			AgentID:       agentID,
			Rate:          rateBasisPoints,
			EffectiveFrom: time.Now(),
			CreatedBy:     uuid.New(), // System user ID
			Notes:         &notes,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		if err := s.commissionRepo.Create(ctx, rate); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create commission rate")
			return fmt.Errorf("failed to create commission rate: %w", err)
		}
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("commission:agent:%s:rate", agentID.String())
	s.cache.Del(ctx, cacheKey)

	span.SetStatus(codes.Ok, "commission rate set successfully")
	return nil
}

// GetAgentCommissionRate retrieves the commission rate for an agent
func (s *commissionService) GetAgentCommissionRate(ctx context.Context, agentID uuid.UUID) (*models.AgentCommissionRate, error) {
	ctx, span := s.tracer.Start(ctx, "GetAgentCommissionRate")
	defer span.End()

	span.SetAttributes(
		attribute.String("agent.id", agentID.String()),
	)

	// Try cache first
	cacheKey := fmt.Sprintf("commission:agent:%s:rate", agentID.String())
	cached, err := s.cache.Get(ctx, cacheKey).Int()
	if err == nil {
		span.SetAttributes(
			attribute.Bool("cache.hit", true),
			attribute.Int("rate.basis_points", cached),
		)
		// Return a default rate object when using cache
		return &models.AgentCommissionRate{
			ID:            uuid.New(),
			AgentID:       agentID,
			Rate:          int32(cached),
			EffectiveFrom: time.Now(),
			CreatedAt:     time.Now(),
		}, nil
	}

	// Get from database
	rate, err := s.commissionRepo.GetByAgentID(ctx, agentID)
	if err == sql.ErrNoRows {
		// Return default rate of 30% (3000 basis points)
		defaultRate := &models.AgentCommissionRate{
			ID:            uuid.New(),
			AgentID:       agentID,
			Rate:          3000,
			EffectiveFrom: time.Now(),
			CreatedBy:     uuid.New(),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		span.SetAttributes(
			attribute.Bool("using.default", true),
			attribute.Int("rate.basis_points", int(defaultRate.Rate)),
		)

		// Cache the default rate
		s.cache.Set(ctx, cacheKey, defaultRate.Rate, 5*time.Minute)
		return defaultRate, nil
	} else if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get commission rate")
		return nil, fmt.Errorf("failed to get commission rate: %w", err)
	}

	span.SetAttributes(
		attribute.Bool("cache.hit", false),
		attribute.Int("rate.basis_points", int(rate.Rate)),
	)

	// Cache the rate
	s.cache.Set(ctx, cacheKey, rate.Rate, 5*time.Minute)

	span.SetStatus(codes.Ok, "commission rate retrieved successfully")
	return rate, nil
}

// CalculateAgentCommission calculates commission for agent deposits using gross-up method
// Formula: Net Amount = 70% of Gross Amount, therefore Gross Amount = Net Amount / 0.7
// The base amount (netAmount) represents 70% of the final grossed-up amount
func (s *commissionService) CalculateAgentCommission(ctx context.Context, agentID uuid.UUID, netAmount int64) (commission int64, grossAmount int64, err error) {
	ctx, span := s.tracer.Start(ctx, "CalculateAgentCommission")
	defer span.End()

	span.SetAttributes(
		attribute.String("agent.id", agentID.String()),
		attribute.Int64("net.amount.pesewas", netAmount),
	)

	// Get commission rate
	rateObj, err := s.GetAgentCommissionRate(ctx, agentID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get commission rate")
		return 0, 0, fmt.Errorf("failed to get commission rate: %w", err)
	}
	rateBasisPoints := rateObj.Rate

	// Calculate gross amount where base amount is 70% of final
	// Formula: Gross Amount = Net Amount / (1 - rate)
	// For 30% commission (3000 basis points): Gross = Net / 0.7
	// Commission = Gross Amount - Net Amount
	rateDecimal := float64(rateBasisPoints) / 10000.0 // Convert basis points to decimal (e.g., 3000 -> 0.30)
	grossAmountFloat := float64(netAmount) / (1 - rateDecimal)
	grossAmount = int64(grossAmountFloat)
	commission = grossAmount - netAmount

	span.SetAttributes(
		attribute.Int("rate.basis_points", int(rateBasisPoints)),
		attribute.Float64("rate.decimal", rateDecimal),
		attribute.Int64("commission.pesewas", commission),
		attribute.Int64("gross.amount.pesewas", grossAmount),
	)

	// Record the calculation in audit table
	calculation := &models.CommissionCalculation{
		ID:               uuid.New(),
		AgentID:          agentID,
		TransactionType:  models.CommissionTypeDeposit,
		InputAmount:      netAmount,
		NetAmount:        netAmount,
		RateBasisPoints:  rateBasisPoints,
		CommissionAmount: commission,
		GrossAmount:      grossAmount,
		CalculatedAt:     time.Now(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.commissionRepo.RecordCalculation(ctx, calculation); err != nil {
		// Log error but don't fail the calculation
		span.RecordError(err)
	}

	span.SetStatus(codes.Ok, "commission calculated successfully")
	return commission, grossAmount, nil
}

// CalculateTransferCommission calculates commission for agent-to-retailer transfers
func (s *commissionService) CalculateTransferCommission(ctx context.Context, agentID uuid.UUID, netAmount int64) (commission int64, grossAmount int64, err error) {
	ctx, span := s.tracer.Start(ctx, "CalculateTransferCommission")
	defer span.End()

	span.SetAttributes(
		attribute.String("agent.id", agentID.String()),
		attribute.Int64("net.amount.pesewas", netAmount),
	)

	// Get commission rate
	rateObj, err := s.GetAgentCommissionRate(ctx, agentID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get commission rate")
		return 0, 0, fmt.Errorf("failed to get commission rate: %w", err)
	}
	rateBasisPoints := rateObj.Rate

	// Calculate gross amount for transfers using same formula as deposits
	// Base amount is 70% of final: Gross Amount = Net Amount / (1 - rate)
	rateDecimal := float64(rateBasisPoints) / 10000.0
	grossAmountFloat := float64(netAmount) / (1 - rateDecimal)
	grossAmount = int64(grossAmountFloat)
	commission = grossAmount - netAmount

	span.SetAttributes(
		attribute.Int("rate.basis_points", int(rateBasisPoints)),
		attribute.Float64("rate.decimal", rateDecimal),
		attribute.Int64("commission.pesewas", commission),
		attribute.Int64("gross.amount.pesewas", grossAmount),
	)

	// Record the calculation
	calculation := &models.CommissionCalculation{
		ID:               uuid.New(),
		AgentID:          agentID,
		TransactionType:  models.CommissionTypeTransfer,
		InputAmount:      netAmount,
		NetAmount:        netAmount,
		RateBasisPoints:  rateBasisPoints,
		CommissionAmount: commission,
		GrossAmount:      grossAmount,
		CalculatedAt:     time.Now(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.commissionRepo.RecordCalculation(ctx, calculation); err != nil {
		// Log error but don't fail the calculation
		span.RecordError(err)
	}

	span.SetStatus(codes.Ok, "transfer commission calculated successfully")
	return commission, grossAmount, nil
}

// RecordCommissionTransaction records a commission transaction for audit purposes
func (s *commissionService) RecordCommissionTransaction(ctx context.Context, agentID uuid.UUID, originalAmount, grossAmount, commissionAmount int64, commissionRate int32, commissionType models.CommissionType, transactionID uuid.UUID) error {
	ctx, span := s.tracer.Start(ctx, "RecordCommissionTransaction")
	defer span.End()

	span.SetAttributes(
		attribute.String("agent.id", agentID.String()),
		attribute.Int64("original.amount.pesewas", originalAmount),
		attribute.Int64("gross.amount.pesewas", grossAmount),
		attribute.Int64("commission.amount.pesewas", commissionAmount),
		attribute.Int("commission.rate", int(commissionRate)),
		attribute.String("commission.type", string(commissionType)),
		attribute.String("transaction.id", transactionID.String()),
	)

	// Generate commission ID in format: COM-YYYYMMDD-HHMMSS-{short-uuid}
	now := time.Now()
	shortUUID := uuid.New().String()[:8]
	commissionID := fmt.Sprintf("COM-%s-%s", now.Format("20060102-150405"), shortUUID)

	transaction := &models.CommissionTransaction{
		ID:               uuid.New(),
		CommissionID:     commissionID,
		AgentID:          agentID,
		TransactionID:    transactionID,
		OriginalAmount:   originalAmount,
		GrossAmount:      grossAmount,
		CommissionAmount: commissionAmount,
		CommissionRate:   commissionRate,
		CommissionType:   commissionType,
		Status:           models.CommissionStatusCredited,
		CreditedAt:       &[]time.Time{now}[0],
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.commissionRepo.RecordTransaction(ctx, transaction); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to record commission transaction")
		return fmt.Errorf("failed to record commission transaction: %w", err)
	}

	// Update commission audit
	audit := &models.CommissionAudit{
		ID:               uuid.New(),
		AgentID:          agentID,
		TransactionID:    transactionID,
		CommissionAmount: commissionAmount,
		Action:           "COMMISSION_CREDITED",
		ActionBy:         "SYSTEM",
		ActionAt:         time.Now(),
		Details:          fmt.Sprintf("Commission of %d pesewas credited for transaction %s", commissionAmount, transactionID.String()),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.commissionRepo.RecordAudit(ctx, audit); err != nil {
		// Log error but don't fail the operation
		span.RecordError(err)
	}

	span.SetStatus(codes.Ok, "commission transaction recorded successfully")
	return nil
}

// GetCommissionReport generates a commission report for an agent
func (s *commissionService) GetCommissionReport(ctx context.Context, agentID uuid.UUID, startDate, endDate time.Time, limit, offset int) ([]*models.CommissionCalculation, int64, int, error) {
	ctx, span := s.tracer.Start(ctx, "GetCommissionReport")
	defer span.End()

	span.SetAttributes(
		attribute.String("agent.id", agentID.String()),
		attribute.String("start.date", startDate.Format(time.RFC3339)),
		attribute.String("end.date", endDate.Format(time.RFC3339)),
	)

	// Get commission calculations for the period with pagination
	calculations, total, err := s.commissionRepo.GetCalculationsByDateRange(ctx, agentID, startDate, endDate, limit, offset)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get commission calculations")
		return nil, 0, 0, fmt.Errorf("failed to get commission calculations: %w", err)
	}

	// Calculate total commission for the period
	var totalCommission int64
	for _, calc := range calculations {
		totalCommission += calc.CommissionAmount
	}

	span.SetAttributes(
		attribute.Int64("total.commission.pesewas", totalCommission),
		attribute.Int("calculations.count", len(calculations)),
		attribute.Int("calculations.total", total),
	)

	span.SetStatus(codes.Ok, "commission report generated successfully")
	return calculations, totalCommission, total, nil
}

// GetCommissionHistory retrieves commission transaction history for an agent
func (s *commissionService) GetCommissionHistory(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*models.CommissionTransaction, int, error) {
	ctx, span := s.tracer.Start(ctx, "GetCommissionHistory")
	defer span.End()

	span.SetAttributes(
		attribute.String("agent.id", agentID.String()),
		attribute.Int("limit", limit),
		attribute.Int("offset", offset),
	)

	transactions, total, err := s.commissionRepo.GetTransactionHistory(ctx, agentID, limit, offset)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get commission history")
		return nil, 0, fmt.Errorf("failed to get commission history: %w", err)
	}

	span.SetAttributes(
		attribute.Int("transactions.count", len(transactions)),
		attribute.Int("transactions.total", total),
	)

	span.SetStatus(codes.Ok, "commission history retrieved successfully")
	return transactions, total, nil
}

// GetTransactionCommission retrieves commission details for a specific transaction
func (s *commissionService) GetTransactionCommission(ctx context.Context, transactionID uuid.UUID) (commission int64, grossAmount int64, err error) {
	ctx, span := s.tracer.Start(ctx, "GetTransactionCommission")
	defer span.End()

	span.SetAttributes(
		attribute.String("transaction.id", transactionID.String()),
	)

	calculation, err := s.commissionRepo.GetCalculationByTransactionID(ctx, transactionID)
	if err != nil {
		if err == sql.ErrNoRows {
			// No commission for this transaction
			return 0, 0, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get commission calculation")
		return 0, 0, fmt.Errorf("failed to get commission calculation: %w", err)
	}

	span.SetStatus(codes.Ok, "commission details retrieved successfully")
	return calculation.CommissionAmount, calculation.GrossAmount, nil
}

// GetCommissionByTransactionID retrieves commission calculation by transaction ID
func (s *commissionService) GetCommissionByTransactionID(ctx context.Context, transactionID uuid.UUID) (*models.CommissionCalculation, error) {
	ctx, span := s.tracer.Start(ctx, "GetCommissionByTransactionID")
	defer span.End()

	span.SetAttributes(
		attribute.String("transaction.id", transactionID.String()),
	)

	// Try to get from cache first - removed for now

	calculation, err := s.commissionRepo.GetCalculationByTransactionID(ctx, transactionID)
	if err != nil {
		if err == sql.ErrNoRows {
			// No commission for this transaction (might be a non-commission transaction)
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get commission calculation")
		return nil, fmt.Errorf("failed to get commission calculation: %w", err)
	}

	span.SetStatus(codes.Ok, "commission calculation retrieved successfully")
	return calculation, nil
}

// GetDailyCommissions retrieves daily commission metrics with change tracking
func (s *commissionService) GetDailyCommissions(ctx context.Context, date string, includeComparison bool) (*DailyCommissionsResult, error) {
	ctx, span := s.tracer.Start(ctx, "GetDailyCommissions")
	defer span.End()

	span.SetAttributes(
		attribute.String("date", date),
		attribute.Bool("include_comparison", includeComparison),
	)

	// Parse date or use today if empty
	var targetDate time.Time
	var err error
	if date == "" {
		targetDate = time.Now()
	} else {
		targetDate, err = time.Parse("2006-01-02", date)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "invalid date format")
			return nil, fmt.Errorf("invalid date format: %w", err)
		}
	}

	// Get daily commissions from repository
	commissions, err := s.commissionRepo.GetDailyCommissions(ctx, targetDate)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get daily commissions")
		return nil, fmt.Errorf("failed to get daily commissions: %w", err)
	}

	// Calculate percentage change
	var changePercentage float64
	if includeComparison && commissions.YesterdayAmount > 0 {
		changePercentage = ((float64(commissions.TodayAmount) - float64(commissions.YesterdayAmount)) / float64(commissions.YesterdayAmount)) * 100
	}

	result := &DailyCommissionsResult{
		Date:             targetDate.Format("2006-01-02"),
		Amount:           commissions.TodayAmount,
		ChangePercentage: changePercentage,
		PreviousAmount:   commissions.YesterdayAmount,
	}

	span.SetAttributes(
		attribute.Int64("commission.today", commissions.TodayAmount),
		attribute.Int64("commission.yesterday", commissions.YesterdayAmount),
		attribute.Float64("commission.change", changePercentage),
	)

	span.SetStatus(codes.Ok, "daily commissions retrieved successfully")
	return result, nil
}

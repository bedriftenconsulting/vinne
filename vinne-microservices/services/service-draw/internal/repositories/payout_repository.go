package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/randco/service-draw/internal/models"
)

// CreatePayoutRecord creates a single payout record
func (r *drawRepository) CreatePayoutRecord(ctx context.Context, record *models.DrawPayoutRecord) error {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.CreatePayoutRecord")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw.id", record.DrawID.String()),
		attribute.String("ticket.id", record.TicketID.String()),
		attribute.Int64("winning.amount", record.WinningAmount),
	)

	query := `
		INSERT INTO draw_payout_records (
			id, draw_id, ticket_id, serial_number, retailer_id, winning_amount,
			status, payout_type, requires_approval, idempotency_key,
			attempt_count, created_at, updated_at
		) VALUES (
			:id, :draw_id, :ticket_id, :serial_number, :retailer_id, :winning_amount,
			:status, :payout_type, :requires_approval, :idempotency_key,
			:attempt_count, :created_at, :updated_at
		)
		ON CONFLICT (idempotency_key) DO NOTHING`

	_, err := r.db.NamedExecContext(ctx, query, record)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return fmt.Errorf("failed to create payout record: %w", err)
	}

	return nil
}

// GetPayoutRecord retrieves a single payout record by draw_id and ticket_id
func (r *drawRepository) GetPayoutRecord(ctx context.Context, drawID, ticketID uuid.UUID) (*models.DrawPayoutRecord, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.GetPayoutRecord")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw.id", drawID.String()),
		attribute.String("ticket.id", ticketID.String()),
	)

	query := `
		SELECT id, draw_id, ticket_id, serial_number, retailer_id, winning_amount,
		       status, payout_type, requires_approval, wallet_transaction_id,
		       idempotency_key, attempt_count, last_attempt_at, last_error,
		       wallet_credited_at, ticket_marked_paid_at, completed_at,
		       approved_by, approved_at, rejection_reason,
		       created_at, updated_at
		FROM draw_payout_records
		WHERE draw_id = $1 AND ticket_id = $2`

	var record models.DrawPayoutRecord
	err := r.db.GetContext(ctx, &record, query, drawID, ticketID)
	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.String("result", "not_found"))
			return nil, fmt.Errorf("payout record not found")
		}
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get payout record: %w", err)
	}

	return &record, nil
}

// UpdatePayoutRecord updates a payout record
func (r *drawRepository) UpdatePayoutRecord(ctx context.Context, record *models.DrawPayoutRecord) error {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.UpdatePayoutRecord")
	defer span.End()

	span.SetAttributes(
		attribute.String("record.id", record.ID.String()),
		attribute.String("status", string(record.Status)),
	)

	query := `
		UPDATE draw_payout_records
		SET status = :status,
		    wallet_transaction_id = :wallet_transaction_id,
		    attempt_count = :attempt_count,
		    last_attempt_at = :last_attempt_at,
		    last_error = :last_error,
		    wallet_credited_at = :wallet_credited_at,
		    ticket_marked_paid_at = :ticket_marked_paid_at,
		    completed_at = :completed_at,
		    approved_by = :approved_by,
		    approved_at = :approved_at,
		    rejection_reason = :rejection_reason,
		    updated_at = :updated_at
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, record)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return fmt.Errorf("failed to update payout record: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		span.SetAttributes(attribute.String("result", "not_found"))
		return fmt.Errorf("payout record not found")
	}

	return nil
}

// ListPayoutRecords lists payout records with optional status filter
func (r *drawRepository) ListPayoutRecords(ctx context.Context, drawID uuid.UUID, status *models.PayoutStatus, limit, offset int) ([]*models.DrawPayoutRecord, int64, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.ListPayoutRecords")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw.id", drawID.String()),
		attribute.Int("limit", limit),
		attribute.Int("offset", offset),
	)

	// Build query with optional status filter
	whereClause := "WHERE draw_id = $1"
	args := []interface{}{drawID}
	if status != nil {
		whereClause += " AND status = $2"
		args = append(args, string(*status))
		span.SetAttributes(attribute.String("filter.status", string(*status)))
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM draw_payout_records " + whereClause
	var total int64
	err := r.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to count payout records: %w", err)
	}

	// Get paginated results
	query := `
		SELECT id, draw_id, ticket_id, serial_number, retailer_id, winning_amount,
		       status, payout_type, requires_approval, wallet_transaction_id,
		       idempotency_key, attempt_count, last_attempt_at, last_error,
		       wallet_credited_at, ticket_marked_paid_at, completed_at,
		       approved_by, approved_at, rejection_reason,
		       created_at, updated_at
		FROM draw_payout_records ` + whereClause + `
		ORDER BY created_at ASC
		LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)

	args = append(args, limit, offset)

	var records []*models.DrawPayoutRecord
	err = r.db.SelectContext(ctx, &records, query, args...)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to list payout records: %w", err)
	}

	span.SetAttributes(attribute.Int64("result.total", total))
	return records, total, nil
}

// CreatePayoutRecordsBatch creates multiple payout records in a batch (idempotent)
func (r *drawRepository) CreatePayoutRecordsBatch(ctx context.Context, records []*models.DrawPayoutRecord) error {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.CreatePayoutRecordsBatch")
	defer span.End()

	span.SetAttributes(attribute.Int("batch.size", len(records)))

	if len(records) == 0 {
		return nil
	}

	// Build bulk insert query with ON CONFLICT DO NOTHING for idempotency
	valueStrings := make([]string, 0, len(records))
	valueArgs := make([]interface{}, 0, len(records)*13)

	for i, record := range records {
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*13+1, i*13+2, i*13+3, i*13+4, i*13+5, i*13+6, i*13+7, i*13+8, i*13+9, i*13+10, i*13+11, i*13+12, i*13+13,
		))
		valueArgs = append(valueArgs,
			record.ID,
			record.DrawID,
			record.TicketID,
			record.SerialNumber,
			record.RetailerID,
			record.WinningAmount,
			string(record.Status),
			string(record.PayoutType),
			record.RequiresApproval,
			record.IdempotencyKey,
			record.AttemptCount,
			record.CreatedAt,
			record.UpdatedAt,
		)
	}

	query := `
		INSERT INTO draw_payout_records (
			id, draw_id, ticket_id, serial_number, retailer_id, winning_amount,
			status, payout_type, requires_approval, idempotency_key,
			attempt_count, created_at, updated_at
		) VALUES ` + strings.Join(valueStrings, ", ") + `
		ON CONFLICT (draw_id, ticket_id) DO NOTHING`

	_, err := r.db.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return fmt.Errorf("failed to batch create payout records: %w", err)
	}

	return nil
}

// GetPendingPayouts retrieves pending payouts for processing
func (r *drawRepository) GetPendingPayouts(ctx context.Context, drawID uuid.UUID, limit int) ([]*models.DrawPayoutRecord, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.GetPendingPayouts")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw.id", drawID.String()),
		attribute.Int("limit", limit),
	)

	query := `
		SELECT id, draw_id, ticket_id, serial_number, retailer_id, winning_amount,
		       status, payout_type, requires_approval, wallet_transaction_id,
		       idempotency_key, attempt_count, last_attempt_at, last_error,
		       wallet_credited_at, ticket_marked_paid_at, completed_at,
		       approved_by, approved_at, rejection_reason,
		       created_at, updated_at
		FROM draw_payout_records
		WHERE draw_id = $1
		  AND status = $2
		  AND requires_approval = false
		ORDER BY created_at ASC
		LIMIT $3`

	var records []*models.DrawPayoutRecord
	err := r.db.SelectContext(ctx, &records, query, drawID, string(models.PayoutStatusPending), limit)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get pending payouts: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(records)))
	return records, nil
}

// GetFailedPayouts retrieves failed payouts that can be retried
func (r *drawRepository) GetFailedPayouts(ctx context.Context, drawID uuid.UUID, maxAttempts int) ([]*models.DrawPayoutRecord, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.GetFailedPayouts")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw.id", drawID.String()),
		attribute.Int("max.attempts", maxAttempts),
	)

	query := `
		SELECT id, draw_id, ticket_id, serial_number, retailer_id, winning_amount,
		       status, payout_type, requires_approval, wallet_transaction_id,
		       idempotency_key, attempt_count, last_attempt_at, last_error,
		       wallet_credited_at, ticket_marked_paid_at, completed_at,
		       approved_by, approved_at, rejection_reason,
		       created_at, updated_at
		FROM draw_payout_records
		WHERE draw_id = $1
		  AND status = $2
		  AND attempt_count < $3
		ORDER BY last_attempt_at ASC`

	var records []*models.DrawPayoutRecord
	err := r.db.SelectContext(ctx, &records, query, drawID, string(models.PayoutStatusFailed), maxAttempts)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get failed payouts: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(records)))
	return records, nil
}

// GetPayoutSummary generates a summary of payout statistics for a draw
func (r *drawRepository) GetPayoutSummary(ctx context.Context, drawID uuid.UUID) (*models.PayoutSummary, error) {
	tracer := otel.Tracer("draw-service")
	ctx, span := tracer.Start(ctx, "DrawRepository.GetPayoutSummary")
	defer span.End()

	span.SetAttributes(attribute.String("draw.id", drawID.String()))

	query := `
		SELECT
			status,
			COUNT(*) as count,
			COALESCE(SUM(winning_amount), 0) as amount
		FROM draw_payout_records
		WHERE draw_id = $1
		GROUP BY status`

	type statusSummary struct {
		Status string `db:"status"`
		Count  int64  `db:"count"`
		Amount int64  `db:"amount"`
	}

	var summaries []statusSummary
	err := r.db.SelectContext(ctx, &summaries, query, drawID)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get payout summary: %w", err)
	}

	// Build summary
	summary := &models.PayoutSummary{
		DrawID: drawID,
	}

	for _, s := range summaries {
		switch models.PayoutStatus(s.Status) {
		case models.PayoutStatusPending:
			summary.PendingCount = s.Count
			summary.PendingAmount = s.Amount
		case models.PayoutStatusProcessing:
			summary.ProcessingCount = s.Count
			summary.ProcessingAmount = s.Amount
		case models.PayoutStatusCompleted:
			summary.CompletedCount = s.Count
			summary.CompletedAmount = s.Amount
		case models.PayoutStatusFailed:
			summary.FailedCount = s.Count
			summary.FailedAmount = s.Amount
		case models.PayoutStatusSkipped:
			summary.SkippedCount = s.Count
			summary.SkippedAmount = s.Amount
		}
	}

	// Get total from draw stage data for comparison
	draw, err := r.GetByID(ctx, drawID)
	if err == nil && draw.StageData != nil && draw.StageData.ResultCalculationData != nil {
		summary.TotalWinningTickets = draw.StageData.ResultCalculationData.WinningTicketsCount
		summary.TotalWinningAmount = draw.StageData.ResultCalculationData.TotalWinnings
	}

	span.SetAttributes(
		attribute.Int64("summary.pending", summary.PendingCount),
		attribute.Int64("summary.completed", summary.CompletedCount),
		attribute.Int64("summary.failed", summary.FailedCount),
		attribute.Int64("summary.skipped", summary.SkippedCount),
	)

	return summary, nil
}

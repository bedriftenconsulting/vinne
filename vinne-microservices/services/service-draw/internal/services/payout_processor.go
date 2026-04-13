package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	ticketv1 "github.com/randco/randco-microservices/proto/ticket/v1"
	walletv1 "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/service-draw/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// BigWinThreshold is the minimum winning amount (in pesewas) that requires manual approval
// 24,000 GHS = 2,400,000 pesewas (NLA regulatory requirement)
const BigWinThreshold = 2400000

// PayoutProcessor handles the robust payout processing workflow
type PayoutProcessor struct {
	drawService       *drawService
	batchSize         int
	concurrentWorkers int
}

// NewPayoutProcessor creates a new payout processor
func NewPayoutProcessor(ds *drawService) *PayoutProcessor {
	return &PayoutProcessor{
		drawService:       ds,
		batchSize:         100, // Process 100 tickets at a time
		concurrentWorkers: 5,   // 5 workers processing concurrently
	}
}

// ProcessPayouts processes all winning ticket payouts with full robustness
func (pp *PayoutProcessor) ProcessPayouts(ctx context.Context, drawID uuid.UUID, processedBy string) (*models.Draw, *PayoutSummary, error) {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "PayoutProcessor.ProcessPayouts")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw_id", drawID.String()),
		attribute.String("processed_by", processedBy),
	)

	// ============================================================
	// Step 1: Validate preconditions
	// ============================================================

	draw, err := pp.drawService.drawRepo.GetByID(ctx, drawID)
	if err != nil {
		span.RecordError(err)
		return nil, nil, fmt.Errorf("failed to get draw: %w", err)
	}

	// Validate stage progression
	// Allow Stage 3 (first-time processing after auto-transition) or Stage 4 (re-processing)
	if draw.StageData == nil || (draw.StageData.CurrentStage != 3 && draw.StageData.CurrentStage != 4) {
		return nil, nil, fmt.Errorf("draw must complete result calculation stage first, current stage: %d", draw.StageData.CurrentStage)
	}

	// If in Stage 3, verify it's completed before transitioning to Stage 4
	if draw.StageData.CurrentStage == 3 && draw.StageData.StageStatus != models.StageStatusCompleted {
		return nil, nil, fmt.Errorf("result calculation must be completed, current status: %s", draw.StageData.StageStatus)
	}

	// Check if already in Stage 4
	if draw.StageData.CurrentStage == 4 {
		pp.drawService.logger.Printf("Draw %s already in Stage 4, checking for incomplete payouts", drawID)
		// Allow re-processing to resume incomplete payouts
	} else {
		// Transition to Stage 4
		now := time.Now()
		draw.StageData.CurrentStage = 4
		draw.StageData.StageName = "Payout"
		draw.StageData.StageStatus = models.StageStatusInProgress
		draw.StageData.StageStartedAt = &now
		draw.StageData.StageCompletedAt = nil

		if err := pp.drawService.drawRepo.Update(ctx, draw); err != nil {
			span.RecordError(err)
			return nil, nil, fmt.Errorf("failed to transition to stage 4: %w", err)
		}
	}

	// ============================================================
	// Step 2: Initialize payout records (idempotent)
	// ============================================================

	err = pp.initializePayoutRecords(ctx, draw)
	if err != nil {
		span.RecordError(err)
		return nil, nil, fmt.Errorf("failed to initialize payout records: %w", err)
	}

	// ============================================================
	// Step 3: Process auto payouts (batch with concurrency)
	// ============================================================

	_, err = pp.processAutoPayouts(ctx, drawID)
	if err != nil {
		span.RecordError(err)
		pp.drawService.logger.Printf("Auto payout processing encountered errors: %v", err)
		// Don't fail - continue with summary
	}

	// ============================================================
	// Step 4: Get final summary
	// ============================================================

	summary, err := pp.drawService.drawRepo.GetPayoutSummary(ctx, drawID)
	if err != nil {
		span.RecordError(err)
		return nil, nil, fmt.Errorf("failed to get payout summary: %w", err)
	}

	// ============================================================
	// Step 5: Update stage data
	// ============================================================

	draw, err = pp.drawService.drawRepo.GetByID(ctx, drawID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to reload draw: %w", err)
	}

	draw.StageData.PayoutData = &models.PayoutStageData{
		AutoProcessedCount:   summary.CompletedCount,
		ManualApprovalCount:  summary.SkippedCount,
		AutoProcessedAmount:  summary.CompletedAmount,
		ManualApprovalAmount: summary.SkippedAmount,
		ProcessedCount:       summary.CompletedCount,
		PendingCount:         summary.PendingCount + summary.FailedCount,
	}

	// Mark stage as completed if no pending or processing items
	// Skipped tickets (big wins) and failed tickets don't block completion
	// Failed tickets will be retried, skipped tickets need manual approval
	if summary.PendingCount == 0 && summary.ProcessingCount == 0 {
		draw.StageData.StageStatus = models.StageStatusCompleted
		now := time.Now()
		draw.StageData.StageCompletedAt = &now

		// Mark the overall draw as completed when Stage 4 is completed
		draw.Status = models.DrawStatusCompleted

		pp.drawService.logger.Printf("STAGE4_STAGE_COMPLETED: draw_id=%s, completed=%d, skipped=%d, failed=%d",
			drawID, summary.CompletedCount, summary.SkippedCount, summary.FailedCount)
	} else {
		pp.drawService.logger.Printf("STAGE4_STAGE_IN_PROGRESS: draw_id=%s, pending=%d, processing=%d, completed=%d, skipped=%d, failed=%d",
			drawID, summary.PendingCount, summary.ProcessingCount, summary.CompletedCount, summary.SkippedCount, summary.FailedCount)
	}

	if err := pp.drawService.drawRepo.Update(ctx, draw); err != nil {
		span.RecordError(err)
		return nil, nil, fmt.Errorf("failed to update draw: %w", err)
	}

	pp.drawService.logger.Printf("Payout processing status: draw_id=%s, completed=%d, pending=%d, failed=%d, skipped=%d",
		drawID, summary.CompletedCount, summary.PendingCount, summary.FailedCount, summary.SkippedCount)

	responseSummary := &PayoutSummary{
		AutoProcessedCount:   summary.CompletedCount,
		ManualApprovalCount:  summary.SkippedCount,
		AutoProcessedAmount:  summary.CompletedAmount,
		ManualApprovalAmount: summary.SkippedAmount,
	}

	return draw, responseSummary, nil
}

// ============================================================
// Supporting functions
// ============================================================

// initializePayoutRecords creates payout records for all winning tickets (idempotent)
func (pp *PayoutProcessor) initializePayoutRecords(ctx context.Context, draw *models.Draw) error {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "PayoutProcessor.initializePayoutRecords")
	defer span.End()

	if draw.StageData == nil || draw.StageData.ResultCalculationData == nil {
		return fmt.Errorf("no result calculation data found")
	}

	winningTickets := draw.StageData.ResultCalculationData.WinningTickets
	if len(winningTickets) == 0 {
		pp.drawService.logger.Printf("No winning tickets to process for draw %s", draw.ID)
		return nil
	}

	pp.drawService.logger.Printf("Initializing payout records for %d winning tickets", len(winningTickets))

	// Create payout records in batch (idempotent - will skip duplicates)
	records := make([]*models.DrawPayoutRecord, 0, len(winningTickets))

	for _, ticket := range winningTickets {
		ticketID, err := uuid.Parse(ticket.TicketID)
		if err != nil {
			pp.drawService.logger.Printf("Invalid ticket ID format: %s, skipping", ticket.TicketID)
			continue
		}

		retailerID, err := uuid.Parse(ticket.RetailerID)
		if err != nil {
			pp.drawService.logger.Printf("Invalid retailer ID format: %s, skipping", ticket.RetailerID)
			continue
		}

		// Determine if requires manual approval (big win threshold: 24,000 GHS)
		requiresApproval := ticket.WinningAmount > BigWinThreshold
		payoutType := models.PayoutTypeAuto
		status := models.PayoutStatusPending

		if requiresApproval {
			payoutType = models.PayoutTypeManual
			status = models.PayoutStatusSkipped // Will require manual approval
		}

		record := &models.DrawPayoutRecord{
			ID:               uuid.New(),
			DrawID:           draw.ID,
			TicketID:         ticketID,
			SerialNumber:     ticket.SerialNumber,
			RetailerID:       retailerID,
			WinningAmount:    ticket.WinningAmount,
			Status:           status,
			PayoutType:       payoutType,
			RequiresApproval: requiresApproval,
			IdempotencyKey:   fmt.Sprintf("payout-%s-%s", draw.ID.String(), ticket.TicketID),
			AttemptCount:     0,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		records = append(records, record)
	}

	// Batch insert (will skip duplicates based on unique constraint)
	if len(records) > 0 {
		if err := pp.drawService.drawRepo.CreatePayoutRecordsBatch(ctx, records); err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to create payout records: %w", err)
		}
	}

	pp.drawService.logger.Printf("Initialized %d payout records for draw %s", len(records), draw.ID)
	span.SetAttributes(attribute.Int("records_created", len(records)))

	return nil
}

// processAutoPayouts processes all auto-eligible payouts with concurrent workers
func (pp *PayoutProcessor) processAutoPayouts(ctx context.Context, drawID uuid.UUID) (*PayoutStats, error) {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "PayoutProcessor.processAutoPayouts")
	defer span.End()

	stats := &PayoutStats{
		Succeeded: 0,
		Failed:    0,
	}

	for {
		// Fetch next batch of pending payouts
		pendingPayouts, err := pp.drawService.drawRepo.GetPendingPayouts(ctx, drawID, pp.batchSize)
		if err != nil {
			return stats, fmt.Errorf("failed to get pending payouts: %w", err)
		}

		if len(pendingPayouts) == 0 {
			pp.drawService.logger.Printf("No more pending payouts for draw %s", drawID)
			break
		}

		pp.drawService.logger.Printf("Processing batch of %d payouts", len(pendingPayouts))

		// Process batch with worker pool
		batchStats := pp.processBatchConcurrent(ctx, pendingPayouts)
		stats.Succeeded += batchStats.Succeeded
		stats.Failed += batchStats.Failed

		// If batch was smaller than batchSize, we're done
		if len(pendingPayouts) < pp.batchSize {
			break
		}
	}

	pp.drawService.logger.Printf("Auto payout processing complete: succeeded=%d, failed=%d", stats.Succeeded, stats.Failed)
	span.SetAttributes(
		attribute.Int64("succeeded", stats.Succeeded),
		attribute.Int64("failed", stats.Failed),
	)

	return stats, nil
}

// processBatchConcurrent processes a batch of payouts using concurrent workers
func (pp *PayoutProcessor) processBatchConcurrent(ctx context.Context, payouts []*models.DrawPayoutRecord) *PayoutStats {
	stats := &PayoutStats{}
	var statsLock sync.Mutex

	// Create work channel
	workChan := make(chan *models.DrawPayoutRecord, len(payouts))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < pp.concurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for payout := range workChan {
				success := pp.processSinglePayout(ctx, payout)
				statsLock.Lock()
				if success {
					stats.Succeeded++
				} else {
					stats.Failed++
				}
				statsLock.Unlock()
			}
		}(i)
	}

	// Send work
	for _, payout := range payouts {
		workChan <- payout
	}
	close(workChan)

	// Wait for completion
	wg.Wait()

	return stats
}

// processSinglePayout processes a single payout record (idempotent)
func (pp *PayoutProcessor) processSinglePayout(ctx context.Context, record *models.DrawPayoutRecord) bool {
	ctx, span := otel.Tracer("draw-service").Start(ctx, "PayoutProcessor.processSinglePayout")
	defer span.End()

	startTime := time.Now()

	span.SetAttributes(
		attribute.String("ticket_id", record.TicketID.String()),
		attribute.String("retailer_id", record.RetailerID.String()),
		attribute.Int64("amount", record.WinningAmount),
	)

	// ========== PAYOUT START ==========
	pp.drawService.logger.Printf("STAGE4_PAYOUT_START: ticket_id=%s, retailer_id=%s, amount=%d pesewas, attempt=%d, serial=%s",
		record.TicketID, record.RetailerID, record.WinningAmount, record.AttemptCount+1, record.SerialNumber)

	// Update status to processing
	record.Status = models.PayoutStatusProcessing
	record.AttemptCount++
	now := time.Now()
	record.LastAttemptAt = &now
	record.UpdatedAt = now

	pp.drawService.logger.Printf("STAGE4_PAYOUT_STATUS_UPDATE: ticket_id=%s, status=processing, attempt_count=%d",
		record.TicketID, record.AttemptCount)

	if err := pp.drawService.drawRepo.UpdatePayoutRecord(ctx, record); err != nil {
		pp.drawService.logger.Printf("STAGE4_PAYOUT_ERROR: ticket_id=%s, step=update_status, error=%v", record.TicketID, err)
		return false
	}

	// Step 1: Credit retailer winning wallet (IDEMPOTENT via idempotency_key)
	pp.drawService.logger.Printf("STAGE4_WALLET_CREDIT_START: ticket_id=%s, retailer_id=%s, amount=%d pesewas, idempotency_key=%s",
		record.TicketID, record.RetailerID, record.WinningAmount, record.IdempotencyKey)

	walletClient, err := pp.drawService.grpcClientManager.WalletServiceClient(ctx)
	if err != nil {
		pp.drawService.logger.Printf("STAGE4_WALLET_CLIENT_ERROR: ticket_id=%s, error=%v", record.TicketID, err)
		pp.markPayoutFailed(ctx, record, fmt.Sprintf("wallet client error: %v", err))
		return false
	}

	walletSvc, ok := walletClient.(walletv1.WalletServiceClient)
	if !ok {
		pp.drawService.logger.Printf("STAGE4_WALLET_CLIENT_TYPE_ERROR: ticket_id=%s, expected=WalletServiceClient", record.TicketID)
		pp.markPayoutFailed(ctx, record, "invalid wallet client type")
		return false
	}

	// Credit wallet with idempotency key
	walletRequest := &walletv1.CreditRetailerWalletRequest{
		RetailerId:     record.RetailerID.String(),
		WalletType:     walletv1.WalletType_RETAILER_WINNING,
		Amount:         float64(record.WinningAmount), // Pesewas
		Reference:      fmt.Sprintf("draw-%s-ticket-%s", record.DrawID.String(), record.TicketID.String()),
		Notes:          fmt.Sprintf("Winning payout for draw %s", record.DrawID),
		IdempotencyKey: record.IdempotencyKey,
	}

	pp.drawService.logger.Printf("STAGE4_WALLET_GRPC_CALL: ticket_id=%s, retailer_id=%s, wallet_type=RETAILER_WINNING, amount=%.2f, reference=%s, idempotency_key=%s",
		record.TicketID, record.RetailerID, walletRequest.Amount, walletRequest.Reference, walletRequest.IdempotencyKey)

	walletResp, err := walletSvc.CreditRetailerWallet(ctx, walletRequest)
	if err != nil {
		pp.drawService.logger.Printf("STAGE4_WALLET_CREDIT_FAILED: ticket_id=%s, retailer_id=%s, amount=%d pesewas, error=%v",
			record.TicketID, record.RetailerID, record.WinningAmount, err)
		pp.markPayoutFailed(ctx, record, fmt.Sprintf("wallet credit error: %v", err))
		return false
	}

	// Record wallet transaction ID
	walletTxID := walletResp.TransactionId
	record.WalletTransactionID = &walletTxID
	walletCreditedAt := time.Now()
	record.WalletCreditedAt = &walletCreditedAt

	pp.drawService.logger.Printf("STAGE4_WALLET_CREDIT_SUCCESS: ticket_id=%s, retailer_id=%s, amount=%d pesewas, wallet_tx_id=%s, duration_ms=%d",
		record.TicketID, record.RetailerID, record.WinningAmount, walletTxID, time.Since(startTime).Milliseconds())

	// Persist wallet transaction details immediately for audit trail
	pp.drawService.logger.Printf("STAGE4_WALLET_AUDIT_PERSIST: ticket_id=%s, wallet_tx_id=%s, persisting audit trail",
		record.TicketID, walletTxID)

	if err := pp.drawService.drawRepo.UpdatePayoutRecord(ctx, record); err != nil {
		pp.drawService.logger.Printf("STAGE4_WALLET_AUDIT_FAILED: ticket_id=%s, wallet_tx_id=%s, error=%v, status=WALLET_CREDITED_BUT_AUDIT_FAILED",
			record.TicketID, walletTxID, err)
		pp.markPayoutNeedsReconcile(ctx, record, fmt.Sprintf("failed to record wallet TX (wallet credited): %v", err))
		return false
	}

	pp.drawService.logger.Printf("STAGE4_WALLET_COMPLETE: ticket_id=%s, wallet_tx_id=%s, audit_persisted=true",
		record.TicketID, walletTxID)

	// Step 2: Mark ticket as paid
	pp.drawService.logger.Printf("STAGE4_TICKET_MARK_START: ticket_id=%s, wallet_tx_id=%s, marking ticket as paid",
		record.TicketID, walletTxID)

	ticketClient, err := pp.drawService.grpcClientManager.TicketServiceClient(ctx)
	if err != nil {
		pp.drawService.logger.Printf("STAGE4_TICKET_CLIENT_ERROR: ticket_id=%s, wallet_tx_id=%s, error=%v, status=WALLET_CREDITED_TICKET_NOT_MARKED",
			record.TicketID, walletTxID, err)
		// Wallet already credited - needs manual reconciliation
		pp.markPayoutNeedsReconcile(ctx, record, fmt.Sprintf("ticket client error (wallet credited): %v", err))
		return false
	}

	ticketSvc, ok := ticketClient.(ticketv1.TicketServiceClient)
	if !ok {
		pp.drawService.logger.Printf("STAGE4_TICKET_CLIENT_TYPE_ERROR: ticket_id=%s, wallet_tx_id=%s, expected=TicketServiceClient, status=WALLET_CREDITED_TICKET_NOT_MARKED",
			record.TicketID, walletTxID)
		pp.markPayoutNeedsReconcile(ctx, record, "invalid ticket client type (wallet credited)")
		return false
	}

	ticketRequest := &ticketv1.MarkTicketAsPaidRequest{
		TicketId:         record.TicketID.String(),
		PaidAmount:       record.WinningAmount,
		PaymentReference: walletTxID,
		PaidBy:           "system",
		DrawId:           record.DrawID.String(),
	}

	pp.drawService.logger.Printf("STAGE4_TICKET_GRPC_CALL: ticket_id=%s, paid_amount=%d pesewas, payment_reference=%s, draw_id=%s",
		record.TicketID, ticketRequest.PaidAmount, ticketRequest.PaymentReference, ticketRequest.DrawId)

	_, err = ticketSvc.MarkTicketAsPaid(ctx, ticketRequest)
	if err != nil {
		pp.drawService.logger.Printf("STAGE4_TICKET_MARK_FAILED: ticket_id=%s, wallet_tx_id=%s, error=%v, status=WALLET_CREDITED_TICKET_NOT_MARKED",
			record.TicketID, walletTxID, err)
		pp.markPayoutNeedsReconcile(ctx, record, fmt.Sprintf("ticket update error (wallet credited): %v", err))
		return false
	}

	ticketMarkedPaidAt := time.Now()
	record.TicketMarkedPaidAt = &ticketMarkedPaidAt

	pp.drawService.logger.Printf("STAGE4_TICKET_MARK_SUCCESS: ticket_id=%s, wallet_tx_id=%s, marked_as_paid=true",
		record.TicketID, walletTxID)

	// Persist ticket marked timestamp immediately for audit trail
	pp.drawService.logger.Printf("STAGE4_TICKET_AUDIT_PERSIST: ticket_id=%s, wallet_tx_id=%s, persisting ticket marked timestamp",
		record.TicketID, walletTxID)

	if err := pp.drawService.drawRepo.UpdatePayoutRecord(ctx, record); err != nil {
		pp.drawService.logger.Printf("STAGE4_TICKET_AUDIT_FAILED: ticket_id=%s, wallet_tx_id=%s, error=%v, status=WALLET_CREDITED_TICKET_MARKED_AUDIT_FAILED",
			record.TicketID, walletTxID, err)
		pp.markPayoutNeedsReconcile(ctx, record, fmt.Sprintf("failed to record ticket marked timestamp (wallet credited, ticket marked): %v", err))
		return false
	}

	pp.drawService.logger.Printf("STAGE4_TICKET_COMPLETE: ticket_id=%s, wallet_tx_id=%s, audit_persisted=true",
		record.TicketID, walletTxID)

	// Mark as completed
	record.Status = models.PayoutStatusCompleted
	completedAt := time.Now()
	record.CompletedAt = &completedAt
	record.UpdatedAt = completedAt

	pp.drawService.logger.Printf("STAGE4_PAYOUT_FINALIZING: ticket_id=%s, wallet_tx_id=%s, marking payout as completed",
		record.TicketID, walletTxID)

	if err := pp.drawService.drawRepo.UpdatePayoutRecord(ctx, record); err != nil {
		pp.drawService.logger.Printf("STAGE4_PAYOUT_FINAL_ERROR: ticket_id=%s, wallet_tx_id=%s, error=%v, status=ALL_OPERATIONS_COMPLETED_BUT_FINAL_UPDATE_FAILED",
			record.TicketID, walletTxID, err)
		return false
	}

	totalDuration := time.Since(startTime).Milliseconds()

	pp.drawService.logger.Printf("STAGE4_PAYOUT_SUCCESS: ticket_id=%s, serial_number=%s, retailer_id=%s, amount=%d pesewas, wallet_tx_id=%s, total_duration_ms=%d",
		record.TicketID, record.SerialNumber, record.RetailerID, record.WinningAmount, walletTxID, totalDuration)

	span.SetAttributes(
		attribute.String("result", "success"),
		attribute.String("wallet_tx_id", walletTxID),
		attribute.Int64("duration_ms", totalDuration),
	)
	return true
}

// markPayoutNeedsReconcile marks a payout record as needing manual reconciliation
// This is used when wallet has been credited but downstream operations failed
func (pp *PayoutProcessor) markPayoutNeedsReconcile(ctx context.Context, record *models.DrawPayoutRecord, errorMsg string) {
	pp.drawService.logger.Printf("STAGE4_PAYOUT_NEEDS_RECONCILE: ticket_id=%s, error=%s, wallet_credited=true, requires_manual_reconciliation=true",
		record.TicketID, errorMsg)

	record.Status = models.PayoutStatusNeedsReconcile
	record.LastError = &errorMsg
	record.UpdatedAt = time.Now()

	if err := pp.drawService.drawRepo.UpdatePayoutRecord(ctx, record); err != nil {
		pp.drawService.logger.Printf("STAGE4_RECONCILE_MARK_FAILED: ticket_id=%s, failed to mark as needs_reconcile: %v", record.TicketID, err)
	} else {
		pp.drawService.logger.Printf("STAGE4_RECONCILE_MARKED: ticket_id=%s, status=needs_reconcile, requires_manual_action=true", record.TicketID)
	}
}

// markPayoutFailed marks a payout record as failed
func (pp *PayoutProcessor) markPayoutFailed(ctx context.Context, record *models.DrawPayoutRecord, errorMsg string) {
	pp.drawService.logger.Printf("STAGE4_PAYOUT_MARK_FAILED: ticket_id=%s, error=%s, attempt=%d",
		record.TicketID, errorMsg, record.AttemptCount)

	record.Status = models.PayoutStatusFailed
	record.LastError = &errorMsg
	record.UpdatedAt = time.Now()

	if err := pp.drawService.drawRepo.UpdatePayoutRecord(ctx, record); err != nil {
		pp.drawService.logger.Printf("STAGE4_PAYOUT_MARK_FAILED_ERROR: ticket_id=%s, failed to update record: %v", record.TicketID, err)
	} else {
		pp.drawService.logger.Printf("STAGE4_PAYOUT_MARKED_FAILED: ticket_id=%s, status=failed, last_error=%s", record.TicketID, errorMsg)
	}
}

// PayoutStats tracks payout processing statistics
type PayoutStats struct {
	Succeeded int64
	Failed    int64
}

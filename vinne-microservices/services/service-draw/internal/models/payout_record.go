package models

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PayoutStatus represents the status of a payout
type PayoutStatus string

const (
	PayoutStatusPending        PayoutStatus = "pending"
	PayoutStatusProcessing     PayoutStatus = "processing"
	PayoutStatusCompleted      PayoutStatus = "completed"
	PayoutStatusFailed         PayoutStatus = "failed"
	PayoutStatusSkipped        PayoutStatus = "skipped"         // Excluded (big win, manual approval needed)
	PayoutStatusNeedsReconcile PayoutStatus = "needs_reconcile" // Wallet credited but downstream operation failed - requires manual reconciliation
)

// PayoutType represents the type of payout processing
type PayoutType string

const (
	PayoutTypeAuto   PayoutType = "auto"
	PayoutTypeManual PayoutType = "manual"
)

// DrawPayoutRecord tracks individual ticket payout processing
type DrawPayoutRecord struct {
	ID           uuid.UUID `json:"id" db:"id"`
	DrawID       uuid.UUID `json:"draw_id" db:"draw_id"`
	TicketID     uuid.UUID `json:"ticket_id" db:"ticket_id"`
	SerialNumber string    `json:"serial_number" db:"serial_number"`
	RetailerID   uuid.UUID `json:"retailer_id" db:"retailer_id"`

	// Amount tracking
	WinningAmount int64 `json:"winning_amount" db:"winning_amount"` // in pesewas

	// Payout status
	Status           PayoutStatus `json:"status" db:"status"`
	PayoutType       PayoutType   `json:"payout_type" db:"payout_type"`
	RequiresApproval bool         `json:"requires_approval" db:"requires_approval"`

	// Processing tracking
	WalletTransactionID *string    `json:"wallet_transaction_id,omitempty" db:"wallet_transaction_id"`
	IdempotencyKey      string     `json:"idempotency_key" db:"idempotency_key"`
	AttemptCount        int        `json:"attempt_count" db:"attempt_count"`
	LastAttemptAt       *time.Time `json:"last_attempt_at,omitempty" db:"last_attempt_at"`
	LastError           *string    `json:"last_error,omitempty" db:"last_error"`

	// Completion tracking
	WalletCreditedAt   *time.Time `json:"wallet_credited_at,omitempty" db:"wallet_credited_at"`
	TicketMarkedPaidAt *time.Time `json:"ticket_marked_paid_at,omitempty" db:"ticket_marked_paid_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty" db:"completed_at"`

	// Manual approval (for big wins)
	ApprovedBy      *string    `json:"approved_by,omitempty" db:"approved_by"`
	ApprovedAt      *time.Time `json:"approved_at,omitempty" db:"approved_at"`
	RejectionReason *string    `json:"rejection_reason,omitempty" db:"rejection_reason"`

	// Audit
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// PayoutSummary aggregates payout statistics for a draw
type PayoutSummary struct {
	DrawID uuid.UUID `json:"draw_id"`

	// Expected from Stage 3
	TotalWinningTickets int64 `json:"total_winning_tickets"`
	TotalWinningAmount  int64 `json:"total_winning_amount"` // in pesewas

	// Actual payout status
	PendingCount     int64 `json:"pending_count"`
	PendingAmount    int64 `json:"pending_amount"` // in pesewas
	ProcessingCount  int64 `json:"processing_count"`
	ProcessingAmount int64 `json:"processing_amount"` // in pesewas
	CompletedCount   int64 `json:"completed_count"`
	CompletedAmount  int64 `json:"completed_amount"` // in pesewas
	FailedCount      int64 `json:"failed_count"`
	FailedAmount     int64 `json:"failed_amount"`  // in pesewas
	SkippedCount     int64 `json:"skipped_count"`  // Big wins awaiting manual approval
	SkippedAmount    int64 `json:"skipped_amount"` // in pesewas
}

// String methods for enums
func (ps PayoutStatus) String() string {
	return string(ps)
}

func (pt PayoutType) String() string {
	return string(pt)
}

// Value and Scan methods for database compatibility
func (ps PayoutStatus) Value() (driver.Value, error) {
	return string(ps), nil
}

func (ps *PayoutStatus) Scan(value interface{}) error {
	if value == nil {
		*ps = PayoutStatusPending
		return nil
	}
	if str, ok := value.(string); ok {
		*ps = PayoutStatus(str)
		return nil
	}
	return fmt.Errorf("cannot scan %T into PayoutStatus", value)
}

func (pt PayoutType) Value() (driver.Value, error) {
	return string(pt), nil
}

func (pt *PayoutType) Scan(value interface{}) error {
	if value == nil {
		*pt = PayoutTypeAuto
		return nil
	}
	if str, ok := value.(string); ok {
		*pt = PayoutType(str)
		return nil
	}
	return fmt.Errorf("cannot scan %T into PayoutType", value)
}

// Helper methods

// IsProcessable checks if the payout can be processed
func (pr *DrawPayoutRecord) IsProcessable() bool {
	return pr.Status == PayoutStatusPending && !pr.RequiresApproval
}

// IsFailed checks if the payout has failed
func (pr *DrawPayoutRecord) IsFailed() bool {
	return pr.Status == PayoutStatusFailed
}

// IsCompleted checks if the payout is completed
func (pr *DrawPayoutRecord) IsCompleted() bool {
	return pr.Status == PayoutStatusCompleted
}

// NeedsManualApproval checks if this is a big win requiring manual approval
func (pr *DrawPayoutRecord) NeedsManualApproval() bool {
	return pr.RequiresApproval && pr.Status == PayoutStatusSkipped
}

// GetWinningAmountInGHS returns the winning amount in Ghana Cedis
func (pr *DrawPayoutRecord) GetWinningAmountInGHS() float64 {
	return PesewasToGHS(pr.WinningAmount)
}

// MarkAsProcessing updates the record status to processing
func (pr *DrawPayoutRecord) MarkAsProcessing() {
	pr.Status = PayoutStatusProcessing
	pr.AttemptCount++
	now := time.Now()
	pr.LastAttemptAt = &now
	pr.UpdatedAt = now
}

// MarkAsCompleted updates the record status to completed
func (pr *DrawPayoutRecord) MarkAsCompleted(walletTxID string) {
	pr.Status = PayoutStatusCompleted
	pr.WalletTransactionID = &walletTxID
	now := time.Now()
	pr.CompletedAt = &now
	pr.UpdatedAt = now
}

// MarkAsFailed updates the record status to failed with error message
func (pr *DrawPayoutRecord) MarkAsFailed(errorMsg string) {
	pr.Status = PayoutStatusFailed
	pr.LastError = &errorMsg
	pr.UpdatedAt = time.Now()
}

// RecordWalletCredit records when the wallet was credited
func (pr *DrawPayoutRecord) RecordWalletCredit(txID string) {
	pr.WalletTransactionID = &txID
	now := time.Now()
	pr.WalletCreditedAt = &now
	pr.UpdatedAt = now
}

// RecordTicketMarkedPaid records when the ticket was marked as paid
func (pr *DrawPayoutRecord) RecordTicketMarkedPaid() {
	now := time.Now()
	pr.TicketMarkedPaidAt = &now
	pr.UpdatedAt = now
}

// PayoutSummary helper methods

// GetCompletionRate returns the completion rate as a percentage
func (ps *PayoutSummary) GetCompletionRate() float64 {
	if ps.TotalWinningTickets == 0 {
		return 0
	}
	return (float64(ps.CompletedCount) / float64(ps.TotalWinningTickets)) * 100
}

// GetDiscrepancy returns the amount discrepancy (expected - actual paid)
func (ps *PayoutSummary) GetDiscrepancy() int64 {
	return ps.TotalWinningAmount - ps.CompletedAmount
}

// IsFullyProcessed checks if all payouts are completed or skipped
func (ps *PayoutSummary) IsFullyProcessed() bool {
	return ps.PendingCount == 0 && ps.ProcessingCount == 0 && ps.FailedCount == 0
}

// HasFailures checks if there are any failed payouts
func (ps *PayoutSummary) HasFailures() bool {
	return ps.FailedCount > 0
}

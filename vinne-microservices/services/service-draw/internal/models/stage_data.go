package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// StageStatus represents the status of a draw execution stage
type StageStatus string

const (
	StageStatusUnspecified StageStatus = "unspecified"
	StageStatusPending     StageStatus = "pending"
	StageStatusInProgress  StageStatus = "in_progress"
	StageStatusCompleted   StageStatus = "completed"
	StageStatusFailed      StageStatus = "failed"
)

// DrawStage contains the execution workflow stage data
type DrawStage struct {
	CurrentStage          int                         `json:"current_stage"`      // 1-4
	StageName             string                      `json:"stage_name"`         // Preparation, Number Selection, Result Calculation, Payout
	StageStatus           StageStatus                 `json:"stage_status"`       // pending, in_progress, completed, failed
	StageStartedAt        *time.Time                  `json:"stage_started_at"`   // When this stage started
	StageCompletedAt      *time.Time                  `json:"stage_completed_at"` // When this stage completed
	PreparationData       *PreparationStageData       `json:"preparation_data,omitempty"`
	NumberSelectionData   *NumberSelectionStageData   `json:"number_selection_data,omitempty"`
	ResultCalculationData *ResultCalculationStageData `json:"result_calculation_data,omitempty"`
	PayoutData            *PayoutStageData            `json:"payout_data,omitempty"`
}

// PreparationStageData contains data for Stage 1
type PreparationStageData struct {
	TicketsLocked int64      `json:"tickets_locked"`
	TotalStakes   int64      `json:"total_stakes"` // in pesewas
	SalesLocked   bool       `json:"sales_locked"`
	LockTime      *time.Time `json:"lock_time"`
}

// NumberSelectionStageData contains data for Stage 2
type NumberSelectionStageData struct {
	WinningNumbers       []int32               `json:"winning_numbers,omitempty"`
	VerificationAttempts []VerificationAttempt `json:"verification_attempts"`
	IsVerified           bool                  `json:"is_verified"`
	VerifiedBy           string                `json:"verified_by,omitempty"`
	VerifiedAt           *time.Time            `json:"verified_at,omitempty"`
}

// VerificationAttempt represents a single verification attempt
type VerificationAttempt struct {
	AttemptNumber int32     `json:"attempt_number"`
	Numbers       []int32   `json:"numbers"`
	SubmittedBy   string    `json:"submitted_by"`
	SubmittedAt   time.Time `json:"submitted_at"`
}

// ResultCalculationStageData contains data for Stage 3
type ResultCalculationStageData struct {
	WinningTicketsCount int64                 `json:"winning_tickets_count"`
	TotalWinnings       int64                 `json:"total_winnings"` // in pesewas
	WinningTiers        []WinningTier         `json:"winning_tiers"`
	WinningTickets      []WinningTicketDetail `json:"winning_tickets,omitempty"` // Individual winning ticket details
	CalculatedAt        *time.Time            `json:"calculated_at"`
}

// WinningTier represents aggregated winnings by bet type
type WinningTier struct {
	BetType      string `json:"bet_type"` // "Direct 1", "Perm 2", etc.
	WinnersCount int64  `json:"winners_count"`
	TotalAmount  int64  `json:"total_amount"` // in pesewas
}

// WinningTicketDetail represents individual winning ticket information
type WinningTicketDetail struct {
	TicketID      string  `json:"ticket_id"`
	SerialNumber  string  `json:"serial_number"`
	RetailerID    string  `json:"retailer_id"`
	Numbers       []int32 `json:"numbers"`        // The numbers the player selected
	BetType       string  `json:"bet_type"`       // "Direct 1", "Perm 2", etc.
	StakeAmount   int64   `json:"stake_amount"`   // in pesewas
	WinningAmount int64   `json:"winning_amount"` // in pesewas
	MatchesCount  int32   `json:"matches_count"`  // How many numbers matched
	IsBigWin      bool    `json:"is_big_win"`     // true if > 25,000 GHS

	// Additional ticket details from ticket service (enriched at runtime)
	AgentCode     string `json:"agent_code,omitempty"`     // Agent code who issued the ticket
	TerminalID    string `json:"terminal_id,omitempty"`    // Terminal ID where ticket was issued
	CustomerPhone string `json:"customer_phone,omitempty"` // Customer phone number
	PaymentMethod string `json:"payment_method,omitempty"` // Payment method used
	Status        string `json:"status,omitempty"`         // Ticket status (active, validated, paid, etc.)
}

// PayoutStageData contains data for Stage 4
type PayoutStageData struct {
	AutoProcessedCount   int64          `json:"auto_processed_count"`   // Tickets <= 25k GHS
	ManualApprovalCount  int64          `json:"manual_approval_count"`  // Tickets > 25k GHS
	AutoProcessedAmount  int64          `json:"auto_processed_amount"`  // in pesewas
	ManualApprovalAmount int64          `json:"manual_approval_amount"` // in pesewas
	ProcessedCount       int64          `json:"processed_count"`
	PendingCount         int64          `json:"pending_count"`
	BigWinPayouts        []BigWinPayout `json:"big_win_payouts,omitempty"`
}

// BigWinPayout represents a payout requiring manual approval
type BigWinPayout struct {
	TicketID        string     `json:"ticket_id"`
	Amount          int64      `json:"amount"` // in pesewas
	Status          string     `json:"status"` // pending, approved, rejected
	ApprovedBy      string     `json:"approved_by,omitempty"`
	RejectionReason string     `json:"rejection_reason,omitempty"`
	ProcessedAt     *time.Time `json:"processed_at,omitempty"`
}

// Value implements driver.Valuer for database storage
func (ds DrawStage) Value() (driver.Value, error) {
	return json.Marshal(ds)
}

// Scan implements sql.Scanner for database retrieval
func (ds *DrawStage) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal DrawStage value: %v", value)
	}

	return json.Unmarshal(bytes, ds)
}

// String methods for enums
func (ss StageStatus) String() string {
	return string(ss)
}

// Value and Scan methods for StageStatus
func (ss StageStatus) Value() (driver.Value, error) {
	return string(ss), nil
}

func (ss *StageStatus) Scan(value interface{}) error {
	if value == nil {
		*ss = StageStatusPending
		return nil
	}
	if str, ok := value.(string); ok {
		*ss = StageStatus(str)
		return nil
	}
	return fmt.Errorf("cannot scan %T into StageStatus", value)
}

// Helper methods
func (ds *DrawStage) GetStageName(stageNum int) string {
	switch stageNum {
	case 1:
		return "Preparation"
	case 2:
		return "Number Selection"
	case 3:
		return "Result Calculation"
	case 4:
		return "Payout"
	default:
		return "Unknown"
	}
}

func (ds *DrawStage) IsStageCompleted(stageNum int) bool {
	return ds.CurrentStage > stageNum || (ds.CurrentStage == stageNum && ds.StageStatus == StageStatusCompleted)
}

func (ds *DrawStage) CanStartStage(stageNum int) bool {
	// Can start stage 1 if no current stage
	if stageNum == 1 {
		return ds.CurrentStage == 0 || ds.CurrentStage == 1
	}
	// Can start next stage if previous stage is completed
	return ds.CurrentStage == stageNum-1 && ds.StageStatus == StageStatusCompleted
}

package models

import (
	"time"

	"github.com/google/uuid"
)

// TransactionStatus represents the status of a transaction
type TransactionStatus string

const (
	StatusPending    TransactionStatus = "PENDING"
	StatusProcessing TransactionStatus = "PROCESSING"
	StatusSuccess    TransactionStatus = "SUCCESS"
	StatusFailed     TransactionStatus = "FAILED"
	StatusVerifying  TransactionStatus = "VERIFYING"
	StatusDuplicate  TransactionStatus = "DUPLICATE"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	TypeDeposit      TransactionType = "DEPOSIT"       // Mobile Money -> Stake Wallet
	TypeWithdrawal   TransactionType = "WITHDRAWAL"    // Winning Wallet -> Mobile Money
	TypeBankTransfer TransactionType = "BANK_TRANSFER" // Wallet -> Bank Account
)

// Transaction represents a payment transaction
type Transaction struct {
	ID                    uuid.UUID              `db:"id"`
	Reference             string                 `db:"reference"`
	ProviderTransactionID *string                `db:"provider_transaction_id"`
	Type                  TransactionType        `db:"type"`
	Status                TransactionStatus      `db:"status"`
	Amount                int64                  `db:"amount"` // Amount in pesewas
	Currency              string                 `db:"currency"`
	Narration             string                 `db:"narration"`
	ProviderName          string                 `db:"provider_name"`
	SourceType            string                 `db:"source_type"`
	SourceIdentifier      string                 `db:"source_identifier"`
	SourceName            string                 `db:"source_name"`
	DestinationType       string                 `db:"destination_type"`
	DestinationIdentifier string                 `db:"destination_identifier"`
	DestinationName       string                 `db:"destination_name"`
	UserID                uuid.UUID              `db:"user_id"`
	CustomerRemarks       string                 `db:"customer_remarks"`
	Metadata              map[string]string      `db:"metadata"`
	ProviderData          map[string]interface{} `db:"provider_data"`
	ErrorMessage          *string                `db:"error_message"`
	ErrorCode             *string                `db:"error_code"`
	RetryCount            int                    `db:"retry_count"`
	LastRetryAt           *time.Time             `db:"last_retry_at"`
	RequestedAt           time.Time              `db:"requested_at"`
	CompletedAt           *time.Time             `db:"completed_at"`
	CreatedAt             time.Time              `db:"created_at"`
	UpdatedAt             time.Time              `db:"updated_at"`
}

// IsTerminal checks if the transaction is in a terminal state
func (t *Transaction) IsTerminal() bool {
	return t.Status == StatusSuccess || t.Status == StatusFailed || t.Status == StatusDuplicate
}

// CanRetry checks if the transaction can be retried
func (t *Transaction) CanRetry(maxRetries int) bool {
	return !t.IsTerminal() && t.RetryCount < maxRetries
}

// IsPending checks if the transaction is pending
func (t *Transaction) IsPending() bool {
	return t.Status == StatusPending || t.Status == StatusProcessing || t.Status == StatusVerifying
}

// MarkAsProcessing updates the transaction status to processing
func (t *Transaction) MarkAsProcessing() {
	t.Status = StatusProcessing
	t.UpdatedAt = time.Now()
}

// MarkAsSuccess marks the transaction as successful
func (t *Transaction) MarkAsSuccess(providerTxID string, completedAt time.Time) {
	t.Status = StatusSuccess
	t.ProviderTransactionID = &providerTxID
	t.CompletedAt = &completedAt
	t.UpdatedAt = time.Now()
}

// MarkAsFailed marks the transaction as failed
func (t *Transaction) MarkAsFailed(errorMsg string, errorCode string) {
	t.Status = StatusFailed
	t.ErrorMessage = &errorMsg
	t.ErrorCode = &errorCode
	now := time.Now()
	t.CompletedAt = &now
	t.UpdatedAt = now
}

// MarkAsVerifying marks the transaction as verifying
func (t *Transaction) MarkAsVerifying() {
	t.Status = StatusVerifying
	t.UpdatedAt = time.Now()
}

// IncrementRetry increments the retry count
func (t *Transaction) IncrementRetry() {
	t.RetryCount++
	now := time.Now()
	t.LastRetryAt = &now
	t.UpdatedAt = now
}

// CreateTransaction creates a new transaction instance
func CreateTransaction(
	reference string,
	txType TransactionType,
	amount int64,
	currency string,
	providerName string,
	userID uuid.UUID,
) *Transaction {
	now := time.Now()
	return &Transaction{
		ID:           uuid.New(),
		Reference:    reference,
		Type:         txType,
		Status:       StatusPending,
		Amount:       amount,
		Currency:     currency,
		ProviderName: providerName,
		UserID:       userID,
		RequestedAt:  now,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata:     make(map[string]string),
		ProviderData: make(map[string]interface{}),
	}
}

// TransactionFilter represents filters for querying transactions
type TransactionFilter struct {
	UserID    *uuid.UUID
	Type      *TransactionType
	Status    *TransactionStatus
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
}

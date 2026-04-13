package events

import (
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of event
type EventType string

const (
	// Transaction events
	EventTypeTransactionCreated    EventType = "payment.transaction.created"
	EventTypeTransactionProcessing EventType = "payment.transaction.processing"
	EventTypeTransactionCompleted  EventType = "payment.transaction.completed"
	EventTypeTransactionFailed     EventType = "payment.transaction.failed"
	EventTypeTransactionVerifying  EventType = "payment.transaction.verifying"

	// Deposit events
	EventTypeDepositInitiated EventType = "payment.deposit.initiated"
	EventTypeDepositCompleted EventType = "payment.deposit.completed"
	EventTypeDepositFailed    EventType = "payment.deposit.failed"

	// Withdrawal events
	EventTypeWithdrawalInitiated EventType = "payment.withdrawal.initiated"
	EventTypeWithdrawalCompleted EventType = "payment.withdrawal.completed"
	EventTypeWithdrawalFailed    EventType = "payment.withdrawal.failed"

	// Saga events
	EventTypeSagaStarted     EventType = "payment.saga.started"
	EventTypeSagaCompleted   EventType = "payment.saga.completed"
	EventTypeSagaFailed      EventType = "payment.saga.failed"
	EventTypeSagaCompensated EventType = "payment.saga.compensated"

	// Provider events
	EventTypeProviderCallSuccess EventType = "payment.provider.call.success"
	EventTypeProviderCallFailed  EventType = "payment.provider.call.failed"
	EventTypeProviderTimeout     EventType = "payment.provider.timeout"
)

// Event represents a base event structure
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
	Metadata  map[string]string      `json:"metadata"`
}

// TransactionEvent represents transaction-related events
type TransactionEvent struct {
	EventID       string            `json:"event_id"`
	EventType     EventType         `json:"event_type"`
	Timestamp     time.Time         `json:"timestamp"`
	TransactionID string            `json:"transaction_id"`
	Reference     string            `json:"reference"`
	Type          string            `json:"type"` // DEPOSIT, WITHDRAWAL, BANK_TRANSFER
	Status        string            `json:"status"`
	Amount        int64             `json:"amount"`
	Currency      string            `json:"currency"`
	UserID        string            `json:"user_id"`
	ProviderName  string            `json:"provider_name"`
	ErrorMessage  string            `json:"error_message,omitempty"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// DepositEvent represents deposit-specific events
type DepositEvent struct {
	EventID        string            `json:"event_id"`
	EventType      EventType         `json:"event_type"`
	Timestamp      time.Time         `json:"timestamp"`
	TransactionID  string            `json:"transaction_id"`
	Reference      string            `json:"reference"`
	UserID         string            `json:"user_id"`
	WalletNumber   string            `json:"wallet_number"`
	WalletProvider string            `json:"wallet_provider"`
	Amount         int64             `json:"amount"`
	Currency       string            `json:"currency"`
	Status         string            `json:"status"`
	ProviderName   string            `json:"provider_name"`
	StakeWalletID  string            `json:"stake_wallet_id,omitempty"`
	ErrorMessage   string            `json:"error_message,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// WithdrawalEvent represents withdrawal-specific events
type WithdrawalEvent struct {
	EventID         string            `json:"event_id"`
	EventType       EventType         `json:"event_type"`
	Timestamp       time.Time         `json:"timestamp"`
	TransactionID   string            `json:"transaction_id"`
	Reference       string            `json:"reference"`
	UserID          string            `json:"user_id"`
	WalletNumber    string            `json:"wallet_number"`
	WalletProvider  string            `json:"wallet_provider"`
	Amount          int64             `json:"amount"`
	Currency        string            `json:"currency"`
	Status          string            `json:"status"`
	ProviderName    string            `json:"provider_name"`
	WinningWalletID string            `json:"winning_wallet_id,omitempty"`
	ReservationID   string            `json:"reservation_id,omitempty"`
	ErrorMessage    string            `json:"error_message,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// SagaEvent represents saga-related events
type SagaEvent struct {
	EventID       string            `json:"event_id"`
	EventType     EventType         `json:"event_type"`
	Timestamp     time.Time         `json:"timestamp"`
	SagaID        string            `json:"saga_id"`
	TransactionID string            `json:"transaction_id"`
	Status        string            `json:"status"`
	CurrentStep   int               `json:"current_step"`
	TotalSteps    int               `json:"total_steps"`
	ErrorMessage  string            `json:"error_message,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// ProviderEvent represents provider interaction events
type ProviderEvent struct {
	EventID       string            `json:"event_id"`
	EventType     EventType         `json:"event_type"`
	Timestamp     time.Time         `json:"timestamp"`
	ProviderName  string            `json:"provider_name"`
	Operation     string            `json:"operation"` // DebitWallet, CreditWallet, etc.
	TransactionID string            `json:"transaction_id,omitempty"`
	Reference     string            `json:"reference,omitempty"`
	ResponseTime  int64             `json:"response_time_ms,omitempty"`
	StatusCode    int               `json:"status_code,omitempty"`
	ErrorMessage  string            `json:"error_message,omitempty"`
	CircuitState  string            `json:"circuit_state,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// NewEvent creates a new base event
func NewEvent(eventType EventType, source string, data map[string]interface{}) *Event {
	return &Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Timestamp: time.Now(),
		Source:    source,
		Data:      data,
		Metadata:  make(map[string]string),
	}
}

// NewTransactionEvent creates a new transaction event
func NewTransactionEvent(eventType EventType, transactionID, reference, txType, status string, amount int64, currency, userID, providerName string) *TransactionEvent {
	return &TransactionEvent{
		EventID:       uuid.New().String(),
		EventType:     eventType,
		Timestamp:     time.Now(),
		TransactionID: transactionID,
		Reference:     reference,
		Type:          txType,
		Status:        status,
		Amount:        amount,
		Currency:      currency,
		UserID:        userID,
		ProviderName:  providerName,
		Metadata:      make(map[string]string),
	}
}

// NewDepositEvent creates a new deposit event
func NewDepositEvent(eventType EventType, transactionID, reference, userID, walletNumber, walletProvider string, amount int64, currency, status, providerName string) *DepositEvent {
	return &DepositEvent{
		EventID:        uuid.New().String(),
		EventType:      eventType,
		Timestamp:      time.Now(),
		TransactionID:  transactionID,
		Reference:      reference,
		UserID:         userID,
		WalletNumber:   walletNumber,
		WalletProvider: walletProvider,
		Amount:         amount,
		Currency:       currency,
		Status:         status,
		ProviderName:   providerName,
		Metadata:       make(map[string]string),
	}
}

// NewWithdrawalEvent creates a new withdrawal event
func NewWithdrawalEvent(eventType EventType, transactionID, reference, userID, walletNumber, walletProvider string, amount int64, currency, status, providerName string) *WithdrawalEvent {
	return &WithdrawalEvent{
		EventID:        uuid.New().String(),
		EventType:      eventType,
		Timestamp:      time.Now(),
		TransactionID:  transactionID,
		Reference:      reference,
		UserID:         userID,
		WalletNumber:   walletNumber,
		WalletProvider: walletProvider,
		Amount:         amount,
		Currency:       currency,
		Status:         status,
		ProviderName:   providerName,
		Metadata:       make(map[string]string),
	}
}

// NewSagaEvent creates a new saga event
func NewSagaEvent(eventType EventType, sagaID, transactionID, status string, currentStep, totalSteps int) *SagaEvent {
	return &SagaEvent{
		EventID:       uuid.New().String(),
		EventType:     eventType,
		Timestamp:     time.Now(),
		SagaID:        sagaID,
		TransactionID: transactionID,
		Status:        status,
		CurrentStep:   currentStep,
		TotalSteps:    totalSteps,
		Metadata:      make(map[string]string),
	}
}

// NewProviderEvent creates a new provider event
func NewProviderEvent(eventType EventType, providerName, operation string) *ProviderEvent {
	return &ProviderEvent{
		EventID:      uuid.New().String(),
		EventType:    eventType,
		Timestamp:    time.Now(),
		ProviderName: providerName,
		Operation:    operation,
		Metadata:     make(map[string]string),
	}
}

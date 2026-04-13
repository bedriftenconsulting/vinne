package events

import (
	"encoding/json"
	"time"
)

// Wallet event types
const (
	// Wallet events
	WalletCreated          EventType = "wallet.created"
	WalletCredited         EventType = "wallet.credited"
	WalletDebited          EventType = "wallet.debited"
	WalletTransferInitiated EventType = "wallet.transfer.initiated"
	WalletTransferCompleted EventType = "wallet.transfer.completed"
	WalletTransferFailed    EventType = "wallet.transfer.failed"
	WalletLocked           EventType = "wallet.locked"
	WalletUnlocked         EventType = "wallet.unlocked"
	
	// Commission events
	CommissionCalculated   EventType = "commission.calculated"
	CommissionApplied      EventType = "commission.applied"
	CommissionRateUpdated  EventType = "commission.rate.updated"
)

// WalletType represents the type of wallet
type WalletType string

const (
	AgentStakeWallet     WalletType = "agent_stake"
	RetailerStakeWallet  WalletType = "retailer_stake"
	RetailerWinningWallet WalletType = "retailer_winning"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionCredit   TransactionType = "credit"
	TransactionDebit    TransactionType = "debit"
	TransactionTransfer TransactionType = "transfer"
	TransactionCommission TransactionType = "commission"
)

// WalletData represents wallet information in events
type WalletData struct {
	WalletID     string     `json:"wallet_id"`
	OwnerID      string     `json:"owner_id"`
	WalletType   WalletType `json:"wallet_type"`
	Balance      int64      `json:"balance"`      // In pesewas
	Currency     string     `json:"currency"`
	LastModified time.Time  `json:"last_modified"`
}

// TransactionData represents transaction information in events
type TransactionData struct {
	TransactionID   string          `json:"transaction_id"`
	WalletID        string          `json:"wallet_id"`
	Type            TransactionType `json:"type"`
	Amount          int64           `json:"amount"`           // In pesewas
	BalanceBefore   int64           `json:"balance_before"`   // In pesewas
	BalanceAfter    int64           `json:"balance_after"`    // In pesewas
	Reference       string          `json:"reference"`
	Description     string          `json:"description"`
	CreatedAt       time.Time       `json:"created_at"`
}

// CommissionData represents commission information in events
type CommissionData struct {
	CommissionID    string    `json:"commission_id"`
	TransactionID   string    `json:"transaction_id"`
	AgentID         string    `json:"agent_id"`
	RetailerID      string    `json:"retailer_id,omitempty"`
	BaseAmount      int64     `json:"base_amount"`      // In pesewas
	CommissionRate  int32     `json:"commission_rate"`  // In basis points
	CommissionAmount int64    `json:"commission_amount"` // In pesewas
	TotalAmount     int64     `json:"total_amount"`     // In pesewas (base + commission)
	CalculatedAt    time.Time `json:"calculated_at"`
}

// TransferData represents transfer information in events
type TransferData struct {
	TransferID      string    `json:"transfer_id"`
	FromWalletID    string    `json:"from_wallet_id"`
	ToWalletID      string    `json:"to_wallet_id"`
	Amount          int64     `json:"amount"`           // In pesewas
	CommissionAmount int64    `json:"commission_amount"` // In pesewas
	NetAmount       int64     `json:"net_amount"`       // In pesewas (amount - commission)
	Status          string    `json:"status"`
	InitiatedAt     time.Time `json:"initiated_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

// WalletCreatedEvent represents a wallet creation event
type WalletCreatedEvent struct {
	BaseEvent
	Wallet WalletData `json:"wallet"`
}

// NewWalletCreatedEvent creates a new wallet created event
func NewWalletCreatedEvent(source string, wallet WalletData) *WalletCreatedEvent {
	return &WalletCreatedEvent{
		BaseEvent: NewBaseEvent(WalletCreated, source).WithUserID(wallet.OwnerID),
		Wallet:    wallet,
	}
}

// Marshal serializes the event to JSON
func (e *WalletCreatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// WalletCreditedEvent represents a wallet credit event
type WalletCreditedEvent struct {
	BaseEvent
	Wallet      WalletData      `json:"wallet"`
	Transaction TransactionData `json:"transaction"`
	Commission  *CommissionData `json:"commission,omitempty"`
}

// NewWalletCreditedEvent creates a new wallet credited event
func NewWalletCreditedEvent(source string, wallet WalletData, transaction TransactionData, commission *CommissionData) *WalletCreditedEvent {
	return &WalletCreditedEvent{
		BaseEvent:   NewBaseEvent(WalletCredited, source).WithUserID(wallet.OwnerID),
		Wallet:      wallet,
		Transaction: transaction,
		Commission:  commission,
	}
}

// Marshal serializes the event to JSON
func (e *WalletCreditedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// WalletDebitedEvent represents a wallet debit event
type WalletDebitedEvent struct {
	BaseEvent
	Wallet      WalletData      `json:"wallet"`
	Transaction TransactionData `json:"transaction"`
}

// NewWalletDebitedEvent creates a new wallet debited event
func NewWalletDebitedEvent(source string, wallet WalletData, transaction TransactionData) *WalletDebitedEvent {
	return &WalletDebitedEvent{
		BaseEvent:   NewBaseEvent(WalletDebited, source).WithUserID(wallet.OwnerID),
		Wallet:      wallet,
		Transaction: transaction,
	}
}

// Marshal serializes the event to JSON
func (e *WalletDebitedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// WalletTransferCompletedEvent represents a completed wallet transfer event
type WalletTransferCompletedEvent struct {
	BaseEvent
	Transfer     TransferData    `json:"transfer"`
	FromWallet   WalletData      `json:"from_wallet"`
	ToWallet     WalletData      `json:"to_wallet"`
	Commission   *CommissionData `json:"commission,omitempty"`
}

// NewWalletTransferCompletedEvent creates a new wallet transfer completed event
func NewWalletTransferCompletedEvent(source string, transfer TransferData, fromWallet, toWallet WalletData, commission *CommissionData) *WalletTransferCompletedEvent {
	return &WalletTransferCompletedEvent{
		BaseEvent:   NewBaseEvent(WalletTransferCompleted, source),
		Transfer:    transfer,
		FromWallet:  fromWallet,
		ToWallet:    toWallet,
		Commission:  commission,
	}
}

// Marshal serializes the event to JSON
func (e *WalletTransferCompletedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// CommissionCalculatedEvent represents a commission calculation event
type CommissionCalculatedEvent struct {
	BaseEvent
	Commission CommissionData `json:"commission"`
}

// NewCommissionCalculatedEvent creates a new commission calculated event
func NewCommissionCalculatedEvent(source string, commission CommissionData) *CommissionCalculatedEvent {
	return &CommissionCalculatedEvent{
		BaseEvent:  NewBaseEvent(CommissionCalculated, source).WithUserID(commission.AgentID),
		Commission: commission,
	}
}

// Marshal serializes the event to JSON
func (e *CommissionCalculatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// CommissionRateUpdatedEvent represents a commission rate update event
type CommissionRateUpdatedEvent struct {
	BaseEvent
	AgentID    string    `json:"agent_id"`
	OldRate    int32     `json:"old_rate"`     // In basis points
	NewRate    int32     `json:"new_rate"`     // In basis points
	EffectiveAt time.Time `json:"effective_at"`
	UpdatedBy   string    `json:"updated_by"`
}

// NewCommissionRateUpdatedEvent creates a new commission rate updated event
func NewCommissionRateUpdatedEvent(source string, agentID string, oldRate, newRate int32, effectiveAt time.Time, updatedBy string) *CommissionRateUpdatedEvent {
	return &CommissionRateUpdatedEvent{
		BaseEvent:   NewBaseEvent(CommissionRateUpdated, source).WithUserID(agentID),
		AgentID:     agentID,
		OldRate:     oldRate,
		NewRate:     newRate,
		EffectiveAt: effectiveAt,
		UpdatedBy:   updatedBy,
	}
}

// Marshal serializes the event to JSON
func (e *CommissionRateUpdatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}
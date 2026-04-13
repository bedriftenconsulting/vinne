package models

import (
	"database/sql/driver"
	"time"

	"github.com/google/uuid"
)

// WalletType represents the type of wallet
type WalletType string

const (
	WalletTypeAgentStake      WalletType = "AGENT_STAKE"
	WalletTypeRetailerStake   WalletType = "RETAILER_STAKE"
	WalletTypeRetailerWinning WalletType = "RETAILER_WINNING"
	WalletTypePlayerWallet    WalletType = "PLAYER_WALLET"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionTypeCredit     TransactionType = "CREDIT"
	TransactionTypeDebit      TransactionType = "DEBIT"
	TransactionTypeTransfer   TransactionType = "TRANSFER"
	TransactionTypeCommission TransactionType = "COMMISSION"
	TransactionTypePayout     TransactionType = "PAYOUT"
)

// TransactionStatus represents the status of a transaction
type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "PENDING"
	TransactionStatusCompleted TransactionStatus = "COMPLETED"
	TransactionStatusFailed    TransactionStatus = "FAILED"
	TransactionStatusReversed  TransactionStatus = "REVERSED"
)

// WalletStatus represents the status of a wallet
type WalletStatus string

const (
	WalletStatusActive    WalletStatus = "ACTIVE"
	WalletStatusInactive  WalletStatus = "INACTIVE"
	WalletStatusLocked    WalletStatus = "LOCKED"
	WalletStatusSuspended WalletStatus = "SUSPENDED"
)

// WalletHoldStatus represents the wallet hold status
type WalletHoldStatus string

const (
	WalletHoldStatusActive   WalletHoldStatus = "ACTIVE"
	WalletHoldStatusReleased WalletHoldStatus = "RELEASED"
	WalletHoldStatusExpired  WalletHoldStatus = "EXPIRED"
)

// CommissionType represents the type of commission
type CommissionType string

const (
	CommissionTypeDeposit  CommissionType = "DEPOSIT"
	CommissionTypeTransfer CommissionType = "TRANSFER"
	CommissionTypeStake    CommissionType = "STAKE"
	CommissionTypePayout   CommissionType = "PAYOUT"
	CommissionTypeBonus    CommissionType = "BONUS"
)

// CreditSource represents the source of a wallet credit
type CreditSource string

const (
	CreditSourceAdminDirect      CreditSource = "admin_direct"
	CreditSourceMobileMoney      CreditSource = "mobile_money"
	CreditSourceBankTransfer     CreditSource = "bank_transfer"
	CreditSourceSystemAdjustment CreditSource = "system_adjustment"
	CreditSourceReversal         CreditSource = "reversal"
)

// CommissionStatus represents the status of a commission
type CommissionStatus string

const (
	CommissionStatusPending  CommissionStatus = "PENDING"
	CommissionStatusCredited CommissionStatus = "CREDITED"
	CommissionStatusReversed CommissionStatus = "REVERSED"
)

// AgentStakeWallet represents an agent's stake wallet
type AgentStakeWallet struct {
	ID                uuid.UUID    `json:"id" db:"id"`
	AgentID           uuid.UUID    `json:"agent_id" db:"agent_id"`
	Balance           int64        `json:"balance" db:"balance"`                     // in pesewas
	PendingBalance    int64        `json:"pending_balance" db:"pending_balance"`     // in pesewas
	AvailableBalance  int64        `json:"available_balance" db:"available_balance"` // in pesewas
	Currency          string       `json:"currency" db:"currency"`
	Status            WalletStatus `json:"status" db:"status"`
	LastTransactionAt *time.Time   `json:"last_transaction_at" db:"last_transaction_at"`
	CreatedAt         time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at" db:"updated_at"`
}

// RetailerStakeWallet represents a retailer's stake wallet
type RetailerStakeWallet struct {
	ID                uuid.UUID    `json:"id" db:"id"`
	RetailerID        uuid.UUID    `json:"retailer_id" db:"retailer_id"`
	Balance           int64        `json:"balance" db:"balance"`                     // in pesewas
	PendingBalance    int64        `json:"pending_balance" db:"pending_balance"`     // in pesewas
	AvailableBalance  int64        `json:"available_balance" db:"available_balance"` // in pesewas
	Currency          string       `json:"currency" db:"currency"`
	Status            WalletStatus `json:"status" db:"status"`
	LastTransactionAt *time.Time   `json:"last_transaction_at" db:"last_transaction_at"`
	CreatedAt         time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at" db:"updated_at"`
}

// RetailerWinningWallet represents a retailer's winning wallet
type RetailerWinningWallet struct {
	ID                uuid.UUID    `json:"id" db:"id"`
	RetailerID        uuid.UUID    `json:"retailer_id" db:"retailer_id"`
	Balance           int64        `json:"balance" db:"balance"`
	PendingBalance    int64        `json:"pending_balance" db:"pending_balance"`
	AvailableBalance  int64        `json:"available_balance" db:"available_balance"`
	Currency          string       `json:"currency" db:"currency"`
	Status            WalletStatus `json:"status" db:"status"`
	LastTransactionAt *time.Time   `json:"last_transaction_at" db:"last_transaction_at"`
	CreatedAt         time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at" db:"updated_at"`
}

type RetailerWinningWalletHold struct {
	ID         uuid.UUID        `json:"id" db:"id"`
	WalletID   uuid.UUID        `json:"wallet_id" db:"wallet_id"`
	RetailerID uuid.UUID        `json:"retailer_id" db:"retailer_id"`
	PlacedBy   uuid.UUID        `json:"placed_by" db:"placed_by"`
	Reason     string           `json:"reason" db:"reason"`
	Status     WalletHoldStatus `json:"status" db:"status"`
	CreatedAt  time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at" db:"updated_at"`
	ExpiresAt  time.Time        `json:"expires_at" db:"expires_at"`
}

type PlayerWallet struct {
	ID                uuid.UUID    `json:"id" db:"id"`
	PlayerID          uuid.UUID    `json:"player_id" db:"player_id"`
	Balance           int64        `json:"balance" db:"balance"`
	PendingBalance    int64        `json:"pending_balance" db:"pending_balance"`
	AvailableBalance  int64        `json:"available_balance" db:"available_balance"`
	Currency          string       `json:"currency" db:"currency"`
	Status            WalletStatus `json:"status" db:"status"`
	LastTransactionAt *time.Time   `json:"last_transaction_at" db:"last_transaction_at"`
	CreatedAt         time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at" db:"updated_at"`
}

// WalletTransaction represents a wallet transaction
type WalletTransaction struct {
	ID                      uuid.UUID              `json:"id" db:"id"`
	TransactionID           string                 `json:"transaction_id" db:"transaction_id"`
	WalletOwnerID           uuid.UUID              `json:"wallet_owner_id" db:"wallet_owner_id"`
	WalletType              WalletType             `json:"wallet_type" db:"wallet_type"`
	TransactionType         TransactionType        `json:"transaction_type" db:"transaction_type"`
	Amount                  int64                  `json:"amount" db:"amount"`
	BalanceBefore           int64                  `json:"balance_before" db:"balance_before"`
	BalanceAfter            int64                  `json:"balance_after" db:"balance_after"`
	Reference               *string                `json:"reference" db:"reference"`
	Description             *string                `json:"description" db:"description"`
	Status                  TransactionStatus      `json:"status" db:"status"`
	CreditSource            CreditSource           `json:"credit_source" db:"credit_source"`
	IdempotencyKey          *string                `json:"idempotency_key" db:"idempotency_key"`
	Metadata                map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt               time.Time              `json:"created_at" db:"created_at"`
	CompletedAt             *time.Time             `json:"completed_at" db:"completed_at"`
	ReversedAt              *time.Time             `json:"reversed_at" db:"reversed_at"`
	ReversedByTransactionID *uuid.UUID             `json:"reversed_by_transaction_id" db:"reversed_by_transaction_id"`
	ReversalReason          *string                `json:"reversal_reason" db:"reversal_reason"`
}

// WalletTransfer represents a wallet-to-wallet transfer
type WalletTransfer struct {
	ID               uuid.UUID         `json:"id" db:"id"`
	TransferID       string            `json:"transfer_id" db:"transfer_id"`
	FromWalletID     uuid.UUID         `json:"from_wallet_id" db:"from_wallet_id"`
	FromWalletType   WalletType        `json:"from_wallet_type" db:"from_wallet_type"`
	ToWalletID       uuid.UUID         `json:"to_wallet_id" db:"to_wallet_id"`
	ToWalletType     WalletType        `json:"to_wallet_type" db:"to_wallet_type"`
	Amount           int64             `json:"amount" db:"amount"`
	CommissionAmount int64             `json:"commission_amount" db:"commission_amount"`
	TotalDeducted    int64             `json:"total_deducted" db:"total_deducted"`
	Reference        *string           `json:"reference" db:"reference"`
	Notes            *string           `json:"notes" db:"notes"`
	Status           TransactionStatus `json:"status" db:"status"`
	IdempotencyKey   *string           `json:"idempotency_key" db:"idempotency_key"`
	CreatedAt        time.Time         `json:"created_at" db:"created_at"`
	CompletedAt      *time.Time        `json:"completed_at" db:"completed_at"`
	ReversedAt       *time.Time        `json:"reversed_at" db:"reversed_at"`
}

// WalletLock represents a lock on a wallet
type WalletLock struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	WalletID   uuid.UUID  `json:"wallet_id" db:"wallet_id"`
	WalletType WalletType `json:"wallet_type" db:"wallet_type"`
	LockReason string     `json:"lock_reason" db:"lock_reason"`
	LockedBy   *string    `json:"locked_by" db:"locked_by"`
	LockedAt   time.Time  `json:"locked_at" db:"locked_at"`
	ExpiresAt  time.Time  `json:"expires_at" db:"expires_at"`
	ReleasedAt *time.Time `json:"released_at" db:"released_at"`
}

// ReservationStatus represents the status of a wallet reservation
type ReservationStatus string

const (
	ReservationStatusActive    ReservationStatus = "ACTIVE"
	ReservationStatusCommitted ReservationStatus = "COMMITTED"
	ReservationStatusReleased  ReservationStatus = "RELEASED"
	ReservationStatusExpired   ReservationStatus = "EXPIRED"
)

// WalletReservation represents a temporary fund reservation (for two-phase commit)
type WalletReservation struct {
	ID             uuid.UUID         `json:"id" db:"id"`
	ReservationID  string            `json:"reservation_id" db:"reservation_id"` // Unique reservation reference
	WalletOwnerID  uuid.UUID         `json:"wallet_owner_id" db:"wallet_owner_id"`
	WalletType     WalletType        `json:"wallet_type" db:"wallet_type"`
	Amount         int64             `json:"amount" db:"amount"`
	Reference      string            `json:"reference" db:"reference"` // Transaction reference
	Reason         string            `json:"reason" db:"reason"`       // e.g., "Pending withdrawal to mobile money"
	Status         ReservationStatus `json:"status" db:"status"`
	TransactionID  *uuid.UUID        `json:"transaction_id" db:"transaction_id"` // Final transaction ID after commit
	IdempotencyKey *string           `json:"idempotency_key" db:"idempotency_key"`
	CreatedAt      time.Time         `json:"created_at" db:"created_at"`
	ExpiresAt      time.Time         `json:"expires_at" db:"expires_at"`
	CommittedAt    *time.Time        `json:"committed_at" db:"committed_at"`
	ReleasedAt     *time.Time        `json:"released_at" db:"released_at"`
}

// Value implements driver.Valuer for ReservationStatus
func (r ReservationStatus) Value() (driver.Value, error) {
	return string(r), nil
}

// AgentCommissionRate represents an agent's commission rate
type AgentCommissionRate struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	AgentID       uuid.UUID  `json:"agent_id" db:"agent_id"`
	Rate          int32      `json:"rate" db:"rate"` // basis points (3000 = 30%)
	EffectiveFrom time.Time  `json:"effective_from" db:"effective_from"`
	EffectiveTo   *time.Time `json:"effective_to" db:"effective_to"`
	Notes         *string    `json:"notes" db:"notes"`
	CreatedBy     uuid.UUID  `json:"created_by" db:"created_by"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// CommissionTransaction represents a commission transaction
type CommissionTransaction struct {
	ID               uuid.UUID        `json:"id" db:"id"`
	CommissionID     string           `json:"commission_id" db:"commission_id"`
	TransactionID    uuid.UUID        `json:"transaction_id" db:"transaction_id"`
	AgentID          uuid.UUID        `json:"agent_id" db:"agent_id"`
	OriginalAmount   int64            `json:"original_amount" db:"original_amount"`
	GrossAmount      int64            `json:"gross_amount" db:"gross_amount"`
	CommissionAmount int64            `json:"commission_amount" db:"commission_amount"` // in pesewas
	CommissionRate   int32            `json:"commission_rate" db:"commission_rate"`     // basis points
	CommissionType   CommissionType   `json:"commission_type" db:"commission_type"`
	Status           CommissionStatus `json:"status" db:"status"`
	Reference        *string          `json:"reference" db:"reference"`
	Notes            *string          `json:"notes" db:"notes"`
	CreatedAt        time.Time        `json:"created_at" db:"created_at"`
	CreditedAt       *time.Time       `json:"credited_at" db:"credited_at"`
	ReversedAt       *time.Time       `json:"reversed_at" db:"reversed_at"`
	UpdatedAt        time.Time        `json:"updated_at" db:"updated_at"`
}

// CommissionCalculation represents a commission calculation for audit
type CommissionCalculation struct {
	ID               uuid.UUID              `json:"id" db:"id"`
	AgentID          uuid.UUID              `json:"agent_id" db:"agent_id"`
	TransactionID    uuid.UUID              `json:"transaction_id" db:"transaction_id"`
	TransactionType  CommissionType         `json:"transaction_type" db:"transaction_type"`
	CalculationType  string                 `json:"calculation_type" db:"calculation_type"`
	InputAmount      int64                  `json:"input_amount" db:"input_amount"`           // in pesewas
	CommissionRate   int32                  `json:"commission_rate" db:"commission_rate"`     // basis points
	RateBasisPoints  int32                  `json:"rate_basis_points" db:"rate_basis_points"` // basis points
	GrossAmount      int64                  `json:"gross_amount" db:"gross_amount"`           // in pesewas
	CommissionAmount int64                  `json:"commission_amount" db:"commission_amount"` // in pesewas
	NetAmount        int64                  `json:"net_amount" db:"net_amount"`               // in pesewas
	FormulaUsed      string                 `json:"formula_used" db:"formula_used"`
	Metadata         map[string]interface{} `json:"metadata" db:"metadata"`
	CalculatedAt     time.Time              `json:"calculated_at" db:"calculated_at"`
	CreatedAt        time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at" db:"updated_at"`
}

// CommissionAudit represents an audit entry for commission changes
type CommissionAudit struct {
	ID               uuid.UUID              `json:"id" db:"id"`
	AgentID          uuid.UUID              `json:"agent_id" db:"agent_id"`
	TransactionID    uuid.UUID              `json:"transaction_id" db:"transaction_id"`
	CommissionAmount int64                  `json:"commission_amount" db:"commission_amount"`
	Action           string                 `json:"action" db:"action"`
	ActionBy         string                 `json:"action_by" db:"action_by"`
	ActionAt         time.Time              `json:"action_at" db:"action_at"`
	Details          string                 `json:"details" db:"details"`
	OldValue         map[string]interface{} `json:"old_value" db:"old_value"`
	NewValue         map[string]interface{} `json:"new_value" db:"new_value"`
	Reason           *string                `json:"reason" db:"reason"`
	CreatedAt        time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at" db:"updated_at"`
}

// Value implements driver.Valuer for WalletType
func (w WalletType) Value() (driver.Value, error) {
	return string(w), nil
}

// Value implements driver.Valuer for TransactionType
func (t TransactionType) Value() (driver.Value, error) {
	return string(t), nil
}

// Value implements driver.Valuer for TransactionStatus
func (t TransactionStatus) Value() (driver.Value, error) {
	return string(t), nil
}

// Value implements driver.Valuer for WalletStatus
func (w WalletStatus) Value() (driver.Value, error) {
	return string(w), nil
}

// Value implements driver.Valuer for CommissionType
func (c CommissionType) Value() (driver.Value, error) {
	return string(c), nil
}

// Value implements driver.Valuer for CommissionStatus
func (c CommissionStatus) Value() (driver.Value, error) {
	return string(c), nil
}

// Value implements driver.Valuer for CreditSource
func (c CreditSource) Value() (driver.Value, error) {
	return string(c), nil
}

// Money conversion helpers

// PesewasToGHS converts pesewas (int64) to Ghana Cedis (float64)
func PesewasToGHS(pesewas int64) float64 {
	return float64(pesewas) / 100.0
}

// GHSToPesewas converts Ghana Cedis (float64) to pesewas (int64)
func GHSToPesewas(ghs float64) int64 {
	return int64(ghs * 100)
}

// BasisPointsToPercentage converts basis points to percentage
func BasisPointsToPercentage(basisPoints int32) float64 {
	return float64(basisPoints) / 100.0
}

// PercentageToBasisPoints converts percentage to basis points
func PercentageToBasisPoints(percentage float64) int32 {
	return int32(percentage * 100)
}

// CalculateCommission calculates commission amount based on basis points
func CalculateCommission(amountPesewas int64, rateBasisPoints int32) int64 {
	return (amountPesewas * int64(rateBasisPoints)) / 10000
}

// CalculateGrossAmount calculates gross amount including commission
func CalculateGrossAmount(netAmountPesewas int64, rateBasisPoints int32) int64 {
	// Gross = Net / (1 - rate)
	// For 30% commission: Gross = Net / 0.70
	return (netAmountPesewas * 10000) / (10000 - int64(rateBasisPoints))
}

// CommissionReport represents a commission report for an agent
type CommissionReport struct {
	AgentID            uuid.UUID `json:"agent_id"`
	StartDate          time.Time `json:"start_date"`
	EndDate            time.Time `json:"end_date"`
	TotalCommission    int64     `json:"total_commission"`
	TotalTransactions  int       `json:"total_transactions"`
	DepositCommission  int64     `json:"deposit_commission"`
	TransferCommission int64     `json:"transfer_commission"`
	DepositCount       int       `json:"deposit_count"`
	TransferCount      int       `json:"transfer_count"`
	GeneratedAt        time.Time `json:"generated_at"`
}

// IdempotencyKey represents an idempotency key for preventing duplicate operations
type IdempotencyKey struct {
	IdempotencyKey string    `json:"idempotency_key" db:"idempotency_key"`
	TransactionID  uuid.UUID `json:"transaction_id" db:"transaction_id"`
	WalletOwnerID  uuid.UUID `json:"wallet_owner_id" db:"wallet_owner_id"`
	WalletType     string    `json:"wallet_type" db:"wallet_type"`
	OperationType  string    `json:"operation_type" db:"operation_type"` // 'credit', 'debit'
	Amount         int64     `json:"amount" db:"amount"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

// TransactionReversal represents a reversal of a wallet transaction
type TransactionReversal struct {
	ID                    uuid.UUID              `json:"id" db:"id"`
	OriginalTransactionID uuid.UUID              `json:"original_transaction_id" db:"original_transaction_id"`
	ReversalTransactionID *uuid.UUID             `json:"reversal_transaction_id" db:"reversal_transaction_id"`
	OriginalAmount        int64                  `json:"original_amount" db:"original_amount"` // in pesewas
	WalletOwnerID         uuid.UUID              `json:"wallet_owner_id" db:"wallet_owner_id"`
	WalletType            WalletType             `json:"wallet_type" db:"wallet_type"`
	Reason                string                 `json:"reason" db:"reason"`
	ReversedBy            uuid.UUID              `json:"reversed_by" db:"reversed_by"`
	ReversedByName        *string                `json:"reversed_by_name" db:"reversed_by_name"`
	ReversedByEmail       *string                `json:"reversed_by_email" db:"reversed_by_email"`
	ReversedAt            time.Time              `json:"reversed_at" db:"reversed_at"`
	Metadata              map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt             time.Time              `json:"created_at" db:"created_at"`
}

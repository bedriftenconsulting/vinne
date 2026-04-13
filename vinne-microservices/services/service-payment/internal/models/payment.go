package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PaymentStatus represents the status of a payment
type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "PENDING"
	PaymentStatusProcessing PaymentStatus = "PROCESSING"
	PaymentStatusCompleted  PaymentStatus = "COMPLETED"
	PaymentStatusFailed     PaymentStatus = "FAILED"
	PaymentStatusCancelled  PaymentStatus = "CANCELLED"
	PaymentStatusRefunded   PaymentStatus = "REFUNDED"
)

// PaymentType represents the type of payment
type PaymentType string

const (
	PaymentTypeWalletCredit      PaymentType = "WALLET_CREDIT"
	PaymentTypeAgentTopup        PaymentType = "AGENT_TOPUP"
	PaymentTypeRetailerPayout    PaymentType = "RETAILER_PAYOUT"
	PaymentTypeGamePurchase      PaymentType = "GAME_PURCHASE"
	PaymentTypePrizePayout       PaymentType = "PRIZE_PAYOUT"
	PaymentTypeCommissionPayment PaymentType = "COMMISSION_PAYMENT"
)

// PaymentMethodType represents the type of payment method
type PaymentMethodType string

const (
	PaymentMethodMobileMoney  PaymentMethodType = "MOBILE_MONEY"
	PaymentMethodBankTransfer PaymentMethodType = "BANK_TRANSFER"
	PaymentMethodCash         PaymentMethodType = "CASH"
	PaymentMethodCard         PaymentMethodType = "CARD"
)

// MoMoProvider represents mobile money providers
type MoMoProvider string

const (
	MoMoProviderMTN        MoMoProvider = "MTN_MOMO"
	MoMoProviderTelecel    MoMoProvider = "TELECEL_CASH"
	MoMoProviderAirtelTigo MoMoProvider = "AIRTELTIGO_MONEY"
)

// Payment represents a payment transaction
type Payment struct {
	ID                uuid.UUID         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Reference         string            `gorm:"type:varchar(100);uniqueIndex;not null" json:"reference"`
	Type              PaymentType       `gorm:"type:varchar(50);not null" json:"type"`
	Status            PaymentStatus     `gorm:"type:varchar(20);not null;default:'PENDING'" json:"status"`
	Amount            int64             `gorm:"not null" json:"amount"` // Amount in pesewas
	Currency          string            `gorm:"type:varchar(3);not null;default:'GHS'" json:"currency"`
	Description       string            `gorm:"type:text" json:"description"`
	PayerID           string            `gorm:"type:varchar(255);not null" json:"payer_id"`
	PayeeID           string            `gorm:"type:varchar(255)" json:"payee_id"`
	MethodType        PaymentMethodType `gorm:"type:varchar(50);not null" json:"method_type"`
	MethodID          *string           `gorm:"type:varchar(255)" json:"method_id"`
	ExternalReference string            `gorm:"type:varchar(255)" json:"external_reference"`
	ProviderResponse  string            `gorm:"type:jsonb" json:"provider_response"`
	Metadata          map[string]string `gorm:"type:jsonb" json:"metadata"`
	ProcessedAt       *time.Time        `json:"processed_at"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	DeletedAt         gorm.DeletedAt    `gorm:"index" json:"-"`

	// Relations
	PaymentMethod *PaymentMethod `gorm:"foreignKey:MethodID" json:"payment_method,omitempty"`
	PaymentLogs   []PaymentLog   `gorm:"foreignKey:PaymentID" json:"payment_logs,omitempty"`
}

// TableName specifies the table name for Payment
func (Payment) TableName() string {
	return "payments"
}

// PaymentMethod represents a user's payment method
type PaymentMethod struct {
	ID        uuid.UUID         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    string            `gorm:"type:varchar(255);not null;index" json:"user_id"`
	Type      PaymentMethodType `gorm:"type:varchar(50);not null" json:"type"`
	Name      string            `gorm:"type:varchar(255);not null" json:"name"`
	IsActive  bool              `gorm:"not null;default:true" json:"is_active"`
	IsDefault bool              `gorm:"not null;default:false" json:"is_default"`
	Details   map[string]string `gorm:"type:jsonb" json:"details"` // Store method-specific details
	Metadata  map[string]string `gorm:"type:jsonb" json:"metadata"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	DeletedAt gorm.DeletedAt    `gorm:"index" json:"-"`
}

// TableName specifies the table name for PaymentMethod
func (PaymentMethod) TableName() string {
	return "payment_methods"
}

// BeforeCreate ensures only one default payment method per user and type
func (pm *PaymentMethod) BeforeCreate(tx *gorm.DB) error {
	if pm.IsDefault {
		// Remove default flag from other methods of same user and type
		return tx.Model(&PaymentMethod{}).
			Where("user_id = ? AND type = ? AND is_default = ? AND id != ?", pm.UserID, pm.Type, true, pm.ID).
			Update("is_default", false).Error
	}
	return nil
}

// PaymentLog represents audit logs for payment operations
type PaymentLog struct {
	ID          uuid.UUID         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	PaymentID   uuid.UUID         `gorm:"type:uuid;not null;index" json:"payment_id"`
	Payment     *Payment          `gorm:"foreignKey:PaymentID" json:"payment,omitempty"`
	Action      string            `gorm:"type:varchar(100);not null" json:"action"`
	OldStatus   PaymentStatus     `gorm:"type:varchar(20)" json:"old_status"`
	NewStatus   PaymentStatus     `gorm:"type:varchar(20)" json:"new_status"`
	Details     map[string]string `gorm:"type:jsonb" json:"details"`
	PerformedBy string            `gorm:"type:varchar(255)" json:"performed_by"`
	IPAddress   string            `gorm:"type:varchar(45)" json:"ip_address"`
	UserAgent   string            `gorm:"type:text" json:"user_agent"`
	CreatedAt   time.Time         `json:"created_at"`
}

// TableName specifies the table name for PaymentLog
func (PaymentLog) TableName() string {
	return "payment_logs"
}

// PaymentReconciliation represents payment reconciliation records
type PaymentReconciliation struct {
	ID              uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	PaymentID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"payment_id"`
	Payment         *Payment   `gorm:"foreignKey:PaymentID" json:"payment,omitempty"`
	Provider        string     `gorm:"type:varchar(100);not null" json:"provider"`
	ProviderTxnID   string     `gorm:"type:varchar(255)" json:"provider_txn_id"`
	ProviderStatus  string     `gorm:"type:varchar(50)" json:"provider_status"`
	ProviderAmount  int64      `json:"provider_amount"`
	ReconcileStatus string     `gorm:"type:varchar(50);not null;default:'PENDING'" json:"reconcile_status"`
	Discrepancy     string     `gorm:"type:text" json:"discrepancy"`
	ResolvedAt      *time.Time `json:"resolved_at"`
	ResolvedBy      string     `gorm:"type:varchar(255)" json:"resolved_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// TableName specifies the table name for PaymentReconciliation
func (PaymentReconciliation) TableName() string {
	return "payment_reconciliation"
}

// MoMoTransaction represents mobile money transaction details
type MoMoTransaction struct {
	ID            uuid.UUID         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	PaymentID     uuid.UUID         `gorm:"type:uuid;not null;index" json:"payment_id"`
	Payment       *Payment          `gorm:"foreignKey:PaymentID" json:"payment,omitempty"`
	Provider      MoMoProvider      `gorm:"type:varchar(50);not null" json:"provider"`
	PhoneNumber   string            `gorm:"type:varchar(15);not null" json:"phone_number"`
	AccountName   string            `gorm:"type:varchar(255)" json:"account_name"`
	TransactionID string            `gorm:"type:varchar(255);index" json:"transaction_id"`
	Status        string            `gorm:"type:varchar(50);not null" json:"status"`
	StatusMessage string            `gorm:"type:text" json:"status_message"`
	USSDCode      string            `gorm:"type:varchar(20)" json:"ussd_code"`
	RequestData   map[string]string `gorm:"type:jsonb" json:"request_data"`
	ResponseData  map[string]string `gorm:"type:jsonb" json:"response_data"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// TableName specifies the table name for MoMoTransaction
func (MoMoTransaction) TableName() string {
	return "momo_transactions"
}

// BankTransfer represents bank transfer transaction details
type BankTransfer struct {
	ID                uuid.UUID         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	PaymentID         uuid.UUID         `gorm:"type:uuid;not null;index" json:"payment_id"`
	Payment           *Payment          `gorm:"foreignKey:PaymentID" json:"payment,omitempty"`
	BankCode          string            `gorm:"type:varchar(10);not null" json:"bank_code"`
	BankName          string            `gorm:"type:varchar(255);not null" json:"bank_name"`
	AccountNumber     string            `gorm:"type:varchar(20);not null" json:"account_number"`
	AccountName       string            `gorm:"type:varchar(255);not null" json:"account_name"`
	BranchCode        string            `gorm:"type:varchar(10)" json:"branch_code"`
	TransferReference string            `gorm:"type:varchar(255);index" json:"transfer_reference"`
	Status            string            `gorm:"type:varchar(50);not null" json:"status"`
	StatusMessage     string            `gorm:"type:text" json:"status_message"`
	Instructions      string            `gorm:"type:text" json:"instructions"`
	RequestData       map[string]string `gorm:"type:jsonb" json:"request_data"`
	ResponseData      map[string]string `gorm:"type:jsonb" json:"response_data"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// TableName specifies the table name for BankTransfer
func (BankTransfer) TableName() string {
	return "bank_transfers"
}

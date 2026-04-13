package providers

import (
	"context"
	"time"
)

// PaymentProvider defines the interface that all payment providers must implement
type PaymentProvider interface {
	// Provider identification
	GetProviderName() string
	GetProviderType() ProviderType

	// Authentication
	Authenticate(ctx context.Context) (*AuthResult, error)
	RefreshAuth(ctx context.Context) error
	IsAuthenticated() bool

	// Account Verification (Enquiry)
	VerifyWallet(ctx context.Context, req *VerifyWalletRequest) (*VerifyWalletResponse, error)
	VerifyBankAccount(ctx context.Context, req *VerifyBankAccountRequest) (*VerifyBankAccountResponse, error)
	VerifyIdentity(ctx context.Context, req *VerifyIdentityRequest) (*VerifyIdentityResponse, error)

	// Transaction Operations (Transfer)
	DebitWallet(ctx context.Context, req *DebitWalletRequest) (*TransactionResponse, error)
	CreditWallet(ctx context.Context, req *CreditWalletRequest) (*TransactionResponse, error)
	TransferToBank(ctx context.Context, req *BankTransferRequest) (*TransactionResponse, error)

	// Transaction Status
	CheckTransactionStatus(ctx context.Context, transactionID, reference string) (*TransactionStatusResponse, error)

	// Provider Metadata
	GetSupportedOperations() []OperationType
	GetTransactionLimits() *TransactionLimits
	GetSupportedCurrencies() []string

	// Health Check
	HealthCheck(ctx context.Context) error
}

// ProviderType represents the category of payment provider
type ProviderType string

const (
	ProviderTypeMobileMoney  ProviderType = "MOBILE_MONEY"
	ProviderTypeBankTransfer ProviderType = "BANK_TRANSFER"
	ProviderTypeCard         ProviderType = "CARD"
	ProviderTypeUSSD         ProviderType = "USSD"
	ProviderTypeAggregator   ProviderType = "AGGREGATOR" // Orange is an aggregator
)

// OperationType represents supported operations
type OperationType string

const (
	OpWalletVerify      OperationType = "WALLET_VERIFY"
	OpBankAccountVerify OperationType = "BANK_ACCOUNT_VERIFY"
	OpIdentityVerify    OperationType = "IDENTITY_VERIFY"
	OpWalletDebit       OperationType = "WALLET_DEBIT"
	OpWalletCredit      OperationType = "WALLET_CREDIT"
	OpBankTransfer      OperationType = "BANK_TRANSFER"
	OpStatusCheck       OperationType = "STATUS_CHECK"
)

// TransactionLimits defines provider-specific limits
type TransactionLimits struct {
	MinAmount    float64
	MaxAmount    float64
	DailyLimit   float64
	MonthlyLimit float64
	Currency     string
}

// AuthResult contains authentication data
type AuthResult struct {
	Token        string
	RefreshToken string
	ExpiresAt    time.Time
	ProviderData map[string]interface{}
}

// VerifyWalletRequest standardizes wallet verification
type VerifyWalletRequest struct {
	WalletNumber   string
	WalletProvider string // MTN, TELECEL, AIRTELTIGO
	Reference      string
}

// VerifyWalletResponse standardizes verification results
type VerifyWalletResponse struct {
	IsValid        bool
	AccountName    string
	WalletNumber   string
	WalletProvider string
	Reference      string
	ProviderData   map[string]interface{}
}

// VerifyBankAccountRequest standardizes bank account verification
type VerifyBankAccountRequest struct {
	AccountNumber string
	BankCode      string // Sort code or bank identifier
	Reference     string
}

// VerifyBankAccountResponse standardizes bank verification results
type VerifyBankAccountResponse struct {
	IsValid       bool
	AccountName   string
	AccountNumber string
	BankCode      string
	BankName      string
	Reference     string
	ProviderData  map[string]interface{}
}

// VerifyIdentityRequest standardizes KYC verification
type VerifyIdentityRequest struct {
	IdentityType   string // GHANA_CARD, PASSPORT, DRIVERS_LICENSE
	IdentityNumber string
	FullKYC        bool // true for full KYC, false for basic verification
	Reference      string
}

// VerifyIdentityResponse standardizes identity verification results
type VerifyIdentityResponse struct {
	IsValid        bool
	FullName       string
	DateOfBirth    *time.Time
	Nationality    string
	IdentityNumber string
	CardValidFrom  *time.Time
	CardValidTo    *time.Time
	Reference      string
	ProviderData   map[string]interface{} // Full KYC data if requested
}

// DebitWalletRequest standardizes wallet debit operations
type DebitWalletRequest struct {
	WalletNumber    string
	WalletProvider  string
	Amount          float64
	Currency        string
	Narration       string
	Reference       string // Client unique reference
	CustomerName    string
	CustomerRemarks string
	Metadata        map[string]string
}

// CreditWalletRequest standardizes wallet credit operations
type CreditWalletRequest struct {
	WalletNumber    string
	WalletProvider  string
	Amount          float64
	Currency        string
	Narration       string
	Reference       string
	CustomerName    string
	CustomerRemarks string
	Metadata        map[string]string
}

// BankTransferRequest standardizes bank-to-bank transfers
type BankTransferRequest struct {
	AccountNumber   string
	BankCode        string
	Amount          float64
	Currency        string
	Narration       string
	Reference       string
	BeneficiaryName string
	CustomerRemarks string
	Metadata        map[string]string
}

// TransactionResponse standardizes transaction results
type TransactionResponse struct {
	Success       bool
	TransactionID string // Provider's transaction ID
	Reference     string // Client reference
	Status        TransactionStatus
	Amount        float64
	Currency      string
	Beneficiary   string
	RequestedAt   time.Time
	CompletedAt   *time.Time
	Message       string
	ProviderData  map[string]interface{}
}

// TransactionStatus represents transaction states
type TransactionStatus string

const (
	StatusPending   TransactionStatus = "PENDING"
	StatusSuccess   TransactionStatus = "SUCCESS"
	StatusFailed    TransactionStatus = "FAILED"
	StatusDuplicate TransactionStatus = "DUPLICATE"
)

// TransactionStatusResponse standardizes status check results
type TransactionStatusResponse struct {
	TransactionID      string
	Reference          string
	Status             TransactionStatus
	ProviderStatusCode string // Raw status code from provider (e.g., "1" for Orange success)
	Amount             float64
	Beneficiary        string
	RequestedAt        *time.Time // Nullable - when transaction was initiated
	CompletedAt        *time.Time // Nullable - when transaction completed
	Message            string
	ProviderData       map[string]interface{}
}

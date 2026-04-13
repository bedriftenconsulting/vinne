package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Ticket represents a lottery ticket
type Ticket struct {
	ID           uuid.UUID `json:"id" db:"id"`
	SerialNumber string    `json:"serial_number" db:"serial_number"`

	// Game and draw information
	GameCode       string     `json:"game_code" db:"game_code"`
	GameScheduleID *uuid.UUID `json:"game_schedule_id,omitempty" db:"game_schedule_id"`
	DrawNumber     int32      `json:"draw_number" db:"draw_number"`
	DrawID         *uuid.UUID `json:"draw_id,omitempty" db:"draw_id"`
	GameName       string     `json:"game_name" db:"game_name"`
	GameType       string     `json:"game_type" db:"game_type"`

	// Number selections
	SelectedNumbers []int32 `json:"selected_numbers" db:"selected_numbers"`
	BankerNumbers   []int32 `json:"banker_numbers" db:"banker_numbers"`
	OpposedNumbers  []int32 `json:"opposed_numbers" db:"opposed_numbers"`

	// Bet lines and pricing (stored as JSONB in database)
	BetLines      []BetLine `json:"bet_lines" db:"bet_lines"`
	NumberOfLines int32     `json:"number_of_lines" db:"number_of_lines"`
	UnitPrice     int64     `json:"unit_price" db:"unit_price"`     // in pesewas
	TotalAmount   int64     `json:"total_amount" db:"total_amount"` // in pesewas

	// Issuer information
	IssuerType    string         `json:"issuer_type" db:"issuer_type"`
	IssuerID      string         `json:"issuer_id" db:"issuer_id"`
	IssuerDetails *IssuerDetails `json:"issuer_details,omitempty" db:"issuer_details"` // JSONB

	// Customer information (optional)
	CustomerPhone *string `json:"customer_phone,omitempty" db:"customer_phone"`
	CustomerName  *string `json:"customer_name,omitempty" db:"customer_name"`
	CustomerEmail *string `json:"customer_email,omitempty" db:"customer_email"`

	// Payment information
	PaymentMethod *string `json:"payment_method,omitempty" db:"payment_method"`
	PaymentRef    *string `json:"payment_ref,omitempty" db:"payment_ref"`
	PaymentStatus *string `json:"payment_status,omitempty" db:"payment_status"`

	// Security features
	SecurityHash     string            `json:"security_hash" db:"security_hash"`
	SecurityFeatures *SecurityFeatures `json:"security_features,omitempty" db:"security_features"` // JSONB

	// Status and lifecycle
	Status        string  `json:"status" db:"status"`
	IsWinning     bool    `json:"is_winning" db:"is_winning"`
	WinningAmount int64   `json:"winning_amount" db:"winning_amount"` // in pesewas
	PrizeTier     *string `json:"prize_tier,omitempty" db:"prize_tier"`
	Matches       int32   `json:"matches" db:"matches"`

	// Draw information
	DrawDate *time.Time `json:"draw_date,omitempty" db:"draw_date"`
	DrawTime *time.Time `json:"draw_time,omitempty" db:"draw_time"`

	// Timestamps
	IssuedAt    time.Time  `json:"issued_at" db:"issued_at"`
	ValidatedAt *time.Time `json:"validated_at,omitempty" db:"validated_at"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty" db:"cancelled_at"`
	PaidAt      *time.Time `json:"paid_at,omitempty" db:"paid_at"`
	VoidedAt    *time.Time `json:"voided_at,omitempty" db:"voided_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`

	// Payout tracking
	PaidBy           *string `json:"paid_by,omitempty" db:"paid_by"`
	PaymentReference *string `json:"payment_reference,omitempty" db:"payment_reference"`
}

// BetLine represents a single bet line in a ticket
type BetLine struct {
	LineNumber int32  `json:"line_number"`
	BetType    string `json:"bet_type"` // Standard format: "DIRECT-1", "DIRECT-2", "DIRECT-3", "DIRECT-4", "DIRECT-5", "PERM-2", "PERM-3", "PERM-4", "PERM-5", "BANKER ALL", "BANKER AG"

	// For DIRECT and PERM bets
	SelectedNumbers []int32 `json:"selected_numbers,omitempty"` // Player's chosen numbers

	// For BANKER and AGAINST bets
	Banker  []int32 `json:"banker,omitempty"`
	Opposed []int32 `json:"opposed,omitempty"`

	// For PERM and Banker bets (new compact format)
	NumberOfCombinations int32 `json:"number_of_combinations,omitempty"` // C(n,r) - calculated value
	AmountPerCombination int64 `json:"amount_per_combination,omitempty"` // Amount per combination in pesewas

	// Common fields
	TotalAmount  int64 `json:"total_amount"`  // Total bet amount in pesewas
	PotentialWin int64 `json:"potential_win"` // Potential winning amount in pesewas

	// Legacy support (deprecated - will be removed after migration)
	Numbers []int32 `json:"numbers,omitempty"` // Old field name, use selected_numbers instead
	Amount  int64   `json:"amount,omitempty"`  // Old field, use total_amount instead
}

// IssuerDetails represents the issuer context (stored as JSONB)
type IssuerDetails struct {
	// For POS
	AgentCode    *string `json:"agent_code,omitempty"`
	RetailerCode *string `json:"retailer_code,omitempty"`
	TerminalID   *string `json:"terminal_id,omitempty"`

	// For Web/Mobile
	PlayerID *string `json:"player_id,omitempty"`

	// For USSD
	SessionID *string `json:"session_id,omitempty"`

	// For Telegram
	TelegramUserID   *string `json:"telegram_user_id,omitempty"`
	TelegramUsername *string `json:"telegram_username,omitempty"`

	// For WhatsApp
	WhatsappNumber *string `json:"whatsapp_number,omitempty"`
	WhatsappName   *string `json:"whatsapp_name,omitempty"`
}

// SecurityFeatures represents ticket security features (stored as JSONB)
type SecurityFeatures struct {
	QRCode           string `json:"qr_code"`
	Barcode          string `json:"barcode"`
	VerificationCode string `json:"verification_code"`
}

// Enums

// TicketStatus represents the status of a ticket
type TicketStatus string

const (
	TicketStatusIssued    TicketStatus = "issued"
	TicketStatusValidated TicketStatus = "validated"
	TicketStatusWon       TicketStatus = "won"
	TicketStatusLost      TicketStatus = "lost"
	TicketStatusPaid      TicketStatus = "paid"
	TicketStatusCancelled TicketStatus = "cancelled"
	TicketStatusExpired   TicketStatus = "expired"
	TicketStatusVoid      TicketStatus = "void"
)

// IssuerType represents the type of ticket issuer
type IssuerType string

const (
	IssuerTypePOS       IssuerType = "pos"
	IssuerTypeWeb       IssuerType = "web"
	IssuerTypeMobileApp IssuerType = "mobile_app"
	IssuerTypeUSSD      IssuerType = "ussd"
	IssuerTypeTelegram  IssuerType = "telegram"
	IssuerTypeWhatsApp  IssuerType = "whatsapp"
)

// ValidationMethod represents the method used to validate a ticket
type ValidationMethod string

const (
	ValidationMethodQRScan       ValidationMethod = "qr_scan"
	ValidationMethodBarcodeScan  ValidationMethod = "barcode_scan"
	ValidationMethodSerialNumber ValidationMethod = "serial_number"
	ValidationMethodAPI          ValidationMethod = "api"
)

// ValidationResult represents the result of a ticket validation
type ValidationResult string

const (
	ValidationResultValid            ValidationResult = "valid"
	ValidationResultInvalid          ValidationResult = "invalid"
	ValidationResultExpired          ValidationResult = "expired"
	ValidationResultAlreadyPaid      ValidationResult = "already_paid"
	ValidationResultCancelled        ValidationResult = "cancelled"
	ValidationResultVoid             ValidationResult = "void"
	ValidationResultAlreadyValidated ValidationResult = "already_validated"
)

// PaymentMethod represents the payment method
type PaymentMethod string

const (
	PaymentMethodCash         PaymentMethod = "cash"
	PaymentMethodMobileMoney  PaymentMethod = "mobile_money"
	PaymentMethodWallet       PaymentMethod = "wallet"
	PaymentMethodBankTransfer PaymentMethod = "bank_transfer"
)

// Utility functions

// PesewasToGHS converts pesewas to Ghana Cedis for display
func PesewasToGHS(pesewas int64) float64 {
	return float64(pesewas) / 100.0
}

// GHSToPesewas converts Ghana Cedis to pesewas for storage
func GHSToPesewas(ghs float64) int64 {
	return int64(ghs * 100)
}

// Validation methods

// ValidateBusinessRules validates ticket business rules
func (t *Ticket) ValidateBusinessRules() error {
	// Validate required fields
	if t.SerialNumber == "" {
		return fmt.Errorf("serial number cannot be empty")
	}
	if t.GameCode == "" {
		return fmt.Errorf("game code cannot be empty")
	}
	if t.GameName == "" {
		return fmt.Errorf("game name cannot be empty")
	}
	if t.GameType == "" {
		return fmt.Errorf("game type cannot be empty")
	}
	if t.DrawNumber < 1 {
		return fmt.Errorf("draw number must be positive")
	}

	// Validate number selections
	if len(t.SelectedNumbers) == 0 && len(t.BetLines) == 0 {
		return fmt.Errorf("ticket must have selected numbers or bet lines")
	}

	// Validate bet lines
	if len(t.BetLines) == 0 {
		return fmt.Errorf("ticket must have at least one bet line")
	}
	if t.NumberOfLines < 1 {
		return fmt.Errorf("number of lines must be at least 1")
	}
	if int32(len(t.BetLines)) != t.NumberOfLines {
		return fmt.Errorf("number of bet lines (%d) must match number_of_lines (%d)", len(t.BetLines), t.NumberOfLines)
	}

	// Validate pricing
	if t.UnitPrice <= 0 {
		return fmt.Errorf("unit price must be positive")
	}
	if t.TotalAmount <= 0 {
		return fmt.Errorf("total amount must be positive")
	}

	// Validate issuer
	if t.IssuerType == "" {
		return fmt.Errorf("issuer type cannot be empty")
	}
	if t.IssuerID == "" {
		return fmt.Errorf("issuer ID cannot be empty")
	}

	// Validate security
	if t.SecurityHash == "" {
		return fmt.Errorf("security hash cannot be empty")
	}

	// Validate status
	if t.Status == "" {
		return fmt.Errorf("status cannot be empty")
	}

	return nil
}

// CanBeCancelled checks if the ticket can be cancelled
func (t *Ticket) CanBeCancelled() bool {
	// Can only cancel issued or validated tickets that haven't been paid or voided
	return (t.Status == string(TicketStatusIssued) || t.Status == string(TicketStatusValidated)) &&
		t.PaidAt == nil && t.VoidedAt == nil
}

// CanBeValidated checks if the ticket can be validated
func (t *Ticket) CanBeValidated() bool {
	// Can only validate issued tickets
	return t.Status == string(TicketStatusIssued) && t.VoidedAt == nil && t.CancelledAt == nil
}

// CanBePaid checks if the ticket can be paid
func (t *Ticket) CanBePaid() bool {
	// Can only pay winning tickets that haven't been paid yet
	return t.IsWinning && t.Status == string(TicketStatusWon) && t.PaidAt == nil && t.VoidedAt == nil
}

// CanTransitionTo checks if the ticket can transition to a new status
func (t *Ticket) CanTransitionTo(newStatus TicketStatus) bool {
	currentStatus := TicketStatus(t.Status)

	switch currentStatus {
	case TicketStatusIssued:
		return newStatus == TicketStatusValidated ||
			newStatus == TicketStatusWon ||
			newStatus == TicketStatusLost ||
			newStatus == TicketStatusCancelled ||
			newStatus == TicketStatusExpired ||
			newStatus == TicketStatusVoid
	case TicketStatusValidated:
		return newStatus == TicketStatusWon ||
			newStatus == TicketStatusLost ||
			newStatus == TicketStatusCancelled ||
			newStatus == TicketStatusExpired ||
			newStatus == TicketStatusVoid
	case TicketStatusWon:
		return newStatus == TicketStatusPaid || newStatus == TicketStatusVoid
	case TicketStatusLost:
		return newStatus == TicketStatusVoid // Can only be voided for exceptional cases
	case TicketStatusPaid:
		return newStatus == TicketStatusVoid // Terminal state, can only be voided
	case TicketStatusCancelled:
		return false // Terminal state
	case TicketStatusExpired:
		return false // Terminal state
	case TicketStatusVoid:
		return false // Terminal state
	default:
		return false
	}
}

// SetStatus updates the ticket status with validation
func (t *Ticket) SetStatus(newStatus TicketStatus) error {
	if !t.CanTransitionTo(newStatus) {
		return fmt.Errorf("cannot transition from %s to %s", t.Status, newStatus)
	}
	t.Status = string(newStatus)
	t.UpdatedAt = time.Now()

	// Update corresponding timestamp
	now := time.Now()
	switch newStatus {
	case TicketStatusValidated:
		t.ValidatedAt = &now
	case TicketStatusCancelled:
		t.CancelledAt = &now
	case TicketStatusPaid:
		t.PaidAt = &now
	case TicketStatusVoid:
		t.VoidedAt = &now
	}

	return nil
}

// CalculateTotalAmount calculates the total ticket amount from bet lines
func (t *Ticket) CalculateTotalAmount() int64 {
	var total int64
	for _, line := range t.BetLines {
		total += line.Amount
	}
	return total
}

// IsExpired checks if the ticket has expired
func (t *Ticket) IsExpired() bool {
	if t.DrawDate == nil {
		return false
	}
	// Tickets expire 90 days after draw date (configurable)
	expiryDate := t.DrawDate.AddDate(0, 0, 90)
	return time.Now().After(expiryDate)
}

// GetDisplayAmount returns the total amount in GHS for display
func (t *Ticket) GetDisplayAmount() float64 {
	return PesewasToGHS(t.TotalAmount)
}

// GetDisplayWinningAmount returns the winning amount in GHS for display
func (t *Ticket) GetDisplayWinningAmount() float64 {
	return PesewasToGHS(t.WinningAmount)
}

// ValidateBetLine validates a single bet line
func (bl *BetLine) Validate() error {
	if bl.LineNumber < 1 {
		return fmt.Errorf("line number must be positive")
	}
	if bl.BetType == "" {
		return fmt.Errorf("bet type cannot be empty")
	}

	// Support both old and new formats
	numbers := bl.SelectedNumbers
	if len(numbers) == 0 && len(bl.Numbers) > 0 {
		numbers = bl.Numbers // Legacy support
	}

	amount := bl.TotalAmount
	if amount == 0 && bl.Amount > 0 {
		amount = bl.Amount // Legacy support
	}

	if len(numbers) == 0 && len(bl.Banker) == 0 {
		return fmt.Errorf("bet line must have selected numbers or banker numbers")
	}
	if amount <= 0 {
		return fmt.Errorf("bet line amount must be positive")
	}
	if bl.PotentialWin < 0 {
		return fmt.Errorf("potential win cannot be negative")
	}

	// Check for duplicates in numbers
	seen := make(map[int32]bool)
	for _, num := range numbers {
		if seen[num] {
			return fmt.Errorf("duplicate number %d in bet line", num)
		}
		seen[num] = true
	}

	return nil
}

// Supporting models for related tables

// TicketPayment represents a payment record for winning tickets
type TicketPayment struct {
	ID               uuid.UUID `json:"id" db:"id"`
	TicketID         uuid.UUID `json:"ticket_id" db:"ticket_id"`
	PaymentReference string    `json:"payment_reference" db:"payment_reference"`

	// Claimant information
	ClaimantName     string  `json:"claimant_name" db:"claimant_name"`
	ClaimantPhone    string  `json:"claimant_phone" db:"claimant_phone"`
	ClaimantIDType   *string `json:"claimant_id_type,omitempty" db:"claimant_id_type"`
	ClaimantIDNumber *string `json:"claimant_id_number,omitempty" db:"claimant_id_number"`

	// Bank information
	BankAccount *string `json:"bank_account,omitempty" db:"bank_account"`
	BankName    *string `json:"bank_name,omitempty" db:"bank_name"`
	BankBranch  *string `json:"bank_branch,omitempty" db:"bank_branch"`

	// Payment details
	PrizeAmount           int64   `json:"prize_amount" db:"prize_amount"` // in pesewas
	PaymentMethod         *string `json:"payment_method,omitempty" db:"payment_method"`
	PaymentStatus         string  `json:"payment_status" db:"payment_status"`
	PaymentNotes          *string `json:"payment_notes,omitempty" db:"payment_notes"`
	PaymentTransactionRef *string `json:"payment_transaction_ref,omitempty" db:"payment_transaction_ref"`

	// Approval workflow
	ApprovedBy *uuid.UUID `json:"approved_by,omitempty" db:"approved_by"`
	ApprovedAt *time.Time `json:"approved_at,omitempty" db:"approved_at"`
	PaidBy     *uuid.UUID `json:"paid_by,omitempty" db:"paid_by"`
	PaidAt     *time.Time `json:"paid_at,omitempty" db:"paid_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TicketCancellation represents a ticket cancellation record
type TicketCancellation struct {
	ID       uuid.UUID `json:"id" db:"id"`
	TicketID uuid.UUID `json:"ticket_id" db:"ticket_id"`

	// Cancellation details
	Reason          string `json:"reason" db:"reason"`
	CancelledByType string `json:"cancelled_by_type" db:"cancelled_by_type"`
	CancelledByID   string `json:"cancelled_by_id" db:"cancelled_by_id"`

	// Refund information
	RefundAmount int64   `json:"refund_amount" db:"refund_amount"` // in pesewas
	RefundMethod *string `json:"refund_method,omitempty" db:"refund_method"`
	RefundStatus *string `json:"refund_status,omitempty" db:"refund_status"`
	RefundRef    *string `json:"refund_ref,omitempty" db:"refund_ref"`
	RefundNotes  *string `json:"refund_notes,omitempty" db:"refund_notes"`

	// Approval
	ApprovedBy *uuid.UUID `json:"approved_by,omitempty" db:"approved_by"`
	ApprovedAt *time.Time `json:"approved_at,omitempty" db:"approved_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TicketVoid represents a ticket void record
type TicketVoid struct {
	ID       uuid.UUID `json:"id" db:"id"`
	TicketID uuid.UUID `json:"ticket_id" db:"ticket_id"`

	// Void details
	Reason             string  `json:"reason" db:"reason"`
	VoidType           string  `json:"void_type" db:"void_type"`
	AuthorizedBy       string  `json:"authorized_by" db:"authorized_by"`
	AuthorizationNotes *string `json:"authorization_notes,omitempty" db:"authorization_notes"`

	VoidedAt  time.Time `json:"voided_at" db:"voided_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// TicketReprint represents a ticket reprint record
type TicketReprint struct {
	ID       uuid.UUID `json:"id" db:"id"`
	TicketID uuid.UUID `json:"ticket_id" db:"ticket_id"`

	// Reprint details
	RequestedByType string  `json:"requested_by_type" db:"requested_by_type"`
	RequestedByID   string  `json:"requested_by_id" db:"requested_by_id"`
	Reason          *string `json:"reason,omitempty" db:"reason"`

	// POS details
	TerminalID *string `json:"terminal_id,omitempty" db:"terminal_id"`
	PrinterID  *string `json:"printer_id,omitempty" db:"printer_id"`

	ReprintedAt time.Time `json:"reprinted_at" db:"reprinted_at"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// TicketValidation represents a ticket validation record
type TicketValidation struct {
	ID       uuid.UUID `json:"id" db:"id"`
	TicketID uuid.UUID `json:"ticket_id" db:"ticket_id"`

	// Validation details
	ValidatedByType  string  `json:"validated_by_type" db:"validated_by_type"`
	ValidatedByID    string  `json:"validated_by_id" db:"validated_by_id"`
	ValidationMethod string  `json:"validation_method" db:"validation_method"`
	ValidationResult string  `json:"validation_result" db:"validation_result"`
	ValidationNotes  *string `json:"validation_notes,omitempty" db:"validation_notes"`

	// Context
	TerminalID *string `json:"terminal_id,omitempty" db:"terminal_id"`
	IPAddress  *string `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent  *string `json:"user_agent,omitempty" db:"user_agent"`

	ValidatedAt time.Time `json:"validated_at" db:"validated_at"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Filter types for queries

// TicketFilter represents filters for querying tickets
type TicketFilter struct {
	GameCode       *string `json:"game_code,omitempty"`
	GameScheduleID *string `json:"game_schedule_id,omitempty"`
	DrawNumber     *int32  `json:"draw_number,omitempty"`
	DrawID         *string `json:"draw_id,omitempty"`
	IssuerType     *string `json:"issuer_type,omitempty"`
	IssuerID       *string `json:"issuer_id,omitempty"`
	CustomerPhone  *string `json:"customer_phone,omitempty"`
	Status         *string `json:"status,omitempty"`
	IsWinning      *bool   `json:"is_winning,omitempty"`
	StartDate      *string `json:"start_date,omitempty"`
	EndDate        *string `json:"end_date,omitempty"`
}

package services

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/service-payment/internal/config"
	"github.com/randco/service-payment/internal/models"
	"gorm.io/gorm"
)

// MoMoProviderService interface defines mobile money provider operations
type MoMoProviderService interface {
	InitiatePayment(ctx context.Context, payment *models.Payment, phone string, provider models.MoMoProvider) (*MoMoPaymentResponse, error)
	CheckPaymentStatus(ctx context.Context, paymentID uuid.UUID, transactionID string) (*MoMoStatusResponse, error)
}

// BankProviderService interface defines bank transfer provider operations
type BankProviderService interface {
	InitiateTransfer(ctx context.Context, payment *models.Payment, bankDetails BankTransferDetails) (*BankTransferResponse, error)
	CheckTransferStatus(ctx context.Context, paymentID uuid.UUID, transferReference string) (*BankTransferStatusResponse, error)
}

// MoMoPaymentResponse represents mobile money payment initiation response
type MoMoPaymentResponse struct {
	Success       bool
	TransactionID string
	Status        string
	Message       string
	USSDCode      string
	RequestData   map[string]string
	ResponseData  map[string]string
}

// MoMoStatusResponse represents mobile money status check response
type MoMoStatusResponse struct {
	Status         models.PaymentStatus
	ProviderStatus string
	Message        string
	Details        map[string]string
}

// BankTransferDetails represents bank transfer request details
type BankTransferDetails struct {
	BankCode      string
	BankName      string
	AccountNumber string
	AccountName   string
	BranchCode    string
}

// BankTransferResponse represents bank transfer initiation response
type BankTransferResponse struct {
	Success           bool
	TransferReference string
	Message           string
	Instructions      string
	RequestData       map[string]string
	ResponseData      map[string]string
}

// BankTransferStatusResponse represents bank transfer status response
type BankTransferStatusResponse struct {
	Status  models.PaymentStatus
	Message string
	Details map[string]string
}

// stubMoMoProviderService implements MoMoProviderService with stub responses
type stubMoMoProviderService struct {
	db     *gorm.DB
	config *config.PaymentConfig
	logger logger.Logger
}

// NewMoMoProviderService creates a new mobile money provider service
func NewMoMoProviderService(db *gorm.DB, config *config.PaymentConfig, logger logger.Logger) MoMoProviderService {
	return &stubMoMoProviderService{
		db:     db,
		config: config,
		logger: logger,
	}
}

// InitiatePayment initiates a mobile money payment (stub implementation)
func (s *stubMoMoProviderService) InitiatePayment(ctx context.Context, payment *models.Payment, phone string, provider models.MoMoProvider) (*MoMoPaymentResponse, error) {
	// Generate random transaction ID
	transactionID := fmt.Sprintf("STUB_%s_%d", provider, time.Now().Unix())

	// Simulate different success/failure rates based on test scenarios
	var success bool
	var status string
	var message string
	var ussdCode string

	if s.config.TestMode {
		// Test mode: configurable responses based on phone number patterns
		switch {
		case phone[len(phone)-1:] == "1": // Success
			success = true
			status = "PENDING"
			message = "Payment initiated successfully"
			ussdCode = "*170#"
		case phone[len(phone)-1:] == "2": // Failure
			success = false
			status = "FAILED"
			message = "Insufficient funds"
		case phone[len(phone)-1:] == "3": // Success after delay
			success = true
			status = "PROCESSING"
			message = "Payment is being processed"
			ussdCode = "*170#"
		default: // Random outcome
			success = rand.Float32() < 0.8 // 80% success rate
			if success {
				status = "PENDING"
				message = "Payment initiated successfully"
				ussdCode = getUSSDCode(provider)
			} else {
				status = "FAILED"
				message = "Payment failed. Please try again."
			}
		}
	} else {
		// Production mode would integrate with actual providers
		return nil, fmt.Errorf("production mode not implemented")
	}

	requestData := map[string]string{
		"phone_number": phone,
		"amount":       fmt.Sprintf("%d", payment.Amount),
		"currency":     payment.Currency,
		"description":  payment.Description,
		"provider":     string(provider),
	}

	responseData := map[string]string{
		"transaction_id": transactionID,
		"status":         status,
		"message":        message,
		"timestamp":      time.Now().Format(time.RFC3339),
	}
	if ussdCode != "" {
		responseData["ussd_code"] = ussdCode
	}

	// Store MoMo transaction record
	momoTxn := &models.MoMoTransaction{
		PaymentID:     payment.ID,
		Provider:      provider,
		PhoneNumber:   phone,
		TransactionID: transactionID,
		Status:        status,
		StatusMessage: message,
		USSDCode:      ussdCode,
		RequestData:   requestData,
		ResponseData:  responseData,
	}

	if err := s.db.WithContext(ctx).Create(momoTxn).Error; err != nil {
		s.logger.Error("Failed to create MoMo transaction record", "error", err)
	}

	return &MoMoPaymentResponse{
		Success:       success,
		TransactionID: transactionID,
		Status:        status,
		Message:       message,
		USSDCode:      ussdCode,
		RequestData:   requestData,
		ResponseData:  responseData,
	}, nil
}

// CheckPaymentStatus checks the status of a mobile money payment (stub implementation)
func (s *stubMoMoProviderService) CheckPaymentStatus(ctx context.Context, paymentID uuid.UUID, transactionID string) (*MoMoStatusResponse, error) {
	// Get MoMo transaction record
	var momoTxn models.MoMoTransaction
	if err := s.db.WithContext(ctx).
		Where("payment_id = ? AND transaction_id = ?", paymentID, transactionID).
		First(&momoTxn).Error; err != nil {
		return nil, fmt.Errorf("MoMo transaction not found: %w", err)
	}

	// Simulate status progression in test mode
	if s.config.TestMode {
		elapsed := time.Since(momoTxn.CreatedAt)

		switch momoTxn.Status {
		case "PENDING":
			if elapsed > 30*time.Second {
				// Transition to completed after 30 seconds
				newStatus := "COMPLETED"
				momoTxn.Status = newStatus
				momoTxn.StatusMessage = "Payment completed successfully"
				s.db.WithContext(ctx).Save(&momoTxn)

				return &MoMoStatusResponse{
					Status:         models.PaymentStatusCompleted,
					ProviderStatus: newStatus,
					Message:        "Payment completed successfully",
					Details: map[string]string{
						"transaction_id": transactionID,
						"completed_at":   time.Now().Format(time.RFC3339),
					},
				}, nil
			}
			return &MoMoStatusResponse{
				Status:         models.PaymentStatusPending,
				ProviderStatus: "PENDING",
				Message:        "Payment is still pending user confirmation",
				Details: map[string]string{
					"transaction_id": transactionID,
				},
			}, nil

		case "PROCESSING":
			if elapsed > 60*time.Second {
				// Transition to completed after 60 seconds
				newStatus := "COMPLETED"
				momoTxn.Status = newStatus
				momoTxn.StatusMessage = "Payment completed successfully"
				s.db.WithContext(ctx).Save(&momoTxn)

				return &MoMoStatusResponse{
					Status:         models.PaymentStatusCompleted,
					ProviderStatus: newStatus,
					Message:        "Payment completed successfully",
					Details: map[string]string{
						"transaction_id": transactionID,
						"completed_at":   time.Now().Format(time.RFC3339),
					},
				}, nil
			}
			return &MoMoStatusResponse{
				Status:         models.PaymentStatusProcessing,
				ProviderStatus: "PROCESSING",
				Message:        "Payment is being processed",
				Details: map[string]string{
					"transaction_id": transactionID,
				},
			}, nil

		case "COMPLETED":
			return &MoMoStatusResponse{
				Status:         models.PaymentStatusCompleted,
				ProviderStatus: "COMPLETED",
				Message:        "Payment completed successfully",
				Details: map[string]string{
					"transaction_id": transactionID,
					"completed_at":   momoTxn.UpdatedAt.Format(time.RFC3339),
				},
			}, nil

		case "FAILED":
			return &MoMoStatusResponse{
				Status:         models.PaymentStatusFailed,
				ProviderStatus: "FAILED",
				Message:        momoTxn.StatusMessage,
				Details: map[string]string{
					"transaction_id": transactionID,
					"failed_at":      momoTxn.UpdatedAt.Format(time.RFC3339),
				},
			}, nil
		}
	}

	return &MoMoStatusResponse{
		Status:         models.PaymentStatusPending,
		ProviderStatus: momoTxn.Status,
		Message:        momoTxn.StatusMessage,
		Details: map[string]string{
			"transaction_id": transactionID,
		},
	}, nil
}

// stubBankProviderService implements BankProviderService with stub responses
type stubBankProviderService struct {
	db     *gorm.DB
	config *config.PaymentConfig
	logger logger.Logger
}

// NewBankProviderService creates a new bank provider service
func NewBankProviderService(db *gorm.DB, config *config.PaymentConfig, logger logger.Logger) BankProviderService {
	return &stubBankProviderService{
		db:     db,
		config: config,
		logger: logger,
	}
}

// InitiateTransfer initiates a bank transfer (stub implementation)
func (s *stubBankProviderService) InitiateTransfer(ctx context.Context, payment *models.Payment, bankDetails BankTransferDetails) (*BankTransferResponse, error) {
	// Generate transfer reference
	transferReference := fmt.Sprintf("BT_%s_%d", payment.ID.String()[:8], time.Now().Unix())

	instructions := fmt.Sprintf(`
Bank Transfer Instructions:
- Bank: %s
- Account Number: %s
- Account Name: %s
- Amount: GHS %.2f
- Reference: %s

Please complete the transfer and notify us with the transaction reference.
`, bankDetails.BankName, bankDetails.AccountNumber, bankDetails.AccountName,
		float64(payment.Amount)/100, transferReference)

	requestData := map[string]string{
		"bank_code":      bankDetails.BankCode,
		"bank_name":      bankDetails.BankName,
		"account_number": bankDetails.AccountNumber,
		"account_name":   bankDetails.AccountName,
		"amount":         fmt.Sprintf("%d", payment.Amount),
		"currency":       payment.Currency,
	}

	responseData := map[string]string{
		"transfer_reference": transferReference,
		"status":             "PENDING_VERIFICATION",
		"message":            "Transfer initiated, awaiting manual verification",
		"timestamp":          time.Now().Format(time.RFC3339),
	}

	// Store bank transfer record
	bankTransfer := &models.BankTransfer{
		PaymentID:         payment.ID,
		BankCode:          bankDetails.BankCode,
		BankName:          bankDetails.BankName,
		AccountNumber:     bankDetails.AccountNumber,
		AccountName:       bankDetails.AccountName,
		BranchCode:        bankDetails.BranchCode,
		TransferReference: transferReference,
		Status:            "PENDING_VERIFICATION",
		StatusMessage:     "Transfer initiated, awaiting manual verification",
		Instructions:      instructions,
		RequestData:       requestData,
		ResponseData:      responseData,
	}

	if err := s.db.WithContext(ctx).Create(bankTransfer).Error; err != nil {
		s.logger.Error("Failed to create bank transfer record", "error", err)
	}

	return &BankTransferResponse{
		Success:           true,
		TransferReference: transferReference,
		Message:           "Transfer initiated successfully",
		Instructions:      instructions,
		RequestData:       requestData,
		ResponseData:      responseData,
	}, nil
}

// CheckTransferStatus checks the status of a bank transfer (stub implementation)
func (s *stubBankProviderService) CheckTransferStatus(ctx context.Context, paymentID uuid.UUID, transferReference string) (*BankTransferStatusResponse, error) {
	// Get bank transfer record
	var bankTransfer models.BankTransfer
	if err := s.db.WithContext(ctx).
		Where("payment_id = ? AND transfer_reference = ?", paymentID, transferReference).
		First(&bankTransfer).Error; err != nil {
		return nil, fmt.Errorf("bank transfer not found: %w", err)
	}

	// In test mode, simulate manual verification after some time
	if s.config.TestMode {
		elapsed := time.Since(bankTransfer.CreatedAt)

		if bankTransfer.Status == "PENDING_VERIFICATION" && elapsed > 2*time.Minute {
			// Auto-approve after 2 minutes for testing
			newStatus := "COMPLETED"
			bankTransfer.Status = newStatus
			bankTransfer.StatusMessage = "Transfer verified and completed"
			s.db.WithContext(ctx).Save(&bankTransfer)

			return &BankTransferStatusResponse{
				Status:  models.PaymentStatusCompleted,
				Message: "Transfer verified and completed",
				Details: map[string]string{
					"transfer_reference": transferReference,
					"verified_at":        time.Now().Format(time.RFC3339),
				},
			}, nil
		}
	}

	// Map bank transfer status to payment status
	var paymentStatus models.PaymentStatus
	switch bankTransfer.Status {
	case "COMPLETED":
		paymentStatus = models.PaymentStatusCompleted
	case "FAILED":
		paymentStatus = models.PaymentStatusFailed
	default:
		paymentStatus = models.PaymentStatusPending
	}

	return &BankTransferStatusResponse{
		Status:  paymentStatus,
		Message: bankTransfer.StatusMessage,
		Details: map[string]string{
			"transfer_reference": transferReference,
			"bank_status":        bankTransfer.Status,
		},
	}, nil
}

// getUSSDCode returns the USSD code for different MoMo providers
func getUSSDCode(provider models.MoMoProvider) string {
	switch provider {
	case models.MoMoProviderMTN:
		return "*170#"
	case models.MoMoProviderTelecel:
		return "*110#"
	case models.MoMoProviderAirtelTigo:
		return "*185#"
	default:
		return "*000#"
	}
}

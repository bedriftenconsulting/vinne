package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/service-payment/internal/models"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// PaymentService interface defines payment business operations
type PaymentService interface {
	ProcessPayment(ctx context.Context, payment *models.Payment) error
	GetPayment(ctx context.Context, paymentID uuid.UUID) (*models.Payment, error)
	ListPayments(ctx context.Context, filter PaymentFilter) ([]*models.Payment, int, error)
	UpdatePaymentStatus(ctx context.Context, paymentID uuid.UUID, status models.PaymentStatus) error
	GetPaymentByReference(ctx context.Context, reference string) (*models.Payment, error)
}

// PaymentMethodService interface defines payment method operations
type PaymentMethodService interface {
	CreatePaymentMethod(ctx context.Context, method *models.PaymentMethod) error
	GetPaymentMethods(ctx context.Context, userID string, methodType *models.PaymentMethodType) ([]*models.PaymentMethod, error)
	UpdatePaymentMethod(ctx context.Context, method *models.PaymentMethod) error
	DeletePaymentMethod(ctx context.Context, methodID uuid.UUID) error
	GetPaymentMethod(ctx context.Context, methodID uuid.UUID) (*models.PaymentMethod, error)
}

// PaymentFilter represents filtering options for payment listing
type PaymentFilter struct {
	UserID    string
	Type      models.PaymentType
	Status    models.PaymentStatus
	StartDate *time.Time
	EndDate   *time.Time
	Page      int
	PageSize  int
	SortBy    string
	SortDesc  bool
}

// paymentService implements PaymentService interface
type paymentService struct {
	db          *gorm.DB
	redisClient *redis.Client
	logger      logger.Logger
}

// NewPaymentService creates a new payment service instance
func NewPaymentService(db *gorm.DB, redisClient *redis.Client, logger logger.Logger) PaymentService {
	return &paymentService{
		db:          db,
		redisClient: redisClient,
		logger:      logger,
	}
}

// ProcessPayment processes a new payment
func (s *paymentService) ProcessPayment(ctx context.Context, payment *models.Payment) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create payment record
		if err := tx.Create(payment).Error; err != nil {
			return fmt.Errorf("failed to create payment: %w", err)
		}

		// Create payment log
		log := &models.PaymentLog{
			PaymentID:   payment.ID,
			Action:      "PAYMENT_CREATED",
			NewStatus:   payment.Status,
			Details:     map[string]string{"type": string(payment.Type)},
			PerformedBy: "system",
		}
		if err := tx.Create(log).Error; err != nil {
			s.logger.Error("Failed to create payment log", "error", err, "payment_id", payment.ID)
		}

		return nil
	})
}

// GetPayment retrieves a payment by ID
func (s *paymentService) GetPayment(ctx context.Context, paymentID uuid.UUID) (*models.Payment, error) {
	var payment models.Payment
	if err := s.db.WithContext(ctx).
		Preload("PaymentMethod").
		Preload("PaymentLogs").
		First(&payment, paymentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("payment not found")
		}
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}
	return &payment, nil
}

// GetPaymentByReference retrieves a payment by reference
func (s *paymentService) GetPaymentByReference(ctx context.Context, reference string) (*models.Payment, error) {
	var payment models.Payment
	if err := s.db.WithContext(ctx).
		Where("reference = ?", reference).
		First(&payment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("payment not found")
		}
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}
	return &payment, nil
}

// ListPayments lists payments with filtering and pagination
func (s *paymentService) ListPayments(ctx context.Context, filter PaymentFilter) ([]*models.Payment, int, error) {
	query := s.db.WithContext(ctx).Model(&models.Payment{}).Preload("PaymentMethod")

	// Apply filters
	if filter.UserID != "" {
		query = query.Where("payer_id = ? OR payee_id = ?", filter.UserID, filter.UserID)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.StartDate != nil {
		query = query.Where("created_at >= ?", filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("created_at <= ?", filter.EndDate)
	}

	// Count total records
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count payments: %w", err)
	}

	// Apply sorting
	if filter.SortBy != "" {
		order := "ASC"
		if filter.SortDesc {
			order = "DESC"
		}
		query = query.Order(fmt.Sprintf("%s %s", filter.SortBy, order))
	} else {
		query = query.Order("created_at DESC")
	}

	// Apply pagination
	if filter.Page > 0 && filter.PageSize > 0 {
		offset := (filter.Page - 1) * filter.PageSize
		query = query.Offset(offset).Limit(filter.PageSize)
	}

	// Fetch payments
	var payments []*models.Payment
	if err := query.Find(&payments).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list payments: %w", err)
	}

	return payments, int(total), nil
}

// UpdatePaymentStatus updates the status of a payment
func (s *paymentService) UpdatePaymentStatus(ctx context.Context, paymentID uuid.UUID, status models.PaymentStatus) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get current payment
		var payment models.Payment
		if err := tx.First(&payment, paymentID).Error; err != nil {
			return fmt.Errorf("payment not found: %w", err)
		}

		oldStatus := payment.Status

		// Update payment status
		updates := map[string]interface{}{
			"status": status,
		}
		if status == models.PaymentStatusCompleted {
			now := time.Now()
			updates["processed_at"] = &now
		}

		if err := tx.Model(&payment).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update payment status: %w", err)
		}

		// Create payment log
		log := &models.PaymentLog{
			PaymentID:   paymentID,
			Action:      "STATUS_UPDATED",
			OldStatus:   oldStatus,
			NewStatus:   status,
			Details:     map[string]string{"reason": "status_update"},
			PerformedBy: "system",
		}
		if err := tx.Create(log).Error; err != nil {
			s.logger.Error("Failed to create payment log", "error", err, "payment_id", paymentID)
		}

		return nil
	})
}

// paymentMethodService implements PaymentMethodService interface
type paymentMethodService struct {
	db          *gorm.DB
	redisClient *redis.Client
	logger      logger.Logger
}

// NewPaymentMethodService creates a new payment method service instance
func NewPaymentMethodService(db *gorm.DB, redisClient *redis.Client, logger logger.Logger) PaymentMethodService {
	return &paymentMethodService{
		db:          db,
		redisClient: redisClient,
		logger:      logger,
	}
}

// CreatePaymentMethod creates a new payment method
func (s *paymentMethodService) CreatePaymentMethod(ctx context.Context, method *models.PaymentMethod) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// If this is set as default, remove default flag from other methods
		if method.IsDefault {
			if err := tx.Model(&models.PaymentMethod{}).
				Where("user_id = ? AND type = ? AND is_default = ? AND id != ?",
					method.UserID, method.Type, true, method.ID).
				Update("is_default", false).Error; err != nil {
				return fmt.Errorf("failed to update default payment methods: %w", err)
			}
		}

		if err := tx.Create(method).Error; err != nil {
			return fmt.Errorf("failed to create payment method: %w", err)
		}

		return nil
	})
}

// GetPaymentMethods retrieves payment methods for a user
func (s *paymentMethodService) GetPaymentMethods(ctx context.Context, userID string, methodType *models.PaymentMethodType) ([]*models.PaymentMethod, error) {
	query := s.db.WithContext(ctx).Where("user_id = ? AND is_active = ?", userID, true)

	if methodType != nil {
		query = query.Where("type = ?", *methodType)
	}

	var methods []*models.PaymentMethod
	if err := query.Order("is_default DESC, created_at DESC").Find(&methods).Error; err != nil {
		return nil, fmt.Errorf("failed to get payment methods: %w", err)
	}

	return methods, nil
}

// UpdatePaymentMethod updates a payment method
func (s *paymentMethodService) UpdatePaymentMethod(ctx context.Context, method *models.PaymentMethod) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// If this is set as default, remove default flag from other methods
		if method.IsDefault {
			if err := tx.Model(&models.PaymentMethod{}).
				Where("user_id = ? AND type = ? AND is_default = ? AND id != ?",
					method.UserID, method.Type, true, method.ID).
				Update("is_default", false).Error; err != nil {
				return fmt.Errorf("failed to update default payment methods: %w", err)
			}
		}

		if err := tx.Save(method).Error; err != nil {
			return fmt.Errorf("failed to update payment method: %w", err)
		}

		return nil
	})
}

// DeletePaymentMethod soft deletes a payment method
func (s *paymentMethodService) DeletePaymentMethod(ctx context.Context, methodID uuid.UUID) error {
	if err := s.db.WithContext(ctx).Delete(&models.PaymentMethod{}, methodID).Error; err != nil {
		return fmt.Errorf("failed to delete payment method: %w", err)
	}
	return nil
}

// GetPaymentMethod retrieves a payment method by ID
func (s *paymentMethodService) GetPaymentMethod(ctx context.Context, methodID uuid.UUID) (*models.PaymentMethod, error) {
	var method models.PaymentMethod
	if err := s.db.WithContext(ctx).First(&method, methodID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("payment method not found")
		}
		return nil, fmt.Errorf("failed to get payment method: %w", err)
	}
	return &method, nil
}

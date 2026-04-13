package services

import (
	"context"
	"fmt"

	"github.com/randco/randco-microservices/services/service-notification/internal/models"
	"github.com/randco/randco-microservices/services/service-notification/internal/repositories"
	"github.com/randco/randco-microservices/shared/common/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RetailerNotificationService defines the interface for retailer notification operations
type RetailerNotificationService interface {
	// Device token management
	RegisterDeviceToken(ctx context.Context, req *models.CreateDeviceTokenRequest) (*models.DeviceToken, error)
	GetRetailerDeviceTokens(ctx context.Context, retailerID string) ([]*models.DeviceToken, error)
	GetActiveDeviceTokens(ctx context.Context, retailerID string) ([]*models.DeviceToken, error)

	// Retailer notification management
	CreateRetailerNotification(ctx context.Context, req *models.CreateRetailerNotificationRequest) (*models.RetailerNotification, error)
	GetRetailerNotifications(ctx context.Context, filter *models.RetailerNotificationFilter) ([]*models.RetailerNotification, int, error)
	GetRetailerNotification(ctx context.Context, id string) (*models.RetailerNotification, error)
	MarkAsRead(ctx context.Context, retailerID string, notificationID string) error
	MarkAllAsRead(ctx context.Context, retailerID string) error
	GetUnreadCount(ctx context.Context, retailerID string) (int, error)
}

type retailerNotificationService struct {
	deviceTokenRepo   repositories.DeviceTokenRepository
	retailerNotifRepo repositories.RetailerNotificationRepository
	logger            logger.Logger
}

// NewRetailerNotificationService creates a new retailer notification service
func NewRetailerNotificationService(
	deviceTokenRepo repositories.DeviceTokenRepository,
	retailerNotifRepo repositories.RetailerNotificationRepository,
	logger logger.Logger,
) RetailerNotificationService {
	return &retailerNotificationService{
		deviceTokenRepo:   deviceTokenRepo,
		retailerNotifRepo: retailerNotifRepo,
		logger:            logger,
	}
}

// RegisterDeviceToken registers or updates a device token for push notifications
func (s *retailerNotificationService) RegisterDeviceToken(ctx context.Context, req *models.CreateDeviceTokenRequest) (*models.DeviceToken, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.retailer_notification.register_device_token")
	defer span.End()

	span.SetAttributes(
		attribute.String("retailer.id", req.RetailerID),
		attribute.String("device.id", req.DeviceID),
		attribute.String("device.platform", req.Platform),
	)

	// Create or update device token
	token, err := s.deviceTokenRepo.Create(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to register device token")
		s.logger.Error("Failed to register device token", "retailer_id", req.RetailerID, "error", err)
		return nil, fmt.Errorf("failed to register device token: %w", err)
	}

	s.logger.Info("Device token registered successfully", "retailer_id", req.RetailerID, "device_id", req.DeviceID)
	span.SetStatus(codes.Ok, "Device token registered")

	return token, nil
}

// GetRetailerDeviceTokens retrieves all device tokens for a retailer
func (s *retailerNotificationService) GetRetailerDeviceTokens(ctx context.Context, retailerID string) ([]*models.DeviceToken, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.retailer_notification.get_device_tokens")
	defer span.End()

	span.SetAttributes(attribute.String("retailer.id", retailerID))

	tokens, err := s.deviceTokenRepo.GetByRetailerID(ctx, retailerID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get device tokens")
		return nil, fmt.Errorf("failed to get device tokens: %w", err)
	}

	span.SetAttributes(attribute.Int("device_tokens.count", len(tokens)))
	span.SetStatus(codes.Ok, "Device tokens retrieved")

	return tokens, nil
}

// GetActiveDeviceTokens retrieves all active device tokens for a retailer
func (s *retailerNotificationService) GetActiveDeviceTokens(ctx context.Context, retailerID string) ([]*models.DeviceToken, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.retailer_notification.get_active_device_tokens")
	defer span.End()

	span.SetAttributes(attribute.String("retailer.id", retailerID))

	tokens, err := s.deviceTokenRepo.GetActiveTokensByRetailerID(ctx, retailerID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get active device tokens")
		return nil, fmt.Errorf("failed to get active device tokens: %w", err)
	}

	span.SetAttributes(attribute.Int("active_tokens.count", len(tokens)))
	span.SetStatus(codes.Ok, "Active device tokens retrieved")

	return tokens, nil
}

// CreateRetailerNotification creates a new retailer notification
func (s *retailerNotificationService) CreateRetailerNotification(ctx context.Context, req *models.CreateRetailerNotificationRequest) (*models.RetailerNotification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.retailer_notification.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("retailer.id", req.RetailerID),
		attribute.String("notification.type", string(req.Type)),
		attribute.String("notification.title", req.Title),
	)

	notification, err := s.retailerNotifRepo.Create(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create retailer notification")
		s.logger.Error("Failed to create retailer notification", "retailer_id", req.RetailerID, "error", err)
		return nil, fmt.Errorf("failed to create retailer notification: %w", err)
	}

	s.logger.Info("Retailer notification created successfully", "retailer_id", req.RetailerID, "type", req.Type)
	span.SetStatus(codes.Ok, "Retailer notification created")

	return notification, nil
}

// GetRetailerNotifications retrieves retailer notifications with filtering and pagination
func (s *retailerNotificationService) GetRetailerNotifications(ctx context.Context, filter *models.RetailerNotificationFilter) ([]*models.RetailerNotification, int, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.retailer_notification.get_notifications")
	defer span.End()

	span.SetAttributes(
		attribute.String("retailer.id", filter.RetailerID),
		attribute.Int("page", filter.Page),
		attribute.Int("page_size", filter.PageSize),
		attribute.Bool("unread_only", filter.UnreadOnly),
	)

	notifications, total, err := s.retailerNotifRepo.List(ctx, filter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get retailer notifications")
		return nil, 0, fmt.Errorf("failed to get retailer notifications: %w", err)
	}

	span.SetAttributes(
		attribute.Int("notifications.count", len(notifications)),
		attribute.Int("notifications.total", total),
	)
	span.SetStatus(codes.Ok, "Retailer notifications retrieved")

	return notifications, total, nil
}

// GetRetailerNotification retrieves a single retailer notification by ID
func (s *retailerNotificationService) GetRetailerNotification(ctx context.Context, id string) (*models.RetailerNotification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.retailer_notification.get_notification")
	defer span.End()

	span.SetAttributes(attribute.String("notification.id", id))

	notification, err := s.retailerNotifRepo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get retailer notification")
		return nil, fmt.Errorf("failed to get retailer notification: %w", err)
	}

	if notification == nil {
		span.SetStatus(codes.Error, "Notification not found")
		return nil, fmt.Errorf("notification not found")
	}

	span.SetStatus(codes.Ok, "Retailer notification retrieved")

	return notification, nil
}

// MarkAsRead marks a notification as read
func (s *retailerNotificationService) MarkAsRead(ctx context.Context, retailerID string, notificationID string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.retailer_notification.mark_as_read")
	defer span.End()

	span.SetAttributes(
		attribute.String("retailer.id", retailerID),
		attribute.String("notification.id", notificationID),
	)

	err := s.retailerNotifRepo.MarkAsRead(ctx, retailerID, notificationID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to mark notification as read")
		s.logger.Error("Failed to mark notification as read", "notification_id", notificationID, "retailer_id", retailerID, "error", err)
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	s.logger.Info("Notification marked as read", "notification_id", notificationID, "retailer_id", retailerID)
	span.SetStatus(codes.Ok, "Notification marked as read")

	return nil
}

// MarkAllAsRead marks all unread notifications as read for a retailer
func (s *retailerNotificationService) MarkAllAsRead(ctx context.Context, retailerID string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.retailer_notification.mark_all_as_read")
	defer span.End()

	span.SetAttributes(attribute.String("retailer.id", retailerID))

	err := s.retailerNotifRepo.MarkAllAsRead(ctx, retailerID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to mark all notifications as read")
		s.logger.Error("Failed to mark all notifications as read", "retailer_id", retailerID, "error", err)
		return fmt.Errorf("failed to mark all notifications as read: %w", err)
	}

	s.logger.Info("All notifications marked as read", "retailer_id", retailerID)
	span.SetStatus(codes.Ok, "All notifications marked as read")

	return nil
}

// GetUnreadCount retrieves the count of unread notifications for a retailer
func (s *retailerNotificationService) GetUnreadCount(ctx context.Context, retailerID string) (int, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.retailer_notification.get_unread_count")
	defer span.End()

	span.SetAttributes(attribute.String("retailer.id", retailerID))

	count, err := s.retailerNotifRepo.GetUnreadCount(ctx, retailerID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get unread count")
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}

	span.SetAttributes(attribute.Int("unread_count", count))
	span.SetStatus(codes.Ok, "Unread count retrieved")

	return count, nil
}

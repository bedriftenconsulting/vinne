package services

import (
	"context"
	"fmt"

	"github.com/randco/randco-microservices/services/service-notification/internal/models"
	"github.com/randco/randco-microservices/services/service-notification/internal/providers/push"
	"github.com/randco/randco-microservices/services/service-notification/internal/repositories"
	"github.com/randco/randco-microservices/shared/common/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// PushNotificationService defines the interface for push notification operations
type PushNotificationService interface {
	SendPushToRetailer(ctx context.Context, retailerID string, title string, body string, data map[string]string) error
	SendPushNotificationWithHistory(ctx context.Context, req *models.CreateRetailerNotificationRequest) error
}

type pushNotificationService struct {
	firebaseProvider  *push.FirebaseProvider
	deviceTokenRepo   repositories.DeviceTokenRepository
	retailerNotifRepo repositories.RetailerNotificationRepository
	logger            logger.Logger
}

// NewPushNotificationService creates a new push notification service
func NewPushNotificationService(
	firebaseProvider *push.FirebaseProvider,
	deviceTokenRepo repositories.DeviceTokenRepository,
	retailerNotifRepo repositories.RetailerNotificationRepository,
	logger logger.Logger,
) PushNotificationService {
	return &pushNotificationService{
		firebaseProvider:  firebaseProvider,
		deviceTokenRepo:   deviceTokenRepo,
		retailerNotifRepo: retailerNotifRepo,
		logger:            logger,
	}
}

// SendPushToRetailer sends a push notification to all active devices for a retailer
func (s *pushNotificationService) SendPushToRetailer(ctx context.Context, retailerID string, title string, body string, data map[string]string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.push_notification.send_to_retailer")
	defer span.End()

	span.SetAttributes(
		attribute.String("retailer.id", retailerID),
		attribute.String("notification.title", title),
	)

	// Get all active device tokens for retailer
	tokens, err := s.deviceTokenRepo.GetActiveTokensByRetailerID(ctx, retailerID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get device tokens")
		return fmt.Errorf("failed to get device tokens: %w", err)
	}

	if len(tokens) == 0 {
		s.logger.Info("No active device tokens found for retailer", "retailer_id", retailerID)
		span.SetStatus(codes.Ok, "No active devices")
		return nil
	}

	// Send push notification to each device
	successCount := 0
	for _, token := range tokens {
		req := &push.PushNotificationRequest{
			DeviceToken: token.FCMToken,
			Title:       title,
			Body:        body,
			Data:        data,
			Priority:    "high",
		}

		resp, err := s.firebaseProvider.SendPushNotification(ctx, req)
		if err != nil {
			s.logger.Error("Failed to send push notification",
				"retailer_id", retailerID,
				"device_id", token.DeviceID,
				"error", err,
			)
			// Mark token as inactive if send fails (might be invalid)
			if err := s.deviceTokenRepo.MarkAsInactive(ctx, token.DeviceID); err != nil {
				s.logger.Error("Failed to mark token as inactive",
					"device_id", token.DeviceID,
					"error", err,
				)
			}
			continue
		}

		if resp.Success {
			successCount++
			// Update last used timestamp
			if err := s.deviceTokenRepo.UpdateLastUsed(ctx, token.DeviceID); err != nil {
				s.logger.Error("Failed to update last used timestamp",
					"device_id", token.DeviceID,
					"error", err,
				)
			}
		}
	}

	span.SetAttributes(
		attribute.Int("devices.total", len(tokens)),
		attribute.Int("devices.success", successCount),
	)
	span.SetStatus(codes.Ok, "Push notifications sent")

	s.logger.Info("Push notifications sent to retailer",
		"retailer_id", retailerID,
		"total_devices", len(tokens),
		"success_count", successCount,
	)

	return nil
}

// SendPushNotificationWithHistory sends a push notification and creates a history record
func (s *pushNotificationService) SendPushNotificationWithHistory(ctx context.Context, req *models.CreateRetailerNotificationRequest) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.push_notification.send_with_history")
	defer span.End()

	span.SetAttributes(
		attribute.String("retailer.id", req.RetailerID),
		attribute.String("notification.type", string(req.Type)),
		attribute.String("notification.title", req.Title),
	)

	// Create notification history record first
	notification, err := s.retailerNotifRepo.Create(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create notification history")
		s.logger.Error("Failed to create notification history", "retailer_id", req.RetailerID, "error", err)
		return fmt.Errorf("failed to create notification history: %w", err)
	}

	// Build data payload
	data := make(map[string]string)
	data["type"] = string(req.Type)
	data["notification_id"] = notification.ID
	if req.TransactionID != nil {
		data["transaction_id"] = *req.TransactionID
	}
	if req.Amount != nil {
		data["amount"] = fmt.Sprintf("%d", *req.Amount)
	}

	// Send push notification
	err = s.SendPushToRetailer(ctx, req.RetailerID, req.Title, req.Body, data)
	if err != nil {
		span.RecordError(err)
		// Don't fail the entire operation if push fails - history is still saved
		s.logger.Error("Failed to send push notification", "retailer_id", req.RetailerID, "error", err)
	}

	span.SetStatus(codes.Ok, "Notification sent with history")

	return nil
}

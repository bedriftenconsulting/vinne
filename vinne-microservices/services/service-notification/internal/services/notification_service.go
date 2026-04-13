package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-notification/internal/models"
	"github.com/randco/randco-microservices/services/service-notification/internal/repositories"
	"github.com/randco/randco-microservices/shared/common/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type NotificationService interface {
	CreateNotification(ctx context.Context, req models.CreateNotificationRequest) (*models.Notification, error)
	CreateManyNotifications(ctx context.Context, reqs []models.CreateNotificationRequest) ([]*models.Notification, error)
	GetNotification(ctx context.Context, id string) (*models.Notification, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*models.Notification, error)
	UpdateNotification(ctx context.Context, req models.UpdateNotificationRequest) (*models.Notification, error)
	DeleteNotification(ctx context.Context, id string) error
	ListNotifications(ctx context.Context, filter models.NotificationFilter, page, limit int) ([]*models.Notification, int64, error)
	GetNotificationsByStatus(ctx context.Context, status models.NotificationStatus) ([]*models.Notification, error)
	MarkAsSent(ctx context.Context, id string, providerName, providerMessageID string, providerResponse any) error
	MarkAsDelivered(ctx context.Context, id string) error
	MarkAsFailed(ctx context.Context, id string, errorMessage string) error
	RetryNotification(ctx context.Context, id string) (*models.Notification, error)
}

type notificationService struct {
	notificationRepo repositories.NotificationRepository
	logger           logger.Logger
}

func NewNotificationService(
	notificationRepo repositories.NotificationRepository,
	logger logger.Logger,
) NotificationService {
	return &notificationService{
		notificationRepo: notificationRepo,
		logger:           logger,
	}
}

func (s *notificationService) CreateNotification(ctx context.Context, req models.CreateNotificationRequest) (*models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.notification.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("notification.type", string(req.Type)),
		attribute.Int("notification.recipients_count", len(req.Recipients)),
		attribute.String("notification.idempotency_key", req.IdempotencyKey),
	)

	if req.IdempotencyKey != "" {
		existing, err := s.notificationRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
		if err == nil {
			span.SetAttributes(attribute.Bool("notification.duplicate", true))
			return existing, nil
		}
	}

	notification := &models.Notification{
		ID:             uuid.New().String(),
		IdempotencyKey: req.IdempotencyKey,
		Type:           req.Type,
		Subject:        req.Subject,
		Content:        req.Content,
		Status:         models.NotificationStatusQueued,
		Provider:       req.Provider,
		ScheduledFor:   req.ScheduledFor,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Variables:      req.Variables,
		TemplateID:     req.TemplateID,
	}

	for _, recipient := range req.Recipients {
		notification.Recipient = append(notification.Recipient, models.Recipient{
			NotificationID: notification.ID,
			Type:           models.RecipientTypeTo,
			Address:        recipient.Address,
			CreatedAt:      time.Now(),
		})
	}

	for _, cc := range req.CC {
		notification.CC = append(notification.CC, models.Recipient{
			NotificationID: notification.ID,
			Type:           models.RecipientTypeCC,
			Address:        cc.Address,
			CreatedAt:      time.Now(),
		})
	}

	for _, bcc := range req.BCC {
		notification.BCC = append(notification.BCC, models.Recipient{
			NotificationID: notification.ID,
			Type:           models.RecipientTypeBCC,
			Address:        bcc.Address,
			CreatedAt:      time.Now(),
		})
	}

	if err := s.notificationRepo.Create(ctx, notification); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create notification")
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	span.SetAttributes(attribute.String("notification.id", notification.ID))
	s.logger.Info("Notification created successfully", "notification_id", notification.ID, "type", notification.Type)
	return notification, nil
}

func (s *notificationService) CreateManyNotifications(ctx context.Context, reqs []models.CreateNotificationRequest) ([]*models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.notification.create_many")
	defer span.End()

	span.SetAttributes(
		attribute.Int("notification.batch_size", len(reqs)),
	)

	if len(reqs) == 0 {
		return []*models.Notification{}, nil
	}

	notifications := make([]*models.Notification, 0, len(reqs))
	duplicates := make(map[string]*models.Notification)

	// First check for duplicates by idempotency key
	for i, req := range reqs {
		if req.IdempotencyKey != "" {
			existing, err := s.notificationRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
			if err == nil {
				duplicates[req.IdempotencyKey] = existing
				span.SetAttributes(attribute.Bool(fmt.Sprintf("notification.%d.duplicate", i), true))
				continue
			}
		}

		notification := &models.Notification{
			ID:             uuid.New().String(),
			IdempotencyKey: req.IdempotencyKey,
			Type:           req.Type,
			Subject:        req.Subject,
			Content:        req.Content,
			Status:         models.NotificationStatusQueued,
			Provider:       req.Provider,
			ScheduledFor:   req.ScheduledFor,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
			Variables:      req.Variables,
			TemplateID:     req.TemplateID,
		}

		for _, recipient := range req.Recipients {
			notification.Recipient = append(notification.Recipient, models.Recipient{
				NotificationID: notification.ID,
				Type:           models.RecipientTypeTo,
				Address:        recipient.Address,
				CreatedAt:      time.Now(),
			})
		}

		for _, cc := range req.CC {
			notification.CC = append(notification.CC, models.Recipient{
				NotificationID: notification.ID,
				Type:           models.RecipientTypeCC,
				Address:        cc.Address,
				CreatedAt:      time.Now(),
			})
		}

		for _, bcc := range req.BCC {
			notification.BCC = append(notification.BCC, models.Recipient{
				NotificationID: notification.ID,
				Type:           models.RecipientTypeBCC,
				Address:        bcc.Address,
				CreatedAt:      time.Now(),
			})
		}

		notifications = append(notifications, notification)
	}

	// Bulk create new notifications
	if len(notifications) > 0 {
		if err := s.notificationRepo.CreateMany(ctx, notifications); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create notifications")
			return nil, fmt.Errorf("failed to create notifications: %w", err)
		}

		s.logger.Info("Notifications created successfully", "count", len(notifications))
	}

	// Combine results maintaining original order
	result := make([]*models.Notification, 0, len(reqs))
	notificationIdx := 0

	for _, req := range reqs {
		if req.IdempotencyKey != "" {
			if duplicate, exists := duplicates[req.IdempotencyKey]; exists {
				result = append(result, duplicate)
				continue
			}
		}
		result = append(result, notifications[notificationIdx])
		notificationIdx++
	}

	span.SetAttributes(
		attribute.Int("notification.created_count", len(notifications)),
		attribute.Int("notification.duplicate_count", len(duplicates)),
	)
	return result, nil
}

func (s *notificationService) GetNotification(ctx context.Context, id string) (*models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.notification.get")
	defer span.End()

	span.SetAttributes(attribute.String("notification.id", id))

	notification, err := s.notificationRepo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get notification")
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	span.SetAttributes(
		attribute.Bool("notification.found", true),
		attribute.String("notification.type", string(notification.Type)),
		attribute.String("notification.status", string(notification.Status)),
	)
	return notification, nil
}

func (s *notificationService) UpdateNotification(ctx context.Context, req models.UpdateNotificationRequest) (*models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.notification.update")
	defer span.End()

	span.SetAttributes(attribute.String("notification.id", req.ID))

	notification, err := s.notificationRepo.GetByID(ctx, req.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "notification not found")
		return nil, fmt.Errorf("notification not found: %w", err)
	}

	if req.Subject != nil {
		notification.Subject = *req.Subject
	}
	if req.Content != nil {
		notification.Content = *req.Content
	}
	if req.Status != nil {
		notification.Status = *req.Status
	}
	if req.ScheduledFor != nil {
		notification.ScheduledFor = req.ScheduledFor
	}
	if req.Provider != nil {
		notification.Provider = *req.Provider
	}
	if req.ProviderResponse != nil {
		notification.ProviderResponse = req.ProviderResponse
	}

	notification.UpdatedAt = time.Now()

	if err := s.notificationRepo.Update(ctx, notification); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update notification")
		return nil, fmt.Errorf("failed to update notification: %w", err)
	}

	return notification, nil
}

func (s *notificationService) DeleteNotification(ctx context.Context, id string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.notification.delete")
	defer span.End()

	span.SetAttributes(attribute.String("notification.id", id))

	if err := s.notificationRepo.Delete(ctx, id); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete notification")
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	return nil
}

func (s *notificationService) ListNotifications(ctx context.Context, filter models.NotificationFilter, page, limit int) ([]*models.Notification, int64, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.notification.list")
	defer span.End()

	span.SetAttributes(
		attribute.Int("page", page),
		attribute.Int("limit", limit),
	)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	notifications, total, err := s.notificationRepo.List(ctx, filter, page, limit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to list notifications")
		return nil, 0, fmt.Errorf("failed to list notifications: %w", err)
	}

	span.SetAttributes(
		attribute.Int("result.count", len(notifications)),
		attribute.Int64("result.total", total),
	)
	return notifications, total, nil
}

func (s *notificationService) GetNotificationsByStatus(ctx context.Context, status models.NotificationStatus) ([]*models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.notification.get_by_status")
	defer span.End()

	span.SetAttributes(attribute.String("notification.status", string(status)))

	notifications, err := s.notificationRepo.GetByStatus(ctx, status)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get notifications by status")
		return nil, fmt.Errorf("failed to get notifications by status: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(notifications)))
	return notifications, nil
}

func (s *notificationService) MarkAsSent(ctx context.Context, id string, providerName, providerMessageID string, providerResponse any) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.notification.mark_as_sent")
	defer span.End()

	span.SetAttributes(
		attribute.String("notification.id", id),
		attribute.String("provider.name", providerName),
		attribute.String("provider.message_id", providerMessageID),
	)

	notification, err := s.notificationRepo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get notification: %w", err)
	}

	now := time.Now()
	notification.Status = models.NotificationStatusSent
	notification.Provider = providerName
	notification.ProviderMessageID = &providerMessageID
	notification.ProviderResponse = providerResponse
	notification.SentAt = &now
	notification.UpdatedAt = now

	if err := s.notificationRepo.Update(ctx, notification); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update notification: %w", err)
	}

	return nil
}

func (s *notificationService) MarkAsDelivered(ctx context.Context, id string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.notification.mark_as_delivered")
	defer span.End()

	span.SetAttributes(attribute.String("notification.id", id))

	notification, err := s.notificationRepo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get notification: %w", err)
	}

	now := time.Now()
	notification.Status = models.NotificationStatusDelivered
	notification.DeliveredAt = &now
	notification.UpdatedAt = now

	if err := s.notificationRepo.Update(ctx, notification); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update notification: %w", err)
	}

	return nil
}

func (s *notificationService) MarkAsFailed(ctx context.Context, id string, errorMessage string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.notification.mark_as_failed")
	defer span.End()

	span.SetAttributes(
		attribute.String("notification.id", id),
		attribute.String("error.message", errorMessage),
	)

	notification, err := s.notificationRepo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get notification: %w", err)
	}

	now := time.Now()
	notification.Status = models.NotificationStatusFailed
	notification.ErrorMessage = &errorMessage
	notification.FailedAt = &now
	notification.RetryCount++
	notification.UpdatedAt = now

	if err := s.notificationRepo.Update(ctx, notification); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update notification: %w", err)
	}

	return nil
}

func (s *notificationService) RetryNotification(ctx context.Context, id string) (*models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.notification.retry")
	defer span.End()

	span.SetAttributes(attribute.String("notification.id", id))

	notification, err := s.notificationRepo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	if notification.Status != models.NotificationStatusFailed {
		return nil, fmt.Errorf("notification must be in failed status to retry")
	}

	now := time.Now()
	notification.Status = models.NotificationStatusQueued
	notification.ErrorMessage = nil
	notification.FailedAt = nil
	notification.UpdatedAt = now

	if err := s.notificationRepo.Update(ctx, notification); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to update notification: %w", err)
	}

	s.logger.Info("Notification marked for retry", "notification_id", notification.ID)
	span.SetAttributes(attribute.Bool("retry_queued", true))

	return notification, nil
}

func (s *notificationService) GetByIdempotencyKey(ctx context.Context, key string) (*models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "service.notification.get_by_idempotency_key")
	defer span.End()

	span.SetAttributes(attribute.String("idempotency.key", key))

	notification, err := s.notificationRepo.GetByIdempotencyKey(ctx, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get notification by idempotency key")
		return nil, fmt.Errorf("failed to get notification by idempotency key: %w", err)
	}

	span.SetAttributes(
		attribute.Bool("notification.found", true),
		attribute.String("notification.id", notification.ID),
		attribute.String("notification.type", string(notification.Type)),
		attribute.String("notification.status", string(notification.Status)),
	)
	return notification, nil
}

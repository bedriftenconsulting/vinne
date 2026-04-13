package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/randco/randco-microservices/services/service-notification/internal/metrics"
	"github.com/randco/randco-microservices/services/service-notification/internal/models"
	"github.com/randco/randco-microservices/services/service-notification/internal/providers"
	"github.com/randco/randco-microservices/services/service-notification/internal/queue"
	"github.com/randco/randco-microservices/services/service-notification/internal/ratelimit"
	"github.com/randco/randco-microservices/services/service-notification/internal/templates"
	"github.com/randco/randco-microservices/shared/idempotency"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type SendNotificationService interface {
	SendEmail(ctx context.Context, req *models.CreateNotificationRequest) (*models.Notification, error)
	SendBulkEmail(ctx context.Context, req *[]models.CreateNotificationRequest) error
	SendSMS(ctx context.Context, req *models.CreateNotificationRequest) (*models.Notification, error)
	SendBulkSMS(ctx context.Context, req *[]models.CreateNotificationRequest) error
	SendPush(ctx context.Context, req *models.CreateNotificationRequest) (*models.Notification, error)
	SendBulkPush(ctx context.Context, req *[]models.CreateNotificationRequest) error
}

type sendNotificationService struct {
	notificationService NotificationService
	templateService     *templates.NotificationTemplateService
	providerManager     *providers.ProviderManager
	rateLimiter         *ratelimit.RateLimiter
	queueManager        queue.QueueManager
	idempotencyStore    idempotency.IdempotencyStore
	metrics             metrics.MetricsInterface
}

func NewSendNotificationService(
	notificationService NotificationService,
	templateService *templates.NotificationTemplateService,
	providerManager *providers.ProviderManager,
	rateLimiter *ratelimit.RateLimiter,
	queueManager queue.QueueManager,
	idempotencyStore idempotency.IdempotencyStore,
	metrics metrics.MetricsInterface,
) SendNotificationService {
	return &sendNotificationService{
		notificationService: notificationService,
		templateService:     templateService,
		providerManager:     providerManager,
		rateLimiter:         rateLimiter,
		queueManager:        queueManager,
		idempotencyStore:    idempotencyStore,
		metrics:             metrics,
	}
}

func (s *sendNotificationService) SendEmail(ctx context.Context, req *models.CreateNotificationRequest) (*models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "send_service.email.send")
	defer span.End()

	span.SetAttributes(
		attribute.String("notification.type", "email"),
		attribute.String("template.id", req.TemplateID),
		attribute.Int("recipients.count", len(req.Recipients)),
	)

	notification, isDuplicate, err := s.processNotification(ctx, req, models.NotificationTypeEmail)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to process email notification")
		s.metrics.RecordNotificationFailed("email", req.Provider, err.Error())
		return nil, fmt.Errorf("failed to process email notification: %w", err)
	}

	if isDuplicate {
		span.SetAttributes(attribute.Bool("notification.duplicate", true))
		s.metrics.RecordIdempotency("email", true)
		return notification, nil
	}

	s.metrics.RecordIdempotency("email", false)

	// Check rate limit BEFORE sending (fail fast)
	if err := s.rateLimiter.Allow(ctx, "email"); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "rate limit exceeded - queueing for later")
		span.SetAttributes(attribute.Bool("notification.queued", true))
		s.metrics.RecordNotificationFailed("email", req.Provider, "rate_limit_exceeded_queued")

		// Queue for later processing instead of failing
		if enqueueErr := s.enqueueForLater(ctx, notification); enqueueErr != nil {
			// If enqueue fails, mark as failed
			if markErr := s.notificationService.MarkAsFailed(ctx, notification.ID, fmt.Sprintf("rate limit exceeded and failed to queue: %v", enqueueErr)); markErr != nil {
				span.RecordError(markErr)
			}
			if req.IdempotencyKey != "" {
				if markErr := s.idempotencyStore.MarkFailed(ctx, req.IdempotencyKey, enqueueErr); markErr != nil {
					span.RecordError(markErr)
				}
			}
			return nil, fmt.Errorf("rate limit exceeded and failed to queue: %w", enqueueErr)
		}

		// Successfully queued - return notification with queued status
		notification.Status = models.NotificationStatusQueued
		span.SetAttributes(attribute.String("notification.id", notification.ID))
		return notification, nil
	}

	// Send email directly to provider
	if len(notification.Recipient) == 0 {
		span.RecordError(fmt.Errorf("no recipients found"))
		span.SetStatus(codes.Error, "no recipients found")
		return nil, fmt.Errorf("no recipients found for email notification")
	}

	emailReq := &providers.EmailRequest{
		To:          notification.Recipient[0].Address,
		Subject:     notification.Subject,
		HTMLContent: notification.Content,
		Priority:    providers.PriorityNormal,
	}

	// Add CC and BCC if present
	if len(notification.CC) > 0 {
		ccAddresses := make([]string, len(notification.CC))
		for i, cc := range notification.CC {
			ccAddresses[i] = cc.Address
		}
		emailReq.CC = ccAddresses
	}
	if len(notification.BCC) > 0 {
		bccAddresses := make([]string, len(notification.BCC))
		for i, bcc := range notification.BCC {
			bccAddresses[i] = bcc.Address
		}
		emailReq.BCC = bccAddresses
	}

	response, err := s.providerManager.SendEmail(ctx, emailReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send email")
		s.metrics.RecordNotificationFailed("email", req.Provider, err.Error())

		// Release rate limit counter since send failed
		if releaseErr := s.rateLimiter.Release(ctx, "email"); releaseErr != nil {
			span.RecordError(releaseErr)
		}

		// Mark idempotency as failed
		if req.IdempotencyKey != "" {
			if markErr := s.idempotencyStore.MarkFailed(ctx, req.IdempotencyKey, err); markErr != nil {
				span.RecordError(markErr)
			}
		}

		return nil, fmt.Errorf("failed to send email: %w", err)
	}

	// Mark notification as sent
	if err := s.notificationService.MarkAsSent(ctx, notification.ID, response.Provider, response.MessageID, response); err != nil {
		span.RecordError(err)
		// Log error but don't fail the entire operation
		span.SetAttributes(attribute.String("mark_as_sent.error", err.Error()))
	}

	// Mark idempotency record as completed
	if req.IdempotencyKey != "" {
		if err := s.idempotencyStore.MarkCompleted(ctx, req.IdempotencyKey, response); err != nil {
			span.RecordError(err)
			// Log error but don't fail the entire operation
			span.SetAttributes(attribute.String("idempotency_completion.error", err.Error()))
		}
	}

	span.SetAttributes(
		attribute.String("provider.message_id", response.MessageID),
		attribute.String("provider.status", string(response.Status)),
		attribute.String("provider.name", response.Provider),
	)
	s.metrics.RecordNotificationSent("email", response.Provider)

	span.SetAttributes(attribute.String("notification.id", notification.ID))
	return notification, nil
}

func (s *sendNotificationService) SendBulkEmail(ctx context.Context, req *[]models.CreateNotificationRequest) error {
	//nolint:ineffassign,staticcheck // SA4006: Context passed to tracer, old value used in call
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "send_service.email.bulk_send")
	defer span.End()

	span.SetAttributes(
		attribute.String("notification.type", "email"),
		attribute.String("template.id", (*req)[0].TemplateID),
		attribute.Int("recipients.count", len(*req)),
		attribute.Bool("bulk.enabled", true),
	)

	s.metrics.RecordBulkNotificationStart("email", len(*req))

	go func() {
		start := time.Now()
		defer func() {
			s.metrics.RecordBulkNotificationComplete("email", time.Since(start))
		}()

		// Process notifications and send directly
		notifications, err := s.processNotificationBulk(context.Background(), req, models.NotificationTypeEmail)
		if err != nil {
			span.RecordError(err)
			s.metrics.RecordNotificationFailed("email", "bulk", err.Error())
			return
		}

		// Send emails directly using goroutines with concurrency control
		s.sendBulkEmailsDirectly(context.Background(), notifications)
	}()

	return nil
}

func (s *sendNotificationService) SendSMS(ctx context.Context, req *models.CreateNotificationRequest) (*models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "send_service.sms.send")
	defer span.End()

	span.SetAttributes(
		attribute.String("notification.type", "sms"),
		attribute.String("template.id", req.TemplateID),
		attribute.Int("recipients.count", len(req.Recipients)),
	)

	notification, isDuplicate, err := s.processNotification(ctx, req, models.NotificationTypeSMS)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to process SMS notification")
		s.metrics.RecordNotificationFailed("sms", req.Provider, err.Error())
		return nil, fmt.Errorf("failed to process SMS notification: %w", err)
	}

	if isDuplicate {
		span.SetAttributes(attribute.Bool("notification.duplicate", true))
		s.metrics.RecordIdempotency("sms", true)
		return notification, nil
	}

	s.metrics.RecordIdempotency("sms", false)

	// Check rate limit BEFORE sending (fail fast)
	if err := s.rateLimiter.Allow(ctx, "sms"); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "rate limit exceeded - queueing for later")
		span.SetAttributes(attribute.Bool("notification.queued", true))
		s.metrics.RecordNotificationFailed("sms", req.Provider, "rate_limit_exceeded_queued")

		// Queue for later processing instead of failing
		if enqueueErr := s.enqueueForLater(ctx, notification); enqueueErr != nil {
			// If enqueue fails, mark as failed
			if markErr := s.notificationService.MarkAsFailed(ctx, notification.ID, fmt.Sprintf("rate limit exceeded and failed to queue: %v", enqueueErr)); markErr != nil {
				span.RecordError(markErr)
			}
			if req.IdempotencyKey != "" {
				if markErr := s.idempotencyStore.MarkFailed(ctx, req.IdempotencyKey, enqueueErr); markErr != nil {
					span.RecordError(markErr)
				}
			}
			return nil, fmt.Errorf("rate limit exceeded and failed to queue: %w", enqueueErr)
		}

		// Successfully queued - return notification with queued status
		notification.Status = models.NotificationStatusQueued
		span.SetAttributes(attribute.String("notification.id", notification.ID))
		return notification, nil
	}

	if len(notification.Recipient) == 0 {
		span.RecordError(fmt.Errorf("no recipients found"))
		span.SetStatus(codes.Error, "no recipients found")
		return nil, fmt.Errorf("no recipients found for SMS notification")
	}

	smsReq := &providers.SMSRequest{
		To:       notification.Recipient[0].Address,
		Content:  notification.Content,
		Priority: providers.PriorityNormal,
	}

	response, err := s.providerManager.SendSMS(ctx, smsReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send SMS")
		s.metrics.RecordNotificationFailed("sms", req.Provider, err.Error())

		// Release rate limit counter since send failed
		if releaseErr := s.rateLimiter.Release(ctx, "sms"); releaseErr != nil {
			span.RecordError(releaseErr)
		}

		if req.IdempotencyKey != "" {
			if markErr := s.idempotencyStore.MarkFailed(ctx, req.IdempotencyKey, err); markErr != nil {
				span.RecordError(markErr)
			}
		}

		return nil, fmt.Errorf("failed to send SMS: %w", err)
	}

	if err := s.notificationService.MarkAsSent(ctx, notification.ID, response.Provider, response.MessageID, response); err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("mark_as_sent.error", err.Error()))
	}

	if req.IdempotencyKey != "" {
		if err := s.idempotencyStore.MarkCompleted(ctx, req.IdempotencyKey, response); err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("idempotency_completion.error", err.Error()))
		}
	}

	span.SetAttributes(
		attribute.String("provider.message_id", response.MessageID),
		attribute.String("provider.status", string(response.Status)),
		attribute.String("provider.name", response.Provider),
	)
	s.metrics.RecordNotificationSent("sms", response.Provider)

	span.SetAttributes(attribute.String("notification.id", notification.ID))
	return notification, nil
}

func (s *sendNotificationService) SendBulkSMS(ctx context.Context, req *[]models.CreateNotificationRequest) error {
	//nolint:ineffassign,staticcheck // SA4006: Context passed to tracer, old value used in call
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "send_service.sms.bulk_send")
	defer span.End()

	span.SetAttributes(
		attribute.String("notification.type", "sms"),
		attribute.String("template.id", (*req)[0].TemplateID),
		attribute.Int("recipients.count", len(*req)),
		attribute.Bool("bulk.enabled", true),
	)

	s.metrics.RecordBulkNotificationStart("sms", len(*req))

	go func() {
		start := time.Now()
		defer func() {
			s.metrics.RecordBulkNotificationComplete("sms", time.Since(start))
		}()

		// Process notifications and send directly
		notifications, err := s.processNotificationBulk(context.Background(), req, models.NotificationTypeSMS)
		if err != nil {
			span.RecordError(err)
			s.metrics.RecordNotificationFailed("sms", "bulk", err.Error())
			return
		}

		// Send SMS messages directly using goroutines with concurrency control
		s.sendBulkSMSDirectly(context.Background(), notifications)
	}()

	return nil
}

func (s *sendNotificationService) SendPush(ctx context.Context, req *models.CreateNotificationRequest) (*models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "send_service.push.send")
	defer span.End()

	span.SetAttributes(
		attribute.String("notification.type", "push"),
		attribute.String("template.id", req.TemplateID),
		attribute.Int("recipients.count", len(req.Recipients)),
	)

	notification, isDuplicate, err := s.processNotification(ctx, req, models.NotificationTypePush)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to process push notification")
		s.metrics.RecordNotificationFailed("push", req.Provider, err.Error())
		return nil, fmt.Errorf("failed to process push notification: %w", err)
	}

	if isDuplicate {
		span.SetAttributes(attribute.Bool("notification.duplicate", true))
		s.metrics.RecordIdempotency("push", true)
		return notification, nil
	}

	s.metrics.RecordIdempotency("push", false)

	// For now, push notifications are not implemented
	// TODO: Implement push notification provider
	span.SetAttributes(
		attribute.String("push.to", notification.Recipient[0].Address),
		attribute.String("push.title", notification.Subject),
		attribute.String("push.content", notification.Content),
	)

	// Mark notification as failed since push is not implemented
	if err := s.notificationService.MarkAsFailed(ctx, notification.ID, "push notifications not yet implemented"); err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("mark_as_failed.error", err.Error()))
	}

	// Mark idempotency as failed
	if req.IdempotencyKey != "" {
		if err := s.idempotencyStore.MarkFailed(ctx, req.IdempotencyKey, fmt.Errorf("push notifications not yet implemented")); err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("idempotency_failure.error", err.Error()))
		}
	}

	span.SetAttributes(
		attribute.String("provider.status", "not_implemented"),
	)
	s.metrics.RecordNotificationFailed("push", "not_implemented", "push notifications not yet implemented")

	span.SetAttributes(attribute.String("notification.id", notification.ID))
	return notification, fmt.Errorf("push notifications are not yet implemented")
}

func (s *sendNotificationService) SendBulkPush(ctx context.Context, req *[]models.CreateNotificationRequest) error {
	//nolint:ineffassign,staticcheck // SA4006: Context passed to tracer, old value used in call
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "send_service.push.bulk_send")
	defer span.End()

	span.SetAttributes(
		attribute.String("notification.type", "push"),
		attribute.String("template.id", (*req)[0].TemplateID),
		attribute.Int("recipients.count", len(*req)),
		attribute.Bool("bulk.enabled", true),
	)

	s.metrics.RecordBulkNotificationStart("push", len(*req))

	go func() {
		start := time.Now()
		notifications, err := s.processNotificationBulk(context.Background(), req, models.NotificationTypePush)
		if err != nil {
			span.RecordError(err)
			s.metrics.RecordNotificationFailed("push", "bulk", err.Error())
			return
		}
		s.metrics.RecordBulkNotificationComplete("push", time.Since(start))
		// For push notifications, we'll mark them as failed since push is not implemented
		s.markBulkPushAsFailed(context.Background(), notifications)
	}()

	return nil
}

func (s *sendNotificationService) processNotification(ctx context.Context, req *models.CreateNotificationRequest, notificationType models.NotificationType) (*models.Notification, bool, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "send_service.process_notification")
	defer span.End()

	if req.IdempotencyKey != "" {
		requestHash := s.generateRequestHash(req)
		record, err := s.idempotencyStore.CheckAndCreate(ctx, req.IdempotencyKey, requestHash)

		if err != nil {
			span.RecordError(err)
			return nil, false, fmt.Errorf("idempotency check failed: %w", err)
		}

		if record.Status == idempotency.StatusCompleted {
			// Retrieve cached notification from database instead of returning error
			cachedNotification, err := s.notificationService.GetByIdempotencyKey(ctx, req.IdempotencyKey)
			if err != nil {
				span.RecordError(err)
				return nil, false, fmt.Errorf("failed to retrieve cached notification: %w", err)
			}

			span.SetAttributes(
				attribute.Bool("notification.duplicate", true),
				attribute.String("cached_notification_id", cachedNotification.ID),
				attribute.String("idempotency.key", req.IdempotencyKey),
			)

			return cachedNotification, true, nil
		}

	}

	req.Type = notificationType
	if req.Content == "" && req.TemplateID != "" {
		templ, err := s.processTemplate(ctx, req)
		if err != nil {
			span.RecordError(err)
			return nil, false, fmt.Errorf("template processing failed: %w", err)
		}
		req.Content = templ.Content

		if req.Subject == "" {
			req.Subject = templ.Subject
		}
	}

	notification, err := s.notificationService.CreateNotification(ctx, *req)

	if err != nil {
		if req.IdempotencyKey != "" {
			if markErr := s.idempotencyStore.MarkFailed(ctx, req.IdempotencyKey, err); markErr != nil {
				span.RecordError(markErr)
			}
		}
		span.RecordError(err)

		return nil, false, fmt.Errorf("notification creation failed: %w", err)
	}

	span.SetAttributes(attribute.String("notification.id", notification.ID))

	return notification, false, nil
}

func (s *sendNotificationService) processNotificationBulk(ctx context.Context, req *[]models.CreateNotificationRequest, notificationType models.NotificationType) (*[]models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "send_service.process_notification_bulk")
	defer span.End()

	if len(*req) == 0 {
		return &[]models.Notification{}, nil
	}

	firstReq := (*req)[0]
	if firstReq.IdempotencyKey != "" {
		bulkRequestHash := s.generateBulkRequestHash(req, notificationType)
		sharedIdempKey := s.idempotencyStore.GetSharedBulkIdempotencyKey(firstReq.IdempotencyKey)
		record, err := s.idempotencyStore.CheckAndCreate(ctx, sharedIdempKey, bulkRequestHash)

		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("bulk idempotency check failed: %w", err)
		}

		if record.Status == idempotency.StatusCompleted {
			span.SetAttributes(attribute.Bool("bulk.duplicate", true))
			return &[]models.Notification{}, nil
		}
	}

	processedReqs := make([]models.CreateNotificationRequest, len(*req))

	for i, singleReq := range *req {
		r := singleReq
		r.Type = notificationType

		// DEBUG: Log what we're about to process
		log.Printf("[DEBUG processNotificationBulk] Request %d: Content=%s, TemplateID=%s, VariableCount=%d",
			i, r.Content, r.TemplateID, len(r.Variables))

		if r.Content == "" && r.TemplateID != "" {
			log.Printf("[DEBUG processNotificationBulk] Processing template for request %d with variables: %+v", i, r.Variables)
			templ, err := s.processTemplate(ctx, &r)
			if err != nil {
				span.RecordError(err)
				if firstReq.IdempotencyKey != "" {
					if markErr := s.idempotencyStore.MarkFailed(ctx, firstReq.IdempotencyKey, err); markErr != nil {
						span.RecordError(markErr)
					}
				}
				return nil, fmt.Errorf("template processing failed: %w", err)
			}
			r.Content = templ.Content
			if r.Subject == "" {
				r.Subject = templ.Subject
			}
			log.Printf("[DEBUG processNotificationBulk] Template processed for request %d, ContentLength=%d", i, len(r.Content))
		} else {
			log.Printf("[DEBUG processNotificationBulk] SKIPPING template processing for request %d (Content=%d bytes, TemplateID=%s)",
				i, len(r.Content), r.TemplateID)
		}

		processedReqs[i] = r
	}

	notifications, err := s.notificationService.CreateManyNotifications(ctx, processedReqs)
	if err != nil {
		span.RecordError(err)
		if firstReq.IdempotencyKey != "" {
			if markErr := s.idempotencyStore.MarkFailed(ctx, firstReq.IdempotencyKey, err); markErr != nil {
				span.RecordError(markErr)
			}
		}
		return nil, fmt.Errorf("bulk notification creation failed: %w", err)
	}

	if firstReq.IdempotencyKey != "" {
		if err := s.idempotencyStore.MarkCompleted(ctx, firstReq.IdempotencyKey, len(notifications)); err != nil {
			span.RecordError(err)
		}
	}

	result := make([]models.Notification, len(notifications))
	for i, n := range notifications {
		result[i] = *n
	}

	span.SetAttributes(
		attribute.Int("bulk.processed_count", len(result)),
		attribute.Bool("bulk.idempotent", firstReq.IdempotencyKey != ""),
	)

	return &result, nil
}

// sendBulkEmailsDirectly sends bulk emails using goroutines with concurrency control
func (s *sendNotificationService) sendBulkEmailsDirectly(ctx context.Context, notifications *[]models.Notification) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "send_service.bulk_email.direct_send")
	defer span.End()

	span.SetAttributes(attribute.Int("bulk.count", len(*notifications)))

	// Concurrency control - limit concurrent email sends
	concurrencyLimit := min(len(*notifications), 50) // Lower limit for email providers
	sem := make(chan struct{}, concurrencyLimit)
	wg := sync.WaitGroup{}

	successCount := 0
	failureCount := 0
	var mu sync.Mutex

	for _, notification := range *notifications {
		wg.Add(1)
		go func(notification models.Notification) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			ctx, span := trace.SpanFromContext(ctx).TracerProvider().
				Tracer("service-notification").Start(ctx, "send_service.bulk_email.send_single")
			defer span.End()

			span.SetAttributes(
				attribute.String("notification.id", notification.ID),
				attribute.String("notification.type", string(notification.Type)),
			)

			// Send email directly
			if len(notification.Recipient) == 0 {
				span.RecordError(fmt.Errorf("no recipients found"))
				span.SetStatus(codes.Error, "no recipients found")
				mu.Lock()
				failureCount++
				mu.Unlock()
				return
			}

			emailReq := &providers.EmailRequest{
				To:          notification.Recipient[0].Address,
				Subject:     notification.Subject,
				HTMLContent: notification.Content,
				Priority:    providers.PriorityNormal,
			}

			// Add CC and BCC if present
			if len(notification.CC) > 0 {
				ccAddresses := make([]string, len(notification.CC))
				for i, cc := range notification.CC {
					ccAddresses[i] = cc.Address
				}
				emailReq.CC = ccAddresses
			}
			if len(notification.BCC) > 0 {
				bccAddresses := make([]string, len(notification.BCC))
				for i, bcc := range notification.BCC {
					bccAddresses[i] = bcc.Address
				}
				emailReq.BCC = bccAddresses
			}

			response, err := s.providerManager.SendEmail(ctx, emailReq)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to send email")
				s.metrics.RecordNotificationFailed("email", "unknown", err.Error())

				// Mark notification as failed
				if markErr := s.notificationService.MarkAsFailed(ctx, notification.ID, err.Error()); markErr != nil {
					span.RecordError(markErr)
				}

				mu.Lock()
				failureCount++
				mu.Unlock()
				return
			}

			// Mark notification as sent
			if err := s.notificationService.MarkAsSent(ctx, notification.ID, response.Provider, response.MessageID, response); err != nil {
				span.RecordError(err)
				span.SetAttributes(attribute.String("mark_as_sent.error", err.Error()))
			}

			span.SetAttributes(
				attribute.String("provider.message_id", response.MessageID),
				attribute.String("provider.status", string(response.Status)),
				attribute.String("provider.name", response.Provider),
			)
			s.metrics.RecordNotificationSent("email", response.Provider)

			mu.Lock()
			successCount++
			mu.Unlock()
		}(notification)
	}

	wg.Wait()

	span.SetAttributes(
		attribute.Int("bulk.success_count", successCount),
		attribute.Int("bulk.failure_count", failureCount),
		attribute.Bool("bulk.processing_complete", true),
	)
}

// sendBulkSMSDirectly sends bulk SMS messages using goroutines with concurrency control
func (s *sendNotificationService) sendBulkSMSDirectly(ctx context.Context, notifications *[]models.Notification) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "send_service.bulk_sms.direct_send")
	defer span.End()

	span.SetAttributes(attribute.Int("bulk.count", len(*notifications)))

	// Concurrency control - limit concurrent SMS sends
	concurrencyLimit := min(len(*notifications), 100) // Higher limit for SMS providers
	sem := make(chan struct{}, concurrencyLimit)
	wg := sync.WaitGroup{}

	successCount := 0
	failureCount := 0
	var mu sync.Mutex

	for _, notification := range *notifications {
		wg.Add(1)
		go func(notification models.Notification) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			ctx, span := trace.SpanFromContext(ctx).TracerProvider().
				Tracer("service-notification").Start(ctx, "send_service.bulk_sms.send_single")
			defer span.End()

			span.SetAttributes(
				attribute.String("notification.id", notification.ID),
				attribute.String("notification.type", string(notification.Type)),
			)

			// Send SMS directly
			if len(notification.Recipient) == 0 {
				span.RecordError(fmt.Errorf("no recipients found"))
				span.SetStatus(codes.Error, "no recipients found")
				mu.Lock()
				failureCount++
				mu.Unlock()
				return
			}

			smsReq := &providers.SMSRequest{
				To:       notification.Recipient[0].Address,
				Content:  notification.Content,
				Priority: providers.PriorityNormal,
			}

			response, err := s.providerManager.SendSMS(ctx, smsReq)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to send SMS")
				s.metrics.RecordNotificationFailed("sms", "unknown", err.Error())

				// Mark notification as failed
				if markErr := s.notificationService.MarkAsFailed(ctx, notification.ID, err.Error()); markErr != nil {
					span.RecordError(markErr)
				}

				mu.Lock()
				failureCount++
				mu.Unlock()
				return
			}

			// Mark notification as sent
			if err := s.notificationService.MarkAsSent(ctx, notification.ID, response.Provider, response.MessageID, response); err != nil {
				span.RecordError(err)
				span.SetAttributes(attribute.String("mark_as_sent.error", err.Error()))
			}

			span.SetAttributes(
				attribute.String("provider.message_id", response.MessageID),
				attribute.String("provider.status", string(response.Status)),
				attribute.String("provider.name", response.Provider),
			)
			s.metrics.RecordNotificationSent("sms", response.Provider)

			mu.Lock()
			successCount++
			mu.Unlock()
		}(notification)
	}

	wg.Wait()

	span.SetAttributes(
		attribute.Int("bulk.success_count", successCount),
		attribute.Int("bulk.failure_count", failureCount),
		attribute.Bool("bulk.processing_complete", true),
	)
}

// markBulkPushAsFailed marks all push notifications as failed since push is not implemented
func (s *sendNotificationService) markBulkPushAsFailed(ctx context.Context, notifications *[]models.Notification) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "send_service.bulk_push.mark_failed")
	defer span.End()

	span.SetAttributes(attribute.Int("bulk.count", len(*notifications)))

	for _, notification := range *notifications {
		if err := s.notificationService.MarkAsFailed(ctx, notification.ID, "push notifications not yet implemented"); err != nil {
			span.RecordError(err)
		}
		s.metrics.RecordNotificationFailed("push", "not_implemented", "push notifications not yet implemented")
	}

	span.SetAttributes(attribute.Bool("bulk.processing_complete", true))
}

// enqueueForLater queues a rate-limited notification for later processing
// This is ONLY called when rate limit is exceeded (not for all notifications)
func (s *sendNotificationService) enqueueForLater(ctx context.Context, notification *models.Notification) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "send_service.enqueue_for_later")
	defer span.End()

	span.SetAttributes(
		attribute.String("notification.id", notification.ID),
		attribute.String("notification.type", string(notification.Type)),
		attribute.String("reason", "rate_limit_exceeded"),
	)

	// Build payload from notification fields
	payload := map[string]any{
		"id":              notification.ID,
		"idempotency_key": notification.IdempotencyKey,
		"type":            string(notification.Type),
		"subject":         notification.Subject,
		"content":         notification.Content,
		"provider":        notification.Provider,
		"template_id":     notification.TemplateID,
	}

	// Add optional fields
	if notification.ScheduledFor != nil {
		payload["scheduled_for"] = notification.ScheduledFor
	}
	if notification.Variables != nil {
		payload["variables"] = notification.Variables
	}

	// Convert recipients
	recipients := make([]map[string]string, 0, len(notification.Recipient))
	for _, r := range notification.Recipient {
		recipients = append(recipients, map[string]string{
			"address": r.Address,
			"type":    string(r.Type),
		})
	}
	payload["recipients"] = recipients

	if len(notification.CC) > 0 {
		cc := make([]map[string]string, 0, len(notification.CC))
		for _, c := range notification.CC {
			cc = append(cc, map[string]string{
				"address": c.Address,
				"type":    string(c.Type),
			})
		}
		payload["cc"] = cc
	}

	if len(notification.BCC) > 0 {
		bcc := make([]map[string]string, 0, len(notification.BCC))
		for _, b := range notification.BCC {
			bcc = append(bcc, map[string]string{
				"address": b.Address,
				"type":    string(b.Type),
			})
		}
		payload["bcc"] = bcc
	}

	// Build queue item
	queueItem := &queue.QueueItem{
		ID:           notification.ID,
		Type:         string(notification.Type),
		Channel:      string(notification.Type), // email, sms, push
		Priority:     2,                         // Normal priority
		Payload:      payload,
		RetryCount:   0,
		MaxRetries:   3,
		CreatedAt:    notification.CreatedAt,
		ScheduledFor: notification.ScheduledFor,
	}

	// Enqueue to Redis
	if err := s.queueManager.Enqueue(ctx, queueItem); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to enqueue notification")
		return fmt.Errorf("failed to enqueue notification: %w", err)
	}

	span.SetAttributes(attribute.Bool("enqueue.success", true))
	return nil
}

func (s *sendNotificationService) processTemplate(ctx context.Context, req *models.CreateNotificationRequest) (templates.Template, error) {
	_, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "send_service.process_template")
	defer span.End()

	var result templates.Template

	span.SetAttributes(
		attribute.String("template.id", req.TemplateID),
		attribute.String("notification.type", string(req.Type)),
	)

	templateName := templates.TemplateName(req.TemplateID)
	result, err := templates.ProcessTemplate(req.Type, templateName, req.Variables)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "template processing failed")
		return result, fmt.Errorf("failed to process template %s: %w", req.TemplateID, err)
	}

	span.SetAttributes(attribute.Int("template.subject", len(result.Subject)))
	span.SetAttributes(attribute.Int("template.content.length", len(result.Content)))

	return result, nil
}

func (s *sendNotificationService) generateRequestHash(req *models.CreateNotificationRequest) string {
	data := fmt.Sprintf("%s-%s-%v-%s-%v",
		req.Type, req.Subject, req.Recipients, req.Content, req.Variables)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

func (s *sendNotificationService) generateBulkRequestHash(req *[]models.CreateNotificationRequest, notificationType models.NotificationType) string {
	var data string
	for i, r := range *req {
		data += fmt.Sprintf("%d-%s-%s-%v-%s-%v;",
			i, notificationType, r.Subject, r.Recipients, r.Content, r.Variables)
	}
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

//nolint:unused // Reserved for future use
func getEventType(notificationType models.NotificationType) string {
	switch notificationType {
	case models.NotificationTypeEmail:
		return "notification.email.request"
	case models.NotificationTypeSMS:
		return "notification.sms.request"
	case models.NotificationTypePush:
		return "notification.push.request"
	default:
		return "notification.unknown.request"
	}
}

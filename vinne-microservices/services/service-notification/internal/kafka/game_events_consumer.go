package kafka

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/randco/randco-microservices/services/service-notification/internal/clients"
	"github.com/randco/randco-microservices/services/service-notification/internal/config"
	"github.com/randco/randco-microservices/services/service-notification/internal/models"
	"github.com/randco/randco-microservices/services/service-notification/internal/repositories"
	"github.com/randco/randco-microservices/services/service-notification/internal/services"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/randco-microservices/shared/events"
	"github.com/randco/randco-microservices/shared/idempotency"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	gameEventsTopic = "game.events"
)

// GameEventConsumer handles game-related events from Kafka
type GameEventConsumer struct {
	config              ConsumerConfig
	eventBus            events.EventBus
	sendNotificationSvc services.SendNotificationService
	pushNotificationSvc services.PushNotificationService
	deviceTokenRepo     repositories.DeviceTokenRepository
	logger              logger.Logger
	tracer              trace.Tracer
	adminClient         clients.AdminClient
	idempotencyStore    idempotency.IdempotencyStore

	ctx    context.Context
	cancel context.CancelFunc

	// Fallback configuration for notification recipients (used if admin client fails)
	fallbackRecipients []string
}

func NewGameEventConsumer(
	cfg config.KafkaConfig,
	eventBus events.EventBus,
	sendNotificationSvc services.SendNotificationService,
	pushNotificationSvc services.PushNotificationService,
	deviceTokenRepo repositories.DeviceTokenRepository,
	logger logger.Logger,
	adminClient clients.AdminClient,
	idempotencyStore idempotency.IdempotencyStore,
	fallbackRecipients []string,
) *GameEventConsumer {
	return &GameEventConsumer{
		config: ConsumerConfig{
			Brokers:           cfg.Brokers,
			GroupID:           "notification-service-game-events",
			SessionTimeout:    30 * time.Second,
			HeartbeatInterval: 10 * time.Second,
			MaxRetries:        3,
			RetryDelay:        5 * time.Second,
		},
		eventBus:            eventBus,
		sendNotificationSvc: sendNotificationSvc,
		pushNotificationSvc: pushNotificationSvc,
		deviceTokenRepo:     deviceTokenRepo,
		logger:              logger,
		tracer:              otel.Tracer(instrumentationName),
		adminClient:         adminClient,
		idempotencyStore:    idempotencyStore,
		fallbackRecipients:  fallbackRecipients,
	}
}

func (c *GameEventConsumer) Start(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)

	c.logger.Info("Starting Game Event consumer",
		"brokers", c.config.Brokers,
		"group_id", c.config.GroupID,
		"topic", gameEventsTopic,
		"fallback_recipients", c.fallbackRecipients,
	)

	// Subscribe to game events topic
	if err := c.subscribeToGameEvents(); err != nil {
		c.logger.Error("Failed to subscribe to game events topic",
			"topic", gameEventsTopic,
			"error", err,
		)
		return fmt.Errorf("failed to subscribe to topic %s: %w", gameEventsTopic, err)
	}

	c.logger.Info("Game Event consumer started successfully")
	return nil
}

func (c *GameEventConsumer) Stop(ctx context.Context) error {
	c.logger.Info("Stopping Game Event consumer")

	if c.cancel != nil {
		c.cancel()
	}

	c.logger.Info("Game Event consumer stopped")
	return nil
}

// getNotificationRecipients fetches admin emails dynamically from admin management service
// Falls back to configured recipients if admin client is unavailable
func (c *GameEventConsumer) getNotificationRecipients(ctx context.Context) []string {
	ctx, span := c.tracer.Start(ctx, "kafka.get_notification_recipients")
	defer span.End()

	// Try to fetch from admin management service
	if c.adminClient != nil {
		emails, err := c.adminClient.GetActiveAdminEmails(ctx)
		if err != nil {
			c.logger.Warn("Failed to fetch admin emails from admin management service, using fallback",
				"error", err,
				"fallback_count", len(c.fallbackRecipients),
			)
			span.RecordError(err)
			span.SetAttributes(
				attribute.Bool("used_fallback", true),
				attribute.Int("fallback_count", len(c.fallbackRecipients)),
			)
			return c.fallbackRecipients
		}

		if len(emails) == 0 {
			c.logger.Warn("No active admin emails found, using fallback",
				"fallback_count", len(c.fallbackRecipients),
			)
			span.SetAttributes(
				attribute.Bool("used_fallback", true),
				attribute.String("reason", "no_active_admins"),
			)
			return c.fallbackRecipients
		}

		c.logger.Info("Fetched admin emails dynamically",
			"count", len(emails),
		)
		span.SetAttributes(
			attribute.Bool("used_fallback", false),
			attribute.Int("admin_count", len(emails)),
		)
		return emails
	}

	// Admin client not available, use fallback
	c.logger.Warn("Admin client not configured, using fallback recipients",
		"fallback_count", len(c.fallbackRecipients),
	)
	span.SetAttributes(
		attribute.Bool("used_fallback", true),
		attribute.String("reason", "no_admin_client"),
	)
	return c.fallbackRecipients
}

func (c *GameEventConsumer) subscribeToGameEvents() error {
	ctx, span := c.tracer.Start(c.ctx, "kafka.subscribe_game_events",
		trace.WithAttributes(
			attribute.String("kafka.topic", gameEventsTopic),
			attribute.String("kafka.group_id", c.config.GroupID),
		),
	)
	defer span.End()

	handler := func(ctx context.Context, event *events.EventEnvelope) error {
		return c.handleGameEvent(ctx, event)
	}

	if err := c.eventBus.Subscribe(ctx, gameEventsTopic, handler); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to subscribe to topic")
		return fmt.Errorf("failed to subscribe to topic %s: %w", gameEventsTopic, err)
	}

	span.SetStatus(codes.Ok, "successfully subscribed to topic")
	c.logger.Info("Subscribed to game events topic", "topic", gameEventsTopic)
	return nil
}

func (c *GameEventConsumer) handleGameEvent(ctx context.Context, event *events.EventEnvelope) error {
	ctx, span := c.tracer.Start(ctx, "kafka.handle_game_event",
		trace.WithAttributes(
			attribute.String("kafka.topic", event.Topic),
			attribute.String("event.id", event.Key),
			attribute.String("event.type", event.Headers["event_type"]),
		),
	)
	defer span.End()

	start := time.Now()

	// Route event to appropriate handler based on event type
	eventType := event.Headers["event_type"]
	span.SetAttributes(attribute.String("game_event.type", eventType))

	// Enhanced logging: log full event envelope details
	c.logger.Info("Received Kafka game event",
		"event_type", eventType,
		"event_id", event.Key,
		"topic", event.Topic,
		"event_timestamp", event.Timestamp,
		"payload_size_bytes", len(event.Payload),
		"headers", event.Headers,
	)

	var err error
	switch events.EventType(eventType) {
	case events.DrawExecuted:
		err = c.handleDrawExecutedEvent(ctx, event)
	case events.SalesCutoffReached:
		err = c.handleSalesCutoffReachedEvent(ctx, event)
	default:
		// Ignore other game events
		c.logger.Debug("Ignoring game event type",
			"event_type", eventType,
			"event_id", event.Key,
		)
		return nil
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to handle game event")
		c.logger.Error("Failed to handle game event",
			"event_type", eventType,
			"event_id", event.Key,
			"error", err,
		)
		return err
	}

	duration := time.Since(start)
	span.SetAttributes(attribute.Int64("processing.duration_ms", duration.Milliseconds()))
	span.SetStatus(codes.Ok, "game event processed successfully")

	c.logger.Info("Successfully processed game event",
		"event_type", eventType,
		"event_id", event.Key,
		"duration_ms", duration.Milliseconds(),
	)

	return nil
}

func (c *GameEventConsumer) handleDrawExecutedEvent(ctx context.Context, event *events.EventEnvelope) error {
	ctx, span := c.tracer.Start(ctx, "kafka.handle_draw_executed",
		trace.WithAttributes(
			attribute.String("event.id", event.Key),
		),
	)
	defer span.End()

	parseStart := time.Now()

	// Parse draw executed event
	var drawEvent events.GameDrawExecutedEvent
	if err := json.Unmarshal(event.Payload, &drawEvent); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to parse draw executed event")
		c.logger.Error("Failed to unmarshal draw executed event",
			"event_id", event.Key,
			"payload_size_bytes", len(event.Payload),
			"parse_duration_ms", time.Since(parseStart).Milliseconds(),
			"error", err,
		)
		return fmt.Errorf("failed to parse draw executed event: %w", err)
	}

	parseDuration := time.Since(parseStart)
	c.logger.Info("Parsed draw executed event",
		"event_id", event.Key,
		"draw_id", drawEvent.DrawID,
		"parse_duration_ms", parseDuration.Milliseconds(),
		"payload_size_bytes", len(event.Payload),
	)

	span.SetAttributes(
		attribute.String("draw.id", drawEvent.DrawID),
		attribute.String("game.id", drawEvent.GameID),
		attribute.String("game.name", drawEvent.GameName),
		attribute.String("schedule.id", drawEvent.ScheduleID),
	)

	c.logger.Info("Processing draw executed event",
		"draw_id", drawEvent.DrawID,
		"game_id", drawEvent.GameID,
		"game_name", drawEvent.GameName,
		"game_code", drawEvent.GameCode,
		"schedule_id", drawEvent.ScheduleID,
	)

	// Event-level idempotency check to prevent duplicate processing
	idempotencyStart := time.Now()
	eventIdempotencyKey := fmt.Sprintf("event-processed-draw-%s-%s", drawEvent.EventID, drawEvent.ScheduleID)
	eventHash := c.generateEventHash(&drawEvent)

	c.logger.Info("Checking event idempotency",
		"event_id", drawEvent.EventID,
		"idempotency_key", eventIdempotencyKey,
		"event_hash", eventHash[:16], // Log first 16 chars of hash
	)

	record, err := c.idempotencyStore.CheckAndCreate(ctx, eventIdempotencyKey, eventHash)
	idempotencyDuration := time.Since(idempotencyStart)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "idempotency check failed")
		c.logger.Error("Idempotency check failed for draw event",
			"event_id", drawEvent.EventID,
			"idempotency_key", eventIdempotencyKey,
			"check_duration_ms", idempotencyDuration.Milliseconds(),
			"error", err,
		)
		return fmt.Errorf("idempotency check failed for draw event: %w", err)
	}

	if record.Status == idempotency.StatusCompleted {
		c.logger.Info("Draw event already processed, skipping duplicate",
			"event_id", drawEvent.EventID,
			"schedule_id", drawEvent.ScheduleID,
			"draw_id", drawEvent.DrawID,
			"idempotency_key", eventIdempotencyKey,
			"check_duration_ms", idempotencyDuration.Milliseconds(),
			"record_status", record.Status,
		)
		span.SetAttributes(
			attribute.Bool("event.duplicate", true),
			attribute.String("idempotency.key", eventIdempotencyKey),
		)
		return nil
	}

	c.logger.Info("Event idempotency check passed - processing new event",
		"event_id", drawEvent.EventID,
		"idempotency_key", eventIdempotencyKey,
		"check_duration_ms", idempotencyDuration.Milliseconds(),
		"record_status", record.Status,
	)

	// Get notification recipients dynamically
	recipients := c.getNotificationRecipients(ctx)
	c.logger.Info("Sending draw executed notifications",
		"recipient_count", len(recipients),
		"event_id", drawEvent.EventID,
	)

	// Send email notification to each recipient
	successCount := 0
	for _, recipient := range recipients {
		if err := c.sendGameEndNotification(ctx, recipient, &drawEvent); err != nil {
			c.logger.Error("Failed to send game end notification",
				"recipient", recipient,
				"draw_id", drawEvent.DrawID,
				"error", err,
			)
			// Continue to next recipient even if one fails
			continue
		}

		successCount++
		c.logger.Info("Game end notification queued",
			"recipient", recipient,
			"draw_id", drawEvent.DrawID,
			"game_name", drawEvent.GameName,
		)
	}

	// Mark event as completed in idempotency store
	if err := c.idempotencyStore.MarkCompleted(ctx, eventIdempotencyKey, map[string]any{
		"recipients_processed": len(recipients),
		"notifications_sent":   successCount,
		"event_id":             drawEvent.EventID,
		"schedule_id":          drawEvent.ScheduleID,
		"draw_id":              drawEvent.DrawID,
	}); err != nil {
		c.logger.Error("Failed to mark event as completed in idempotency store",
			"event_id", drawEvent.EventID,
			"error", err,
		)
		span.RecordError(err)
		// Don't fail the entire operation, just log the error
	}

	span.SetAttributes(
		attribute.Int("recipients.total", len(recipients)),
		attribute.Int("notifications.success", successCount),
	)

	return nil
}

func (c *GameEventConsumer) sendGameEndNotification(ctx context.Context, recipient string, drawEvent *events.GameDrawExecutedEvent) error {
	ctx, span := c.tracer.Start(ctx, "kafka.send_game_end_notification",
		trace.WithAttributes(
			attribute.String("recipient", recipient),
			attribute.String("draw.id", drawEvent.DrawID),
		),
	)
	defer span.End()

	// Format timestamps for email
	scheduledDrawTime := drawEvent.ScheduledDrawTime.Format("3:04 PM, Jan 2, 2006")
	actualDrawTime := drawEvent.ActualDrawTime.Format("3:04 PM, Jan 2, 2006")
	notificationTime := time.Now().Format("3:04 PM, Jan 2, 2006")
	currentYear := time.Now().Format("2006")

	// Create notification request
	idempotencyKey := fmt.Sprintf("game-end-%s-%s-%s", drawEvent.EventID, drawEvent.ScheduleID, recipient)
	sendRequest := &models.CreateNotificationRequest{
		IdempotencyKey: idempotencyKey,
		Type:           models.NotificationTypeEmail,
		Recipients: []models.CreateRecipientRequest{
			{Address: recipient},
		},
		Subject:    "Game Draw Executed - " + drawEvent.GameName,
		TemplateID: "game_end",
		Variables: map[string]string{
			"CompanyName":       "RAND Lottery",
			"GameName":          drawEvent.GameName,
			"GameCode":          drawEvent.GameCode,
			"ScheduledDrawTime": scheduledDrawTime,
			"ActualDrawTime":    actualDrawTime,
			"DrawID":            drawEvent.DrawID,
			"ScheduleID":        drawEvent.ScheduleID,
			"NotificationTime":  notificationTime,
			"CurrentYear":       currentYear,
			"CompanyAddress":    "Accra, Ghana",
		},
	}

	// Send email directly via SendNotificationService
	sendStart := time.Now()
	c.logger.Info("Sending game end notification email",
		"recipient", recipient,
		"draw_id", drawEvent.DrawID,
		"idempotency_key", idempotencyKey,
	)

	response, err := c.sendNotificationSvc.SendEmail(ctx, sendRequest)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send notification")
		c.logger.Error("Failed to send game end notification",
			"recipient", recipient,
			"draw_id", drawEvent.DrawID,
			"send_duration_ms", time.Since(sendStart).Milliseconds(),
			"error", err,
		)
		return fmt.Errorf("failed to send notification: %w", err)
	}

	sendDuration := time.Since(sendStart)
	c.logger.Info("Game end notification sent successfully",
		"recipient", recipient,
		"draw_id", drawEvent.DrawID,
		"notification_id", response.ID,
		"send_duration_ms", sendDuration.Milliseconds(),
	)

	span.SetAttributes(
		attribute.String("notification.id", response.ID),
	)
	if response.ProviderMessageID != nil {
		span.SetAttributes(
			attribute.String("provider.message_id", *response.ProviderMessageID),
		)
	}
	span.SetStatus(codes.Ok, "notification sent successfully")

	return nil
}

func (c *GameEventConsumer) handleSalesCutoffReachedEvent(ctx context.Context, event *events.EventEnvelope) error {
	ctx, span := c.tracer.Start(ctx, "kafka.handle_sales_cutoff_reached",
		trace.WithAttributes(
			attribute.String("event.id", event.Key),
		),
	)
	defer span.End()

	parseStart := time.Now()

	// Parse sales cutoff reached event
	var salesCutoffEvent events.GameSalesCutoffReachedEvent
	if err := json.Unmarshal(event.Payload, &salesCutoffEvent); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to parse sales cutoff reached event")
		c.logger.Error("Failed to unmarshal sales cutoff reached event",
			"event_id", event.Key,
			"payload_size_bytes", len(event.Payload),
			"parse_duration_ms", time.Since(parseStart).Milliseconds(),
			"error", err,
		)
		return fmt.Errorf("failed to parse sales cutoff reached event: %w", err)
	}

	parseDuration := time.Since(parseStart)
	c.logger.Info("Parsed sales cutoff reached event",
		"event_id", event.Key,
		"game_id", salesCutoffEvent.GameID,
		"parse_duration_ms", parseDuration.Milliseconds(),
		"payload_size_bytes", len(event.Payload),
	)

	span.SetAttributes(
		attribute.String("game.id", salesCutoffEvent.GameID),
		attribute.String("game.name", salesCutoffEvent.GameName),
		attribute.String("schedule.id", salesCutoffEvent.ScheduleID),
	)

	c.logger.Info("Processing sales cutoff reached event",
		"game_id", salesCutoffEvent.GameID,
		"game_name", salesCutoffEvent.GameName,
		"game_code", salesCutoffEvent.GameCode,
		"schedule_id", salesCutoffEvent.ScheduleID,
	)

	// Event-level idempotency check to prevent duplicate processing
	idempotencyStart := time.Now()
	eventIdempotencyKey := fmt.Sprintf("event-processed-cutoff-%s-%s", salesCutoffEvent.EventID, salesCutoffEvent.ScheduleID)
	eventHash := c.generateEventHash(&salesCutoffEvent)

	c.logger.Info("Checking event idempotency",
		"event_id", salesCutoffEvent.EventID,
		"idempotency_key", eventIdempotencyKey,
		"event_hash", eventHash[:16], // Log first 16 chars of hash
	)

	record, err := c.idempotencyStore.CheckAndCreate(ctx, eventIdempotencyKey, eventHash)
	idempotencyDuration := time.Since(idempotencyStart)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "idempotency check failed")
		c.logger.Error("Idempotency check failed for sales cutoff event",
			"event_id", salesCutoffEvent.EventID,
			"idempotency_key", eventIdempotencyKey,
			"check_duration_ms", idempotencyDuration.Milliseconds(),
			"error", err,
		)
		return fmt.Errorf("idempotency check failed for sales cutoff event: %w", err)
	}

	if record.Status == idempotency.StatusCompleted {
		c.logger.Info("Sales cutoff event already processed, skipping duplicate",
			"event_id", salesCutoffEvent.EventID,
			"schedule_id", salesCutoffEvent.ScheduleID,
			"game_id", salesCutoffEvent.GameID,
			"idempotency_key", eventIdempotencyKey,
			"check_duration_ms", idempotencyDuration.Milliseconds(),
			"record_status", record.Status,
		)
		span.SetAttributes(
			attribute.Bool("event.duplicate", true),
			attribute.String("idempotency.key", eventIdempotencyKey),
		)
		return nil
	}

	c.logger.Info("Event idempotency check passed - processing new event",
		"event_id", salesCutoffEvent.EventID,
		"idempotency_key", eventIdempotencyKey,
		"check_duration_ms", idempotencyDuration.Milliseconds(),
		"record_status", record.Status,
	)

	// Check if push notification service is available
	if c.pushNotificationSvc == nil {
		c.logger.Warn("Push notification service not available, skipping retailer notifications",
			"event_id", salesCutoffEvent.EventID,
			"game_id", salesCutoffEvent.GameID,
		)
		span.SetAttributes(
			attribute.Bool("push.skipped", true),
			attribute.String("skip_reason", "service_not_configured"),
		)
		// Mark event as completed even though we skipped it
		if err := c.idempotencyStore.MarkCompleted(ctx, eventIdempotencyKey, map[string]any{
			"retailers_processed": 0,
			"notifications_sent":  0,
			"event_id":            salesCutoffEvent.EventID,
			"schedule_id":         salesCutoffEvent.ScheduleID,
			"game_id":             salesCutoffEvent.GameID,
			"skipped":             true,
			"skip_reason":         "push_service_not_configured",
		}); err != nil {
			c.logger.Error("Failed to mark event as completed", "error", err)
		}
		return nil
	}

	// Get all retailers with active device tokens
	retailerIDs, err := c.deviceTokenRepo.GetAllActiveRetailerIDs(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get active retailer IDs")
		c.logger.Error("Failed to get active retailer IDs",
			"event_id", salesCutoffEvent.EventID,
			"error", err,
		)
		return fmt.Errorf("failed to get active retailer IDs: %w", err)
	}

	if len(retailerIDs) == 0 {
		c.logger.Info("No active retailers found with device tokens, skipping push notifications",
			"event_id", salesCutoffEvent.EventID,
		)
		span.SetAttributes(attribute.Int("retailers.total", 0))
		// Mark event as completed
		if err := c.idempotencyStore.MarkCompleted(ctx, eventIdempotencyKey, map[string]any{
			"retailers_processed": 0,
			"notifications_sent":  0,
			"event_id":            salesCutoffEvent.EventID,
			"schedule_id":         salesCutoffEvent.ScheduleID,
			"game_id":             salesCutoffEvent.GameID,
		}); err != nil {
			c.logger.Error("Failed to mark event as completed", "error", err)
		}
		return nil
	}

	c.logger.Info("Sending sales cutoff push notifications to all retailers",
		"retailer_count", len(retailerIDs),
		"event_id", salesCutoffEvent.EventID,
	)

	// Format timestamps
	scheduledEndTime := salesCutoffEvent.ScheduledEndTime.Format("3:04 PM")
	nextDrawTime := salesCutoffEvent.NextDrawTime.Format("3:04 PM, Jan 2")

	// Send push notification to each retailer
	successCount := 0
	for _, retailerID := range retailerIDs {
		if err := c.sendSalesCutoffPushNotification(ctx, retailerID, &salesCutoffEvent, scheduledEndTime, nextDrawTime); err != nil {
			c.logger.Error("Failed to send sales cutoff push notification",
				"retailer_id", retailerID,
				"game_id", salesCutoffEvent.GameID,
				"error", err,
			)
			// Continue to next retailer even if one fails
			continue
		}

		successCount++
		c.logger.Info("Sales cutoff push notification sent",
			"retailer_id", retailerID,
			"game_id", salesCutoffEvent.GameID,
			"game_name", salesCutoffEvent.GameName,
		)
	}

	// Mark event as completed in idempotency store
	if err := c.idempotencyStore.MarkCompleted(ctx, eventIdempotencyKey, map[string]any{
		"retailers_processed": len(retailerIDs),
		"notifications_sent":  successCount,
		"event_id":            salesCutoffEvent.EventID,
		"schedule_id":         salesCutoffEvent.ScheduleID,
		"game_id":             salesCutoffEvent.GameID,
	}); err != nil {
		c.logger.Error("Failed to mark event as completed in idempotency store",
			"event_id", salesCutoffEvent.EventID,
			"error", err,
		)
		span.RecordError(err)
		// Don't fail the entire operation, just log the error
	}

	span.SetAttributes(
		attribute.Int("retailers.total", len(retailerIDs)),
		attribute.Int("notifications.success", successCount),
	)
	span.SetStatus(codes.Ok, "push notifications sent to retailers")

	return nil
}

func (c *GameEventConsumer) sendSalesCutoffPushNotification(ctx context.Context, retailerID string, salesCutoffEvent *events.GameSalesCutoffReachedEvent, scheduledEndTime, nextDrawTime string) error {
	ctx, span := c.tracer.Start(ctx, "kafka.send_sales_cutoff_push_notification",
		trace.WithAttributes(
			attribute.String("retailer.id", retailerID),
			attribute.String("game.id", salesCutoffEvent.GameID),
		),
	)
	defer span.End()

	// Create push notification with history record
	notificationReq := &models.CreateRetailerNotificationRequest{
		RetailerID:     retailerID,
		Type:           "general", // sales cutoff is a general notification type
		Title:          fmt.Sprintf("Sales Closed: %s", salesCutoffEvent.GameName),
		Body:           fmt.Sprintf("Sales for %s have ended at %s. Next draw: %s", salesCutoffEvent.GameName, scheduledEndTime, nextDrawTime),
		Amount:         nil,                          // No amount for sales cutoff notifications
		TransactionID:  nil,                          // No transaction for sales cutoff
		NotificationID: &salesCutoffEvent.ScheduleID, // Link to schedule ID
	}

	// Send push notification with history record
	sendStart := time.Now()
	c.logger.Info("Sending sales cutoff push notification with history",
		"retailer_id", retailerID,
		"game_id", salesCutoffEvent.GameID,
		"game_name", salesCutoffEvent.GameName,
	)

	if err := c.pushNotificationSvc.SendPushNotificationWithHistory(ctx, notificationReq); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send push notification")
		c.logger.Error("Failed to send sales cutoff push notification",
			"retailer_id", retailerID,
			"game_id", salesCutoffEvent.GameID,
			"send_duration_ms", time.Since(sendStart).Milliseconds(),
			"error", err,
		)
		return fmt.Errorf("failed to send push notification: %w", err)
	}

	sendDuration := time.Since(sendStart)
	c.logger.Info("Sales cutoff push notification sent successfully",
		"retailer_id", retailerID,
		"game_id", salesCutoffEvent.GameID,
		"game_name", salesCutoffEvent.GameName,
		"send_duration_ms", sendDuration.Milliseconds(),
	)

	span.SetStatus(codes.Ok, "push notification sent successfully")

	return nil
}

// generateEventHash creates a deterministic hash from event content
// Used for idempotency checks to detect duplicate events based on content
func (c *GameEventConsumer) generateEventHash(event interface{}) string {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		c.logger.Warn("Failed to marshal event for hashing, using empty hash", "error", err)
		return ""
	}

	hash := sha256.Sum256(eventJSON)
	return fmt.Sprintf("%x", hash)
}

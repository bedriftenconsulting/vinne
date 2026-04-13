package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/randco/randco-microservices/services/service-notification/internal/config"
	"github.com/randco/randco-microservices/services/service-notification/internal/models"
	"github.com/randco/randco-microservices/services/service-notification/internal/queue"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/randco-microservices/shared/events"
	"github.com/randco/randco-microservices/shared/idempotency"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	instrumentationName                    = "github.com/randco/randco-microservices/services/service-notification/kafka"
	notificationTopic                      = "notification.requests"
	NotificationAlreadyBeingProcessedError = "notification already being processed"
	NotificationAlreadyProcessedError      = "notification already processed"
)

type ConsumerConfig struct {
	Brokers           []string
	GroupID           string
	SessionTimeout    time.Duration
	HeartbeatInterval time.Duration
	MaxRetries        int
	RetryDelay        time.Duration
	MaxEventAge       time.Duration // Maximum age of events to process (prevents processing old events)
}

type KafkaConsumer struct {
	config           ConsumerConfig
	eventBus         events.EventBus
	queue            queue.QueueManager
	idempotencyStore idempotency.IdempotencyStore
	logger           logger.Logger
	tracer           trace.Tracer

	ctx    context.Context
	cancel context.CancelFunc
}

func NewKafkaConsumer(
	cfg config.KafkaConfig,
	eventBus events.EventBus,
	queue queue.QueueManager,
	idempotencyStore idempotency.IdempotencyStore,
	logger logger.Logger,
) *KafkaConsumer {
	return &KafkaConsumer{
		config: ConsumerConfig{
			Brokers:           cfg.Brokers,
			GroupID:           "notification-service",
			SessionTimeout:    30 * time.Second,
			HeartbeatInterval: 10 * time.Second,
			MaxRetries:        3,
			RetryDelay:        5 * time.Second,
			MaxEventAge:       5 * time.Minute, // Skip events older than 5 minutes
		},
		eventBus:         eventBus,
		queue:            queue,
		idempotencyStore: idempotencyStore,
		logger:           logger,
		tracer:           otel.Tracer(instrumentationName),
	}
}

func (c *KafkaConsumer) Start(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)

	c.logger.Info("Starting Kafka consumer",
		"brokers", c.config.Brokers,
		"group_id", c.config.GroupID,
		"topic", notificationTopic,
	)

	// Subscribe to notification requests topic
	if err := c.subscribeToTopic(notificationTopic); err != nil {
		c.logger.Error("Failed to subscribe to notification topic",
			"topic", notificationTopic,
			"error", err,
		)
		return fmt.Errorf("failed to subscribe to topic %s: %w", notificationTopic, err)
	}

	c.logger.Info("Kafka consumer started successfully")
	return nil
}

func (c *KafkaConsumer) Stop(ctx context.Context) error {
	c.logger.Info("Stopping Kafka consumer")

	if c.cancel != nil {
		c.cancel()
	}

	if c.eventBus != nil {
		if err := c.eventBus.Close(); err != nil {
			c.logger.Error("Error closing event bus", "error", err)
		}
	}

	c.logger.Info("Kafka consumer stopped")
	return nil
}

func (c *KafkaConsumer) subscribeToTopic(topic string) error {
	ctx, span := c.tracer.Start(c.ctx, "kafka.subscribe",
		trace.WithAttributes(
			attribute.String("kafka.topic", topic),
			attribute.String("kafka.group_id", c.config.GroupID),
		),
	)
	defer span.End()

	handler := func(ctx context.Context, event *events.EventEnvelope) error {
		return c.handleEvent(ctx, event)
	}

	if err := c.eventBus.Subscribe(ctx, topic, handler); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to subscribe to topic")
		return fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
	}

	span.SetStatus(codes.Ok, "successfully subscribed to topic")
	c.logger.Info("Subscribed to Kafka topic", "topic", topic)
	return nil
}

func (c *KafkaConsumer) handleEvent(ctx context.Context, event *events.EventEnvelope) error {
	ctx, span := c.tracer.Start(ctx, "kafka.handle_notification_request",
		trace.WithAttributes(
			attribute.String("kafka.topic", event.Topic),
			attribute.String("event.id", event.Key),
			attribute.String("event.type", event.Headers["event_type"]),
		),
	)
	defer span.End()

	start := time.Now()

	request, err := c.parseNotificationRequest(event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to parse notification request")
		c.logger.Error("Failed to parse notification request",
			"event_id", event.Key,
			"error", err,
		)
		return err
	}

	// Filter out events older than 5 minutes to prevent processing stale events
	// This helps when consumer group has old committed offsets
	if err := c.checkEventAge(request); err != nil {
		span.SetAttributes(attribute.String("skip_reason", "event_too_old"))
		span.SetStatus(codes.Ok, "skipped old event")
		c.logger.Warn("Skipping old event",
			"event_id", request.EventID,
			"event_timestamp", request.Timestamp,
			"age_seconds", time.Since(request.Timestamp).Seconds(),
		)
		return nil // Return nil to mark message as processed and move on
	}

	span.SetAttributes(
		attribute.String("notification.channel", request.Data.Channel),
		attribute.String("notification.recipient", request.Data.Recipient),
		attribute.String("notification.template_id", request.Data.TemplateID),
		attribute.String("notification.idempotency_key", request.Data.IdempotencyKey),
	)

	if err := c.checkIdempotency(ctx, request); err != nil {
		if err.Error() == NotificationAlreadyBeingProcessedError {
			return nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "idempotency check failed")
		c.logger.Error("Idempotency check failed",
			"idempotency_key", request.Data.IdempotencyKey,
			"error", err,
		)
		return err
	}

	queueItem, err := c.createQueueItem(request)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create queue item")
		c.logger.Error("Failed to create queue item",
			"request_id", request.EventID,
			"error", err,
		)
		return err
	}

	if err := c.queue.Enqueue(ctx, queueItem); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to enqueue notification")
		c.logger.Error("Failed to enqueue notification",
			"queue_item_id", queueItem.ID,
			"error", err,
		)
		return err
	}

	duration := time.Since(start)
	span.SetAttributes(
		attribute.Int64("processing.duration_ms", duration.Milliseconds()),
		attribute.String("queue.item_id", queueItem.ID),
	)
	span.SetStatus(codes.Ok, "notification request processed successfully")

	c.logger.Info("Successfully processed notification request",
		"event_id", request.EventID,
		"idempotency_key", request.Data.IdempotencyKey,
		"channel", request.Data.Channel,
		"recipient", request.Data.Recipient,
		"queue_item_id", queueItem.ID,
		"duration_ms", duration.Milliseconds(),
	)

	return nil
}

func (c *KafkaConsumer) parseNotificationRequest(event *events.EventEnvelope) (*models.NotificationRequest, error) {
	var request models.NotificationRequest
	if err := json.Unmarshal(event.Payload, &request); err != nil {
		c.logger.Error("Failed to unmarshal notification request",
			"error", err,
			"payload", string(event.Payload),
			"event_topic", event.Topic,
			"event_key", event.Key,
		)
		return nil, fmt.Errorf("failed to unmarshal notification request: %w", err)
	}

	// Validate required fields
	if request.Data.IdempotencyKey == "" {
		return nil, fmt.Errorf("idempotency_key is required")
	}
	if request.Data.Channel == "" {
		return nil, fmt.Errorf("channel is required")
	}
	if request.Data.Recipient == "" {
		return nil, fmt.Errorf("recipient is required")
	}
	if request.Data.TemplateID == "" {
		return nil, fmt.Errorf("template_id is required")
	}

	return &request, nil
}

func (c *KafkaConsumer) checkIdempotency(ctx context.Context, request *models.NotificationRequest) error {
	requestHash := fmt.Sprintf("%s:%s:%s:%s",
		request.Data.Channel,
		request.Data.Recipient,
		request.Data.TemplateID,
		request.EventID)

	record, err := c.idempotencyStore.CheckAndCreate(ctx, request.Data.IdempotencyKey, requestHash)
	if err != nil {
		return fmt.Errorf("idempotency check failed: %w", err)
	}

	if record.Status == idempotency.StatusCompleted {
		c.logger.Info("Notification request already processed",
			"idempotency_key", request.Data.IdempotencyKey,
			"status", record.Status,
		)
		return fmt.Errorf("%s", NotificationAlreadyProcessedError)
	}

	if record.Status == idempotency.StatusPending {
		c.logger.Info("Notification request already being processed",
			"idempotency_key", request.Data.IdempotencyKey,
			"status", record.Status,
		)
		return nil
	}

	return nil
}

func (c *KafkaConsumer) checkEventAge(request *models.NotificationRequest) error {
	if c.config.MaxEventAge == 0 {
		// No max age configured, allow all events
		return nil
	}

	eventAge := time.Since(request.Timestamp)
	if eventAge > c.config.MaxEventAge {
		return fmt.Errorf("event too old: age=%v, max_age=%v", eventAge, c.config.MaxEventAge)
	}

	return nil
}

func (c *KafkaConsumer) createQueueItem(request *models.NotificationRequest) (*queue.QueueItem, error) {
	priority := c.convertPriority(request.Data.Priority)

	queueItem := &queue.QueueItem{
		ID:       fmt.Sprintf("kafka-%s-%s", request.EventID, time.Now().Format("20060102150405")),
		Type:     request.Data.Channel,
		Channel:  request.Data.Channel,
		Priority: priority,
		Payload: map[string]any{
			"notification_request": request,
		},
		RetryCount: 0,
		MaxRetries: 3,
		CreatedAt:  time.Now().UTC(),
		Tags: map[string]string{
			"source":          "kafka",
			"event_id":        request.EventID,
			"correlation_id":  request.CorrelationID,
			"idempotency_key": request.Data.IdempotencyKey,
			"template_id":     request.Data.TemplateID,
			"recipient":       request.Data.Recipient,
		},
	}

	return queueItem, nil
}

func (c *KafkaConsumer) convertPriority(priority string) int {
	switch priority {
	case "low":
		return 1
	case "normal":
		return 2
	case "high":
		return 3
	case "critical":
		return 4
	default:
		return 2
	}
}

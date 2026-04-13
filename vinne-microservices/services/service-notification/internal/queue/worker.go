package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/randco/randco-microservices/services/service-notification/internal/models"
	"github.com/randco/randco-microservices/shared/common/logger"
)

type WorkerConfig struct {
	WorkerID       string
	PollInterval   time.Duration
	MaxConcurrency int
	RetryBackoff   time.Duration
	MaxBackoff     time.Duration
	BatchSize      int
	MaxItemAge     time.Duration // Maximum age of queue items to process (prevents processing old items)
}

type Worker interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	ProcessItem(ctx context.Context, item *QueueItem) (*models.Notification, error)
}

type NotificationProcessor interface {
	SendEmail(ctx context.Context, req *models.CreateNotificationRequest) (*models.Notification, error)
	SendSMS(ctx context.Context, req *models.CreateNotificationRequest) (*models.Notification, error)
	SendPush(ctx context.Context, req *models.CreateNotificationRequest) (*models.Notification, error)
}

type queueWorker struct {
	config    WorkerConfig
	queue     QueueManager
	processor NotificationProcessor
	logger    logger.Logger

	// Worker state
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	workerChan chan bool // Controls concurrency
	running    bool
	draining   bool // Flag to indicate shutdown in progress
	mu         sync.RWMutex
}

func NewQueueWorker(
	queue QueueManager,
	processor NotificationProcessor,
	logger logger.Logger,
	config WorkerConfig,
) Worker {
	return &queueWorker{
		config:     config,
		queue:      queue,
		processor:  processor,
		logger:     logger,
		workerChan: make(chan bool, config.MaxConcurrency),
	}
}

func (w *queueWorker) Start(ctx context.Context) error {
	if w.running {
		return fmt.Errorf("worker is already running")
	}

	w.ctx, w.cancel = context.WithCancel(ctx)
	w.running = true

	w.logger.Info("Starting queue worker",
		"worker_id", w.config.WorkerID,
		"poll_interval", w.config.PollInterval,
		"max_concurrency", w.config.MaxConcurrency,
	)

	w.wg.Add(1)
	go w.processLoop()

	return nil
}

func (w *queueWorker) Stop(ctx context.Context) error {
	if !w.running {
		return fmt.Errorf("worker is not running")
	}

	w.logger.Info("Stopping queue worker - initiating graceful drain",
		"worker_id", w.config.WorkerID,
		"max_drain_timeout", "30s",
	)

	// Set draining flag to stop accepting new items
	w.mu.Lock()
	w.draining = true
	w.mu.Unlock()

	// Signal to stop the processing loop
	w.cancel()

	// Wait for in-flight items to complete with timeout
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	// Wait for either all items to complete or timeout
	drainTimeout := 30 * time.Second
	select {
	case <-done:
		w.logger.Info("Queue worker stopped gracefully - all items processed",
			"worker_id", w.config.WorkerID,
		)
	case <-time.After(drainTimeout):
		w.logger.Warn("Queue worker stopped with timeout - some items may not have completed",
			"worker_id", w.config.WorkerID,
			"drain_timeout", drainTimeout,
		)
	case <-ctx.Done():
		w.logger.Warn("Queue worker stop cancelled by context",
			"worker_id", w.config.WorkerID,
		)
		return ctx.Err()
	}

	w.running = false
	return nil
}

func (w *queueWorker) processLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info("Worker stopping", "worker_id", w.config.WorkerID)
			return
		case <-ticker.C:
			w.processItems()
		}
	}
}

func (w *queueWorker) isItemTooOld(item *QueueItem) bool {
	// If MaxItemAge is not set (0), allow all items
	if w.config.MaxItemAge == 0 {
		return false
	}

	itemAge := time.Since(item.CreatedAt)
	return itemAge > w.config.MaxItemAge
}

func (w *queueWorker) processItems() {
	// Check if we're draining - don't accept new items during shutdown
	w.mu.RLock()
	isDraining := w.draining
	w.mu.RUnlock()

	if isDraining {
		w.logger.Debug("Worker is draining, not accepting new items",
			"worker_id", w.config.WorkerID,
		)
		return
	}

	for i := 0; i < w.config.MaxConcurrency; i++ {
		select {
		case w.workerChan <- true:
			// Acquire worker slot
			w.wg.Add(1)
			go w.processSingleItem()
		default:
			// All workers busy
			return
		}
	}
}

func (w *queueWorker) processSingleItem() {
	defer w.wg.Done()
	defer func() { <-w.workerChan }()

	ctx, cancel := context.WithTimeout(w.ctx, 30*time.Second)
	defer cancel()

	item, err := w.queue.Dequeue(ctx)
	if err != nil {
		w.logger.Debug("No items available in queue", "worker_id", w.config.WorkerID)
		return
	}

	if err := w.queue.ClaimItem(ctx, w.config.WorkerID, item); err != nil {
		w.logger.Error("Failed to claim queue item",
			"worker_id", w.config.WorkerID,
			"item_id", item.ID,
			"error", err,
		)
		return
	}

	// Check if item is too old and should be skipped
	if w.isItemTooOld(item) {
		w.logger.Warn("Skipping old queue item",
			"worker_id", w.config.WorkerID,
			"item_id", item.ID,
			"created_at", item.CreatedAt,
			"age_seconds", time.Since(item.CreatedAt).Seconds(),
			"max_age_seconds", w.config.MaxItemAge.Seconds(),
		)

		// Complete the item to remove it from the queue
		if err := w.queue.CompleteItem(ctx, w.config.WorkerID, item.ID); err != nil {
			w.logger.Error("Failed to complete old item",
				"worker_id", w.config.WorkerID,
				"item_id", item.ID,
				"error", err,
			)
		}
		return
	}

	// Log queue depth before processing
	w.logQueueDepth(ctx, item.Channel)

	startTime := time.Now()
	w.logger.Info("Processing queue item",
		"worker_id", w.config.WorkerID,
		"item_id", item.ID,
		"type", item.Type,
		"channel", item.Channel,
		"retry_count", item.RetryCount,
		"priority", item.Priority,
		"age_seconds", time.Since(item.CreatedAt).Seconds(),
	)

	if _, err := w.ProcessItem(ctx, item); err != nil {
		processingDuration := time.Since(startTime)
		w.logger.Error("Failed to process queue item",
			"worker_id", w.config.WorkerID,
			"item_id", item.ID,
			"type", item.Type,
			"channel", item.Channel,
			"retry_count", item.RetryCount,
			"processing_duration_ms", processingDuration.Milliseconds(),
			"error", err,
		)
		w.handleProcessingError(ctx, item, err)
		return
	}

	if err := w.queue.CompleteItem(ctx, w.config.WorkerID, item.ID); err != nil {
		w.logger.Error("Failed to complete item",
			"worker_id", w.config.WorkerID,
			"item_id", item.ID,
			"error", err,
		)
	}

	processingDuration := time.Since(startTime)
	w.logger.Info("Successfully processed queue item",
		"worker_id", w.config.WorkerID,
		"item_id", item.ID,
		"type", item.Type,
		"channel", item.Channel,
		"retry_count", item.RetryCount,
		"processing_duration_ms", processingDuration.Milliseconds(),
	)
}

// parseQueueItem converts a QueueItem back to a NotificationRequest
func (w *queueWorker) parseQueueItem(queueItem *QueueItem) (*models.CreateNotificationRequest, error) {
	slog.Info("Parsing queue item", "queueItem", queueItem)

	// Try to find notification_request wrapper first (from Kafka)
	notificationRequestData, exists := queueItem.Payload["notification_request"]
	if exists {
		slog.Info("Found notification_request wrapper", "notificationRequestData", notificationRequestData)

		switch req := notificationRequestData.(type) {
		case *models.NotificationRequest:
			return w.convertNotificationRequest(req), nil

		case *models.CreateNotificationRequest:
			// Already the correct type
			return req, nil

		case map[string]any:
			// Handle map data (most common case from Redis)
			return w.convertMapToCreateRequest(req)

		default:
			return nil, fmt.Errorf("invalid notification request type in queue item payload: %T", req)
		}
	}

	// If no notification_request wrapper, try to parse payload directly (from templates/direct queue insertion)
	slog.Info("No notification_request wrapper found, parsing payload directly")
	return w.parseDirectPayload(queueItem.Payload)
}

// convertNotificationRequest converts a NotificationRequest to CreateNotificationRequest
func (w *queueWorker) convertNotificationRequest(req *models.NotificationRequest) *models.CreateNotificationRequest {
	variables := make(map[string]string)
	for k, v := range req.Data.Variables {
		if str, ok := v.(string); ok {
			variables[k] = str
		} else {
			variables[k] = fmt.Sprintf("%v", v)
		}
	}

	var notificationType models.NotificationType
	switch req.Data.Channel {
	case "email":
		notificationType = models.NotificationTypeEmail
	case "sms":
		notificationType = models.NotificationTypeSMS
	case "push":
		notificationType = models.NotificationTypePush
	default:
		notificationType = models.NotificationTypeEmail // default
	}

	createReq := &models.CreateNotificationRequest{
		IdempotencyKey: req.Data.IdempotencyKey,
		Type:           notificationType,
		Recipients: []models.CreateRecipientRequest{
			{Address: req.Data.Recipient},
		},
		TemplateID: req.Data.TemplateID,
		Variables:  variables,
	}

	w.logger.Debug("Converted NotificationRequest to CreateNotificationRequest",
		"original_idempotency_key", req.Data.IdempotencyKey,
		"converted_idempotency_key", createReq.IdempotencyKey,
		"channel", req.Data.Channel,
		"type", createReq.Type,
	)

	return createReq
}

// convertMapToCreateRequest converts a map to CreateNotificationRequest
func (w *queueWorker) convertMapToCreateRequest(req map[string]any) (*models.CreateNotificationRequest, error) {
	// First try to unmarshal as NotificationRequest
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal notification request data: %w", err)
	}

	var notificationRequest models.NotificationRequest
	if err := json.Unmarshal(jsonData, &notificationRequest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal notification request: %w", err)
	}

	// Convert to CreateNotificationRequest
	createReq := w.convertNotificationRequest(&notificationRequest)

	w.logger.Debug("Converted map to CreateNotificationRequest",
		"idempotency_key", createReq.IdempotencyKey,
		"type", createReq.Type,
		"template_id", createReq.TemplateID,
	)

	return createReq, nil
}

// parseDirectPayload parses queue payload that contains notification fields directly (not wrapped in notification_request)
// This handles legacy template-rendered notifications or direct queue insertions
func (w *queueWorker) parseDirectPayload(payload map[string]any) (*models.CreateNotificationRequest, error) {
	w.logger.Debug("Parsing direct payload", "payload_keys", getMapKeys(payload))

	// Extract notification fields
	notificationID, _ := payload["notification_id"].(string)
	templateID, _ := payload["template_id"].(string)
	content, _ := payload["content"].(string)
	subject, _ := payload["subject"].(string)
	provider, _ := payload["provider"].(string)
	notifType, _ := payload["type"].(string)

	// Parse recipients
	recipients := []models.CreateRecipientRequest{}
	if recipientsData, ok := payload["recipients"].([]interface{}); ok {
		for _, recipientData := range recipientsData {
			if recipientMap, ok := recipientData.(map[string]interface{}); ok {
				address, _ := recipientMap["address"].(string)
				if address != "" {
					recipients = append(recipients, models.CreateRecipientRequest{
						Address: address,
					})
				}
			}
		}
	}

	// Parse variables
	variables := make(map[string]string)
	if variablesData, ok := payload["variables"].(map[string]interface{}); ok {
		for key, value := range variablesData {
			if str, ok := value.(string); ok {
				variables[key] = str
			} else {
				variables[key] = fmt.Sprintf("%v", value)
			}
		}
	}

	// Determine notification type
	var notificationType models.NotificationType
	switch notifType {
	case "email":
		notificationType = models.NotificationTypeEmail
	case "sms":
		notificationType = models.NotificationTypeSMS
	case "push":
		notificationType = models.NotificationTypePush
	default:
		notificationType = models.NotificationTypeEmail // default
	}

	createReq := &models.CreateNotificationRequest{
		IdempotencyKey: notificationID, // Use notification_id as idempotency key
		Type:           notificationType,
		Recipients:     recipients,
		TemplateID:     templateID,
		Variables:      variables,
		Content:        content,
		Subject:        subject,
		Provider:       provider,
	}

	w.logger.Debug("Parsed direct payload to CreateNotificationRequest",
		"idempotency_key", createReq.IdempotencyKey,
		"type", createReq.Type,
		"template_id", createReq.TemplateID,
		"recipients_count", len(createReq.Recipients),
		"has_content", content != "",
	)

	return createReq, nil
}

// getMapKeys returns the keys of a map as a slice
func getMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func (w *queueWorker) ProcessItem(ctx context.Context, item *QueueItem) (*models.Notification, error) {
	parsedItem, err := w.parseQueueItem(item)
	if err != nil {
		return nil, fmt.Errorf("failed to parse queue item: %w", err)
	}

	w.logger.Debug("Processing parsed item",
		"idempotency_key", parsedItem.IdempotencyKey,
		"type", parsedItem.Type,
		"template_id", parsedItem.TemplateID,
		"recipients_count", len(parsedItem.Recipients),
	)

	switch item.Type {
	case "email":
		return w.processor.SendEmail(ctx, parsedItem)
	case "sms":
		return w.processor.SendSMS(ctx, parsedItem)
	case "push":
		return w.processor.SendPush(ctx, parsedItem)
	default:
		return nil, fmt.Errorf("unknown notification type: %s", item.Type)
	}
}

func (w *queueWorker) handleProcessingError(ctx context.Context, item *QueueItem, err error) {
	w.logger.Error("Failed to process queue item",
		"worker_id", w.config.WorkerID,
		"item_id", item.ID,
		"type", item.Type,
		"retry_count", item.RetryCount,
		"error", err,
	)

	// Check if error is due to rate limiting
	retryDelay := w.calculateRetryDelayForError(item, err)

	if item.RetryCount >= item.MaxRetries {
		// Send to dead letter queue
		w.logger.Error("Item exceeded max retries, sending to DLQ",
			"worker_id", w.config.WorkerID,
			"item_id", item.ID,
			"max_retries", item.MaxRetries,
		)

		if dlqErr := w.queue.SendToDeadLetter(ctx, item, err.Error()); dlqErr != nil {
			w.logger.Error("Failed to send item to DLQ",
				"worker_id", w.config.WorkerID,
				"item_id", item.ID,
				"error", dlqErr,
			)
		}

		// Remove from processing queue
		if err := w.queue.CompleteItem(ctx, w.config.WorkerID, item.ID); err != nil {
			w.logger.Error("Failed to complete item after sending to DLQ",
				"worker_id", w.config.WorkerID,
				"item_id", item.ID,
				"error", err,
			)
		}
		return
	}

	// Increment retry count and update item in Redis first
	// This ensures retry count is persisted even if removal/re-enqueue fails
	item.RetryCount++

	// Update item with new retry count BEFORE removing from queue
	// This prevents infinite retry loops if removal fails
	if err := w.queue.ClaimItem(ctx, w.config.WorkerID, item); err != nil {
		w.logger.Error("Failed to update item retry count",
			"worker_id", w.config.WorkerID,
			"item_id", item.ID,
			"retry_count", item.RetryCount,
			"error", err,
		)
		// Release the item so it can be processed again later
		if err := w.queue.ReleaseItem(ctx, w.config.WorkerID, item.ID); err != nil {
			w.logger.Error("Failed to release item after update failure",
				"worker_id", w.config.WorkerID,
				"item_id", item.ID,
				"error", err,
			)
		}
		return
	}

	if err := w.queue.RemoveItem(ctx, item.ID); err != nil {
		w.logger.Error("Failed to remove item for retry",
			"worker_id", w.config.WorkerID,
			"item_id", item.ID,
			"error", err,
		)
		// Item data is already updated with new retry count via ClaimItem above
		// Release it so another worker can pick it up with the correct retry count
		if err := w.queue.ReleaseItem(ctx, w.config.WorkerID, item.ID); err != nil {
			w.logger.Error("Failed to release item after removal failure",
				"worker_id", w.config.WorkerID,
				"item_id", item.ID,
				"error", err,
			)
		}
		return
	}

	// Re-enqueue with backoff delay
	if retryDelay > 0 {
		if item.ScheduledFor == nil {
			item.ScheduledFor = new(time.Time)
		}
		*item.ScheduledFor = time.Now().Add(retryDelay)
	}

	if err := w.queue.Enqueue(ctx, item); err != nil {
		w.logger.Error("Failed to re-enqueue item for retry",
			"worker_id", w.config.WorkerID,
			"item_id", item.ID,
			"error", err,
		)
		// Send to DLQ as fallback
		if dlqErr := w.queue.SendToDeadLetter(ctx, item, fmt.Sprintf("Failed to re-enqueue for retry: %v", err)); dlqErr != nil {
			w.logger.Error("Failed to send to DLQ after re-enqueue failure",
				"worker_id", w.config.WorkerID,
				"item_id", item.ID,
				"error", dlqErr,
			)
		}
		return
	}

	// Determine retry reason for logging
	retryReason := "generic_error"
	if w.isRateLimitError(err) {
		retryReason = "rate_limit"
	} else if w.isCircuitBreakerError(err) {
		retryReason = "circuit_breaker"
	}

	w.logger.Info("Re-enqueued item for retry",
		"worker_id", w.config.WorkerID,
		"item_id", item.ID,
		"channel", item.Channel,
		"retry_count", item.RetryCount,
		"max_retries", item.MaxRetries,
		"delay_seconds", retryDelay.Seconds(),
		"retry_reason", retryReason,
		"scheduled_for", item.ScheduledFor,
	)
}

func (w *queueWorker) calculateRetryDelay(retryCount int) time.Duration {
	if retryCount == 0 {
		return 0
	}

	delay := min(
		w.config.RetryBackoff*time.Duration(1<<uint(retryCount-1)), w.config.MaxBackoff)

	return delay
}

// calculateRetryDelayForError determines the retry delay based on the error type
// For rate limit errors, it calculates delay until next rate limit window reset
// For circuit breaker errors, uses 30-second delay to allow recovery
// For other errors, it uses exponential backoff
func (w *queueWorker) calculateRetryDelayForError(item *QueueItem, err error) time.Duration {
	// Check if this is a rate limit error
	if w.isRateLimitError(err) {
		delay := w.calculateRateLimitRetryDelay(item.Channel)
		w.logger.Warn("Rate limit error detected, scheduling retry for next window",
			"worker_id", w.config.WorkerID,
			"item_id", item.ID,
			"channel", item.Channel,
			"retry_delay_minutes", delay.Minutes(),
		)
		return delay
	}

	// Check if this is a circuit breaker error
	if w.isCircuitBreakerError(err) {
		delay := 30 * time.Second // Circuit breaker opens for 30s
		w.logger.Warn("Circuit breaker error detected, scheduling retry after recovery period",
			"worker_id", w.config.WorkerID,
			"item_id", item.ID,
			"channel", item.Channel,
			"retry_delay_seconds", delay.Seconds(),
		)
		return delay
	}

	// For other errors, use standard exponential backoff
	return w.calculateRetryDelay(item.RetryCount)
}

// isRateLimitError checks if the error is a rate limit error
func (w *queueWorker) isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	// Check if error message contains rate limit indicators
	errMsg := err.Error()
	return contains(errMsg, "rate limit") || contains(errMsg, "Rate limit exceeded")
}

// isCircuitBreakerError checks if the error is a circuit breaker error
func (w *queueWorker) isCircuitBreakerError(err error) bool {
	if err == nil {
		return false
	}
	// Check if error message contains circuit breaker indicators
	errMsg := err.Error()
	return contains(errMsg, "circuit breaker")
}

// calculateRateLimitRetryDelay calculates when the next rate limit window resets
// For email (hourly limit): retry at top of next hour + 1 minute buffer
// For SMS (per-minute limit): retry at top of next minute + 5 seconds buffer
func (w *queueWorker) calculateRateLimitRetryDelay(channel string) time.Duration {
	now := time.Now()

	switch channel {
	case "email":
		// Email rate limit is hourly
		// Calculate next hour boundary
		nextHour := now.Truncate(time.Hour).Add(time.Hour)
		// Add 1-minute buffer to ensure rate limit counter has reset
		nextHour = nextHour.Add(1 * time.Minute)
		delay := time.Until(nextHour)

		// Handle edge case where delay might be negative due to clock skew
		if delay < 0 {
			delay = 0
		}

		// Ensure minimum delay of 5 minutes to avoid immediate retry
		if delay < 5*time.Minute {
			delay = 5 * time.Minute
		}
		return delay

	case "sms":
		// SMS rate limit is per-minute
		// Calculate next minute boundary
		nextMinute := now.Truncate(time.Minute).Add(time.Minute)
		// Add 5-second buffer
		nextMinute = nextMinute.Add(5 * time.Second)
		delay := time.Until(nextMinute)

		// Handle edge case where delay might be negative due to clock skew
		if delay < 0 {
			delay = 0
		}

		// Ensure minimum delay of 10 seconds
		if delay < 10*time.Second {
			delay = 10 * time.Second
		}
		return delay

	default:
		// For unknown channels, use 5-minute default delay
		return 5 * time.Minute
	}
}

// logQueueDepth logs current queue depth for monitoring
func (w *queueWorker) logQueueDepth(ctx context.Context, channel string) {
	depth, err := w.queue.GetQueueDepth(ctx, channel)
	if err != nil {
		w.logger.Warn("Failed to get queue depth",
			"worker_id", w.config.WorkerID,
			"channel", channel,
			"error", err,
		)
		return
	}

	w.logger.Info("Queue depth",
		"worker_id", w.config.WorkerID,
		"channel", channel,
		"pending_items", depth,
	)
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		WorkerID:       fmt.Sprintf("worker-%d", time.Now().Unix()),
		PollInterval:   1 * time.Second,
		MaxConcurrency: 10,
		RetryBackoff:   5 * time.Second,
		MaxBackoff:     5 * time.Minute,
		BatchSize:      1,
	}
}

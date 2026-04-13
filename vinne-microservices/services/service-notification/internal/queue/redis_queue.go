package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/randco/randco-microservices/shared/common/errors"
	"github.com/redis/go-redis/v9"
)

type QueueManager interface {
	Enqueue(ctx context.Context, item *QueueItem) error
	Dequeue(ctx context.Context) (*QueueItem, error)
	DequeueChannel(ctx context.Context, channel string) (*QueueItem, error)

	GetQueueDepth(ctx context.Context, channel string) (int64, error)
	SetPriority(ctx context.Context, itemID string, priority int) error
	RemoveItem(ctx context.Context, itemID string) error

	SendToDeadLetter(ctx context.Context, item *QueueItem, reason string) error
	GetDeadLetterItems(ctx context.Context, channel string) ([]*DeadLetterItem, error)
	RetryDeadLetterItem(ctx context.Context, itemID string) error

	ClaimItem(ctx context.Context, workerID string, item *QueueItem) error
	ReleaseItem(ctx context.Context, workerID string, itemID string) error
	CompleteItem(ctx context.Context, workerID string, itemID string) error

	GetProcessingItems(ctx context.Context, workerID string) ([]*QueueItem, error)
	GetQueueStats(ctx context.Context) (*QueueStats, error)

	Health(ctx context.Context) error
}

type QueueItem struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`     // email, sms, push
	Channel      string            `json:"channel"`  // email, sms, push
	Priority     int               `json:"priority"` // 1-4 (1=lowest, 4=highest)
	Payload      map[string]any    `json:"payload"`
	RetryCount   int               `json:"retry_count"`
	MaxRetries   int               `json:"max_retries"`
	CreatedAt    time.Time         `json:"created_at"`
	ScheduledFor *time.Time        `json:"scheduled_for,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	WorkerID     string            `json:"worker_id,omitempty"`
}

type DeadLetterItem struct {
	*QueueItem
	FailedAt      time.Time `json:"failed_at"`
	FailureReason string    `json:"failure_reason"`
	LastError     string    `json:"last_error"`
}

type QueueStats struct {
	TotalItems      int64            `json:"total_items"`
	ProcessingItems int64            `json:"processing_items"`
	DeadLetterItems int64            `json:"dead_letter_items"`
	ByPriority      map[string]int64 `json:"by_priority"`
	ByChannel       map[string]int64 `json:"by_channel"`
}

type redisQueueManager struct {
	client *redis.Client
}

func NewRedisQueueManager(client *redis.Client) QueueManager {
	// if err := redisotel.InstrumentTracing(client); err != nil {
	// 	fmt.Printf("Failed to instrument Redis tracing: %v\n", err)
	// }

	return &redisQueueManager{
		client: client,
	}
}

func (q *redisQueueManager) Enqueue(ctx context.Context, item *QueueItem) error {
	if item.Channel == "" {
		item.Channel = item.Type
	}
	if item.Priority == 0 {
		item.Priority = 2 // Default: normal priority
	}
	if item.MaxRetries == 0 {
		item.MaxRetries = 3
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}

	data, err := json.Marshal(item)
	if err != nil {
		return errors.NewInternalError("failed to marshal queue item", err)
	}

	pipe := q.client.Pipeline()

	var score float64
	if item.ScheduledFor != nil && item.ScheduledFor.After(time.Now()) {
		score = float64(item.ScheduledFor.Unix())
	} else {
		timestamp := int64(time.Now().Unix())
		score = float64(10000 - int64(item.Priority)*1000 - timestamp)
	}

	queueKey := queueKey(item.Channel)
	pipe.ZAdd(ctx, queueKey, redis.Z{Score: score, Member: item.ID})

	itemKey := itemKey(item.ID)
	pipe.Set(ctx, itemKey, data, 24*time.Hour) // TTL: 24 hours

	pipe.HIncrBy(ctx, statsKey("total"), "count", 1)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return errors.NewInternalError("failed to enqueue item", err)
	}

	return nil
}

func (q *redisQueueManager) Dequeue(ctx context.Context) (*QueueItem, error) {
	channels := []string{"push", "email", "sms"}

	for _, channel := range channels {
		if item, err := q.DequeueChannel(ctx, channel); err == nil {
			return item, nil
		}
	}

	return nil, errors.NewNotFoundError("no items available in queue")
}

func (q *redisQueueManager) DequeueChannel(ctx context.Context, channel string) (*QueueItem, error) {
	queueKey := queueKey(channel)
	processingKey := processingQueueKey(channel)

	// Lua script to atomically move item from queue to processing set
	// This ensures only one worker can claim an item
	luaScript := `
		local queue_key = KEYS[1]
		local processing_key = KEYS[2]
		local now = tonumber(ARGV[1])

		-- Get highest priority item (ZRevRange gets highest score first)
		local items = redis.call('ZREVRANGE', queue_key, 0, 0, 'WITHSCORES')
		if #items == 0 then
			return nil
		end

		local item_id = items[1]
		local score = items[2]

		-- Remove from queue and add to processing set
		redis.call('ZREM', queue_key, item_id)
		redis.call('ZADD', processing_key, now, item_id)

		return item_id
	`

	result, err := q.client.Eval(ctx, luaScript, []string{queueKey, processingKey}, time.Now().Unix()).Result()
	if err != nil {
		return nil, errors.NewInternalError("failed to dequeue item", err)
	}

	if result == nil {
		return nil, errors.NewNotFoundError("no items in channel")
	}

	itemID, ok := result.(string)
	if !ok {
		return nil, errors.NewInternalError("invalid item ID returned from queue", nil)
	}

	item, err := q.getItem(ctx, itemID)
	if err != nil {
		// Item data missing but ID exists in queue - clean up the orphaned entry
		q.client.ZRem(ctx, processingKey, itemID)
		return nil, errors.NewNotFoundError("item data not found")
	}

	return item, nil
}

func (q *redisQueueManager) GetQueueDepth(ctx context.Context, channel string) (int64, error) {
	queueKey := queueKey(channel)
	count, err := q.client.ZCard(ctx, queueKey).Result()
	if err != nil {
		return 0, errors.NewInternalError("failed to get queue depth", err)
	}
	return count, nil
}

func (q *redisQueueManager) SetPriority(ctx context.Context, itemID string, priority int) error {
	item, err := q.getItem(ctx, itemID)
	if err != nil {
		return err
	}

	item.Priority = priority

	queueKey := queueKey(item.Channel)
	pipe := q.client.Pipeline()

	pipe.ZRem(ctx, queueKey, itemID)

	data, err := json.Marshal(item)
	if err != nil {
		return errors.NewInternalError("failed to marshal queue item", err)
	}

	itemKeyStr := itemKey(itemID)
	pipe.Set(ctx, itemKeyStr, data, 24*time.Hour)

	score := float64(10000 - int64(item.Priority)*1000 - int64(time.Now().Unix()))
	pipe.ZAdd(ctx, queueKey, redis.Z{Score: score, Member: itemID})

	_, err = pipe.Exec(ctx)
	if err != nil {
		return errors.NewInternalError("failed to update priority", err)
	}

	return nil
}

func (q *redisQueueManager) RemoveItem(ctx context.Context, itemID string) error {
	item, err := q.getItem(ctx, itemID)
	if err != nil {
		return err
	}

	queueKey := queueKey(item.Channel)
	processingKey := processingQueueKey(item.Channel)
	itemKeyStr := itemKey(itemID)

	pipe := q.client.Pipeline()
	pipe.ZRem(ctx, queueKey, itemID)
	pipe.ZRem(ctx, processingKey, itemID) // Also remove from processing set
	pipe.Del(ctx, itemKeyStr)
	pipe.HIncrBy(ctx, statsKey("total"), "count", -1)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return errors.NewInternalError("failed to remove item", err)
	}

	return nil
}

func (q *redisQueueManager) SendToDeadLetter(ctx context.Context, item *QueueItem, reason string) error {
	dlqItem := &DeadLetterItem{
		QueueItem:     item,
		FailedAt:      time.Now().UTC(),
		FailureReason: reason,
	}

	// Serialize DLQ item
	data, err := json.Marshal(dlqItem)
	if err != nil {
		return errors.NewInternalError("failed to marshal DLQ item", err)
	}

	dlqKey := deadLetterQueueKey(item.Channel)
	dlqItemKey := deadLetterItemKey(item.ID)

	pipe := q.client.Pipeline()

	// Add to DLQ sorted set
	pipe.ZAdd(ctx, dlqKey, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: item.ID,
	})

	// Store DLQ item data
	pipe.Set(ctx, dlqItemKey, data, 24*time.Hour) // Keep for 24 hours

	// Update stats
	pipe.HIncrBy(ctx, statsKey("dlq"), item.Channel, 1)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return errors.NewInternalError("failed to send to dead letter queue", err)
	}

	return nil
}

func (q *redisQueueManager) GetDeadLetterItems(ctx context.Context, channel string) ([]*DeadLetterItem, error) {
	dlqKey := deadLetterQueueKey(channel)

	// Get all DLQ items (most recent first)
	result, err := q.client.ZRevRangeWithScores(ctx, dlqKey, 0, -1).Result()
	if err != nil {
		return nil, errors.NewInternalError("failed to get DLQ items", err)
	}

	var items []*DeadLetterItem
	for _, z := range result {
		itemID := z.Member.(string)
		item, err := q.getDeadLetterItem(ctx, itemID)
		if err != nil {
			continue // Skip invalid items
		}
		items = append(items, item)
	}

	return items, nil
}

func (q *redisQueueManager) RetryDeadLetterItem(ctx context.Context, itemID string) error {
	item, err := q.getDeadLetterItem(ctx, itemID)
	if err != nil {
		return err
	}

	// Reset retry count and re-enqueue
	item.RetryCount = 0
	item.WorkerID = ""

	// Remove from DLQ and re-enqueue
	dlqKey := deadLetterQueueKey(item.Channel)
	dlqItemKey := deadLetterItemKey(itemID)

	pipe := q.client.Pipeline()
	pipe.ZRem(ctx, dlqKey, itemID)
	pipe.Del(ctx, dlqItemKey)
	pipe.HIncrBy(ctx, statsKey("dlq"), item.Channel, -1)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return errors.NewInternalError("failed to remove from DLQ", err)
	}

	// Re-enqueue the item
	return q.Enqueue(ctx, item.QueueItem)
}

func (q *redisQueueManager) ClaimItem(ctx context.Context, workerID string, item *QueueItem) error {
	item.WorkerID = workerID

	// Update item data
	data, err := json.Marshal(item)
	if err != nil {
		return errors.NewInternalError("failed to marshal queue item", err)
	}

	itemKeyStr := itemKey(item.ID)
	return q.client.Set(ctx, itemKeyStr, data, 24*time.Hour).Err()
}

func (q *redisQueueManager) ReleaseItem(ctx context.Context, workerID string, itemID string) error {
	item, err := q.getItem(ctx, itemID)
	if err != nil {
		return err
	}

	if item.WorkerID != "" && item.WorkerID != workerID {
		return errors.NewConflictError("item claimed by different worker")
	}

	// Remove from processing set first
	processingKey := processingQueueKey(item.Channel)
	if err := q.client.ZRem(ctx, processingKey, itemID).Err(); err != nil {
		return errors.NewInternalError("failed to remove from processing set", err)
	}

	// Clear worker and re-enqueue
	item.WorkerID = ""
	return q.Enqueue(ctx, item)
}

func (q *redisQueueManager) CompleteItem(ctx context.Context, workerID string, itemID string) error {
	item, err := q.getItem(ctx, itemID)
	if err != nil {
		return err
	}

	if item.WorkerID != "" && item.WorkerID != workerID {
		return errors.NewConflictError("item claimed by different worker")
	}

	// Remove from processing set
	processingKey := processingQueueKey(item.Channel)
	pipe := q.client.Pipeline()
	pipe.ZRem(ctx, processingKey, itemID)
	pipe.Del(ctx, itemKey(itemID))
	pipe.HIncrBy(ctx, statsKey("total"), "count", -1)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return errors.NewInternalError("failed to complete item", err)
	}

	return nil
}

func (q *redisQueueManager) GetProcessingItems(ctx context.Context, workerID string) ([]*QueueItem, error) {
	// This would need a separate tracking mechanism in a real implementation
	// For now, return empty slice
	return []*QueueItem{}, nil
}

// GetQueueStats returns current queue statistics
func (q *redisQueueManager) GetQueueStats(ctx context.Context) (*QueueStats, error) {
	stats := &QueueStats{
		ByPriority: make(map[string]int64),
		ByChannel:  make(map[string]int64),
	}

	channels := []string{"email", "sms", "push"}
	for _, channel := range channels {
		depth, err := q.GetQueueDepth(ctx, channel)
		if err != nil {
			continue
		}
		stats.ByChannel[channel] = depth
		stats.TotalItems += depth
	}

	return stats, nil
}

func (q *redisQueueManager) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := q.client.Ping(ctx).Err(); err != nil {
		return errors.NewInternalError("Redis queue health check failed", err)
	}
	return nil
}

func (q *redisQueueManager) getItem(ctx context.Context, itemID string) (*QueueItem, error) {
	itemKeyStr := itemKey(itemID)
	data, err := q.client.Get(ctx, itemKeyStr).Result()
	if err == redis.Nil {
		return nil, errors.NewNotFoundError("queue item not found")
	}
	if err != nil {
		return nil, errors.NewInternalError("failed to get queue item", err)
	}

	var item QueueItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return nil, errors.NewInternalError("failed to unmarshal queue item", err)
	}

	return &item, nil
}

func (q *redisQueueManager) getDeadLetterItem(ctx context.Context, itemID string) (*DeadLetterItem, error) {
	dlqItemKey := deadLetterItemKey(itemID)
	data, err := q.client.Get(ctx, dlqItemKey).Result()
	if err == redis.Nil {
		return nil, errors.NewNotFoundError("DLQ item not found")
	}
	if err != nil {
		return nil, errors.NewInternalError("failed to get DLQ item", err)
	}

	var item DeadLetterItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return nil, errors.NewInternalError("failed to unmarshal DLQ item", err)
	}

	return &item, nil
}

func queueKey(channel string) string {
	return fmt.Sprintf("notification:queue:%s", channel)
}

func processingQueueKey(channel string) string {
	return fmt.Sprintf("notification:processing:%s", channel)
}

func itemKey(itemID string) string {
	return fmt.Sprintf("notification:queue:item:%s", itemID)
}

func deadLetterQueueKey(channel string) string {
	return fmt.Sprintf("notification:dlq:%s", channel)
}

func deadLetterItemKey(itemID string) string {
	return fmt.Sprintf("notification:dlq:item:%s", itemID)
}

func statsKey(metric string) string {
	return fmt.Sprintf("notification:stats:%s", metric)
}

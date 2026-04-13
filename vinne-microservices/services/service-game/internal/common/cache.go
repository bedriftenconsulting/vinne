package common

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// ProcessedScheduleCache tracks recently processed schedules to prevent duplicate processing
type ProcessedScheduleCache struct {
	mu    sync.RWMutex
	cache map[string]time.Time // key format: "scheduleID:eventType"
}

// NewProcessedScheduleCache creates a new ProcessedScheduleCache
func NewProcessedScheduleCache() *ProcessedScheduleCache {
	return &ProcessedScheduleCache{
		cache: make(map[string]time.Time),
	}
}

// WasRecentlyProcessed checks if a schedule was processed for a specific event within the given duration
func (c *ProcessedScheduleCache) WasRecentlyProcessed(id uuid.UUID, eventType string, within time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := id.String() + ":" + eventType
	if processedAt, exists := c.cache[key]; exists {
		return time.Since(processedAt) < within
	}
	return false
}

// MarkProcessed marks a schedule as processed for a specific event with the current timestamp
func (c *ProcessedScheduleCache) MarkProcessed(id uuid.UUID, eventType string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := id.String() + ":" + eventType
	c.cache[key] = time.Now()
}

// Cleanup removes entries older than the specified duration
func (c *ProcessedScheduleCache) Cleanup(olderThan time.Duration) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	removedCount := 0

	for id, processedAt := range c.cache {
		if processedAt.Before(cutoff) {
			delete(c.cache, id)
			removedCount++
		}
	}

	return removedCount
}

// Size returns the current number of entries in the cache
func (c *ProcessedScheduleCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.cache)
}

// Clear removes all entries from the cache
func (c *ProcessedScheduleCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]time.Time)
}

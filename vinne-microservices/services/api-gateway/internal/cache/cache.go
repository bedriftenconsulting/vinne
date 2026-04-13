package cache

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheConfig holds cache configuration
type CacheConfig struct {
	TTL                time.Duration
	KeyPrefix          string
	MaxCacheSize       int64
	CacheableEndpoints map[string]time.Duration // endpoint -> custom TTL
}

// ResponseCache handles response caching
type ResponseCache struct {
	client *redis.Client
	config *CacheConfig
}

// CachedResponse represents a cached API response
type CachedResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	CachedAt   time.Time         `json:"cached_at"`
	TTL        time.Duration     `json:"ttl"`
}

// NewResponseCache creates a new response cache
func NewResponseCache(client *redis.Client, config *CacheConfig) *ResponseCache {
	if config == nil {
		config = &CacheConfig{
			TTL:          5 * time.Minute,
			KeyPrefix:    "api:cache:",
			MaxCacheSize: 1024 * 1024, // 1MB default
		}
	}

	if config.CacheableEndpoints == nil {
		// Default cacheable endpoints - these are not security-sensitive
		config.CacheableEndpoints = map[string]time.Duration{
			"GET:/api/v1/games":              10 * time.Minute,
			"GET:/api/v1/games/{id}":         5 * time.Minute,
			"GET:/api/v1/draws/latest":       1 * time.Minute,
			"GET:/api/v1/draws/{id}/results": 30 * time.Minute,
			"GET:/api/v1/public/games":       15 * time.Minute,
			"GET:/api/v1/public/results":     5 * time.Minute,
			"GET:/api/v1/public/jackpots":    2 * time.Minute,
		}
	}

	return &ResponseCache{
		client: client,
		config: config,
	}
}

// GenerateCacheKey creates a cache key from request details
func (c *ResponseCache) GenerateCacheKey(method, path string, queryParams map[string][]string, userID string) string {
	h := md5.New()
	h.Write([]byte(method))
	h.Write([]byte(path))

	// Include query params in cache key
	if len(queryParams) > 0 {
		params, _ := json.Marshal(queryParams)
		h.Write(params)
	}

	// Include user ID for personalized caching
	if userID != "" {
		h.Write([]byte(userID))
	}

	hash := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("%s%s", c.config.KeyPrefix, hash)
}

// Get retrieves a cached response
func (c *ResponseCache) Get(ctx context.Context, key string) (*CachedResponse, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, err
	}

	var response CachedResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// Set stores a response in cache
func (c *ResponseCache) Set(ctx context.Context, key string, response *CachedResponse, ttl time.Duration) error {
	// Check size limit
	if int64(len(response.Body)) > c.config.MaxCacheSize {
		return fmt.Errorf("response too large to cache: %d bytes", len(response.Body))
	}

	data, err := json.Marshal(response)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, data, ttl).Err()
}

// Delete removes a cached response
func (c *ResponseCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// InvalidatePattern invalidates all cache entries matching a pattern
func (c *ResponseCache) InvalidatePattern(ctx context.Context, pattern string) error {
	keys, err := c.client.Keys(ctx, c.config.KeyPrefix+pattern).Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return c.client.Del(ctx, keys...).Err()
	}

	return nil
}

// IsCacheable determines if a request should be cached
func (c *ResponseCache) IsCacheable(method, path string) (bool, time.Duration) {
	endpoint := fmt.Sprintf("%s:%s", method, path)

	// Check if endpoint is in cacheable list
	if ttl, ok := c.config.CacheableEndpoints[endpoint]; ok {
		return true, ttl
	}

	// Only cache GET requests by default
	if method == "GET" {
		return true, c.config.TTL
	}

	return false, 0
}

// InvalidateOnWrite invalidates related cache entries on write operations
func (c *ResponseCache) InvalidateOnWrite(ctx context.Context, method, path string) error {
	// Map of write operations to cache invalidation patterns - not security-sensitive
	invalidationRules := map[string][]string{
		"POST:/api/v1/games":        {"games*"},
		"PUT:/api/v1/games/{id}":    {"games*"},
		"DELETE:/api/v1/games/{id}": {"games*"},
		"POST:/api/v1/draws":        {"draws*", "results*"},
		"PUT:/api/v1/draws/{id}":    {"draws*", "results*"},
	}

	endpoint := fmt.Sprintf("%s:%s", method, path)

	if patterns, ok := invalidationRules[endpoint]; ok {
		for _, pattern := range patterns {
			if err := c.InvalidatePattern(ctx, pattern); err != nil {
				return err
			}
		}
	}

	return nil
}

// CacheStats returns cache statistics
func (c *ResponseCache) CacheStats(ctx context.Context) (map[string]interface{}, error) {
	info, err := c.client.Info(ctx, "stats").Result()
	if err != nil {
		return nil, err
	}

	keys, err := c.client.DBSize(ctx).Result()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"keys":   keys,
		"info":   info,
		"prefix": c.config.KeyPrefix,
	}, nil
}

// CacheWarming pre-populates cache with frequently accessed data
func (c *ResponseCache) WarmCache(ctx context.Context, warmer func(context.Context) map[string]*CachedResponse) error {
	responses := warmer(ctx)

	for key, response := range responses {
		if err := c.Set(ctx, key, response, response.TTL); err != nil {
			return err
		}
	}

	return nil
}

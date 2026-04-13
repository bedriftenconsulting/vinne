package middleware

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/randco/randco-microservices/services/api-gateway/internal/cache"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
)

// cacheResponseWriter captures response for caching
type cacheResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	headers    http.Header
}

func newCacheResponseWriter(w http.ResponseWriter) *cacheResponseWriter {
	return &cacheResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
		headers:        make(http.Header),
	}
}

func (rw *cacheResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *cacheResponseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

func (rw *cacheResponseWriter) Header() http.Header {
	return rw.ResponseWriter.Header()
}

// CacheMiddleware handles response caching
func CacheMiddleware(responseCache *cache.ResponseCache) router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Only cache if method and path are cacheable
			cacheable, ttl := responseCache.IsCacheable(r.Method, r.URL.Path)
			if !cacheable {
				// For write operations, invalidate related cache
				if r.Method != "GET" {
					_ = responseCache.InvalidateOnWrite(r.Context(), r.Method, r.URL.Path)
				}
				return next(w, r)
			}

			// Generate cache key
			userID := router.GetUserID(r)
			cacheKey := responseCache.GenerateCacheKey(
				r.Method,
				r.URL.Path,
				r.URL.Query(),
				userID,
			)

			// Check if response is cached
			if cached, err := responseCache.Get(r.Context(), cacheKey); err == nil && cached != nil {
				// Serve from cache
				w.Header().Set("X-Cache", "HIT")
				w.Header().Set("X-Cache-Key", cacheKey)
				w.Header().Set("X-Cached-At", cached.CachedAt.Format(time.RFC3339))

				// Set cached headers
				for key, value := range cached.Headers {
					w.Header().Set(key, value)
				}

				w.WriteHeader(cached.StatusCode)
				_, _ = w.Write(cached.Body)
				return nil
			}

			// Cache miss - capture response
			w.Header().Set("X-Cache", "MISS")
			w.Header().Set("X-Cache-Key", cacheKey)

			rw := newCacheResponseWriter(w)
			err := next(rw, r)

			// Only cache successful responses
			if err == nil && rw.statusCode >= 200 && rw.statusCode < 300 {
				// Store in cache
				response := &cache.CachedResponse{
					StatusCode: rw.statusCode,
					Headers:    make(map[string]string),
					Body:       rw.body.Bytes(),
					CachedAt:   time.Now(),
					TTL:        ttl,
				}

				// Copy important headers
				for key := range rw.Header() {
					response.Headers[key] = rw.Header().Get(key)
				}

				// Store in cache asynchronously with independent context
				go func() {
					// Use background context since request context may be cancelled
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					_ = responseCache.Set(ctx, cacheKey, response, ttl)
				}()
			}

			return err
		}
	}
}

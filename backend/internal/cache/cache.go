package cache

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/monitoring"
	"github.com/gin-gonic/gin"
)

// CacheItem represents a cached item with expiration
type CacheItem struct {
	Data      []byte    `json:"data"`
	ExpiresAt time.Time `json:"expires_at"`
}

// IsExpired checks if the cache item has expired
func (c *CacheItem) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// Cache provides thread-safe caching with TTL
type Cache struct {
	mu    sync.RWMutex
	items map[string]*CacheItem
	ttl   time.Duration
}

// NewCache creates a new cache with the specified TTL
func NewCache(ttl time.Duration) *Cache {
	cache := &Cache{
		items: make(map[string]*CacheItem),
		ttl:   ttl,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// cleanup removes expired items periodically
func (c *Cache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		for key, item := range c.items {
			if item.IsExpired() {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

// generateKey creates a consistent key from the input
func (c *Cache) generateKey(input string) string {
	hash := md5.Sum([]byte(input))
	return fmt.Sprintf("%x", hash)
}

// Get retrieves an item from the cache
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists || item.IsExpired() {
		if exists && item.IsExpired() {
			// Clean up expired item
			go func() {
				c.mu.Lock()
				delete(c.items, key)
				c.mu.Unlock()
			}()
		}
		return nil, false
	}

	return item.Data, true
}

// Set stores an item in the cache
func (c *Cache) Set(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &CacheItem{
		Data:      data,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Delete removes an item from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Clear removes all items from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*CacheItem)
}

// Size returns the number of items in the cache
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}

// Stats returns cache statistics
func (c *Cache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalItems := len(c.items)
	expiredItems := 0

	for _, item := range c.items {
		if item.IsExpired() {
			expiredItems++
		}
	}

	return map[string]interface{}{
		"total_items":   totalItems,
		"expired_items": expiredItems,
		"active_items":  totalItems - expiredItems,
		"ttl_seconds":   c.ttl.Seconds(),
	}
}

// Middleware creates a Gin middleware for caching responses
func (c *Cache) Middleware(metrics *monitoring.Metrics) func(*gin.Context) {
	return func(ctx *gin.Context) {
		// Only cache POST requests to /analyze
		if ctx.Request.Method != "POST" || ctx.Request.URL.Path != "/analyze" {
			ctx.Next()
			return
		}

		// Read request body
		body, err := io.ReadAll(ctx.Request.Body)
		if err != nil {
			ctx.Next()
			return
		}

		// Restore body for next handler
		ctx.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		// Generate cache key from request body
		cacheKey := c.generateKey(string(body))

		// Check cache
		if cachedData, found := c.Get(cacheKey); found {
			slog.Info("Cache hit", "key", cacheKey[:8]+"...")
			metrics.IncrementCacheHit()
			ctx.Data(http.StatusOK, "application/json", cachedData)
			ctx.Abort()
			return
		}

		// Cache miss - capture response
		slog.Info("Cache miss", "key", cacheKey[:8]+"...")
		metrics.IncrementCacheMiss()

		// Create a response writer wrapper to capture the response
		wrapper := &responseWriter{ResponseWriter: ctx.Writer, body: &bytes.Buffer{}}

		ctx.Writer = wrapper
		ctx.Next()

		// Cache the response if successful
		if ctx.Writer.Status() == http.StatusOK {
			c.Set(cacheKey, wrapper.body.Bytes())
			slog.Info("Response cached", "key", cacheKey[:8]+"...")
		}
	}
}

// responseWriter wraps gin.ResponseWriter to capture response body
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *responseWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

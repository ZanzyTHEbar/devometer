package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/monitoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiterFallbackMode(t *testing.T) {
	// Create rate limiter without Redis (fallback mode)
	redisClient := &RedisClient{enabled: false}
	config := Config{
		IPLimit:         10,
		UserLimit:       5,
		BurstMultiplier: 2,
		EnableFallback:  true,
		CleanupInterval: 1 * time.Hour,
	}
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()
	key := "test:user:123"
	rateLimit := Rate{
		Limit:  5,
		Period: time.Minute,
	}

	// First 5 requests should be allowed
	for i := 0; i < 5; i++ {
		result, err := limiter.Allow(ctx, key, rateLimit)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Request %d should be allowed", i+1)
		assert.Equal(t, 5, result.Limit)
	}

	// 6th request should be blocked
	result, err := limiter.Allow(ctx, key, rateLimit)
	require.NoError(t, err)
	assert.False(t, result.Allowed, "6th request should be blocked")
	assert.Greater(t, result.RetryAfter, time.Duration(0))
}

func TestRateLimiterBurstCapacity(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := Config{
		IPLimit:         10,
		UserLimit:       5,
		BurstMultiplier: 2,
		EnableFallback:  true,
		CleanupInterval: 1 * time.Hour,
	}
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()
	key := "test:burst:user"
	rateLimit := Rate{
		Limit:  5,
		Period: time.Second,
	}

	// With burst multiplier of 2, we should allow 10 requests initially
	allowedCount := 0
	for i := 0; i < 15; i++ {
		result, err := limiter.Allow(ctx, key, rateLimit)
		require.NoError(t, err)
		if result.Allowed {
			allowedCount++
		}
	}

	// Should allow burst capacity
	assert.GreaterOrEqual(t, allowedCount, 5, "Should allow at least limit amount")
	assert.LessOrEqual(t, allowedCount, 12, "Should not exceed burst + small margin")
}

func TestRateLimiterMultipleKeys(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := DefaultConfig()
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()
	rateLimit := Rate{
		Limit:  3,
		Period: time.Minute,
	}

	// Test that different keys have independent rate limits
	keys := []string{"user:1", "user:2", "user:3"}

	for _, key := range keys {
		for i := 0; i < 3; i++ {
			result, err := limiter.Allow(ctx, key, rateLimit)
			require.NoError(t, err)
			assert.True(t, result.Allowed, "Key %s request %d should be allowed", key, i+1)
		}

		// 4th request for each key should be blocked
		result, err := limiter.Allow(ctx, key, rateLimit)
		require.NoError(t, err)
		assert.False(t, result.Allowed, "Key %s 4th request should be blocked", key)
	}
}

func TestRateLimiterStats(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := DefaultConfig()
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()
	rateLimit := Rate{Limit: 5, Period: time.Minute}

	// Make some requests
	for i := 0; i < 3; i++ {
		_, _ = limiter.Allow(ctx, "test:stats", rateLimit)
	}

	stats := limiter.GetStats()
	assert.NotNil(t, stats)
	assert.False(t, stats["redis_enabled"].(bool))
	assert.True(t, stats["fallback_enabled"].(bool))

	statsConfig := stats["config"].(map[string]interface{})
	assert.Equal(t, 60, statsConfig["ip_limit_per_min"])
}

func TestRateLimiterCleanup(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := Config{
		IPLimit:         10,
		UserLimit:       5,
		BurstMultiplier: 2,
		EnableFallback:  true,
		CleanupInterval: 10 * time.Millisecond,
	}
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()
	rateLimit := Rate{Limit: 5, Period: time.Minute}

	// Create many limiters to trigger cleanup threshold
	for i := 0; i < 10001; i++ {
		key := "test:cleanup:" + string(rune(i))
		_, _ = limiter.Allow(ctx, key, rateLimit)
	}

	// Wait for cleanup
	time.Sleep(50 * time.Millisecond)

	// Force cleanup
	limiter.cleanup()

	stats := limiter.GetStats()
	fallbackCount := stats["fallback_limiters"].(int)
	assert.Less(t, fallbackCount, 10001, "Cleanup should have reduced limiter count")
}

func TestRateLimiterConcurrency(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := DefaultConfig()
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()
	key := "test:concurrent"
	rateLimit := Rate{Limit: 100, Period: time.Second}

	// Run 50 concurrent goroutines making requests
	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_, err := limiter.Allow(ctx, key, rateLimit)
				assert.NoError(t, err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 50; i++ {
		<-done
	}
}

func TestRateLimiterContextCancellation(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := DefaultConfig()
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	key := "test:cancelled"
	rateLimit := Rate{Limit: 5, Period: time.Minute}

	// Should still work with cancelled context in fallback mode
	result, err := limiter.Allow(ctx, key, rateLimit)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRateLimiterDifferentPeriods(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := DefaultConfig()
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()

	tests := []struct {
		name   string
		limit  int
		period time.Duration
	}{
		{"per second", 10, time.Second},
		{"per minute", 60, time.Minute},
		{"per hour", 1000, time.Hour},
		{"per day", 5000, 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "test:" + tt.name
			rateLimit := Rate{Limit: tt.limit, Period: tt.period}

			// First request should always be allowed
			result, err := limiter.Allow(ctx, key, rateLimit)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, tt.limit, result.Limit)
		})
	}
}

package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/monitoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvalidateUser(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := DefaultConfig()
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()
	userID := "user123"

	// Create rate limit for user
	key := "ratelimit:user:" + userID + ":week"
	rateLimit := Rate{Limit: 5, Period: time.Hour}

	// Use up some requests
	for i := 0; i < 3; i++ {
		_, err := limiter.Allow(ctx, key, rateLimit)
		require.NoError(t, err)
	}

	// Verify user has limits
	result, err := limiter.Allow(ctx, key, rateLimit)
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	// Invalidate user
	err = limiter.InvalidateUser(ctx, userID)
	require.NoError(t, err)

	// After invalidation, user should have fresh limits
	for i := 0; i < 5; i++ {
		result, err := limiter.Allow(ctx, key, rateLimit)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Request %d should be allowed after invalidation", i+1)
	}
}

func TestInvalidateIP(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := DefaultConfig()
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()
	ip := "192.168.1.1"

	// Create rate limit for IP
	key := "ratelimit:ip:" + ip + ":minute"
	rateLimit := Rate{Limit: 3, Period: time.Minute}

	// Use up all requests
	for i := 0; i < 3; i++ {
		_, err := limiter.Allow(ctx, key, rateLimit)
		require.NoError(t, err)
	}

	// Next request should be blocked
	result, err := limiter.Allow(ctx, key, rateLimit)
	require.NoError(t, err)
	assert.False(t, result.Allowed)

	// Invalidate IP
	err = limiter.InvalidateIP(ctx, ip)
	require.NoError(t, err)

	// After invalidation, IP should have fresh limits
	result, err = limiter.Allow(ctx, key, rateLimit)
	require.NoError(t, err)
	assert.True(t, result.Allowed, "Request should be allowed after IP invalidation")
}

func TestResetOnUpgrade(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := DefaultConfig()
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()
	userID := "user456"

	// Create weekly limit for free user
	weekKey := "ratelimit:user:" + userID + ":week"
	rateLimit := Rate{Limit: 5, Period: 7 * 24 * time.Hour}

	// Use all free requests
	for i := 0; i < 5; i++ {
		_, err := limiter.Allow(ctx, weekKey, rateLimit)
		require.NoError(t, err)
	}

	// Verify user is rate limited
	result, err := limiter.Allow(ctx, weekKey, rateLimit)
	require.NoError(t, err)
	assert.False(t, result.Allowed, "User should be rate limited")

	// User upgrades to paid
	err = limiter.ResetOnUpgrade(ctx, userID)
	require.NoError(t, err)

	// After upgrade, weekly limit should be reset
	result, err = limiter.Allow(ctx, weekKey, rateLimit)
	require.NoError(t, err)
	assert.True(t, result.Allowed, "User should have access after upgrade")
}

func TestInvalidateAll(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := DefaultConfig()
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()
	rateLimit := Rate{Limit: 5, Period: time.Minute}

	// Create multiple rate limiters
	keys := []string{"user:1", "user:2", "ip:1", "ip:2"}
	for _, key := range keys {
		for i := 0; i < 3; i++ {
			_, err := limiter.Allow(ctx, key, rateLimit)
			require.NoError(t, err)
		}
	}

	// Verify limiters exist
	stats := limiter.GetStats()
	assert.Greater(t, stats["fallback_limiters"].(int), 0)

	// Invalidate all
	err := limiter.InvalidateAll(ctx)
	require.NoError(t, err)

	// After invalidation, all keys should have fresh limits
	for _, key := range keys {
		result, err := limiter.Allow(ctx, key, rateLimit)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Key %s should have fresh limits", key)
	}
}

func TestGetKeyCount(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := DefaultConfig()
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()
	rateLimit := Rate{Limit: 5, Period: time.Minute}

	// Initially should have no keys
	count, err := limiter.GetKeyCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Create some limiters
	keys := []string{"user:1", "user:2", "user:3"}
	for _, key := range keys {
		_, _ = limiter.Allow(ctx, key, rateLimit)
	}

	// Should have 3 keys
	count, err = limiter.GetKeyCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestInvalidationWithMultiplePatterns(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := DefaultConfig()
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()
	rateLimit := Rate{Limit: 5, Period: time.Minute}

	userID := "user123"

	// Create multiple keys for the same user
	userKeys := []string{
		"ratelimit:user:" + userID + ":week",
		"ratelimit:user:" + userID + ":day",
		"ratelimit:user:" + userID + ":hour",
	}

	for _, key := range userKeys {
		for i := 0; i < 3; i++ {
			_, err := limiter.Allow(ctx, key, rateLimit)
			require.NoError(t, err)
		}
	}

	// Invalidate user should remove all user keys
	err := limiter.InvalidateUser(ctx, userID)
	require.NoError(t, err)

	// All user keys should have fresh limits
	for _, key := range userKeys {
		result, err := limiter.Allow(ctx, key, rateLimit)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Key %s should have fresh limits", key)
	}
}

func TestInvalidationDoesNotAffectOtherUsers(t *testing.T) {
	redisClient := &RedisClient{enabled: false}
	config := DefaultConfig()
	metrics := monitoring.NewMetrics()

	limiter := NewRateLimiter(redisClient, config, metrics)
	defer limiter.Close()

	ctx := context.Background()
	rateLimit := Rate{Limit: 5, Period: time.Minute}

	user1Key := "ratelimit:user:user1:week"
	user2Key := "ratelimit:user:user2:week"

	// Use up requests for both users
	for i := 0; i < 3; i++ {
		_, _ = limiter.Allow(ctx, user1Key, rateLimit)
		_, _ = limiter.Allow(ctx, user2Key, rateLimit)
	}

	// Invalidate only user1
	err := limiter.InvalidateUser(ctx, "user1")
	require.NoError(t, err)

	// User1 should have fresh limits
	result, err := limiter.Allow(ctx, user1Key, rateLimit)
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	// User2 should still have used requests
	result, err = limiter.Allow(ctx, user2Key, rateLimit)
	require.NoError(t, err)
	assert.True(t, result.Allowed) // Still has remaining from initial 5
}

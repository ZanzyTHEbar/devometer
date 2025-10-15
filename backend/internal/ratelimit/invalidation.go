package ratelimit

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// InvalidateUser removes all rate limit keys for a specific user
// Used when user upgrades to paid plan or when manually resetting limits
func (rl *RateLimiter) InvalidateUser(ctx context.Context, userID string) error {
	if !rl.redisClient.IsEnabled() {
		// For in-memory fallback, remove the specific limiters
		rl.fallbackMutex.Lock()
		defer rl.fallbackMutex.Unlock()
		
		// Remove user week key
		weekKey := fmt.Sprintf("ratelimit:user:%s:week", userID)
		delete(rl.fallbackLimiters, weekKey)
		
		slog.Info("Invalidated user rate limits (in-memory)", "user_id", userID[:8]+"...")
		return nil
	}

	// Delete all keys matching the user pattern
	pattern := fmt.Sprintf("ratelimit:user:%s:*", userID)
	return rl.deleteByPattern(ctx, pattern)
}

// InvalidateIP removes all rate limit keys for a specific IP address
// Used for manual IP ban/unban or limit resets
func (rl *RateLimiter) InvalidateIP(ctx context.Context, ip string) error {
	if !rl.redisClient.IsEnabled() {
		// For in-memory fallback, remove the specific limiter
		rl.fallbackMutex.Lock()
		defer rl.fallbackMutex.Unlock()
		
		ipKey := fmt.Sprintf("ratelimit:ip:%s", ip)
		delete(rl.fallbackLimiters, ipKey)
		
		slog.Info("Invalidated IP rate limits (in-memory)", "ip", ip)
		return nil
	}

	// Delete all keys matching the IP pattern
	pattern := fmt.Sprintf("ratelimit:ip:%s*", ip)
	return rl.deleteByPattern(ctx, pattern)
}

// ResetOnUpgrade immediately resets limits when a user upgrades to paid
func (rl *RateLimiter) ResetOnUpgrade(ctx context.Context, userID string) error {
	slog.Info("Resetting rate limits for user upgrade", "user_id", userID[:8]+"...")
	return rl.InvalidateUser(ctx, userID)
}

// BumpVersion forces all clients to restart their limit window
// Used for emergency rate limit policy changes
func (rl *RateLimiter) BumpVersion(ctx context.Context, scope string) error {
	if !rl.redisClient.IsEnabled() {
		slog.Warn("Version bumping not available in fallback mode", "scope", scope)
		return fmt.Errorf("version bumping requires Redis")
	}

	versionKey := fmt.Sprintf("ratelimit:version:%s", scope)
	result := rl.redisClient.GetClient().Incr(ctx, versionKey)
	if result.Err() != nil {
		return fmt.Errorf("failed to bump version: %w", result.Err())
	}

	newVersion := result.Val()
	slog.Info("Bumped rate limit version", "scope", scope, "version", newVersion)
	return nil
}

// GetVersion returns the current version for a scope
func (rl *RateLimiter) GetVersion(ctx context.Context, scope string) (int64, error) {
	if !rl.redisClient.IsEnabled() {
		return 0, fmt.Errorf("version tracking requires Redis")
	}

	versionKey := fmt.Sprintf("ratelimit:version:%s", scope)
	result := rl.redisClient.GetClient().Get(ctx, versionKey)
	
	if result.Err() == redis.Nil {
		// Key doesn't exist, version is 0
		return 0, nil
	}
	
	if result.Err() != nil {
		return 0, fmt.Errorf("failed to get version: %w", result.Err())
	}

	version, err := result.Int64()
	if err != nil {
		return 0, fmt.Errorf("failed to parse version: %w", err)
	}

	return version, nil
}

// deleteByPattern deletes all Redis keys matching a pattern
func (rl *RateLimiter) deleteByPattern(ctx context.Context, pattern string) error {
	client := rl.redisClient.GetClient()

	// Use SCAN to find matching keys (more efficient than KEYS)
	var cursor uint64
	var deletedCount int

	for {
		keys, nextCursor, err := client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan keys: %w", err)
		}

		// Delete found keys
		if len(keys) > 0 {
			deleted, err := client.Del(ctx, keys...).Result()
			if err != nil {
				return fmt.Errorf("failed to delete keys: %w", err)
			}
			deletedCount += int(deleted)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	slog.Info("Deleted rate limit keys by pattern", "pattern", pattern, "count", deletedCount)
	return nil
}

// InvalidateAll removes all rate limit keys (emergency use only)
func (rl *RateLimiter) InvalidateAll(ctx context.Context) error {
	if !rl.redisClient.IsEnabled() {
		// For in-memory fallback, clear everything
		rl.fallbackMutex.Lock()
		defer rl.fallbackMutex.Unlock()
		
		count := len(rl.fallbackLimiters)
		rl.fallbackLimiters = make(map[string]*rate.Limiter)
		
		slog.Warn("Invalidated all rate limits (in-memory)", "count", count)
		return nil
	}

	// Delete all rate limit keys
	pattern := "ratelimit:*"
	slog.Warn("Invalidating ALL rate limits", "pattern", pattern)
	return rl.deleteByPattern(ctx, pattern)
}

// CleanupExpired removes expired keys (Redis handles this automatically via TTL)
// This is a no-op for Redis but provides consistency with the interface
func (rl *RateLimiter) CleanupExpired(ctx context.Context) error {
	if !rl.redisClient.IsEnabled() {
		// For in-memory, we rely on the periodic cleanup goroutine
		slog.Debug("Cleanup triggered (handled by periodic goroutine)")
		return nil
	}

	// Redis handles expiration automatically via TTL
	slog.Debug("Cleanup triggered (Redis handles TTL automatically)")
	return nil
}


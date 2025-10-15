package ratelimit

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// InvalidateUser removes all rate limit keys for a specific user
// This is useful when a user upgrades to paid tier or when resetting their limits
func (rl *RateLimiter) InvalidateUser(ctx context.Context, userID string) error {
	if !rl.redisClient.IsEnabled() {
		// For in-memory, remove the user's limiter
		rl.fallbackMutex.Lock()
		defer rl.fallbackMutex.Unlock()

		pattern := fmt.Sprintf("ratelimit:user:%s:", userID)
		for key := range rl.fallbackLimiters {
			if len(key) >= len(pattern) && key[:len(pattern)] == pattern {
				delete(rl.fallbackLimiters, key)
			}
		}

		slog.Info("User rate limit invalidated (in-memory)", "user_id", userID)
		return nil
	}

	// For Redis, delete all keys matching the user pattern
	pattern := fmt.Sprintf("ratelimit:user:%s:*", userID)
	err := rl.deleteByPattern(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to invalidate user rate limits: %w", err)
	}

	slog.Info("User rate limit invalidated (Redis)", "user_id", userID)
	return nil
}

// InvalidateIP removes all rate limit keys for a specific IP address
// This is useful for removing rate limits on trusted IPs or after an IP ban is lifted
func (rl *RateLimiter) InvalidateIP(ctx context.Context, ip string) error {
	if !rl.redisClient.IsEnabled() {
		// For in-memory, remove the IP's limiter
		rl.fallbackMutex.Lock()
		defer rl.fallbackMutex.Unlock()

		pattern := fmt.Sprintf("ratelimit:ip:%s:", ip)
		for key := range rl.fallbackLimiters {
			if len(key) >= len(pattern) && key[:len(pattern)] == pattern {
				delete(rl.fallbackLimiters, key)
			}
		}

		slog.Info("IP rate limit invalidated (in-memory)", "ip", ip)
		return nil
	}

	// For Redis, delete all keys matching the IP pattern
	pattern := fmt.Sprintf("ratelimit:ip:%s:*", ip)
	err := rl.deleteByPattern(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to invalidate IP rate limits: %w", err)
	}

	slog.Info("IP rate limit invalidated (Redis)", "ip", ip)
	return nil
}

// BumpVersion forces all clients to restart their limit window for a given scope
// This is useful for rolling out new rate limit policies
func (rl *RateLimiter) BumpVersion(ctx context.Context, scope string) error {
	if !rl.redisClient.IsEnabled() {
		slog.Warn("Version bumping not supported in in-memory mode", "scope", scope)
		return nil
	}

	versionKey := fmt.Sprintf("ratelimit:version:%s", scope)
	err := rl.redisClient.Client().Incr(ctx, versionKey).Err()
	if err != nil {
		return fmt.Errorf("failed to bump version: %w", err)
	}

	slog.Info("Rate limit version bumped", "scope", scope)
	return nil
}

// ResetOnUpgrade immediately resets rate limits when a user upgrades to paid tier
// This removes the weekly limit key so they can start using unlimited access immediately
func (rl *RateLimiter) ResetOnUpgrade(ctx context.Context, userID string) error {
	if !rl.redisClient.IsEnabled() {
		// For in-memory, remove the user's week limiter
		rl.fallbackMutex.Lock()
		defer rl.fallbackMutex.Unlock()

		weekKey := fmt.Sprintf("ratelimit:user:%s:week", userID)
		delete(rl.fallbackLimiters, weekKey)

		slog.Info("User rate limit reset on upgrade (in-memory)", "user_id", userID)
		return nil
	}

	// For Redis, delete the weekly limit key
	weekKey := fmt.Sprintf("ratelimit:user:%s:week", userID)
	err := rl.redisClient.Client().Del(ctx, weekKey).Err()
	if err != nil {
		return fmt.Errorf("failed to reset user rate limit on upgrade: %w", err)
	}

	slog.Info("User rate limit reset on upgrade (Redis)", "user_id", userID, "key", weekKey)
	return nil
}

// InvalidateAll removes all rate limit keys (nuclear option for debugging/testing)
// Use with extreme caution in production
func (rl *RateLimiter) InvalidateAll(ctx context.Context) error {
	if !rl.redisClient.IsEnabled() {
		// For in-memory, clear all limiters
		rl.fallbackMutex.Lock()
		defer rl.fallbackMutex.Unlock()

		count := len(rl.fallbackLimiters)
		rl.fallbackLimiters = make(map[string]*rate.Limiter)

		slog.Warn("All rate limits invalidated (in-memory)", "count", count)
		return nil
	}

	// For Redis, delete all ratelimit keys
	pattern := "ratelimit:*"
	err := rl.deleteByPattern(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to invalidate all rate limits: %w", err)
	}

	slog.Warn("All rate limits invalidated (Redis)")
	return nil
}

// deleteByPattern deletes all Redis keys matching a pattern using SCAN for safety
func (rl *RateLimiter) deleteByPattern(ctx context.Context, pattern string) error {
	if !rl.redisClient.IsEnabled() {
		return fmt.Errorf("redis not enabled")
	}

	client := rl.redisClient.Client()
	var cursor uint64
	var deletedCount int

	// Use SCAN to iterate through matching keys safely (doesn't block Redis)
	for {
		var keys []string
		var err error

		// Scan for keys matching the pattern
		keys, cursor, err = client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		// Delete matching keys
		if len(keys) > 0 {
			pipe := client.Pipeline()
			for _, key := range keys {
				pipe.Del(ctx, key)
			}
			_, err = pipe.Exec(ctx)
			if err != nil && err != redis.Nil {
				return fmt.Errorf("delete failed: %w", err)
			}
			deletedCount += len(keys)
		}

		// Check if we've scanned all keys
		if cursor == 0 {
			break
		}
	}

	slog.Info("Redis keys deleted", "pattern", pattern, "count", deletedCount)
	return nil
}

// GetKeyCount returns the number of rate limit keys in Redis
func (rl *RateLimiter) GetKeyCount(ctx context.Context) (int, error) {
	if !rl.redisClient.IsEnabled() {
		rl.fallbackMutex.RLock()
		defer rl.fallbackMutex.RUnlock()
		return len(rl.fallbackLimiters), nil
	}

	client := rl.redisClient.Client()
	var cursor uint64
	var count int

	// Use SCAN to count all ratelimit keys
	for {
		var keys []string
		var err error

		keys, cursor, err = client.Scan(ctx, cursor, "ratelimit:*", 100).Result()
		if err != nil {
			return 0, fmt.Errorf("scan failed: %w", err)
		}

		count += len(keys)

		if cursor == 0 {
			break
		}
	}

	return count, nil
}

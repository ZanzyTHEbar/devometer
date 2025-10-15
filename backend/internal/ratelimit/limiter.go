package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/monitoring"
	"golang.org/x/time/rate"
)

// Config holds rate limiter configuration
type Config struct {
	IPLimitPerMin   int // IP-based rate limit per minute
	UserLimitPerWeek int // User-based rate limit per week
	BurstMultiplier int // Burst capacity multiplier
}

// DefaultConfig returns default rate limiting configuration
func DefaultConfig() Config {
	return Config{
		IPLimitPerMin:   60,
		UserLimitPerWeek: 5,
		BurstMultiplier: 2,
	}
}

// Result represents the result of a rate limit check
type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	ResetAt    time.Time
	RetryAfter time.Duration
}

// RateLimiter provides distributed rate limiting with Redis and in-memory fallback
type RateLimiter struct {
	redisLimiter  *redis_rate.Limiter
	redisClient   *RedisClient
	config        Config
	metrics       *monitoring.Metrics
	
	// In-memory fallback rate limiters
	fallbackLimiters map[string]*rate.Limiter
	fallbackMutex    sync.RWMutex
}

// NewRateLimiter creates a new rate limiter with Redis and in-memory fallback
func NewRateLimiter(redisClient *RedisClient, config Config, metrics *monitoring.Metrics) *RateLimiter {
	rl := &RateLimiter{
		redisClient:      redisClient,
		config:           config,
		metrics:          metrics,
		fallbackLimiters: make(map[string]*rate.Limiter),
	}

	// Initialize Redis rate limiter if enabled
	if redisClient.IsEnabled() {
		rl.redisLimiter = redis_rate.NewLimiter(redisClient.GetClient())
		slog.Info("Redis rate limiter initialized")
	} else {
		slog.Warn("Redis unavailable, using in-memory rate limiting only")
	}

	// Start cleanup goroutine for fallback limiters
	go rl.cleanupFallbackLimiters()

	return rl
}

// AllowIP checks if an IP address is allowed to make a request (per-minute limit)
func (rl *RateLimiter) AllowIP(ctx context.Context, ip string) (*Result, error) {
	key := fmt.Sprintf("ratelimit:ip:%s", ip)
	limit := rl.config.IPLimitPerMin
	period := time.Minute

	return rl.allow(ctx, key, limit, period)
}

// AllowUser checks if a user is allowed to make a request (per-week limit)
func (rl *RateLimiter) AllowUser(ctx context.Context, userID string) (*Result, error) {
	key := fmt.Sprintf("ratelimit:user:%s:week", userID)
	limit := rl.config.UserLimitPerWeek
	period := 7 * 24 * time.Hour // 1 week

	return rl.allow(ctx, key, limit, period)
}

// allow performs the actual rate limit check using Redis or fallback
func (rl *RateLimiter) allow(ctx context.Context, key string, limit int, period time.Duration) (*Result, error) {
	// Try Redis first if enabled
	if rl.redisClient.IsEnabled() && rl.redisLimiter != nil {
		result, err := rl.allowRedis(ctx, key, limit, period)
		if err != nil {
			// Redis error - fall back to in-memory
			slog.Warn("Redis rate limit check failed, using fallback", "key", key, "error", err)
			if rl.metrics != nil {
				rl.metrics.IncrementRateLimitRedisError()
			}
			return rl.allowFallback(key, limit, period)
		}
		return result, nil
	}

	// Use in-memory fallback
	if rl.metrics != nil {
		rl.metrics.IncrementRateLimitFallback()
	}
	return rl.allowFallback(key, limit, period)
}

// allowRedis performs rate limiting using Redis sliding window
func (rl *RateLimiter) allowRedis(ctx context.Context, key string, limit int, period time.Duration) (*Result, error) {
	// Create rate limit configuration
	rateLimit := redis_rate.Limit{
		Rate:   limit,
		Burst:  limit,
		Period: period,
	}

	// Use Redis sliding window counter
	res, err := rl.redisLimiter.Allow(ctx, key, rateLimit)
	if err != nil {
		return nil, fmt.Errorf("redis rate limit check failed: %w", err)
	}

	// Check if allowed (Allowed > 0 means we can proceed)
	allowed := res.Allowed > 0

	result := &Result{
		Allowed:    allowed,
		Limit:      res.Limit.Rate,
		Remaining:  res.Remaining,
		ResetAt:    time.Now().Add(res.ResetAfter),
		RetryAfter: res.RetryAfter,
	}

	return result, nil
}

// allowFallback performs rate limiting using in-memory token bucket
func (rl *RateLimiter) allowFallback(key string, limit int, period time.Duration) (*Result, error) {
	rl.fallbackMutex.Lock()
	limiter, exists := rl.fallbackLimiters[key]
	if !exists {
		// Create new limiter with token bucket algorithm
		rps := rate.Limit(float64(limit) / period.Seconds())
		burst := limit * rl.config.BurstMultiplier
		if burst < 5 {
			burst = 5
		}
		limiter = rate.NewLimiter(rps, burst)
		rl.fallbackLimiters[key] = limiter
	}
	rl.fallbackMutex.Unlock()

	// Check if request is allowed
	allowed := limiter.Allow()

	// Calculate remaining tokens (approximate)
	reservation := limiter.Reserve()
	if !reservation.OK() {
		reservation.Cancel()
		return &Result{
			Allowed:    false,
			Limit:      limit,
			Remaining:  0,
			ResetAt:    time.Now().Add(period),
			RetryAfter: period,
		}, nil
	}
	reservation.Cancel()

	tokens := limiter.Tokens()
	remaining := int(tokens)
	if remaining < 0 {
		remaining = 0
	}

	result := &Result{
		Allowed:    allowed,
		Limit:      limit,
		Remaining:  remaining,
		ResetAt:    time.Now().Add(period),
		RetryAfter: 0,
	}

	if !allowed {
		result.RetryAfter = time.Until(result.ResetAt)
	}

	return result, nil
}

// cleanupFallbackLimiters periodically removes old fallback limiters
func (rl *RateLimiter) cleanupFallbackLimiters() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		rl.fallbackMutex.Lock()
		// Simple cleanup: clear all limiters periodically
		// More sophisticated: track last access time per limiter
		if len(rl.fallbackLimiters) > 1000 {
			slog.Info("Cleaning up fallback rate limiters", "count", len(rl.fallbackLimiters))
			rl.fallbackLimiters = make(map[string]*rate.Limiter)
		}
		rl.fallbackMutex.Unlock()
	}
}

// GetStats returns rate limiter statistics
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.fallbackMutex.RLock()
	fallbackCount := len(rl.fallbackLimiters)
	rl.fallbackMutex.RUnlock()

	stats := map[string]interface{}{
		"redis_enabled":     rl.redisClient.IsEnabled(),
		"fallback_limiters": fallbackCount,
	}

	// Add Redis pool stats if available
	if rl.redisClient.IsEnabled() {
		stats["redis_pool"] = rl.redisClient.GetPoolStats()
	}

	return stats
}


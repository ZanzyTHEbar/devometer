package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/monitoring"
	"github.com/go-redis/redis_rate/v10"
	"golang.org/x/time/rate"
)

// Config holds rate limiter configuration
type Config struct {
	IPLimit         int           // Requests per minute per IP
	UserLimit       int           // Requests per week per user
	BurstMultiplier int           // Burst capacity multiplier
	EnableFallback  bool          // Enable in-memory fallback
	CleanupInterval time.Duration // Cleanup interval for in-memory limiters
}

// DefaultConfig returns sensible default configuration
func DefaultConfig() Config {
	return Config{
		IPLimit:         60,
		UserLimit:       5,
		BurstMultiplier: 2,
		EnableFallback:  true,
		CleanupInterval: 1 * time.Hour,
	}
}

// Rate represents a rate limit configuration
type Rate struct {
	Limit  int           // Number of requests
	Period time.Duration // Time period
}

// Result contains rate limit check results
type Result struct {
	Allowed    bool          // Whether request is allowed
	Limit      int           // Rate limit
	Remaining  int           // Remaining requests
	ResetAt    time.Time     // When the limit resets
	RetryAfter time.Duration // How long to wait before retrying
}

// RateLimiter implements distributed rate limiting with Redis and in-memory fallback
type RateLimiter struct {
	redisLimiter *redis_rate.Limiter
	redisClient  *RedisClient
	config       Config
	metrics      *monitoring.Metrics

	// In-memory fallback
	fallbackLimiters map[string]*rate.Limiter
	fallbackMutex    sync.RWMutex
	lastCleanup      time.Time
}

// NewRateLimiter creates a new distributed rate limiter
func NewRateLimiter(redisClient *RedisClient, config Config, metrics *monitoring.Metrics) *RateLimiter {
	rl := &RateLimiter{
		redisClient:      redisClient,
		config:           config,
		metrics:          metrics,
		fallbackLimiters: make(map[string]*rate.Limiter),
		lastCleanup:      time.Now(),
	}

	// Initialize Redis limiter if available
	if redisClient.IsEnabled() {
		rl.redisLimiter = redis_rate.NewLimiter(redisClient.Client())
		slog.Info("Rate limiter initialized with Redis backend")
	} else {
		slog.Warn("Rate limiter initialized with in-memory fallback only")
	}

	// Start cleanup goroutine for in-memory limiters
	if config.EnableFallback {
		go rl.cleanupLoop()
	}

	return rl
}

// Allow checks if a request is allowed based on the rate limit
func (rl *RateLimiter) Allow(ctx context.Context, key string, rateLimit Rate) (*Result, error) {
	// Try Redis first if available
	if rl.redisClient.IsEnabled() && rl.redisLimiter != nil {
		result, err := rl.allowRedis(ctx, key, rateLimit)
		if err == nil {
			return result, nil
		}

		// Log Redis error and fall back
		slog.Error("Redis rate limit check failed, falling back to in-memory",
			"error", err,
			"key", key)

		if rl.metrics != nil {
			// Track Redis errors
			rl.metrics.IncrementRateLimitRedisError()
		}
	}

	// Use in-memory fallback
	if rl.config.EnableFallback {
		return rl.allowFallback(key, rateLimit), nil
	}

	return nil, fmt.Errorf("rate limiting unavailable")
}

// allowRedis checks rate limit using Redis sliding window algorithm
func (rl *RateLimiter) allowRedis(ctx context.Context, key string, rateLimit Rate) (*Result, error) {
	// Use redis_rate's Allow which implements sliding window counter
	redisLimit := redis_rate.Limit{
		Rate:   rateLimit.Limit,
		Burst:  rateLimit.Limit * rl.config.BurstMultiplier,
		Period: rateLimit.Period,
	}

	res, err := rl.redisLimiter.Allow(ctx, key, redisLimit)
	if err != nil {
		return nil, fmt.Errorf("redis rate limit check failed: %w", err)
	}

	// Calculate retry after duration
	var retryAfter time.Duration
	allowed := res.Allowed > 0
	if !allowed {
		retryAfter = res.RetryAfter
	}

	// Calculate reset time from RetryAfter
	resetAt := time.Now().Add(res.RetryAfter)

	result := &Result{
		Allowed:    allowed,
		Limit:      res.Limit.Rate,
		Remaining:  res.Remaining,
		ResetAt:    resetAt,
		RetryAfter: retryAfter,
	}

	return result, nil
}

// allowFallback checks rate limit using in-memory token bucket algorithm
func (rl *RateLimiter) allowFallback(key string, rateLimit Rate) *Result {
	rl.fallbackMutex.Lock()

	// Get or create limiter for this key
	limiter, exists := rl.fallbackLimiters[key]
	if !exists {
		rps := rate.Limit(float64(rateLimit.Limit) / rateLimit.Period.Seconds())
		burst := rateLimit.Limit * rl.config.BurstMultiplier
		if burst < 5 {
			burst = 5
		}
		limiter = rate.NewLimiter(rps, burst)
		rl.fallbackLimiters[key] = limiter
	}

	rl.fallbackMutex.Unlock()

	// Check if request is allowed
	allowed := limiter.Allow()

	// Calculate remaining tokens
	reservation := limiter.Reserve()
	remaining := limiter.Burst() - int(reservation.Delay().Seconds()*float64(rateLimit.Limit)/rateLimit.Period.Seconds())
	if remaining < 0 {
		remaining = 0
	}
	reservation.Cancel() // Cancel the reservation

	var retryAfter time.Duration
	if !allowed {
		// Calculate when next token will be available
		r := limiter.Reserve()
		retryAfter = r.Delay()
		r.Cancel()
	}

	// Track fallback usage
	if rl.metrics != nil {
		rl.metrics.IncrementRateLimitFallback()
	}

	return &Result{
		Allowed:    allowed,
		Limit:      rateLimit.Limit,
		Remaining:  remaining,
		ResetAt:    time.Now().Add(rateLimit.Period),
		RetryAfter: retryAfter,
	}
}

// cleanupLoop periodically removes old in-memory rate limiters
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes rate limiters that haven't been used recently
func (rl *RateLimiter) cleanup() {
	if time.Since(rl.lastCleanup) < rl.config.CleanupInterval {
		return
	}

	rl.fallbackMutex.Lock()
	defer rl.fallbackMutex.Unlock()

	// In a production system, track last access time
	// For now, clear all if we have too many
	if len(rl.fallbackLimiters) > 10000 {
		slog.Info("Cleaning up in-memory rate limiters", "count", len(rl.fallbackLimiters))
		rl.fallbackLimiters = make(map[string]*rate.Limiter)
	}

	rl.lastCleanup = time.Now()
}

// Close closes the rate limiter and Redis connection
func (rl *RateLimiter) Close() error {
	if rl.redisClient != nil {
		return rl.redisClient.Close()
	}
	return nil
}

// GetStats returns rate limiter statistics
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.fallbackMutex.RLock()
	fallbackCount := len(rl.fallbackLimiters)
	rl.fallbackMutex.RUnlock()

	stats := map[string]interface{}{
		"redis_enabled":     rl.redisClient.IsEnabled(),
		"fallback_enabled":  rl.config.EnableFallback,
		"fallback_limiters": fallbackCount,
		"config": map[string]interface{}{
			"ip_limit_per_min":    rl.config.IPLimit,
			"user_limit_per_week": rl.config.UserLimit,
			"burst_multiplier":    rl.config.BurstMultiplier,
		},
	}

	// Add Redis pool stats if available
	if rl.redisClient.IsEnabled() {
		stats["redis_pool"] = rl.redisClient.GetPoolStats()
	}

	return stats
}

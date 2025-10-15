package ratelimit

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// IPRateLimitMiddleware creates middleware for per-IP rate limiting
func (rl *RateLimiter) IPRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ip := c.ClientIP()

		// Create rate limit key for IP
		key := fmt.Sprintf("ratelimit:ip:%s", ip)

		// Define rate limit (60 requests per minute)
		limit := Rate{
			Limit:  rl.config.IPLimit,
			Period: time.Minute,
		}

		// Check rate limit
		result, err := rl.Allow(ctx, key, limit)
		if err != nil {
			// On error, log but don't block (fail open for better availability)
			c.Next()
			return
		}

		// Inject standard rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(result.Limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		// If rate limit exceeded, return 429
		if !result.Allowed {
			// Track rate limit block
			if rl.metrics != nil {
				rl.metrics.IncrementRateLimitIPBlock()
			}

			c.Header("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"message":     fmt.Sprintf("Too many requests from IP %s", ip),
				"retry_after": result.RetryAfter.Seconds(),
				"limit":       result.Limit,
				"period":      "1 minute",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// UserRateLimitMiddleware creates middleware for per-user rate limiting
// This is applied to specific endpoints that require user tracking
func (rl *RateLimiter) UserRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only apply to analyze endpoints
		if c.Request.URL.Path != "/analyze" && c.Request.URL.Path != "/api/analyze" {
			c.Next()
			return
		}

		ctx := c.Request.Context()

		// Get user ID from context (set by auth middleware or user tracking)
		userID, exists := c.Get("user_id")
		if !exists {
			// If no user ID, skip user rate limiting (rely on IP limiting)
			c.Next()
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			c.Next()
			return
		}

		// Check if user is paid (skip rate limiting for paid users)
		userStats, exists := c.Get("user_stats")
		if exists {
			if stats, ok := userStats.(map[string]interface{}); ok {
				if isPaid, ok := stats["is_paid"].(bool); ok && isPaid {
					// Paid user - no rate limit
					c.Next()
					return
				}
			}
		}

		// Create rate limit key for user
		key := fmt.Sprintf("ratelimit:user:%s:week", userIDStr)

		// Define rate limit (5 requests per week for free users)
		limit := Rate{
			Limit:  rl.config.UserLimit,
			Period: 7 * 24 * time.Hour, // 1 week
		}

		// Check rate limit
		result, err := rl.Allow(ctx, key, limit)
		if err != nil {
			// On error, log but don't block
			c.Next()
			return
		}

		// Inject rate limit headers
		c.Header("X-RateLimit-User-Limit", strconv.Itoa(result.Limit))
		c.Header("X-RateLimit-User-Remaining", strconv.Itoa(result.Remaining))
		c.Header("X-RateLimit-User-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		// If rate limit exceeded, return 429
		if !result.Allowed {
			// Track rate limit block
			if rl.metrics != nil {
				rl.metrics.IncrementRateLimitUserBlock()
			}

			c.Header("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":              "weekly request limit exceeded",
				"message":            "You've used all 5 free requests this week",
				"remaining_requests": result.Remaining,
				"retry_after":        result.RetryAfter.Seconds(),
				"limit":              result.Limit,
				"period":             "1 week",
				"reset_at":           result.ResetAt.Format(time.RFC3339),
				"upgrade_url":        "/upgrade",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// EndpointRateLimitMiddleware creates middleware for per-endpoint rate limiting
// This allows different rate limits for different endpoints
func (rl *RateLimiter) EndpointRateLimitMiddleware(endpoint string, limit Rate) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ip := c.ClientIP()

		// Create rate limit key for endpoint + IP
		key := fmt.Sprintf("ratelimit:endpoint:%s:ip:%s", endpoint, ip)

		// Check rate limit
		result, err := rl.Allow(ctx, key, limit)
		if err != nil {
			// On error, log but don't block
			c.Next()
			return
		}

		// Inject rate limit headers
		c.Header("X-RateLimit-Endpoint-Limit", strconv.Itoa(result.Limit))
		c.Header("X-RateLimit-Endpoint-Remaining", strconv.Itoa(result.Remaining))
		c.Header("X-RateLimit-Endpoint-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		// If rate limit exceeded, return 429
		if !result.Allowed {
			// Track rate limit block
			if rl.metrics != nil {
				rl.metrics.IncrementRateLimitEndpoint(endpoint)
			}

			c.Header("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "endpoint rate limit exceeded",
				"message":     fmt.Sprintf("Too many requests to %s", endpoint),
				"retry_after": result.RetryAfter.Seconds(),
				"limit":       result.Limit,
				"endpoint":    endpoint,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GlobalRateLimitMiddleware creates middleware for global rate limiting
// This is useful for protecting against distributed attacks
func (rl *RateLimiter) GlobalRateLimitMiddleware(limit Rate) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Use a global key
		key := "ratelimit:global"

		// Check rate limit
		result, err := rl.Allow(ctx, key, limit)
		if err != nil {
			// On error, log but don't block
			c.Next()
			return
		}

		// Inject rate limit headers
		c.Header("X-RateLimit-Global-Limit", strconv.Itoa(result.Limit))
		c.Header("X-RateLimit-Global-Remaining", strconv.Itoa(result.Remaining))

		// If rate limit exceeded, return 503 (Service Unavailable)
		if !result.Allowed {
			c.Header("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":       "service temporarily unavailable",
				"message":     "Server is experiencing high load, please try again later",
				"retry_after": result.RetryAfter.Seconds(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

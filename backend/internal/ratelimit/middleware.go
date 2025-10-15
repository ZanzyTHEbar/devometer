package ratelimit

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/database"
	"github.com/gin-gonic/gin"
)

// IPRateLimitMiddleware creates middleware for IP-based rate limiting
func (rl *RateLimiter) IPRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ip := c.ClientIP()

		// Check rate limit
		result, err := rl.AllowIP(ctx, ip)
		if err != nil {
			// Log error but don't block request on rate limiter failure
			slog.Error("Rate limit check failed", "ip", ip, "error", err)
			c.Next()
			return
		}

		// Inject standard rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(result.Limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		// Check if request is allowed
		if !result.Allowed {
			// Increment metrics
			if rl.metrics != nil {
				rl.metrics.IncrementRateLimitIPBlock()
			}

			// Add Retry-After header
			c.Header("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))

			// Return 429 Too Many Requests
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded for IP",
				"message":     fmt.Sprintf("You have exceeded the rate limit of %d requests per minute", result.Limit),
				"retry_after": int(result.RetryAfter.Seconds()),
				"reset_at":    result.ResetAt.Unix(),
			})
			c.Abort()
			return
		}

		// Request allowed, continue
		c.Next()
	}
}

// UserRateLimitMiddleware creates middleware for user-based rate limiting
func (rl *RateLimiter) UserRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only apply to analyze endpoints
		if c.Request.URL.Path != "/analyze" && c.Request.URL.Path != "/api/analyze" {
			c.Next()
			return
		}

		// Get user ID from context (set by security middleware)
		userID, exists := c.Get("user_id")
		if !exists {
			// No user ID, skip user rate limiting
			c.Next()
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			slog.Warn("Invalid user ID type in context")
			c.Next()
			return
		}

		ctx := c.Request.Context()

		// Check if user is paid (paid users bypass weekly limits)
		isPaid := false
		if userStats, exists := c.Get("user_stats"); exists {
			if stats, ok := userStats.(*database.UsageStats); ok {
				isPaid = stats.IsPaid
			}
		}

		if isPaid {
			// Paid users have unlimited access
			c.Header("X-RateLimit-User-Limit", "unlimited")
			c.Header("X-RateLimit-User-Remaining", "unlimited")
			c.Next()
			return
		}

		// Check user rate limit
		result, err := rl.AllowUser(ctx, userIDStr)
		if err != nil {
			// Log error but don't block request on rate limiter failure
			slog.Error("User rate limit check failed", "user_id", userIDStr[:8]+"...", "error", err)
			c.Next()
			return
		}

		// Inject user-specific rate limit headers
		c.Header("X-RateLimit-User-Limit", strconv.Itoa(result.Limit))
		c.Header("X-RateLimit-User-Remaining", strconv.Itoa(result.Remaining))
		c.Header("X-RateLimit-User-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		// Check if request is allowed
		if !result.Allowed {
			// Increment metrics
			if rl.metrics != nil {
				rl.metrics.IncrementRateLimitUserBlock()
			}

			// Add Retry-After header
			c.Header("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))

			// Return 429 with upgrade information
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":              "weekly request limit exceeded",
				"message":            fmt.Sprintf("You have used all %d free requests this week", result.Limit),
				"remaining_requests": result.Remaining,
				"reset_at":           result.ResetAt.Unix(),
				"retry_after":        int(result.RetryAfter.Seconds()),
				"upgrade_url":        "/upgrade",
				"upgrade_message":    "Upgrade to unlimited access",
			})
			c.Abort()
			return
		}

		// Request allowed, continue
		c.Next()
	}
}

// EndpointRateLimitMiddleware creates middleware for endpoint-specific rate limiting
func (rl *RateLimiter) EndpointRateLimitMiddleware(endpoint string, limit int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ip := c.ClientIP()

		// Create endpoint-specific key
		key := fmt.Sprintf("ratelimit:endpoint:%s:%s", endpoint, ip)

		// Use custom limit for this endpoint
		result, err := rl.allow(ctx, key, limit, 60*time.Second) // Per minute
		if err != nil {
			slog.Error("Endpoint rate limit check failed", "endpoint", endpoint, "ip", ip, "error", err)
			c.Next()
			return
		}

		// Inject headers
		c.Header("X-RateLimit-Endpoint-Limit", strconv.Itoa(result.Limit))
		c.Header("X-RateLimit-Endpoint-Remaining", strconv.Itoa(result.Remaining))

		if !result.Allowed {
			// Increment metrics
			if rl.metrics != nil {
				rl.metrics.IncrementRateLimitEndpoint(endpoint)
			}

			c.Header("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       fmt.Sprintf("rate limit exceeded for endpoint: %s", endpoint),
				"message":     fmt.Sprintf("You have exceeded the rate limit of %d requests per minute for this endpoint", result.Limit),
				"retry_after": int(result.RetryAfter.Seconds()),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}


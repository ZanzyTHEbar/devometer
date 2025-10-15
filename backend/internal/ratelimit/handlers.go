package ratelimit

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HandleRateLimitStatus returns the current rate limit status for the requesting IP/user
func (rl *RateLimiter) HandleRateLimitStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		status := gin.H{
			"ip": ip,
			"limits": gin.H{
				"ip_per_minute": gin.H{
					"limit":  rl.config.IPLimit,
					"period": "1 minute",
				},
				"user_per_week": gin.H{
					"limit":  rl.config.UserLimit,
					"period": "1 week",
				},
			},
			"timestamp": time.Now().Format(time.RFC3339),
		}

		// Add user-specific info if available
		if userID, exists := c.Get("user_id"); exists {
			if userIDStr, ok := userID.(string); ok {
				status["user_id"] = userIDStr

				// Check if user is paid
				if userStats, exists := c.Get("user_stats"); exists {
					if stats, ok := userStats.(map[string]interface{}); ok {
						status["is_paid"] = stats["is_paid"]
						if isPaid, ok := stats["is_paid"].(bool); ok && isPaid {
							status["limits"].(gin.H)["user_per_week"] = "unlimited"
						}
					}
				}
			}
		}

		c.JSON(http.StatusOK, status)
	}
}

// HandleAdminRateLimits returns comprehensive rate limit information (admin only)
func (rl *RateLimiter) HandleAdminRateLimits() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Get key count
		keyCount, err := rl.GetKeyCount(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to get key count",
			})
			return
		}

		// Get stats
		stats := rl.GetStats()

		// Get metrics if available
		var rateLimitMetrics map[string]interface{}
		if rl.metrics != nil {
			rateLimitMetrics = rl.metrics.GetRateLimitStats()
		}

		response := gin.H{
			"total_keys":    keyCount,
			"limiter_stats": stats,
			"metrics":       rateLimitMetrics,
			"timestamp":     time.Now().Format(time.RFC3339),
		}

		c.JSON(http.StatusOK, response)
	}
}

// HandleAdminResetRateLimit resets rate limits for a specific user (admin only)
func (rl *RateLimiter) HandleAdminResetRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID := c.Param("userID")

		if userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "user ID is required",
			})
			return
		}

		// Reset user rate limit
		err := rl.ResetOnUpgrade(ctx, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "failed to reset rate limit",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":   "rate limit reset successfully",
			"user_id":   userID,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// HandleAdminInvalidateUser invalidates all rate limits for a user (admin only)
func (rl *RateLimiter) HandleAdminInvalidateUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID := c.Param("userID")

		if userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "user ID is required",
			})
			return
		}

		err := rl.InvalidateUser(ctx, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "failed to invalidate user rate limits",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":   "user rate limits invalidated successfully",
			"user_id":   userID,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// HandleAdminInvalidateIP invalidates all rate limits for an IP (admin only)
func (rl *RateLimiter) HandleAdminInvalidateIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ip := c.Param("ip")

		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "IP address is required",
			})
			return
		}

		err := rl.InvalidateIP(ctx, ip)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "failed to invalidate IP rate limits",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":   "IP rate limits invalidated successfully",
			"ip":        ip,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// HandleAdminRateLimitMetrics returns detailed rate limiting metrics (admin only)
func (rl *RateLimiter) HandleAdminRateLimitMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl.metrics == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "metrics not configured",
			})
			return
		}

		metrics := rl.metrics.GetRateLimitStats()

		c.JSON(http.StatusOK, gin.H{
			"rate_limit_metrics": metrics,
			"timestamp":          time.Now().Format(time.RFC3339),
		})
	}
}

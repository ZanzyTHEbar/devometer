package monitoring

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// MonitoringMiddleware creates Gin middleware for request monitoring
func MonitoringMiddleware(metrics *Metrics, logger *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Increment request count
		metrics.IncrementRequest()

		// Get client information
		ip := c.ClientIP()
		userAgent := c.GetHeader("User-Agent")
		method := c.Request.Method
		path := c.Request.URL.Path

		// Process request
		c.Next()

		// Calculate response time
		duration := time.Since(start)
		statusCode := c.Writer.Status()

		// Record enhanced metrics
		metrics.RecordResponseTime(duration)
		metrics.RecordRequestByStatus(statusCode)

		if statusCode >= 400 {
			metrics.IncrementError()
		}

		// Log request details
		logger.RequestLogger(method, path, ip, userAgent, statusCode, duration)

		// Log errors if any
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				logger.APIErrorLogger(err.Err, method, path, ip, statusCode)
			}
		}

		// Log performance warnings for slow requests
		if duration > 5*time.Second {
			logger.PerformanceLogger("slow_request", duration.Seconds(), "seconds")
		}

		// Log high error rates or unusual patterns
		if statusCode >= 500 {
			logger.SystemLogger("high_error_rate_detected", fmt.Sprintf("Status %d for %s %s", statusCode, method, path))
		}
	}
}

// SecurityMonitoringMiddleware monitors for suspicious activity
func SecurityMonitoringMiddleware(logger *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		userAgent := c.GetHeader("User-Agent")
		method := c.Request.Method
		path := c.Request.URL.Path

		// Check for suspicious patterns
		suspicious := false
		details := make(map[string]interface{})

		// Check for SQL injection patterns
		if containsSQLInjectionPatterns(c.Request.URL.RawQuery) {
			suspicious = true
			details["type"] = "potential_sql_injection"
			details["query"] = c.Request.URL.RawQuery
		}

		// Check for unusual request patterns
		if method == "POST" && path == "/analyze" {
			bodySize := c.Request.ContentLength
			if bodySize > 10000 { // 10KB limit
				suspicious = true
				details["type"] = "large_request_body"
				details["size_bytes"] = bodySize
			}
		}

		// Check for rapid requests (would need rate limiting context)
		if containsSuspiciousUserAgent(userAgent) {
			suspicious = true
			details["type"] = "suspicious_user_agent"
			details["user_agent"] = userAgent
		}

		if suspicious {
			logger.SecurityLogger("suspicious_activity_detected", ip, userAgent, details)
		}

		c.Next()
	}
}

// containsSQLInjectionPatterns checks for common SQL injection patterns
func containsSQLInjectionPatterns(query string) bool {
	patterns := []string{
		"UNION SELECT",
		"UNION ALL",
		"SELECT * FROM",
		"DROP TABLE",
		"DELETE FROM",
		"UPDATE users SET",
		"';--",
		"/*",
		"*/",
		" xp_",
		" sp_",
	}

	for _, pattern := range patterns {
		if len(query) > len(pattern) && containsIgnoreCase(query, pattern) {
			return true
		}
	}

	return false
}

// containsSuspiciousUserAgent checks for suspicious user agents
func containsSuspiciousUserAgent(userAgent string) bool {
	suspiciousAgents := []string{
		"sqlmap",
		"nmap",
		"masscan",
		"zmap",
		"dirbuster",
		"gobuster",
		"nikto",
		"acunetix",
		"openvas",
		"rapid7",
		"qualys",
		"nessus",
	}

	for _, agent := range suspiciousAgents {
		if containsIgnoreCase(userAgent, agent) {
			return true
		}
	}

	return false
}

// containsIgnoreCase checks if a string contains a substring (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			containsIgnoreCase(s[1:], substr) ||
			(len(s) > 0 && len(substr) > 0 &&
				toLower(s[0]) == toLower(substr[0]) &&
				containsIgnoreCase(s[1:], substr[1:])))
}

// toLower converts a byte to lowercase
func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

// HealthMonitoringMiddleware provides health check endpoint
func HealthMonitoringMiddleware(metrics *Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "GET" && c.Request.URL.Path == "/health" {
			c.JSON(http.StatusOK, gin.H{
				"status":    "ok",
				"timestamp": time.Now().Format(time.RFC3339),
				"version":   "1.0.0",
				"metrics":   metrics.GetStats(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

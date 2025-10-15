package security

import (
	"os"

	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware adds comprehensive security headers to all responses
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// X-Frame-Options: Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")

		// X-Content-Type-Options: Prevent MIME sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// X-XSS-Protection: Enable browser XSS protection
		c.Header("X-XSS-Protection", "1; mode=block")

		// Referrer-Policy: Control referrer information
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions-Policy: Restrict feature access
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// HSTS: Enforce HTTPS (only in production with HTTPS)
		if os.Getenv("ENABLE_HSTS") == "true" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		c.Next()
	}
}

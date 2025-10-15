package security

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

const nonceKey = "csp-nonce"

// GenerateNonce generates a cryptographically secure random nonce
func GenerateNonce() (string, error) {
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	return base64.StdEncoding.EncodeToString(nonceBytes), nil
}

// CSPMiddleware creates a middleware that generates CSP nonces and sets CSP headers
func CSPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate nonce
		nonce, err := GenerateNonce()
		if err != nil {
			c.AbortWithStatusJSON(500, gin.H{"error": "internal server error"})
			return
		}

		// Store nonce in context for template access
		c.Set(nonceKey, nonce)

		// Build CSP policy with nonce
		cspPolicy := buildCSPPolicy(nonce)

		// Set CSP header
		c.Header("Content-Security-Policy", cspPolicy)

		// Optional: CSP reporting
		if os.Getenv("ENABLE_CSP_REPORT") == "true" {
			reportURI := os.Getenv("CSP_REPORT_URI")
			if reportURI != "" {
				c.Header("Content-Security-Policy-Report-Only", cspPolicy+"; report-uri "+reportURI)
			}
		}

		c.Next()
	}
}

// GetNonce retrieves the nonce from the Gin context
func GetNonce(c *gin.Context) string {
	if nonce, exists := c.Get(nonceKey); exists {
		if nonceStr, ok := nonce.(string); ok {
			return nonceStr
		}
	}
	return ""
}

// buildCSPPolicy constructs the Content Security Policy with the provided nonce
func buildCSPPolicy(nonce string) string {
	return fmt.Sprintf(
		"default-src 'self'; "+
			"script-src 'self' 'nonce-%s'; "+
			"style-src 'self' 'nonce-%s' 'unsafe-inline'; "+
			"img-src 'self' data: https:; "+
			"font-src 'self' data:; "+
			"connect-src 'self'; "+
			"frame-ancestors 'none'; "+
			"base-uri 'self'; "+
			"form-action 'self'",
		nonce, nonce,
	)
}

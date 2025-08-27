package security

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/database"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/types"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// SecurityConfig holds security configuration
type SecurityConfig struct {
	MaxInputLength    int           `json:"max_input_length"`
	MaxRequestsPerMin int           `json:"max_requests_per_min"`
	EnableCORS        bool          `json:"enable_cors"`
	AllowedOrigins    []string      `json:"allowed_origins"`
	TrustedProxies    []string      `json:"trusted_proxies"`
	RequestTimeout    time.Duration `json:"request_timeout"`
}

// DefaultSecurityConfig returns secure defaults
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		MaxInputLength:    200,
		MaxRequestsPerMin: 60,
		EnableCORS:        true,
		AllowedOrigins:    []string{"http://localhost:3000", "http://localhost:5173", "https://js.stripe.com", "https://checkout.stripe.com"},
		TrustedProxies:    []string{"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
		RequestTimeout:    30 * time.Second,
	}
}

// SecurityMiddleware provides comprehensive security middleware
type SecurityMiddleware struct {
	config      SecurityConfig
	rateLimiter *rate.Limiter
	ipLimiters  map[string]*rate.Limiter
	userService *database.UserService
}

// NewSecurityMiddleware creates a new security middleware instance
func NewSecurityMiddleware(config SecurityConfig) *SecurityMiddleware {
	return &SecurityMiddleware{
		config:      config,
		rateLimiter: rate.NewLimiter(rate.Limit(config.MaxRequestsPerMin/60.0), config.MaxRequestsPerMin/10),
		ipLimiters:  make(map[string]*rate.Limiter),
	}
}

// SetUserService sets the user service for user-based rate limiting
func (sm *SecurityMiddleware) SetUserService(userService *database.UserService) {
	sm.userService = userService
}

// ValidateInput performs comprehensive input validation and sanitization
func (sm *SecurityMiddleware) ValidateInput(input string) error {
	// Check length limits
	if len(input) > sm.config.MaxInputLength {
		return fmt.Errorf("input exceeds maximum length of %d characters", sm.config.MaxInputLength)
	}

	// Check for null bytes (potential injection attempt)
	if strings.Contains(input, "\x00") {
		return fmt.Errorf("input contains invalid characters")
	}

	// Validate UTF-8 encoding
	if !utf8.ValidString(input) {
		return fmt.Errorf("input contains invalid UTF-8 encoding")
	}

	// Check for suspicious patterns (basic XSS/SQL injection detection)
	suspiciousPatterns := []string{
		`<script`, `</script>`, `javascript:`, `on\w+=`,
		`union select`, `drop table`, `alter table`,
		`--`, `/*`, `*/`, `xp_`, `sp_`,
	}

	inputLower := strings.ToLower(input)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(inputLower, pattern) {
			return fmt.Errorf("input contains suspicious patterns")
		}
	}

	// Validate GitHub username/repo format if applicable
	if err := sm.validateGitHubFormat(input); err != nil {
		return err
	}

	return nil
}

// validateGitHubFormat validates GitHub username and repository formats
func (sm *SecurityMiddleware) validateGitHubFormat(input string) error {
	// GitHub username/repo validation pattern
	// - Must start and end with alphanumeric
	// - No consecutive dots or dashes
	// - Only dots, dashes, underscores allowed as separators
	githubPattern := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*[a-zA-Z0-9]$`)

	// Check for consecutive dots or dashes (simple validation)
	if strings.Contains(input, "..") || strings.Contains(input, "--") {
		return fmt.Errorf("invalid GitHub username/repository format")
	}

	// Only validate if it contains slashes (indicating repo format) or github: prefix
	if strings.Contains(input, "github:") {
		// Extract GitHub part if prefixed
		githubPart := strings.TrimPrefix(input, "github:")
		if githubPart == "" {
			return fmt.Errorf("empty GitHub reference")
		}
		if !githubPattern.MatchString(githubPart) {
			return fmt.Errorf("invalid GitHub username/repository format")
		}
	} else if strings.Contains(input, "/") {
		// Looks like owner/repo format - validate each part
		parts := strings.Split(input, "/")
		for _, part := range parts {
			if part != "" {
				if !githubPattern.MatchString(part) {
					return fmt.Errorf("invalid GitHub username/repository format")
				}
			}
		}
	}

	return nil
}

// SanitizeInput sanitizes user input by removing potentially dangerous content
func (sm *SecurityMiddleware) SanitizeInput(input string) string {
	// Trim whitespace
	input = strings.TrimSpace(input)

	// Remove script tags and their content (more comprehensive)
	scriptPattern := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	input = scriptPattern.ReplaceAllString(input, "")

	// Remove other HTML tags (but keep content between them)
	htmlTagPattern := regexp.MustCompile(`<[^>]+>`)
	input = htmlTagPattern.ReplaceAllString(input, "")

	// Remove excessive whitespace
	input = regexp.MustCompile(`\s+`).ReplaceAllString(input, " ")

	// Decode HTML entities (basic)
	htmlEntities := map[string]string{
		"&lt;":   "<",
		"&gt;":   ">",
		"&amp;":  "&",
		"&quot;": "\"",
		"&#x27;": "'",
		"&#39;":  "'",
	}

	for entity, char := range htmlEntities {
		input = strings.ReplaceAll(input, entity, char)
	}

	return input
}

// RateLimitByIP implements per-IP rate limiting
func (sm *SecurityMiddleware) RateLimitByIP(c *gin.Context) {
	clientIP := c.ClientIP()

	// Get or create rate limiter for this IP
	if _, exists := sm.ipLimiters[clientIP]; !exists {
		// Create limiter with burst capacity for initial requests
		rps := rate.Limit(sm.config.MaxRequestsPerMin / 60.0)
		// Allow burst of up to half the requests per minute for initial allowance
		burst := sm.config.MaxRequestsPerMin / 2
		if burst < 5 {
			burst = 5 // Minimum burst of 5 requests
		}
		sm.ipLimiters[clientIP] = rate.NewLimiter(rps, burst)
	}

	limiter := sm.ipLimiters[clientIP]

	if !limiter.Allow() {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":       "rate limit exceeded for IP",
			"retry_after": "60", // seconds
		})
		c.Abort()
		return
	}

	c.Next()
}

// UserRateLimit implements user-based rate limiting (5 free requests per week)
func (sm *SecurityMiddleware) UserRateLimit(c *gin.Context) {
	// Only apply user rate limiting to analyze endpoints
	if c.Request.URL.Path != "/analyze" && c.Request.URL.Path != "/api/analyze" {
		c.Next()
		return
	}

	// Skip if user service is not configured
	if sm.userService == nil {
		c.Next()
		return
	}

	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	// Process the request through user service
	result, err := sm.userService.ProcessRequest(clientIP, userAgent, c.Request.URL.Path, c.Request.Method)
	if err != nil {
		// Log error but don't block - fallback to IP limiting
		fmt.Printf("[USER-RATE-LIMIT] Error processing user request: %v\n", err)
		c.Next()
		return
	}

	// Store user and usage info in context for handlers
	c.Set("user_id", result.User.ID)
	c.Set("user_stats", result.Usage)
	c.Set("request_logged", result.RequestLogged)

	// Check if user can make request
	if !result.CanMakeRequest {
		remainingRequests, _ := sm.userService.GetRemainingRequests(result.User.ID)

		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":              "weekly request limit exceeded",
			"message":            "You've used all 5 free requests this week",
			"remaining_requests": remainingRequests,
			"is_paid":            result.Usage.IsPaid,
			"week_start":         result.Usage.WeekStart.Format("2006-01-02"),
			"week_end":           result.Usage.WeekEnd.Format("2006-01-02"),
			"upgrade_url":        "/upgrade", // Frontend route for payment
		})
		c.Abort()
		return
	}

	c.Next()
}

// SecurityHeaders adds security headers to responses
func (sm *SecurityMiddleware) SecurityHeaders(c *gin.Context) {
	// Prevent MIME type sniffing
	c.Header("X-Content-Type-Options", "nosniff")

	// Prevent clickjacking - allow Stripe checkout
	c.Header("X-Frame-Options", "SAMEORIGIN")

	// XSS protection
	c.Header("X-XSS-Protection", "1; mode=block")

	// HSTS (HTTP Strict Transport Security) - only in production
	if c.Request.TLS != nil {
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}

	// Content Security Policy - allow Stripe and external resources
	c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://js.stripe.com https://checkout.stripe.com; style-src 'self' 'unsafe-inline'; connect-src 'self' https://api.stripe.com; frame-src https://checkout.stripe.com https://js.stripe.com")

	// Referrer Policy
	c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

	// Permissions Policy for camera/microphone (not needed)
	c.Header("Permissions-Policy", "camera=(), microphone=()")

	c.Next()
}

// ValidateContentType validates request content type
func (sm *SecurityMiddleware) ValidateContentType(c *gin.Context) {
	contentType := c.GetHeader("Content-Type")

	// Allow JSON and form-encoded content
	allowedTypes := []string{
		"application/json",
		"application/x-www-form-urlencoded",
		"multipart/form-data",
	}

	if contentType != "" {
		found := false
		for _, allowed := range allowedTypes {
			if strings.Contains(strings.ToLower(contentType), allowed) {
				found = true
				break
			}
		}

		if !found {
			c.JSON(http.StatusUnsupportedMediaType, gin.H{
				"error": "unsupported content type",
			})
			c.Abort()
			return
		}
	}

	c.Next()
}

// RequestTimeout enforces request timeout
func (sm *SecurityMiddleware) RequestTimeout(c *gin.Context) {
	// Create a timeout context
	ctx, cancel := context.WithTimeout(c.Request.Context(), sm.config.RequestTimeout)
	defer cancel()

	// Replace request context
	c.Request = c.Request.WithContext(ctx)

	// Set timeout header for client
	c.Header("X-Timeout", strconv.Itoa(int(sm.config.RequestTimeout.Seconds())))

	c.Next()
}

// RequestLogging provides secure request logging
func (sm *SecurityMiddleware) RequestLogging(c *gin.Context) {
	start := time.Now()
	path := c.Request.URL.Path
	raw := c.Request.URL.RawQuery

	// Log request
	c.Next()

	// Calculate latency
	latency := time.Since(start)

	// Log response (excluding sensitive data)
	statusCode := c.Writer.Status()
	clientIP := c.ClientIP()
	method := c.Request.Method

	// Sanitize path for logging
	if raw != "" {
		path = path + "?" + raw
	}

	// Log successful requests at info level, errors at warn level
	if statusCode >= 400 {
		c.Error(fmt.Errorf("[SECURITY] %s %s %d %v %s",
			method, path, statusCode, latency, clientIP))
	} else {
		// Only log non-sensitive paths or truncate sensitive data
		if !strings.Contains(path, "/health") {
			fmt.Printf("[SECURITY] %s %s %d %v %s\n",
				method, path, statusCode, latency, clientIP)
		}
	}
}

// ValidateAnalyzeRequest validates the analyze endpoint request
func (sm *SecurityMiddleware) ValidateAnalyzeRequest(c *gin.Context) {
	var req types.AnalyzeRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid JSON format",
		})
		c.Abort()
		return
	}

	// Validate input field
	if req.Input == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "input field is required",
		})
		c.Abort()
		return
	}

	// Sanitize and validate input
	req.Input = sm.SanitizeInput(req.Input)
	if err := sm.ValidateInput(req.Input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("input validation failed: %v", err),
		})
		c.Abort()
		return
	}

	// Store sanitized input in context for handler
	c.Set("sanitized_input", req.Input)
	c.Next()
}

// CORSConfig provides secure CORS configuration
func (sm *SecurityMiddleware) CORSConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range sm.config.AllowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Cleanup periodically cleans up old rate limiters
func (sm *SecurityMiddleware) Cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			sm.cleanupOldLimiters()
		}
	}()
}

// cleanupOldLimiters removes rate limiters for IPs that haven't been seen recently
func (sm *SecurityMiddleware) cleanupOldLimiters() {
	// In a production system, you'd want to track last seen time
	// For now, we'll keep all limiters to avoid complexity
	// This is a placeholder for future enhancement
}

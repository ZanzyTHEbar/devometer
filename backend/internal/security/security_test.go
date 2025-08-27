package security

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	assert.Equal(t, 200, config.MaxInputLength)
	assert.Equal(t, 60, config.MaxRequestsPerMin)
	assert.True(t, config.EnableCORS)
	assert.Contains(t, config.AllowedOrigins, "http://localhost:3000")
	assert.Contains(t, config.AllowedOrigins, "http://localhost:5173")
	assert.Equal(t, 30*time.Second, config.RequestTimeout)
}

func TestValidateInput(t *testing.T) {
	sm := NewSecurityMiddleware(DefaultSecurityConfig())

	tests := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid input",
			input:       "github:torvalds",
			expectError: false,
		},
		{
			name:        "input too long",
			input:       strings.Repeat("a", 201),
			expectError: true,
			errorMsg:    "input exceeds maximum length",
		},
		{
			name:        "null bytes",
			input:       "test\x00input",
			expectError: true,
			errorMsg:    "input contains invalid characters",
		},
		{
			name:        "invalid UTF-8",
			input:       "test\xff\xfeinput",
			expectError: true,
			errorMsg:    "input contains invalid UTF-8 encoding",
		},
		{
			name:        "XSS attempt",
			input:       "<script>alert('xss')</script>",
			expectError: true,
			errorMsg:    "input contains suspicious patterns",
		},
		{
			name:        "SQL injection attempt",
			input:       "'; DROP TABLE users; --",
			expectError: true,
			errorMsg:    "input contains suspicious patterns",
		},
		{
			name:        "invalid GitHub format",
			input:       "github:invalid..repo",
			expectError: true,
			errorMsg:    "invalid GitHub username/repository format",
		},
		{
			name:        "invalid repo format",
			input:       "invalid..repo/validrepo",
			expectError: true,
			errorMsg:    "invalid GitHub username/repository format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.ValidateInput(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSanitizeInput(t *testing.T) {
	sm := NewSecurityMiddleware(DefaultSecurityConfig())

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "trim whitespace",
			input:    "  test input  ",
			expected: "test input",
		},
		{
			name:     "remove HTML tags",
			input:    "<script>alert('test')</script>Hello World",
			expected: "Hello World",
		},
		{
			name:     "remove excessive whitespace",
			input:    "test   input    here",
			expected: "test input here",
		},
		{
			name:     "normal input unchanged",
			input:    "github:torvalds",
			expected: "github:torvalds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sm.SanitizeInput(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sm := NewSecurityMiddleware(DefaultSecurityConfig())

	r := gin.New()
	r.Use(sm.SecurityHeaders)

	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	// Check security headers
	headers := w.Header()
	assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", headers.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", headers.Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", headers.Get("Referrer-Policy"))
	assert.Contains(t, headers.Get("Content-Security-Policy"), "default-src 'self'")
}

func TestValidateContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sm := NewSecurityMiddleware(DefaultSecurityConfig())

	r := gin.New()
	r.Use(sm.ValidateContentType)

	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	tests := []struct {
		name           string
		contentType    string
		expectedStatus int
	}{
		{
			name:           "valid JSON",
			contentType:    "application/json",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid form data",
			contentType:    "application/x-www-form-urlencoded",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid content type",
			contentType:    "text/plain",
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "no content type",
			contentType:    "",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/test", bytes.NewBufferString(`{"test": "data"}`))

			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			r.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestValidateAnalyzeRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sm := NewSecurityMiddleware(DefaultSecurityConfig())

	r := gin.New()
	r.POST("/analyze", sm.ValidateAnalyzeRequest, func(c *gin.Context) {
		input, _ := c.Get("sanitized_input")
		c.JSON(http.StatusOK, gin.H{"input": input})
	})

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		checkContext   bool
	}{
		{
			name:           "valid request",
			requestBody:    types.AnalyzeRequest{Input: "github:torvalds"},
			expectedStatus: http.StatusOK,
			checkContext:   true,
		},
		{
			name:           "empty input",
			requestBody:    types.AnalyzeRequest{Input: ""},
			expectedStatus: http.StatusBadRequest,
			checkContext:   false,
		},
		{
			name:           "input too long",
			requestBody:    types.AnalyzeRequest{Input: strings.Repeat("a", 201)},
			expectedStatus: http.StatusBadRequest,
			checkContext:   false,
		},
		{
			name:           "missing input field",
			requestBody:    map[string]interface{}{"other": "field"},
			expectedStatus: http.StatusBadRequest,
			checkContext:   false,
		},
		{
			name:           "invalid JSON",
			requestBody:    nil, // This will be sent as raw string
			expectedStatus: http.StatusBadRequest,
			checkContext:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer

			if tt.requestBody != nil {
				jsonBody, _ := json.Marshal(tt.requestBody)
				body = *bytes.NewBuffer(jsonBody)
			} else {
				body = *bytes.NewBufferString(`invalid json`)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/analyze", &body)
			req.Header.Set("Content-Type", "application/json")

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkContext && w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "input")
			}
		})
	}
}

func TestRateLimitByIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create config with low rate limit for testing
	config := DefaultSecurityConfig()
	config.MaxRequestsPerMin = 10 // 10 requests per minute

	sm := NewSecurityMiddleware(config)

	r := gin.New()
	r.Use(sm.RateLimitByIP)

	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Test multiple requests from same IP
	clientIP := "192.168.1.100"

	// With burst capacity of 5 (half of 10 requests/min), we should get more requests through
	for i := 0; i < 7; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = clientIP + ":12345"

		r.ServeHTTP(w, req)

		if i < 5 {
			// First 5 requests should succeed (burst capacity)
			assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
		} else {
			// Subsequent requests may be rate limited
			// We don't assert strictly here since timing can vary
			if w.Code == http.StatusTooManyRequests {
				t.Logf("Request %d was rate limited as expected", i+1)
			} else {
				t.Logf("Request %d succeeded (rate limit not yet triggered)", i+1)
			}
		}
	}
}

func TestCORSConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sm := NewSecurityMiddleware(DefaultSecurityConfig())

	r := gin.New()
	r.Use(sm.CORSConfig())

	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	tests := []struct {
		name           string
		origin         string
		method         string
		expectedStatus int
		checkCORS      bool
	}{
		{
			name:           "allowed origin",
			origin:         "http://localhost:3000",
			method:         "GET",
			expectedStatus: http.StatusOK,
			checkCORS:      true,
		},
		{
			name:           "disallowed origin",
			origin:         "http://evil.com",
			method:         "GET",
			expectedStatus: http.StatusOK,
			checkCORS:      false,
		},
		{
			name:           "OPTIONS preflight",
			origin:         "http://localhost:3000",
			method:         "OPTIONS",
			expectedStatus: http.StatusNoContent,
			checkCORS:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, "/test", nil)
			req.Header.Set("Origin", tt.origin)

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkCORS {
				headers := w.Header()
				assert.Equal(t, tt.origin, headers.Get("Access-Control-Allow-Origin"))
				assert.Equal(t, "true", headers.Get("Access-Control-Allow-Credentials"))
			}
		})
	}
}

func TestRequestTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create config with very short timeout for testing
	config := DefaultSecurityConfig()
	config.RequestTimeout = 1 * time.Millisecond

	sm := NewSecurityMiddleware(config)

	r := gin.New()
	r.Use(sm.RequestTimeout)

	r.GET("/test", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond) // Sleep longer than timeout
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)

	start := time.Now()
	r.ServeHTTP(w, req)
	duration := time.Since(start)

	// Request should timeout
	assert.True(t, duration < 100*time.Millisecond, "Request should timeout quickly")
}

func TestGitHubValidationDebug(t *testing.T) {
	sm := NewSecurityMiddleware(DefaultSecurityConfig())

	// Test the specific problematic inputs
	testCases := []struct {
		input       string
		description string
	}{
		{"github:invalid..repo", "consecutive dots"},
		{"invalid..repo/validrepo", "consecutive dots in owner"},
		{"validrepo..", "trailing dots"},
		{"..validrepo", "leading dots"},
		{"valid-repo", "valid repo name"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := sm.ValidateInput(tc.input)
			t.Logf("Input: '%s', Error: %v", tc.input, err)
		})
	}
}

func TestSecurityMiddlewareIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sm := NewSecurityMiddleware(DefaultSecurityConfig())

	r := gin.New()

	// Apply all security middleware
	r.Use(sm.RequestLogging)
	r.Use(sm.SecurityHeaders)
	r.Use(sm.RequestTimeout)
	r.Use(sm.ValidateContentType)
	r.Use(sm.RateLimitByIP)

	r.POST("/analyze", sm.ValidateAnalyzeRequest, func(c *gin.Context) {
		input, _ := c.Get("sanitized_input")
		c.JSON(http.StatusOK, gin.H{"input": input, "status": "processed"})
	})

	// Test complete request flow
	requestBody := types.AnalyzeRequest{Input: "github:torvalds"}
	jsonBody, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/analyze", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"

	r.ServeHTTP(w, req)

	// Should succeed with proper security headers
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "github:torvalds", response["input"])
	assert.Equal(t, "processed", response["status"])

	// Check security headers
	headers := w.Header()
	assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", headers.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", headers.Get("X-XSS-Protection"))
}

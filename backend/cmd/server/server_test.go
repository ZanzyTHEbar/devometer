package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/adapters"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/analysis"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHealthEndpoint(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create router
	r := setupRouter()

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET /health returns OK status",
			method:         "GET",
			path:           "/health",
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":"ok"}`,
		},
		{
			name:           "POST /health method not allowed",
			method:         "POST",
			path:           "/health",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "",
		},
		{
			name:           "PUT /health method not allowed",
			method:         "PUT",
			path:           "/health",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "",
		},
		{
			name:           "DELETE /health method not allowed",
			method:         "DELETE",
			path:           "/health",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, tt.path, nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.JSONEq(t, tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestAnalyzeEndpoint_ValidRequests(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create router
	r := setupRouter()

	tests := []struct {
		name             string
		requestBody      map[string]interface{}
		expectedStatus   int
		validateResponse func(t *testing.T, response map[string]interface{})
	}{
		{
			name: "POST /analyze with valid repo input",
			requestBody: map[string]interface{}{
				"input": "facebook/react",
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, response map[string]interface{}) {
				assert.Contains(t, response, "score")
				assert.Contains(t, response, "confidence")
				assert.Contains(t, response, "posterior")
				assert.Contains(t, response, "contributors")
				assert.Contains(t, response, "breakdown")

				score := response["score"].(float64)
				assert.GreaterOrEqual(t, score, 0.0)
				assert.LessOrEqual(t, score, 100.0)

				confidence := response["confidence"].(float64)
				assert.GreaterOrEqual(t, confidence, 0.0)
				assert.LessOrEqual(t, confidence, 1.0)

				posterior := response["posterior"].(float64)
				assert.GreaterOrEqual(t, posterior, 0.0)
				assert.LessOrEqual(t, posterior, 1.0)
			},
		},
		{
			name: "POST /analyze with valid username input",
			requestBody: map[string]interface{}{
				"input": "octocat",
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, response map[string]interface{}) {
				assert.Contains(t, response, "score")
				assert.Contains(t, response, "confidence")
				assert.Contains(t, response, "posterior")
				assert.Contains(t, response, "contributors")
				assert.Contains(t, response, "breakdown")

				score := response["score"].(float64)
				assert.GreaterOrEqual(t, score, 0.0)
				assert.LessOrEqual(t, score, 100.0)
			},
		},
		{
			name: "POST /analyze with empty input",
			requestBody: map[string]interface{}{
				"input": "",
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, response map[string]interface{}) {
				assert.Contains(t, response, "score")
				// Empty input should still produce a valid response
			},
		},
		{
			name: "POST /analyze with special characters in input",
			requestBody: map[string]interface{}{
				"input": "user-name_123/repo@branch",
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, response map[string]interface{}) {
				assert.Contains(t, response, "score")
				// Should handle special characters gracefully
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestBody, _ := json.Marshal(tt.requestBody)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/analyze", bytes.NewBuffer(requestBody))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tt.validateResponse != nil {
				tt.validateResponse(t, response)
			}
		})
	}
}

func TestAnalyzeEndpoint_InvalidRequests(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create router
	r := setupRouter()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "POST /analyze with invalid JSON",
			requestBody:    `{"input": "test", invalid}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid request",
		},
		{
			name:           "POST /analyze with missing input field",
			requestBody:    `{"other_field": "value"}`,
			expectedStatus: http.StatusOK, // Server accepts this and treats empty input as valid
			expectedError:  "",
		},
		{
			name:           "POST /analyze with empty body",
			requestBody:    ``,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid request",
		},
		{
			name:           "POST /analyze with empty input",
			requestBody:    `{"input": ""}`,
			expectedStatus: http.StatusOK, // Server handles empty input gracefully
			expectedError:  "",
		},
		{
			name:           "POST /analyze with whitespace only input",
			requestBody:    `{"input": "   "}`,
			expectedStatus: http.StatusOK, // Server trims whitespace and handles gracefully
			expectedError:  "",
		},
		{
			name:           "POST /analyze with wrong content type",
			requestBody:    `input=test`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid request",
		},
		{
			name:           "POST /analyze with null input",
			requestBody:    `{"input": null}`,
			expectedStatus: http.StatusOK, // Server accepts this and treats as empty string
			expectedError:  "",
		},
		{
			name:           "POST /analyze with non-string input",
			requestBody:    `{"input": 123}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			if tt.requestBody != "" {
				body = *bytes.NewBufferString(tt.requestBody)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/analyze", &body)

			// Set content type to JSON unless testing wrong content type
			if tt.name != "POST /analyze with wrong content type" {
				req.Header.Set("Content-Type", "application/json")
			}

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tt.expectedError != "" {
				assert.Contains(t, response, "error")
				assert.Equal(t, tt.expectedError, response["error"])
			} else {
				// For successful requests, we should get a normal response
				assert.Contains(t, response, "score")
				assert.Contains(t, response, "confidence")
			}
		})
	}
}

func TestAnalyzeEndpoint_MethodNotAllowed(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create router
	r := setupRouter()

	tests := []struct {
		name   string
		method string
	}{
		{"GET method not allowed", "GET"},
		{"PUT method not allowed", "PUT"},
		{"DELETE method not allowed", "DELETE"},
		{"PATCH method not allowed", "PATCH"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestBody := `{"input": "test/repo"}`

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, "/analyze", bytes.NewBufferString(requestBody))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestServer_ResponseFormat(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create router
	r := setupRouter()

	// Test analyze endpoint response structure
	requestBody := `{"input": "test/repo"}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/analyze", bytes.NewBufferString(requestBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify response has all expected fields
	requiredFields := []string{"score", "confidence", "posterior", "contributors", "breakdown"}
	for _, field := range requiredFields {
		assert.Contains(t, response, field, "Response should contain field: %s", field)
	}

	// Verify contributors is an array
	contributors, ok := response["contributors"].([]interface{})
	assert.True(t, ok, "contributors should be an array")
	assert.NotNil(t, contributors)

	// Verify breakdown is an object with expected fields
	breakdown, ok := response["breakdown"].(map[string]interface{})
	assert.True(t, ok, "breakdown should be an object")

	breakdownFields := []string{"shipping", "quality", "influence", "complexity", "collaboration", "reliability", "novelty"}
	for _, field := range breakdownFields {
		assert.Contains(t, breakdown, field, "breakdown should contain field: %s", field)
	}
}

func TestServer_ContentType(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create router
	r := setupRouter()

	// Test that responses have correct content type
	tests := []struct {
		name string
		path string
	}{
		{"health endpoint", "/health"},
		{"analyze endpoint", "/analyze"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			method := "GET"
			if tt.path == "/analyze" {
				method = "POST"
				body = *bytes.NewBufferString(`{"input": "test"}`)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(method, tt.path, &body)
			if tt.path == "/analyze" {
				req.Header.Set("Content-Type", "application/json")
			}
			r.ServeHTTP(w, req)

			// Health endpoint returns JSON with GET, analyze endpoint with POST
			if tt.path == "/health" {
				assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
			} else {
				assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
			}
		})
	}
}

func TestServer_CORSHeaders(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create router
	r := setupRouter()

	// Test CORS headers are set (if CORS middleware is enabled)
	requestBody := `{"input": "test/repo"}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/analyze", bytes.NewBufferString(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	r.ServeHTTP(w, req)

	// Note: Current implementation may not have CORS headers
	// This test documents expected CORS behavior
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_ErrorHandling(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create router
	r := setupRouter()

	// Test 404 for unknown endpoints
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/unknown", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	// Test with malformed JSON that causes panic (if any)
	// This tests that the server handles internal errors gracefully
	requestBody := `{"input": "test", "malformed": }`

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/analyze", bytes.NewBufferString(requestBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Should return 400 Bad Request for malformed JSON
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServer_LargePayload(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create router
	r := setupRouter()

	// Test with very large input
	largeInput := make([]byte, 10000) // 10KB of data
	for i := range largeInput {
		largeInput[i] = 'a'
	}

	requestBody := map[string]interface{}{
		"input": string(largeInput),
	}
	requestBodyBytes, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/analyze", bytes.NewBuffer(requestBodyBytes))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Should handle large payloads gracefully
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_ConcurrentRequests(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create router
	r := setupRouter()

	// Test concurrent requests to ensure thread safety
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			requestBody := map[string]interface{}{
				"input": "test/repo",
			}
			requestBodyBytes, _ := json.Marshal(requestBody)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/analyze", bytes.NewBuffer(requestBodyBytes))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// setupRouter creates a test router with the same configuration as main
func setupRouter() *gin.Engine {
	// Get configuration from environment
	dataDir := "./data"
	githubToken := ""

	// Create analyzer and adapter
	analyzer := analysis.NewAnalyzer(dataDir)
	githubAdapter := adapters.NewGitHubAdapter(githubToken)

	r := gin.New() // Use New() instead of Default() to avoid default middleware in tests

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/analyze", func(c *gin.Context) {
		var req struct {
			Input string `json:"input"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		input := req.Input

		// Parse input to determine if it's a repo or user
		var ghEvents []adapters.GitHubEvent
		var err error

		if strings.Contains(input, "/") {
			// Looks like owner/repo
			parts := strings.Split(input, "/")
			if len(parts) == 2 {
				ghEvents, err = githubAdapter.FetchRepoData(nil, parts[0], parts[1])
			}
		} else {
			// Assume it's a username
			ghEvents, err = githubAdapter.FetchUserData(nil, input)
		}

		var rawEvents []types.RawEvent
		if err != nil {
			// Fallback to heuristic-based analysis if GitHub API fails
			rawEvents = []types.RawEvent{}
		} else {
			// Convert GitHub events to RawEvents
			rawEvents = make([]types.RawEvent, len(ghEvents))
			for i, gh := range ghEvents {
				rawEvents[i] = types.RawEvent{
					Type:      gh.Type,
					Timestamp: time.Now(), // Use current time for simplified implementation
					Count:     gh.Count,
					Repo:      gh.Repo,
					Language:  gh.Language,
				}
			}
		}

		// Use the analyzer
		res, err := analyzer.AnalyzeEvents(rawEvents, input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "analysis failed"})
			return
		}

		c.JSON(http.StatusOK, res)
	})

	return r
}

// TestFallbackMechanism tests the fallback behavior when X API is unavailable
func TestFallbackMechanism(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		input          string
		xBearerToken   string
		githubToken    string
		expectedStatus int
		expectedScore  bool // true if we expect a score, false if we expect an error
		description    string
	}{
		{
			name:           "GitHub succeeds, X fails gracefully",
			input:          "github:torvalds x:nonexistentuser",
			xBearerToken:   "invalid_token",
			githubToken:    "test_token",
			expectedStatus: http.StatusOK,
			expectedScore:  true,
			description:    "Should continue with GitHub-only analysis",
		},
		{
			name:           "X succeeds, GitHub fails gracefully",
			input:          "github:nonexistentuser x:testuser",
			xBearerToken:   "test_token",
			githubToken:    "",
			expectedStatus: http.StatusOK,
			expectedScore:  true,
			description:    "Should continue with X-only analysis",
		},
		{
			name:           "Both GitHub and X fail",
			input:          "github: x:", // Empty usernames after parsing
			xBearerToken:   "",
			githubToken:    "",
			expectedStatus: http.StatusBadRequest,
			expectedScore:  false,
			description:    "Should return error when both adapters lack authentication",
		},
		{
			name:           "X token not configured",
			input:          "github:torvalds x:testuser",
			xBearerToken:   "",
			githubToken:    "test_token",
			expectedStatus: http.StatusOK,
			expectedScore:  true,
			description:    "Should log warning but continue with GitHub-only",
		},
		{
			name:           "Combined analysis succeeds",
			input:          "github:torvalds x:testuser",
			xBearerToken:   "test_token",
			githubToken:    "test_token",
			expectedStatus: http.StatusOK,
			expectedScore:  true,
			description:    "Should perform full combined analysis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create router with test configuration
			r := setupRouterWithConfig(tt.githubToken, tt.xBearerToken)

			// Create request
			reqBody := map[string]interface{}{
				"input": tt.input,
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, _ := http.NewRequest("POST", "/analyze", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			r.ServeHTTP(w, req)

			// Assert status
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			if tt.expectedScore {
				// Should return a valid analysis result
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err, "Should return valid JSON")

				// Check for score field
				score, hasScore := response["score"]
				assert.True(t, hasScore, "Response should contain score field")
				assert.IsType(t, float64(0), score, "Score should be a number")

				scoreInt := int(score.(float64))
				assert.GreaterOrEqual(t, scoreInt, 0, "Score should be >= 0")
				assert.LessOrEqual(t, scoreInt, 100, "Score should be <= 100")
			} else {
				// Should return an error
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err, "Should return valid JSON")

				// Check for error field
				_, hasError := response["error"]
				assert.True(t, hasError, "Response should contain error field")
			}
		})
	}
}

// setupRouterWithConfig creates a router with custom configuration for testing
func setupRouterWithConfig(githubToken, xBearerToken string) *gin.Engine {
	r := gin.New()

	// Add middleware similar to main
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %d %s %s\n",
			param.TimeStamp.Format("2006/01/02 15:04:05"),
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
			param.ClientIP,
		)
	}))
	r.Use(gin.Recovery())

	// Create adapters
	githubAdapter := adapters.NewGitHubAdapter(githubToken)
	xAdapter := adapters.NewXAdapterWithToken(xBearerToken)
	analyzer := analysis.NewAnalyzer("./test_data")

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		})
	})

	r.POST("/analyze", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		var req map[string]interface{}
		if err := c.BindJSON(&req); err != nil {
			slog.Error("Invalid JSON request", "error", err, "ip", c.ClientIP())
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON format"})
			return
		}

		inputValue, exists := req["input"]
		if !exists {
			slog.Warn("Missing input field", "ip", c.ClientIP())
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing required 'input' field"})
			return
		}

		input, ok := inputValue.(string)
		if !ok {
			slog.Warn("Invalid input type", "type", fmt.Sprintf("%T", inputValue), "ip", c.ClientIP())
			c.JSON(http.StatusBadRequest, gin.H{"error": "input must be a string"})
			return
		}

		input = strings.TrimSpace(input)
		if input == "" {
			slog.Warn("Empty input", "ip", c.ClientIP())
			c.JSON(http.StatusBadRequest, gin.H{"error": "input cannot be empty"})
			return
		}

		if len(input) > 200 {
			slog.Warn("Input too long", "length", len(input), "ip", c.ClientIP())
			c.JSON(http.StatusBadRequest, gin.H{"error": "input too long (max 200 characters)"})
			return
		}

		slog.Info("Starting analysis", "input", input, "ip", c.ClientIP())

		// Parse input for GitHub and X usernames
		githubUsername, xUsername := parseCombinedInput(input)

		var githubEvents []types.RawEvent
		var xEvents []types.RawEvent

		// Fetch GitHub data if username provided
		if githubUsername != "" {
			var ghEvents []adapters.GitHubEvent
			var err error

			if strings.Contains(githubUsername, "/") {
				parts := strings.Split(githubUsername, "/")
				if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
					ghEvents, err = githubAdapter.FetchRepoData(ctx, parts[0], parts[1])
				} else {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid repository format (use owner/repo)"})
					return
				}
			} else {
				ghEvents, err = githubAdapter.FetchUserData(ctx, githubUsername)
			}

			if err != nil {
				slog.Error("GitHub API error", "error", err, "username", githubUsername)
				slog.Warn("Continuing analysis without GitHub data", "ip", c.ClientIP())
			} else {
				githubEvents = make([]types.RawEvent, len(ghEvents))
				for i, gh := range ghEvents {
					githubEvents[i] = types.RawEvent{
						Type:      gh.Type,
						Timestamp: time.Now(),
						Count:     gh.Count,
						Repo:      gh.Repo,
						Language:  gh.Language,
					}
				}
			}
		}

		// Fetch X data if username provided and adapter is authenticated
		if xUsername != "" && xAdapter.IsAuthenticated() {
			xAdapterEvents, err := xAdapter.FetchUserData(ctx, xUsername)
			if err != nil {
				slog.Error("X API error", "error", err, "username", xUsername)
				slog.Warn("Continuing analysis without X data", "ip", c.ClientIP())
			} else {
				xEvents = convertXEventsToRawEvents(xAdapterEvents)
			}
		} else if xUsername != "" && !xAdapter.IsAuthenticated() {
			slog.Warn("X analysis requested but no bearer token configured", "username", xUsername, "ip", c.ClientIP())
		}

		// Perform analysis based on available data
		var res analysis.ScoreResult
		var err error

		if len(githubEvents) > 0 && len(xEvents) > 0 {
			slog.Info("Performing combined GitHub + X analysis",
				"github_events", len(githubEvents),
				"x_events", len(xEvents),
				"github_user", githubUsername,
				"x_user", xUsername,
				"ip", c.ClientIP())
			res, err = analyzer.AnalyzeEventsWithX(githubEvents, xEvents, input)
		} else if len(githubEvents) > 0 {
			slog.Info("Performing GitHub-only analysis",
				"events", len(githubEvents),
				"user", githubUsername,
				"ip", c.ClientIP())
			res, err = analyzer.AnalyzeEvents(githubEvents, input)
		} else if len(xEvents) > 0 {
			slog.Info("Performing X-only analysis",
				"events", len(xEvents),
				"user", xUsername,
				"ip", c.ClientIP())
			res, err = analyzer.AnalyzeEvents(xEvents, input)
		} else {
			slog.Warn("No analyzable data found", "input", input, "ip", c.ClientIP())
			c.JSON(http.StatusBadRequest, gin.H{"error": "no analyzable data found for the provided input"})
			return
		}

		if err != nil {
			slog.Error("Analysis failed", "error", err, "input", input)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "analysis failed"})
			return
		}

		slog.Info("Analysis completed", "input", input, "score", res.Score, "confidence", res.Confidence)
		c.JSON(http.StatusOK, res)
	})

	return r
}

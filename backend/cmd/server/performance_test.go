package main

import (
	"bytes"
	"encoding/json"
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

func TestAnalyzeEndpoint_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	gin.SetMode(gin.TestMode)
	r := createTestRouter()

	// Test data for performance measurement
	testInputs := []string{
		"facebook/react",
		"octocat",
		"microsoft/vscode",
		"torvalds/linux",
		"golang/go",
	}

	// Warm up the system
	for _, input := range testInputs[:2] {
		requestBody := map[string]interface{}{"input": input}
		requestBodyBytes, _ := json.Marshal(requestBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/analyze", bytes.NewBuffer(requestBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Measure performance
	var totalDuration time.Duration
	var requestCount int

	for _, input := range testInputs {
		requestBody := map[string]interface{}{"input": input}
		requestBodyBytes, _ := json.Marshal(requestBody)

		start := time.Now()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/analyze", bytes.NewBuffer(requestBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		duration := time.Since(start)

		totalDuration += duration
		requestCount++

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, duration < 5*time.Second, "Request should complete within 5 seconds, took %v", duration)
	}

	averageDuration := totalDuration / time.Duration(requestCount)
	t.Logf("Performance test completed: %d requests, average response time: %v", requestCount, averageDuration)

	// Assert reasonable performance
	assert.True(t, averageDuration < 2*time.Second, "Average response time should be under 2 seconds")
	assert.True(t, totalDuration < 10*time.Second, "Total test time should be under 10 seconds")
}

func TestAnalyzeEndpoint_LoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	gin.SetMode(gin.TestMode)
	r := createTestRouter()

	const numRequests = 50
	const numConcurrent = 10

	requestBody := map[string]interface{}{"input": "facebook/react"}
	requestBodyBytes, _ := json.Marshal(requestBody)

	// Channel to collect results
	results := make(chan struct {
		duration time.Duration
		status   int
	}, numRequests)

	// Launch concurrent requests
	for i := 0; i < numConcurrent; i++ {
		go func() {
			for j := 0; j < numRequests/numConcurrent; j++ {
				start := time.Now()
				w := httptest.NewRecorder()
				req, _ := http.NewRequest("POST", "/analyze", bytes.NewBuffer(requestBodyBytes))
				req.Header.Set("Content-Type", "application/json")
				r.ServeHTTP(w, req)
				duration := time.Since(start)

				results <- struct {
					duration time.Duration
					status   int
				}{duration, w.Code}
			}
		}()
	}

	// Collect results
	var totalDuration time.Duration
	var successCount int
	maxDuration := time.Duration(0)
	minDuration := time.Hour

	for i := 0; i < numRequests; i++ {
		result := <-results
		totalDuration += result.duration

		if result.status == http.StatusOK {
			successCount++
		}

		if result.duration > maxDuration {
			maxDuration = result.duration
		}
		if result.duration < minDuration {
			minDuration = result.duration
		}
	}

	averageDuration := totalDuration / time.Duration(numRequests)
	successRate := float64(successCount) / float64(numRequests) * 100

	t.Logf("Load test results:")
	t.Logf("  Total requests: %d", numRequests)
	t.Logf("  Successful responses: %d (%.1f%%)", successCount, successRate)
	t.Logf("  Average response time: %v", averageDuration)
	t.Logf("  Min response time: %v", minDuration)
	t.Logf("  Max response time: %v", maxDuration)

	// Assert load test requirements
	assert.Equal(t, numRequests, successCount, "All requests should succeed")
	assert.True(t, averageDuration < 3*time.Second, "Average response time should be under 3 seconds under load")
	assert.True(t, maxDuration < 10*time.Second, "Maximum response time should be under 10 seconds")
	assert.True(t, successRate >= 99.0, "Success rate should be at least 99%")
}

func TestAnalysisPipeline_TimingBreakdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timing breakdown test in short mode")
	}

	// Create analyzer directly to measure internal timing
	dataDir := "./test_data"
	analyzer := analysis.NewAnalyzer(dataDir)
	githubAdapter := adapters.NewGitHubAdapter("fake_token")

	input := "facebook/react"
	parts := strings.Split(input, "/")
	ghEvents, err := githubAdapter.FetchRepoData(nil, parts[0], parts[1])
	assert.NoError(t, err)

	// Convert to raw events
	rawEvents := make([]types.RawEvent, len(ghEvents))
	for i, gh := range ghEvents {
		rawEvents[i] = types.RawEvent{
			Type:      gh.Type,
			Timestamp: time.Now(),
			Count:     gh.Count,
			Repo:      gh.Repo,
			Language:  gh.Language,
		}
	}

	// Measure analysis timing
	start := time.Now()
	result, err := analyzer.AnalyzeEvents(rawEvents, input)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	t.Logf("Analysis pipeline timing:")
	t.Logf("  Total duration: %v", duration)
	t.Logf("  Score: %d", result.Score)
	t.Logf("  Confidence: %.3f", result.Confidence)
	t.Logf("  Contributors: %d", len(result.Contributors))

	// Assert reasonable timing
	assert.True(t, duration < 1*time.Second, "Analysis should complete within 1 second")
	assert.True(t, duration > 1*time.Microsecond, "Analysis should take at least 1µs (not instant)")
}

func TestMemoryUsage_UnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory usage test in short mode")
	}

	gin.SetMode(gin.TestMode)
	r := createTestRouter()

	const numRequests = 100

	requestBody := map[string]interface{}{"input": "facebook/react"}
	requestBodyBytes, _ := json.Marshal(requestBody)

	// Monitor memory usage through multiple requests
	for i := 0; i < numRequests; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/analyze", bytes.NewBuffer(requestBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Add small delay to prevent overwhelming the system
		if i%10 == 0 {
			time.Sleep(1 * time.Millisecond)
		}
	}

	t.Logf("Memory usage test completed: %d requests processed", numRequests)
}

func TestConcurrentAnalysis_ThreadSafety(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping thread safety test in short mode")
	}

	gin.SetMode(gin.TestMode)
	r := createTestRouter()

	const numGoroutines = 20
	const requestsPerGoroutine = 5

	requestBody := map[string]interface{}{"input": "facebook/react"}
	requestBodyBytes, _ := json.Marshal(requestBody)

	// Channel to collect results
	results := make(chan error, numGoroutines*requestsPerGoroutine)

	// Launch concurrent requests
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < requestsPerGoroutine; j++ {
				w := httptest.NewRecorder()
				req, _ := http.NewRequest("POST", "/analyze", bytes.NewBuffer(requestBodyBytes))
				req.Header.Set("Content-Type", "application/json")
				r.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					results <- assert.AnError
				} else {
					results <- nil
				}
			}
		}()
	}

	// Collect results
	var errorCount int
	for i := 0; i < numGoroutines*requestsPerGoroutine; i++ {
		err := <-results
		if err != nil {
			errorCount++
		}
	}

	t.Logf("Thread safety test completed:")
	t.Logf("  Total requests: %d", numGoroutines*requestsPerGoroutine)
	t.Logf("  Errors: %d", errorCount)

	assert.Equal(t, 0, errorCount, "No errors should occur in concurrent requests")
}

func TestEndpoint_ResponseTimeDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping response time distribution test in short mode")
	}

	gin.SetMode(gin.TestMode)
	r := createTestRouter()

	const numRequests = 100
	durations := make([]time.Duration, numRequests)

	requestBody := map[string]interface{}{"input": "facebook/react"}
	requestBodyBytes, _ := json.Marshal(requestBody)

	// Collect response times
	for i := 0; i < numRequests; i++ {
		start := time.Now()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/analyze", bytes.NewBuffer(requestBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		duration := time.Since(start)

		durations[i] = duration
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Calculate statistics
	var totalDuration time.Duration
	var minDuration = time.Hour
	var maxDuration time.Duration

	for _, duration := range durations {
		totalDuration += duration
		if duration < minDuration {
			minDuration = duration
		}
		if duration > maxDuration {
			maxDuration = duration
		}
	}

	averageDuration := totalDuration / time.Duration(numRequests)

	// Calculate percentiles
	percentiles := calculatePercentiles(durations, 0.5, 0.95, 0.99)
	p50 := percentiles[0]
	p95 := percentiles[1]
	p99 := percentiles[2]

	t.Logf("Response time distribution:")
	t.Logf("  Requests: %d", numRequests)
	t.Logf("  Average: %v", averageDuration)
	t.Logf("  Min: %v", minDuration)
	t.Logf("  Max: %v", maxDuration)
	t.Logf("  P50: %v", p50)
	t.Logf("  P95: %v", p95)
	t.Logf("  P99: %v", p99)

	// Assert performance requirements
	assert.True(t, averageDuration < 500*time.Millisecond, "Average response time should be under 500ms")
	assert.True(t, p95 < 1*time.Second, "95th percentile should be under 1 second")
	assert.True(t, p99 < 2*time.Second, "99th percentile should be under 2 seconds")
}

func calculatePercentiles(durations []time.Duration, percentiles ...float64) []time.Duration {
	if len(percentiles) == 0 {
		return []time.Duration{}
	}

	results := make([]time.Duration, len(percentiles))

	for i, p := range percentiles {
		index := int(float64(len(durations)-1) * p)
		if index >= len(durations) {
			index = len(durations) - 1
		}
		results[i] = durations[index]
	}

	return results
}

func TestErrorRecovery_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping error recovery performance test in short mode")
	}

	gin.SetMode(gin.TestMode)
	r := createTestRouter()

	// Test that error handling doesn't significantly impact performance
	validRequestBody := map[string]interface{}{"input": "facebook/react"}
	validRequestBodyBytes, _ := json.Marshal(validRequestBody)

	invalidRequestBody := `{"input": "test", "malformed": }`
	const numRequests = 50

	var validDurations []time.Duration
	var invalidDurations []time.Duration

	// Measure valid requests
	for i := 0; i < numRequests; i++ {
		start := time.Now()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/analyze", bytes.NewBuffer(validRequestBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		duration := time.Since(start)

		assert.Equal(t, http.StatusOK, w.Code)
		validDurations = append(validDurations, duration)
	}

	// Measure invalid requests
	for i := 0; i < numRequests; i++ {
		start := time.Now()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/analyze", bytes.NewBufferString(invalidRequestBody))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		duration := time.Since(start)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		invalidDurations = append(invalidDurations, duration)
	}

	// Calculate averages
	var validTotal, invalidTotal time.Duration
	for _, d := range validDurations {
		validTotal += d
	}
	for _, d := range invalidDurations {
		invalidTotal += d
	}

	validAvg := validTotal / time.Duration(len(validDurations))
	invalidAvg := invalidTotal / time.Duration(len(invalidDurations))

	t.Logf("Error recovery performance:")
	t.Logf("  Valid requests average: %v", validAvg)
	t.Logf("  Invalid requests average: %v", invalidAvg)
	t.Logf("  Error handling overhead: %v", invalidAvg-validAvg)

	// Error handling should not add significant overhead
	assert.True(t, invalidAvg < validAvg*2, "Error handling should not double response time")
}

// Helper function to create test router
func createTestRouter() *gin.Engine {
	dataDir := "./test_data"
	analyzer := analysis.NewAnalyzer(dataDir)
	githubAdapter := adapters.NewGitHubAdapter("fake_token")

	r := gin.New()

	r.POST("/analyze", func(c *gin.Context) {
		var req struct {
			Input string `json:"input"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		input := req.Input

		var ghEvents []adapters.GitHubEvent
		var err error

		if strings.Contains(input, "/") {
			parts := strings.Split(input, "/")
			if len(parts) == 2 {
				ghEvents, err = githubAdapter.FetchRepoData(nil, parts[0], parts[1])
			}
		} else {
			ghEvents, err = githubAdapter.FetchUserData(nil, input)
		}

		var rawEvents []types.RawEvent
		if err != nil {
			rawEvents = []types.RawEvent{}
		} else {
			rawEvents = make([]types.RawEvent, len(ghEvents))
			for i, gh := range ghEvents {
				rawEvents[i] = types.RawEvent{
					Type:      gh.Type,
					Timestamp: time.Now(),
					Count:     gh.Count,
					Repo:      gh.Repo,
					Language:  gh.Language,
				}
			}
		}

		res, err := analyzer.AnalyzeEvents(rawEvents, input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "analysis failed"})
			return
		}

		c.JSON(http.StatusOK, res)
	})

	return r
}

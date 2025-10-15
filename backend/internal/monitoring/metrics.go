package monitoring

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds application metrics
type Metrics struct {
	RequestCount        int64
	ErrorCount          int64
	CacheHits           int64
	CacheMisses         int64
	GitHubAPICalls      int64
	XAPICalls           int64
	AverageResponseTime int64 // in nanoseconds
	StartTime           time.Time

	// Enhanced metrics for percentiles and histograms
	ResponseTimes      []time.Duration
	ResponseTimesMutex sync.RWMutex

	// Status code tracking
	RequestCountByStatus map[int]int64
	StatusMutex          sync.RWMutex

	// Circuit breaker metrics
	CircuitBreakerOpens  int64
	CircuitBreakerCloses int64

	// External API metrics
	ExternalAPIRequests   map[string]int64
	ExternalAPIErrorCount map[string]int64
	ExternalAPIMutex      sync.RWMutex

	// Memory and system metrics
	GCCount        int64
	GCPauseTotalNs int64
	HeapAlloc      int64
	HeapSys        int64

	// Rate limit metrics
	RateLimitIPBlocks       int64
	RateLimitUserBlocks     int64
	RateLimitRedisErrors    int64
	RateLimitFallbackCount  int64
	RateLimitEndpointBlocks map[string]int64
	RateLimitMutex          sync.RWMutex
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		StartTime:               time.Now(),
		ResponseTimes:           make([]time.Duration, 0, 1000), // Pre-allocate for better performance
		RequestCountByStatus:    make(map[int]int64),
		ExternalAPIRequests:     make(map[string]int64),
		ExternalAPIErrorCount:   make(map[string]int64),
		RateLimitEndpointBlocks: make(map[string]int64),
	}
}

// IncrementRequest increments the request count
func (m *Metrics) IncrementRequest() {
	atomic.AddInt64(&m.RequestCount, 1)
}

// IncrementError increments the error count
func (m *Metrics) IncrementError() {
	atomic.AddInt64(&m.ErrorCount, 1)
}

// IncrementCacheHit increments cache hit count
func (m *Metrics) IncrementCacheHit() {
	atomic.AddInt64(&m.CacheHits, 1)
}

// IncrementCacheMiss increments cache miss count
func (m *Metrics) IncrementCacheMiss() {
	atomic.AddInt64(&m.CacheMisses, 1)
}

// IncrementGitHubCalls increments GitHub API call count
func (m *Metrics) IncrementGitHubCalls() {
	atomic.AddInt64(&m.GitHubAPICalls, 1)
}

// IncrementXCalls increments X API call count
func (m *Metrics) IncrementXCalls() {
	atomic.AddInt64(&m.XAPICalls, 1)
}

// RecordResponseTime records response time for averaging and percentiles
func (m *Metrics) RecordResponseTime(duration time.Duration) {
	// Update simple average
	current := atomic.LoadInt64(&m.AverageResponseTime)
	newAverage := (current + duration.Nanoseconds()) / 2
	atomic.StoreInt64(&m.AverageResponseTime, newAverage)

	// Store detailed response time for percentiles (keep last 1000 samples)
	m.ResponseTimesMutex.Lock()
	m.ResponseTimes = append(m.ResponseTimes, duration)
	if len(m.ResponseTimes) > 1000 {
		m.ResponseTimes = m.ResponseTimes[1:] // Remove oldest
	}
	m.ResponseTimesMutex.Unlock()
}

// RecordRequestByStatus records request count by HTTP status code
func (m *Metrics) RecordRequestByStatus(statusCode int) {
	m.StatusMutex.Lock()
	defer m.StatusMutex.Unlock()
	m.RequestCountByStatus[statusCode]++
}

// IncrementCircuitBreakerOpen increments circuit breaker open count
func (m *Metrics) IncrementCircuitBreakerOpen() {
	atomic.AddInt64(&m.CircuitBreakerOpens, 1)
}

// IncrementCircuitBreakerClose increments circuit breaker close count
func (m *Metrics) IncrementCircuitBreakerClose() {
	atomic.AddInt64(&m.CircuitBreakerCloses, 1)
}

// RecordExternalAPIRequest records an external API request
func (m *Metrics) RecordExternalAPIRequest(apiName string, success bool) {
	m.ExternalAPIMutex.Lock()
	defer m.ExternalAPIMutex.Unlock()

	m.ExternalAPIRequests[apiName]++
	if !success {
		m.ExternalAPIErrorCount[apiName]++
	}
}

// RecordGCMetrics records Go garbage collector metrics
func (m *Metrics) RecordGCMetrics(gcCount int64, gcPauseTotalNs int64, heapAlloc, heapSys int64) {
	atomic.StoreInt64(&m.GCCount, gcCount)
	atomic.StoreInt64(&m.GCPauseTotalNs, gcPauseTotalNs)
	atomic.StoreInt64(&m.HeapAlloc, heapAlloc)
	atomic.StoreInt64(&m.HeapSys, heapSys)
}

// GetPercentileResponseTime calculates percentile response time
func (m *Metrics) GetPercentileResponseTime(percentile float64) time.Duration {
	m.ResponseTimesMutex.RLock()
	defer m.ResponseTimesMutex.RUnlock()

	if len(m.ResponseTimes) == 0 {
		return 0
	}

	// Create a copy for sorting
	times := make([]time.Duration, len(m.ResponseTimes))
	copy(times, m.ResponseTimes)

	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})

	index := int(float64(len(times)-1) * percentile / 100.0)
	if index >= len(times) {
		index = len(times) - 1
	}

	return times[index]
}

// GetStatusCodeDistribution returns request count by status code
func (m *Metrics) GetStatusCodeDistribution() map[int]int64 {
	m.StatusMutex.RLock()
	defer m.StatusMutex.RUnlock()

	distribution := make(map[int]int64)
	for code, count := range m.RequestCountByStatus {
		distribution[code] = count
	}
	return distribution
}

// GetExternalAPIStats returns external API statistics
func (m *Metrics) GetExternalAPIStats() map[string]interface{} {
	m.ExternalAPIMutex.RLock()
	defer m.ExternalAPIMutex.RUnlock()

	stats := make(map[string]interface{})
	for api, requests := range m.ExternalAPIRequests {
		errors := m.ExternalAPIErrorCount[api]
		errorRate := float64(0)
		if requests > 0 {
			errorRate = float64(errors) / float64(requests) * 100
		}

		stats[api] = map[string]interface{}{
			"requests":   requests,
			"errors":     errors,
			"error_rate": errorRate,
		}
	}
	return stats
}

// GetStats returns current metrics statistics
func (m *Metrics) GetStats() map[string]interface{} {
	requests := atomic.LoadInt64(&m.RequestCount)
	errors := atomic.LoadInt64(&m.ErrorCount)
	cacheHits := atomic.LoadInt64(&m.CacheHits)
	cacheMisses := atomic.LoadInt64(&m.CacheMisses)
	githubCalls := atomic.LoadInt64(&m.GitHubAPICalls)
	xCalls := atomic.LoadInt64(&m.XAPICalls)
	avgResponseTime := atomic.LoadInt64(&m.AverageResponseTime)

	errorRate := float64(0)
	if requests > 0 {
		errorRate = float64(errors) / float64(requests) * 100
	}

	cacheHitRate := float64(0)
	totalCacheRequests := cacheHits + cacheMisses
	if totalCacheRequests > 0 {
		cacheHitRate = float64(cacheHits) / float64(totalCacheRequests) * 100
	}

	uptime := time.Since(m.StartTime)

	cbOpens := atomic.LoadInt64(&m.CircuitBreakerOpens)
	cbCloses := atomic.LoadInt64(&m.CircuitBreakerCloses)
	gcCount := atomic.LoadInt64(&m.GCCount)
	gcPauseTotalNs := atomic.LoadInt64(&m.GCPauseTotalNs)
	heapAlloc := atomic.LoadInt64(&m.HeapAlloc)
	heapSys := atomic.LoadInt64(&m.HeapSys)

	return map[string]interface{}{
		"uptime_seconds":         uptime.Seconds(),
		"total_requests":         requests,
		"error_count":            errors,
		"error_rate_percent":     errorRate,
		"cache_hits":             cacheHits,
		"cache_misses":           cacheMisses,
		"cache_hit_rate_percent": cacheHitRate,
		"github_api_calls":       githubCalls,
		"x_api_calls":            xCalls,
		"avg_response_time_ms":   float64(avgResponseTime) / 1000000,
		"start_time":             m.StartTime.Format(time.RFC3339),

		// Enhanced metrics
		"p50_response_time_ms":     float64(m.GetPercentileResponseTime(50)) / 1000000,
		"p95_response_time_ms":     float64(m.GetPercentileResponseTime(95)) / 1000000,
		"p99_response_time_ms":     float64(m.GetPercentileResponseTime(99)) / 1000000,
		"status_code_distribution": m.GetStatusCodeDistribution(),
		"external_api_stats":       m.GetExternalAPIStats(),

		// Circuit breaker metrics
		"circuit_breaker_opens":  cbOpens,
		"circuit_breaker_closes": cbCloses,

		// System metrics
		"go_gc_count":           gcCount,
		"go_gc_pause_total_ns":  gcPauseTotalNs,
		"go_heap_alloc_bytes":   heapAlloc,
		"go_heap_sys_bytes":     heapSys,
		"go_heap_usage_percent": float64(heapAlloc) / float64(heapSys) * 100,
	}
}

// Ensure Metrics implements cache.Metrics interface
var _ interface {
	IncrementCacheHit()
	IncrementCacheMiss()
} = (*Metrics)(nil)

// Reset resets all metrics (useful for testing)
func (m *Metrics) Reset() {
	atomic.StoreInt64(&m.RequestCount, 0)
	atomic.StoreInt64(&m.ErrorCount, 0)
	atomic.StoreInt64(&m.CacheHits, 0)
	atomic.StoreInt64(&m.CacheMisses, 0)
	atomic.StoreInt64(&m.GitHubAPICalls, 0)
	atomic.StoreInt64(&m.XAPICalls, 0)
	atomic.StoreInt64(&m.AverageResponseTime, 0)
	atomic.StoreInt64(&m.CircuitBreakerOpens, 0)
	atomic.StoreInt64(&m.CircuitBreakerCloses, 0)
	atomic.StoreInt64(&m.GCCount, 0)
	atomic.StoreInt64(&m.GCPauseTotalNs, 0)
	atomic.StoreInt64(&m.HeapAlloc, 0)
	atomic.StoreInt64(&m.HeapSys, 0)

	m.ResponseTimesMutex.Lock()
	m.ResponseTimes = m.ResponseTimes[:0]
	m.ResponseTimesMutex.Unlock()

	m.StatusMutex.Lock()
	m.RequestCountByStatus = make(map[int]int64)
	m.StatusMutex.Unlock()

	m.ExternalAPIMutex.Lock()
	m.ExternalAPIRequests = make(map[string]int64)
	m.ExternalAPIErrorCount = make(map[string]int64)
	m.ExternalAPIMutex.Unlock()

	m.RateLimitMutex.Lock()
	m.RateLimitEndpointBlocks = make(map[string]int64)
	m.RateLimitMutex.Unlock()

	m.StartTime = time.Now()
}

// IncrementRateLimitIPBlock increments IP-based rate limit blocks
func (m *Metrics) IncrementRateLimitIPBlock() {
	atomic.AddInt64(&m.RateLimitIPBlocks, 1)
}

// IncrementRateLimitUserBlock increments user-based rate limit blocks
func (m *Metrics) IncrementRateLimitUserBlock() {
	atomic.AddInt64(&m.RateLimitUserBlocks, 1)
}

// IncrementRateLimitRedisError increments Redis error count for rate limiting
func (m *Metrics) IncrementRateLimitRedisError() {
	atomic.AddInt64(&m.RateLimitRedisErrors, 1)
}

// IncrementRateLimitFallback increments fallback rate limiter usage count
func (m *Metrics) IncrementRateLimitFallback() {
	atomic.AddInt64(&m.RateLimitFallbackCount, 1)
}

// IncrementRateLimitEndpoint increments rate limit blocks for a specific endpoint
func (m *Metrics) IncrementRateLimitEndpoint(endpoint string) {
	m.RateLimitMutex.Lock()
	defer m.RateLimitMutex.Unlock()
	m.RateLimitEndpointBlocks[endpoint]++
}

// GetRateLimitStats returns rate limiting statistics
func (m *Metrics) GetRateLimitStats() map[string]interface{} {
	m.RateLimitMutex.RLock()
	endpointBlocksCopy := make(map[string]int64, len(m.RateLimitEndpointBlocks))
	for k, v := range m.RateLimitEndpointBlocks {
		endpointBlocksCopy[k] = v
	}
	m.RateLimitMutex.RUnlock()

	return map[string]interface{}{
		"ip_blocks":       atomic.LoadInt64(&m.RateLimitIPBlocks),
		"user_blocks":     atomic.LoadInt64(&m.RateLimitUserBlocks),
		"redis_errors":    atomic.LoadInt64(&m.RateLimitRedisErrors),
		"fallback_count":  atomic.LoadInt64(&m.RateLimitFallbackCount),
		"endpoint_blocks": endpointBlocksCopy,
	}
}

package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// MemoryStats tracks memory usage statistics
type MemoryStats struct {
	// Current memory usage
	Alloc      uint64 `json:"alloc_bytes"`
	TotalAlloc uint64 `json:"total_alloc_bytes"`
	Sys        uint64 `json:"sys_bytes"`
	Lookups    uint64 `json:"lookups"`
	Mallocs    uint64 `json:"mallocs"`
	Frees      uint64 `json:"frees"`

	// Heap statistics
	HeapAlloc    uint64 `json:"heap_alloc_bytes"`
	HeapSys      uint64 `json:"heap_sys_bytes"`
	HeapIdle     uint64 `json:"heap_idle_bytes"`
	HeapInuse    uint64 `json:"heap_inuse_bytes"`
	HeapReleased uint64 `json:"heap_released_bytes"`
	HeapObjects  uint64 `json:"heap_objects"`

	// Stack statistics
	StackInuse uint64 `json:"stack_inuse_bytes"`
	StackSys   uint64 `json:"stack_sys_bytes"`

	// Garbage collection statistics
	GCCPUFraction float64 `json:"gc_cpu_fraction"`
	NumGC         uint32  `json:"num_gc"`
	NumGoroutine  int     `json:"num_goroutine"`

	// Timestamps
	Timestamp time.Time `json:"timestamp"`
	mutex     sync.RWMutex
}

// MemoryMonitor monitors memory usage and GC performance
type MemoryMonitor struct {
	stats       *MemoryStats
	history     []MemoryStats
	maxHistory  int
	interval    time.Duration
	stopChannel chan struct{}
	gcThreshold uint64 // Trigger GC when heap exceeds this size (bytes)
	logger      *Logger
	mutex       sync.RWMutex
}

// NewMemoryMonitor creates a new memory monitor
func NewMemoryMonitor(interval time.Duration, gcThreshold uint64, logger *Logger) *MemoryMonitor {
	return &MemoryMonitor{
		stats:       &MemoryStats{},
		history:     make([]MemoryStats, 0),
		maxHistory:  100, // Keep last 100 measurements
		interval:    interval,
		stopChannel: make(chan struct{}),
		gcThreshold: gcThreshold,
		logger:      logger,
	}
}

// Start begins memory monitoring in a goroutine
func (mm *MemoryMonitor) Start() {
	go func() {
		ticker := time.NewTicker(mm.interval)
		defer ticker.Stop()

		slog.Info("Starting memory monitoring", "interval_ms", mm.interval.Milliseconds())

		for {
			select {
			case <-ticker.C:
				mm.collectStats()

			case <-mm.stopChannel:
				slog.Info("Memory monitoring stopped")
				return
			}
		}
	}()
}

// Stop stops memory monitoring
func (mm *MemoryMonitor) Stop() {
	close(mm.stopChannel)
}

// collectStats collects current memory statistics
func (mm *MemoryMonitor) collectStats() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	stats := MemoryStats{
		Alloc:         memStats.Alloc,
		TotalAlloc:    memStats.TotalAlloc,
		Sys:           memStats.Sys,
		Lookups:       memStats.Lookups,
		Mallocs:       memStats.Mallocs,
		Frees:         memStats.Frees,
		HeapAlloc:     memStats.HeapAlloc,
		HeapSys:       memStats.HeapSys,
		HeapIdle:      memStats.HeapIdle,
		HeapInuse:     memStats.HeapInuse,
		HeapReleased:  memStats.HeapReleased,
		HeapObjects:   memStats.HeapObjects,
		StackInuse:    memStats.StackInuse,
		StackSys:      memStats.StackSys,
		GCCPUFraction: memStats.GCCPUFraction,
		NumGC:         memStats.NumGC,
		NumGoroutine:  runtime.NumGoroutine(),
		Timestamp:     time.Now(),
	}

	mm.mutex.Lock()
	mm.stats = &stats

	// Add to history
	mm.history = append(mm.history, stats)
	if len(mm.history) > mm.maxHistory {
		mm.history = mm.history[1:] // Remove oldest entry
	}
	mm.mutex.Unlock()

	// Check if GC should be triggered
	if memStats.HeapAlloc > mm.gcThreshold {
		slog.Info("Triggering manual garbage collection",
			"heap_alloc_mb", memStats.HeapAlloc/(1024*1024),
			"gc_threshold_mb", mm.gcThreshold/(1024*1024))

		start := time.Now()
		runtime.GC()
		gcDuration := time.Since(start)

		mm.logger.PerformanceLogger("manual_gc", float64(gcDuration.Milliseconds()), "ms")
	}

	// Log memory stats periodically
	if stats.Timestamp.Second()%30 == 0 { // Log every 30 seconds
		mm.logMemoryStats(&stats)
	}
}

// logMemoryStats logs detailed memory statistics
func (mm *MemoryMonitor) logMemoryStats(stats *MemoryStats) {
	mm.logger.SystemLogger("memory_stats", fmt.Sprintf(
		"alloc:%dMB total:%dMB sys:%dMB heap:%dMB/%dMB gc:%d goroutines:%d",
		stats.Alloc/(1024*1024),
		stats.TotalAlloc/(1024*1024),
		stats.Sys/(1024*1024),
		stats.HeapInuse/(1024*1024),
		stats.HeapSys/(1024*1024),
		stats.NumGC,
		stats.NumGoroutine,
	))
}

// GetStats returns current memory statistics
func (mm *MemoryMonitor) GetStats() map[string]interface{} {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	// Calculate derived metrics
	heapUtilization := float64(0)
	if mm.stats.HeapSys > 0 {
		heapUtilization = float64(mm.stats.HeapInuse) / float64(mm.stats.HeapSys)
	}

	mallocRate := float64(0)
	if len(mm.history) >= 2 {
		timeDiff := mm.history[len(mm.history)-1].Timestamp.Sub(mm.history[0].Timestamp).Seconds()
		if timeDiff > 0 {
			mallocRate = float64(mm.stats.Mallocs-mm.history[0].Mallocs) / timeDiff
		}
	}

	return map[string]interface{}{
		"current": map[string]interface{}{
			"alloc_mb":        mm.stats.Alloc / (1024 * 1024),
			"total_alloc_mb":  mm.stats.TotalAlloc / (1024 * 1024),
			"sys_mb":          mm.stats.Sys / (1024 * 1024),
			"heap_alloc_mb":   mm.stats.HeapAlloc / (1024 * 1024),
			"heap_sys_mb":     mm.stats.HeapSys / (1024 * 1024),
			"heap_idle_mb":    mm.stats.HeapIdle / (1024 * 1024),
			"heap_inuse_mb":   mm.stats.HeapInuse / (1024 * 1024),
			"gc_cpu_fraction": mm.stats.GCCPUFraction,
			"num_gc":          mm.stats.NumGC,
			"num_goroutine":   mm.stats.NumGoroutine,
		},
		"derived": map[string]interface{}{
			"heap_utilization":    heapUtilization,
			"malloc_rate_per_sec": mallocRate,
			"gc_efficiency":       1.0 - mm.stats.GCCPUFraction,
		},
		"history_count":   len(mm.history),
		"gc_threshold_mb": mm.gcThreshold / (1024 * 1024),
	}
}

// GetHistory returns the memory statistics history
func (mm *MemoryMonitor) GetHistory() []MemoryStats {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	// Return a copy to prevent external modification
	history := make([]MemoryStats, len(mm.history))
	copy(history, mm.history)
	return history
}

// ForceGC forces a garbage collection cycle
func (mm *MemoryMonitor) ForceGC() {
	start := time.Now()
	runtime.GC()
	duration := time.Since(start)

	mm.logger.PerformanceLogger("forced_gc", float64(duration.Milliseconds()), "ms")

	slog.Info("Forced garbage collection completed", "duration_ms", duration.Milliseconds())
}

// OptimizeMemory performs memory optimization actions based on current stats
func (mm *MemoryMonitor) OptimizeMemory() {
	stats := mm.GetStats()

	// Check for memory pressure indicators
	heapUtilization := stats["derived"].(map[string]interface{})["heap_utilization"].(float64)
	gcEfficiency := stats["derived"].(map[string]interface{})["gc_efficiency"].(float64)

	if heapUtilization > 0.8 { // High heap utilization
		slog.Warn("High heap utilization detected", "utilization", heapUtilization)
		mm.ForceGC()
	}

	if gcEfficiency < 0.5 { // Inefficient GC
		slog.Warn("Inefficient garbage collection detected", "efficiency", gcEfficiency)
		// Could trigger more aggressive GC tuning here
	}

	// Log optimization actions
	mm.logger.SystemLogger("memory_optimization", fmt.Sprintf(
		"heap_utilization:%.2f gc_efficiency:%.2f action:monitoring",
		heapUtilization, gcEfficiency,
	))
}

// TuneGC adjusts garbage collection settings for better performance
func TuneGC(targetHeapSize uint64) {
	// Set GC target percentage (default is 100%)
	// Lower values trigger GC more aggressively
	gcPercent := 80
	if targetHeapSize > 100*1024*1024 { // > 100MB
		gcPercent = 60 // More aggressive GC for larger heaps
	}

	debug.SetGCPercent(gcPercent)

	// Memory limit setting would be here if supported by Go version
	// runtime.MemLimit() is read-only in current Go versions

	slog.Info("GC tuning applied",
		"gc_percent", gcPercent,
		"target_heap_mb", targetHeapSize/(1024*1024))
}

// BenchmarkMemoryOperations benchmarks memory-intensive operations
func BenchmarkMemoryOperations() {
	// Test memory allocation patterns
	dataSizes := []int{1024, 10240, 102400, 1024000} // 1KB to 1MB

	for _, size := range dataSizes {
		start := time.Now()

		// Allocate and manipulate memory
		data := make([]byte, size)
		for i := 0; i < size; i += 1024 {
			data[i] = byte(i % 256)
		}

		// Force some memory pressure
		_ = data

		duration := time.Since(start)
		throughput := float64(size) / duration.Seconds()

		slog.Info("Memory benchmark completed",
			"size_bytes", size,
			"duration_ms", duration.Milliseconds(),
			"throughput_mbps", throughput/(1024*1024))
	}
}

// MonitorMemoryPressure continuously monitors for memory pressure
func MonitorMemoryPressure(ctx context.Context, thresholdPercent float64, logger *Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			heapUtilization := float64(memStats.HeapInuse) / float64(memStats.HeapSys)

			if heapUtilization > thresholdPercent {
				logger.SystemLogger("memory_pressure", fmt.Sprintf(
					"utilization:%.2f inuse:%dMB sys:%dMB threshold:%.2f",
					heapUtilization,
					memStats.HeapInuse/(1024*1024),
					memStats.HeapSys/(1024*1024),
					thresholdPercent,
				))

				// Trigger GC when under pressure
				runtime.GC()
			}
		}
	}
}

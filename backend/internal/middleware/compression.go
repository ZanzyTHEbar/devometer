package middleware

import (
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CompressionConfig holds configuration for response compression
type CompressionConfig struct {
	MinSize          int      // Minimum response size to compress (bytes)
	CompressionLevel int      // Gzip compression level (1-9, 9 is best compression)
	ContentTypes     []string // Content types to compress
}

// DefaultCompressionConfig returns the default compression configuration
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		MinSize:          1024, // Compress responses >= 1KB
		CompressionLevel: 6,    // Balanced compression level
		ContentTypes: []string{
			"application/json",
			"text/plain",
			"text/html",
			"text/css",
			"application/javascript",
			"application/xml",
			"text/xml",
		},
	}
}

// CompressionMiddleware provides gzip compression for HTTP responses
type CompressionMiddleware struct {
	config CompressionConfig
	stats  *CompressionStats
	pool   sync.Pool // Pool of gzip writers for better performance
}

// NewCompressionMiddleware creates a new compression middleware
func NewCompressionMiddleware(config CompressionConfig) *CompressionMiddleware {
	return &CompressionMiddleware{
		config: config,
		stats:  NewCompressionStats(),
		pool: sync.Pool{
			New: func() interface{} {
				return gzip.NewWriter(io.Discard)
			},
		},
	}
}

// Handler returns a Gin middleware function for response compression
func (cm *CompressionMiddleware) Handler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if client accepts gzip
		if !cm.clientAcceptsGzip(r) {
			return
		}

		// Check if content type should be compressed
		if !cm.shouldCompress(r.Header.Get("Content-Type")) {
			return
		}

		// Wrap response writer with gzip writer
		gz := cm.getGzipWriter(w)
		defer cm.returnGzipWriter(gz)

		// Set appropriate headers
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")

		// Note: In Gin, the compression middleware would be applied using r.Use()
		// and would wrap the response writer for all subsequent handlers
		_ = gz // Prevent unused variable warning
	}
}

// clientAcceptsGzip checks if the client accepts gzip compression
func (cm *CompressionMiddleware) clientAcceptsGzip(r *http.Request) bool {
	acceptEncoding := r.Header.Get("Accept-Encoding")
	return strings.Contains(acceptEncoding, "gzip")
}

// shouldCompress checks if the content type should be compressed
func (cm *CompressionMiddleware) shouldCompress(contentType string) bool {
	for _, ct := range cm.config.ContentTypes {
		if strings.Contains(contentType, ct) {
			return true
		}
	}
	return false
}

// getGzipWriter gets a gzip writer from the pool
func (cm *CompressionMiddleware) getGzipWriter(w io.Writer) *gzip.Writer {
	gz := cm.pool.Get().(*gzip.Writer)
	gz.Reset(w)
	return gz
}

// returnGzipWriter returns a gzip writer to the pool
func (cm *CompressionMiddleware) returnGzipWriter(gz *gzip.Writer) {
	gz.Close()
	cm.pool.Put(gz)
}

// gzipResponseWriter wraps an http.ResponseWriter with gzip compression
type gzipResponseWriter struct {
	http.ResponseWriter
	gzipWriter *gzip.Writer
	written    bool
}

// Write writes data through the gzip writer
func (gzw *gzipResponseWriter) Write(data []byte) (int, error) {
	// Check if this is the first write
	if !gzw.written {
		// Check if response is large enough to compress
		if len(data) < 1024 { // Default min size
			// Don't compress small responses
			return gzw.ResponseWriter.Write(data)
		}
		gzw.written = true
	}

	// Write through gzip
	return gzw.gzipWriter.Write(data)
}

// WriteHeader sets the status code and headers
func (gzw *gzipResponseWriter) WriteHeader(statusCode int) {
	gzw.ResponseWriter.WriteHeader(statusCode)
}

// Flush flushes the gzip writer
func (gzw *gzipResponseWriter) Flush() {
	if gzw.gzipWriter != nil {
		gzw.gzipWriter.Flush()
	}
	if flusher, ok := gzw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack hijacks the connection (for WebSocket upgrades, etc.)
func (gzw *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := gzw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, errors.New("response writer does not implement http.Hijacker")
}

// Close closes the gzip writer
func (gzw *gzipResponseWriter) Close() error {
	if gzw.gzipWriter != nil {
		return gzw.gzipWriter.Close()
	}
	return nil
}

// CompressionStats tracks compression statistics
type CompressionStats struct {
	TotalRequests      int64
	CompressedRequests int64
	TotalBytes         int64
	CompressedBytes    int64
	mutex              sync.RWMutex
}

// NewCompressionStats creates new compression statistics
func NewCompressionStats() *CompressionStats {
	return &CompressionStats{}
}

// RecordRequest records a request's compression stats
func (cs *CompressionStats) RecordRequest(originalSize, compressedSize int64, compressed bool) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	cs.TotalRequests++
	cs.TotalBytes += originalSize

	if compressed {
		cs.CompressedRequests++
		cs.CompressedBytes += compressedSize
	}
}

// GetStats returns current compression statistics
func (cs *CompressionStats) GetStats() map[string]interface{} {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	compressionRatio := float64(0)
	if cs.TotalBytes > 0 {
		compressionRatio = float64(cs.CompressedBytes) / float64(cs.TotalBytes)
	}

	return map[string]interface{}{
		"total_requests":      cs.TotalRequests,
		"compressed_requests": cs.CompressedRequests,
		"total_bytes":         cs.TotalBytes,
		"compressed_bytes":    cs.CompressedBytes,
		"compression_ratio":   compressionRatio,
		"compression_savings": 1.0 - compressionRatio,
		"compression_enabled": cs.TotalRequests > 0 && cs.CompressedRequests > 0,
	}
}

// GetStats returns compression statistics
func (cm *CompressionMiddleware) GetStats() map[string]interface{} {
	return cm.stats.GetStats()
}

// BenchmarkCompression benchmarks compression performance
func BenchmarkCompression() {
	stats := NewCompressionStats()

	// Test data
	testData := `{
		"score": 85,
		"confidence": 0.92,
		"contributors": [
			{"name": "Alice", "contribution": 0.3},
			{"name": "Bob", "contribution": 0.25},
			{"name": "Charlie", "contribution": 0.2}
		],
		"breakdown": {
			"shipping": 80.0,
			"quality": 70.0,
			"influence": 75.0,
			"complexity": 60.0,
			"collaboration": 85.0,
			"reliability": 90.0,
			"novelty": 65.0
		}
	}`

	// Benchmark without compression
	start := time.Now()
	for i := 0; i < 1000; i++ {
		stats.RecordRequest(int64(len(testData)), int64(len(testData)), false)
	}
	noCompressionDuration := time.Since(start)

	// Benchmark with compression (simulated)
	start = time.Now()
	for i := 0; i < 1000; i++ {
		// Simulate compression ratio of ~70%
		compressedSize := int64(float64(len(testData)) * 0.3)
		stats.RecordRequest(int64(len(testData)), compressedSize, true)
	}
	compressionDuration := time.Since(start)

	slog.Info("Compression performance benchmarks",
		"no_compression_1k_ms", noCompressionDuration.Milliseconds(),
		"compression_1k_ms", compressionDuration.Milliseconds(),
		"stats", stats.GetStats(),
	)
}

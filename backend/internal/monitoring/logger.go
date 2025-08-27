package monitoring

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"time"
)

// Logger provides enhanced structured logging with context
type Logger struct {
	*slog.Logger
}

// NewLogger creates a new enhanced logger
func NewLogger() *Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Add timestamp in RFC3339 format
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "timestamp",
					Value: slog.StringValue(a.Value.Time().Format(time.RFC3339)),
				}
			}
			return a
		},
	})

	return &Logger{
		Logger: slog.New(handler),
	}
}

// RequestLogger logs HTTP request details
func (l *Logger) RequestLogger(method, path, ip, userAgent string, statusCode int, duration time.Duration) {
	l.Info("HTTP Request",
		"method", method,
		"path", path,
		"ip", ip,
		"user_agent", userAgent,
		"status_code", statusCode,
		"duration_ms", duration.Milliseconds(),
	)
}

// AnalysisLogger logs analysis operation details
func (l *Logger) AnalysisLogger(input, analysisType string, score float64, confidence float64, duration time.Duration, cacheHit bool) {
	l.Info("Analysis Completed",
		"input_length", len(input),
		"analysis_type", analysisType,
		"score", score,
		"confidence", confidence,
		"duration_ms", duration.Milliseconds(),
		"cache_hit", cacheHit,
	)
}

// APIErrorLogger logs API errors with context
func (l *Logger) APIErrorLogger(err error, method, path, ip string, statusCode int) {
	// Get caller information for better debugging
	_, file, line, ok := runtime.Caller(2)
	caller := "unknown"
	if ok {
		caller = file + ":" + string(rune(line))
	}

	l.Error("API Error",
		"error", err.Error(),
		"method", method,
		"path", path,
		"ip", ip,
		"status_code", statusCode,
		"caller", caller,
	)
}

// ExternalAPILogger logs external API calls
func (l *Logger) ExternalAPILogger(apiName, method, endpoint string, statusCode int, duration time.Duration, success bool) {
	level := slog.LevelInfo
	if !success {
		level = slog.LevelWarn
	}

	l.Log(context.Background(), level, "External API Call",
		"api_name", apiName,
		"method", method,
		"endpoint", endpoint,
		"status_code", statusCode,
		"duration_ms", duration.Milliseconds(),
		"success", success,
	)
}

// CacheLogger logs cache operations
func (l *Logger) CacheLogger(operation, key string, hit bool, itemCount int) {
	l.Debug("Cache Operation",
		"operation", operation,
		"key_hash", key[:8]+"...",
		"hit", hit,
		"cache_size", itemCount,
	)
}

// SystemLogger logs system-level events
func (l *Logger) SystemLogger(event, details string) {
	l.Info("System Event",
		"event", event,
		"details", details,
		"uptime", time.Since(startTime).String(),
	)
}

// SecurityLogger logs security-related events
func (l *Logger) SecurityLogger(event, ip, userAgent string, details map[string]interface{}) {
	attrs := []any{
		"event", event,
		"ip", ip,
		"user_agent", userAgent,
		"timestamp", time.Now().Format(time.RFC3339),
	}

	// Add additional details
	for key, value := range details {
		attrs = append(attrs, key, value)
	}

	l.Warn("Security Event", attrs...)
}

// PerformanceLogger logs performance metrics
func (l *Logger) PerformanceLogger(metric string, value float64, unit string) {
	l.Info("Performance Metric",
		"metric", metric,
		"value", value,
		"unit", unit,
		"timestamp", time.Now().Format(time.RFC3339),
	)
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level slog.Level) {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})
	l.Logger = slog.New(handler)
}

var startTime = time.Now()

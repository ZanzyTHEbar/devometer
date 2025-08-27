package monitoring

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// TraceID represents a unique trace identifier
type TraceID string

// SpanID represents a unique span identifier
type SpanID string

// TraceContext holds tracing information
type TraceContext struct {
	TraceID     TraceID           `json:"trace_id"`
	SpanID      SpanID            `json:"span_id"`
	ParentID    *SpanID           `json:"parent_id,omitempty"`
	ServiceName string            `json:"service_name"`
	Operation   string            `json:"operation"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     *time.Time        `json:"end_time,omitempty"`
	Duration    *time.Duration    `json:"duration,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Events      []TraceEvent      `json:"events,omitempty"`
	Error       string            `json:"error,omitempty"`
	Status      SpanStatus        `json:"status"`
}

// SpanStatus represents the status of a span
type SpanStatus string

const (
	SpanStatusOK      SpanStatus = "ok"
	SpanStatusError   SpanStatus = "error"
	SpanStatusTimeout SpanStatus = "timeout"
)

// TraceEvent represents an event within a trace
type TraceEvent struct {
	Name       string                 `json:"name"`
	Timestamp  time.Time              `json:"timestamp"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// Tracer manages distributed tracing
type Tracer struct {
	serviceName string
	logger      *Logger
	spans       map[SpanID]*TraceContext
	spansMutex  sync.RWMutex
}

// NewTracer creates a new tracer instance
func NewTracer(serviceName string, logger *Logger) *Tracer {
	return &Tracer{
		serviceName: serviceName,
		logger:      logger,
		spans:       make(map[SpanID]*TraceContext),
	}
}

// StartSpan starts a new trace span
func (t *Tracer) StartSpan(ctx context.Context, operation string, opts ...SpanOption) (*TraceContext, context.Context) {
	traceID := TraceID(t.getTraceIDFromContext(ctx))
	parentID := t.getParentSpanIDFromContext(ctx)
	spanID := t.generateSpanID()

	span := &TraceContext{
		TraceID:     traceID,
		SpanID:      spanID,
		ParentID:    parentID,
		ServiceName: t.serviceName,
		Operation:   operation,
		StartTime:   time.Now(),
		Tags:        make(map[string]string),
		Events:      []TraceEvent{},
		Status:      SpanStatusOK,
	}

	// Apply options
	for _, opt := range opts {
		opt(span)
	}

	// Store span
	t.spansMutex.Lock()
	t.spans[spanID] = span
	t.spansMutex.Unlock()

	// Add to context
	newCtx := context.WithValue(ctx, "trace_context", span)

	t.logger.SystemLogger("span_started", fmt.Sprintf("TraceID=%s SpanID=%s Operation=%s", traceID, spanID, operation))

	return span, newCtx
}

// EndSpan ends a trace span
func (t *Tracer) EndSpan(span *TraceContext, err error) {
	endTime := time.Now()
	duration := endTime.Sub(span.StartTime)

	span.EndTime = &endTime
	span.Duration = &duration

	if err != nil {
		span.Error = err.Error()
		span.Status = SpanStatusError
		t.logger.SystemLogger("span_error", fmt.Sprintf("TraceID=%s SpanID=%s Error=%s Duration=%v", span.TraceID, span.SpanID, err.Error(), duration))
	} else {
		t.logger.SystemLogger("span_completed", fmt.Sprintf("TraceID=%s SpanID=%s Duration=%v", span.TraceID, span.SpanID, duration))
	}

	// Log the complete trace
	t.logSpan(span)

	// Clean up
	t.spansMutex.Lock()
	delete(t.spans, span.SpanID)
	t.spansMutex.Unlock()
}

// AddEvent adds an event to a span
func (t *Tracer) AddEvent(span *TraceContext, name string, attributes map[string]interface{}) {
	event := TraceEvent{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attributes,
	}

	span.Events = append(span.Events, event)
}

// SetTag sets a tag on a span
func (t *Tracer) SetTag(span *TraceContext, key, value string) {
	if span.Tags == nil {
		span.Tags = make(map[string]string)
	}
	span.Tags[key] = value
}

// SpanOption represents an option for configuring a span
type SpanOption func(*TraceContext)

// WithTag sets a tag on the span
func WithTag(key, value string) SpanOption {
	return func(span *TraceContext) {
		if span.Tags == nil {
			span.Tags = make(map[string]string)
		}
		span.Tags[key] = value
	}
}

// WithParentSpanID sets the parent span ID
func WithParentSpanID(parentID SpanID) SpanOption {
	return func(span *TraceContext) {
		span.ParentID = &parentID
	}
}

// generateSpanID generates a unique span ID
func (t *Tracer) generateSpanID() SpanID {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return SpanID(fmt.Sprintf("%x", bytes))
}

// getTraceIDFromContext extracts trace ID from context
func (t *Tracer) getTraceIDFromContext(ctx context.Context) string {
	if span := t.getSpanFromContext(ctx); span != nil {
		return string(span.TraceID)
	}

	// Generate new trace ID
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%x", bytes)
}

// getParentSpanIDFromContext extracts parent span ID from context
func (t *Tracer) getParentSpanIDFromContext(ctx context.Context) *SpanID {
	if span := t.getSpanFromContext(ctx); span != nil {
		return &span.SpanID
	}
	return nil
}

// getSpanFromContext extracts span from context
func (t *Tracer) getSpanFromContext(ctx context.Context) *TraceContext {
	if span, ok := ctx.Value("trace_context").(*TraceContext); ok {
		return span
	}
	return nil
}

// logSpan logs span information
func (t *Tracer) logSpan(span *TraceContext) {
	logEntry := []any{
		"trace_id", span.TraceID,
		"span_id", span.SpanID,
		"service", span.ServiceName,
		"operation", span.Operation,
		"start_time", span.StartTime.Format(time.RFC3339),
		"status", span.Status,
	}

	if span.ParentID != nil {
		logEntry = append(logEntry, "parent_id", *span.ParentID)
	}

	if span.Duration != nil {
		logEntry = append(logEntry, "duration_ms", span.Duration.Milliseconds())
	}

	if span.Error != "" {
		logEntry = append(logEntry, "error", span.Error)
	}

	if len(span.Tags) > 0 {
		for k, v := range span.Tags {
			logEntry = append(logEntry, fmt.Sprintf("tag_%s", k), v)
		}
	}

	if len(span.Events) > 0 {
		logEntry = append(logEntry, "event_count", len(span.Events))
	}

	t.logger.Info("Trace Span", logEntry...)
}

// TracingMiddleware creates Gin middleware for distributed tracing
func TracingMiddleware(tracer *Tracer) gin.HandlerFunc {
	return func(c *gin.Context) {
		operation := fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)

		span, ctx := tracer.StartSpan(c.Request.Context(), operation,
			WithTag("http.method", c.Request.Method),
			WithTag("http.url", c.Request.URL.String()),
			WithTag("http.user_agent", c.GetHeader("User-Agent")),
			WithTag("client_ip", c.ClientIP()),
		)

		// Add span to Gin context
		c.Set("trace_context", span)

		// Add trace headers to response
		c.Header("X-Trace-ID", string(span.TraceID))
		c.Header("X-Span-ID", string(span.SpanID))

		// Replace request context
		c.Request = c.Request.WithContext(ctx)

		// Track request processing
		tracer.AddEvent(span, "request_started", map[string]interface{}{
			"method": c.Request.Method,
			"path":   c.Request.URL.Path,
			"query":  c.Request.URL.RawQuery,
		})

		// Process request
		c.Next()

		// Track response
		tracer.AddEvent(span, "request_completed", map[string]interface{}{
			"status_code": c.Writer.Status(),
			"size_bytes":  c.Writer.Size(),
		})

		// Set tags based on response
		tracer.SetTag(span, "http.status_code", fmt.Sprintf("%d", c.Writer.Status()))

		if c.Writer.Status() >= 400 {
			tracer.SetTag(span, "error", "true")
		}

		// Handle errors
		if len(c.Errors) > 0 {
			var errMsgs []string
			for _, err := range c.Errors {
				errMsgs = append(errMsgs, err.Error())
			}
			tracer.SetTag(span, "gin_errors", fmt.Sprintf("%v", errMsgs))
		}

		// End span
		var spanErr error
		if len(c.Errors) > 0 {
			spanErr = fmt.Errorf("request errors: %v", c.Errors)
		}

		tracer.EndSpan(span, spanErr)
	}
}

// GetSpanFromGinContext extracts span from Gin context
func GetSpanFromGinContext(c *gin.Context) *TraceContext {
	if span, exists := c.Get("trace_context"); exists {
		if traceSpan, ok := span.(*TraceContext); ok {
			return traceSpan
		}
	}
	return nil
}

// GetTracerFromContext gets tracer from context (helper function)
func GetTracerFromContext(ctx context.Context, tracer *Tracer) *TraceContext {
	return tracer.getSpanFromContext(ctx)
}

// Global tracer instance
var globalTracer *Tracer

// InitGlobalTracer initializes the global tracer
func InitGlobalTracer(serviceName string, logger *Logger) {
	globalTracer = NewTracer(serviceName, logger)
}

// GetGlobalTracer returns the global tracer
func GetGlobalTracer() *Tracer {
	return globalTracer
}

// GetSpans returns all active spans (for debugging/monitoring)
func (t *Tracer) GetSpans() map[SpanID]*TraceContext {
	t.spansMutex.RLock()
	defer t.spansMutex.RUnlock()

	spans := make(map[SpanID]*TraceContext)
	for id, span := range t.spans {
		spans[id] = span
	}
	return spans
}

// GetSpanCount returns the number of active spans
func (t *Tracer) GetSpanCount() int {
	t.spansMutex.RLock()
	defer t.spansMutex.RUnlock()
	return len(t.spans)
}

// TraceFunction is a helper for tracing function calls
func TraceFunction(ctx context.Context, tracer *Tracer, operation string, fn func(context.Context) error) error {
	span, spanCtx := tracer.StartSpan(ctx, operation)

	defer func() {
		if r := recover(); r != nil {
			tracer.SetTag(span, "panic", "true")
			tracer.EndSpan(span, fmt.Errorf("panic: %v", r))
			panic(r) // Re-panic after tracing
		}
	}()

	err := fn(spanCtx)
	tracer.EndSpan(span, err)

	return err
}

package errors

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/ZanzyTHEbar/errbuilder-go"
	"github.com/gin-gonic/gin"
)

// ErrorCategory defines the type of error for proper handling
type ErrorCategory string

const (
	CategoryValidation    ErrorCategory = "validation"
	CategoryNetwork       ErrorCategory = "network"
	CategoryTimeout       ErrorCategory = "timeout"
	CategoryRateLimit     ErrorCategory = "rate_limit"
	CategoryInternal      ErrorCategory = "internal"
	CategoryExternalAPI   ErrorCategory = "external_api"
	CategoryConfiguration ErrorCategory = "configuration"
)

// AppError wraps errbuilder error with additional context for backward compatibility
type AppError struct {
	*errbuilder.ErrBuilder
	Category   ErrorCategory `json:"category"`
	HTTPStatus int           `json:"http_status"`
	Timestamp  time.Time     `json:"timestamp"`
	RequestID  string        `json:"request_id,omitempty"`
	StackTrace string        `json:"stack_trace,omitempty"`
}

// Error implements the error interface with backward compatibility
func (e *AppError) Error() string {
	// Provide backward compatibility by returning the old format
	// Map errbuilder codes to legacy error codes for display
	codeStr := "UNKNOWN_ERROR"
	switch e.ErrBuilder.ErrCode() {
	case errbuilder.CodeInvalidArgument:
		codeStr = "VALIDATION_ERROR"
	case errbuilder.CodeUnavailable:
		codeStr = "NETWORK_ERROR"
	case errbuilder.CodeDeadlineExceeded:
		codeStr = "TIMEOUT_ERROR"
	case errbuilder.CodeResourceExhausted:
		codeStr = "RATE_LIMIT_EXCEEDED"
	case errbuilder.CodeInternal:
		codeStr = "INTERNAL_ERROR"
	case errbuilder.CodeFailedPrecondition:
		codeStr = "CONFIGURATION_ERROR"
	}

	return fmt.Sprintf("[%s] %s", codeStr, e.ErrBuilder.Msg)
}

// Unwrap returns the underlying cause
func (e *AppError) Unwrap() error {
	return e.ErrBuilder.Unwrap()
}

// NewAppError creates an AppError from errbuilder with additional context
func NewAppError(builder *errbuilder.ErrBuilder, category ErrorCategory, httpStatus int) *AppError {
	return &AppError{
		ErrBuilder: builder,
		Category:   category,
		HTTPStatus: httpStatus,
		Timestamp:  time.Now(),
	}
}

// NewValidationError creates a validation error using errbuilder
func NewValidationError(message string, details ...interface{}) *AppError {
	detailStr := ""
	if len(details) > 0 {
		detailStr = fmt.Sprintf("%v", details[0])
	}

	builder := errbuilder.New().
		WithCode(errbuilder.CodeInvalidArgument).
		WithMsg(message)

	if detailStr != "" {
		errorMap := errbuilder.ErrorMap{}
		errorMap.Set("validation_details", errors.New(detailStr))
		builder = builder.WithDetails(errbuilder.NewErrDetails(errorMap))
	}

	return NewAppError(builder, CategoryValidation, http.StatusBadRequest)
}

// NewNetworkError creates a network error using errbuilder
func NewNetworkError(message string, cause error) *AppError {
	builder := errbuilder.New().
		WithCode(errbuilder.CodeUnavailable).
		WithMsg(message)

	if cause != nil {
		builder = builder.WithCause(cause)
	}

	return NewAppError(builder, CategoryNetwork, http.StatusBadGateway)
}

// NewTimeoutError creates a timeout error using errbuilder
func NewTimeoutError(message string, cause error) *AppError {
	builder := errbuilder.New().
		WithCode(errbuilder.CodeDeadlineExceeded).
		WithMsg(message)

	if cause != nil {
		builder = builder.WithCause(cause)
	}

	return NewAppError(builder, CategoryTimeout, http.StatusGatewayTimeout)
}

// NewRateLimitError creates a rate limit error using errbuilder
func NewRateLimitError(retryAfter string) *AppError {
	errorMap := errbuilder.ErrorMap{}
	errorMap.Set("retry_after", errors.New(retryAfter))

	builder := errbuilder.New().
		WithCode(errbuilder.CodeResourceExhausted).
		WithMsg("Rate limit exceeded").
		WithDetails(errbuilder.NewErrDetails(errorMap))

	return NewAppError(builder, CategoryRateLimit, http.StatusTooManyRequests)
}

// NewExternalAPIError creates an external API error using errbuilder
func NewExternalAPIError(apiName string, cause error) *AppError {
	errorMap := errbuilder.ErrorMap{}
	errorMap.Set("api_name", errors.New(apiName))

	builder := errbuilder.New().
		WithCode(errbuilder.CodeUnavailable).
		WithMsg(fmt.Sprintf("%s API error", apiName)).
		WithDetails(errbuilder.NewErrDetails(errorMap))

	if cause != nil {
		builder = builder.WithCause(cause)
	}

	return NewAppError(builder, CategoryExternalAPI, http.StatusBadGateway)
}

// NewInternalError creates an internal server error using errbuilder
func NewInternalError(message string, cause error) *AppError {
	errorMap := errbuilder.ErrorMap{}
	errorMap.Set("internal_details", errors.New(message))

	builder := errbuilder.New().
		WithCode(errbuilder.CodeInternal).
		WithMsg("Internal server error").
		WithDetails(errbuilder.NewErrDetails(errorMap))

	if cause != nil {
		builder = builder.WithCause(cause)
	}

	appErr := NewAppError(builder, CategoryInternal, http.StatusInternalServerError)

	// Capture stack trace in development/debug mode
	if gin.Mode() == gin.DebugMode || gin.Mode() == gin.TestMode {
		appErr.StackTrace = captureStackTrace()
	}

	return appErr
}

// NewConfigurationError creates a configuration error using errbuilder
func NewConfigurationError(message string, cause error) *AppError {
	errorMap := errbuilder.ErrorMap{}
	errorMap.Set("config_details", errors.New(message))

	builder := errbuilder.New().
		WithCode(errbuilder.CodeFailedPrecondition).
		WithMsg("Configuration error").
		WithDetails(errbuilder.NewErrDetails(errorMap))

	if cause != nil {
		builder = builder.WithCause(cause)
	}

	return NewAppError(builder, CategoryConfiguration, http.StatusInternalServerError)
}

// captureStackTrace captures a stack trace for debugging
func captureStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// ErrorHandler is a Gin middleware that provides centralized error handling
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			// Convert to AppError if it's not already
			appErr := ToAppError(err)

			// Log the error
			LogError(c, appErr)

			// Send structured error response
			c.JSON(appErr.HTTPStatus, appErr)
			return
		}
	}
}

// RecoveryHandler provides panic recovery with structured error responses
func RecoveryHandler() gin.HandlerFunc {
	return gin.RecoveryWithWriter(nil, func(c *gin.Context, err interface{}) {
		appErr := NewInternalError(
			fmt.Sprintf("Panic recovered: %v", err),
			fmt.Errorf("%v", err),
		)

		// Capture stack trace
		appErr.StackTrace = captureStackTrace()

		LogError(c, appErr)
		c.JSON(appErr.HTTPStatus, appErr)
	})
}

// ToAppError converts any error to an AppError
func ToAppError(err error) *AppError {
	if err == nil {
		return nil
	}

	// If it's already an AppError, return it
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}

	// Check if it's already an errbuilder error
	if ebErr, ok := err.(*errbuilder.ErrBuilder); ok {
		return NewAppError(ebErr, CategoryInternal, http.StatusInternalServerError)
	}

	// Check for specific error types
	errMsg := err.Error()

	// Network errors
	if strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "no such host") ||
		strings.Contains(errMsg, "network is unreachable") {
		return NewNetworkError("Network connection failed", err)
	}

	// Timeout errors
	if strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "deadline exceeded") {
		return NewTimeoutError("Request timeout", err)
	}

	// Context cancellation
	if errors.Is(err, context.Canceled) {
		return NewTimeoutError("Request cancelled", err)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return NewTimeoutError("Request deadline exceeded", err)
	}

	// Default to internal error
	return NewInternalError("An unexpected error occurred", err)
}

// LogError logs an error with appropriate level and context
func LogError(c *gin.Context, err *AppError) {
	// Get request context
	ip := c.ClientIP()
	method := c.Request.Method
	path := c.Request.URL.Path
	requestID := c.GetHeader("X-Request-ID")

	// Get error details from errbuilder
	errorCode := err.ErrBuilder.ErrCode()
	errorMsg := err.ErrBuilder.Msg
	errorDetails := err.ErrBuilder.Details

	// Create log entry
	logEntry := slog.With(
		"error_category", err.Category,
		"error_code", errorCode,
		"http_status", err.HTTPStatus,
		"ip", ip,
		"method", method,
		"path", path,
		"request_id", requestID,
	)

	// Log based on error category
	switch err.Category {
	case CategoryValidation, CategoryRateLimit:
		if len(errorDetails.Errors) > 0 {
			logEntry.Warn(errorMsg, "details", errorDetails.Errors)
		} else {
			logEntry.Warn(errorMsg)
		}
	case CategoryNetwork, CategoryTimeout, CategoryExternalAPI:
		if cause := err.ErrBuilder.Unwrap(); cause != nil {
			logEntry.Info(errorMsg, "cause", cause)
		} else {
			logEntry.Info(errorMsg)
		}
	case CategoryConfiguration:
		if cause := err.ErrBuilder.Unwrap(); cause != nil {
			logEntry.Error(errorMsg, "cause", cause)
		} else {
			logEntry.Error(errorMsg)
		}
	default:
		if cause := err.ErrBuilder.Unwrap(); cause != nil {
			logEntry.Error(errorMsg, "cause", cause)
		} else {
			logEntry.Error(errorMsg)
		}
	}

	// Log stack trace in development
	if err.StackTrace != "" && (gin.Mode() == gin.DebugMode || gin.Mode() == gin.TestMode) {
		logEntry.Debug("stack_trace", "trace", err.StackTrace)
	}
}

// IsRetryableError checks if an error should trigger a retry
func IsRetryableError(err error) bool {
	appErr := ToAppError(err)

	switch appErr.Category {
	case CategoryNetwork, CategoryTimeout, CategoryExternalAPI:
		return true
	case CategoryRateLimit:
		// Rate limits might be retryable with backoff
		return true
	default:
		return false
	}
}

// IsRetryableErrorFromBuilder checks if an errbuilder error should trigger a retry
func IsRetryableErrorFromBuilder(err *errbuilder.ErrBuilder) bool {
	if err == nil {
		return false
	}

	code := err.ErrCode()

	// Map errbuilder codes to retryable categories
	switch code {
	case errbuilder.CodeUnavailable, errbuilder.CodeDeadlineExceeded, errbuilder.CodeResourceExhausted:
		return true
	default:
		return false
	}
}

// GetRetryDelay returns appropriate retry delay based on error type
func GetRetryDelay(err error, attempt int) time.Duration {
	appErr := ToAppError(err)

	baseDelay := time.Duration(100*attempt) * time.Millisecond

	switch appErr.Category {
	case CategoryRateLimit:
		// For rate limits, use longer delay
		return time.Duration(attempt*attempt) * time.Second
	case CategoryNetwork, CategoryTimeout:
		// Exponential backoff for network issues
		return baseDelay * time.Duration(1<<attempt)
	case CategoryExternalAPI:
		// Moderate backoff for API errors
		return baseDelay * time.Duration(attempt)
	default:
		return baseDelay
	}
}

// GetRetryDelayFromBuilder returns appropriate retry delay based on errbuilder error
func GetRetryDelayFromBuilder(err *errbuilder.ErrBuilder, attempt int) time.Duration {
	if err == nil {
		return time.Duration(100*attempt) * time.Millisecond
	}

	baseDelay := time.Duration(100*attempt) * time.Millisecond
	code := err.ErrCode()

	switch code {
	case errbuilder.CodeResourceExhausted:
		// For rate limits, use longer delay
		return time.Duration(attempt*attempt) * time.Second
	case errbuilder.CodeUnavailable, errbuilder.CodeDeadlineExceeded:
		// Exponential backoff for network issues
		return baseDelay * time.Duration(1<<attempt)
	default:
		return baseDelay
	}
}

// WrapError wraps an error with additional context
func WrapError(err error, message string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	contextMsg := fmt.Sprintf(message, args...)
	return fmt.Errorf("%s: %w", contextMsg, err)
}

// SafeClose safely closes a resource and logs any errors
func SafeClose(closer interface{ Close() error }, resourceName string) {
	if closer == nil {
		return
	}

	if err := closer.Close(); err != nil {
		slog.Warn("Failed to close resource",
			"resource", resourceName,
			"error", err)
	}
}

// SafeExecute executes a function and recovers from panics
func SafeExecute(fn func(), panicHandler func(interface{})) {
	defer func() {
		if r := recover(); r != nil {
			if panicHandler != nil {
				panicHandler(r)
			} else {
				slog.Error("Panic in safe execution", "panic", r)
			}
		}
	}()

	fn()
}

// NewValidationErrorWithMap creates a validation error using ErrorMap for multiple validation issues
func NewValidationErrorWithMap(validationErrors map[string]string) *AppError {
	errMap := errbuilder.ErrorMap{}

	for field, message := range validationErrors {
		errMap.Set(field, errbuilder.New().
			WithCode(errbuilder.CodeInvalidArgument).
			WithMsg(message))
	}

	builder := errbuilder.New().
		WithCode(errbuilder.CodeInvalidArgument).
		WithMsg("Multiple validation errors").
		WithDetails(errbuilder.NewErrDetails(errMap))

	return NewAppError(builder, CategoryValidation, http.StatusBadRequest)
}

// NewBuilder creates a new errbuilder.ErrBuilder for custom error construction
func NewBuilder() *errbuilder.ErrBuilder {
	return errbuilder.New()
}

// NewErrorMap creates a new ErrorMap for collecting multiple errors
func NewErrorMap() errbuilder.ErrorMap {
	return errbuilder.ErrorMap{}
}

// BuildValidationError creates a validation error using errbuilder directly
func BuildValidationError(message string, details map[string]interface{}) *AppError {
	builder := errbuilder.New().
		WithCode(errbuilder.CodeInvalidArgument).
		WithMsg(message)

	if details != nil {
		errorMap := errbuilder.ErrorMap{}
		for key, value := range details {
			errorMap.Set(key, fmt.Errorf("%v", value))
		}
		builder = builder.WithDetails(errbuilder.NewErrDetails(errorMap))
	}

	return NewAppError(builder, CategoryValidation, http.StatusBadRequest)
}

// BuildNetworkError creates a network error using errbuilder directly
func BuildNetworkError(message string, cause error, details map[string]interface{}) *AppError {
	builder := errbuilder.New().
		WithCode(errbuilder.CodeUnavailable).
		WithMsg(message)

	if cause != nil {
		builder = builder.WithCause(cause)
	}

	if details != nil {
		errorMap := errbuilder.ErrorMap{}
		for key, value := range details {
			errorMap.Set(key, fmt.Errorf("%v", value))
		}
		builder = builder.WithDetails(errbuilder.NewErrDetails(errorMap))
	}

	return NewAppError(builder, CategoryNetwork, http.StatusBadGateway)
}

// BuildTimeoutError creates a timeout error using errbuilder directly
func BuildTimeoutError(message string, cause error, timeoutDuration time.Duration) *AppError {
	builder := errbuilder.New().
		WithCode(errbuilder.CodeDeadlineExceeded).
		WithMsg(message)

	if cause != nil {
		builder = builder.WithCause(cause)
	}

	errorMap := errbuilder.ErrorMap{}
	errorMap.Set("timeout_duration", errors.New(timeoutDuration.String()))
	builder = builder.WithDetails(errbuilder.NewErrDetails(errorMap))

	return NewAppError(builder, CategoryTimeout, http.StatusGatewayTimeout)
}

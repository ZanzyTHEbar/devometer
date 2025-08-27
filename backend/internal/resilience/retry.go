package resilience

import (
	"context"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/errors"
)

// RetryConfig holds configuration for retry behavior
type RetryConfig struct {
	MaxAttempts     int              `json:"max_attempts"`
	InitialDelay    time.Duration    `json:"initial_delay"`
	MaxDelay        time.Duration    `json:"max_delay"`
	BackoffFactor   float64          `json:"backoff_factor"`
	JitterEnabled   bool             `json:"jitter_enabled"`
	RetryableErrors func(error) bool `json:"-"` // Function to determine if error is retryable
}

// DefaultRetryConfig returns sensible defaults for retry behavior
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		JitterEnabled: true,
		RetryableErrors: func(err error) bool {
			return errors.IsRetryableError(err)
		},
	}
}

// RetryableFunc represents a function that can be retried
type RetryableFunc func() error

// RetryWithConfig executes a function with retry logic using custom configuration
func RetryWithConfig(ctx context.Context, config RetryConfig, fn RetryableFunc) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the function
		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !config.RetryableErrors(err) {
			break // Don't retry non-retryable errors
		}

		// Don't delay on the last attempt
		if attempt == config.MaxAttempts-1 {
			break
		}

		// Calculate delay for next attempt
		delay := calculateDelay(config, attempt)

		// Wait before retrying, but respect context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// Retry executes a function with retry logic using default configuration
func Retry(ctx context.Context, fn RetryableFunc) error {
	return RetryWithConfig(ctx, DefaultRetryConfig(), fn)
}

// RetryWithBackoff executes a function with exponential backoff retry
func RetryWithBackoff(ctx context.Context, maxAttempts int, initialDelay time.Duration, fn RetryableFunc) error {
	config := DefaultRetryConfig()
	config.MaxAttempts = maxAttempts
	config.InitialDelay = initialDelay

	return RetryWithConfig(ctx, config, fn)
}

// calculateDelay computes the delay for the next retry attempt
func calculateDelay(config RetryConfig, attempt int) time.Duration {
	// Exponential backoff: initial_delay * (backoff_factor ^ attempt)
	delay := time.Duration(float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt)))

	// Cap at max delay
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	// Add jitter to prevent thundering herd
	if config.JitterEnabled {
		jitter := time.Duration(rand.Int63n(int64(delay / 10))) // Up to 10% jitter
		delay += jitter
	}

	return delay
}

// RetryableHTTPFunc represents an HTTP function that can be retried
type RetryableHTTPFunc func() (*http.Response, error)

// RetryHTTP executes an HTTP request with retry logic
func RetryHTTP(ctx context.Context, config RetryConfig, fn RetryableHTTPFunc) (*http.Response, error) {
	var lastResp *http.Response
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Execute the HTTP request
		resp, err := fn()
		if err == nil {
			// Check for successful status codes
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return resp, nil
			}

			// Check if status code is retryable
			if !isRetryableHTTPStatus(resp.StatusCode) {
				return resp, nil // Don't retry non-retryable status codes
			}

			lastResp = resp
			lastErr = NewHTTPError(resp.StatusCode, resp.Status)
		} else {
			lastErr = err

			// Check if error is retryable
			if !config.RetryableErrors(err) {
				return nil, err
			}
		}

		// Don't delay on the last attempt
		if attempt == config.MaxAttempts-1 {
			break
		}

		// Calculate delay for next attempt
		delay := calculateDelay(config, attempt)

		// Wait before retrying, but respect context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastResp, lastErr
}

// isRetryableHTTPStatus checks if an HTTP status code should trigger a retry
func isRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case 408, 429: // Request Timeout, Too Many Requests
		return true
	case 500, 502, 503, 504: // Server errors
		return true
	default:
		return false
	}
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	StatusCode int
	Status     string
	Message    string
}

func (e *HTTPError) Error() string {
	return e.Message
}

// NewHTTPError creates a new HTTP error
func NewHTTPError(statusCode int, status string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Status:     status,
		Message:    status,
	}
}

// RetryPolicy defines different retry strategies
type RetryPolicy struct {
	Name   string
	Config RetryConfig
}

// Common retry policies
var (
	// FastRetryPolicy for quick-retry scenarios
	FastRetryPolicy = RetryPolicy{
		Name: "fast",
		Config: RetryConfig{
			MaxAttempts:   3,
			InitialDelay:  50 * time.Millisecond,
			MaxDelay:      1 * time.Second,
			BackoffFactor: 2.0,
			JitterEnabled: true,
		},
	}

	// StandardRetryPolicy for general use cases
	StandardRetryPolicy = RetryPolicy{
		Name: "standard",
		Config: RetryConfig{
			MaxAttempts:   3,
			InitialDelay:  100 * time.Millisecond,
			MaxDelay:      10 * time.Second,
			BackoffFactor: 2.0,
			JitterEnabled: true,
		},
	}

	// SlowRetryPolicy for external APIs that need longer delays
	SlowRetryPolicy = RetryPolicy{
		Name: "slow",
		Config: RetryConfig{
			MaxAttempts:   5,
			InitialDelay:  1 * time.Second,
			MaxDelay:      30 * time.Second,
			BackoffFactor: 1.5,
			JitterEnabled: true,
		},
	}
)

// RetryWithPolicy executes a function with a predefined retry policy
func RetryWithPolicy(ctx context.Context, policy RetryPolicy, fn RetryableFunc) error {
	policy.Config.RetryableErrors = DefaultRetryConfig().RetryableErrors
	return RetryWithConfig(ctx, policy.Config, fn)
}

// RetryManager manages retry policies for different services
type RetryManager struct {
	policies map[string]RetryPolicy
}

// NewRetryManager creates a new retry manager
func NewRetryManager() *RetryManager {
	return &RetryManager{
		policies: make(map[string]RetryPolicy),
	}
}

// RegisterPolicy registers a retry policy for a service
func (rm *RetryManager) RegisterPolicy(serviceName string, policy RetryPolicy) {
	rm.policies[serviceName] = policy
}

// GetPolicy returns the retry policy for a service, or standard policy if not found
func (rm *RetryManager) GetPolicy(serviceName string) RetryPolicy {
	if policy, exists := rm.policies[serviceName]; exists {
		return policy
	}
	return StandardRetryPolicy
}

// Execute executes a function with retry using the appropriate policy for the service
func (rm *RetryManager) Execute(ctx context.Context, serviceName string, fn RetryableFunc) error {
	policy := rm.GetPolicy(serviceName)
	return RetryWithPolicy(ctx, policy, fn)
}

// Global retry manager instance
var globalRetryManager = NewRetryManager()

// RegisterServicePolicy registers a retry policy for a service globally
func RegisterServicePolicy(serviceName string, policy RetryPolicy) {
	globalRetryManager.RegisterPolicy(serviceName, policy)
}

// ExecuteWithRetry executes a function with retry using the appropriate policy
func ExecuteWithRetry(ctx context.Context, serviceName string, fn RetryableFunc) error {
	return globalRetryManager.Execute(ctx, serviceName, fn)
}

// HTTPExecuteWithRetry executes an HTTP request with retry using the appropriate policy
func HTTPExecuteWithRetry(ctx context.Context, serviceName string, fn RetryableHTTPFunc) (*http.Response, error) {
	policy := globalRetryManager.GetPolicy(serviceName)
	policy.Config.RetryableErrors = DefaultRetryConfig().RetryableErrors

	return RetryHTTP(ctx, policy.Config, fn)
}

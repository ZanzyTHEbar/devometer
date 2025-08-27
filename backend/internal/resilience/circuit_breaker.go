package resilience

import (
	"sync/atomic"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int32

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreakerConfig holds configuration for the circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold int           `json:"failure_threshold"` // Number of failures before opening
	RecoveryTimeout  time.Duration `json:"recovery_timeout"`  // Time to wait before attempting recovery
	SuccessThreshold int           `json:"success_threshold"` // Number of successes needed to close circuit
}

// CircuitBreaker implements a circuit breaker pattern for external service calls
type CircuitBreaker struct {
	config      CircuitBreakerConfig
	state       int32
	failures    int32
	successes   int32
	lastFailure time.Time
	nextAttempt time.Time
}

// NewCircuitBreaker creates a new circuit breaker with default configuration
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.FailureThreshold == 0 {
		config.FailureThreshold = 5
	}
	if config.RecoveryTimeout == 0 {
		config.RecoveryTimeout = 30 * time.Second
	}
	if config.SuccessThreshold == 0 {
		config.SuccessThreshold = 3
	}

	return &CircuitBreaker{
		config: config,
		state:  int32(StateClosed),
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))

	switch state {
	case StateOpen:
		if time.Now().Before(cb.nextAttempt) {
			return NewCircuitBreakerError("circuit breaker is open", state)
		}
		// Transition to half-open
		atomic.StoreInt32(&cb.state, int32(StateHalfOpen))
		atomic.StoreInt32(&cb.successes, 0)
		fallthrough

	case StateHalfOpen, StateClosed:
		err := fn()

		if err != nil {
			cb.onFailure()
			return err
		}

		cb.onSuccess()
		return nil

	default:
		return NewCircuitBreakerError("unknown circuit breaker state", state)
	}
}

// onFailure handles failure events
func (cb *CircuitBreaker) onFailure() {
	failures := atomic.AddInt32(&cb.failures, 1)
	atomic.StoreInt32(&cb.successes, 0)

	if failures >= int32(cb.config.FailureThreshold) {
		atomic.StoreInt32(&cb.state, int32(StateOpen))
		cb.lastFailure = time.Now()
		cb.nextAttempt = cb.lastFailure.Add(cb.config.RecoveryTimeout)
	}
}

// onSuccess handles success events
func (cb *CircuitBreaker) onSuccess() {
	atomic.StoreInt32(&cb.failures, 0)

	if CircuitBreakerState(atomic.LoadInt32(&cb.state)) == StateHalfOpen {
		successes := atomic.AddInt32(&cb.successes, 1)
		if successes >= int32(cb.config.SuccessThreshold) {
			atomic.StoreInt32(&cb.state, int32(StateClosed))
		}
	}
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitBreakerState {
	return CircuitBreakerState(atomic.LoadInt32(&cb.state))
}

// Failures returns the current failure count
func (cb *CircuitBreaker) Failures() int {
	return int(atomic.LoadInt32(&cb.failures))
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	atomic.StoreInt32(&cb.state, int32(StateClosed))
	atomic.StoreInt32(&cb.failures, 0)
	atomic.StoreInt32(&cb.successes, 0)
}

// CircuitBreakerError represents an error from the circuit breaker
type CircuitBreakerError struct {
	Message string
	State   CircuitBreakerState
}

func (e *CircuitBreakerError) Error() string {
	return e.Message
}

// NewCircuitBreakerError creates a new circuit breaker error
func NewCircuitBreakerError(message string, state CircuitBreakerState) *CircuitBreakerError {
	return &CircuitBreakerError{
		Message: message,
		State:   state,
	}
}

// CircuitBreakerRegistry manages multiple circuit breakers
type CircuitBreakerRegistry struct {
	breakers map[string]*CircuitBreaker
}

// NewCircuitBreakerRegistry creates a new registry
func NewCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetOrCreate gets an existing circuit breaker or creates a new one
func (r *CircuitBreakerRegistry) GetOrCreate(name string, config CircuitBreakerConfig) *CircuitBreaker {
	if breaker, exists := r.breakers[name]; exists {
		return breaker
	}

	breaker := NewCircuitBreaker(config)
	r.breakers[name] = breaker
	return breaker
}

// Get returns a circuit breaker by name
func (r *CircuitBreakerRegistry) Get(name string) (*CircuitBreaker, bool) {
	breaker, exists := r.breakers[name]
	return breaker, exists
}

// ResetAll resets all circuit breakers
func (r *CircuitBreakerRegistry) ResetAll() {
	for _, breaker := range r.breakers {
		breaker.Reset()
	}
}

// GetStats returns statistics for all circuit breakers
func (r *CircuitBreakerRegistry) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	for name, breaker := range r.breakers {
		stats[name] = map[string]interface{}{
			"state":    breaker.State(),
			"failures": breaker.Failures(),
		}
	}

	return stats
}

// Global registry instance
var globalRegistry = NewCircuitBreakerRegistry()

// GetCircuitBreaker gets a circuit breaker from the global registry
func GetCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	return globalRegistry.GetOrCreate(name, config)
}

// GetCircuitBreakerStats returns stats from the global registry
func GetCircuitBreakerStats() map[string]interface{} {
	return globalRegistry.GetStats()
}

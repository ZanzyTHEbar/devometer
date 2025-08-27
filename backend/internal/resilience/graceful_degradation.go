package resilience

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/errors"
)

// DegradationLevel represents the current degradation state
type DegradationLevel int

const (
	LevelNormal DegradationLevel = iota
	LevelDegraded
	LevelCritical
	LevelEmergency
)

// DegradationConfig holds configuration for graceful degradation
type DegradationConfig struct {
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	DegradedThreshold   float64       `json:"degraded_threshold"`    // Error rate threshold (0.0-1.0)
	CriticalThreshold   float64       `json:"critical_threshold"`    // Error rate threshold (0.0-1.0)
	EmergencyThreshold  float64       `json:"emergency_threshold"`   // Error rate threshold (0.0-1.0)
	RecoveryTimeWindow  time.Duration `json:"recovery_time_window"`  // Time window for error rate calculation
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`  // Timeout for health checks
	MaxDegradedDuration time.Duration `json:"max_degraded_duration"` // Max time in degraded state before emergency
}

// DefaultDegradationConfig returns sensible defaults
func DefaultDegradationConfig() DegradationConfig {
	return DegradationConfig{
		HealthCheckInterval: 30 * time.Second,
		DegradedThreshold:   0.1,  // 10% error rate
		CriticalThreshold:   0.25, // 25% error rate
		EmergencyThreshold:  0.5,  // 50% error rate
		RecoveryTimeWindow:  5 * time.Minute,
		HealthCheckTimeout:  5 * time.Second,
		MaxDegradedDuration: 10 * time.Minute,
	}
}

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	ServiceName   string           `json:"service_name"`
	Level         DegradationLevel `json:"level"`
	ErrorRate     float64          `json:"error_rate"`
	TotalRequests int64            `json:"total_requests"`
	ErrorCount    int64            `json:"error_count"`
	LastError     error            `json:"-"` // Don't serialize
	LastErrorTime time.Time        `json:"last_error_time"`
	DegradedSince *time.Time       `json:"degraded_since,omitempty"`
	StatusMessage string           `json:"status_message"`
}

// DegradationManager manages graceful degradation for multiple services
type DegradationManager struct {
	config       DegradationConfig
	services     map[string]*ServiceHealth
	healthChecks map[string]HealthCheckFunc
	mutex        sync.RWMutex
}

// HealthCheckFunc represents a function that checks service health
type HealthCheckFunc func(ctx context.Context) error

// NewDegradationManager creates a new degradation manager
func NewDegradationManager(config DegradationConfig) *DegradationManager {
	return &DegradationManager{
		config:       config,
		services:     make(map[string]*ServiceHealth),
		healthChecks: make(map[string]HealthCheckFunc),
	}
}

// RegisterService registers a service with its health check function
func (dm *DegradationManager) RegisterService(serviceName string, healthCheck HealthCheckFunc) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	dm.services[serviceName] = &ServiceHealth{
		ServiceName:   serviceName,
		Level:         LevelNormal,
		ErrorRate:     0.0,
		TotalRequests: 0,
		ErrorCount:    0,
		StatusMessage: "Service is healthy",
	}

	if healthCheck != nil {
		dm.healthChecks[serviceName] = healthCheck
	}

	slog.Info("Registered service for degradation management", "service", serviceName)
}

// RecordRequest records a request and its success/failure
func (dm *DegradationManager) RecordRequest(serviceName string, success bool) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	service, exists := dm.services[serviceName]
	if !exists {
		return
	}

	service.TotalRequests++

	if !success {
		service.ErrorCount++
		service.LastErrorTime = time.Now()
		service.LastError = errors.NewInternalError("Service request failed", nil)
	}

	// Calculate error rate
	if service.TotalRequests > 0 {
		service.ErrorRate = float64(service.ErrorCount) / float64(service.TotalRequests)
	}

	// Update degradation level
	dm.updateDegradationLevel(service)
}

// RecordError records an error for a service
func (dm *DegradationManager) RecordError(serviceName string, err error) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	service, exists := dm.services[serviceName]
	if !exists {
		return
	}

	service.TotalRequests++
	service.ErrorCount++
	service.LastError = err
	service.LastErrorTime = time.Now()

	// Calculate error rate
	if service.TotalRequests > 0 {
		service.ErrorRate = float64(service.ErrorCount) / float64(service.TotalRequests)
	}

	// Update degradation level
	dm.updateDegradationLevel(service)
}

// updateDegradationLevel updates the degradation level based on current metrics
func (dm *DegradationManager) updateDegradationLevel(service *ServiceHealth) {
	oldLevel := service.Level
	now := time.Now()

	// Determine new level based on error rate
	var newLevel DegradationLevel
	var statusMessage string

	switch {
	case service.ErrorRate >= dm.config.EmergencyThreshold:
		newLevel = LevelEmergency
		statusMessage = "Service is in emergency state - high error rate"
	case service.ErrorRate >= dm.config.CriticalThreshold:
		newLevel = LevelCritical
		statusMessage = "Service is in critical state - elevated error rate"
	case service.ErrorRate >= dm.config.DegradedThreshold:
		newLevel = LevelDegraded
		statusMessage = "Service is degraded - moderate error rate"
	default:
		newLevel = LevelNormal
		statusMessage = "Service is healthy"
	}

	// Handle degraded duration timeout
	if newLevel == LevelDegraded && service.DegradedSince != nil {
		if now.Sub(*service.DegradedSince) > dm.config.MaxDegradedDuration {
			newLevel = LevelEmergency
			statusMessage = "Service has been degraded too long - entering emergency state"
		}
	}

	// Update degraded timestamp
	if newLevel == LevelDegraded && oldLevel != LevelDegraded {
		service.DegradedSince = &now
	} else if newLevel != LevelDegraded {
		service.DegradedSince = nil
	}

	// Update service status
	service.Level = newLevel
	service.StatusMessage = statusMessage

	// Log level changes
	if oldLevel != newLevel {
		slog.Warn("Service degradation level changed",
			"service", service.ServiceName,
			"old_level", oldLevel,
			"new_level", newLevel,
			"error_rate", service.ErrorRate,
			"total_requests", service.TotalRequests,
			"error_count", service.ErrorCount)
	}
}

// GetServiceHealth returns the health status of a service
func (dm *DegradationManager) GetServiceHealth(serviceName string) (*ServiceHealth, bool) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	service, exists := dm.services[serviceName]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modification
	return &ServiceHealth{
		ServiceName:   service.ServiceName,
		Level:         service.Level,
		ErrorRate:     service.ErrorRate,
		TotalRequests: service.TotalRequests,
		ErrorCount:    service.ErrorCount,
		LastError:     service.LastError,
		LastErrorTime: service.LastErrorTime,
		DegradedSince: service.DegradedSince,
		StatusMessage: service.StatusMessage,
	}, true
}

// GetAllServiceHealth returns health status for all services
func (dm *DegradationManager) GetAllServiceHealth() map[string]*ServiceHealth {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	result := make(map[string]*ServiceHealth)
	for name, service := range dm.services {
		result[name] = &ServiceHealth{
			ServiceName:   service.ServiceName,
			Level:         service.Level,
			ErrorRate:     service.ErrorRate,
			TotalRequests: service.TotalRequests,
			ErrorCount:    service.ErrorCount,
			LastError:     service.LastError,
			LastErrorTime: service.LastErrorTime,
			DegradedSince: service.DegradedSince,
			StatusMessage: service.StatusMessage,
		}
	}

	return result
}

// IsServiceAvailable checks if a service is available for use
func (dm *DegradationManager) IsServiceAvailable(serviceName string) bool {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	service, exists := dm.services[serviceName]
	if !exists {
		return false
	}

	// Service is unavailable only in emergency state
	return service.Level != LevelEmergency
}

// ShouldThrottleRequests checks if requests should be throttled due to high error rates
func (dm *DegradationManager) ShouldThrottleRequests(serviceName string) bool {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	service, exists := dm.services[serviceName]
	if !exists {
		return false
	}

	// Throttle requests when in critical or emergency state
	return service.Level >= LevelCritical
}

// GetThrottleFactor returns a factor to reduce request volume (0.0-1.0)
func (dm *DegradationManager) GetThrottleFactor(serviceName string) float64 {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	service, exists := dm.services[serviceName]
	if !exists {
		return 1.0 // No throttling
	}

	switch service.Level {
	case LevelNormal:
		return 1.0
	case LevelDegraded:
		return 0.7 // Reduce to 70% of normal load
	case LevelCritical:
		return 0.3 // Reduce to 30% of normal load
	case LevelEmergency:
		return 0.1 // Reduce to 10% of normal load
	default:
		return 1.0
	}
}

// StartHealthChecks starts periodic health checks for all registered services
func (dm *DegradationManager) StartHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(dm.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dm.performHealthChecks(ctx)
		}
	}
}

// performHealthChecks performs health checks for all services
func (dm *DegradationManager) performHealthChecks(ctx context.Context) {
	for serviceName, healthCheck := range dm.healthChecks {
		go func(name string, check HealthCheckFunc) {
			// Create timeout context for health check
			checkCtx, cancel := context.WithTimeout(ctx, dm.config.HealthCheckTimeout)
			defer cancel()

			err := check(checkCtx)
			if err != nil {
				dm.RecordError(name, errors.WrapError(err, "health check failed for service %s", name))
			} else {
				// Record successful health check
				dm.RecordRequest(name, true)
			}
		}(serviceName, healthCheck)
	}
}

// ResetService resets a service's health status
func (dm *DegradationManager) ResetService(serviceName string) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if service, exists := dm.services[serviceName]; exists {
		service.Level = LevelNormal
		service.ErrorRate = 0.0
		service.TotalRequests = 0
		service.ErrorCount = 0
		service.LastError = nil
		service.LastErrorTime = time.Time{}
		service.DegradedSince = nil
		service.StatusMessage = "Service is healthy"

		slog.Info("Service health reset", "service", serviceName)
	}
}

// GracefulShutdown performs a graceful shutdown of the degradation manager
func (dm *DegradationManager) GracefulShutdown() {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	slog.Info("Degradation manager shutting down", "services", len(dm.services))

	// Log final status of all services
	for name, service := range dm.services {
		slog.Info("Final service status",
			"service", name,
			"level", service.Level,
			"error_rate", service.ErrorRate,
			"total_requests", service.TotalRequests,
			"error_count", service.ErrorCount)
	}
}

// Global degradation manager instance
var globalDegradationManager = NewDegradationManager(DefaultDegradationConfig())

// RegisterService registers a service globally
func RegisterService(serviceName string, healthCheck HealthCheckFunc) {
	globalDegradationManager.RegisterService(serviceName, healthCheck)
}

// RecordRequest records a request globally
func RecordRequest(serviceName string, success bool) {
	globalDegradationManager.RecordRequest(serviceName, success)
}

// RecordError records an error globally
func RecordError(serviceName string, err error) {
	globalDegradationManager.RecordError(serviceName, err)
}

// IsServiceAvailable checks availability globally
func IsServiceAvailable(serviceName string) bool {
	return globalDegradationManager.IsServiceAvailable(serviceName)
}

// GetServiceHealth gets health status globally
func GetServiceHealth(serviceName string) (*ServiceHealth, bool) {
	return globalDegradationManager.GetServiceHealth(serviceName)
}

// GetAllServiceHealth gets all health statuses globally
func GetAllServiceHealth() map[string]*ServiceHealth {
	return globalDegradationManager.GetAllServiceHealth()
}

// StartHealthChecks starts global health checks
func StartHealthChecks(ctx context.Context) {
	go globalDegradationManager.StartHealthChecks(ctx)
}

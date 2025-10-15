package resilience

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// ConnectionPool manages a pool of HTTP connections with circuit breaker integration
type ConnectionPool struct {
	// Pool configuration
	maxIdle     int
	maxActive   int
	idleTimeout time.Duration

	// Circuit breaker integration
	circuitBreaker *CircuitBreaker

	// Connection tracking
	activeConnections int
	idleConnections   []*pooledConnection
	mutex             sync.RWMutex

	// Transport configuration
	transport *http.Transport
}

// PooledConnection represents a connection in the pool
type pooledConnection struct {
	client   *http.Client
	lastUsed time.Time
	inUse    bool
}

// NewConnectionPool creates a new connection pool with circuit breaker
func NewConnectionPool(maxIdle, maxActive int, idleTimeout time.Duration, cb *CircuitBreaker) *ConnectionPool {
	transport := &http.Transport{
		MaxIdleConns:          maxIdle,
		MaxConnsPerHost:       maxActive,
		MaxIdleConnsPerHost:   maxIdle / 2,
		IdleConnTimeout:       idleTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &ConnectionPool{
		maxIdle:           maxIdle,
		maxActive:         maxActive,
		idleTimeout:       idleTimeout,
		circuitBreaker:    cb,
		transport:         transport,
		activeConnections: 0,
		idleConnections:   make([]*pooledConnection, 0),
	}
}

// GetClient retrieves a pooled HTTP client
func (cp *ConnectionPool) GetClient() (*http.Client, error) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Clean up expired idle connections
	cp.cleanupIdleConnections()

	// Try to get an idle connection first
	if len(cp.idleConnections) > 0 {
		conn := cp.idleConnections[0]
		cp.idleConnections = cp.idleConnections[1:]

		conn.lastUsed = time.Now()
		conn.inUse = true

		slog.Debug("Reusing idle connection", "active", cp.activeConnections, "idle", len(cp.idleConnections))
		return conn.client, nil
	}

	// Check if we can create a new connection
	if cp.activeConnections >= cp.maxActive {
		return nil, fmt.Errorf("connection pool exhausted: %d/%d active connections", cp.activeConnections, cp.maxActive)
	}

	// Create new connection
	client := &http.Client{
		Transport: cp.transport,
		Timeout:   30 * time.Second,
	}

	cp.activeConnections++

	slog.Debug("Created new connection", "active", cp.activeConnections, "idle", len(cp.idleConnections))
	return client, nil
}

// ReturnClient returns a connection to the pool
func (cp *ConnectionPool) ReturnClient(client *http.Client) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Find the connection in our tracking
	for _, conn := range cp.idleConnections {
		if conn.client == client {
			conn.inUse = false
			conn.lastUsed = time.Now()
			return
		}
	}

	// If not found in idle connections, it might be a new connection
	// Check if we can add it to the pool
	if len(cp.idleConnections) < cp.maxIdle {
		conn := &pooledConnection{
			client:   client,
			lastUsed: time.Now(),
			inUse:    false,
		}
		cp.idleConnections = append(cp.idleConnections, conn)
		slog.Debug("Added connection to idle pool", "idle", len(cp.idleConnections))
	} else {
		// Pool is full, don't track this connection
		slog.Debug("Connection pool full, not tracking returned connection")
	}
}

// cleanupIdleConnections removes expired idle connections
func (cp *ConnectionPool) cleanupIdleConnections() {
	now := time.Now()
	validConnections := make([]*pooledConnection, 0)

	for _, conn := range cp.idleConnections {
		if now.Sub(conn.lastUsed) > cp.idleTimeout {
			// Connection expired, don't include it
			slog.Debug("Removing expired idle connection")
		} else {
			validConnections = append(validConnections, conn)
		}
	}

	cp.idleConnections = validConnections
}

// GetStats returns connection pool statistics
func (cp *ConnectionPool) GetStats() map[string]interface{} {
	cp.mutex.RLock()
	defer cp.mutex.RUnlock()

	return map[string]interface{}{
		"active_connections":    cp.activeConnections,
		"idle_connections":      len(cp.idleConnections),
		"max_idle":              cp.maxIdle,
		"max_active":            cp.maxActive,
		"idle_timeout_ms":       cp.idleTimeout.Milliseconds(),
		"circuit_breaker_state": cp.circuitBreaker.State(),
	}
}

// DoRequest executes an HTTP request with circuit breaker and connection pooling
func (cp *ConnectionPool) DoRequest(ctx context.Context, method, url string, headers map[string]string) (*http.Response, error) {
	var resp *http.Response

	// Execute request with circuit breaker protection
	err := cp.circuitBreaker.Call(func() error {
		// Get a pooled client
		client, err := cp.GetClient()
		if err != nil {
			return err
		}

		// Create request
		req, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			cp.ReturnClient(client)
			return err
		}

		// Add headers
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		// Execute request
		start := time.Now()
		resp, err = client.Do(req)
		duration := time.Since(start)

		// Update circuit breaker based on result
		if err != nil {
			slog.Warn("Request failed", "url", url, "error", err, "duration_ms", duration.Milliseconds())
			return err
		}

		slog.Debug("Request completed", "url", url, "status", resp.StatusCode, "duration_ms", duration.Milliseconds())

		// Return client to pool
		cp.ReturnClient(client)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Close closes all connections in the pool
func (cp *ConnectionPool) Close() error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Close idle connections
	for _, conn := range cp.idleConnections {
		if transport, ok := conn.client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}

	cp.idleConnections = nil
	cp.activeConnections = 0

	slog.Info("Connection pool closed")
	return nil
}

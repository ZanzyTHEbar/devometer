package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps the Redis client with health checks and graceful degradation
type RedisClient struct {
	client  *redis.Client
	enabled bool
	addr    string
}

// NewRedisClient creates a new Redis client with connection pooling
func NewRedisClient(addr, password string, db int) (*RedisClient, error) {
	if addr == "" {
		slog.Warn("Redis URL not configured, rate limiting will use in-memory fallback")
		return &RedisClient{enabled: false}, nil // Graceful degradation
	}

	slog.Info("Initializing Redis client", "addr", addr, "db", db)

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,              // Maximum number of socket connections
		MinIdleConns: 2,                // Minimum number of idle connections
		PoolTimeout:  4 * time.Second, // Time to wait for connection from pool
	})

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		slog.Error("Redis ping failed, falling back to in-memory rate limiting", "error", err)
		return &RedisClient{enabled: false, addr: addr}, fmt.Errorf("redis ping failed: %w", err)
	}

	slog.Info("Redis client connected successfully", "addr", addr)

	return &RedisClient{
		client:  client,
		enabled: true,
		addr:    addr,
	}, nil
}

// GetClient returns the underlying Redis client
func (r *RedisClient) GetClient() *redis.Client {
	return r.client
}

// IsEnabled returns whether Redis is enabled and healthy
func (r *RedisClient) IsEnabled() bool {
	return r.enabled
}

// HealthCheck performs a health check on the Redis connection
func (r *RedisClient) HealthCheck(ctx context.Context) error {
	if !r.enabled {
		return fmt.Errorf("redis is disabled")
	}

	return r.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	if r.enabled && r.client != nil {
		slog.Info("Closing Redis client connection")
		return r.client.Close()
	}
	return nil
}

// GetPoolStats returns Redis connection pool statistics
func (r *RedisClient) GetPoolStats() map[string]interface{} {
	if !r.enabled || r.client == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	stats := r.client.PoolStats()

	return map[string]interface{}{
		"enabled":        true,
		"addr":           r.addr,
		"hits":           stats.Hits,
		"misses":         stats.Misses,
		"timeouts":       stats.Timeouts,
		"total_conns":    stats.TotalConns,
		"idle_conns":     stats.IdleConns,
		"stale_conns":    stats.StaleConns,
	}
}


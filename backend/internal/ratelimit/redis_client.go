package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps the Redis client with graceful degradation support
type RedisClient struct {
	client  *redis.Client
	enabled bool
}

// NewRedisClient creates a new Redis client with connection pooling and health checking
func NewRedisClient(addr, password string, db int) (*RedisClient, error) {
	if addr == "" {
		slog.Warn("Redis address not provided, rate limiting will use in-memory fallback")
		return &RedisClient{enabled: false}, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
		// Connection pool settings
		PoolTimeout:  4 * time.Second,
		MaxIdleConns: 5,
	})

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		slog.Error("Redis ping failed, falling back to in-memory rate limiting",
			"error", err,
			"addr", addr)
		return &RedisClient{enabled: false}, fmt.Errorf("redis ping failed: %w", err)
	}

	slog.Info("Redis client connected successfully",
		"addr", addr,
		"db", db,
		"pool_size", 10)

	return &RedisClient{
		client:  client,
		enabled: true,
	}, nil
}

// IsEnabled returns whether Redis is available
func (r *RedisClient) IsEnabled() bool {
	return r.enabled
}

// Client returns the underlying Redis client (only if enabled)
func (r *RedisClient) Client() *redis.Client {
	if !r.enabled {
		return nil
	}
	return r.client
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	if r.enabled && r.client != nil {
		return r.client.Close()
	}
	return nil
}

// HealthCheck performs a health check on the Redis connection
func (r *RedisClient) HealthCheck(ctx context.Context) error {
	if !r.enabled {
		return fmt.Errorf("redis not enabled")
	}

	return r.client.Ping(ctx).Err()
}

// GetPoolStats returns connection pool statistics
func (r *RedisClient) GetPoolStats() map[string]interface{} {
	if !r.enabled || r.client == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	stats := r.client.PoolStats()

	return map[string]interface{}{
		"enabled":       true,
		"hits":          stats.Hits,
		"misses":        stats.Misses,
		"timeouts":      stats.Timeouts,
		"total_conns":   stats.TotalConns,
		"idle_conns":    stats.IdleConns,
		"stale_conns":   stats.StaleConns,
		"max_idle_time": "5m", // From config
		"pool_size":     10,   // From config
	}
}

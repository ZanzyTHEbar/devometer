<!-- 7bd5fda4-0c7b-420c-a2b1-156d8c6beea3 90d5b67a-4a6a-4d78-afe5-fa1d26aec8f4 -->
# Enhanced Rate Limiting Implementation with Best Practices

## Current State Analysis

Existing implementation uses `golang.org/x/time/rate` with in-memory storage:

- IP-based: 60 req/min with burst capacity (lines 173-201 in `backend/internal/security/security.go`)
- User-based: 5 req/week via database (lines 203-252)
- Single-instance only (no distributed support)
- Limited invalidation strategy (hourly cleanup of old entries)

## Problems with Current Approach

1. **Not distributed**: Won't work with multiple server instances
2. **In-memory only**: Lost on server restart
3. **No sliding window**: Fixed time windows can be gamed
4. **Manual cleanup**: Memory leaks if cleanup fails
5. **Limited observability**: No detailed metrics per user/IP

## Proposed Solution: Redis-Based Distributed Rate Limiting

### Package Selection

**Primary**: `github.com/go-redis/redis_rate` v10

- Built on go-redis v9 (modern, maintained)
- Implements sliding window counter algorithm
- Atomic operations (Redis Lua scripts)
- Production-tested by major companies
- Automatic key expiration (no manual cleanup)

**Alternative**: `golang.org/x/time/rate` (keep as fallback)

- Used when Redis unavailable
- Graceful degradation

### Architecture Design

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│   Client    │────▶│  Rate Limiter    │────▶│   Redis     │
│             │     │   Middleware     │     │  (Primary)  │
└─────────────┘     └──────────────────┘     └─────────────┘
                            │                        │
                            │ (Fallback)             │
                            ▼                        │
                    ┌──────────────────┐             │
                    │   In-Memory      │◀────────────┘
                    │   (Degraded)     │   (On failure)
                    └──────────────────┘
```

## Implementation Steps

### 1. Add Redis Dependencies

**File**: `backend/go.mod`

Add:

```
github.com/redis/go-redis/v9 v9.5.1
github.com/go-redis/redis_rate/v10 v10.0.1
```

**File**: `docker-compose.yml`

Add Redis service:

```yaml
redis:
  image: redis:7-alpine
  ports:
    - "6379:6379"
  volumes:
    - redis_data:/data
  command: redis-server --appendonly yes --maxmemory 256mb --maxmemory-policy allkeys-lru
  healthcheck:
    test: ["CMD", "redis-cli", "ping"]
    interval: 10s
    timeout: 3s
    retries: 3
```

### 2. Create Redis Client Package

**New File**: `backend/internal/ratelimit/redis_client.go`

```go
package ratelimit

import (
    "context"
    "fmt"
    "time"
    
    "github.com/redis/go-redis/v9"
)

type RedisClient struct {
    client *redis.Client
    enabled bool
}

func NewRedisClient(addr, password string, db int) (*RedisClient, error) {
    if addr == "" {
        return &RedisClient{enabled: false}, nil // Graceful degradation
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
    })
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := client.Ping(ctx).Err(); err != nil {
        return &RedisClient{enabled: false}, fmt.Errorf("redis ping failed: %w", err)
    }
    
    return &RedisClient{client: client, enabled: true}, nil
}

func (r *RedisClient) IsEnabled() bool {
    return r.enabled
}

func (r *RedisClient) Close() error {
    if r.enabled {
        return r.client.Close()
    }
    return nil
}
```

### 3. Create Distributed Rate Limiter

**New File**: `backend/internal/ratelimit/limiter.go`

Implements:

- Sliding window counter algorithm via `redis_rate.Limiter`
- Per-IP and per-user rate limiting
- Automatic key expiration (Redis TTL)
- Fallback to in-memory when Redis unavailable
- Context-aware for proper cancellation

Key features:

```go
type RateLimiter struct {
    redisLimiter  *redis_rate.Limiter  // Primary
    fallback      *rate.Limiter         // In-memory fallback
    redisClient   *RedisClient
    config        Config
    metrics       *monitoring.Metrics
}

// Allow checks if request is allowed and returns limit info
func (rl *RateLimiter) Allow(ctx context.Context, key string, limit Rate) (*Result, error) {
    if rl.redisClient.IsEnabled() {
        // Use Redis sliding window
        res, err := rl.redisLimiter.Allow(ctx, key, limit)
        if err != nil {
            // Fall back to in-memory on Redis error
            return rl.allowFallback(key, limit)
        }
        return res, nil
    }
    // Use in-memory limiter
    return rl.allowFallback(key, limit)
}
```

### 4. Implement Strong Invalidation Strategies

**File**: `backend/internal/ratelimit/invalidation.go`

Multiple invalidation strategies:

**a) Event-Driven Invalidation**

```go
// InvalidateUser removes all rate limit keys for a user
func (rl *RateLimiter) InvalidateUser(ctx context.Context, userID string) error {
    pattern := fmt.Sprintf("ratelimit:user:%s:*", userID)
    return rl.deleteByPattern(ctx, pattern)
}

// InvalidateIP removes all rate limit keys for an IP
func (rl *RateLimiter) InvalidateIP(ctx context.Context, ip string) error {
    pattern := fmt.Sprintf("ratelimit:ip:%s:*", ip)
    return rl.deleteByPattern(ctx, pattern)
}
```

**b) TTL-Based Expiration**

- Redis keys auto-expire via TTL
- Weekly keys: 8 days TTL
- Per-minute keys: 2 minutes TTL
- No manual cleanup needed

**c) Version-Based Invalidation**

```go
// BumpVersion forces all clients to restart limit window
func (rl *RateLimiter) BumpVersion(ctx context.Context, scope string) error {
    versionKey := fmt.Sprintf("ratelimit:version:%s", scope)
    return rl.redisClient.client.Incr(ctx, versionKey).Err()
}
```

**d) Policy-Based Reset**

```go
// ResetOnUpgrade immediately resets limits when user upgrades
func (rl *RateLimiter) ResetOnUpgrade(ctx context.Context, userID string) error {
    // Remove weekly limit key
    weekKey := fmt.Sprintf("ratelimit:user:%s:week", userID)
    return rl.redisClient.client.Del(ctx, weekKey).Err()
}
```

### 5. Create Rate Limit Middleware Package

**New File**: `backend/internal/ratelimit/middleware.go`

Features:

- Automatic header injection (X-RateLimit-*)
- Multiple limit tiers (IP, user, endpoint)
- Graceful degradation on Redis failure
- Comprehensive metrics tracking
- Circuit breaker for Redis calls
```go
func (rl *RateLimiter) IPRateLimitMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx := c.Request.Context()
        ip := c.ClientIP()
        
        key := fmt.Sprintf("ratelimit:ip:%s", ip)
        limit := Rate{Limit: 60, Period: time.Minute}
        
        result, err := rl.Allow(ctx, key, limit)
        if err != nil {
            // Log but don't block on error
            c.Next()
            return
        }
        
        // Inject standard headers
        c.Header("X-RateLimit-Limit", strconv.Itoa(result.Limit))
        c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
        c.Header("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))
        
        if !result.Allowed {
            rl.metrics.IncrementRateLimitIPBlock()
            c.Header("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error": "rate limit exceeded",
                "retry_after": result.RetryAfter.Seconds(),
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}
```


### 6. Enhance Monitoring & Metrics

**File**: `backend/internal/monitoring/metrics.go`

Add comprehensive rate limit metrics:

```go
// Rate limit metrics
RateLimitIPBlocks         int64
RateLimitUserBlocks       int64
RateLimitRedisErrors      int64
RateLimitFallbackCount    int64
RateLimitByEndpoint       map[string]*RateLimitStats
RateLimitMutex            sync.RWMutex

type RateLimitStats struct {
    TotalRequests   int64
    BlockedRequests int64
    AverageUsage    float64 // % of limit used
}
```

Methods:

- `IncrementRateLimitBlock(limitType, endpoint string)`
- `RecordRateLimitUsage(key string, remaining, limit int)`
- `GetRateLimitStats() map[string]interface{}`

### 7. Create Management Endpoints

**File**: `backend/cmd/server/main.go`

Add admin endpoints for rate limit management:

```go
// Get current rate limit status
r.GET("/api/rate-limit/status", rateLimitStatusHandler)

// Admin: View all rate limits
r.GET("/api/admin/rate-limits", adminRateLimitsHandler)

// Admin: Reset user rate limit (for support)
r.POST("/api/admin/rate-limit/reset/:userID", adminResetRateLimitHandler)

// Admin: Get rate limit metrics
r.GET("/api/admin/rate-limit/metrics", adminRateLimitMetricsHandler)

// Health check includes Redis status
r.GET("/health", enhancedHealthCheckHandler)
```

### 8. Update Security Middleware Integration

**File**: `backend/cmd/server/main.go`

Replace current security middleware rate limiting:

```go
// Initialize Redis client
redisClient, err := ratelimit.NewRedisClient(
    os.Getenv("REDIS_URL"),
    os.Getenv("REDIS_PASSWORD"),
    0, // DB 0
)

// Initialize distributed rate limiter
rateLimiter := ratelimit.NewRateLimiter(
    redisClient,
    ratelimit.Config{
        IPLimit:       60, // per minute
        UserLimit:     5,  // per week
        BurstMultiplier: 2,
    },
    appMetrics,
)

// Apply middleware
r.Use(rateLimiter.IPRateLimitMiddleware())
r.Use(rateLimiter.UserRateLimitMiddleware())
```

### 9. Add Configuration

**File**: `env.example`

```
# Redis Configuration
REDIS_URL=redis://localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_ENABLED=true

# Rate Limiting Configuration
RATE_LIMIT_IP_PER_MIN=60
RATE_LIMIT_USER_PER_WEEK=5
RATE_LIMIT_FALLBACK_ENABLED=true
```

### 10. Comprehensive Testing

**New File**: `backend/internal/ratelimit/limiter_test.go`

Test coverage:

- Sliding window accuracy
- Redis failover to in-memory
- Concurrent request handling
- Key expiration
- Invalidation strategies
- Header injection
- Metrics accuracy

**New File**: `backend/internal/ratelimit/invalidation_test.go`

Test invalidation:

- User upgrade resets limits
- IP ban clears all IP keys
- Version bumps force refresh
- TTL expiration works correctly

## Key Advantages Over Current Implementation

1. **Distributed**: Works across multiple server instances
2. **Persistent**: Survives server restarts
3. **Sliding Window**: More accurate, harder to game
4. **Auto-Cleanup**: Redis TTL handles expiration
5. **Graceful Degradation**: Falls back when Redis unavailable
6. **Industry Standard**: Using proven packages
7. **Strong Invalidation**: Multiple strategies for different scenarios
8. **Observable**: Comprehensive metrics and logging
9. **Maintainable**: Less custom code, more community support

## Migration Strategy

1. Deploy Redis alongside existing system
2. Run both rate limiters in parallel (log-only mode for Redis)
3. Compare results over 24 hours
4. Switch to Redis as primary
5. Keep in-memory as fallback
6. Remove old code after 1 week stable operation

## Testing Checklist

- [ ] Redis connection with retry logic
- [ ] Sliding window counter accuracy
- [ ] Fallback to in-memory on Redis failure
- [ ] All invalidation strategies work
- [ ] Headers injected correctly
- [ ] Metrics track all events
- [ ] Load test: 1000 req/sec from 100 IPs
- [ ] Chaos test: Redis restart during load
- [ ] Memory leak test: 24-hour soak
- [ ] Multi-instance test: 3 servers + Redis

## Performance Expectations

- Redis latency: <2ms (p99)
- Fallback latency: <0.1ms
- Memory usage: ~10MB Redis for 10K active users
- Throughput: 50K+ req/sec per Redis instance

### To-dos

- [ ] Add Redis to docker-compose.yml with proper configuration
- [ ] Create Redis client package with connection pooling
- [ ] Implement distributed rate limiter with redis_rate package
- [ ] Implement all invalidation strategies (event, TTL, version, policy)
- [ ] Create rate limit middleware with header injection
- [ ] Add comprehensive rate limit metrics to monitoring
- [ ] Create management endpoints for rate limit status and admin
- [ ] Integrate new rate limiter into main server
- [ ] Implement graceful degradation to in-memory limiter
- [ ] Write comprehensive tests for limiter and invalidation
- [ ] Perform load testing with 1000 req/sec
- [ ] Chaos engineering test with Redis failures
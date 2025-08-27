package leaderboard

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/cache"
)

// LeaderboardCache provides caching for leaderboard data
type LeaderboardCache struct {
	cache *cache.Cache
}

// NewLeaderboardCache creates a new leaderboard cache
func NewLeaderboardCache(ttl time.Duration) *LeaderboardCache {
	return &LeaderboardCache{
		cache: cache.NewCache(ttl),
	}
}

// generateCacheKey creates a cache key for leaderboard data
func (lc *LeaderboardCache) generateCacheKey(period string, limit int) string {
	return fmt.Sprintf("leaderboard:%s:%d", period, limit)
}

// generateRankCacheKey creates a cache key for individual rank data
func (lc *LeaderboardCache) generateRankCacheKey(hash, period string) string {
	return fmt.Sprintf("rank:%s:%s", hash, period)
}

// GetLeaderboard retrieves cached leaderboard data
func (lc *LeaderboardCache) GetLeaderboard(period string, limit int) (*LeaderboardResponse, bool) {
	cacheKey := lc.generateCacheKey(period, limit)

	data, found := lc.cache.Get(cacheKey)
	if !found {
		return nil, false
	}

	var response LeaderboardResponse
	if err := json.Unmarshal(data, &response); err != nil {
		slog.Error("Failed to unmarshal cached leaderboard data", "error", err, "key", cacheKey)
		return nil, false
	}

	slog.Debug("Leaderboard cache hit", "period", period, "limit", limit)
	return &response, true
}

// SetLeaderboard caches leaderboard data
func (lc *LeaderboardCache) SetLeaderboard(period string, limit int, response *LeaderboardResponse) {
	cacheKey := lc.generateCacheKey(period, limit)

	data, err := json.Marshal(response)
	if err != nil {
		slog.Error("Failed to marshal leaderboard data for cache", "error", err, "period", period)
		return
	}

	lc.cache.Set(cacheKey, data)
	slog.Debug("Leaderboard cached", "period", period, "limit", limit, "entries", len(response.Entries))
}

// GetDeveloperRank retrieves cached rank data
func (lc *LeaderboardCache) GetDeveloperRank(hash, period string) (*LeaderboardEntry, bool) {
	cacheKey := lc.generateRankCacheKey(hash, period)

	data, found := lc.cache.Get(cacheKey)
	if !found {
		return nil, false
	}

	var entry LeaderboardEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		slog.Error("Failed to unmarshal cached rank data", "error", err, "key", cacheKey)
		return nil, false
	}

	slog.Debug("Rank cache hit", "hash", hash[:8]+"...", "period", period)
	return &entry, true
}

// SetDeveloperRank caches rank data
func (lc *LeaderboardCache) SetDeveloperRank(hash, period string, entry *LeaderboardEntry) {
	cacheKey := lc.generateRankCacheKey(hash, period)

	data, err := json.Marshal(entry)
	if err != nil {
		slog.Error("Failed to marshal rank data for cache", "error", err, "hash", hash[:8]+"...")
		return
	}

	lc.cache.Set(cacheKey, data)
	slog.Debug("Rank cached", "hash", hash[:8]+"...", "period", period, "rank", entry.Rank)
}

// InvalidatePeriod invalidates all cache entries for a specific period
func (lc *LeaderboardCache) InvalidatePeriod(period string) {
	// Note: This is a simple implementation. In a production system,
	// you might want to use a more sophisticated cache invalidation strategy
	// like cache tags or pub/sub patterns.

	slog.Info("Invalidating leaderboard cache for period", "period", period)

	// The cache cleanup will handle expired entries, but for immediate invalidation
	// we would need to track cache keys or use a different caching strategy
	// For now, we'll rely on TTL expiration
}

// InvalidateAll invalidates all leaderboard cache entries
func (lc *LeaderboardCache) InvalidateAll() {
	slog.Info("Invalidating all leaderboard cache entries")
	// Similar to InvalidatePeriod, this relies on TTL expiration
}

// GetStats returns cache statistics
func (lc *LeaderboardCache) GetStats() map[string]interface{} {
	return lc.cache.Stats()
}

// WarmCache pre-populates the cache with popular leaderboard data
func (lc *LeaderboardCache) WarmCache(service *Service) {
	popularConfigs := []struct {
		period string
		limit  int
	}{
		{"daily", 50},
		{"weekly", 50},
		{"monthly", 50},
		{"all_time", 50},
		{"daily", 25},
		{"weekly", 25},
		{"monthly", 25},
		{"all_time", 25},
	}

	slog.Info("Starting leaderboard cache warming")

	for _, config := range popularConfigs {
		response, err := service.GetLeaderboard(config.period, config.limit)
		if err != nil {
			slog.Error("Failed to warm cache for leaderboard",
				"error", err, "period", config.period, "limit", config.limit)
			continue
		}

		lc.SetLeaderboard(config.period, config.limit, response)
		slog.Debug("Warmed cache for leaderboard", "period", config.period, "limit", config.limit)
	}

	slog.Info("Leaderboard cache warming completed")
}

// AutoRefresh sets up automatic cache refresh for leaderboard data
func (lc *LeaderboardCache) AutoRefresh(service *Service, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			slog.Debug("Auto-refreshing leaderboard cache")
			lc.WarmCache(service)
		}
	}()
}

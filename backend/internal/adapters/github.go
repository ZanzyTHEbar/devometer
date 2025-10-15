package adapters

import (
	"context"
	"net/http"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/resilience"
)

// GitHubEvent represents a raw event from GitHub
type GitHubEvent struct {
	Type      string  `json:"type"`
	Timestamp string  `json:"timestamp"`
	Count     float64 `json:"count"`
	Repo      string  `json:"repo"`
	Language  string  `json:"language"`
}

// GitHubAdapter fetches data from GitHub API
type GitHubAdapter struct {
	token string
	pool  *resilience.ConnectionPool
}

// NewGitHubAdapter creates a new GitHub adapter with connection pooling
func NewGitHubAdapter(token string) *GitHubAdapter {
	// Create circuit breaker for GitHub API
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		FailureThreshold: 5,
		RecoveryTimeout:  30 * time.Second,
		SuccessThreshold: 3,
	})

	// Create connection pool
	pool := resilience.NewConnectionPool(10, 20, 30*time.Second, cb)

	return &GitHubAdapter{
		token: token,
		pool:  pool,
	}
}

// FetchRepoData fetches repository statistics
func (g *GitHubAdapter) FetchRepoData(ctx interface{}, owner, repo string) ([]GitHubEvent, error) {
	// MVP: Return realistic mock data for testing
	events := []GitHubEvent{
		{
			Type:      "stars",
			Timestamp: "now",
			Count:     150,
			Repo:      owner + "/" + repo,
		},
		{
			Type:      "forks",
			Timestamp: "now",
			Count:     45,
			Repo:      owner + "/" + repo,
		},
		{
			Type:      "merged_pr",
			Timestamp: "now",
			Count:     25,
			Repo:      owner + "/" + repo,
		},
		{
			Type:      "commit",
			Timestamp: "now",
			Count:     1,
			Repo:      owner + "/" + repo,
		},
		{
			Type:      "language",
			Timestamp: "now",
			Count:     1,
			Repo:      owner + "/" + repo,
			Language:  "TypeScript",
		},
	}

	return events, nil
}

// FetchUserData fetches user statistics
func (g *GitHubAdapter) FetchUserData(ctx interface{}, username string) ([]GitHubEvent, error) {
	// MVP: Return realistic mock data for testing
	events := []GitHubEvent{
		{
			Type:      "followers",
			Timestamp: "now",
			Count:     200,
		},
		{
			Type:      "following",
			Timestamp: "now",
			Count:     80,
		},
		{
			Type:      "total_stars",
			Timestamp: "now",
			Count:     500,
		},
		{
			Type:      "total_forks",
			Timestamp: "now",
			Count:     120,
		},
		{
			Type:      "user_language",
			Timestamp: "now",
			Count:     3,
			Language:  "Go",
		},
	}

	return events, nil
}

// makeRequest makes an HTTP request using the connection pool
func (g *GitHubAdapter) makeRequest(ctx context.Context, method, url string) (*http.Response, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + g.token,
		"User-Agent":    "Cracked-Dev-o-Meter/1.0",
		"Accept":        "application/vnd.github.v3+json",
	}

	return g.pool.DoRequest(ctx, method, url, headers)
}

// GetPoolStats returns connection pool statistics
func (g *GitHubAdapter) GetPoolStats() map[string]interface{} {
	return g.pool.GetStats()
}

// Close closes the connection pool
func (g *GitHubAdapter) Close() error {
	return g.pool.Close()
}

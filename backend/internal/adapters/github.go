package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// GitHubRepo represents GitHub repository data
type GitHubRepo struct {
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	StargazersCount int    `json:"stargazers_count"`
	ForksCount      int    `json:"forks_count"`
	Language        string `json:"language"`
	UpdatedAt       string `json:"updated_at"`
}

// GitHubUser represents GitHub user data
type GitHubUser struct {
	Login       string `json:"login"`
	Followers   int    `json:"followers"`
	Following   int    `json:"following"`
	PublicRepos int    `json:"public_repos"`
}

// GitHubPullRequest represents a pull request
type GitHubPullRequest struct {
	State     string `json:"state"`
	MergedAt  string `json:"merged_at"`
	CreatedAt string `json:"created_at"`
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

// FetchRepoData fetches repository statistics from GitHub API
func (g *GitHubAdapter) FetchRepoData(ctx context.Context, owner, repo string) ([]GitHubEvent, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	resp, err := g.makeRequest(ctx, "GET", url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repo data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var repoData GitHubRepo
	if err := json.NewDecoder(resp.Body).Decode(&repoData); err != nil {
		return nil, fmt.Errorf("failed to decode repo data: %w", err)
	}

	// Convert API response to events
	events := []GitHubEvent{
		{
			Type:      "stars",
			Timestamp: repoData.UpdatedAt,
			Count:     float64(repoData.StargazersCount),
			Repo:      repoData.FullName,
		},
		{
			Type:      "forks",
			Timestamp: repoData.UpdatedAt,
			Count:     float64(repoData.ForksCount),
			Repo:      repoData.FullName,
		},
		{
			Type:      "language",
			Timestamp: repoData.UpdatedAt,
			Count:     1,
			Repo:      repoData.FullName,
			Language:  repoData.Language,
		},
	}

	return events, nil
}

// FetchUserData fetches user statistics from GitHub API
func (g *GitHubAdapter) FetchUserData(ctx context.Context, username string) ([]GitHubEvent, error) {
	url := fmt.Sprintf("https://api.github.com/users/%s", username)

	resp, err := g.makeRequest(ctx, "GET", url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var userData GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&userData); err != nil {
		return nil, fmt.Errorf("failed to decode user data: %w", err)
	}

	// Convert API response to events
	now := time.Now().Format(time.RFC3339)
	events := []GitHubEvent{
		{
			Type:      "followers",
			Timestamp: now,
			Count:     float64(userData.Followers),
		},
		{
			Type:      "following",
			Timestamp: now,
			Count:     float64(userData.Following),
		},
		{
			Type:      "public_repos",
			Timestamp: now,
			Count:     float64(userData.PublicRepos),
		},
	}

	return events, nil
}

// makeRequest makes an HTTP request to GitHub API using the connection pool
func (g *GitHubAdapter) makeRequest(ctx context.Context, method, url string) (*http.Response, error) {
	headers := map[string]string{
		"Accept": "application/vnd.github.v3+json",
	}

	// Add authorization if token is provided
	if g.token != "" {
		headers["Authorization"] = "Bearer " + g.token
	}

	// Add user agent (required by GitHub API)
	headers["User-Agent"] = "Cracked-Dev-o-Meter/1.0"

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

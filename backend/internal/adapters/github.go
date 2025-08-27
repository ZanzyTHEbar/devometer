package adapters

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
}

// NewGitHubAdapter creates a new GitHub adapter
func NewGitHubAdapter(token string) *GitHubAdapter {
	return &GitHubAdapter{token: token}
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

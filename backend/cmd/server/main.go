package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/adapters"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/analysis"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/types"
	"github.com/gin-gonic/gin"
)

func main() {
	// Get configuration from environment
	dataDir := "./data"
	githubToken := ""

	// Create analyzer and adapter
	analyzer := analysis.NewAnalyzer(dataDir)
	githubAdapter := adapters.NewGitHubAdapter(githubToken)

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/analyze", func(c *gin.Context) {
		var req struct {
			Input string `json:"input"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		input := req.Input

		// Parse input to determine if it's a repo or user
		var ghEvents []adapters.GitHubEvent
		var err error

		if strings.Contains(input, "/") {
			// Looks like owner/repo
			parts := strings.Split(input, "/")
			if len(parts) == 2 {
				ghEvents, err = githubAdapter.FetchRepoData(nil, parts[0], parts[1])
			}
		} else {
			// Assume it's a username
			ghEvents, err = githubAdapter.FetchUserData(nil, input)
		}

		var rawEvents []types.RawEvent
		if err != nil {
			// Fallback to heuristic-based analysis if GitHub API fails
			rawEvents = []types.RawEvent{}
		} else {
			// Convert GitHub events to RawEvents
			rawEvents = make([]types.RawEvent, len(ghEvents))
			for i, gh := range ghEvents {
				rawEvents[i] = types.RawEvent{
					Type:      gh.Type,
					Timestamp: time.Now(), // Use current time for simplified implementation
					Count:     gh.Count,
					Repo:      gh.Repo,
					Language:  gh.Language,
				}
			}
		}

		// Use the analyzer
		res, err := analyzer.AnalyzeEvents(rawEvents, input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "analysis failed"})
			return
		}

		c.JSON(http.StatusOK, res)
	})

	r.Run(":8080")
}

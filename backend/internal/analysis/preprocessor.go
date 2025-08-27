package analysis

import (
	"sort"
	"strings"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/types"
)

// Preprocessor handles anti-gaming and data cleaning
type Preprocessor struct {
	minSpacing time.Duration
}

// NewPreprocessor creates a new preprocessor
func NewPreprocessor(minSpacing time.Duration) *Preprocessor {
	return &Preprocessor{minSpacing: minSpacing}
}

// ProcessEvents applies anti-gaming rules and data cleaning
func (p *Preprocessor) ProcessEvents(events []types.RawEvent) []types.RawEvent {
	// Sort by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	// Remove duplicates
	events = p.removeDuplicates(events)

	// Discount trivial events
	events = p.discountTrivial(events)

	// Penalize abnormal timing patterns
	events = p.penalizeAbnormalTiming(events)

	// Exclude bot accounts (basic heuristic)
	events = p.excludeBots(events)

	return events
}

// removeDuplicates collapses near-duplicate events
func (p *Preprocessor) removeDuplicates(events []types.RawEvent) []types.RawEvent {
	if len(events) == 0 {
		return events
	}

	cleaned := []types.RawEvent{events[0]}
	for _, event := range events[1:] {
		last := &cleaned[len(cleaned)-1]

		// Same type, repo, within min spacing
		if event.Type == last.Type && event.Repo == last.Repo &&
			event.Timestamp.Sub(last.Timestamp) < p.minSpacing {
			// Merge counts
			last.Count += event.Count
			continue
		}

		cleaned = append(cleaned, event)
	}

	return cleaned
}

// discountTrivial discounts trivial changes and boilerplate
func (p *Preprocessor) discountTrivial(events []types.RawEvent) []types.RawEvent {
	for i := range events {
		switch events[i].Type {
		case "commit":
			// Discount very small commits (likely trivial)
			if events[i].Count < 10 {
				events[i].Count *= 0.5
			}
		case "merged_pr":
			// Discount PRs with very few changed files
			if events[i].Count < 5 {
				events[i].Count *= 0.7
			}
		}
	}
	return events
}

// penalizeAbnormalTiming penalizes commits/PRs at abnormal times
func (p *Preprocessor) penalizeAbnormalTiming(events []types.RawEvent) []types.RawEvent {
	for i := range events {
		hour := events[i].Timestamp.Hour()

		// Penalize commits between 2-5 AM (likely bot/scripted)
		if hour >= 2 && hour <= 5 {
			events[i].Count *= 0.3
		}

		// Boost commits during normal work hours (9-17)
		if hour >= 9 && hour <= 17 {
			events[i].Count *= 1.1
		}
	}
	return events
}

// excludeBots removes events from suspected bot accounts
func (p *Preprocessor) excludeBots(events []types.RawEvent) []types.RawEvent {
	cleaned := []types.RawEvent{}

	for _, event := range events {
		// Basic bot detection heuristics
		isBot := false

		// Check for bot-like patterns in repo names
		if strings.Contains(event.Repo, "bot") ||
			strings.Contains(event.Repo, "-ci") ||
			strings.Contains(event.Repo, "-automation") {
			isBot = true
		}

		// Check metadata for bot indicators
		if event.Metadata != nil {
			if bot, ok := event.Metadata["is_bot"].(bool); ok && bot {
				isBot = true
			}
		}

		if !isBot {
			cleaned = append(cleaned, event)
		}
	}

	return cleaned
}

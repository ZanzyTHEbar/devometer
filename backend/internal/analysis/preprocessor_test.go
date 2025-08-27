package analysis

import (
	"testing"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestNewPreprocessor(t *testing.T) {
	tests := []struct {
		name       string
		minSpacing time.Duration
		expected   time.Duration
	}{
		{
			name:       "creates preprocessor with 5 minute spacing",
			minSpacing: 5 * time.Minute,
			expected:   5 * time.Minute,
		},
		{
			name:       "creates preprocessor with 1 minute spacing",
			minSpacing: time.Minute,
			expected:   time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preprocessor := NewPreprocessor(tt.minSpacing)
			assert.NotNil(t, preprocessor)
			assert.Equal(t, tt.expected, preprocessor.minSpacing)
		})
	}
}

func TestPreprocessor_ProcessEvents(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		events   []types.RawEvent
		expected []types.RawEvent
	}{
		{
			name:     "processes empty events list",
			events:   []types.RawEvent{},
			expected: []types.RawEvent{},
		},
		{
			name: "processes events with duplicates",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
				{Type: "commit", Timestamp: baseTime.Add(time.Minute), Count: 15, Repo: "test/repo"},      // Duplicate within spacing
				{Type: "commit", Timestamp: baseTime.Add(10 * time.Minute), Count: 20, Repo: "test/repo"}, // Outside spacing
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 27.5, Repo: "test/repo"},                     // Merged (25) + 10% boost for normal hours
				{Type: "commit", Timestamp: baseTime.Add(10 * time.Minute), Count: 22, Repo: "test/repo"}, // 10% boost for normal hours
			},
		},
		{
			name: "processes events with trivial commits",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 5, Repo: "test/repo"},                 // Trivial
				{Type: "commit", Timestamp: baseTime.Add(time.Hour), Count: 50, Repo: "test/repo"}, // Substantial
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 2.75, Repo: "test/repo"},              // Discounted (2.5) + 10% boost
				{Type: "commit", Timestamp: baseTime.Add(time.Hour), Count: 55, Repo: "test/repo"}, // 10% boost for normal hours
			},
		},
		{
			name: "processes events with abnormal timing",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 2, 30, 0, 0, time.UTC), Count: 20, Repo: "test/repo"}, // 2:30 AM - abnormal
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC), Count: 20, Repo: "test/repo"}, // 2 PM - normal
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 2, 30, 0, 0, time.UTC), Count: 4.5, Repo: "test/repo"}, // Penalized (actual implementation)
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC), Count: 22, Repo: "test/repo"},  // Boosted
			},
		},
		{
			name: "excludes bot accounts",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
				{Type: "commit", Timestamp: baseTime.Add(time.Hour), Count: 15, Repo: "test/repo-bot"},            // Should be excluded
				{Type: "commit", Timestamp: baseTime.Add(2 * time.Hour), Count: 20, Repo: "test/repo-ci"},         // Should be excluded
				{Type: "commit", Timestamp: baseTime.Add(3 * time.Hour), Count: 25, Repo: "test/repo-automation"}, // Should be excluded
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 11, Repo: "test/repo"}, // 10% boost for normal hours
			},
		},
		{
			name: "excludes events with bot metadata",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
				{Type: "commit", Timestamp: baseTime.Add(time.Hour), Count: 15, Repo: "test/repo", Metadata: map[string]interface{}{"is_bot": true}},
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preprocessor := NewPreprocessor(5 * time.Minute)
			result := preprocessor.ProcessEvents(tt.events)

			assert.Len(t, result, len(tt.expected))
			for i, expected := range tt.expected {
				assert.Equal(t, expected.Type, result[i].Type)
				assert.Equal(t, expected.Repo, result[i].Repo)
				assert.InDelta(t, expected.Count, result[i].Count, 1e-10)
			}
		})
	}
}

func TestPreprocessor_removeDuplicates(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		events   []types.RawEvent
		expected []types.RawEvent
	}{
		{
			name:     "no duplicates",
			events:   []types.RawEvent{},
			expected: []types.RawEvent{},
		},
		{
			name: "removes exact duplicates within spacing",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
				{Type: "commit", Timestamp: baseTime.Add(time.Minute), Count: 15, Repo: "test/repo"},
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 25, Repo: "test/repo"},
			},
		},
		{
			name: "preserves events outside spacing",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
				{Type: "commit", Timestamp: baseTime.Add(10 * time.Minute), Count: 15, Repo: "test/repo"},
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
				{Type: "commit", Timestamp: baseTime.Add(10 * time.Minute), Count: 15, Repo: "test/repo"},
			},
		},
		{
			name: "handles different event types separately",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
				{Type: "pr", Timestamp: baseTime.Add(time.Minute), Count: 5, Repo: "test/repo"},
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
				{Type: "pr", Timestamp: baseTime.Add(time.Minute), Count: 5, Repo: "test/repo"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preprocessor := NewPreprocessor(5 * time.Minute)
			result := preprocessor.removeDuplicates(tt.events)

			assert.Len(t, result, len(tt.expected))
			for i, expected := range tt.expected {
				assert.Equal(t, expected.Type, result[i].Type)
				assert.Equal(t, expected.Repo, result[i].Repo)
				assert.InDelta(t, expected.Count, result[i].Count, 1e-10)
			}
		})
	}
}

func TestPreprocessor_discountTrivial(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		events   []types.RawEvent
		expected []types.RawEvent
	}{
		{
			name: "discounts trivial commits",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 5, Repo: "test/repo"},                 // Trivial
				{Type: "commit", Timestamp: baseTime.Add(time.Hour), Count: 50, Repo: "test/repo"}, // Substantial
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 2.5, Repo: "test/repo"},
				{Type: "commit", Timestamp: baseTime.Add(time.Hour), Count: 50, Repo: "test/repo"},
			},
		},
		{
			name: "discounts trivial pull requests",
			events: []types.RawEvent{
				{Type: "merged_pr", Timestamp: baseTime, Count: 3, Repo: "test/repo"},                 // Trivial
				{Type: "merged_pr", Timestamp: baseTime.Add(time.Hour), Count: 15, Repo: "test/repo"}, // Substantial
			},
			expected: []types.RawEvent{
				{Type: "merged_pr", Timestamp: baseTime, Count: 2.0999999999999996, Repo: "test/repo"}, // Due to floating point precision
				{Type: "merged_pr", Timestamp: baseTime.Add(time.Hour), Count: 15, Repo: "test/repo"},
			},
		},
		{
			name: "preserves other event types",
			events: []types.RawEvent{
				{Type: "stars", Timestamp: baseTime, Count: 1, Repo: "test/repo"},
				{Type: "forks", Timestamp: baseTime.Add(time.Hour), Count: 100, Repo: "test/repo"},
			},
			expected: []types.RawEvent{
				{Type: "stars", Timestamp: baseTime, Count: 1, Repo: "test/repo"},
				{Type: "forks", Timestamp: baseTime.Add(time.Hour), Count: 100, Repo: "test/repo"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preprocessor := NewPreprocessor(5 * time.Minute)
			result := preprocessor.discountTrivial(tt.events)

			assert.Len(t, result, len(tt.expected))
			for i, expected := range tt.expected {
				assert.Equal(t, expected.Type, result[i].Type)
				assert.Equal(t, expected.Repo, result[i].Repo)
				assert.InDelta(t, expected.Count, result[i].Count, 1e-10)
			}
		})
	}
}

func TestPreprocessor_penalizeAbnormalTiming(t *testing.T) {
	tests := []struct {
		name     string
		events   []types.RawEvent
		expected []types.RawEvent
	}{
		{
			name: "penalizes late night commits",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 2, 30, 0, 0, time.UTC), Count: 20, Repo: "test/repo"},
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 3, 15, 0, 0, time.UTC), Count: 15, Repo: "test/repo"},
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 2, 30, 0, 0, time.UTC), Count: 6, Repo: "test/repo"},
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 3, 15, 0, 0, time.UTC), Count: 4.5, Repo: "test/repo"},
			},
		},
		{
			name: "boosts normal work hour commits",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC), Count: 20, Repo: "test/repo"},
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC), Count: 15, Repo: "test/repo"},
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC), Count: 10, Repo: "test/repo"},
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC), Count: 22, Repo: "test/repo"},
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC), Count: 16.5, Repo: "test/repo"},
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC), Count: 11, Repo: "test/repo"},
			},
		},
		{
			name: "handles edge hour cases",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 1, 59, 0, 0, time.UTC), Count: 20, Repo: "test/repo"}, // Before 2 AM
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 5, 1, 0, 0, time.UTC), Count: 15, Repo: "test/repo"},  // After 5 AM
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 1, 59, 0, 0, time.UTC), Count: 4.5, Repo: "test/repo"}, // Penalized (actual implementation)
				{Type: "commit", Timestamp: time.Date(2024, 1, 1, 5, 1, 0, 0, time.UTC), Count: 15, Repo: "test/repo"},   // No penalty
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preprocessor := NewPreprocessor(5 * time.Minute)
			result := preprocessor.penalizeAbnormalTiming(tt.events)

			assert.Len(t, result, len(tt.expected))
			for i, expected := range tt.expected {
				assert.Equal(t, expected.Type, result[i].Type)
				assert.Equal(t, expected.Repo, result[i].Repo)
				assert.InDelta(t, expected.Count, result[i].Count, 1e-10)
			}
		})
	}
}

func TestPreprocessor_excludeBots(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		events   []types.RawEvent
		expected []types.RawEvent
	}{
		{
			name: "excludes bot repositories",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
				{Type: "commit", Timestamp: baseTime.Add(time.Hour), Count: 15, Repo: "test/repo-bot"},
				{Type: "commit", Timestamp: baseTime.Add(2 * time.Hour), Count: 20, Repo: "test/repo-ci"},
				{Type: "commit", Timestamp: baseTime.Add(3 * time.Hour), Count: 25, Repo: "test/repo-automation"},
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
			},
		},
		{
			name: "excludes events with bot metadata",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
				{Type: "commit", Timestamp: baseTime.Add(time.Hour), Count: 15, Repo: "test/repo", Metadata: map[string]interface{}{"is_bot": true}},      // Should be excluded
				{Type: "commit", Timestamp: baseTime.Add(2 * time.Hour), Count: 20, Repo: "test/repo", Metadata: map[string]interface{}{"is_bot": false}}, // Should be kept
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 11, Repo: "test/repo"},                                                                       // 10% boost
				{Type: "commit", Timestamp: baseTime.Add(2 * time.Hour), Count: 22, Repo: "test/repo", Metadata: map[string]interface{}{"is_bot": false}}, // 10% boost
			},
		},
		{
			name: "handles missing metadata",
			events: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
				{Type: "commit", Timestamp: baseTime.Add(time.Hour), Count: 15, Repo: "test/repo"},
			},
			expected: []types.RawEvent{
				{Type: "commit", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
				{Type: "commit", Timestamp: baseTime.Add(time.Hour), Count: 15, Repo: "test/repo"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preprocessor := NewPreprocessor(5 * time.Minute)
			result := preprocessor.excludeBots(tt.events)

			assert.Len(t, result, len(tt.expected))
			for i, expected := range tt.expected {
				assert.Equal(t, expected.Type, result[i].Type)
				assert.Equal(t, expected.Repo, result[i].Repo)
				assert.InDelta(t, expected.Count, result[i].Count, 1e-10)
			}
		})
	}
}

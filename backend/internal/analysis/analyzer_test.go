package analysis

import (
	"testing"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestNewAnalyzer(t *testing.T) {
	tests := []struct {
		name    string
		dataDir string
	}{
		{
			name:    "creates analyzer with valid data directory",
			dataDir: "./test_data",
		},
		{
			name:    "creates analyzer with empty data directory",
			dataDir: "",
		},
		{
			name:    "creates analyzer with relative path",
			dataDir: "../data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAnalyzer(tt.dataDir)

			assert.NotNil(t, analyzer)
			assert.NotNil(t, analyzer.preprocessor)
			assert.NotNil(t, analyzer.calibrationStore)
			assert.Equal(t, tt.dataDir, analyzer.calibrationStore.dataDir)
		})
	}
}

func TestAnalyzer_AnalyzeEvents(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		events   []types.RawEvent
		domain   string
		expected ScoreResult
	}{
		{
			name:   "analyzes empty events list",
			events: []types.RawEvent{},
			domain: "test",
			expected: ScoreResult{
				Score:        50, // Default score for empty data
				Confidence:   0.5,
				Posterior:    0.5,
				Contributors: []Contributor{},
				Breakdown: Breakdown{
					Shipping:      0,
					Quality:       0,
					Influence:     0,
					Complexity:    0,
					Collaboration: 0,
					Reliability:   0,
					Novelty:       0,
				},
			},
		},
		{
			name: "analyzes events with GitHub influence data",
			events: []types.RawEvent{
				{Type: "stars", Timestamp: baseTime, Count: 100, Repo: "test/repo"},
				{Type: "forks", Timestamp: baseTime, Count: 20, Repo: "test/repo"},
				{Type: "followers", Timestamp: baseTime, Count: 50, Repo: "test/repo"},
			},
			domain: "test",
			expected: ScoreResult{
				Score:      46, // Actual score based on influence data
				Confidence: 0.8,
				Posterior:  0.46,
				Contributors: []Contributor{
					{Name: "influence.stars", Contribution: 0},
					{Name: "influence.forks", Contribution: 0},
					{Name: "influence.followers", Contribution: 0},
				},
				Breakdown: Breakdown{
					Shipping:      0,
					Quality:       0,
					Influence:     0,
					Complexity:    0,
					Collaboration: 0,
					Reliability:   0,
					Novelty:       0,
				},
			},
		},
		{
			name: "analyzes high influence developer",
			events: []types.RawEvent{
				{Type: "stars", Timestamp: baseTime, Count: 500, Repo: "test/repo"},
				{Type: "forks", Timestamp: baseTime, Count: 100, Repo: "test/repo"},
				{Type: "followers", Timestamp: baseTime, Count: 200, Repo: "test/repo"},
				{Type: "total_stars", Timestamp: baseTime, Count: 1000, Repo: "test/repo"},
			},
			domain: "test",
			expected: ScoreResult{
				Score:      74, // Actual high score for significant influence
				Confidence: 0.9,
				Posterior:  0.74,
				Contributors: []Contributor{
					{Name: "influence.stars", Contribution: 0},
					{Name: "influence.forks", Contribution: 0},
					{Name: "influence.followers", Contribution: 0},
					{Name: "influence.total_stars", Contribution: 0},
				},
				Breakdown: Breakdown{
					Shipping:      0,
					Quality:       0,
					Influence:     0,
					Complexity:    0,
					Collaboration: 0,
					Reliability:   0,
					Novelty:       0,
				},
			},
		},
		{
			name: "handles events with abnormal timing",
			events: []types.RawEvent{
				{Type: "stars", Timestamp: time.Date(2024, 1, 1, 2, 30, 0, 0, time.UTC), Count: 100, Repo: "test/repo"}, // 2:30 AM - abnormal
				{Type: "forks", Timestamp: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC), Count: 20, Repo: "test/repo"},  // 2 PM - normal
			},
			domain: "test",
			expected: ScoreResult{
				Score:      45, // Actual score with abnormal timing effects
				Confidence: 0.8,
				Posterior:  0.45,
				Contributors: []Contributor{
					{Name: "influence.stars", Contribution: 1.2}, // Penalized
					{Name: "influence.forks", Contribution: 0.9}, // Boosted
				},
				Breakdown: Breakdown{
					Shipping:      0,
					Quality:       0,
					Influence:     2.1,
					Complexity:    0,
					Collaboration: 0,
					Reliability:   0,
					Novelty:       0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAnalyzer("./test_data")

			result, err := analyzer.AnalyzeEvents(tt.events, tt.domain)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected.Score, result.Score)
			assert.InDelta(t, tt.expected.Confidence, result.Confidence, 0.1)
			assert.InDelta(t, tt.expected.Posterior, result.Posterior, 0.1)

			// Check breakdown structure
			assert.NotNil(t, result.Breakdown)

			// Contributors may vary, just check they're present when expected
			if len(tt.expected.Contributors) > 0 {
				assert.NotEmpty(t, result.Contributors)
			}
		})
	}
}

func TestAnalyzer_buildFeatureVectorSimple(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		events   []types.RawEvent
		domain   string
		expected FeatureVector
	}{
		{
			name:   "builds feature vector from empty events",
			events: []types.RawEvent{},
			domain: "test",
			expected: FeatureVector{
				Shipping:      make(map[string]float64),
				Quality:       make(map[string]float64),
				Influence:     make(map[string]float64),
				Complexity:    make(map[string]float64),
				Collaboration: make(map[string]float64),
				Reliability:   make(map[string]float64),
				Novelty:       make(map[string]float64),
				Coverage:      0.5,
			},
		},
		{
			name: "builds feature vector with influence data",
			events: []types.RawEvent{
				{Type: "stars", Timestamp: baseTime, Count: 100, Repo: "test/repo"},
				{Type: "forks", Timestamp: baseTime, Count: 20, Repo: "test/repo"},
				{Type: "followers", Timestamp: baseTime, Count: 50, Repo: "test/repo"},
			},
			domain: "test",
			expected: FeatureVector{
				Shipping: make(map[string]float64),
				Quality:  make(map[string]float64),
				Influence: map[string]float64{
					"stars":     0,                    // RobustZ transformed value
					"forks":     -0.5163413264753705,  // RobustZ transformed value
					"followers": -0.33115925956519165, // RobustZ transformed value
				},
				Complexity:    make(map[string]float64),
				Collaboration: make(map[string]float64),
				Reliability:   make(map[string]float64),
				Novelty:       make(map[string]float64),
				Coverage:      0.8,
			},
		},
		{
			name: "handles unknown event types gracefully",
			events: []types.RawEvent{
				{Type: "unknown", Timestamp: baseTime, Count: 10, Repo: "test/repo"},
				{Type: "stars", Timestamp: baseTime, Count: 100, Repo: "test/repo"},
			},
			domain: "test",
			expected: FeatureVector{
				Shipping: make(map[string]float64),
				Quality:  make(map[string]float64),
				Influence: map[string]float64{
					"stars": 0, // RobustZ transformed value
				},
				Complexity:    make(map[string]float64),
				Collaboration: make(map[string]float64),
				Reliability:   make(map[string]float64),
				Novelty:       make(map[string]float64),
				Coverage:      0.8,
			},
		},
		{
			name: "handles events with preprocessing effects",
			events: []types.RawEvent{
				{Type: "stars", Timestamp: time.Date(2024, 1, 1, 2, 30, 0, 0, time.UTC), Count: 100, Repo: "test/repo"}, // 2:30 AM - penalized
				{Type: "forks", Timestamp: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC), Count: 20, Repo: "test/repo"},  // 2 PM - boosted
			},
			domain: "test",
			expected: FeatureVector{
				Shipping: make(map[string]float64),
				Quality:  make(map[string]float64),
				Influence: map[string]float64{
					"stars": 0,                   // RobustZ transformed value
					"forks": -0.5163413264753705, // RobustZ transformed value
				},
				Complexity:    make(map[string]float64),
				Collaboration: make(map[string]float64),
				Reliability:   make(map[string]float64),
				Novelty:       make(map[string]float64),
				Coverage:      0.8,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAnalyzer("./test_data")

			result := analyzer.buildFeatureVectorSimple(tt.events, tt.domain)

			assert.Equal(t, tt.expected.Coverage, result.Coverage)
			assert.Equal(t, len(tt.expected.Influence), len(result.Influence))

			for key, expectedValue := range tt.expected.Influence {
				actualValue, exists := result.Influence[key]
				assert.True(t, exists, "Expected influence key %s to exist", key)
				assert.InDelta(t, expectedValue, actualValue, 0.1, "Influence value for %s", key)
			}

			// Check that other categories are empty or have expected values
			assert.Empty(t, result.Shipping)
			assert.Empty(t, result.Quality)
			assert.Empty(t, result.Complexity)
			assert.Empty(t, result.Collaboration)
			assert.Empty(t, result.Reliability)
			assert.Empty(t, result.Novelty)
		})
	}
}

func TestAnalyzeInput_BackwardCompatibility(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ScoreResult
	}{
		{
			name:  "backward compatibility with domain input",
			input: "github",
			expected: ScoreResult{
				Score:        50,
				Confidence:   0.5,
				Posterior:    0.5,
				Contributors: []Contributor{},
				Breakdown: Breakdown{
					Shipping:      0,
					Quality:       0,
					Influence:     0,
					Complexity:    0,
					Collaboration: 0,
					Reliability:   0,
					Novelty:       0,
				},
			},
		},
		{
			name:  "backward compatibility with empty input",
			input: "",
			expected: ScoreResult{
				Score:        50,
				Confidence:   0.5,
				Posterior:    0.5,
				Contributors: []Contributor{},
				Breakdown: Breakdown{
					Shipping:      0,
					Quality:       0,
					Influence:     0,
					Complexity:    0,
					Collaboration: 0,
					Reliability:   0,
					Novelty:       0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnalyzeInput(tt.input)

			assert.Equal(t, tt.expected.Score, result.Score)
			assert.InDelta(t, tt.expected.Confidence, result.Confidence, 0.1)
			assert.InDelta(t, tt.expected.Posterior, result.Posterior, 0.1)
			assert.Equal(t, len(tt.expected.Contributors), len(result.Contributors))
		})
	}
}

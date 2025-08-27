package analysis

import (
	"testing"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/types"
)

func TestDecayWeight(t *testing.T) {
	tests := []struct {
		name     string
		deltaDays float64
		tau      float64
		expected float64
	}{
		{"zero days", 0, 60, 1.0},
		{"half life", 60, 60, 0.5},
		{"very old", 365, 60, 0.1353352832366127},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecayWeight(tt.deltaDays, tt.tau)
			if result < tt.expected-0.001 || result > tt.expected+0.001 {
				t.Errorf("DecayWeight(%v, %v) = %v, want %v", tt.deltaDays, tt.tau, result, tt.expected)
			}
		})
	}
}

func TestRobustZ(t *testing.T) {
	sample := []float64{1, 2, 3, 4, 5}

	tests := []struct {
		name     string
		value    float64
		expected float64
	}{
		{"median value", 3, 0}, // median is 3, so z-score should be 0
		{"below median", 1, -1.099787206469059}, // asinh((1-3)/1.4826*1.4826)
		{"above median", 5, 1.099787206469059},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RobustZ(tt.value, sample)
			if result < tt.expected-0.001 || result > tt.expected+0.001 {
				t.Errorf("RobustZ(%v, %v) = %v, want %v", tt.value, sample, result, tt.expected)
			}
		})
	}
}

func TestAggregateScore(t *testing.T) {
	// Test with minimal data
	fv := FeatureVector{
		Shipping:      map[string]float64{"commits": 1.0},
		Quality:       map[string]float64{"reviews": 1.0},
		Influence:     map[string]float64{"stars": 1.0},
		Complexity:    map[string]float64{"languages": 1.0},
		Collaboration: map[string]float64{"collaborators": 1.0},
		Reliability:   map[string]float64{"ci_pass": 1.0},
		Novelty:       map[string]float64{"new_lang": 1.0},
		Coverage:      0.8,
	}

	result := AggregateScore(fv)

	// Check that we get a valid score
	if result.Score < 0 || result.Score > 100 {
		t.Errorf("Score should be between 0-100, got %d", result.Score)
	}

	// Check confidence
	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("Confidence should be between 0-1, got %f", result.Confidence)
	}

	// Check that breakdown sums make sense
	breakdown := result.Breakdown
	if breakdown.Shipping < 0 || breakdown.Shipping > 1 {
		t.Errorf("Shipping breakdown should be between 0-1, got %f", breakdown.Shipping)
	}
}

func TestPreprocessor(t *testing.T) {
	preprocessor := NewPreprocessor(5 * time.Minute)

	// Create test events
	events := []types.RawEvent{
		{
			Type:      "commit",
			Timestamp: time.Now(),
			Count:     100,
			Repo:      "test/repo",
		},
		{
			Type:      "merged_pr",
			Timestamp: time.Now(),
			Count:     5,
			Repo:      "test/repo",
		},
		{
			Type:      "stars",
			Timestamp: time.Now(),
			Count:     50,
			Repo:      "test/repo",
		},
	}

	processed := preprocessor.ProcessEvents(events)

	// Should have same number of events after processing
	if len(processed) != len(events) {
		t.Errorf("Expected %d events after processing, got %d", len(events), len(processed))
	}

	// Check that trivial events were penalized
	for _, event := range processed {
		if event.Type == "commit" && event.Count >= 100 {
			t.Error("Large commit should have been penalized")
		}
		if event.Type == "merged_pr" && event.Count >= 5 {
			t.Error("Small PR should have been penalized")
		}
	}
}

func TestMedian(t *testing.T) {
	tests := []struct {
		name     string
		input    []float64
		expected float64
	}{
		{"odd length", []float64{1, 3, 2}, 2},
		{"even length", []float64{1, 2, 3, 4}, 2.5},
		{"empty", []float64{}, 0},
		{"single", []float64{5}, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := median(tt.input)
			if result != tt.expected {
				t.Errorf("median(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

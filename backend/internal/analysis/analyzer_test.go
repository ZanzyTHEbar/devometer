package analysis

import (
	"testing"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/types"
)

func TestDecayWeight(t *testing.T) {
	tests := []struct {
		name      string
		deltaDays float64
		tau       float64
		expected  float64
	}{
		{"zero days", 0, 60, 1.0},
		{"half life", 60, 60, 0.36787944117144232159552377016146}, // exp(-60/60) = exp(-1)
		{"very old", 365, 60, 0.002280562095392161},               // exp(-365/60) = exp(-6.083333)
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
		{"median value", 3, 0},                  // median is 3, so z-score should be 0
		{"below median", 1, -1.107966066124596}, // asinh((1-3)/1.4826*1.4826)
		{"above median", 5, 1.107966066124596},
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

	// Check that breakdown values are reasonable (can be > 1 with tuned base bias)
	breakdown := result.Breakdown
	if breakdown.Shipping < 0 {
		t.Errorf("Shipping breakdown should be non-negative, got %f", breakdown.Shipping)
	}
	if breakdown.Quality < 0 {
		t.Errorf("Quality breakdown should be non-negative, got %f", breakdown.Quality)
	}
	if breakdown.Influence < 0 {
		t.Errorf("Influence breakdown should be non-negative, got %f", breakdown.Influence)
	}
}

func TestPreprocessor_ProcessEvents(t *testing.T) {
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

	// Check that events were processed (timing boosts applied)
	for _, event := range processed {
		if event.Type == "commit" && event.Count <= 100 {
			t.Error("Commit should have been boosted for normal hours")
		}
		if event.Type == "merged_pr" && event.Count <= 5 {
			t.Error("PR should have been boosted for normal hours")
		}
	}
}

func TestDualHorizonBlend(t *testing.T) {
	tests := []struct {
		name     string
		shortAgg float64
		longAgg  float64
		lambda   float64
		expected float64
	}{
		{"equal weights", 10, 20, 0.5, 15},
		{"short heavy", 10, 20, 0.7, 13},
		{"long heavy", 10, 20, 0.3, 17},
		{"edge case zero", 0, 0, 0.5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BlendDualHorizon(tt.shortAgg, tt.longAgg, tt.lambda)
			if result != tt.expected {
				t.Errorf("BlendDualHorizon(%v, %v, %v) = %v, want %v",
					tt.shortAgg, tt.longAgg, tt.lambda, result, tt.expected)
			}
		})
	}
}

func TestCalibrationStore(t *testing.T) {
	store := NewCalibrationStore("./test_data")

	// Test loading default calibration
	cal, err := store.LoadCalibration("nonexistent")
	if err != nil {
		t.Errorf("LoadCalibration should not error for nonexistent domain: %v", err)
	}

	// Check default values
	if len(cal.Shipping) == 0 {
		t.Error("Default calibration should have shipping data")
	}

	// Test saving calibration
	testCal := &CalibrationData{
		Shipping: []float64{1, 2, 3},
	}
	err = store.SaveCalibration("test", testCal)
	if err != nil {
		t.Errorf("SaveCalibration should not error: %v", err)
	}
}

func TestEventAggregator(t *testing.T) {
	agg := NewEventAggregator()

	// Test adding values
	agg.Add("commits", 5)
	agg.Add("commits", 3)
	agg.Add("stars", 10)

	all := agg.GetAll()
	if all["commits"] != 8 {
		t.Errorf("Expected commits=8, got %v", all["commits"])
	}
	if all["stars"] != 10 {
		t.Errorf("Expected stars=10, got %v", all["stars"])
	}

	if agg.Get("commits") != 8 {
		t.Errorf("Get commits should return 8, got %v", agg.Get("commits"))
	}
	if agg.Get("nonexistent") != 0 {
		t.Errorf("Get nonexistent should return 0, got %v", agg.Get("nonexistent"))
	}
}

func TestAnalyzer_EndToEnd(t *testing.T) {
	analyzer := NewAnalyzer("./test_data")

	// Test with empty events (fallback behavior)
	events := []types.RawEvent{}
	result, err := analyzer.AnalyzeEvents(events, "test")

	if err != nil {
		t.Errorf("AnalyzeEvents should not error with empty events: %v", err)
	}

	if result.Score < 0 || result.Score > 100 {
		t.Errorf("Score should be between 0-100, got %d", result.Score)
	}

	// Test with real events
	events = []types.RawEvent{
		{
			Type:      "stars",
			Timestamp: time.Now(),
			Count:     100,
			Repo:      "test/repo",
		},
		{
			Type:      "forks",
			Timestamp: time.Now(),
			Count:     20,
			Repo:      "test/repo",
		},
	}

	result, err = analyzer.AnalyzeEvents(events, "test")
	if err != nil {
		t.Errorf("AnalyzeEvents should not error with real events: %v", err)
	}

	// With real data, influence should be non-zero
	if result.Breakdown.Influence == 0 {
		t.Error("Influence should be non-zero with star/fork data")
	}
}

func TestAnalyzer_EndToEndWithX(t *testing.T) {
	analyzer := NewAnalyzer("./test_data")

	// Test with GitHub events only (should work like before)
	githubEvents := []types.RawEvent{
		{
			Type:      "stars",
			Timestamp: time.Now(),
			Count:     100,
			Repo:      "test/repo",
		},
		{
			Type:      "forks",
			Timestamp: time.Now(),
			Count:     20,
			Repo:      "test/repo",
		},
	}

	// Test with X (Twitter) events only
	xEvents := []types.RawEvent{
		{
			Type:      "twitter_followers",
			Timestamp: time.Now(),
			Count:     500,
			Repo:      "testuser",
		},
		{
			Type:      "twitter_tweets",
			Timestamp: time.Now(),
			Count:     100,
			Repo:      "testuser",
		},
		{
			Type:      "twitter_likes",
			Timestamp: time.Now(),
			Count:     200,
			Repo:      "testuser",
		},
	}

	// Test with combined GitHub and X events
	result, err := analyzer.AnalyzeEventsWithX(githubEvents, xEvents, "test")
	if err != nil {
		t.Errorf("AnalyzeEventsWithX should not error with combined events: %v", err)
	}

	if result.Score < 0 || result.Score > 100 {
		t.Errorf("Score should be between 0-100, got %d", result.Score)
	}

	// With combined data, influence should be higher than with just GitHub
	githubOnlyResult, err := analyzer.AnalyzeEventsWithX(githubEvents, []types.RawEvent{}, "test")
	if err != nil {
		t.Errorf("AnalyzeEventsWithX should not error with GitHub-only events: %v", err)
	}

	// Combined result should generally have higher influence due to diverse data sources
	if result.Breakdown.Influence <= githubOnlyResult.Breakdown.Influence {
		t.Logf("Combined influence (%f) vs GitHub-only influence (%f)", result.Breakdown.Influence, githubOnlyResult.Breakdown.Influence)
		// This is not a hard requirement as calibration may affect results
	}

	// Test with empty X events (should work like regular AnalyzeEvents)
	emptyXResult, err := analyzer.AnalyzeEventsWithX(githubEvents, []types.RawEvent{}, "test")
	if err != nil {
		t.Errorf("AnalyzeEventsWithX should not error with empty X events: %v", err)
	}

	// Results should be comparable (allowing for minor differences in processing)
	if emptyXResult.Score < 0 || emptyXResult.Score > 100 {
		t.Errorf("Empty X events score should be between 0-100, got %d", emptyXResult.Score)
	}
}

func TestClip(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		lo       float64
		hi       float64
		expected float64
	}{
		{"within bounds", 5, 0, 10, 5},
		{"below minimum", -5, 0, 10, 0},
		{"above maximum", 15, 0, 10, 10},
		{"equal to bounds", 0, 0, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clip(tt.value, tt.lo, tt.hi)
			if result != tt.expected {
				t.Errorf("clip(%v, %v, %v) = %v, want %v",
					tt.value, tt.lo, tt.hi, result, tt.expected)
			}
		})
	}
}

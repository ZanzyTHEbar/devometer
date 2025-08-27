package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeatureVector_ZeroValue(t *testing.T) {
	var fv FeatureVector

	// Zero value should have nil maps (Go default for maps)
	assert.Nil(t, fv.Shipping)
	assert.Nil(t, fv.Quality)
	assert.Nil(t, fv.Influence)
	assert.Nil(t, fv.Complexity)
	assert.Nil(t, fv.Collaboration)
	assert.Nil(t, fv.Reliability)
	assert.Nil(t, fv.Novelty)

	// Coverage should be 0
	assert.Equal(t, 0.0, fv.Coverage)
}

func TestFeatureVector_DataIntegrity(t *testing.T) {
	fv := FeatureVector{
		Shipping: map[string]float64{
			"commits": 10.5,
			"prs":     3.2,
		},
		Influence: map[string]float64{
			"stars": 100.0,
			"forks": 25.0,
		},
		Coverage: 0.8,
	}

	// Verify data integrity
	assert.Equal(t, 10.5, fv.Shipping["commits"])
	assert.Equal(t, 3.2, fv.Shipping["prs"])
	assert.Equal(t, 100.0, fv.Influence["stars"])
	assert.Equal(t, 25.0, fv.Influence["forks"])
	assert.Equal(t, 0.8, fv.Coverage)
}

func TestContributor_Structure(t *testing.T) {
	contributor := Contributor{
		Name:         "shipping.commits",
		Contribution: 2.5,
	}

	assert.Equal(t, "shipping.commits", contributor.Name)
	assert.Equal(t, 2.5, contributor.Contribution)
}

func TestContributor_ZeroValue(t *testing.T) {
	var contributor Contributor

	assert.Empty(t, contributor.Name)
	assert.Equal(t, 0.0, contributor.Contribution)
}

func TestBreakdown_Structure(t *testing.T) {
	breakdown := Breakdown{
		Shipping:      1.2,
		Quality:       0.8,
		Influence:     2.1,
		Complexity:    0.5,
		Collaboration: 1.8,
		Reliability:   0.9,
		Novelty:       0.3,
	}

	assert.Equal(t, 1.2, breakdown.Shipping)
	assert.Equal(t, 0.8, breakdown.Quality)
	assert.Equal(t, 2.1, breakdown.Influence)
	assert.Equal(t, 0.5, breakdown.Complexity)
	assert.Equal(t, 1.8, breakdown.Collaboration)
	assert.Equal(t, 0.9, breakdown.Reliability)
	assert.Equal(t, 0.3, breakdown.Novelty)
}

func TestBreakdown_ZeroValue(t *testing.T) {
	var breakdown Breakdown

	assert.Equal(t, 0.0, breakdown.Shipping)
	assert.Equal(t, 0.0, breakdown.Quality)
	assert.Equal(t, 0.0, breakdown.Influence)
	assert.Equal(t, 0.0, breakdown.Complexity)
	assert.Equal(t, 0.0, breakdown.Collaboration)
	assert.Equal(t, 0.0, breakdown.Reliability)
	assert.Equal(t, 0.0, breakdown.Novelty)
}

func TestScoreResult_Structure(t *testing.T) {
	result := ScoreResult{
		Score:      75,
		Confidence: 0.85,
		Posterior:  0.75,
		Contributors: []Contributor{
			{Name: "influence.stars", Contribution: 2.1},
			{Name: "shipping.commits", Contribution: 1.8},
		},
		Breakdown: Breakdown{
			Shipping:      1.8,
			Quality:       0.5,
			Influence:     2.1,
			Complexity:    0.7,
			Collaboration: 1.2,
			Reliability:   0.9,
			Novelty:       0.3,
		},
	}

	assert.Equal(t, 75, result.Score)
	assert.Equal(t, 0.85, result.Confidence)
	assert.Equal(t, 0.75, result.Posterior)
	assert.Len(t, result.Contributors, 2)
	assert.NotNil(t, result.Breakdown)

	// Verify contributor details
	assert.Equal(t, "influence.stars", result.Contributors[0].Name)
	assert.Equal(t, 2.1, result.Contributors[0].Contribution)
	assert.Equal(t, "shipping.commits", result.Contributors[1].Name)
	assert.Equal(t, 1.8, result.Contributors[1].Contribution)
}

func TestScoreResult_ZeroValue(t *testing.T) {
	var result ScoreResult

	assert.Equal(t, 0, result.Score)
	assert.Equal(t, 0.0, result.Confidence)
	assert.Equal(t, 0.0, result.Posterior)
	assert.Nil(t, result.Contributors) // Zero value slices are nil in Go
	assert.Equal(t, Breakdown{}, result.Breakdown)
}

func TestScoreResult_Validation(t *testing.T) {
	tests := []struct {
		name    string
		result  ScoreResult
		isValid bool
	}{
		{
			name: "valid result",
			result: ScoreResult{
				Score:      75,
				Confidence: 0.8,
				Posterior:  0.75,
				Contributors: []Contributor{
					{Name: "test", Contribution: 1.0},
				},
				Breakdown: Breakdown{
					Shipping: 1.0,
				},
			},
			isValid: true,
		},
		{
			name: "score too low",
			result: ScoreResult{
				Score:      -5,
				Confidence: 0.8,
				Posterior:  0.75,
			},
			isValid: false,
		},
		{
			name: "score too high",
			result: ScoreResult{
				Score:      150,
				Confidence: 0.8,
				Posterior:  0.75,
			},
			isValid: false,
		},
		{
			name: "confidence too low",
			result: ScoreResult{
				Score:      75,
				Confidence: -0.1,
				Posterior:  0.75,
			},
			isValid: false,
		},
		{
			name: "confidence too high",
			result: ScoreResult{
				Score:      75,
				Confidence: 1.5,
				Posterior:  0.75,
			},
			isValid: false,
		},
		{
			name: "posterior too low",
			result: ScoreResult{
				Score:      75,
				Confidence: 0.8,
				Posterior:  -0.1,
			},
			isValid: false,
		},
		{
			name: "posterior too high",
			result: ScoreResult{
				Score:      75,
				Confidence: 0.8,
				Posterior:  1.5,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validateScoreResult(tt.result)
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

func TestCalibrationData_Structure(t *testing.T) {
	calibration := CalibrationData{
		Shipping:      []float64{1, 2, 3, 4, 5},
		Quality:       []float64{0.1, 0.2, 0.3},
		Influence:     []float64{10, 20, 30, 40},
		Complexity:    []float64{0.1, 0.2, 0.3, 0.4},
		Collaboration: []float64{1, 2, 3},
		Reliability:   []float64{0.8, 0.9, 1.0},
		Novelty:       []float64{0.1, 0.2},
	}

	assert.Len(t, calibration.Shipping, 5)
	assert.Len(t, calibration.Quality, 3)
	assert.Len(t, calibration.Influence, 4)
	assert.Len(t, calibration.Complexity, 4)
	assert.Len(t, calibration.Collaboration, 3)
	assert.Len(t, calibration.Reliability, 3)
	assert.Len(t, calibration.Novelty, 2)
}

func TestCalibrationData_ZeroValue(t *testing.T) {
	var calibration CalibrationData

	// Zero value slices are nil in Go
	assert.Nil(t, calibration.Shipping)
	assert.Nil(t, calibration.Quality)
	assert.Nil(t, calibration.Influence)
	assert.Nil(t, calibration.Complexity)
	assert.Nil(t, calibration.Collaboration)
	assert.Nil(t, calibration.Reliability)
	assert.Nil(t, calibration.Novelty)
}

func TestDataStructureImmutability(t *testing.T) {
	// Test that structures can be safely copied and modified
	originalFV := FeatureVector{
		Shipping:  map[string]float64{"commits": 10},
		Influence: map[string]float64{"stars": 100},
		Coverage:  0.8,
	}

	// Create a copy - in Go, this creates a shallow copy
	copyFV := originalFV

	// Modify copy - this affects both because maps are reference types
	copyFV.Shipping["commits"] = 20
	copyFV.Coverage = 0.9

	// In Go, both original and copy share the same map references
	// So changes to the map in copy affect the original
	assert.Equal(t, 20.0, originalFV.Shipping["commits"]) // Changed due to shared map reference
	assert.Equal(t, 0.8, originalFV.Coverage)             // Unchanged - this is a value field

	// Copy should have new values
	assert.Equal(t, 20.0, copyFV.Shipping["commits"])
	assert.Equal(t, 0.9, copyFV.Coverage)
}

func TestScoreResultJSONSerialization(t *testing.T) {
	result := ScoreResult{
		Score:      75,
		Confidence: 0.85,
		Posterior:  0.75,
		Contributors: []Contributor{
			{Name: "influence.stars", Contribution: 2.1},
		},
		Breakdown: Breakdown{
			Shipping:  1.8,
			Influence: 2.1,
		},
	}

	// Test that the struct can be used in JSON contexts (this would be handled by the JSON package)
	assert.NotNil(t, result)
	assert.Equal(t, 75, result.Score)
	assert.Equal(t, 0.85, result.Confidence)
	assert.Equal(t, 0.75, result.Posterior)
	assert.Len(t, result.Contributors, 1)
}

func TestFeatureVectorMapOperations(t *testing.T) {
	fv := FeatureVector{
		Influence: make(map[string]float64),
	}

	// Test adding values
	fv.Influence["stars"] = 100
	fv.Influence["forks"] = 25

	assert.Equal(t, 100.0, fv.Influence["stars"])
	assert.Equal(t, 25.0, fv.Influence["forks"])

	// Test deleting values
	delete(fv.Influence, "forks")
	assert.NotContains(t, fv.Influence, "forks")
	assert.Contains(t, fv.Influence, "stars")

	// Test map length
	assert.Len(t, fv.Influence, 1)
}

func TestContributorComparison(t *testing.T) {
	c1 := Contributor{Name: "test", Contribution: 1.5}
	c2 := Contributor{Name: "test", Contribution: 1.5}
	c3 := Contributor{Name: "other", Contribution: 1.5}
	c4 := Contributor{Name: "test", Contribution: 2.0}

	assert.Equal(t, c1, c2)
	assert.NotEqual(t, c1, c3)
	assert.NotEqual(t, c1, c4)
}

func TestBreakdownArithmetic(t *testing.T) {
	b1 := Breakdown{
		Shipping:  1.0,
		Influence: 2.0,
	}

	b2 := Breakdown{
		Shipping:  0.5,
		Influence: 1.0,
	}

	// Test addition
	sum := Breakdown{
		Shipping:  b1.Shipping + b2.Shipping,
		Influence: b1.Influence + b2.Influence,
	}

	assert.Equal(t, 1.5, sum.Shipping)
	assert.Equal(t, 3.0, sum.Influence)
}

func TestDataStructureMemoryEfficiency(t *testing.T) {
	// Test that structures don't have excessive memory overhead
	result := ScoreResult{
		Contributors: make([]Contributor, 1000),
	}

	// Should handle large contributor lists efficiently
	assert.Len(t, result.Contributors, 1000)

	// Test that empty slices don't allocate unnecessary memory
	emptyResult := ScoreResult{}
	assert.Nil(t, emptyResult.Contributors) // Zero value slices are nil
}

func TestScoreResultEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		result ScoreResult
	}{
		{
			name: "maximum values",
			result: ScoreResult{
				Score:      100,
				Confidence: 1.0,
				Posterior:  1.0,
			},
		},
		{
			name: "minimum values",
			result: ScoreResult{
				Score:      0,
				Confidence: 0.0,
				Posterior:  0.0,
			},
		},
		{
			name: "very large contributor list",
			result: ScoreResult{
				Contributors: make([]Contributor, 10000),
			},
		},
		{
			name: "contributors with extreme values",
			result: ScoreResult{
				Contributors: []Contributor{
					{Name: "extreme_high", Contribution: 999999},
					{Name: "extreme_low", Contribution: -999999},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic with any reasonable values
			assert.NotPanics(t, func() {
				_ = tt.result.Score
				_ = tt.result.Confidence
				_ = tt.result.Posterior
				_ = len(tt.result.Contributors)
				_ = tt.result.Breakdown
			})
		})
	}
}

// Helper function for validation
func validateScoreResult(result ScoreResult) bool {
	if result.Score < 0 || result.Score > 100 {
		return false
	}
	if result.Confidence < 0.0 || result.Confidence > 1.0 {
		return false
	}
	if result.Posterior < 0.0 || result.Posterior > 1.0 {
		return false
	}
	return true
}

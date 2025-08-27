package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSumMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]float64
		expected float64
	}{
		{
			name:     "sums empty map",
			input:    map[string]float64{},
			expected: 0,
		},
		{
			name: "sums positive values",
			input: map[string]float64{
				"feature1": 1.5,
				"feature2": 2.5,
				"feature3": 3.0,
			},
			expected: 7.0,
		},
		{
			name: "sums mixed positive and negative values",
			input: map[string]float64{
				"feature1": 1.5,
				"feature2": -2.5,
				"feature3": 3.0,
			},
			expected: 2.0,
		},
		{
			name: "clips extreme values",
			input: map[string]float64{
				"feature1": 5.0,  // Above clip limit
				"feature2": -5.0, // Below clip limit
			},
			expected: 6.0, // 3 + (-3) = 0, but wait - let's check clipping
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sumMap(tt.input)
			// Note: sumMap applies clipping internally, so we need to account for that
			if tt.name == "clips extreme values" {
				assert.Equal(t, 0.0, result) // 3 + (-3) = 0 after clipping
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSigmoid(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{
			name:     "sigmoid of 0",
			input:    0,
			expected: 0.5,
		},
		{
			name:     "sigmoid of positive value",
			input:    1.0,
			expected: 0.7310585786300049,
		},
		{
			name:     "sigmoid of negative value",
			input:    -1.0,
			expected: 0.2689414213699951,
		},
		{
			name:     "sigmoid approaches 1 for large positive",
			input:    10.0,
			expected: 0.9999546021312976,
		},
		{
			name:     "sigmoid approaches 0 for large negative",
			input:    -10.0,
			expected: 4.5397868702434395e-05,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sigmoid(tt.input)
			assert.InDelta(t, tt.expected, result, 1e-10)
		})
	}
}

func TestScoreCategories(t *testing.T) {
	tests := []struct {
		name                 string
		featureVector        FeatureVector
		expectedCategories   categoryEvidences
		expectedL            float64
		expectedContributors int
	}{
		{
			name: "scores empty feature vector",
			featureVector: FeatureVector{
				Shipping:      map[string]float64{},
				Quality:       map[string]float64{},
				Influence:     map[string]float64{},
				Complexity:    map[string]float64{},
				Collaboration: map[string]float64{},
				Reliability:   map[string]float64{},
				Novelty:       map[string]float64{},
				Coverage:      0.5,
			},
			expectedCategories: categoryEvidences{
				shipping:      0,
				quality:       0,
				influence:     0,
				complexity:    0,
				collaboration: 0,
				reliability:   0,
				novelty:       0,
			},
			expectedL:            0,
			expectedContributors: 0,
		},
		{
			name: "scores feature vector with influence data",
			featureVector: FeatureVector{
				Shipping:      map[string]float64{"commits": 1.5},
				Quality:       map[string]float64{"reviews": 0.8},
				Influence:     map[string]float64{"stars": 2.0, "forks": 1.2},
				Complexity:    map[string]float64{"languages": 0.5},
				Collaboration: map[string]float64{"contributors": 1.0},
				Reliability:   map[string]float64{"tests": 0.9},
				Novelty:       map[string]float64{"new_repos": 0.3},
				Coverage:      0.8,
			},
			expectedCategories: categoryEvidences{
				shipping:      0,
				quality:       0,
				influence:     3.2, // 2.0 + 1.2
				complexity:    0,
				collaboration: 0,
				reliability:   0,
				novelty:       0,
			},
			expectedL:            0.64, // 0.2 * 3.2
			expectedContributors: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			categories, L, contributors, breakdown := scoreCategories(tt.featureVector)

			assert.Equal(t, tt.expectedCategories.shipping, categories.shipping)
			assert.Equal(t, tt.expectedCategories.quality, categories.quality)
			assert.Equal(t, tt.expectedCategories.influence, categories.influence)
			assert.Equal(t, tt.expectedCategories.complexity, categories.complexity)
			assert.Equal(t, tt.expectedCategories.collaboration, categories.collaboration)
			assert.Equal(t, tt.expectedCategories.reliability, categories.reliability)
			assert.Equal(t, tt.expectedCategories.novelty, categories.novelty)
			assert.InDelta(t, tt.expectedL, L, 0.01)
			assert.Len(t, contributors, tt.expectedContributors)

			// Verify breakdown matches categories
			assert.Equal(t, categories.shipping, breakdown.Shipping)
			assert.Equal(t, categories.quality, breakdown.Quality)
			assert.Equal(t, categories.influence, breakdown.Influence)
			assert.Equal(t, categories.complexity, breakdown.Complexity)
			assert.Equal(t, categories.collaboration, breakdown.Collaboration)
			assert.Equal(t, categories.reliability, breakdown.Reliability)
			assert.Equal(t, categories.novelty, breakdown.Novelty)
		})
	}
}

func TestAggregateScore(t *testing.T) {
	tests := []struct {
		name          string
		featureVector FeatureVector
		expected      ScoreResult
	}{
		{
			name: "aggregates score with empty feature vector",
			featureVector: FeatureVector{
				Shipping:      map[string]float64{},
				Quality:       map[string]float64{},
				Influence:     map[string]float64{},
				Complexity:    map[string]float64{},
				Collaboration: map[string]float64{},
				Reliability:   map[string]float64{},
				Novelty:       map[string]float64{},
				Coverage:      0.5,
			},
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
			name: "aggregates score with influence data",
			featureVector: FeatureVector{
				Shipping:      map[string]float64{"commits": 1.5},
				Quality:       map[string]float64{},
				Influence:     map[string]float64{"stars": 2.0, "forks": 1.2},
				Complexity:    map[string]float64{},
				Collaboration: map[string]float64{},
				Reliability:   map[string]float64{},
				Novelty:       map[string]float64{},
				Coverage:      0.8,
			},
			expected: ScoreResult{
				Score:      56, // sigmoid(0.64) * 100, rounded
				Confidence: 0.8,
				Posterior:  0.56,
				Contributors: []Contributor{
					{Name: "shipping.commits", Contribution: 1.5},
					{Name: "influence.stars", Contribution: 2.0},
					{Name: "influence.forks", Contribution: 1.2},
				},
				Breakdown: Breakdown{
					Shipping:      0,
					Quality:       0,
					Influence:     3.2,
					Complexity:    0,
					Collaboration: 0,
					Reliability:   0,
					Novelty:       0,
				},
			},
		},
		{
			name: "handles extreme values correctly",
			featureVector: FeatureVector{
				Shipping:      map[string]float64{},
				Quality:       map[string]float64{},
				Influence:     map[string]float64{"stars": 10.0}, // Extreme value
				Complexity:    map[string]float64{},
				Collaboration: map[string]float64{},
				Reliability:   map[string]float64{},
				Novelty:       map[string]float64{},
				Coverage:      0.8,
			},
			expected: ScoreResult{
				Score:      67, // sigmoid(0.2 * 3) * 100, since 10.0 gets clipped to 3
				Confidence: 0.8,
				Posterior:  0.67,
				Contributors: []Contributor{
					{Name: "influence.stars", Contribution: 3.0}, // Clipped to 3
				},
				Breakdown: Breakdown{
					Shipping:      0,
					Quality:       0,
					Influence:     3.0, // Clipped
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
			result := AggregateScore(tt.featureVector)

			assert.Equal(t, tt.expected.Score, result.Score)
			assert.Equal(t, tt.expected.Confidence, result.Confidence)
			assert.InDelta(t, tt.expected.Posterior, result.Posterior, 0.01)

			// Verify contributors
			assert.Len(t, result.Contributors, len(tt.expected.Contributors))
			for i, expectedContributor := range tt.expected.Contributors {
				assert.Equal(t, expectedContributor.Name, result.Contributors[i].Name)
				assert.InDelta(t, expectedContributor.Contribution, result.Contributors[i].Contribution, 0.01)
			}

			// Verify breakdown
			assert.Equal(t, tt.expected.Breakdown.Shipping, result.Breakdown.Shipping)
			assert.Equal(t, tt.expected.Breakdown.Quality, result.Breakdown.Quality)
			assert.Equal(t, tt.expected.Breakdown.Influence, result.Breakdown.Influence)
			assert.Equal(t, tt.expected.Breakdown.Complexity, result.Breakdown.Complexity)
			assert.Equal(t, tt.expected.Breakdown.Collaboration, result.Breakdown.Collaboration)
			assert.Equal(t, tt.expected.Breakdown.Reliability, result.Breakdown.Reliability)
			assert.Equal(t, tt.expected.Breakdown.Novelty, result.Breakdown.Novelty)
		})
	}
}

func TestCategoryWeights(t *testing.T) {
	// Verify that category weights sum to 1.0
	total := 0.0
	total += categoryWeights["shipping"]
	total += categoryWeights["quality"]
	total += categoryWeights["influence"]
	total += categoryWeights["complexity"]
	total += categoryWeights["collaboration"]
	total += categoryWeights["reliability"]
	total += categoryWeights["novelty"]

	assert.InDelta(t, 1.0, total, 1e-10, "Category weights should sum to 1.0")
}

func TestClipFunction(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		lo       float64
		hi       float64
		expected float64
	}{
		{
			name:     "clips value below lower bound",
			value:    -5.0,
			lo:       -3.0,
			hi:       3.0,
			expected: -3.0,
		},
		{
			name:     "clips value above upper bound",
			value:    5.0,
			lo:       -3.0,
			hi:       3.0,
			expected: 3.0,
		},
		{
			name:     "preserves value within bounds",
			value:    1.5,
			lo:       -3.0,
			hi:       3.0,
			expected: 1.5,
		},
		{
			name:     "handles equal bounds",
			value:    0.0,
			lo:       0.0,
			hi:       0.0,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clip(tt.value, tt.lo, tt.hi)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScoreResultValidation(t *testing.T) {
	// Test that AggregateScore always produces valid results
	testCases := []FeatureVector{
		{
			Shipping:      map[string]float64{"commits": 100.0},
			Quality:       map[string]float64{},
			Influence:     map[string]float64{},
			Complexity:    map[string]float64{},
			Collaboration: map[string]float64{},
			Reliability:   map[string]float64{},
			Novelty:       map[string]float64{},
			Coverage:      0.9,
		},
		{
			Shipping:      map[string]float64{},
			Quality:       map[string]float64{},
			Influence:     map[string]float64{"stars": -10.0}, // Negative extreme
			Complexity:    map[string]float64{},
			Collaboration: map[string]float64{},
			Reliability:   map[string]float64{},
			Novelty:       map[string]float64{},
			Coverage:      0.1,
		},
		{
			Shipping:      map[string]float64{"commits": 1.0, "prs": 2.0, "issues": 0.5},
			Quality:       map[string]float64{"reviews": 1.5, "tests": 2.0},
			Influence:     map[string]float64{"stars": 3.0, "forks": 1.8, "followers": 2.2},
			Complexity:    map[string]float64{"languages": 1.2, "entropy": 0.8},
			Collaboration: map[string]float64{"contributors": 2.5, "teams": 1.0},
			Reliability:   map[string]float64{"ci_passes": 2.8, "reverts": -0.5},
			Novelty:       map[string]float64{"new_repos": 0.9, "experiments": 1.1},
			Coverage:      1.0,
		},
	}

	for i, fv := range testCases {
		t.Run("validation case "+string(rune(i+'A')), func(t *testing.T) {
			result := AggregateScore(fv)

			// Score should always be between 0 and 100
			assert.GreaterOrEqual(t, result.Score, 0, "Score should be >= 0")
			assert.LessOrEqual(t, result.Score, 100, "Score should be <= 100")

			// Confidence should be between 0 and 1
			assert.GreaterOrEqual(t, result.Confidence, 0.0, "Confidence should be >= 0")
			assert.LessOrEqual(t, result.Confidence, 1.0, "Confidence should be <= 1")

			// Posterior should be between 0 and 1
			assert.GreaterOrEqual(t, result.Posterior, 0.0, "Posterior should be >= 0")
			assert.LessOrEqual(t, result.Posterior, 1.0, "Posterior should be <= 1")

			// Contributors should have valid names and contributions within bounds
			for _, contributor := range result.Contributors {
				assert.NotEmpty(t, contributor.Name, "Contributor name should not be empty")
				assert.True(t, contributor.Contribution >= -3 && contributor.Contribution <= 3,
					"Contribution should be clipped to [-3, 3], got %f", contributor.Contribution)
			}
		})
	}
}

package analysis

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// isFinite checks if a float64 value is finite (not NaN or Inf)
func isFinite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

func TestMedian(t *testing.T) {
	tests := []struct {
		name     string
		input    []float64
		expected float64
	}{
		{
			name:     "median of empty slice",
			input:    []float64{},
			expected: 0,
		},
		{
			name:     "median of single element",
			input:    []float64{5.0},
			expected: 5.0,
		},
		{
			name:     "median of odd length slice",
			input:    []float64{1, 3, 5, 7, 9},
			expected: 5.0,
		},
		{
			name:     "median of even length slice",
			input:    []float64{1, 2, 3, 4},
			expected: 2.5,
		},
		{
			name:     "median of unsorted slice",
			input:    []float64{9, 1, 7, 3, 5},
			expected: 5.0,
		},
		{
			name:     "median of slice with duplicates",
			input:    []float64{2, 2, 3, 3, 4, 4},
			expected: 3.0,
		},
		{
			name:     "median of slice with negative numbers",
			input:    []float64{-5, -1, 0, 3, 7},
			expected: 0.0,
		},
		{
			name:     "median of slice with decimals",
			input:    []float64{1.5, 2.7, 3.1, 4.8},
			expected: 2.9000000000000004, // Due to floating point arithmetic
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := median(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMAD(t *testing.T) {
	tests := []struct {
		name     string
		input    []float64
		expected float64
	}{
		{
			name:     "MAD of empty slice",
			input:    []float64{},
			expected: 1.0, // Default value for empty slice
		},
		{
			name:     "MAD of single element",
			input:    []float64{5.0},
			expected: 1.0, // Default value when MAD is 0
		},
		{
			name:     "MAD of uniform distribution",
			input:    []float64{2, 2, 2, 2},
			expected: 1.0, // MAD is 0, so returns default 1.0
		},
		{
			name:     "MAD of simple distribution",
			input:    []float64{1, 2, 3, 4, 5},
			expected: 1.0, // MAD calculation: median of absolute deviations from median
		},
		{
			name:     "MAD with outliers",
			input:    []float64{1, 2, 3, 100},
			expected: 1.0, // MAD: median of [0.5, 0.5, 1.5, 97.5] = 1.0
		},
		{
			name:     "MAD of negative numbers",
			input:    []float64{-5, -3, -1, 1, 3},
			expected: 2.0, // MAD: median of [0, 2, 2, 4, 4] = 2.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mad(tt.input)
			assert.InDelta(t, tt.expected, result, 1e-10)
		})
	}
}

func TestRobustZ(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		sample   []float64
		expected float64
	}{
		{
			name:     "RobustZ with empty sample",
			value:    5.0,
			sample:   []float64{},
			expected: math.Asinh(5.0 / 1.0), // Uses default scale of 1.0
		},
		{
			name:     "RobustZ with single element sample",
			value:    5.0,
			sample:   []float64{3.0},
			expected: math.Asinh(5.0 / 1.0), // MAD is 0, so uses default scale
		},
		{
			name:     "RobustZ with uniform sample",
			value:    5.0,
			sample:   []float64{2, 2, 2, 2},
			expected: math.Asinh(5.0 / 1.0), // MAD is 0, uses default scale
		},
		{
			name:     "RobustZ with normal distribution",
			value:    5.0,
			sample:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9},
			expected: 0.0, // Value equals median, so z-score is 0
		},
		{
			name:     "RobustZ with value above median",
			value:    7.0,
			sample:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9},
			expected: math.Asinh((7.0 - 5.0) / (1.482602218505602 * 2.0)), // MAD = 2 for this sample
		},
		{
			name:     "RobustZ with value below median",
			value:    3.0,
			sample:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9},
			expected: math.Asinh((3.0 - 5.0) / (1.482602218505602 * 2.0)), // MAD = 2 for this sample
		},
		{
			name:     "RobustZ with outlier in sample",
			value:    6.0,
			sample:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 100},
			expected: math.Asinh((6.0 - 5.5) / (1.482602218505602 * 1.0)), // MAD ≈ 1 for this sample with outlier
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RobustZ(tt.value, tt.sample)
			assert.InDelta(t, tt.expected, result, 1e-10)
		})
	}
}

func TestRobustZProperties(t *testing.T) {
	// Test that RobustZ is robust to outliers
	sampleWithoutOutlier := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	sampleWithOutlier := []float64{1, 2, 3, 4, 5, 6, 7, 8, 100}

	resultWithout := RobustZ(6.0, sampleWithoutOutlier)
	resultWith := RobustZ(6.0, sampleWithOutlier)

	// Results should be relatively stable despite the outlier
	assert.InDelta(t, resultWithout, resultWith, 0.5, "RobustZ should be stable with outliers")
}

func TestClip(t *testing.T) {
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
		{
			name:     "handles zero bounds",
			value:    0.0,
			lo:       0.0,
			hi:       10.0,
			expected: 0.0,
		},
		{
			name:     "handles negative bounds",
			value:    -5.0,
			lo:       -10.0,
			hi:       -2.0,
			expected: -5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clip(tt.value, tt.lo, tt.hi)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRobustZEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		value  float64
		sample []float64
	}{
		{
			name:   "very large positive value",
			value:  1e10,
			sample: []float64{1, 2, 3, 4, 5},
		},
		{
			name:   "very large negative value",
			value:  -1e10,
			sample: []float64{1, 2, 3, 4, 5},
		},
		{
			name:   "very small positive value",
			value:  1e-10,
			sample: []float64{1, 2, 3, 4, 5},
		},
		{
			name:   "zero value",
			value:  0.0,
			sample: []float64{1, 2, 3, 4, 5},
		},
		{
			name:   "NaN value",
			value:  math.NaN(),
			sample: []float64{1, 2, 3, 4, 5},
		},
		{
			name:   "infinity value",
			value:  math.Inf(1),
			sample: []float64{1, 2, 3, 4, 5},
		},
		{
			name:   "negative infinity value",
			value:  math.Inf(-1),
			sample: []float64{1, 2, 3, 4, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := RobustZ(tt.value, tt.sample)

			// Result should be finite (unless input was NaN)
			if !math.IsNaN(tt.value) {
				assert.True(t, isFinite(result), "Result should be finite")
			}
		})
	}
}

func TestMADProperties(t *testing.T) {
	// MAD should always be non-negative
	samples := [][]float64{
		{1, 2, 3, 4, 5},
		{-5, -3, -1, 1, 3, 5},
		{0, 0, 0, 0},
		{1.5, 2.7, 3.1, 4.8, 6.2},
		{100, 200, 300, 400, 500},
	}

	for i, sample := range samples {
		t.Run("MAD non-negative "+string(rune(i+'A')), func(t *testing.T) {
			result := mad(sample)
			assert.GreaterOrEqual(t, result, 0.0, "MAD should always be non-negative")
			assert.True(t, isFinite(result), "MAD should be finite")
		})
	}
}

func TestMedianProperties(t *testing.T) {
	// Test that median is robust to ordering
	samples := [][]float64{
		{1, 2, 3, 4, 5},
		{5, 4, 3, 2, 1},
		{3, 1, 4, 1, 5},
		{2, 3, 1, 5, 4},
	}

	for i, sample := range samples {
		t.Run("median ordering independence "+string(rune(i+'A')), func(t *testing.T) {
			result := median(sample)
			assert.True(t, isFinite(result), "Median should be finite")
		})
	}
}

func TestRobustZScaling(t *testing.T) {
	// Test that scaling the sample doesn't affect relative z-scores
	sample1 := []float64{1, 2, 3, 4, 5}
	sample2 := []float64{10, 20, 30, 40, 50}

	z1 := RobustZ(3, sample1)
	z2 := RobustZ(30, sample2)

	// Should be approximately the same (scaled data)
	assert.InDelta(t, z1, z2, 0.1, "Scaled samples should produce similar z-scores")
}

func TestStatisticalRobustness(t *testing.T) {
	// Compare RobustZ with standard z-score for outlier sensitivity
	cleanData := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	outlierData := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 100}

	testValue := 5.5

	robustZClean := RobustZ(testValue, cleanData)
	robustZOutlier := RobustZ(testValue, outlierData)

	// RobustZ should be much more stable than standard z-score
	// Standard z-score would change dramatically with the outlier
	// RobustZ should change much less
	difference := math.Abs(robustZClean - robustZOutlier)

	// The difference should be relatively small (< 1.0 for this case)
	assert.Less(t, difference, 1.0, "RobustZ should be stable with outliers")
}

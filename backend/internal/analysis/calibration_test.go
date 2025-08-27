package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCalibrationStore(t *testing.T) {
	tests := []struct {
		name     string
		dataDir  string
		expected string
	}{
		{
			name:     "creates calibration store with valid data directory",
			dataDir:  "./test_data",
			expected: "./test_data",
		},
		{
			name:     "creates calibration store with empty data directory",
			dataDir:  "",
			expected: "",
		},
		{
			name:     "creates calibration store with relative path",
			dataDir:  "../data",
			expected: "../data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewCalibrationStore(tt.dataDir)
			assert.NotNil(t, store)
			assert.Equal(t, tt.expected, store.dataDir)
		})
	}
}

func TestCalibrationStore_LoadCalibration(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "calibration_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	store := NewCalibrationStore(tempDir)

	tests := []struct {
		name     string
		domain   string
		expected *CalibrationData
		hasError bool
	}{
		{
			name:     "loads default calibration for non-existent domain",
			domain:   "nonexistent",
			expected: store.getDefaultCalibration(),
			hasError: false,
		},
		{
			name:     "loads default calibration for empty domain",
			domain:   "",
			expected: store.getDefaultCalibration(),
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := store.LoadCalibration(tt.domain)

			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assertEqualCalibrationData(t, tt.expected, result)
			}
		})
	}
}

func TestCalibrationStore_SaveCalibration(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "calibration_save_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	store := NewCalibrationStore(tempDir)

	testData := &CalibrationData{
		Shipping:      []float64{10, 20, 30, 40, 50},
		Quality:       []float64{0.1, 0.2, 0.3, 0.4, 0.5},
		Influence:     []float64{100, 200, 300, 400, 500},
		Complexity:    []float64{0.2, 0.4, 0.6, 0.8, 1.0},
		Collaboration: []float64{1, 2, 3, 4, 5},
		Reliability:   []float64{0.8, 0.85, 0.9, 0.95, 1.0},
		Novelty:       []float64{0.1, 0.2, 0.3, 0.4, 0.5},
	}

	tests := []struct {
		name     string
		domain   string
		data     *CalibrationData
		hasError bool
	}{
		{
			name:     "saves calibration data successfully",
			domain:   "test_domain",
			data:     testData,
			hasError: false,
		},
		{
			name:     "saves calibration with special characters in domain",
			domain:   "test-domain.com",
			data:     testData,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.SaveCalibration(tt.domain, tt.data)

			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify file was created
				filePath := filepath.Join(tempDir, tt.domain+".json")
				assert.FileExists(t, filePath)

				// Verify we can load it back
				loaded, err := store.LoadCalibration(tt.domain)
				assert.NoError(t, err)
				assertEqualCalibrationData(t, tt.data, loaded)
			}
		})
	}
}

func TestCalibrationStore_SaveAndLoadIntegration(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "calibration_integration_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	store := NewCalibrationStore(tempDir)

	originalData := &CalibrationData{
		Shipping:      []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		Quality:       []float64{0.1, 0.15, 0.2, 0.25, 0.3},
		Influence:     []float64{50, 100, 150, 200, 250, 300},
		Complexity:    []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6},
		Collaboration: []float64{1, 3, 5, 7, 9},
		Reliability:   []float64{0.9, 0.92, 0.94, 0.96, 0.98, 1.0},
		Novelty:       []float64{0.05, 0.1, 0.15, 0.2, 0.25, 0.3},
	}

	domain := "integration_test"

	// Save the data
	err = store.SaveCalibration(domain, originalData)
	assert.NoError(t, err)

	// Load it back
	loadedData, err := store.LoadCalibration(domain)
	assert.NoError(t, err)

	// Verify all fields match
	assertEqualCalibrationData(t, originalData, loadedData)
}

func TestCalibrationStore_getDefaultCalibration(t *testing.T) {
	store := NewCalibrationStore("./test")

	defaultCal := store.getDefaultCalibration()

	// Verify structure
	assert.NotNil(t, defaultCal)
	assert.NotEmpty(t, defaultCal.Shipping)
	assert.NotEmpty(t, defaultCal.Quality)
	assert.NotEmpty(t, defaultCal.Influence)
	assert.NotEmpty(t, defaultCal.Complexity)
	assert.NotEmpty(t, defaultCal.Collaboration)
	assert.NotEmpty(t, defaultCal.Reliability)
	assert.NotEmpty(t, defaultCal.Novelty)

	// Verify data is reasonable
	assert.Greater(t, len(defaultCal.Shipping), 0)
	assert.Greater(t, len(defaultCal.Influence), 0)
	assert.Greater(t, len(defaultCal.Quality), 0)

	// Verify influence data is sorted (should be increasing)
	for i := 1; i < len(defaultCal.Influence); i++ {
		assert.GreaterOrEqual(t, defaultCal.Influence[i], defaultCal.Influence[i-1], "Influence data should be non-decreasing")
	}

	// Verify quality scores are between 0 and 1
	for _, score := range defaultCal.Quality {
		assert.GreaterOrEqual(t, score, 0.0, "Quality scores should be >= 0")
		assert.LessOrEqual(t, score, 1.0, "Quality scores should be <= 1")
	}

	// Verify reliability scores are between 0 and 1
	for _, score := range defaultCal.Reliability {
		assert.GreaterOrEqual(t, score, 0.0, "Reliability scores should be >= 0")
		assert.LessOrEqual(t, score, 1.0, "Reliability scores should be <= 1")
	}

	// Verify complexity scores are between 0 and 1
	for _, score := range defaultCal.Complexity {
		assert.GreaterOrEqual(t, score, 0.0, "Complexity scores should be >= 0")
		assert.LessOrEqual(t, score, 1.0, "Complexity scores should be <= 1")
	}

	// Verify novelty scores are between 0 and 1
	for _, score := range defaultCal.Novelty {
		assert.GreaterOrEqual(t, score, 0.0, "Novelty scores should be >= 0")
		assert.LessOrEqual(t, score, 1.0, "Novelty scores should be <= 1")
	}
}

func TestCalibrationStore_BootstrapCalibration(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "calibration_bootstrap_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	store := NewCalibrationStore(tempDir)

	testData := map[string]*CalibrationData{
		"domain1": {
			Shipping:  []float64{1, 2, 3},
			Influence: []float64{10, 20, 30},
		},
		"domain2": {
			Shipping:  []float64{4, 5, 6},
			Influence: []float64{40, 50, 60},
		},
	}

	err = store.BootstrapCalibration(testData)
	assert.NoError(t, err)

	// Verify both domains were saved
	for _, domain := range []string{"domain1", "domain2"} {
		loaded, err := store.LoadCalibration(domain)
		assert.NoError(t, err)
		assert.NotNil(t, loaded)
		assertEqualCalibrationData(t, testData[domain], loaded)
	}
}

func TestCalibrationDataJSONSerialization(t *testing.T) {
	original := &CalibrationData{
		Shipping:      []float64{1.1, 2.2, 3.3},
		Quality:       []float64{0.1, 0.2, 0.3},
		Influence:     []float64{100, 200, 300},
		Complexity:    []float64{0.1, 0.2, 0.3},
		Collaboration: []float64{1, 2, 3},
		Reliability:   []float64{0.8, 0.9, 1.0},
		Novelty:       []float64{0.1, 0.2, 0.3},
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "calibration_json_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	store := NewCalibrationStore(tempDir)
	domain := "json_test"

	// Save and reload
	err = store.SaveCalibration(domain, original)
	assert.NoError(t, err)

	loaded, err := store.LoadCalibration(domain)
	assert.NoError(t, err)

	assertEqualCalibrationData(t, original, loaded)
}

func TestCalibrationStore_ErrorHandling(t *testing.T) {
	// Test with invalid directory
	store := NewCalibrationStore("/nonexistent/directory")

	// Should handle gracefully and return default calibration
	calibration, err := store.LoadCalibration("test")
	assert.NoError(t, err) // Should not error, just return default
	assert.NotNil(t, calibration)
	assertEqualCalibrationData(t, store.getDefaultCalibration(), calibration)
}

func TestCalibrationStore_FileOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "calibration_file_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	store := NewCalibrationStore(tempDir)

	// Test creating directory structure
	data := &CalibrationData{
		Shipping: []float64{1, 2, 3},
	}

	// Create a nested domain that requires directory creation
	domain := "nested/deep/domain"
	err = store.SaveCalibration(domain, data)
	assert.NoError(t, err)

	// Verify file exists
	filePath := filepath.Join(tempDir, domain+".json")
	assert.FileExists(t, filePath)

	// Verify we can load it
	loaded, err := store.LoadCalibration(domain)
	assert.NoError(t, err)
	assertEqualCalibrationData(t, data, loaded)
}

// Helper function to compare CalibrationData structs
func assertEqualCalibrationData(t *testing.T, expected, actual *CalibrationData) {
	assert.Equal(t, expected.Shipping, actual.Shipping)
	assert.Equal(t, expected.Quality, actual.Quality)
	assert.Equal(t, expected.Influence, actual.Influence)
	assert.Equal(t, expected.Complexity, actual.Complexity)
	assert.Equal(t, expected.Collaboration, actual.Collaboration)
	assert.Equal(t, expected.Reliability, actual.Reliability)
	assert.Equal(t, expected.Novelty, actual.Novelty)
}

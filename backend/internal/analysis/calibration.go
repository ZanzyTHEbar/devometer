package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CalibrationData holds baselines for robust normalization
type CalibrationData struct {
	Shipping      []float64 `json:"shipping"`
	Quality       []float64 `json:"quality"`
	Influence     []float64 `json:"influence"`
	Complexity    []float64 `json:"complexity"`
	Collaboration []float64 `json:"collaboration"`
	Reliability   []float64 `json:"reliability"`
	Novelty       []float64 `json:"novelty"`
}

// CalibrationStore manages calibration data by domain
type CalibrationStore struct {
	dataDir string
}

// NewCalibrationStore creates a new calibration store
func NewCalibrationStore(dataDir string) *CalibrationStore {
	return &CalibrationStore{dataDir: dataDir}
}

// LoadCalibration loads calibration data for a domain
func (c *CalibrationStore) LoadCalibration(domain string) (*CalibrationData, error) {
	filePath := filepath.Join(c.dataDir, fmt.Sprintf("%s.json", domain))

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Return default calibration if file doesn't exist
		return c.getDefaultCalibration(), nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open calibration file: %w", err)
	}
	defer file.Close()

	var data CalibrationData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode calibration data: %w", err)
	}

	return &data, nil
}

// SaveCalibration saves calibration data for a domain
func (c *CalibrationStore) SaveCalibration(domain string, data *CalibrationData) error {
	// Ensure the data directory exists
	if err := os.MkdirAll(c.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create calibration directory: %w", err)
	}

	filePath := filepath.Join(c.dataDir, fmt.Sprintf("%s.json", domain))

	// Create the directory path for the file if it doesn't exist
	fileDir := filepath.Dir(filePath)
	if err := os.MkdirAll(fileDir, 0755); err != nil {
		return fmt.Errorf("failed to create calibration file directory: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create calibration file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode calibration data: %w", err)
	}

	return nil
}

// getDefaultCalibration returns default calibration baselines
func (c *CalibrationStore) getDefaultCalibration() *CalibrationData {
	return &CalibrationData{
		Shipping:      []float64{0, 1, 2, 5, 10, 20, 50, 100},      // commits/PRs per period
		Quality:       []float64{0, 0.2, 0.5, 0.8, 0.9, 0.95, 1.0}, // review depth/CI pass rate (capped at 1.0)
		Influence:     []float64{0, 1, 5, 15, 50, 150, 500},        // stars/followers - baseline for preprocessed values
		Complexity:    []float64{0, 0.1, 0.3, 0.5, 0.7, 0.9, 1.0},  // lang entropy/tests ratio
		Collaboration: []float64{0, 1, 3, 5, 10, 20, 50},           // unique collaborators
		Reliability:   []float64{0, 0.5, 0.7, 0.8, 0.9, 0.95, 1.0}, // CI pass rate/revert rarity
		Novelty:       []float64{0, 0.1, 0.3, 0.5, 0.7, 0.9, 1.0},  // new languages/topics
	}
}

// BootstrapCalibration creates initial calibration from sample data
func (c *CalibrationStore) BootstrapCalibration(sampleData map[string]*CalibrationData) error {
	for domain, data := range sampleData {
		if err := c.SaveCalibration(domain, data); err != nil {
			return fmt.Errorf("failed to save calibration for %s: %w", domain, err)
		}
	}
	return nil
}

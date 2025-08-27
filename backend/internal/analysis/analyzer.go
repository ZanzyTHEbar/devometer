package analysis

import (
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/types"
)

// Analyzer orchestrates the full analysis pipeline
type Analyzer struct {
	preprocessor     *Preprocessor
	calibrationStore *CalibrationStore
}

// NewAnalyzer creates a new analyzer with all components
func NewAnalyzer(dataDir string) *Analyzer {
	return &Analyzer{
		preprocessor:     NewPreprocessor(5 * 60 * 1000000000), // 5 minutes in nanoseconds
		calibrationStore: NewCalibrationStore(dataDir),
	}
}

// AnalyzeEvents analyzes processed events using the full pipeline
func (a *Analyzer) AnalyzeEvents(events []types.RawEvent, domain string) (ScoreResult, error) {
	// Apply preprocessing (anti-gaming rules)
	processedEvents := a.preprocessor.ProcessEvents(events)

	// Build feature vector from events
	fv := a.buildFeatureVectorSimple(processedEvents, domain)

	return AggregateScore(fv), nil
}

// buildFeatureVectorSimple builds a simple FeatureVector from events
func (a *Analyzer) buildFeatureVectorSimple(events []types.RawEvent, domain string) FeatureVector {
	fv := FeatureVector{
		Shipping:      make(map[string]float64),
		Quality:       make(map[string]float64),
		Influence:     make(map[string]float64),
		Complexity:    make(map[string]float64),
		Collaboration: make(map[string]float64),
		Reliability:   make(map[string]float64),
		Novelty:       make(map[string]float64),
		Coverage:      0.5,
	}

	// Simple aggregation for now
	for _, event := range events {
		switch event.Type {
		case "stars":
			fv.Influence["stars"] += event.Count
		case "forks":
			fv.Influence["forks"] += event.Count
		case "followers":
			fv.Influence["followers"] += event.Count
		case "total_stars":
			fv.Influence["total_stars"] += event.Count
		}
	}

	// Apply robust z-score transformation
	calibration, err := a.calibrationStore.LoadCalibration(domain)
	if err != nil {
		calibration = a.calibrationStore.getDefaultCalibration()
	}

	for key, value := range fv.Influence {
		fv.Influence[key] = RobustZ(value, calibration.Influence)
	}

	// Boost coverage if we have data
	if len(events) > 0 {
		fv.Coverage = 0.8
	}

	return fv
}

// Legacy function for backward compatibility
func AnalyzeInput(input string) ScoreResult {
	analyzer := NewAnalyzer("./data")
	events := []types.RawEvent{}
	result, _ := analyzer.AnalyzeEvents(events, input)
	return result
}

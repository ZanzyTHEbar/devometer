package analysis

import (
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/types"
)

// EventAggregator helps aggregate events by type
type EventAggregator struct {
	data map[string]float64
}

func NewEventAggregator() *EventAggregator {
	return &EventAggregator{data: make(map[string]float64)}
}

func (ea *EventAggregator) Add(key string, value float64) {
	ea.data[key] += value
}

func (ea *EventAggregator) Get(key string) float64 {
	return ea.data[key]
}

func (ea *EventAggregator) GetAll() map[string]float64 {
	return ea.data
}

// Analyzer orchestrates the full analysis pipeline
type Analyzer struct {
	preprocessor     *Preprocessor
	calibrationStore *CalibrationStore
}

// NewAnalyzer creates a new analyzer with all components
func NewAnalyzer(dataDir string) *Analyzer {
	return &Analyzer{
		preprocessor:     NewPreprocessor(5 * time.Minute), // 5 min min spacing for duplicates
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

// AnalyzeEventsWithX analyzes events from both GitHub and X (Twitter) using the full pipeline
func (a *Analyzer) AnalyzeEventsWithX(githubEvents []types.RawEvent, xEvents []types.RawEvent, domain string) (ScoreResult, error) {
	// Apply preprocessing to GitHub events (anti-gaming rules)
	processedGitHubEvents := a.preprocessor.ProcessEvents(githubEvents)

	// Combine GitHub and X events
	allEvents := append(processedGitHubEvents, xEvents...)

	// Build feature vector from combined events
	fv := a.buildFeatureVectorWithX(allEvents, domain)

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

// buildFeatureVectorWithX builds a FeatureVector from both GitHub and X events
func (a *Analyzer) buildFeatureVectorWithX(events []types.RawEvent, domain string) FeatureVector {
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

	// Process events and categorize them
	for _, event := range events {
		switch event.Type {
		// GitHub events (existing logic)
		case "stars":
			fv.Influence["github_stars"] += event.Count
		case "forks":
			fv.Influence["github_forks"] += event.Count
		case "followers":
			fv.Influence["github_followers"] += event.Count
		case "total_stars":
			fv.Influence["github_total_stars"] += event.Count
		case "merged_pr":
			fv.Shipping["merged_prs"] += event.Count
		case "commit":
			fv.Shipping["commits"] += event.Count
		case "language":
			fv.Complexity["languages"] += event.Count
		case "total_forks":
			fv.Influence["github_total_forks"] += event.Count

		// X (Twitter) events (new integration)
		case "twitter_followers":
			fv.Influence["twitter_followers"] += event.Count
		case "twitter_following":
			fv.Influence["twitter_following"] += event.Count
		case "twitter_tweets":
			fv.Novelty["twitter_tweets"] += event.Count
		case "twitter_likes":
			fv.Influence["twitter_likes"] += event.Count
		case "twitter_retweets":
			fv.Influence["twitter_retweets"] += event.Count
		case "twitter_replies":
			fv.Collaboration["twitter_replies"] += event.Count
		case "twitter_mentions":
			fv.Influence["twitter_mentions"] += event.Count
		case "twitter_engagement_rate":
			fv.Influence["twitter_engagement_rate"] += event.Count
		case "twitter_avg_likes":
			fv.Quality["twitter_avg_likes"] += event.Count
		case "twitter_avg_retweets":
			fv.Influence["twitter_avg_retweets"] += event.Count
		case "twitter_avg_replies":
			fv.Collaboration["twitter_avg_replies"] += event.Count
		case "twitter_tweet":
			fv.Novelty["twitter_tweets"] += event.Count
		case "twitter_hashtag_usage":
			fv.Influence["twitter_hashtag_usage"] += event.Count
		}
	}

	// Load calibration data
	calibration, err := a.calibrationStore.LoadCalibration(domain)
	if err != nil {
		calibration = a.calibrationStore.getDefaultCalibration()
	}

	// Apply robust z-score transformation to influence features
	for key, value := range fv.Influence {
		fv.Influence[key] = RobustZ(value, calibration.Influence)
	}

	// Apply robust z-score transformation to other categories
	for key, value := range fv.Shipping {
		fv.Shipping[key] = RobustZ(value, calibration.Shipping)
	}

	for key, value := range fv.Quality {
		fv.Quality[key] = RobustZ(value, calibration.Quality)
	}

	for key, value := range fv.Collaboration {
		fv.Collaboration[key] = RobustZ(value, calibration.Collaboration)
	}

	for key, value := range fv.Complexity {
		fv.Complexity[key] = RobustZ(value, calibration.Complexity)
	}

	for key, value := range fv.Reliability {
		fv.Reliability[key] = RobustZ(value, calibration.Reliability)
	}

	for key, value := range fv.Novelty {
		fv.Novelty[key] = RobustZ(value, calibration.Novelty)
	}

	// Boost coverage if we have diverse data sources
	eventTypes := make(map[string]bool)
	for _, event := range events {
		eventTypes[event.Type] = true
	}

	// Increase coverage based on data diversity
	if len(eventTypes) > 5 {
		fv.Coverage = 0.9 // High coverage with diverse signals
	} else if len(eventTypes) > 2 {
		fv.Coverage = 0.8 // Good coverage with multiple signals
	} else if len(events) > 0 {
		fv.Coverage = 0.7 // Basic coverage with some data
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

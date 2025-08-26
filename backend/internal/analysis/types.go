package analysis

type FeatureVector struct {
	Shipping      map[string]float64
	Quality       map[string]float64
	Influence     map[string]float64
	Complexity    map[string]float64
	Collaboration map[string]float64
	Reliability   map[string]float64
	Novelty       map[string]float64
	Coverage      float64
}

type Contributor struct {
	Name         string  `json:"name"`
	Contribution float64 `json:"contribution"`
}

type Breakdown struct {
	Shipping      float64 `json:"shipping"`
	Quality       float64 `json:"quality"`
	Influence     float64 `json:"influence"`
	Complexity    float64 `json:"complexity"`
	Collaboration float64 `json:"collaboration"`
	Reliability   float64 `json:"reliability"`
	Novelty       float64 `json:"novelty"`
}

type ScoreResult struct {
	Score        int           `json:"score"`
	Confidence   float64       `json:"confidence"`
	Posterior    float64       `json:"posterior"`
	Contributors []Contributor `json:"contributors"`
	Breakdown    Breakdown     `json:"breakdown"`
}

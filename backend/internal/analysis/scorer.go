package analysis

import "math"

var (
	categoryWeights = map[string]float64{
		"shipping":      0.20,
		"quality":       0.15,
		"influence":     0.35, // Increased influence weight significantly
		"complexity":    0.12,
		"collaboration": 0.08,
		"reliability":   0.06,
		"novelty":       0.04,
	}
	// per-category base bias in log-odds space - increased for higher base scores
	baseBias   float64 = 1.5 // Increased from 0 to 1.5 to boost base scores
	scoreScale float64 = 1.2 // Scaling factor to make scores more sensitive to moderate values
	clipZ      float64 = 3
)

func sumMap(m map[string]float64) float64 {
	s := 0.0
	for _, v := range m {
		s += clip(v, -clipZ, clipZ)
	}
	return s
}

func sigmoid(x float64) float64 { return 1 / (1 + math.Exp(-x)) }

type categoryEvidences struct {
	shipping, quality, influence, complexity, collaboration, reliability, novelty float64
}

func scoreCategories(f FeatureVector) (categoryEvidences, float64, []Contributor, Breakdown) {
	// equal alpha per feature within a category; robust z expected upstream or raw values acceptable for v0
	ce := categoryEvidences{
		shipping:      baseBias + sumMap(f.Shipping),
		quality:       baseBias + sumMap(f.Quality),
		influence:     baseBias + sumMap(f.Influence),
		complexity:    baseBias + sumMap(f.Complexity),
		collaboration: baseBias + sumMap(f.Collaboration),
		reliability:   baseBias + sumMap(f.Reliability),
		novelty:       baseBias + sumMap(f.Novelty),
	}

	// contributors: take top few absolute contributions across all features
	contribs := make([]Contributor, 0, 8)
	appendContribs := func(prefix string, m map[string]float64) {
		for k, v := range m {
			contribs = append(contribs, Contributor{Name: prefix + "." + k, Contribution: clip(v, -clipZ, clipZ)})
		}
	}
	appendContribs("shipping", f.Shipping)
	appendContribs("quality", f.Quality)
	appendContribs("influence", f.Influence)
	appendContribs("complexity", f.Complexity)
	appendContribs("collaboration", f.Collaboration)
	appendContribs("reliability", f.Reliability)
	appendContribs("novelty", f.Novelty)

	breakdown := Breakdown{
		Shipping:      ce.shipping,
		Quality:       ce.quality,
		Influence:     ce.influence,
		Complexity:    ce.complexity,
		Collaboration: ce.collaboration,
		Reliability:   ce.reliability,
		Novelty:       ce.novelty,
	}

	// log-odds aggregate
	L := baseBias +
		categoryWeights["shipping"]*ce.shipping +
		categoryWeights["quality"]*ce.quality +
		categoryWeights["influence"]*ce.influence +
		categoryWeights["complexity"]*ce.complexity +
		categoryWeights["collaboration"]*ce.collaboration +
		categoryWeights["reliability"]*ce.reliability +
		categoryWeights["novelty"]*ce.novelty

	return ce, L, contribs, breakdown
}

func AggregateScore(f FeatureVector) ScoreResult {
	_, L, contribs, breakdown := scoreCategories(f)
	// Apply scaling factor to make the sigmoid more sensitive
	scaledL := L * scoreScale
	p := sigmoid(scaledL)
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	score := int(math.Round(100 * p))
	conf := f.Coverage
	return ScoreResult{
		Score:        score,
		Confidence:   conf,
		Posterior:    p,
		Contributors: contribs,
		Breakdown:    breakdown,
	}
}

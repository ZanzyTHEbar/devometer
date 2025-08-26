package analysis

import (
	"strings"
)

// AnalyzeInput is a temporary analyzer using input-only heuristics.
// It will be replaced with real adapters (GitHub/X) feeding FeatureVector.
func AnalyzeInput(input string) ScoreResult {
	trim := strings.TrimSpace(strings.ToLower(input))
	L := float64(len(trim))
	// naive feature seeds based on simple patterns for v0
	shipping := map[string]float64{
		"star_velocity": clip(L/50, 0, 2),
		"merged_prs":    clip(L/60, 0, 2),
	}
	quality := map[string]float64{
		"review_depth": clip(L/90, 0, 2),
		"ci_pass":      0.3,
	}
	influence := map[string]float64{
		"followers": clip(L/70, 0, 2),
	}
	complexity := map[string]float64{
		"lang_entropy": 0.2,
	}
	collab := map[string]float64{
		"unique_collabs": clip(L/100, 0, 2),
	}
	reliability := map[string]float64{
		"revert_rarity": 0.1,
	}
	novelty := map[string]float64{
		"new_topics": 0.05,
	}

	fv := FeatureVector{
		Shipping:      shipping,
		Quality:       quality,
		Influence:     influence,
		Complexity:    complexity,
		Collaboration: collab,
		Reliability:   reliability,
		Novelty:       novelty,
		Coverage:      0.3,
	}
	return AggregateScore(fv)
}

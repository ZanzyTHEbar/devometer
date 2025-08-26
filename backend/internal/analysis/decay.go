package analysis

import "math"

// DecayWeight computes exp(-deltaDays/tau).
func DecayWeight(deltaDays float64, tau float64) float64 {
	if tau <= 0 {
		return 0
	}
	return math.Exp(-deltaDays / tau)
}

// BlendDualHorizon blends short- and long-horizon aggregates.
func BlendDualHorizon(shortAgg, longAgg, lambda float64) float64 {
	if lambda < 0 {
		lambda = 0
	}
	if lambda > 1 {
		lambda = 1
	}
	return lambda*shortAgg + (1-lambda)*longAgg
}

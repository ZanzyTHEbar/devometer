package analysis

import (
	"math"
	"sort"
)

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return 0.5 * (cp[mid-1] + cp[mid])
}

func mad(xs []float64) float64 {
	if len(xs) == 0 {
		return 1
	}
	m := median(xs)
	res := make([]float64, len(xs))
	for i, v := range xs {
		res[i] = math.Abs(v - m)
	}
	md := median(res)
	if md == 0 {
		return 1
	}
	return md
}

// RobustZ computes asinh((x - med)/(1.4826*MAD)).
func RobustZ(x float64, sample []float64) float64 {
	med := median(sample)
	m := mad(sample)
	s := 1.4826 * m
	if s == 0 {
		s = 1
	}
	return math.Asinh((x - med) / s)
}

func clip(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

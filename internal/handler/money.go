package handler

import "math"

// points converts an amount stored in hundredths of a point to points.
func points(hundredths int64) float64 {
	return float64(hundredths) / 100
}

// toHundredths converts points to hundredths of a point, saturating at the
// largest int64. It reports false if the amount rounds to less than one
// hundredth.
func toHundredths(sum float64) (int64, bool) {
	h := math.Round(sum * 100)
	if !(h >= 1) {
		return 0, false
	}
	if h >= math.MaxInt64 {
		return math.MaxInt64, true
	}
	return int64(h), true
}

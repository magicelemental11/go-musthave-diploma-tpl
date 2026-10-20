package handler

// points converts an amount stored in hundredths of a point to points.
func points(hundredths int64) float64 {
	return float64(hundredths) / 100
}

package internal

import "math"

// Percentage is a helper function, to calculate percentage.
func Percentage(total, current int64, width int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(int64(width)*current) / float64(total)
}

func PercentageRound(total, current int64, width int) float64 {
	return math.Round(Percentage(total, current, width))
}

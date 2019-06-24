package internal

import "math"

// Percentage is a helper function, to calculate percentage.
func Percentage(total, current, width int64) int64 {
	if total <= 0 {
		return 0
	}
	p := float64(width*current) / float64(total)
	return int64(math.Round(p))
}

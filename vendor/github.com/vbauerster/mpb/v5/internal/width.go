package internal

func WidthForBarFiller(reqWidth, available int) int {
	if reqWidth <= 0 || reqWidth >= available {
		return available
	}
	return reqWidth
}

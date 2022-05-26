package parser

// isEscape returns true if byte i is prefixed by an odd number of backslahses.
func isEscape(data []byte, i int) bool {
	if i == 0 {
		return false
	}
	if i == 1 {
		return data[0] == '\\'
	}
	j := i - 1
	for ; j >= 0; j-- {
		if data[j] != '\\' {
			break
		}
	}
	j++
	// odd number of backslahes means escape
	return (i-j)%2 != 0
}

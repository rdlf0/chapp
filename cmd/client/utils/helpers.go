package utils

// splitString splits a string into lines of specified length
func splitString(s string, maxLength int) []string {
	if len(s) <= maxLength {
		return []string{s}
	}

	var lines []string
	for i := 0; i < len(s); i += maxLength {
		end := i + maxLength
		if end > len(s) {
			end = len(s)
		}
		lines = append(lines, s[i:end])
	}
	return lines
}

// SplitString splits a string into lines of specified length (exported function)
func SplitString(s string, maxLength int) []string {
	return splitString(s, maxLength)
}

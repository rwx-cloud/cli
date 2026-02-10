package text

import "strings"

// WrapText breaks s into lines of at most width characters, splitting at word
// boundaries when possible. If width is zero or negative, or the string fits
// within the width, the original string is returned as a single-element slice.
func WrapText(s string, width int) []string {
	if width <= 0 || len(s) <= width {
		return []string{s}
	}

	var lines []string
	for len(s) > width {
		// Find the last space at or before the width limit
		breakAt := strings.LastIndex(s[:width+1], " ")
		if breakAt <= 0 {
			// No space found; break at width
			breakAt = width
		}
		lines = append(lines, s[:breakAt])
		s = strings.TrimLeft(s[breakAt:], " ")
	}
	if len(s) > 0 {
		lines = append(lines, s)
	}
	return lines
}

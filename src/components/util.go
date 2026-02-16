package components

import (
	"strings"
	"unicode/utf8"
)

func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func wrapText(s string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	s = strings.ReplaceAll(s, "\t", "  ")
	paras := strings.Split(s, "\n")
	var out []string
	for _, p := range paras {
		p = strings.TrimRight(p, "\r")
		if p == "" {
			out = append(out, "")
			continue
		}

		words := strings.Fields(p)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}

		line := words[0]
		for _, w := range words[1:] {
			if utf8.RuneCountInString(line)+1+utf8.RuneCountInString(w) <= width {
				line += " " + w
				continue
			}
			out = append(out, line)
			line = w
		}
		out = append(out, line)
	}
	return out
}

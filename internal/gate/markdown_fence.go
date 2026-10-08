package gate

import "strings"

// markdownFence preserves literal code through EOF when a fence is unclosed.
type markdownFence struct {
	marker byte
	length int
}

// consume includes delimiters; closers require the same marker and length.
func (fence *markdownFence) consume(line string) bool {
	line = strings.TrimSuffix(line, "\r")
	start := 0
	for start < len(line) && line[start] == ' ' {
		start++
	}
	if start > 3 || start == len(line) || line[start] != '`' && line[start] != '~' {
		return fence.length != 0
	}
	marker := line[start]
	end := start
	for end < len(line) && line[end] == marker {
		end++
	}
	length := end - start
	if fence.length != 0 {
		if marker == fence.marker && length >= fence.length && strings.Trim(line[end:], " \t") == "" {
			*fence = markdownFence{}
		}
		return true
	}
	if length < 3 || marker == '`' && strings.ContainsRune(line[end:], '`') {
		return false
	}
	fence.marker, fence.length = marker, length
	return true
}

package generator

import "strings"

// Canon: §Graph — reusable CLAUDE.md sentinel-block operations.
//
// RULE: the BEGIN/END sentinel-pair pattern (e.g. "<!-- LIVE-STATE:BEGIN -->"
// ... "<!-- LIVE-STATE:END -->") is the structural backbone of every
// generated CLAUDE.md. Three operations recur:
//
//  1. WrapBlock(name, content) — wrap content in its sentinel pair.
//  2. ExtractBlock(text, name) — pull the inner text between sentinels.
//  3. ReplaceBlock(text, name, content) — splice new content between
//     sentinels, preserving the surrounding text byte-for-byte.
//
// Pure string functions, no file I/O.

// BeginSentinel returns the "<!-- <name>:BEGIN -->" sentinel for a block name.
func BeginSentinel(name string) string {
	return "<!-- " + name + ":BEGIN -->"
}

// EndSentinel returns the "<!-- <name>:END -->" sentinel for a block name.
func EndSentinel(name string) string {
	return "<!-- " + name + ":END -->"
}

// WrapBlock wraps content in the name sentinel pair (BEGIN\n<content>\nEND).
func WrapBlock(name, content string) string {
	return BeginSentinel(name) + "\n" + content + "\n" + EndSentinel(name)
}

// ExtractBlock returns the text between the name sentinels (excluding the
// sentinels themselves), with leading/trailing newlines stripped. Returns
// ("", false) if either sentinel is absent or END precedes BEGIN.
func ExtractBlock(text, name string) (string, bool) {
	begin := BeginSentinel(name)
	end := EndSentinel(name)
	beginPos := strings.Index(text, begin)
	endPos := strings.Index(text, end)
	if beginPos == -1 || endPos == -1 || endPos <= beginPos {
		return "", false
	}
	inner := text[beginPos+len(begin) : endPos]
	return strings.Trim(inner, "\n"), true
}

// ReplaceBlock splices content between the name sentinels in text,
// preserving everything before BEGIN and after END byte-for-byte. Returns
// an error if either sentinel is absent.
func ReplaceBlock(text, name, content string) (string, error) {
	begin := BeginSentinel(name)
	end := EndSentinel(name)
	beginPos := strings.Index(text, begin)
	endPos := strings.Index(text, end)
	if beginPos == -1 || endPos == -1 {
		return "", &sentinelError{name: name, begin: begin, end: end}
	}
	before := text[:beginPos+len(begin)]
	after := text[endPos:]
	return before + "\n" + content + "\n" + after, nil
}

// InsertBlockAfter inserts a new name block immediately after the
// afterName END sentinel. Returns an error if the afterName END sentinel
// is absent.
func InsertBlockAfter(text, afterName, name, content string) (string, error) {
	end := EndSentinel(afterName)
	pos := strings.Index(text, end)
	if pos == -1 {
		return "", &anchorError{afterName: afterName, name: name, end: end}
	}
	insertAt := pos + len(end)
	block := "\n\n" + WrapBlock(name, content)
	return text[:insertAt] + block + text[insertAt:], nil
}

type sentinelError struct {
	name, begin, end string
}

func (e *sentinelError) Error() string {
	return "CLAUDE.md block '" + e.name + "' sentinels not found ('" + e.begin + "' / '" + e.end + "'). Manual corruption suspected."
}

type anchorError struct {
	afterName, name, end string
}

func (e *anchorError) Error() string {
	return "Anchor block '" + e.afterName + "' END sentinel not found ('" + e.end + "'). Cannot insert '" + e.name + "' block."
}

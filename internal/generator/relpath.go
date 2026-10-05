package generator

import (
	"path/filepath"
	"strings"
)

// rewriteRepoAbsPaths rewrites absolute filesystem paths located under root
// (the repo root a renderer already received) into root-relative paths with
// forward slashes, so violation/signal text never leaks a private machine
// path into generated, committed documents (docs/gen/*.md, the crystals'
// LIVE-STATE/DOMAIN-MAP blocks) and the same repo renders byte-identical
// output on any machine. Paths NOT under root are left untouched.
//
// Matching is exact-prefix on a path boundary: the candidate occurrence must
// start at a non-path character (or the start of text) and be followed by a
// separator, so a root of C:\proj never mangles C:\proj2\file. Both
// Windows (\\) and POSIX (/) separators are handled, in the root and in the
// text, independently.
func rewriteRepoAbsPaths(root, text string) string {
	if root == "" || text == "" || !strings.Contains(text, root) &&
		!strings.Contains(text, filepath.ToSlash(root)) &&
		!strings.Contains(text, filepath.FromSlash(root)) {
		return text
	}
	cleaned := filepath.Clean(root)
	// The same root spelled with either separator; deduped (on POSIX the
	// three variants are usually one string).
	seen := map[string]bool{}
	for _, variant := range []string{cleaned, filepath.ToSlash(cleaned), filepath.FromSlash(cleaned)} {
		if !seen[variant] {
			seen[variant] = true
			text = replaceRootPrefixedPaths(text, variant)
		}
	}
	return text
}

// isPathTokenChar reports whether c can appear inside a filesystem path
// token as embedded in prose (anything but the delimiters that end it:
// whitespace, backtick, quote, parentheses, em dash, comma...).
func isPathTokenChar(c byte) bool {
	switch {
	case c == '/' || c == '\\' || c == '.' || c == '_' || c == '-':
		return true
	case c >= '0' && c <= '9':
		return true
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	}
	return false
}

// replaceRootPrefixedPaths rewrites every path-boundary occurrence of root
// (one exact spelling) plus its remainder into a root-relative, forward-
// slashed path, scanning left to right so rewritten output is never
// rescanned.
func replaceRootPrefixedPaths(text, root string) string {
	var b strings.Builder
	for {
		i := strings.Index(text, root)
		if i < 0 {
			b.WriteString(text)
			return b.String()
		}
		// Left boundary: the match must not continue a longer path token
		// (e.g. ...xC:\proj\... or a sibling like C:\proj-2 whose
		// prefix happens to equal root).
		if i > 0 && isPathTokenChar(text[i-1]) {
			b.WriteString(text[:i+1])
			text = text[i+1:]
			continue
		}
		rest := text[i+len(root):]
		// Right boundary: only rewrite when a separator follows, i.e. a
		// path UNDER the root; the bare root itself (or a directory whose
		// name merely extends root, like C:\proj2) is left as-is.
		if len(rest) == 0 || (rest[0] != '/' && rest[0] != '\\') {
			b.WriteString(text[:i+len(root)])
			text = rest
			continue
		}
		j := 1
		for j < len(rest) && isPathTokenChar(rest[j]) {
			j++
		}
		b.WriteString(text[:i])
		b.WriteString(filepath.ToSlash(rest[1:j]))
		text = rest[j:]
	}
}

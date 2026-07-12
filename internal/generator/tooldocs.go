package generator

import (
	"strings"
	"sync"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

// BuildToolDocs renders one Markdown doc per tool. Each tool's content is a
// pure function of that tool's own fields (read only, no shared mutable
// state), so the renders run concurrently — same indexed-slice-then-merge
// shape as invariants.AllViolations — while the final map assembly stays
// single-threaded (concurrent map writes are not safe in Go even when keys
// are disjoint).
func BuildToolDocs() map[string]string {
	tools := methodology.Tools.All()
	keys := make([]string, len(tools))
	contents := make([]string, len(tools))
	var wg sync.WaitGroup
	for i, t := range tools {
		wg.Add(1)
		go func(idx int, tool methodology.Tool) {
			defer wg.Done()
			lines := []string{
				Banner,
				"",
				"# " + tool.Command + " " + statusBadge(tool.Status),
				"",
				"## Status",
				"",
				statusLine(tool.Status),
				"",
				"## Canon",
				"",
				tool.Canon,
				"",
				"## Purpose",
				"",
				tool.Purpose,
			}
			keys[idx] = tool.Command
			contents[idx] = strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
		}(i, t)
	}
	wg.Wait()
	out := make(map[string]string, len(tools))
	for i, k := range keys {
		out[k] = contents[i]
	}
	return out
}

// statusBadge renders the short inline marker appended to a tool doc's H1,
// so a reader scanning docs/gen/tools/*.md (or the file listing) sees
// working-vs-aspirational at a glance without opening the file — the same
// distinction methodology.Status exists to make (see internal/methodology/
// tool.go's doc comment: "registry stores only rules and commands that
// actually work, not intentions").
func statusBadge(s methodology.Status) string {
	switch s {
	case methodology.Ported:
		return "[PORTED]"
	case methodology.Declared:
		return "[DECLARED — not ported]"
	default:
		return "[" + string(s) + "]"
	}
}

// statusLine renders the one-line "## Status" section body: a longer,
// unambiguous prose form of statusBadge's short marker, for the reader who
// opens the doc after the badge caught their eye.
func statusLine(s methodology.Status) string {
	switch s {
	case methodology.Ported:
		return "Ported — this is a real `hotam` CLI subcommand; running it does something."
	case methodology.Declared:
		return "Declared — methodology surface only; no Go command exists for it yet. The name below is historical (pre-port Python methodology); invoking it as `hotam <name>` will fail with \"unknown command\"."
	default:
		return string(s)
	}
}

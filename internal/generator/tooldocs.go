package generator

import (
	"strings"
	"sync"

	"github.com/PHPCraftdream/HotamSpecGo/internal/methodology"
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
				"# " + tool.Command,
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

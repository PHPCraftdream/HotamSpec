package generator

import (
	"regexp"
	"strings"
	"sync"

	"github.com/PHPCraftdream/HotamSpecGo/internal/methodology"
)

var topicSlugNonAlnum = regexp.MustCompile("[^a-z0-9]+")

func topicSlug(slug string) string {
	t := strings.TrimPrefix(slug, "§")
	t = strings.ToLower(t)
	t = topicSlugNonAlnum.ReplaceAllString(t, "-")
	return strings.Trim(t, "-")
}

// BuildThinkingDocs renders one Markdown doc per methodology section. Each
// section's content is a pure function of that section's own fields (read
// only, no shared mutable state), so the renders run concurrently — same
// indexed-slice-then-merge shape as invariants.AllViolations — while the
// final map assembly stays single-threaded (concurrent map writes are not
// safe in Go even when keys are disjoint).
func BuildThinkingDocs() map[string]string {
	sections := methodology.Sections.All()
	keys := make([]string, len(sections))
	contents := make([]string, len(sections))
	var wg sync.WaitGroup
	for i, s := range sections {
		wg.Add(1)
		go func(idx int, sec methodology.Section) {
			defer wg.Done()
			lines := []string{
				Banner,
				"",
				"# " + sec.Slug,
				"",
				"## Canon",
				"",
				sec.Canon,
				"",
				"## Narrative",
				"",
				sec.Narrative,
				"",
				"## Why",
				"",
				sec.Why,
			}
			keys[idx] = topicSlug(sec.Slug)
			contents[idx] = strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
		}(i, s)
	}
	wg.Wait()
	out := make(map[string]string, len(sections))
	for i, k := range keys {
		out[k] = contents[i]
	}
	return out
}

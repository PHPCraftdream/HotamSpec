package generator

import (
	"regexp"
	"strings"
	"sync"

	"github.com/PHPCraftdream/HotamSpec/internal/localization"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

var topicSlugNonAlnum = regexp.MustCompile("[^a-z0-9]+")

func topicSlug(slug string) string {
	t := strings.TrimPrefix(slug, "§")
	t = strings.ToLower(t)
	t = topicSlugNonAlnum.ReplaceAllString(t, "-")
	return strings.Trim(t, "-")
}

// BuildThinkingDocs renders the legacy English/source-language methodology
// pages. Each section is independent and may be rendered concurrently.
func BuildThinkingDocs() map[string]string {
	docs, _ := BuildThinkingDocsLocalized("")
	return docs
}

// BuildThinkingDocsLocalized renders methodology registry pages in one
// explicit locale. Section slugs and authored Canon/Narrative/Why contents
// remain unchanged; only fixed service labels are selected from the catalog.
func BuildThinkingDocsLocalized(language string) (map[string]string, error) {
	banner, err := localization.Lookup(language, Banner)
	if err != nil {
		return nil, err
	}
	canonHeading, err := localization.Lookup(language, "## Canon")
	if err != nil {
		return nil, err
	}
	narrativeHeading, err := localization.Lookup(language, "## Narrative")
	if err != nil {
		return nil, err
	}
	whyHeading, err := localization.Lookup(language, "## Why")
	if err != nil {
		return nil, err
	}
	sections := methodology.Sections.All()
	keys := make([]string, len(sections))
	contents := make([]string, len(sections))
	var wg sync.WaitGroup
	for i, s := range sections {
		wg.Add(1)
		go func(idx int, sec methodology.Section) {
			defer wg.Done()
			lines := []string{
				banner,
				"",
				"# " + sec.Slug,
				"",
				canonHeading,
				"",
				sec.Canon,
				"",
				narrativeHeading,
				"",
				sec.Narrative,
				"",
				whyHeading,
				"",
				sec.Why,
			}
			keys[idx] = topicSlug(sec.Slug)
			contents[idx] = strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
		}(i, s)
	}
	wg.Wait()
	out := make(map[string]string, len(sections))
	for i, key := range keys {
		out[key] = contents[i]
	}
	return out, nil
}

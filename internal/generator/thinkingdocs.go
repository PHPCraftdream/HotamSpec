package generator

import (
	"regexp"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/methodology"
)

var topicSlugNonAlnum = regexp.MustCompile("[^a-z0-9]+")

func topicSlug(slug string) string {
	t := strings.TrimPrefix(slug, "§")
	t = strings.ToLower(t)
	t = topicSlugNonAlnum.ReplaceAllString(t, "-")
	return strings.Trim(t, "-")
}

func BuildThinkingDocs() map[string]string {
	sections := methodology.Sections.All()
	out := make(map[string]string, len(sections))
	for _, s := range sections {
		lines := []string{
			Banner,
			"",
			"# " + s.Slug,
			"",
			"## Canon",
			"",
			s.Canon,
			"",
			"## Narrative",
			"",
			s.Narrative,
			"",
			"## Why",
			"",
			s.Why,
		}
		out[topicSlug(s.Slug)] = strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}
	return out
}

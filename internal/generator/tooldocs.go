package generator

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/methodology"
)

func BuildToolDocs() map[string]string {
	tools := methodology.Tools.All()
	out := make(map[string]string, len(tools))
	for _, t := range tools {
		lines := []string{
			Banner,
			"",
			"# " + t.Command,
			"",
			"## Canon",
			"",
			t.Canon,
			"",
			"## Purpose",
			"",
			t.Purpose,
		}
		out[t.Command] = strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}
	return out
}

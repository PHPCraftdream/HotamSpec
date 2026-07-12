package generator

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func BuildRepoMap(g *ontology.Graph) string {
	lines := []string{Banner, ReaderHeaderLine("REPO_MAP", g), ""}
	lines = append(lines, "# REPO-MAP.md — Repository file index (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines, repoMapContent)
	lines = append(lines, "")
	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

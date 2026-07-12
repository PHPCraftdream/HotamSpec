package generator

import (
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

// BuildRepoMap builds REPO-MAP.md (repository file index). domainName and
// genDocs identify which domain this doc describes and which docs/gen/ files
// were actually written for it in this run — scanning domains/<name>/*.json
// for "Domain content" and the actual generated files for "Generated docs"
// rather than hardcoding either section (R-doc-names-reader's sibling bug:
// REPO-MAP.md must name the real domain, not always hotam-spec-self).
//
// decisionsWritten/entitiesWritten additionally control the two conditional
// "_(not written: ...)_ " placeholder lines emitted when DECISIONS.md /
// ENTITIES.md were withheld because their source registry is empty.
func BuildRepoMap(g *ontology.Graph, domainName string, genDocs []GenDocEntry, decisionsWritten, entitiesWritten bool) string {
	lines := []string{Banner, ReaderHeaderLine("REPO_MAP", g), ""}
	lines = append(lines, "# REPO-MAP.md — Repository file index (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines, repoMapFrameworkAndToolsContent)
	lines = append(lines, "")

	lines = append(lines, "**Domain content** (`domains/"+domainName+"/`)")
	lines = append(lines, "")
	lines = append(lines, "- `domains/"+domainName+"/graph.json` — "+domainGraphPyRole(domainName))
	lines = append(lines, "- `domains/"+domainName+"/manifest.json` — manifest of domain '"+domainName+"'.")
	lines = append(lines, "")

	lines = append(lines, "**Generated docs** (`domains/"+domainName+"/docs/gen/`)")
	lines = append(lines, "")
	sortedDocs := make([]GenDocEntry, len(genDocs))
	copy(sortedDocs, genDocs)
	sort.Slice(sortedDocs, func(i, j int) bool { return sortedDocs[i].Filename < sortedDocs[j].Filename })
	for _, d := range sortedDocs {
		lines = append(lines, "- `domains/"+domainName+"/docs/gen/"+d.Filename+"` — "+mdTitle(d.Content))
	}
	if !decisionsWritten {
		lines = append(lines, "- `domains/"+domainName+"/docs/gen/DECISIONS.md` — _(not written: M-registry empty)_")
	}
	if !entitiesWritten {
		lines = append(lines, "- `domains/"+domainName+"/docs/gen/ENTITIES.md` — _(not written: no entity_types declared)_")
	}

	lines = append(lines, "")
	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

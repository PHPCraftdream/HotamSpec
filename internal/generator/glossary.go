package generator

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

var glossaryKindOrder = []string{"SECTION", "LIFECYCLE_STATE", "STATUS", "ROLE", "CONCEPT"}

var glossaryKindLabels = map[string]string{
	"SECTION":         "Sections (§-anchors)",
	"LIFECYCLE_STATE": "Lifecycle states",
	"STATUS":          "Statuses",
	"ROLE":            "Roles",
	"CONCEPT":         "Concepts",
}

func BuildGlossary(g *ontology.Graph) string {
	grouped := map[string][]glossaryTerm{}
	for _, k := range glossaryKindOrder {
		grouped[k] = nil
	}
	for _, term := range glossaryTerms {
		if _, ok := grouped[term.Kind]; ok {
			grouped[term.Kind] = append(grouped[term.Kind], term)
		}
	}

	lines := []string{Banner, ReaderHeaderLine("GLOSSARY", g), ""}
	lines = append(lines, "# GLOSSARY.md — Methodology controlled vocabulary (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines,
		"Generated mirror of the methodology's own canon terms — the framework's\n"+
			"controlled vocabulary that every docstring and generated doc must use\n"+
			"consistently. Terminology drift is invisibility (R-glossary-sync-test).")
	lines = append(lines, "")
	lines = append(lines,
		"Source: `internal/generator/glossary_terms_data.go`. Domain-side business terms\n"+
			"(R-ids, axis slugs, stakeholders) live in `domains/<name>/graph.json` and are\n"+
			"listed in REQUIREMENTS.md / TENSIONS.md — not duplicated here.")
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	for _, kind := range glossaryKindOrder {
		entries := grouped[kind]
		if len(entries) == 0 {
			continue
		}
		lines = append(lines, "## "+glossaryKindLabels[kind])
		lines = append(lines, "| slug | definition |")
		lines = append(lines, "|---|---|")
		for _, term := range entries {
			lines = append(lines, "| `"+Cell(term.Slug)+"` | "+Cell(term.Definition)+" |")
		}
		lines = append(lines, "")
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

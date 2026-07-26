package generator

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

var glossaryKindOrder = []string{"SECTION", "LIFECYCLE_STATE", "STATUS", "ROLE", "CONCEPT"}

var glossaryKindLabels = map[string]string{
	"SECTION":         "Sections (§-anchors)",
	"LIFECYCLE_STATE": "Lifecycle states",
	"STATUS":          "Statuses",
	"ROLE":            "Roles",
	"CONCEPT":         "Concepts",
}

func BuildGlossary(g *ontology.Graph, consumer bool) string {
	grouped := map[string][]glossaryTerm{}
	for _, k := range glossaryKindOrder {
		grouped[k] = nil
	}
	for _, term := range glossaryTerms {
		if _, ok := grouped[term.Kind]; ok {
			grouped[term.Kind] = append(grouped[term.Kind], term)
		}
	}

	// GLOSSARY.md lives at the PROJECT root (framework/GLOSSARY.md, task #357),
	// shared across all domains — byte-identical regardless of which domain's
	// gen-spec regenerates it. The header therefore carries the shared Banner
	// only, with NO per-domain `reader:` line (mirroring tools/*.md and
	// tools/INDEX.md, the other project-shared framework files): a reader line
	// resolves against the generating domain's stakeholder graph and would
	// differ between domains, breaking the idempotent-overwrite contract.
	// g remains a parameter (callers pass it) but is intentionally unused in
	// the body — the glossary body is a pure function of the methodology
	// registry (glossaryTerms), not the domain graph.
	lines := []string{Banner, ""}
	lines = append(lines, "# GLOSSARY.md — Methodology controlled vocabulary (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines,
		"Generated mirror of the methodology's own canon terms — the framework's\n"+
			"controlled vocabulary that every docstring and generated doc must use\n"+
			"consistently. Terminology drift is invisibility (R-glossary-sync-test).")
	lines = append(lines, "")
	// Under consumer, drop the "Source: `internal/generator/...`." clause — a
	// dead-end framework-source-file reference for an external consumer with no
	// internal/ tree. The "Domain-side business terms ..." prose that follows
	// stays; full-profile output is byte-identical (the Source clause is kept).
	glossarySourceLine := "Source: `internal/generator/glossary_terms_data.go`. Domain-side business terms\n" +
		"(R-ids, axis slugs, stakeholders) live in `domains/<name>/graph.json` and are\n" +
		"listed in REQUIREMENTS.md / TENSIONS.md — not duplicated here."
	if consumer {
		glossarySourceLine = "Domain-side business terms\n" +
			"(R-ids, axis slugs, stakeholders) live in `domains/<name>/graph.json` and are\n" +
			"listed in REQUIREMENTS.md / TENSIONS.md — not duplicated here."
	}
	lines = append(lines, glossarySourceLine)
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

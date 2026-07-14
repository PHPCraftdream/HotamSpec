package generator

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

// repoMapFrameworkBodyContent holds the domain-independent "Framework body"
// section of REPO-MAP.md — a hand-maintained description of internal/ontology/
// package purpose/architecture, which is the same regardless of which domain
// the doc is generated for. This is intentionally NOT registry-derived
// (unlike the Tools section below): it describes package PURPOSE, not a fact
// any registry tracks, so hand-maintenance is the correct home for it (see
// BuildFrameworkInvariants / frameworkinvariants.go for the sibling
// "static intro + derived list" pattern this file follows for its own
// Tools section).
var repoMapFrameworkBodyContent = strings.Join([]string{
	"### Repository Map",
	"",
	"**Framework body** (`internal/ontology/`)",
	"",
	"- `internal/ontology/assumption.go` — a claim with its OWN lifecycle (the root of context drift).",
	"- `internal/ontology/axis.go` — controlled vocabulary of tension dimensions.",
	"- `internal/ontology/conflict.go` — the first-class connector NODE, a held property of the discipline (not its headline; J1, commit b2c58c8).",
	"- `internal/ontology/entity.go` — domain-declared business concept with its own lifecycle.",
	"- `internal/ontology/graph.go` — the tension graph store and its traversal helpers.",
	"- `internal/ontology/graph_traversal.go` — traversal helpers over the tension graph.",
	"- `internal/ontology/lifecycle.go` — the generic state-machine value-type (framework keystone).",
	"- `internal/ontology/operator.go` — the acting facet of a Stakeholder (M20: NEW TYPE).",
	"- `internal/ontology/process.go` — opt-in behavioral aspect (M12).",
	"- `internal/ontology/requirement.go` — a business requirement as a node in the tension graph.",
	"- `internal/ontology/signoff.go` — the frozen provenance record of a human steward decision.",
	"- `internal/ontology/stakeholder.go` — who owns requirements and stewards conflicts.",
	"- `internal/invariants/` — structural form of the tension graph (the check_* layer).",
	"- `internal/diagnose/` — the operator's next-action diagnosis (what_now equivalent).",
	"- `internal/proposal/` — structured operator-→-steward change proposals + the mechanical apply writer.",
	"- `internal/loader/` — reads a domain's graph.json into an in-memory Graph.",
	"- `internal/generator/` — regenerates docs/gen/ from the graph (the gen-spec engine).",
	"- `internal/gate/` — T1 tiered LAND gate: select a targeted test subset instead of the full suite.",
	"- `internal/paths/` — project-root resolution for the consumer's data directory.",
}, "\n")

// renderRepoMapToolsSection renders the "Tools" section of REPO-MAP.md as a
// projection of the methodology.Tools registry — one line per Implemented
// tool (command + one-sentence Claim), followed by a comma-separated roster
// of Planned (not-yet-implemented) command names.
//
// This REPLACES the former hand-maintained tools list, which had already
// drifted twice (found 2026-07-13: 5 of the then-10 Implemented commands
// were missing; corrected by hand that pass, but the underlying hand-
// maintenance was flagged as the real defect and left for this follow-up).
// Deriving from methodology.Tools.All() — the same registry
// RenderEmbeddedToolsBlock (claudemd.go) projects from — closes this drift
// class permanently: a new Implemented tool automatically appears here on
// the next `hotam gen-spec`, with no second hand-edit required.
//
// Iteration order is registry declaration order (internal/registry.All()),
// the same deterministic order every other registry-derived renderer in this
// package relies on (e.g. RenderEmbeddedToolsBlock, ScanToolRequirements).
func renderRepoMapToolsSection() string {
	lines := []string{
		"**Tools** (`cmd/hotam/`, dispatched by `hotam <command>`)",
		"",
	}

	var planned []string
	for _, t := range methodology.Tools.All() {
		display := strings.ReplaceAll(t.Command, "_", "-")
		if t.Status == methodology.Implemented {
			lines = append(lines, "- `hotam "+display+"` — "+t.Claim)
		} else {
			planned = append(planned, t.Command)
		}
	}

	if len(planned) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Registered in the methodology but not yet implemented as `hotam` subcommands: "+strings.Join(planned, ", ")+".")
	}

	return strings.Join(lines, "\n")
}

// domainGraphPyRole returns the one-line role text for domains/<name>/graph.json
// in the "Domain content" section. Domains carry graph.json (no per-domain
// source file to introspect), so the known domains' role text is captured
// here directly; any other domain falls back to generic phrasing.
func domainGraphPyRole(domainName string) string {
	switch domainName {
	case "hotam-spec-self":
		return "Hotam-Spec modeling itself — the meta-domain (the framework's own design)."
	default:
		return "content graph of domain '" + domainName + "'."
	}
}

// GenDocEntry describes one file written into a domain's docs/gen/ during
// this run, for the REPO-MAP.md "Generated docs" section.
type GenDocEntry struct {
	Filename string
	Content  string
}

// mdTitle extracts the first H1/H2 heading from generated Markdown content as
// a short title: take the first line starting with '#', drop everything up
// to (and including) the last " — ", then strip a trailing " (...)" suffix.
func mdTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		stripped := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if strings.HasPrefix(line, "#") && stripped != "" {
			parts := strings.Split(stripped, " — ")
			title := parts[len(parts)-1]
			if idx := strings.Index(title, " ("); idx >= 0 {
				title = title[:idx]
			}
			return strings.TrimSpace(title)
		}
	}
	return ""
}

package generator

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

type moduleEntry struct {
	Mod   string
	Label string
}

var ModuleOrder = []moduleEntry{
	{Mod: "__init__", Label: "Methodology overview + the closed loop"},
	{Mod: "stakeholder", Label: "§Stakeholder — owners and resolvers"},
	{Mod: "axis", Label: "§Axis — controlled vocabulary of tension dimensions"},
	{Mod: "assumption", Label: "§Assumption — beliefs with a lifecycle"},
	{Mod: "requirement", Label: "§Requirement — the requirement node"},
	{Mod: "conflict", Label: "§Conflict — the connector node"},
	{Mod: "graph", Label: "§Graph — the store, the loader, and traversal"},
	{Mod: "lifecycle", Label: "§Lifecycle — the generic state-machine keystone"},
	{Mod: "operator", Label: "§Operator — the acting facet of a Stakeholder"},
	{Mod: "process", Label: "§Process / §Goal — behavioral aspect (M12) and Goal type (M19)"},
	{Mod: "invariants", Label: "§Invariants — structural form"},
}

// moduleSectionSlug maps a legacy module key (ModuleOrder's Mod) to the
// methodology.Sections slug that now carries its content. "__init__" is
// deliberately absent: its prose (the framework overview + closed loop) has
// no Section counterpart and lives in methodology.Overview instead — see
// internal/methodology/overview_data.go.
var moduleSectionSlug = map[string]string{
	"stakeholder": "§Stakeholder",
	"axis":        "§Axis",
	"assumption":  "§Assumption",
	"requirement": "§Requirement",
	"conflict":    "§Conflict",
	"graph":       "§Graph",
	"lifecycle":   "§Lifecycle",
	"operator":    "§Operator",
	"process":     "§Process",
	"invariants":  "§Invariants",
}

// ModuleDocstring renders the module-doc text for a legacy module key.
// internal/methodology is the single source of truth for this content:
// "__init__" reads methodology.Overview (unique framework-overview prose);
// every other key reads the matching methodology.Sections entry and renders
// it as "Canon: <slug> — <Canon>\n\n<Narrative>\n\nWHY <Why>", the same
// Canon/Narrative/Why shape internal/generator.BuildThinkingDocs uses to
// render sections elsewhere. This keeps module docs and Sections rendered
// from one shared registry so they cannot drift apart.
func ModuleDocstring(mod string) string {
	if mod == "__init__" {
		return strings.TrimRight(methodology.Overview, " \t\r\n")
	}
	slug, ok := moduleSectionSlug[mod]
	if !ok {
		return ""
	}
	sec, ok := methodology.Sections.Get(slug)
	if !ok {
		return ""
	}
	why := sec.Why
	if !strings.HasPrefix(why, "WHY ") {
		why = "WHY " + why
	}
	doc := "Canon: " + sec.Slug + " — " + sec.Canon + "\n\n" + sec.Narrative + "\n\n" + why
	return strings.TrimRight(doc, " \t\r\n")
}

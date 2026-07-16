package generator

import (
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func selectSettled(g *ontology.Graph, predicate func(ontology.Requirement) bool) []ontology.Requirement {
	var out []ontology.Requirement
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED && predicate(r) {
			out = append(out, r)
		}
	}
	return out
}

func renderFreshnessLines(r ontology.Requirement) []string {
	var out []string
	if r.LastReviewedAt != "" {
		out = append(out, "**Last reviewed.** "+r.LastReviewedAt, "")
	}
	if r.ReviewAfter != "" {
		out = append(out, "**Review after.** "+r.ReviewAfter, "")
	}
	if len(r.Evidence) > 0 {
		out = append(out, "**Evidence.** "+strings.Join(r.Evidence, "; "), "")
	}
	if len(r.SourceRefs) > 0 {
		out = append(out, "**Sources.** "+strings.Join(r.SourceRefs, ", "), "")
	}
	if len(r.History) > 0 {
		out = append(out, "**Change history.**", "")
		for _, h := range r.History {
			who := ""
			if h.DecidedBy != "" {
				who = " · " + h.DecidedBy
			}
			out = append(out, "- "+h.At+who+" — "+h.Summary)
		}
		out = append(out, "")
	}
	return out
}

func renderAtoms(title, intro string, reqs []ontology.Requirement, reader string) string {
	header := []string{Banner}
	if reader != "" {
		header = append(header, reader)
	}
	header = append(header, "")

	lines := append(header, "# "+title, "", intro, "", "---", "")
	if len(reqs) == 0 {
		lines = append(lines, "_No atomic requirements in this topic yet._", "")
	} else {
		sorted := make([]ontology.Requirement, len(reqs))
		copy(sorted, reqs)
		sort.SliceStable(sorted, func(i, j int) bool {
			return sorted[i].ID < sorted[j].ID
		})
		for _, r := range sorted {
			lines = append(lines, "## `"+r.ID+"` ("+r.Enforcement+")", "", "**Claim.** "+r.Claim, "")
			if strings.TrimSpace(r.Why) != "" {
				lines = append(lines, "**Why.** "+r.Why, "")
			}
			if len(r.EnforcedBy) > 0 {
				parts := make([]string, len(r.EnforcedBy))
				for i, e := range r.EnforcedBy {
					parts[i] = "`" + e + "`"
				}
				lines = append(lines, "**Enforced by:** "+strings.Join(parts, ", "), "")
			}
			if len(r.ImplementedBy) > 0 {
				parts := make([]string, len(r.ImplementedBy))
				for i, e := range r.ImplementedBy {
					parts[i] = "`" + e + "`"
				}
				lines = append(lines, "**Implemented by:** "+strings.Join(parts, ", "), "")
			}
			if len(r.VerifiedBy) > 0 {
				parts := make([]string, len(r.VerifiedBy))
				for i, e := range r.VerifiedBy {
					parts[i] = "`" + e + "`"
				}
				lines = append(lines, "**Verified by:** "+strings.Join(parts, ", "), "")
			}
			lines = append(lines, renderFreshnessLines(r)...)
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

func hasPrefixAny(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func BuildAtomsOperator(g *ontology.Graph) string {
	sel := selectSettled(g, func(r ontology.Requirement) bool {
		return hasPrefixAny(r.ID, "R-operator-", "R-agent-", "R-boot-", "R-prefer-tool-")
	})
	return renderAtoms(
		"Operator atoms",
		"The atomic requirements that constitute the operator's role, identity, and discipline.",
		sel,
		ReaderHeaderLine("ATOMS_OPERATOR", g),
	)
}

func BuildAtomsSubstrate(g *ontology.Graph) string {
	sel := selectSettled(g, func(r ontology.Requirement) bool {
		return hasPrefixAny(r.ID, "R-claude-md-", "R-content-", "R-deterministic-", "R-drift-", "R-rejected-")
	})
	return renderAtoms(
		"Substrate atoms",
		"The atomic requirements that govern how the substrate (graph + generated docs) behaves.",
		sel,
		ReaderHeaderLine("ATOMS_SUBSTRATE", g),
	)
}

func BuildAtomsDiscipline(g *ontology.Graph) string {
	sel := selectSettled(g, func(r ontology.Requirement) bool {
		if hasPrefixAny(r.ID, "R-anchor-", "R-speak-", "R-crystallize-") {
			return true
		}
		return r.ID == "R-prefer-tool-over-hand" || r.ID == "R-shared-tools-in-spec-tools"
	})
	return renderAtoms(
		"Discipline atoms",
		"The atomic requirements that govern operator discipline — anchoring, crystallizing, tool-preference.",
		sel,
		ReaderHeaderLine("ATOMS_DISCIPLINE", g),
	)
}

func BuildAtomsCheck(g *ontology.Graph) string {
	sel := selectSettled(g, func(r ontology.Requirement) bool {
		return hasPrefixAny(r.ID, "R-check-", "R-requirement-", "R-bijection-", "R-enforcement-", "R-decided-")
	})
	return renderAtoms(
		"Check & enforcement atoms",
		"The atomic requirements about how rules are enforced — atomicity of claims, atomicity of checks, bijection.",
		sel,
		ReaderHeaderLine("ATOMS_CHECK", g),
	)
}

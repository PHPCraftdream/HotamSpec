package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

// Canon: §Requirement — R-constitution-is-index: the CLAUDE.md CONSTITUTION
// sentinel block is a compact index over SETTLED requirements (id +
// enforcement flag only), distinct from the full docs/gen/CONSTITUTION.md
// catalog (BuildConstitution, constitution.go). Ported from
// spec/tools/gen_spec.py's build_constitution_index_model /
// _render_constitution_block / _cluster_index_items (Python reference,
// backup/python-legacy-2026-07-12).
//
// frameworkPlumbingIDs, digestCategories, enforcementFlag,
// isFrameworkPlumbing, categorizeRequirement, and constitutionIndexLine
// already exist in frameworkinvariants.go (the FRAMEWORK-INVARIANTS.md
// renderer shares the same plumbing partition and category table); this
// file adds only what CLAUDE.md's CONSTITUTION block needs beyond that:
// the D5 rule-cluster collapsing and the block's own index-model grouping
// (business + discipline atoms, i.e. NOT framework-plumbing — the inverse
// selection from BuildFrameworkInvariants).

// ruleClusterPrefixes mirrors _RULE_CLUSTER_PREFIXES: (representative id,
// id-prefix) pairs. Every SETTLED requirement whose id starts with prefix
// collapses into one index token headed by representative. First matching
// prefix wins (table is small and non-overlapping by construction).
var ruleClusterPrefixes = []struct {
	Representative string
	Prefix         string
}{
	{"R-land-gate-tier-selector", "R-land-gate-"},
	{"R-land-gate-tier-selector", "R-land-tier-"},
	{"R-land-gate-tier-selector", "R-tiered-gate-"},
	{"R-land-gate-tier-selector", "R-commit-boundary-checkable"},
	{"R-attention-registry", "R-attention-"},
	{"R-tension-audit-shortlist-tool", "R-tension-audit-"},
	{"R-active-loop-protocol", "R-active-loop-"},
}

func clusterRepresentative(rid string) (string, bool) {
	for _, c := range ruleClusterPrefixes {
		if strings.HasPrefix(rid, c.Prefix) {
			return c.Representative, true
		}
	}
	return "", false
}

func flagFor(enforcement string) string {
	if f, ok := enforcementFlag[enforcement]; ok {
		return f
	}
	return "?"
}

// clusterIndexItems groups a (pre-sorted-by-id) requirement list into
// index tokens: requirements sharing a cluster representative collapse
// into one "<rep> [<flag>] (+N related: <id>[<flag>], ...)" token; every
// other requirement renders as a plain "<id> [<flag>]" token (via the
// shared constitutionIndexLine). Ported from _cluster_index_items.
func clusterIndexItems(reqs []ontology.Requirement) []string {
	byRepresentative := make(map[string][]ontology.Requirement)
	for _, r := range reqs {
		if rep, ok := clusterRepresentative(r.ID); ok {
			byRepresentative[rep] = append(byRepresentative[rep], r)
		}
	}

	emittedReps := make(map[string]struct{})
	var items []string
	for _, r := range reqs {
		rep, ok := clusterRepresentative(r.ID)
		if !ok {
			items = append(items, constitutionIndexLine(r.ID, r.Claim, r.Enforcement))
			continue
		}
		if _, done := emittedReps[rep]; done {
			continue
		}
		members := byRepresentative[rep]
		if len(members) == 1 {
			// Solo membership (cluster prefix matched but no siblings present
			// in this requirement set) — no value in a "(+0 related)" token.
			items = append(items, constitutionIndexLine(r.ID, r.Claim, r.Enforcement))
			emittedReps[rep] = struct{}{}
			continue
		}
		head := members[0]
		for _, m := range members {
			if m.ID == rep {
				head = m
				break
			}
		}
		var rest []ontology.Requirement
		for _, m := range members {
			if m.ID != head.ID {
				rest = append(rest, m)
			}
		}
		restParts := make([]string, len(rest))
		for i, m := range rest {
			restParts[i] = fmt.Sprintf("%s[%s]", m.ID, flagFor(m.Enforcement))
		}
		items = append(items, fmt.Sprintf("%s [%s] (+%d related: %s)", head.ID, flagFor(head.Enforcement), len(rest), strings.Join(restParts, ", ")))
		emittedReps[rep] = struct{}{}
	}
	return items
}

type constitutionCategory struct {
	Label        string
	Requirements []ontology.Requirement
}

// buildConstitutionIndexModel mirrors build_constitution_index_model: pure
// graph -> ordered category list, business + discipline SETTLED
// requirements only (framework-plumbing ids excluded — the inverse
// selection from BuildFrameworkInvariants's settled filter). Same category
// order as digestCategories, then "Other"; each category's requirements
// sorted by id.
func buildConstitutionIndexModel(g *ontology.Graph) []constitutionCategory {
	var settled []ontology.Requirement
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED && !isFrameworkPlumbing(r.ID) {
			settled = append(settled, r)
		}
	}
	if len(settled) == 0 {
		return nil
	}

	groups := make(map[string][]ontology.Requirement)
	for _, r := range settled {
		cat := categorizeRequirement(r.ID)
		groups[cat] = append(groups[cat], r)
	}
	for cat := range groups {
		sort.Slice(groups[cat], func(i, j int) bool { return groups[cat][i].ID < groups[cat][j].ID })
	}

	var order []string
	for _, dc := range digestCategories {
		if _, ok := groups[dc.Label]; ok {
			order = append(order, dc.Label)
		}
	}
	if _, ok := groups["Other"]; ok {
		order = append(order, "Other")
	}

	out := make([]constitutionCategory, 0, len(order))
	for _, label := range order {
		out = append(out, constitutionCategory{Label: label, Requirements: groups[label]})
	}
	return out
}

// BuildConstitutionBlock renders the CONSTITUTION index block content
// (without sentinels) for the CLAUDE.md CONSTITUTION sentinel: a compact
// per-category summary (requirement count only, no id listing), with
// framework-plumbing atoms relocated to a pointer at
// docs/gen/FRAMEWORK-INVARIANTS.md and the full id+flag index relocated to
// docs/gen/AGENT-CONTEXT.md / REQUIREMENTS.md (P2-1 compaction — the
// hundreds-of-ids full listing lives only in the generated docs, not
// duplicated into every regeneration of the root crystal). Distinct from
// BuildConstitution (constitution.go), which renders the full
// docs/gen/CONSTITUTION.md catalog, and from the full per-category listing
// that AGENT-CONTEXT.md still carries in full (BuildAgentContext,
// agentcontext.go) — this root-crystal block is intentionally the
// summarized one.
func BuildConstitutionBlock(g *ontology.Graph, domainName string) string {
	if domainName == "" {
		domainName = "hotam-spec-self"
	}

	var allSettled []ontology.Requirement
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED {
			allSettled = append(allSettled, r)
		}
	}
	nPlumbing := 0
	for _, r := range allSettled {
		if isFrameworkPlumbing(r.ID) {
			nPlumbing++
		}
	}

	categories := buildConstitutionIndexModel(g)
	if len(categories) == 0 {
		return generatedHeaderComment + "\n\n_No SETTLED requirements yet._"
	}

	agentContextPath := fmt.Sprintf("domains/%s/docs/gen/AGENT-CONTEXT.md", domainName)
	rosterPath := fmt.Sprintf("domains/%s/docs/gen/REQUIREMENTS.md", domainName)
	invariantsPath := fmt.Sprintf("domains/%s/docs/gen/FRAMEWORK-INVARIANTS.md", domainName)

	lines := []string{
		generatedHeaderComment,
		"",
		"### Constitution index (business + discipline SETTLED requirements — summary)",
		"",
		fmt.Sprintf("> Full id+flag index: `%s`. Full claim + WHY + assumptions: `%s` (roster) ·", agentContextPath, rosterPath),
		fmt.Sprintf("> one requirement: `hotam req show <id> --domain domains/%s`. enforcement detail: `docs/gen/UNENFORCED.md`.", domainName),
		"> Flags: [E] ENFORCED · [S] STRUCTURAL · [P] PROSE.",
		fmt.Sprintf("> Framework internals (%d atoms): `%s`.", nPlumbing, invariantsPath),
		"",
	}
	var catParts []string
	for _, category := range categories {
		catParts = append(catParts, fmt.Sprintf("%s (%d)", category.Label, len(category.Requirements)))
	}
	lines = append(lines, strings.Join(catParts, " · "))

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n")
}

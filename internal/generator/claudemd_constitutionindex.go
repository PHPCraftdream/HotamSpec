package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// Canon: §Requirement — R-constitution-is-index: the CLAUDE.md CONSTITUTION
// sentinel block is a compact index over SETTLED requirements (id +
// enforcement flag only), distinct from the full docs/gen/CONSTITUTION.md
// catalog (BuildConstitution, constitution.go).
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
// shared constitutionIndexLine).
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

// consumerCategoryOrder is the CONSUMER-profile categorization for the
// Constitution index: a business-domain requirement id carries no framework
// semantics in its prefix (a consumer domain's ids are typically
// spreadsheet-derived, e.g. R-FR-01..R-FR-32 — sequential, not topic-coded),
// so id-prefix bucketing (digestCategories, framework-specific: "R-operator-",
// "R-lifecycle-", ...) degenerates to dumping everything into "Other" (see
// task #337 / external review R4 §4.6). The one dimension every Requirement
// carries structurally, regardless of domain or id shape, is Enforcement —
// exactly the same ENFORCED/STRUCTURAL/PROSE tri-state already shown to the
// reader via the [E]/[S]/[P] flags on each index token. Grouping by that
// tier answers a real orienting question for a business domain ("which
// requirements are actually enforced today vs. still a prose commitment?")
// without inventing a business taxonomy this engine has no authority to
// assert (no cross-domain field like "axis" or "stakeholder" is populated
// per-requirement — Owner is domain-wide here, Axis lives on Conflict, not
// Requirement). Order: most-proven first (Enforced), then Structural, then
// Prose, mirroring the flags legend's own E/S/P order.
var consumerCategoryOrder = []struct {
	Label       string
	Enforcement string
}{
	{"Enforced", ontology.EnforcementENFORCED},
	{"Structural", ontology.EnforcementSTRUCTURAL},
	{"Prose", ontology.EnforcementPROSE},
}

// categorizeRequirementByEnforcement is consumerCategoryOrder's lookup: any
// Enforcement value outside the three known constants (should not occur for
// a SETTLED requirement, but the ontology field is a plain string) falls to
// "Other" — the same honest-fallback discipline categorizeRequirement
// already uses for unmatched id prefixes.
func categorizeRequirementByEnforcement(r ontology.Requirement) string {
	for _, c := range consumerCategoryOrder {
		if r.Enforcement == c.Enforcement {
			return c.Label
		}
	}
	return "Other"
}

// buildConstitutionIndexModel mirrors build_constitution_index_model: pure
// graph -> ordered category list, business + discipline SETTLED
// requirements only (framework-plumbing ids excluded — the inverse
// selection from BuildFrameworkInvariants's settled filter). Each category's
// requirements sorted by id.
//
// consumer selects the categorization scheme: the FULL profile (hotam's own
// self-hosting domains) keeps id-prefix bucketing (digestCategories, then
// "Other") — its ids are framework-semantic by construction
// (R-operator-..., R-lifecycle-..., ...), so the categories are meaningful.
// The CONSUMER profile (external business domains) uses
// categorizeRequirementByEnforcement instead — see consumerCategoryOrder's
// doc comment for why.
func buildConstitutionIndexModel(g *ontology.Graph, consumer bool) []constitutionCategory {
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
	var order []string
	if consumer {
		for _, r := range settled {
			cat := categorizeRequirementByEnforcement(r)
			groups[cat] = append(groups[cat], r)
		}
		for _, c := range consumerCategoryOrder {
			if _, ok := groups[c.Label]; ok {
				order = append(order, c.Label)
			}
		}
		if _, ok := groups["Other"]; ok {
			order = append(order, "Other")
		}
	} else {
		for _, r := range settled {
			cat := categorizeRequirement(r.ID)
			groups[cat] = append(groups[cat], r)
		}
		for _, dc := range digestCategories {
			if _, ok := groups[dc.Label]; ok {
				order = append(order, dc.Label)
			}
		}
		if _, ok := groups["Other"]; ok {
			order = append(order, "Other")
		}
	}
	for cat := range groups {
		sort.Slice(groups[cat], func(i, j int) bool { return groups[cat][i].ID < groups[cat][j].ID })
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
func BuildConstitutionBlock(g *ontology.Graph, domainName string, consumer bool) string {
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

	categories := buildConstitutionIndexModel(g, consumer)
	if len(categories) == 0 {
		return generatedHeaderComment + "\n\n_No SETTLED requirements yet._"
	}

	agentContextPath := fmt.Sprintf("domains/%s/docs/gen/AGENT-CONTEXT.md", domainName)
	rosterPath := fmt.Sprintf("domains/%s/docs/gen/REQUIREMENTS.md", domainName)
	invariantsPath := fmt.Sprintf("domains/%s/docs/gen/FRAMEWORK-INVARIANTS.md", domainName)
	unenforcedPath := fmt.Sprintf("domains/%s/docs/gen/UNENFORCED.md", domainName)

	lines := []string{
		generatedHeaderComment,
		"",
		"### Constitution index (business + discipline SETTLED requirements — summary)",
		"",
		fmt.Sprintf("> Full id+flag index: `%s`. Full claim + WHY + assumptions: `%s` (roster) ·", agentContextPath, rosterPath),
		fmt.Sprintf("> one requirement: `hotam req show <id> --domain domains/%s`. enforcement detail: `%s`.", domainName, unenforcedPath),
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

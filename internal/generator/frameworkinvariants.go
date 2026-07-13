package generator

import (
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

var frameworkPlumbingIDs = set(
	"R-entity-type-lifecycle-wellformed",
	"R-entity-instance-state-in-lifecycle",
	"R-entity-instance-required-fields",
	"R-entity-instance-id-prefix",
	"R-entity-instance-refs-resolve",
	"R-entity-field-kind-known",
	"R-entity-typed-anchors",
	"R-process-drives-existing-entities",
	"R-step-invokes-known-transition",
	"R-entity-derived-requirement",
	"R-entity-is-declarative",
	"R-entity-reuses-lifecycle",
	"R-entity-checks-by-iteration",
	"R-entity-state-conflict-surfaced",
	"R-entities-md-generated",
	"R-agent-has-own-tools-dir",
	"R-agent-declares-purpose",
	"R-agent-map-generated",
	"R-agent-scoped-constitution",
	"R-agent-is-recursive-director",
	"R-agent-has-docs-dir",
	"R-agent-references-shared-docs",
	"R-subagent-gets-its-claude-md",
	"R-agent-is-a-directory",
	"R-sub-agent-crystal-triad",
	"R-domain-is-a-directory",
	"R-domain-has-manifest",
	"R-domain-declares-director",
	"R-domain-owns-graph-py",
	"R-domain-owns-docs-gen",
	"R-domain-owns-tools-and-agents",
	"R-domain-map-generated",
	"R-director-agent-required-per-domain",
	"R-domain-has-docs-dir",
	"R-content-layout-evolution",
	"R-process-types-exist",
	"R-process-opt-in",
	"R-process-lifecycle-wellformed-aspect",
	"R-process-roles-declared-aspect",
	"R-process-goal-owner-is-operator-aspect",
	"R-process-typed-anchors-extended",
	"R-goal-target-kind-known",
	"R-goal-owner-is-operator",
	"R-operator-is-frozen-dataclass",
	"R-operator-references-stakeholder",
	"R-operator-has-context-budget",
	"R-operator-may-have-parent",
	"R-context-budget-rule",
	"R-operator-not-self-approve",
	"R-operator-type-vs-facet",
	"R-lifecycle-type-exists",
	"R-lifecycle-validates-requirement",
	"R-lifecycle-validates-conflict",
	"R-lifecycle-validates-operator",
	"R-lifecycle-validates-goal",
	"R-statemachine-reachable",
	"R-statemachine-deterministic",
	"R-statemachine-terminal-or-cyclic",
	"R-statemachine-guard-on-assumption",
	"R-drift-structurally-impossible",
	"R-deterministic-generation",
	"R-glossary-generated",
	"R-glossary-sync-fails-dead",
	"R-glossary-sync-fails-unused",
	"R-glossary-drift-stable",
	"R-history-generated-from-rejected",
	"R-history-generated-from-decided",
	"R-docs-generated-from-requirements",
	"R-repo-map-generated",
	"R-claude-md-live-state-generated",
	"R-root-claude-md-is-sentinel-only",
	"R-claude-md-template-driven",
	"R-framework-shared-docs-generated",
	"R-shared-tool-doc-from-docstring-and-help",
	"R-shared-thinking-doc-from-canon-sections",
	"R-content-free-no-business-data",
	"R-content-free-no-examples",
	"R-content-free-no-seed-graph",
	"R-empty-content-wellformed",
	"R-empty-content-calm-banner",
	"R-empty-content-gen-notice",
	"R-bijection-r-to-enforcer",
	"R-enforcement-levels-declared",
	"R-enforced-names-enforcer",
	"R-requirement-enforced",
	"R-enforceability-kind-declared",
	"R-check-method-is-atomic",
	"R-audit-atomicity-tool",
	"R-method-matches-docstring",
	"R-m-tag-format-valid",
	"R-anchor-taxonomy",
	"R-recently-rejected-surfaced",
	"R-operator-prompt-loaded-at-session-start",
	"R-three-cipher-pulse-structurally-injected",
	"R-post-compact-regen-from-substrate",
	"R-claude-md-consolidates-when-single-agent",
	"R-operator-crystal-embeds-thinking-distilled",
	"R-operator-crystal-embeds-tools-distilled",
	"R-crystal-carries-role-seed",
	"R-crystal-carries-mediation-loop",
	"R-crystal-carries-recursion-seed",
	"R-constitution-is-index",
	"R-crystal-is-claude-md",
	"R-crystal-reload-by-reference",
	"R-crystal-tree-hierarchy",
	"R-project-name-hotam-spec",
	"R-parallel-mutating-agents-use-worktree",
	"R-dependency-tracked",
	"R-dependency-drives-parallel",
	"R-dependency-drives-sequential",
	"R-tools-registry-generated",
	"R-tool-is-its-own-requirement",
	"R-constitution-separates-plumbing",
)

var digestCategories = []struct {
	Label    string
	Prefixes []string
}{
	{"Operator", []string{"R-operator-", "R-crystal-", "R-context-", "R-budget-", "R-agent-"}},
	{"Substrate / Anchoring", []string{"R-anchor-", "R-speak-", "R-stale-", "R-claude-md-"}},
	{"Discipline", []string{"R-prefer-", "R-crystallize-", "R-delegation-", "R-task-", "R-active-loop-", "R-shared-tools-", "R-verify-", "R-working-"}},
	{"Check / Invariant", []string{"R-statemachine-", "R-bijection-", "R-conflict-", "R-decided-", "R-axis-", "R-m-tag-", "R-typed-", "R-requirement-", "R-enforcement-", "R-check-", "R-stable-", "R-steward-", "R-open-"}},
	{"Framework Self", []string{"R-drift-", "R-deterministic-", "R-content-", "R-empty-", "R-two-altitude-", "R-rejected-"}},
	{"Lifecycle / Process / Goal", []string{"R-lifecycle-", "R-process-", "R-goal-"}},
	{"Boot / Glossary / History / Docs", []string{"R-boot-", "R-glossary-", "R-history-", "R-docs-"}},
}

var enforcementFlag = map[string]string{
	"ENFORCED":   "E",
	"STRUCTURAL": "S",
	"PROSE":      "P",
}

func isFrameworkPlumbing(rid string) bool {
	_, ok := frameworkPlumbingIDs[rid]
	return ok
}

func categorizeRequirement(rid string) string {
	for _, cat := range digestCategories {
		for _, prefix := range cat.Prefixes {
			if strings.HasPrefix(rid, prefix) {
				return cat.Label
			}
		}
	}
	return "Other"
}

func constitutionIndexLine(rid, claim, enforcement string) string {
	flag := "?"
	if f, ok := enforcementFlag[enforcement]; ok {
		flag = f
	}
	return rid + " [" + flag + "]"
}

func BuildFrameworkInvariants(g *ontology.Graph, domainName string) string {
	var settled []ontology.Requirement
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED && isFrameworkPlumbing(r.ID) {
			settled = append(settled, r)
		}
	}
	if domainName == "" {
		domainName = "hotam-spec-self"
	}
	rosterPath := "domains/" + domainName + "/docs/gen/REQUIREMENTS.md"

	lines := []string{Banner, ReaderHeaderLine("FRAMEWORK_INVARIANTS", g), ""}
	lines = append(lines, "# FRAMEWORK-INVARIANTS.md — Framework-plumbing index (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines, "Hotam-Spec is the framework modeling ITSELF (hotam-spec-self domain), so most of its SETTLED requirements are internal guarantees of the framework's own machinery (Entity/Agent/Domain/Process/Operator-internals/Lifecycle-keystone/Generator/bijection/anchor mechanics/CLAUDE.md machinery), not business claims the operator mediates as reality. This index holds exactly those framework-internal atoms, relocated out of the root CLAUDE.md CONSTITUTION index (R-constitution-separates-plumbing, Phase 3, task #9).")
	lines = append(lines, "")
	lines = append(lines, "> Full claim + WHY + assumptions: `"+rosterPath+"` (roster) · enforcement detail: `domains/"+domainName+"/docs/gen/UNENFORCED.md`.")
	lines = append(lines, "> Flags: [E] ENFORCED · [S] STRUCTURAL · [P] PROSE.")
	lines = append(lines, "> No atom here changed status by this relocation — every id below is (and remains) SETTLED in the graph; only ITS RENDER LOCATION moved.")
	lines = append(lines, "")

	if len(settled) == 0 {
		lines = append(lines, "_No framework-plumbing SETTLED requirements yet._")
	} else {
		groups := map[string][]ontology.Requirement{}
		for _, r := range settled {
			cat := categorizeRequirement(r.ID)
			groups[cat] = append(groups[cat], r)
		}
		for cat := range groups {
			sorted := groups[cat]
			sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
			groups[cat] = sorted
		}
		catOrder := []string{}
		for _, dc := range digestCategories {
			if _, exists := groups[dc.Label]; exists {
				catOrder = append(catOrder, dc.Label)
			}
		}
		if _, exists := groups["Other"]; exists {
			catOrder = append(catOrder, "Other")
		}
		for _, cat := range catOrder {
			lines = append(lines, "**"+cat+"**")
			lines = append(lines, "")
			for _, r := range groups[cat] {
				lines = append(lines, constitutionIndexLine(r.ID, r.Claim, r.Enforcement))
			}
			lines = append(lines, "")
		}
	}

	toolReqs := ScanToolRequirements()
	if len(toolReqs) > 0 {
		lines = append(lines, "**Tool-derived requirements**")
		lines = append(lines, "")
		for _, tr := range toolReqs {
			lines = append(lines, constitutionIndexLine(tr.ID, tr.Claim, "STRUCTURAL"))
		}
		lines = append(lines, "")
	}

	entitySection := renderEntityDerivedConstitutionSection(g)
	if entitySection != "" {
		lines = append(lines, entitySection)
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

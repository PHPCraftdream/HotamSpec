package generator

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

var constitutionSet = map[string]struct{}{
	"R-agent-never-lost":                 {},
	"R-drift-structurally-impossible":    {},
	"R-deterministic-generation":         {},
	"R-conflict-is-connector-node":       {},
	"R-two-altitude-ontology":            {},
	"R-empty-content-is-legitimate":      {},
	"R-ai-presents-not-decides":          {},
	"R-resolver-distinct-from-owners":    {},
	"R-operator-not-self-approve":        {},
	"R-decided-needs-human-signoff":      {},
	"R-open-states-question":             {},
	"R-rejected-preserved-not-deleted":   {},
	"R-axis-controlled-vocab":            {},
	"R-stable-conflict-identity":         {},
	"R-operator-acting-facet":            {},
	"R-context-budget-rule":              {},
	"R-operator-crystal-is-claude-md":    {},
	"R-crystallize-knowledge-to-code":    {},
	"R-crystallize-before-split":         {},
	"R-working-vs-substrate-budget":      {},
	"R-enforcement-gradient":             {},
	"R-requirement-enforced":             {},
	"R-anchor-everything":                {},
	"R-speak-by-reference":               {},
	"R-active-loop-playbooks":            {},
	"R-verify-closure-per-action":        {},
	"R-uncrystallizable-is-missing-type": {},
	"R-stale-substrate":                  {},
	"R-critical-core-scope":              {},
}

var constitutionCategories = []struct {
	Label string
	IDs   map[string]struct{}
}{
	{"Closed loop & operator role", set("R-agent-never-lost", "R-drift-structurally-impossible", "R-deterministic-generation", "R-conflict-is-connector-node", "R-two-altitude-ontology", "R-empty-content-is-legitimate")},
	{"Hard boundary", set("R-ai-presents-not-decides", "R-resolver-distinct-from-owners", "R-operator-not-self-approve", "R-decided-needs-human-signoff", "R-open-states-question", "R-rejected-preserved-not-deleted", "R-axis-controlled-vocab", "R-stable-conflict-identity")},
	{"Self + delegation", set("R-operator-acting-facet", "R-context-budget-rule", "R-operator-crystal-is-claude-md")},
	{"Super-rules (crystallize + anchor)", set("R-crystallize-knowledge-to-code", "R-crystallize-before-split", "R-working-vs-substrate-budget", "R-enforcement-gradient", "R-requirement-enforced", "R-anchor-everything", "R-speak-by-reference")},
	{"Loop machinery", set("R-active-loop-playbooks", "R-verify-closure-per-action")},
	{"Conscience", set("R-uncrystallizable-is-missing-type", "R-stale-substrate", "R-critical-core-scope")},
}

var criticalCoreNames = []string{
	"check_resolver_not_a_member_owner",
	"check_operator_resolver_not_self",
	"check_decided_has_decided_by",
	"check_typed_anchors",
	"check_no_dangling_ids",
	"check_open_has_question",
}

func set(ids ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

// ConstitutionMDHasContent reports whether CONSTITUTION.md carries real
// domain content — i.e. whether the graph is non-empty (task #364: withheld
// entirely from the docs/gen/ write set when the domain has zero axes/
// stakeholders/requirements/conflicts/assumptions/operators/processes/goals/
// entity_types/entities, mirroring the conditional-write pattern
// DECISIONS.md/ENTITIES.md/TENSIONS.md/PIPELINE.md/MODELS.md already use). A
// genuinely empty domain has nothing to render but the EmptyNotice
// placeholder BuildConstitution already falls back to (g.IsEmpty()) — so a
// fresh, unmodeled domain now produces ZERO files under docs/gen/, not a
// directory full of calm-but-empty placeholders.
func ConstitutionMDHasContent(g *ontology.Graph) bool {
	return !g.IsEmpty()
}

func BuildConstitution(g *ontology.Graph, domainName string, consumer bool) string {
	if domainName == "" {
		domainName = "hotam-spec-self"
	}
	reqByID := make(map[string]ontology.Requirement, len(g.Requirements))
	for _, r := range g.Requirements {
		reqByID[r.ID] = r
	}

	lines := []string{localizedBanner(g), ReaderHeaderLine("CONSTITUTION", g), ""}
	lines = append(lines, serviceText(g, "# CONSTITUTION.md — The operator's boot sequence (Hotam-Spec)"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "You — the AI agent reading this cold — are the prospective Operator of this\nrepository. Read this file end-to-end before any action. It is generated from\nthe methodology's SETTLED laws (the active domain's `graph.json`). It is your\n*reconstitution from the substrate*: you do not need a session checkpoint\nto know what to do."))
	lines = append(lines, "")

	lines = append(lines, serviceText(g, "## 1. The role"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "You are operator #1 — the **acting facet** of a Stakeholder. (R-operator-acting-facet)"))
	opIDs := []string{"OP-director"}
	if len(g.Operators) > 0 {
		opIDs = []string{g.Operators[0].ID}
	}
	lines = append(lines, serviceText(g, "You appear in the graph as `%s`. (verify by reading `g.operators`)", opIDs[0]))
	lines = append(lines, serviceText(g, "You read CLAUDE.md as your crystal — the anchored map of the whole graph. (R-operator-crystal-is-claude-md)"))
	lines = append(lines, serviceText(g, "You speak by REFERENCE to anchors, not by re-carrying content. (R-speak-by-reference)"))
	lines = append(lines, "")

	lines = append(lines, serviceText(g, "## 2. The closed loop (the operating procedure)"))
	lines = append(lines, "")
	closedLoopDoc := ModuleDocstring("__init__")
	loopText := ""
	if strings.Contains(closedLoopDoc, "THE CLOSED LOOP") {
		start := strings.Index(closedLoopDoc, "THE CLOSED LOOP")
		rest := closedLoopDoc[start:]
		end := len(closedLoopDoc)
		if idx := strings.Index(rest, "\nTHE AI"); idx >= 0 {
			end = start + idx
		}
		loopText = strings.TrimSpace(closedLoopDoc[start:end])
	}
	if loopText != "" {
		lines = append(lines, loopText)
	} else {
		lines = append(lines, serviceText(g, "State (graph + generated docs + test status)\n  -> Diagnosis (`hotam what-now`)\n  -> Next-action (typed, prioritized)\n  -> Action (edit the graph)\n  -> regenerate (`hotam gen-spec`)\n  -> State."))
	}
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "Anchors: R-agent-never-lost, R-deterministic-generation, R-drift-structurally-impossible."))
	lines = append(lines, "")

	lines = append(lines, serviceText(g, "## 3. The hard boundary"))
	lines = append(lines, "")
	hardBoundaryIDs := []string{
		"R-ai-presents-not-decides",
		"R-resolver-distinct-from-owners",
		"R-operator-not-self-approve",
		"R-decided-needs-human-signoff",
		"R-open-states-question",
		"R-rejected-preserved-not-deleted",
		"R-axis-controlled-vocab",
		"R-stable-conflict-identity",
	}
	anyHardBoundary := false
	for _, rid := range hardBoundaryIDs {
		r, ok := reqByID[rid]
		if ok {
			lines = append(lines, "**"+rid+"** — "+requirementClaim(g, r))
			lines = append(lines, "")
			anyHardBoundary = true
		}
	}
	if !anyHardBoundary {
		lines = append(lines, serviceText(g, "_No hard-boundary requirements SETTLED in this domain's graph yet._"))
		lines = append(lines, "")
	}

	lines = append(lines, serviceText(g, "## 4. The two super-rules (context discipline)"))
	lines = append(lines, "")
	superRuleIDs := []struct {
		Label string
		ID    string
	}{
		{"CRYSTALLIZE", "R-crystallize-knowledge-to-code"},
		{"ANCHOR", "R-anchor-everything"},
		{"REFERENCE", "R-speak-by-reference"},
		{"ORDER", "R-crystallize-before-split"},
		{"BUDGET", "R-working-vs-substrate-budget"},
	}
	anySuperRule := false
	for _, sr := range superRuleIDs {
		r, ok := reqByID[sr.ID]
		if ok {
			lines = append(lines, "**"+serviceText(g, sr.Label)+"** ("+sr.ID+"):")
			lines = append(lines, serviceText(g, "  Claim: %s", requirementClaim(g, r)))
			lines = append(lines, serviceText(g, "  Why: %s", r.Why))
			lines = append(lines, "")
			anySuperRule = true
		}
	}
	if !anySuperRule {
		lines = append(lines, serviceText(g, "_No super-rule requirements SETTLED in this domain's graph yet._"))
		lines = append(lines, "")
	}

	lines = append(lines, serviceText(g, "## 5. The conscience"))
	lines = append(lines, "")
	rCCS, ok := reqByID["R-critical-core-scope"]
	if ok {
		lines = append(lines, requirementClaim(g, rCCS))
		lines = append(lines, "")
		lines = append(lines, rCCS.Why)
		lines = append(lines, "")
	}
	criticalCoreVerifyLine := serviceText(g, "The six critical-core invariants (M7 / R-critical-core-scope) — verified on every run by `go test ./internal/invariants/...`. Do NOT skip them; do NOT soften them.")
	criticalCoreNamesLine := serviceText(g, "The six `CRITICAL_CORE_INVARIANTS` (verbatim check names from `internal/invariants`):")
	if consumer {
		// Rephrase without naming the Go package path (internal/invariants) —
		// a dead-end reference for an external consumer with no internal/
		// tree. The guarantee is identical: the six checks run on every suite
		// invocation. Full-profile wording (above) stays byte-identical.
		criticalCoreVerifyLine = serviceText(g, "The six critical-core invariants (M7 / R-critical-core-scope) — verified on every run by the framework's built-in critical-core checks. Do NOT skip them; do NOT soften them.")
		criticalCoreNamesLine = serviceText(g, "The six `CRITICAL_CORE_INVARIANTS` (the framework's verbatim critical-core check names):")
	}
	lines = append(lines, criticalCoreVerifyLine)
	lines = append(lines, "")
	lines = append(lines, criticalCoreNamesLine)
	lines = append(lines, "")
	for _, name := range criticalCoreNames {
		lines = append(lines, "  - `"+name+"`")
	}
	lines = append(lines, "")

	lines = append(lines, serviceText(g, "## 6. The boot sequence (what to do RIGHT NOW)"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "Run, in order:"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "  1. `go test ./...`                                     → suite green?"))
	lines = append(lines, serviceText(g, "  2. `hotam gen-spec` (twice)                            → deterministic?"))
	lines = append(lines, serviceText(g, "  3. `hotam what-now --limit 20`                         → what is the top action?"))
	lines = append(lines, serviceText(g, "  4. `hotam all-violations`                              → any structural violations?"))
	lines = append(lines, serviceText(g, "  5. Read `%s`     → what's claimed but not guaranteed?", localizedDomainDocPath(g, domainName, "UNENFORCED.md")))
	lines = append(lines, serviceText(g, "  6. Read `%s`        → what's been decided / rejected?", localizedDomainDocPath(g, domainName, "HISTORY.md")))
	lines = append(lines, serviceText(g, "  7. Read `%s`      → which M-decisions are open?", localizedDomainDocPath(g, domainName, "DECISIONS.md")))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "If the top action is P3 CONFLICT_STALLED: invoke the relevant playbook\n(`docs/playbooks/`), surface assumptions, propose 2-3 variants, get resolver\napproval, apply via `hotam apply-proposal <file.json> --domain <path> --today YYYY-MM-DD`.\nThe closure check (R-verify-closure-per-action) will confirm advancement."))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "If the top action is P4 OPEN_ITEM: same procedure."))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "If the top action is P1 STRUCTURE: stop. A structural violation means the\ngraph is malformed — investigate the root cause; do not edit by hand.\n`hotam apply-proposal` refuses non-resolvered structural changes."))
	lines = append(lines, "")

	lines = append(lines, serviceText(g, "## 7. The methodology's laws (full constitutional set)"))
	lines = append(lines, "")
	if g.IsEmpty() {
		lines = append(lines, serviceText(g, "_No content domain loaded yet — no `domains/<name>/graph.json` found or empty. The framework laws above still hold; the roster below will populate once a domain is loaded._"))
		lines = append(lines, "")
	} else {
		reqsOrdered := NarrativeOrder(g.Requirements, func(r ontology.Requirement) int { return r.DeclOrder })
		var tableRows []string
		tableRows = append(tableRows, serviceText(g, "| anchor | enforcement | claim |"))
		tableRows = append(tableRows, "|---|---|---|")
		anyConstitutional := false
		for _, cat := range constitutionCategories {
			catHasReqs := false
			for _, r := range reqsOrdered {
				if _, in := cat.IDs[r.ID]; in {
					if !catHasReqs {
						tableRows = append(tableRows, "| **"+serviceText(g, cat.Label)+"** | | |")
						catHasReqs = true
						anyConstitutional = true
					}
					enf := r.Enforcement
					if enf == "" {
						enf = "PROSE"
					}
					tableRows = append(tableRows, "| `"+r.ID+"` | "+enf+" | "+Cell(requirementClaim(g, r))+" |")
				}
			}
		}
		if !anyConstitutional {
			lines = append(lines, serviceText(g, "_No requirements from the methodology's constitutional set are SETTLED in this domain's graph yet._"))
			lines = append(lines, "")
		} else {
			lines = append(lines, tableRows...)
			lines = append(lines, "")
		}
	}

	lines = append(lines, serviceText(g, "## 8. What is yours; what is not"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "YOUR scope (within the hard boundary):"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "  - propose Requirements / Conflict transitions / Rejections via the proposal"))
	lines = append(lines, serviceText(g, "    protocol;"))
	lines = append(lines, serviceText(g, "  - run `hotam what-now`, `hotam gen-spec`;"))
	lines = append(lines, serviceText(g, "  - call `hotam apply-proposal` with a resolver-approved JSON;"))
	lines = append(lines, serviceText(g, "  - crystallize working knowledge into requirement-code;"))
	lines = append(lines, serviceText(g, "  - cite anchors in every communication."))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "NOT yours (resolver's act):"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "  - approving a proposal (the resolver writes the `decided_by`);"))
	lines = append(lines, serviceText(g, "  - resolving an OPEN(question) requirement's content;"))
	lines = append(lines, serviceText(g, "  - closing a Conflict (the operator presents, the resolver decides);"))
	lines = append(lines, serviceText(g, "  - running `git commit` (the act of recording in history is the resolver's)."))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "This is verbatim from R-ai-presents-not-decides + R-operator-not-self-approve."))
	lines = append(lines, "")

	lines = append(lines, serviceText(g, "## 9. If you are unsure"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "Re-read this file. Then read CLAUDE.md (your crystal — the index).\nIf a question remains, surface it to the resolver as a `ProposedRequirement`\nwith status OPEN(<question>). That is how the methodology questions itself."))
	lines = append(lines, "")

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

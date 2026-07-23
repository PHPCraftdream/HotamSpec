package generator

import (
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

const hostCharWarn = 130000
const hostCharCap = 150000

var bandLabel = map[int]string{
	0: "REFLECTION",
	1: "STRUCTURE",
	2: "DRIFT_FALLOUT",
	3: "CONFLICT_STALLED",
	4: "OPEN_ITEM",
	5: "LATENT_CONNECTOR",
	6: "PENDING_PROPOSAL",
	7: "ADVISORY",
}

const ctxLineStatic = "context: UNMEASURED — measuring working-context requires host cooperation the framework will not touch (R-work-within-launch-dir); it measures only if the local stdin payload honestly carries ctx_pct — R-unmeasured-cipher-names-host-boundary"

func BuildLiveState(g *ontology.Graph, domainName string, claudeMDCharCount int, today string) string {
	return BuildLiveStateWithViolations(g, domainName, claudeMDCharCount, today, invariants.AllViolations(g))
}

// BuildLiveStateWithViolations is BuildLiveState's core, parameterized over
// an already-computed violations slice instead of computing
// invariants.AllViolations(g) itself. Exported (not merely
// package-private) so cmd/hotam/gen_spec.go's genSpec can compute
// invariants.AllViolations(g) exactly ONCE and thread the SAME result
// through every render that needs it (live-state.md, AGENT-CONTEXT.md, the
// crystal's own LIVE-STATE block AND its DOMAIN-MAP self-entry) — both for
// efficiency (removes what used to be a redundant second/third/fourth
// AllViolations(g) pass per gen-spec run) and correctness (see
// ViolationsOverride's doc comment in claudemd.go: DOMAIN-MAP's per-domain
// pulse, including the active domain's OWN self-entry, must use this exact
// SAME violations value or its "open actions" count can silently disagree
// with LIVE-STATE's for the identical domain, one block apart in the same
// file — a real bug found while implementing check_domain_claude_md_current,
// E4). BuildLiveState above remains the entry point every caller that does
// NOT need this consistency (a standalone BuildLiveState call with no
// DOMAIN-MAP block alongside it) uses unchanged.
func BuildLiveStateWithViolations(g *ontology.Graph, domainName string, claudeMDCharCount int, today string, violations []invariants.Violation) string {
	if domainName == "" {
		domainName = "hotam-spec-self"
	}
	signals := diagnose.DiagnoseSignalsWithViolations(g, today, violations)
	var topLine string
	if len(signals) > 0 {
		sig := signals[0]
		// diagnose.Finding.Imperative is domain-agnostic (the diagnose package
		// never carries a domain name — see internal/diagnose/signal.go); a
		// bare "docs/gen/UNENFORCED.md" pointer embedded in a Finding message
		// (internal/diagnose/finding.go ReflectUnenforcedSettled) would resolve
		// to a nonexistent repo-root path once rendered here (every domain's
		// generated docs live under domains/<name>/docs/gen/, never at the repo
		// root — see RenderEmbeddedThinkingBlock / BuildAgentContext's own
		// domainName-qualification of the same pointer pattern). Qualify it at
		// the one point domainName is actually known, the same fix pattern
		// task #103 already applied to this exact call site's sibling
		// (domainPulse's mid-word truncation bug).
		msg := strings.ReplaceAll(sig.Message, "docs/gen/UNENFORCED.md", "domains/"+domainName+"/docs/gen/UNENFORCED.md")
		topLine = fmt.Sprintf("[P%d] %s on `%s` — %s", sig.Priority, bandLabel[sig.Priority], sig.Target, msg)
	} else {
		topLine = "none — graph clean"
	}

	var settledTotal, settledEnforced, draft, openReqs, unenforced int
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED {
			settledTotal++
			if r.Enforcement == ontology.EnforcementENFORCED {
				settledEnforced++
			}
			if r.IsCloseableDebt() {
				unenforced++
			}
		}
		if r.Status == ontology.StatusDRAFT {
			draft++
		}
		if r.IsOpen() {
			openReqs++
		}
	}
	debtLine := fmt.Sprintf("%d/%d SETTLED ENFORCED · %d DRAFT · %d OPEN · %d closeable debt (ENFORCEABLE, still PROSE/STRUCTURAL)", settledEnforced, settledTotal, draft, openReqs, unenforced)

	nodes := len(g.Requirements) + len(g.Conflicts) + len(g.Assumptions)
	opBudget := 0
	opMeasure := ontology.BudgetMeasureNODE_COUNT
	budgetDeclared := false
	if len(g.Operators) > 0 {
		opBudget = g.Operators[0].ContextBudget.Limit
		opMeasure = g.Operators[0].ContextBudget.Measure
		budgetDeclared = opBudget > 0
	}

	// budgetDeclared is false both when a domain declares no Operator at all
	// (g.Operators empty, e.g. gpsm-sm — a consumer domain with no OP-...
	// node yet) and when an Operator exists but its ContextBudget.Limit is
	// genuinely the zero-value (never configured). Either way, computing
	// "headroom" against a 0 budget yields a large NEGATIVE number that
	// reads as an alarming false signal on literally the first content line
	// of the pulse a fresh agent reads (external review R4 §4.6, task
	// #337). Render the honest "not set" form instead of a misleading
	// negative headroom — no arithmetic against a budget that was never
	// declared. Domains that DO declare a real budget (opBudget > 0, e.g.
	// hotam-spec-self) are unaffected: this branch is unreachable for them.
	var budgetLine string
	if !budgetDeclared {
		budgetLine = fmt.Sprintf("- **graph:** %d nodes (req+conflict+assumption); OP-director budget: not set (no context_budget declared — headroom not computed)", nodes)
	} else if opMeasure == ontology.BudgetMeasureCRYSTAL_CHARS {
		used := claudeMDCharCount
		budgetLine = fmt.Sprintf("- **graph:** %d nodes (req+conflict+assumption); OP-director budget %d chars (CRYSTAL_CHARS measure) — resident crystal %d chars (headroom %d)", nodes, opBudget, used, opBudget-used)
	} else {
		budgetLine = fmt.Sprintf("- **graph:** %d nodes (req+conflict+assumption); OP-director budget %d nodes (NODE_COUNT measure) (headroom %d)", nodes, opBudget, opBudget-nodes)
	}

	approxSize := claudeMDCharCount
	var crystalLine string
	if approxSize >= hostCharCap {
		crystalLine = fmt.Sprintf("OVER host cap %d chars — split/distill required", hostCharCap)
	} else if approxSize >= hostCharWarn {
		crystalLine = fmt.Sprintf("NEAR — approaching %d char warn threshold (host cap %d)", hostCharWarn, hostCharCap)
	} else {
		crystalLine = fmt.Sprintf("OK — under %d char warn threshold (host cap %d)", hostCharWarn, hostCharCap)
	}

	lines := []string{
		"### Live state (autogenerated by `hotam gen-spec` — do not hand-edit)",
		"",
		fmt.Sprintf("- **top action:** %s", topLine),
		fmt.Sprintf("- **debt:** %s", debtLine),
		budgetLine,
		fmt.Sprintf("- **crystal:** %s", crystalLine),
		fmt.Sprintf("- %s", ctxLineStatic),
	}
	return strings.Join(lines, "\n")
}

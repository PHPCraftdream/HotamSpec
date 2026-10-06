// live_state_current.go holds check_live_state_md_current: the mechanical
// staleness gate for a domain's committed docs/gen/live-state.md (review
// finding P3-12). Like AGENT-CONTEXT.md (agent_context_current.go) and
// CLAUDE.md (claude_md_current.go), a domain's live-state.md is regenerated
// only by `hotam gen-spec`, never by a graph mutation itself, so nothing
// previously proved a COMMITTED live-state.md still matches what a fresh
// render of the current graph would produce — fully overwriting the file with
// garbage yielded 0 violations from `hotam all-violations`.
//
// WHY A SEPARATE CHECK, not a widening of check_agent_context_md_current:
// that check's Claim/Rule name AGENT-CONTEXT.md specifically, and widening
// them to also cover live-state.md would make the name lie about what it
// judges. A distinct check keeps each projection's gate honestly scoped.
//
// WHY THIS FILE ONLY DECLARES THE STUB (see checkLiveStateMDCurrentUnwired
// below): the real comparison needs internal/generator's
// BuildLiveStateWithViolationsRoot — but internal/invariants must NEVER
// import internal/generator (a real, mechanically-enforced import cycle: see
// claude_md_current.go's package doc comment for the full rationale). The
// real Go logic therefore lives in cmd/hotam
// (live_state_current_wiring.go), the one place that already imports both
// internal/generator and internal/invariants. This file registers
// check_live_state_md_current with an honest, always-clean PLACEHOLDER
// PostProcessCheck, patched to the real implementation by cmd/hotam's init()
// via registry.Update — the same wiring split check_agent_context_md_current
// and check_domain_claude_md_current already establish.
package invariants

import (
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkLiveStateMDCurrentUnwired is the pre-wiring placeholder: an honest
// no-op (never reports a violation) rather than a panic or a silently wrong
// verdict. See checkAgentContextMDCurrentUnwired's doc comment for why an
// absent wiring degrades to honest silence, never a false green OR a hard
// failure.
func checkLiveStateMDCurrentUnwired(g *ontology.Graph, priorViolations []Violation) []Violation {
	return nil
}

var _ = All.MustRegister("check_live_state_md_current", Invariant{
	Name:                     "check_live_state_md_current",
	ComparesOnDiskProjection: true,
	Canon:                    methodology.Domain,
	Claim: "a domain's committed docs/gen/live-state.md, if the standalone projection is written at all, is byte-identical " +
		"to what a fresh `hotam gen-spec` run produces right now (the render embedding the same publication violation " +
		"snapshot genSpec threads, not the full AllViolations set, and pinned to the generation date the file stamps " +
		"itself with); a domain that never gets the file (empty graph, or explicit language configurations) is an honest no-op.",
	Rule: "IF <domainDir>/docs/gen/live-state.md exists AND generator.LiveStateMDHasContent says the standalone " +
		"projection is not withheld for this graph, its bytes MUST equal generator.BuildLiveStateWithViolationsRoot's " +
		"output over the current graph, the SAME charCount fixpoint genSpec embeds, the publication-flavored violation " +
		"set (PublicationViolationsFromPhaseOne of the current pass's phase-1 result), and the generation date the " +
		"file stamps itself with (`- **generated:** <date>`); file absent OR withheld OR the graph has explicit " +
		"language configurations = honest NO-OP (localized live-state views are judged by check_language_outputs_current).",
	Why: "Review finding P3-12: docs/gen/live-state.md was the last non-localized docs/gen projection with no freshness " +
		"check — fully overwriting the committed file with garbage yielded 0 violations from `hotam all-violations`. The " +
		"render genuinely depends on `today` (diagnose.DiagnoseSignalsWithViolations embeds it: OVERDUE counts vs " +
		"review_after, the `--today` advisory), yet the file carried no date, so a naive compare-as-of-today would flag " +
		"a content-identical file every midnight; the fix stamps the generation date into the file itself and the check " +
		"pins its render to that stamped date. The FLAVOR choice mirrors check_agent_context_md_current: comparing " +
		"against a full-AllViolations-flavored render would fire spuriously whenever an unrelated disk projection is " +
		"transiently stale, so the comparison embeds the publication flavor — the exact set genSpec threads into the " +
		"written file. See this file's own package doc comment for why the real Go logic is wired in from cmd/hotam " +
		"rather than implemented directly in this package.",
	PostProcessCheck: checkLiveStateMDCurrentUnwired,
})

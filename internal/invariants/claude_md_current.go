// claude_md_current.go holds check_domain_claude_md_current: the mechanical
// staleness gate for a domain's committed root/local CLAUDE.md, the sibling
// gap external review (P1) named alongside check_spec_md_current's own
// staleness gate for docs/gen/SPEC.md (spec_md_current.go) — a domain's
// CLAUDE.md is regenerated only by `hotam gen-spec` (with --claude-md, or
// via resolveClaudeMDPath's auto-write defaults), never by a graph mutation
// itself, so nothing previously proved a COMMITTED CLAUDE.md still matches
// what a fresh render of the current graph would produce.
//
// WHY THIS FILE ONLY DECLARES THE STUB (see checkDomainClaudeMDCurrentUnwired
// below): the real comparison needs internal/generator.
// RenderClaudeMDFromTemplateWithViolations — but internal/invariants must
// NEVER import internal/generator (a real, mechanically-enforced import
// cycle: internal/generator already depends on internal/invariants
// TRANSITIVELY via internal/generator -> internal/diagnose ->
// internal/invariants; verified directly — adding the reverse edge breaks
// `go build ./internal/invariants/...`, and even `_test.go` files inside
// package invariants hit the identical cycle). check_spec_md_current
// sidestepped this by relocating the shared render logic into internal/gate
// (a true leaf both internal/generator and internal/invariants already
// depend on downward). That relocation is NOT available here: CLAUDE.md's
// render is not a pure function of the graph the way SPEC.md's is — its
// LIVE-STATE and DOMAIN-MAP blocks embed internal/diagnose.DiagnoseSignals,
// which itself calls invariants.AllViolations(g). A render call placed
// directly inside this check's own Check func (even via a relocated
// internal/gate copy) would therefore recurse UNBOUNDEDLY the moment
// AllViolations reached this check: AllViolations -> this check -> render ->
// DiagnoseSignals -> AllViolations -> this check -> ... forever — a strictly
// harder problem than check_spec_md_current's own recursion concern (that
// one is bounded by go test's PROCESS boundary; see
// internal/gate/test_exec.go's recursionGuardEnv doc comment — there is no
// such boundary here, this is all one call stack).
//
// The fix has two halves, both already in place:
//
//  1. internal/invariants/all_violations.go's two-phase runViolations engine
//     (Invariant.PostProcessCheck) runs every ORDINARY check first, then
//     feeds that phase's complete violation list to this check as a plain
//     argument — never re-entering AllViolations for the same graph.
//  2. internal/generator/claudemd.go's ViolationsOverride +
//     RenderClaudeMDFromTemplateWithViolations let a caller supply that same
//     already-computed violation list to the render instead of recomputing
//     it via DiagnoseSignals — so the render this check verifies against is
//     the SAME snapshot every other check in the current AllViolations pass
//     saw, not a stale or re-derived one.
//
// What is STILL missing is the wiring between the two: this check's real
// Go logic needs internal/generator (for the render) AND cmd/hotam's own
// resolveClaudeMDPath/repoRootForDomain/domainNameFromDir family (for
// "where does this domain's crystal live on disk" — package main, not
// re-derivable here without duplicating that logic). cmd/hotam already
// imports both internal/generator and internal/invariants, so it is the one
// place that CAN implement the real comparison. This file therefore
// registers check_domain_claude_md_current with an honest, always-clean
// PLACEHOLDER PostProcessCheck (checkDomainClaudeMDCurrentUnwired) — mirrors
// tool_wiring.go's identical pattern for methodology.Tools' Run field
// (Tool declared with Run: nil in internal/methodology/tools_data.go,
// patched to a real cmd* function by cmd/hotam/tool_wiring.go's init(), via
// the SAME registry.Update mechanism used here) — and cmd/hotam/
// claude_md_current_wiring.go's init() patches in the real implementation
// via invariants.All.Update the moment cmd/hotam itself loads. Go's init()
// ordering guarantees every dependency's init() (including this file's
// MustRegister call) completes before cmd/hotam's own init() runs, so by the
// time any cmd/hotam subcommand executes, the real check is always wired —
// the placeholder below is reachable ONLY from a test that imports
// internal/invariants without also importing (or being imported by) cmd/hotam.
package invariants

import (
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkDomainClaudeMDCurrentUnwired is the pre-wiring placeholder: an honest
// no-op (never reports a violation) rather than a panic or a silently wrong
// verdict. A caller that never links cmd/hotam (e.g. `go test
// ./internal/invariants/...` in isolation) gets a check that correctly
// contributes nothing rather than crashing the whole AllViolations fan-out —
// the same "absent capability degrades to honest silence, never a false
// green OR a hard failure" posture check_recorder_current/
// check_spec_md_current already establish for their own opt-in absence
// cases, applied here to an absent WIRING rather than an absent file.
func checkDomainClaudeMDCurrentUnwired(g *ontology.Graph, priorViolations []Violation) []Violation {
	return nil
}

var _ = All.MustRegister("check_domain_claude_md_current", Invariant{
	Name:                     "check_domain_claude_md_current",
	ComparesOnDiskProjection: true,
	Canon:                    methodology.Domain,
	Claim: "a domain's committed root or local CLAUDE.md, if present, has its GENERATED portion (everything up to and " +
		"including the durable-notes marker line) byte-identical to what a fresh `hotam gen-spec` render produces right " +
		"now; a domain with no CLAUDE.md yet, or whose project has not adopted the crystal convention at all, is an " +
		"honest no-op; content the operator authored below the durable-notes marker is never compared and never flagged.",
	Rule: "IF a file exists at the domain's resolved crystal path (repo-root CLAUDE.md for the active/unambiguous " +
		"domain, or <domainDir>/CLAUDE.md for a non-active consumer domain under domains/<name> -- the SAME resolution " +
		"`hotam land`/`hotam gen-spec` already use), the bytes from the start of the file THROUGH the durable-notes " +
		"marker line MUST equal internal/generator.RenderClaudeMDFromTemplateWithViolations's own output over that same " +
		"span, computed against the CURRENT graph and the SAME violation snapshot every other check in the current " +
		"all-violations pass just saw. Bytes AFTER the marker line (the operator's own durable notes) are never read for " +
		"comparison. A domain with no committed CLAUDE.md, or whose project root carries neither a CLAUDE.md nor a " +
		"crystal-convention marker, is an honest NO-OP -- mirrors check_spec_md_current's own opt-in-when-absent shape, " +
		"except CLAUDE.md is the domain's mandatory boot crystal once the convention is adopted at all, not an " +
		"additionally opt-in discipline layer.",
	Why: "External review P1: check_spec_md_current closed the staleness gap for docs/gen/SPEC.md, but CLAUDE.md -- the " +
		"resident boot crystal every operator session actually reads first -- had no equivalent freshness invariant at " +
		"all. A domain could land a graph change (a new SETTLED requirement, a closed debt item, a changed pulse) and " +
		"`hotam all-violations` would report nothing wrong even though the committed CLAUDE.md/AGENTS.md/GEMINI.md now " +
		"silently disagreed with the graph it claims to summarize -- exactly the class of drift check_spec_md_current " +
		"exists to catch for SPEC.md, just for the ONE file every session boots from. The durable-notes-tail exclusion " +
		"is load-bearing, not incidental: CLAUDE.md's own template (internal/generator/claudemd.go's claudeMDTemplate) " +
		"explicitly invites the operator to write freeform notes below a marker line the generator promises never to " +
		"touch on regeneration -- a naive whole-file byte comparison would turn every legitimate use of that documented " +
		"escape hatch into a false staleness violation, punishing the exact behavior the template invites. See this " +
		"file's own package doc comment for why the comparison runs as a POST-PROCESS check (Invariant." +
		"PostProcessCheck) rather than an ordinary one, and why the real Go logic is wired in from cmd/hotam rather than " +
		"implemented directly in this package.",
	PostProcessCheck: checkDomainClaudeMDCurrentUnwired,
})

// agent_context_current.go holds check_agent_context_md_current: the
// mechanical staleness gate for a domain's committed docs/gen/AGENT-CONTEXT.md
// (review finding P3-11). Like CLAUDE.md, a domain's AGENT-CONTEXT.md is
// regenerated only by `hotam gen-spec`, never by a graph mutation itself, so
// nothing previously proved a COMMITTED AGENT-CONTEXT.md still matches what a
// fresh render of the current graph would produce — `hotam all-violations`
// could return 0 while the file on disk listed violations.
//
// WHY THIS FILE ONLY DECLARES THE STUB (see checkAgentContextMDCurrentUnwired
// below): the real comparison needs internal/generator's
// BuildAgentContextRootWithViolations — but internal/invariants must NEVER
// import internal/generator (a real, mechanically-enforced import cycle: see
// claude_md_current.go's own package doc comment for the full rationale).
// The real Go logic therefore lives in cmd/hotam
// (agent_context_current_wiring.go), the one place that already imports both
// internal/generator and internal/invariants. This file registers
// check_agent_context_md_current with an honest, always-clean PLACEHOLDER
// PostProcessCheck, patched to the real implementation by cmd/hotam's init()
// via registry.Update — the same wiring split check_domain_claude_md_current
// already establishes.
package invariants

import (
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkAgentContextMDCurrentUnwired is the pre-wiring placeholder: an honest
// no-op (never reports a violation) rather than a panic or a silently wrong
// verdict. See checkDomainClaudeMDCurrentUnwired's doc comment for why an
// absent wiring degrades to honest silence, never a false green OR a hard
// failure.
func checkAgentContextMDCurrentUnwired(g *ontology.Graph, priorViolations []Violation) []Violation {
	return nil
}

var _ = All.MustRegister("check_agent_context_md_current", Invariant{
	Name:                     "check_agent_context_md_current",
	ComparesOnDiskProjection: true,
	Canon:                    methodology.Domain,
	Claim: "a domain's committed docs/gen/AGENT-CONTEXT.md, if present, is byte-identical to what a fresh `hotam " +
		"gen-spec` run produces right now (the render embedding the same publication violation snapshot genSpec " +
		"threads, not the full AllViolations set); a domain that has none yet is an honest no-op.",
	Rule: "IF <domainDir>/docs/gen/AGENT-CONTEXT.md exists, its bytes MUST equal " +
		"generator.BuildAgentContextRootWithViolations's output over the current graph, the SAME charCount fixpoint " +
		"genSpec embeds, and the publication-flavored violation set (PublicationViolationsFromPhaseOne of the current " +
		"pass's phase-1 result); absent file = honest NO-OP; explicit language configurations are judged by " +
		"check_language_outputs_current instead.",
	Why: "Review finding P3-11: AGENT-CONTEXT.md freshness was covered by no check at all — the file is written from " +
		"a violation snapshot taken before the staged bundle publishes, so it could list disk-projection STRUCTURE " +
		"signals the same run has already made false, and `all-violations` returning 0 coexisted with a file listing " +
		"violations. The FLAVOR choice is load-bearing, not incidental: comparing against a full-AllViolations-flavored " +
		"render would fire spuriously whenever an unrelated disk projection is transiently stale (e.g. a hand-tampered " +
		"ENGINE-VERSION stamp would make the fresh render disagree even though a re-run would rewrite AGENT-CONTEXT " +
		"byte-identically), so the comparison embeds the publication flavor — the exact set genSpec threads into the " +
		"written file. See this file's own package doc comment for why the real Go logic is wired in from cmd/hotam " +
		"rather than implemented directly in this package.",
	PostProcessCheck: checkAgentContextMDCurrentUnwired,
})

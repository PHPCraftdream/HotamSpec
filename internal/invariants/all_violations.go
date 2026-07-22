package invariants

import (
	"sync"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

var frameworkScopedInvariantNames = map[string]struct{}{
	"check_bijection_r_to_enforcer":                 {},
	"check_method_matches_docstring":                {},
	"check_rules_as_data_classification_coherent":   {},
	"check_domain_manifest_exists_and_importable":   {},
	"check_domain_manifest_id_matches_dirname":      {},
	"check_domain_manifest_description_nonempty":    {},
	"check_domain_manifest_goals_nonempty":          {},
	"check_domain_manifest_director_nonempty":       {},
	"check_domain_director_exists":                  {},
	"check_agent_has_agents_subdir":                 {},
	"check_agent_has_docs_subdir":                   {},
	"check_agent_has_tools_subdir":                  {},
	"check_constituting_not_in_unresolved_conflict": {},
}

// AllViolations runs every registered, non-delegator, in-scope invariant
// against g and returns the union of every violation found — the full,
// unfiltered signal internal/diagnose, `hotam all-violations`, and `hotam
// status` all report staleness debt (including ComparesOnDiskProjection
// checks like check_spec_md_current/check_domain_claude_md_current) through.
//
// Two phases: ordinary checks (Invariant.Check, unchanged shape) run first,
// fanned out in parallel exactly as before; POST-PROCESS checks
// (Invariant.PostProcessCheck non-nil — see that field's doc comment) run
// second, strictly after phase 1 completes, each receiving phase 1's full
// violation list as an argument. This ordering is what lets
// check_domain_claude_md_current render a byte-fresh CLAUDE.md (which embeds
// the SAME violation set phase 1 just computed, for its LIVE-STATE/
// DOMAIN-MAP pulse lines) without ever calling back into AllViolations for
// the same graph.
func AllViolations(g *ontology.Graph) []Violation {
	return runViolations(g, All.All())
}

// AllViolationsForProposalGate is the SAME two-phase run as AllViolations,
// additionally excluding every ComparesOnDiskProjection check from BOTH
// phases. It exists for exactly one caller: internal/proposal/apply.go's
// applyToGraph, which uses a pre/post-mutation violation DIFF to decide
// whether a proposal is safe to write — a comparison ComparesOnDiskProjection
// checks cannot meaningfully participate in (see that field's own doc
// comment for why: the "after" graph is in-memory only, never regenerated
// through `hotam gen-spec`, so a committed SPEC.md/CLAUDE.md is guaranteed
// to disagree with a fresh render immediately after any substantive
// mutation — not a real signal). Every OTHER invariant (the structural
// floor) still runs and still gates the proposal exactly as before; this
// filter narrows nothing else. AllViolations itself is UNCHANGED and
// continues to report ComparesOnDiskProjection staleness for every other
// caller (`all-violations`, `status`, internal/diagnose) — see
// ComparesOnDiskProjection's doc comment.
//
// This is now a thin wrapper over AllViolationsExcludingDiskProjection,
// which generalizes the same filter for a second, independent caller (see
// that function's own doc comment) — the proposal-gate NAME is kept as the
// public entry point internal/proposal already imports, unchanged.
func AllViolationsForProposalGate(g *ontology.Graph) []Violation {
	return AllViolationsExcludingDiskProjection(g)
}

// AllViolationsExcludingDiskProjection runs every registered invariant
// EXCEPT those marked ComparesOnDiskProjection: true (see that field's doc
// comment) — the same filtered candidate set AllViolationsForProposalGate
// uses, under a name that does not imply "only for the proposal gate".
//
// SECOND CALLER (the reason this was pulled out of
// AllViolationsForProposalGate as its own function rather than left as a
// proposal-gate-only implementation detail): internal/diagnose's
// DiagnoseSignalsExcludingDiskProjection, in turn used by
// internal/generator's domainPulse for every SIBLING domain's pulse line in
// a DOMAIN-MAP block render. A sibling's one-line pulse summary has never
// depended on check_spec_md_current/check_domain_claude_md_current firing —
// those are byte-comparison staleness checks against generated files, never
// a signal a human would expect to see surfacing as a domain's top-action
// headline. Calling plain AllViolations there instead would (and, before
// this fix, DID) create real mutual recursion: rendering domain A's
// DOMAIN-MAP block computes sibling domain B's violations via
// AllViolations(B), which — because check_domain_claude_md_current is
// registered unconditionally for every graph — runs B's OWN
// check_domain_claude_md_current, which renders B's OWN DOMAIN-MAP block,
// which lists ITS siblings (including A, freshly reloaded from disk as a
// distinct graph pointer), computing AllViolations(A) again, and so on —
// confirmed in practice via a captured goroutine stack trace showing exactly
// this chain, observed as `hotam gen-spec --claude-md` hanging 20+ minutes
// against a real two-domain project (hotam-spec-self / hotam-dev) before
// this fix. Excluding both ComparesOnDiskProjection checks from the
// SIBLING-pulse computation breaks the cycle at its source: a sibling's
// AllViolations-equivalent pass can no longer re-enter
// RenderClaudeMDFromTemplateWithViolations for anything. The ACTIVE domain's
// own precise LIVE-STATE signal set is unaffected — check_domain_claude_md_current
// supplies it directly via ViolationsOverride (internal/generator/claudemd.go),
// never through this filtered fallback path.
func AllViolationsExcludingDiskProjection(g *ontology.Graph) []Violation {
	all := All.All()
	filtered := make([]Invariant, 0, len(all))
	for _, inv := range all {
		if inv.ComparesOnDiskProjection {
			continue
		}
		filtered = append(filtered, inv)
	}
	return runViolations(g, filtered)
}

// PriorToPostProcessViolations returns EXACTLY the violation set
// check_domain_claude_md_current's own PostProcessCheck receives as
// priorViolations inside a real AllViolations(g) run — i.e. runViolations'
// phase 1 (every ordinary, Check-based invariant, in-scope for g), computed
// standalone rather than as a side effect of a full AllViolations call.
//
// WHY THIS EXISTS (a real, found-in-practice bug, not speculative): a naive
// caller wanting to render a fresh CLAUDE.md "the same way
// check_domain_claude_md_current will compare it" might reach for plain
// AllViolations(g) — but that returns MORE than what the check's own
// priorViolations argument actually contains, because AllViolations' return
// value additionally includes phase 2's own output (every PostProcessCheck's
// violations, including check_domain_claude_md_current's OWN verdict about
// the file that is ABOUT TO BE OVERWRITTEN by this very render). Using that
// larger set to render the file being written creates a structural,
// permanent mismatch: cmd/hotam/gen_spec.go's genSpec used to call plain
// AllViolations(g) for its own activeViolations (fed via ViolationsOverride
// into the crystal render), while check_domain_claude_md_current itself
// only ever sees the SMALLER phase-1-only set — so a freshly-written,
// genuinely-correct CLAUDE.md would STILL fail check_domain_claude_md_current
// on the very next all-violations run, forever, because the two sides of the
// comparison were fed different violation counts by construction (observed
// directly: a real gen-spec run against hotam-spec-self produced a
// DOMAIN-MAP "open actions" count reflecting invariants.AllViolations(g)'s
// full 2-violation result, while check_domain_claude_md_current's own
// priorViolations argument in the SAME logical run only ever contains 1).
// genSpec now calls THIS function instead, for the exact same reason
// check_domain_claude_md_current itself must use phase 1 — see
// Invariant.PostProcessCheck's doc comment.
func PriorToPostProcessViolations(g *ontology.Graph) []Violation {
	all := All.All()
	ordinary := make([]Invariant, 0, len(all))
	for _, inv := range all {
		if inv.PostProcessCheck != nil {
			continue
		}
		ordinary = append(ordinary, inv)
	}
	return runViolations(g, ordinary)
}

// runViolations is the shared two-phase engine behind AllViolations and
// AllViolationsForProposalGate: candidates is the already-filtered
// (IsDelegator/frameworkScoped/ComparesOnDiskProjection as applicable)
// invariant list to run against g.
func runViolations(g *ontology.Graph, candidates []Invariant) []Violation {
	var ordinary, postProcess []Invariant
	for _, inv := range candidates {
		if inv.IsDelegator {
			continue
		}
		if _, scoped := frameworkScopedInvariantNames[inv.Name]; scoped && !g.SelfHosting {
			continue
		}
		if inv.PostProcessCheck != nil {
			postProcess = append(postProcess, inv)
			continue
		}
		ordinary = append(ordinary, inv)
	}

	// Phase 1: every ordinary check, fanned out in parallel exactly as
	// before this function existed.
	results := make([][]Violation, len(ordinary))
	var wg sync.WaitGroup
	for i, inv := range ordinary {
		wg.Add(1)
		go func(idx int, in Invariant) {
			defer wg.Done()
			results[idx] = in.Check(g)
		}(i, inv)
	}
	wg.Wait()
	var out []Violation
	for _, r := range results {
		out = append(out, r...)
	}

	// Phase 2: post-process checks, run strictly AFTER phase 1 completes,
	// each fed phase 1's full violation list (a plain value, not a re-entrant
	// AllViolations call) — see Invariant.PostProcessCheck's doc comment.
	// Also fanned out in parallel (independent of each other, same as phase
	// 1) since nothing here depends on sibling post-process results.
	if len(postProcess) > 0 {
		phase1 := out
		postResults := make([][]Violation, len(postProcess))
		var wg2 sync.WaitGroup
		for i, inv := range postProcess {
			wg2.Add(1)
			go func(idx int, in Invariant) {
				defer wg2.Done()
				postResults[idx] = in.PostProcessCheck(g, phase1)
			}(i, inv)
		}
		wg2.Wait()
		for _, r := range postResults {
			out = append(out, r...)
		}
	}

	return out
}

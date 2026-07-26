// claim_scenario_current.go holds check_claim_matches_scenario (task #369,
// RAC3-A): the drift-detection sibling of check_settled_requires_scenario
// (scenario_discipline.go) — that check proves a scenario CARRIER exists;
// this check proves the graph's CURRENT Claim text still matches what that
// carrier would derive TODAY (internal/selfspec.DeriveClaimsFromScenarios,
// the same real-`go test`-execution machinery `hotam sync-domain` already
// runs at write time — see cmd/hotam/sync_domain.go's own wiring comment).
//
// This is the same "stale generated artifact" class check_recorder_current/
// check_ontology_vendor_current/check_spec_md_current already establish in
// this package (internal/invariants/recorder_check.go,
// ontology_vendor_check.go, spec_md_current.go): read the CURRENT committed
// value, recompute what it SHOULD be right now, fire a violation naming
// exactly what drifted if they disagree. Modeled most closely on
// check_spec_md_current (a real re-render, not a cheap hash compare) because
// Claim derivation, like SPEC.md, is not a pure function of source text
// alone — it requires actually re-running the verified_by test(s) to prove
// what their CURRENT recorded scenario description(s) say.
package invariants

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/selfspec"
)

// checkClaimMatchesScenario is the ONE-WAY discipline gate
// (task #369, RAC3-A): in a domain whose manifest.json declares
// discipline:"full" (mirroring checkSettledRequiresScenario's own g.Discipline
// == loader.DisciplineFull top-of-function guard, scenario_discipline.go),
// every Requirement IN SCOPE for Claim derivation
// (selfspec.RequirementInClaimDerivationScope: Enforceability !=
// INHERENTLY_PROSE AND at least one verified_by entry) MUST carry a Claim
// that equals what a fresh re-derivation (selfspec.DeriveClaimsFromScenarios'
// own per-entry logic: concatenate every verified_by test's currently-
// recorded hotamspec scenario description, in verified_by's declared order,
// joined by selfspec.ClaimDerivationSeparator) would produce RIGHT NOW.
//
// HONEST NO-OP CASES (not false positives), mirroring
// checkSettledRequiresScenario's and checkSpecMDCurrent's own bail
// conventions:
//   - g.Discipline != loader.DisciplineFull: soft discipline (the default,
//     every real domain in this wave) -- honest no-op, regardless of how
//     any Requirement's Claim/verified_by fields are shaped.
//   - A Requirement outside selfspec.RequirementInClaimDerivationScope
//     (INHERENTLY_PROSE, or no verified_by entries at all) -- not a
//     derivation candidate at all, so there is nothing to compare it
//     against; skipped exactly like checkSettledRequiresScenario's own
//     INHERENTLY_PROSE branch.
//   - A Requirement in scope whose verified_by entries currently produce NO
//     narrated scenario at all (every entry fails to resolve/compile/pass,
//     or genuinely records nothing) -- deriveClaimFromVerifiedBy's own "ok"
//     return is false in that case (see internal/selfspec/claim_derive.go),
//     which this check treats as "cannot prove drift either way right now"
//     and skips, rather than firing a confusing violation about an empty
//     expected Claim; check_settled_requires_scenario is the check
//     responsible for flagging a SETTLED requirement that lacks a real
//     scenario carrier at all.
//
// NOT gated on Status == SETTLED, mirroring
// selfspec.RequirementInClaimDerivationScope's own choice not to: a Claim
// that has drifted from its own verified_by scenario is equally dishonest
// whether the requirement is DRAFT or SETTLED — the derivation rule itself
// (see internal/selfspec/claim_derive.go's package doc comment) is about
// where the text comes from, not about lifecycle stage.
//
// COST: real, one `go test` subprocess execution per verified_by entry of
// every in-scope Requirement (gate.RunVerifiedByTestRecording, via
// selfspec.DeriveClaimsFromScenarios) -- the SAME per-entry price
// check_spec_md_current and check_scenario_executes_impl already accept
// elsewhere in this package. ComparesOnDiskProjection: true for the SAME
// reason check_spec_md_current carries it (see that field's own doc comment,
// internal/invariants/invariant.go): immediately after a graph mutation
// (e.g. `hotam sync-domain`'s own SyncGraph call, which ALREADY derives and
// writes the current Claim via this exact same machinery before the mutation
// even happens), a pre/post-mutation diff pass would either always agree
// (redundant, wasted cost) or -- worse -- disagree on unrelated pre-existing
// drift and false-block an unrelated proposal. AllViolationsForProposalGate
// already excludes every ComparesOnDiskProjection check for exactly this
// reason; this check inherits that exclusion by carrying the same flag.
func checkClaimMatchesScenario(g *ontology.Graph) []Violation {
	if g.Discipline != loader.DisciplineFull {
		// Soft discipline -- honest no-op, mirroring
		// checkSettledRequiresScenario's identical guard.
		return nil
	}
	specRoot := gate.SpecRootForGraph(g)
	var out []Violation
	for _, r := range g.Requirements {
		if !selfspec.RequirementInClaimDerivationScope(r) {
			continue
		}
		fresh, ok := freshDerivedClaim(specRoot, g.SelfHosting, r.VerifiedBy)
		if !ok {
			// Nothing currently derivable (every verified_by entry fails to
			// resolve/compile/pass, or records no scenario) -- cannot prove
			// drift either way right now; check_settled_requires_scenario
			// is the check responsible for flagging the missing-carrier case.
			continue
		}
		if fresh == r.Claim {
			continue
		}
		out = append(out, Violation{
			Check: "check_claim_matches_scenario",
			ID:    r.ID,
			Message: fmt.Sprintf(
				"discipline:full requires Claim to match its verified_by scenario description(s): %s's committed Claim "+
					"%q does not match what a fresh derivation from its verified_by entries (%s) produces right now (%q) -- "+
					"either the verified_by test's hotamspec.NewScenario(...) description changed without re-running "+
					"`hotam sync-domain` to re-derive Claim, or graph.json's Claim was hand-edited despite the domain's "+
					"code-authored requirements_authority",
				r.ID, r.Claim, strings.Join(r.VerifiedBy, ", "), fresh),
		})
	}
	return out
}

// freshDerivedClaim mirrors internal/selfspec's own unexported
// deriveClaimFromVerifiedBy exactly (same RunVerifiedByTestRecording +
// concatenation logic) -- duplicated at this package boundary (internal/
// invariants cannot reach an unexported function in internal/selfspec, and
// exporting deriveClaimFromVerifiedBy from selfspec purely for this one
// cross-package call would widen that package's public surface for a single
// caller) via the SAME exported building blocks selfspec.
// DeriveClaimsFromScenarios itself is built from: gate.RunVerifiedByTestRecording,
// gate.ParseFileColonSymbol, and selfspec.ClaimDerivationSeparator (the one
// exported constant naming the join separator, so the two copies can never
// silently disagree on IT even though the surrounding loop is duplicated).
func freshDerivedClaim(specRoot string, selfHosting bool, verifiedBy []string) (claim string, ok bool) {
	var titles []string
	for _, entry := range verifiedBy {
		file, testName, parsedOK := gate.ParseFileColonSymbol(strings.TrimSpace(entry))
		if !parsedOK {
			continue
		}
		result := gate.RunVerifiedByTestRecording(specRoot, file, testName, "")
		if result.Skipped || result.Err != nil || result.CompileFailed || !result.Passed {
			continue
		}
		for _, art := range result.Artifacts {
			title, verdictOK := scenarioArtifactTitleForClaimCheck(art.RawJSON)
			if verdictOK {
				titles = append(titles, title)
			}
		}
	}
	if len(titles) == 0 {
		return "", false
	}
	return strings.Join(titles, selfspec.ClaimDerivationSeparator), true
}

var _ = All.MustRegister("check_claim_matches_scenario", Invariant{
	Name:                     "check_claim_matches_scenario",
	ComparesOnDiskProjection: true,
	Canon:                    methodology.Requirement,
	Claim: "in a discipline:full domain, every Requirement in Claim-derivation scope (not INHERENTLY_PROSE, carries " +
		"verified_by entries) has a Claim that matches the concatenation of its verified_by test(s)' currently-recorded " +
		"hotamspec scenario description(s), joined by a single space, in verified_by's own declared order; a domain " +
		"without discipline:full is an honest no-op.",
	Rule: "IF g.Discipline == loader.DisciplineFull, THEN for every Requirement r where " +
		"selfspec.RequirementInClaimDerivationScope(r) is true (Enforceability != INHERENTLY_PROSE AND len(VerifiedBy) > 0), " +
		"a fresh re-derivation (running every verified_by entry via gate.RunVerifiedByTestRecording and concatenating each " +
		"currently-passing entry's recorded scenario title(s), joined by selfspec.ClaimDerivationSeparator, in " +
		"verified_by's own declared order -- the SAME logic selfspec.DeriveClaimsFromScenarios performs at `hotam " +
		"sync-domain` write time) MUST equal r.Claim exactly. A Requirement outside derivation scope, or one whose fresh " +
		"derivation currently produces nothing at all (every verified_by entry fails to resolve/compile/pass, or records " +
		"no scenario), is skipped -- not a violation (check_settled_requires_scenario is the check responsible for a " +
		"SETTLED requirement's missing carrier). IF g.Discipline is NOT loader.DisciplineFull, this check is a pure " +
		"HONEST NO-OP.",
	Why: "task #369 (RAC3-A) makes a Requirement's Claim a DERIVED projection of its verified_by scenario test(s) -- the " +
		"same class of promise check_recorder_current/check_ontology_vendor_current/check_spec_md_current already make " +
		"mechanical for their own generated artifacts (a vendored recorder copy, a vendored ontology mirror, generated " +
		"SPEC.md text). Without this check, a Requirement's Claim could silently drift from its own proof: an author " +
		"edits a verified_by test's hotamspec.NewScenario(...) title (rewording what the scenario narrates) without " +
		"re-running `hotam sync-domain` to re-derive Claim, and the graph's committed Claim keeps showing the OLD text " +
		"forever, undetected by every other check (none of which re-executes verified_by tests to compare against Claim " +
		"specifically -- check_spec_md_current compares SPEC.md's rendered narrative, not the short Claim field itself). " +
		"RELATION TO check_settled_requires_scenario: that check proves a real scenario CARRIER exists for a SETTLED " +
		"requirement in a discipline:full domain (structural presence); this check proves the CONTENT currently agrees " +
		"with that carrier (semantic freshness) -- the same carrier-vs-content split check_recorder_current (structural: " +
		"file exists) and check_spec_md_current (content: bytes match a fresh render) already draw for their own " +
		"artifacts. ComparesOnDiskProjection is set for the identical reason check_spec_md_current carries it: this " +
		"check's own fresh derivation would otherwise pollute `hotam sync-domain`'s and `hotam land`'s pre/post-mutation " +
		"violation diff gate with guaranteed-stale or redundant signal immediately after a mutation that itself just " +
		"derived and wrote the current Claim through the exact same machinery (see AllViolationsForProposalGate's own " +
		"doc comment, and cmd/hotam/sync_domain.go's DeriveClaimsFromScenarios wiring comment).",
	Check: checkClaimMatchesScenario,
})

// scenarioArtifactTitleForClaimCheck mirrors internal/selfspec's own
// unexported scenarioArtifactTitleIfPass exactly (decode raw JSON, return
// Title only when Verdict == "pass") -- duplicated at this package boundary
// for the identical unexported-symbol reason freshDerivedClaim's own doc
// comment gives.
func scenarioArtifactTitleForClaimCheck(raw []byte) (title string, ok bool) {
	var parsed struct {
		Title   string `json:"title"`
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", false
	}
	if parsed.Verdict != "pass" {
		return "", false
	}
	return parsed.Title, true
}

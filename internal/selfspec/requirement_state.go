// requirement_state.go implements task #370 (RAC3-B, Phase B of the "is a
// Requirement's Claim real?" wave that #369/RAC3-A started): a COMPUTED
// ontology.RequirementProofLifecycle state for a Requirement, derived from
// the same real-`go test`-execution machinery claim_derive.go and
// internal/invariants/claim_scenario_current.go already established --
// rather than every doc generator (TRACEABILITY.md/COVERAGE.md/UNENFORCED.md/
// REQUIREMENTS.md/AGENT-CONTEXT.md) inventing its own "is this requirement
// real/proven/current" predicate over raw Status/Enforcement/carrier-presence
// fields. This is the SAME class of unification #358/#361 already performed
// for "does this generated doc have real content" (the *MDHasContent
// pattern) -- one predicate, many callers, instead of N independent
// reinventions that can silently drift from each other.
//
// RequirementState is deliberately NOT a method on ontology.Requirement
// itself: computing it requires actually running the requirement's
// verified_by test(s) (gate.RunVerifiedByTestRecording), and internal/gate
// already imports internal/ontology -- ontology importing gate back would be
// an import cycle. This mirrors claim_derive.go's own placement choice
// exactly (DeriveClaimsFromScenarios / RequirementInClaimDerivationScope live
// here, in internal/selfspec, for the identical reason), and the resulting
// call shape -- a package-level function taking a Requirement plus its
// graph/spec-root context, returning one of a canonical ontology.Lifecycle's
// state names -- mirrors how ontology.ConflictLifecycle/RequirementStatusLifecycle
// are consumed at their OWN call sites (internal/loader/loader.go,
// internal/invariants/lifecycle_checks.go): a Lifecycle is a value-type
// description of the state space, matched against by callers via
// Lifecycle.Matches(), never a method the node type itself exposes.
package selfspec

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// RequirementState computes r's current ontology.RequirementProofLifecycle
// state -- one of NO_CARRIER / UNVERIFIED / FAILING / STALE / PROVEN (see
// that Lifecycle's own doc comment in internal/ontology/lifecycle.go for the
// full definition of each state) -- by actually re-running every verified_by
// entry, exactly the way DeriveClaimsFromScenarios and
// check_claim_matches_scenario's freshDerivedClaim already do.
//
// specRoot/selfHosting carry the same resolution context those two callers
// already require (gate.SpecRootForGraph(g) for an ordinary domain, the
// engine repository root for a self-hosting one).
//
// COST: identical to freshDerivedClaim / deriveClaimFromVerifiedBy -- one
// real `go test` subprocess execution per verified_by entry
// (gate.RunVerifiedByTestRecording). Callers that need this for MANY
// requirements at once (a doc generator rendering a whole roster) pay this
// cost per requirement, the same per-entry price COVERAGE.md's own
// computeScenarioRatchet and SPEC.md's `--spec` rendering already accept --
// this function does not memoize beyond what gate's own compile/run cache
// already provides.
//
// NOTE: unlike RequirementInClaimDerivationScope, this function does NOT
// gate on Enforceability -- an INHERENTLY_PROSE requirement with a
// verified_by entry still has a real, checkable proof state (it simply also
// happens to be exempt from the Claim-derivation and
// check_settled_requires_scenario carrier demands elsewhere); State() answers
// "is the evidence that exists actually current and passing", a strictly
// narrower question than "is this requirement in Claim-derivation scope".
func RequirementState(r ontology.Requirement, specRoot string, selfHosting bool, atoms *gate.AtomSourceIndex) string {
	if len(r.VerifiedBy) == 0 {
		return "NO_CARRIER"
	}

	sawResolved := false
	allPassed := true
	for _, entry := range r.VerifiedBy {
		file, testName, ok := gate.ParseFileColonSymbol(strings.TrimSpace(entry))
		if !ok {
			continue
		}
		result := gate.RunVerifiedByTestRecording(specRoot, file, testName, "")
		if result.Skipped || result.Err != nil {
			// Neither proven nor disproven at this nesting level (recursion
			// guard) or an infrastructure failure short of a real red bar --
			// contributes no verdict either way, mirroring
			// deriveClaimFromVerifiedBy's own treatment of these cases.
			continue
		}
		sawResolved = true
		if result.CompileFailed || !result.Passed {
			allPassed = false
		}
	}

	if !sawResolved {
		// Declared, but the freshest attempt produced no verdict at all --
		// every entry failed to parse, or every entry that DID parse was
		// Skipped/Err.
		return "UNVERIFIED"
	}
	if !allPassed {
		return "FAILING"
	}

	fresh, ok := deriveClaimFromVerifiedBy(specRoot, selfHosting, r.VerifiedBy, atoms)
	if !ok || fresh == r.Claim {
		// Either nothing narrated (a plain, non-scenario passing test -- no
		// drift is even computable, so the passing evidence alone is enough
		// to call this PROVEN, mirroring check_claim_matches_scenario's own
		// "cannot prove drift either way right now" skip), or the committed
		// Claim already agrees with a fresh re-derivation.
		return "PROVEN"
	}
	return "STALE"
}

package invariants

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkClaimAuthorityRatchet is the claim_authority:"scenario" ONE-WAY
// ratchet gate (task #388/W0.1), symmetric with checkDisciplineRatchet
// (discipline_ratchet.go)'s own F2 ratchet: once a domain has opted into
// claim_authority:"scenario" (loader.ClaimAuthorityScenario) -- the trigger
// that turns on check_claim_matches_scenario's real per-Requirement exact-
// drift gate (claim_scenario_current.go) -- flipping it back to absent (or
// any other value) would be a silent DOWNGRADE of a promise already made
// public in the domain's own manifest, exactly the class of regression
// checkDisciplineRatchet exists to catch for discipline:"full" itself.
//
// This check closes that gap via the SAME graph.lock pin mechanism
// checkDisciplineRatchet already uses: loader.WriteLock records
// ClaimAuthorityScenarioObserved=true once it ever observes the live
// manifest resolving claim_authority:"scenario", and NEVER clears it back to
// false (the ratchet). This check reads that pin (loader.ReadClaimAuthorityPin)
// and compares it against the live manifest's resolved claim_authority: if
// the pin says true (the domain WAS claim_authority:"scenario" at some
// point) but the live value is no longer "scenario" (the resolver removed or
// downgraded the key), this check fires a violation -- the one-way door was
// violated.
//
// HONEST NO-OP CASES (not false positives), mirroring
// checkDisciplineRatchet's own bail conventions exactly:
//   - g.DomainDir == "": no on-disk domain (a synthetic in-memory fixture
//     graph built without loader.LoadGraph) -- honest no-op.
//   - No graph.lock on disk: a domain that has never gone through
//     apply-proposal/land (WriteGraph/WriteLock) has no lock at all -- the
//     ratchet has nothing to check.
//   - Lock exists but ClaimAuthorityScenarioObserved=false: a domain that
//     was never claim_authority:"scenario" (or whose lock predates this
//     field -- additive backward compat) -- no regression possible, honest
//     no-op.
//   - Lock has ClaimAuthorityScenarioObserved=true AND live claim_authority
//     is "scenario": the happy path -- domain is and was
//     claim_authority:"scenario", no regression.
//
// DOMAIN-LEVEL, NOT PER-REQUIREMENT: this check evaluates exactly ONCE per
// all-violations run against g (it asks a question about the manifest's
// claim_authority field, not about any individual requirement), matching the
// shape of checkDisciplineRatchet.
func checkClaimAuthorityRatchet(g *ontology.Graph) []Violation {
	if g.DomainDir == "" {
		// No on-disk domain (synthetic fixture graph) -- honest no-op.
		return nil
	}
	graphPath := graphPathForDomainDir(g.DomainDir)
	pinObserved, lockExists := loader.ReadClaimAuthorityPin(graphPath)
	if !lockExists {
		// No graph.lock on disk -- a domain that has never gone through
		// WriteGraph/WriteLock has no ratchet pin to check. Honest no-op,
		// same convention as checkDisciplineRatchet's absent-lock bail.
		return nil
	}
	if !pinObserved {
		// The domain was never observed as claim_authority:"scenario" -- no
		// ratchet to enforce. Covers both "lock predates this field" and
		// "lock was written for a domain that never opted into
		// claim_authority:scenario".
		return nil
	}
	// pinObserved is true: the domain WAS claim_authority:"scenario" at some
	// point. Check the live manifest now.
	if g.ClaimAuthorityScenario {
		// Happy path: still claim_authority:"scenario" -- no regression.
		return nil
	}
	// Regression detected: the pin says claim_authority:"scenario" was
	// observed, but the live manifest no longer resolves it -- the one-way
	// door was violated.
	return []Violation{{
		Check: "check_claim_authority_ratchet",
		ID:    g.DomainDir,
		Message: fmt.Sprintf(
			"claim_authority regression detected for %s: graph.lock pins claim_authority:\"scenario\" as previously "+
				"observed, but the current manifest.json no longer resolves claim_authority:\"scenario\" -- task #388 "+
				"(W0.1) makes the claim_authority:\"scenario\" flip a ONE-WAY door, symmetric with discipline:\"full\"'s "+
				"own F2 ratchet (check_discipline_ratchet); restore \"claim_authority\": \"scenario\" in manifest.json, "+
				"or if the downgrade is intentional, land it explicitly via `hotam apply-proposal` (which rewrites "+
				"graph.lock and resets the pin)",
			g.DomainDir),
	}}
}

var _ = All.MustRegister("check_claim_authority_ratchet", Invariant{
	Name:  "check_claim_authority_ratchet",
	Canon: methodology.Domain,
	Claim: "once a domain's manifest.json has been observed with claim_authority:\"scenario\", a later manifest that no " +
		"longer resolves claim_authority:\"scenario\" is a regression violation (task #388/W0.1, symmetric with " +
		"check_discipline_ratchet's own F2 ratchet for discipline:\"full\") -- the ratchet pin lives in graph.lock " +
		"(ClaimAuthorityScenarioObserved, written by loader.WriteLock).",
	Rule: "RULE: for the domain graph actually being checked (g.DomainDir), read graph.lock's " +
		"ClaimAuthorityScenarioObserved pin (loader.ReadClaimAuthorityPin). IF the lock does not exist (no graph.lock on " +
		"disk) OR the pin is false (the domain was never observed as claim_authority:\"scenario\", including locks " +
		"written before this field existed), this check is a HONEST NO-OP. OTHERWISE (the pin is true -- the domain WAS " +
		"claim_authority:\"scenario\" at some point, recorded by WriteLock's ratchet), IF g.ClaimAuthorityScenario is " +
		"true (the live manifest still resolves claim_authority:\"scenario\"), no violation (happy path -- domain stayed " +
		"claim_authority:scenario). OTHERWISE (pin is true but live claim_authority is no longer \"scenario\" -- the " +
		"resolver removed or downgraded the manifest key), this check fires ONE violation naming the domain and the " +
		"regression. Domain-level (fires once per all-violations run), matching check_discipline_ratchet's shape.",
	Why: "task #388 (W0.1): claim_authority:\"scenario\" is check_claim_matches_scenario's own opt-in trigger (split " +
		"apart from discipline:\"full\" precisely so a domain's consent to Claim's exact-drift gate is EXPLICIT and " +
		"SEPARATE, not implied by an unrelated discipline flag -- see claim_scenario_current.go's own doc comment). " +
		"Once a domain has made that explicit promise, silently withdrawing it would be the identical class of quiet " +
		"regression check_discipline_ratchet (F2, task W7.2) already closed for discipline:\"full\" itself -- a " +
		"resolver could otherwise flip claim_authority:\"scenario\" on, let check_claim_matches_scenario run clean for " +
		"a while, then quietly remove the key and have that check become an honest no-op again with zero signal. This " +
		"check closes that gap using the SAME lock-file mechanism check_discipline_ratchet already established: " +
		"loader.WriteLock now additionally records ClaimAuthorityScenarioObserved (a one-way ratchet -- once true, " +
		"never false), and this check reads that pin and compares it against the live manifest claim_authority. " +
		"References: check_discipline_ratchet (the established ratchet precedent, internal/invariants/" +
		"discipline_ratchet.go), loader.WriteLock/ReadClaimAuthorityPin (the pin mechanism).",
	Check: checkClaimAuthorityRatchet,
})

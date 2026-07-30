package invariants

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkScenarioAuthorityRatchet is the scenario_authority:"quality" ONE-WAY
// ratchet gate (task #397/W1.5), symmetric with
// checkPublicSurfaceAuthorityRatchet (public_surface_authority_ratchet.go)'s
// own ratchet (itself symmetric with checkClaimAuthorityRatchet's ratchet and
// checkDisciplineRatchet's F2 ratchet, discipline_ratchet.go): once a domain
// has opted into scenario_authority:"quality" (loader.ScenarioAuthorityQuality)
// -- the trigger that turns on check_scenario_quality's real per-requirement
// scenario-quality gate (scenario_quality.go) -- flipping it back to absent
// (or any other value) would be a silent DOWNGRADE of a promise already made
// public in the domain's own manifest, exactly the class of regression
// checkDisciplineRatchet/checkClaimAuthorityRatchet/
// checkPublicSurfaceAuthorityRatchet already exist to catch for their own
// triggers.
//
// This check closes that gap via the SAME graph.lock pin mechanism
// checkPublicSurfaceAuthorityRatchet already uses: loader.WriteLock records
// ScenarioAuthorityQualityObserved=true once it ever observes the live
// manifest resolving scenario_authority:"quality", and NEVER clears it back
// to false (the ratchet). This check reads that pin
// (loader.ReadScenarioAuthorityPin) and compares it against the live
// manifest's resolved scenario_authority: if the pin says true (the domain
// WAS scenario_authority:"quality" at some point) but the live value is no
// longer "quality" (the resolver removed or downgraded the key), this check
// fires a violation -- the one-way door was violated.
//
// HONEST NO-OP CASES (not false positives), mirroring
// checkPublicSurfaceAuthorityRatchet's own bail conventions exactly:
//   - g.DomainDir == "": no on-disk domain (a synthetic in-memory fixture
//     graph built without loader.LoadGraph) -- honest no-op.
//   - No graph.lock on disk: a domain that has never gone through
//     apply-proposal/land (WriteGraph/WriteLock) has no lock at all -- the
//     ratchet has nothing to check.
//   - Lock exists but ScenarioAuthorityQualityObserved=false: a domain that
//     was never scenario_authority:"quality" (or whose lock predates this
//     field -- additive backward compat) -- no regression possible, honest
//     no-op.
//   - Lock has ScenarioAuthorityQualityObserved=true AND live
//     scenario_authority is "quality": the happy path -- domain is and was
//     scenario_authority:"quality", no regression.
//
// DOMAIN-LEVEL, NOT PER-REQUIREMENT: this check evaluates exactly ONCE per
// all-violations run against g (it asks a question about the manifest's
// scenario_authority field, not about any individual requirement), matching
// the shape of checkPublicSurfaceAuthorityRatchet/checkClaimAuthorityRatchet/
// checkDisciplineRatchet.
//
// INDEPENDENT from checkDisciplineRatchet, checkClaimAuthorityRatchet, and
// checkPublicSurfaceAuthorityRatchet: scenario_authority:"quality" is its own
// trigger, pinned in its own lock field, checked against its own live
// manifest value -- a domain can be pinned for discipline:"full" and/or
// claim_authority:"scenario" and/or public_surface_authority:"linked"
// without ever having been scenario_authority:"quality" (and vice versa);
// see scenario_authority_ratchet_test.go's independence tests.
func checkScenarioAuthorityRatchet(g *ontology.Graph) []Violation {
	if g.DomainDir == "" {
		// No on-disk domain (synthetic fixture graph) -- honest no-op.
		return nil
	}
	graphPath := graphPathForDomainDir(g.DomainDir)
	pinObserved, lockExists := loader.ReadScenarioAuthorityPin(graphPath)
	if !lockExists {
		// No graph.lock on disk -- a domain that has never gone through
		// WriteGraph/WriteLock has no ratchet pin to check. Honest no-op,
		// same convention as checkPublicSurfaceAuthorityRatchet's absent-lock
		// bail.
		return nil
	}
	if !pinObserved {
		// The domain was never observed as scenario_authority:"quality" --
		// no ratchet to enforce. Covers both "lock predates this field" and
		// "lock was written for a domain that never opted into
		// scenario_authority:quality".
		return nil
	}
	// pinObserved is true: the domain WAS scenario_authority:"quality" at
	// some point. Check the live manifest now.
	if g.ScenarioAuthorityQuality {
		// Happy path: still scenario_authority:"quality" -- no regression.
		return nil
	}
	// Regression detected: the pin says scenario_authority:"quality" was
	// observed, but the live manifest no longer resolves it -- the one-way
	// door was violated.
	return []Violation{{
		Check: "check_scenario_authority_ratchet",
		ID:    g.DomainDir,
		Message: fmt.Sprintf(
			"scenario_authority regression detected for %s: graph.lock pins scenario_authority:\"quality\" as "+
				"previously observed, but the current manifest.json no longer resolves "+
				"scenario_authority:\"quality\" -- task #397 (W1.5) makes the scenario_authority:\"quality\" flip a "+
				"ONE-WAY door, symmetric with discipline:\"full\"'s own F2 ratchet (check_discipline_ratchet), "+
				"claim_authority:\"scenario\"'s own ratchet (check_claim_authority_ratchet), and "+
				"public_surface_authority:\"linked\"'s own ratchet (check_public_surface_authority_ratchet); restore "+
				"\"scenario_authority\": \"quality\" in manifest.json, or if the downgrade is intentional, land it "+
				"explicitly via `hotam apply-proposal` (which rewrites graph.lock and resets the pin)",
			g.DomainDir),
	}}
}

var _ = All.MustRegister("check_scenario_authority_ratchet", Invariant{
	Name:  "check_scenario_authority_ratchet",
	Canon: methodology.Domain,
	Claim: "once a domain's manifest.json has been observed with scenario_authority:\"quality\", a later manifest " +
		"that no longer resolves scenario_authority:\"quality\" is a regression violation (task #397/W1.5, symmetric " +
		"with check_public_surface_authority_ratchet's own ratchet for public_surface_authority:\"linked\", " +
		"check_claim_authority_ratchet's own ratchet for claim_authority:\"scenario\", and check_discipline_ratchet's " +
		"F2 ratchet for discipline:\"full\") -- the ratchet pin lives in graph.lock " +
		"(ScenarioAuthorityQualityObserved, written by loader.WriteLock).",
	Rule: "RULE: for the domain graph actually being checked (g.DomainDir), read graph.lock's " +
		"ScenarioAuthorityQualityObserved pin (loader.ReadScenarioAuthorityPin). IF the lock does not exist (no " +
		"graph.lock on disk) OR the pin is false (the domain was never observed as scenario_authority:\"quality\", " +
		"including locks written before this field existed), this check is a HONEST NO-OP. OTHERWISE (the pin is " +
		"true -- the domain WAS scenario_authority:\"quality\" at some point, recorded by WriteLock's ratchet), IF " +
		"g.ScenarioAuthorityQuality is true (the live manifest still resolves scenario_authority:\"quality\"), no " +
		"violation (happy path -- domain stayed scenario_authority:quality). OTHERWISE (pin is true but live " +
		"scenario_authority is no longer \"quality\" -- the resolver removed or downgraded the manifest key), this " +
		"check fires ONE violation naming the domain and the regression. Domain-level (fires once per all-violations " +
		"run), matching check_public_surface_authority_ratchet/check_claim_authority_ratchet/" +
		"check_discipline_ratchet's shape. INDEPENDENT of all three of those ratchets -- its own trigger, its own " +
		"lock field, its own live-manifest comparison.",
	Why: "task #397 (W1.5): check_scenario_quality is a BRAND NEW class of duty (every scenario-carrying verified_by " +
		"artifact must have a non-empty title, match the citing requirement's own reqID, carry at least one Then " +
		"step, and -- for a behavioral scenario with a When step -- hold a strict Given-before-When-before-Then " +
		"order), and scenario_authority:\"quality\" is that check's own, separately-declared opt-in trigger -- see " +
		"loader.ScenarioAuthorityQuality's own doc comment for why R-opt-in-trigger-owns-its-own-obligations forbids " +
		"riding in on an already-spent trigger (discipline:\"full\"/claim_authority:\"scenario\"/" +
		"public_surface_authority:\"linked\" were all spent by real or prior consumer domains before this check " +
		"existed). Once a domain has made that explicit promise, silently withdrawing it would be the identical " +
		"class of quiet regression check_discipline_ratchet (F2, task W7.2), check_claim_authority_ratchet (task " +
		"#388/W0.1), and check_public_surface_authority_ratchet (task #396/W1.4) already closed for their own " +
		"triggers -- a resolver could otherwise flip scenario_authority:\"quality\" on, let check_scenario_quality " +
		"run clean for a while, then quietly remove the key and have that check become an honest no-op again with " +
		"zero signal. This check closes that gap using the SAME lock-file mechanism " +
		"check_public_surface_authority_ratchet already established: loader.WriteLock now additionally records " +
		"ScenarioAuthorityQualityObserved (a one-way ratchet -- once true, never false), and this check reads that " +
		"pin and compares it against the live manifest scenario_authority. References: " +
		"check_public_surface_authority_ratchet (the immediate structural precedent, internal/invariants/" +
		"public_surface_authority_ratchet.go), check_claim_authority_ratchet and check_discipline_ratchet (the " +
		"earlier precedents), loader.WriteLock/ReadScenarioAuthorityPin (the pin mechanism), " +
		"R-opt-in-trigger-owns-its-own-obligations (the general law this check's own trigger independence " +
		"instantiates).",
	Check: checkScenarioAuthorityRatchet,
})

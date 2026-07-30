package invariants

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkPublicSurfaceAuthorityRatchet is the public_surface_authority:"linked"
// ONE-WAY ratchet gate (task #396/W1.4), symmetric with
// checkClaimAuthorityRatchet (claim_authority_ratchet.go)'s own ratchet
// (itself symmetric with checkDisciplineRatchet's F2 ratchet,
// discipline_ratchet.go): once a domain has opted into
// public_surface_authority:"linked" (loader.PublicSurfaceAuthorityLinked) --
// the trigger that turns on check_public_surface_linked_or_marked's real
// per-symbol completeness gate (model_complete_symmetric.go) -- flipping it
// back to absent (or any other value) would be a silent DOWNGRADE of a
// promise already made public in the domain's own manifest, exactly the
// class of regression checkDisciplineRatchet/checkClaimAuthorityRatchet
// already exist to catch for their own triggers.
//
// This check closes that gap via the SAME graph.lock pin mechanism
// checkClaimAuthorityRatchet already uses: loader.WriteLock records
// PublicSurfaceAuthorityLinkedObserved=true once it ever observes the live
// manifest resolving public_surface_authority:"linked", and NEVER clears it
// back to false (the ratchet). This check reads that pin
// (loader.ReadPublicSurfaceAuthorityPin) and compares it against the live
// manifest's resolved public_surface_authority: if the pin says true (the
// domain WAS public_surface_authority:"linked" at some point) but the live
// value is no longer "linked" (the resolver removed or downgraded the key),
// this check fires a violation -- the one-way door was violated.
//
// HONEST NO-OP CASES (not false positives), mirroring
// checkClaimAuthorityRatchet's own bail conventions exactly:
//   - g.DomainDir == "": no on-disk domain (a synthetic in-memory fixture
//     graph built without loader.LoadGraph) -- honest no-op.
//   - No graph.lock on disk: a domain that has never gone through
//     apply-proposal/land (WriteGraph/WriteLock) has no lock at all -- the
//     ratchet has nothing to check.
//   - Lock exists but PublicSurfaceAuthorityLinkedObserved=false: a domain
//     that was never public_surface_authority:"linked" (or whose lock
//     predates this field -- additive backward compat) -- no regression
//     possible, honest no-op.
//   - Lock has PublicSurfaceAuthorityLinkedObserved=true AND live
//     public_surface_authority is "linked": the happy path -- domain is and
//     was public_surface_authority:"linked", no regression.
//
// DOMAIN-LEVEL, NOT PER-SYMBOL: this check evaluates exactly ONCE per
// all-violations run against g (it asks a question about the manifest's
// public_surface_authority field, not about any individual symbol), matching
// the shape of checkClaimAuthorityRatchet/checkDisciplineRatchet.
//
// INDEPENDENT from both checkDisciplineRatchet and checkClaimAuthorityRatchet:
// public_surface_authority:"linked" is its own trigger, pinned in its own
// lock field, checked against its own live manifest value -- a domain can be
// pinned for discipline:"full" and/or claim_authority:"scenario" without ever
// having been public_surface_authority:"linked" (and vice versa); see
// public_surface_authority_ratchet_test.go's independence tests.
func checkPublicSurfaceAuthorityRatchet(g *ontology.Graph) []Violation {
	if g.DomainDir == "" {
		// No on-disk domain (synthetic fixture graph) -- honest no-op.
		return nil
	}
	graphPath := graphPathForDomainDir(g.DomainDir)
	pinObserved, lockExists := loader.ReadPublicSurfaceAuthorityPin(graphPath)
	if !lockExists {
		// No graph.lock on disk -- a domain that has never gone through
		// WriteGraph/WriteLock has no ratchet pin to check. Honest no-op,
		// same convention as checkClaimAuthorityRatchet's absent-lock bail.
		return nil
	}
	if !pinObserved {
		// The domain was never observed as public_surface_authority:"linked"
		// -- no ratchet to enforce. Covers both "lock predates this field"
		// and "lock was written for a domain that never opted into
		// public_surface_authority:linked".
		return nil
	}
	// pinObserved is true: the domain WAS public_surface_authority:"linked"
	// at some point. Check the live manifest now.
	if g.PublicSurfaceAuthorityLinked {
		// Happy path: still public_surface_authority:"linked" -- no regression.
		return nil
	}
	// Regression detected: the pin says public_surface_authority:"linked" was
	// observed, but the live manifest no longer resolves it -- the one-way
	// door was violated.
	return []Violation{{
		Check: "check_public_surface_authority_ratchet",
		ID:    g.DomainDir,
		Message: fmt.Sprintf(
			"public_surface_authority regression detected for %s: graph.lock pins public_surface_authority:\"linked\" "+
				"as previously observed, but the current manifest.json no longer resolves "+
				"public_surface_authority:\"linked\" -- task #396 (W1.4) makes the public_surface_authority:\"linked\" "+
				"flip a ONE-WAY door, symmetric with discipline:\"full\"'s own F2 ratchet (check_discipline_ratchet) and "+
				"claim_authority:\"scenario\"'s own ratchet (check_claim_authority_ratchet); restore "+
				"\"public_surface_authority\": \"linked\" in manifest.json, or if the downgrade is intentional, land it "+
				"explicitly via `hotam apply-proposal` (which rewrites graph.lock and resets the pin)",
			g.DomainDir),
	}}
}

var _ = All.MustRegister("check_public_surface_authority_ratchet", Invariant{
	Name:  "check_public_surface_authority_ratchet",
	Canon: methodology.Domain,
	Claim: "once a domain's manifest.json has been observed with public_surface_authority:\"linked\", a later manifest " +
		"that no longer resolves public_surface_authority:\"linked\" is a regression violation (task #396/W1.4, " +
		"symmetric with check_claim_authority_ratchet's own ratchet for claim_authority:\"scenario\" and " +
		"check_discipline_ratchet's F2 ratchet for discipline:\"full\") -- the ratchet pin lives in graph.lock " +
		"(PublicSurfaceAuthorityLinkedObserved, written by loader.WriteLock).",
	Rule: "RULE: for the domain graph actually being checked (g.DomainDir), read graph.lock's " +
		"PublicSurfaceAuthorityLinkedObserved pin (loader.ReadPublicSurfaceAuthorityPin). IF the lock does not exist " +
		"(no graph.lock on disk) OR the pin is false (the domain was never observed as " +
		"public_surface_authority:\"linked\", including locks written before this field existed), this check is a " +
		"HONEST NO-OP. OTHERWISE (the pin is true -- the domain WAS public_surface_authority:\"linked\" at some point, " +
		"recorded by WriteLock's ratchet), IF g.PublicSurfaceAuthorityLinked is true (the live manifest still resolves " +
		"public_surface_authority:\"linked\"), no violation (happy path -- domain stayed " +
		"public_surface_authority:linked). OTHERWISE (pin is true but live public_surface_authority is no longer " +
		"\"linked\" -- the resolver removed or downgraded the manifest key), this check fires ONE violation naming the " +
		"domain and the regression. Domain-level (fires once per all-violations run), matching " +
		"check_claim_authority_ratchet/check_discipline_ratchet's shape. INDEPENDENT of both of those ratchets -- its " +
		"own trigger, its own lock field, its own live-manifest comparison.",
	Why: "task #396 (W1.4): public_surface_authority:\"linked\" is check_public_surface_linked_or_marked's own opt-in " +
		"trigger (a BRAND NEW, wholly independent trigger, deliberately not co-gated with discipline:\"full\" -- see " +
		"loader.PublicSurfaceAuthorityLinked's own doc comment for why R-opt-in-trigger-owns-its-own-obligations " +
		"forbids riding in on the already-spent discipline:\"full\" trigger prat/gpsm-sm consented to before this check " +
		"existed). Once a domain has made that explicit promise, silently withdrawing it would be the identical class " +
		"of quiet regression check_discipline_ratchet (F2, task W7.2) and check_claim_authority_ratchet (task #388/" +
		"W0.1) already closed for their own triggers -- a resolver could otherwise flip " +
		"public_surface_authority:\"linked\" on, let check_public_surface_linked_or_marked run clean for a while, then " +
		"quietly remove the key and have that check become an honest no-op again with zero signal. This check closes " +
		"that gap using the SAME lock-file mechanism check_claim_authority_ratchet already established: " +
		"loader.WriteLock now additionally records PublicSurfaceAuthorityLinkedObserved (a one-way ratchet -- once " +
		"true, never false), and this check reads that pin and compares it against the live manifest " +
		"public_surface_authority. References: check_claim_authority_ratchet (the immediate structural precedent, " +
		"internal/invariants/claim_authority_ratchet.go), check_discipline_ratchet (the original F2 ratchet, " +
		"internal/invariants/discipline_ratchet.go), loader.WriteLock/ReadPublicSurfaceAuthorityPin (the pin " +
		"mechanism), R-opt-in-trigger-owns-its-own-obligations (the general law this check's own trigger independence " +
		"instantiates).",
	Check: checkPublicSurfaceAuthorityRatchet,
})

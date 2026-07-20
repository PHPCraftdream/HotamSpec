package invariants

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkDisciplineRatchet is the discipline:"full" ONE-WAY ratchet gate (PLAN-
// scenario-generated-spec.md §2 D4, task W7.2, @fx finding F2). D4 specifies
// that flipping a domain's manifest.json from discipline:"" to discipline:"full"
// is a ONE-WAY door -- once a domain has migrated its SETTLED requirements to
// carry real coverage and declared discipline:"full", silently flipping it
// back to "" would be a silent DOWNGRADE of a promise already made public.
// Before F2, this one-way property was purely a DOCUMENTED convention (see
// loader.ResolveDiscipline's own doc comment: "This engine version does NOT
// yet mechanically enforce the one-way property for manifest.json") -- a
// steward could delete or change the discipline key in manifest.json and
// every discipline-gated check (check_settled_requires_scenario,
// check_model_complete) became an honest no-op again, with zero violations
// from all-violations.
//
// This check closes that gap via the graph.lock discipline pin: WriteLock
// (internal/loader/lock.go) records DisciplineFullObserved=true once it ever
// observes the live manifest resolving discipline:"full", and NEVER clears it
// back to false (the ratchet). This check reads that pin and compares it
// against the live manifest's resolved discipline: if the pin says true (the
// domain WAS discipline:full at some point) but the live discipline is no
// longer "full" (the steward removed or downgraded the key), this check fires
// a violation -- the one-way door was violated.
//
// HONEST NO-OP CASES (not false positives), mirroring
// check_graph_lock_pins_graph_json's own bail conventions exactly:
//   - g.DomainDir == "": no on-disk domain (a synthetic in-memory fixture
//     graph built without loader.LoadGraph) -- honest no-op.
//   - No graph.lock on disk: a domain that has never gone through
//     apply-proposal/land (WriteGraph/WriteLock) has no lock at all -- the
//     ratchet has nothing to check, same shape as check_graph_lock_pins_graph_json's
//     own absent-lock bail.
//   - Lock exists but DisciplineFullObserved=false: a domain that was never
//     discipline:full (or whose lock predates the F2 field -- additive
//     backward compat) -- no regression possible, honest no-op.
//   - Lock has DisciplineFullObserved=true AND live discipline is "full": the
//     happy path -- domain is and was discipline:full, no regression. This is
//     the important non-regression case: a domain that stays discipline:full
//     throughout must NOT falsely violate.
//
// DOMAIN-LEVEL, NOT PER-REQUIREMENT: this check evaluates exactly ONCE per
// all-violations run against g (it asks a question about the manifest's
// discipline field, not about any individual requirement), matching the shape
// of check_graph_lock_pins_graph_json and check_project_parent_declared.
func checkDisciplineRatchet(g *ontology.Graph) []Violation {
	if g.DomainDir == "" {
		// No on-disk domain (synthetic fixture graph) -- honest no-op.
		return nil
	}
	graphPath := graphPathForDomainDir(g.DomainDir)
	pinObserved, lockExists := loader.ReadDisciplinePin(graphPath)
	if !lockExists {
		// No graph.lock on disk -- a domain that has never gone through
		// WriteGraph/WriteLock has no ratchet pin to check. Honest no-op,
		// same convention as check_graph_lock_pins_graph_json's absent-lock
		// bail: a freshly scaffolded/copied fixture is not yet under lock
		// discipline at all.
		return nil
	}
	if !pinObserved {
		// The domain was never observed as discipline:"full" -- no ratchet
		// to enforce. Covers both "lock predates F2" (additive backward
		// compat: field absent, decodes as false) and "lock was written for
		// a domain that never opted into discipline:full".
		return nil
	}
	// pinObserved is true: the domain WAS discipline:"full" at some point.
	// Check the live manifest now.
	if g.Discipline == loader.DisciplineFull {
		// Happy path: still discipline:"full" -- no regression. This is the
		// important non-regression case: a domain that stays discipline:full
		// throughout must NOT falsely violate.
		return nil
	}
	// Regression detected: the pin says discipline:"full" was observed, but
	// the live manifest no longer resolves discipline:"full" -- the one-way
	// door was violated.
	return []Violation{{
		Check: "check_discipline_ratchet",
		ID:    g.DomainDir,
		Message: fmt.Sprintf(
			"discipline regression detected for %s: graph.lock pins discipline:\"full\" as previously observed, "+
				"but the current manifest.json no longer resolves discipline:\"full\" (current value: %q) -- "+
				"PLAN-scenario-generated-spec.md §2 D4 makes the discipline:\"full\" flip a ONE-WAY door; "+
				"restore \"discipline\": \"full\" in manifest.json, or if the downgrade is intentional, "+
				"land it explicitly via `hotam apply-proposal` (which rewrites graph.lock and resets the pin)",
			g.DomainDir, g.Discipline),
	}}
}

var _ = All.MustRegister("check_discipline_ratchet", Invariant{
	Name:  "check_discipline_ratchet",
	Canon: methodology.Domain,
	Claim: "once a domain's manifest.json has been observed with discipline:\"full\", a later manifest that no longer " +
		"resolves discipline:\"full\" is a regression violation (PLAN-scenario-generated-spec.md §2 D4: the discipline:\"full\" " +
		"flip is a one-way door) -- the ratchet pin lives in graph.lock (DisciplineFullObserved, written by loader.WriteLock).",
	Rule: "RULE: for the domain graph actually being checked (g.DomainDir), read graph.lock's DisciplineFullObserved pin " +
		"(loader.ReadDisciplinePin). IF the lock does not exist (no graph.lock on disk -- a domain that has never gone " +
		"through WriteGraph/WriteLock, same absent-lock bail as check_graph_lock_pins_graph_json) OR the pin is false " +
		"(the domain was never observed as discipline:\"full\", including locks written before the F2 additive field " +
		"existed), this check is a HONEST NO-OP. OTHERWISE (the pin is true -- the domain WAS discipline:\"full\" at " +
		"some point, recorded by WriteLock's ratchet), IF g.Discipline == loader.DisciplineFull (the live manifest still " +
		"resolves discipline:\"full\"), no violation (happy path -- domain stayed discipline:full). OTHERWISE (pin is true " +
		"but live discipline is no longer \"full\" -- the steward removed or downgraded the manifest key), this check " +
		"fires ONE violation naming the domain and the regression. Domain-level (fires once per all-violations run), " +
		"matching check_graph_lock_pins_graph_json's shape.",
	Why: "PLAN-scenario-generated-spec.md §2 D4 (@fh/@fx finding F2, task W7.2): before this check, the one-way-door " +
		"property of discipline:\"full\" was purely a DOCUMENTED convention -- loader.ResolveDiscipline's own doc comment " +
		"explicitly stated 'This engine version does NOT yet mechanically enforce the one-way property for manifest.json " +
		"(manifest.json, unlike graph.json, has no graph.lock-style content pin today)'. A steward could silently delete " +
		"or change the discipline key in manifest.json, and every discipline-gated check (check_settled_requires_scenario, " +
		"check_model_complete, and the F1 gate in check_scenario_executes_impl) became an honest no-op again -- zero " +
		"violations, silent regression of a public promise. This check closes that gap using the SAME lock-file mechanism " +
		"check_graph_lock_pins_graph_json already established for graph.json's own content pin: loader.WriteLock now " +
		"additionally records DisciplineFullObserved (a one-way ratchet -- once true, never false), and this check reads " +
		"that pin and compares it against the live manifest discipline. The mechanism is deliberately MINIMAL (no " +
		"persistent history, no audit trail of WHEN discipline was first observed -- just a single boolean pin, matching " +
		"the established one-hash-one-lock convention); a fuller mechanism would track the full manifest content the way " +
		"a future schema migration might, but the one-bit ratchet is the minimum that closes F2's silent-regression gap. " +
		"References: PLAN-scenario-generated-spec.md §2 D4, loader.WriteLock/ReadDisciplinePin (the pin mechanism), " +
		"check_graph_lock_pins_graph_json (the established lock-check precedent).",
	Check: checkDisciplineRatchet,
})

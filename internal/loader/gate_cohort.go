package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// GateCohortSpec declares, for a domain that opts in, WHICH Requirements form
// the denominator a "gate_signoff_count" orientation_faq assert (see
// OrientationFAQAssert's own doc comment) must answer "have ALL of these
// passed this stage" against — closing a real gap the pre-existing
// total=Signed+Deferred denominator cannot see: a Requirement that was NEVER
// EVALUATED at a stage (no GateSignoff record at all, neither SIGNED nor
// DEFERRED) is invisible to that sum, so `expect:"all"` can silently pass
// even though some cohort member was never assessed.
//
// Statuses names which Requirement.Status values count as cohort members
// (exact match against ontology.StatusDRAFT/StatusSETTLED/StatusREJECTED, or
// prefix match via ontology.StatusOPENPrefix — the SAME status vocabulary
// and matching rule graphfacts.RequirementStatusTally already establishes;
// this field intentionally reuses that rule rather than inventing a new one).
// An empty/absent Statuses defaults to []string{"SETTLED"} — the common case
// for a staged-gate methodology (prat/gpsm-sm's P-G0..P-G4: only SETTLED
// requirements are IN the pipeline; DRAFT is pre-cohort and REJECTED is
// explicitly out-of-cohort by design, exactly the shape gpsm-sm's own 35
// total / 32 in-cohort split reflects).
//
// Exclude names specific Requirement ids to drop from the cohort even though
// their Status matches Statuses — an escape hatch for a legitimately
// out-of-cohort requirement whose status alone does not distinguish it (e.g.
// a SETTLED meta-requirement describing the domain/pipeline itself rather
// than a requirement that passes THROUGH the gated pipeline). Every id named
// here MUST resolve to a real Requirement in the graph — an id that does not
// (a typo, a since-renamed/removed requirement) is a FAIL-CLOSED violation,
// diagnosed at CHECK time (internal/invariants/orientation_faq_assert.go),
// never silently ignored by the loader — mirroring this file's own
// established loader/check split (see ResolveGateStageOrder's doc comment:
// the loader stays lenient, the invariant layer diagnoses).
type GateCohortSpec struct {
	Statuses []string `json:"statuses"`
	Exclude  []string `json:"exclude"`
}

// ResolveGateCohort reads the optional "gate_cohort" field from the
// manifest.json sitting next to the given graph.json path, mirroring
// ResolveGateStageOrder's exact pattern (read manifest, tolerate a missing
// file, tolerate malformed JSON, default when absent). Returns nil (the
// HONEST NO-OP — the same shape every sibling opt-in resolver already
// establishes) for every absent/missing-field/malformed-JSON case —
// preserving 100% backward compatibility with every manifest.json in this
// repo and in the wild that predates the gate_cohort field: a
// "gate_signoff_count" assert falls back to today's total=Signed+Deferred
// denominator, byte-identical, when no cohort is declared.
//
// When the field IS present but its Statuses list is empty, this resolver
// defaults it to []string{"SETTLED"} (see GateCohortSpec's own doc comment
// for why SETTLED is the sane default) — the ONE piece of default-filling
// this resolver performs; every other shape (a non-empty Statuses, an
// Exclude list of any length, including entries that will turn out to be
// invalid ids or invalid status names) is passed through UNVALIDATED. Id and
// status validation is deliberately NOT this loader's job — per this file's
// established loader/check split, a malformed cohort declaration is
// diagnosed as a violation at CHECK time
// (internal/invariants/orientation_faq_assert.go), not silently dropped or
// corrected here.
func ResolveGateCohort(graphPath string) *GateCohortSpec {
	manifestPath := filepath.Join(filepath.Dir(graphPath), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// manifest.json absent — honest no-op, mirroring every sibling
		// resolver's missing-manifest default.
		return nil
	}
	var raw struct {
		GateCohort *GateCohortSpec `json:"gate_cohort"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		// manifest.json exists but is malformed JSON — honest no-op,
		// mirroring ResolveGateStageOrder's identical malformed-manifest
		// default.
		return nil
	}
	if raw.GateCohort == nil {
		// Field absent — honest no-op, the backward-compat path every
		// manifest.json predating gate_cohort takes.
		return nil
	}
	if len(raw.GateCohort.Statuses) == 0 {
		raw.GateCohort.Statuses = []string{"SETTLED"}
	}
	return raw.GateCohort
}

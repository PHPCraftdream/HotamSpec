package proposal

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestSimulateRequirementResult_Create proves the CREATE path: a brand-new
// requirement id yields the simulated result carrying exactly the fields the
// proposal supplied (mirroring mutate()'s CREATE branch), and the source
// graph g is left completely untouched (no disk I/O, no in-place mutation).
func TestSimulateRequirementResult_Create(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	before := len(g.Requirements)

	p := ProposedRequirement{
		ID:             "R-sim-create",
		Claim:          "a new requirement",
		Owner:          "sa",
		Status:         ontology.StatusSETTLED,
		SourceRefs:     []string{"https://example.com/doc"},
		LastReviewedAt: today,
		ReviewAfter:    "2027-01-01",
		Evidence:       []string{"resolver sign-off"},
	}
	result, err := SimulateRequirementResult(g, today, p)
	if err != nil {
		t.Fatalf("SimulateRequirementResult: %v", err)
	}
	if result.ID != "R-sim-create" {
		t.Errorf("result.ID = %q, want R-sim-create", result.ID)
	}
	if len(result.SourceRefs) != 1 || result.SourceRefs[0] != "https://example.com/doc" {
		t.Errorf("result.SourceRefs = %v, want [https://example.com/doc]", result.SourceRefs)
	}
	if result.LastReviewedAt != today {
		t.Errorf("result.LastReviewedAt = %q, want %q", result.LastReviewedAt, today)
	}

	// g itself must be untouched: no new requirement, same length.
	if len(g.Requirements) != before {
		t.Errorf("source graph g was mutated by SimulateRequirementResult: len(Requirements) = %d, want %d", len(g.Requirements), before)
	}
	if _, ok := findReq(g, "R-sim-create"); ok {
		t.Error("R-sim-create leaked into the source graph g — SimulateRequirementResult must not mutate its input")
	}
}

// TestSimulateRequirementResult_UpdatePreservesOmittedProvenance is the
// CREATE-vs-UPDATE correctness proof cited by provenanceGate's design: an
// UPDATE proposal that omits source_refs/evidence/last_reviewed_at/
// review_after (relying on coalesce-preserve semantics) must simulate a
// result that STILL carries the provenance already on the existing node —
// proving the simulation reflects the POST-MERGE state, not the raw
// (empty) proposal fields.
func TestSimulateRequirementResult_UpdatePreservesOmittedProvenance(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	g.Requirements = append(g.Requirements, ontology.Requirement{
		ID:             "R-sim-update",
		Claim:          "original claim",
		Owner:          "sa",
		Status:         ontology.StatusSETTLED,
		Enforcement:    ontology.EnforcementPROSE,
		Enforceability: ontology.EnforceabilityENFORCEABLE,
		SourceRefs:     []string{"https://example.com/original"},
		LastReviewedAt: "2026-01-01",
		ReviewAfter:    "2027-01-01",
		Evidence:       []string{"original evidence"},
	})

	// UPDATE that only changes claim; provenance fields are omitted.
	p := ProposedRequirement{
		ID:     "R-sim-update",
		Claim:  "updated claim only",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
	}
	result, err := SimulateRequirementResult(g, today, p)
	if err != nil {
		t.Fatalf("SimulateRequirementResult: %v", err)
	}
	if result.Claim != "updated claim only" {
		t.Errorf("result.Claim = %q, want the updated claim", result.Claim)
	}
	if len(result.SourceRefs) != 1 || result.SourceRefs[0] != "https://example.com/original" {
		t.Errorf("result.SourceRefs = %v, want preserved [https://example.com/original]", result.SourceRefs)
	}
	if result.LastReviewedAt != "2026-01-01" {
		t.Errorf("result.LastReviewedAt = %q, want preserved 2026-01-01", result.LastReviewedAt)
	}
	if result.ReviewAfter != "2027-01-01" {
		t.Errorf("result.ReviewAfter = %q, want preserved 2027-01-01", result.ReviewAfter)
	}

	// The source graph's existing requirement must be untouched.
	orig, ok := findReq(g, "R-sim-update")
	if !ok {
		t.Fatal("R-sim-update missing from source graph g")
	}
	if orig.Claim != "original claim" {
		t.Errorf("source graph g was mutated: Claim = %q, want unchanged %q", orig.Claim, "original claim")
	}
}

// TestSimulateRequirementResult_NotFoundAfterMutateError proves a mutate()
// error (e.g. the blocked_on clear-sentinel-on-CREATE guard) is surfaced by
// SimulateRequirementResult rather than silently returning a zero value.
func TestSimulateRequirementResult_MutateError(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	p := ProposedRequirement{
		ID:        "R-sim-bad-create",
		Claim:     "bad create",
		Owner:     "sa",
		Status:    ontology.StatusDRAFT,
		BlockedOn: clearSentinel,
	}
	if _, err := SimulateRequirementResult(g, today, p); err == nil {
		t.Fatal("expected an error for a CREATE proposal using the blocked_on clear-sentinel")
	}
}

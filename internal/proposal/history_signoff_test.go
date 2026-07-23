package proposal

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// --- ProposedRequirement.Signoff ---

// TestApply_Requirement_UpdateWithSignoff_LandsIntoHistory proves a
// Requirement UPDATE carrying a Signoff lands a HistoryEntry with both
// DecidedBy AND Signoff populated (task #335, R4F-req-signoff) -- closing
// the gap where task #328's landed UPDATEs recorded real human approval
// only as free text with DecidedBy left "".
func TestApply_Requirement_UpdateWithSignoff_LandsIntoHistory(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "revised claim for R-1 with a real human decision",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
		Signoff: &ontology.Signoff{
			DecidedBy: "outsider",
			Verbatim:  "yes, land this as-is",
		},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	r, ok := findReq(g, "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if len(r.History) != 1 {
		t.Fatalf("History len = %d, want 1", len(r.History))
	}
	h := r.History[0]
	if h.DecidedBy != "outsider" {
		t.Errorf("History[0].DecidedBy = %q, want %q", h.DecidedBy, "outsider")
	}
	if h.Signoff == nil {
		t.Fatalf("History[0].Signoff = nil, want populated")
	}
	if h.Signoff.DecidedBy != "outsider" {
		t.Errorf("History[0].Signoff.DecidedBy = %q, want %q", h.Signoff.DecidedBy, "outsider")
	}
	if h.Signoff.Verbatim != "yes, land this as-is" {
		t.Errorf("History[0].Signoff.Verbatim = %q, want the recorded verbatim", h.Signoff.Verbatim)
	}
	if h.Signoff.Date != today {
		t.Errorf("History[0].Signoff.Date = %q, want defaulted to today %q", h.Signoff.Date, today)
	}
	if h.Signoff.Instrument != ontology.SignoffInstrumentPersonal {
		t.Errorf("History[0].Signoff.Instrument = %q, want default %q", h.Signoff.Instrument, ontology.SignoffInstrumentPersonal)
	}
}

// TestApply_Requirement_CreateWithSignoffFails proves CREATE-time signoff is
// rejected: a typed signoff records a decision about an EXISTING
// requirement's UPDATE, and CREATE-time signoff is explicitly out of scope
// for task #335.
func TestApply_Requirement_CreateWithSignoffFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:     "R-brand-new",
		Claim:  "a brand new claim",
		Owner:  "sa",
		Status: ontology.StatusDRAFT,
		Signoff: &ontology.Signoff{
			DecidedBy: "outsider",
			Verbatim:  "approved",
		},
	}
	assertApplyFails(t, path, p, "CREATE")
}

// TestApply_Requirement_UpdateWithoutSignoff_UnchangedBehavior is the
// regression-proof for "when Signoff is nil, behavior is byte-identical to
// pre-#335": an UPDATE proposal with no field changes and no Signoff must
// still append NO History entry at all (summarizeFieldDiff empty -> no
// append), exactly as before this task.
func TestApply_Requirement_UpdateWithoutSignoff_UnchangedBehavior(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	// Resend R-1's exact existing field values -- a genuine no-op UPDATE.
	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "claim R-1",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	r, ok := findReq(g, "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if len(r.History) != 0 {
		t.Fatalf("History len = %d, want 0 -- a no-op UPDATE with no Signoff must append nothing (pre-#335 behavior unchanged)", len(r.History))
	}
}

// TestApply_Requirement_UpdateWithSignoff_UnconditionalAppendEvenWithNoFieldChanges
// proves the unconditional-append discipline: a Signoff-bearing UPDATE with
// an otherwise no-op field diff (resending R-1's exact existing values)
// STILL appends a HistoryEntry -- a signoff-bearing update can never
// silently skip its own audit trail.
func TestApply_Requirement_UpdateWithSignoff_UnconditionalAppendEvenWithNoFieldChanges(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "claim R-1",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
		Signoff: &ontology.Signoff{
			DecidedBy: "outsider",
			Verbatim:  "re-affirmed, no changes needed",
		},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	r, ok := findReq(g, "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if len(r.History) != 1 {
		t.Fatalf("History len = %d, want 1 -- a signoff-bearing update must ALWAYS append, even with no field diff", len(r.History))
	}
	if !strings.Contains(r.History[0].Summary, "no field changes") {
		t.Errorf("History[0].Summary = %q, want the no-field-changes fallback summary", r.History[0].Summary)
	}
	if r.History[0].Signoff == nil || r.History[0].Signoff.DecidedBy != "outsider" {
		t.Errorf("History[0].Signoff = %+v, want populated with decided_by=outsider", r.History[0].Signoff)
	}
}

// TestApply_Requirement_UpdateWithUnresolvableSignoffDecidedByFails proves
// an unknown decided_by is rejected at mutate time, UNGATED (unlike
// ProposedConflict.mutate's zero-Stakeholder escape hatch) -- this rejection
// must fire even though baseGraph() DOES declare Stakeholders (proving the
// lookup itself works); the "ungated" property is that it would ALSO fire
// on a graph with zero declared Stakeholders, which is covered by the sibling
// test below.
func TestApply_Requirement_UpdateWithUnresolvableSignoffDecidedByFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "claim R-1",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
		Signoff: &ontology.Signoff{
			DecidedBy: "nobody-known",
			Verbatim:  "approved",
		},
	}
	assertApplyFails(t, path, p, "not a known Stakeholder")
}

// TestApply_Requirement_UpdateWithSignoff_UngatedOnZeroStakeholders proves
// the Stakeholder-resolution check for a History signoff is UNGATED: unlike
// ProposedConflict.mutate's `if len(g.Stakeholders) > 0` escape hatch, a
// signoff decided_by must resolve even when the domain has declared ZERO
// Stakeholders -- a typed History signoff is per-proposal opt-in, so a
// domain choosing to use it must have named its humans.
func TestApply_Requirement_UpdateWithSignoff_UngatedOnZeroStakeholders(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	g.Stakeholders = nil
	path := writeTempGraph(t, g)
	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "claim R-1",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
		Signoff: &ontology.Signoff{
			DecidedBy: "outsider",
			Verbatim:  "approved",
		},
	}
	assertApplyFails(t, path, p, "not a known Stakeholder")
}

// TestApply_Requirement_UpdateWithSignoffChosenVariantFails proves
// validate() rejects a non-empty chosen_variant on a Requirement Signoff --
// chosen_variant is a Conflict-variant-only concept, unauditable junk here.
func TestApply_Requirement_UpdateWithSignoffChosenVariantFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "claim R-1",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
		Signoff: &ontology.Signoff{
			DecidedBy:     "outsider",
			Verbatim:      "approved",
			ChosenVariant: "variant-a",
		},
	}
	assertApplyFails(t, path, p, "chosen_variant")
}

// TestApply_Requirement_UpdateWithSignoffMissingDecidedByFails and
// TestApply_Requirement_UpdateWithSignoffMissingVerbatimFails cover
// validateHistorySignoffShape's two required-field checks.

func TestApply_Requirement_UpdateWithSignoffMissingDecidedByFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:      "R-1",
		Claim:   "claim R-1",
		Owner:   "sa",
		Status:  ontology.StatusSETTLED,
		Signoff: &ontology.Signoff{Verbatim: "approved"},
	}
	assertApplyFails(t, path, p, "decided_by")
}

func TestApply_Requirement_UpdateWithSignoffMissingVerbatimFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:      "R-1",
		Claim:   "claim R-1",
		Owner:   "sa",
		Status:  ontology.StatusSETTLED,
		Signoff: &ontology.Signoff{DecidedBy: "outsider"},
	}
	assertApplyFails(t, path, p, "verbatim")
}

// --- ProposedAssumptionRewrite.Signoff ---

// TestApply_AssumptionRewrite_WithSignoff_EnrichesUnconditionalHistoryEntry
// proves a rewrite's ALREADY-unconditional HistoryEntry (task #306) gets
// enriched with DecidedBy/Signoff when Signoff is present, with no change
// to the unconditional-append behavior itself.
func TestApply_AssumptionRewrite_WithSignoff_EnrichesUnconditionalHistoryEntry(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumptionRewrite{
		AssumptionID: "A-base",
		NewStatement: "the substrate is stable under normal load, confirmed by a human review",
		Reason:       "clarified after resolver review",
		Signoff: &ontology.Signoff{
			DecidedBy: "sa",
			Verbatim:  "confirmed, this rewrite is accurate",
		},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	var a ontology.Assumption
	found := false
	for _, x := range g.Assumptions {
		if x.ID == "A-base" {
			a = x
			found = true
		}
	}
	if !found {
		t.Fatalf("assumption A-base missing")
	}
	if len(a.History) != 1 {
		t.Fatalf("History len = %d, want 1", len(a.History))
	}
	h := a.History[0]
	if h.DecidedBy != "sa" {
		t.Errorf("History[0].DecidedBy = %q, want %q", h.DecidedBy, "sa")
	}
	if h.Signoff == nil || h.Signoff.Verbatim != "confirmed, this rewrite is accurate" {
		t.Errorf("History[0].Signoff = %+v, want populated with the recorded verbatim", h.Signoff)
	}
	if !strings.Contains(h.Summary, "clarified after resolver review") {
		t.Errorf("History[0].Summary = %q, want it to still contain the Reason text", h.Summary)
	}
}

// TestApply_AssumptionRewrite_ReasonStillRequiredWithSignoff proves Reason
// stays required even when Signoff is also present -- signoff supplements
// Reason, never replaces it.
func TestApply_AssumptionRewrite_ReasonStillRequiredWithSignoff(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumptionRewrite{
		AssumptionID: "A-base",
		NewStatement: "a rewrite with signoff but no reason",
		Signoff: &ontology.Signoff{
			DecidedBy: "sa",
			Verbatim:  "approved",
		},
	}
	assertApplyFails(t, path, p, "reason")
}

// TestApply_AssumptionRewrite_UnresolvableSignoffDecidedByFails mirrors the
// Requirement-side Stakeholder-resolution rejection for AssumptionRewrite.
func TestApply_AssumptionRewrite_UnresolvableSignoffDecidedByFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumptionRewrite{
		AssumptionID: "A-base",
		NewStatement: "a rewrite with an unresolvable signoff",
		Reason:       "testing",
		Signoff: &ontology.Signoff{
			DecidedBy: "nobody-known",
			Verbatim:  "approved",
		},
	}
	assertApplyFails(t, path, p, "not a known Stakeholder")
}

// TestApply_AssumptionRewrite_SignoffChosenVariantFails mirrors the
// Requirement-side chosen_variant rejection for AssumptionRewrite.
func TestApply_AssumptionRewrite_SignoffChosenVariantFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumptionRewrite{
		AssumptionID: "A-base",
		NewStatement: "a rewrite with a bogus chosen_variant",
		Reason:       "testing",
		Signoff: &ontology.Signoff{
			DecidedBy:     "sa",
			Verbatim:      "approved",
			ChosenVariant: "variant-a",
		},
	}
	assertApplyFails(t, path, p, "chosen_variant")
}

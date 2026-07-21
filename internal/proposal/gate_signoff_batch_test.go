package proposal

import (
	"encoding/json"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestProposedGateSignoffBatch_JSONRoundTrip proves the snake_case json tags
// on ProposedGateSignoffBatch/GateSignoffEntry actually populate every
// field, mirroring TestProposedReviewMark_JSONRoundTrip's guard in
// review_mark_test.go.
func TestProposedGateSignoffBatch_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"entries": [
			{
				"requirement_id": "R-1", "stage": "P-G1", "state": "SIGNED",
				"deferred_reason": "", "evidence": ["docs/review.md"],
				"pipeline_run": "run-2026-07", "decided_by": "outsider",
				"date": "2026-07-19", "verbatim": "approved at review",
				"instrument": "personal"
			}
		]
	}`)
	var p ProposedGateSignoffBatch
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Entries) != 1 {
		t.Fatalf("Entries len = %d, want 1", len(p.Entries))
	}
	e := p.Entries[0]
	if e.RequirementID != "R-1" || e.Stage != "P-G1" || e.State != "SIGNED" {
		t.Errorf("RequirementID/Stage/State = %q/%q/%q, want R-1/P-G1/SIGNED", e.RequirementID, e.Stage, e.State)
	}
	if len(e.Evidence) != 1 || e.Evidence[0] != "docs/review.md" {
		t.Errorf("Evidence = %v, want [docs/review.md]", e.Evidence)
	}
	if e.PipelineRun != "run-2026-07" {
		t.Errorf("PipelineRun = %q, want run-2026-07", e.PipelineRun)
	}
	if e.DecidedBy != "outsider" || e.Date != "2026-07-19" || e.Verbatim != "approved at review" || e.Instrument != "personal" {
		t.Errorf("DecidedBy/Date/Verbatim/Instrument = %q/%q/%q/%q, want outsider/2026-07-19/approved at review/personal",
			e.DecidedBy, e.Date, e.Verbatim, e.Instrument)
	}
	if p.Kind() != KindGateSignoffBatch {
		t.Errorf("Kind() = %q, want %q", p.Kind(), KindGateSignoffBatch)
	}
}

// TestApply_GateSignoffBatch_AppliesAllEntries proves a batch of entries
// targeting DIFFERENT requirements lands atomically: both requirements gain
// their new gate_signoffs entry after a single Apply.
func TestApply_GateSignoffBatch_AppliesAllEntries(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedGateSignoffBatch{
		Entries: []GateSignoffEntry{
			{RequirementID: "R-1", Stage: "P-G0", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1", DecidedBy: "outsider"},
			{RequirementID: "R-2", Stage: "P-G0", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1", DecidedBy: "outsider"},
		},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	r1, ok := findReq(g, "R-1")
	if !ok {
		t.Fatal("R-1 missing")
	}
	if len(r1.GateSignoffs) != 1 || r1.GateSignoffs[0].Stage != "P-G0" {
		t.Errorf("R-1.GateSignoffs = %+v, want one P-G0 entry", r1.GateSignoffs)
	}
	r2, ok := findReq(g, "R-2")
	if !ok {
		t.Fatal("R-2 missing")
	}
	if len(r2.GateSignoffs) != 1 || r2.GateSignoffs[0].Stage != "P-G0" {
		t.Errorf("R-2.GateSignoffs = %+v, want one P-G0 entry", r2.GateSignoffs)
	}
}

// TestApply_GateSignoffBatch_MultipleEntriesSameRequirement proves entries
// targeting the SAME requirement (e.g. a requirement clearing two stages in
// one wave) both land, appended in order.
func TestApply_GateSignoffBatch_MultipleEntriesSameRequirement(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedGateSignoffBatch{
		Entries: []GateSignoffEntry{
			{RequirementID: "R-1", Stage: "P-G0", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1"},
			{RequirementID: "R-1", Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1"},
		},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	r1, ok := findReq(g, "R-1")
	if !ok {
		t.Fatal("R-1 missing")
	}
	if len(r1.GateSignoffs) != 2 {
		t.Fatalf("R-1.GateSignoffs len = %d, want 2", len(r1.GateSignoffs))
	}
	if r1.GateSignoffs[0].Stage != "P-G0" || r1.GateSignoffs[1].Stage != "P-G1" {
		t.Errorf("R-1.GateSignoffs = %+v, want [P-G0, P-G1] in order", r1.GateSignoffs)
	}
}

// TestApply_GateSignoffBatch_SignoffPayloadCarried proves decided_by/date/
// verbatim/instrument land on the entry's embedded *ontology.Signoff, and
// that omitting decided_by leaves Signoff nil (a DEFERRED entry with no
// decision yet).
func TestApply_GateSignoffBatch_SignoffPayloadCarried(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedGateSignoffBatch{
		Entries: []GateSignoffEntry{
			{
				RequirementID: "R-1", Stage: "P-G0", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1",
				DecidedBy: "outsider", Date: "2026-01-01", Verbatim: "approved", Instrument: ontology.SignoffInstrumentDEL,
			},
			{RequirementID: "R-2", Stage: "P-G0", State: ontology.GateSignoffStateDeferred, PipelineRun: "run-1", DeferredReason: "awaiting review"},
		},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	r1, _ := findReq(g, "R-1")
	if r1.GateSignoffs[0].Signoff == nil {
		t.Fatal("R-1.GateSignoffs[0].Signoff is nil, want a populated Signoff")
	}
	s := r1.GateSignoffs[0].Signoff
	if s.DecidedBy != "outsider" || s.Date != "2026-01-01" || s.Verbatim != "approved" || s.Instrument != ontology.SignoffInstrumentDEL {
		t.Errorf("Signoff = %+v, want decided_by=outsider date=2026-01-01 verbatim=approved instrument=DEL", s)
	}
	r2, _ := findReq(g, "R-2")
	if r2.GateSignoffs[0].Signoff != nil {
		t.Errorf("R-2.GateSignoffs[0].Signoff = %+v, want nil (no decided_by supplied)", r2.GateSignoffs[0].Signoff)
	}
	if r2.GateSignoffs[0].DeferredReason != "awaiting review" {
		t.Errorf("R-2.GateSignoffs[0].DeferredReason = %q, want %q", r2.GateSignoffs[0].DeferredReason, "awaiting review")
	}
}

// TestApply_GateSignoffBatch_GeneratesHistoryEntry proves the batch reuses
// the EXISTING diff->history machinery (summarizeFieldDiff, extended for
// gate_signoffs in history.go) rather than hand-writing history text: each
// affected Requirement gets a HistoryEntry naming the gate_signoffs change.
func TestApply_GateSignoffBatch_GeneratesHistoryEntry(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	before := reload(t, path)
	r1Before, _ := findReq(before, "R-1")
	if len(r1Before.History) != 0 {
		t.Fatalf("fixture precondition: R-1.History should start empty, got %v", r1Before.History)
	}

	p := ProposedGateSignoffBatch{
		Entries: []GateSignoffEntry{
			{RequirementID: "R-1", Stage: "P-G0", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1"},
		},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	r1, _ := findReq(g, "R-1")
	if len(r1.History) != 1 {
		t.Fatalf("R-1.History len = %d, want 1 derived entry", len(r1.History))
	}
	if r1.History[0].At != today {
		t.Errorf("History[0].At = %q, want %q", r1.History[0].At, today)
	}
	if !containsString(r1.History[0].Summary, "gate_signoffs") {
		t.Errorf("History[0].Summary = %q, want it to mention gate_signoffs", r1.History[0].Summary)
	}
}

// TestApply_GateSignoffBatch_UnknownRequirementFails_GraphUnchanged proves
// an entry whose requirement_id does not resolve aborts the WHOLE proposal
// (mutate() returns an error, applyToGraph never reaches WriteGraph) --
// atomicity within a single ProposedGateSignoffBatch mirrors ApplyBatch's
// atomicity ACROSS several proposals.
func TestApply_GateSignoffBatch_UnknownRequirementFails_GraphUnchanged(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedGateSignoffBatch{
		Entries: []GateSignoffEntry{
			{RequirementID: "R-1", Stage: "P-G0", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1"},
			{RequirementID: "R-does-not-exist", Stage: "P-G0", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1"},
		},
	}
	assertApplyFails(t, path, p, "not found")
	g := reload(t, path)
	r1, _ := findReq(g, "R-1")
	if len(r1.GateSignoffs) != 0 {
		t.Errorf("R-1.GateSignoffs = %+v, want unchanged (whole proposal must be rejected)", r1.GateSignoffs)
	}
}

// TestApply_GateSignoffBatch_EmptyEntriesFails proves an empty batch is
// rejected at validate() time, before any graph I/O.
func TestApply_GateSignoffBatch_EmptyEntriesFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	assertApplyFails(t, path, ProposedGateSignoffBatch{}, "entries")
}

// TestApply_GateSignoffBatch_InvalidStateFails proves an entry with an
// unrecognized state (not SIGNED/DEFERRED) is rejected.
func TestApply_GateSignoffBatch_InvalidStateFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedGateSignoffBatch{
		Entries: []GateSignoffEntry{
			{RequirementID: "R-1", Stage: "P-G0", State: "MAYBE", PipelineRun: "run-1"},
		},
	}
	assertApplyFails(t, path, p, "state")
}

// TestApply_GateSignoffBatch_DeferredWithoutReasonFails proves a DEFERRED
// entry with an empty deferred_reason is rejected at validate() time (the
// SAME rule check_gate_signoff_deferred_reason_present polices post-land,
// caught earlier here so a malformed batch never reaches the graph).
func TestApply_GateSignoffBatch_DeferredWithoutReasonFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedGateSignoffBatch{
		Entries: []GateSignoffEntry{
			{RequirementID: "R-1", Stage: "P-G0", State: ontology.GateSignoffStateDeferred, PipelineRun: "run-1"},
		},
	}
	assertApplyFails(t, path, p, "deferred_reason")
}

// TestApply_GateSignoffBatch_MissingPipelineRunFails proves pipeline_run is
// mandatory (it is the unit check_gate_signoff_monotonic groups by; an entry
// with no pipeline_run can never be checked for ordering).
func TestApply_GateSignoffBatch_MissingPipelineRunFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedGateSignoffBatch{
		Entries: []GateSignoffEntry{
			{RequirementID: "R-1", Stage: "P-G0", State: ontology.GateSignoffStateSigned},
		},
	}
	assertApplyFails(t, path, p, "pipeline_run")
}

// TestApply_GateSignoffBatch_MissingStageFails proves stage is mandatory.
func TestApply_GateSignoffBatch_MissingStageFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedGateSignoffBatch{
		Entries: []GateSignoffEntry{
			{RequirementID: "R-1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1"},
		},
	}
	assertApplyFails(t, path, p, "stage")
}

// TestApply_GateSignoffBatch_ViaApplyBatch proves the new kind also works
// through the multi-proposal ApplyBatch entry point (alongside other
// proposal kinds in the same call), not just the single-proposal Apply path.
func TestApply_GateSignoffBatch_ViaApplyBatch(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	batch := []Proposal{
		ProposedRequirement{ID: "R-new", Claim: "a new requirement", Owner: "sa", Status: ontology.StatusDRAFT},
		ProposedGateSignoffBatch{
			Entries: []GateSignoffEntry{
				{RequirementID: "R-1", Stage: "P-G0", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1"},
			},
		},
	}
	if err := ApplyBatch(path, today, batch, nil, nil); err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}
	g := reload(t, path)
	if _, ok := findReq(g, "R-new"); !ok {
		t.Error("R-new missing after batch")
	}
	r1, _ := findReq(g, "R-1")
	if len(r1.GateSignoffs) != 1 {
		t.Errorf("R-1.GateSignoffs = %+v, want one entry landed via ApplyBatch", r1.GateSignoffs)
	}
}

// TestProposedGateSignoffBatch_Kind_TargetAnchor proves the Kind/TargetAnchor
// contract: TargetAnchor joins every entry's RequirementID.
func TestProposedGateSignoffBatch_Kind_TargetAnchor(t *testing.T) {
	t.Parallel()
	p := ProposedGateSignoffBatch{
		Entries: []GateSignoffEntry{
			{RequirementID: "R-1"},
			{RequirementID: "R-2"},
		},
	}
	if p.Kind() != KindGateSignoffBatch {
		t.Errorf("Kind() = %q, want %q", p.Kind(), KindGateSignoffBatch)
	}
	if p.TargetAnchor() != "R-1,R-2" {
		t.Errorf("TargetAnchor() = %q, want %q", p.TargetAnchor(), "R-1,R-2")
	}
}

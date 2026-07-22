package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// gateSignoffFixture builds a temp domain directory with a manifest.json
// carrying the supplied gate_stage_order JSON fragment (or no field at all
// when gateStageOrder == ""). Returns the domain directory usable as
// g.DomainDir, mirroring orientationFAQFixture's shape in
// orientation_faq_test.go.
func gateSignoffFixture(t *testing.T, gateStageOrder string) string {
	t.Helper()
	tmp := t.TempDir()
	domainDir := filepath.Join(tmp, "domains", "testdomain")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("MkdirAll domainDir: %v", err)
	}
	manifest := `{"purpose": "test domain", "parent": null}`
	if gateStageOrder != "" {
		manifest = `{"purpose": "test domain", "parent": null, "gate_stage_order": ` + gateStageOrder + `}`
	}
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	return domainDir
}

func gsSigned(stage, run string) ontology.GateSignoff {
	return ontology.GateSignoff{Stage: stage, State: ontology.GateSignoffStateSigned, PipelineRun: run}
}

func gsSignedWithProvenance(stage, run, decidedBy, verbatim string, evidence ...string) ontology.GateSignoff {
	return ontology.GateSignoff{
		Stage:       stage,
		State:       ontology.GateSignoffStateSigned,
		PipelineRun: run,
		Evidence:    evidence,
		Signoff:     &ontology.Signoff{DecidedBy: decidedBy, Verbatim: verbatim},
	}
}

func gsDeferred(stage, run, reason string) ontology.GateSignoff {
	return ontology.GateSignoff{Stage: stage, State: ontology.GateSignoffStateDeferred, PipelineRun: run, DeferredReason: reason}
}

// --- check_gate_signoff_monotonic ---

func TestCheckGateSignoffMonotonic_NoOpWhenDomainDirEmpty(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{gsSigned("P-G2", "run-1")}},
		},
	}
	if vs := runCheck(t, "check_gate_signoff_monotonic", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a graph with no DomainDir, got %v", vs)
	}
}

func TestCheckGateSignoffMonotonic_NoOpWhenGateStageOrderAbsent(t *testing.T) {
	t.Parallel()
	domainDir := gateSignoffFixture(t, "")
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			// SIGNED at a "late" stage with no earlier stage signed -- would
			// violate monotonicity IF gate_stage_order were declared, but it
			// is not, so this must be a pure no-op.
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{gsSigned("P-G3", "run-1")}},
		},
	}
	if vs := runCheck(t, "check_gate_signoff_monotonic", g); len(vs) != 0 {
		t.Fatalf("expected no violations when the domain has not declared gate_stage_order, got %v", vs)
	}
}

func TestCheckGateSignoffMonotonic_PassesWhenPrefixSignedInOrder(t *testing.T) {
	t.Parallel()
	domainDir := gateSignoffFixture(t, `["P-G0", "P-G1", "P-G2", "P-G3"]`)
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				gsSigned("P-G0", "run-1"),
				gsSigned("P-G1", "run-1"),
				gsSigned("P-G2", "run-1"),
			}},
		},
	}
	if vs := runCheck(t, "check_gate_signoff_monotonic", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a prefix-closed SIGNED sequence, got %v", vs)
	}
}

func TestCheckGateSignoffMonotonic_FiresWhenEarlierStageSkipped(t *testing.T) {
	t.Parallel()
	domainDir := gateSignoffFixture(t, `["P-G0", "P-G1", "P-G2", "P-G3"]`)
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			// SIGNED at P-G2 without P-G0/P-G1 signed in the SAME run.
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{gsSigned("P-G2", "run-1")}},
		},
	}
	vs := runCheck(t, "check_gate_signoff_monotonic", g)
	if len(vs) == 0 {
		t.Fatal("expected violations for a SIGNED stage with earlier stages missing in the same run")
	}
	if !hasViolationFor(vs, "R-1") {
		t.Errorf("expected a violation naming R-1, got %v", vs)
	}
}

func TestCheckGateSignoffMonotonic_DifferentPipelineRunsCheckedIndependently(t *testing.T) {
	t.Parallel()
	domainDir := gateSignoffFixture(t, `["P-G0", "P-G1", "P-G2"]`)
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			// run-1 fully prefix-closed through P-G1; run-2 SIGNED at P-G2
			// only -- run-2 must NOT be excused by run-1's progress, since
			// each pipeline_run is a fresh attempt.
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				gsSigned("P-G0", "run-1"),
				gsSigned("P-G1", "run-1"),
				gsSigned("P-G2", "run-2"),
			}},
		},
	}
	vs := runCheck(t, "check_gate_signoff_monotonic", g)
	if len(vs) == 0 {
		t.Fatal("expected a violation for run-2's un-prefixed SIGNED entry, pipeline_run scoping must not leak across runs")
	}
}

func TestCheckGateSignoffMonotonic_FiresWhenStageNotDeclared(t *testing.T) {
	t.Parallel()
	domainDir := gateSignoffFixture(t, `["P-G0", "P-G1"]`)
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{gsSigned("P-G99-typo", "run-1")}},
		},
	}
	vs := runCheck(t, "check_gate_signoff_monotonic", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for an undeclared stage name, got %d: %v", len(vs), vs)
	}
}

func TestCheckGateSignoffMonotonic_DeferredEntriesIgnoredForOrdering(t *testing.T) {
	t.Parallel()
	domainDir := gateSignoffFixture(t, `["P-G0", "P-G1", "P-G2"]`)
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			// Only DEFERRED entries -- no SIGNED entries at all -- must never
			// fire the monotonicity check (it only polices SIGNED order).
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				gsDeferred("P-G0", "run-1", "waiting on review"),
				gsDeferred("P-G2", "run-1", "blocked"),
			}},
		},
	}
	if vs := runCheck(t, "check_gate_signoff_monotonic", g); len(vs) != 0 {
		t.Fatalf("expected DEFERRED-only entries to never trigger the monotonicity check, got %v", vs)
	}
}

// --- check_gate_signoff_deferred_reason_present ---

func TestCheckGateSignoffDeferredReasonPresent_PassesWithReason(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{gsDeferred("P-G1", "run-1", "awaiting review")}},
		},
	}
	if vs := runCheck(t, "check_gate_signoff_deferred_reason_present", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a DEFERRED entry with a non-empty reason, got %v", vs)
	}
}

func TestCheckGateSignoffDeferredReasonPresent_FiresWithoutReason(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{gsDeferred("P-G1", "run-1", "")}},
		},
	}
	vs := runCheck(t, "check_gate_signoff_deferred_reason_present", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for a DEFERRED entry with an empty reason, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "R-1") {
		t.Errorf("expected the violation to name R-1, got %v", vs)
	}
}

func TestCheckGateSignoffDeferredReasonPresent_SignedNeverFires(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{gsSigned("P-G1", "run-1")}},
		},
	}
	if vs := runCheck(t, "check_gate_signoff_deferred_reason_present", g); len(vs) != 0 {
		t.Fatalf("expected a SIGNED entry to never trigger the deferred-reason check, got %v", vs)
	}
}

// --- check_gate_signoff_signed_has_provenance ---

func TestCheckGateSignoffSignedHasProvenance_PassesWithFullProvenance(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				gsSignedWithProvenance("P-G1", "run-1", "outsider", "approved at review", "docs/review.md"),
			}},
		},
	}
	if vs := runCheck(t, "check_gate_signoff_signed_has_provenance", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a SIGNED entry with decided_by/verbatim/evidence, got %v", vs)
	}
}

func TestCheckGateSignoffSignedHasProvenance_FiresWithNilSignoff(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{gsSigned("P-G1", "run-1")}},
		},
	}
	vs := runCheck(t, "check_gate_signoff_signed_has_provenance", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for a SIGNED entry with no Signoff and no evidence, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "R-1") {
		t.Errorf("expected the violation to name R-1, got %v", vs)
	}
}

func TestCheckGateSignoffSignedHasProvenance_FiresWithEmptyDecidedBy(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				gsSignedWithProvenance("P-G1", "run-1", "", "approved at review", "docs/review.md"),
			}},
		},
	}
	vs := runCheck(t, "check_gate_signoff_signed_has_provenance", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for a SIGNED entry with empty decided_by, got %d: %v", len(vs), vs)
	}
}

func TestCheckGateSignoffSignedHasProvenance_FiresWithEmptyVerbatim(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				gsSignedWithProvenance("P-G1", "run-1", "outsider", "", "docs/review.md"),
			}},
		},
	}
	vs := runCheck(t, "check_gate_signoff_signed_has_provenance", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for a SIGNED entry with empty verbatim, got %d: %v", len(vs), vs)
	}
}

func TestCheckGateSignoffSignedHasProvenance_FiresWithNoEvidence(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				gsSignedWithProvenance("P-G1", "run-1", "outsider", "approved at review"),
			}},
		},
	}
	vs := runCheck(t, "check_gate_signoff_signed_has_provenance", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for a SIGNED entry with no evidence, got %d: %v", len(vs), vs)
	}
}

func TestCheckGateSignoffSignedHasProvenance_DeferredNeverFires(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{gsDeferred("P-G1", "run-1", "awaiting review")}},
		},
	}
	if vs := runCheck(t, "check_gate_signoff_signed_has_provenance", g); len(vs) != 0 {
		t.Fatalf("expected a DEFERRED entry (no provenance required) to never trigger the SIGNED-provenance check, got %v", vs)
	}
}

// --- check_gate_signoff_decided_by_is_known_stakeholder ---

func TestCheckGateSignoffDecidedByIsKnownStakeholder_PassesWithKnownStakeholder(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sOut},
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				gsSignedWithProvenance("P-G1", "run-1", "outsider", "approved at review", "docs/review.md"),
			}},
		},
	}
	if vs := runCheck(t, "check_gate_signoff_decided_by_is_known_stakeholder", g); len(vs) != 0 {
		t.Fatalf("expected no violations when decided_by resolves to a known Stakeholder, got %v", vs)
	}
}

func TestCheckGateSignoffDecidedByIsKnownStakeholder_FiresWithUnknownStakeholder(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sOut},
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				gsSignedWithProvenance("P-G1", "run-1", "nobody-known", "approved at review", "docs/review.md"),
			}},
		},
	}
	vs := runCheck(t, "check_gate_signoff_decided_by_is_known_stakeholder", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for decided_by naming an unknown Stakeholder, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "R-1") {
		t.Errorf("expected the violation to name R-1, got %v", vs)
	}
}

func TestCheckGateSignoffDecidedByIsKnownStakeholder_NoOpWhenSignoffNil(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sOut},
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{gsSigned("P-G1", "run-1")}},
		},
	}
	if vs := runCheck(t, "check_gate_signoff_decided_by_is_known_stakeholder", g); len(vs) != 0 {
		t.Fatalf("expected no violations when Signoff is nil (a different check polices provenance presence), got %v", vs)
	}
}

func TestCheckGateSignoffDecidedByIsKnownStakeholder_DeferredNeverFires(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sOut},
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{gsDeferred("P-G1", "run-1", "awaiting review")}},
		},
	}
	if vs := runCheck(t, "check_gate_signoff_decided_by_is_known_stakeholder", g); len(vs) != 0 {
		t.Fatalf("expected a DEFERRED entry to never trigger the decided_by-stakeholder check, got %v", vs)
	}
}

// --- check_gate_signoff_deferred_conflict_resolves ---

func TestCheckGateSignoffDeferredConflictResolves_PassesWhenConflictExists(t *testing.T) {
	t.Parallel()
	c := baseConflict()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				gsDeferred("P-G1", "run-1", "blocked on "+c.ID),
			}},
		},
		Conflicts: []ontology.Conflict{c},
	}
	if vs := runCheck(t, "check_gate_signoff_deferred_conflict_resolves", g); len(vs) != 0 {
		t.Fatalf("expected no violations when the referenced conflict exists, got %v", vs)
	}
}

func TestCheckGateSignoffDeferredConflictResolves_FiresWhenConflictMissing(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				gsDeferred("P-G1", "run-1", "blocked on C-deadbeef"),
			}},
		},
	}
	vs := runCheck(t, "check_gate_signoff_deferred_conflict_resolves", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for a dangling conflict reference, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "R-1") {
		t.Errorf("expected the violation to name R-1, got %v", vs)
	}
}

func TestCheckGateSignoffDeferredConflictResolves_NoOpWhenNoConflictIdReferenced(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				gsDeferred("P-G1", "run-1", "waiting on external review, no conflict involved"),
			}},
		},
	}
	if vs := runCheck(t, "check_gate_signoff_deferred_conflict_resolves", g); len(vs) != 0 {
		t.Fatalf("expected no violations when the deferred_reason names no Conflict-id-shaped token, got %v", vs)
	}
}

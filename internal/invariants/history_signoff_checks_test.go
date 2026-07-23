package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func heWithSignoff(decidedBy, verbatim string) ontology.HistoryEntry {
	return ontology.HistoryEntry{
		At:        "2026-07-23",
		Summary:   "test entry",
		DecidedBy: decidedBy,
		Signoff:   &ontology.Signoff{DecidedBy: decidedBy, Verbatim: verbatim},
	}
}

// --- check_history_signoff_has_provenance ---

func TestCheckHistorySignoffHasProvenance_PassesWithFullProvenance(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", History: []ontology.HistoryEntry{heWithSignoff("outsider", "approved verbatim")}},
		},
	}
	if vs := runCheck(t, "check_history_signoff_has_provenance", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a signoff with decided_by+verbatim, got %v", vs)
	}
}

func TestCheckHistorySignoffHasProvenance_NoOpWhenSignoffNil(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", History: []ontology.HistoryEntry{{At: "2026-07-23", Summary: "plain edit, no signoff"}}},
		},
	}
	if vs := runCheck(t, "check_history_signoff_has_provenance", g); len(vs) != 0 {
		t.Fatalf("expected no violations when Signoff is nil, got %v", vs)
	}
}

func TestCheckHistorySignoffHasProvenance_FiresWithEmptyDecidedBy(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", History: []ontology.HistoryEntry{heWithSignoff("", "approved verbatim")}},
		},
	}
	vs := runCheck(t, "check_history_signoff_has_provenance", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for a signoff with empty decided_by, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "Requirement R-1") {
		t.Errorf("expected the violation to name Requirement R-1, got %v", vs)
	}
}

func TestCheckHistorySignoffHasProvenance_FiresWithEmptyVerbatim(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", History: []ontology.HistoryEntry{heWithSignoff("outsider", "")}},
		},
	}
	vs := runCheck(t, "check_history_signoff_has_provenance", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for a signoff with empty verbatim, got %d: %v", len(vs), vs)
	}
}

// TestCheckHistorySignoffHasProvenance_SweepsAssumption proves the check is
// not Requirement-only -- it sweeps every HistoryEntry-carrying node type
// (see allHistoryEntries in history_signoff_checks.go).
func TestCheckHistorySignoffHasProvenance_SweepsAssumption(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: "A-1", History: []ontology.HistoryEntry{heWithSignoff("", "approved verbatim")}},
		},
	}
	vs := runCheck(t, "check_history_signoff_has_provenance", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for an Assumption's malformed signoff, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "Assumption A-1") {
		t.Errorf("expected the violation to name Assumption A-1, got %v", vs)
	}
}

// --- check_history_signoff_decided_by_is_known_stakeholder ---

func TestCheckHistorySignoffDecidedByIsKnownStakeholder_PassesWithKnownStakeholder(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sOut},
		Requirements: []ontology.Requirement{
			{ID: "R-1", History: []ontology.HistoryEntry{heWithSignoff("outsider", "approved verbatim")}},
		},
	}
	if vs := runCheck(t, "check_history_signoff_decided_by_is_known_stakeholder", g); len(vs) != 0 {
		t.Fatalf("expected no violations when decided_by resolves to a known Stakeholder, got %v", vs)
	}
}

func TestCheckHistorySignoffDecidedByIsKnownStakeholder_FiresWithUnknownStakeholder(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sOut},
		Requirements: []ontology.Requirement{
			{ID: "R-1", History: []ontology.HistoryEntry{heWithSignoff("nobody-known", "approved verbatim")}},
		},
	}
	vs := runCheck(t, "check_history_signoff_decided_by_is_known_stakeholder", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for decided_by naming an unknown Stakeholder, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "Requirement R-1") {
		t.Errorf("expected the violation to name Requirement R-1, got %v", vs)
	}
}

// TestCheckHistorySignoffDecidedByIsKnownStakeholder_SkipsOnEmptyDecidedBy
// proves the "skip (no violation) when DecidedBy is empty" rule from the
// check's own doc comment: check_history_signoff_has_provenance owns that
// case, so this check must not double-report it.
func TestCheckHistorySignoffDecidedByIsKnownStakeholder_SkipsOnEmptyDecidedBy(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sOut},
		Requirements: []ontology.Requirement{
			{ID: "R-1", History: []ontology.HistoryEntry{heWithSignoff("", "approved verbatim")}},
		},
	}
	if vs := runCheck(t, "check_history_signoff_decided_by_is_known_stakeholder", g); len(vs) != 0 {
		t.Fatalf("expected no violations when decided_by is empty (provenance check owns that case), got %v", vs)
	}
}

func TestCheckHistorySignoffDecidedByIsKnownStakeholder_NoOpWhenSignoffNil(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sOut},
		Requirements: []ontology.Requirement{
			{ID: "R-1", History: []ontology.HistoryEntry{{At: "2026-07-23", Summary: "plain edit, no signoff"}}},
		},
	}
	if vs := runCheck(t, "check_history_signoff_decided_by_is_known_stakeholder", g); len(vs) != 0 {
		t.Fatalf("expected no violations when Signoff is nil, got %v", vs)
	}
}

// TestCheckHistorySignoffDecidedByIsKnownStakeholder_SweepsAssumption mirrors
// TestCheckHistorySignoffHasProvenance_SweepsAssumption for the second check.
func TestCheckHistorySignoffDecidedByIsKnownStakeholder_SweepsAssumption(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sOut},
		Assumptions: []ontology.Assumption{
			{ID: "A-1", History: []ontology.HistoryEntry{heWithSignoff("nobody-known", "approved verbatim")}},
		},
	}
	vs := runCheck(t, "check_history_signoff_decided_by_is_known_stakeholder", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for an Assumption's unresolvable decided_by, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "Assumption A-1") {
		t.Errorf("expected the violation to name Assumption A-1, got %v", vs)
	}
}

// TestHistorySignoffChecks_CleanOnRealGraph confirms both new checks start
// clean against the real hotam-spec-self graph -- no landed HistoryEntry
// anywhere in it currently carries a signoff (task #328's landed
// R-shared-projections-mode-independent / R-orientation-faq-answerable
// entries record real human approval only as free-text History.Summary with
// DecidedBy == "", which check_history_signoff_has_provenance does NOT flag
// since it only fires when Signoff is non-nil).
func TestHistorySignoffChecks_CleanOnRealGraph(t *testing.T) {
	t.Parallel()
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", domainGraphPath, err)
	}
	if vs := runCheck(t, "check_history_signoff_has_provenance", g); len(vs) != 0 {
		t.Fatalf("expected 0 violations for check_history_signoff_has_provenance on the real graph, got %d: %v", len(vs), vs)
	}
	if vs := runCheck(t, "check_history_signoff_decided_by_is_known_stakeholder", g); len(vs) != 0 {
		t.Fatalf("expected 0 violations for check_history_signoff_decided_by_is_known_stakeholder on the real graph, got %d: %v", len(vs), vs)
	}
}

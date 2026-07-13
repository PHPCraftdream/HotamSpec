package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestCheckEnforcedNamesInvariant_OK(t *testing.T) {
	t.Parallel()
	r := reqEnforced("R-1", "sa", "check_typed_anchors")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_enforced_names_invariant", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckEnforcedNamesInvariant_OKProseAndStructural(t *testing.T) {
	t.Parallel()
	r1 := req("R-1", "sa")
	r1.Enforcement = ontology.EnforcementPROSE
	r2 := req("R-2", "sb")
	r2.Enforcement = ontology.EnforcementSTRUCTURAL
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA, sB}, Requirements: []ontology.Requirement{r1, r2}}
	if vs := runCheck(t, "check_enforced_names_invariant", g); len(vs) != 0 {
		t.Fatalf("expected no violations for PROSE/STRUCTURAL, got %v", vs)
	}
}

func TestCheckEnforcedNamesInvariant_FiresOnEnforcedWithEmptyEnforcedBy(t *testing.T) {
	t.Parallel()
	r := reqEnforced("R-1", "sa")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_enforced_names_invariant", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation on R-1 for ENFORCED with empty enforced_by, got %v", vs)
	}
}

func TestCheckEnforcedNamesInvariant_FiresOnBogusEnforcementLevel(t *testing.T) {
	t.Parallel()
	r := req("R-1", "sa")
	r.Enforcement = "BOGUS"
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_enforced_names_invariant", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation on R-1 for bogus enforcement level, got %v", vs)
	}
}

func TestCheckEnforcedByResolvable_OKRegisteredCheck(t *testing.T) {
	t.Parallel()
	r := reqEnforced("R-1", "sa", "check_typed_anchors")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_enforced_by_resolvable", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a registered check_* enforcer, got %v", vs)
	}
}

// TestCheckEnforcedByResolvable_FiresOnStaleLowercaseTestName documents the
// semantic change from the earlier no-op: a bare lowercase test_* name is
// NOT a valid Go test function (Go requires Test*), so it now fires instead of
// being silently trusted. This is exactly the wave-2 regression shape.
func TestCheckEnforcedByResolvable_FiresOnStaleLowercaseTestName(t *testing.T) {
	t.Parallel()
	r := reqEnforced("R-1", "sa", "test_foo")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_enforced_by_resolvable", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("a bare lowercase test_* name is a dead reference; expected a violation, got %v", vs)
	}
}

// TestCheckEnforcedByResolvable_OKRealTestEntry is the positive Test* half: a
// real top-level Go Test* function name (resolved via the same internal/gate
// scan used for targeted selection) must pass.
func TestCheckEnforcedByResolvable_OKRealTestEntry(t *testing.T) {
	t.Parallel()
	r := reqEnforced("R-1", "sa", "TestApply_Rejection")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_enforced_by_resolvable", g); len(vs) != 0 {
		t.Fatalf("a real Test* function name must resolve; expected no violations, got %v", vs)
	}
}

// TestCheckEnforcedByResolvable_FiresOnPythonPytestNodeId is the headline
// wave-2 regression guard: a leftover pytest-style node-id (test_x.py or
// test_x.py::test_y) does not resolve to any Go check_*/Test* and must fire.
func TestCheckEnforcedByResolvable_FiresOnPythonPytestNodeId(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{
		"test_fake.py::test_nonexistent",
		"test_tool_gate.py",
	} {
		r := reqEnforced("R-1", "sa", bad)
		g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
		vs := runCheck(t, "check_enforced_by_resolvable", g)
		if !hasViolationFor(vs, "R-1") {
			t.Fatalf("expected violation for stale pytest-style enforced_by %q, got %v", bad, vs)
		}
	}
}

// TestCheckEnforcedByResolvable_FiresOnBogusTestStar covers a Test*-shaped name
// that is nonetheless not a real function -- the typo / renamed-enforcer case
// for the Test* half.
func TestCheckEnforcedByResolvable_FiresOnBogusTestStar(t *testing.T) {
	t.Parallel()
	r := reqEnforced("R-1", "sa", "TestDoesNotExist")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_enforced_by_resolvable", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation for a non-existent Test* name, got %v", vs)
	}
}

// TestCheckEnforcedByResolvable_RealGraphIsClean is the real-domain regression
// test: after wave 2 rebound every enforced_by entry in hotam-spec-self to a
// real check_*/Test* name, this invariant must report zero violations there.
// If it fires, either wave 2 regressed or a new ENFORCED requirement landed
// with a stale/typo'd enforcer.
func TestCheckEnforcedByResolvable_RealGraphIsClean(t *testing.T) {
	t.Parallel()
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", domainGraphPath, err)
	}
	if vs := runCheck(t, "check_enforced_by_resolvable", g); len(vs) != 0 {
		t.Fatalf("expected 0 violations on the real hotam-spec-self graph, got %d: %v", len(vs), vs)
	}
}

func TestCheckEnforcedByResolvable_FiresOnUnregisteredCheck(t *testing.T) {
	t.Parallel()
	r := reqEnforced("R-1", "sa", "check_does_not_exist")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_enforced_by_resolvable", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation on R-1 for an unregistered check_* enforcer, got %v", vs)
	}
}

func TestCheckEnforcedByResolvable_SkipsNonSettledOrNonEnforced(t *testing.T) {
	t.Parallel()
	r1 := reqStatus("R-1", "sa", ontology.StatusDRAFT)
	r1.Enforcement = ontology.EnforcementENFORCED
	r1.EnforcedBy = []string{"check_does_not_exist"}
	r2 := req("R-2", "sb")
	r2.Enforcement = ontology.EnforcementPROSE
	r2.EnforcedBy = []string{"check_does_not_exist"}
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA, sB}, Requirements: []ontology.Requirement{r1, r2}}
	if vs := runCheck(t, "check_enforced_by_resolvable", g); len(vs) != 0 {
		t.Fatalf("non-SETTLED or non-ENFORCED requirements must be skipped, got %v", vs)
	}
}

func TestCheckEnforcedByTestHasTeeth_Noop(t *testing.T) {
	t.Parallel()
	r := reqEnforced("R-1", "sa", "test_foo")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_enforced_by_test_has_teeth", g); len(vs) != 0 {
		t.Fatalf("check_enforced_by_test_has_teeth is an honest no-op; expected no violations, got %v", vs)
	}
}

func TestCheckEnforceabilityKindKnown_OK(t *testing.T) {
	t.Parallel()
	r1 := req("R-1", "sa")
	r1.Enforceability = ontology.EnforceabilityENFORCEABLE
	r2 := req("R-2", "sb")
	r2.Enforceability = ontology.EnforceabilityINHERENTLY_PROSE
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA, sB}, Requirements: []ontology.Requirement{r1, r2}}
	if vs := runCheck(t, "check_enforceability_kind_known", g); len(vs) != 0 {
		t.Fatalf("expected no violations for known kinds, got %v", vs)
	}
}

func TestCheckEnforceabilityKindKnown_FiresOnBogusKind(t *testing.T) {
	t.Parallel()
	r := req("R-1", "sa")
	r.Enforceability = "BOGUS"
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_enforceability_kind_known", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation on R-1 for bogus enforceability, got %v", vs)
	}
}

func TestCheckMTagValidFormat_OK(t *testing.T) {
	t.Parallel()
	r := req("R-1", "sa")
	r.MTag = "M3"
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_m_tag_valid_format", g); len(vs) != 0 {
		t.Fatalf("expected no violations for M3, got %v", vs)
	}
}

func TestCheckMTagValidFormat_OKEmptyMTag(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{req("R-1", "sa")}}
	if vs := runCheck(t, "check_m_tag_valid_format", g); len(vs) != 0 {
		t.Fatalf("empty m_tag must be skipped, got %v", vs)
	}
}

func TestCheckMTagValidFormat_FiresOnBadFormats(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"M01", "m17", "M", "Mfoo", "M0", "3M"} {
		r := req("R-1", "sa")
		r.MTag = bad
		g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
		if vs := runCheck(t, "check_m_tag_valid_format", g); !hasViolationFor(vs, "R-1") {
			t.Fatalf("expected violation for m_tag %q, got %v", bad, vs)
		}
	}
}

func TestCheckMTagUnique_OK(t *testing.T) {
	t.Parallel()
	r1 := req("R-1", "sa")
	r1.MTag = "M3"
	r2 := req("R-2", "sb")
	r2.MTag = "M4"
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA, sB}, Requirements: []ontology.Requirement{r1, r2}}
	if vs := runCheck(t, "check_m_tag_unique", g); len(vs) != 0 {
		t.Fatalf("expected no violations for distinct m_tags, got %v", vs)
	}
}

func TestCheckMTagUnique_FiresOnDuplicate(t *testing.T) {
	t.Parallel()
	r1 := req("R-1", "sa")
	r1.MTag = "M3"
	r2 := req("R-2", "sb")
	r2.MTag = "M3"
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA, sB}, Requirements: []ontology.Requirement{r1, r2}}
	vs := runCheck(t, "check_m_tag_unique", g)
	if !hasViolationFor(vs, "R-2") {
		t.Fatalf("expected violation on R-2 for duplicate m_tag, got %v", vs)
	}
}

func TestCheckMTagOpenOnly_OK(t *testing.T) {
	t.Parallel()
	r := reqStatus("R-1", "sa", "OPEN(should we?)")
	r.MTag = "M3"
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_m_tag_open_only", g); len(vs) != 0 {
		t.Fatalf("expected no violations for m_tag on OPEN requirement, got %v", vs)
	}
}

func TestCheckMTagOpenOnly_FiresOnSettled(t *testing.T) {
	t.Parallel()
	r := req("R-1", "sa")
	r.MTag = "M3"
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_m_tag_open_only", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation on R-1 for m_tag on SETTLED requirement, got %v", vs)
	}
}

func TestCheckMTagFormat_DelegatesAndFires(t *testing.T) {
	t.Parallel()
	r1 := req("R-1", "sa")
	r1.MTag = "bad"
	r2 := req("R-2", "sb")
	r2.MTag = "bad"
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA, sB}, Requirements: []ontology.Requirement{r1, r2}}
	vs := runCheck(t, "check_m_tag_format", g)
	if len(vs) < 2 {
		t.Fatalf("check_m_tag_format (delegator) must surface format + uniqueness violations, got %v", vs)
	}
}

func TestCheckMTagFormat_OK(t *testing.T) {
	t.Parallel()
	r := reqStatus("R-1", "sa", "OPEN(question)")
	r.MTag = "M7"
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_m_tag_format", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a well-formed m_tag on OPEN, got %v", vs)
	}
}

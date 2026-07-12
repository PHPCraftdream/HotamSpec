package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func TestCheckSectionAnchorsKnown_AllCanonNonNil(t *testing.T) {
	vs := runCheck(t, "check_section_anchors_known", &ontology.Graph{})
	if len(vs) != 0 {
		t.Fatalf("expected no violations (every registered invariant has non-nil Canon), got %v", vs)
	}
}

func TestCheckBijectionRToEnforcer_OKResolvableCheckName(t *testing.T) {
	r := reqEnforced("R-1", "sa", "check_typed_anchors")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_bijection_r_to_enforcer", g)
	for _, v := range vs {
		if v.ID == "R-1" {
			t.Fatalf("expected no resolvability violation on R-1 for a registered check_* enforcer, got %v", vs)
		}
	}
}

func TestCheckBijectionRToEnforcer_OKTestEntriesExempt(t *testing.T) {
	r := reqEnforced("R-1", "sa", "test_foo.py")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_bijection_r_to_enforcer", g)
	for _, v := range vs {
		if v.ID == "R-1" {
			t.Fatalf("test_* entries are exempt from check_* resolvability, got violation %v", v)
		}
	}
}

func TestCheckBijectionRToEnforcer_FiresOnUnresolvableCheckName(t *testing.T) {
	r := reqEnforced("R-1", "sa", "check_does_not_exist")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_bijection_r_to_enforcer", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected resolvability violation on R-1 for an unregistered check_* enforcer, got %v", vs)
	}
}

func TestCheckBijectionRToEnforcer_OrphanDetectionNoEnforcedNoOrphans(t *testing.T) {
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	vs := runCheck(t, "check_bijection_r_to_enforcer", g)
	if len(vs) != 0 {
		t.Fatalf("expected no orphan violations when graph has no SETTLED/ENFORCED requirements, got %v", vs)
	}
}

func TestCheckBijectionRToEnforcer_OrphanDetectionFiresWhenEnforcedExists(t *testing.T) {
	r := reqEnforced("R-1", "sa", "test_foo.py")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_bijection_r_to_enforcer", g)
	found := false
	for _, v := range vs {
		if v.ID == "check_typed_anchors_requirement" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected orphan violation for check_typed_anchors_requirement (not named by any SETTLED/ENFORCED enforced_by), got %v", vs)
	}
}

func TestCheckMethodMatchesDocstring_Noop(t *testing.T) {
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	if vs := runCheck(t, "check_method_matches_docstring", g); len(vs) != 0 {
		t.Fatalf("check_method_matches_docstring is an honest no-op in the Go port; expected no violations, got %v", vs)
	}
}

func TestCheckRulesAsDataClassificationCoherent_Noop(t *testing.T) {
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	if vs := runCheck(t, "check_rules_as_data_classification_coherent", g); len(vs) != 0 {
		t.Fatalf("check_rules_as_data_classification_coherent is an honest no-op in the Go port; expected no violations, got %v", vs)
	}
}

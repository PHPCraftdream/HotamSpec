package invariants

import (
	"sort"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

var expectedDelegatorNames = []string{
	"check_conflict_has_axis_context_steward",
	"check_decided_has_decided_by",
	"check_domain_manifest_valid",
	"check_held_has_decided_by",
	"check_m_tag_format",
	"check_no_dangling_ids",
	"check_status_in_lifecycle",
	"check_typed_anchors",
}

func TestDelegators_MarkedAndCount(t *testing.T) {
	t.Parallel()
	var marked []string
	for _, inv := range All.All() {
		if inv.IsDelegator {
			marked = append(marked, inv.Name)
		}
	}
	if len(marked) != len(expectedDelegatorNames) {
		t.Fatalf("expected exactly %d IsDelegator invariants, got %d: %v", len(expectedDelegatorNames), len(marked), marked)
	}
	want := append([]string{}, expectedDelegatorNames...)
	got := append([]string{}, marked...)
	sort.Strings(want)
	sort.Strings(got)
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("delegator set mismatch: want %v, got %v", want, got)
		}
	}
}

func TestDelegators_StillResolvableByName(t *testing.T) {
	t.Parallel()
	for _, name := range expectedDelegatorNames {
		inv, ok := All.Get(name)
		if !ok {
			t.Errorf("delegator %q must remain registered and resolvable via All.Get", name)
			continue
		}
		if !inv.IsDelegator {
			t.Errorf("delegator %q must have IsDelegator==true", name)
		}
		if inv.Check == nil {
			t.Errorf("delegator %q must retain its Check function (direct invocation must still work)", name)
		}
	}
}

func TestDelegators_AllViolationsExcludesDelegatorCheckNames(t *testing.T) {
	t.Parallel()
	badReq := ontology.Requirement{
		ID:             "X-1",
		Claim:          "claim X-1",
		Owner:          "sa",
		Status:         ontology.StatusSETTLED,
		Enforcement:    ontology.EnforcementPROSE,
		Enforceability: ontology.EnforceabilityENFORCEABLE,
	}
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{badReq},
	}
	vs := AllViolations(g)
	var typedAnchorsDelegator, typedAnchorsRequirement int
	for _, v := range vs {
		switch v.Check {
		case "check_typed_anchors":
			typedAnchorsDelegator++
		case "check_typed_anchors_requirement":
			if v.ID == "X-1" {
				typedAnchorsRequirement++
			}
		}
	}
	if typedAnchorsDelegator != 0 {
		t.Fatalf("AllViolations must NOT invoke the check_typed_anchors delegator (IsDelegator==true), got %d violations with that Check name: %v", typedAnchorsDelegator, vs)
	}
	if typedAnchorsRequirement != 1 {
		t.Fatalf("expected exactly 1 atomic violation from check_typed_anchors_requirement for id X-1, got %d (the delegator must not duplicate the sub-check): %v", typedAnchorsRequirement, vs)
	}
}

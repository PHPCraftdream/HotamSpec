package proposal

// Tests for the two authored-spec link fields on Requirement —
// implemented_by (file:symbol, where a requirement is EMBODIED in authored
// code) and verified_by (file:test, where it is PROVEN) — mirroring the
// established EnforcedBy proposal-layer contract exactly (see
// PLAN-authored-spec-discipline.md §4/§12): explicit value replaces, empty
// preserves (patch semantics), single-element ["<clear>"] empties, sentinel
// mixed with real entries is rejected by validate().

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestApply_Requirement_SetImplementedByAndVerifiedBy covers the plain SET
// path: an UPDATE proposal carrying implemented_by and verified_by lands both
// on the target requirement.
func TestApply_Requirement_SetImplementedByAndVerifiedBy(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:            "R-1",
		Claim:         "claim R-1",
		Owner:         "sa",
		Status:        ontology.StatusSETTLED,
		ImplementedBy: []string{"spec/model/risk.go:NewRisk"},
		VerifiedBy:    []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	r, ok := findReq(reload(t, path), "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if len(r.ImplementedBy) != 1 || r.ImplementedBy[0] != "spec/model/risk.go:NewRisk" {
		t.Errorf("ImplementedBy = %v, want [spec/model/risk.go:NewRisk]", r.ImplementedBy)
	}
	if len(r.VerifiedBy) != 1 || r.VerifiedBy[0] != "spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner" {
		t.Errorf("VerifiedBy = %v, want [spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner]", r.VerifiedBy)
	}
}

// TestApply_Requirement_EmptyPreservesImplementedByAndVerifiedBy covers the
// patch-semantics half (mirror of TestApply_Requirement_UpdateAppendsHistory's
// Why assertion): an UPDATE proposal that omits implemented_by/verified_by
// leaves previously-set values untouched.
func TestApply_Requirement_EmptyPreservesImplementedByAndVerifiedBy(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	g.Requirements[0].ImplementedBy = []string{"spec/model/risk.go:NewRisk"}
	g.Requirements[0].VerifiedBy = []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"}
	path := writeTempGraph(t, g)

	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "revised claim, links untouched",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
		// ImplementedBy / VerifiedBy intentionally omitted.
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	r, ok := findReq(reload(t, path), "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if len(r.ImplementedBy) != 1 || r.ImplementedBy[0] != "spec/model/risk.go:NewRisk" {
		t.Errorf("ImplementedBy = %v, want preserved [spec/model/risk.go:NewRisk] (patch semantics)", r.ImplementedBy)
	}
	if len(r.VerifiedBy) != 1 || r.VerifiedBy[0] != "spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner" {
		t.Errorf("VerifiedBy = %v, want preserved (patch semantics)", r.VerifiedBy)
	}
}

// TestApply_Requirement_ClearImplementedByAndVerifiedBy is the mirror of
// TestApply_Requirement_ClearEnforcedBy: an UPDATE whose implemented_by /
// verified_by is exactly ["<clear>"] empties a previously-populated list.
func TestApply_Requirement_ClearImplementedByAndVerifiedBy(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	g.Requirements[0].ImplementedBy = []string{"spec/model/risk.go:NewRisk"}
	g.Requirements[0].VerifiedBy = []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"}
	path := writeTempGraph(t, g)

	p := ProposedRequirement{
		ID:            "R-1",
		Claim:         "claim R-1",
		Owner:         "sa",
		Status:        ontology.StatusSETTLED,
		ImplementedBy: []string{clearSentinel},
		VerifiedBy:    []string{clearSentinel},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	r, ok := findReq(reload(t, path), "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if len(r.ImplementedBy) != 0 {
		t.Errorf("ImplementedBy = %v, want empty (cleared by sentinel)", r.ImplementedBy)
	}
	if len(r.VerifiedBy) != 0 {
		t.Errorf("VerifiedBy = %v, want empty (cleared by sentinel)", r.VerifiedBy)
	}
}

// TestApply_Requirement_ImplementedByClearSentinelMixedWithRealFails and its
// verified_by twin mirror TestApply_Requirement_ClearSentinelMixedWithRealFails.
func TestApply_Requirement_ImplementedByClearSentinelMixedWithRealFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:            "R-1",
		Claim:         "claim R-1",
		Owner:         "sa",
		Status:        ontology.StatusSETTLED,
		ImplementedBy: []string{clearSentinel, "spec/model/risk.go:NewRisk"},
	}
	assertApplyFails(t, path, p, clearSentinel)
}

func TestApply_Requirement_VerifiedByClearSentinelMixedWithRealFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:         "R-1",
		Claim:      "claim R-1",
		Owner:      "sa",
		Status:     ontology.StatusSETTLED,
		VerifiedBy: []string{clearSentinel, "spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"},
	}
	assertApplyFails(t, path, p, clearSentinel)
}

// TestApply_Requirement_CreateCarriesImplementedByAndVerifiedBy covers the
// CREATE path (mirror of EnforcedBy on CREATE): a brand-new requirement may
// declare its authored-code links at creation.
func TestApply_Requirement_CreateCarriesImplementedByAndVerifiedBy(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:            "R-new-authored",
		Claim:         "a brand new claim with authored links",
		Owner:         "sa",
		Status:        ontology.StatusDRAFT,
		ImplementedBy: []string{"spec/model/risk.go:NewRisk"},
		VerifiedBy:    []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	r, ok := findReq(reload(t, path), "R-new-authored")
	if !ok {
		t.Fatalf("R-new-authored missing")
	}
	if len(r.ImplementedBy) != 1 || r.ImplementedBy[0] != "spec/model/risk.go:NewRisk" {
		t.Errorf("ImplementedBy = %v, want [spec/model/risk.go:NewRisk]", r.ImplementedBy)
	}
	if len(r.VerifiedBy) != 1 || r.VerifiedBy[0] != "spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner" {
		t.Errorf("VerifiedBy = %v, want the file:test ref", r.VerifiedBy)
	}
}

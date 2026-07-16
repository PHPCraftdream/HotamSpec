package loader

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadGraph_ImplementedByAndVerifiedByKeysAccepted proves the additive
// authored-spec link fields survive the strict decoder: LoadGraph uses
// DisallowUnknownFields, so BEFORE ontology.Requirement carried
// ImplementedBy/VerifiedBy a graph.json with these keys was rejected with an
// opaque "unknown field" error. This is the serialization half of the
// authored-spec discipline (PLAN-authored-spec-discipline.md §4/§12) — the
// resolver/enforcement half is a separate change.
func TestLoadGraph_ImplementedByAndVerifiedByKeysAccepted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	withLinks := `{"requirements": [{"id": "R-1", "claim": "x", "owner": "o", "status": "DRAFT", "why": "", "assumptions": [], "relations": [], "enforcement": "PROSE", "enforced_by": [], "implemented_by": ["spec/model/risk.go:NewRisk"], "verified_by": ["spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"], "m_tag": "", "enforceability": "ENFORCEABLE", "summary": "", "created_at": "", "settled_at": "", "last_reviewed_at": "", "review_after": "", "evidence": [], "source_refs": [], "history": []}]}` + "\n"
	if err := os.WriteFile(out, []byte(withLinks), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	g, err := LoadGraph(out)
	if err != nil {
		t.Fatalf("LoadGraph: %v (implemented_by/verified_by must be accepted by the strict DisallowUnknownFields decoder)", err)
	}
	if len(g.Requirements) != 1 {
		t.Fatalf("expected one requirement, got %d", len(g.Requirements))
	}
	r := g.Requirements[0]
	if len(r.ImplementedBy) != 1 || r.ImplementedBy[0] != "spec/model/risk.go:NewRisk" {
		t.Errorf("ImplementedBy = %v, want [spec/model/risk.go:NewRisk]", r.ImplementedBy)
	}
	if len(r.VerifiedBy) != 1 || r.VerifiedBy[0] != "spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner" {
		t.Errorf("VerifiedBy = %v, want [spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner]", r.VerifiedBy)
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// setupProvenanceTestDomain scaffolds a clean, invariant-valid minimal domain
// (via initDomain), mirroring setupGateTestDomain's shape. When
// requireProvenance is true, manifest.json is overwritten with
// {"require_provenance": true} (same self_hosting:false as initDomain's
// default, since the field is orthogonal) — mirroring the
// os.WriteFile(manifestPath, ...) pattern gen_spec_profile_test.go already
// uses to control manifest content directly in a test.
func setupProvenanceTestDomain(t *testing.T, requireProvenance bool) string {
	t.Helper()
	root := t.TempDir()
	domainDir := filepath.Join(root, "domains", "prov-test")
	if _, err := initDomain(domainDir, "prov-test", "2026-07-15"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}
	if requireProvenance {
		manifestPath := filepath.Join(domainDir, "manifest.json")
		if err := os.WriteFile(manifestPath, []byte("{\"self_hosting\": false, \"require_provenance\": true}\n"), 0o644); err != nil {
			t.Fatalf("write require_provenance manifest: %v", err)
		}
	}
	return domainDir
}

// writeBareSettledReqJSON writes a minimal SETTLED Requirement proposal with
// ZERO provenance fields (no source_refs, evidence, last_reviewed_at,
// review_after).
func writeBareSettledReqJSON(t *testing.T, dir, id string) string {
	t.Helper()
	path := filepath.Join(dir, id+".json")
	jsonStr := `{
		"kind": "Requirement",
		"id": "` + id + `",
		"claim": "a bare settled requirement with no provenance",
		"owner": "owner",
		"status": "SETTLED",
		"why": "provenance gate test"
	}`
	if err := os.WriteFile(path, []byte(jsonStr), 0o644); err != nil {
		t.Fatalf("write proposal %s: %v", path, err)
	}
	return path
}

// writeCompleteSettledReqJSON writes a minimal SETTLED Requirement proposal
// with ALL provenance fields populated.
func writeCompleteSettledReqJSON(t *testing.T, dir, id string) string {
	t.Helper()
	path := filepath.Join(dir, id+".json")
	jsonStr := `{
		"kind": "Requirement",
		"id": "` + id + `",
		"claim": "a settled requirement with complete provenance",
		"owner": "owner",
		"status": "SETTLED",
		"why": "provenance gate test",
		"source_refs": ["https://example.com/source"],
		"evidence": ["steward review"],
		"last_reviewed_at": "2026-07-15",
		"review_after": "2027-07-15"
	}`
	if err := os.WriteFile(path, []byte(jsonStr), 0o644); err != nil {
		t.Fatalf("write proposal %s: %v", path, err)
	}
	return path
}

// --- Scenario 1: default manifest (no require_provenance) — bare SETTLED lands fine ---

func TestProvenanceGate_DefaultManifest_BareSettledLands_Land(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, false)
	p := writeBareSettledReqJSON(t, t.TempDir(), "R-prov-default-land")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-15", p}); err != nil {
		t.Fatalf("expected bare SETTLED requirement to land under default manifest, got: %v", err)
	}
}

func TestProvenanceGate_DefaultManifest_BareSettledLands_ApplyProposal(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, false)
	p := writeBareSettledReqJSON(t, t.TempDir(), "R-prov-default-apply")
	if err := cmdApplyProposal([]string{"--domain", domainDir, "--today", "2026-07-15", p}); err != nil {
		t.Fatalf("expected bare SETTLED requirement to apply under default manifest, got: %v", err)
	}
}

// --- Scenario 2: require_provenance: true — bare SETTLED refused on all 4 surfaces ---

func TestProvenanceGate_RequireProvenance_BareSettledRefused_Land(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, true)
	p := writeBareSettledReqJSON(t, t.TempDir(), "R-prov-bare-land")
	err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-15", p})
	if err == nil {
		t.Fatal("expected hotam land to refuse a bare SETTLED requirement under require_provenance:true")
	}
	assertNamesMissingFields(t, err.Error())
}

func TestProvenanceGate_RequireProvenance_BareSettledRefused_ApplyProposal(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, true)
	p := writeBareSettledReqJSON(t, t.TempDir(), "R-prov-bare-apply")
	err := cmdApplyProposal([]string{"--domain", domainDir, "--today", "2026-07-15", p})
	if err == nil {
		t.Fatal("expected hotam apply-proposal to refuse a bare SETTLED requirement under require_provenance:true")
	}
	assertNamesMissingFields(t, err.Error())
}

func TestProvenanceGate_RequireProvenance_BareSettledRefused_ApplyProposalBatch(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, true)
	batchDir := t.TempDir()
	writeBareSettledReqJSON(t, batchDir, "R-prov-bare-batch-apply")
	err := cmdApplyProposal([]string{"--domain", domainDir, "--today", "2026-07-15", "--batch", batchDir})
	if err == nil {
		t.Fatal("expected hotam apply-proposal --batch to refuse a bare SETTLED requirement under require_provenance:true")
	}
	assertNamesMissingFields(t, err.Error())
}

func TestProvenanceGate_RequireProvenance_BareSettledRefused_LandBatch(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, true)
	batchDir := t.TempDir()
	writeBareSettledReqJSON(t, batchDir, "R-prov-bare-batch-land")
	err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-15", "--batch", batchDir})
	if err == nil {
		t.Fatal("expected hotam land --batch to refuse a bare SETTLED requirement under require_provenance:true")
	}
	assertNamesMissingFields(t, err.Error())
}

func assertNamesMissingFields(t *testing.T, errStr string) {
	t.Helper()
	for _, field := range []string{"source_refs", "last_reviewed_at", "review_after"} {
		if !strings.Contains(errStr, field) {
			t.Errorf("error must name missing field %q:\n%s", field, errStr)
		}
	}
	if !strings.Contains(errStr, "require_provenance") {
		t.Errorf("error must point at the require_provenance manifest flag:\n%s", errStr)
	}
}

// --- Scenario 3: require_provenance: true — complete provenance lands on all 4 surfaces ---

func TestProvenanceGate_RequireProvenance_CompleteProvenanceLands_Land(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, true)
	p := writeCompleteSettledReqJSON(t, t.TempDir(), "R-prov-complete-land")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-15", p}); err != nil {
		t.Fatalf("expected complete-provenance requirement to land, got: %v", err)
	}
}

func TestProvenanceGate_RequireProvenance_CompleteProvenanceLands_ApplyProposal(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, true)
	p := writeCompleteSettledReqJSON(t, t.TempDir(), "R-prov-complete-apply")
	if err := cmdApplyProposal([]string{"--domain", domainDir, "--today", "2026-07-15", p}); err != nil {
		t.Fatalf("expected complete-provenance requirement to apply, got: %v", err)
	}
}

func TestProvenanceGate_RequireProvenance_CompleteProvenanceLands_ApplyProposalBatch(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, true)
	batchDir := t.TempDir()
	writeCompleteSettledReqJSON(t, batchDir, "R-prov-complete-batch-apply")
	if err := cmdApplyProposal([]string{"--domain", domainDir, "--today", "2026-07-15", "--batch", batchDir}); err != nil {
		t.Fatalf("expected complete-provenance batch to apply, got: %v", err)
	}
}

func TestProvenanceGate_RequireProvenance_CompleteProvenanceLands_LandBatch(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, true)
	batchDir := t.TempDir()
	writeCompleteSettledReqJSON(t, batchDir, "R-prov-complete-batch-land")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-15", "--batch", batchDir}); err != nil {
		t.Fatalf("expected complete-provenance batch to land, got: %v", err)
	}
}

// --- Scenario 4: CREATE-vs-UPDATE correctness through the real CLI ---

// TestProvenanceGate_UpdateOmittingProvenancePreservesIt_Land is the
// CREATE-vs-UPDATE correctness test through the real `hotam land` CLI: land a
// SETTLED requirement with COMPLETE provenance first, then send an UPDATE
// proposal that only changes claim (omitting all provenance fields, relying
// on coalesce-preserve semantics) — the UPDATE must SUCCEED because the
// simulated post-merge result still carries the earlier provenance.
func TestProvenanceGate_UpdateOmittingProvenancePreservesIt_Land(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, true)

	first := writeCompleteSettledReqJSON(t, t.TempDir(), "R-prov-update-preserve")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-15", first}); err != nil {
		t.Fatalf("initial complete-provenance land failed: %v", err)
	}

	updatePath := filepath.Join(t.TempDir(), "update.json")
	updateJSON := `{
		"kind": "Requirement",
		"id": "R-prov-update-preserve",
		"claim": "an updated claim, provenance intentionally omitted",
		"owner": "owner",
		"status": "SETTLED",
		"why": "provenance gate test"
	}`
	if err := os.WriteFile(updatePath, []byte(updateJSON), 0o644); err != nil {
		t.Fatalf("write update proposal: %v", err)
	}
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-15", updatePath}); err != nil {
		t.Fatalf("UPDATE omitting provenance fields must succeed (coalesce-preserve semantics) — the gate must check the MERGED result, not the raw proposal: %v", err)
	}

	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("reload domain graph: %v", err)
	}
	var found bool
	for _, r := range g.Requirements {
		if r.ID == "R-prov-update-preserve" {
			found = true
			if r.Claim != "an updated claim, provenance intentionally omitted" {
				t.Errorf("Claim = %q, want the update to have applied", r.Claim)
			}
			if len(r.SourceRefs) == 0 {
				t.Error("SourceRefs empty after update — provenance must be preserved via coalesce")
			}
		}
	}
	if !found {
		t.Fatal("R-prov-update-preserve missing after update")
	}
}

// TestProvenanceGate_CreateMasqueradingAsUpdateStillRefused proves the
// converse: an UPDATE-shaped proposal that actually targets a nonexistent id
// (so mutate() takes the CREATE branch) with incomplete provenance must still
// be REFUSED — the gate does not accidentally treat "no existing node found"
// as "provenance was already fine".
func TestProvenanceGate_CreateMasqueradingAsUpdateStillRefused(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, true)

	// Looks like a content-only edit, but R-prov-new-create does not exist yet
	// — this is a genuine CREATE with incomplete provenance.
	path := filepath.Join(t.TempDir(), "create.json")
	createJSON := `{
		"kind": "Requirement",
		"id": "R-prov-new-create",
		"claim": "a brand-new settled requirement, no provenance supplied",
		"owner": "owner",
		"status": "SETTLED",
		"why": "provenance gate test"
	}`
	if err := os.WriteFile(path, []byte(createJSON), 0o644); err != nil {
		t.Fatalf("write create proposal: %v", err)
	}
	err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-15", path})
	if err == nil {
		t.Fatal("expected a CREATE with incomplete provenance to be refused even though it looks like an UPDATE")
	}
	assertNamesMissingFields(t, err.Error())
}

// --- Scenario 5: DRAFT status bypasses the gate even under require_provenance:true ---

func TestProvenanceGate_DraftStatusBypassesGate(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, true)
	path := filepath.Join(t.TempDir(), "draft.json")
	draftJSON := `{
		"kind": "Requirement",
		"id": "R-prov-draft",
		"claim": "a draft requirement with zero provenance",
		"owner": "owner",
		"status": "DRAFT",
		"why": "provenance gate test"
	}`
	if err := os.WriteFile(path, []byte(draftJSON), 0o644); err != nil {
		t.Fatalf("write draft proposal: %v", err)
	}
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-15", path}); err != nil {
		t.Fatalf("expected a DRAFT requirement with zero provenance to land even under require_provenance:true, got: %v", err)
	}
}

// TestProvenanceGate_NonRequirementProposalUnaffected proves the gate is a
// pure no-op for a non-Requirement proposal kind (mirrors
// semanticConflictGate's `_, ok := p.(proposal.ProposedRequirement)` guard).
func TestProvenanceGate_NonRequirementProposalUnaffected(t *testing.T) {
	t.Parallel()
	domainDir := setupProvenanceTestDomain(t, true)
	if err := provenanceGate(domainDir, proposal.ProposedAxis{Slug: "test-axis", Description: "d"}); err != nil {
		t.Fatalf("provenanceGate must no-op for a non-Requirement proposal, got: %v", err)
	}
}

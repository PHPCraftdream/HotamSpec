package selfspec

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func appendOnlyFixture() *ontology.Graph {
	return &ontology.Graph{
		Requirements: []ontology.Requirement{
			{
				ID: "R-a",
				History: []ontology.HistoryEntry{
					{At: "2026-01-01", Summary: "first"},
					{At: "2026-01-02", Summary: "second"},
				},
				GateSignoffs: []ontology.GateSignoff{
					{Stage: "P-G0", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1"},
				},
			},
			{
				ID: "R-b",
				History: []ontology.HistoryEntry{
					{At: "2026-01-01", Summary: "only entry"},
				},
			},
		},
	}
}

// cloneAppendOnlyFixture returns a deep-enough copy of g (new slices, so
// mutating the copy's slices never aliases the original) for test cases
// that need to mutate "new" independently of "old".
func cloneAppendOnlyFixture(g *ontology.Graph) *ontology.Graph {
	out := &ontology.Graph{Requirements: make([]ontology.Requirement, len(g.Requirements))}
	for i, r := range g.Requirements {
		cp := r
		cp.History = append([]ontology.HistoryEntry(nil), r.History...)
		cp.GateSignoffs = append([]ontology.GateSignoff(nil), r.GateSignoffs...)
		out.Requirements[i] = cp
	}
	return out
}

func TestVerifyAppendOnly_ValidAppendPasses(t *testing.T) {
	old := appendOnlyFixture()
	newG := cloneAppendOnlyFixture(old)
	newG.Requirements[0].History = append(newG.Requirements[0].History, ontology.HistoryEntry{At: "2026-01-03", Summary: "third"})
	newG.Requirements[0].GateSignoffs = append(newG.Requirements[0].GateSignoffs, ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1"})

	if err := verifyAppendOnly(old, newG); err != nil {
		t.Fatalf("verifyAppendOnly: want nil for a valid pure-append transition, got %v", err)
	}
}

func TestVerifyAppendOnly_IdenticalGraphsPass(t *testing.T) {
	old := appendOnlyFixture()
	newG := cloneAppendOnlyFixture(old)

	if err := verifyAppendOnly(old, newG); err != nil {
		t.Fatalf("verifyAppendOnly: want nil for an unchanged graph, got %v", err)
	}
}

func TestVerifyAppendOnly_HistoryTruncationFails(t *testing.T) {
	old := appendOnlyFixture()
	newG := cloneAppendOnlyFixture(old)
	// Truncate R-a's History from 2 entries down to 1.
	newG.Requirements[0].History = newG.Requirements[0].History[:1]

	if err := verifyAppendOnly(old, newG); err == nil {
		t.Fatal("verifyAppendOnly: want error when History shrinks, got nil")
	}
}

func TestVerifyAppendOnly_HistoryMidEntryMutationFails(t *testing.T) {
	old := appendOnlyFixture()
	newG := cloneAppendOnlyFixture(old)
	// Same length, but the FIRST entry's content changed — a mutation in
	// place, not a truncation, must still be caught by the prefix check.
	newG.Requirements[0].History[0] = ontology.HistoryEntry{At: "2026-01-01", Summary: "TAMPERED"}

	if err := verifyAppendOnly(old, newG); err == nil {
		t.Fatal("verifyAppendOnly: want error when a mid-History entry is mutated (same length), got nil")
	}
}

func TestVerifyAppendOnly_MissingRequirementFails(t *testing.T) {
	old := appendOnlyFixture()
	newG := cloneAppendOnlyFixture(old)
	// Drop R-b entirely.
	newG.Requirements = newG.Requirements[:1]

	if err := verifyAppendOnly(old, newG); err == nil {
		t.Fatal("verifyAppendOnly: want error when a requirement present in old vanishes from new, got nil")
	}
}

func TestVerifyAppendOnly_GateSignoffsTruncationFails(t *testing.T) {
	old := appendOnlyFixture()
	newG := cloneAppendOnlyFixture(old)
	newG.Requirements[0].GateSignoffs = nil

	if err := verifyAppendOnly(old, newG); err == nil {
		t.Fatal("verifyAppendOnly: want error when GateSignoffs shrinks, got nil")
	}
}

func TestVerifyAppendOnly_GateSignoffsMidEntryMutationFails(t *testing.T) {
	old := appendOnlyFixture()
	newG := cloneAppendOnlyFixture(old)
	newG.Requirements[0].GateSignoffs[0] = ontology.GateSignoff{Stage: "P-G0", State: ontology.GateSignoffStateDeferred, DeferredReason: "tampered", PipelineRun: "run-1"}

	if err := verifyAppendOnly(old, newG); err == nil {
		t.Fatal("verifyAppendOnly: want error when a mid-GateSignoffs entry is mutated (same length), got nil")
	}
}

func TestVerifyAppendOnly_NilGraphsAreErrors(t *testing.T) {
	g := appendOnlyFixture()
	if err := verifyAppendOnly(nil, g); err == nil {
		t.Fatal("verifyAppendOnly(nil, g): want error, got nil")
	}
	if err := verifyAppendOnly(g, nil); err == nil {
		t.Fatal("verifyAppendOnly(g, nil): want error, got nil")
	}
}

// TestSyncGraph_SatisfiesAppendOnly is an integration proof that SyncGraph
// itself (not just synthetic fixtures) produces a transition
// verifyAppendOnly accepts — the two functions' contracts must actually
// compose for RAC-B2's future CLI to safely chain them.
func TestSyncGraph_SatisfiesAppendOnly(t *testing.T) {
	reg := firstRegisteredRequirement(t)
	existing := ontology.Requirement{
		ID:      reg.ID,
		Claim:   "STALE, will be replaced",
		History: []ontology.HistoryEntry{{At: "2020-01-01", Summary: "old event"}},
	}
	old := &ontology.Graph{Requirements: []ontology.Requirement{existing}}
	newG := cloneAppendOnlyFixture(old)

	if _, err := SyncGraph(newG, "2026-07-23"); err != nil {
		t.Fatalf("SyncGraph: %v", err)
	}

	if err := verifyAppendOnly(old, newG); err != nil {
		t.Fatalf("verifyAppendOnly after a real SyncGraph run: want nil, got %v", err)
	}
}

package main

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/selfspec"
)

func TestSyncConfrontFlagsOnlyExplicitUnresolvedCarriers(t *testing.T) {
	graph := syncGatesTestGraph()
	report := &selfspec.SyncReport{Entries: []selfspec.SyncReportEntry{
		{ID: "R-sync-gate-changing", Kind: selfspec.SyncKindChanged, FieldDiffs: []selfspec.FieldDiff{{Field: "Claim", New: "NEVER frobnicate"}}},
	}}
	if flagged := syncConfrontFlaggedEntries(graph, report); len(flagged) != 0 {
		t.Fatalf("lexical words fabricated acknowledged conflicts: %v", flagged)
	}
	graph.Conflicts = []ontology.Conflict{{
		ID: "C-formal", Members: []string{"R-sync-gate-existing", "R-sync-gate-changing"},
		Lifecycle: ontology.ConflictDETECTED,
	}}
	flagged := syncConfrontFlaggedEntries(graph, report)
	if !flagged["R-sync-gate-changing"] || flagged["R-sync-gate-existing"] {
		t.Fatalf("formal acknowledgement scope is incorrect: %v", flagged)
	}
	graph.Conflicts[0].Lifecycle = "DECIDED(approved)"
	if flagged := syncConfrontFlaggedEntries(graph, report); len(flagged) != 0 {
		t.Fatalf("resolved carrier was treated as a new conflict: %v", flagged)
	}
}

func TestSyncConflictAckMustCoverEveryActualCarrier(t *testing.T) {
	blockers := []confrontBlockerDigest{
		{EntryID: "R-a", HitID: "R-b", ConflictIDs: []string{"C-ab"}},
		{EntryID: "R-a", HitID: "R-c", ConflictIDs: []string{"C-ac"}},
	}
	if err := validateSyncConflictCoverage(landAckOptions{AckConflict: "C-unrelated"}, blockers); err == nil {
		t.Fatal("unrelated Conflict authorized a sync")
	}
	if err := validateSyncConflictCoverage(landAckOptions{AckConflict: "C-ab"}, blockers); err == nil {
		t.Fatal("acknowledging one carrier authorized a different conflict")
	}
	blockers[1].ConflictIDs = []string{"C-ab", "C-ac"}
	if err := validateSyncConflictCoverage(landAckOptions{AckConflict: "C-ab"}, blockers); err != nil {
		t.Fatal(err)
	}
}

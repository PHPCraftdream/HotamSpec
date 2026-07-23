package selfspec

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// syncFixtureReq returns a structural-only ontology.Requirement, matching
// the shape a registry entry actually carries (no event fields set) — used
// to build a tiny synthetic *ontology.Registry-shaped test setup without
// depending on the real, 301-entry package-level Requirements registry.
func syncFixtureReq(id string) ontology.Requirement {
	return ontology.Requirement{
		ID:             id,
		Claim:          "claim for " + id,
		Owner:          "owner-a",
		Status:         ontology.StatusSETTLED,
		Why:            "why for " + id,
		Assumptions:    []string{},
		Relations:      []ontology.Relation{},
		Enforcement:    ontology.EnforcementPROSE,
		EnforcedBy:     []string{},
		MTag:           "",
		Enforceability: ontology.EnforceabilityENFORCEABLE,
		Summary:        "",
		CreatedAt:      "2026-01-01",
		SettledAt:      "",
		SourceRefs:     []string{},
		DeclOrder:      1,
	}
}

// withTempRegistration registers id/req into the real package-level
// Requirements registry only for the duration of fn, then leaves the
// registry as SyncGraph/MergeIntoGraph's tests found it. registry.Registry
// has no Unregister, so this test helper instead operates against a graph
// containing ONLY IDs that are guaranteed already registered — see
// TestSyncGraph_* below, which use real registered IDs from the package's
// production registry (Requirements.All()) rather than trying to inject
// synthetic ones (the registry is package-level global state, shared with
// merge_test.go's byte-identity tests, and MustRegister panics on a
// duplicate — this package cannot safely register throwaway test-only IDs
// into it).
func firstRegisteredRequirement(t *testing.T) ontology.Requirement {
	t.Helper()
	all := Requirements.All()
	if len(all) == 0 {
		t.Fatal("selfspec.Requirements is empty — this test would be vacuous")
	}
	return all[0]
}

func TestSyncGraph_NilGraphIsError(t *testing.T) {
	if _, err := SyncGraph(nil, "2026-07-23"); err == nil {
		t.Fatal("SyncGraph(nil, ...): want error, got nil")
	}
}

// TestSyncGraph_CreatesNewNode proves the CREATE path: a registered ID
// absent from g.Requirements gets a brand-new node appended, with
// structural fields copied from the registry, event fields empty, and
// exactly one History entry recording the creation.
func TestSyncGraph_CreatesNewNode(t *testing.T) {
	reg := firstRegisteredRequirement(t)

	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-unrelated-existing-node", Claim: "unrelated", Status: ontology.StatusDRAFT},
		},
	}

	report, err := SyncGraph(g, "2026-07-23")
	if err != nil {
		t.Fatalf("SyncGraph: %v", err)
	}

	// The report must record an ADDED entry for every registered ID absent
	// from g — at minimum, reg.ID.
	var entry *SyncReportEntry
	for i := range report.Entries {
		if report.Entries[i].ID == reg.ID {
			entry = &report.Entries[i]
			break
		}
	}
	if entry == nil {
		t.Fatalf("SyncReport: missing entry for newly-created %q; entries=%+v", reg.ID, report.Entries)
	}
	if entry.Kind != SyncKindAdded {
		t.Errorf("SyncReport entry for %q: Kind = %q, want %q", reg.ID, entry.Kind, SyncKindAdded)
	}

	var created *ontology.Requirement
	for i := range g.Requirements {
		if g.Requirements[i].ID == reg.ID {
			created = &g.Requirements[i]
			break
		}
	}
	if created == nil {
		t.Fatalf("SyncGraph did not append a node for %q", reg.ID)
	}

	if created.Claim != reg.Claim {
		t.Errorf("created node Claim = %q, want registry's %q", created.Claim, reg.Claim)
	}
	if created.Owner != reg.Owner {
		t.Errorf("created node Owner = %q, want registry's %q", created.Owner, reg.Owner)
	}
	if created.Status != reg.Status {
		t.Errorf("created node Status = %q, want registry's %q", created.Status, reg.Status)
	}
	if created.DeclOrder != reg.DeclOrder {
		t.Errorf("created node DeclOrder = %d, want registry's %d", created.DeclOrder, reg.DeclOrder)
	}

	// Event fields must be empty.
	if created.LastReviewedAt != "" {
		t.Errorf("created node LastReviewedAt = %q, want empty", created.LastReviewedAt)
	}
	if created.ReviewAfter != "" {
		t.Errorf("created node ReviewAfter = %q, want empty", created.ReviewAfter)
	}
	if len(created.Evidence) != 0 {
		t.Errorf("created node Evidence = %v, want empty", created.Evidence)
	}
	if len(created.GateSignoffs) != 0 {
		t.Errorf("created node GateSignoffs = %v, want empty", created.GateSignoffs)
	}

	// History must carry exactly one entry: "created via sync-self".
	if len(created.History) != 1 {
		t.Fatalf("created node History = %+v, want exactly 1 entry", created.History)
	}
	if created.History[0].At != "2026-07-23" {
		t.Errorf("created node History[0].At = %q, want %q", created.History[0].At, "2026-07-23")
	}
	if created.History[0].Summary != "created via sync-self" {
		t.Errorf("created node History[0].Summary = %q, want %q", created.History[0].Summary, "created via sync-self")
	}

	// The unrelated pre-existing node must be untouched.
	if g.Requirements[0].ID != "R-unrelated-existing-node" || g.Requirements[0].Claim != "unrelated" {
		t.Errorf("pre-existing unrelated node was mutated: %+v", g.Requirements[0])
	}
}

// TestSyncGraph_ChangedNodeAppendsHistory proves the UPDATE path: an
// existing node whose structural fields disagree with the registry gets
// those fields replaced AND a new History entry describing the change,
// while OLD History entries are preserved untouched (append, not replace).
func TestSyncGraph_ChangedNodeAppendsHistory(t *testing.T) {
	reg := firstRegisteredRequirement(t)

	priorEntry := ontology.HistoryEntry{At: "2020-01-01", Summary: "some earlier event"}
	existing := ontology.Requirement{
		ID:             reg.ID,
		Claim:          "STALE placeholder claim, must be replaced",
		Owner:          "stale-owner",
		Status:         reg.Status,
		Enforcement:    reg.Enforcement,
		Enforceability: reg.Enforceability,
		DeclOrder:      reg.DeclOrder,
		History:        []ontology.HistoryEntry{priorEntry},
		LastReviewedAt: "2025-05-05",
		Evidence:       []string{"kept-evidence"},
	}

	g := &ontology.Graph{Requirements: []ontology.Requirement{existing}}

	report, err := SyncGraph(g, "2026-07-23")
	if err != nil {
		t.Fatalf("SyncGraph: %v", err)
	}

	var entry *SyncReportEntry
	for i := range report.Entries {
		if report.Entries[i].ID == reg.ID {
			entry = &report.Entries[i]
			break
		}
	}
	if entry == nil {
		t.Fatalf("SyncReport: missing entry for changed %q; entries=%+v", reg.ID, report.Entries)
	}
	if entry.Kind != SyncKindChanged {
		t.Errorf("SyncReport entry for %q: Kind = %q, want %q", reg.ID, entry.Kind, SyncKindChanged)
	}
	if len(entry.FieldDiffs) == 0 {
		t.Error("SyncReport entry for CHANGED node: FieldDiffs is empty, want at least Claim/Owner")
	}

	updated := g.Requirements[0]
	if updated.Claim != reg.Claim {
		t.Errorf("updated node Claim = %q, want registry's %q", updated.Claim, reg.Claim)
	}
	if updated.Owner != reg.Owner {
		t.Errorf("updated node Owner = %q, want registry's %q", updated.Owner, reg.Owner)
	}

	// Event fields pass through untouched (except History, which gets an
	// append).
	if updated.LastReviewedAt != "2025-05-05" {
		t.Errorf("updated node LastReviewedAt = %q, want unchanged %q", updated.LastReviewedAt, "2025-05-05")
	}
	if len(updated.Evidence) != 1 || updated.Evidence[0] != "kept-evidence" {
		t.Errorf("updated node Evidence = %v, want unchanged [kept-evidence]", updated.Evidence)
	}

	if len(updated.History) != 2 {
		t.Fatalf("updated node History = %+v, want exactly 2 entries (1 preserved + 1 appended)", updated.History)
	}
	if updated.History[0] != priorEntry {
		t.Errorf("updated node History[0] = %+v, want unchanged prior entry %+v", updated.History[0], priorEntry)
	}
	if updated.History[1].At != "2026-07-23" {
		t.Errorf("updated node History[1].At = %q, want %q", updated.History[1].At, "2026-07-23")
	}
	if !strings.Contains(updated.History[1].Summary, "Claim") {
		t.Errorf("updated node History[1].Summary = %q, want it to mention the changed Claim field", updated.History[1].Summary)
	}
}

// TestSyncGraph_UnchangedNodeNotInReport proves a node that already
// structurally matches the registry gets no History append and does not
// appear in the SyncReport.
func TestSyncGraph_UnchangedNodeNotInReport(t *testing.T) {
	reg := firstRegisteredRequirement(t)

	priorEntry := ontology.HistoryEntry{At: "2020-01-01", Summary: "some earlier event"}
	existing := reg // exact structural copy
	existing.History = []ontology.HistoryEntry{priorEntry}

	g := &ontology.Graph{Requirements: []ontology.Requirement{existing}}

	report, err := SyncGraph(g, "2026-07-23")
	if err != nil {
		t.Fatalf("SyncGraph: %v", err)
	}

	for _, e := range report.Entries {
		if e.ID == reg.ID {
			t.Fatalf("SyncReport: unchanged node %q unexpectedly appears in report: %+v", reg.ID, e)
		}
	}

	updated := g.Requirements[0]
	if len(updated.History) != 1 || updated.History[0] != priorEntry {
		t.Errorf("unchanged node History = %+v, want untouched [prior entry] only", updated.History)
	}
}

// TestSyncGraph_UnregisteredNodeUntouchedAndNotInReport proves a graph node
// with no matching registry ID is left completely untouched and never
// appears in the SyncReport — deletion/drift-detection is explicitly out of
// this function's scope.
func TestSyncGraph_UnregisteredNodeUntouchedAndNotInReport(t *testing.T) {
	const sentinelClaim = "SENTINEL-must-survive-sync-untouched"
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-not-in-any-registry", Claim: sentinelClaim, Status: ontology.StatusDRAFT},
		},
	}

	report, err := SyncGraph(g, "2026-07-23")
	if err != nil {
		t.Fatalf("SyncGraph: %v", err)
	}

	for _, e := range report.Entries {
		if e.ID == "R-not-in-any-registry" {
			t.Fatalf("SyncReport: unregistered node unexpectedly appears in report: %+v", e)
		}
	}

	var found *ontology.Requirement
	for i := range g.Requirements {
		if g.Requirements[i].ID == "R-not-in-any-registry" {
			found = &g.Requirements[i]
			break
		}
	}
	if found == nil {
		t.Fatal("unregistered node vanished from the graph — SyncGraph must never delete")
	}
	if found.Claim != sentinelClaim {
		t.Errorf("unregistered node Claim = %q, want unchanged %q", found.Claim, sentinelClaim)
	}
	if len(found.History) != 0 {
		t.Errorf("unregistered node History = %+v, want untouched (empty)", found.History)
	}
}

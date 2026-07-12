package query

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

// fixtureGraph is a small synthetic graph — NOT the 380KB hotam-spec-self
// domain — covering every relation shape the query package must resolve:
// forward/backward Relations, shared Assumptions, and Conflict membership.
func fixtureGraph() *ontology.Graph {
	str := func(s string) *string { return &s }
	return &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: "A-shared", Statement: "the shared assumption both requirements rest on", Status: ontology.AssumptionHOLDS, Owner: "tester"},
			{ID: "A-lonely", Statement: "an assumption only R-alpha uses", Status: ontology.AssumptionUNCERTAIN, Owner: "tester"},
		},
		Requirements: []ontology.Requirement{
			{
				ID:             "R-alpha",
				Claim:          "Alpha claims something that is quite long, long enough to exceed the eighty character preview truncation threshold easily",
				Owner:          "tester",
				Status:         ontology.StatusSETTLED,
				Why:            "because alpha needs it",
				Assumptions:    []string{"A-shared", "A-lonely"},
				Relations:      []ontology.Relation{{Kind: "refines", Target: "R-beta"}},
				Enforcement:    ontology.EnforcementENFORCED,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
				EnforcedBy:     []string{"check_alpha"},
				LastReviewedAt: "2026-07-01",
				ReviewAfter:    "2026-08-01",
				Evidence:       []string{"evidence-1"},
				SourceRefs:     []string{"src/alpha.go"},
				History:        []ontology.HistoryEntry{{At: "2026-06-01", Summary: "created", DecidedBy: "tester"}},
			},
			{
				ID:          "R-beta",
				Claim:       "Beta is short",
				Owner:       "other-owner",
				Status:      ontology.StatusSETTLED,
				Why:         "because beta needs it too",
				Assumptions: []string{"A-shared"},
				Enforcement: ontology.EnforcementPROSE,
			},
			{
				ID:          "R-gamma",
				Claim:       "Gamma is draft and unrelated",
				Owner:       "tester",
				Status:      ontology.StatusDRAFT,
				Why:         "gamma reasoning",
				Enforcement: ontology.EnforcementSTRUCTURAL,
			},
		},
		Conflicts: []ontology.Conflict{
			{
				ID:               "C-ab",
				Axis:             "test-axis",
				Context:          "alpha vs beta tension",
				Members:          []string{"R-alpha", "R-beta"},
				Steward:          "tester",
				Lifecycle:        ontology.ConflictDETECTED,
				SharedAssumption: str("A-shared"),
			},
		},
	}
}

func TestShowRequirement(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	c, err := ShowRequirement(g, "R-alpha")
	if err != nil {
		t.Fatalf("ShowRequirement: %v", err)
	}
	if c.ID != "R-alpha" || c.Owner != "tester" || c.Status != ontology.StatusSETTLED {
		t.Errorf("unexpected card: %+v", c)
	}
	if len(c.Relations) != 1 || c.Relations[0].Target != "R-beta" {
		t.Errorf("relations not carried through: %+v", c.Relations)
	}
	if c.LastReviewedAt != "2026-07-01" || c.ReviewAfter != "2026-08-01" {
		t.Errorf("freshness fields missing: %+v", c)
	}
}

func TestShowRequirement_NotFound(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	_, err := ShowRequirement(g, "R-does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing requirement")
	}
	if _, ok := err.(*ErrNotFound); !ok {
		t.Errorf("expected *ErrNotFound, got %T", err)
	}
}

func TestShowConflict(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	c, err := ShowConflict(g, "C-ab")
	if err != nil {
		t.Fatalf("ShowConflict: %v", err)
	}
	if len(c.Members) != 2 {
		t.Errorf("expected 2 members, got %v", c.Members)
	}
}

func TestShowAssumption(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	c, err := ShowAssumption(g, "A-shared")
	if err != nil {
		t.Fatalf("ShowAssumption: %v", err)
	}
	if c.Statement == "" {
		t.Error("expected non-empty statement")
	}
}

func TestShow_DispatchesByPrefix(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	cases := []struct {
		id       string
		wantKind AnchorKind
	}{
		{"R-alpha", KindRequirement},
		{"C-ab", KindConflict},
		{"A-shared", KindAssumption},
	}
	for _, tc := range cases {
		v, err := Show(g, tc.id)
		if err != nil {
			t.Fatalf("Show(%s): %v", tc.id, err)
		}
		switch tc.wantKind {
		case KindRequirement:
			if _, ok := v.(RequirementCard); !ok {
				t.Errorf("Show(%s) = %T, want RequirementCard", tc.id, v)
			}
		case KindConflict:
			if _, ok := v.(ConflictCard); !ok {
				t.Errorf("Show(%s) = %T, want ConflictCard", tc.id, v)
			}
		case KindAssumption:
			if _, ok := v.(AssumptionCard); !ok {
				t.Errorf("Show(%s) = %T, want AssumptionCard", tc.id, v)
			}
		}
	}
}

func TestShow_NotFound(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	_, err := Show(g, "R-nope")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestList_NoFilter(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	items := List(g, ListFilter{})
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].ID != "R-alpha" || items[1].ID != "R-beta" || items[2].ID != "R-gamma" {
		t.Errorf("expected id-sorted order, got %v", items)
	}
}

func TestList_FilterByStatus(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	items := List(g, ListFilter{Status: ontology.StatusSETTLED})
	if len(items) != 2 {
		t.Fatalf("expected 2 SETTLED items, got %d: %v", len(items), items)
	}
}

func TestList_FilterByOwner(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	items := List(g, ListFilter{Owner: "other-owner"})
	if len(items) != 1 || items[0].ID != "R-beta" {
		t.Fatalf("expected only R-beta, got %v", items)
	}
}

func TestList_FilterByEnforcement(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	items := List(g, ListFilter{Enforcement: ontology.EnforcementPROSE})
	if len(items) != 1 || items[0].ID != "R-beta" {
		t.Fatalf("expected only R-beta, got %v", items)
	}
}

func TestList_SummaryTruncation(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	items := List(g, ListFilter{})
	var alpha ListItem
	for _, it := range items {
		if it.ID == "R-alpha" {
			alpha = it
		}
	}
	if len([]rune(alpha.Summary)) > summaryPreviewLen+1 { // +1 for ellipsis rune
		t.Errorf("summary not truncated: %q (%d runes)", alpha.Summary, len([]rune(alpha.Summary)))
	}
}

func TestSearch_RanksIDAboveClaimAboveWhy(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	// "beta" appears in R-beta's id (rank 0) and nowhere else notably.
	results := Search(g, "beta")
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].ID != "R-beta" {
		t.Errorf("expected R-beta to rank first for id match, got %v", results)
	}
}

func TestSearch_MatchesWhy(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	results := Search(g, "gamma reasoning")
	if len(results) != 1 || results[0].ID != "R-gamma" {
		t.Fatalf("expected R-gamma matched via why, got %v", results)
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	results := Search(g, "ALPHA")
	found := false
	for _, r := range results {
		if r.ID == "R-alpha" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected case-insensitive match for R-alpha, got %v", results)
	}
}

func TestSearch_Empty(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	if got := Search(g, ""); got != nil {
		t.Errorf("expected nil for empty search text, got %v", got)
	}
	if got := Search(g, "no-such-text-anywhere"); len(got) != 0 {
		t.Errorf("expected no matches, got %v", got)
	}
}

func TestContext_IncludesRelationsAssumptionsConflicts(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	cc, err := Context(g, "R-alpha")
	if err != nil {
		t.Fatalf("Context: %v", err)
	}
	if cc.Requirement.ID != "R-alpha" {
		t.Errorf("wrong requirement: %+v", cc.Requirement)
	}

	var foundOutgoing bool
	for _, r := range cc.Relations {
		if r.ID == "R-beta" && r.RelKind == "refines" {
			foundOutgoing = true
		}
	}
	if !foundOutgoing {
		t.Errorf("expected outgoing refines->R-beta, got %v", cc.Relations)
	}

	if len(cc.Assumptions) != 2 {
		t.Errorf("expected 2 full assumption texts, got %v", cc.Assumptions)
	}

	if len(cc.Conflicts) != 1 || cc.Conflicts[0].ID != "C-ab" {
		t.Errorf("expected member of C-ab, got %v", cc.Conflicts)
	}

	var sharesWithBeta bool
	for _, s := range cc.SharedAssumptionWith {
		if s.ID == "R-beta" {
			sharesWithBeta = true
		}
	}
	if !sharesWithBeta {
		t.Errorf("expected R-beta in shared-assumption neighbors, got %v", cc.SharedAssumptionWith)
	}
}

func TestContext_IncomingRelation(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	cc, err := Context(g, "R-beta")
	if err != nil {
		t.Fatalf("Context: %v", err)
	}
	var foundIncoming bool
	for _, r := range cc.Relations {
		if r.ID == "R-alpha" && r.RelKind == "refines(in)" {
			foundIncoming = true
		}
	}
	if !foundIncoming {
		t.Errorf("expected incoming refines(in) from R-alpha, got %v", cc.Relations)
	}
}

func TestContext_NotFound(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	_, err := Context(g, "R-missing")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestRelated_Requirement(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	refs, err := Related(g, "R-alpha")
	if err != nil {
		t.Fatalf("Related: %v", err)
	}
	want := map[string]bool{
		"R-beta:refines":       false,
		"A-shared:assumes":     false,
		"A-lonely:assumes":     false,
		"C-ab:conflict_member": false,
	}
	for _, r := range refs {
		key := r.ID + ":" + r.RelKind
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("expected neighbor %q not found in %v", k, refs)
		}
	}
}

func TestRelated_Conflict(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	refs, err := Related(g, "C-ab")
	if err != nil {
		t.Fatalf("Related: %v", err)
	}
	var hasAlpha, hasBeta, hasSharedAssumption bool
	for _, r := range refs {
		if r.ID == "R-alpha" && r.RelKind == "member" {
			hasAlpha = true
		}
		if r.ID == "R-beta" && r.RelKind == "member" {
			hasBeta = true
		}
		if r.ID == "A-shared" && r.RelKind == "shared_assumption" {
			hasSharedAssumption = true
		}
	}
	if !hasAlpha || !hasBeta || !hasSharedAssumption {
		t.Errorf("missing expected neighbors in %v", refs)
	}
}

func TestRelated_Assumption(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	refs, err := Related(g, "A-shared")
	if err != nil {
		t.Fatalf("Related: %v", err)
	}
	var hasAlpha, hasBeta, hasConflict bool
	for _, r := range refs {
		if r.ID == "R-alpha" && r.RelKind == "assumed_by" {
			hasAlpha = true
		}
		if r.ID == "R-beta" && r.RelKind == "assumed_by" {
			hasBeta = true
		}
		if r.ID == "C-ab" && r.RelKind == "shared_assumption_of" {
			hasConflict = true
		}
	}
	if !hasAlpha || !hasBeta || !hasConflict {
		t.Errorf("missing expected neighbors in %v", refs)
	}
}

func TestRelated_NotFound(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	_, err := Related(g, "R-nowhere")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestFormatShow_AllKinds(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	for _, id := range []string{"R-alpha", "C-ab", "A-shared"} {
		v, err := Show(g, id)
		if err != nil {
			t.Fatalf("Show(%s): %v", id, err)
		}
		text, err := FormatShow(v)
		if err != nil {
			t.Fatalf("FormatShow(%s): %v", id, err)
		}
		if text == "" {
			t.Errorf("FormatShow(%s) returned empty text", id)
		}
	}
}

func TestFormatContext_NoPanic(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	cc, err := Context(g, "R-alpha")
	if err != nil {
		t.Fatalf("Context: %v", err)
	}
	text := FormatContext(cc)
	if text == "" {
		t.Fatal("FormatContext returned empty text")
	}
}

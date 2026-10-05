package selfspec

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func fullReqFixture() ontology.Requirement {
	return ontology.Requirement{
		ID:         "R-fixture",
		Claim:      "claim text",
		ClaimTexts: ontology.LocalizedText{"en": "claim text"},
		AtomKind:   "rule",
		Cases: []ontology.CaseDefinition{{
			ID: "case-1", Operation: "parse", Producer: "direct",
			Input: &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: ""},
		}},
		ClauseLinks: []ontology.ClauseLink{{ClauseID: "clause-1", Side: "valid"}},
		Strength:    "MUST",
		Applicability: &ontology.Applicability{
			Operations: []string{"parse"}, Profiles: []string{"standard"}, Features: []string{"roundtrip"},
		},
		Precedence:     []ontology.PrecedenceLink{{Target: "R-other", Scope: "parse"}},
		Owner:          "owner-a",
		Status:         ontology.StatusSETTLED,
		Why:            "why text",
		Assumptions:    []string{"A-one"},
		Relations:      []ontology.Relation{{Kind: "refines", Target: "R-other"}},
		Enforcement:    ontology.EnforcementPROSE,
		EnforcedBy:     []string{"check_one"},
		MTag:           "M1",
		Enforceability: ontology.EnforceabilityENFORCEABLE,
		Summary:        "summary text",
		CreatedAt:      "2026-01-01",
		SettledAt:      "2026-01-02",
		SourceRefs:     []string{"src:1"},
		DeclOrder:      5,
		BlockedOn:      "blocked",
		ImplementedBy:  []string{"pkg:Fn"},
		VerifiedBy:     []string{"pkg:TestFn"},
		SourceLinks:    []ontology.SourceLink{{SourceID: "source-1", Anchor: "L2-L4"}},
		Coverage: &ontology.CoverageDeclaration{
			Status: ontology.CoverageUnverified, Rationale: "fixture has no runtime evidence",
		},
		// Event fields, deliberately set to values that would trip a diff
		// if StructuralFieldDiffs mistakenly compared them.
		LastReviewedAt: "2026-02-01",
		ReviewAfter:    "2026-03-01",
		Evidence:       []string{"evidence-1"},
		History: []ontology.HistoryEntry{
			{At: "2026-01-01", Summary: "created"},
		},
		GateSignoffs: []ontology.GateSignoff{
			{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1"},
		},
	}
}

func TestStructuralFieldDiffs_EmptyWhenIdentical(t *testing.T) {
	reg := fullReqFixture()
	graph := fullReqFixture()
	// Deliberately diverge only event fields — must not affect the result.
	graph.LastReviewedAt = "some-other-date"
	graph.ReviewAfter = "yet-another-date"
	graph.Evidence = []string{"different-evidence"}
	graph.History = nil
	graph.GateSignoffs = nil

	diffs := StructuralFieldDiffs(reg, graph)
	if len(diffs) != 0 {
		t.Fatalf("StructuralFieldDiffs: want empty diff for structurally-identical requirements (event fields differ only), got %+v", diffs)
	}
}

// TestStructuralFieldDiffs_OneFieldAtATime proves every structural field
// StructuralFieldDiffs claims to compare (per its own doc comment) actually
// participates: each subtest mutates exactly ONE structural field on the
// graph copy and asserts StructuralFieldDiffs reports exactly that field.
func TestStructuralFieldDiffs_OneFieldAtATime(t *testing.T) {
	cases := []struct {
		field  string
		mutate func(r *ontology.Requirement)
	}{
		{"Claim", func(r *ontology.Requirement) { r.Claim = "different claim" }},
		{"Owner", func(r *ontology.Requirement) { r.Owner = "owner-b" }},
		{"Status", func(r *ontology.Requirement) { r.Status = ontology.StatusDRAFT }},
		{"Why", func(r *ontology.Requirement) { r.Why = "different why" }},
		{"Assumptions", func(r *ontology.Requirement) { r.Assumptions = []string{"A-two"} }},
		{"Relations", func(r *ontology.Requirement) { r.Relations = []ontology.Relation{{Kind: "depends_on", Target: "R-x"}} }},
		{"Enforcement", func(r *ontology.Requirement) { r.Enforcement = ontology.EnforcementENFORCED }},
		{"EnforcedBy", func(r *ontology.Requirement) { r.EnforcedBy = []string{"check_two"} }},
		{"MTag", func(r *ontology.Requirement) { r.MTag = "M2" }},
		{"Enforceability", func(r *ontology.Requirement) { r.Enforceability = ontology.EnforceabilityINHERENTLY_PROSE }},
		{"Summary", func(r *ontology.Requirement) { r.Summary = "different summary" }},
		{"CreatedAt", func(r *ontology.Requirement) { r.CreatedAt = "2027-01-01" }},
		{"SettledAt", func(r *ontology.Requirement) { r.SettledAt = "2027-01-02" }},
		{"SourceRefs", func(r *ontology.Requirement) { r.SourceRefs = []string{"src:2"} }},
		{"DeclOrder", func(r *ontology.Requirement) { r.DeclOrder = 99 }},
		{"BlockedOn", func(r *ontology.Requirement) { r.BlockedOn = "different-blocker" }},
		{"ImplementedBy", func(r *ontology.Requirement) { r.ImplementedBy = []string{"pkg:Other"} }},
		{"VerifiedBy", func(r *ontology.Requirement) { r.VerifiedBy = []string{"pkg:TestOther"} }},
		{"SourceLinks", func(r *ontology.Requirement) {
			r.SourceLinks = []ontology.SourceLink{{SourceID: "source-2", Anchor: "#alternate"}}
		}},
		{"Coverage", func(r *ontology.Requirement) {
			r.Coverage = &ontology.CoverageDeclaration{Status: ontology.CoverageUnsupported, Rationale: "not supported", Profile: "other"}
		}},
		{"ClaimTexts", func(r *ontology.Requirement) { r.ClaimTexts["en"] = "different claim text" }},
		{"AtomKind", func(r *ontology.Requirement) { r.AtomKind = "" }},
		{"Cases", func(r *ontology.Requirement) { r.Cases = []ontology.CaseDefinition{{ID: "case-2"}} }},
		{"ClauseLinks", func(r *ontology.Requirement) { r.ClauseLinks = []ontology.ClauseLink{{ClauseID: "clause-2"}} }},
		{"Strength", func(r *ontology.Requirement) { r.Strength = "SHOULD" }},
		{"Applicability", func(r *ontology.Requirement) { r.Applicability = &ontology.Applicability{Profiles: []string{"other"}} }},
		{"Precedence", func(r *ontology.Requirement) {
			r.Precedence = []ontology.PrecedenceLink{{Target: "R-other-2", Scope: "parse"}}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			reg := fullReqFixture()
			graph := fullReqFixture()
			tc.mutate(&graph)

			diffs := StructuralFieldDiffs(reg, graph)
			if len(diffs) != 1 {
				t.Fatalf("StructuralFieldDiffs: want exactly 1 diff for field %s, got %d: %+v", tc.field, len(diffs), diffs)
			}
			if diffs[0].Field != tc.field {
				t.Fatalf("StructuralFieldDiffs: want diff for field %q, got %q", tc.field, diffs[0].Field)
			}
			// SyncGraph replaces graph fields FROM the registry, so
			// Old=graph's (mutated) value and New=registry's (unmutated,
			// original) value is the natural "before -> after" direction a
			// History summary renders. reflect.DeepEqual handles both
			// scalar and slice-valued fields uniformly.
			wantOld := reflectField(graph, tc.field)
			wantNew := reflectField(reg, tc.field)
			if !reflect.DeepEqual(diffs[0].Old, wantOld) {
				t.Errorf("field %s: Old = %#v, want %#v (graph's value)", tc.field, diffs[0].Old, wantOld)
			}
			if !reflect.DeepEqual(diffs[0].New, wantNew) {
				t.Errorf("field %s: New = %#v, want %#v (registry's value)", tc.field, diffs[0].New, wantNew)
			}
		})
	}
}

// reflectField returns r's field named name via reflection, used only to
// keep the one-field-at-a-time subtests above from having to hand-write a
// giant switch over every structural field.
func reflectField(r ontology.Requirement, name string) any {
	return reflect.ValueOf(r).FieldByName(name).Interface()
}

// TestStructuralFieldDiffs_MultipleFieldsAtOnce proves several simultaneous
// structural differences are ALL reported, not just the first (the property
// that makes StructuralFieldDiffs a strictly richer replacement for
// internal/invariants' firstStructuralFieldDiff).
func TestStructuralFieldDiffs_MultipleFieldsAtOnce(t *testing.T) {
	reg := fullReqFixture()
	graph := fullReqFixture()
	graph.Claim = "different claim"
	graph.Owner = "owner-b"
	graph.DeclOrder = 42
	graph.ImplementedBy = []string{"pkg:Other"}

	diffs := StructuralFieldDiffs(reg, graph)
	if len(diffs) != 4 {
		t.Fatalf("StructuralFieldDiffs: want exactly 4 diffs, got %d: %+v", len(diffs), diffs)
	}

	seen := make(map[string]bool, len(diffs))
	for _, d := range diffs {
		seen[d.Field] = true
	}
	for _, want := range []string{"Claim", "Owner", "DeclOrder", "ImplementedBy"} {
		if !seen[want] {
			t.Errorf("StructuralFieldDiffs: missing expected diff for field %q, got %+v", want, diffs)
		}
	}
}

// TestStructuralFieldDiffs_ValuesNotTruncated proves long field values pass
// through the diff whole, un-truncated — truncation is explicitly a
// caller's job, never this function's (see its own doc comment).
func TestStructuralFieldDiffs_ValuesNotTruncated(t *testing.T) {
	reg := fullReqFixture()
	graph := fullReqFixture()

	longOld := make([]byte, 5000)
	for i := range longOld {
		longOld[i] = 'a'
	}
	longNew := make([]byte, 5000)
	for i := range longNew {
		longNew[i] = 'b'
	}
	graph.Claim = string(longOld)
	reg.Claim = string(longNew)

	diffs := StructuralFieldDiffs(reg, graph)
	if len(diffs) != 1 {
		t.Fatalf("want exactly 1 diff, got %d", len(diffs))
	}
	gotOld, ok := diffs[0].Old.(string)
	if !ok || len(gotOld) != 5000 {
		t.Fatalf("Old value truncated or wrong type: len=%d ok=%v", len(gotOld), ok)
	}
	gotNew, ok := diffs[0].New.(string)
	if !ok || len(gotNew) != 5000 {
		t.Fatalf("New value truncated or wrong type: len=%d ok=%v", len(gotNew), ok)
	}
}
func TestStructuralFieldDiffs_ObservationPresenceBookkeepingIsNotStructural(t *testing.T) {
	reg := fullReqFixture()
	encoded, err := json.Marshal(reg.Cases)
	if err != nil {
		t.Fatalf("marshal cases: %v", err)
	}
	var decodedCases []ontology.CaseDefinition
	if err := json.Unmarshal(encoded, &decodedCases); err != nil {
		t.Fatalf("unmarshal cases: %v", err)
	}
	graph := fullReqFixture()
	graph.Cases = decodedCases
	if diffs := StructuralFieldDiffs(reg, graph); len(diffs) != 0 {
		t.Fatalf("wire-presence bookkeeping became a structural diff: %+v", diffs)
	}
}

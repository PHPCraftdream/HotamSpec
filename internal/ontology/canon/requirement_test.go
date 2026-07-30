package hotamontology

import (
	"encoding/json"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestRequirement_WhyFieldRoundTripsToOntologyRequirement is task #390's
// (W0.3) narrow, infrastructure-free proof: a canon.Requirement (the type a
// consumer domain's vendored spec/hotamontology package actually compiles
// against) carrying a non-empty Why must survive a JSON marshal/unmarshal
// round trip into the full internal/ontology.Requirement — the EXACT
// mechanism cmd/hotam's domainRegistryFromSubprocess relies on
// (registrydump's subprocess prints json.Marshal(spec.Requirements.All())
// -- a []hotamontology.Requirement -- and sync-domain unmarshals that same
// JSON directly into []ontology.Requirement).
//
// Before this task's fix, canon.Requirement had NO Why field at all, so (a)
// this test could not even be written (the struct literal below would not
// compile) and (b) the JSON registrydump printed never carried a "why" key,
// so ontology.Requirement's Why always unmarshaled to its zero value "" --
// silently discarding any pre-existing hand-authored Why once a domain
// adopted requirements_authority:"code" and ran its first sync.
func TestRequirement_WhyFieldRoundTripsToOntologyRequirement(t *testing.T) {
	const wantWhy = "this component exists because downstream billing depends on its output being deterministic"

	canonReq := Requirement{
		ID:             "R-canon-why-roundtrip",
		Claim:          "the canon mirror shall carry Why",
		Owner:          "fixture-owner",
		Status:         "SETTLED",
		Why:            wantWhy,
		Relations:      []Relation{},
		Assumptions:    []string{},
		Enforcement:    "PROSE",
		EnforcedBy:     []string{},
		Enforceability: "INHERENTLY_PROSE",
		MTag:           "",
		Summary:        "",
		CreatedAt:      "2026-01-01",
		SettledAt:      "2026-01-01",
		SourceRefs:     []string{},
		DeclOrder:      1,
	}

	// Mirrors registrydump/main.go: json.Marshal(spec.Requirements.All()),
	// a []Requirement (this package's minimal mirror type).
	data, err := json.Marshal([]Requirement{canonReq})
	if err != nil {
		t.Fatalf("marshal []canon.Requirement: %v", err)
	}

	// Mirrors domainRegistryFromSubprocess: json.Unmarshal(stdout, &entries)
	// where entries is []ontology.Requirement -- the full engine-side type.
	var decoded []ontology.Requirement
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal into []ontology.Requirement: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("decoded %d entries, want exactly 1", len(decoded))
	}

	if got := decoded[0].Why; got != wantWhy {
		t.Errorf("ontology.Requirement.Why after round trip = %q, want %q — Why must not be silently dropped by the vendored canon.Requirement mirror", got, wantWhy)
	}
	if got := decoded[0].ID; got != canonReq.ID {
		t.Errorf("ontology.Requirement.ID after round trip = %q, want %q", got, canonReq.ID)
	}
}

// TestRequirement_WhyJSONTagMatchesOntology proves the two types' JSON
// wire shape for Why agrees exactly: both marshal to the same "why" key,
// so a canon.Requirement's JSON output and an ontology.Requirement's JSON
// output are structurally interchangeable for this field (the property the
// whole vendoring mechanism depends on for every field, not just Why).
func TestRequirement_WhyJSONTagMatchesOntology(t *testing.T) {
	canonData, err := json.Marshal(Requirement{Why: "x"})
	if err != nil {
		t.Fatalf("marshal canon.Requirement: %v", err)
	}
	var canonMap map[string]any
	if err := json.Unmarshal(canonData, &canonMap); err != nil {
		t.Fatalf("unmarshal canon JSON into map: %v", err)
	}
	if _, ok := canonMap["why"]; !ok {
		t.Fatalf("canon.Requirement JSON has no %q key: %s", "why", canonData)
	}

	ontologyData, err := json.Marshal(ontology.Requirement{Why: "x"})
	if err != nil {
		t.Fatalf("marshal ontology.Requirement: %v", err)
	}
	var ontologyMap map[string]any
	if err := json.Unmarshal(ontologyData, &ontologyMap); err != nil {
		t.Fatalf("unmarshal ontology JSON into map: %v", err)
	}
	if _, ok := ontologyMap["why"]; !ok {
		t.Fatalf("ontology.Requirement JSON has no %q key: %s", "why", ontologyData)
	}

	if canonMap["why"] != ontologyMap["why"] {
		t.Errorf("canon vs ontology \"why\" JSON values disagree: %v vs %v", canonMap["why"], ontologyMap["why"])
	}
}

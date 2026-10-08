package ontology

import (
	"encoding/json"
	"testing"
)

func TestEqualObservedValuesReconcilesAuthoredAndDecodedOracle(t *testing.T) {
	no := false
	authored := ObservedValue{Kind: "object", Fields: map[string]ObservedValue{
		"accepted": {Kind: "bool", ScalarKind: "bool", Bool: &no},
		"bytes":    {Kind: "bytes", Encoding: "base64", Bytes: ""},
	}}
	wire, err := json.Marshal(authored)
	if err != nil {
		t.Fatal(err)
	}
	var recorded ObservedValue
	if err := json.Unmarshal(wire, &recorded); err != nil {
		t.Fatal(err)
	}
	if !EqualObservedValues(&authored, &recorded) {
		t.Fatal("valid recorded oracle differs only by decoder bookkeeping")
	}
	yes := true
	recorded.Fields["accepted"] = ObservedValue{Kind: "bool", ScalarKind: "bool", Bool: &yes}
	if EqualObservedValues(&authored, &recorded) {
		t.Fatal("changed boolean oracle was accepted")
	}
}

func TestEqualObservedValuesRetainsMissingPayloadDistinction(t *testing.T) {
	for _, raw := range []string{`{"kind":"bytes","encoding":"base64"}`, `{"kind":"object"}`} {
		var missing ObservedValue
		if err := json.Unmarshal([]byte(raw), &missing); err != nil {
			t.Fatal(err)
		}
		present := ObservedValue{Kind: missing.Kind, Encoding: missing.Encoding}
		if present.Kind == "object" {
			present.Fields = map[string]ObservedValue{}
		}
		if EqualObservedValues(&present, &missing) {
			t.Fatalf("missing %s payload equated with explicit empty value", missing.Kind)
		}
	}
}

func TestObservedValuePreservesArbitraryObjectKeys(t *testing.T) {
	var value ObservedValue
	if err := json.Unmarshal([]byte(`{"kind":"object","fields":{"":{"kind":"text","text":"empty key"}," ":{"kind":"text","text":"space key"}}}`), &value); err != nil {
		t.Fatal(err)
	}
	if err := value.Validate(); err != nil {
		t.Fatalf("raw object keys must not acquire metadata-name restrictions: %v", err)
	}
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ObservedValue
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if !EqualObservedValues(&value, &decoded) || len(decoded.Fields) != 2 {
		t.Fatal("raw object keys or payloads changed during transport")
	}
}

func TestEqualCaseDefinitionsIgnoresOmittedEmptyOptionalLists(t *testing.T) {
	authored := []CaseDefinition{{
		ID: "boundary", Test: "spec/model/value_test.go:TestBoundary",
		AtomIDs: []string{"R-value"}, Fixtures: []FixtureRef{},
		Conditions: []ConditionEvidence{}, Sides: []string{},
	}}
	wire, err := json.Marshal(authored)
	if err != nil {
		t.Fatal(err)
	}
	var stored []CaseDefinition
	if err := json.Unmarshal(wire, &stored); err != nil {
		t.Fatal(err)
	}
	if !EqualCaseDefinitions(authored, stored) {
		t.Fatal("persisted case creates a false structural change after omitted empty lists")
	}
	stored[0].Operation = "different operation"
	if EqualCaseDefinitions(authored, stored) {
		t.Fatal("actual case metadata change was ignored")
	}
}

func TestRetiredCaseReactivationRestoresLiveValidation(t *testing.T) {
	graph := &Graph{
		Conformance: &ConformanceConfig{RuleCases: true},
		Requirements: []Requirement{{
			ID: "R-retired", Status: StatusREJECTED, AtomKind: "rule",
			Cases: []CaseDefinition{{ID: "C-history"}},
		}},
	}
	if issues := ValidateConformance(graph); len(issues) != 0 {
		t.Fatalf("historical descriptor acquired a live execution obligation: %+v", issues)
	}
	graph.Requirements[0].Status = StatusSETTLED
	found := false
	for _, issue := range ValidateConformance(graph) {
		if issue.Kind == "case" && issue.ID == "C-history" {
			found = true
		}
	}
	if !found {
		t.Fatal("reactivating a historical case failed to restore its live descriptor obligations")
	}
}

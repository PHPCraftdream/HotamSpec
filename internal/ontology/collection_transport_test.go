package ontology

import (
	"encoding/json"
	"testing"

	hotamontology "github.com/PHPCraftdream/HotamSpec/internal/ontology/canon"
)

func TestOrderedCollectionTransportRetainsExactPayloadsAndKinds(t *testing.T) {
	no, empty, line := false, "", 1
	value := ObservedValue{Kind: "array", Items: []ObservedValue{
		{Kind: "object", Fields: map[string]ObservedValue{}},
		{Kind: "array", Items: []ObservedValue{}},
		{Kind: "text", Text: &empty, ScalarKind: "string"},
		{Kind: "bool", Bool: &no, ScalarKind: "bool"},
		{Kind: "integer", Integer: "9223372036854775807", ScalarKind: "integer"},
		{Kind: "integer", Integer: "-9223372036854775808", ScalarKind: "integer"},
		{Kind: "integer", Integer: "18446744073709551616", ScalarKind: "integer"},
		{Kind: "float", FloatBits: "8000000000000000", ScalarKind: "float64"},
		{Kind: "float", FloatBits: "7ff8000000000042", ScalarKind: "float64"},
		{Kind: "float", FloatBits: "fff0000000000000", ScalarKind: "float64"},
		{Kind: "bytes", Encoding: "base64", Bytes: "/wA="},
		{Kind: "diagnostic", Diagnostic: &DiagnosticValue{Code: "InvalidUTF8", Class: &empty, Line: &line, Span: &ByteSpan{Start: 0, End: 1}}},
	}}
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var mirror hotamontology.ObservedValue
	if err := json.Unmarshal(wire, &mirror); err != nil {
		t.Fatal(err)
	}
	wire, err = json.Marshal(mirror)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ObservedValue
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
	if !EqualObservedValues(&value, &decoded) {
		t.Fatalf("collection changed across engine/mirror transport: %s", wire)
	}
	clone := CloneObservedValue(&decoded)
	clone.Items[0].Fields["new"] = ObservedValue{Kind: "null"}
	clone.Items[11].Diagnostic.Span.End = 2
	if len(decoded.Items[0].Fields) != 0 || decoded.Items[11].Diagnostic.Span.End != 1 || EqualObservedValues(clone, &decoded) {
		t.Fatal("cloned arrays alias nested object or diagnostic payloads")
	}
	reordered := CloneObservedValue(&decoded)
	reordered.Items[0], reordered.Items[1] = reordered.Items[1], reordered.Items[0]
	if EqualObservedValues(reordered, &decoded) {
		t.Fatal("ordered array comparison ignored item position or empty collection kind")
	}
}

func TestCollectionDecoderRejectsWrongShapesAndValidatorRejectsWrongKinds(t *testing.T) {
	for _, raw := range []string{
		`{"kind":"array","items":null}`,
		`{"kind":"array","items":{}}`,
		`{"kind":"array","items":[null]}`,
		`{"kind":"array","items":[{"kind":"null","extra":0}]}`,
		`{"kind":"object","fields":[]}`,
		`{"kind":"object","fields":null}`,
	} {
		var value ObservedValue
		if err := json.Unmarshal([]byte(raw), &value); err == nil {
			t.Errorf("strict decoder accepted malformed collection: %s", raw)
		}
		var mirror hotamontology.ObservedValue
		if err := json.Unmarshal([]byte(raw), &mirror); err == nil {
			t.Errorf("mirror decoder accepted malformed collection: %s", raw)
		}
	}
	for _, raw := range []string{
		`{"kind":"array"}`,
		`{"kind":"array","fields":{}}`,
		`{"kind":"array","items":[],"fields":{}}`,
		`{"kind":"array","items":[],"scalar_kind":"array"}`,
		`{"kind":"object","items":[]}`,
		`{"kind":"array","items":[{"kind":"integer","integer":"1.5"}]}`,
	} {
		var value ObservedValue
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			t.Fatal(err)
		}
		if err := value.Validate(); err == nil {
			t.Errorf("validator accepted malformed collection payload: %s", raw)
		}
	}
	var missing ObservedValue
	if err := json.Unmarshal([]byte(`{"kind":"array"}`), &missing); err != nil {
		t.Fatal(err)
	}
	empty := ObservedValue{Kind: "array", Items: []ObservedValue{}}
	if EqualObservedValues(&missing, &empty) {
		t.Fatal("absent array payload became explicitly empty")
	}
}

func TestDocumentSchemaAndCaseScopeRetainDeclarationsWithoutObservedValues(t *testing.T) {
	config := ConformanceConfig{DocumentSections: []DocumentSection{{
		ID: "values", Order: 5, TitleTexts: LocalizedText{"en": "Values"},
		Blocks: []NormativeBlock{{ID: "values-root", TextRef: "spec/model/text.go:Root", Role: "normative", ClauseIDs: []string{"values.root"}, Examples: []DocumentExample{{CaseID: "root-object", ComparisonNames: []string{"parse-value"}}}}},
	}}}
	wire, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ConformanceConfig
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	block := decoded.DocumentSections[0].Blocks[0]
	if block.ID != "values-root" || block.TextRef != "spec/model/text.go:Root" || block.ClauseIDs[0] != "values.root" || block.Examples[0].CaseID != "root-object" {
		t.Fatalf("authored document bindings changed: %s", wire)
	}
	for _, raw := range []string{
		`{"document_sections":null}`,
		`{"document_sections":[{"id":"s","order":1,"title_texts":{"en":"S"},"unknown":true}]}`,
		`{"document_sections":[{"id":"s","order":1,"title_texts":{"en":"S"},"blocks":[{"id":"b","text_ref":"r","role":"normative","clause_ids":null}]}]}`,
	} {
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			t.Errorf("document schema accepted null or unknown metadata: %s", raw)
		}
	}
	for _, scope := range [][]string{nil, {}, {"values.root"}} {
		authored := []CaseDefinition{{ID: "root-object", ClauseIDs: scope}}
		wire, err := json.Marshal(authored)
		if err != nil {
			t.Fatal(err)
		}
		var mirror []hotamontology.CaseDefinition
		if err := json.Unmarshal(wire, &mirror); err != nil {
			t.Fatal(err)
		}
		wire, err = json.Marshal(mirror)
		if err != nil {
			t.Fatal(err)
		}
		var cases []CaseDefinition
		if err := json.Unmarshal(wire, &cases); err != nil {
			t.Fatal(err)
		}
		if !EqualCaseDefinitions(authored, cases) || cases[0].Input != nil || cases[0].Expected != nil {
			t.Fatalf("declared scope presence or observation separation changed: %s", wire)
		}
		clone := CloneCaseDefinitions(cases)
		if len(scope) > 0 {
			clone[0].ClauseIDs[0] = "different"
			if cases[0].ClauseIDs[0] != "values.root" {
				t.Fatal("case clone aliases proof scope")
			}
		}
	}
}

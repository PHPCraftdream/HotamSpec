package hotamspec

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestEmptyTypedPayloadsRemainExplicitAndSchemaValid(t *testing.T) {
	cases := []struct {
		name  string
		value TypedValue
	}{
		{"empty bytes", Bytes([]byte{})},
		{"empty object", Object(map[string]TypedValue{})},
		{"empty array", Array(nil)},
		{"diagnostic", Diagnostic(DiagnosticValue{Code: "UnclosedCompound"})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := json.Marshal(c.value)
			if err != nil {
				t.Fatal(err)
			}
			var decoded ontology.ObservedValue
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			if err := decoded.Validate(); err != nil {
				t.Fatalf("valid producer payload became invalid in transport: %s: %v", data, err)
			}
			if c.value.Kind == "object" && decoded.Fields == nil {
				t.Fatal("empty object became absent")
			}
			if c.value.Kind == "array" && decoded.Items == nil {
				t.Fatal("empty array became absent")
			}
		})
	}
	var absent ontology.ObservedValue
	if err := json.Unmarshal([]byte(`{"kind":"bytes","encoding":"base64"}`), &absent); err != nil {
		t.Fatal(err)
	}
	if absent.Validate() == nil {
		t.Fatal("absent byte payload was treated as an empty byte sequence")
	}
}

func TestRecorderArraysUseExactOrderedDefensiveCopies(t *testing.T) {
	items := make([]TypedValue, 12)
	for i := range items {
		items[i] = Integer(fmt.Sprintf("%d", i))
	}
	items[10] = Object(map[string]TypedValue{"kind": Text("Object"), "entries": Text("kept")})
	items[11] = Array([]TypedValue{Float64Bits(math.Float64frombits(0x8000000000000000)), Bytes([]byte{0xff})})
	value := Array(items)
	items[10].Fields["entries"] = Text("changed")
	items[11].Items[0] = Float64Bits(0)
	if !Observe("defensive copy", nil, value.Items[10].Fields["entries"], Text("kept")).Passed ||
		value.Items[11].Items[0].FloatBits != "8000000000000000" {
		t.Fatal("array constructor aliases nested object or array payloads")
	}
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded TypedValue
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if !Observe("typed round trip", nil, value, decoded).Passed {
		t.Fatalf("decoder bookkeeping changed exact typed comparison: %s", wire)
	}
	if decoded.Items[9].Integer != "9" || decoded.Items[10].Kind != "object" ||
		decoded.Items[10].Fields["kind"].Text == nil || *decoded.Items[10].Fields["kind"].Text != "Object" {
		t.Fatalf("array order or ordinary user object fields changed: %s", wire)
	}
	reordered := cloneTypedValue(decoded)
	reordered.Items[9], reordered.Items[10] = reordered.Items[10], reordered.Items[9]
	if Observe("position matters", nil, value, reordered).Passed ||
		Observe("empty kind matters", nil, Array(nil), Object(nil)).Passed {
		t.Fatal("array comparator ignored order or collection kind")
	}
	implicit := Observe("native array", nil, []int{0, 1, 2}, Array([]TypedValue{Integer("0"), Integer("1"), Integer("2")}))
	if !implicit.Passed || implicit.RawActual.Kind != "array" || len(implicit.RawActual.Items) != 3 {
		t.Fatalf("typed native slice fell back to flattened text: %+v", implicit)
	}
	fixed := Observe("fixed array", nil, [2]uint64{0, ^uint64(0)}, Array([]TypedValue{Integer("0"), Integer("18446744073709551615")}))
	if !fixed.Passed {
		t.Fatalf("fixed array lost exact integer boundary: %+v", fixed)
	}
}

func TestRecorderCollectionDecoderFailsClosedAndPreservesAbsence(t *testing.T) {
	for _, raw := range []string{
		`{"kind":"array","items":null}`,
		`{"kind":"array","items":{}}`,
		`{"kind":"array","items":[null]}`,
		`{"kind":"array","items":[{"kind":"null","unknown":1}]}`,
		`{"kind":"object","fields":null}`,
		`{"kind":"object","fields":[]}`,
		`{"kind":"object","fields":{},"entries":{}}`,
		`{"kind":"array","items":[{"kind":"diagnostic","diagnostic":{"code":"parse","reason":null}}]}`,
		`{"kind":"array","items":[{"kind":"diagnostic","diagnostic":{"code":"parse","span":{"start":0}}}]}`,
		`{"kind":"array","items":[{"kind":"diagnostic","diagnostic":{}}]}`,
	} {
		var value TypedValue
		if err := json.Unmarshal([]byte(raw), &value); err == nil {
			t.Errorf("recorder accepted malformed collection: %s", raw)
		}
	}
	for _, malformed := range []TypedValue{
		{Kind: "array"},
		{Kind: "object"},
		{Kind: "array", Items: []TypedValue{}, Fields: map[string]TypedValue{}},
		{Kind: "object", Fields: map[string]TypedValue{}, Items: []TypedValue{}},
		{Kind: "array", Items: []TypedValue{}, ScalarKind: "array"},
		{Kind: "array", Items: []TypedValue{{Kind: "object"}}},
	} {
		if Observe("malformed collections cannot pass", nil, malformed, malformed).Passed {
			t.Errorf("matching malformed collection payload fabricated PASS: %+v", malformed)
		}
	}
	for _, sample := range []struct {
		raw   string
		empty TypedValue
	}{
		{`{"kind":"array"}`, Array(nil)},
		{`{"kind":"object"}`, Object(nil)},
		{`{"kind":"bytes","encoding":"base64"}`, Bytes(nil)},
	} {
		var missing TypedValue
		if err := json.Unmarshal([]byte(sample.raw), &missing); err != nil {
			t.Fatal(err)
		}
		if Observe("absence is not empty", nil, missing, sample.empty).Passed {
			t.Fatalf("missing payload equated with explicit empty: %s", sample.raw)
		}
	}
}

func TestCaseContextRetainsExplicitClauseScopeAndClonesIt(t *testing.T) {
	for _, scope := range [][]string{nil, {}, {"values.object"}} {
		context := CaseContext{ID: "object", ClauseIDs: scope}
		copied := cloneCaseContext(&context)
		wire, err := json.Marshal(copied)
		if err != nil {
			t.Fatal(err)
		}
		var decoded ontology.CaseDefinition
		if err := json.Unmarshal(wire, &decoded); err != nil {
			t.Fatal(err)
		}
		if (decoded.ClauseIDs == nil) != (scope == nil) || !slices.Equal(decoded.ClauseIDs, scope) {
			t.Fatalf("proof scope presence lost: %s", wire)
		}
		if len(scope) > 0 {
			copied.ClauseIDs[0] = "other"
			if context.ClauseIDs[0] != "values.object" {
				t.Fatal("recorded context aliases authored clause scope")
			}
		}
	}
}

package gate

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestJSON5ValuesPreserveExactScalars(t *testing.T) {
	for _, original := range []string{"", "a\x00b", "\ufeffkey: value", "\U0010ffff", "quote\" and slash\\"} {
		value := ontology.ObservedValue{Kind: "text", Text: &original}
		var decoded string
		if err := json.Unmarshal([]byte(ReadableObservedValue(&value)), &decoded); err != nil || decoded != original {
			t.Fatalf("string changed in JSON5: got=%q want=%q err=%v", decoded, original, err)
		}
	}
	integer := ontology.ObservedValue{Kind: "integer", Integer: "9223372036854775807"}
	formatted := ReadableObservedValue(&integer)
	var decimal string
	literal := formatted[strings.Index(formatted, "*/")+2:]
	if err := json.Unmarshal([]byte(literal), &decimal); err != nil || decimal != integer.Integer {
		t.Fatalf("large Integer lost precision: %s (%v)", formatted, err)
	}
	negativeZero := ontology.ObservedValue{Kind: "float", FloatBits: "8000000000000000"}
	formatted = ReadableObservedValue(&negativeZero)
	literal = strings.TrimSpace(formatted[strings.Index(formatted, "*/")+2:])
	zero, err := strconv.ParseFloat(literal, 64)
	if err != nil || math.Float64bits(zero) != 0x8000000000000000 || !strings.Contains(formatted, negativeZero.FloatBits) {
		t.Fatalf("negative zero lost its sign or bits: %s", formatted)
	}
	invalid := ontology.ObservedValue{Kind: "bytes", Bytes: base64.StdEncoding.EncodeToString([]byte{0xff, 0x00})}
	if source, ok := readableSourceBytes(&invalid); ok || source != "" {
		t.Fatal("invalid UTF-8 was presented as Ktav source")
	}
}

func TestJSON5ExplicitArraysPreserveIndexOrderAndEmptyKind(t *testing.T) {
	items := make([]ontology.ObservedValue, 12)
	for index := range items {
		items[index] = ontology.ObservedValue{Kind: "integer", Integer: strconv.Itoa(index)}
	}
	array := ontology.ObservedValue{Kind: "array", Items: items}
	var decoded []int
	if err := json.Unmarshal([]byte(ReadableObservedValue(&array)), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(items) {
		t.Fatalf("array lost elements: %v", decoded)
	}
	for index, value := range decoded {
		if value != index {
			t.Fatalf("array index %d moved: %v", index, decoded)
		}
	}
	array.Items = []ontology.ObservedValue{}
	if rendered := ReadableObservedValue(&array); rendered != "[]" {
		t.Fatalf("empty array became an object: %s", rendered)
	}
	object := ontology.ObservedValue{Kind: "object", Fields: map[string]ontology.ObservedValue{}}
	if rendered := ReadableObservedValue(&object); rendered != "{}" {
		t.Fatalf("empty object became an array: %s", rendered)
	}
}

func TestJSON5ObjectKeysNeverSelectCollectionType(t *testing.T) {
	objectLabel, arrayLabel, kept := "Object", "Array", "value"
	object := ontology.ObservedValue{Kind: "object", Fields: map[string]ontology.ObservedValue{
		"kind": {Kind: "text", Text: &objectLabel},
		"entries": {Kind: "object", Fields: map[string]ontology.ObservedValue{
			"keep": {Kind: "text", Text: &kept},
		}},
	}}
	rendered := ReadableObservedValue(&object)
	for _, field := range []string{`kind: "Object"`, "entries: {", `keep: "value"`} {
		if !strings.Contains(rendered, field) {
			t.Fatalf("ordinary object field %q was lost: %s", field, rendered)
		}
	}
	object.Fields = map[string]ontology.ObservedValue{
		"kind":   {Kind: "text", Text: &arrayLabel},
		"length": {Kind: "integer", Integer: "0"},
		"items":  {Kind: "object", Fields: map[string]ontology.ObservedValue{}},
	}
	rendered = ReadableObservedValue(&object)
	for _, field := range []string{`kind: "Array"`, "length: 0", "items: {}"} {
		if !strings.Contains(rendered, field) {
			t.Fatalf("ordinary object field %q selected a transport wrapper: %s", field, rendered)
		}
	}
}

func TestJSON5IdentifierKeyBoundaries(t *testing.T) {
	for _, key := range []string{"app", "server", "port", "default", "$value", "_id", "ключ", "配置", "a0", "a\u200c", "a\u0301"} {
		if !json5Identifier(key) {
			t.Fatalf("valid IdentifierName was quoted: %q", key)
		}
	}
	for _, key := range []string{"", "0app", "two words", "with-dash", "a.b", "quote\"", "\u0301a", "line\nbreak", "\ufeffkey"} {
		if json5Identifier(key) {
			t.Fatalf("non-IdentifierName would be emitted bare: %q", key)
		}
	}
}

func TestJSON5IntegerPrecisionBoundaries(t *testing.T) {
	for _, decimal := range []string{"9007199254740991", "-9007199254740991", "0"} {
		value := ontology.ObservedValue{Kind: "integer", Integer: decimal}
		if rendered := ReadableObservedValue(&value); rendered != decimal {
			t.Fatalf("safe integer changed: %s => %s", decimal, rendered)
		}
	}
	for _, decimal := range []string{"9007199254740992", "-9007199254740992", "9223372036854775807", "-9223372036854775808", "18446744073709551616"} {
		value := ontology.ObservedValue{Kind: "integer", Integer: decimal}
		rendered := ReadableObservedValue(&value)
		var exact string
		literal := rendered[strings.Index(rendered, "*/")+2:]
		if err := json.Unmarshal([]byte(literal), &exact); err != nil || exact != decimal {
			t.Fatalf("integer rounded at boundary: %s => %s (%v)", decimal, rendered, err)
		}
	}
}

func TestDocumentValuesKeepSemanticsAndEvidenceKeepsDiagnostics(t *testing.T) {
	for _, sample := range []struct {
		bits string
		want string
	}{
		{"8000000000000000", "-0.0"},
		{"7ff0000000000000", "Infinity"},
		{"fff0000000000000", "-Infinity"},
		{"7ff8000000000042", "NaN"},
	} {
		value := ontology.ObservedValue{Kind: "array", Items: []ontology.ObservedValue{{Kind: "float", FloatBits: sample.bits}}}
		document, err := ReadableObservedValueForDocument(&value)
		if err != nil {
			t.Fatal(err)
		}
		evidence := ReadableObservedValue(&value)
		if !strings.Contains(document, sample.want) || strings.Contains(document, sample.bits) || !strings.Contains(evidence, sample.bits) {
			t.Fatalf("semantic/evidence float separation failed: document=%s evidence=%s", document, evidence)
		}
	}
	invalid := ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: "/wA="}
	document, err := ReadableObservedValueForDocument(&invalid)
	if !errors.Is(err, ErrOpaqueDocumentBytes) || document != "" ||
		!strings.Contains(ReadableObservedValue(&invalid), "FF 00") {
		t.Fatalf("opaque bytes were invented as a JSON5 value or lost from evidence: document=%q err=%v", document, err)
	}
	line := 12
	value := ontology.ObservedValue{Kind: "diagnostic", Diagnostic: &ontology.DiagnosticValue{
		Code: "InvalidUTF8", Line: &line, Span: &ontology.ByteSpan{Start: 10, End: 11},
	}}
	document, err = ReadableObservedValueForDocument(&value)
	if err != nil {
		t.Fatal(err)
	}
	evidence := ReadableObservedValue(&value)
	if !strings.Contains(document, "InvalidUTF8") || strings.Contains(document, `"line"`) ||
		strings.Contains(document, `"span"`) || !strings.Contains(evidence, `"line": 12`) ||
		value.Diagnostic.Line == nil || value.Diagnostic.Span == nil {
		t.Fatalf("reader projection mutated or exposed diagnostic offsets: document=%s evidence=%s value=%+v", document, evidence, value)
	}
}

func TestDocumentValuesRejectMalformedAndOpaquePayloadsBeforeRendering(t *testing.T) {
	zeroLine := 0
	for _, value := range []*ontology.ObservedValue{
		nil,
		{Kind: "float", FloatBits: "not-binary64"},
		{Kind: "bytes", Encoding: "base64", Bytes: "***"},
		{Kind: "array"},
		{Kind: "object"},
		{Kind: "object", Fields: map[string]ontology.ObservedValue{"missing": {Kind: "bool"}}},
		{Kind: "diagnostic", Diagnostic: &ontology.DiagnosticValue{Code: "parse", Line: &zeroLine}},
		{Kind: "array", Items: []ontology.ObservedValue{
			{Kind: "bytes", Encoding: "base64", Bytes: "/wA="},
			{Kind: "float", FloatBits: "invalid"},
		}},
	} {
		rendered, err := ReadableObservedValueForDocument(value)
		if err == nil || errors.Is(err, ErrOpaqueDocumentBytes) || rendered != "" {
			t.Errorf("malformed selected value was suppressed as opaque bytes or invented JSON5: value=%+v rendered=%q err=%v", value, rendered, err)
		}
	}
	opaque := ontology.ObservedValue{Kind: "object", Fields: map[string]ontology.ObservedValue{
		"valid":  {Kind: "integer", Integer: "1"},
		"source": {Kind: "array", Items: []ontology.ObservedValue{{Kind: "bytes", Encoding: "base64", Bytes: "/wA="}}},
	}}
	rendered, err := ReadableObservedValueForDocument(&opaque)
	if !errors.Is(err, ErrOpaqueDocumentBytes) || rendered != "" {
		t.Fatalf("nested opaque bytes leaked a partial JSON5 value rather than requiring typed prose/evidence: rendered=%q err=%v", rendered, err)
	}
}

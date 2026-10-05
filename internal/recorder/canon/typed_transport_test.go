package hotamspec

import (
	"encoding/json"
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

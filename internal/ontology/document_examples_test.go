package ontology

import (
	"encoding/json"
	"testing"

	hotamontology "github.com/PHPCraftdream/HotamSpec/internal/ontology/canon"
)

func TestDocumentExampleDecodersRejectAbsentEmptyAndAmbiguousSelectors(t *testing.T) {
	decoders := map[string]func([]byte) error{
		"engine":        func(data []byte) error { var value DocumentExample; return json.Unmarshal(data, &value) },
		"domain mirror": func(data []byte) error { var value hotamontology.DocumentExample; return json.Unmarshal(data, &value) },
	}
	invalid := []string{
		`{"comparison_names":["semantic-value"]}`,
		`{"case_id":null,"comparison_names":["semantic-value"]}`,
		`{"case_id":"","comparison_names":["semantic-value"]}`,
		`{"case_id":"case","comparison_names":null}`,
		`{"case_id":"case"}`,
		`{"case_id":"case","comparison_names":[]}`,
		`{"case_id":"case","comparison_names":[""]}`,
		`{"case_id":"case","comparison_names":["semantic-value","semantic-value"]}`,
		`{"case_id":"case","comparison_names":[" semantic-value"]}`,
		`{"case_id":"case","comparison_names":"semantic-value"}`,
		`{"case_id":"case","comparison_names":["semantic-value"],"all_comparisons":true}`,
	}
	for label, decode := range decoders {
		t.Run(label, func(t *testing.T) {
			// A valid authored selection is the control for each rejected mutation.
			if err := decode([]byte(`{"case_id":"case","comparison_names":["semantic-value","canonical-bytes"]}`)); err != nil {
				t.Fatal(err)
			}
			for _, raw := range invalid {
				if err := decode([]byte(raw)); err == nil {
					t.Errorf("ambiguous or absent explicit selection was accepted: %s", raw)
				}
			}
		})
	}
}

func TestNormativeBlockDecodersRefuseObsoleteWholeCaseSelection(t *testing.T) {
	decoders := map[string]func([]byte) error{
		"engine":        func(data []byte) error { var value NormativeBlock; return json.Unmarshal(data, &value) },
		"domain mirror": func(data []byte) error { var value hotamontology.NormativeBlock; return json.Unmarshal(data, &value) },
	}
	for label, decode := range decoders {
		t.Run(label, func(t *testing.T) {
			if err := decode([]byte(`{"id":"b","text_ref":"r","role":"normative","examples":[{"case_id":"case","comparison_names":["semantic-value"]}]}`)); err != nil {
				t.Fatal(err)
			}
			for _, raw := range []string{
				`{"id":"b","text_ref":"r","role":"normative","example_case_ids":["case"]}`,
				`{"id":"b","text_ref":"r","role":"normative","example_case_ids":["case"],"examples":[{"case_id":"case","comparison_names":["semantic-value"]}]}`,
				`{"id":"b","text_ref":"r","role":"normative","examples":null}`,
				`{"id":"b","text_ref":"r","role":"normative","examples":[{"case_id":"case","comparison_names":null}]}`,
			} {
				if err := decode([]byte(raw)); err == nil {
					t.Errorf("obsolete whole-case or null comparison selection was accepted: %s", raw)
				}
			}
		})
	}
}

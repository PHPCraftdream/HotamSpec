package gocode

import (
	"errors"
	"testing"
)

// TestToPascalCase_ContractExamples exercises the exact worked examples in
// GEN-CODE-CONTRACT.md §4.3: "ид_фт" -> "IdFT", "исходный_номер_ott" ->
// "SourceNumberOTT", "на-gate" -> "AtGate".
func TestToPascalCase_ContractExamples(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"abbreviation after glossary word", "ид_фт", "IdFT"},
		{"glossary word + glossary word + ASCII passthrough abbreviation", "исходный_номер_ott", "SourceNumberOTT"},
		{"glossary word + ASCII passthrough word, hyphen separator", "на-gate", "AtGate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ToPascalCase(tc.in)
			if err != nil {
				t.Fatalf("ToPascalCase(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ToPascalCase(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestToCamelCase_ContractExamples(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// First word lower-cased; the abbreviation stays upper-case verbatim
		// per contract §4 step 2 (transliterated, not re-cased).
		{"ид_фт", "idFT"},
		{"исходный_номер_ott", "sourceNumberOTT"},
		{"на-gate", "atGate"},
	}
	for _, tc := range cases {
		got, err := ToCamelCase(tc.in)
		if err != nil {
			t.Fatalf("ToCamelCase(%q) unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("ToCamelCase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestToPascalCase_UnknownTerm verifies contract §4 step 6: a name part
// missing from both the glossary and the abbreviation table (and not ASCII)
// is a hard, explicit generation error naming the exact unrecognized part —
// never a silent fallback.
func TestToPascalCase_UnknownTerm(t *testing.T) {
	_, err := ToPascalCase("совершенно_неизвестный_термин")
	if err == nil {
		t.Fatal("expected error for unknown term, got nil")
	}
	var unknown *UnknownTermError
	if !errors.As(err, &unknown) {
		t.Fatalf("expected *UnknownTermError, got %T: %v", err, err)
	}
	if unknown.Term != "совершенно" {
		t.Errorf("expected first unrecognized part %q, got %q", "совершенно", unknown.Term)
	}
	if unknown.Source != "совершенно_неизвестный_термин" {
		t.Errorf("expected Source to be the full original name, got %q", unknown.Source)
	}
}

// TestToPascalCase_UnknownTerm_MidComposite checks the error names the
// specific failing part even when earlier parts resolve fine, so a caller
// debugging a large composite name isn't left guessing which word broke.
func TestToPascalCase_UnknownTerm_MidComposite(t *testing.T) {
	_, err := ToPascalCase("ид_бла_фт")
	var unknown *UnknownTermError
	if !errors.As(err, &unknown) {
		t.Fatalf("expected *UnknownTermError, got %T: %v", err, err)
	}
	if unknown.Term != "бла" {
		t.Errorf("expected unrecognized part %q, got %q", "бла", unknown.Term)
	}
}

func TestToPascalCase_PureASCIIPassthrough(t *testing.T) {
	got, err := ToPascalCase("api_dto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ApiDto" {
		t.Errorf("ToPascalCase(%q) = %q, want %q", "api_dto", got, "ApiDto")
	}
}

func TestToPascalCase_SingleAbbreviation(t *testing.T) {
	got, err := ToPascalCase("отт")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "OTT" {
		t.Errorf("ToPascalCase(%q) = %q, want %q", "отт", got, "OTT")
	}
}

func TestToPascalCase_WordOrderPreserved(t *testing.T) {
	// резюме_главный_риск -> summary + main + risk, in that order (no
	// reordering, per contract §4 step 5).
	got, err := ToPascalCase("резюме_главный_риск")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "SummaryMainRisk" {
		t.Errorf("ToPascalCase(%q) = %q, want %q", "резюме_главный_риск", got, "SummaryMainRisk")
	}
}

// TestToKebabCase_ContractExamples exercises the exact worked examples in
// GEN-CODE-CONTRACT.md §4.3: "черновик" -> "draft", "на-gate" -> "at-gate",
// "утвердить-pm" -> "approve-pm", "ид_фт" -> "id-ft". Unlike
// ToPascalCase/ToCamelCase, abbreviation parts are lower-cased too (contract
// §4.3: "все части lower-case (включая аббревиатуры)").
func TestToKebabCase_ContractExamples(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"черновик", "draft"},
		{"на-gate", "at-gate"},
		{"утвердить-pm", "approve-pm"},
		{"ид_фт", "id-ft"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ToKebabCase(tc.in)
			if err != nil {
				t.Fatalf("ToKebabCase(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ToKebabCase(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestToKebabCase_UnknownTerm verifies ToKebabCase shares resolveParts'
// error behavior (contract §4 step 6): an unrecognized part is a loud,
// named error, not a silent fallback, exactly like ToPascalCase/ToCamelCase.
func TestToKebabCase_UnknownTerm(t *testing.T) {
	_, err := ToKebabCase("совершенно_неизвестный_термин")
	if err == nil {
		t.Fatal("expected error for unknown term, got nil")
	}
	var unknown *UnknownTermError
	if !errors.As(err, &unknown) {
		t.Fatalf("expected *UnknownTermError, got %T: %v", err, err)
	}
	if unknown.Term != "совершенно" {
		t.Errorf("expected first unrecognized part %q, got %q", "совершенно", unknown.Term)
	}
}

// TestToKebabCase_SameSourceAsPascalCase asserts ToKebabCase resolves parts
// through the same resolveParts as ToPascalCase/ToCamelCase (contract §4.3:
// "Один источник (resolveParts) — три производные формы") by checking a
// composite name that mixes a glossary word and an abbreviation renders
// consistently across all three: same word order, same word boundaries,
// only casing/joiner differs.
func TestToKebabCase_SameSourceAsPascalCase(t *testing.T) {
	pascal, err := ToPascalCase("ид_фт")
	if err != nil {
		t.Fatalf("ToPascalCase: %v", err)
	}
	kebab, err := ToKebabCase("ид_фт")
	if err != nil {
		t.Fatalf("ToKebabCase: %v", err)
	}
	if pascal != "IdFT" {
		t.Fatalf("ToPascalCase(%q) = %q, want %q", "ид_фт", pascal, "IdFT")
	}
	if kebab != "id-ft" {
		t.Fatalf("ToKebabCase(%q) = %q, want %q", "ид_фт", kebab, "id-ft")
	}
}

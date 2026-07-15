package gocode

import "testing"

// TestTermMatch_FeatureLead_MatchesUnderscoreField is the exact case
// diagnosed on the real prat domain (GEN-CODE-CONTRACT.md §3.1): the claim
// text spells the field as two space-separated words ("Feature Lead"), the
// graph's raw field name is underscore-joined ("feature_lead"). Before this
// fix, wholeWordMatch required the literal substring "feature_lead" to
// appear in the claim, which it never does (the claim has no underscore) —
// termMatch's word-sequence comparison fixes this.
func TestTermMatch_FeatureLead_MatchesUnderscoreField(t *testing.T) {
	claim := "Feature Lead (SA) назван, DoR выполнен"
	if !termMatch(claim, "feature_lead") {
		t.Errorf("termMatch(%q, %q) = false, want true", claim, "feature_lead")
	}
}

// TestTermMatch_GenericTwoWordPhraseVsUnderscoreField is a synthetic,
// domain-agnostic case of the same shape as feature_lead: a two-word claim
// phrase against an underscore-joined graph field name, unrelated to prat's
// actual vocabulary, to prove the fix generalizes rather than special-casing
// one field.
func TestTermMatch_GenericTwoWordPhraseVsUnderscoreField(t *testing.T) {
	tests := []struct {
		name  string
		claim string
		term  string
	}{
		{"space claim vs underscore term", "The Risk Owner MUST sign off.", "risk_owner"},
		{"hyphen claim vs underscore term", "The Risk-Owner MUST sign off.", "risk_owner"},
		{"case-insensitive", "the RISK OWNER must sign off.", "risk_owner"},
		{"underscore claim vs hyphen term", "risk_owner is unresolved.", "risk-owner"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !termMatch(tt.claim, tt.term) {
				t.Errorf("termMatch(%q, %q) = false, want true", tt.claim, tt.term)
			}
		})
	}
}

// TestTermMatch_PascalCaseTranslationAlsoMatches asserts termMatch also
// recognizes a claim that spells the field's translated PascalCase Go
// identifier out as separate words (e.g. "Feature Lead" against the
// translated identifier "FeatureLead" itself, as passed for f.fieldName in
// BuildRequirementModel's row-1 field scan) — not just the raw graph
// spelling ("feature_lead").
func TestTermMatch_PascalCaseTranslationAlsoMatches(t *testing.T) {
	claim := "Feature Lead (SA) назван"
	if !termMatch(claim, "FeatureLead") {
		t.Errorf("termMatch(%q, %q) = false, want true", claim, "FeatureLead")
	}
}

// TestTermMatch_UnrelatedTwoWordPhrase_DoesNotMatch is the negative case:
// two claim words that happen to be adjacent but do NOT correspond to the
// field's word sequence must not match — termMatch requires the SAME words
// in the SAME order, not just both words present anywhere in the claim
// (contract §3.1's explicit false-positive warning).
func TestTermMatch_UnrelatedTwoWordPhrase_DoesNotMatch(t *testing.T) {
	tests := []struct {
		name  string
		claim string
		term  string
	}{
		{"reversed word order", "Lead Feature MUST be present.", "feature_lead"},
		{"words present but not adjacent", "Feature scope and the project Lead disagree.", "feature_lead"},
		{"completely unrelated two words", "The risk owner reviews the report.", "feature_lead"},
		{"partial word only", "Feature toggles MUST be documented.", "feature_lead"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if termMatch(tt.claim, tt.term) {
				t.Errorf("termMatch(%q, %q) = true, want false", tt.claim, tt.term)
			}
		})
	}
}

// TestTermMatch_SingleWordBehavesLikeWholeWordMatch pins termMatch's
// single-word fallback to wholeWordMatch's existing, already-correct
// behavior (short tokens like "us"/"ac"/"dor") — the fix must not broaden
// single-word matching, only multi-word/separator-equivalence matching.
func TestTermMatch_SingleWordBehavesLikeWholeWordMatch(t *testing.T) {
	tests := []struct {
		name  string
		claim string
		term  string
		want  bool
	}{
		{"short ascii token matches as whole word", "The dor gate MUST pass.", "dor", true},
		{"short ascii token substring does not match", "Endorsement MUST be present.", "dor", false},
		{"empty term never matches", "anything", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := termMatch(tt.claim, tt.term)
			if got != tt.want {
				t.Errorf("termMatch(%q, %q) = %v, want %v", tt.claim, tt.term, got, tt.want)
			}
			if got != wholeWordMatch(tt.claim, tt.term) {
				t.Errorf("termMatch/wholeWordMatch disagree on single-word term %q against claim %q", tt.term, tt.claim)
			}
		})
	}
}

func TestSplitCamelWords(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"FeatureLead", []string{"feature", "lead"}},
		{"IdFT", []string{"id", "ft"}},
		{"SourceNumberOTT", []string{"source", "number", "ott"}},
		{"dor", []string{"dor"}},
	}
	for _, tt := range tests {
		got := splitCamelWords(tt.in)
		if len(got) != len(tt.want) {
			t.Fatalf("splitCamelWords(%q) = %v, want %v", tt.in, got, tt.want)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitCamelWords(%q) = %v, want %v", tt.in, got, tt.want)
			}
		}
	}
}

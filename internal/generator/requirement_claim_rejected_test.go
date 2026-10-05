package generator

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// A REJECTED requirement may carry only a historical wording — authored in
// one language, with no claim_texts coverage for the render language. The
// projection must show that wording as is instead of failing with a
// missing-translation error.
func TestRequirementClaimREJECTEDWithoutTranslation(t *testing.T) {
	g := &ontology.Graph{
		Languages:       []string{"ru", "en"},
		DefaultLanguage: "ru",
		RenderLanguage:  "en",
	}
	historical := "Историческая формулировка"

	rejected := ontology.Requirement{
		ID: "R-old", Status: ontology.StatusREJECTED, Claim: historical,
	}
	if got := requirementClaim(g, rejected); got != historical {
		t.Fatalf("requirementClaim = %q, want historical wording %q", got, historical)
	}

	// Claim empty but the historical wording was recorded in claim_texts
	// (possibly for a subset of languages) — fall back to it, never panic.
	onlyText := ontology.Requirement{
		ID: "R-texts", Status: ontology.StatusREJECTED,
		ClaimTexts: ontology.LocalizedText{"ru": historical},
	}
	if got := requirementClaim(g, onlyText); got != historical {
		t.Fatalf("requirementClaim = %q, want historical claim_texts entry %q", got, historical)
	}

	// Control: an active requirement without a translation still fails loudly.
	active := ontology.Requirement{ID: "R-active", Status: ontology.StatusSETTLED, Claim: historical}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("active requirement without translation did not fail as missing-translation")
			}
		}()
		requirementClaim(g, active)
	}()
}

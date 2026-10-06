package localization

import (
	"testing"

	canon "github.com/PHPCraftdream/HotamSpec/internal/recorder/canon"
)

// One recorded atom per bool method: each canon.Holds/Fact maps 1:1 to an
// atom, and the derived ID is R-<receiver>-<method>. A second Holds on the
// same method with Expect(false) derives the SAME ID but the `not:` claim,
// which the discovery merge guard rejects as a conflicting subject. Negative
// paths therefore stay as plain asserts; the `not:` doc phrase remains for
// fact-mode false values.

func TestAtomCatalogSupported(t *testing.T) {
	ru := Catalog{Language: "ru", Template: "## Status"}
	canon.Holds(t, ru.Supported)
	de := MissingSubject("de")
	if de.Supported() {
		t.Errorf("MissingSubject(%q).Supported() = true, want false", de.Language)
	}
}

func TestAtomCatalogTranslated(t *testing.T) {
	known := Catalog{Language: "ru", Template: "## Status"}
	canon.Holds(t, known.Translated)
	unknown := Catalog{Language: "ru", Template: "## No such section"}
	if _, err := Lookup(unknown.Language, unknown.Template); err == nil {
		t.Errorf("Lookup(%q, %q) = nil error, want missing-translation", unknown.Language, unknown.Template)
	}
}

func TestAtomMissingTranslationMessage(t *testing.T) {
	err := &MissingTranslation{Language: "de", Template: "## Status"}
	canon.Fact(t, err.Message, `missing authored translation for locale "de" and template "## Status"`)
}

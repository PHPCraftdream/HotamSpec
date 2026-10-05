package localization

import (
	"errors"
	"testing"
)

func TestTextSelectsLocaleAndPreservesTemplateArguments(t *testing.T) {
	for _, tc := range []struct {
		locale string
		want   string
	}{
		{locale: "en", want: "reader: R-rule-π"},
		{locale: "", want: "reader: R-rule-π"},
		{locale: "ru", want: "читатель: R-rule-π"},
		{locale: "zh", want: "读者：R-rule-π"},
	} {
		if got := Text(tc.locale, "reader: %s", "R-rule-π"); got != tc.want {
			t.Errorf("Text(%q) = %q, want %q", tc.locale, got, tc.want)
		}
	}
}

func TestLookupAndTextRefuseMissingTranslationsWithoutFallback(t *testing.T) {
	for _, locale := range []string{"ru", "zh", "fr"} {
		_, err := Lookup(locale, "author-authored template not in catalog")
		var missing *MissingTranslation
		if !errors.As(err, &missing) || missing.Language != locale || missing.Template != "author-authored template not in catalog" {
			t.Fatalf("Lookup(%q) error = %#v, want typed MissingTranslation with exact source key", locale, err)
		}
		renderErr := SafeRender(func() error {
			_ = Text(locale, "author-authored template not in catalog")
			return nil
		})
		if !errors.As(renderErr, &missing) || missing.Language != locale {
			t.Fatalf("SafeRender did not restore typed refusal for %q: %#v", locale, renderErr)
		}
	}
}

func TestSupportedCatalogLocales(t *testing.T) {
	for _, locale := range []string{"", "en", "ru", "zh"} {
		if !Supported(locale) {
			t.Errorf("Supported(%q) = false", locale)
		}
	}
	if Supported("fr") {
		t.Fatal("locale without a complete authored catalog reported as supported")
	}
}

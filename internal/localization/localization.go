// Package localization contains author-written translations for fixed HotamSpec
// service templates. It intentionally has no dependency on ontology or rendered
// documents: callers pass a source template, never a completed document.
package localization

import "fmt"

// MissingTranslation is returned when a fixed service template has no exact
// translation for a requested locale. Template is the unchanged English source
// key, not a rendered string or interpolated user value.
type MissingTranslation struct {
	Language string
	Template string
}

func (e *MissingTranslation) Error() string {
	return fmt.Sprintf("missing authored translation for locale %q and template %q", e.Language, e.Template)
}

// Supported reports whether HotamSpec has a complete, author-written service
// catalog for language. The empty language is the legacy source-template mode.
func Supported(language string) bool {
	switch language {
	case "", "en", "ru", "zh":
		return true
	default:
		return false
	}
}

// catalogs are keyed by exact source templates. Values are fixed authored
// translations; formatting arguments are applied only after selection.
var catalogs = map[string]map[string]string{
	"ru": catalogRU,
	"zh": catalogZH,
}

// Lookup returns the exact authored template for language. Empty/en preserve
// the source template exactly. A missing or unsupported locale is an error,
// never an implicit English fallback.
func Lookup(language, sourceTemplate string) (string, error) {
	if language == "" || language == "en" {
		return sourceTemplate, nil
	}
	if !Supported(language) {
		return "", &MissingTranslation{Language: language, Template: sourceTemplate}
	}
	translated, ok := catalogs[language][sourceTemplate]
	if !ok {
		return "", &MissingTranslation{Language: language, Template: sourceTemplate}
	}
	return translated, nil
}

// Text selects a fixed template before applying formatting arguments. A missing
// or unsupported translation panics with the typed MissingTranslation error;
// whole-bundle renderers must catch it with SafeRender before publishing.
func Text(language, sourceTemplate string, args ...any) string {
	template, err := Lookup(language, sourceTemplate)
	if err != nil {
		panic(err)
	}
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}

// SafeRender converts only Text's typed missing-translation panic to an error.
// Any unrelated panic is rethrown unchanged so it cannot be mistaken for a
// translation refusal.
func SafeRender(render func() error) (err error) {
	defer func() {
		if value := recover(); value != nil {
			missing, ok := value.(*MissingTranslation)
			if !ok {
				panic(value)
			}
			err = missing
		}
	}()
	return render()
}

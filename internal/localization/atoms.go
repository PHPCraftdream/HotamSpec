package localization

import "fmt"

// Catalog is an atomic facade over the authored service catalogs for one
// exact (language, template) pair. It exists so the self-executing atom
// pipeline can bind zero-argument methods over the package-level
// Supported/Lookup entry points without changing their signatures.
type Catalog struct {
	Language string
	Template string
}

// the catalog covers the language
//
// Supported reports whether the catalog has a complete authored translation
// set for the language. An empty language is the legacy source-template mode.
// not: the catalog does not cover the language
func (c Catalog) Supported() bool { return Supported(c.Language) }

// the template has an authored translation
//
// Translated reports whether Lookup resolves the exact template for the
// language without a MissingTranslation error.
func (c Catalog) Translated() bool {
	_, err := Lookup(c.Language, c.Template)
	return err == nil
}

// MissingSubject returns a Catalog whose language has no authored catalog,
// for negative-path atoms and tests.
func MissingSubject(language string) Catalog {
	return Catalog{Language: language, Template: "## Status"}
}

// the missing-translation error names the locale and template
//
// Message renders the typed error text; it is a value atom over
// MissingTranslation so the error's observable shape is executed, not
// hand-authored.
func (e *MissingTranslation) Message() string {
	return fmt.Sprintf("missing authored translation for locale %q and template %q", e.Language, e.Template)
}

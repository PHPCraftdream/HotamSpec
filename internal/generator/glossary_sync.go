package generator

import (
	"regexp"
	"sort"
	"strings"
)

// sectionAnchorRE matches §-anchor tokens (e.g. "§Conflict", "§Requirement")
// as used in framework sources to reference a methodology section by anchor.
var sectionAnchorRE = regexp.MustCompile(`§[A-Za-z][A-Za-z0-9_-]*`)

// GlossarySyncReport carries the terminology drift between the glossary's
// controlled vocabulary and a framework-source corpus it is meant to mirror.
type GlossarySyncReport struct {
	// DeadTerms are glossary slugs referenced nowhere in the corpus — defined
	// vocabulary that has accumulated as noise (R-glossary-sync-fails-dead).
	DeadTerms []string
	// UndefinedRefs are §-anchor tokens used in the corpus but absent from the
	// glossary's SECTION entries — unresolved speak-by-reference references
	// (R-glossary-sync-fails-unused).
	UndefinedRefs []string
}

// AuditGlossarySync checks the glossary against a corpus of framework-source
// text. A glossary slug is "dead" when it appears nowhere in the corpus; a
// §-anchor reference in the corpus is "undefined" when no SECTION glossary
// term carries it. Both are terminology drift the sync check must surface.
//
// The corpus is passed in (rather than read from disk inside this function)
// so callers can doctor it for a deterministic, hermetic test that proves the
// check actually catches drift rather than merely running.
func AuditGlossarySync(terms []glossaryTerm, corpus []string) GlossarySyncReport {
	corpusJoined := strings.Join(corpus, "\n")
	var report GlossarySyncReport

	// dead terms: glossary slugs referenced nowhere in the corpus.
	for _, term := range terms {
		if !strings.Contains(corpusJoined, term.Slug) {
			report.DeadTerms = append(report.DeadTerms, term.Slug)
		}
	}

	// undefined refs: §-anchor tokens used in the corpus but not carried as a
	// SECTION glossary entry.
	definedSections := make(map[string]bool)
	for _, term := range terms {
		if term.Kind == "SECTION" {
			definedSections[term.Slug] = true
		}
	}
	seen := make(map[string]bool)
	for _, m := range sectionAnchorRE.FindAllString(corpusJoined, -1) {
		if seen[m] {
			continue
		}
		seen[m] = true
		if !definedSections[m] {
			report.UndefinedRefs = append(report.UndefinedRefs, m)
		}
	}

	sort.Strings(report.DeadTerms)
	sort.Strings(report.UndefinedRefs)
	return report
}

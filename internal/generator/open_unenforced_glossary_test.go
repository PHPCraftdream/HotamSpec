package generator

import (
	"os"
	"testing"
)

func TestBuildOpen_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildOpen(g)
	want, err := os.ReadFile("testdata/fixture/OPEN.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "OPEN.md", got, string(want))
}

func TestBuildUnenforced_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildUnenforced(g)
	want, err := os.ReadFile("testdata/fixture/UNENFORCED.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "UNENFORCED.md", got, string(want))
}

func TestBuildGlossary_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildGlossary(g)
	want, err := os.ReadFile("testdata/fixture/GLOSSARY.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "GLOSSARY.md", got, string(want))
}

// TestAuditGlossarySync_FlagsDeadTerms enforces R-glossary-sync-fails-dead:
// the sync audit must flag a glossary term referenced nowhere in the framework
// corpus (a dead term). A doctored glossary carries a fake SECTION term absent
// from the corpus; the audit must report it. A genuinely-referenced term must
// NOT be flagged (discrimination). This proves the check catches drift, not
// merely that it runs.
func TestAuditGlossarySync_FlagsDeadTerms(t *testing.T) {
	t.Parallel()
	terms := []glossaryTerm{
		{Slug: "§RealUsed", Kind: "SECTION", Definition: "used in corpus"},
		{Slug: "§FakeDeadTerm", Kind: "SECTION", Definition: "nowhere in corpus"},
	}
	corpus := []string{
		"the §RealUsed section is referenced here",
		"no mention of the fake term anywhere",
	}
	report := AuditGlossarySync(terms, corpus)

	found := false
	for _, d := range report.DeadTerms {
		if d == "§FakeDeadTerm" {
			found = true
		}
	}
	if !found {
		t.Errorf("dead-term audit must flag §FakeDeadTerm (defined but unreferenced), got DeadTerms=%v", report.DeadTerms)
	}
	for _, d := range report.DeadTerms {
		if d == "§RealUsed" {
			t.Errorf("referenced term §RealUsed must NOT be flagged dead, got DeadTerms=%v", report.DeadTerms)
		}
	}
}

// TestAuditGlossarySync_FlagsUndefinedRefs enforces R-glossary-sync-fails-unused:
// the sync audit must flag a §-anchor token used in the framework corpus but
// absent from the glossary's SECTION entries. A doctored corpus references a
// fake §-anchor the glossary does not define; the audit must report it. A
// defined §-anchor must NOT be flagged (discrimination).
func TestAuditGlossarySync_FlagsUndefinedRefs(t *testing.T) {
	t.Parallel()
	terms := []glossaryTerm{
		{Slug: "§DefinedAnchor", Kind: "SECTION", Definition: "defined"},
	}
	corpus := []string{
		"the §DefinedAnchor is known",
		"the §FakeUndefinedAnchor is used but undefined",
	}
	report := AuditGlossarySync(terms, corpus)

	found := false
	for _, u := range report.UndefinedRefs {
		if u == "§FakeUndefinedAnchor" {
			found = true
		}
	}
	if !found {
		t.Errorf("unused-ref audit must flag §FakeUndefinedAnchor (used but undefined), got UndefinedRefs=%v", report.UndefinedRefs)
	}
	for _, u := range report.UndefinedRefs {
		if u == "§DefinedAnchor" {
			t.Errorf("defined §-anchor §DefinedAnchor must NOT be flagged undefined, got UndefinedRefs=%v", report.UndefinedRefs)
		}
	}
}

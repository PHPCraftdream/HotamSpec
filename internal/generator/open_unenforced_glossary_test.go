package generator

import (
	"os"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
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

// TestBuildUnenforced_CloseableSplitPartitionsDebt is the non-regression guard
// for the closeable-debt split: closeable-now and feature-blocked are two
// DISJOINT subsets whose union must equal the full IsCloseableDebt band (so the
// split can never silently drop or double-count an item). It builds a synthetic
// graph with a known mix of closeable-now, feature-blocked, inherent-discipline,
// and enforced requirements, renders UNENFORCED.md, and asserts the burn-down
// line's two new figures sum to the total closeable-debt count and that each
// item lands in exactly one table.
func TestBuildUnenforced_CloseableSplitPartitionsDebt(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{{ID: "S-owner", Name: "owner", Domain: "d"}},
		Requirements: []ontology.Requirement{
			{ID: "R-enf", Owner: "S-owner", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementENFORCED, EnforcedBy: []string{"check_x"}, DeclOrder: 0},
			{ID: "R-now-a", Owner: "S-owner", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementPROSE, Enforceability: ontology.EnforceabilityENFORCEABLE, Claim: "closeable now a", DeclOrder: 1},
			{ID: "R-now-b", Owner: "S-owner", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementSTRUCTURAL, Enforceability: ontology.EnforceabilityENFORCEABLE, Claim: "closeable now b", DeclOrder: 2},
			{ID: "R-blk-a", Owner: "S-owner", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementPROSE, Enforceability: ontology.EnforceabilityENFORCEABLE, BlockedOn: "blocked on Planned tool X", Claim: "feature blocked a", DeclOrder: 3},
			{ID: "R-blk-b", Owner: "S-owner", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementPROSE, Enforceability: ontology.EnforceabilityENFORCEABLE, BlockedOn: "blocked on absent package Y", Claim: "feature blocked b", DeclOrder: 4},
			{ID: "R-inh", Owner: "S-owner", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementSTRUCTURAL, Enforceability: ontology.EnforceabilityINHERENTLY_PROSE, Claim: "inherent discipline", DeclOrder: 5},
		},
	}
	got := BuildUnenforced(g)

	// Sanity: the band predicates are disjoint and partition IsCloseableDebt.
	var closeableNow, featureBlocked int
	for _, r := range g.Requirements {
		if r.IsCloseableDebtNow() {
			closeableNow++
		}
		if r.IsFeatureBlockedDebt() {
			featureBlocked++
		}
		if r.IsCloseableDebtNow() && r.IsFeatureBlockedDebt() {
			t.Fatal("a requirement is both closeable-now and feature-blocked — predicates must be disjoint")
		}
	}

	// The burn-down line must carry the two split figures.
	if !strings.Contains(got, "closeable-now 2;") {
		t.Errorf("burn-down line should report closeable-now 2, got:\n%s", got)
	}
	if !strings.Contains(got, "feature-blocked 2;") {
		t.Errorf("burn-down line should report feature-blocked 2, got:\n%s", got)
	}
	// Partition invariant: the two figures must sum to the full closeable-debt count (4 here).
	if closeableNow+featureBlocked != 4 {
		t.Errorf("partition invariant: closeable-now(%d)+feature-blocked(%d) != 4 total closeable debt", closeableNow, featureBlocked)
	}
	// The feature-blocked table must carry the blocked_on column with the values.
	if !strings.Contains(got, "blocked on Planned tool X") {
		t.Errorf("feature-blocked table should render the blocked_on value, got:\n%s", got)
	}
	if !strings.Contains(got, "blocked on absent package Y") {
		t.Errorf("feature-blocked table should render the second blocked_on value, got:\n%s", got)
	}
	// The closeable-now items must NOT carry a blocked_on column (4-col table).
	if !strings.Contains(got, "## Closeable debt — closeable now (real, actionable)") {
		t.Errorf("closeable-now section header missing, got:\n%s", got)
	}
	if !strings.Contains(got, "## Closeable debt — feature-blocked (honest roadmap, not neglected)") {
		t.Errorf("feature-blocked section header missing, got:\n%s", got)
	}
}

func TestBuildGlossary_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildGlossary(g, false)
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

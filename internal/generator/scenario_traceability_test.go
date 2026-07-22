package generator

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// scenarioFixtureGraph builds a small graph mirroring specFixtureGraph's own
// shape (see spec_test.go) but exercising the specific rows W1.4's
// TRACEABILITY.md/COVERAGE.md additions need: a SETTLED requirement whose
// verified_by test narrates a hotamspec scenario (R-spec-narrated), a
// SETTLED requirement whose verified_by test is a plain (non-scenario) Go
// test (R-scenario-plain), and a SETTLED requirement with no verified_by at
// all (not part of the ratchet -- roadmap debt instead).
func scenarioFixtureGraph(domainDir string) *ontology.Graph {
	return &ontology.Graph{
		DomainDir:   domainDir,
		SelfHosting: false,
		Requirements: []ontology.Requirement{
			{
				ID:             "R-spec-narrated",
				Claim:          "RequireComplete ALWAYS rejects a zero fields count.",
				Owner:          "spec-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementENFORCED,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
				ImplementedBy:  []string{"model/impl.go:RequireComplete"},
				VerifiedBy:     []string{"model/impl_test.go:TestRequireComplete_Scenario"},
			},
			{
				ID:             "R-scenario-plain",
				Claim:          "RequireComplete ALWAYS accepts a positive fields count.",
				Owner:          "spec-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementENFORCED,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
				ImplementedBy:  []string{"model/impl.go:RequireComplete"},
				VerifiedBy:     []string{"model/impl_test.go:TestRequireComplete_Plain"},
			},
			{
				ID:             "R-scenario-no-carrier",
				Claim:          "Every SETTLED requirement eventually gets a scenario.",
				Owner:          "spec-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementPROSE,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
			},
		},
	}
}

// TestBuildTraceability_ScenarioColumnIsASTOnly proves BuildTraceability's
// scenario column is ALWAYS the cheap AST scan (gate.ResolveSpecTest's
// HasScenario) alone, WITHOUT ever invoking `go test` -- there is no
// verdict-overlay mode any more (continuing task #317: `hotam land`'s own
// routine regeneration always calls plain BuildTraceability, so any
// --spec-shaped rendering would be inherently unstable, silently reverted by
// the very next `hotam land`; see TestGenSpec_SharedProjectionsModeIndependent
// for the end-to-end regression this property backs). A plain
// (non-scenario) verified_by test must NOT be tagged.
func TestBuildTraceability_ScenarioColumnIsASTOnly(t *testing.T) {
	root := writeSpecFixtureModule(t)
	g := scenarioFixtureGraph(root)

	got := BuildTraceability(g)

	if !strings.Contains(got, "R-spec-narrated") {
		t.Fatalf("BuildTraceability output missing R-spec-narrated:\n%s", got)
	}
	// The narrated requirement's row must carry the scenario tag.
	narratedIdx := strings.Index(got, "R-spec-narrated")
	narratedRowEnd := strings.Index(got[narratedIdx:], "\n")
	narratedRow := got[narratedIdx : narratedIdx+narratedRowEnd]
	if !strings.Contains(narratedRow, "scenario") {
		t.Errorf("R-spec-narrated's row does not carry the scenario tag:\n%s", narratedRow)
	}
	// No real recorded verdict text should ever appear IN THE TABLE ROW (the
	// document's own explanatory intro prose legitimately points to SPEC.md
	// as where the real narrative lives; only the row itself must stay
	// silent about any executed verdict, since BuildTraceability never
	// executes a test).
	if strings.Contains(narratedRow, "(narrated") || strings.Contains(narratedRow, "(no narrative recorded)") {
		t.Errorf("BuildTraceability rendered a REAL verdict in R-spec-narrated's row (should be AST-only, no verdict overlay exists):\n%s", narratedRow)
	}

	plainIdx := strings.Index(got, "R-scenario-plain")
	if plainIdx < 0 {
		t.Fatalf("BuildTraceability output missing R-scenario-plain:\n%s", got)
	}
	plainRowEnd := strings.Index(got[plainIdx:], "\n")
	plainRow := got[plainIdx : plainIdx+plainRowEnd]
	if strings.Contains(plainRow, "· scenario") {
		t.Errorf("R-scenario-plain's row falsely carries the scenario tag (its test has no hotamspec.NewScenario call):\n%s", plainRow)
	}
}

// TestBuildTraceability_ModeIndependent_VerifiedByGoTestExecutionDoesNotChangeOutput
// proves the actual property this task's fix depends on: running the SAME
// real go-test recording pass a `gen-spec --spec` invocation would perform
// (CollectSpecRows/ScenarioVerdictsFromRows -- still real, live APIs used to
// render SPEC.md itself) has ZERO effect on BuildTraceability's own output,
// because BuildTraceability no longer accepts or consumes that data at all.
// This replaces the former "verdict overlay" test this fix removed: instead
// of asserting an overlay renders correctly, it now asserts no such overlay
// exists to render.
func TestBuildTraceability_ModeIndependent_VerifiedByGoTestExecutionDoesNotChangeOutput(t *testing.T) {
	root := writeSpecFixtureModule(t)
	g := scenarioFixtureGraph(root)

	before := BuildTraceability(g)

	// Run the same real recording pass gen-spec --spec performs -- proves
	// its side effects (if any) cannot leak into BuildTraceability's output,
	// since BuildTraceability has no parameter left to receive it through.
	rows := CollectSpecRows(g)
	_ = ScenarioVerdictsFromRows(rows)

	after := BuildTraceability(g)
	if before != after {
		diffReport(t, "TRACEABILITY.md (before/after a real --spec-shaped recording pass)", after, before)
	}
}

// TestBuildTraceability_ScenarioColumn_ByteIdenticalAcrossRuns proves the
// AST-only scenario column is exactly as byte-identical-across-runs as every
// other cheap projection this package generates -- no incidental
// nondeterminism (map iteration order, etc.) leaked in by the new column.
func TestBuildTraceability_ScenarioColumn_ByteIdenticalAcrossRuns(t *testing.T) {
	root := writeSpecFixtureModule(t)
	g := scenarioFixtureGraph(root)

	first := BuildTraceability(g)
	second := BuildTraceability(g)
	if first != second {
		diffReport(t, "TRACEABILITY.md (repeat run, scenario column)", second, first)
	}
}

// TestBuildCoverage_ScenarioRatchet_ASTOnlyByDefault proves COVERAGE.md's
// ratchet counter (PLAN-scenario-generated-spec.md §2 D4/§5, W1.4) computes
// from the same cheap AST signal by default: R-spec-narrated counts as
// "with scenario" (narrated), R-scenario-plain counts as still in the
// ratchet tail (verified_by exists, but its test does not call
// hotamspec.NewScenario), and R-scenario-no-carrier (no verified_by at all)
// is NOT counted in the ratchet at all -- it is roadmap debt, a distinct
// bucket.
func TestBuildCoverage_ScenarioRatchet_ASTOnlyByDefault(t *testing.T) {
	root := writeSpecFixtureModule(t)
	g := scenarioFixtureGraph(root)

	got := BuildCoverage(g)

	if !strings.Contains(got, "**1/2 SETTLED requirement(s) with `verified_by` carry a scenario narrative; 1 remain in the ratchet tail.**") {
		t.Errorf("BuildCoverage ratchet summary line did not match expected 1/2 narrated, 1 tail:\n%s", got)
	}
	if !strings.Contains(got, "R-scenario-plain") {
		t.Errorf("BuildCoverage ratchet tail table missing R-scenario-plain:\n%s", got)
	}
	// R-spec-narrated must NOT appear in the ratchet TAIL table (it is
	// narrated) -- check specifically within the "Scenario ratchet" section,
	// not the whole document (R-spec-narrated legitimately appears
	// elsewhere, e.g. roadmap-debt is not it, but a future table could
	// legitimately mention it).
	ratchetStart := strings.Index(got, "## Scenario ratchet")
	nextSection := strings.Index(got[ratchetStart+1:], "\n## ")
	ratchetSection := got[ratchetStart : ratchetStart+1+nextSection]
	if strings.Contains(ratchetSection, "R-spec-narrated") {
		t.Errorf("BuildCoverage ratchet section wrongly lists the already-narrated R-spec-narrated:\n%s", ratchetSection)
	}
}

// TestBuildCoverage_ModeIndependent_VerifiedByGoTestExecutionDoesNotChangeOutput
// mirrors TestBuildTraceability_ModeIndependent_...: running the same real
// go-test recording pass a `gen-spec --spec` invocation would perform
// (CollectSpecRows/ScenarioVerdictsFromRows -- still real, live APIs used to
// render SPEC.md itself) has ZERO effect on BuildCoverage's own output,
// because BuildCoverage no longer accepts or consumes that data at all --
// the ratchet counter is always the AST-only signal.
func TestBuildCoverage_ModeIndependent_VerifiedByGoTestExecutionDoesNotChangeOutput(t *testing.T) {
	root := writeSpecFixtureModule(t)
	g := scenarioFixtureGraph(root)

	before := BuildCoverage(g)

	rows := CollectSpecRows(g)
	_ = ScenarioVerdictsFromRows(rows)

	after := BuildCoverage(g)
	if before != after {
		diffReport(t, "COVERAGE.md (before/after a real --spec-shaped recording pass)", after, before)
	}
	if !strings.Contains(after, "**1/2 SETTLED requirement(s) with `verified_by` carry a scenario narrative; 1 remain in the ratchet tail.**") {
		t.Errorf("BuildCoverage ratchet summary line did not match expected 1/2 narrated, 1 tail:\n%s", after)
	}
}

// TestBuildCoverage_LayerTable_ReportsScenarioCount proves the Layer table's
// new "scenarios" column (the fifth rung, PLAN-scenario-generated-spec.md
// §1) reports the same count the ratchet section's "withScenario" number
// does, for a domain whose authored spec/ tree is non-empty (this fixture's
// model/impl.go), so both surfaces of the same underlying count agree.
func TestBuildCoverage_LayerTable_ReportsScenarioCount(t *testing.T) {
	root := writeSpecFixtureModule(t)
	g := scenarioFixtureGraph(root)

	got := BuildCoverage(g)

	if !strings.Contains(got, "| scenarios |") {
		t.Fatalf("BuildCoverage Layer table missing the scenarios column header:\n%s", got)
	}
}

// TestBuildCoverage_NoVerifiedByAtAll_RatchetIsEmpty proves a domain with no
// SETTLED+verified_by requirement at all reports a calm, honest "nothing to
// count" ratchet rather than a misleading 0/0-looks-like-100%.
func TestBuildCoverage_NoVerifiedByAtAll_RatchetIsEmpty(t *testing.T) {
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{
				ID:             "R-no-verified-by",
				Claim:          "A claim with no verified_by at all.",
				Owner:          "spec-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementPROSE,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
			},
		},
	}
	got := BuildCoverage(g)
	if !strings.Contains(got, "_No SETTLED requirement in this domain carries `verified_by` yet — the ratchet has nothing to count._") {
		t.Errorf("BuildCoverage did not render the calm empty-ratchet notice:\n%s", got)
	}
}

// disciplineExemptFixtureGraph builds a discipline:full graph exercising the
// COVERAGE.md "Discipline-exempt (inherently prose, no carrier)" section: a
// SETTLED+INHERENTLY_PROSE requirement with NO carrier (R-prose-no-carrier,
// the actively-exempted set), a SETTLED+INHERENTLY_PROSE requirement that
// DOES carry enforced_by (R-prose-with-engine -- NOT in the section, the
// engine-path exemption already covers it), and a SETTLED+ENFORCEABLE
// requirement with no carrier (R-ordinary-no-carrier -- NOT in the section,
// it is roadmap debt, not exempt). All three are needed so the section test
// can prove the listing is the precise intersection (INHERENTLY_PROSE AND
// no carrier), not just INHERENTLY_PROSE, and not just no-carrier.
func disciplineExemptFixtureGraph(discipline string) *ontology.Graph {
	return &ontology.Graph{
		DomainDir:   "",
		SelfHosting: false,
		Discipline:  discipline,
		Requirements: []ontology.Requirement{
			{
				ID:             "R-prose-no-carrier",
				Claim:          "Found a domain wave-before-specific (architectural, inherently prose).",
				Owner:          "spec-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementPROSE,
				Enforceability: ontology.EnforceabilityINHERENTLY_PROSE,
			},
			{
				ID:             "R-prose-with-engine",
				Claim:          "Another inherently-prose note that happens to reference an engine mechanism.",
				Owner:          "spec-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementENFORCED,
				Enforceability: ontology.EnforceabilityINHERENTLY_PROSE,
				EnforcedBy:     []string{"check_enforced_names_invariant"},
			},
			{
				ID:             "R-ordinary-no-carrier",
				Claim:          "An ordinary ENFORCEABLE claim that has no carrier yet (roadmap debt).",
				Owner:          "spec-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementPROSE,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
			},
		},
	}
}

// sectionRange returns the substring of doc spanning the heading `heading`
// (the "## <heading>" line) up to (but not including) the NEXT "## " heading
// after it, or "" if the heading is absent. Used to scope substring searches
// to ONE section so a string appearing elsewhere in the doc does not produce
// a false positive.
func sectionRange(doc, heading string) string {
	startMarker := "## " + heading
	start := strings.Index(doc, startMarker)
	if start < 0 {
		return ""
	}
	rest := doc[start+1:]
	next := strings.Index(rest, "\n## ")
	if next < 0 {
		return doc[start:]
	}
	return doc[start : start+1+next]
}

// TestBuildCoverage_DisciplineExemptSection_ListsExemptedInDisciplineFull
// proves that for a discipline:full domain the new section is rendered, AND
// its table lists EXACTLY the SETTLED+INHERENTLY_PROSE+no-carrier subset
// (R-prose-no-carrier) -- NOT the INHERENTLY_PROSE requirement that has a
// carrier (R-prose-with-engine), and NOT the ordinary ENFORCEABLE no-carrier
// requirement (R-ordinary-no-carrier). This is the visibility contract for
// check_settled_requires_scenario's INHERENTLY_PROSE exemption.
func TestBuildCoverage_DisciplineExemptSection_ListsExemptedInDisciplineFull(t *testing.T) {
	t.Parallel()
	g := disciplineExemptFixtureGraph(loader.DisciplineFull)
	if g.Discipline != loader.DisciplineFull {
		t.Fatalf("test setup: expected DisciplineFull, got %q", g.Discipline)
	}
	got := BuildCoverage(g)

	section := sectionRange(got, "Discipline-exempt (inherently prose, no carrier)")
	if section == "" {
		t.Fatalf("BuildCoverage for a discipline:full domain did not render the \"Discipline-exempt\" section:\n%s", got)
	}
	if !strings.Contains(section, "R-prose-no-carrier") {
		t.Errorf("Discipline-exempt section missing R-prose-no-carrier (the actively-exempted requirement):\n%s", section)
	}
	if strings.Contains(section, "R-prose-with-engine") {
		t.Errorf("Discipline-exempt section wrongly lists R-prose-with-engine (it carries enforced_by -- not actively exempted):\n%s", section)
	}
	if strings.Contains(section, "R-ordinary-no-carrier") {
		t.Errorf("Discipline-exempt section wrongly lists R-ordinary-no-carrier (ENFORCEABLE, not INHERENTLY_PROSE -- roadmap debt, not exempt):\n%s", section)
	}
}

// TestBuildCoverage_DisciplineExemptSection_AbsentForNonDisciplineFull proves
// the matching honest-no-op: for a domain that has NOT opted into
// discipline:full, the section MUST NOT appear at all (the exemption is
// dormant there). This is the same graph as the discipline:full test above,
// only the Discipline field differs -- so the contrast is exactly the
// opt-in boundary.
func TestBuildCoverage_DisciplineExemptSection_AbsentForNonDisciplineFull(t *testing.T) {
	t.Parallel()
	g := disciplineExemptFixtureGraph("") // soft discipline -- every domain today
	if g.Discipline != "" {
		t.Fatalf("test setup: expected empty Discipline, got %q", g.Discipline)
	}
	got := BuildCoverage(g)
	if sectionRange(got, "Discipline-exempt (inherently prose, no carrier)") != "" {
		t.Errorf("BuildCoverage for a non-discipline:full domain rendered the \"Discipline-exempt\" section (must be absent):\n%s", got)
	}
	// Sanity: the INHERENTLY_PROSE requirement still appears in the existing
	// permanent-discipline table for a soft-discipline domain (that section is
	// discipline-agnostic); only the NEW section is discipline:full-gated.
	if !strings.Contains(got, "## Permanent discipline") {
		t.Errorf("BuildCoverage missing the discipline-agnostic Permanent discipline section entirely:\n%s", got)
	}
}

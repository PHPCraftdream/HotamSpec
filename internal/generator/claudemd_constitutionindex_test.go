package generator

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// consumerCategorizationFixtureGraph builds a small SETTLED-requirement set
// whose ids are deliberately non-semantic (R-FR-01..R-FR-04, mirroring
// gpsm-sm's real CSV-derived id shape — sequential, not topic-coded) so
// id-prefix bucketing (digestCategories) would dump ALL of them into
// "Other", the exact failure mode task #337 / external review R4 §4.6
// found. Enforcement varies across the four so every consumer category
// (Enforced/Structural/Prose) is exercised.
func consumerCategorizationFixtureGraph() *ontology.Graph {
	return &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-FR-01", Claim: "one", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementENFORCED},
			{ID: "R-FR-02", Claim: "two", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementENFORCED},
			{ID: "R-FR-03", Claim: "three", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementSTRUCTURAL},
			{ID: "R-FR-04", Claim: "four", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementPROSE},
		},
	}
}

// TestBuildConstitutionIndexModel_ConsumerGroupsByEnforcementNotOther proves
// the core fix: under consumer=true, non-semantic ids (R-FR-NN) land in
// Enforced/Structural/Prose buckets by their actual Enforcement tier, never
// collapsed into one uninformative "Other" bucket.
func TestBuildConstitutionIndexModel_ConsumerGroupsByEnforcementNotOther(t *testing.T) {
	t.Parallel()
	g := consumerCategorizationFixtureGraph()
	categories := buildConstitutionIndexModel(g, true)

	got := map[string]int{}
	for _, c := range categories {
		got[c.Label] = len(c.Requirements)
	}
	if got["Enforced"] != 2 {
		t.Errorf("expected 2 Enforced, got %d (categories: %+v)", got["Enforced"], got)
	}
	if got["Structural"] != 1 {
		t.Errorf("expected 1 Structural, got %d (categories: %+v)", got["Structural"], got)
	}
	if got["Prose"] != 1 {
		t.Errorf("expected 1 Prose, got %d (categories: %+v)", got["Prose"], got)
	}
	if n := got["Other"]; n != 0 {
		t.Errorf("consumer categorization must not dump requirements into Other when Enforcement is known, got %d in Other", n)
	}
}

// TestBuildConstitutionIndexModel_FullProfileUnchangedIdPrefixBucketing is
// the regression guard for the FULL profile (hotam-spec-self and other
// self-hosting domains): consumer=false must keep the pre-existing
// id-prefix categorization (digestCategories) byte-for-byte — this task
// touches ONLY the consumer branch.
func TestBuildConstitutionIndexModel_FullProfileUnchangedIdPrefixBucketing(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	full := buildConstitutionIndexModel(g, false)
	if len(full) == 0 {
		t.Fatalf("expected non-empty categories for the full profile on the fixture graph")
	}
	// None of the full-profile category labels are the consumer-only labels.
	for _, c := range full {
		if c.Label == "Enforced" || c.Label == "Structural" || c.Label == "Prose" {
			t.Errorf("full profile must not use consumer category labels, got %q", c.Label)
		}
	}
}

// TestCategorizeRequirementByEnforcement_UnknownFallsBackToOther proves the
// honest-fallback path: an Enforcement value outside the three known
// constants (should not occur for a real SETTLED requirement, but the field
// is a plain string) categorizes as "Other" rather than panicking or
// silently mis-bucketing.
func TestCategorizeRequirementByEnforcement_UnknownFallsBackToOther(t *testing.T) {
	t.Parallel()
	got := categorizeRequirementByEnforcement(ontology.Requirement{ID: "R-x", Enforcement: "WEIRD"})
	if got != "Other" {
		t.Errorf("unknown Enforcement should categorize as Other, got %q", got)
	}
}

// TestBuildConstitutionBlock_ConsumerRendersEnforcementCategories is the
// end-to-end smoke: BuildConstitutionBlock(consumer=true) against a
// gpsm-sm-shaped fixture must render the Enforced/Structural/Prose category
// line, not "Other (N)" alone.
func TestBuildConstitutionBlock_ConsumerRendersEnforcementCategories(t *testing.T) {
	t.Parallel()
	g := consumerCategorizationFixtureGraph()
	out := BuildConstitutionBlock(g, "gpsm-sm-like", true)
	for _, want := range []string{"Enforced (2)", "Structural (1)", "Prose (1)"} {
		if !strings.Contains(out, want) {
			t.Errorf("consumer CONSTITUTION block missing %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Other (4)") {
		t.Errorf("consumer CONSTITUTION block must not dump every requirement into Other, got:\n%s", out)
	}
}

// TestBuildConstitutionBlock_FullProfileStillUsesIdPrefixCategories is the
// regression guard mirroring TestBuildConstitutionBlock_EmptyGraph's
// existing full-profile call shape (consumer=false) — confirms the default
// (non-consumer) call path is unaffected by the new parameter.
func TestBuildConstitutionBlock_FullProfileStillUsesIdPrefixCategories(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	out := BuildConstitutionBlock(g, "hotam-spec-self", false)
	if strings.Contains(out, "Enforced (") || strings.Contains(out, "Structural (") || strings.Contains(out, "Prose (") {
		t.Errorf("full profile must not render consumer-only enforcement-tier category labels, got:\n%s", out)
	}
}

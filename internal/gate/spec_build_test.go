package gate

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// specTestGraph returns a graph with two no-verified_by requirements — one
// SETTLED (honest roadmap debt) and one REJECTED (withdrawn, not debt) —
// exercising BuildSpecFromRows's honest-gap partition without any go-test
// execution (the rows map is empty; neither requirement has verified_by).
func specRejectedGraph() *ontology.Graph {
	return &ontology.Graph{
		Requirements: []ontology.Requirement{
			{
				ID:          "R-spec-settled-debt",
				Claim:       "A SETTLED requirement without a carrier is honest debt.",
				Owner:       "spec-author",
				Status:      ontology.StatusSETTLED,
				Enforcement: ontology.EnforcementPROSE,
			},
			{
				ID:          "R-spec-rejected",
				Claim:       "A REJECTED requirement is withdrawn, not roadmap debt.",
				Owner:       "spec-author",
				Status:      ontology.StatusREJECTED,
				Enforcement: ontology.EnforcementPROSE,
			},
		},
	}
}

// TestBuildSpecFromRows_RejectedNotHonestGap proves a REJECTED requirement
// with no verified_by is excluded from the "Without a scenario" table AND
// from the summary counts: a rejection is not roadmap debt (mirrors
// coverage.go's SETTLED-only partition).
func TestBuildSpecFromRows_RejectedNotHonestGap(t *testing.T) {
	got := BuildSpecFromRows(specRejectedGraph(), map[string]SpecRow{})

	if strings.Contains(got, "R-spec-rejected") {
		t.Errorf("REJECTED requirement must not render in SPEC.md:\n%s", got)
	}
	if !strings.Contains(got, "`R-spec-settled-debt`") {
		t.Errorf("SETTLED no-carrier requirement missing from honest-gap table:\n%s", got)
	}
	if !strings.Contains(got, "**0 requirement(s) carry `verified_by`; 0 have at least one recorded scenario narrative; 1 carry no `verified_by` yet (no code carrier, honest gap).**") {
		t.Errorf("summary line must count only non-REJECTED requirements:\n%s", got)
	}
}

// TestBuildSpecFromRows_TitleSuppressedWhenEqualsClaim proves the duplicate
// title/claim fix: when an artifact's title equals the requirement's Claim
// (discipline:"full" derives the Claim from that very title), the bold title
// line is suppressed; a differing title keeps its bold line.
func TestBuildSpecFromRows_TitleSuppressedWhenEqualsClaim(t *testing.T) {
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{
				ID:         "R-spec-title-eq",
				Claim:      "Same sentence both places.",
				Status:     ontology.StatusSETTLED,
				VerifiedBy: []string{"model/impl_test.go:TestX"},
			},
		},
	}
	rows := map[string]SpecRow{
		"R-spec-title-eq": {
			req: g.Requirements[0],
			outcomes: []specTestOutcome{
				{
					entry: "model/impl_test.go:TestX",
					artifacts: []specArtifact{
						{
							Title:   "Same sentence both places.",
							Verdict: "pass",
							Steps:   []specArtifactStep{{Kind: "then", Desc: "it held", Passed: true}},
						},
					},
				},
			},
		},
	}

	got := BuildSpecFromRows(g, rows)

	if strings.Contains(got, "**Same sentence both places.**") {
		t.Errorf("bold title equal to Claim must be suppressed:\n%s", got)
	}
	if !strings.Contains(got, "- Then it held — **held**") {
		t.Errorf("scenario steps must still render:\n%s", got)
	}

	// A differing title keeps its bold line.
	rows["R-spec-title-eq"].outcomes[0].artifacts[0].Title = "A different title."
	got = BuildSpecFromRows(g, rows)
	if !strings.Contains(got, "**A different title.**") {
		t.Errorf("bold title differing from Claim must be kept:\n%s", got)
	}
}

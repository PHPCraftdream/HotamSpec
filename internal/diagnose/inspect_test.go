package diagnose

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func settledClaim(id, owner, claim string) ontology.Requirement {
	return ontology.Requirement{
		ID:     id,
		Owner:  owner,
		Status: ontology.StatusSETTLED,
		Claim:  claim,
	}
}

func TestInspectLexicalClaimOverlap_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		reqs       []ontology.Requirement
		wantFire   bool
		wantMember string // an id expected in the fired candidate's Members, if wantFire
	}{
		{
			name: "opposite markers never vs always on same PII/cache topic fire",
			reqs: []ontology.Requirement{
				settledClaim("R-cache-no-pii", "team-a", "cache must never store PII"),
				settledClaim("R-cache-all-fields", "team-a", "cache stores all fields always"),
			},
			wantFire:   true,
			wantMember: "R-cache-no-pii",
		},
		{
			name: "must vs must-not on same subject fire",
			reqs: []ontology.Requirement{
				settledClaim("R-must-log", "team-a", "the gateway must log every request"),
				settledClaim("R-must-not-log", "team-b", "the gateway must not log every request"),
			},
			wantFire:   true,
			wantMember: "R-must-log",
		},
		{
			name: "only vs any on same subject fire",
			reqs: []ontology.Requirement{
				settledClaim("R-only-admins", "team-a", "only admins may delete a project"),
				settledClaim("R-any-user", "team-b", "any user may delete a project"),
			},
			wantFire:   true,
			wantMember: "R-only-admins",
		},
		{
			name: "high token overlap with different owners fires",
			reqs: []ontology.Requirement{
				settledClaim("R-billing-retries-a", "team-billing", "billing retries failed payments within five minutes"),
				settledClaim("R-billing-retries-b", "team-payments", "billing retries failed payments within one hour"),
			},
			wantFire:   true,
			wantMember: "R-billing-retries-a",
		},
		{
			name: "high token overlap but same owner does not fire",
			reqs: []ontology.Requirement{
				settledClaim("R-same-owner-a", "team-billing", "billing retries failed payments within five minutes"),
				settledClaim("R-same-owner-b", "team-billing", "billing retries failed payments within one hour"),
			},
			wantFire: false,
		},
		{
			name: "unrelated claims, different owners, no marker do not fire",
			reqs: []ontology.Requirement{
				settledClaim("R-unrelated-a", "team-a", "the frontend renders a login page"),
				settledClaim("R-unrelated-b", "team-b", "the database backs up nightly"),
			},
			wantFire: false,
		},
		{
			name: "unrelated claims that both happen to contain a marker word but share no topical token do not fire",
			reqs: []ontology.Requirement{
				settledClaim("R-marker-noise-a", "team-a", "the frontend must render a login page within one second"),
				settledClaim("R-marker-noise-b", "team-b", "only the finance team may approve refunds over one thousand dollars"),
			},
			wantFire: false,
		},
		{
			name: "DRAFT requirements are excluded even if they would otherwise fire",
			reqs: []ontology.Requirement{
				{ID: "R-draft-a", Owner: "team-a", Status: ontology.StatusDRAFT, Claim: "cache must never store PII"},
				{ID: "R-draft-b", Owner: "team-b", Status: ontology.StatusDRAFT, Claim: "cache stores all fields always"},
			},
			wantFire: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := &ontology.Graph{Requirements: tc.reqs}
			candidates := InspectLexicalClaimOverlap(g)

			fired := len(candidates) > 0
			if fired != tc.wantFire {
				t.Fatalf("InspectLexicalClaimOverlap fired=%v, want %v; candidates=%+v", fired, tc.wantFire, candidates)
			}
			if !tc.wantFire {
				return
			}
			found := false
			for _, c := range candidates {
				for _, m := range c.Members {
					if m == tc.wantMember {
						found = true
					}
				}
				if c.Heuristic != HeuristicLexicalClaimOverlap {
					t.Errorf("candidate heuristic = %q, want %q", c.Heuristic, HeuristicLexicalClaimOverlap)
				}
				if c.Evidence == "" {
					t.Errorf("candidate evidence must not be empty: %+v", c)
				}
				if !strings.Contains(c.Recommendation, "ADVISORY") {
					t.Errorf("candidate recommendation must be explicitly ADVISORY, got %q", c.Recommendation)
				}
			}
			if !found {
				t.Errorf("expected member %q among fired candidates, got %+v", tc.wantMember, candidates)
			}
		})
	}
}

func TestInspectLexicalClaimOverlap_ScoreOrdering(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Requirements: []ontology.Requirement{
		settledClaim("R-weak-overlap-a", "team-a", "the service handles retries gracefully"),
		settledClaim("R-weak-overlap-b", "team-b", "the service handles timeouts gracefully"),
		settledClaim("R-marker-a", "team-a", "access is only granted to owners"),
		settledClaim("R-marker-b", "team-b", "access is granted to any employee"),
	}}
	candidates := InspectLexicalClaimOverlap(g)
	if len(candidates) < 2 {
		t.Fatalf("expected at least 2 candidates, got %d: %+v", len(candidates), candidates)
	}
	for i := 1; i < len(candidates); i++ {
		if candidates[i].Score > candidates[i-1].Score {
			t.Errorf("candidates not sorted by descending score: [%d]=%d > [%d]=%d", i, candidates[i].Score, i-1, candidates[i-1].Score)
		}
	}
}

func TestAllCandidates_ExitZeroShapeAlwaysAdvisory(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Requirements: []ontology.Requirement{
		settledClaim("R-a", "team-a", "cache must never store PII"),
		settledClaim("R-b", "team-a", "cache stores all fields always"),
	}}
	candidates := AllCandidates(g)
	for _, c := range candidates {
		if c.ID == "" {
			t.Errorf("candidate missing ID: %+v", c)
		}
		if !strings.Contains(c.Recommendation, "ADVISORY") {
			t.Errorf("candidate %q recommendation must say ADVISORY: %q", c.ID, c.Recommendation)
		}
	}
}

package diagnose

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
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

// rareWordPool backs buildFrequencyCorpus: synthetic (non-business) filler
// nouns used purely so each generated requirement can carry its own
// near-unique token.
var rareWordPool = []string{
	"widget", "gadget", "sprocket", "gizmo", "trinket", "doohickey",
	"contraption", "apparatus", "mechanism", "instrument", "device",
	"appliance", "gubbins", "thingamajig", "contrivance", "implement",
	"utensil", "tool", "machine", "engine", "motor", "turbine",
	"generator", "actuator", "sensor", "transducer", "regulator",
	"governor", "flywheel", "pulley", "lever", "cog", "spindle",
	"bearing", "piston", "valve", "hinge", "bracket", "bolt", "rivet",
	"gasket", "coupling", "shaft", "rotor", "stator",
}

// frequencyCorpusSize returns the smallest n such that a token occurring in
// exactly one of n claims stays strictly under CorpusCommonTokenFraction (so
// buildFrequencyCorpus's per-claim rare words are genuinely NOT excluded),
// while remaining at or above MinCorpusSizeForFrequencyFilter (so the
// frequency-exclusion layer is actually engaged, not bypassed by the
// small-corpus guard). This makes the test fixtures self-consistent with
// whatever CorpusCommonTokenFraction/MinCorpusSizeForFrequencyFilter are
// currently set to, instead of hand-computed magic numbers that silently
// drift out of sync if either constant changes.
func frequencyCorpusSize() int {
	n := MinCorpusSizeForFrequencyFilter
	for float64(1) > CorpusCommonTokenFraction*float64(n) {
		n++
	}
	if n > len(rareWordPool) {
		panic(fmt.Sprintf("frequencyCorpusSize: computed n=%d exceeds rareWordPool=%d — extend the pool", n, len(rareWordPool)))
	}
	return n
}

// buildFrequencyCorpus returns n SETTLED requirements where every claim
// shares the token "system" (a stand-in for a domain-frequent connective
// word, document frequency 100%) plus one UNIQUE rare token per requirement
// drawn from rareWordPool (so each rare token's document frequency is 1/n).
// Every other word in the sentence template ("shall") is a stopWords entry,
// so it never reaches claimTokens in the first place and cannot confound the
// frequency measurement — only "system" and the one rare word per claim are
// significant tokens at all. Used to exercise corpusCommonTokens /
// claimTokens' frequency-exclusion layer without hardcoding any real
// business vocabulary into the test (the domain content here — "system",
// "widget", "gadget" — is synthetic fixture data, not framework logic).
func buildFrequencyCorpus(n int) []ontology.Requirement {
	if n > len(rareWordPool) {
		panic(fmt.Sprintf("buildFrequencyCorpus: n=%d exceeds rareWordPool=%d — rare tokens would repeat and stop being rare", n, len(rareWordPool)))
	}
	out := make([]ontology.Requirement, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, settledClaim(
			fmt.Sprintf("R-freq-%02d", i),
			"team-a",
			fmt.Sprintf("system shall %s", rareWordPool[i]),
		))
	}
	return out
}

// TestCorpusCommonTokens_BelowMinCorpusSizeIsNoOp proves the minimum-corpus
// guard: with fewer than MinCorpusSizeForFrequencyFilter SETTLED
// requirements, corpusCommonTokens must return an empty set even when a
// token appears in every single claim (100% document frequency) — small
// fixtures (like the table-driven tests above, 2 requirements each) must
// keep the plain stop-word-only behavior the existing tests depend on.
func TestCorpusCommonTokens_BelowMinCorpusSizeIsNoOp(t *testing.T) {
	t.Parallel()
	n := MinCorpusSizeForFrequencyFilter - 1
	g := &ontology.Graph{Requirements: buildFrequencyCorpus(n)}
	common := corpusCommonTokens(g)
	if len(common) != 0 {
		t.Fatalf("corpusCommonTokens on a %d-requirement corpus (below the %d-requirement guard) = %v, want empty",
			n, MinCorpusSizeForFrequencyFilter, common)
	}
}

// TestCorpusCommonTokens_ExcludesHighFrequencyTokens proves the core
// frequency-exclusion mechanism at/above the corpus-size guard: "system"
// (document frequency 100%, present in every claim) must be excluded, while
// a rare per-claim token (document frequency 1/n) must NOT be excluded. n
// comes from frequencyCorpusSize() so the fixture stays correct even if
// CorpusCommonTokenFraction / MinCorpusSizeForFrequencyFilter are retuned.
func TestCorpusCommonTokens_ExcludesHighFrequencyTokens(t *testing.T) {
	t.Parallel()
	n := frequencyCorpusSize()
	g := &ontology.Graph{Requirements: buildFrequencyCorpus(n)}
	common := corpusCommonTokens(g)
	if _, ok := common["system"]; !ok {
		t.Errorf("corpusCommonTokens = %v, want it to contain \"system\" (document frequency 100%% over %d claims)", common, n)
	}
	if _, ok := common["widget"]; ok {
		t.Errorf("corpusCommonTokens = %v, want it to NOT contain \"widget\" (document frequency 1/%d = %.1f%%, under the %.0f%% ceiling)",
			common, n, 100.0/float64(n), CorpusCommonTokenFraction*100)
	}
}

// TestInspectLexicalClaimOverlap_CorpusCommonSuppressesGenericOnlyOverlap is
// the non-vacuous, both-directions proof that frequency-based exclusion
// changes InspectLexicalClaimOverlap's real firing behavior on a corpus
// large enough for the guard to engage AND large enough that a token shared
// by 2 of the claims (the "shared-rare" pair below) still stays under the
// ceiling — the base corpus size is doubled from frequencyCorpusSize() (which
// only guarantees a 1-occurrence token clears the ceiling) specifically so a
// 2-occurrence token clears it too.
//
// R-generic-only-a/b share ONLY the corpus-common word "system" — their
// other words ("persist configuration" vs "reload settings") are
// deliberately disjoint, and they have different owners (satisfying the
// non-marker firing condition) — so without the frequency-exclusion fix they
// would fire on "system" alone; with the fix they must NOT fire, since
// "system" is excluded from the overlap count entirely. R-shared-rare-a/b
// ALSO share "system" but additionally share genuinely rare, narrow tokens
// (encrypt/telemetry/payloads) and must still fire — proving the fix does
// not blanket-suppress the heuristic, only the corpus-common contribution.
func TestInspectLexicalClaimOverlap_CorpusCommonSuppressesGenericOnlyOverlap(t *testing.T) {
	t.Parallel()
	baseN := frequencyCorpusSize() * 2
	base := buildFrequencyCorpus(baseN)

	genericOnlyA := settledClaim("R-generic-only-a", "team-a", "the system shall persist configuration")
	genericOnlyB := settledClaim("R-generic-only-b", "team-b", "the system shall reload settings")

	sharedRareA := settledClaim("R-shared-rare-a", "team-a", "the system shall encrypt telemetry payloads before transit")
	sharedRareB := settledClaim("R-shared-rare-b", "team-b", "the system shall encrypt telemetry payloads before storage")

	g := &ontology.Graph{Requirements: append(append([]ontology.Requirement{}, base...), genericOnlyA, genericOnlyB, sharedRareA, sharedRareB)}
	total := len(g.Requirements)
	if ceiling := CorpusCommonTokenFraction * float64(total); 2 > ceiling {
		t.Fatalf("test setup: a 2-occurrence token (2/%d=%.1f%%) does not clear CorpusCommonTokenFraction ceiling %.1f%% — grow baseN",
			total, 200.0/float64(total), ceiling*100.0/float64(total))
	}
	candidates := InspectLexicalClaimOverlap(g)

	for _, c := range candidates {
		if containsBoth(c.Members, "R-generic-only-a", "R-generic-only-b") {
			t.Errorf("R-generic-only-a/b fired as a candidate (%+v) — they share only the corpus-common word \"system\", which must be excluded from the overlap count", c)
		}
	}

	found := false
	for _, c := range candidates {
		if containsBoth(c.Members, "R-shared-rare-a", "R-shared-rare-b") {
			found = true
			if !strings.Contains(c.Evidence, "encrypt") && !strings.Contains(c.Evidence, "telemetry") && !strings.Contains(c.Evidence, "payloads") {
				t.Errorf("R-shared-rare-a/b evidence = %q, want it to cite the genuinely shared rare tokens", c.Evidence)
			}
		}
	}
	if !found {
		t.Errorf("R-shared-rare-a/b did not fire — they share rare, narrow tokens (encrypt/telemetry/payloads) beyond the corpus-common \"system\" and should still be flagged")
	}
}

func containsBoth(members []string, a, b string) bool {
	hasA, hasB := false, false
	for _, m := range members {
		if m == a {
			hasA = true
		}
		if m == b {
			hasB = true
		}
	}
	return hasA && hasB
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

// TestInspect_DeterministicShortlist enforces R-tension-audit-shortlist-tool:
// the inspect engine shall emit a DETERMINISTIC, LLM-free shortlist. Proven by
// running AllCandidates twice over the same fixture and asserting byte-identical
// JSON. The fixture carries a known opposite-marker tension pair (never/always)
// so AllCandidates produces at least one Candidate — an empty==empty comparison
// would be vacuous, so the Fatal on zero candidates guards against that.
//
// If any heuristic ever introduced map-iteration ordering or time-dependent
// state into its output, the two marshalings would diverge and this test fails.
func TestInspect_DeterministicShortlist(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Requirements: []ontology.Requirement{
		settledClaim("R-cache-no-pii", "team-a", "cache must never store PII"),
		settledClaim("R-cache-all-fields", "team-a", "cache stores all fields always"),
	}}
	first := AllCandidates(g)
	if len(first) == 0 {
		t.Fatal("fixture produced zero candidates — determinism check would be vacuous")
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first run: %v", err)
	}
	second := AllCandidates(g)
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second run: %v", err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Errorf("R-tension-audit-shortlist-tool: AllCandidates not deterministic over the same fixture —\nfirst=%s\nsecond=%s",
			firstJSON, secondJSON)
	}
}

// TestInspect_PresentsOnly_NeverMutatesGraph enforces
// R-tension-audit-presents-only: the inspect engine is advisory/read-only and
// shall NEVER mutate graph state. Proven by marshaling the fixture graph to JSON
// before and after AllCandidates runs and asserting the bytes are identical.
// AllCandidates runs every inspect heuristic, so any write to the graph (an
// appended Requirement, a mutated Conflict, a touched slice) changes the
// serialization and fails the test. The Fatal on zero candidates ensures the
// engine actually executed over the fixture rather than trivially no-op'ing.
func TestInspect_PresentsOnly_NeverMutatesGraph(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Requirements: []ontology.Requirement{
		settledClaim("R-cache-no-pii", "team-a", "cache must never store PII"),
		settledClaim("R-cache-all-fields", "team-a", "cache stores all fields always"),
	}}
	before, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal before: %v", err)
	}
	candidates := AllCandidates(g)
	if len(candidates) == 0 {
		t.Fatal("fixture produced zero candidates — read-only check would be vacuous (nothing ran that could mutate)")
	}
	after, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("R-tension-audit-presents-only: AllCandidates mutated the graph —\nbefore=%s\nafter=%s",
			before, after)
	}
}

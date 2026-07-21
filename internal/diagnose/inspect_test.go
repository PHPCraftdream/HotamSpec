package diagnose

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
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

// gateCorpusClaims returns 13 claim strings shaped like the real pilot
// scenario that surfaced this bug: several requirements share common
// requirement-prose vocabulary ("gate", "approve") at genuinely high
// in-corpus frequency (10/13 and 8/13), alongside narrow topical tokens
// ("budget", "audit", "export") that occur only once each — exactly the
// shape IsBlockingHit (blocking_hit.go) needs a surviving topical anchor
// from. n must be 13 (len of this slice); callers needing a different N pad
// or trim independently.
func gateCorpusClaims() []string {
	return []string{
		"the gate must be approved by P-G before release",
		"the gate must record who approved it",
		"the gate must never be skipped for production",
		"the gate must log every approve decision",
		"the gate must escalate to P-G on rejection",
		"the gate must be reviewed before P-G signs off",
		"the gate must block on missing approve evidence",
		"the gate must be re-run after P-G requests changes",
		"the gate must never approve without P-G present",
		"the gate must archive its approve history",
		"the budget report is generated monthly",
		"the audit log is retained for one year",
		"the export pipeline runs nightly",
	}
}

func settledClaimsFrom(idPrefix string, claims []string) []ontology.Requirement {
	out := make([]ontology.Requirement, 0, len(claims))
	for i, c := range claims {
		out = append(out, settledClaim(fmt.Sprintf("%s-%02d", idPrefix, i), "team-a", c))
	}
	return out
}

// TestCorpusCommonTokens_SmallCorpusHoleClosed is the regression pin for the
// real bug found while pilot-testing HotamSpec: for any corpus size N with
// CorpusCommonTokenFraction*N < 1 (with CorpusCommonTokenFraction=0.05, that
// is N in [8,19] under the OLD MinCorpusSizeForFrequencyFilter=8 guard, which
// let the filter engage at those sizes), ceiling < 1.0 means ANY token
// occurring in just one SETTLED requirement (n=1) already satisfies n >
// ceiling and gets excluded — wiping out 100% of the corpus vocabulary as a
// mathematical certainty, not an occasional false positive. Discovered via a
// real 13-SETTLED external pilot domain and confirmed live in this repo's OWN
// hotam-dev domain (9 SETTLED) — both squarely in the hole.
//
// The fix raises MinCorpusSizeForFrequencyFilter to
// ceil(1/CorpusCommonTokenFraction)=20, so N=13 now falls BELOW the guard and
// the frequency filter does not engage at all (falls back to the safe
// stop-word-only path) — proving the bug is closed: a narrow,
// single-occurrence topical token ("budget") must SURVIVE, and the
// vocabulary must NOT be wiped out wholesale, exactly the property that was
// violated before the fix.
func TestCorpusCommonTokens_SmallCorpusHoleClosed(t *testing.T) {
	t.Parallel()

	claims := gateCorpusClaims()
	n := len(claims)
	if n < 8 || n > 19 {
		t.Fatalf("test setup: n=%d must stay in the historical [8,19] hole this test guards against", n)
	}
	if ceiling := CorpusCommonTokenFraction * float64(n); ceiling >= 1.0 {
		t.Fatalf("test setup: n=%d no longer produces ceiling<1.0 (ceiling=%.2f) — CorpusCommonTokenFraction changed, adjust n", n, ceiling)
	}
	if n >= MinCorpusSizeForFrequencyFilter {
		t.Fatalf("test setup: n=%d must stay BELOW the current MinCorpusSizeForFrequencyFilter=%d guard — that is the exact fix being pinned (this size no longer engages frequency-based exclusion at all)", n, MinCorpusSizeForFrequencyFilter)
	}

	reqs := settledClaimsFrom("R-hole", claims)
	g := &ontology.Graph{Requirements: reqs}
	common := corpusCommonTokens(g)

	allTokens := claimTokens(strings.Join(claims, " "), nil)
	if len(common) >= len(allTokens) {
		t.Fatalf("corpusCommonTokens wiped out the entire corpus vocabulary (excluded=%d, total=%d) — the bug this test guards against: %v", len(common), len(allTokens), common)
	}
	if len(common) != 0 {
		t.Errorf("corpusCommonTokens = %v, want empty — n=%d is below the fixed MinCorpusSizeForFrequencyFilter=%d guard, so frequency exclusion must be a no-op (stop-words-only fallback)", common, n, MinCorpusSizeForFrequencyFilter)
	}

	for _, narrow := range []string{"budget", "audit", "export"} {
		if _, excluded := common[narrow]; excluded {
			t.Errorf("corpusCommonTokens incorrectly excluded narrow single-occurrence token %q (document frequency 1/%d) — bug reproduced: %v", narrow, n, common)
		}
	}
}

// TestCorpusCommonTokens_GenuinelyCommonTokenStillExcludedAboveGuard proves
// the fix does not simply disable frequency-based exclusion — it only raises
// the corpus-size threshold at which the exclusion math becomes meaningful.
// At N=MinCorpusSizeForFrequencyFilter (the new guard, 20 by construction —
// ceiling reaches exactly 1.0 there), a token appearing in most of the corpus
// ("gate", padded to 15 of 20 claims) must still be excluded as
// corpus-common, while a narrow single-occurrence token ("budget") must still
// survive — the same shape as the hole-closed test above, just at a size
// where the filter is legitimately active rather than a no-op.
func TestCorpusCommonTokens_GenuinelyCommonTokenStillExcludedAboveGuard(t *testing.T) {
	t.Parallel()

	n := MinCorpusSizeForFrequencyFilter
	if ceiling := CorpusCommonTokenFraction * float64(n); ceiling < 1.0 {
		t.Fatalf("test setup: MinCorpusSizeForFrequencyFilter=%d still produces ceiling<1.0 (ceiling=%.2f) — the derivation itself is broken", n, ceiling)
	}

	claims := gateCorpusClaims()
	// Pad with extra "gate"-carrying claims so "gate"'s document frequency
	// stays well over the ceiling at the larger N, while budget/audit/export
	// remain single-occurrence.
	for len(claims) < n {
		claims = append(claims, fmt.Sprintf("the gate must satisfy extra condition number %d", len(claims)))
	}
	claims = claims[:n]

	reqs := settledClaimsFrom("R-guard", claims)
	g := &ontology.Graph{Requirements: reqs}
	common := corpusCommonTokens(g)

	if _, excluded := common["gate"]; !excluded {
		t.Errorf(`corpusCommonTokens did not exclude "gate" at N=%d (well over the %.0f%% ceiling) — fix must not disable frequency filtering once the corpus is large enough, only correct the small-corpus threshold: %v`,
			n, CorpusCommonTokenFraction*100, common)
	}
	for _, narrow := range []string{"budget", "audit", "export"} {
		if _, excluded := common[narrow]; excluded {
			t.Errorf("corpusCommonTokens incorrectly excluded narrow single-occurrence token %q at N=%d: %v", narrow, n, common)
		}
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

// buildEntityStateConflictFixture returns a graph with one EntityType
// ("order") driven by two DIFFERENT Processes whose steps push it to
// disjoint terminal (resting) states — PR-ship lands it on FULFILLED,
// PR-cancel lands it on CANCELLED — the exact shape
// ontology.EntityStateConflictSuspects' disjoint-destination check requires.
// Mirrors internal/ontology/graph_smoke_test.go's buildSmokeGraph fixture
// shape (same package cannot be imported directly, so the equivalent nodes
// are rebuilt here).
func buildEntityStateConflictFixture() *ontology.Graph {
	entityTypes := []ontology.EntityType{
		{
			Slug: "order",
			Lifecycle: ontology.Lifecycle{
				Slug: "order-states",
				States: []ontology.State{
					{Name: "PENDING", Kind: ontology.StateKindInitial},
					{Name: "FULFILLED", Kind: ontology.StateKindQuiescent},
					{Name: "CANCELLED", Kind: ontology.StateKindQuiescent},
				},
				Transitions: []ontology.Transition{
					{Src: "PENDING", Dst: "FULFILLED", Event: "fulfill"},
					{Src: "PENDING", Dst: "CANCELLED", Event: "cancel"},
				},
			},
		},
	}
	processes := []ontology.Process{
		{
			ID: "PR-ship", Lifecycle: ontology.ProcessLifecycle, DrivesEntities: []string{"order"},
			Steps: []ontology.Step{{Name: "ship", RequiresRole: "ops", Invokes: "order.fulfill"}},
		},
		{
			ID: "PR-cancel", Lifecycle: ontology.ProcessLifecycle, DrivesEntities: []string{"order"},
			Steps: []ontology.Step{{Name: "abort", RequiresRole: "ops", Invokes: "order.cancel"}},
		},
	}
	return &ontology.Graph{EntityTypes: entityTypes, Processes: processes}
}

// TestInspectEntityStateConflicts_ScoresBaseline pins
// EntityStateConflictBaselineScore as the exact score every
// InspectEntityStateConflicts candidate carries (the underlying
// ontology.LatentSuspect shape carries no per-pair gradient to scale
// against — see that constant's doc comment), and proves the score clears
// defaultInspectMinScore=5 by construction, fixing the bug this task exists
// to fix (Score was previously always the Go zero value, so this heuristic
// could never clear ANY positive --min-score).
func TestInspectEntityStateConflicts_ScoresBaseline(t *testing.T) {
	t.Parallel()
	g := buildEntityStateConflictFixture()
	candidates := InspectEntityStateConflicts(g)
	if len(candidates) != 1 {
		t.Fatalf("expected exactly 1 entity-state-conflict candidate, got %d: %+v", len(candidates), candidates)
	}
	c := candidates[0]
	if c.Score != EntityStateConflictBaselineScore {
		t.Errorf("Score = %d, want EntityStateConflictBaselineScore = %d", c.Score, EntityStateConflictBaselineScore)
	}
	if c.Score < defaultInspectMinScoreForTests {
		t.Errorf("Score = %d must clear the default --min-score threshold %d — this is the exact bug this task fixes", c.Score, defaultInspectMinScoreForTests)
	}
	if c.Heuristic != HeuristicEntityStateConflict {
		t.Errorf("Heuristic = %q, want %q", c.Heuristic, HeuristicEntityStateConflict)
	}
}

// defaultInspectMinScoreForTests mirrors cmd/hotam's defaultInspectMinScore
// (5) without importing the cmd package (internal/diagnose must not depend
// on cmd/hotam). Kept as a named constant, not a bare literal, so the intent
// ("the new baseline scores must clear the CLI's real default threshold") is
// self-documenting at the call site.
const defaultInspectMinScoreForTests = 5

// buildAxisCoReferenceFixture returns a graph with two SEPARATE Conflict
// nodes that share the same Axis — the exact shape InspectAxisCoReference
// looks for. C-floor has the minimum 2 members each side (the smallest a
// real Conflict node can be), exercising axisCoReferenceScore's floor case;
// callers that want to test the member-count bonus add extra members on top
// of this shape.
func buildAxisCoReferenceFixture() *ontology.Graph {
	return &ontology.Graph{
		Conflicts: []ontology.Conflict{
			{ID: "C-floor-a", Axis: "shared-axis", Members: []string{"R-1", "R-2"}},
			{ID: "C-floor-b", Axis: "shared-axis", Members: []string{"R-3", "R-4"}},
		},
	}
}

// TestInspectAxisCoReference_FloorScoresAtDefaultThreshold pins the minimal
// case — two Conflicts sharing an axis, each carrying the smallest possible
// membership (2) — to AxisCoReferenceBaselineScore exactly (no member bonus
// at the floor), and proves this floor case clears
// defaultInspectMinScoreForTests: the task's own real-graph example
// (hotam-spec-self's "core-vs-aspect" axis, C-8600b1b8 + C-be22cdd1, both
// 2-member Conflicts) is exactly this floor shape, and was the single
// candidate the review flagged as always-invisible before this fix (Score
// was previously unset/0 for every InspectAxisCoReference candidate).
func TestInspectAxisCoReference_FloorScoresAtDefaultThreshold(t *testing.T) {
	t.Parallel()
	g := buildAxisCoReferenceFixture()
	candidates := InspectAxisCoReference(g)
	if len(candidates) != 1 {
		t.Fatalf("expected exactly 1 axis-co-reference candidate, got %d: %+v", len(candidates), candidates)
	}
	c := candidates[0]
	if c.Score != AxisCoReferenceBaselineScore {
		t.Errorf("Score = %d, want AxisCoReferenceBaselineScore = %d (floor case: 2+2=4 combined members, at AxisCoReferenceMemberFloor)", c.Score, AxisCoReferenceBaselineScore)
	}
	if c.Score < defaultInspectMinScoreForTests {
		t.Errorf("Score = %d must clear the default --min-score threshold %d — this is the exact bug this task fixes", c.Score, defaultInspectMinScoreForTests)
	}
	if c.Heuristic != HeuristicAxisCoReference {
		t.Errorf("Heuristic = %q, want %q", c.Heuristic, HeuristicAxisCoReference)
	}
}

// TestInspectAxisCoReference_MemberBonusScalesAboveFloor proves
// axisCoReferenceScore rewards combined Conflict membership beyond the
// 2+2=4 floor: three extra members (5 total on one side + 2 on the other =
// 7 combined, 3 over the floor of 4) must score exactly
// AxisCoReferenceBaselineScore+3, strictly higher than the floor-case score
// pinned above.
func TestInspectAxisCoReference_MemberBonusScalesAboveFloor(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Conflicts: []ontology.Conflict{
			{ID: "C-broad-a", Axis: "shared-axis", Members: []string{"R-1", "R-2", "R-3", "R-4", "R-5"}},
			{ID: "C-broad-b", Axis: "shared-axis", Members: []string{"R-6", "R-7"}},
		},
	}
	candidates := InspectAxisCoReference(g)
	if len(candidates) != 1 {
		t.Fatalf("expected exactly 1 axis-co-reference candidate, got %d: %+v", len(candidates), candidates)
	}
	want := AxisCoReferenceBaselineScore + 3 // combined 7 members, 3 over the 4-member floor
	if candidates[0].Score != want {
		t.Errorf("Score = %d, want %d (baseline %d + 3-member bonus)", candidates[0].Score, want, AxisCoReferenceBaselineScore)
	}
}

// TestInspectSharedAssumptionClusters_FloorAndDensityBonus pins
// SharedAssumptionClusterBaselineScore for the minimal 1-pair cluster (2
// requirements sharing one specific assumption, no mediating Conflict) and
// proves clusterDensityBonus adds 1 point per pair beyond that floor, using
// the same 3-requirement/2-pair shape as
// internal/ontology/graph_smoke_test.go's TestLatentConnectorSuspectsAndClusters
// fixture (R-1/R-2/R-3 all sharing A-single-customer, fully pairwise
// connected since none of the pairs are already mediated by a Conflict).
func TestInspectSharedAssumptionClusters_FloorAndDensityBonus(t *testing.T) {
	t.Parallel()

	// Minimal case: exactly one pair.
	floorGraph := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", Owner: "team-a", Status: ontology.StatusSETTLED, Assumptions: []string{"A-shared"}},
			{ID: "R-2", Owner: "team-a", Status: ontology.StatusSETTLED, Assumptions: []string{"A-shared"}},
		},
		Assumptions: []ontology.Assumption{{ID: "A-shared", Status: ontology.AssumptionHOLDS}},
	}
	floorCandidates := InspectSharedAssumptionClusters(floorGraph)
	if len(floorCandidates) != 1 {
		t.Fatalf("expected exactly 1 cluster candidate, got %d: %+v", len(floorCandidates), floorCandidates)
	}
	if floorCandidates[0].Score != SharedAssumptionClusterBaselineScore {
		t.Errorf("floor cluster Score = %d, want SharedAssumptionClusterBaselineScore = %d", floorCandidates[0].Score, SharedAssumptionClusterBaselineScore)
	}

	// Denser case: 3 requirements all sharing the same assumption, no
	// mediating Conflict for any pair -> C(3,2)=3 contributing pairs.
	denseGraph := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", Owner: "team-a", Status: ontology.StatusSETTLED, Assumptions: []string{"A-shared"}},
			{ID: "R-2", Owner: "team-a", Status: ontology.StatusSETTLED, Assumptions: []string{"A-shared"}},
			{ID: "R-3", Owner: "team-a", Status: ontology.StatusSETTLED, Assumptions: []string{"A-shared"}},
		},
		Assumptions: []ontology.Assumption{{ID: "A-shared", Status: ontology.AssumptionHOLDS}},
	}
	denseCandidates := InspectSharedAssumptionClusters(denseGraph)
	if len(denseCandidates) != 1 {
		t.Fatalf("expected exactly 1 cluster candidate, got %d: %+v", len(denseCandidates), denseCandidates)
	}
	wantDense := SharedAssumptionClusterBaselineScore + 2 // 3 pairs, 2 over the 1-pair floor
	if denseCandidates[0].Score != wantDense {
		t.Errorf("dense cluster Score = %d, want %d (baseline %d + density bonus for 3 pairs)", denseCandidates[0].Score, wantDense, SharedAssumptionClusterBaselineScore)
	}
	if denseCandidates[0].Score <= floorCandidates[0].Score {
		t.Errorf("denser cluster (3 pairs) must score strictly higher than the 1-pair floor: dense=%d floor=%d", denseCandidates[0].Score, floorCandidates[0].Score)
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

// knownLexicalOverlapMissByDesign lists every real hotam-spec-self Conflict
// member pair task #99 verified InspectLexicalClaimOverlap does NOT (and, for
// 7 of the 8, never did) flag as a candidate, together with the reason. This
// is the ground-truth allow-list for
// TestInspectLexicalClaimOverlap_KnownConflictGroundTruth below: any pair NOT
// in this map must fire, or the test fails — that is the actual regression
// guard against a future corpus-frequency (or other) change silently
// widening the miss set beyond what has been manually verified here.
//
// Two different reasons show up:
//
//  1. Seven pairs miss independently of corpusCommonTokens — even with the
//     frequency filter fully disabled (common=nil, stop-words-only, the
//     pre-fix behavior), none of them clears
//     MinLexicalOverlapTokens/MinLexicalOverlapTokensWithMarker given their
//     actual shared-token count, owner, and marker state. They were
//     discovered by the resolver through channels other than lexical overlap
//     (semantic judgment, shared-assumption/entity-state/axis signals, or
//     plain human review) — exactly what R-tension-audit-shortlist-tool's own
//     why documents as "0 of 8 conflicts machine-surfaced over its whole
//     history." Verified by hand against the corpus as of 2026-07-13:
//
//     C-06e2d84e R-content-free-framework+R-crystallize-knowledge-to-code:    0 shared tokens even stop-words-only
//     C-186c4347 R-agent-never-lost+R-ai-presents-not-decides:                1 shared ("agent"), SAME owner, no split marker -> below threshold=2
//     C-7f86e41d R-budget-measure+R-operator-prompt-from-substrate:           1 shared ("operator") -> below threshold=2
//     C-8600b1b8 R-content-free-framework+R-agent-never-lost:                 1 shared ("hotam") -> below threshold=2
//     C-be22cdd1 R-entity-derived-requirement+R-speculative-aspects-frozen:   2 shared, but SAME owner and only ONE side carries the only/any marker (not a split) -> does not fire
//     C-d210d6d0 R-context-bounded-delegation+R-crystallize-knowledge-to-code: 1 shared ("operator") -> below threshold=2
//     C-d4f3eadf R-context-bounded-delegation+R-dependency-graph-parallelism:  1 shared ("sub") -> below threshold=2
//
//  2. One pair, C-c3911f28 (R-content-free-framework +
//     R-empty-content-is-legitimate), IS a genuine corpus-frequency false
//     negative: with the filter disabled it shares 3 tokens
//     ("content"/"graph"/"spec") across different owners and clears the
//     firing bar; with CorpusCommonTokenFraction=0.05 active all three sit
//     at 5.5%/19.9%/7.6% document frequency (comfortably over the 5%
//     ceiling) and get excluded, dropping shared tokens to 0. A threshold
//     sweep (task #99) showed rescuing it is not worth the cost: 0.05->0.06
//     alone (just letting "content" back in) grows total
//     lexical_claim_overlap candidates from 232 to 316 (+36%); fully
//     rescuing all three tokens needs frac=0.20, which explodes candidates
//     to 1186 — undoing essentially all of the 1231->234 noise fix
//     R-tension-audit-shortlist-tool's NOISE FIX note describes. Accepted as
//     a documented tradeoff rather than tuned: this Conflict is ALREADY a
//     graph ground-truth node (no heuristic needs to "discover" it), and
//     inspect's purpose per its own top-of-file doc comment is surfacing NEW
//     undiscovered tension candidates, not reproducing every known Conflict.
var knownLexicalOverlapMissByDesign = map[[2]string]struct{}{
	{"R-agent-never-lost", "R-content-free-framework"}:                  {},
	{"R-agent-never-lost", "R-ai-presents-not-decides"}:                 {},
	{"R-budget-measure", "R-operator-prompt-from-substrate"}:            {},
	{"R-content-free-framework", "R-crystallize-knowledge-to-code"}:     {},
	{"R-context-bounded-delegation", "R-crystallize-knowledge-to-code"}: {},
	{"R-context-bounded-delegation", "R-dependency-graph-parallelism"}:  {},
	{"R-entity-derived-requirement", "R-speculative-aspects-frozen"}:    {},
	{"R-content-free-framework", "R-empty-content-is-legitimate"}:       {}, // proven corpus-frequency false negative, accepted (see above)
}

// TestInspectLexicalClaimOverlap_KnownConflictGroundTruth is the honest
// anti-false-negative pin task #99 asked for: it uses the graph's OWN
// resolver-confirmed Conflict nodes (domains/hotam-spec-self/graph.json's
// `conflicts` array) as ground truth — not a synthetic fixture — and checks
// every pairwise Conflict-member combination against
// InspectLexicalClaimOverlap's real output on the real graph.
//
// A pair in knownLexicalOverlapMissByDesign (documented above) is allowed to
// miss — asserting otherwise would be a false expectation the task
// instructions explicitly warned against. Any OTHER real Conflict pair
// (including any newly-added Conflict) must fire as a lexical_claim_overlap
// candidate, or this test fails: that is the regression guard against a
// future corpus-frequency (or other) change silently widening the miss set
// beyond what has been manually verified.
func TestInspectLexicalClaimOverlap_KnownConflictGroundTruth(t *testing.T) {
	t.Parallel()
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}

	fired := map[[2]string]Candidate{}
	for _, c := range InspectLexicalClaimOverlap(g) {
		if len(c.Members) != 2 {
			continue
		}
		key := memberKey(c.Members[0], c.Members[1])
		fired[key] = c
	}

	checked := 0
	for _, conflict := range g.Conflicts {
		members := conflict.Members
		for i := 0; i < len(members); i++ {
			for j := i + 1; j < len(members); j++ {
				key := memberKey(members[i], members[j])
				checked++
				_, didFire := fired[key]

				if _, knownMiss := knownLexicalOverlapMissByDesign[key]; knownMiss {
					if didFire {
						t.Logf("%s member pair %v now fires (previously a documented miss) — heuristic improvement; safe to remove from knownLexicalOverlapMissByDesign", conflict.ID, key)
					}
					continue
				}

				if !didFire {
					t.Errorf("%s member pair %v (a real resolver-confirmed Conflict) is NOT a lexical_claim_overlap candidate and is not in knownLexicalOverlapMissByDesign — this is either a new corpus-frequency false negative (investigate CorpusCommonTokenFraction impact) or a legitimately lexical-overlap-free conflict that needs documenting in knownLexicalOverlapMissByDesign with the reasoning", conflict.ID, key)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no Conflict member pairs checked — domains/hotam-spec-self/graph.json's conflicts array appears empty; ground-truth check would be vacuous")
	}
	t.Logf("checked %d real Conflict member pairs against lexical_claim_overlap candidates", checked)
}

// memberKey returns a and b as a canonically sorted 2-tuple, so lookups
// against knownLexicalOverlapMissByDesign don't depend on Conflict.Members
// or Candidate.Members ordering.
func memberKey(a, b string) [2]string {
	if a > b {
		a, b = b, a
	}
	return [2]string{a, b}
}

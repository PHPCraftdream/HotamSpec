package diagnose

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// rejectedClaimWithSuccessor builds a REJECTED requirement id whose successor
// (a SETTLED requirement carrying a `replaces` edge pointing at id) is also in
// g, so Confront's ReplacedBy lookup has a real REPLACES chain to find.
func rejectedClaimWithSuccessor(id, claim, successorID string) (ontology.Requirement, ontology.Requirement) {
	rejected := ontology.Requirement{
		ID:     id,
		Owner:  "team-a",
		Status: ontology.StatusREJECTED,
		Claim:  claim,
	}
	successor := ontology.Requirement{
		ID:     successorID,
		Owner:  "team-a",
		Status: ontology.StatusSETTLED,
		Claim:  "the replacement claim that supersedes " + id,
		Relations: []ontology.Relation{
			{Kind: "replaces", Target: id},
		},
	}
	return rejected, successor
}

// TestConfront_UniqueCandidateIsClear is the negative branch: a candidate that
// shares no significant tokens with any SETTLED or REJECTED requirement yields
// Clear=true and empty hit lists on both sides.
func TestConfront_UniqueCandidateIsClear(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			settledClaim("R-zorblatt", "team-a", "the frobnicator shall quux the blargh nightly"),
		},
	}
	res := Confront(g, "a totally unrelated novel idea about quantum banana scheduling")
	if !res.Clear {
		t.Fatalf("Clear = false, want true; got %d settled / %d rejected hits",
			len(res.Settled), len(res.Rejected))
	}
	if len(res.Settled) != 0 || len(res.Rejected) != 0 {
		t.Errorf("expected empty hit lists, got settled=%v rejected=%v", res.Settled, res.Rejected)
	}
}

// TestConfront_ClearResult_JSONArraysNotNull is the byte-level regression pin
// for the P2 "empty arrays instead of null in JSON output" fix: on a clear
// result (the common `hotam confront --json` outcome — no overlap found),
// the marshaled ConfrontResult must literally contain `"settled":[]` and
// `"rejected":[]`, never `"settled":null` / `"rejected":null`. A Go-level
// len()==0 check does not prove what encoding/json actually emits for a nil
// vs. empty slice, so this asserts the raw marshaled bytes.
func TestConfront_ClearResult_JSONArraysNotNull(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			settledClaim("R-zorblatt", "team-a", "the frobnicator shall quux the blargh nightly"),
		},
	}
	res := Confront(g, "a totally unrelated novel idea about quantum banana scheduling")
	if !res.Clear {
		t.Fatalf("Clear = false, want true (fixture must exercise the empty-array path)")
	}

	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)

	if !strings.Contains(raw, `"settled":[]`) {
		t.Errorf("expected literal `\"settled\":[]` in marshaled JSON, got:\n%s", raw)
	}
	if strings.Contains(raw, `"settled":null`) {
		t.Errorf("marshaled JSON must never contain `\"settled\":null`, got:\n%s", raw)
	}
	if !strings.Contains(raw, `"rejected":[]`) {
		t.Errorf("expected literal `\"rejected\":[]` in marshaled JSON, got:\n%s", raw)
	}
	if strings.Contains(raw, `"rejected":null`) {
		t.Errorf("marshaled JSON must never contain `\"rejected\":null`, got:\n%s", raw)
	}
}

// TestConfront_VerbatimSettledClaimIsDuplicate is the positive SETTLED branch:
// a candidate that IS a settled claim must surface that requirement as a
// duplicate suspect. Reading the real claim back from the graph (rather than
// hardcoding it) keeps the test robust to claim edits.
func TestConfront_VerbatimSettledClaimIsDuplicate(t *testing.T) {
	t.Parallel()
	settled := settledClaim("R-keep-logs", "team-a", "the gateway must retain audit logs for ninety days")
	g := &ontology.Graph{Requirements: []ontology.Requirement{settled}}

	res := Confront(g, settled.Claim)
	if res.Clear {
		t.Fatalf("Clear = true, want false: verbatim settled claim must be flagged")
	}
	if len(res.Settled) != 1 {
		t.Fatalf("expected exactly 1 settled hit, got %d (%+v)", len(res.Settled), res.Settled)
	}
	hit := res.Settled[0]
	if hit.ID != "R-keep-logs" {
		t.Errorf("hit.ID = %q, want R-keep-logs", hit.ID)
	}
	if len(hit.Shared) < MinLexicalOverlapTokens {
		t.Errorf("len(Shared) = %d, want >= %d (threshold)", len(hit.Shared), MinLexicalOverlapTokens)
	}
	if hit.Claim != settled.Claim {
		t.Errorf("hit.Claim mismatch: got %q", hit.Claim)
	}
	if len(res.Rejected) != 0 {
		t.Errorf("expected 0 rejected hits for a settled-only graph, got %d", len(res.Rejected))
	}
}

// TestConfront_VerbatimRejectedClaimIsRelitigation is the positive REJECTED
// branch: a candidate matching a REJECTED claim must surface it as a
// re-litigation suspect.
func TestConfront_VerbatimRejectedClaimIsRelitigation(t *testing.T) {
	t.Parallel()
	rejected := ontology.Requirement{
		ID:     "R-dead-rdf-store",
		Owner:  "team-a",
		Status: ontology.StatusREJECTED,
		Claim:  "the framework shall persist every requirement into an rdf triple store",
	}
	g := &ontology.Graph{Requirements: []ontology.Requirement{rejected}}

	res := Confront(g, rejected.Claim)
	if res.Clear {
		t.Fatalf("Clear = true, want false: verbatim rejected claim must be flagged")
	}
	if len(res.Rejected) != 1 {
		t.Fatalf("expected exactly 1 rejected hit, got %d (%+v)", len(res.Rejected), res.Rejected)
	}
	if res.Rejected[0].ID != "R-dead-rdf-store" {
		t.Errorf("rejected hit ID = %q, want R-dead-rdf-store", res.Rejected[0].ID)
	}
	if len(res.Settled) != 0 {
		t.Errorf("expected 0 settled hits, got %d", len(res.Settled))
	}
	if len(res.Rejected[0].ReplacedBy) != 0 {
		t.Errorf("expected empty ReplacedBy (no successor in this graph), got %v", res.Rejected[0].ReplacedBy)
	}
}

// TestConfront_RejectedHitCarriesReplacesSuccessor verifies the anti-
// relitigation chain: when a REJECTED requirement has a known REPLACES successor
// in the graph, the hit's ReplacedBy is populated so the operator can cite the
// replacement instead of re-deriving the rejected idea.
func TestConfront_RejectedHitCarriesReplacesSuccessor(t *testing.T) {
	t.Parallel()
	rejected, successor := rejectedClaimWithSuccessor(
		"R-dead-store", "the framework shall store nodes in a per-node json file", "R-per-node-store")
	g := &ontology.Graph{Requirements: []ontology.Requirement{rejected, successor}}

	res := Confront(g, rejected.Claim)
	if len(res.Rejected) != 1 {
		t.Fatalf("expected 1 rejected hit, got %d", len(res.Rejected))
	}
	hit := res.Rejected[0]
	if hit.ID != "R-dead-store" {
		t.Fatalf("hit.ID = %q, want R-dead-store", hit.ID)
	}
	if len(hit.ReplacedBy) != 1 || hit.ReplacedBy[0] != "R-per-node-store" {
		t.Errorf("ReplacedBy = %v, want [R-per-node-store]", hit.ReplacedBy)
	}
}

// TestConfront_OppositeMarkerLowersThreshold verifies the marker half of the
// inspect threshold logic carries over: a candidate that shares only a single
// significant token with a settled claim still fires when the two use opposite
// markers (never/always) — the canonical "cache must never store PII" vs "cache
// stores all fields always" example from inspect.
func TestConfront_OppositeMarkerLowersThreshold(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			settledClaim("R-cache-no-pii", "team-a", "cache must never store PII"),
		},
	}
	// Shares only the single significant token "cache" (PII/store are on one
	// side only), plus the opposite marker never/always — must still fire.
	res := Confront(g, "cache stores all fields always")
	if res.Clear {
		t.Fatalf("Clear = true, want false: opposite-marker single-token overlap must fire")
	}
	if len(res.Settled) != 1 || res.Settled[0].ID != "R-cache-no-pii" {
		t.Fatalf("expected single hit R-cache-no-pii, got %+v", res.Settled)
	}
	if res.Settled[0].Score < 1+3 {
		t.Errorf("score = %d, want >= 4 (1 shared token + 3 marker bonus)", res.Settled[0].Score)
	}
}

// TestConfront_CorpusCommonWordsAloneDoNotFire is the direct proof of goal
// (2) from the P1 noise-reduction task: on a corpus large enough for
// corpusCommonTokens to engage (>= MinCorpusSizeForFrequencyFilter SETTLED
// requirements, with "requirement" pushed past the 5% document-frequency
// ceiling by repeating it across the filler corpus, mirroring the review's
// own cited false-positive word), a CANDIDATE that shares ONLY corpus-common
// words with a SETTLED requirement must NOT fire — this is the "confront
// flags common words like 'every', 'requirement', 'enforce' as suspicious"
// complaint from the external review, reproduced and closed. The companion
// test TestConfront_RareSharedTokensStillFire proves the other direction
// (candidates sharing genuinely narrow vocabulary still fire), so this pair
// is non-vacuous in both directions per the task's goal (2).
func TestConfront_CorpusCommonWordsAloneDoNotFire(t *testing.T) {
	t.Parallel()
	n := frequencyCorpusSize()
	filler := buildFrequencyCorpus(n)
	// Every filler claim ALSO repeats "every" and "requirement" so both
	// clear the 5% ceiling, mirroring the review's own cited false-positive
	// words, not just the fixture's built-in "system".
	for i := range filler {
		filler[i].Claim = "every requirement: " + filler[i].Claim
	}
	target := settledClaim("R-target", "team-a", "every requirement governs the system")

	g := &ontology.Graph{Requirements: append(append([]ontology.Requirement{}, filler...), target)}

	// Candidate shares ONLY corpus-common words ("every", "requirement",
	// "system") with R-target — "governs" vs "protects" keeps every OTHER
	// token disjoint, so no rare/topical token is shared.
	res := Confront(g, "every requirement protects the system")
	for _, hit := range res.Settled {
		if hit.ID == "R-target" {
			t.Errorf("R-target fired as a duplicate suspect (%+v) on a candidate sharing only corpus-common words (every/requirement/system) — corpus-common exclusion did not engage", hit)
		}
	}
}

// TestConfront_RareSharedTokensStillFire is the positive-direction companion
// to TestConfront_CorpusCommonWordsAloneDoNotFire: a candidate sharing a
// genuinely rare, narrow token with a SETTLED requirement (in addition to
// corpus-common words) must still fire — proving the frequency exclusion
// narrows the signal, it does not silently disable CONFRONT altogether.
func TestConfront_RareSharedTokensStillFire(t *testing.T) {
	t.Parallel()
	n := frequencyCorpusSize()
	filler := buildFrequencyCorpus(n)
	for i := range filler {
		filler[i].Claim = "every requirement: " + filler[i].Claim
	}
	target := settledClaim("R-target-rare", "team-a", "every requirement governs the system quarantine escalation window")

	g := &ontology.Graph{Requirements: append(append([]ontology.Requirement{}, filler...), target)}

	// Shares corpus-common words AND two rare, narrow tokens (quarantine,
	// escalation) — two, not one, because with no opposite marker present
	// MinLexicalOverlapTokens (2) is the threshold that must be cleared.
	res := Confront(g, "every requirement protects the system quarantine escalation period")
	found := false
	for _, hit := range res.Settled {
		if hit.ID == "R-target-rare" {
			found = true
			wantRare := map[string]bool{"quarantine": false, "escalation": false}
			for _, tok := range hit.Shared {
				if _, ok := wantRare[tok]; ok {
					wantRare[tok] = true
				}
			}
			for tok, seen := range wantRare {
				if !seen {
					t.Errorf("R-target-rare hit.Shared = %v, want it to include the rare token %q", hit.Shared, tok)
				}
			}
		}
	}
	if !found {
		t.Errorf("R-target-rare did not fire — it shares the rare, narrow tokens quarantine/escalation with the candidate and should still be flagged")
	}
}

// TestConfront_HitsSortedByScoreDescThenIDAsc verifies deterministic ordering:
// ties break by ID, top score first, so the report is reproducible run-to-run.
func TestConfront_HitsSortedByScoreDescThenIDAsc(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			settledClaim("R-billing-retries-a", "team-a", "billing retries failed payments within five minutes window"),
			settledClaim("R-billing-retries-b", "team-a", "billing retries failed payments within one hour window"),
			settledClaim("R-unrelated", "team-a", "the frobnicator shall quux the blargh nightly"),
		},
	}
	res := Confront(g, "billing retries failed payments within five minutes window")
	if len(res.Settled) < 2 {
		t.Fatalf("expected >= 2 settled hits, got %d", len(res.Settled))
	}
	// The verbatim match (R-billing-retries-a) must outscore the partial match.
	if res.Settled[0].ID != "R-billing-retries-a" {
		t.Errorf("top hit = %q (score %d), want R-billing-retries-a first", res.Settled[0].ID, res.Settled[0].Score)
	}
	for i := 1; i < len(res.Settled); i++ {
		prev, cur := res.Settled[i-1], res.Settled[i]
		if prev.Score < cur.Score {
			t.Errorf("hits not sorted by score desc: [%d]=%d < [%d]=%d", i-1, prev.Score, i, cur.Score)
		}
		if prev.Score == cur.Score && prev.ID > cur.ID {
			t.Errorf("ties not broken by ID asc: %q before %q", prev.ID, cur.ID)
		}
	}
}

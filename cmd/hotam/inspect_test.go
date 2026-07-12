package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
)

// synthCandidate builds a Candidate with the given id and score for the pure
// filtering/formatter tests below — no graph is needed, buildInspectResult and
// formatInspectReport operate on the candidate slice directly.
func synthCandidate(id string, score int) diagnose.Candidate {
	return diagnose.Candidate{
		ID:             id,
		Heuristic:      "test_heuristic",
		Members:        []string{id},
		Evidence:       "synthetic evidence",
		Score:          score,
		Recommendation: "synthetic recommendation",
	}
}

// TestBuildInspectResult_MinScoreFiltersLowSignalCandidates verifies the core
// contract of the --min-score feature: candidates below the threshold are
// counted as suppressed and excluded from the shown slice.
func TestBuildInspectResult_MinScoreFiltersLowSignalCandidates(t *testing.T) {
	t.Parallel()
	all := []diagnose.Candidate{
		synthCandidate("R-a", 0),
		synthCandidate("R-b", 3),
		synthCandidate("R-c", 5),
		synthCandidate("R-d", 8),
	}
	r := buildInspectResult(all, 5, 0) // limit 0 = unlimited

	if r.TotalCandidates != 4 {
		t.Fatalf("TotalCandidates = %d, want 4", r.TotalCandidates)
	}
	if r.SuppressedCount != 2 {
		t.Fatalf("SuppressedCount = %d, want 2 (scores 0 and 3 < 5)", r.SuppressedCount)
	}
	if len(r.Candidates) != 2 {
		t.Fatalf("len(Candidates) = %d, want 2", len(r.Candidates))
	}
	for _, c := range r.Candidates {
		if c.Score < 5 {
			t.Errorf("candidate %q with score %d survived min-score 5 filter", c.ID, c.Score)
		}
	}
	if r.MinScore != 5 {
		t.Errorf("MinScore = %d, want 5", r.MinScore)
	}
}

// TestBuildInspectResult_MinScoreZeroKeepsAll verifies the regression guard:
// --min-score 0 suppresses nothing and shows every candidate (the pre-feature
// behavior).
func TestBuildInspectResult_MinScoreZeroKeepsAll(t *testing.T) {
	t.Parallel()
	all := []diagnose.Candidate{
		synthCandidate("R-a", 0),
		synthCandidate("R-b", 3),
		synthCandidate("R-c", 5),
	}
	r := buildInspectResult(all, 0, 0)

	if r.SuppressedCount != 0 {
		t.Fatalf("SuppressedCount = %d, want 0 for min-score 0", r.SuppressedCount)
	}
	if len(r.Candidates) != len(all) {
		t.Fatalf("len(Candidates) = %d, want %d (min-score 0 keeps all)", len(r.Candidates), len(all))
	}
	if r.TotalCandidates != len(all) {
		t.Fatalf("TotalCandidates = %d, want %d", r.TotalCandidates, len(all))
	}
}

// TestBuildInspectResult_SuppressedPlusShownEqualsTotal pins the invariant the
// e2e test also checks on the real domain: with no display limit, every
// candidate is either shown or counted as suppressed, so the two counts sum to
// the pre-filter total.
func TestBuildInspectResult_SuppressedPlusShownEqualsTotal(t *testing.T) {
	t.Parallel()
	all := []diagnose.Candidate{
		synthCandidate("R-a", 1),
		synthCandidate("R-b", 4),
		synthCandidate("R-c", 5),
		synthCandidate("R-d", 9),
		synthCandidate("R-e", 2),
	}
	r := buildInspectResult(all, 5, 0) // limit 0 = unlimited
	if r.SuppressedCount+len(r.Candidates) != r.TotalCandidates {
		t.Fatalf("suppressed(%d) + shown(%d) != total(%d)",
			r.SuppressedCount, len(r.Candidates), r.TotalCandidates)
	}
}

// TestBuildInspectResult_LimitCapsShownWithoutChangingSuppression verifies the
// display limit is applied AFTER the score filter: it caps how many
// score-survivors are shown but leaves SuppressedCount (the threshold drop
// count) untouched.
func TestBuildInspectResult_LimitCapsShownWithoutChangingSuppression(t *testing.T) {
	t.Parallel()
	all := []diagnose.Candidate{
		synthCandidate("R-a", 5),
		synthCandidate("R-b", 6),
		synthCandidate("R-c", 7),
		synthCandidate("R-d", 1), // below threshold
	}
	r := buildInspectResult(all, 5, 2) // limit 2

	if r.SuppressedCount != 1 {
		t.Fatalf("SuppressedCount = %d, want 1 (limit must not change threshold count)", r.SuppressedCount)
	}
	if len(r.Candidates) != 2 {
		t.Fatalf("len(Candidates) = %d, want 2 (limit cap)", len(r.Candidates))
	}
	if r.TotalCandidates != 4 {
		t.Fatalf("TotalCandidates = %d, want 4 (pre-filter total, independent of limit)", r.TotalCandidates)
	}
}

// TestFormatInspectReport_SuppressedLineAppearsWhenThresholdActive verifies the
// honest-summary contract: when --min-score dropped candidates, the text report
// names the count and how to see everything.
func TestFormatInspectReport_SuppressedLineAppearsWhenThresholdActive(t *testing.T) {
	t.Parallel()
	r := buildInspectResult([]diagnose.Candidate{
		synthCandidate("R-a", 1),
		synthCandidate("R-b", 5),
	}, 5, 0)
	text := formatInspectReport(r)
	if !strings.Contains(text, "suppressed 1 candidate(s) below score threshold 5") {
		t.Errorf("expected suppressed summary line, got:\n%s", text)
	}
	if !strings.Contains(text, "(use --min-score 0 to see all)") {
		t.Errorf("expected --min-score 0 hint, got:\n%s", text)
	}
	if !strings.Contains(text, "at score≥5") {
		t.Errorf("expected header to name the active threshold, got:\n%s", text)
	}
}

// TestFormatInspectReport_NoSuppressedLineWhenMinScoreZero is the text-output
// regression guard: with nothing suppressed the report matches the pre-feature
// shape (no suppressed line, classic "found (showing N)" header).
func TestFormatInspectReport_NoSuppressedLineWhenMinScoreZero(t *testing.T) {
	t.Parallel()
	r := buildInspectResult([]diagnose.Candidate{
		synthCandidate("R-a", 0),
		synthCandidate("R-b", 3),
	}, 0, 0)
	text := formatInspectReport(r)
	if strings.Contains(text, "suppressed") {
		t.Errorf("min-score 0 must not print a suppressed line, got:\n%s", text)
	}
	if !strings.Contains(text, "conflict candidate(s) found (showing 2)") {
		t.Errorf("expected classic header when nothing suppressed, got:\n%s", text)
	}
}

// TestFormatInspectReport_NoCandidatesMessage verifies the empty-graph path is
// unchanged by the feature.
func TestFormatInspectReport_NoCandidatesMessage(t *testing.T) {
	t.Parallel()
	r := buildInspectResult(nil, 5, 0)
	text := formatInspectReport(r)
	if !strings.Contains(text, "no conflict candidates found") {
		t.Errorf("expected no-candidates message, got:\n%s", text)
	}
}

// TestInspectResult_JSONShape verifies the machine-readable contract: the JSON
// payload is an object carrying suppressed_count / total_candidates / min_score
// alongside the candidates array (not a bare array), so callers can audit the
// threshold's effect programmatically.
func TestInspectResult_JSONShape(t *testing.T) {
	t.Parallel()
	r := buildInspectResult([]diagnose.Candidate{
		synthCandidate("R-a", 1),
		synthCandidate("R-b", 5),
	}, 5, 0)
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"suppressed_count", "total_candidates", "min_score", "candidates"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("JSON missing top-level key %q: %s", key, data)
		}
	}
	if got := parsed["suppressed_count"]; got != float64(1) {
		t.Errorf("suppressed_count = %v, want 1", got)
	}
	if got := parsed["total_candidates"]; got != float64(2) {
		t.Errorf("total_candidates = %v, want 2", got)
	}
	cands, ok := parsed["candidates"].([]any)
	if !ok || len(cands) != 1 {
		t.Errorf("candidates = %v, want array of length 1", parsed["candidates"])
	}
}

// TestCmdInspect_E2E_DefaultMinScoreSuppressesNoiseOnRealDomain is the
// end-to-end proof: it runs the REAL hotam binary (shared build) against a copy
// of the hotam-spec-self domain and checks that (a) the default --min-score
// filters a meaningful share of candidates, (b) --min-score 0 returns all of
// them, and (c) the suppressed_count + shown invariant holds. Output is
// captured from a child process (not the global os.Stdout) so it is safe under
// t.Parallel() alongside other cmd/hotam tests.
func TestCmdInspect_E2E_DefaultMinScoreSuppressesNoiseOnRealDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("inspect e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	runJSON := func(args ...string) inspectResult {
		t.Helper()
		cmd := exec.Command(binPath, args...)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("hotam %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
		var r inspectResult
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatalf("parse inspect JSON: %v\nraw:\n%s", err, out)
		}
		return r
	}

	// --limit 0 so the shown count equals the score-survivor count (the
	// suppressed+shown==total invariant only holds when the display limit
	// does not bite).
	all := runJSON("inspect", "--domain", domainDir, "--json", "--limit", "0", "--min-score", "0")
	def := runJSON("inspect", "--domain", domainDir, "--json", "--limit", "0") // default --min-score 5

	// (b) --min-score 0 returns everything: nothing suppressed.
	if all.SuppressedCount != 0 {
		t.Errorf("min-score 0 suppressed %d candidates, want 0", all.SuppressedCount)
	}
	if all.TotalCandidates == 0 {
		t.Skip("real domain produced no candidates; cannot assert filtering behavior")
	}

	// (a) default --min-score filters a meaningful share.
	if def.SuppressedCount <= 0 {
		t.Errorf("default --min-score suppressed nothing; expected some low-signal noise filtered (total=%d)", def.TotalCandidates)
	}
	if len(def.Candidates) >= len(all.Candidates) {
		t.Errorf("default should show fewer candidates than --min-score 0: got %d vs %d",
			len(def.Candidates), len(all.Candidates))
	}

	// (c) invariant: suppressed + shown == pre-filter total (limit is 0).
	if def.SuppressedCount+len(def.Candidates) != def.TotalCandidates {
		t.Errorf("default invariant broken: suppressed(%d)+shown(%d) != total(%d)",
			def.SuppressedCount, len(def.Candidates), def.TotalCandidates)
	}
	if all.SuppressedCount+len(all.Candidates) != all.TotalCandidates {
		t.Errorf("min-score 0 invariant broken: suppressed(%d)+shown(%d) != total(%d)",
			all.SuppressedCount, len(all.Candidates), all.TotalCandidates)
	}
}

// TestCmdInspect_E2E_TextReportNamesSuppressedCount checks the human-readable
// default output (no --json) carries the honest suppressed summary line the
// task requires for a no-flags invocation.
func TestCmdInspect_E2E_TextReportNamesSuppressedCount(t *testing.T) {
	if testing.Short() {
		t.Skip("inspect e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	cmd := exec.Command(binPath, "inspect", "--domain", domainDir) // default flags
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hotam inspect failed: %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "suppressed") {
		t.Errorf("default text report must name the suppressed count, got:\n%s", text)
	}
}

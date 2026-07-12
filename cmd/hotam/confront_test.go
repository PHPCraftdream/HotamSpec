package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestFormatConfrontReport_ClearBranch verifies the "clear to propose" verdict
// is explicit (never silent) when the result has no hits on either side.
func TestFormatConfrontReport_ClearBranch(t *testing.T) {
	t.Parallel()
	r := diagnose.ConfrontResult{Candidate: "novel idea", Clear: true}
	text := formatConfrontReport(r)
	for _, want := range []string{
		"CONFRONT check",
		"clear to propose",
		"candidate:",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("clear-branch report missing %q:\n%s", want, text)
		}
	}
}

// TestFormatConfrontReport_HitBranch verifies the duplicate + re-litigation
// lines render the id, score, shared tokens, claim, and replaced_by pointer.
func TestFormatConfrontReport_HitBranch(t *testing.T) {
	t.Parallel()
	r := diagnose.ConfrontResult{
		Candidate: "cache stores all fields always",
		Settled: []diagnose.ConfrontHit{
			{ID: "R-cache-no-pii", Claim: "cache must never store PII", Score: 4, Shared: []string{"cache"}},
		},
		Rejected: []diagnose.ConfrontHit{
			{ID: "R-dead-store", Claim: "store nodes in per-node json", Score: 2, Shared: []string{"store"}, ReplacedBy: []string{"R-per-node-store"}},
		},
	}
	text := formatConfrontReport(r)
	for _, want := range []string{
		"possible DUPLICATE of 1",
		"R-cache-no-pii (score 4)",
		"shared tokens: [cache]",
		"possible RE-LITIGATION of 1",
		"R-dead-store (score 2)",
		"replaced by: R-per-node-store",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("hit-branch report missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "clear to propose") {
		t.Errorf("non-clear result must not print the clear-to-propose line:\n%s", text)
	}
}

// TestReadConfrontCandidate covers the two input modes (positional join,
// --file path) and the two usage errors (neither source, both sources).
func TestReadConfrontCandidate(t *testing.T) {
	t.Parallel()

	t.Run("positional joined with spaces", func(t *testing.T) {
		t.Parallel()
		got, err := readConfrontCandidate("", []string{"billing", "retries", "failed"})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if got != "billing retries failed" {
			t.Errorf("got %q, want joined positional", got)
		}
	})
	t.Run("empty is a usage error", func(t *testing.T) {
		t.Parallel()
		if _, err := readConfrontCandidate("", nil); err == nil {
			t.Error("expected error when neither --file nor positional given")
		}
	})
	t.Run("both sources is a usage error", func(t *testing.T) {
		t.Parallel()
		if _, err := readConfrontCandidate("some.txt", []string{"text"}); err == nil {
			t.Error("expected error when both --file and positional given")
		}
	})
}

// TestCmdConfront_E2E_UniqueTextIsClearOnRealDomain is the negative e2e proof:
// a deliberately-unique candidate against the real hotam-spec-self domain must
// report "clear to propose" (no overlap with SETTLED or REJECTED).
func TestCmdConfront_E2E_UniqueTextIsClearOnRealDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("confront e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	out := runConfrontText(t, binPath, domainDir, "zzz qxp wumbo nonexistent totally novel speculative banana franchise 12345")
	if !strings.Contains(out, "clear to propose") {
		t.Errorf("unique candidate must be clear to propose; got:\n%s", out)
	}
	if strings.Contains(out, "DUPLICATE") || strings.Contains(out, "RE-LITIGATION") {
		t.Errorf("unique candidate must not surface hits; got:\n%s", out)
	}
}

// TestCmdConfront_E2E_VerbatimSettledClaimIsDuplicate is the positive SETTLED
// e2e proof: feeding a verbatim (or near-verbatim) SETTLED claim back in must
// surface that requirement's id. The claim is read from the live graph so the
// test does not rot when a claim is reworded.
func TestCmdConfront_E2E_VerbatimSettledClaimIsDuplicate(t *testing.T) {
	if testing.Short() {
		t.Skip("confront e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	claim, id := pickRealSettledClaim(t, filepath.Join(domainDir, "graph.json"))
	out := runConfrontText(t, binPath, domainDir, claim)
	if strings.Contains(out, "clear to propose") {
		t.Fatalf("verbatim settled claim must be flagged as duplicate, got clear:\n%s", out)
	}
	if !strings.Contains(out, "DUPLICATE") {
		t.Errorf("expected DUPLICATE section, got:\n%s", out)
	}
	if !strings.Contains(out, id) {
		t.Errorf("expected report to name %q (verbatim settled claim fed back in), got:\n%s", id, out)
	}
}

// TestCmdConfront_E2E_JSONShape verifies the machine-readable contract.
func TestCmdConfront_E2E_JSONShape(t *testing.T) {
	if testing.Short() {
		t.Skip("confront e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	claim, id := pickRealSettledClaim(t, filepath.Join(domainDir, "graph.json"))
	cmd := exec.Command(binPath, "confront", "--domain", domainDir, "--json", claim)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hotam confront --json failed: %v\n%s", err, out)
	}
	var res diagnose.ConfrontResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("parse confront JSON: %v\nraw:\n%s", err, out)
	}
	if res.Clear {
		t.Errorf("Clear=true, want false for verbatim settled claim")
	}
	found := false
	for _, h := range res.Settled {
		if h.ID == id {
			found = true
		}
	}
	if !found {
		t.Errorf("JSON settled hits %v do not include %q", res.Settled, id)
	}
	if res.Candidate != claim {
		t.Errorf("Candidate echo mismatch: got %q", res.Candidate)
	}
}

// TestCmdConfront_E2E_FileMode verifies the --file <path> input path: writing
// the candidate text to a file and pointing --file at it must produce the same
// duplicate verdict as passing the text positionally.
func TestCmdConfront_E2E_FileMode(t *testing.T) {
	if testing.Short() {
		t.Skip("confront e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	claim, id := pickRealSettledClaim(t, filepath.Join(domainDir, "graph.json"))
	draftFile := filepath.Join(t.TempDir(), "candidate.txt")
	if err := os.WriteFile(draftFile, []byte(claim), 0o644); err != nil {
		t.Fatalf("write draft file: %v", err)
	}

	cmd := exec.Command(binPath, "confront", "--domain", domainDir, "--file", draftFile)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hotam confront --file failed: %v\n%s", err, out)
	}
	text := string(out)
	if strings.Contains(text, "clear to propose") {
		t.Errorf("--file mode with a verbatim settled claim must be flagged, got clear:\n%s", text)
	}
	if !strings.Contains(text, id) {
		t.Errorf("--file mode report must name %q, got:\n%s", id, text)
	}
}

// pickRealSettledClaim loads the domain graph at graphPath and returns the
// claim + id of a SETTLED requirement that the engine itself flags as a
// duplicate when fed its own claim back verbatim. Using the engine (rather
// than a hand-rolled token count) makes the picker's notion of "will match"
// identical to the assertion's, so the e2e never picks a claim the command
// would then fail to surface.
func pickRealSettledClaim(t *testing.T, graphPath string) (string, string) {
	t.Helper()
	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", graphPath, err)
	}
	for _, r := range g.Requirements {
		if r.Status != ontology.StatusSETTLED {
			continue
		}
		res := diagnose.Confront(g, r.Claim)
		for _, h := range res.Settled {
			if h.ID == r.ID {
				return r.Claim, r.ID
			}
		}
	}
	t.Skip("real domain has no SETTLED requirement that self-matches under the engine; cannot exercise the duplicate branch")
	return "", ""
}

func runConfrontText(t *testing.T, binPath, domainDir, text string) string {
	t.Helper()
	cmd := exec.Command(binPath, "confront", "--domain", domainDir, text)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hotam confront failed: %v\nOUTPUT:\n%s", err, out)
	}
	return string(out)
}

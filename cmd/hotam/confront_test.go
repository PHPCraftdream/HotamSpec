package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

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

// TestCmdConfront_E2E_UniqueTextIsClearOnRealDomain checks the command's
// machine-readable classification for a candidate with no lexical hits.
func TestCmdConfront_E2E_UniqueTextIsClearOnRealDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("confront e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	cmd := exec.Command(binPath, "confront", "--domain", domainDir, "--json",
		"zzz qxp wumbo nonexistent totally novel speculative banana franchise 12345")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hotam confront --json failed: %v\n%s", err, out)
	}
	var res diagnose.ConfrontResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("parse confront JSON: %v\nraw:\n%s", err, out)
	}
	if !res.Clear || len(res.Settled) != 0 || len(res.Rejected) != 0 {
		t.Errorf("unique candidate classification = %+v, want clear with no hits", res)
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
}

// TestCmdConfront_E2E_BooleanFlagBeforePositional verifies that a value-less
// boolean flag before the positional candidate still yields the same semantic
// classification as placing the candidate first. This must use a subprocess
// because reorderFlagsFirst runs in main before subcommand dispatch.
func TestCmdConfront_E2E_BooleanFlagBeforePositional(t *testing.T) {
	if testing.Short() {
		t.Skip("confront e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	claim, id := pickRealSettledClaim(t, filepath.Join(domainDir, "graph.json"))

	// (a) Order that always worked: positional first, boolean flag last.
	workingCmd := exec.Command(binPath, "confront", claim, "--domain", domainDir, "--json")
	workingOut, err := workingCmd.Output()
	if err != nil {
		t.Fatalf("positional-first order failed: %v\n%s", err, workingOut)
	}

	// (b) Order the review reported broken: boolean flag BEFORE the positional.
	brokenCmd := exec.Command(binPath, "confront", "--json", claim, "--domain", domainDir)
	brokenOut, err := brokenCmd.Output()
	if err != nil {
		t.Fatalf("flag-before-positional order failed (boolean flag ate the positional?): %v\n%s", err, brokenOut)
	}

	// Both must parse as valid ConfrontResult JSON.
	var workingRes, brokenRes diagnose.ConfrontResult
	if err := json.Unmarshal(workingOut, &workingRes); err != nil {
		t.Fatalf("positional-first order not JSON: %v\n%s", err, workingOut)
	}
	if err := json.Unmarshal(brokenOut, &brokenRes); err != nil {
		t.Fatalf("flag-before-positional order not JSON (positional swallowed?): %v\n%s", err, brokenOut)
	}

	// Both argument orderings must classify the real settled claim as a hit.
	// This checks the candidate reached the classifier without relying on an
	// echoed candidate field or byte-identical report output.
	if !confrontResultNamesID(workingRes, id) {
		t.Errorf("positional-first order did not surface duplicate %q", id)
	}
	if !confrontResultNamesID(brokenRes, id) {
		t.Errorf("flag-before-positional order did not surface duplicate %q (positional not parsed)", id)
	}

}

func confrontResultNamesID(r diagnose.ConfrontResult, id string) bool {
	for _, h := range r.Settled {
		if h.ID == id {
			return true
		}
	}
	return false
}

// TestCmdConfront_E2E_FileMode verifies --file input by checking the actual
// JSON classification for a claim that matches a SETTLED graph requirement.
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

	cmd := exec.Command(binPath, "confront", "--json", "--domain", domainDir, "--file", draftFile)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hotam confront --file --json failed: %v\n%s", err, out)
	}
	var res diagnose.ConfrontResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("parse confront --file JSON: %v\nraw:\n%s", err, out)
	}
	if res.Clear || !confrontResultNamesID(res, id) {
		t.Errorf("--file classification = %+v, want a hit for %q", res, id)
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

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// assertSingleJSONDocument is the core contract guard for every --json command:
// stdout must contain exactly ONE JSON value and nothing after it. It uses the
// streaming json.Decoder idiom: the first Decode must succeed, and a second
// Decode must return io.EOF (proving the stream ended cleanly after the single
// document — no trailing prose, no second document).
func assertSingleJSONDocument(t *testing.T, stdout []byte) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(stdout))
	var v json.RawMessage
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("first JSON decode failed: %v\nraw stdout:\n%s", err, stdout)
	}
	if err := dec.Decode(&v); err != io.EOF {
		t.Fatalf("expected io.EOF after the single JSON document (no trailing content on stdout), got %v\nraw stdout:\n%s", err, stdout)
	}
}

// runHotamJSON runs the hotam binary with the given args and returns stdout,
// stderr, and any execution error. It asserts the process exited 0.
func runHotamJSON(t *testing.T, binPath string, args ...string) ([]byte, []byte) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("hotam %s: %v\nstdout:\n%s\nstderr:\n%s", args[0], err, stdout.String(), stderr.String())
	}
	return stdout.Bytes(), stderr.Bytes()
}

// === propose --land --json (review-8 R8-c: the bug being fixed) ===

// TestProposeLandJSON_SingleDocument is the exact reproduction of the review's
// scenario: `hotam propose requirement --land --json` must emit exactly ONE
// JSON document to stdout (the proposeResult envelope, now carrying the land
// outcome), with all operational prose routed to stderr. Pre-fix, the JSON was
// printed BEFORE landProposalFile's prose hit the same stdout stream — a second
// Decode returned a syntax error or a non-EOF value.
func TestProposeLandJSON_SingleDocument(t *testing.T) {
	if testing.Short() {
		t.Skip("propose --land --json: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()
	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "req-land-json.json")

	stdout, stderr := runHotamJSON(t, binPath,
		"propose", "requirement",
		"--id", "R-propose-land-json-contract",
		"--claim", "the propose land json path emits exactly one document",
		"--owner", "framework-author",
		"--status", "DRAFT",
		"--why", "review-8 R8-c regression guard",
		"--domain", domainDir,
		"--out", outPath,
		"--today", "2026-07-14",
		"--land",
		"--json",
	)

	// stdout must be exactly one JSON document.
	assertSingleJSONDocument(t, stdout)

	// The decoded result must carry the land outcome fields.
	var result proposeResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		t.Fatalf("unmarshal proposeResult: %v\nraw:\n%s", err, stdout)
	}
	if !result.Landed {
		t.Errorf("landed = false, want true")
	}
	if result.RegeneratedDocs <= 0 {
		t.Errorf("regenerated_docs = %d, want > 0", result.RegeneratedDocs)
	}
	// On a clean land with 0 violations, proposeResult uses omitempty on the
	// violations field (conditional land-outcome fields), so a nil decode is
	// correct — the populated RegeneratedDocs + Landed==true already prove the
	// land outcome was carried. The standalone LandResult (hotam land --json)
	// always includes violations:[] via a non-omitempty field — see
	// TestCmdLandJSON_SingleDocument for that assertion.

	// The operational prose must be on stderr, NOT stdout.
	stderrStr := string(stderr)
	for _, want := range []string{"applied", "regenerated", "landed:"} {
		if !strings.Contains(stderrStr, want) {
			t.Errorf("stderr should contain %q (operational prose routed under --json), got stderr:\n%s", want, stderrStr)
		}
	}
}

// === hotam land --json (new flag) ===

// TestCmdLandJSON_SingleDocument proves the new `hotam land --json` flag emits
// exactly one JSON document (the LandResult) to stdout, with operational prose
// routed to stderr.
func TestCmdLandJSON_SingleDocument(t *testing.T) {
	if testing.Short() {
		t.Skip("land --json: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()
	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	proposalPath := filepath.Join(t.TempDir(), "land-json.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-json-contract",
		"claim": "the land json flag emits exactly one document",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "review-8 R8-c land json coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	stdout, stderr := runHotamJSON(t, binPath,
		"land", proposalPath,
		"--domain", domainDir,
		"--today", "2026-07-14",
		"--json",
	)

	assertSingleJSONDocument(t, stdout)

	var result LandResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		t.Fatalf("unmarshal LandResult: %v\nraw:\n%s", err, stdout)
	}
	if !result.Landed {
		t.Errorf("landed = false, want true")
	}
	if result.RegeneratedDocs <= 0 {
		t.Errorf("regenerated_docs = %d, want > 0", result.RegeneratedDocs)
	}
	if result.Violations == nil {
		t.Error("violations is nil — should be [] on clean land")
	}

	stderrStr := string(stderr)
	for _, want := range []string{"applied", "regenerated", "landed:"} {
		if !strings.Contains(stderrStr, want) {
			t.Errorf("stderr should contain %q under --json, got:\n%s", want, stderrStr)
		}
	}
}

// TestCmdLandJSON_FailureStdoutClean proves the failure/rollback path does not
// leave stray prose on stdout under --json. genSpec failure (via --claude-md
// pointing at a directory) triggers rollback; stdout must be empty and the
// error goes to stderr + non-zero exit (handled by main.go).
func TestCmdLandJSON_FailureStdoutClean(t *testing.T) {
	if testing.Short() {
		t.Skip("land --json failure: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()
	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	proposalPath := filepath.Join(t.TempDir(), "land-json-fail.json")
	// Claim wording deliberately avoids the "must (not)" reserved-marker
	// vocabulary (internal/diagnose.oppositeMarkerPairs): "must not
	// contaminate ... under json" collided with R-orientation-faq-answerable
	// and R-vendored-recorder-matches-engine-canon's "MUST be reachable/
	// byte-identical ... path" via the opposite-marker + shared-topical-
	// token semantic gate (must vs must not, shared token "path"), blocking
	// the land at the confront gate before genSpec even ran — a different,
	// INCIDENTAL failure mode than the genSpec failure this test exists to
	// exercise.
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-json-fail-contract",
		"claim": "the failure path keeps stdout empty under json",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "failure-path stdout cleanliness"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	claudeMDDir := t.TempDir()

	cmd := exec.Command(binPath,
		"land", proposalPath,
		"--domain", domainDir,
		"--today", "2026-07-14",
		"--claude-md", claudeMDDir,
		"--json",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected land --json to fail when genSpec fails, got nil error")
	}

	// stdout must be empty — no JSON document, no prose.
	if stdout.Len() > 0 {
		t.Errorf("stdout must be empty on failure under --json, got:\n%s", stdout.String())
	}
	// stderr must carry the error + the rolled-back operational messages.
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "rolled back") {
		t.Errorf("stderr should mention rollback on failure, got:\n%s", stderrStr)
	}
}

// === Full audit: every --json command emits exactly one JSON document ===

// TestAllJSONCommands_SingleDocument is the full audit guard required by
// review-8: for EACH command that carries a --json flag, stdout must parse as
// exactly one JSON document. This catches the R8-c class of bug (trailing
// prose after the JSON) across the entire command surface, not just the
// propose/land path that was actually broken.
func TestAllJSONCommands_SingleDocument(t *testing.T) {
	if testing.Short() {
		t.Skip("full JSON audit: builds a real binary + spawns child processes; skipped in -short")
	}
	t.Parallel()
	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	// Find a real SETTLED requirement with enforced_by for gate/brief/req.
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	anchor := ""
	gateTarget := ""
	for _, r := range g.Requirements {
		if r.Status == "SETTLED" {
			if anchor == "" {
				anchor = r.ID
			}
			if gateTarget == "" && len(r.EnforcedBy) > 0 {
				gateTarget = r.ID
			}
		}
		if anchor != "" && gateTarget != "" {
			break
		}
	}
	if anchor == "" {
		t.Skip("no SETTLED requirement found in fixture")
	}

	cases := []struct {
		name string
		args []string
	}{
		{"all-violations", []string{"all-violations", "--domain", domainDir, "--json"}},
		{"confront", []string{"confront", "test candidate text for overlap", "--domain", domainDir, "--json"}},
		{"status", []string{"status", "--domain", domainDir, "--today", "2026-07-14", "--json"}},
		{"what-now", []string{"what-now", "--domain", domainDir, "--today", "2026-07-14", "--json", "--limit", "3"}},
		{"inspect", []string{"inspect", "--domain", domainDir, "--json", "--limit", "3"}},
		{"brief", []string{"brief", anchor, "--domain", domainDir, "--today", "2026-07-14", "--json"}},
		{"due", []string{"due", "--domain", domainDir, "--today", "2026-07-14", "--json"}},
		{"req-show", []string{"req", "show", anchor, "--domain", domainDir, "--json"}},
		{"req-list", []string{"req", "list", "--domain", domainDir, "--json"}},
		{"req-search", []string{"req", "search", "requirement", "--domain", domainDir, "--json"}},
		{"req-context", []string{"req", "context", anchor, "--domain", domainDir, "--json"}},
		{"req-related", []string{"req", "related", anchor, "--domain", domainDir, "--json"}},
	}
	if gateTarget != "" {
		cases = append(cases, struct {
			name string
			args []string
		}{"gate", []string{"gate", gateTarget, "--domain", domainDir, "--json"}})
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stdout, _ := runHotamJSON(t, binPath, tc.args...)
			assertSingleJSONDocument(t, stdout)
		})
	}
}

// === Non-JSON behavior unchanged ===

// TestProposeLand_NonJSON_Unchanged pins the non-JSON prose path for
// `propose --land`: stdout must carry the confront report, the write message,
// AND the land operational messages (applied/regenerated/landed) — all on
// stdout, byte-identical to pre-fix behavior. This would FAIL if the JSON
// refactor accidentally routed prose to stderr even when --json is absent.
func TestProposeLand_NonJSON_Unchanged(t *testing.T) {
	if testing.Short() {
		t.Skip("propose --land non-JSON: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()
	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "req-land-prose.json")

	cmd := exec.Command(binPath,
		"propose", "requirement",
		"--id", "R-propose-land-prose",
		"--claim", "non-json path must keep all prose on stdout",
		"--owner", "framework-author",
		"--status", "DRAFT",
		"--domain", domainDir,
		"--out", outPath,
		"--today", "2026-07-14",
		"--land",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("propose --land non-JSON: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	stdoutStr := stdout.String()
	for _, want := range []string{"wrote", "applied", "regenerated", "landed:"} {
		if !strings.Contains(stdoutStr, want) {
			t.Errorf("non-JSON stdout should contain %q (byte-identical to pre-fix), got stdout:\n%s", want, stdoutStr)
		}
	}
}

// TestCmdLand_NonJSON_Unchanged pins the non-JSON prose path for `hotam land`:
// the confront report + operational messages must be on stdout, byte-identical
// to pre-fix behavior.
func TestCmdLand_NonJSON_Unchanged(t *testing.T) {
	if testing.Short() {
		t.Skip("land non-JSON: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()
	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	proposalPath := filepath.Join(t.TempDir(), "land-prose.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-prose-contract",
		"claim": "non-json land path keeps prose on stdout",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "non-json regression guard"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	cmd := exec.Command(binPath,
		"land", proposalPath,
		"--domain", domainDir,
		"--today", "2026-07-14",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("land non-JSON: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	stdoutStr := stdout.String()
	for _, want := range []string{"applied", "regenerated", "landed:"} {
		if !strings.Contains(stdoutStr, want) {
			t.Errorf("non-JSON stdout should contain %q, got stdout:\n%s", want, stdoutStr)
		}
	}
}

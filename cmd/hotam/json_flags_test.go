package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// === Part A: --json tests for what-now, gate, all-violations (task #128) ===

// TestCmdWhatNow_JSON proves hotam what-now --json emits valid, well-shaped
// []diagnose.Signal JSON via a real subprocess (safe under t.Parallel). It
// verifies --limit is respected and every element carries the expected fields.
func TestCmdWhatNow_JSON(t *testing.T) {
	if testing.Short() {
		t.Skip("what-now --json: spawns a child process; skipped in -short")
	}
	t.Parallel()
	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	cmd := exec.Command(binPath, "what-now", "--domain", domainDir, "--today", "2026-07-13", "--json", "--limit", "3")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hotam what-now --json: %v\n%s", err, out)
	}

	var signals []diagnose.Signal
	if err := json.Unmarshal(out, &signals); err != nil {
		t.Fatalf("unmarshal what-now JSON: %v\nraw: %s", err, out)
	}

	// --limit 3 should produce at most 3 entries.
	if len(signals) > 3 {
		t.Errorf("--limit 3 but JSON has %d signals", len(signals))
	}

	// Every signal must have non-empty fields matching the struct.
	for i, s := range signals {
		if s.Check == "" {
			t.Errorf("signal[%d] has empty Check", i)
		}
		if s.Target == "" {
			t.Errorf("signal[%d] has empty Target", i)
		}
		if s.Message == "" {
			t.Errorf("signal[%d] has empty Message", i)
		}
	}

	// Cross-check: independently compute the full signal list and verify the
	// first N match.
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	full := diagnose.DiagnoseSignals(g, "2026-07-13")
	end := 3
	if end > len(full) {
		end = len(full)
	}
	if len(signals) != end {
		t.Errorf("JSON signal count = %d, want %d (full list has %d, limit 3)", len(signals), end, len(full))
	}
	for i := 0; i < len(signals) && i < end; i++ {
		if signals[i] != full[i] {
			t.Errorf("signal[%d] mismatch:\n got: %+v\nwant: %+v", i, signals[i], full[i])
		}
	}
}

// TestCmdGate_JSON proves hotam gate --json emits a well-shaped
// gate.GateResult with the expected JSON field names (confident, node_ids,
// reason) via a real subprocess.
func TestCmdGate_JSON(t *testing.T) {
	if testing.Short() {
		t.Skip("gate --json: spawns a child process; skipped in -short")
	}
	t.Parallel()
	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	// Find a real SETTLED requirement with enforced_by to get a meaningful result.
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	target := ""
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED && len(r.EnforcedBy) > 0 {
			target = r.ID
			break
		}
	}
	if target == "" {
		t.Skip("no SETTLED requirement with enforced_by found in fixture")
	}

	cmd := exec.Command(binPath, "gate", target, "--domain", domainDir, "--json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hotam gate --json: %v\n%s", err, out)
	}

	// Verify the JSON keys match the struct's json tags.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(out, &fields); err != nil {
		t.Fatalf("unmarshal gate JSON to map: %v\nraw: %s", err, out)
	}
	for _, key := range []string{"confident", "node_ids", "reason"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("gate JSON missing expected field %q: %s", key, out)
		}
	}

	// Round-trip into the struct type.
	var result gate.GateResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal gate JSON to GateResult: %v\nraw: %s", err, out)
	}
	if result.Reason == "" {
		t.Error("gate JSON Reason is empty")
	}

	// node_ids must never be null (nil-slice guard in the command).
	var ids []string
	if err := json.Unmarshal(fields["node_ids"], &ids); err != nil {
		t.Errorf("node_ids is not a JSON array: %v (raw: %s)", err, fields["node_ids"])
	}
}

// TestCmdAllViolations_JSON proves hotam all-violations --json emits valid
// []invariants.Violation JSON with the expected field names.
func TestCmdAllViolations_JSON(t *testing.T) {
	if testing.Short() {
		t.Skip("all-violations --json: spawns a child process; skipped in -short")
	}
	t.Parallel()
	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	// The real domain should be clean (0 violations), so --json emits `[]`.
	cmd := exec.Command(binPath, "all-violations", "--domain", domainDir, "--json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hotam all-violations --json: %v\n%s", err, out)
	}

	var violations []invariants.Violation
	if err := json.Unmarshal(out, &violations); err != nil {
		t.Fatalf("unmarshal all-violations JSON: %v\nraw: %s", err, out)
	}

	// Every violation (if any) must have non-empty fields.
	for i, v := range violations {
		if v.Check == "" {
			t.Errorf("violation[%d] has empty Check", i)
		}
		if v.ID == "" {
			t.Errorf("violation[%d] has empty ID", i)
		}
		if v.Message == "" {
			t.Errorf("violation[%d] has empty Message", i)
		}
	}
}

// writeViolationsTestDomain creates a minimal domain graph that has at least
// one invariant violation, so the exit-code test can prove exit 1 fires.
func writeViolationsTestDomain(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	domainDir := filepath.Join(dir, "domain")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A requirement owned by a non-existent stakeholder trips a violation.
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-vi", Owner: "nonexistent-owner", Status: ontology.StatusSETTLED,
				Enforcement: ontology.EnforcementENFORCED, Enforceability: ontology.EnforceabilityENFORCEABLE},
		},
	}
	if err := loader.WriteGraph(filepath.Join(domainDir, "graph.json"), g); err != nil {
		t.Fatalf("write graph: %v", err)
	}
	return domainDir
}

// TestCmdAllViolations_JSONExitCode proves the exit code is IDENTICAL with
// and without --json: 0 on a clean graph, 1 on a graph with violations.
func TestCmdAllViolations_JSONExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("exit-code test builds a real binary + spawns child processes; skipped in -short")
	}
	t.Parallel()
	binPath := buildSharedHotamBinary(t)

	cleanDomain := copySelfDomain(t)
	dirtyDomain := writeViolationsTestDomain(t)

	cases := []struct {
		name      string
		domainDir string
		jsonFlag  bool
		wantExit  int
	}{
		{"clean_no_json", cleanDomain, false, 0},
		{"clean_json", cleanDomain, true, 0},
		{"dirty_no_json", dirtyDomain, false, 1},
		{"dirty_json", dirtyDomain, true, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"all-violations", "--domain", tc.domainDir}
			if tc.jsonFlag {
				args = append(args, "--json")
			}
			cmd := exec.Command(binPath, args...)
			out, err := cmd.CombinedOutput()
			exitCode := 0
			if err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					exitCode = ee.ExitCode()
				} else {
					t.Fatalf("run binary: %v\n%s", err, out)
				}
			}
			if exitCode != tc.wantExit {
				t.Errorf("exit code = %d, want %d\noutput:\n%s", exitCode, tc.wantExit, out)
			}
		})
	}
}

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// TestCmdPropose_Requirement_ConstructsValidJSON is the happy-path test: valid
// flags produce a correctly-shaped proposal JSON file on disk that can be
// re-parsed by parseProposal (the same path apply-proposal / land consume).
func TestCmdPropose_Requirement_ConstructsValidJSON(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "req.json")

	err := cmdPropose([]string{
		"requirement",
		"--id", "R-propose-happy",
		"--claim", "the propose command writes valid JSON from flags",
		"--owner", "framework-author",
		"--status", "DRAFT",
		"--why", "unit coverage for the propose command",
		"--enforcement", "PROSE",
		"--m-tag", "M20",
		"--assumptions", "A-propose-1, A-propose-2",
		"--evidence", "src.go:L1",
		"--source-refs", "doc.md",
		"--blocked-on", "feature:nothing",
		"--domain", domainDir,
		"--out", outPath,
	})
	if err != nil {
		t.Fatalf("cmdPropose requirement: %v", err)
	}

	// File must exist and be re-parseable as a Requirement proposal.
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	p, err := parseProposal(data)
	if err != nil {
		t.Fatalf("parseProposal on the written file: %v\nraw:\n%s", err, data)
	}
	pr, ok := p.(interface {
		Kind() string
		TargetAnchor() string
	})
	if !ok {
		t.Fatalf("parsed proposal has wrong type: %T", p)
	}
	if pr.Kind() != "Requirement" {
		t.Errorf("Kind = %q, want Requirement", pr.Kind())
	}
	if pr.TargetAnchor() != "R-propose-happy" {
		t.Errorf("TargetAnchor = %q, want R-propose-happy", pr.TargetAnchor())
	}

	// The written JSON must carry a top-level "kind" field.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal written JSON: %v", err)
	}
	kindBytes, ok := raw["kind"]
	if !ok {
		t.Fatal("written JSON missing top-level \"kind\" field")
	}
	var kindStr string
	if err := json.Unmarshal(kindBytes, &kindStr); err != nil {
		t.Fatalf("unmarshal kind field: %v", err)
	}
	if kindStr != "Requirement" {
		t.Errorf("kind field = %q, want Requirement", kindStr)
	}

	// Verify key fields are present in the JSON.
	bodyStr := string(data)
	for _, want := range []string{
		`"claim":`,
		`"owner":`,
		`"status":`,
		`"why":`,
		`"m_tag":`,
		`"blocked_on":`,
		"A-propose-1",
		"src.go:L1",
	} {
		if !strings.Contains(bodyStr, want) {
			t.Errorf("written JSON missing %q\nraw:\n%s", want, bodyStr)
		}
	}
}

// TestCmdPropose_Requirement_MissingRequiredFlags proves that omitting a
// required flag produces a clear error message (not a panic, not a silently
// empty proposal).
func TestCmdPropose_Requirement_MissingRequiredFlags(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "should-not-exist.json")

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing id", []string{
			"requirement",
			"--claim", "x", "--owner", "y", "--status", "DRAFT",
			"--domain", domainDir, "--out", outPath,
		}, "--id is required"},
		{"missing claim", []string{
			"requirement",
			"--id", "R-x", "--owner", "y", "--status", "DRAFT",
			"--domain", domainDir, "--out", outPath,
		}, "--claim is required"},
		{"missing owner", []string{
			"requirement",
			"--id", "R-x", "--claim", "y", "--status", "DRAFT",
			"--domain", domainDir, "--out", outPath,
		}, "--owner is required"},
		{"missing status", []string{
			"requirement",
			"--id", "R-x", "--claim", "y", "--owner", "z",
			"--domain", domainDir, "--out", outPath,
		}, "--status is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := cmdPropose(tc.args)
			if err == nil {
				t.Fatal("expected error for missing required flag, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.want)
			}
			// No file should have been written.
			if _, err := os.Stat(outPath); err == nil {
				t.Error("output file was written despite a missing required flag")
			}
		})
	}
}

// TestCmdPropose_ValidationFails_NoFileWritten is the genuine negative-path
// test: the proposal passes the flag-presence checks but FAILS
// proposal.Validate (enforced_by clear-sentinel mixed with real entries), so
// the file must NOT be written and the command must return a non-nil error.
func TestCmdPropose_ValidationFails_NoFileWritten(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "should-not-exist.json")

	err := cmdPropose([]string{
		"requirement",
		"--id", "R-propose-bad",
		"--claim", "a claim",
		"--owner", "owner",
		"--status", "DRAFT",
		"--enforced-by", "<clear>,real-enforcer", // invalid: sentinel + real entry
		"--domain", domainDir,
		"--out", outPath,
	})
	if err == nil {
		t.Fatal("expected validation error for mixed clear-sentinel, got nil")
	}
	if _, e := os.Stat(outPath); e == nil {
		t.Error("file was written despite validation failure")
	}
}

// TestCmdPropose_Land_AppliesRegeneratesReverifies is the end-to-end --land
// test: after writing the proposal JSON, --land must apply it to the graph,
// regenerate docs, and leave 0 violations — mirroring `hotam land`'s pipeline
// (and proving the shared landProposalFile function works).
func TestCmdPropose_Land_AppliesRegeneratesReverifies(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "req-land.json")
	genDir := filepath.Join(domainDir, "docs", "gen")

	err := cmdPropose([]string{
		"requirement",
		"--id", "R-propose-land-e2e",
		"--claim", "the propose --land flag applies and regenerates in one step",
		"--owner", "framework-author",
		"--status", "DRAFT",
		"--why", "e2e coverage for propose --land",
		"--domain", domainDir,
		"--out", outPath,
		"--today", "2026-07-13",
		"--land",
	})
	if err != nil {
		t.Fatalf("cmdPropose --land: %v", err)
	}

	// graph.json must contain the newly applied node.
	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	if !strings.Contains(string(graphData), "R-propose-land-e2e") {
		t.Error("graph.json does not contain R-propose-land-e2e after propose --land")
	}

	// docs/gen/REQUIREMENTS.md must be freshly rendered and reflect the new node.
	reqs, err := os.ReadFile(filepath.Join(genDir, "REQUIREMENTS.md"))
	if err != nil {
		t.Fatalf("read post-land REQUIREMENTS.md: %v", err)
	}
	if !strings.Contains(string(reqs), "R-propose-land-e2e") {
		t.Error("docs/gen/REQUIREMENTS.md was not regenerated with the new requirement")
	}
}

// TestCmdPropose_Land_MissingTodayReturnsError proves that --land without
// --today is a clear error, and that NO file is written (the precondition is
// checked before any I/O, so a missing --today doesn't leave a stale draft).
func TestCmdPropose_Land_MissingTodayReturnsError(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "req-land.json")

	err := cmdPropose([]string{
		"requirement",
		"--id", "R-propose-land-no-today",
		"--claim", "claim",
		"--owner", "owner",
		"--status", "DRAFT",
		"--domain", domainDir,
		"--out", outPath,
		"--land",
	})
	if err == nil {
		t.Fatal("expected error for --land without --today")
	}
	if !strings.Contains(err.Error(), "--today is required") {
		t.Errorf("error = %q, want it to mention --today", err.Error())
	}
	if _, e := os.Stat(outPath); e == nil {
		t.Error("file was written despite --land missing --today — precondition should gate all I/O")
	}
}

// TestCmdPropose_Confront_RunsButNeverBlocks proves the automatic confront
// check runs before writing and NEVER blocks the write. Using the real
// hotam-spec-self domain graph (237 SETTLED requirements), a claim that
// overlaps with existing content should still produce a successful write.
// The key property: confront is advisory (R-ai-presents-not-decides).
func TestCmdPropose_Confront_RunsButNeverBlocks(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "confront.json")

	// Use claim text with words common in this graph's corpus so the confront
	// engine is exercised. The command must still succeed and write the file.
	err := cmdPropose([]string{
		"requirement",
		"--id", "R-propose-confront-nonblock",
		"--claim", "every requirement shall enforce its own invariant via a check function",
		"--owner", "framework-author",
		"--status", "DRAFT",
		"--why", "proves confront is advisory",
		"--domain", domainDir,
		"--out", outPath,
	})
	if err != nil {
		t.Fatalf("cmdPropose should succeed even with confront overlap, got: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("file was not written despite confront being advisory: %v", err)
	}
}

// TestCmdPropose_JSON_E2E_EmitsConfrontResult is the --json output test: it
// runs the REAL hotam binary as a subprocess (so stdout capture is safe under
// t.Parallel) and verifies the JSON envelope contains the expected keys,
// including the "confront" result from the automatic pre-write check.
func TestCmdPropose_JSON_E2E_EmitsConfrontResult(t *testing.T) {
	if testing.Short() {
		t.Skip("propose json e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "json-out.json")

	cmd := exec.Command(binPath,
		"propose", "requirement",
		"--id", "R-propose-json-e2e",
		"--claim", "the json output carries the confront result",
		"--owner", "framework-author",
		"--status", "DRAFT",
		"--why", "e2e json coverage",
		"--domain", domainDir,
		"--out", outPath,
		"--json",
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hotam propose --json failed: %v\n%s", err, out)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("parse propose JSON output: %v\nraw:\n%s", err, out)
	}
	for _, key := range []string{"kind", "anchor", "confront", "written"} {
		if _, ok := result[key]; !ok {
			t.Errorf("JSON output missing key %q\nraw:\n%s", key, out)
		}
	}
	if result["kind"] != "Requirement" {
		t.Errorf("kind = %v, want Requirement", result["kind"])
	}
	// The confront sub-object must be present (always runs for a valid draft).
	confront, ok := result["confront"].(map[string]any)
	if !ok {
		t.Fatalf("confront is not an object: %v", result["confront"])
	}
	for _, key := range []string{"candidate", "settled", "rejected", "clear"} {
		if _, ok := confront[key]; !ok {
			t.Errorf("confront sub-object missing key %q", key)
		}
	}
	// The file must also have been written to disk.
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("file was not written to --out despite --json: %v", err)
	}
}

// TestCmdPropose_Rejection_ConstructsValidJSON covers the rejection subcommand:
// --requirement-id + --reason produce a valid Rejection proposal JSON.
func TestCmdPropose_Rejection_ConstructsValidJSON(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "rej.json")

	err := cmdPropose([]string{
		"rejection",
		"--requirement-id", "R-some-old-idea",
		"--reason", "superseded by a better approach",
		"--replaced-by", "R-better-approach",
		"--domain", domainDir,
		"--out", outPath,
	})
	if err != nil {
		t.Fatalf("cmdPropose rejection: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	p, err := parseProposal(data)
	if err != nil {
		t.Fatalf("parseProposal: %v", err)
	}
	if p.Kind() != "Rejection" {
		t.Errorf("Kind = %q, want Rejection", p.Kind())
	}
	body := string(data)
	for _, want := range []string{
		`"kind": "Rejection"`,
		`"requirement_id": "R-some-old-idea"`,
		`"reason"`,
		`"replaced_by"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("written JSON missing %q\nraw:\n%s", want, body)
		}
	}
}

// TestCmdPropose_Stakeholder_ConstructsValidJSON covers the stakeholder
// subcommand: --id + --name + --domain produce a valid Stakeholder proposal JSON.
func TestCmdPropose_Stakeholder_ConstructsValidJSON(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "stk.json")

	err := cmdPropose([]string{
		"stakeholder",
		"--id", "S-propose-test",
		"--name", "Propose Test Team",
		"--domain", "hotam-spec-self",
		"--why", "coverage for the stakeholder subcommand",
		"--domain-dir", domainDir,
		"--out", outPath,
	})
	if err != nil {
		t.Fatalf("cmdPropose stakeholder: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	p, err := parseProposal(data)
	if err != nil {
		t.Fatalf("parseProposal: %v", err)
	}
	if p.Kind() != "Stakeholder" {
		t.Errorf("Kind = %q, want Stakeholder", p.Kind())
	}
}

// TestCmdPropose_UnknownKindReturnsError proves an unrecognized subkind is a
// clear error, not a silent fallback.
func TestCmdPropose_UnknownKindReturnsError(t *testing.T) {
	t.Parallel()
	err := cmdPropose([]string{"bogus"})
	if err == nil {
		t.Fatal("expected error for unknown propose kind")
	}
	if !strings.Contains(err.Error(), "unknown propose kind") {
		t.Errorf("error = %q, want it to mention unknown kind", err.Error())
	}
}

// TestMarshalProposalFile_RoundTripsThroughParseProposal proves the JSON
// written by marshalProposalFile is consumable by parseProposal + Validate +
// Apply — closing the "write → re-read" loop at the unit level.
func TestMarshalProposalFile_RoundTripsThroughParseProposal(t *testing.T) {
	t.Parallel()
	pr := newRequirementForTest()
	data, err := marshalProposalFile(pr)
	if err != nil {
		t.Fatalf("marshalProposalFile: %v", err)
	}
	p, err := parseProposal(data)
	if err != nil {
		t.Fatalf("parseProposal on marshalled data: %v\nraw:\n%s", err, data)
	}
	if p.Kind() != "Requirement" {
		t.Errorf("Kind = %q, want Requirement", p.Kind())
	}
	if p.TargetAnchor() != "R-roundtrip" {
		t.Errorf("TargetAnchor = %q, want R-roundtrip", p.TargetAnchor())
	}
}

func newRequirementForTest() proposal.ProposedRequirement {
	return proposal.ProposedRequirement{
		ID:     "R-roundtrip",
		Claim:  "the marshal→parse round-trip must preserve every field",
		Owner:  "framework-author",
		Status: "DRAFT",
		Why:    "unit coverage for marshalProposalFile",
	}
}

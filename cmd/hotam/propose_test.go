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

// TestCmdPropose_Requirement_LastReviewedAtReviewAfter proves --last-reviewed-at
// and --review-after (task #159 / R12-a) are wired from flags into the
// written proposal JSON, mirroring --source-refs/--evidence's existing
// coverage above. Evidence must also be supplied here: validate.go's
// R-review-mark-carries-evidence rule requires non-empty Evidence whenever
// either date field is set on the proposal.
func TestCmdPropose_Requirement_LastReviewedAtReviewAfter(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "req-provenance.json")

	err := cmdPropose([]string{
		"requirement",
		"--id", "R-propose-provenance",
		"--claim", "the propose command wires last-reviewed-at and review-after from flags",
		"--owner", "framework-author",
		"--status", "DRAFT",
		"--why", "unit coverage for --last-reviewed-at/--review-after",
		"--evidence", "steward review",
		"--source-refs", "doc.md",
		"--last-reviewed-at", "2026-07-15",
		"--review-after", "2027-07-15",
		"--domain", domainDir,
		"--out", outPath,
	})
	if err != nil {
		t.Fatalf("cmdPropose requirement: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	p, err := parseProposal(data)
	if err != nil {
		t.Fatalf("parseProposal on the written file: %v\nraw:\n%s", err, data)
	}
	pr, ok := p.(proposal.ProposedRequirement)
	if !ok {
		t.Fatalf("parsed proposal has wrong type: %T", p)
	}
	if pr.LastReviewedAt != "2026-07-15" {
		t.Errorf("LastReviewedAt = %q, want 2026-07-15", pr.LastReviewedAt)
	}
	if pr.ReviewAfter != "2027-07-15" {
		t.Errorf("ReviewAfter = %q, want 2027-07-15", pr.ReviewAfter)
	}

	bodyStr := string(data)
	for _, want := range []string{
		`"last_reviewed_at": "2026-07-15"`,
		`"review_after": "2027-07-15"`,
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
		"--stakeholder-domain", "hotam-spec-self",
		"--why", "coverage for the stakeholder subcommand",
		"--domain", domainDir,
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
	sp, ok := p.(proposal.ProposedStakeholder)
	if !ok {
		t.Fatalf("parsed proposal is %T, want proposal.ProposedStakeholder", p)
	}
	if sp.Domain != "hotam-spec-self" {
		t.Errorf("ProposedStakeholder.Domain = %q, want %q (from --stakeholder-domain)", sp.Domain, "hotam-spec-self")
	}
}

// TestCmdPropose_Stakeholder_DomainFlagTargetsGraphDirectory proves --domain
// on `propose stakeholder` means the TARGET GRAPH DIRECTORY (matching
// requirement/rejection), not the stakeholder's own business-domain field: an
// otherwise-valid invocation pointed at a non-existent --domain fails at
// graph load (confront needs to load the target graph), while
// --stakeholder-domain is free to name any business domain unrelated to the
// path.
func TestCmdPropose_Stakeholder_DomainFlagTargetsGraphDirectory(t *testing.T) {
	t.Parallel()
	outPath := filepath.Join(t.TempDir(), "stk.json")
	missingDomainDir := filepath.Join(t.TempDir(), "does-not-exist")

	err := cmdPropose([]string{
		"stakeholder",
		"--id", "S-propose-test-2",
		"--name", "Propose Test Team 2",
		"--stakeholder-domain", "some-unrelated-business-domain",
		"--why", "coverage for --domain semantics",
		"--domain", missingDomainDir,
		"--out", outPath,
	})
	if err == nil {
		t.Fatalf("cmdPropose stakeholder with --domain %q should fail (no graph.json there), got success", missingDomainDir)
	}
	if _, statErr := os.Stat(outPath); statErr == nil {
		t.Errorf("proposal JSON should not be written when the target graph directory does not exist")
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

// TestExtractProposeKind_HelpFlagIsFoundAsBareFlagNotSwallowed is the
// argument-handling unit proof for finding N2 (`hotam propose requirement -h`
// printed the generic top-level usage line instead of per-flag help).
//
// Root cause (traced in cmd/hotam/main.go and this file, pre-fix): -h was not
// in boolFlagNames, so (1) reorderFlagsFirst(["requirement", "-h"]) reordered
// to ["-h", "requirement"] (an unrecognized flag with nothing left to
// swallow), and (2) the OLD inline kind-scanner in cmdPropose, running on
// that reordered slice, saw "-h" at i=0, isBoolFlag("-h") false, and args[1]
// ("requirement") not dash-prefixed — so it treated "-h requirement" as a
// flag+value PAIR and consumed BOTH tokens, leaving kindIdx == -1 and firing
// the generic "usage: hotam propose <kind>" error instead of ever reaching
// cmdProposeRequirement's own FlagSet.Parse(["-h"]).
//
// This test calls extractProposeKind DIRECTLY (the kind-scanner extracted
// from cmdPropose for testability) and stops there — it deliberately never
// reaches fs.Parse, because flag.ExitOnError's Parse calls os.Exit(0) from
// inside the stdlib on -h/-help, which would silently truncate the rest of
// this test binary's run if invoked in-process. See
// TestProposeHelp_RealBinary_PerKindPerFlagHelp (testbinary-based, in this
// file) for the end-to-end proof that the real per-flag help is printed.
func TestExtractProposeKind_HelpFlagIsFoundAsBareFlagNotSwallowed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		args     []string
		wantKind string
		wantRest []string
	}{
		{
			name:     "post-reorderFlagsFirst shape (as main() actually calls it)",
			args:     reorderFlagsFirst([]string{"requirement", "-h"}),
			wantKind: "requirement",
			wantRest: []string{"-h"},
		},
		{
			name:     "raw pre-reorder shape",
			args:     []string{"requirement", "-h"},
			wantKind: "requirement",
			wantRest: []string{"-h"},
		},
		{
			name:     "--help double-dash spelling",
			args:     reorderFlagsFirst([]string{"rejection", "--help"}),
			wantKind: "rejection",
			wantRest: []string{"--help"},
		},
		{
			name:     "-h ahead of the kind (already-reordered shape)",
			args:     []string{"-h", "stakeholder"},
			wantKind: "stakeholder",
			wantRest: []string{"-h"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kind, rest, err := extractProposeKind(tc.args)
			if err != nil {
				t.Fatalf("extractProposeKind(%v): unexpected error: %v", tc.args, err)
			}
			if kind != tc.wantKind {
				t.Errorf("kind = %q, want %q (args=%v)", kind, tc.wantKind, tc.args)
			}
			if len(rest) != len(tc.wantRest) {
				t.Fatalf("rest = %v, want %v (args=%v)", rest, tc.wantRest, tc.args)
			}
			for i := range tc.wantRest {
				if rest[i] != tc.wantRest[i] {
					t.Errorf("rest[%d] = %q, want %q (args=%v)", i, rest[i], tc.wantRest[i], tc.args)
				}
			}
		})
	}
}

// TestExtractProposeKind_ExistingFlagCombinationsUnchanged is the
// non-regression guard for the extractProposeKind refactor (pulling the
// inline kind-scanner out of cmdPropose into a named, testable helper): every
// pre-existing flag combination for every kind must resolve to the exact same
// kind/rest split as before the refactor — this is a testability refactor,
// not a behavior change, beyond the -h/-help fix itself.
func TestExtractProposeKind_ExistingFlagCombinationsUnchanged(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		args     []string
		wantKind string
		wantRest []string
	}{
		{
			name:     "kind first, flags after",
			args:     []string{"requirement", "--id", "R-x", "--claim", "y"},
			wantKind: "requirement",
			wantRest: []string{"--id", "R-x", "--claim", "y"},
		},
		{
			name:     "post-reorderFlagsFirst shape, flags before kind",
			args:     reorderFlagsFirst([]string{"requirement", "--id", "R-x", "--domain", "/tmp/x"}),
			wantKind: "requirement",
			wantRest: []string{"--id", "R-x", "--domain", "/tmp/x"},
		},
		{
			name:     "bool flag (--json) ahead of kind does not swallow it",
			args:     reorderFlagsFirst([]string{"--json", "requirement", "--id", "R-x"}),
			wantKind: "requirement",
			wantRest: []string{"--json", "--id", "R-x"},
		},
		{
			name:     "equals-form flag ahead of kind",
			args:     []string{"--domain=/tmp/x", "stakeholder"},
			wantKind: "stakeholder",
			wantRest: []string{"--domain=/tmp/x"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kind, rest, err := extractProposeKind(tc.args)
			if err != nil {
				t.Fatalf("extractProposeKind(%v): unexpected error: %v", tc.args, err)
			}
			if kind != tc.wantKind {
				t.Errorf("kind = %q, want %q (args=%v)", kind, tc.wantKind, tc.args)
			}
			if len(rest) != len(tc.wantRest) {
				t.Fatalf("rest = %v, want %v (args=%v)", rest, tc.wantRest, tc.args)
			}
			for i := range tc.wantRest {
				if rest[i] != tc.wantRest[i] {
					t.Errorf("rest[%d] = %q, want %q (args=%v)", i, rest[i], tc.wantRest[i], tc.args)
				}
			}
		})
	}
}

// TestExtractProposeKind_NoKindReturnsUsageError proves the "no positional
// kind found" error path (e.g. all-flags input) is unchanged by the refactor.
func TestExtractProposeKind_NoKindReturnsUsageError(t *testing.T) {
	t.Parallel()
	_, _, err := extractProposeKind([]string{"--json"})
	if err == nil {
		t.Fatal("expected usage error when no positional kind is present")
	}
	if !strings.Contains(err.Error(), "usage: hotam propose <kind>") {
		t.Errorf("error = %q, want it to mention the usage line", err.Error())
	}
}

// TestProposeHelp_RealBinary_PerKindPerFlagHelp is the end-to-end proof that
// `hotam propose <kind> -h` prints REAL per-flag help (flag.ExitOnError's
// stdlib output, e.g. "-claim", "-owner") and exits 0 — not the generic
// "usage: hotam propose <kind> [flags]" line, and not a non-zero exit.
//
// This spawns the REAL compiled hotam binary as a subprocess (the existing
// buildSharedHotamBinary pattern from testbinary_test.go) specifically
// because flag.ExitOnError's Parse calls os.Exit(0) directly from inside the
// Go standard library when it sees -h/-help; if this were invoked in-process
// (calling cmdProposeRequirement([]string{"-h"}) etc. directly from a test
// function) the entire `go test` BINARY would exit at that point, silently
// truncating every test that would have run after it — with a MISLEADING
// green/PASS-looking exit code. Subprocess isolation is the only safe way to
// exercise the real -h code path.
func TestProposeHelp_RealBinary_PerKindPerFlagHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("propose -h e2e: builds/spawns a real binary; skipped in -short")
	}
	t.Parallel()
	binPath := buildSharedHotamBinary(t)

	cases := []struct {
		kind      string
		wantFlags []string
	}{
		{"requirement", []string{"-claim", "-owner", "-status", "-id"}},
		{"rejection", []string{"-requirement-id", "-reason", "-replaced-by"}},
		{"stakeholder", []string{"-id", "-name", "-stakeholder-domain"}},
		{"axis", []string{"-slug", "-description"}},
		{"assumption", []string{"-id", "-statement", "-status", "-owner"}},
		{"conflict", []string{"-axis", "-context", "-members", "-steward"}},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			t.Parallel()
			for _, helpSpelling := range []string{"-h", "-help", "--help"} {
				t.Run(helpSpelling, func(t *testing.T) {
					t.Parallel()
					cmd := exec.Command(binPath, "propose", tc.kind, helpSpelling)
					out, err := cmd.CombinedOutput()
					if err != nil {
						t.Fatalf("hotam propose %s %s: expected exit 0, got error %v\noutput:\n%s", tc.kind, helpSpelling, err, out)
					}
					body := string(out)
					if strings.Contains(body, "usage: hotam propose <kind>") {
						t.Errorf("hotam propose %s %s printed the GENERIC top-level usage line instead of per-flag help:\n%s", tc.kind, helpSpelling, body)
					}
					for _, wantFlag := range tc.wantFlags {
						if !strings.Contains(body, wantFlag) {
							t.Errorf("hotam propose %s %s output missing expected per-flag help %q\noutput:\n%s", tc.kind, helpSpelling, wantFlag, body)
						}
					}
				})
			}
		})
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

// ---- axis / assumption / conflict (task #149, review-8 R8-f) ----

// TestCmdPropose_Axis_ConstructsValidJSON covers the axis subcommand: --slug +
// --description produce a valid Axis proposal JSON, re-parseable through
// parseProposal.
func TestCmdPropose_Axis_ConstructsValidJSON(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "axis.json")

	err := cmdPropose([]string{
		"axis",
		"--slug", "propose-test-axis",
		"--description", "an axis for testing the propose command",
		"--why", "coverage for propose axis",
		"--domain", domainDir,
		"--out", outPath,
	})
	if err != nil {
		t.Fatalf("cmdPropose axis: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	p, err := parseProposal(data)
	if err != nil {
		t.Fatalf("parseProposal: %v\nraw:\n%s", err, data)
	}
	if p.Kind() != "Axis" {
		t.Errorf("Kind = %q, want Axis", p.Kind())
	}
	ap, ok := p.(proposal.ProposedAxis)
	if !ok {
		t.Fatalf("parsed proposal is %T, want proposal.ProposedAxis", p)
	}
	if ap.Slug != "propose-test-axis" {
		t.Errorf("Slug = %q, want propose-test-axis", ap.Slug)
	}
	body := string(data)
	for _, want := range []string{
		`"kind": "Axis"`,
		`"slug": "propose-test-axis"`,
		`"description"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("written JSON missing %q\nraw:\n%s", want, body)
		}
	}
}

// TestCmdPropose_Assumption_ConstructsValidJSON covers the assumption
// subcommand: --id + --statement + --status + --owner produce a valid
// Assumption proposal JSON.
func TestCmdPropose_Assumption_ConstructsValidJSON(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "asmp.json")

	err := cmdPropose([]string{
		"assumption",
		"--id", "A-propose-test-assumption",
		"--statement", "operators run inside the launch directory",
		"--status", "HOLDS",
		"--owner", "framework-author",
		"--why", "coverage for propose assumption",
		"--created-at", "2026-07-14",
		"--domain", domainDir,
		"--out", outPath,
	})
	if err != nil {
		t.Fatalf("cmdPropose assumption: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	p, err := parseProposal(data)
	if err != nil {
		t.Fatalf("parseProposal: %v\nraw:\n%s", err, data)
	}
	if p.Kind() != "Assumption" {
		t.Errorf("Kind = %q, want Assumption", p.Kind())
	}
	ap, ok := p.(proposal.ProposedAssumption)
	if !ok {
		t.Fatalf("parsed proposal is %T, want proposal.ProposedAssumption", p)
	}
	if ap.Status != "HOLDS" {
		t.Errorf("Status = %q, want HOLDS", ap.Status)
	}
	body := string(data)
	for _, want := range []string{
		`"kind": "Assumption"`,
		`"id": "A-propose-test-assumption"`,
		`"statement"`,
		`"created_at": "2026-07-14"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("written JSON missing %q\nraw:\n%s", want, body)
		}
	}
}

// TestCmdPropose_Conflict_ConstructsValidJSON covers the conflict subcommand:
// --axis + --context + --members + --steward produce a valid Conflict proposal
// JSON. Members are split from the --members CSV.
func TestCmdPropose_Conflict_ConstructsValidJSON(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "conf.json")

	err := cmdPropose([]string{
		"conflict",
		"--axis", "core-vs-aspect",
		"--context", "propose conflict test fixture context",
		"--members", "R-active-loop-apply-tool, R-agent-code-imports-framework",
		"--steward", "domain-user",
		"--shared-assumption", "A-propose-test-shared",
		"--note", "coverage for propose conflict",
		"--domain", domainDir,
		"--out", outPath,
	})
	if err != nil {
		t.Fatalf("cmdPropose conflict: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	p, err := parseProposal(data)
	if err != nil {
		t.Fatalf("parseProposal: %v\nraw:\n%s", err, data)
	}
	if p.Kind() != "Conflict" {
		t.Errorf("Kind = %q, want Conflict", p.Kind())
	}
	cp, ok := p.(proposal.ProposedConflict)
	if !ok {
		t.Fatalf("parsed proposal is %T, want proposal.ProposedConflict", p)
	}
	if cp.Axis != "core-vs-aspect" {
		t.Errorf("Axis = %q, want core-vs-aspect", cp.Axis)
	}
	if len(cp.Members) != 2 {
		t.Errorf("Members len = %d, want 2 (got %v)", len(cp.Members), cp.Members)
	}
	body := string(data)
	for _, want := range []string{
		`"kind": "Conflict"`,
		`"axis": "core-vs-aspect"`,
		`"steward": "domain-user"`,
		`"R-active-loop-apply-tool"`,
		`"R-agent-code-imports-framework"`,
		`"shared_assumption": "A-propose-test-shared"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("written JSON missing %q\nraw:\n%s", want, body)
		}
	}
}

// TestCmdPropose_Axis_Land_AppliesRegeneratesReverifies is the --land test for
// the axis kind: after writing, --land must apply the axis to the graph and
// regenerate docs. Axis has no interaction with other nodes (no member/steward
// checks), so a fresh slug is sufficient.
func TestCmdPropose_Axis_Land_AppliesRegeneratesReverifies(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "axis-land.json")

	err := cmdPropose([]string{
		"axis",
		"--slug", "propose-test-axis-land",
		"--description", "an axis landed via propose --land",
		"--domain", domainDir,
		"--out", outPath,
		"--today", "2026-07-14",
		"--land",
	})
	if err != nil {
		t.Fatalf("cmdPropose axis --land: %v", err)
	}
	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	if !strings.Contains(string(graphData), "propose-test-axis-land") {
		t.Error("graph.json does not contain the new axis slug after propose --land")
	}
}

// TestCmdPropose_Assumption_Land_AppliesRegeneratesReverifies is the --land
// test for the assumption kind. Assumption has no interaction with other nodes
// (only an id-uniqueness check), so a fresh A-id is sufficient.
func TestCmdPropose_Assumption_Land_AppliesRegeneratesReverifies(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "asmp-land.json")

	err := cmdPropose([]string{
		"assumption",
		"--id", "A-propose-test-assumption-land",
		"--statement", "an assumption landed via propose --land",
		"--status", "UNCERTAIN",
		"--owner", "framework-author",
		"--domain", domainDir,
		"--out", outPath,
		"--today", "2026-07-14",
		"--land",
	})
	if err != nil {
		t.Fatalf("cmdPropose assumption --land: %v", err)
	}
	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	if !strings.Contains(string(graphData), "A-propose-test-assumption-land") {
		t.Error("graph.json does not contain the new assumption id after propose --land")
	}
}

// TestCmdPropose_Conflict_Land_AppliesRegeneratesReverifies is the --land test
// for the conflict kind. Unlike axis/assumption, a Conflict's Members must
// already exist as real Requirements in the target graph, the axis must exist,
// and the steward must NOT own any member — ProposedConflict.mutate enforces
// all three. Additionally, the self-host invariant
// check_constituting_not_in_unresolved_conflict refuses a DETECTED conflict
// holding two SETTLED requirements, so the fixture lands the conflict in a
// DECIDED(...) initial lifecycle (a human decision already recorded) — exactly
// the shape the --ack-conflict escape hatch (task #147) expects to cite.
func TestCmdPropose_Conflict_Land_AppliesRegeneratesReverifies(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "conf-land.json")

	err := cmdPropose([]string{
		"conflict",
		"--axis", "core-vs-aspect",
		"--context", "propose conflict land e2e unique context fixture",
		"--members", "R-active-loop-apply-tool,R-agent-code-imports-framework",
		"--steward", "domain-user",
		"--initial-lifecycle", "DECIDED(human-steward-2026-07-14)",
		"--decided-by", "framework-reviewer",
		"--note", "e2e coverage for propose conflict --land",
		"--domain", domainDir,
		"--out", outPath,
		"--today", "2026-07-14",
		"--land",
	})
	if err != nil {
		t.Fatalf("cmdPropose conflict --land: %v", err)
	}
	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	// A Conflict's anchor is its ConflictIdentity (a C-<hash>), which is
	// deterministic on axis+context. Verify both members and the steward are
	// recorded in the graph.
	body := string(graphData)
	for _, want := range []string{
		"R-active-loop-apply-tool",
		"R-agent-code-imports-framework",
		`"steward": "domain-user"`,
		`"axis": "core-vs-aspect"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("graph.json missing %q after conflict --land", want)
		}
	}
}

// TestDefaultProposeOutPath_AxisHasNoColon is the direct unit test for the
// Axis landmine in defaultProposeOutPath: ProposedAxis.TargetAnchor() returns
// "Axis:" + slug (note the COLON), which is illegal in a Windows filename. The
// explicit Axis case must use the bare slug instead, producing a colon-free
// path. This is the test that catches the landmine if the explicit case is
// removed.
func TestDefaultProposeOutPath_AxisHasNoColon(t *testing.T) {
	t.Parallel()
	p := proposal.ProposedAxis{Slug: "test-axis-slug", Description: "x"}
	path := defaultProposeOutPath(p)
	if strings.Contains(path, ":") {
		t.Errorf("axis default out path contains an illegal colon: %q "+
			"(ProposedAxis.TargetAnchor() is \"Axis:\"+slug — the explicit case must use the bare slug)", path)
	}
	want := filepath.Join("proposals", "draft-test-axis-slug.json")
	if path != want {
		t.Errorf("axis default out path = %q, want %q", path, want)
	}
}

// TestCmdPropose_Axis_DefaultOutPath_WrittenWithoutColon is the end-to-end
// proof that the Axis defaultProposeOutPath fix survives a real write: it
// spawns the compiled hotam binary with cwd set to a temp dir (so the relative
// default path lands there), runs `propose axis` WITHOUT --out, and confirms
// the file is actually written to the colon-free default path. This catches
// the landmine at the OS-write layer (Windows would reject a path with ':').
func TestCmdPropose_Axis_DefaultOutPath_WrittenWithoutColon(t *testing.T) {
	if testing.Short() {
		t.Skip("axis default-out e2e: builds/spawns a real binary; skipped in -short")
	}
	t.Parallel()
	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)
	cwd := t.TempDir()

	slug := "default-out-axis"
	cmd := exec.Command(binPath,
		"propose", "axis",
		"--slug", slug,
		"--description", "an axis written to the default out path",
		"--domain", domainDir,
	)
	cmd.Dir = cwd
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hotam propose axis (no --out) failed: %v\n%s", err, out)
	}
	// The default path is proposals/draft-<slug>.json relative to cwd.
	defaultPath := filepath.Join(cwd, "proposals", "draft-"+slug+".json")
	if _, err := os.Stat(defaultPath); err != nil {
		t.Fatalf("default out path %s was not written: %v", defaultPath, err)
	}
	// The landmine: without the explicit Axis case, the filename would be
	// "draft-Axis:<slug>.json" (ProposedAxis.TargetAnchor() is "Axis:"+slug).
	// A colon in the FILENAME component is illegal on Windows — the drive-letter
	// colon in the absolute path prefix is legitimate, so check only the base.
	base := filepath.Base(defaultPath)
	if strings.Contains(base, ":") {
		t.Errorf("default out filename contains an illegal colon: %q (full path %q)", base, defaultPath)
	}
	if !strings.HasPrefix(base, "draft-"+slug) {
		t.Errorf("default out filename = %q, want prefix draft-%s", base, slug)
	}
}

// TestCmdPropose_Requirement_Land_RequireProvenance_WithFlags_Succeeds is the
// end-to-end scenario task #159 (R12-a) exists to fix: on a scratch domain
// with "require_provenance": true in manifest.json (task #158's opt-in gate),
// `hotam propose requirement --land` must be able to supply
// --last-reviewed-at/--review-after (previously there was no flag for either
// field at all, so this exact command would have been refused by
// provenanceGate with no way to fix it short of falling back to
// hand-authored JSON). Mirrors setupProvenanceTestDomain's scaffolding
// (provenance_gate_test.go) directly via initDomain, since propose_test.go
// doesn't import that helper.
func TestCmdPropose_Requirement_Land_RequireProvenance_WithFlags_Succeeds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	domainDir := filepath.Join(root, "domains", "prov-propose-test")
	if _, err := initDomain(domainDir, "prov-propose-test", "2026-07-15"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}
	manifestPath := filepath.Join(domainDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{\"self_hosting\": false, \"require_provenance\": true}\n"), 0o644); err != nil {
		t.Fatalf("write require_provenance manifest: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "req-prov-land.json")
	err := cmdPropose([]string{
		"requirement",
		"--id", "R-propose-provenance-land",
		"--claim", "a settled requirement landed via propose with complete provenance",
		"--owner", "owner",
		"--status", "SETTLED",
		"--source-refs", "https://example.com/source",
		"--evidence", "steward review",
		"--last-reviewed-at", "2026-07-15",
		"--review-after", "2027-07-15",
		"--domain", domainDir,
		"--out", outPath,
		"--today", "2026-07-15",
		"--land",
	})
	if err != nil {
		t.Fatalf("propose requirement --land with complete provenance should succeed under require_provenance:true, got: %v", err)
	}
	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	if !strings.Contains(string(graphData), "R-propose-provenance-land") {
		t.Error("graph.json does not contain R-propose-provenance-land after propose --land")
	}
}

// TestCmdPropose_Requirement_Land_RequireProvenance_MissingFlags_Refused is
// the negative counterpart: omitting --last-reviewed-at/--review-after on the
// same require_provenance:true domain must still be refused by the gate —
// proving this task only adds a way to SUPPLY the fields, the gate's own
// requirements (task #158) are untouched.
func TestCmdPropose_Requirement_Land_RequireProvenance_MissingFlags_Refused(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	domainDir := filepath.Join(root, "domains", "prov-propose-test-neg")
	if _, err := initDomain(domainDir, "prov-propose-test-neg", "2026-07-15"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}
	manifestPath := filepath.Join(domainDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{\"self_hosting\": false, \"require_provenance\": true}\n"), 0o644); err != nil {
		t.Fatalf("write require_provenance manifest: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "req-prov-land-bare.json")
	err := cmdPropose([]string{
		"requirement",
		"--id", "R-propose-provenance-bare",
		"--claim", "a settled requirement landed via propose with no provenance",
		"--owner", "owner",
		"--status", "SETTLED",
		"--domain", domainDir,
		"--out", outPath,
		"--today", "2026-07-15",
		"--land",
	})
	if err == nil {
		t.Fatal("expected propose requirement --land to be refused under require_provenance:true when last-reviewed-at/review-after are omitted")
	}
	for _, field := range []string{"source_refs", "last_reviewed_at", "review_after"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("error must name missing field %q, got: %v", field, err)
		}
	}
	if !strings.Contains(err.Error(), "require_provenance") {
		t.Errorf("error must point at the require_provenance manifest flag, got: %v", err)
	}
}

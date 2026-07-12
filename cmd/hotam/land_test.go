package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCmdLand_AppliesRegeneratesAndVerifies is the e2e test for `hotam land`
// (TaskList P1-4): a valid proposal lands against a domain fixture and the
// command must (1) exit 0, (2) leave graph.json containing the new node,
// and (3) leave docs/gen/*.md re-rendered so they actually describe the
// post-apply graph — this is exactly the gap internal/proposal/apply.go
// leaves open on its own (it writes graph.json + graph.lock but never
// touches docs/gen), which is the bug this command exists to close.
//
// copySelfDomain (main_test.go) supplies the fixture: a real, invariant-
// clean graph.json+manifest.json pair already used throughout this package
// for exactly this purpose, so land has something non-trivial to apply
// against without hand-building a synthetic graph that would need to
// satisfy all ~47 invariants from scratch.
func TestCmdLand_AppliesRegeneratesAndVerifies(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	genDir := filepath.Join(domainDir, "docs", "gen")

	// copySelfDomain only copies graph.json + manifest.json (main_test.go);
	// docs/gen/ does not exist yet, which itself proves any content found
	// there after land ran was produced BY land's gen-spec step, not
	// inherited from the fixture.
	reqPath := filepath.Join(genDir, "REQUIREMENTS.md")
	if _, err := os.Stat(reqPath); !os.IsNotExist(err) {
		t.Fatalf("expected docs/gen/REQUIREMENTS.md absent before land, stat err = %v", err)
	}

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-e2e-smoke",
		"claim": "hotam land applies a proposal and regenerates docs in one step",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "e2e coverage for the land pipeline"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal fixture: %v", err)
	}

	err := cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-12",
		proposalPath,
	})
	if err != nil {
		t.Fatalf("cmdLand: %v", err)
	}

	// graph.json (source of truth, sibling of manifest.json — NOT
	// docs/gen/graph.json) must contain the newly applied node.
	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	if !strings.Contains(string(graphData), "R-land-e2e-smoke") {
		t.Error("graph.json does not contain R-land-e2e-smoke after land")
	}

	// docs/gen/REQUIREMENTS.md must be freshly rendered and reflect the new
	// node — this is the specific drift TaskList P1-4 exists to close.
	after, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("read post-land REQUIREMENTS.md: %v", err)
	}
	if !strings.Contains(string(after), "R-land-e2e-smoke") {
		t.Error("docs/gen/REQUIREMENTS.md was not regenerated with the new requirement after land")
	}

	// docs/gen/graph.json (the rendered copy gen-spec writes) must also be
	// current, not just the domain-root graph.json apply-proposal wrote.
	genGraphData, err := os.ReadFile(filepath.Join(genDir, "graph.json"))
	if err != nil {
		t.Fatalf("read docs/gen/graph.json: %v", err)
	}
	if !strings.Contains(string(genGraphData), "R-land-e2e-smoke") {
		t.Error("docs/gen/graph.json was not regenerated after land")
	}
}

// TestCmdLand_MissingRequiredFlags mirrors apply-proposal's own flag
// validation (land shares the same --domain/--today contract for step 1).
func TestCmdLand_MissingRequiredFlags(t *testing.T) {
	t.Parallel()

	t.Run("no proposal arg", func(t *testing.T) {
		t.Parallel()
		err := cmdLand([]string{"--domain", "/tmp/d", "--today", "2026-07-12"})
		if err == nil {
			t.Fatal("expected error when no proposal file given")
		}
	})
	t.Run("missing domain", func(t *testing.T) {
		t.Parallel()
		err := cmdLand([]string{"--today", "2026-07-12", "proposal.json"})
		if err == nil {
			t.Fatal("expected error when --domain missing")
		}
	})
	t.Run("missing today", func(t *testing.T) {
		t.Parallel()
		err := cmdLand([]string{"--domain", "/tmp/d", "proposal.json"})
		if err == nil {
			t.Fatal("expected error when --today missing")
		}
	})
}

// TestCmdLand_InvalidProposalAppliesNothing proves step (a) failing stops
// the pipeline before gen-spec or all-violations run — an unparsable
// proposal must not silently regenerate docs from an unmodified graph and
// report success.
func TestCmdLand_InvalidProposalAppliesNothing(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)

	proposalPath := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(proposalPath, []byte(`{"kind":"Bogus"}`), 0o644); err != nil {
		t.Fatalf("write proposal fixture: %v", err)
	}

	err := cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-12",
		proposalPath,
	})
	if err == nil {
		t.Fatal("expected error for unknown proposal kind")
	}
	if !strings.Contains(err.Error(), "apply step failed") {
		t.Errorf("error = %q, want it to identify the apply step as the failure point", err.Error())
	}
}

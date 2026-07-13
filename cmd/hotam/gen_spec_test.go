package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenSpec_MissingGraphRendersCalmNotice enforces R-empty-content-gen-notice:
// when the active domain has NO graph.json at all (a freshly cloned framework
// with no domain populated yet), gen-spec must NOT fail — it must render a calm
// 'no content yet' notice into docs/gen/*.md, mirroring the empty-but-present
// case. The two situations are indistinguishable to an adopter with nothing
// modeled yet, so they produce identical output: a missing graph.json is
// substituted with an empty graph (loadGraphOrEmpty), and every generator
// already detects g.IsEmpty() and emits the generator.EmptyNotice placeholder.
//
// EXACT RULE (mechanically checked): genSpec against a temp dir containing NO
// graph.json returns no error, writes docs/gen/REQUIREMENTS.md, and that file
// contains the calm 'No domain content loaded' notice.
//
// Discrimination: see TestGenSpec_MissingGraph_MalformedStillErrors — a
// graph.json that EXISTS but is malformed (a decode error, not IsNotExist)
// must still surface as a real error, proving errors.Is(err, os.ErrNotExist)
// is the discrimination rather than a blanket error-swallow.
func TestGenSpec_MissingGraphRendersCalmNotice(t *testing.T) {
	t.Parallel()
	// A genuinely empty domain dir: exists, but NO graph.json.
	domainDir := t.TempDir()
	if _, err := os.Stat(filepath.Join(domainDir, "graph.json")); !os.IsNotExist(err) {
		t.Fatalf("precondition: graph.json must not exist in the temp domain dir")
	}

	written, err := genSpec(domainDir, "", "2026-07-12")
	if err != nil {
		t.Fatalf("R-empty-content-gen-notice: genSpec on missing graph.json must not fail, got: %v", err)
	}
	if len(written) == 0 {
		t.Fatal("R-empty-content-gen-notice: genSpec wrote no files")
	}

	reqMD, err := os.ReadFile(filepath.Join(domainDir, "docs", "gen", "REQUIREMENTS.md"))
	if err != nil {
		t.Fatalf("read generated REQUIREMENTS.md: %v", err)
	}
	const calmSubstring = "No domain content loaded"
	if !strings.Contains(string(reqMD), calmSubstring) {
		t.Fatalf("R-empty-content-gen-notice: generated REQUIREMENTS.md must carry the calm 'no content yet' notice, got:\n%s", string(reqMD))
	}

	// The calm notice is specific to emptiness: a generated graph.json (the
	// normalized artifact under docs/gen/) is also written, reflecting the
	// empty graph the generators ran over.
	genGraph := filepath.Join(domainDir, "docs", "gen", "graph.json")
	if _, err := os.Stat(genGraph); err != nil {
		t.Fatalf("R-empty-content-gen-notice: generated docs/gen/graph.json must be written, got: %v", err)
	}
}

// TestGenSpec_MissingGraph_MalformedStillErrors is the non-vacuity control: the
// calm missing-file path must NOT swallow genuine errors. A graph.json that
// EXISTS but is malformed (a decode error, which is NOT os.IsNotExist) must
// still propagate as a real error — proving the IsNotExist check is the
// discrimination, not a blanket error-swallow.
func TestGenSpec_MissingGraph_MalformedStillErrors(t *testing.T) {
	t.Parallel()
	domainDir := t.TempDir()
	garbage := []byte("{ this is not valid json")
	if err := os.WriteFile(filepath.Join(domainDir, "graph.json"), garbage, 0o644); err != nil {
		t.Fatalf("write malformed graph.json: %v", err)
	}
	if _, err := genSpec(domainDir, "", "2026-07-12"); err == nil {
		t.Fatal("R-empty-content-gen-notice non-vacuity: a malformed graph.json must still produce a real decode error, got nil")
	}
}

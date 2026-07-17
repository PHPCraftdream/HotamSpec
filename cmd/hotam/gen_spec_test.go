package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/paths"
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

	written, _, err := genSpec(domainDir, "", "2026-07-12", "", false)
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
	if _, _, err := genSpec(domainDir, "", "2026-07-12", "", false); err == nil {
		t.Fatal("R-empty-content-gen-notice non-vacuity: a malformed graph.json must still produce a real decode error, got nil")
	}
}

// chdirAndRestore changes the process working directory to dir for the
// duration of the test, restoring the original on cleanup. It mirrors
// internal/paths's same-named helper (project_root_chain_test.go), which is
// package-private there and so cannot be imported here. Used by the
// repoRootForDomain tier-3 test below, which must isolate CWD from this
// repo's own tree (whose domains/ marker would otherwise satisfy the
// ProjectRootOrRaise fallback) to exercise the no-markers-found branch.
func chdirAndRestore(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})
}

// TestRepoRootForDomain_DomainsConvention covers tier 1: a domainDir whose
// parent is literally "domains" derives repoRoot as the parent of domains/,
// with no CWD/env dependency (tier 1 is checked before any marker search).
// This is the common in-repo / external-project-convention case, unchanged
// by the tier-3 addition.
func TestRepoRootForDomain_DomainsConvention(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	domainDir := filepath.Join(root, "domains", "acme")
	got := repoRootForDomain(domainDir)
	if got != root {
		t.Errorf("repoRootForDomain(%q) = %q, want %q (parent of domains/)", domainDir, got, root)
	}
}

// TestRepoRootForDomain_ProjectRootResolves covers tier 2: a domainDir that
// does NOT follow the domains/<name> layout, where paths.ProjectRootOrRaise()
// succeeds (here via the R1 env override HOTAM_SPEC_PROJECT_ROOT). The
// resolved root must be the env-supplied project root — this preserves the
// CWD/env-based resolution that copySelfDomain-style fixtures (flat
// t.TempDir()/hotam-spec-self layouts) and TestGenSpec_SmokeWritesByte
// IdenticalFiles rely on when go test runs from inside this checkout.
func TestRepoRootForDomain_ProjectRootResolves(t *testing.T) {
	// Bare domain dir with no domains/ parent — tier 1 is skipped.
	domainDir := filepath.Join(t.TempDir(), "hotam-spec-self")
	target := t.TempDir()
	t.Setenv(paths.EnvProjectRoot, target)
	got := repoRootForDomain(domainDir)
	if got != target {
		t.Errorf("repoRootForDomain(%q) = %q, want env-resolved project root %q", domainDir, got, target)
	}
}

// TestRepoRootForDomain_NoProjectRootFallsBackToDomainDir covers tier 3: a
// bare domain dir with NO project markers discoverable (both project-root env
// vars cleared, CWD an isolated marker-less temp dir outside the repo). This
// is exactly the shape `hotam init <dir>` scaffolds anywhere on disk and then
// advertises via "next: hotam gen-spec --domain <dir>". repoRootForDomain
// must NOT error — it returns domainDir itself, which RenderDomainMapBlock
// turns into the graceful "domains/ directory absent" text (the e2e test
// asserts the rendered output; this asserts the resolution function).
//
// CWD/env isolation mirrors internal/paths's
// TestProjectRootOrRaise_FailureNoEnv exactly.
func TestRepoRootForDomain_NoProjectRootFallsBackToDomainDir(t *testing.T) {
	empty := t.TempDir()
	chdirAndRestore(t, empty)
	t.Setenv(paths.EnvProjectRoot, "")
	t.Setenv(paths.EnvDomainsRoot, "")

	// Hermeticity precondition: this test exercises the tier-3 fallback
	// (repoRootForDomain returns domainDir when NO project root resolves),
	// reachable only when the CWD ancestry carries no project-root marker
	// within MaxMarkerSearchDepth levels. On a host whose own directory tree
	// is polluted with stray markers (e.g. a developer home with domains/,
	// .claude/, CLAUDE.md from unrelated tooling) within that depth, R3
	// resolves and the tier-3 branch is unreachable — skip (green) with an
	// actionable message rather than fail red, since the production logic is
	// correct and only the test's environmental precondition is unmet.
	skipIfCwdAncestryNotHermetic(t, empty)

	domainDir := filepath.Join(t.TempDir(), "bare-acme")
	got := repoRootForDomain(domainDir)
	if got != domainDir {
		t.Errorf("repoRootForDomain(%q) = %q, want domainDir itself (tier-3 fallback when no project root resolves)", domainDir, got)
	}
}

package generator

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestRewriteRepoAbsPaths_WindowsStyle(t *testing.T) {
	root := `D:\dev\proj`
	in := "stale: D:\\dev\\proj\\CLAUDE.md differs from render"
	want := "stale: CLAUDE.md differs from render"
	if got := rewriteRepoAbsPaths(root, in); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRewriteRepoAbsPaths_POSIXStyle(t *testing.T) {
	root := "/home/u/proj"
	in := "stale: /home/u/proj/domains/x/docs/gen/SPEC.md differs"
	want := "stale: domains/x/docs/gen/SPEC.md differs"
	if got := rewriteRepoAbsPaths(root, in); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRewriteRepoAbsPaths_SeparatorMismatchHandled(t *testing.T) {
	// Root given with forward slashes, text with backslashes (and vice versa).
	if got := rewriteRepoAbsPaths(`D:/dev/proj`, `see D:\dev\proj\a\b.md`); got != "see a/b.md" {
		t.Fatalf("forward-slash root vs backslash text: got %q", got)
	}
	if got := rewriteRepoAbsPaths(`D:\dev\proj`, "see D:/dev/proj/a/b.md"); got != "see a/b.md" {
		t.Fatalf("backslash root vs forward-slash text: got %q", got)
	}
}

func TestRewriteRepoAbsPaths_BoundaryNotMangled(t *testing.T) {
	root := `D:\dev\proj`
	in := `D:\dev\proj2\keep.md and D:\dev\proj\drop.md`
	want := `D:\dev\proj2\keep.md and drop.md`
	if got := rewriteRepoAbsPaths(root, in); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRewriteRepoAbsPaths_OutsideRootLeftAlone(t *testing.T) {
	root := `D:\dev\proj`
	in := "other E:\\elsewhere\\x.md and D:\\other\\y.md stay"
	if got := rewriteRepoAbsPaths(root, in); got != in {
		t.Fatalf("got %q, want unchanged %q", got, in)
	}
}

func TestRewriteRepoAbsPaths_EmptyRootOrText(t *testing.T) {
	if got := rewriteRepoAbsPaths("", `D:\dev\proj\x.md`); got != `D:\dev\proj\x.md` {
		t.Fatalf("empty root: got %q", got)
	}
	if got := rewriteRepoAbsPaths(`D:\dev\proj`, ""); got != "" {
		t.Fatalf("empty text: got %q", got)
	}
}

// fixtureWithoutAgingAspiration loads the fixture graph and drops its
// aging IMPLEMENTS aspiration, whose P0 reflection otherwise outranks every
// P1 STRUCTURE violation regardless of `today` — LIVE-STATE and DOMAIN-MAP
// render only the single TOP action, so the injected violation must sit at
// signals[0] to appear on the page at all.
func fixtureWithoutAgingAspiration(t *testing.T) *ontology.Graph {
	g := loadFixtureGraph(t)
	kept := g.Assumptions[:0]
	for _, a := range g.Assumptions {
		if a.ID != "A-implements-example" {
			kept = append(kept, a)
		}
	}
	g.Assumptions = kept
	return g
}

// Render-level proof: a violation message carrying an absolute path under
// repoRoot comes out repo-root-relative in the rendered crystal content,
// and the absolute form appears nowhere. This test failed before
// relpath.go's rewrite was wired into the render sites.
func TestRenderBusinessContentWithViolations_RelativizesAbsPaths(t *testing.T) {
	g := fixtureWithoutAgingAspiration(t)
	repoRoot := `D:\dev\fakeproj`
	abs := `D:\dev\fakeproj\domains\alpha\docs\gen\SPEC.md`
	violations := &ViolationsOverride{
		For: g,
		Violations: []invariants.Violation{{
			Check:   "check_spec_md_current",
			ID:      "SPEC",
			Message: abs + " differs from fresh render",
		}},
	}
	out := renderBusinessContentWithViolations(g, "hotam-spec-self", repoRoot, 1000,
		map[string]*ontology.Graph{"hotam-spec-self": g}, "2026-01-10", false, violations, "")
	if strings.Contains(out, `D:\dev\fakeproj`) || strings.Contains(out, "D:/dev/fakeproj") {
		t.Fatalf("rendered crystal leaks absolute repo path:\n%s", out)
	}
	if !strings.Contains(out, "domains/alpha/docs/gen/SPEC.md") {
		t.Fatalf("rendered crystal lost the (rewritten) violation path:\n%s", out)
	}
}

// Same contract for the standalone docs/gen/live-state.md projection
// (LIVE-STATE prints only the single top action, so the injected violation
// must be the top signal: drop the fixture's aging IMPLEMENTS aspiration,
// whose P0 reflection otherwise outranks every P1 STRUCTURE violation
// regardless of `today`.)
func TestBuildLiveStateWithViolationsRoot_RelativizesAbsPaths(t *testing.T) {
	g := fixtureWithoutAgingAspiration(t)
	repoRoot := `D:\dev\fakeproj`
	abs := `D:\dev\fakeproj\CLAUDE.md`
	out := BuildLiveStateWithViolationsRoot(g, "hotam-spec-self", 1000, "2026-01-10",
		[]invariants.Violation{{Check: "check_domain_claude_md_current", ID: "CLAUDE", Message: abs + " stale"}}, repoRoot)
	if strings.Contains(out, `D:\dev\fakeproj`) {
		t.Fatalf("live-state leaks absolute repo path:\n%s", out)
	}
	if !strings.Contains(out, "CLAUDE.md stale") {
		t.Fatalf("live-state lost the rewritten violation message:\n%s", out)
	}
}

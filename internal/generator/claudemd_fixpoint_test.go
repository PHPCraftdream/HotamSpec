package generator

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestComputeCrystalCharCountFixpoint_Converges proves the resident crystal's
// CRYSTAL_CHARS measurement is a deterministic fixpoint: two independent calls
// return the identical value, and embedding that value via
// RenderClaudeMDFromTemplate yields a crystal whose rune count equals it. The
// fixture graph uses the NODE_COUNT budget measure, so its crystal does not
// embed the charCount at all (the self-referential feedback is absent) and the
// fixpoint collapses to the plain rune count — the meaningful self-referential
// convergence path is exercised against the real CRYSTAL_CHARS graph in
// TestComputeCrystalCharCountFixpoint_RealDomainGraph.
func TestComputeCrystalCharCountFixpoint_Converges(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	repoRoot := t.TempDir()
	const today = "2026-07-12"

	fp1, err := ComputeCrystalCharCountFixpoint(g, "hotam-spec-self", repoRoot, nil, today)
	if err != nil {
		t.Fatalf("ComputeCrystalCharCountFixpoint (call 1): %v", err)
	}
	fp2, err := ComputeCrystalCharCountFixpoint(g, "hotam-spec-self", repoRoot, nil, today)
	if err != nil {
		t.Fatalf("ComputeCrystalCharCountFixpoint (call 2): %v", err)
	}
	if fp1 != fp2 {
		t.Errorf("fixpoint not deterministic: call 1 = %d, call 2 = %d", fp1, fp2)
	}
	if fp1 == 0 {
		t.Fatalf("fixpoint is 0 — the measurement was not computed")
	}

	// Embedding the fixpoint must yield a crystal whose rune count equals the
	// fixpoint — the definition of convergence.
	rendered := RenderClaudeMDFromTemplate(g, "hotam-spec-self", repoRoot, fp1, nil, today)
	if measured := utf8.RuneCountInString(rendered); measured != fp1 {
		t.Errorf("fixpoint did not converge: embedding %d produced a crystal of %d runes (want %d)", fp1, measured, fp1)
	}
}

// TestComputeCrystalCharCountFixpoint_RealDomainGraph exercises the actual
// self-referential convergence path against the real hotam-spec-self domain
// graph, whose operator uses the CRYSTAL_CHARS budget measure — so the
// crystal's LIVE-STATE block DOES embed its own rune count, and the fixpoint
// must converge to a number that, once embedded, reproduces itself AND appears
// in the rendered "resident crystal N chars" line. This is the property CI's
// regen-idempotency check (.github/workflows/ci.yml) depends on.
func TestComputeCrystalCharCountFixpoint_RealDomainGraph(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	repoRoot := t.TempDir()
	const today = "2026-07-12"

	fp, err := ComputeCrystalCharCountFixpoint(g, "hotam-spec-self", repoRoot, nil, today)
	if err != nil {
		t.Fatalf("ComputeCrystalCharCountFixpoint on real graph: %v", err)
	}
	rendered := RenderClaudeMDFromTemplate(g, "hotam-spec-self", repoRoot, fp, nil, today)
	if got := utf8.RuneCountInString(rendered); got != fp {
		t.Errorf("fixpoint did not converge on real graph: embedding %d produced %d runes", fp, got)
	}

	// The converged number must be the one carried by the LIVE-STATE line.
	want := fmt.Sprintf("resident crystal %d chars", fp)
	if !strings.Contains(rendered, want) {
		t.Errorf("rendered real crystal does not contain the converged count %q", want)
	}
}

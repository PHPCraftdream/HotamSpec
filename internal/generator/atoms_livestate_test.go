package generator

import (
	"strings"
	"testing"
)

// TestBuildAtoms_* are covered by TestGenSpec_DeterministicOnRealDomain and
// TestGenSpec_SmokeOnRealDomain (byteidentical_test.go): BuildAtomsOperator/
// Substrate/Discipline/Check select SETTLED requirements by hardcoded
// real-domain ID prefixes (R-operator-, R-claude-md-, R-anchor-, R-check-,
// etc. — see atoms.go), so the small synthetic fixture (whose ids are all
// R-fixture-*) cannot exercise their non-empty branch; only the real
// hotam-spec-self domain can (P2-2). The empty-selection branch ("_No atomic
// requirements in this topic yet._") IS covered directly below.

func TestBuildAtomsOperator_EmptyOnFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildAtomsOperator(g)
	if !strings.Contains(got, "_No atomic requirements in this topic yet._") {
		t.Errorf("expected empty-selection notice for fixture graph (no R-operator-/R-agent-/R-boot-/R-prefer-tool- ids), got:\n%s", got)
	}
}

func TestBuildLiveState_RendersOnFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildLiveState(g, 5000)
	if strings.TrimSpace(got) == "" {
		t.Fatal("BuildLiveState: empty output on fixture graph")
	}
	for _, want := range []string{"top action:", "debt:", "graph:", "crystal:"} {
		if !strings.Contains(got, want) {
			t.Errorf("BuildLiveState output missing expected fragment %q:\n%s", want, got)
		}
	}
	// NODE_COUNT budget measure branch: the fixture operator uses
	// NODE_COUNT (not CRYSTAL_CHARS), so the rendered line must report
	// "nodes" and "NODE_COUNT measure", not "chars"/"CRYSTAL_CHARS".
	if !strings.Contains(got, "NODE_COUNT measure") {
		t.Errorf("BuildLiveState: expected NODE_COUNT measure branch for fixture graph, got:\n%s", got)
	}
}

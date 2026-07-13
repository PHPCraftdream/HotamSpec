package generator

import (
	"strings"
	"testing"
)

// TestRenderMediationLoopBlock_ConfrontReferencesPortedCommand is the
// acceptance test for the CONFRONT step of P1-4: the generated CLAUDE.md
// mediation loop must point operators at the real `hotam confront` command
// instead of the old "scan REQUIREMENTS.md/HISTORY.md by hand" fallback.
// Pinning this here (rather than only by regenerating the real CLAUDE.md)
// keeps the generator honest without requiring a full-domain golden that
// would need regenerating on every content change.
func TestRenderMediationLoopBlock_ConfrontReferencesPortedCommand(t *testing.T) {
	t.Parallel()
	got := RenderMediationLoopBlock()

	if strings.Contains(got, "scan REQUIREMENTS.md/HISTORY.md by hand") {
		t.Errorf("mediation loop still tells operators to scan by hand — CONFRONT should reference hotam confront:\n%s", got)
	}
	if strings.Contains(got, "Tool: not yet ported") {
		t.Errorf("mediation loop CONFRONT line still carries the stale fallback phrasing instead of the real command:\n%s", got)
	}
	if !strings.Contains(got, "hotam confront") {
		t.Errorf("mediation loop CONFRONT line does not reference the `hotam confront` command:\n%s", got)
	}
	if !strings.Contains(got, "**CONFRONT**") {
		t.Errorf("mediation loop lost the CONFRONT step heading:\n%s", got)
	}
}

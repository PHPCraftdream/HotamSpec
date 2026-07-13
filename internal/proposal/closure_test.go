package proposal

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestApply_ThenDiagnose_ClosesTriggeringAction enforces
// R-verify-closure-per-action COMPOSITIONALLY: a diagnosable condition flagged
// by diagnose.DiagnoseSignals (an ACKNOWLEDGED conflict stalled for a decision)
// is resolved by an applied ConflictTransition, and re-running the diagnosis
// confirms the triggering action is GONE. This proves the apply->re-diagnose
// loop actually closes the loop — not merely that apply does not crash.
//
// If apply ever landed a transition yet the same (Check, Target) action
// re-surfaced in the post-apply diagnosis, this test fails: the feedback edge
// that makes the active loop safe to automate (per the requirement's WHY) would
// be broken. The pre-apply Fatal guards against the fixture silently losing the
// triggering condition (which would make the post-apply assertion vacuous).
func TestApply_ThenDiagnose_ClosesTriggeringAction(t *testing.T) {
	t.Parallel()
	cid := ontology.ConflictIdentity("cost-vs-flexibility", "shared scenario")
	path := writeTempGraph(t, baseGraph())

	// BEFORE: the stalled-conflict action is present in the diagnosis.
	before := diagnose.DiagnoseSignals(reload(t, path), today)
	if !hasAcknowledgedStalledSignal(before, cid) {
		t.Fatalf("pre-apply fixture broken: no conflict_acknowledged_stalled signal for %s among %d signals — the triggering condition this test resolves is absent",
			cid, len(before))
	}

	// LAND: apply a DECIDED transition that resolves the stalled conflict.
	transition := ProposedConflictTransition{
		ConflictID:   cid,
		NewLifecycle: "DECIDED(chose option A for clarity)",
		DecidedBy:    "outsider",
	}
	if err := Apply(path, today, transition); err != nil {
		t.Fatalf("Apply DECIDED transition: %v", err)
	}

	// AFTER: re-run diagnosis; the triggering action is GONE.
	after := diagnose.DiagnoseSignals(reload(t, path), today)
	if hasAcknowledgedStalledSignal(after, cid) {
		t.Errorf("post-apply: conflict_acknowledged_stalled for %s still present after a DECIDED transition — apply->re-diagnose did not close the triggering action",
			cid)
	}
}

// hasAcknowledgedStalledSignal reports whether signals contains the
// conflict_acknowledged_stalled action (Check+Target) for conflictID. It keys on
// the exact (Check, Target) pair the diagnosis emits for an ACKNOWLEDGED
// conflict, so other unrelated signals never mask the result.
func hasAcknowledgedStalledSignal(signals []diagnose.Signal, conflictID string) bool {
	for _, s := range signals {
		if s.Check == "conflict_acknowledged_stalled" && s.Target == conflictID {
			return true
		}
	}
	return false
}

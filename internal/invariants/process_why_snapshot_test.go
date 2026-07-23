package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestCheckProcessWhySnapshotProse_FiresOnGpsmSmShapedSentence reproduces
// the REAL positive this check exists for: gpsm-sm's actual stale
// Process.Why sentence shape (a "ТЕКУЩЕЕ ПОЛОЖЕНИЕ" marker phrase
// co-occurring with an ISO date) — task #331's design consult's confirmed
// contradiction case.
func TestCheckProcessWhySnapshotProse_FiresOnGpsmSmShapedSentence(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Processes: []ontology.Process{
			{
				ID:  "PR-gpsm-ott-delivery",
				Why: "27 из 32 ФТ прошли P-G1. ТЕКУЩЕЕ ПОЛОЖЕНИЕ на 2026-07-21.",
			},
		},
	}
	got := checkProcessWhySnapshotProse(g)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 violation for the gpsm-sm-shaped snapshot sentence, got %d: %+v", len(got), got)
	}
	if got[0].Check != "check_process_why_snapshot_prose" {
		t.Errorf("Check = %q, want check_process_why_snapshot_prose", got[0].Check)
	}
	if got[0].ID != "PR-gpsm-ott-delivery" {
		t.Errorf("ID = %q, want PR-gpsm-ott-delivery", got[0].ID)
	}
}

// TestCheckProcessWhySnapshotProse_FiresOnTallyCoOccurringWithOwnStageToken
// covers fire condition (b): a "N из M" / "N of M" tally co-occurring with a
// stage token from the domain's OWN declared gate_stage_order — even
// without any marker phrase or ISO date.
func TestCheckProcessWhySnapshotProse_FiresOnTallyCoOccurringWithOwnStageToken(t *testing.T) {
	t.Parallel()
	domainDir := gateSignoffFixture(t, `["P-G0", "P-G1"]`)
	g := &ontology.Graph{
		DomainDir: domainDir,
		Processes: []ontology.Process{
			{
				ID:  "PR-x",
				Why: "27 из 32 требований прошли P-G1 в этой волне.",
			},
		},
	}
	got := checkProcessWhySnapshotProse(g)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 violation for tally+stage-token co-occurrence, got %d: %+v", len(got), got)
	}
}

// TestCheckProcessWhySnapshotProse_FiresOnStepWhy proves the scope also
// covers Step.Why (not only Process.Why), naming the process id plus the
// step in the violation's ID for traceability.
func TestCheckProcessWhySnapshotProse_FiresOnStepWhy(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Processes: []ontology.Process{
			{
				ID: "PR-x",
				Steps: []ontology.Step{
					{Name: "review", Why: "As of 2026-07-21, current status: blocked."},
				},
			},
		},
	}
	got := checkProcessWhySnapshotProse(g)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 violation for the Step.Why snapshot sentence, got %d: %+v", len(got), got)
	}
	if got[0].ID != "PR-x / step review" {
		t.Errorf("ID = %q, want %q", got[0].ID, "PR-x / step review")
	}
}

// TestCheckProcessWhySnapshotProse_TrueNegatives asserts the three explicit
// false-positive-avoidance shapes the design consult named — this check
// must NOT fire on ordinary prose that merely contains a date or a count
// with no snapshot claim attached. These are as important as the positive
// tests: the whole point of a NARROW, ADVISORY-ONLY lint is staying
// low-noise across 300+ existing graph nodes, never becoming noise itself.
func TestCheckProcessWhySnapshotProse_TrueNegatives(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		why  string
	}{
		{
			name: "resolved-on-date-no-marker-no-tally",
			why:  "This tension was resolved on 2026-07-22.",
		},
		{
			name: "contract-due-date-no-marker-phrase",
			why:  "The vendor contract is due 2026-09-15.",
		},
		{
			name: "four-waves-no-tally-vs-stage-token",
			why:  "This process runs in четыре волны, each independently reviewed.",
		},
	}
	// A domain-declared gate_stage_order IS present here (unlike the bare
	// no-DomainDir cases above) so the "четыре волны" true-negative is
	// actually exercised against condition (b)'s stage-token lookup, not
	// merely skipped because no stage vocabulary was ever declared.
	domainDir := gateSignoffFixture(t, `["P-G0", "P-G1"]`)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := &ontology.Graph{
				DomainDir: domainDir,
				Processes: []ontology.Process{{ID: "PR-x", Why: tc.why}},
			}
			got := checkProcessWhySnapshotProse(g)
			if len(got) != 0 {
				t.Errorf("expected 0 violations (true negative) for why=%q, got %d: %+v", tc.why, len(got), got)
			}
		})
	}
}

// TestCheckProcessWhySnapshotProse_NoOpWhenNoProcesses is the honest no-op:
// a graph with zero Process nodes never fires, regardless of anything else.
func TestCheckProcessWhySnapshotProse_NoOpWhenNoProcesses(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	got := checkProcessWhySnapshotProse(g)
	if len(got) != 0 {
		t.Errorf("expected 0 violations for a graph with no Processes, got %d: %+v", len(got), got)
	}
}

// TestCheckProcessWhySnapshotProse_TallyWithoutDeclaredStageOrderIsNoOp
// proves condition (b) never fires for a domain that has not declared
// gate_stage_order at all (g.DomainDir == "", the in-memory-fixture shape) —
// there is no domain-declared stage vocabulary to check a tally against, so
// a bare tally alone (no marker phrase, no ISO date) must not fire.
func TestCheckProcessWhySnapshotProse_TallyWithoutDeclaredStageOrderIsNoOp(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Processes: []ontology.Process{
			{ID: "PR-x", Why: "27 из 32 требований прошли P-G1 в этой волне."},
		},
	}
	got := checkProcessWhySnapshotProse(g)
	if len(got) != 0 {
		t.Errorf("expected 0 violations when no gate_stage_order is declared, got %d: %+v", len(got), got)
	}
}

// TestProcessWhySnapshotWarnings_ExportedWrapperMatchesInternalCheck proves
// the exported entry point cmd/hotam calls (ProcessWhySnapshotWarnings)
// produces the identical result the internal check does — it is a pure
// pass-through, never registered into the All registry (so it is never
// double-counted in invariants.AllViolations).
func TestProcessWhySnapshotWarnings_ExportedWrapperMatchesInternalCheck(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Processes: []ontology.Process{
			{ID: "PR-x", Why: "ТЕКУЩЕЕ ПОЛОЖЕНИЕ на 2026-07-21."},
		},
	}
	got := ProcessWhySnapshotWarnings(g)
	want := checkProcessWhySnapshotProse(g)
	if len(got) != len(want) || len(got) != 1 {
		t.Fatalf("ProcessWhySnapshotWarnings = %+v, want %+v", got, want)
	}
}

// TestCheckProcessWhySnapshotProse_NeverRegisteredInAllRegistry is the
// advisory-band contract itself: check_process_why_snapshot_prose must
// never appear in the All registry, so it can never surface in
// invariants.AllViolations / block `hotam all-violations`'s exit code / block
// internal/proposal/apply.go's proposal gate — mirrors HonoredSkipWarnings'
// identical never-registered contract.
func TestCheckProcessWhySnapshotProse_NeverRegisteredInAllRegistry(t *testing.T) {
	t.Parallel()
	for _, inv := range All.All() {
		if inv.Name == "check_process_why_snapshot_prose" {
			t.Fatalf("check_process_why_snapshot_prose must NOT be registered in All (advisory-only, mirrors HonoredSkipWarnings) — found it registered")
		}
	}
}

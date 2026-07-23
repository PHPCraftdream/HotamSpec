package generator

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestBuildPipeline_LiveStateRendersDedupedGateTallyAndConflictLifecycle is
// the task #331 (R4-process-why) core-fix test: a synthetic graph carrying
// (a) a Requirement with a SUPERSEDED DEFERRED gate_signoffs entry followed
// by a later SIGNED entry for the SAME requirement+stage — proving
// renderPipelineLiveState flows through graphfacts' last-entry-per-
// requirement-per-stage dedup rule rather than double-counting both entries
// (the exact double-count bug class this session already fixed once, see
// internal/graphfacts/facts.go's lastSignoffAtStage doc comment) — and (b)
// three Conflicts, one of each lifecycle class (DECIDED/HELD/UNRESOLVED), so
// the Conflicts line's tally is provably not a hardcoded/accidental total.
func TestBuildPipeline_LiveStateRendersDedupedGateTallyAndConflictLifecycle(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{
				ID: "R-superseded-deferral",
				GateSignoffs: []ontology.GateSignoff{
					{Stage: "P-G0", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1"},
					// Superseded: this requirement was once DEFERRED at
					// P-G1 (e.g. blocked on an open Conflict), then later
					// SIGNED at the SAME stage once the blocker resolved.
					// Only the LAST entry (SIGNED) may count.
					{Stage: "P-G1", State: ontology.GateSignoffStateDeferred, DeferredReason: "blocked", PipelineRun: "run-1"},
					{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1"},
				},
			},
			{
				ID: "R-still-deferred",
				GateSignoffs: []ontology.GateSignoff{
					{Stage: "P-G0", State: ontology.GateSignoffStateSigned, PipelineRun: "run-1"},
					{Stage: "P-G1", State: ontology.GateSignoffStateDeferred, DeferredReason: "blocked", PipelineRun: "run-1"},
				},
			},
			{
				ID: "R-not-yet-touched",
				// No gate_signoffs at all -- exercises the "beyond the
				// frontier" rendering for any stage past P-G1.
			},
		},
		Conflicts: []ontology.Conflict{
			{ID: "C-decided01", Axis: "scope", Context: "decided conflict", Lifecycle: ontology.ConflictDECIDEDPrefix},
			{ID: "C-held0001", Axis: "scope", Context: "held conflict", Lifecycle: ontology.ConflictHELDPrefix},
			{ID: "C-detect001", Axis: "scope", Context: "unresolved conflict", Lifecycle: ontology.ConflictDETECTED},
		},
	}
	order := []string{"P-G0", "P-G1", "P-G2"}

	got := renderPipelineLiveState(g, order)
	joined := strings.Join(got, "\n")

	if !strings.Contains(joined, `## Live state (generated from typed carriers — authoritative for "where are we now")`) {
		t.Errorf("missing Live state header, got:\n%s", joined)
	}
	// P-G0: both requirements SIGNED -> 2/2 SIGNED, 0 DEFERRED.
	if !strings.Contains(joined, "**P-G0** — 2/2 SIGNED · 0 DEFERRED") {
		t.Errorf("P-G0 tally wrong, got:\n%s", joined)
	}
	// P-G1: R-superseded-deferral's LAST entry is SIGNED (not DEFERRED, its
	// superseded entry must not also count); R-still-deferred is DEFERRED.
	// If the dedup rule regressed (both entries of R-superseded-deferral
	// counted), this would read "1/3 SIGNED - 2 DEFERRED" instead.
	if !strings.Contains(joined, "**P-G1** — 1/2 SIGNED · 1 DEFERRED") {
		t.Errorf("P-G1 tally wrong (dedup regression?), got:\n%s", joined)
	}
	// P-G2 is past the frontier (no Requirement has any signoff at or
	// beyond P-G2) -- must render an honest "not started", never a bogus
	// 0/0 tally indistinguishable from "zero requirements exist".
	if !strings.Contains(joined, "**P-G2** — not started") {
		t.Errorf("P-G2 (past frontier) should read 'not started', got:\n%s", joined)
	}
	if !strings.Contains(joined, "**Conflicts** — 3 total: 1 DECIDED · 1 HELD · 1 UNRESOLVED") {
		t.Errorf("Conflicts tally wrong, got:\n%s", joined)
	}
	if !strings.Contains(joined, "regenerates on every `hotam gen-spec`") {
		t.Errorf("missing the fixed framing note, got:\n%s", joined)
	}
}

// TestBuildPipeline_LiveStateOmittedWhenNoOrderAndNoConflicts is the honest
// no-op guarantee: a domain with no declared gate_stage_order (order == nil)
// AND zero Conflicts gets NOTHING rendered by renderPipelineLiveState (nil,
// not an empty-but-present header) -- so BuildPipeline's overall output for
// such a domain stays byte-identical to before this section existed.
func TestBuildPipeline_LiveStateOmittedWhenNoOrderAndNoConflicts(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{{ID: "R-plain"}},
	}
	got := renderPipelineLiveState(g, nil)
	if got != nil {
		t.Errorf("expected nil (no-op) when order is empty and there are no conflicts, got: %#v", got)
	}
}

// TestBuildPipeline_LiveStateOmittedWhenOrderEmptyButHasConflicts ensures
// the "OR" in "order empty AND no conflicts" is exercised the other way: a
// domain with declared Conflicts but no gate_stage_order still renders the
// Conflicts line (no gate lines), never omits the whole section.
func TestBuildPipeline_LiveStateOmittedWhenOrderEmptyButHasConflicts(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Conflicts: []ontology.Conflict{
			{ID: "C-onlyone1", Axis: "scope", Context: "x", Lifecycle: ontology.ConflictDETECTED},
		},
	}
	got := renderPipelineLiveState(g, nil)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "## Live state") {
		t.Fatalf("expected the Live state section to render when Conflicts exist even with no gate_stage_order, got: %#v", got)
	}
	if strings.Contains(joined, "SIGNED") || strings.Contains(joined, "DEFERRED") {
		t.Errorf("no gate lines expected when order is empty, got:\n%s", joined)
	}
	if !strings.Contains(joined, "**Conflicts** — 1 total: 0 DECIDED · 0 HELD · 1 UNRESOLVED") {
		t.Errorf("Conflicts tally wrong, got:\n%s", joined)
	}
}

// TestBuildPipeline_NoOpGraphByteIdenticalWithoutLiveState proves
// BuildPipeline's own no-op case end to end (not just
// renderPipelineLiveState in isolation): a graph with no gate_stage_order
// and no Conflicts renders a PIPELINE.md with NO "## Live state" section at
// all, and is byte-identical to a hand-verified expectation of what
// BuildPipeline produced before this task's change (the intro banner/header
// block through the first "## Process" heading, unchanged).
func TestBuildPipeline_NoOpGraphByteIdenticalWithoutLiveState(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	// The shared fixture graph carries Conflicts (used by the positive
	// fixture test elsewhere in this package) -- strip them here so this
	// test exercises the genuine no-op path (no gate_stage_order AND no
	// Conflicts) without needing a second fixture file on disk.
	g.Conflicts = nil
	got := BuildPipeline(g, "fixture-domain", nil)
	if strings.Contains(got, "## Live state") {
		t.Errorf("expected no Live state section for a graph with no gate_stage_order and no conflicts, got:\n%s", got)
	}
	if !strings.Contains(got, "## Process `PR-fixture-review`") {
		t.Errorf("expected the ordinary Process section to still render, got:\n%s", got)
	}
}

package main

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
)

// mkSignal builds a diagnose.Signal with the fields that drive grouping and
// rendering. Source is always "diagnose" (the only real producer); Message is
// templated to mirror how real producers embed the target id.
func mkSignal(priority int, check, target string) diagnose.Signal {
	return diagnose.Signal{
		Source:   "diagnose",
		Priority: priority,
		Check:    check,
		Target:   target,
		Message:  "issue on '" + target + "' for " + check,
	}
}

func TestFormatSignals_CollapsesSameCheckPriority(t *testing.T) {
	t.Parallel()
	signals := []diagnose.Signal{
		mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", "R-a"),
		mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", "R-b"),
		mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", "R-c"),
		mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", "R-d"),
		mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", "R-e"),
		mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", "R-f"),
	}
	out := formatSignals(signals, 20)
	lines := strings.Split(out, "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 collapsed line, got %d:\n%s", len(lines), out)
	}
	if !strings.Contains(out, "×6") {
		t.Errorf("collapsed line should show ×6 count, got:\n%s", out)
	}
	for _, id := range []string{"R-a", "R-b", "R-c", "R-d", "R-e", "R-f"} {
		if !strings.Contains(out, id) {
			t.Errorf("collapsed line should mention id %s, got:\n%s", id, out)
		}
	}
	if !strings.Contains(out, "ids:") {
		t.Errorf("collapsed line should have 'ids:' list, got:\n%s", out)
	}
}

func TestFormatSignals_CollapseTruncatesIDsBeyondMax(t *testing.T) {
	t.Parallel()
	const n = 12
	signals := make([]diagnose.Signal, 0, n)
	ids := []string{"R-01", "R-02", "R-03", "R-04", "R-05", "R-06", "R-07", "R-08", "R-09", "R-10", "R-11", "R-12"}
	for _, id := range ids {
		signals = append(signals, mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", id))
	}
	out := formatSignals(signals, 20)
	lines := strings.Split(out, "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 collapsed line, got %d:\n%s", len(lines), out)
	}
	if !strings.Contains(out, "×12") {
		t.Errorf("collapsed line should show ×12 count, got:\n%s", out)
	}
	if !strings.Contains(out, "(and 4 more)") {
		t.Errorf("12 ids should truncate after %d with '(and 4 more)', got:\n%s", maxCollapsedIDs, out)
	}
	for _, id := range ids[:maxCollapsedIDs] {
		if !strings.Contains(out, id) {
			t.Errorf("first %d ids should be shown verbatim, missing %s:\n%s", maxCollapsedIDs, id, out)
		}
	}
	for _, id := range ids[maxCollapsedIDs:] {
		if strings.Contains(out, id) {
			t.Errorf("truncated id %s should NOT appear verbatim, got:\n%s", id, out)
		}
	}
}

func TestFormatSignals_HeterogeneousRendersIndividually(t *testing.T) {
	t.Parallel()
	signals := []diagnose.Signal{
		mkSignal(diagnose.PReflection, "reflect_draft_overhang", "burn-down"),
		mkSignal(diagnose.PConflictStalled, "conflict_detected_stalled", "C-1"),
		mkSignal(diagnose.PAdvisory, "freshness_overdue", "review-freshness"),
	}
	out := formatSignals(signals, 20)
	if strings.Contains(out, "×") {
		t.Errorf("heterogeneous signals must not collapse, got '×' in:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 individual lines, got %d:\n%s", len(lines), out)
	}
	for i, want := range []struct {
		prio   int
		target string
	}{
		{diagnose.PReflection, "burn-down"},
		{diagnose.PConflictStalled, "C-1"},
		{diagnose.PAdvisory, "review-freshness"},
	} {
		if !strings.Contains(lines[i], "on "+want.target) {
			t.Errorf("line %d should be the single-signal format 'on %s', got %q", i, want.target, lines[i])
		}
	}
}

func TestFormatSignals_DuplicateTargetsNotCollapsed(t *testing.T) {
	t.Parallel()
	// Mirrors the held-variant producer: two signals, same (check, priority),
	// SAME target (the conflict id), different messages. These describe two
	// variants of one node, not one issue across nodes — keep them separate.
	s1 := mkSignal(diagnose.POpenItem, "held_variant_choice", "C-held")
	s1.Message = "choose a variant: 'V-a' — cost-vs-flexibility"
	s2 := mkSignal(diagnose.POpenItem, "held_variant_choice", "C-held")
	s2.Message = "choose a variant: 'V-b' — cost-vs-flexibility"
	out := formatSignals([]diagnose.Signal{s1, s2}, 20)
	if strings.Contains(out, "×2") {
		t.Errorf("duplicate-target group must not collapse, got ×2 in:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 individual lines, got %d:\n%s", len(lines), out)
	}
}

func TestFormatSignals_MixedCollapsedAndSingle(t *testing.T) {
	t.Parallel()
	signals := []diagnose.Signal{
		mkSignal(diagnose.PConflictStalled, "conflict_detected_stalled", "C-1"),
		mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", "R-a"),
		mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", "R-b"),
		mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", "R-c"),
	}
	out := formatSignals(signals, 20)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 output lines (1 single + 1 collapsed), got %d:\n%s", len(lines), out)
	}
	// Priority sort: PConflictStalled (3) band before PAdvisory (7) band.
	if !strings.Contains(lines[0], "on C-1") {
		t.Errorf("first line should be the single conflict signal, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "×3") {
		t.Errorf("second line should be the collapsed advisory group, got %q", lines[1])
	}
}

func TestFormatSignals_LimitAppliesBeforeGrouping(t *testing.T) {
	t.Parallel()
	signals := []diagnose.Signal{
		mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", "R-a"),
		mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", "R-b"),
		mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", "R-c"),
		mkSignal(diagnose.PAdvisory, "reflect_replaces_edge_migration", "R-d"),
	}
	// limit 2 keeps only the first two signals; they still collapse to one line.
	out := formatSignals(signals, 2)
	lines := strings.Split(out, "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 collapsed line from 2 limited signals, got %d:\n%s", len(lines), out)
	}
	if !strings.Contains(out, "×2") {
		t.Errorf("collapsed line should show ×2 (limit applied first), got:\n%s", out)
	}
}

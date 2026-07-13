package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/freshness"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// writeStatusTestDomain mirrors writeDueTestDomain's shape (cmd/hotam/due_test.go)
// but is standalone so status_test.go doesn't depend on due_test.go's fixture
// staying stable independently: a mix of OVERDUE, NEVER-REVIEWED, DUE-SOON,
// enforced/unenforced SETTLED, DRAFT and OPEN requirements so every field of
// StatusReport has a non-trivial value to assert on.
func writeStatusTestDomain(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-overdue-1", Owner: "sa", Status: ontology.StatusSETTLED, ReviewAfter: "2020-01-01", Enforcement: ontology.EnforcementENFORCED, Enforceability: ontology.EnforceabilityENFORCEABLE},
			{ID: "R-overdue-2", Owner: "sb", Status: ontology.StatusSETTLED, ReviewAfter: "2026-07-01"},
			{ID: "R-never-1", Owner: "sa", Status: ontology.StatusSETTLED},
			{ID: "R-never-2", Owner: "sb", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementENFORCED, Enforceability: ontology.EnforceabilityENFORCEABLE},
			{ID: "R-fresh", Owner: "sa", Status: ontology.StatusSETTLED, ReviewAfter: "2030-01-01"},
			{ID: "R-draft", Owner: "sa", Status: ontology.StatusDRAFT},
		},
	}
	path := filepath.Join(dir, "graph.json")
	if err := loader.WriteGraph(path, g); err != nil {
		t.Fatalf("write graph: %v", err)
	}
	return dir
}

// TestBuildStatusReport_MatchesWhatNowDueAllViolations is the real regression
// protection this command exists to guard: it independently calls the exact
// functions what-now (diagnose.DiagnoseSignals), due (freshness.ClassifyGraph),
// and all-violations (invariants.AllViolations) call on the SAME loaded
// graph, and asserts buildStatusReport's numbers agree field-by-field. If
// status's aggregation ever drifts from the sources it composes over — a
// copy-pasted definition of "closeable debt" that diverges, a different
// freshness window — this test catches it, not a human spot-checking output.
func TestBuildStatusReport_MatchesWhatNowDueAllViolations(t *testing.T) {
	t.Parallel()
	domainDir := writeStatusTestDomain(t)
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	today := "2026-07-12"

	report := buildStatusReport(g, today)

	// Independently recompute via the same paths what-now/due/all-violations use.
	signals := diagnose.DiagnoseSignals(g)
	wantTopAction := "none — graph clean"
	if len(signals) > 0 {
		wantTopAction = formatSingleSignal(signals[0])
	}
	if report.TopAction != wantTopAction {
		t.Errorf("TopAction = %q, want %q (from diagnose.DiagnoseSignals, same as what-now)", report.TopAction, wantTopAction)
	}

	var wantSettled, wantEnforced, wantCloseable int
	for _, r := range g.Requirements {
		if r.Status != ontology.StatusSETTLED {
			continue
		}
		wantSettled++
		if r.Enforcement == ontology.EnforcementENFORCED {
			wantEnforced++
		}
		if r.IsCloseableDebt() {
			wantCloseable++
		}
	}
	if report.SettledCount != wantSettled {
		t.Errorf("SettledCount = %d, want %d", report.SettledCount, wantSettled)
	}
	if report.EnforcedCount != wantEnforced {
		t.Errorf("EnforcedCount = %d, want %d", report.EnforcedCount, wantEnforced)
	}
	if report.CloseableDebtCount != wantCloseable {
		t.Errorf("CloseableDebtCount = %d, want %d", report.CloseableDebtCount, wantCloseable)
	}

	dueReport := buildDueReport(g, today)
	if report.OverdueCount != dueReport.OverdueCount {
		t.Errorf("OverdueCount = %d, want %d (from buildDueReport, same as `hotam due`)", report.OverdueCount, dueReport.OverdueCount)
	}
	if report.NeverReviewedCount != dueReport.NeverReviewedCount {
		t.Errorf("NeverReviewedCount = %d, want %d (from buildDueReport, same as `hotam due`)", report.NeverReviewedCount, dueReport.NeverReviewedCount)
	}
	// Cross-check against freshness.ClassifyGraph directly too, so this test
	// does not just prove agreement with another aggregation function but
	// with the underlying primitive both status and due call.
	classified := freshness.ClassifyGraph(g, today)
	var wantOverdue, wantNeverReviewed int
	for _, c := range classified {
		switch c.Status {
		case freshness.Overdue:
			wantOverdue++
		case freshness.NeverReviewed:
			wantNeverReviewed++
		}
	}
	if report.OverdueCount != wantOverdue {
		t.Errorf("OverdueCount = %d, want %d (from freshness.ClassifyGraph directly)", report.OverdueCount, wantOverdue)
	}
	if report.NeverReviewedCount != wantNeverReviewed {
		t.Errorf("NeverReviewedCount = %d, want %d (from freshness.ClassifyGraph directly)", report.NeverReviewedCount, wantNeverReviewed)
	}

	wantViolations := invariants.AllViolations(g)
	if report.ViolationCount != len(wantViolations) {
		t.Errorf("ViolationCount = %d, want %d (from invariants.AllViolations, same as `hotam all-violations`)", report.ViolationCount, len(wantViolations))
	}

	wantNodes := len(g.Requirements) + len(g.Conflicts) + len(g.Assumptions)
	if report.NodeCount != wantNodes {
		t.Errorf("NodeCount = %d, want %d", report.NodeCount, wantNodes)
	}
}

// TestBuildStatusReport_MatchesOnRealDomain runs the same cross-check as
// above but against the real hotam-spec-self domain graph, so the
// consistency guarantee holds on production-shaped data, not just the
// small synthetic fixture.
func TestBuildStatusReport_MatchesOnRealDomain(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	today := "2026-07-12"

	report := buildStatusReport(g, today)

	signals := diagnose.DiagnoseSignals(g)
	wantTopAction := "none — graph clean"
	if len(signals) > 0 {
		wantTopAction = formatSingleSignal(signals[0])
	}
	if report.TopAction != wantTopAction {
		t.Errorf("TopAction mismatch on real domain:\ngot:  %q\nwant: %q", report.TopAction, wantTopAction)
	}

	dueReport := buildDueReport(g, today)
	if report.OverdueCount != dueReport.OverdueCount || report.NeverReviewedCount != dueReport.NeverReviewedCount {
		t.Errorf("freshness mismatch on real domain: got overdue=%d never=%d, want overdue=%d never=%d",
			report.OverdueCount, report.NeverReviewedCount, dueReport.OverdueCount, dueReport.NeverReviewedCount)
	}

	violations := invariants.AllViolations(g)
	if report.ViolationCount != len(violations) {
		t.Errorf("ViolationCount mismatch on real domain: got %d, want %d", report.ViolationCount, len(violations))
	}
}

// TestFormatStatusReport_ContainsLabeledLines verifies the human-readable
// rendering has one clearly labeled line per field an agent would otherwise
// have to gather from three separate command outputs.
func TestFormatStatusReport_ContainsLabeledLines(t *testing.T) {
	t.Parallel()
	r := StatusReport{
		Today:              "2026-07-12",
		TopAction:          "[P0] REFLECTION on `x` — something",
		SettledCount:       10,
		EnforcedCount:      4,
		CloseableDebtCount: 6,
		OverdueCount:       2,
		NeverReviewedCount: 3,
		ViolationCount:     0,
		NodeCount:          42,
	}
	text := formatStatusReport(r)
	for _, want := range []string{
		"top action:",
		"[P0] REFLECTION on `x` — something",
		"debt:",
		"4/10",
		"6 closeable debt",
		"freshness:",
		"2 overdue",
		"3 never-reviewed",
		"violations:",
		"0",
		"graph:",
		"42 nodes",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("formatStatusReport missing %q:\n%s", want, text)
		}
	}
}

// TestStatusReport_JSONRoundTrips verifies the JSON encoding uses the exact
// flat field names an agent is told to expect (top_action, settled_count,
// enforced_count, closeable_debt_count, overdue_count, never_reviewed_count,
// violation_count, node_count) and that a round trip preserves values.
func TestStatusReport_JSONRoundTrips(t *testing.T) {
	t.Parallel()
	r := StatusReport{
		Today:              "2026-07-12",
		TopAction:          "none — graph clean",
		SettledCount:       10,
		EnforcedCount:      4,
		CloseableDebtCount: 6,
		OverdueCount:       2,
		NeverReviewedCount: 3,
		ViolationCount:     1,
		NodeCount:          42,
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	for _, key := range []string{
		"today", "top_action", "settled_count", "enforced_count",
		"closeable_debt_count", "overdue_count", "never_reviewed_count",
		"violation_count", "node_count",
	} {
		if _, ok := fields[key]; !ok {
			t.Errorf("JSON output missing expected field %q: %s", key, data)
		}
	}

	var back StatusReport
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal back to StatusReport: %v", err)
	}
	if back != r {
		t.Errorf("round trip mismatch: got %+v, want %+v", back, r)
	}
}

// TestCmdStatus_SmokeNoPanicOnRealDomain verifies the command runs end to
// end (flag parsing, domain resolution, graph load, human-readable render)
// against the real hotam-spec-self domain without error, and that status
// always exits nil (never gates) exactly like due/inspect/confront.
func TestCmdStatus_SmokeNoPanicOnRealDomain(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	err := cmdStatus([]string{"--domain", domainDir, "--today", "2026-07-12"})
	if err != nil {
		t.Fatalf("cmdStatus: %v", err)
	}
}

// TestCmdStatus_JSONFlagNoPanic verifies the --json branch also runs clean.
func TestCmdStatus_JSONFlagNoPanic(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	err := cmdStatus([]string{"--domain", domainDir, "--today", "2026-07-12", "--json"})
	if err != nil {
		t.Fatalf("cmdStatus --json: %v", err)
	}
}

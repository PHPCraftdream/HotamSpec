package gate

import (
	"sort"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestSelectTier1_CheckNameResolves(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-test-check", EnforcedBy: []string{"check_axis_in_registry"}},
		},
	}
	result := SelectTier1("R-test-check", g)
	if !result.Confident {
		t.Fatalf("expected confident=true, got false: %s", result.Reason)
	}
	m, err := BuildCheckToTestsMap(defaultInvariantsDir())
	if err != nil {
		t.Fatalf("BuildCheckToTestsMap: %v", err)
	}
	hits, ok := m["check_axis_in_registry"]
	if !ok || len(hits) == 0 {
		t.Fatalf("expected check_axis_in_registry in map, got %v", m)
	}
	want := append([]string{}, hits...)
	want = append(want, alwaysRun...)
	sort.Strings(want)
	if !equalSlices(result.NodeIDs, want) {
		t.Fatalf("NodeIDs mismatch:\n got %v\nwant %v", result.NodeIDs, want)
	}
	for _, h := range hits {
		if !strings.HasPrefix(h, "Test") {
			t.Fatalf("expected all resolved names to start with Test, got %q", h)
		}
	}
}

func TestSelectTier1_TestFuncNameResolves(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-test-func", EnforcedBy: []string{"TestCheckAxisInRegistry_OK"}},
		},
	}
	result := SelectTier1("R-test-func", g)
	if !result.Confident {
		t.Fatalf("expected confident=true, got false: %s", result.Reason)
	}
	want := append([]string{}, alwaysRun...)
	want = append(want, "TestCheckAxisInRegistry_OK")
	sort.Strings(want)
	if !equalSlices(result.NodeIDs, want) {
		t.Fatalf("NodeIDs mismatch:\n got %v\nwant %v", result.NodeIDs, want)
	}
}

func TestSelectTier1_MultipleChecksUnion(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-multi", EnforcedBy: []string{"check_axis_in_registry", "check_conflict_has_axis"}},
		},
	}
	result := SelectTier1("R-multi", g)
	if !result.Confident {
		t.Fatalf("expected confident=true, got false: %s", result.Reason)
	}
	m, err := BuildCheckToTestsMap(defaultInvariantsDir())
	if err != nil {
		t.Fatalf("BuildCheckToTestsMap: %v", err)
	}
	wantSet := map[string]struct{}{}
	for _, n := range alwaysRun {
		wantSet[n] = struct{}{}
	}
	for _, check := range []string{"check_axis_in_registry", "check_conflict_has_axis"} {
		for _, h := range m[check] {
			wantSet[h] = struct{}{}
		}
	}
	want := keysSorted(wantSet)
	if !equalSlices(result.NodeIDs, want) {
		t.Fatalf("NodeIDs mismatch:\n got %v\nwant %v", result.NodeIDs, want)
	}
}

func TestSelectTier1_FailClosed_TargetNotFound(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	result := SelectTier1("R-nonexistent", g)
	if result.Confident {
		t.Fatalf("expected confident=false for missing target")
	}
	if len(result.NodeIDs) != 0 {
		t.Fatalf("expected empty NodeIDs, got %v", result.NodeIDs)
	}
	if !strings.Contains(result.Reason, "not found") {
		t.Fatalf("reason should mention not found, got %q", result.Reason)
	}
}

func TestSelectTier1_FailClosed_EmptyEnforcedBy(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-empty-eb"},
		},
	}
	result := SelectTier1("R-empty-eb", g)
	if result.Confident {
		t.Fatalf("expected confident=false for empty enforced_by")
	}
	if !strings.Contains(result.Reason, "empty enforced_by") {
		t.Fatalf("reason should mention empty enforced_by, got %q", result.Reason)
	}
}

func TestSelectTier1_FailClosed_PythonTestPathUnresolved(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-py-path", EnforcedBy: []string{"test_apply_proposal.py"}},
		},
	}
	result := SelectTier1("R-py-path", g)
	if result.Confident {
		t.Fatalf("expected confident=false for an unresolvable .py test path entry")
	}
	if !strings.Contains(result.Reason, "could not be resolved") {
		t.Fatalf("reason should mention unresolved, got %q", result.Reason)
	}
}

func TestSelectTier1_FailClosed_BareTestFuncUnresolved(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-bare-test", EnforcedBy: []string{"test_smoke"}},
		},
	}
	result := SelectTier1("R-bare-test", g)
	if result.Confident {
		t.Fatalf("expected confident=false for bare test_ name")
	}
}

func TestSelectTier1_FailClosed_PartiallyUnresolved(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-partial", EnforcedBy: []string{"check_axis_in_registry", "CRITICAL_CORE_INVARIANTS"}},
		},
	}
	result := SelectTier1("R-partial", g)
	if result.Confident {
		t.Fatalf("expected confident=false for partially unresolved enforced_by")
	}
	if !strings.Contains(result.Reason, "CRITICAL_CORE_INVARIANTS") {
		t.Fatalf("reason should name the unresolved entry, got %q", result.Reason)
	}
}

func TestSelectTier1_FailClosed_CheckWithNoTestCoverage(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-no-coverage", EnforcedBy: []string{"check_nonexistent_fake_check"}},
		},
	}
	result := SelectTier1("R-no-coverage", g)
	if result.Confident {
		t.Fatalf("expected confident=false for check_ with no test coverage")
	}
}

func TestSelectTier1_FailClosed_ConflictTarget(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Conflicts: []ontology.Conflict{
			{ID: "C-test-conflict"},
		},
	}
	result := SelectTier1("C-test-conflict", g)
	if result.Confident {
		t.Fatalf("expected confident=false for Conflict target")
	}
	if !strings.Contains(result.Reason, "Conflict") {
		t.Fatalf("reason should mention Conflict, got %q", result.Reason)
	}
}

func TestBuildCheckToTestsMap_NonEmptyAndSorted(t *testing.T) {
	t.Parallel()
	m, err := BuildCheckToTestsMap(defaultInvariantsDir())
	if err != nil {
		t.Fatalf("BuildCheckToTestsMap: %v", err)
	}
	if len(m) == 0 {
		t.Fatalf("expected non-empty check-to-tests map")
	}
	for check, funcs := range m {
		if !strings.HasPrefix(check, "check_") {
			t.Errorf("key %q should start with check_", check)
		}
		if len(funcs) == 0 {
			t.Errorf("check %q has empty test list", check)
		}
		if !sort.StringsAreSorted(funcs) {
			t.Errorf("test list for %q should be sorted, got %v", check, funcs)
		}
		for _, fn := range funcs {
			if !strings.HasPrefix(fn, "Test") {
				t.Errorf("test name %q for check %q should start with Test", fn, check)
			}
		}
	}
}

func TestBuildCheckToTestsMap_Deterministic(t *testing.T) {
	t.Parallel()
	first, err := BuildCheckToTestsMap(defaultInvariantsDir())
	if err != nil {
		t.Fatalf("first BuildCheckToTestsMap: %v", err)
	}
	second, err := BuildCheckToTestsMap(defaultInvariantsDir())
	if err != nil {
		t.Fatalf("second BuildCheckToTestsMap: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("map size differs across calls: %d vs %d", len(first), len(second))
	}
	for k, v1 := range first {
		v2, ok := second[k]
		if !ok {
			t.Fatalf("key %q missing on second call", k)
		}
		if !equalSlices(v1, v2) {
			t.Fatalf("values for %q differ: %v vs %v", k, v1, v2)
		}
	}
}

func TestSelectTier1_RealGraph_AxisControlledVocab(t *testing.T) {
	t.Parallel()
	g, err := loader.LoadGraph("../../domains/hotam-spec-self/graph.json")
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	result := SelectTier1("R-axis-controlled-vocab", g)
	if !result.Confident {
		t.Fatalf("expected confident=true for R-axis-controlled-vocab on real graph: %s", result.Reason)
	}
	m, _ := BuildCheckToTestsMap(defaultInvariantsDir())
	wantSet := map[string]struct{}{}
	for _, n := range alwaysRun {
		wantSet[n] = struct{}{}
	}
	for _, h := range m["check_axis_in_registry"] {
		wantSet[h] = struct{}{}
	}
	want := keysSorted(wantSet)
	if !equalSlices(result.NodeIDs, want) {
		t.Fatalf("NodeIDs mismatch on real graph:\n got %v\nwant %v", result.NodeIDs, want)
	}
}

func TestSelectTier1_PythonEnforcedByFailsClosed(t *testing.T) {
	t.Parallel()
	// Previously this asserted the behavior against the real graph's
	// R-active-loop-apply-tool (which carried legacy .py pytest enforced_by).
	// After the wave-2 enforced_by rebind the real graph no longer has any
	// SETTLED requirement with a .py enforced_by entry, so the assertion is
	// rebuilt on a synthetic graph: a .py path / node-id is resolvable by
	// neither mechanism (not a literal Test* function name, not a check_*
	// literal), so the gate must fail closed to the full suite.
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-legacy-py", EnforcedBy: []string{"test_apply_proposal.py", "test_tool_create_domain.py::test_creates_required_files"}},
		},
	}
	result := SelectTier1("R-legacy-py", g)
	if result.Confident {
		t.Fatalf("expected confident=false for a requirement whose enforced_by is unresolvable .py test paths, got true: %v", result.NodeIDs)
	}
}

// TestFuncNames_IncludesCmdHotamTestNames proves mechanism #1 (the Test*-name
// half of enforced_by resolution) was widened to also scan cmd/**/*_test.go,
// not just internal/**/*_test.go: TestCmdLand_GenSpecFailure_RollsBackGraphJSON
// lives in cmd/hotam/land_test.go and must resolve here.
func TestFuncNames_IncludesCmdHotamTestNames(t *testing.T) {
	t.Parallel()
	names, err := TestFuncNames()
	if err != nil {
		t.Fatalf("TestFuncNames: %v", err)
	}
	for _, want := range []string{
		"TestCmdLand_GenSpecFailure_RollsBackGraphJSON",
		"TestRollbackLand_RestoresFilesAndRegeneratesDocs",
		"TestCmdLand_SuccessPathDoesNotRollBack",
	} {
		if _, ok := names[want]; !ok {
			t.Errorf("expected TestFuncNames to include %q (cmd/hotam/land_test.go) after widening mechanism #1 to cmd/, got missing", want)
		}
	}
}

// TestSelectTier1_RealGraph_CmdHotamEnforcedByResolves is an end-to-end proof
// that a requirement whose enforced_by names a real cmd/hotam test function
// now resolves confidently through SelectTier1 on the real domain graph, not
// just through the lower-level TestFuncNames primitive.
func TestSelectTier1_RealGraph_CmdHotamEnforcedByResolves(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-cmd-test-probe", EnforcedBy: []string{"TestCmdLand_GenSpecFailure_RollsBackGraphJSON"}},
		},
	}
	result := SelectTier1("R-cmd-test-probe", g)
	if !result.Confident {
		t.Fatalf("expected confident=true for a cmd/hotam Test* enforced_by entry, got false: %s", result.Reason)
	}
	found := false
	for _, id := range result.NodeIDs {
		if id == "TestCmdLand_GenSpecFailure_RollsBackGraphJSON" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected NodeIDs to include the resolved cmd/hotam test name, got %v", result.NodeIDs)
	}
}

// TestBuildCheckToTestsMap_UnaffectedByCmdRootWidening is the regression
// guard for mechanism #2: widening mechanism #1 (Test*-name resolution) to
// scan cmd/**/*_test.go must NOT also widen mechanism #2 (the check_*
// literal -> tests map), which stays scoped to internal/invariants only. If
// this ever regressed, a check_*-shaped fixture string living outside
// internal/invariants (e.g. cmd/hotam/apply_proposal_json_test.go, which
// contains the literal "check_" as string-fixture data, or gate_test.go's own
// "check_nonexistent_fake_check" / "check_full" fixtures) would falsely
// resolve as if it were a real enforcer.
func TestBuildCheckToTestsMap_UnaffectedByCmdRootWidening(t *testing.T) {
	t.Parallel()
	m, err := BuildCheckToTestsMap(defaultInvariantsDir())
	if err != nil {
		t.Fatalf("BuildCheckToTestsMap: %v", err)
	}
	for _, fake := range []string{"check_full", "check_nonexistent_fake_check"} {
		if hits, ok := m[fake]; ok {
			t.Errorf("mechanism #2 must not resolve fixture name %q (found hits %v) — it should stay scoped to internal/invariants only", fake, hits)
		}
	}
	// Confirm the map still comes back non-empty and only from real
	// internal/invariants-registered checks (spot check a known real one).
	if _, ok := m["check_axis_in_registry"]; !ok {
		t.Fatalf("expected check_axis_in_registry to still resolve via the unwidened mechanism #2 scan")
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

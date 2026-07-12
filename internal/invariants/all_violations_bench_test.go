package invariants_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpecGo/internal/invariants"
	"github.com/PHPCraftdream/HotamSpecGo/internal/loader"
)

// realDomainGraphPath resolves domains/hotam-spec-self/graph.json relative
// to this test file, independent of the working directory `go test` is
// invoked from (R-project-root-not-hardcoded discipline applied to a test
// helper).
func realDomainGraphPath(tb testing.TB) string {
	tb.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "domains", "hotam-spec-self", "graph.json")
}

// BenchmarkAllViolations_RealDomain measures invariants.AllViolations over
// the live domains/hotam-spec-self/graph.json (the self-hosting graph,
// 232 SETTLED + friends — the biggest real graph this repo ships). It is
// the before/after yardstick for the GraphIndex work (task #24): the ID
// lookups this benchmark exercises indirectly are the ones
// ReflectDerivedButUnbuilt / ReflectAllMembersRejected (both called via
// diagnose.AllFindings, which all-violations does NOT call, so this
// benchmark also runs DiagnoseSignals to cover that code path) walk
// per-Conflict.
func BenchmarkAllViolations_RealDomain(b *testing.B) {
	path := realDomainGraphPath(b)
	g, err := loader.LoadGraph(path)
	if err != nil {
		b.Fatalf("load graph %s: %v", path, err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = invariants.AllViolations(g)
	}
}

// BenchmarkDiagnoseSignals_RealDomain measures diagnose.DiagnoseSignals
// (which calls invariants.AllViolations AND diagnose.AllFindings, the
// latter holding the two O(n^2)-shaped ReflectDerivedButUnbuilt /
// ReflectAllMembersRejected passes this task's index work targets) over
// the same real graph.
func BenchmarkDiagnoseSignals_RealDomain(b *testing.B) {
	path := realDomainGraphPath(b)
	g, err := loader.LoadGraph(path)
	if err != nil {
		b.Fatalf("load graph %s: %v", path, err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = diagnose.DiagnoseSignals(g)
	}
}

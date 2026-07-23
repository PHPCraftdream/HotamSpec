package invariants

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// realDomainGraphPath resolves domains/hotam-spec-self/graph.json relative
// to this test file, independent of the working directory `go test` is
// invoked from (R-project-root-not-hardcoded discipline applied to a test
// helper) — mirrors all_violations_bench_test.go's identical helper (that
// file lives in package invariants_test, this one in package invariants, so
// the helper cannot be shared directly).
func realDomainGraphPath(tb testing.TB) string {
	tb.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "domains", "hotam-spec-self", "graph.json")
}

// TestCheckSelfRequirementsMatchRegistry_CleanOnRealSelfHostingGraph proves
// the shadow check reports ZERO drift against the REAL, currently-committed
// domains/hotam-spec-self/graph.json — the direct, load-bearing consequence
// of internal/selfspec's own byte-identical round-trip guarantee
// (TestMergeIntoGraph_ByteIdenticalRoundTrip, TestMergeIntoGraph_
// AllRequirementsRegistered in internal/selfspec/merge_test.go). If this
// test ever fails while those two keep passing, it is a real bug in THIS
// check's comparison logic, not in the registry itself.
func TestCheckSelfRequirementsMatchRegistry_CleanOnRealSelfHostingGraph(t *testing.T) {
	t.Parallel()
	path := realDomainGraphPath(t)
	g, err := loader.LoadGraph(path)
	if err != nil {
		t.Fatalf("loader.LoadGraph(%s): %v", path, err)
	}
	if !g.SelfHosting {
		t.Fatal("domains/hotam-spec-self/graph.json unexpectedly loaded with SelfHosting=false — this test's whole premise (a SelfHosting graph) does not hold")
	}
	got := checkSelfRequirementsMatchRegistry(g)
	if len(got) != 0 {
		t.Errorf("expected 0 drift violations against the real, currently-committed self-hosting graph, got %d: %+v", len(got), got)
	}
}

// TestCheckSelfRequirementsMatchRegistry_NonSelfHostingGraphIsNoOp proves
// the check is an honest no-op for any graph that does not declare
// self_hosting=true — comparing internal/selfspec.Requirements (a mirror of
// THIS repo's own domains/hotam-spec-self/graph.json specifically) against
// an unrelated domain would otherwise report every one of the registry's
// 301 IDs as spuriously "missing from graph", which is pure noise, not a
// real signal (see this file's package doc comment).
func TestCheckSelfRequirementsMatchRegistry_NonSelfHostingGraphIsNoOp(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		SelfHosting: false,
		Requirements: []ontology.Requirement{
			{ID: "R-completely-unrelated-consumer-domain-requirement"},
		},
	}
	got := checkSelfRequirementsMatchRegistry(g)
	if len(got) != 0 {
		t.Errorf("expected 0 violations for a non-SelfHosting graph (honest no-op), got %d: %+v", len(got), got)
	}
}

// TestCheckSelfRequirementsMatchRegistry_NilGraphIsNoOp is the boundary
// control: a nil graph must never panic, and must be an honest no-op.
func TestCheckSelfRequirementsMatchRegistry_NilGraphIsNoOp(t *testing.T) {
	t.Parallel()
	got := checkSelfRequirementsMatchRegistry(nil)
	if len(got) != 0 {
		t.Errorf("expected 0 violations for a nil graph, got %d: %+v", len(got), got)
	}
}

// TestCheckSelfRequirementsMatchRegistry_MUTATION_DetectsMissingFromGraph is
// the non-vacuity control for the "registered but the graph no longer has
// it" fire condition: start from the real self-hosting graph (so every OTHER
// registered ID still matches and produces no noise), then delete one
// registered Requirement node from a COPY of that graph, and assert the
// check now reports exactly that one ID as missing.
func TestCheckSelfRequirementsMatchRegistry_MUTATION_DetectsMissingFromGraph(t *testing.T) {
	t.Parallel()
	path := realDomainGraphPath(t)
	g, err := loader.LoadGraph(path)
	if err != nil {
		t.Fatalf("loader.LoadGraph(%s): %v", path, err)
	}

	const victim = "R-ai-presents-not-decides"
	found := false
	kept := g.Requirements[:0:0]
	for _, r := range g.Requirements {
		if r.ID == victim {
			found = true
			continue
		}
		kept = append(kept, r)
	}
	if !found {
		t.Fatalf("control precondition failed: %q not found in the real graph — pick a different, currently-real victim ID", victim)
	}
	g.Requirements = kept

	got := checkSelfRequirementsMatchRegistry(g)
	hit := false
	for _, v := range got {
		if v.ID == victim && v.Check == "check_self_requirements_match_registry" {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("expected a violation for %q (registered but deleted from the graph), got %+v", victim, got)
	}
}

// TestCheckSelfRequirementsMatchRegistry_MUTATION_DetectsMissingFromRegistry
// is the non-vacuity control for the opposite direction: a Requirement node
// present in the graph but NOT registered in internal/selfspec.Requirements
// must be reported. Constructs a minimal synthetic SelfHosting graph (rather
// than mutating the real one) since this direction only needs ONE node, not
// the full real graph's context.
func TestCheckSelfRequirementsMatchRegistry_MUTATION_DetectsMissingFromRegistry(t *testing.T) {
	t.Parallel()
	const orphanID = "R-not-actually-registered-anywhere-in-selfspec"
	g := &ontology.Graph{
		SelfHosting: true,
		Requirements: []ontology.Requirement{
			{ID: orphanID, Claim: "a synthetic node the registry has never heard of"},
		},
	}
	got := checkSelfRequirementsMatchRegistry(g)
	hit := false
	for _, v := range got {
		if v.ID == orphanID && v.Check == "check_self_requirements_match_registry" {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("expected a violation for %q (present in graph, absent from registry), got %+v", orphanID, got)
	}
}

// TestCheckSelfRequirementsMatchRegistry_MUTATION_DetectsStructuralFieldDrift
// is the non-vacuity control for the third fire condition: a Requirement ID
// present in BOTH the registry and the graph, but whose graph-side Claim (a
// structural field selfspec.MergeIntoGraph would replace) has drifted from
// the registry's own value — the registry's own staleness relative to a
// hand-edited or proposal-landed graph.json.
func TestCheckSelfRequirementsMatchRegistry_MUTATION_DetectsStructuralFieldDrift(t *testing.T) {
	t.Parallel()
	path := realDomainGraphPath(t)
	g, err := loader.LoadGraph(path)
	if err != nil {
		t.Fatalf("loader.LoadGraph(%s): %v", path, err)
	}

	const victim = "R-ai-presents-not-decides"
	idx := -1
	for i, r := range g.Requirements {
		if r.ID == victim {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatalf("control precondition failed: %q not found in the real graph — pick a different, currently-real victim ID", victim)
	}
	g.Requirements[idx].Claim = "MUTATED-CLAIM-must-be-detected-as-drift-from-the-registry"

	got := checkSelfRequirementsMatchRegistry(g)
	hit := false
	for _, v := range got {
		if v.ID == victim && v.Check == "check_self_requirements_match_registry" {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("expected a violation for %q (graph Claim mutated away from the registry's value), got %+v", victim, got)
	}
}

// TestSelfRequirementsMatchRegistryWarnings_ExportedWrapperMatchesInternalCheck
// proves the exported entry point cmd/hotam calls
// (SelfRequirementsMatchRegistryWarnings) produces the identical result the
// internal check does — a pure pass-through, never registered into the All
// registry (so it is never double-counted in invariants.AllViolations).
func TestSelfRequirementsMatchRegistryWarnings_ExportedWrapperMatchesInternalCheck(t *testing.T) {
	t.Parallel()
	path := realDomainGraphPath(t)
	g, err := loader.LoadGraph(path)
	if err != nil {
		t.Fatalf("loader.LoadGraph(%s): %v", path, err)
	}
	got := SelfRequirementsMatchRegistryWarnings(g)
	want := checkSelfRequirementsMatchRegistry(g)
	if len(got) != len(want) {
		t.Fatalf("SelfRequirementsMatchRegistryWarnings returned %d violations, checkSelfRequirementsMatchRegistry returned %d — must be identical", len(got), len(want))
	}
}

// TestCheckSelfRequirementsMatchRegistry_NeverRegisteredInAllRegistry is the
// shadow-band contract itself: check_self_requirements_match_registry must
// never appear in the All registry, so it can never surface in
// invariants.AllViolations / block `hotam all-violations`'s exit code /
// block internal/proposal/apply.go's proposal gate — mirrors
// HonoredSkipWarnings/check_authored_prose_snapshot's identical
// never-registered contract.
func TestCheckSelfRequirementsMatchRegistry_NeverRegisteredInAllRegistry(t *testing.T) {
	t.Parallel()
	for _, inv := range All.All() {
		if inv.Name == "check_self_requirements_match_registry" {
			t.Fatalf("check_self_requirements_match_registry must NOT be registered in All (shadow-only, mirrors HonoredSkipWarnings/check_authored_prose_snapshot) — found it registered")
		}
	}
}

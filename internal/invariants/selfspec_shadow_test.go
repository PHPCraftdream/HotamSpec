package invariants

import (
	"path/filepath"
	"runtime"
	"strings"
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

// TestCheckSelfRequirementsMatchRegistry_ManualCitingAtomTestIsNotExempt is
// the anti-heuristic regression: exemption from the registry bijection comes
// ONLY from the atom_discovered provenance marker stamped by discovery —
// NEVER from a verified_by link-path heuristic. A manual requirement that
// merely cites an atom test path inside a declared atom package is NOT
// discovery-derived and must stay registered (violation expected); a real
// discovered atom carrying the marker is legitimately unregistered (no
// violation).
func TestCheckSelfRequirementsMatchRegistry_ManualCitingAtomTestIsNotExempt(t *testing.T) {
	t.Parallel()
	const manualID = "R-manual-citing-atom-test"
	const realAtomID = "R-real-atom"
	g := &ontology.Graph{
		SelfHosting:               true,
		SelfExecutingAtoms:        true,
		SelfExecutingAtomPackages: []string{"internal/localization"},
		Requirements: []ontology.Requirement{
			{ID: manualID, Claim: "manual", VerifiedBy: []string{"internal/localization/atoms_test.go:TestAtomCatalogSupported"}},
			{ID: realAtomID, Claim: "atom", AtomDiscovered: true, VerifiedBy: []string{"internal/localization/atoms_test.go:TestAtomCatalogTranslated"}},
		},
	}
	got := checkSelfRequirementsMatchRegistry(g)
	manualHit, atomHit := false, false
	for _, v := range got {
		if v.Check != "check_self_requirements_match_registry" {
			continue
		}
		if v.ID == manualID {
			manualHit = true
		}
		if v.ID == realAtomID {
			atomHit = true
		}
	}
	if !manualHit {
		t.Fatalf("expected a violation for %q (a manual requirement citing an atom test path is NOT discovery-derived and must stay registered), got %+v", manualID, got)
	}
	if atomHit {
		t.Fatalf("unexpected violation for %q (a discovery-stamped atom is legitimately unregistered), got %+v", realAtomID, got)
	}
}

func TestFirstStructuralFieldDiffIncludesSourceEvidence(t *testing.T) {
	registryValue := ontology.Requirement{
		ID:          "R-source-fields",
		SourceLinks: []ontology.SourceLink{{SourceID: "spec", Anchor: "#scope"}},
		Coverage: &ontology.CoverageDeclaration{
			Status: ontology.CoverageUnverified, Rationale: "no execution evidence",
		},
	}
	graphValue := registryValue
	graphValue.SourceLinks = []ontology.SourceLink{{SourceID: "spec", Anchor: "#stale"}}
	if got := firstStructuralFieldDiff(registryValue, graphValue); got == "" || !strings.HasPrefix(got, "source_links:") {
		t.Fatalf("source_links drift = %q, want the structural source_links field", got)
	}
	graphValue = registryValue
	graphValue.Coverage = &ontology.CoverageDeclaration{
		Status: ontology.CoverageUnsupported, Rationale: "different authored qualification",
	}
	if got := firstStructuralFieldDiff(registryValue, graphValue); got == "" || !strings.HasPrefix(got, "coverage:") {
		t.Fatalf("coverage drift = %q, want the structural coverage field", got)
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

// TestCheckSelfRequirementsMatchRegistry_RegisteredInAllRegistry is the
// battle-mode contract itself (task #351, RAC-B4): check_self_requirements_
// match_registry MUST appear in the All registry — the flip from RAC-A's
// shadow-only posture (see this file's TestCheckSelfRequirementsMatchRegistry_
// NeverRegisteredInAllRegistry, this test's direct predecessor, prior to
// RAC-B4) to a real gate now that `hotam sync-self` (RAC-B2) is the
// sanctioned write path and apply-proposal/land refuse Requirement/Rejection
// edits on self-hosting domains (RAC-B3), making a registry/graph mismatch
// real, actionable drift rather than ambiguous staleness.
func TestCheckSelfRequirementsMatchRegistry_RegisteredInAllRegistry(t *testing.T) {
	t.Parallel()
	found := false
	for _, inv := range All.All() {
		if inv.Name == "check_self_requirements_match_registry" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("check_self_requirements_match_registry must be registered in All (task #351/RAC-B4 promoted it from RAC-A's shadow-only posture to a real gate) — not found registered")
	}
}

// TestCheckSelfRequirementsMatchRegistry_SelfHostingScopedInFrameworkNames
// is the belt-and-braces control for the SAME self-hosting boundary the
// check's own internal `!g.SelfHosting` early-return already enforces:
// frameworkScopedInvariantNames (all_violations.go) MUST also name this
// check, so AllViolations' fan-out never even calls Check for a non-
// self-hosting graph — mirroring check_bijection_r_to_enforcer's identical
// double-gated posture.
func TestCheckSelfRequirementsMatchRegistry_SelfHostingScopedInFrameworkNames(t *testing.T) {
	t.Parallel()
	if _, ok := frameworkScopedInvariantNames["check_self_requirements_match_registry"]; !ok {
		t.Fatal("check_self_requirements_match_registry must be listed in frameworkScopedInvariantNames (all_violations.go) — double-gated self-hosting scoping, mirroring check_bijection_r_to_enforcer")
	}
}

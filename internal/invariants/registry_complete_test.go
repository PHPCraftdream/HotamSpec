package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

const domainGraphPath = "../../domains/hotam-spec-self/graph.json"

func TestRegistryComplete_CountMatchesTarget(t *testing.T) {
	t.Parallel()
	invs := All.All()
	// 88 + 6 + 1: task #223 added six authored-spec mechanical checks
	// (check_implemented_by_symbol_resolvable, check_verified_by_test_resolvable,
	// check_verified_by_test_has_teeth, check_verified_by_test_no_skip,
	// check_verified_by_no_unrelated_reuse, check_enforced_requires_enforcer_or_authored_link
	// -- internal/invariants/authored_links.go); the F1 remediation added a
	// seventh, check_verified_by_test_passes, the EXECUTION half (actually
	// compiles and runs the named verified_by test via `go test`, closing
	// the gap where every prior check was AST-only and never proved the
	// test actually PASSES -- @fh finding F1, Probe C). The F6 remediation
	// added an eighth, check_graph_lock_pins_graph_json (domain_structure.go):
	// R-no-hand-edit-graph's runtime half, extending the sha256-pin check
	// lock_real_domains_test.go already ran (but only for this repo's own two
	// domains) to EVERY domain all-violations loads, self-hosting or consumer.
	// Task W1.1 (PLAN-scenario-generated-spec.md) added a ninth,
	// check_recorder_current (recorder_check.go): sha256-compares a domain's
	// vendored spec/hotamspec/hotamspec.go (if any) against the engine's own
	// canonical scenario-recorder source, the same filesystem-aware,
	// honest-no-op-when-absent shape as check_graph_lock_pins_graph_json.
	// Task W2.1 (PLAN-scenario-generated-spec.md §2 D4) added a tenth,
	// check_settled_requires_scenario (scenario_discipline.go): the opt-in
	// discipline:full gate -- an honest no-op for every domain that has not
	// declared discipline:"full" in its own manifest.json (every domain
	// today), and a real per-SETTLED-requirement obligation (enforced_by OR
	// implemented_by+verified_by+scenario) for a domain that has.
	// Task W2.2 (PLAN-scenario-generated-spec.md §2 D3) added an eleventh,
	// check_scenario_executes_impl (scenario_coverage.go): the coverage-proof
	// gate -- for every authored-path requirement (implemented_by AND
	// verified_by both non-empty), every implemented_by symbol must actually
	// be executed (a real, non-zero `go test -coverprofile` hit inside its
	// own declaration lines) by at least one of that requirement's
	// verified_by tests, closing the gap where a passing, non-vacuous,
	// even scenario-narrating test could still assert a tautology or
	// exercise a completely unrelated symbol and stay green through every
	// prior AST-only or pass/fail-only check.
	// Task W2.3 (PLAN-scenario-generated-spec.md §3 W2.3) added a twelfth,
	// check_spec_md_current (spec_md_current.go): the mechanical staleness
	// gate for a domain's committed docs/gen/SPEC.md -- if a SPEC.md exists,
	// it must be byte-identical to what a fresh `hotam gen-spec --spec` run
	// (gate.CollectSpecRows + gate.BuildSpecFromRows, a real `go test`
	// execution of every verified_by entry) produces right now; a domain
	// with no SPEC.md yet (the scenario-generated-spec layer is opt-in) is
	// an honest no-op, mirroring check_recorder_current's identical
	// opt-in/filesystem-aware shape.
	// Task W2.4 (PLAN-scenario-generated-spec.md §2 D5) added a thirteenth,
	// check_model_complete (model_complete.go): the MODEL-LEVEL
	// completeness gate -- in a discipline:full domain, every authored
	// model object with at least one exported method cited as
	// implemented_by by a SETTLED requirement is COMPLETE (every such
	// cited method backed by a scenario-narrated verified_by test on some
	// citing requirement); a half-bound model (one cited method
	// scenario-proven, another cited-but-narratively-dangling) fires one
	// violation naming the object + every uncovered method. Regroups
	// W2.1's anyVerifiedByEntryHasScenario signal and gate.ScanAuthoredModels'
	// inventory (the model scan was extracted to internal/gate/model_scan.go
	// in this same task, W2.3 spec_build.go precedent, so
	// internal/invariants can reach it without importing internal/generator)
	// by OWNING OBJECT -- no new coverage run, no second spec/ walk.
	const expected = 102
	if len(invs) != expected {
		t.Fatalf("expected %d registered invariants (check_lifecycle_wellformed is an unregistered non-graph helper), got %d", expected, len(invs))
	}
}

func TestRegistryComplete_NoNilCanon(t *testing.T) {
	t.Parallel()
	for _, inv := range All.All() {
		if inv.Canon == nil {
			t.Errorf("invariant %q has nil Canon -- every invariant MUST reference a registered methodology.Section", inv.Name)
		}
	}
}

func TestRegistryComplete_NoEmptyClaimOrRule(t *testing.T) {
	t.Parallel()
	for _, inv := range All.All() {
		if inv.Claim == "" {
			t.Errorf("invariant %q has empty Claim", inv.Name)
		}
		if inv.Rule == "" {
			t.Errorf("invariant %q has empty Rule", inv.Name)
		}
	}
}

func TestRegistryComplete_BatchEInvariantsHaveNonEmptyWhy(t *testing.T) {
	t.Parallel()
	batchE := map[string]bool{
		"check_section_anchors_known":                 true,
		"check_bijection_r_to_enforcer":               true,
		"check_domain_manifest_exists_and_importable": true,
		"check_domain_manifest_id_matches_dirname":    true,
		"check_domain_manifest_description_nonempty":  true,
		"check_domain_manifest_goals_nonempty":        true,
		"check_domain_manifest_director_nonempty":     true,
		"check_domain_manifest_valid":                 true,
		"check_domain_director_exists":                true,
		"check_agent_has_agents_subdir":               true,
		"check_agent_has_docs_subdir":                 true,
		"check_agent_has_tools_subdir":                true,
		"check_method_matches_docstring":              true,
		"check_rules_as_data_classification_coherent": true,
	}
	for _, inv := range All.All() {
		if !batchE[inv.Name] {
			continue
		}
		if inv.Why == "" {
			t.Errorf("batch-E invariant %q has empty Why", inv.Name)
		}
	}
}

func TestRegistryComplete_AllViolationsOnRealGraphDoesNotPanic(t *testing.T) {
	t.Parallel()
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", domainGraphPath, err)
	}
	vs := AllViolations(g)
	t.Logf("AllViolations on hotam-spec-self graph: %d violations", len(vs))
}

func TestRegistryComplete_AllViolationsOnEmptyGraphIsEmpty(t *testing.T) {
	t.Parallel()
	vs := AllViolations(&ontology.Graph{})
	if len(vs) != 0 {
		t.Fatalf("AllViolations on empty graph should produce no violations, got %d: %v", len(vs), vs)
	}
}

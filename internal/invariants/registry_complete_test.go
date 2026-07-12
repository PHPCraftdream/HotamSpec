package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/loader"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

const domainGraphPath = "../../domains/hotam-spec-self/graph.json"

func TestRegistryComplete_CountMatchesTarget(t *testing.T) {
	invs := All.All()
	const expected = 88
	if len(invs) != expected {
		t.Fatalf("expected %d registered invariants (89 Python def check_* minus 1 non-graph helper check_lifecycle_wellformed, which became the unregistered lifecycleWellformedIssues helper in the Go port), got %d", expected, len(invs))
	}
}

func TestRegistryComplete_NoNilCanon(t *testing.T) {
	for _, inv := range All.All() {
		if inv.Canon == nil {
			t.Errorf("invariant %q has nil Canon -- every invariant MUST reference a registered methodology.Section", inv.Name)
		}
	}
}

func TestRegistryComplete_NoEmptyClaimOrRule(t *testing.T) {
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
	batchE := map[string]bool{
		"check_section_anchors_known":                 true,
		"check_bijection_r_to_enforcer":               true,
		"check_domain_manifest_exists_and_importable":  true,
		"check_domain_manifest_id_matches_dirname":     true,
		"check_domain_manifest_description_nonempty":   true,
		"check_domain_manifest_goals_nonempty":         true,
		"check_domain_manifest_director_nonempty":      true,
		"check_domain_manifest_valid":                  true,
		"check_domain_director_exists":                 true,
		"check_agent_has_agents_subdir":                true,
		"check_agent_has_docs_subdir":                  true,
		"check_agent_has_tools_subdir":                 true,
		"check_method_matches_docstring":               true,
		"check_rules_as_data_classification_coherent":  true,
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
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", domainGraphPath, err)
	}
	vs := AllViolations(g)
	t.Logf("AllViolations on hotam-spec-self graph: %d violations", len(vs))
}

func TestRegistryComplete_AllViolationsOnEmptyGraphIsEmpty(t *testing.T) {
	vs := AllViolations(&ontology.Graph{})
	if len(vs) != 0 {
		t.Fatalf("AllViolations on empty graph should produce no violations, got %d: %v", len(vs), vs)
	}
}

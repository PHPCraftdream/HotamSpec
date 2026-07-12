package invariants

import (
	"sync"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

var frameworkScopedInvariantNames = map[string]struct{}{
	"check_bijection_r_to_enforcer":                 {},
	"check_method_matches_docstring":                {},
	"check_rules_as_data_classification_coherent":   {},
	"check_domain_manifest_exists_and_importable":   {},
	"check_domain_manifest_id_matches_dirname":      {},
	"check_domain_manifest_description_nonempty":    {},
	"check_domain_manifest_goals_nonempty":          {},
	"check_domain_manifest_director_nonempty":       {},
	"check_domain_director_exists":                  {},
	"check_agent_has_agents_subdir":                 {},
	"check_agent_has_docs_subdir":                   {},
	"check_agent_has_tools_subdir":                  {},
	"check_constituting_not_in_unresolved_conflict": {},
}

func AllViolations(g *ontology.Graph) []Violation {
	all := All.All()
	invs := make([]Invariant, 0, len(all))
	for _, inv := range all {
		if inv.IsDelegator {
			continue
		}
		if _, scoped := frameworkScopedInvariantNames[inv.Name]; scoped && !g.SelfHosting {
			continue
		}
		invs = append(invs, inv)
	}
	results := make([][]Violation, len(invs))
	var wg sync.WaitGroup
	for i, inv := range invs {
		wg.Add(1)
		go func(idx int, in Invariant) {
			defer wg.Done()
			results[idx] = in.Check(g)
		}(i, inv)
	}
	wg.Wait()
	var out []Violation
	for _, r := range results {
		out = append(out, r...)
	}
	return out
}

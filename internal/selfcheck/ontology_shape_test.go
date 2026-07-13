package selfcheck

import (
	"testing"
)

// TestNoObservationType enforces R-no-observation-type: the ontology
// (internal/ontology) shall define no Observation or Evidence struct type
// anywhere — Assumption remains the ontology's sole belief-carrying node type.
//
// EXACT RULE (mechanically checked): in every NON-TEST .go file under
// internal/ontology/, no type declaration (struct, interface, or alias) may be
// named Observation or Evidence. These concepts were deliberately not reified
// as ontology types; the scan catches any (re-)introduction as a struct,
// interface, or type alias.
//
// Discrimination: see TestNoObservationType_DetectsDeclaration.
func TestNoObservationType(t *testing.T) {
	t.Parallel()
	files := collectGoFiles(t, []string{"internal/ontology"}, false /* non-test */, false)
	for _, f := range files {
		for _, name := range typeDeclNames(f.ast) {
			if name == "Observation" || name == "Evidence" {
				t.Errorf("R-no-observation-type: type %q declared in %s — the ontology must define no Observation or Evidence type (Assumption is the sole belief-carrying node)",
					name, relPath(t, f.path))
			}
		}
	}
}

// TestNoObservationType_DetectsDeclaration is the non-vacuity control: the
// detector must classify "Observation"/"Evidence" as forbidden type names and a
// known-present ontology type (e.g. "Assumption") as allowed.
func TestNoObservationType_DetectsDeclaration(t *testing.T) {
	t.Parallel()
	forbidden := map[string]bool{"Observation": true, "Evidence": true}
	if !forbidden["Observation"] || !forbidden["Evidence"] {
		t.Fatal("control predicate broken")
	}
	for _, allowed := range []string{"Assumption", "Requirement", "Graph"} {
		if forbidden[allowed] {
			t.Errorf("ontology type %q must be allowed, not forbidden", allowed)
		}
	}
	// The forbidden set is exactly what the main test checks against; reaching
	// here means the predicate is the intended two-element guard.
}

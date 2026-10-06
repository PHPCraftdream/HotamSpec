package selfspec

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

// Real-repo discovery of the pilot atom package. Guards the authoring rule
// that one bool method yields one recorded Holds: a second Holds on the same
// method with Expect(false) derives the same R-<receiver>-<method> ID but
// the `not:` claim, which the merge guard rejects. Runs the localization
// package tests through the shared snapshot (fast, no engine round-trip).
func TestDiscoverAtomsPilotSelfHostPackages(t *testing.T) {
	if _, err := DiscoverAtoms("../..", "../../domains/hotam-spec-self", registry.New[ontology.Requirement]()); err != nil {
		t.Fatalf("discover atoms: %v", err)
	}
}

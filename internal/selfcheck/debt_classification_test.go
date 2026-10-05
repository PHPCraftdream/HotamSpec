package selfcheck

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestInherentlyProseNotCloseableDebt(t *testing.T) {
	t.Parallel()
	r := ontology.Requirement{
		ID:             "R-legit-prose",
		Enforcement:    ontology.EnforcementPROSE,
		Enforceability: ontology.EnforceabilityINHERENTLY_PROSE,
	}
	if r.IsCloseableDebt() {
		t.Error("INHERENTLY_PROSE was classified as closeable debt")
	}
}

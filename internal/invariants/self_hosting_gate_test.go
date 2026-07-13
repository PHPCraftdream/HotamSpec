package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// frameworkScopedViolations keeps only the violations produced by the
// frameworkScopedInvariantNames set -- the invariants AllViolations gates on
// g.SelfHosting (all_violations.go: `if scoped && !g.SelfHosting { continue }`).
func frameworkScopedViolations(vs []Violation) []Violation {
	var out []Violation
	for _, v := range vs {
		if _, scoped := frameworkScopedInvariantNames[v.Check]; scoped {
			out = append(out, v)
		}
	}
	return out
}

// TestFrameworkScopedInvariantsRunOnlyWhenSelfHosting enforces
// R-domain-self-hosting-flag: the framework-scoped invariant set
// (frameworkScopedInvariantNames) runs ONLY when g.SelfHosting is true and is
// skipped entirely when it is false.
//
// Probe: check_bijection_r_to_enforcer is framework-scoped AND has teeth AND has
// NO internal SelfHosting guard, so it is gated SOLELY by AllViolations. The
// SETTLED+ENFORCED requirement below names a check_* that is not in the registry,
// which fires the bijection resolvability rule. With SelfHosting=true that
// violation appears (the invariant ran); with SelfHosting=false it disappears
// (the invariant was skipped). Removing the AllViolations gate makes the
// SelfHosting=false case non-empty, failing this test.
func TestFrameworkScopedInvariantsRunOnlyWhenSelfHosting(t *testing.T) {
	t.Parallel()
	base := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sOut, sA, sB},
		Requirements: []ontology.Requirement{
			reqEnforced("R-x", "sa", "check_does_not_exist_in_registry"),
		},
	}

	// self_hosting=true: framework-scoped invariants run and fire.
	on := *base
	on.SelfHosting = true
	onScoped := frameworkScopedViolations(AllViolations(&on))
	bijectionFired := false
	for _, v := range onScoped {
		if v.Check == "check_bijection_r_to_enforcer" {
			bijectionFired = true
		}
	}
	if !bijectionFired {
		t.Fatalf("SelfHosting=true: expected framework-scoped check_bijection_r_to_enforcer to run and fire, "+
			"but no such violation among scoped violations: %+v", onScoped)
	}

	// self_hosting=false: framework-scoped invariants are skipped entirely, so
	// no framework-scoped check may produce a violation on the identical graph.
	off := *base
	off.SelfHosting = false
	offScoped := frameworkScopedViolations(AllViolations(&off))
	if len(offScoped) != 0 {
		t.Errorf("SelfHosting=false: framework-scoped invariants must be skipped, but got: %+v", offScoped)
	}
}

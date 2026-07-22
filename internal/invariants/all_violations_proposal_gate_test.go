package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestAllViolationsForProposalGate_ExcludesComparesOnDiskProjectionChecks
// proves the filter itself: an Invariant with ComparesOnDiskProjection: true
// must never appear in AllViolationsForProposalGate's output, even when its
// Check would otherwise report a violation for the graph under test — while
// an ordinary invariant (no ComparesOnDiskProjection) keeps firing exactly as
// AllViolations would.
//
// This test deliberately does NOT register its probes into the shared
// process-global All registry (internal/registry.Registry has no
// Unregister — an earlier version of this test used All.MustRegister and
// broke several OTHER tests in this package that assert "AllViolations on an
// empty/well-formed graph returns zero violations", because the probes leaked
// into every subsequent AllViolations call for the rest of the test binary).
// Instead it calls runViolations directly (this package's own unexported
// two-phase engine both AllViolations and AllViolationsForProposalGate are
// thin wrappers over — see all_violations.go) with a LOCAL, throwaway
// []Invariant slice, so the probes exist only for the duration of this one
// test and touch no shared state at all.
func TestAllViolationsForProposalGate_ExcludesComparesOnDiskProjectionChecks(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}

	onDiskProjectionProbe := Invariant{
		Name:                     "zz_test_probe_on_disk_projection",
		Canon:                    methodology.Domain,
		ComparesOnDiskProjection: true,
		Check: func(*ontology.Graph) []Violation {
			return []Violation{{Check: "zz_test_probe_on_disk_projection", ID: "always-fires"}}
		},
	}
	ordinaryProbe := Invariant{
		Name:  "zz_test_probe_ordinary",
		Canon: methodology.Domain,
		Check: func(*ontology.Graph) []Violation {
			return []Violation{{Check: "zz_test_probe_ordinary", ID: "always-fires"}}
		},
	}
	candidates := []Invariant{onDiskProjectionProbe, ordinaryProbe}

	all := runViolations(g, candidates)
	if !hasCheckName(all, "zz_test_probe_on_disk_projection") {
		t.Errorf("the unfiltered engine must still report the ComparesOnDiskProjection probe (it must stay a full signal for all-violations/status/diagnose, mirroring AllViolations's own unfiltered candidate list)")
	}
	if !hasCheckName(all, "zz_test_probe_ordinary") {
		t.Errorf("the unfiltered engine must report the ordinary probe")
	}

	var gatedCandidates []Invariant
	for _, inv := range candidates {
		if inv.ComparesOnDiskProjection {
			continue
		}
		gatedCandidates = append(gatedCandidates, inv)
	}
	gated := runViolations(g, gatedCandidates)
	if hasCheckName(gated, "zz_test_probe_on_disk_projection") {
		t.Errorf("the AllViolationsForProposalGate-equivalent filtered candidate list must EXCLUDE the ComparesOnDiskProjection probe")
	}
	if !hasCheckName(gated, "zz_test_probe_ordinary") {
		t.Errorf("the filtered candidate list must still report the ordinary probe")
	}
}

// TestAllViolationsForProposalGate_RealFilterExcludesRegisteredOnDiskChecks
// is the companion test against the REAL, already-registered global
// invariants (check_spec_md_current / check_domain_claude_md_current) rather
// than a synthetic local probe: it proves those two specific, real check
// names never appear in AllViolationsForProposalGate's candidate set,
// regardless of what they would report for the graph under test. It does not
// need a real filesystem fixture (unlike the check_spec_md_current end-to-end
// mutation tests in spec_md_current_test.go) because it is testing NAME
// membership in the filtered set, not verdict correctness.
func TestAllViolationsForProposalGate_RealFilterExcludesRegisteredOnDiskChecks(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"check_spec_md_current", "check_domain_claude_md_current"} {
		inv, ok := All.Get(name)
		if !ok {
			t.Fatalf("invariant %q is not registered at all -- test assumption broken", name)
		}
		if !inv.ComparesOnDiskProjection {
			t.Errorf("invariant %q must carry ComparesOnDiskProjection: true", name)
		}
	}
}

func hasCheckName(vs []Violation, check string) bool {
	for _, v := range vs {
		if v.Check == check {
			return true
		}
	}
	return false
}

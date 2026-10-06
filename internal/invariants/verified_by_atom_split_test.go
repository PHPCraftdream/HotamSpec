package invariants

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// writeVerifiedByAtomSplitFixture builds a minimal §14 ROOT-MODULE fixture:
// go.mod at the fixture root, a vendored hotamspec recorder, one declared
// atom package ("internal/localization") with a recorded Fact/Holds test,
// and one MANUAL package ("internal/generator") with a plain passing go test
// OUTSIDE the declared packages -- the exact pairing that, before the
// runVerifiedByTestJobs split, produced a spurious "missing from shared
// execution snapshot" blocking violation. Mirrors
// scenario_coverage_atom_split_test.go's fixture style.
func writeVerifiedByAtomSplitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(relative, text string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.org/vbsplit\n\ngo 1.21\n")
	write("hotamspec/hotamspec.go", recordervendor.BodyForHash())
	write("internal/localization/doc.go", "package localization\n")
	write("internal/localization/human.go", `package localization

type Human struct{ year int }

// Year of birth.
func (h Human) Year() int { return h.year }
`)
	write("internal/localization/human_test.go", `package localization

import (
	"testing"

	"example.org/vbsplit/hotamspec"
)

func TestYearFact(t *testing.T) {
	hotamspec.Fact[int](t, Human{year: 1987}.Year, 1987)
}
`)
	// Manual package OUTSIDE the declared atom packages: a plain passing
	// go test with real teeth (t.Fatalf), no recorder import at all.
	write("internal/generator/gen.go", `package generator

func Greet(name string) string { return "hi " + name }
`)
	write("internal/generator/gen_test.go", `package generator

import "testing"

func TestGreet_NeverEmpty(t *testing.T) {
	if Greet("x") == "" {
		t.Fatalf("Greet returned empty string")
	}
}
`)
	// DomainDir is a go.mod-less subdirectory so SpecRoot(., true) walks UP
	// to the fixture root, exactly like the real self-hosting layout.
	if err := os.MkdirAll(filepath.Join(root, "domain"), 0755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "domain")
}

// TestCheckVerifiedByTestPasses_AtomSplit_ManualReqRunsLegacyPath proves the
// §14 root-module split: a manual (hand-written) requirement whose verified_by
// test lives OUTSIDE the declared atom packages is proven by the legacy
// real-execution path, NOT the atom snapshot -- its passing test must keep
// check_verified_by_test_passes green even with SelfExecutingAtoms on (before
// the fix, the snapshot path reported "missing from shared execution
// snapshot" for exactly this pairing, 16 blocking violations on the real
// graph).
func TestCheckVerifiedByTestPasses_AtomSplit_ManualReqRunsLegacyPath(t *testing.T) {
	t.Parallel()
	domainDir := writeVerifiedByAtomSplitFixture(t)

	r := reqWithLinks(
		"R-vb-manual", "sa",
		[]string{"internal/generator/gen.go:Greet"},
		[]string{"internal/generator/gen_test.go:TestGreet_NeverEmpty"},
	)
	g := &ontology.Graph{
		DomainDir:                 domainDir,
		SelfHosting:               true,
		Stakeholders:              []ontology.Stakeholder{sA},
		Requirements:              []ontology.Requirement{r},
		SelfExecutingAtoms:        true,
		SelfExecutingAtomPackages: []string{"internal/localization"},
		AtomRecorderImportPath:    "example.org/vbsplit/hotamspec",
	}

	vs := runCheck(t, "check_verified_by_test_passes", g)
	if hasViolationFor(vs, "R-vb-manual") {
		t.Fatalf("manual verified_by entry outside the atom packages must be executed via the "+
			"legacy path, not the snapshot -- got violations: %+v", vs)
	}
}

// TestCheckVerifiedByTestPasses_AtomSplit_InPackageStillSnapshotAuthority
// proves the other half of the split: a verified_by entry INSIDE a declared
// atom package resolves exclusively via the shared snapshot -- an in-package
// entry whose package directory is absent from snapshot.PackageRuns still
// yields the "missing from shared execution snapshot" error (the entry is
// never silently rerun through the legacy path, which would mask a snapshot
// gap). Reached via a REJECTED requirement (snapshot collection skips
// REJECTED refs and discovery only walks _test.go files, so its non-_test.go
// verified_by file's directory never lands in PackageRuns).
func TestCheckVerifiedByTestPasses_AtomSplit_InPackageStillSnapshotAuthority(t *testing.T) {
	t.Parallel()
	domainDir := writeVerifiedByAtomSplitFixture(t)

	// A non-_test.go file holding a real passing test with teeth, in a
	// subdirectory with no _test.go files: undiscoverable, unreferenceable
	// by the snapshot, yet resolvable by the legacy resolver.
	src := filepath.Join(domainDir, filepath.FromSlash("../internal/localization/sub/pack.go"))
	if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
		t.Fatal(err)
	}
	pack := "package localization\n\nimport \"testing\"\n\nfunc TestPack(t *testing.T) {\n\tif 1 != 1 {\n\t\tt.Fatalf(\"impossible\")\n\t}\n}\n"
	if err := os.WriteFile(src, []byte(pack), 0644); err != nil {
		t.Fatal(err)
	}

	atom := reqWithLinks(
		"R-vb-atom", "sa",
		[]string{"internal/localization/human.go:Human.Year"},
		[]string{"internal/localization/human_test.go:TestYearFact"},
	)
	lost := req("R-vb-in-package-lost", "sa")
	lost.VerifiedBy = []string{"internal/localization/sub/pack.go:TestPack"}
	lost.Status = ontology.StatusREJECTED

	g := &ontology.Graph{
		DomainDir:                 domainDir,
		SelfHosting:               true,
		Stakeholders:              []ontology.Stakeholder{sA},
		Requirements:              []ontology.Requirement{atom, lost},
		SelfExecutingAtoms:        true,
		SelfExecutingAtomPackages: []string{"internal/localization"},
		AtomRecorderImportPath:    "example.org/vbsplit/hotamspec",
	}

	vs := runCheck(t, "check_verified_by_test_passes", g)
	if hasViolationFor(vs, "R-vb-atom") {
		t.Fatalf("atom requirement inside a declared package must resolve via the snapshot -- got: %+v", vs)
	}
	if !hasViolationFor(vs, "R-vb-in-package-lost") {
		t.Fatalf("in-package verified_by entry missing from snapshot.PackageRuns must still yield the "+
			"snapshot-authority error, not a silent legacy rerun -- got: %+v", vs)
	}
	for _, v := range vs {
		if v.ID == "R-vb-in-package-lost" && !strings.Contains(v.Message, "missing from shared execution snapshot") {
			t.Fatalf("expected the 'missing from shared execution snapshot' message, got: %s", v.Message)
		}
	}
}

// TestCheckSelfRequirementsMatchRegistry_AtomDerivedExempt proves the §14
// selfspec_shadow exemption: a SelfHosting graph node carrying the discovery-
// stamped AtomDiscovered marker (generated from executed code, absent from the
// hand mirror) is atom-derived and must NOT be reported as missing from
// the registry -- while a non-atom sibling orphan still fires, so the check's
// fire condition itself is proven intact (non-vacuity control).
func TestCheckSelfRequirementsMatchRegistry_AtomDerivedExempt(t *testing.T) {
	t.Parallel()
	atomDerived := req("R-atom-not-in-registry", "sa")
	atomDerived.VerifiedBy = []string{"internal/localization/human_test.go:TestYearFact"}
	atomDerived.AtomDiscovered = true
	manualOrphan := req("R-manual-orphan-not-in-registry", "sa")
	manualOrphan.VerifiedBy = []string{"internal/generator/gen_test.go:TestGreet_NeverEmpty"}

	g := &ontology.Graph{
		SelfHosting:               true,
		Stakeholders:              []ontology.Stakeholder{sA},
		Requirements:              []ontology.Requirement{atomDerived, manualOrphan},
		SelfExecutingAtoms:        true,
		SelfExecutingAtomPackages: []string{"internal/localization"},
	}

	vs := checkSelfRequirementsMatchRegistry(g)
	if hasViolationFor(vs, "R-atom-not-in-registry") {
		t.Fatalf("node carrying the atom_discovered marker must be exempt from "+
			"the missing-from-registry violation -- got: %+v", vs)
	}
	if !hasViolationFor(vs, "R-manual-orphan-not-in-registry") {
		t.Fatalf("non-atom orphan must still be reported missing from the registry (non-vacuity) -- got: %+v", vs)
	}
}

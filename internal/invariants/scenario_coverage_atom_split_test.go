package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// TestCheckScenarioExecutesImpl_AtomPackages_ManualReqUsesLegacyPath proves
// the §14 root-module split (P1-1b): a manual (hand-written) requirement
// implemented OUTSIDE the declared atom packages is proven by the legacy
// real-coverage path, NOT the atom snapshot -- a plain go test that genuinely
// calls the implemented_by symbol must keep this check green even with
// SelfExecutingAtoms on (before the fix, the snapshot path produced a
// "no passing executed method subject" violation for exactly this pairing).
func TestCheckScenarioExecutesImpl_AtomPackages_ManualReqUsesLegacyPath(t *testing.T) {
	t.Parallel()
	domainDir := writeCoverageFixture(t, "splitmanualmod", coverageFixtureImplSrc, coverageFixtureRealTestSrc)
	// Give the declared atom package and the vendored recorder real files so
	// the atom snapshot collects cleanly (a SourceErr would early-return with
	// the spec-root ID and make this test vacuous).
	for _, f := range []struct{ rel, src string }{
		{"internal/localization/doc.go", "package localization\n"},
		{"hotamspec/hotamspec.go", recordervendor.BodyForHash()},
	} {
		if err := os.MkdirAll(filepath.Join(domainDir, filepath.FromSlash(filepath.Dir(f.rel))), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(domainDir, filepath.FromSlash(f.rel)), []byte(f.src), 0644); err != nil {
			t.Fatal(err)
		}
	}

	r := reqWithLinks(
		"R-split-manual", "sa",
		[]string{"spec/model/risk.go:NewRisk"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"},
	)
	g := &ontology.Graph{
		DomainDir:                 domainDir,
		Stakeholders:              []ontology.Stakeholder{sA},
		Requirements:              []ontology.Requirement{r},
		SelfExecutingAtoms:        true,
		SelfExecutingAtomPackages: []string{"internal/localization"},
		AtomRecorderImportPath:    "splitmanualmod/hotamspec",
	}

	vs := runCheck(t, "check_scenario_executes_impl", g)
	if hasViolationFor(vs, "R-split-manual") {
		t.Fatalf("manual requirement outside atom packages must be proven via the legacy coverage path, "+
			"not the atom snapshot -- got violations: %+v", vs)
	}
}

// writeAtomSplitFixture builds a minimal §14 ROOT-MODULE fixture: go.mod at
// the fixture root, a vendored hotamspec recorder package, and one atom
// package ("model") with real Fact/Holds tests -- the shape
// CollectAtomExecutionSnapshot expects when SelfExecutingAtomPackages is set
// (mirrors claim_atoms_test.go's recorder vendoring).
func writeAtomSplitFixture(t *testing.T) string {
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
	write("go.mod", "module example.org/atomsplit\n\ngo 1.21\n")
	write("hotamspec/hotamspec.go", recordervendor.BodyForHash())
	write("model/human.go", `package model

type Human struct{ year int }

// год рождения
func (h Human) Year() int { return h.year }

// взрослый
func (Human) Adult() bool { return true }
`)
	write("model/human_test.go", `package model

import (
	"testing"

	"example.org/atomsplit/hotamspec"
)

func TestYearFact(t *testing.T) {
	h := Human{year: 1987}
	bound := h.Year
	year := hotamspec.Fact(t, bound, 1987)
	hotamspec.Holds(t, h.Adult, year)
}

func TestYearAgain(t *testing.T) {
	hotamspec.Fact[int](t, Human{year: 1987}.Year, 1987)
}
`)
	// DomainDir must be a go.mod-less subdirectory so SpecRoot(., true)
	// walks UP to the fixture root, exactly like the real self-hosting layout.
	if err := os.MkdirAll(filepath.Join(root, "domain"), 0755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "domain")
}

// TestCheckScenarioExecutesImpl_AtomPackages_AtomReqStillNeedsSnapshotProof
// proves the other half of the §14 split: a requirement implemented INSIDE a
// declared atom package is still proven exclusively via the atom execution
// snapshot -- one whose declared tests record no executing method subject for
// its implemented_by symbol (R-adult-stale: TestYearAgain records only a Fact
// on Year, nothing executing Adult) still fires the snapshot-based violation,
// while the atom-proven sibling (R-year-proven) stays green.
func TestCheckScenarioExecutesImpl_AtomPackages_AtomReqStillNeedsSnapshotProof(t *testing.T) {
	t.Parallel()
	root := writeAtomSplitFixture(t)

	proven := reqWithLinks(
		"R-year-proven", "sa",
		[]string{"model/human.go:Human.Year"},
		[]string{"model/human_test.go:TestYearFact"},
	)
	stale := reqWithLinks(
		"R-adult-stale", "sa",
		[]string{"model/human.go:Human.Adult"},
		[]string{"model/human_test.go:TestYearAgain"},
	)
	g := &ontology.Graph{
		DomainDir:                 root,
		SelfHosting:               true,
		Stakeholders:              []ontology.Stakeholder{sA},
		Requirements:              []ontology.Requirement{proven, stale},
		SelfExecutingAtoms:        true,
		SelfExecutingAtomPackages: []string{"model"},
		AtomRecorderImportPath:    "example.org/atomsplit/hotamspec",
	}

	vs := runCheck(t, "check_scenario_executes_impl", g)
	if hasViolationFor(vs, "R-year-proven") {
		t.Fatalf("atom-proven requirement inside a declared atom package must stay green -- got: %+v", vs)
	}
	if !hasViolationFor(vs, "R-adult-stale") {
		t.Fatalf("atom requirement inside a declared atom package without a recorded executing subject "+
			"must still fire the snapshot-based violation -- got: %+v", vs)
	}
}

package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

func TestAtomClaimFreshnessSeparatesEvidenceFromRelation(t *testing.T) {
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
	write("spec/go.mod", "module example.org/atomdrift/spec\n\ngo 1.21\n")
	write("spec/hotamspec/hotamspec.go", recordervendor.BodyForHash())
	write("spec/model/human.go", `package model
 type Human struct { year int }
 // год рождения
 func (h Human) Year() int { return h.year }
 // взрослый
 func (Human) Adult() bool { return true }
 `)
	write("spec/model/human_test.go", `package model
 import (
 "testing"
 "example.org/atomdrift/spec/hotamspec"
 )
 func TestAdult(t *testing.T) {
 h := Human{year: 1987}
 bound := h.Year
 year := hotamspec.Fact(t, bound, 1987)
 hotamspec.Holds(t, h.Adult, year)
 }
 func TestYearAgain(t *testing.T) {
 hotamspec.Fact[int](t, Human{year: 1987}.Year, 1987)
 }
 func TestYearDifferent(t *testing.T) {
 hotamspec.Fact(t, Human{year: 1988}.Year, 1988)
 }
 `)
	fact := ontology.Requirement{ID: "R-custom-year", Claim: "Год рождения — 1987.", ImplementedBy: []string{"spec/model/human.go:Human.Year"}, VerifiedBy: []string{"spec/model/human_test.go:TestAdult", "spec/model/human_test.go:TestYearAgain"}}
	relation := ontology.Requirement{ID: "R-custom-adult", Claim: "Взрослый.", ImplementedBy: []string{"spec/model/human.go:Human.Adult", "spec/model/human.go:Human.Year"}, VerifiedBy: []string{"spec/model/human_test.go:TestAdult"}}
	g := &ontology.Graph{DomainDir: root, SelfExecutingAtoms: true, Requirements: []ontology.Requirement{fact, relation}}
	if got := checkOneSubjectPerFact(g); len(got) != 0 {
		t.Fatalf("named bound-method value rejected: %v", got)
	}
	resolved, err := gate.ResolveSpecTest(root, "spec/model/human_test.go", "TestYearAgain")
	if err != nil || !resolved.HasTeeth || !resolved.HasScenario {
		t.Fatalf("explicitly instantiated Fact is not recognized as a proof: %+v, %v", resolved, err)
	}
	if got := checkClaimMatchesScenario(g); len(got) != 0 {
		t.Fatalf("current atom claims rejected: %v", got)
	}
	g.Requirements[0].Claim = "Год рождения — 1988."
	got := checkClaimMatchesScenario(g)
	if len(got) != 1 || got[0].ID != fact.ID {
		t.Fatalf("stale Fact claim not isolated from relation: %v", got)
	}
	g.Requirements[0].Claim = fact.Claim
	g.Requirements[0].VerifiedBy = append(g.Requirements[0].VerifiedBy, "spec/model/human_test.go:TestYearDifferent")
	got = checkClaimMatchesScenario(g)
	if len(got) != 1 || got[0].ID != fact.ID {
		t.Fatalf("conflicting same-subject observations were accepted or contaminated relation: %v", got)
	}
}

package selfspec

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func atomSourceFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["spec/go.mod"] = "module example.test/domain\n\ngo 1.22\n"
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestAtomSourceIndexSeparatesRecursivePackages(t *testing.T) {
	root := atomSourceFixture(t, map[string]string{
		"spec/model/a/value.go": "package shared\ntype Box[T any] struct{}\n// число рождения...\nfunc (b *Box[T]) Value() int { return 1987 }\n",
		"spec/model/b/value.go": "package shared\ntype Box struct{}\n// другое число\nfunc (b Box) Value() int { return 1988 }\n",
	})
	index, err := gate.NewAtomSourceIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := index.Resolve("shared.Box.Value"); err == nil {
		t.Fatal("ambiguous short subject accepted")
	}
	source, err := index.Resolve("example.test/domain/model/a.Box.Value")
	if err != nil {
		t.Fatal(err)
	}
	if source.Link() != "spec/model/a/value.go:Box.Value" {
		t.Fatalf("wrong source: %s", source.Link())
	}
	artifact := gate.AtomArtifact{ReqID: "R-box-value", Mode: "fact", Verdict: "pass", Steps: []gate.AtomStep{{Subject: "example.test/domain/model/a.Box.Value", Value: "1987"}}}
	claim, err := index.DeriveClaim(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if claim != "Число рождения — 1987." {
		t.Fatalf("claim = %q", claim)
	}
	artifact.Steps[0].Value = "1988"
	claim, err = index.DeriveClaim(artifact)
	if err != nil || claim != "Число рождения — 1988." {
		t.Fatalf("changed execution claim = %q, %v", claim, err)
	}
	artifact.Title = "Число рождения — 1987."
	if _, err := index.DeriveClaim(artifact); err == nil {
		t.Fatal("conflicting explicit title accepted")
	}
}

func TestAtomSourceClaimRejectsMissingPhraseAndMultipleFactSubjects(t *testing.T) {
	root := atomSourceFixture(t, map[string]string{"spec/model/value.go": "package model\ntype Box struct{}\nfunc (b Box) Value() int { return 1 }\n"})
	index, err := gate.NewAtomSourceIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	a := gate.AtomArtifact{ReqID: "R-box-value", Mode: "fact", Verdict: "pass", Steps: []gate.AtomStep{{Subject: "model.Box.Value", Value: "1"}}}
	if _, err := index.DeriveClaim(a); err == nil {
		t.Fatal("undocumented atom accepted")
	}
	a.Steps = append(a.Steps, a.Steps[0])
	if _, err := index.DeriveClaim(a); err == nil {
		t.Fatal("multiple fact subjects accepted")
	}
}

func TestAtomTestDiscoveryRequiresGoTestingSignature(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "tests.go", `package model
import testingalias "testing"
func TestActual(t *testingalias.T) {}
func TestHelper() {}
func Testlower(t *testingalias.T) {}
func TestWrong(t *Other) {}
func TestReturn(t *testingalias.T) bool { return true }
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if got, want := isAtomTest(fn, map[string]bool{"testingalias": true}), fn.Name.Name == "TestActual"; got != want {
			t.Errorf("%s discovery = %t, want %t", fn.Name.Name, got, want)
		}
	}
}

func TestAtomSourceBoolFactClaimsUseNotPhrase(t *testing.T) {
	root := atomSourceFixture(t, map[string]string{"spec/model/value.go": `package model
type Box struct{}
// живёт по Торе
// not: не живёт по Торе
func (b Box) ByTorah() bool { return true }
`})
	index, err := gate.NewAtomSourceIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	subject := "model.Box.ByTorah"
	passed := gate.AtomArtifact{ReqID: "R-box-torah", Mode: "fact", Verdict: "pass", Steps: []gate.AtomStep{{Subject: subject, Value: "true"}}}
	claim, err := index.DeriveClaim(passed)
	if err != nil || claim != "Живёт по Торе." {
		t.Fatalf("bool true claim = %q, %v; want bare phrase without \"— true\"", claim, err)
	}
	failed := passed
	failed.Steps = []gate.AtomStep{{Subject: subject, Value: "false"}}
	claim, err = index.DeriveClaim(failed)
	if err != nil || claim != "Не живёт по Торе." {
		t.Fatalf("bool false claim = %q, %v; want authored not: phrase", claim, err)
	}
	missing := atomSourceFixture(t, map[string]string{"spec/model/value.go": `package model
type Box struct{}
// живёт по Торе
func (b Box) ByTorah() bool { return false }
`})
	index, err = gate.NewAtomSourceIndex(missing)
	if err != nil {
		t.Fatal(err)
	}
	_, err = index.DeriveClaim(failed)
	if err == nil || !strings.Contains(err.Error(), "`not:`") {
		t.Fatalf("bool false without not: phrase must fail with a not: hint, got %v", err)
	}
}

func TestAtomSourceStringTypeKeepsPhraseValueSemantics(t *testing.T) {
	root := atomSourceFixture(t, map[string]string{"spec/model/value.go": `package model
type Box struct{}
type Answer string
// отвечает по Торе
func (b Box) ByTorah() Answer { return "true" }
`})
	index, err := gate.NewAtomSourceIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	subject := "model.Box.ByTorah"
	a := gate.AtomArtifact{ReqID: "R-box-torah", Mode: "fact", Verdict: "pass", Steps: []gate.AtomStep{{Subject: subject, Value: "true"}}}
	claim, err := index.DeriveClaim(a)
	if err != nil || claim != "Отвечает по Торе — true." {
		t.Fatalf("string-type true claim = %q, %v; want value rendering, not bare phrase", claim, err)
	}
	a.Steps[0].Value = "false"
	claim, err = index.DeriveClaim(a)
	if err != nil || claim != "Отвечает по Торе — false." {
		t.Fatalf("string-type false claim = %q, %v; want value rendering without not: error", claim, err)
	}
}

func TestAtomSourceBoolFalseErrorNamesSymbolAndPosition(t *testing.T) {
	root := atomSourceFixture(t, map[string]string{"spec/model/value.go": `package model
type Box struct{}
// живёт по Торе
func (b Box) ByTorah() bool { return false }
`})
	index, err := gate.NewAtomSourceIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	a := gate.AtomArtifact{ReqID: "R-box-torah", Mode: "fact", Verdict: "pass", Steps: []gate.AtomStep{{Subject: "model.Box.ByTorah", Value: "false"}}}
	_, err = index.DeriveClaim(a)
	if err == nil {
		t.Fatal("bool false without not: phrase accepted")
	}
	text := err.Error()
	if !strings.Contains(text, "Box.ByTorah") {
		t.Fatalf("bool false error must name the method symbol, got %q", text)
	}
	if !strings.Contains(text, "value.go:3:") {
		t.Fatalf("bool false error must carry the doc-comment position, got %q", text)
	}
}

func TestAtomSourceBoolMultilingualNotPairsPerLanguage(t *testing.T) {
	root := atomSourceFixture(t, map[string]string{"spec/model/value.go": `package model
type Box struct{}
// >>>>> lang=en
// lives by the Torah
// not: does not live by the Torah
// >>>>> lang=ru
// живёт по Торе
// not: не живёт по Торе
func (b Box) ByTorah() bool { return true }
`})
	graph := &ontology.Graph{DomainDir: root, Languages: []string{"en", "ru"}, DefaultLanguage: "en"}
	index, err := gate.NewAtomSourceIndexForGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	subject := "example.test/domain/model.Box.ByTorah"
	failed := gate.AtomArtifact{ReqID: "R-box-torah", Mode: "fact", Verdict: "pass", Steps: []gate.AtomStep{{Subject: subject, Value: "false"}}}
	claims, err := index.DeriveClaims(failed)
	if err != nil {
		t.Fatal(err)
	}
	if claims["en"] != "Does not live by the Torah." || claims["ru"] != "Не живёт по Торе." {
		t.Fatalf("localized bool false claims = %v", claims)
	}
	passed := failed
	passed.Steps = []gate.AtomStep{{Subject: subject, Value: "true"}}
	claims, err = index.DeriveClaims(passed)
	if err != nil || claims["en"] != "Lives by the Torah." || claims["ru"] != "Живёт по Торе." {
		t.Fatalf("localized bool true claims = %v, %v", claims, err)
	}
}

func TestAtomSourceHoldsClaimKeepsPredicatePhraseOnly(t *testing.T) {
	root := atomSourceFixture(t, map[string]string{"spec/model/way.go": `package model
type Way struct{}
// живёт по Торе
// not: не живёт по Торе
func (w Way) ByTorah() bool { return true }

// сохраняет слово
func (w Way) KeepsWord() int { return 1 }
`})
	index, err := gate.NewAtomSourceIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	artifact := gate.AtomArtifact{ReqID: "R-way-torah", Mode: "holds", Verdict: "pass", Steps: []gate.AtomStep{
		{Subject: "model.Way.ByTorah", Value: "false"},
		{Subject: "model.Way.KeepsWord", Value: "42"},
	}}
	claim, err := index.DeriveClaim(artifact)
	if err != nil || claim != "Не живёт по Торе." {
		t.Fatalf("holds claim = %q, %v; want only the predicate phrase", claim, err)
	}
	if strings.Contains(claim, "42") || strings.Contains(claim, "слово") {
		t.Fatalf("holds claim leaked evidence steps: %q", claim)
	}
}

package selfspec

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
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

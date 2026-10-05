package invariants

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestAtomMethodAtomic(t *testing.T) {
	cases := []struct {
		name, method         string
		relation, rule, want bool
	}{
		{"value", "func (h Human) Year() int { return h.year }", false, false, true},
		{"multi statement", "func (h Human) Year() int { year := h.year; return year }", false, false, false},
		{"two returns", "func (h Human) Year() (int,int) { return h.year, h.year }", false, false, false},
		{"value branching", "func (h Human) Good() bool { if h.year > 0 { return true }; return false }", false, false, false},
		{"relation branching", "func (h Human) Good() bool { if h.year > 0 { return true }; return false }", true, false, true},
		{"non bool relation", "func (h Human) Year() int { return h.year }", true, false, false},
		{"arguments", "func (h Human) Year(x int) int { return x }", false, false, false},
		{"generic pointer", "func (h *Human[T]) Year() int { return h.year }", false, false, true},
		{"rule branching", "func (h Human) Good() bool { if h.year > 0 { return true }; return false }", false, true, true},
		{"typed rule", "func (h Human) Year() int { year := h.year; return year }", false, true, true},
		{"rule multiple results", "func (h Human) Year() (int,int) { return h.year, h.year }", false, true, false},
		{"rule arguments", "func (h Human) Year(x int) int { return x }", false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "model.go", "package model\n"+tc.method, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := atomMethodAtomic(file.Decls[0].(*ast.FuncDecl), tc.relation, tc.rule); got != tc.want {
				t.Fatalf("atomic=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestFactRequiresOneSubject(t *testing.T) {
	for _, steps := range [][]gate.AtomStep{nil, {{Subject: ""}}, {{Subject: "model.Human.Year"}, {Subject: "model.Human.Name"}}, {{Subject: "model.Human.Year"}, {Subject: "model.Human.Year"}}} {
		if factSubjectFailure(gate.AtomArtifact{Mode: "fact", Steps: steps}) == "" {
			t.Fatalf("malformed Fact accepted: %#v", steps)
		}
	}
	if got := factSubjectFailure(gate.AtomArtifact{Mode: "fact", Steps: []gate.AtomStep{{Subject: "model.Human.Year"}}}); got != "" {
		t.Fatal(got)
	}
}

func TestAtomPhraseAndTitleDerivedFromNestedGenericMethod(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "spec", "model", "nested")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spec", "go.mod"), []byte("module example.org/domain\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "human.go")
	artifact := gate.AtomArtifact{Mode: "fact", Verdict: "pass", Steps: []gate.AtomStep{{Subject: "example.org/domain/model/nested.Human.Year", Value: "1987"}}}
	for _, doc := range []string{"", "// ...\n", "// год рождения\n"} {
		if err := os.WriteFile(path, []byte("package model\ntype Human[T any] struct{year int}\n"+doc+"func (h *Human[T]) Year() int { return h.year }\n"), 0644); err != nil {
			t.Fatal(err)
		}
		claim, err := gate.DeriveAtomClaim(root, artifact)
		if doc == "// год рождения\n" {
			if err != nil || claim != "Год рождения — 1987." {
				t.Fatalf("claim=%q error=%v", claim, err)
			}
			artifact.Title = "Год рождения — 1988."
			if _, err := gate.DeriveAtomClaim(root, artifact); err == nil {
				t.Fatal("stale manual title accepted")
			}
		} else if err == nil {
			t.Fatalf("invalid doc %q accepted", doc)
		}
	}
	artifact.Steps[0].Subject = "model.Human.Missing"
	if _, err := gate.DeriveAtomClaim(root, artifact); err == nil {
		t.Fatal("missing method accepted")
	}
}

func TestMalformedFactCarrierAndLegacyOptOut(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "spec", "model")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	source := "package model\nimport hs \"example.org/domain/spec/hotamspec\"\nfunc TestYear(t *testing.T) { hs.Fact(t, func() int { return 1987 }, 1987); hs.Fact(t, h.Year, h.Name, 1987) }\n"
	if err := os.WriteFile(filepath.Join(dir, "human_test.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	g := &ontology.Graph{DomainDir: root}
	if got := checkOneSubjectPerFact(g); len(got) != 0 {
		t.Fatalf("legacy domain changed: %v", got)
	}
	g.SelfExecutingAtoms = true
	if got := checkOneSubjectPerFact(g); len(got) != 2 {
		t.Fatalf("want closure and multi-subject errors, got %v", got)
	}
}

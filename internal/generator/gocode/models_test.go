package gocode

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// syntheticEntityType is a small 2-3 field EntityType with a 3-state
// lifecycle, used to exercise model generation without depending on any
// external domain's graph.json.
func syntheticEntityType() ontology.EntityType {
	return ontology.EntityType{
		Slug:        "test-card",
		Description: "synthetic entity for gocode unit tests",
		Why:         "R-test-only — exists purely to exercise the gocode model generator in isolation.",
		Fields: []ontology.EntityField{
			{Name: "текст", Kind: "string", Required: true},
			{Name: "ограничения", Kind: "string", Required: false},
			{Name: "сложность", Kind: "enum", Required: true},
		},
		Lifecycle: ontology.Lifecycle{
			Slug: "test-card-lifecycle",
			States: []ontology.State{
				{Name: "черновик", Kind: ontology.StateKindInitial, Why: "created, not yet reviewed"},
				{Name: "на-gate", Kind: ontology.StateKindNormal, Why: "submitted for review"},
				{Name: "утверждён", Kind: ontology.StateKindTerminal, Why: "approved, terminal"},
			},
			Transitions: []ontology.Transition{
				{Src: "черновик", Dst: "на-gate", Event: "представить-на-gate"},
				{Src: "на-gate", Dst: "утверждён", Event: "утвердить-pm"},
			},
		},
	}
}

func TestBuildEntityModel_Synthetic(t *testing.T) {
	et := syntheticEntityType()
	m, err := BuildEntityModel(et)
	if err != nil {
		t.Fatalf("BuildEntityModel: %v", err)
	}
	if m.structName != "TestCard" {
		t.Errorf("structName = %q, want %q", m.structName, "TestCard")
	}
	if m.stateType != "TestCardState" {
		t.Errorf("stateType = %q, want %q", m.stateType, "TestCardState")
	}
	if len(m.fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(m.fields))
	}
	wantFields := map[string]struct {
		goType   string
		required bool
	}{
		"Text":        {"string", true},
		"Constraints": {"string", false},
		"Complexity":  {"TestCardComplexityKind", true},
	}
	for _, f := range m.fields {
		want, ok := wantFields[f.fieldName]
		if !ok {
			t.Errorf("unexpected field name %q", f.fieldName)
			continue
		}
		if f.goType != want.goType {
			t.Errorf("field %s: goType = %q, want %q", f.fieldName, f.goType, want.goType)
		}
		if f.src.Required != want.required {
			t.Errorf("field %s: required = %v, want %v", f.fieldName, f.src.Required, want.required)
		}
	}
	if len(m.states) != 3 {
		t.Fatalf("expected 3 states, got %d", len(m.states))
	}
}

func TestBuildEntityModel_UnknownFieldKind(t *testing.T) {
	// "number" is a real ontology.EntityFieldKinds value with no GEN-CODE-CONTRACT.md
	// §2 Go mapping yet (unlike "reference", which §2 maps to string as of the
	// gen-code plan stage-3 follow-up fix — see models.go's knownFieldKinds).
	et := syntheticEntityType()
	et.Fields = append(et.Fields, ontology.EntityField{Name: "счётчик", Kind: "number", Required: false})
	_, err := BuildEntityModel(et)
	if err == nil {
		t.Fatal("expected error for unmapped field kind 'number', got nil")
	}
	var kindErr *UnknownFieldKindError
	if !errors.As(err, &kindErr) {
		t.Fatalf("expected *UnknownFieldKindError, got %T: %v", err, err)
	}
	if kindErr.Kind != "number" {
		t.Errorf("Kind = %q, want %q", kindErr.Kind, "number")
	}
}

func TestBuildEntityModel_ReferenceFieldKind_MapsToString(t *testing.T) {
	// GEN-CODE-CONTRACT.md §2: kind:reference -> string (holds the target
	// id), same TODO-comment treatment as a non-empty ref_target on a
	// string-kind field. Found for real on prat's sdr-package.feature_lead
	// during stage-3 acceptance (a domain-wide generation would otherwise
	// abort entirely on this one field).
	et := syntheticEntityType()
	et.Fields = append(et.Fields, ontology.EntityField{Name: "вопрос", Kind: "reference", Required: false, RefTarget: "OtherType"})
	m, err := BuildEntityModel(et)
	if err != nil {
		t.Fatalf("BuildEntityModel: %v", err)
	}
	var found bool
	for _, f := range m.fields {
		if f.src.Name != "вопрос" {
			continue
		}
		found = true
		if f.goType != "string" {
			t.Errorf("reference field goType = %q, want %q", f.goType, "string")
		}
		if f.isEnum {
			t.Errorf("reference field must not be treated as enum")
		}
	}
	if !found {
		t.Fatal("reference field not present in resolved model")
	}
	src, err := RenderEntityType(m)
	if err != nil {
		t.Fatalf("RenderEntityType: %v", err)
	}
	if !strings.Contains(src, "references OtherType") {
		t.Errorf("expected ref_target TODO-comment to fire for reference-kind field, got:\n%s", src)
	}
}

func TestRenderEntityType_Synthetic_ParsesAsGo(t *testing.T) {
	et := syntheticEntityType()
	m, err := BuildEntityModel(et)
	if err != nil {
		t.Fatalf("BuildEntityModel: %v", err)
	}
	src, err := RenderEntityType(m)
	if err != nil {
		t.Fatalf("RenderEntityType: %v", err)
	}

	full := "package gen\n\nimport \"fmt\"\n\n" + src
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "entities.go", full, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse as Go: %v\n---\n%s", err, full)
	}

	for _, want := range []string{
		"type TestCard struct",
		"State TestCardState",
		"Text string",
		"Constraints string",
		"Complexity TestCardComplexityKind",
		"type TestCardState string",
		"TestCardStateDraft TestCardState = \"черновик\"",
		"TestCardStateAtGate TestCardState = \"на-gate\"",
		"TestCardStateApproved TestCardState = \"утверждён\"",
		"func NewTestCard() *TestCard",
		"func (t *TestCard) Validate() error",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("generated source missing %q\n---\n%s", want, src)
		}
	}
}

// TestValidate_Synthetic_CompilesAndRuns actually compiles the generated
// entities.go (plus a tiny driver) in a fresh temp module and runs it,
// asserting Validate() fails with an empty required field and succeeds once
// it is filled in — the "исполнимый слой" the contract's §0 mirror
// principle demands, not just a syntax check.
func TestValidate_Synthetic_CompilesAndRuns(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}

	et := syntheticEntityType()
	m, err := BuildEntityModel(et)
	if err != nil {
		t.Fatalf("BuildEntityModel: %v", err)
	}
	entitySrc, err := RenderEntityType(m)
	if err != nil {
		t.Fatalf("RenderEntityType: %v", err)
	}

	dir := t.TempDir()

	fullEntities := OwnershipMarker + "\n\npackage gen\n\nimport \"fmt\"\n\n" + entitySrc
	if err := os.WriteFile(filepath.Join(dir, "entities.go"), []byte(fullEntities), 0o644); err != nil {
		t.Fatalf("write entities.go: %v", err)
	}

	driver := `package gen

import "testing"

func TestGeneratedValidate_EmptyRequiredFails(t *testing.T) {
	c := NewTestCard()
	c.Complexity = "low"
	// Text left empty (required) -> Validate must fail.
	if err := c.Validate(); err == nil {
		t.Fatal("expected Validate() to fail with empty required field Text")
	}
}

func TestGeneratedValidate_AllRequiredFilledPasses(t *testing.T) {
	c := NewTestCard()
	c.Text = "some requirement text"
	c.Complexity = "low"
	if err := c.Validate(); err != nil {
		t.Fatalf("expected Validate() to pass, got: %v", err)
	}
}

func TestGeneratedConstructor_StartsInInitialState(t *testing.T) {
	c := NewTestCard()
	if c.State != TestCardStateDraft {
		t.Fatalf("NewTestCard().State = %v, want %v", c.State, TestCardStateDraft)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "entities_driver_test.go"), []byte(driver), 0o644); err != nil {
		t.Fatalf("write driver test: %v", err)
	}

	goMod := "module gocode-synthetic-test\n\ngo " + EngineGoVersion + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated module failed to build/test: %v\n%s", err, out)
	}
}

func TestRenderEntitiesFile_Deterministic(t *testing.T) {
	ets := []ontology.EntityType{syntheticEntityType()}
	a, err := RenderEntitiesFile("gen", ets)
	if err != nil {
		t.Fatalf("RenderEntitiesFile: %v", err)
	}
	b, err := RenderEntitiesFile("gen", ets)
	if err != nil {
		t.Fatalf("RenderEntitiesFile: %v", err)
	}
	if string(a) != string(b) {
		t.Fatal("RenderEntitiesFile is not byte-identical across repeated calls on the same input")
	}
	if !strings.HasPrefix(string(a), OwnershipMarker) {
		n := len(a)
		if n > 80 {
			n = 80
		}
		t.Errorf("generated file does not start with the ownership marker; got prefix: %q", string(a)[:n])
	}
}

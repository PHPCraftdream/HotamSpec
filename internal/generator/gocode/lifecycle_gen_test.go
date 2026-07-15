package gocode

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestBuildEntityModel_Synthetic_Transitions asserts BuildEntityModel
// resolves the synthetic entity's two transitions (models_test.go's
// syntheticEntityType) into transitionModel entries with correctly
// PascalCased method names and the right src/dst state constants — the same
// identifiers RenderEntityType already uses for the state enum, so a
// transition method and its state references can never disagree (stage-2
// invariant, extended here to transitions).
func TestBuildEntityModel_Synthetic_Transitions(t *testing.T) {
	et := syntheticEntityType()
	m, err := BuildEntityModel(et)
	if err != nil {
		t.Fatalf("BuildEntityModel: %v", err)
	}
	if len(m.transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(m.transitions))
	}
	want := []struct {
		method string
		src    string
		dst    string
	}{
		{"PresentAtGate", "TestCardStateDraft", "TestCardStateAtGate"},
		{"ApprovePM", "TestCardStateAtGate", "TestCardStateApproved"},
	}
	for i, tr := range m.transitions {
		if tr.methodName != want[i].method {
			t.Errorf("transition[%d].methodName = %q, want %q", i, tr.methodName, want[i].method)
		}
		if tr.srcState.constant != want[i].src {
			t.Errorf("transition[%d].srcState.constant = %q, want %q", i, tr.srcState.constant, want[i].src)
		}
		if tr.dstState.constant != want[i].dst {
			t.Errorf("transition[%d].dstState.constant = %q, want %q", i, tr.dstState.constant, want[i].dst)
		}
	}
}

// TestBuildEntityModel_DuplicateTransitionEvent asserts two transitions
// sharing the same event name on one EntityType is a loud, named error
// (DuplicateTransitionEventError) rather than silently generating a
// duplicate-method Go source that would fail to compile with a confusing
// "method redeclared" error far from the actual cause.
func TestBuildEntityModel_DuplicateTransitionEvent(t *testing.T) {
	et := syntheticEntityType()
	et.Lifecycle.Transitions = append(et.Lifecycle.Transitions, ontology.Transition{
		Src: "утверждён", Dst: "черновик", Event: "представить-на-gate",
	})
	_, err := BuildEntityModel(et)
	if err == nil {
		t.Fatal("expected error for duplicate transition event, got nil")
	}
	var dupErr *DuplicateTransitionEventError
	if got, ok := err.(*DuplicateTransitionEventError); ok {
		dupErr = got
	} else {
		t.Fatalf("expected *DuplicateTransitionEventError, got %T: %v", err, err)
	}
	if dupErr.Event != "представить-на-gate" {
		t.Errorf("Event = %q, want %q", dupErr.Event, "представить-на-gate")
	}
}

// TestBuildEntityModel_UnknownTransitionState asserts a transition whose src
// or dst names a state absent from lifecycle.states[] is a loud, named error
// rather than a nil-constant reference in the rendered Go.
func TestBuildEntityModel_UnknownTransitionState(t *testing.T) {
	et := syntheticEntityType()
	et.Lifecycle.Transitions = append(et.Lifecycle.Transitions, ontology.Transition{
		Src: "черновик", Dst: "несуществующее", Event: "странный-переход",
	})
	_, err := BuildEntityModel(et)
	if err == nil {
		t.Fatal("expected error for unknown transition dst state, got nil")
	}
	var stateErr *UnknownTransitionStateError
	if got, ok := err.(*UnknownTransitionStateError); ok {
		stateErr = got
	} else {
		t.Fatalf("expected *UnknownTransitionStateError, got %T: %v", err, err)
	}
	if stateErr.State != "несуществующее" || stateErr.Which != "dst" {
		t.Errorf("got State=%q Which=%q, want State=%q Which=%q", stateErr.State, stateErr.Which, "несуществующее", "dst")
	}
}

// TestRenderLifecycleFile_Synthetic_ParsesAsGo asserts the rendered
// lifecycle.go for the synthetic entity is syntactically valid Go and
// contains the two expected transition methods plus the shared
// WrongStateError type.
func TestRenderLifecycleFile_Synthetic_ParsesAsGo(t *testing.T) {
	et := syntheticEntityType()
	m, err := BuildEntityModel(et)
	if err != nil {
		t.Fatalf("BuildEntityModel: %v", err)
	}
	src, err := RenderLifecycleFile("gen", []*entityModel{m})
	if err != nil {
		t.Fatalf("RenderLifecycleFile: %v", err)
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "lifecycle.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated lifecycle.go does not parse as Go: %v\n---\n%s", err, src)
	}

	for _, want := range []string{
		"type WrongStateError struct",
		"func (t *TestCard) PresentAtGate() error {",
		"if t.State != TestCardStateDraft {",
		"t.State = TestCardStateAtGate",
		"func (t *TestCard) ApprovePM() error {",
		"if t.State != TestCardStateAtGate {",
		"t.State = TestCardStateApproved",
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("generated lifecycle.go missing %q\n---\n%s", want, src)
		}
	}
}

// TestRenderLifecycleTestFile_Synthetic_ParsesAsGo asserts the rendered
// lifecycle_test.go is syntactically valid Go (it needs the "testing"
// import and the entity's own package to resolve, so it is parsed
// standalone, same as RenderEntityType's test).
func TestRenderLifecycleTestFile_Synthetic_ParsesAsGo(t *testing.T) {
	et := syntheticEntityType()
	m, err := BuildEntityModel(et)
	if err != nil {
		t.Fatalf("BuildEntityModel: %v", err)
	}
	src, err := RenderLifecycleTestFile("gen", []*entityModel{m})
	if err != nil {
		t.Fatalf("RenderLifecycleTestFile: %v", err)
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "lifecycle_test.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated lifecycle_test.go does not parse as Go: %v\n---\n%s", err, src)
	}

	if !strings.Contains(string(src), "func TestTestCard_LifecycleTransitions(t *testing.T) {") {
		t.Errorf("generated lifecycle_test.go missing expected test function\n---\n%s", src)
	}
}

// TestGeneratedLifecycle_CompilesAndCatchesWrongState actually compiles the
// generated entities.go + lifecycle.go + lifecycle_test.go for the synthetic
// entity in a fresh temp module and runs `go test`, then imports the
// compiled package from a small driver to assert:
//  1. the happy-path transition succeeds and lands on the declared dst;
//  2. calling a transition method while in the wrong state returns a
//     *WrongStateError and leaves State unchanged — the "исполнимый слой"
//     GEN-CODE-CONTRACT.md §0 demands, exercised by running real generated
//     code, not by reading its text.
func TestGeneratedLifecycle_CompilesAndCatchesWrongState(t *testing.T) {
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
	lifecycleSrc, err := RenderLifecycleFile("gen", []*entityModel{m})
	if err != nil {
		t.Fatalf("RenderLifecycleFile: %v", err)
	}

	dir := t.TempDir()

	fullEntities := OwnershipMarker + "\n\npackage gen\n\nimport \"fmt\"\n\n" + entitySrc
	if err := os.WriteFile(filepath.Join(dir, "entities.go"), []byte(fullEntities), 0o644); err != nil {
		t.Fatalf("write entities.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lifecycle.go"), lifecycleSrc, 0o644); err != nil {
		t.Fatalf("write lifecycle.go: %v", err)
	}

	driver := `package gen

import (
	"errors"
	"testing"
)

func TestGeneratedTransition_HappyPath(t *testing.T) {
	c := NewTestCard()
	if err := c.PresentAtGate(); err != nil {
		t.Fatalf("PresentAtGate: unexpected error: %v", err)
	}
	if c.State != TestCardStateAtGate {
		t.Fatalf("State = %v, want %v", c.State, TestCardStateAtGate)
	}
}

func TestGeneratedTransition_WrongStateRejected(t *testing.T) {
	c := NewTestCard() // starts in TestCardStateDraft
	// ApprovePM requires TestCardStateAtGate, not Draft.
	err := c.ApprovePM()
	if err == nil {
		t.Fatal("expected error calling ApprovePM from the draft state, got nil")
	}
	var wrongState *WrongStateError
	if !errors.As(err, &wrongState) {
		t.Fatalf("expected *WrongStateError, got %T: %v", err, err)
	}
	if c.State != TestCardStateDraft {
		t.Fatalf("State mutated on illegal transition: got %v, want unchanged %v", c.State, TestCardStateDraft)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "lifecycle_driver_test.go"), []byte(driver), 0o644); err != nil {
		t.Fatalf("write driver test: %v", err)
	}

	goMod := "module gocode-lifecycle-synthetic-test\n\ngo " + EngineGoVersion + "\n"
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

// TestGenerateLifecycle_RealPratDomain_AllEntityTypes is the stage-3
// real-domain acceptance run required by PLAN-gen-code.md's stage-3
// acceptance criteria: generate entities.go + lifecycle.go +
// lifecycle_test.go for every EntityType in the real PRAT-hotam "prat"
// domain that builds under BuildEntityModel (8/9 today — sdr-package's
// feature_lead field has kind "reference", unmapped since stage 2, per
// TestGenerateModels_RealPratDomain), assemble them into one temp Go
// module, and run `go build && go test` for real. This is the
// "исполнимый слой" check from contract §0 exercised against real graph
// data, not just the synthetic fixture.
func TestGenerateLifecycle_RealPratDomain_AllEntityTypes(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	domainDir := pratDomainDir(t)

	g, err := loader.LoadGraph(filepath.Join(domainDir, "graph.json"))
	if err != nil {
		t.Fatalf("LoadGraph(prat): %v", err)
	}

	var buildable []ontology.EntityType
	var skipped []string
	for _, et := range g.EntityTypes {
		if _, err := BuildEntityModel(et); err != nil {
			skipped = append(skipped, et.Slug+": "+err.Error())
			continue
		}
		buildable = append(buildable, et)
	}
	t.Logf("buildable: %d/%d EntityTypes", len(buildable), len(g.EntityTypes))
	for _, s := range skipped {
		t.Logf("skipped (pre-existing stage-2 gap): %s", s)
	}
	if len(buildable) == 0 {
		t.Fatal("expected at least one buildable EntityType in the real prat domain")
	}

	models := make([]*entityModel, 0, len(buildable))
	for _, et := range buildable {
		m, err := BuildEntityModel(et)
		if err != nil {
			t.Fatalf("EntityType %q: BuildEntityModel: %v", et.Slug, err)
		}
		models = append(models, m)
	}

	entitiesSrc, err := RenderEntitiesFile("gen", buildable)
	if err != nil {
		t.Fatalf("RenderEntitiesFile: %v", err)
	}
	lifecycleSrc, err := RenderLifecycleFile("gen", models)
	if err != nil {
		t.Fatalf("RenderLifecycleFile: %v", err)
	}
	lifecycleTestSrc, err := RenderLifecycleTestFile("gen", models)
	if err != nil {
		t.Fatalf("RenderLifecycleTestFile: %v", err)
	}

	dir := t.TempDir()
	writeFile := func(name string, content []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	writeFile("entities.go", entitiesSrc)
	writeFile("lifecycle.go", lifecycleSrc)
	writeFile("lifecycle_test.go", lifecycleTestSrc)
	goMod := "module gocode-prat-lifecycle-e2e-test\n\ngo " + EngineGoVersion + "\n"
	writeFile("go.mod", []byte(goMod))

	buildCmd := exec.Command("go", "build", "./...")
	buildCmd.Dir = dir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("generated prat module failed to build: %v\n%s", err, out)
	}

	testCmd := exec.Command("go", "test", "-v", "./...")
	testCmd.Dir = dir
	out, err := testCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated prat module failed go test: %v\n%s", err, out)
	}
	t.Logf("generated prat lifecycle module go test -v output:\n%s", out)
}

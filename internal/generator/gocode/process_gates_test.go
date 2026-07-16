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

// syntheticGatedProcess returns a minimal Process with one gated Step whose
// why text cites an R-anchor, driving two EntityTypes: one whose why cites
// that same anchor (signal 1) and one whose own lifecycle state why cites the
// step's short "P-G7" token (signal 2, state-level, precise).
func syntheticGatedProcess() (ontology.Process, []ontology.EntityType) {
	anchorEntity := ontology.EntityType{
		Slug: "anchor-entity",
		Why:  "R-test-gate-alpha requires this artifact before the gate.",
		Fields: []ontology.EntityField{
			{Name: "text", Kind: "string", Required: true},
		},
		Lifecycle: ontology.Lifecycle{
			Slug: "anchor-entity-lifecycle",
			States: []ontology.State{
				{Name: "draft", Kind: ontology.StateKindInitial},
				{Name: "approved", Kind: ontology.StateKindTerminal},
			},
			Transitions: []ontology.Transition{
				{Src: "draft", Dst: "approved", Event: "approve"},
			},
		},
	}
	tokenEntity := ontology.EntityType{
		Slug: "token-entity",
		Why:  "Unrelated why text, no anchor citation here.",
		Fields: []ontology.EntityField{
			{Name: "text", Kind: "string", Required: true},
		},
		Lifecycle: ontology.Lifecycle{
			Slug: "token-entity-lifecycle",
			States: []ontology.State{
				{Name: "draft", Kind: ontology.StateKindInitial},
				{Name: "reviewed", Kind: ontology.StateKindNormal, Why: "reached during P-G7 review"},
				{Name: "final", Kind: ontology.StateKindTerminal},
			},
			Transitions: []ontology.Transition{
				{Src: "draft", Dst: "reviewed", Event: "review"},
				{Src: "reviewed", Dst: "final", Event: "finalize"},
			},
		},
	}
	unrelatedEntity := ontology.EntityType{
		Slug: "unrelated-entity",
		Why:  "No anchor and no short token here at all.",
		Fields: []ontology.EntityField{
			{Name: "text", Kind: "string", Required: true},
		},
		Lifecycle: ontology.Lifecycle{
			Slug: "unrelated-entity-lifecycle",
			States: []ontology.State{
				{Name: "draft", Kind: ontology.StateKindInitial},
				{Name: "done", Kind: ontology.StateKindTerminal},
			},
			Transitions: []ontology.Transition{
				{Src: "draft", Dst: "done", Event: "finish"},
			},
		},
	}

	proc := ontology.Process{
		ID:        "PR-synthetic",
		Lifecycle: ontology.ProcessLifecycle,
		Steps: []ontology.Step{
			{
				Name:         "gated-step",
				RequiresRole: "tester",
				Why:          "P-G7: gated step. Anchor: R-test-gate-alpha.",
			},
			{
				Name:         "handoff-step",
				RequiresRole: "tester",
				Why:          "A pure handoff step, nothing typed cited at all.",
			},
		},
		RolesRequired:  []string{"tester"},
		DrivesEntities: []string{"anchor-entity", "token-entity", "unrelated-entity"},
	}

	return proc, []ontology.EntityType{anchorEntity, tokenEntity, unrelatedEntity}
}

// TestBuildProcessStepGateModels_Synthetic asserts: (a) exactly one gate is
// built (the anchor-less "handoff-step" produces none - not a gate in the
// methodology's own sense), (b) the gate resolves anchor-entity via signal 1
// and token-entity via signal 2's state-level precise match, and (c)
// unrelated-entity (tied to the step by neither signal) is correctly
// excluded, not swept in by a whole-process fallback that should never
// trigger here (two of three driven entities DID resolve).
func TestBuildProcessStepGateModels_Synthetic(t *testing.T) {
	proc, ets := syntheticGatedProcess()
	models := buildSyntheticModels(t, ets)

	gates, err := BuildProcessStepGateModels([]ontology.Process{proc}, models, nil)
	if err != nil {
		t.Fatalf("BuildProcessStepGateModels: %v", err)
	}
	if len(gates) != 1 {
		t.Fatalf("expected exactly 1 process-step gate (handoff-step has no R-anchor), got %d: %v", len(gates), sortedProcessGateFuncNames(gates))
	}

	g := gates[0]
	if g.step.Name != "gated-step" {
		t.Fatalf("expected gate for step 'gated-step', got %q", g.step.Name)
	}
	if g.wholeProcessFallback {
		t.Fatal("expected a precise per-step entity set, not the whole-process fallback")
	}
	if len(g.entities) != 2 {
		t.Fatalf("expected exactly 2 relevant entities (anchor-entity, token-entity), got %d", len(g.entities))
	}

	bySlug := make(map[string]processGateEntity, len(g.entities))
	for _, e := range g.entities {
		bySlug[e.entity.src.Slug] = e
	}

	anchorHit, ok := bySlug["anchor-entity"]
	if !ok {
		t.Fatal("expected anchor-entity to be resolved via signal 1 (R-anchor citation)")
	}
	if anchorHit.signal != processGateSignalAnchor {
		t.Errorf("anchor-entity: expected processGateSignalAnchor, got %v", anchorHit.signal)
	}
	if anchorHit.preciseState != nil {
		t.Errorf("anchor-entity: expected general RequiresTerminal (no precise state), got %q", anchorHit.preciseState.src.Name)
	}

	tokenHit, ok := bySlug["token-entity"]
	if !ok {
		t.Fatal("expected token-entity to be resolved via signal 2 (short gate-token citation)")
	}
	if tokenHit.signal != processGateSignalState {
		t.Errorf("token-entity: expected processGateSignalState (precise), got %v", tokenHit.signal)
	}
	if tokenHit.preciseState == nil || tokenHit.preciseState.src.Name != "reviewed" {
		t.Errorf("token-entity: expected precise state 'reviewed', got %v", tokenHit.preciseState)
	}

	if _, ok := bySlug["unrelated-entity"]; ok {
		t.Fatal("unrelated-entity must NOT be part of this gate (tied to the step by neither signal)")
	}
}

// TestBuildProcessStepGateModels_WholeProcessFallback asserts the honest
// weaker-case escape hatch: a step whose why cites an R-anchor but which
// matches NEITHER signal against any driven entity falls back to the
// process's entire DrivesEntities set, with wholeProcessFallback=true.
func TestBuildProcessStepGateModels_WholeProcessFallback(t *testing.T) {
	entityA := ontology.EntityType{
		Slug: "entity-a",
		Why:  "No anchor citation, no short token, nothing tying this to any step.",
		Fields: []ontology.EntityField{
			{Name: "text", Kind: "string", Required: true},
		},
		Lifecycle: ontology.Lifecycle{
			Slug: "entity-a-lifecycle",
			States: []ontology.State{
				{Name: "draft", Kind: ontology.StateKindInitial},
				{Name: "done", Kind: ontology.StateKindTerminal},
			},
			Transitions: []ontology.Transition{
				{Src: "draft", Dst: "done", Event: "finish"},
			},
		},
	}
	proc := ontology.Process{
		ID:        "PR-fallback",
		Lifecycle: ontology.ProcessLifecycle,
		Steps: []ontology.Step{
			{Name: "orphan-step", RequiresRole: "tester", Why: "Cites R-orphan-gate but nothing else knows about it."},
		},
		RolesRequired:  []string{"tester"},
		DrivesEntities: []string{"entity-a"},
	}
	models := buildSyntheticModels(t, []ontology.EntityType{entityA})

	gates, err := BuildProcessStepGateModels([]ontology.Process{proc}, models, nil)
	if err != nil {
		t.Fatalf("BuildProcessStepGateModels: %v", err)
	}
	if len(gates) != 1 {
		t.Fatalf("expected exactly 1 gate, got %d", len(gates))
	}
	g := gates[0]
	if !g.wholeProcessFallback {
		t.Fatal("expected wholeProcessFallback=true when neither signal resolves any entity")
	}
	if len(g.entities) != 1 || g.entities[0].entity.src.Slug != "entity-a" {
		t.Fatalf("expected the fallback to include the process's entire DrivesEntities set, got %v", g.entities)
	}
	if g.entities[0].signal != processGateSignalFallback {
		t.Errorf("expected processGateSignalFallback, got %v", g.entities[0].signal)
	}
}

// TestBuildProcessStepGateModels_AnchorLessStepProducesNoGate asserts a step
// with zero R-anchor citations in its own why text (a pure handoff) never
// gets a composite gate function at all - not even an always-true one.
func TestBuildProcessStepGateModels_AnchorLessStepProducesNoGate(t *testing.T) {
	proc, ets := syntheticGatedProcess()
	models := buildSyntheticModels(t, ets)
	gates, err := BuildProcessStepGateModels([]ontology.Process{proc}, models, nil)
	if err != nil {
		t.Fatalf("BuildProcessStepGateModels: %v", err)
	}
	for _, g := range gates {
		if g.step.Name == "handoff-step" {
			t.Fatalf("handoff-step has zero R-anchors and must not produce a composite gate, got %s", g.funcName)
		}
	}
}

// TestRenderProcessGatesFile_Synthetic_ParsesAsGo and its test-half
// counterpart assert the two rendered halves are syntactically valid,
// zero-Cyrillic Go source (GEN-CODE-CONTRACT.md §1.1/§5).
func TestRenderProcessGatesFile_Synthetic_ParsesAsGo(t *testing.T) {
	proc, ets := syntheticGatedProcess()
	models := buildSyntheticModels(t, ets)
	gates, err := BuildProcessStepGateModels([]ontology.Process{proc}, models, nil)
	if err != nil {
		t.Fatalf("BuildProcessStepGateModels: %v", err)
	}
	src, err := RenderProcessGatesFile("gen", gates)
	if err != nil {
		t.Fatalf("RenderProcessGatesFile: %v", err)
	}
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "process_gates.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated process-gate functions do not parse as Go: %v\n%s", err, src)
	}
	for i, r := range string(src) {
		if r > 127 {
			t.Fatalf("non-ASCII rune %q at byte offset %d in generated process-gate functions (GEN-CODE-CONTRACT.md section 1.1 zero-Cyrillic rule)", r, i)
		}
	}
}

func TestRenderProcessGatesTestFile_Synthetic_ParsesAsGo(t *testing.T) {
	proc, ets := syntheticGatedProcess()
	models := buildSyntheticModels(t, ets)
	gates, err := BuildProcessStepGateModels([]ontology.Process{proc}, models, nil)
	if err != nil {
		t.Fatalf("BuildProcessStepGateModels: %v", err)
	}
	src, err := RenderProcessGatesTestFile("gen", gates)
	if err != nil {
		t.Fatalf("RenderProcessGatesTestFile: %v", err)
	}
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "process_gates_test.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated process-gate tests do not parse as Go: %v\n%s", err, src)
	}
}

// TestRenderProcessGatesFile_ZeroGates_NoUnusedImport asserts the empty-gate
// placeholder branch never emits an "fmt" import with nothing using it (the
// same unused-import discipline pipeline.go's own zero-gates branch follows).
func TestRenderProcessGatesFile_ZeroGates_NoUnusedImport(t *testing.T) {
	src, err := RenderProcessGatesFile("gen", nil)
	if err != nil {
		t.Fatalf("RenderProcessGatesFile(nil): %v", err)
	}
	if strings.Contains(string(src), "import \"fmt\"") {
		t.Fatalf("zero-gates branch must not import \"fmt\" with nothing using it:\n%s", src)
	}
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "process_gates.go", src, parser.AllErrors); err != nil {
		t.Fatalf("zero-gates placeholder does not parse as Go: %v\n%s", err, src)
	}

	testSrc, err := RenderProcessGatesTestFile("gen", nil)
	if err != nil {
		t.Fatalf("RenderProcessGatesTestFile(nil): %v", err)
	}
	if strings.Contains(string(testSrc), "import \"testing\"") {
		t.Fatalf("zero-gates branch must not import \"testing\" with nothing using it:\n%s", testSrc)
	}
}

// TestBuildProcessStepGateModels_RealPratDomain asserts the resolved entity
// set for R-gate-pg1-planning-approved's own step (planning-approved) on the
// real prat domain - the task's central acceptance criterion: fr-registry,
// fr-graph, fr-record, implementation-order, risk-registry, forecast
// (precise v1), and gate-decision must ALL be resolved, and NOT via the
// whole-process fallback; brd-package/sdr-package/source-package - which
// genuinely belong to OTHER steps - must be excluded.
//
// Deliberately NOT a pinned exact-count test (invariant form chosen over
// exact-count sync): the prat domain is live and keeps growing entities
// that legitimately join this step (2026-07: jira-write-permit landed with
// P-G1-scoped lifecycle whys and correctly resolves into planning-approved's
// composite gate). Pinning "exactly 7" made this test fail on every such
// enrichment without adding regression signal. The signal that matters is:
// (1) the documented core set is present with the right resolution (incl.
// forecast precisely at v1), (2) other-step entities are excluded, (3) no
// fallback, (4) every resolved entity carries a usable required-state form
// (precise state or non-empty terminal set).
func TestBuildProcessStepGateModels_RealPratDomain(t *testing.T) {
	t.Skip("gen-code retired — authored-spec pivot 2026-07-16; see PLAN-authored-spec-discipline.md")
	domainDir := pratDomainDir(t)
	g, err := loader.LoadGraph(filepath.Join(domainDir, "graph.json"))
	if err != nil {
		t.Fatalf("LoadGraph(prat): %v", err)
	}

	models := buildSyntheticModels(t, g.EntityTypes)
	var settled []ontology.Requirement
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED {
			settled = append(settled, r)
		}
	}

	gates, err := BuildProcessStepGateModels(g.Processes, models, settled)
	if err != nil {
		t.Fatalf("BuildProcessStepGateModels: %v", err)
	}

	var pg *processStepGateModel
	for _, gt := range gates {
		if gt.step.Name == "planning-approved" {
			pg = gt
			break
		}
	}
	if pg == nil {
		t.Fatalf("expected a composite gate for step 'planning-approved', found: %v", sortedProcessGateFuncNames(gates))
	}
	if pg.requirementID != "R-gate-pg1-planning-approved" {
		t.Errorf("expected requirementID R-gate-pg1-planning-approved, got %q", pg.requirementID)
	}
	if pg.wholeProcessFallback {
		t.Fatal("planning-approved must resolve a precise per-step entity set, not the whole-process fallback")
	}

	gotSlugs := make(map[string]processGateEntity, len(pg.entities))
	for _, e := range pg.entities {
		gotSlugs[e.entity.src.Slug] = e
	}

	wantSlugs := []string{"fr-registry", "fr-graph", "fr-record", "implementation-order", "risk-registry", "forecast", "gate-decision"}
	if len(gotSlugs) < len(wantSlugs) {
		t.Fatalf("expected at least the %d documented core entities for planning-approved, got %d: %v", len(wantSlugs), len(gotSlugs), gotSlugs)
	}
	for _, slug := range wantSlugs {
		if _, ok := gotSlugs[slug]; !ok {
			t.Errorf("expected planning-approved's composite gate to require entity %q, not found among: %v", slug, gotSlugs)
		}
	}

	// Every resolved entity — core or newly grown — must carry a usable
	// required-state form: either a precise state with a non-empty runtime
	// value, or a non-empty terminal-state set for the general form.
	for slug, e := range gotSlugs {
		if e.preciseState != nil {
			if e.preciseState.value == "" {
				t.Errorf("entity %q: precise state with empty runtime value", slug)
			}
		} else if len(e.terminal) == 0 {
			t.Errorf("entity %q: neither a precise state nor any terminal state resolved", slug)
		}
	}

	excluded := []string{"brd-package", "sdr-package", "source-package"}
	for _, slug := range excluded {
		if _, ok := gotSlugs[slug]; ok {
			t.Errorf("entity %q belongs to a DIFFERENT step and must NOT be part of planning-approved's composite gate", slug)
		}
	}

	// forecast must resolve to the PRECISE v1 state, not a general terminal
	// gate - the exact precision this task's claim requires ("forecast_v1",
	// not "any terminal forecast state").
	forecastEntity, ok := gotSlugs["forecast"]
	if !ok {
		t.Fatal("expected forecast to be one of planning-approved's required entities")
	}
	if forecastEntity.preciseState == nil || forecastEntity.preciseState.src.Name != "v1" {
		t.Fatalf("expected forecast's required state to be precisely 'v1', got %v", forecastEntity.preciseState)
	}
}

// TestGenerateAllFromGraph_RealPratDomain_ProcessGatesCompileAndRun generates
// the FULL gen-code output set (contract §1 stages 2-6) for the real prat
// domain into a temp dir and asserts `go build && go test` are green,
// including the new process_gates_test.go file - the task's "реально вызывающий
// составной гейт" acceptance criterion end to end.
func TestGenerateAllFromGraph_RealPratDomain_ProcessGatesCompileAndRun(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	domainDir := pratDomainDir(t)
	g, err := loader.LoadGraph(filepath.Join(domainDir, "graph.json"))
	if err != nil {
		t.Fatalf("LoadGraph(prat): %v", err)
	}

	files, err := GenerateAllFromGraph(g.EntityTypes, g.Requirements, g.Processes, "prat-process-gates-test")
	if err != nil {
		t.Fatalf("GenerateAllFromGraph: %v", err)
	}
	if _, ok := files["process_gates_test.go"]; !ok {
		t.Fatal("expected process_gates_test.go to be generated for the real prat domain")
	}

	dir := t.TempDir()
	for name, content := range files {
		if strings.HasSuffix(name, ".md") {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	buildCmd := exec.Command("go", "build", "./...")
	buildCmd.Dir = dir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("generated prat module (with process gates) failed to build: %v\n%s", err, out)
	}

	testCmd := exec.Command("go", "test", "./...", "-run", "TestGatePr", "-v")
	testCmd.Dir = dir
	out, err := testCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated prat module's process-gate tests failed: %v\n%s", err, out)
	}
	t.Logf("go test -run TestGatePr -v output:\n%s", out)
}

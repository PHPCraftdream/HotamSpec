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

// twoEntitySyntheticGraph returns two EntityTypes joined by one kind:reference
// field ("upstream" -> "downstream", mirroring fr-graph.входной_реестр ->
// fr-registry): downstream has a 2-state lifecycle (draft/initial ->
// approved/terminal) via one transition, and upstream carries a required
// reference field targeting downstream's slug plus an unresolvable reference
// field (ref_target "Stakeholder", mirroring sdr-package.feature_lead) that
// must NOT produce a gate.
func twoEntitySyntheticGraph() []ontology.EntityType {
	downstream := ontology.EntityType{
		Slug: "downstream",
		Why:  "R-test-only downstream artifact.",
		Fields: []ontology.EntityField{
			{Name: "текст", Kind: "string", Required: true},
		},
		Lifecycle: ontology.Lifecycle{
			Slug: "downstream-lifecycle",
			States: []ontology.State{
				{Name: "черновик", Kind: ontology.StateKindInitial},
				{Name: "утверждён", Kind: ontology.StateKindTerminal},
			},
			Transitions: []ontology.Transition{
				{Src: "черновик", Dst: "утверждён", Event: "утвердить-pm"},
			},
		},
	}
	upstream := ontology.EntityType{
		Slug: "upstream",
		Why:  "R-test-only upstream artifact referencing downstream.",
		Fields: []ontology.EntityField{
			{Name: "ссылка", Kind: "reference", Required: true, RefTarget: "downstream"},
			{Name: "feature_lead", Kind: "reference", Required: true, RefTarget: "Stakeholder"},
		},
		Lifecycle: ontology.Lifecycle{
			Slug: "upstream-lifecycle",
			States: []ontology.State{
				{Name: "черновик", Kind: ontology.StateKindInitial},
				{Name: "утверждён", Kind: ontology.StateKindTerminal},
			},
			Transitions: []ontology.Transition{
				{Src: "черновик", Dst: "утверждён", Event: "утвердить-pm"},
			},
		},
	}
	return []ontology.EntityType{downstream, upstream}
}

func buildSyntheticModels(t *testing.T, ets []ontology.EntityType) []*entityModel {
	t.Helper()
	var models []*entityModel
	for _, et := range ets {
		m, err := BuildEntityModel(et)
		if err != nil {
			t.Fatalf("BuildEntityModel(%s): %v", et.Slug, err)
		}
		models = append(models, m)
	}
	return models
}

// TestBuildPipelineGateModels_Synthetic asserts exactly one gate is produced
// for the resolvable reference field (upstream.ссылка -> downstream) and
// none for the unresolvable one (upstream.feature_lead -> "Stakeholder",
// which has no EntityType of that slug in this synthetic domain) — the same
// "honest TODO, not a gate" behavior the real prat domain's
// sdr-package.feature_lead field must also get.
func TestBuildPipelineGateModels_Synthetic(t *testing.T) {
	models := buildSyntheticModels(t, twoEntitySyntheticGraph())
	gates, err := BuildPipelineGateModels(models)
	if err != nil {
		t.Fatalf("BuildPipelineGateModels: %v", err)
	}
	if len(gates) != 1 {
		t.Fatalf("expected exactly 1 gate (feature_lead's Stakeholder ref_target must not resolve), got %d", len(gates))
	}
	g := gates[0]
	if g.referencer.structName != "Upstream" {
		t.Errorf("referencer = %q, want Upstream", g.referencer.structName)
	}
	if g.referenced.structName != "Downstream" {
		t.Errorf("referenced = %q, want Downstream", g.referenced.structName)
	}
	wantFunc := "GateUpstreamReferenceRequiresDownstreamTerminal"
	if g.funcName != wantFunc {
		t.Errorf("funcName = %q, want %q", g.funcName, wantFunc)
	}
	if len(g.terminalStates) != 1 || g.terminalStates[0].constant != "DownstreamStateApproved" {
		t.Fatalf("terminalStates = %+v, want [DownstreamStateApproved]", g.terminalStates)
	}
}

// TestShortestPathToTerminal_Synthetic asserts the BFS walk from downstream's
// initial state reaches its one terminal state via exactly the one declared
// transition method (ApprovePM) — the real generated method, not a
// hand-authored stand-in.
func TestShortestPathToTerminal_Synthetic(t *testing.T) {
	models := buildSyntheticModels(t, twoEntitySyntheticGraph())
	var downstream *entityModel
	for _, m := range models {
		if m.structName == "Downstream" {
			downstream = m
		}
	}
	if downstream == nil {
		t.Fatal("Downstream model not found")
	}
	path, err := shortestPathToTerminal(downstream)
	if err != nil {
		t.Fatalf("shortestPathToTerminal: %v", err)
	}
	if len(path.methods) != 1 || path.methods[0] != "ApprovePM" {
		t.Fatalf("path.methods = %v, want [ApprovePM]", path.methods)
	}
	if path.dst.constant != "DownstreamStateApproved" {
		t.Errorf("path.dst = %q, want DownstreamStateApproved", path.dst.constant)
	}
}

// TestRenderPipelineFile_Synthetic_ParsesAsGo asserts the gate-function half
// renders as syntactically valid Go and contains the expected function
// signature.
func TestRenderPipelineFile_Synthetic_ParsesAsGo(t *testing.T) {
	models := buildSyntheticModels(t, twoEntitySyntheticGraph())
	gates, err := BuildPipelineGateModels(models)
	if err != nil {
		t.Fatalf("BuildPipelineGateModels: %v", err)
	}
	src, err := RenderPipelineFile("gen", gates)
	if err != nil {
		t.Fatalf("RenderPipelineFile: %v", err)
	}
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "pipeline_gate.go", src, 0); err != nil {
		t.Fatalf("RenderPipelineFile output does not parse as Go: %v\n%s", err, src)
	}
	if !strings.Contains(string(src), "func GateUpstreamReferenceRequiresDownstreamTerminal(referenced *Downstream) error {") {
		t.Errorf("expected gate function signature in output:\n%s", src)
	}
}

// TestRenderPipelineTestFile_Synthetic_ParsesAsGo asserts the table-driven
// test half renders as syntactically valid Go and contains both the
// not-yet-terminal and reaches-terminal sub-tests.
func TestRenderPipelineTestFile_Synthetic_ParsesAsGo(t *testing.T) {
	models := buildSyntheticModels(t, twoEntitySyntheticGraph())
	gates, err := BuildPipelineGateModels(models)
	if err != nil {
		t.Fatalf("BuildPipelineGateModels: %v", err)
	}
	src, err := RenderPipelineTestFile("gen", gates)
	if err != nil {
		t.Fatalf("RenderPipelineTestFile: %v", err)
	}
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "pipeline_gate_test.go", src, 0); err != nil {
		t.Fatalf("RenderPipelineTestFile output does not parse as Go: %v\n%s", err, src)
	}
	for _, want := range []string{
		"func TestGateUpstreamReferenceRequiresDownstreamTerminal(t *testing.T) {",
		`t.Run("not yet terminal"`,
		`t.Run("reaches terminal"`,
		"referenced.ApprovePM()",
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("expected %q in output:\n%s", want, src)
		}
	}
}

// TestPipelineFiles_Synthetic_NonASCII asserts both rendered halves contain
// zero non-ASCII bytes (GEN-CODE-CONTRACT.md §1.1/§5) even though the
// synthetic source graph above is entirely Cyrillic-named.
func TestPipelineFiles_Synthetic_NonASCII(t *testing.T) {
	models := buildSyntheticModels(t, twoEntitySyntheticGraph())
	gates, err := BuildPipelineGateModels(models)
	if err != nil {
		t.Fatalf("BuildPipelineGateModels: %v", err)
	}
	gateSrc, err := RenderPipelineFile("gen", gates)
	if err != nil {
		t.Fatalf("RenderPipelineFile: %v", err)
	}
	testSrc, err := RenderPipelineTestFile("gen", gates)
	if err != nil {
		t.Fatalf("RenderPipelineTestFile: %v", err)
	}
	assertASCIIOnly(t, "pipeline gate funcs", gateSrc)
	assertASCIIOnly(t, "pipeline gate tests", testSrc)
}

func assertASCIIOnly(t *testing.T, label string, src []byte) {
	t.Helper()
	for i, r := range string(src) {
		if r > 127 {
			t.Fatalf("%s: non-ASCII rune %q at byte offset %d (GEN-CODE-CONTRACT.md section 1.1):\n%s", label, r, i, src)
		}
	}
}

// TestGeneratePipelineFromGraph_ZeroGates_CompilesAndVets asserts a domain
// with NO resolvable kind:reference field (no pipeline gates at all) still
// renders a pipeline_test.go that passes `go vet`/`go test`, not merely
// `go build` — an earlier version of this generator unconditionally emitted
// `import "fmt"`/`import "testing"` even in the zero-gates placeholder-
// comment branch, which parses as valid Go (go/parser does not type-check)
// but fails `go vet`/`go test` with "imported and not used", a regression
// `go build ./...` alone (which skips _test.go files) would never catch.
func TestGeneratePipelineFromGraph_ZeroGates_CompilesAndVets(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	et := ontology.EntityType{
		Slug: "solo",
		Fields: []ontology.EntityField{
			{Name: "text", Kind: "string", Required: true},
		},
		Lifecycle: ontology.Lifecycle{
			Slug: "solo-lifecycle",
			States: []ontology.State{
				{Name: "draft", Kind: ontology.StateKindInitial},
				{Name: "done", Kind: ontology.StateKindTerminal},
			},
			Transitions: []ontology.Transition{
				{Src: "draft", Dst: "done", Event: "finish"},
			},
		},
	}
	ets := []ontology.EntityType{et}

	modelsFiles, err := GenerateModelsFromGraph("zero-gates-test", ets)
	if err != nil {
		t.Fatalf("GenerateModelsFromGraph: %v", err)
	}
	lifecycleFiles, err := GenerateLifecycleFromGraph(ets)
	if err != nil {
		t.Fatalf("GenerateLifecycleFromGraph: %v", err)
	}
	pipelineFiles, err := GeneratePipelineFromGraph(ets)
	if err != nil {
		t.Fatalf("GeneratePipelineFromGraph: %v", err)
	}
	pipelineSrc := pipelineFiles["pipeline_test.go"]
	if strings.Contains(string(pipelineSrc), "import") {
		t.Errorf("expected zero-gates pipeline_test.go to have NO import statement at all, got:\n%s", pipelineSrc)
	}

	dir := t.TempDir()
	writeAll := func(files map[string][]byte) {
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
	}
	writeAll(modelsFiles)
	writeAll(lifecycleFiles)
	writeAll(pipelineFiles)

	vetCmd := exec.Command("go", "vet", "./...")
	vetCmd.Dir = dir
	if out, err := vetCmd.CombinedOutput(); err != nil {
		t.Fatalf("go vet failed on zero-gates pipeline_test.go: %v\n%s", err, out)
	}
	testCmd := exec.Command("go", "test", "./...")
	testCmd.Dir = dir
	if out, err := testCmd.CombinedOutput(); err != nil {
		t.Fatalf("go test failed on zero-gates pipeline_test.go: %v\n%s", err, out)
	}
}

// TestGeneratePipelineFromGraph_Synthetic_CompilesAndRuns writes the full
// generated module (entities.go + lifecycle.go + pipeline_test.go, the
// minimum set pipeline_test.go's gate functions and tests need to compile)
// for the synthetic two-entity graph into a temp dir and runs `go test`,
// asserting it is green — the real end-to-end proof that the generated gate
// + generated transition methods + generated test cohere (contract §5:
// "Компилируемость и проходимость").
func TestGeneratePipelineFromGraph_Synthetic_CompilesAndRuns(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	ets := twoEntitySyntheticGraph()

	modelsFiles, err := GenerateModelsFromGraph("synthetic-pipeline", ets)
	if err != nil {
		t.Fatalf("GenerateModelsFromGraph: %v", err)
	}
	lifecycleFiles, err := GenerateLifecycleFromGraph(ets)
	if err != nil {
		t.Fatalf("GenerateLifecycleFromGraph: %v", err)
	}
	pipelineFiles, err := GeneratePipelineFromGraph(ets)
	if err != nil {
		t.Fatalf("GeneratePipelineFromGraph: %v", err)
	}

	dir := t.TempDir()
	writeAll := func(files map[string][]byte) {
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
	}
	writeAll(modelsFiles)
	writeAll(lifecycleFiles)
	writeAll(pipelineFiles)
	// requirements_audit.md is emitted by both GenerateModelsFromGraph (no —
	// actually only GenerateLifecycleFromGraph/GenerateRequirementsFromGraph
	// do) — lifecycleFiles already carries it; no extra write needed here.

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated synthetic pipeline module failed go test: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("expected 'go test' success output, got:\n%s", out)
	}
}

// TestGeneratePipelineFromGraph_Synthetic_MutationCatchesRegression is the
// contract §5 mutational-proof for stage 5: it takes the same synthetic
// generated module as the test above, confirms it is green, then MUTATES the
// gate function (deletes the terminal-state comparison, making it always
// return nil regardless of referenced.State — the exact "gate without
// checking terminal" regression contract §5 names) and confirms `go test`
// now FAILS. A gate test suite that cannot go red on this mutation is not a
// real regression guard.
func TestGeneratePipelineFromGraph_Synthetic_MutationCatchesRegression(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	ets := twoEntitySyntheticGraph()

	modelsFiles, err := GenerateModelsFromGraph("synthetic-pipeline-mutation", ets)
	if err != nil {
		t.Fatalf("GenerateModelsFromGraph: %v", err)
	}
	lifecycleFiles, err := GenerateLifecycleFromGraph(ets)
	if err != nil {
		t.Fatalf("GenerateLifecycleFromGraph: %v", err)
	}
	pipelineFiles, err := GeneratePipelineFromGraph(ets)
	if err != nil {
		t.Fatalf("GeneratePipelineFromGraph: %v", err)
	}

	dir := t.TempDir()
	writeAll := func(files map[string][]byte) {
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
	}
	writeAll(modelsFiles)
	writeAll(lifecycleFiles)
	writeAll(pipelineFiles)

	runGoTest := func() ([]byte, error) {
		cmd := exec.Command("go", "test", "./...")
		cmd.Dir = dir
		return cmd.CombinedOutput()
	}

	// --- BEFORE: unmutated, must be green ---
	beforeOut, err := runGoTest()
	if err != nil {
		t.Fatalf("BEFORE mutation: expected green `go test`, got error: %v\n%s", err, beforeOut)
	}
	t.Logf("BEFORE mutation (green):\n%s", beforeOut)

	// --- mutate: replace the switch-based terminal check with "always nil" ---
	pipelinePath := filepath.Join(dir, "pipeline_test.go")
	original, err := os.ReadFile(pipelinePath)
	if err != nil {
		t.Fatalf("read pipeline_test.go: %v", err)
	}
	mutated := mutateGateAlwaysNil(t, string(original))
	if err := os.WriteFile(pipelinePath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("write mutated pipeline_test.go: %v", err)
	}

	// --- AFTER: mutated, must be red ---
	afterOut, afterErr := runGoTest()
	t.Logf("AFTER mutation (must be red):\n%s", afterOut)
	if afterErr == nil {
		t.Fatalf("AFTER mutating the gate to always return nil, expected `go test` to FAIL (red), but it passed:\n%s", afterOut)
	}
	if !strings.Contains(string(afterOut), "FAIL") {
		t.Fatalf("AFTER mutation: expected output to contain FAIL, got:\n%s", afterOut)
	}
	// The specific sub-test the mutation should break is "not yet terminal"
	// (the gate now accepts a fresh, non-terminal referenced entity) - assert
	// the failure is that precise sub-test, not merely SOME unrelated FAIL
	// (e.g. a build error, which would defeat the point of this proof).
	if !strings.Contains(string(afterOut), "not_yet_terminal") && !strings.Contains(string(afterOut), "not yet terminal") {
		t.Fatalf("AFTER mutation: expected the \"not yet terminal\" sub-test to be named in the failure output, got:\n%s", afterOut)
	}
}

// mutateGateAlwaysNil performs the exact §5 mutation named in the task
// brief: it removes the gate function's terminal-state comparison, replacing
// the whole switch statement with an unconditional `return nil` — the
// generated gate then accepts ANY referenced.State, including a freshly
// constructed non-terminal one, which the "not yet terminal" sub-test must
// then catch and fail on. The mutated body keeps a `var _ = fmt.Sprint`
// reference so the file still COMPILES (an unused "fmt" import would fail
// the build instead of letting the test suite itself catch the regression —
// contract §5's mutational proof is specifically about the GENERATED TEST
// going red, not about a compile error standing in for it).
func mutateGateAlwaysNil(t *testing.T, src string) string {
	t.Helper()
	start := strings.Index(src, "func GateUpstreamReferenceRequiresDownstreamTerminal(referenced *Downstream) error {")
	if start == -1 {
		t.Fatalf("mutateGateAlwaysNil: gate function not found in source:\n%s", src)
	}
	bodyStart := strings.Index(src[start:], "{") + start
	// Find the matching closing brace for the function body via simple
	// depth counting (the generated function body has no nested braces
	// other than the switch, which this mutation is specifically targeting).
	depth := 0
	end := -1
	for i := bodyStart; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end != -1 {
			break
		}
	}
	if end == -1 {
		t.Fatalf("mutateGateAlwaysNil: could not find matching closing brace")
	}
	mutatedBody := "{\n\tvar _ = fmt.Sprint\n\treturn nil\n}"
	return src[:bodyStart] + mutatedBody + src[end+1:]
}

// TestGeneratePipelineFromGraph_RealPratDomain runs the full stage-5
// generator against the real PRAT-hotam "prat" domain and asserts exactly
// the 4 documented pipeline gates are present, with the exact resolved
// identifiers (confirmed once via BuildEntityModel against the real
// glossary, GEN-CODE-CONTRACT.md §4.1 - прогноз/forecast, входной/input,
// реестр/registry, граф/graph, зависимостей/dependencies were added to the
// glossary as part of this stage; see identifiers.go):
//
//	fr-graph.входной_реестр          -> fr-registry           (field: InputRegistry)
//	implementation-order.граф_зависимостей -> fr-graph         (field: GraphDependencies)
//	brd-package.прогноз              -> forecast               (field: Forecast)
//	sdr-package.прогноз              -> forecast               (field: Forecast)
//
// sdr-package.feature_lead (ref_target "Stakeholder", field: FeatureLead)
// must NOT produce a 5th gate — "Stakeholder" resolves to no EntityType slug
// in this domain's graph (contract §2/§6's existing honest TODO case).
func TestGeneratePipelineFromGraph_RealPratDomain(t *testing.T) {
	domainDir := pratDomainDir(t)
	g, err := loader.LoadGraph(filepath.Join(domainDir, "graph.json"))
	if err != nil {
		t.Fatalf("LoadGraph(prat): %v", err)
	}

	models := buildSyntheticModels(t, g.EntityTypes)
	gates, err := BuildPipelineGateModels(models)
	if err != nil {
		t.Fatalf("BuildPipelineGateModels: %v", err)
	}
	for _, gt := range gates {
		t.Logf("gate: %s (%s.%s -> %s)", gt.funcName, gt.referencer.structName, gt.field.fieldName, gt.referenced.structName)
	}
	if len(gates) != 4 {
		t.Fatalf("expected exactly 4 pipeline gates on the real prat domain, got %d: %v", len(gates), gateFuncNames(gates))
	}

	type triple struct {
		referencer, field, referenced string
	}
	wantFuncs := map[string]triple{
		"GateFrGraphInputRegistryRequiresFrRegistryTerminal":              {"FrGraph", "InputRegistry", "FrRegistry"},
		"GateImplementationOrderGraphDependenciesRequiresFrGraphTerminal": {"ImplementationOrder", "GraphDependencies", "FrGraph"},
		"GateBrdPackageForecastRequiresForecastTerminal":                  {"BrdPackage", "Forecast", "Forecast"},
		"GateSdrPackageForecastRequiresForecastTerminal":                  {"SdrPackage", "Forecast", "Forecast"},
	}

	byFunc := make(map[string]*pipelineGateModel, len(gates))
	for _, gt := range gates {
		byFunc[gt.funcName] = gt
	}
	for funcName, want := range wantFuncs {
		gt, ok := byFunc[funcName]
		if !ok {
			t.Errorf("expected gate function %q, not found among: %v", funcName, gateFuncNames(gates))
			continue
		}
		if gt.referencer.structName != want.referencer || gt.field.fieldName != want.field || gt.referenced.structName != want.referenced {
			t.Errorf("gate %q = (%s.%s -> %s), want (%s.%s -> %s)",
				funcName, gt.referencer.structName, gt.field.fieldName, gt.referenced.structName,
				want.referencer, want.field, want.referenced)
		}
	}

	// sdr-package.feature_lead (ref_target "Stakeholder") must not resolve.
	for _, gt := range gates {
		if gt.referencer.structName == "SdrPackage" && gt.field.src.Name == "feature_lead" {
			t.Fatalf("sdr-package.feature_lead (ref_target Stakeholder) must NOT produce a gate, but got %s", gt.funcName)
		}
	}
}

func gateFuncNames(gates []*pipelineGateModel) []string {
	out := make([]string, len(gates))
	for i, g := range gates {
		out[i] = g.funcName
	}
	return out
}

// TestGeneratePipelineFromGraph_RealPratDomain_CompilesAndRuns generates the
// full go/ module set (entities.go, lifecycle.go, lifecycle_test.go,
// requirements_test.go, pipeline_test.go) for the real prat domain into a
// temp dir and asserts `go build && go test` are green — the task brief's
// "реальный прогон ... покажи что все 4 gate-функции реально сгенерированы и
// содержательны" end-to-end proof.
func TestGeneratePipelineFromGraph_RealPratDomain_CompilesAndRuns(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	domainDir := pratDomainDir(t)
	g, err := loader.LoadGraph(filepath.Join(domainDir, "graph.json"))
	if err != nil {
		t.Fatalf("LoadGraph(prat): %v", err)
	}

	modelsFiles, err := GenerateModelsFromGraph("prat-pipeline-test", g.EntityTypes)
	if err != nil {
		t.Fatalf("GenerateModelsFromGraph: %v", err)
	}
	lifecycleFiles, err := GenerateLifecycleFromGraph(g.EntityTypes)
	if err != nil {
		t.Fatalf("GenerateLifecycleFromGraph: %v", err)
	}
	reqFiles, err := GenerateRequirementsFromGraph(g.EntityTypes, g.Requirements)
	if err != nil {
		t.Fatalf("GenerateRequirementsFromGraph: %v", err)
	}
	pipelineFiles, err := GeneratePipelineFromGraph(g.EntityTypes)
	if err != nil {
		t.Fatalf("GeneratePipelineFromGraph: %v", err)
	}

	dir := t.TempDir()
	writeAll := func(files map[string][]byte) {
		for name, content := range files {
			if strings.HasSuffix(name, ".md") {
				continue // requirements_audit.md is markdown, not part of the Go module build
			}
			if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
	}
	writeAll(modelsFiles)
	writeAll(lifecycleFiles)
	writeAll(reqFiles)
	writeAll(pipelineFiles)

	if pipelineSrc, ok := pipelineFiles["pipeline_test.go"]; ok {
		t.Logf("generated pipeline_test.go (%d bytes)", len(pipelineSrc))
	}

	buildCmd := exec.Command("go", "build", "./...")
	buildCmd.Dir = dir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("generated prat module failed to build: %v\n%s", err, out)
	}

	testCmd := exec.Command("go", "test", "./...", "-v")
	testCmd.Dir = dir
	out, err := testCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated prat module failed go test: %v\n%s", err, out)
	}
	t.Logf("go test -v output:\n%s", out)
}

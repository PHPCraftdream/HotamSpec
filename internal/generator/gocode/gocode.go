package gocode

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// EngineGoVersion is the Go version declared in HotamSpec's own go.mod
// (module github.com/PHPCraftdream/HotamSpec). GEN-CODE-CONTRACT.md §1
// requires the generated domain go.mod to declare "go <версия из движка>" —
// this is that version, kept as a constant here (rather than parsed from the
// engine's go.mod at generation time) so generation has no runtime
// dependency on the engine's own working directory layout. It is exercised
// by identifiers_test.go/models_test.go/gocode_test.go, which fail loudly if
// it drifts from the real go.mod.
const EngineGoVersion = "1.25.0"

// DomainSlug returns the last path element of domainDir, used as the
// "hotam-gen/<domain-slug>" module name (contract §1: "module
// hotam-gen/<d>").
func DomainSlug(domainDir string) string {
	clean := filepath.ToSlash(filepath.Clean(domainDir))
	parts := strings.Split(clean, "/")
	return parts[len(parts)-1]
}

// ModuleName returns the Go module name generated go.mod declares for the
// domain at domainDir.
func ModuleName(domainDir string) string {
	return "hotam-gen/" + DomainSlug(domainDir)
}

// GenerateModels loads the domain graph at domainDir/graph.json and renders
// the Go model layer (contract §1 stage-2 scope): entities.go (structs +
// state enums + Validate) and a minimal go.mod. It returns file contents
// keyed by filename, relative to the eventual domains/<d>/gen/go/ output
// directory — this function does not write to disk (the CLI command, a
// later stage, owns that).
func GenerateModels(domainDir string) (map[string][]byte, error) {
	graphPath := filepath.Join(domainDir, "graph.json")
	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateModels: %w", err)
	}
	return GenerateModelsFromGraph(domainDir, g.EntityTypes)
}

// GenerateModelsFromGraph renders the Go model layer directly from a slice
// of EntityType (no filesystem/graph.json load), letting callers (tests, or
// a future in-memory pipeline) supply EntityTypes without a domain
// directory on disk.
func GenerateModelsFromGraph(domainDir string, entityTypes []ontology.EntityType) (map[string][]byte, error) {
	entitiesSrc, err := RenderEntitiesFile("gen", entityTypes)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateModelsFromGraph: %w", err)
	}

	goMod := renderGoMod(ModuleName(domainDir))

	return map[string][]byte{
		"entities.go": entitiesSrc,
		"go.mod":      []byte(goMod),
	}, nil
}

// GenerateLifecycleFromGraph renders stage-3 output (contract §1:
// lifecycle.go + lifecycle_test.go) plus the stage-3.5 audit artifact
// (requirements_audit.md, contract §1.1) directly from a slice of
// EntityType. Building every EntityType's *entityModel once and sharing it
// between all three renderers keeps method-name/state-constant/entity-slug
// identifiers guaranteed consistent across the transition methods, the
// tests that exercise them, and the audit headings that anchor back to them
// (the same reasoning BuildEntityModel's doc comment gives for entities.go's
// struct/enum/Validate trio) — no renderer here re-derives an identifier or
// translated value BuildEntityModel already computed.
func GenerateLifecycleFromGraph(entityTypes []ontology.EntityType) (map[string][]byte, error) {
	sorted := make([]ontology.EntityType, len(entityTypes))
	copy(sorted, entityTypes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Slug < sorted[j].Slug })

	models := make([]*entityModel, 0, len(sorted))
	for _, et := range sorted {
		m, err := BuildEntityModel(et)
		if err != nil {
			return nil, fmt.Errorf("gocode: GenerateLifecycleFromGraph: %w", err)
		}
		models = append(models, m)
	}

	lifecycleSrc, err := RenderLifecycleFile("gen", models)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateLifecycleFromGraph: %w", err)
	}
	lifecycleTestSrc, err := RenderLifecycleTestFile("gen", models)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateLifecycleFromGraph: %w", err)
	}
	auditSrc, err := RenderAuditFile(models, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateLifecycleFromGraph: %w", err)
	}

	return map[string][]byte{
		"lifecycle.go":          lifecycleSrc,
		"lifecycle_test.go":     lifecycleTestSrc,
		"requirements_audit.md": auditSrc,
	}, nil
}

// GenerateRequirementsFromGraph renders stage-4 output (contract §1:
// requirements_test.go) plus its section of requirements_audit.md, directly
// from a slice of EntityType and Requirement. Building every EntityType's
// *entityModel once (mirroring GenerateLifecycleFromGraph) and threading it
// into BuildRequirementModels keeps the requirement atoms' field/state
// identifiers guaranteed consistent with entities.go/lifecycle.go's own
// identifiers — no independent re-derivation. Pipeline gate models
// (BuildPipelineGateModels) are also built here and threaded into
// BuildRequirementModels (task #209) so a field atom on a kind:reference
// field can mirror its own already-built precise-state pipeline gate,
// exactly as GenerateAllFromGraph does — this standalone stage-4 entry point
// must not silently skip that wiring just because it does not itself return
// pipeline_test.go.
func GenerateRequirementsFromGraph(entityTypes []ontology.EntityType, requirements []ontology.Requirement) (map[string][]byte, error) {
	sorted := make([]ontology.EntityType, len(entityTypes))
	copy(sorted, entityTypes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Slug < sorted[j].Slug })

	entityModels := make([]*entityModel, 0, len(sorted))
	for _, et := range sorted {
		m, err := BuildEntityModel(et)
		if err != nil {
			return nil, fmt.Errorf("gocode: GenerateRequirementsFromGraph: %w", err)
		}
		entityModels = append(entityModels, m)
	}

	gates, err := BuildPipelineGateModels(entityModels, requirements)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateRequirementsFromGraph: %w", err)
	}

	reqModels, err := BuildRequirementModels(requirements, entityModels, gates)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateRequirementsFromGraph: %w", err)
	}

	requirementsTestSrc, err := RenderRequirementsTestFile("gen", reqModels)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateRequirementsFromGraph: %w", err)
	}

	auditSrc, err := RenderAuditFile(entityModels, reqModels, nil)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateRequirementsFromGraph: %w", err)
	}

	return map[string][]byte{
		"requirements_test.go":  requirementsTestSrc,
		"requirements_audit.md": auditSrc,
	}, nil
}

// GeneratePipelineFromGraph renders stage-5 output (contract §1:
// pipeline_test.go — both the gate-function half, pipeline.go, and the
// table-driven test half, pipeline_test_gen.go, concatenated into the single
// file the contract's §1 layout names) directly from a slice of EntityType.
// Building every EntityType's *entityModel once (mirroring
// GenerateLifecycleFromGraph/GenerateRequirementsFromGraph) keeps every gate
// function's referencer/referenced/field identifiers guaranteed consistent
// with entities.go/lifecycle.go's own identifiers — no independent
// re-derivation (contract §0). requirements is the domain's full requirement
// corpus (contract §2.1's precise-state gate search needs the SETTLED
// claims); pass nil for a domain with no requirements loaded, which simply
// disables precise-gate detection and falls back to RequiresTerminal for
// every gate, unchanged from pre-§2.1 behavior.
func GeneratePipelineFromGraph(entityTypes []ontology.EntityType, requirements []ontology.Requirement) (map[string][]byte, error) {
	sorted := make([]ontology.EntityType, len(entityTypes))
	copy(sorted, entityTypes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Slug < sorted[j].Slug })

	models := make([]*entityModel, 0, len(sorted))
	for _, et := range sorted {
		m, err := BuildEntityModel(et)
		if err != nil {
			return nil, fmt.Errorf("gocode: GeneratePipelineFromGraph: %w", err)
		}
		models = append(models, m)
	}

	gates, err := BuildPipelineGateModels(models, requirements)
	if err != nil {
		return nil, fmt.Errorf("gocode: GeneratePipelineFromGraph: %w", err)
	}

	gateFuncsSrc, err := RenderPipelineFile("gen", gates)
	if err != nil {
		return nil, fmt.Errorf("gocode: GeneratePipelineFromGraph: %w", err)
	}
	gateTestsSrc, err := RenderPipelineTestFile("gen", gates)
	if err != nil {
		return nil, fmt.Errorf("gocode: GeneratePipelineFromGraph: %w", err)
	}

	pipelineTestSrc := mergePipelineFile(gateFuncsSrc, gateTestsSrc)

	auditSrc, err := RenderAuditFile(models, nil, gates)
	if err != nil {
		return nil, fmt.Errorf("gocode: GeneratePipelineFromGraph: %w", err)
	}

	return map[string][]byte{
		"pipeline_test.go":      pipelineTestSrc,
		"requirements_audit.md": auditSrc,
	}, nil
}

// GenerateAllFromGraph renders the complete gen-code output set (contract
// §1: go.mod, entities.go, lifecycle.go, lifecycle_test.go,
// requirements_test.go, pipeline_test.go, and requirements_audit.md) from a
// single already-loaded graph, in one map[filename][]byte the caller can
// write out directly.
//
// Unlike calling GenerateModelsFromGraph/GenerateLifecycleFromGraph/
// GenerateRequirementsFromGraph/GeneratePipelineFromGraph separately and
// merging their four returned maps (cmd/hotam/gen_code.go's genCode used to
// do exactly that), this function builds every []*entityModel/
// []*requirementModel/[]*pipelineGateModel exactly ONCE and threads them into
// a single RenderAuditFile call at the very end, with all three parameters
// fully populated. That is the point of this function: each of the four
// Generate*FromGraph functions above independently calls RenderAuditFile
// with only the subset of models/reqModels/gates it happens to own (lifecycle
// passes reqModels=nil,gates=nil; requirements passes gates=nil; pipeline
// passes reqModels=nil) — every one of those calls is individually correct
// for what that stage owns, but genCode was merging the four stages' file
// maps by filename, so requirements_audit.md kept getting overwritten by
// whichever stage rendered last (pipeline), discarding the "## Requirements"
// section requirements-stage's own render had. Contract §0 requires exactly
// one source of truth for requirements_audit.md; this function is now that
// one place with the full picture, called once, after everything else has
// already been computed — not two (or four) competing renders merged by
// last-write-wins file-map semantics.
func GenerateAllFromGraph(entityTypes []ontology.EntityType, requirements []ontology.Requirement, domainDir string) (map[string][]byte, error) {
	sorted := make([]ontology.EntityType, len(entityTypes))
	copy(sorted, entityTypes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Slug < sorted[j].Slug })

	models := make([]*entityModel, 0, len(sorted))
	for _, et := range sorted {
		m, err := BuildEntityModel(et)
		if err != nil {
			return nil, fmt.Errorf("gocode: GenerateAllFromGraph: %w", err)
		}
		models = append(models, m)
	}

	// gates is built BEFORE reqModels (task #209): a field atom on a
	// kind:reference field needs to look up its own already-built pipeline
	// gate (findPipelineGate, pipeline.go) while BuildRequirementModels
	// classifies that same field's claim match — the dependency runs gates ->
	// reqModels, not the other way around.
	gates, err := BuildPipelineGateModels(models, requirements)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateAllFromGraph: %w", err)
	}

	reqModels, err := BuildRequirementModels(requirements, models, gates)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateAllFromGraph: %w", err)
	}

	entitiesSrc, err := RenderEntitiesFile("gen", sorted)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateAllFromGraph: %w", err)
	}
	lifecycleSrc, err := RenderLifecycleFile("gen", models)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateAllFromGraph: %w", err)
	}
	lifecycleTestSrc, err := RenderLifecycleTestFile("gen", models)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateAllFromGraph: %w", err)
	}
	requirementsTestSrc, err := RenderRequirementsTestFile("gen", reqModels)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateAllFromGraph: %w", err)
	}
	gateFuncsSrc, err := RenderPipelineFile("gen", gates)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateAllFromGraph: %w", err)
	}
	gateTestsSrc, err := RenderPipelineTestFile("gen", gates)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateAllFromGraph: %w", err)
	}
	pipelineTestSrc := mergePipelineFile(gateFuncsSrc, gateTestsSrc)

	// The one authoritative RenderAuditFile call: models, reqModels, AND
	// gates are all real and fully populated here, so requirements_audit.md
	// carries every EntityType/state/transition-why section, the full
	// "## Requirements" section, and the full "## Pipeline Gates" section in
	// a single render — nothing to overwrite afterward.
	auditSrc, err := RenderAuditFile(models, reqModels, gates)
	if err != nil {
		return nil, fmt.Errorf("gocode: GenerateAllFromGraph: %w", err)
	}

	goMod := renderGoMod(ModuleName(domainDir))

	return map[string][]byte{
		"go.mod":                []byte(goMod),
		"entities.go":           entitiesSrc,
		"lifecycle.go":          lifecycleSrc,
		"lifecycle_test.go":     lifecycleTestSrc,
		"requirements_test.go":  requirementsTestSrc,
		"pipeline_test.go":      pipelineTestSrc,
		"requirements_audit.md": auditSrc,
	}, nil
}

// mergePipelineFile combines the gate-function half (RenderPipelineFile) and
// the table-driven-test half (RenderPipelineTestFile) — each a
// self-contained, independently ownership-marked/package-clause'd Go source
// string — into the single pipeline_test.go contract §1 names: one ownership
// marker, one package clause, the union of both halves' imports (each half
// omits its own import when it has nothing to import - RenderPipelineFile/
// RenderPipelineTestFile's zero-gates branch - so this merge only imports
// "fmt"/"testing" when the corresponding half actually needs it, never
// leaving an unused import that would fail `go build`/`go vet` on an empty
// domain), then the gate functions followed by the test functions. Both
// halves declare `package gen` with no other top-level declarations sharing
// a name (gate funcs are Gate*, tests are Test<Gate*>), so simple textual
// splicing after each half's own header is safe and never produces a naming
// collision.
func mergePipelineFile(gateFuncsSrc, gateTestsSrc []byte) []byte {
	hasGates := strings.Contains(string(gateFuncsSrc), "import \"fmt\"")
	hasTests := strings.Contains(string(gateTestsSrc), "import \"testing\"")
	gateBody := stripGoFileHeader(string(gateFuncsSrc))
	testBody := stripGoFileHeader(string(gateTestsSrc))

	var b strings.Builder
	b.WriteString(OwnershipMarker)
	b.WriteString("\n\n")
	b.WriteString("package gen\n\n")
	switch {
	case hasGates && hasTests:
		b.WriteString("import (\n\t\"fmt\"\n\t\"testing\"\n)\n\n")
	case hasGates:
		b.WriteString("import \"fmt\"\n\n")
	case hasTests:
		b.WriteString("import \"testing\"\n\n")
	}
	b.WriteString(gateBody)
	if gateBody != "" && testBody != "" {
		b.WriteString("\n")
	}
	b.WriteString(testBody)

	return []byte(b.String())
}

// stripGoFileHeader removes a rendered file's leading ownership-marker
// comment and package clause — plus its single-line import statement, when
// present — returning only the declarations (or, for the zero-gates case,
// the placeholder comment) that follow. Used by mergePipelineFile to splice
// two independently-rendered halves into one file with a single shared
// header. RenderPipelineFile/RenderPipelineTestFile emit one of two fixed
// shapes: OwnershipMarker, blank line, "package …", blank line, EITHER a
// single `import "…"` line + blank line (when there is at least one
// gate/test to render) OR nothing (the zero-gates branch, which goes
// straight from the package clause's blank line into a placeholder comment)
// — so this looks for an "import " line and, if found, skips through the
// blank line that follows it; if absent, it skips only the fixed
// marker+package prefix instead (not a general-purpose Go source parser: it
// depends on this package's own renderers never emitting a multi-line
// `import (...)` block or reordering this header, both true today).
func stripGoFileHeader(src string) string {
	lines := strings.Split(src, "\n")
	importIdx := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "import ") {
			importIdx = i
			break
		}
	}
	if importIdx == -1 {
		// No import line (zero-gates branch): the fixed prefix is exactly
		// OwnershipMarker, blank line, "package …", blank line - 4 lines.
		if len(lines) < 5 {
			return ""
		}
		return strings.Join(lines[4:], "\n")
	}
	if importIdx+1 >= len(lines) {
		return ""
	}
	bodyLines := lines[importIdx+1:]
	if len(bodyLines) > 0 && bodyLines[0] == "" {
		bodyLines = bodyLines[1:]
	}
	return strings.Join(bodyLines, "\n")
}

func renderGoMod(moduleName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "module %s\n\n", moduleName)
	fmt.Fprintf(&b, "go %s\n", EngineGoVersion)
	return b.String()
}

package gocode

import (
	"fmt"
	"path/filepath"
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

func renderGoMod(moduleName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "module %s\n\n", moduleName)
	fmt.Fprintf(&b, "go %s\n", EngineGoVersion)
	return b.String()
}

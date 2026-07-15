package main

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/PHPCraftdream/HotamSpec/internal/generator/gocode"
)

// cmdGenCode is the CLI entry point for `hotam gen-code` (GEN-CODE-CONTRACT.md
// §1): it loads a domain's graph.json and writes the generated Go model/
// lifecycle/requirements/pipeline layer into <domainDir>/gen/go/. Unlike
// gen-spec, this command takes only --domain — none of the four
// Generate*FromGraph functions in internal/generator/gocode accept a
// today/profile parameter (their output is a pure function of the graph's
// EntityTypes/Requirements alone), so no --today/--profile flags exist here.
func cmdGenCode(args []string) error {
	fs := newFlagSet("gen-code")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}

	written, err := genCode(domainDir)
	if err != nil {
		return err
	}
	for _, p := range written {
		fmt.Println(relPathForDisplay(p))
	}
	return nil
}

// genCode loads the domain graph at domainDir and renders the full gen-code
// output set (GEN-CODE-CONTRACT.md §1: go.mod, entities.go, lifecycle.go,
// lifecycle_test.go, requirements_test.go, requirements_audit.md,
// pipeline_test.go) into <domainDir>/gen/go/, returning the sorted list of
// absolute paths written.
//
// The four Generate*FromGraph functions (gocode package, stages 2-5) each
// return their own map[string][]byte keyed by filename; several of them
// independently render requirements_audit.md (lifecycle/requirements/
// pipeline each contribute their own section — see RenderAuditFile's models/
// reqModels/gates parameters). Contract §1 names exactly one
// requirements_audit.md in the final output, so later stages' renders
// intentionally OVERWRITE earlier ones in the merged map below — the
// merge order (models, lifecycle, requirements, pipeline) mirrors the
// generation stage order, and only the pipeline stage's render (built from
// the fullest set of inputs: models AND gates) is not literally "more
// complete" than requirements' own render for the requirements section, so
// this is revisited if the two audit renders ever diverge in coverage; today
// each stage's RenderAuditFile call is passed nil for the artifacts it does
// not own (see gocode.go), so whichever stage runs last simply re-renders
// the same audit content contributed by the earlier stages plus its own new
// section skipping nothing — safe to take the last writer as final.
//
// A domain whose EntityTypes/Requirements legitimately produce zero content
// for a given generator stage (e.g. zero pipeline gates because no
// reference-typed fields exist) is not an error — the corresponding
// Generate*FromGraph call still succeeds and returns whatever real files it
// has (per GEN-CODE-CONTRACT.md §2's "generator obligated to explicitly
// refuse [only on genuinely unknown graph elements]" principle); this
// function never synthesizes an empty placeholder file for content that
// generation legitimately decided not to produce.
func genCode(domainDir string) ([]string, error) {
	g, err := loadGraphOrEmpty(domainDir)
	if err != nil {
		return nil, err
	}

	files := make(map[string][]byte)

	modelFiles, err := gocode.GenerateModelsFromGraph(domainDir, g.EntityTypes)
	if err != nil {
		return nil, fmt.Errorf("gen-code: models: %w", err)
	}
	for name, content := range modelFiles {
		files[name] = content
	}

	lifecycleFiles, err := gocode.GenerateLifecycleFromGraph(g.EntityTypes)
	if err != nil {
		return nil, fmt.Errorf("gen-code: lifecycle: %w", err)
	}
	for name, content := range lifecycleFiles {
		files[name] = content
	}

	requirementFiles, err := gocode.GenerateRequirementsFromGraph(g.EntityTypes, g.Requirements)
	if err != nil {
		return nil, fmt.Errorf("gen-code: requirements: %w", err)
	}
	for name, content := range requirementFiles {
		files[name] = content
	}

	pipelineFiles, err := gocode.GeneratePipelineFromGraph(g.EntityTypes)
	if err != nil {
		return nil, fmt.Errorf("gen-code: pipeline: %w", err)
	}
	for name, content := range pipelineFiles {
		files[name] = content
	}

	genGoDir := filepath.Join(domainDir, "gen", "go")
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	paths := make([]string, len(names))
	contents := make([][]byte, len(names))
	for i, name := range names {
		paths[i] = filepath.Join(genGoDir, name)
		contents[i] = files[name]
	}
	if err := writeFilesParallel(paths, contents); err != nil {
		return nil, err
	}

	return paths, nil
}

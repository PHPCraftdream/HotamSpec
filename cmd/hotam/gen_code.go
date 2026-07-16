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
// This calls gocode.GenerateAllFromGraph, the single function that builds
// every []*entityModel/[]*requirementModel/[]*pipelineGateModel exactly ONCE
// and renders requirements_audit.md exactly ONCE, from all three fully
// populated (contract §0: one source of truth). Previously this function
// called the four stage-scoped Generate*FromGraph functions separately and
// merged their four file maps by filename; each of those functions renders
// its OWN partial requirements_audit.md (lifecycle passes reqModels=nil,
// requirements passes gates=nil, pipeline passes reqModels=nil — see
// gocode.go/audit.go), so merging by last-write-wins silently discarded the
// "## Requirements" section entirely (pipeline, the last stage to run, has
// no reqModels to contribute) — every "// Atom: ... - see
// requirements_audit.md#<anchor>" comment tied to a requirement then pointed
// at a heading that did not exist in the file actually written to disk.
// GenerateAllFromGraph is the fix: one render, all three inputs real.
//
// A domain whose EntityTypes/Requirements legitimately produce zero content
// for a given generator stage (e.g. zero pipeline gates because no
// reference-typed fields exist) is not an error — GenerateAllFromGraph still
// succeeds and returns whatever real files it has (per
// GEN-CODE-CONTRACT.md §2's "generator obligated to explicitly refuse [only
// on genuinely unknown graph elements]" principle); this function never
// synthesizes an empty placeholder file for content that generation
// legitimately decided not to produce.
func genCode(domainDir string) ([]string, error) {
	g, err := loadGraphOrEmpty(domainDir)
	if err != nil {
		return nil, err
	}

	files, err := gocode.GenerateAllFromGraph(g.EntityTypes, g.Requirements, g.Processes, domainDir)
	if err != nil {
		return nil, fmt.Errorf("gen-code: %w", err)
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

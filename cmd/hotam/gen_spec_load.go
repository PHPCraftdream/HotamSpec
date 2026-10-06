package main

import (
	"errors"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
	"github.com/PHPCraftdream/HotamSpec/internal/selfspec"
	"os"
	"path/filepath"
	"strings"
)

// gen_spec_load.go loads the domain graph and picks the atom-requirement override source for gen-spec.

// atomOverrides picks the atom-requirement overrides source for gen-spec.
// Self-hosting domains mirror the in-process selfspec.Requirements registry
// (same reference sync-self uses); consumer domains shell out to the domain's
// spec/ dump as before.
func atomOverrides(g *ontology.Graph, domainDir string) (*registry.Registry[ontology.Requirement], error) {
	if g.SelfHosting {
		return selfspec.Requirements, nil
	}
	return domainRegistryFromSubprocess(domainDir)
}

// requiresAtomDiscovered reports whether a non-REJECTED requirement must be
// present in the discovered-atom registry. Empty packages (consumer domains)
// keep the legacy all-requirements-must-be-atoms behavior; otherwise only
// requirements implemented inside a listed self-executing atom package count.
func requiresAtomDiscovered(req ontology.Requirement, packages []string) bool {
	if len(packages) == 0 {
		return true
	}
	for _, entry := range req.ImplementedBy {
		file, _, ok := gate.ParseFileColonSymbol(entry)
		if !ok {
			continue
		}
		file = filepath.ToSlash(file)
		for _, pkg := range packages {
			pkg = strings.TrimSuffix(filepath.ToSlash(pkg), "/")
			if file == pkg || strings.HasPrefix(file, pkg+"/") {
				return true
			}
		}
	}
	return false
}

func repoRootForDomain(domainDir string) string {
	if filepath.Base(filepath.Dir(domainDir)) == "domains" {
		return filepath.Dir(filepath.Dir(domainDir))
	}
	if root, err := paths.ProjectRootOrRaise(); err == nil {
		relative, relativeErr := filepath.Rel(root, domainDir)
		if relativeErr == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return root
		}
	}
	return domainDir
}

func loadGraphForGenSpec(domainDir string) (*ontology.Graph, error) {
	graphPath := graphPathForDomain(domainDir)
	if _, err := os.Stat(graphPath); err != nil {
		return loadGraphOrEmpty(domainDir)
	}
	manifest, err := loader.LoadManifest(filepath.Join(domainDir, "manifest.json"))
	if err == nil && manifest.SelfExecutingAtoms {
		return loader.LoadGraphForCodeProjection(graphPath)
	}
	return loadGraphOrEmpty(domainDir)
}

// loadGraphOrEmpty loads the domain's graph.json, mirroring loadDomainGraph,
// but treats a MISSING graph.json (os.IsNotExist) as an empty graph instead of
// a hard error (R-empty-content-gen-notice). An empty graph flows through every
// generator already: the Build* helpers detect g.IsEmpty() and render the calm
// EmptyNotice into docs/gen/*.md, so the missing-file case yields output
// identical to the empty-but-present case. Any error that is NOT os.IsNotExist
// (a decode failure, a permissions error) is a genuine problem and is still
// returned to the caller.
func loadGraphOrEmpty(domainDir string) (*ontology.Graph, error) {
	gp := graphPathForDomain(domainDir)
	g, err := loader.LoadGraph(gp)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &ontology.Graph{DomainDir: domainDir}, nil
		}
		return nil, err
	}
	return g, nil
}

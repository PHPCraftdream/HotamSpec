// spec_build.go holds the data-collection AND rendering machinery behind
// docs/gen/SPEC.md: the generated NORMATIVE TEXT projection
// PLAN-scenario-generated-spec.md §2 D2/§3 W1.3 names — the successor stage
// to the authored-spec discipline's own projections (traceability.go/
// models.go/coverage.go, internal/generator): a requirement's claim (still
// the short AUTHORED intent from graph.json, D2 — never invented here)
// followed by the GENERATED prose narrative of its verified_by scenario(s),
// rendered from the ACTUAL Given/When/Then/Value steps a real, passing
// `go test` run just recorded via internal/recorder/canon's hotamspec API
// (PLAN-scenario-generated-spec.md §1's "text incarnates the actually-run
// test", never a second, independently-writable source of truth).
//
// LAYERING: collection and rendering live here so internal/invariants can
// consume SPEC freshness without importing internal/generator and closing its
// existing diagnose -> invariants dependency cycle. This package remains a
// leaf over loader, ontology, localization, and docbundle; generator and CLI
// entry points share the same rows/renderers through their wrappers.
//
// Unlike traceability.go/models.go/coverage.go, SPEC includes real execution
// evidence. Legacy scenario graphs record each verified_by test through
// RunVerifiedByTestRecording; self-executing atom graphs record each distinct
// source package once through RunAtomPackageRecording, preserving per-test and
// per-subtest verdicts/artifacts in one invocation-local snapshot.
//
// This file is read-only over the graph and re-EXECUTES the domain's own
// authored code via `go test` purely to observe what a real scenario
// narrated; it never mutates the graph, never writes to the domain's spec/
// tree. check_spec_md_current (internal/invariants/spec_md_current.go, W2.3)
// is the mechanical staleness gate that consumes this file's output; this
// file itself only renders what a fresh run reports NOW.
package gate

import (
	"fmt"
	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"path/filepath"
	"sort"
	"strings"
)

// spec_shards.go writes the per-package SPEC shards and their index for multilingual bundles.

// SpecPackage identifies the source package that owns a requirement.
func SpecPackage(r ontology.Requirement) string {
	file := firstImplementedByFile(r)
	if file == "" && len(r.VerifiedBy) > 0 {
		file, _, _ = ParseFileColonSymbol(r.VerifiedBy[0])
	}
	if file == "" {
		for _, caseDef := range r.Cases {
			if caseFile, _, ok := ParseFileColonSymbol(caseDef.Test); ok {
				file = caseFile
				break
			}
		}
	}
	pkg := strings.TrimPrefix(filepath.ToSlash(filepath.Dir(file)), "./")
	if pkg == "." || pkg == "" {
		return "root"
	}
	return pkg
}

func specTestReferences(requirement ontology.Requirement) []string {
	references := make([]string, 0, len(requirement.VerifiedBy)+len(requirement.Cases))
	seen := make(map[string]bool, len(requirement.VerifiedBy)+len(requirement.Cases))
	add := func(reference string) {
		reference = strings.TrimSpace(reference)
		if reference != "" && !seen[reference] {
			seen[reference] = true
			references = append(references, reference)
		}
	}
	for _, reference := range requirement.VerifiedBy {
		add(reference)
	}
	for _, caseDef := range requirement.Cases {
		if caseDef.Test == "" || caseTestCoveredByVerifiedBy(caseDef.Test, requirement.VerifiedBy) {
			continue
		}
		add(caseDef.Test)
	}
	return references
}

func caseTestCoveredByVerifiedBy(caseTest string, verifiedBy []string) bool {
	caseFile, caseName, caseOK := ParseFileColonSymbol(strings.TrimSpace(caseTest))
	if !caseOK {
		return false
	}
	for _, reference := range verifiedBy {
		file, test, ok := ParseFileColonSymbol(strings.TrimSpace(reference))
		if ok && file == caseFile && (caseName == test || strings.HasPrefix(caseName, test+"/")) {
			return true
		}
	}
	return false
}

// BuildSpecDocumentsFromRows strictly renders a full language bundle from one
// already-collected execution snapshot. No output is returned on any catalog
// or claim translation error.
func BuildSpecDocumentsFromRows(g *ontology.Graph, rows map[string]SpecRow) (map[string]string, error) {
	if hasSpecDocument(g) {
		return buildNormativeSpecDocuments(g, rows)
	}
	layout, err := docbundle.NewLayout(g.Languages, g.DefaultLanguage)
	if err != nil {
		return nil, err
	}
	hasAtoms := g.SelfExecutingAtoms
	includePackages := hasAtoms || layout.Multilingual()
	groups := make(map[string][]ontology.Requirement)
	if includePackages {
		for _, req := range g.Requirements {
			if req.Status != ontology.StatusREJECTED {
				groups[SpecPackage(req)] = append(groups[SpecPackage(req)], req)
			}
		}
	}
	packages := make([]string, 0, len(groups))
	for pkg := range groups {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	packagePaths := make([]string, 0, len(packages))
	for _, pkg := range packages {
		packagePaths = append(packagePaths, pkg+".md")
	}
	if _, err := docbundle.SpecShardNames(packagePaths); err != nil {
		return nil, err
	}
	languages := layout.LanguagesForViews()
	docs := make(map[string]string, len(packages)*len(languages)+len(languages))
	for _, language := range languages {
		indexPath, err := layout.SpecIndexPath(language)
		if err != nil {
			return nil, err
		}
		indexKey, err := specDocGenKey(indexPath)
		if err != nil {
			return nil, err
		}
		view := *g
		view.RenderLanguage = language
		if !includePackages {
			document, err := BuildSpecFromRowsForLanguage(&view, rows, language)
			if err != nil {
				return nil, err
			}
			docs[indexKey] = document
			continue
		}
		shardPaths := make(map[string]string, len(packages))
		for _, pkg := range packages {
			shardPath, err := layout.SpecShardPath(language, pkg+".md")
			if err != nil {
				return nil, err
			}
			shardPaths[pkg] = shardPath
			shardKey, err := specDocGenKey(shardPath)
			if err != nil {
				return nil, err
			}
			shard := view
			shard.Requirements = groups[pkg]
			document, err := BuildSpecFromRowsForLanguage(&shard, rows, language)
			if err != nil {
				return nil, err
			}
			docs[shardKey] = document
		}
		if g.Conformance != nil && g.Conformance.RuleCases {
			// Render every rule at the root; shards remain focused reading views.
			view.SelfExecutingAtoms = false
			document, err := BuildSpecFromRowsForLanguage(&view, rows, language)
			if err != nil {
				return nil, err
			}
			docs[indexKey] = document
			continue
		}
		index, err := buildSpecPackageIndex(&view, language, indexPath, packages, shardPaths, groups)
		if err != nil {
			return nil, err
		}
		docs[indexKey] = index
	}
	return docs, nil
}

// Document-enabled domains publish a complete canonical root and package-focused
// reading views. Shards are projections of authored blocks, not test journals.
func buildNormativeSpecDocuments(g *ontology.Graph, rows map[string]SpecRow) (map[string]string, error) {
	projection, err := prepareSpecDocument(g, rows)
	if err != nil {
		return nil, err
	}
	layout, err := docbundle.NewLayout(g.Languages, g.DefaultLanguage)
	if err != nil {
		return nil, err
	}
	blockIDsByClause := make(map[string][]string)
	for _, section := range projection.sections {
		for _, block := range section.Blocks {
			for _, clauseID := range block.ClauseIDs {
				blockIDsByClause[clauseID] = append(blockIDsByClause[clauseID], block.ID)
			}
		}
	}
	groups := make(map[string]map[string]bool)
	for _, requirement := range g.Requirements {
		if requirement.Status == ontology.StatusREJECTED {
			continue
		}
		pkg := SpecPackage(requirement)
		for _, link := range requirement.ClauseLinks {
			for _, blockID := range blockIDsByClause[link.ClauseID] {
				if groups[pkg] == nil {
					groups[pkg] = make(map[string]bool)
				}
				groups[pkg][blockID] = true
			}
		}
	}
	packages := make([]string, 0, len(groups))
	packagePaths := make([]string, 0, len(groups))
	for pkg := range groups {
		packages = append(packages, pkg)
		packagePaths = append(packagePaths, pkg+".md")
	}
	sort.Strings(packages)
	if _, err := docbundle.SpecShardNames(packagePaths); err != nil {
		return nil, err
	}
	documents := make(map[string]string)
	for _, language := range layout.LanguagesForViews() {
		canonicalPath, err := layout.SpecIndexPath(language)
		if err != nil {
			return nil, err
		}
		root, err := renderSpecDocument(g, projection, language, canonicalPath, canonicalPath, nil)
		if err != nil {
			return nil, err
		}
		key, err := specDocGenKey(canonicalPath)
		if err != nil {
			return nil, err
		}
		documents[key] = root
		for _, pkg := range packages {
			shardPath, err := layout.SpecShardPath(language, pkg+".md")
			if err != nil {
				return nil, err
			}
			shard, err := renderSpecDocument(g, projection, language, shardPath, canonicalPath, groups[pkg])
			if err != nil {
				return nil, err
			}
			key, err := specDocGenKey(shardPath)
			if err != nil {
				return nil, err
			}
			documents[key] = shard
		}
	}
	return documents, nil
}

func specDocGenKey(domainPath string) (string, error) {
	normalized := filepath.ToSlash(filepath.Clean(filepath.FromSlash(domainPath)))
	const prefix = "docs/gen/"
	if !strings.HasPrefix(normalized, prefix) {
		return "", fmt.Errorf("SPEC bundle path %q is outside docs/gen", domainPath)
	}
	return strings.TrimPrefix(normalized, prefix), nil
}

func buildSpecPackageIndex(g *ontology.Graph, language, indexPath string, packages []string, shardPaths map[string]string, groups map[string][]ontology.Requirement) (string, error) {
	banner, err := specText(language, specBanner)
	if err != nil {
		return "", err
	}
	reader, err := specReaderHeaderLine(g, language)
	if err != nil {
		return "", err
	}
	title, err := specText(language, "# SPEC.md — package index")
	if err != nil {
		return "", err
	}
	header, err := specText(language, "| Package | Requirements |")
	if err != nil {
		return "", err
	}
	lines := []string{banner}
	if reader != "" {
		lines = append(lines, reader)
	}
	lines = append(lines, "", title, "", header, "|---|---:|")
	for _, pkg := range packages {
		link, err := filepath.Rel(filepath.Dir(filepath.FromSlash(indexPath)), filepath.FromSlash(shardPaths[pkg]))
		if err != nil {
			return "", err
		}
		row, err := specText(language, "| [%s](%s) | %d |", specCell(pkg), filepath.ToSlash(link), len(groups[pkg]))
		if err != nil {
			return "", err
		}
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n") + "\n", nil
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/evidence"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/localization"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func init() {
	invariants.All.Update("check_language_outputs_current", *withCheckLanguageOutputsCurrent(mustGetInvariant("check_language_outputs_current")))
}

func withCheckLanguageOutputsCurrent(inv invariants.Invariant) *invariants.Invariant {
	inv.PostProcessCheck = func(g *ontology.Graph, prior []invariants.Violation) []invariants.Violation {
		return checkLanguageOutputsCurrentReal(g, prior, time.Now().Format("2006-01-02"))
	}
	inv.PostProcessCheckAsOf = checkLanguageOutputsCurrentReal
	return &inv
}

func checkLanguageOutputsCurrentReal(g *ontology.Graph, prior []invariants.Violation, today string) []invariants.Violation {
	var violations []invariants.Violation
	err := localization.SafeRender(func() error {
		violations = checkLanguageOutputsCurrentStaged(g, prior, today)
		return nil
	})
	if err != nil {
		domainDir := ""
		if g != nil {
			domainDir = g.DomainDir
		}
		return []invariants.Violation{{Check: "check_language_outputs_current", ID: domainDir, Message: fmt.Sprintf("localized rendering is incomplete: %v", err)}}
	}
	return violations
}

func checkLanguageOutputsCurrentStaged(g *ontology.Graph, prior []invariants.Violation, today string) []invariants.Violation {
	if g == nil || len(g.Languages) == 0 || g.DomainDir == "" {
		return nil
	}
	// Every comparative render below (the localized document bundle and the
	// localized boot crystals) must be fed the SAME publication flavor genSpec
	// writes with — invariants.PublicationViolationsFromPhaseOne of the phase-1
	// list, THE one shared flavor selector (review finding P3-13). Feeding the
	// unfiltered prior would report false staleness whenever an unrelated
	// on-disk projection (e.g. a tampered ENGINE-VERSION stamp) is transiently
	// stale, even though a re-run rewrites every file byte-identically.
	publication := invariants.PublicationViolationsFromPhaseOne(prior)
	layout, err := docbundle.NewLayout(g.Languages, g.DefaultLanguage)
	if err != nil {
		return []invariants.Violation{{Check: "check_language_outputs_current", ID: g.DomainDir, Message: fmt.Sprintf("invalid language output layout: %v", err)}}
	}
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}
	repoRoot := repoRootForDomain(g.DomainDir)
	genDir := filepath.Join(g.DomainDir, "docs", "gen")
	includeSpec, err := languageSpecBundleRequired(g, layout, genDir)
	if err != nil {
		return []invariants.Violation{{Check: "check_language_outputs_current", ID: g.DomainDir, Message: fmt.Sprintf("inspect SPEC bundle inventory: %v", err)}}
	}

	reportPresent, rawReportPresent, err := evidenceReportBundleState(genDir)
	if err != nil {
		return []invariants.Violation{{Check: "check_language_outputs_current", ID: filepath.Join(genDir, "evidence.json"), Message: fmt.Sprintf("inspect shared evidence report store: %v", err)}}
	}
	shareEvidenceSnapshot := reportPresent || invariants.ConformanceAuditRequired(g)
	needExecutionSnapshot := g.SelfExecutingAtoms || shareEvidenceSnapshot
	graphView := g
	var atomSnapshot *gate.AtomExecutionSnapshot
	if needExecutionSnapshot {
		graphView, atomSnapshot, err = invariants.InvocationExecutionSnapshot(g)
		if err != nil {
			return []invariants.Violation{{Check: "check_language_outputs_current", ID: g.DomainDir, Message: fmt.Sprintf("collect one atom execution snapshot: %v", err)}}
		}
	}
	var reportSnapshot evidence.Report
	if shareEvidenceSnapshot {
		reportSnapshot, err = invariants.InvocationEvidenceSnapshot(graphView)
		if err != nil {
			return []invariants.Violation{{Check: "check_language_outputs_current", ID: g.DomainDir, Message: fmt.Sprintf("project shared evidence snapshot: %v", err)}}
		}
		if (reportPresent || includeSpec) && reportSnapshot.SchemaVersion <= 0 {
			return []invariants.Violation{{Check: "check_language_outputs_current", ID: g.DomainDir, Message: "evidence collector returned no valid report schema"}}
		}
	}
	specDocs := make(map[string]string)
	if includeSpec {
		var rows map[string]gate.SpecRow
		if shareEvidenceSnapshot {
			rows = reportSnapshot.SpecRows
		} else if atomSnapshot != nil {
			rows = gate.CollectSpecRowsFromSnapshot(graphView, atomSnapshot)
		} else {
			rows = gate.CollectSpecRows(g)
		}
		specDocs, err = gate.BuildSpecDocumentsFromRows(graphView, rows)
		if err != nil {
			return []invariants.Violation{{Check: "check_language_outputs_current", ID: g.DomainDir, Message: fmt.Sprintf("render complete language SPEC bundle: %v", err)}}
		}
	} else {
		specDocs, err = existingSpecDocsForLanguageViews(genDir, layout)
		if err != nil {
			return []invariants.Violation{{Check: "check_language_outputs_current", ID: g.DomainDir, Message: fmt.Sprintf("read existing SPEC bundle for localized navigation: %v", err)}}
		}
	}

	crystalPaths := localizedCrystalPaths(layout, resolveClaudeMDPath(g.DomainDir, ""))
	localized, err := generator.BuildLocalizedDocumentsWithSnapshotAndCrystalPathsForProfile(
		graphView, filepath.Base(graphView.DomainDir), repoRoot, today, publication, specDocs, crystalPaths, loader.ResolveGenProfile(graphPathForDomain(graphView.DomainDir)),
	)
	if err != nil {
		return []invariants.Violation{{Check: "check_language_outputs_current", ID: g.DomainDir, Message: fmt.Sprintf("render complete localized document bundle: %v", err)}}
	}

	expected := make(map[string]string, len(localized)+len(specDocs)+8)
	add := func(path, content string) error {
		clean := filepath.Clean(path)
		if _, exists := expected[clean]; exists {
			return fmt.Errorf("multiple renderers produced %s", clean)
		}
		expected[clean] = content
		return nil
	}
	for relative, content := range localized {
		path, err := docbundle.ResolveOutputPath(repoRoot, g.DomainDir, relative)
		if err != nil {
			return []invariants.Violation{{Check: "check_language_outputs_current", ID: relative, Message: fmt.Sprintf("localized renderer returned an unowned output path: %v", err)}}
		}
		if err := add(path, content); err != nil {
			return []invariants.Violation{{Check: "check_language_outputs_current", ID: path, Message: err.Error()}}
		}
	}
	if includeSpec {
		for relative, content := range specDocs {
			path := filepath.Join(genDir, filepath.FromSlash(relative))
			if err := add(path, content); err != nil {
				return []invariants.Violation{{Check: "check_language_outputs_current", ID: path, Message: err.Error()}}
			}
		}
	}

	var violations []invariants.Violation
	if reportPresent {
		if !rawReportPresent {
			violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: filepath.Join(genDir, "evidence.json"), Message: "localized evidence views exist without the one shared evidence.json store; run `hotam evidence --write`"})
		}
		for _, language := range layout.LanguagesForViews() {
			view := *graphView
			view.RenderLanguage = language
			evidenceMD, err := generator.BuildEvidenceLocalized(&view, reportSnapshot, language)
			if err != nil {
				return []invariants.Violation{{Check: "check_language_outputs_current", ID: genDir, Message: fmt.Sprintf("render localized EVIDENCE view for %q: %v", language, err)}}
			}
			findingsMD, err := generator.BuildFindingsLocalized(&view, reportSnapshot, language)
			if err != nil {
				return []invariants.Violation{{Check: "check_language_outputs_current", ID: genDir, Message: fmt.Sprintf("render localized FINDINGS view for %q: %v", language, err)}}
			}
			evidencePath, err := layout.DocumentPath("docs/gen/EVIDENCE.md", language)
			if err != nil {
				return []invariants.Violation{{Check: "check_language_outputs_current", ID: g.DomainDir, Message: err.Error()}}
			}
			findingsPath, err := layout.DocumentPath("docs/gen/FINDINGS.md", language)
			if err != nil {
				return []invariants.Violation{{Check: "check_language_outputs_current", ID: g.DomainDir, Message: err.Error()}}
			}
			if err := add(filepath.Join(g.DomainDir, filepath.FromSlash(evidencePath)), evidenceMD); err != nil {
				return []invariants.Violation{{Check: "check_language_outputs_current", ID: evidencePath, Message: err.Error()}}
			}
			if err := add(filepath.Join(g.DomainDir, filepath.FromSlash(findingsPath)), findingsMD); err != nil {
				return []invariants.Violation{{Check: "check_language_outputs_current", ID: findingsPath, Message: err.Error()}}
			}
		}
	}
	violations = append(violations, compareLanguageOutputs(expected)...)
	violations = append(violations, obsoleteLanguageOutputs(genDir, expected, layout)...)
	violations = append(violations, checkLocalizedCrystalsCurrent(graphView, layout, repoRoot, publication, today)...)
	if !hasOtherFullFrameworkOwner(repoRoot, g.DomainDir) {
		violations = append(violations, obsoleteProjectLanguageOutputs(filepath.Join(repoRoot, "framework"), expected, layout)...)
	}
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].ID != violations[j].ID {
			return violations[i].ID < violations[j].ID
		}
		return violations[i].Message < violations[j].Message
	})
	return violations
}

func languageSpecBundleRequired(g *ontology.Graph, layout docbundle.Layout, genDir string) (bool, error) {
	if layout.Multilingual() || g.SelfExecutingAtoms || g.Discipline == loader.DisciplineFull {
		return true, nil
	}
	candidates, err := docbundle.DomainCandidates(genDir)
	if err != nil {
		return false, err
	}
	for _, path := range candidates {
		if !layout.IsSpecPath(genDir, path) {
			continue
		}
		content, err := os.ReadFile(path)
		if err == nil {
			if gate.IsGeneratedSpecDocument(string(content)) {
				return true, nil
			}
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
}

func existingSpecDocsForLanguageViews(genDir string, layout docbundle.Layout) (map[string]string, error) {
	candidates, err := docbundle.DomainCandidates(genDir)
	if err != nil {
		return nil, err
	}
	documents := make(map[string]string)
	for _, path := range candidates {
		if !layout.IsSpecPath(genDir, path) {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !gate.IsGeneratedSpecDocument(string(content)) {
			continue
		}
		relative, err := filepath.Rel(genDir, path)
		if err != nil {
			return nil, err
		}
		documents[filepath.ToSlash(relative)] = string(content)
	}
	return documents, nil
}

func evidenceReportBundleState(genDir string) (present, rawPresent bool, err error) {
	rawPath := filepath.Join(genDir, "evidence.json")
	info, err := os.Stat(rawPath)
	if err == nil {
		present = true
		rawPresent = info.Mode().IsRegular()
	} else if !os.IsNotExist(err) {
		return false, false, err
	}
	for _, path := range docbundle.ReportCandidates(genDir) {
		if filepath.Clean(path) == filepath.Clean(rawPath) {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			present = true
		} else if !os.IsNotExist(err) {
			return present, rawPresent, err
		}
	}
	return present, rawPresent, nil
}

func compareLanguageOutputs(expected map[string]string) []invariants.Violation {
	paths := make([]string, 0, len(expected))
	for path := range expected {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var violations []invariants.Violation
	for _, path := range paths {
		actual, err := os.ReadFile(path)
		if err != nil || string(actual) != expected[path] {
			violations = append(violations, invariants.Violation{
				Check:   "check_language_outputs_current",
				ID:      path,
				Message: fmt.Sprintf("localized output is missing, unreadable, or stale (read error: %v); run `hotam gen-spec` and `hotam evidence --write` to publish the complete language bundle", err),
			})
		}
	}
	return violations
}

func obsoleteLanguageOutputs(genDir string, expected map[string]string, layout docbundle.Layout) []invariants.Violation {
	candidates, err := docbundle.DomainCandidates(genDir)
	if err != nil {
		return []invariants.Violation{{Check: "check_language_outputs_current", ID: genDir, Message: fmt.Sprintf("inspect owned localized output inventory: %v", err)}}
	}
	var violations []invariants.Violation
	for _, path := range candidates {
		if filepath.Base(path) == "graph.json" || filepath.Base(path) == "evidence.json" {
			continue
		}
		if _, current := expected[filepath.Clean(path)]; current {
			continue
		}
		relative, err := filepath.Rel(genDir, path)
		if err != nil {
			violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: path, Message: fmt.Sprintf("classify owned localized output: %v", err)})
			continue
		}
		relative = filepath.ToSlash(relative)
		if docbundle.UnknownLocaleSuffixedMarkdown(relative) {
			continue
		}
		var owns func(string) bool
		if docbundle.IsSpecOutputPath(relative) {
			owns = gate.IsGeneratedSpecDocument
		} else if reportPredicate := generatedReportDocumentPredicate(relative); reportPredicate != nil {
			owns = reportPredicate
		} else if language, localized := docbundle.LocalizedOutputLanguage(relative); localized {
			owns = func(content string) bool { return generator.IsGeneratedLocalizedDocument(language, content) }
		} else if language, localized := localizedLayoutBaseLanguage(layout); localized && filepath.Dir(relative) == "." && filepath.Ext(relative) == ".md" {
			owns = func(content string) bool { return generator.IsGeneratedLocalizedDocument(language, content) }
		}
		if owns != nil {
			exists, generated, err := inspectGeneratedOutput(path, owns)
			if err != nil {
				violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: path, Message: fmt.Sprintf("inspect owned localized output: %v", err)})
				continue
			}
			if !exists || !generated {
				continue
			}
		} else if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: path, Message: fmt.Sprintf("inspect owned localized output: %v", err)})
			continue
		}
		violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: path, Message: "obsolete owned localized output remains; run `hotam gen-spec` or `hotam evidence --write`"})
	}
	return violations
}

func obsoleteProjectLanguageOutputs(frameworkDir string, expected map[string]string, layout docbundle.Layout) []invariants.Violation {
	candidates, err := docbundle.ProjectCandidates(frameworkDir)
	if err != nil {
		return []invariants.Violation{{Check: "check_language_outputs_current", ID: frameworkDir, Message: fmt.Sprintf("inspect owned project language outputs: %v", err)}}
	}
	var violations []invariants.Violation
	for _, path := range candidates {
		if _, current := expected[filepath.Clean(path)]; current {
			continue
		}
		relative, err := filepath.Rel(filepath.Dir(frameworkDir), path)
		if err != nil {
			violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: path, Message: fmt.Sprintf("classify project language output: %v", err)})
			continue
		}
		relative = filepath.ToSlash(relative)
		if docbundle.UnknownLocaleSuffixedMarkdown(relative) {
			continue
		}
		if language, localized := docbundle.LocalizedOutputLanguage(relative); localized {
			exists, owned, err := inspectGeneratedOutput(path, func(content string) bool {
				return generator.IsGeneratedLocalizedDocument(language, content)
			})
			if err != nil {
				violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: path, Message: fmt.Sprintf("inspect owned project language output: %v", err)})
				continue
			}
			if !exists || !owned {
				continue
			}
		} else if relative == "framework/GLOSSARY.md" {
			if language, localized := localizedLayoutBaseLanguage(layout); localized {
				exists, owned, err := inspectGeneratedOutput(path, func(content string) bool {
					return generator.IsGeneratedLocalizedDocument(language, content)
				})
				if err != nil {
					violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: path, Message: fmt.Sprintf("inspect owned project language output: %v", err)})
					continue
				}
				if !exists || !owned {
					continue
				}
			}
		}
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: path, Message: fmt.Sprintf("inspect owned project language output: %v", err)})
			continue
		}
		violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: path, Message: "obsolete owned project language output remains; run `hotam gen-spec`"})
	}
	return violations
}

func hasOtherFullFrameworkOwner(repoRoot, activeDomainDir string) bool {
	entries, err := os.ReadDir(filepath.Join(repoRoot, "domains"))
	if err != nil {
		return false
	}
	active := filepath.Clean(activeDomainDir)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), "_") {
			continue
		}
		domainDir := filepath.Join(repoRoot, "domains", entry.Name())
		if filepath.Clean(domainDir) == active {
			continue
		}
		if loader.ResolveGenProfile(filepath.Join(domainDir, "graph.json")) == loader.GenProfileFull {
			return true
		}
	}
	return false
}

func checkLocalizedCrystalsCurrent(g *ontology.Graph, layout docbundle.Layout, repoRoot string, publication []invariants.Violation, today string) []invariants.Violation {
	// publication (not the raw phase-one list) feeds every comparative crystal
	// render — see checkLanguageOutputsCurrentStaged's flavor-selector comment.
	consumer := loader.ResolveGenProfile(graphPathForDomain(g.DomainDir)) == loader.GenProfileConsumer
	basePath := resolveClaudeMDPath(g.DomainDir, "")
	if basePath == "" {
		return nil
	}
	committed, err := os.ReadFile(basePath)
	if err != nil {
		return nil
	}
	if _, _, ok := generator.SplitAtDurableNotesMarker(string(committed)); !ok {
		return nil
	}
	defaultLanguage := layout.DefaultLanguage
	if len(layout.Languages) == 1 && defaultLanguage == "" {
		defaultLanguage = layout.Languages[0]
	}
	var violations []invariants.Violation
	for _, language := range layout.LanguagesForViews() {
		path := basePath
		if layout.Multilingual() && language != defaultLanguage {
			name, err := layout.CrystalPath(language)
			if err != nil {
				violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: basePath, Message: err.Error()})
				continue
			}
			path = filepath.Join(filepath.Dir(basePath), name)
		}
		view := *g
		view.RenderLanguage = language
		viewGraphs := map[string]*ontology.Graph{domainNameFromDir(g.DomainDir): &view}
		override := &generator.ViolationsOverride{For: &view, Violations: publication}
		charCount, err := generator.ComputeCrystalCharCountFixpointWithViolations(&view, domainNameFromDir(g.DomainDir), repoRoot, viewGraphs, today, consumer, override, path)
		if err != nil {
			violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: path, Message: fmt.Sprintf("compute localized boot crystal: %v", err)})
			continue
		}
		fresh := generator.RenderClaudeMDFromTemplateWithViolations(&view, domainNameFromDir(g.DomainDir), repoRoot, charCount, viewGraphs, today, consumer, override, path)
		freshGenerated, _, freshOK := generator.SplitAtDurableNotesMarker(fresh)
		actual, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: path, Message: "localized boot crystal is missing; run `hotam gen-spec` to render every declared locale"})
			} else {
				violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: path, Message: fmt.Sprintf("read localized boot crystal: %v", err)})
			}
			continue
		}
		actualGenerated, _, actualOK := generator.SplitAtDurableNotesMarker(string(actual))
		if !freshOK || !actualOK || freshGenerated != actualGenerated {
			violations = append(violations, invariants.Violation{Check: "check_language_outputs_current", ID: path, Message: "localized boot crystal generated portion is stale; run `hotam gen-spec`"})
		}
	}
	return violations
}

func localizedCrystalPaths(layout docbundle.Layout, basePath string) map[string]string {
	defaultLanguage := layout.DefaultLanguage
	if len(layout.Languages) == 1 && defaultLanguage == "" {
		defaultLanguage = layout.Languages[0]
	}
	paths := make(map[string]string, len(layout.LanguagesForViews()))
	for _, language := range layout.LanguagesForViews() {
		path := basePath
		if basePath != "" && layout.Multilingual() && language != defaultLanguage {
			name, err := layout.CrystalPath(language)
			if err != nil {
				path = ""
			} else {
				path = filepath.Join(filepath.Dir(basePath), name)
			}
		}
		paths[language] = path
	}
	return paths
}

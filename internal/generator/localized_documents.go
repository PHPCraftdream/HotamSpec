package generator

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/localization"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// BuildLocalizedDocuments renders the graph-derived human-readable projections
// for every declared language without executing tests or mutating the shared
// graph. Returned keys are slash-separated paths relative to repoRoot. SPEC
// documents are deliberately supplied by gate's one shared execution snapshot,
// not generated here.
func BuildLocalizedDocuments(g *ontology.Graph, domainName, repoRoot, today string) (docs map[string]string, err error) {
	if g == nil {
		return nil, fmt.Errorf("build localized documents: nil graph")
	}
	if len(g.Languages) == 0 {
		return nil, nil
	}
	err = localization.SafeRender(func() error {
		docs, err = BuildLocalizedDocumentsWithViolations(g, domainName, repoRoot, today, invariants.PriorToPostProcessViolations(g))
		return err
	})
	return docs, err

}

// BuildLocalizedDocumentsWithViolations renders all localized human-readable
// projections from one invocation-local violation snapshot. Publisher callers
// should provide their overlay-aware snapshot so no locale is based on stale
// on-disk projections. It never executes tests or mutates g.
func BuildLocalizedDocumentsWithViolations(g *ontology.Graph, domainName, repoRoot, today string, violations []invariants.Violation) (map[string]string, error) {
	return BuildLocalizedDocumentsWithSnapshot(g, domainName, repoRoot, today, violations, nil)
}

// BuildLocalizedDocumentsWithSnapshot stages locale-matched REPO-MAP links to
// already-rendered SPEC documents. SPEC values remain the caller's output entries.
func BuildLocalizedDocumentsWithSnapshot(g *ontology.Graph, domainName, repoRoot, today string, violations []invariants.Violation, specDocs map[string]string) (map[string]string, error) {
	return BuildLocalizedDocumentsWithSnapshotAndCrystalPaths(g, domainName, repoRoot, today, violations, specDocs, nil)
}

// BuildLocalizedDocumentsWithSnapshotAndCrystalPaths is the safe staging
// boundary. crystalPaths supplies the actual per-locale boot destinations used
// to compute each view's LIVE-STATE/AGENT-CONTEXT count; an empty value means
// no boot crystal is in scope for that locale.
func BuildLocalizedDocumentsWithSnapshotAndCrystalPaths(g *ontology.Graph, domainName, repoRoot, today string, violations []invariants.Violation, specDocs, crystalPaths map[string]string) (map[string]string, error) {
	return BuildLocalizedDocumentsWithSnapshotAndCrystalPathsForProfile(g, domainName, repoRoot, today, violations, specDocs, crystalPaths, "")
}

// BuildLocalizedDocumentsWithSnapshotAndCrystalPathsForProfile honors the
// gen-spec invocation's resolved profile even when it overrides manifest.json.
func BuildLocalizedDocumentsWithSnapshotAndCrystalPathsForProfile(g *ontology.Graph, domainName, repoRoot, today string, violations []invariants.Violation, specDocs, crystalPaths map[string]string, resolvedProfile string) (docs map[string]string, err error) {
	err = localization.SafeRender(func() error {
		docs, err = buildLocalizedDocumentsWithSnapshot(g, domainName, repoRoot, today, violations, specDocs, crystalPaths, resolvedProfile)
		return err
	})
	return docs, err
}

func buildLocalizedDocumentsWithSnapshot(g *ontology.Graph, domainName, repoRoot, today string, violations []invariants.Violation, specDocs, crystalPaths map[string]string, resolvedProfile string) (map[string]string, error) {
	if g == nil {
		return nil, fmt.Errorf("build localized documents: nil graph")
	}
	if len(g.Languages) == 0 {
		return nil, nil
	}
	layout, err := docbundle.NewLayout(g.Languages, g.DefaultLanguage)
	if err != nil {
		return nil, fmt.Errorf("build localized documents: %w", err)
	}
	languages := layout.LanguagesForViews()
	for _, language := range languages {
		if !localization.Supported(language) {
			return nil, &localization.MissingTranslation{Language: language, Template: Banner}
		}
	}
	consumer := false
	switch resolvedProfile {
	case loader.GenProfileConsumer:
		consumer = true
	case loader.GenProfileFull:
		consumer = false
	case "":
		if g.DomainDir != "" {
			profile := loader.ResolveGenProfile(filepath.Join(g.DomainDir, "graph.json"))
			consumer = profile == loader.GenProfileConsumer
		}
	default:
		return nil, fmt.Errorf("build localized documents: unknown gen-spec profile %q", resolvedProfile)
	}
	outputs := make(map[string]string)
	for _, language := range languages {
		view := *g
		view.RenderLanguage = language
		if err := renderLocalizedView(outputs, &view, layout, domainName, repoRoot, today, consumer, language, violations, specDocs, crystalPaths); err != nil {
			return nil, err
		}
	}
	return outputs, nil
}

func renderLocalizedView(outputs map[string]string, g *ontology.Graph, layout docbundle.Layout, domainName, repoRoot, today string, consumer bool, language string, violations []invariants.Violation, specDocs, crystalPaths map[string]string) error {
	var pathErr error
	localizedPath := func(basePath string) string {
		if pathErr != nil {
			return ""
		}
		path, err := layout.DocumentPath(basePath, language)
		if err != nil {
			pathErr = err
			return ""
		}
		return path
	}
	localizedName := func(filename string) string {
		path := localizedPath("docs/gen/" + filename)
		if path == "" {
			return ""
		}
		return filepath.Base(path)
	}
	localizedOutput := func(content string) string {
		if !layout.Multilingual() {
			return content
		}
		banner := localizedBanner(g)
		if strings.HasPrefix(content, banner+"\n") {
			return content
		}
		header := serviceText(g, generatedHeaderComment)
		if strings.HasPrefix(content, header+"\n") {
			content = strings.TrimPrefix(content, header+"\n")
		}
		return banner + "\n" + content
	}
	put := func(filename, content string) {
		outputs[localizedPath("docs/gen/"+filename)] = localizedOutput(content)
	}
	var repoMapDocs []GenDocEntry
	var frameworkDocs []GenDocEntry
	var requirementsMD, openMD, unenforcedMD, frameworkInvariantsMD, historyMD string
	var constitutionMD, traceabilityMD, coverageMD string
	var decisionsMD, entitiesMD, tensionsMD, pipelineMD, modelsMD string
	requirementsWritten := RequirementsMDWritten(g, consumer)
	openWritten := OpenMDHasContent(g) && (!consumer || ConsumerOpenMDHasContent(g))
	unenforcedWritten := UnenforcedMDHasContent(g) && (!consumer || ConsumerUnenforcedMDHasContent(g))
	historyWritten := HistoryMDHasContent(g) && (!consumer || ConsumerHistoryMDHasContent(g))
	decisionsWritten := DecisionsMDHasContent(g)
	entitiesWritten := EntitiesMDHasContent(g)
	tensionsWritten := TensionsMDHasContent(g)
	pipelineWritten := PipelineMDHasContent(g)
	modelsWritten := ModelsMDHasContent(g) && !g.IsEmpty()
	frameworkInvariantsWritten := FrameworkInvariantsMDHasContent(g) && !consumer
	constitutionWritten := ConstitutionMDHasContent(g) && !consumer
	traceabilityWritten := TraceabilityMDHasContent(g) && !consumer
	coverageWritten := CoverageMDHasContent(g) && !consumer
	repoMapWritten := RepoMapMDHasContent(g) && !consumer
	agentContextWritten := AgentContextMDHasContent(g) && !consumer
	liveStateWritten := LiveStateMDHasContent(g) && !consumer

	if requirementsWritten {
		requirementsMD = BuildRequirements(g, domainName, consumer)
		put("REQUIREMENTS.md", requirementsMD)
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName("REQUIREMENTS.md"), Content: requirementsMD})
	}
	if openWritten {
		openMD = BuildOpen(g)
		put("OPEN.md", openMD)
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName("OPEN.md"), Content: openMD})
	}
	if unenforcedWritten {
		unenforcedMD = BuildUnenforced(g)
		put("UNENFORCED.md", unenforcedMD)
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName("UNENFORCED.md"), Content: unenforcedMD})
	}
	if frameworkInvariantsWritten {
		frameworkInvariantsMD = BuildFrameworkInvariants(g, domainName)
		put("FRAMEWORK-INVARIANTS.md", frameworkInvariantsMD)
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName("FRAMEWORK-INVARIANTS.md"), Content: frameworkInvariantsMD})
	}
	if historyWritten {
		historyMD = BuildHistory(g)
		put("HISTORY.md", historyMD)
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName("HISTORY.md"), Content: historyMD})
	}
	if constitutionWritten {
		constitutionMD = BuildConstitution(g, domainName, consumer)
		put("CONSTITUTION.md", constitutionMD)
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName("CONSTITUTION.md"), Content: constitutionMD})
	}
	if traceabilityWritten {
		traceabilityMD = BuildTraceability(g)
		put("TRACEABILITY.md", traceabilityMD)
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName("TRACEABILITY.md"), Content: traceabilityMD})
	}
	if coverageWritten {
		coverageMD = BuildCoverage(g)
		put("COVERAGE.md", coverageMD)
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName("COVERAGE.md"), Content: coverageMD})
	}
	if decisionsWritten {
		decisionsMD = BuildDecisions(g)
		put("DECISIONS.md", decisionsMD)
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName("DECISIONS.md"), Content: decisionsMD})
	}
	if entitiesWritten {
		entitiesMD = BuildEntities(g, domainName)
		put("ENTITIES.md", entitiesMD)
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName("ENTITIES.md"), Content: entitiesMD})
	}
	if tensionsWritten {
		tensionsMD = BuildTensions(g)
		put("TENSIONS.md", tensionsMD)
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName("TENSIONS.md"), Content: tensionsMD})
	}
	if pipelineWritten {
		pipelineMD = BuildPipeline(g, domainName, nil)
		put("PIPELINE.md", pipelineMD)
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName("PIPELINE.md"), Content: pipelineMD})
	}
	if modelsWritten {
		modelsMD = BuildModels(g)
		put("MODELS.md", modelsMD)
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName("MODELS.md"), Content: modelsMD})
	}
	engineVersion, engineVersionErr := BuildEngineVersionMDLocalized(repoRoot, language)
	if engineVersionErr == nil {
		put("ENGINE-VERSION.md", engineVersion)
	} else {
		var missing *localization.MissingTranslation
		if errors.As(engineVersionErr, &missing) {
			return fmt.Errorf("render %s engine-version: %w", language, engineVersionErr)
		}
	}

	if !consumer {
		glossary := BuildGlossary(g, consumer)
		frameworkDocs = append(frameworkDocs, GenDocEntry{Filename: localizedName("GLOSSARY.md"), Content: glossary})
		outputs[localizedPath("framework/GLOSSARY.md")] = localizedOutput(glossary)
	}
	specIndexPath, err := layout.SpecIndexPath(language)
	if err != nil {
		return fmt.Errorf("render %s SPEC index path: %w", language, err)
	}
	specOutputPaths := make(map[string]struct{})
	specOutputPaths[filepath.ToSlash(specIndexPath)] = struct{}{}
	if g.SelfExecutingAtoms {
		packageSet := make(map[string]struct{})
		for _, req := range g.Requirements {
			if req.Status != ontology.StatusREJECTED {
				packageSet[gate.SpecPackage(req)] = struct{}{}
			}
		}
		packagePaths := make([]string, 0, len(packageSet))
		for packagePath := range packageSet {
			packagePaths = append(packagePaths, packagePath+".md")
		}
		sort.Strings(packagePaths)
		if _, err := docbundle.SpecShardNames(packagePaths); err != nil {
			return fmt.Errorf("render SPEC shard paths: %w", err)
		}
		for packagePath := range packageSet {
			shardPath, err := layout.SpecShardPath(language, packagePath+".md")
			if err != nil {
				return fmt.Errorf("render %s SPEC shard path for %s: %w", language, packagePath, err)
			}
			specOutputPaths[filepath.ToSlash(shardPath)] = struct{}{}
		}
	}
	specKeys := make([]string, 0, len(specDocs))
	for key := range specDocs {
		specKeys = append(specKeys, key)
	}
	sort.Strings(specKeys)
	for _, key := range specKeys {
		relative := filepath.ToSlash(key)
		candidate := filepath.ToSlash(filepath.Join("docs", "gen", filepath.FromSlash(relative)))
		if _, ok := specOutputPaths[candidate]; ok {
			repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: relative, Content: specDocs[key]})
		}
	}
	if repoMapWritten {
		repoMapName := localizedName("REPO-MAP.md")
		repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: repoMapName, Content: "# " + repoMapName + " — " + serviceText(g, "Repository file index")})
		repoMapMD := BuildRepoMap(g, domainName, repoMapDocs, frameworkDocs, decisionsWritten, entitiesWritten, tensionsWritten, pipelineWritten, modelsWritten, consumer)
		put("REPO-MAP.md", repoMapMD)
	}

	atoms := []struct {
		name string
		text string
	}{
		{"atoms-operator.md", BuildAtomsOperator(g)},
		{"atoms-substrate.md", BuildAtomsSubstrate(g)},
		{"atoms-discipline.md", BuildAtomsDiscipline(g)},
		{"atoms-check.md", BuildAtomsCheck(g)},
	}
	for _, atom := range atoms {
		if !consumer || atomDocumentHasContent(g, atom.name) {
			put(atom.name, atom.text)
			repoMapDocs = append(repoMapDocs, GenDocEntry{Filename: localizedName(atom.name), Content: atom.text})
		}
	}

	if !consumer {
		thinkingDocs, err := BuildThinkingDocsLocalized(language)
		if err != nil {
			return fmt.Errorf("render %s methodology docs: %w", language, err)
		}
		for name, content := range thinkingDocs {
			outputs[localizedPath("docs/gen/thinking/"+name+".md")] = localizedOutput(content)
		}
		toolDocs, err := BuildToolDocsLocalized(language, false)
		if err != nil {
			return fmt.Errorf("render %s tool docs: %w", language, err)
		}
		for command, content := range toolDocs {
			outputs[localizedPath("framework/tools/"+command+".md")] = localizedOutput(content)
		}
		index, err := BuildToolDocsIndexLocalized(language, false)
		if err != nil {
			return fmt.Errorf("render %s tool index: %w", language, err)
		}
		outputs[localizedPath("framework/tools/INDEX.md")] = localizedOutput(index)
	}

	override := &ViolationsOverride{For: g, Violations: violations}
	graphs := map[string]*ontology.Graph{}
	if domainName != "" {
		graphs[domainName] = g
	}
	crystalPath := crystalPaths[language]
	if crystalPath != "" && !filepath.IsAbs(crystalPath) {
		crystalPath = filepath.Join(repoRoot, crystalPath)
	}
	charCount, err := ComputeCrystalCharCountFixpointWithViolations(g, domainName, repoRoot, graphs, today, consumer, override, crystalPath)
	if err != nil {
		return fmt.Errorf("build %s localized crystal budget: %w", language, err)
	}
	if liveStateWritten {
		put("live-state.md", BuildLiveStateWithViolationsRoot(g, domainName, charCount, today, violations, repoRoot))
	}
	if agentContextWritten {
		put("AGENT-CONTEXT.md", BuildAgentContextRootWithViolations(g, domainName, charCount, today, consumer, violations, repoRoot))
	}

	if pathErr != nil {
		return fmt.Errorf("render locale %s output path: %w", language, pathErr)
	}
	return nil
}

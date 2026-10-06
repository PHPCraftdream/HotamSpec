package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func convergeCrystalReaders(g *ontology.Graph, domainName, domainDir, repoRoot, genDir, today string, consumer bool, claudeMDPath string, localizedConfigured, liveStateWritten, agentContextWritten bool, resolvedProfile string, snapshot []invariants.Violation) ([]string, error) {
	written := []string{}
	layout, err := docbundle.NewLayout(g.Languages, g.DefaultLanguage)
	if err != nil {
		return nil, fmt.Errorf("gen-spec fixpoint: output languages: %w", err)
	}
	for iteration := 0; iteration < 3; iteration++ {
		fresh := invariants.CrystalReaderViolations(g)
		patched := make([]invariants.Violation, 0, len(snapshot)+len(fresh))
		for _, v := range snapshot {
			if v.Check != "check_orientation_faq_answered" && v.Check != "check_operator_within_budget" {
				patched = append(patched, v)
			}
		}
		patched = append(patched, fresh...)
		if sameViolationMultiset(patched, snapshot) {
			return written, nil
		}

		var paths []string
		var contents [][]byte
		add := func(p string, content []byte) {
			paths = append(paths, p)
			contents = append(contents, content)
		}
		charCount, err := generator.ComputeCrystalCharCountFixpointWithViolations(g, domainName, repoRoot, map[string]*ontology.Graph{domainName: g}, today, consumer, &generator.ViolationsOverride{For: g, Violations: patched}, claudeMDPath)
		if err != nil {
			return nil, err
		}

		if localizedConfigured {
			specDocsForRepoMap := map[string]string{}
			candidates, err := docbundle.DomainCandidates(genDir)
			if err != nil {
				return nil, err
			}
			for _, path := range candidates {
				if !layout.IsSpecPath(genDir, path) {
					continue
				}
				content, err := os.ReadFile(path)
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return nil, fmt.Errorf("read existing SPEC view %s: %w", path, err)
				}
				if !gate.IsGeneratedSpecDocument(string(content)) {
					continue
				}
				relative, err := filepath.Rel(genDir, path)
				if err != nil {
					return nil, err
				}
				specDocsForRepoMap[filepath.ToSlash(relative)] = string(content)
			}
			crystalPaths := make(map[string]string, len(layout.LanguagesForViews()))
			defaultLanguage := layout.DefaultLanguage
			if len(layout.Languages) == 1 && defaultLanguage == "" {
				defaultLanguage = layout.Languages[0]
			}
			for _, language := range layout.LanguagesForViews() {
				path := claudeMDPath
				if claudeMDPath != "" && layout.Multilingual() && language != defaultLanguage {
					name, err := layout.CrystalPath(language)
					if err != nil {
						return nil, err
					}
					path = filepath.Join(filepath.Dir(claudeMDPath), name)
				}
				crystalPaths[language] = path
			}
			docs, err := generator.BuildLocalizedDocumentsWithSnapshotAndCrystalPathsForProfile(g, domainName, repoRoot, today, patched, specDocsForRepoMap, crystalPaths, resolvedProfile)
			if err != nil {
				return nil, fmt.Errorf("gen-spec fixpoint: render localized documents: %w", err)
			}
			keys := make([]string, 0, len(docs))
			for key := range docs {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				path, err := docbundle.ResolveOutputPath(repoRoot, domainDir, key)
				if err != nil {
					return nil, fmt.Errorf("gen-spec: localized output %q: %w", key, err)
				}
				language, localized := docbundle.LocalizedOutputLanguage(key)
				if !localized && !layout.Multilingual() {
					language, localized = localizedLayoutBaseLanguage(layout)
				}
				if localized {
					if err := validateLocalizedRendererTarget(path, language); err != nil {
						return nil, err
					}
				}
				add(path, []byte(docs[key]))
			}
		}
		if liveStateWritten && !localizedConfigured {
			add(filepath.Join(genDir, "live-state.md"), []byte(generator.BuildLiveStateWithViolationsRoot(g, domainName, charCount, today, patched, repoRoot)))
		}
		if agentContextWritten && !localizedConfigured {
			add(filepath.Join(genDir, "AGENT-CONTEXT.md"), []byte(generator.BuildAgentContextRoot(g, domainName, charCount, today, consumer, repoRoot)))
		}

		if claudeMDPath != "" {
			defaultLanguage := layout.DefaultLanguage
			if len(layout.Languages) == 1 && defaultLanguage == "" {
				defaultLanguage = layout.Languages[0]
			}
			for _, language := range layout.LanguagesForViews() {
				view := *g
				view.RenderLanguage = language
				viewGraphs := map[string]*ontology.Graph{domainName: &view}
				override := &generator.ViolationsOverride{For: &view, Violations: patched}
				crystalPath := claudeMDPath
				if localizedConfigured && language != defaultLanguage {
					name, err := layout.CrystalPath(language)
					if err != nil {
						return nil, err
					}
					crystalPath = filepath.Join(filepath.Dir(claudeMDPath), name)
					if err := validateLocalizedCrystalTarget(crystalPath); err != nil {
						return nil, err
					}
				}
				viewCharCount, err := generator.ComputeCrystalCharCountFixpointWithViolations(&view, domainName, repoRoot, viewGraphs, today, consumer, override, crystalPath)
				if err != nil {
					return nil, err
				}
				text := generator.RenderClaudeMDFromTemplateWithViolations(&view, domainName, repoRoot, viewCharCount, viewGraphs, today, consumer, override, crystalPath) + preserveDurableNotesTail(crystalPath)
				b := []byte(text)
				groupPaths := []string{crystalPath}
				groupContents := [][]byte{b}
				if !localizedConfigured || language == defaultLanguage {
					dir := filepath.Dir(claudeMDPath)
					groupPaths = append(groupPaths, filepath.Join(dir, "AGENTS.md"), filepath.Join(dir, "GEMINI.md"))
					groupContents = append(groupContents, b, b)
				}
				for i := range groupPaths {
					add(groupPaths[i], groupContents[i])
				}
			}
		}
		if err := writeFilesParallel(paths, contents); err != nil {
			return nil, err
		}
		written = append(written, paths...)
		snapshot = patched
	}
	return written, nil
}

func sameViolationMultiset(a, b []invariants.Violation) bool {
	key := func(in []invariants.Violation) []invariants.Violation {
		out := append([]invariants.Violation(nil), in...)
		sort.Slice(out, func(i, j int) bool {
			if out[i].Check != out[j].Check {
				return out[i].Check < out[j].Check
			}
			if out[i].ID != out[j].ID {
				return out[i].ID < out[j].ID
			}
			return out[i].Message < out[j].Message
		})
		return out
	}
	return reflect.DeepEqual(key(a), key(b))
}

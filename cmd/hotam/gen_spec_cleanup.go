package main

import (
	"fmt"
	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// gen_spec_cleanup.go validates gen-spec write targets and removes stale generated files from previous runs.

func preserveDurableNotesTail(path string) string {
	existing, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	_, tail, ok := generator.SplitAtDurableNotesMarker(string(existing))
	if !ok {
		return ""
	}
	return tail
}

func validateLocalizedCrystalTarget(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect localized crystal destination %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to overwrite non-regular localized crystal destination %s", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read localized crystal destination %s: %w", path, err)
	}
	if _, _, ok := generator.SplitAtDurableNotesMarker(string(content)); !ok {
		return fmt.Errorf("refusing to overwrite authored or unrecognized localized crystal %s", path)
	}
	return nil
}

func validateLocalizedRendererTarget(path, language string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect localized document destination %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to overwrite non-regular localized document destination %s", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read localized document destination %s: %w", path, err)
	}
	if !generator.IsGeneratedLocalizedDocument(language, string(content)) {
		return fmt.Errorf("refusing to overwrite authored or unrecognized localized document %s", path)
	}
	return nil
}

func localizedLayoutBaseLanguage(layout docbundle.Layout) (string, bool) {
	if len(layout.Languages) == 1 {
		return layout.Languages[0], true
	}
	if layout.Multilingual() && layout.DefaultLanguage != "" {
		return layout.DefaultLanguage, true
	}
	return "", false
}

// cleanupStaleLocalizedCrystals retires only known generated localized
// CLAUDE.<lang>.md paths. A hand-authored non-template file is preserved; when
// a generated file contains durable notes, its generated section is removed
// while the author's tail remains at the same path.
func cleanupStaleLocalizedCrystals(directory string, written []string) ([]string, error) {
	current := make(map[string]bool, len(written))
	for _, path := range written {
		current[filepath.Clean(path)] = true
	}
	var removed []string
	for _, path := range docbundle.CrystalCandidates(directory) {
		path = filepath.Clean(path)
		if current[path] {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat stale localized crystal %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read stale localized crystal %s: %w", path, err)
		}
		_, tail, templateShaped := generator.SplitAtDurableNotesMarker(string(content))
		if !templateShaped {
			continue
		}
		if strings.TrimSpace(tail) != "" {
			if err := os.WriteFile(path, []byte(generator.DurableNotesMarkerLine+"\n"+tail), info.Mode().Perm()); err != nil {
				return nil, fmt.Errorf("preserve notes from stale localized crystal %s: %w", path, err)
			}
			continue
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove stale localized crystal %s: %w", path, err)
		}
		removed = append(removed, path)
	}
	sort.Strings(removed)
	return removed, nil
}

// cleanupStaleGenFiles removes stale paths only from the shared closed
// docs/gen inventory in internal/docbundle. Unknown top-level names and
// unrecognized localized suffixes are not candidates; the inventory owns
// generated SPEC shards and the known thinking/tools directories.
//
// exemptRelativePaths are domain-relative generated paths intentionally not
// refreshed by this run (such as the current SPEC bundle on a non---spec run)
// or owned by another command (the evidence views and shared evidence.json).
func cleanupStaleGenFiles(genDir string, written []string, exemptRelativePaths []string, layout docbundle.Layout) ([]string, error) {
	candidates, err := docbundle.DomainCandidates(genDir)
	if err != nil {
		return nil, err
	}
	exempt := make([]string, 0, len(exemptRelativePaths))
	for _, relative := range exemptRelativePaths {
		exempt = append(exempt, filepath.Join(genDir, filepath.FromSlash(relative)))
	}
	reportOwned := make(map[string]bool)
	reportCandidates, err := docbundle.ReportCandidates(genDir)
	if err != nil {
		return nil, err
	}
	for _, path := range reportCandidates {
		reportOwned[filepath.Clean(path)] = true
	}
	ownedCandidates := make([]string, 0, len(candidates))
	for _, path := range candidates {
		if reportOwned[filepath.Clean(path)] {
			continue
		}
		relative, err := filepath.Rel(genDir, path)
		if err != nil {
			return nil, err
		}
		relative = filepath.ToSlash(relative)
		if docbundle.UnknownLocaleSuffixedMarkdown(relative) {
			continue
		}
		if docbundle.IsSpecOutputPath(relative) {
			exists, owned, err := inspectGeneratedOutput(path, gate.IsGeneratedSpecDocument)
			if err != nil {
				return nil, err
			}
			if !exists || !owned {
				continue
			}
		} else if language, localized := docbundle.LocalizedOutputLanguage(relative); localized {
			exists, owned, err := inspectGeneratedOutput(path, func(content string) bool {
				return generator.IsGeneratedLocalizedDocument(language, content)
			})
			if err != nil {
				return nil, err
			}
			if !exists || !owned {
				continue
			}
		} else if language, localized := localizedLayoutBaseLanguage(layout); localized && !strings.Contains(relative, "/") && strings.HasSuffix(relative, ".md") {
			exists, owned, err := inspectGeneratedOutput(path, func(content string) bool {
				return generator.IsGeneratedLocalizedDocument(language, content)
			})
			if err != nil {
				return nil, err
			}
			if !exists || !owned {
				continue
			}
		}
		ownedCandidates = append(ownedCandidates, path)
	}
	candidates = ownedCandidates
	stale := docbundle.StalePaths(candidates, written, exempt)
	var removed []string
	for _, path := range stale {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat stale gen file %s: %w", path, err)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove stale gen file %s: %w", path, err)
		}
		removed = append(removed, path)
	}
	return removed, nil
}

func inspectGeneratedOutput(path string, owns func(string) bool) (exists, owned bool, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("inspect generated output %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return true, false, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return true, false, fmt.Errorf("read generated output %s: %w", path, err)
	}
	return true, owns(string(content)), nil
}

func validateSpecOutputTargets(genDir string, documents map[string]string) error {
	relativePaths := make([]string, 0, len(documents))
	for relative := range documents {
		relativePaths = append(relativePaths, relative)
	}
	sort.Strings(relativePaths)
	for _, relative := range relativePaths {
		path := filepath.Join(genDir, filepath.FromSlash(relative))
		exists, owned, err := inspectGeneratedOutput(path, gate.IsGeneratedSpecDocument)
		if err != nil {
			return err
		}
		if exists && !owned {
			return fmt.Errorf("refusing to overwrite authored or unrecognized SPEC file %s", path)
		}
	}
	return nil
}

func cleanupStaleEvidenceLocaleViews(genDir string, layout docbundle.Layout, written []string) ([]string, error) {
	current := append([]string(nil), written...)
	for _, language := range layout.LanguagesForViews() {
		for _, base := range []string{"docs/gen/EVIDENCE.md", "docs/gen/FINDINGS.md"} {
			relative, err := layout.DocumentPath(base, language)
			if err != nil {
				return nil, err
			}
			current = append(current, filepath.Join(genDir, filepath.Base(relative)))
		}
	}
	var candidates []string
	reportCandidates, err := docbundle.ReportCandidates(genDir)
	if err != nil {
		return nil, err
	}
	for _, path := range reportCandidates {
		if filepath.Base(path) == "evidence.json" {
			continue
		}
		candidates = append(candidates, path)
	}
	stale := docbundle.StalePaths(candidates, current, nil)
	var removed []string
	for _, path := range stale {
		relative, err := filepath.Rel(genDir, path)
		if err != nil {
			return nil, err
		}
		owns := generatedReportDocumentPredicate(filepath.ToSlash(relative))
		if owns == nil {
			continue
		}
		exists, generated, err := inspectGeneratedOutput(path, owns)
		if err != nil {
			return nil, err
		}
		if !exists || !generated {
			continue
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove obsolete evidence locale view %s: %w", path, err)
		}
		removed = append(removed, path)
	}
	return removed, nil
}

// projectUsesFullProfile reports whether any domain under repoRoot/domains
// resolves to the full gen profile, in which case the project-shared
// framework/ files belong to that domain and a consumer run must not delete
// them. activeConsumer is this run's own profile: when this run is full the
// answer is trivially yes (its own written list protects the files anyway).

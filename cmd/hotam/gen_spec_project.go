package main

import (
	"fmt"
	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// gen_spec_project.go maintains the project-shared framework/ directory and retires the old per-domain layout.

func projectUsesFullProfile(repoRoot string, activeConsumer bool) bool {
	if !activeConsumer {
		return true
	}
	entries, err := os.ReadDir(filepath.Join(repoRoot, "domains"))
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		gp := filepath.Join(repoRoot, "domains", e.Name(), "graph.json")
		if loader.ResolveGenProfile(gp) == loader.GenProfileFull {
			return true
		}
	}
	return false
}

func projectHasOtherFullProfile(repoRoot, activeDomainDir string) bool {
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

// cleanupStaleProjectFrameworkFiles reconciles the closed shared framework
// inventory from internal/docbundle (localized and default GLOSSARY, plus
// generator-owned framework/tools pages) against this run's staged path set.
// Unrecognized files directly under framework/ are never candidates.
func cleanupStaleProjectFrameworkFiles(projectFrameworkDir string, written []string, layout docbundle.Layout) ([]string, error) {
	candidates, err := docbundle.ProjectCandidates(projectFrameworkDir)
	if err != nil {
		return nil, err
	}
	stale := docbundle.StalePaths(candidates, written, nil)
	var removed []string
	for _, path := range stale {
		relative, err := filepath.Rel(filepath.Dir(projectFrameworkDir), path)
		if err != nil {
			return nil, err
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
				return nil, err
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
					return nil, err
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
			return nil, fmt.Errorf("stat stale framework file %s: %w", path, err)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove stale framework file %s: %w", path, err)
		}
		removed = append(removed, path)
	}
	return removed, nil
}

// cleanupStaleDomainFrameworkDir removes the OLD per-domain framework/ directory
// retired by task #357 (which superseded #355's per-domain framework/ layout).
// Every generator-owned file #355 wrote there — FRAMEWORK-INVARIANTS.md and
// tools/*.md — has moved (FRAMEWORK-INVARIANTS.md back to docs/gen/, tools/*.md
// up to the project-root framework/), so ALL of them are stale here and get
// removed. After the files are gone, the now-empty tools/ subdir and the
// framework/ dir itself are pruned so neither orphan files nor empty
// directories survive the migration (os.Remove on an empty dir succeeds; on a
// non-empty dir it fails and is silently ignored — a hand-placed file would
// block removal, the safe outcome). A non-existent dir is a no-op.
func cleanupStaleDomainFrameworkDir(domainFrameworkDir string) ([]string, error) {
	var candidates []string
	// Closed list: exactly the files genSpec (under #355) ever wrote to the
	// per-domain framework/ dir.
	topLevelFiles := []string{"FRAMEWORK-INVARIANTS.md"}
	for _, name := range topLevelFiles {
		candidates = append(candidates, filepath.Join(domainFrameworkDir, name))
	}
	matches, err := filepath.Glob(filepath.Join(domainFrameworkDir, "tools", "*.md"))
	if err != nil {
		return nil, fmt.Errorf("glob domain framework/tools: %w", err)
	}
	candidates = append(candidates, matches...)

	var removed []string
	for _, c := range candidates {
		cp := filepath.Clean(c)
		if _, err := os.Stat(cp); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat stale domain-framework file %s: %w", cp, err)
		}
		if err := os.Remove(cp); err != nil {
			return nil, fmt.Errorf("remove stale domain-framework file %s: %w", cp, err)
		}
		removed = append(removed, cp)
	}
	// Prune the now-empty tools/ subdir and the framework/ dir itself. os.Remove
	// succeeds only on a truly empty directory; a non-empty one (a hand-placed
	// file an operator added) blocks removal — the safe, conservative outcome.
	toolsSubdir := filepath.Join(domainFrameworkDir, "tools")
	if err := os.Remove(toolsSubdir); err != nil && !os.IsNotExist(err) {
		// Non-empty or permission error: not fatal — the files that matter are
		// already gone; an empty-dir prune failure is best-effort.
		_ = err
	}
	if err := os.Remove(domainFrameworkDir); err != nil && !os.IsNotExist(err) {
		_ = err
	}
	sort.Strings(removed)
	return removed, nil
}

// toolIsImplemented reports whether the tool with the given Command field
// (the key BuildToolDocs uses, e.g. "gen_spec") is registered as
// methodology.Implemented. Used by the consumer-profile tool-docs filter to
// skip Planned tools (whose page is purely aspirational prose for a command
// that does not exist yet). An unrecognized command defaults to true (keep
// writing) — BuildToolDocs only emits entries for registered tools, so an
// unrecognized key here is unreachable in practice; the default is safe.
func toolIsImplemented(cmd string) bool {
	t, ok := methodology.Tools.Get(cmd)
	if !ok {
		return true
	}
	return t.Status == methodology.Implemented
}

// repoRootForDomain resolves the repository root used to render the
// DOMAIN-MAP block (RenderDomainMapBlock lists filepath.Join(repoRoot,
// "domains")). Resolution is three tiers, and never errors — an explicit
// --domain is a complete instruction that must not be blocked by project-root
// discovery.
//
// Tier 1 — <repoRoot>/domains/<name> convention: when domainDir's parent is
// literally "domains" (see internal/generator/claudemd.go's repoRoot doc
// comment: "the parent of domains/"), repoRoot is derived directly from
// domainDir's own path. This is required for genuinely external projects
// (any --domain outside this repository), where paths.ProjectRootOrRaise()'s
// CWD-based marker search has no reason to find anything and must not be
// asked to (R-project-root-not-hardcoded; see
// cmd/hotam/external_e2e_test.go, which regressed when this call was made
// unconditional in task #102 without this fallback: hotam land against a
// foreign project fails loudly instead of resolving via --domain alone).
//
// Tier 2 — non-conforming layout where ProjectRootOrRaise() SUCCEEDS:
// domainDir fixtures that do NOT follow the domains/<name> layout (e.g. this
// package's own test helpers, which copy a domain straight into a bare
// t.TempDir() with no domains/ parent) fall back to
// paths.ProjectRootOrRaise(), preserving the pre-existing CWD-based resolution
// those tests already rely on.
//
// Tier 3 — non-conforming layout where ProjectRootOrRaise() FAILS: a
// genuinely bare domain dir with no project markers discoverable from CWD —
// exactly the shape `hotam init <dir>` scaffolds anywhere on disk and then
// points the user at via "next: hotam gen-spec --domain <dir>". Rather than
// propagate the error, return domainDir itself as the minimal root.
// RenderDomainMapBlock then looks for <domainDir>/domains, does not find it
// (a bare domain dir has no domains/ subdirectory of its own), and renders
// its existing graceful "_(no domains yet — domains/ directory absent)_"
// text — the correct "no DOMAIN-MAP siblings" outcome for a domain with no
// sibling domains to list. The render path is ALREADY graceful about an
// empty/absent domains root, so no error is needed here.

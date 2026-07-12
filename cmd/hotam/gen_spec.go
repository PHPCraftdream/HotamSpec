package main

import (
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/PHPCraftdream/HotamSpecGo/internal/generator"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
	"github.com/PHPCraftdream/HotamSpecGo/internal/paths"
)

func cmdGenSpec(args []string) error {
	fs := newFlagSet("gen-spec")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	claudeMD := fs.String("claude-md", "", "path to CLAUDE.md for rune count")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	written, err := genSpec(domainDir, *claudeMD)
	if err != nil {
		return err
	}
	for _, p := range written {
		fmt.Println(relPathForDisplay(p))
	}
	return nil
}

func genSpec(domainDir, claudeMDPath string) ([]string, error) {
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return nil, err
	}
	genDir := filepath.Join(domainDir, "docs", "gen")
	domainName := domainNameFromDir(domainDir)
	charCount := 0
	if claudeMDPath != "" {
		data, err := os.ReadFile(claudeMDPath)
		if err != nil {
			// A first-ever render has no prior CLAUDE.md to measure; the
			// Python reference treats this the same way (used = 0 when
			// CLAUDE_MD.exists() is False) rather than erroring — only a
			// genuine I/O failure (permissions, etc.) is fatal.
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read --claude-md %s: %w", claudeMDPath, err)
			}
		} else {
			charCount = utf8.RuneCountInString(string(data))
		}
	}

	var written []string

	type docEntry struct {
		filename string
		content  string
	}

	decisionsWritten := generator.DecisionsMDHasContent(g)
	entitiesWritten := generator.EntitiesMDHasContent(g)

	// repoMapDocs mirrors what Python's _process_domains actually writes into
	// domains/<name>/docs/gen/ (REQUIREMENTS/TENSIONS/OPEN/UNENFORCED/
	// GLOSSARY/HISTORY/CONSTITUTION/FRAMEWORK-INVARIANTS/REPO-MAP.md, plus
	// conditional DECISIONS/ENTITIES.md) — it deliberately excludes
	// atoms-*.md and live-state.md, which the Python reference only ever
	// materializes at the repo-root docs/methodology/atoms/ (and inside root
	// CLAUDE.md's LIVE-STATE block) for the single active domain, never
	// per-domain under docs/gen/. REPO-MAP.md's own "Generated docs" section
	// must list exactly this Python-equivalent set to stay byte-identical,
	// even though the Go CLI additionally writes atoms-*.md/live-state.md
	// alongside them on disk (see mdDocs below).
	repoMapDocs := []generator.GenDocEntry{
		{Filename: "REQUIREMENTS.md", Content: generator.BuildRequirements(g)},
		{Filename: "TENSIONS.md", Content: generator.BuildTensions(g)},
		{Filename: "OPEN.md", Content: generator.BuildOpen(g)},
		{Filename: "UNENFORCED.md", Content: generator.BuildUnenforced(g)},
		{Filename: "GLOSSARY.md", Content: generator.BuildGlossary(g)},
		{Filename: "HISTORY.md", Content: generator.BuildHistory(g)},
		{Filename: "CONSTITUTION.md", Content: generator.BuildConstitution(g)},
		{Filename: "FRAMEWORK-INVARIANTS.md", Content: generator.BuildFrameworkInvariants(g, domainName)},
	}
	// REPO-MAP.md lists itself too (Python's _scan_repo_map globs docs/gen/*.md
	// including the file it is about to (re)write); its own title is fixed by
	// the H1 line BuildRepoMap always emits, so a placeholder with just that
	// heading is enough for mdTitle() to extract "Repository file index".
	repoMapSelfEntry := generator.GenDocEntry{Filename: "REPO-MAP.md", Content: "# REPO-MAP.md — Repository file index (Hotam-Spec)"}
	fullRepoMapDocs := append(append([]generator.GenDocEntry{}, repoMapDocs...), repoMapSelfEntry)
	var decisionsMD, entitiesMD string
	if decisionsWritten {
		decisionsMD = generator.BuildDecisions(g)
		fullRepoMapDocs = append(fullRepoMapDocs, generator.GenDocEntry{Filename: "DECISIONS.md", Content: decisionsMD})
	}
	if entitiesWritten {
		entitiesMD = generator.BuildEntities(g, domainName)
		fullRepoMapDocs = append(fullRepoMapDocs, generator.GenDocEntry{Filename: "ENTITIES.md", Content: entitiesMD})
	}
	repoMapMD := generator.BuildRepoMap(g, domainName, fullRepoMapDocs, decisionsWritten, entitiesWritten)

	mdDocs := []docEntry{
		{"REQUIREMENTS.md", repoMapDocs[0].Content},
		{"TENSIONS.md", repoMapDocs[1].Content},
		{"OPEN.md", repoMapDocs[2].Content},
		{"UNENFORCED.md", repoMapDocs[3].Content},
		{"GLOSSARY.md", repoMapDocs[4].Content},
		{"HISTORY.md", repoMapDocs[5].Content},
		{"CONSTITUTION.md", repoMapDocs[6].Content},
		{"FRAMEWORK-INVARIANTS.md", repoMapDocs[7].Content},
		{"REPO-MAP.md", repoMapMD},
		{"atoms-operator.md", generator.BuildAtomsOperator(g)},
		{"atoms-substrate.md", generator.BuildAtomsSubstrate(g)},
		{"atoms-discipline.md", generator.BuildAtomsDiscipline(g)},
		{"atoms-check.md", generator.BuildAtomsCheck(g)},
		{"live-state.md", generator.BuildLiveState(g, charCount)},
	}
	if decisionsWritten {
		mdDocs = append(mdDocs, docEntry{"DECISIONS.md", decisionsMD})
	}
	if entitiesWritten {
		mdDocs = append(mdDocs, docEntry{"ENTITIES.md", entitiesMD})
	}

	for _, d := range mdDocs {
		p := filepath.Join(genDir, d.filename)
		if err := writeFileMkdir(p, []byte(d.content)); err != nil {
			return written, err
		}
		written = append(written, p)
	}

	graphJSON, err := generator.BuildGraphJSON(g)
	if err != nil {
		return written, fmt.Errorf("build graph.json: %w", err)
	}
	gp := filepath.Join(genDir, "graph.json")
	if err := writeFileMkdir(gp, []byte(graphJSON)); err != nil {
		return written, err
	}
	written = append(written, gp)

	for key, content := range generator.BuildThinkingDocs() {
		p := filepath.Join(genDir, "thinking", key+".md")
		if err := writeFileMkdir(p, []byte(content)); err != nil {
			return written, err
		}
		written = append(written, p)
	}
	for cmd, content := range generator.BuildToolDocs() {
		p := filepath.Join(genDir, "tools", cmd+".md")
		if err := writeFileMkdir(p, []byte(content)); err != nil {
			return written, err
		}
		written = append(written, p)
	}

	// Root CLAUDE.md (R-claude-md-template-driven): only rendered when
	// --claude-md points at a path, mirroring the Python reference's
	// unconditional root-crystal regen except that the Go CLI is also used
	// against non-root domain checkouts / tests where no CLAUDE.md is
	// wanted — the flag opts in. charCount (read above, from whatever
	// CLAUDE.md already exists at that path, 0 if absent) feeds the
	// LIVE-STATE CRYSTAL_CHARS budget line; the freshly rendered file is
	// then written back to the SAME path.
	if claudeMDPath != "" {
		repoRoot, err := paths.ProjectRootOrRaise()
		if err != nil {
			return written, fmt.Errorf("resolve project root for --claude-md: %w", err)
		}
		domainGraphs := map[string]*ontology.Graph{domainName: g}
		claudeMD := generator.RenderClaudeMDFromTemplate(g, domainName, repoRoot, charCount, domainGraphs)
		if err := writeFileMkdir(claudeMDPath, []byte(claudeMD)); err != nil {
			return written, err
		}
		written = append(written, claudeMDPath)
	}

	return written, nil
}

package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

func cmdGenSpec(args []string) error {
	fs := newFlagSet("gen-spec")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	claudeMD := fs.String("claude-md", "", "path to CLAUDE.md for rune count")
	todayFlag := fs.String("today", "", "date in YYYY-MM-DD format (default: system date) — embedded in freshness/status lines of the generated docs and root crystal; pin this for reproducible/byte-identical regeneration")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	today := *todayFlag
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}
	written, err := genSpec(domainDir, *claudeMD, today)
	if err != nil {
		return err
	}
	for _, p := range written {
		fmt.Println(relPathForDisplay(p))
	}
	return nil
}

func genSpec(domainDir, claudeMDPath, today string) ([]string, error) {
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return nil, err
	}
	genDir := filepath.Join(domainDir, "docs", "gen")
	domainName := domainNameFromDir(domainDir)

	// Resident-crystal char count (CRYSTAL_CHARS budget measure,
	// R-context-budget-rule): a fixpoint computed at generation time, NOT a
	// read of a stale pre-existing CLAUDE.md. The crystal's LIVE-STATE block
	// embeds its own rune count, so the measurement must converge
	// (render→measure→re-render until the embedded number stops changing);
	// the converged value feeds BOTH the root crystal (CLAUDE.md/AGENTS.md/
	// GEMINI.md) AND docs/gen/AGENT-CONTEXT.md + live-state.md, which carry
	// the same LIVE-STATE block via BuildLiveState. The --claude-md flag
	// below gates only whether the root crystal is written to disk, not
	// whether the measurement is computed — so a gen-spec run WITHOUT
	// --claude-md still embeds the correct fixpoint count into AGENT-CONTEXT.md
	// (closing the former mode-dependent "0 chars" disagreement).
	repoRoot, err := repoRootForDomain(domainDir)
	if err != nil {
		return nil, fmt.Errorf("resolve project root for crystal char-count fixpoint: %w", err)
	}
	domainGraphs := map[string]*ontology.Graph{domainName: g}
	charCount, err := generator.ComputeCrystalCharCountFixpoint(g, domainName, repoRoot, domainGraphs, today)
	if err != nil {
		return nil, err
	}

	var written []string

	type docEntry struct {
		filename string
		content  string
	}

	decisionsWritten := generator.DecisionsMDHasContent(g)
	entitiesWritten := generator.EntitiesMDHasContent(g)

	// repoMapDocs is the canonical set of docs written into
	// domains/<name>/docs/gen/ (REQUIREMENTS/TENSIONS/OPEN/UNENFORCED/
	// GLOSSARY/HISTORY/CONSTITUTION/FRAMEWORK-INVARIANTS/REPO-MAP.md, plus
	// conditional DECISIONS/ENTITIES.md) — it deliberately excludes
	// atoms-*.md and live-state.md, which are only ever materialized at the
	// repo-root docs/methodology/atoms/ (and inside root CLAUDE.md's
	// LIVE-STATE block) for the single active domain, never per-domain under
	// docs/gen/. REPO-MAP.md's own "Generated docs" section must list exactly
	// this set to stay byte-identical, even though atoms-*.md/live-state.md
	// are additionally written alongside them on disk (see mdDocs below).
	repoMapDocs := []generator.GenDocEntry{
		{Filename: "REQUIREMENTS.md", Content: generator.BuildRequirements(g)},
		{Filename: "TENSIONS.md", Content: generator.BuildTensions(g)},
		{Filename: "OPEN.md", Content: generator.BuildOpen(g)},
		{Filename: "UNENFORCED.md", Content: generator.BuildUnenforced(g)},
		{Filename: "GLOSSARY.md", Content: generator.BuildGlossary(g)},
		{Filename: "HISTORY.md", Content: generator.BuildHistory(g)},
		{Filename: "CONSTITUTION.md", Content: generator.BuildConstitution(g, domainName)},
		{Filename: "FRAMEWORK-INVARIANTS.md", Content: generator.BuildFrameworkInvariants(g, domainName)},
	}
	// REPO-MAP.md lists itself too (the repo-map scan globs docs/gen/*.md
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
		{"live-state.md", generator.BuildLiveState(g, charCount, today)},
		{"AGENT-CONTEXT.md", generator.BuildAgentContext(g, domainName, charCount, today)},
	}
	if decisionsWritten {
		mdDocs = append(mdDocs, docEntry{"DECISIONS.md", decisionsMD})
	}
	if entitiesWritten {
		mdDocs = append(mdDocs, docEntry{"ENTITIES.md", entitiesMD})
	}

	// mdDocs's content was already fully rendered above (each entry a pure
	// function of the graph); only the disk write is left, and every write
	// targets a distinct path, so the group fans out over writeFilesParallel
	// (same indexed-slice shape as invariants.AllViolations) instead of the
	// former sequential loop. written must stay in mdDocs' declared order
	// for the console listing (R-doc-names-reader-adjacent tooling greps
	// this output), so it is rebuilt from mdDocs AFTER all writes finish —
	// never appended to from inside a goroutine.
	mdPaths := make([]string, len(mdDocs))
	mdContents := make([][]byte, len(mdDocs))
	for i, d := range mdDocs {
		mdPaths[i] = filepath.Join(genDir, d.filename)
		mdContents[i] = []byte(d.content)
	}
	if err := writeFilesParallel(mdPaths, mdContents); err != nil {
		return written, err
	}
	written = append(written, mdPaths...)

	graphJSON, err := generator.BuildGraphJSON(g)
	if err != nil {
		return written, fmt.Errorf("build graph.json: %w", err)
	}
	gp := filepath.Join(genDir, "graph.json")
	if err := writeFileMkdir(gp, []byte(graphJSON)); err != nil {
		return written, err
	}
	written = append(written, gp)

	// thinking/*.md and tools/*.md: BuildThinkingDocs/BuildToolDocs return
	// maps (Go randomizes map iteration order per run), so the filenames are
	// sorted before writing/appending — this makes `written`'s order for
	// these two groups deterministic run-to-run, which the sequential
	// version never actually guaranteed either. File contents (and thus
	// byte-identity against git) are unaffected either way: only console
	// listing order is at stake here, not what lands on disk.
	thinkingDocs := generator.BuildThinkingDocs()
	thinkingKeys := make([]string, 0, len(thinkingDocs))
	for key := range thinkingDocs {
		thinkingKeys = append(thinkingKeys, key)
	}
	sort.Strings(thinkingKeys)
	thinkingPaths := make([]string, len(thinkingKeys))
	thinkingContents := make([][]byte, len(thinkingKeys))
	for i, key := range thinkingKeys {
		thinkingPaths[i] = filepath.Join(genDir, "thinking", key+".md")
		thinkingContents[i] = []byte(thinkingDocs[key])
	}
	if err := writeFilesParallel(thinkingPaths, thinkingContents); err != nil {
		return written, err
	}
	written = append(written, thinkingPaths...)

	toolDocs := generator.BuildToolDocs()
	toolKeys := make([]string, 0, len(toolDocs))
	for cmd := range toolDocs {
		toolKeys = append(toolKeys, cmd)
	}
	sort.Strings(toolKeys)
	toolPaths := make([]string, len(toolKeys))
	toolContents := make([][]byte, len(toolKeys))
	for i, cmd := range toolKeys {
		toolPaths[i] = filepath.Join(genDir, "tools", cmd+".md")
		toolContents[i] = []byte(toolDocs[cmd])
	}
	if err := writeFilesParallel(toolPaths, toolContents); err != nil {
		return written, err
	}
	written = append(written, toolPaths...)

	// Root CLAUDE.md (R-claude-md-template-driven): the crystal is WRITTEN
	// to disk only when --claude-md points at a path — the reference behavior
	// is an unconditional root-crystal regen, but this CLI is also used
	// against non-root domain checkouts / tests where no CLAUDE.md is wanted,
	// so the flag opts in to the write. charCount is the converged fixpoint
	// computed unconditionally above (against this same render), so the
	// bytes written here embed the crystal's true self-measurement — not a
	// stale pre-existing-file size — and two consecutive --claude-md passes
	// over the same tree now converge byte-for-byte.
	if claudeMDPath != "" {
		claudeMD := generator.RenderClaudeMDFromTemplate(g, domainName, repoRoot, charCount, domainGraphs, today)
		claudeMDBytes := []byte(claudeMD)

		// CLAUDE.md, AGENTS.md and GEMINI.md all receive the identical
		// rendered crystal (same render, same byte slice) at three distinct
		// paths, so the three writes fan out together instead of
		// sequentially; written keeps CLAUDE.md first, then AGENTS.md,
		// GEMINI.md, matching the prior sequential order exactly.
		claudeMDDir := filepath.Dir(claudeMDPath)
		crystalPaths := []string{
			claudeMDPath,
			filepath.Join(claudeMDDir, "AGENTS.md"),
			filepath.Join(claudeMDDir, "GEMINI.md"),
		}
		crystalContents := [][]byte{claudeMDBytes, claudeMDBytes, claudeMDBytes}
		if err := writeFilesParallel(crystalPaths, crystalContents); err != nil {
			return written, err
		}
		written = append(written, crystalPaths...)
	}

	return written, nil
}

// repoRootForDomain resolves the repository root used to render the
// DOMAIN-MAP block (RenderDomainMapBlock lists filepath.Join(repoRoot,
// "domains")). When domainDir follows the established <repoRoot>/domains/
// <name> convention (see internal/generator/claudemd.go's repoRoot doc
// comment: "the parent of domains/"), repoRoot is derived directly from
// domainDir's own path — this is required for genuinely external projects
// (any --domain outside this repository), where paths.ProjectRootOrRaise()'s
// CWD-based marker search has no reason to find anything and must not be
// asked to (R-project-root-not-hardcoded; see
// cmd/hotam/external_e2e_test.go, which regressed when this call was made
// unconditional in task #102 without this fallback: hotam land against a
// foreign project fails loudly instead of resolving via --domain alone).
//
// domainDir fixtures that do NOT follow the domains/<name> layout (e.g. this
// package's own test helpers, which copy a domain straight into a bare
// t.TempDir() with no domains/ parent) fall back to
// paths.ProjectRootOrRaise(), preserving the pre-existing CWD-based
// resolution those tests already rely on.
func repoRootForDomain(domainDir string) (string, error) {
	if filepath.Base(filepath.Dir(domainDir)) == "domains" {
		return filepath.Dir(filepath.Dir(domainDir)), nil
	}
	return paths.ProjectRootOrRaise()
}

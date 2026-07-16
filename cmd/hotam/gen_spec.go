package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

func cmdGenSpec(args []string) error {
	fs := newFlagSet("gen-spec")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	claudeMD := fs.String("claude-md", "", "path to CLAUDE.md for rune count")
	todayFlag := fs.String("today", "", "date in YYYY-MM-DD format (default: system date) — embedded in freshness/status lines of the generated docs and root crystal; pin this for reproducible/byte-identical regeneration")
	profile := fs.String("profile", "", "output profile: consumer|full (default: resolve from the domain's manifest.json, falling back to full)")
	fs.Parse(args)

	// Validate --profile: only "consumer", "full", or empty (resolve from
	// manifest) are accepted. A garbage value is a usage error, not silently
	// ignored — a typo like "consmer" must not silently degrade to "full"
	// and surprise the user with ~90 files instead of ~30.
	switch *profile {
	case "", loader.GenProfileConsumer, loader.GenProfileFull:
		// ok
	default:
		return fmt.Errorf("--profile must be %q, %q, or empty (resolve from manifest), got %q", loader.GenProfileConsumer, loader.GenProfileFull, *profile)
	}

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	today := *todayFlag
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}
	written, removed, err := genSpec(domainDir, *claudeMD, today, *profile)
	if err != nil {
		return err
	}
	for _, p := range written {
		fmt.Println(relPathForDisplay(p))
	}
	if len(removed) > 0 {
		fmt.Printf("removed %d stale file(s):\n", len(removed))
		for _, p := range removed {
			fmt.Println(relPathForDisplay(p))
		}
	}
	return nil
}

func genSpec(domainDir, claudeMDPath, today, profile string) ([]string, []string, error) {
	// Profile resolution (R-gen-spec-profile): an explicit non-empty profile
	// (only cmdGenSpec's --profile flag passes one) overrides the domain's
	// manifest for THIS invocation without rewriting it. An empty profile
	// (every other caller — land, init-project, tests) resolves from the
	// domain's manifest.json gen_profile field, defaulting to "full" when
	// absent — so consumer-profile domains stay consumer on every subsequent
	// hotam land, and every pre-existing domain (no gen_profile field) keeps
	// the full output set byte-identical.
	resolvedProfile := profile
	if resolvedProfile == "" {
		resolvedProfile = loader.ResolveGenProfile(graphPathForDomain(domainDir))
	}
	consumer := resolvedProfile == loader.GenProfileConsumer

	// R-empty-content-gen-notice: a MISSING graph.json (os.IsNotExist — a
	// freshly cloned framework with no domain populated yet) is treated as an
	// empty graph, not a hard error. Every generator already handles
	// g.IsEmpty() by rendering the calm EmptyNotice into docs/gen/*.md, so the
	// missing-file case produces output identical to the empty-but-present
	// case (the two are indistinguishable to an adopter with nothing modeled
	// yet). Any OTHER read error (decode failure, permissions) still surfaces
	// as a real error via loadGraphOrEmpty.
	g, err := loadGraphOrEmpty(domainDir)
	if err != nil {
		return nil, nil, err
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
	repoRoot := repoRootForDomain(domainDir)
	domainGraphs := map[string]*ontology.Graph{domainName: g}
	charCount, err := generator.ComputeCrystalCharCountFixpoint(g, domainName, repoRoot, domainGraphs, today, consumer)
	if err != nil {
		return nil, nil, err
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
	// GLOSSARY/HISTORY/CONSTITUTION/FRAMEWORK-INVARIANTS/PIPELINE/
	// TRACEABILITY/MODELS/REPO-MAP.md, plus conditional DECISIONS/ENTITIES.md) — it deliberately excludes
	// atoms-*.md and live-state.md, which are only ever materialized at the
	// repo-root docs/methodology/atoms/ (and inside root CLAUDE.md's
	// LIVE-STATE block) for the single active domain, never per-domain under
	// docs/gen/. REPO-MAP.md's own "Generated docs" section must list exactly
	// this set to stay byte-identical, even though atoms-*.md/live-state.md
	// are additionally written alongside them on disk (see mdDocs below).
	repoMapDocs := []generator.GenDocEntry{
		{Filename: "REQUIREMENTS.md", Content: generator.BuildRequirements(g, domainName, consumer)},
		{Filename: "TENSIONS.md", Content: generator.BuildTensions(g)},
		{Filename: "OPEN.md", Content: generator.BuildOpen(g)},
		{Filename: "UNENFORCED.md", Content: generator.BuildUnenforced(g)},
		{Filename: "GLOSSARY.md", Content: generator.BuildGlossary(g, consumer)},
		{Filename: "HISTORY.md", Content: generator.BuildHistory(g)},
		{Filename: "CONSTITUTION.md", Content: generator.BuildConstitution(g, domainName, consumer)},
		{Filename: "FRAMEWORK-INVARIANTS.md", Content: generator.BuildFrameworkInvariants(g, domainName)},
		{Filename: "PIPELINE.md", Content: generator.BuildPipeline(g, domainName)},
		{Filename: "TRACEABILITY.md", Content: generator.BuildTraceability(g)},
		{Filename: "MODELS.md", Content: generator.BuildModels(g)},
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
	repoMapMD := generator.BuildRepoMap(g, domainName, fullRepoMapDocs, decisionsWritten, entitiesWritten, consumer)

	// Atoms docs: under the consumer profile, each of the four atoms-*.md
	// files is rendered first (unchanged Build* behavior), then WRITTEN only
	// if it carries real content — i.e. NOT the empty-notice case. An external
	// business domain's own requirements (e.g. R-invoice-approval-flow) will
	// essentially never match the framework-internal prefixes the four
	// selectSettled filters use (R-operator-*, R-claude-md-*, R-anchor-*,
	// R-check-*, …), so all four render the SAME calm empty notice for virtually
	// every external consumer domain — genuinely empty, pure noise. This is a
	// WRITE-time filter in genSpec; BuildAtomsOperator/etc.'s rendering is
	// unchanged. The emptyNotice marker is read from renderAtoms's exact
	// literal (internal/generator/atoms.go) so a future drift in that string
	// stays in sync.
	const emptyAtomsNotice = "_No atomic requirements in this topic yet._"
	atomsOperator := generator.BuildAtomsOperator(g)
	atomsSubstrate := generator.BuildAtomsSubstrate(g)
	atomsDiscipline := generator.BuildAtomsDiscipline(g)
	atomsCheck := generator.BuildAtomsCheck(g)
	shouldWriteAtoms := func(content string) bool {
		return !consumer || !strings.Contains(content, emptyAtomsNotice)
	}

	mdDocs := []docEntry{
		{"REQUIREMENTS.md", repoMapDocs[0].Content},
		{"TENSIONS.md", repoMapDocs[1].Content},
		{"OPEN.md", repoMapDocs[2].Content},
		{"UNENFORCED.md", repoMapDocs[3].Content},
		{"GLOSSARY.md", repoMapDocs[4].Content},
		{"HISTORY.md", repoMapDocs[5].Content},
		{"CONSTITUTION.md", repoMapDocs[6].Content},
		{"FRAMEWORK-INVARIANTS.md", repoMapDocs[7].Content},
		{"PIPELINE.md", repoMapDocs[8].Content},
		{"TRACEABILITY.md", repoMapDocs[9].Content},
		{"MODELS.md", repoMapDocs[10].Content},
		{"REPO-MAP.md", repoMapMD},
	}
	if shouldWriteAtoms(atomsOperator) {
		mdDocs = append(mdDocs, docEntry{"atoms-operator.md", atomsOperator})
	}
	if shouldWriteAtoms(atomsSubstrate) {
		mdDocs = append(mdDocs, docEntry{"atoms-substrate.md", atomsSubstrate})
	}
	if shouldWriteAtoms(atomsDiscipline) {
		mdDocs = append(mdDocs, docEntry{"atoms-discipline.md", atomsDiscipline})
	}
	if shouldWriteAtoms(atomsCheck) {
		mdDocs = append(mdDocs, docEntry{"atoms-check.md", atomsCheck})
	}
	mdDocs = append(mdDocs,
		docEntry{"live-state.md", generator.BuildLiveState(g, domainName, charCount, today)},
		docEntry{"AGENT-CONTEXT.md", generator.BuildAgentContext(g, domainName, charCount, today)},
	)
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
		return written, nil, err
	}
	written = append(written, mdPaths...)

	graphJSON, err := generator.BuildGraphJSON(g)
	if err != nil {
		return written, nil, fmt.Errorf("build graph.json: %w", err)
	}
	gp := filepath.Join(genDir, "graph.json")
	if err := writeFileMkdir(gp, []byte(graphJSON)); err != nil {
		return written, nil, err
	}
	written = append(written, gp)

	// thinking/*.md and tools/*.md: BuildThinkingDocs/BuildToolDocs return
	// maps (Go randomizes map iteration order per run), so the filenames are
	// sorted before writing/appending — this makes `written`'s order for
	// these two groups deterministic run-to-run, which the sequential
	// version never actually guaranteed either. File contents (and thus
	// byte-identity against git) are unaffected either way: only console
	// listing order is at stake here, not what lands on disk.
	//
	// Consumer profile: thinking/*.md (the full Canon/Narrative/Why prose for
	// every §-section of the METHODOLOGY ITSELF) is framework
	// self-documentation, not domain content — skipped entirely for an
	// external business consumer who just uses the tool.
	if !consumer {
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
			return written, nil, err
		}
		written = append(written, thinkingPaths...)
	}

	// Consumer profile: write tools/*.md pages ONLY for Implemented tools
	// (skip the 27 Planned tools whose page is entirely historical/aspirational
	// prose for a command that doesn't exist yet). tools/INDEX.md is written
	// unconditionally below regardless of profile (cheap, useful, and already
	// the recommended pointer per the crystal-trim philosophy).
	toolDocs := generator.BuildToolDocs(consumer)
	toolKeys := make([]string, 0, len(toolDocs))
	for cmd := range toolDocs {
		if consumer && !toolIsImplemented(cmd) {
			continue
		}
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
		return written, nil, err
	}
	written = append(written, toolPaths...)

	// tools/INDEX.md: a single entry-point page splitting the registry into
	// Implemented (real commands) vs Planned (methodology surface only), so a
	// browser of docs/gen/tools/ is not misled by the raw file count (40 .md
	// files, only 13 backing runnable commands). Purely additive — one extra
	// file alongside the per-tool docs above.
	toolIndexPath := filepath.Join(genDir, "tools", "INDEX.md")
	if err := writeFileMkdir(toolIndexPath, []byte(generator.BuildToolDocsIndex(consumer))); err != nil {
		return written, nil, err
	}
	written = append(written, toolIndexPath)

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
		claudeMD := generator.RenderClaudeMDFromTemplate(g, domainName, repoRoot, charCount, domainGraphs, today, consumer)
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
			return written, nil, err
		}
		written = append(written, crystalPaths...)
	}

	// R-profile-switch-cleanup: genSpec only ever WRITES files, never
	// deletes — so switching a domain's profile from full to consumer (via
	// --profile, or a manifest gen_profile change landed through `hotam land`)
	// would leave the now-unwanted thinking/*.md and Planned-tool pages on disk
	// even though the printed written list shrank. A single cleanup pass at the
	// end removes any generator-owned file under docs/gen/ that is NOT in this
	// run's written list — correct in BOTH directions (full→consumer deletes;
	// consumer→full is a no-op since written only grows). The comparison is
	// against written, not the profile flag directly, so it stays correct for
	// any future variance in the written set (e.g. conditional DECISIONS/
	// ENTITIES), not just the profile switch.
	removed, err := cleanupStaleGenFiles(genDir, written)
	if err != nil {
		return written, nil, err
	}
	return written, removed, nil
}

// cleanupStaleGenFiles deletes generator-owned files under <domainDir>/docs/gen/
// that exist on disk but are NOT in this run's written list. Deletion is scoped
// strictly to three categories genSpec is authoritative over — nothing outside
// docs/gen/ is ever touched, and within docs/gen/ only (1) a CLOSED filename
// list of top-level files, (2) every docs/gen/thinking/*.md, and (3) every
// docs/gen/tools/*.md are candidates, so a hand-placed or future file with an
// unrecognized top-level name is left alone (no blind glob of docs/gen/*.md).
// thinking/ and tools/ ARE fully globbed because every file in them is
// generator-owned. It returns the sorted list of deleted file paths so the
// caller can report the shrinkage.
func cleanupStaleGenFiles(genDir string, written []string) ([]string, error) {
	writtenSet := make(map[string]bool, len(written))
	for _, p := range written {
		writtenSet[filepath.Clean(p)] = true
	}

	// (1) Closed list of top-level docs/gen/ filenames genSpec ever produces.
	// graph.json is always written so it never qualifies for removal, but is
	// listed for completeness so the candidate set matches genSpec's surface.
	topLevelFiles := []string{
		"REQUIREMENTS.md", "TENSIONS.md", "OPEN.md", "UNENFORCED.md",
		"GLOSSARY.md", "HISTORY.md", "CONSTITUTION.md", "FRAMEWORK-INVARIANTS.md",
		"PIPELINE.md", "TRACEABILITY.md", "MODELS.md",
		"REPO-MAP.md", "atoms-operator.md", "atoms-substrate.md",
		"atoms-discipline.md", "atoms-check.md", "live-state.md",
		"AGENT-CONTEXT.md", "DECISIONS.md", "ENTITIES.md", "graph.json",
	}
	var candidates []string
	for _, name := range topLevelFiles {
		candidates = append(candidates, filepath.Join(genDir, name))
	}

	// (2) thinking/*.md and (3) tools/*.md — every .md in these two directories
	// is generator-owned (nothing else is ever placed there), so glob-and-diff
	// against written is safe. A non-existent directory yields an empty match
	// set from filepath.Glob (nil error), which is the correct no-candidates
	// outcome.
	for _, sub := range []string{"thinking", "tools"} {
		matches, err := filepath.Glob(filepath.Join(genDir, sub, "*.md"))
		if err != nil {
			return nil, fmt.Errorf("glob docs/gen/%s: %w", sub, err)
		}
		candidates = append(candidates, matches...)
	}

	var removed []string
	for _, c := range candidates {
		cp := filepath.Clean(c)
		if writtenSet[cp] {
			continue
		}
		if _, err := os.Stat(cp); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat stale gen file %s: %w", cp, err)
		}
		if err := os.Remove(cp); err != nil {
			return nil, fmt.Errorf("remove stale gen file %s: %w", cp, err)
		}
		removed = append(removed, cp)
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
func repoRootForDomain(domainDir string) string {
	if filepath.Base(filepath.Dir(domainDir)) == "domains" {
		return filepath.Dir(filepath.Dir(domainDir))
	}
	if root, err := paths.ProjectRootOrRaise(); err == nil {
		return root
	}
	return domainDir
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

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

// countFilesUnder walks dir and returns the total file count, plus per-category
// counts keyed by the immediate subdirectory (or "root" for files directly in
// dir). Works for either docs/gen/ or framework/ (task #355: the generator's
// output tree now spans both sibling directories).
func countFilesUnder(t *testing.T, genDir string) (total int, byCat map[string]int) {
	t.Helper()
	byCat = map[string]int{"root": 0}
	err := filepath.Walk(genDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		total++
		rel, err := filepath.Rel(genDir, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) == 1 {
			byCat["root"]++
		} else {
			byCat[parts[0]]++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", genDir, err)
	}
	return total, byCat
}

// implementedToolCount counts the methodology tools with Status == Implemented.
func implementedToolCount() int {
	n := 0
	for _, t := range methodology.Tools.All() {
		if t.Status == methodology.Implemented {
			n++
		}
	}
	return n
}

// plannedToolCount counts the methodology tools with Status == Planned.
func plannedToolCount() int {
	n := 0
	for _, t := range methodology.Tools.All() {
		if t.Status == methodology.Planned {
			n++
		}
	}
	return n
}

// thinkingDocsCount counts the methodology sections (one thinking doc each).
func thinkingDocsCount() int {
	return len(methodology.Sections.All())
}

// TestGenSpec_ConsumerProfileSkipsFrameworkNoise proves the consumer profile
// cuts the external seed output by skipping thinking/*.md, Planned tool docs,
// and empty atoms docs — the three categories of framework-self-documentation
// noise for an external business consumer. Uses a freshly-scaffolded domain
// seeded with seedMinimalRequirement's placeholder content (its id,
// "R-domain-exists", matches no framework-internal atoms prefix), so all four
// atoms docs render the empty notice and are correctly skipped, while the
// non-empty graph still exercises every OTHER docs/gen/ projection (task
// #364 made an actually EMPTY graph skip those entirely, which would mask
// the profile-specific skip this test exists to prove). Exact file counts
// are pinned so a regression that silently stops skipping is caught
// immediately.
func TestGenSpec_ConsumerProfileSkipsFrameworkNoise(t *testing.T) {
	t.Parallel()

	// Scaffold a fresh external domain with minimal non-framework-internal
	// content — non-empty (so REQUIREMENTS.md/etc. render), but the one
	// requirement's id matches no framework-internal atoms prefix, so all
	// four atoms docs are still genuinely empty.
	projectRoot, domainDir := initDomainUnderRoot(t, "test-external", "2026-07-13")
	seedMinimalRequirement(t, domainDir, "test-external", "2026-07-13")
	// initDomain writes {"self_hosting": false, "gen_profile": "consumer"}
	// (R8-e: unified with init-project). We pass explicit "consumer" here to
	// test the cut directly regardless of the manifest default.

	consumerWritten, _, err := genSpec(domainDir, "", "2026-07-13", "consumer", false)
	if err != nil {
		t.Fatalf("genSpec consumer: %v", err)
	}
	genDir := filepath.Join(domainDir, "docs", "gen")
	// Task #357: GLOSSARY.md + tools/*.md live at the PROJECT-root framework/.
	frameworkDir := filepath.Join(projectRoot, "framework")
	genTotal, genByCat := countFilesUnder(t, genDir)
	fwTotal, fwByCat := countFilesUnder(t, frameworkDir)
	total := genTotal + fwTotal

	// (1) No thinking/ directory at all under docs/gen/.
	if _, err := os.Stat(filepath.Join(genDir, "thinking")); !os.IsNotExist(err) {
		t.Errorf("consumer profile must not create thinking/ dir, got err=%v", err)
	}

	// (2) framework/tools/ has exactly Implemented tools + INDEX.md (no Planned
	// pages). Task #355 relocated tools/*.md out of docs/gen/tools/ into the
	// sibling framework/tools/ directory.
	wantToolCount := implementedToolCount() + 1 // +INDEX.md
	if fwByCat["tools"] != wantToolCount {
		t.Errorf("consumer framework/tools/ file count = %d, want %d (Implemented=%d + INDEX)", fwByCat["tools"], wantToolCount, implementedToolCount())
	}

	// (3) No atoms-*.md files (all four are empty-notice for an external graph).
	for _, name := range []string{"atoms-operator.md", "atoms-substrate.md", "atoms-discipline.md", "atoms-check.md"} {
		if _, err := os.Stat(filepath.Join(genDir, name)); err == nil {
			t.Errorf("consumer profile must skip empty atoms doc %s, but it exists", name)
		} else if !os.IsNotExist(err) {
			t.Errorf("stat %s: %v", name, err)
		}
	}

	// (4) framework/tools/INDEX.md still written.
	if _, err := os.Stat(filepath.Join(frameworkDir, "tools", "INDEX.md")); err != nil {
		t.Errorf("consumer profile must still write framework/tools/INDEX.md: %v", err)
	}

	// (5) Pin the exact total across both output directories: docs/gen/ root
	//     (11 non-atoms .md: TENSIONS/PIPELINE/MODELS not written for this
	//     minimal graph — no conflicts/axes, no processes, no spec/ model files;
	//     task #361) + live-state.md + AGENT-CONTEXT.md; GLOSSARY.md moved out
	//     under task #357, FRAMEWORK-INVARIANTS.md moved back in — net same
	//     count; DECISIONS/ENTITIES not written for this minimal graph) +
	//     graph.json (1) + framework/ root (GLOSSARY.md=1) + framework/tools/
	//     (Implemented + INDEX). This catches a regression that silently stops
	//     skipping any category.
	wantGenRoot := 11
	wantGenTotal := wantGenRoot + 1 // + graph.json
	wantFwRoot := 1                 // GLOSSARY.md
	wantTotal := wantGenTotal + wantFwRoot + wantToolCount
	if total != wantTotal {
		t.Errorf("consumer total (docs/gen + framework) file count = %d, want %d (genRoot=%d + graph.json=1 + fwRoot=1 + tools=%d)", total, wantTotal, wantGenRoot, wantToolCount)
	}
	_ = genByCat // (genDir per-category not asserted here; thinking/tools/atoms checked above)

	// (6) written must report exactly the files that landed on disk.
	if len(consumerWritten) != total {
		t.Errorf("len(written)=%d != files on disk=%d — written must reflect only files actually written under the active profile", len(consumerWritten), total)
	}
}

// TestGenSpec_FullProfileUnchanged proves the full profile writes the SAME file
// set as today (no regression for existing domains), and that genSpec(...,"", false)
// (empty profile, manifest without gen_profile → resolves to "full") produces
// an identical file set to genSpec(...,"full", false).
func TestGenSpec_FullProfileUnchanged(t *testing.T) {
	t.Parallel()

	projectRoot, domainDir := initDomainUnderRoot(t, "test-external", "2026-07-13")
	seedMinimalRequirement(t, domainDir, "test-external", "2026-07-13")

	// Full profile
	fullWritten, _, err := genSpec(domainDir, "", "2026-07-13", "full", false)
	if err != nil {
		t.Fatalf("genSpec full: %v", err)
	}
	genDir := filepath.Join(domainDir, "docs", "gen")
	frameworkDir := filepath.Join(projectRoot, "framework")
	fullGenTotal, fullByCat := countFilesUnder(t, genDir)
	fullFwTotal, fullFwByCat := countFilesUnder(t, frameworkDir)
	fullTotal := fullGenTotal + fullFwTotal

	// (1) Full writes thinking docs.
	wantThinking := thinkingDocsCount()
	if fullByCat["thinking"] != wantThinking {
		t.Errorf("full thinking/ count = %d, want %d (methodology sections)", fullByCat["thinking"], wantThinking)
	}

	// (2) Full writes ALL tool docs + INDEX (under framework/tools/, task #355).
	wantToolFull := len(methodology.Tools.All()) + 1
	if fullFwByCat["tools"] != wantToolFull {
		t.Errorf("full framework/tools/ count = %d, want %d (all tools + INDEX)", fullFwByCat["tools"], wantToolFull)
	}

	// (3) Full writes all 4 atoms docs (even when empty — the empty notice).
	for _, name := range []string{"atoms-operator.md", "atoms-substrate.md", "atoms-discipline.md", "atoms-check.md"} {
		if _, err := os.Stat(filepath.Join(genDir, name)); err != nil {
			t.Errorf("full profile must write atoms doc %s, got err: %v", name, err)
		}
	}

	// (4) Pin the exact total for full (regression guard).
	//     docs/gen/ root = 15 (11 non-atoms + 4 atoms; TENSIONS/PIPELINE/MODELS
	//     not written for this minimal graph — task #361; FRAMEWORK-INVARIANTS.md
	//     moved out under task #355) + graph.json(1) + thinking(N).
	//     framework/ root = 1 (FRAMEWORK-INVARIANTS.md) + tools(all+1).
	wantRootFull := 15 // 11 non-atoms + 4 atoms
	wantGenTotalFull := wantRootFull + 1 + wantThinking
	wantFwRootFull := 1
	wantTotalFull := wantGenTotalFull + wantFwRootFull + wantToolFull
	if fullTotal != wantTotalFull {
		t.Errorf("full total (docs/gen + framework) file count = %d, want %d (genRoot=%d + graph.json=1 + thinking=%d + fwRoot=1 + tools=%d)", fullTotal, wantTotalFull, wantRootFull, wantThinking, wantToolFull)
	}

	// (5) Empty profile against a manifest without gen_profile → resolves to
	//     "full" → same file set as explicit "full". initDomain now writes
	//     gen_profile: consumer (R8-e), so to test the ResolveGenProfile
	//     absent-field fallback (backward compat for pre-existing domains
	//     whose manifests predate the profile feature) we must explicitly
	//     write a gen_profile-less manifest here.
	manifestPath := filepath.Join(domainDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{\"self_hosting\": false}\n"), 0o644); err != nil {
		t.Fatalf("write gen_profile-less manifest: %v", err)
	}
	emptyWritten, _, err := genSpec(domainDir, "", "2026-07-13", "", false)
	if err != nil {
		t.Fatalf("genSpec empty-profile: %v", err)
	}
	if len(emptyWritten) != len(fullWritten) {
		t.Errorf("empty-profile written count %d != full-profile written count %d — empty must resolve to full when manifest has no gen_profile", len(emptyWritten), len(fullWritten))
	}

	// Compare basenames sorted (the paths are identical since same domainDir).
	fullBases := sortedBasenames(fullWritten)
	emptyBases := sortedBasenames(emptyWritten)
	for i := range fullBases {
		if fullBases[i] != emptyBases[i] {
			t.Errorf("file set mismatch at index %d: full=%q empty=%q", i, fullBases[i], emptyBases[i])
			break
		}
	}
}

// TestGenSpec_ConsumerVsFullDelta pins the exact reduction: the consumer cut
// removes thinking/*.md (N sections) + Planned tool docs (M tools) + 4 empty
// atoms docs. The delta must equal exactly N+M+4 — if any category stops being
// skipped, the delta shrinks and this test fails.
func TestGenSpec_ConsumerVsFullDelta(t *testing.T) {
	t.Parallel()

	// Two identical fresh domains under two SEPARATE project roots (so each
	// gets its own project-root framework/ — a shared dir would let the second
	// genSpec's cleanup remove the first's tool pages, corrupting the count).
	rootConsumer, dirConsumer := initDomainUnderRoot(t, "ext-consumer", "2026-07-13")
	seedMinimalRequirement(t, dirConsumer, "ext-consumer", "2026-07-13")
	if _, _, err := genSpec(dirConsumer, "", "2026-07-13", "consumer", false); err != nil {
		t.Fatalf("genSpec consumer: %v", err)
	}
	consumerGenTotal, _ := countFilesUnder(t, filepath.Join(dirConsumer, "docs", "gen"))
	consumerFwTotal, _ := countFilesUnder(t, filepath.Join(rootConsumer, "framework"))
	consumerTotal := consumerGenTotal + consumerFwTotal

	rootFull, dirFull := initDomainUnderRoot(t, "ext-full", "2026-07-13")
	seedMinimalRequirement(t, dirFull, "ext-full", "2026-07-13")
	if _, _, err := genSpec(dirFull, "", "2026-07-13", "full", false); err != nil {
		t.Fatalf("genSpec full: %v", err)
	}
	fullGenTotal, _ := countFilesUnder(t, filepath.Join(dirFull, "docs", "gen"))
	fullFwTotal, _ := countFilesUnder(t, filepath.Join(rootFull, "framework"))
	fullTotal := fullGenTotal + fullFwTotal

	wantDelta := thinkingDocsCount() + plannedToolCount() + 4 // +4 empty atoms
	actualDelta := fullTotal - consumerTotal
	if actualDelta != wantDelta {
		t.Errorf("consumer/full delta = %d (full=%d, consumer=%d), want exactly %d (thinking=%d + planned=%d + atoms=4)",
			actualDelta, fullTotal, consumerTotal, wantDelta, thinkingDocsCount(), plannedToolCount())
	}
}

func sortedBasenames(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}
	sort.Strings(out)
	return out
}

// TestGenSpec_ProfileSwitchCleansStaleFiles proves the R6-c fix: switching a
// domain's gen-spec profile from full to consumer does NOT just shrink the
// printed written list — it actually DELETES the now-unwanted thinking/*.md and
// Planned-tool pages from disk. Before the fix genSpec only ever WROTE files
// (never deleted), so a full→consumer switch left ~60 stale files on disk even
// though the printed summary shrank. The round-trip (full→consumer→full) must be
// non-destructive to content — only file PRESENCE cycles. A hand-placed file
// inside docs/gen/ with a name outside the closed generator-owned list must
// survive untouched, proving the cleanup is scoped, not a blind wipe.
func TestGenSpec_ProfileSwitchCleansStaleFiles(t *testing.T) {
	t.Parallel()

	// Pick a concrete Planned tool so we can assert its page's PRESENCE/ABSENCE
	// directly on disk (not just via the written slice).
	plannedCmd := ""
	for _, tl := range methodology.Tools.All() {
		if tl.Status == methodology.Planned {
			plannedCmd = tl.Command
			break
		}
	}
	if plannedCmd == "" {
		t.Fatal("precondition: at least one Planned tool must exist")
	}

	projectRoot, domainDir := initDomainUnderRoot(t, "test-external", "2026-07-13")
	seedMinimalRequirement(t, domainDir, "test-external", "2026-07-13")
	genDir := filepath.Join(domainDir, "docs", "gen")
	thinkingDir := filepath.Join(genDir, "thinking")
	projectFrameworkDir := filepath.Join(projectRoot, "framework")
	fullPlannedToolPage := filepath.Join(projectFrameworkDir, "tools", plannedCmd+".md")

	// (1) full profile: thinking/*.md and the Planned-tool page exist on disk.
	if _, _, err := genSpec(domainDir, "", "2026-07-13", "full", false); err != nil {
		t.Fatalf("genSpec full (pass 1): %v", err)
	}
	fullThinking, err := filepath.Glob(filepath.Join(thinkingDir, "*.md"))
	if err != nil {
		t.Fatalf("glob thinking full: %v", err)
	}
	if len(fullThinking) == 0 {
		t.Fatal("full profile must write thinking/*.md, found none on disk")
	}
	if _, err := os.Stat(fullPlannedToolPage); err != nil {
		t.Fatalf("full profile must write planned-tool page %s on disk: %v", plannedCmd, err)
	}

	// Drop a hand-placed file with a name OUTSIDE the closed top-level list. It
	// is NOT generator output, so it must survive every cleanup pass below.
	handPlaced := filepath.Join(genDir, "NOTES-not-generated.md")
	if err := os.WriteFile(handPlaced, []byte("# hand-placed notes\n"), 0o644); err != nil {
		t.Fatalf("write hand-placed: %v", err)
	}

	// (2) consumer profile on the SAME domainDir (simulating a real profile
	// switch on an existing checkout): thinking/*.md and the Planned-tool page
	// must now be ABSENT from disk, not merely absent from written.
	consumerWritten, removed, err := genSpec(domainDir, "", "2026-07-13", "consumer", false)
	if err != nil {
		t.Fatalf("genSpec consumer (pass 2): %v", err)
	}
	consumerThinking, err := filepath.Glob(filepath.Join(thinkingDir, "*.md"))
	if err != nil {
		t.Fatalf("glob thinking consumer: %v", err)
	}
	if len(consumerThinking) != 0 {
		t.Errorf("consumer profile must DELETE thinking/*.md from disk, found %d file(s): %v", len(consumerThinking), consumerThinking)
	}
	if _, err := os.Stat(fullPlannedToolPage); !os.IsNotExist(err) {
		t.Errorf("consumer profile must DELETE planned-tool page %s from disk, got err=%v", plannedCmd, err)
	}
	if _, err := os.Stat(handPlaced); err != nil {
		t.Errorf("hand-placed docs/gen/NOTES-not-generated.md must survive cleanup, got err=%v", err)
	}
	// The cleanup pass must report what it removed (the reporting path), and it
	// must include the thinking dir + the planned-tool page.
	if len(removed) == 0 {
		t.Error("consumer run after full must report removed stale files (removed slice empty)")
	}
	removedHas := func(target string) bool {
		ct := filepath.Clean(target)
		for _, r := range removed {
			if filepath.Clean(r) == ct {
				return true
			}
		}
		return false
	}
	if !removedHas(fullPlannedToolPage) {
		t.Errorf("removed slice must list the deleted planned-tool page %s, got %v", plannedCmd, removed)
	}
	// On disk, the only file NOT accounted for by written must be the single
	// hand-placed file — every stale GENERATOR file must be gone. Counts BOTH
	// output directories (docs/gen/ + project-root framework/, task #357) since
	// `written` spans both.
	genOnDisk, _ := countFilesUnder(t, genDir)
	fwOnDisk, _ := countFilesUnder(t, filepath.Join(projectRoot, "framework"))
	totalOnDisk := genOnDisk + fwOnDisk
	if totalOnDisk != len(consumerWritten)+1 {
		t.Errorf("consumer on-disk=%d, want written(%d)+1 (the hand-placed file) — a larger gap means stale generator output survived cleanup", totalOnDisk, len(consumerWritten))
	}

	// (3) full profile again on the same domain: everything restored. The
	// round-trip is non-destructive to content; only file PRESENCE cycles.
	if _, _, err := genSpec(domainDir, "", "2026-07-13", "full", false); err != nil {
		t.Fatalf("genSpec full (pass 3): %v", err)
	}
	restoredThinking, err := filepath.Glob(filepath.Join(thinkingDir, "*.md"))
	if err != nil {
		t.Fatalf("glob thinking restored: %v", err)
	}
	if len(restoredThinking) != len(fullThinking) {
		t.Errorf("full→consumer→full round-trip: thinking/ count changed %d → %d", len(fullThinking), len(restoredThinking))
	}
	if _, err := os.Stat(fullPlannedToolPage); err != nil {
		t.Errorf("full→consumer→full round-trip: planned-tool page %s not restored: %v", plannedCmd, err)
	}
	if _, err := os.Stat(handPlaced); err != nil {
		t.Errorf("hand-placed file must STILL survive the consumer→full pass: %v", err)
	}
}

// TestGenSpec_ConsumerRequirementsToolsIndexReferenceExistsOnDisk proves the
// R7-a fix (F2, review-6 @fl follow-up on task #140): the consumer-profile
// REQUIREMENTS.md closing section's "Implemented commands" pointer must be
// domain-prefixed AND resolve to a real file on disk after a real
// consumer-profile gen-spec run — mirroring how claudemd_links_test.go proves
// the root crystal's own cross-references exist on disk (task #135), applied
// here to task #140's consumer closing section instead. Before the fix the
// pointer read the bare `docs/gen/tools/INDEX.md` (missing the
// domains/<name>/ prefix every other cross-reference in this codebase
// follows), which would resolve at the repo root — a path that never exists,
// since every domain's generated docs live under domains/<name>/docs/gen/
// (see initDomain / repoMapDocs in gen_spec.go).
func TestGenSpec_ConsumerRequirementsToolsIndexReferenceExistsOnDisk(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	domainDir := filepath.Join(repoRoot, "domains", "test-linkcheck-requirements")
	if _, err := initDomain(domainDir, "test-linkcheck-requirements", "2026-07-14"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}
	// A genuinely empty graph (task #364) now skips REQUIREMENTS.md entirely;
	// seed minimal content so it renders and this test can assert on its body.
	seedMinimalRequirement(t, domainDir, "test-linkcheck-requirements", "2026-07-14")
	if _, _, err := genSpec(domainDir, "", "2026-07-14", "consumer", false); err != nil {
		t.Fatalf("genSpec consumer: %v", err)
	}

	reqPath := filepath.Join(domainDir, "docs", "gen", "REQUIREMENTS.md")
	content, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("read REQUIREMENTS.md: %v", err)
	}
	text := string(content)

	// Task #357: tools/INDEX.md lives at the PROJECT-root framework/, so the
	// consumer REQUIREMENTS.md references it with a bare repo-root-relative
	// path (NOT a domain-prefixed one — framework/ is a sibling of domains/).
	wantToken := "framework/tools/INDEX.md"
	if !strings.Contains(text, "`"+wantToken+"`") {
		t.Fatalf("consumer REQUIREMENTS.md must reference %q, got:\n%s", wantToken, text)
	}
	if strings.Contains(text, "`docs/gen/tools/INDEX.md`") {
		t.Errorf("consumer REQUIREMENTS.md must NOT reference the pre-#355 location `docs/gen/tools/INDEX.md`")
	}

	resolved := filepath.Join(repoRoot, filepath.FromSlash(wantToken))
	if _, err := os.Stat(resolved); err != nil {
		t.Errorf("REQUIREMENTS.md references %q but it does not exist on disk at %s: %v", wantToken, resolved, err)
	}
}

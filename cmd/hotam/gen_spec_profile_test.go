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
// counts keyed by the immediate subdirectory under docs/gen/ (or "root" for
// files directly in docs/gen/).
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
// (initDomain's R-domain-exists matches no framework-internal atoms prefix),
// so all four atoms docs render the empty notice and are correctly skipped.
// Exact file counts are pinned so a regression that silently stops skipping is
// caught immediately.
func TestGenSpec_ConsumerProfileSkipsFrameworkNoise(t *testing.T) {
	t.Parallel()

	// Scaffold a fresh external domain — no framework-internal requirements,
	// so all four atoms docs are genuinely empty.
	domainDir := t.TempDir()
	if _, err := initDomain(domainDir, "test-external", "2026-07-13"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}
	// initDomain writes {"self_hosting": false, "gen_profile": "consumer"}
	// (R8-e: unified with init-project). We pass explicit "consumer" here to
	// test the cut directly regardless of the manifest default.

	consumerWritten, _, err := genSpec(domainDir, "", "2026-07-13", "consumer")
	if err != nil {
		t.Fatalf("genSpec consumer: %v", err)
	}
	genDir := filepath.Join(domainDir, "docs", "gen")
	total, byCat := countFilesUnder(t, genDir)

	// (1) No thinking/ directory at all.
	if _, err := os.Stat(filepath.Join(genDir, "thinking")); !os.IsNotExist(err) {
		t.Errorf("consumer profile must not create thinking/ dir, got err=%v", err)
	}

	// (2) tools/ has exactly Implemented tools + INDEX.md (no Planned pages).
	wantToolCount := implementedToolCount() + 1 // +INDEX.md
	if byCat["tools"] != wantToolCount {
		t.Errorf("consumer tools/ file count = %d, want %d (Implemented=%d + INDEX)", byCat["tools"], wantToolCount, implementedToolCount())
	}

	// (3) No atoms-*.md files (all four are empty-notice for an external graph).
	for _, name := range []string{"atoms-operator.md", "atoms-substrate.md", "atoms-discipline.md", "atoms-check.md"} {
		if _, err := os.Stat(filepath.Join(genDir, name)); err == nil {
			t.Errorf("consumer profile must skip empty atoms doc %s, but it exists", name)
		} else if !os.IsNotExist(err) {
			t.Errorf("stat %s: %v", name, err)
		}
	}

	// (4) INDEX.md still written.
	if _, err := os.Stat(filepath.Join(genDir, "tools", "INDEX.md")); err != nil {
		t.Errorf("consumer profile must still write tools/INDEX.md: %v", err)
	}

	// (5) Pin the exact total: root (14 non-atoms .md) + graph.json (1) +
	//     tools (Implemented + INDEX). This catches a regression that silently
	//     stops skipping any category.
	//     root = 12 fixed docs (incl. PIPELINE.md, TRACEABILITY.md, MODELS.md) +
	//     live-state.md + AGENT-CONTEXT.md = 14
	//     (DECISIONS/ENTITIES not written for this minimal graph)
	wantRoot := 14
	wantTotal := wantRoot + 1 + wantToolCount
	if total != wantTotal {
		t.Errorf("consumer total docs/gen file count = %d, want %d (root=%d + graph.json=1 + tools=%d)", total, wantTotal, wantRoot, wantToolCount)
	}

	// (6) written must report exactly the files that landed on disk.
	if len(consumerWritten) != total {
		t.Errorf("len(written)=%d != files on disk=%d — written must reflect only files actually written under the active profile", len(consumerWritten), total)
	}
}

// TestGenSpec_FullProfileUnchanged proves the full profile writes the SAME file
// set as today (no regression for existing domains), and that genSpec(...,"")
// (empty profile, manifest without gen_profile → resolves to "full") produces
// an identical file set to genSpec(...,"full").
func TestGenSpec_FullProfileUnchanged(t *testing.T) {
	t.Parallel()

	domainDir := t.TempDir()
	if _, err := initDomain(domainDir, "test-external", "2026-07-13"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}

	// Full profile
	fullWritten, _, err := genSpec(domainDir, "", "2026-07-13", "full")
	if err != nil {
		t.Fatalf("genSpec full: %v", err)
	}
	genDir := filepath.Join(domainDir, "docs", "gen")
	fullTotal, fullByCat := countFilesUnder(t, genDir)

	// (1) Full writes thinking docs.
	wantThinking := thinkingDocsCount()
	if fullByCat["thinking"] != wantThinking {
		t.Errorf("full thinking/ count = %d, want %d (methodology sections)", fullByCat["thinking"], wantThinking)
	}

	// (2) Full writes ALL tool docs + INDEX.
	wantToolFull := len(methodology.Tools.All()) + 1
	if fullByCat["tools"] != wantToolFull {
		t.Errorf("full tools/ count = %d, want %d (all tools + INDEX)", fullByCat["tools"], wantToolFull)
	}

	// (3) Full writes all 4 atoms docs (even when empty — the empty notice).
	for _, name := range []string{"atoms-operator.md", "atoms-substrate.md", "atoms-discipline.md", "atoms-check.md"} {
		if _, err := os.Stat(filepath.Join(genDir, name)); err != nil {
			t.Errorf("full profile must write atoms doc %s, got err: %v", name, err)
		}
	}

	// (4) Pin the exact total for full (regression guard).
	wantRootFull := 18 // 14 non-atoms (incl. PIPELINE.md, TRACEABILITY.md, MODELS.md) + 4 atoms
	wantTotalFull := wantRootFull + 1 + wantToolFull + wantThinking
	if fullTotal != wantTotalFull {
		t.Errorf("full total docs/gen file count = %d, want %d (root=%d + graph.json=1 + tools=%d + thinking=%d)", fullTotal, wantTotalFull, wantRootFull, wantToolFull, wantThinking)
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
	emptyWritten, _, err := genSpec(domainDir, "", "2026-07-13", "")
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

	// Two identical fresh domains, one consumer, one full.
	dirConsumer := t.TempDir()
	if _, err := initDomain(dirConsumer, "ext", "2026-07-13"); err != nil {
		t.Fatalf("initDomain consumer: %v", err)
	}
	if _, _, err := genSpec(dirConsumer, "", "2026-07-13", "consumer"); err != nil {
		t.Fatalf("genSpec consumer: %v", err)
	}
	consumerTotal, _ := countFilesUnder(t, filepath.Join(dirConsumer, "docs", "gen"))

	dirFull := t.TempDir()
	if _, err := initDomain(dirFull, "ext", "2026-07-13"); err != nil {
		t.Fatalf("initDomain full: %v", err)
	}
	if _, _, err := genSpec(dirFull, "", "2026-07-13", "full"); err != nil {
		t.Fatalf("genSpec full: %v", err)
	}
	fullTotal, _ := countFilesUnder(t, filepath.Join(dirFull, "docs", "gen"))

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
	plannedToolPage := filepath.Join("docs", "gen", "tools", plannedCmd+".md")

	domainDir := t.TempDir()
	if _, err := initDomain(domainDir, "test-external", "2026-07-13"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}
	genDir := filepath.Join(domainDir, "docs", "gen")
	thinkingDir := filepath.Join(genDir, "thinking")
	fullPlannedToolPage := filepath.Join(domainDir, plannedToolPage)

	// (1) full profile: thinking/*.md and the Planned-tool page exist on disk.
	if _, _, err := genSpec(domainDir, "", "2026-07-13", "full"); err != nil {
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
	consumerWritten, removed, err := genSpec(domainDir, "", "2026-07-13", "consumer")
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
	// hand-placed file — every stale GENERATOR file must be gone.
	total, _ := countFilesUnder(t, genDir)
	if total != len(consumerWritten)+1 {
		t.Errorf("consumer on-disk=%d, want written(%d)+1 (the hand-placed file) — a larger gap means stale generator output survived cleanup", total, len(consumerWritten))
	}

	// (3) full profile again on the same domain: everything restored. The
	// round-trip is non-destructive to content; only file PRESENCE cycles.
	if _, _, err := genSpec(domainDir, "", "2026-07-13", "full"); err != nil {
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
	if _, _, err := genSpec(domainDir, "", "2026-07-14", "consumer"); err != nil {
		t.Fatalf("genSpec consumer: %v", err)
	}

	reqPath := filepath.Join(domainDir, "docs", "gen", "REQUIREMENTS.md")
	content, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("read REQUIREMENTS.md: %v", err)
	}
	text := string(content)

	domainName := domainNameFromDir(domainDir)
	wantToken := "domains/" + domainName + "/docs/gen/tools/INDEX.md"
	if !strings.Contains(text, wantToken) {
		t.Fatalf("consumer REQUIREMENTS.md must reference %q, got:\n%s", wantToken, text)
	}
	if strings.Contains(text, "`docs/gen/tools/INDEX.md`") {
		t.Errorf("consumer REQUIREMENTS.md must NOT reference the bare, non-domain-prefixed form `docs/gen/tools/INDEX.md`")
	}

	resolved := filepath.Join(repoRoot, filepath.FromSlash(wantToken))
	if _, err := os.Stat(resolved); err != nil {
		t.Errorf("REQUIREMENTS.md references %q but it does not exist on disk at %s: %v", wantToken, resolved, err)
	}
}

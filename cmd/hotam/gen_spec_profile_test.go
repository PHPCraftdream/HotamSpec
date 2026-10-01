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

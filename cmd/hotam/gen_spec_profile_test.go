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
	if _, err := initDomain(domainDir, "test-external"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}
	// initDomain writes {"self_hosting": false} with NO gen_profile field,
	// so genSpec(...,"") resolves to "full" — proving the default is
	// backward-compatible. We pass explicit "consumer" here to test the cut.

	consumerWritten, err := genSpec(domainDir, "", "2026-07-13", "consumer")
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

	// (5) Pin the exact total: root (11 non-atoms .md) + graph.json (1) +
	//     tools (Implemented + INDEX). This catches a regression that silently
	//     stops skipping any category.
	//     root = 9 fixed docs + live-state.md + AGENT-CONTEXT.md = 11
	//     (DECISIONS/ENTITIES not written for this minimal graph)
	wantRoot := 11
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
	if _, err := initDomain(domainDir, "test-external"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}

	// Full profile
	fullWritten, err := genSpec(domainDir, "", "2026-07-13", "full")
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
	wantRootFull := 15 // 11 non-atoms + 4 atoms
	wantTotalFull := wantRootFull + 1 + wantToolFull + wantThinking
	if fullTotal != wantTotalFull {
		t.Errorf("full total docs/gen file count = %d, want %d (root=%d + graph.json=1 + tools=%d + thinking=%d)", fullTotal, wantTotalFull, wantRootFull, wantToolFull, wantThinking)
	}

	// (5) Empty profile against a manifest without gen_profile → resolves to
	//     "full" → same file set as explicit "full".
	emptyWritten, err := genSpec(domainDir, "", "2026-07-13", "")
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
	if _, err := initDomain(dirConsumer, "ext"); err != nil {
		t.Fatalf("initDomain consumer: %v", err)
	}
	if _, err := genSpec(dirConsumer, "", "2026-07-13", "consumer"); err != nil {
		t.Fatalf("genSpec consumer: %v", err)
	}
	consumerTotal, _ := countFilesUnder(t, filepath.Join(dirConsumer, "docs", "gen"))

	dirFull := t.TempDir()
	if _, err := initDomain(dirFull, "ext"); err != nil {
		t.Fatalf("initDomain full: %v", err)
	}
	if _, err := genSpec(dirFull, "", "2026-07-13", "full"); err != nil {
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

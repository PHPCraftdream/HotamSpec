package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

const selfDomainGraph = "../../domains/hotam-spec-self/graph.json"
const selfDomainManifest = "../../domains/hotam-spec-self/manifest.json"

func copySelfDomain(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	domainDir := filepath.Join(dir, "hotam-spec-self")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir domain: %v", err)
	}
	copyFile(t, selfDomainGraph, filepath.Join(domainDir, "graph.json"))
	copyFile(t, selfDomainManifest, filepath.Join(domainDir, "manifest.json"))
	return domainDir
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

// TestGenSpec_SmokeWritesByteIdenticalFiles verifies genSpec writes every
// expected file with non-empty content. It used to byte-compare against
// internal/generator/testdata/*.md golden files, but those were real-domain
// goldens replaced by a compact synthetic fixture in P2-2 (see
// internal/generator/byteidentical_test.go and fixture_test.go, which own
// the byte-identity guarantee against internal/generator/testdata/fixture/
// now). This test's job is narrower: prove genSpec's file-writing contract
// (which files, non-empty) holds against a real, large domain graph.
func TestGenSpec_SmokeWritesByteIdenticalFiles(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)

	written, err := genSpec(domainDir, "", "2026-07-12")
	if err != nil {
		t.Fatalf("genSpec: %v", err)
	}
	if len(written) == 0 {
		t.Fatal("genSpec wrote no files")
	}

	genDir := filepath.Join(domainDir, "docs", "gen")
	filenames := []string{
		"REQUIREMENTS.md", "TENSIONS.md", "OPEN.md", "UNENFORCED.md",
		"GLOSSARY.md", "HISTORY.md", "CONSTITUTION.md", "FRAMEWORK-INVARIANTS.md",
		"REPO-MAP.md", "atoms-operator.md", "atoms-substrate.md",
		"atoms-discipline.md", "atoms-check.md", "graph.json",
	}
	for _, filename := range filenames {
		got, err := os.ReadFile(filepath.Join(genDir, filename))
		if err != nil {
			t.Fatalf("read generated %s: %v", filename, err)
		}
		if len(got) == 0 {
			t.Errorf("%s: written but empty", filename)
		}
	}

	liveState := filepath.Join(genDir, "live-state.md")
	if _, err := os.Stat(liveState); err != nil {
		t.Errorf("live-state.md not written: %v", err)
	}

	thinkingDir := filepath.Join(genDir, "thinking")
	entries, err := os.ReadDir(thinkingDir)
	if err != nil || len(entries) == 0 {
		t.Errorf("thinking/ docs not written: %v", err)
	}
	toolsDir := filepath.Join(genDir, "tools")
	entries, err = os.ReadDir(toolsDir)
	if err != nil || len(entries) == 0 {
		t.Errorf("tools/ docs not written: %v", err)
	}

	decPath := filepath.Join(genDir, "DECISIONS.md")
	if _, err := os.Stat(decPath); err == nil {
		t.Error("DECISIONS.md should be skipped (DecisionsMDHasContent=false)")
	}
	entPath := filepath.Join(genDir, "ENTITIES.md")
	if _, err := os.Stat(entPath); err == nil {
		t.Error("ENTITIES.md should be skipped (EntitiesMDHasContent=false)")
	}
}

func TestWhatNow_SmokeNoPanic(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	out, err := whatNow(domainDir, 20, "2026-07-12")
	if err != nil {
		t.Fatalf("whatNow: %v", err)
	}
	if out == "" {
		t.Fatal("whatNow returned empty output")
	}
}

func TestAllViolations_SmokeNoPanic(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	violations, err := allViolations(domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	if violations == nil {
		t.Log("allViolations returned nil slice (graph clean)")
	}
}

func TestParseProposal_Requirement(t *testing.T) {
	t.Parallel()
	json := `{"kind":"Requirement","ID":"R-smoke","Claim":"smoke claim","Owner":"sa","Status":"DRAFT"}`
	p, err := parseProposal([]byte(json))
	if err != nil {
		t.Fatalf("parseProposal: %v", err)
	}
	if p.Kind() != "Requirement" {
		t.Errorf("Kind = %q, want Requirement", p.Kind())
	}
	if p.TargetAnchor() != "R-smoke" {
		t.Errorf("TargetAnchor = %q, want R-smoke", p.TargetAnchor())
	}
}

func TestParseProposal_UnknownKind(t *testing.T) {
	t.Parallel()
	json := `{"kind":"Bogus"}`
	_, err := parseProposal([]byte(json))
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

func TestReorderFlagsFirst_ProposalBeforeFlags(t *testing.T) {
	t.Parallel()
	args := []string{"proposal.json", "--domain", "/tmp/x", "--today", "2026-07-12"}
	got := reorderFlagsFirst(args)
	want := []string{"--domain", "/tmp/x", "--today", "2026-07-12", "proposal.json"}
	if len(got) != len(want) {
		t.Fatalf("len got %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestReorderFlagsFirst_EqualsForm(t *testing.T) {
	t.Parallel()
	args := []string{"proposal.json", "--domain=/tmp/x"}
	got := reorderFlagsFirst(args)
	if len(got) != 2 || got[0] != "--domain=/tmp/x" || got[1] != "proposal.json" {
		t.Errorf("got %v", got)
	}
}

func TestApplyProposal_SmokeEndToEnd(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	proposalJSON := `{"kind":"Requirement","ID":"R-smoke-test","Claim":"smoke claim","Owner":"framework-author","Status":"DRAFT","Why":"smoke"}`
	p, err := parseProposal([]byte(proposalJSON))
	if err != nil {
		t.Fatalf("parseProposal: %v", err)
	}
	gp := graphPathForDomain(domainDir)
	if err := proposal.Apply(gp, "2026-07-13", p); err != nil {
		t.Fatalf("apply: %v", err)
	}
	data, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	if !contains(string(data), "R-smoke-test") {
		t.Error("R-smoke-test not found in graph after apply")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestGenSpec_ClaudeMDRuneCount(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)

	content := "Hello, 世界!\nThis is a test string."
	claudeMDPath := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write claude md: %v", err)
	}

	_, err := genSpec(domainDir, claudeMDPath, "2026-07-12")
	if err != nil {
		t.Fatalf("genSpec with claude-md: %v", err)
	}

	liveStatePath := filepath.Join(domainDir, "docs", "gen", "live-state.md")
	data, err := os.ReadFile(liveStatePath)
	if err != nil {
		t.Fatalf("read live-state.md: %v", err)
	}

	expected := utf8.RuneCountInString(content)
	want := fmt.Sprintf("resident crystal %d chars", expected)
	if !contains(string(data), want) {
		t.Errorf("live-state.md does not contain %q\nactual content:\n%s", want, string(data))
	}
}

// TestGenSpec_SameTodayIsByteIdenticalIncludingCrystal is the CLI-level
// idempotency proof CI's regen-idempotency step (.github/workflows/ci.yml)
// relies on: running genSpec (the same code path `hotam gen-spec --claude-md
// <path> --today <date>` drives) TWICE with the SAME --today value, against
// the same domain fixture, must produce byte-identical docs/gen/*.md AND a
// byte-identical root CLAUDE.md — independent of wall-clock time. Before the
// today-threading fix, internal/generator/agentcontext.go and
// internal/diagnose/freshness_signals.go each called time.Now() internally,
// so this property was structurally impossible to guarantee: a CI run today
// and a CI run tomorrow (or a dev machine regenerating on a different day
// than the committed crystal) would show a spurious byte diff purely from
// the embedded date, independent of any real graph drift.
func TestGenSpec_SameTodayIsByteIdenticalIncludingCrystal(t *testing.T) {
	t.Parallel()
	domainDirA := copySelfDomain(t)
	domainDirB := copySelfDomain(t)

	claudeMDA := filepath.Join(t.TempDir(), "CLAUDE.md")
	claudeMDB := filepath.Join(t.TempDir(), "CLAUDE.md")

	const today = "2026-07-12"
	if _, err := genSpec(domainDirA, claudeMDA, today); err != nil {
		t.Fatalf("genSpec (first run): %v", err)
	}
	if _, err := genSpec(domainDirB, claudeMDB, today); err != nil {
		t.Fatalf("genSpec (second run): %v", err)
	}

	// The rendered root CLAUDE.md must be byte-identical between the two
	// independent runs (same today, same source graph, different wall-clock
	// moment the test happened to execute at).
	crystalA, err := os.ReadFile(claudeMDA)
	if err != nil {
		t.Fatalf("read first CLAUDE.md: %v", err)
	}
	crystalB, err := os.ReadFile(claudeMDB)
	if err != nil {
		t.Fatalf("read second CLAUDE.md: %v", err)
	}
	if string(crystalA) != string(crystalB) {
		t.Error("CLAUDE.md differs between two genSpec runs with the same --today value — root crystal regeneration is not idempotent")
	}

	// Same check for docs/gen/AGENT-CONTEXT.md and docs/gen/live-state.md,
	// the two files that embed today-derived text most directly.
	for _, rel := range []string{
		filepath.Join("docs", "gen", "AGENT-CONTEXT.md"),
		filepath.Join("docs", "gen", "live-state.md"),
	} {
		dataA, err := os.ReadFile(filepath.Join(domainDirA, rel))
		if err != nil {
			t.Fatalf("read first %s: %v", rel, err)
		}
		dataB, err := os.ReadFile(filepath.Join(domainDirB, rel))
		if err != nil {
			t.Fatalf("read second %s: %v", rel, err)
		}
		if string(dataA) != string(dataB) {
			t.Errorf("%s differs between two genSpec runs with the same --today value — not idempotent", rel)
		}
	}
}

// TestVersion_DefaultAndLdflagsOverride checks both a plain (default
// version = "dev") build and one with -ldflags "-X main.version=..."
// (plus commit/buildDate) print the expected `version`/`--version` line.
// The plain build is shared (buildSharedHotamBinary); only the ldflags build
// is specific to this test.
// See external_e2e_test.go for the full external e2e which also uses the
// shared binary.
func TestVersion_DefaultAndLdflagsOverride(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a real binary; skipped in -short")
	}
	t.Parallel()
	repoRoot := repoRootForTest(t)
	binDir := t.TempDir()

	const defaultLine = "hotam dev (commit: unknown, built: unknown)"

	// Default (no ldflags) build is shared across this package's tests
	// (see testbinary_test.go) — only the ldflags-injected build below is
	// specific to this test and needs its own `go build`.
	binPath := buildSharedHotamBinary(t)
	out, err := exec.Command(binPath, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("hotam version: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != defaultLine {
		t.Errorf("hotam version = %q, want %q", strings.TrimSpace(string(out)), defaultLine)
	}
	out, err = exec.Command(binPath, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("hotam --version: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != defaultLine {
		t.Errorf("hotam --version = %q, want %q", strings.TrimSpace(string(out)), defaultLine)
	}

	// Inject all three ldflags vars to verify the full wiring.
	binPathLdflags := filepath.Join(binDir, "hotam-ldflags"+filepath.Ext(binPath))
	buildLd := exec.Command("go", "build", "-ldflags",
		"-X main.version=v0.9.9 -X main.commit=abc1234 -X main.buildDate=2026-01-02",
		"-o", binPathLdflags, "./cmd/hotam")
	buildLd.Dir = repoRoot
	if out, err := buildLd.CombinedOutput(); err != nil {
		t.Fatalf("go build (ldflags version): %v\n%s", err, out)
	}
	out, err = exec.Command(binPathLdflags, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("hotam-ldflags version: %v\n%s", err, out)
	}
	const ldflagsLine = "hotam v0.9.9 (commit: abc1234, built: 2026-01-02)"
	if strings.TrimSpace(string(out)) != ldflagsLine {
		t.Errorf("hotam version (ldflags) = %q, want %q", strings.TrimSpace(string(out)), ldflagsLine)
	}
}

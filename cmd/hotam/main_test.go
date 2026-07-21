package main

import (
	"encoding/json"
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

// copySelfDomain scaffolds the self-hosting domain fixture at
// <tempRoot>/domains/hotam-spec-self and returns the domain dir. The
// domains/<name> parent makes repoRootForDomain's tier-1 resolve the project
// root to the temp root itself, so the fixture is HERMETIC: it never leaks
// through tier-2's CWD-based ProjectRootOrRaise() walk to THIS repo's real
// root (which carries a real CLAUDE.md + .hotam-spec-project marker). Without
// this, any land without --claude-md would auto-write the real crystal
// (resolveClaudeMDPath), and DOMAIN-MAP rendering would list the real repo's
// sibling domains — both cross-test contamination. Tests that need to control
// whether the project root carries a crystal/marker use
// copySelfDomainUnderRoot directly.
func copySelfDomain(t *testing.T) string {
	t.Helper()
	_, domainDir := copySelfDomainUnderRoot(t)
	return domainDir
}

// copySelfDomainUnderRoot scaffolds the self-hosting domain fixture at
// <root>/domains/hotam-spec-self and returns BOTH the synthetic project root
// and the domain dir. Placing the domain under a domains/ parent makes
// repoRootForDomain's tier-1 (<root>/domains/<name>) resolve the project root
// to the temp root itself, so resolveClaudeMDPath's os.Stat checks run against
// the temp root (fully test-controlled) rather than leaking through tier-2's
// CWD-based ProjectRootOrRaise() walk to THIS repo's real root (which carries
// a real CLAUDE.md + .hotam-spec-project marker and would otherwise be
// contaminated by any land auto-crystal-write). Callers that WANT the land
// auto-crystal-write to fire create <root>/CLAUDE.md or .hotam-spec-project;
// callers that DON'T leave the root empty so resolveClaudeMDPath returns "".
func copySelfDomainUnderRoot(t *testing.T) (projectRoot, domainDir string) {
	t.Helper()
	projectRoot = t.TempDir()
	domainDir = filepath.Join(projectRoot, "domains", "hotam-spec-self")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir domain: %v", err)
	}
	copyFile(t, selfDomainGraph, filepath.Join(domainDir, "graph.json"))
	copySelfDomainManifestSansOrientationFAQ(t, filepath.Join(domainDir, "manifest.json"))
	return projectRoot, domainDir
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

// copySelfDomainManifestSansOrientationFAQ copies selfDomainManifest to dst
// with its "orientation_faq" field stripped. The real hotam-spec-self
// manifest.json opts into that field (R-orientation-faq-answerable /
// check_orientation_faq_answered): every declared question's answer must be
// reachable in <=1 hop from THIS domain's OWN generated crystal, via links
// that are literally "domains/hotam-spec-self/docs/gen/....md" — a promise
// that is only true for the real domain at its real repo path with its real
// docs/gen/ tree and a real generated crystal on disk.
//
// The many test fixtures across this package that copy selfDomainManifest are
// generic "any invariant-clean graph+manifest pair will do" doubles for
// unrelated mechanics (land/gen-spec/propose/confront plumbing) — they place
// the domain under throwaway temp roots, frequently under a DIFFERENT domain
// name/path (e.g. "marked-domain"), frequently without ever running gen-spec
// to produce a crystal at all (e.g. the --json contract tests, which assert
// on stdout shape, not on crystal content). Carrying the opt-in into those
// copies would make check_orientation_faq_answered fire spuriously against
// fixtures that were never meant to exercise — let alone satisfy — the
// orientation-showcase contract, which is exclusively covered by the real
// self-example (TestCheckOrientationFAQAnswered_RealHotamSpecSelfSelfExample
// in internal/invariants, plus `all-violations` against the real domain).
// Stripping the field here keeps every one of those fixtures an honest no-op
// for this check, the same "no committed opt-in = no lie" boundary the check
// itself documents.
func copySelfDomainManifestSansOrientationFAQ(t *testing.T, dst string) {
	t.Helper()
	data, err := os.ReadFile(selfDomainManifest)
	if err != nil {
		t.Fatalf("read %s: %v", selfDomainManifest, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", selfDomainManifest, err)
	}
	delete(m, "orientation_faq")
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal stripped manifest: %v", err)
	}
	if err := os.WriteFile(dst, out, 0o644); err != nil {
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

	written, _, err := genSpec(domainDir, "", "2026-07-12", "", false)
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
		"PIPELINE.md", "TRACEABILITY.md", "MODELS.md", "COVERAGE.md",
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

// TestClearInheritedVerifiedByGuard_UnsetsEnv is the in-process unit proof
// that main()'s CLI-entry guard-clear actually removes
// HOTAM_VERIFIED_BY_EXEC_GUARD from this process's own environment --
// complementing the real-subprocess kill-switch reproductions in
// killswitch_e2e_test.go, which prove the end-to-end effect through the
// compiled binary. This test proves the specific function main() calls does
// what it claims, independent of process-spawn overhead.
func TestClearInheritedVerifiedByGuard_UnsetsEnv(t *testing.T) {
	const guardEnv = "HOTAM_VERIFIED_BY_EXEC_GUARD"
	prev, hadPrev := os.LookupEnv(guardEnv)
	if err := os.Setenv(guardEnv, "externally-forged-value"); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	t.Cleanup(func() {
		if hadPrev {
			os.Setenv(guardEnv, prev)
		} else {
			os.Unsetenv(guardEnv)
		}
	})

	clearInheritedVerifiedByGuard()

	if v, ok := os.LookupEnv(guardEnv); ok {
		t.Fatalf("expected %s to be unset after clearInheritedVerifiedByGuard, got %q", guardEnv, v)
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

// TestReorderFlagsFirst_BooleanFlagDoesNotConsumePositional is the unit proof
// for the boolean-flag fix: a bare --json (the package's only value-less flag)
// placed BEFORE a positional must NOT swallow that positional as its "value".
// Pre-fix, reorderFlagsFirst saw "--json" followed by a non-dash token and
// moved the claim text into the flags group, so the positional never reached
// the subcommand. Here the claim text must survive as a positional (land after
// all flags), not get pulled into the flags group.
func TestReorderFlagsFirst_BooleanFlagDoesNotConsumePositional(t *testing.T) {
	t.Parallel()
	args := []string{"--json", "some claim text", "--domain", "X"}
	got := reorderFlagsFirst(args)
	want := []string{"--json", "--domain", "X", "some claim text"}
	if len(got) != len(want) {
		t.Fatalf("len got %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
	// The claim text must be the trailing positional, never inside the flags.
	if got[len(got)-1] != "some claim text" {
		t.Errorf("claim text did not survive as the trailing positional: %v", got)
	}
	for _, g := range got[:len(got)-1] {
		if g == "some claim text" {
			t.Errorf("claim text wrongly pulled into the flags group: %v", got)
		}
	}
}

// TestIsBoolFlag covers the dash/equals-stripping: bare, single-dash, and
// =value forms all resolve to the boolean set; value-taking names do not.
func TestIsBoolFlag(t *testing.T) {
	t.Parallel()
	for _, tok := range []string{"--json", "-json", "--json=true", "--json=false"} {
		if !isBoolFlag(tok) {
			t.Errorf("isBoolFlag(%q) = false, want true", tok)
		}
	}
	for _, tok := range []string{"--domain", "--domain=/x", "-", "--unknown"} {
		if isBoolFlag(tok) {
			t.Errorf("isBoolFlag(%q) = true, want false", tok)
		}
	}
}

// TestIsBoolFlag_HelpSpellings is the unit proof that every dash spelling of
// help ("-h", "--h", "-help", "--help") normalizes (via isBoolFlag's leading
// "-" strip) to the same boolFlagNames entries added for finding N2
// (`hotam propose requirement -h` printed the generic usage line instead of
// per-flag help). -h/-help are NOT registered via any fs.Bool(...) call
// anywhere in this codebase — they are Go's stdlib flag package's own
// built-in help recognition — but reorderFlagsFirst and cmdPropose's
// extractProposeKind both need to treat them as value-less BEFORE any
// FlagSet exists to ask, which is why they live in boolFlagNames too.
func TestIsBoolFlag_HelpSpellings(t *testing.T) {
	t.Parallel()
	for _, tok := range []string{"-h", "--h", "-help", "--help"} {
		if !isBoolFlag(tok) {
			t.Errorf("isBoolFlag(%q) = false, want true (help spellings must be treated as value-less)", tok)
		}
	}
}

// TestReorderFlagsFirst_HelpFlagDoesNotConsumePositional is the
// reorderFlagsFirst-level proof for finding N2: given
// ["requirement", "-h"] (as `hotam propose requirement -h` produces after
// os.Args[2:] splitting), -h must NOT be treated as consuming a following
// token as its value — there is none here, but critically -h itself must
// end up correctly recognized as a value-less bool flag so downstream
// scanners (extractProposeKind) see it as a bare flag, not a flag+value
// pair that swallows "requirement".
func TestReorderFlagsFirst_HelpFlagDoesNotConsumePositional(t *testing.T) {
	t.Parallel()
	got := reorderFlagsFirst([]string{"requirement", "-h"})
	want := []string{"-h", "requirement"}
	if len(got) != len(want) {
		t.Fatalf("len got %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
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

func TestGenSpec_CrystalCharCountIsRenderedFixpoint(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)

	// A bogus, deliberately wrong pre-existing file at the --claude-md path.
	// Pre-fix, genSpec read THIS file's rune count and embedded it into
	// LIVE-STATE; this test fails against that bug because the bogus count
	// cannot equal the rendered crystal's real rune count.
	bogus := "BOGUS pre-existing crystal content — must be ignored, not measured.\n"
	claudeMDPath := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte(bogus), 0o644); err != nil {
		t.Fatalf("write bogus claude md: %v", err)
	}

	if _, _, err := genSpec(domainDir, claudeMDPath, "2026-07-12", "", false); err != nil {
		t.Fatalf("genSpec with claude-md: %v", err)
	}

	// The rendered crystal's actual rune count is the fixpoint that must be
	// embedded everywhere.
	crystal, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("read rendered CLAUDE.md: %v", err)
	}
	crystalRunes := utf8.RuneCountInString(string(crystal))
	want := fmt.Sprintf("resident crystal %d chars", crystalRunes)

	liveState, err := os.ReadFile(filepath.Join(domainDir, "docs", "gen", "live-state.md"))
	if err != nil {
		t.Fatalf("read live-state.md: %v", err)
	}
	if !contains(string(liveState), want) {
		t.Errorf("live-state.md does not embed the rendered crystal's own rune count %q (the bogus pre-existing file's size must NOT be measured)\nactual content:\n%s", want, string(liveState))
	}

	// AGENT-CONTEXT.md carries the same LIVE-STATE block and must agree with
	// the root crystal — no mode-dependent "0 chars" disagreement.
	agentContext, err := os.ReadFile(filepath.Join(domainDir, "docs", "gen", "AGENT-CONTEXT.md"))
	if err != nil {
		t.Fatalf("read AGENT-CONTEXT.md: %v", err)
	}
	if !contains(string(agentContext), want) {
		t.Errorf("AGENT-CONTEXT.md does not embed the same resident-crystal count %q — the two LIVE-STATE carriers must agree\nactual content:\n%s", want, string(agentContext))
	}

	if crystalRunes == utf8.RuneCountInString(bogus) {
		t.Errorf("embedded crystal count %d equals the bogus pre-existing file's size — the stale-file read was not removed", crystalRunes)
	}
	if crystalRunes == 0 {
		t.Errorf("embedded crystal count is 0 — the fixpoint measurement was not computed (the former no---claude-md \"0 chars\" bug)")
	}
}

// TestGenSpec_CrystalFixpointConvergesAcrossRuns is the convergence proof for
// the self-referential crystal-size measurement: running genSpec TWICE over
// the SAME tree (same domain dir, same --claude-md path, same --today) must
// produce byte-identical output, including the LIVE-STATE "resident crystal N
// chars" line specifically. Pre-fix, genSpec read the size of the
// PRE-EXISTING CLAUDE.md (written by the previous run) BEFORE regenerating, so
// pass 2 embedded pass 1's file size into a differently-sized file — the
// "resident crystal N chars" line changed every pass and never converged,
// which is exactly why CI's regen-idempotency check (regen twice, diff) was
// red. This reproduces that exact two-pass-over-one-tree scenario.
func TestGenSpec_CrystalFixpointConvergesAcrossRuns(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	claudeMDPath := filepath.Join(t.TempDir(), "CLAUDE.md")
	const today = "2026-07-12"

	// Pass 1: no pre-existing CLAUDE.md exists yet.
	if _, _, err := genSpec(domainDir, claudeMDPath, today, "", false); err != nil {
		t.Fatalf("genSpec (pass 1): %v", err)
	}
	crystal1, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("read CLAUDE.md (pass 1): %v", err)
	}
	liveState1, err := os.ReadFile(filepath.Join(domainDir, "docs", "gen", "live-state.md"))
	if err != nil {
		t.Fatalf("read live-state.md (pass 1): %v", err)
	}
	agentContext1, err := os.ReadFile(filepath.Join(domainDir, "docs", "gen", "AGENT-CONTEXT.md"))
	if err != nil {
		t.Fatalf("read AGENT-CONTEXT.md (pass 1): %v", err)
	}

	// Pass 2: over the SAME tree — now CLAUDE.md exists from pass 1, which is
	// exactly the state that triggered the stale-read bug.
	if _, _, err := genSpec(domainDir, claudeMDPath, today, "", false); err != nil {
		t.Fatalf("genSpec (pass 2): %v", err)
	}
	crystal2, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("read CLAUDE.md (pass 2): %v", err)
	}
	liveState2, err := os.ReadFile(filepath.Join(domainDir, "docs", "gen", "live-state.md"))
	if err != nil {
		t.Fatalf("read live-state.md (pass 2): %v", err)
	}
	agentContext2, err := os.ReadFile(filepath.Join(domainDir, "docs", "gen", "AGENT-CONTEXT.md"))
	if err != nil {
		t.Fatalf("read AGENT-CONTEXT.md (pass 2): %v", err)
	}

	if string(crystal1) != string(crystal2) {
		t.Errorf("CLAUDE.md differs between two genSpec passes over the same tree — the resident-crystal size measurement did not converge:\n--- pass 1 live-state ---\n%s\n--- pass 2 live-state ---\n%s", liveState1, liveState2)
	}
	if string(liveState1) != string(liveState2) {
		t.Errorf("live-state.md differs between two genSpec passes over the same tree — not converged:\n--- pass 1 ---\n%s\n--- pass 2 ---\n%s", liveState1, liveState2)
	}
	if string(agentContext1) != string(agentContext2) {
		t.Errorf("AGENT-CONTEXT.md differs between two genSpec passes over the same tree — not converged")
	}

	// The converged embedded number must equal the crystal's actual rune
	// count (the fixpoint), proving the LIVE-STATE line is self-consistent.
	want := fmt.Sprintf("resident crystal %d chars", utf8.RuneCountInString(string(crystal2)))
	if !contains(string(liveState2), want) {
		t.Errorf("live-state.md embedded number is not the rendered crystal's own rune count %q\nactual:\n%s", want, string(liveState2))
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
	if _, _, err := genSpec(domainDirA, claudeMDA, today, "", false); err != nil {
		t.Fatalf("genSpec (first run): %v", err)
	}
	if _, _, err := genSpec(domainDirB, claudeMDB, today, "", false); err != nil {
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

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// crystalPathTokenRE extracts backtick-wrapped path-like tokens from a
// rendered crystal (CLAUDE.md/AGENTS.md/GEMINI.md). It is deliberately
// permissive (a test helper, not production code): it matches ANY
// non-whitespace, non-backtick run inside a single backtick span, then the
// caller filters to genuine relative-file-path tokens by requiring the
// candidate contain "docs/gen/" or "domains/" AND end in a recognizable file
// extension (.md/.json) — excluding backtick-wrapped commands like
// `hotam gate <target-anchor>` (no file extension) or bare identifiers like
// `hotam-spec-self` (no docs/gen//domains/ substring).
var crystalBacktickSpanRE = regexp.MustCompile("`([^`\n]+)`")

// pathLikeSuffixRE matches a trailing .md or .json extension, optionally
// followed by punctuation the surrounding prose might have attached (a
// trailing period ending a sentence, e.g. "...HISTORY.md.").
var pathLikeSuffixRE = regexp.MustCompile(`\.(md|json)\.?$`)

// extractCrystalPathTokens scans rendered crystal text for backtick-wrapped
// path tokens that reference a docs/gen/ or domains/ relative file path, and
// returns the cleaned (trailing-sentence-period stripped) path list, sorted
// and deduplicated. This is the link-existence test's extraction logic
// (task #135 / R6-e): every such token is a claim the crystal makes about a
// file that must actually exist on disk once generated.
func extractCrystalPathTokens(t *testing.T, text string) []string {
	t.Helper()
	seen := map[string]bool{}
	var out []string
	for _, m := range crystalBacktickSpanRE.FindAllStringSubmatch(text, -1) {
		candidate := m[1]
		// Reject tokens that carry whitespace (a command like "hotam gate
		// <target-anchor>" or "hotam status --json") -- a real relative file
		// path is a single unbroken token with no interior space.
		if strings.ContainsAny(candidate, " \t") {
			continue
		}
		if !strings.Contains(candidate, "docs/gen/") && !strings.HasPrefix(candidate, "domains/") {
			continue
		}
		if !pathLikeSuffixRE.MatchString(candidate) {
			continue
		}
		// Reject templated (non-literal) paths carrying a "<placeholder>"
		// segment, e.g. "domains/X/docs/gen/thinking/<slug>.md" in the
		// EMBEDDED-THINKING intro -- <slug> is never a real file on disk by
		// construction (it names the whole directory's file-naming scheme,
		// not one concrete file), so it is not a link-existence claim.
		if strings.Contains(candidate, "<") || strings.Contains(candidate, ">") {
			continue
		}
		// Strip a trailing sentence-ending period that isn't part of the
		// extension itself (".md." -> ".md"), leaving ".json"/".md" intact.
		cleaned := candidate
		if strings.HasSuffix(cleaned, ".md.") {
			cleaned = strings.TrimSuffix(cleaned, ".")
		} else if strings.HasSuffix(cleaned, ".json.") {
			cleaned = strings.TrimSuffix(cleaned, ".")
		}
		if seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		out = append(out, cleaned)
	}
	return out
}

// resolveCrystalPathToken maps an extracted crystal path token to a
// filesystem path relative to repoRoot. Two shapes appear in the crystal:
//   - already domain-qualified: "domains/<name>/docs/gen/X.md" -- resolved
//     directly under repoRoot.
//   - bare "docs/gen/X.md" -- this shape should NEVER survive the fix (it is
//     exactly bug 1 the fix closes), so it is resolved literally at
//     repoRoot/docs/gen/X.md, which correctly does NOT exist -- proving the
//     test fails loudly if a bare path regresses back into the rendered
//     crystal.
func resolveCrystalPathToken(repoRoot, token string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(token))
}

// assertCrystalLinksExist renders the given domain's crystal (via genSpec)
// under the given profile and asserts every extracted path token resolves to
// a real file on disk relative to repoRoot. profileLabel is used only for
// failure-message clarity.
func assertCrystalLinksExist(t *testing.T, repoRoot, domainDir, claudeMDPath, profile, profileLabel string) {
	t.Helper()
	if _, _, err := genSpec(domainDir, claudeMDPath, "2026-07-14", profile); err != nil {
		t.Fatalf("genSpec (%s): %v", profileLabel, err)
	}
	crystal, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("read rendered crystal (%s): %v", profileLabel, err)
	}
	tokens := extractCrystalPathTokens(t, string(crystal))
	if len(tokens) == 0 {
		t.Fatalf("(%s) extracted zero path tokens from the rendered crystal -- extraction regex likely broken", profileLabel)
	}
	for _, tok := range tokens {
		p := resolveCrystalPathToken(repoRoot, tok)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("(%s) crystal references %q but it does not exist on disk at %s: %v", profileLabel, tok, p, err)
		}
	}
}

// TestCrystalLinks_EveryReferencedPathExistsOnDisk is the link-existence
// acceptance test for task #135 (review-6 R6-e): every backtick-wrapped
// docs/gen/ or domains/... relative path the rendered root crystal claims to
// exist must actually exist on disk, for BOTH the full and consumer gen-spec
// profiles.
//
// Before the fix this failed two ways:
//   - bug 1 (bare paths): mediationLoopText's `docs/gen/TENSIONS.md` /
//     `docs/gen/REQUIREMENTS.md`, claudemd.go's `docs/gen/HISTORY.md`, and
//     `docs/gen/tools/INDEX.md` had no domain prefix, so they resolved to
//     <repoRoot>/docs/gen/... which never exists (every domain's docs live
//     under domains/<name>/docs/gen/ -- see cmd/hotam/init_project.go:118).
//     This failure is REACHABLE under BOTH profiles (unconditional bug).
//   - bug 2 (profile-unaware thinking/ references): neither of the two
//     thinking/-referencing clauses is a concrete extractable file-path token
//     for THIS test -- the boot line's "Deep-dives:
//     `domains/<name>/docs/gen/thinking/`" is a directory reference (no
//     .md/.json suffix) and the EMBEDDED-THINKING intro's
//     "domains/<name>/docs/gen/thinking/<slug>.md" is a templated
//     (non-literal, `<slug>` placeholder) path, both deliberately excluded by
//     this extractor's filters. Bug 2's reachable failure is exercised
//     separately by TestCrystalLinks_ConsumerNeverReferencesThinkingDir below
//     (directory-reference existence, not a concrete .md file).
//
// Uses two SEPARATE domains under two SEPARATE repo roots (rather than one
// domain re-genspec'd twice) so the full-profile pass's thinking/*.md files
// are not still on disk to accidentally make the consumer-profile pass look
// correct (cleanupStaleGenFiles already deletes them on a real profile
// switch -- task #133 -- but an independent domain per profile is a more
// direct, less coupled test setup here).
func TestCrystalLinks_EveryReferencedPathExistsOnDisk(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		profile string
		label   string
	}{
		{"full", "full profile"},
		{"consumer", "consumer profile"},
	} {
		tc := tc
		t.Run(tc.label, func(t *testing.T) {
			t.Parallel()
			repoRoot := t.TempDir()
			domainDir := filepath.Join(repoRoot, "domains", "test-linkcheck")
			if _, err := initDomain(domainDir, "test-linkcheck"); err != nil {
				t.Fatalf("initDomain: %v", err)
			}
			claudeMDPath := filepath.Join(repoRoot, "CLAUDE.md")
			assertCrystalLinksExist(t, repoRoot, domainDir, claudeMDPath, tc.profile, tc.label)
		})
	}
}

// TestCrystalLinks_ConsumerNeverReferencesThinkingDir enforces the bug-2 half
// of task #135 directly: under the consumer profile, genSpec never writes
// docs/gen/thinking/ at all (cmd/hotam/gen_spec.go's `if !consumer {
// thinkingDocs := ... }` gate), so the rendered crystal must not point at
// that directory. This complements
// TestCrystalLinks_EveryReferencedPathExistsOnDisk, whose file-extension
// filter cannot catch a directory-only reference (the boot line's
// "Deep-dives: `domains/<name>/docs/gen/thinking/`" clause) or a templated
// non-literal path (the EMBEDDED-THINKING intro's ".../thinking/<slug>.md").
func TestCrystalLinks_ConsumerNeverReferencesThinkingDir(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	domainDir := filepath.Join(repoRoot, "domains", "test-linkcheck-consumer")
	if _, err := initDomain(domainDir, "test-linkcheck-consumer"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}
	claudeMDPath := filepath.Join(repoRoot, "CLAUDE.md")
	if _, _, err := genSpec(domainDir, claudeMDPath, "2026-07-14", "consumer"); err != nil {
		t.Fatalf("genSpec consumer: %v", err)
	}
	crystal, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("read rendered crystal: %v", err)
	}
	text := string(crystal)
	if strings.Contains(text, "docs/gen/thinking/") {
		t.Errorf("consumer-profile crystal must not reference docs/gen/thinking/ (never written under consumer), but it does:\n%s", text)
	}
	thinkingDir := filepath.Join(domainDir, "docs", "gen", "thinking")
	if _, err := os.Stat(thinkingDir); !os.IsNotExist(err) {
		t.Fatalf("test precondition failed: consumer profile should not have written %s, stat err=%v", thinkingDir, err)
	}
}

// TestCrystalLinks_FullProfileStillReferencesThinkingDir is the full-profile
// mirror: full profile DOES write docs/gen/thinking/, so the crystal's boot
// line and EMBEDDED-THINKING intro must still reference it (byte-identical to
// pre-fix full-profile behavior -- part of the byte-identity guarantee).
func TestCrystalLinks_FullProfileStillReferencesThinkingDir(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	domainDir := filepath.Join(repoRoot, "domains", "test-linkcheck-full")
	if _, err := initDomain(domainDir, "test-linkcheck-full"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}
	claudeMDPath := filepath.Join(repoRoot, "CLAUDE.md")
	if _, _, err := genSpec(domainDir, claudeMDPath, "2026-07-14", "full"); err != nil {
		t.Fatalf("genSpec full: %v", err)
	}
	crystal, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("read rendered crystal: %v", err)
	}
	text := string(crystal)
	if !strings.Contains(text, "domains/test-linkcheck-full/docs/gen/thinking/") {
		t.Errorf("full-profile crystal must still reference docs/gen/thinking/, but it does not:\n%s", text)
	}
	thinkingDir := filepath.Join(domainDir, "docs", "gen", "thinking")
	entries, err := os.ReadDir(thinkingDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("test precondition failed: full profile should have written docs/gen/thinking/*.md, dir=%s err=%v entries=%d", thinkingDir, err, len(entries))
	}
}

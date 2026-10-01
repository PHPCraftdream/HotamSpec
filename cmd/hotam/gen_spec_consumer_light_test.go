package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// listFilesUnder returns the slash-relative file paths under dir, sorted.
func listFilesUnder(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	sort.Strings(out)
	return out
}

// TestGenSpec_ConsumerProfileIsLightweight pins the consumer file set: only
// the docs that carry their own data (REQUIREMENTS, UNENFORCED for this
// graph), no engine self-documentation, no project-shared framework/ files.
func TestGenSpec_ConsumerProfileIsLightweight(t *testing.T) {
	t.Parallel()

	projectRoot, domainDir := initDomainUnderRoot(t, "test-external", "2026-07-13")
	seedMinimalRequirement(t, domainDir, "test-external", "2026-07-13")

	written, _, err := genSpec(domainDir, "", "2026-07-13", "consumer", false)
	if err != nil {
		t.Fatalf("genSpec consumer: %v", err)
	}
	genDir := filepath.Join(domainDir, "docs", "gen")
	got := listFilesUnder(t, genDir)
	want := []string{"REQUIREMENTS.md", "UNENFORCED.md"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("consumer docs/gen = %v, want %v", got, want)
	}
	if fw := listFilesUnder(t, filepath.Join(projectRoot, "framework")); len(fw) != 0 {
		t.Errorf("consumer profile must not write framework/, found %v", fw)
	}
	if len(written) != len(want) {
		t.Errorf("len(written)=%d, want %d (files on disk)", len(written), len(want))
	}
}

// TestGenSpec_ConsumerCodeAuthorityFullDropsRequirementsMD: a code-authority +
// discipline:"full" domain keeps its requirement text in spec/requirements.go
// and SPEC.md, so REQUIREMENTS.md is not written.
func TestGenSpec_ConsumerCodeAuthorityFullDropsRequirementsMD(t *testing.T) {
	t.Parallel()

	_, domainDir := initDomainUnderRoot(t, "test-code", "2026-07-13")
	seedMinimalRequirement(t, domainDir, "test-code", "2026-07-13")
	manifest := "{\"self_hosting\": false, \"gen_profile\": \"consumer\", \"parent\": null, \"discipline\": \"full\", \"requirements_authority\": \"code\"}\n"
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if _, _, err := genSpec(domainDir, "", "2026-07-13", "", false); err != nil {
		t.Fatalf("genSpec: %v", err)
	}
	if _, err := os.Stat(filepath.Join(domainDir, "docs", "gen", "REQUIREMENTS.md")); !os.IsNotExist(err) {
		t.Errorf("code-authority + discipline full must not write REQUIREMENTS.md, stat err=%v", err)
	}
}

// TestGenSpec_ConsumerLeavesFrameworkAloneWhenAnotherDomainIsFull: the shared
// framework/ files belong to any full-profile domain of the project, so a
// consumer run must not delete them.
func TestGenSpec_ConsumerLeavesFrameworkAloneWhenAnotherDomainIsFull(t *testing.T) {
	t.Parallel()

	projectRoot, domainDir := initDomainUnderRoot(t, "test-external", "2026-07-13")
	seedMinimalRequirement(t, domainDir, "test-external", "2026-07-13")
	other := filepath.Join(projectRoot, "domains", "other-full")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No gen_profile => resolves to the full profile.
	if err := os.WriteFile(filepath.Join(other, "manifest.json"), []byte("{\"self_hosting\": false}\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if _, _, err := genSpec(domainDir, "", "2026-07-13", "full", false); err != nil {
		t.Fatalf("genSpec full: %v", err)
	}
	before := listFilesUnder(t, filepath.Join(projectRoot, "framework"))
	if len(before) == 0 {
		t.Fatal("precondition: full run must write framework/ files")
	}
	if _, _, err := genSpec(domainDir, "", "2026-07-13", "consumer", false); err != nil {
		t.Fatalf("genSpec consumer: %v", err)
	}
	after := listFilesUnder(t, filepath.Join(projectRoot, "framework"))
	if strings.Join(before, ",") != strings.Join(after, ",") {
		t.Errorf("consumer run deleted framework/ files owned by a full-profile domain: before=%d after=%d", len(before), len(after))
	}
}

// TestGenSpec_ConsumerVsFullDelta: the full profile is a strict superset of
// the consumer file set for the same graph.
func TestGenSpec_ConsumerVsFullDelta(t *testing.T) {
	t.Parallel()

	rootC, dirC := initDomainUnderRoot(t, "ext-consumer", "2026-07-13")
	seedMinimalRequirement(t, dirC, "ext-consumer", "2026-07-13")
	if _, _, err := genSpec(dirC, "", "2026-07-13", "consumer", false); err != nil {
		t.Fatalf("genSpec consumer: %v", err)
	}
	rootF, dirF := initDomainUnderRoot(t, "ext-full", "2026-07-13")
	seedMinimalRequirement(t, dirF, "ext-full", "2026-07-13")
	if _, _, err := genSpec(dirF, "", "2026-07-13", "full", false); err != nil {
		t.Fatalf("genSpec full: %v", err)
	}
	consumerGen := listFilesUnder(t, filepath.Join(dirC, "docs", "gen"))
	fullGen := map[string]bool{}
	for _, f := range listFilesUnder(t, filepath.Join(dirF, "docs", "gen")) {
		fullGen[f] = true
	}
	for _, f := range consumerGen {
		if !fullGen[f] {
			t.Errorf("consumer wrote %s which the full profile does not", f)
		}
	}
	cTotal := len(consumerGen) + len(listFilesUnder(t, filepath.Join(rootC, "framework")))
	fTotal := len(fullGen) + len(listFilesUnder(t, filepath.Join(rootF, "framework")))
	wantDelta := fTotal - cTotal
	minDelta := thinkingDocsCount() + len(listFilesUnder(t, filepath.Join(rootF, "framework", "tools"))) + 1 // + GLOSSARY.md
	if wantDelta < minDelta {
		t.Errorf("full-consumer delta=%d, want at least %d (thinking + tools + glossary)", wantDelta, minDelta)
	}
}

// TestGenSpec_ConsumerRequirementsMDDoesNotReferenceFrameworkTools: the
// consumer REQUIREMENTS.md points at `hotam -h`, never at framework/tools/
// (which the consumer profile no longer writes).
func TestGenSpec_ConsumerRequirementsMDDoesNotReferenceFrameworkTools(t *testing.T) {
	t.Parallel()

	_, domainDir := initDomainUnderRoot(t, "test-linkcheck-requirements", "2026-07-14")
	seedMinimalRequirement(t, domainDir, "test-linkcheck-requirements", "2026-07-14")
	if _, _, err := genSpec(domainDir, "", "2026-07-14", "consumer", false); err != nil {
		t.Fatalf("genSpec consumer: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(domainDir, "docs", "gen", "REQUIREMENTS.md"))
	if err != nil {
		t.Fatalf("read REQUIREMENTS.md: %v", err)
	}
	text := string(content)
	if strings.Contains(text, "framework/tools") {
		t.Errorf("consumer REQUIREMENTS.md must not reference framework/tools, got:\n%s", text)
	}
	if !strings.Contains(text, "`hotam -h`") {
		t.Errorf("consumer REQUIREMENTS.md must point at `hotam -h`, got:\n%s", text)
	}
}

package docbundle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutNamesRemainStableAcrossLocaleTransitions(t *testing.T) {
	legacy, err := NewLayout(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := legacy.SpecIndexPath(""); err != nil || got != "docs/gen/SPEC.md" {
		t.Fatalf("legacy SPEC path = %q, %v", got, err)
	}
	if got, err := legacy.SpecShardPath("", "pkg/model.md"); err != nil || got != "docs/gen/spec/pkg/model.md" {
		t.Fatalf("legacy shard path = %q, %v", got, err)
	}

	single, err := NewLayout([]string{"ru"}, "ru")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := single.DocumentPath("docs/gen/REQUIREMENTS.md", "ru"); err != nil || got != "docs/gen/REQUIREMENTS.md" {
		t.Fatalf("explicit single-language path = %q, %v", got, err)
	}
	if got, err := single.CrystalPath("ru"); err != nil || got != "CLAUDE.md" {
		t.Fatalf("single-language crystal path = %q, %v", got, err)
	}

	multi, err := NewLayout([]string{"ru", "en"}, "ru")
	if err != nil {
		t.Fatal(err)
	}
	for language, wantIndex := range map[string]string{"ru": "docs/gen/SPEC.ru.md", "en": "docs/gen/SPEC.en.md"} {
		if got, err := multi.SpecIndexPath(language); err != nil || got != wantIndex {
			t.Errorf("SPEC index for %s = %q, %v; want %q", language, got, err, wantIndex)
		}
	}
	if got, err := multi.SpecShardPath("en", "pkg/model.md"); err != nil || got != "docs/gen/spec/en/pkg/model.md" {
		t.Fatalf("localized shard path = %q, %v", got, err)
	}
	if got, err := multi.CrystalPath("ru"); err != nil || got != "CLAUDE.md" {
		t.Fatalf("default-language crystal path = %q, %v", got, err)
	}
	if got, err := multi.CrystalPath("en"); err != nil || got != "CLAUDE.en.md" {
		t.Fatalf("additional crystal path = %q, %v", got, err)
	}
}

func TestLocaleRemovalCleanupInventoryPreservesAuthoredFiles(t *testing.T) {
	root := t.TempDir()
	genDir := filepath.Join(root, "docs", "gen")
	for relative, text := range map[string]string{
		"SPEC.ru.md":         "old Russian index",
		"SPEC.en.md":         "current English index",
		"spec/ru/model.md":   "old Russian shard",
		"spec/en/model.md":   "current English shard",
		"SPEC.es.md":         "authored Spanish note",
		"hand-placed.md":     "authored document",
		"thinking/legacy.md": "old generated thinking doc",
	} {
		path := filepath.Join(genDir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	candidates, err := DomainCandidates(genDir)
	if err != nil {
		t.Fatal(err)
	}
	var present []string
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			present = append(present, path)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	current := []string{
		filepath.Join(genDir, "SPEC.md"),
		filepath.Join(genDir, "spec", "model.md"),
		filepath.Join(genDir, "REQUIREMENTS.md"),
	}
	stale := StalePaths(present, current, nil)
	want := []string{
		filepath.Join(genDir, "SPEC.en.md"),
		filepath.Join(genDir, "SPEC.ru.md"),
		filepath.Join(genDir, "spec", "en", "model.md"),
		filepath.Join(genDir, "spec", "ru", "model.md"),
		filepath.Join(genDir, "thinking", "legacy.md"),
	}
	if len(stale) != len(want) {
		t.Fatalf("stale locale/profile outputs = %v; want %v", stale, want)
	}
	for i := range want {
		if stale[i] != want[i] {
			t.Fatalf("stale locale/profile outputs = %v; want %v", stale, want)
		}
	}
	for _, relative := range []string{"docs/gen/SPEC.es.md", "docs/gen/hand-placed.md"} {
		if OwnedDomainPath(relative) {
			t.Errorf("authored path %q classified as generator-owned", relative)
		}
	}
}

func TestLayoutRejectsUnsafeOrAmbiguousOutputLocales(t *testing.T) {
	for _, languages := range [][]string{{"en", "../ru"}, {"en", "EN"}, {"en", "ru"}} {
		defaultLanguage := "en"
		if languages[1] == "ru" {
			defaultLanguage = "fr"
		}
		if _, err := NewLayout(languages, defaultLanguage); err == nil {
			t.Errorf("NewLayout(%q, %q) accepted an unsafe, case-ambiguous, or non-member default", languages, defaultLanguage)
		}
	}
}

func TestResolveOutputPathRestrictsLocalizedRendererInventory(t *testing.T) {
	repoRoot := t.TempDir()
	domainDir := filepath.Join(repoRoot, "domains", "demo")
	valid := filepath.Join(domainDir, "docs", "gen", "REQUIREMENTS.ru.md")
	got, err := ResolveOutputPath(repoRoot, domainDir, "docs/gen/REQUIREMENTS.ru.md")
	if err != nil || got != valid {
		t.Fatalf("owned localized document resolved to %q, %v; want %q", got, err, valid)
	}
	for _, relative := range []string{
		"docs/gen/NOTES.md",
		"docs/gen/SPEC.ru.md",
		"docs/gen/evidence.json",
		"../other/docs/gen/REQUIREMENTS.ru.md",
		"domains/other/docs/gen/REQUIREMENTS.ru.md",
	} {
		if got, err := ResolveOutputPath(repoRoot, domainDir, relative); err == nil {
			t.Errorf("unowned or separately-published path %q resolved to %q", relative, got)
		}
	}
}

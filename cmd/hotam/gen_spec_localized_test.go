package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/generator"
)

func TestCleanupStaleLocalizedCrystalsPreservesAuthoredFilesAndNotes(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "CLAUDE.en.md")
	obsoleteWithNotes := filepath.Join(dir, "CLAUDE.ru.md")
	obsoleteWithoutNotes := filepath.Join(dir, "CLAUDE.zh.md")
	authoredUnknown := filepath.Join(dir, "CLAUDE.es.md")
	generated := "# Generated crystal\n" + generator.DurableNotesMarkerLine + "\n"
	withNotes := generated + "operator-authored durable note\n"
	for path, content := range map[string]string{
		current:              generated + "current locale note\n",
		obsoleteWithNotes:    withNotes,
		obsoleteWithoutNotes: generated,
		authoredUnknown:      "hand-authored Spanish document\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := cleanupStaleLocalizedCrystals(dir, []string{current})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != obsoleteWithoutNotes {
		t.Fatalf("removed localized crystals = %v; want only %q", removed, obsoleteWithoutNotes)
	}
	if got, err := os.ReadFile(current); err != nil || string(got) != generated+"current locale note\n" {
		t.Fatalf("current locale output changed: %q, %v", got, err)
	}
	if got, err := os.ReadFile(obsoleteWithNotes); err != nil || string(got) != generator.DurableNotesMarkerLine+"\noperator-authored durable note\n" {
		t.Fatalf("obsolete generated portion was not removed while preserving notes: %q, %v", got, err)
	}
	if got, err := os.ReadFile(authoredUnknown); err != nil || string(got) != "hand-authored Spanish document\n" {
		t.Fatalf("unowned authored locale document changed: %q, %v", got, err)
	}
}

func TestValidateLocalizedCrystalTargetRefusesAuthoredCollision(t *testing.T) {
	dir := t.TempDir()
	authored := filepath.Join(dir, "CLAUDE.ru.md")
	content := []byte("operator-authored Russian boot document\n")
	if err := os.WriteFile(authored, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateLocalizedCrystalTarget(authored); err == nil {
		t.Fatal("localized output collision with an authored file was accepted")
	}
	if got, err := os.ReadFile(authored); err != nil || string(got) != string(content) {
		t.Fatalf("refused locale collision changed authored file: %q, %v", got, err)
	}

	generated := filepath.Join(dir, "CLAUDE.en.md")
	template := "# generated crystal\n" + generator.DurableNotesMarkerLine + "\n"
	if err := os.WriteFile(generated, []byte(template), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateLocalizedCrystalTarget(generated); err != nil {
		t.Fatalf("recognized generator-owned crystal was refused: %v", err)
	}
}

func TestValidateLocalizedRendererTargetRefusesAuthoredCollision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docs", "gen", "REQUIREMENTS.ru.md")
	content := []byte("operator-authored Russian requirements\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateLocalizedRendererTarget(path, "ru"); err == nil {
		t.Fatal("localized renderer collision with an authored document was accepted")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(content) {
		t.Fatalf("refused localized document collision changed authored file: %q, %v", got, err)
	}
}

func TestLocaleCleanupPreservesAuthoredBaseDocumentPaths(t *testing.T) {
	layout, err := docbundle.NewLayout([]string{"en", "ru"}, "en")
	if err != nil {
		t.Fatal(err)
	}
	genDir := filepath.Join(t.TempDir(), "docs", "gen")
	domainDocument := filepath.Join(genDir, "REQUIREMENTS.md")
	domainContent := []byte("operator-authored base requirements document\n")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(domainDocument, domainContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cleanupStaleGenFiles(genDir, nil, nil, layout); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(domainDocument); err != nil || string(got) != string(domainContent) {
		t.Fatalf("domain cleanup changed authored base document: %q, %v", got, err)
	}

	frameworkDir := filepath.Join(t.TempDir(), "framework")
	projectDocument := filepath.Join(frameworkDir, "GLOSSARY.md")
	projectContent := []byte("operator-authored shared glossary\n")
	if err := os.MkdirAll(frameworkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectDocument, projectContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cleanupStaleProjectFrameworkFiles(frameworkDir, nil, layout); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(projectDocument); err != nil || string(got) != string(projectContent) {
		t.Fatalf("project cleanup changed authored base document: %q, %v", got, err)
	}
}

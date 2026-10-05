package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCmdGenSpecUnsupportedLocaleRefusesBeforePublishing(t *testing.T) {
	_, domainDir := copyNonSelfHostingDomainUnderRoot(t)
	manifestPath := filepath.Join(domainDir, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["languages"] = []string{"es"}
	manifest["default_language"] = "es"
	manifestBytes, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	genDir := filepath.Join(domainDir, "docs", "gen")
	before := map[string]string{
		"REQUIREMENTS.md":        "pre-generation authored baseline",
		"SPEC.ru.md":             "pre-generation localized baseline",
		"NOTES-not-generated.md": "authored note",
	}
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range before {
		if err := os.WriteFile(filepath.Join(genDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := cmdGenSpec([]string{"--domain", domainDir, "--spec"}); err == nil {
		t.Fatal("gen-spec accepted a locale without a complete authored service catalog")
	}
	for name, want := range before {
		got, err := os.ReadFile(filepath.Join(genDir, name))
		if err != nil || string(got) != want {
			t.Errorf("refused gen-spec publish changed %s: got %q, %v; want %q", name, got, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(genDir, "graph.json")); !os.IsNotExist(err) {
		t.Errorf("refused gen-spec publish created graph.json: %v", err)
	}
}

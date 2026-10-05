package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCmdEvidenceUnsupportedLocaleRefusesBeforePublishing(t *testing.T) {
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
		"EVIDENCE.md":    "authored evidence baseline",
		"FINDINGS.md":    "authored findings baseline",
		"evidence.json":  "authored raw baseline",
		"EVIDENCE.ru.md": "prior localized output",
	}
	for name, content := range before {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(genDir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(genDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := cmdEvidence([]string{"--domain", domainDir, "--write"}); err == nil {
		t.Fatal("evidence accepted a locale without a complete authored service catalog")
	}
	for name, want := range before {
		got, err := os.ReadFile(filepath.Join(genDir, name))
		if err != nil || string(got) != want {
			t.Errorf("refused evidence publish changed %s: got %q, %v; want %q", name, got, err, want)
		}
	}
}

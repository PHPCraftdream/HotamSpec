package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestObsoleteLanguageOutputsFlagsRetiredLocaleButNotAuthoredOrSharedData(t *testing.T) {
	genDir := filepath.Join(t.TempDir(), "docs", "gen")
	sourceGraph := &ontology.Graph{
		SelfExecutingAtoms: true,
		Requirements: []ontology.Requirement{
			{ID: "REQ-MODEL", ImplementedBy: []string{"pkg/model/model.go:Model"}},
		},
	}
	generated, err := gate.BuildSpecDocumentsFromRows(sourceGraph, map[string]gate.SpecRow{})
	if err != nil {
		t.Fatal(err)
	}
	for relative, content := range map[string]string{
		"SPEC.en.md":           "current English SPEC",
		"SPEC.ru.md":           generated["SPEC.md"],
		"spec/ru/pkg/model.md": generated["spec/pkg/model.md"],
		"REQUIREMENTS.md":      "operator-authored base requirements document",
		"NOTES.md":             "authored note",
		"graph.json":           "common graph snapshot",
		"evidence.json":        "one common evidence snapshot",
	} {
		path := filepath.Join(genDir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	layout, err := docbundle.NewLayout([]string{"en", "ru"}, "en")
	if err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(genDir, "SPEC.en.md")
	violations := obsoleteLanguageOutputs(genDir, map[string]string{current: "current English SPEC"}, layout)
	if len(violations) != 2 {
		t.Fatalf("obsolete language outputs = %v; want retired index and shard only", violations)
	}
	want := []string{filepath.Join(genDir, "SPEC.ru.md"), filepath.Join(genDir, "spec", "ru", "pkg", "model.md")}
	for i := range want {
		if violations[i].ID != want[i] {
			t.Fatalf("obsolete output IDs = %v; want %v", []string{violations[0].ID, violations[1].ID}, want)
		}
	}
	for _, violation := range violations {
		if violation.Check != "check_language_outputs_current" {
			t.Errorf("obsolete output reported by %q, want check_language_outputs_current", violation.Check)
		}
	}
}

func TestLanguageFreshnessDoesNotTriggerFromLegacyOptIns(t *testing.T) {
	legacy := &ontology.Graph{SelfExecutingAtoms: true, Discipline: loader.DisciplineFull}
	if got := checkLanguageOutputsCurrentReal(legacy, nil, "2026-10-04"); len(got) != 0 {
		t.Fatalf("legacy atom/discipline opt-ins activated language freshness: %v", got)
	}
}

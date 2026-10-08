package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/conformance"
	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/evidence"
	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestWriteEvidenceBundleRemovesObsoleteLocaleViewsOnly(t *testing.T) {
	domainDir := t.TempDir()
	genDir := filepath.Join(domainDir, "docs", "gen")
	view := &ontology.Graph{Languages: []string{"en", "ru"}, DefaultLanguage: "en", RenderLanguage: "ru"}
	report := evidence.Report{SchemaVersion: 1}
	oldEvidence, err := generator.BuildEvidenceLocalized(view, report, "ru")
	if err != nil {
		t.Fatal(err)
	}
	oldFindings, err := generator.BuildFindingsLocalized(view, report, "ru")
	if err != nil {
		t.Fatal(err)
	}
	for relative, text := range map[string]string{
		"EVIDENCE.ru.md": oldEvidence,
		"FINDINGS.ru.md": oldFindings,
		"EVIDENCE.es.md": "authored Spanish notes",
		"NOTES.md":       "authored notes",
	} {
		path := filepath.Join(genDir, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	machine := []byte("{\"schema_version\":1}\n")
	documents := map[string]string{
		"docs/gen/EVIDENCE.md": "current evidence view",
		"docs/gen/FINDINGS.md": "current findings view",
	}
	if err := writeEvidenceBundle(domainDir, documents, machine); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"EVIDENCE.ru.md", "FINDINGS.ru.md"} {
		if _, err := os.Stat(filepath.Join(genDir, relative)); !os.IsNotExist(err) {
			t.Errorf("obsolete generated report %s remains: %v", relative, err)
		}
	}
	for _, relative := range []string{"EVIDENCE.md", "FINDINGS.md", "evidence.json"} {
		info, err := os.Stat(filepath.Join(genDir, relative))
		if err != nil || !info.Mode().IsRegular() {
			t.Errorf("current report bundle member %s is absent or not a regular file: %v", relative, err)
		}
	}
	if _, err := os.Stat(filepath.Join(genDir, "evidence.ru.json")); !os.IsNotExist(err) {
		t.Errorf("localized JSON sidecar exists; there must be exactly one shared evidence.json: %v", err)
	}
	for relative, want := range map[string]string{
		"EVIDENCE.es.md": "authored Spanish notes",
		"NOTES.md":       "authored notes",
	} {
		got, err := os.ReadFile(filepath.Join(genDir, relative))
		if err != nil || string(got) != want {
			t.Errorf("authored %s changed during report cleanup: %q, %v", relative, got, err)
		}
	}
}

func TestReportHasFailuresIgnoresQualificationsButKeepsRawComparisons(t *testing.T) {
	qualification := evidence.Finding{Disposition: evidence.FindingDispositionQualification}
	report := evidence.Report{Findings: []evidence.Finding{qualification}}
	if reportHasFailures(report) {
		t.Fatal("qualification-only conformance review became a command failure")
	}

	report.Requirements = []evidence.RequirementResult{{
		Tests: []evidence.TestEvidence{{
			Artifacts: []evidence.ArtifactEvidence{{
				Observations: []evidence.Observation{{Passed: false}},
			}},
		}},
	}}
	if !reportHasFailures(report) {
		t.Fatal("a raw failed comparison was suppressed by a qualification finding")
	}
}

func TestCleanupStaleEvidenceLocaleViewsUsesGeneratedOwnership(t *testing.T) {
	genDir := filepath.Join(t.TempDir(), "docs", "gen")
	view := &ontology.Graph{Languages: []string{"en", "ru"}, DefaultLanguage: "en"}
	report := evidence.Report{SchemaVersion: 1}
	generated, err := generator.BuildEvidenceLocalized(view, report, "ru")
	if err != nil {
		t.Fatal(err)
	}
	for relative, content := range map[string]string{
		"EVIDENCE.ru.md": generated,
		"FINDINGS.ru.md": "operator-authored findings notes\n",
	} {
		path := filepath.Join(genDir, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	layout, err := docbundle.NewLayout([]string{"en"}, "en")
	if err != nil {
		t.Fatal(err)
	}
	removed, err := cleanupStaleEvidenceLocaleViews(genDir, layout, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != filepath.Join(genDir, "EVIDENCE.ru.md") {
		t.Fatalf("stale report cleanup removed %v; want only generated EVIDENCE.ru.md", removed)
	}
	if got, err := os.ReadFile(filepath.Join(genDir, "FINDINGS.ru.md")); err != nil || string(got) != "operator-authored findings notes\n" {
		t.Fatalf("authored stale-locale report changed: %q, %v", got, err)
	}
}

func TestWriteEvidenceBundleRefusesAuthoredLocalizedReport(t *testing.T) {
	domainDir := t.TempDir()
	authoredPath := filepath.Join(domainDir, "docs", "gen", "EVIDENCE.ru.md")
	content := []byte("operator-authored Russian report\n")
	if err := os.MkdirAll(filepath.Dir(authoredPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authoredPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	err := writeEvidenceBundle(domainDir, map[string]string{
		"docs/gen/EVIDENCE.ru.md": "generated Russian report",
	}, []byte("{\"schema_version\":1}\n"))
	if err == nil {
		t.Fatal("evidence publisher accepted an authored localized report collision")
	}
	if got, err := os.ReadFile(authoredPath); err != nil || string(got) != string(content) {
		t.Fatalf("refused report collision changed authored file: %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(domainDir, "docs", "gen", "evidence.json")); !os.IsNotExist(err) {
		t.Fatalf("report bundle was partially written despite collision: %v", err)
	}
}

func TestWriteEvidenceBundleUpdatesValidCaseJournalAndProtectsDamagedOrAuthoredOutput(t *testing.T) {
	domainDir := t.TempDir()
	graph := &ontology.Graph{Languages: []string{"en"}, DefaultLanguage: "en"}
	report := evidence.Report{SchemaVersion: 1}
	report.Conformance.Cases = []conformance.CaseAssessment{{ID: "case", AtomID: "atom", Status: "unverified"}}
	documents, err := generator.BuildEvidenceBundleLocalized(graph, report, "en")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := docbundle.NewLayout(graph.Languages, graph.DefaultLanguage)
	if err != nil {
		t.Fatal(err)
	}
	relative, err := layout.EvidenceCasePath("en", "atom", "case")
	if err != nil {
		t.Fatal(err)
	}
	machine := []byte("{\"schema_version\":1}\n")
	if err := writeEvidenceBundle(domainDir, documents, machine); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(domainDir, filepath.FromSlash(relative))
	original := documents[relative]
	report.Conformance.Cases[0].Qualification = "unproved"
	updated, err := generator.BuildEvidenceBundleLocalized(graph, report, "en")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeEvidenceBundle(domainDir, updated, machine); err != nil {
		t.Fatalf("valid generated case journal could not be updated: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != updated[relative] || string(got) == original {
		t.Fatalf("new captured qualification was not published exactly: %v", err)
	}
	for _, protected := range []string{updated[relative] + "damaged\n", "operator-authored case review\n"} {
		if err := os.WriteFile(path, []byte(protected), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := writeEvidenceBundle(domainDir, documents, machine); err == nil {
			t.Fatal("damaged or authored case output lost overwrite protection")
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != protected {
			t.Fatalf("refused journal collision changed protected bytes: %v", err)
		}
	}
}

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func writeDueTestDomain(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-overdue-1", Owner: "sa", Status: ontology.StatusSETTLED, ReviewAfter: "2020-01-01"},
			{ID: "R-overdue-2", Owner: "sb", Status: ontology.StatusSETTLED, ReviewAfter: "2026-07-01"},
			{ID: "R-never-1", Owner: "sa", Status: ontology.StatusSETTLED},
			{ID: "R-never-2", Owner: "sb", Status: ontology.StatusSETTLED},
			{ID: "R-fresh", Owner: "sa", Status: ontology.StatusSETTLED, ReviewAfter: "2030-01-01"},
			{ID: "R-draft", Owner: "sa", Status: ontology.StatusDRAFT},
		},
	}
	path := filepath.Join(dir, "graph.json")
	if err := loader.WriteGraph(path, g); err != nil {
		t.Fatalf("write graph: %v", err)
	}
	return dir
}

func TestBuildDueReport_OverdueSortedOldestFirst(t *testing.T) {
	t.Parallel()
	domainDir := writeDueTestDomain(t)
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	report := buildDueReport(g, "2026-07-12")

	if report.OverdueCount != 2 {
		t.Fatalf("OverdueCount = %d, want 2", report.OverdueCount)
	}
	if len(report.Overdue) != 2 {
		t.Fatalf("len(Overdue) = %d, want 2", len(report.Overdue))
	}
	if report.Overdue[0].ID != "R-overdue-1" {
		t.Errorf("Overdue[0].ID = %q, want R-overdue-1 (oldest review_after first)", report.Overdue[0].ID)
	}
	if report.Overdue[0].OverdueDays <= report.Overdue[1].OverdueDays {
		t.Errorf("Overdue not sorted oldest-first: %+v", report.Overdue)
	}
}

func TestBuildDueReport_NeverReviewedSummaryAndSample(t *testing.T) {
	t.Parallel()
	domainDir := writeDueTestDomain(t)
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	report := buildDueReport(g, "2026-07-12")

	if report.NeverReviewedCount != 2 {
		t.Fatalf("NeverReviewedCount = %d, want 2", report.NeverReviewedCount)
	}
	if len(report.NeverReviewedSample) != 2 {
		t.Errorf("len(NeverReviewedSample) = %d, want 2 (below the top-N cap)", len(report.NeverReviewedSample))
	}
}

func TestBuildDueReport_ExcludesDraftAndFresh(t *testing.T) {
	t.Parallel()
	domainDir := writeDueTestDomain(t)
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	report := buildDueReport(g, "2026-07-12")

	for _, e := range report.Overdue {
		if e.ID == "R-draft" || e.ID == "R-fresh" {
			t.Errorf("DRAFT/FRESH requirement %q should not appear in OVERDUE", e.ID)
		}
	}
	for _, e := range report.NeverReviewedSample {
		if e.ID == "R-draft" {
			t.Errorf("DRAFT requirement should not appear in NEVER-REVIEWED sample")
		}
	}
}

func TestFormatDueReport_NeverReviewedFoldedNotOneLinePerRequirement(t *testing.T) {
	t.Parallel()
	domainDir := writeDueTestDomain(t)
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	report := buildDueReport(g, "2026-07-12")
	text := formatDueReport(report)
	if !strings.Contains(text, "NEVER-REVIEWED: 2 requirement(s)") {
		t.Errorf("expected summary count line, got:\n%s", text)
	}
}

// TestBuildDueReport_JSONEmptyArraysNotNull is the byte-level regression pin
// for the P2 "empty arrays instead of null in JSON output" fix: on a graph
// with zero OVERDUE and zero NEVER-REVIEWED SETTLED requirements, the
// marshaled `hotam due --json` payload must literally contain
// `"overdue":[]` and `"never_reviewed_sample":[]`, never `"overdue":null` /
// `"never_reviewed_sample":null`. A Go-level len()==0 or !=nil check would
// not prove what encoding/json actually emits, so this asserts the raw
// marshaled bytes.
func TestBuildDueReport_JSONEmptyArraysNotNull(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			// SETTLED, freshly reviewed, review window far in the future:
			// neither OVERDUE nor NEVER-REVIEWED.
			{
				ID: "R-fresh", Owner: "sa", Status: ontology.StatusSETTLED,
				LastReviewedAt: "2026-07-01", ReviewAfter: "2030-01-01",
			},
		},
	}
	path := filepath.Join(dir, "graph.json")
	if err := loader.WriteGraph(path, g); err != nil {
		t.Fatalf("write graph: %v", err)
	}
	loaded, err := loadDomainGraph(dir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}

	report := buildDueReport(loaded, "2026-07-12")
	if report.OverdueCount != 0 || report.NeverReviewedCount != 0 {
		t.Fatalf("fixture is not empty as expected: overdue=%d never_reviewed=%d", report.OverdueCount, report.NeverReviewedCount)
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)

	if !strings.Contains(raw, `"overdue":[]`) {
		t.Errorf("expected literal `\"overdue\":[]` in marshaled JSON, got:\n%s", raw)
	}
	if strings.Contains(raw, `"overdue":null`) {
		t.Errorf("marshaled JSON must never contain `\"overdue\":null`, got:\n%s", raw)
	}
	if !strings.Contains(raw, `"never_reviewed_sample":[]`) {
		t.Errorf("expected literal `\"never_reviewed_sample\":[]` in marshaled JSON, got:\n%s", raw)
	}
	if strings.Contains(raw, `"never_reviewed_sample":null`) {
		t.Errorf("marshaled JSON must never contain `\"never_reviewed_sample\":null`, got:\n%s", raw)
	}
}

func TestCmdDue_SmokeNoPanicOnRealDomain(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	err := cmdDue([]string{"--domain", domainDir, "--today", "2026-07-12"})
	if err != nil {
		t.Fatalf("cmdDue: %v", err)
	}
}

func TestCmdDue_JSONFlagNoPanic(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	err := cmdDue([]string{"--domain", domainDir, "--today", "2026-07-12", "--json"})
	if err != nil {
		t.Fatalf("cmdDue --json: %v", err)
	}
}

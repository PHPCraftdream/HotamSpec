package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestLocalizedLogicalFreshnessSurvivesMidnightWhileReviewAlertsAdvance(t *testing.T) {
	projectRoot, domainDir := initDomainUnderRoot(t, "date-independent", "2026-10-05")
	seedMinimalRequirement(t, domainDir, "date-independent", "2026-10-01")
	manifest := `{"self_hosting":false,"gen_profile":"consumer","parent":null,"languages":["en","ru"],"default_language":"en"}`
	graphPath := filepath.Join(domainDir, "graph.json")
	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatal(err)
	}
	for index := range g.Requirements {
		r := &g.Requirements[index]
		r.ClaimTexts = ontology.LocalizedText{"en": r.Claim, "ru": r.Claim}
		r.LastReviewedAt = "2026-10-01"
		r.ReviewAfter = "2026-10-05"
	}
	if err := loader.WriteGraph(graphPath, g); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	seedFingerprintSource(t, projectRoot)
	for relative, contents := range map[string]string{
		"spec/go.mod":         "module freshness-spec\n\ngo 1.22\n",
		"spec/model/model.go": "package model\ntype Model struct{}\n// >>>>> lang=en\n// Source value remains stable.\n// >>>>> lang=ru\n// Значение источника остается постоянным.\nfunc (Model) Value() int { return 1 }\n",
	} {
		path := filepath.Join(domainDir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := genSpec(domainDir, "", "2026-10-05", "consumer", false); err != nil {
		t.Fatal(err)
	}
	g, err = loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatal(err)
	}
	publication := invariants.PriorToPostProcessViolationsForPublication(g, true)
	for _, date := range []string{"2026-10-05", "2026-10-06", "2027-10-05"} {
		if violations := checkLanguageOutputsCurrentReal(g, publication, date); len(violations) != 0 {
			t.Fatalf("unchanged logical output became stale on %s: %v", date, violations)
		}
	}
	for _, signal := range diagnose.FreshnessSignals(g, "2026-10-05") {
		if signal.Check == "freshness_overdue" {
			t.Fatal("review was prematurely overdue on its boundary date")
		}
	}
	alerted := false
	for _, signal := range diagnose.FreshnessSignals(g, "2026-10-06") {
		alerted = alerted || signal.Check == "freshness_overdue"
	}
	if !alerted {
		t.Fatal("publication date suppressed the current calendar review alert")
	}
	g.Requirements[0].Claim += " changed"
	g.Requirements[0].ClaimTexts["en"] = g.Requirements[0].Claim
	g.Requirements[0].ClaimTexts["ru"] = g.Requirements[0].Claim
	if violations := checkLanguageOutputsCurrentReal(g, publication, "2026-10-06"); len(violations) == 0 {
		t.Fatal("content mutation was hidden by date-independent freshness")
	}
	g, err = loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest = `{"self_hosting":false,"gen_profile":"full","parent":null,"languages":["en","ru"],"default_language":"en"}`
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	if violations := checkLanguageOutputsCurrentReal(g, publication, "2026-10-06"); len(violations) == 0 {
		t.Fatal("output profile mutation was hidden by the publication date")
	}
}

package generator

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/evidence"
	"github.com/PHPCraftdream/HotamSpec/internal/localization"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestLocalizedEvidenceViewsShareTypedDataAndCaseIdentity(t *testing.T) {
	g := &ontology.Graph{}
	g.Languages = []string{"en", "ru"}
	g.DefaultLanguage = "ru"
	g.RenderLanguage = "en"
	value := &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: "/wA="}
	report := evidence.Report{SchemaVersion: 2}
	report.Requirements = []evidence.RequirementResult{{
		ID: "R-result", Claim: "Универсальная формулировка",
		ClaimTexts:     ontology.LocalizedText{"en": "The result is retained exactly.", "ru": "Результат сохраняется точно."},
		AtomKind:       "rule",
		Cases:          []ontology.CaseDefinition{{ID: "case-raw", AtomIDs: []string{"R-result"}, Profile: "raw-v1", Target: "direct-sdk", Expected: value}},
		CoverageStatus: ontology.CoverageDiscrepancy,
	}}
	report.Findings = []evidence.Finding{{
		ID: "F-case-stable", RequirementID: "R-result", CaseID: "case-raw",
		Claim:      report.Requirements[0].Claim,
		ClaimTexts: ontology.LocalizedText{"en": "The result is retained exactly.", "ru": "Результат сохраняется точно."},
		Kind:       evidence.FindingDiscrepancy, ReviewStatus: evidence.ReviewUnreviewed,
		Target: "direct-sdk", Producer: "sdk-v2",
		Observations: []evidence.Observation{{Name: "bytes", Actual: "base64(/wA=)", Expected: "base64(/wA=)", Passed: false, RawActual: value, RawExpected: value}},
	}}
	before, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	enDocuments, err := BuildEvidenceBundleLocalized(g, report, "en")
	if err != nil {
		t.Fatal(err)
	}
	ruDocuments, err := BuildEvidenceBundleLocalized(g, report, "ru")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := reportLayout(g)
	if err != nil {
		t.Fatal(err)
	}
	enPath, err := layout.EvidenceRequirementPath("en", "R-result")
	if err != nil {
		t.Fatal(err)
	}
	ruPath, err := layout.EvidenceRequirementPath("ru", "R-result")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(enDocuments[enPath], "The result is retained exactly.") || !strings.Contains(ruDocuments[ruPath], "Результат сохраняется точно.") {
		t.Fatal("requirement detail views did not select their authored claim language")
	}
	findings, err := BuildFindingsLocalized(g, report, "ru")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(findings, "case-raw") || !strings.Contains(findings, "direct-sdk") || !strings.Contains(findings, "sdk-v2") {
		t.Fatalf("finding view omitted locale-neutral case/target/producer identity: %q", findings)
	}
	for _, language := range []string{"en", "ru"} {
		view, err := BuildFindingsLocalized(g, report, language)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, cell := range strings.Split(view, "`") {
			if !strings.HasPrefix(cell, "{") {
				continue
			}
			var observed ontology.ObservedValue
			if json.Unmarshal([]byte(cell), &observed) == nil && ontology.EqualObservedValues(&observed, value) {
				found = true
			}
		}
		if !found {
			t.Fatalf("finding view lost the exact invalid-byte observation: %q", view)
		}
	}
	after, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) || report.Findings[0].ID != "F-case-stable" {
		t.Fatal("render selection mutated machine evidence or finding/review identity")
	}
}

func TestLocalizedEvidenceRefusesMissingCatalogBeforeReturningBytes(t *testing.T) {
	g := &ontology.Graph{}
	g.Languages = []string{"en", "ru"}
	g.DefaultLanguage = "ru"
	contents, err := BuildEvidenceLocalized(g, evidence.Report{}, "xx")
	var missing *localization.MissingTranslation
	if !errors.As(err, &missing) || contents != "" {
		t.Fatalf("missing catalog result = (%q, %v), want empty output and typed MissingTranslation", contents, err)
	}
}
func TestLocalizedOwnershipBannersRemainRecognizable(t *testing.T) {
	for _, language := range []string{"en", "ru", "zh"} {
		evidenceDoc, err := BuildEvidenceLocalized(nil, evidence.Report{}, language)
		if err != nil {
			t.Fatalf("BuildEvidenceLocalized(%s): %v", language, err)
		}
		findingsDoc, err := BuildFindingsLocalized(nil, evidence.Report{}, language)
		if err != nil {
			t.Fatalf("BuildFindingsLocalized(%s): %v", language, err)
		}
		evidenceTitle := strings.SplitN(evidenceDoc, "\n", 2)[0]
		findingsTitle := strings.SplitN(findingsDoc, "\n", 2)[0]
		if !strings.HasPrefix(evidenceTitle, "# EVIDENCE.md — ") || !strings.HasPrefix(findingsTitle, "# FINDINGS.md — ") {
			t.Fatalf("%s title lost its stable filename prefix: evidence=%q findings=%q", language, evidenceTitle, findingsTitle)
		}
		if !IsGeneratedEvidence(evidenceDoc) || IsGeneratedFindings(evidenceDoc) {
			t.Fatalf("%s evidence ownership predicate misclassified its banner", language)
		}
		if !IsGeneratedFindings(findingsDoc) || IsGeneratedEvidence(findingsDoc) {
			t.Fatalf("%s findings ownership predicate misclassified its banner", language)
		}
		if language == "en" && (evidenceTitle != "# EVIDENCE.md — Current verification evidence" ||
			findingsTitle != "# FINDINGS.md — Observed evidence requiring attention") {
			t.Fatalf("English legacy title bytes changed: evidence=%q findings=%q", evidenceTitle, findingsTitle)
		}
	}
	if IsGeneratedEvidence("# EVIDENCE.md — Current verification evidence\n\nunowned content\n") {
		t.Fatal("ownership predicate accepted a matching title without the generated provenance banner")
	}
}

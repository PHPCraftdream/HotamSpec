package generator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/conformance"
	"github.com/PHPCraftdream/HotamSpec/internal/evidence"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestEvidenceCaseCurrentnessChecksIntegrityAndStableProof(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module journal-source\n\ngo 1.22\n"), 0600); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "source.go")
	if err := os.WriteFile(sourcePath, []byte("package source\nconst Revision = 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	graph := &ontology.Graph{DomainDir: root, Requirements: []ontology.Requirement{{ID: "atom", Claim: "retain diagnostic class", Cases: []ontology.CaseDefinition{{ID: "io-case", Test: "source_test.go:TestIO", AtomIDs: []string{"atom"}, Profile: "core", Operation: "read"}}}}}
	capture := func() (evidence.Report, conformance.CaseAssessment, string) {
		t.Helper()
		// Independent real observations retain their exact paths and messages.
		missing := filepath.Join(t.TempDir(), "missing.txt")
		_, err := os.Open(missing)
		if err == nil {
			t.Fatal("missing file unexpectedly opened")
		}
		message := err.Error()
		raw := ontology.ObservedValue{Kind: "text", Text: &message}
		isMissing := errors.Is(err, os.ErrNotExist)
		expectedMissing := true
		if !isMissing {
			t.Fatalf("missing-file probe produced another error: %v", err)
		}
		actual := ontology.ObservedValue{Kind: "bool", Bool: &isMissing}
		expected := ontology.ObservedValue{Kind: "bool", Bool: &expectedMissing}
		comparison := conformance.Comparison{Name: "filesystem-error-class", Passed: isMissing, Input: &raw, Actual: &actual, Expected: &expected}
		item := conformance.CaseAssessment{ID: "io-case", AtomID: "atom", Test: "source_test.go:TestIO", Profile: "core", Operation: "read", Producer: "producer-v1", Status: "observed", Qualification: "profile-supported", ProfileFingerprint: strings.Repeat("1", 64), ProducerFingerprint: strings.Repeat("2", 64), CaseFingerprint: strings.Repeat("3", 64), Executions: []conformance.Execution{{CaseID: "io-case", AtomID: "atom", Test: "source_test.go:TestIO", Verdict: "pass", Profile: "core", Operation: "read", Producer: "producer-v1", Comparisons: []conformance.Comparison{comparison}, Components: []ontology.Component{{ID: "native", Version: "1", SHA256: strings.Repeat("4", 64), Measured: true}}}}}
		report := evidence.Report{SchemaVersion: 1}
		report.Conformance.Cases = []conformance.CaseAssessment{item}
		return report, item, "# Exact case journal\n\n" + missing + "\n" + message + "\n"
	}
	firstReport, firstCase, firstBody := capture()
	identity, err := NewEvidenceCaseIdentity(graph, firstReport)
	if err != nil {
		t.Fatal(err)
	}
	published, err := identity.Stamp(firstReport, firstCase, "en", "docs/gen/evidence/en/cases/case.md", firstBody)
	if err != nil {
		t.Fatal(err)
	}
	secondReport, secondCase, secondBody := capture()
	if firstBody == secondBody {
		t.Fatal("independent filesystem captures unexpectedly used identical temporary paths")
	}
	fresh, err := identity.Stamp(secondReport, secondCase, "en", "docs/gen/evidence/en/cases/case.md", secondBody)
	if err != nil {
		t.Fatal(err)
	}
	if err := EvidenceCasePageCurrent(published, fresh); err != nil {
		t.Fatalf("ephemeral raw diagnostic invalidated logically current journal: %v", err)
	}
	if !strings.HasSuffix(published, firstBody) || !strings.HasSuffix(fresh, secondBody) {
		t.Fatal("integrity contract rewrote exact raw diagnostic bodies")
	}
	if err := EvidenceCasePageCurrent(published+"tampered\n", fresh); err == nil {
		t.Fatal("modified published body passed its checksum")
	}
	literalBody := firstBody + "\n```text\n<!-- hotam-evidence-case: {\"literal\":true} -->\n```\n"
	literalPage, err := identity.Stamp(firstReport, firstCase, "en", "docs/gen/evidence/en/cases/case.md", literalBody)
	if err != nil {
		t.Fatalf("literal user data was interpreted as integrity metadata: %v", err)
	}
	_, retainedBody, _ := strings.Cut(literalPage, "\n")
	if retainedBody != literalBody {
		t.Fatal("marker-looking literal input bytes changed")
	}
	if err := EvidenceCasePageCurrent(literalPage, fresh); err != nil {
		t.Fatalf("literal user data invalidated the journal identity: %v", err)
	}
	_, body, _ := strings.Cut(published, "\n")
	for _, broken := range []string{body, "<!-- hotam-evidence-case: {} -->\n" + body, strings.Replace(published, `"version":1`, `"version":2`, 1), strings.Replace(published, `"body_sha256":"`, `"body_sha256":"x`, 1)} {
		if err := EvidenceCasePageCurrent(broken, fresh); err == nil {
			t.Fatal("missing/malformed/tampered marker was accepted")
		}
	}
	for _, changed := range []struct {
		name   string
		mutate func(*conformance.CaseAssessment)
	}{
		{"verdict", func(item *conformance.CaseAssessment) { item.Executions[0].Verdict = "fail" }},
		{"qualification", func(item *conformance.CaseAssessment) { item.Qualification = "profile-unsupported" }},
		{"profile", func(item *conformance.CaseAssessment) { item.ProfileFingerprint = strings.Repeat("5", 64) }},
		{"producer", func(item *conformance.CaseAssessment) { item.ProducerFingerprint = strings.Repeat("6", 64) }},
		{"comparison verdict", func(item *conformance.CaseAssessment) { item.Executions[0].Comparisons[0].Passed = false }},
		{"comparison shape", func(item *conformance.CaseAssessment) { item.Executions[0].Comparisons = nil }},
		{"case identity", func(item *conformance.CaseAssessment) { item.ID = "changed-case" }},
		{"binary identity", func(item *conformance.CaseAssessment) {
			item.Executions[0].Components[0].SHA256 = strings.Repeat("7", 64)
		}},
	} {
		t.Run(changed.name, func(t *testing.T) {
			report, item, raw := capture()
			changed.mutate(&item)
			current, err := identity.Stamp(report, item, "en", "docs/gen/evidence/en/cases/case.md", raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := EvidenceCasePageCurrent(published, current); err == nil {
				t.Fatalf("%s mutation retained logical currentness", changed.name)
			}
		})
	}
	for _, destination := range []struct{ language, target string }{{"ru", "docs/gen/evidence/en/cases/case.md"}, {"en", "docs/gen/evidence/en/cases/other.md"}} {
		current, err := identity.Stamp(secondReport, secondCase, destination.language, destination.target, secondBody)
		if err != nil {
			t.Fatal(err)
		}
		if err := EvidenceCasePageCurrent(published, current); err == nil {
			t.Fatal("locale/path identity mutation was accepted")
		}
	}
	if err := os.WriteFile(sourcePath, []byte("package source\nconst Revision = 2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	changedIdentity, err := NewEvidenceCaseIdentity(graph, secondReport)
	if err != nil {
		t.Fatal(err)
	}
	changedSource, err := changedIdentity.Stamp(secondReport, secondCase, "en", "docs/gen/evidence/en/cases/case.md", secondBody)
	if err != nil {
		t.Fatal(err)
	}
	if err := EvidenceCasePageCurrent(published, changedSource); err == nil {
		t.Fatal("authored source mutation retained logical currentness")
	}
	graph.Requirements[0].Cases[0].Profile = "other-profile"
	declaredIdentity, err := NewEvidenceCaseIdentity(graph, secondReport)
	if err != nil {
		t.Fatal(err)
	}
	changedDeclaration, err := declaredIdentity.Stamp(secondReport, secondCase, "en", "docs/gen/evidence/en/cases/case.md", secondBody)
	if err != nil {
		t.Fatal(err)
	}
	if err := EvidenceCasePageCurrent(changedSource, changedDeclaration); err == nil {
		t.Fatal("declared profile mutation retained logical currentness")
	}
}

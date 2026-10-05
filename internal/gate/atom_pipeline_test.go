package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

func writeAtomPipelineFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["spec/go.mod"] = "module example.test/pipeline\n\ngo 1.22\n"
	files["spec/hotamspec/hotamspec.go"] = vendor.Source()
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestAtomRecordingOutcomeKeepsPassingSubtestBesideFailure(t *testing.T) {
	root := writeAtomPipelineFixture(t, map[string]string{
		"manifest.json": `{"self_hosting":false,"parent":null,"conformance":{"rule_cases":true}}`,
		"spec/model/value.go": `package model

type Box struct { value int }
// values follow the rule
func (b Box) Value() int { return b.value }
`,
		"spec/model/value_test.go": `package model

import (
	"testing"
	"example.test/pipeline/hotamspec"
)

func TestCases(t *testing.T) {
	for _, sample := range []struct { name string; actual, want int }{{"good", 7, 7}, {"bad", 9, 8}} {
		t.Run(sample.name, func(t *testing.T) {
			box := Box{value: sample.actual}
			ctx := hotamspec.CaseContext{ID: "case-" + sample.name, AtomIDs: []string{"R-box-value"}, Operation: "decode", Producer: "adapter"}
			hotamspec.Fact(t, box.Value, sample.want, hotamspec.WithInput(sample.actual), hotamspec.WithCase(ctx))
		})
	}
}
`,
	})
	graph := &ontology.Graph{
		DomainDir:   root,
		Conformance: &ontology.ConformanceConfig{RuleCases: true},
	}
	index, err := NewAtomSourceIndexForGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	entry := "spec/model/value_test.go:TestCases"
	result := RunAtomPackageRecording(root, "spec/model/value_test.go")
	if result.Err != nil || result.CompileFailed {
		t.Fatalf("fixture recording failed to execute: %+v", result)
	}
	requirement := ontology.Requirement{
		ID: "R-box-value", Claim: "Values follow the rule.", AtomKind: "rule",
		ImplementedBy: []string{"spec/model/value.go:Box.Value"}, VerifiedBy: []string{entry},
	}
	outcome := atomRecordingOutcome(index, requirement, entry, "TestCases", result)
	if outcome.passed {
		t.Fatal("entry with a failing nested case must not report all-pass")
	}
	if len(outcome.artifacts) != 1 || len(outcome.failedArtifacts) != 1 {
		t.Fatalf("passing/failing sibling artifacts = %d/%d, want 1/1", len(outcome.artifacts), len(outcome.failedArtifacts))
	}
	if outcome.artifacts[0].Case == nil || outcome.artifacts[0].Case.ID != "case-good" {
		t.Fatalf("passing sibling case lost: %+v", outcome.artifacts[0].Case)
	}
	if outcome.failedArtifacts[0].Case == nil || outcome.failedArtifacts[0].Case.ID != "case-bad" {
		t.Fatalf("failed case descriptor lost: %+v", outcome.failedArtifacts[0].Case)
	}
	if outcome.failedArtifacts[0].Title != "" {
		t.Fatalf("failed artifact was given a verified claim title: %q", outcome.failedArtifacts[0].Title)
	}

	row := SpecRow{req: requirement, outcomes: []specTestOutcome{outcome}}
	lines, err := renderSpecRequirement(graph, row, "")
	if err != nil {
		t.Fatal(err)
	}
	document := strings.Join(lines, "\n")
	if !strings.Contains(document, "case-good") || !strings.Contains(document, "case-bad") {
		t.Fatalf("SPEC view lost a passing/failing sibling case:\n%s", document)
	}
}

func TestCollectSpecRowsRefusesMissingSourceLocaleBeforeExecution(t *testing.T) {
	root := writeAtomPipelineFixture(t, map[string]string{
		"manifest.json": `{"self_hosting":false,"parent":null}`,
		"spec/model/value.go": `package model

import "os"

type Box struct{}
// unmarked phrase in a multilingual domain
func (b Box) Value() int {
	_ = os.WriteFile("ran", []byte("method executed"), 0o644)
	return 1
}
`,
		"spec/model/value_test.go": `package model

import (
	"testing"
	"example.test/pipeline/hotamspec"
)

func TestValue(t *testing.T) {
	hotamspec.Fact(t, (Box{}).Value, 1)
}
`,
	})
	graph := &ontology.Graph{
		DomainDir: root, SelfExecutingAtoms: true,
		Languages: []string{"en", "ru"}, DefaultLanguage: "en",
		Requirements: []ontology.Requirement{{
			ID: "R-box-value", Claim: "The value is exact.",
			ClaimTexts:    ontology.LocalizedText{"en": "The value is exact.", "ru": "Значение точно задано."},
			ImplementedBy: []string{"spec/model/value.go:Box.Value"},
			VerifiedBy:    []string{"spec/model/value_test.go:TestValue"},
		}},
	}
	rows := CollectSpecRows(graph)
	row := rows["R-box-value"]
	if row.sourceError == nil || !strings.Contains(row.sourceError.Error(), `language "en"`) {
		t.Fatalf("missing method translation was not surfaced before execution: %+v", row)
	}
	if _, err := os.Stat(filepath.Join(root, "spec", "model", "ran")); !os.IsNotExist(err) {
		t.Fatalf("method executed despite source translation failure; stat error=%v", err)
	}
	documents, err := BuildSpecDocumentsFromRows(graph, rows)
	if err == nil || documents != nil {
		t.Fatalf("SPEC bundle should refuse the source translation failure without partial output: documents=%v err=%v", documents, err)
	}
}

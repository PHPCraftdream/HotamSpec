package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/conformance"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

func TestCollectEmptySelfExecutingGraphKeepsSiblingEvidenceAndFreshFindingIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the evidence collection pipeline over full fixture domains; skipped in -short")
	}

	root := t.TempDir()
	writeEvidenceFixture(t, root, "mismatch-v1", "request-v1", "spec-v1")
	graph := &ontology.Graph{DomainDir: root, SelfExecutingAtoms: true}

	first, err := Collect(graph)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Requirements) != 1 {
		t.Fatalf("report-only atom requirements = %d, want one: %+v", len(first.Requirements), first.Requirements)
	}
	result := first.Requirements[0]
	if result.CoverageStatus != ontology.CoverageDiscrepancy {
		t.Fatalf("failed observed atom coverage = %q, want discrepancy", result.CoverageStatus)
	}
	if len(result.Tests) != 2 {
		t.Fatalf("per-test results = %+v, want passing and failing siblings", result.Tests)
	}
	var passingTest, failingTest *TestEvidence
	for i := range result.Tests {
		switch {
		case strings.HasSuffix(result.Tests[i].Reference, ":TestPassingSibling"):
			passingTest = &result.Tests[i]
		case strings.HasSuffix(result.Tests[i].Reference, ":TestFailingSibling"):
			failingTest = &result.Tests[i]
		}
	}
	if passingTest == nil || passingTest.Verdict != VerdictPass || failingTest == nil || failingTest.Verdict != VerdictFail {
		t.Fatalf("qualified sibling test verdicts = %+v", result.Tests)
	}
	if len(failingTest.Artifacts) != 1 {
		t.Fatalf("failing test artifacts = %+v", failingTest.Artifacts)
	}
	artifact := failingTest.Artifacts[0]
	if artifact.Test != failingTest.Reference || !strings.HasSuffix(artifact.Subject, ".Result.Value") {
		t.Fatalf("artifact trace = %+v; want qualified test and bound method", artifact)
	}
	var nested, outer bool
	for _, observation := range artifact.Observations {
		switch observation.Name {
		case "year":
			nested = observation.Input == "request-v1" && observation.Actual == "1968" && observation.Expected == "mismatch-v1" && !observation.Passed
		case artifact.Subject:
			outer = observation.Input == "request-v1" && observation.Actual == "actual" && observation.Expected == "actual" && observation.Passed
		}
	}
	if !nested || !outer {
		t.Fatalf("nested and passing outer comparisons not both preserved: %+v", artifact.Observations)
	}

	var initialFinding Finding
	for _, finding := range first.Findings {
		if finding.Kind != FindingDiscrepancy || !strings.HasSuffix(finding.Test, "result_test.go:TestFailingSibling") {
			continue
		}
		initialFinding = finding
		break
	}
	if initialFinding.ID == "" {
		t.Fatalf("failed observation finding = %+v", initialFinding)
	}
	var observation *Observation
	for i := range initialFinding.Observations {
		if initialFinding.Observations[i].Name == "year" {
			observation = &initialFinding.Observations[i]
			break
		}
	}
	if observation == nil {
		t.Fatalf("finding omitted its nested comparison: %+v", initialFinding)
	}
	if observation.Input != "request-v1" || observation.Actual != "1968" || observation.Expected != "mismatch-v1" || observation.Passed {
		t.Fatalf("failed observation details = %+v", observation)
	}
	if !strings.HasSuffix(initialFinding.Subject, ".Result.Value") || initialFinding.Subject == observation.Name {
		t.Fatalf("finding subject is not the bound method: finding=%+v", initialFinding)
	}
	if initialFinding.Context.ImplementationVersion != "impl-v1" || initialFinding.Context.SpecVersion != "spec-v1" {
		t.Fatalf("finding context = %+v", initialFinding.Context)
	}

	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := Collect(graph)
	if err != nil {
		t.Fatal(err)
	}
	repeatedJSON, err := json.Marshal(repeated)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(repeatedJSON) {
		t.Fatalf("unchanged fresh evidence report is not stable:\nfirst: %s\nrepeat: %s", firstJSON, repeatedJSON)
	}

	writeEvidenceFixture(t, root, "mismatch-v2", "request-v2", "spec-v2")
	changed, err := Collect(graph)
	if err != nil {
		t.Fatal(err)
	}
	var changedFinding Finding
	for _, finding := range changed.Findings {
		if finding.Kind == FindingDiscrepancy && strings.HasSuffix(finding.Test, "result_test.go:TestFailingSibling") {
			changedFinding = finding
			break
		}
	}
	if changedFinding.ID == "" || changedFinding.ID == initialFinding.ID {
		t.Fatalf("changed input/expectation/version reused finding identity: old=%+v new=%+v", initialFinding, changedFinding)
	}
	changedObservation := changedFinding.Observations[0]
	if changedObservation.Input != "request-v2" || changedObservation.Expected != "mismatch-v2" || changedFinding.Context.SpecVersion != "spec-v2" {
		t.Fatalf("changed report omitted updated evidence: finding=%+v", changedFinding)
	}
}

func writeEvidenceFixture(t *testing.T, root, expected, input, specVersion string) {
	t.Helper()
	write := func(relative, content string) {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("spec/go.mod", "module example.com/evidencefixture\n\ngo 1.22\n")
	write("spec/hotamspec/hotamspec.go", recordervendor.Source())
	write("spec/model/result.go", `package model

import "example.com/evidencefixture/hotamspec"

// Result returns an observable string value.
type Result struct {
	Input       string
	Expected    string
	CheckActual string
}

// Value returns the current method result.
func (r Result) Value() hotamspec.ObservedValue[string] {
	return hotamspec.Observed("actual", hotamspec.Observe("year", r.Input, r.CheckActual, r.Expected))
}
`)
	tests := fmt.Sprintf(`package model_test

import (
	"testing"

	"example.com/evidencefixture/hotamspec"
	"example.com/evidencefixture/model"
)

func TestPassingSibling(t *testing.T) {
	result := model.Result{Input: "passing-input", Expected: "actual", CheckActual: "actual"}
	hotamspec.Fact(t, result.Value, "actual", hotamspec.WithInput(result.Input), hotamspec.WithContext(hotamspec.ArtifactContext{SpecVersion: "spec-pass"}))
}

func TestFailingSibling(t *testing.T) {
	result := model.Result{Input: %s, Expected: %s, CheckActual: "1968"}
	hotamspec.Fact(t, result.Value, "actual", hotamspec.WithInput(result.Input), hotamspec.WithContext(hotamspec.ArtifactContext{ImplementationVersion: "impl-v1", SpecVersion: %s, Profile: "fixture"}))
}
`, strconv.Quote(input), strconv.Quote(expected), strconv.Quote(specVersion))
	write("spec/model/result_test.go", tests)
}

func TestCollectConcurrentDomainsKeepEvidenceLocal(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the evidence collection pipeline over full fixture domains; skipped in -short")
	}

	type outcome struct {
		name   string
		report Report
		err    error
	}
	domains := []struct {
		name     string
		expected string
		input    string
		version  string
	}{
		{name: "left", expected: "left-expected", input: "left-input", version: "left-spec"},
		{name: "right", expected: "right-expected", input: "right-input", version: "right-spec"},
	}
	results := make(chan outcome, len(domains))
	for _, domain := range domains {
		root := t.TempDir()
		writeEvidenceFixture(t, root, domain.expected, domain.input, domain.version)
		graph := &ontology.Graph{DomainDir: root, SelfExecutingAtoms: true}
		go func(name string, graph *ontology.Graph) {
			report, err := Collect(graph)
			results <- outcome{name: name, report: report, err: err}
		}(domain.name, graph)
	}
	for range domains {
		result := <-results
		if result.err != nil {
			t.Fatalf("%s collection: %v", result.name, result.err)
		}
		if len(result.report.Requirements) != 1 || len(result.report.Findings) == 0 {
			t.Fatalf("%s report is incomplete: %+v", result.name, result.report)
		}
		encoded, err := json.Marshal(result.report)
		if err != nil {
			t.Fatal(err)
		}
		var ownInput, otherInput string
		if result.name == "left" {
			ownInput, otherInput = "left-input", "right-input"
		} else {
			ownInput, otherInput = "right-input", "left-input"
		}
		if !strings.Contains(string(encoded), ownInput) || strings.Contains(string(encoded), otherInput) {
			t.Fatalf("%s report contains another domain's evidence: %s", result.name, encoded)
		}
	}
}
func TestCollectRuleCasesPreservesRawBytesBitsAbsenceAndCompositionIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the evidence collection pipeline over full fixture domains; skipped in -short")
	}

	root := t.TempDir()
	writeCase := func(relative, content string) {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeCase("spec/go.mod", "module example.com/casefixture\n\ngo 1.25.0\n")
	writeCase("spec/hotamspec/hotamspec.go", recordervendor.Source())
	writeCase("spec/model/result.go", `package model

import "example.com/casefixture/hotamspec"

// Value preserves a result's exact typed payload.
type Result struct {
	Kind  string
	Bytes []byte
	Float float64
}

// Value returns one typed observation without decoding or normalizing it.
func (r Result) Value() hotamspec.TypedValue {
	switch r.Kind {
	case "bytes":
		return hotamspec.Bytes(r.Bytes)
	case "float":
		return hotamspec.Float64Bits(r.Float)
	default:
		return hotamspec.Scalar("bool", false)
	}
}
`)
	writeCase("spec/model/result_test.go", `package model_test

import (
	"math"
	"testing"

	"example.com/casefixture/hotamspec"
	"example.com/casefixture/model"
)

func TestCases(t *testing.T) {
	t.Run("raw-bytes", func(t *testing.T) {
		input := []byte{0xff, 0x00, '\r', '\n'}
		result := model.Result{Kind: "bytes", Bytes: input}
		hotamspec.Fact(t, result.Value, hotamspec.Bytes(input),
			hotamspec.WithInput(hotamspec.Bytes(input)),
			hotamspec.WithCase(hotamspec.CaseContext{
				ID: "case-raw-bytes", AtomIDs: []string{"R-typed-value"},
				Profile: "typed-v1", Operation: "decode", Target: "direct-sdk", Producer: "sdk",
			}),
			hotamspec.WithContext(hotamspec.ArtifactContext{
				Profile: "typed-v1", Operation: "decode", Target: "direct-sdk", Producer: "sdk",
			}))
	})
	t.Run("signed-zero", func(t *testing.T) {
		result := model.Result{Kind: "float", Float: 0}
		hotamspec.Fact(t, result.Value, hotamspec.Float64Bits(math.Copysign(0, -1)),
			hotamspec.WithInput(hotamspec.Text("negative-zero")),
			hotamspec.WithCase(hotamspec.CaseContext{
				ID: "case-signed-zero", AtomIDs: []string{"R-typed-value"},
				Profile: "typed-v1", Operation: "float", Target: "composed-app", Producer: "adapter",
			}),
			hotamspec.WithContext(hotamspec.ArtifactContext{
				Profile: "typed-v1", Operation: "float", Target: "composed-app", Producer: "adapter-v2",
				Components: []hotamspec.Component{{ID: "adapter", Role: "adapter", Version: "v2", SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Measured: true}},
			}))
	})
	t.Run("bool-false-absent-input", func(t *testing.T) {
		result := model.Result{Kind: "bool"}
		hotamspec.Fact(t, result.Value, hotamspec.Scalar("bool", false),
			hotamspec.WithCase(hotamspec.CaseContext{
				ID: "case-bool-false", AtomIDs: []string{"R-typed-value"},
				Profile: "typed-v1", Operation: "boolean", Target: "direct-sdk", Producer: "sdk",
			}),
			hotamspec.WithContext(hotamspec.ArtifactContext{
				Profile: "typed-v1", Operation: "boolean", Target: "direct-sdk", Producer: "sdk",
			}))
	})
}
`)

	g := &ontology.Graph{}
	g.DomainDir = root
	g.SelfExecutingAtoms = true
	g.Conformance = &ontology.ConformanceConfig{
		RuleCases: true,
		Profiles: []ontology.Profile{{
			ID: "typed-v1", Operations: []string{"decode", "float", "boolean"},
			Features: []string{"raw-bytes", "float-bits", "typed-bool"},
		}},
		Compositions: []ontology.Composition{{
			ID:         "composed-app",
			Components: []ontology.Component{{ID: "adapter", Role: "adapter", Version: "v1", SHA256: strings.Repeat("a", 64)}},
		}},
	}
	negativeZeroInput := "negative-zero"
	wantFalse := false
	requirement := ontology.Requirement{
		ID: "R-typed-value", Claim: "A typed result preserves exact payload identity.",
		AtomKind: "rule", Strength: "MUST",
		ImplementedBy: []string{"spec/model/result.go:Result.Value"},
		Cases: []ontology.CaseDefinition{
			{
				ID: "case-raw-bytes", Test: "spec/model/result_test.go:TestCases/raw-bytes",
				AtomIDs: []string{"R-typed-value"}, Profile: "typed-v1", Operation: "decode",
				Target: "direct-sdk", Producer: "sdk",
				Input:    &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: "/wANCg=="},
				Expected: &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: "/wANCg=="},
			},
			{
				ID: "case-signed-zero", Test: "spec/model/result_test.go:TestCases/signed-zero",
				AtomIDs: []string{"R-typed-value"}, Profile: "typed-v1", Operation: "float",
				Target: "composed-app", Producer: "adapter",
				Input:    &ontology.ObservedValue{Kind: "text", Text: &negativeZeroInput},
				Expected: &ontology.ObservedValue{Kind: "float", FloatBits: "8000000000000000"},
			},
			{
				ID: "case-bool-false", Test: "spec/model/result_test.go:TestCases/bool-false-absent-input",
				AtomIDs: []string{"R-typed-value"}, Profile: "typed-v1", Operation: "boolean",
				Target: "direct-sdk", Producer: "sdk",
				Expected: &ontology.ObservedValue{Kind: "bool", Bool: &wantFalse},
			},
		},
	}
	g.Requirements = []ontology.Requirement{requirement}

	report, err := Collect(g)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Requirements) != 1 || len(report.Requirements[0].Tests) != 3 {
		t.Fatalf("case executions or passing siblings were omitted: %+v", report.Requirements)
	}
	caseByID := make(map[string]conformance.CaseAssessment, len(report.Conformance.Cases))
	for _, assessment := range report.Conformance.Cases {
		caseByID[assessment.ID] = assessment
	}
	rawBytes := caseByID["case-raw-bytes"]
	if rawBytes.Status != "verified_case" || rawBytes.Input == nil || rawBytes.Input.Kind != "bytes" ||
		rawBytes.Input.Bytes != "/wANCg==" || rawBytes.Expected == nil || rawBytes.Expected.Bytes != "/wANCg==" {
		t.Fatalf("raw byte case input/expectation was not preserved: %+v", rawBytes)
	}
	if len(rawBytes.Executions) != 1 || len(rawBytes.Executions[0].Comparisons) == 0 {
		t.Fatalf("raw byte case lacks its actual comparison: %+v", rawBytes)
	}
	rawComparison := rawBytes.Executions[0].Comparisons[0]
	if rawComparison.Input == nil || rawComparison.Input.Kind != "bytes" || rawComparison.Input.Bytes != "/wANCg==" ||
		rawComparison.Actual == nil || rawComparison.Actual.Bytes != "/wANCg==" {
		t.Fatalf("raw invalid-UTF-8 bytes were changed in transit: %+v", rawComparison)
	}
	signedZero := caseByID["case-signed-zero"]
	if signedZero.Status != "discrepancy" || len(signedZero.Executions) != 1 ||
		len(signedZero.Executions[0].Comparisons) == 0 {
		t.Fatalf("signed-zero disagreement was hidden: %+v", signedZero)
	}
	floatComparison := signedZero.Executions[0].Comparisons[0]
	if floatComparison.Actual == nil || floatComparison.Actual.FloatBits != "0000000000000000" ||
		floatComparison.Expected == nil || floatComparison.Expected.FloatBits != "8000000000000000" {
		t.Fatalf("binary64 signed-zero bits were not retained: %+v", floatComparison)
	}
	boolCase := caseByID["case-bool-false"]
	if boolCase.Input != nil || boolCase.Expected == nil || boolCase.Expected.Bool == nil || *boolCase.Expected.Bool {
		t.Fatalf("absent input and explicit false expectation were conflated: %+v", boolCase)
	}
	boolComparison := boolCase.Executions[0].Comparisons[0]
	if boolComparison.Input != nil || boolComparison.Actual == nil || boolComparison.Actual.Bool == nil ||
		boolComparison.Expected == nil || boolComparison.Expected.Bool == nil ||
		*boolComparison.Actual.Bool || *boolComparison.Expected.Bool {
		t.Fatalf("raw absent/bool-false comparison was not preserved: %+v", boolComparison)
	}
	var composition conformance.CompositionAssessment
	for _, candidate := range report.Conformance.Compositions {
		if candidate.ID == "composed-app" {
			composition = candidate
		}
	}
	if composition.Status != "changed" || composition.MeasuredFingerprint == "" ||
		len(composition.ObservedComponents) != 1 || composition.ObservedComponents[0].Version != "v2" {
		t.Fatalf("measured composed producer identity was lost: %+v", composition)
	}
	var failingFinding Finding
	for _, finding := range report.Findings {
		if finding.CaseID == "case-signed-zero" && finding.Kind == FindingDiscrepancy {
			failingFinding = finding
		}
	}
	if failingFinding.ID == "" || failingFinding.Target != "composed-app" || failingFinding.Context.Producer != "adapter-v2" {
		t.Fatalf("case finding omitted actual target/producer identity: %+v", failingFinding)
	}

	oldFindingID := failingFinding.ID
	changedExpected := &ontology.ObservedValue{Kind: "float", FloatBits: "0000000000000000"}
	g.Requirements[0].Cases[1].Expected = changedExpected
	changed, err := Collect(g)
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range changed.Findings {
		if finding.CaseID == "case-signed-zero" && finding.Kind == FindingDiscrepancy && finding.ID == oldFindingID {
			t.Fatalf("changed graph case expectation inherited the prior review identity: %s", oldFindingID)
		}
	}
}
func TestSharedCaseInfersPrimaryAtomsBeforeAndAfterGraphSync(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the evidence collection pipeline over full fixture domains; skipped in -short")
	}

	root := t.TempDir()
	write := func(relative, contents string) {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("spec/go.mod", "module example.com/casefixture\n\ngo 1.25.0\n")
	write("spec/hotamspec/hotamspec.go", recordervendor.Source())
	write("spec/model/result.go", `package model

import (
	"strconv"

	"example.com/casefixture/hotamspec"
)

type Result struct{ Data []byte }

// Bytes returns the exact payload bytes.
func (r Result) Bytes() hotamspec.TypedValue {
	return hotamspec.Bytes(r.Data)
}

// Length returns the payload byte count.
func (r Result) Length() hotamspec.TypedValue {
	return hotamspec.Integer(strconv.Itoa(len(r.Data)))
}
`)
	write("spec/model/result_test.go", `package model_test

import (
	"testing"

	"example.com/casefixture/hotamspec"
	"example.com/casefixture/model"
)

func TestShared(t *testing.T) {
	input := []byte{'a', 0}
	oracle := hotamspec.Bytes(input)
	caseContext := hotamspec.CaseContext{
		ID: "case-shared", Profile: "typed-v1", Operation: "inspect",
		Target: "direct-sdk", Producer: "sdk", Expected: &oracle,
	}
	context := hotamspec.ArtifactContext{
		Profile: "typed-v1", Operation: "inspect", Target: "direct-sdk", Producer: "sdk",
	}
	result := model.Result{Data: input}
	hotamspec.Fact(t, result.Bytes, hotamspec.Bytes(input),
		hotamspec.WithInput(hotamspec.Bytes(input)),
		hotamspec.WithCase(caseContext), hotamspec.WithContext(context))
	hotamspec.Fact(t, result.Length, hotamspec.Integer("2"),
		hotamspec.WithInput(hotamspec.Bytes(input)),
		hotamspec.WithCase(caseContext), hotamspec.WithContext(context))
}
`)

	reportOnlyGraph := &ontology.Graph{DomainDir: root, SelfExecutingAtoms: true}
	reportOnlyGraph.Conformance = &ontology.ConformanceConfig{RuleCases: true}
	reportOnly, err := Collect(reportOnlyGraph)
	if err != nil {
		t.Fatal(err)
	}
	if reportOnly.Conformance.CasesObserved != 1 {
		t.Fatalf("report-only shared case observations = %d, want one case: %+v", reportOnly.Conformance.CasesObserved, reportOnly.Conformance)
	}
	for _, issue := range reportOnly.Conformance.Issues {
		if issue.Code == conformance.IssueCaseMetadataConflict {
			t.Fatalf("heterogeneous report-only case acquired a false metadata conflict: %+v", issue)
		}
	}

	input := &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: "YQA="}
	expected := &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: "YQA="}
	definition := ontology.CaseDefinition{
		ID: "case-shared", Test: "spec/model/result_test.go:TestShared",
		AtomIDs: []string{"R-shared-bytes", "R-shared-length"},
		Profile: "typed-v1", Operation: "inspect", Target: "direct-sdk", Producer: "sdk",
		Input: input, Expected: expected,
	}
	syncedGraph := &ontology.Graph{DomainDir: root, SelfExecutingAtoms: true}
	syncedGraph.Conformance = &ontology.ConformanceConfig{
		RuleCases: true,
		Profiles:  []ontology.Profile{{ID: "typed-v1", Operations: []string{"inspect"}}},
	}
	syncedGraph.Requirements = []ontology.Requirement{
		{
			ID: "R-shared-bytes", Claim: "The byte property is preserved.",
			AtomKind: "rule", Strength: "MUST", ImplementedBy: []string{"spec/model/result.go:Result.Bytes"},
			Cases: []ontology.CaseDefinition{definition},
		},
		{
			ID: "R-shared-length", Claim: "The length property is preserved.",
			AtomKind: "rule", Strength: "MUST", ImplementedBy: []string{"spec/model/result.go:Result.Length"},
			Cases: []ontology.CaseDefinition{definition},
		},
	}
	synced, err := Collect(syncedGraph)
	if err != nil {
		t.Fatal(err)
	}
	if len(synced.Conformance.Cases) != 2 {
		t.Fatalf("synced shared case was not assessed per atom: %+v", synced.Conformance.Cases)
	}
	assessmentByAtom := make(map[string]conformance.CaseAssessment, len(synced.Conformance.Cases))
	for _, assessment := range synced.Conformance.Cases {
		assessmentByAtom[assessment.AtomID] = assessment
	}
	for _, atomID := range []string{"R-shared-bytes", "R-shared-length"} {
		assessment := assessmentByAtom[atomID]
		if assessment.Status != "verified_case" || len(assessment.Executions) != 1 ||
			assessment.Executions[0].AtomID != atomID || len(assessment.Executions[0].Comparisons) != 1 {
			t.Fatalf("shared property was not tied to its own observation: atom=%s assessment=%+v", atomID, assessment)
		}
		if assessment.Expected == nil || assessment.Expected.Kind != "bytes" || assessment.Expected.Bytes != "YQA=" {
			t.Fatalf("case-wide oracle was not retained for %s: %+v", atomID, assessment)
		}
	}
	bytesComparison := assessmentByAtom["R-shared-bytes"].Executions[0].Comparisons[0]
	lengthComparison := assessmentByAtom["R-shared-length"].Executions[0].Comparisons[0]
	if bytesComparison.Expected == nil || bytesComparison.Expected.Kind != "bytes" ||
		lengthComparison.Expected == nil || lengthComparison.Expected.Integer != "2" {
		t.Fatalf("property-specific comparison expectations were conflated: bytes=%+v length=%+v", bytesComparison, lengthComparison)
	}
	if synced.Conformance.HasBlockingIssues() {
		t.Fatalf("inferred case atom links produced blocking audit issues: %+v", synced.Conformance.Issues)
	}
	for _, issue := range synced.Conformance.Issues {
		if issue.Code == conformance.IssueCaseMetadataConflict || issue.Code == conformance.IssueCaseUnknownAtom {
			t.Fatalf("synced inferred atom IDs were not normalized consistently: %+v", issue)
		}
	}
}

func TestLegacyFindingIdentityIgnoresAddedTypedEvidenceMetadata(t *testing.T) {
	legacy := Finding{
		RequirementID: "R-legacy", Claim: "claim", Test: "spec/model/test.go:TestLegacy",
		Subject: "Model.Read", Kind: FindingDiscrepancy, Rationale: "mismatch", Profile: "default",
		Observations: []Observation{{Name: "value", Input: "in", Actual: "�", Expected: "expected", Passed: false}},
		Context:      Context{Implementation: "impl", ImplementationVersion: "v1", SpecVersion: "s1", Profile: "default"},
	}
	expanded := legacy
	expanded.Observations = append([]Observation(nil), legacy.Observations...)
	expanded.Observations[0].RawActual = &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: "/w=="}
	expanded.Observations[0].RawExpected = &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: "AA=="}
	expanded.Context.ProfileDetails = &ontology.Profile{ID: "default"}
	expanded.Context.Operation = "decode"
	expanded.Context.Target = "composed"
	expanded.Context.Producer = "adapter-v2"
	expanded.Context.ProfileFingerprint = "profile-fingerprint"
	expanded.Context.ProducerFingerprint = "producer-fingerprint"
	expanded.Context.CaseFingerprint = "case-fingerprint"

	if got, want := findingID(expanded, nil), findingID(legacy, nil); got != want {
		t.Fatalf("typed metadata changed a legacy review identity: got %s, want %s", got, want)
	}
}

func TestPassingRuleCasesAreNotPromotedToUniversalCoverage(t *testing.T) {
	result := RequirementResult{
		Tests: []TestEvidence{{
			Reference: "spec/model/result_test.go:TestCases",
			Verdict:   VerdictPass,
			Artifacts: []ArtifactEvidence{{
				Verdict:      VerdictPass,
				Observations: []Observation{{Name: "case", Actual: "true", Expected: "true", Passed: true}},
			}},
		}},
	}
	requirement := ontology.Requirement{ID: "R-finite-rule", AtomKind: "rule"}
	if status := deriveCoverage(result, requirement, false, false, nil); status != ontology.CoverageUnverified {
		t.Fatalf("passing finite rule cases implied universal coverage: got %q", status)
	}
}

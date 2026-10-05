package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingsReviewPreservesObservedPayloadAndDoesNotCarryToChangedID(t *testing.T) {
	domainDir := t.TempDir()
	reportPath := filepath.Join(domainDir, "docs", "gen", "evidence.json")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"schema_version":1,"requirements":[],"findings":[{"id":"F-old","requirement_id":"R-example","claim":"The output matches the source contract.","test":"spec/example_test.go:TestOutput","subject":"example.Output","kind":"discrepancy","review_status":"unreviewed","observations":[{"name":"output","input":"{\"mode\":\"safe\"}","actual":"unsafe","expected":"safe","passed":false}],"context":{"implementation":"spec/output.go:Output","implementation_version":"v1","spec_version":"s1","profile":"consumer"}}],"sources":[]}`)
	if err := os.WriteFile(reportPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	reviewOutput := captureFindingsStdout(t, func() error {
		return cmdFindingsReview([]string{
			"--domain", domainDir,
			"--kind", "implementation_issue",
			"--status", "resolved",
			"--rationale", "The implementation output diverges from the cited contract.",
			"--decision-ref", "DEC-42",
			"--json",
			"F-old",
		})
	})
	var saved findingReview
	if err := json.Unmarshal([]byte(reviewOutput), &saved); err != nil {
		t.Fatalf("review command JSON: %v\n%s", err, reviewOutput)
	}
	if saved.FindingID != "F-old" || saved.Kind != "implementation_issue" || saved.Status != "resolved" || saved.DecisionReference != "DEC-42" {
		t.Fatalf("saved review = %+v", saved)
	}
	persistedReport, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(persistedReport) != string(original) {
		t.Fatalf("human review changed the generated observation payload\nwant: %s\n got: %s", original, persistedReport)
	}

	showOutput := captureFindingsStdout(t, func() error {
		return cmdFindingsShow([]string{"--domain", domainDir, "--json", "F-old"})
	})
	var shown findingView
	if err := json.Unmarshal([]byte(showOutput), &shown); err != nil {
		t.Fatalf("findings show JSON: %v\n%s", err, showOutput)
	}
	if shown.Finding.ID != "F-old" || len(shown.Finding.Observations) != 1 || shown.Finding.Observations[0].Actual != "unsafe" || shown.Finding.Observations[0].Expected != "safe" || shown.Finding.Observations[0].Passed {
		t.Fatalf("show lost observed evidence: %+v", shown.Finding)
	}
	if shown.Review == nil || shown.Review.Kind != "implementation_issue" || shown.Review.Status != "resolved" || shown.Review.DecisionReference != "DEC-42" {
		t.Fatalf("show did not expose the separate human review: %+v", shown.Review)
	}

	changed := strings.Replace(string(original), `"F-old"`, `"F-new"`, 1)
	changed = strings.Replace(changed, `"unsafe"`, `"new-output"`, 1)
	if err := os.WriteFile(reportPath, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	newShowOutput := captureFindingsStdout(t, func() error {
		return cmdFindingsShow([]string{"--domain", domainDir, "--json", "F-new"})
	})
	var changedView findingView
	if err := json.Unmarshal([]byte(newShowOutput), &changedView); err != nil {
		t.Fatalf("changed findings show JSON: %v\n%s", err, newShowOutput)
	}
	if changedView.Review != nil || changedView.Finding.Observations[0].Actual != "new-output" {
		t.Fatalf("changed evidence inherited an old review or lost its actual value: %+v", changedView)
	}
	if err := cmdFindingsShow([]string{"--domain", domainDir, "F-old"}); err == nil || !strings.Contains(err.Error(), "stale ID") {
		t.Fatalf("stale finding ID error = %v, want clear stale-ID error", err)
	}

	listOutput := captureFindingsStdout(t, func() error {
		return cmdFindingsList([]string{"--domain", domainDir, "--json"})
	})
	var listed []findingView
	if err := json.Unmarshal([]byte(listOutput), &listed); err != nil {
		t.Fatalf("findings list JSON: %v\n%s", err, listOutput)
	}
	if len(listed) != 1 || listed[0].Finding.ID != "F-new" || listed[0].Review != nil {
		t.Fatalf("list did not reflect only the current report: %+v", listed)
	}
}

func captureFindingsStdout(t *testing.T, run func() error) string {
	t.Helper()
	old := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	runErr := run()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	output, readErr := io.ReadAll(reader)
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatalf("command failed: %v", runErr)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(output)
}

package generator

import (
	"os"
	"testing"
)

func TestBuildHistory_ByteIdenticalToPython(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	got := BuildHistory(g)
	want, err := os.ReadFile("testdata/HISTORY.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "HISTORY.md", got, string(want))
}

func TestBuildDecisions_ByteIdenticalToPython(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	got := BuildDecisions(g)
	want, err := os.ReadFile("testdata/DECISIONS.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "DECISIONS.md", got, string(want))
}

func TestBuildGraphJSON_ByteIdenticalToPython(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	got, err := BuildGraphJSON(g)
	if err != nil {
		t.Fatalf("BuildGraphJSON: %v", err)
	}
	want, err := os.ReadFile("testdata/docs-gen-graph.json")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "docs-gen-graph.json", got, string(want))
}

func TestBuildHistoryDecisionsGraphJSON_AgainstOriginalHotamSpecPath(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)

	historyWant, err := os.ReadFile(`D:\ai_dev\prat\HotamSpec\domains\hotam-spec-self\docs\gen\HISTORY.md`)
	if err != nil {
		t.Logf("skip HISTORY.md: %v", err)
	} else {
		diffReport(t, "HISTORY.md (original path)", BuildHistory(g), string(historyWant))
	}

	graphJSONWant, err := os.ReadFile(`D:\ai_dev\prat\HotamSpec\domains\hotam-spec-self\docs\gen\graph.json`)
	if err != nil {
		t.Logf("skip graph.json: %v", err)
	} else {
		got, err := BuildGraphJSON(g)
		if err != nil {
			t.Fatalf("BuildGraphJSON: %v", err)
		}
		diffReport(t, "graph.json (original path)", got, string(graphJSONWant))
	}
}

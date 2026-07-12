package generator

import (
	"os"
	"testing"
)

func TestBuildHistory_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildHistory(g)
	want, err := os.ReadFile("testdata/fixture/HISTORY.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "HISTORY.md", got, string(want))
}

func TestBuildDecisions_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildDecisions(g)
	want, err := os.ReadFile("testdata/fixture/DECISIONS.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "DECISIONS.md", got, string(want))
}

func TestBuildGraphJSON_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got, err := BuildGraphJSON(g)
	if err != nil {
		t.Fatalf("BuildGraphJSON: %v", err)
	}
	want, err := os.ReadFile("testdata/fixture/docs-gen-graph.json")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "docs-gen-graph.json", got, string(want))
}

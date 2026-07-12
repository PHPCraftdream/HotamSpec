package generator

import (
	"os"
	"testing"
)

func TestBuildOpen_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildOpen(g)
	want, err := os.ReadFile("testdata/fixture/OPEN.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "OPEN.md", got, string(want))
}

func TestBuildUnenforced_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildUnenforced(g)
	want, err := os.ReadFile("testdata/fixture/UNENFORCED.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "UNENFORCED.md", got, string(want))
}

func TestBuildGlossary_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildGlossary(g)
	want, err := os.ReadFile("testdata/fixture/GLOSSARY.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "GLOSSARY.md", got, string(want))
}

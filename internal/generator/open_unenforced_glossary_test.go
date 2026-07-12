package generator

import (
	"os"
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func TestBuildOpen_ByteIdenticalToPython(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	got := BuildOpen(g)
	want, err := os.ReadFile("testdata/OPEN.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "OPEN.md", got, string(want))
}

func TestBuildUnenforced_ByteIdenticalToPython(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	got := BuildUnenforced(g)
	want, err := os.ReadFile("testdata/UNENFORCED.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "UNENFORCED.md", got, string(want))
}

func TestBuildGlossary_ByteIdenticalToPython(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	got := BuildGlossary(g)
	want, err := os.ReadFile("testdata/GLOSSARY.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "GLOSSARY.md", got, string(want))
}

func TestByteIdentical_OpenUnenforcedGlossary_AgainstOriginalHotamSpecPath(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		path string
		fn   func(*ontology.Graph) string
	}{
		{"OPEN.md", `D:\ai_dev\prat\HotamSpec\domains\hotam-spec-self\docs\gen\OPEN.md`, BuildOpen},
		{"UNENFORCED.md", `D:\ai_dev\prat\HotamSpec\domains\hotam-spec-self\docs\gen\UNENFORCED.md`, BuildUnenforced},
		{"GLOSSARY.md", `D:\ai_dev\prat\HotamSpec\domains\hotam-spec-self\docs\gen\GLOSSARY.md`, BuildGlossary},
	} {
		want, err := os.ReadFile(c.path)
		if err != nil {
			t.Logf("skip %s: %v", c.name, err)
			continue
		}
		g := loadDomainGraph(t)
		got := c.fn(g)
		diffReport(t, c.name+" (original path)", got, string(want))
	}
}

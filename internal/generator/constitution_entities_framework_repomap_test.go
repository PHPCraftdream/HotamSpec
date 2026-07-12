package generator

import (
	"os"
	"testing"
)

func TestBuildConstitution_ByteIdenticalToPython(t *testing.T) {
	g := loadDomainGraph(t)
	got := BuildConstitution(g)
	want, err := os.ReadFile("testdata/CONSTITUTION.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "CONSTITUTION.md", got, string(want))
}

func TestBuildFrameworkInvariants_ByteIdenticalToPython(t *testing.T) {
	g := loadDomainGraph(t)
	got := BuildFrameworkInvariants(g, "hotam-spec-self")
	want, err := os.ReadFile("testdata/FRAMEWORK-INVARIANTS.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "FRAMEWORK-INVARIANTS.md", got, string(want))
}

func TestBuildRepoMap_ByteIdenticalToPython(t *testing.T) {
	g := loadDomainGraph(t)
	got := BuildRepoMap(g)
	want, err := os.ReadFile("testdata/REPO-MAP.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "REPO-MAP.md", got, string(want))
}

func TestBuildEntities_MatchesPythonOutput(t *testing.T) {
	g := loadDomainGraph(t)
	got := BuildEntities(g)
	want, err := os.ReadFile("testdata/ENTITIES.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "ENTITIES.md", got, string(want))
}

func TestBuildConstitutionFrameworkRepoMap_AgainstOriginalHotamSpecPath(t *testing.T) {
	g := loadDomainGraph(t)

	for _, c := range []struct {
		name string
		path string
		got  string
	}{
		{"CONSTITUTION.md", `D:\ai_dev\prat\HotamSpec\domains\hotam-spec-self\docs\gen\CONSTITUTION.md`, BuildConstitution(g)},
		{"FRAMEWORK-INVARIANTS.md", `D:\ai_dev\prat\HotamSpec\domains\hotam-spec-self\docs\gen\FRAMEWORK-INVARIANTS.md`, BuildFrameworkInvariants(g, "hotam-spec-self")},
		{"REPO-MAP.md", `D:\ai_dev\prat\HotamSpec\domains\hotam-spec-self\docs\gen\REPO-MAP.md`, BuildRepoMap(g)},
	} {
		want, err := os.ReadFile(c.path)
		if err != nil {
			t.Logf("skip %s: %v", c.name, err)
			continue
		}
		diffReport(t, c.name+" (original path)", c.got, string(want))
	}
}

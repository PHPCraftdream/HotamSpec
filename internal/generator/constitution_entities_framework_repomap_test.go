package generator

import (
	"os"
	"testing"
)

// hotamSpecSelfFixtureGenDocs reconstructs the docs/gen/ file listing that
// the Go port writes for domains/hotam-spec-self — matching the frozen
// testdata/REPO-MAP.md snapshot. It excludes AUDIT.md (a separate,
// low-traffic review-tool artifact the Go port does not generate at all) and
// DECISIONS.md/ENTITIES.md (both legitimately absent for hotam-spec-self: no
// M-tagged OPEN requirements, no entity_types declared).
func hotamSpecSelfFixtureGenDocs() []GenDocEntry {
	title := func(filename, h1 string) GenDocEntry {
		return GenDocEntry{Filename: filename, Content: "# " + h1}
	}
	return []GenDocEntry{
		title("CONSTITUTION.md", "CONSTITUTION.md — The operator's boot sequence (Hotam-Spec)"),
		title("FRAMEWORK-INVARIANTS.md", "FRAMEWORK-INVARIANTS.md — Framework-plumbing index (Hotam-Spec)"),
		title("GLOSSARY.md", "GLOSSARY.md — Methodology controlled vocabulary (Hotam-Spec)"),
		title("HISTORY.md", "HISTORY.md — Methodology decision history (Hotam-Spec)"),
		title("OPEN.md", "OPEN.md — Open registry (Hotam-Spec)"),
		title("REPO-MAP.md", "REPO-MAP.md — Repository file index (Hotam-Spec)"),
		title("REQUIREMENTS.md", "REQUIREMENTS.md — Requirement roster & methodology (Hotam-Spec)"),
		title("TENSIONS.md", "TENSIONS.md — The tension map (Hotam-Spec)"),
		title("UNENFORCED.md", "UNENFORCED.md — Burn-down meter (Hotam-Spec)"),
	}
}

func TestBuildConstitution_ByteIdenticalToPython(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	got := BuildConstitution(g)
	want, err := os.ReadFile("testdata/CONSTITUTION.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "CONSTITUTION.md", got, string(want))
}

func TestBuildFrameworkInvariants_ByteIdenticalToPython(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	got := BuildFrameworkInvariants(g, "hotam-spec-self")
	want, err := os.ReadFile("testdata/FRAMEWORK-INVARIANTS.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "FRAMEWORK-INVARIANTS.md", got, string(want))
}

func TestBuildRepoMap_ByteIdenticalToPython(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	got := BuildRepoMap(g, "hotam-spec-self", hotamSpecSelfFixtureGenDocs(), false, false)
	want, err := os.ReadFile("testdata/REPO-MAP.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "REPO-MAP.md", got, string(want))
}

func TestBuildEntities_MatchesPythonOutput(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	got := BuildEntities(g, "hotam-spec-self")
	want, err := os.ReadFile("testdata/ENTITIES.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "ENTITIES.md", got, string(want))
}

func TestBuildConstitutionFrameworkRepoMap_AgainstOriginalHotamSpecPath(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)

	for _, c := range []struct {
		name string
		path string
		got  string
	}{
		{"CONSTITUTION.md", `D:\ai_dev\prat\HotamSpec\domains\hotam-spec-self\docs\gen\CONSTITUTION.md`, BuildConstitution(g)},
		{"FRAMEWORK-INVARIANTS.md", `D:\ai_dev\prat\HotamSpec\domains\hotam-spec-self\docs\gen\FRAMEWORK-INVARIANTS.md`, BuildFrameworkInvariants(g, "hotam-spec-self")},
		{"REPO-MAP.md", `D:\ai_dev\prat\HotamSpec\domains\hotam-spec-self\docs\gen\REPO-MAP.md`, BuildRepoMap(g, "hotam-spec-self", hotamSpecSelfFixtureGenDocs(), false, false)},
	} {
		want, err := os.ReadFile(c.path)
		if err != nil {
			t.Logf("skip %s: %v", c.name, err)
			continue
		}
		diffReport(t, c.name+" (original path)", c.got, string(want))
	}
}

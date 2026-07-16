package generator

import (
	"os"
	"testing"
)

// hotamSpecSelfFixtureGenDocs reconstructs the docs/gen/ file listing that
// the generator writes for domains/hotam-spec-self — matching the real domain's
// actual generated file set. It excludes AUDIT.md (a separate, low-traffic
// review-tool artifact the generator does not produce at all) and
// DECISIONS.md/ENTITIES.md (both legitimately absent for hotam-spec-self: no
// M-tagged OPEN requirements, no entity_types declared). Used by the
// real-domain determinism/smoke tests in byteidentical_test.go.
func hotamSpecSelfFixtureGenDocs() []GenDocEntry {
	title := func(filename, h1 string) GenDocEntry {
		return GenDocEntry{Filename: filename, Content: "# " + h1}
	}
	return []GenDocEntry{
		title("CONSTITUTION.md", "CONSTITUTION.md — The operator's boot sequence (Hotam-Spec)"),
		title("COVERAGE.md", "COVERAGE.md — authored-spec discipline coverage (Hotam-Spec)"),
		title("FRAMEWORK-INVARIANTS.md", "FRAMEWORK-INVARIANTS.md — Framework-plumbing index (Hotam-Spec)"),
		title("GLOSSARY.md", "GLOSSARY.md — Methodology controlled vocabulary (Hotam-Spec)"),
		title("HISTORY.md", "HISTORY.md — Methodology decision history (Hotam-Spec)"),
		title("MODELS.md", "MODELS.md — authored object model overview (Hotam-Spec)"),
		title("OPEN.md", "OPEN.md — Open registry (Hotam-Spec)"),
		title("PIPELINE.md", "PIPELINE.md — Domain overview: how this is put together, stage by stage (Hotam-Spec)"),
		title("REPO-MAP.md", "REPO-MAP.md — Repository file index (Hotam-Spec)"),
		title("REQUIREMENTS.md", "REQUIREMENTS.md — Requirement roster & methodology (Hotam-Spec)"),
		title("TENSIONS.md", "TENSIONS.md — The tension map (Hotam-Spec)"),
		title("TRACEABILITY.md", "TRACEABILITY.md — requirement -> implemented_by -> verified_by (Hotam-Spec)"),
		title("UNENFORCED.md", "UNENFORCED.md — Burn-down meter (Hotam-Spec)"),
	}
}

// fixtureGenDocs is the docs/gen/ file listing for the small synthetic
// fixture domain (P2-2): unlike hotam-spec-self, the fixture DOES declare an
// entity_type and an M-tagged OPEN requirement, so DECISIONS.md/ENTITIES.md
// are written too — exercising BuildRepoMap's decisionsWritten=true /
// entitiesWritten=true branch (the hotam-spec-self case above only exercises
// the false/false "not written" placeholder branch).
func fixtureGenDocs() []GenDocEntry {
	title := func(filename, h1 string) GenDocEntry {
		return GenDocEntry{Filename: filename, Content: "# " + h1}
	}
	return []GenDocEntry{
		title("CONSTITUTION.md", "CONSTITUTION.md — The operator's boot sequence (Hotam-Spec)"),
		title("COVERAGE.md", "COVERAGE.md — authored-spec discipline coverage (Hotam-Spec)"),
		title("DECISIONS.md", "DECISIONS.md — Open methodology decisions (Hotam-Spec)"),
		title("ENTITIES.md", "Entities"),
		title("FRAMEWORK-INVARIANTS.md", "FRAMEWORK-INVARIANTS.md — Framework-plumbing index (Hotam-Spec)"),
		title("GLOSSARY.md", "GLOSSARY.md — Methodology controlled vocabulary (Hotam-Spec)"),
		title("HISTORY.md", "HISTORY.md — Methodology decision history (Hotam-Spec)"),
		title("MODELS.md", "MODELS.md — authored object model overview (Hotam-Spec)"),
		title("OPEN.md", "OPEN.md — Open registry (Hotam-Spec)"),
		title("PIPELINE.md", "PIPELINE.md — Domain overview: how this is put together, stage by stage (Hotam-Spec)"),
		title("REPO-MAP.md", "REPO-MAP.md — Repository file index (Hotam-Spec)"),
		title("REQUIREMENTS.md", "REQUIREMENTS.md — Requirement roster & methodology (Hotam-Spec)"),
		title("TENSIONS.md", "TENSIONS.md — The tension map (Hotam-Spec)"),
		title("TRACEABILITY.md", "TRACEABILITY.md — requirement -> implemented_by -> verified_by (Hotam-Spec)"),
		title("UNENFORCED.md", "UNENFORCED.md — Burn-down meter (Hotam-Spec)"),
	}
}

func TestBuildConstitution_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildConstitution(g, "fixture-domain", false)
	want, err := os.ReadFile("testdata/fixture/CONSTITUTION.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "CONSTITUTION.md", got, string(want))
}

func TestBuildFrameworkInvariants_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildFrameworkInvariants(g, "fixture-domain")
	want, err := os.ReadFile("testdata/fixture/FRAMEWORK-INVARIANTS.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "FRAMEWORK-INVARIANTS.md", got, string(want))
}

func TestBuildRepoMap_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildRepoMap(g, "fixture-domain", fixtureGenDocs(), true, true, false)
	want, err := os.ReadFile("testdata/fixture/REPO-MAP.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "REPO-MAP.md", got, string(want))
}

func TestBuildEntities_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildEntities(g, "fixture-domain")
	want, err := os.ReadFile("testdata/fixture/ENTITIES.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "ENTITIES.md", got, string(want))
}

func TestBuildPipeline_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildPipeline(g, "fixture-domain")
	want, err := os.ReadFile("testdata/fixture/PIPELINE.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "PIPELINE.md", got, string(want))
}

func TestBuildTraceability_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildTraceability(g)
	want, err := os.ReadFile("testdata/fixture/TRACEABILITY.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "TRACEABILITY.md", got, string(want))
}

func TestBuildModels_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildModels(g)
	want, err := os.ReadFile("testdata/fixture/MODELS.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "MODELS.md", got, string(want))
}

func TestBuildCoverage_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildCoverage(g)
	want, err := os.ReadFile("testdata/fixture/COVERAGE.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "COVERAGE.md", got, string(want))
}

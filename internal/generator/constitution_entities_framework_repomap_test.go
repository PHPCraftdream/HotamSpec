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
// M-tagged OPEN requirements, no entity_types declared). Task #357 returned
// FRAMEWORK-INVARIANTS.md to docs/gen/ (it is per-domain content). GLOSSARY.md
// is NOT here (task #357 promoted it to the project-root framework/ — see
// fixtureFrameworkDocs). Used by the real-domain determinism/smoke tests in
// byteidentical_test.go.
func hotamSpecSelfFixtureGenDocs() []GenDocEntry {
	title := func(filename, h1 string) GenDocEntry {
		return GenDocEntry{Filename: filename, Content: "# " + h1}
	}
	return []GenDocEntry{
		title("CONSTITUTION.md", "CONSTITUTION.md — The operator's boot sequence (Hotam-Spec)"),
		title("COVERAGE.md", "COVERAGE.md — authored-spec discipline coverage (Hotam-Spec)"),
		title("FRAMEWORK-INVARIANTS.md", "FRAMEWORK-INVARIANTS.md — Framework-plumbing index (Hotam-Spec)"),
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

// fixtureFrameworkDocs is the PROJECT-root framework/ file listing shared
// across all domains (task #357): GLOSSARY.md promoted from docs/gen/ to the
// project root. tools/*.md are NOT listed here — REPO-MAP.md deliberately does
// not enumerate subdirectory contents (mirroring its treatment of
// docs/gen/thinking/), so framework/tools/ stays unlisted too.
func fixtureFrameworkDocs() []GenDocEntry {
	return []GenDocEntry{
		{Filename: "GLOSSARY.md", Content: "# GLOSSARY.md — Methodology controlled vocabulary (Hotam-Spec)"},
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
	got := BuildPipeline(g, "fixture-domain", nil)
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

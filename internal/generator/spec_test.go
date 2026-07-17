package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// specFixtureImplSrc is the implemented_by symbol for the narrated
// requirement below -- a tiny RequireComplete-shaped function, mirroring
// internal/gate/test_exec_test.go's own scenarioImplSrc fixture shape.
const specFixtureImplSrc = `package model

func RequireComplete(fields int) error {
	if fields < 1 {
		return errNotComplete
	}
	return nil
}

var errNotComplete = errStub{}

type errStub struct{}

func (errStub) Error() string { return "not complete" }
`

// specFixtureScenarioTestSrc is the verified_by test for
// "R-spec-narrated": a real hotamspec.Scenario-based test, Given/When/Then/
// Value, calling the real RequireComplete symbol -- this is the entry
// BuildSpec must render as a full narrative.
func specFixtureScenarioTestSrc(modulePath string) string {
	return `package model

import (
	"testing"

	"` + modulePath + `/hotamspec"
)

func TestRequireComplete_Scenario(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-spec-narrated", "RequireComplete rejects zero fields")
	s.Given("a fields count of zero", "fields", 0)
	err := RequireComplete(0)
	s.When("RequireComplete is called")
	s.Then("an error is returned", err != nil)
	s.Value("error_text", err)
}

// TestRequireComplete_Plain proves R-spec-plain-only without ever
// constructing a hotamspec.Scenario -- BuildSpec must report this
// verified_by entry as passing but with no recorded narrative.
func TestRequireComplete_Plain(t *testing.T) {
	if err := RequireComplete(1); err != nil {
		t.Fatalf("RequireComplete(1) = %v, want nil", err)
	}
}
`
}

// writeSpecFixtureModule builds a standalone temp Go module shaped exactly
// like a domain's authored spec/ tree (go.mod + vendored hotamspec recorder
// + model/impl.go + model/impl_test.go), mirroring
// internal/gate/test_exec_test.go's writeRecordingFixture. Returns the
// module root, usable directly as a Graph's DomainDir for a non-self-hosting
// SpecRootForGraph resolution EXCEPT that entries here are NOT prefixed with
// "spec/" (this fixture's model/ sits directly at the module root, not
// under a spec/ subdirectory) -- BuildSpec/gate.RunVerifiedByTestRecording
// only need a valid Go module to `go test` against, they do not enforce the
// "spec/"-prefix scope gate gate.EntryWithinSpecScope applies elsewhere.
func writeSpecFixtureModule(t *testing.T) (moduleRoot string) {
	t.Helper()
	root := t.TempDir()
	modulePath := "example.com/specgen"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	modelDir := filepath.Join(root, "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "impl.go"), []byte(specFixtureImplSrc), 0o644); err != nil {
		t.Fatalf("WriteFile impl.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "impl_test.go"), []byte(specFixtureScenarioTestSrc(modulePath)), 0o644); err != nil {
		t.Fatalf("WriteFile impl_test.go: %v", err)
	}

	hotamspecDir := filepath.Join(root, "hotamspec")
	if err := os.MkdirAll(hotamspecDir, 0o755); err != nil {
		t.Fatalf("MkdirAll hotamspec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hotamspecDir, "hotamspec.go"), []byte(recordervendor.BodyForHash()), 0o644); err != nil {
		t.Fatalf("WriteFile vendored hotamspec.go: %v", err)
	}

	return root
}

// specFixtureGraph builds the small, hand-constructed graph BuildSpec is
// exercised against: one requirement with a narrated scenario
// (R-spec-narrated), one requirement whose verified_by test passes but
// records no scenario (R-spec-plain-only), and one requirement with no
// verified_by at all (R-spec-no-carrier) -- covering every branch BuildSpec
// renders (task W1.3's own honesty requirement: narrated / no-scenario /
// no-verified_by must never blur together).
func specFixtureGraph(domainDir string) *ontology.Graph {
	return &ontology.Graph{
		DomainDir:   domainDir,
		SelfHosting: false,
		Requirements: []ontology.Requirement{
			{
				ID:             "R-spec-narrated",
				Claim:          "RequireComplete ALWAYS rejects a zero fields count.",
				Owner:          "spec-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementENFORCED,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
				ImplementedBy:  []string{"model/impl.go:RequireComplete"},
				VerifiedBy:     []string{"model/impl_test.go:TestRequireComplete_Scenario"},
			},
			{
				ID:             "R-spec-plain-only",
				Claim:          "RequireComplete ALWAYS accepts a positive fields count.",
				Owner:          "spec-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementENFORCED,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
				ImplementedBy:  []string{"model/impl.go:RequireComplete"},
				VerifiedBy:     []string{"model/impl_test.go:TestRequireComplete_Plain"},
			},
			{
				ID:             "R-spec-no-carrier",
				Claim:          "Every SETTLED requirement eventually gets a scenario.",
				Owner:          "spec-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementPROSE,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
			},
		},
	}
}

// TestBuildSpec_NarratesRealScenario proves BuildSpec's core contract: a
// requirement whose verified_by test constructs a hotamspec.Scenario gets
// its ACTUAL Given/When/Then/Value narrative rendered, sourced from a real,
// currently-passing `go test` run (not invented text).
func TestBuildSpec_NarratesRealScenario(t *testing.T) {
	root := writeSpecFixtureModule(t)
	g := specFixtureGraph(root)

	got := BuildSpec(g)

	for _, want := range []string{
		"R-spec-narrated",
		"RequireComplete ALWAYS rejects a zero fields count.",
		"model/impl_test.go:TestRequireComplete_Scenario",
		"RequireComplete rejects zero fields",
		"Given a fields count of zero (fields=0)",
		"When RequireComplete is called",
		"Then an error is returned",
		"**held**",
		"Value (error_text=not complete)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("BuildSpec output missing %q\n--- full output ---\n%s", want, got)
		}
	}
}

// TestBuildSpec_HonestNoScenario proves the "test passes but records no
// scenario" branch: R-spec-plain-only's verified_by test never touches
// hotamspec, so BuildSpec must say so honestly rather than inventing a
// narrative or silently omitting the requirement.
func TestBuildSpec_HonestNoScenario(t *testing.T) {
	root := writeSpecFixtureModule(t)
	g := specFixtureGraph(root)

	got := BuildSpec(g)

	if !strings.Contains(got, "R-spec-plain-only") {
		t.Fatalf("BuildSpec output missing R-spec-plain-only:\n%s", got)
	}
	if !strings.Contains(got, "no hotamspec scenario") {
		t.Errorf("BuildSpec output does not honestly report the no-scenario gap for R-spec-plain-only:\n%s", got)
	}
}

// TestBuildSpec_HonestNoVerifiedBy proves the "no verified_by at all" branch:
// R-spec-no-carrier is listed in its own honest section, never silently
// dropped and never blurred together with the narrated/no-scenario buckets.
func TestBuildSpec_HonestNoVerifiedBy(t *testing.T) {
	root := writeSpecFixtureModule(t)
	g := specFixtureGraph(root)

	got := BuildSpec(g)

	if !strings.Contains(got, "## Without a scenario (no verified_by — honest gap)") {
		t.Fatalf("BuildSpec output missing the no-verified_by section heading:\n%s", got)
	}
	if !strings.Contains(got, "R-spec-no-carrier") {
		t.Errorf("BuildSpec output missing R-spec-no-carrier in the no-verified_by section:\n%s", got)
	}
}

// TestBuildSpec_EmptyGraph proves the calm-empty-notice contract every other
// Build* generator already carries (g.IsEmpty()) holds for BuildSpec too.
func TestBuildSpec_EmptyGraph(t *testing.T) {
	got := BuildSpec(&ontology.Graph{})
	if !strings.Contains(got, EmptyNotice) {
		t.Errorf("BuildSpec on an empty graph did not render EmptyNotice:\n%s", got)
	}
}

// TestBuildSpec_ByteIdenticalAcrossRuns is W1.3's own mandated determinism
// guard (PLAN-scenario-generated-spec.md §3 W1.3: "Детерминизм/byte-
// identical: два прогона BuildSpec -> идентичный SPEC.md"): running BuildSpec
// twice against the SAME unchanged fixture module -- each run independently
// re-executing every verified_by test via a fresh `go test` subprocess, per
// RunVerifiedByTestRecording's own no-cross-call-memoization contract --
// must still produce byte-identical output. This is the generator-level
// proof that internal/recorder/canon's own determinism guarantee (W1.1:
// sorted Given/Value keys, canonical float/pointer/map rendering, no time/
// random capture) survives all the way through to the rendered Markdown.
func TestBuildSpec_ByteIdenticalAcrossRuns(t *testing.T) {
	root := writeSpecFixtureModule(t)
	g := specFixtureGraph(root)

	first := BuildSpec(g)
	second := BuildSpec(g)

	if first != second {
		diffReport(t, "SPEC.md (repeat run)", second, first)
	}
}

// TestBuildSpec_ByteIdenticalToGolden pins BuildSpec's exact rendered output
// against the checked-in golden fixture (testdata/fixture/SPEC.md), the same
// byte-identity discipline TestBuildRequirements_ByteIdenticalToFixture and
// TestBuildTensions_ByteIdenticalToFixture already hold traceability.go's
// sibling generators to -- any future accidental change to BuildSpec's
// rendering shape (heading text, ordering, fact formatting) fails this test
// with a line-level diff instead of silently drifting.
func TestBuildSpec_ByteIdenticalToGolden(t *testing.T) {
	root := writeSpecFixtureModule(t)
	g := specFixtureGraph(root)

	got := BuildSpec(g)
	want, err := os.ReadFile("testdata/fixture/SPEC.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "SPEC.md", got, string(want))
}

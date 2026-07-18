package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// specMDFixtureImplSrc/specMDFixtureTestSrc mirror
// internal/generator/spec_test.go's specFixtureImplSrc/
// specFixtureScenarioTestSrc shape exactly (a tiny RequireComplete-style
// function plus a real hotamspec.Scenario-based verified_by test) -- that
// file's own helpers are package-private to internal/generator and cannot be
// imported here, so this is a deliberate, minimal, self-contained
// reimplementation for this package's own fixture needs.
const specMDFixtureImplSrc = `package model

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

func specMDFixtureTestSrc(modulePath string) string {
	return `package model

import (
	"testing"

	"` + modulePath + `/hotamspec"
)

func TestRequireComplete_Scenario(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-specmd-narrated", "RequireComplete rejects zero fields")
	s.Given("a fields count of zero", "fields", 0)
	err := RequireComplete(0)
	s.When("RequireComplete is called")
	s.Then("an error is returned", err != nil)
	s.Value("error_text", err)
}
`
}

// writeSpecMDFixtureModule builds a standalone temp Go module shaped like a
// domain's authored spec/ tree (go.mod + vendored hotamspec recorder +
// model/impl.go + model/impl_test.go) directly at the module root (no
// "spec/" prefix layer -- mirrors internal/generator/spec_test.go's
// writeSpecFixtureModule, which notes gate.RunVerifiedByTestRecording only
// needs a valid Go module, not the "spec/"-prefix scope gate). Returns the
// module root, usable directly as a Graph's DomainDir.
func writeSpecMDFixtureModule(t *testing.T) (moduleRoot string) {
	t.Helper()
	root := t.TempDir()
	modulePath := "example.com/specmdcheck"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	modelDir := filepath.Join(root, "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "impl.go"), []byte(specMDFixtureImplSrc), 0o644); err != nil {
		t.Fatalf("WriteFile impl.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "impl_test.go"), []byte(specMDFixtureTestSrc(modulePath)), 0o644); err != nil {
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

// specMDFixtureGraph is the small, hand-built graph matching
// writeSpecMDFixtureModule's fixture: one requirement whose verified_by test
// narrates a real hotamspec scenario.
func specMDFixtureGraph(domainDir string) *ontology.Graph {
	return &ontology.Graph{
		DomainDir:   domainDir,
		SelfHosting: false,
		Requirements: []ontology.Requirement{
			{
				ID:             "R-specmd-narrated",
				Claim:          "RequireComplete ALWAYS rejects a zero fields count.",
				Owner:          "spec-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementENFORCED,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
				ImplementedBy:  []string{"model/impl.go:RequireComplete"},
				VerifiedBy:     []string{"model/impl_test.go:TestRequireComplete_Scenario"},
			},
		},
	}
}

// writeCurrentSpecMD renders the ACTUAL current SPEC.md for g (via the same
// gate.CollectSpecRows + gate.BuildSpecFromRows pipeline the check itself
// uses) and writes it to <domainDir>/docs/gen/SPEC.md -- the "genuinely
// current" fixture state every positive-control test in this file starts
// from.
func writeCurrentSpecMD(t *testing.T, g *ontology.Graph) string {
	t.Helper()
	fresh := gate.BuildSpecFromRows(g, gate.CollectSpecRows(g))
	genDir := filepath.Join(g.DomainDir, "docs", "gen")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatalf("MkdirAll docs/gen: %v", err)
	}
	specPath := filepath.Join(genDir, "SPEC.md")
	if err := os.WriteFile(specPath, []byte(fresh), 0o644); err != nil {
		t.Fatalf("WriteFile SPEC.md: %v", err)
	}
	return specPath
}

func TestCheckSpecMDCurrent_NoOpWhenDomainDirEmpty(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	if vs := runCheck(t, "check_spec_md_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a graph with no DomainDir, got %v", vs)
	}
}

func TestCheckSpecMDCurrent_NoOpWhenSpecMDAbsent(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecMDFixtureModule(t)
	g := specMDFixtureGraph(domainDir)
	// Precondition: no docs/gen/SPEC.md written at all -- the scenario-
	// generated-spec layer has not been adopted for this domain.
	if _, err := os.Stat(filepath.Join(domainDir, "docs", "gen", "SPEC.md")); !os.IsNotExist(err) {
		t.Fatalf("precondition: docs/gen/SPEC.md must not exist")
	}
	if vs := runCheck(t, "check_spec_md_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a domain with no committed SPEC.md, got %v", vs)
	}
}

// TestCheckSpecMDCurrent_OK_WhenFreshlyGenerated proves the positive
// control: a SPEC.md that IS exactly what gate.BuildSpecFromRows(g,
// gate.CollectSpecRows(g)) currently produces must pass clean.
func TestCheckSpecMDCurrent_OK_WhenFreshlyGenerated(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecMDFixtureModule(t)
	g := specMDFixtureGraph(domainDir)
	writeCurrentSpecMD(t, g)

	if vs := runCheck(t, "check_spec_md_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a freshly generated, unmodified SPEC.md, got %v", vs)
	}
}

// TestCheckSpecMDCurrent_MUTATION_HandEditedFiresThenClears is the mutation
// probe the task's own verification step calls for: start from a genuinely
// current SPEC.md (green), hand-edit it (simulating either a stale
// regeneration or a direct hand-edit despite the do-not-edit banner), confirm
// the check goes red, then restore the byte-identical current content and
// confirm it goes green again -- proving this is a live content comparison,
// not a one-shot flag.
func TestCheckSpecMDCurrent_MUTATION_HandEditedFiresThenClears(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecMDFixtureModule(t)
	g := specMDFixtureGraph(domainDir)
	specPath := writeCurrentSpecMD(t, g)

	// Sanity: starts green.
	if vs := runCheck(t, "check_spec_md_current", g); len(vs) != 0 {
		t.Fatalf("precondition: freshly generated SPEC.md must start clean, got %v", vs)
	}

	genuine, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read genuine SPEC.md: %v", err)
	}
	tampered := string(genuine) + "\n<!-- hand-edited: this line was never generated -->\n"
	if err := os.WriteFile(specPath, []byte(tampered), 0o644); err != nil {
		t.Fatalf("write tampered SPEC.md: %v", err)
	}

	vs := runCheck(t, "check_spec_md_current", g)
	if len(vs) == 0 {
		t.Fatalf("expected a violation for a hand-edited/stale SPEC.md, got none")
	}
	for _, v := range vs {
		if v.Check != "check_spec_md_current" {
			t.Errorf("violation Check = %q, want check_spec_md_current", v.Check)
		}
	}

	// Restore byte-identical content -- must go back to green.
	if err := os.WriteFile(specPath, genuine, 0o644); err != nil {
		t.Fatalf("restore genuine SPEC.md: %v", err)
	}
	if vs := runCheck(t, "check_spec_md_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations after restoring byte-identical current content, got %v", vs)
	}
}

// TestCheckSpecMDCurrent_FiresWhenEmptyFile proves the divergence detector
// fires for the simplest possible stale case: an empty (zero-byte) SPEC.md,
// as if the file were created as a placeholder but never actually
// regenerated.
func TestCheckSpecMDCurrent_FiresWhenEmptyFile(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecMDFixtureModule(t)
	g := specMDFixtureGraph(domainDir)
	genDir := filepath.Join(domainDir, "docs", "gen")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatalf("MkdirAll docs/gen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(genDir, "SPEC.md"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile empty SPEC.md: %v", err)
	}
	vs := runCheck(t, "check_spec_md_current", g)
	if len(vs) == 0 {
		t.Fatalf("expected a violation for an empty (never generated) SPEC.md, got none")
	}
}

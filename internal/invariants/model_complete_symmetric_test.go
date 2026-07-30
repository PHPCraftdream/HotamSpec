package invariants

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// --- infrastructureOrIgnoredReason -----------------------------------------

func TestInfrastructureOrIgnoredReason_InfrastructureMarker(t *testing.T) {
	t.Parallel()
	doc := "Recognize probes the OCR service. INFRASTRUCTURE: thin adapter wrapper, no independent behavior to prove."
	reason, marked := infrastructureOrIgnoredReason(doc)
	if !marked {
		t.Fatalf("expected marked=true for a doc carrying INFRASTRUCTURE: with a reason")
	}
	if reason != "thin adapter wrapper, no independent behavior to prove." {
		t.Fatalf("reason = %q, want the trimmed trailing text", reason)
	}
}

func TestInfrastructureOrIgnoredReason_IgnoredMarker(t *testing.T) {
	t.Parallel()
	doc := "Deprecated shim, kept for backward compatibility only. IGNORED: superseded by NewRisk, will be removed."
	reason, marked := infrastructureOrIgnoredReason(doc)
	if !marked {
		t.Fatalf("expected marked=true for a doc carrying IGNORED: with a reason")
	}
	if reason != "superseded by NewRisk, will be removed." {
		t.Fatalf("reason = %q, want the trimmed trailing text", reason)
	}
}

func TestInfrastructureOrIgnoredReason_EmptyReasonAfterMarker_NotMarked(t *testing.T) {
	t.Parallel()
	cases := []string{
		"Recognize does the thing. INFRASTRUCTURE:",
		"Recognize does the thing. INFRASTRUCTURE:   ",
		"IGNORED:",
	}
	for _, doc := range cases {
		reason, marked := infrastructureOrIgnoredReason(doc)
		if marked {
			t.Errorf("doc %q: expected marked=false (empty reason after marker is NOT a real explanation), got true (reason=%q)", doc, reason)
		}
	}
}

func TestInfrastructureOrIgnoredReason_NoMarkerPresent(t *testing.T) {
	t.Parallel()
	doc := "Recognize extracts the VIN/STS identity from a photo."
	reason, marked := infrastructureOrIgnoredReason(doc)
	if marked {
		t.Fatalf("expected marked=false for a doc with no marker, got true (reason=%q)", reason)
	}
	if reason != "" {
		t.Fatalf("reason = %q, want empty", reason)
	}
}

func TestInfrastructureOrIgnoredReason_EmptyDoc(t *testing.T) {
	t.Parallel()
	reason, marked := infrastructureOrIgnoredReason("")
	if marked {
		t.Fatalf("expected marked=false for an empty doc, got true (reason=%q)", reason)
	}
}

func TestInfrastructureOrIgnoredReason_MarkerMidSentenceStillDetected(t *testing.T) {
	t.Parallel()
	doc := "This wraps the driver. Nothing fancy here -- INFRASTRUCTURE: pure passthrough, see driver package for real logic."
	reason, marked := infrastructureOrIgnoredReason(doc)
	if !marked {
		t.Fatalf("expected marked=true for a marker embedded mid-sentence")
	}
	if reason != "pure passthrough, see driver package for real logic." {
		t.Fatalf("reason = %q, want the trimmed trailing text", reason)
	}
}

// --- check_public_surface_linked_or_marked ---------------------------------
//
// Fixture shapes mirror model_complete_test.go's own writeModelCompleteFixture
// conventions, extended with an interface + a top-level function so all
// three symbol categories (receiver method, interface method, function) can
// be exercised in one model file. publicSurfaceGraph builds the graph with
// BOTH Discipline and PublicSurfaceAuthorityLinked resolved from the
// fixture's own manifest.json (mirroring graphForDiscipline, but this
// check's trigger is PublicSurfaceAuthorityLinked, not Discipline).

// publicSurfaceModelSrc declares one struct (Risk) with one exported method
// (Validate), one exported interface (Recognizer) with one exported method
// (Recognize), and one top-level exported constructor (NewRisk) -- the
// minimal surface needed to exercise all three symbolKind categories in a
// single fixture.
const publicSurfaceModelSrc = `package model

// Risk is a small aggregate.
type Risk struct {
	Owner string
}

// Validate checks the risk is well-formed.
func (r *Risk) Validate() error { return nil }

// Recognizer is a photo recognition seam.
type Recognizer interface {
	// Recognize extracts an identity from a photo.
	Recognize(photo []byte) (string, error)
}

// NewRisk constructs a Risk from an owner name.
func NewRisk(owner string) (*Risk, error) {
	return &Risk{Owner: owner}, nil
}
`

// publicSurfaceModelMarkedSrc is publicSurfaceModelSrc's twin, except every
// symbol's doc comment carries an explicit INFRASTRUCTURE:/IGNORED: marker
// with a real reason -- the fully-compliant-via-marker fixture.
const publicSurfaceModelMarkedSrc = `package model

// Risk is a small aggregate.
type Risk struct {
	Owner string
}

// Validate checks the risk is well-formed. INFRASTRUCTURE: trivial no-op validator, no behavior to prove.
func (r *Risk) Validate() error { return nil }

// Recognizer is a photo recognition seam.
type Recognizer interface {
	// Recognize extracts an identity from a photo. IGNORED: port contract only, mocked in tests, no scenario needed.
	Recognize(photo []byte) (string, error)
}

// NewRisk constructs a Risk from an owner name. INFRASTRUCTURE: thin constructor, no independent behavior to prove.
func NewRisk(owner string) (*Risk, error) {
	return &Risk{Owner: owner}, nil
}
`

// publicSurfaceModelMarkedButEmptySrc carries markers with NO reason after
// them -- must NOT count as marked (still a violation).
const publicSurfaceModelMarkedButEmptySrc = `package model

// Risk is a small aggregate.
type Risk struct {
	Owner string
}

// Validate checks the risk is well-formed. INFRASTRUCTURE:
func (r *Risk) Validate() error { return nil }

// Recognizer is a photo recognition seam.
type Recognizer interface {
	// Recognize extracts an identity from a photo. IGNORED:
	Recognize(photo []byte) (string, error)
}

// NewRisk constructs a Risk from an owner name. INFRASTRUCTURE:
func NewRisk(owner string) (*Risk, error) {
	return &Risk{Owner: owner}, nil
}
`

// writePublicSurfaceFixture writes a model file, an optional test file (only
// when testSrc != ""), and a manifest.json carrying the given discipline and
// public_surface_authority values. Returns the domain directory.
func writePublicSurfaceFixture(t *testing.T, modelSrc, testSrc, discipline, publicSurfaceAuthority string) string {
	t.Helper()
	tmp := t.TempDir()
	writeInto := func(rel, content string) {
		full := filepath.Join(tmp, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", rel, err)
		}
	}
	writeInto("spec/model/risk.go", modelSrc)
	if testSrc != "" {
		writeInto("spec/model/risk_test.go", testSrc)
		if strings.Contains(testSrc, "hotamspec.NewScenario") {
			// Banner-stamped (unlike scenario_discipline_test.go's bare
			// scenarioRecorderStubSrc, which check_model_complete's own
			// citation-driven iteration never notices): THIS check walks the
			// FULL scanned inventory, so an un-bannered recorder stub would
			// leak into it as if it were domain-authored public surface --
			// gate.IsGeneratedOrVendoredFile excludes a file ONLY by its
			// exact banner first line, matching how a REAL vendored
			// recorder (written by `hotam vendor-recorder`) is excluded in
			// every real domain.
			writeInto("spec/hotamspec/hotamspec.go", recordervendor.Banner+scenarioRecorderStubSrc)
		}
	}
	manifest := `{"discipline": "` + discipline + `", "public_surface_authority": "` + publicSurfaceAuthority + `"}`
	writeInto("manifest.json", manifest)
	writeInto("graph.json", `{"schema_version":3}`)
	return tmp
}

// publicSurfaceGraph builds a graph with Discipline AND
// PublicSurfaceAuthorityLinked resolved from the fixture's own
// manifest.json via the real loader.LoadGraph path (the resolver
// resolvePublicSurfaceAuthorityLinked is unexported to internal/loader, so
// this -- unlike graphForDiscipline's hand-built literal -- goes through the
// full loader to populate both fields correctly), with the given
// requirements spliced in afterward.
func publicSurfaceGraph(t *testing.T, domainDir string, reqs []ontology.Requirement) *ontology.Graph {
	t.Helper()
	graphPath := filepath.Join(domainDir, "graph.json")
	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	g.Stakeholders = []ontology.Stakeholder{sA}
	g.Requirements = reqs
	return g
}

// TestCheckPublicSurfaceLinkedOrMarked_NoOpWhenNotOptedIn is the central
// backward-compatibility guarantee: a domain WITHOUT
// public_surface_authority:"linked" sees zero violations no matter how bare
// its model is -- even with discipline:"full" ALSO set (the trigger is
// deliberately NOT co-gated with discipline:full).
func TestCheckPublicSurfaceLinkedOrMarked_NoOpWhenNotOptedIn(t *testing.T) {
	t.Parallel()
	domainDir := writePublicSurfaceFixture(t, publicSurfaceModelSrc, "", "full", "")
	g := publicSurfaceGraph(t, domainDir, nil)
	if g.PublicSurfaceAuthorityLinked {
		t.Fatalf("test setup: expected PublicSurfaceAuthorityLinked=false, got true")
	}
	vs := runCheck(t, "check_public_surface_linked_or_marked", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations for a domain that has not opted into public_surface_authority:linked, got %v", vs)
	}
}

// TestCheckPublicSurfaceLinkedOrMarked_NoOpWithoutDisciplineFull proves the
// trigger does NOT require discipline:full as a co-requirement -- opting in
// with public_surface_authority:"linked" alone (discipline left at its
// default/absent) still activates the check.
func TestCheckPublicSurfaceLinkedOrMarked_NoOpWithoutDisciplineFull(t *testing.T) {
	t.Parallel()
	domainDir := writePublicSurfaceFixture(t, publicSurfaceModelSrc, "", "", loader.PublicSurfaceAuthorityLinked)
	g := publicSurfaceGraph(t, domainDir, nil)
	if g.Discipline == loader.DisciplineFull {
		t.Fatalf("test setup: expected Discipline NOT full")
	}
	if !g.PublicSurfaceAuthorityLinked {
		t.Fatalf("test setup: expected PublicSurfaceAuthorityLinked=true")
	}
	vs := runCheck(t, "check_public_surface_linked_or_marked", g)
	// Every symbol is uncited and unmarked -- must fire, proving the check
	// runs regardless of discipline:full.
	if len(vs) == 0 {
		t.Fatalf("expected violations for an opted-in domain with uncited/unmarked symbols, even without discipline:full, got none")
	}
}

// TestCheckPublicSurfaceLinkedOrMarked_UncitedUnmarked_FiresForAllThreeCategories
// proves every one of the three symbol categories (receiver method,
// interface method, top-level function) fires its own violation when
// neither cited nor marked.
func TestCheckPublicSurfaceLinkedOrMarked_UncitedUnmarked_FiresForAllThreeCategories(t *testing.T) {
	t.Parallel()
	domainDir := writePublicSurfaceFixture(t, publicSurfaceModelSrc, "", "full", loader.PublicSurfaceAuthorityLinked)
	g := publicSurfaceGraph(t, domainDir, nil)
	vs := runCheck(t, "check_public_surface_linked_or_marked", g)

	if !hasViolationFor(vs, "Risk.Validate") {
		t.Errorf("expected a violation for uncited/unmarked receiver method Risk.Validate, got %v", vs)
	}
	if !hasViolationFor(vs, "Recognizer.Recognize") {
		t.Errorf("expected a violation for uncited/unmarked interface method Recognizer.Recognize, got %v", vs)
	}
	if !hasViolationFor(vs, "NewRisk") {
		t.Errorf("expected a violation for uncited/unmarked function NewRisk, got %v", vs)
	}
	if len(vs) != 3 {
		t.Fatalf("expected exactly 3 violations (one per symbol), got %d: %v", len(vs), vs)
	}
}

// TestCheckPublicSurfaceLinkedOrMarked_CitedAndScenarioComplete_Compliant
// proves a symbol cited by a SETTLED requirement with a scenario-narrated
// verified_by test is compliant -- across all three categories.
func TestCheckPublicSurfaceLinkedOrMarked_CitedAndScenarioComplete_Compliant(t *testing.T) {
	t.Parallel()
	testSrc := `package model

import (
	"testing"

	"example.com/fixture/spec/hotamspec"
)

func TestValidate_Scenario(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-validate", "validate is a real scenario")
	r := &Risk{}
	s.Then("validate runs", r.Validate() == nil)
}

func TestRecognize_Scenario(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-recognize", "recognize is a real scenario")
	s.Then("recognize declared", true)
}

func TestNewRisk_Scenario(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-newrisk", "new risk is a real scenario")
	r, err := NewRisk("owner")
	s.Then("constructs", r != nil && err == nil)
}
`
	domainDir := writePublicSurfaceFixture(t, publicSurfaceModelSrc, testSrc, "full", loader.PublicSurfaceAuthorityLinked)

	rValidate := reqWithLinks("R-validate", "sa",
		[]string{"spec/model/risk.go:Risk.Validate"},
		[]string{"spec/model/risk_test.go:TestValidate_Scenario"})
	rValidate.Status = ontology.StatusSETTLED

	rRecognize := reqWithLinks("R-recognize", "sa",
		[]string{"spec/model/risk.go:Recognizer.Recognize"},
		[]string{"spec/model/risk_test.go:TestRecognize_Scenario"})
	rRecognize.Status = ontology.StatusSETTLED

	rNewRisk := reqWithLinks("R-newrisk", "sa",
		[]string{"spec/model/risk.go:NewRisk"},
		[]string{"spec/model/risk_test.go:TestNewRisk_Scenario"})
	rNewRisk.Status = ontology.StatusSETTLED

	g := publicSurfaceGraph(t, domainDir, []ontology.Requirement{rValidate, rRecognize, rNewRisk})
	vs := runCheck(t, "check_public_surface_linked_or_marked", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations: every symbol is cited + scenario-complete, got %v", vs)
	}
}

// TestCheckPublicSurfaceLinkedOrMarked_CitedButNoScenario_Violation proves a
// symbol cited by a SETTLED requirement WITHOUT a scenario-narrated
// verified_by test is still a violation (cited-but-incomplete, distinct
// diagnostic wording from the uncited case).
func TestCheckPublicSurfaceLinkedOrMarked_CitedButNoScenario_Violation(t *testing.T) {
	t.Parallel()
	testSrc := `package model

import "testing"

func TestValidate_Plain(t *testing.T) {
	r := &Risk{}
	if r.Validate() != nil {
		t.Fatalf("expected nil")
	}
}
`
	domainDir := writePublicSurfaceFixture(t, publicSurfaceModelSrc, testSrc, "full", loader.PublicSurfaceAuthorityLinked)

	r := reqWithLinks("R-validate", "sa",
		[]string{"spec/model/risk.go:Risk.Validate"},
		[]string{"spec/model/risk_test.go:TestValidate_Plain"})
	r.Status = ontology.StatusSETTLED

	g := publicSurfaceGraph(t, domainDir, []ontology.Requirement{r})
	vs := runCheck(t, "check_public_surface_linked_or_marked", g)
	if !hasViolationFor(vs, "Risk.Validate") {
		t.Fatalf("expected a violation for Risk.Validate (cited but no scenario), got %v", vs)
	}
	var v Violation
	for _, c := range vs {
		if c.ID == "Risk.Validate" {
			v = c
			break
		}
	}
	if !strings.Contains(v.Message, "cited by a SETTLED requirement") {
		t.Fatalf("expected the cited-but-incomplete diagnostic wording, got: %s", v.Message)
	}
}

// TestCheckPublicSurfaceLinkedOrMarked_UncitedButMarkedInfrastructure_Compliant
// proves an uncited symbol marked INFRASTRUCTURE: with a real reason is
// compliant.
func TestCheckPublicSurfaceLinkedOrMarked_UncitedButMarkedInfrastructure_Compliant(t *testing.T) {
	t.Parallel()
	domainDir := writePublicSurfaceFixture(t, publicSurfaceModelMarkedSrc, "", "full", loader.PublicSurfaceAuthorityLinked)
	g := publicSurfaceGraph(t, domainDir, nil)
	vs := runCheck(t, "check_public_surface_linked_or_marked", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations: every symbol carries an explicit INFRASTRUCTURE:/IGNORED: marker with a reason, got %v", vs)
	}
}

// TestCheckPublicSurfaceLinkedOrMarked_UncitedAndMarkedButEmptyReason_Violation
// proves a marker with NO trailing reason does NOT count as marked -- still
// a violation.
func TestCheckPublicSurfaceLinkedOrMarked_UncitedAndMarkedButEmptyReason_Violation(t *testing.T) {
	t.Parallel()
	domainDir := writePublicSurfaceFixture(t, publicSurfaceModelMarkedButEmptySrc, "", "full", loader.PublicSurfaceAuthorityLinked)
	g := publicSurfaceGraph(t, domainDir, nil)
	vs := runCheck(t, "check_public_surface_linked_or_marked", g)
	if len(vs) != 3 {
		t.Fatalf("expected 3 violations (marker present but reason empty counts as unmarked), got %d: %v", len(vs), vs)
	}
}

// TestCheckPublicSurfaceLinkedOrMarked_EmbeddedInterfaceEntrySkipped proves
// an embedded interface entry (Name == "") is never treated as this
// object's own declared method -- it must not appear as a violation ID, and
// must not prevent the check from running cleanly.
func TestCheckPublicSurfaceLinkedOrMarked_EmbeddedInterfaceEntrySkipped(t *testing.T) {
	t.Parallel()
	const src = `package model

import "io"

// Port embeds io.Closer and declares its own method.
type Port interface {
	io.Closer
	// Do does the thing. INFRASTRUCTURE: port contract only, mocked in tests.
	Do() error
}
`
	domainDir := writePublicSurfaceFixture(t, src, "", "full", loader.PublicSurfaceAuthorityLinked)
	g := publicSurfaceGraph(t, domainDir, nil)
	vs := runCheck(t, "check_public_surface_linked_or_marked", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations (Do is marked, io.Closer embedded entry is skipped entirely), got %v", vs)
	}
	for _, v := range vs {
		if strings.Contains(v.ID, "Closer") {
			t.Fatalf("embedded interface entry must never appear as a violation ID, got %v", vs)
		}
	}
}

// TestCheckPublicSurfaceLinkedOrMarked_NoAuthoredModels_NoOp proves the
// honest no-op for a domain with zero authored model files, mirroring
// check_model_complete's own "no authored models" bail.
func TestCheckPublicSurfaceLinkedOrMarked_NoAuthoredModels_NoOp(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"),
		[]byte(`{"discipline": "full", "public_surface_authority": "linked"}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "graph.json"), []byte(`{"schema_version":3}`), 0o644); err != nil {
		t.Fatalf("WriteFile graph.json: %v", err)
	}
	g := publicSurfaceGraph(t, tmp, nil)
	vs := runCheck(t, "check_public_surface_linked_or_marked", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations for a domain with zero authored model files, got %v", vs)
	}
}

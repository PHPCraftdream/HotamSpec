package query

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// writeEvidenceDomain creates a temp domain directory (non-self-hosting)
// with a go.mod and the given files (relative path -> content). Returns the
// domain dir. Mirrors internal/invariants/scenario_quality_test.go's own
// writeScenarioQualityFixture fixture-construction style: a REAL
// gate.ScanAuthoredModels-scannable spec/ tree with real Go files under
// t.TempDir(), never hand-built JSON.
func writeEvidenceDomain(t *testing.T, modulePath string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", rel, err)
		}
	}
	return root
}

// evidenceGraph builds a minimal in-memory *ontology.Graph for a non-self-
// hosting domain -- the same convention internal/invariants' test helpers use.
func evidenceGraph(domainDir string, reqs ...ontology.Requirement) *ontology.Graph {
	return &ontology.Graph{
		DomainDir:    domainDir,
		Requirements: reqs,
	}
}

func settledReqWithImpl(rid string, impl, vb []string) ontology.Requirement {
	return ontology.Requirement{
		ID:            rid,
		Claim:         "a claim for " + rid,
		Status:        ontology.StatusSETTLED,
		ImplementedBy: impl,
		VerifiedBy:    vb,
	}
}

// --- Section 1: object/method signatures -------------------------------------

// modelSrc declares a struct (Risk) with an exported method (Validate), an
// interface (Recognizer) with an exported method, and a top-level constructor
// (NewRisk) -- the minimal surface to exercise receiver-method,
// interface-method, and func resolution in one file.
const modelSrc = `package model

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

func TestBrief_EvidenceSignatures_ReceiverMethodAndFunc(t *testing.T) {
	t.Parallel()
	domainDir := writeEvidenceDomain(t, "example.com/sigmod", map[string]string{
		"spec/model/risk.go": modelSrc,
	})
	r := settledReqWithImpl("R-sig", []string{
		"spec/model/risk.go:Risk.Validate",
		"spec/model/risk.go:NewRisk",
	}, nil)
	g := evidenceGraph(domainDir, r)

	b, err := Brief(g, "R-sig", "2026-07-30")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if len(b.Signatures) != 2 {
		t.Fatalf("expected 2 signatures, got %d: %+v", len(b.Signatures), b.Signatures)
	}

	// Find the method and func signatures.
	var methodSig, funcSig *EvidenceSignature
	for i := range b.Signatures {
		if b.Signatures[i].SymbolName == "Validate" {
			methodSig = &b.Signatures[i]
		}
		if b.Signatures[i].SymbolName == "NewRisk" {
			funcSig = &b.Signatures[i]
		}
	}
	if methodSig == nil {
		t.Fatal("missing Validate signature")
	}
	if methodSig.SymbolKind != "method" {
		t.Errorf("Validate SymbolKind = %q, want method", methodSig.SymbolKind)
	}
	if methodSig.ObjectName != "Risk" {
		t.Errorf("Validate ObjectName = %q, want Risk", methodSig.ObjectName)
	}
	if methodSig.ObjectModelKind != "object" {
		t.Errorf("Validate ObjectModelKind = %q, want object", methodSig.ObjectModelKind)
	}
	if methodSig.Signature == "" {
		t.Error("Validate Signature is empty")
	}
	if methodSig.Source == "" {
		t.Error("Validate Source (provenance) is empty")
	}

	if funcSig == nil {
		t.Fatal("missing NewRisk signature")
	}
	if funcSig.SymbolKind != "func" {
		t.Errorf("NewRisk SymbolKind = %q, want func", funcSig.SymbolKind)
	}
	if funcSig.ObjectName != "" {
		t.Errorf("NewRisk ObjectName = %q, want empty (func has no owning object)", funcSig.ObjectName)
	}
}

// --- Section 2: port/mock contract ------------------------------------------

func TestBrief_EvidencePortContract(t *testing.T) {
	t.Parallel()
	domainDir := writeEvidenceDomain(t, "example.com/portmod", map[string]string{
		"spec/model/risk.go": modelSrc,
	})
	r := settledReqWithImpl("R-port", []string{
		"spec/model/risk.go:Recognizer.Recognize",
	}, nil)
	g := evidenceGraph(domainDir, r)

	b, err := Brief(g, "R-port", "2026-07-30")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if len(b.PortContracts) != 1 {
		t.Fatalf("expected 1 port contract, got %d: %+v", len(b.PortContracts), b.PortContracts)
	}
	c := b.PortContracts[0]
	if c.ObjectName != "Recognizer" {
		t.Errorf("ObjectName = %q, want Recognizer", c.ObjectName)
	}
	if c.ModelKind != "port" {
		t.Errorf("ModelKind = %q, want port", c.ModelKind)
	}
	if len(c.Methods) != 1 {
		t.Fatalf("expected 1 method in contract, got %d", len(c.Methods))
	}
	if c.Methods[0].Name != "Recognize" {
		t.Errorf("method name = %q, want Recognize", c.Methods[0].Name)
	}
}

func TestBrief_EvidenceMockContract(t *testing.T) {
	t.Parallel()
	// A mock: an unexported struct in a _test.go file whose method set is a
	// superset of an eligible port's required method names.
	const portSrc = `package model

// Recognizer is the OCR seam.
type Recognizer interface {
	Recognize(photo []byte) (string, error)
}
`
	const mockSrc = `package model

// mockRecognizer is a test double for Recognizer.
type mockRecognizer struct{}

func (m *mockRecognizer) Recognize(photo []byte) (string, error) {
	return "", nil
}
`
	domainDir := writeEvidenceDomain(t, "example.com/mockmod", map[string]string{
		"spec/model/risk.go":      portSrc,
		"spec/model/risk_test.go": mockSrc,
	})
	// The requirement cites the mock's method. Since the mock is discovered by
	// the cross-file mock-matching pass, its ModelKind will be "mock".
	r := settledReqWithImpl("R-mock", []string{
		"spec/model/risk_test.go:mockRecognizer.Recognize",
	}, nil)
	g := evidenceGraph(domainDir, r)

	b, err := Brief(g, "R-mock", "2026-07-30")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	// The mock must appear in port contracts with ModelKind "mock".
	var mockContract *EvidencePortContract
	for i := range b.PortContracts {
		if b.PortContracts[i].ModelKind == "mock" {
			mockContract = &b.PortContracts[i]
		}
	}
	if mockContract == nil {
		t.Fatalf("expected a mock contract, got %+v", b.PortContracts)
	}
	if mockContract.ObjectName != "mockRecognizer" {
		t.Errorf("ObjectName = %q, want mockRecognizer", mockContract.ObjectName)
	}
	if len(mockContract.Methods) != 1 {
		t.Fatalf("expected 1 method in mock contract, got %d", len(mockContract.Methods))
	}
}

// --- Section 3: actual Given/When/Then narrative ----------------------------

// scenarioTestSrc is a verified_by test that records a real scenario via the
// vendored hotamspec recorder. Module-path-templated (%s) so the import path
// matches the fixture's go.mod.
const scenarioTestSrc = `package model

import (
	"testing"

	hotamspec "example.com/scenmod/spec/hotamspec"
)

func TestValidate_Scenario(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-scen", "validate is a real scenario")
	s.Given("a well-formed risk")
	s.When("Validate is called")
	s.Then("no error is returned", true)
}
`

func TestBrief_EvidenceScenario_PassingNarrative(t *testing.T) {
	if testing.Short() {
		t.Skip("derives a brief through the scenario narrative pipeline; skipped in -short")
	}

	// NOT t.Parallel() -- this test runs a real go test subprocess.
	modulePath := "example.com/scenmod"
	domainDir := writeEvidenceDomain(t, modulePath, map[string]string{
		"spec/model/risk.go":          modelSrc,
		"spec/model/risk_test.go":     strings.ReplaceAll(scenarioTestSrc, "example.com/scenmod", modulePath),
		"spec/hotamspec/hotamspec.go": recordervendor.BodyForHash(),
	})
	r := settledReqWithImpl("R-scen", nil, []string{
		"spec/model/risk_test.go:TestValidate_Scenario",
	})
	g := evidenceGraph(domainDir, r)

	b, err := Brief(g, "R-scen", "2026-07-30")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if b.Scenario == nil {
		t.Fatal("expected a non-nil Scenario from the passing recorded test")
	}
	if b.Scenario.Title != "validate is a real scenario" {
		t.Errorf("Title = %q, want 'validate is a real scenario'", b.Scenario.Title)
	}
	// Must have at least Given, When, Then steps.
	var hasGiven, hasWhen, hasThen bool
	for _, step := range b.Scenario.Steps {
		switch step.Kind {
		case "given":
			hasGiven = true
		case "when":
			hasWhen = true
		case "then":
			hasThen = true
		}
	}
	if !hasGiven || !hasWhen || !hasThen {
		t.Errorf("expected at least given/when/then steps, got: %+v", b.Scenario.Steps)
	}
	if b.Scenario.Source == "" {
		t.Error("Scenario Source (provenance) is empty")
	}
}

func TestBrief_EvidenceScenario_HonestAbsentWhenNoScenario(t *testing.T) {
	t.Parallel()
	domainDir := writeEvidenceDomain(t, "example.com/noscenmod", map[string]string{
		"spec/model/risk.go": modelSrc,
	})
	// A requirement with implemented_by but NO verified_by at all.
	r := settledReqWithImpl("R-noscen", []string{
		"spec/model/risk.go:Risk.Validate",
	}, nil)
	g := evidenceGraph(domainDir, r)

	b, err := Brief(g, "R-noscen", "2026-07-30")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if b.Scenario != nil {
		t.Errorf("Scenario must be nil (honest-absent) when no verified_by entry carries a scenario, got: %+v", b.Scenario)
	}
}

// --- Section 4: related entity/process ---------------------------------------

func TestBrief_EvidenceEntityProcess_ReverseMatch(t *testing.T) {
	t.Parallel()
	domainDir := writeEvidenceDomain(t, "example.com/entitymod", map[string]string{
		"spec/model/risk.go": modelSrc,
	})
	r := settledReqWithImpl("R-entity", []string{
		"spec/model/risk.go:Risk.Validate",
	}, nil)
	g := evidenceGraph(domainDir, r)

	// Add an EntityType whose ModelSymbol names the same file as the
	// requirement's implemented_by citation.
	g.EntityTypes = []ontology.EntityType{
		{
			Slug:        "risk",
			Description: "a risk entity",
			ModelSymbol: "spec/model/risk.go:Risk",
		},
	}
	// Add a Process that drives the risk entity.
	g.Processes = []ontology.Process{
		{
			ID:             "P-risk-mgmt",
			DrivesEntities: []string{"risk"},
			Why:            "drives the risk lifecycle",
		},
	}

	b, err := Brief(g, "R-entity", "2026-07-30")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if b.EntityProcess == nil {
		t.Fatal("expected a non-nil EntityProcess from the reverse-match")
	}
	if len(b.EntityProcess.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d: %+v", len(b.EntityProcess.Entities), b.EntityProcess.Entities)
	}
	e := b.EntityProcess.Entities[0]
	if e.EntityTypeSlug != "risk" {
		t.Errorf("EntityTypeSlug = %q, want risk", e.EntityTypeSlug)
	}
	if len(e.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(e.Processes))
	}
	if e.Processes[0].ID != "P-risk-mgmt" {
		t.Errorf("Process ID = %q, want P-risk-mgmt", e.Processes[0].ID)
	}
}

// --- Honest-absent: nothing matches ------------------------------------------

func TestBrief_EvidenceNothingMatches_AllAbsent(t *testing.T) {
	t.Parallel()
	// A requirement with no implemented_by, no verified_by, no entity types.
	domainDir := writeEvidenceDomain(t, "example.com/emptymod", map[string]string{
		"spec/model/risk.go": modelSrc,
	})
	r := settledReqWithImpl("R-empty", nil, nil)
	g := evidenceGraph(domainDir, r)

	b, err := Brief(g, "R-empty", "2026-07-30")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if len(b.Signatures) != 0 {
		t.Errorf("Signatures should be empty, got %d", len(b.Signatures))
	}
	if len(b.PortContracts) != 0 {
		t.Errorf("PortContracts should be empty, got %d", len(b.PortContracts))
	}
	if b.Scenario != nil {
		t.Errorf("Scenario should be nil, got %+v", b.Scenario)
	}
	if b.EntityProcess != nil {
		t.Errorf("EntityProcess should be nil, got %+v", b.EntityProcess)
	}
}

// --- Conflict/Assumption never populate evidence fields ----------------------

func TestBrief_ConflictRendersNoEvidence(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	b, err := Brief(g, "C-ab", "2026-07-13")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if len(b.Signatures) != 0 {
		t.Errorf("Conflict brief must have no Signatures, got %d", len(b.Signatures))
	}
	if len(b.PortContracts) != 0 {
		t.Errorf("Conflict brief must have no PortContracts, got %d", len(b.PortContracts))
	}
	if b.Scenario != nil {
		t.Errorf("Conflict brief must have no Scenario")
	}
	if b.EntityProcess != nil {
		t.Errorf("Conflict brief must have no EntityProcess")
	}
	// JSON must not carry any of the new keys for a Conflict brief.
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)
	for _, key := range []string{`"signatures"`, `"port_contracts"`, `"scenario"`, `"entity_process"`} {
		if strings.Contains(raw, key) {
			t.Errorf("Conflict brief JSON must not contain %s, got: %s", key, raw)
		}
	}
}

func TestBrief_AssumptionRendersNoEvidence(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	b, err := Brief(g, "A-shared", "2026-07-13")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if len(b.Signatures) != 0 {
		t.Errorf("Assumption brief must have no Signatures, got %d", len(b.Signatures))
	}
	if b.Scenario != nil {
		t.Errorf("Assumption brief must have no Scenario")
	}
	if b.EntityProcess != nil {
		t.Errorf("Assumption brief must have no EntityProcess")
	}
}

// --- Stale/unresolvable citation omitted (not an error) ----------------------

func TestBrief_EvidenceStaleCitationOmitted(t *testing.T) {
	t.Parallel()
	domainDir := writeEvidenceDomain(t, "example.com/stalemod", map[string]string{
		"spec/model/risk.go": modelSrc,
	})
	// A citation to a symbol that does not exist in the scanned file.
	r := settledReqWithImpl("R-stale", []string{
		"spec/model/risk.go:Risk.NoSuchMethod",
	}, nil)
	g := evidenceGraph(domainDir, r)

	b, err := Brief(g, "R-stale", "2026-07-30")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if len(b.Signatures) != 0 {
		t.Errorf("stale citation must produce no signature, got %d: %+v", len(b.Signatures), b.Signatures)
	}
}

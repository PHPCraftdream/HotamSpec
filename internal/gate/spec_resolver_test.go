package gate

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSpecFixture writes a single Go source file under tmp/spec/<rel> and
// returns tmp (the domain root -- SpecRoot(domainDir) convention: entries
// are domain-relative paths already prefixed with "spec/").
func writeSpecFixture(t *testing.T, rel, content string) string {
	t.Helper()
	tmp := t.TempDir()
	full := filepath.Join(tmp, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return tmp
}

const riskModelSrc = `package model

type Risk struct {
	Owner string
}

func NewRisk(owner string) (*Risk, error) {
	return &Risk{Owner: owner}, nil
}

func (r *Risk) Validate() error {
	return nil
}

func (r Risk) Owner2() string {
	return r.Owner
}
`

// --- ResolveSpecSymbol ---------------------------------------------------

func TestResolveSpecSymbol_TopLevelFunc_Found(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk.go", riskModelSrc)
	res, err := ResolveSpecSymbol(SpecRoot(domainDir), "spec/model/risk.go", "NewRisk")
	if err != nil {
		t.Fatalf("ResolveSpecSymbol: %v", err)
	}
	if !res.Found() || res.Kind != SpecSymbolFunc {
		t.Fatalf("expected Found func, got %+v", res)
	}
}

// TestResolveSpecSymbol_Method_BareNameMatchesAnyReceiver proves the
// documented convention: a bare "Validate" matches the method regardless of
// receiver type (existing gate.collectTestFuncNames-style scanners skip
// methods entirely -- fn.Recv != nil -- but implemented_by MUST resolve
// methods, since a lot of authored behavior lives on model methods).
func TestResolveSpecSymbol_Method_BareNameMatchesAnyReceiver(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk.go", riskModelSrc)
	res, err := ResolveSpecSymbol(SpecRoot(domainDir), "spec/model/risk.go", "Validate")
	if err != nil {
		t.Fatalf("ResolveSpecSymbol: %v", err)
	}
	if !res.Found() || res.Kind != SpecSymbolMethod {
		t.Fatalf("expected Found method, got %+v", res)
	}
}

// TestResolveSpecSymbol_Method_QualifiedNameMatchesReceiverType proves the
// "Type.Method" qualified form resolves against the receiver's base type
// name (pointer receiver unwrapped).
func TestResolveSpecSymbol_Method_QualifiedNameMatchesReceiverType(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk.go", riskModelSrc)
	res, err := ResolveSpecSymbol(SpecRoot(domainDir), "spec/model/risk.go", "Risk.Validate")
	if err != nil {
		t.Fatalf("ResolveSpecSymbol: %v", err)
	}
	if !res.Found() || res.Kind != SpecSymbolMethod {
		t.Fatalf("expected Found method via qualified name, got %+v", res)
	}
	// Value-receiver method also qualifies under its base type name.
	res2, err := ResolveSpecSymbol(SpecRoot(domainDir), "spec/model/risk.go", "Risk.Owner2")
	if err != nil {
		t.Fatalf("ResolveSpecSymbol: %v", err)
	}
	if !res2.Found() || res2.Kind != SpecSymbolMethod {
		t.Fatalf("expected Found method via qualified name (value receiver), got %+v", res2)
	}
}

func TestResolveSpecSymbol_Method_QualifiedNameWrongTypeNotFound(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk.go", riskModelSrc)
	res, err := ResolveSpecSymbol(SpecRoot(domainDir), "spec/model/risk.go", "Wrong.Validate")
	if err != nil {
		t.Fatalf("ResolveSpecSymbol: %v", err)
	}
	if res.Found() {
		t.Fatalf("expected NOT found for mismatched receiver type, got %+v", res)
	}
}

func TestResolveSpecSymbol_Type_Found(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk.go", riskModelSrc)
	res, err := ResolveSpecSymbol(SpecRoot(domainDir), "spec/model/risk.go", "Risk")
	if err != nil {
		t.Fatalf("ResolveSpecSymbol: %v", err)
	}
	if !res.Found() || res.Kind != SpecSymbolType {
		t.Fatalf("expected Found type, got %+v", res)
	}
}

// TestResolveSpecSymbol_MUTATION_RemoveSymbolFlipsToNotFound is the
// break->fix mutation proof for staleness: remove the symbol from the file
// (simulating a model rename that orphans the implemented_by reference) and
// confirm resolution flips from Found to NOT-found, then restore and confirm
// it flips back.
func TestResolveSpecSymbol_MUTATION_RemoveSymbolFlipsToNotFound(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk.go", riskModelSrc)

	// Intact: found.
	res, err := ResolveSpecSymbol(SpecRoot(domainDir), "spec/model/risk.go", "NewRisk")
	if err != nil || !res.Found() {
		t.Fatalf("expected Found before mutation, got %+v err=%v", res, err)
	}

	// Mutate: rewrite the file WITHOUT NewRisk (simulates a rename/deletion
	// orphaning the implemented_by reference).
	mutated := `package model

type Risk struct {
	Owner string
}

func (r *Risk) Validate() error {
	return nil
}
`
	path := filepath.Join(domainDir, "spec", "model", "risk.go")
	if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
		t.Fatalf("WriteFile mutation: %v", err)
	}
	res2, err := ResolveSpecSymbol(SpecRoot(domainDir), "spec/model/risk.go", "NewRisk")
	if err != nil {
		t.Fatalf("ResolveSpecSymbol after mutation: %v", err)
	}
	if res2.Found() {
		t.Fatalf("expected NOT found after removing NewRisk, got %+v", res2)
	}

	// Fix: restore the original content -- must resolve again.
	if err := os.WriteFile(path, []byte(riskModelSrc), 0o644); err != nil {
		t.Fatalf("WriteFile restore: %v", err)
	}
	res3, err := ResolveSpecSymbol(SpecRoot(domainDir), "spec/model/risk.go", "NewRisk")
	if err != nil || !res3.Found() {
		t.Fatalf("expected Found after restoring symbol, got %+v err=%v", res3, err)
	}
}

// --- ResolveSpecTest -------------------------------------------------------

const riskTestGoodSrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	r, err := NewRisk("")
	if err == nil {
		t.Fatalf("expected error for missing owner, got risk=%v", r)
	}
}
`

const riskTestVacuousSrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	t.Log("no structural atom")
}
`

const riskTestEmptySrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
}
`

const riskTestSkipSrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	t.Skip("not ready")
	if 1 == 2 {
		t.Fatal("unreachable")
	}
}
`

const riskTestConditionalSkipSrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("slow test")
	}
	r, err := NewRisk("")
	if err == nil {
		t.Fatalf("expected error, got %v", r)
	}
}
`

func TestResolveSpecTest_Found_HasTeeth_NoSkip(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk_test.go", riskTestGoodSrc)
	res, err := ResolveSpecTest(SpecRoot(domainDir), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if !res.Found {
		t.Fatalf("expected Found, got %+v", res)
	}
	if !res.HasTeeth {
		t.Fatalf("expected HasTeeth=true for a real t.Fatalf assertion, got %+v", res)
	}
	if res.HasSkip {
		t.Fatalf("expected HasSkip=false, got %+v", res)
	}
}

func TestResolveSpecTest_NotFound_WrongName(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk_test.go", riskTestGoodSrc)
	res, err := ResolveSpecTest(SpecRoot(domainDir), "spec/model/risk_test.go", "TestDoesNotExist")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if res.Found {
		t.Fatalf("expected NOT found, got %+v", res)
	}
}

// TestResolveSpecTest_MUTATION_VacuousBodyFlipsTeeth is the break->fix proof
// for the anti-vacuousness detector: a t.Log-only body has HasTeeth=false;
// replacing it with a real assertion flips HasTeeth to true.
func TestResolveSpecTest_MUTATION_VacuousBodyFlipsTeeth(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk_test.go", riskTestVacuousSrc)
	path := filepath.Join(domainDir, "spec", "model", "risk_test.go")

	res, err := ResolveSpecTest(SpecRoot(domainDir), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if !res.Found {
		t.Fatalf("expected Found, got %+v", res)
	}
	if res.HasTeeth {
		t.Fatalf("BROKEN CASE: expected HasTeeth=false for t.Log-only body, got %+v", res)
	}

	// Fix: rewrite with a real assertion.
	if err := os.WriteFile(path, []byte(riskTestGoodSrc), 0o644); err != nil {
		t.Fatalf("WriteFile fix: %v", err)
	}
	res2, err := ResolveSpecTest(SpecRoot(domainDir), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest after fix: %v", err)
	}
	if !res2.HasTeeth {
		t.Fatalf("FIXED CASE: expected HasTeeth=true after adding a real assertion, got %+v", res2)
	}
}

func TestResolveSpecTest_EmptyBodyHasNoTeeth(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk_test.go", riskTestEmptySrc)
	res, err := ResolveSpecTest(SpecRoot(domainDir), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if res.HasTeeth {
		t.Fatalf("expected HasTeeth=false for an empty body, got %+v", res)
	}
}

// TestResolveSpecTest_MUTATION_TopLevelSkipFlipsHasSkip is the break->fix
// proof for the no-skip prohibition: an unconditional top-level t.Skip sets
// HasSkip=true; removing it (or nesting it under a runtime condition) sets
// HasSkip=false.
func TestResolveSpecTest_MUTATION_TopLevelSkipFlipsHasSkip(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk_test.go", riskTestSkipSrc)
	path := filepath.Join(domainDir, "spec", "model", "risk_test.go")

	res, err := ResolveSpecTest(SpecRoot(domainDir), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if !res.Found {
		t.Fatalf("expected Found, got %+v", res)
	}
	if !res.HasSkip {
		t.Fatalf("BROKEN CASE: expected HasSkip=true for unconditional top-level t.Skip, got %+v", res)
	}

	// Fix: rewrite without a top-level skip (a conditional skip is fine).
	if err := os.WriteFile(path, []byte(riskTestConditionalSkipSrc), 0o644); err != nil {
		t.Fatalf("WriteFile fix: %v", err)
	}
	res2, err := ResolveSpecTest(SpecRoot(domainDir), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest after fix: %v", err)
	}
	if res2.HasSkip {
		t.Fatalf("FIXED CASE: expected HasSkip=false for a runtime-conditional skip, got %+v", res2)
	}
	if !res2.HasTeeth {
		t.Fatalf("FIXED CASE: expected HasTeeth=true (real assertion present), got %+v", res2)
	}
}

// --- ParseFileColonSymbol ---------------------------------------------------

func TestParseFileColonSymbol_OK(t *testing.T) {
	t.Parallel()
	file, symbol, ok := ParseFileColonSymbol("spec/model/risk.go:NewRisk")
	if !ok || file != "spec/model/risk.go" || symbol != "NewRisk" {
		t.Fatalf("got file=%q symbol=%q ok=%v", file, symbol, ok)
	}
}

func TestParseFileColonSymbol_QualifiedMethod(t *testing.T) {
	t.Parallel()
	file, symbol, ok := ParseFileColonSymbol("spec/model/risk.go:Risk.Validate")
	if !ok || file != "spec/model/risk.go" || symbol != "Risk.Validate" {
		t.Fatalf("got file=%q symbol=%q ok=%v", file, symbol, ok)
	}
}

func TestParseFileColonSymbol_MalformedEntries(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"", "noColonAtAll", "onlyfile:", ":onlysymbol"} {
		if _, _, ok := ParseFileColonSymbol(bad); ok {
			t.Fatalf("expected ok=false for malformed entry %q", bad)
		}
	}
}

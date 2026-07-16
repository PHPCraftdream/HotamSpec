package gate

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSpecFixture writes a single Go source file under tmp/spec/<rel> and
// returns tmp (the domain root -- SpecRoot(domainDir, false) convention: entries
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
	res, err := ResolveSpecSymbol(SpecRoot(domainDir, false), "spec/model/risk.go", "NewRisk")
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
	res, err := ResolveSpecSymbol(SpecRoot(domainDir, false), "spec/model/risk.go", "Validate")
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
	res, err := ResolveSpecSymbol(SpecRoot(domainDir, false), "spec/model/risk.go", "Risk.Validate")
	if err != nil {
		t.Fatalf("ResolveSpecSymbol: %v", err)
	}
	if !res.Found() || res.Kind != SpecSymbolMethod {
		t.Fatalf("expected Found method via qualified name, got %+v", res)
	}
	// Value-receiver method also qualifies under its base type name.
	res2, err := ResolveSpecSymbol(SpecRoot(domainDir, false), "spec/model/risk.go", "Risk.Owner2")
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
	res, err := ResolveSpecSymbol(SpecRoot(domainDir, false), "spec/model/risk.go", "Wrong.Validate")
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
	res, err := ResolveSpecSymbol(SpecRoot(domainDir, false), "spec/model/risk.go", "Risk")
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
	res, err := ResolveSpecSymbol(SpecRoot(domainDir, false), "spec/model/risk.go", "NewRisk")
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
	res2, err := ResolveSpecSymbol(SpecRoot(domainDir, false), "spec/model/risk.go", "NewRisk")
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
	res3, err := ResolveSpecSymbol(SpecRoot(domainDir, false), "spec/model/risk.go", "NewRisk")
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
	res, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
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
	res, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestDoesNotExist")
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

	res, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
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
	res2, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
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
	res, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
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

	res, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
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
	res2, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
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

// --- SpecRoot / self-hosting -------------------------------------------

// writeSelfHostingEngineFixture builds a temp "engine repo": go.mod at the
// root, an engine source file + engine test file directly under internal/
// (mirroring how the REAL engine's internal/ontology/lifecycle.go +
// lifecycle_test.go sit relative to HotamSpec's own go.mod), and a domain
// directory nested at domains/<domainName>/ two levels below the root --
// matching domains/hotam-spec-self's real layout, though SpecRoot's engine
// walk does not hardcode that distance. Returns the domain directory (what
// g.DomainDir would be for that domain).
func writeSelfHostingEngineFixture(t *testing.T, domainName string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/engine\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	internalDir := filepath.Join(root, "internal")
	if err := os.MkdirAll(internalDir, 0o755); err != nil {
		t.Fatalf("MkdirAll internal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(internalDir, "foo.go"), []byte(engineFooSrc), 0o644); err != nil {
		t.Fatalf("WriteFile foo.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(internalDir, "foo_test.go"), []byte(engineFooTestSrc), 0o644); err != nil {
		t.Fatalf("WriteFile foo_test.go: %v", err)
	}
	domainDir := filepath.Join(root, "domains", domainName)
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("MkdirAll domainDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(`{"self_hosting": true}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "graph.json"), []byte(`{"schema_version":3}`), 0o644); err != nil {
		t.Fatalf("WriteFile graph.json: %v", err)
	}
	return domainDir
}

const engineFooSrc = `package internal

func Bar() int {
	return 42
}
`

const engineFooTestSrc = `package internal

import "testing"

func TestBar(t *testing.T) {
	if Bar() != 42 {
		t.Fatalf("expected 42")
	}
}
`

// TestSpecRoot_SelfHosting_ResolvesAgainstEngineRoot proves the self-hosting
// recursion (PLAN-authored-spec-discipline.md §9): implemented_by
// "internal/foo.go:Bar" and verified_by "internal/foo_test.go:TestBar"
// resolve when SpecRoot is given selfHosting=true and a domainDir nested
// under the engine root, because SpecRoot walks UP from domainDir to the
// nearest go.mod rather than joining onto domainDir directly.
func TestSpecRoot_SelfHosting_ResolvesAgainstEngineRoot(t *testing.T) {
	t.Parallel()
	domainDir := writeSelfHostingEngineFixture(t, "hotam-spec-self")

	root := SpecRoot(domainDir, true)

	symRes, err := ResolveSpecSymbol(root, "internal/foo.go", "Bar")
	if err != nil {
		t.Fatalf("ResolveSpecSymbol: %v", err)
	}
	if !symRes.Found() {
		t.Fatalf("expected implemented_by internal/foo.go:Bar to resolve against the engine root, got %+v", symRes)
	}

	testRes, err := ResolveSpecTest(root, "internal/foo_test.go", "TestBar")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if !testRes.Found {
		t.Fatalf("expected verified_by internal/foo_test.go:TestBar to resolve against the engine root, got %+v", testRes)
	}
}

// TestSpecRoot_SelfHosting_MUTATION_RemoveSymbolFlipsToNotFound is the
// break->fix mutation proof for the self-hosting path: remove Bar from the
// engine source file (simulating a rename/deletion of the real engine
// symbol an implemented_by entry names) and confirm resolution flips from
// Found to NOT-found, then restore and confirm it flips back. Proves the
// self-hosting resolver is not accidentally succeeding for an unrelated
// reason (e.g. resolving to some other file by chance).
func TestSpecRoot_SelfHosting_MUTATION_RemoveSymbolFlipsToNotFound(t *testing.T) {
	t.Parallel()
	domainDir := writeSelfHostingEngineFixture(t, "hotam-spec-self")
	root := SpecRoot(domainDir, true)
	fooPath := filepath.Join(root, "internal", "foo.go")

	res, err := ResolveSpecSymbol(root, "internal/foo.go", "Bar")
	if err != nil || !res.Found() {
		t.Fatalf("expected Found before mutation, got %+v err=%v", res, err)
	}

	mutated := `package internal

func Baz() int {
	return 0
}
`
	if err := os.WriteFile(fooPath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("WriteFile mutation: %v", err)
	}
	res2, err := ResolveSpecSymbol(root, "internal/foo.go", "Bar")
	if err != nil {
		t.Fatalf("ResolveSpecSymbol after mutation: %v", err)
	}
	if res2.Found() {
		t.Fatalf("expected NOT found after removing Bar, got %+v", res2)
	}

	if err := os.WriteFile(fooPath, []byte(engineFooSrc), 0o644); err != nil {
		t.Fatalf("WriteFile restore: %v", err)
	}
	res3, err := ResolveSpecSymbol(root, "internal/foo.go", "Bar")
	if err != nil || !res3.Found() {
		t.Fatalf("expected Found after restoring symbol, got %+v err=%v", res3, err)
	}
}

// TestSpecRoot_NonSelfHosting_StillResolvesAgainstDomainDir is the
// regression proof: an ordinary (non-self-hosting) domain's SpecRoot MUST
// still return domainDir itself, unaffected by the self-hosting engine-root
// walk added above -- i.e. the pilot domain (prat, spec/model/risk.go)
// keeps resolving exactly as before.
func TestSpecRoot_NonSelfHosting_StillResolvesAgainstDomainDir(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk.go", riskModelSrc)

	if got := SpecRoot(domainDir, false); got != domainDir {
		t.Fatalf("expected SpecRoot(domainDir, false) == domainDir, got %q want %q", got, domainDir)
	}

	res, err := ResolveSpecSymbol(SpecRoot(domainDir, false), "spec/model/risk.go", "NewRisk")
	if err != nil {
		t.Fatalf("ResolveSpecSymbol: %v", err)
	}
	if !res.Found() {
		t.Fatalf("expected non-self-hosting resolution against domainDir to still succeed, got %+v", res)
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

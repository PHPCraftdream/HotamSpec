package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"
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

// TestEngineRoot_NoGoModAboveDomainDir_FallsBackToCWD proves the third leg
// of the fix for the #232/#226 regression: cmd/hotam's test fixtures (see
// cmd/hotam/main_test.go's copySelfDomainUnderRoot) copy ONLY graph.json and
// manifest.json for the self-hosting domain into an isolated t.TempDir(),
// with no go.mod and no internal/ tree anywhere above that copy. Walking up
// from such a domainDir alone can never find go.mod, so engineRoot must fall
// back to walking up from the process's current working directory instead --
// `go test` (like the built hotam binary) always runs from inside the real
// engine module, so that walk lands on the real repository root, letting
// internal/-relative implemented_by/verified_by entries resolve against the
// real engine tree even though domainDir itself is an unrelated island.
func TestEngineRoot_NoGoModAboveDomainDir_FallsBackToCWD(t *testing.T) {
	// Intentionally NOT t.Parallel(): this test depends on process-wide
	// os.Getwd(), and running in parallel with other tests that might change
	// the working directory could make the assertion flaky.
	isolatedRoot := t.TempDir()
	domainDir := filepath.Join(isolatedRoot, "domains", "hotam-spec-self")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("MkdirAll domainDir: %v", err)
	}
	// Confirm the fixture premise: no go.mod anywhere above domainDir inside
	// the isolated tree (t.TempDir() roots are never inside a Go module).
	if _, ok := walkUpToGoMod(domainDir); ok {
		t.Fatalf("test fixture invalid: expected no go.mod walking up from %s", domainDir)
	}

	root, ok := engineRoot(domainDir)
	if !ok {
		t.Fatalf("expected engineRoot to fall back to CWD-walk and succeed, got ok=false")
	}
	if root == domainDir {
		t.Fatalf("expected engineRoot to resolve to the real engine root (via CWD), not degrade to domainDir=%s", domainDir)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("expected resolved root %s to contain go.mod: %v", root, err)
	}

	// The resolved root must be the SAME one a direct CWD-walk finds --
	// proving engineRoot's fallback is exactly the CWD walk, not some other
	// coincidental directory.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	wantRoot, ok := walkUpToGoMod(cwd)
	if !ok {
		t.Fatalf("test environment invalid: expected go.mod walking up from CWD %s", cwd)
	}
	if root != wantRoot {
		t.Fatalf("engineRoot resolved to %s, want CWD-walk result %s", root, wantRoot)
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

// --- EntryWithinSpecScope (F2: @fh Probe D remediation) --------------------

// TestEntryWithinSpecScope_NonSelfHosting_LegitimateSpecPath_OK proves the
// legitimate case is NOT broken by the scope gate: an ordinary domain-
// relative "spec/..." path (the pilot prat domain's real shape) is accepted.
func TestEntryWithinSpecScope_NonSelfHosting_LegitimateSpecPath_OK(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk.go", riskModelSrc)
	ok, reason := EntryWithinSpecScope(SpecRoot(domainDir, false), "spec/model/risk.go", false)
	if !ok {
		t.Fatalf("expected legitimate spec/ path to be within scope, got reason=%q", reason)
	}
}

// TestEntryWithinSpecScope_NonSelfHosting_ParentTraversal_Rejected is the
// direct reproduction of @fh Probe D: a non-self-hosting domain citing
// "../HotamSpec/internal/ontology/lifecycle.go:Lifecycle" (or any "../"
// escape) must be rejected BEFORE resolution is even attempted.
func TestEntryWithinSpecScope_NonSelfHosting_ParentTraversal_Rejected(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk.go", riskModelSrc)
	root := SpecRoot(domainDir, false)
	for _, bad := range []string{
		"../HotamSpec/internal/ontology/lifecycle.go",
		"../../etc/passwd",
		"..",
		"spec/../../../internal/ontology/lifecycle.go",
	} {
		ok, reason := EntryWithinSpecScope(root, bad, false)
		if ok {
			t.Fatalf("expected traversal path %q to be rejected, got ok=true", bad)
		}
		if reason == "" {
			t.Fatalf("expected a non-empty reason for rejecting %q", bad)
		}
	}
}

// TestEntryWithinSpecScope_NonSelfHosting_OutsideSpecPrefix_Rejected proves a
// non-self-hosting domain cannot cite a path that never traverses via ".."
// but also never sits under "spec/" -- e.g. a sibling domain's own spec/
// tree reached by a relative path that does not start with "spec/", or an
// absolute path. Neither shape is a legitimate authored-spec reference for
// an ordinary domain.
func TestEntryWithinSpecScope_NonSelfHosting_OutsideSpecPrefix_Rejected(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk.go", riskModelSrc)
	root := SpecRoot(domainDir, false)
	for _, bad := range []string{
		"internal/ontology/lifecycle.go",
		"cmd/hotam/main.go",
		"README.md",
	} {
		ok, reason := EntryWithinSpecScope(root, bad, false)
		if ok {
			t.Fatalf("expected non-spec/-prefixed path %q to be rejected, got ok=true", bad)
		}
		if reason == "" {
			t.Fatalf("expected a non-empty reason for rejecting %q", bad)
		}
	}
}

// TestEntryWithinSpecScope_SelfHosting_LegitimateEnginePath_OK proves the
// self-hosting legitimate case is not broken: an "internal/..." path (the
// real hotam-spec-self recursion shape, PLAN-authored-spec-discipline.md
// §9) is accepted when selfHosting=true, even though it does NOT start with
// "spec/" -- the spec/ prefix requirement is only for non-self-hosting
// domains.
func TestEntryWithinSpecScope_SelfHosting_LegitimateEnginePath_OK(t *testing.T) {
	t.Parallel()
	domainDir := writeSelfHostingEngineFixture(t, "hotam-spec-self")
	root := SpecRoot(domainDir, true)
	ok, reason := EntryWithinSpecScope(root, "internal/foo.go", true)
	if !ok {
		t.Fatalf("expected legitimate internal/ engine path to be within scope, got reason=%q", reason)
	}
}

// TestEntryWithinSpecScope_SelfHosting_ParentTraversal_Rejected proves the
// self-hosting path is equally hardened: a "../" escape out of the engine
// repository root is rejected even though self-hosting has no required
// "spec/" prefix.
func TestEntryWithinSpecScope_SelfHosting_ParentTraversal_Rejected(t *testing.T) {
	t.Parallel()
	domainDir := writeSelfHostingEngineFixture(t, "hotam-spec-self")
	root := SpecRoot(domainDir, true)
	for _, bad := range []string{
		"../outside-the-engine-repo/evil.go",
		"internal/../../outside/evil.go",
	} {
		ok, reason := EntryWithinSpecScope(root, bad, true)
		if ok {
			t.Fatalf("expected self-hosting traversal path %q to be rejected, got ok=true", bad)
		}
		if reason == "" {
			t.Fatalf("expected a non-empty reason for rejecting %q", bad)
		}
	}
}

// TestEntryWithinSpecScope_AbsolutePath_Rejected proves an absolute path
// (which would bypass the domain root entirely) is rejected for both
// self-hosting and non-self-hosting domains.
func TestEntryWithinSpecScope_AbsolutePath_Rejected(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk.go", riskModelSrc)
	root := SpecRoot(domainDir, false)
	abs := filepath.ToSlash(filepath.Join(root, "..", "..", "etc", "passwd"))
	if ok, _ := EntryWithinSpecScope(root, "/"+abs, false); ok {
		t.Fatalf("expected absolute path to be rejected")
	}
	if ok, _ := EntryWithinSpecScope(root, "C:/Windows/system32", false); ok {
		t.Fatalf("expected drive-letter absolute path to be rejected")
	}
}

// --- testBodyHasTeeth / testBodyHasTopLevelSkip hardening (F3: @fh Probe A) -

// TestResolveSpecTest_EmptyLoopIsNotTeeth is the direct reproduction of @fh
// Probe A: a test body containing ONLY an empty loop (`for i := 0; i < 0;
// i++ {}`) with no real assertion call inside it must NOT count as teeth --
// the old detector treated any bare for/if/switch as "teeth" by shape alone,
// letting an always-green, always-vacuous test through.
func TestResolveSpecTest_EmptyLoopIsNotTeeth(t *testing.T) {
	t.Parallel()
	const src = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	for i := 0; i < 0; i++ {
	}
}
`
	domainDir := writeSpecFixture(t, "spec/model/risk_test.go", src)
	res, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if res.HasTeeth {
		t.Fatalf("BROKEN CASE: expected HasTeeth=false for an empty loop with no assertion, got %+v", res)
	}
}

// TestResolveSpecTest_BareIfWithNoAssertIsNotTeeth proves the same hardening
// for a bare `if` branch: a condition with an empty (or assert-free) body is
// not teeth merely because it is an `if` statement.
func TestResolveSpecTest_BareIfWithNoAssertIsNotTeeth(t *testing.T) {
	t.Parallel()
	const src = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	x := 1
	if x == 1 {
		x = 2
	}
	_ = x
}
`
	domainDir := writeSpecFixture(t, "spec/model/risk_test.go", src)
	res, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if res.HasTeeth {
		t.Fatalf("BROKEN CASE: expected HasTeeth=false for an if-branch with no assertion, got %+v", res)
	}
}

// TestResolveSpecTest_MUTATION_EmptyLoopToRealAssertFlipsTeeth is the
// break->fix mutation proof: start from the Probe A shape (empty loop, no
// teeth), then add a real t.Fatalf inside the loop body -- teeth must flip
// to true.
func TestResolveSpecTest_MUTATION_EmptyLoopToRealAssertFlipsTeeth(t *testing.T) {
	t.Parallel()
	const vacuous = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	for i := 0; i < 0; i++ {
	}
}
`
	const withAssert = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	for i := 0; i < 1; i++ {
		if i != 0 {
			t.Fatalf("unexpected i=%d", i)
		}
	}
}
`
	domainDir := writeSpecFixture(t, "spec/model/risk_test.go", vacuous)
	path := filepath.Join(domainDir, "spec", "model", "risk_test.go")

	res, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if res.HasTeeth {
		t.Fatalf("BROKEN CASE: expected HasTeeth=false before fix, got %+v", res)
	}

	if err := os.WriteFile(path, []byte(withAssert), 0o644); err != nil {
		t.Fatalf("WriteFile fix: %v", err)
	}
	res2, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest after fix: %v", err)
	}
	if !res2.HasTeeth {
		t.Fatalf("FIXED CASE: expected HasTeeth=true once a real assertion is nested inside the loop, got %+v", res2)
	}
}

// TestResolveSpecTest_IfTrueSkip_IsUnconditional is the direct reproduction
// of @fh Probe A's second half: `if true { t.Skip(...) }` must be treated as
// an unconditional skip, exactly like a bare top-level t.Skip -- the old
// detector only looked at direct top-level statements and missed a skip
// dressed up behind a constant-true condition.
func TestResolveSpecTest_IfTrueSkip_IsUnconditional(t *testing.T) {
	t.Parallel()
	const src = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	if true {
		t.Skip("dressed-up unconditional skip")
	}
	r, err := NewRisk("")
	if err == nil {
		t.Fatalf("expected error, got %v", r)
	}
}
`
	domainDir := writeSpecFixture(t, "spec/model/risk_test.go", src)
	res, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if !res.HasSkip {
		t.Fatalf("BROKEN CASE: expected HasSkip=true for `if true { t.Skip(...) }`, got %+v", res)
	}
}

// TestResolveSpecTest_ConditionalTestingShortSkip_RemainsIdiomExempt is the
// non-regression proof: the genuine Go idiom `if testing.Short() {
// t.Skip(...) }` must NOT be flagged -- only a CONSTANT-true condition is
// treated as unconditional, a real runtime check is left alone.
func TestResolveSpecTest_ConditionalTestingShortSkip_RemainsIdiomExempt(t *testing.T) {
	t.Parallel()
	domainDir := writeSpecFixture(t, "spec/model/risk_test.go", riskTestConditionalSkipSrc)
	res, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if res.HasSkip {
		t.Fatalf("REGRESSION: expected HasSkip=false for the testing.Short() idiom, got %+v", res)
	}
}

// TestResolveSpecTest_MUTATION_IfTrueSkipToRealConditionFlipsHasSkip is the
// break->fix mutation proof: start from `if true { t.Skip(...) }`
// (HasSkip=true), then replace the constant-true condition with a real
// runtime condition (testing.Short()) -- HasSkip must flip to false.
func TestResolveSpecTest_MUTATION_IfTrueSkipToRealConditionFlipsHasSkip(t *testing.T) {
	t.Parallel()
	const ifTrueSkip = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	if true {
		t.Skip("dressed-up unconditional skip")
	}
}
`
	domainDir := writeSpecFixture(t, "spec/model/risk_test.go", ifTrueSkip)
	path := filepath.Join(domainDir, "spec", "model", "risk_test.go")

	res, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if !res.HasSkip {
		t.Fatalf("BROKEN CASE: expected HasSkip=true before fix, got %+v", res)
	}

	if err := os.WriteFile(path, []byte(riskTestConditionalSkipSrc), 0o644); err != nil {
		t.Fatalf("WriteFile fix: %v", err)
	}
	res2, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest after fix: %v", err)
	}
	if res2.HasSkip {
		t.Fatalf("FIXED CASE: expected HasSkip=false after replacing with a real runtime condition, got %+v", res2)
	}
}

// --- hotamspec Scenario.Then recognized as teeth (task W1.1) ---------------

// TestResolveSpecTest_ScenarioThenCountsAsTeeth proves a verified_by test
// written entirely against the hotamspec scenario recorder (s.Given/s.When/
// s.Then/s.Value, PLAN-scenario-generated-spec.md §2 D1) -- with NO literal
// t.Error/t.Fatal/require/assert call anywhere in its own body -- is still
// recognized as having real teeth, because s.Then(...) is exactly the shape
// isTeethCall now also matches (a method literally named "Then"). Without
// this, every scenario-recorder-based verified_by test would be
// mechanically indistinguishable from a t.Log-only vacuous test to
// check_verified_by_test_has_teeth.
func TestResolveSpecTest_ScenarioThenCountsAsTeeth(t *testing.T) {
	t.Parallel()
	const src = `package model

import (
	"testing"

	"prat-spec/hotamspec"
)

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-example", "example")
	r, err := NewRisk("")
	s.Then("rejects a missing owner", err != nil && r == nil)
}
`
	domainDir := writeSpecFixture(t, "spec/model/risk_test.go", src)
	res, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if !res.HasTeeth {
		t.Fatalf("expected HasTeeth=true for a test using hotamspec's s.Then(...), got %+v", res)
	}
}

// TestResolveSpecTest_UnrelatedThenMethodAlsoCountsAsTeeth documents the
// deliberate, narrow over-approximation isTeethCall's doc comment accepts:
// ANY method literally named "Then" (not just hotamspec.Scenario's) is
// treated as teeth, since AST-only inspection cannot resolve the receiver's
// concrete type. This is intentional, not a regression to guard against --
// see isTeethCall's doc comment for why the remaining verified_by checks
// (especially check_verified_by_test_passes actually running the test)
// keep this widening safe.
func TestResolveSpecTest_UnrelatedThenMethodAlsoCountsAsTeeth(t *testing.T) {
	t.Parallel()
	const src = `package model

import "testing"

type notAScenario struct{}

func (n notAScenario) Then(desc string) {}

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	var n notAScenario
	n.Then("unrelated Then method, not hotamspec's")
}
`
	domainDir := writeSpecFixture(t, "spec/model/risk_test.go", src)
	res, err := ResolveSpecTest(SpecRoot(domainDir, false), "spec/model/risk_test.go", "TestNewRisk_RejectsMissingOwner")
	if err != nil {
		t.Fatalf("ResolveSpecTest: %v", err)
	}
	if !res.HasTeeth {
		t.Fatalf("expected HasTeeth=true (documented over-approximation for any .Then(...) call), got %+v", res)
	}
}

// --- parseTestFileCached (task #384: AST-parse cache) -----------------------
//
// These tests exercise parseTestFileCached directly (rather than only
// through scanTestFile/collectTestFuncNames) so each cache property below is
// pinned independently of the two consumers' own merge logic.

// writeGoTestFile writes a standalone _test.go file (not under a "spec/"
// prefix -- parseTestFileCached itself has no spec/-scope requirement, it
// operates on any path) and returns its full path.
func writeGoTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", full, err)
	}
	return full
}

const parseCacheFixtureOneTest = `package fixture

import "testing"

func TestAlpha(t *testing.T) {
	if 1 != 1 {
		t.Fatal("check_alpha_marker")
	}
}
`

const parseCacheFixtureTwoTests = `package fixture

import "testing"

func TestAlpha(t *testing.T) {
	if 1 != 1 {
		t.Fatal("check_alpha_marker")
	}
}

func TestBeta(t *testing.T) {
	if 1 != 1 {
		t.Fatal("check_beta_marker")
	}
}
`

// testFuncNamesSorted returns parsed.testFuncs sorted, so assertions do not
// depend on AST declaration order (which parseTestFileUncached already
// preserves as first-seen order, but sorting here keeps the test itself
// independent of that incidental ordering guarantee).
func testFuncNamesSorted(parsed parsedTestFile) []string {
	out := append([]string(nil), parsed.testFuncs...)
	sort.Strings(out)
	return out
}

// (a) Parsing the same unchanged file twice returns identical results, and
// the second call is served from the cache (never touching go/parser again).
//
// Because this task's own constraint forbids running anything (no go test,
// no benchmark) in the session that authored it, direct proof that the
// SECOND call skips parser.ParseFile cannot be captured by an execution
// trace here. Structural proof instead: parseTestFileCached's own body
// (gate.go) shows the ONLY call to parseTestFileUncached (the sole
// caller of parser.ParseFile in this path) is guarded by `if cached, ok :=
// testFileParseCache.Load(absPath); ok { ... return entry.parsed, nil }` --
// a cache hit returns before parseTestFileUncached is ever reached. This
// test proves the OBSERVABLE half of that contract (repeated calls for an
// unchanged file yield byte-identical results, satisfying what a caller can
// actually depend on); the resolver is asked to additionally confirm via
// -race/benchmark that the second call is actually a cache hit, since a
// benchmark run is exactly the kind of measurement this session must not
// perform itself.
func TestParseTestFileCached_UnchangedFile_IdenticalResults(t *testing.T) {
	ResetTestFileParseCacheForTest()
	dir := t.TempDir()
	path := writeGoTestFile(t, dir, "sample_test.go", parseCacheFixtureTwoTests)

	first, err := parseTestFileCached(path)
	if err != nil {
		t.Fatalf("parseTestFileCached (1st call): %v", err)
	}
	second, err := parseTestFileCached(path)
	if err != nil {
		t.Fatalf("parseTestFileCached (2nd call): %v", err)
	}

	wantFuncs := []string{"TestAlpha", "TestBeta"}
	if got := testFuncNamesSorted(first); !equalStringSlices(got, wantFuncs) {
		t.Fatalf("1st call testFuncs = %v, want %v", got, wantFuncs)
	}
	if got := testFuncNamesSorted(second); !equalStringSlices(got, wantFuncs) {
		t.Fatalf("2nd call testFuncs = %v, want %v", got, wantFuncs)
	}
	if len(first.checkHits) != len(second.checkHits) {
		t.Fatalf("checkHits length differs across calls: 1st=%d 2nd=%d", len(first.checkHits), len(second.checkHits))
	}
	for i := range first.checkHits {
		if first.checkHits[i] != second.checkHits[i] {
			t.Fatalf("checkHits[%d] differs across calls: 1st=%+v 2nd=%+v", i, first.checkHits[i], second.checkHits[i])
		}
	}

	// The cache entry itself must exist and carry the file's real mtime/size
	// after both calls -- direct proof the second call went through the
	// cache-populated path rather than some other code path.
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	cachedRaw, ok := testFileParseCache.Load(absPath)
	if !ok {
		t.Fatalf("expected a cache entry for %s after two calls, found none", absPath)
	}
	cached := cachedRaw.(testFileParseCacheEntry)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat: %v", err)
	}
	if !cached.modTime.Equal(info.ModTime()) || cached.size != info.Size() {
		t.Fatalf("cache entry (modTime=%v size=%d) does not match current file stat (modTime=%v size=%d)",
			cached.modTime, cached.size, info.ModTime(), info.Size())
	}
}

// (b) Mutating the file's content (which changes its size, and -- given
// filesystem mtime resolution -- normally also its mtime) invalidates the
// cache: a subsequent call reflects the NEW content, not the stale cached
// one. The test forces mtime forward explicitly via os.Chtimes (rather than
// relying on wall-clock granularity between the two WriteFile calls, which
// on some filesystems/OSes can share the same truncated mtime tick) so the
// invalidation is deterministic regardless of host clock resolution --
// still a genuine mtime+size change, exactly the shape parseTestFileCached's
// check is built to catch, not a weakened test.
func TestParseTestFileCached_MutatedFile_Invalidates(t *testing.T) {
	ResetTestFileParseCacheForTest()
	dir := t.TempDir()
	path := writeGoTestFile(t, dir, "sample_test.go", parseCacheFixtureOneTest)

	before, err := parseTestFileCached(path)
	if err != nil {
		t.Fatalf("parseTestFileCached (before mutation): %v", err)
	}
	if got := testFuncNamesSorted(before); !equalStringSlices(got, []string{"TestAlpha"}) {
		t.Fatalf("before mutation: testFuncs = %v, want [TestAlpha]", got)
	}

	// Mutate: add a second test function (changes size unconditionally, and
	// we additionally force mtime forward so this test never depends on
	// filesystem timestamp granularity to observe a change).
	if err := os.WriteFile(path, []byte(parseCacheFixtureTwoTests), 0o644); err != nil {
		t.Fatalf("WriteFile mutation: %v", err)
	}
	future := time.Now().Add(1 * time.Hour)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("os.Chtimes: %v", err)
	}

	after, err := parseTestFileCached(path)
	if err != nil {
		t.Fatalf("parseTestFileCached (after mutation): %v", err)
	}
	wantAfter := []string{"TestAlpha", "TestBeta"}
	if got := testFuncNamesSorted(after); !equalStringSlices(got, wantAfter) {
		t.Fatalf("BROKEN CACHE: after mutation testFuncs = %v, want %v (stale cache entry was served)", got, wantAfter)
	}

	// Fix/reverse direction: mutate back down to one test function, forcing
	// mtime forward again, and confirm the cache follows the shrink too (not
	// just growth) -- proves invalidation is symmetric, not merely "a bigger
	// file always reparses by accident of some other check".
	if err := os.WriteFile(path, []byte(parseCacheFixtureOneTest), 0o644); err != nil {
		t.Fatalf("WriteFile revert: %v", err)
	}
	future2 := future.Add(1 * time.Hour)
	if err := os.Chtimes(path, future2, future2); err != nil {
		t.Fatalf("os.Chtimes (2nd): %v", err)
	}
	reverted, err := parseTestFileCached(path)
	if err != nil {
		t.Fatalf("parseTestFileCached (after revert): %v", err)
	}
	if got := testFuncNamesSorted(reverted); !equalStringSlices(got, []string{"TestAlpha"}) {
		t.Fatalf("after revert testFuncs = %v, want [TestAlpha] (shrink was not observed)", got)
	}
}

// (c) Two different files must never collide in the cache even if they
// happen to share an identical (mtime, size) pair -- the cache key must
// include the full path, not just (mtime, size). This test constructs two
// files with DELIBERATELY IDENTICAL sizes (parseCacheFixtureOneTest's
// content, byte-for-byte, written to two different paths) and forces
// IDENTICAL mtimes via os.Chtimes, then proves each path's cache entry still
// reflects that path's own (distinct) content -- catches a hypothetical bug
// where the cache key was (mtime, size) alone, which would let file B read
// back file A's cached parse.
func TestParseTestFileCached_DifferentFilesSameMtimeSize_NoCollision(t *testing.T) {
	ResetTestFileParseCacheForTest()
	dir := t.TempDir()

	// Same byte length, different content: TestAlpha vs a same-length
	// renamed TestZzzzz (pad/trim so both files are EXACTLY the same size in
	// bytes, isolating the test from "size differs" as an accidental
	// discriminator).
	srcA := parseCacheFixtureOneTest
	srcB := makeSameSizeDifferentTestName(t, srcA, "TestAlpha", "TestZzzzz")
	if len(srcA) != len(srcB) {
		t.Fatalf("test fixture invalid: srcA len=%d srcB len=%d, want equal", len(srcA), len(srcB))
	}

	pathA := writeGoTestFile(t, dir, "a_test.go", srcA)
	pathB := writeGoTestFile(t, dir, "b_test.go", srcB)

	// Force identical mtimes on both files -- the collision scenario this
	// test targets requires (mtime, size) to match exactly across the two
	// paths.
	same := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(pathA, same, same); err != nil {
		t.Fatalf("os.Chtimes pathA: %v", err)
	}
	if err := os.Chtimes(pathB, same, same); err != nil {
		t.Fatalf("os.Chtimes pathB: %v", err)
	}
	infoA, err := os.Stat(pathA)
	if err != nil {
		t.Fatalf("os.Stat pathA: %v", err)
	}
	infoB, err := os.Stat(pathB)
	if err != nil {
		t.Fatalf("os.Stat pathB: %v", err)
	}
	if !infoA.ModTime().Equal(infoB.ModTime()) || infoA.Size() != infoB.Size() {
		t.Fatalf("test fixture invalid: expected identical (mtime,size), got A=(%v,%d) B=(%v,%d)",
			infoA.ModTime(), infoA.Size(), infoB.ModTime(), infoB.Size())
	}

	parsedA, err := parseTestFileCached(pathA)
	if err != nil {
		t.Fatalf("parseTestFileCached pathA: %v", err)
	}
	parsedB, err := parseTestFileCached(pathB)
	if err != nil {
		t.Fatalf("parseTestFileCached pathB: %v", err)
	}

	if got := testFuncNamesSorted(parsedA); !equalStringSlices(got, []string{"TestAlpha"}) {
		t.Fatalf("COLLISION: pathA testFuncs = %v, want [TestAlpha] (identical mtime/size as pathB leaked its result in)", got)
	}
	if got := testFuncNamesSorted(parsedB); !equalStringSlices(got, []string{"TestZzzzz"}) {
		t.Fatalf("COLLISION: pathB testFuncs = %v, want [TestZzzzz] (identical mtime/size as pathA leaked its result in)", got)
	}

	// Re-read both from cache (now populated) and confirm the SAME
	// non-collision holds on a guaranteed cache-hit path too.
	parsedA2, err := parseTestFileCached(pathA)
	if err != nil {
		t.Fatalf("parseTestFileCached pathA (2nd, cache hit): %v", err)
	}
	parsedB2, err := parseTestFileCached(pathB)
	if err != nil {
		t.Fatalf("parseTestFileCached pathB (2nd, cache hit): %v", err)
	}
	if got := testFuncNamesSorted(parsedA2); !equalStringSlices(got, []string{"TestAlpha"}) {
		t.Fatalf("COLLISION on cache hit: pathA testFuncs = %v, want [TestAlpha]", got)
	}
	if got := testFuncNamesSorted(parsedB2); !equalStringSlices(got, []string{"TestZzzzz"}) {
		t.Fatalf("COLLISION on cache hit: pathB testFuncs = %v, want [TestZzzzz]", got)
	}
}

// makeSameSizeDifferentTestName renames oldName to newName inside src,
// requiring the caller to pass names of EQUAL byte length (asserted below)
// so the result is trivially byte-identical in length to src -- deliberately
// simple rather than general: the one real call site in this file
// (TestParseTestFileCached_DifferentFilesSameMtimeSize_NoCollision) passes
// "TestAlpha" -> "TestZzzzz", both 9 bytes, and the test's own fixture-
// validity check after calling this re-verifies the length invariant
// independently, so this function only needs to be correct for equal-length
// renames, never for the general shrink/grow case.
func makeSameSizeDifferentTestName(t *testing.T, src, oldName, newName string) string {
	t.Helper()
	if len(oldName) != len(newName) {
		t.Fatalf("makeSameSizeDifferentTestName: oldName %q (%d bytes) and newName %q (%d bytes) must be equal length",
			oldName, len(oldName), newName, len(newName))
	}
	renamed := replaceAllLiteral(src, oldName, newName)
	if len(renamed) != len(src) {
		t.Fatalf("makeSameSizeDifferentTestName: got len=%d, want len=%d (oldName=%q newName=%q)", len(renamed), len(src), oldName, newName)
	}
	return renamed
}

func replaceAllLiteral(s, old, replacement string) string {
	out := ""
	for {
		i := indexOf(s, old)
		if i < 0 {
			return out + s
		}
		out += s[:i] + replacement
		s = s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// (d) Concurrent calls from multiple goroutines, against a single shared
// path AND against several distinct paths, must not race. This test is
// written to be run under `go test -race` (which this session cannot invoke
// itself, per the task's execution ban) -- the reasoning for why it is race
// -free by construction is documented inline below; the resolver is asked to
// confirm empirically via -race.
//
// STATIC RACE REASONING: parseTestFileCached's only shared mutable state is
// testFileParseCache (a sync.Map). Every read goes through
// testFileParseCache.Load and every write through testFileParseCache.Store,
// both of which are individually atomic/safe per sync.Map's own contract
// (concurrent Load/Store/Delete/Range calls are safe without any additional
// locking, https://pkg.go.dev/sync#Map). There is no read-modify-write
// sequence spanning two separate sync.Map calls on the SAME key that could
// race destructively: the Load-then-maybe-Store sequence in
// parseTestFileCached is a classic "check then update" pattern, but the
// worst outcome from two goroutines racing on it for the same key is a
// harmless redundant reparse (both goroutines miss the cache, both parse
// independently, both call Store -- the LAST Store simply wins, and since
// both parses are of the same on-disk content at that moment, whichever
// value wins is equally correct), never a corrupted or partially-written
// value, because parsedTestFile itself is only ever constructed as a whole
// value (parseTestFileUncached returns a complete, immutable
// parsedTestFile; no goroutine ever mutates a parsedTestFile or checkHits
// slice that another goroutine might be reading). This test exercises that
// exact contention shape (many goroutines racing on the SAME path, plus many
// goroutines racing across DIFFERENT paths) so `go test -race` has real
// concurrent traffic to verify against, even though the reasoning above is
// what actually establishes race-freedom, not the test's mere existence.
func TestParseTestFileCached_ConcurrentAccess_NoRace(t *testing.T) {
	ResetTestFileParseCacheForTest()
	dir := t.TempDir()

	const numFiles = 5
	paths := make([]string, numFiles)
	for i := 0; i < numFiles; i++ {
		name := fmt.Sprintf("concurrent_%d_test.go", i)
		src := fmt.Sprintf(`package fixture

import "testing"

func TestConcurrent%d(t *testing.T) {
	if 1 != 1 {
		t.Fatal("check_concurrent_%d_marker")
	}
}
`, i, i)
		paths[i] = writeGoTestFile(t, dir, name, src)
	}

	var wg sync.WaitGroup
	const goroutinesPerFile = 8
	errCh := make(chan error, numFiles*goroutinesPerFile)

	for i := 0; i < numFiles; i++ {
		path := paths[i]
		wantFunc := fmt.Sprintf("TestConcurrent%d", i)
		for g := 0; g < goroutinesPerFile; g++ {
			wg.Add(1)
			go func(path, wantFunc string) {
				defer wg.Done()
				parsed, err := parseTestFileCached(path)
				if err != nil {
					errCh <- fmt.Errorf("parseTestFileCached(%s): %w", path, err)
					return
				}
				if got := testFuncNamesSorted(parsed); len(got) != 1 || got[0] != wantFunc {
					errCh <- fmt.Errorf("parseTestFileCached(%s) = %v, want [%s]", path, got, wantFunc)
				}
			}(path, wantFunc)
		}
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

// equalStringSlices is a small local helper (spec_resolver_test.go's own
// equalSlices lives in gate_test.go and is unexported there but in the same
// package -- reusing it directly is fine, this local alias just documents
// intent at each call site above without depending on gate_test.go's naming
// staying stable).
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

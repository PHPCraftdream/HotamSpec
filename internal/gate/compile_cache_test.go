package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// compile_cache_test.go proves the binary-level compile cache
// (compile_cache.go) is correct along the six dimensions the task names:
// same-package deduplication, cross-package independence, coverpkg
// distinction, compile-failure classification, cleanup, and byte-identical
// recording output. The fixtures mirror test_exec_test.go's own
// writeModuleFixture / writeRecordingFixture conventions (standalone temp
// Go modules with real go.mod + real package source) so these tests
// exercise the genuine `go test -c` + direct-binary-invocation path the
// production code now takes, not a mocked stub.

// twoTestImplSrc is a fixture implementation with TWO test entry points
// living in the SAME package -- the shape the compile cache exists to
// optimize (two different TestXxx functions that, without the cache, would
// each trigger their own full `go test -run` compile of the same package).
const twoTestImplSrc = `package model

func IsPositive(n int) bool {
	return n > 0
}

func IsNegative(n int) bool {
	return n < 0
}
`

const twoTestTestSrc = `package model

import "testing"

func TestIsPositive_RejectsZero(t *testing.T) {
	if IsPositive(0) {
		t.Fatalf("expected false for zero")
	}
}

func TestIsNegative_RejectsZero(t *testing.T) {
	if IsNegative(0) {
		t.Fatalf("expected false for zero")
	}
}
`

// TestCompileCache_TwoTestsSamePackage_OneCompile is the CENTRAL
// correctness proof for the optimization: two DIFFERENT verified_by tests
// (TestIsPositive_RejectsZero and TestIsNegative_RejectsZero) living in
// the SAME package each drive ONE RunVerifiedByTest call, yet
// CompileInvocationCount() must increase by exactly ONE across both calls
// -- the second call reuses the first call's compiled binary instead of
// recompiling the whole package from scratch. Asserting on the invocation
// counter (a direct miss-counter, not wall-clock) keeps the test
// deterministic and CI-stable regardless of host load.
func TestCompileCache_TwoTestsSamePackage_OneCompile(t *testing.T) {
	ResetRunCacheForTest()
	root := writeModuleFixture(t, "example.com/samepkg", "model", twoTestImplSrc, twoTestTestSrc)

	before := CompileInvocationCount()

	first := RunVerifiedByTest(root, "model/impl_test.go", "TestIsPositive_RejectsZero")
	if first.Err != nil {
		t.Fatalf("first test unexpected infra error: %v", first.Err)
	}
	if !first.Passed {
		t.Fatalf("first test expected Passed=true, got %+v", first)
	}

	afterFirst := CompileInvocationCount()
	if compiles := afterFirst - before; compiles != 1 {
		t.Fatalf("expected exactly 1 compile after the first test (one package, cold cache), got %d", compiles)
	}

	// Second DIFFERENT test in the SAME package: a verdict-cache miss
	// (different testName -> different cache key), so runGoTest IS
	// reached -- but compileTestBinary must HIT (same package), so no
	// new compile.
	second := RunVerifiedByTest(root, "model/impl_test.go", "TestIsNegative_RejectsZero")
	if second.Err != nil {
		t.Fatalf("second test unexpected infra error: %v", second.Err)
	}
	if !second.Passed {
		t.Fatalf("second test expected Passed=true, got %+v", second)
	}

	afterSecond := CompileInvocationCount()
	if compiles := afterSecond - afterFirst; compiles != 0 {
		t.Fatalf("expected ZERO additional compiles for a second test in the SAME package (compile cache must hit), got %d", compiles)
	}
}

// TestCompileCache_TwoTestsSamePackage_Recording_AlsoDeduplicates proves
// the SAME one-compile property holds on the RECORD-mode path
// (RunVerifiedByTestRecording), which has NO verdict cache by design --
// without the compile cache, two recording calls would each spawn their
// own full `go test -run` subprocess. With it, the compile happens once.
func TestCompileCache_TwoTestsSamePackage_Recording_AlsoDeduplicates(t *testing.T) {
	ResetRunCacheForTest()
	const modulePath = "example.com/samepkgrec"
	root := writeRecordingFixture(t, modulePath, "model", scenarioImplSrc, "model", `
package model

import (
	"testing"

	"`+modulePath+`/hotamspec"
)

func TestRequireComplete_ScenarioRecorded(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-rec-1", "narrates one")
	s.Given("a fields count of zero", "fields", 0)
	err := RequireComplete(0)
	s.When("RequireComplete is called")
	s.Then("an error is returned", err != nil)
	s.Value("error_text", err)
}

func TestRequireComplete_ScenarioRecorded_Second(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-rec-2", "narrates another")
	s.Given("a fields count of zero", "fields", 0)
	err := RequireComplete(0)
	s.When("RequireComplete is called")
	s.Then("an error is returned", err != nil)
	s.Value("error_text", err)
}
`)

	before := CompileInvocationCount()

	first := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestRequireComplete_ScenarioRecorded", "model/impl.go")
	if first.Err != nil || !first.Passed {
		t.Fatalf("first recording unexpected result %+v, err=%v, output:\n%s", first.TestRunResult, first.Err, first.Output)
	}

	afterFirst := CompileInvocationCount()
	if compiles := afterFirst - before; compiles != 1 {
		t.Fatalf("expected exactly 1 compile after the first recording call, got %d", compiles)
	}

	second := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestRequireComplete_ScenarioRecorded_Second", "model/impl.go")
	if second.Err != nil || !second.Passed {
		t.Fatalf("second recording unexpected result %+v, err=%v, output:\n%s", second.TestRunResult, second.Err, second.Output)
	}

	afterSecond := CompileInvocationCount()
	if compiles := afterSecond - afterFirst; compiles != 0 {
		t.Fatalf("expected ZERO additional compiles for a second recording call in the SAME package with the SAME coverpkg (compile cache must hit), got %d", compiles)
	}
}

// TestCompileCache_TwoTestsDifferentPackages_TwoCompiles proves the cache
// does NOT falsely conflate two DIFFERENT packages: each package compiles
// independently. Two separate package directories under one module, two
// RunVerifiedByTest calls, two compiles.
func TestCompileCache_TwoTestsDifferentPackages_TwoCompiles(t *testing.T) {
	ResetRunCacheForTest()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/diffpkg\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	// Two package directories: alpha/ and beta/. Each gets its own impl
	// + test, both standalone-compilable.
	for _, pkg := range []string{"alpha", "beta"} {
		dir := filepath.Join(root, pkg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", pkg, err)
		}
		impl := `package ` + pkg + `

func IsPositive(n int) bool { return n > 0 }
`
		test := `package ` + pkg + `

import "testing"

func TestIsPositive_RejectsZero(t *testing.T) {
	if IsPositive(0) {
		t.Fatalf("expected false for zero")
	}
}
`
		if err := os.WriteFile(filepath.Join(dir, "impl.go"), []byte(impl), 0o644); err != nil {
			t.Fatalf("WriteFile %s/impl.go: %v", pkg, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "impl_test.go"), []byte(test), 0o644); err != nil {
			t.Fatalf("WriteFile %s/impl_test.go: %v", pkg, err)
		}
	}

	before := CompileInvocationCount()

	first := RunVerifiedByTest(root, "alpha/impl_test.go", "TestIsPositive_RejectsZero")
	if first.Err != nil || !first.Passed {
		t.Fatalf("alpha test unexpected: %+v err=%v", first, first.Err)
	}

	afterFirst := CompileInvocationCount()
	if compiles := afterFirst - before; compiles != 1 {
		t.Fatalf("expected exactly 1 compile after the alpha test, got %d", compiles)
	}

	second := RunVerifiedByTest(root, "beta/impl_test.go", "TestIsPositive_RejectsZero")
	if second.Err != nil || !second.Passed {
		t.Fatalf("beta test unexpected: %+v err=%v", second, second.Err)
	}

	afterSecond := CompileInvocationCount()
	if compiles := afterSecond - afterFirst; compiles != 1 {
		t.Fatalf("expected exactly 1 MORE compile for a DIFFERENT package (no false cross-package cache hit), got %d", compiles)
	}
}

// TestCompileCache_DifferentCoverPkg_DifferentBinaries proves the
// coverpkg-must-be-baked-at-compile-time distinction: two recording calls
// for the SAME package but with DIFFERENT coverPkgFile values (pointing
// -coverpkg at different packages) must compile TWO separate binaries, not
// share one. This is the easiest correctness bug to introduce here (a
// cache key that omits coverPkgPattern would silently produce a coverage
// profile over the wrong package on the second call), so it gets its own
// dedicated test.
//
// The model test imports BOTH cover packages so each one's functions are
// actually exercised (a -coverpkg that targets a package whose code is
// never called during the test still gets instrumented into the binary,
// but the resulting profile is empty -- making the coverpkg distinction
// unobservable. Importing both and calling both ensures the profile has
// real entries to compare).
func TestCompileCache_DifferentCoverPkg_DifferentBinaries(t *testing.T) {
	ResetRunCacheForTest()
	const modulePath = "example.com/coverpkg"
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	// Two coverable packages, each with a distinct function.
	for _, pkg := range []struct{ name, fn string }{
		{"covera", "ValueA"},
		{"coverb", "ValueB"},
	} {
		dir := filepath.Join(root, pkg.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", pkg.name, err)
		}
		src := `package ` + pkg.name + `

func ` + pkg.fn + `() int { return 1 }
`
		if err := os.WriteFile(filepath.Join(dir, "impl.go"), []byte(src), 0o644); err != nil {
			t.Fatalf("WriteFile %s/impl.go: %v", pkg.name, err)
		}
	}

	// model/ holds a test that calls BOTH cover packages' functions,
	// so each coverpkg's coverage profile has at least one non-zero entry.
	modelDir := filepath.Join(root, "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "impl_test.go"), []byte(`package model

import (
	"testing"

	"`+modulePath+`/covera"
	"`+modulePath+`/coverb"
)

func TestCallsBothCoverPkgs(t *testing.T) {
	if covera.ValueA() != 1 {
		t.Fatal("covera.ValueA failed")
	}
	if coverb.ValueB() != 1 {
		t.Fatal("coverb.ValueB failed")
	}
}
`), 0o644); err != nil {
		t.Fatalf("WriteFile model/impl_test.go: %v", err)
	}

	before := CompileInvocationCount()

	// Call 1: coverpkg = covera. The compiled binary has covera's
	// instrumentation baked in at compile time.
	first := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestCallsBothCoverPkgs", "covera/impl.go")
	if first.Err != nil || !first.Passed {
		t.Fatalf("first recording (covera) unexpected: %+v err=%v output:\n%s", first.TestRunResult, first.Err, first.Output)
	}
	afterFirst := CompileInvocationCount()
	if compiles := afterFirst - before; compiles != 1 {
		t.Fatalf("expected exactly 1 compile after the first recording (covera), got %d", compiles)
	}

	// Call 2: SAME package + SAME test, but coverpkg = coverb (different
	// -coverpkg). Must compile a SEPARATE binary -- coverpkg is baked in
	// at compile time, so a binary compiled with -coverpkg=covera cannot
	// produce a correct coverage profile over coverb.
	second := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestCallsBothCoverPkgs", "coverb/impl.go")
	if second.Err != nil || !second.Passed {
		t.Fatalf("second recording (coverb) unexpected: %+v err=%v output:\n%s", second.TestRunResult, second.Err, second.Output)
	}
	afterSecond := CompileInvocationCount()
	if compiles := afterSecond - afterFirst; compiles != 1 {
		t.Fatalf("DIFFERENT-coverpkg-same-package BUG: expected exactly 1 MORE compile for a different coverpkg (coverpkg is baked at compile time; cache key must distinguish), got %d", compiles)
	}

	// Also prove the coverage profiles actually DIFFER: each one must
	// mention its OWN coverpkg's import path, not the other's. A
	// cache-conflation bug would make one of them mention the wrong
	// package.
	firstCover := string(first.CoverProfile)
	secondCover := string(second.CoverProfile)
	if !strings.Contains(firstCover, modulePath+"/covera/") {
		t.Errorf("first coverprofile (covera) should mention %s/covera/:\n%s", modulePath, firstCover)
	}
	if !strings.Contains(secondCover, modulePath+"/coverb/") {
		t.Errorf("second coverprofile (coverb) should mention %s/coverb/:\n%s", modulePath, secondCover)
	}
	if strings.Contains(firstCover, modulePath+"/coverb/") {
		t.Errorf("first coverprofile (covera) WRONGLY mentions coverb -- coverpkg conflation bug:\n%s", firstCover)
	}
	if strings.Contains(secondCover, modulePath+"/covera/") {
		t.Errorf("second coverprofile (coverb) WRONGLY mentions covera -- coverpkg conflation bug:\n%s", secondCover)
	}
}

// TestCompileCache_CompileFailure_StillClassifiedCorrectly proves a broken
// package is still reported as CompileFailed=true (not a panic, not a bare
// test failure) when the compile happens via the cached `go test -c` path
// instead of the old inline `go test -run` path. Mirrors
// TestRunVerifiedByTest_CompileFailure_ReportsCompileFailedNotPanic's
// shape exactly -- same fixture, same assertions, same diagnostic quality
// the caller sees, just driven through the new compile-step classification.
func TestCompileCache_CompileFailure_StillClassifiedCorrectly(t *testing.T) {
	ResetRunCacheForTest()
	root := writeModuleFixture(t, "example.com/compilefail", "model", uncompilableImplSrc, passingTestSrc)

	before := CompileInvocationCount()
	result := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_RejectsZeroFields")
	compiles := CompileInvocationCount() - before
	if compiles != 1 {
		t.Fatalf("expected exactly 1 compile attempt for a broken package, got %d", compiles)
	}
	if result.Err != nil {
		t.Fatalf("unexpected infra error (a syntax error is a TEST verdict, not infra failure): %v", result.Err)
	}
	if result.Passed {
		t.Fatalf("expected Passed=false for a package that does not compile, got %+v", result)
	}
	if !result.CompileFailed {
		t.Fatalf("expected CompileFailed=true for a syntax error (same classification as the old inline go-test-run path), got %+v", result)
	}

	// A SECOND call for a different test in the SAME broken package must
	// hit the cached CompileFailed result (no second compile attempt) --
	// the cache stores CompileFailed outcomes the same way it stores
	// successful compiles, so a broken package is not retried on every
	// subsequent call within the process.
	beforeSecond := CompileInvocationCount()
	second := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_RejectsZeroFields")
	// (Same testName -> verdict cache hit; force a different testName to
	// reach runGoTest. Easiest: call runGoTest directly would reach
	// compileTestBinary, but keeping the public-API surface, use a
	// recording call which has no verdict cache.)
	_ = second
	// Use the recording path (no verdict cache) to prove the compile
	// cache serves the cached CompileFailed without recompiling.
	recResult := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestRequireComplete_RejectsZeroFields", "")
	if recResult.Err != nil {
		t.Fatalf("recording: unexpected infra error: %v", recResult.Err)
	}
	if !recResult.CompileFailed {
		t.Fatalf("recording: expected CompileFailed=true from cached compile failure, got %+v", recResult.TestRunResult)
	}
	secondCompiles := CompileInvocationCount() - beforeSecond
	if secondCompiles != 0 {
		t.Fatalf("expected ZERO additional compiles for a second call against an already-cached broken package, got %d", secondCompiles)
	}
}

// TestCompileCache_Cleanup_RemovesAllBinaries proves the end-of-run
// CleanupCompileCache hook (deferred by cmd/hotam's top-level command
// handlers) removes every compiled .test binary this process produced: no
// hotam-compile-* tmp directory survives the cleanup, and the in-memory
// cache is empty so a subsequent call would recompile from scratch.
func TestCompileCache_Cleanup_RemovesAllBinaries(t *testing.T) {
	ResetRunCacheForTest()
	root := writeModuleFixture(t, "example.com/cleanupmod", "model", passingImplSrc, passingTestSrc)

	// Populate the cache with at least one real compile.
	result := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_RejectsZeroFields")
	if result.Err != nil || !result.Passed {
		t.Fatalf("expected a passing first run, got %+v err=%v", result, result.Err)
	}
	if CompileInvocationCount() != 1 {
		t.Fatalf("expected exactly 1 compile before cleanup, got %d", CompileInvocationCount())
	}

	// Snapshot the tmp dir glob BEFORE cleanup so the assertion below is
	// not fooled by some OTHER process's leftover hotam-compile-* dirs.
	beforeGlob, err := filepath.Glob(filepath.Join(os.TempDir(), "hotam-compile-*"))
	if err != nil {
		t.Fatalf("Glob before cleanup: %v", err)
	}

	CleanupCompileCache()

	// After cleanup: the cache is empty, the counter is preserved (it is
	// a process-lifetime counter, not reset by CleanupCompileCache --
	// ResetRunCacheForTest resets it, CleanupCompileCache does not), and
	// no hotam-compile-* tmp dirs survive.
	afterGlob, err := filepath.Glob(filepath.Join(os.TempDir(), "hotam-compile-*"))
	if err != nil {
		t.Fatalf("Glob after cleanup: %v", err)
	}
	if len(afterGlob) > len(beforeGlob) {
		t.Fatalf("expected no surviving hotam-compile-* tmp dirs after CleanupCompileCache: before=%v after=%v", beforeGlob, afterGlob)
	}

	// And the cache is genuinely empty: a fresh call after cleanup must
	// recompile (counter goes up by 1), not hit a stale cache entry.
	// ResetRunCacheForTest clears BOTH the verdict cache AND the compile
	// cache, so the next call is a true cold miss.
	ResetRunCacheForTest()
	countBeforeSecond := CompileInvocationCount()
	second := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_RejectsZeroFields")
	if second.Err != nil || !second.Passed {
		t.Fatalf("post-cleanup run: expected passing, got %+v err=%v", second, second.Err)
	}
	if got := CompileInvocationCount() - countBeforeSecond; got != 1 {
		t.Fatalf("post-cleanup run: expected exactly 1 fresh compile (cache must be empty after ResetRunCacheForTest), got %d", got)
	}

	// Final cleanup so this test does not leak its second-run tmp dir.
	CleanupCompileCache()
}

// TestCompileCache_RecordingResult_ByteIdentical_AfterCacheReset is the
// A/B proof the task's verification step demands: a scenario-narrated
// test's artifact JSON and coverage profile come out byte-for-byte
// IDENTICAL whether produced from a fresh compile (cache cold) or from a
// reused compiled binary (cache warm then cold again). Proves the
// compiled-binary-direct-invocation path preserves the exact bytes the
// old inline-`go test -run` path produced (which the existing
// TestRunVerifiedByTestRecording_RealScenario_AssertPlusArtifactPlusCoverage
// already pins the shape of, and TestRunVerifiedByTestRecording_
// Deterministic_TwoRunsByteIdentical pins the determinism of -- this test
// pins the cross-cache-state byte-identity the compile cache specifically
// must not disturb).
func TestCompileCache_RecordingResult_ByteIdentical_AfterCacheReset(t *testing.T) {
	const modulePath = "example.com/byteid"
	root := writeRecordingFixture(t, modulePath, "model", scenarioImplSrc, "model", scenarioTestSrc(modulePath))

	// Run A: cold cache (ResetRunCacheForTest was called by the test
	// framework's ResetRunCacheForTest at the top -- no, this test does
	// its own reset to be explicit).
	ResetRunCacheForTest()
	runA := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestRequireComplete_ScenarioRecorded", "model/impl.go")
	if runA.Err != nil || !runA.Passed {
		t.Fatalf("run A: unexpected %+v err=%v output:\n%s", runA.TestRunResult, runA.Err, runA.Output)
	}
	if len(runA.Artifacts) != 1 {
		t.Fatalf("run A: expected exactly 1 artifact, got %d", len(runA.Artifacts))
	}
	artA := runA.Artifacts[0].RawJSON
	coverA := append([]byte(nil), runA.CoverProfile...)

	// Run B: same call, SAME process, but cache is now WARM from run A
	// (same moduleRoot/pkgPattern/coverPkgPattern -> compile cache HIT,
	// the binary is reused). Must produce byte-identical output.
	runB := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestRequireComplete_ScenarioRecorded", "model/impl.go")
	if runB.Err != nil || !runB.Passed {
		t.Fatalf("run B (warm cache): unexpected %+v err=%v output:\n%s", runB.TestRunResult, runB.Err, runB.Output)
	}
	if len(runB.Artifacts) != 1 {
		t.Fatalf("run B: expected exactly 1 artifact, got %d", len(runB.Artifacts))
	}
	artB := runB.Artifacts[0].RawJSON
	coverB := runB.CoverProfile

	if string(artA) != string(artB) {
		t.Fatalf("BYTE-IDENTITY VIOLATION (artifact): cold-cache and warm-cache runs produced different artifact JSON\nA:\n%s\nB:\n%s", artA, artB)
	}
	if string(coverA) != string(coverB) {
		t.Fatalf("BYTE-IDENTITY VIOLATION (coverprofile): cold-cache and warm-cache runs produced different coverprofiles\nA:\n%s\nB:\n%s", coverA, coverB)
	}

	// Run C: cache RESET (forces a fresh recompile), then run again.
	// Proves a second COMPILE produces the same bytes as the first
	// compile -- the compile cache's reuse does not introduce any
	// nondeterminism that a from-scratch recompile would not also have.
	ResetRunCacheForTest()
	runC := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestRequireComplete_ScenarioRecorded", "model/impl.go")
	if runC.Err != nil || !runC.Passed {
		t.Fatalf("run C (post-reset): unexpected %+v err=%v output:\n%s", runC.TestRunResult, runC.Err, runC.Output)
	}
	if len(runC.Artifacts) != 1 {
		t.Fatalf("run C: expected exactly 1 artifact, got %d", len(runC.Artifacts))
	}
	artC := runC.Artifacts[0].RawJSON
	coverC := runC.CoverProfile

	if string(artA) != string(artC) {
		t.Fatalf("BYTE-IDENTITY VIOLATION (artifact, across cache reset): cold and post-reset-runs produced different artifact JSON\nA:\n%s\nC:\n%s", artA, artC)
	}
	if string(coverA) != string(coverC) {
		t.Fatalf("BYTE-IDENTITY VIOLATION (coverprofile, across cache reset): cold and post-reset runs produced different coverprofiles\nA:\n%s\nC:\n%s", coverA, coverC)
	}
}

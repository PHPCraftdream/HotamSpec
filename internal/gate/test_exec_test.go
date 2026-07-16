package gate

import (
	"os"
	"path/filepath"
	"testing"
)

// writeModuleFixture builds a standalone temp Go module: go.mod at root,
// plus a package directory pkgRelDir containing implSrc (as impl.go) and
// testSrc (as impl_test.go). Returns the module root. This is the shape
// RunVerifiedByTest needs to actually invoke `go test` -- unlike
// writeSpecFixture (spec_resolver_test.go), which only ever needs a single
// parseable file with no working module, RunVerifiedByTest requires a real,
// buildable module since it shells out to the real `go` toolchain.
func writeModuleFixture(t *testing.T, modulePath, pkgRelDir, implSrc, testSrc string) (moduleRoot string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	pkgDir := filepath.Join(root, filepath.FromSlash(pkgRelDir))
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("MkdirAll pkgDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "impl.go"), []byte(implSrc), 0o644); err != nil {
		t.Fatalf("WriteFile impl.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "impl_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatalf("WriteFile impl_test.go: %v", err)
	}
	return root
}

const passingImplSrc = `package model

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

const passingTestSrc = `package model

import "testing"

func TestRequireComplete_RejectsZeroFields(t *testing.T) {
	if err := RequireComplete(0); err == nil {
		t.Fatalf("expected error for zero fields, got nil")
	}
}
`

// guttedImplSrc is the Probe C mutation at unit-test scale: RequireComplete
// unconditionally returns nil (the validation is gutted), so
// TestRequireComplete_RejectsZeroFields above must fail when actually run.
const guttedImplSrc = `package model

func RequireComplete(fields int) error {
	return nil
}
`

const uncompilableImplSrc = `package model

func RequireComplete(fields int) error {
	this is not valid go syntax {{{
`

// Note: the subprocess-spawning tests below deliberately do NOT call
// t.Parallel() -- each shells out to a real `go test` child process, and
// running several of those concurrently contends hard for the Go build
// cache / module lock (worse on Windows), which was observed to push the
// whole package past the DEFAULT 30s `go test` timeout under load even
// though each test individually completes in a few seconds. Running them
// sequentially costs a little wall-clock time but removes that contention
// entirely -- determinism matters far more here than shaving a few seconds
// off a package that already finishes in ~15-25s total.
// TestRunVerifiedByTest_RealPassingTest_Passes also covers the cache-hit
// contract (folded in here, rather than a separate test, so the ONE real
// `go test` subprocess this test needs to spawn is shared rather than
// duplicated across two module fixtures -- see the package-level comment
// above about sequential subprocess tests and package-default-timeout
// contention): calling RunVerifiedByTest twice in a row with nothing
// changed must return the byte-identical cached result on the second call
// without a second real invocation -- verified by exact equality of both
// results (Output included), which would not hold if the second call raced
// a fresh `go test -v` run's non-deterministic elapsed-time text
// (go test's summary embeds e.g. "(0.00s)") into Output on a cache MISS.
func TestRunVerifiedByTest_RealPassingTest_Passes(t *testing.T) {
	ResetRunCacheForTest()
	root := writeModuleFixture(t, "example.com/passmod", "model", passingImplSrc, passingTestSrc)
	first := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_RejectsZeroFields")
	if first.Err != nil {
		t.Fatalf("unexpected infra error: %v", first.Err)
	}
	if !first.Passed {
		t.Fatalf("expected Passed=true for a genuinely passing test, got %+v", first)
	}

	second := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_RejectsZeroFields")
	if second.Err != nil {
		t.Fatalf("unexpected infra error (cache hit): %v", second.Err)
	}
	if first != second {
		t.Fatalf("expected the second call to return the byte-identical cached result (no re-run), got first=%+v second=%+v", first, second)
	}
}

// TestRunVerifiedByTest_MUTATION_CacheInvalidatesOnImplChange is Probe C
// reproduced at unit-test granularity AND the coordinator's central
// cache-correctness proof in one: run the SAME verified_by entry (same
// file:test string) twice, with the package's implementation file mutated
// in between (mirroring Probe C exactly: only impl.go changes, never
// impl_test.go -- the validation is gutted to an unconditional `return
// nil`). The first run must PASS (real implementation); the second must
// FAIL, and FAIL as a real test failure, not a compile failure -- proving
// BOTH that RunVerifiedByTest actually executes the test (not just AST-
// inspects it) AND that the content-hash cache (hashPackageInputs, keyed by
// every *.go file in the package directory, not just the two named files)
// is invalidated by an impl-only edit, never just a test-file edit. A cache
// keyed ONLY on the test file's own content (a plausible but wrong
// simplification) would wrongly return the stale PASSED result for the
// second call -- exactly the false-clean outcome @fh finding F1 reported.
func TestRunVerifiedByTest_MUTATION_CacheInvalidatesOnImplChange(t *testing.T) {
	ResetRunCacheForTest()
	root := writeModuleFixture(t, "example.com/mutatemod", "model", passingImplSrc, passingTestSrc)

	before := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_RejectsZeroFields")
	if before.Err != nil {
		t.Fatalf("unexpected infra error (before): %v", before.Err)
	}
	if !before.Passed {
		t.Fatalf("expected Passed=true before mutation, got %+v", before)
	}

	implPath := filepath.Join(root, "model", "impl.go")
	if err := os.WriteFile(implPath, []byte(guttedImplSrc), 0o644); err != nil {
		t.Fatalf("WriteFile gutted impl.go: %v", err)
	}

	after := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_RejectsZeroFields")
	if after.Err != nil {
		t.Fatalf("unexpected infra error (after): %v", after.Err)
	}
	if after.Passed {
		t.Fatalf("expected Passed=false after gutting the implementation (cache must invalidate on impl-file change), got %+v", after)
	}
	if after.CompileFailed {
		t.Fatalf("expected a real test failure after gutting, not a compile failure, got %+v", after)
	}
}

func TestRunVerifiedByTest_CompileFailure_ReportsCompileFailedNotPanic(t *testing.T) {
	ResetRunCacheForTest()
	root := writeModuleFixture(t, "example.com/badsyntaxmod", "model", uncompilableImplSrc, passingTestSrc)
	result := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_RejectsZeroFields")
	if result.Err != nil {
		t.Fatalf("unexpected infra error (a syntax error is a TEST verdict, not infra failure): %v", result.Err)
	}
	if result.Passed {
		t.Fatalf("expected Passed=false for a package that does not compile, got %+v", result)
	}
	if !result.CompileFailed {
		t.Fatalf("expected CompileFailed=true for a syntax error, got %+v", result)
	}
}

func TestRunVerifiedByTest_NoGoModFound_ReturnsInfraError(t *testing.T) {
	t.Parallel()
	ResetRunCacheForTest()
	tmp := t.TempDir()
	// No go.mod anywhere under tmp, and tmp is isolated from any real
	// module (t.TempDir() never sits under this repo's own go.mod).
	pkgDir := filepath.Join(tmp, "spec", "model")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "impl_test.go"), []byte(passingTestSrc), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	result := RunVerifiedByTest(tmp, "spec/model/impl_test.go", "TestRequireComplete_RejectsZeroFields")
	if result.Err == nil {
		t.Fatalf("expected an infrastructure error when no go.mod can be found, got %+v", result)
	}
}

func TestRelativePackagePattern_OK(t *testing.T) {
	t.Parallel()
	root := filepath.FromSlash("/repo")
	file := filepath.FromSlash("/repo/spec/model/risk_test.go")
	pattern, err := relativePackagePattern(root, file)
	if err != nil {
		t.Fatalf("relativePackagePattern: %v", err)
	}
	if pattern != "./spec/model/" {
		t.Fatalf("expected './spec/model/', got %q", pattern)
	}
}

func TestRelativePackagePattern_ModuleRootItself(t *testing.T) {
	t.Parallel()
	root := filepath.FromSlash("/repo")
	file := filepath.FromSlash("/repo/risk_test.go")
	pattern, err := relativePackagePattern(root, file)
	if err != nil {
		t.Fatalf("relativePackagePattern: %v", err)
	}
	if pattern != "./" {
		t.Fatalf("expected './', got %q", pattern)
	}
}

func TestHashPackageInputs_DifferentContentDifferentHash(t *testing.T) {
	t.Parallel()
	root := writeModuleFixture(t, "example.com/hashmod", "model", passingImplSrc, passingTestSrc)
	pkgDir := filepath.Join(root, "model")

	h1, err := hashPackageInputs(root, pkgDir)
	if err != nil {
		t.Fatalf("hashPackageInputs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "impl.go"), []byte(guttedImplSrc), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	h2, err := hashPackageInputs(root, pkgDir)
	if err != nil {
		t.Fatalf("hashPackageInputs (after mutation): %v", err)
	}
	if h1 == h2 {
		t.Fatalf("expected different hashes before/after mutating impl.go, got the same hash %q both times", h1)
	}
}

func TestHashPackageInputs_UnchangedContentSameHash(t *testing.T) {
	t.Parallel()
	root := writeModuleFixture(t, "example.com/stablemod", "model", passingImplSrc, passingTestSrc)
	pkgDir := filepath.Join(root, "model")

	h1, err := hashPackageInputs(root, pkgDir)
	if err != nil {
		t.Fatalf("hashPackageInputs: %v", err)
	}
	h2, err := hashPackageInputs(root, pkgDir)
	if err != nil {
		t.Fatalf("hashPackageInputs (second call): %v", err)
	}
	if h1 != h2 {
		t.Fatalf("expected the same hash across two calls with no file changes, got %q then %q", h1, h2)
	}
}

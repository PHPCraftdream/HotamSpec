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

// writeModelPolicyFixture builds a two-PACKAGE module (model/ and policy/,
// both siblings under the same go.mod, mirroring PLAN-authored-spec-
// discipline.md §8's own prescribed spec/model + spec/application + spec/
// policy layout): policy/ exports Validate(fields int) bool, and model/
// imports policy and its own test (TestRequireComplete_UsesPolicy) asserts
// Validate's real verdict. Returns the module root, the model package dir,
// and the policy impl.go path (the file the mutation test below gut-edits).
func writeModelPolicyFixture(t *testing.T, modulePath string) (moduleRoot, modelDir, policyImplPath string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	policyDir := filepath.Join(root, "policy")
	if err := os.MkdirAll(policyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll policyDir: %v", err)
	}
	policySrc := `package policy

// Validate is the real gate: fields below 1 are invalid.
func Validate(fields int) bool {
	return fields >= 1
}
`
	policyPath := filepath.Join(policyDir, "impl.go")
	if err := os.WriteFile(policyPath, []byte(policySrc), 0o644); err != nil {
		t.Fatalf("WriteFile policy/impl.go: %v", err)
	}

	modelDirPath := filepath.Join(root, "model")
	if err := os.MkdirAll(modelDirPath, 0o755); err != nil {
		t.Fatalf("MkdirAll modelDir: %v", err)
	}
	modelSrc := `package model

import "` + modulePath + `/policy"

// RequireComplete delegates to policy.Validate -- model/ itself never
// changes; only policy/'s behavior determines the verdict.
func RequireComplete(fields int) error {
	if !policy.Validate(fields) {
		return errNotComplete
	}
	return nil
}

var errNotComplete = errStub{}

type errStub struct{}

func (errStub) Error() string { return "not complete" }
`
	if err := os.WriteFile(filepath.Join(modelDirPath, "impl.go"), []byte(modelSrc), 0o644); err != nil {
		t.Fatalf("WriteFile model/impl.go: %v", err)
	}
	modelTestSrc := `package model

import "testing"

func TestRequireComplete_UsesPolicy(t *testing.T) {
	if err := RequireComplete(0); err == nil {
		t.Fatalf("expected error for zero fields (policy.Validate should reject), got nil")
	}
}
`
	if err := os.WriteFile(filepath.Join(modelDirPath, "impl_test.go"), []byte(modelTestSrc), 0o644); err != nil {
		t.Fatalf("WriteFile model/impl_test.go: %v", err)
	}

	return root, modelDirPath, policyPath
}

// TestRunVerifiedByTest_MUTATION_NEW2_SiblingPackageChangeInvalidatesCache is
// @fh's NEW-2 finding reproduced at unit-test granularity: a verified_by test
// lives in model/, but the requirement it proves transitively depends on a
// SIBLING package (policy/) per PLAN-authored-spec-discipline.md §8's own
// model/application/policy split. The ORIGINAL hashPackageInputs hashed only
// the named test's OWN package directory (model/) -- a behavioral mutation to
// policy/'s source (a different directory) never touched that hash, so the
// disk/in-memory cache kept serving the stale PASSED verdict even after `go
// test ./model/` would, if actually re-run, fail. This test proves the fix
// (NEW-2 design B, whole-module hashing): run once (PASS, cache warmed),
// mutate ONLY policy/impl.go (gut Validate to always return true -- model/'s
// own files are untouched), run again with the SAME (file, test) key -- the
// second call MUST actually re-execute and report Passed=false, not silently
// replay the stale cached PASS.
func TestRunVerifiedByTest_MUTATION_NEW2_SiblingPackageChangeInvalidatesCache(t *testing.T) {
	ResetRunCacheForTest()
	root, _, policyImplPath := writeModelPolicyFixture(t, "example.com/new2mod")

	before := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_UsesPolicy")
	if before.Err != nil {
		t.Fatalf("unexpected infra error (before): %v", before.Err)
	}
	if !before.Passed {
		t.Fatalf("expected Passed=true before mutating the sibling policy/ package, got %+v", before)
	}

	// Mutate ONLY policy/impl.go -- model/'s directory (the test's OWN
	// package) is completely untouched, exactly the shape pkgDir-only
	// hashing could never observe.
	guttedPolicySrc := `package policy

// Validate is GUTTED -- always reports valid, regardless of fields.
func Validate(fields int) bool {
	return true
}
`
	if err := os.WriteFile(policyImplPath, []byte(guttedPolicySrc), 0o644); err != nil {
		t.Fatalf("WriteFile gutted policy/impl.go: %v", err)
	}

	after := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_UsesPolicy")
	if after.Err != nil {
		t.Fatalf("unexpected infra error (after): %v", after.Err)
	}
	if after.Passed {
		t.Fatalf("STALE-GREEN (NEW-2): expected Passed=false after gutting sibling package policy/ (cache must invalidate across the package boundary), got %+v", after)
	}
	if after.CompileFailed {
		t.Fatalf("expected a real test failure after gutting policy/, not a compile failure, got %+v", after)
	}
}

// TestHashPackageInputs_NEW2_SiblingPackageChangeChangesModuleHash is the
// hashPackageInputs-level companion proof: mutating a *.go file in a SIBLING
// package directory (policy/), never the named pkgDir (model/) itself, MUST
// still change the digest -- this is what whole-module hashing (design B)
// buys over the original pkgDir-only hashing, and is the direct mechanical
// reason TestRunVerifiedByTest_MUTATION_NEW2_SiblingPackageChangeInvalidatesCache
// above observes a real re-run instead of a stale cache hit.
func TestHashPackageInputs_NEW2_SiblingPackageChangeChangesModuleHash(t *testing.T) {
	t.Parallel()
	root, modelDir, policyImplPath := writeModelPolicyFixture(t, "example.com/new2hashmod")

	h1, err := hashPackageInputs(root, modelDir)
	if err != nil {
		t.Fatalf("hashPackageInputs: %v", err)
	}

	guttedPolicySrc := `package policy

func Validate(fields int) bool {
	return true
}
`
	if err := os.WriteFile(policyImplPath, []byte(guttedPolicySrc), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	h2, err := hashPackageInputs(root, modelDir)
	if err != nil {
		t.Fatalf("hashPackageInputs (after sibling mutation): %v", err)
	}
	if h1 == h2 {
		t.Fatalf("expected a different module hash after mutating sibling package policy/impl.go (pkgDir=model/ itself untouched), got the same hash %q both times -- this is exactly the NEW-2 stale-green shape", h1)
	}
}

// TestRunVerifiedByTest_MUTATION_NEW1_ExternalGuardEnvDoesNotSilentlySkip is
// @fh's NEW-1 finding reproduced at unit-test granularity, at the level this
// package alone can prove (the FULL CLI-level reproduction --
// `HOTAM_VERIFIED_BY_EXEC_GUARD=<anything> hotam all-violations` genuinely
// spawning a top-level `hotam` process and observing a real violation -- lives
// in cmd/hotam's own e2e suite, since the actual root-cause fix is
// cmd/hotam/main's unconditional os.Unsetenv, not anything in this package).
// At THIS package's level, inRecursionGuard is now a bare presence check
// (marker-vouching was removed -- see recursionGuardEnv's doc comment for why
// it was forgeable and is no longer needed): a directly-set guard env DOES
// get honored as recursion by this package alone, exactly as intended for a
// genuine `go test` child. This test proves the OTHER guaranteed-loud half of
// the contract instead: whenever RunVerifiedByTest DOES honor the guard and
// Skip, it is never silent -- Skipped=true always carries a non-empty
// InfraWarning, so a caller can never mistake a skipped, unproven entry for a
// quietly-passed one.
func TestRunVerifiedByTest_MUTATION_NEW1_HonoredSkipAlwaysCarriesInfraWarning(t *testing.T) {
	ResetRunCacheForTest()

	root := writeModuleFixture(t, "example.com/new1mod", "model", guttedImplSrc, passingTestSrc)

	prev, hadPrev := os.LookupEnv(recursionGuardEnv)
	if err := os.Setenv(recursionGuardEnv, newGuardNonce()); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	t.Cleanup(func() {
		if hadPrev {
			os.Setenv(recursionGuardEnv, prev)
		} else {
			os.Unsetenv(recursionGuardEnv)
		}
	})

	result := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_RejectsZeroFields")

	if !result.Skipped {
		t.Fatalf("expected Skipped=true with the guard env set (this package's own inRecursionGuard honors any non-empty value -- the CLI-side os.Unsetenv is what prevents an EXTERNAL actor from ever reaching this state at a `hotam` process's top level), got %+v", result)
	}
	if result.InfraWarning == "" {
		t.Fatalf("SILENT HONORED SKIP (NEW-1, second re-review): a Skipped result must always carry a non-empty InfraWarning -- 'clean' must never look identical to 'silently deferred', got %+v", result)
	}
}

// TestInRecursionGuard_NoEnv_False proves the baseline: with no guard env set
// at all, inRecursionGuard must report false (a genuine root process runs its
// checks for real).
func TestInRecursionGuard_NoEnv_False(t *testing.T) {
	prev, hadPrev := os.LookupEnv(recursionGuardEnv)
	os.Unsetenv(recursionGuardEnv)
	t.Cleanup(func() {
		if hadPrev {
			os.Setenv(recursionGuardEnv, prev)
		}
	})
	if inRecursionGuard() {
		t.Fatalf("expected inRecursionGuard()==false with no guard env set")
	}
}

// TestInRecursionGuard_AnyNonEmptyEnv_Honored proves inRecursionGuard's
// current, deliberately simple contract: ANY non-empty recursionGuardEnv
// value is honored as genuine recursion at THIS package's level -- no marker
// file, no corroborating secret. This is sound only because the OTHER half of
// the NEW-1 fix (cmd/hotam/main's unconditional ClearInheritedRecursionGuard
// call, proven by TestMain_ClearsInheritedRecursionGuardBeforeDispatch in
// cmd/hotam) guarantees no top-level `hotam` process can ever observe an
// externally-inherited value here -- see recursionGuardEnv's doc comment.
func TestInRecursionGuard_AnyNonEmptyEnv_Honored(t *testing.T) {
	nonce := newGuardNonce()

	prev, hadPrev := os.LookupEnv(recursionGuardEnv)
	if err := os.Setenv(recursionGuardEnv, nonce); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	t.Cleanup(func() {
		if hadPrev {
			os.Setenv(recursionGuardEnv, prev)
		} else {
			os.Unsetenv(recursionGuardEnv)
		}
	})

	if !inRecursionGuard() {
		t.Fatalf("expected inRecursionGuard()==true for any non-empty guard-env value")
	}
}

// writeEmbedThresholdFixture builds a standalone temp Go module whose test
// verdict depends on a NON-.go file: threshold.txt is pulled in via
// //go:embed at compile time, parsed as an int, and the test asserts
// RequireComplete(fields) against that embedded threshold rather than a
// hardcoded literal. This is the NEW-4 mutation shape: editing ONLY
// threshold.txt (never impl.go, never impl_test.go) must be able to flip the
// test's real PASS/FAIL verdict, which is exactly the case the pre-fix
// *.go-suffix-only hashPackageInputs walk could never observe.
func writeEmbedThresholdFixture(t *testing.T, modulePath, pkgRelDir, thresholdContent string) (moduleRoot, thresholdPath string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	pkgDir := filepath.Join(root, filepath.FromSlash(pkgRelDir))
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("MkdirAll pkgDir: %v", err)
	}
	implSrc := `package model

import (
	_ "embed"
	"strconv"
	"strings"
)

//go:embed threshold.txt
var thresholdRaw string

// Threshold parses the embedded threshold.txt at call time (not init time)
// so a change to the embedded file's CONTENT (this whole file is recompiled
// whenever it changes, since //go:embed bakes the file's bytes into the
// compiled binary) is observed by a fresh ` + "`go test`" + ` invocation of this
// package -- exactly the input hashPackageInputs must also observe.
func Threshold() int {
	n, err := strconv.Atoi(strings.TrimSpace(thresholdRaw))
	if err != nil {
		return -1
	}
	return n
}

// RequireComplete is valid only when fields is AT LEAST the embedded
// threshold -- so editing threshold.txt alone (never this file, never the
// test file) can flip whether a given fields value passes.
func RequireComplete(fields int) error {
	if fields < Threshold() {
		return errNotComplete
	}
	return nil
}

var errNotComplete = errStub{}

type errStub struct{}

func (errStub) Error() string { return "not complete" }
`
	if err := os.WriteFile(filepath.Join(pkgDir, "impl.go"), []byte(implSrc), 0o644); err != nil {
		t.Fatalf("WriteFile impl.go: %v", err)
	}
	testSrc := `package model

import "testing"

// TestRequireComplete_MeetsEmbeddedThreshold asserts fields=5 satisfies
// RequireComplete -- true only while threshold.txt's embedded value is <= 5.
func TestRequireComplete_MeetsEmbeddedThreshold(t *testing.T) {
	if err := RequireComplete(5); err != nil {
		t.Fatalf("expected fields=5 to satisfy the embedded threshold, got error: %v", err)
	}
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "impl_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatalf("WriteFile impl_test.go: %v", err)
	}
	thresholdPath = filepath.Join(pkgDir, "threshold.txt")
	if err := os.WriteFile(thresholdPath, []byte(thresholdContent), 0o644); err != nil {
		t.Fatalf("WriteFile threshold.txt: %v", err)
	}
	return root, thresholdPath
}

// TestRunVerifiedByTest_MUTATION_NEW4_EmbeddedNonGoFileChangeInvalidatesCache
// is the NEW-4 finding reproduced end-to-end through RunVerifiedByTest's real
// cache: a verified_by test's real PASS/FAIL verdict depends on a //go:embed
// non-.go file (threshold.txt), never on any *.go file's content changing.
// Before the fix, hashPackageInputs walked only files whose name ends in
// ".go", so threshold.txt was invisible to the cache key -- warm the cache
// with threshold="3" (fields=5 >= 3, PASS), then edit ONLY threshold.txt to
// "10" (fields=5 < 10, the SAME compiled test now fails for real) and call
// RunVerifiedByTest again with the identical (file, test) key. Pre-fix this
// returned the stale cached PASS (a stale-green vector: the cache never saw
// the file that determines the verdict change at all). Post-fix the digest
// must move, forcing a real re-run that reports Passed=false.
func TestRunVerifiedByTest_MUTATION_NEW4_EmbeddedNonGoFileChangeInvalidatesCache(t *testing.T) {
	ResetRunCacheForTest()
	root, thresholdPath := writeEmbedThresholdFixture(t, "example.com/new4mod", "model", "3\n")

	before := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_MeetsEmbeddedThreshold")
	if before.Err != nil {
		t.Fatalf("unexpected infra error (before): %v", before.Err)
	}
	if !before.Passed {
		t.Fatalf("expected Passed=true before mutating threshold.txt (fields=5 >= threshold=3), got %+v", before)
	}

	// Mutate ONLY the embedded non-.go file -- impl.go and impl_test.go are
	// completely untouched, exactly the shape a *.go-suffix-only hash walk
	// could never observe.
	if err := os.WriteFile(thresholdPath, []byte("10\n"), 0o644); err != nil {
		t.Fatalf("WriteFile mutated threshold.txt: %v", err)
	}

	after := RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_MeetsEmbeddedThreshold")
	if after.Err != nil {
		t.Fatalf("unexpected infra error (after): %v", after.Err)
	}
	if after.Passed {
		t.Fatalf("STALE-GREEN (NEW-4): expected Passed=false after raising the embedded threshold.txt past fields=5 (cache must invalidate on a non-.go input change), got %+v", after)
	}
	if after.CompileFailed {
		t.Fatalf("expected a real test failure after raising the threshold, not a compile failure, got %+v", after)
	}
}

// TestHashPackageInputs_NEW4_NonGoFileChangeChangesModuleHash is the
// hashPackageInputs-level companion proof: mutating a NON-.go file
// (threshold.txt) under moduleRoot, with every *.go file byte-for-byte
// unchanged, MUST still change the digest -- the direct mechanical reason
// TestRunVerifiedByTest_MUTATION_NEW4_EmbeddedNonGoFileChangeInvalidatesCache
// above observes a real re-run instead of a stale cache hit.
func TestHashPackageInputs_NEW4_NonGoFileChangeChangesModuleHash(t *testing.T) {
	t.Parallel()
	root, thresholdPath := writeEmbedThresholdFixture(t, "example.com/new4hashmod", "model", "3\n")

	h1, err := hashPackageInputs(root, filepath.Join(root, "model"))
	if err != nil {
		t.Fatalf("hashPackageInputs: %v", err)
	}

	if err := os.WriteFile(thresholdPath, []byte("10\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	h2, err := hashPackageInputs(root, filepath.Join(root, "model"))
	if err != nil {
		t.Fatalf("hashPackageInputs (after non-.go mutation): %v", err)
	}
	if h1 == h2 {
		t.Fatalf("expected a different module hash after mutating the non-.go file threshold.txt (every *.go file untouched), got the same hash %q both times -- this is exactly the NEW-4 stale-green shape", h1)
	}
}

// TestClearInheritedRecursionGuard_UnsetsEnv is the direct unit proof for the
// exported function cmd/hotam's main() calls at CLI entry: given a
// (simulated-externally-forged) non-empty recursionGuardEnv, calling
// ClearInheritedRecursionGuard must leave it unset, and inRecursionGuard must
// then report false.
func TestClearInheritedRecursionGuard_UnsetsEnv(t *testing.T) {
	prev, hadPrev := os.LookupEnv(recursionGuardEnv)
	if err := os.Setenv(recursionGuardEnv, "forged-external-value"); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	t.Cleanup(func() {
		if hadPrev {
			os.Setenv(recursionGuardEnv, prev)
		} else {
			os.Unsetenv(recursionGuardEnv)
		}
	})

	ClearInheritedRecursionGuard()

	if v, ok := os.LookupEnv(recursionGuardEnv); ok {
		t.Fatalf("expected %s to be unset after ClearInheritedRecursionGuard, got %q", recursionGuardEnv, v)
	}
	if inRecursionGuard() {
		t.Fatalf("expected inRecursionGuard()==false after ClearInheritedRecursionGuard")
	}
}

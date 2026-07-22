package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
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

// TestRecordDirEnvName_MatchesCanonLiteral is the mechanical guard named in
// recordDirEnvName's own doc comment: this package intentionally does NOT
// import internal/recorder/canon (to avoid coupling the engine-internal gate
// package to the vendored-recorder canon), so the two sides of the
// HOTAM_RECORD_DIR contract are two independent string literals that could
// silently drift. recordervendor.BodyForHash() re-exposes the canon's exact
// source text (it is a go:embed of that same file, one hop away via
// internal/recorder/vendor -- itself already an existing, approved
// dependency direction, see that package's own doc comment), so this test
// greps for the literal RecordDirEnv assignment inside the real canon source
// and compares it against this package's own recordDirEnvName constant.
func TestRecordDirEnvName_MatchesCanonLiteral(t *testing.T) {
	canonSrc := recordervendor.BodyForHash()
	want := `"` + recordDirEnvName + `"`
	if !strings.Contains(canonSrc, "RecordDirEnv = "+want) {
		t.Fatalf("canon source does not declare RecordDirEnv = %s -- recordDirEnvName (%q) has drifted from internal/recorder/canon/hotamspec.go's own RecordDirEnv literal", want, recordDirEnvName)
	}
}

// writeRecordingFixture builds a standalone temp Go module that mirrors
// EXACTLY what a real consumer domain's spec/ tree looks like once the
// recorder has been vendored (PLAN-scenario-generated-spec.md §2 D1): a
// go.mod, an implementation package (implPkgRelDir/impl.go, the
// implemented_by symbol's home), and a model package
// (modelPkgRelDir/impl_test.go) that imports "<modulePath>/hotamspec" -- a
// LOCAL package inside this same fixture module, populated with the REAL
// canonical recorder source (recordervendor.BodyForHash(), the exact bytes
// `hotam vendor-recorder` would write into a real domain, banner aside) so
// this test exercises the genuine hotamspec.NewScenario/Given/When/Then/
// Value API, not a hand-rolled stand-in that could drift from it. testSrc is
// the verified_by test's own source, free to reference "hotamspec" via that
// local import.
func writeRecordingFixture(t *testing.T, modulePath, implPkgRelDir, implSrc, modelPkgRelDir, testSrc string) (moduleRoot string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	implDir := filepath.Join(root, filepath.FromSlash(implPkgRelDir))
	if err := os.MkdirAll(implDir, 0o755); err != nil {
		t.Fatalf("MkdirAll implDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(implDir, "impl.go"), []byte(implSrc), 0o644); err != nil {
		t.Fatalf("WriteFile impl.go: %v", err)
	}

	hotamspecDir := filepath.Join(root, "hotamspec")
	if err := os.MkdirAll(hotamspecDir, 0o755); err != nil {
		t.Fatalf("MkdirAll hotamspecDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hotamspecDir, "hotamspec.go"), []byte(recordervendor.BodyForHash()), 0o644); err != nil {
		t.Fatalf("WriteFile vendored hotamspec.go: %v", err)
	}

	modelDir := filepath.Join(root, filepath.FromSlash(modelPkgRelDir))
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll modelDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "impl_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatalf("WriteFile impl_test.go: %v", err)
	}

	return root
}

// scenarioImplSrc is the implemented_by symbol under test: a tiny
// RequireComplete function, deliberately with MORE THAN ONE executable
// branch (an if/else, not a single unconditional return) so the coverage
// profile a real run produces has more than one counted statement block to
// assert on.
const scenarioImplSrc = `package model

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

// scenarioTestSrc is the verified_by test: a real hotamspec.Scenario-based
// test (Given/When/Then/Value), calling the REAL RequireComplete symbol
// (imported from the sibling model package) so both the recorder's record-
// mode wiring AND the coverage profile prove genuine execution of that
// symbol's lines, not just of the test file itself.
func scenarioTestSrc(modulePath string) string {
	return `package model

import (
	"testing"

	"` + modulePath + `/hotamspec"
)

func TestRequireComplete_ScenarioRecorded(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-example-recording", "RequireComplete rejects zero fields")
	s.Given("a fields count of zero", "fields", 0)
	err := RequireComplete(0)
	s.When("RequireComplete is called")
	s.Then("an error is returned", err != nil)
	s.Value("error_text", err)
}
`
}

// TestRunVerifiedByTestRecording_RealScenario_AssertPlusArtifactPlusCoverage
// is the end-to-end proof this task's own verification step calls for: ONE
// RunVerifiedByTestRecording call against a real vendored-recorder fixture
// (a) reports Passed=true (the plain-assert half, unchanged from
// RunVerifiedByTest's own contract), (b) returns exactly one artifact whose
// JSON matches the expected canonical shape (req_id/test/title/steps/
// verdict), and (c) returns a non-empty coverage profile whose text mentions
// the implemented_by package/file, proving the SAME run that produced (a)
// and (b) also exercised RequireComplete's lines.
func TestRunVerifiedByTestRecording_RealScenario_AssertPlusArtifactPlusCoverage(t *testing.T) {
	const modulePath = "example.com/recordmod"
	root := writeRecordingFixture(t, modulePath, "model", scenarioImplSrc, "model", scenarioTestSrc(modulePath))

	result := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestRequireComplete_ScenarioRecorded", "model/impl.go")

	if result.Err != nil {
		t.Fatalf("unexpected infra error: %v\noutput:\n%s", result.Err, result.Output)
	}
	if result.Skipped {
		t.Fatalf("unexpected Skipped=true: %+v", result.TestRunResult)
	}
	if !result.Passed {
		t.Fatalf("expected Passed=true for a genuinely passing scenario test, got %+v\noutput:\n%s", result.TestRunResult, result.Output)
	}
	if result.CompileFailed {
		t.Fatalf("unexpected CompileFailed=true, output:\n%s", result.Output)
	}

	if len(result.Artifacts) != 1 {
		t.Fatalf("expected exactly 1 artifact, got %d: %+v", len(result.Artifacts), result.Artifacts)
	}
	art := result.Artifacts[0]
	wantName := "R-example-recording__TestRequireComplete_ScenarioRecorded.json"
	if art.FileName != wantName {
		t.Errorf("artifact FileName = %q, want %q", art.FileName, wantName)
	}

	var parsed struct {
		ReqID string `json:"req_id"`
		Test  string `json:"test"`
		Title string `json:"title"`
		Steps []struct {
			Kind   string `json:"kind"`
			Desc   string `json:"desc"`
			Values []struct {
				K string `json:"k"`
				V string `json:"v"`
			} `json:"values,omitempty"`
			Passed bool `json:"passed,omitempty"`
		} `json:"steps"`
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal(art.RawJSON, &parsed); err != nil {
		t.Fatalf("artifact RawJSON is not valid JSON: %v\n%s", err, art.RawJSON)
	}
	if parsed.ReqID != "R-example-recording" {
		t.Errorf("req_id = %q, want R-example-recording", parsed.ReqID)
	}
	if parsed.Test != "TestRequireComplete_ScenarioRecorded" {
		t.Errorf("test = %q, want TestRequireComplete_ScenarioRecorded", parsed.Test)
	}
	if parsed.Verdict != "pass" {
		t.Errorf("verdict = %q, want pass", parsed.Verdict)
	}
	if len(parsed.Steps) != 4 {
		t.Fatalf("steps len = %d, want 4: %+v", len(parsed.Steps), parsed.Steps)
	}
	if parsed.Steps[0].Kind != "given" || len(parsed.Steps[0].Values) != 1 || parsed.Steps[0].Values[0].K != "fields" || parsed.Steps[0].Values[0].V != "0" {
		t.Errorf("steps[0] = %+v, want given fields=0", parsed.Steps[0])
	}
	if parsed.Steps[2].Kind != "then" || !parsed.Steps[2].Passed {
		t.Errorf("steps[2] = %+v, want then passed=true", parsed.Steps[2])
	}

	if len(result.CoverProfile) == 0 {
		t.Fatalf("expected a non-empty coverage profile")
	}
	coverText := string(result.CoverProfile)
	if !strings.HasPrefix(coverText, "mode:") {
		t.Errorf("coverage profile does not start with a mode: header:\n%s", coverText)
	}
	if !strings.Contains(coverText, filepath.ToSlash(modulePath+"/model/impl.go")) {
		t.Errorf("coverage profile does not mention the implemented_by file %s/model/impl.go:\n%s", modulePath, coverText)
	}
	// RequireComplete has 2 branches (if/else via early return) -- a real
	// execution with fields=0 covers the "return errNotComplete" branch;
	// prove at least one non-zero hit COUNT is present (the last
	// whitespace-separated field on a coverage line), not just that the
	// package NAME appears in the profile (which alone would not prove any
	// STATEMENT was actually executed).
	if !hasNonZeroCoverageCount(coverText) {
		t.Errorf("coverage profile has no line with a non-zero execution count -- implemented_by lines were not actually proven executed:\n%s", coverText)
	}
}

// hasNonZeroCoverageCount reports whether cover (Go's text coverage-profile
// format: "file:startLine.startCol,endLine.endCol numStmt count" per line,
// after the leading "mode:" line) contains at least one line whose trailing
// count field is non-zero -- the direct proof that some statement block was
// actually executed, not merely instrumented.
func hasNonZeroCoverageCount(cover string) bool {
	for _, line := range strings.Split(cover, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		count := fields[len(fields)-1]
		if count != "0" {
			return true
		}
	}
	return false
}

// TestRunVerifiedByTestRecording_Deterministic_TwoRunsByteIdentical is the
// hard determinism proof PLAN-scenario-generated-spec.md §2 D1/the task's own
// verification step demands: running the identical scenario test TWICE via
// RunVerifiedByTestRecording (two separate calls, two separate per-run tmp
// dirs, exactly as two independent `hotam` invocations would each get their
// own) must produce byte-identical artifact JSON both times -- proven here by
// sha256, and the exact byte comparison too.
func TestRunVerifiedByTestRecording_Deterministic_TwoRunsByteIdentical(t *testing.T) {
	const modulePath = "example.com/determinismmod"
	root := writeRecordingFixture(t, modulePath, "model", scenarioImplSrc, "model", scenarioTestSrc(modulePath))

	first := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestRequireComplete_ScenarioRecorded", "model/impl.go")
	if first.Err != nil || !first.Passed {
		t.Fatalf("first run: unexpected result %+v, err=%v", first.TestRunResult, first.Err)
	}
	second := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestRequireComplete_ScenarioRecorded", "model/impl.go")
	if second.Err != nil || !second.Passed {
		t.Fatalf("second run: unexpected result %+v, err=%v", second.TestRunResult, second.Err)
	}

	if len(first.Artifacts) != 1 || len(second.Artifacts) != 1 {
		t.Fatalf("expected exactly 1 artifact per run, got first=%d second=%d", len(first.Artifacts), len(second.Artifacts))
	}
	firstBytes := first.Artifacts[0].RawJSON
	secondBytes := second.Artifacts[0].RawJSON
	firstHash := sha256Hex(firstBytes)
	secondHash := sha256Hex(secondBytes)
	t.Logf("record-mode artifact sha256 run1=%s run2=%s", firstHash, secondHash)
	if firstHash != secondHash {
		t.Fatalf("DETERMINISM VIOLATION: two record-mode runs of the identical scenario produced different artifact bytes\nrun1 sha256=%s:\n%s\nrun2 sha256=%s:\n%s", firstHash, firstBytes, secondHash, secondBytes)
	}
	if string(firstBytes) != string(secondBytes) {
		t.Fatalf("DETERMINISM VIOLATION: hashes matched but raw bytes differ (should be impossible) -- run1:\n%s\nrun2:\n%s", firstBytes, secondBytes)
	}
}

// sha256Hex is this test file's own tiny local helper (mirrors the
// unexported same-named helper in internal/invariants/recorder_check.go --
// duplicated rather than exported/shared across packages for a one-line
// hex-encode, matching that file's own reasoning for why it stayed a small
// private helper there).
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// TestRunVerifiedByTestRecording_NoCoverPkg_SkipsCoverageCleanly proves the
// coverPkgFile="" escape hatch: RunVerifiedByTestRecording must still run and
// record an artifact normally, with CoverProfile left nil (no -coverprofile
// flag passed at all), when the caller does not ask for coverage.
func TestRunVerifiedByTestRecording_NoCoverPkg_SkipsCoverageCleanly(t *testing.T) {
	const modulePath = "example.com/nocovermod"
	root := writeRecordingFixture(t, modulePath, "model", scenarioImplSrc, "model", scenarioTestSrc(modulePath))

	result := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestRequireComplete_ScenarioRecorded", "")

	if result.Err != nil || !result.Passed {
		t.Fatalf("unexpected result %+v, err=%v, output:\n%s", result.TestRunResult, result.Err, result.Output)
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("expected exactly 1 artifact even with coverage skipped, got %d", len(result.Artifacts))
	}
	if result.CoverProfile != nil {
		t.Errorf("expected CoverProfile=nil when coverPkgFile is empty, got %d bytes", len(result.CoverProfile))
	}
}

// TestRunVerifiedByTestRecording_TmpDirCleanedUpAfterReturn proves the
// per-run tmp dir contract (task requirement: "artifacts... read into memory,
// tmp deleted" -- no persistent disk trace once RunVerifiedByTestRecording
// returns). This test cannot directly observe the tmp dir's own path (it is
// internal), so it instead asserts on os.TempDir()'s own root: no directory
// matching the "hotam-record-*" prefix this function uses may survive after
// a call returns.
func TestRunVerifiedByTestRecording_TmpDirCleanedUpAfterReturn(t *testing.T) {
	const modulePath = "example.com/tmpcleanupmod"
	root := writeRecordingFixture(t, modulePath, "model", scenarioImplSrc, "model", scenarioTestSrc(modulePath))

	before, err := filepath.Glob(filepath.Join(os.TempDir(), "hotam-record-*"))
	if err != nil {
		t.Fatalf("Glob (before): %v", err)
	}

	result := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestRequireComplete_ScenarioRecorded", "model/impl.go")
	if result.Err != nil || !result.Passed {
		t.Fatalf("unexpected result %+v, err=%v", result.TestRunResult, result.Err)
	}

	after, err := filepath.Glob(filepath.Join(os.TempDir(), "hotam-record-*"))
	if err != nil {
		t.Fatalf("Glob (after): %v", err)
	}
	if len(after) > len(before) {
		t.Fatalf("expected no surviving hotam-record-* tmp dirs after RunVerifiedByTestRecording returns: before=%v after=%v", before, after)
	}
}

// TestRunVerifiedByTestRecording_RecursionGuard_Skips proves
// RunVerifiedByTestRecording honors the SAME recursion guard
// RunVerifiedByTest does -- a process already nested inside a guarded `go
// test` child must not spawn yet another generation, in record-mode exactly
// as in plain mode.
func TestRunVerifiedByTestRecording_RecursionGuard_Skips(t *testing.T) {
	const modulePath = "example.com/recordguardmod"
	root := writeRecordingFixture(t, modulePath, "model", scenarioImplSrc, "model", scenarioTestSrc(modulePath))

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

	result := RunVerifiedByTestRecording(root, "model/impl_test.go", "TestRequireComplete_ScenarioRecorded", "model/impl.go")
	if !result.Skipped {
		t.Fatalf("expected Skipped=true with the guard env set, got %+v", result.TestRunResult)
	}
	if result.InfraWarning == "" {
		t.Fatalf("expected a non-empty InfraWarning on a Skipped record-mode result")
	}
	if len(result.Artifacts) != 0 {
		t.Fatalf("expected no artifacts on a Skipped run, got %d", len(result.Artifacts))
	}
}

// --- F6: artifact shape validation + req_id cross-check (task W7.2) ---------

// TestLooksLikeRecorderArtifact_ValidShape proves the shape checker accepts a
// genuine recorder-produced artifact.
func TestLooksLikeRecorderArtifact_ValidShape(t *testing.T) {
	t.Parallel()
	genuine := []byte(`{
  "req_id": "R-test",
  "test": "TestFoo",
  "title": "Foo does bar",
  "steps": [
    {"kind": "given", "desc": "a precondition", "values": [{"k": "x", "v": "1"}]}
  ],
  "verdict": "pass"
}` + "\n")
	if !looksLikeRecorderArtifact(genuine) {
		t.Fatalf("expected looksLikeRecorderArtifact to accept a genuine recorder artifact")
	}
}

// TestLooksLikeRecorderArtifact_RejectsMalformed proves the shape checker
// rejects common off-shape JSON a hand-crafted file might carry (F6 forge
// vector: a test process os.WriteFile-ing arbitrary JSON into the record dir).
func TestLooksLikeRecorderArtifact_RejectsMalformed(t *testing.T) {
	t.Parallel()
	cases := map[string][]byte{
		"empty req_id":        []byte(`{"req_id":"","test":"T","title":"t","steps":[],"verdict":"pass"}` + "\n"),
		"bad verdict":         []byte(`{"req_id":"R","test":"T","title":"t","steps":[],"verdict":"maybe"}` + "\n"),
		"missing steps field": []byte(`{"req_id":"R","test":"T","title":"t","verdict":"pass"}` + "\n"),
		"not even json":       []byte(`hello world`),
		"wrong structure":     []byte(`{"foo": "bar"}`),
		"empty json object":   []byte(`{}`),
	}
	for name, data := range cases {
		if looksLikeRecorderArtifact(data) {
			t.Errorf("case %q: expected looksLikeRecorderArtifact to REJECT this shape", name)
		}
	}
}

// TestReadArtifacts_F6_RejectsOffShapeFiles proves readArtifacts skips files
// that do not match the recorder's canonical shape, rather than ingesting them
// as genuine artifacts. Plants a mix of genuine + malformed files in a temp
// record dir and checks only the genuine one survives.
func TestReadArtifacts_F6_RejectsOffShapeFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	genuine := []byte(`{"req_id":"R-real","test":"TestReal","title":"real","steps":[],"verdict":"pass"}` + "\n")
	forged := []byte(`{"whatever": "this is not a recorder artifact"}`)

	if err := os.WriteFile(filepath.Join(dir, "R-real__TestReal.json"), genuine, 0o644); err != nil {
		t.Fatalf("WriteFile genuine: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "forged.json"), forged, 0o644); err != nil {
		t.Fatalf("WriteFile forged: %v", err)
	}

	arts, err := readArtifacts(dir)
	if err != nil {
		t.Fatalf("readArtifacts: %v", err)
	}
	if len(arts) != 1 {
		t.Fatalf("F6: expected readArtifacts to return exactly 1 artifact (the genuine one), got %d: %+v", len(arts), arts)
	}
	if arts[0].FileName != "R-real__TestReal.json" {
		t.Errorf("expected the genuine artifact, got %q", arts[0].FileName)
	}
}

// scenarioTestWrongReqIDSrc is a fixture test whose NewScenario call names a
// DIFFERENT requirement (R-different-req) than the requirement it will be
// cited from (R-citing-req). F6's req_id cross-check in recordVerifiedByEntry
// must filter this artifact out of R-citing-req's SPEC.md section.
const scenarioTestWrongReqIDSrc = `package model

import (
	"testing"

	"__MODULEPATH__/hotamspec"
)

func TestRequireComplete_ScenarioRecorded(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-different-req", "narrates a DIFFERENT requirement")
	s.Given("a fields count of zero", "fields", 0)
	err := RequireComplete(0)
	s.When("RequireComplete is called")
	s.Then("an error is returned", err != nil)
	s.Value("error_text", err)
}
`

// TestRecordVerifiedByEntry_F6_FiltersMismatchedReqID proves the F6 req_id
// cross-check: a verified_by test whose recorded artifact names a DIFFERENT
// requirement than the one being rendered is filtered out, not silently
// rendered into the wrong requirement's SPEC.md section.
func TestRecordVerifiedByEntry_F6_FiltersMismatchedReqID(t *testing.T) {
	const modulePath = "example.com/f6reqidcheck"
	// Use the same impl source as the recording fixture, but a test that
	// records under "R-different-req" instead of "R-citing-req".
	testSrc := strings.ReplaceAll(scenarioTestWrongReqIDSrc, "__MODULEPATH__", modulePath)
	root := writeRecordingFixture(t, modulePath, "model", scenarioImplSrc, "model", testSrc)

	// Render for R-citing-req -- the test's artifact says R-different-req.
	out := recordVerifiedByEntry(root, "R-citing-req", "model/impl_test.go:TestRequireComplete_ScenarioRecorded", "model/impl.go")
	if !out.passed {
		t.Fatalf("expected the test to pass (out.passed=true), got problem: %s", out.problem)
	}
	if len(out.artifacts) != 0 {
		t.Fatalf("F6: expected recordVerifiedByEntry to filter out artifacts whose req_id (R-different-req) does not "+
			"match the requirement being rendered (R-citing-req), got %d artifacts: %+v", len(out.artifacts), out.artifacts)
	}
}

// TestRecordVerifiedByEntry_F6_KeepsMatchingReqID proves the F6 req_id
// cross-check does NOT break the legitimate case: a verified_by test whose
// recorded artifact names the SAME requirement as the one being rendered
// passes through normally.
func TestRecordVerifiedByEntry_F6_KeepsMatchingReqID(t *testing.T) {
	const modulePath = "example.com/f6reqmatch"
	testSrc := strings.ReplaceAll(scenarioTestSrc(modulePath), "R-example-recording", "R-citing-req")
	root := writeRecordingFixture(t, modulePath, "model", scenarioImplSrc, "model", testSrc)

	out := recordVerifiedByEntry(root, "R-citing-req", "model/impl_test.go:TestRequireComplete_ScenarioRecorded", "model/impl.go")
	if !out.passed {
		t.Fatalf("expected the test to pass, got problem: %s", out.problem)
	}
	if len(out.artifacts) != 1 {
		t.Fatalf("F6: expected recordVerifiedByEntry to keep the artifact whose req_id matches the rendered requirement, "+
			"got %d artifacts", len(out.artifacts))
	}
}

// test_exec_recording.go holds the record-mode path: artifact capture and verdict parsing for scenario recording runs.
package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type RecordedArtifact struct {
	// FileName is the artifact's file name as the recorder wrote it
	// (<reqID>__<TestName>.json), useful for diagnostics and for a caller
	// that wants to correlate an artifact back to which requirement/test
	// produced it without re-parsing RawJSON.
	FileName string
	// RawJSON is the exact bytes read back from disk -- the canonical
	// encoding hotamspec.Scenario.writeArtifact produced, byte-identical
	// across repeated runs of the same scenario (see
	// PLAN-scenario-generated-spec.md §2 D1's determinism requirement).
	RawJSON []byte
}

// TestVerdict is one individual Go test or subtest's terminal result from
// `go test -json`. Package failure does not erase passing siblings.
type TestVerdict struct {
	Test    string
	Verdict string
}

// RecordingResult is the outcome of RunVerifiedByTestRecording: everything
// TestRunResult already reports (the test's own pass/fail verdict), plus the
// canonical scenario artifact(s) the test wrote in record-mode and the raw
// coverage profile bytes proving which lines of the implemented_by symbol's
// package this SAME test run actually executed (PLAN-scenario-generated-
// spec.md §2 D3 -- consumed by W2.2's coverage-proof gate, but collected
// here, in this one `go test` invocation, per the task's "one run gives
// asserts + artifact + coverprofile" contract).
type RecordingResult struct {
	TestRunResult
	// TestVerdicts preserves each terminal test/subtest action independently
	// of the package exit status.
	TestVerdicts []TestVerdict
	// Artifacts holds every canonical JSON scenario artifact found in the
	// record dir after the run -- ordinarily exactly one (a verified_by test
	// that constructs a single hotamspec.Scenario), but a test that
	// constructs more than one Scenario (e.g. several sub-cases, each with
	// its own NewScenario call) writes one artifact per Scenario, and all of
	// them are returned here. Empty (never nil vs non-nil distinguished
	// beyond len==0) when the test does not use hotamspec at all, or was not
	// reached (Skipped/Err).
	Artifacts []RecordedArtifact
	// CoverProfile is the raw bytes of the `go test -coverprofile` output
	// file for this run (Go's own text coverage-profile format: a "mode:"
	// header line followed by one "file:startLine.startCol,endLine.endCol
	// numStmt count" line per counted statement block) -- nil when coverage
	// collection was not requested, the run never reached execution
	// (Skipped/Err/CompileFailed), or go test produced no profile (can
	// happen for a package with zero coverable statements).
	CoverProfile []byte
}

// RunVerifiedByTestRecording is the RECORD-MODE sibling of RunVerifiedByTest
// (PLAN-scenario-generated-spec.md §2 D1/D3, task W1.2): ONE `go test`
// invocation that simultaneously (a) asserts normally (Then still calls
// t.Errorf exactly as plain mode), (b) writes a canonical hotamspec.Scenario
// JSON artifact per PLAN §2 D1 (via internal/recorder/canon's env-gated
// t.Cleanup -- see that package's RecordDirEnv), and (c) collects a Go
// coverage profile over coverPkgFile's OWN package (the implemented_by
// symbol's package -- PLAN §2 D3's coverage-proof input, consumed by a later
// gate, W2.2, but the profile is captured here because it can only ever be
// produced by actually re-running the test, and re-running it a SECOND time
// just to add -coverprofile would defeat the whole "one run proves
// everything" point of this task).
//
// specRoot/file/testName identify the verified_by test exactly as
// RunVerifiedByTest's own parameters do. coverPkgFile is a file living in the
// PACKAGE coverage should be measured over -- ordinarily the implemented_by
// entry's own file (e.g. "model/brd_package.go") -- so -coverpkg targets
// exactly that package's import path; pass "" to skip coverage collection
// entirely (a plain record-mode run with no coverprofile).
//
// Deliberately DOES NOT read or write the runCache/singleflightRun in-memory
// verdict cache RunVerifiedByTest maintains: a recording run's whole point is
// to produce FRESH artifacts and a fresh coverage profile for THIS call, so
// memoizing it under the same cache used by the boolean-verdict fast path
// would either (a) let a plain RunVerifiedByTest call silently reuse a
// recording run's cached TestRunResult without ever having asked for
// artifacts (harmless but confusing), or (b) let a recording call reuse a
// PLAIN cached result and return stale/absent artifacts for a test that
// really did just run with recording requested -- neither is worth the
// complexity of teaching one cache to carry two different result shapes.
// Every call to this function spawns its OWN real `go test` subprocess --
// unlike RunVerifiedByTest there is no in-memory memoization or singleflight
// collapsing here at all (a caller invoking this twice for the same test
// gets two real subprocess runs), which is deliberately simple rather than
// teaching one cache/singleflight pair to carry two different result shapes;
// callers that need to avoid redundant recording runs are expected to call
// this at most once per (file, test) per invocation, the same way a
// generator (W1.3) would only ever record a given verified_by entry once per
// `hotam gen-spec` run. There is no PERSISTENT disk cache of any kind here
// either way (holding b014a63's fix): the tmp directory this function
// creates is deleted before it returns, on every return path, success or
// failure.
//
// Although this function has no verdict cache, it DOES hash the module
// (hashPackageInputs) and sync the shared compile cache against that hash
// via syncCompileCacheToHash -- so a source change between two recording
// calls in one process invalidates the module's stale compiled binaries
// instead of serving a pre-mutation binary. See compile_cache.go.
func RunVerifiedByTestRecording(specRoot, file, testName, coverPkgFile string) RecordingResult {
	if inRecursionGuard() {
		return RecordingResult{TestRunResult: TestRunResult{
			Skipped: true,
			InfraWarning: fmt.Sprintf(
				"%s recursion guard honored -- this process did not execute %s itself (record-mode); it is skipped at this nesting level and must be proven PASSING by the outer, non-nested process that set this guard",
				recursionGuardEnv, testName),
		}}
	}

	path := filepath.Join(specRoot, filepath.FromSlash(file))
	pkgDir := filepath.Dir(path)
	absPkgDir, err := filepath.Abs(pkgDir)
	if err != nil {
		return RecordingResult{TestRunResult: TestRunResult{Err: fmt.Errorf("could not resolve package directory for %s: %w", path, err)}}
	}

	moduleRoot, ok := ModuleRoot(absPkgDir)
	if !ok {
		return RecordingResult{TestRunResult: TestRunResult{Err: fmt.Errorf("no go.mod found walking up from %s -- cannot determine which Go module owns %s", absPkgDir, path)}}
	}

	// The recording path has no verdict cache, so without this sync the
	// shared compile cache would never invalidate here; hash errors are
	// infrastructure failures, same classification as RunVerifiedByTest's
	// identical step. See syncCompileCacheToHash.
	hash, err := hashPackageInputs(moduleRoot, absPkgDir)
	if err != nil {
		return RecordingResult{TestRunResult: TestRunResult{Err: fmt.Errorf("could not hash package inputs for %s: %w", absPkgDir, err)}}
	}
	syncCompileCacheToHash(moduleRoot, hash)

	pattern, err := relativePackagePattern(moduleRoot, path)
	if err != nil {
		return RecordingResult{TestRunResult: TestRunResult{Err: err}}
	}

	var coverPkgPattern string
	if coverPkgFile != "" {
		coverAbs, err := filepath.Abs(filepath.Join(specRoot, filepath.FromSlash(coverPkgFile)))
		if err != nil {
			return RecordingResult{TestRunResult: TestRunResult{Err: fmt.Errorf("could not resolve coverage package file %s: %w", coverPkgFile, err)}}
		}
		coverPkgPattern, err = relativePackagePattern(moduleRoot, coverAbs)
		if err != nil {
			return RecordingResult{TestRunResult: TestRunResult{Err: fmt.Errorf("could not compute -coverpkg pattern for %s: %w", coverPkgFile, err)}}
		}
	}

	recordDir, err := os.MkdirTemp("", "hotam-record-")
	if err != nil {
		return RecordingResult{TestRunResult: TestRunResult{Err: fmt.Errorf("could not create per-run record tmp dir: %w", err)}}
	}
	defer os.RemoveAll(recordDir)

	// This ctx bounds only the PRE-execution waits (compile singleflight +
	// globalExecSlots slot-wait); the record-mode cmd.Run execution gets its
	// OWN execCtx inside runGoTestRecording (decoupled, see runGoTestRecording's
	// execCtx comment), independently sized by testExecTimeout.
	ctx, cancel := context.WithTimeout(context.Background(), testExecTimeout())
	defer cancel()
	runResult, coverProfile := runGoTestRecording(ctx, moduleRoot, pattern, testName, recordDir, coverPkgPattern)

	artifacts, artErr := readArtifacts(recordDir)
	if artErr != nil && runResult.Err == nil {
		runResult.Err = fmt.Errorf("record-mode run completed but artifacts could not be read back from %s: %w", recordDir, artErr)
	}

	return RecordingResult{
		TestRunResult: runResult,
		Artifacts:     artifacts,
		CoverProfile:  coverProfile,
	}
}

// RunAtomPackageRecording records all tests in one package. Artifacts travel
// through cached go-test stdout, not filesystem side effects: a native cache
// hit therefore reproduces exactly the evidence of the original passing run.
func RunAtomPackageRecording(specRoot, file string) RecordingResult {
	return runAtomRecording(specRoot, file, "", "")
}

// RunAtomTestRecording retains test-specific coverage attribution while using
// Go's native result/coverage cache. The Go command, not the test binary,
// restores -coverprofile output on cache hits.
func RunAtomTestRecording(specRoot, file, testName, coverPkgFile string) RecordingResult {
	return runAtomRecording(specRoot, file, testName, coverPkgFile)
}

func runAtomRecording(specRoot, file, testName, coverPkgFile string) RecordingResult {
	if inRecursionGuard() {
		return RecordingResult{TestRunResult: TestRunResult{Skipped: true, InfraWarning: "recursion guard honored"}}
	}
	path, err := filepath.Abs(filepath.Join(specRoot, filepath.FromSlash(file)))
	if err != nil {
		return RecordingResult{TestRunResult: TestRunResult{Err: err}}
	}
	root, ok := ModuleRoot(filepath.Dir(path))
	if !ok {
		return RecordingResult{TestRunResult: TestRunResult{Err: fmt.Errorf("no go.mod found for %s", path)}}
	}
	pattern, err := relativePackagePattern(root, path)
	if err != nil {
		return RecordingResult{TestRunResult: TestRunResult{Err: err}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), testExecTimeout())
	defer cancel()
	select {
	case globalExecSlots <- struct{}{}:
	case <-ctx.Done():
		return RecordingResult{TestRunResult: TestRunResult{Err: ctx.Err()}}
	}
	defer func() { <-globalExecSlots }()
	args := []string{"test", "-json", "-v"}
	if testName != "" {
		args = append(args, "-run", "^"+regexp.QuoteMeta(testName)+"$")
	}
	var coverProfile string
	if coverPkgFile != "" {
		coverPath, err := filepath.Abs(filepath.Join(specRoot, filepath.FromSlash(coverPkgFile)))
		if err != nil {
			return RecordingResult{TestRunResult: TestRunResult{Err: err}}
		}
		coverPattern, err := relativePackagePattern(root, coverPath)
		if err != nil {
			return RecordingResult{TestRunResult: TestRunResult{Err: err}}
		}
		dir, err := os.MkdirTemp("", "hotam-atom-cover-")
		if err != nil {
			return RecordingResult{TestRunResult: TestRunResult{Err: err}}
		}
		defer os.RemoveAll(dir)
		coverProfile = filepath.Join(dir, "cover.out")
		args = append(args, "-coverpkg", coverPattern, "-coverprofile", coverProfile)
	}
	args = append(args, pattern)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	// Stable values are intentional: Go records getenv dependencies in its
	// test cache. A fresh nonce or temporary recording path defeats that cache.
	cmd.Env = append(os.Environ(), recursionGuardEnv+"=atom-package", "HOTAM_RECORD_STDOUT=1", recordDirEnvName+"=")
	data, runErr := cmd.CombinedOutput()
	result := RecordingResult{TestRunResult: TestRunResult{Output: boundOutput(string(data))}}
	if coverProfile != "" {
		profile, err := os.ReadFile(coverProfile)
		if err == nil {
			result.CoverProfile = profile
		} else if runErr == nil {
			result.Err = fmt.Errorf("reading native atom coverage profile: %w", err)
		}
	}
	stdoutText, testVerdicts, decodeErr := parseAtomEvents(data)
	result.TestVerdicts = testVerdicts
	if decodeErr != nil {
		result.Err = fmt.Errorf("decoding go test JSON output: %w", decodeErr)
	}
	for _, line := range strings.Split(stdoutText, "\n") {
		const marker = "HOTAMSPEC_ARTIFACT:"
		if !strings.HasPrefix(line, marker) {
			continue
		}
		raw := []byte(strings.TrimSpace(strings.TrimPrefix(line, marker)))
		if !looksLikeRecorderArtifact(raw) {
			result.Err = fmt.Errorf("invalid atom recording artifact")
			continue
		}
		result.Artifacts = append(result.Artifacts, RecordedArtifact{RawJSON: raw})
	}
	if ctx.Err() != nil {
		result.Err = ctx.Err()
	} else if runErr != nil {
		var exit *exec.ExitError
		if !isExitError(runErr, &exit) {
			result.Err = runErr
		}
		result.CompileFailed = looksLikeCompileFailure(string(data))
	} else {
		result.Passed = true
	}
	return result
}

func parseAtomEvents(data []byte) (string, []TestVerdict, error) {
	var stdout strings.Builder
	var verdicts []TestVerdict
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var event struct {
			Action string
			Test   string
			Output string
		}
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			sortTestVerdicts(verdicts)
			return stdout.String(), verdicts, err
		}
		stdout.WriteString(event.Output)
		switch event.Action {
		case "pass", "fail", "skip":
			if event.Test != "" {
				verdicts = append(verdicts, TestVerdict{Test: event.Test, Verdict: event.Action})
			}
		}
	}
	sortTestVerdicts(verdicts)
	return stdout.String(), verdicts, nil
}

func sortTestVerdicts(verdicts []TestVerdict) {
	sort.Slice(verdicts, func(i, j int) bool {
		if verdicts[i].Test == verdicts[j].Test {
			return verdicts[i].Verdict < verdicts[j].Verdict
		}
		return verdicts[i].Test < verdicts[j].Test
	})
}

// runGoTestRecording is runGoTest's record-mode sibling: same recursion-guard
// nonce, same globalExecSlots bound, same PASS/FAIL/CompileFailed
// classification -- but additionally sets hotamspec.RecordDirEnv
// ("HOTAM_RECORD_DIR") to recordDir on the child process's environment, and,
// when coverPkgPattern is non-empty, the compiled binary was built with
// -coverpkg so a -test.coverprofile (written inside recordDir, never a
// shared/predictable path) at RUN time instruments the implemented_by
// symbol's package. Returns the classified TestRunResult plus the raw
// coverprofile bytes (nil if coverage was not requested or the file was
// never produced).
//
// The -coverpkg flag is a COMPILE-time input (Go bakes the coverage
// instrumentation into the binary when `go test -c` runs), so the compiled
// binary is cached keyed by coverPkgPattern -- a record-mode call for
// (pkg P, coverpkg A) and a second call for (pkg P, coverpkg B) get
// DIFFERENT compiled binaries and do NOT share, even though they share the
// same pkgPattern. The -test.coverprofile flag, by contrast, is a RUNTIME
// input (the binary writes the profile to whatever path the caller names),
// so it stays per-invocation and points into this call's own recordDir.
func runGoTestRecording(ctx context.Context, moduleRoot, pkgPattern, testName, recordDir, coverPkgPattern string) (TestRunResult, []byte) {
	// Step 1: get-or-compile the binary for this package AND coverpkg
	// pattern. The coverpkg is baked in at compile time, so it is part of
	// the cache KEY -- two record-mode calls for the same package but
	// different coverpkg patterns get different binaries (correctly).
	bin := compileTestBinary(ctx, moduleRoot, pkgPattern, coverPkgPattern)
	if bin.err != nil {
		return TestRunResult{Output: bin.output, Err: bin.err}, nil
	}
	if bin.compileFailed {
		return TestRunResult{Passed: false, CompileFailed: true, Output: bin.output}, nil
	}

	select {
	case globalExecSlots <- struct{}{}:
	case <-ctx.Done():
		return TestRunResult{Err: fmt.Errorf("go test (record-mode) for %s in %s: %w (timed out waiting for an execution slot)", testName, pkgPattern, ctx.Err())}, nil
	}
	defer func() { <-globalExecSlots }()

	runPattern := "^" + testName + "$"
	args := []string{"-test.run", runPattern, "-test.count", "1"}
	var coverProfilePath string
	if coverPkgPattern != "" {
		coverProfilePath = filepath.Join(recordDir, "cover.out")
		args = append(args, "-test.coverprofile="+coverProfilePath)
	}

	// execCtx decouples EXECUTION from the caller's slot-wait ctx, for the
	// same load-contention reason as runGoTest (see its Step 2 comment);
	// testExecTimeout makes it operator-tunable.
	execCtx, execCancel := context.WithTimeout(context.Background(), testExecTimeout())
	defer execCancel()
	cmd := exec.CommandContext(execCtx, bin.path, args...)
	// See runGoTest: a directly-invoked .test binary does not chdir into
	// the package directory on its own the way `go test` does, so set
	// cmd.Dir explicitly to preserve testdata/ + cwd-relative behavior.
	cmd.Dir = packageDirFromPattern(moduleRoot, pkgPattern)
	nonce := guardNonce()
	cmd.Env = append(os.Environ(),
		recursionGuardEnv+"="+nonce,
		recordDirEnvName+"="+recordDir,
	)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	output := boundOutput(buf.String())

	var coverProfile []byte
	if coverProfilePath != "" {
		if data, readErr := os.ReadFile(coverProfilePath); readErr == nil {
			coverProfile = data
		}
		// A missing coverprofile (e.g. the run failed before any test ran at
		// all, or a package with zero coverable statements) is not itself an
		// error worth surfacing here -- TestRunResult's own Passed/
		// CompileFailed/Err already carries whatever went wrong with the run
		// itself; CoverProfile simply stays nil.
	}

	if execCtx.Err() == context.DeadlineExceeded {
		return TestRunResult{
			Output: output,
			Err:    fmt.Errorf("go test (record-mode) timed out running %s in %s: %w", runPattern, pkgPattern, execCtx.Err()),
		}, coverProfile
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !isExitError(err, &exitErr) {
			return TestRunResult{Output: output, Err: fmt.Errorf("could not run go test (record-mode): %w", err)}, coverProfile
		}
		// A directly-invoked .test binary has already been compiled, so
		// looksLikeCompileFailure(output) should not fire in practice;
		// kept defensive for the same reason runGoTest keeps it.
		compileFailed := looksLikeCompileFailure(output)
		return TestRunResult{Passed: false, CompileFailed: compileFailed, Output: output}, coverProfile
	}
	if strings.Contains(output, "\nFAIL") || strings.HasPrefix(output, "FAIL") {
		return TestRunResult{Passed: false, Output: output}, coverProfile
	}
	return TestRunResult{Passed: true, Output: output}, coverProfile
}

// recordDirEnvName is HotamSpec's own copy of internal/recorder/canon's
// RecordDirEnv literal ("HOTAM_RECORD_DIR"). Declared as a separate literal
// here, rather than importing internal/recorder/canon directly, so this
// package (internal/gate, engine-internal) never depends on the recorder
// canon package that gets VENDORED (copied) into consumer domains -- the two
// sides of this contract (the engine setting the env var, the vendored
// recorder reading it) only need to agree on the STRING, not share a Go
// import; a mismatch between the two literals is caught mechanically by
// TestRecordDirEnvName_MatchesCanonLiteral (test_exec_test.go), which reads
// both packages' source at test time to compare the two constants without
// creating a compile-time dependency.
const recordDirEnvName = "HOTAM_RECORD_DIR"

// readArtifacts reads every *.json file directly inside dir (non-recursive --
// hotamspec.Scenario's writer never creates subdirectories) into memory as
// RecordedArtifact values, sorted by file name for a deterministic return
// order (os.ReadDir's own result is already name-sorted, but sorting again
// explicitly here documents the guarantee rather than relying on an incidental
// stdlib behavior this function does not itself control).
//
// F6 ARTIFACT SHAPE VALIDATION (task W7.2, @fx finding F6): before trusting a
// JSON file as a genuine recorder artifact, this function verifies it matches
// the EXACT shape internal/recorder/canon's writeArtifact produces
// (req_id/test/title/steps/verdict fields, verdict is "pass" or "fail",
// req_id non-empty). A file that does not match is SILENTLY SKIPPED (not
// included in the returned slice) -- the record dir is a per-run tmp
// directory the engine itself creates and sets via HOTAM_RECORD_DIR, so a
// file that does not match the recorder's shape was not written by the
// recorder. This closes the forge vector where a test process could
// os.WriteFile a hand-crafted JSON mimicking the artifact shape into the
// record dir, bypassing the real recorder API entirely.
func readArtifacts(dir string) ([]RecordedArtifact, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	artifacts := make([]RecordedArtifact, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return artifacts, fmt.Errorf("could not read artifact %s: %w", name, err)
		}
		if !looksLikeRecorderArtifact(data) {
			// F6: this file does not match the recorder's canonical artifact
			// shape -- skip it rather than trusting arbitrary JSON as a
			// genuine recorder-produced artifact.
			continue
		}
		artifacts = append(artifacts, RecordedArtifact{FileName: name, RawJSON: data})
	}
	return artifacts, nil
}

// looksLikeRecorderArtifact verifies that data decodes into the EXACT shape
// internal/recorder/canon's writeArtifact produces (hotamspec.go's Artifact
// struct): top-level fields req_id (non-empty string), test (string), title
// (string), steps (array), and verdict ("pass" or "fail"). The recorder
// ALWAYS writes all five fields in this shape, so a file missing any of them
// or carrying unexpected types was not produced by the recorder. This is a
// STRUCTURAL shape check only -- it does not validate the steps' internal
// structure (that is specArtifact's own json.Unmarshal's job in
// recordVerifiedByEntry) or the req_id's correctness for the requirement
// being rendered (that is recordVerifiedByEntry's F6 req_id cross-check).
func looksLikeRecorderArtifact(data []byte) bool {
	var shape struct {
		ReqID   string `json:"req_id"`
		Test    string `json:"test"`
		Title   string `json:"title"`
		Steps   []any  `json:"steps"`
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal(data, &shape); err != nil {
		return false
	}
	// The recorder always writes a non-empty reqID (NewScenario's first arg).
	if shape.ReqID == "" {
		return false
	}
	// The recorder only writes "pass" or "fail" (derived from t.Failed()).
	if shape.Verdict != "pass" && shape.Verdict != "fail" {
		return false
	}
	// Steps must be present (even if empty -- a scenario with zero steps is
	// structurally possible, and the recorder writes `"steps": []`).
	if shape.Steps == nil {
		return false
	}
	return true
}

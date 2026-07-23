package selfcheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// ciWorkflowPath is the CI workflow file whose test-race job package list is
// this repo's single source of truth for "what CI actually race-tests".
const ciWorkflowPath = ".github/workflows/ci.yml"

// TestRaceRatchet_GoroutinePackagesCoveredByCI enforces R-race-ratchet
// (task #336, R4F-race-ratchet — fourth external review's final synthesis
// §4.5): every framework package with a real goroutine spawn in non-test
// code is covered by CI's `-race` job.
//
// BACKGROUND: task #327 scoped CI's test-race job to a manually-chosen
// package list, justified by a CHANGELOG/comment claim that these are "the
// only packages with real goroutine/sync usage in non-test code". The
// fourth external review found that claim was already false the day it was
// written: cmd/hotam/common.go's writeFilesParallel used real goroutines
// (go func + sync.WaitGroup) and was NOT covered, because cmd/hotam is
// excluded from -race wholesale for an unrelated reason (its e2e tests
// spawn a compiled subprocess -race on the parent can't instrument). That
// gap was safe in that one instance (index-based writes, no shared mutable
// state — since fixed by extracting it into internal/fsio, which IS in the
// -race scope) but nothing MECHANICALLY connected "what has goroutines" to
// "what CI race-tests" -- so the gap could silently reappear and grow as
// the codebase does. This test is that mechanical connection.
//
// EXACT RULE (mechanically checked): for every NON-TEST .go file under
// internal/ and cmd/hotam/ (frameworkScanRoots), AST-detect any `go`
// statement (go/ast.GoStmt — a real goroutine spawn, not a comment or
// string mentioning "goroutine"). Collect the set of packages (import-path
// style, e.g. "internal/gate", "cmd/hotam") containing at least one such
// file. Cross-reference that set against the package list CI's test-race
// job actually race-tests, parsed directly out of .github/workflows/ci.yml
// (the `go test -race ... <pkg>/... <pkg>/...` line) -- ONE source of
// truth (the YAML), not a second hand-maintained list duplicated in Go.
// Every AST-detected package must be a member of the parsed CI set, or a
// literal prefix of the module (an explicit `./...`).
//
// Discrimination: see TestRaceRatchet_DetectsUncoveredPackage and
// TestRaceRatchet_CIParserNonVacuous.
func TestRaceRatchet_GoroutinePackagesCoveredByCI(t *testing.T) {
	t.Parallel()
	goroutinePkgs := goroutinePackages(t, frameworkScanRoots)
	if len(goroutinePkgs) == 0 {
		t.Fatal("goroutinePackages found zero packages with `go` statements — detector is broken (internal/gate and internal/invariants are known to have real goroutines)")
	}

	racePkgs := ciRacePackages(t)
	if len(racePkgs) == 0 {
		t.Fatal("ciRacePackages parsed zero packages from the test-race job — CI YAML parser is broken or the job shape changed")
	}

	for pkg := range goroutinePkgs {
		if !racePackageCovers(racePkgs, pkg) {
			t.Errorf("R-race-ratchet: package %q spawns a real goroutine (go statement) in non-test code but is NOT covered by CI's test-race job (%s) — add it to the -race package list in %s, move the goroutine into a package that IS covered, or (if genuinely -race-irrelevant, e.g. e2e subprocess spawning) document an explicit, auditable exemption here",
				pkg, ciWorkflowPath, ciWorkflowPath)
		}
	}
}

// TestRaceRatchet_DetectsUncoveredPackage is the non-vacuity control: the
// coverage predicate must flag a synthetic package that has a goroutine but
// is absent from the CI set, and must NOT flag a package present in the CI
// set (mirroring TestCorePeriphery_RatchetDetectsViolation's shape).
func TestRaceRatchet_DetectsUncoveredPackage(t *testing.T) {
	t.Parallel()
	racePkgs := map[string]bool{
		"internal/gate":       true,
		"internal/generator":  true,
		"internal/invariants": true,
		"internal/fsio":       true,
	}

	if racePackageCovers(racePkgs, "internal/nonexistent-uncovered-package") {
		t.Error("detector failed to flag an uncovered synthetic package — the main ratchet test would be vacuous")
	}
	if !racePackageCovers(racePkgs, "internal/gate") {
		t.Error("detector incorrectly flagged a package that IS in the CI race set")
	}
	if !racePackageCovers(racePkgs, "internal/fsio") {
		t.Error("detector incorrectly flagged internal/fsio, which IS in the CI race set")
	}
}

// TestRaceRatchet_ASTDetectorNonVacuous proves goStmtInFile genuinely
// distinguishes a real `go` statement from source that merely mentions
// goroutines in a comment or string literal, by parsing small synthetic
// source snippets rather than relying only on this repo's current shape.
func TestRaceRatchet_ASTDetectorNonVacuous(t *testing.T) {
	t.Parallel()

	const hasGoStmt = `package p

import "sync"

func f() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
	}()
	wg.Wait()
}
`
	const noGoStmt = `package p

// This function spawns a goroutine to do work. (comment only — not code)
const note = "go func() launches a goroutine"

func f() {
	// nothing concurrent here
}
`

	if !goStmtInSource(t, hasGoStmt) {
		t.Error("detector failed to find a real go statement in source that has one")
	}
	if goStmtInSource(t, noGoStmt) {
		t.Error("detector false-positived on a comment/string merely mentioning goroutines, with no real go statement")
	}
}

// TestRaceRatchet_CIParserNonVacuous proves ciRacePackages's regex-based
// extraction genuinely parses the package list out of a YAML `go test -race`
// line, rather than trivially returning something.
func TestRaceRatchet_CIParserNonVacuous(t *testing.T) {
	t.Parallel()
	const fixture = `      - name: go test -race (concurrency-bearing packages)
        run: go test -race -timeout 30m ./internal/gate/... ./internal/generator/... ./internal/invariants/...
`
	got := parseRacePackagesFromYAML(fixture)
	want := map[string]bool{
		"internal/gate":       true,
		"internal/generator":  true,
		"internal/invariants": true,
	}
	if len(got) != len(want) {
		t.Fatalf("parsed %v packages, want %v", got, want)
	}
	for pkg := range want {
		if !got[pkg] {
			t.Errorf("parser missed expected package %q, got %v", pkg, got)
		}
	}
}

// goroutinePackages returns the set of packages (repo-relative dir path,
// e.g. "internal/gate", "cmd/hotam") containing at least one NON-TEST .go
// file with a real `go` statement (goroutine spawn) under roots.
func goroutinePackages(t *testing.T, roots []string) map[string]bool {
	t.Helper()
	root := repoRoot(t)
	files := collectGoFiles(t, roots, false /* non-test */, false /* no testdata */)
	pkgs := map[string]bool{}
	for _, f := range files {
		if !goStmtInFile(f.ast) {
			continue
		}
		dir := filepath.Dir(f.path)
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			t.Fatalf("rel(%s, %s): %v", root, dir, err)
		}
		pkgs[filepath.ToSlash(rel)] = true
	}
	return pkgs
}

// goStmtInFile reports whether file contains at least one real `go`
// statement (go/ast.GoStmt) anywhere in its declarations — a goroutine
// spawn, as opposed to a comment or string literal merely mentioning one.
func goStmtInFile(file *ast.File) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if found {
			return false
		}
		if _, ok := n.(*ast.GoStmt); ok {
			found = true
			return false
		}
		return true
	})
	return found
}

// goStmtInSource parses src as a standalone Go file and reports whether it
// contains a real `go` statement. Used only by the AST detector's own
// non-vacuity control (small synthetic snippets, not full repo files).
func goStmtInSource(t *testing.T, src string) bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "synth_race_ratchet.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	return goStmtInFile(file)
}

// racePackageCovers reports whether pkg (e.g. "internal/fsio") is covered by
// the CI race package set racePkgs (e.g. {"internal/gate": true, ...}),
// parsed from the `<pkg>/...` tokens on the workflow's `go test -race` line.
func racePackageCovers(racePkgs map[string]bool, pkg string) bool {
	return racePkgs[pkg]
}

// ciRacePackages reads .github/workflows/ci.yml from repo root and returns
// the set of packages covered by the test-race job's `go test -race` line.
func ciRacePackages(t *testing.T) map[string]bool {
	t.Helper()
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ciWorkflowPath))
	if err != nil {
		t.Fatalf("read %s: %v", ciWorkflowPath, err)
	}
	return parseRacePackagesFromYAML(string(data))
}

// raceLineRE matches the `run:` line inside the test-race job that invokes
// `go test -race ...`, capturing the full remainder of the line (the
// -timeout flag plus every ./pkg/... argument).
var raceLineRE = regexp.MustCompile(`(?m)^\s*run:\s*go test -race\s+(.*)$`)

// racePkgTokenRE matches a single `./some/pkg/...` package pattern token.
var racePkgTokenRE = regexp.MustCompile(`\./(\S+?)/\.\.\.`)

// parseRacePackagesFromYAML extracts the set of packages (e.g.
// "internal/gate") from a `go test -race ... ./internal/gate/... ...` line
// found anywhere in yaml text. It intentionally does NOT attempt full YAML
// parsing (job/step structure) — the workflow file's `run:` line is plain
// text and a targeted regex is far less fragile here than teaching this
// test a YAML schema it doesn't otherwise need, since the only thing this
// ratchet needs is "which package patterns follow `go test -race`".
func parseRacePackagesFromYAML(yaml string) map[string]bool {
	pkgs := map[string]bool{}
	m := raceLineRE.FindStringSubmatch(yaml)
	if m == nil {
		return pkgs
	}
	for _, tok := range racePkgTokenRE.FindAllStringSubmatch(m[1], -1) {
		pkgs[tok[1]] = true
	}
	return pkgs
}

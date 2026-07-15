package main

import (
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pratDomainDirForCmdTest locates the real PRAT-hotam "prat" domain, a
// sibling checkout to this repo (D:/ai_dev/prat/{HotamSpec,PRAT-hotam}).
// Mirrors internal/generator/gocode/gocode_test.go's pratDomainDir, but this
// package cannot import that unexported helper, so it is duplicated here at
// cmd/hotam's own relative depth (cmd/hotam -> ../.. -> D:/ai_dev/prat).
// This helper only READS the domain (never writes into it) — write-side
// tests in this file always copy the graph into a t.TempDir() first.
func pratDomainDirForCmdTest(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	candidate := filepath.Join(wd, "..", "..", "..", "PRAT-hotam", "domains", "prat")
	graphPath := filepath.Join(candidate, "graph.json")
	if _, err := os.Stat(graphPath); err != nil {
		t.Skipf("sibling PRAT-hotam checkout not found at %s (%v) — skipping real-domain test", graphPath, err)
	}
	return candidate
}

// copyPratDomainGraph copies the real prat domain's graph.json (and
// manifest.json, if present) into a fresh t.TempDir()-backed domain
// directory, so gen-code's write-side tests never touch the read-only
// sibling checkout (task instructions: PRAT-hotam is read-only for this
// task, gen-code's own landing there is task #181).
func copyPratDomainGraph(t *testing.T) string {
	t.Helper()
	src := pratDomainDirForCmdTest(t)
	domainDir := t.TempDir()
	copyFile(t, filepath.Join(src, "graph.json"), filepath.Join(domainDir, "graph.json"))
	if _, err := os.Stat(filepath.Join(src, "manifest.json")); err == nil {
		copyFile(t, filepath.Join(src, "manifest.json"), filepath.Join(domainDir, "manifest.json"))
	}
	return domainDir
}

// TestGenCode_WritesFilesToGenGoDir proves genCode actually writes the
// gen-code output set to <domainDir>/gen/go/ (GEN-CODE-CONTRACT.md §1) for a
// real domain graph with EntityTypes (the local hotam-spec-self fixture
// currently carries zero EntityTypes, so this exercises the real prat
// domain, mirroring internal/generator/gocode/gocode_test.go's own choice of
// fixture for the same reason).
func TestGenCode_WritesFilesToGenGoDir(t *testing.T) {
	t.Parallel()
	domainDir := copyPratDomainGraph(t)

	written, err := genCode(domainDir)
	if err != nil {
		t.Fatalf("genCode: %v", err)
	}
	if len(written) == 0 {
		t.Fatal("genCode wrote no files")
	}

	genGoDir := filepath.Join(domainDir, "gen", "go")
	// go.mod and entities.go are unconditionally produced by
	// GenerateModelsFromGraph for any non-empty EntityTypes set.
	for _, name := range []string{"go.mod", "entities.go", "lifecycle.go", "lifecycle_test.go", "requirements_test.go", "requirements_audit.md"} {
		p := filepath.Join(genGoDir, name)
		info, statErr := os.Stat(p)
		if statErr != nil {
			t.Fatalf("expected %s to be written, stat error: %v", name, statErr)
		}
		if info.Size() == 0 {
			t.Fatalf("expected %s to be non-empty", name)
		}
		found := false
		for _, w := range written {
			if w == p {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("genCode's written list did not include %s", p)
		}
	}
}

// TestGenCode_IdempotentAcrossReruns enforces GEN-CODE-CONTRACT.md §5's
// idempotency invariant at the CLI-command layer (not just inside the
// gocode package's own generators, which are already proven deterministic):
// two genCode runs against an unchanged graph must produce byte-identical
// files on disk, verified via sha256 (not git diff, per the task's explicit
// acceptance requirement).
func TestGenCode_IdempotentAcrossReruns(t *testing.T) {
	t.Parallel()
	domainDir := copyPratDomainGraph(t)

	written1, err := genCode(domainDir)
	if err != nil {
		t.Fatalf("genCode (first run): %v", err)
	}
	sums1 := sha256SumFiles(t, written1)

	written2, err := genCode(domainDir)
	if err != nil {
		t.Fatalf("genCode (second run): %v", err)
	}
	sums2 := sha256SumFiles(t, written2)

	if len(written1) != len(written2) {
		t.Fatalf("file count changed across reruns: %d vs %d", len(written1), len(written2))
	}
	for path, sum1 := range sums1 {
		sum2, ok := sums2[path]
		if !ok {
			t.Fatalf("file %s present in first run, missing in second", path)
		}
		if sum1 != sum2 {
			t.Fatalf("file %s changed across reruns (sha256 %s vs %s) — idempotency violated", path, sum1, sum2)
		}
	}
}

func sha256SumFiles(t *testing.T, paths []string) map[string]string {
	t.Helper()
	sums := make(map[string]string, len(paths))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		sum := sha256.Sum256(data)
		sums[p] = string(sum[:])
	}
	return sums
}

// captureStdout redirects os.Stdout to a pipe for the duration of fn,
// returning whatever fn wrote to stdout. Mirrors captureStderr
// (common_test.go) exactly, for os.Stdout instead of os.Stderr. Process-global
// (os.Stdout is a single *os.File), so tests using it MUST NOT call
// t.Parallel().
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	outC := make(chan string)
	go func() {
		var buf strings.Builder
		_, _ = io.Copy(&buf, r)
		outC <- buf.String()
	}()
	fn()
	w.Close()
	os.Stdout = orig
	return <-outC
}

// TestCmdGenCode_PrintsWrittenFiles proves the command-level entry point
// (cmdGenCode, the --domain flag plumbing) prints the relative path of every
// written file to stdout, mirroring gen-spec's own console-listing contract.
func TestCmdGenCode_PrintsWrittenFiles(t *testing.T) {
	domainDir := copyPratDomainGraph(t)

	stdout := captureStdout(t, func() {
		if err := cmdGenCode([]string{"--domain", domainDir}); err != nil {
			t.Fatalf("cmdGenCode: %v", err)
		}
	})

	for _, want := range []string{"go.mod", "entities.go", "lifecycle.go"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected cmdGenCode output to mention %q, got:\n%s", want, stdout)
		}
	}
}

// TestGenCode_EmptyGraphNoEntityTypes proves genCode does not fail on a
// domain with zero EntityTypes (e.g. the real hotam-spec-self domain today) —
// GenerateModelsFromGraph et al. still run over an empty EntityTypes slice
// and simply produce whatever real (possibly empty) file set they legitimately
// have; genCode must not synthesize placeholder files for content that was
// never produced.
func TestGenCode_EmptyGraphNoEntityTypes(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)

	written, err := genCode(domainDir)
	if err != nil {
		t.Fatalf("genCode on zero-EntityType domain must not error, got: %v", err)
	}
	// go.mod is unconditionally produced by GenerateModelsFromGraph
	// regardless of EntityTypes count.
	foundGoMod := false
	for _, w := range written {
		if filepath.Base(w) == "go.mod" {
			foundGoMod = true
		}
	}
	if !foundGoMod {
		t.Fatalf("expected go.mod to be written even for a zero-EntityType domain, written: %v", written)
	}
}

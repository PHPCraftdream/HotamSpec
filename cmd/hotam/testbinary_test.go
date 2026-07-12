package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	sharedBinaryOnce sync.Once
	sharedBinaryPath string
	sharedBinaryErr  error
)

// buildSharedHotamBinary go build's the default (no ldflags) hotam binary
// exactly once per `go test` process and returns its path to every caller.
// On Windows a single `go build` of this module is dominated by antivirus
// scanning of the freshly written exe, so tests that each ran their own
// `go build` for a plain default-version binary (TestVersion's default
// check, the external e2e test) were paying that cost redundantly. Tests
// that need a SPECIFIC ldflags-injected version string still build their
// own — that content can't be shared.
func buildSharedHotamBinary(t *testing.T) string {
	t.Helper()
	sharedBinaryOnce.Do(func() {
		repoRoot := repoRootForTest(t)
		binDir, err := os.MkdirTemp("", "hotam-shared-bin-")
		if err != nil {
			sharedBinaryErr = fmt.Errorf("MkdirTemp shared binDir: %w", err)
			return
		}
		binName := "hotam"
		if runtime.GOOS == "windows" {
			binName = "hotam.exe"
		}
		binPath := filepath.Join(binDir, binName)
		cmd := exec.Command("go", "build", "-o", binPath, "./cmd/hotam")
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			sharedBinaryErr = fmt.Errorf("go build shared hotam binary: %w\n%s", err, out)
			return
		}
		sharedBinaryPath = binPath
	})
	if sharedBinaryErr != nil {
		t.Fatalf("%v", sharedBinaryErr)
	}
	return sharedBinaryPath
}

// TestMain removes the shared binary's temp directory once every test in
// this package has finished with it (individual tests deliberately do not
// register a t.Cleanup for it, since more than one test shares the build).
func TestMain(m *testing.M) {
	code := m.Run()
	if sharedBinaryPath != "" {
		os.RemoveAll(filepath.Dir(sharedBinaryPath))
	}
	os.Exit(code)
}

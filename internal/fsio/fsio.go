// Package fsio holds this repository's real in-process concurrency for
// writing generated files to disk. It exists as its own package (task #336,
// R4F-race-ratchet — fourth external review's final synthesis §4.5) so this
// goroutine-bearing code can sit in CI's `-race` scope without pulling the
// whole of cmd/hotam (dominated by e2e tests that spawn a compiled
// subprocess `-race` on the parent process cannot instrument, task #327)
// into that job.
package fsio

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// WriteFileMkdir writes data to path, creating any missing parent
// directories first.
func WriteFileMkdir(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// WriteFilesParallel writes each (path, content) pair in paths[i]/contents[i]
// to disk concurrently — one goroutine per file, each touching only its own
// path, so there is no write collision to guard against. It mirrors
// invariants.AllViolations' indexed-slice-then-merge shape: results land in
// a pre-sized slot per index (never via append from a goroutine, which would
// race), so the caller can rebuild output in the original, deterministic
// order after wg.Wait() rather than in whatever order goroutines happen to
// finish.
//
// It returns the first error encountered, selected deterministically by
// index (lowest i wins) rather than by goroutine completion order, so a
// failure is reproducible across runs even though the writes themselves are
// concurrent.
func WriteFilesParallel(paths []string, contents [][]byte) error {
	errs := make([]error, len(paths))
	var wg sync.WaitGroup
	for i := range paths {
		wg.Add(1)
		go func(idx int, p string, data []byte) {
			defer wg.Done()
			errs[idx] = WriteFileMkdir(p, data)
		}(i, paths[i], contents[i])
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

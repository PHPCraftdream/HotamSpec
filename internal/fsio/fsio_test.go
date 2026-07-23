package fsio

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestWriteFileMkdir_CreatesParentDirs proves WriteFileMkdir creates any
// missing parent directories and writes the exact bytes given.
func TestWriteFileMkdir_CreatesParentDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "a", "b", "c", "file.txt")
	want := []byte("hello, fsio")

	if err := WriteFileMkdir(target, want); err != nil {
		t.Fatalf("WriteFileMkdir: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content = %q, want %q", got, want)
	}
}

// TestWriteFilesParallel_WritesEveryFileCorrectly is the primary reason this
// package exists in CI's -race scope: it drives WriteFilesParallel's real
// goroutine-per-file fan-out (task #336) across enough files that the race
// detector has a genuine chance to catch a regression (e.g. someone later
// "optimizing" errs[idx] into a shared append or a captured loop variable),
// and asserts every file landed with its own correct content — proving the
// indexed-slot design does not cross-write between goroutines.
func TestWriteFilesParallel_WritesEveryFileCorrectly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	const n = 64
	paths := make([]string, n)
	contents := make([][]byte, n)
	for i := 0; i < n; i++ {
		paths[i] = filepath.Join(dir, "sub", "nested", filepath.FromSlash("f"+strconv.Itoa(i)+".txt"))
		contents[i] = []byte("content-" + strconv.Itoa(i))
	}

	if err := WriteFilesParallel(paths, contents); err != nil {
		t.Fatalf("WriteFilesParallel: %v", err)
	}

	for i := 0; i < n; i++ {
		got, err := os.ReadFile(paths[i])
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", paths[i], err)
		}
		want := "content-" + strconv.Itoa(i)
		if string(got) != want {
			t.Errorf("file %d: content = %q, want %q (possible cross-goroutine write collision)", i, got, want)
		}
	}
}

// TestWriteFilesParallel_ReturnsDeterministicLowestIndexError proves the
// error-selection contract documented on WriteFilesParallel: when multiple
// writes fail, the returned error is chosen by lowest index, not by
// goroutine completion order, so a failure is reproducible across runs.
func TestWriteFilesParallel_ReturnsDeterministicLowestIndexError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Every path in this run is invalid: a required parent directory segment
	// is actually a FILE (not a dir), so MkdirAll fails for all of them.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocker file: %v", err)
	}

	const n = 8
	paths := make([]string, n)
	contents := make([][]byte, n)
	for i := 0; i < n; i++ {
		paths[i] = filepath.Join(blocker, "child"+strconv.Itoa(i)+".txt")
		contents[i] = []byte("x")
	}

	err := WriteFilesParallel(paths, contents)
	if err == nil {
		t.Fatal("WriteFilesParallel: want error, got nil")
	}
	// The error must reference index 0's path specifically (lowest-index
	// selection), not whichever goroutine happened to finish first.
	if got := err.Error(); !strings.Contains(got, paths[0]) {
		t.Errorf("error = %q, want it to reference the lowest-index path %q", got, paths[0])
	}
}

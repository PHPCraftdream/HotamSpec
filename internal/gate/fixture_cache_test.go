package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// fixture_cache_test.go proves EnsureContentAddressedFixture's correctness
// contract directly (fixture_cache.go's design points 1-4): identical
// content converges on the identical path, different content never
// collides, concurrent publishers of the same content never corrupt or
// duplicate work, and a directory lacking the completeness marker is never
// trusted as a hit.

func TestEnsureContentAddressedFixture_IdenticalContentSamePath(t *testing.T) {
	files := map[string][]byte{
		"go.mod":        []byte("module example.com/fx\n\ngo 1.21\n"),
		"model/impl.go": []byte("package model\n\nfunc F() int { return 1 }\n"),
	}
	a := EnsureContentAddressedFixture(files, t.Fatalf)
	b := EnsureContentAddressedFixture(files, t.Fatalf)
	if a != b {
		t.Fatalf("identical content published to different paths: %q vs %q", a, b)
	}
	if _, err := os.Stat(filepath.Join(a, "go.mod")); err != nil {
		t.Fatalf("published fixture missing go.mod: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a, "model", "impl.go")); err != nil {
		t.Fatalf("published fixture missing model/impl.go: %v", err)
	}
}

func TestEnsureContentAddressedFixture_DifferentContentDifferentPath(t *testing.T) {
	filesA := map[string][]byte{"go.mod": []byte("module example.com/a\n\ngo 1.21\n")}
	filesB := map[string][]byte{"go.mod": []byte("module example.com/b\n\ngo 1.21\n")}
	a := EnsureContentAddressedFixture(filesA, t.Fatalf)
	b := EnsureContentAddressedFixture(filesB, t.Fatalf)
	if a == b {
		t.Fatalf("different content published to the SAME path: %q", a)
	}
}

func TestEnsureContentAddressedFixture_DifferentPathsSameBytesDifferentHash(t *testing.T) {
	// Proves the path is part of the hash, not just the concatenated bytes:
	// swapping which file holds which content must NOT collide.
	filesA := map[string][]byte{
		"one.go": []byte("AAAA"),
		"two.go": []byte("BBBB"),
	}
	filesB := map[string][]byte{
		"one.go": []byte("BBBB"),
		"two.go": []byte("AAAA"),
	}
	a := EnsureContentAddressedFixture(filesA, t.Fatalf)
	b := EnsureContentAddressedFixture(filesB, t.Fatalf)
	if a == b {
		t.Fatalf("different path->content assignment collided at the same published path: %q", a)
	}
}

func TestEnsureContentAddressedFixture_MarkerPresentAfterPublish(t *testing.T) {
	files := map[string][]byte{"go.mod": []byte("module example.com/marker\n\ngo 1.21\n")}
	root := EnsureContentAddressedFixture(files, t.Fatalf)
	if !fixtureCachePublishedComplete(root) {
		t.Fatalf("expected completeness marker present at %s after a successful publish", root)
	}
}

func TestEnsureContentAddressedFixture_DirectoryWithoutMarkerNeverTrusted(t *testing.T) {
	files := map[string][]byte{"go.mod": []byte("module example.com/nomarker\n\ngo 1.21\n")}
	hash := hashFixtureTree(files)
	published := filepath.Join(FixtureCacheRoot(), hash)

	// Simulate a directory that exists at the published path but was NOT
	// produced by this package's own publish sequence (no marker) -- e.g. a
	// human poking around, or a hypothetical future bug. Must not be
	// trusted as complete.
	if err := os.MkdirAll(published, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(published) })

	if fixtureCachePublishedComplete(published) {
		t.Fatalf("a directory with no completeness marker must never be treated as a published hit")
	}
}

// TestEnsureContentAddressedFixture_ConcurrentPublishersConverge simulates
// several goroutines racing to publish the IDENTICAL fixture content at
// once (the in-process analogue of two separate `go test` processes racing
// -- the real race this design defends against, see design point 1). Every
// caller must return the SAME path, every caller must succeed (no onFail
// call), and the published directory must be genuinely complete afterward.
func TestEnsureContentAddressedFixture_ConcurrentPublishersConverge(t *testing.T) {
	files := map[string][]byte{
		"go.mod":         []byte("module example.com/race\n\ngo 1.21\n"),
		"model/impl.go":  []byte("package model\n\nfunc Race() int { return 1 }\n"),
		"model/other.go": []byte("package model\n\nfunc Other() int { return 2 }\n"),
	}

	const n = 12
	var wg sync.WaitGroup
	paths := make([]string, n)
	var failMu sync.Mutex
	var failures []string
	onFail := func(format string, args ...any) {
		failMu.Lock()
		failures = append(failures, fmt.Sprintf(format, args...))
		failMu.Unlock()
	}

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			paths[idx] = EnsureContentAddressedFixture(files, onFail)
		}(i)
	}
	wg.Wait()

	if len(failures) > 0 {
		t.Fatalf("concurrent publishers reported %d infra failures, expected zero: %v", len(failures), failures)
	}
	first := paths[0]
	for i, p := range paths {
		if p != first {
			t.Fatalf("publisher %d returned a different path (%q) than publisher 0 (%q) for identical content", i, p, first)
		}
	}
	if !fixtureCachePublishedComplete(first) {
		t.Fatalf("published path %s is not marked complete after concurrent publish", first)
	}
	if _, err := os.Stat(filepath.Join(first, "model", "other.go")); err != nil {
		t.Fatalf("published fixture missing model/other.go: %v", err)
	}

	// No orphaned .tmp- sibling directories should remain (the loser(s)
	// must have cleaned up their private tmp dir via the deferred
	// os.RemoveAll in EnsureContentAddressedFixture).
	hash := hashFixtureTree(files)
	leftovers, err := filepath.Glob(filepath.Join(FixtureCacheRoot(), hash+".tmp-*"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("expected no orphaned .tmp- directories after concurrent publish, found: %v", leftovers)
	}
}

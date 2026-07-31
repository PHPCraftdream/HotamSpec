package gate

import (
	"os"
	"path/filepath"
	"testing"
)

// engineFPFixture builds a temp module root with the three watched package
// directories, each containing a .go file with the given content. Returns the
// module root path.
func engineFPFixture(t *testing.T, genContent, ontContent, loaderContent string) string {
	t.Helper()
	root := t.TempDir()
	for _, pkg := range []struct {
		rel, content string
	}{
		{"internal/generator", genContent},
		{"internal/ontology", ontContent},
		{"internal/loader", loaderContent},
	} {
		dir := filepath.Join(root, filepath.FromSlash(pkg.rel))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "file.go"), []byte(pkg.content), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", dir, err)
		}
	}
	return root
}

func TestEngineDocsFingerprint_Deterministic(t *testing.T) {
	t.Parallel()
	root := engineFPFixture(t, "package generator\n", "package ontology\n", "package loader\n")
	h1, err := EngineDocsFingerprint(root)
	if err != nil {
		t.Fatalf("EngineDocsFingerprint (first): %v", err)
	}
	h2, err := EngineDocsFingerprint(root)
	if err != nil {
		t.Fatalf("EngineDocsFingerprint (second): %v", err)
	}
	if h1 != h2 {
		t.Fatalf("expected the same fingerprint across two calls with no changes, got %q then %q", h1, h2)
	}
}

func TestEngineDocsFingerprint_ContentSensitive(t *testing.T) {
	t.Parallel()
	root := engineFPFixture(t, "package generator\n", "package ontology\n", "package loader\n")
	h1, err := EngineDocsFingerprint(root)
	if err != nil {
		t.Fatalf("EngineDocsFingerprint (before): %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(root, filepath.FromSlash("internal/generator/file.go")),
		[]byte("package generator // CHANGED\n"), 0o644,
	); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	h2, err := EngineDocsFingerprint(root)
	if err != nil {
		t.Fatalf("EngineDocsFingerprint (after): %v", err)
	}
	if h1 == h2 {
		t.Fatalf("expected different fingerprints after mutating internal/generator/file.go, got the same %q both times", h1)
	}
}

func TestEngineDocsFingerprint_OutsideDirInsensitive(t *testing.T) {
	t.Parallel()
	root := engineFPFixture(t, "package generator\n", "package ontology\n", "package loader\n")
	h1, err := EngineDocsFingerprint(root)
	if err != nil {
		t.Fatalf("EngineDocsFingerprint (before): %v", err)
	}
	// Create a file in a package OUTSIDE the three watched ones.
	outsideDir := filepath.Join(root, filepath.FromSlash("internal/invariants"))
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "something.go"), []byte("package invariants\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	h2, err := EngineDocsFingerprint(root)
	if err != nil {
		t.Fatalf("EngineDocsFingerprint (after): %v", err)
	}
	if h1 != h2 {
		t.Fatalf("expected the SAME fingerprint after changing a file outside the three watched packages, got %q then %q", h1, h2)
	}
}

func TestEngineDocsFingerprint_MissingAllReturnsError(t *testing.T) {
	t.Parallel()
	root := t.TempDir() // empty — no internal/* dirs
	_, err := EngineDocsFingerprint(root)
	if err == nil {
		t.Fatalf("expected a non-nil error when none of the three engine packages exist, got nil")
	}
}

func TestEngineDocsFingerprint_SkipsBuildArtifacts(t *testing.T) {
	t.Parallel()
	root := engineFPFixture(t, "package generator\n", "package ontology\n", "package loader\n")
	// Drop a build artifact (.exe) into one of the watched packages.
	artifactPath := filepath.Join(root, filepath.FromSlash("internal/generator/hotam.exe"))
	if err := os.WriteFile(artifactPath, []byte("binary junk"), 0o755); err != nil {
		t.Fatalf("WriteFile artifact: %v", err)
	}
	h1, err := EngineDocsFingerprint(root)
	if err != nil {
		t.Fatalf("EngineDocsFingerprint (with artifact): %v", err)
	}
	if err := os.Remove(artifactPath); err != nil {
		t.Fatalf("Remove artifact: %v", err)
	}
	h2, err := EngineDocsFingerprint(root)
	if err != nil {
		t.Fatalf("EngineDocsFingerprint (without artifact): %v", err)
	}
	if h1 != h2 {
		t.Fatalf("expected the SAME fingerprint regardless of build artifacts, got %q then %q", h1, h2)
	}
}

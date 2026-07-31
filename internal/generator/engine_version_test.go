package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildEngineVersionMD_ContainsFingerprintAndBanner proves the new
// ENGINE-VERSION.md file is written with the current fingerprint and the
// expected banner/note text.
func TestBuildEngineVersionMD_ContainsFingerprintAndBanner(t *testing.T) {
	t.Parallel()
	// Build a minimal module root with the three watched package dirs.
	root := t.TempDir()
	for _, pkg := range []string{"internal/generator", "internal/ontology", "internal/loader"} {
		dir := filepath.Join(root, filepath.FromSlash(pkg))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "file.go"), []byte("package "+filepath.Base(pkg)+"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	content, err := BuildEngineVersionMD(root)
	if err != nil {
		t.Fatalf("BuildEngineVersionMD: %v", err)
	}

	// Must carry the do-not-hand-edit banner.
	if !strings.Contains(content, generatedHeaderComment) {
		t.Errorf("ENGINE-VERSION.md missing the generated-file banner:\n%s", content)
	}

	// Must carry the fingerprint label line with a non-empty hex value.
	if !strings.Contains(content, engineFingerprintLabel) {
		t.Errorf("ENGINE-VERSION.md missing the fingerprint label %q:\n%s", engineFingerprintLabel, content)
	}

	// Must carry the human-readable note pointing to gen-spec.
	if !strings.Contains(content, "regenerate via") {
		t.Errorf("ENGINE-VERSION.md missing the regenerate note:\n%s", content)
	}

	// The fingerprint label line must contain a backtick-wrapped 16-char hex value.
	idx := strings.Index(content, engineFingerprintLabel)
	if idx < 0 {
		t.Fatalf("fingerprint label not found")
	}
	rest := content[idx+len(engineFingerprintLabel):]
	openTick := strings.Index(rest, "`")
	closeTick := strings.Index(rest[openTick+1:], "`")
	if openTick < 0 || closeTick < 0 {
		t.Fatalf("ENGINE-VERSION.md fingerprint label not followed by a backtick-wrapped value:\n%s", content)
	}
	stampedHex := rest[openTick+1 : openTick+1+closeTick]
	if len(stampedHex) != engineFingerprintShortLen {
		t.Errorf("expected stamped fingerprint to be %d hex chars, got %d (%q)", engineFingerprintShortLen, len(stampedHex), stampedHex)
	}
}

// TestBuildEngineVersionMD_Deterministic proves two calls with the same
// moduleRoot produce byte-identical content (idempotency).
func TestBuildEngineVersionMD_Deterministic(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, pkg := range []string{"internal/generator", "internal/ontology", "internal/loader"} {
		dir := filepath.Join(root, filepath.FromSlash(pkg))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "file.go"), []byte("package "+filepath.Base(pkg)+"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	c1, err := BuildEngineVersionMD(root)
	if err != nil {
		t.Fatalf("BuildEngineVersionMD (first): %v", err)
	}
	c2, err := BuildEngineVersionMD(root)
	if err != nil {
		t.Fatalf("BuildEngineVersionMD (second): %v", err)
	}
	if c1 != c2 {
		t.Errorf("expected byte-identical content across two calls, got differences")
	}
}

// TestBuildEngineVersionMD_ErrorOnMissingPackages proves a moduleRoot with
// none of the three packages returns a non-nil error.
func TestBuildEngineVersionMD_ErrorOnMissingPackages(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := BuildEngineVersionMD(root)
	if err == nil {
		t.Fatalf("expected a non-nil error when none of the three engine packages exist")
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// TestVendorRecorder_RequiresSpecGoMod proves vendorRecorder refuses to
// write into a domain whose spec/ tree is not yet its own Go module (no
// spec/go.mod) -- see vendorRecorder's own doc comment for why this is a
// hard requirement rather than a "create go.mod too" convenience.
func TestVendorRecorder_RequiresSpecGoMod(t *testing.T) {
	domainDir := t.TempDir()
	_, err := vendorRecorder(domainDir)
	if err == nil {
		t.Fatalf("expected an error for a domain with no spec/go.mod, got nil")
	}
	if !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("error = %v, want a message naming the missing go.mod", err)
	}
}

// TestVendorRecorder_WritesBannerStampedCopy proves vendorRecorder writes
// spec/hotamspec/hotamspec.go, banner-stamped, byte-identical to
// recordervendor.Source() -- the exact content check_recorder_current
// expects to find.
func TestVendorRecorder_WritesBannerStampedCopy(t *testing.T) {
	domainDir := t.TempDir()
	specDir := filepath.Join(domainDir, "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("MkdirAll spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module fixture-spec\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	written, err := vendorRecorder(domainDir)
	if err != nil {
		t.Fatalf("vendorRecorder: %v", err)
	}
	wantPath := filepath.Join(specDir, "hotamspec", "hotamspec.go")
	if written != wantPath {
		t.Fatalf("vendorRecorder returned %q, want %q", written, wantPath)
	}

	got, err := os.ReadFile(written)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != recordervendor.Source() {
		t.Fatalf("written content does not match recordervendor.Source()")
	}
}

// TestVendorRecorder_Idempotent proves re-running vendorRecorder against the
// same domain twice produces byte-identical output both times -- the
// "always safe to re-run, always the current canon" contract described in
// cmdVendorRecorder's doc comment.
func TestVendorRecorder_Idempotent(t *testing.T) {
	domainDir := t.TempDir()
	specDir := filepath.Join(domainDir, "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("MkdirAll spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module fixture-spec\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	if _, err := vendorRecorder(domainDir); err != nil {
		t.Fatalf("first vendorRecorder: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(specDir, "hotamspec", "hotamspec.go"))
	if err != nil {
		t.Fatalf("ReadFile after first run: %v", err)
	}

	if _, err := vendorRecorder(domainDir); err != nil {
		t.Fatalf("second vendorRecorder: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(specDir, "hotamspec", "hotamspec.go"))
	if err != nil {
		t.Fatalf("ReadFile after second run: %v", err)
	}

	if string(first) != string(second) {
		t.Fatalf("vendorRecorder is not idempotent: first run and second run produced different content")
	}
}

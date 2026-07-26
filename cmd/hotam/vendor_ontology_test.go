package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ontologyvendor "github.com/PHPCraftdream/HotamSpec/internal/ontology/vendor"
)

// TestVendorOntology_RequiresSpecGoMod proves vendorOntology refuses to
// write into a domain whose spec/ tree is not yet its own Go module (no
// spec/go.mod) -- see vendorOntology's own doc comment for why this is a
// hard requirement rather than a "create go.mod too" convenience, mirroring
// TestVendorRecorder_RequiresSpecGoMod.
func TestVendorOntology_RequiresSpecGoMod(t *testing.T) {
	domainDir := t.TempDir()
	_, err := vendorOntology(domainDir)
	if err == nil {
		t.Fatalf("expected an error for a domain with no spec/go.mod, got nil")
	}
	if !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("error = %v, want a message naming the missing go.mod", err)
	}
}

// TestVendorOntology_WritesBannerStampedCopies proves vendorOntology writes
// spec/hotamontology/requirement.go and spec/hotamontology/registry.go,
// banner-stamped, byte-identical to ontologyvendor.RequirementSource() /
// RegistrySource() -- the exact content check_ontology_vendor_current
// expects to find.
func TestVendorOntology_WritesBannerStampedCopies(t *testing.T) {
	domainDir := t.TempDir()
	specDir := filepath.Join(domainDir, "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("MkdirAll spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module fixture-spec\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	written, err := vendorOntology(domainDir)
	if err != nil {
		t.Fatalf("vendorOntology: %v", err)
	}
	wantReq := filepath.Join(specDir, "hotamontology", "requirement.go")
	wantReg := filepath.Join(specDir, "hotamontology", "registry.go")
	if len(written) != 2 || written[0] != wantReq || written[1] != wantReg {
		t.Fatalf("vendorOntology returned %v, want [%q %q]", written, wantReq, wantReg)
	}

	gotReq, err := os.ReadFile(wantReq)
	if err != nil {
		t.Fatalf("ReadFile requirement.go: %v", err)
	}
	if string(gotReq) != ontologyvendor.RequirementSource() {
		t.Fatalf("written requirement.go content does not match ontologyvendor.RequirementSource()")
	}

	gotReg, err := os.ReadFile(wantReg)
	if err != nil {
		t.Fatalf("ReadFile registry.go: %v", err)
	}
	if string(gotReg) != ontologyvendor.RegistrySource() {
		t.Fatalf("written registry.go content does not match ontologyvendor.RegistrySource()")
	}
}

// TestVendorOntology_Idempotent proves re-running vendorOntology against the
// same domain twice produces byte-identical output both times -- the
// "always safe to re-run, always the current canon" contract described in
// cmdVendorOntology's doc comment, mirroring TestVendorRecorder_Idempotent.
func TestVendorOntology_Idempotent(t *testing.T) {
	domainDir := t.TempDir()
	specDir := filepath.Join(domainDir, "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("MkdirAll spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module fixture-spec\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	if _, err := vendorOntology(domainDir); err != nil {
		t.Fatalf("first vendorOntology: %v", err)
	}
	reqPath := filepath.Join(specDir, "hotamontology", "requirement.go")
	regPath := filepath.Join(specDir, "hotamontology", "registry.go")
	firstReq, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("ReadFile requirement.go after first run: %v", err)
	}
	firstReg, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatalf("ReadFile registry.go after first run: %v", err)
	}

	if _, err := vendorOntology(domainDir); err != nil {
		t.Fatalf("second vendorOntology: %v", err)
	}
	secondReq, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("ReadFile requirement.go after second run: %v", err)
	}
	secondReg, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatalf("ReadFile registry.go after second run: %v", err)
	}

	if string(firstReq) != string(secondReq) {
		t.Fatalf("vendorOntology is not idempotent: requirement.go differs between runs")
	}
	if string(firstReg) != string(secondReg) {
		t.Fatalf("vendorOntology is not idempotent: registry.go differs between runs")
	}
}

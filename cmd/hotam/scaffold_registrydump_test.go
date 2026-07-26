package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ontologyvendor "github.com/PHPCraftdream/HotamSpec/internal/ontology/vendor"
)

// TestScaffoldRegistrydump_RequiresSpecGoMod proves scaffoldRegistrydump
// refuses to write into a domain whose spec/ tree is not yet its own Go
// module — mirroring TestVendorOntology_RequiresSpecGoMod.
func TestScaffoldRegistrydump_RequiresSpecGoMod(t *testing.T) {
	domainDir := t.TempDir()
	_, err := scaffoldRegistrydump(domainDir)
	if err == nil {
		t.Fatalf("expected an error for a domain with no spec/go.mod, got nil")
	}
	if !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("error = %v, want a message naming the missing go.mod", err)
	}
}

// TestScaffoldRegistrydump_RequiresVendoredOntology proves scaffoldRegistrydump
// refuses to write registrydump/main.go before `hotam vendor-ontology` has
// vendored spec/hotamontology — the generated source imports that package
// directly, so scaffolding first would leave a non-compiling stub.
func TestScaffoldRegistrydump_RequiresVendoredOntology(t *testing.T) {
	domainDir := t.TempDir()
	specDir := filepath.Join(domainDir, "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("MkdirAll spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module fixture-spec\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	_, err := scaffoldRegistrydump(domainDir)
	if err == nil {
		t.Fatalf("expected an error when spec/hotamontology has not been vendored yet, got nil")
	}
	if !strings.Contains(err.Error(), "vendor-ontology") {
		t.Fatalf("error = %v, want a message pointing at `hotam vendor-ontology`", err)
	}
}

// TestScaffoldRegistrydump_WritesCompilableProgram proves the happy path: a
// domain with spec/go.mod + vendored spec/hotamontology gets a
// spec/registrydump/main.go that (a) imports the module's own root package
// under the exact module path declared in spec/go.mod, and (b) is idempotent
// across repeated runs.
func TestScaffoldRegistrydump_WritesCompilableProgram(t *testing.T) {
	domainDir := t.TempDir()
	specDir := filepath.Join(domainDir, "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("MkdirAll spec: %v", err)
	}
	const modulePath = "fixture-spec-module"
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(specDir, "hotamontology"), 0o755); err != nil {
		t.Fatalf("MkdirAll hotamontology: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "hotamontology", "requirement.go"), []byte(ontologyvendor.RequirementSource()), 0o644); err != nil {
		t.Fatalf("WriteFile requirement.go: %v", err)
	}

	written, err := scaffoldRegistrydump(domainDir)
	if err != nil {
		t.Fatalf("scaffoldRegistrydump: %v", err)
	}
	want := filepath.Join(specDir, "registrydump", "main.go")
	if written != want {
		t.Fatalf("scaffoldRegistrydump returned %q, want %q", written, want)
	}

	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("ReadFile main.go: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `spec "`+modulePath+`"`) {
		t.Errorf("main.go does not import the domain's own module path %q:\n%s", modulePath, content)
	}
	if !strings.Contains(content, "DO NOT EDIT") {
		t.Errorf("main.go is missing the do-not-edit banner:\n%s", content)
	}
	if !strings.Contains(content, "spec.Requirements.All()") {
		t.Errorf("main.go does not call spec.Requirements.All():\n%s", content)
	}

	// Idempotent: re-running produces byte-identical output.
	if _, err := scaffoldRegistrydump(domainDir); err != nil {
		t.Fatalf("second scaffoldRegistrydump: %v", err)
	}
	second, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("ReadFile main.go (second run): %v", err)
	}
	if string(second) != content {
		t.Fatalf("scaffoldRegistrydump is not idempotent: main.go differs between runs")
	}
}

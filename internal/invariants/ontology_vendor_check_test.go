package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	ontologyvendor "github.com/PHPCraftdream/HotamSpec/internal/ontology/vendor"
)

// writeVendoredOntologyFixture writes content at
// <tmp>/spec/hotamontology/<fileName> and returns tmp as the domain
// directory (g.DomainDir) -- mirrors writeVendoredRecorderFixture's shape
// (recorder_check_test.go) but targets the fixed ontology mirror directory
// check_ontology_vendor_current reads.
func writeVendoredOntologyFixture(t *testing.T, fileName, content string) string {
	t.Helper()
	tmp := t.TempDir()
	full := filepath.Join(tmp, "spec", "hotamontology", fileName)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return tmp
}

func TestCheckOntologyVendorCurrent_NoOpWhenNeverVendored(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{DomainDir: t.TempDir()}
	if vs := runCheck(t, "check_ontology_vendor_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a domain that never vendored the ontology mirror, got %v", vs)
	}
}

func TestCheckOntologyVendorCurrent_NoOpWhenDomainDirEmpty(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	if vs := runCheck(t, "check_ontology_vendor_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a graph with no DomainDir, got %v", vs)
	}
}

func TestCheckOntologyVendorCurrent_OK_WhenIdenticalToCanon(t *testing.T) {
	t.Parallel()
	domainDir := t.TempDir()
	specDir := filepath.Join(domainDir, "spec", "hotamontology")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "requirement.go"), []byte(ontologyvendor.RequirementSource()), 0o644); err != nil {
		t.Fatalf("WriteFile requirement.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "registry.go"), []byte(ontologyvendor.RegistrySource()), 0o644); err != nil {
		t.Fatalf("WriteFile registry.go: %v", err)
	}
	g := &ontology.Graph{DomainDir: domainDir}
	if vs := runCheck(t, "check_ontology_vendor_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a freshly vendored, unmodified copy, got %v", vs)
	}
}

// TestCheckOntologyVendorCurrent_MUTATION_TamperedBodyFires is the mutation
// probe the task's own verification step calls for: corrupt a vendored
// file's BODY (after the banner, exactly the shape a hand-edit would take),
// confirm the check goes red, then restore byte-identical content and
// confirm it goes green again.
func TestCheckOntologyVendorCurrent_MUTATION_TamperedBodyFires(t *testing.T) {
	t.Parallel()
	genuine := ontologyvendor.RequirementSource()
	tampered := genuine + "\n// hand-edited: this line was never vendored\n"

	domainDir := writeVendoredOntologyFixture(t, "requirement.go", tampered)
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_ontology_vendor_current", g)
	if len(vs) == 0 {
		t.Fatalf("expected a violation for a tampered vendored requirement.go, got none")
	}
	for _, v := range vs {
		if v.Check != "check_ontology_vendor_current" {
			t.Errorf("violation Check = %q, want check_ontology_vendor_current", v.Check)
		}
	}

	// Restore byte-identical content -- the check must go back to green,
	// proving this is a live content comparison, not a one-shot flag.
	target := filepath.Join(domainDir, "spec", "hotamontology", "requirement.go")
	if err := os.WriteFile(target, []byte(genuine), 0o644); err != nil {
		t.Fatalf("restore fixture: %v", err)
	}
	if vs := runCheck(t, "check_ontology_vendor_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations after restoring byte-identical content, got %v", vs)
	}
}

func TestCheckOntologyVendorCurrent_FiresWhenBannerMissing(t *testing.T) {
	t.Parallel()
	// The exact canon body, but with NO banner at all -- a file that was
	// hand-created rather than produced by `hotam vendor-ontology`.
	domainDir := writeVendoredOntologyFixture(t, "requirement.go", ontologyvendor.RequirementBodyForHash())
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_ontology_vendor_current", g)
	if len(vs) == 0 {
		t.Fatalf("expected a violation for a vendored requirement.go with no do-not-edit banner, got none")
	}
}

func TestCheckOntologyVendorCurrent_FiresWhenStale(t *testing.T) {
	t.Parallel()
	// Simulate an OLDER canon: banner intact, but the body differs (as if an
	// engine upgrade changed the ontology mirror and this domain never
	// re-vendored).
	stale := ontologyvendor.Banner + "package hotamontology\n\n// an older, different requirement shape\n"
	domainDir := writeVendoredOntologyFixture(t, "requirement.go", stale)
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_ontology_vendor_current", g)
	if len(vs) == 0 {
		t.Fatalf("expected a violation for a stale vendored requirement.go, got none")
	}
}

// TestCheckOntologyVendorCurrent_RegistryFileCheckedIndependently proves the
// registry.go half of the mirror is checked on its own: a genuine
// requirement.go alongside a tampered registry.go still fires (only) for
// the registry file, and vice versa is symmetric by construction (same
// helper, same code path).
func TestCheckOntologyVendorCurrent_RegistryFileCheckedIndependently(t *testing.T) {
	t.Parallel()
	domainDir := t.TempDir()
	specDir := filepath.Join(domainDir, "spec", "hotamontology")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "requirement.go"), []byte(ontologyvendor.RequirementSource()), 0o644); err != nil {
		t.Fatalf("WriteFile requirement.go: %v", err)
	}
	tamperedRegistry := ontologyvendor.RegistrySource() + "\n// hand-edited\n"
	if err := os.WriteFile(filepath.Join(specDir, "registry.go"), []byte(tamperedRegistry), 0o644); err != nil {
		t.Fatalf("WriteFile registry.go: %v", err)
	}

	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_ontology_vendor_current", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation (registry.go only), got %d: %v", len(vs), vs)
	}
	if got := vs[0].Message; got == "" {
		t.Fatalf("expected a non-empty violation message")
	}
}

// TestCheckOntologyVendorCurrent_StakeholderFileChecked proves the
// stakeholder.go mirror is checked: genuine copy is green, a hand-edited one
// fires, and a domain vendored before it existed (file absent) stays green.
func TestCheckOntologyVendorCurrent_StakeholderFileChecked(t *testing.T) {
	t.Parallel()
	genuine := ontologyvendor.StakeholderSource()

	domainDir := writeVendoredOntologyFixture(t, "stakeholder.go", genuine)
	g := &ontology.Graph{DomainDir: domainDir}
	if vs := runCheck(t, "check_ontology_vendor_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a genuine stakeholder.go, got %v", vs)
	}

	target := filepath.Join(domainDir, "spec", "hotamontology", "stakeholder.go")
	if err := os.WriteFile(target, []byte(genuine+"\n// hand-edited\n"), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	if vs := runCheck(t, "check_ontology_vendor_current", g); len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation for a tampered stakeholder.go, got %v", vs)
	}

	if err := os.Remove(target); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if vs := runCheck(t, "check_ontology_vendor_current", g); len(vs) != 0 {
		t.Fatalf("expected honest no-op for an absent stakeholder.go, got %v", vs)
	}
}

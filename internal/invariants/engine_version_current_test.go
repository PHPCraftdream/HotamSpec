package invariants

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// engineVersionFixture builds a temp module root with the three watched engine
// package dirs and a domain dir under domains/<name>/. Returns the domain dir
// (usable as g.DomainDir) — resolveModuleRoot will walk up two levels to find
// the module root, exactly as genSpec's repoRootForDomain does.
func engineVersionFixture(t *testing.T) (moduleRoot, domainDir string) {
	t.Helper()
	moduleRoot = t.TempDir()
	for _, pkg := range []string{"internal/generator", "internal/ontology", "internal/loader"} {
		dir := filepath.Join(moduleRoot, filepath.FromSlash(pkg))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "file.go"), []byte("package "+filepath.Base(pkg)+"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	domainDir = filepath.Join(moduleRoot, "domains", "test-domain")
	genDir := filepath.Join(domainDir, "docs", "gen")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatalf("MkdirAll docs/gen: %v", err)
	}
	return moduleRoot, domainDir
}

// writeEngineVersionMD writes a fake ENGINE-VERSION.md with the given
// fingerprint hex stamped on the label line.
func writeEngineVersionMD(t *testing.T, domainDir, fingerprint string) {
	t.Helper()
	versionPath := filepath.Join(domainDir, "docs", "gen", "ENGINE-VERSION.md")
	content := fmt.Sprintf("<!-- banner -->\n\n%s `%s`\n", engineFingerprintLabel, fingerprint)
	if err := os.WriteFile(versionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile ENGINE-VERSION.md: %v", err)
	}
}

// currentShortFingerprint computes the 16-char short fingerprint the check
// would produce for the given moduleRoot.
func currentShortFingerprint(t *testing.T, moduleRoot string) string {
	t.Helper()
	fp, err := gate.EngineDocsFingerprint(moduleRoot)
	if err != nil {
		t.Fatalf("EngineDocsFingerprint: %v", err)
	}
	if len(fp) > 16 {
		return fp[:16]
	}
	return fp
}

func TestCheckEngineDocsFingerprintCurrent_NoOpWhenDomainDirEmpty(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	if vs := runCheck(t, "check_engine_docs_fingerprint_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a graph with no DomainDir, got %v", vs)
	}
}

func TestCheckEngineDocsFingerprintCurrent_NoOpWhenFileAbsent(t *testing.T) {
	t.Parallel()
	_, domainDir := engineVersionFixture(t)
	g := &ontology.Graph{DomainDir: domainDir}
	// Precondition: no docs/gen/ENGINE-VERSION.md.
	if _, err := os.Stat(filepath.Join(domainDir, "docs", "gen", "ENGINE-VERSION.md")); !os.IsNotExist(err) {
		t.Fatalf("precondition: ENGINE-VERSION.md must not exist")
	}
	if vs := runCheck(t, "check_engine_docs_fingerprint_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations when ENGINE-VERSION.md is absent, got %v", vs)
	}
}

func TestCheckEngineDocsFingerprintCurrent_OK_WhenFingerprintMatches(t *testing.T) {
	t.Parallel()
	moduleRoot, domainDir := engineVersionFixture(t)
	writeEngineVersionMD(t, domainDir, currentShortFingerprint(t, moduleRoot))
	g := &ontology.Graph{DomainDir: domainDir}
	if vs := runCheck(t, "check_engine_docs_fingerprint_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations when the stamped fingerprint matches current, got %v", vs)
	}
}

func TestCheckEngineDocsFingerprintCurrent_FiresWhenFingerprintDiffers(t *testing.T) {
	t.Parallel()
	_, domainDir := engineVersionFixture(t)
	// Stamp a DELIBERATELY wrong fingerprint (different from what the current
	// engine would produce for this module root).
	writeEngineVersionMD(t, domainDir, "0000000000000000")
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_engine_docs_fingerprint_current", g)
	if len(vs) == 0 {
		t.Fatalf("expected a violation when the stamped fingerprint differs from current, got none")
	}
	for _, v := range vs {
		if v.Check != "check_engine_docs_fingerprint_current" {
			t.Errorf("violation Check = %q, want check_engine_docs_fingerprint_current", v.Check)
		}
		if v.ID != domainDir {
			t.Errorf("violation ID = %q, want %q", v.ID, domainDir)
		}
	}
}

func TestCheckEngineDocsFingerprintCurrent_MUTATION_FiresThenClears(t *testing.T) {
	t.Parallel()
	moduleRoot, domainDir := engineVersionFixture(t)
	g := &ontology.Graph{DomainDir: domainDir}

	// Start clean: stamped fingerprint matches current.
	writeEngineVersionMD(t, domainDir, currentShortFingerprint(t, moduleRoot))
	if vs := runCheck(t, "check_engine_docs_fingerprint_current", g); len(vs) != 0 {
		t.Fatalf("precondition: matching fingerprint must start clean, got %v", vs)
	}

	// Tamper: stamp a wrong fingerprint.
	writeEngineVersionMD(t, domainDir, "ffffffffffffffff")
	vs := runCheck(t, "check_engine_docs_fingerprint_current", g)
	if len(vs) == 0 {
		t.Fatalf("expected a violation after tampering with the fingerprint, got none")
	}

	// Restore: re-stamp the correct fingerprint.
	writeEngineVersionMD(t, domainDir, currentShortFingerprint(t, moduleRoot))
	if vs := runCheck(t, "check_engine_docs_fingerprint_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations after restoring the correct fingerprint, got %v", vs)
	}
}

func TestCheckEngineDocsFingerprintCurrent_NoOpWhenEnginePackagesAbsent(t *testing.T) {
	t.Parallel()
	// A module root with NO engine packages — simulates a consumer repo
	// whose engine is a compiled binary without source.
	moduleRoot := t.TempDir()
	domainDir := filepath.Join(moduleRoot, "domains", "test-domain")
	genDir := filepath.Join(domainDir, "docs", "gen")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Write an ENGINE-VERSION.md with a stale fingerprint.
	writeEngineVersionMD(t, domainDir, "deadbeefdeadbeef")
	g := &ontology.Graph{DomainDir: domainDir}
	// The check must be an honest no-op: it cannot compute the current
	// engine fingerprint (no packages found), so it stays silent.
	if vs := runCheck(t, "check_engine_docs_fingerprint_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations when engine packages are absent (cannot compute current fingerprint), got %v", vs)
	}
}

package source

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

func TestProjectSourcesResolveFromDeclaredDomain(t *testing.T) {
	root := t.TempDir()
	domainDir := filepath.Join(root, "domains", "language")
	if err := os.MkdirAll(domainDir, 0o700); err != nil {
		t.Fatal(err)
	}
	upstream := filepath.Join(root, "upstream")
	if err := os.MkdirAll(upstream, 0o700); err != nil {
		t.Fatal(err)
	}
	content := []byte("# Pinned language\n\nA normative assertion.\n")
	if err := os.WriteFile(filepath.Join(upstream, "spec.md"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	// Explicit domain ownership must win over another process-selected project.
	t.Setenv(paths.EnvProjectRoot, t.TempDir())
	t.Setenv(paths.EnvDomainsRoot, t.TempDir())
	g := &ontology.Graph{
		DomainDir: domainDir,
		SpecificationSources: []ontology.SpecificationSource{{
			ID: "language", Path: "project://upstream/spec.md", Version: "pinned", SHA256: sha256Hex(content),
		}},
	}
	checks := VerifySources(g)
	if len(checks) != 1 || checks[0].Status != StatusVerified || checks[0].ActualSHA256 != sha256Hex(content) {
		t.Fatalf("project source resolution did not verify declared project's bytes: %#v", checks)
	}
	if checks[0].Version != "pinned" {
		t.Fatalf("source version lost: %#v", checks[0])
	}
	if err := os.WriteFile(filepath.Join(upstream, "spec.md"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	checks = VerifySources(g)
	if len(checks) != 1 || checks[0].Status != StatusDrift {
		t.Fatalf("project source drift was not detected: %#v", checks)
	}
}

func TestProjectSourcePathsRejectInvalidRootsAndEscapes(t *testing.T) {
	root := t.TempDir()
	domainDir := filepath.Join(root, "domains", "language")
	if err := os.MkdirAll(domainDir, 0o700); err != nil {
		t.Fatal(err)
	}
	g := &ontology.Graph{DomainDir: domainDir}
	for _, path := range []string{
		"project://", "project:///absolute.md", "project://C:/absolute.md",
		"project://C:relative.md", "project://\\server\\share.md",
		"project://../outside.md", "project://upstream/../../outside.md",
	} {
		t.Run(path, func(t *testing.T) {
			if resolved, err := ResolvePath(g, path); err == nil {
				t.Fatalf("invalid project path %q resolved to %q", path, resolved)
			}
		})
	}
	if resolved, err := ResolvePath(&ontology.Graph{DomainDir: t.TempDir()}, "project://spec.md"); err == nil {
		t.Fatalf("unmarked domain must not borrow a process/framework root: %q", resolved)
	}
	if resolved, err := ResolvePath(nil, "project://spec.md"); err == nil {
		t.Fatalf("project source without domain resolved to %q", resolved)
	}
}

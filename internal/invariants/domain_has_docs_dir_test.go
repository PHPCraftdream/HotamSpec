package invariants

import (
	"os"
	"path/filepath"
	"testing"
)

// realDomainsRoot are the on-disk domain directories the docs/ wrapper check
// (R-domain-has-docs-dir) walks. Paths are relative to the internal/invariants
// test working directory.
var realDomainsRoot = []string{
	"../../domains/hotam-spec-self",
	"../../domains/hotam-dev",
}

// TestDomainHasDocsDir_RealDomains enforces R-domain-has-docs-dir: every
// domains/<name>/ directory MUST contain a docs/ subdirectory that wraps the
// generated docs/gen/ plus any hand-written domain material. This is a sibling
// filesystem-structure check to the check_domain_* / check_agent_has_docs_subdir
// family -- those graph Checks are honest no-ops because the Check(*Graph)
// signature cannot reach the filesystem, so the real enforcement is this test
// walking the actual domain directories. It fails the moment any domain loses its
// docs/ directory.
func TestDomainHasDocsDir_RealDomains(t *testing.T) {
	t.Parallel()
	for _, dir := range realDomainsRoot {
		dir := dir
		t.Run(dir, func(t *testing.T) {
			t.Parallel()
			docsDir := filepath.Join(dir, "docs")
			info, err := os.Stat(docsDir)
			if err != nil {
				t.Fatalf("domains/ missing docs/ directory at %s (R-domain-has-docs-dir): %v", docsDir, err)
			}
			if !info.IsDir() {
				t.Errorf("%s exists but is not a directory (R-domain-has-docs-dir wants a docs/ subdir)", docsDir)
			}
		})
	}
}

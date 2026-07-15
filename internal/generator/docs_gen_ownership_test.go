package generator

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// realDomainsDocsGen pairs each on-disk domain with the paths the ownership test
// (R-domain-owns-docs-gen) walks. Paths are relative to the internal/generator
// test working directory, matching byteidentical_test.go's domainGraphPath.
var realDomainsDocsGen = []struct {
	name, graphPath, genDir string
}{
	{"hotam-spec-self", "../../domains/hotam-spec-self/graph.json", "../../domains/hotam-spec-self/docs/gen"},
	{"hotam-dev", "../../domains/hotam-dev/graph.json", "../../domains/hotam-dev/docs/gen"},
}

// genTopLevelOwned are the file names the generator always writes directly into
// docs/gen/ (the fixed half of cmd/hotam/gen_spec.go's mdDocs slice). The
// conditional DECISIONS.md / ENTITIES.md and the thinking/ + tools/ subtrees are
// derived from the graph below, not hardcoded here.
var genTopLevelOwned = []string{
	"REQUIREMENTS.md", "TENSIONS.md", "OPEN.md", "UNENFORCED.md",
	"GLOSSARY.md", "HISTORY.md", "CONSTITUTION.md", "FRAMEWORK-INVARIANTS.md",
	"PIPELINE.md",
	"REPO-MAP.md",
	"atoms-operator.md", "atoms-substrate.md", "atoms-discipline.md", "atoms-check.md",
	"live-state.md",
	"AGENT-CONTEXT.md",
	"graph.json",
}

// ownedGenRelPaths returns the set of paths (relative to docs/gen/) that the
// generator writes for g -- the generator's own output manifest, mirroring the
// write list in cmd/hotam/gen_spec.go. The thinking/ and tools/ basenames are
// the KEYS of BuildThinkingDocs / BuildToolDocs (derived, not hardcoded, so a
// renamed generator function cannot silently strand an orphan file); DECISIONS.md
// and ENTITIES.md are gated by DecisionsMDHasContent / EntitiesMDHasContent, the
// same predicates gen_spec.go uses to decide whether to write them.
func ownedGenRelPaths(t *testing.T, g *ontology.Graph) map[string]struct{} {
	t.Helper()
	owned := map[string]struct{}{}
	for _, f := range genTopLevelOwned {
		owned[f] = struct{}{}
	}
	if DecisionsMDHasContent(g) {
		owned["DECISIONS.md"] = struct{}{}
	}
	if EntitiesMDHasContent(g) {
		owned["ENTITIES.md"] = struct{}{}
	}
	for slug := range BuildThinkingDocs() {
		owned[filepath.ToSlash(filepath.Join("thinking", slug+".md"))] = struct{}{}
	}
	for cmd := range BuildToolDocs(false) {
		owned[filepath.ToSlash(filepath.Join("tools", cmd+".md"))] = struct{}{}
	}
	// tools/INDEX.md is a generator-owned entry-point page (BuildToolDocsIndex)
	// written alongside the per-tool docs; it is not a key of BuildToolDocs.
	owned[filepath.ToSlash(filepath.Join("tools", "INDEX.md"))] = struct{}{}
	return owned
}

// TestDomainOwnsDocsGen_NoForeignOrOrphanFiles enforces R-domain-owns-docs-gen:
// every file present under domains/<name>/docs/gen/ on disk MUST be one the
// generator declares it owns for that domain. A cross-domain dump or an orphan
// left behind by a renamed generator function shows up as an on-disk path that is
// not in the generator's output manifest, failing this test.
func TestDomainOwnsDocsGen_NoForeignOrOrphanFiles(t *testing.T) {
	t.Parallel()
	for _, d := range realDomainsDocsGen {
		d := d
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			g, err := loader.LoadGraph(d.graphPath)
			if err != nil {
				t.Fatalf("LoadGraph(%s): %v", d.graphPath, err)
			}
			owned := ownedGenRelPaths(t, g)

			var foreign []string
			walkErr := filepath.WalkDir(d.genDir, func(path string, e fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if e.IsDir() {
					return nil
				}
				base := e.Name()
				// Dotfiles (e.g. .gitkeep) are git directory-tracking
				// scaffolding, not generated docs -- the claim governs "the
				// markdown generated from that domain's graph", which is never
				// a dotfile. They are exempt by kind, not to mask drift.
				if strings.HasPrefix(base, ".") {
					return nil
				}
				rel, relErr := filepath.Rel(d.genDir, path)
				if relErr != nil {
					return relErr
				}
				rel = filepath.ToSlash(rel)
				if _, ok := owned[rel]; !ok {
					foreign = append(foreign, rel)
				}
				return nil
			})
			if walkErr != nil {
				t.Fatalf("walk %s: %v", d.genDir, walkErr)
			}
			if len(foreign) != 0 {
				t.Errorf("domains/%s/docs/gen/ holds files the generator does not own "+
					"(R-domain-owns-docs-gen -- no cross-domain/orphan files): %v", d.name, foreign)
			}
		})
	}
}

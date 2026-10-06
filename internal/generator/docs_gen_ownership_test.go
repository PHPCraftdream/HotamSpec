package generator

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// realDomainsDocsGen pairs each on-disk domain with the paths the ownership test
// (R-domain-owns-docs-gen) walks. Paths are relative to the internal/generator
// test working directory, matching byteidentical_test.go's domainGraphPath.
// There is no per-domain frameworkDir anymore: task #357 promoted the shared
// framework files (GLOSSARY.md + tools/*.md) to the PROJECT-root framework/
// directory, and returned FRAMEWORK-INVARIANTS.md to docs/gen/ (per-domain).
var realDomainsDocsGen = []struct {
	name, graphPath, genDir string
}{
	{"hotam-spec-self", "../../domains/hotam-spec-self/graph.json", "../../domains/hotam-spec-self/docs/gen"},
	{"hotam-dev", "../../domains/hotam-dev/graph.json", "../../domains/hotam-dev/docs/gen"},
}

// projectFrameworkDir is the single PROJECT-root framework/ directory shared
// across all domains (task #357): GLOSSARY.md + tools/*.md live here in ONE
// copy, byte-identical regardless of which domain regenerated it. Relative to
// the internal/generator test working directory (../../framework = repo root).
const projectFrameworkDir = "../../framework"

// genTopLevelOwned are the file names the generator always writes directly into
// docs/gen/ regardless of domain content. Task #357 removed GLOSSARY.md from
// this set (promoted to the project-root framework/). Task #364 moved
// REQUIREMENTS.md/OPEN.md/UNENFORCED.md/FRAMEWORK-INVARIANTS.md/HISTORY.md/
// CONSTITUTION.md/TRACEABILITY.md/COVERAGE.md/REPO-MAP.md/live-state.md/
// AGENT-CONTEXT.md/graph.json OUT of this fixed set and into the
// content-gated predicate list below (ownedGenRelPaths), mirroring the SAME
// conditional-write pattern DECISIONS.md/ENTITIES.md/TENSIONS.md/
// PIPELINE.md/MODELS.md already used: a genuinely empty domain now withholds
// all of them. atoms-*.md remain here — their own gate (shouldWriteAtoms,
// cmd/hotam/gen_spec.go) is keyed on the CONSUMER PROFILE's empty-notice
// check, not on this task's graph-emptiness predicate, so they stay
// unconditional for this ownership test's purposes (the two real fixture
// domains below are both full-profile). tools/*.md no longer live under
// docs/gen/ either (moved to the project-root framework/tools/ — see
// ownedProjectFrameworkRelPaths).
var genTopLevelOwned = []string{
	// SPEC.md (W1.3) is written only under `hotam gen-spec --spec` -- opt-in,
	// like DECISIONS.md/ENTITIES.md's predicate-gated entries below -- but
	// unlike those two, its presence has no cheap in-memory predicate to gate
	// on (BuildSpec's own cost IS a real `go test` run per verified_by entry,
	// see internal/generator/spec.go's doc comment): if it exists on disk at
	// all, only the generator could have written it (W2.3's genSpec never
	// orphan-deletes it), so it belongs in the always-owned set exactly like
	// the other top-level entries, present or not.
	"SPEC.md",
	// ENGINE-VERSION.md (task #400, W2.3) is always written unconditionally
	// (not content-gated) — its content is engine metadata (a content-hash
	// fingerprint of the engine's own source packages), not a graph-derived
	// projection, so every domain has one regardless of graph emptiness.
	"ENGINE-VERSION.md",
	"atoms-operator.md", "atoms-substrate.md", "atoms-discipline.md", "atoms-check.md",
}

// ownedGenRelPaths returns the set of paths (relative to docs/gen/) that the
// generator writes for g -- the generator's own output manifest, mirroring the
// write list in cmd/hotam/gen_spec.go. The thinking/ basenames are the KEYS of
// BuildThinkingDocs (derived, not hardcoded, so a renamed generator function
// cannot silently strand an orphan file); every entry below is gated by its
// own *MDHasContent(g) predicate (internal/generator), the SAME predicates
// gen_spec.go itself queries to decide whether to write each file.
func ownedGenRelPaths(t *testing.T, g *ontology.Graph) map[string]struct{} {
	t.Helper()
	owned := map[string]struct{}{}
	for _, f := range genTopLevelOwned {
		owned[f] = struct{}{}
	}
	layout, err := docbundle.NewLayout(g.Languages, g.DefaultLanguage)
	if err != nil {
		t.Fatalf("docbundle.NewLayout: %v", err)
	}
	if g.SelfExecutingAtoms || layout.Multilingual() {
		// SPEC shards are generator-owned because only `gen-spec --spec` writes
		// them (W1.3); staleness/cleanup belongs to check_spec_md_current's
		// compareSpecDocuments (obsolete-shard detection; see
		// internal/invariants/spec_shards_test.go). Derive this inventory the
		// same way BuildSpecDocumentsFromRows does, so a renamed writer cannot
		// strand an orphan unnoticed.
		packages := map[string]struct{}{}
		for _, req := range g.Requirements {
			if req.Status != ontology.StatusREJECTED {
				packages[gate.SpecPackage(req)] = struct{}{}
			}
		}
		for _, language := range layout.LanguagesForViews() {
			if indexPath, err := layout.SpecIndexPath(language); err == nil {
				key := strings.TrimPrefix(filepath.ToSlash(indexPath), "docs/gen/")
				owned[key] = struct{}{}
			}
			for pkg := range packages {
				if shardPath, err := layout.SpecShardPath(language, pkg+".md"); err == nil {
					key := strings.TrimPrefix(filepath.ToSlash(shardPath), "docs/gen/")
					owned[key] = struct{}{}
				}
			}
		}
	}
	if DecisionsMDHasContent(g) {
		owned["DECISIONS.md"] = struct{}{}
	}
	if EntitiesMDHasContent(g) {
		owned["ENTITIES.md"] = struct{}{}
	}
	if TensionsMDHasContent(g) {
		owned["TENSIONS.md"] = struct{}{}
	}
	if PipelineMDHasContent(g) {
		owned["PIPELINE.md"] = struct{}{}
	}
	if ModelsMDHasContent(g) {
		owned["MODELS.md"] = struct{}{}
	}
	if RequirementsMDHasContent(g) {
		owned["REQUIREMENTS.md"] = struct{}{}
	}
	if OpenMDHasContent(g) {
		owned["OPEN.md"] = struct{}{}
	}
	if UnenforcedMDHasContent(g) {
		owned["UNENFORCED.md"] = struct{}{}
	}
	if FrameworkInvariantsMDHasContent(g) {
		owned["FRAMEWORK-INVARIANTS.md"] = struct{}{}
	}
	if HistoryMDHasContent(g) {
		owned["HISTORY.md"] = struct{}{}
	}
	if ConstitutionMDHasContent(g) {
		owned["CONSTITUTION.md"] = struct{}{}
	}
	if TraceabilityMDHasContent(g) {
		owned["TRACEABILITY.md"] = struct{}{}
	}
	if CoverageMDHasContent(g) {
		owned["COVERAGE.md"] = struct{}{}
	}
	if RepoMapMDHasContent(g) {
		owned["REPO-MAP.md"] = struct{}{}
	}
	if AgentContextMDHasContent(g) {
		owned["AGENT-CONTEXT.md"] = struct{}{}
	}
	if LiveStateMDHasContent(g) {
		owned["live-state.md"] = struct{}{}
	}
	if GraphJSONHasContent(g) {
		owned["graph.json"] = struct{}{}
	}
	for slug := range BuildThinkingDocs() {
		owned[filepath.ToSlash(filepath.Join("thinking", slug+".md"))] = struct{}{}
	}
	return owned
}

// ownedProjectFrameworkRelPaths returns the set of paths (relative to the
// project-root framework/) that the generator writes — the project-shared
// framework output manifest (task #357). GLOSSARY.md is the one top-level file;
// tools/*.md basenames are the KEYS of BuildToolDocs plus INDEX.md
// (BuildToolDocsIndex). FRAMEWORK-INVARIANTS.md is NOT here — it is per-domain,
// living under docs/gen/ (see ownedGenRelPaths).
func ownedProjectFrameworkRelPaths(t *testing.T, g *ontology.Graph) map[string]struct{} {
	t.Helper()
	owned := map[string]struct{}{}
	owned["GLOSSARY.md"] = struct{}{}
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

// TestProjectOwnsFramework_NoForeignOrOrphanFiles is the PROJECT-root
// framework/-side ownership test (task #357): every file present under the
// shared project-root framework/ directory on disk MUST be one the generator
// declares it owns. Unlike the per-domain docs/gen/ test (which loops over
// domains), this walks a SINGLE shared directory — one copy for the whole
// project, byte-identical regardless of which domain regenerated it. An orphan
// left behind by the #355→#357 migration (a stray FRAMEWORK-INVARIANTS.md that
// used to live in the per-domain framework/) or a cross-domain dump shows up as
// an on-disk path not in the generator's framework/ output manifest, failing
// this test.
func TestProjectOwnsFramework_NoForeignOrOrphanFiles(t *testing.T) {
	t.Parallel()
	// The project-root framework/ is profile-independent in its FILE SET (the
	// only profile variance is whether Planned-tool pages are written, which
	// only shrinks the set under consumer); full-profile BuildToolDocs gives
	// the superset, so a superset-owned check passes under both profiles.
	g, err := loader.LoadGraph(realDomainsDocsGen[0].graphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	owned := ownedProjectFrameworkRelPaths(t, g)

	var foreign []string
	walkErr := filepath.WalkDir(projectFrameworkDir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		base := e.Name()
		if strings.HasPrefix(base, ".") {
			return nil
		}
		rel, relErr := filepath.Rel(projectFrameworkDir, path)
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
		t.Fatalf("walk %s: %v", projectFrameworkDir, walkErr)
	}
	if len(foreign) != 0 {
		t.Errorf("project-root framework/ holds files the generator does not own "+
			"(task #357 -- no orphan/migration-leftover files): %v", foreign)
	}
}

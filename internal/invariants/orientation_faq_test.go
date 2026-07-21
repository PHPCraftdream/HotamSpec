package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// orientationFAQFixture builds a temp domain directory laid out as
// <tmp>/domains/<name>/ so that resolveCrystalRepoRoot's tier-1 rule
// (domainDir's parent is "domains" -> repoRoot = grandparent = tmp) makes
// link resolution DETERMINISTIC against tmp, independent of whatever
// paths.ProjectRootOrRaise() would resolve from the test process's CWD.
// It writes a manifest.json (carrying the supplied orientation_faq JSON
// fragment) and a crystal CLAUDE.md (the supplied crystal text) at
// <domainDir>/CLAUDE.md (the local-crystal path resolveCrystalPath tries
// first), and creates the named link target files under tmp (repo-root
// relative). Returns the domain directory usable as g.DomainDir.
func orientationFAQFixture(t *testing.T, manifestOrientationFAQ, crystal string, linkFiles map[string]string) string {
	t.Helper()
	tmp := t.TempDir()
	domainDir := filepath.Join(tmp, "domains", "testdomain")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("MkdirAll domainDir: %v", err)
	}
	manifest := `{
  "purpose": "test domain",
  "parent": null,
  "orientation_faq": ` + manifestOrientationFAQ + `
}
`
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if crystal != "" {
		if err := os.WriteFile(filepath.Join(domainDir, "CLAUDE.md"), []byte(crystal), 0o644); err != nil {
			t.Fatalf("WriteFile CLAUDE.md: %v", err)
		}
	}
	// linkFiles keys are repo-root-relative (slash) paths; the target file
	// is created under tmp (the derived repo root).
	for rel, content := range linkFiles {
		target := filepath.Join(tmp, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("MkdirAll link target dir: %v", err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile link target: %v", err)
		}
	}
	return domainDir
}

func TestCheckOrientationFAQAnswered_NoOpWhenDomainDirEmpty(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected no violations for an in-memory graph with no DomainDir, got %v", vs)
	}
}

func TestCheckOrientationFAQAnswered_NoOpWhenNoManifest(t *testing.T) {
	t.Parallel()
	// A domain dir with NO manifest.json at all -- honest no-op, the same
	// missing-manifest default every sibling opt-in resolver establishes.
	tmp := t.TempDir()
	g := &ontology.Graph{DomainDir: tmp}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a domain with no manifest.json, got %v", vs)
	}
}

func TestCheckOrientationFAQAnswered_NoOpWhenFieldAbsent(t *testing.T) {
	t.Parallel()
	// A manifest.json that exists but carries NO orientation_faq key --
	// honest no-op ("no committed opt-in = no lie"), the whole point of the
	// opt-in boundary.
	domainDir := orientationFAQFixture(t, "", "# crystal\n", nil)
	// orientationFAQFixture always writes the key; overwrite manifest with
	// one that lacks it entirely to exercise the absent-field path.
	manifest := `{
  "purpose": "test domain",
  "parent": null
}
`
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	g := &ontology.Graph{DomainDir: domainDir}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a domain whose manifest lacks orientation_faq, got %v", vs)
	}
}

func TestCheckOrientationFAQAnswered_PassesWhenKeywordsInline(t *testing.T) {
	t.Parallel()
	faq := `[
    {"question": "what is this?", "keywords": ["tension graph", "purpose"]}
  ]`
	crystal := "# Crystal\n\nThis project is a tension graph. Its purpose is orientation.\n"
	domainDir := orientationFAQFixture(t, faq, crystal, nil)
	g := &ontology.Graph{DomainDir: domainDir}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected no violations when all keywords are inline in the crystal, got %v", vs)
	}
}

func TestCheckOrientationFAQAnswered_KeywordsMatchCaseInsensitively(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "q", "keywords": ["PURPOSE"]}]`
	crystal := "# Crystal\n\nthe purpose of this domain\n"
	domainDir := orientationFAQFixture(t, faq, crystal, nil)
	g := &ontology.Graph{DomainDir: domainDir}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected case-insensitive keyword match to satisfy the entry, got %v", vs)
	}
}

func TestCheckOrientationFAQAnswered_PassesWhenLinkOneHopResolves(t *testing.T) {
	t.Parallel()
	faq := `[
    {"question": "lifecycle?", "link": "domains/testdomain/docs/gen/PIPELINE.md"}
  ]`
	// The crystal references the link path (as a markdown link), and the
	// target file really exists under the repo root -- exactly one hop.
	crystal := "# Crystal\n\nSee [pipeline](domains/testdomain/docs/gen/PIPELINE.md) for the lifecycle.\n"
	linkFiles := map[string]string{"domains/testdomain/docs/gen/PIPELINE.md": "# Pipeline\n..."}
	domainDir := orientationFAQFixture(t, faq, crystal, linkFiles)
	g := &ontology.Graph{DomainDir: domainDir}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected no violations when the link is in the crystal and resolves to a real file, got %v", vs)
	}
}

func TestCheckOrientationFAQAnswered_PassesWhenLinkIsBarePathNotMarkdown(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "q", "link": "domains/testdomain/docs/gen/REQS.md"}]`
	// A bare path string (backticked, not a [text](path) link) must also
	// count as "the crystal references this file".
	crystal := "# Crystal\n\nFull list at `domains/testdomain/docs/gen/REQS.md`.\n"
	linkFiles := map[string]string{"domains/testdomain/docs/gen/REQS.md": "# Reqs"}
	domainDir := orientationFAQFixture(t, faq, crystal, linkFiles)
	g := &ontology.Graph{DomainDir: domainDir}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected a bare-path link reference to satisfy the entry, got %v", vs)
	}
}

func TestCheckOrientationFAQAnswered_FiresWhenKeywordMissing(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "what is this?", "keywords": ["tension graph", "NONEXISTENT_MARKER"]}]`
	crystal := "# Crystal\n\nThis project is a tension graph.\n"
	domainDir := orientationFAQFixture(t, faq, crystal, nil)
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation for a missing keyword, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "what is this?") {
		t.Errorf("expected a violation naming the question, got %v", vs)
	}
}

func TestCheckOrientationFAQAnswered_FiresWhenLinkMissingFromCrystal(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "lifecycle?", "link": "domains/testdomain/docs/gen/PIPELINE.md"}]`
	// Crystal does NOT reference the link path at all.
	crystal := "# Crystal\n\nNothing about the lifecycle here.\n"
	linkFiles := map[string]string{"domains/testdomain/docs/gen/PIPELINE.md": "# Pipeline"}
	domainDir := orientationFAQFixture(t, faq, crystal, linkFiles)
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation when the link is absent from the crystal, got %d: %v", len(vs), vs)
	}
}

func TestCheckOrientationFAQAnswered_FiresWhenLinkTargetFileMissing(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "lifecycle?", "link": "domains/testdomain/docs/gen/PIPELINE.md"}]`
	// Crystal references the link, but the target file does NOT exist on
	// disk -- a dangling one-hop pointer.
	crystal := "# Crystal\n\nSee [pipeline](domains/testdomain/docs/gen/PIPELINE.md).\n"
	domainDir := orientationFAQFixture(t, faq, crystal, nil) // no linkFiles -> file absent
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation when the link target file is missing, got %d: %v", len(vs), vs)
	}
}

func TestCheckOrientationFAQAnswered_FiresWhenEntryHasNeitherSignal(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "empty entry"}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQFixture(t, faq, crystal, nil)
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation for an entry with neither keywords nor a link, got %d: %v", len(vs), vs)
	}
}

func TestCheckOrientationFAQAnswered_FiresPerEntryWhenNoCrystal(t *testing.T) {
	t.Parallel()
	faq := `[
    {"question": "q1", "keywords": ["foo"]},
    {"question": "q2", "link": "x.md"}
  ]`
	// crystal == "" -> no CLAUDE.md written -> resolveCrystalPath returns "".
	domainDir := orientationFAQFixture(t, faq, "", nil)
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 2 {
		t.Fatalf("expected 2 violations (one per declared question) when no crystal exists, got %d: %v", len(vs), vs)
	}
}

// TestCheckOrientationFAQAnswered_MUTATION_BrokenAnswerFiresThenRestores is
// the mutation probe the task's own verification step demands: start green
// (all answers reachable), BREAK one declared answer (remove the keyword the
// crystal relied on), confirm the check goes RED on exactly that question,
// then restore the crystal and confirm it goes GREEN again -- proving the
// check actually polices orientability rather than always passing.
func TestCheckOrientationFAQAnswered_MUTATION_BrokenAnswerFiresThenRestores(t *testing.T) {
	t.Parallel()
	faq := `[
    {"question": "purpose?", "keywords": ["tension graph"]},
    {"question": "lifecycle?", "link": "domains/testdomain/docs/gen/PIPELINE.md"}
  ]`
	crystal := "# Crystal\n\nThis is a tension graph.\nSee [pipeline](domains/testdomain/docs/gen/PIPELINE.md).\n"
	linkFiles := map[string]string{"domains/testdomain/docs/gen/PIPELINE.md": "# Pipeline"}
	domainDir := orientationFAQFixture(t, faq, crystal, linkFiles)
	g := &ontology.Graph{DomainDir: domainDir}

	// Baseline: green.
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("baseline: expected 0 violations, got %v", vs)
	}

	// BREAK: rewrite the crystal so the "purpose?" keyword is gone (the
	// answer is no longer inline) -- exactly the drift the invariant exists
	// to catch. Expect ONE violation, naming "purpose?".
	brokenCrystal := "# Crystal\n\nNothing relevant here.\nSee [pipeline](domains/testdomain/docs/gen/PIPELINE.md).\n"
	crystalPath := filepath.Join(domainDir, "CLAUDE.md")
	if err := os.WriteFile(crystalPath, []byte(brokenCrystal), 0o644); err != nil {
		t.Fatalf("WriteFile broken crystal: %v", err)
	}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("after break: expected exactly 1 violation (the orphaned purpose question), got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "purpose?") {
		t.Errorf("after break: expected the violation to name the broken question %q, got %v", "purpose?", vs)
	}
	// The link-based "lifecycle?" question must NOT have fired -- it is
	// still reachable -- proving the check isolates the broken answer rather
	// than failing everything.
	if hasViolationFor(vs, "lifecycle?") {
		t.Errorf("after break: the still-reachable lifecycle question should NOT have fired, got %v", vs)
	}

	// RESTORE: put the original crystal back. The check must go green again,
	// proving this is a live comparison, not a one-shot flag.
	if err := os.WriteFile(crystalPath, []byte(crystal), 0o644); err != nil {
		t.Fatalf("WriteFile restored crystal: %v", err)
	}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("after restore: expected 0 violations again, got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_RealHotamSpecSelfSelfExample passes the
// REAL self-hosting domain's own declared orientation_faq against its own
// generated crystal (the repo root CLAUDE.md) -- the integration proof that
// the showcase self-example actually satisfies the invariant it ships, not
// just an isolated fixture. Skips when the repo layout is not present (e.g.
// running this test tree in isolation without the parent repo).
func TestCheckOrientationFAQAnswered_RealHotamSpecSelfSelfExample(t *testing.T) {
	t.Parallel()
	// domains/hotam-spec-self is two levels up from internal/invariants
	// (internal/invariants -> internal -> repo root).
	selfDomainDir := filepath.Join("..", "..", "domains", "hotam-spec-self")
	if _, err := os.Stat(filepath.Join(selfDomainDir, "manifest.json")); err != nil {
		t.Skipf("hotam-spec-self manifest not found at %s -- running outside the repo layout", selfDomainDir)
	}
	g := &ontology.Graph{DomainDir: selfDomainDir}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected the real hotam-spec-self orientation_faq self-example to pass with 0 violations, got %v", vs)
	}
}

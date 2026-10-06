package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

// Both tests pin review finding P3-13: the COMPARATIVE renders inside the
// freshness checks (check_domain_claude_md_current, check_agent_context_md_current,
// check_language_outputs_current) must consume the publication-flavored
// violation snapshot (invariants.PublicationViolationsFromPhaseOne) — the same
// flavor genSpec writes its projections from — not the unfiltered phase-1
// priorViolations. With the fix, tampering an unrelated on-disk projection
// (the ENGINE-VERSION fingerprint stamp) reports ONLY
// check_engine_docs_fingerprint_current: no projection check claims a file is
// stale when a re-run would rewrite it byte-identically.

// tamperEngineVersionStamp rewrites the stamped engine content fingerprint in
// docs/gen/ENGINE-VERSION.md to a wrong hex value, leaving every byte of the
// staged tree otherwise intact.
func tamperEngineVersionStamp(t *testing.T, domainDir string) {
	t.Helper()
	engineVersionPath := filepath.Join(domainDir, "docs", "gen", "ENGINE-VERSION.md")
	engineBytes, err := os.ReadFile(engineVersionPath)
	if err != nil {
		t.Fatalf("read ENGINE-VERSION.md: %v", err)
	}
	re := regexp.MustCompile(`(\*\*Engine content fingerprint:\*\*\s*\x60)[0-9a-f]+(\x60)`)
	tampered := re.ReplaceAll(engineBytes, []byte("${1}0123456789abcdef${2}"))
	if string(tampered) == string(engineBytes) {
		t.Fatal("precondition: tamper must have changed the stamped fingerprint")
	}
	if err := os.WriteFile(engineVersionPath, tampered, 0o644); err != nil {
		t.Fatalf("write tampered ENGINE-VERSION.md: %v", err)
	}
}

// seedFingerprintSource seeds the deterministic fingerprint inputs
// (internal/{generator,ontology,loader}) under the synthetic project root —
// see TestGenSpec_AgentContextConvergesInOnePass for the full rationale.
func seedFingerprintSource(t *testing.T, projectRoot string) {
	t.Helper()
	for _, pkg := range []string{"generator", "ontology", "loader"} {
		dir := filepath.Join(projectRoot, "internal", pkg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "dummy.go"), []byte("package "+pkg+"\n"), 0o644); err != nil {
			t.Fatalf("write dummy.go: %v", err)
		}
	}
}

// TestClaudeMDCurrent_UsesPublicationFlavor_ComparativeRender pins P3-13 for
// the crystal check: after a clean genSpec, tampering ONLY the ENGINE-VERSION
// stamp must produce exactly ONE check_engine_docs_fingerprint_current
// violation and ZERO check_domain_claude_md_current /
// check_agent_context_md_current violations, because both projection checks
// render their fresh comparison from the publication flavor that omits the
// stale ENGINE-VERSION signal — and a re-run therefore rewrites the tree
// byte-identically.
func TestClaudeMDCurrent_UsesPublicationFlavor_ComparativeRender(t *testing.T) {
	projectRoot, domainDir := initDomainUnderRoot(t, "p3-13-flavor", "2026-10-05")
	manifestPath := filepath.Join(domainDir, "manifest.json")
	manifest := `{"self_hosting":false,"gen_profile":"full","parent":null,"orientation_faq":[{"question":"project identity","keywords":["p3-13-flavor"]}]}` + "\n"
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	graphPath := filepath.Join(domainDir, "graph.json")
	graph, err := loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("load graph.json: %v", err)
	}
	graph.Stakeholders = append(graph.Stakeholders, ontology.Stakeholder{ID: "st-ops", Name: "Ops", Domain: "p3-13-flavor", DeclOrder: 1})
	if err := loader.WriteGraph(graphPath, graph); err != nil {
		t.Fatalf("write graph.json: %v", err)
	}
	crystalPath := filepath.Join(projectRoot, "CLAUDE.md")
	if err := os.WriteFile(filepath.Join(projectRoot, paths.MarkerFilename), nil, 0o644); err != nil {
		t.Fatalf("write project marker: %v", err)
	}
	seedFingerprintSource(t, projectRoot)
	if _, _, err := genSpec(domainDir, crystalPath, "2026-10-05", "full", false); err != nil {
		t.Fatalf("warm-up genSpec: %v", err)
	}
	if _, err := os.Stat(filepath.Join(domainDir, "docs", "gen", "ENGINE-VERSION.md")); err != nil {
		t.Fatalf("precondition: ENGINE-VERSION.md must exist after warm-up: %v", err)
	}

	tamperEngineVersionStamp(t, domainDir)

	violations, err := allViolationsAsOf(domainDir, "2026-10-05")
	if err != nil {
		t.Fatalf("allViolationsAsOf: %v", err)
	}
	fingerprint := 0
	for _, v := range violations {
		switch v.Check {
		case "check_engine_docs_fingerprint_current":
			fingerprint++
		case "check_domain_claude_md_current", "check_agent_context_md_current":
			t.Errorf("stale ENGINE-VERSION stamp must not flag %s (P3-13: comparative render must use the publication flavor): %s", v.Check, v.Message)
		default:
			t.Errorf("unexpected violation from tampered stamp: [%s] %s: %s", v.Check, v.ID, v.Message)
		}
	}
	if fingerprint != 1 {
		t.Fatalf("want exactly 1 check_engine_docs_fingerprint_current violation, got %d", fingerprint)
	}

	// One genSpec pass repairs the stamp; the tree is then stable.
	if _, _, err := genSpec(domainDir, crystalPath, "2026-10-05", "full", false); err != nil {
		t.Fatalf("repair genSpec: %v", err)
	}
	violations, err = allViolationsAsOf(domainDir, "2026-10-05")
	if err != nil {
		t.Fatalf("allViolationsAsOf after repair: %v", err)
	}
	for _, v := range violations {
		t.Errorf("violation survived repair genSpec: [%s] %s: %s", v.Check, v.ID, v.Message)
	}
	before, err := snapshotGeneratedTree(t, projectRoot, domainDir, crystalPath)
	if err != nil {
		t.Fatalf("snapshot before: %v", err)
	}
	if _, _, err := genSpec(domainDir, crystalPath, "2026-10-05", "full", false); err != nil {
		t.Fatalf("second genSpec: %v", err)
	}
	after, err := snapshotGeneratedTree(t, projectRoot, domainDir, crystalPath)
	if err != nil {
		t.Fatalf("snapshot after: %v", err)
	}
	if len(before) != len(after) {
		t.Fatalf("generated tree size changed between identical runs: %d != %d", len(before), len(after))
	}
	for path, hash := range before {
		if afterHash, ok := after[path]; !ok || hash != afterHash {
			t.Errorf("file changed between identical genSpec runs: %s", path)
		}
	}
}

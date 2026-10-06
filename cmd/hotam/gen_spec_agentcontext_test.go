package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

// TestGenSpec_AgentContextConvergesInOnePass pins review finding P3-11:
// genSpec must embed the publication violation snapshot into AGENT-CONTEXT.md
// (not self-compute the full AllViolations set at render time against the
// stale pre-write disk), so a tampered ENGINE-VERSION fingerprint stamp —
// which makes check_engine_docs_fingerprint_current fire against the stale
// disk — is repaired in ONE genSpec pass, the written AGENT-CONTEXT.md lists
// no stale STRUCTURE signals, and a second identical run is byte-identical
// across the whole generated tree.
func TestGenSpec_AgentContextConvergesInOnePass(t *testing.T) {
	projectRoot, domainDir := initDomainUnderRoot(t, "agentcontext-converge", "2026-10-05")
	manifestPath := filepath.Join(domainDir, "manifest.json")
	manifest := `{"self_hosting":false,"gen_profile":"full","parent":null,"orientation_faq":[{"question":"project identity","keywords":["agentcontext-converge"]}]}` + "\n"
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	// Seed one invariant-clean Stakeholder so the freshly-initialized graph is
	// non-empty (generator.AgentContextMDHasContent == !g.IsEmpty()) and
	// AGENT-CONTEXT.md is actually written under the full profile. loader.
	// WriteGraph (not a hand edit) keeps graph.json and graph.lock in sync.
	graphPath := filepath.Join(domainDir, "graph.json")
	graph, err := loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("load graph.json: %v", err)
	}
	graph.Stakeholders = append(graph.Stakeholders, ontology.Stakeholder{ID: "st-ops", Name: "Ops", Domain: "agentcontext-converge", DeclOrder: 1})
	if err := loader.WriteGraph(graphPath, graph); err != nil {
		t.Fatalf("write graph.json: %v", err)
	}
	crystalPath := filepath.Join(projectRoot, "CLAUDE.md")
	if err := os.WriteFile(filepath.Join(projectRoot, paths.MarkerFilename), nil, 0o644); err != nil {
		t.Fatalf("write project marker: %v", err)
	}
	// Seed the fingerprint source: EngineDocsFingerprint hashes
	// <moduleRoot>/internal/{generator,ontology,loader}, and for a
	// domains/<name> layout the module root IS the project root. Deterministic
	// bytes => deterministic fingerprint => genSpec's stamp and the check's
	// recomputation always agree.
	for _, pkg := range []string{"generator", "ontology", "loader"} {
		dir := filepath.Join(projectRoot, "internal", pkg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "dummy.go"), []byte("package "+pkg+"\n"), 0o644); err != nil {
			t.Fatalf("write dummy.go: %v", err)
		}
	}
	if _, _, err := genSpec(domainDir, crystalPath, "2026-10-05", "full", false); err != nil {
		t.Fatalf("warm-up genSpec: %v", err)
	}
	engineVersionPath := filepath.Join(domainDir, "docs", "gen", "ENGINE-VERSION.md")
	if _, err := os.Stat(engineVersionPath); err != nil {
		t.Fatalf("precondition: ENGINE-VERSION.md must exist after warm-up (fingerprint computable): %v", err)
	}

	// TAMPER: replace the stamped fingerprint with a wrong hex value — the
	// stale-stamp state that made AGENT-CONTEXT.md embed a stale
	// check_engine_docs_fingerprint_current signal before the P3-11 fix.
	engineBytes, err := os.ReadFile(engineVersionPath)
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(\*\*Engine content fingerprint:\*\*\s*\x60)[0-9a-f]+(\x60)`)
	tampered := re.ReplaceAll(engineBytes, []byte("${1}0123456789abcdef${2}"))
	if string(tampered) == string(engineBytes) {
		t.Fatal("precondition: tamper must have changed the stamped fingerprint")
	}
	if err := os.WriteFile(engineVersionPath, tampered, 0o644); err != nil {
		t.Fatalf("write tampered ENGINE-VERSION.md: %v", err)
	}

	// RUN 1.
	if _, _, err := genSpec(domainDir, crystalPath, "2026-10-05", "full", false); err != nil {
		t.Fatalf("run 1 genSpec: %v", err)
	}
	agentContextPath := filepath.Join(domainDir, "docs", "gen", "AGENT-CONTEXT.md")
	agentContext, err := os.ReadFile(agentContextPath)
	if err != nil {
		t.Fatalf("read AGENT-CONTEXT.md after run 1: %v", err)
	}
	for _, stale := range []string{"check_engine_docs_fingerprint_current", "check_domain_claude_md_current"} {
		if strings.Contains(string(agentContext), stale) {
			t.Errorf("AGENT-CONTEXT.md after one genSpec pass still embeds stale STRUCTURE signal %q", stale)
		}
	}
	violations, err := allViolationsAsOf(domainDir, "2026-10-05")
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	for _, v := range violations {
		t.Errorf("violation survived one genSpec pass: [%s] %s: %s", v.Check, v.ID, v.Message)
	}
	before, err := snapshotGeneratedTree(t, projectRoot, domainDir, crystalPath)
	if err != nil {
		t.Fatalf("snapshot before: %v", err)
	}

	// RUN 2: byte-identical across the whole generated set.
	if _, _, err := genSpec(domainDir, crystalPath, "2026-10-05", "full", false); err != nil {
		t.Fatalf("run 2 genSpec: %v", err)
	}
	after, err := snapshotGeneratedTree(t, projectRoot, domainDir, crystalPath)
	if err != nil {
		t.Fatalf("snapshot after: %v", err)
	}
	if len(before) != len(after) {
		t.Fatalf("generated tree size changed between identical runs: %d != %d", len(before), len(after))
	}
	for path, hash := range before {
		afterHash, ok := after[path]
		if !ok {
			t.Errorf("file disappeared between identical runs: %s", path)
			continue
		}
		if hash != afterHash {
			t.Errorf("file changed between identical genSpec runs: %s", path)
		}
	}
	violations, err = allViolationsAsOf(domainDir, "2026-10-05")
	if err != nil {
		t.Fatalf("allViolations after run 2: %v", err)
	}
	for _, v := range violations {
		t.Errorf("violation survived second genSpec pass: [%s] %s: %s", v.Check, v.ID, v.Message)
	}
}

// snapshotGeneratedTree hashes every regular file of the generated set: the
// domain's docs/gen tree, the crystal plus its sibling AGENTS.md/GEMINI.md,
// and the repoRoot framework/ dir genSpec writes tools to.
func snapshotGeneratedTree(t *testing.T, projectRoot, domainDir, crystalPath string) (map[string]string, error) {
	t.Helper()
	snap := map[string]string{}
	roots := []string{
		filepath.Join(domainDir, "docs", "gen"),
		crystalPath,
		filepath.Join(filepath.Dir(crystalPath), "AGENTS.md"),
		filepath.Join(filepath.Dir(crystalPath), "GEMINI.md"),
		filepath.Join(projectRoot, "framework"),
	}
	for _, root := range roots {
		info, err := os.Stat(root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			data, err := os.ReadFile(root)
			if err != nil {
				return nil, err
			}
			sum := sha256.Sum256(data)
			snap[root] = hex.EncodeToString(sum[:])
			continue
		}
		var files []string
		if err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if fi.Mode().IsRegular() {
				files = append(files, path)
			}
			return nil
		}); err != nil {
			return nil, err
		}
		sort.Strings(files)
		for _, path := range files {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			sum := sha256.Sum256(data)
			snap[path] = hex.EncodeToString(sum[:])
		}
	}
	return snap, nil
}

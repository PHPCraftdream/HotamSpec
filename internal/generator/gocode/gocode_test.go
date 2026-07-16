package gocode

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
)

// pratDomainDir locates the real PRAT-hotam "prat" domain, a sibling
// checkout to this repo (D:/ai_dev/prat/{HotamSpec,PRAT-hotam}). This test
// only READS that domain (LoadGraph) — it never writes into PRAT-hotam. It
// is skipped if the sibling checkout is not present (e.g. CI running
// HotamSpec in isolation).
func pratDomainDir(t *testing.T) string {
	t.Helper()
	// internal/generator/gocode -> ../../../.. -> D:/ai_dev/prat
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	candidate := filepath.Join(wd, "..", "..", "..", "..", "PRAT-hotam", "domains", "prat")
	graphPath := filepath.Join(candidate, "graph.json")
	if _, err := os.Stat(graphPath); err != nil {
		t.Skipf("sibling PRAT-hotam checkout not found at %s (%v) — skipping real-domain test", graphPath, err)
	}
	return candidate
}

// TestGenerateModels_RealPratDomain runs the model generator against the
// real PRAT-hotam "prat" domain (9 EntityTypes) and asserts every EntityType
// either renders to valid Go or fails with a named, expected error (unknown
// glossary term / unmapped field kind) — never panics, never silently
// coerces. It is the "показать какие термины не хватило" step from the task:
// any *UnknownTermError/*UnknownFieldKindError encountered is reported via
// t.Log, not treated as a test failure, since the contract (§6) explicitly
// says the glossary is not expected to cover the whole graph on day one.
func TestGenerateModels_RealPratDomain(t *testing.T) {
	domainDir := pratDomainDir(t)

	g, err := loader.LoadGraph(filepath.Join(domainDir, "graph.json"))
	if err != nil {
		t.Fatalf("LoadGraph(prat): %v", err)
	}
	if len(g.EntityTypes) == 0 {
		t.Fatal("expected the prat domain to have at least one EntityType")
	}
	t.Logf("prat domain: %d EntityTypes", len(g.EntityTypes))

	var missingTerms []string
	var unmappedKinds []string
	succeeded := 0

	for _, et := range g.EntityTypes {
		m, err := BuildEntityModel(et)
		if err != nil {
			var kindErr *UnknownFieldKindError
			var termErr *UnknownTermError
			switch {
			case errors.As(err, &kindErr):
				unmappedKinds = append(unmappedKinds, et.Slug+"."+kindErr.Field+": kind="+kindErr.Kind)
			case errors.As(err, &termErr):
				missingTerms = append(missingTerms, termErr.Term+" (from "+termErr.Source+", EntityType "+et.Slug+")")
			default:
				t.Fatalf("EntityType %q: unexpected error building model: %v", et.Slug, err)
			}
			continue
		}
		src, err := RenderEntityType(m)
		if err != nil {
			t.Fatalf("EntityType %q: RenderEntityType: %v", et.Slug, err)
		}
		if src == "" {
			t.Fatalf("EntityType %q: rendered empty source", et.Slug)
		}
		succeeded++
	}

	t.Logf("succeeded: %d/%d EntityTypes", succeeded, len(g.EntityTypes))
	if len(missingTerms) > 0 {
		t.Logf("terms missing from GEN-CODE-CONTRACT.md glossary/abbreviation table: %v", missingTerms)
	}
	if len(unmappedKinds) > 0 {
		t.Logf("field kinds with no §2 Go mapping yet: %v", unmappedKinds)
	}
	if succeeded == 0 {
		t.Fatal("expected at least one EntityType to build successfully on the real prat domain")
	}
}

// TestGenerateModels_RealPratDomain_FrRecord renders the fr-record
// EntityType specifically (the richest node: 6 fields, 3 states) and
// verifies the generated Go actually compiles in a fresh temp module.
func TestGenerateModels_RealPratDomain_FrRecord(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	domainDir := pratDomainDir(t)

	g, err := loader.LoadGraph(filepath.Join(domainDir, "graph.json"))
	if err != nil {
		t.Fatalf("LoadGraph(prat): %v", err)
	}

	found := false
	var src string
	for _, et := range g.EntityTypes {
		if et.Slug != "fr-record" {
			continue
		}
		found = true
		m, err := BuildEntityModel(et)
		if err != nil {
			t.Fatalf("fr-record: BuildEntityModel: %v", err)
		}
		if len(et.Fields) != 6 {
			t.Fatalf("expected fr-record to have 6 fields (per task description), got %d", len(et.Fields))
		}
		if len(et.Lifecycle.States) != 3 {
			t.Fatalf("expected fr-record to have 3 lifecycle states, got %d", len(et.Lifecycle.States))
		}
		rendered, err := RenderEntityType(m)
		if err != nil {
			t.Fatalf("fr-record: RenderEntityType: %v", err)
		}
		src = rendered
	}
	if !found {
		t.Fatal("EntityType 'fr-record' not found in prat domain")
	}

	t.Logf("generated Go for fr-record:\n%s", src)

	dir := t.TempDir()
	full := OwnershipMarker + "\n\npackage gen\n\nimport \"fmt\"\n\n" + src
	if err := os.WriteFile(filepath.Join(dir, "entities.go"), []byte(full), 0o644); err != nil {
		t.Fatalf("write entities.go: %v", err)
	}
	goMod := "module gocode-frrecord-test\n\ngo " + EngineGoVersion + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated fr-record module failed to build: %v\n%s", err, out)
	}
}

// seeAuditAnchorPattern matches every "see requirements_audit.md#<anchor>"
// occurrence a generated .go file may carry (both "// Atom: ... - see
// requirements_audit.md#<anchor>" comments and
// `t.Log("no structural atom - see requirements_audit.md#<anchor>")` calls),
// capturing just the anchor. Anchors are always ASCII kebab-case identifiers
// (contract §1.1's zero-Cyrillic rule + BuildEntityModel/BuildRequirementModel's
// own ToKebabCase-derived slugs), so a plain `[\w-]+` capture is sufficient —
// this is the same anchor shape RenderAuditFile's own "{#<anchor>}" headings
// use.
var seeAuditAnchorPattern = regexp.MustCompile(`see requirements_audit\.md#([\w-]+)`)

// extractAuditHeadingAnchors scans requirements_audit.md's own rendered
// markdown for every "{#<anchor>}" heading id RenderAuditFile emits (##/###
// headings across the EntityType, Requirements, and Pipeline Gates
// sections), returning the set of anchors that actually resolve.
func extractAuditHeadingAnchors(audit string) map[string]bool {
	headingPattern := regexp.MustCompile(`\{#([\w-]+)\}`)
	out := make(map[string]bool)
	for _, m := range headingPattern.FindAllStringSubmatch(audit, -1) {
		out[m[1]] = true
	}
	return out
}

// TestGenerateAllFromGraph_RealPratDomain_AuditAnchorsResolve is the
// regression proof for the requirements_audit.md merge-order bug: previously
// cmd/hotam's genCode merged four independently-generated file maps by
// filename, and GeneratePipelineFromGraph's own requirements_audit.md render
// (built with reqModels=nil, since pipeline stage never had SETTLED
// requirements in scope) always ran last and clobbered the fuller
// GenerateRequirementsFromGraph render — so every "// Atom: <id> - see
// requirements_audit.md#<anchor>" comment tied to a requirement pointed at a
// heading that plainly did not exist in the file actually written to disk.
// GenerateAllFromGraph is the fix (single call site, all three of
// models/reqModels/gates real): this test proves EVERY anchor any generated
// .go file references via "see requirements_audit.md#<anchor>" has a
// matching "{#<anchor>}" heading in the SAME generation run's
// requirements_audit.md — zero dangling anchors, all four generated .go
// files considered (entities.go/lifecycle.go/requirements_test.go/
// pipeline_test.go), not just requirements_test.go.
func TestGenerateAllFromGraph_RealPratDomain_AuditAnchorsResolve(t *testing.T) {
	domainDir := pratDomainDir(t)

	g, err := loader.LoadGraph(filepath.Join(domainDir, "graph.json"))
	if err != nil {
		t.Fatalf("LoadGraph(prat): %v", err)
	}

	files, err := GenerateAllFromGraph(g.EntityTypes, g.Requirements, g.Processes, domainDir)
	if err != nil {
		t.Fatalf("GenerateAllFromGraph: %v", err)
	}

	audit, ok := files["requirements_audit.md"]
	if !ok {
		t.Fatal("GenerateAllFromGraph did not produce requirements_audit.md")
	}
	auditText := string(audit)

	if !strings.Contains(auditText, "\n## Requirements\n") {
		t.Fatal("requirements_audit.md is missing the '## Requirements' section — the merge-order bug is back")
	}
	if !strings.Contains(auditText, "\n## Pipeline Gates\n") {
		t.Fatal("requirements_audit.md is missing the '## Pipeline Gates' section")
	}

	headings := extractAuditHeadingAnchors(auditText)

	var checked int
	var missing []string
	for name, content := range files {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		for _, m := range seeAuditAnchorPattern.FindAllStringSubmatch(string(content), -1) {
			anchor := m[1]
			checked++
			if !headings[anchor] {
				missing = append(missing, name+"#"+anchor)
			}
		}
	}

	if checked == 0 {
		t.Fatal("found zero 'see requirements_audit.md#...' anchors across generated .go files — test fixture assumption broken")
	}
	if len(missing) > 0 {
		t.Fatalf("%d/%d anchors referenced from generated .go files do not resolve to a requirements_audit.md heading: %v", len(missing), checked, missing)
	}
	t.Logf("verified %d/%d anchors resolve", checked, checked)
}

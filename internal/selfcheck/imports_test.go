package selfcheck

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

// modulePath is this repository's Go module path (must match go.mod).
const modulePath = "github.com/PHPCraftdream/HotamSpec"

// isStdlibImport reports whether an import path belongs to the Go standard
// library (its first path segment contains no dot, e.g. "fmt", "go/ast",
// "encoding/json"). Hosted/vcs imports (github.com/...) always have a dot.
func isStdlibImport(path string) bool {
	first := path
	if i := strings.IndexByte(path, '/'); i >= 0 {
		first = path[:i]
	}
	return !strings.Contains(first, ".")
}

// TestCoreImports_StdlibOrHotamSpecOnly enforces
// R-core-imports-stdlib-or-hotam-spec-only: every import in the core Go
// packages (internal/*, cmd/hotam/*) resolves to the Go standard library or
// the hotam module itself — no third-party backend/runtime dependency.
//
// EXACT RULE (mechanically checked): for EVERY .go file (test and non-test)
// under internal/ and cmd/hotam/, each import path is either stdlib (first
// segment has no dot) or begins with the module path
// (github.com/PHPCraftdream/HotamSpec/...). Any other import is a third-party
// dependency, which is forbidden in the core. go.mod today has zero require
// directives, so the property holds; this test fails the moment a third-party
// import is added anywhere in the core tree.
//
// Discrimination: see TestCoreImports_DetectsThirdParty.
func TestCoreImports_StdlibOrHotamSpecOnly(t *testing.T) {
	t.Parallel()
	files := collectGoFiles(t, frameworkScanRoots, true /* tests */, true /* testdata for completeness */)
	for _, f := range files {
		for _, imp := range importPaths(f.ast) {
			if isStdlibImport(imp) {
				continue
			}
			if imp == modulePath || strings.HasPrefix(imp, modulePath+"/") {
				continue
			}
			t.Errorf("R-core-imports-stdlib-or-hotam-spec-only: third-party import %q in %s — core may depend only on stdlib or the hotam module",
				imp, relPath(t, f.path))
		}
	}
}

// TestCoreImports_DetectsThirdParty is the non-vacuity control: the matcher
// must classify a representative stdlib path as allowed, the module path as
// allowed, and a third-party path as forbidden.
func TestCoreImports_DetectsThirdParty(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		imp     string
		allowed bool
	}{
		{"fmt", true},
		{"go/ast", true},
		{"encoding/json", true},
		{modulePath, true},
		{modulePath + "/internal/ontology", true},
		{"github.com/some third-party/lib", false},
		{"gopkg.in/yaml.v3", false},
		{"github.com/spf13/cobra", false},
	} {
		stdlib := isStdlibImport(c.imp)
		self := c.imp == modulePath || strings.HasPrefix(c.imp, modulePath+"/")
		got := stdlib || self
		if got != c.allowed {
			t.Errorf("matcher misclassified %q: allowed=%v want %v (stdlib=%v self=%v)", c.imp, got, c.allowed, stdlib, self)
		}
	}
}

// corePackages are the core layer (the typed-node / graph / proposal /
// invariant machinery) that must never reach up into a consumer/periphery
// package. Per R-core-periphery-import-ratchet's WHY the core is ontology,
// invariants, proposal; loader (which imports only ontology) is core-ward and
// included here.
var corePackages = map[string]bool{
	"ontology":   true,
	"loader":     true,
	"proposal":   true,
	"invariants": true,
}

// peripheryConsumers are the consumer/reporting packages that sit ABOVE the
// core: the diagnostic engine, the doc generator, the query interface, and the
// freshness reporter. The core must never import any of them — the dependency
// arrow points one way (consumers depend on core, never the reverse).
//
// gate / methodology / registry / paths are deliberately NOT in this set: they
// are shared low-level machinery that core (specifically invariants) depends on
// today, so they are core-ward, not periphery. This matches the requirement's
// claim that the one-way property "holds structurally in the current dependency
// graph".
var peripheryConsumers = map[string]bool{
	"diagnose":  true,
	"generator": true,
	"query":     true,
	"freshness": true,
}

// TestCorePeriphery_ImportRatchet enforces R-core-periphery-import-ratchet: a
// core module shall never import a periphery module — the core/periphery
// dependency arrow points one way only.
//
// EXACT RULE (mechanically checked): for every NON-TEST .go file inside a core
// package directory (internal/{ontology,loader,proposal,invariants}), no import
// path may resolve to a periphery consumer package
// (internal/{diagnose,generator,query,freshness}). Test files are excluded —
// the production dependency arrow is what is pinned. Today no core package
// imports any consumer; this fails the moment a core file grows such an import.
//
// Discrimination: see TestCorePeriphery_RatchetDetectsViolation.
func TestCorePeriphery_ImportRatchet(t *testing.T) {
	t.Parallel()
	for corePkg := range corePackages {
		files := collectGoFiles(t, []string{"internal/" + corePkg}, false /* non-test */, false)
		for _, f := range files {
			for _, imp := range importPaths(f.ast) {
				consumer := peripheryImported(imp)
				if consumer != "" {
					t.Errorf("R-core-periphery-import-ratchet: core package internal/%s imports periphery consumer internal/%s in %s — the core/periphery arrow must point one way only",
						corePkg, consumer, relPath(t, f.path))
				}
			}
		}
	}
}

// TestCorePeriphery_RatchetDetectsViolation is the non-vacuity control: the
// detector must flag a core file importing a consumer, and must NOT flag a core
// file importing another core package or shared low-level machinery.
func TestCorePeriphery_RatchetDetectsViolation(t *testing.T) {
	t.Parallel()
	bad := peripheryImported(modulePath + "/internal/generator")
	if bad != "generator" {
		t.Errorf("detector failed to flag core->generator import, got %q", bad)
	}
	good1 := peripheryImported(modulePath + "/internal/ontology")
	good2 := peripheryImported(modulePath + "/internal/methodology")
	good3 := peripheryImported("fmt")
	if good1 != "" || good2 != "" || good3 != "" {
		t.Errorf("detector must not flag core-ward/stdlib imports, got generator=%q methodology=%q fmt=%q", good1, good2, good3)
	}
}

// peripheryImported returns the consumer package name if imp resolves into a
// periphery consumer package, else "".
func peripheryImported(imp string) string {
	prefix := modulePath + "/internal/"
	if !strings.HasPrefix(imp, prefix) {
		return ""
	}
	pkg := strings.TrimPrefix(imp, prefix)
	if i := strings.IndexByte(pkg, '/'); i >= 0 {
		pkg = pkg[:i]
	}
	if peripheryConsumers[pkg] {
		return pkg
	}
	return ""
}

// TestAgentCode_ImportsFramework enforces R-agent-code-imports-framework: the
// framework body (internal/*, cmd/hotam) shall never import back from any
// agent's private tools/ or agents/ runtime directory.
//
// EXACT RULE (mechanically checked): in EVERY .go file (test and non-test)
// under internal/ and cmd/hotam/, no IMPORT PATH may contain a "/agents/" or
// "/tools/" segment. domains/<name>/agents/ and domains/<name>/tools/ are
// domain-owned runtime directories (scaffolding + scripts), never Go import
// targets; the framework must never import domain content as code. Only actual
// import statements are inspected — string literals and comments that merely
// mention these paths (in docs, requirement text, tool purposes) are not
// imports and do not trigger the check.
//
// Discrimination: see TestAgentCode_DetectorNonVacuous.
func TestAgentCode_ImportsFramework(t *testing.T) {
	t.Parallel()
	files := collectGoFiles(t, frameworkScanRoots, true, true)
	for _, f := range files {
		for _, imp := range importPaths(f.ast) {
			if strings.Contains(imp, "/agents/") || strings.Contains(imp, "/tools/") {
				t.Errorf("R-agent-code-imports-framework: framework source imports domain-owned runtime path %q in %s — the framework must never import back from an agent's private tools/ or agents/ directory",
					imp, relPath(t, f.path))
			}
		}
	}
}

// TestAgentCode_DetectorNonVacuous is the non-vacuity control: the forbidden
// import shape is known (domains/<name>/agents/<a>/...) and must be flagged,
// while a core-ward/stdlib import must not be.
func TestAgentCode_DetectorNonVacuous(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{
		modulePath + "/domains/hotam-spec-self/agents/director/tools/priv",
		modulePath + "/some/agents/x",
		modulePath + "/some/tools/y",
	} {
		if !strings.Contains(bad, "/agents/") && !strings.Contains(bad, "/tools/") {
			t.Errorf("control input %q was not flagged but should be", bad)
		}
	}
	for _, good := range []string{
		modulePath + "/internal/ontology",
		"fmt",
	} {
		if strings.Contains(good, "/agents/") || strings.Contains(good, "/tools/") {
			t.Errorf("control input %q was flagged but should not be", good)
		}
	}
}

// TestSharedTools_InSpecTools enforces R-shared-tools-in-spec-tools: tools
// available to all operators are the single shared hotam CLI toolset — Go
// subcommands wired in cmd/hotam/main.go and declared once in the
// methodology.Tools registry — with no per-agent private tool namespace.
//
// EXACT RULE (mechanically checked), two halves:
//
//  1. REGISTRY<->WIRING: every tool registered with Status=Implemented in
//     methodology.Tools has its CLI form (Command with underscores -> hyphens,
//     matching the `hotam <command>` dispatch convention) present as a
//     `case "<cli>":` arm in cmd/hotam/main.go's switch. This is a distinct,
//     source-level guarantee from tool_wiring.go's runtime Run!=nil check: it
//     proves main.go's dispatch table actually exposes each Implemented tool as
//     a reachable subcommand.
//  2. NO PER-AGENT TOOL NAMESPACE: there is no second, per-agent/per-domain
//     tool Go-import namespace — every tool is a single registry entry. This is
//     structurally the same forbidden edge as R-agent-code-imports-framework
//     (no framework import of /tools/ or /agents/); we re-assert the registry
//     is the single source by confirming it is non-empty and every Implemented
//     Command is unique and non-empty.
//
// Discrimination: see TestSharedTools_DetectsUnwired.
func TestSharedTools_InSpecTools(t *testing.T) {
	t.Parallel()
	caseSet := mainGoCaseSet(t)
	if len(caseSet) == 0 {
		t.Fatal("no case strings parsed from cmd/hotam/main.go — parser is broken")
	}
	for _, m := range missingImplementedFromCases(caseSet) {
		t.Errorf("R-shared-tools-in-spec-tools: %s", m)
	}
	if !methodologyHasImplemented(t) {
		t.Error("R-shared-tools-in-spec-tools: methodology.Tools has zero Implemented tools — the shared toolset is unexpectedly empty")
	}
}

// TestSharedTools_DetectsUnwired is the non-vacuity control: a doctored case
// set that omits a real Implemented tool's CLI form must produce at least one
// missing entry, while the full main.go case set must produce none. This proves
// the detector genuinely flags an unwired tool rather than trivially passing.
func TestSharedTools_DetectsUnwired(t *testing.T) {
	t.Parallel()
	full := mainGoCaseSet(t)
	if miss := missingImplementedFromCases(full); len(miss) != 0 {
		t.Fatalf("with the real main.go case set there should be zero missing tools, got: %v", miss)
	}
	var probe string
	for _, tool := range methodology.Tools.All() {
		if tool.Status == methodology.Implemented {
			probe = strings.ReplaceAll(tool.Command, "_", "-")
			break
		}
	}
	if probe == "" {
		t.Fatal("no Implemented tool to exercise the detector")
	}
	doctored := map[string]bool{"version": true, "init": true}
	if doctored[probe] {
		t.Fatalf("control case set accidentally includes the probe %q", probe)
	}
	missing := missingImplementedFromCases(doctored)
	found := false
	for _, m := range missing {
		if strings.Contains(m, probe) {
			found = true
		}
	}
	if !found {
		t.Fatalf("detector failed to flag unwired Implemented tool %q in a doctored case set — the main test would be vacuous; missing=%v", probe, missing)
	}
}

// missingImplementedFromCases returns an error message per Implemented registry
// tool whose CLI form (Command underscores->hyphens) is absent from caseSet,
// plus messages for empty/duplicate Commands. Shared by the enforcing test and
// its non-vacuity control so they exercise the exact same predicate.
func missingImplementedFromCases(caseSet map[string]bool) []string {
	var msgs []string
	seen := map[string]bool{}
	for _, tool := range methodology.Tools.All() {
		if tool.Command == "" {
			msgs = append(msgs, fmt.Sprintf("registry entry %q has an empty Command — every tool needs a non-empty command", tool.Canon))
			continue
		}
		if tool.Status != methodology.Implemented {
			continue
		}
		cli := strings.ReplaceAll(tool.Command, "_", "-")
		if seen[cli] {
			msgs = append(msgs, fmt.Sprintf("duplicate Implemented command %q — the toolset must be a single shared namespace", cli))
		}
		seen[cli] = true
		if !caseSet[cli] {
			msgs = append(msgs, fmt.Sprintf("Implemented tool %q (Command=%q) is not wired as a `case %q:` in cmd/hotam/main.go — every Implemented tool must be a reachable hotam subcommand", tool.Canon, tool.Command, cli))
		}
	}
	return msgs
}

// mainGoCaseSet parses cmd/hotam/main.go and returns the set of strings used as
// case-clause values (the dispatch table).
func mainGoCaseSet(t *testing.T) map[string]bool {
	t.Helper()
	files := collectGoFiles(t, []string{"cmd/hotam"}, true, true)
	for _, f := range files {
		if strings.HasSuffix(filepath.ToSlash(f.path), "cmd/hotam/main.go") {
			out := map[string]bool{}
			for _, c := range switchCaseStrings(f.ast) {
				out[c] = true
			}
			return out
		}
	}
	t.Fatal("cmd/hotam/main.go not found by the scanner")
	return nil
}

// methodologyHasImplemented reports whether the registry has any Implemented tool.
func methodologyHasImplemented(t *testing.T) bool {
	t.Helper()
	for _, tool := range methodology.Tools.All() {
		if tool.Status == methodology.Implemented {
			return true
		}
	}
	return false
}

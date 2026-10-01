package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenSpec_LiveStateAndAgentContextRelativizeAbsPaths proves the
// no-machine-paths contract END TO END: after genSpec on a domain whose
// violation set includes check_spec_md_current's discipline:full-missing-SPEC.md
// violation (its Message embeds the absolute g.DomainDir twice, its ID IS the
// absolute DomainDir), the written docs/gen/live-state.md and
// docs/gen/AGENT-CONTEXT.md must carry root-relative, forward-slashed paths
// only — never the absolute form — regardless of the process working
// directory (this test runs with CWD = cmd/hotam, which has no domains/
// child, so no working-directory guessing could ever produce the relative
// form; only the explicit repoRoot threading into
// generator.BuildLiveStateWithViolationsRoot / generator.BuildAgentContextRoot
// can). check_spec_md_current is an ORDINARY check (not post-process), so it
// IS part of genSpec's activeViolations that feed these two projections.
func TestGenSpec_LiveStateAndAgentContextRelativizeAbsPaths(t *testing.T) {
	projectRoot, domainDir := copySelfDomainUnderRoot(t)
	// Flip the copied domain to discipline:full so check_spec_md_current's
	// missing-SPEC.md branch fires (P1 STRUCTURE — outranking the self
	// domain's own P3 conflict-stalled signal, so it is the TOP action both
	// projections render) with the absolute DomainDir embedded in Message+ID.
	manifestPath := filepath.Join(domainDir, "manifest.json")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if !strings.Contains(string(manifest), "{") {
		t.Fatalf("precondition: manifest.json must be a JSON object")
	}
	withDiscipline := strings.Replace(string(manifest), "{", "{\n  \"discipline\": \"full\",", 1)
	if err := os.WriteFile(manifestPath, []byte(withDiscipline), 0o644); err != nil {
		t.Fatalf("write manifest with discipline full: %v", err)
	}

	if _, _, err := genSpec(domainDir, "", "2026-01-10", "", false); err != nil {
		t.Fatalf("genSpec: %v", err)
	}

	absNative := projectRoot
	absSlashed := filepath.ToSlash(projectRoot)
	for _, name := range []string{"live-state.md", "AGENT-CONTEXT.md"} {
		data, err := os.ReadFile(filepath.Join(domainDir, "docs", "gen", name))
		if err != nil {
			t.Fatalf("read docs/gen/%s: %v", name, err)
		}
		out := string(data)
		if strings.Contains(out, absNative) || strings.Contains(out, absSlashed) {
			t.Errorf("docs/gen/%s leaks the absolute project path:\n%s", name, out)
		}
		if !strings.Contains(out, "domains/hotam-spec-self") {
			t.Errorf("docs/gen/%s lost the root-relative form of the violation path:\n%s", name, out)
		}
	}
}

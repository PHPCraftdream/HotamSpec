package main

import (
	"strings"
	"testing"
)

func TestCmdBrief_RequirementSmoke(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	err := cmdBrief([]string{"--domain", domainDir, "--today", "2026-07-13", "R-anchor-everything", "--json"})
	if err != nil {
		t.Fatalf("cmdBrief --json: %v", err)
	}
}

func TestCmdBrief_RequirementHumanReadable(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	err := cmdBrief([]string{"--domain", domainDir, "--today", "2026-07-13", "R-anchor-everything"})
	if err != nil {
		t.Fatalf("cmdBrief: %v", err)
	}
}

func TestCmdBrief_ConflictSmoke(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	// C-186c4347 is a real conflict in the hotam-spec-self domain fixture
	// (R-ai-presents-not-decides is a member — see TestCmdReqContext above).
	err := cmdBrief([]string{"--domain", domainDir, "--today", "2026-07-13", "C-186c4347", "--json"})
	if err != nil {
		t.Fatalf("cmdBrief conflict --json: %v", err)
	}
}

func TestCmdBrief_AssumptionSmoke(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	// Find a real assumption id from the fixture. The graph has assumptions;
	// A-ai-presents-not-decides-style prefix is used. Use the first one we
	// can find by trying a known anchor from the domain.
	// A-agent-code-imports-framework-directionally is a real assumption in
	// hotam-spec-self.
	err := cmdBrief([]string{"--domain", domainDir, "--today", "2026-07-13", "A-agent-code-imports-framework-directionally", "--json"})
	if err != nil {
		t.Fatalf("cmdBrief assumption --json: %v", err)
	}
}

func TestCmdBrief_NotFound(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	err := cmdBrief([]string{"--domain", domainDir, "--today", "2026-07-13", "R-nonexistent-xyz"})
	if err == nil {
		t.Fatal("expected error for non-existent anchor")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestCmdBrief_NoArgs(t *testing.T) {
	t.Parallel()
	err := cmdBrief(nil)
	if err == nil {
		t.Fatal("expected usage error for no args")
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

// TestCmdApplyProposal_OmittedDomainFallsThroughActiveDomainChain proves
// `hotam apply-proposal` no longer hard-requires --domain (review-6 R6-f
// issue 1): an omitted --domain must fall through resolveDomain's active-
// domain chain exactly like every other command (gen-spec, status, req, …)
// instead of failing with a premature "--domain is required" check that ran
// BEFORE resolveDomain got a chance to resolve tier 2/3/4.
//
// This test drives tier 4 (the silent legacy default): HOTAM_SPEC_PROJECT_ROOT
// pins the project root to a temp dir scaffolded via copySelfDomainUnderRoot
// (which places the fixture at <root>/domains/hotam-spec-self, i.e. exactly
// where tier 4's defaultDomainName resolves), with no HOTAM_DOMAIN env and no
// active_domain marker, so resolveDomain("") must land on that same fixture.
func TestCmdApplyProposal_OmittedDomainFallsThroughActiveDomainChain(t *testing.T) {
	// Not t.Parallel(): t.Setenv (HOTAM_SPEC_PROJECT_ROOT/HOTAM_DOMAIN) must
	// not race with other tests mutating the same process-global env vars.
	projectRoot, domainDir := copySelfDomainUnderRoot(t)
	t.Setenv(paths.EnvProjectRoot, projectRoot)
	t.Setenv(paths.EnvActiveDomain, "")

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-apply-proposal-domain-chain",
		"claim": "apply-proposal without --domain falls through the active-domain chain",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "R6-f issue 1 regression coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal fixture: %v", err)
	}

	err := cmdApplyProposal([]string{
		"--today", "2026-07-14",
		proposalPath,
	})
	if err != nil {
		t.Fatalf("cmdApplyProposal without --domain should fall through to the tier-4 legacy default, got error: %v", err)
	}

	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	if !strings.Contains(string(graphData), "R-apply-proposal-domain-chain") {
		t.Error("graph.json at the tier-4-resolved domain does not contain the applied requirement — apply-proposal did not target the active-domain-chain-resolved directory")
	}
}

// TestCmdLand_OmittedDomainFallsThroughActiveDomainChain is the `hotam land`
// counterpart to TestCmdApplyProposal_OmittedDomainFallsThroughActiveDomainChain
// (review-6 R6-f issue 1): an omitted --domain must fall through the same
// 4-tier active-domain chain instead of hard-erroring on a premature
// "--domain is required" check.
func TestCmdLand_OmittedDomainFallsThroughActiveDomainChain(t *testing.T) {
	// Not t.Parallel(): t.Setenv must not race with other env-mutating tests.
	projectRoot, domainDir := copySelfDomainUnderRoot(t)
	t.Setenv(paths.EnvProjectRoot, projectRoot)
	t.Setenv(paths.EnvActiveDomain, "")

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-domain-chain",
		"claim": "land without --domain falls through the active-domain chain",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "R6-f issue 1 regression coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal fixture: %v", err)
	}

	err := cmdLand([]string{
		"--today", "2026-07-14",
		proposalPath,
	})
	if err != nil {
		t.Fatalf("cmdLand without --domain should fall through to the tier-4 legacy default, got error: %v", err)
	}

	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	if !strings.Contains(string(graphData), "R-land-domain-chain") {
		t.Error("graph.json at the tier-4-resolved domain does not contain the landed requirement — land did not target the active-domain-chain-resolved directory")
	}

	reqPath := filepath.Join(domainDir, "docs", "gen", "REQUIREMENTS.md")
	after, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("read post-land REQUIREMENTS.md: %v", err)
	}
	if !strings.Contains(string(after), "R-land-domain-chain") {
		t.Error("docs/gen/REQUIREMENTS.md at the tier-4-resolved domain was not regenerated with the new requirement")
	}
}

// TestCmdLand_OmittedDomainUsesMarkerTier3 exercises tier 3 (the
// .hotam-spec-project marker's active_domain) specifically for `hotam land`,
// so the fix is proven against more than just the silent tier-4 default: a
// marker recording a domain NAME other than the legacy default must steer
// land to that domain even though --domain was never passed.
func TestCmdLand_OmittedDomainUsesMarkerTier3(t *testing.T) {
	// Not t.Parallel(): t.Setenv must not race with other env-mutating tests.
	projectRoot := t.TempDir()
	domainDir := filepath.Join(projectRoot, "domains", "marked-domain")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir domain: %v", err)
	}
	copyFile(t, selfDomainGraph, filepath.Join(domainDir, "graph.json"))
	copyFile(t, selfDomainManifest, filepath.Join(domainDir, "manifest.json"))

	markerPath := filepath.Join(projectRoot, paths.MarkerFilename)
	if err := paths.WriteActiveDomain(markerPath, "marked-domain"); err != nil {
		t.Fatalf("seed marker: %v", err)
	}

	t.Setenv(paths.EnvProjectRoot, projectRoot)
	t.Setenv(paths.EnvActiveDomain, "")

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-marker-tier3",
		"claim": "land without --domain honors the marker active_domain preference",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "R6-f issue 1 regression coverage (tier 3)"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal fixture: %v", err)
	}

	if err := cmdLand([]string{
		"--today", "2026-07-14",
		proposalPath,
	}); err != nil {
		t.Fatalf("cmdLand without --domain should resolve via the tier-3 marker, got error: %v", err)
	}

	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read marked-domain graph.json: %v", err)
	}
	if !strings.Contains(string(graphData), "R-land-marker-tier3") {
		t.Error("marker-resolved domain's graph.json does not contain the landed requirement")
	}
}

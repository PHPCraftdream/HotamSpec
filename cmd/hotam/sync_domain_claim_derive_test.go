package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// claimDeriveSyncDomainRequirementsGoTemplate mirrors
// syncDomainRequirementsGoTemplate (sync_domain_test.go) exactly, except its
// Claim is a deliberately STALE placeholder — proving `hotam sync-domain`
// itself (not merely internal/selfspec.DeriveClaimsFromScenarios in
// isolation) overrides a hand-authored Claim with the real derived text
// before the registry ever reaches SyncGraph (task #369's own wiring in
// cmd/hotam/sync_domain.go). verified_by names a real scenario-narrated
// test this fixture's own module carries (see
// claimDeriveSyncDomainTestSrc below).
const claimDeriveSyncDomainRequirementsGoTemplate = `package spec

import hotamontology "{{MODULE}}/hotamontology"

var Requirements = hotamontology.New[hotamontology.Requirement]()

func init() {
	Requirements.MustRegister("R-claim-derive-e2e", hotamontology.Requirement{
		ID:             "R-claim-derive-e2e",
		Claim:          "STALE placeholder claim — must be overridden by sync-domain",
		Owner:          "fixture-owner",
		Status:         "SETTLED",
		Relations:      []hotamontology.Relation{},
		Assumptions:    []string{},
		Enforcement:    "ENFORCED",
		EnforcedBy:     []string{},
		Enforceability: "ENFORCEABLE",
		MTag:           "",
		Summary:        "",
		CreatedAt:      "2026-01-01",
		SettledAt:      "2026-01-01",
		SourceRefs:     []string{},
		DeclOrder:      1,
		ImplementedBy:  []string{"spec/model/impl.go:RequireComplete"},
		VerifiedBy:     []string{"spec/model/impl_test.go:TestRequireComplete_Scenario"},
	})
}
`

const claimDeriveSyncDomainImplSrc = `package model

func RequireComplete(fields int) error {
	if fields < 1 {
		return errNotComplete
	}
	return nil
}

var errNotComplete = errStub{}

type errStub struct{}

func (errStub) Error() string { return "not complete" }
`

func claimDeriveSyncDomainTestSrc(modulePath string) string {
	return `package model

import (
	"testing"

	"` + modulePath + `/hotamspec"
)

func TestRequireComplete_Scenario(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-claim-derive-e2e", "RequireComplete rejects a zero fields count end to end")
	err := RequireComplete(0)
	s.Then("an error is returned", err != nil)
}
`
}

// newClaimDeriveSyncDomainFixture builds a full, real, compilable consumer-
// domain fixture (mirroring newSyncDomainFixture exactly) with THREE
// additions beyond that base shape: (1) manifest.json declares
// discipline:"full" (the domain-level gate DeriveClaimsFromScenarios/
// check_claim_matches_scenario both require), (2) a REAL vendored hotamspec
// recorder at spec/hotamspec/hotamspec.go (via vendorRecorder — the same
// writer `hotam vendor-recorder` itself uses), and (3) a real model/impl.go +
// model/impl_test.go pair under spec/ whose test genuinely narrates a
// hotamspec.Scenario for R-claim-derive-e2e, matching the stale
// registry literal's own verified_by entry.
func newClaimDeriveSyncDomainFixture(t *testing.T) *syncDomainFixture {
	t.Helper()
	root, domainDir := initDomainUnderRoot(t, "fixture-claim-derive", "2026-07-26")

	// Flip manifest.json to discipline:"full" — initDomain's own default
	// manifest has no discipline key at all (soft discipline).
	manifestPath := filepath.Join(domainDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"self_hosting": false, "gen_profile": "consumer", "parent": null, "discipline": "full"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json (discipline:full): %v", err)
	}

	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{
			{ID: "fixture-owner", Name: "Fixture Owner", DeclOrder: 1},
		},
	}
	graphPath := graphPathForDomain(domainDir)
	if err := loader.WriteGraph(graphPath, g); err != nil {
		t.Fatalf("seed graph with stakeholder: %v", err)
	}

	specDir := filepath.Join(domainDir, "spec")
	const modulePath = "hotamspec-fixture-claim-derive"
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write spec/go.mod: %v", err)
	}

	if _, err := vendorOntology(domainDir); err != nil {
		t.Fatalf("vendorOntology: %v", err)
	}
	if _, err := vendorRecorder(domainDir); err != nil {
		t.Fatalf("vendorRecorder: %v", err)
	}

	modelDir := filepath.Join(specDir, "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("mkdir spec/model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "impl.go"), []byte(claimDeriveSyncDomainImplSrc), 0o644); err != nil {
		t.Fatalf("write spec/model/impl.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "impl_test.go"), []byte(claimDeriveSyncDomainTestSrc(modulePath)), 0o644); err != nil {
		t.Fatalf("write spec/model/impl_test.go: %v", err)
	}

	reqSrc := strings.ReplaceAll(claimDeriveSyncDomainRequirementsGoTemplate, "{{MODULE}}", modulePath)
	if err := os.WriteFile(filepath.Join(specDir, "requirements.go"), []byte(reqSrc), 0o644); err != nil {
		t.Fatalf("write spec/requirements.go: %v", err)
	}

	if _, err := scaffoldRegistrydump(domainDir); err != nil {
		t.Fatalf("scaffoldRegistrydump: %v", err)
	}

	return &syncDomainFixture{root: root, domainDir: domainDir, graphPath: graphPath, specDir: specDir}
}

// TestCmdSyncDomain_ClaimDerivedFromScenarioEndToEnd is the full end-to-end
// proof (task #369's own verification step) that `hotam sync-domain`, not
// merely internal/selfspec.DeriveClaimsFromScenarios in isolation, derives
// Claim from the verified_by test's real recorded scenario description and
// writes THAT (not the registry literal's own stale placeholder) into
// graph.json — exercising the exact integration point this task chose
// (cmd/hotam/sync_domain.go's DeriveClaimsFromScenarios wiring, right before
// SyncGraph).
func TestCmdSyncDomain_ClaimDerivedFromScenarioEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("sync-domain claim derivation spawns real `go run`/`go test` subprocesses; skipped in -short")
	}
	fx := newClaimDeriveSyncDomainFixture(t)

	dryOut, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, dryOut)
	}
	if strings.Contains(dryOut, "STALE placeholder claim") {
		t.Errorf("dry-run preview still shows the stale placeholder Claim (derivation should already have overridden it before diffing):\n%s", dryOut)
	}
	hash := extractDiffHash(t, dryOut)

	confirmOut, _, err := runSyncDomain(t, []string{
		"--domain", fx.domainDir, "--today", "2026-07-26",
		"--confirm-hash", hash,
	})
	if err != nil {
		t.Fatalf("confirm-hash run failed: %v\n%s", err, confirmOut)
	}
	if !strings.Contains(confirmOut, "sync-domain landed") {
		t.Errorf("stdout missing the landed confirmation:\n%s", confirmOut)
	}

	g, err := loader.LoadGraph(fx.graphPath)
	if err != nil {
		t.Fatalf("reload written graph: %v", err)
	}
	var found bool
	wantClaim := "RequireComplete rejects a zero fields count end to end"
	for _, r := range g.Requirements {
		if r.ID != "R-claim-derive-e2e" {
			continue
		}
		found = true
		if r.Claim != wantClaim {
			t.Errorf("requirement Claim = %q, want the DERIVED scenario title %q (not the registry's stale placeholder)", r.Claim, wantClaim)
		}
	}
	if !found {
		t.Fatalf("R-claim-derive-e2e was not created by sync-domain")
	}

	violations, err := allViolations(fx.domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("expected 0 violations after a clean sync-domain round-trip, got %d: %+v", len(violations), violations)
	}
}

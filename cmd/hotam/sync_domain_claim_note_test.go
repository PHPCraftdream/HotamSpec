package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
)

// TestCmdSyncDomain_ClaimNotePrintedForIgnoredRegistryClaim covers the
// sync-domain NOTE (task: "registry Claim ignored"): when the domain is
// discipline:"full" and a requirement's registry Claim is NON-EMPTY and
// DIFFERS from the derived scenario-title claim, both dry-run and confirm
// mode print one NOTE line (human output) and the --json envelope carries
// it in "notes" — while the diff-hash itself is unchanged (output-only).
func TestCmdSyncDomain_ClaimNotePrintedForIgnoredRegistryClaim(t *testing.T) {
	if testing.Short() {
		t.Skip("sync-domain claim derivation spawns real `go run`/`go test` subprocesses; skipped in -short")
	}
	fx := newClaimDeriveSyncDomainFixture(t)

	const wantNote = "NOTE R-claim-derive-e2e: registry Claim ignored — under discipline:\"full\" the claim is derived from the scenario title; leave Claim empty in spec/requirements.go"

	dryOut, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, dryOut)
	}
	if !strings.Contains(dryOut, wantNote) {
		t.Errorf("dry-run stdout missing NOTE for the differing registry claim:\n%s", dryOut)
	}
	hash := extractDiffHash(t, dryOut)

	// JSON dry-run: the note must land in the "notes" field, not free text.
	jsonOut, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir, "--json"})
	if err != nil {
		t.Fatalf("dry-run --json failed: %v\n%s", err, jsonOut)
	}
	var res syncDomainResult
	if err := json.Unmarshal([]byte(jsonOut), &res); err != nil {
		t.Fatalf("decode --json output: %v\n%s", err, jsonOut)
	}
	found := false
	for _, n := range res.Notes {
		if n == wantNote {
			found = true
		}
	}
	if !found {
		t.Errorf("--json result notes = %v, want the claim NOTE %q", res.Notes, wantNote)
	}

	confirmOut, _, err := runSyncDomain(t, []string{
		"--domain", fx.domainDir, "--today", "2026-07-26",
		"--confirm-hash", hash,
	})
	if err != nil {
		t.Fatalf("confirm-hash run failed: %v\n%s", err, confirmOut)
	}
	if !strings.Contains(confirmOut, wantNote) {
		t.Errorf("confirm-mode stdout missing NOTE for the differing registry claim:\n%s", confirmOut)
	}
}

// TestCmdSyncDomain_ClaimNoteNotPrintedForEmptyClaim covers the
// empty-claim half of the note gate: a discipline:"full" domain whose
// registry Claim is EMPTY for an in-scope requirement (narrated verified_by
// scenario) must produce NO note — nothing to ignore — while the claim is
// still derived and written to the graph.
func TestCmdSyncDomain_ClaimNoteNotPrintedForEmptyClaim(t *testing.T) {
	if testing.Short() {
		t.Skip("sync-domain claim derivation spawns real `go run`/`go test` subprocesses; skipped in -short")
	}
	fx := newClaimDeriveSyncDomainFixture(t)

	emptyClaimSrc := strings.ReplaceAll(
		strings.ReplaceAll(claimDeriveSyncDomainRequirementsGoTemplate,
			"STALE placeholder claim — must be overridden by sync-domain", ""),
		"{{MODULE}}", "hotamspec-fixture-claim-derive")
	if err := os.WriteFile(filepath.Join(fx.specDir, "requirements.go"), []byte(emptyClaimSrc), 0o644); err != nil {
		t.Fatalf("rewrite spec/requirements.go with empty Claim: %v", err)
	}

	dryOut, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, dryOut)
	}
	if strings.Contains(dryOut, "registry Claim ignored") {
		t.Errorf("dry-run printed a claim NOTE for an EMPTY registry claim:\n%s", dryOut)
	}
	hash := extractDiffHash(t, dryOut)

	jsonOut, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir, "--json"})
	if err != nil {
		t.Fatalf("dry-run --json failed: %v\n%s", err, jsonOut)
	}
	var res syncDomainResult
	if err := json.Unmarshal([]byte(jsonOut), &res); err != nil {
		t.Fatalf("decode --json output: %v\n%s", err, jsonOut)
	}
	if len(res.Notes) != 0 {
		t.Errorf("--json result notes = %v, want none for an empty registry claim", res.Notes)
	}

	confirmOut, _, err := runSyncDomain(t, []string{
		"--domain", fx.domainDir, "--today", "2026-07-26",
		"--confirm-hash", hash,
	})
	if err != nil {
		t.Fatalf("confirm-hash run failed: %v\n%s", err, confirmOut)
	}
	if strings.Contains(confirmOut, "registry Claim ignored") {
		t.Errorf("confirm-mode printed a claim NOTE for an EMPTY registry claim:\n%s", confirmOut)
	}

	g, err := loader.LoadGraph(fx.graphPath)
	if err != nil {
		t.Fatalf("reload written graph: %v", err)
	}
	wantClaim := "RequireComplete rejects a zero fields count end to end"
	for _, r := range g.Requirements {
		if r.ID == "R-claim-derive-e2e" && r.Claim != wantClaim {
			t.Errorf("requirement Claim = %q, want the derived scenario title %q (empty registry claim must still derive)", r.Claim, wantClaim)
		}
	}
}

// TestCmdSyncDomain_ClaimNoteNotPrintedForSoftDiscipline verifies the
// domain-level gate: a domain WITHOUT discipline:"full" (the base
// newSyncDomainFixture, whose requirement is INHERENTLY_PROSE anyway) never
// produces a claim NOTE — derivation is an honest no-op there.
func TestCmdSyncDomain_ClaimNoteNotPrintedForSoftDiscipline(t *testing.T) {
	if testing.Short() {
		t.Skip("sync-domain claim derivation spawns real `go run`/`go test` subprocesses; skipped in -short")
	}
	fx := newSyncDomainFixture(t)

	dryOut, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, dryOut)
	}
	if strings.Contains(dryOut, "registry Claim ignored") {
		t.Errorf("dry-run printed a claim NOTE for a non-discipline:full domain:\n%s", dryOut)
	}
}

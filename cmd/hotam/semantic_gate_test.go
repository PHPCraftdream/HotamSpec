package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// setupGateTestDomain scaffolds a clean, invariant-valid minimal domain (via
// initDomain, then seedOwnerStakeholder — task #364 removed initDomain's own
// auto-seeded Stakeholder "owner" + Requirement, so this fixture now adds the
// Stakeholder itself) placed under a domains/<name> parent so
// resolveClaudeMDPath does not auto-write any crystal. writeReqProposalJSON's
// proposals use owner: "owner" by convention, so this Stakeholder must exist
// before the first test requirement can be landed without a dangling-owner
// violation. No unresolved formal Conflict carrier exists yet, so the first
// seed is nonblocking.
func setupGateTestDomain(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	domainDir := filepath.Join(root, "domains", "gate-test")
	if _, err := initDomain(domainDir, "gate-test", "2026-07-14"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}
	seedOwnerStakeholder(t, domainDir, "2026-07-14")
	return domainDir
}

// writeReqProposalJSON writes a minimal valid Requirement proposal JSON to a
// temp file and returns its path. The claim and id are interpolated directly
// (callers must keep them JSON-safe — no quotes or backslashes).
func writeReqProposalJSON(t *testing.T, id, claim string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), id+".json")
	jsonStr := `{
		"kind": "Requirement",
		"id": "` + id + `",
		"claim": "` + claim + `",
		"owner": "owner",
		"status": "SETTLED",
		"why": "semantic-conflict gate test"
	}`
	if err := os.WriteFile(path, []byte(jsonStr), 0o644); err != nil {
		t.Fatalf("write proposal %s: %v", path, err)
	}
	return path
}

// addTestConflict creates a valid Conflict node in the domain's graph via the
// proposal pipeline (resolver stakeholder + axis + conflict), returning the
// Conflict ID. members must already exist as requirements in the graph.
func addTestConflict(t *testing.T, graphPath, today string, members []string) string {
	t.Helper()
	if err := proposal.Apply(graphPath, today, proposal.ProposedStakeholder{
		ID: "resolver-gate", Name: "Gate Resolver", Domain: "gate-test",
	}); err != nil {
		t.Fatalf("add resolver: %v", err)
	}
	if err := proposal.Apply(graphPath, today, proposal.ProposedAxis{
		Slug: "security", Description: "security tension axis",
	}); err != nil {
		t.Fatalf("add axis: %v", err)
	}
	pc := proposal.ProposedConflict{
		Axis:     "security",
		Context:  "encrypt export tension",
		Members:  members,
		Resolver: "resolver-gate",
	}
	if err := proposal.Apply(graphPath, today, pc); err != nil {
		t.Fatalf("add conflict: %v", err)
	}
	return ontology.ConflictIdentity("security", "encrypt export tension")
}

func seedFormalGateConflict(t *testing.T, domainDir, idA, claimA, idB, claimB string) string {
	t.Helper()
	for _, item := range []struct {
		id    string
		claim string
	}{{idA, claimA}, {idB, claimB}} {
		path := writeReqProposalJSON(t, item.id, item.claim)
		if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", path}); err != nil {
			t.Fatalf("land %s: %v", item.id, err)
		}
	}
	return addTestConflict(t, graphPathForDomain(domainDir), "2026-07-14", []string{idA, idB})
}

func TestSemanticGate_OnlyAnyAcrossUnrelatedDutiesRemainAdvisory(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	first := writeReqProposalJSON(t, "R-only-admins", "only admins must archive invoices")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", first}); err != nil {
		t.Fatalf("land first requirement: %v", err)
	}
	second := writeReqProposalJSON(t, "R-any-visitor", "any visitors must create passports")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", second}); err != nil {
		t.Fatalf("only/any marker text across unrelated duties must remain advisory: %v", err)
	}
	g, err := loader.LoadGraph(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	if _, ok := ontology.RequirementByID(g, "R-any-visitor"); !ok {
		t.Fatal("advisory-only requirement did not land")
	}
}

func TestSemanticGate_UnresolvedFormalConflictBlocksWithoutLexicalEvidence(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	conflictID := seedFormalGateConflict(
		t, domainDir,
		"R-formal-a", "operators may archive audit logs",
		"R-formal-b", "build agents calculate package checksums",
	)
	hadConflict, err := semanticConflictGate(domainDir, proposal.ProposedRequirement{
		ID: "R-formal-a", Claim: "independent quantum scheduling rule", Owner: "owner",
		Status: ontology.StatusSETTLED, Why: "formal carrier test",
	}, landAckOptions{})
	if !hadConflict || err == nil {
		t.Fatalf("unresolved carrier must block even without lexical overlap: hadConflict=%v err=%v", hadConflict, err)
	}
	if !strings.Contains(err.Error(), conflictID) {
		t.Errorf("blocker evidence should name the matching formal carrier %s: %v", conflictID, err)
	}
}
func TestSemanticGate_UnrelatedAckCannotOverrideMatchingCarrier(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	gp := graphPathForDomain(domainDir)
	seedFormalGateConflict(
		t, domainDir,
		"R-formal-a", "operators may archive audit logs",
		"R-formal-b", "build agents calculate package checksums",
	)
	third := writeReqProposalJSON(t, "R-formal-c", "reviewers classify package metadata")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", third}); err != nil {
		t.Fatalf("land third requirement: %v", err)
	}
	if err := proposal.Apply(gp, "2026-07-14", proposal.ProposedAxis{
		Slug: "operations", Description: "operational policy",
	}); err != nil {
		t.Fatalf("add unrelated axis: %v", err)
	}
	otherConflict := proposal.ProposedConflict{
		Axis: "operations", Context: "unrelated member pair",
		Members: []string{"R-formal-b", "R-formal-c"}, Resolver: "resolver-gate",
	}
	if err := proposal.Apply(gp, "2026-07-14", otherConflict); err != nil {
		t.Fatalf("add unrelated formal carrier: %v", err)
	}
	unrelatedID := ontology.ConflictIdentity(otherConflict.Axis, otherConflict.Context)
	hadConflict, err := semanticConflictGate(domainDir, proposal.ProposedRequirement{
		ID: "R-formal-a", Claim: "independent quantum scheduling rule", Owner: "owner",
		Status: ontology.StatusSETTLED, Why: "formal carrier test",
	}, landAckOptions{AckConflict: unrelatedID})
	if !hadConflict || err == nil {
		t.Fatalf("an unrelated existing Conflict must not override the matching carrier: hadConflict=%v err=%v", hadConflict, err)
	}
}

func TestLandGate_RecordedDecisionsOverrideFormalCarrier(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	conflictID := seedFormalGateConflict(
		t, domainDir,
		"R-formal-a", "operators may archive audit logs",
		"R-formal-b", "build agents calculate package checksums",
	)
	withConflict := writeReqProposalJSON(t, "R-formal-a", "independent quantum scheduling rule")
	if err := cmdLand([]string{
		"--domain", domainDir, "--today", "2026-07-14",
		"--ack-conflict", conflictID, withConflict,
	}); err != nil {
		t.Fatalf("matching Conflict citation should override the formal blocker: %v", err)
	}
	const decisionRef = "decision-record-42"
	withReference := writeReqProposalJSON(t, "R-formal-a", "independent calendar coordination rule")
	if err := cmdLand([]string{
		"--domain", domainDir, "--today", "2026-07-14",
		"--decision-ref", decisionRef, withReference,
	}); err != nil {
		t.Fatalf("decision reference should override the formal blocker: %v", err)
	}
	g, err := loader.LoadGraph(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	r, ok := ontology.RequirementByID(g, "R-formal-a")
	if !ok {
		t.Fatal("R-formal-a missing after recorded overrides")
	}
	foundConflict, foundReference := false, false
	for _, entry := range r.History {
		foundConflict = foundConflict || strings.Contains(entry.Summary, conflictID)
		foundReference = foundReference || strings.Contains(entry.Summary, decisionRef)
	}
	if !foundConflict || !foundReference {
		t.Fatalf("both recorded decision forms should remain in History: %+v", r.History)
	}
}

func TestSemanticGate_DecidedConflictDoesNotBlock(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	conflictID := seedFormalGateConflict(
		t, domainDir,
		"R-formal-a", "operators may archive audit logs",
		"R-formal-b", "build agents calculate package checksums",
	)
	if err := proposal.Apply(graphPathForDomain(domainDir), "2026-07-14", proposal.ProposedConflictTransition{
		ConflictID: conflictID, NewLifecycle: "DECIDED(tension resolved)",
		DecidedBy: "resolver-gate",
	}); err != nil {
		t.Fatalf("record conflict decision: %v", err)
	}
	hadConflict, err := semanticConflictGate(domainDir, proposal.ProposedRequirement{
		ID: "R-formal-a", Claim: "independent quantum scheduling rule", Owner: "owner",
		Status: ontology.StatusSETTLED, Why: "formal carrier test",
	}, landAckOptions{})
	if hadConflict || err != nil {
		t.Fatalf("DECIDED carrier must no longer block: hadConflict=%v err=%v", hadConflict, err)
	}
}

func TestApplyProposalGate_ExplicitCarrierLeavesGraphUnchanged(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	seedFormalGateConflict(
		t, domainDir,
		"R-formal-a", "operators may archive audit logs",
		"R-formal-b", "build agents calculate package checksums",
	)
	gp := graphPathForDomain(domainDir)
	before, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph before apply: %v", err)
	}
	candidate := writeReqProposalJSON(t, "R-formal-a", "independent quantum scheduling rule")
	err = cmdApplyProposal([]string{"--domain", domainDir, "--today", "2026-07-14", candidate})
	if err == nil {
		t.Fatal("matching unresolved Conflict carrier must block apply-proposal")
	}
	after, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph after blocked apply: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("blocked apply-proposal changed graph state")
	}
}

func TestApplyProposalBatch_ExplicitCarrierRefusesAtomically(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	seedFormalGateConflict(
		t, domainDir,
		"R-formal-a", "operators may archive audit logs",
		"R-formal-b", "build agents calculate package checksums",
	)
	gp := graphPathForDomain(domainDir)
	before, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph before batch: %v", err)
	}
	batchDir := t.TempDir()
	proposals := []struct {
		name string
		data string
	}{
		{"01-benign.json", `{"kind":"Requirement","id":"R-batch-benign","claim":"tea grading remains stable","owner":"owner","status":"DRAFT","why":"batch test"}`},
		{"02-formal.json", `{"kind":"Requirement","id":"R-formal-a","claim":"independent quantum scheduling rule","owner":"owner","status":"SETTLED","why":"batch test"}`},
	}
	for _, item := range proposals {
		if err := os.WriteFile(filepath.Join(batchDir, item.name), []byte(item.data), 0o644); err != nil {
			t.Fatalf("write batch proposal %s: %v", item.name, err)
		}
	}
	err = cmdApplyProposal([]string{"--domain", domainDir, "--today", "2026-07-14", "--batch", batchDir})
	if err == nil {
		t.Fatal("matching unresolved carrier must block the batch")
	}
	after, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph after blocked batch: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("formal blocker must leave the entire batch unapplied")
	}
}

func TestOnlyAnyAckDoesNotCreateFalseConflictHistory(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	first := writeReqProposalJSON(t, "R-only-admins", "only admins must archive invoices")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", first}); err != nil {
		t.Fatalf("land first requirement: %v", err)
	}
	second := writeReqProposalJSON(t, "R-any-visitor", "any visitors must create passports")
	if err := cmdLand([]string{
		"--domain", domainDir, "--today", "2026-07-14",
		"--decision-ref", "no formal conflict exists",
		second,
	}); err != nil {
		t.Fatalf("advisory-only marker match should land: %v", err)
	}
	g, err := loader.LoadGraph(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	r, ok := ontology.RequirementByID(g, "R-any-visitor")
	if !ok {
		t.Fatal("R-any-visitor not found after land")
	}
	for _, entry := range r.History {
		if strings.Contains(entry.Summary, "semantic conflict acknowledged") {
			t.Fatalf("advisory-only marker match must not create conflict history: %+v", entry)
		}
	}
}

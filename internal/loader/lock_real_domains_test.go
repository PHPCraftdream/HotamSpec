package loader_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
	hotamspec "github.com/PHPCraftdream/HotamSpec/internal/recorder/canon"
)

// realDomains are the on-disk domain directories whose source graph.json MUST be
// governed by the proposal writer (R-no-hand-edit-graph). Each carries a sibling
// graph.lock whose sha256 pin is produced/verified by internal/loader (lock.go);
// a pin that no longer matches the current graph.json is the structural signal that
// the graph was edited outside hotam apply-proposal / hotam land.
//
// Paths are relative to the internal/loader package directory (the test working
// dir), matching the convention used by loader_test.go's fixturePath.
var realDomains = []string{
	"../../domains/hotam-spec-self",
	"../../domains/hotam-dev",
}

// minimalValidGraph is the smallest graph that passes the engine's own invariant
// suite (mirrors internal/proposal/fixture_test.go's baseGraph): one axis, three
// stakeholders (a non-member steward "outsider" + two requirement owners), one
// holding assumption, three SETTLED requirements, and one ACKNOWLEDGED conflict
// whose steward owns none of its members. It exists so R-no-hand-edit-graph's
// sanctioned-write-path half can be exercised against a graph that proposal.Apply's
// internal invariants.AllViolations check will not reject -- proving Apply is the
// path that writes graph.json + graph.lock TOGETHER, not just that a stale lock can
// be detected after the fact.
func minimalValidGraph() *ontology.Graph {
	axis := "cost-vs-flexibility"
	context := "shared scenario"
	return &ontology.Graph{
		Axes: []ontology.Axis{
			{Slug: axis, Description: "cost vs flexibility"},
		},
		Stakeholders: []ontology.Stakeholder{
			{ID: "outsider", Name: "Outsider", Domain: "x"},
			{ID: "sa", Name: "A", Domain: "x"},
			{ID: "sb", Name: "B", Domain: "x"},
		},
		Assumptions: []ontology.Assumption{
			{ID: "A-base", Statement: "the substrate is stable", Status: ontology.AssumptionHOLDS, Owner: "sa"},
		},
		Requirements: []ontology.Requirement{
			{ID: "R-1", Claim: "claim R-1", Owner: "sa", Status: ontology.StatusSETTLED, Why: "why R-1", Enforcement: ontology.EnforcementPROSE, Enforceability: ontology.EnforceabilityENFORCEABLE},
			{ID: "R-2", Claim: "claim R-2", Owner: "sb", Status: ontology.StatusSETTLED, Why: "why R-2", Enforcement: ontology.EnforcementPROSE, Enforceability: ontology.EnforceabilityENFORCEABLE},
			{ID: "R-3", Claim: "claim R-3", Owner: "sa", Status: ontology.StatusSETTLED, Why: "why R-3", Enforcement: ontology.EnforcementPROSE, Enforceability: ontology.EnforceabilityENFORCEABLE},
		},
		Conflicts: []ontology.Conflict{
			{
				ID:        ontology.ConflictIdentity(axis, context),
				Axis:      axis,
				Context:   context,
				Members:   []string{"R-1", "R-2"},
				Steward:   "outsider",
				Lifecycle: "ACKNOWLEDGED",
			},
		},
	}
}

// TestNoHandEditGraph_RealDomainLocksPinCurrentGraph enforces R-no-hand-edit-graph
// ("Changes to graph.json shall be made only through hotam apply-proposal/land,
// hand-edits prohibited") along BOTH halves of the claim:
//
//  1. the GUARD half -- for every real domain, the sha256 in graph.lock MUST equal
//     the sha256 of the current graph.json. A hand-edit that bypassed hotam land /
//     apply-proposal (which always writes graph.json + graph.lock together via
//     WriteGraph/WriteLock) leaves the lock stale, which VerifyLock detects.
//  2. the SANCTIONED-PATH half -- proposal.Apply, the very function the requirement
//     cites as its implemented_by (internal/proposal/apply.go:Apply), MUST write
//     graph.json and graph.lock together so that VerifyLock passes on the result.
//     Exercising Apply directly (against a temp copy of a minimal valid graph, not a
//     real domain) proves the sanctioned write path is what KEEPS the lock current;
//     without this the test would assert only that staleness is DETECTABLE, never
//     that the legitimate writer is what prevents it -- which is exactly the gap
//     task W2.2's check_scenario_executes_impl flagged (the cited verified_by never
//     executed Apply's own lines).
//
// Rewritten onto the hotamspec scenario recorder (task W3.1 self-example): a plain
// `go test` run is PURE ASSERTS, byte-identical in enforcement to the hand-rolled
// version it replaces -- every s.Then still asserts through t.Errorf exactly as the
// old bare `if !ok { t.Errorf }` did; the Given/When/Then narration becomes the
// source of generated SPEC.md prose.
//
// This file is the EXTERNAL test package loader_test (not the internal package
// loader) deliberately: internal/proposal imports internal/loader, so a test
// compiled INTO package loader could not import internal/proposal without an import
// cycle. Go's external test package (package loader_test) is compiled as a separate
// package and is the standard idiom for exactly this situation -- it can import both
// loader and proposal while adding nothing to loader's own import graph.
func TestNoHandEditGraph_RealDomainLocksPinCurrentGraph(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-no-hand-edit-graph",
		"graph.json changes go only through proposal.Apply (writes graph+lock together) and a stale lock is detectable by VerifyLock")

	// --- GUARD half: real-domain locks must match (hand-edit detector) ------
	s.Given("the real on-disk domains each carry a sibling graph.lock pinning their graph.json",
		"domain_count", len(realDomains))
	allRealOK := true
	for _, dir := range realDomains {
		graphPath := filepath.Join(dir, "graph.json")
		ok, err := loader.VerifyLock(graphPath)
		if err != nil {
			t.Fatalf("VerifyLock(%s): %v", graphPath, err)
		}
		if !ok {
			allRealOK = false
			t.Errorf("graph.lock pin does not match graph.json for %s -- "+
				"the graph was changed outside hotam apply-proposal/land (R-no-hand-edit-graph)", dir)
		}
	}
	s.When("VerifyLock checks each real domain's graph.lock against its graph.json")
	s.Then("every real graph.lock pins its current graph.json (no hand-edits bypassed apply-proposal/land)", allRealOK)

	// --- SANCTIONED-PATH half: proposal.Apply writes graph+lock together ----
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	if err := loader.WriteGraph(graphPath, minimalValidGraph()); err != nil {
		t.Fatalf("WriteGraph(minimal): %v", err)
	}
	s.Given("a minimal valid graph written to a temp dir via the sanctioned writer (no lock yet on disk)")

	p := proposal.ProposedRequirement{
		ID:     "R-no-hand-edit-probe",
		Claim:  "a change landed through the sanctioned apply-proposal path",
		Owner:  "sa",
		Status: ontology.StatusDRAFT,
		Why:    "exercises internal/proposal/apply.go:Apply to prove it writes graph+lock together",
	}
	if err := proposal.Apply(graphPath, "2026-07-18", p); err != nil {
		t.Fatalf("Apply (sanctioned write path): %v", err)
	}
	s.When("proposal.Apply lands a ProposedRequirement against the temp graph (R-no-hand-edit-graph's implemented_by symbol)")

	applyOK, applyErr := loader.VerifyLock(graphPath)
	s.Then("VerifyLock confirms the post-apply graph.lock matches the post-apply graph.json",
		applyOK && applyErr == nil)

	reloaded, err := loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("LoadGraph(reload): %v", err)
	}
	found := false
	for _, r := range reloaded.Requirements {
		if r.ID == "R-no-hand-edit-probe" {
			found = true
		}
	}
	s.Then("the applied proposal is actually present in the graph on disk (Apply wrote it)", found)
	s.Value("applied_requirement", fmt.Sprintf("%d requirements after apply", len(reloaded.Requirements)))
}

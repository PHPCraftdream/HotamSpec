package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

// These tests pin review finding P3-12: docs/gen/live-state.md — the last
// non-localized docs/gen projection with no freshness check — must be judged
// by check_live_state_md_current (real logic wired in from
// live_state_current_wiring.go): fully overwriting the committed file with
// garbage must yield exactly ONE violation, a clean genSpec must yield zero,
// and the check must pin its render to the generation date the file stamps
// itself with (`- **generated:** <date>`), so a content-identical file never
// goes stale at midnight.

// p3_12Fixture builds the standard non-empty full-profile fixture domain
// (same shape TestClaudeMDCurrent_UsesPublicationFlavor_ComparativeRender
// uses), runs one warm-up genSpec pass, and returns the paths.
func p3_12Fixture(t *testing.T, name string) (projectRoot, domainDir, crystalPath, liveStatePath string) {
	t.Helper()
	projectRoot, domainDir = initDomainUnderRoot(t, name, "2026-10-05")
	manifestPath := filepath.Join(domainDir, "manifest.json")
	manifest := `{"self_hosting":false,"gen_profile":"full","parent":null,"orientation_faq":[{"question":"project identity","keywords":["` + name + `"]}]}` + "\n"
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	graphPath := filepath.Join(domainDir, "graph.json")
	graph, err := loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("load graph.json: %v", err)
	}
	// Seed one Stakeholder so the graph is non-empty
	// (generator.LiveStateMDHasContent == !g.IsEmpty()) and the standalone
	// live-state.md is actually written under the full profile.
	graph.Stakeholders = append(graph.Stakeholders, ontology.Stakeholder{ID: "st-ops", Name: "Ops", Domain: name, DeclOrder: 1})
	if err := loader.WriteGraph(graphPath, graph); err != nil {
		t.Fatalf("write graph.json: %v", err)
	}
	crystalPath = filepath.Join(projectRoot, "CLAUDE.md")
	if err := os.WriteFile(filepath.Join(projectRoot, paths.MarkerFilename), nil, 0o644); err != nil {
		t.Fatalf("write project marker: %v", err)
	}
	seedFingerprintSource(t, projectRoot)
	if _, _, err := genSpec(domainDir, crystalPath, "2026-10-05", "full", false); err != nil {
		t.Fatalf("warm-up genSpec: %v", err)
	}
	liveStatePath = filepath.Join(domainDir, "docs", "gen", "live-state.md")
	if _, err := os.Stat(liveStatePath); err != nil {
		t.Fatalf("precondition: live-state.md must exist after warm-up genSpec: %v", err)
	}
	return projectRoot, domainDir, crystalPath, liveStatePath
}

func p3_12LiveStateViolations(t *testing.T, domainDir, today string) []string {
	t.Helper()
	violations, err := allViolationsAsOf(domainDir, today)
	if err != nil {
		t.Fatalf("allViolationsAsOf: %v", err)
	}
	// Only check_live_state_md_current is asserted here: other checks (e.g.
	// check_domain_claude_md_current) may legitimately fire against a render
	// dated years later — full-tree cleanliness under a later --today is the
	// domain of TestGenSpec_AgentContextConvergesInOnePass's own assertions.
	var found []string
	for _, v := range violations {
		if v.Check == "check_live_state_md_current" {
			found = append(found, v.Message)
		}
	}
	return found
}

// TestLiveStateCurrent_GarbageYieldsExactlyOneViolation pins the P3-12 root
// finding: before the fix, fully overwriting live-state.md with garbage
// yielded 0 violations.
func TestLiveStateCurrent_GarbageYieldsExactlyOneViolation(t *testing.T) {
	_, domainDir, _, _ := p3_12Fixture(t, "p3-12-garbage")
	liveStatePath := filepath.Join(domainDir, "docs", "gen", "live-state.md")
	if err := os.WriteFile(liveStatePath, []byte("garbage"), 0o644); err != nil {
		t.Fatalf("overwrite live-state.md with garbage: %v", err)
	}
	found := p3_12LiveStateViolations(t, domainDir, "2026-10-05")
	if len(found) != 1 {
		t.Fatalf("want exactly 1 check_live_state_md_current violation for garbage, got %d: %v", len(found), found)
	}
}

// TestLiveStateCurrent_CleanGenSpecYieldsZero pins the clean case: after a
// genSpec run, the committed live-state.md (including its new
// `- **generated:**` stamp) matches the fresh publication-flavored render.
func TestLiveStateCurrent_CleanGenSpecYieldsZero(t *testing.T) {
	_, domainDir, _, liveStatePath := p3_12Fixture(t, "p3-12-clean")
	committed, err := os.ReadFile(liveStatePath)
	if err != nil {
		t.Fatalf("read live-state.md: %v", err)
	}
	if !strings.Contains(string(committed), "- **generated:** 2026-10-05") {
		t.Fatalf("live-state.md must stamp its generation date, got:\n%s", committed)
	}
	if found := p3_12LiveStateViolations(t, domainDir, "2026-10-05"); len(found) != 0 {
		t.Fatalf("clean genSpec must yield 0 check_live_state_md_current violations, got %v", found)
	}
	// Full-tree cleanliness of the same fixture (every check) is pinned by
	// TestGenSpec_AgentContextConvergesInOnePass / TestClaudeMDCurrent_
	// UsesPublicationFlavor_ComparativeRender; here the live-state stamp is
	// the new moving part and the strict cross-check is done above.
}

// TestLiveStateCurrent_StampedDatePin pins the date handling: the render
// depends on `today` (freshness signals), so the check must compare as of
// the generation date the file stamps itself with — a later calendar day on
// an unchanged file yields zero violations.
func TestLiveStateCurrent_StampedDatePin(t *testing.T) {
	_, domainDir, crystalPath, _ := p3_12Fixture(t, "p3-12-stamp")
	// The pin can only be observed if the render actually depends on `today`:
	// seed a SETTLED requirement whose review_after lies in the past, so the
	// freshness signals' OVERDUE count (and the `--today` advisory) differ
	// between the stamped date and a later calendar day.
	graphPath := filepath.Join(domainDir, "graph.json")
	graph, err := loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("load graph.json: %v", err)
	}
	graph.Requirements = append(graph.Requirements, ontology.Requirement{
		ID: "R-p3-12-stale-review", Claim: "date-sensitivity probe for the stamped-date pin",
		Status: ontology.StatusSETTLED, ReviewAfter: "2020-01-01",
		Enforcement: ontology.EnforcementENFORCED, Enforceability: "ENFORCEABLE",
	})
	if err := loader.WriteGraph(graphPath, graph); err != nil {
		t.Fatalf("write graph.json: %v", err)
	}
	// Regenerate so the committed file matches the CURRENT graph as of the
	// stamped date; the probe is only about the file's own date pin, not
	// staleness of the graph.
	if _, _, err := genSpec(domainDir, crystalPath, "2026-10-05", "full", false); err != nil {
		t.Fatalf("regen genSpec: %v", err)
	}
	if found := p3_12LiveStateViolations(t, domainDir, "2030-01-01"); len(found) != 0 {
		t.Fatalf("unchanged live-state.md flagged stale on a later day (stamped-date pin broken): %v", found)
	}
}

// TestLiveStateCurrent_EmptyGraphHonestNoOp pins the withheld-projection
// no-op: a domain whose graph is genuinely empty never gets live-state.md
// written (generator.LiveStateMDHasContent == !g.IsEmpty(), task #364), so
// the check must stay silent — an absent projection is an honest no-op, not
// a violation.
func TestLiveStateCurrent_EmptyGraphHonestNoOp(t *testing.T) {
	projectRoot, domainDir := initDomainUnderRoot(t, "p3-12-empty", "2026-10-05")
	manifestPath := filepath.Join(domainDir, "manifest.json")
	manifest := `{"self_hosting":false,"gen_profile":"full","parent":null,"orientation_faq":[{"question":"project identity","keywords":["p3-12-empty"]}]}` + "\n"
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	crystalPath := filepath.Join(projectRoot, "CLAUDE.md")
	if err := os.WriteFile(filepath.Join(projectRoot, paths.MarkerFilename), nil, 0o644); err != nil {
		t.Fatalf("write project marker: %v", err)
	}
	seedFingerprintSource(t, projectRoot)
	if _, _, err := genSpec(domainDir, crystalPath, "2026-10-05", "full", false); err != nil {
		t.Fatalf("genSpec on empty-graph domain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(domainDir, "docs", "gen", "live-state.md")); !os.IsNotExist(err) {
		t.Fatalf("precondition: empty-graph domain must NOT get live-state.md (err=%v)", err)
	}
	if found := p3_12LiveStateViolations(t, domainDir, "2026-10-05"); len(found) != 0 {
		t.Fatalf("empty-graph domain must be an honest no-op, got %v", found)
	}
}

// TestLiveStateCurrent_LocalizedDomainHonestNoOp unit-tests the wiring
// function directly: a graph with explicit language configurations is judged
// by check_language_outputs_current, never by this root renderer.
func TestLiveStateCurrent_LocalizedDomainHonestNoOp(t *testing.T) {
	g := &ontology.Graph{DomainDir: t.TempDir(), Languages: []string{"de"}}
	if got := checkLiveStateMDCurrentReal(g, nil, "2026-10-05"); got != nil {
		t.Fatalf("localized domain must be an honest no-op, got %v", got)
	}
	// And the graph-without-DomainDir no-op mirrors agent-context's.
	if got := checkLiveStateMDCurrentReal(&ontology.Graph{}, nil, "2026-10-05"); got != nil {
		t.Fatalf("graph without DomainDir must be an honest no-op, got %v", got)
	}
}

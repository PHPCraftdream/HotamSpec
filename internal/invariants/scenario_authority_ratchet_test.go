package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// --- scenario_authority:"quality" one-way ratchet (task #397/W1.5) ---
//
// Symmetric with public_surface_authority_ratchet_test.go's own ratchet test
// suite: once a domain's manifest.json has been observed with
// scenario_authority:"quality" (pinned in graph.lock's
// ScenarioAuthorityQualityObserved by loader.WriteLock), a later manifest
// that no longer resolves scenario_authority:"quality" is a regression
// violation.

// writeScenarioAuthorityRatchetFixture writes a minimal domain directory
// (manifest.json + graph.json) with the given discipline/scenario_authority
// values, plus a graph.lock, and returns the domain dir. If writeLock is
// true, loader.WriteLock is called to produce a real graph.lock (which will
// ratchet scenario_authority_quality_observed based on the current
// manifest's scenario_authority value), mirroring
// writePublicSurfaceAuthorityRatchetFixture exactly.
func writeScenarioAuthorityRatchetFixture(t *testing.T, discipline, scenarioAuthority string, writeLock bool) string {
	t.Helper()
	tmp := t.TempDir()
	manifest := `{"discipline": "` + discipline + `", "scenario_authority": "` + scenarioAuthority + `"}` + "\n"
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "graph.json"), []byte(`{"schema_version":3}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile graph.json: %v", err)
	}
	if writeLock {
		graphPath := filepath.Join(tmp, "graph.json")
		if err := loader.WriteLock(graphPath, "test scenario_authority ratchet pin"); err != nil {
			t.Fatalf("WriteLock: %v", err)
		}
	}
	return tmp
}

// scenarioAuthorityRatchetGraph builds a graph with Discipline/
// ScenarioAuthorityQuality resolved from the fixture's own manifest.json,
// mirroring publicSurfaceAuthorityRatchetGraph exactly.
func scenarioAuthorityRatchetGraph(t *testing.T, domainDir string) *ontology.Graph {
	t.Helper()
	graphPath := filepath.Join(domainDir, "graph.json")
	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	return g
}

// TestCheckScenarioAuthorityRatchet_FiresOnRegression is the core exploit
// case (mirrors TestCheckPublicSurfaceAuthorityRatchet_FiresOnRegression): a
// domain first landed with scenario_authority:"quality" (graph.lock pins
// ScenarioAuthorityQualityObserved=true), then the resolver silently removed
// (or downgraded) the scenario_authority key from manifest.json.
// check_scenario_authority_ratchet MUST fire -- the one-way door was
// violated.
func TestCheckScenarioAuthorityRatchet_FiresOnRegression(t *testing.T) {
	t.Parallel()
	// Step 1: land with discipline:full + scenario_authority:quality + write
	// lock (pins the ratchet).
	domainDir := writeScenarioAuthorityRatchetFixture(t, "full", loader.ScenarioAuthorityQuality, true)

	// Verify the lock was written with the pin.
	graphPath := filepath.Join(domainDir, "graph.json")
	pin, exists := loader.ReadScenarioAuthorityPin(graphPath)
	if !exists {
		t.Fatalf("test setup: graph.lock should exist after WriteLock")
	}
	if !pin {
		t.Fatalf("test setup: expected ScenarioAuthorityQualityObserved=true after WriteLock with scenario_authority:quality, got false")
	}

	// Step 2: resolver silently rolls scenario_authority back to some other
	// value -- a regression attempt.
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"),
		[]byte(`{"discipline": "full", "scenario_authority": "informal"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile regression: %v", err)
	}

	g := scenarioAuthorityRatchetGraph(t, domainDir)
	if g.ScenarioAuthorityQuality {
		t.Fatalf("test setup: expected ScenarioAuthorityQuality=false after rollback to \"informal\", got true")
	}

	vs := runCheck(t, "check_scenario_authority_ratchet", g)
	if len(vs) == 0 {
		t.Fatalf("expected check_scenario_authority_ratchet to fire for a scenario_authority regression (was quality, now informal), got 0 violations")
	}
	if vs[0].Check != "check_scenario_authority_ratchet" {
		t.Errorf("violation Check = %q, want check_scenario_authority_ratchet", vs[0].Check)
	}
}

// TestCheckScenarioAuthorityRatchet_FiresWhenKeyRemoved proves the
// regression also fires when the key is removed entirely (absent), not just
// downgraded to another literal.
func TestCheckScenarioAuthorityRatchet_FiresWhenKeyRemoved(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioAuthorityRatchetFixture(t, "full", loader.ScenarioAuthorityQuality, true)

	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"),
		[]byte(`{"discipline": "full"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile regression: %v", err)
	}

	g := scenarioAuthorityRatchetGraph(t, domainDir)
	if g.ScenarioAuthorityQuality {
		t.Fatalf("test setup: expected ScenarioAuthorityQuality=false after key removal, got true")
	}

	vs := runCheck(t, "check_scenario_authority_ratchet", g)
	if len(vs) == 0 {
		t.Fatalf("expected check_scenario_authority_ratchet to fire when scenario_authority key is removed entirely, got 0 violations")
	}
}

// TestCheckScenarioAuthorityRatchet_GreenWhenStaysQuality is the happy-path
// non-regression case: a domain that stays scenario_authority:"quality"
// throughout must NOT falsely violate.
func TestCheckScenarioAuthorityRatchet_GreenWhenStaysQuality(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioAuthorityRatchetFixture(t, "full", loader.ScenarioAuthorityQuality, true)

	g := scenarioAuthorityRatchetGraph(t, domainDir)
	if !g.ScenarioAuthorityQuality {
		t.Fatalf("test setup: expected ScenarioAuthorityQuality=true, got false")
	}

	vs := runCheck(t, "check_scenario_authority_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations for a domain that stays scenario_authority:quality (happy path), got: %+v", vs)
	}
}

// TestCheckScenarioAuthorityRatchet_NoOpWhenNeverQuality proves the ratchet
// does NOT fire for a domain that was never scenario_authority:"quality" --
// the honest-no-op default (absent key), matching
// TestCheckPublicSurfaceAuthorityRatchet_NoOpWhenNeverLinked's shape.
func TestCheckScenarioAuthorityRatchet_NoOpWhenNeverQuality(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioAuthorityRatchetFixture(t, "full", "", true) // scenario_authority absent + lock

	graphPath := filepath.Join(domainDir, "graph.json")
	pin, exists := loader.ReadScenarioAuthorityPin(graphPath)
	if !exists {
		t.Fatalf("test setup: graph.lock should exist")
	}
	if pin {
		t.Fatalf("test setup: expected ScenarioAuthorityQualityObserved=false for a never-quality domain, got true")
	}

	g := scenarioAuthorityRatchetGraph(t, domainDir)
	vs := runCheck(t, "check_scenario_authority_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations for a domain that was never scenario_authority:quality, got: %+v", vs)
	}
}

// TestCheckScenarioAuthorityRatchet_NoOpWhenNoLock proves the honest-no-op
// case for a domain with no graph.lock at all (never went through
// WriteGraph/WriteLock).
func TestCheckScenarioAuthorityRatchet_NoOpWhenNoLock(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioAuthorityRatchetFixture(t, "full", loader.ScenarioAuthorityQuality, false) // no lock written

	g := scenarioAuthorityRatchetGraph(t, domainDir)
	vs := runCheck(t, "check_scenario_authority_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations for a domain with no graph.lock, got: %+v", vs)
	}
}

// TestCheckScenarioAuthorityRatchet_NoOpOnSyntheticGraph proves the bail-out
// for a synthetic in-memory graph (no DomainDir).
func TestCheckScenarioAuthorityRatchet_NoOpOnSyntheticGraph(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	vs := runCheck(t, "check_scenario_authority_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations on a synthetic graph (no DomainDir), got: %+v", vs)
	}
}

// TestWriteLock_RatchetsScenarioAuthorityQuality proves the WriteLock ratchet
// mechanism itself: once WriteLock observes scenario_authority:"quality", the
// pin is set true, and a subsequent WriteLock with scenario_authority removed
// PRESERVES the pin (once true, always true) -- mirrors
// TestWriteLock_RatchetsPublicSurfaceAuthorityLinked exactly.
func TestWriteLock_RatchetsScenarioAuthorityQuality(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioAuthorityRatchetFixture(t, "full", loader.ScenarioAuthorityQuality, false)
	graphPath := filepath.Join(domainDir, "graph.json")

	// First WriteLock: pins ScenarioAuthorityQualityObserved=true (live value
	// is "quality").
	if err := loader.WriteLock(graphPath, "first lock"); err != nil {
		t.Fatalf("WriteLock first: %v", err)
	}
	pin, _ := loader.ReadScenarioAuthorityPin(graphPath)
	if !pin {
		t.Fatalf("after first WriteLock with scenario_authority:quality, expected pin=true, got false")
	}

	// Resolver removes scenario_authority from manifest.
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(`{"discipline": "full"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile remove scenario_authority: %v", err)
	}

	// Second WriteLock: pin MUST stay true (ratchet -- once true, always
	// true).
	if err := loader.WriteLock(graphPath, "second lock after regression"); err != nil {
		t.Fatalf("WriteLock second: %v", err)
	}
	pin2, _ := loader.ReadScenarioAuthorityPin(graphPath)
	if !pin2 {
		t.Fatalf("after second WriteLock with scenario_authority removed, expected pin=true (ratchet), got false")
	}
}

// TestScenarioAuthorityRatchet_IndependentFromDisciplineRatchet proves the
// two ratchets are independently pinned -- a domain can be discipline:full
// (pinned) while scenario_authority was never quality (not pinned).
func TestScenarioAuthorityRatchet_IndependentFromDisciplineRatchet(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioAuthorityRatchetFixture(t, "full", "", true) // discipline:full, scenario_authority absent

	graphPath := filepath.Join(domainDir, "graph.json")
	disciplinePin, exists1 := loader.ReadDisciplinePin(graphPath)
	if !exists1 {
		t.Fatalf("test setup: graph.lock should exist")
	}
	if !disciplinePin {
		t.Fatalf("expected DisciplineFullObserved=true (discipline:full was live), got false")
	}
	scenarioPin, exists2 := loader.ReadScenarioAuthorityPin(graphPath)
	if !exists2 {
		t.Fatalf("test setup: graph.lock should exist")
	}
	if scenarioPin {
		t.Fatalf("expected ScenarioAuthorityQualityObserved=false (scenario_authority was never quality), got true")
	}

	g := scenarioAuthorityRatchetGraph(t, domainDir)
	// Neither ratchet should fire: discipline stayed full (no regression),
	// and scenario_authority was never observed as quality (nothing to
	// ratchet).
	if vs := runCheck(t, "check_discipline_ratchet", g); len(vs) != 0 {
		t.Fatalf("expected no discipline ratchet violations, got: %+v", vs)
	}
	if vs := runCheck(t, "check_scenario_authority_ratchet", g); len(vs) != 0 {
		t.Fatalf("expected no scenario_authority ratchet violations, got: %+v", vs)
	}
}

// TestScenarioAuthorityRatchet_IndependentFromClaimAuthorityRatchet proves
// scenario_authority:"quality" and claim_authority:"scenario" are
// independently pinned and independently ratcheted -- a domain can be
// claim_authority:"scenario" (pinned) while scenario_authority was never
// quality (not pinned), and vice versa.
func TestScenarioAuthorityRatchet_IndependentFromClaimAuthorityRatchet(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	manifest := `{"discipline": "full", "claim_authority": "scenario", "scenario_authority": "quality"}` + "\n"
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "graph.json"), []byte(`{"schema_version":3}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile graph.json: %v", err)
	}
	graphPath := filepath.Join(tmp, "graph.json")
	if err := loader.WriteLock(graphPath, "both triggers on"); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	// Now roll BOTH back independently and confirm each ratchet fires on its
	// own, and clearing one does not affect the other's pin.
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"),
		[]byte(`{"discipline": "full", "claim_authority": "scenario"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile regression (drop scenario_authority only): %v", err)
	}
	g := scenarioAuthorityRatchetGraph(t, tmp)
	if !g.ClaimAuthorityScenario {
		t.Fatalf("expected ClaimAuthorityScenario still true (untouched), got false")
	}
	if g.ScenarioAuthorityQuality {
		t.Fatalf("expected ScenarioAuthorityQuality false after removal, got true")
	}
	if vs := runCheck(t, "check_claim_authority_ratchet", g); len(vs) != 0 {
		t.Fatalf("expected no claim_authority ratchet violation (claim_authority untouched), got: %+v", vs)
	}
	if vs := runCheck(t, "check_scenario_authority_ratchet", g); len(vs) == 0 {
		t.Fatalf("expected a scenario_authority ratchet violation (was quality, now removed), got none")
	}
}

// TestScenarioAuthorityRatchet_IndependentFromPublicSurfaceAuthorityRatchet
// proves scenario_authority:"quality" and public_surface_authority:"linked"
// are independently pinned and independently ratcheted -- a domain can be
// public_surface_authority:"linked" (pinned) while scenario_authority was
// never quality (not pinned), and vice versa.
func TestScenarioAuthorityRatchet_IndependentFromPublicSurfaceAuthorityRatchet(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	manifest := `{"discipline": "full", "public_surface_authority": "linked", "scenario_authority": "quality"}` + "\n"
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "graph.json"), []byte(`{"schema_version":3}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile graph.json: %v", err)
	}
	graphPath := filepath.Join(tmp, "graph.json")
	if err := loader.WriteLock(graphPath, "both triggers on"); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"),
		[]byte(`{"discipline": "full", "public_surface_authority": "linked"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile regression (drop scenario_authority only): %v", err)
	}
	g := scenarioAuthorityRatchetGraph(t, tmp)
	if !g.PublicSurfaceAuthorityLinked {
		t.Fatalf("expected PublicSurfaceAuthorityLinked still true (untouched), got false")
	}
	if g.ScenarioAuthorityQuality {
		t.Fatalf("expected ScenarioAuthorityQuality false after removal, got true")
	}
	if vs := runCheck(t, "check_public_surface_authority_ratchet", g); len(vs) != 0 {
		t.Fatalf("expected no public_surface_authority ratchet violation (untouched), got: %+v", vs)
	}
	if vs := runCheck(t, "check_scenario_authority_ratchet", g); len(vs) == 0 {
		t.Fatalf("expected a scenario_authority ratchet violation (was quality, now removed), got none")
	}
}

package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// --- claim_authority:"scenario" one-way ratchet (task #388/W0.1) -----------
//
// Symmetric with discipline_ratchet_test.go's own F2 ratchet test suite:
// once a domain's manifest.json has been observed with
// claim_authority:"scenario" (pinned in graph.lock's
// ClaimAuthorityScenarioObserved by loader.WriteLock), a later manifest that
// no longer resolves claim_authority:"scenario" is a regression violation.

// writeClaimAuthorityRatchetFixture writes a minimal domain directory
// (manifest.json + graph.json) with the given discipline/claim_authority
// values, plus a graph.lock, and returns the domain dir. If writeLock is
// true, loader.WriteLock is called to produce a real graph.lock (which will
// ratchet claim_authority_scenario_observed based on the current manifest's
// claim_authority value), mirroring writeRatchetFixture exactly.
func writeClaimAuthorityRatchetFixture(t *testing.T, discipline, claimAuthority string, writeLock bool) string {
	t.Helper()
	tmp := t.TempDir()
	manifest := `{"discipline": "` + discipline + `", "claim_authority": "` + claimAuthority + `"}` + "\n"
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "graph.json"), []byte(`{"schema_version":3}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile graph.json: %v", err)
	}
	if writeLock {
		graphPath := filepath.Join(tmp, "graph.json")
		if err := loader.WriteLock(graphPath, "test claim_authority ratchet pin"); err != nil {
			t.Fatalf("WriteLock: %v", err)
		}
	}
	return tmp
}

// claimAuthorityRatchetGraph builds a graph with Discipline/
// ClaimAuthorityScenario resolved from the fixture's own manifest.json,
// mirroring ratchetGraph exactly.
func claimAuthorityRatchetGraph(t *testing.T, domainDir string) *ontology.Graph {
	t.Helper()
	graphPath := filepath.Join(domainDir, "graph.json")
	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	return g
}

// TestCheckClaimAuthorityRatchet_FiresOnRegression is the core exploit case
// (mirrors TestCheckDisciplineRatchet_FiresOnRegression): a domain first
// landed with claim_authority:"scenario" (graph.lock pins
// ClaimAuthorityScenarioObserved=true), then the resolver silently removed
// (or downgraded) the claim_authority key from manifest.json.
// check_claim_authority_ratchet MUST fire -- the one-way door was violated.
// This is ALSO the acceptance-criterion (c) mutation test the task calls
// for: rolling back "scenario" -> absent/"authored" after the pin was
// observed must be a violation.
func TestCheckClaimAuthorityRatchet_FiresOnRegression(t *testing.T) {
	t.Parallel()
	// Step 1: land with discipline:full + claim_authority:scenario + write
	// lock (pins the ratchet).
	domainDir := writeClaimAuthorityRatchetFixture(t, "full", loader.ClaimAuthorityScenario, true)

	// Verify the lock was written with the pin.
	graphPath := filepath.Join(domainDir, "graph.json")
	pin, exists := loader.ReadClaimAuthorityPin(graphPath)
	if !exists {
		t.Fatalf("test setup: graph.lock should exist after WriteLock")
	}
	if !pin {
		t.Fatalf("test setup: expected ClaimAuthorityScenarioObserved=true after WriteLock with claim_authority:scenario, got false")
	}

	// Step 2: resolver silently rolls claim_authority back to "authored"
	// (the documented default literal) -- a regression attempt.
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"),
		[]byte(`{"discipline": "full", "claim_authority": "authored"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile regression: %v", err)
	}

	g := claimAuthorityRatchetGraph(t, domainDir)
	if g.ClaimAuthorityScenario {
		t.Fatalf("test setup: expected ClaimAuthorityScenario=false after rollback to \"authored\", got true")
	}

	vs := runCheck(t, "check_claim_authority_ratchet", g)
	if len(vs) == 0 {
		t.Fatalf("expected check_claim_authority_ratchet to fire for a claim_authority regression (was scenario, now authored), got 0 violations")
	}
	if vs[0].Check != "check_claim_authority_ratchet" {
		t.Errorf("violation Check = %q, want check_claim_authority_ratchet", vs[0].Check)
	}
}

// TestCheckClaimAuthorityRatchet_FiresWhenKeyRemoved proves the regression
// also fires when the key is removed entirely (absent), not just
// downgraded to the literal "authored".
func TestCheckClaimAuthorityRatchet_FiresWhenKeyRemoved(t *testing.T) {
	t.Parallel()
	domainDir := writeClaimAuthorityRatchetFixture(t, "full", loader.ClaimAuthorityScenario, true)

	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"),
		[]byte(`{"discipline": "full"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile regression: %v", err)
	}

	g := claimAuthorityRatchetGraph(t, domainDir)
	if g.ClaimAuthorityScenario {
		t.Fatalf("test setup: expected ClaimAuthorityScenario=false after key removal, got true")
	}

	vs := runCheck(t, "check_claim_authority_ratchet", g)
	if len(vs) == 0 {
		t.Fatalf("expected check_claim_authority_ratchet to fire when claim_authority key is removed entirely, got 0 violations")
	}
}

// TestCheckClaimAuthorityRatchet_GreenWhenStaysScenario is the happy-path
// non-regression case: a domain that stays claim_authority:"scenario"
// throughout must NOT falsely violate.
func TestCheckClaimAuthorityRatchet_GreenWhenStaysScenario(t *testing.T) {
	t.Parallel()
	domainDir := writeClaimAuthorityRatchetFixture(t, "full", loader.ClaimAuthorityScenario, true)

	g := claimAuthorityRatchetGraph(t, domainDir)
	if !g.ClaimAuthorityScenario {
		t.Fatalf("test setup: expected ClaimAuthorityScenario=true, got false")
	}

	vs := runCheck(t, "check_claim_authority_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations for a domain that stays claim_authority:scenario (happy path), got: %+v", vs)
	}
}

// TestCheckClaimAuthorityRatchet_NoOpWhenNeverScenario proves the ratchet
// does NOT fire for a domain that was never claim_authority:"scenario" --
// the honest-no-op default (absent key), matching TestCheckDisciplineRatchet_
// NoOpWhenNeverFull's shape. This is ALSO acceptance-criterion (a): a domain
// without the claim_authority key stays a clean no-op.
func TestCheckClaimAuthorityRatchet_NoOpWhenNeverScenario(t *testing.T) {
	t.Parallel()
	domainDir := writeClaimAuthorityRatchetFixture(t, "full", "", true) // claim_authority absent + lock

	graphPath := filepath.Join(domainDir, "graph.json")
	pin, exists := loader.ReadClaimAuthorityPin(graphPath)
	if !exists {
		t.Fatalf("test setup: graph.lock should exist")
	}
	if pin {
		t.Fatalf("test setup: expected ClaimAuthorityScenarioObserved=false for a never-scenario domain, got true")
	}

	g := claimAuthorityRatchetGraph(t, domainDir)
	vs := runCheck(t, "check_claim_authority_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations for a domain that was never claim_authority:scenario, got: %+v", vs)
	}
}

// TestCheckClaimAuthorityRatchet_NoOpWhenNoLock proves the honest-no-op case
// for a domain with no graph.lock at all (never went through
// WriteGraph/WriteLock).
func TestCheckClaimAuthorityRatchet_NoOpWhenNoLock(t *testing.T) {
	t.Parallel()
	domainDir := writeClaimAuthorityRatchetFixture(t, "full", loader.ClaimAuthorityScenario, false) // no lock written

	g := claimAuthorityRatchetGraph(t, domainDir)
	vs := runCheck(t, "check_claim_authority_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations for a domain with no graph.lock, got: %+v", vs)
	}
}

// TestCheckClaimAuthorityRatchet_NoOpOnSyntheticGraph proves the bail-out
// for a synthetic in-memory graph (no DomainDir).
func TestCheckClaimAuthorityRatchet_NoOpOnSyntheticGraph(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	vs := runCheck(t, "check_claim_authority_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations on a synthetic graph (no DomainDir), got: %+v", vs)
	}
}

// TestWriteLock_RatchetsClaimAuthorityScenario proves the WriteLock ratchet
// mechanism itself: once WriteLock observes claim_authority:"scenario", the
// pin is set true, and a subsequent WriteLock with claim_authority removed
// PRESERVES the pin (once true, always true) -- mirrors
// TestWriteLock_RatchetsDisciplineFull exactly.
func TestWriteLock_RatchetsClaimAuthorityScenario(t *testing.T) {
	t.Parallel()
	domainDir := writeClaimAuthorityRatchetFixture(t, "full", loader.ClaimAuthorityScenario, false)
	graphPath := filepath.Join(domainDir, "graph.json")

	// First WriteLock: pins ClaimAuthorityScenarioObserved=true (live value
	// is "scenario").
	if err := loader.WriteLock(graphPath, "first lock"); err != nil {
		t.Fatalf("WriteLock first: %v", err)
	}
	pin, _ := loader.ReadClaimAuthorityPin(graphPath)
	if !pin {
		t.Fatalf("after first WriteLock with claim_authority:scenario, expected pin=true, got false")
	}

	// Resolver removes claim_authority from manifest.
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(`{"discipline": "full"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile remove claim_authority: %v", err)
	}

	// Second WriteLock: pin MUST stay true (ratchet -- once true, always true).
	if err := loader.WriteLock(graphPath, "second lock after regression"); err != nil {
		t.Fatalf("WriteLock second: %v", err)
	}
	pin2, _ := loader.ReadClaimAuthorityPin(graphPath)
	if !pin2 {
		t.Fatalf("after second WriteLock with claim_authority removed, expected pin=true (ratchet), got false")
	}
}

// TestClaimAuthorityRatchet_IndependentFromDisciplineRatchet proves the two
// ratchets are independently pinned -- a domain can be discipline:full
// (pinned) while claim_authority was never scenario (not pinned), matching
// the real shape of prat/gpsm-sm after this task's own fix.
func TestClaimAuthorityRatchet_IndependentFromDisciplineRatchet(t *testing.T) {
	t.Parallel()
	domainDir := writeClaimAuthorityRatchetFixture(t, "full", "", true) // discipline:full, claim_authority absent

	graphPath := filepath.Join(domainDir, "graph.json")
	disciplinePin, exists1 := loader.ReadDisciplinePin(graphPath)
	if !exists1 {
		t.Fatalf("test setup: graph.lock should exist")
	}
	if !disciplinePin {
		t.Fatalf("expected DisciplineFullObserved=true (discipline:full was live), got false")
	}
	claimAuthorityPin, exists2 := loader.ReadClaimAuthorityPin(graphPath)
	if !exists2 {
		t.Fatalf("test setup: graph.lock should exist")
	}
	if claimAuthorityPin {
		t.Fatalf("expected ClaimAuthorityScenarioObserved=false (claim_authority was never scenario), got true")
	}

	g := claimAuthorityRatchetGraph(t, domainDir)
	// Neither ratchet should fire: discipline stayed full (no regression),
	// and claim_authority was never observed as scenario (nothing to ratchet).
	if vs := runCheck(t, "check_discipline_ratchet", g); len(vs) != 0 {
		t.Fatalf("expected no discipline ratchet violations, got: %+v", vs)
	}
	if vs := runCheck(t, "check_claim_authority_ratchet", g); len(vs) != 0 {
		t.Fatalf("expected no claim_authority ratchet violations, got: %+v", vs)
	}
}

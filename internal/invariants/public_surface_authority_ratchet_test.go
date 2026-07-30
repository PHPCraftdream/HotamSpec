package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// --- public_surface_authority:"linked" one-way ratchet (task #396/W1.4) ---
//
// Symmetric with claim_authority_ratchet_test.go's own ratchet test suite:
// once a domain's manifest.json has been observed with
// public_surface_authority:"linked" (pinned in graph.lock's
// PublicSurfaceAuthorityLinkedObserved by loader.WriteLock), a later
// manifest that no longer resolves public_surface_authority:"linked" is a
// regression violation.

// writePublicSurfaceAuthorityRatchetFixture writes a minimal domain
// directory (manifest.json + graph.json) with the given discipline/
// public_surface_authority values, plus a graph.lock, and returns the domain
// dir. If writeLock is true, loader.WriteLock is called to produce a real
// graph.lock (which will ratchet public_surface_authority_linked_observed
// based on the current manifest's public_surface_authority value), mirroring
// writeClaimAuthorityRatchetFixture exactly.
func writePublicSurfaceAuthorityRatchetFixture(t *testing.T, discipline, publicSurfaceAuthority string, writeLock bool) string {
	t.Helper()
	tmp := t.TempDir()
	manifest := `{"discipline": "` + discipline + `", "public_surface_authority": "` + publicSurfaceAuthority + `"}` + "\n"
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "graph.json"), []byte(`{"schema_version":3}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile graph.json: %v", err)
	}
	if writeLock {
		graphPath := filepath.Join(tmp, "graph.json")
		if err := loader.WriteLock(graphPath, "test public_surface_authority ratchet pin"); err != nil {
			t.Fatalf("WriteLock: %v", err)
		}
	}
	return tmp
}

// publicSurfaceAuthorityRatchetGraph builds a graph with Discipline/
// PublicSurfaceAuthorityLinked resolved from the fixture's own
// manifest.json, mirroring claimAuthorityRatchetGraph exactly.
func publicSurfaceAuthorityRatchetGraph(t *testing.T, domainDir string) *ontology.Graph {
	t.Helper()
	graphPath := filepath.Join(domainDir, "graph.json")
	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	return g
}

// TestCheckPublicSurfaceAuthorityRatchet_FiresOnRegression is the core
// exploit case (mirrors TestCheckClaimAuthorityRatchet_FiresOnRegression): a
// domain first landed with public_surface_authority:"linked" (graph.lock
// pins PublicSurfaceAuthorityLinkedObserved=true), then the resolver
// silently removed (or downgraded) the public_surface_authority key from
// manifest.json. check_public_surface_authority_ratchet MUST fire -- the
// one-way door was violated.
func TestCheckPublicSurfaceAuthorityRatchet_FiresOnRegression(t *testing.T) {
	t.Parallel()
	// Step 1: land with discipline:full + public_surface_authority:linked +
	// write lock (pins the ratchet).
	domainDir := writePublicSurfaceAuthorityRatchetFixture(t, "full", loader.PublicSurfaceAuthorityLinked, true)

	// Verify the lock was written with the pin.
	graphPath := filepath.Join(domainDir, "graph.json")
	pin, exists := loader.ReadPublicSurfaceAuthorityPin(graphPath)
	if !exists {
		t.Fatalf("test setup: graph.lock should exist after WriteLock")
	}
	if !pin {
		t.Fatalf("test setup: expected PublicSurfaceAuthorityLinkedObserved=true after WriteLock with public_surface_authority:linked, got false")
	}

	// Step 2: resolver silently rolls public_surface_authority back to some
	// other value -- a regression attempt.
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"),
		[]byte(`{"discipline": "full", "public_surface_authority": "authored"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile regression: %v", err)
	}

	g := publicSurfaceAuthorityRatchetGraph(t, domainDir)
	if g.PublicSurfaceAuthorityLinked {
		t.Fatalf("test setup: expected PublicSurfaceAuthorityLinked=false after rollback to \"authored\", got true")
	}

	vs := runCheck(t, "check_public_surface_authority_ratchet", g)
	if len(vs) == 0 {
		t.Fatalf("expected check_public_surface_authority_ratchet to fire for a public_surface_authority regression (was linked, now authored), got 0 violations")
	}
	if vs[0].Check != "check_public_surface_authority_ratchet" {
		t.Errorf("violation Check = %q, want check_public_surface_authority_ratchet", vs[0].Check)
	}
}

// TestCheckPublicSurfaceAuthorityRatchet_FiresWhenKeyRemoved proves the
// regression also fires when the key is removed entirely (absent), not just
// downgraded to another literal.
func TestCheckPublicSurfaceAuthorityRatchet_FiresWhenKeyRemoved(t *testing.T) {
	t.Parallel()
	domainDir := writePublicSurfaceAuthorityRatchetFixture(t, "full", loader.PublicSurfaceAuthorityLinked, true)

	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"),
		[]byte(`{"discipline": "full"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile regression: %v", err)
	}

	g := publicSurfaceAuthorityRatchetGraph(t, domainDir)
	if g.PublicSurfaceAuthorityLinked {
		t.Fatalf("test setup: expected PublicSurfaceAuthorityLinked=false after key removal, got true")
	}

	vs := runCheck(t, "check_public_surface_authority_ratchet", g)
	if len(vs) == 0 {
		t.Fatalf("expected check_public_surface_authority_ratchet to fire when public_surface_authority key is removed entirely, got 0 violations")
	}
}

// TestCheckPublicSurfaceAuthorityRatchet_GreenWhenStaysLinked is the
// happy-path non-regression case: a domain that stays
// public_surface_authority:"linked" throughout must NOT falsely violate.
func TestCheckPublicSurfaceAuthorityRatchet_GreenWhenStaysLinked(t *testing.T) {
	t.Parallel()
	domainDir := writePublicSurfaceAuthorityRatchetFixture(t, "full", loader.PublicSurfaceAuthorityLinked, true)

	g := publicSurfaceAuthorityRatchetGraph(t, domainDir)
	if !g.PublicSurfaceAuthorityLinked {
		t.Fatalf("test setup: expected PublicSurfaceAuthorityLinked=true, got false")
	}

	vs := runCheck(t, "check_public_surface_authority_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations for a domain that stays public_surface_authority:linked (happy path), got: %+v", vs)
	}
}

// TestCheckPublicSurfaceAuthorityRatchet_NoOpWhenNeverLinked proves the
// ratchet does NOT fire for a domain that was never
// public_surface_authority:"linked" -- the honest-no-op default (absent
// key), matching TestCheckClaimAuthorityRatchet_NoOpWhenNeverScenario's
// shape.
func TestCheckPublicSurfaceAuthorityRatchet_NoOpWhenNeverLinked(t *testing.T) {
	t.Parallel()
	domainDir := writePublicSurfaceAuthorityRatchetFixture(t, "full", "", true) // public_surface_authority absent + lock

	graphPath := filepath.Join(domainDir, "graph.json")
	pin, exists := loader.ReadPublicSurfaceAuthorityPin(graphPath)
	if !exists {
		t.Fatalf("test setup: graph.lock should exist")
	}
	if pin {
		t.Fatalf("test setup: expected PublicSurfaceAuthorityLinkedObserved=false for a never-linked domain, got true")
	}

	g := publicSurfaceAuthorityRatchetGraph(t, domainDir)
	vs := runCheck(t, "check_public_surface_authority_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations for a domain that was never public_surface_authority:linked, got: %+v", vs)
	}
}

// TestCheckPublicSurfaceAuthorityRatchet_NoOpWhenNoLock proves the
// honest-no-op case for a domain with no graph.lock at all (never went
// through WriteGraph/WriteLock).
func TestCheckPublicSurfaceAuthorityRatchet_NoOpWhenNoLock(t *testing.T) {
	t.Parallel()
	domainDir := writePublicSurfaceAuthorityRatchetFixture(t, "full", loader.PublicSurfaceAuthorityLinked, false) // no lock written

	g := publicSurfaceAuthorityRatchetGraph(t, domainDir)
	vs := runCheck(t, "check_public_surface_authority_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations for a domain with no graph.lock, got: %+v", vs)
	}
}

// TestCheckPublicSurfaceAuthorityRatchet_NoOpOnSyntheticGraph proves the
// bail-out for a synthetic in-memory graph (no DomainDir).
func TestCheckPublicSurfaceAuthorityRatchet_NoOpOnSyntheticGraph(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	vs := runCheck(t, "check_public_surface_authority_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("expected no violations on a synthetic graph (no DomainDir), got: %+v", vs)
	}
}

// TestWriteLock_RatchetsPublicSurfaceAuthorityLinked proves the WriteLock
// ratchet mechanism itself: once WriteLock observes
// public_surface_authority:"linked", the pin is set true, and a subsequent
// WriteLock with public_surface_authority removed PRESERVES the pin (once
// true, always true) -- mirrors TestWriteLock_RatchetsClaimAuthorityScenario
// exactly.
func TestWriteLock_RatchetsPublicSurfaceAuthorityLinked(t *testing.T) {
	t.Parallel()
	domainDir := writePublicSurfaceAuthorityRatchetFixture(t, "full", loader.PublicSurfaceAuthorityLinked, false)
	graphPath := filepath.Join(domainDir, "graph.json")

	// First WriteLock: pins PublicSurfaceAuthorityLinkedObserved=true (live
	// value is "linked").
	if err := loader.WriteLock(graphPath, "first lock"); err != nil {
		t.Fatalf("WriteLock first: %v", err)
	}
	pin, _ := loader.ReadPublicSurfaceAuthorityPin(graphPath)
	if !pin {
		t.Fatalf("after first WriteLock with public_surface_authority:linked, expected pin=true, got false")
	}

	// Resolver removes public_surface_authority from manifest.
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(`{"discipline": "full"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile remove public_surface_authority: %v", err)
	}

	// Second WriteLock: pin MUST stay true (ratchet -- once true, always true).
	if err := loader.WriteLock(graphPath, "second lock after regression"); err != nil {
		t.Fatalf("WriteLock second: %v", err)
	}
	pin2, _ := loader.ReadPublicSurfaceAuthorityPin(graphPath)
	if !pin2 {
		t.Fatalf("after second WriteLock with public_surface_authority removed, expected pin=true (ratchet), got false")
	}
}

// TestPublicSurfaceAuthorityRatchet_IndependentFromDisciplineRatchet proves
// the two ratchets are independently pinned -- a domain can be
// discipline:full (pinned) while public_surface_authority was never linked
// (not pinned).
func TestPublicSurfaceAuthorityRatchet_IndependentFromDisciplineRatchet(t *testing.T) {
	t.Parallel()
	domainDir := writePublicSurfaceAuthorityRatchetFixture(t, "full", "", true) // discipline:full, public_surface_authority absent

	graphPath := filepath.Join(domainDir, "graph.json")
	disciplinePin, exists1 := loader.ReadDisciplinePin(graphPath)
	if !exists1 {
		t.Fatalf("test setup: graph.lock should exist")
	}
	if !disciplinePin {
		t.Fatalf("expected DisciplineFullObserved=true (discipline:full was live), got false")
	}
	publicSurfacePin, exists2 := loader.ReadPublicSurfaceAuthorityPin(graphPath)
	if !exists2 {
		t.Fatalf("test setup: graph.lock should exist")
	}
	if publicSurfacePin {
		t.Fatalf("expected PublicSurfaceAuthorityLinkedObserved=false (public_surface_authority was never linked), got true")
	}

	g := publicSurfaceAuthorityRatchetGraph(t, domainDir)
	// Neither ratchet should fire: discipline stayed full (no regression),
	// and public_surface_authority was never observed as linked (nothing to
	// ratchet).
	if vs := runCheck(t, "check_discipline_ratchet", g); len(vs) != 0 {
		t.Fatalf("expected no discipline ratchet violations, got: %+v", vs)
	}
	if vs := runCheck(t, "check_public_surface_authority_ratchet", g); len(vs) != 0 {
		t.Fatalf("expected no public_surface_authority ratchet violations, got: %+v", vs)
	}
}

// TestPublicSurfaceAuthorityRatchet_IndependentFromClaimAuthorityRatchet
// proves public_surface_authority:"linked" and claim_authority:"scenario"
// are independently pinned and independently ratcheted -- a domain can be
// claim_authority:"scenario" (pinned) while public_surface_authority was
// never linked (not pinned), and vice versa.
func TestPublicSurfaceAuthorityRatchet_IndependentFromClaimAuthorityRatchet(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	manifest := `{"discipline": "full", "claim_authority": "scenario", "public_surface_authority": "linked"}` + "\n"
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "graph.json"), []byte(`{"schema_version":3}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile graph.json: %v", err)
	}
	graphPath := filepath.Join(tmp, "graph.json")
	if err := loader.WriteLock(graphPath, "both triggers linked"); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	// Now roll BOTH back independently and confirm each ratchet fires on its
	// own, and clearing one does not affect the other's pin.
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"),
		[]byte(`{"discipline": "full", "claim_authority": "scenario"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile regression (drop public_surface_authority only): %v", err)
	}
	g := publicSurfaceAuthorityRatchetGraph(t, tmp)
	if !g.ClaimAuthorityScenario {
		t.Fatalf("expected ClaimAuthorityScenario still true (untouched), got false")
	}
	if g.PublicSurfaceAuthorityLinked {
		t.Fatalf("expected PublicSurfaceAuthorityLinked false after removal, got true")
	}
	if vs := runCheck(t, "check_claim_authority_ratchet", g); len(vs) != 0 {
		t.Fatalf("expected no claim_authority ratchet violation (claim_authority untouched), got: %+v", vs)
	}
	if vs := runCheck(t, "check_public_surface_authority_ratchet", g); len(vs) == 0 {
		t.Fatalf("expected a public_surface_authority ratchet violation (was linked, now removed), got none")
	}
}

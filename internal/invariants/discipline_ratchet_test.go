package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// --- F2: discipline:"full" one-way ratchet (task W7.2, @fx finding F2) -------
//
// Once a domain's manifest.json has been observed with discipline:"full"
// (pinned in graph.lock's DisciplineFullObserved by loader.WriteLock), a later
// manifest that no longer resolves discipline:"full" is a regression violation.
// Before F2, this one-way-door property was purely a documented convention --
// a resolver could silently delete/downgrade the discipline key and every
// discipline-gated check became an honest no-op again with zero signal.

// writeRatchetFixture writes a minimal domain directory (manifest.json +
// graph.json) with the given discipline value, plus a graph.lock, and returns
// the domain dir. If writeLock is true, loader.WriteLock is called to produce
// a real graph.lock (which will ratchet discipline_full_observed based on the
// current manifest's discipline value).
func writeRatchetFixture(t *testing.T, discipline string, writeLock bool) string {
	t.Helper()
	tmp := t.TempDir()
	manifest := `{"discipline": "` + discipline + `"}` + "\n"
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "graph.json"), []byte(`{"schema_version":3}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile graph.json: %v", err)
	}
	if writeLock {
		graphPath := filepath.Join(tmp, "graph.json")
		if err := loader.WriteLock(graphPath, "test ratchet pin"); err != nil {
			t.Fatalf("WriteLock: %v", err)
		}
	}
	return tmp
}

// ratchetGraph builds a graph with Discipline resolved from the fixture's own
// manifest.json, mirroring graphForDiscipline.
func ratchetGraph(t *testing.T, domainDir string) *ontology.Graph {
	t.Helper()
	graphPath := filepath.Join(domainDir, "graph.json")
	return &ontology.Graph{
		DomainDir:    domainDir,
		Discipline:   loader.ResolveDiscipline(graphPath),
		Stakeholders: []ontology.Stakeholder{sA},
	}
}

// TestCheckDisciplineRatchet_FiresOnRegression is the core F2 exploit case:
// a domain first landed with discipline:"full" (graph.lock pins
// DisciplineFullObserved=true), then the resolver silently removed the
// discipline key from manifest.json. check_discipline_ratchet MUST fire --
// the one-way door was violated.
func TestCheckDisciplineRatchet_FiresOnRegression(t *testing.T) {
	t.Parallel()
	// Step 1: land with discipline:full + write lock (pins the ratchet).
	domainDir := writeRatchetFixture(t, "full", true)

	// Verify the lock was written with the pin.
	graphPath := filepath.Join(domainDir, "graph.json")
	pin, exists := loader.ReadDisciplinePin(graphPath)
	if !exists {
		t.Fatalf("test setup: graph.lock should exist after WriteLock")
	}
	if !pin {
		t.Fatalf("test setup: expected DisciplineFullObserved=true after WriteLock with discipline:full, got false")
	}

	// Step 2: resolver silently removes the discipline key (regression).
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile regression: %v", err)
	}

	g := ratchetGraph(t, domainDir)
	if g.Discipline == loader.DisciplineFull {
		t.Fatalf("test setup: expected discipline to no longer be full after removal, got %q", g.Discipline)
	}

	vs := runCheck(t, "check_discipline_ratchet", g)
	if len(vs) == 0 {
		t.Fatalf("F2: expected check_discipline_ratchet to fire for a discipline regression (was full, now empty), got 0 violations")
	}
	if vs[0].Check != "check_discipline_ratchet" {
		t.Errorf("violation Check = %q, want check_discipline_ratchet", vs[0].Check)
	}
}

// TestCheckDisciplineRatchet_GreenWhenStaysFull is the happy-path
// non-regression case: a domain that stays discipline:"full" throughout must
// NOT falsely violate. The pin says true AND the live discipline is still
// "full" -- no regression.
func TestCheckDisciplineRatchet_GreenWhenStaysFull(t *testing.T) {
	t.Parallel()
	domainDir := writeRatchetFixture(t, "full", true)

	g := ratchetGraph(t, domainDir)
	if g.Discipline != loader.DisciplineFull {
		t.Fatalf("test setup: expected discipline=full, got %q", g.Discipline)
	}

	vs := runCheck(t, "check_discipline_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("F2: expected no violations for a domain that stays discipline:full (happy path), got: %+v", vs)
	}
}

// TestCheckDisciplineRatchet_NoOpWhenNeverFull proves the ratchet does NOT
// fire for a domain that was never discipline:"full" -- a soft-discipline
// domain whose lock has DisciplineFullObserved=false (or absent, as in older
// locks predating the F2 additive field).
func TestCheckDisciplineRatchet_NoOpWhenNeverFull(t *testing.T) {
	t.Parallel()
	domainDir := writeRatchetFixture(t, "", true) // soft discipline + lock

	graphPath := filepath.Join(domainDir, "graph.json")
	pin, exists := loader.ReadDisciplinePin(graphPath)
	if !exists {
		t.Fatalf("test setup: graph.lock should exist")
	}
	if pin {
		t.Fatalf("test setup: expected DisciplineFullObserved=false for a never-full domain, got true")
	}

	g := ratchetGraph(t, domainDir)
	vs := runCheck(t, "check_discipline_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("F2: expected no violations for a domain that was never discipline:full, got: %+v", vs)
	}
}

// TestCheckDisciplineRatchet_NoOpWhenNoLock proves the honest-no-op case for
// a domain with no graph.lock at all (never went through WriteGraph/WriteLock).
func TestCheckDisciplineRatchet_NoOpWhenNoLock(t *testing.T) {
	t.Parallel()
	domainDir := writeRatchetFixture(t, "full", false) // no lock written

	g := ratchetGraph(t, domainDir)
	vs := runCheck(t, "check_discipline_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("F2: expected no violations for a domain with no graph.lock, got: %+v", vs)
	}
}

// TestCheckDisciplineRatchet_NoOpOnSyntheticGraph proves the bail-out for a
// synthetic in-memory graph (no DomainDir).
func TestCheckDisciplineRatchet_NoOpOnSyntheticGraph(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	vs := runCheck(t, "check_discipline_ratchet", g)
	if len(vs) != 0 {
		t.Fatalf("F2: expected no violations on a synthetic graph (no DomainDir), got: %+v", vs)
	}
}

// TestWriteLock_RatchetsDisciplineFull proves the WriteLock ratchet mechanism
// itself: once WriteLock observes discipline:"full", the pin is set true, and
// a subsequent WriteLock with discipline removed PRESERVES the pin (once true,
// always true).
func TestWriteLock_RatchetsDisciplineFull(t *testing.T) {
	t.Parallel()
	domainDir := writeRatchetFixture(t, "full", false)
	graphPath := filepath.Join(domainDir, "graph.json")

	// First WriteLock: pins DisciplineFullObserved=true (live discipline is full).
	if err := loader.WriteLock(graphPath, "first lock"); err != nil {
		t.Fatalf("WriteLock first: %v", err)
	}
	pin, _ := loader.ReadDisciplinePin(graphPath)
	if !pin {
		t.Fatalf("after first WriteLock with discipline:full, expected pin=true, got false")
	}

	// Resolver removes discipline from manifest.
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile remove discipline: %v", err)
	}

	// Second WriteLock: pin MUST stay true (ratchet -- once true, always true).
	if err := loader.WriteLock(graphPath, "second lock after regression"); err != nil {
		t.Fatalf("WriteLock second: %v", err)
	}
	pin2, _ := loader.ReadDisciplinePin(graphPath)
	if !pin2 {
		t.Fatalf("after second WriteLock with discipline removed, expected pin=true (ratchet), got false")
	}
}

// --- F3: SPEC.md absence under discipline:full (task W7.2, @fx finding F3) ---
//
// A discipline:full domain that has no docs/gen/SPEC.md at all is NOT an
// honest no-op -- it has made the explicit promise that its normative text is
// generated from real scenario test runs. Before F3, the file-absent branch
// was an unconditional honest no-op regardless of discipline.

// TestCheckSpecMDCurrent_F3_FiresWhenAbsentUnderDisciplineFull proves F3: a
// discipline:full domain with NO SPEC.md must violate -- the domain promised
// generated normative text, and its absence is a broken promise, not an
// "hasn't adopted yet" state.
func TestCheckSpecMDCurrent_F3_FiresWhenAbsentUnderDisciplineFull(t *testing.T) {
	t.Parallel()
	// Build the spec-md fixture module, then add a manifest with discipline:full.
	domainDir := writeSpecMDFixtureModule(t)
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"),
		[]byte(`{"discipline": "full"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "graph.json"),
		[]byte(`{"schema_version":3}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile graph.json: %v", err)
	}

	g := specMDFixtureGraph(domainDir)
	g.Discipline = loader.ResolveDiscipline(filepath.Join(domainDir, "graph.json"))
	if g.Discipline != loader.DisciplineFull {
		t.Fatalf("test setup: expected discipline=full, got %q", g.Discipline)
	}

	// Precondition: no SPEC.md.
	specPath := filepath.Join(domainDir, "docs", "gen", "SPEC.md")
	if _, err := os.Stat(specPath); !os.IsNotExist(err) {
		t.Fatalf("precondition: docs/gen/SPEC.md must not exist")
	}

	vs := runCheck(t, "check_spec_md_current", g)
	if len(vs) == 0 {
		t.Fatalf("F3: expected check_spec_md_current to fire for a discipline:full domain with no SPEC.md, got 0 violations")
	}
	if vs[0].Check != "check_spec_md_current" {
		t.Errorf("violation Check = %q, want check_spec_md_current", vs[0].Check)
	}
}

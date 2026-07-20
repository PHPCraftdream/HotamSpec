package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// --- check_project_parent_declared ----------------------------------------
//
// Fixture shapes mirror scenario_discipline_test.go's
// writeScenarioDisciplineFixture / graphForDiscipline conventions: the
// manifest.json carries the field under test (here "parent" rather than
// "discipline"), and graphForParent builds an ontology.Graph the way
// loader.LoadGraph would populate it (DomainDir + ParentDeclared/Parent
// resolved from the fixture's own manifest.json via loader.ResolveParent),
// so the check under test sees exactly the same shape a real `hotam
// all-violations` run would -- not a hand-built graph with the field poked in
// directly, which would not exercise loader.ResolveParent at all.

// writeParentFixture writes a manifest.json (carrying the given parent
// representation, or no parent key at all) plus a graph.json, and returns the
// domain directory. parentJSON is the EXACT raw JSON text written for the
// manifest's body, so each tri-state case is authored at the byte level the
// real manifest would carry (the absent case writes a manifest with no
// "parent" key at all, NOT a manifest with parent:null).
func writeParentFixture(t *testing.T, manifestBody string) string {
	t.Helper()
	tmp := t.TempDir()
	writeInto := func(rel, content string) {
		full := filepath.Join(tmp, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", rel, err)
		}
	}
	writeInto("manifest.json", manifestBody)
	writeInto("graph.json", `{"schema_version":3}`)
	return tmp
}

// graphForParent builds an ontology.Graph the way loader.LoadGraph would
// populate it (DomainDir + ParentDeclared/Parent resolved from the fixture's
// own manifest.json), so the check under test sees exactly the same shape a
// real `hotam all-violations` run would. Mirrors graphForDiscipline.
func graphForParent(t *testing.T, domainDir string, reqs []ontology.Requirement) *ontology.Graph {
	t.Helper()
	graphPath := filepath.Join(domainDir, "graph.json")
	parent := loader.ResolveParent(graphPath)
	return &ontology.Graph{
		DomainDir:      domainDir,
		ManifestExists: parent.ManifestExists,
		ParentDeclared: parent.Declared,
		Parent:         parent.Value,
		Stakeholders:   []ontology.Stakeholder{sA},
		Requirements:   reqs,
	}
}

// TestCheckProjectParentDeclared_FiresWhenKeyAbsent is the RED case at the
// heart of D6: a manifest.json with NO "parent" key at all must fire exactly
// one violation. D6 makes the field MANDATORY, so its total absence is the
// canonical violation (distinct from "present and null", which is valid).
func TestCheckProjectParentDeclared_FiresWhenKeyAbsent(t *testing.T) {
	t.Parallel()
	// manifest carries other fields but NO "parent" key — the violation case.
	domainDir := writeParentFixture(t, `{"self_hosting": true}`+"\n")
	g := graphForParent(t, domainDir, []ontology.Requirement{settledReq("R-1", "sa")})
	if g.ParentDeclared {
		t.Fatalf("test setup: expected ParentDeclared=false for an absent key, got true")
	}
	vs := runCheck(t, "check_project_parent_declared", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly one violation for an absent parent key, got %d: %v", len(vs), vs)
	}
	if vs[0].Check != "check_project_parent_declared" {
		t.Errorf("violation Check = %q, want check_project_parent_declared", vs[0].Check)
	}
	if vs[0].ID != domainDir {
		t.Errorf("violation ID = %q, want the domain dir %q", vs[0].ID, domainDir)
	}
}

// TestCheckProjectParentDeclared_GreenWhenNull is the root-domain GREEN case:
// a manifest.json with "parent": null (the EXPLICIT root declaration D6
// reserves JSON null for) must contribute ZERO violations.
func TestCheckProjectParentDeclared_GreenWhenNull(t *testing.T) {
	t.Parallel()
	domainDir := writeParentFixture(t, `{"self_hosting": true, "parent": null}`+"\n")
	g := graphForParent(t, domainDir, []ontology.Requirement{settledReq("R-1", "sa")})
	if !g.ParentDeclared {
		t.Fatalf("test setup: expected ParentDeclared=true for parent:null, got false")
	}
	if g.Parent != "" {
		t.Fatalf("test setup: expected Parent=\"\" for parent:null, got %q", g.Parent)
	}
	if vs := runCheck(t, "check_project_parent_declared", g); len(vs) != 0 {
		t.Fatalf("expected no violations for parent:null (explicit root declaration), got %v", vs)
	}
}

// TestCheckProjectParentDeclared_GreenWhenString is the child-domain GREEN
// case: a manifest.json with "parent": "<name>" (a non-empty string naming
// the parent domain) must contribute ZERO violations.
func TestCheckProjectParentDeclared_GreenWhenString(t *testing.T) {
	t.Parallel()
	domainDir := writeParentFixture(t, `{"parent": "hotam-spec-self"}`+"\n")
	g := graphForParent(t, domainDir, []ontology.Requirement{settledReq("R-1", "sa")})
	if !g.ParentDeclared {
		t.Fatalf("test setup: expected ParentDeclared=true for a string parent, got false")
	}
	if g.Parent != "hotam-spec-self" {
		t.Fatalf("test setup: expected Parent=\"hotam-spec-self\", got %q", g.Parent)
	}
	if vs := runCheck(t, "check_project_parent_declared", g); len(vs) != 0 {
		t.Fatalf("expected no violations for parent:\"<name>\" (child declaration), got %v", vs)
	}
}

// TestCheckProjectParentDeclared_MUTATION_AbsentNullStringRoundTrip is the
// mutation probe the task's own verification step calls for (mirroring
// scenario_discipline_test.go's _MUTATION_ convention): the SAME domain's
// manifest.json is mutated through all three tri-states, proving the check is
// live (reads the real manifest.json each call, not a one-shot flag baked in
// at fixture-build time): absent (RED) → null (GREEN) → string (GREEN) →
// absent again (RED).
func TestCheckProjectParentDeclared_MUTATION_AbsentNullStringRoundTrip(t *testing.T) {
	t.Parallel()
	domainDir := writeParentFixture(t, `{"self_hosting": true}`+"\n") // start: no parent key
	manifestPath := filepath.Join(domainDir, "manifest.json")
	reqs := []ontology.Requirement{settledReq("R-1", "sa")}

	// ABSENT: no parent key → RED (one violation).
	g := graphForParent(t, domainDir, reqs)
	if g.ParentDeclared {
		t.Fatalf("ABSENT setup: expected ParentDeclared=false, got true")
	}
	if vs := runCheck(t, "check_project_parent_declared", g); len(vs) != 1 {
		t.Fatalf("ABSENT: expected one violation, got %d: %v", len(vs), vs)
	}

	// FLIP to null: add "parent": null → GREEN (explicit root declaration).
	if err := os.WriteFile(manifestPath, []byte(`{"self_hosting": true, "parent": null}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile flip-to-null: %v", err)
	}
	g2 := graphForParent(t, domainDir, reqs)
	if !g2.ParentDeclared {
		t.Fatalf("NULL setup: expected ParentDeclared=true after flip, got false")
	}
	if vs := runCheck(t, "check_project_parent_declared", g2); len(vs) != 0 {
		t.Fatalf("NULL: expected no violations after declaring parent:null, got %v", vs)
	}

	// FLIP to string: "parent": "some-parent" → still GREEN (child declaration).
	if err := os.WriteFile(manifestPath, []byte(`{"parent": "some-parent"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile flip-to-string: %v", err)
	}
	g3 := graphForParent(t, domainDir, reqs)
	if !g3.ParentDeclared || g3.Parent != "some-parent" {
		t.Fatalf("STRING setup: expected ParentDeclared=true Parent=\"some-parent\", got Declared=%v Parent=%q", g3.ParentDeclared, g3.Parent)
	}
	if vs := runCheck(t, "check_project_parent_declared", g3); len(vs) != 0 {
		t.Fatalf("STRING: expected no violations for a child declaration, got %v", vs)
	}

	// FLIP back to absent: remove the parent key entirely → RED again.
	if err := os.WriteFile(manifestPath, []byte(`{"self_hosting": true}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile flip-back-absent: %v", err)
	}
	g4 := graphForParent(t, domainDir, reqs)
	if g4.ParentDeclared {
		t.Fatalf("ABSENT-AGAIN setup: expected ParentDeclared=false, got true")
	}
	if vs := runCheck(t, "check_project_parent_declared", g4); len(vs) != 1 {
		t.Fatalf("ABSENT-AGAIN: expected one violation after removing the parent key, got %d: %v", len(vs), vs)
	}
}

// TestCheckProjectParentDeclared_NoOpOnSyntheticGraph proves the bail-out for
// a synthetic in-memory graph (no DomainDir) is an honest no-op, not a false
// positive: a graph built in-memory by a test fixture never went through
// loader.LoadGraph, so g.ParentDeclared is the meaningless zero value false,
// not a real "manifest lacks the key" signal. This is the exact shape and
// rationale check_graph_lock_pins_graph_json's own DomainDir bail establishes,
// and it is what keeps every existing test fixture that builds
// &ontology.Graph{...} directly from going red when this check lands.
func TestCheckProjectParentDeclared_NoOpOnSyntheticGraph(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}} // no DomainDir
	if g.ParentDeclared {
		t.Fatalf("test setup: expected the zero-value ParentDeclared=false on a synthetic graph, got true")
	}
	if vs := runCheck(t, "check_project_parent_declared", g); len(vs) != 0 {
		t.Fatalf("expected no violations on a synthetic in-memory graph (no DomainDir), got %v", vs)
	}
}

// TestCheckProjectParentDeclared_NoOpWhenManifestAbsentButDomainDirSet is the
// case that distinguishes g.ManifestExists from a bare g.DomainDir == "" bail:
// a graph with a REAL DomainDir (a genuine t.TempDir() path) but NO
// manifest.json ever written there -- the exact shape countless existing
// test fixtures across this codebase use (e.g. writeAuthoredSpecFixture in
// authored_links_test.go, which writes only source files, never a
// manifest.json). Gating on DomainDir alone would fire a false-positive
// violation against every one of them; gating on ManifestExists (resolved by
// actually trying to read manifest.json from that DomainDir) correctly
// no-ops here while still firing for a real on-disk domain whose manifest
// exists but omits "parent" (TestCheckProjectParentDeclared_FiresWhenKeyAbsent
// above).
func TestCheckProjectParentDeclared_NoOpWhenManifestAbsentButDomainDirSet(t *testing.T) {
	t.Parallel()
	domainDir := t.TempDir() // real directory, but no manifest.json written into it
	g := graphForParent(t, domainDir, []ontology.Requirement{settledReq("R-1", "sa")})
	if g.DomainDir == "" {
		t.Fatalf("test setup: expected a real (non-empty) DomainDir, got empty")
	}
	if g.ManifestExists {
		t.Fatalf("test setup: expected ManifestExists=false when no manifest.json was written, got true")
	}
	if vs := runCheck(t, "check_project_parent_declared", g); len(vs) != 0 {
		t.Fatalf("expected no violations when DomainDir is real but manifest.json is entirely absent, got %v", vs)
	}
}

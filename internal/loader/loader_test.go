package loader

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

const fixturePath = "testdata/hotam-spec-self.graph.json"

func TestLoadGraph_FixtureCounts(t *testing.T) {
	t.Parallel()
	g, err := LoadGraph(fixturePath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"axes", len(g.Axes), 9},
		{"stakeholders", len(g.Stakeholders), 4},
		{"assumptions", len(g.Assumptions), 16},
		{"requirements", len(g.Requirements), 275},
		{"conflicts", len(g.Conflicts), 8},
		{"operators", len(g.Operators), 1},
		{"processes", len(g.Processes), 1},
		{"goals", len(g.Goals), 1},
		{"entity_types", len(g.EntityTypes), 0},
		{"entities", len(g.Entities), 0},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %d want %d", c.name, c.got, c.want)
		}
	}
	if g.SelfHosting {
		t.Errorf("SelfHosting: fixture has no sibling manifest.json, want false, got true")
	}
}

func TestRoundTrip_DeepEqual(t *testing.T) {
	t.Parallel()
	g1, err := LoadGraph(fixturePath)
	if err != nil {
		t.Fatalf("LoadGraph #1: %v", err)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	if err := WriteGraph(out, g1); err != nil {
		t.Fatalf("WriteGraph: %v", err)
	}
	g2, err := LoadGraph(out)
	if err != nil {
		t.Fatalf("LoadGraph #2: %v", err)
	}
	if !reflect.DeepEqual(g1.Axes, g2.Axes) {
		t.Errorf("Axes differ")
	}
	if !reflect.DeepEqual(g1.Stakeholders, g2.Stakeholders) {
		t.Errorf("Stakeholders differ")
	}
	if !reflect.DeepEqual(g1.Assumptions, g2.Assumptions) {
		t.Errorf("Assumptions differ")
	}
	if !reflect.DeepEqual(g1.Requirements, g2.Requirements) {
		t.Errorf("Requirements differ")
	}
	if !reflect.DeepEqual(g1.Conflicts, g2.Conflicts) {
		t.Errorf("Conflicts differ")
	}
	if !reflect.DeepEqual(g1.Operators, g2.Operators) {
		t.Errorf("Operators differ")
	}
	if !reflect.DeepEqual(g1.Processes, g2.Processes) {
		t.Errorf("Processes differ")
	}
	if !reflect.DeepEqual(g1.Goals, g2.Goals) {
		t.Errorf("Goals differ")
	}
	if !reflect.DeepEqual(g1.EntityTypes, g2.EntityTypes) {
		t.Errorf("EntityTypes differ")
	}
	if !reflect.DeepEqual(g1.Entities, g2.Entities) {
		t.Errorf("Entities differ")
	}
}

func TestRoundTrip_ByteIdentical(t *testing.T) {
	t.Parallel()
	g, err := LoadGraph(fixturePath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	if err := WriteGraph(out, g); err != nil {
		t.Fatalf("WriteGraph: %v", err)
	}
	orig, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read written: %v", err)
	}
	if string(orig) != string(written) {
		t.Errorf("round-trip not byte-identical: orig=%d bytes, written=%d bytes", len(orig), len(written))
		o := string(orig)
		w := string(written)
		for i := 0; i < len(o) && i < len(w); i++ {
			if o[i] != w[i] {
				start := i - 40
				if start < 0 {
					start = 0
				}
				oEnd := i + 40
				if oEnd > len(o) {
					oEnd = len(o)
				}
				wEnd := i + 40
				if wEnd > len(w) {
					wEnd = len(w)
				}
				t.Errorf("first diff at byte %d\norig: %q\nwant: %q", i, o[start:oEnd], w[start:wEnd])
				break
			}
		}
	}
}

func TestWriteGraph_NoTmpLeft(t *testing.T) {
	t.Parallel()
	g, err := LoadGraph(fixturePath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	if err := WriteGraph(out, g); err != nil {
		t.Fatalf("WriteGraph: %v", err)
	}
	if _, err := os.Stat(out + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("graph .tmp file left behind after successful write: %v", err)
	}
	if _, err := os.Stat(LockPath(out) + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("lock .tmp file left behind after successful write: %v", err)
	}
}

func TestVerifyLock_HappyPath(t *testing.T) {
	t.Parallel()
	g, err := LoadGraph(fixturePath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	if err := WriteGraph(out, g); err != nil {
		t.Fatalf("WriteGraph: %v", err)
	}
	ok, err := VerifyLock(out)
	if err != nil {
		t.Fatalf("VerifyLock: %v", err)
	}
	if !ok {
		t.Errorf("VerifyLock: expected true, got false")
	}
}

func TestVerifyLock_Tampered(t *testing.T) {
	t.Parallel()
	g, err := LoadGraph(fixturePath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	if err := WriteGraph(out, g); err != nil {
		t.Fatalf("WriteGraph: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	data = append(data, []byte("\n  \n")...)
	if err := os.WriteFile(out, data, 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	ok, err := VerifyLock(out)
	if err != nil {
		t.Fatalf("VerifyLock returned error: %v", err)
	}
	if ok {
		t.Errorf("VerifyLock: expected false after tamper, got true")
	}
}

func TestVerifyLock_Absent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(out, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ok, err := VerifyLock(out)
	if ok {
		t.Errorf("VerifyLock: expected false when lock absent, got true")
	}
	if err == nil {
		t.Errorf("VerifyLock: expected error when lock absent, got nil")
	}
}

func TestWriteLock_ManualNote(t *testing.T) {
	t.Parallel()
	g, err := LoadGraph(fixturePath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	if err := WriteGraph(out, g); err != nil {
		t.Fatalf("WriteGraph: %v", err)
	}
	if err := WriteLock(out, "proposal-XYZ applied"); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}
	ok, err := VerifyLock(out)
	if err != nil {
		t.Fatalf("VerifyLock: %v", err)
	}
	if !ok {
		t.Errorf("VerifyLock after manual WriteLock: expected true")
	}
	raw, err := os.ReadFile(LockPath(out))
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	if !strings.Contains(string(raw), "proposal-XYZ applied") {
		t.Errorf("lock note not persisted: %s", string(raw))
	}
}

func TestLoadGraph_UnknownTopLevelKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	bad := `{"axes": [], "bogus_collection": []}` + "\n"
	if err := os.WriteFile(out, []byte(bad), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadGraph(out)
	if err == nil {
		t.Fatalf("LoadGraph: expected error for unknown top-level key, got nil")
	}
	if !strings.Contains(err.Error(), "bogus_collection") && !strings.Contains(err.Error(), "unknown") {
		t.Errorf("LoadGraph error does not name the unknown key: %v", err)
	}
}

func TestLoadGraph_MissingKeyIsEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	partial := `{"requirements": [{"id": "R-1", "claim": "x", "owner": "o", "status": "DRAFT", "why": "", "assumptions": [], "relations": [], "enforcement": "PROSE", "enforced_by": [], "m_tag": "", "enforceability": "ENFORCEABLE", "summary": "", "created_at": "", "settled_at": "", "last_reviewed_at": "", "review_after": "", "evidence": [], "source_refs": [], "history": []}]}` + "\n"
	if err := os.WriteFile(out, []byte(partial), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	g, err := LoadGraph(out)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	if len(g.Requirements) != 1 || g.Requirements[0].ID != "R-1" {
		t.Errorf("expected one requirement R-1, got %+v", g.Requirements)
	}
	if len(g.Axes) != 0 {
		t.Errorf("missing axes key should be empty, got %d", len(g.Axes))
	}
	if len(g.Conflicts) != 0 {
		t.Errorf("missing conflicts key should be empty, got %d", len(g.Conflicts))
	}
}

func TestLoadGraph_InvalidStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	bad := `{"requirements": [{"id": "R-1", "claim": "x", "owner": "o", "status": "NOPE", "why": "", "assumptions": [], "relations": [], "enforcement": "PROSE", "enforced_by": [], "m_tag": "", "enforceability": "ENFORCEABLE", "summary": "", "created_at": "", "settled_at": "", "last_reviewed_at": "", "review_after": "", "evidence": [], "source_refs": [], "history": []}]}` + "\n"
	if err := os.WriteFile(out, []byte(bad), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadGraph(out)
	if err == nil {
		t.Fatalf("LoadGraph: expected validation error for invalid status, got nil")
	}
	if !strings.Contains(err.Error(), "invalid status") {
		t.Errorf("LoadGraph error does not mention invalid status: %v", err)
	}
}

func TestSortedCopy_PreservesInput(t *testing.T) {
	t.Parallel()
	in := []ontology.Requirement{
		{ID: "R-2"},
		{ID: "R-1"},
		{ID: "R-3"},
	}
	out := sortedCopy(in, func(r ontology.Requirement) string { return r.ID })
	if in[0].ID != "R-2" {
		t.Errorf("sortedCopy mutated input: in[0].ID=%s", in[0].ID)
	}
	if out[0].ID != "R-1" || out[1].ID != "R-2" || out[2].ID != "R-3" {
		t.Errorf("sortedCopy wrong order: %v", []string{out[0].ID, out[1].ID, out[2].ID})
	}
}

// TestLoadGraph_NoSchemaVersion_BackwardCompat verifies that a graph.json with
// NO schema_version field (today's two committed domain graphs before backfill,
// and any synthetic fixture that omits it) loads successfully — the loader
// defaults a missing/zero version to CurrentSchemaVersion for backward
// compatibility rather than rejecting it.
func TestLoadGraph_NoSchemaVersion_BackwardCompat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	noVersion := `{"axes": [{"slug": "ax-1", "description": "d", "decl_order": 0}]}` + "\n"
	if err := os.WriteFile(out, []byte(noVersion), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	g, err := LoadGraph(out)
	if err != nil {
		t.Fatalf("LoadGraph without schema_version must succeed (backward compat): %v", err)
	}
	if g.SchemaVersion != ontology.CurrentSchemaVersion {
		t.Errorf("SchemaVersion: got %d, want %d (CurrentSchemaVersion)", g.SchemaVersion, ontology.CurrentSchemaVersion)
	}
	if len(g.Axes) != 1 || g.Axes[0].Slug != "ax-1" {
		t.Errorf("expected one axis ax-1, got %+v", g.Axes)
	}
}

// TestLoadGraph_CurrentSchemaVersion_RoundTrip verifies that a graph.json
// carrying the current schema_version loads, and that WriteGraph→LoadGraph
// preserves the version through a full round-trip.
func TestLoadGraph_CurrentSchemaVersion_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "graph.json")
	cv := ontology.CurrentSchemaVersion
	withVersion := `{"schema_version": ` + strconv.Itoa(cv) + `, "axes": [{"slug": "ax-1", "description": "d", "decl_order": 0}]}` + "\n"
	if err := os.WriteFile(src, []byte(withVersion), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	g1, err := LoadGraph(src)
	if err != nil {
		t.Fatalf("LoadGraph with current schema_version: %v", err)
	}
	if g1.SchemaVersion != cv {
		t.Fatalf("SchemaVersion: got %d, want %d", g1.SchemaVersion, cv)
	}
	// Round-trip through WriteGraph → LoadGraph.
	rt := filepath.Join(dir, "rt", "graph.json")
	if err := WriteGraph(rt, g1); err != nil {
		t.Fatalf("WriteGraph: %v", err)
	}
	g2, err := LoadGraph(rt)
	if err != nil {
		t.Fatalf("LoadGraph round-trip: %v", err)
	}
	if g2.SchemaVersion != cv {
		t.Errorf("round-trip SchemaVersion: got %d, want %d", g2.SchemaVersion, cv)
	}
}

// TestLoadGraph_NewerSchemaVersion_ClearError is the regression guard for the
// forward-compatibility concern: a graph.json whose schema_version is NEWER
// than the binary supports must produce a clear, actionable error naming the
// version gap — not an opaque "json: unknown field" decode failure.
func TestLoadGraph_NewerSchemaVersion_ClearError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	future := ontology.CurrentSchemaVersion + 5
	futureJSON := `{"schema_version": ` + strconv.Itoa(future) + `, "future_field": []}` + "\n"
	if err := os.WriteFile(out, []byte(futureJSON), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadGraph(out)
	if err == nil {
		t.Fatalf("LoadGraph: expected error for newer schema_version, got nil")
	}
	if !strings.Contains(err.Error(), "newer than this hotam binary supports") {
		t.Errorf("error must explain the version gap, got: %v", err)
	}
	if !strings.Contains(err.Error(), "upgrade hotam") {
		t.Errorf("error must suggest upgrading hotam, got: %v", err)
	}
	// Must NOT surface as the opaque unknown-field error from DisallowUnknownFields.
	if strings.Contains(err.Error(), "unknown field") {
		t.Errorf("error must not be opaque 'unknown field' for a newer version, got: %v", err)
	}
}

// TestLoadGraph_UnknownFieldOnCurrentVersion_StillCaught verifies that adding
// schema_version tolerance did NOT weaken DisallowUnknownFields for the CURRENT
// version — a genuinely unrecognized field (a typo, not a newer version) on a
// current-version graph is still rejected with a message naming the bad key.
func TestLoadGraph_UnknownFieldOnCurrentVersion_StillCaught(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	cv := ontology.CurrentSchemaVersion
	bad := `{"schema_version": ` + strconv.Itoa(cv) + `, "axes": [], "bogus_key": 42}` + "\n"
	if err := os.WriteFile(out, []byte(bad), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadGraph(out)
	if err == nil {
		t.Fatalf("LoadGraph: expected error for unknown field on current version, got nil")
	}
	if !strings.Contains(err.Error(), "bogus_key") && !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error must name the unknown key, got: %v", err)
	}
}

// TestLoadGraph_V1SchemaVersion_LoadsUnderCurrent is the migration regression
// guard for the loader's case-1 arm: a graph.json written in the v1 format
// (the shape before Requirement.blocked_on existed — the field is simply
// absent) must load losslessly under the CURRENT binary, whatever
// CurrentSchemaVersion has been bumped to since (v2, v3, ...). Because
// blocked_on is a purely additive OPTIONAL field (omitempty), a v1 file that
// lacks it decodes into the current Requirement struct as the Go zero-value
// "" with no data transformation and nothing for DisallowUnknownFields to
// reject. The loader's case-1 arm documents this; this test proves it on a
// minimal real v1-shaped payload and is deliberately version-agnostic (no
// CurrentSchemaVersion guard) so it keeps exercising case 1 across future
// schema bumps instead of silently skipping itself.
func TestLoadGraph_V1SchemaVersion_LoadsUnderCurrent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	// A true v1 shape: schema_version 1, a closeable-debt requirement with NO
	// blocked_on field at all (the field did not exist in v1).
	v1 := `{"schema_version": 1, "requirements": [{"id": "R-debt", "claim": "c", "owner": "S-o", "status": "SETTLED", "enforcement": "PROSE", "enforceability": "ENFORCEABLE", "decl_order": 0}]}` + "\n"
	if err := os.WriteFile(out, []byte(v1), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	g, err := LoadGraph(out)
	if err != nil {
		t.Fatalf("a v1 graph (schema_version:1) must load losslessly under the current binary: %v", err)
	}
	if g.SchemaVersion != ontology.CurrentSchemaVersion {
		t.Errorf("loaded SchemaVersion: got %d, want %d (CurrentSchemaVersion, normalized on load)", g.SchemaVersion, ontology.CurrentSchemaVersion)
	}
	if len(g.Requirements) != 1 || g.Requirements[0].ID != "R-debt" {
		t.Fatalf("requirement did not round-trip: %+v", g.Requirements)
	}
	// The additive-migration contract: a v1 requirement lacking blocked_on
	// decodes as the zero-value "", so it is closeable-NOW (not feature-blocked).
	r := g.Requirements[0]
	if r.BlockedOn != "" {
		t.Errorf("a v1 requirement with no blocked_on must decode as \"\", got %q", r.BlockedOn)
	}
	if !r.IsCloseableDebtNow() {
		t.Error("a v1 closeable-debt requirement with empty BlockedOn must be closeable-now")
	}
	if r.IsFeatureBlockedDebt() {
		t.Error("a v1 closeable-debt requirement with empty BlockedOn must NOT be feature-blocked")
	}
}

// TestLoadGraph_V2SchemaVersion_LoadsUnderCurrent is the mirror regression
// guard for the loader's case-2 arm: a graph.json written in the v2 format
// (the shape before Requirement.implemented_by/verified_by existed — both
// fields are simply absent) must load losslessly under the CURRENT binary.
// Because implemented_by/verified_by are purely additive OPTIONAL fields
// (omitempty []string), a v2 file that lacks them decodes into the current
// Requirement struct as nil slices with no data transformation and nothing
// for DisallowUnknownFields to reject. The loader's case-2 arm documents
// this; this test proves it on a minimal real v2-shaped payload and is
// deliberately version-agnostic (no CurrentSchemaVersion guard), mirroring
// TestLoadGraph_V1SchemaVersion_LoadsUnderCurrent.
func TestLoadGraph_V2SchemaVersion_LoadsUnderCurrent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "graph.json")
	// A true v2 shape: schema_version 2, a requirement with NO implemented_by
	// or verified_by fields at all (neither field existed in v2).
	v2 := `{"schema_version": 2, "requirements": [{"id": "R-tracked", "claim": "c", "owner": "S-o", "status": "SETTLED", "enforcement": "PROSE", "enforceability": "ENFORCEABLE", "decl_order": 0}]}` + "\n"
	if err := os.WriteFile(out, []byte(v2), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	g, err := LoadGraph(out)
	if err != nil {
		t.Fatalf("a v2 graph (schema_version:2) must load losslessly under the current binary: %v", err)
	}
	if g.SchemaVersion != ontology.CurrentSchemaVersion {
		t.Errorf("loaded SchemaVersion: got %d, want %d (CurrentSchemaVersion, normalized on load)", g.SchemaVersion, ontology.CurrentSchemaVersion)
	}
	if len(g.Requirements) != 1 || g.Requirements[0].ID != "R-tracked" {
		t.Fatalf("requirement did not round-trip: %+v", g.Requirements)
	}
	// The additive-migration contract: a v2 requirement lacking
	// implemented_by/verified_by decodes as nil slices.
	r := g.Requirements[0]
	if r.ImplementedBy != nil {
		t.Errorf("a v2 requirement with no implemented_by must decode as nil, got %#v", r.ImplementedBy)
	}
	if r.VerifiedBy != nil {
		t.Errorf("a v2 requirement with no verified_by must decode as nil, got %#v", r.VerifiedBy)
	}
}

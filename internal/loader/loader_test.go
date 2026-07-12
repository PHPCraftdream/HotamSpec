package loader

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

const fixturePath = "testdata/hotam-spec-self.graph.json"

func TestLoadGraph_FixtureCounts(t *testing.T) {
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

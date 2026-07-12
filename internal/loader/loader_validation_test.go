package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// validGraph returns a fresh minimal graph that passes validateGraph. Each
// collection carries exactly one well-formed entry so a test can mutate a
// single field and assert the corresponding validation message.
func validGraph() *ontology.Graph {
	return &ontology.Graph{
		Axes:         []ontology.Axis{{Slug: "ax-1"}},
		Stakeholders: []ontology.Stakeholder{{ID: "sh-1"}},
		Assumptions:  []ontology.Assumption{{ID: "A-1", Status: "HOLDS"}},
		Requirements: []ontology.Requirement{{
			ID:             "R-1",
			Status:         "DRAFT",
			Enforcement:    "PROSE",
			Enforceability: "ENFORCEABLE",
		}},
		Conflicts: []ontology.Conflict{{ID: "C-1", Axis: "ax-1", Lifecycle: "DETECTED"}},
		Operators: []ontology.Operator{{
			ID:            "OP-1",
			ContextBudget: ontology.ContextBudget{Measure: "CRYSTAL_CHARS"},
		}},
		Processes:   []ontology.Process{{ID: "P-1"}},
		Goals:       []ontology.Goal{{ID: "G-1", TargetState: ontology.TargetState{Kind: "GRAPH_PROPERTY"}}},
		EntityTypes: []ontology.EntityType{{Slug: "et-1"}},
		Entities:    []ontology.EntityInstance{{ID: "E-1", EntityType: "et-1"}},
	}
}

func TestValidateGraph_ValidPasses(t *testing.T) {
	t.Parallel()
	if err := validateGraph(validGraph()); err != nil {
		t.Fatalf("well-formed graph should pass validation, got: %v", err)
	}
}

// TestValidateGraph_InvalidBranches drives every error branch in validateGraph
// directly (the function is package-private), each case mutating one field of
// the otherwise-valid baseline so the resulting error names exactly the field
// under test.
func TestValidateGraph_InvalidBranches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		mutate     func(*ontology.Graph)
		wantSubstr string
	}{
		{"axis empty slug", func(g *ontology.Graph) { g.Axes[0].Slug = "" }, "axes[0]: empty slug"},
		{"stakeholder empty id", func(g *ontology.Graph) { g.Stakeholders[0].ID = "" }, "stakeholders[0]: empty id"},
		{"assumption empty id", func(g *ontology.Graph) { g.Assumptions[0].ID = "" }, "assumptions[0]: empty id"},
		{"assumption invalid status", func(g *ontology.Graph) { g.Assumptions[0].Status = "NOPE" }, "invalid status"},
		{"requirement empty id", func(g *ontology.Graph) { g.Requirements[0].ID = "" }, "requirements[0]: empty id"},
		{"requirement invalid status", func(g *ontology.Graph) { g.Requirements[0].Status = "NOPE" }, "invalid status"},
		{"requirement invalid enforcement", func(g *ontology.Graph) { g.Requirements[0].Enforcement = "NOPE" }, "invalid enforcement"},
		{"requirement invalid enforceability", func(g *ontology.Graph) { g.Requirements[0].Enforceability = "NOPE" }, "invalid enforceability"},
		{"requirement invalid relation kind", func(g *ontology.Graph) {
			g.Requirements[0].Relations = []ontology.Relation{{Kind: "NOPE", Target: "R-2"}}
		}, "invalid kind"},
		{"conflict empty id", func(g *ontology.Graph) { g.Conflicts[0].ID = "" }, "conflicts[0]: empty id"},
		{"conflict empty axis", func(g *ontology.Graph) { g.Conflicts[0].Axis = "" }, "empty axis"},
		{"conflict invalid lifecycle", func(g *ontology.Graph) { g.Conflicts[0].Lifecycle = "NOPE" }, "invalid lifecycle"},
		{"operator empty id", func(g *ontology.Graph) { g.Operators[0].ID = "" }, "operators[0]: empty id"},
		{"operator invalid budget measure", func(g *ontology.Graph) {
			g.Operators[0].ContextBudget.Measure = "NOPE"
		}, "invalid budget measure"},
		{"process empty id", func(g *ontology.Graph) { g.Processes[0].ID = "" }, "processes[0]: empty id"},
		{"goal empty id", func(g *ontology.Graph) { g.Goals[0].ID = "" }, "goals[0]: empty id"},
		{"goal invalid target kind", func(g *ontology.Graph) { g.Goals[0].TargetState.Kind = "NOPE" }, "invalid target_state.kind"},
		{"entity_type empty slug", func(g *ontology.Graph) { g.EntityTypes[0].Slug = "" }, "entity_types[0]: empty slug"},
		{"entity empty id", func(g *ontology.Graph) { g.Entities[0].ID = "" }, "entities[0]: empty id"},
		{"entity empty entity_type", func(g *ontology.Graph) { g.Entities[0].EntityType = "" }, "empty entity_type"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := validGraph()
			c.mutate(g)
			err := validateGraph(g)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.wantSubstr)
			}
			if !strings.Contains(err.Error(), c.wantSubstr) {
				t.Errorf("expected error containing %q, got: %v", c.wantSubstr, err)
			}
		})
	}
}

func TestResolveSelfHosting_TrueManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"self_hosting": true}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if got := resolveSelfHosting(filepath.Join(dir, "graph.json")); !got {
		t.Errorf("manifest self_hosting=true must yield true, got false")
	}
}

func TestResolveSelfHosting_MalformedManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{not valid json`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if got := resolveSelfHosting(filepath.Join(dir, "graph.json")); got {
		t.Errorf("malformed manifest must fall back to false, got true")
	}
}

func TestWriteGraph_NilGraph(t *testing.T) {
	t.Parallel()
	err := WriteGraph(filepath.Join(t.TempDir(), "graph.json"), nil)
	if err == nil {
		t.Fatalf("WriteGraph(nil) must error")
	}
	if !strings.Contains(err.Error(), "nil graph") {
		t.Errorf("expected nil-graph error, got: %v", err)
	}
}

func TestWriteGraph_ParentIsFileFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// 'blocker' is a regular file; using it as the parent dir makes
	// atomicWriteFile's MkdirAll fail — the atomic write must surface that
	// error rather than silently writing into a bogus path.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	g, err := LoadGraph(fixturePath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	err = WriteGraph(filepath.Join(blocker, "graph.json"), g)
	if err == nil {
		t.Fatalf("WriteGraph under a regular file must fail")
	}
	if !strings.Contains(err.Error(), "mkdir") {
		t.Errorf("expected mkdir error, got: %v", err)
	}
}

func TestWriteLock_MissingGraphFile(t *testing.T) {
	t.Parallel()
	err := WriteLock(filepath.Join(t.TempDir(), "missing.json"), "note")
	if err == nil {
		t.Fatalf("WriteLock on a missing graph file must error")
	}
	if !strings.Contains(err.Error(), "write lock") {
		t.Errorf("expected write lock error, got: %v", err)
	}
}

func TestVerifyLock_MalformedLock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(graphPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LockPath(graphPath), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := VerifyLock(graphPath)
	if ok {
		t.Errorf("malformed lock must yield ok=false, got true")
	}
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestVerifyLock_EmptySHA256(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(graphPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LockPath(graphPath), []byte(`{"sha256":"","updated_at":"","note":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := VerifyLock(graphPath)
	if ok {
		t.Errorf("lock with empty sha256 must yield ok=false, got true")
	}
	if err == nil || !strings.Contains(err.Error(), "empty sha256") {
		t.Errorf("expected empty sha256 error, got: %v", err)
	}
}

func TestVerifyLock_GraphFileGone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	// lock references a hash, but the graph file itself is absent → sha256File fails
	if err := os.WriteFile(LockPath(graphPath), []byte(`{"sha256":"abc","updated_at":"","note":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := VerifyLock(graphPath)
	if ok {
		t.Errorf("missing graph file must yield ok=false, got true")
	}
	if err == nil || !strings.Contains(err.Error(), "hash") {
		t.Errorf("expected hash error, got: %v", err)
	}
}

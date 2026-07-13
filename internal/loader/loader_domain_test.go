package loader

import (
	"os"
	"path/filepath"
	"testing"
)

const domainGraphPath = "../../domains/hotam-spec-self/graph.json"

func TestLoadGraph_DomainHotamSpecSelf(t *testing.T) {
	t.Parallel()
	g, err := LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", domainGraphPath, err)
	}

	cases := []struct {
		name string
		got  int
		want int
	}{
		{"axes", len(g.Axes), 9},
		{"stakeholders", len(g.Stakeholders), 4},
		{"assumptions", len(g.Assumptions), 16},
		{"requirements", len(g.Requirements), 281},
		{"conflicts", len(g.Conflicts), 8},
		{"operators", len(g.Operators), 1},
		{"processes", len(g.Processes), 1},
		{"goals", len(g.Goals), 1},
		{"entity_types", len(g.EntityTypes), 0},
		{"entities", len(g.Entities), 0},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}

	if !g.SelfHosting {
		t.Errorf("SelfHosting: want true (manifest.json present), got false")
	}
}

func TestLoadGraph_DomainHotamSpecSelf_GenerateLock(t *testing.T) {
	t.Parallel()
	lockPath := LockPath(domainGraphPath)
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Logf("generating %s …", lockPath)
		if err := WriteLock(domainGraphPath, "initial domain lock from test"); err != nil {
			t.Fatalf("WriteLock(%s): %v", domainGraphPath, err)
		}
	}

	ok, err := VerifyLock(domainGraphPath)
	if err != nil {
		t.Fatalf("VerifyLock(%s): %v", domainGraphPath, err)
	}
	if !ok {
		abs, _ := filepath.Abs(domainGraphPath)
		t.Errorf("VerifyLock(%s): lock does not match graph; re-run to regenerate", abs)
	}
}
